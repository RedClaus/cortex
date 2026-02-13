// Package config tests
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg == nil {
		t.Fatal("Default() returned nil")
	}

	// Check version
	if cfg.Version != 1 {
		t.Errorf("expected Version=1, got %d", cfg.Version)
	}

	// Check brain defaults
	if cfg.Brain.Mode != "embedded" {
		t.Errorf("expected Brain.Mode='embedded', got %q", cfg.Brain.Mode)
	}

	// Check server defaults
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("expected Server.Host='127.0.0.1', got %q", cfg.Server.Host)
	}
	if cfg.Server.Port != 18800 {
		t.Errorf("expected Server.Port=18800, got %d", cfg.Server.Port)
	}
	if cfg.Server.WebUIPort != 18801 {
		t.Errorf("expected Server.WebUIPort=18801, got %d", cfg.Server.WebUIPort)
	}

	// Check permissions defaults
	if cfg.Permissions.DefaultTier != "some" {
		t.Errorf("expected Permissions.DefaultTier='some', got %q", cfg.Permissions.DefaultTier)
	}

	// Check persona defaults
	if cfg.Persona.Default != "professional" {
		t.Errorf("expected Persona.Default='professional', got %q", cfg.Persona.Default)
	}

	// Check logging defaults
	if cfg.Logging.Level != "info" {
		t.Errorf("expected Logging.Level='info', got %q", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "text" {
		t.Errorf("expected Logging.Format='text', got %q", cfg.Logging.Format)
	}

	// Check channels are disabled by default
	if cfg.Channels.Telegram.Enabled {
		t.Error("expected Telegram to be disabled by default")
	}
	if cfg.Channels.Discord.Enabled {
		t.Error("expected Discord to be disabled by default")
	}
	if cfg.Channels.Slack.Enabled {
		t.Error("expected Slack to be disabled by default")
	}

	// Check inference defaults
	if cfg.Inference.DefaultLane != "fast" {
		t.Errorf("expected Inference.DefaultLane='fast', got %q", cfg.Inference.DefaultLane)
	}
	if _, ok := cfg.Inference.Lanes["fast"]; !ok {
		t.Error("expected 'fast' lane to exist")
	}
}

func TestLoadSave(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "pinky-config-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfgPath := filepath.Join(tmpDir, "config.yaml")

	// Create and save a config
	cfg := Default()
	cfg.Brain.Mode = "remote"
	cfg.Brain.RemoteURL = "http://localhost:8080"
	cfg.Server.Port = 9999
	cfg.Permissions.DefaultTier = "restricted"
	cfg.Logging.Level = "debug"

	if err := cfg.Save(cfgPath); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}

	// Load the config back
	loaded, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Verify loaded values
	if loaded.Brain.Mode != "remote" {
		t.Errorf("expected Brain.Mode='remote', got %q", loaded.Brain.Mode)
	}
	if loaded.Brain.RemoteURL != "http://localhost:8080" {
		t.Errorf("expected Brain.RemoteURL='http://localhost:8080', got %q", loaded.Brain.RemoteURL)
	}
	if loaded.Server.Port != 9999 {
		t.Errorf("expected Server.Port=9999, got %d", loaded.Server.Port)
	}
	if loaded.Permissions.DefaultTier != "restricted" {
		t.Errorf("expected Permissions.DefaultTier='restricted', got %q", loaded.Permissions.DefaultTier)
	}
	if loaded.Logging.Level != "debug" {
		t.Errorf("expected Logging.Level='debug', got %q", loaded.Logging.Level)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error when loading non-existent file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	// Create a temporary file with invalid YAML
	tmpDir, err := os.MkdirTemp("", "pinky-config-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfgPath := filepath.Join(tmpDir, "bad-config.yaml")
	if err := os.WriteFile(cfgPath, []byte("invalid: yaml: content: ["), 0644); err != nil {
		t.Fatalf("failed to write bad config: %v", err)
	}

	_, err = Load(cfgPath)
	if err == nil {
		t.Error("expected error when loading invalid YAML")
	}
}

func TestSave_CreatesDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pinky-config-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Path with nested directory that doesn't exist
	cfgPath := filepath.Join(tmpDir, "nested", "subdir", "config.yaml")

	cfg := Default()
	if err := cfg.Save(cfgPath); err != nil {
		t.Fatalf("Save() failed to create nested directories: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Fatal("config file was not created in nested directory")
	}
}

func TestChannelConfigs(t *testing.T) {
	cfg := Default()

	// Telegram
	cfg.Channels.Telegram.Enabled = true
	cfg.Channels.Telegram.Token = "test-telegram-token"

	// Discord
	cfg.Channels.Discord.Enabled = true
	cfg.Channels.Discord.Token = "test-discord-token"

	// Slack
	cfg.Channels.Slack.Enabled = true
	cfg.Channels.Slack.Token = "test-slack-token"
	cfg.Channels.Slack.AppToken = "test-slack-app-token"

	// Verify
	if !cfg.Channels.Telegram.Enabled {
		t.Error("Telegram should be enabled")
	}
	if cfg.Channels.Telegram.Token != "test-telegram-token" {
		t.Error("Telegram token not set correctly")
	}

	if !cfg.Channels.Discord.Enabled {
		t.Error("Discord should be enabled")
	}
	if cfg.Channels.Discord.Token != "test-discord-token" {
		t.Error("Discord token not set correctly")
	}

	if !cfg.Channels.Slack.Enabled {
		t.Error("Slack should be enabled")
	}
	if cfg.Channels.Slack.Token != "test-slack-token" {
		t.Error("Slack token not set correctly")
	}
	if cfg.Channels.Slack.AppToken != "test-slack-app-token" {
		t.Error("Slack app token not set correctly")
	}
}

func TestInferenceConfig(t *testing.T) {
	cfg := Default()

	// Add custom lanes
	cfg.Inference.Lanes["smart"] = Lane{
		Engine: "ollama",
		Model:  "llama3:70b",
		URL:    "http://custom:11434",
	}

	if lane, ok := cfg.Inference.Lanes["smart"]; !ok {
		t.Error("expected 'smart' lane to exist")
	} else {
		if lane.Engine != "ollama" {
			t.Errorf("expected lane.Engine='ollama', got %q", lane.Engine)
		}
		if lane.Model != "llama3:70b" {
			t.Errorf("expected lane.Model='llama3:70b', got %q", lane.Model)
		}
		if lane.URL != "http://custom:11434" {
			t.Errorf("expected lane.URL='http://custom:11434', got %q", lane.URL)
		}
	}
}
