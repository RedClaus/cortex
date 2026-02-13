// Package eval provides conversation logging and model capability assessment.
package eval

import (
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// ROUTING OUTCOME
// ═══════════════════════════════════════════════════════════════════════════════

// RoutingOutcome captures the result of a routing decision for learning.
type RoutingOutcome struct {
	Lane           string  `json:"lane"`            // "fast" or "smart"
	Reason         string  `json:"reason"`          // Why this lane was chosen
	ModelSelected  string  `json:"model_selected"`  // Final model used
	Forced         bool    `json:"forced"`          // Was routing forced by constraint?
	Constraint     string  `json:"constraint"`      // e.g., "vision", "context_overflow"
	OutcomeSuccess bool    `json:"outcome_success"` // Did the response succeed?
	OutcomeScore   float64 `json:"outcome_score"`   // 0-1 quality score
	LatencyMs      int     `json:"latency_ms"`      // Response latency
}

// ═══════════════════════════════════════════════════════════════════════════════
// CONVERSATION LOG
// ═══════════════════════════════════════════════════════════════════════════════

// ConversationLog represents a single LLM interaction.
type ConversationLog struct {
	ID              int64      `json:"id"`
	RequestID       string     `json:"request_id"`
	SessionID       string     `json:"session_id,omitempty"`
	ParentRequestID string     `json:"parent_request_id,omitempty"`

	// Model info
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	ModelTier string `json:"model_tier,omitempty"`

	// Request
	Prompt        string `json:"prompt"`
	SystemPrompt  string `json:"system_prompt,omitempty"`
	ContextTokens int    `json:"context_tokens,omitempty"`

	// Response
	Response         string `json:"response,omitempty"`
	CompletionTokens int    `json:"completion_tokens,omitempty"`
	TotalTokens      int    `json:"total_tokens,omitempty"`

	// Performance
	DurationMs         int `json:"duration_ms"`
	TimeToFirstTokenMs int `json:"time_to_first_token_ms,omitempty"`

	// Classification
	TaskType        string `json:"task_type,omitempty"`
	ComplexityScore int    `json:"complexity_score,omitempty"`

	// Outcome
	Success      bool   `json:"success"`
	ErrorCode    string `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`

	// Issue flags
	HadTimeout     bool `json:"had_timeout"`
	HadRepetition  bool `json:"had_repetition"`
	HadToolFailure bool `json:"had_tool_failure"`
	HadTruncation  bool `json:"had_truncation"`
	HadJSONError   bool `json:"had_json_error"`

	// Routing decision tracking (for RoamPal learning)
	RoutingLane       string  `json:"routing_lane,omitempty"`       // "fast" or "smart"
	RoutingReason     string  `json:"routing_reason,omitempty"`     // Why this lane was chosen
	RoutingForced     bool    `json:"routing_forced"`               // Was routing forced by constraint?
	RoutingConstraint string  `json:"routing_constraint,omitempty"` // e.g., "vision", "context_overflow"
	OutcomeScore      float64 `json:"outcome_score,omitempty"`      // 0-1 quality score for learning

	// Assessment
	CapabilityScore    float64 `json:"capability_score,omitempty"`
	RecommendedUpgrade string  `json:"recommended_upgrade,omitempty"`
	AssessmentReason   string  `json:"assessment_reason,omitempty"`

	// Timestamps
	CreatedAt  time.Time  `json:"created_at"`
	AssessedAt *time.Time `json:"assessed_at,omitempty"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// ASSESSMENT
// ═══════════════════════════════════════════════════════════════════════════════

// Assessment contains the result of capability assessment.
type Assessment struct {
	CapabilityScore    float64 `json:"capability_score"` // 0-100
	Issues             []Issue `json:"issues"`
	RecommendedUpgrade string  `json:"recommended_upgrade,omitempty"`
	UpgradeReason      string  `json:"upgrade_reason,omitempty"`
	Confidence         float64 `json:"confidence"` // 0-1
}

// HasIssues returns true if any issues were detected.
func (a *Assessment) HasIssues() bool {
	return len(a.Issues) > 0
}

// NeedsUpgrade returns true if a model upgrade is recommended.
func (a *Assessment) NeedsUpgrade() bool {
	return a.RecommendedUpgrade != ""
}

// Issue represents a detected problem.
type Issue struct {
	Type        IssueType `json:"type"`
	Severity    Severity  `json:"severity"`
	Description string    `json:"description"`
	Evidence    string    `json:"evidence,omitempty"`
}

// IssueType categorizes the type of issue detected.
type IssueType string

const (
	IssueTimeout     IssueType = "timeout"
	IssueRepetition  IssueType = "repetition"
	IssueToolFailure IssueType = "tool_failure"
	IssueTruncation  IssueType = "truncation"
	IssueJSONError   IssueType = "json_error"
)

// Severity indicates how serious an issue is.
type Severity string

const (
	SeverityLow    Severity = "low"
	SeverityMedium Severity = "medium"
	SeverityHigh   Severity = "high"
)

// ═══════════════════════════════════════════════════════════════════════════════
// RECOMMENDATION
// ═══════════════════════════════════════════════════════════════════════════════

// Recommendation contains model upgrade suggestion.
type Recommendation struct {
	CurrentModel        string    `json:"current_model"`
	CurrentProvider     string    `json:"current_provider"`
	CurrentTier         ModelTier `json:"current_tier"`
	RecommendedModel    string    `json:"recommended_model"`
	RecommendedProvider string    `json:"recommended_provider"`
	RecommendedTier     ModelTier `json:"recommended_tier"`
	Reason              string    `json:"reason"`
	Confidence          float64   `json:"confidence"`
	Issues              []Issue   `json:"issues"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// LOG REQUEST/RESPONSE
// ═══════════════════════════════════════════════════════════════════════════════

// LogRequest contains request details for logging.
type LogRequest struct {
	SessionID       string
	ParentRequestID string
	Provider        string
	Model           string
	Prompt          string
	SystemPrompt    string
	TaskType        string
	ComplexityScore int
}

// LogResponse contains response details for logging.
type LogResponse struct {
	Response         string
	ContextTokens    int
	CompletionTokens int
	DurationMs       int
	Success          bool
	ErrorCode        string
	ErrorMessage     string
}

// ═══════════════════════════════════════════════════════════════════════════════
// MODEL METRICS
// ═══════════════════════════════════════════════════════════════════════════════

// ModelMetrics contains aggregated per-model performance data.
type ModelMetrics struct {
	ID        int64  `json:"id"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	ModelTier string `json:"model_tier"`
	Date      string `json:"date"` // YYYY-MM-DD

	// Counts
	TotalRequests      int `json:"total_requests"`
	SuccessfulRequests int `json:"successful_requests"`
	FailedRequests     int `json:"failed_requests"`

	// Issue counts
	TimeoutCount     int `json:"timeout_count"`
	RepetitionCount  int `json:"repetition_count"`
	ToolFailureCount int `json:"tool_failure_count"`
	TruncationCount  int `json:"truncation_count"`
	JSONErrorCount   int `json:"json_error_count"`

	// Performance
	TotalDurationMs int     `json:"total_duration_ms"`
	MinDurationMs   int     `json:"min_duration_ms"`
	MaxDurationMs   int     `json:"max_duration_ms"`
	AvgDurationMs   float64 `json:"avg_duration_ms"`

	// Tokens
	TotalPromptTokens     int     `json:"total_prompt_tokens"`
	TotalCompletionTokens int     `json:"total_completion_tokens"`
	AvgTokensPerRequest   float64 `json:"avg_tokens_per_request"`

	// Assessment
	AvgCapabilityScore     float64 `json:"avg_capability_score"`
	UpgradeRecommendations int     `json:"upgrade_recommendations"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// UPGRADE EVENT
// ═══════════════════════════════════════════════════════════════════════════════

// UpgradeEvent records a model upgrade recommendation.
type UpgradeEvent struct {
	ID                int64   `json:"id"`
	ConversationLogID int64   `json:"conversation_log_id,omitempty"`
	RequestID         string  `json:"request_id"`
	FromProvider      string  `json:"from_provider"`
	FromModel         string  `json:"from_model"`
	FromTier          string  `json:"from_tier"`
	ToProvider        string  `json:"to_provider"`
	ToModel           string  `json:"to_model"`
	ToTier            string  `json:"to_tier"`
	Reason            string  `json:"reason"`
	IssueType         string  `json:"issue_type"`
	CapabilityScore   float64 `json:"capability_score"`
	UserAction        string  `json:"user_action"` // accepted, dismissed, ignored, pending
	CreatedAt         time.Time  `json:"created_at"`
	ResolvedAt        *time.Time `json:"resolved_at,omitempty"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// UNIFIED LLM CAPABILITY SCORING
// ═══════════════════════════════════════════════════════════════════════════════

// CapabilityFlags represents boolean model capabilities.
type CapabilityFlags struct {
	Vision          bool `json:"vision"`           // Can process images
	FunctionCalling bool `json:"function_calling"` // Supports tool/function calling
	JSONMode        bool `json:"json_mode"`        // Can output structured JSON
	Streaming       bool `json:"streaming"`        // Supports token streaming
	SystemPrompt    bool `json:"system_prompt"`    // Supports system prompts
}

// PricingInfo contains cost data for cloud models.
type PricingInfo struct {
	InputPer1MTokens  float64 `json:"input_per_1m"`  // USD per 1M input tokens
	OutputPer1MTokens float64 `json:"output_per_1m"` // USD per 1M output tokens
}

// CapabilityScoreSource indicates how the score was determined.
type CapabilityScoreSource string

const (
	ScoreSourceRegistry  CapabilityScoreSource = "registry"  // Lookup from static registry
	ScoreSourceHeuristic CapabilityScoreSource = "heuristic" // Estimated from name/size
)

// UnifiedCapabilityScore represents the unified 0-100 score for any LLM.
type UnifiedCapabilityScore struct {
	Overall     int                   `json:"overall"`     // 0-100 composite score
	Reasoning   int                   `json:"reasoning"`   // Logic/analysis ability
	Coding      int                   `json:"coding"`      // Code generation quality
	Instruction int                   `json:"instruction"` // Following directions
	Speed       int                   `json:"speed"`       // Relative speed (100 = fastest)
	Confidence  float64               `json:"confidence"`  // 0-1 confidence in score
	Source      CapabilityScoreSource `json:"source"`      // How score was determined
}

// ModelCapability combines all information about an LLM model.
type ModelCapability struct {
	ID            string                 `json:"id"`                      // e.g., "anthropic/claude-sonnet-4"
	Provider      string                 `json:"provider"`                // e.g., "anthropic"
	Model         string                 `json:"model"`                   // e.g., "claude-sonnet-4-20250514"
	DisplayName   string                 `json:"display_name"`            // e.g., "Claude Sonnet 4"
	Tier          ModelTier              `json:"tier"`                    // small/medium/large/xl/frontier
	Score         UnifiedCapabilityScore `json:"score"`                   // Unified 0-100 scores
	Capabilities  CapabilityFlags        `json:"capabilities"`            // Boolean flags
	Pricing       *PricingInfo           `json:"pricing,omitempty"`       // nil for local models
	ContextWindow int                    `json:"context_window"`          // Max tokens
	Aliases       []string               `json:"aliases,omitempty"`       // Alternative model names
}

// TierFromScore converts a 0-100 score to a ModelTier.
func TierFromScore(score int) ModelTier {
	switch {
	case score >= 90:
		return TierFrontier
	case score >= 76:
		return TierXL
	case score >= 56:
		return TierLarge
	case score >= 36:
		return TierMedium
	default:
		return TierSmall
	}
}

// ScoreRangeForTier returns the min/max scores for a tier.
func ScoreRangeForTier(tier ModelTier) (min, max int) {
	switch tier {
	case TierSmall:
		return 0, 35
	case TierMedium:
		return 36, 55
	case TierLarge:
		return 56, 75
	case TierXL:
		return 76, 89
	case TierFrontier:
		return 90, 100
	default:
		return 36, 55
	}
}
