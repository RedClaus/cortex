// Package vision provides unified interfaces for vision/image analysis providers.
package vision

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// PrivacyFilter controls what CortexEyes can see.
// It provides app exclusion, window filtering, and sensitive content detection.
//
// CR-023: CortexEyes - Screen Awareness & Contextual Learning
type PrivacyFilter struct {
	config *PrivacyConfig
	mu     sync.RWMutex

	// Compiled regex patterns for sensitive content
	sensitivePatterns []*regexp.Regexp

	// State
	paused       bool
	pausedUntil  time.Time
	lastActivity time.Time
}

// PrivacyConfig controls what CortexEyes can see.
type PrivacyConfig struct {
	Enabled           bool          `yaml:"enabled" json:"enabled"`                         // Master switch
	ExcludedApps      []string      `yaml:"excluded_apps" json:"excluded_apps"`             // Apps to never capture
	ExcludedWindows   []string      `yaml:"excluded_windows" json:"excluded_windows"`       // Window title patterns to exclude
	SensitivePatterns []string      `yaml:"sensitive_patterns" json:"sensitive_patterns"`   // Regex patterns for sensitive content
	AutoPauseOnIdle   time.Duration `yaml:"auto_pause_idle" json:"auto_pause_idle"`         // Pause after N minutes of no activity
	MaxRetentionDays  int           `yaml:"max_retention_days" json:"max_retention_days"`   // Auto-delete observations after N days
	RequireConsent    bool          `yaml:"require_consent" json:"require_consent"`         // Prompt before first capture
	AllowedHours      *TimeRange    `yaml:"allowed_hours" json:"allowed_hours"`             // Only capture during these hours
}

// TimeRange represents a time range during which capture is allowed.
type TimeRange struct {
	Start string `yaml:"start" json:"start"` // "08:00"
	End   string `yaml:"end" json:"end"`     // "22:00"
}

// DefaultPrivacyConfig returns a privacy config with sensible defaults.
func DefaultPrivacyConfig() *PrivacyConfig {
	return &PrivacyConfig{
		Enabled: true,
		ExcludedApps: []string{
			"1Password",
			"Keychain Access",
			"System Preferences",
			"System Settings",
			"Security & Privacy",
			"Bitwarden",
			"LastPass",
			"KeePassXC",
		},
		ExcludedWindows: []string{
			"*password*",
			"*credential*",
			"*secret*",
			"*private*",
			"*login*",
			"*signin*",
			"*sign in*",
		},
		SensitivePatterns: []string{
			`\b\d{16}\b`,                        // Credit card numbers
			`\b\d{3}-\d{2}-\d{4}\b`,             // SSN
			`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`, // Email (optional)
		},
		AutoPauseOnIdle:  5 * time.Minute,
		MaxRetentionDays: 30,
		RequireConsent:   true,
		AllowedHours: &TimeRange{
			Start: "08:00",
			End:   "22:00",
		},
	}
}

// NewPrivacyFilter creates a new privacy filter.
func NewPrivacyFilter(config *PrivacyConfig) *PrivacyFilter {
	if config == nil {
		config = DefaultPrivacyConfig()
	}

	pf := &PrivacyFilter{
		config:       config,
		lastActivity: time.Now(),
	}

	// Compile regex patterns
	pf.compileSensitivePatterns()

	return pf
}

// compileSensitivePatterns compiles the sensitive content patterns.
func (pf *PrivacyFilter) compileSensitivePatterns() {
	pf.sensitivePatterns = make([]*regexp.Regexp, 0, len(pf.config.SensitivePatterns))

	for _, pattern := range pf.config.SensitivePatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			pf.sensitivePatterns = append(pf.sensitivePatterns, re)
		}
	}
}

// ShouldCapture returns whether capture is allowed for the given app/window.
func (pf *PrivacyFilter) ShouldCapture(appName, windowTitle string) bool {
	pf.mu.RLock()
	defer pf.mu.RUnlock()

	// Check if disabled
	if !pf.config.Enabled {
		return false
	}

	// Check if paused
	if pf.paused {
		if time.Now().Before(pf.pausedUntil) {
			return false
		}
		// Pause expired
		pf.paused = false
	}

	// Check excluded apps
	appLower := strings.ToLower(appName)
	for _, excluded := range pf.config.ExcludedApps {
		if strings.Contains(appLower, strings.ToLower(excluded)) {
			return false
		}
	}

	// Check excluded windows
	windowLower := strings.ToLower(windowTitle)
	for _, pattern := range pf.config.ExcludedWindows {
		if pf.matchWildcard(windowLower, strings.ToLower(pattern)) {
			return false
		}
	}

	// Check allowed hours
	if pf.config.AllowedHours != nil {
		if !pf.isWithinAllowedHours() {
			return false
		}
	}

	return true
}

// matchWildcard performs simple wildcard matching (* at start/end).
func (pf *PrivacyFilter) matchWildcard(text, pattern string) bool {
	// Handle * wildcards at start and end
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		// *pattern* - contains
		middle := strings.Trim(pattern, "*")
		return strings.Contains(text, middle)
	} else if strings.HasPrefix(pattern, "*") {
		// *pattern - ends with
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(text, suffix)
	} else if strings.HasSuffix(pattern, "*") {
		// pattern* - starts with
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(text, prefix)
	}

	// Exact match
	return text == pattern
}

// isWithinAllowedHours checks if current time is within allowed hours.
func (pf *PrivacyFilter) isWithinAllowedHours() bool {
	if pf.config.AllowedHours == nil {
		return true
	}

	now := time.Now()
	startH, startM := pf.parseTime(pf.config.AllowedHours.Start)
	endH, endM := pf.parseTime(pf.config.AllowedHours.End)

	currentMinutes := now.Hour()*60 + now.Minute()
	startMinutes := startH*60 + startM
	endMinutes := endH*60 + endM

	return currentMinutes >= startMinutes && currentMinutes <= endMinutes
}

// parseTime parses a time string like "08:00" into hour and minute.
func (pf *PrivacyFilter) parseTime(timeStr string) (hour, minute int) {
	parts := strings.Split(timeStr, ":")
	if len(parts) >= 1 {
		fmt.Sscanf(parts[0], "%d", &hour)
	}
	if len(parts) >= 2 {
		fmt.Sscanf(parts[1], "%d", &minute)
	}
	return hour, minute
}

// SanitizeText removes sensitive content from text.
func (pf *PrivacyFilter) SanitizeText(text string) string {
	pf.mu.RLock()
	patterns := pf.sensitivePatterns
	pf.mu.RUnlock()

	for _, pattern := range patterns {
		text = pattern.ReplaceAllString(text, "[REDACTED]")
	}

	return text
}

// SanitizeObservation removes sensitive content from an observation.
func (pf *PrivacyFilter) SanitizeObservation(obs *Observation) *Observation {
	if obs == nil {
		return nil
	}

	// Create a copy to avoid modifying the original
	sanitized := &Observation{
		ID:        obs.ID,
		SessionID: obs.SessionID,
		Timestamp: obs.Timestamp,
		CreatedAt: obs.CreatedAt,
	}

	// Sanitize summary
	sanitized.Summary = pf.SanitizeText(obs.Summary)

	// Sanitize insights
	if obs.Insights != nil {
		sanitized.Insights = make([]string, len(obs.Insights))
		for i, insight := range obs.Insights {
			sanitized.Insights[i] = pf.SanitizeText(insight)
		}
	}

	// Sanitize context
	if obs.Context != nil {
		sanitized.Context = &UserContext{
			Activity:      obs.Context.Activity,
			Application:   obs.Context.Application,
			Domain:        obs.Context.Domain,
			ContentType:   obs.Context.ContentType,
			FocusArea:     pf.SanitizeText(obs.Context.FocusArea),
			Confidence:    obs.Context.Confidence,
			ExtractedText: pf.SanitizeText(obs.Context.ExtractedText),
			Timestamp:     obs.Context.Timestamp,
			AnalysisMs:    obs.Context.AnalysisMs,
		}
	}

	return sanitized
}

// Pause temporarily pauses capture.
func (pf *PrivacyFilter) Pause(duration time.Duration) {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	pf.paused = true
	pf.pausedUntil = time.Now().Add(duration)
}

// Resume resumes capture after pause.
func (pf *PrivacyFilter) Resume() {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	pf.paused = false
	pf.pausedUntil = time.Time{}
}

// IsPaused returns whether capture is currently paused.
func (pf *PrivacyFilter) IsPaused() bool {
	pf.mu.RLock()
	defer pf.mu.RUnlock()

	if !pf.paused {
		return false
	}

	if time.Now().After(pf.pausedUntil) {
		return false
	}

	return true
}

// RecordActivity updates the last activity timestamp.
func (pf *PrivacyFilter) RecordActivity() {
	pf.mu.Lock()
	defer pf.mu.Unlock()
	pf.lastActivity = time.Now()
}

// IsIdle returns whether the system has been idle longer than AutoPauseOnIdle.
func (pf *PrivacyFilter) IsIdle() bool {
	pf.mu.RLock()
	defer pf.mu.RUnlock()

	if pf.config.AutoPauseOnIdle <= 0 {
		return false
	}

	return time.Since(pf.lastActivity) >= pf.config.AutoPauseOnIdle
}

// AddExcludedApp adds an app to the exclusion list.
func (pf *PrivacyFilter) AddExcludedApp(appName string) {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	// Check if already excluded
	for _, app := range pf.config.ExcludedApps {
		if strings.EqualFold(app, appName) {
			return
		}
	}

	pf.config.ExcludedApps = append(pf.config.ExcludedApps, appName)
}

// RemoveExcludedApp removes an app from the exclusion list.
func (pf *PrivacyFilter) RemoveExcludedApp(appName string) bool {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	for i, app := range pf.config.ExcludedApps {
		if strings.EqualFold(app, appName) {
			pf.config.ExcludedApps = append(pf.config.ExcludedApps[:i], pf.config.ExcludedApps[i+1:]...)
			return true
		}
	}

	return false
}

// GetExcludedApps returns the list of excluded apps.
func (pf *PrivacyFilter) GetExcludedApps() []string {
	pf.mu.RLock()
	defer pf.mu.RUnlock()

	apps := make([]string, len(pf.config.ExcludedApps))
	copy(apps, pf.config.ExcludedApps)
	return apps
}

// SetEnabled enables or disables the privacy filter.
func (pf *PrivacyFilter) SetEnabled(enabled bool) {
	pf.mu.Lock()
	defer pf.mu.Unlock()
	pf.config.Enabled = enabled
}

// IsEnabled returns whether the privacy filter is enabled.
func (pf *PrivacyFilter) IsEnabled() bool {
	pf.mu.RLock()
	defer pf.mu.RUnlock()
	return pf.config.Enabled
}

// Status returns the current privacy filter status.
type PrivacyStatus struct {
	Enabled         bool      `json:"enabled"`
	Paused          bool      `json:"paused"`
	PausedUntil     time.Time `json:"paused_until,omitempty"`
	ExcludedApps    int       `json:"excluded_apps"`
	ExcludedWindows int       `json:"excluded_windows"`
	IsIdle          bool      `json:"is_idle"`
	WithinHours     bool      `json:"within_allowed_hours"`
}

func (pf *PrivacyFilter) Status() PrivacyStatus {
	pf.mu.RLock()
	defer pf.mu.RUnlock()

	return PrivacyStatus{
		Enabled:         pf.config.Enabled,
		Paused:          pf.paused && time.Now().Before(pf.pausedUntil),
		PausedUntil:     pf.pausedUntil,
		ExcludedApps:    len(pf.config.ExcludedApps),
		ExcludedWindows: len(pf.config.ExcludedWindows),
		IsIdle:          pf.config.AutoPauseOnIdle > 0 && time.Since(pf.lastActivity) >= pf.config.AutoPauseOnIdle,
		WithinHours:     pf.isWithinAllowedHours(),
	}
}

