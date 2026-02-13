// Package brain provides RemoteBrain implementation for connecting to CortexBrain servers.
package brain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/normanking/pinky/internal/config"
	"github.com/normanking/pinky/internal/persona"
)

// RemoteBrain implements Brain by connecting to a remote CortexBrain server.
type RemoteBrain struct {
	cfg    config.BrainConfig
	client *http.Client
}

// NewRemoteBrain creates a new remote brain client.
func NewRemoteBrain(cfg config.BrainConfig) *RemoteBrain {
	return &RemoteBrain{
		cfg: cfg,
		client: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// Mode returns ModeRemote.
func (b *RemoteBrain) Mode() BrainMode {
	return ModeRemote
}

// Ping checks if the remote brain server is reachable.
func (b *RemoteBrain) Ping(ctx context.Context) error {
	url := b.cfg.RemoteURL + "/health"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	b.setAuthHeader(req)

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("remote brain unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("remote brain unhealthy: status %d", resp.StatusCode)
	}

	return nil
}

// remoteThinkRequest is the request format for the CortexBrain API.
type remoteThinkRequest struct {
	UserID      string        `json:"user_id,omitempty"`
	Persona     *persona.Persona `json:"persona,omitempty"`
	Messages    []Message     `json:"messages"`
	Memories    []Memory      `json:"memories,omitempty"`
	Tools       []ToolSpec    `json:"tools,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream"`
}

// remoteThinkResponse is the response format from the CortexBrain API.
type remoteThinkResponse struct {
	Content   string      `json:"content"`
	ToolCalls []ToolCall  `json:"tool_calls,omitempty"`
	Reasoning string      `json:"reasoning,omitempty"`
	Usage     TokenUsage  `json:"usage"`
	Done      bool        `json:"done"`
	Error     string      `json:"error,omitempty"`
}

// Think processes a request and returns a response.
func (b *RemoteBrain) Think(ctx context.Context, req *ThinkRequest) (*ThinkResponse, error) {
	url := b.cfg.RemoteURL + "/v1/think"

	remoteReq := remoteThinkRequest{
		UserID:      req.UserID,
		Persona:     req.Persona,
		Messages:    req.Messages,
		Memories:    req.Memories,
		Tools:       req.Tools,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      false,
	}

	body, err := json.Marshal(remoteReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	b.setAuthHeader(httpReq)

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("remote brain request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("remote brain error %d: %s", resp.StatusCode, string(respBody))
	}

	var remoteResp remoteThinkResponse
	if err := json.NewDecoder(resp.Body).Decode(&remoteResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if remoteResp.Error != "" {
		return nil, fmt.Errorf("remote brain error: %s", remoteResp.Error)
	}

	return &ThinkResponse{
		Content:   remoteResp.Content,
		ToolCalls: remoteResp.ToolCalls,
		Reasoning: remoteResp.Reasoning,
		Usage:     remoteResp.Usage,
		Done:      true,
	}, nil
}

// ThinkStream returns a channel of response chunks for streaming.
func (b *RemoteBrain) ThinkStream(ctx context.Context, req *ThinkRequest) (<-chan *ThinkChunk, error) {
	url := b.cfg.RemoteURL + "/v1/think"

	remoteReq := remoteThinkRequest{
		UserID:      req.UserID,
		Persona:     req.Persona,
		Messages:    req.Messages,
		Memories:    req.Memories,
		Tools:       req.Tools,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      true,
	}

	body, err := json.Marshal(remoteReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	b.setAuthHeader(httpReq)

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("remote brain request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("remote brain error %d: %s", resp.StatusCode, string(respBody))
	}

	chunks := make(chan *ThinkChunk, 100)

	go func() {
		defer resp.Body.Close()
		defer close(chunks)

		decoder := json.NewDecoder(resp.Body)
		for {
			var chunk remoteThinkResponse
			if err := decoder.Decode(&chunk); err != nil {
				if err != io.EOF {
					chunks <- &ThinkChunk{Error: err}
				}
				return
			}

			if chunk.Error != "" {
				chunks <- &ThinkChunk{Error: fmt.Errorf("remote brain: %s", chunk.Error)}
				return
			}

			chunks <- &ThinkChunk{
				Content:   chunk.Content,
				ToolCalls: chunk.ToolCalls,
				Done:      chunk.Done,
			}

			if chunk.Done {
				return
			}
		}
	}()

	return chunks, nil
}

// remoteMemoryRequest is the request format for memory operations.
type remoteMemoryRequest struct {
	Memory *Memory `json:"memory,omitempty"`
	Query  string  `json:"query,omitempty"`
	Limit  int     `json:"limit,omitempty"`
}

// remoteMemoryResponse is the response format for memory operations.
type remoteMemoryResponse struct {
	Memories []Memory `json:"memories,omitempty"`
	Error    string   `json:"error,omitempty"`
}

// Remember stores a memory on the remote brain.
func (b *RemoteBrain) Remember(ctx context.Context, memory *Memory) error {
	url := b.cfg.RemoteURL + "/v1/memory"

	reqBody := remoteMemoryRequest{Memory: memory}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	b.setAuthHeader(httpReq)

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("remote brain request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("remote brain error %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// Recall retrieves relevant memories from the remote brain.
func (b *RemoteBrain) Recall(ctx context.Context, query string, limit int) ([]Memory, error) {
	url := b.cfg.RemoteURL + "/v1/memory/search"

	reqBody := remoteMemoryRequest{Query: query, Limit: limit}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	b.setAuthHeader(httpReq)

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("remote brain request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("remote brain error %d: %s", resp.StatusCode, string(respBody))
	}

	var memResp remoteMemoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&memResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if memResp.Error != "" {
		return nil, fmt.Errorf("remote brain error: %s", memResp.Error)
	}

	return memResp.Memories, nil
}

// setAuthHeader adds authentication header if token is configured.
func (b *RemoteBrain) setAuthHeader(req *http.Request) {
	if b.cfg.RemoteToken != "" {
		req.Header.Set("Authorization", "Bearer "+b.cfg.RemoteToken)
	}
}
