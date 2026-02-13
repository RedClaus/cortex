// Package resemble provides Resemble.ai API clients and webhook server for Voice Agents.
// This file implements the webhook server for CR-016 Resemble Agents Integration.
package resemble

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/normanking/cortex/internal/logging"
)

// WebhookServer handles incoming webhook requests from Resemble Agents.
// It bridges Resemble's tool calls to Cortex's tool execution system.
type WebhookServer struct {
	port      int
	host      string
	authToken string
	server    *http.Server
	log       *logging.Logger

	// Tool executor function - set by the caller to execute Cortex tools
	toolExecutor ToolExecutor

	// Session management
	sessions   map[string]*Session
	sessionsMu sync.RWMutex

	// Server state
	running bool
	mu      sync.Mutex
}

// ToolExecutor is the function signature for executing Cortex tools.
type ToolExecutor func(ctx context.Context, toolName string, args map[string]interface{}, sessionID string) (interface{}, error)

// Session tracks an active voice agent session.
type Session struct {
	ID        string
	AgentUUID string
	AgentName string
	StartedAt time.Time
	LastCall  time.Time
	ToolCalls int
}

// WebhookConfig holds configuration for the webhook server.
type WebhookConfig struct {
	Port      int
	Host      string
	AuthToken string
}

// DefaultWebhookConfig returns sensible defaults for the webhook server.
func DefaultWebhookConfig() WebhookConfig {
	return WebhookConfig{
		Port:      8766,
		Host:      "localhost",
		AuthToken: "",
	}
}

// NewWebhookServer creates a new webhook server for Resemble Agent tool calls.
func NewWebhookServer(cfg WebhookConfig) *WebhookServer {
	return &WebhookServer{
		port:      cfg.Port,
		host:      cfg.Host,
		authToken: cfg.AuthToken,
		log:       logging.Global(),
		sessions:  make(map[string]*Session),
	}
}

// SetToolExecutor sets the function that executes Cortex tools.
func (s *WebhookServer) SetToolExecutor(executor ToolExecutor) {
	s.toolExecutor = executor
}

// Start starts the webhook server.
func (s *WebhookServer) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("webhook server already running")
	}
	s.running = true
	s.mu.Unlock()

	mux := http.NewServeMux()

	// Tool endpoints
	mux.HandleFunc("/api/tools/execute", s.handleToolExecute)
	mux.HandleFunc("/api/tools/list", s.handleToolList)

	// Memory endpoint
	mux.HandleFunc("/api/memory/query", s.handleMemoryQuery)

	// Session endpoints
	mux.HandleFunc("/api/session/start", s.handleSessionStart)
	mux.HandleFunc("/api/session/end", s.handleSessionEnd)

	// Health check
	mux.HandleFunc("/health", s.handleHealth)

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.withMiddleware(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	s.log.Info("[WebhookServer] Starting on %s", addr)

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Error("[WebhookServer] Server error: %v", err)
		}
	}()

	return nil
}

// Stop stops the webhook server gracefully.
func (s *WebhookServer) Stop() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = false
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.log.Info("[WebhookServer] Shutting down...")
	return s.server.Shutdown(ctx)
}

// IsRunning returns whether the server is running.
func (s *WebhookServer) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// GetAddress returns the server address.
func (s *WebhookServer) GetAddress() string {
	return fmt.Sprintf("http://%s:%d", s.host, s.port)
}

// withMiddleware wraps the handler with logging and auth middleware.
func (s *WebhookServer) withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Auth check (if token configured)
		if s.authToken != "" {
			auth := r.Header.Get("Authorization")
			expected := "Bearer " + s.authToken
			if auth != expected {
				s.log.Warn("[WebhookServer] Unauthorized request from %s", r.RemoteAddr)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}

		// Set CORS headers for local development
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)

		s.log.Info("[WebhookServer] %s %s %v", r.Method, r.URL.Path, time.Since(start))
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// TOOL EXECUTION
// ═══════════════════════════════════════════════════════════════════════════════

// ToolExecuteRequest is the request body for tool execution.
type ToolExecuteRequest struct {
	Tool      string                 `json:"tool"`
	Args      map[string]interface{} `json:"args"`
	SessionID string                 `json:"session_id,omitempty"`
}

// ToolExecuteResponse is the response from tool execution.
type ToolExecuteResponse struct {
	Success bool        `json:"success"`
	Result  interface{} `json:"result,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// handleToolExecute handles POST /api/tools/execute.
func (s *WebhookServer) handleToolExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.sendError(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var req ToolExecuteRequest
	if err := json.Unmarshal(body, &req); err != nil {
		s.sendError(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	s.log.Info("[WebhookServer] Tool execute: tool=%s session=%s", req.Tool, req.SessionID)

	// Check if tool executor is configured
	if s.toolExecutor == nil {
		s.sendError(w, "Tool executor not configured", http.StatusServiceUnavailable)
		return
	}

	// Update session stats
	if req.SessionID != "" {
		s.updateSessionStats(req.SessionID)
	}

	// Execute the tool
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	result, err := s.toolExecutor(ctx, req.Tool, req.Args, req.SessionID)
	if err != nil {
		s.log.Warn("[WebhookServer] Tool execution failed: %v", err)
		s.sendJSON(w, ToolExecuteResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	s.sendJSON(w, ToolExecuteResponse{
		Success: true,
		Result:  result,
	})
}

// ToolInfo describes an available tool.
type ToolInfo struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Parameters  map[string]string `json:"parameters,omitempty"`
}

// ToolListResponse is the response from /api/tools/list.
type ToolListResponse struct {
	Success bool       `json:"success"`
	Tools   []ToolInfo `json:"tools"`
}

// handleToolList handles GET /api/tools/list.
func (s *WebhookServer) handleToolList(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Return the list of available Cortex tools
	tools := []ToolInfo{
		{
			Name:        "bash",
			Description: "Execute shell commands",
			Parameters:  map[string]string{"command": "The command to execute"},
		},
		{
			Name:        "read_file",
			Description: "Read contents of a file",
			Parameters:  map[string]string{"path": "Path to the file"},
		},
		{
			Name:        "write_file",
			Description: "Write content to a file",
			Parameters:  map[string]string{"path": "Path to the file", "content": "Content to write"},
		},
		{
			Name:        "list_directory",
			Description: "List files in a directory",
			Parameters:  map[string]string{"path": "Directory path"},
		},
		{
			Name:        "glob",
			Description: "Find files matching a pattern",
			Parameters:  map[string]string{"pattern": "Glob pattern"},
		},
		{
			Name:        "grep",
			Description: "Search file contents",
			Parameters:  map[string]string{"pattern": "Search pattern", "path": "Directory to search"},
		},
		{
			Name:        "websearch",
			Description: "Search the web",
			Parameters:  map[string]string{"query": "Search query"},
		},
		{
			Name:        "memory_query",
			Description: "Query Cortex memory/knowledge base",
			Parameters:  map[string]string{"query": "Search query"},
		},
	}

	s.sendJSON(w, ToolListResponse{
		Success: true,
		Tools:   tools,
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// MEMORY QUERY
// ═══════════════════════════════════════════════════════════════════════════════

// MemoryQueryRequest is the request for memory queries.
type MemoryQueryRequest struct {
	Query     string `json:"query"`
	Limit     int    `json:"limit,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

// MemoryQueryResponse is the response from memory queries.
type MemoryQueryResponse struct {
	Success bool          `json:"success"`
	Results []MemoryEntry `json:"results,omitempty"`
	Error   string        `json:"error,omitempty"`
}

// MemoryEntry represents a memory/knowledge item.
type MemoryEntry struct {
	ID       string  `json:"id"`
	Content  string  `json:"content"`
	Score    float64 `json:"score"`
	Source   string  `json:"source,omitempty"`
	Metadata string  `json:"metadata,omitempty"`
}

// handleMemoryQuery handles GET /api/memory/query.
func (s *WebhookServer) handleMemoryQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req MemoryQueryRequest

	if r.Method == "GET" {
		req.Query = r.URL.Query().Get("query")
	} else {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			s.sendError(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		if err := json.Unmarshal(body, &req); err != nil {
			s.sendError(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	if req.Query == "" {
		s.sendError(w, "Query parameter is required", http.StatusBadRequest)
		return
	}

	s.log.Info("[WebhookServer] Memory query: %s", req.Query)

	// TODO: Integrate with Cortex knowledge base
	// For now, return empty results
	s.sendJSON(w, MemoryQueryResponse{
		Success: true,
		Results: []MemoryEntry{},
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// SESSION MANAGEMENT
// ═══════════════════════════════════════════════════════════════════════════════

// SessionStartRequest is the request to start a session.
type SessionStartRequest struct {
	AgentUUID string `json:"agent_uuid"`
	AgentName string `json:"agent_name,omitempty"`
}

// SessionResponse is the response for session operations.
type SessionResponse struct {
	Success   bool   `json:"success"`
	SessionID string `json:"session_id,omitempty"`
	Error     string `json:"error,omitempty"`
}

// handleSessionStart handles POST /api/session/start.
func (s *WebhookServer) handleSessionStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.sendError(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var req SessionStartRequest
	if err := json.Unmarshal(body, &req); err != nil {
		s.sendError(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Generate session ID
	sessionID := fmt.Sprintf("session_%d", time.Now().UnixNano())

	session := &Session{
		ID:        sessionID,
		AgentUUID: req.AgentUUID,
		AgentName: req.AgentName,
		StartedAt: time.Now(),
		LastCall:  time.Now(),
	}

	s.sessionsMu.Lock()
	s.sessions[sessionID] = session
	s.sessionsMu.Unlock()

	s.log.Info("[WebhookServer] Session started: %s (agent=%s)", sessionID, req.AgentName)

	s.sendJSON(w, SessionResponse{
		Success:   true,
		SessionID: sessionID,
	})
}

// SessionEndRequest is the request to end a session.
type SessionEndRequest struct {
	SessionID string `json:"session_id"`
}

// handleSessionEnd handles POST /api/session/end.
func (s *WebhookServer) handleSessionEnd(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.sendError(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var req SessionEndRequest
	if err := json.Unmarshal(body, &req); err != nil {
		s.sendError(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	s.sessionsMu.Lock()
	session, exists := s.sessions[req.SessionID]
	if exists {
		delete(s.sessions, req.SessionID)
	}
	s.sessionsMu.Unlock()

	if !exists {
		s.sendError(w, "Session not found", http.StatusNotFound)
		return
	}

	s.log.Info("[WebhookServer] Session ended: %s (calls=%d)", req.SessionID, session.ToolCalls)

	s.sendJSON(w, SessionResponse{
		Success:   true,
		SessionID: req.SessionID,
	})
}

// updateSessionStats updates the stats for an active session.
func (s *WebhookServer) updateSessionStats(sessionID string) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	if session, exists := s.sessions[sessionID]; exists {
		session.LastCall = time.Now()
		session.ToolCalls++
	}
}

// handleHealth handles GET /health.
func (s *WebhookServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.sendJSON(w, map[string]interface{}{
		"status":   "healthy",
		"service":  "cortex-webhook",
		"version":  "1.0.0",
		"sessions": len(s.sessions),
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

// sendJSON sends a JSON response.
func (s *WebhookServer) sendJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// sendError sends an error response.
func (s *WebhookServer) sendError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   message,
	})
}

// GetSessions returns the current active sessions.
func (s *WebhookServer) GetSessions() []*Session {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	sessions := make([]*Session, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}
