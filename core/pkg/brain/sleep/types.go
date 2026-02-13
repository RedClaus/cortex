// Package sleep implements the sleep cycle self-improvement system.
// It allows Cortex to reflect on interactions during idle periods
// and propose personality/behavior improvements.
package sleep

import (
	"errors"
	"time"
)

// Errors
var (
	ErrAlreadySleeping = errors.New("sleep cycle already in progress")
	ErrNoProposal      = errors.New("proposal not found")
	ErrImmutableTrait  = errors.New("cannot modify immutable trait")
	ErrInvalidPath     = errors.New("invalid personality path")
)

// ImprovementMode defines how personality changes are handled.
type ImprovementMode int

const (
	// ImprovementOff disables self-improvement entirely.
	ImprovementOff ImprovementMode = iota
	// ImprovementSupervised requires user approval for all changes.
	ImprovementSupervised
	// ImprovementAuto auto-applies safe changes, asks for risky ones.
	ImprovementAuto
)

// String returns the string representation of the mode.
func (m ImprovementMode) String() string {
	switch m {
	case ImprovementOff:
		return "off"
	case ImprovementSupervised:
		return "supervised"
	case ImprovementAuto:
		return "auto"
	default:
		return "unknown"
	}
}

// ParseImprovementMode parses a string into an ImprovementMode.
func ParseImprovementMode(s string) ImprovementMode {
	switch s {
	case "off":
		return ImprovementOff
	case "supervised":
		return ImprovementSupervised
	case "auto":
		return ImprovementAuto
	default:
		return ImprovementSupervised
	}
}

// RiskLevel indicates the risk of a personality change.
type RiskLevel string

const (
	// RiskSafe represents minimal impact, easily reversible.
	RiskSafe RiskLevel = "safe"
	// RiskModerate represents noticeable impact, reversible.
	RiskModerate RiskLevel = "moderate"
	// RiskSignificant represents major impact, requires caution.
	RiskSignificant RiskLevel = "significant"
)

// WakeReport summarizes what happened during a sleep cycle.
type WakeReport struct {
	SleepDuration        time.Duration
	InteractionsReviewed int
	PatternsFound        int
	Insights             []ReflectionInsight
	Proposals            []PersonalityProposal
	AutoApplied          []PersonalityProposal
	PendingApproval      []PersonalityProposal

	// DMN Worker results (cold-path learning)
	DMNResult *DMNResult `json:"dmn_result,omitempty"`
}

// ConsolidationResult holds the output of Phase 1 (memory consolidation).
type ConsolidationResult struct {
	InteractionCount int
	Patterns         []Pattern
	Emotions         []EmotionSignature
	Outcomes         []InteractionOutcome
	Preferences      []UserPreference
	TimeRange        TimeRange
}

// TimeRange represents a time period.
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// Pattern represents a recurring interaction pattern.
type Pattern struct {
	ID          string
	Type        string // "request_type", "response_style", "topic", "timing"
	Description string
	Frequency   int
	Confidence  float64
	Examples    []string // Interaction IDs that exemplify this pattern
}

// EmotionSignature captures detected emotional context.
type EmotionSignature struct {
	Emotion        string  // "frustrated", "satisfied", "confused", "engaged"
	Intensity      float64 // 0.0 to 1.0
	Context        string  // What triggered this emotion
	InteractionIDs []string
}

// InteractionOutcome tracks success/failure of interactions.
type InteractionOutcome struct {
	InteractionID    string
	Success          bool
	Indicators       []string // What indicated success/failure
	UserFeedback     string   // Explicit feedback if any
	FeedbackPositive bool
}

// UserPreference is an inferred user preference.
type UserPreference struct {
	Category   string // "communication", "code_style", "response_length", etc.
	Preference string // The actual preference
	Confidence float64
	Evidence   []string // Interaction IDs supporting this
}

// ReflectionInsight is an insight from Phase 2 (reflection).
type ReflectionInsight struct {
	ID            string
	Category      string // "strength", "weakness", "opportunity", "pattern"
	Description   string
	Evidence      []string // Interaction IDs that support this
	Confidence    float64
	ActionableFor []string // Which personality traits this might affect
}

// IsActionable returns true if this insight can lead to a proposal.
func (i *ReflectionInsight) IsActionable() bool {
	return len(i.ActionableFor) > 0 || i.Category == "opportunity"
}

// PersonalityProposal is a proposed change from Phase 3.
type PersonalityProposal struct {
	ID          string
	Type        string // "trait_adjustment", "new_pattern", "style_change"
	Description string // Human-readable explanation
	Impact      string // How this will change behavior
	RiskLevel   RiskLevel
	Changes     []PersonalityChange
	Evidence    []string // Supporting interaction IDs
	Confidence  float64
	Reversible  bool
	CreatedAt   time.Time
}

// PersonalityChange is a specific change to the personality file.
type PersonalityChange struct {
	Path     string // e.g., "traits.warmth" or "learned_patterns[3]"
	OldValue interface{}
	NewValue interface{}
	Reason   string
}

// Interaction represents a single user interaction for analysis.
// This is used by the memory store interface.
type Interaction struct {
	ID               string
	Type             string // "question", "task", "debug", "creative", etc.
	UserMessage      string
	AssistantMessage string
	Summary          string
	Timestamp        time.Time
	Feedback         string
	FeedbackPositive bool
	TaskCompleted    bool
	FollowUpCount    int
	UserCorrected    bool
}

// MemoryStore is the interface for accessing interaction history.
type MemoryStore interface {
	// GetInteractionsSince returns all interactions since the given time.
	GetInteractionsSince(since time.Time) ([]Interaction, error)
}
