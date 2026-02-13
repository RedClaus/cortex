package inference

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// TGIConfig holds TGI client configuration
type TGIConfig struct {
	BaseURL string
}

// TGIClient is a Text Generation Inference client
type TGIClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewTGIClient creates a new TGI client
func NewTGIClient(baseURL string) *TGIClient {
	return &TGIClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Infer sends an inference request to TGI
func (c *TGIClient) Infer(req *Request) (*Response, error) {
	tgiReq := map[string]interface{}{
		"inputs": req.Prompt,
		"parameters": map[string]interface{}{
			"max_new_tokens": 512,
			"do_sample":      false,
			"temperature":    0.7,
		},
	}

	body, err := json.Marshal(tgiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/generate", c.baseURL)
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
		return nil, fmt.Errorf("TGI returned status %d: %s", resp.StatusCode, string(body))
	}

	var tgiResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&tgiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	content, ok := tgiResp["generated_text"].(string)
	if !ok {
		return nil, fmt.Errorf("no generated_text in response")
	}

	return &Response{
		Content:   content,
		Model:     req.Model,
		TokensUsed: 0, // TGI doesn't return tokens easily
		SessionID: req.SessionID,
	}, nil
}

// Health checks if TGI is healthy
func (c *TGIClient) Health() error {
	url := fmt.Sprintf("%s/health", c.baseURL)
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
		return fmt.Errorf("TGI health check returned status %d", resp.StatusCode)
	}

	return nil
}
