package config_test

import (
	"fmt"
	"log"
	"os"

	"github.com/normanking/cortex/internal/config"
)

// ExampleLoad demonstrates how to load configuration from the default location.
func ExampleLoad() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("Default provider: %s\n", cfg.LLM.DefaultProvider)
	fmt.Printf("Knowledge DB: %s\n", cfg.Knowledge.DBPath)
	fmt.Printf("Theme: %s\n", cfg.TUI.Theme)
}

// ExampleLoadFromPath demonstrates loading config from a specific path.
func ExampleLoadFromPath() {
	cfg, err := config.LoadFromPath("/tmp/test-cortex/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("Loaded from custom path\n")
	fmt.Printf("Provider: %s\n", cfg.LLM.DefaultProvider)
}

// ExampleConfig_Save demonstrates saving configuration changes.
func ExampleConfig_Save() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Modify configuration
	cfg.TUI.VimMode = true
	cfg.LLM.DefaultProvider = "openai"

	// Save changes
	if err := cfg.Save(); err != nil {
		log.Fatalf("Failed to save config: %v", err)
	}

	fmt.Println("Configuration saved successfully")
}

// ExampleConfig_Validate demonstrates configuration validation.
func ExampleConfig_Validate() {
	cfg := config.Default()

	// Validate default config
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid config: %v", err)
	}

	fmt.Println("Configuration is valid")

	// Try an invalid configuration
	cfg.Knowledge.DefaultTier = "invalid-tier"
	if err := cfg.Validate(); err != nil {
		fmt.Printf("Validation error: %v\n", err)
	}
}

// ExampleConfig_EnsureDirectories demonstrates directory creation.
func ExampleConfig_EnsureDirectories() {
	cfg := config.Default()

	if err := cfg.EnsureDirectories(); err != nil {
		log.Fatalf("Failed to create directories: %v", err)
	}

	fmt.Println("All directories created successfully")
}

// ExampleDefault demonstrates creating a config with default values.
func ExampleDefault() {
	cfg := config.Default()

	fmt.Printf("Default provider: %s\n", cfg.LLM.DefaultProvider)
	fmt.Printf("Ollama endpoint: %s\n", cfg.LLM.Providers["ollama"].Endpoint)
	fmt.Printf("Knowledge tier: %s\n", cfg.Knowledge.DefaultTier)
	fmt.Printf("Sync enabled: %v\n", cfg.Sync.Enabled)
}

// ExampleProviderConfig demonstrates working with provider configurations.
func Example_providerConfig() {
	cfg := config.Default()

	// Access provider configuration
	ollamaProvider, exists := cfg.LLM.Providers["ollama"]
	if exists {
		fmt.Printf("Ollama endpoint: %s\n", ollamaProvider.Endpoint)
		fmt.Printf("Ollama model: %s\n", ollamaProvider.Model)
	}

	// Add a new provider
	cfg.LLM.Providers["custom"] = config.ProviderConfig{
		Endpoint: "http://localhost:8080",
		APIKey:   "custom-key",
		Model:    "custom-model",
	}

	// Switch default provider
	cfg.LLM.DefaultProvider = "custom"

	fmt.Printf("New default provider: %s\n", cfg.LLM.DefaultProvider)
}

// Example_environmentVariables demonstrates how environment variables override config.
func Example_environmentVariables() {
	// Set environment variables before loading config
	os.Setenv("CORTEX_LLM_DEFAULT_PROVIDER", "anthropic")
	os.Setenv("CORTEX_TUI_VIM_MODE", "true")
	defer func() {
		os.Unsetenv("CORTEX_LLM_DEFAULT_PROVIDER")
		os.Unsetenv("CORTEX_TUI_VIM_MODE")
	}()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Environment variables override file values
	fmt.Printf("Provider (from env): %s\n", cfg.LLM.DefaultProvider)
	fmt.Printf("Vim mode (from env): %v\n", cfg.TUI.VimMode)
}

// Example_apiKeyConfiguration demonstrates secure API key handling.
func Example_apiKeyConfiguration() {
	cfg := config.Default()

	// API keys should be set via environment variables for security
	// Example: export CORTEX_LLM_PROVIDERS_OPENAI_API_KEY="sk-..."

	// Check if API key is set
	openaiProvider := cfg.LLM.Providers["openai"]
	if openaiProvider.APIKey == "" {
		fmt.Println("OpenAI API key not configured")
		fmt.Println("Set via: export CORTEX_LLM_PROVIDERS_OPENAI_API_KEY=your-key")
	} else {
		fmt.Println("OpenAI API key is configured")
	}

	// Anthropic API key
	anthropicProvider := cfg.LLM.Providers["anthropic"]
	if anthropicProvider.APIKey == "" {
		fmt.Println("Anthropic API key not configured")
		fmt.Println("Set via: export CORTEX_LLM_PROVIDERS_ANTHROPIC_API_KEY=your-key")
	}
}

// Example_syncConfiguration demonstrates configuring cloud sync.
func Example_syncConfiguration() {
	cfg := config.Default()

	// Enable sync
	cfg.Sync.Enabled = true
	cfg.Sync.AuthToken = "your-auth-token"

	// Configure sync interval
	// cfg.Sync.Interval is already set to 5 minutes by default
	fmt.Printf("Sync enabled: %v\n", cfg.Sync.Enabled)
	fmt.Printf("Sync endpoint: %s\n", cfg.Sync.Endpoint)
	fmt.Printf("Sync interval: %v\n", cfg.Sync.Interval)

	// Save configuration
	if err := cfg.Save(); err != nil {
		log.Fatalf("Failed to save config: %v", err)
	}
}

// Example_knowledgeConfiguration demonstrates knowledge system configuration.
func Example_knowledgeConfiguration() {
	cfg := config.Default()

	fmt.Printf("Knowledge DB: %s\n", cfg.Knowledge.DBPath)
	fmt.Printf("Default tier: %s\n", cfg.Knowledge.DefaultTier)
	fmt.Printf("Trust decay: %d days\n", cfg.Knowledge.TrustDecayDays)

	// Change knowledge settings
	cfg.Knowledge.DefaultTier = "team"
	cfg.Knowledge.TrustDecayDays = 60

	fmt.Println("Knowledge settings updated")
}

// Example_tuiConfiguration demonstrates TUI customization.
func Example_tuiConfiguration() {
	cfg := config.Default()

	// Customize TUI
	cfg.TUI.Theme = "light"
	cfg.TUI.VimMode = true
	cfg.TUI.SidebarWidth = 40

	fmt.Printf("Theme: %s\n", cfg.TUI.Theme)
	fmt.Printf("Vim mode: %v\n", cfg.TUI.VimMode)
	fmt.Printf("Sidebar width: %d\n", cfg.TUI.SidebarWidth)
}

// Example_loggingConfiguration demonstrates logging setup.
func Example_loggingConfiguration() {
	cfg := config.Default()

	fmt.Printf("Log level: %s\n", cfg.Logging.Level)
	fmt.Printf("Log file: %s\n", cfg.Logging.File)

	// Change log level for debugging
	cfg.Logging.Level = "debug"

	fmt.Println("Log level set to debug")
}

// Example_fullWorkflow demonstrates a complete configuration workflow.
func Example_fullWorkflow() {
	// 1. Load existing config or create default
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Ensure all directories exist
	if err := cfg.EnsureDirectories(); err != nil {
		log.Fatalf("Failed to create directories: %v", err)
	}

	// 3. Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// 4. Use configuration
	fmt.Printf("Using provider: %s\n", cfg.LLM.DefaultProvider)

	provider := cfg.LLM.Providers[cfg.LLM.DefaultProvider]
	fmt.Printf("Model: %s\n", provider.Model)

	// 5. Make changes if needed
	if cfg.TUI.VimMode {
		fmt.Println("Vim mode is enabled")
	}

	// 6. Save any changes
	if err := cfg.Save(); err != nil {
		log.Fatalf("Failed to save config: %v", err)
	}

	fmt.Println("Configuration workflow complete")
}
