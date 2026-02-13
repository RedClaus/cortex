// Package vision provides unified interfaces for vision/image analysis providers.
package vision

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/normanking/cortex/internal/bus"
	"github.com/normanking/cortex/internal/logging"
)

// InsightType categorizes the kind of insight generated.
type InsightType string

const (
	InsightTypeWorkflow    InsightType = "workflow"     // Workflow optimization suggestions
	InsightTypeContextual  InsightType = "contextual"   // Context-aware assistance
	InsightTypePattern     InsightType = "pattern"      // Detected usage patterns
	InsightTypeProductivity InsightType = "productivity" // Productivity improvements
	InsightTypeReminder    InsightType = "reminder"     // Contextual reminders
)

// InsightPriority indicates urgency of an insight.
type InsightPriority string

const (
	InsightPriorityHigh   InsightPriority = "high"   // Show immediately
	InsightPriorityMedium InsightPriority = "medium" // Show when convenient
	InsightPriorityLow    InsightPriority = "low"    // Background suggestion
)

// Insight represents a proactive suggestion generated from screen observations.
type Insight struct {
	ID          string          `json:"id"`
	Type        InsightType     `json:"type"`
	Priority    InsightPriority `json:"priority"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Action      string          `json:"action,omitempty"`      // Suggested action
	ActionType  string          `json:"action_type,omitempty"` // e.g., "command", "navigation", "reminder"
	Context     *UserContext    `json:"context,omitempty"`     // Screen context that triggered insight
	Confidence  float64         `json:"confidence"`            // 0.0-1.0
	CreatedAt   time.Time       `json:"created_at"`
	ExpiresAt   *time.Time      `json:"expires_at,omitempty"` // When insight becomes irrelevant
	Dismissed   bool            `json:"dismissed"`
	Shown       bool            `json:"shown"`
}

// InsightEngineConfig configures the insight engine.
type InsightEngineConfig struct {
	// Analysis settings
	MinObservationsForPattern int           // Minimum observations before pattern detection
	PatternWindow             time.Duration // Time window for pattern analysis
	InsightCooldown           time.Duration // Minimum time between similar insights
	MaxActiveInsights         int           // Maximum insights to keep active

	// Confidence thresholds
	MinPatternConfidence float64 // Minimum confidence for pattern-based insights
	MinContextConfidence float64 // Minimum confidence for context-based insights

	// Feature flags
	EnableWorkflowInsights    bool
	EnableContextualInsights  bool
	EnablePatternInsights     bool
	EnableProductivityInsights bool
}

// DefaultInsightEngineConfig returns default configuration.
func DefaultInsightEngineConfig() *InsightEngineConfig {
	return &InsightEngineConfig{
		MinObservationsForPattern:  10,
		PatternWindow:              24 * time.Hour,
		InsightCooldown:            15 * time.Minute,
		MaxActiveInsights:          10,
		MinPatternConfidence:       0.7,
		MinContextConfidence:       0.6,
		EnableWorkflowInsights:     true,
		EnableContextualInsights:   true,
		EnablePatternInsights:      true,
		EnableProductivityInsights: true,
	}
}

// InsightPattern is an internal pattern representation for the insight engine.
type InsightPattern struct {
	DominantApp      string   `json:"dominant_app"`
	DominantActivity string   `json:"dominant_activity"`
	DominantDomain   string   `json:"dominant_domain"`
	FrequentApps     []string `json:"frequent_apps"`
	Confidence       float64  `json:"confidence"`
}

// InsightEngine generates proactive suggestions from screen observations.
// It analyzes patterns and context to provide helpful insights.
//
// CR-023: Phase 2 - Proactive Insights
type InsightEngine struct {
	// Dependencies
	memory   *ObservationMemory
	eventBus bus.Bus
	log      *logging.Logger

	// State
	mu             sync.RWMutex
	insights       []*Insight
	patternCache   map[string]*InsightPattern // app -> recent pattern
	lastInsightAt  map[string]time.Time       // insight type -> last shown
	insightCounter int64

	// Configuration
	config *InsightEngineConfig

	// Lifecycle
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewInsightEngine creates a new insight engine.
func NewInsightEngine(memory *ObservationMemory, eventBus bus.Bus, config *InsightEngineConfig) *InsightEngine {
	if config == nil {
		config = DefaultInsightEngineConfig()
	}

	return &InsightEngine{
		memory:        memory,
		eventBus:      eventBus,
		log:           logging.Global(),
		insights:      make([]*Insight, 0),
		patternCache:  make(map[string]*InsightPattern),
		lastInsightAt: make(map[string]time.Time),
		config:        config,
		stopCh:        make(chan struct{}),
	}
}

// Start begins the insight engine's background analysis.
func (ie *InsightEngine) Start(ctx context.Context) error {
	ie.log.Info("[InsightEngine] Starting background analysis")

	ie.wg.Add(1)
	go ie.analysisLoop(ctx)

	return nil
}

// Stop stops the insight engine.
func (ie *InsightEngine) Stop() {
	close(ie.stopCh)
	ie.wg.Wait()
	ie.log.Info("[InsightEngine] Stopped")
}

// analysisLoop periodically analyzes observations for insights.
func (ie *InsightEngine) analysisLoop(ctx context.Context) {
	defer ie.wg.Done()

	// Run analysis every 5 minutes
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ie.stopCh:
			return
		case <-ticker.C:
			ie.analyzePatterns(ctx)
			ie.pruneExpiredInsights()
		}
	}
}

// ProcessObservation analyzes a new observation for immediate insights.
func (ie *InsightEngine) ProcessObservation(ctx context.Context, obs *Observation) {
	if obs == nil || obs.Context == nil {
		return
	}

	// Check for contextual insights (immediate)
	if ie.config.EnableContextualInsights {
		ie.checkContextualInsights(ctx, obs)
	}

	// Check for workflow insights
	if ie.config.EnableWorkflowInsights {
		ie.checkWorkflowInsights(ctx, obs)
	}
}

// checkContextualInsights looks for context-specific assistance opportunities.
func (ie *InsightEngine) checkContextualInsights(ctx context.Context, obs *Observation) {
	userCtx := obs.Context

	// Skip low confidence contexts
	if userCtx.Confidence < ie.config.MinContextConfidence {
		return
	}

	// Check for specific patterns that warrant insights

	// Example: User looking at error messages
	if containsError(userCtx.Activity) || containsError(userCtx.FocusArea) {
		ie.maybeCreateInsight(&Insight{
			Type:        InsightTypeContextual,
			Priority:    InsightPriorityMedium,
			Title:       "Error Detected",
			Description: fmt.Sprintf("I noticed an error in %s. Would you like help troubleshooting?", userCtx.Application),
			Action:      "Analyze the error and suggest solutions",
			ActionType:  "command",
			Context:     userCtx,
			Confidence:  userCtx.Confidence,
		})
	}

	// Example: User in documentation
	if isDocumentation(userCtx.Activity) || isDocumentation(userCtx.Domain) {
		ie.maybeCreateInsight(&Insight{
			Type:        InsightTypeContextual,
			Priority:    InsightPriorityLow,
			Title:       "Documentation Context",
			Description: fmt.Sprintf("You're reading docs about %s. I can help explain or demonstrate.", userCtx.FocusArea),
			ActionType:  "assistance",
			Context:     userCtx,
			Confidence:  userCtx.Confidence,
		})
	}

	// Example: User debugging
	if isDebugging(userCtx.Activity) {
		ie.maybeCreateInsight(&Insight{
			Type:        InsightTypeContextual,
			Priority:    InsightPriorityMedium,
			Title:       "Debugging Session",
			Description: "I can help analyze your debugging session and suggest potential issues.",
			ActionType:  "assistance",
			Context:     userCtx,
			Confidence:  userCtx.Confidence,
		})
	}
}

// checkWorkflowInsights looks for workflow optimization opportunities.
func (ie *InsightEngine) checkWorkflowInsights(ctx context.Context, obs *Observation) {
	userCtx := obs.Context

	// Check for app switching patterns
	ie.mu.RLock()
	cachedPattern := ie.patternCache[userCtx.Application]
	ie.mu.RUnlock()

	if cachedPattern != nil && cachedPattern.Confidence > ie.config.MinPatternConfidence {
		// Frequent app switches might indicate a workflow issue
		if len(cachedPattern.FrequentApps) > 3 {
			ie.maybeCreateInsight(&Insight{
				Type:        InsightTypeWorkflow,
				Priority:    InsightPriorityLow,
				Title:       "Workflow Pattern Detected",
				Description: fmt.Sprintf("You frequently switch between %s. Consider using split views or keyboard shortcuts.", formatAppList(cachedPattern.FrequentApps)),
				ActionType:  "suggestion",
				Confidence:  cachedPattern.Confidence,
			})
		}
	}
}

// analyzePatterns runs deeper pattern analysis on historical observations.
func (ie *InsightEngine) analyzePatterns(ctx context.Context) {
	if ie.memory == nil {
		return
	}

	// Get recent observations
	observations, err := ie.memory.GetRecent(ctx, 50)
	if err != nil {
		ie.log.Warn("[InsightEngine] Failed to get observations: %v", err)
		return
	}

	if len(observations) < ie.config.MinObservationsForPattern {
		return
	}

	// Analyze patterns
	patterns := ie.detectPatterns(observations)

	// Update pattern cache
	ie.mu.Lock()
	for app, pattern := range patterns {
		ie.patternCache[app] = pattern
	}
	ie.mu.Unlock()

	// Generate pattern insights
	if ie.config.EnablePatternInsights {
		ie.generatePatternInsights(patterns)
	}

	// Generate productivity insights
	if ie.config.EnableProductivityInsights {
		ie.generateProductivityInsights(observations)
	}
}

// detectPatterns analyzes observations for usage patterns.
func (ie *InsightEngine) detectPatterns(observations []*Observation) map[string]*InsightPattern {
	patterns := make(map[string]*InsightPattern)

	// Count app usage
	appCounts := make(map[string]int)
	activityCounts := make(map[string]int)
	domainCounts := make(map[string]int)

	for _, obs := range observations {
		if obs.Context == nil {
			continue
		}
		appCounts[obs.Context.Application]++
		activityCounts[obs.Context.Activity]++
		domainCounts[obs.Context.Domain]++
	}

	// Find dominant patterns
	dominantApp := maxKey(appCounts)
	dominantActivity := maxKey(activityCounts)
	dominantDomain := maxKey(domainCounts)

	// Create pattern for dominant app
	if dominantApp != "" {
		frequentApps := topKeys(appCounts, 5)
		pattern := &InsightPattern{
			DominantApp:      dominantApp,
			DominantActivity: dominantActivity,
			DominantDomain:   dominantDomain,
			FrequentApps:     frequentApps,
			Confidence:       float64(appCounts[dominantApp]) / float64(len(observations)),
		}
		patterns[dominantApp] = pattern
	}

	return patterns
}

// generatePatternInsights creates insights from detected patterns.
func (ie *InsightEngine) generatePatternInsights(patterns map[string]*InsightPattern) {
	for _, pattern := range patterns {
		if pattern.Confidence < ie.config.MinPatternConfidence {
			continue
		}

		// Check for interesting patterns
		if pattern.DominantActivity == "coding" && pattern.DominantDomain == "development" {
			ie.maybeCreateInsight(&Insight{
				Type:        InsightTypePattern,
				Priority:    InsightPriorityLow,
				Title:       "Development Focus Detected",
				Description: fmt.Sprintf("You've been focused on %s development. I can help with code reviews, refactoring, or documentation.", pattern.DominantApp),
				ActionType:  "suggestion",
				Confidence:  pattern.Confidence,
			})
		}
	}
}

// generateProductivityInsights analyzes productivity patterns.
func (ie *InsightEngine) generateProductivityInsights(observations []*Observation) {
	if len(observations) == 0 {
		return
	}

	// Check time distribution
	hourCounts := make(map[int]int)
	for _, obs := range observations {
		hourCounts[obs.Timestamp.Hour()]++
	}

	// Find peak hours
	peakHour := 0
	maxCount := 0
	for hour, count := range hourCounts {
		if count > maxCount {
			peakHour = hour
			maxCount = count
		}
	}

	// If there's a clear peak, suggest insight
	if maxCount > len(observations)/4 {
		ie.maybeCreateInsight(&Insight{
			Type:        InsightTypeProductivity,
			Priority:    InsightPriorityLow,
			Title:       "Peak Productivity Hours",
			Description: fmt.Sprintf("You're most active around %d:00. Consider scheduling focused work during this time.", peakHour),
			ActionType:  "suggestion",
			Confidence:  float64(maxCount) / float64(len(observations)),
		})
	}
}

// maybeCreateInsight creates an insight if cooldown allows.
func (ie *InsightEngine) maybeCreateInsight(insight *Insight) {
	ie.mu.Lock()
	defer ie.mu.Unlock()

	// Check cooldown
	key := fmt.Sprintf("%s:%s", insight.Type, insight.Title)
	if lastTime, exists := ie.lastInsightAt[key]; exists {
		if time.Since(lastTime) < ie.config.InsightCooldown {
			return
		}
	}

	// Create insight
	ie.insightCounter++
	insight.ID = fmt.Sprintf("insight_%d", ie.insightCounter)
	insight.CreatedAt = time.Now()

	// Set expiration
	expiry := time.Now().Add(1 * time.Hour)
	insight.ExpiresAt = &expiry

	// Add to list
	ie.insights = append(ie.insights, insight)
	ie.lastInsightAt[key] = time.Now()

	// Trim if over limit
	if len(ie.insights) > ie.config.MaxActiveInsights {
		ie.insights = ie.insights[len(ie.insights)-ie.config.MaxActiveInsights:]
	}

	// Publish event
	if ie.eventBus != nil {
		ie.eventBus.Publish(bus.NewCortexEyesInsightEvent(
			insight.ID,
			string(insight.Type),
			insight.Title,
			insight.Description,
			insight.Confidence,
		))
	}

	ie.log.Info("[InsightEngine] New insight: %s - %s", insight.Type, insight.Title)
}

// pruneExpiredInsights removes expired insights.
func (ie *InsightEngine) pruneExpiredInsights() {
	ie.mu.Lock()
	defer ie.mu.Unlock()

	now := time.Now()
	active := make([]*Insight, 0, len(ie.insights))

	for _, insight := range ie.insights {
		if insight.ExpiresAt != nil && now.After(*insight.ExpiresAt) {
			continue
		}
		if insight.Dismissed {
			continue
		}
		active = append(active, insight)
	}

	ie.insights = active
}

// GetActiveInsights returns non-dismissed, non-expired insights.
func (ie *InsightEngine) GetActiveInsights() []*Insight {
	ie.mu.RLock()
	defer ie.mu.RUnlock()

	now := time.Now()
	active := make([]*Insight, 0)

	for _, insight := range ie.insights {
		if insight.Dismissed {
			continue
		}
		if insight.ExpiresAt != nil && now.After(*insight.ExpiresAt) {
			continue
		}
		active = append(active, insight)
	}

	return active
}

// GetUnshownInsights returns insights that haven't been shown yet.
func (ie *InsightEngine) GetUnshownInsights() []*Insight {
	ie.mu.RLock()
	defer ie.mu.RUnlock()

	unshown := make([]*Insight, 0)
	now := time.Now()

	for _, insight := range ie.insights {
		if insight.Shown || insight.Dismissed {
			continue
		}
		if insight.ExpiresAt != nil && now.After(*insight.ExpiresAt) {
			continue
		}
		unshown = append(unshown, insight)
	}

	return unshown
}

// MarkInsightShown marks an insight as shown.
func (ie *InsightEngine) MarkInsightShown(id string) {
	ie.mu.Lock()
	defer ie.mu.Unlock()

	for _, insight := range ie.insights {
		if insight.ID == id {
			insight.Shown = true
			return
		}
	}
}

// DismissInsight dismisses an insight.
func (ie *InsightEngine) DismissInsight(id string) {
	ie.mu.Lock()
	defer ie.mu.Unlock()

	for _, insight := range ie.insights {
		if insight.ID == id {
			insight.Dismissed = true
			return
		}
	}
}

// ClearInsights clears all insights.
func (ie *InsightEngine) ClearInsights() {
	ie.mu.Lock()
	defer ie.mu.Unlock()

	ie.insights = make([]*Insight, 0)
}

// Helper functions

func containsError(s string) bool {
	keywords := []string{"error", "fail", "exception", "crash", "panic", "bug"}
	lower := stringToLower(s)
	for _, kw := range keywords {
		if containsStr(lower, kw) {
			return true
		}
	}
	return false
}

func isDocumentation(s string) bool {
	keywords := []string{"doc", "readme", "wiki", "manual", "reference", "guide", "tutorial"}
	lower := stringToLower(s)
	for _, kw := range keywords {
		if containsStr(lower, kw) {
			return true
		}
	}
	return false
}

func isDebugging(s string) bool {
	keywords := []string{"debug", "breakpoint", "inspect", "step", "watch", "trace"}
	lower := stringToLower(s)
	for _, kw := range keywords {
		if containsStr(lower, kw) {
			return true
		}
	}
	return false
}

func stringToLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && findSubstr(s, substr) >= 0
}

func findSubstr(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func maxKey(m map[string]int) string {
	maxVal := 0
	maxKey := ""
	for k, v := range m {
		if v > maxVal {
			maxVal = v
			maxKey = k
		}
	}
	return maxKey
}

func topKeys(m map[string]int, n int) []string {
	type kv struct {
		key   string
		value int
	}

	sorted := make([]kv, 0, len(m))
	for k, v := range m {
		sorted = append(sorted, kv{k, v})
	}

	// Simple bubble sort for small n
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].value > sorted[i].value {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	result := make([]string, 0, n)
	for i := 0; i < len(sorted) && i < n; i++ {
		result = append(result, sorted[i].key)
	}
	return result
}

func formatAppList(apps []string) string {
	if len(apps) == 0 {
		return ""
	}
	if len(apps) == 1 {
		return apps[0]
	}
	if len(apps) == 2 {
		return apps[0] + " and " + apps[1]
	}
	result := ""
	for i, app := range apps[:len(apps)-1] {
		if i > 0 {
			result += ", "
		}
		result += app
	}
	return result + ", and " + apps[len(apps)-1]
}
