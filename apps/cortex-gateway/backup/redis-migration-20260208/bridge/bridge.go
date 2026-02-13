package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cortexhub/cortex-gateway/internal/config"
	"github.com/cortexhub/cortex-gateway/internal/discovery"
)

// Client represents the A2A Bridge client
type Client struct {
	baseURL    string
	disc       *discovery.Discovery
	agentCard  AgentCard
	httpClient *http.Client
	stopCh     chan struct{}
}

// AgentCard represents the agent registration card
type AgentCard struct {
	Name         string            `json:"name"`
	Capabilities []string          `json:"capabilities"`
	Metadata     map[string]string `json:"metadata"`
}

// TaskAssignment represents a task assignment from the bridge
type TaskAssignment struct {
	ID          string                 `json:"id"`
	Description string                 `json:"description"`
	Payload     map[string]interface{} `json:"payload"`
	AssignedAt  time.Time              `json:"assigned_at"`
}

// TaskCompletion represents a task completion report
type TaskCompletion struct {
	TaskID      string                 `json:"task_id"`
	Status      string                 `json:"status"`
	Result      map[string]interface{} `json:"result,omitempty"`
	CompletedAt time.Time              `json:"completed_at"`
}

// RegisterRequest represents the registration request
type RegisterRequest struct {
	Agent AgentCard `json:"agent"`
}

// HeartbeatRequest represents the heartbeat request
type HeartbeatRequest struct {
	AgentName string    `json:"agent_name"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

// NewClient creates a new bridge client
func NewClient(disc *discovery.Discovery, cfg *config.BridgeConfig) *Client {
	return &Client{
		disc:        disc,
		baseURL:     cfg.URL,
		agentCard: AgentCard{
			Name:         "cortex-gateway",
			Capabilities: []string{"chat", "tool_execution", "memory_search"},
			Metadata: map[string]string{
				"version": "1.0.0",
				"type":    "gateway",
			},
		},
		httpClient: &http.Client{Timeout: 30 * time.Second},
		stopCh:     make(chan struct{}),
	}
}

// Start starts the bridge integration
func (c *Client) Start(ctx context.Context) error {
	url, err := c.getBridgeURL(ctx)
	if err != nil {
		return fmt.Errorf("failed to resolve bridge URL: %w", err)
	}
	c.baseURL = url

	// Register with bridge
	if err := c.register(); err != nil {
		return fmt.Errorf("failed to register with bridge: %w", err)
	}

	// Start heartbeat goroutine
	go c.heartbeatLoop(ctx)

	// TODO: Start task polling or WS listener
	// For now, assume tasks are handled via other means

	return nil
}

// Stop stops the bridge integration
func (c *Client) Stop() error {
	close(c.stopCh)
	return nil
}

// getBridgeURL gets the bridge URL using discovery or fallback
func (c *Client) getBridgeURL(ctx context.Context) (string, error) {
	if c.disc == nil {
		return c.baseURL, nil
	}
	return c.disc.ServiceURL("harold", "bridge")
}

// register registers the gateway with the bridge
func (c *Client) register() error {
	req := RegisterRequest{Agent: c.agentCard}
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	resp, err := c.doRequest(http.MethodPost, "/register", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("registration failed with status %d", resp.StatusCode)
	}

	return nil
}

// heartbeatLoop sends periodic heartbeats
func (c *Client) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.sendHeartbeat()
		}
	}
}

// sendHeartbeat sends a heartbeat to the bridge
func (c *Client) sendHeartbeat() {
	req := HeartbeatRequest{
		AgentName: c.agentCard.Name,
		Status:    "healthy",
		Timestamp: time.Now(),
	}
	body, err := json.Marshal(req)
	if err != nil {
		// Log error
		return
	}

	resp, err := c.doRequest(http.MethodPost, "/heartbeat", bytes.NewReader(body))
	if err != nil {
		// Log error
		return
	}
	defer resp.Body.Close()
}

// ReportCompletion reports task completion to the bridge
func (c *Client) ReportCompletion(completion *TaskCompletion) error {
	body, err := json.Marshal(completion)
	if err != nil {
		return err
	}

	resp, err := c.doRequest(http.MethodPost, "/task/complete", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// SendMessage sends a message via the bridge
func (c *Client) SendMessage(ctx context.Context, from, to, msgType, content string) error {
	if c.baseURL == "" {
		url, err := c.getBridgeURL(ctx)
		if err != nil {
			return err
		}
		c.baseURL = url
	}

	bodyMap := map[string]interface{}{
		"from":    from,
		"to":      to,
		"type":    msgType,
		"content": content,
	}
	body, err := json.Marshal(bodyMap)
	if err != nil {
		return err
	}

	resp, err := c.doRequest(http.MethodPost, "/send", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("send failed: %d %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// doRequest performs an HTTP request
func (c *Client) doRequest(method, path string, body io.Reader) (*http.Response, error) {
	url := c.baseURL + path
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Cortex-Gateway/1.0.0")

	return c.httpClient.Do(req)
}
