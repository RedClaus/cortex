package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// MLXProvider implements the Provider interface for mlx-lm local inference.
// mlx-lm exposes an OpenAI-compatible API at /v1/chat/completions.
// It provides 5-10x faster inference than Ollama on Apple Silicon.
type MLXProvider struct {
	config *ProviderConfig
	client *http.Client
}

// NewMLXProvider creates a new mlx-lm provider.
// Default endpoint is http://localhost:8081 (avoids conflict with A2A on 8080).
func NewMLXProvider(cfg *ProviderConfig) *MLXProvider {
	if cfg == nil {
		cfg = &ProviderConfig{
			Name:        "mlx",
			Endpoint:    "http://localhost:8081",
			Model:       "mlx-community/Llama-3.2-3B-Instruct-4bit",
			MaxTokens:   4096,
			Temperature: 0.7,
			Timeout:     5 * time.Minute,
		}
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = "http://localhost:8081"
	}
	if cfg.Model == "" {
		cfg.Model = "mlx-community/Llama-3.2-3B-Instruct-4bit"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Minute
	}
	cfg.Name = "mlx"

	return &MLXProvider{
		config: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// Name returns the provider identifier.
func (p *MLXProvider) Name() string {
	return "mlx"
}

// Available checks if mlx-lm server is running.
func (p *MLXProvider) Available() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// mlx-lm doesn't have a /health endpoint, check /v1/models instead
	req, err := http.NewRequestWithContext(ctx, "GET", p.config.Endpoint+"/v1/models", nil)
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

// Chat sends a chat request to mlx-lm using OpenAI-compatible API.
func (p *MLXProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	start := time.Now()

	// Build OpenAI-compatible request
	openaiReq := mlxChatRequest{
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
		openaiReq.Messages = append(openaiReq.Messages, mlxMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}

	// Convert messages
	for _, msg := range req.Messages {
		openaiReq.Messages = append(openaiReq.Messages, mlxMessage{
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
		return nil, fmt.Errorf("mlx error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var mlxResp mlxChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&mlxResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Extract content from response
	content := ""
	finishReason := "stop"
	if len(mlxResp.Choices) > 0 {
		content = mlxResp.Choices[0].Message.Content
		// Strip Llama special tokens that may leak through
		content = strings.TrimSuffix(content, "<|eot_id|>")
		content = strings.TrimSuffix(content, "<|end_of_text|>")
		content = strings.TrimSpace(content)
		if mlxResp.Choices[0].FinishReason != "" {
			finishReason = mlxResp.Choices[0].FinishReason
		}
	}

	return &ChatResponse{
		Content:          content,
		Model:            mlxResp.Model,
		PromptTokens:     mlxResp.Usage.PromptTokens,
		CompletionTokens: mlxResp.Usage.CompletionTokens,
		TokensUsed:       mlxResp.Usage.TotalTokens,
		Duration:         time.Since(start),
		FinishReason:     finishReason,
	}, nil
}

// ChatStream implements StreamingProvider for real-time token streaming.
// Uses OpenAI-compatible SSE streaming from mlx-lm server.
func (p *MLXProvider) ChatStream(ctx context.Context, req *ChatRequest, onToken func(token string)) (string, error) {
	// Build OpenAI-compatible request with streaming enabled
	openaiReq := mlxChatRequest{
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      true, // Enable streaming
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
		openaiReq.Messages = append(openaiReq.Messages, mlxMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}

	// Convert messages
	for _, msg := range req.Messages {
		openaiReq.Messages = append(openaiReq.Messages, mlxMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Marshal request
	body, err := json.Marshal(openaiReq)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.config.Endpoint+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	// Execute request
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := readLimitedBody(resp.Body, MaxErrorBodySize)
		return "", fmt.Errorf("mlx error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// Read SSE stream
	var fullContent strings.Builder
	reader := bufio.NewReader(resp.Body)

	for {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return fullContent.String(), ctx.Err()
		default:
		}

		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return fullContent.String(), fmt.Errorf("read stream: %w", err)
		}

		lineStr := strings.TrimSpace(string(line))
		if lineStr == "" {
			continue
		}

		// SSE format: "data: {...}"
		if !strings.HasPrefix(lineStr, "data: ") {
			continue
		}

		data := strings.TrimPrefix(lineStr, "data: ")
		if data == "[DONE]" {
			break
		}

		// Parse streaming chunk
		var chunk mlxStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // Skip malformed chunks
		}

		// Extract delta content
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			token := chunk.Choices[0].Delta.Content
			fullContent.WriteString(token)
			if onToken != nil {
				onToken(token)
			}
		}

		// Check for finish
		if len(chunk.Choices) > 0 && chunk.Choices[0].FinishReason != "" {
			break
		}
	}

	// Clean up Llama special tokens
	content := fullContent.String()
	content = strings.TrimSuffix(content, "<|eot_id|>")
	content = strings.TrimSuffix(content, "<|end_of_text|>")
	content = strings.TrimSpace(content)

	return content, nil
}

// mlxStreamChunk represents a streaming response chunk from mlx-lm
type mlxStreamChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role    string `json:"role,omitempty"`
			Content string `json:"content,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason,omitempty"`
	} `json:"choices"`
}

// ListModels returns the list of available models on mlx-lm.
func (p *MLXProvider) ListModels(ctx context.Context) ([]string, error) {
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

// mlx-lm OpenAI-compatible API types
type mlxChatRequest struct {
	Model       string       `json:"model"`
	Messages    []mlxMessage `json:"messages"`
	MaxTokens   int          `json:"max_tokens,omitempty"`
	Temperature float64      `json:"temperature,omitempty"`
	Stream      bool         `json:"stream,omitempty"`
}

type mlxMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type mlxChatResponse struct {
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

// MLXModel represents a model available on the MLX-LM server.
type MLXModel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// FetchMLXModels fetches the list of available models from an MLX-LM server.
// This is a standalone function that can be used without instantiating a provider.
func FetchMLXModels(endpoint string) ([]MLXModel, error) {
	if endpoint == "" {
		endpoint = "http://127.0.0.1:8081"
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(endpoint + "/v1/models")
	if err != nil {
		return nil, fmt.Errorf("MLX server not reachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MLX server returned status %d", resp.StatusCode)
	}

	var modelsResp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to parse MLX models response: %w", err)
	}

	models := make([]MLXModel, len(modelsResp.Data))
	for i, m := range modelsResp.Data {
		// Extract display name from ID (e.g., "mlx-community/Llama-3.2-3B" -> "Llama-3.2-3B")
		name := m.ID
		if idx := strings.LastIndex(m.ID, "/"); idx >= 0 {
			name = m.ID[idx+1:]
		}
		models[i] = MLXModel{
			ID:   m.ID,
			Name: name,
		}
	}

	return models, nil
}
