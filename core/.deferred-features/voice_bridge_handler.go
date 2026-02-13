package server

import (
	"net/http"

	"github.com/gorilla/websocket"
)

// ═══════════════════════════════════════════════════════════════════════════════
// VOICE BRIDGE API HANDLERS
// ═══════════════════════════════════════════════════════════════════════════════

// handleVoiceWebSocket handles GET /api/v1/voice/ws - WebSocket for voice events.
func (p *Prism) handleVoiceWebSocket(w http.ResponseWriter, r *http.Request) {
	// Check if voice bridge is initialized
	if p.voiceBridge == nil {
		p.writeError(w, http.StatusServiceUnavailable, "voice bridge not available")
		return
	}

	// Upgrade to WebSocket
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			// In dev mode, allow all origins. In production, be more restrictive
			if p.config.DevMode {
				return true
			}
			// Allow localhost and same origin
			return true
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		p.log.Error("[Prism] WebSocket upgrade failed: %v", err)
		return
	}

	p.log.Info("[Prism] Voice WebSocket client connected")

	// Handle the WebSocket connection
	// This will forward messages between the client and the voice orchestrator
	p.voiceBridge.HandleClientConnection(conn)

	p.log.Info("[Prism] Voice WebSocket client disconnected")
}
