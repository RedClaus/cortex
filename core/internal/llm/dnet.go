package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// DNetProvider implements the Provider interface for dnet distributed MLX inference.
// dnet exposes an OpenAI-compatible API at /v1/chat/completions.
type DNetProvider struct {
	config *ProviderConfig
	client *http.Client
}

// NewDNetProvider creates a new dnet provider.
// Default endpoint is http://localhost:9080 (dnet API server).
func NewDNetProvider(cfg *ProviderConfig) *DNetProvider {
	if cfg == nil {
		cfg = &ProviderConfig{
			Name:        "dnet",
			Endpoint:    "http://localhost:9080",
			Model:       "mlx-community/Llama-3.2-3B-Instruct-4bit",
			MaxTokens:   4096,
			Temperature: 0.7,
			Timeout:     5 * time.Minute,
		}
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = "http://localhost:9080"
	}
	if cfg.Model == "" {
		cfg.Model = "mlx-community/Llama-3.2-3B-Instruct-4bit"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Minute
	}
	cfg.Name = "dnet"

	return &DNetProvider{
		config: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// Name returns the provider identifier.
func (p *DNetProvider) Name() string {
	return "dnet"
}

// Available checks if dnet API is running.
func (p *DNetProvider) Available() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", p.config.Endpoint+"/health", nil)
	if err != nil {
		return false
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// Chat sends a chat request to dnet using OpenAI-compatible API.
func (p *DNetProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	start := time.Now()

	// Build OpenAI-compatible request
	openaiReq := dnetChatRequest{
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}

	if openaiReq.Model == "" {
		openaiReq.Model = p.config.Model
	}
	if openaiReq.MaxTokens == 0 {
		openaiReq.MaxTokens = p.config.MaxTokens
	}
	if openaiReq.Temperature == 0 {
		openaiReq.Temperature = p.config.Temperature
	}

	// Add system prompt as first message if provided
	if req.SystemPrompt != "" {
		openaiReq.Messages = append(openaiReq.Messages, dnetMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}

	// Convert messages
	for _, msg := range req.Messages {
		openaiReq.Messages = append(openaiReq.Messages, dnetMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Marshal request
	body, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.config.Endpoint+"/v1/chat/completions", bytes.NewReader(body))
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
		bodyBytes, _ := readLimitedBody(resp.Body, MaxErrorBodySize)
		return nil, fmt.Errorf("dnet error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var dnetResp dnetChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&dnetResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Extract content from response
	content := ""
	finishReason := "stop"
	if len(dnetResp.Choices) > 0 {
		content = dnetResp.Choices[0].Message.Content
		// Strip Llama special tokens that may leak through
		content = strings.TrimSuffix(content, "<|eot_id|>")
		content = strings.TrimSuffix(content, "<|end_of_text|>")
		content = strings.TrimSpace(content)
		if dnetResp.Choices[0].FinishReason != "" {
			finishReason = dnetResp.Choices[0].FinishReason
		}
	}

	return &ChatResponse{
		Content:          content,
		Model:            dnetResp.Model,
		PromptTokens:     dnetResp.Usage.PromptTokens,
		CompletionTokens: dnetResp.Usage.CompletionTokens,
		TokensUsed:       dnetResp.Usage.TotalTokens,
		Duration:         time.Since(start),
		FinishReason:     finishReason,
	}, nil
}

// GetLoadedModel returns the currently loaded model on the dnet cluster.
func (p *DNetProvider) GetLoadedModel(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.config.Endpoint+"/v1/topology", nil)
	if err != nil {
		return "", err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("no model loaded")
	}

	var topology struct {
		Model string `json:"model"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&topology); err != nil {
		return "", err
	}

	return topology.Model, nil
}

// ListModels returns the list of supported models on dnet.
func (p *DNetProvider) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.config.Endpoint+"/v1/models", nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list models")
	}

	var modelsResp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, err
	}

	models := make([]string, len(modelsResp.Data))
	for i, m := range modelsResp.Data {
		models[i] = m.ID
	}

	return models, nil
}

// dnet OpenAI-compatible API types
type dnetChatRequest struct {
	Model       string        `json:"model"`
	Messages    []dnetMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

type dnetMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type dnetChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// DnetModel represents a model available on the dnet distributed inference server.
type DnetModel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// FetchDnetModels fetches the list of available models from a dnet server.
// This is a standalone function that can be used without instantiating a provider.
func FetchDnetModels(endpoint string) ([]DnetModel, error) {
	if endpoint == "" {
		endpoint = "http://127.0.0.1:9080"
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(endpoint + "/v1/models")
	if err != nil {
		return nil, fmt.Errorf("dnet server not reachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("dnet server returned status %d", resp.StatusCode)
	}

	var modelsResp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to parse dnet models response: %w", err)
	}

	models := make([]DnetModel, len(modelsResp.Data))
	for i, m := range modelsResp.Data {
		// Extract display name from ID
		name := m.ID
		if idx := strings.LastIndex(m.ID, "/"); idx >= 0 {
			name = m.ID[idx+1:]
		}
		models[i] = DnetModel{
			ID:   m.ID,
			Name: name,
		}
	}

	return models, nil
}
