// Package vision provides unified interfaces for vision/image analysis providers.
package vision

import (
	"context"
	"strings"
	"sync"
	"time"
)

// ContextExtractor analyzes frames to understand user activity.
// It uses the Vision Router to classify screen content and extract context.
//
// CR-023: CortexEyes - Screen Awareness & Contextual Learning
type ContextExtractor struct {
	router *Router // Existing vision router for analysis

	mu           sync.RWMutex
	contextCache []*UserContext // Recent context history (ring buffer)
	cacheSize    int
	cacheIndex   int
}

// UserContext represents what the user is currently doing.
type UserContext struct {
	Activity      string    `json:"activity"`       // "coding", "browsing", "writing", "designing", etc.
	Application   string    `json:"application"`    // "VS Code", "Chrome", "Figma", etc.
	Domain        string    `json:"domain"`         // "go", "javascript", "documentation", etc.
	ContentType   string    `json:"content_type"`   // "code", "article", "video", "chat", etc.
	FocusArea     string    `json:"focus_area"`     // What specifically is visible
	Confidence    float64   `json:"confidence"`     // 0.0-1.0
	ExtractedText string    `json:"extracted_text"` // OCR results if relevant
	Timestamp     time.Time `json:"timestamp"`
	AnalysisMs    int64     `json:"analysis_ms"` // How long analysis took
}

// ContextExtractorConfig configures the context extractor.
type ContextExtractorConfig struct {
	CacheSize int // Number of recent contexts to keep (default: 10)
}

// NewContextExtractor creates a new context extractor.
func NewContextExtractor(router *Router, config *ContextExtractorConfig) *ContextExtractor {
	cacheSize := 10
	if config != nil && config.CacheSize > 0 {
		cacheSize = config.CacheSize
	}

	return &ContextExtractor{
		router:       router,
		contextCache: make([]*UserContext, cacheSize),
		cacheSize:    cacheSize,
		cacheIndex:   0,
	}
}

// ExtractContext analyzes a frame and returns user context.
func (e *ContextExtractor) ExtractContext(ctx context.Context, frame *Frame) (*UserContext, error) {
	if e.router == nil {
		return nil, ErrVisionDisabled
	}

	start := time.Now()

	// Build analysis request
	req := &AnalyzeRequest{
		Image:    frame.Data,
		MimeType: frame.MimeType,
		Prompt:   contextExtractionPrompt,
	}

	// Use vision router to analyze
	resp, err := e.router.Analyze(ctx, req)
	if err != nil {
		return nil, err
	}

	// Parse response into UserContext
	userCtx := e.parseResponse(resp.Content)
	userCtx.Timestamp = time.Now()
	userCtx.AnalysisMs = time.Since(start).Milliseconds()

	// Cache the context
	e.cacheContext(userCtx)

	return userCtx, nil
}

// contextExtractionPrompt is the prompt used to extract context from screenshots.
const contextExtractionPrompt = `Analyze this screenshot and identify:
1. ACTIVITY: What is the user doing? (coding, browsing, writing, designing, chatting, reading, watching, terminal, unknown)
2. APPLICATION: What application is visible? (VS Code, Chrome, Safari, Terminal, Slack, etc.)
3. DOMAIN: What domain/language/topic? (go, python, javascript, documentation, email, social, etc.)
4. CONTENT_TYPE: What type of content? (code, article, video, chat, terminal_output, form, unknown)
5. FOCUS_AREA: Brief description of what's on screen (max 20 words)

Respond in this exact format:
ACTIVITY: <activity>
APPLICATION: <app>
DOMAIN: <domain>
CONTENT_TYPE: <type>
FOCUS_AREA: <description>`

// parseResponse parses the vision model response into UserContext.
func (e *ContextExtractor) parseResponse(content string) *UserContext {
	ctx := &UserContext{
		Activity:    "unknown",
		Application: "unknown",
		Domain:      "unknown",
		ContentType: "unknown",
		FocusArea:   "",
		Confidence:  0.5, // Default confidence
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "ACTIVITY:") {
			ctx.Activity = strings.TrimSpace(strings.TrimPrefix(line, "ACTIVITY:"))
			ctx.Confidence += 0.1
		} else if strings.HasPrefix(line, "APPLICATION:") {
			ctx.Application = strings.TrimSpace(strings.TrimPrefix(line, "APPLICATION:"))
			ctx.Confidence += 0.1
		} else if strings.HasPrefix(line, "DOMAIN:") {
			ctx.Domain = strings.TrimSpace(strings.TrimPrefix(line, "DOMAIN:"))
			ctx.Confidence += 0.1
		} else if strings.HasPrefix(line, "CONTENT_TYPE:") {
			ctx.ContentType = strings.TrimSpace(strings.TrimPrefix(line, "CONTENT_TYPE:"))
			ctx.Confidence += 0.1
		} else if strings.HasPrefix(line, "FOCUS_AREA:") {
			ctx.FocusArea = strings.TrimSpace(strings.TrimPrefix(line, "FOCUS_AREA:"))
			ctx.Confidence += 0.1
		}
	}

	// Cap confidence at 1.0
	if ctx.Confidence > 1.0 {
		ctx.Confidence = 1.0
	}

	// Normalize activity names
	ctx.Activity = normalizeActivity(ctx.Activity)

	return ctx
}

// normalizeActivity normalizes activity names to canonical form.
func normalizeActivity(activity string) string {
	activity = strings.ToLower(strings.TrimSpace(activity))

	// Map variations to canonical names
	activityMap := map[string]string{
		"code":       "coding",
		"programming": "coding",
		"development": "coding",
		"browse":     "browsing",
		"web":        "browsing",
		"internet":   "browsing",
		"write":      "writing",
		"typing":     "writing",
		"document":   "writing",
		"design":     "designing",
		"graphic":    "designing",
		"chat":       "chatting",
		"message":    "chatting",
		"slack":      "chatting",
		"read":       "reading",
		"watch":      "watching",
		"video":      "watching",
		"term":       "terminal",
		"shell":      "terminal",
		"console":    "terminal",
	}

	if canonical, ok := activityMap[activity]; ok {
		return canonical
	}

	return activity
}

// cacheContext adds a context to the ring buffer cache.
func (e *ContextExtractor) cacheContext(ctx *UserContext) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.contextCache[e.cacheIndex] = ctx
	e.cacheIndex = (e.cacheIndex + 1) % e.cacheSize
}

// GetRecentContexts returns recent contexts from the cache.
func (e *ContextExtractor) GetRecentContexts(count int) []*UserContext {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if count > e.cacheSize {
		count = e.cacheSize
	}

	result := make([]*UserContext, 0, count)

	// Read from cache in reverse order (most recent first)
	for i := 0; i < count; i++ {
		idx := (e.cacheIndex - 1 - i + e.cacheSize) % e.cacheSize
		if e.contextCache[idx] != nil {
			result = append(result, e.contextCache[idx])
		}
	}

	return result
}

// GetCurrentContext returns the most recent context.
func (e *ContextExtractor) GetCurrentContext() *UserContext {
	e.mu.RLock()
	defer e.mu.RUnlock()

	idx := (e.cacheIndex - 1 + e.cacheSize) % e.cacheSize
	return e.contextCache[idx]
}

// DetectPattern analyzes recent contexts to detect patterns.
func (e *ContextExtractor) DetectPattern() *ContextPattern {
	contexts := e.GetRecentContexts(e.cacheSize)
	if len(contexts) < 3 {
		return nil // Not enough data for pattern detection
	}

	pattern := &ContextPattern{
		TimeOfDay:   getTimeOfDay(time.Now()),
		DayOfWeek:   getDayOfWeek(time.Now()),
		CommonApps:  make([]string, 0),
		CommonTasks: make([]string, 0),
		TypicalFlow: make([]string, 0),
	}

	// Count app and activity occurrences
	appCounts := make(map[string]int)
	activityCounts := make(map[string]int)

	for _, ctx := range contexts {
		if ctx != nil {
			appCounts[ctx.Application]++
			activityCounts[ctx.Activity]++
		}
	}

	// Find most common apps (top 3)
	for app, count := range appCounts {
		if count >= 2 { // At least 2 occurrences
			pattern.CommonApps = append(pattern.CommonApps, app)
		}
	}

	// Find most common activities (top 3)
	for activity, count := range activityCounts {
		if count >= 2 { // At least 2 occurrences
			pattern.CommonTasks = append(pattern.CommonTasks, activity)
		}
	}

	// Build typical flow (sequence of activities)
	seen := make(map[string]bool)
	for _, ctx := range contexts {
		if ctx != nil && !seen[ctx.Activity] {
			pattern.TypicalFlow = append(pattern.TypicalFlow, ctx.Activity)
			seen[ctx.Activity] = true
		}
	}

	return pattern
}

// ContextPattern represents learned patterns over time.
type ContextPattern struct {
	TimeOfDay   string   `json:"time_of_day"`   // "morning", "afternoon", "evening", "night"
	DayOfWeek   string   `json:"day_of_week"`   // "weekday", "weekend"
	CommonApps  []string `json:"common_apps"`   // Most used apps
	CommonTasks []string `json:"common_tasks"`  // Most common activities
	TypicalFlow []string `json:"typical_flow"`  // Sequence of activities
}

// getTimeOfDay returns the time of day category.
func getTimeOfDay(t time.Time) string {
	hour := t.Hour()
	switch {
	case hour >= 5 && hour < 12:
		return "morning"
	case hour >= 12 && hour < 17:
		return "afternoon"
	case hour >= 17 && hour < 21:
		return "evening"
	default:
		return "night"
	}
}

// getDayOfWeek returns weekday or weekend.
func getDayOfWeek(t time.Time) string {
	day := t.Weekday()
	if day == time.Saturday || day == time.Sunday {
		return "weekend"
	}
	return "weekday"
}

// ClearCache clears the context cache.
func (e *ContextExtractor) ClearCache() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i := range e.contextCache {
		e.contextCache[i] = nil
	}
	e.cacheIndex = 0
}
