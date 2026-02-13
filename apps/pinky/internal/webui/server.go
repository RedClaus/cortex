// Package webui provides the HTTP server and WebSocket handler for the Pinky Web UI
package webui

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/normanking/pinky/internal/brain"
	"github.com/normanking/pinky/internal/config"
	"github.com/normanking/pinky/internal/logging"
)

//go:embed dist/*
var staticFiles embed.FS

// LaneSwitcher interface for brain lane management
type LaneSwitcher interface {
	SetLane(name string) error
	GetLane() string
	GetLanes() []brain.LaneInfo
	SetAutoLLM(enabled bool)
	GetAutoLLM() bool
}

// Server handles the WebUI HTTP server and WebSocket connections
type Server struct {
	config       *config.Config
	logger       *logging.Logger
	httpServer   *http.Server
	clients      map[*Client]bool
	mu           sync.RWMutex
	broadcast    chan []byte
	laneSwitcher LaneSwitcher
	brain        brain.Brain
	tools        []brain.ToolSpec
}

// Client represents a connected WebSocket client
type Client struct {
	server *Server
	send   chan []byte
}

// Message represents a chat message
type Message struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"` // "user" or "assistant"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// ChatRequest represents an incoming chat request
type ChatRequest struct {
	Content string `json:"content"`
}

// ChatResponse represents the response to a chat request
type ChatResponse struct {
	Message  Message       `json:"message"`
	Thinking []ThinkStep   `json:"thinking,omitempty"`
	Tools    []ToolCall    `json:"tools,omitempty"`
}

// ThinkStep represents a reasoning step
type ThinkStep struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Status      string `json:"status"` // "pending", "active", "completed", "failed"
}

// ToolCall represents a tool execution
type ToolCall struct {
	ID     string                 `json:"id"`
	Tool   string                 `json:"tool"`
	Input  map[string]interface{} `json:"input"`
	Status string                 `json:"status"` // "pending", "approved", "denied", "running", "completed", "failed"
	Output string                 `json:"output,omitempty"`
	Error  string                 `json:"error,omitempty"`
}

// New creates a new WebUI server
func New(cfg *config.Config, logger *logging.Logger) *Server {
	return &Server{
		config:    cfg,
		logger:    logger,
		clients:   make(map[*Client]bool),
		broadcast: make(chan []byte, 256),
	}
}

// SetLaneSwitcher sets the brain's lane switching interface
func (s *Server) SetLaneSwitcher(ls LaneSwitcher) {
	s.laneSwitcher = ls
}

// SetBrain sets the brain interface for processing requests
func (s *Server) SetBrain(b brain.Brain) {
	s.brain = b
}

// SetTools sets the available tools for the brain to use
func (s *Server) SetTools(tools []brain.ToolSpec) {
	s.tools = tools
}

// Start starts the WebUI HTTP server
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/v1/health", s.handleHealth)
	mux.HandleFunc("/api/v1/chat", s.handleChat)
	mux.HandleFunc("/api/v1/config", s.handleConfig)
	mux.HandleFunc("/api/v1/personas", s.handlePersonas)
	mux.HandleFunc("/api/v1/channels", s.handleChannels)
	mux.HandleFunc("/api/v1/lanes", s.handleLanes)
	mux.HandleFunc("/api/v1/lane", s.handleLane)
	mux.HandleFunc("/api/v1/autollm", s.handleAutoLLM)
	mux.HandleFunc("/api/v1/apikeys", s.handleAPIKeys)

	// WebSocket
	mux.HandleFunc("/ws", s.handleWebSocket)

	// Static files (SPA)
	mux.HandleFunc("/", s.handleStatic)

	addr := s.config.Server.Host + ":" + itoa(s.config.Server.WebUIPort)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      corsMiddleware(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second, // LLM responses with web search can take longer
		IdleTimeout:  120 * time.Second,
	}

	s.logger.Info("Starting WebUI server", "addr", addr)

	// Start broadcast goroutine
	go s.runBroadcast()

	// Run server
	errChan := make(chan error, 1)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(shutdownCtx)
	}
}

// runBroadcast sends messages to all connected clients
func (s *Server) runBroadcast() {
	for msg := range s.broadcast {
		s.mu.RLock()
		for client := range s.clients {
			select {
			case client.send <- msg:
			default:
				close(client.send)
				delete(s.clients, client)
			}
		}
		s.mu.RUnlock()
	}
}

// handleHealth returns server health status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"version": "1.0.0",
		"brain":   s.config.Brain.Mode,
	})
}

// handleChat processes chat messages
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// SECURITY: Limit request body size to prevent DoS
	const maxBodySize = 1 * 1024 * 1024 // 1MB
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if err.Error() == "http: request body too large" {
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Call the Brain to process the request
	var responseContent string
	if s.brain != nil {
		// DEBUG: Log tools being passed
		s.logger.Info("WebUI calling brain.Think", "num_tools", len(s.tools), "user_content", req.Content)

		thinkReq := &brain.ThinkRequest{
			// Use the primary user ID for memory retrieval
			// TODO: Support per-session user identification
			UserID: "ed1ac3a8-d8a9-415f-be4f-20a1b5666e20",
			Messages: []brain.Message{
				{Role: "user", Content: req.Content},
			},
			Tools: s.tools, // Include tools so the brain knows what's available
		}
		thinkResp, err := s.brain.Think(r.Context(), thinkReq)
		if err != nil {
			s.logger.Error("Brain.Think error: %v", err)
			responseContent = "I apologize, but I encountered an error processing your request: " + err.Error()
		} else {
			responseContent = thinkResp.Content
		}
	} else {
		// Fallback to demo response if brain not configured
		responseContent = generateResponse(req.Content)
	}

	response := ChatResponse{
		Message: Message{
			ID:        generateID(),
			Role:      "assistant",
			Content:   responseContent,
			Timestamp: time.Now(),
		},
		Thinking: []ThinkStep{
			{ID: "1", Description: "Analyzing request...", Status: "completed"},
			{ID: "2", Description: "Processing with Brain", Status: "completed"},
			{ID: "3", Description: "Generating output", Status: "completed"},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleConfig returns/updates configuration
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"brain": map[string]interface{}{
				"mode":      s.config.Brain.Mode,
				"remoteUrl": s.config.Brain.RemoteURL,
			},
			"server": map[string]interface{}{
				"host":      s.config.Server.Host,
				"port":      s.config.Server.Port,
				"webuiPort": s.config.Server.WebUIPort,
			},
			"permissions": map[string]interface{}{
				"defaultTier": s.config.Permissions.DefaultTier,
			},
			"persona": map[string]interface{}{
				"default": s.config.Persona.Default,
			},
		})
	case http.MethodPut:
		// TODO: Update configuration
		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePersonas returns available personas
func (s *Server) handlePersonas(w http.ResponseWriter, r *http.Request) {
	personas := []map[string]string{
		{"id": "professional", "name": "Professional", "description": "Clear, concise, formal"},
		{"id": "casual", "name": "Casual", "description": "Friendly, conversational"},
		{"id": "mentor", "name": "Mentor", "description": "Patient, educational"},
		{"id": "minimalist", "name": "Minimalist", "description": "Terse, just the facts"},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(personas)
}

// handleChannels returns channel status
func (s *Server) handleChannels(w http.ResponseWriter, r *http.Request) {
	channels := []map[string]interface{}{
		{"name": "telegram", "enabled": s.config.Channels.Telegram.Enabled, "connected": false},
		{"name": "discord", "enabled": s.config.Channels.Discord.Enabled, "connected": false},
		{"name": "slack", "enabled": s.config.Channels.Slack.Enabled, "connected": false},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(channels)
}

// handleLanes returns all available inference lanes
func (s *Server) handleLanes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.laneSwitcher == nil {
		http.Error(w, "Lane switching not available", http.StatusServiceUnavailable)
		return
	}

	lanes := s.laneSwitcher.GetLanes()
	autoLLM := s.laneSwitcher.GetAutoLLM()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"lanes":   lanes,
		"autoLLM": autoLLM,
		"current": s.laneSwitcher.GetLane(),
	})
}

// handleLane switches the active inference lane
func (s *Server) handleLane(w http.ResponseWriter, r *http.Request) {
	if s.laneSwitcher == nil {
		http.Error(w, "Lane switching not available", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"lane": s.laneSwitcher.GetLane(),
		})

	case http.MethodPost, http.MethodPut:
		var req struct {
			Lane string `json:"lane"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if err := s.laneSwitcher.SetLane(req.Lane); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		s.logger.Info("Lane switched via WebUI", "lane", req.Lane)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"lane":   req.Lane,
			"status": "ok",
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAutoLLM enables/disables AutoLLM routing
func (s *Server) handleAutoLLM(w http.ResponseWriter, r *http.Request) {
	if s.laneSwitcher == nil {
		http.Error(w, "Lane switching not available", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{
			"enabled": s.laneSwitcher.GetAutoLLM(),
		})

	case http.MethodPost, http.MethodPut:
		var req struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		s.laneSwitcher.SetAutoLLM(req.Enabled)
		s.logger.Info("AutoLLM toggled via WebUI", "enabled", req.Enabled)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled": req.Enabled,
			"status":  "ok",
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// APIKeyInfo represents an API key with masked value
type APIKeyInfo struct {
	Lane      string `json:"lane"`
	Engine    string `json:"engine"`
	Model     string `json:"model"`
	KeySet    bool   `json:"keySet"`
	KeyMasked string `json:"keyMasked,omitempty"` // Shows last 4 chars only
}

// expandAPIKey expands environment variables in API key config
func expandAPIKey(key string) string {
	if key == "" {
		return ""
	}
	// Expand environment variables like ${OPENAI_API_KEY}
	if len(key) > 3 && key[0] == '$' && key[1] == '{' && key[len(key)-1] == '}' {
		envVar := key[2 : len(key)-1]
		return os.Getenv(envVar)
	}
	return key
}

// handleAPIKeys manages API keys for inference lanes
func (s *Server) handleAPIKeys(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Return list of lanes with masked API key status
		var keys []APIKeyInfo
		for name, lane := range s.config.Inference.Lanes {
			// Expand environment variables to check if key is actually set
			actualKey := expandAPIKey(lane.APIKey)
			info := APIKeyInfo{
				Lane:   name,
				Engine: lane.Engine,
				Model:  lane.Model,
				KeySet: actualKey != "",
			}
			if actualKey != "" && len(actualKey) > 4 {
				// Mask the key, show last 4 chars
				info.KeyMasked = "****" + actualKey[len(actualKey)-4:]
			}
			keys = append(keys, info)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(keys)

	case http.MethodPost, http.MethodPut:
		var req struct {
			Lane   string `json:"lane"`
			APIKey string `json:"apiKey"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate lane exists
		lane, ok := s.config.Inference.Lanes[req.Lane]
		if !ok {
			http.Error(w, "Lane not found", http.StatusNotFound)
			return
		}

		// Update the API key
		lane.APIKey = req.APIKey
		s.config.Inference.Lanes[req.Lane] = lane

		// Save config to file
		if err := s.config.Save(""); err != nil {
			s.logger.Error("Failed to save config", "error", err)
			http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
			return
		}

		s.logger.Info("API key updated via WebUI", "lane", req.Lane)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"lane":   req.Lane,
			"status": "ok",
		})

	case http.MethodDelete:
		var req struct {
			Lane string `json:"lane"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		lane, ok := s.config.Inference.Lanes[req.Lane]
		if !ok {
			http.Error(w, "Lane not found", http.StatusNotFound)
			return
		}

		// Clear the API key
		lane.APIKey = ""
		s.config.Inference.Lanes[req.Lane] = lane

		if err := s.config.Save(""); err != nil {
			s.logger.Error("Failed to save config", "error", err)
			http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
			return
		}

		s.logger.Info("API key cleared via WebUI", "lane", req.Lane)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"lane":   req.Lane,
			"status": "cleared",
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleWebSocket upgrades the connection to WebSocket
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Note: In production, use gorilla/websocket for proper WebSocket handling
	// This is a placeholder that returns an error for now
	http.Error(w, "WebSocket not implemented", http.StatusNotImplemented)
}

// handleStatic serves static files from the embedded filesystem
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	// Try to serve from embedded dist/ directory
	subFS, err := fs.Sub(staticFiles, "dist")
	if err != nil {
		// In development, proxy to Vite dev server
		s.logger.Debug("Static files not found, ensure Vite dev server is running")
		http.Error(w, "Static files not available", http.StatusNotFound)
		return
	}

	// Serve static files, fallback to index.html for SPA routing
	fileServer := http.FileServer(http.FS(subFS))

	// Check if the requested file exists
	path := r.URL.Path
	if path == "/" {
		path = "/index.html"
	}

	if _, err := fs.Stat(subFS, path[1:]); err != nil {
		// File doesn't exist, serve index.html for SPA routing
		r.URL.Path = "/"
	}

	fileServer.ServeHTTP(w, r)
}

// corsMiddleware adds CORS headers.
// SECURITY: In production, set PINKY_CORS_ORIGIN environment variable to restrict origins.
// Default allows localhost origins only for development safety.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowedOrigin := getAllowedOrigin(origin)

		if allowedOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// getAllowedOrigin returns the origin to allow, or empty string if not allowed.
func getAllowedOrigin(origin string) string {
	// Check for environment variable override
	if envOrigin := getEnv("PINKY_CORS_ORIGIN"); envOrigin != "" {
		if envOrigin == "*" {
			return "*" // Explicitly allow all (not recommended for production)
		}
		if origin == envOrigin {
			return origin
		}
		return ""
	}

	// Default: only allow localhost origins for development safety
	if origin == "" {
		return ""
	}
	if strings.HasPrefix(origin, "http://localhost:") ||
		strings.HasPrefix(origin, "http://127.0.0.1:") ||
		origin == "http://localhost" ||
		origin == "http://127.0.0.1" {
		return origin
	}

	return ""
}

// getEnv is a helper to get environment variables (allows for testing)
func getEnv(key string) string {
	return "" // TODO: implement with os.Getenv when config system is ready
}

// Helper functions
func itoa(i int) string {
	return string(rune('0'+i/10000)) + string(rune('0'+(i/1000)%10)) + string(rune('0'+(i/100)%10)) + string(rune('0'+(i/10)%10)) + string(rune('0'+i%10))
}

func generateID() string {
	return time.Now().Format("20060102150405.000")
}

func generateResponse(input string) string {
	// Demo response generator
	switch {
	case contains(input, "deploy"):
		return "Poit! I'll help you deploy. Let me check the git status and run the tests first."
	case contains(input, "hello") || contains(input, "hi"):
		return "Zort! Hello there! What would you like to do tonight?"
	case contains(input, "help"):
		return "Narf! I can help with shell commands, file operations, Git, code execution, and more!"
	default:
		return "Egad! That sounds interesting. Would you like me to help with that?"
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
