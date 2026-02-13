//go:build integration
// +build integration

package llm_test

import (
	"context"
	"testing"
	"time"

	"github.com/normanking/cortex/internal/config"
	"github.com/normanking/cortex/internal/llm"
)

// TestOllamaIntegration tests the Ollama provider against a real running instance.
// Run with: go test -tags=integration -v ./internal/llm/
func TestOllamaIntegration(t *testing.T) {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		t.Skipf("Config error: %v", err)
	}

	t.Logf("Default provider: %s", cfg.LLM.DefaultProvider)
	t.Logf("Model: %s", cfg.LLM.Providers["ollama"].Model)

	// Create provider
	provider, err := llm.NewProvider(cfg)
	if err != nil {
		t.Skipf("Provider error: %v", err)
	}

	t.Logf("Provider name: %s", provider.Name())
	t.Logf("Provider available: %v", provider.Available())

	if !provider.Available() {
		t.Skip("Ollama not available")
	}

	// Test simple chat
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req := &llm.ChatRequest{
		Model: cfg.LLM.Providers["ollama"].Model,
		Messages: []llm.Message{
			{Role: "user", Content: "What is 2+2? Reply with just the number."},
		},
		MaxTokens:   50,
		Temperature: 0.1,
	}

	t.Log("Sending test request...")
	resp, err := provider.Chat(ctx, req)
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}

	t.Logf("✅ LLM Response: %s", resp.Content)
	t.Logf("Duration: %v", resp.Duration)
	t.Logf("Tokens used: %d", resp.TokensUsed)

	// Basic validation
	if resp.Content == "" {
		t.Error("Expected non-empty response")
	}
}
