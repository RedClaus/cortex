package inference

import (
	"context"
	"testing"

	"github.com/cortexhub/cortex-gateway/internal/config"
)

func TestNewRouter(t *testing.T) {
	cfg := &config.Config{
		Inference: config.InferenceConfig{
			AutoDetect: false,
			Lanes: []config.LaneConfig{
				{Name: "local", Provider: "ollama", BaseURL: "http://localhost:11434", Models: []string{"test"}},
			},
			DefaultLane: "local",
		},
		Ollama: config.OllamaConfig{URL: "http://localhost:11434"},
	}
	router, err := NewRouter(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewRouter failed: %v", err)
	}
	if router == nil {
		t.Fatal("Expected non-nil router")
	}
}
