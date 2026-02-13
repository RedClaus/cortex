package llm

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/normanking/cortex/internal/config"
)

// NewProvider creates an LLM provider based on configuration.
func NewProvider(cfg *config.Config) (Provider, error) {
	providerName := cfg.LLM.DefaultProvider
	if providerName == "" {
		providerName = "ollama"
	}

	providerCfg, exists := cfg.LLM.Providers[providerName]
	if !exists {
		return nil, fmt.Errorf("provider '%s' not found in configuration", providerName)
	}

	// Get API key from config, falling back to environment variables
	apiKey := providerCfg.APIKey
	if apiKey == "" {
		apiKey = getAPIKeyFromEnv(providerName)
	}

	llmCfg := &ProviderConfig{
		Name:     providerName,
		Endpoint: providerCfg.Endpoint,
		APIKey:   apiKey,
		Model:    providerCfg.Model,
	}

	return NewProviderByNameWithConfig(providerName, llmCfg, providerCfg.Timeouts)
}

// getAPIKeyFromEnv retrieves the API key from standard environment variables.
func getAPIKeyFromEnv(providerName string) string {
	envVars := map[string]string{
		"grok":       "XAI_API_KEY",
		"groq":       "GROQ_API_KEY",
		"openai":     "OPENAI_API_KEY",
		"anthropic":  "ANTHROPIC_API_KEY",
		"gemini":     "GEMINI_API_KEY",
		"openrouter": "OPENROUTER_API_KEY",
	}
	if envVar, ok := envVars[providerName]; ok {
		return os.Getenv(envVar)
	}
	return ""
}

// NewProviderByNameWithConfig creates a provider with optional timeout configuration.
// All providers are wrapped with MetricsProvider for call counting and latency tracking.
func NewProviderByNameWithConfig(name string, cfg *ProviderConfig, timeouts *config.TimeoutConfig) (Provider, error) {
	var provider Provider

	switch name {
	case "mlx":
		// MLX-LM provider (5-10x faster than Ollama on Apple Silicon)
		provider = NewMLXProvider(cfg)
	case "ollama":
		opts := buildOllamaOptions(timeouts)
		ollamaProvider := NewOllamaProvider(cfg, opts...)

		// Always trigger warmup for Ollama to avoid cold start delays (30-90+ seconds)
		// This runs in background and doesn't block startup.
		// The first LLM request would otherwise wait for model loading.
		// Warmup is enabled by default for Ollama providers.
		if ollamaProvider.Available() {
			ollamaProvider.WarmupAsync(context.Background())
		}

		provider = ollamaProvider
	case "openai":
		provider = NewOpenAIProvider(cfg)
	case "anthropic":
		provider = NewAnthropicProvider(cfg)
	case "gemini":
		provider = NewGeminiProvider(cfg)
	case "grok":
		provider = NewGrokProvider(cfg)
	case "groq":
		provider = NewGroqProvider(cfg)
	case "dnet":
		provider = NewDNetProvider(cfg)
	case "openrouter":
		provider = NewOpenRouterProvider(cfg)
	default:
		return nil, fmt.Errorf("unknown provider: %s", name)
	}

	// Wrap with MetricsProvider for call counting and register globally
	metricsProvider := NewMetricsProvider(provider)
	RegisterMetricsProvider(metricsProvider)

	return metricsProvider, nil
}

// buildOllamaOptions converts config.TimeoutConfig to OllamaOptions.
func buildOllamaOptions(timeouts *config.TimeoutConfig) []OllamaOption {
	if timeouts == nil {
		return nil
	}

	var opts []OllamaOption

	if timeouts.ConnectionTimeoutSec > 0 {
		opts = append(opts, WithConnectionTimeout(time.Duration(timeouts.ConnectionTimeoutSec)*time.Second))
	}
	if timeouts.FirstTokenTimeoutSec > 0 {
		opts = append(opts, WithFirstTokenTimeout(time.Duration(timeouts.FirstTokenTimeoutSec)*time.Second))
	}
	if timeouts.StreamIdleTimeoutSec > 0 {
		opts = append(opts, WithStreamIdleTimeout(time.Duration(timeouts.StreamIdleTimeoutSec)*time.Second))
	}

	return opts
}

// NewProviderByName creates a specific provider by name (without custom timeout config).
// For Ollama with custom timeouts, use NewProviderByNameWithConfig instead.
func NewProviderByName(name string, cfg *ProviderConfig) (Provider, error) {
	return NewProviderByNameWithConfig(name, cfg, nil)
}

// AvailableProviders returns a list of configured and available providers.
func AvailableProviders(cfg *config.Config) []string {
	var available []string

	for name, providerCfg := range cfg.LLM.Providers {
		llmCfg := &ProviderConfig{
			Name:     name,
			Endpoint: providerCfg.Endpoint,
			APIKey:   providerCfg.APIKey,
			Model:    providerCfg.Model,
		}

		provider, err := NewProviderByName(name, llmCfg)
		if err != nil {
			continue
		}

		if provider.Available() {
			available = append(available, name)
		}
	}

	return available
}
