// Package a2a provides LLM proxy endpoints for external agents.
//
// This file implements HTTP endpoints that allow external agents (like SpannishTutor)
// to query available LLM providers and send chat requests through CortexBrain.
//
// Endpoints:
//   - GET /api/llm/providers - List available LLM providers
//   - POST /api/llm/chat - Send a chat request to a specific provider
package a2a

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/normanking/cortex/internal/llm"
	"github.com/normanking/cortex/internal/logging"
)

// ═══════════════════════════════════════════════════════════════════════════════
// LLM PROXY TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// ProviderInfo describes an available LLM provider.
type ProviderInfo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Model        string `json:"model"`
	Available    bool   `json:"available"`
	Description  string `json:"description,omitempty"`
	Capabilities struct {
		Streaming bool `json:"streaming"`
		MaxTokens int  `json:"maxTokens"`
	} `json:"capabilities"`
}

// ChatRequest is the request body for /api/llm/chat.
type ChatRequest struct {
	Provider     string       `json:"provider"`
	Messages     []ChatMsg    `json:"messages"`
	SystemPrompt string       `json:"systemPrompt,omitempty"`
	MaxTokens    int          `json:"maxTokens,omitempty"`
	Temperature  float64      `json:"temperature,omitempty"`
	Stream       bool         `json:"stream,omitempty"`
	// Context injection fields
	UserID    string `json:"userId,omitempty"`
	PersonaID string `json:"personaId,omitempty"`
}

// ChatMsg represents a message in the chat history.
type ChatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponseData is the response body for /api/llm/chat.
type ChatResponseData struct {
	Content      string `json:"content"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	TokensUsed   int    `json:"tokensUsed,omitempty"`
	FinishReason string `json:"finishReason,omitempty"`
	DurationMs   int64  `json:"durationMs"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// LLM PROXY
// ═══════════════════════════════════════════════════════════════════════════════

// LLMProxy manages multiple LLM providers and exposes them via HTTP.
type LLMProxy struct {
	providers   map[string]llm.Provider
	configs     map[string]*llm.ProviderConfig
	keyManager  *APIKeyManager
	log         *logging.Logger
	mu          sync.RWMutex
	lessonStore *LessonStore
}

// NewLLMProxy creates a new LLM proxy with the given providers.
func NewLLMProxy() *LLMProxy {
	proxy := &LLMProxy{
		providers: make(map[string]llm.Provider),
		configs:   make(map[string]*llm.ProviderConfig),
		log:       logging.Global(),
	}

	// Create key manager with callback to reinitialize providers when keys change
	proxy.keyManager = NewAPIKeyManager(func() {
		proxy.InitializeProviders()
	})

	return proxy
}

// GetKeyManager returns the API key manager for external use.
func (p *LLMProxy) GetKeyManager() *APIKeyManager {
	return p.keyManager
}

// SetLessonStore sets the lesson store for memory context injection.
func (p *LLMProxy) SetLessonStore(store *LessonStore) {
	p.lessonStore = store
}

// InitializeProviders sets up all LLM providers from stored config and environment variables.
func (p *LLMProxy) InitializeProviders() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Clear existing providers
	p.providers = make(map[string]llm.Provider)
	p.configs = make(map[string]*llm.ProviderConfig)

	providerDefs := []struct {
		name        string
		description string
	}{
		{"gemini", "Google Gemini - Fast and efficient"},
		{"anthropic", "Anthropic Claude - Thoughtful and nuanced"},
		{"openai", "OpenAI GPT - Versatile and capable"},
		{"grok", "xAI Grok - Real-time knowledge"},
		{"ollama", "Ollama - Local LLM"},
	}

	for _, def := range providerDefs {
		// Get config from key manager
		keyCfg, exists := p.keyManager.GetProviderConfig(def.name)
		if !exists {
			continue
		}

		cfg := &llm.ProviderConfig{
			Name:     def.name,
			Model:    keyCfg.Model,
			APIKey:   keyCfg.APIKey,
			Endpoint: keyCfg.Endpoint,
		}

		// Set default endpoints if not configured
		if cfg.Endpoint == "" {
			switch def.name {
			case "ollama":
				cfg.Endpoint = "http://127.0.0.1:11434"
			case "grok":
				cfg.Endpoint = "https://api.x.ai/v1"
			}
		}

		// Set default models if not configured
		if cfg.Model == "" {
			switch def.name {
			case "gemini":
				cfg.Model = "gemini-2.0-flash-exp"
			case "anthropic":
				cfg.Model = "claude-3-5-sonnet-20241022"
			case "openai":
				cfg.Model = "gpt-4o-mini"
			case "grok":
				cfg.Model = "grok-3-fast"
			case "ollama":
				cfg.Model = "deepseek-r1:latest"
			}
		}

		provider, err := llm.NewProviderByName(def.name, cfg)
		if err != nil {
			p.log.Warn("[LLMProxy] Failed to create provider %s: %v", def.name, err)
			continue
		}

		p.providers[def.name] = provider
		p.configs[def.name] = cfg
		p.log.Debug("[LLMProxy] Initialized provider: %s (available: %v)", def.name, provider.Available())
	}

	p.log.Info("[LLMProxy] Initialized %d LLM providers", len(p.providers))
}

// GetProviders returns information about all configured providers.
func (p *LLMProxy) GetProviders() []ProviderInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	providers := make([]ProviderInfo, 0, len(p.providers))

	descriptions := map[string]string{
		"gemini":    "Google Gemini - Fast, efficient, great for structured analysis",
		"anthropic": "Anthropic Claude - Thoughtful, nuanced, excellent reasoning",
		"openai":    "OpenAI GPT - Versatile, capable, broad knowledge",
		"grok":      "xAI Grok - Real-time knowledge, witty explanations",
		"ollama":    "Local LLM - Private, no API costs, runs locally",
	}

	for name, provider := range p.providers {
		cfg := p.configs[name]
		info := ProviderInfo{
			ID:          name,
			Name:        getDisplayName(name),
			Model:       cfg.Model,
			Available:   provider.Available(),
			Description: descriptions[name],
		}
		info.Capabilities.Streaming = true
		info.Capabilities.MaxTokens = 4096

		providers = append(providers, info)
	}

	return providers
}

// Chat sends a chat request to the specified provider.
func (p *LLMProxy) Chat(ctx context.Context, req *ChatRequest) (*ChatResponseData, error) {
	p.mu.RLock()
	provider, exists := p.providers[req.Provider]
	cfg := p.configs[req.Provider]
	p.mu.RUnlock()

	if !exists {
		return nil, &ProviderError{Provider: req.Provider, Message: "provider not found"}
	}

	if !provider.Available() {
		return nil, &ProviderError{Provider: req.Provider, Message: "provider not available (check API key)"}
	}

	// Inject memory context if user and persona are provided
	systemPrompt := req.SystemPrompt
	if p.lessonStore != nil && req.UserID != "" && req.PersonaID != "" {
		systemPrompt = p.injectMemoryContext(ctx, req.UserID, req.PersonaID, systemPrompt, req.Messages)
	}

	// Build LLM request
	messages := make([]llm.Message, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = llm.Message{Role: m.Role, Content: m.Content}
	}

	llmReq := &llm.ChatRequest{
		Model:        cfg.Model,
		SystemPrompt: systemPrompt,
		Messages:     messages,
		MaxTokens:    req.MaxTokens,
		Temperature:  req.Temperature,
		Stream:       req.Stream,
	}

	if llmReq.MaxTokens == 0 {
		llmReq.MaxTokens = 4096
	}
	if llmReq.Temperature == 0 {
		llmReq.Temperature = 0.7
	}

	start := time.Now()
	resp, err := provider.Chat(ctx, llmReq)
	if err != nil {
		return nil, &ProviderError{Provider: req.Provider, Message: err.Error()}
	}

	return &ChatResponseData{
		Content:      resp.Content,
		Provider:     req.Provider,
		Model:        resp.Model,
		TokensUsed:   resp.TokensUsed,
		FinishReason: resp.FinishReason,
		DurationMs:   time.Since(start).Milliseconds(),
	}, nil
}

// injectMemoryContext adds relevant past learning context to the system prompt.
func (p *LLMProxy) injectMemoryContext(ctx context.Context, userID, personaID, systemPrompt string, messages []ChatMsg) string {
	if p.lessonStore == nil {
		return systemPrompt
	}

	// Get current query from the last user message
	var currentQuery string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			currentQuery = messages[i].Content
			break
		}
	}

	var contextParts []string

	// Get recent conversation history (last 10 messages)
	recentContext, err := p.lessonStore.GetRecentContext(ctx, userID, personaID, 10)
	if err != nil {
		p.log.Warn("[LLMProxy] Failed to get recent context: %v", err)
	} else if recentContext != "" {
		contextParts = append(contextParts, recentContext)
	}

	// Get relevant past discussions for this query
	if currentQuery != "" {
		relevantContext, err := p.lessonStore.SearchRelevant(ctx, userID, personaID, currentQuery, 5)
		if err != nil {
			p.log.Warn("[LLMProxy] Failed to search relevant context: %v", err)
		} else if relevantContext != "" {
			contextParts = append(contextParts, relevantContext)
		}
	}

	// Build enhanced system prompt with memory context
	if len(contextParts) > 0 {
		var enhanced string
		enhanced = systemPrompt + "\n\n"
		enhanced += "# Memory Context from Previous Lessons\n\n"
		enhanced += "Use the following past learning context to personalize your response:\n\n"
		for _, part := range contextParts {
			enhanced += part + "\n"
		}
		enhanced += "---\n"
		p.log.Info("[LLMProxy] Injected %d context sections for user=%s persona=%s", len(contextParts), userID, personaID)
		return enhanced
	}

	return systemPrompt
}

// ProviderError represents an error from a specific provider.
type ProviderError struct {
	Provider string
	Message  string
}

func (e *ProviderError) Error() string {
	return e.Provider + ": " + e.Message
}

// getDisplayName returns a human-readable name for a provider.
func getDisplayName(id string) string {
	names := map[string]string{
		"gemini":    "Gemini",
		"anthropic": "Claude",
		"openai":    "GPT",
		"grok":      "Grok",
		"ollama":    "Ollama",
	}
	if name, ok := names[id]; ok {
		return name
	}
	return id
}

// ═══════════════════════════════════════════════════════════════════════════════
// HTTP HANDLERS
// ═══════════════════════════════════════════════════════════════════════════════

// HandleGetProviders handles GET /api/llm/providers.
func (p *LLMProxy) HandleGetProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	providers := p.GetProviders()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"providers": providers,
	})
}

// HandleChat handles POST /api/llm/chat.
func (p *LLMProxy) HandleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Provider == "" {
		http.Error(w, "provider is required", http.StatusBadRequest)
		return
	}

	if len(req.Messages) == 0 {
		http.Error(w, "messages are required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	resp, err := p.Chat(ctx, &req)
	if err != nil {
		if pe, ok := err.(*ProviderError); ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":    pe.Message,
				"provider": pe.Provider,
			})
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// RegisterRoutes registers LLM proxy routes on the given mux.
func (p *LLMProxy) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/llm/providers", p.HandleGetProviders)
	mux.HandleFunc("/api/llm/chat", p.HandleChat)

	p.log.Info("[LLMProxy] Registered routes: /api/llm/providers, /api/llm/chat")
}
