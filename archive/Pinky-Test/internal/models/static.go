package models

import "context"

// AnthropicModels is the static list of Anthropic Claude models
var AnthropicModels = []ModelInfo{
	{
		ID:          "claude-sonnet-4-20250514",
		Name:        "Claude Sonnet 4",
		Description: "Latest Claude Sonnet model with excellent reasoning",
		ContextSize: 200000,
	},
	{
		ID:          "claude-3-5-sonnet-20241022",
		Name:        "Claude 3.5 Sonnet",
		Description: "Balanced performance and speed",
		ContextSize: 200000,
	},
	{
		ID:          "claude-3-opus-20240229",
		Name:        "Claude 3 Opus",
		Description: "Most capable Claude model for complex tasks",
		ContextSize: 200000,
	},
	{
		ID:          "claude-3-haiku-20240307",
		Name:        "Claude 3 Haiku",
		Description: "Fast and cost-effective for simple tasks",
		ContextSize: 200000,
	},
}

// OpenAIModels is the static list of OpenAI models
var OpenAIModels = []ModelInfo{
	{
		ID:          "gpt-4o",
		Name:        "GPT-4o",
		Description: "Most capable multimodal model",
		ContextSize: 128000,
	},
	{
		ID:          "gpt-4o-mini",
		Name:        "GPT-4o Mini",
		Description: "Fast and affordable for most tasks",
		ContextSize: 128000,
	},
	{
		ID:          "gpt-4-turbo",
		Name:        "GPT-4 Turbo",
		Description: "Previous generation flagship model",
		ContextSize: 128000,
	},
	{
		ID:          "gpt-3.5-turbo",
		Name:        "GPT-3.5 Turbo",
		Description: "Cost-effective for simple tasks",
		ContextSize: 16385,
	},
}

// GroqModels is the static list of Groq-hosted models
var GroqModels = []ModelInfo{
	{
		ID:          "llama-3.3-70b-versatile",
		Name:        "Llama 3.3 70B",
		Description: "Meta's Llama 3.3 70B with high performance",
		ContextSize: 128000,
	},
	{
		ID:          "llama-3.1-8b-instant",
		Name:        "Llama 3.1 8B Instant",
		Description: "Fast responses with Llama 3.1 8B",
		ContextSize: 128000,
	},
	{
		ID:          "mixtral-8x7b-32768",
		Name:        "Mixtral 8x7B",
		Description: "Mixture of Experts model by Mistral",
		ContextSize: 32768,
	},
	{
		ID:          "gemma2-9b-it",
		Name:        "Gemma 2 9B",
		Description: "Google's lightweight Gemma 2 model",
		ContextSize: 8192,
	},
}

// StaticProvider implements the Provider interface for static model lists
type StaticProvider struct {
	engine string
	models []ModelInfo
}

// NewStaticProvider creates a new static provider
func NewStaticProvider(engine string, models []ModelInfo) *StaticProvider {
	// Create a copy of the models slice to prevent external modification
	modelsCopy := make([]ModelInfo, len(models))
	copy(modelsCopy, models)

	return &StaticProvider{
		engine: engine,
		models: modelsCopy,
	}
}

// Engine returns the provider engine name
func (p *StaticProvider) Engine() string {
	return p.engine
}

// ListModels returns the static list of models
func (p *StaticProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
	// Return a copy to prevent modification
	result := make([]ModelInfo, len(p.models))
	copy(result, p.models)
	return result, nil
}

// ValidateModel checks if a model ID exists in the static list
func (p *StaticProvider) ValidateModel(model string) bool {
	for _, m := range p.models {
		if m.ID == model {
			return true
		}
	}
	return false
}

// NewAnthropicProvider creates a provider for Anthropic models
func NewAnthropicProvider() *StaticProvider {
	return NewStaticProvider("anthropic", AnthropicModels)
}

// NewOpenAIProvider creates a provider for OpenAI models
func NewOpenAIProvider() *StaticProvider {
	return NewStaticProvider("openai", OpenAIModels)
}

// NewGroqProvider creates a provider for Groq models
func NewGroqProvider() *StaticProvider {
	return NewStaticProvider("groq", GroqModels)
}
