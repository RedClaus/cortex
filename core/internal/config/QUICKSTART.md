---
project: Cortex
component: UI
phase: Archive
date_created: 2025-12-18T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:51.429279
---

# Config Package Quick Start

## Installation

The config package is already part of the Cortex project. No additional installation needed.

## 5-Minute Quick Start

### 1. Basic Usage

```go
package main

import (
    "log"
    "github.com/normanking/cortex/internal/config"
)

func main() {
    // Load config (auto-creates if missing)
    cfg, err := config.Load()
    if err != nil {
        log.Fatal(err)
    }

    // Ensure directories exist
    cfg.EnsureDirectories()

    // Validate config
    if err := cfg.Validate(); err != nil {
        log.Fatal(err)
    }

    // Use it!
    log.Printf("Using provider: %s", cfg.LLM.DefaultProvider)
}
```

### 2. Set API Keys (Recommended)

```bash
# Option 1: Environment variables (most secure)
export CORTEX_LLM_PROVIDERS_OPENAI_API_KEY="sk-..."
export CORTEX_LLM_PROVIDERS_ANTHROPIC_API_KEY="sk-ant-..."

# Option 2: Edit config file
nano ~/.cortex/config.yaml
# Add your API keys under llm.providers.<provider>.api_key
```

### 3. Change Settings

```go
cfg, _ := config.Load()

// Enable vim mode
cfg.TUI.VimMode = true

// Switch to Anthropic
cfg.LLM.DefaultProvider = "anthropic"

// Change log level
cfg.Logging.Level = "debug"

// Save changes
cfg.Save()
```

## Common Operations

### Get Current Provider

```go
cfg, _ := config.Load()
provider := cfg.LLM.Providers[cfg.LLM.DefaultProvider]
fmt.Printf("Model: %s\n", provider.Model)
```

### Add Custom Provider

```go
cfg, _ := config.Load()
cfg.LLM.Providers["custom"] = config.ProviderConfig{
    Endpoint: "http://localhost:8080",
    Model:    "custom-model",
}
cfg.LLM.DefaultProvider = "custom"
cfg.Save()
```

### Enable Cloud Sync

```go
cfg, _ := config.Load()
cfg.Sync.Enabled = true
cfg.Sync.AuthToken = "your-token"
cfg.Save()
```

### Change Theme

```go
cfg, _ := config.Load()
cfg.TUI.Theme = "light"  // or "dark"
cfg.Save()
```

## Environment Variable Cheat Sheet

```bash
# LLM Configuration
export CORTEX_LLM_DEFAULT_PROVIDER=anthropic
export CORTEX_LLM_PROVIDERS_OPENAI_API_KEY=sk-...
export CORTEX_LLM_PROVIDERS_ANTHROPIC_API_KEY=sk-ant-...

# TUI Configuration
export CORTEX_TUI_VIM_MODE=true
export CORTEX_TUI_THEME=light
export CORTEX_TUI_SIDEBAR_WIDTH=40

# Logging
export CORTEX_LOGGING_LEVEL=debug
export CORTEX_LOGGING_FILE=~/.cortex/logs/debug.log

# Knowledge
export CORTEX_KNOWLEDGE_DEFAULT_TIER=team
export CORTEX_KNOWLEDGE_TRUST_DECAY_DAYS=60

# Sync
export CORTEX_SYNC_ENABLED=true
export CORTEX_SYNC_AUTH_TOKEN=your-token
```

## Configuration File Location

```
~/.cortex/config.yaml
```

## Default Providers

1. **Ollama** (Local, default)
   - No API key needed
   - Model: `llama3.2`
   - Endpoint: `http://localhost:11434`

2. **OpenAI**
   - Requires API key
   - Model: `gpt-4o-mini`

3. **Anthropic**
   - Requires API key
   - Model: `claude-3-5-sonnet-20241022`

## Troubleshooting

### Config file not created
- Check write permissions on `~/.cortex/`
- Run `cfg.EnsureDirectories()` before operations

### API key not working
- Check environment variable name format
- Restart application after setting env vars
- Verify key is correct in config file or env var

### Validation errors
- Run `cfg.Validate()` to see specific errors
- Check README.md for valid values

## Next Steps

- Read [README.md](README.md) for detailed documentation
- Check [example_test.go](example_test.go) for more examples
- Run tests: `go test ./internal/config/`

## File Structure

```
~/.cortex/
├── config.yaml          # Main config file
├── knowledge.db         # Knowledge database
└── logs/
    └── cortex.log       # Application logs
```

## Support

For issues or questions:
1. Check validation: `cfg.Validate()`
2. Review logs: `~/.cortex/logs/cortex.log`
3. See full documentation in README.md
