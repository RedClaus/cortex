package persona

import (
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// ModeManager handles behavioral mode transitions using a finite state machine.
// It analyzes user input and transitions between modes based on keyword patterns.
type ModeManager struct {
	current     *BehavioralMode
	transitions []TransitionRule
	history     []ModeTransition
	maxHistory  int
	mu          sync.RWMutex
}

// TransitionRule defines when and how to switch between modes.
type TransitionRule struct {
	Name     string          // Human-readable rule name
	Pattern  *regexp.Regexp  // Compiled regex pattern
	Keywords []string        // Original keywords (for display)
	FromMode ModeType        // Required current mode ("" = any mode)
	ToMode   ModeType        // Target mode
	Priority int             // Higher = checked first
}

// ModeTransition records a mode change for history/debugging.
type ModeTransition struct {
	From      ModeType  `json:"from"`
	To        ModeType  `json:"to"`
	Trigger   string    `json:"trigger"`
	Timestamp time.Time `json:"timestamp"`
	RuleName  string    `json:"rule_name"`
}

// NewModeManager creates a new ModeManager with default transition rules.
func NewModeManager() *ModeManager {
	mm := &ModeManager{
		current:     DefaultMode(),
		transitions: DefaultTransitions(),
		history:     make([]ModeTransition, 0, 100),
		maxHistory:  100,
	}
	return mm
}

// DefaultTransitions returns the standard mode transition rules.
// Rules are checked in priority order (highest first).
func DefaultTransitions() []TransitionRule {
	rules := []struct {
		name     string
		keywords []string
		from     ModeType
		to       ModeType
		priority int
	}{
		// Reset to normal (highest priority)
		{"reset", []string{"reset mode", "normal mode", "back to normal", "exit mode"},
			"", ModeNormal, 100},

		// Enter debugging mode
		{"debugging", []string{"debug", "debugger", "error", "fix this", "broken", "failing", "crash", "exception", "stack trace"},
			ModeNormal, ModeDebugging, 10},

		// Enter teaching mode
		{"teaching", []string{"explain", "teach me", "help me understand", "what is", "how does", "why does", "learn"},
			ModeNormal, ModeTeaching, 10},

		// Enter pair programming mode
		{"pair", []string{"let's build", "work together", "pair with me", "code with me", "help me implement"},
			ModeNormal, ModePair, 10},

		// Enter code review mode
		{"review", []string{"review this", "code review", "check this code", "review my", "critique"},
			ModeNormal, ModeReview, 10},

		// Allow returning to normal from any mode
		{"finish-debug", []string{"fixed", "solved", "working now", "that fixed it"},
			ModeDebugging, ModeNormal, 5},

		{"finish-teach", []string{"got it", "understand now", "makes sense", "thanks for explaining"},
			ModeTeaching, ModeNormal, 5},

		{"finish-pair", []string{"done pairing", "good enough", "ship it"},
			ModePair, ModeNormal, 5},

		{"finish-review", []string{"review done", "lgtm", "approved"},
			ModeReview, ModeNormal, 5},
	}

	transitions := make([]TransitionRule, 0, len(rules))
	for _, r := range rules {
		// Build regex pattern from keywords
		escapedKeywords := make([]string, len(r.keywords))
		for i, kw := range r.keywords {
			escapedKeywords[i] = regexp.QuoteMeta(kw)
		}
		pattern := `(?i)\b(` + strings.Join(escapedKeywords, "|") + `)\b`

		transitions = append(transitions, TransitionRule{
			Name:     r.name,
			Pattern:  regexp.MustCompile(pattern),
			Keywords: r.keywords,
			FromMode: r.from,
			ToMode:   r.to,
			Priority: r.priority,
		})
	}

	// Sort by priority (descending)
	sort.Slice(transitions, func(i, j int) bool {
		return transitions[i].Priority > transitions[j].Priority
	})

	return transitions
}

// Current returns the current behavioral mode (thread-safe).
func (mm *ModeManager) Current() *BehavioralMode {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	return mm.current
}

// ProcessInput analyzes user input and potentially transitions modes.
// Returns true if a mode transition occurred.
func (mm *ModeManager) ProcessInput(input string) bool {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	currentType := mm.current.Type

	// Check each transition rule
	for _, rule := range mm.transitions {
		// Check if rule applies to current mode
		if rule.FromMode != "" && rule.FromMode != currentType {
			continue
		}

		// Check if pattern matches
		if rule.Pattern.MatchString(input) {
			// Don't transition if already in target mode
			if rule.ToMode == currentType {
				continue
			}

			// Perform transition
			mm.transitionTo(rule.ToMode, input, rule.Name)
			return true
		}
	}

	return false
}

// transitionTo changes the current mode (must be called with lock held).
func (mm *ModeManager) transitionTo(newMode ModeType, trigger, ruleName string) {
	oldMode := mm.current.Type

	// Record transition in history
	transition := ModeTransition{
		From:      oldMode,
		To:        newMode,
		Trigger:   trigger,
		Timestamp: time.Now(),
		RuleName:  ruleName,
	}

	mm.history = append(mm.history, transition)
	if len(mm.history) > mm.maxHistory {
		mm.history = mm.history[1:]
	}

	// Create new mode with appropriate adjustments
	mm.current = &BehavioralMode{
		Type:        newMode,
		Adjustments: getAdjustmentsForMode(newMode),
		EnteredAt:   time.Now(),
		Trigger:     trigger,
	}
}

// getAdjustmentsForMode returns the preset adjustments for each mode type.
func getAdjustmentsForMode(mode ModeType) ModeAdjustments {
	switch mode {
	case ModeDebugging:
		return ModeAdjustments{
			Verbosity:      0.7,
			ThinkingDepth:  0.8,
			CodeVsExplain:  0.3, // More explanation
			CheckpointFreq: 2,   // Check every 2 steps
		}
	case ModeTeaching:
		return ModeAdjustments{
			Verbosity:      0.9, // Very verbose
			ThinkingDepth:  0.6,
			CodeVsExplain:  0.2, // Heavy on explanation
			CheckpointFreq: 3,
		}
	case ModePair:
		return ModeAdjustments{
			Verbosity:      0.4, // Concise
			ThinkingDepth:  0.7,
			CodeVsExplain:  0.7, // More code
			CheckpointFreq: 1,
		}
	case ModeReview:
		return ModeAdjustments{
			Verbosity:      0.8,
			ThinkingDepth:  0.9, // Deep analysis
			CodeVsExplain:  0.1, // Focus on explanation
			CheckpointFreq: 0,   // Review all at once
		}
	default: // ModeNormal
		return ModeAdjustments{
			Verbosity:      0.5,
			ThinkingDepth:  0.5,
			CodeVsExplain:  0.5,
			CheckpointFreq: 0,
		}
	}
}

// SetMode explicitly sets the mode (bypassing pattern matching).
func (mm *ModeManager) SetMode(mode ModeType, trigger string) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	mm.transitionTo(mode, trigger, "explicit")
}

// Reset returns to normal mode.
func (mm *ModeManager) Reset() {
	mm.SetMode(ModeNormal, "reset")
}

// History returns recent mode transitions.
func (mm *ModeManager) History() []ModeTransition {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	result := make([]ModeTransition, len(mm.history))
	copy(result, mm.history)
	return result
}

// AddTransition adds a custom transition rule.
func (mm *ModeManager) AddTransition(rule TransitionRule) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	// Insert in priority order
	inserted := false
	for i, existing := range mm.transitions {
		if rule.Priority > existing.Priority {
			mm.transitions = append(mm.transitions[:i],
				append([]TransitionRule{rule}, mm.transitions[i:]...)...)
			inserted = true
			break
		}
	}
	if !inserted {
		mm.transitions = append(mm.transitions, rule)
	}
}

// Duration returns how long the current mode has been active.
func (mm *ModeManager) Duration() time.Duration {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	return time.Since(mm.current.EnteredAt)
}

// IsInMode checks if the manager is currently in the specified mode.
func (mm *ModeManager) IsInMode(mode ModeType) bool {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	return mm.current.Type == mode
}

// ModePresets provides access to mode adjustment presets.
var ModePresets = map[ModeType]ModeAdjustments{
	ModeNormal:    getAdjustmentsForMode(ModeNormal),
	ModeDebugging: getAdjustmentsForMode(ModeDebugging),
	ModeTeaching:  getAdjustmentsForMode(ModeTeaching),
	ModePair:      getAdjustmentsForMode(ModePair),
	ModeReview:    getAdjustmentsForMode(ModeReview),
}
