package memory

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
)

// Handler handles memory HTTP requests
type Handler struct {
	store  *Store
	logger *slog.Logger
}

// NewHandler creates a new memory handler
func NewHandler(store *Store, logger *slog.Logger) *Handler {
	return &Handler{
		store:  store,
		logger: logger,
	}
}

// SearchRequest represents a memory search request
type SearchRequest struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

// SearchResponse represents a memory search response
type SearchResponse struct {
	Query   string  `json:"query"`
	Results []Entry `json:"results"`
	Count   int     `json:"count"`
}

// StoreRequest represents a memory store request
type StoreRequest struct {
	Content    string  `json:"content"`
	Importance float64 `json:"importance,omitempty"`
	Type       string  `json:"type,omitempty"`
}

// StoreResponse represents a memory store response
type StoreResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// RecentRequest represents a recent memories request
type RecentRequest struct {
	Limit int `json:"limit,omitempty"`
}

// RecentResponse represents a recent memories response
type RecentResponse struct {
	Results []Entry `json:"results"`
	Count   int     `json:"count"`
}

// StatsResponse represents memory statistics response
type StatsResponse struct {
	Stats *Stats `json:"stats"`
}

// SearchHandler handles GET /api/v1/memories/search
func (h *Handler) SearchHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get query parameter
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Missing query parameter 'q'", http.StatusBadRequest)
		return
	}

	// Get limit parameter (optional)
	limitStr := r.URL.Query().Get("limit")
	limit := 50 // default
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// Search memories
	results, err := h.store.Search(query, limit)
	if err != nil {
		h.logger.Error("Failed to search memories", "error", err)
		http.Error(w, "Failed to search memories", http.StatusInternalServerError)
		return
	}

	// Return response
	response := SearchResponse{
		Query:   query,
		Results: results,
		Count:   len(results),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)

	h.logger.Info("Memory search", "query", query, "results", len(results))
}

// StoreHandler handles POST /api/v1/memories/store
func (h *Handler) StoreHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req StoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate content
	if req.Content == "" {
		http.Error(w, "Missing content", http.StatusBadRequest)
		return
	}

	// Default values
	if req.Importance == 0 {
		req.Importance = 0.5
	}
	if req.Type == "" {
		req.Type = "episodic"
	}

	// Store memory
	if err := h.store.Store(req.Content, req.Importance, req.Type); err != nil {
		h.logger.Error("Failed to store memory", "error", err)
		http.Error(w, "Failed to store memory", http.StatusInternalServerError)
		return
	}

	// Return response
	response := StoreResponse{
		Status:  "success",
		Message: "Memory stored successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)

	h.logger.Info("Memory stored", "type", req.Type, "importance", req.Importance)
}

// RecentHandler handles GET /api/v1/memories/recent
func (h *Handler) RecentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get limit parameter (optional)
	limitStr := r.URL.Query().Get("limit")
	limit := 10 // default
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// Get recent memories
	results, err := h.store.Recent(limit)
	if err != nil {
		h.logger.Error("Failed to get recent memories", "error", err)
		http.Error(w, "Failed to get recent memories", http.StatusInternalServerError)
		return
	}

	// Return response
	response := RecentResponse{
		Results: results,
		Count:   len(results),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)

	h.logger.Info("Recent memories retrieved", "count", len(results))
}

// StatsHandler handles GET /api/v1/memories/stats
func (h *Handler) StatsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get memory stats
	stats, err := h.store.Stats()
	if err != nil {
		h.logger.Error("Failed to get memory stats", "error", err)
		http.Error(w, "Failed to get memory stats", http.StatusInternalServerError)
		return
	}

	// Return response
	response := StatsResponse{
		Stats: stats,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)

	h.logger.Info("Memory stats retrieved",
		"total", stats.TotalEntries,
		"episodic", stats.EpisodicCount,
		"knowledge", stats.KnowledgeCount,
	)
}
