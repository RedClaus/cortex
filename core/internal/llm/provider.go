// Package llm provides Language Model provider implementations for Cortex.
// Supports Ollama (local), OpenAI, Anthropic, and Google Gemini.
package llm

import (
	"context"
	"io"
	"net/http"
	"time"
)

// Security limits to prevent unbounded memory usage
const (
	// MaxErrorBodySize limits how much error response body we read (1MB)
	// This prevents memory exhaustion from malformed/malicious error responses
	MaxErrorBodySize = 1 * 1024 * 1024

	// MaxStreamedResponseSize limits total streamed response size (50MB)
	// This prevents runaway generation from consuming all memory
	MaxStreamedResponseSize = 50 * 1024 * 1024
)

// readLimitedBody reads up to maxBytes from r, returning the bytes read.
// This is used for error responses to prevent unbounded memory allocation.
func readLimitedBody(r io.Reader, maxBytes int64) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, maxBytes))
}

// Provider defines the interface for LLM providers.
type Provider interface {
	// Chat sends a message and returns the response.
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)

	// Name returns the provider identifier.
	Name() string

	// Available returns true if the provider is configured and reachable.
	Available() bool
}

// StreamingProvider extends Provider with streaming support.
type StreamingProvider interface {
	Provider
	// ChatStream is like Chat but calls onToken for each token as it's generated.
	// Returns the complete response when done.
	ChatStream(ctx context.Context, req *ChatRequest, onToken func(token string)) (string, error)
}

// ChatRequest represents a chat completion request.
type ChatRequest struct {
	// Model to use (provider-specific).
	Model string `json:"model"`

	// SystemPrompt sets the AI's behavior.
	SystemPrompt string `json:"system_prompt,omitempty"`

	// Messages in the conversation.
	Messages []Message `json:"messages"`

	// MaxTokens limits response length.
	MaxTokens int `json:"max_tokens,omitempty"`

	// Temperature controls randomness (0.0-1.0).
	Temperature float64 `json:"temperature,omitempty"`

	// Stream enables streaming responses.
	Stream bool `json:"stream,omitempty"`
}

// Message represents a conversation message.
type Message struct {
	Role    string `json:"role"` // "user", "assistant", "system"
	Content string `json:"content"`
}

// ChatResponse contains the LLM's response.
type ChatResponse struct {
	Content          string           `json:"content"`
	Model            string           `json:"model"`
	TokensUsed       int              `json:"tokens_used,omitempty"`
	PromptTokens     int              `json:"prompt_tokens,omitempty"`
	CompletionTokens int              `json:"completion_tokens,omitempty"`
	Duration         time.Duration    `json:"duration"`
	FinishReason     string           `json:"finish_reason,omitempty"`
	ToolCalls        []ToolCallResult `json:"tool_calls,omitempty"`
}

type ToolCallResult struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ProviderConfig contains configuration for an LLM provider.
type ProviderConfig struct {
	// Name identifies the provider (ollama, openai, anthropic, gemini).
	Name string

	// Endpoint is the API base URL.
	Endpoint string

	// APIKey for authentication.
	APIKey string

	// Model is the default model to use.
	Model string

	// MaxTokens default for responses.
	MaxTokens int

	// Temperature default.
	Temperature float64

	// Timeout for API calls.
	Timeout time.Duration
}

// DefaultConfig returns sensible defaults for a provider.
func DefaultConfig(name string) *ProviderConfig {
	switch name {
	case "ollama":
		return &ProviderConfig{
			Name:        "ollama",
			Endpoint:    "http://127.0.0.1:11434",
			Model:       "llama3",
			MaxTokens:   4096,
			Temperature: 0.7,
			Timeout:     2 * time.Minute,
		}
	case "openai":
		return &ProviderConfig{
			Name:        "openai",
			Endpoint:    "https://api.openai.com/v1",
			Model:       "gpt-4o-mini",
			MaxTokens:   4096,
			Temperature: 0.7,
			Timeout:     2 * time.Minute,
		}
	case "anthropic":
		return &ProviderConfig{
			Name:        "anthropic",
			Endpoint:    "https://api.anthropic.com",
			Model:       "claude-3-5-sonnet-20241022",
			MaxTokens:   4096,
			Temperature: 0.7,
			Timeout:     2 * time.Minute,
		}
	case "gemini":
		return &ProviderConfig{
			Name:        "gemini",
			Endpoint:    "https://generativelanguage.googleapis.com/v1beta",
			Model:       "gemini-1.5-flash",
			MaxTokens:   4096,
			Temperature: 0.7,
			Timeout:     2 * time.Minute,
		}
	case "grok":
		return &ProviderConfig{
			Name:        "grok",
			Endpoint:    "https://api.x.ai/v1",
			Model:       "grok-3-fast",
			MaxTokens:   4096,
			Temperature: 0.7,
			Timeout:     2 * time.Minute,
		}
	case "groq":
		// Groq provides ultra-fast LLM inference (~20-100ms)
		// Perfect for real-time voice conversations
		return &ProviderConfig{
			Name:        "groq",
			Endpoint:    "https://api.groq.com/openai/v1",
			Model:       "llama-3.3-70b-versatile",
			MaxTokens:   2048, // Keep short for fast responses
			Temperature: 0.7,
			Timeout:     30 * time.Second, // Groq is fast, short timeout is fine
		}
	default:
		return &ProviderConfig{
			Name:        name,
			MaxTokens:   4096,
			Temperature: 0.7,
			Timeout:     2 * time.Minute,
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// BASE PROVIDER (DRY helper for HTTP-based providers)
// ═══════════════════════════════════════════════════════════════════════════════

// baseProvider provides common functionality for HTTP-based LLM providers.
type baseProvider struct {
	config *ProviderConfig
	client *http.Client
}

// newBaseProvider creates a new base provider with defaults applied.
func newBaseProvider(cfg *ProviderConfig, providerName string) baseProvider {
	if cfg == nil {
		cfg = DefaultConfig(providerName)
	}

	defaults := DefaultConfig(providerName)
	if cfg.Endpoint == "" {
		cfg.Endpoint = defaults.Endpoint
	}
	if cfg.Model == "" {
		cfg.Model = defaults.Model
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = defaults.Timeout
	}
	cfg.Name = providerName

	return baseProvider{
		config: cfg,
		client: &http.Client{Timeout: cfg.Timeout},
	}
}

// Name returns the provider identifier.
func (b *baseProvider) Name() string {
	return b.config.Name
}

// Available checks if the API key is configured.
func (b *baseProvider) Available() bool {
	return b.config.APIKey != ""
}
