// Package voice provides voice-related types and utilities for Cortex.
// wakeword_client.go provides a client for wake word detection events (CR-015).
package voice

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

// WakeWordEvent represents a wake word detection event from the server.
type WakeWordEvent struct {
	Type        string  `json:"type"`
	WakeWord    string  `json:"wake_word"`
	Confidence  float64 `json:"confidence"`
	Timestamp   float64 `json:"timestamp"`
	AudioBase64 string  `json:"audio_base64,omitempty"`
}

// WakeWordClientConfig holds configuration for the wake word client.
type WakeWordClientConfig struct {
	// Endpoint is the WebSocket endpoint for the voice orchestrator
	Endpoint string

	// WakeWords is the list of wake words to detect
	WakeWords []string

	// Threshold is the detection threshold (0.0-1.0)
	Threshold float64

	// Enabled enables/disables wake word detection
	Enabled bool
}

// DefaultWakeWordClientConfig returns production defaults for the wake word client.
func DefaultWakeWordClientConfig() WakeWordClientConfig {
	return WakeWordClientConfig{
		Endpoint:  "ws://127.0.0.1:8765/ws/voice",
		WakeWords: []string{"hey_cortex", "hey_henry", "hey_hannah", "cortex", "henry", "hannah"},
		Threshold: 0.5,
		Enabled:   true,
	}
}

// WakeWordClient listens for wake word detection events from the voice orchestrator.
type WakeWordClient struct {
	mu           sync.RWMutex
	config       WakeWordClientConfig
	conn         *websocket.Conn
	running      bool
	ctx          context.Context
	cancel       context.CancelFunc
	lastDetected string
	lastTime     time.Time

	// Callbacks
	OnWakeWord func(event WakeWordEvent)
	OnError    func(err error)
}

// NewWakeWordClient creates a new wake word client with the given configuration.
func NewWakeWordClient(config WakeWordClientConfig) *WakeWordClient {
	if config.Endpoint == "" {
		config = DefaultWakeWordClientConfig()
	}

	return &WakeWordClient{
		config: config,
	}
}

// Connect establishes connection and sends wake word configuration.
// Note: This uses the same WebSocket connection as the voice orchestrator.
// Wake word events are received as part of the normal message stream.
func (c *WakeWordClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return fmt.Errorf("wake word client: already running")
	}

	// Create a child context for managing goroutines
	c.ctx, c.cancel = context.WithCancel(ctx)
	c.running = true

	log.Info().
		Strs("wake_words", c.config.WakeWords).
		Float64("threshold", c.config.Threshold).
		Bool("enabled", c.config.Enabled).
		Msg("WakeWordClient initialized")

	return nil
}

// SendConfig sends wake word configuration to the voice orchestrator.
// This should be called after connecting to enable wake word detection.
func (c *WakeWordClient) SendConfig(conn *websocket.Conn) error {
	c.mu.RLock()
	enabled := c.config.Enabled
	wakeWords := c.config.WakeWords
	threshold := c.config.Threshold
	c.mu.RUnlock()

	configMsg := map[string]interface{}{
		"type":       "wake_word_config",
		"enabled":    enabled,
		"wake_words": wakeWords,
		"threshold":  threshold,
		"timestamp":  float64(time.Now().UnixNano()) / 1e9,
	}

	data, err := json.Marshal(configMsg)
	if err != nil {
		return fmt.Errorf("wake word client: failed to marshal config: %w", err)
	}

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("wake word client: failed to send config: %w", err)
	}

	log.Debug().
		Bool("enabled", enabled).
		Strs("wake_words", wakeWords).
		Float64("threshold", threshold).
		Msg("WakeWordClient: sent configuration")

	return nil
}

// HandleMessage processes an incoming message and checks if it's a wake word event.
// Returns true if the message was a wake word event, false otherwise.
func (c *WakeWordClient) HandleMessage(data []byte) bool {
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		return false
	}

	msgType, ok := msg["type"].(string)
	if !ok || msgType != "wake_word" {
		return false
	}

	// Parse wake word event
	var event WakeWordEvent
	if err := json.Unmarshal(data, &event); err != nil {
		log.Error().Err(err).Msg("WakeWordClient: failed to parse wake word event")
		return false
	}

	c.mu.Lock()
	c.lastDetected = event.WakeWord
	c.lastTime = time.Now()
	c.mu.Unlock()

	log.Info().
		Str("wake_word", event.WakeWord).
		Float64("confidence", event.Confidence).
		Msg("WakeWordClient: wake word detected")

	// Fire callback
	if c.OnWakeWord != nil {
		go c.OnWakeWord(event)
	}

	return true
}

// SetConfig updates the wake word configuration.
func (c *WakeWordClient) SetConfig(enabled bool, wakeWords []string, threshold float64) {
	c.mu.Lock()
	c.config.Enabled = enabled
	if wakeWords != nil {
		c.config.WakeWords = wakeWords
	}
	if threshold > 0 {
		c.config.Threshold = threshold
	}
	c.mu.Unlock()
}

// IsEnabled returns whether wake word detection is enabled.
func (c *WakeWordClient) IsEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config.Enabled
}

// GetLastDetected returns the last detected wake word and when it was detected.
func (c *WakeWordClient) GetLastDetected() (string, time.Time) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastDetected, c.lastTime
}

// WasRecentlyDetected returns true if a wake word was detected within the given duration.
func (c *WakeWordClient) WasRecentlyDetected(within time.Duration) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.lastDetected == "" {
		return false
	}

	return time.Since(c.lastTime) < within
}

// ClearLastDetected clears the last detected wake word state.
func (c *WakeWordClient) ClearLastDetected() {
	c.mu.Lock()
	c.lastDetected = ""
	c.lastTime = time.Time{}
	c.mu.Unlock()
}

// Close stops the wake word client.
func (c *WakeWordClient) Close() error {
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

	log.Info().Msg("WakeWordClient: closed")
	return nil
}

// GetConfig returns a copy of the current configuration.
func (c *WakeWordClient) GetConfig() WakeWordClientConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config
}
