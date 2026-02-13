// Package server provides HTTP handlers for the Avatar State API.
//
// The Avatar State API provides real-time streaming of avatar animation state
// including phoneme/lip sync, emotion, and gaze direction. This enables frontend
// clients to render synchronized animated avatars.
//
// Endpoints:
//   - GET /api/v1/avatar/state    - Server-Sent Events stream of avatar state
//   - GET /api/v1/avatar/health   - System health check
//   - GET /api/v1/avatar/current  - Current state snapshot (non-streaming)
//
// SSE Events:
//   - event: state    - Full avatar state (phoneme, emotion, gaze, intensity)
//   - event: phoneme  - Phoneme and intensity only
//   - event: emotion  - Emotion state only
//   - event: gaze     - Gaze direction only
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/normanking/cortex/internal/avatar"
	"github.com/normanking/cortex/internal/logging"
)

// ═══════════════════════════════════════════════════════════════════════════════
// AVATAR STATE API HANDLERS
// ═══════════════════════════════════════════════════════════════════════════════

// AvatarHandler handles avatar-related HTTP requests.
type AvatarHandler struct {
	stateManager *avatar.StateManager
	log          *logging.Logger
}

// NewAvatarHandler creates a new avatar handler.
func NewAvatarHandler(stateManager *avatar.StateManager, logger *logging.Logger) *AvatarHandler {
	return &AvatarHandler{
		stateManager: stateManager,
		log:          logger,
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// SSE ENDPOINT: GET /api/v1/avatar/state
// ═══════════════════════════════════════════════════════════════════════════════

// handleAvatarStateSSE streams avatar state via Server-Sent Events.
// GET /api/v1/avatar/state
func (h *AvatarHandler) handleAvatarStateSSE(w http.ResponseWriter, r *http.Request) {
	// 1. Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// 2. Check for Flusher support
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		h.log.Error("[AvatarHandler] SSE not supported by response writer")
		return
	}

	h.log.Info("[AvatarHandler] SSE client connected from %s", r.RemoteAddr)

	// 3. Subscribe to state updates
	stateCh, unsubscribe := h.stateManager.Subscribe()
	defer func() {
		unsubscribe()
		h.log.Info("[AvatarHandler] SSE client disconnected from %s", r.RemoteAddr)
	}()

	// 4. Send initial state
	currentState := h.stateManager.GetCurrentState()
	if err := h.sendSSEEvent(w, flusher, "state", currentState); err != nil {
		h.log.Warn("[AvatarHandler] Failed to send initial state: %v", err)
		return
	}

	// 5. Loop sending updates until client disconnects
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			// Client disconnected
			return

		case state, ok := <-stateCh:
			if !ok {
				// Channel closed
				h.log.Warn("[AvatarHandler] State channel closed")
				return
			}

			// Send full state update
			if err := h.sendSSEEvent(w, flusher, "state", state); err != nil {
				h.log.Warn("[AvatarHandler] Failed to send state update: %v", err)
				return
			}

			// Send individual event types for convenience
			// Clients can subscribe to specific event types if needed
			if err := h.sendSSEEvent(w, flusher, "phoneme", map[string]interface{}{
				"phoneme":   state.Phoneme,
				"intensity": state.Intensity,
			}); err != nil {
				h.log.Warn("[AvatarHandler] Failed to send phoneme event: %v", err)
				return
			}

			if err := h.sendSSEEvent(w, flusher, "emotion", state.Emotion); err != nil {
				h.log.Warn("[AvatarHandler] Failed to send emotion event: %v", err)
				return
			}

			if err := h.sendSSEEvent(w, flusher, "gaze", state.Gaze); err != nil {
				h.log.Warn("[AvatarHandler] Failed to send gaze event: %v", err)
				return
			}
		}
	}
}

// sendSSEEvent sends a single SSE event.
func (h *AvatarHandler) sendSSEEvent(w http.ResponseWriter, flusher http.Flusher, eventType string, data interface{}) error {
	// Marshal data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal event data: %w", err)
	}

	// Write event type and data
	fmt.Fprintf(w, "event: %s\n", eventType)
	fmt.Fprintf(w, "data: %s\n\n", jsonData)

	// Flush to client
	flusher.Flush()

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// HEALTH ENDPOINT: GET /api/v1/avatar/health
// ═══════════════════════════════════════════════════════════════════════════════

// AvatarHealthResponse represents the health status of the avatar system.
type AvatarHealthResponse struct {
	Status            string        `json:"status"`
	StateManager      bool          `json:"state_manager"`
	PhonemeExtractor  string        `json:"phoneme_extractor"`
	ConnectedClients  int           `json:"connected_clients"`
	CurrentState      *avatar.AvatarState `json:"current_state,omitempty"`
}

// handleAvatarHealth returns avatar system health.
// GET /api/v1/avatar/health
func (h *AvatarHandler) handleAvatarHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	_ = ctx // Use context if needed for future async checks

	// Check state manager availability
	stateManagerOK := h.stateManager != nil
	status := "healthy"
	if !stateManagerOK {
		status = "degraded"
	}

	// Get current state
	var currentState *avatar.AvatarState
	if stateManagerOK {
		currentState = h.stateManager.GetCurrentState()
	}

	// Get connected clients count
	connectedClients := 0
	if stateManagerOK {
		connectedClients = h.stateManager.GetClientCount()
	}

	response := AvatarHealthResponse{
		Status:           status,
		StateManager:     stateManagerOK,
		PhonemeExtractor: "text_based", // Could be dynamic based on config
		ConnectedClients: connectedClients,
		CurrentState:     currentState,
	}

	h.writeJSON(w, http.StatusOK, response)
}

// ═══════════════════════════════════════════════════════════════════════════════
// CURRENT STATE ENDPOINT: GET /api/v1/avatar/current
// ═══════════════════════════════════════════════════════════════════════════════

// handleAvatarCurrent returns current avatar state (non-streaming).
// GET /api/v1/avatar/current
func (h *AvatarHandler) handleAvatarCurrent(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	_ = ctx // Use context if needed for future async operations

	if h.stateManager == nil {
		h.writeError(w, http.StatusServiceUnavailable, "state manager not available")
		return
	}

	currentState := h.stateManager.GetCurrentState()
	h.writeJSON(w, http.StatusOK, currentState)
}

// ═══════════════════════════════════════════════════════════════════════════════
// ROUTE REGISTRATION
// ═══════════════════════════════════════════════════════════════════════════════

// RegisterRoutes registers avatar routes with the mux.
func (h *AvatarHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/avatar/state", h.handleAvatarStateSSE)
	mux.HandleFunc("GET /api/v1/avatar/health", h.handleAvatarHealth)
	mux.HandleFunc("GET /api/v1/avatar/current", h.handleAvatarCurrent)

	h.log.Info("[AvatarHandler] Registered avatar routes")
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPER METHODS
// ═══════════════════════════════════════════════════════════════════════════════

// writeJSON writes a JSON response.
func (h *AvatarHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.Error("[AvatarHandler] Failed to encode JSON response: %v", err)
	}
}

// writeError writes an error response.
func (h *AvatarHandler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, APIError{
		Code:    status,
		Message: message,
	})
}
