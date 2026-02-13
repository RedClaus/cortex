package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOllamaStreamingContextCancellation verifies that streaming goroutines exit cleanly on cancellation.
// This tests the fix in ollama.go lines 300-307 and 310-314 (context-aware channel sends).
func TestOllamaStreamingContextCancellation(t *testing.T) {
	// Create mock server that streams slowly
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Stream chunks with delays
		for i := 0; i < 10; i++ {
			chunk := ollamaChatResponse{
				Model: "test-model",
				Message: ollamaMessage{
					Role:    "assistant",
					Content: "token ",
				},
				Done: i == 9,
			}

			json.NewEncoder(w).Encode(chunk)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}

			// Delay to allow cancellation during streaming
			time.Sleep(50 * time.Millisecond)
		}
	}))
	defer server.Close()

	provider := NewOllamaProvider(&ProviderConfig{
		Endpoint: server.URL,
		Model:    "test-model",
	})

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	req := &ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "test"},
		},
	}

	// Start streaming in goroutine
	done := make(chan struct{})
	var err error

	go func() {
		_, err = provider.Chat(ctx, req)
		close(done)
	}()

	// Let it receive a few tokens
	time.Sleep(150 * time.Millisecond)

	// Cancel mid-stream
	cancel()

	// Wait for completion with timeout
	select {
	case <-done:
		// Should complete quickly after cancellation
		assert.Error(t, err, "Should return error on cancellation")
		assert.Contains(t, err.Error(), "context canceled", "Error should mention cancellation")
	case <-time.After(2 * time.Second):
		t.Fatal("Chat() did not return after context cancellation")
	}
}

// TestOllamaStreamingNoLeak verifies that streaming goroutines don't leak.
func TestOllamaStreamingNoLeak(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Stream a few chunks
		for i := 0; i < 5; i++ {
			chunk := ollamaChatResponse{
				Model: "test-model",
				Message: ollamaMessage{
					Role:    "assistant",
					Content: "token ",
				},
				Done: i == 4,
			}
			json.NewEncoder(w).Encode(chunk)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	provider := NewOllamaProvider(&ProviderConfig{
		Endpoint: server.URL,
		Model:    "test-model",
	})

	// Force GC and wait
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	baseline := runtime.NumGoroutine()
	t.Logf("Baseline goroutines: %d", baseline)

	// Create and abandon multiple streams
	const numStreams = 10
	var wg sync.WaitGroup

	for i := 0; i < numStreams; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			req := &ChatRequest{
				Messages: []Message{
					{Role: "user", Content: "test"},
				},
			}

			// Start request, let it cancel via timeout
			_, _ = provider.Chat(ctx, req)
		}()
	}

	wg.Wait()

	// Force GC
	runtime.GC()
	time.Sleep(200 * time.Millisecond)

	final := runtime.NumGoroutine()
	leaked := final - baseline

	t.Logf("Final goroutines: %d (leaked: %d)", final, leaked)

	// Allow some tolerance for background goroutines (HTTP client, test server, etc.)
	// httptest.Server creates background goroutines that may not exit immediately
	maxAcceptableLeak := 10
	assert.LessOrEqual(t, leaked, maxAcceptableLeak,
		"Should not leak more than %d goroutines (leaked %d)", maxAcceptableLeak, leaked)
}

// TestOllamaStreamingErrorHandling verifies error handling in streaming goroutine.
func TestOllamaStreamingErrorHandling(t *testing.T) {
	// Create mock server that returns malformed JSON mid-stream
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Valid chunk
		chunk := ollamaChatResponse{
			Model: "test-model",
			Message: ollamaMessage{
				Role:    "assistant",
				Content: "token",
			},
			Done: false,
		}
		json.NewEncoder(w).Encode(chunk)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		// Malformed JSON
		w.Write([]byte("{invalid json\n"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	defer server.Close()

	provider := NewOllamaProvider(&ProviderConfig{
		Endpoint: server.URL,
		Model:    "test-model",
	})

	ctx := context.Background()
	req := &ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "test"},
		},
	}

	_, err := provider.Chat(ctx, req)
	assert.Error(t, err, "Should return error for malformed JSON")
}

// TestOllamaStreamingWithTools verifies tool streaming handles cancellation.
func TestOllamaStreamingWithToolsContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Stream chunks with tool calls
		for i := 0; i < 10; i++ {
			chunk := ollamaChatResponse{
				Model: "test-model",
				Message: ollamaMessage{
					Role:    "assistant",
					Content: "thinking ",
				},
				Done: i == 9,
			}
			json.NewEncoder(w).Encode(chunk)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			time.Sleep(50 * time.Millisecond)
		}
	}))
	defer server.Close()

	provider := NewOllamaProvider(&ProviderConfig{
		Endpoint: server.URL,
		Model:    "test-model",
	})

	ctx, cancel := context.WithCancel(context.Background())

	tools := []OllamaToolDef{
		{
			Type: "function",
			Function: OllamaFunctionDef{
				Name:        "test_tool",
				Description: "A test tool",
			},
		},
	}

	req := &ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "test"},
		},
	}

	done := make(chan struct{})
	var err error

	go func() {
		_, err = provider.ChatWithTools(ctx, req, tools)
		close(done)
	}()

	// Let it stream a bit
	time.Sleep(150 * time.Millisecond)

	// Cancel
	cancel()

	// Should complete quickly
	select {
	case <-done:
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	case <-time.After(2 * time.Second):
		t.Fatal("ChatWithTools() did not return after cancellation")
	}
}

// TestOllamaFirstTokenTimeout verifies first token timeout detection.
func TestOllamaFirstTokenTimeout(t *testing.T) {
	// Create server that sends headers but no data
	serverDone := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(serverDone)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Flush headers but never send data - simulate stalled stream
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		// Block until test completes
		<-r.Context().Done()
	}))
	defer server.Close()

	provider := NewOllamaProvider(&ProviderConfig{
		Endpoint: server.URL,
		Model:    "test-model",
	}, WithFirstTokenTimeout(200*time.Millisecond))

	ctx := context.Background()
	req := &ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "test"},
		},
	}

	start := time.Now()
	_, err := provider.Chat(ctx, req)
	duration := time.Since(start)

	// Wait for server handler to exit
	<-serverDone

	assert.Error(t, err)
	// Can be either timeout or empty response depending on timing
	assert.True(t,
		strings.Contains(err.Error(), "timeout waiting for first token") ||
		strings.Contains(err.Error(), "empty response"),
		"Error should mention timeout or empty response, got: %v", err)

	// Should timeout quickly (within 2 seconds)
	assert.Less(t, duration, 2*time.Second, "Should timeout quickly")
}

// TestOllamaStreamIdleTimeout verifies idle timeout between tokens.
func TestOllamaStreamIdleTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Send first token
		chunk := ollamaChatResponse{
			Model: "test-model",
			Message: ollamaMessage{
				Role:    "assistant",
				Content: "first ",
			},
			Done: false,
		}
		json.NewEncoder(w).Encode(chunk)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		// Stall - don't send next token
		time.Sleep(10 * time.Second)
	}))
	defer server.Close()

	provider := NewOllamaProvider(&ProviderConfig{
		Endpoint: server.URL,
		Model:    "test-model",
	}, WithStreamIdleTimeout(100*time.Millisecond))

	ctx := context.Background()
	req := &ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "test"},
		},
	}

	start := time.Now()
	resp, err := provider.Chat(ctx, req)
	duration := time.Since(start)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stream idle timeout")

	// Should have received first token
	assert.Nil(t, resp)

	// Should timeout around 100ms after first token
	assert.Less(t, duration, 1*time.Second, "Should timeout quickly after idle")
}

// TestOllamaStreamingNormalCompletion verifies normal stream completion.
func TestOllamaStreamingNormalCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		tokens := []string{"Hello", " ", "world", "!"}
		for i, token := range tokens {
			chunk := ollamaChatResponse{
				Model: "test-model",
				Message: ollamaMessage{
					Role:    "assistant",
					Content: token,
				},
				Done:            i == len(tokens)-1,
				PromptEvalCount: 10,
				EvalCount:       4,
			}
			json.NewEncoder(w).Encode(chunk)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	provider := NewOllamaProvider(&ProviderConfig{
		Endpoint: server.URL,
		Model:    "test-model",
	})

	ctx := context.Background()
	req := &ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "test"},
		},
	}

	resp, err := provider.Chat(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, "Hello world!", resp.Content)
	assert.Equal(t, "test-model", resp.Model)
	assert.Equal(t, 10, resp.PromptTokens)
	assert.Equal(t, 4, resp.CompletionTokens)
	assert.Equal(t, 14, resp.TokensUsed)
}

// TestOllamaStreamingMaxResponseSize verifies size limit protection.
func TestOllamaStreamingMaxResponseSize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Send chunks that exceed MaxStreamedResponseSize
		// MaxStreamedResponseSize is defined in provider.go
		largeContent := strings.Repeat("x", 1024*1024) // 1MB chunks
		for i := 0; i < 200; i++ { // 200MB total
			chunk := ollamaChatResponse{
				Model: "test-model",
				Message: ollamaMessage{
					Role:    "assistant",
					Content: largeContent,
				},
				Done: false,
			}
			if err := json.NewEncoder(w).Encode(chunk); err != nil {
				return
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	provider := NewOllamaProvider(&ProviderConfig{
		Endpoint: server.URL,
		Model:    "test-model",
	})

	ctx := context.Background()
	req := &ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "test"},
		},
	}

	_, err := provider.Chat(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "response size exceeded limit")
}

// BenchmarkOllamaStreamingThroughput measures streaming performance.
func BenchmarkOllamaStreamingThroughput(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Stream 100 tokens
		for i := 0; i < 100; i++ {
			chunk := ollamaChatResponse{
				Model: "test-model",
				Message: ollamaMessage{
					Role:    "assistant",
					Content: "token ",
				},
				Done: i == 99,
			}
			json.NewEncoder(w).Encode(chunk)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	provider := NewOllamaProvider(&ProviderConfig{
		Endpoint: server.URL,
		Model:    "test-model",
	})

	req := &ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "test"},
		},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		_, err := provider.Chat(ctx, req)
		if err != nil {
			b.Fatalf("Chat error: %v", err)
		}
	}
}

// BenchmarkOllamaStreamingCancellation measures cancellation overhead.
func BenchmarkOllamaStreamingCancellation(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Stream slowly to allow cancellation
		for i := 0; i < 100; i++ {
			chunk := ollamaChatResponse{
				Model: "test-model",
				Message: ollamaMessage{
					Role:    "assistant",
					Content: "token ",
				},
				Done: i == 99,
			}
			json.NewEncoder(w).Encode(chunk)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer server.Close()

	provider := NewOllamaProvider(&ProviderConfig{
		Endpoint: server.URL,
		Model:    "test-model",
	})

	req := &ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "test"},
		},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan struct{})
		go func() {
			provider.Chat(ctx, req)
			close(done)
		}()

		// Cancel after brief delay
		time.Sleep(50 * time.Millisecond)
		cancel()

		// Wait for completion
		<-done
	}
}

// TestOllamaHandleStreamingResponse unit tests the internal streaming handler.
func TestOllamaHandleStreamingResponse(t *testing.T) {
	provider := NewOllamaProvider(&ProviderConfig{
		Model: "test-model",
	})

	t.Run("empty_response", func(t *testing.T) {
		body := io.NopCloser(bytes.NewReader([]byte{}))
		_, err := provider.handleStreamingResponse(context.Background(), body, time.Now())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty response")
	})

	t.Run("valid_stream", func(t *testing.T) {
		chunks := []ollamaChatResponse{
			{
				Model:   "test-model",
				Message: ollamaMessage{Role: "assistant", Content: "Hello"},
				Done:    false,
			},
			{
				Model:           "test-model",
				Message:         ollamaMessage{Role: "assistant", Content: " world"},
				Done:            true,
				PromptEvalCount: 5,
				EvalCount:       2,
			},
		}

		var buf bytes.Buffer
		for _, chunk := range chunks {
			json.NewEncoder(&buf).Encode(chunk)
		}

		body := io.NopCloser(&buf)
		resp, err := provider.handleStreamingResponse(context.Background(), body, time.Now())
		require.NoError(t, err)
		assert.Equal(t, "Hello world", resp.Content)
		assert.Equal(t, 5, resp.PromptTokens)
		assert.Equal(t, 2, resp.CompletionTokens)
	})

	t.Run("context_cancelled", func(t *testing.T) {
		// Create a body that would block indefinitely
		pr, pw := io.Pipe()
		defer pw.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		body := io.NopCloser(pr)
		_, err := provider.handleStreamingResponse(ctx, body, time.Now())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	})
}

// TestOllamaTimeoutConfigOptions verifies timeout configuration options.
func TestOllamaTimeoutConfigOptions(t *testing.T) {
	t.Run("default_config", func(t *testing.T) {
		cfg := DefaultTimeoutConfig()
		assert.Equal(t, 30*time.Second, cfg.ConnectionTimeout)
		assert.Equal(t, 120*time.Second, cfg.FirstTokenTimeout)
		assert.Equal(t, 30*time.Second, cfg.StreamIdleTimeout)
	})

	t.Run("remote_config", func(t *testing.T) {
		cfg := RemoteTimeoutConfig()
		assert.Equal(t, 60*time.Second, cfg.ConnectionTimeout)
		assert.Equal(t, 300*time.Second, cfg.FirstTokenTimeout)
		assert.Equal(t, 60*time.Second, cfg.StreamIdleTimeout)
	})

	t.Run("custom_config", func(t *testing.T) {
		custom := TimeoutConfig{
			ConnectionTimeout: 10 * time.Second,
			FirstTokenTimeout: 20 * time.Second,
			StreamIdleTimeout: 5 * time.Second,
		}

		provider := NewOllamaProvider(&ProviderConfig{
			Endpoint: "http://localhost:11434",
			Model:    "test",
		}, WithTimeoutConfig(custom))

		assert.Equal(t, custom, provider.timeoutConfig)
	})

	t.Run("individual_options", func(t *testing.T) {
		provider := NewOllamaProvider(&ProviderConfig{
			Endpoint: "http://localhost:11434",
			Model:    "test",
		},
			WithConnectionTimeout(15*time.Second),
			WithFirstTokenTimeout(45*time.Second),
			WithStreamIdleTimeout(10*time.Second),
		)

		assert.Equal(t, 15*time.Second, provider.timeoutConfig.ConnectionTimeout)
		assert.Equal(t, 45*time.Second, provider.timeoutConfig.FirstTokenTimeout)
		assert.Equal(t, 10*time.Second, provider.timeoutConfig.StreamIdleTimeout)
	})
}

// TestIsRemoteEndpoint verifies remote endpoint detection.
func TestIsRemoteEndpoint(t *testing.T) {
	tests := []struct {
		endpoint string
		want     bool
	}{
		{"http://localhost:11434", false},
		{"http://127.0.0.1:11434", false},
		{"http://[::1]:11434", false},
		{"http://host.docker.internal:11434", false},
		{"http://docker.for.mac.localhost:11434", false},
		{"http://192.168.1.100:11434", true},
		{"http://example.com:11434", true},
		{"https://api.ollama.ai", true},
		{"invalid-url", true}, // Invalid URLs without scheme are considered remote
	}

	for _, tt := range tests {
		t.Run(tt.endpoint, func(t *testing.T) {
			got := isRemoteEndpoint(tt.endpoint)
			assert.Equal(t, tt.want, got)
		})
	}
}
