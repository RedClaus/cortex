---
project: Cortex
component: Unknown
phase: Design
date_created: 2026-01-17T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:18:14.518294
---

# CR-094: Core Session Management

**Status:** Draft
**Phase:** 1
**Priority:** P0
**Estimated Effort:** 3-5 days
**Created:** 2026-01-17

---

## Summary

Implement the foundational session management system that enables users to create, resume, list, and archive brainstorming sessions. Sessions must persist across application restarts and survive memory compaction through file-based state recovery.

---

## Requirements

### Functional Requirements

1. **Create Session**
   - Accept session name and target project folder path
   - Generate unique session ID (UUID)
   - Initialize session state with status "created"
   - Create session directory in `~/.cortex-evaluator/sessions/{session_id}/`

2. **Resume Session**
   - List available sessions with status and last accessed time
   - Load full session state from storage
   - Restore context from context.md, todos.md, insights.md files
   - Update last_accessed_at timestamp

3. **List Sessions**
   - Display all sessions with: name, project path, status, created date, last accessed
   - Support filtering by status (active, archived)
   - Sort by last accessed (default) or created date

4. **Archive Session**
   - Mark session as archived
   - Retain all data for future reference
   - Exclude from active session list by default

5. **State Persistence**
   - Save session state to SQLite database
   - Create/update context.md, todos.md, insights.md on state changes
   - Implement save-before-compaction hook
   - Implement load-after-compaction recovery

---

## Technical Design

### Data Models

```go
// internal/session/types.go

type Session struct {
    ID              string          `json:"id"`
    Name            string          `json:"name"`
    ProjectPath     string          `json:"project_path"`
    Status          SessionStatus   `json:"status"`
    CreatedAt       time.Time       `json:"created_at"`
    UpdatedAt       time.Time       `json:"updated_at"`
    LastAccessedAt  time.Time       `json:"last_accessed_at"`

    // File paths for compaction recovery
    ContextFile     string          `json:"context_file"`
    TodosFile       string          `json:"todos_file"`
    InsightsFile    string          `json:"insights_file"`
}

type SessionStatus string

const (
    StatusCreated   SessionStatus = "created"
    StatusIndexing  SessionStatus = "indexing"
    StatusReady     SessionStatus = "ready"
    StatusArchived  SessionStatus = "archived"
)
```

### Package Structure

```
internal/
├── session/
│   ├── manager.go      # SessionManager implementation
│   ├── types.go        # Data types and constants
│   ├── storage.go      # SQLite persistence
│   └── recovery.go     # Memory compaction recovery
├── data/
│   ├── database.go     # SQLite connection and migrations
│   └── schema.sql      # Database schema
```

### API Design

```go
// internal/session/manager.go

type SessionManager interface {
    // CRUD operations
    Create(name, projectPath string) (*Session, error)
    Get(sessionID string) (*Session, error)
    List(filter SessionFilter) ([]Session, error)
    Update(session *Session) error
    Archive(sessionID string) error

    // State management
    SaveState(sessionID string) error
    LoadState(sessionID string) error

    // Compaction hooks
    OnBeforeCompaction(sessionID string) error
    OnAfterCompaction(sessionID string) error
}

type SessionFilter struct {
    Status      *SessionStatus
    ProjectPath *string
    SortBy      string  // "last_accessed" | "created_at"
    Limit       int
}
```

### SQLite Schema

```sql
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    project_path TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'created',
    context_file TEXT,
    todos_file TEXT,
    insights_file TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_accessed_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_sessions_status ON sessions(status);
CREATE INDEX idx_sessions_project ON sessions(project_path);
CREATE INDEX idx_sessions_accessed ON sessions(last_accessed_at DESC);
```

### File Storage

```
~/.cortex-evaluator/
├── evaluator.db                    # SQLite database
└── sessions/
    └── {session_id}/
        ├── context.md              # Analysis goal and context
        ├── todos.md                # File analysis checklist
        └── insights.md             # Iterative insights
```

---

## Implementation Tasks

- [ ] Create project structure with go.mod and directories
- [ ] Implement internal/data/database.go with SQLite connection
- [ ] Create database schema and migrations
- [ ] Implement internal/session/types.go with data models
- [ ] Implement internal/session/storage.go for persistence
- [ ] Implement internal/session/manager.go with CRUD operations
- [ ] Implement internal/session/recovery.go for compaction handling
- [ ] Create cmd/evaluator/main.go CLI entry point
- [ ] Add `session new` command
- [ ] Add `session resume` command
- [ ] Add `session list` command
- [ ] Add `session archive` command
- [ ] Write unit tests for session manager
- [ ] Write integration tests for persistence

---

## Files to Create/Modify

| File | Action | Description |
|------|--------|-------------|
| `go.mod` | Create | Go module definition |
| `go.sum` | Create | Dependency checksums |
| `internal/data/database.go` | Create | SQLite connection |
| `internal/data/schema.sql` | Create | Database schema |
| `internal/session/types.go` | Create | Data models |
| `internal/session/storage.go` | Create | Persistence layer |
| `internal/session/manager.go` | Create | Session manager |
| `internal/session/recovery.go` | Create | Compaction recovery |
| `cmd/evaluator/main.go` | Create | CLI entry point |

---

## Acceptance Criteria

- [ ] User can create a new session with `evaluator session new <name> --project <path>`
- [ ] User can list sessions with `evaluator session list`
- [ ] User can resume a session with `evaluator session resume <id>`
- [ ] User can archive a session with `evaluator session archive <id>`
- [ ] Sessions persist across application restarts
- [ ] Session state is saved to context.md, todos.md, insights.md
- [ ] Sessions can be recovered after simulated memory compaction
- [ ] All operations complete within 100ms (excluding I/O)

---

## Dependencies

- None (first CR in sequence)

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| SQLite locking issues | Medium | Use WAL mode, single writer |
| File permission errors | Low | Check permissions on startup |
| Path handling cross-platform | Medium | Use filepath.Clean, test on macOS |
