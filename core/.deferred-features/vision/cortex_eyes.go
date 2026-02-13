// Package vision provides unified interfaces for vision/image analysis providers.
package vision

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/normanking/cortex/internal/bus"
	"github.com/normanking/cortex/internal/logging"
)

// CortexEyes coordinates screen awareness and contextual learning.
// It watches screen content, detects activity changes, extracts context,
// and stores observations for pattern learning.
//
// CR-023: CortexEyes - Screen Awareness & Contextual Learning
type CortexEyes struct {
	// Core components
	activityDetector  *ActivityDetector
	contextExtractor  *ContextExtractor
	observationMemory *ObservationMemory
	privacyFilter     *PrivacyFilter
	insightEngine     *InsightEngine   // Phase 2: Proactive insights
	screenCapturer    *ScreenCapturer  // Screen capture loop
	webcamCapturer    *WebcamCapturer  // Webcam capture loop (optional)

	// Dependencies
	router   *Router
	eventBus bus.Bus
	log      *logging.Logger

	// State
	mu            sync.RWMutex
	enabled       bool
	watching      bool
	currentCtx    *UserContext
	lastObsID     string
	observedToday int64

	// Vision health tracking
	visionFailures     int       // Consecutive vision failures
	visionDisabledAt   time.Time // When vision was disabled
	visionWarningShown bool      // Whether we've shown the startup warning

	// Configuration
	config *CortexEyesConfig

	// Lifecycle
	stopCh chan struct{}
	wg     sync.WaitGroup
}

const (
	maxVisionFailures      = 5               // Disable vision after N consecutive failures
	visionRetryInterval    = 5 * time.Minute // How long to wait before retrying vision
)

// CortexEyesConfig configures CortexEyes.
type CortexEyesConfig struct {
	// Screen capture settings
	CaptureFPS       float64       // Frames per second to analyze (default: 0.2 = 1 every 5s)
	ChangeThreshold  float64       // 0.0-1.0, how different must frame be (default: 0.3)
	MinInterval      time.Duration // Minimum time between analyses (default: 5s)

	// Webcam capture settings
	Webcam *WebcamConfig // Webcam configuration (nil = disabled)

	// Privacy settings
	Privacy *PrivacyConfig

	// Storage settings
	DBPath           string // Path to SQLite database
	MaxRetentionDays int    // Auto-delete observations after N days (default: 30)

	// Feature flags
	Enabled            bool // Master switch
	EnablePatterns     bool // Enable pattern detection
	EnableInsights     bool // Enable proactive insights (Phase 2)
}

// DefaultCortexEyesConfig returns default configuration.
func DefaultCortexEyesConfig() *CortexEyesConfig {
	return &CortexEyesConfig{
		CaptureFPS:         0.2, // 1 frame every 5 seconds
		ChangeThreshold:    0.3,
		MinInterval:        5 * time.Second,
		Privacy:            DefaultPrivacyConfig(),
		MaxRetentionDays:   30,
		Enabled:            true,
		EnablePatterns:     true,
		EnableInsights:     false, // Phase 2
	}
}

// NewCortexEyes creates a new CortexEyes instance.
func NewCortexEyes(router *Router, db *sql.DB, eventBus bus.Bus, config *CortexEyesConfig) (*CortexEyes, error) {
	if config == nil {
		config = DefaultCortexEyesConfig()
	}

	log := logging.Global()

	// Create activity detector
	activityDetector := NewActivityDetector(&ActivityDetectorConfig{
		ChangeThreshold: config.ChangeThreshold,
		MinInterval:     config.MinInterval,
	})

	// Create context extractor
	contextExtractor := NewContextExtractor(router, &ContextExtractorConfig{
		CacheSize: 10,
	})

	// Create observation memory
	var observationMemory *ObservationMemory
	if db != nil {
		var err error
		observationMemory, err = NewObservationMemory(db, &ObservationMemoryConfig{
			MaxRetention: time.Duration(config.MaxRetentionDays) * 24 * time.Hour,
		})
		if err != nil {
			return nil, fmt.Errorf("create observation memory: %w", err)
		}
	}

	// Create privacy filter
	privacyFilter := NewPrivacyFilter(config.Privacy)

	// Create insight engine if insights are enabled (Phase 2)
	var insightEngine *InsightEngine
	if config.EnableInsights && observationMemory != nil {
		insightEngine = NewInsightEngine(observationMemory, eventBus, nil)
	}

	ce := &CortexEyes{
		activityDetector:  activityDetector,
		contextExtractor:  contextExtractor,
		observationMemory: observationMemory,
		privacyFilter:     privacyFilter,
		insightEngine:     insightEngine,
		router:            router,
		eventBus:          eventBus,
		log:               log,
		enabled:           config.Enabled,
		config:            config,
		stopCh:            make(chan struct{}),
	}

	// Create screen capturer for macOS
	ce.screenCapturer = NewScreenCapturer(ce, config.CaptureFPS)

	// Create webcam capturer if enabled
	if config.Webcam != nil && config.Webcam.Enabled {
		ce.webcamCapturer = NewWebcamCapturer(ce, config.Webcam)
		log.Info("[CortexEyes] Webcam capture enabled (camera=%d, fps=%.2f)", config.Webcam.CameraIndex, config.Webcam.FPS)
	}

	insightsStr := "disabled"
	if insightEngine != nil {
		insightsStr = "enabled"
	}
	webcamStr := "disabled"
	if ce.webcamCapturer != nil {
		webcamStr = "enabled"
	}
	log.Info("[CortexEyes] Initialized (enabled=%v, fps=%.2f, threshold=%.2f, insights=%s, webcam=%s)",
		config.Enabled, config.CaptureFPS, config.ChangeThreshold, insightsStr, webcamStr)

	return ce, nil
}

// Start begins the CortexEyes watching loop.
func (ce *CortexEyes) Start(ctx context.Context) error {
	ce.mu.Lock()
	if ce.watching {
		ce.mu.Unlock()
		return nil
	}
	ce.watching = true
	ce.mu.Unlock()

	ce.log.Info("[CortexEyes] Started watching")

	// Start background cleanup task
	ce.wg.Add(1)
	go ce.cleanupLoop(ctx)

	// Start screen capturer (macOS only)
	if ce.screenCapturer != nil {
		if err := ce.screenCapturer.Start(ctx); err != nil {
			ce.log.Warn("[CortexEyes] Failed to start screen capturer: %v", err)
		} else {
			ce.log.Info("[CortexEyes] Screen capturer started")
		}
	}

	// Start webcam capturer if enabled
	if ce.webcamCapturer != nil {
		if err := ce.webcamCapturer.Start(ctx); err != nil {
			ce.log.Warn("[CortexEyes] Failed to start webcam capturer: %v", err)
		} else {
			ce.log.Info("[CortexEyes] Webcam capturer started")
		}
	}

	// Start insight engine if available
	if ce.insightEngine != nil {
		if err := ce.insightEngine.Start(ctx); err != nil {
			ce.log.Warn("[CortexEyes] Failed to start insight engine: %v", err)
		} else {
			ce.log.Info("[CortexEyes] Insight engine started")
		}
	}

	return nil
}

// Stop stops the CortexEyes watching loop.
func (ce *CortexEyes) Stop() {
	ce.mu.Lock()
	if !ce.watching {
		ce.mu.Unlock()
		return
	}
	ce.watching = false
	ce.mu.Unlock()

	// Stop screen capturer first
	if ce.screenCapturer != nil {
		ce.screenCapturer.Stop()
	}

	// Stop webcam capturer
	if ce.webcamCapturer != nil {
		ce.webcamCapturer.Stop()
	}

	// Stop insight engine
	if ce.insightEngine != nil {
		ce.insightEngine.Stop()
	}

	close(ce.stopCh)
	ce.wg.Wait()

	ce.log.Info("[CortexEyes] Stopped watching")
}

// ProcessFrame processes a screen capture frame.
// This is the main entry point called by the stream handler or external capture.
func (ce *CortexEyes) ProcessFrame(ctx context.Context, frame *Frame, appName, windowTitle string) error {
	ce.mu.RLock()
	if !ce.enabled || !ce.watching {
		ce.mu.RUnlock()
		ce.log.Debug("[CortexEyes] Frame skipped: enabled=%v, watching=%v", ce.enabled, ce.watching)
		return nil
	}
	ce.mu.RUnlock()

	ce.log.Info("[CortexEyes] Processing frame: %d bytes, app=%s, window=%s", len(frame.Data), appName, windowTitle)

	// Check privacy filter
	if !ce.privacyFilter.ShouldCapture(appName, windowTitle) {
		ce.log.Info("[CortexEyes] Frame skipped due to privacy filter (app=%s)", appName)
		return nil
	}

	// Record activity for idle detection
	ce.privacyFilter.RecordActivity()

	// Check for significant change
	change := ce.activityDetector.DetectChange(frame)
	if !change.Changed {
		ce.log.Debug("[CortexEyes] No significant change detected (similarity=%.2f)", change.Similarity)
		return nil
	}

	ce.log.Info("[CortexEyes] Activity change detected: %s (similarity=%.2f)", change.Reason, change.Similarity)

	// Publish activity change event
	if ce.eventBus != nil {
		ce.eventBus.Publish(bus.NewCortexEyesActivityChangeEvent(change.Reason, change.Similarity, appName))
	}

	// Check if vision is temporarily disabled due to failures
	ce.mu.RLock()
	visionDisabled := ce.visionFailures >= maxVisionFailures
	disabledAt := ce.visionDisabledAt
	ce.mu.RUnlock()

	if visionDisabled {
		// Check if enough time has passed to retry
		if time.Since(disabledAt) < visionRetryInterval {
			// Skip silently - already logged the warning
			return nil
		}
		// Reset for retry
		ce.mu.Lock()
		ce.visionFailures = 0
		ce.mu.Unlock()
		ce.log.Info("[CortexEyes] Retrying vision after %v cooldown...", visionRetryInterval)
	}

	// Extract context using vision model
	userCtx, err := ce.contextExtractor.ExtractContext(ctx, frame)
	if err != nil {
		ce.mu.Lock()
		ce.visionFailures++
		failures := ce.visionFailures
		if failures >= maxVisionFailures && !ce.visionWarningShown {
			ce.visionDisabledAt = time.Now()
			ce.visionWarningShown = true
			ce.mu.Unlock()
			// Log actionable error message once
			ce.log.Warn("[CortexEyes] Vision disabled after %d failures. Context extraction paused for %v. Error: %v",
				maxVisionFailures, visionRetryInterval, err)
			ce.log.Warn("[CortexEyes] To enable vision, start MLX vision server: python -m mlx_lm.server --model mlx-community/Qwen2-VL-2B-Instruct-4bit --port 8082")
		} else {
			ce.mu.Unlock()
			if failures < maxVisionFailures {
				ce.log.Debug("[CortexEyes] Vision failure %d/%d: %v", failures, maxVisionFailures, err)
			}
		}
		// Still track activity even without vision context
		ce.trackActivityWithoutVision(appName, windowTitle, change.Reason)
		return nil
	}

	// Success! Reset failure count
	ce.mu.Lock()
	if ce.visionFailures > 0 {
		ce.log.Info("[CortexEyes] Vision restored after %d failures", ce.visionFailures)
	}
	ce.visionFailures = 0
	ce.visionWarningShown = false
	ce.mu.Unlock()

	ce.log.Info("[CortexEyes] Context extracted: activity=%s, domain=%s, focus=%s", userCtx.Activity, userCtx.Domain, userCtx.FocusArea)

	// Update app context from external info if provided
	if appName != "" {
		userCtx.Application = appName
	}

	// Check for context change
	ce.checkContextChange(userCtx, change.Reason)

	// Store observation
	if ce.observationMemory != nil {
		obs := &Observation{
			Context:   userCtx,
			Summary:   fmt.Sprintf("%s in %s: %s", userCtx.Activity, userCtx.Application, userCtx.FocusArea),
			Timestamp: time.Now(),
		}

		// Sanitize before storing
		obs = ce.privacyFilter.SanitizeObservation(obs)

		if err := ce.observationMemory.Store(ctx, obs); err != nil {
			ce.log.Warn("[CortexEyes] Failed to store observation: %v", err)
		} else {
			ce.mu.Lock()
			ce.lastObsID = obs.ID
			ce.observedToday++
			todayCount := ce.observedToday
			ce.mu.Unlock()

			ce.log.Info("[CortexEyes] Observation stored: %s in %s (today: %d)", userCtx.Activity, userCtx.Application, todayCount)

			// Publish observation event
			if ce.eventBus != nil {
				ce.eventBus.Publish(bus.NewCortexEyesObservationEvent(
					obs.ID,
					userCtx.Activity,
					userCtx.Application,
					userCtx.Domain,
					userCtx.FocusArea,
					ce.observationMemory.SessionID(),
					userCtx.Confidence,
				))
			}
		}
	} else {
		ce.log.Warn("[CortexEyes] No observation memory - cannot store observation")
	}

	return nil
}

// trackActivityWithoutVision stores basic activity when vision model is unavailable.
// This allows some learning to continue even without context extraction.
func (ce *CortexEyes) trackActivityWithoutVision(appName, windowTitle, changeReason string) {
	if ce.observationMemory == nil {
		return
	}

	// Create a minimal context from available metadata
	userCtx := &UserContext{
		Application: appName,
		Activity:    inferActivityFromApp(appName),
		FocusArea:   windowTitle,
		Confidence:  0.3, // Low confidence without vision
		Timestamp:   time.Now(),
	}

	obs := &Observation{
		Context:   userCtx,
		Summary:   fmt.Sprintf("%s in %s (vision unavailable)", userCtx.Activity, appName),
		Timestamp: time.Now(),
	}

	// Sanitize before storing
	obs = ce.privacyFilter.SanitizeObservation(obs)

	if err := ce.observationMemory.Store(context.Background(), obs); err != nil {
		ce.log.Debug("[CortexEyes] Failed to store fallback observation: %v", err)
	} else {
		ce.mu.Lock()
		ce.observedToday++
		ce.mu.Unlock()
		ce.log.Debug("[CortexEyes] Stored fallback observation: %s in %s", userCtx.Activity, appName)
	}
}

// inferActivityFromApp guesses activity from app name when vision is unavailable.
func inferActivityFromApp(appName string) string {
	appLower := appName
	if len(appLower) > 0 {
		appLower = string(appLower[0]|32) + appLower[1:] // Lowercase first char
	}

	switch {
	case contains(appLower, "code", "cursor", "vim", "nvim", "xcode", "intellij", "pycharm", "goland"):
		return "coding"
	case contains(appLower, "chrome", "safari", "firefox", "brave", "edge", "arc"):
		return "browsing"
	case contains(appLower, "terminal", "iterm", "warp", "alacritty", "kitty"):
		return "terminal"
	case contains(appLower, "slack", "discord", "teams", "zoom", "meet"):
		return "chatting"
	case contains(appLower, "mail", "outlook", "gmail"):
		return "email"
	case contains(appLower, "notes", "notion", "obsidian", "bear"):
		return "writing"
	case contains(appLower, "figma", "sketch", "photoshop", "illustrator"):
		return "designing"
	case contains(appLower, "spotify", "music", "youtube"):
		return "media"
	default:
		return "unknown"
	}
}

// contains checks if s contains any of the substrings.
func contains(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

// checkContextChange detects and publishes context change events.
func (ce *CortexEyes) checkContextChange(newCtx *UserContext, reason string) {
	ce.mu.Lock()
	prevCtx := ce.currentCtx
	ce.currentCtx = newCtx
	ce.mu.Unlock()

	if prevCtx == nil {
		return
	}

	// Check if context actually changed
	if prevCtx.Activity != newCtx.Activity || prevCtx.Application != newCtx.Application {
		if ce.eventBus != nil {
			ce.eventBus.Publish(bus.NewCortexEyesContextChangedEvent(
				prevCtx.Activity,
				prevCtx.Application,
				newCtx.Activity,
				newCtx.Application,
				reason,
			))
		}
		ce.log.Info("[CortexEyes] Context changed: %s/%s -> %s/%s",
			prevCtx.Activity, prevCtx.Application,
			newCtx.Activity, newCtx.Application)
	}
}

// cleanupLoop periodically cleans up old observations.
func (ce *CortexEyes) cleanupLoop(ctx context.Context) {
	defer ce.wg.Done()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ce.stopCh:
			return
		case <-ticker.C:
			if ce.observationMemory != nil {
				deleted, err := ce.observationMemory.Cleanup(ctx)
				if err != nil {
					ce.log.Warn("[CortexEyes] Cleanup failed: %v", err)
				} else if deleted > 0 {
					ce.log.Info("[CortexEyes] Cleaned up %d old observations", deleted)
				}
			}
		}
	}
}

// GetCurrentContext returns the current user context.
func (ce *CortexEyes) GetCurrentContext() *UserContext {
	ce.mu.RLock()
	defer ce.mu.RUnlock()
	return ce.currentCtx
}

// GetRecentContexts returns recent contexts from the cache.
func (ce *CortexEyes) GetRecentContexts(count int) []*UserContext {
	return ce.contextExtractor.GetRecentContexts(count)
}

// GetPattern returns detected patterns from recent observations.
func (ce *CortexEyes) GetPattern() *ContextPattern {
	if !ce.config.EnablePatterns {
		return nil
	}
	return ce.contextExtractor.DetectPattern()
}

// Pause temporarily pauses watching.
func (ce *CortexEyes) Pause(duration time.Duration) {
	ce.privacyFilter.Pause(duration)
	if ce.eventBus != nil {
		ce.eventBus.Publish(bus.NewCortexEyesPausedEvent(duration, "user_request"))
	}
	ce.log.Info("[CortexEyes] Paused for %v", duration)
}

// Resume resumes watching.
func (ce *CortexEyes) Resume() {
	ce.privacyFilter.Resume()
	if ce.eventBus != nil {
		ce.eventBus.Publish(bus.NewCortexEyesResumedEvent(0))
	}
	ce.log.Info("[CortexEyes] Resumed")
}

// AddExcludedApp adds an app to the privacy exclusion list.
func (ce *CortexEyes) AddExcludedApp(appName string) {
	ce.privacyFilter.AddExcludedApp(appName)
	ce.log.Info("[CortexEyes] Added excluded app: %s", appName)
}

// GetExcludedApps returns the list of excluded apps.
func (ce *CortexEyes) GetExcludedApps() []string {
	return ce.privacyFilter.GetExcludedApps()
}

// SetEnabled enables or disables CortexEyes.
func (ce *CortexEyes) SetEnabled(enabled bool) {
	ce.mu.Lock()
	ce.enabled = enabled
	ce.mu.Unlock()
	ce.log.Info("[CortexEyes] Enabled: %v", enabled)
}

// IsEnabled returns whether CortexEyes is enabled.
func (ce *CortexEyes) IsEnabled() bool {
	ce.mu.RLock()
	defer ce.mu.RUnlock()
	return ce.enabled
}

// IsWatching returns whether CortexEyes is currently watching.
func (ce *CortexEyes) IsWatching() bool {
	ce.mu.RLock()
	defer ce.mu.RUnlock()
	return ce.watching && ce.enabled && !ce.privacyFilter.IsPaused()
}

// VisionStatus represents the health of the vision subsystem.
type VisionStatus struct {
	Available    bool      `json:"available"`     // Vision is working
	Failures     int       `json:"failures"`      // Consecutive failures
	DisabledAt   time.Time `json:"disabled_at"`   // When vision was disabled
	RetryIn      string    `json:"retry_in"`      // Time until retry (if disabled)
}

// Status returns the current CortexEyes status.
type CortexEyesStatus struct {
	Enabled         bool                    `json:"enabled"`
	Watching        bool                    `json:"watching"`
	Privacy         PrivacyStatus           `json:"privacy"`
	Vision          VisionStatus            `json:"vision"`
	CurrentContext  *UserContext            `json:"current_context,omitempty"`
	ObservedToday   int64                   `json:"observed_today"`
	ActivityStats   ActivityDetectorStats   `json:"activity_stats"`
}

func (ce *CortexEyes) Status() CortexEyesStatus {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	visionStatus := VisionStatus{
		Available:  ce.visionFailures < maxVisionFailures,
		Failures:   ce.visionFailures,
		DisabledAt: ce.visionDisabledAt,
	}

	if !visionStatus.Available {
		remaining := visionRetryInterval - time.Since(ce.visionDisabledAt)
		if remaining > 0 {
			visionStatus.RetryIn = remaining.Round(time.Second).String()
		} else {
			visionStatus.RetryIn = "retrying"
		}
	}

	return CortexEyesStatus{
		Enabled:        ce.enabled,
		Watching:       ce.watching && ce.enabled && !ce.privacyFilter.IsPaused(),
		Privacy:        ce.privacyFilter.Status(),
		Vision:         visionStatus,
		CurrentContext: ce.currentCtx,
		ObservedToday:  ce.observedToday,
		ActivityStats:  ce.activityDetector.Stats(),
	}
}

// GetRecentObservations returns recent observations.
func (ce *CortexEyes) GetRecentObservations(ctx context.Context, limit int) ([]*Observation, error) {
	if ce.observationMemory == nil {
		return nil, nil
	}
	return ce.observationMemory.GetRecent(ctx, limit)
}

// GetObservationCount returns the total observation count.
func (ce *CortexEyes) GetObservationCount(ctx context.Context) (int64, error) {
	if ce.observationMemory == nil {
		return 0, nil
	}
	return ce.observationMemory.Count(ctx)
}

// GetTodayCount returns today's observation count.
func (ce *CortexEyes) GetTodayCount(ctx context.Context) (int64, error) {
	if ce.observationMemory == nil {
		return ce.observedToday, nil
	}
	return ce.observationMemory.TodayCount(ctx)
}

// ClearHistory clears observation history.
func (ce *CortexEyes) ClearHistory(ctx context.Context) error {
	if ce.observationMemory == nil {
		return nil
	}
	// Clear by setting retention to 0 and running cleanup
	_, err := ce.observationMemory.Cleanup(ctx)
	return err
}

// ActivityDetector returns the activity detector for testing.
func (ce *CortexEyes) ActivityDetector() *ActivityDetector {
	return ce.activityDetector
}

// ContextExtractor returns the context extractor for testing.
func (ce *CortexEyes) ContextExtractor() *ContextExtractor {
	return ce.contextExtractor
}

// PrivacyFilter returns the privacy filter for testing.
func (ce *CortexEyes) PrivacyFilter() *PrivacyFilter {
	return ce.privacyFilter
}

// GetActiveInsights returns active insights from the insight engine.
func (ce *CortexEyes) GetActiveInsights() []*Insight {
	if ce.insightEngine == nil {
		return nil
	}
	return ce.insightEngine.GetActiveInsights()
}

// GetUnshownInsights returns insights that haven't been shown yet.
func (ce *CortexEyes) GetUnshownInsights() []*Insight {
	if ce.insightEngine == nil {
		return nil
	}
	return ce.insightEngine.GetUnshownInsights()
}

// DismissInsight dismisses an insight by ID.
func (ce *CortexEyes) DismissInsight(id string) {
	if ce.insightEngine != nil {
		ce.insightEngine.DismissInsight(id)
	}
}

// MarkInsightShown marks an insight as shown.
func (ce *CortexEyes) MarkInsightShown(id string) {
	if ce.insightEngine != nil {
		ce.insightEngine.MarkInsightShown(id)
	}
}

// GetDetectedPatterns returns detected usage patterns.
func (ce *CortexEyes) GetDetectedPatterns(ctx context.Context) ([]*StoredPattern, error) {
	if ce.observationMemory == nil {
		return nil, nil
	}
	return ce.observationMemory.DetectPatterns(ctx, 24*time.Hour, 10)
}

// GetStoredPatterns returns stored patterns from the database.
func (ce *CortexEyes) GetStoredPatterns(ctx context.Context) ([]*StoredPattern, error) {
	if ce.observationMemory == nil {
		return nil, nil
	}
	return ce.observationMemory.GetPatterns(ctx, "", "")
}

// InsightsEnabled returns whether insights are enabled.
func (ce *CortexEyes) InsightsEnabled() bool {
	return ce.insightEngine != nil
}
