---
project: Cortex
component: Docs
phase: Ideation
date_created: 2026-01-17T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:18:14.508477
---

# CR-097: PRD and CR Generation

**Status:** Draft
**Phase:** 4
**Priority:** P0
**Estimated Effort:** 3-5 days
**Created:** 2026-01-17
**Depends On:** CR-094, CR-095, CR-096

---

## Summary

Implement the PRD (Product Requirements Document) and CR (Change Request) generation system. Users can generate PRDs from brainstorming insights, save them to prd.json in the project folder, create CRs with implementation details, and have all operations tracked in searchable history.

---

## Requirements

### Functional Requirements

1. **PRD Generation**
   - Generate PRD from brainstorming session insights
   - Include goals, non-goals, requirements with acceptance criteria
   - Follow consistent PRD format (matching prd.json schema)
   - Allow iterative refinement before saving

2. **PRD Management**
   - Save PRD to prd.json in target project folder
   - Update existing PRD with new requirements
   - Version PRDs (increment on each update)
   - Record all PRD operations in history

3. **CR Generation**
   - Generate CRs from PRD requirements
   - Include title, description, acceptance criteria
   - Estimate effort (S/M/L/XL)
   - Identify files to modify
   - Support multiple CRs per PRD

4. **CR Validation**
   - Validate CR feasibility with Claude Code
   - Check for conflicts with existing code
   - Suggest improvements or clarifications
   - Return validation result with concerns

5. **History Tracking**
   - Record every PRD creation/update
   - Record every CR creation
   - Store timestamp, session reference, details
   - Enable querying by project, session, or text

---

## Technical Design

### Data Models

```go
// internal/prd/types.go

type PRD struct {
    ID           string        `json:"id"`
    SessionID    string        `json:"session_id"`
    ProjectPath  string        `json:"project_path"`
    Version      int           `json:"version"`

    // Content
    Title        string        `json:"title"`
    Summary      string        `json:"summary"`
    Goals        []string      `json:"goals"`
    NonGoals     []string      `json:"non_goals"`
    Requirements []Requirement `json:"requirements"`

    // Execution plan
    ChangeRequests []CRReference `json:"change_requests"`

    // Metadata
    CreatedAt    time.Time     `json:"created_at"`
    UpdatedAt    time.Time     `json:"updated_at"`
    CreatedBy    string        `json:"created_by"`
}

type Requirement struct {
    ID          string   `json:"id"`        // REQ-XXX
    Description string   `json:"description"`
    Priority    string   `json:"priority"`  // P0, P1, P2
    Acceptance  []string `json:"acceptance_criteria"`
}

type CRReference struct {
    CRID   string `json:"cr_id"`
    Title  string `json:"title"`
    Status string `json:"status"`
}

// internal/cr/types.go

type ChangeRequest struct {
    ID                 string     `json:"id"`  // CR-XXX
    SessionID          string     `json:"session_id"`
    PRDID              string     `json:"prd_id"`

    // Content
    Title              string     `json:"title"`
    Description        string     `json:"description"`
    Requirements       []string   `json:"requirements"`
    AcceptanceCriteria []string   `json:"acceptance_criteria"`

    // Implementation details
    FilesToModify      []string   `json:"files_to_modify"`
    EstimatedEffort    string     `json:"estimated_effort"` // S, M, L, XL

    // State
    Status             CRStatus   `json:"status"`
    ValidationResult   *ValidationResult `json:"validation_result,omitempty"`

    // Metadata
    CreatedAt          time.Time  `json:"created_at"`
    StartedAt          *time.Time `json:"started_at,omitempty"`
    CompletedAt        *time.Time `json:"completed_at,omitempty"`
}

type CRStatus string

const (
    CRDraft      CRStatus = "draft"
    CRValidated  CRStatus = "validated"
    CRInProgress CRStatus = "in_progress"
    CRCompleted  CRStatus = "completed"
    CRFailed     CRStatus = "failed"
)

type ValidationResult struct {
    Valid      bool     `json:"valid"`
    Concerns   []string `json:"concerns,omitempty"`
    Suggestions []string `json:"suggestions,omitempty"`
    ValidatedAt time.Time `json:"validated_at"`
}

// internal/history/types.go

type HistoryEntry struct {
    ID          string       `json:"id"`
    Type        HistoryType  `json:"type"`
    SessionID   string       `json:"session_id"`
    SessionName string       `json:"session_name"`
    ProjectPath string       `json:"project_path"`
    PRDID       *string      `json:"prd_id,omitempty"`
    CRID        *string      `json:"cr_id,omitempty"`
    Title       string       `json:"title"`
    Summary     string       `json:"summary"`
    CreatedAt   time.Time    `json:"created_at"`
}

type HistoryType string

const (
    HistoryPRDCreated  HistoryType = "prd_created"
    HistoryPRDUpdated  HistoryType = "prd_updated"
    HistoryCRCreated   HistoryType = "cr_created"
    HistoryCRValidated HistoryType = "cr_validated"
    HistoryCRCompleted HistoryType = "cr_completed"
)
```

### Package Structure

```
internal/
├── prd/
│   ├── generator.go    # PRD generation from session
│   ├── types.go        # PRD data types
│   ├── file.go         # prd.json file operations
│   └── storage.go      # Database persistence
├── cr/
│   ├── generator.go    # CR generation from PRD
│   ├── types.go        # CR data types
│   ├── validator.go    # Claude Code validation
│   └── storage.go      # Database persistence
├── history/
│   ├── service.go      # History tracking service
│   ├── types.go        # History data types
│   └── storage.go      # Database persistence
```

### API Design

```go
// internal/prd/generator.go

type PRDGenerator interface {
    // Generate PRD from session insights
    Generate(session *Session) (*PRD, error)

    // Update existing PRD
    Update(prdID string, changes PRDChanges) (*PRD, error)

    // Save PRD to file
    SaveToFile(prd *PRD, projectPath string) error

    // Load PRD from file
    LoadFromFile(projectPath string) (*PRD, error)

    // Get PRD by ID
    Get(prdID string) (*PRD, error)

    // List PRDs for session
    List(sessionID string) ([]PRD, error)
}

// internal/cr/generator.go

type CRGenerator interface {
    // Generate CRs from PRD
    Generate(prd *PRD) ([]ChangeRequest, error)

    // Validate CR with Claude Code
    Validate(cr *ChangeRequest) (*ValidationResult, error)

    // Get CR by ID
    Get(crID string) (*ChangeRequest, error)

    // List CRs for session
    List(sessionID string) ([]ChangeRequest, error)

    // Update CR status
    UpdateStatus(crID string, status CRStatus) error
}

// internal/history/service.go

type HistoryService interface {
    // Record operations
    RecordPRDCreation(prd *PRD) error
    RecordPRDUpdate(prd *PRD, changes string) error
    RecordCRCreation(cr *ChangeRequest) error
    RecordCRValidation(cr *ChangeRequest, result *ValidationResult) error
    RecordCRCompletion(cr *ChangeRequest) error

    // Query operations
    Query(filter HistoryFilter) ([]HistoryEntry, error)
    GetBySession(sessionID string) ([]HistoryEntry, error)
    GetByProject(projectPath string) ([]HistoryEntry, error)
    Search(query string) ([]HistoryEntry, error)
}

type HistoryFilter struct {
    Type        *HistoryType
    SessionID   *string
    ProjectPath *string
    Since       *time.Time
    Until       *time.Time
    Limit       int
}
```

### SQLite Schema

```sql
-- PRDs table
CREATE TABLE IF NOT EXISTS prds (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id),
    project_path TEXT NOT NULL,
    version INTEGER DEFAULT 1,
    content TEXT NOT NULL,  -- Full PRD as JSON
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_prds_session ON prds(session_id);
CREATE INDEX idx_prds_project ON prds(project_path);

-- Change Requests table
CREATE TABLE IF NOT EXISTS change_requests (
    id TEXT PRIMARY KEY,  -- CR-XXX
    session_id TEXT NOT NULL REFERENCES sessions(id),
    prd_id TEXT REFERENCES prds(id),
    title TEXT NOT NULL,
    content TEXT NOT NULL,  -- Full CR as JSON
    status TEXT NOT NULL DEFAULT 'draft',
    validation_result TEXT,  -- JSON
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    started_at DATETIME,
    completed_at DATETIME
);

CREATE INDEX idx_crs_session ON change_requests(session_id);
CREATE INDEX idx_crs_prd ON change_requests(prd_id);
CREATE INDEX idx_crs_status ON change_requests(status);

-- History table
CREATE TABLE IF NOT EXISTS history (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    session_id TEXT NOT NULL,
    session_name TEXT NOT NULL,
    project_path TEXT NOT NULL,
    prd_id TEXT,
    cr_id TEXT,
    title TEXT NOT NULL,
    summary TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_history_type ON history(type);
CREATE INDEX idx_history_session ON history(session_id);
CREATE INDEX idx_history_project ON history(project_path);
CREATE INDEX idx_history_created ON history(created_at DESC);

-- Full-text search for history
CREATE VIRTUAL TABLE IF NOT EXISTS history_fts USING fts5(
    title, summary,
    content='history',
    content_rowid='rowid'
);
```

### PRD Generation Prompt

```go
const PRDGenerationPrompt = `
Based on the brainstorming session insights, generate a Product Requirements Document.

Session Context:
{session_insights}

Conversation Summary:
{conversation_summary}

Key Decisions:
{key_decisions}

Generate a PRD with:
1. Title - concise project/feature name
2. Summary - 2-3 sentences describing the goal
3. Goals - what we're trying to achieve (3-5 items)
4. Non-Goals - what's explicitly out of scope (2-3 items)
5. Requirements - detailed requirements with:
   - ID (REQ-001, REQ-002, etc.)
   - Description
   - Priority (P0 = critical, P1 = important, P2 = nice-to-have)
   - Acceptance criteria (testable conditions)

Output as JSON matching the PRD schema.
`
```

---

## Implementation Tasks

- [ ] Implement internal/prd/types.go with data models
- [ ] Implement internal/prd/storage.go for database persistence
- [ ] Implement internal/prd/file.go for prd.json operations
- [ ] Implement internal/prd/generator.go for PRD generation
- [ ] Implement internal/cr/types.go with data models
- [ ] Implement internal/cr/storage.go for database persistence
- [ ] Implement internal/cr/generator.go for CR generation
- [ ] Implement internal/cr/validator.go for Claude Code validation
- [ ] Implement internal/history/types.go with data models
- [ ] Implement internal/history/storage.go for persistence
- [ ] Implement internal/history/service.go for tracking
- [ ] Add `/prd create` command
- [ ] Add `/prd update` command
- [ ] Add `/cr create` command
- [ ] Add `/cr validate` command
- [ ] Add `history` command for querying
- [ ] Write unit tests for PRD generator
- [ ] Write unit tests for CR generator
- [ ] Write integration tests for history tracking

---

## Files to Create/Modify

| File | Action | Description |
|------|--------|-------------|
| `internal/prd/types.go` | Create | PRD data models |
| `internal/prd/storage.go` | Create | Database persistence |
| `internal/prd/file.go` | Create | prd.json operations |
| `internal/prd/generator.go` | Create | PRD generation |
| `internal/cr/types.go` | Create | CR data models |
| `internal/cr/storage.go` | Create | Database persistence |
| `internal/cr/generator.go` | Create | CR generation |
| `internal/cr/validator.go` | Create | Claude Code validation |
| `internal/history/types.go` | Create | History data models |
| `internal/history/storage.go` | Create | Database persistence |
| `internal/history/service.go` | Create | History service |
| `internal/data/schema.sql` | Modify | Add new tables |
| `cmd/evaluator/main.go` | Modify | Add PRD/CR/history commands |

---

## Acceptance Criteria

- [ ] User can generate PRD with `/prd create`
- [ ] PRD is saved to prd.json in project folder
- [ ] User can update PRD with `/prd update`
- [ ] PRD versions are tracked
- [ ] User can generate CRs with `/cr create`
- [ ] CRs include files to modify and effort estimate
- [ ] User can validate CR with `/cr validate <id>`
- [ ] Validation returns feasibility and concerns
- [ ] All PRD/CR operations are recorded in history
- [ ] User can query history with `evaluator history`
- [ ] History is searchable by project, session, or text

---

## Dependencies

- CR-094: Core Session Management
- CR-095: Project Indexer
- CR-096: Brainstorm Q&A
- Claude Code CLI for validation

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| PRD quality varies | Medium | Structured prompts, templates |
| CR validation slow | Low | Async validation, caching |
| History grows large | Low | Pagination, archival |
| prd.json conflicts | Medium | Backup before write, versioning |
