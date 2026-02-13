package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// OpenAIProvider implements the Provider interface for OpenAI.
type OpenAIProvider struct {
	baseProvider
}

// NewOpenAIProvider creates a new OpenAI provider.
func NewOpenAIProvider(cfg *ProviderConfig) *OpenAIProvider {
	return &OpenAIProvider{
		baseProvider: newBaseProvider(cfg, "openai"),
	}
}

// Chat sends a chat request to OpenAI.
func (p *OpenAIProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if p.config.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API key not configured")
	}

	start := time.Now()

	// Build OpenAI request
	openaiReq := openAIChatRequest{
		Model: req.Model,
	}

	if openaiReq.Model == "" {
		openaiReq.Model = p.config.Model
	}

	// Add system prompt
	if req.SystemPrompt != "" {
		openaiReq.Messages = append(openaiReq.Messages, openAIMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}

	// Convert messages
	for _, msg := range req.Messages {
		openaiReq.Messages = append(openaiReq.Messages, openAIMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Set parameters
	openaiReq.MaxTokens = req.MaxTokens
	if openaiReq.MaxTokens == 0 {
		openaiReq.MaxTokens = p.config.MaxTokens
	}
	openaiReq.Temperature = req.Temperature
	if openaiReq.Temperature == 0 {
		openaiReq.Temperature = p.config.Temperature
	}

	// Marshal request
	body, err := json.Marshal(openaiReq)
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
		return nil, fmt.Errorf("OpenAI error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var openaiResp openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(openaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := openaiResp.Choices[0]
	return &ChatResponse{
		Content:          choice.Message.Content,
		Model:            openaiResp.Model,
		PromptTokens:     openaiResp.Usage.PromptTokens,
		CompletionTokens: openaiResp.Usage.CompletionTokens,
		TokensUsed:       openaiResp.Usage.TotalTokens,
		Duration:         time.Since(start),
		FinishReason:     choice.FinishReason,
	}, nil
}

// OpenAI API types
type openAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int           `json:"index"`
		Message      openAIMessage `json:"message"`
		FinishReason string        `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}
