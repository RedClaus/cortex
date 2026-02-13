package autollm

// ═══════════════════════════════════════════════════════════════════════════════
// DEFAULT CONFIGURATION
// ═══════════════════════════════════════════════════════════════════════════════

// DefaultConfig returns a production-ready default configuration.
// Fast Lane: Local → Groq → Cheap Cloud (strict priority order)
// Smart Lane: Best frontier models in order of preference
//
// NOTE: Qwen models are prioritized for tool/function calling tasks due to
// superior accuracy (62% vs Llama's 20% on Berkeley Function Calling Benchmark).
func DefaultConfig() RouterConfig {
	return RouterConfig{
		// =========================================================================
		// FAST LANE: Local → Fast Cloud → Cheap Cloud
		// Order matters! First available model in list is selected.
		//
		// STRATEGY: Qwen models are prioritized for their superior tool-calling
		// accuracy. Research shows Qwen2.5 achieves 62% accuracy on function
		// calling benchmarks vs Llama's ~20%.
		// =========================================================================
		FastModels: []string{
			// === LOCAL (Ollama) - $0, lowest latency ===
			// Qwen models FIRST - best tool/function calling accuracy
			"qwen2.5-coder:32b", // Best: coding + tool calling
			"qwen2.5-coder:14b", // Good balance: quality + speed
			"qwen2.5-coder:7b",  // Fast: reliable tool calling
			"qwen2.5:32b",       // Large Qwen general
			"qwen2.5:14b",       // Medium Qwen general
			"qwen2.5:7b",        // Fast Qwen general
			// Large non-Qwen models (fallback)
			"llama3.1:70b",
			"llama3:70b",
			"mixtral:8x22b",
			// Medium local models
			"llama3.1:8b",
			"llama3:8b",
			"mixtral:8x7b",
			"mistral:7b",
			"codellama:34b",
			"codellama:13b",
			"codellama:7b",
			"deepseek-coder:6.7b",
			// Small local models (fast but less capable for tools)
			"qwen2.5:3b", // Small Qwen - still decent tool calling
			"phi3:medium",
			"phi3:mini",
			"llama3.2:3b",
			"llama3.2:1b",
			"gemma2:2b",
			"qwen2.5:1.5b",

			// === FAST CLOUD (Groq) - ~$0, fast inference ===
			"groq/llama-3.3-70b-versatile",
			"groq/llama-3.1-70b-versatile",
			"groq/llama-3.1-8b-instant",
			"groq/mixtral-8x7b-32768",
			"groq/gemma2-9b-it",

			// === CHEAP CLOUD - $0.075-0.80/M tokens ===
			"gemini-1.5-flash-8b", // $0.0375/$0.15/M - cheapest
			"gpt-4o-mini",         // $0.15/$0.60/M - excellent value
			"claude-3-haiku",      // $0.25/$1.25/M - fast Anthropic
			"gemini-1.5-flash",    // $0.075/$0.30/M - good value
		},

		// =========================================================================
		// SMART LANE: Best frontier models
		// For complex tasks, vision, or when user requests --strong
		// =========================================================================
		SmartModels: []string{
			"claude-3-5-sonnet", // Best balance of quality/cost, excellent coding
			"gpt-4o",            // Strong all-around, good vision
			"claude-3-opus",     // Highest quality Anthropic
			"gpt-4-turbo",       // Strong reasoning
			"gemini-1.5-pro",    // Good for very long context
			"claude-3-5-haiku",  // Fast smart model
			"o1",                // Best for complex reasoning (expensive)
			"o1-mini",           // Good reasoning, more affordable
		},

		// Fallbacks if preferred models unavailable
		DefaultFastModel:  "gpt-4o-mini",
		DefaultSmartModel: "claude-3-5-sonnet",

		// Default Ollama endpoint
		OllamaEndpoint: "http://127.0.0.1:11434",
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// SPECIALIZED CONFIGURATIONS
// ═══════════════════════════════════════════════════════════════════════════════

// LocalOnlyConfig returns config that never uses cloud APIs.
// Prioritizes Qwen models for best local tool-calling performance.
func LocalOnlyConfig() RouterConfig {
	return RouterConfig{
		FastModels: []string{
			// Qwen first for tool calling
			"qwen2.5-coder:32b",
			"qwen2.5-coder:14b",
			"qwen2.5-coder:7b",
			"qwen2.5:32b",
			"qwen2.5:14b",
			"qwen2.5:7b",
			// Large non-Qwen models
			"llama3.1:70b",
			"llama3:70b",
			"mixtral:8x22b",
			// Medium local models
			"llama3.1:8b",
			"llama3:8b",
			"mixtral:8x7b",
			"mistral:7b",
			"codellama:34b",
			"codellama:7b",
			"deepseek-coder:6.7b",
			// Small local models
			"qwen2.5:3b",
			"phi3:medium",
			"phi3:mini",
			"llama3.2:3b",
			"llama3.2:1b",
		},
		SmartModels: []string{
			// Best local models for complex tasks
			"qwen2.5-coder:32b",
			"qwen2.5:72b",
			"llama3.1:70b",
			"mixtral:8x22b",
			"command-r-plus",
			"deepseek-coder-v2:236b",
		},
		DefaultFastModel:  "qwen2.5-coder:7b",
		DefaultSmartModel: "qwen2.5-coder:32b",
		OllamaEndpoint:    "http://127.0.0.1:11434",
	}
}

// BudgetConfig returns config optimized for minimal cost.
// Prioritizes free/cheap models over quality.
func BudgetConfig() RouterConfig {
	return RouterConfig{
		FastModels: []string{
			// Local first (free)
			"llama3.2:1b",
			"llama3.2:3b",
			"phi3:mini",
			"gemma2:2b",
			"qwen2.5:1.5b",
			"llama3:8b",
			"mistral:7b",
			// Then Groq (essentially free)
			"groq/llama-3.1-8b-instant",
			"groq/gemma2-9b-it",
			// Then cheapest cloud
			"gemini-1.5-flash-8b", // $0.0375/M
			"gpt-4o-mini",         // $0.15/M
		},
		SmartModels: []string{
			"gemini-1.5-pro",    // $1.25/M - cheapest "smart"
			"claude-3-5-sonnet", // $3/M - best value for quality
			"gpt-4o",            // $2.5/M
			"claude-3-haiku",    // $0.25/M - fast
		},
		DefaultFastModel:  "gpt-4o-mini",
		DefaultSmartModel: "gemini-1.5-pro",
		OllamaEndpoint:    "http://127.0.0.1:11434",
	}
}

// QualityConfig returns config optimized for output quality.
// Uses best models available at each tier.
func QualityConfig() RouterConfig {
	return RouterConfig{
		FastModels: []string{
			// Best local models first
			"llama3.1:70b",
			"mixtral:8x22b",
			"qwen2.5-coder:32b",
			// Best cloud fast models
			"groq/llama-3.3-70b-versatile",
			"gpt-4o-mini",
			"claude-3-haiku",
		},
		SmartModels: []string{
			"claude-3-opus",     // Highest quality
			"o1",                // Best reasoning
			"claude-3-5-sonnet", // Excellent coding
			"gpt-4o",            // Strong all-around
			"gpt-4-turbo",       // Good reasoning
		},
		DefaultFastModel:  "gpt-4o-mini",
		DefaultSmartModel: "claude-3-opus",
		OllamaEndpoint:    "http://127.0.0.1:11434",
	}
}

// CodingConfig returns config optimized for code generation tasks.
// Prioritizes Qwen coder models for superior tool/function calling accuracy.
func CodingConfig() RouterConfig {
	return RouterConfig{
		FastModels: []string{
			// Qwen coder models first - best tool calling + coding
			"qwen2.5-coder:32b",
			"qwen2.5-coder:14b",
			"qwen2.5-coder:7b",
			"qwen2.5:14b",
			"qwen2.5:7b",
			// Other coding models
			"deepseek-coder:33b",
			"deepseek-coder:6.7b",
			"codellama:34b",
			"codellama:13b",
			"codellama:7b",
			// General local models
			"llama3.1:70b",
			"llama3:8b",
			// Cloud fast models
			"groq/llama-3.3-70b-versatile",
			"gpt-4o-mini",
		},
		SmartModels: []string{
			"claude-3-5-sonnet",
			"gpt-4o",
			"claude-3-opus",
			"o1",
		},
		DefaultFastModel:  "qwen2.5-coder:7b",
		DefaultSmartModel: "claude-3-5-sonnet",
		OllamaEndpoint:    "http://127.0.0.1:11434",
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// CONFIG BUILDER
// ═══════════════════════════════════════════════════════════════════════════════

// ConfigBuilder allows fluent configuration building.
type ConfigBuilder struct {
	config RouterConfig
}

// NewConfigBuilder starts building a custom config from defaults.
func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{
		config: DefaultConfig(),
	}
}

// WithFastModels sets the fast lane model list.
func (b *ConfigBuilder) WithFastModels(models []string) *ConfigBuilder {
	b.config.FastModels = models
	return b
}

// WithSmartModels sets the smart lane model list.
func (b *ConfigBuilder) WithSmartModels(models []string) *ConfigBuilder {
	b.config.SmartModels = models
	return b
}

// WithDefaultFastModel sets the fallback fast model.
func (b *ConfigBuilder) WithDefaultFastModel(model string) *ConfigBuilder {
	b.config.DefaultFastModel = model
	return b
}

// WithDefaultSmartModel sets the fallback smart model.
func (b *ConfigBuilder) WithDefaultSmartModel(model string) *ConfigBuilder {
	b.config.DefaultSmartModel = model
	return b
}

// WithOllamaEndpoint sets the Ollama API endpoint.
func (b *ConfigBuilder) WithOllamaEndpoint(endpoint string) *ConfigBuilder {
	b.config.OllamaEndpoint = endpoint
	return b
}

// PrependFastModel adds a model to the front of the fast lane.
func (b *ConfigBuilder) PrependFastModel(model string) *ConfigBuilder {
	b.config.FastModels = append([]string{model}, b.config.FastModels...)
	return b
}

// PrependSmartModel adds a model to the front of the smart lane.
func (b *ConfigBuilder) PrependSmartModel(model string) *ConfigBuilder {
	b.config.SmartModels = append([]string{model}, b.config.SmartModels...)
	return b
}

// Build returns the constructed config.
func (b *ConfigBuilder) Build() RouterConfig {
	return b.config
}
