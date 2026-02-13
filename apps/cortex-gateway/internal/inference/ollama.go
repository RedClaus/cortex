package inference

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OllamaConfig holds Ollama client configuration
type OllamaConfig struct {
	URL          string
	DefaultModel string
}

// OllamaClient is an Ollama inference client
type OllamaClient struct {
	baseURL     string
	defaultModel string
	httpClient  *http.Client
}

// NewOllamaClient creates a new Ollama client
func NewOllamaClient(cfg *OllamaConfig) (*OllamaClient, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("ollama URL is required")
	}

	return &OllamaClient{
		baseURL:     cfg.URL,
		defaultModel: cfg.DefaultModel,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}, nil
}

// Infer sends an inference request to Ollama
func (c *OllamaClient) Infer(req *Request) (*Response, error) {
	model := req.Model
	if model == "" {
		model = c.defaultModel
	}

	ollamaReq := map[string]interface{}{
		"model":  model,
		"prompt": req.Prompt,
		"stream": false,
	}

	if req.Options != nil {
		ollamaReq["options"] = req.Options
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/generate", c.baseURL)
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var ollamaResp OllamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &Response{
		Content:    ollamaResp.Response,
		Model:      ollamaResp.Model,
		TokensUsed: ollamaResp.EvalCount,
		SessionID:  req.SessionID,
	}, nil
}

// Health checks if Ollama is healthy
func (c *OllamaClient) Health() error {
	url := fmt.Sprintf("%s/api/tags", c.baseURL)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama health check returned status %d", resp.StatusCode)
	}

	return nil
}

// OllamaResponse represents an Ollama API response
type OllamaResponse struct {
	Model     string `json:"model"`
	Response  string `json:"response"`
	Done      bool   `json:"done"`
	PromptCount int  `json:"prompt_eval_count"`
	EvalCount int   `json:"eval_count"`
}
