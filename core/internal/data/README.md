---
project: Cortex
component: Docs
phase: Archive
date_created: 2025-12-18T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:51.727404
---

# Cortex Data Layer

SQLite-based local-first data access layer for the Cortex AI assistant.

## Features

- **Pure Go**: Uses `modernc.org/sqlite` (no CGO dependencies)
- **WAL Mode**: Write-Ahead Logging for concurrent reads
- **Full-Text Search**: FTS5 for fast, fuzzy knowledge retrieval
- **Type-Safe**: Comprehensive Go types from `pkg/types`
- **Context-Aware**: All operations support cancellation and timeouts
- **Soft Deletes**: Knowledge items use tombstones for sync safety
- **Trust Tracking**: Exponential moving average for user reliability scores

## Quick Start

```go
import (
    "context"
    "github.com/normanking/cortex/internal/data"
    "github.com/normanking/cortex/pkg/types"
)

// 1. Initialize database
store, err := data.NewDB("~/.cortex")
if err != nil {
    log.Fatal(err)
}
defer store.Close()

ctx := context.Background()

// 2. Create knowledge
item := &types.KnowledgeItem{
    ID:         "lesson-001",
    Type:       types.TypeLesson,
    Title:      "Always check disk space",
    Content:    "When: Deploying\nDo: Run 'df -h'",
    Tags:       []string{"deployment", "linux"},
    Scope:      types.ScopePersonal,
    AuthorID:   "user-123",
    Confidence: 0.85,
    TrustScore: 0.75,
    SyncStatus: "pending",
}

err = store.CreateKnowledge(ctx, item)

// 3. Search
results, err := store.SearchKnowledgeFTS(ctx, "deployment disk", 10)

// 4. Track trust
err = store.UpdateTrustScore(ctx, "user-123", "linux", true)
```

## Architecture

```
internal/data/
├── db.go              # Database connection and initialization
├── store.go           # CRUD operations for all entities
├── migrations/        # Embedded SQL schema
│   └── 001_initial_schema.sql
└── queries/           # Complex queries (future)
```

## Database Location

**Critical:** The database MUST be on a local filesystem, not a network drive.

- Default: `~/.cortex/knowledge.db`
- WAL files: `knowledge.db-wal`, `knowledge.db-shm`
- Network paths (SMB, NFS) are **rejected** to prevent corruption

## Core Operations

### Knowledge Items

```go
// Create
err := store.CreateKnowledge(ctx, item)

// Read
item, err := store.GetKnowledge(ctx, "lesson-001")

// Update
item.Confidence = 0.95
err := store.UpdateKnowledge(ctx, item)

// Delete (soft delete)
err := store.DeleteKnowledge(ctx, "lesson-001")

// List with filters
results, err := store.ListKnowledge(ctx, types.SearchOptions{
    Types:    []string{"lesson", "sop"},
    MinTrust: 0.7,
    Limit:    10,
})

// Full-text search
results, err := store.SearchKnowledgeFTS(ctx, "docker deployment", 5)
```

### Trust Profiles

```go
// Get profile (creates default if not found)
profile, err := store.GetTrustProfile(ctx, "user-123", "linux")

// Update score based on task outcome
err := store.UpdateTrustScore(ctx, "user-123", "linux", true)  // success
err := store.UpdateTrustScore(ctx, "user-123", "linux", false) // failure

// Score uses exponential moving average (alpha = 0.1)
// Higher recent success rate increases score
```

### Sessions

```go
// Create session
session := &types.Session{
    ID:             "sess-001",
    UserID:         "user-123",
    Title:          "Deployment troubleshooting",
    CWD:            "/home/user/project",
    PlatformVendor: "linux",
    Status:         "active",
    StartedAt:      time.Now(),
    LastActivityAt: time.Now(),
}
err := store.CreateSession(ctx, session)

// Add messages
msg := &types.SessionMessage{
    SessionID: "sess-001",
    Role:      "user",
    Content:   "Why is my deployment failing?",
    CreatedAt: time.Now(),
}
err := store.AddMessage(ctx, msg)

// Get all messages
messages, err := store.GetSessionMessages(ctx, "sess-001")

// Update status
err := store.UpdateSessionStatus(ctx, "sess-001", "completed")
```

## Schema Highlights

### Knowledge Items Table

| Field | Type | Description |
|-------|------|-------------|
| `id` | TEXT | Primary key (ULID/UUID) |
| `type` | TEXT | sop, lesson, pattern, session, document |
| `scope` | TEXT | global, team, personal |
| `confidence` | REAL | 0.0 - 1.0 (quality signal) |
| `trust_score` | REAL | 0.0 - 1.0 (author reliability) |
| `version` | INTEGER | Auto-incremented on updates |
| `sync_status` | TEXT | pending, synced, conflict, local_only |
| `deleted_at` | DATETIME | NULL = active, set = soft deleted |

### Full-Text Search Index

- Uses FTS5 with Porter stemming
- Indexes: `id`, `title`, `content`, `tags`
- Auto-synced via triggers
- Ranked by relevance and trust score

### Trust Profiles

- Track per-user, per-domain reliability
- Score calculation: Exponential Moving Average
  - `new_score = old_score * 0.9 + raw_score * 0.1`
  - Raw score = `success_count / (success_count + failure_count)`
- Used to weight knowledge items in search

## Performance Optimizations

### Connection Settings
```go
db.SetMaxOpenConns(1)    // SQLite single-writer model
db.SetMaxIdleConns(1)
db.SetConnMaxLifetime(0) // Never expire connections
```

### SQLite PRAGMAs
```sql
PRAGMA journal_mode = WAL;          -- Concurrent reads
PRAGMA synchronous = NORMAL;        -- Balanced safety
PRAGMA cache_size = -64000;         -- 64MB cache
PRAGMA mmap_size = 268435456;       -- 256MB memory-mapped I/O
PRAGMA temp_store = MEMORY;         -- In-memory temp tables
PRAGMA auto_vacuum = INCREMENTAL;   -- Gradual space reclaim
```

### Indexing Strategy
- Composite indexes on frequent filters (type, scope, trust_score)
- Partial indexes for pending sync items
- FTS5 for text search (much faster than LIKE)

## Error Handling

All functions return wrapped errors with context:

```go
item, err := store.GetKnowledge(ctx, "invalid-id")
if err != nil {
    // Error format: "operation: details: root cause"
    log.Printf("Failed to get knowledge: %v", err)

    // Check for not found
    if strings.Contains(err.Error(), "not found") {
        // Handle not found case
    }
}
```

## Testing

Run the test suite:

```bash
go test ./internal/data/...
```

Example test output:
```
=== RUN   TestDatabaseLifecycle
--- PASS: TestDatabaseLifecycle (0.05s)
=== RUN   TestTrustProfile
--- PASS: TestTrustProfile (0.03s)
=== RUN   TestSessionOperations
--- PASS: TestSessionOperations (0.04s)
=== RUN   TestFullTextSearch
--- PASS: TestFullTextSearch (0.06s)
```

## Health Checks

```go
// Verify database is responsive
if err := store.Health(); err != nil {
    log.Printf("Database unhealthy: %v", err)
    // Trigger reconnect or alert
}
```

## Transaction Support

For atomic multi-operation updates:

```go
err := store.WithTx(ctx, func(tx *sql.Tx) error {
    // Multiple operations here
    _, err := tx.ExecContext(ctx, "INSERT INTO ...")
    if err != nil {
        return err // Automatic rollback
    }

    _, err = tx.ExecContext(ctx, "UPDATE ...")
    return err
})
// Transaction committed if no error returned
```

## Migration Strategy

Migrations are embedded in the binary using `//go:embed`:

```go
//go:embed migrations/001_initial_schema.sql
var initialSchema string
```

The `Migrate()` function:
1. Splits multi-statement SQL
2. Runs in a transaction
3. Idempotent (safe to run multiple times)
4. Tracks applied migrations in `migrations` table

Future migrations:
- Add `002_add_feature.sql`
- Update embed directive
- Add version check in `Migrate()`

## Security Considerations

1. **Local Storage Only**: Network paths are rejected
2. **No SQL Injection**: All queries use prepared statements
3. **Soft Deletes**: Prevents accidental data loss
4. **Foreign Keys**: Enforced at database level
5. **Context Timeouts**: Prevent runaway queries

## Future Enhancements

- [ ] Query builder for complex searches
- [ ] Bulk operations for sync efficiency
- [ ] Conflict resolution merge strategies
- [ ] Metrics/observability hooks
- [ ] Database vacuum scheduling
- [ ] Backup/restore utilities
- [ ] Migration rollback support

## Troubleshooting

### Database Locked Errors

```
database is locked (5)
```

**Cause:** Another process has the database open, or WAL checkpoint is running.

**Solution:** Increase `busy_timeout` or ensure single-process access.

### Performance Degradation

```
Queries getting slower over time
```

**Cause:** WAL file growing too large, or fragmentation.

**Solution:**
```go
store.DB().Exec("PRAGMA wal_checkpoint(TRUNCATE)")
store.DB().Exec("PRAGMA incremental_vacuum")
```

### FTS Index Out of Sync

```
Search results missing recent items
```

**Cause:** Trigger failure or manual data modification.

**Solution:**
```sql
INSERT INTO knowledge_fts(knowledge_fts) VALUES('rebuild');
```

## References

- [SQLite WAL Mode](https://www.sqlite.org/wal.html)
- [FTS5 Documentation](https://www.sqlite.org/fts5.html)
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)
- [Cortex Types](../../pkg/types/types.go)
- [Schema DDL](./migrations/001_initial_schema.sql)
