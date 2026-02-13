package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// AnthropicProvider implements the Provider interface for Anthropic Claude.
type AnthropicProvider struct {
	baseProvider
}

// NewAnthropicProvider creates a new Anthropic provider.
func NewAnthropicProvider(cfg *ProviderConfig) *AnthropicProvider {
	return &AnthropicProvider{
		baseProvider: newBaseProvider(cfg, "anthropic"),
	}
}

// Chat sends a chat request to Anthropic.
func (p *AnthropicProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if p.config.APIKey == "" {
		return nil, fmt.Errorf("Anthropic API key not configured")
	}

	start := time.Now()

	// Build Anthropic request
	anthropicReq := anthropicChatRequest{
		Model: req.Model,
	}

	if anthropicReq.Model == "" {
		anthropicReq.Model = p.config.Model
	}

	// Set system prompt
	if req.SystemPrompt != "" {
		anthropicReq.System = req.SystemPrompt
	}

	// Convert messages (Anthropic uses different format)
	for _, msg := range req.Messages {
		anthropicReq.Messages = append(anthropicReq.Messages, anthropicMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Set parameters
	anthropicReq.MaxTokens = req.MaxTokens
	if anthropicReq.MaxTokens == 0 {
		anthropicReq.MaxTokens = p.config.MaxTokens
	}
	anthropicReq.Temperature = req.Temperature
	if anthropicReq.Temperature == 0 {
		anthropicReq.Temperature = p.config.Temperature
	}

	// Marshal request
	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.config.Endpoint+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.config.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	// Execute request
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := readLimitedBody(resp.Body, MaxErrorBodySize)
		return nil, fmt.Errorf("Anthropic error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var anthropicResp anthropicChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Extract content from response
	var content string
	for _, block := range anthropicResp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	return &ChatResponse{
		Content:          content,
		Model:            anthropicResp.Model,
		PromptTokens:     anthropicResp.Usage.InputTokens,
		CompletionTokens: anthropicResp.Usage.OutputTokens,
		TokensUsed:       anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
		Duration:         time.Since(start),
		FinishReason:     anthropicResp.StopReason,
	}, nil
}

// Anthropic API types
type anthropicChatRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	System      string             `json:"system,omitempty"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicChatResponse struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Role       string `json:"role"`
	Model      string `json:"model"`
	StopReason string `json:"stop_reason"`
	Content    []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}
