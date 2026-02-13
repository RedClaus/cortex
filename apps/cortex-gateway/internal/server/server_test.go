package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cortexhub/cortex-gateway/internal/config"
)

func testServer(t *testing.T, port int) *Server {
	t.Helper()
	cfg := &config.Config{
		Server:      config.ServerConfig{Port: port, Host: "localhost"},
		CortexBrain: config.CortexBrainConfig{URL: "http://localhost:18892"},
		Ollama:      config.OllamaConfig{URL: "http://localhost:11434"},
		Inference:   config.InferenceConfig{DefaultLane: "local", Lanes: []config.LaneConfig{{Name: "local", Provider: "ollama", BaseURL: "http://localhost:11434", Models: []string{"test"}}}},
	}
	return New(cfg, nil, nil, nil, nil, nil, nil, slog.Default())
}

func TestNew(t *testing.T) {
	srv := testServer(t, 18800)
	if srv == nil {
		t.Fatal("Expected non-nil server")
	}
}

func TestHealthHandler(t *testing.T) {
	srv := testServer(t, 18800)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	srv.healthHandler(w, req)
	resp := w.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
	var hr HealthResponse
	json.NewDecoder(resp.Body).Decode(&hr)
	if hr.Status != "healthy" {
		t.Errorf("Expected healthy, got %s", hr.Status)
	}
}

func TestShutdown(t *testing.T) {
	srv := testServer(t, 18801)
	go srv.Start()
	time.Sleep(100 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}
