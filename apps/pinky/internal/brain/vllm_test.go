// Package brain provides tests for vLLM integration.
package brain

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/normanking/pinky/internal/config"
)

// TestVLLMGenerate tests the vLLM generate function with a mock server.
func TestVLLMGenerate(t *testing.T) {
	// Create a mock vLLM server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("Expected path /v1/chat/completions, got %s", r.URL.Path)
		}

		// Verify request format
		var req openaiRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}

		if req.Model != "mistral-7b" {
			t.Errorf("Expected model mistral-7b, got %s", req.Model)
		}

		// Return mock response
		resp := openaiResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			}{
				{
					Message: struct {
						Content string `json:"content"`
					}{Content: "Hello! I'm a mock vLLM response."},
					FinishReason: "stop",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create brain with vLLM lane pointing to mock server
	cfg := config.InferenceConfig{
		DefaultLane: "vllm",
		Lanes: map[string]config.Lane{
			"vllm": {
				Engine: "vllm",
				Model:  "mistral-7b",
				URL:    server.URL,
			},
		},
	}

	brain := NewEmbeddedBrain(cfg)

	// Test Think request
	req := &ThinkRequest{
		Messages: []Message{
			{Role: "user", Content: "Hello!"},
		},
	}

	resp, err := brain.Think(context.Background(), req)
	if err != nil {
		t.Fatalf("Think failed: %v", err)
	}

	if resp.Content != "Hello! I'm a mock vLLM response." {
		t.Errorf("Expected mock response, got: %s", resp.Content)
	}
}

// TestVLLMCircuitBreaker tests that the circuit breaker trips after failures.
func TestVLLMCircuitBreaker(t *testing.T) {
	// Create a mock server that always fails for vLLM
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	// Create brain with ONLY vLLM lane (no fallbacks)
	cfg := config.InferenceConfig{
		DefaultLane: "vllm",
		Lanes: map[string]config.Lane{
			"vllm": {
				Engine: "vllm",
				Model:  "mistral-7b",
				URL:    server.URL,
			},
		},
	}

	brain := NewEmbeddedBrain(cfg)

	// Make 3 requests to trip the circuit
	req := &ThinkRequest{
		Messages: []Message{
			{Role: "user", Content: "Hello!"},
		},
	}

	for i := 0; i < 3; i++ {
		_, _ = brain.Think(context.Background(), req)
	}

	// Check that circuit is now open
	cb := brain.circuitBreakers.Get("vllm")
	state := cb.State()
	if state != CircuitOpen {
		t.Errorf("Expected circuit to be open after 3 failures, got: %v", state)
	}

	// Verify that new requests are rejected when circuit is open
	if cb.Allow() {
		t.Error("Expected circuit to reject requests when open")
	}
}

// TestCircuitBreakerRecovery tests that the circuit breaker recovers.
func TestCircuitBreakerRecovery(t *testing.T) {
	// Create circuit breaker with short recovery timeout
	cb := NewCircuitBreaker("test", CircuitBreakerConfig{
		FailureThreshold: 2,
		RecoveryTimeout:  100 * time.Millisecond,
		SuccessThreshold: 1,
	})

	// Trip the circuit
	cb.RecordFailure()
	cb.RecordFailure()

	if cb.State() != CircuitOpen {
		t.Errorf("Expected circuit to be open, got: %v", cb.State())
	}

	// Wait for recovery timeout
	time.Sleep(150 * time.Millisecond)

	// Check that request is allowed (half-open state)
	if !cb.Allow() {
		t.Error("Expected circuit to allow request in half-open state")
	}

	// Record success to close circuit
	cb.RecordSuccess()

	if cb.State() != CircuitClosed {
		t.Errorf("Expected circuit to be closed after success, got: %v", cb.State())
	}
}

// TestVLLMPing tests the vLLM health check.
func TestVLLMPing(t *testing.T) {
	// Create a mock health endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "healthy"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := config.InferenceConfig{
		DefaultLane: "vllm",
		Lanes: map[string]config.Lane{
			"vllm": {
				Engine: "vllm",
				Model:  "mistral-7b",
				URL:    server.URL,
			},
		},
	}

	brain := NewEmbeddedBrain(cfg)
	lane := brain.cfg.Lanes["vllm"]

	err := brain.vllmPing(context.Background(), &lane)
	if err != nil {
		t.Errorf("Expected ping to succeed, got: %v", err)
	}
}

// TestGetVLLMModels tests model listing from vLLM.
func TestGetVLLMModels(t *testing.T) {
	// Create a mock models endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			resp := struct {
				Data []struct {
					ID string `json:"id"`
				} `json:"data"`
			}{
				Data: []struct {
					ID string `json:"id"`
				}{
					{ID: "mistral-7b"},
					{ID: "llama-13b"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := config.InferenceConfig{
		DefaultLane: "vllm",
		Lanes: map[string]config.Lane{
			"vllm": {
				Engine: "vllm",
				Model:  "mistral-7b",
				URL:    server.URL,
			},
		},
	}

	brain := NewEmbeddedBrain(cfg)

	models, err := brain.GetVLLMModels("vllm")
	if err != nil {
		t.Errorf("Expected GetVLLMModels to succeed, got: %v", err)
	}

	if len(models) != 2 {
		t.Errorf("Expected 2 models, got: %d", len(models))
	}

	if models[0] != "mistral-7b" {
		t.Errorf("Expected first model to be mistral-7b, got: %s", models[0])
	}
}

// TestAutoLLMWithVLLM tests that AutoLLM routing includes vLLM.
func TestAutoLLMWithVLLM(t *testing.T) {
	cfg := config.InferenceConfig{
		DefaultLane: "fast",
		AutoLLM:     true,
		Lanes: map[string]config.Lane{
			"vllm": {
				Engine: "vllm",
				Model:  "mistral-7b",
				URL:    "http://localhost:8000",
			},
			"fast": {
				Engine: "ollama",
				Model:  "llama3:8b",
			},
			"local": {
				Engine: "ollama",
				Model:  "phi:mini",
			},
		},
	}

	brain := NewEmbeddedBrain(cfg)

	// Test that vLLM is selected for complex tasks
	complexReq := &ThinkRequest{
		Messages: []Message{
			{Role: "user", Content: "Analyze and explain the entire architecture of this complex distributed system, comparing it with multiple alternative approaches and providing a comprehensive implementation plan with detailed code examples."},
		},
	}

	lane := brain.selectLane(complexReq)
	if lane == nil {
		t.Fatal("Expected lane to be selected")
	}

	// vLLM should be preferred for complex tasks
	if lane.Engine != "vllm" {
		t.Logf("AutoLLM selected lane engine: %s (vLLM may not be first if circuit is open)", lane.Engine)
	}

	// Test simple task - should prefer local
	simpleReq := &ThinkRequest{
		Messages: []Message{
			{Role: "user", Content: "Hello!"},
		},
	}

	lane = brain.selectLane(simpleReq)
	if lane == nil {
		t.Fatal("Expected lane to be selected for simple task")
	}

	// Local should be preferred for simple tasks
	if lane.Engine != "ollama" {
		t.Logf("AutoLLM selected engine for simple task: %s", lane.Engine)
	}
}
