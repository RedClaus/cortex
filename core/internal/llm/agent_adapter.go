package llm

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/normanking/cortex/internal/agent"
)

// AgentLLMAdapter adapts our Provider interface to the agent's LLMProvider interface.
// It also implements agent.TokenAccumulator to track token usage across calls.
type AgentLLMAdapter struct {
	provider    Provider
	model       string
	totalTokens int
	mu          sync.Mutex
}

// NewAgentAdapter creates an adapter for the agent to use our LLM provider.
func NewAgentAdapter(p Provider, model string) *AgentLLMAdapter {
	return &AgentLLMAdapter{
		provider: p,
		model:    model,
	}
}

// Chat implements agent.LLMProvider.
func (a *AgentLLMAdapter) Chat(ctx context.Context, messages []agent.ChatMessage, systemPrompt string) (string, error) {
	// Convert agent messages to our message format
	var llmMessages []Message
	for _, msg := range messages {
		llmMessages = append(llmMessages, Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	req := &ChatRequest{
		Model:        a.model,
		SystemPrompt: systemPrompt,
		Messages:     llmMessages,
		MaxTokens:    4096,
		Temperature:  0.7,
	}

	resp, err := a.provider.Chat(ctx, req)
	if err != nil {
		return "", err
	}

	// Accumulate tokens
	a.mu.Lock()
	a.totalTokens += resp.TokensUsed
	a.mu.Unlock()

	return resp.Content, nil
}

// GetTotalTokens implements agent.TokenAccumulator.
func (a *AgentLLMAdapter) GetTotalTokens() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.totalTokens
}

// ResetTokens implements agent.TokenAccumulator.
func (a *AgentLLMAdapter) ResetTokens() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.totalTokens = 0
}

// ChatStream implements agent.StreamingLLMProvider for real-time token streaming.
func (a *AgentLLMAdapter) ChatStream(ctx context.Context, messages []agent.ChatMessage, systemPrompt string, onToken func(string)) (string, error) {
	// Check if provider supports streaming
	if streamingProvider, ok := a.provider.(StreamingProvider); ok {
		// Convert agent messages to our message format
		var llmMessages []Message
		for _, msg := range messages {
			llmMessages = append(llmMessages, Message{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}

		req := &ChatRequest{
			Model:        a.model,
			SystemPrompt: systemPrompt,
			Messages:     llmMessages,
			MaxTokens:    4096,
			Temperature:  0.7,
		}

		result, err := streamingProvider.ChatStream(ctx, req, onToken)

		// Estimate tokens since streaming doesn't return exact counts
		// Rough estimate: ~4 chars per token for English text
		if err == nil {
			estimatedTokens := (len(result) + 3) / 4 // completion tokens
			for _, msg := range messages {
				estimatedTokens += (len(msg.Content) + 3) / 4 // input tokens
			}
			estimatedTokens += (len(systemPrompt) + 3) / 4 // system prompt tokens

			a.mu.Lock()
			a.totalTokens += estimatedTokens
			a.mu.Unlock()
		}

		return result, err
	}

	// Fallback to non-streaming
	return a.Chat(ctx, messages, systemPrompt)
}

// SetModel updates the model being used.
func (a *AgentLLMAdapter) SetModel(model string) {
	a.model = model
}

// ═══════════════════════════════════════════════════════════════════════════════
// MULTI-PROVIDER ADAPTER
// ═══════════════════════════════════════════════════════════════════════════════

// MultiProviderAdapter routes requests to the correct provider based on model name.
// This solves the issue where model switching in TUI would send MLX model names
// to Ollama provider (or vice versa), causing 404 errors.
// It also implements agent.TokenAccumulator to track token usage across calls.
type MultiProviderAdapter struct {
	providers       map[string]Provider // provider name -> provider
	model           string
	defaultProvider string
	totalTokens     int
	mu              sync.RWMutex
}

// NewMultiProviderAdapter creates an adapter that routes to multiple providers.
func NewMultiProviderAdapter(defaultProvider string, defaultModel string) *MultiProviderAdapter {
	return &MultiProviderAdapter{
		providers:       make(map[string]Provider),
		model:           defaultModel,
		defaultProvider: defaultProvider,
	}
}

// AddProvider registers a provider by name.
func (m *MultiProviderAdapter) AddProvider(name string, p Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[name] = p
}

// Chat implements agent.LLMProvider, routing to the correct provider.
func (m *MultiProviderAdapter) Chat(ctx context.Context, messages []agent.ChatMessage, systemPrompt string) (string, error) {
	m.mu.RLock()
	model := m.model
	m.mu.RUnlock()

	// Detect which provider this model belongs to
	providerName := detectProviderFromModel(model)

	// Get the provider
	m.mu.RLock()
	provider, ok := m.providers[providerName]
	if !ok {
		// Try default provider
		provider, ok = m.providers[m.defaultProvider]
	}
	m.mu.RUnlock()

	if !ok || provider == nil {
		return "", fmt.Errorf("no provider available for model %s (detected: %s, default: %s)", model, providerName, m.defaultProvider)
	}

	// Convert agent messages to our message format
	var llmMessages []Message
	for _, msg := range messages {
		llmMessages = append(llmMessages, Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	req := &ChatRequest{
		Model:        model,
		SystemPrompt: systemPrompt,
		Messages:     llmMessages,
		MaxTokens:    4096,
		Temperature:  0.7,
	}

	resp, err := provider.Chat(ctx, req)
	if err != nil {
		return "", err
	}

	// Accumulate tokens
	m.mu.Lock()
	m.totalTokens += resp.TokensUsed
	m.mu.Unlock()

	return resp.Content, nil
}

// GetTotalTokens implements agent.TokenAccumulator.
func (m *MultiProviderAdapter) GetTotalTokens() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.totalTokens
}

// ResetTokens implements agent.TokenAccumulator.
func (m *MultiProviderAdapter) ResetTokens() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalTokens = 0
}

// SetModel updates the model being used.
func (m *MultiProviderAdapter) SetModel(model string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.model = model
}

// ChatStream implements agent.StreamingLLMProvider for real-time token streaming.
func (m *MultiProviderAdapter) ChatStream(ctx context.Context, messages []agent.ChatMessage, systemPrompt string, onToken func(string)) (string, error) {
	m.mu.RLock()
	model := m.model
	m.mu.RUnlock()

	// Detect which provider this model belongs to
	providerName := detectProviderFromModel(model)

	// Get the provider
	m.mu.RLock()
	provider, ok := m.providers[providerName]
	if !ok {
		// Try default provider
		provider, ok = m.providers[m.defaultProvider]
	}
	m.mu.RUnlock()

	if !ok || provider == nil {
		return "", fmt.Errorf("no provider available for model %s (detected: %s, default: %s)", model, providerName, m.defaultProvider)
	}

	// Check if provider supports streaming
	if streamingProvider, ok := provider.(StreamingProvider); ok {
		// Convert agent messages to our message format
		var llmMessages []Message
		for _, msg := range messages {
			llmMessages = append(llmMessages, Message{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}

		req := &ChatRequest{
			Model:        model,
			SystemPrompt: systemPrompt,
			Messages:     llmMessages,
			MaxTokens:    4096,
			Temperature:  0.7,
		}

		result, err := streamingProvider.ChatStream(ctx, req, onToken)

		// Estimate tokens since streaming doesn't return exact counts
		// Rough estimate: ~4 chars per token for English text
		if err == nil {
			estimatedTokens := (len(result) + 3) / 4 // completion tokens
			for _, msg := range messages {
				estimatedTokens += (len(msg.Content) + 3) / 4 // input tokens
			}
			estimatedTokens += (len(systemPrompt) + 3) / 4 // system prompt tokens

			m.mu.Lock()
			m.totalTokens += estimatedTokens
			m.mu.Unlock()
		}

		return result, err
	}

	// Fallback to non-streaming
	return m.Chat(ctx, messages, systemPrompt)
}

// GetModel returns the current model.
func (m *MultiProviderAdapter) GetModel() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.model
}

// detectProviderFromModel infers provider from model name.
// This mirrors the logic in autollm/router.go detectProvider().
func detectProviderFromModel(modelName string) string {
	modelLower := strings.ToLower(modelName)

	// Explicit provider prefix (e.g., "groq/llama-3.1-70b")
	if strings.HasPrefix(modelLower, "groq/") {
		return "groq"
	}

	// Grok models (xAI)
	if strings.HasPrefix(modelLower, "grok") {
		return "grok"
	}

	// Anthropic models
	if strings.Contains(modelLower, "claude") {
		return "anthropic"
	}

	// OpenAI models
	if strings.HasPrefix(modelLower, "gpt") ||
		strings.HasPrefix(modelLower, "o1") ||
		strings.HasPrefix(modelLower, "chatgpt") {
		return "openai"
	}

	// Gemini models
	if strings.Contains(modelLower, "gemini") {
		return "gemini"
	}

	// MLX models (HuggingFace format with mlx-community prefix)
	if strings.HasPrefix(modelLower, "mlx") ||
		strings.Contains(modelLower, "mlx-community/") {
		return "mlx"
	}

	// Ollama models (colon-separated tag indicates local Ollama)
	if strings.Contains(modelName, ":") {
		return "ollama"
	}

	// Common local model families (likely Ollama)
	localFamilies := []string{"llama", "mistral", "mixtral", "qwen", "phi", "codellama", "deepseek", "tinyllama", "orca", "vicuna", "neural", "wizard", "gemma", "starcoder", "command-r"}
	for _, family := range localFamilies {
		if strings.HasPrefix(modelLower, family) {
			return "ollama"
		}
	}

	// Default to ollama for unknown models (assume local)
	return "ollama"
}
