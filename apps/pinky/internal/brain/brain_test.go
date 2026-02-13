// Package brain tests
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

func TestBrainMode(t *testing.T) {
	if ModeEmbedded != "embedded" {
		t.Errorf("expected ModeEmbedded='embedded', got %q", ModeEmbedded)
	}
	if ModeRemote != "remote" {
		t.Errorf("expected ModeRemote='remote', got %q", ModeRemote)
	}
}

func TestMemoryType(t *testing.T) {
	tests := []struct {
		memType MemoryType
		want    string
	}{
		{MemoryEpisodic, "episodic"},
		{MemorySemantic, "semantic"},
		{MemoryProcedural, "procedural"},
	}

	for _, tt := range tests {
		if string(tt.memType) != tt.want {
			t.Errorf("expected %q, got %q", tt.want, tt.memType)
		}
	}
}

func TestNew_Embedded(t *testing.T) {
	cfg := &config.Config{
		Brain: config.BrainConfig{
			Mode: "embedded",
		},
		Inference: config.InferenceConfig{
			DefaultLane: "fast",
			Lanes: map[string]config.Lane{
				"fast": {Engine: "ollama", Model: "llama3:8b"},
			},
		},
	}

	brain, err := New(cfg)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if brain == nil {
		t.Fatal("New() returned nil")
	}
	if brain.Mode() != ModeEmbedded {
		t.Errorf("expected ModeEmbedded, got %s", brain.Mode())
	}
}

func TestNew_EmptyModeDefaultsToEmbedded(t *testing.T) {
	cfg := &config.Config{
		Brain: config.BrainConfig{
			Mode: "", // Empty should default to embedded
		},
		Inference: config.InferenceConfig{
			DefaultLane: "fast",
			Lanes: map[string]config.Lane{
				"fast": {Engine: "ollama", Model: "llama3:8b"},
			},
		},
	}

	brain, err := New(cfg)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if brain.Mode() != ModeEmbedded {
		t.Errorf("expected ModeEmbedded for empty mode, got %s", brain.Mode())
	}
}

func TestNew_Remote(t *testing.T) {
	cfg := &config.Config{
		Brain: config.BrainConfig{
			Mode:      "remote",
			RemoteURL: "http://localhost:8080",
		},
	}

	brain, err := New(cfg)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if brain == nil {
		t.Fatal("New() returned nil")
	}
	if brain.Mode() != ModeRemote {
		t.Errorf("expected ModeRemote, got %s", brain.Mode())
	}
}

func TestNew_RemoteWithoutURL(t *testing.T) {
	cfg := &config.Config{
		Brain: config.BrainConfig{
			Mode:      "remote",
			RemoteURL: "", // Missing URL
		},
	}

	_, err := New(cfg)
	if err == nil {
		t.Error("expected error for remote mode without URL")
	}
}

func TestNew_UnknownMode(t *testing.T) {
	cfg := &config.Config{
		Brain: config.BrainConfig{
			Mode: "unknown",
		},
	}

	_, err := New(cfg)
	if err == nil {
		t.Error("expected error for unknown mode")
	}
}

func TestEmbeddedBrain_Mode(t *testing.T) {
	brain := NewEmbeddedBrain(config.InferenceConfig{})
	if brain.Mode() != ModeEmbedded {
		t.Errorf("expected ModeEmbedded, got %s", brain.Mode())
	}
}

func TestEmbeddedBrain_RememberAndRecall(t *testing.T) {
	brain := NewEmbeddedBrain(config.InferenceConfig{})
	ctx := context.Background()

	// Remember some memories
	memories := []*Memory{
		{Content: "The capital of France is Paris", Type: MemorySemantic},
		{Content: "User asked about weather yesterday", Type: MemoryEpisodic},
		{Content: "How to make coffee: boil water, add grounds", Type: MemoryProcedural},
	}

	for _, mem := range memories {
		if err := brain.Remember(ctx, mem); err != nil {
			t.Fatalf("Remember() failed: %v", err)
		}
	}

	// Recall matching memories
	results, err := brain.Recall(ctx, "france", 10)
	if err != nil {
		t.Fatalf("Recall() failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'france', got %d", len(results))
	}
	if len(results) > 0 && results[0].Content != "The capital of France is Paris" {
		t.Error("unexpected memory content")
	}

	// Recall with limit
	results, err = brain.Recall(ctx, "a", 2) // Should match all but limited to 2
	if err != nil {
		t.Fatalf("Recall() failed: %v", err)
	}
	if len(results) > 2 {
		t.Errorf("expected max 2 results, got %d", len(results))
	}

	// Recall with no matches
	results, err = brain.Recall(ctx, "nonexistent", 10)
	if err != nil {
		t.Fatalf("Recall() failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for 'nonexistent', got %d", len(results))
	}
}

func TestEmbeddedBrain_Remember_SetsTimestamps(t *testing.T) {
	brain := NewEmbeddedBrain(config.InferenceConfig{})
	ctx := context.Background()

	mem := &Memory{Content: "Test memory"}

	before := time.Now()
	if err := brain.Remember(ctx, mem); err != nil {
		t.Fatalf("Remember() failed: %v", err)
	}
	after := time.Now()

	results, _ := brain.Recall(ctx, "Test", 1)
	if len(results) == 0 {
		t.Fatal("no results returned")
	}

	if results[0].CreatedAt.Before(before) || results[0].CreatedAt.After(after) {
		t.Error("CreatedAt not set correctly")
	}
	if results[0].AccessedAt.Before(before) || results[0].AccessedAt.After(after) {
		t.Error("AccessedAt not set correctly")
	}
}

func TestEmbeddedBrain_Think_NoDefaultLane(t *testing.T) {
	brain := NewEmbeddedBrain(config.InferenceConfig{
		DefaultLane: "nonexistent",
		Lanes:       map[string]config.Lane{},
	})
	ctx := context.Background()

	_, err := brain.Think(ctx, &ThinkRequest{})
	if err == nil {
		t.Error("expected error when no default lane configured")
	}
}

func TestEmbeddedBrain_ThinkStream_NoDefaultLane(t *testing.T) {
	brain := NewEmbeddedBrain(config.InferenceConfig{
		DefaultLane: "nonexistent",
		Lanes:       map[string]config.Lane{},
	})
	ctx := context.Background()

	_, err := brain.ThinkStream(ctx, &ThinkRequest{})
	if err == nil {
		t.Error("expected error when no default lane configured")
	}
}

func TestEmbeddedBrain_Think_UnsupportedEngine(t *testing.T) {
	brain := NewEmbeddedBrain(config.InferenceConfig{
		DefaultLane: "fast",
		Lanes: map[string]config.Lane{
			"fast": {Engine: "unsupported", Model: "test"},
		},
	})
	ctx := context.Background()

	_, err := brain.Think(ctx, &ThinkRequest{})
	if err == nil {
		t.Error("expected error for unsupported engine")
	}
}

func TestRemoteBrain_Mode(t *testing.T) {
	brain := NewRemoteBrain(config.BrainConfig{
		RemoteURL: "http://localhost:8080",
	})
	if brain.Mode() != ModeRemote {
		t.Errorf("expected ModeRemote, got %s", brain.Mode())
	}
}

func TestRemoteBrain_Ping(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("expected path /health, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	brain := NewRemoteBrain(config.BrainConfig{
		RemoteURL: server.URL,
	})

	ctx := context.Background()
	if err := brain.Ping(ctx); err != nil {
		t.Errorf("Ping() failed: %v", err)
	}
}

func TestRemoteBrain_Ping_Unhealthy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	brain := NewRemoteBrain(config.BrainConfig{
		RemoteURL: server.URL,
	})

	ctx := context.Background()
	if err := brain.Ping(ctx); err == nil {
		t.Error("expected error for unhealthy remote")
	}
}

func TestRemoteBrain_Think(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/think" {
			t.Errorf("expected path /v1/think, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		resp := map[string]interface{}{
			"content": "Hello, I'm Pinky!",
			"done":    true,
			"usage": map[string]int{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	brain := NewRemoteBrain(config.BrainConfig{
		RemoteURL: server.URL,
	})

	ctx := context.Background()
	req := &ThinkRequest{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	}

	resp, err := brain.Think(ctx, req)
	if err != nil {
		t.Fatalf("Think() failed: %v", err)
	}

	if resp.Content != "Hello, I'm Pinky!" {
		t.Errorf("expected content 'Hello, I'm Pinky!', got %q", resp.Content)
	}
	if !resp.Done {
		t.Error("expected Done=true")
	}
}

func TestRemoteBrain_Think_WithAuth(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		resp := map[string]interface{}{"content": "ok", "done": true}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	brain := NewRemoteBrain(config.BrainConfig{
		RemoteURL:   server.URL,
		RemoteToken: "secret-token",
	})

	ctx := context.Background()
	_, err := brain.Think(ctx, &ThinkRequest{})
	if err != nil {
		t.Fatalf("Think() failed: %v", err)
	}

	if receivedAuth != "Bearer secret-token" {
		t.Errorf("expected auth header 'Bearer secret-token', got %q", receivedAuth)
	}
}

func TestRemoteBrain_Think_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{"error": "rate limit exceeded"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	brain := NewRemoteBrain(config.BrainConfig{
		RemoteURL: server.URL,
	})

	ctx := context.Background()
	_, err := brain.Think(ctx, &ThinkRequest{})
	if err == nil {
		t.Error("expected error for error response")
	}
}

func TestRemoteBrain_Remember(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/memory" {
			t.Errorf("expected path /v1/memory, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	brain := NewRemoteBrain(config.BrainConfig{
		RemoteURL: server.URL,
	})

	ctx := context.Background()
	err := brain.Remember(ctx, &Memory{Content: "Test memory"})
	if err != nil {
		t.Errorf("Remember() failed: %v", err)
	}
}

func TestRemoteBrain_Recall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/memory/search" {
			t.Errorf("expected path /v1/memory/search, got %s", r.URL.Path)
		}
		resp := map[string]interface{}{
			"memories": []map[string]interface{}{
				{"content": "Memory 1"},
				{"content": "Memory 2"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	brain := NewRemoteBrain(config.BrainConfig{
		RemoteURL: server.URL,
	})

	ctx := context.Background()
	memories, err := brain.Recall(ctx, "test", 10)
	if err != nil {
		t.Fatalf("Recall() failed: %v", err)
	}

	if len(memories) != 2 {
		t.Errorf("expected 2 memories, got %d", len(memories))
	}
}

func TestThinkRequest_Fields(t *testing.T) {
	req := &ThinkRequest{
		UserID:      "user123",
		MaxTokens:   100,
		Temperature: 0.7,
		Stream:      true,
	}

	if req.UserID != "user123" {
		t.Error("UserID not set")
	}
	if req.MaxTokens != 100 {
		t.Error("MaxTokens not set")
	}
	if req.Temperature != 0.7 {
		t.Error("Temperature not set")
	}
	if !req.Stream {
		t.Error("Stream not set")
	}
}

func TestThinkResponse_Fields(t *testing.T) {
	resp := &ThinkResponse{
		Content:   "Hello",
		Reasoning: "Thinking...",
		Done:      true,
		Usage: TokenUsage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	if resp.Content != "Hello" {
		t.Error("Content not set")
	}
	if resp.Usage.TotalTokens != 15 {
		t.Error("Usage not set correctly")
	}
}

func TestToolCall_Fields(t *testing.T) {
	tc := ToolCall{
		ID:     "call123",
		Tool:   "shell",
		Input:  map[string]any{"command": "ls"},
		Reason: "List files",
	}

	if tc.ID != "call123" {
		t.Error("ID not set")
	}
	if tc.Tool != "shell" {
		t.Error("Tool not set")
	}
	if tc.Input["command"] != "ls" {
		t.Error("Input not set")
	}
}

func TestToolResult_Fields(t *testing.T) {
	tr := ToolResult{
		ToolCallID: "call123",
		Success:    true,
		Output:     "file1.txt\nfile2.txt",
	}

	if tr.ToolCallID != "call123" {
		t.Error("ToolCallID not set")
	}
	if !tr.Success {
		t.Error("Success not set")
	}
}

func TestMemory_Fields(t *testing.T) {
	mem := Memory{
		ID:         "mem123",
		UserID:     "user456",
		Type:       MemoryEpisodic,
		Content:    "User said hello",
		Importance: 0.8,
		Source:     "telegram",
		Context:    map[string]string{"channel": "main"},
	}

	if mem.ID != "mem123" {
		t.Error("ID not set")
	}
	if mem.Type != MemoryEpisodic {
		t.Error("Type not set")
	}
	if mem.Importance != 0.8 {
		t.Error("Importance not set")
	}
}
