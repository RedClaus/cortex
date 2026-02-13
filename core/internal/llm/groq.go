package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// GroqProvider implements the Provider interface for Groq.
// Groq provides ultra-fast LLM inference with sub-100ms latency,
// making it ideal for real-time voice conversations.
type GroqProvider struct {
	baseProvider
}

// NewGroqProvider creates a new Groq provider.
// Groq uses an OpenAI-compatible API at https://api.groq.com/openai/v1
func NewGroqProvider(cfg *ProviderConfig) *GroqProvider {
	return &GroqProvider{
		baseProvider: newBaseProvider(cfg, "groq"),
	}
}

// Chat sends a chat request to Groq.
func (p *GroqProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if p.config.APIKey == "" {
		return nil, fmt.Errorf("Groq API key not configured")
	}

	start := time.Now()

	// Build OpenAI-compatible request (Groq uses the same format)
	groqReq := groqChatRequest{
		Model: req.Model,
	}

	if groqReq.Model == "" {
		groqReq.Model = p.config.Model
	}

	// Add system prompt
	if req.SystemPrompt != "" {
		groqReq.Messages = append(groqReq.Messages, groqMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}

	// Convert messages
	for _, msg := range req.Messages {
		groqReq.Messages = append(groqReq.Messages, groqMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Set parameters
	groqReq.MaxTokens = req.MaxTokens
	if groqReq.MaxTokens == 0 {
		groqReq.MaxTokens = p.config.MaxTokens
	}
	groqReq.Temperature = req.Temperature
	if groqReq.Temperature == 0 {
		groqReq.Temperature = p.config.Temperature
	}

	// Marshal request
	body, err := json.Marshal(groqReq)
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
		return nil, fmt.Errorf("Groq error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var groqResp groqChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&groqResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(groqResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := groqResp.Choices[0]
	return &ChatResponse{
		Content:          choice.Message.Content,
		Model:            groqResp.Model,
		PromptTokens:     groqResp.Usage.PromptTokens,
		CompletionTokens: groqResp.Usage.CompletionTokens,
		TokensUsed:       groqResp.Usage.TotalTokens,
		Duration:         time.Since(start),
		FinishReason:     choice.FinishReason,
	}, nil
}

// Groq API types (OpenAI-compatible)
type groqChatRequest struct {
	Model       string        `json:"model"`
	Messages    []groqMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

type groqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int         `json:"index"`
		Message      groqMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}
