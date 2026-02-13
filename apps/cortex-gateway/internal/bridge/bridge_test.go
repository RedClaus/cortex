package bridge

import (
	"testing"

	"github.com/cortexhub/cortex-gateway/internal/config"
)

func TestNewClient(t *testing.T) {
	cfg := &config.BridgeConfig{URL: "http://localhost:8080"}
	client := NewClient(nil, cfg)
	if client == nil {
		t.Fatal("NewClient returned nil")
	}
	if client.baseURL != "http://localhost:8080" {
		t.Errorf("expected baseURL %s, got %s", "http://localhost:8080", client.baseURL)
	}
}
