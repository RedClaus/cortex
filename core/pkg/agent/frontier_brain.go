package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// FrontierBrain wraps frontier AI APIs (Claude, OpenAI) as a BrainInterface.
type FrontierBrain struct {
	provider   string       // "anthropic" or "openai"
	model      string       // e.g., "claude-sonnet-4-20250514"
	apiKey     string
	baseURL    string
	client     *http.Client
	maxRetries int
}

// FrontierConfig holds configuration for FrontierBrain.
type FrontierConfig struct {
	Provider   string // "anthropic" or "openai"
	Model      string // Model name
	APIKey     string // API key (or uses env var)
	BaseURL    string // Optional custom base URL
	TimeoutSec int    // Request timeout in seconds
}

// NewFrontierBrain creates a new FrontierBrain instance.
func NewFrontierBrain(cfg FrontierConfig) (*FrontierBrain, error) {
	apiKey := cfg.APIKey
	if apiKey == "" {
		// Try environment variables
		switch cfg.Provider {
		case "anthropic":
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
		case "openai":
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
	}

	if apiKey == "" {
		return nil, fmt.Errorf("no API key provided for %s", cfg.Provider)
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		switch cfg.Provider {
		case "anthropic":
			baseURL = "https://api.anthropic.com"
		case "openai":
			baseURL = "https://api.openai.com"
		}
	}

	timeout := cfg.TimeoutSec
	if timeout <= 0 {
		timeout = 60
	}

	return &FrontierBrain{
		provider:   cfg.Provider,
		model:      cfg.Model,
		apiKey:     apiKey,
		baseURL:    baseURL,
		client:     &http.Client{Timeout: time.Duration(timeout) * time.Second},
		maxRetries: 2,
	}, nil
}

// Type returns "frontier".
func (f *FrontierBrain) Type() string {
	return "frontier"
}

// Available checks if the API is reachable.
func (f *FrontierBrain) Available() bool {
	return f.apiKey != ""
}

// Process sends a request to the frontier API and returns the result.
func (f *FrontierBrain) Process(ctx context.Context, input *BrainInput) (*BrainResult, error) {
	startTime := time.Now()

	result := &BrainResult{
		Source: fmt.Sprintf("frontier:%s", f.provider),
		Model:  f.model,
	}

	var response string
	var tokensUsed int
	var err error

	switch f.provider {
	case "anthropic":
		response, tokensUsed, err = f.callAnthropic(ctx, input)
	case "openai":
		response, tokensUsed, err = f.callOpenAI(ctx, input)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", f.provider)
	}

	result.Latency = time.Since(startTime)
	result.TokensUsed = tokensUsed

	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return result, err
	}

	result.Content = response
	result.Success = true
	result.Confidence = 0.95 // Frontier models are generally high confidence

	return result, nil
}

// AnthropicRequest represents a Claude API request.
type AnthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	System      string             `json:"system,omitempty"`
	Messages    []AnthropicMessage `json:"messages"`
	Temperature float64            `json:"temperature,omitempty"`
}

// AnthropicMessage represents a message in Claude format.
type AnthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AnthropicResponse represents a Claude API response.
type AnthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// callAnthropic makes a request to the Claude API.
func (f *FrontierBrain) callAnthropic(ctx context.Context, input *BrainInput) (string, int, error) {
	// Build messages
	messages := make([]AnthropicMessage, 0)
	for _, msg := range input.ConversationHistory {
		messages = append(messages, AnthropicMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}
	// Add current query as user message
	messages = append(messages, AnthropicMessage{
		Role:    "user",
		Content: input.Query,
	})

	maxTokens := input.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	temperature := input.Temperature
	if temperature <= 0 {
		temperature = 0.7
	}

	reqBody := AnthropicRequest{
		Model:       f.model,
		MaxTokens:   maxTokens,
		System:      input.SystemPrompt,
		Messages:    messages,
		Temperature: temperature,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", f.baseURL+"/v1/messages", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", f.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := f.client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var apiResp AnthropicResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", 0, fmt.Errorf("failed to parse response: %w", err)
	}

	if apiResp.Error != nil {
		return "", 0, fmt.Errorf("API error: %s", apiResp.Error.Message)
	}

	// Extract text content
	var content string
	for _, c := range apiResp.Content {
		if c.Type == "text" {
			content += c.Text
		}
	}

	tokensUsed := apiResp.Usage.InputTokens + apiResp.Usage.OutputTokens
	return content, tokensUsed, nil
}

// callOpenAI makes a request to the OpenAI API.
func (f *FrontierBrain) callOpenAI(ctx context.Context, input *BrainInput) (string, int, error) {
	// Build messages
	messages := make([]map[string]string, 0)

	// Add system prompt if provided
	if input.SystemPrompt != "" {
		messages = append(messages, map[string]string{
			"role":    "system",
			"content": input.SystemPrompt,
		})
	}

	// Add conversation history
	for _, msg := range input.ConversationHistory {
		messages = append(messages, map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}

	// Add current query
	messages = append(messages, map[string]string{
		"role":    "user",
		"content": input.Query,
	})

	maxTokens := input.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	temperature := input.Temperature
	if temperature <= 0 {
		temperature = 0.7
	}

	reqBody := map[string]interface{}{
		"model":       f.model,
		"messages":    messages,
		"max_tokens":  maxTokens,
		"temperature": temperature,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", f.baseURL+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+f.apiKey)

	resp, err := f.client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", 0, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return "", 0, fmt.Errorf("no response choices returned")
	}

	return apiResp.Choices[0].Message.Content, apiResp.Usage.TotalTokens, nil
}
