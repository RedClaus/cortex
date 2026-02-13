---
project: Cortex
component: Unknown
phase: Archive
date_created: 2025-12-18T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:51.697245
---

# Integration Guide: Cortex Data Layer

How to integrate the SQLite data layer into the Cortex application.

## Overview

The data layer provides a complete SQLite-based persistence solution with:
- Knowledge item storage (SOPs, lessons, patterns, documents)
- Full-text search (FTS5)
- Trust score tracking
- Session/conversation history
- Local-first architecture with sync support

## Step 1: Initialize at Application Startup

```go
// cmd/cortex/main.go or internal/app/app.go

package main

import (
    "context"
    "log"
    "os"
    "path/filepath"
    "time"

    "github.com/normanking/cortex/internal/data"
)

type App struct {
    store *data.Store
    // ... other fields
}

func NewApp() (*App, error) {
    // Determine data directory
    homeDir, err := os.UserHomeDir()
    if err != nil {
        return nil, err
    }
    dataDir := filepath.Join(homeDir, ".cortex")

    // Initialize database
    store, err := data.NewDB(dataDir)
    if err != nil {
        return nil, err
    }

    // Verify health
    if err := store.Health(); err != nil {
        store.Close()
        return nil, err
    }

    return &App{
        store: store,
    }, nil
}

func (a *App) Close() error {
    if a.store != nil {
        return a.store.Close()
    }
    return nil
}

func main() {
    app, err := NewApp()
    if err != nil {
        log.Fatalf("Failed to initialize app: %v", err)
    }
    defer app.Close()

    // ... rest of application
}
```

## Step 2: Knowledge Recording

When the AI assistant completes a task, record the knowledge:

```go
// internal/agent/observer.go

package agent

import (
    "context"
    "time"

    "github.com/google/uuid"
    "github.com/normanking/cortex/internal/data"
    "github.com/normanking/cortex/pkg/types"
)

type Observer struct {
    store *data.Store
}

// RecordLesson captures a lesson learned from a successful task.
func (o *Observer) RecordLesson(ctx context.Context, lesson string, tags []string) error {
    item := &types.KnowledgeItem{
        ID:         uuid.New().String(),
        Type:       types.TypeLesson,
        Content:    lesson,
        Tags:       tags,
        Scope:      types.ScopePersonal,
        AuthorID:   getCurrentUserID(), // Implement based on your user management
        Confidence: 0.7,                // Initial confidence
        TrustScore: 0.5,                // Will improve with feedback
        SyncStatus: "pending",
        CreatedAt:  time.Now(),
        UpdatedAt:  time.Now(),
    }

    return o.store.CreateKnowledge(ctx, item)
}

// RecordSOP captures a standard operating procedure.
func (o *Observer) RecordSOP(ctx context.Context, title, content string, tags []string) error {
    item := &types.KnowledgeItem{
        ID:         uuid.New().String(),
        Type:       types.TypeSOP,
        Title:      title,
        Content:    content,
        Tags:       tags,
        Scope:      types.ScopePersonal,
        AuthorID:   getCurrentUserID(),
        Confidence: 0.8,
        TrustScore: 0.6,
        SyncStatus: "pending",
        CreatedAt:  time.Now(),
        UpdatedAt:  time.Now(),
    }

    return o.store.CreateKnowledge(ctx, item)
}
```

## Step 3: Knowledge Retrieval

Integrate retrieval into your AI prompt builder:

```go
// internal/agent/retriever.go

package agent

import (
    "context"
    "fmt"
    "strings"

    "github.com/normanking/cortex/internal/data"
    "github.com/normanking/cortex/pkg/types"
)

type Retriever struct {
    store *data.Store
}

// GetRelevantKnowledge retrieves knowledge items for a given query.
func (r *Retriever) GetRelevantKnowledge(ctx context.Context, query string, limit int) ([]*types.KnowledgeItem, error) {
    // Try full-text search first
    results, err := r.store.SearchKnowledgeFTS(ctx, query, limit)
    if err != nil {
        return nil, fmt.Errorf("search knowledge: %w", err)
    }

    // If no results, try broader search
    if len(results) == 0 {
        results, err = r.store.ListKnowledge(ctx, types.SearchOptions{
            MinTrust: 0.6,
            Limit:    limit,
        })
        if err != nil {
            return nil, fmt.Errorf("list knowledge: %w", err)
        }
    }

    return results, nil
}

// BuildContextPrompt creates a prompt section with relevant knowledge.
func (r *Retriever) BuildContextPrompt(ctx context.Context, userQuery string) (string, error) {
    knowledge, err := r.GetRelevantKnowledge(ctx, userQuery, 5)
    if err != nil {
        return "", err
    }

    if len(knowledge) == 0 {
        return "", nil
    }

    var builder strings.Builder
    builder.WriteString("\n## Relevant Knowledge\n\n")

    for i, item := range knowledge {
        builder.WriteString(fmt.Sprintf("### %d. %s (%s, trust: %.2f)\n",
            i+1, item.Title, item.Type, item.TrustScore))
        builder.WriteString(item.Content)
        builder.WriteString("\n\n")
    }

    return builder.String(), nil
}
```

## Step 4: Session Management

Track conversations for context and history:

```go
// internal/agent/session.go

package agent

import (
    "context"
    "time"

    "github.com/google/uuid"
    "github.com/normanking/cortex/internal/data"
    "github.com/normanking/cortex/pkg/types"
)

type SessionManager struct {
    store     *data.Store
    currentID string
}

// Start creates a new conversation session.
func (s *SessionManager) Start(ctx context.Context, cwd string) error {
    s.currentID = uuid.New().String()

    session := &types.Session{
        ID:             s.currentID,
        UserID:         getCurrentUserID(),
        CWD:            cwd,
        Status:         "active",
        StartedAt:      time.Now(),
        LastActivityAt: time.Now(),
    }

    return s.store.CreateSession(ctx, session)
}

// AddUserMessage records a user message.
func (s *SessionManager) AddUserMessage(ctx context.Context, content string) error {
    msg := &types.SessionMessage{
        SessionID: s.currentID,
        Role:      "user",
        Content:   content,
        CreatedAt: time.Now(),
    }

    return s.store.AddMessage(ctx, msg)
}

// AddAssistantMessage records an AI response.
func (s *SessionManager) AddAssistantMessage(ctx context.Context, content string) error {
    msg := &types.SessionMessage{
        SessionID: s.currentID,
        Role:      "assistant",
        Content:   content,
        CreatedAt: time.Now(),
    }

    return s.store.AddMessage(ctx, msg)
}

// AddToolExecution records a command execution.
func (s *SessionManager) AddToolExecution(ctx context.Context, toolName, input, output string, success bool) error {
    msg := &types.SessionMessage{
        SessionID:   s.currentID,
        Role:        "tool",
        Content:     fmt.Sprintf("Executed: %s", toolName),
        ToolName:    toolName,
        ToolInput:   input,
        ToolOutput:  output,
        ToolSuccess: &success,
        CreatedAt:   time.Now(),
    }

    return s.store.AddMessage(ctx, msg)
}

// GetHistory retrieves conversation history.
func (s *SessionManager) GetHistory(ctx context.Context) ([]*types.SessionMessage, error) {
    return s.store.GetSessionMessages(ctx, s.currentID)
}

// End marks the session as completed.
func (s *SessionManager) End(ctx context.Context) error {
    return s.store.UpdateSessionStatus(ctx, s.currentID, "completed")
}
```

## Step 5: Trust Score Updates

Update trust scores based on task outcomes:

```go
// internal/agent/trust.go

package agent

import (
    "context"

    "github.com/normanking/cortex/internal/data"
)

type TrustManager struct {
    store *data.Store
}

// RecordOutcome updates trust scores after a task.
func (t *TrustManager) RecordOutcome(ctx context.Context, userID, domain string, success bool) error {
    return t.store.UpdateTrustScore(ctx, userID, domain, success)
}

// GetReliability retrieves a user's trust profile for a domain.
func (t *TrustManager) GetReliability(ctx context.Context, userID, domain string) (float64, error) {
    profile, err := t.store.GetTrustProfile(ctx, userID, domain)
    if err != nil {
        return 0, err
    }

    return profile.Score, nil
}

// Example: After executing a Linux command
func (t *TrustManager) UpdateAfterCommand(ctx context.Context, userID, platform string, exitCode int) error {
    success := exitCode == 0
    return t.RecordOutcome(ctx, userID, platform, success)
}
```

## Step 6: Wiring It All Together

```go
// internal/agent/agent.go

package agent

import (
    "context"

    "github.com/normanking/cortex/internal/data"
)

type Agent struct {
    store     *data.Store
    observer  *Observer
    retriever *Retriever
    sessions  *SessionManager
    trust     *TrustManager
}

func NewAgent(store *data.Store) *Agent {
    return &Agent{
        store:     store,
        observer:  &Observer{store: store},
        retriever: &Retriever{store: store},
        sessions:  &SessionManager{store: store},
        trust:     &TrustManager{store: store},
    }
}

// ProcessQuery handles a user query with full context.
func (a *Agent) ProcessQuery(ctx context.Context, query string) (string, error) {
    // 1. Add user message to session
    if err := a.sessions.AddUserMessage(ctx, query); err != nil {
        return "", err
    }

    // 2. Retrieve relevant knowledge
    contextPrompt, err := a.retriever.BuildContextPrompt(ctx, query)
    if err != nil {
        return "", err
    }

    // 3. Get conversation history
    history, err := a.sessions.GetHistory(ctx)
    if err != nil {
        return "", err
    }

    // 4. Build full prompt with context
    fullPrompt := buildPrompt(query, contextPrompt, history)

    // 5. Call LLM (implement based on your provider)
    response, err := callLLM(ctx, fullPrompt)
    if err != nil {
        return "", err
    }

    // 6. Record assistant response
    if err := a.sessions.AddAssistantMessage(ctx, response); err != nil {
        return "", err
    }

    return response, nil
}

// LearnFromSuccess records a successful task completion.
func (a *Agent) LearnFromSuccess(ctx context.Context, lesson string, tags []string, domain string) error {
    // 1. Record the lesson
    if err := a.observer.RecordLesson(ctx, lesson, tags); err != nil {
        return err
    }

    // 2. Update trust score
    userID := getCurrentUserID()
    return a.trust.RecordOutcome(ctx, userID, domain, true)
}
```

## Step 7: CLI Integration

```go
// cmd/cortex/main.go

package main

import (
    "context"
    "fmt"
    "os"
    "time"

    "github.com/normanking/cortex/internal/agent"
    "github.com/normanking/cortex/internal/data"
    "github.com/spf13/cobra"
)

func main() {
    var rootCmd = &cobra.Command{
        Use:   "cortex",
        Short: "Cortex AI Assistant",
        RunE:  runInteractive,
    }

    if err := rootCmd.Execute(); err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
}

func runInteractive(cmd *cobra.Command, args []string) error {
    // Initialize database
    homeDir, _ := os.UserHomeDir()
    store, err := data.NewDB(homeDir + "/.cortex")
    if err != nil {
        return fmt.Errorf("initialize database: %w", err)
    }
    defer store.Close()

    // Create agent
    ag := agent.NewAgent(store)

    // Start session
    ctx := context.Background()
    cwd, _ := os.Getwd()
    if err := ag.sessions.Start(ctx, cwd); err != nil {
        return fmt.Errorf("start session: %w", err)
    }
    defer ag.sessions.End(ctx)

    // Interactive loop
    fmt.Println("Cortex AI Assistant (type 'exit' to quit)")
    for {
        fmt.Print("\n> ")
        var query string
        fmt.Scanln(&query)

        if query == "exit" {
            break
        }

        response, err := ag.ProcessQuery(ctx, query)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            continue
        }

        fmt.Println(response)
    }

    return nil
}
```

## Configuration

Add database configuration to your config file:

```go
// internal/config/config.go

type Config struct {
    Database DatabaseConfig `yaml:"database"`
    // ... other fields
}

type DatabaseConfig struct {
    DataDir        string `yaml:"data_dir"`
    MaxOpenConns   int    `yaml:"max_open_conns"`
    BusyTimeout    int    `yaml:"busy_timeout_ms"`
    EnableMetrics  bool   `yaml:"enable_metrics"`
}

// Default configuration
func DefaultConfig() *Config {
    homeDir, _ := os.UserHomeDir()
    return &Config{
        Database: DatabaseConfig{
            DataDir:       filepath.Join(homeDir, ".cortex"),
            MaxOpenConns:  1,
            BusyTimeout:   5000,
            EnableMetrics: true,
        },
    }
}
```

## Error Handling Best Practices

```go
// Wrap all database errors with context
if err := store.CreateKnowledge(ctx, item); err != nil {
    return fmt.Errorf("save knowledge item: %w", err)
}

// Check for specific errors
import "errors"

_, err := store.GetKnowledge(ctx, id)
if err != nil {
    if errors.Is(err, sql.ErrNoRows) {
        // Handle not found
        return nil, nil
    }
    return nil, fmt.Errorf("get knowledge: %w", err)
}

// Use context for timeouts
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

results, err := store.SearchKnowledgeFTS(ctx, query, 10)
```

## Health Checks and Monitoring

```go
// Periodic health check
func (a *App) healthCheck() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        if err := a.store.Health(); err != nil {
            log.Printf("Database health check failed: %v", err)
            // Trigger alert or reconnect
        }
    }
}

// Start health monitoring
go app.healthCheck()
```

## Migration Path

If you have existing data:

1. **Export current data** to JSON
2. **Create migration script** to convert to KnowledgeItem format
3. **Import using CreateKnowledge** in a transaction

```go
func importLegacyData(ctx context.Context, store *data.Store, legacyFile string) error {
    // Read legacy JSON
    data, err := os.ReadFile(legacyFile)
    if err != nil {
        return err
    }

    var legacyItems []LegacyItem
    if err := json.Unmarshal(data, &legacyItems); err != nil {
        return err
    }

    // Import in transaction
    return store.WithTx(ctx, func(tx *sql.Tx) error {
        for _, legacy := range legacyItems {
            item := convertToKnowledgeItem(legacy)
            if err := store.CreateKnowledge(ctx, item); err != nil {
                return err
            }
        }
        return nil
    })
}
```

## Next Steps

1. ✅ **Database layer** - Complete (this implementation)
2. ⏭️ **Retrieval engine** - Implement tiered search (strict/fuzzy/fallback)
3. ⏭️ **LLM integration** - Connect to OpenAI/Claude API
4. ⏭️ **CLI interface** - Bubble Tea UI for interactions
5. ⏭️ **Sync engine** - Acontext cloud synchronization

## Resources

- [Data Layer README](./README.md) - Full API documentation
- [Example Tests](./example_test.go) - Usage examples
- [Schema DDL](./migrations/001_initial_schema.sql) - Database structure
- [Type Definitions](../../pkg/types/types.go) - Go types
