---
project: Cortex
component: Docs
phase: Ideation
date_created: 2026-01-17T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:18:14.467676
---

# CR-095: Project Indexer with Prep Prompt

**Status:** Draft
**Phase:** 2
**Priority:** P0
**Estimated Effort:** 5-7 days
**Created:** 2026-01-17
**Depends On:** CR-094

---

## Summary

Implement the project indexing system that scans and analyzes project folders using the "prep prompt". The indexer creates context.md, todos.md, and insights.md files, analyzes each file for purpose/communication/protocols, and maintains progress through memory compaction events.

---

## Requirements

### Functional Requirements

1. **Start Indexing**
   - Accept session ID and indexing configuration
   - Create initial context.md with analysis goal
   - Create todos.md with file list to analyze
   - Create empty insights.md for iterative updates

2. **File Analysis**
   - Traverse project directory respecting include/exclude patterns
   - For each file, extract:
     - Exact code purpose
     - Communication method (HTTP, gRPC, WebSocket, events, IPC)
     - Module affinity (related modules)
     - Messaging structure
     - Protocols used
     - API structure
     - Notes and future work
     - Current state

3. **Progress Tracking**
   - Update todos.md as each file is processed
   - Update insights.md after each component
   - Report progress percentage to caller
   - Support pause/resume

4. **Compaction Recovery**
   - Before compaction: flush current state to files
   - After compaction: read context.md and todos.md to continue

5. **Completion**
   - Generate final summary in insights.md
   - Update session status to "ready"
   - Store index state in session

---

## Technical Design

### The Prep Prompt

```go
const PrepPrompt = `
I want you to analyze all the files in this folder to understand:
1. What this project is for
2. How it is constructed
3. How the system communicates
4. What the most recent changes have been

Before you start:
1. Create a context markdown file with the goal: extracting the main context of this codebase
2. Create a todos markdown file to track which files you've analyzed and findings
3. Create an insights markdown file to iteratively update after processing each component

As you work:
- Iteratively update the insights file after processing each item
- Check off each item in todos as you complete them
- Make sure todos is updated before your memory gets compacted
- After any memory compaction, read context and todos files before continuing

For each item, extract:
- Exact code purpose
- Communication method
- Module affinity
- Messaging structure
- Protocols used
- API structure
- Notes and future work
- Current state if available

Work through all files until complete.
`
```

### Data Models

```go
// internal/indexer/types.go

type IndexConfig struct {
    IncludePatterns []string   // e.g., ["*.go", "*.md", "*.yaml"]
    ExcludePatterns []string   // e.g., ["vendor/*", "node_modules/*", ".git/*"]
    MaxFileSize     int64      // Skip files larger than this (bytes)
    MaxDepth        int        // Directory traversal depth (0 = unlimited)
}

type IndexState struct {
    TotalFiles      int                   `json:"total_files"`
    ProcessedFiles  int                   `json:"processed_files"`
    CurrentFile     string                `json:"current_file"`
    StartedAt       time.Time             `json:"started_at"`
    CompletedAt     *time.Time            `json:"completed_at"`
    Status          IndexStatus           `json:"status"`

    // Extracted information
    ProjectPurpose  string                `json:"project_purpose"`
    Architecture    string                `json:"architecture"`
    Communications  []CommunicationMethod `json:"communications"`
    RecentChanges   []RecentChange        `json:"recent_changes"`
    ModuleAffinity  map[string][]string   `json:"module_affinity"`

    // Per-file analysis
    FileAnalyses    []FileAnalysis        `json:"file_analyses"`
}

type IndexStatus string

const (
    IndexPending    IndexStatus = "pending"
    IndexRunning    IndexStatus = "running"
    IndexPaused     IndexStatus = "paused"
    IndexCompleted  IndexStatus = "completed"
    IndexFailed     IndexStatus = "failed"
)

type FileAnalysis struct {
    Path            string    `json:"path"`
    Purpose         string    `json:"purpose"`
    Communication   string    `json:"communication"`
    ModuleAffinity  []string  `json:"module_affinity"`
    Messaging       string    `json:"messaging"`
    Protocols       []string  `json:"protocols"`
    APIStructure    string    `json:"api_structure"`
    Notes           string    `json:"notes"`
    FutureWork      string    `json:"future_work"`
    CurrentState    string    `json:"current_state"`
    AnalyzedAt      time.Time `json:"analyzed_at"`
}

type CommunicationMethod struct {
    Type        string   `json:"type"`
    Endpoints   []string `json:"endpoints"`
    Protocols   []string `json:"protocols"`
}

type RecentChange struct {
    File        string    `json:"file"`
    Description string    `json:"description"`
    Date        time.Time `json:"date"`
}
```

### Package Structure

```
internal/
├── indexer/
│   ├── indexer.go      # Main ProjectIndexer
│   ├── types.go        # Data types
│   ├── scanner.go      # File system traversal
│   ├── analyzer.go     # LLM-based file analysis
│   ├── files.go        # context.md, todos.md, insights.md management
│   └── recovery.go     # Compaction recovery
```

### API Design

```go
// internal/indexer/indexer.go

type ProjectIndexer interface {
    // Start indexing a session's project
    Start(session *Session, config IndexConfig) error

    // Get current progress
    GetProgress(sessionID string) (*IndexProgress, error)

    // Pause/resume indexing
    Pause(sessionID string) error
    Resume(sessionID string) error

    // Cancel indexing
    Cancel(sessionID string) error

    // Compaction hooks
    OnBeforeCompaction(sessionID string) error
    OnAfterCompaction(sessionID string) error
}

type IndexProgress struct {
    TotalFiles      int
    ProcessedFiles  int
    CurrentFile     string
    PercentComplete float64
    Status          IndexStatus
    EstimatedTimeRemaining time.Duration
}
```

### File Templates

**context.md Template:**
```markdown
# Codebase Analysis Context

## Goal
Extract the main context of this codebase to enable informed brainstorming and planning.

## Project
- **Path:** {project_path}
- **Session:** {session_name}
- **Started:** {timestamp}

## Analysis Focus
1. What this project is for
2. How it is constructed
3. How the system communicates
4. What the most recent changes have been

## Recovery Instructions
If memory was compacted, read this file and todos.md before continuing analysis.
```

**todos.md Template:**
```markdown
# File Analysis Checklist

## Progress
- Total Files: {total}
- Processed: {processed}
- Remaining: {remaining}

## Files

- [ ] path/to/file1.go
- [ ] path/to/file2.go
- [x] path/to/analyzed.go ✓
...

## Last Updated
{timestamp}
```

**insights.md Template:**
```markdown
# Codebase Insights

## Project Overview
{iteratively updated}

## Architecture
{iteratively updated}

## Communication Patterns
{iteratively updated}

## Key Components
{iteratively updated}

## Recent Changes
{iteratively updated}

## Notes
{iteratively updated}

---
*Last updated: {timestamp}*
```

---

## Implementation Tasks

- [ ] Implement internal/indexer/types.go with all data models
- [ ] Implement internal/indexer/scanner.go for file traversal
- [ ] Implement internal/indexer/files.go for md file management
- [ ] Implement internal/indexer/analyzer.go for LLM analysis
- [ ] Implement internal/indexer/indexer.go main coordinator
- [ ] Implement internal/indexer/recovery.go for compaction handling
- [ ] Add `session index` command to CLI
- [ ] Add progress display during indexing
- [ ] Implement pause/resume functionality
- [ ] Handle large files (skip or chunk)
- [ ] Handle binary files (skip with note)
- [ ] Write unit tests for scanner
- [ ] Write unit tests for file management
- [ ] Write integration tests for full indexing flow

---

## Files to Create/Modify

| File | Action | Description |
|------|--------|-------------|
| `internal/indexer/types.go` | Create | Data models |
| `internal/indexer/scanner.go` | Create | File traversal |
| `internal/indexer/files.go` | Create | MD file management |
| `internal/indexer/analyzer.go` | Create | LLM analysis |
| `internal/indexer/indexer.go` | Create | Main coordinator |
| `internal/indexer/recovery.go` | Create | Compaction recovery |
| `internal/session/manager.go` | Modify | Add index state |
| `cmd/evaluator/main.go` | Modify | Add index command |

---

## Acceptance Criteria

- [ ] Indexing starts with `evaluator session index <id>`
- [ ] context.md is created with analysis goal
- [ ] todos.md tracks all files with checkboxes
- [ ] insights.md is updated after each file
- [ ] Progress is displayed: `[████░░░░░░] 40% - analyzing internal/foo.go`
- [ ] Indexing can be paused with Ctrl+C and resumed
- [ ] After simulated compaction, indexing continues from checkpoint
- [ ] Session status is "ready" after indexing completes
- [ ] Binary files and vendor directories are skipped
- [ ] Indexing completes for 100+ file project in < 5 minutes

---

## Dependencies

- CR-094: Core Session Management
- LLM provider (Ollama, Anthropic, etc.) for analysis

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Large codebases slow | High | Parallel file reading, batch LLM calls |
| LLM rate limits | Medium | Implement backoff, use local Ollama |
| Memory usage | Medium | Stream files, don't load all at once |
| Inconsistent analysis | Low | Structured prompts, validation |
