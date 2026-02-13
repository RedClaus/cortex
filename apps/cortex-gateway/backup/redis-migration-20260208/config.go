package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for Cortex-Gateway
type Config struct {
	Server      ServerConfig      `yaml:"server"`
	CortexBrain CortexBrainConfig `yaml:"cortexbrain"`
	Bridge      BridgeConfig      `yaml:"bridge"`
	Ollama      OllamaConfig      `yaml:"ollama"`
	Channels    ChannelsConfig    `yaml:"channels"`
	Inference   InferenceConfig   `yaml:"inference"`
	Logging     LoggingConfig     `yaml:"logging"`
	Swarm       SwarmConfig       `yaml:"swarm,omitempty"`
	HealthRing  HealthRingConfig  `yaml:"healthring,omitempty"`
}

// ServerConfig defines HTTP server settings
type ServerConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

// CortexBrainConfig defines CortexBrain connection settings
type CortexBrainConfig struct {
	URL        string `yaml:"url"`
	JWTSecret  string `yaml:"jwt_secret"`
	Timeout    string `yaml:"timeout"`
}

// BridgeConfig defines A2A Bridge connection settings
type BridgeConfig struct {
	URL string `yaml:"url"`
}

// GetTimeout returns the timeout as a time.Duration
func (c *CortexBrainConfig) GetTimeout() time.Duration {
	if c.Timeout == "" {
		return 30 * time.Second
	}
	d, err := time.ParseDuration(c.Timeout)
	if err != nil {
		return 30 * time.Second
	}
	return d
}

// OllamaConfig defines Ollama connection settings
type OllamaConfig struct {
	URL          string `yaml:"url"`
	DefaultModel string `yaml:"default_model"`
	Timeout      string `yaml:"timeout"`
}

// GetTimeout returns the timeout as a time.Duration
func (o *OllamaConfig) GetTimeout() time.Duration {
	if o.Timeout == "" {
		return 60 * time.Second
	}
	d, err := time.ParseDuration(o.Timeout)
	if err != nil {
		return 60 * time.Second
	}
	return d
}

// ChannelsConfig defines channel configurations
type ChannelsConfig struct {
	Telegram TelegramConfig `yaml:"telegram"`
	Discord  DiscordConfig  `yaml:"discord"`
	WebChat  WebChatConfig  `yaml:"webchat"`
}

// TelegramConfig defines Telegram channel settings
type TelegramConfig struct {
	Enabled bool   `yaml:"enabled"`
	Token   string `yaml:"token"`
}

// DiscordConfig defines Discord channel settings
type DiscordConfig struct {
	Enabled bool   `yaml:"enabled"`
	Token   string `yaml:"token"`
}

// WebChatConfig defines WebChat channel settings
type WebChatConfig struct {
	Enabled bool `yaml:"enabled"`
	Port    int  `yaml:"port"`
}

// EngineConfig defines an inference engine configuration
type EngineConfig struct {
	Name            string   `yaml:"name"`
	Type            string   `yaml:"type"`
	URL             string   `yaml:"url,omitempty"`
	APIKey          string   `yaml:"api_key,omitempty"`
	PreferredModels []string `yaml:"preferred_models,omitempty"`
	Models          []string `yaml:"models,omitempty"`
}

// LaneConfig defines an inference lane configuration
type LaneConfig struct {
	Name      string   `yaml:"name"`
	Engine    string   `yaml:"engine,omitempty"`     // new: reference to engine
	Provider  string   `yaml:"provider,omitempty"`   // old
	BaseURL   string   `yaml:"base_url,omitempty"`   // old
	APIKey    string   `yaml:"api_key,omitempty"`    // old, per lane
	Models    []string `yaml:"models,omitempty"`     // old
	Strategy  string   `yaml:"strategy,omitempty"`
}

// InferenceConfig defines inference configurations
type InferenceConfig struct {
	AutoDetect bool         `yaml:"auto_detect"`
	Engines    []EngineConfig `yaml:"engines,omitempty"`
	Lanes      []LaneConfig   `yaml:"lanes"`
	DefaultLane string         `yaml:"default_lane,omitempty"`
}

// LoggingConfig defines logging settings
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// SwarmConfig defines swarm agent discovery settings
type SwarmConfig struct {
	Agents    []AgentConfig     `yaml:"agents"`
	Discovery DiscoveryConfig   `yaml:"discovery"`
}

// AgentConfig defines an agent in the swarm
type AgentConfig struct {
	Name     string            `yaml:"name"`
	Host     string            `yaml:"host"`
	Services map[string]int    `yaml:"services,omitempty"`
}

// DiscoveryConfig defines discovery settings
type DiscoveryConfig struct {
	Method       string        `yaml:"method"`
	ScanInterval time.Duration `yaml:"scan_interval"`
	Subnet       string        `yaml:"subnet"`
}

// HealthRingConfig defines health ring settings
type HealthRingConfig struct {
	Enabled       bool               `yaml:"enabled"`
	CheckInterval time.Duration      `yaml:"check_interval"`
	Members       []HealthMemberConfig `yaml:"members"`
}

// HealthMemberConfig defines a health ring member
type HealthMemberConfig struct {
	Name   string            `yaml:"name"`
	Checks []HealthCheckConfig `yaml:"checks"`
}

// HealthCheckConfig defines a health check
type HealthCheckConfig struct {
	Type         string  `yaml:"type"`
	URL          string  `yaml:"url,omitempty"`
	Port         *int    `yaml:"port,omitempty"`
	ExpectStatus *int    `yaml:"expect_status,omitempty"`
}

// Load loads configuration from a YAML file with environment variable overrides
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Apply environment variable overrides
	cfg.applyEnvOverrides()

	return &cfg, nil
}

// applyEnvOverrides applies environment variable overrides to the config
func (c *Config) applyEnvOverrides() {
	if port := os.Getenv("GATEWAY_PORT"); port != "" {
		fmt.Sscanf(port, "%d", &c.Server.Port)
	}
	if url := os.Getenv("CORTEXBRAIN_URL"); url != "" {
		c.CortexBrain.URL = url
	}
	if url := os.Getenv("OLLAMA_URL"); url != "" {
		c.Ollama.URL = url
	}
	if token := os.Getenv("TELEGRAM_TOKEN"); token != "" {
		c.Channels.Telegram.Token = token
	}
	if token := os.Getenv("DISCORD_TOKEN"); token != "" {
		c.Channels.Discord.Token = token
	}
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		for i := range c.Inference.Lanes {
			if c.Inference.Lanes[i].Provider == "openai" {
				c.Inference.Lanes[i].APIKey = apiKey
			}
		}
		// Also for new engines
		for i := range c.Inference.Engines {
			if c.Inference.Engines[i].Type == "openai-compatible" || c.Inference.Engines[i].Type == "openai" {
				c.Inference.Engines[i].APIKey = apiKey
			}
		}
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}
	if c.CortexBrain.URL == "" {
		return fmt.Errorf("cortexbrain URL is required")
	}
	if c.Ollama.URL == "" {
		return fmt.Errorf("ollama URL is required")
	}
	if len(c.Inference.Lanes) == 0 {
		return fmt.Errorf("at least one inference lane is required")
	}
	return nil
}
