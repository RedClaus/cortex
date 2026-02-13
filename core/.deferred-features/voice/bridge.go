package voice

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/normanking/cortex/internal/bus"
	"github.com/normanking/cortex/internal/logging"
	pkgvoice "github.com/normanking/cortex/pkg/voice"
)

// ConnectionState represents the state of the WebSocket connection.
type ConnectionState int

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateConnected
	StateReconnecting
)

func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateReconnecting:
		return "reconnecting"
	default:
		return "unknown"
	}
}

// BridgeConfig holds configuration for the voice bridge.
type BridgeConfig struct {
	// OrchestratorURL is the WebSocket URL for the voice orchestrator
	OrchestratorURL string

	// SessionID is the unique session identifier
	SessionID string

	// InitialReconnectDelay is the initial delay between reconnection attempts
	InitialReconnectDelay time.Duration

	// MaxReconnectDelay is the maximum delay between reconnection attempts (for backoff)
	MaxReconnectDelay time.Duration

	// MaxReconnects is the maximum number of reconnection attempts (0 = infinite)
	MaxReconnects int

	// PingInterval is how often to send ping messages to keep the connection alive
	PingInterval time.Duration

	// PongTimeout is how long to wait for a pong response before considering the connection dead
	PongTimeout time.Duration

	// WriteTimeout is the timeout for write operations
	WriteTimeout time.Duration

	// ReadTimeout is the timeout for read operations (should be > PingInterval + PongTimeout)
	ReadTimeout time.Duration

	// MaxPendingMessages is the maximum number of messages to queue during reconnection
	MaxPendingMessages int
}

// DefaultBridgeConfig returns sensible defaults for the voice bridge.
func DefaultBridgeConfig() BridgeConfig {
	return BridgeConfig{
		OrchestratorURL:       "ws://localhost:8765",
		SessionID:             "default",
		InitialReconnectDelay: 1 * time.Second,
		MaxReconnectDelay:     30 * time.Second,
		MaxReconnects:         10,
		PingInterval:          30 * time.Second,
		PongTimeout:           60 * time.Second,
		WriteTimeout:          10 * time.Second,
		ReadTimeout:           120 * time.Second, // Should be > PingInterval + PongTimeout
		MaxPendingMessages:    50,
	}
}

// VoiceBridge manages the WebSocket connection to the Python voice orchestrator.
// It handles bidirectional communication for voice transcripts and interrupts.
type VoiceBridge struct {
	config BridgeConfig
	conn   *websocket.Conn
	mu     sync.RWMutex

	// Connection state
	state          ConnectionState
	reconnectCount int
	lastPong       time.Time
	ctx            context.Context
	cancel         context.CancelFunc

	// Event bus for publishing voice events
	eventBus *bus.EventBus

	// Event handlers (multiple handlers supported)
	onInterruptHandlers  []func(reason string)
	onTranscriptHandlers []func(text string, isFinal bool)
	onStatusHandlers     []func(state string)
	handlersMu           sync.RWMutex

	// Interrupt channel
	interruptChan chan *InterruptSignal
	sendChan      chan []byte

	// Message queue for reconnection (holds messages while reconnecting)
	pendingMessages [][]byte
	pendingMu       sync.Mutex

	// Client connections for WebSocket passthrough
	clients   map[*websocket.Conn]struct{}
	clientsMu sync.RWMutex

	// Logger
	log *logging.Logger
}

// VoiceMessage represents a message sent to/from the voice orchestrator.
type VoiceMessage struct {
	Type      string                 `json:"type"` // "transcript", "interrupt", "config", "status", "error", "wake_word"
	Text      string                 `json:"text,omitempty"`
	IsFinal   bool                   `json:"is_final,omitempty"`
	Reason    string                 `json:"reason,omitempty"`  // For interrupt messages
	Error     string                 `json:"error,omitempty"`   // For error messages
	Code      string                 `json:"code,omitempty"`    // Error code
	Status    string                 `json:"status,omitempty"`  // For status messages
	Details   map[string]interface{} `json:"details,omitempty"` // For status details
	Data      string                 `json:"data,omitempty"`    // For audio_out messages (base64)
	Timestamp float64                `json:"timestamp,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	// Config fields
	Voice        string  `json:"voice,omitempty"`         // TTS voice name
	VadThreshold float64 `json:"vad_threshold,omitempty"` // VAD threshold
	ModelSize    string  `json:"model_size,omitempty"`    // STT model size
	Language     string  `json:"language,omitempty"`      // STT language
	// CR-015: Wake word fields
	WakeWord    string  `json:"wake_word,omitempty"`    // Detected wake word name
	Confidence  float64 `json:"confidence,omitempty"`   // Wake word detection confidence
	AudioBase64 string  `json:"audio_base64,omitempty"` // Pre-detection audio buffer
}

// NewVoiceBridge creates a new voice bridge with the given configuration.
func NewVoiceBridge(config BridgeConfig) *VoiceBridge {
	ctx, cancel := context.WithCancel(context.Background())
	return &VoiceBridge{
		config:          config,
		state:           StateDisconnected,
		ctx:             ctx,
		cancel:          cancel,
		eventBus:        nil, // Set via SetEventBus if needed
		interruptChan:   make(chan *InterruptSignal, 10),
		sendChan:        make(chan []byte, 100),
		pendingMessages: make([][]byte, 0, config.MaxPendingMessages),
		clients:         make(map[*websocket.Conn]struct{}),
		log:             logging.Global(),
	}
}

// SetEventBus sets the event bus for publishing voice events.
func (vb *VoiceBridge) SetEventBus(eventBus *bus.EventBus) {
	vb.mu.Lock()
	defer vb.mu.Unlock()
	vb.eventBus = eventBus
}

// GetState returns the current connection state.
func (vb *VoiceBridge) GetState() ConnectionState {
	vb.mu.RLock()
	defer vb.mu.RUnlock()
	return vb.state
}

// setState updates the connection state and logs the transition.
func (vb *VoiceBridge) setState(newState ConnectionState) {
	vb.mu.Lock()
	oldState := vb.state
	vb.state = newState
	vb.mu.Unlock()

	if oldState != newState {
		vb.log.Info("[VoiceBridge] State change: %s -> %s", oldState, newState)
	}
}

// Connect establishes a WebSocket connection to the voice orchestrator.
func (vb *VoiceBridge) Connect(ctx context.Context) error {
	vb.mu.Lock()
	if vb.state == StateConnected || vb.state == StateConnecting {
		vb.mu.Unlock()
		return nil // Already connected or connecting
	}
	vb.state = StateConnecting
	vb.mu.Unlock()

	// Parse WebSocket URL
	u, err := url.Parse(vb.config.OrchestratorURL)
	if err != nil {
		vb.setState(StateDisconnected)
		return fmt.Errorf("invalid orchestrator URL: %w", err)
	}

	// Connect to WebSocket
	vb.log.Info("[VoiceBridge] Connecting to voice orchestrator: %s", vb.config.OrchestratorURL)

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		vb.setState(StateDisconnected)
		return fmt.Errorf("failed to connect to voice orchestrator: %w", err)
	}

	// Configure connection timeouts
	vb.mu.Lock()
	vb.conn = conn
	vb.state = StateConnected
	vb.reconnectCount = 0
	vb.lastPong = time.Now()
	vb.mu.Unlock()

	// Set up pong handler to track last pong time
	conn.SetPongHandler(func(appData string) error {
		vb.mu.Lock()
		vb.lastPong = time.Now()
		vb.mu.Unlock()
		vb.log.Debug("[VoiceBridge] Pong received")
		return nil
	})

	// Set read deadline based on config
	if err := conn.SetReadDeadline(time.Now().Add(vb.config.ReadTimeout)); err != nil {
		vb.log.Warn("[VoiceBridge] Failed to set read deadline: %v", err)
	}

	vb.log.Info("[VoiceBridge] Connected to voice orchestrator")

	// Publish connected event
	if vb.eventBus != nil {
		event := NewVoiceConnectedEvent(vb.config.SessionID, vb.config.OrchestratorURL)
		vb.eventBus.Publish(event)
	}

	// Flush any pending messages
	vb.flushPendingMessages()

	// Start message handler
	go vb.handleMessages()

	// Start write loop
	go vb.writeLoop()

	// Start ping loop
	go vb.pingLoop()

	// Start interrupt handler
	go vb.interruptLoop()

	return nil
}

// flushPendingMessages sends any messages queued during reconnection.
func (vb *VoiceBridge) flushPendingMessages() {
	vb.pendingMu.Lock()
	pending := vb.pendingMessages
	vb.pendingMessages = make([][]byte, 0, vb.config.MaxPendingMessages)
	vb.pendingMu.Unlock()

	if len(pending) > 0 {
		vb.log.Info("[VoiceBridge] Flushing %d pending messages", len(pending))
		for _, msg := range pending {
			select {
			case vb.sendChan <- msg:
			default:
				vb.log.Warn("[VoiceBridge] Send channel full, dropping pending message")
			}
		}
	}
}

// queueMessage queues a message to be sent when connection is restored.
func (vb *VoiceBridge) queueMessage(data []byte) bool {
	vb.pendingMu.Lock()
	defer vb.pendingMu.Unlock()

	if len(vb.pendingMessages) >= vb.config.MaxPendingMessages {
		vb.log.Warn("[VoiceBridge] Pending message queue full, dropping oldest message")
		// Drop oldest message
		vb.pendingMessages = vb.pendingMessages[1:]
	}

	vb.pendingMessages = append(vb.pendingMessages, data)
	vb.log.Debug("[VoiceBridge] Message queued for reconnection (%d pending)", len(vb.pendingMessages))
	return true
}

// handleMessages processes incoming messages from the voice orchestrator.
func (vb *VoiceBridge) handleMessages() {
	defer func() {
		vb.handleDisconnect()
	}()

	for {
		select {
		case <-vb.ctx.Done():
			return
		default:
			vb.mu.RLock()
			conn := vb.conn
			vb.mu.RUnlock()

			if conn == nil {
				return
			}

			// Reset read deadline before each read
			if err := conn.SetReadDeadline(time.Now().Add(vb.config.ReadTimeout)); err != nil {
				vb.log.Warn("[VoiceBridge] Failed to set read deadline: %v", err)
			}

			// Read message
			_, data, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
					vb.log.Error("[VoiceBridge] Unexpected websocket close: %v", err)
				} else if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					vb.log.Info("[VoiceBridge] WebSocket closed normally: %v", err)
				} else {
					vb.log.Error("[VoiceBridge] WebSocket read error: %v", err)
				}
				return
			}

			// Parse message
			var msg VoiceMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				vb.log.Error("[VoiceBridge] Failed to parse voice message: %v", err)
				continue
			}

			// Route message to handler
			vb.handleMessage(&msg)

			// Broadcast to clients
			vb.broadcastToClients(data)
		}
	}
}

// handleMessage routes a message to the appropriate handler.
func (vb *VoiceBridge) handleMessage(msg *VoiceMessage) {
	switch msg.Type {
	case "transcript":
		// Publish event
		confidence := 0.0
		if c, ok := msg.Metadata["confidence"].(float64); ok {
			confidence = c
		}
		language := ""
		if l, ok := msg.Metadata["language"].(string); ok {
			language = l
		}

		// Clean up STT artifacts from the transcript
		originalText := msg.Text
		cleanedText, hadWakeWord := pkgvoice.CleanTranscriptionWithInfo(msg.Text)

		// Log if cleanup made changes
		if cleanedText != originalText && msg.IsFinal {
			vb.log.Info("[VoiceBridge] STT Cleanup: %q -> %q (wake_word=%v)", originalText, cleanedText, hadWakeWord)
		}

		// Use cleaned text for events and handlers
		if vb.eventBus != nil {
			event := NewVoiceTranscriptEvent(vb.config.SessionID, cleanedText, msg.IsFinal, confidence)
			// Store original text for debugging
			event.OriginalText = originalText
			event.WasCleaned = cleanedText != originalText
			event.HadWakeWord = hadWakeWord
			vb.log.Info("[VoiceBridge] Publishing transcript event: text=%q isFinal=%v wake_word=%v", cleanedText, msg.IsFinal, hadWakeWord)
			vb.eventBus.Publish(event)
		} else {
			vb.log.Warn("[VoiceBridge] EventBus is nil, cannot publish transcript event!")
		}

		// Call registered handlers with cleaned text
		vb.handlersMu.RLock()
		handlers := vb.onTranscriptHandlers
		vb.handlersMu.RUnlock()

		for _, handler := range handlers {
			go handler(cleanedText, msg.IsFinal)
		}

		// Enhanced logging for transcripts
		if msg.IsFinal {
			vb.log.Info("[VoiceBridge] STT Final transcript: text=%q confidence=%.2f lang=%s", cleanedText, confidence, language)
		} else {
			vb.log.Debug("[VoiceBridge] STT Interim transcript: text=%q", cleanedText)
		}

	case "interrupt":
		// Publish event
		interruptType := "unknown"
		if t, ok := msg.Metadata["interrupt_type"].(string); ok {
			interruptType = t
		}

		if vb.eventBus != nil {
			event := NewVoiceInterruptEvent(vb.config.SessionID, interruptType, msg.Reason)
			vb.eventBus.PublishAsync(event)
		}

		// Call registered handlers
		vb.handlersMu.RLock()
		handlers := vb.onInterruptHandlers
		vb.handlersMu.RUnlock()

		for _, handler := range handlers {
			go handler(msg.Reason)
		}

		// Log interrupt prominently
		vb.log.Info("[VoiceBridge] INTERRUPT received: type=%s reason=%s", interruptType, msg.Reason)

	case "status":
		// Get status from the Status field (Python sends "status" field)
		state := msg.Status
		if state == "" {
			// Fallback to metadata if Status field is empty
			if s, ok := msg.Metadata["state"].(string); ok {
				state = s
			}
		}

		// Publish event
		if vb.eventBus != nil {
			event := NewVoiceStatusEvent(vb.config.SessionID, state)
			event.Metadata = msg.Details
			if event.Metadata == nil {
				event.Metadata = msg.Metadata
			}
			vb.eventBus.PublishAsync(event)
		}

		// Call registered handlers
		vb.handlersMu.RLock()
		handlers := vb.onStatusHandlers
		vb.handlersMu.RUnlock()

		for _, handler := range handlers {
			go handler(state)
		}

		vb.log.Info("[VoiceBridge] Status: %s", state)

	case "synthesizing":
		// Publish event
		voiceID := ""
		if v, ok := msg.Metadata["voice_id"].(string); ok {
			voiceID = v
		}
		provider := ""
		if p, ok := msg.Metadata["provider"].(string); ok {
			provider = p
		}

		if vb.eventBus != nil {
			event := NewVoiceSynthesizingEvent(vb.config.SessionID, msg.Text, voiceID)
			event.Provider = provider
			vb.eventBus.PublishAsync(event)
		}

		// Log TTS start
		textPreview := msg.Text
		if len(textPreview) > 50 {
			textPreview = textPreview[:50] + "..."
		}
		vb.log.Info("[VoiceBridge] TTS Synthesizing: voice=%s provider=%s text=%q", voiceID, provider, textPreview)

	case "complete":
		// Publish event
		duration := time.Duration(0)
		if d, ok := msg.Metadata["duration_ms"].(float64); ok {
			duration = time.Duration(d) * time.Millisecond
		}
		provider := ""
		if p, ok := msg.Metadata["provider"].(string); ok {
			provider = p
		}
		voiceID := ""
		if v, ok := msg.Metadata["voice_id"].(string); ok {
			voiceID = v
		}

		if vb.eventBus != nil {
			event := NewVoiceCompleteEvent(vb.config.SessionID, msg.Text, duration)
			event.Provider = provider
			event.VoiceID = voiceID
			vb.eventBus.PublishAsync(event)
		}

		// Log TTS completion
		vb.log.Info("[VoiceBridge] TTS Complete: duration=%v provider=%s voice=%s", duration, provider, voiceID)

	case "vad_start":
		vb.log.Info("[VoiceBridge] VAD: Speech started (user is speaking)")

	case "vad_end":
		vb.log.Info("[VoiceBridge] VAD: Speech ended (user stopped speaking)")

	case "audio_out":
		// Audio output chunk - don't log every chunk, just track it
		vb.log.Debug("[VoiceBridge] TTS audio chunk received")

	case "error":
		// Get error message from Error field (Python sends "error" field)
		errorMsg := msg.Error
		if errorMsg == "" {
			errorMsg = msg.Reason // Fallback to reason
		}
		errorCode := msg.Code

		// Publish event
		if vb.eventBus != nil {
			component := ""
			if c, ok := msg.Details["component"].(string); ok {
				component = c
			}
			recoverable, _ := msg.Details["recoverable"].(bool)
			event := NewVoiceErrorEvent(vb.config.SessionID, errorMsg, component, recoverable)
			vb.eventBus.PublishAsync(event)
		}

		// Enhanced error logging with full message details
		vb.log.Error("[VoiceBridge] Error from orchestrator: error=%q code=%q details=%+v",
			errorMsg, errorCode, msg.Details)

	case "wake_word":
		// CR-015: Wake word detected (pre-STT hotword detection)
		wakeWord := msg.WakeWord
		if wakeWord == "" {
			// Fallback to metadata
			if w, ok := msg.Metadata["wake_word"].(string); ok {
				wakeWord = w
			}
		}
		confidence := msg.Confidence
		if confidence == 0 {
			if c, ok := msg.Metadata["confidence"].(float64); ok {
				confidence = c
			}
		}
		audioBase64 := msg.AudioBase64
		if audioBase64 == "" {
			if a, ok := msg.Metadata["audio_base64"].(string); ok {
				audioBase64 = a
			}
		}

		// Publish event
		if vb.eventBus != nil {
			event := NewVoiceWakeWordEvent(vb.config.SessionID, wakeWord, confidence)
			event.AudioBase64 = audioBase64
			vb.eventBus.PublishAsync(event)
		}

		vb.log.Info("[VoiceBridge] Wake word detected: word=%s confidence=%.2f", wakeWord, confidence)

	default:
		vb.log.Debug("[VoiceBridge] Unknown voice message type: %s", msg.Type)
	}
}

// writeLoop writes messages to the orchestrator WebSocket.
func (vb *VoiceBridge) writeLoop() {
	for {
		select {
		case <-vb.ctx.Done():
			return

		case message := <-vb.sendChan:
			vb.mu.RLock()
			conn := vb.conn
			state := vb.state
			vb.mu.RUnlock()

			// If reconnecting, queue the message
			if state == StateReconnecting {
				vb.queueMessage(message)
				continue
			}

			if state != StateConnected || conn == nil {
				vb.log.Debug("[VoiceBridge] Not connected, dropping message")
				continue
			}

			// Set write deadline
			if err := conn.SetWriteDeadline(time.Now().Add(vb.config.WriteTimeout)); err != nil {
				vb.log.Warn("[VoiceBridge] Failed to set write deadline: %v", err)
			}

			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				vb.log.Error("[VoiceBridge] Failed to write message to orchestrator: %v", err)
				// Queue for retry and trigger disconnect handling
				vb.queueMessage(message)
				return
			}
		}
	}
}

// interruptLoop handles interrupt signals.
func (vb *VoiceBridge) interruptLoop() {
	for {
		select {
		case <-vb.ctx.Done():
			return

		case interrupt := <-vb.interruptChan:
			// Send interrupt to orchestrator
			msg := VoiceMessage{
				Type:      "interrupt",
				Reason:    interrupt.Reason,
				Timestamp: float64(interrupt.Timestamp.UnixNano()) / 1e9,
				Metadata: map[string]interface{}{
					"interrupt_type": interrupt.Type.String(),
					"session_id":     interrupt.SessionID,
				},
			}

			// Merge interrupt metadata
			for k, v := range interrupt.Metadata {
				msg.Metadata[k] = v
			}

			data, err := json.Marshal(msg)
			if err != nil {
				vb.log.Error("[VoiceBridge] Failed to marshal interrupt message: %v", err)
				continue
			}

			select {
			case vb.sendChan <- data:
			default:
				vb.log.Debug("[VoiceBridge] Send buffer full, dropping interrupt")
			}

			// Publish interrupt event
			if vb.eventBus != nil {
				event := NewVoiceInterruptEvent(interrupt.SessionID, interrupt.Type.String(), interrupt.Reason)
				event.Metadata = interrupt.Metadata
				vb.eventBus.PublishAsync(event)
			}
		}
	}
}

// calculateBackoff returns the backoff duration using exponential backoff with jitter.
func (vb *VoiceBridge) calculateBackoff() time.Duration {
	// Exponential backoff: initial * 2^(attempt-1)
	backoff := vb.config.InitialReconnectDelay
	for i := 1; i < vb.reconnectCount; i++ {
		backoff *= 2
		if backoff > vb.config.MaxReconnectDelay {
			backoff = vb.config.MaxReconnectDelay
			break
		}
	}
	return backoff
}

// handleDisconnect handles connection loss and attempts reconnection.
func (vb *VoiceBridge) handleDisconnect() {
	vb.mu.Lock()
	wasConnected := vb.state == StateConnected
	if vb.state == StateDisconnected || vb.state == StateReconnecting {
		vb.mu.Unlock()
		return
	}

	// Close existing connection
	if vb.conn != nil {
		vb.conn.Close()
		vb.conn = nil
	}

	vb.state = StateReconnecting
	vb.mu.Unlock()

	if !wasConnected {
		return
	}

	vb.log.Info("[VoiceBridge] Voice orchestrator connection lost")

	// Publish disconnected event
	if vb.eventBus != nil {
		event := NewVoiceDisconnectedEvent(vb.config.SessionID, "connection lost")
		vb.eventBus.PublishAsync(event)
	}

	// Attempt reconnection with exponential backoff
	vb.attemptReconnection()
}

// attemptReconnection tries to reconnect with exponential backoff.
func (vb *VoiceBridge) attemptReconnection() {
	for {
		select {
		case <-vb.ctx.Done():
			vb.setState(StateDisconnected)
			return
		default:
		}

		// Check if max reconnects reached
		if vb.config.MaxReconnects > 0 && vb.reconnectCount >= vb.config.MaxReconnects {
			vb.log.Error("[VoiceBridge] Max reconnection attempts (%d) reached, giving up", vb.config.MaxReconnects)
			vb.setState(StateDisconnected)

			// Publish error event
			if vb.eventBus != nil {
				event := NewVoiceErrorEvent(vb.config.SessionID, "max reconnection attempts reached", "bridge", false)
				vb.eventBus.PublishAsync(event)
			}
			return
		}

		vb.reconnectCount++
		backoff := vb.calculateBackoff()

		vb.log.Info("[VoiceBridge] Attempting to reconnect to voice orchestrator (attempt %d/%d, backoff %v)",
			vb.reconnectCount, vb.config.MaxReconnects, backoff)

		// Wait with backoff
		select {
		case <-vb.ctx.Done():
			vb.setState(StateDisconnected)
			return
		case <-time.After(backoff):
		}

		// Try to connect
		vb.mu.Lock()
		vb.state = StateConnecting
		vb.mu.Unlock()

		if err := vb.doConnect(); err != nil {
			vb.log.Error("[VoiceBridge] Reconnection failed: %v", err)
			vb.setState(StateReconnecting)
			continue
		}

		// Successfully reconnected
		vb.log.Info("[VoiceBridge] Successfully reconnected to voice orchestrator")
		return
	}
}

// doConnect performs the actual connection without locking.
func (vb *VoiceBridge) doConnect() error {
	// Parse WebSocket URL
	u, err := url.Parse(vb.config.OrchestratorURL)
	if err != nil {
		return fmt.Errorf("invalid orchestrator URL: %w", err)
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(vb.ctx, u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to connect to voice orchestrator: %w", err)
	}

	// Configure connection
	vb.mu.Lock()
	vb.conn = conn
	vb.state = StateConnected
	vb.lastPong = time.Now()
	vb.mu.Unlock()

	// Set up pong handler
	conn.SetPongHandler(func(appData string) error {
		vb.mu.Lock()
		vb.lastPong = time.Now()
		vb.mu.Unlock()
		vb.log.Debug("[VoiceBridge] Pong received")
		return nil
	})

	// Set read deadline
	if err := conn.SetReadDeadline(time.Now().Add(vb.config.ReadTimeout)); err != nil {
		vb.log.Warn("[VoiceBridge] Failed to set read deadline: %v", err)
	}

	// Publish connected event
	if vb.eventBus != nil {
		event := NewVoiceConnectedEvent(vb.config.SessionID, vb.config.OrchestratorURL)
		vb.eventBus.Publish(event)
	}

	// Flush pending messages
	vb.flushPendingMessages()

	// Restart message handlers
	go vb.handleMessages()
	go vb.writeLoop()
	go vb.pingLoop()

	return nil
}

// pingLoop sends periodic ping messages to keep the connection alive.
func (vb *VoiceBridge) pingLoop() {
	ticker := time.NewTicker(vb.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-vb.ctx.Done():
			return
		case <-ticker.C:
			vb.mu.RLock()
			conn := vb.conn
			state := vb.state
			lastPong := vb.lastPong
			vb.mu.RUnlock()

			if state != StateConnected || conn == nil {
				return // Exit ping loop if not connected
			}

			// Check if pong timeout exceeded
			if time.Since(lastPong) > vb.config.PongTimeout {
				vb.log.Error("[VoiceBridge] Pong timeout exceeded (%v since last pong), triggering reconnection",
					time.Since(lastPong))
				vb.handleDisconnect()
				return
			}

			// Set write deadline for ping
			if err := conn.SetWriteDeadline(time.Now().Add(vb.config.WriteTimeout)); err != nil {
				vb.log.Warn("[VoiceBridge] Failed to set write deadline for ping: %v", err)
			}

			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				vb.log.Error("[VoiceBridge] Failed to send ping: %v", err)
				vb.handleDisconnect()
				return
			}
			vb.log.Debug("[VoiceBridge] Ping sent")
		}
	}
}

// ensureConnected verifies connection is alive and triggers reconnection if needed.
// Returns true if connected, false otherwise.
func (vb *VoiceBridge) ensureConnected() bool {
	vb.mu.RLock()
	state := vb.state
	vb.mu.RUnlock()

	if state == StateConnected {
		return true
	}

	if state == StateReconnecting {
		// Already reconnecting, wait briefly for connection
		vb.log.Debug("[VoiceBridge] Waiting for reconnection...")
		return false
	}

	// Not connected and not reconnecting, trigger reconnection
	if state == StateDisconnected {
		vb.log.Info("[VoiceBridge] ensureConnected: triggering reconnection")
		go vb.attemptReconnection()
	}

	return false
}

// SendText sends text to the TTS pipeline for synthesis.
func (vb *VoiceBridge) SendText(text string) error {
	state := vb.GetState()

	vb.log.Debug("[VoiceBridge] SendText called: state=%s text=%q", state, text)

	msg := VoiceMessage{
		Type:      "synthesize",
		Text:      text,
		Timestamp: float64(time.Now().UnixNano()) / 1e9,
		Metadata: map[string]interface{}{
			"session_id": vb.config.SessionID,
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		vb.log.Error("[VoiceBridge] SendText failed to marshal: %v", err)
		return fmt.Errorf("failed to marshal synthesize message: %w", err)
	}

	// If reconnecting, queue the message
	if state == StateReconnecting {
		vb.queueMessage(data)
		vb.log.Info("[VoiceBridge] SendText: message queued during reconnection")
		return nil
	}

	if state != StateConnected {
		// Try to trigger reconnection
		if !vb.ensureConnected() {
			vb.log.Error("[VoiceBridge] SendText failed: not connected to orchestrator")
			return errors.New("not connected to orchestrator")
		}
	}

	vb.log.Info("[VoiceBridge] Sending synthesize message: %s", string(data))

	select {
	case vb.sendChan <- data:
		vb.log.Debug("[VoiceBridge] SendText queued successfully")
		return nil
	case <-time.After(5 * time.Second):
		vb.log.Error("[VoiceBridge] SendText timeout after 5s")
		return errors.New("timeout sending text to orchestrator")
	}
}

// Cancel sends a cancel command to stop current TTS playback.
func (vb *VoiceBridge) Cancel() error {
	state := vb.GetState()

	if state != StateConnected {
		return errors.New("not connected to orchestrator")
	}

	msg := VoiceMessage{
		Type:      "cancel",
		Timestamp: float64(time.Now().UnixNano()) / 1e9,
		Metadata: map[string]interface{}{
			"session_id": vb.config.SessionID,
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal cancel message: %w", err)
	}

	select {
	case vb.sendChan <- data:
		return nil
	case <-time.After(5 * time.Second):
		return errors.New("timeout sending cancel to orchestrator")
	}
}

// SendAudio sends pre-cached audio bytes to the orchestrator for playback (CR-012-C).
// This is the fast path for wake responses and backchannels.
func (vb *VoiceBridge) SendAudio(audioData []byte, format string) error {
	state := vb.GetState()

	vb.log.Debug("[VoiceBridge] SendAudio called: state=%s bytes=%d format=%s", state, len(audioData), format)

	if state != StateConnected {
		vb.log.Error("[VoiceBridge] SendAudio failed: not connected to orchestrator")
		return errors.New("not connected to orchestrator")
	}

	// Base64 encode the audio data
	audioBase64 := base64.StdEncoding.EncodeToString(audioData)

	msg := VoiceMessage{
		Type:      "play_audio",
		Data:      audioBase64,
		Timestamp: float64(time.Now().UnixNano()) / 1e9,
		Metadata: map[string]interface{}{
			"session_id": vb.config.SessionID,
			"format":     format,
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		vb.log.Error("[VoiceBridge] SendAudio failed to marshal: %v", err)
		return fmt.Errorf("failed to marshal play_audio message: %w", err)
	}

	vb.log.Info("[VoiceBridge] Sending play_audio message: %d bytes", len(audioData))

	select {
	case vb.sendChan <- data:
		vb.log.Debug("[VoiceBridge] SendAudio queued successfully")
		return nil
	case <-time.After(5 * time.Second):
		vb.log.Error("[VoiceBridge] SendAudio timeout after 5s")
		return errors.New("timeout sending audio to orchestrator")
	}
}

// SendInterrupt sends an interrupt signal to the orchestrator and event bus.
func (vb *VoiceBridge) SendInterrupt(signal *InterruptSignal) {
	select {
	case vb.interruptChan <- signal:
	default:
		vb.log.Debug("[VoiceBridge] Interrupt channel full, dropping signal")
	}
}

// OnInterrupt registers a callback for interrupt events.
func (vb *VoiceBridge) OnInterrupt(handler func(reason string)) {
	vb.handlersMu.Lock()
	defer vb.handlersMu.Unlock()
	vb.onInterruptHandlers = append(vb.onInterruptHandlers, handler)
}

// OnTranscript registers a callback for transcript events.
func (vb *VoiceBridge) OnTranscript(handler func(text string, isFinal bool)) {
	vb.handlersMu.Lock()
	defer vb.handlersMu.Unlock()
	vb.onTranscriptHandlers = append(vb.onTranscriptHandlers, handler)
}

// OnStatus registers a callback for status events.
func (vb *VoiceBridge) OnStatus(handler func(state string)) {
	vb.handlersMu.Lock()
	defer vb.handlersMu.Unlock()
	vb.onStatusHandlers = append(vb.onStatusHandlers, handler)
}

// SendConfig sends configuration to the voice orchestrator.
func (vb *VoiceBridge) SendConfig(config map[string]interface{}) error {
	vb.mu.RLock()
	state := vb.state
	conn := vb.conn
	vb.mu.RUnlock()

	vb.log.Debug("[VoiceBridge] SendConfig called: state=%s config=%+v", state, config)

	if state != StateConnected || conn == nil {
		vb.log.Error("[VoiceBridge] SendConfig failed: not connected to voice orchestrator")
		return fmt.Errorf("not connected to voice orchestrator")
	}

	msg := VoiceMessage{
		Type:      "config",
		Timestamp: float64(time.Now().UnixNano()) / 1e9,
		Metadata:  config,
	}

	// Extract known config fields to top-level
	if voice, ok := config["voice"].(string); ok {
		msg.Voice = voice
	}
	if vadThreshold, ok := config["vad_threshold"].(float64); ok {
		msg.VadThreshold = vadThreshold
	}
	if modelSize, ok := config["model_size"].(string); ok {
		msg.ModelSize = modelSize
	}
	if language, ok := config["language"].(string); ok {
		msg.Language = language
	}

	data, err := json.Marshal(msg)
	if err != nil {
		vb.log.Error("[VoiceBridge] SendConfig failed to marshal: %v", err)
		return fmt.Errorf("failed to marshal config message: %w", err)
	}

	vb.log.Info("[VoiceBridge] Sending config message: %s", string(data))

	// Set write deadline
	if err := conn.SetWriteDeadline(time.Now().Add(vb.config.WriteTimeout)); err != nil {
		vb.log.Warn("[VoiceBridge] Failed to set write deadline: %v", err)
	}

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		vb.log.Error("[VoiceBridge] SendConfig write failed: %v", err)
		return fmt.Errorf("failed to send config: %w", err)
	}

	vb.log.Debug("[VoiceBridge] SendConfig completed successfully")
	return nil
}

// IsConnected returns whether the bridge is currently connected.
func (vb *VoiceBridge) IsConnected() bool {
	return vb.GetState() == StateConnected
}

// URL returns the orchestrator WebSocket URL.
func (vb *VoiceBridge) URL() string {
	return vb.config.OrchestratorURL
}

// HandleClientConnection handles a WebSocket connection from a frontend client.
// This allows the frontend to directly receive voice events.
func (vb *VoiceBridge) HandleClientConnection(conn *websocket.Conn) {
	vb.clientsMu.Lock()
	vb.clients[conn] = struct{}{}
	vb.clientsMu.Unlock()

	defer func() {
		vb.clientsMu.Lock()
		delete(vb.clients, conn)
		vb.clientsMu.Unlock()
		conn.Close()
	}()

	// Read messages from client (for potential bidirectional communication)
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			vb.log.Debug("[VoiceBridge] Client disconnected: %v", err)
			return
		}

		// Forward client messages to orchestrator if needed
		vb.mu.RLock()
		orchestratorConn := vb.conn
		state := vb.state
		vb.mu.RUnlock()

		if state == StateConnected && orchestratorConn != nil {
			if err := orchestratorConn.WriteMessage(websocket.TextMessage, data); err != nil {
				vb.log.Error("[VoiceBridge] Failed to forward client message to orchestrator: %v", err)
			}
		}
	}
}

// broadcastToClients sends a message to all connected frontend clients.
func (vb *VoiceBridge) broadcastToClients(data []byte) {
	vb.clientsMu.RLock()
	defer vb.clientsMu.RUnlock()

	for client := range vb.clients {
		if err := client.WriteMessage(websocket.TextMessage, data); err != nil {
			vb.log.Error("[VoiceBridge] Failed to broadcast to client: %v", err)
		}
	}
}

// Close gracefully closes the voice bridge connection.
func (vb *VoiceBridge) Close() error {
	vb.cancel()

	vb.mu.Lock()
	if vb.conn != nil {
		// Send close message
		err := vb.conn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		)
		if err != nil {
			vb.log.Error("[VoiceBridge] Failed to send close message: %v", err)
		}

		// Close connection
		vb.conn.Close()
		vb.conn = nil
	}

	vb.state = StateDisconnected
	vb.mu.Unlock()

	// Close all client connections
	vb.clientsMu.Lock()
	for client := range vb.clients {
		client.Close()
	}
	vb.clients = make(map[*websocket.Conn]struct{})
	vb.clientsMu.Unlock()

	vb.log.Info("[VoiceBridge] Voice bridge closed")
	return nil
}

// Speak is an alias for SendText to satisfy the TUI VoiceBridge interface.
func (vb *VoiceBridge) Speak(text string) error {
	return vb.SendText(text)
}

// StopSpeaking is an alias for Cancel to satisfy the TUI VoiceBridge interface.
func (vb *VoiceBridge) StopSpeaking() error {
	return vb.Cancel()
}

// StartListening is a stub for the interface (orchestrator handles this via config/state)
func (vb *VoiceBridge) StartListening() error {
	// In the real implementation, we might send a config update or just assume it's always ready
	// For now, we'll send a config message to enable mic if supported
	return vb.SendConfig(map[string]interface{}{
		"mode": "listening",
	})
}

// StopListening is a stub for the interface
func (vb *VoiceBridge) StopListening() error {
	return vb.SendConfig(map[string]interface{}{
		"mode": "idle",
	})
}

// GetLastError returns the last error (stub)
func (vb *VoiceBridge) GetLastError() string {
	return ""
}
