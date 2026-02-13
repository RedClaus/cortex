package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "cortex-coder", cfg.Agent.Name)
	assert.Equal(t, "interactive", cfg.Agent.Mode)
	assert.Equal(t, "http://192.168.1.186:18892", cfg.CortexBrain.URL)
	assert.Equal(t, "ws://192.168.1.186:18892/bus", cfg.CortexBrain.WSURL)
	assert.Equal(t, "", cfg.CortexBrain.Token)
	assert.Equal(t, "dracula", cfg.TUI.Theme)
	assert.True(t, cfg.TUI.ShowLineNumbers)
	assert.Equal(t, 4, cfg.TUI.TabSize)
	assert.True(t, cfg.Skills.AutoLoad)
	assert.Contains(t, cfg.Skills.Directories, "~/.config/cortex-coder/skills")
	assert.True(t, cfg.Session.AutoSave)
	assert.Equal(t, "30s", cfg.Session.SaveInterval)
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			cfg: Config{
				Agent: AgentConfig{Name: "test", Mode: "interactive"},
				CortexBrain: CortexBrainConfig{URL: "http://localhost:18892"},
				TUI: TUIConfig{Theme: "dracula"},
			},
			wantErr: false,
		},
		{
			name: "invalid mode",
			cfg: Config{
				Agent: AgentConfig{Name: "test", Mode: "invalid"},
				CortexBrain: CortexBrainConfig{URL: "http://localhost:18892"},
			},
			wantErr: true,
			errMsg:  "invalid agent mode",
		},
		{
			name: "missing URL",
			cfg: Config{
				Agent: AgentConfig{Name: "test", Mode: "interactive"},
				CortexBrain: CortexBrainConfig{URL: ""},
			},
			wantErr: true,
			errMsg:  "cortexbrain URL is required",
		},
		{
			name: "invalid theme",
			cfg: Config{
				Agent: AgentConfig{Name: "test", Mode: "interactive"},
				CortexBrain: CortexBrainConfig{URL: "http://localhost:18892"},
				TUI: TUIConfig{Theme: "invalid"},
			},
			wantErr: true,
			errMsg:  "invalid theme",
		},
		{
			name: "json mode valid",
			cfg: Config{
				Agent: AgentConfig{Name: "test", Mode: "json"},
				CortexBrain: CortexBrainConfig{URL: "http://localhost:18892"},
				TUI: TUIConfig{Theme: "dracula"},
			},
			wantErr: false,
		},
		{
			name: "rpc mode valid",
			cfg: Config{
				Agent: AgentConfig{Name: "test", Mode: "rpc"},
				CortexBrain: CortexBrainConfig{URL: "http://localhost:18892"},
				TUI: TUIConfig{Theme: "default"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
agent:
  name: "test-agent"
  mode: "json"
cortexbrain:
  url: "http://test.local:18892"
  ws_url: "ws://test.local:18892/bus"
  token: "test-token"
tui:
  theme: "default"
  show_line_numbers: false
  tab_size: 2
skills:
  auto_load: false
  directories:
    - "/custom/skills"
session:
  auto_save: false
  save_interval: "60s"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := Load(configPath)
	require.NoError(t, err)

	assert.Equal(t, "test-agent", cfg.Agent.Name)
	assert.Equal(t, "json", cfg.Agent.Mode)
	assert.Equal(t, "http://test.local:18892", cfg.CortexBrain.URL)
	assert.Equal(t, "ws://test.local:18892/bus", cfg.CortexBrain.WSURL)
	assert.Equal(t, "test-token", cfg.CortexBrain.Token)
	assert.Equal(t, "default", cfg.TUI.Theme)
	assert.False(t, cfg.TUI.ShowLineNumbers)
	assert.Equal(t, 2, cfg.TUI.TabSize)
	assert.False(t, cfg.Skills.AutoLoad)
	assert.Contains(t, cfg.Skills.Directories, "/custom/skills")
	assert.False(t, cfg.Session.AutoSave)
	assert.Equal(t, "60s", cfg.Session.SaveInterval)
}

func TestLoadWithEnvVars(t *testing.T) {
	// Set environment variables
	os.Setenv("CORTEXBRAIN_URL", "http://env-test.local:18892")
	os.Setenv("CORTEXBRAIN_TOKEN", "env-token")
	defer func() {
		os.Unsetenv("CORTEXBRAIN_URL")
		os.Unsetenv("CORTEXBRAIN_TOKEN")
	}()

	// Create a minimal config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
agent:
  name: "env-test"
cortexbrain:
  url: "http://default.local:18892"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := Load(configPath)
	require.NoError(t, err)

	// Environment variables should override config file
	assert.Equal(t, "http://env-test.local:18892", cfg.CortexBrain.URL)
	assert.Equal(t, "env-token", cfg.CortexBrain.Token)
}

func TestSaveAndLoad(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agent.Name = "saved-config"
	cfg.CortexBrain.Token = "secret-token"

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "saved-config.yaml")

	err := cfg.SaveToFile(configPath)
	require.NoError(t, err)

	// Load the saved config
	loadedCfg, err := Load(configPath)
	require.NoError(t, err)

	assert.Equal(t, "saved-config", loadedCfg.Agent.Name)
	assert.Equal(t, "secret-token", loadedCfg.CortexBrain.Token)
}

func TestExpandPaths(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	cfg := DefaultConfig()
	cfg.Skills.Directories = []string{
		"~/.config/cortex-coder/skills",
		"/absolute/path",
	}

	expanded := expandPaths(*cfg)

	assert.Equal(t, home+"/.config/cortex-coder/skills", expanded.Skills.Directories[0])
	assert.Equal(t, "/absolute/path", expanded.Skills.Directories[1])
}

func TestGetConfigPath(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	expected := filepath.Join(home, ".config", "cortex-coder", "config.yaml")
	assert.Equal(t, expected, GetConfigPath())
}

func TestConfigFileExists(t *testing.T) {
	// Test with non-existent file
	assert.False(t, ConfigFileExists())
}
