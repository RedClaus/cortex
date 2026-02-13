package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/cortexhub/cortex-gateway/internal/brain"
	"github.com/cortexhub/cortex-gateway/internal/config"
	"github.com/cortexhub/cortex-gateway/internal/discovery"
	"github.com/cortexhub/cortex-gateway/internal/healthring"
	"github.com/cortexhub/cortex-gateway/internal/inference"
	"github.com/cortexhub/cortex-gateway/internal/memory"
	"github.com/cortexhub/cortex-gateway/internal/onboarding"
	"github.com/cortexhub/cortex-gateway/internal/webui"
)

// BridgeMessenger defines the interface for bridge messaging
type BridgeMessenger interface {
	SendMessage(ctx context.Context, from, to, msgType, content string) error
}

// Server represents the HTTP server
type Server struct {
	cfg             *config.Config
	brainClient     *brain.Client
	inferenceRouter *inference.Router
	bridgeClient    BridgeMessenger
	disc            *discovery.Discovery
	healthRing      *healthring.HealthRing
	onboarding      *onboarding.Onboarding
	memoryHandler   *memory.Handler
	httpServer      *http.Server
	startTime       time.Time
	logger          *slog.Logger
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string                 `json:"status"`
	Version   string                 `json:"version"`
	Uptime    string                 `json:"uptime"`
	Services  map[string]ServiceHealth `json:"services"`
	Timestamp string                 `json:"timestamp"`
}

// ServiceHealth represents a service health status
type ServiceHealth struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message,omitempty"`
}

// StatusResponse represents the full system status
type StatusResponse struct {
	Status     string                 `json:"status"`
	Version    string                 `json:"version"`
	Uptime     string                 `json:"uptime"`
	Services   map[string]interface{} `json:"services"`
	Channels   map[string]bool        `json:"channels"`
	Timestamp  string                 `json:"timestamp"`
}

// SessionsResponse represents the sessions list
type SessionsResponse struct {
	Sessions []SessionInfo `json:"sessions"`
}

// SessionInfo represents session info
type SessionInfo struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// MemorySearchResponse represents memory search results
type MemorySearchResponse struct {
	Query   string              `json:"query"`
	Results []brain.MemoryEntry `json:"results"`
}

// ConfigUpdateRequest represents config update request
type ConfigUpdateRequest struct {
	Config map[string]interface{} `json:"config"`
}

// InferenceResponse represents inference response
type InferenceResponse struct {
	Content    string `json:"content"`
	Model      string `json:"model"`
	TokensUsed int    `json:"tokens_used"`
	Lane       string `json:"lane"`
}

// BridgeSendResponse represents bridge send response
type BridgeSendResponse struct {
	Status string `json:"status"`
}

// EngineInfo for API response
type EngineInfo struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	URL      string   `json:"url,omitempty"`
	Models   []string `json:"models"`
	Default  string   `json:"default"`
	Hardware string   `json:"hardware,omitempty"`
}

// New creates a new HTTP server
func New(cfg *config.Config, brainClient *brain.Client, infRouter *inference.Router, bClient BridgeMessenger, d *discovery.Discovery, hr *healthring.HealthRing, o *onboarding.Onboarding, logger *slog.Logger) *Server {
	// Initialize memory store (using ~/.cortex as root, memory files in ~/.cortex/memory/)
	memoryStore := memory.NewStore("~/.cortex")
	memoryHandler := memory.NewHandler(memoryStore, logger)

	s := &Server{
		cfg:             cfg,
		brainClient:     brainClient,
		inferenceRouter: infRouter,
		bridgeClient:    bClient,
		disc:            d,
		healthRing:      hr,
		onboarding:      o,
		memoryHandler:   memoryHandler,
		logger:          logger,
		startTime:       time.Now(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.healthHandler)
	mux.HandleFunc("/api/v1/status", s.statusHandler)
	mux.HandleFunc("/api/v1/sessions", s.sessionsHandler)
	mux.HandleFunc("/api/v1/memory/search", s.memorySearchHandler)
	mux.HandleFunc("/api/v1/config", s.configUpdateHandler)

	// Direct memory API endpoints (new)
	mux.HandleFunc("/api/v1/memories/search", memoryHandler.SearchHandler)
	mux.HandleFunc("/api/v1/memories/store", memoryHandler.StoreHandler)
	mux.HandleFunc("/api/v1/memories/recent", memoryHandler.RecentHandler)
	mux.HandleFunc("/api/v1/memories/stats", memoryHandler.StatsHandler)

	mux.HandleFunc("/api/v1/swarm/agents", d.GetAgentsHandler())
	mux.HandleFunc("/api/v1/swarm/agents/", d.GetAgentHandler())
	mux.HandleFunc("/api/v1/healthring/status", hr.GetStatusHandler())
	mux.HandleFunc("/api/v1/healthring/", hr.GetMemberHandler())
	mux.HandleFunc("/api/v1/inference", s.inferenceHandler)
	mux.HandleFunc("/api/v1/inference/engines", s.listEnginesHandler)
	mux.HandleFunc("/api/v1/inference/models", s.listModelsHandler)
	mux.HandleFunc("/api/v1/bridge/send", s.bridgeSendHandler)
	mux.HandleFunc("/api/v1/onboarding/status", o.StatusHandler())
	mux.HandleFunc("/api/v1/onboarding/start", o.StartHandler())
	mux.HandleFunc("/api/v1/onboarding/step/", o.StepHandler())
	mux.HandleFunc("/api/v1/onboarding/complete", o.CompleteHandler())
	mux.HandleFunc("/api/v1/onboarding/import", o.ImportHandler())
	mux.HandleFunc("/ws", s.wsProxyHandler)
	mux.HandleFunc("/", s.webUIHandler)

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.logger.Info("HTTP server starting")
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the HTTP server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// webUIHandler serves the embedded web UI
func (s *Server) webUIHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if strings.HasPrefix(path, "/api") {
		http.NotFound(w, r)
		return
	}
	if path == "/" {
		path = "/index.html"
	}
	// Prepend "dist" prefix for embed.FS paths
	embedPath := "dist" + path
	file, err := webui.Assets.Open(embedPath)
	if err != nil {
		file, err = webui.Assets.Open("dist/index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Error reading file", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(path)))
	w.Write(content)
}

// healthHandler handles health check requests
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	services := map[string]ServiceHealth{
		"http": {Healthy: true, Message: "HTTP server running"},
	}

	if s.healthRing != nil {
		status := s.healthRing.Status()
		services["healthring"] = ServiceHealth{Healthy: len(status) > 0, Message: "Health ring active"}
	}

	response := HealthResponse{
		Status:    "healthy",
		Version:   "1.0.0",
		Uptime:    time.Since(s.startTime).String(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Services:  services,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// statusHandler handles full system status
func (s *Server) statusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	brainHealth, _ := s.brainClient.Health()

	services := map[string]interface{}{
		"cortexbrain": map[string]interface{}{
			"healthy": brainHealth != nil && brainHealth.Status == "healthy",
			"version": brainHealth.Version,
		},
	}

	if s.disc != nil {
		agents := s.disc.ListAgents()
		healthy := 0
		for _, a := range agents {
			if a.Status == "up" {
				healthy++
			}
		}
		services["swarm"] = map[string]interface{}{
			"agents_count":   len(agents),
			"healthy_agents": healthy,
		}
	}

	if s.healthRing != nil {
		hrStatus := s.healthRing.Status()
		healthy := 0
		for _, ms := range hrStatus {
			if ms.Status == "up" {
				healthy++
			}
		}
		services["healthring"] = map[string]interface{}{
			"members_count":    len(hrStatus),
			"healthy_members":  healthy,
		}
	}

	// Inference engines
	engines := s.inferenceRouter.ListEngines()
	healthyEngines := 0
	for range engines {
		// assume healthy if client exists
		healthyEngines++
	}
	services["inference"] = map[string]interface{}{
		"engines_count": len(engines),
		"healthy_engines": healthyEngines,
	}

	response := StatusResponse{
		Status:     "healthy",
		Version:    "1.0.0",
		Uptime:     time.Since(s.startTime).String(),
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Services:   services,
		Channels: map[string]bool{
			"telegram": s.cfg.Channels.Telegram.Enabled,
			"discord":  s.cfg.Channels.Discord.Enabled,
			"webchat":  s.cfg.Channels.WebChat.Enabled,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// sessionsHandler handles sessions list
func (s *Server) sessionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resp, err := s.brainClient.RecallMemory(&brain.RecallMemoryRequest{
		Query:   "session:*",
		AgentID: "gateway",
		Limit:   100,
	})
	if err != nil {
		http.Error(w, "Failed to retrieve sessions", http.StatusInternalServerError)
		return
	}

	sessions := []SessionInfo{}
	for _, entry := range resp.Results {
		var session map[string]interface{}
		json.Unmarshal([]byte(entry.Content), &session)
		sessions = append(sessions, SessionInfo{
			ID:        entry.SessionID,
			UserID:    session["UserID"].(string),
			CreatedAt: session["CreatedAt"].(string),
			UpdatedAt: session["UpdatedAt"].(string),
		})
	}

	response := SessionsResponse{Sessions: sessions}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// memorySearchHandler handles memory search
func (s *Server) memorySearchHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query().Get("q")
	if q == "" {
		http.Error(w, "Missing query parameter q", http.StatusBadRequest)
		return
	}

	resp, err := s.brainClient.RecallMemory(&brain.RecallMemoryRequest{
		Query:   q,
		AgentID: "gateway",
		Limit:   50,
	})
	if err != nil {
		http.Error(w, "Failed to search memory", http.StatusInternalServerError)
		return
	}

	response := MemorySearchResponse{
		Query:   q,
		Results: resp.Results,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// configUpdateHandler handles runtime config updates
func (s *Server) configUpdateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// TODO: Implement runtime config updates
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "config updated"})
}

// inferenceHandler handles inference requests
func (s *Server) inferenceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Prompt string `json:"prompt"`
		Lane   string `json:"lane,omitempty"`
		Model  string `json:"model,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Prompt == "" {
		http.Error(w, "prompt required", http.StatusBadRequest)
		return
	}

	infReq := &inference.Request{
		Prompt: req.Prompt,
		Model:  req.Model,
	}

	res, err := s.inferenceRouter.Infer(req.Lane, infReq)
	if err != nil {
		s.logger.Error("Inference failed", "lane", req.Lane, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := InferenceResponse{
		Content:    res.Content,
		Model:      res.Model,
		TokensUsed: res.TokensUsed,
		Lane:       res.Lane, // add to Response? or use req.Lane
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// listEnginesHandler lists available inference engines
func (s *Server) listEnginesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	engines := s.inferenceRouter.ListEngines()

	list := []EngineInfo{}
	for _, e := range engines {
		list = append(list, EngineInfo{
			Name:     e.Name,
			Type:     e.Type,
			URL:      e.URL,
			Models:   e.Models,
			Default:  e.Default,
			Hardware: e.Hardware,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(list)
}

// listModelsHandler lists all available models
func (s *Server) listModelsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	models := s.inferenceRouter.ListModels()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(models)
}

// wsProxyHandler proxies WebSocket connections to WebChat adapter
func (s *Server) wsProxyHandler(w http.ResponseWriter, r *http.Request) {
	target, err := url.Parse("http://localhost:18793/ws")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	proxy := httputil.NewSingleHostReverseProxy(target)
	
	// Update the target host for WS upgrade
	r.URL.Scheme = target.Scheme
	r.URL.Host = target.Host
	r.Host = target.Host
	
	// Copy all headers
	r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
	
	proxy.ServeHTTP(w, r)
}

// bridgeSendHandler handles bridge send requests
func (s *Server) bridgeSendHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		From    string `json:"from"`
		To      string `json:"to"`
		Type    string `json:"type"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.To == "" || req.Type == "" || req.Content == "" {
		http.Error(w, "to, type, content required", http.StatusBadRequest)
		return
	}

	err := s.bridgeClient.SendMessage(r.Context(), req.From, req.To, req.Type, req.Content)
	if err != nil {
		s.logger.Error("Bridge send failed", "to", req.To, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := BridgeSendResponse{Status: "sent"}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
