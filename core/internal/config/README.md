---
project: Cortex
component: Docs
phase: Archive
date_created: 2025-12-18T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:51.441512
---

# Config Package

The `config` package provides comprehensive configuration management for the Cortex AI assistant. It uses [Viper](https://github.com/spf13/viper) for flexible configuration loading from YAML files and environment variables.

## Features

- **YAML Configuration**: Human-readable configuration in `~/.cortex/config.yaml`
- **Environment Variable Overrides**: Override any config value using `CORTEX_*` environment variables
- **Default Values**: Sensible defaults for immediate usage
- **Validation**: Built-in validation for configuration values
- **Path Expansion**: Automatic `~` expansion to home directory
- **Auto-Creation**: Automatically creates config file and directories if they don't exist

## Configuration Structure

```go
type Config struct {
    LLM       LLMConfig       // Language model configuration
    Knowledge KnowledgeConfig // Knowledge management settings
    Sync      SyncConfig      // Cloud synchronization settings
    TUI       TUIConfig       // Terminal UI preferences
    Logging   LoggingConfig   // Logging configuration
}
```

## Usage

### Basic Usage

```go
import "github.com/normanking/cortex/internal/config"

// Load configuration from default location (~/.cortex/config.yaml)
cfg, err := config.Load()
if err != nil {
    log.Fatal(err)
}

// Use configuration
provider := cfg.LLM.Providers[cfg.LLM.DefaultProvider]
fmt.Printf("Using model: %s\n", provider.Model)
```

### Loading from Custom Path

```go
cfg, err := config.LoadFromPath("/custom/path/config.yaml")
if err != nil {
    log.Fatal(err)
}
```

### Modifying and Saving

```go
cfg, err := config.Load()
if err != nil {
    log.Fatal(err)
}

// Modify configuration
cfg.TUI.VimMode = true
cfg.LLM.DefaultProvider = "anthropic"

// Save changes
if err := cfg.Save(); err != nil {
    log.Fatal(err)
}
```

### Validation

```go
cfg := config.Default()

if err := cfg.Validate(); err != nil {
    log.Fatalf("Invalid configuration: %v", err)
}
```

### Directory Management

```go
cfg, err := config.Load()
if err != nil {
    log.Fatal(err)
}

// Create all necessary directories
if err := cfg.EnsureDirectories(); err != nil {
    log.Fatal(err)
}
```

## Default Configuration

The default configuration is automatically created at `~/.cortex/config.yaml`:

```yaml
llm:
  default_provider: ollama
  providers:
    ollama:
      endpoint: http://localhost:11434
      model: llama3.2
    openai:
      api_key: ""
      model: gpt-4o-mini
    anthropic:
      api_key: ""
      model: claude-3-5-sonnet-20241022

knowledge:
  db_path: ~/.cortex/knowledge.db
  default_tier: personal
  trust_decay_days: 30

sync:
  enabled: false
  endpoint: https://api.acontext.io
  interval: 5m

tui:
  theme: dark
  vim_mode: false
  sidebar_width: 30

logging:
  level: info
  file: ~/.cortex/logs/cortex.log
```

## Environment Variable Overrides

Any configuration value can be overridden using environment variables with the `CORTEX_` prefix:

```bash
# Override default provider
export CORTEX_LLM_DEFAULT_PROVIDER=anthropic

# Set API keys (recommended for security)
export CORTEX_LLM_PROVIDERS_OPENAI_API_KEY=sk-...
export CORTEX_LLM_PROVIDERS_ANTHROPIC_API_KEY=sk-ant-...

# Enable vim mode
export CORTEX_TUI_VIM_MODE=true

# Set log level
export CORTEX_LOGGING_LEVEL=debug
```

### Environment Variable Naming Convention

- Use `CORTEX_` prefix
- Nested fields are separated by underscores
- All uppercase
- Example: `llm.providers.openai.api_key` → `CORTEX_LLM_PROVIDERS_OPENAI_API_KEY`

## LLM Configuration

### Supported Providers

1. **Ollama** (Local)
   - Endpoint: `http://localhost:11434`
   - No API key required
   - Default model: `llama3.2`

2. **OpenAI**
   - API key required
   - Default model: `gpt-4o-mini`

3. **Anthropic**
   - API key required
   - Default model: `claude-3-5-sonnet-20241022`

### Adding a New Provider

```go
cfg.LLM.Providers["custom"] = config.ProviderConfig{
    Endpoint: "http://localhost:8080",
    APIKey:   "your-key",
    Model:    "custom-model",
}

// Set as default
cfg.LLM.DefaultProvider = "custom"

// Save configuration
cfg.Save()
```

## Knowledge Configuration

The knowledge system stores learned patterns and context:

- **db_path**: SQLite database location
- **default_tier**: Trust tier for new knowledge (`personal`, `team`, `public`)
- **trust_decay_days**: Days before trust scores decay

```go
cfg.Knowledge.DefaultTier = "team"
cfg.Knowledge.TrustDecayDays = 60
```

## Sync Configuration

Cloud synchronization settings:

```go
cfg.Sync.Enabled = true
cfg.Sync.AuthToken = "your-token"
cfg.Sync.Interval = 10 * time.Minute
```

## TUI Configuration

Terminal user interface preferences:

- **theme**: `"dark"` or `"light"`
- **vim_mode**: Enable vim keybindings
- **sidebar_width**: Width in characters (10-100)

```go
cfg.TUI.Theme = "light"
cfg.TUI.VimMode = true
cfg.TUI.SidebarWidth = 40
```

## Logging Configuration

- **level**: `"debug"`, `"info"`, `"warn"`, `"error"`
- **file**: Log file path

```go
cfg.Logging.Level = "debug"
cfg.Logging.File = "~/.cortex/logs/cortex.log"
```

## Validation Rules

The `Validate()` method checks:

1. **LLM Configuration**
   - Default provider must not be empty
   - Default provider must exist in providers map

2. **Knowledge Configuration**
   - Default tier must be `personal`, `team`, or `public`
   - Trust decay days cannot be negative

3. **TUI Configuration**
   - Theme must be `dark` or `light`
   - Sidebar width must be between 10 and 100

4. **Logging Configuration**
   - Level must be `debug`, `info`, `warn`, or `error`

## API Reference

### Functions

- `Load() (*Config, error)` - Load from default location
- `LoadFromPath(path string) (*Config, error)` - Load from specific path
- `Default() *Config` - Create config with default values

### Methods

- `(c *Config) Save() error` - Save to default location
- `(c *Config) SaveToPath(path string) error` - Save to specific path
- `(c *Config) GetDataDir() string` - Get Cortex data directory
- `(c *Config) GetConfigPath() string` - Get config file path
- `(c *Config) EnsureDirectories() error` - Create all necessary directories
- `(c *Config) Validate() error` - Validate configuration

## Testing

Run the test suite:

```bash
go test ./internal/config/
```

Run with verbose output:

```bash
go test -v ./internal/config/
```

Run specific test:

```bash
go test -v ./internal/config/ -run TestLoadFromPath
```

## Security Best Practices

1. **API Keys**: Store API keys in environment variables, not in the config file
2. **File Permissions**: Config directory is created with `0755` permissions
3. **Sensitive Data**: Never commit config files with API keys to version control

```bash
# Recommended: Use environment variables
export CORTEX_LLM_PROVIDERS_OPENAI_API_KEY=sk-...

# Or use a .env file (not tracked by git)
echo "CORTEX_LLM_PROVIDERS_OPENAI_API_KEY=sk-..." > ~/.cortex/.env
```

## Directory Structure

```
~/.cortex/
├── config.yaml           # Main configuration file
├── knowledge.db          # SQLite knowledge database
└── logs/
    └── cortex.log        # Application logs
```

## Common Patterns

### Initialization Workflow

```go
func main() {
    // 1. Load configuration
    cfg, err := config.Load()
    if err != nil {
        log.Fatal(err)
    }

    // 2. Ensure directories exist
    if err := cfg.EnsureDirectories(); err != nil {
        log.Fatal(err)
    }

    // 3. Validate configuration
    if err := cfg.Validate(); err != nil {
        log.Fatal(err)
    }

    // 4. Use configuration
    // ...
}
```

### Dynamic Provider Selection

```go
cfg, _ := config.Load()

// Get provider from user input or default
providerName := cfg.LLM.DefaultProvider
provider, exists := cfg.LLM.Providers[providerName]
if !exists {
    log.Fatalf("Provider %s not configured", providerName)
}

// Use provider configuration
fmt.Printf("Using %s with model %s\n", providerName, provider.Model)
```

### Runtime Configuration Changes

```go
cfg, _ := config.Load()

// Change setting based on user preference
if userWantsVimMode {
    cfg.TUI.VimMode = true
    cfg.Save()
}
```

## Troubleshooting

### Config file not found
The config file is automatically created with default values on first load.

### Permission denied
Ensure the `~/.cortex` directory is writable by your user.

### Environment variables not working
Verify the variable name follows the convention: `CORTEX_SECTION_SUBSECTION_KEY`

### Validation errors
Run `cfg.Validate()` to see specific validation errors and fix accordingly.

## Contributing

When adding new configuration options:

1. Add the field to the appropriate config struct
2. Add default value in `Default()`
3. Add validation rules in `Validate()`
4. Update this README
5. Add tests in `config_test.go`
