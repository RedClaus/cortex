package brain

import (
	"context"
	"time"
)

// ThinkingStrategy defines how to process a request.
type ThinkingStrategy struct {
	Name        string           `json:"name"`
	Phases      []ExecutionPhase `json:"phases"`
	ComputeTier ComputeTier      `json:"compute_tier"`
}

// ExecutionPhase represents a single phase in a thinking strategy.
type ExecutionPhase struct {
	Name      string   `json:"name"`
	Lobes     []LobeID `json:"lobes"`
	Parallel  bool     `json:"parallel"`
	TimeoutMS int      `json:"timeout_ms"`
	CanReplan bool     `json:"can_replan"`
}

// Lobe is the interface all cognitive modules must implement.
// A Lobe is a specialized processing unit within the brain responsible for
// handling specific types of inputs or tasks.
type Lobe interface {
	// ID returns the unique identifier for this lobe
	ID() LobeID

	// Process runs the lobe with access to shared state
	Process(ctx context.Context, input LobeInput, blackboard *Blackboard) (*LobeResult, error)

	// CanHandle returns confidence (0.0-1.0) that this lobe can handle the input
	CanHandle(input string) float64

	// ResourceEstimate returns expected compute cost
	ResourceEstimate(input LobeInput) ResourceEstimate
}

// LobeInput contains all input data for a lobe
type LobeInput struct {
	RawInput     string                 `json:"raw_input"`
	ParsedIntent *Intent                `json:"parsed_intent,omitempty"`
	Strategy     *ThinkingStrategy      `json:"strategy,omitempty"`
	PhaseConfig  map[string]interface{} `json:"phase_config,omitempty"`
}

// Intent represents parsed user intent
type Intent struct {
	Action     string            `json:"action"`
	Subject    string            `json:"subject"`
	Parameters map[string]string `json:"parameters,omitempty"`
	Confidence float64           `json:"confidence"`
}

// LobeResult is the output from any cognitive module
type LobeResult struct {
	LobeID  LobeID      `json:"lobe_id"`
	Content interface{} `json:"content"`

	// Metadata
	Meta LobeMeta `json:"meta"`

	// Replanning signals
	RequestReplan bool     `json:"request_replan"`
	ReplanReason  string   `json:"replan_reason,omitempty"`
	SuggestLobes  []LobeID `json:"suggest_lobes,omitempty"`

	// Confidence
	Confidence float64  `json:"confidence"`
	Caveats    []string `json:"caveats,omitempty"`
}

// LobeMeta contains execution metadata
type LobeMeta struct {
	StartedAt  time.Time     `json:"started_at"`
	Duration   time.Duration `json:"duration"`
	TokensUsed int           `json:"tokens_used"`
	ModelUsed  string        `json:"model_used"`
	CacheHit   bool          `json:"cache_hit"`
}

// ResourceEstimate predicts compute requirements
type ResourceEstimate struct {
	EstimatedTokens int           `json:"estimated_tokens"`
	EstimatedTime   time.Duration `json:"estimated_time"`
	RequiresGPU     bool          `json:"requires_gpu"`
}
