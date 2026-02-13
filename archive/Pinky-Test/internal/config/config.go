// Package config handles Pinky configuration loading and management
package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds all Pinky configuration
type Config struct {
	Version     int              `yaml:"version"`
	Brain       BrainConfig      `yaml:"brain"`
	Inference   InferenceConfig  `yaml:"inference"`
	Server      ServerConfig     `yaml:"server"`
	Channels    ChannelsConfig   `yaml:"channels"`
	Permissions PermissionConfig `yaml:"permissions"`
	Persona     PersonaConfig    `yaml:"persona"`
	Logging     LoggingConfig    `yaml:"logging"`
}

// BrainConfig configures the cognitive engine
type BrainConfig struct {
	Mode       string `yaml:"mode"` // "embedded" or "remote"
	RemoteURL  string `yaml:"remote_url"`
	RemoteToken string `yaml:"remote_token"`
}

// InferenceConfig configures LLM inference (for embedded mode)
type InferenceConfig struct {
	DefaultLane string          `yaml:"default_lane"`
	AutoLLM     bool            `yaml:"autollm"`      // Enable automatic lane selection based on task complexity
	Lanes       map[string]Lane `yaml:"lanes"`
}

// Lane defines an inference lane
// Supported engines: ollama, openai, anthropic, groq, mlx
type Lane struct {
	Engine string `yaml:"engine"`
	Model  string `yaml:"model"`
	URL    string `yaml:"url,omitempty"`
	APIKey string `yaml:"api_key,omitempty"` // For cloud providers, can use env var like ${OPENAI_API_KEY}
}

// ServerConfig configures the HTTP server
type ServerConfig struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	WebUIPort int    `yaml:"webui_port"`
}

// ChannelsConfig configures messaging channels
type ChannelsConfig struct {
	Telegram TelegramConfig `yaml:"telegram"`
	Discord  DiscordConfig  `yaml:"discord"`
	Slack    SlackConfig    `yaml:"slack"`
}

// TelegramConfig for Telegram bot
type TelegramConfig struct {
	Enabled bool   `yaml:"enabled"`
	Token   string `yaml:"token"`
}

// DiscordConfig for Discord bot
type DiscordConfig struct {
	Enabled bool   `yaml:"enabled"`
	Token   string `yaml:"token"`
}

// SlackConfig for Slack bot
type SlackConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Token    string `yaml:"token"`
	AppToken string `yaml:"app_token"`
}

// PermissionConfig configures the permission system
type PermissionConfig struct {
	DefaultTier string `yaml:"default_tier"` // "unrestricted", "some", "restricted"
}

// PersonaConfig configures personality
type PersonaConfig struct {
	Default string `yaml:"default"`
}

// LoggingConfig configures logging
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	File   string `yaml:"file"`
}

// Default returns default configuration
func Default() *Config {
	return &Config{
		Version: 1,
		Brain: BrainConfig{
			Mode: "embedded",
		},
		Inference: InferenceConfig{
			DefaultLane: "fast",
			Lanes: map[string]Lane{
				"fast": {Engine: "ollama", Model: "llama3:8b"},
			},
		},
		Server: ServerConfig{
			Host:      "127.0.0.1",
			Port:      18800,
			WebUIPort: 18801,
		},
		Channels: ChannelsConfig{
			Telegram: TelegramConfig{Enabled: false},
			Discord:  DiscordConfig{Enabled: false},
			Slack:    SlackConfig{Enabled: false},
		},
		Permissions: PermissionConfig{
			DefaultTier: "some",
		},
		Persona: PersonaConfig{
			Default: "professional",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
	}
}

// Load reads configuration from file
func Load(path string) (*Config, error) {
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".pinky", "config.yaml")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Save writes configuration to file
func (c *Config) Save(path string) error {
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".pinky", "config.yaml")
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
