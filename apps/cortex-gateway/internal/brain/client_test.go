package brain

import (
	"testing"
	"github.com/cortexhub/cortex-gateway/internal/config"
)

func TestClientHealth(t *testing.T) {
	client := NewClient(&config.CortexBrainConfig{
		URL:       "http://localhost:18892",
		JWTSecret: "test",
	})
	resp, err := client.Health()
	if err != nil {
		t.Errorf("Health failed: %v", err)
	}
	if resp == nil {
		t.Errorf("No response")
	}
}
