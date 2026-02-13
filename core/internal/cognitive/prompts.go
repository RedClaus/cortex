package cognitive

import (
	"sync"

	"github.com/normanking/cortex/internal/prompts"
)

// PromptManager manages tier-optimized prompts for the cognitive pipeline
type PromptManager struct {
	provider *prompts.TemplateProvider
	mu       sync.RWMutex

	// defaultModelParams is the assumed model size when not specified
	defaultModelParams int64
}

// PromptManagerConfig configures the PromptManager
type PromptManagerConfig struct {
	// DefaultModelParams is used when model size is unknown
	// Default: 7B (7,000,000,000) - typical small model
	DefaultModelParams int64

	// CustomProvider allows injecting a pre-configured provider
	// If nil, a new provider is created from the default store
	CustomProvider *prompts.TemplateProvider
}

// DefaultPromptManagerConfig returns sensible defaults
func DefaultPromptManagerConfig() *PromptManagerConfig {
	return &PromptManagerConfig{
		DefaultModelParams: 7_000_000_000, // 7B - typical small model
	}
}

// NewPromptManager creates a PromptManager with the default store
func NewPromptManager() *PromptManager {
	return NewPromptManagerWithConfig(DefaultPromptManagerConfig())
}

// NewPromptManagerWithConfig creates a PromptManager with custom configuration
func NewPromptManagerWithConfig(cfg *PromptManagerConfig) *PromptManager {
	if cfg == nil {
		cfg = DefaultPromptManagerConfig()
	}

	provider := cfg.CustomProvider
	if provider == nil {
		// Load the default prompts store
		store := prompts.Load()
		provider = prompts.NewTemplateProvider(store)
	}

	return &PromptManager{
		provider:           provider,
		defaultModelParams: cfg.DefaultModelParams,
	}
}

// GetOptimizedPrompt returns the best prompt for the given model
// If modelParams is 0, uses the default model size
func (m *PromptManager) GetOptimizedPrompt(task string, modelParams int64) string {
	if modelParams == 0 {
		modelParams = m.defaultModelParams
	}
	return m.provider.GetSystemPrompt(task, modelParams)
}

// GetPromptForTier returns a prompt for an explicit tier (small/large)
func (m *PromptManager) GetPromptForTier(task string, tier string) string {
	return m.provider.GetPromptTemplate(task, tier)
}

// ApplyToTemplate applies an optimized prompt to a template context
// This is useful for injecting prompts into cognitive template rendering
// The prompt is added to the context under the key "system_prompt"
func (m *PromptManager) ApplyToTemplate(task string, modelParams int64, ctx map[string]interface{}) map[string]interface{} {
	if ctx == nil {
		ctx = make(map[string]interface{})
	}

	prompt := m.GetOptimizedPrompt(task, modelParams)
	ctx["system_prompt"] = prompt

	return ctx
}

// ListTasks returns all available task types
func (m *PromptManager) ListTasks() []string {
	return m.provider.ListTasks()
}

// HasTask checks if a task exists
func (m *PromptManager) HasTask(task string) bool {
	return m.provider.HasTask(task)
}

// GetTiers returns available tiers for a task
func (m *PromptManager) GetTiers(task string) []string {
	return m.provider.GetTiers(task)
}

// RegisterCustomPrompt adds a user-defined prompt
// This allows runtime customization of prompts
func (m *PromptManager) RegisterCustomPrompt(task, tier, prompt string) {
	m.provider.RegisterCustomPrompt(task, tier, prompt)
}

// RemoveCustomPrompt removes a custom prompt
func (m *PromptManager) RemoveCustomPrompt(task, tier string) {
	m.provider.RemoveCustomPrompt(task, tier)
}

// ClearCustomPrompts removes all custom prompts
func (m *PromptManager) ClearCustomPrompts() {
	m.provider.ClearCustomPrompts()
}

// GetModelTierFromParams returns the ModelTier based on parameter count
// This helps map model sizes to cognitive architecture tiers
func GetModelTierFromParams(modelParams int64) ModelTier {
	switch {
	case modelParams == 0:
		return TierLocal // Default to local if unknown
	case modelParams < 3_000_000_000: // < 3B
		return TierLocal
	case modelParams < 14_000_000_000: // 3B - 14B
		return TierMid
	case modelParams < 70_000_000_000: // 14B - 70B
		return TierAdvanced
	default: // >= 70B
		return TierFrontier
	}
}

// GetPromptTierFromModelTier maps cognitive ModelTier to prompt tier (small/large)
func GetPromptTierFromModelTier(tier ModelTier) string {
	switch tier {
	case TierLocal, TierMid:
		return "small"
	case TierAdvanced, TierFrontier:
		return "large"
	default:
		return "small"
	}
}

// GetOptimizedPromptForTier is a convenience method that combines
// model tier logic with prompt retrieval
func (m *PromptManager) GetOptimizedPromptForTier(task string, tier ModelTier) string {
	promptTier := GetPromptTierFromModelTier(tier)
	return m.GetPromptForTier(task, promptTier)
}

// EnrichTemplateContext adds prompt-related metadata to a template context
// This includes:
// - system_prompt: The optimized prompt for the task
// - prompt_tier: The tier used (small/large)
// - model_tier: The cognitive model tier
func (m *PromptManager) EnrichTemplateContext(
	task string,
	modelParams int64,
	ctx map[string]interface{},
) map[string]interface{} {
	if ctx == nil {
		ctx = make(map[string]interface{})
	}

	modelTier := GetModelTierFromParams(modelParams)
	promptTier := GetPromptTierFromModelTier(modelTier)
	prompt := m.GetPromptForTier(task, promptTier)

	ctx["system_prompt"] = prompt
	ctx["prompt_tier"] = promptTier
	ctx["model_tier"] = string(modelTier)

	return ctx
}
