package voice

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/normanking/cortex/internal/logging"
)

// WebSocketHandler provides WebSocket endpoints for browser-based voice interaction.
// This allows Prism UI clients to connect directly to voice services.
type WebSocketHandler struct {
	bridge   *VoiceBridge
	upgrader websocket.Upgrader
	log      *logging.Logger
}

// NewWebSocketHandler creates a new WebSocket handler for voice.
func NewWebSocketHandler(bridge *VoiceBridge) *WebSocketHandler {
	return &WebSocketHandler{
		bridge: bridge,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// TODO: Implement proper origin checking in production
				return true
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		log: logging.Global(),
	}
}

// HandleVoiceWebSocket handles WebSocket connections at /api/v1/voice/ws.
// Browser clients connect here to receive real-time voice events (transcripts, interrupts, status).
func (h *WebSocketHandler) HandleVoiceWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP connection to WebSocket
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Error("[VoiceWS] Failed to upgrade websocket connection: %v", err)
		return
	}

	h.log.Info("[VoiceWS] New voice websocket client connected: %s", r.RemoteAddr)

	// Delegate to bridge's client handler
	// This allows the bridge to broadcast orchestrator messages to all connected clients
	h.bridge.HandleClientConnection(conn)
}

// VoiceClientMessage represents a message from browser clients.
// This allows bidirectional communication (e.g., sending config, triggering actions).
type VoiceClientMessage struct {
	Type      string                 `json:"type"` // "config", "synthesize", "cancel", "interrupt"
	Text      string                 `json:"text,omitempty"`
	VoiceID   string                 `json:"voice_id,omitempty"`
	Config    map[string]interface{} `json:"config,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// HandleClientMessage processes messages from browser clients and forwards to bridge.
func (h *WebSocketHandler) HandleClientMessage(conn *websocket.Conn, data []byte) error {
	var msg VoiceClientMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		h.log.Error("[VoiceWS] Failed to parse client message: %v", err)
		return err
	}

	switch msg.Type {
	case "synthesize":
		if err := h.bridge.SendText(msg.Text); err != nil {
			h.log.Error("[VoiceWS] Failed to send text to bridge: %v", err)
			return err
		}

	case "cancel":
		if err := h.bridge.Cancel(); err != nil {
			h.log.Error("[VoiceWS] Failed to cancel via bridge: %v", err)
			return err
		}

	case "config":
		if err := h.bridge.SendConfig(msg.Config); err != nil {
			h.log.Error("[VoiceWS] Failed to send config to bridge: %v", err)
			return err
		}

	case "interrupt":
		// Manual interrupt from UI
		interrupt := NewInterruptSignal(InterruptTypeManual, "user requested cancellation", h.bridge.config.SessionID)
		h.bridge.SendInterrupt(interrupt)

	default:
		h.log.Debug("[VoiceWS] Unknown client message type: %s", msg.Type)
	}

	return nil
}

// RegisterRoutes registers WebSocket routes to the given HTTP mux.
func (h *WebSocketHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/voice/ws", h.HandleVoiceWebSocket)
}

// VoiceServerMessage represents a message sent to browser clients.
// This matches the structure of messages from the Python orchestrator.
type VoiceServerMessage struct {
	Type       string                 `json:"type"` // "transcript", "interrupt", "status", "synthesizing", "complete", "error"
	Text       string                 `json:"text,omitempty"`
	IsFinal    bool                   `json:"is_final,omitempty"`
	Confidence float64                `json:"confidence,omitempty"`
	Reason     string                 `json:"reason,omitempty"`
	State      string                 `json:"state,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// Example usage documentation
const WebSocketUsageExample = `
# Connect to voice WebSocket from browser
const ws = new WebSocket('ws://localhost:7890/api/v1/voice/ws');

ws.onopen = () => {
  console.log('Connected to voice bridge');

  // Send text for synthesis
  ws.send(JSON.stringify({
    type: 'synthesize',
    text: 'Hello, world!',
    voice_id: 'af_sky',
    timestamp: new Date().toISOString()
  }));
};

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);

  switch (msg.type) {
    case 'transcript':
      console.log('Transcript:', msg.text, 'Final:', msg.is_final);
      break;

    case 'interrupt':
      console.log('Interrupted:', msg.reason);
      break;

    case 'status':
      console.log('Status:', msg.state);
      break;

    case 'synthesizing':
      console.log('Synthesizing:', msg.text);
      break;

    case 'complete':
      console.log('Synthesis complete');
      break;

    case 'error':
      console.error('Error:', msg.reason);
      break;
  }
};

// Cancel current synthesis
ws.send(JSON.stringify({
  type: 'cancel',
  timestamp: new Date().toISOString()
}));

// Manual interrupt
ws.send(JSON.stringify({
  type: 'interrupt',
  timestamp: new Date().toISOString()
}));
`
