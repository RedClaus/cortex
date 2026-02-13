// CR-005: Vision Configuration
// Part of the Visual Cortex implementation for Cortex.
package vision

import "time"

// Config holds vision system configuration.
// Local-First: All URLs point to local services (Ollama).
type Config struct {
	// Enabled controls whether vision analysis is available
	Enabled bool `json:"enabled" mapstructure:"enabled"`

	// OllamaURL is the base URL for Ollama API
	// Default: http://127.0.0.1:11434
	OllamaURL string `json:"ollama_url" mapstructure:"ollama_url"`

	// FastModel is the model for Fast Lane (quick classification)
	// Default: moondream
	FastModel string `json:"fast_model" mapstructure:"fast_model"`

	// SmartModel is the model for Smart Lane (OCR, code analysis)
	// Default: minicpm-v
	SmartModel string `json:"smart_model" mapstructure:"smart_model"`

	// MaxImageSizeMB is the maximum allowed image size in megabytes
	// Default: 10
	MaxImageSizeMB int `json:"max_image_size_mb" mapstructure:"max_image_size_mb"`

	// FastModelTimeout is the timeout for Fast Lane requests
	// Default: 10s (should complete in <500ms, but allow buffer)
	FastModelTimeout time.Duration `json:"fast_model_timeout" mapstructure:"fast_model_timeout"`

	// SmartModelTimeout is the timeout for Smart Lane requests
	// Default: 30s (complex analysis can take 2-4s, allow buffer)
	SmartModelTimeout time.Duration `json:"smart_model_timeout" mapstructure:"smart_model_timeout"`

	// EnableFallback controls automatic fallback to Fast Lane when Smart fails
	// Default: true (Fail Gracefully principle)
	EnableFallback bool `json:"enable_fallback" mapstructure:"enable_fallback"`

	// HealthCheckInterval is how often to refresh provider health status
	// Default: 30s
	HealthCheckInterval time.Duration `json:"health_check_interval" mapstructure:"health_check_interval"`
}

// DefaultConfig returns sensible defaults for vision configuration.
// Optimized for local-first operation with Ollama.
func DefaultConfig() Config {
	return Config{
		Enabled:             true,
		OllamaURL:           "http://127.0.0.1:11434",
		FastModel:           "moondream",
		SmartModel:          "minicpm-v",
		MaxImageSizeMB:      10,
		FastModelTimeout:    10 * time.Second,
		SmartModelTimeout:   30 * time.Second,
		EnableFallback:      true,
		HealthCheckInterval: 30 * time.Second,
	}
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c.OllamaURL == "" {
		c.OllamaURL = "http://127.0.0.1:11434"
	}
	if c.FastModel == "" {
		c.FastModel = "moondream"
	}
	if c.SmartModel == "" {
		c.SmartModel = "minicpm-v"
	}
	if c.MaxImageSizeMB <= 0 {
		c.MaxImageSizeMB = 10
	}
	if c.FastModelTimeout <= 0 {
		c.FastModelTimeout = 10 * time.Second
	}
	if c.SmartModelTimeout <= 0 {
		c.SmartModelTimeout = 30 * time.Second
	}
	if c.HealthCheckInterval <= 0 {
		c.HealthCheckInterval = 30 * time.Second
	}
	return nil
}

// RouterConfig contains configuration specific to the vision router.
// This is derived from Config but includes runtime state configuration.
type RouterConfig struct {
	Config

	// FastLaneDefaultPrompt is the default prompt for simple queries
	FastLaneDefaultPrompt string `json:"fast_lane_default_prompt"`

	// SmartLaneDefaultPrompt is the default prompt for complex analysis
	SmartLaneDefaultPrompt string `json:"smart_lane_default_prompt"`
}

// DefaultRouterConfig returns default router configuration.
func DefaultRouterConfig() RouterConfig {
	return RouterConfig{
		Config:                 DefaultConfig(),
		FastLaneDefaultPrompt:  "Describe this image briefly.",
		SmartLaneDefaultPrompt: "Analyze this image in detail.",
	}
}
