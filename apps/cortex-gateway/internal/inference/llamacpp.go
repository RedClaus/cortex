package inference

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// LlamaCPPClient is a llama.cpp server client
type LlamaCPPClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewLlamaCPPClient creates a new llama.cpp client
func NewLlamaCPPClient(baseURL string) *LlamaCPPClient {
	return &LlamaCPPClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Infer sends an inference request to llama.cpp server
func (c *LlamaCPPClient) Infer(req *Request) (*Response, error) {
	llamaReq := map[string]interface{}{
		"prompt": req.Prompt,
		"n_predict": -1, // generate until stop
		"stream": false,
		"temperature": 0.7,
		"top_p": 0.9,
	}

	body, err := json.Marshal(llamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/completion", c.baseURL)
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
		return nil, fmt.Errorf("llama.cpp returned status %d: %s", resp.StatusCode, string(body))
	}

	var llamaResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&llamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	content, ok := llamaResp["content"].(string)
	if !ok {
		return nil, fmt.Errorf("no content in response")
	}

	return &Response{
		Content:   content,
		Model:     req.Model,
		TokensUsed: 0, // Not provided
		SessionID: req.SessionID,
	}, nil
}

// Health checks if llama.cpp server is healthy
func (c *LlamaCPPClient) Health() error {
	url := fmt.Sprintf("%s/health", c.baseURL) // or /completion with empty
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
		return fmt.Errorf("llama.cpp health check returned status %d", resp.StatusCode)
	}

	return nil
}
