// Package autollm provides a two-lane model router for automatic LLM selection.
// It routes requests to either a "Fast" lane (local/cheap models) or "Smart" lane
// (frontier models) based on hard constraints, user intent, and availability.
package autollm

import (
	"context"
	"time"

	"github.com/normanking/cortex/internal/eval"
)

// ═══════════════════════════════════════════════════════════════════════════════
// LANE TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// Lane represents the routing mode for model selection.
type Lane string

const (
	// LaneFast routes to local/Groq/cheap models for speed and cost efficiency.
	// Default behavior - $0 to ~$0.001 per request.
	LaneFast Lane = "fast"

	// LaneSmart routes to frontier models for maximum quality.
	// Triggered by --strong flag or forced by constraints (vision, context overflow).
	LaneSmart Lane = "smart"
)

// String returns the lane name for display.
func (l Lane) String() string {
	return string(l)
}

// ═══════════════════════════════════════════════════════════════════════════════
// REQUEST
// ═══════════════════════════════════════════════════════════════════════════════

// Request contains the completion request with routing hints.
type Request struct {
	// Prompt is the user's input text.
	Prompt string

	// Images contains base64-encoded images for vision models.
	// If non-empty, the router will ensure a vision-capable model is selected.
	Images []string

	// Mode specifies the user's lane preference.
	// Empty string means "use default" (fast lane).
	Mode Lane

	// LocalOnly forces local-only inference (--local flag).
	// When true, the router will never fall back to cloud APIs.
	LocalOnly bool

	// EstimatedTokens is the expected total token count (prompt + output).
	// Used to check against per-model context limits.
	// If 0, context checks are skipped.
	EstimatedTokens int

	// SystemPrompt is an optional system prompt for the request.
	SystemPrompt string

	// Messages is the conversation history for multi-turn chat.
	Messages []Message

	// TaskType specifies the task category for prompt optimization.
	// When set and the prompt store has an optimized prompt for this task,
	// it will be injected as the system prompt.
	TaskType string
}

// Message represents a conversation message.
type Message struct {
	Role    string `json:"role"`    // "user", "assistant", "system"
	Content string `json:"content"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// ROUTING DECISION
// ═══════════════════════════════════════════════════════════════════════════════

// RoutingDecision explains why a model was selected.
type RoutingDecision struct {
	// Model is the selected model identifier (e.g., "llama3:8b", "claude-3-5-sonnet").
	Model string

	// Lane is which routing lane was used.
	Lane Lane

	// Provider is the model provider (ollama, groq, openai, anthropic, gemini).
	Provider string

	// Reason is a human-readable explanation of the routing decision.
	Reason string

	// Forced indicates if a hard constraint forced this decision.
	// When true, user intent was overridden by physical requirements.
	Forced bool

	// Constraint names which constraint forced the decision (if Forced is true).
	// Values: "vision", "context_overflow", "no_local_models", "no_fast_models", "no_models"
	Constraint string

	// LearnedConfidence is the outcome-adjusted confidence for this routing decision.
	// Range: 0-1, where higher values indicate better historical performance.
	LearnedConfidence float64

	// ModelCapability contains the full capability info for the selected model.
	ModelCapability *eval.ModelCapability
}

// ═══════════════════════════════════════════════════════════════════════════════
// AVAILABILITY CACHE
// ═══════════════════════════════════════════════════════════════════════════════

// AvailabilityCache caches model availability to avoid repeated checks.
type AvailabilityCache struct {
	// OllamaModels maps model names to availability (pulled and ready).
	OllamaModels map[string]bool

	// OllamaOnline indicates if the Ollama daemon is running.
	OllamaOnline bool

	// MLXModels maps model names to availability on MLX-LM server.
	MLXModels map[string]bool

	// MLXOnline indicates if the MLX-LM server is running.
	MLXOnline bool

	// DnetModels maps model names to availability on dnet server.
	DnetModels map[string]bool

	// DnetOnline indicates if the dnet distributed inference server is running.
	DnetOnline bool

	// PrimaryLocalBackend is the detected fastest local backend.
	// Values: "mlx", "ollama", "dnet", or "" if none available.
	PrimaryLocalBackend string

	// CloudProviders maps provider names to API key availability.
	// e.g., "openai" -> true means OPENAI_API_KEY is set.
	CloudProviders map[string]bool

	// ClaudeMaxAvailable indicates if claude-code CLI is installed and authenticated.
	ClaudeMaxAvailable bool

	// LastRefresh is the Unix timestamp of the last cache update.
	LastRefresh int64
}

// ═══════════════════════════════════════════════════════════════════════════════
// ROUTER CONFIG
// ═══════════════════════════════════════════════════════════════════════════════

// RouterConfig configures the model router.
type RouterConfig struct {
	// FastModels is the priority-ordered list of fast lane models.
	// Order: Local (MLX/Ollama/dnet) → Fast Cloud (Groq) → Cheap Cloud (GPT-4o-mini, etc.)
	FastModels []string

	// SmartModels is the priority-ordered list of smart lane models.
	// Order: Best quality → Good alternatives.
	SmartModels []string

	// DefaultFastModel is the fallback if no fast model is available.
	DefaultFastModel string

	// DefaultSmartModel is the fallback if no smart model is available.
	DefaultSmartModel string

	// OllamaEndpoint is the Ollama API endpoint.
	// Defaults to "http://127.0.0.1:11434" if empty.
	OllamaEndpoint string

	// MLXEndpoint is the MLX-LM server API endpoint (OpenAI-compatible).
	// Defaults to "http://127.0.0.1:8081" if empty.
	// MLX provides 5-10x faster inference on Apple Silicon.
	MLXEndpoint string

	// DnetEndpoint is the dnet distributed inference API endpoint (OpenAI-compatible).
	// Defaults to "http://127.0.0.1:9080" if empty.
	// dnet provides distributed inference across Apple Silicon clusters.
	DnetEndpoint string

	// PreferredLocalBackend allows forcing a specific local backend.
	// Values: "auto" (detect fastest), "mlx", "ollama", "dnet".
	// Defaults to "auto" if empty.
	PreferredLocalBackend string

	// UseClaudeMax enables using Claude Max subscription via claude-code CLI.
	// When true, Claude models will use the subscription instead of API keys.
	UseClaudeMax bool
}

// ═══════════════════════════════════════════════════════════════════════════════
// PROVIDER CONSTANTS
// ═══════════════════════════════════════════════════════════════════════════════

// Provider constants for clarity.
const (
	ProviderOllama    = "ollama"
	ProviderMLX       = "mlx"  // MLX-LM server (Apple Silicon optimized, 5-10x faster)
	ProviderDnet      = "dnet" // dnet distributed inference (Apple Silicon clusters)
	ProviderGroq      = "groq"
	ProviderOpenAI    = "openai"
	ProviderAnthropic = "anthropic"
	ProviderClaudeMax = "claude_max" // Claude Max subscription via claude-code CLI
	ProviderGemini    = "gemini"
	ProviderGoogle    = "google"
	ProviderMistral   = "mistral"
	ProviderGrok      = "grok"
)

// LocalProviders is the set of providers that run locally (no cloud API).
var LocalProviders = map[string]bool{
	ProviderOllama: true,
	ProviderMLX:    true,
	ProviderDnet:   true,
	"local":        true,
}

// IsLocalProvider returns true if the provider runs locally.
func IsLocalProvider(provider string) bool {
	return LocalProviders[provider]
}

// ═══════════════════════════════════════════════════════════════════════════════
// OUTCOME STORE (RoamPal Integration)
// ═══════════════════════════════════════════════════════════════════════════════

// OutcomeStore provides access to routing outcome data for learning.
// This interface enables the router to learn from past decisions and
// adjust confidence based on historical success rates.
type OutcomeStore interface {
	// GetModelSuccessRate returns the success rate for a model on a task type.
	// Returns: successRate (0-1), sampleCount, error
	GetModelSuccessRate(ctx context.Context, provider, model, taskType string) (float64, int, error)

	// GetLaneSuccessRate returns the success rate for a lane on a task type.
	GetLaneSuccessRate(ctx context.Context, lane, taskType string) (float64, int, error)

	// RecordOutcome records a routing outcome for future learning.
	RecordOutcome(ctx context.Context, outcome *RoutingOutcomeRecord) error
}

// RoutingOutcomeRecord is a single routing outcome for storage.
// Used by OutcomeStore to persist routing decisions and their outcomes.
type RoutingOutcomeRecord struct {
	Timestamp    time.Time
	Provider     string
	Model        string
	Lane         string
	TaskType     string
	Success      bool
	Score        float64
	LatencyMs    int
	WasEscalated bool
}

// ═══════════════════════════════════════════════════════════════════════════════
// ROUTING CONFIDENCE (RoamPal Integration)
// ═══════════════════════════════════════════════════════════════════════════════

// RoutingConfidence represents learned confidence for routing decisions.
// Combines heuristic-based confidence with outcome-based adjustments.
type RoutingConfidence struct {
	BaseConfidence     float64 // Heuristic-based confidence
	LearnedConfidence  float64 // Outcome-based adjustment
	SampleCount        int     // Number of samples used
	AdjustedConfidence float64 // Final combined confidence
}

// LearnedRoutingConfig holds thresholds for learned routing.
// These parameters control how the router adjusts confidence based on
// historical outcome data from the OutcomeStore.
type LearnedRoutingConfig struct {
	MinSamplesForConfidence    int     // Minimum samples before using learned confidence (default: 5)
	ConfidenceBoostThreshold   float64 // Success rate above this boosts confidence (default: 0.85)
	ConfidencePenaltyThreshold float64 // Success rate below this penalizes (default: 0.4)
	MaxConfidenceAdjustment    float64 // Maximum adjustment to base confidence (default: 0.3)
	DecayFactor                float64 // Weight decay for older outcomes (default: 0.95)
}

// DefaultLearnedRoutingConfig returns sensible defaults for learned routing.
func DefaultLearnedRoutingConfig() LearnedRoutingConfig {
	return LearnedRoutingConfig{
		MinSamplesForConfidence:    5,
		ConfidenceBoostThreshold:   0.85,
		ConfidencePenaltyThreshold: 0.4,
		MaxConfidenceAdjustment:    0.3,
		DecayFactor:                0.95,
	}
}