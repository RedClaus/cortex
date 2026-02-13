// Package vision provides vision streaming to CortexBrain via WebSocket.
package vision

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

// WSFrameMessage is sent to CortexBrain to ingest a frame
type WSFrameMessage struct {
	Type      string `json:"type"`
	Data      string `json:"data"`
	MimeType  string `json:"mime_type"`
	Sequence  int64  `json:"sequence,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

// WSAnalysisMessage is received from CortexBrain with analysis results
type WSAnalysisMessage struct {
	Type          string `json:"type"`
	FrameSequence int64  `json:"frame_sequence"`
	Content       string `json:"content"`
	Provider      string `json:"provider"`
	LatencyMs     int64  `json:"latency_ms"`
	Timestamp     string `json:"timestamp"`
}

// WSAckMessage acknowledges frame receipt
type WSAckMessage struct {
	Type     string `json:"type"`
	Sequence int64  `json:"sequence"`
}

// WSErrorMessage reports errors
type WSErrorMessage struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// StreamClient connects to CortexBrain's vision streaming WebSocket
type StreamClient struct {
	baseURL string
	logger  zerolog.Logger
	conn    *websocket.Conn

	mu        sync.RWMutex
	connected bool
	sequence  int64
	cancel    context.CancelFunc

	// Callbacks
	onAnalysis func(content string, provider string)
	onError    func(err error)
}

// NewStreamClient creates a new vision stream client
func NewStreamClient(baseURL string, logger zerolog.Logger) *StreamClient {
	return &StreamClient{
		baseURL: baseURL,
		logger:  logger.With().Str("component", "vision-stream").Logger(),
	}
}

// SetAnalysisCallback sets the callback for analysis results
func (c *StreamClient) SetAnalysisCallback(cb func(content string, provider string)) {
	c.onAnalysis = cb
}

// SetErrorCallback sets the callback for errors
func (c *StreamClient) SetErrorCallback(cb func(err error)) {
	c.onError = cb
}

// Connect establishes the WebSocket connection
func (c *StreamClient) Connect(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	go c.connectLoop(ctx)
	return nil
}

// Disconnect closes the WebSocket connection
func (c *StreamClient) Disconnect() {
	if c.cancel != nil {
		c.cancel()
	}
	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.connected = false
	c.mu.Unlock()
}

// IsConnected returns connection status
func (c *StreamClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// SendFrame sends a video frame to CortexBrain
func (c *StreamClient) SendFrame(frame *Frame) error {
	c.mu.Lock()
	if !c.connected || c.conn == nil {
		c.mu.Unlock()
		return fmt.Errorf("not connected")
	}
	conn := c.conn
	c.sequence++
	seq := c.sequence
	c.mu.Unlock()

	// Convert format to MIME type
	mimeType := "image/jpeg"
	if frame.Format == "png" {
		mimeType = "image/png"
	}

	msg := WSFrameMessage{
		Type:      "frame",
		Data:      base64.StdEncoding.EncodeToString(frame.Data),
		MimeType:  mimeType,
		Sequence:  seq,
		Timestamp: frame.Timestamp.Format(time.RFC3339),
	}

	if err := conn.WriteJSON(msg); err != nil {
		return fmt.Errorf("write frame: %w", err)
	}

	c.logger.Debug().Int64("sequence", seq).Msg("Frame sent")
	return nil
}

// connectLoop maintains the WebSocket connection with reconnection
func (c *StreamClient) connectLoop(ctx context.Context) {
	backoff := 3 * time.Second
	maxBackoff := 60 * time.Second
	consecutiveFailures := 0

	for {
		select {
		case <-ctx.Done():
			return
		default:
			if err := c.connectWS(ctx); err != nil {
				consecutiveFailures++
				c.mu.Lock()
				c.connected = false
				c.mu.Unlock()

				if consecutiveFailures >= 3 {
					if consecutiveFailures == 3 {
						c.logger.Warn().
							Err(err).
							Int("failures", consecutiveFailures).
							Msg("Vision stream WebSocket not available, will retry less frequently")
					} else {
						c.logger.Debug().
							Int("failures", consecutiveFailures).
							Msg("Vision stream WebSocket still unavailable")
					}
					backoff = maxBackoff
				} else {
					c.logger.Warn().Err(err).Msg("WebSocket connection failed, reconnecting...")
				}

				// Wait before reconnecting with exponential backoff
				select {
				case <-ctx.Done():
					return
				case <-time.After(backoff):
				}

				// Increase backoff
				if backoff < maxBackoff {
					backoff = backoff * 2
					if backoff > maxBackoff {
						backoff = maxBackoff
					}
				}
			} else {
				// Reset backoff on success
				backoff = 3 * time.Second
				consecutiveFailures = 0
			}
		}
	}
}

// connectWS establishes WebSocket connection
func (c *StreamClient) connectWS(ctx context.Context) error {
	// Convert HTTP URL to WebSocket URL
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}

	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}
	u.Path = "/api/v1/vision/stream/ws"

	c.logger.Info().Str("url", u.String()).Msg("Connecting to vision stream WebSocket")

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.connected = true
	c.mu.Unlock()

	c.logger.Info().Msg("Connected to vision stream WebSocket")

	// Read messages
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			var msg json.RawMessage
			if err := conn.ReadJSON(&msg); err != nil {
				return fmt.Errorf("read: %w", err)
			}

			c.handleMessage(msg)
		}
	}
}

// handleMessage processes incoming WebSocket messages
func (c *StreamClient) handleMessage(raw json.RawMessage) {
	// First parse to determine message type
	var typeMsg struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &typeMsg); err != nil {
		c.logger.Warn().Err(err).Msg("Failed to parse message type")
		return
	}

	switch typeMsg.Type {
	case "analysis":
		var msg WSAnalysisMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			c.logger.Warn().Err(err).Msg("Failed to parse analysis message")
			return
		}
		c.logger.Info().
			Int64("frame", msg.FrameSequence).
			Int64("latency_ms", msg.LatencyMs).
			Str("provider", msg.Provider).
			Msg("Received analysis")

		if c.onAnalysis != nil {
			c.onAnalysis(msg.Content, msg.Provider)
		}

	case "ack":
		var msg WSAckMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			c.logger.Warn().Err(err).Msg("Failed to parse ack message")
			return
		}
		c.logger.Debug().Int64("sequence", msg.Sequence).Msg("Frame acknowledged")

	case "error":
		var msg WSErrorMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			c.logger.Warn().Err(err).Msg("Failed to parse error message")
			return
		}
		c.logger.Warn().Str("message", msg.Message).Msg("Server error")

		if c.onError != nil {
			c.onError(fmt.Errorf("server: %s", msg.Message))
		}

	default:
		c.logger.Debug().Str("type", typeMsg.Type).Msg("Unknown message type")
	}
}

// CheckHealth checks the vision stream health endpoint
func (c *StreamClient) CheckHealth(ctx context.Context) error {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return err
	}
	u.Path = "/api/v1/vision/stream/stats"

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: %d", resp.StatusCode)
	}

	return nil
}
