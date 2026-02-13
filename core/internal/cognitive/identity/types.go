// Package identity provides identity persistence and drift detection for System 3 meta-cognition.
// It implements the Creed system from the Sophia paper (arXiv:2512.18202) for maintaining
// consistent AI identity across long conversations.
package identity

import (
	"time"
)

// Config configures the identity guardian system.
type Config struct {
	// Enabled controls whether identity checking is active
	Enabled bool

	// DriftThreshold is the maximum allowed drift score (0-1) before triggering repair
	DriftThreshold float64

	// CheckInterval is the number of responses between drift checks
	CheckInterval int

	// AutoRepair enables automatic drift correction
	AutoRepair bool

	// WindowSize is the number of recent responses to analyze for drift
	WindowSize int
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Enabled:        true,
		DriftThreshold: 0.3,
		CheckInterval:  100,
		AutoRepair:     false,
		WindowSize:     10,
	}
}

// Creed represents the immutable identity anchors for a persona.
type Creed struct {
	// Statements are the core identity anchors (typically 5 sentences)
	Statements []string `json:"statements"`

	// Embeddings are pre-computed embeddings for each statement
	// These are computed once at startup and never change
	Embeddings [][]float32 `json:"embeddings,omitempty"`

	// CombinedEmbedding is the averaged embedding of all statements
	CombinedEmbedding []float32 `json:"combined_embedding,omitempty"`

	// Version tracks creed updates
	Version string `json:"version"`

	// CreatedAt is when this creed was established
	CreatedAt time.Time `json:"created_at"`
}

// DriftEvent records a drift detection event.
type DriftEvent struct {
	// Timestamp of the drift check
	Timestamp time.Time `json:"timestamp"`

	// DriftScore is the measured drift (0 = no drift, 1 = complete drift)
	DriftScore float64 `json:"drift_score"`

	// ResponseCount is the total responses analyzed
	ResponseCount int `json:"response_count"`

	// Triggered indicates if drift exceeded threshold
	Triggered bool `json:"triggered"`

	// MostDrifted is the creed statement with highest drift
	MostDrifted string `json:"most_drifted,omitempty"`

	// RepairApplied indicates if auto-repair was executed
	RepairApplied bool `json:"repair_applied"`
}

// DriftAnalysis contains detailed drift analysis results.
type DriftAnalysis struct {
	// OverallDrift is the aggregate drift score (0-1)
	OverallDrift float64 `json:"overall_drift"`

	// PerStatementDrift maps each creed statement to its drift score
	PerStatementDrift map[string]float64 `json:"per_statement_drift"`

	// RecentResponseSimilarity is the avg similarity of recent responses to creed
	RecentResponseSimilarity float64 `json:"recent_response_similarity"`

	// DriftTrend indicates drift direction: "stable", "increasing", "decreasing"
	DriftTrend string `json:"drift_trend"`

	// Confidence in the drift measurement (0-1)
	Confidence float64 `json:"confidence"`

	// Duration of the analysis
	Duration time.Duration `json:"duration"`
}

// RepairAction describes a corrective action for drift.
type RepairAction struct {
	// Type of repair: "reinforce", "anchor", "reset"
	Type string `json:"type"`

	// Statement is the creed statement to reinforce
	Statement string `json:"statement"`

	// Injection is text to prepend to next response
	Injection string `json:"injection"`

	// Priority from 0 (low) to 1 (critical)
	Priority float64 `json:"priority"`
}

// RepairPlan contains the repair strategy for detected drift.
type RepairPlan struct {
	// Actions to apply in order
	Actions []RepairAction `json:"actions"`

	// Severity of the drift: "low", "medium", "high", "critical"
	Severity string `json:"severity"`

	// RequiresApproval indicates if user must approve
	RequiresApproval bool `json:"requires_approval"`

	// Reason explains the repair plan
	Reason string `json:"reason"`
}

// ValidationResult is the result of validating a response against creed.
type ValidationResult struct {
	// Valid indicates if response is consistent with creed
	Valid bool `json:"valid"`

	// Similarity score (0-1, higher = more aligned)
	Similarity float64 `json:"similarity"`

	// ViolatedStatements lists creed statements that may be violated
	ViolatedStatements []string `json:"violated_statements,omitempty"`

	// Suggestions for alignment improvement
	Suggestions []string `json:"suggestions,omitempty"`

	// Duration of the validation
	Duration time.Duration `json:"duration"`
}

// IdentityStats contains statistics about identity management.
type IdentityStats struct {
	// Enabled indicates if identity checking is active
	Enabled bool `json:"enabled"`

	// TotalChecks is the number of drift checks performed
	TotalChecks int64 `json:"total_checks"`

	// DriftEvents is the number of drift threshold violations
	DriftEvents int64 `json:"drift_events"`

	// RepairsApplied is the number of auto-repairs executed
	RepairsApplied int64 `json:"repairs_applied"`

	// AverageDrift is the running average drift score
	AverageDrift float64 `json:"average_drift"`

	// LastCheckTime is when drift was last checked
	LastCheckTime time.Time `json:"last_check_time"`

	// ResponsesSinceCheck is responses since last drift check
	ResponsesSinceCheck int `json:"responses_since_check"`
}

// Embedder is the interface for computing embeddings.
// This matches the existing memory.Embedder interface.
type Embedder interface {
	Embed(text string) ([]float32, error)
	EmbedBatch(texts []string) ([][]float32, error)
}
