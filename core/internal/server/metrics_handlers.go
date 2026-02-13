package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/normanking/cortex/internal/llm"
)

// LLMMetricsResponse is the JSON response for the metrics endpoint.
type LLMMetricsResponse struct {
	Timestamp string                 `json:"timestamp"`
	Summary   map[string]interface{} `json:"summary"`
	Providers map[string]interface{} `json:"providers"`
}

// HandleLLMMetrics returns LLM call metrics as JSON.
// GET /api/metrics/llm
func HandleLLMMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get metrics from global registry
	providers := llm.GetAllMetrics()
	summary := llm.GetMetricsSummary()

	response := LLMMetricsResponse{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Summary:   summary,
		Providers: providers,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// HandleMetricsReset resets all LLM metrics.
// POST /api/metrics/llm/reset
func HandleMetricsReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	llm.ResetAllMetrics()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "All LLM metrics have been reset",
	})
}

// RegisterMetricsRoutes registers all metrics-related routes on the given mux.
func RegisterMetricsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/metrics/llm", HandleLLMMetrics)
	mux.HandleFunc("POST /api/metrics/llm/reset", HandleMetricsReset)
}
