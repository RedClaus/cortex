// Package voice provides voice-related types and utilities for Cortex.
// vad_client.go provides a WebSocket client for Voice Activity Detection (VAD) streaming (CR-013).
package voice

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

// VADMode represents the VAD operating mode.
type VADMode string

const (
	// VADModeFull is the normal detection mode.
	VADModeFull VADMode = "FULL"
	// VADModePlayback is used during TTS playback (higher threshold to avoid false triggers).
	VADModePlayback VADMode = "PLAYBACK"
)

// VADEvent represents a voice activity detection event from the server.
type VADEvent struct {
	Type          string  `json:"type"`
	Timestamp     float64 `json:"timestamp"`
	Confidence    float64 `json:"confidence"`
	AudioBase64   string  `json:"audio_base64,omitempty"`
	AudioLengthMs float64 `json:"audio_length_ms,omitempty"`
	DurationMs    float64 `json:"duration_ms,omitempty"`
}

// VADClientConfig holds configuration for the VAD WebSocket client.
type VADClientConfig struct {
	// Endpoint is the WebSocket endpoint for VAD streaming
	Endpoint string

	// HTTPBaseURL is the base URL for HTTP API calls (e.g., http://127.0.0.1:8880)
	// If empty, it will be derived from the WebSocket endpoint.
	HTTPBaseURL string

	// ReconnectWait is the initial wait time before reconnecting
	ReconnectWait time.Duration

	// MaxReconnects is the maximum number of reconnection attempts (0 = unlimited)
	MaxReconnects int

	// PingInterval is the interval between ping messages to keep connection alive
	PingInterval time.Duration

	// HTTPTimeout is the timeout for HTTP requests (defaults to 10 seconds)
	HTTPTimeout time.Duration
}

// DefaultVADClientConfig returns production defaults for the VAD client.
func DefaultVADClientConfig() VADClientConfig {
	return VADClientConfig{
		Endpoint:      "ws://127.0.0.1:8880/v1/vad/stream",
		HTTPBaseURL:   "http://127.0.0.1:8880",
		ReconnectWait: 1 * time.Second,
		MaxReconnects: 5,
		PingInterval:  30 * time.Second,
		HTTPTimeout:   10 * time.Second,
	}
}

// VADClient provides WebSocket-based VAD streaming functionality.
type VADClient struct {
	mu           sync.RWMutex
	config       VADClientConfig
	conn         *websocket.Conn
	httpClient   *http.Client
	running      bool
	reconnecting bool
	ctx          context.Context
	cancel       context.CancelFunc
	currentMode  VADMode

	// Callbacks
	OnSpeechStart func(event VADEvent)
	OnSpeechEnd   func(event VADEvent, audioData []byte)
	OnInterrupt   func(event VADEvent, audioData []byte) // Called when user interrupts during TTS playback
	OnError       func(err error)
}

// NewVADClient creates a new VAD client with the given configuration.
func NewVADClient(config VADClientConfig) *VADClient {
	if config.Endpoint == "" {
		config = DefaultVADClientConfig()
	}

	// Derive HTTP base URL from WebSocket endpoint if not provided
	if config.HTTPBaseURL == "" {
		config.HTTPBaseURL = deriveHTTPBaseURL(config.Endpoint)
	}

	// Set default HTTP timeout
	if config.HTTPTimeout == 0 {
		config.HTTPTimeout = 10 * time.Second
	}

	return &VADClient{
		config: config,
		httpClient: &http.Client{
			Timeout: config.HTTPTimeout,
		},
		currentMode: VADModeFull, // Default mode
	}
}

// deriveHTTPBaseURL converts a WebSocket URL to an HTTP base URL.
// e.g., "ws://127.0.0.1:8880/v1/vad/stream" -> "http://127.0.0.1:8880"
func deriveHTTPBaseURL(wsEndpoint string) string {
	// Replace ws:// with http:// or wss:// with https://
	httpURL := strings.Replace(wsEndpoint, "wss://", "https://", 1)
	httpURL = strings.Replace(httpURL, "ws://", "http://", 1)

	// Remove the path portion to get just the base URL
	// Find the third slash (after http://host:port)
	slashCount := 0
	for i, c := range httpURL {
		if c == '/' {
			slashCount++
			if slashCount == 3 {
				return httpURL[:i]
			}
		}
	}
	return httpURL
}

// Connect establishes a WebSocket connection to the VAD server.
func (c *VADClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return fmt.Errorf("vad client: already connected")
	}

	// Create a child context for managing goroutines
	c.ctx, c.cancel = context.WithCancel(ctx)

	// Configure WebSocket dialer with handshake timeout
	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}

	log.Debug().
		Str("endpoint", c.config.Endpoint).
		Msg("connecting to VAD server")

	conn, _, err := dialer.DialContext(ctx, c.config.Endpoint, nil)
	if err != nil {
		return fmt.Errorf("vad client: failed to connect: %w", err)
	}

	c.conn = conn
	c.running = true

	// Start event listener goroutine
	go c.listenEvents()

	// Start ping goroutine to keep connection alive
	go c.pingLoop()

	log.Info().
		Str("endpoint", c.config.Endpoint).
		Msg("VAD client connected")

	return nil
}

// listenEvents reads and processes events from the WebSocket connection.
func (c *VADClient) listenEvents() {
	for {
		c.mu.RLock()
		conn := c.conn
		running := c.running
		ctx := c.ctx
		c.mu.RUnlock()

		if !running || conn == nil {
			return
		}

		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Read message from WebSocket
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			c.mu.RLock()
			stillRunning := c.running
			c.mu.RUnlock()

			if !stillRunning {
				// Client was closed intentionally
				return
			}

			log.Error().
				Err(err).
				Msg("VAD client: error reading message")

			// Attempt reconnection
			go c.reconnect(ctx)
			return
		}

		// Only process text (JSON) messages
		if messageType != websocket.TextMessage {
			log.Debug().
				Int("type", messageType).
				Msg("VAD client: ignoring non-text message")
			continue
		}

		// Parse VAD event
		var event VADEvent
		if err := json.Unmarshal(message, &event); err != nil {
			log.Error().
				Err(err).
				Str("message", string(message)).
				Msg("VAD client: failed to parse event")
			continue
		}

		// Handle event based on type
		c.handleEvent(event)
	}
}

// handleEvent processes a VAD event and calls appropriate callbacks.
func (c *VADClient) handleEvent(event VADEvent) {
	log.Debug().
		Str("type", event.Type).
		Float64("timestamp", event.Timestamp).
		Float64("confidence", event.Confidence).
		Msg("VAD client: received event")

	switch event.Type {
	case "speech_start":
		if c.OnSpeechStart != nil {
			c.OnSpeechStart(event)
		}

	case "speech_end":
		if c.OnSpeechEnd != nil {
			// Decode base64 audio if present
			var audioData []byte
			if event.AudioBase64 != "" {
				var err error
				audioData, err = base64.StdEncoding.DecodeString(event.AudioBase64)
				if err != nil {
					log.Error().
						Err(err).
						Msg("VAD client: failed to decode audio data")
					// Still call callback with nil audio
					audioData = nil
				}
			}
			c.OnSpeechEnd(event, audioData)
		}

	case "interrupt":
		// User is interrupting during TTS playback (detected in PLAYBACK mode)
		log.Info().
			Float64("confidence", event.Confidence).
			Float64("duration_ms", event.DurationMs).
			Msg("VAD client: user interrupt detected during playback")

		if c.OnInterrupt != nil {
			// Decode base64 audio if present
			var audioData []byte
			if event.AudioBase64 != "" {
				var err error
				audioData, err = base64.StdEncoding.DecodeString(event.AudioBase64)
				if err != nil {
					log.Error().
						Err(err).
						Msg("VAD client: failed to decode interrupt audio data")
					audioData = nil
				}
			}
			c.OnInterrupt(event, audioData)
		}

	case "error":
		log.Error().
			Str("event_type", event.Type).
			Float64("timestamp", event.Timestamp).
			Msg("VAD client: received error event from server")
		if c.OnError != nil {
			c.OnError(fmt.Errorf("vad server error at timestamp %.3f", event.Timestamp))
		}

	default:
		log.Debug().
			Str("type", event.Type).
			Msg("VAD client: ignoring unknown event type")
	}
}

// pingLoop sends periodic ping messages to keep the connection alive.
func (c *VADClient) pingLoop() {
	ticker := time.NewTicker(c.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.mu.RLock()
			conn := c.conn
			running := c.running
			c.mu.RUnlock()

			if !running || conn == nil {
				return
			}

			c.mu.Lock()
			err := conn.WriteMessage(websocket.PingMessage, nil)
			c.mu.Unlock()

			if err != nil {
				log.Debug().
					Err(err).
					Msg("VAD client: ping failed")
				return
			}
		}
	}
}

// reconnect attempts to reconnect to the VAD server with exponential backoff.
func (c *VADClient) reconnect(ctx context.Context) {
	c.mu.Lock()
	if c.reconnecting {
		c.mu.Unlock()
		return
	}
	c.reconnecting = true
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.reconnecting = false
		c.mu.Unlock()
	}()

	wait := c.config.ReconnectWait
	maxReconnects := c.config.MaxReconnects
	attempts := 0

	for {
		select {
		case <-ctx.Done():
			log.Debug().Msg("VAD client: reconnection cancelled")
			return
		default:
		}

		// Check if max reconnects exceeded
		if maxReconnects > 0 && attempts >= maxReconnects {
			err := fmt.Errorf("vad client: max reconnection attempts (%d) exceeded", maxReconnects)
			log.Error().Err(err).Msg("VAD client: giving up reconnection")
			if c.OnError != nil {
				c.OnError(err)
			}
			c.mu.Lock()
			c.running = false
			c.mu.Unlock()
			return
		}

		attempts++
		log.Info().
			Int("attempt", attempts).
			Dur("wait", wait).
			Msg("VAD client: attempting reconnection")

		// Wait before reconnecting
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}

		// Configure WebSocket dialer
		dialer := websocket.Dialer{
			HandshakeTimeout: 5 * time.Second,
		}

		conn, _, err := dialer.DialContext(ctx, c.config.Endpoint, nil)
		if err != nil {
			log.Error().
				Err(err).
				Int("attempt", attempts).
				Msg("VAD client: reconnection failed")

			// Exponential backoff (cap at 30 seconds)
			wait = wait * 2
			if wait > 30*time.Second {
				wait = 30 * time.Second
			}
			continue
		}

		// Reconnection successful
		c.mu.Lock()
		if c.conn != nil {
			c.conn.Close()
		}
		c.conn = conn
		c.running = true
		c.mu.Unlock()

		log.Info().
			Int("attempts", attempts).
			Msg("VAD client: reconnected successfully")

		// Restart event listener
		go c.listenEvents()
		go c.pingLoop()
		return
	}
}

// SendAudioFrame sends a binary audio frame to the VAD server.
func (c *VADClient) SendAudioFrame(frame []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running || c.conn == nil {
		return fmt.Errorf("vad client: not connected")
	}

	if err := c.conn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
		return fmt.Errorf("vad client: failed to send audio frame: %w", err)
	}

	return nil
}

// Close closes the WebSocket connection and stops the client.
func (c *VADClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return nil
	}

	c.running = false

	// Cancel context to stop goroutines
	if c.cancel != nil {
		c.cancel()
	}

	// Close WebSocket connection
	if c.conn != nil {
		// Send close message
		err := c.conn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		)
		if err != nil {
			log.Debug().
				Err(err).
				Msg("VAD client: error sending close message")
		}

		if err := c.conn.Close(); err != nil {
			return fmt.Errorf("vad client: failed to close connection: %w", err)
		}
		c.conn = nil
	}

	log.Info().Msg("VAD client: closed")
	return nil
}

// IsConnected returns true if the client is currently connected.
func (c *VADClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running && c.conn != nil
}

// SetMode changes the VAD operating mode.
// PLAYBACK mode uses a higher threshold to avoid false triggers during TTS playback.
// FULL mode uses normal detection thresholds.
func (c *VADClient) SetMode(mode VADMode) error {
	c.mu.Lock()
	previousMode := c.currentMode
	c.mu.Unlock()

	log.Debug().
		Str("previous_mode", string(previousMode)).
		Str("new_mode", string(mode)).
		Msg("VAD client: changing mode")

	payload := map[string]string{"mode": string(mode)}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("vad client: failed to marshal mode payload: %w", err)
	}

	url := c.config.HTTPBaseURL + "/v1/vad/mode"
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("vad client: failed to set VAD mode: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Try to read error body for more context
		errBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("vad client: mode change failed: status %d, body: %s", resp.StatusCode, string(errBody))
	}

	// Update local state
	c.mu.Lock()
	c.currentMode = mode
	c.mu.Unlock()

	log.Info().
		Str("previous_mode", string(previousMode)).
		Str("new_mode", string(mode)).
		Str("endpoint", url).
		Msg("VAD client: mode changed successfully")

	return nil
}

// GetMode returns the current VAD operating mode from the server.
func (c *VADClient) GetMode() (VADMode, error) {
	url := c.config.HTTPBaseURL + "/v1/vad/mode"

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("vad client: failed to get VAD mode: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("vad client: get mode failed: status %d, body: %s", resp.StatusCode, string(errBody))
	}

	var result struct {
		Mode string `json:"mode"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("vad client: failed to decode mode response: %w", err)
	}

	mode := VADMode(result.Mode)

	// Update local state to stay in sync
	c.mu.Lock()
	c.currentMode = mode
	c.mu.Unlock()

	log.Debug().
		Str("mode", string(mode)).
		Msg("VAD client: retrieved current mode")

	return mode, nil
}

// CurrentMode returns the locally cached VAD mode (does not query server).
func (c *VADClient) CurrentMode() VADMode {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.currentMode
}
