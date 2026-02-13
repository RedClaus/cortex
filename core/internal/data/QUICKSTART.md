---
project: Cortex
component: UI
phase: Archive
date_created: 2025-12-18T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:51.713179
---

# Quick Start: Cortex Data Layer

5-minute guide to get started with the SQLite data layer.

## Installation

The data layer is already part of the Cortex codebase. Dependencies are in `go.mod`:

```bash
# All dependencies should already be installed
go mod download
```

## 30-Second Test

```bash
# Run the test suite
cd /Users/normanking/ServerProjectsMac/Cortex
go test ./internal/data/... -v

# Expected output:
# === RUN   TestDatabaseLifecycle
# --- PASS: TestDatabaseLifecycle
# === RUN   TestTrustProfile
# --- PASS: TestTrustProfile
# === RUN   TestSessionOperations
# --- PASS: TestSessionOperations
# === RUN   TestFullTextSearch
# --- PASS: TestFullTextSearch
# PASS
```

## First Program

Create `main.go`:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "path/filepath"
    "time"

    "github.com/google/uuid"
    "github.com/normanking/cortex/internal/data"
    "github.com/normanking/cortex/pkg/types"
)

func main() {
    // 1. Initialize database
    homeDir, _ := os.UserHomeDir()
    dataDir := filepath.Join(homeDir, ".cortex")

    store, err := data.NewDB(dataDir)
    if err != nil {
        log.Fatalf("Failed to open database: %v", err)
    }
    defer store.Close()

    ctx := context.Background()

    // 2. Create a knowledge item
    lesson := &types.KnowledgeItem{
        ID:         uuid.New().String(),
        Type:       types.TypeLesson,
        Title:      "Always check disk space before deploying",
        Content:    "When: Deploying to production\nDo: Run 'df -h' first\nAvoid: Assuming space is available\nBecause: Out of disk causes silent failures",
        Tags:       []string{"deployment", "linux", "troubleshooting"},
        Scope:      types.ScopePersonal,
        AuthorID:   "user-123",
        AuthorName: "Demo User",
        Confidence: 0.85,
        TrustScore: 0.75,
        SyncStatus: "local_only",
        CreatedAt:  time.Now(),
        UpdatedAt:  time.Now(),
    }

    if err := store.CreateKnowledge(ctx, lesson); err != nil {
        log.Fatalf("Failed to create knowledge: %v", err)
    }

    fmt.Printf("✅ Created lesson: %s\n", lesson.ID)

    // 3. Search for it
    results, err := store.SearchKnowledgeFTS(ctx, "deployment disk", 5)
    if err != nil {
        log.Fatalf("Failed to search: %v", err)
    }

    fmt.Printf("📚 Found %d results\n", len(results))
    for _, item := range results {
        fmt.Printf("  - %s (confidence: %.2f, trust: %.2f)\n",
            item.Title, item.Confidence, item.TrustScore)
    }

    // 4. Update trust score
    if err := store.UpdateTrustScore(ctx, "user-123", "linux", true); err != nil {
        log.Fatalf("Failed to update trust: %v", err)
    }

    profile, err := store.GetTrustProfile(ctx, "user-123", "linux")
    if err != nil {
        log.Fatalf("Failed to get trust profile: %v", err)
    }

    fmt.Printf("🎯 Trust score for user-123 in linux: %.2f\n", profile.Score)

    // 5. Create a session
    sessionID := uuid.New().String()
    session := &types.Session{
        ID:             sessionID,
        UserID:         "user-123",
        Title:          "Deployment troubleshooting",
        CWD:            "/home/user/project",
        PlatformVendor: "linux",
        PlatformName:   "ubuntu",
        Status:         "active",
        StartedAt:      time.Now(),
        LastActivityAt: time.Now(),
    }

    if err := store.CreateSession(ctx, session); err != nil {
        log.Fatalf("Failed to create session: %v", err)
    }

    fmt.Printf("💬 Created session: %s\n", sessionID)

    // 6. Add messages
    messages := []*types.SessionMessage{
        {
            SessionID: sessionID,
            Role:      "user",
            Content:   "Why is my deployment failing?",
            CreatedAt: time.Now(),
        },
        {
            SessionID: sessionID,
            Role:      "assistant",
            Content:   "Let me help you troubleshoot. First, have you checked disk space with 'df -h'?",
            CreatedAt: time.Now().Add(1 * time.Second),
        },
    }

    for _, msg := range messages {
        if err := store.AddMessage(ctx, msg); err != nil {
            log.Fatalf("Failed to add message: %v", err)
        }
    }

    fmt.Printf("📝 Added %d messages to session\n", len(messages))

    // 7. Get session history
    history, err := store.GetSessionMessages(ctx, sessionID)
    if err != nil {
        log.Fatalf("Failed to get history: %v", err)
    }

    fmt.Println("\n📜 Session history:")
    for _, msg := range history {
        fmt.Printf("  [%s]: %s\n", msg.Role, msg.Content)
    }

    fmt.Println("\n✨ All operations completed successfully!")
}
```

Run it:

```bash
go run main.go
```

Expected output:

```
✅ Created lesson: 550e8400-e29b-41d4-a716-446655440000
📚 Found 1 results
  - Always check disk space before deploying (confidence: 0.85, trust: 0.75)
🎯 Trust score for user-123 in linux: 0.53
💬 Created session: 660e8400-e29b-41d4-a716-446655440000
📝 Added 2 messages to session

📜 Session history:
  [user]: Why is my deployment failing?
  [assistant]: Let me help you troubleshoot. First, have you checked disk space with 'df -h'?

✨ All operations completed successfully!
```

## Database Location

After running, check the database:

```bash
ls -lh ~/.cortex/
# knowledge.db       <- Main database
# knowledge.db-shm   <- Shared memory (WAL mode)
# knowledge.db-wal   <- Write-Ahead Log
```

## Inspect with SQLite CLI

```bash
sqlite3 ~/.cortex/knowledge.db

# Useful queries:
.tables                              # List all tables
.schema knowledge_items              # Show table schema
SELECT * FROM knowledge_items;       # View all knowledge
SELECT * FROM trust_profiles;        # View trust scores
SELECT * FROM sessions;              # View sessions
SELECT * FROM v_recent_activity;     # View recent activity
```

## Common Operations

### Create Knowledge

```go
item := &types.KnowledgeItem{
    ID:         uuid.New().String(),
    Type:       types.TypeSOP,        // or TypeLesson, TypePattern, TypeDocument
    Title:      "How to restart nginx",
    Content:    "sudo systemctl restart nginx",
    Tags:       []string{"nginx", "linux"},
    Scope:      types.ScopePersonal,  // or ScopeTeam, ScopeGlobal
    AuthorID:   "user-id",
    Confidence: 0.9,
    TrustScore: 0.8,
    SyncStatus: "local_only",
    CreatedAt:  time.Now(),
    UpdatedAt:  time.Now(),
}

err := store.CreateKnowledge(ctx, item)
```

### Search Knowledge

```go
// Full-text search
results, err := store.SearchKnowledgeFTS(ctx, "nginx restart", 10)

// Filtered search
results, err := store.ListKnowledge(ctx, types.SearchOptions{
    Types:    []string{"lesson", "sop"},
    MinTrust: 0.7,
    Limit:    5,
})
```

### Manage Sessions

```go
// Create session
sessionID := uuid.New().String()
session := &types.Session{
    ID:        sessionID,
    UserID:    "user-123",
    CWD:       cwd,
    Status:    "active",
    StartedAt: time.Now(),
    LastActivityAt: time.Now(),
}
err := store.CreateSession(ctx, session)

// Add message
msg := &types.SessionMessage{
    SessionID: sessionID,
    Role:      "user",
    Content:   "How do I debug this?",
    CreatedAt: time.Now(),
}
err := store.AddMessage(ctx, msg)

// Get history
history, err := store.GetSessionMessages(ctx, sessionID)
```

### Track Trust

```go
// Record success
err := store.UpdateTrustScore(ctx, "user-123", "linux", true)

// Record failure
err := store.UpdateTrustScore(ctx, "user-123", "docker", false)

// Get profile
profile, err := store.GetTrustProfile(ctx, "user-123", "linux")
fmt.Printf("Trust score: %.2f (successes: %d, failures: %d)\n",
    profile.Score, profile.SuccessCount, profile.FailureCount)
```

## Troubleshooting

### "Database is locked"

```go
// Increase timeout in db.go
PRAGMA busy_timeout = 10000;  // 10 seconds
```

### "No such table"

```bash
# Re-run migration
rm ~/.cortex/knowledge.db
# Run your program again - it will recreate the database
```

### "FTS index out of sync"

```sql
-- Rebuild FTS index
INSERT INTO knowledge_fts(knowledge_fts) VALUES('rebuild');
```

## Next Steps

1. ✅ **You are here** - Basic data layer working
2. 📖 Read [README.md](./README.md) - Full API documentation
3. 🔧 Read [INTEGRATION.md](./INTEGRATION.md) - How to integrate into your app
4. 🧪 Review [example_test.go](./example_test.go) - More examples
5. 🗄️ Study [schema](./migrations/001_initial_schema.sql) - Database structure

## Resources

- **SQLite Docs**: https://www.sqlite.org/docs.html
- **FTS5 Guide**: https://www.sqlite.org/fts5.html
- **modernc.org/sqlite**: https://pkg.go.dev/modernc.org/sqlite
- **WAL Mode**: https://www.sqlite.org/wal.html

## Getting Help

If you encounter issues:

1. Check the [README.md](./README.md) troubleshooting section
2. Run the test suite: `go test ./internal/data/... -v`
3. Check database health: `store.Health()`
4. Inspect database directly: `sqlite3 ~/.cortex/knowledge.db`

Happy coding! 🚀
