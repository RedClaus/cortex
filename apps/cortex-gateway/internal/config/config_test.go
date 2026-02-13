package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	yaml := []byte(`
server:
  port: 18800
  host: localhost
cortexbrain:
  url: http://localhost:18892
ollama:
  url: http://localhost:11434
inference:
  auto_detect: true
  lanes:
    - name: local
      provider: ollama
      base_url: http://localhost:11434
      models: [test-model]
  default_lane: local
`)
	f, _ := os.CreateTemp("", "config-*.yaml")
	f.Write(yaml)
	f.Close()
	defer os.Remove(f.Name())

	cfg, err := Load(f.Name())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Server.Port != 18800 {
		t.Errorf("Expected port 18800, got %d", cfg.Server.Port)
	}
	if cfg.Inference.DefaultLane != "local" {
		t.Errorf("Expected default_lane local, got %s", cfg.Inference.DefaultLane)
	}
}

func TestValidate(t *testing.T) {
	cfg := &Config{
		Server:      ServerConfig{Port: 18800, Host: "localhost"},
		CortexBrain: CortexBrainConfig{URL: "http://localhost:18892"},
		Ollama:      OllamaConfig{URL: "http://localhost:11434"},
		Inference:   InferenceConfig{Lanes: []LaneConfig{{Name: "local", Provider: "ollama", BaseURL: "http://localhost:11434", Models: []string{"test"}}}, DefaultLane: "local"},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate failed: %v", err)
	}
}

func TestValidateInvalidPort(t *testing.T) {
	cfg := &Config{Server: ServerConfig{Port: -1}}
	if err := cfg.Validate(); err == nil {
		t.Error("Expected validation error for invalid port")
	}
}
