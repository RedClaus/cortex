---
project: Cortex
component: Unknown
phase: Archive
date_created: 2025-12-18T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:51.401869
---

# Config Integration Guide

This guide shows how to integrate the config package into the Cortex application.

## Integration with main.go

Here's how to integrate configuration into `cmd/cortex/main.go`:

```go
package main

import (
    "fmt"
    "os"

    "github.com/normanking/cortex/internal/config"
    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
    "github.com/spf13/cobra"
)

var (
    version = "0.1.0-dev"
    cfg     *config.Config
)

func main() {
    // Initialize configuration early
    var err error
    cfg, err = config.Load()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
        os.Exit(1)
    }

    // Ensure all directories exist
    if err := cfg.EnsureDirectories(); err != nil {
        fmt.Fprintf(os.Stderr, "Failed to create directories: %v\n", err)
        os.Exit(1)
    }

    // Validate configuration
    if err := cfg.Validate(); err != nil {
        fmt.Fprintf(os.Stderr, "Invalid configuration: %v\n", err)
        fmt.Fprintf(os.Stderr, "Please check %s\n", cfg.GetConfigPath())
        os.Exit(1)
    }

    // Setup logging based on config
    setupLogging(cfg)

    log.Info().
        Str("version", version).
        Str("provider", cfg.LLM.DefaultProvider).
        Str("config_path", cfg.GetConfigPath()).
        Msg("Starting Cortex")

    rootCmd := &cobra.Command{
        Use:   "cortex",
        Short: "Cortex - Local-first AI assistant",
        Run:   runTUI,
    }

    // Add commands
    rootCmd.AddCommand(versionCmd())
    rootCmd.AddCommand(configCmd())
    rootCmd.AddCommand(syncCmd())
    rootCmd.AddCommand(knowledgeCmd())

    if err := rootCmd.Execute(); err != nil {
        log.Error().Err(err).Msg("Command failed")
        os.Exit(1)
    }
}

func setupLogging(cfg *config.Config) {
    // Set log level
    level, err := zerolog.ParseLevel(cfg.Logging.Level)
    if err != nil {
        level = zerolog.InfoLevel
    }
    zerolog.SetGlobalLevel(level)

    // Setup log file
    if cfg.Logging.File != "" {
        file, err := os.OpenFile(
            cfg.Logging.File,
            os.O_CREATE|os.O_APPEND|os.O_WRONLY,
            0644,
        )
        if err == nil {
            log.Logger = log.Output(file)
        }
    }
}

func runTUI(cmd *cobra.Command, args []string) {
    log.Info().Msg("Starting TUI")
    fmt.Printf("🧠 Cortex v%s\n", version)
    fmt.Printf("Provider: %s\n", cfg.LLM.DefaultProvider)
    fmt.Printf("Theme: %s\n", cfg.TUI.Theme)
    fmt.Printf("Vim Mode: %v\n", cfg.TUI.VimMode)

    // TODO: Launch BubbleTea TUI with config
}

func versionCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "version",
        Short: "Print version information",
        Run: func(cmd *cobra.Command, args []string) {
            fmt.Printf("Cortex v%s\n", version)
            fmt.Printf("Config: %s\n", cfg.GetConfigPath())
            fmt.Printf("Provider: %s\n", cfg.LLM.DefaultProvider)
        },
    }
}

func configCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "config",
        Short: "Manage configuration",
    }

    // config show
    cmd.AddCommand(&cobra.Command{
        Use:   "show",
        Short: "Show current configuration",
        Run: func(cmd *cobra.Command, args []string) {
            fmt.Printf("Configuration file: %s\n\n", cfg.GetConfigPath())
            fmt.Printf("LLM:\n")
            fmt.Printf("  Default Provider: %s\n", cfg.LLM.DefaultProvider)
            for name, prov := range cfg.LLM.Providers {
                fmt.Printf("  %s:\n", name)
                if prov.Endpoint != "" {
                    fmt.Printf("    Endpoint: %s\n", prov.Endpoint)
                }
                fmt.Printf("    Model: %s\n", prov.Model)
                if prov.APIKey != "" {
                    fmt.Printf("    API Key: [configured]\n")
                } else {
                    fmt.Printf("    API Key: [not set]\n")
                }
            }
            fmt.Printf("\nKnowledge:\n")
            fmt.Printf("  DB Path: %s\n", cfg.Knowledge.DBPath)
            fmt.Printf("  Default Tier: %s\n", cfg.Knowledge.DefaultTier)
            fmt.Printf("  Trust Decay: %d days\n", cfg.Knowledge.TrustDecayDays)
            fmt.Printf("\nSync:\n")
            fmt.Printf("  Enabled: %v\n", cfg.Sync.Enabled)
            fmt.Printf("  Endpoint: %s\n", cfg.Sync.Endpoint)
            fmt.Printf("\nTUI:\n")
            fmt.Printf("  Theme: %s\n", cfg.TUI.Theme)
            fmt.Printf("  Vim Mode: %v\n", cfg.TUI.VimMode)
            fmt.Printf("  Sidebar Width: %d\n", cfg.TUI.SidebarWidth)
        },
    })

    // config edit
    cmd.AddCommand(&cobra.Command{
        Use:   "edit",
        Short: "Edit configuration file",
        Run: func(cmd *cobra.Command, args []string) {
            editor := os.Getenv("EDITOR")
            if editor == "" {
                editor = "nano"
            }
            configPath := cfg.GetConfigPath()
            fmt.Printf("Opening %s with %s...\n", configPath, editor)
            // TODO: exec.Command(editor, configPath).Run()
        },
    })

    // config validate
    cmd.AddCommand(&cobra.Command{
        Use:   "validate",
        Short: "Validate configuration",
        Run: func(cmd *cobra.Command, args []string) {
            if err := cfg.Validate(); err != nil {
                fmt.Fprintf(os.Stderr, "❌ Configuration is invalid: %v\n", err)
                os.Exit(1)
            }
            fmt.Println("✅ Configuration is valid")
        },
    })

    // config set-provider
    cmd.AddCommand(&cobra.Command{
        Use:   "set-provider [provider]",
        Short: "Set default LLM provider",
        Args:  cobra.ExactArgs(1),
        Run: func(cmd *cobra.Command, args []string) {
            provider := args[0]
            if _, exists := cfg.LLM.Providers[provider]; !exists {
                fmt.Fprintf(os.Stderr, "Unknown provider: %s\n", provider)
                fmt.Fprintf(os.Stderr, "Available providers: ")
                for name := range cfg.LLM.Providers {
                    fmt.Fprintf(os.Stderr, "%s ", name)
                }
                fmt.Fprintln(os.Stderr)
                os.Exit(1)
            }
            cfg.LLM.DefaultProvider = provider
            if err := cfg.Save(); err != nil {
                fmt.Fprintf(os.Stderr, "Failed to save config: %v\n", err)
                os.Exit(1)
            }
            fmt.Printf("✅ Default provider set to %s\n", provider)
        },
    })

    return cmd
}

func syncCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "sync",
        Short: "Synchronize knowledge with Acontext",
        Run: func(cmd *cobra.Command, args []string) {
            if !cfg.Sync.Enabled {
                fmt.Println("⚠️  Sync is disabled in configuration")
                fmt.Println("Enable with: cortex config set sync.enabled true")
                return
            }
            log.Info().
                Str("endpoint", cfg.Sync.Endpoint).
                Msg("Starting sync")
            fmt.Printf("Syncing with %s...\n", cfg.Sync.Endpoint)
            // TODO: Implement sync
        },
    }
}

func knowledgeCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "knowledge",
        Short: "Manage knowledge items",
    }

    cmd.AddCommand(&cobra.Command{
        Use:   "list",
        Short: "List knowledge items",
        Run: func(cmd *cobra.Command, args []string) {
            log.Info().
                Str("db_path", cfg.Knowledge.DBPath).
                Msg("Listing knowledge")
            // TODO: Implement knowledge list
        },
    })

    cmd.AddCommand(&cobra.Command{
        Use:   "search [query]",
        Short: "Search knowledge items",
        Args:  cobra.MinimumNArgs(1),
        Run: func(cmd *cobra.Command, args []string) {
            query := args[0]
            log.Info().
                Str("query", query).
                Str("db_path", cfg.Knowledge.DBPath).
                Msg("Searching knowledge")
            // TODO: Implement knowledge search
        },
    })

    return cmd
}
```

## Using Config in Services

### LLM Service Example

```go
package llm

import (
    "github.com/normanking/cortex/internal/config"
    "github.com/sashabaranov/go-openai"
)

type Service struct {
    cfg      *config.Config
    clients  map[string]interface{}
}

func NewService(cfg *config.Config) *Service {
    return &Service{
        cfg:     cfg,
        clients: make(map[string]interface{}),
    }
}

func (s *Service) GetClient() (interface{}, error) {
    provider := s.cfg.LLM.DefaultProvider
    providerCfg := s.cfg.LLM.Providers[provider]

    switch provider {
    case "openai":
        if client, exists := s.clients[provider]; exists {
            return client, nil
        }
        client := openai.NewClient(providerCfg.APIKey)
        s.clients[provider] = client
        return client, nil

    case "ollama":
        // Create Ollama client with endpoint
        // ...

    case "anthropic":
        // Create Anthropic client
        // ...
    }

    return nil, fmt.Errorf("unsupported provider: %s", provider)
}
```

### TUI Service Example

```go
package tui

import (
    "github.com/normanking/cortex/internal/config"
    "github.com/charmbracelet/lipgloss"
)

type Model struct {
    cfg        *config.Config
    theme      Theme
    vimMode    bool
    sidebarWidth int
}

func NewModel(cfg *config.Config) Model {
    return Model{
        cfg:          cfg,
        theme:        loadTheme(cfg.TUI.Theme),
        vimMode:      cfg.TUI.VimMode,
        sidebarWidth: cfg.TUI.SidebarWidth,
    }
}

func loadTheme(name string) Theme {
    if name == "dark" {
        return Theme{
            Primary:   lipgloss.Color("#007AFF"),
            Secondary: lipgloss.Color("#5856D6"),
            // ...
        }
    }
    // Light theme
    return Theme{
        Primary:   lipgloss.Color("#0066CC"),
        // ...
    }
}
```

### Knowledge Service Example

```go
package knowledge

import (
    "database/sql"
    "github.com/normanking/cortex/internal/config"
    _ "modernc.org/sqlite"
)

type Service struct {
    db  *sql.DB
    cfg *config.Config
}

func NewService(cfg *config.Config) (*Service, error) {
    db, err := sql.Open("sqlite", cfg.Knowledge.DBPath)
    if err != nil {
        return nil, err
    }

    return &Service{
        db:  db,
        cfg: cfg,
    }, nil
}

func (s *Service) AddKnowledge(item Knowledge) error {
    // Use cfg.Knowledge.DefaultTier if no tier specified
    if item.Tier == "" {
        item.Tier = s.cfg.Knowledge.DefaultTier
    }

    // Set trust decay based on config
    item.TrustDecayDays = s.cfg.Knowledge.TrustDecayDays

    // Insert into database
    // ...
}
```

## Testing with Config

```go
package mypackage

import (
    "testing"
    "github.com/normanking/cortex/internal/config"
)

func TestWithConfig(t *testing.T) {
    // Use default config for tests
    cfg := config.Default()

    // Or create custom config for testing
    cfg := &config.Config{
        LLM: config.LLMConfig{
            DefaultProvider: "ollama",
            Providers: map[string]config.ProviderConfig{
                "ollama": {
                    Endpoint: "http://localhost:11434",
                    Model:    "llama3.2",
                },
            },
        },
        // ... other fields
    }

    // Run tests with config
    service := NewService(cfg)
    // ...
}
```

## Environment-Specific Configuration

### Development

```bash
export CORTEX_LOGGING_LEVEL=debug
export CORTEX_LLM_DEFAULT_PROVIDER=ollama
go run cmd/cortex/main.go
```

### Production

```bash
export CORTEX_LOGGING_LEVEL=info
export CORTEX_LLM_PROVIDERS_OPENAI_API_KEY=sk-...
export CORTEX_SYNC_ENABLED=true
export CORTEX_SYNC_AUTH_TOKEN=prod-token
./cortex
```

### Testing

```bash
export CORTEX_KNOWLEDGE_DB_PATH=/tmp/test-knowledge.db
export CORTEX_LOGGING_FILE=/tmp/test.log
go test ./...
```

## Configuration Lifecycle

```
1. Application Start
   ├─ Load config (config.Load())
   ├─ Validate config (cfg.Validate())
   ├─ Ensure directories (cfg.EnsureDirectories())
   └─ Setup logging

2. Initialize Services
   ├─ LLM service with cfg.LLM
   ├─ Knowledge service with cfg.Knowledge
   ├─ Sync service with cfg.Sync
   └─ TUI with cfg.TUI

3. Runtime
   ├─ Access config via services
   └─ Config changes via commands

4. Shutdown
   └─ Save config if modified (cfg.Save())
```

## Best Practices

1. **Load Once**: Load config once at startup and pass to services
2. **Read-Only**: Treat config as mostly read-only after init
3. **Validate Early**: Always validate after loading
4. **Environment Variables**: Use env vars for secrets and deployment-specific settings
5. **Directory Setup**: Call `EnsureDirectories()` before using any paths
6. **Error Handling**: Always check errors from `Load()`, `Save()`, and `Validate()`
7. **Logging**: Setup logging based on config before any other operations

## Migration Path

If you need to migrate from old config format:

```go
func migrateConfig() error {
    oldConfig := loadOldConfig() // Your old config loader

    newConfig := config.Default()
    newConfig.LLM.DefaultProvider = oldConfig.Provider
    // ... map other fields

    if err := newConfig.Save(); err != nil {
        return err
    }

    return nil
}
```
