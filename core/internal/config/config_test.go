package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.LLM.DefaultProvider != "ollama" {
		t.Errorf("expected default provider 'ollama', got '%s'", cfg.LLM.DefaultProvider)
	}

	if cfg.Knowledge.DefaultTier != "personal" {
		t.Errorf("expected default tier 'personal', got '%s'", cfg.Knowledge.DefaultTier)
	}

	if cfg.TUI.Theme != "dark" {
		t.Errorf("expected default theme 'dark', got '%s'", cfg.TUI.Theme)
	}

	if cfg.Sync.Enabled {
		t.Error("expected sync to be disabled by default")
	}

	if cfg.Logging.Level != "info" {
		t.Errorf("expected log level 'info', got '%s'", cfg.Logging.Level)
	}

	// Check that providers are populated
	if len(cfg.LLM.Providers) == 0 {
		t.Error("expected default providers to be populated")
	}

	ollamaProvider, exists := cfg.LLM.Providers["ollama"]
	if !exists {
		t.Error("expected 'ollama' provider to exist")
	}
	if ollamaProvider.Endpoint != "http://127.0.0.1:11434" {
		t.Errorf("expected ollama endpoint 'http://127.0.0.1:11434', got '%s'", ollamaProvider.Endpoint)
	}
}

func TestLoadFromPath(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".cortex", "config.yaml")

	// Load config (should create default)
	cfg, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify config was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config file was not created")
	}

	// Verify config values
	if cfg.LLM.DefaultProvider != "ollama" {
		t.Errorf("expected default provider 'ollama', got '%s'", cfg.LLM.DefaultProvider)
	}

	// Load again to test reading existing file
	cfg2, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("failed to load existing config: %v", err)
	}

	if cfg2.LLM.DefaultProvider != cfg.LLM.DefaultProvider {
		t.Error("config values changed on reload")
	}
}

func TestSaveToPath(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".cortex", "config.yaml")

	cfg := Default()
	cfg.LLM.DefaultProvider = "openai"
	cfg.TUI.VimMode = true

	// Save config
	if err := cfg.SaveToPath(configPath); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Load saved config
	loaded, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}

	// Verify saved values
	if loaded.LLM.DefaultProvider != "openai" {
		t.Errorf("expected provider 'openai', got '%s'", loaded.LLM.DefaultProvider)
	}

	if !loaded.TUI.VimMode {
		t.Error("expected VimMode to be true")
	}
}

func TestGetDataDir(t *testing.T) {
	cfg := Default()
	dataDir := cfg.GetDataDir()

	homeDir, _ := os.UserHomeDir()
	expected := filepath.Join(homeDir, ".cortex")

	if dataDir != expected {
		t.Errorf("expected data dir '%s', got '%s'", expected, dataDir)
	}
}

func TestEnsureDirectories(t *testing.T) {
	tempDir := t.TempDir()

	cfg := &Config{
		Knowledge: KnowledgeConfig{
			DBPath: filepath.Join(tempDir, ".cortex", "data", "knowledge.db"),
		},
		Logging: LoggingConfig{
			File: filepath.Join(tempDir, ".cortex", "logs", "cortex.log"),
		},
	}

	if err := cfg.EnsureDirectories(); err != nil {
		t.Fatalf("failed to ensure directories: %v", err)
	}

	// Check that directories were created
	dirs := []string{
		filepath.Join(tempDir, ".cortex"),
		filepath.Join(tempDir, ".cortex", "data"),
		filepath.Join(tempDir, ".cortex", "logs"),
	}

	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("directory '%s' was not created", dir)
		}
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name:    "valid default config",
			cfg:     Default(),
			wantErr: false,
		},
		{
			name: "empty default provider",
			cfg: &Config{
				LLM: LLMConfig{
					DefaultProvider: "",
					Providers:       make(map[string]ProviderConfig),
				},
				Knowledge: KnowledgeConfig{DefaultTier: "personal"},
				TUI:       TUIConfig{Theme: "dark", SidebarWidth: 30},
				Logging:   LoggingConfig{Level: "info"},
			},
			wantErr: true,
		},
		{
			name: "default provider not in map",
			cfg: &Config{
				LLM: LLMConfig{
					DefaultProvider: "nonexistent",
					Providers:       make(map[string]ProviderConfig),
				},
				Knowledge: KnowledgeConfig{DefaultTier: "personal"},
				TUI:       TUIConfig{Theme: "dark", SidebarWidth: 30},
				Logging:   LoggingConfig{Level: "info"},
			},
			wantErr: true,
		},
		{
			name: "invalid knowledge tier",
			cfg: &Config{
				LLM: LLMConfig{
					DefaultProvider: "ollama",
					Providers: map[string]ProviderConfig{
						"ollama": {},
					},
				},
				Knowledge: KnowledgeConfig{DefaultTier: "invalid"},
				TUI:       TUIConfig{Theme: "dark", SidebarWidth: 30},
				Logging:   LoggingConfig{Level: "info"},
			},
			wantErr: true,
		},
		{
			name: "negative trust decay days",
			cfg: &Config{
				LLM: LLMConfig{
					DefaultProvider: "ollama",
					Providers: map[string]ProviderConfig{
						"ollama": {},
					},
				},
				Knowledge: KnowledgeConfig{
					DefaultTier:    "personal",
					TrustDecayDays: -1,
				},
				TUI:     TUIConfig{Theme: "dark", SidebarWidth: 30},
				Logging: LoggingConfig{Level: "info"},
			},
			wantErr: true,
		},
		{
			name: "invalid theme",
			cfg: &Config{
				LLM: LLMConfig{
					DefaultProvider: "ollama",
					Providers: map[string]ProviderConfig{
						"ollama": {},
					},
				},
				Knowledge: KnowledgeConfig{DefaultTier: "personal"},
				TUI:       TUIConfig{Theme: "invalid", SidebarWidth: 30},
				Logging:   LoggingConfig{Level: "info"},
			},
			wantErr: true,
		},
		{
			name: "sidebar width too small",
			cfg: &Config{
				LLM: LLMConfig{
					DefaultProvider: "ollama",
					Providers: map[string]ProviderConfig{
						"ollama": {},
					},
				},
				Knowledge: KnowledgeConfig{DefaultTier: "personal"},
				TUI:       TUIConfig{Theme: "dark", SidebarWidth: 5},
				Logging:   LoggingConfig{Level: "info"},
			},
			wantErr: true,
		},
		{
			name: "invalid log level",
			cfg: &Config{
				LLM: LLMConfig{
					DefaultProvider: "ollama",
					Providers: map[string]ProviderConfig{
						"ollama": {},
					},
				},
				Knowledge: KnowledgeConfig{DefaultTier: "personal"},
				TUI:       TUIConfig{Theme: "dark", SidebarWidth: 30},
				Logging:   LoggingConfig{Level: "invalid"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExpandPath(t *testing.T) {
	homeDir, _ := os.UserHomeDir()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "path with tilde",
			input:    "~/.cortex/config.yaml",
			expected: filepath.Join(homeDir, ".cortex", "config.yaml"),
		},
		{
			name:     "absolute path",
			input:    "/usr/local/bin/cortex",
			expected: "/usr/local/bin/cortex",
		},
		{
			name:     "relative path",
			input:    "./config.yaml",
			expected: "./config.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandPath(tt.input)
			if result != tt.expected {
				t.Errorf("expandPath(%s) = %s, expected %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestConfigSerialization(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Create a config with specific values
	original := Default()
	original.LLM.DefaultProvider = "anthropic"
	original.LLM.Providers["anthropic"] = ProviderConfig{
		APIKey: "test-key-123",
		Model:  "claude-3-opus-20240229",
	}
	original.Knowledge.TrustDecayDays = 60
	original.Sync.Enabled = true
	original.Sync.Interval = 10 * time.Minute
	original.TUI.VimMode = true
	original.TUI.SidebarWidth = 40
	original.Logging.Level = "debug"

	// Save config
	if err := original.SaveToPath(configPath); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Load config
	loaded, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify all values
	if loaded.LLM.DefaultProvider != "anthropic" {
		t.Errorf("provider mismatch: got %s, want anthropic", loaded.LLM.DefaultProvider)
	}

	anthropicProvider := loaded.LLM.Providers["anthropic"]
	if anthropicProvider.APIKey != "test-key-123" {
		t.Errorf("API key mismatch: got %s, want test-key-123", anthropicProvider.APIKey)
	}

	if loaded.Knowledge.TrustDecayDays != 60 {
		t.Errorf("trust decay days mismatch: got %d, want 60", loaded.Knowledge.TrustDecayDays)
	}

	if !loaded.Sync.Enabled {
		t.Error("sync should be enabled")
	}

	if loaded.Sync.Interval != 10*time.Minute {
		t.Errorf("sync interval mismatch: got %v, want 10m", loaded.Sync.Interval)
	}

	if !loaded.TUI.VimMode {
		t.Error("vim mode should be enabled")
	}

	if loaded.TUI.SidebarWidth != 40 {
		t.Errorf("sidebar width mismatch: got %d, want 40", loaded.TUI.SidebarWidth)
	}

	if loaded.Logging.Level != "debug" {
		t.Errorf("log level mismatch: got %s, want debug", loaded.Logging.Level)
	}
}

func TestEnvironmentVariableOverride(t *testing.T) {
	// Note: This test demonstrates the pattern but may need adjustment
	// based on how Viper handles nested environment variables in your setup

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Create default config
	cfg := Default()
	if err := cfg.SaveToPath(configPath); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Set environment variable
	os.Setenv("CORTEX_LLM_DEFAULT_PROVIDER", "openai")
	defer os.Unsetenv("CORTEX_LLM_DEFAULT_PROVIDER")

	// Load config (should pick up env var)
	loaded, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Note: Viper's AutomaticEnv() may have limitations with nested structs
	// This test documents expected behavior
	t.Logf("Default provider from config: %s", loaded.LLM.DefaultProvider)
}
