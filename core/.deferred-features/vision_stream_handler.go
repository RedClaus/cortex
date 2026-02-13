package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/normanking/cortex/internal/logging"
	"github.com/normanking/cortex/internal/vision"
)

// ═══════════════════════════════════════════════════════════════════════════════
// VISION STREAM HANDLER
// ═══════════════════════════════════════════════════════════════════════════════

// VisionStreamHandler handles video frame streaming endpoints.
// It provides HTTP POST for single frame ingestion, WebSocket for bidirectional
// streaming, and SSE for analysis result streaming.
type VisionStreamHandler struct {
	streamHandler *vision.StreamHandler
	log           *logging.Logger

	// SSE clients for analysis results
	sseClients map[chan *vision.AnalysisResult]struct{}
	sseMu      sync.RWMutex

	// CR-023: CortexEyes frame callback
	frameCallback func(frame *vision.Frame, appName, windowTitle string)
	frameCallbackMu sync.RWMutex

	// Background goroutine for broadcasting analysis results
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewVisionStreamHandler creates a new vision stream handler.
func NewVisionStreamHandler(streamHandler *vision.StreamHandler, log *logging.Logger) *VisionStreamHandler {
	ctx, cancel := context.WithCancel(context.Background())

	handler := &VisionStreamHandler{
		streamHandler: streamHandler,
		log:           log,
		sseClients:    make(map[chan *vision.AnalysisResult]struct{}),
		ctx:           ctx,
		cancel:        cancel,
	}

	// Start background goroutine to broadcast analysis results to SSE clients
	handler.wg.Add(1)
	go handler.broadcastAnalysisResults()

	return handler
}

// Close gracefully shuts down the handler.
func (h *VisionStreamHandler) Close() {
	h.cancel()
	h.wg.Wait()
}

// SetFrameCallback sets a callback function that receives every ingested frame.
// CR-023: Used by CortexEyes to receive frames for screen awareness.
func (h *VisionStreamHandler) SetFrameCallback(cb func(frame *vision.Frame, appName, windowTitle string)) {
	h.frameCallbackMu.Lock()
	defer h.frameCallbackMu.Unlock()
	h.frameCallback = cb
	h.log.Info("Frame callback registered for CortexEyes")
}

// invokeFrameCallback safely invokes the frame callback if set.
func (h *VisionStreamHandler) invokeFrameCallback(frame *vision.Frame, appName, windowTitle string) {
	h.frameCallbackMu.RLock()
	cb := h.frameCallback
	h.frameCallbackMu.RUnlock()

	if cb != nil {
		go cb(frame, appName, windowTitle)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// REQUEST/RESPONSE TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// VisionStreamFrameRequest is the request body for POST /api/v1/vision/stream
type VisionStreamFrameRequest struct {
	Frame     string `json:"frame"`               // Base64-encoded image data
	MimeType  string `json:"mime_type"`           // Image format (image/jpeg, image/png)
	Sequence  int64  `json:"sequence,omitempty"`  // Optional client-side sequence number
	Timestamp string `json:"timestamp,omitempty"` // Optional ISO8601 timestamp
}

// VisionStreamFrameResponse is the response for POST /api/v1/vision/stream
type VisionStreamFrameResponse struct {
	Accepted   bool  `json:"accepted"`    // True if frame was accepted
	Sequence   int64 `json:"sequence"`    // Server-assigned sequence number
	QueueDepth int   `json:"queue_depth"` // Current buffer utilization
}

// VisionStreamControlRequest is the request body for POST /api/v1/vision/stream/control
type VisionStreamControlRequest struct {
	Action string                 `json:"action"` // "start", "stop", "configure"
	Config *vision.StreamConfig   `json:"config,omitempty"` // Configuration for "configure" action
}

// VisionStreamControlResponse is the response for POST /api/v1/vision/stream/control
type VisionStreamControlResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Status  string `json:"status"` // "running", "stopped"
}

// VisionStreamStatsResponse is the response for GET /api/v1/vision/stream/stats
type VisionStreamStatsResponse struct {
	IsRunning         bool    `json:"is_running"`
	FramesReceived    int64   `json:"frames_received"`
	FramesDropped     int64   `json:"frames_dropped"`
	FramesAnalyzed    int64   `json:"frames_analyzed"`
	CurrentFPS        float64 `json:"current_fps"`
	BufferUtilization float64 `json:"buffer_utilization"` // 0.0-1.0
	AvgAnalysisMs     float64 `json:"avg_analysis_ms"`
}

// WebSocketFrameMessage is sent by client to ingest a frame via WebSocket
type WebSocketFrameMessage struct {
	Type        string `json:"type"`                   // "frame"
	Data        string `json:"data"`                   // Base64-encoded image
	MimeType    string `json:"mime_type"`              // Image format
	Sequence    int64  `json:"sequence,omitempty"`
	Timestamp   string `json:"timestamp,omitempty"`
	AppName     string `json:"app_name,omitempty"`     // CR-023: Active application name
	WindowTitle string `json:"window_title,omitempty"` // CR-023: Window title
}

// WebSocketAnalysisMessage is sent by server with analysis result via WebSocket
type WebSocketAnalysisMessage struct {
	Type          string `json:"type"`           // "analysis"
	FrameSequence int64  `json:"frame_sequence"` // Frame that was analyzed
	Content       string `json:"content"`        // Analysis text
	Provider      string `json:"provider"`       // Model used
	LatencyMs     int64  `json:"latency_ms"`     // Analysis duration
	Timestamp     string `json:"timestamp"`      // ISO8601 timestamp
}

// WebSocketAckMessage acknowledges frame receipt via WebSocket
type WebSocketAckMessage struct {
	Type     string `json:"type"`     // "ack"
	Sequence int64  `json:"sequence"` // Frame sequence acknowledged
}

// WebSocketErrorMessage reports errors via WebSocket
type WebSocketErrorMessage struct {
	Type    string `json:"type"`    // "error"
	Message string `json:"message"` // Error description
}

// ═══════════════════════════════════════════════════════════════════════════════
// HTTP HANDLERS
// ═══════════════════════════════════════════════════════════════════════════════

// handleVisionStreamFrame accepts a single video frame via HTTP POST.
// POST /api/v1/vision/stream
func (h *VisionStreamHandler) handleVisionStreamFrame(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Parse request
	var req VisionStreamFrameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if req.Frame == "" {
		h.writeError(w, http.StatusBadRequest, "frame is required")
		return
	}
	if req.MimeType == "" {
		h.writeError(w, http.StatusBadRequest, "mime_type is required")
		return
	}

	// Decode base64 frame
	frameData, err := base64.StdEncoding.DecodeString(req.Frame)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid base64 frame data")
		return
	}

	// Parse optional timestamp
	var timestamp time.Time
	if req.Timestamp != "" {
		timestamp, err = time.Parse(time.RFC3339, req.Timestamp)
		if err != nil {
			timestamp = time.Now()
		}
	} else {
		timestamp = time.Now()
	}

	// Create frame
	frame := &vision.Frame{
		Data:      frameData,
		MimeType:  req.MimeType,
		Timestamp: timestamp,
	}

	// Ingest frame
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	err = h.streamHandler.IngestFrame(ctx, frame)
	if err != nil {
		h.log.Warn("Failed to ingest frame: %v", err)
		h.writeError(w, http.StatusTooManyRequests, err.Error())
		return
	}

	// CR-023: Invoke CortexEyes frame callback
	h.invokeFrameCallback(frame, "", "") // HTTP POST doesn't include app context

	// Build response
	response := VisionStreamFrameResponse{
		Accepted:   true,
		Sequence:   frame.Sequence,
		QueueDepth: h.streamHandler.GetBuffer().Len(),
	}

	h.log.Debug("Frame ingested: sequence=%d, mime_type=%s, queue_depth=%d",
		response.Sequence, req.MimeType, response.QueueDepth)

	h.writeJSON(w, http.StatusOK, response)
}

// handleVisionStreamWebSocket handles bidirectional WebSocket frame streaming.
// GET /api/v1/vision/stream/ws
func (h *VisionStreamHandler) handleVisionStreamWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade to WebSocket
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			// In production, validate origin properly
			return true
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Error("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	h.log.Info("WebSocket client connected: %s", r.RemoteAddr)

	// Create channels for coordination
	done := make(chan struct{})
	analysisCh := make(chan *vision.AnalysisResult, 10)

	// Register for analysis results
	h.sseMu.Lock()
	h.sseClients[analysisCh] = struct{}{}
	h.sseMu.Unlock()

	// Cleanup on disconnect
	defer func() {
		h.sseMu.Lock()
		delete(h.sseClients, analysisCh)
		h.sseMu.Unlock()
		close(done)
		h.log.Info("WebSocket client disconnected: %s", r.RemoteAddr)
	}()

	// Start goroutine to send analysis results to client
	go func() {
		for {
			select {
			case <-done:
				return
			case result := <-analysisCh:
				if result.Error != nil {
					// Send error message
					errMsg := WebSocketErrorMessage{
						Type:    "error",
						Message: result.Error.Error(),
					}
					if err := conn.WriteJSON(errMsg); err != nil {
						h.log.Error("Failed to send WebSocket error: %v", err)
						return
					}
				} else {
					// Send analysis result
					analysisMsg := WebSocketAnalysisMessage{
						Type:          "analysis",
						FrameSequence: result.FrameSequence,
						Content:       result.Analysis.Content,
						Provider:      result.Analysis.Provider,
						LatencyMs:     result.LatencyMs,
						Timestamp:     result.Timestamp.Format(time.RFC3339),
					}
					if err := conn.WriteJSON(analysisMsg); err != nil {
						h.log.Error("Failed to send WebSocket analysis: %v", err)
						return
					}
				}
			}
		}
	}()

	// Read frames from client
	for {
		var msg WebSocketFrameMessage
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				h.log.Error("WebSocket read error: %v", err)
			}
			return
		}

		if msg.Type != "frame" {
			h.log.Warn("Unknown WebSocket message type: %s", msg.Type)
			continue
		}

		// Decode frame
		frameData, err := base64.StdEncoding.DecodeString(msg.Data)
		if err != nil {
			errMsg := WebSocketErrorMessage{
				Type:    "error",
				Message: "invalid base64 frame data",
			}
			conn.WriteJSON(errMsg)
			continue
		}

		// Parse timestamp
		var timestamp time.Time
		if msg.Timestamp != "" {
			timestamp, err = time.Parse(time.RFC3339, msg.Timestamp)
			if err != nil {
				timestamp = time.Now()
			}
		} else {
			timestamp = time.Now()
		}

		// Create frame
		frame := &vision.Frame{
			Data:      frameData,
			MimeType:  msg.MimeType,
			Timestamp: timestamp,
		}

		// Ingest frame
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = h.streamHandler.IngestFrame(ctx, frame)
		cancel()

		if err != nil {
			errMsg := WebSocketErrorMessage{
				Type:    "error",
				Message: err.Error(),
			}
			conn.WriteJSON(errMsg)
			continue
		}

		// CR-023: Invoke CortexEyes frame callback with app context
		h.invokeFrameCallback(frame, msg.AppName, msg.WindowTitle)

		// Send acknowledgment
		ackMsg := WebSocketAckMessage{
			Type:     "ack",
			Sequence: frame.Sequence,
		}
		if err := conn.WriteJSON(ackMsg); err != nil {
			h.log.Error("Failed to send WebSocket ack: %v", err)
			return
		}
	}
}

// handleVisionStreamResults streams analysis results via Server-Sent Events (SSE).
// GET /api/v1/vision/stream/results
func (h *VisionStreamHandler) handleVisionStreamResults(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create channel for this client
	clientCh := make(chan *vision.AnalysisResult, 10)

	// Register client
	h.sseMu.Lock()
	h.sseClients[clientCh] = struct{}{}
	h.sseMu.Unlock()

	// Cleanup on disconnect
	defer func() {
		h.sseMu.Lock()
		delete(h.sseClients, clientCh)
		h.sseMu.Unlock()
		close(clientCh)
	}()

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.log.Error("Streaming not supported")
		return
	}

	h.log.Info("SSE client connected for analysis results: %s", r.RemoteAddr)

	// Send initial connection message
	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"connected\"}\n\n")
	flusher.Flush()

	// Stream results to client
	for {
		select {
		case <-r.Context().Done():
			h.log.Info("SSE client disconnected: %s", r.RemoteAddr)
			return

		case result := <-clientCh:
			if result.Error != nil {
				// Send error event
				errData := map[string]interface{}{
					"frame_sequence": result.FrameSequence,
					"error":          result.Error.Error(),
					"timestamp":      result.Timestamp.Format(time.RFC3339),
				}
				errJSON, _ := json.Marshal(errData)
				fmt.Fprintf(w, "event: error\ndata: %s\n\n", errJSON)
			} else {
				// Send analysis event
				resultData := map[string]interface{}{
					"frame_sequence": result.FrameSequence,
					"content":        result.Analysis.Content,
					"provider":       result.Analysis.Provider,
					"latency_ms":     result.LatencyMs,
					"timestamp":      result.Timestamp.Format(time.RFC3339),
				}
				resultJSON, _ := json.Marshal(resultData)
				fmt.Fprintf(w, "event: analysis\ndata: %s\n\n", resultJSON)
			}
			flusher.Flush()
		}
	}
}

// handleVisionStreamStats returns current streaming statistics.
// GET /api/v1/vision/stream/stats
func (h *VisionStreamHandler) handleVisionStreamStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	stats := h.streamHandler.GetStats()

	response := VisionStreamStatsResponse{
		IsRunning:         stats.IsRunning,
		FramesReceived:    stats.FramesReceived,
		FramesDropped:     stats.FramesDropped,
		FramesAnalyzed:    stats.FramesAnalyzed,
		CurrentFPS:        stats.CurrentFPS,
		BufferUtilization: stats.BufferUtilization,
		AvgAnalysisMs:     stats.AvgAnalysisMs,
	}

	h.writeJSON(w, http.StatusOK, response)
}

// handleVisionStreamControl starts/stops/configures the stream handler.
// POST /api/v1/vision/stream/control
func (h *VisionStreamHandler) handleVisionStreamControl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Parse request
	var req VisionStreamControlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var response VisionStreamControlResponse

	switch req.Action {
	case "start":
		if h.streamHandler.IsRunning() {
			response.Success = false
			response.Message = "stream handler already running"
			response.Status = "running"
		} else {
			ctx := context.Background()
			if err := h.streamHandler.Start(ctx); err != nil {
				response.Success = false
				response.Message = err.Error()
				response.Status = "stopped"
				h.writeJSON(w, http.StatusInternalServerError, response)
				return
			}
			response.Success = true
			response.Message = "stream handler started"
			response.Status = "running"
			h.log.Info("Stream handler started via control endpoint")
		}

	case "stop":
		if !h.streamHandler.IsRunning() {
			response.Success = false
			response.Message = "stream handler not running"
			response.Status = "stopped"
		} else {
			if err := h.streamHandler.Stop(); err != nil {
				response.Success = false
				response.Message = err.Error()
				response.Status = "running"
				h.writeJSON(w, http.StatusInternalServerError, response)
				return
			}
			response.Success = true
			response.Message = "stream handler stopped"
			response.Status = "stopped"
			h.log.Info("Stream handler stopped via control endpoint")
		}

	case "configure":
		if req.Config == nil {
			h.writeError(w, http.StatusBadRequest, "config is required for 'configure' action")
			return
		}
		h.streamHandler.UpdateConfig(*req.Config)
		response.Success = true
		response.Message = "stream configuration updated"
		if h.streamHandler.IsRunning() {
			response.Status = "running"
		} else {
			response.Status = "stopped"
		}
		h.log.Info("Stream handler configuration updated via control endpoint")

	default:
		h.writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown action: %s", req.Action))
		return
	}

	h.writeJSON(w, http.StatusOK, response)
}

// ═══════════════════════════════════════════════════════════════════════════════
// ROUTE REGISTRATION
// ═══════════════════════════════════════════════════════════════════════════════

// RegisterRoutes registers all vision streaming routes with the HTTP mux.
func (h *VisionStreamHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/vision/stream", h.handleVisionStreamFrame)
	mux.HandleFunc("GET /api/v1/vision/stream/ws", h.handleVisionStreamWebSocket)
	mux.HandleFunc("GET /api/v1/vision/stream/results", h.handleVisionStreamResults)
	mux.HandleFunc("GET /api/v1/vision/stream/stats", h.handleVisionStreamStats)
	mux.HandleFunc("POST /api/v1/vision/stream/control", h.handleVisionStreamControl)

	h.log.Info("Vision streaming routes registered")
}

// ═══════════════════════════════════════════════════════════════════════════════
// BACKGROUND TASKS
// ═══════════════════════════════════════════════════════════════════════════════

// broadcastAnalysisResults runs in background, forwarding analysis results
// from the stream handler to all connected SSE/WebSocket clients.
func (h *VisionStreamHandler) broadcastAnalysisResults() {
	defer h.wg.Done()

	analysisCh := h.streamHandler.GetAnalysisChannel()

	for {
		select {
		case <-h.ctx.Done():
			return

		case result, ok := <-analysisCh:
			if !ok {
				// Analysis channel closed
				return
			}

			// Broadcast to all connected clients
			h.sseMu.RLock()
			for clientCh := range h.sseClients {
				select {
				case clientCh <- result:
				default:
					// Client channel full, skip
					h.log.Warn("Client channel full, dropping analysis result")
				}
			}
			h.sseMu.RUnlock()
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPER METHODS
// ═══════════════════════════════════════════════════════════════════════════════

// writeJSON writes a JSON response with the given status code.
func (h *VisionStreamHandler) writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// writeError writes a JSON error response.
func (h *VisionStreamHandler) writeError(w http.ResponseWriter, statusCode int, message string) {
	h.writeJSON(w, statusCode, map[string]string{
		"error": message,
	})
}
