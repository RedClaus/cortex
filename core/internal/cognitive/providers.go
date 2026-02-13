package cognitive

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OllamaProvider adapts Ollama for the cognitive pipeline (standalone implementation).
type OllamaProvider struct {
	endpoint string
	model    string
	client   *http.Client
}

// NewOllamaProvider creates a new Ollama provider adapter.
// Uses longer timeouts to handle cold start (model loading) scenarios.
func NewOllamaProvider(endpoint, model string) *OllamaProvider {
	if endpoint == "" {
		endpoint = "http://127.0.0.1:11434"
	}
	if model == "" {
		model = "llama3.2:1b"
	}
	return &OllamaProvider{
		endpoint: endpoint,
		model:    model,
		client: &http.Client{
			// Use Transport-level timeouts instead of http.Client.Timeout
			// to allow streaming responses to work properly during cold starts.
			// http.Client.Timeout applies to the ENTIRE request including body reading,
			// which causes timeouts on streaming responses during model loading.
			Transport: &http.Transport{
				ResponseHeaderTimeout: 120 * time.Second, // Allow time for model loading
				IdleConnTimeout:       90 * time.Second,  // Keep-alive idle timeout
				TLSHandshakeTimeout:   10 * time.Second,  // TLS negotiation
			},
		},
	}
}

// Complete implements the LLMProvider interface for the cognitive pipeline.
func (p *OllamaProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	// Build Ollama request
	ollamaReq := struct {
		Model    string          `json:"model"`
		Messages []ollamaMessage `json:"messages"`
		Stream   bool            `json:"stream"`
		Options  struct {
			Temperature float64 `json:"temperature,omitempty"`
			NumPredict  int     `json:"num_predict,omitempty"`
		} `json:"options,omitempty"`
	}{
		Model:  p.model,
		Stream: false,
	}

	if req.Model != "" {
		ollamaReq.Model = req.Model
	}

	// Convert messages
	for _, msg := range req.Messages {
		ollamaReq.Messages = append(ollamaReq.Messages, ollamaMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Set options
	ollamaReq.Options.Temperature = req.Temperature
	if ollamaReq.Options.Temperature == 0 {
		ollamaReq.Options.Temperature = 0.7
	}
	ollamaReq.Options.NumPredict = req.MaxTokens
	if ollamaReq.Options.NumPredict == 0 {
		ollamaReq.Options.NumPredict = 2000
	}

	// Marshal request
	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.endpoint+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var ollamaResp struct {
		Model           string        `json:"model"`
		Message         ollamaMessage `json:"message"`
		Done            bool          `json:"done"`
		PromptEvalCount int           `json:"prompt_eval_count"`
		EvalCount       int           `json:"eval_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &CompletionResponse{
		Content:    ollamaResp.Message.Content,
		TokensUsed: ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
		Model:      ollamaResp.Model,
	}, nil
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ClaudeProvider adapts Anthropic Claude for the cognitive pipeline (standalone implementation).
type ClaudeProvider struct {
	apiKey string
	model  string
	client *http.Client
}

// NewClaudeProvider creates a new Claude provider adapter.
func NewClaudeProvider(apiKey, model string) *ClaudeProvider {
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}
	return &ClaudeProvider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Complete implements the LLMProvider interface for the cognitive pipeline.
func (p *ClaudeProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	// Build Anthropic request
	anthropicReq := struct {
		Model       string             `json:"model"`
		Messages    []anthropicMessage `json:"messages"`
		System      string             `json:"system,omitempty"`
		MaxTokens   int                `json:"max_tokens"`
		Temperature float64            `json:"temperature,omitempty"`
	}{
		Model:       p.model,
		MaxTokens:   2000,
		Temperature: 0.7,
	}

	if req.Model != "" {
		anthropicReq.Model = req.Model
	}
	if req.MaxTokens > 0 {
		anthropicReq.MaxTokens = req.MaxTokens
	}
	if req.Temperature > 0 {
		anthropicReq.Temperature = req.Temperature
	}

	// Extract system prompt and convert messages
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			anthropicReq.System = msg.Content
		} else {
			anthropicReq.Messages = append(anthropicReq.Messages, anthropicMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	// Marshal request
	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	// Execute request
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var anthropicResp struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Role    string `json:"role"`
		Model   string `json:"model"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
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

	return &CompletionResponse{
		Content:    content,
		TokensUsed: anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
		Model:      anthropicResp.Model,
	}, nil
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
