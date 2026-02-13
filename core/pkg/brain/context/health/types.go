// Package health provides context health monitoring for the AttentionBlackboard.
// It detects degradation patterns and triggers remediation actions.
//
// Currently implements only LostInMiddle detection (per YAGNI principle).
// Other patterns (Poisoning, Distraction, Confusion, Clash) will be added
// when proven necessary in production.
package health

import (
	"time"

	"github.com/normanking/cortex/pkg/brain/context"
)

// DegradationPattern identifies types of context quality degradation.
type DegradationPattern string

const (
	// PatternLostInMiddle indicates important information is buried in the middle
	// of the context where LLM attention is weakest.
	// Based on "Lost in the Middle" research (Liu et al., 2023).
	PatternLostInMiddle DegradationPattern = "lost_in_middle"

	// Future patterns (implement when detected in production):
	// PatternPoisoning   DegradationPattern = "poisoning"   // Errors compound
	// PatternDistraction DegradationPattern = "distraction" // Irrelevant overwhelms relevant
	// PatternConfusion   DegradationPattern = "confusion"   // Can't determine which context applies
	// PatternClash       DegradationPattern = "clash"       // Accumulated info conflicts
)

// HealthStatus represents the overall health of the context.
type HealthStatus string

const (
	StatusHealthy   HealthStatus = "healthy"
	StatusDegraded  HealthStatus = "degraded"
	StatusCritical  HealthStatus = "critical"
)

// HealthReport contains the results of a health check.
type HealthReport struct {
	// Status is the overall health status
	Status HealthStatus `json:"status"`

	// Score is the health score (0-100, higher is better)
	Score int `json:"score"`

	// Patterns contains detected degradation patterns
	Patterns []DetectedPattern `json:"patterns,omitempty"`

	// Recommendations for remediation
	Recommendations []string `json:"recommendations,omitempty"`

	// Stats from the blackboard
	Stats context.BlackboardStats `json:"stats"`

	// Timestamp of the check
	Timestamp time.Time `json:"timestamp"`

	// Duration of the health check
	Duration time.Duration `json:"duration"`
}

// DetectedPattern represents a detected degradation pattern.
type DetectedPattern struct {
	// Pattern type
	Pattern DegradationPattern `json:"pattern"`

	// Severity (0.0-1.0, higher is worse)
	Severity float64 `json:"severity"`

	// Description of the issue
	Description string `json:"description"`

	// AffectedItems IDs of items affected by this pattern
	AffectedItems []string `json:"affected_items,omitempty"`
}

// HealthConfig configures health monitoring behavior.
type HealthConfig struct {
	// DegradationThreshold is the score below which status becomes "degraded" (default: 70)
	DegradationThreshold int `yaml:"degradation_threshold" json:"degradation_threshold"`

	// CriticalThreshold is the score below which status becomes "critical" (default: 40)
	CriticalThreshold int `yaml:"critical_threshold" json:"critical_threshold"`

	// LostInMiddleConfig configures the LostInMiddle detector
	LostInMiddle LostInMiddleConfig `yaml:"lost_in_middle" json:"lost_in_middle"`
}

// LostInMiddleConfig configures LostInMiddle detection.
type LostInMiddleConfig struct {
	// Enabled turns detection on/off
	Enabled bool `yaml:"enabled" json:"enabled"`

	// HighPriorityThreshold - items above this priority shouldn't be in middle (default: 0.7)
	HighPriorityThreshold float64 `yaml:"high_priority_threshold" json:"high_priority_threshold"`

	// MaxMiddleRatio - max ratio of supporting zone to total (default: 0.6)
	MaxMiddleRatio float64 `yaml:"max_middle_ratio" json:"max_middle_ratio"`
}

// DefaultHealthConfig returns default health configuration.
func DefaultHealthConfig() HealthConfig {
	return HealthConfig{
		DegradationThreshold: 70,
		CriticalThreshold:    40,
		LostInMiddle: LostInMiddleConfig{
			Enabled:               true,
			HighPriorityThreshold: 0.7,
			MaxMiddleRatio:        0.6,
		},
	}
}

// TriggerType identifies what triggered a health check.
type TriggerType string

const (
	// TriggerBudget50 fires when 50% of budget is used
	TriggerBudget50 TriggerType = "budget_50"

	// TriggerBudget75 fires when 75% of budget is used
	TriggerBudget75 TriggerType = "budget_75"

	// TriggerBudget90 fires when 90% of budget is used
	TriggerBudget90 TriggerType = "budget_90"

	// TriggerPhaseComplete fires when a processing phase completes
	TriggerPhaseComplete TriggerType = "phase_complete"

	// TriggerCompactionComplete fires after compaction
	TriggerCompactionComplete TriggerType = "compaction_complete"

	// TriggerManual fires from explicit request
	TriggerManual TriggerType = "manual"
)

// TriggerConfig configures event-driven health check triggers.
type TriggerConfig struct {
	// BudgetThresholds are utilization percentages that trigger checks
	BudgetThresholds []float64 `yaml:"budget_thresholds" json:"budget_thresholds"`

	// OnPhaseComplete triggers check after each phase
	OnPhaseComplete bool `yaml:"on_phase_complete" json:"on_phase_complete"`

	// OnCompactionComplete triggers check after compaction
	OnCompactionComplete bool `yaml:"on_compaction_complete" json:"on_compaction_complete"`
}

// DefaultTriggerConfig returns default trigger configuration.
func DefaultTriggerConfig() TriggerConfig {
	return TriggerConfig{
		BudgetThresholds:     []float64{0.50, 0.75, 0.90},
		OnPhaseComplete:      true,
		OnCompactionComplete: true,
	}
}
