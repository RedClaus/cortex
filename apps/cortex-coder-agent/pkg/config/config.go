// Package config provides the configuration system for Cortex Coder Agent
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds the complete application configuration
type Config struct {
	Agent       AgentConfig       `mapstructure:"agent"`
	CortexBrain CortexBrainConfig `mapstructure:"cortexbrain"`
	TUI         TUIConfig         `mapstructure:"tui"`
	Skills      SkillsConfig      `mapstructure:"skills"`
	Session     SessionConfig     `mapstructure:"session"`
}

// AgentConfig holds agent-specific configuration
type AgentConfig struct {
	Name string `mapstructure:"name"`
	Mode string `mapstructure:"mode"` // interactive, json, rpc
}

// CortexBrainConfig holds CortexBrain connection settings
type CortexBrainConfig struct {
	URL   string `mapstructure:"url"`
	WSURL string `mapstructure:"ws_url"`
	Token string `mapstructure:"token"`
}

// TUIConfig holds TUI-specific configuration
type TUIConfig struct {
	Theme           string `mapstructure:"theme"`
	ShowLineNumbers bool   `mapstructure:"show_line_numbers"`
	TabSize         int    `mapstructure:"tab_size"`
}

// SkillsConfig holds skill system configuration
type SkillsConfig struct {
	AutoLoad    bool     `mapstructure:"auto_load"`
	Directories []string `mapstructure:"directories"`
}

// SessionConfig holds session management configuration
type SessionConfig struct {
	AutoSave     bool   `mapstructure:"auto_save"`
	SaveInterval string `mapstructure:"save_interval"`
}

// DefaultConfig returns a new configuration with default values
func DefaultConfig() *Config {
	return &Config{
		Agent: AgentConfig{
			Name: "cortex-coder",
			Mode: "interactive",
		},
		CortexBrain: CortexBrainConfig{
			URL:   "http://192.168.1.186:18892",
			WSURL: "ws://192.168.1.186:18892/bus",
			Token: "",
		},
		TUI: TUIConfig{
			Theme:           "dracula",
			ShowLineNumbers: true,
			TabSize:         4,
		},
		Skills: SkillsConfig{
			AutoLoad: true,
			Directories: []string{
				"~/.config/cortex-coder/skills",
				"./.coder/skills",
			},
		},
		Session: SessionConfig{
			AutoSave:     true,
			SaveInterval: "30s",
		},
	}
}

// Load loads configuration from file, environment variables, and CLI flags
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Set up environment variable support
	v.SetEnvPrefix("CORTEX")
	v.AutomaticEnv()

	// Environment variable mapping
	v.BindEnv("cortexbrain.url", "CORTEXBRAIN_URL")
	v.BindEnv("cortexbrain.ws_url", "CORTEXBRAIN_WSURL")
	v.BindEnv("cortexbrain.token", "CORTEXBRAIN_TOKEN")
	v.BindEnv("agent.name", "CORTEX_AGENT_NAME")
	v.BindEnv("agent.mode", "CORTEX_AGENT_MODE")

	// Set up config file search paths
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("$HOME/.config/cortex-coder")
	}

	// Read config file (ignore if not found)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Unmarshal into struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Expand home directory in paths
	cfg = expandPaths(cfg)

	return &cfg, nil
}

// Save saves the configuration to the default config file
func (c *Config) Save() error {
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "cortex-coder")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configFile := filepath.Join(configDir, "config.yaml")
	return c.SaveToFile(configFile)
}

// SaveToFile saves the configuration to a specific file
func (c *Config) SaveToFile(path string) error {
	v := viper.New()
	v.Set("agent", c.Agent)
	v.Set("cortexbrain", c.CortexBrain)
	v.Set("tui", c.TUI)
	v.Set("skills", c.Skills)
	v.Set("session", c.Session)

	return v.WriteConfigAs(path)
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Agent.Mode != "interactive" && c.Agent.Mode != "json" && c.Agent.Mode != "rpc" {
		return fmt.Errorf("invalid agent mode: %s (must be interactive, json, or rpc)", c.Agent.Mode)
	}

	if c.CortexBrain.URL == "" {
		return fmt.Errorf("cortexbrain URL is required")
	}

	if c.TUI.Theme != "dracula" && c.TUI.Theme != "default" {
		return fmt.Errorf("invalid theme: %s (must be dracula or default)", c.TUI.Theme)
	}

	return nil
}

// GetConfigPath returns the default configuration file path
func GetConfigPath() string {
	home := os.Getenv("HOME")
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	return filepath.Join(home, ".config", "cortex-coder", "config.yaml")
}

// ConfigFileExists checks if the config file exists
func ConfigFileExists() bool {
	_, err := os.Stat(GetConfigPath())
	return err == nil
}

func setDefaults(v *viper.Viper) {
	defaults := DefaultConfig()
	v.SetDefault("agent.name", defaults.Agent.Name)
	v.SetDefault("agent.mode", defaults.Agent.Mode)
	v.SetDefault("cortexbrain.url", defaults.CortexBrain.URL)
	v.SetDefault("cortexbrain.ws_url", defaults.CortexBrain.WSURL)
	v.SetDefault("cortexbrain.token", defaults.CortexBrain.Token)
	v.SetDefault("tui.theme", defaults.TUI.Theme)
	v.SetDefault("tui.show_line_numbers", defaults.TUI.ShowLineNumbers)
	v.SetDefault("tui.tab_size", defaults.TUI.TabSize)
	v.SetDefault("skills.auto_load", defaults.Skills.AutoLoad)
	v.SetDefault("skills.directories", defaults.Skills.Directories)
	v.SetDefault("session.auto_save", defaults.Session.AutoSave)
	v.SetDefault("session.save_interval", defaults.Session.SaveInterval)
}

func expandPaths(cfg Config) Config {
	home := os.Getenv("HOME")
	if home == "" {
		home, _ = os.UserHomeDir()
	}

	// Expand skill directories
	for i, dir := range cfg.Skills.Directories {
		if len(dir) > 0 && dir[0] == '~' {
			cfg.Skills.Directories[i] = home + dir[1:]
		}
	}

	return cfg
}
