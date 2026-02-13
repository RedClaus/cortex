package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// OpenRouterProvider implements the Provider interface for OpenRouter API.
// OpenRouter provides access to multiple models including Claude through a unified API.
type OpenRouterProvider struct {
	baseProvider
}

// NewOpenRouterProvider creates a new OpenRouter provider.
func NewOpenRouterProvider(cfg *ProviderConfig) *OpenRouterProvider {
	// Use OpenRouter endpoint if not specified
	if cfg.Endpoint == "" {
		cfg.Endpoint = "https://openrouter.ai/api"
	}
	return &OpenRouterProvider{
		baseProvider: newBaseProvider(cfg, "openrouter"),
	}
}

// Chat sends a chat request to OpenRouter.
func (p *OpenRouterProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if p.config.APIKey == "" {
		return nil, fmt.Errorf("OpenRouter API key not configured")
	}

	start := time.Now()

	// Build OpenRouter request (uses OpenAI-compatible format)
	openRouterReq := openRouterChatRequest{
		Model: req.Model,
	}

	if openRouterReq.Model == "" {
		openRouterReq.Model = p.config.Model
	}

	// Convert messages
	for _, msg := range req.Messages {
		openRouterReq.Messages = append(openRouterReq.Messages, openRouterMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Add system prompt as first message if provided
	if req.SystemPrompt != "" {
		systemMsg := openRouterMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		}
		// Prepend system message
		openRouterReq.Messages = append([]openRouterMessage{systemMsg}, openRouterReq.Messages...)
	}

	// Set parameters
	openRouterReq.MaxTokens = req.MaxTokens
	if openRouterReq.MaxTokens == 0 {
		openRouterReq.MaxTokens = p.config.MaxTokens
	}
	openRouterReq.Temperature = req.Temperature
	if openRouterReq.Temperature == 0 {
		openRouterReq.Temperature = p.config.Temperature
	}

	// Marshal request
	body, err := json.Marshal(openRouterReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.config.Endpoint+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	httpReq.Header.Set("X-Title", "CortexBrain") // Optional: helps OpenRouter track usage

	// Execute request
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := readLimitedBody(resp.Body, MaxErrorBodySize)
		return nil, fmt.Errorf("OpenRouter error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var openRouterResp openRouterChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&openRouterResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Extract content from the first choice
	var content string
	if len(openRouterResp.Choices) > 0 {
		content = openRouterResp.Choices[0].Message.Content
	}

	return &ChatResponse{
		Content:          content,
		Model:            openRouterResp.Model,
		PromptTokens:     openRouterResp.Usage.PromptTokens,
		CompletionTokens: openRouterResp.Usage.CompletionTokens,
		TokensUsed:       openRouterResp.Usage.TotalTokens,
		Duration:         time.Since(start),
		FinishReason:     openRouterResp.Choices[0].FinishReason,
	}, nil
}

// OpenRouter API types (OpenAI-compatible)
type openRouterChatRequest struct {
	Model       string               `json:"model"`
	Messages    []openRouterMessage  `json:"messages"`
	MaxTokens   int                  `json:"max_tokens,omitempty"`
	Temperature float64              `json:"temperature,omitempty"`
	Stream      bool                 `json:"stream,omitempty"`
}

type openRouterMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openRouterChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int               `json:"index"`
		Message      openRouterMessage `json:"message"`
		FinishReason string            `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}
