package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// GrokProvider implements the Provider interface for xAI's Grok models.
// Grok uses an OpenAI-compatible API, so the request/response formats are similar.
type GrokProvider struct {
	baseProvider
}

// NewGrokProvider creates a new Grok provider.
func NewGrokProvider(cfg *ProviderConfig) *GrokProvider {
	return &GrokProvider{
		baseProvider: newBaseProvider(cfg, "grok"),
	}
}

// Chat sends a chat request to Grok.
func (p *GrokProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if p.config.APIKey == "" {
		return nil, fmt.Errorf("Grok API key not configured")
	}

	start := time.Now()

	// Build request (OpenAI-compatible format)
	grokReq := grokChatRequest{
		Model: req.Model,
	}

	if grokReq.Model == "" {
		grokReq.Model = p.config.Model
	}

	// Add system prompt
	if req.SystemPrompt != "" {
		grokReq.Messages = append(grokReq.Messages, grokMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}

	// Convert messages
	for _, msg := range req.Messages {
		grokReq.Messages = append(grokReq.Messages, grokMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Set parameters
	grokReq.MaxTokens = req.MaxTokens
	if grokReq.MaxTokens == 0 {
		grokReq.MaxTokens = p.config.MaxTokens
	}
	grokReq.Temperature = req.Temperature
	if grokReq.Temperature == 0 {
		grokReq.Temperature = p.config.Temperature
	}

	// Marshal request
	body, err := json.Marshal(grokReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.config.Endpoint+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	// Execute request
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := readLimitedBody(resp.Body, MaxErrorBodySize)
		return nil, fmt.Errorf("Grok error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response (OpenAI-compatible format)
	var grokResp grokChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&grokResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(grokResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := grokResp.Choices[0]
	return &ChatResponse{
		Content:          choice.Message.Content,
		Model:            grokResp.Model,
		PromptTokens:     grokResp.Usage.PromptTokens,
		CompletionTokens: grokResp.Usage.CompletionTokens,
		TokensUsed:       grokResp.Usage.TotalTokens,
		Duration:         time.Since(start),
		FinishReason:     choice.FinishReason,
	}, nil
}

// Grok API types (OpenAI-compatible)
type grokChatRequest struct {
	Model       string        `json:"model"`
	Messages    []grokMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

type grokMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type grokChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int         `json:"index"`
		Message      grokMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}
