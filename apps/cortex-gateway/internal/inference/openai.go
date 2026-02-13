package inference

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAIConfig holds OpenAI-compatible client configuration
type OpenAIConfig struct {
	BaseURL string
	APIKey  string
	Model   string
}

// OpenAIClient is an OpenAI-compatible inference client
type OpenAIClient struct {
	baseURL    string
	apiKey     string
	defaultModel string
	httpClient *http.Client
}

// NewOpenAIClient creates a new OpenAI-compatible client
func NewOpenAIClient(cfg *OpenAIConfig) (*OpenAIClient, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("base URL is required")
	}

	return &OpenAIClient{
		baseURL:     cfg.BaseURL,
		apiKey:      cfg.APIKey,
		defaultModel: cfg.Model,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}, nil
}

// Infer sends an inference request to OpenAI-compatible API
func (c *OpenAIClient) Infer(req *Request) (*Response, error) {
	model := req.Model
	if model == "" {
		model = c.defaultModel
	}

	openaiReq := OpenAIRequest{
		Model: model,
		Messages: []Message{
			{Role: "user", Content: req.Prompt},
		},
	}

	body, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/chat/completions", c.baseURL)
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai API returned status %d: %s", resp.StatusCode, string(body))
	}

	var openaiResp OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(openaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &Response{
		Content:    openaiResp.Choices[0].Message.Content,
		Model:      openaiResp.Model,
		TokensUsed: openaiResp.Usage.TotalTokens,
		SessionID:  req.SessionID,
	}, nil
}

// Health checks if OpenAI-compatible API is healthy
func (c *OpenAIClient) Health() error {
	if c.apiKey == "" {
		return fmt.Errorf("API key is not configured")
	}
	return nil
}

// OpenAIRequest represents an OpenAI API request
type OpenAIRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIResponse represents an OpenAI API response
type OpenAIResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents a completion choice
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage represents token usage
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
