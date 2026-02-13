// Package config provides configuration management for the Cortex AI assistant.
//
// # Overview
//
// The config package uses Viper to load configuration from YAML files and
// environment variables. It provides a type-safe configuration structure with
// validation, default values, and automatic file creation.
//
// # Configuration File
//
// The configuration is stored at ~/.cortex/config.yaml and is automatically
// created with sensible defaults on first use. The file structure mirrors
// the Go structs defined in this package.
//
// # Environment Variables
//
// All configuration values can be overridden using environment variables
// with the CORTEX_ prefix. Nested fields are separated by underscores.
//
// Examples:
//   - CORTEX_LLM_DEFAULT_PROVIDER=openai
//   - CORTEX_LLM_PROVIDERS_OPENAI_API_KEY=sk-...
//   - CORTEX_TUI_VIM_MODE=true
//   - CORTEX_LOGGING_LEVEL=debug
//
// # Usage Example
//
//	package main
//
//	import (
//	    "log"
//	    "github.com/normanking/cortex/internal/config"
//	)
//
//	func main() {
//	    // Load configuration
//	    cfg, err := config.Load()
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//
//	    // Ensure all directories exist
//	    if err := cfg.EnsureDirectories(); err != nil {
//	        log.Fatal(err)
//	    }
//
//	    // Validate configuration
//	    if err := cfg.Validate(); err != nil {
//	        log.Fatal(err)
//	    }
//
//	    // Use configuration
//	    provider := cfg.LLM.Providers[cfg.LLM.DefaultProvider]
//	    log.Printf("Using %s with model %s", cfg.LLM.DefaultProvider, provider.Model)
//	}
//
// # Security Best Practices
//
// API keys should be stored in environment variables rather than in the
// config file to prevent accidental exposure:
//
//	export CORTEX_LLM_PROVIDERS_OPENAI_API_KEY=sk-...
//	export CORTEX_LLM_PROVIDERS_ANTHROPIC_API_KEY=sk-ant-...
//
// # Configuration Sections
//
//   - LLM: Language model provider configuration (Ollama, OpenAI, Anthropic)
//   - Knowledge: Knowledge database and trust management settings
//   - Sync: Cloud synchronization configuration
//   - TUI: Terminal user interface preferences (theme, vim mode, layout)
//   - Logging: Log level and output file configuration
//
// # Path Expansion
//
// The package automatically expands ~ to the user's home directory in
// all path configurations, making config files portable across systems.
//
// # Validation
//
// The Validate() method checks configuration for common errors:
//   - Provider existence and consistency
//   - Valid enum values (theme, tier, log level)
//   - Numeric range validation
//   - Required field presence
//
// # Thread Safety
//
// Config instances are not thread-safe. If you need concurrent access,
// wrap the config in a sync.RWMutex or create separate instances.
//
// # Testing
//
// The package includes comprehensive tests demonstrating all functionality.
// Run tests with:
//
//	go test ./internal/config/
//
// See example_test.go for usage examples and patterns.
package config
