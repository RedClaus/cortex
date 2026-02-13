// Package cortexbrain provides the CortexBrain client for HTTP API and Neural Bus WebSocket
package cortexbrain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client is the CortexBrain API and Neural Bus client
type Client struct {
	// Configuration
	baseURL   string
	wsURL     string
	authToken string

	// HTTP client with retry logic
	httpClient *http.Client
	maxRetries int
	retryDelay time.Duration

	// WebSocket connection
	wsConn     *websocket.Conn
	wsMutex    sync.RWMutex
	eventHandlers map[string][]EventHandler
	handlerMutex  sync.RWMutex
	
	// Connection state
	connected  bool
	stopCh     chan struct{}
	reconnectCh chan struct{}
}

// EventHandler is a callback function for Neural Bus events
type EventHandler func(event Event) error

// Event represents a Neural Bus event
type Event struct {
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Payload   map[string]interface{} `json:"payload"`
	Source    string                 `json:"source"`
}

// PromptRequest represents a prompt sent to CortexBrain
type PromptRequest struct {
	SessionID string                 `json:"session_id"`
	Prompt    string                 `json:"prompt"`
	Context   map[string]interface{} `json:"context,omitempty"`
	Skill     string                 `json:"skill,omitempty"`
}

// PromptResponse represents a response from CortexBrain
type PromptResponse struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Type      string    `json:"type"` // text, code, diff, etc.
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// KnowledgeEntry represents a knowledge base entry
type KnowledgeEntry struct {
	ID        string                 `json:"id"`
	Content   string                 `json:"content"`
	Category  string                 `json:"category"`
	Metadata  map[string]interface{} `json:"metadata"`
	Relevance float64                `json:"relevance,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// SearchRequest represents a knowledge search request
type SearchRequest struct {
	Query    string `json:"query"`
	Category string `json:"category,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

// SearchResponse represents a knowledge search response
type SearchResponse struct {
	Query   string           `json:"query"`
	Results []KnowledgeEntry `json:"results"`
	Total   int              `json:"total"`
}

// Session represents a coding session stored in CortexBrain
type Session struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	ProjectPath  string                 `json:"project_path"`
	FilesOpened  []string               `json:"files_opened"`
	FilesModified []string              `json:"files_modified"`
	Prompts      []PromptRecord         `json:"prompts"`
	SkillsUsed   []string               `json:"skills_used"`
	Metadata     map[string]interface{} `json:"metadata"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// PromptRecord represents a single prompt-response pair
type PromptRecord struct {
	Prompt    string    `json:"prompt"`
	Response  string    `json:"response"`
	Skill     string    `json:"skill,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// HealthResponse represents a health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Version   string    `json:"version"`
	Uptime    string    `json:"uptime"`
	Timestamp time.Time `json:"timestamp"`
}

// NewClient creates a new CortexBrain client
func NewClient(baseURL, wsURL, authToken string) *Client {
	return &Client{
		baseURL:       baseURL,
		wsURL:         wsURL,
		authToken:     authToken,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
		maxRetries:    3,
		retryDelay:    1 * time.Second,
		eventHandlers: make(map[string][]EventHandler),
		stopCh:        make(chan struct{}),
		reconnectCh:   make(chan struct{}, 1),
	}
}

// SetRetryConfig configures retry behavior
func (c *Client) SetRetryConfig(maxRetries int, delay time.Duration) {
	c.maxRetries = maxRetries
	c.retryDelay = delay
}

// SetHTTPTimeout sets the HTTP client timeout
func (c *Client) SetHTTPTimeout(timeout time.Duration) {
	c.httpClient.Timeout = timeout
}

// ==================== HTTP API Methods ====================

// ChatMessage represents a message in the chat API
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest represents a chat completion request
type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream,omitempty"`
	Lane     string        `json:"lane,omitempty"`
}

// OpenAIChatResponse represents an OpenAI-compatible chat completion response
type OpenAIChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int         `json:"index"`
		Message ChatMessage `json:"message"`
	} `json:"choices"`
}

// OllamaChatResponse represents an Ollama chat response
type OllamaChatResponse struct {
	Model   string `json:"model"`
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done       bool   `json:"done"`
	DoneReason string `json:"done_reason,omitempty"`
}

// SendPrompt sends a prompt to CortexBrain and returns the response
func (c *Client) SendPrompt(ctx context.Context, req PromptRequest) (*PromptResponse, error) {
	// Convert to chat API format
	chatReq := ChatRequest{
		Model: req.Context["model"].(string),
		Messages: []ChatMessage{
			{Role: "user", Content: req.Prompt},
		},
	}
	
	if lane, ok := req.Context["lane"].(string); ok {
		chatReq.Lane = lane
	}

	payload, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal chat request: %w", err)
	}

	respBody, err := c.doRequestWithRetry(ctx, http.MethodPost, "/v1/chat/completions", payload)
	if err != nil {
		return nil, fmt.Errorf("failed to send prompt: %w", err)
	}

	// Try OpenAI format first
	var openAIResp OpenAIChatResponse
	if err := json.Unmarshal(respBody, &openAIResp); err == nil && len(openAIResp.Choices) > 0 {
		return &PromptResponse{
			ID:        openAIResp.ID,
			Content:   openAIResp.Choices[0].Message.Content,
			Type:      "text",
			Timestamp: time.Now(),
		}, nil
	}

	// Try Ollama format
	var ollamaResp OllamaChatResponse
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode chat response: %w", err)
	}

	if ollamaResp.Message.Content == "" {
		return nil, fmt.Errorf("no response from model")
	}

	return &PromptResponse{
		ID:        "",
		Content:   ollamaResp.Message.Content,
		Type:      "text",
		Timestamp: time.Now(),
	}, nil
}

// GetModels fetches available models from CortexBrain
func (c *Client) GetModels(ctx context.Context) ([]ModelInfo, error) {
	respBody, err := c.doRequestWithRetry(ctx, http.MethodGet, "/api/models", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get models: %w", err)
	}

	var resp modelsResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to decode models response: %w", err)
	}

	return resp.Models, nil
}

// modelsResponse represents Pink's models endpoint response
type modelsResponse struct {
	Models []ModelInfo `json:"models"`
}

// ModelInfo represents a model from the API
type ModelInfo struct {
	ID       string `json:"id"`
	Backend  string `json:"backend"`
	Type     string `json:"type"`
}

// SearchKnowledge searches the knowledge base
func (c *Client) SearchKnowledge(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal search request: %w", err)
	}

	respBody, err := c.doRequestWithRetry(ctx, http.MethodPost, "/api/knowledge/search", payload)
	if err != nil {
		return nil, fmt.Errorf("failed to search knowledge: %w", err)
	}

	var resp SearchResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	return &resp, nil
}

// StoreSession stores a session in CortexBrain memory
func (c *Client) StoreSession(ctx context.Context, session Session) error {
	payload, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	_, err = c.doRequestWithRetry(ctx, http.MethodPost, "/api/memory/session", payload)
	if err != nil {
		return fmt.Errorf("failed to store session: %w", err)
	}

	return nil
}

// GetSession retrieves a session from CortexBrain memory
func (c *Client) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	respBody, err := c.doRequestWithRetry(ctx, http.MethodGet, "/api/memory/session/"+sessionID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	var session Session
	if err := json.Unmarshal(respBody, &session); err != nil {
		return nil, fmt.Errorf("failed to decode session: %w", err)
	}

	return &session, nil
}

// HealthCheck checks if CortexBrain is healthy
func (c *Client) HealthCheck(ctx context.Context) (*HealthResponse, error) {
	respBody, err := c.doRequestWithRetry(ctx, http.MethodGet, "/health", nil)
	if err != nil {
		return nil, fmt.Errorf("health check failed: %w", err)
	}

	var health HealthResponse
	if err := json.Unmarshal(respBody, &health); err != nil {
		return nil, fmt.Errorf("failed to decode health response: %w", err)
	}

	return &health, nil
}

// Ping performs a simple connectivity check
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.doRequestWithRetry(ctx, http.MethodGet, "/api/ping", nil)
	return err
}

// ==================== WebSocket / Neural Bus Methods ====================

// Connect establishes a WebSocket connection to the Neural Bus
func (c *Client) Connect(ctx context.Context) error {
	c.wsMutex.Lock()
	defer c.wsMutex.Unlock()

	if c.connected {
		return nil // Already connected
	}

	headers := http.Header{}
	if c.authToken != "" {
		headers.Set("Authorization", "Bearer "+c.authToken)
	}

	conn, _, err := websocket.DefaultDialer.Dial(c.wsURL, headers)
	if err != nil {
		return fmt.Errorf("failed to connect to Neural Bus: %w", err)
	}

	c.wsConn = conn
	c.connected = true

	// Start event listener
	go c.listenForEvents()
	
	// Start reconnection handler
	go c.handleReconnection()

	return nil
}

// Disconnect closes the WebSocket connection
func (c *Client) Disconnect() error {
	c.wsMutex.Lock()
	defer c.wsMutex.Unlock()

	if !c.connected {
		return nil
	}

	close(c.stopCh)

	if c.wsConn != nil {
		// Send close message
		c.wsConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		c.wsConn.Close()
	}

	c.connected = false
	return nil
}

// IsConnected returns whether the WebSocket is connected
func (c *Client) IsConnected() bool {
	c.wsMutex.RLock()
	defer c.wsMutex.RUnlock()
	return c.connected
}

// Subscribe registers an event handler for a specific event type
func (c *Client) Subscribe(eventType string, handler EventHandler) {
	c.handlerMutex.Lock()
	defer c.handlerMutex.Unlock()
	c.eventHandlers[eventType] = append(c.eventHandlers[eventType], handler)
}

// Unsubscribe removes all handlers for an event type
func (c *Client) Unsubscribe(eventType string) {
	c.handlerMutex.Lock()
	defer c.handlerMutex.Unlock()
	delete(c.eventHandlers, eventType)
}

// Publish sends an event to the Neural Bus
func (c *Client) Publish(event Event) error {
	c.wsMutex.RLock()
	defer c.wsMutex.RUnlock()

	if !c.connected || c.wsConn == nil {
		return fmt.Errorf("not connected to Neural Bus")
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	return c.wsConn.WriteMessage(websocket.TextMessage, data)
}

// ==================== Private Methods ====================

func (c *Client) doRequestWithRetry(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	var lastErr error
	
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(c.retryDelay * time.Duration(attempt)):
				// Exponential backoff
			}
		}

		respBody, err := c.doRequest(ctx, method, path, body)
		if err == nil {
			return respBody, nil
		}

		lastErr = err
		
		// Don't retry on client errors (4xx)
		if httpErr, ok := err.(*HTTPError); ok && httpErr.StatusCode >= 400 && httpErr.StatusCode < 500 {
			return nil, err
		}
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

func (c *Client) doRequest(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	u.Path = path

	var bodyReader *bytes.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	} else {
		bodyReader = bytes.NewReader([]byte{})
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &HTTPError{
			StatusCode: resp.StatusCode,
			Message:    buf.String(),
		}
	}

	return buf.Bytes(), nil
}

func (c *Client) listenForEvents() {
	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		c.wsMutex.RLock()
		conn := c.wsConn
		c.wsMutex.RUnlock()

		if conn == nil {
			return
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// Trigger reconnection
				select {
				case c.reconnectCh <- struct{}{}:
				default:
				}
			}
			return
		}

		var event Event
		if err := json.Unmarshal(data, &event); err != nil {
			continue // Skip invalid events
		}

		c.dispatchEvent(event)
	}
}

func (c *Client) dispatchEvent(event Event) {
	c.handlerMutex.RLock()
	handlers := c.eventHandlers[event.Type]
	// Also get handlers for wildcard "*"
	wildcardHandlers := c.eventHandlers["*"]
	c.handlerMutex.RUnlock()

	// Combine specific and wildcard handlers
	allHandlers := append(handlers, wildcardHandlers...)

	for _, handler := range allHandlers {
		go func(h EventHandler) {
			if err := h(event); err != nil {
				// Log error but don't stop other handlers
				fmt.Printf("Event handler error: %v\n", err)
			}
		}(handler)
	}
}

func (c *Client) handleReconnection() {
	for {
		select {
		case <-c.stopCh:
			return
		case <-c.reconnectCh:
			// Attempt reconnection with exponential backoff
			for attempt := 1; attempt <= c.maxRetries; attempt++ {
				select {
				case <-c.stopCh:
					return
				case <-time.After(c.retryDelay * time.Duration(attempt)):
				}

				c.wsMutex.Lock()
				c.connected = false
				c.wsMutex.Unlock()

				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				err := c.Connect(ctx)
				cancel()

				if err == nil {
					break // Reconnected successfully
				}
			}
		}
	}
}

// HTTPError represents an HTTP error response
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}
