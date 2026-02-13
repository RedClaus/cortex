package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// ClientConfig configures the A2A client
type ClientConfig struct {
	ServerURL      string        // e.g., "http://localhost:8080"
	Timeout        time.Duration // HTTP request timeout
	ReconnectDelay time.Duration // SSE reconnection delay
	MaxReconnects  int           // Max reconnection attempts
	UserID         string        // User ID for requests
	PersonaID      string        // Persona ID for requests
}

// DefaultClientConfig returns sensible defaults
func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		ServerURL:      "http://localhost:8080",
		Timeout:        120 * time.Second,
		ReconnectDelay: 5 * time.Second,
		MaxReconnects:  10,
		UserID:         "default-user",
		PersonaID:      "hannah",
	}
}

// Client manages A2A protocol communication with CortexBrain
type Client struct {
	config     *ClientConfig
	httpClient *http.Client
	agentCard  *AgentCard
	logger     zerolog.Logger

	mu        sync.RWMutex
	connected bool

	onStatusChange func(connected bool, agentCard *AgentCard)
	onError        func(err error)
}

// NewClient creates a new A2A client
func NewClient(cfg *ClientConfig, logger zerolog.Logger) *Client {
	if cfg == nil {
		cfg = DefaultClientConfig()
	}

	return &Client{
		config: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		logger: logger.With().Str("component", "a2a-client").Logger(),
	}
}

// SetStatusHandler sets the callback for connection status changes
func (c *Client) SetStatusHandler(handler func(connected bool, agentCard *AgentCard)) {
	c.onStatusChange = handler
}

// SetErrorHandler sets the callback for errors
func (c *Client) SetErrorHandler(handler func(err error)) {
	c.onError = handler
}

// Connect discovers the agent and establishes connection
func (c *Client) Connect(ctx context.Context) error {
	card, err := c.DiscoverAgent(ctx)
	if err != nil {
		c.setConnected(false, nil)
		return fmt.Errorf("failed to discover agent: %w", err)
	}

	c.mu.Lock()
	c.agentCard = card
	c.mu.Unlock()

	c.setConnected(true, card)
	c.logger.Info().
		Str("agent", card.Name).
		Str("version", card.Version).
		Str("protocol", card.ProtocolVersion).
		Msg("Connected to CortexBrain")

	return nil
}

// DiscoverAgent fetches the agent card from CortexBrain
func (c *Client) DiscoverAgent(ctx context.Context) (*AgentCard, error) {
	url := c.config.ServerURL + "/.well-known/agent-card.json"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch agent card: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("agent card request failed: %d - %s", resp.StatusCode, string(body))
	}

	var card AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, fmt.Errorf("failed to decode agent card: %w", err)
	}

	return &card, nil
}

// SendMessage sends a message and returns the response
func (c *Client) SendMessage(ctx context.Context, text string) (*Message, error) {
	return c.SendMessageWithVision(ctx, text, "", "")
}

// SendMessageWithVision sends a message with optional vision data
func (c *Client) SendMessageWithVision(ctx context.Context, text, imageBase64, mimeType string) (*Message, error) {
	var msg *Message
	if imageBase64 != "" {
		msg = NewVisionMessage("user", text, imageBase64, mimeType, map[string]any{
			"userId":    c.config.UserID,
			"personaId": c.config.PersonaID,
		})
	} else {
		msg = NewTextMessage("user", text, map[string]any{
			"userId":    c.config.UserID,
			"personaId": c.config.PersonaID,
		})
	}

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "message/send", // A2A Protocol v0.3.0 method name
		Params: MessageSendParams{
			Message: msg,
		},
		ID: 1,
	}

	body, err := json.Marshal(rpcReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.ServerURL+"/", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.setConnected(false, nil)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check if SSE stream
	if resp.Header.Get("Content-Type") == "text/event-stream" {
		return c.parseSSEResponse(resp.Body)
	}

	// Parse JSON response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	c.logger.Debug().
		Int("bodyLen", len(respBody)).
		Str("bodyPreview", truncateForLog(string(respBody), 500)).
		Msg("A2A raw response received")

	var rpcResp JSONRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		c.logger.Error().Err(err).Str("body", string(respBody)).Msg("Failed to parse JSON-RPC response")
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if rpcResp.Error != nil {
		c.logger.Error().Int("code", rpcResp.Error.Code).Str("msg", rpcResp.Error.Message).Msg("JSON-RPC error from CortexBrain")
		return nil, fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	// Extract message from result (A2A v0.3.0 Task response)
	if result, ok := rpcResp.Result.(map[string]any); ok {
		c.logger.Debug().Interface("resultKeys", getMapKeys(result)).Msg("Parsing A2A result")

		// Try result.status.message (A2A Task response format)
		if status, ok := result["status"].(map[string]any); ok {
			c.logger.Debug().Interface("statusKeys", getMapKeys(status)).Msg("Found status in result")
			if msgData, ok := status["message"].(map[string]any); ok {
				c.logger.Debug().Msg("Extracting message from result.status.message")
				return c.parseMessageFromMap(msgData), nil
			}
		}
		// Fallback: try result.message directly
		if msgData, ok := result["message"].(map[string]any); ok {
			c.logger.Debug().Msg("Extracting message from result.message")
			return c.parseMessageFromMap(msgData), nil
		}
		// Try to extract from history if present
		if history, ok := result["history"].([]any); ok && len(history) > 0 {
			c.logger.Debug().Int("historyLen", len(history)).Msg("Found history in result")
			// Get the last message from history
			for i := len(history) - 1; i >= 0; i-- {
				if histItem, ok := history[i].(map[string]any); ok {
					if role, ok := histItem["role"].(string); ok && role == "agent" {
						c.logger.Debug().Int("idx", i).Msg("Extracting agent message from history")
						return c.parseMessageFromMap(histItem), nil
					}
				}
			}
		}
	}

	c.logger.Warn().Interface("result", rpcResp.Result).Msg("Unexpected response format - no message found")
	return nil, fmt.Errorf("unexpected response format")
}

// SendMessageWithOptions sends a message with configurable options
func (c *Client) SendMessageWithOptions(ctx context.Context, text string, opts SendMessageOptions) (*Message, error) {
	// Determine persona - use option override or fall back to config
	personaID := c.config.PersonaID
	if opts.Persona != "" {
		personaID = opts.Persona
	}

	// Build message metadata (server expects mode here for Voice Executive routing)
	metadata := map[string]any{
		"userId":    c.config.UserID,
		"personaId": personaID,
	}
	// CR-093: Add mode to metadata so server can route to Voice Executive
	if opts.Mode != "" {
		metadata["mode"] = opts.Mode
	}

	var msg *Message
	if opts.ImageBase64 != "" {
		msg = NewVisionMessage("user", text, opts.ImageBase64, opts.MimeType, metadata)
	} else {
		msg = NewTextMessage("user", text, metadata)
	}

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "message/send",
		Params: MessageSendParams{
			Message: msg,
			Mode:    opts.Mode, // Include mode (e.g., "voice") for Voice Executive routing
		},
		ID: 1,
	}

	body, err := json.Marshal(rpcReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	c.logger.Debug().
		Str("mode", opts.Mode).
		Str("personaId", personaID).
		Int("bodyLen", len(body)).
		Msg("Sending message with options")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.ServerURL+"/", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.setConnected(false, nil)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check if SSE stream
	if resp.Header.Get("Content-Type") == "text/event-stream" {
		return c.parseSSEResponse(resp.Body)
	}

	// Parse JSON response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	c.logger.Debug().
		Int("bodyLen", len(respBody)).
		Str("bodyPreview", truncateForLog(string(respBody), 500)).
		Msg("A2A raw response received")

	var rpcResp JSONRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		c.logger.Error().Err(err).Str("body", string(respBody)).Msg("Failed to parse JSON-RPC response")
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if rpcResp.Error != nil {
		c.logger.Error().Int("code", rpcResp.Error.Code).Str("msg", rpcResp.Error.Message).Msg("JSON-RPC error from CortexBrain")
		return nil, fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	// Extract message from result (A2A v0.3.0 Task response)
	if result, ok := rpcResp.Result.(map[string]any); ok {
		c.logger.Debug().Interface("resultKeys", getMapKeys(result)).Msg("Parsing A2A result")

		// Try result.status.message (A2A Task response format)
		if status, ok := result["status"].(map[string]any); ok {
			c.logger.Debug().Interface("statusKeys", getMapKeys(status)).Msg("Found status in result")
			if msgData, ok := status["message"].(map[string]any); ok {
				c.logger.Debug().Msg("Extracting message from result.status.message")
				return c.parseMessageFromMap(msgData), nil
			}
		}
		// Fallback: try result.message directly
		if msgData, ok := result["message"].(map[string]any); ok {
			c.logger.Debug().Msg("Extracting message from result.message")
			return c.parseMessageFromMap(msgData), nil
		}
		// Try to extract from history if present
		if history, ok := result["history"].([]any); ok && len(history) > 0 {
			c.logger.Debug().Int("historyLen", len(history)).Msg("Found history in result")
			for i := len(history) - 1; i >= 0; i-- {
				if histItem, ok := history[i].(map[string]any); ok {
					if role, ok := histItem["role"].(string); ok && role == "agent" {
						c.logger.Debug().Int("idx", i).Msg("Extracting agent message from history")
						return c.parseMessageFromMap(histItem), nil
					}
				}
			}
		}
	}

	c.logger.Warn().Interface("result", rpcResp.Result).Msg("Unexpected response format - no message found")
	return nil, fmt.Errorf("unexpected response format")
}

// SendMessageStream sends a message and streams responses via callback
func (c *Client) SendMessageStream(ctx context.Context, text string, handler func(TaskEvent)) error {
	return c.SendMessageStreamWithVision(ctx, text, "", "", handler)
}

// SendMessageStreamWithVision sends a message with optional vision and streams responses
func (c *Client) SendMessageStreamWithVision(ctx context.Context, text, imageBase64, mimeType string, handler func(TaskEvent)) error {
	return c.SendMessageStreamWithOptions(ctx, text, SendMessageOptions{
		ImageBase64: imageBase64,
		MimeType:    mimeType,
	}, handler)
}

// SendMessageStreamWithOptions sends a message with configurable options and streams responses
func (c *Client) SendMessageStreamWithOptions(ctx context.Context, text string, opts SendMessageOptions, handler func(TaskEvent)) error {
	// Determine persona - use option override or fall back to config
	personaID := c.config.PersonaID
	if opts.Persona != "" {
		personaID = opts.Persona
	}

	// Build message metadata (server expects mode here for Voice Executive routing)
	metadata := map[string]any{
		"userId":    c.config.UserID,
		"personaId": personaID,
	}
	// CR-093: Add mode to metadata so server can route to Voice Executive
	if opts.Mode != "" {
		metadata["mode"] = opts.Mode
	}

	var msg *Message
	if opts.ImageBase64 != "" {
		msg = NewVisionMessage("user", text, opts.ImageBase64, opts.MimeType, metadata)
	} else {
		msg = NewTextMessage("user", text, metadata)
	}

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "message/stream", // A2A Protocol v0.3.0 streaming method
		Params: MessageSendParams{
			Message: msg,
			Mode:    opts.Mode, // Include mode (e.g., "voice") for Voice Executive routing
		},
		ID: 1,
	}

	c.logger.Debug().
		Str("mode", opts.Mode).
		Str("personaId", personaID).
		Msg("Streaming message with options")

	body, err := json.Marshal(rpcReq)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.ServerURL+"/", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	// Use a client without timeout for streaming
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.setConnected(false, nil)
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	return c.handleSSEStream(ctx, resp.Body, handler)
}

// parseSSEResponse parses SSE events and returns the final message
func (c *Client) parseSSEResponse(reader io.Reader) (*Message, error) {
	var finalMessage *Message

	err := c.handleSSEStream(context.Background(), reader, func(event TaskEvent) {
		if event.Final && event.Message != nil {
			finalMessage = event.Message
		}
	})

	if err != nil {
		return nil, err
	}

	if finalMessage == nil {
		return nil, fmt.Errorf("no final message received")
	}

	return finalMessage, nil
}

// handleSSEStream processes SSE events
func (c *Client) handleSSEStream(ctx context.Context, reader io.Reader, handler func(TaskEvent)) error {
	sse := NewSSEReader(reader)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		event, err := sse.ReadEvent()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("SSE read error: %w", err)
		}

		if event.Data == "" {
			continue
		}

		var taskEvent TaskEvent
		if err := json.Unmarshal([]byte(event.Data), &taskEvent); err != nil {
			c.logger.Warn().Err(err).Str("data", event.Data).Msg("Failed to parse SSE event")
			continue
		}

		taskEvent.EventType = event.Event
		handler(taskEvent)

		if taskEvent.Final {
			return nil
		}
	}
}

// parseMessageFromMap converts a map to a Message
func (c *Client) parseMessageFromMap(data map[string]any) *Message {
	msg := &Message{}

	if role, ok := data["role"].(string); ok {
		msg.Role = role
	}

	if parts, ok := data["parts"].([]any); ok {
		for _, p := range parts {
			if partMap, ok := p.(map[string]any); ok {
				// A2A v0.3.0 uses "kind" not "type"
				partKind, _ := partMap["kind"].(string)
				switch partKind {
				case "text":
					text, _ := partMap["text"].(string)
					msg.Parts = append(msg.Parts, TextPart{Kind: "text", Text: text})
				case "data":
					if d, ok := partMap["data"].(map[string]any); ok {
						msg.Parts = append(msg.Parts, DataPart{Kind: "data", Data: d})
					}
				case "file":
					name, _ := partMap["name"].(string)
					mimeType, _ := partMap["mimeType"].(string)
					bytes, _ := partMap["bytes"].(string)
					msg.Parts = append(msg.Parts, FilePart{Kind: "file", Name: name, MimeType: mimeType, Bytes: bytes})
				}
			}
		}
	}

	if metadata, ok := data["metadata"].(map[string]any); ok {
		msg.Metadata = metadata
	}

	return msg
}

// setConnected updates connection status and notifies handler
func (c *Client) setConnected(connected bool, card *AgentCard) {
	c.mu.Lock()
	changed := c.connected != connected
	c.connected = connected
	if card != nil {
		c.agentCard = card
	}
	c.mu.Unlock()

	if changed && c.onStatusChange != nil {
		c.onStatusChange(connected, card)
	}
}

// IsConnected returns current connection status
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// GetAgentCard returns the current agent card
func (c *Client) GetAgentCard() *AgentCard {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.agentCard
}

// SetPersonaID updates the persona ID for future messages
func (c *Client) SetPersonaID(personaID string) {
	c.mu.Lock()
	c.config.PersonaID = personaID
	c.mu.Unlock()
	c.logger.Info().Str("personaId", personaID).Msg("Persona ID updated")
}

// UpdateServerURL updates the server URL for the client
func (c *Client) UpdateServerURL(url string) {
	c.mu.Lock()
	c.config.ServerURL = url
	c.mu.Unlock()
	c.logger.Info().Str("serverURL", url).Msg("Server URL updated")
}

// GetServerURL returns the current server URL
func (c *Client) GetServerURL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config.ServerURL
}

// Close closes the client
func (c *Client) Close() error {
	c.setConnected(false, nil)
	return nil
}

// StreamingResponse contains streaming response data
type StreamingResponse struct {
	Text    string
	Delta   string
	IsFinal bool
	State   TaskState
	Message *Message
	Error   error
}

// SendMessageStreamChan sends a message and returns a channel of streaming responses
func (c *Client) SendMessageStreamChan(ctx context.Context, text string) (<-chan StreamingResponse, error) {
	return c.SendMessageStreamChanWithOptions(ctx, text, SendMessageOptions{Stream: true})
}

// SendMessageStreamChanWithOptions sends a message with options and returns a channel of streaming responses.
// If opts.Stream is true (or this method is called), streaming is used; the Stream field
// explicitly signals the intent to use SSE streaming for incremental response delivery.
func (c *Client) SendMessageStreamChanWithOptions(ctx context.Context, text string, opts SendMessageOptions) (<-chan StreamingResponse, error) {
	ch := make(chan StreamingResponse, 32)

	go func() {
		defer close(ch)

		var accumulatedText string

		err := c.SendMessageStreamWithOptions(ctx, text, opts, func(event TaskEvent) {
			resp := StreamingResponse{
				State:   event.State,
				IsFinal: event.Final,
				Message: event.Message,
			}

			if event.Message != nil {
				newText := event.Message.ExtractText()
				if len(newText) > len(accumulatedText) {
					resp.Delta = newText[len(accumulatedText):]
					accumulatedText = newText
				}
				resp.Text = newText
			}

			select {
			case ch <- resp:
			case <-ctx.Done():
				return
			}
		})

		if err != nil {
			select {
			case ch <- StreamingResponse{Error: err, IsFinal: true}:
			case <-ctx.Done():
			}
		}
	}()

	return ch, nil
}

// truncateForLog truncates a string for logging purposes
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// getMapKeys returns the keys of a map for logging
func getMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
