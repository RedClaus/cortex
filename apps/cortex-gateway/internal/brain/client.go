package brain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cortexhub/cortex-gateway/internal/config"
)

// Client represents a CortexBrain REST client
type Client struct {
	baseURL    string
	jwtSecret  string
	httpClient *http.Client
}

// NewClient creates a new CortexBrain client
func NewClient(cfg *config.CortexBrainConfig) *Client {
	return &Client{
		baseURL:   cfg.URL,
		jwtSecret: cfg.JWTSecret,
		httpClient: &http.Client{
			Timeout: cfg.GetTimeout(),
		},
	}
}

// HealthResponse represents a health check response from CortexBrain
type HealthResponse struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
}

// InferRequest represents an inference request to CortexBrain
type InferRequest struct {
	Prompt     string                 `json:"prompt"`
	SessionID  string                 `json:"session_id,omitempty"`
	Model      string                 `json:"model,omitempty"`
	Options    map[string]interface{} `json:"options,omitempty"`
}

// InferResponse represents an inference response from CortexBrain
type InferResponse struct {
	Content    string `json:"content"`
	Model      string `json:"model"`
	TokensUsed int    `json:"tokens_used,omitempty"`
	SessionID  string `json:"session_id,omitempty"`
}

// StoreMemoryRequest represents a memory storage request
type StoreMemoryRequest struct {
	Content     string                 `json:"content"`
	AgentID     string                 `json:"agent_id"`
	SessionID   string                 `json:"session_id,omitempty"`
	Importance  float64                `json:"importance,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// RecallMemoryRequest represents a memory recall request
type RecallMemoryRequest struct {
	Query      string                 `json:"query"`
	AgentID    string                 `json:"agent_id"`
	SessionID  string                 `json:"session_id,omitempty"`
	Limit      int                    `json:"limit,omitempty"`
	Threshold  float64                `json:"threshold,omitempty"`
}

// RecallMemoryResponse represents memory recall results
type RecallMemoryResponse struct {
	Results []MemoryEntry `json:"results"`
	Query   string        `json:"query"`
}

// MemoryEntry represents a retrieved memory entry
type MemoryEntry struct {
	ID        string                 `json:"id"`
	Content   string                 `json:"content"`
	AgentID   string                 `json:"agent_id"`
	SessionID string                 `json:"session_id,omitempty"`
	Score     float64                `json:"score"`
	Timestamp string                 `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// BlackboardReadRequest represents a blackboard read request
type BlackboardReadRequest struct {
	Key string `json:"key"`
}

// BlackboardReadResponse represents blackboard read response
type BlackboardReadResponse struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

// BlackboardWriteRequest represents a blackboard write request
type BlackboardWriteRequest struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

// NeuralBusEvent represents a neural bus event
type NeuralBusEvent struct {
	EventType string                 `json:"event_type"`
	Payload   map[string]interface{} `json:"payload"`
	Timestamp string                 `json:"timestamp"`
}

// PublishEventRequest represents a publish event request
type PublishEventRequest struct {
	Event NeuralBusEvent `json:"event"`
}

// SleepCycleTriggerRequest represents a sleep cycle trigger request
type SleepCycleTriggerRequest struct {
	Force bool `json:"force,omitempty"`
}

// Health checks if CortexBrain is healthy
func (c *Client) Health() (*HealthResponse, error) {
	resp, err := c.doRequest(http.MethodGet, "/health", nil)
	if err != nil {
		return nil, fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	var healthResp HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&healthResp); err != nil {
		return nil, fmt.Errorf("failed to decode health response: %w", err)
	}

	return &healthResp, nil
}

// Infer sends an inference request to CortexBrain
func (c *Client) Infer(req *InferRequest) (*InferResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal inference request: %w", err)
	}

	resp, err := c.doRequest(http.MethodPost, "/api/v1/infer", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("inference request failed: %w", err)
	}
	defer resp.Body.Close()

	var inferResp InferResponse
	if err := json.NewDecoder(resp.Body).Decode(&inferResp); err != nil {
		return nil, fmt.Errorf("failed to decode inference response: %w", err)
	}

	return &inferResp, nil
}

// StoreMemory stores a memory in CortexBrain
func (c *Client) StoreMemory(req *StoreMemoryRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal store memory request: %w", err)
	}

	resp, err := c.doRequest(http.MethodPost, "/api/v1/memory", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("store memory request failed: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

// RecallMemory recalls memories from CortexBrain
func (c *Client) RecallMemory(req *RecallMemoryRequest) (*RecallMemoryResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal recall memory request: %w", err)
	}

	resp, err := c.doRequest(http.MethodPost, "/api/v1/memory/recall", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("recall memory request failed: %w", err)
	}
	defer resp.Body.Close()

	var recallResp RecallMemoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&recallResp); err != nil {
		return nil, fmt.Errorf("failed to decode recall memory response: %w", err)
	}

	return &recallResp, nil
}

// ReadBlackboard reads from CortexBrain's Blackboard
func (c *Client) ReadBlackboard(key string) (*BlackboardReadResponse, error) {
	req := BlackboardReadRequest{Key: key}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal blackboard read request: %w", err)
	}

	resp, err := c.doRequest(http.MethodPost, "/api/v1/blackboard/read", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("blackboard read request failed: %w", err)
	}
	defer resp.Body.Close()

	var readResp BlackboardReadResponse
	if err := json.NewDecoder(resp.Body).Decode(&readResp); err != nil {
		return nil, fmt.Errorf("failed to decode blackboard read response: %w", err)
	}

	return &readResp, nil
}

// WriteBlackboard writes to CortexBrain's Blackboard
func (c *Client) WriteBlackboard(req *BlackboardWriteRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal blackboard write request: %w", err)
	}

	resp, err := c.doRequest(http.MethodPost, "/api/v1/blackboard/write", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("blackboard write request failed: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

// PublishEvent publishes an event to the Neural Bus
func (c *Client) PublishEvent(event *NeuralBusEvent) error {
	req := PublishEventRequest{Event: *event}
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal publish event request: %w", err)
	}

	resp, err := c.doRequest(http.MethodPost, "/api/v1/bus/publish", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("publish event request failed: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

// TriggerSleepCycle triggers CortexBrain's Sleep Cycle
func (c *Client) TriggerSleepCycle(force bool) error {
	req := SleepCycleTriggerRequest{Force: force}
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal sleep cycle trigger request: %w", err)
	}

	resp, err := c.doRequest(http.MethodPost, "/api/v1/sleep/trigger", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("sleep cycle trigger request failed: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

// doRequest performs an HTTP request with proper headers
func (c *Client) doRequest(method, path string, body io.Reader) (*http.Response, error) {
	url := c.baseURL + path
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.jwtSecret)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Cortex-Gateway/1.0.0")

	return c.httpClient.Do(req)
}