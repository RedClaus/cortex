---
project: Cortex
component: Brain Kernel
phase: Ideation
date_created: 2026-01-17T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:18:14.427294
---

# Brainstorm Evaluator Design Document

**Version:** 1.0
**Date:** 2026-01-17
**Status:** Draft
**Author:** Norman King + Claude

---

## Executive Summary

The **Brainstorm Evaluator** is a new Cortex-03 feature that provides an interactive codebase analysis and planning environment. Users can create brainstorming sessions tied to project folders, have AI-assisted Q&A with full codebase context, and seamlessly transition from ideation to executable Change Requests (CRs).

### Core Value Proposition
- **Jump into evaluator mode instantly** with pre-indexed codebase knowledge
- **Persistent session state** stored in Cortex memory (survives restarts, memory compaction)
- **End-to-end workflow**: Brainstorm → PRD/CR → Code Execution → Progress Tracking

---

## Table of Contents

1. [System Architecture](#1-system-architecture)
2. [Data Models](#2-data-models)
3. [User Flows](#3-user-flows)
4. [Component Design](#4-component-design)
5. [Storage & Persistence](#5-storage--persistence)
6. [UI/UX Design](#6-uiux-design)
7. [Integration Points](#7-integration-points)
8. [Implementation Plan](#8-implementation-plan)

---

## 1. System Architecture

### 1.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        BRAINSTORM EVALUATOR SYSTEM                          │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐  │
│  │   Session   │───▶│   Index/    │───▶│  Brainstorm │───▶│   PRD/CR    │  │
│  │   Create    │    │   Scan      │    │   Q&A       │    │   Creation  │  │
│  └─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘  │
│         │                  │                  │                  │          │
│         ▼                  ▼                  ▼                  ▼          │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                     CORTEX MEMORY LAYER                              │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐            │   │
│  │  │ Sessions │  │ Insights │  │ Artifacts│  │ History  │            │   │
│  │  │  Table   │  │  Table   │  │  Table   │  │  Table   │            │   │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘            │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│         │                                                                   │
│         ▼                                                                   │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                     CODE EXECUTION LAYER                             │   │
│  │  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐          │   │
│  │  │ Claude Code  │───▶│  TODO Watch  │───▶│ Notification │          │   │
│  │  │  Executor    │    │   Service    │    │   Banner     │          │   │
│  │  └──────────────┘    └──────────────┘    └──────────────┘          │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 1.2 Component Overview

| Component | Purpose | Location |
|-----------|---------|----------|
| **Session Manager** | Create/resume/list brainstorm sessions | `internal/brainstorm/session.go` |
| **Project Indexer** | Scan and analyze project folders | `internal/brainstorm/indexer.go` |
| **Context Builder** | Build AI context from indexed data | `internal/brainstorm/context.go` |
| **Insight Tracker** | Track analysis insights iteratively | `internal/brainstorm/insights.go` |
| **Artifact Store** | Store session outputs (text, code, files) | `internal/brainstorm/artifacts.go` |
| **PRD Generator** | Generate/update prd.json files | `internal/brainstorm/prd.go` |
| **CR Manager** | Create and track Change Requests | `internal/brainstorm/cr.go` |
| **Progress Watcher** | Monitor TODO completion in real-time | `internal/brainstorm/progress.go` |
| **History Service** | Query past sessions and CRs | `internal/brainstorm/history.go` |

### 1.3 Integration with Existing Cortex Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    EXISTING CORTEX SYSTEMS                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐    │
│  │   Memory     │     │   Knowledge  │     │   AutoLLM    │    │
│  │   System     │◀───▶│   Fabric     │◀───▶│   Router     │    │
│  │ (3-tier)     │     │  (FTS5+LSH)  │     │  (2-lane)    │    │
│  └──────┬───────┘     └──────────────┘     └──────────────┘    │
│         │                                                        │
│         ▼                                                        │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │              BRAINSTORM EVALUATOR (NEW)                   │   │
│  │                                                            │   │
│  │  - Stores sessions in Personal Memory tier                │   │
│  │  - Uses Knowledge Fabric for codebase indexing            │   │
│  │  - Routes queries through Smart Lane for analysis         │   │
│  │  - Integrates with Agent Tools for file operations        │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## 2. Data Models

### 2.1 Session Model

```go
// BrainstormSession represents a single brainstorming session
type BrainstormSession struct {
    ID            string                 `json:"id"`             // UUID
    Name          string                 `json:"name"`           // User-friendly name
    ProjectPath   string                 `json:"project_path"`   // Target folder
    Status        SessionStatus          `json:"status"`         // created|indexing|ready|archived
    CreatedAt     time.Time              `json:"created_at"`
    UpdatedAt     time.Time              `json:"updated_at"`
    LastAccessedAt time.Time             `json:"last_accessed_at"`

    // Indexing State
    IndexState    *IndexState            `json:"index_state"`

    // Context Files (for memory compaction recovery)
    ContextFile   string                 `json:"context_file"`   // context.md path
    TodosFile     string                 `json:"todos_file"`     // todos.md path
    InsightsFile  string                 `json:"insights_file"`  // insights.md path

    // Session Data
    Messages      []SessionMessage       `json:"messages"`       // Q&A history
    Artifacts     []Artifact             `json:"artifacts"`      // Generated outputs

    // PRD/CR State
    PRDPath       string                 `json:"prd_path"`       // prd.json location
    CRs           []ChangeRequest        `json:"crs"`            // Created CRs
}

type SessionStatus string
const (
    SessionCreated   SessionStatus = "created"
    SessionIndexing  SessionStatus = "indexing"
    SessionReady     SessionStatus = "ready"
    SessionArchived  SessionStatus = "archived"
)
```

### 2.2 Index State Model

```go
// IndexState tracks the project indexing progress
type IndexState struct {
    TotalFiles      int                    `json:"total_files"`
    ProcessedFiles  int                    `json:"processed_files"`
    StartedAt       time.Time              `json:"started_at"`
    CompletedAt     *time.Time             `json:"completed_at"`

    // Extracted Information
    ProjectPurpose  string                 `json:"project_purpose"`
    Architecture    string                 `json:"architecture"`
    Communications  []CommunicationMethod  `json:"communications"`
    RecentChanges   []RecentChange         `json:"recent_changes"`
    ModuleAffinity  map[string][]string    `json:"module_affinity"`

    // File Analysis
    FileAnalyses    []FileAnalysis         `json:"file_analyses"`
}

type FileAnalysis struct {
    Path            string                 `json:"path"`
    Purpose         string                 `json:"purpose"`
    Protocols       []string               `json:"protocols"`
    APIStructure    string                 `json:"api_structure"`
    Notes           string                 `json:"notes"`
    FutureWork      string                 `json:"future_work"`
    CurrentState    string                 `json:"current_state"`
    AnalyzedAt      time.Time              `json:"analyzed_at"`
}

type CommunicationMethod struct {
    Type        string   `json:"type"`        // http|grpc|websocket|event|ipc
    Endpoints   []string `json:"endpoints"`
    Protocols   []string `json:"protocols"`
}

type RecentChange struct {
    File        string    `json:"file"`
    Description string    `json:"description"`
    Date        time.Time `json:"date"`
}
```

### 2.3 Artifact Model

```go
// Artifact represents any output from a brainstorm session
type Artifact struct {
    ID          string        `json:"id"`
    SessionID   string        `json:"session_id"`
    Type        ArtifactType  `json:"type"`
    Name        string        `json:"name"`
    Content     string        `json:"content"`
    FilePath    *string       `json:"file_path"`    // If saved to disk
    CreatedAt   time.Time     `json:"created_at"`
    Iteration   int           `json:"iteration"`    // Which brainstorm iteration
}

type ArtifactType string
const (
    ArtifactText     ArtifactType = "text"
    ArtifactCode     ArtifactType = "code"
    ArtifactMarkdown ArtifactType = "markdown"
    ArtifactDiagram  ArtifactType = "diagram"
    ArtifactPRD      ArtifactType = "prd"
    ArtifactCR       ArtifactType = "cr"
)
```

### 2.4 PRD Model

```go
// PRD represents a Product Requirements Document
type PRD struct {
    ID              string            `json:"id"`
    SessionID       string            `json:"session_id"`
    ProjectPath     string            `json:"project_path"`
    Version         int               `json:"version"`

    // Content
    Title           string            `json:"title"`
    Summary         string            `json:"summary"`
    Goals           []string          `json:"goals"`
    NonGoals        []string          `json:"non_goals"`
    Requirements    []Requirement     `json:"requirements"`

    // Execution Plan
    ChangeRequests  []CRReference     `json:"change_requests"`

    // Metadata
    CreatedAt       time.Time         `json:"created_at"`
    UpdatedAt       time.Time         `json:"updated_at"`
    CreatedBy       string            `json:"created_by"`
}

type Requirement struct {
    ID          string   `json:"id"`
    Description string   `json:"description"`
    Priority    string   `json:"priority"`  // P0|P1|P2
    Acceptance  []string `json:"acceptance_criteria"`
}

type CRReference struct {
    CRID        string   `json:"cr_id"`
    Title       string   `json:"title"`
    Status      string   `json:"status"`
}
```

### 2.5 Change Request Model

```go
// ChangeRequest represents a coding task derived from brainstorming
type ChangeRequest struct {
    ID              string            `json:"id"`           // CR-XXX format
    SessionID       string            `json:"session_id"`
    PRDID           string            `json:"prd_id"`

    // Content
    Title           string            `json:"title"`
    Description     string            `json:"description"`
    Requirements    []string          `json:"requirements"`
    AcceptanceCriteria []string       `json:"acceptance_criteria"`

    // Implementation
    FilesToModify   []string          `json:"files_to_modify"`
    EstimatedEffort string            `json:"estimated_effort"` // S|M|L|XL

    // Execution State
    Status          CRStatus          `json:"status"`
    Progress        *CRProgress       `json:"progress"`

    // Metadata
    CreatedAt       time.Time         `json:"created_at"`
    StartedAt       *time.Time        `json:"started_at"`
    CompletedAt     *time.Time        `json:"completed_at"`
}

type CRStatus string
const (
    CRDraft      CRStatus = "draft"
    CRValidated  CRStatus = "validated"    // Claude Code validated
    CRInProgress CRStatus = "in_progress"
    CRCompleted  CRStatus = "completed"
    CRFailed     CRStatus = "failed"
)

type CRProgress struct {
    TotalTodos      int       `json:"total_todos"`
    CompletedTodos  int       `json:"completed_todos"`
    InProgressTodos int       `json:"in_progress_todos"`
    PendingTodos    int       `json:"pending_todos"`
    PercentComplete float64   `json:"percent_complete"`
    LastUpdated     time.Time `json:"last_updated"`
}
```

### 2.6 History Model

```go
// HistoryEntry tracks all PRD insertions and CR creations
type HistoryEntry struct {
    ID          string        `json:"id"`
    Type        HistoryType   `json:"type"`
    SessionID   string        `json:"session_id"`
    SessionName string        `json:"session_name"`
    ProjectPath string        `json:"project_path"`

    // Reference
    PRDID       *string       `json:"prd_id"`
    CRID        *string       `json:"cr_id"`

    // Details
    Title       string        `json:"title"`
    Summary     string        `json:"summary"`

    // Timing
    CreatedAt   time.Time     `json:"created_at"`
}

type HistoryType string
const (
    HistoryPRDCreated  HistoryType = "prd_created"
    HistoryPRDUpdated  HistoryType = "prd_updated"
    HistoryCRCreated   HistoryType = "cr_created"
    HistoryCRCompleted HistoryType = "cr_completed"
)
```

---

## 3. User Flows

### 3.1 Complete Workflow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          BRAINSTORM EVALUATOR FLOW                          │
└─────────────────────────────────────────────────────────────────────────────┘

    USER                           SYSTEM                         STORAGE
      │                               │                               │
      │  1. Create Session            │                               │
      │──────────────────────────────▶│                               │
      │  "brainstorm cortex-03"       │  Create session record        │
      │                               │──────────────────────────────▶│
      │                               │                               │
      │  2. Select Project Folder     │                               │
      │──────────────────────────────▶│                               │
      │  "/path/to/project"           │                               │
      │                               │                               │
      │                               │  3. Index/Scan Project        │
      │                               │  ┌─────────────────────────┐  │
      │                               │  │ - Create context.md     │  │
      │                               │  │ - Create todos.md       │  │
      │                               │  │ - Create insights.md    │  │
      │                               │  │ - Analyze each file     │  │
      │                               │  │ - Update insights       │  │
      │                               │  │ - Check off todos       │  │
      │  ◀─────Progress Updates───────│  └─────────────────────────┘  │
      │                               │                               │
      │  4. Q&A Session               │                               │
      │──────────────────────────────▶│                               │
      │  "What if I add feature X?"   │  Query with full context      │
      │                               │──────────────────────────────▶│
      │  ◀────────Response────────────│                               │
      │                               │  Store message + artifacts    │
      │                               │──────────────────────────────▶│
      │                               │                               │
      │  5. Attach Files              │                               │
      │──────────────────────────────▶│                               │
      │  [PDF, TXT, Images]           │  Process attachments          │
      │                               │  Add to context               │
      │                               │                               │
      │  6. Multiple Iterations       │                               │
      │  ◀───────────────────────────▶│                               │
      │                               │                               │
      │  7. Create PRD/CR             │                               │
      │──────────────────────────────▶│                               │
      │  "Create a plan for this"     │  Generate PRD                 │
      │                               │  Update prd.json              │
      │                               │  Create CR-XXX                │
      │                               │──────────────────────────────▶│
      │                               │  Record in history            │
      │                               │──────────────────────────────▶│
      │                               │                               │
      │  8. Validate with Claude Code │                               │
      │──────────────────────────────▶│                               │
      │                               │  Run validation               │
      │                               │  Check feasibility            │
      │  ◀────────Validation──────────│                               │
      │                               │                               │
      │  9. Execute CR                │                               │
      │──────────────────────────────▶│                               │
      │                               │  Launch Claude Code           │
      │                               │  Start TODO watcher           │
      │                               │                               │
      │  10. Progress Monitoring      │                               │
      │  ◀─────────────────────────────────Real-time updates──────────│
      │  [████████░░░░] 67%           │                               │
      │                               │                               │
      │  11. Completion Notification  │                               │
      │  ◀────────────────────────────│                               │
      │  🎉 CR-042 Complete!          │                               │
      │                               │                               │
```

### 3.2 Session Resume Flow

```
    USER                           SYSTEM                         STORAGE
      │                               │                               │
      │  "resume brainstorm"          │                               │
      │──────────────────────────────▶│                               │
      │                               │  List active sessions         │
      │                               │──────────────────────────────▶│
      │  ◀────────Session List────────│                               │
      │                               │                               │
      │  Select "cortex-03-jan17"     │                               │
      │──────────────────────────────▶│                               │
      │                               │  Load session state           │
      │                               │  Load context.md              │
      │                               │  Load todos.md                │
      │                               │  Load insights.md             │
      │                               │  Restore message history      │
      │                               │──────────────────────────────▶│
      │                               │                               │
      │  ◀────────Ready──────────────│                               │
      │  "Session restored with       │                               │
      │   full context"               │                               │
      │                               │                               │
```

### 3.3 Memory Compaction Recovery

```
    SYSTEM                                    STORAGE
      │                                          │
      │  [Memory compaction triggered]           │
      │                                          │
      │  Before compaction:                      │
      │  - Flush all state to storage            │
      │  - Update context.md                     │
      │  - Update todos.md                       │
      │  - Update insights.md                    │
      │─────────────────────────────────────────▶│
      │                                          │
      │  [After compaction]                      │
      │                                          │
      │  Recovery sequence:                      │
      │  1. Read context.md                      │
      │◀─────────────────────────────────────────│
      │  2. Read todos.md                        │
      │◀─────────────────────────────────────────│
      │  3. Continue from last checkpoint        │
      │                                          │
```

---

## 4. Component Design

### 4.1 Session Manager

```go
// internal/brainstorm/session.go

type SessionManager struct {
    db          *data.Database
    memory      memory.CoreStore
    indexer     *ProjectIndexer
    llm         llm.Provider
}

// Public API
func (sm *SessionManager) Create(name, projectPath string) (*BrainstormSession, error)
func (sm *SessionManager) Resume(sessionID string) (*BrainstormSession, error)
func (sm *SessionManager) List(filter SessionFilter) ([]BrainstormSession, error)
func (sm *SessionManager) Archive(sessionID string) error

// Session Operations
func (sm *SessionManager) SendMessage(sessionID, message string, attachments []Attachment) (*SessionMessage, error)
func (sm *SessionManager) GetContext(sessionID string) (*SessionContext, error)
func (sm *SessionManager) AddArtifact(sessionID string, artifact Artifact) error

// State Management
func (sm *SessionManager) SaveState(sessionID string) error  // Before compaction
func (sm *SessionManager) LoadState(sessionID string) error  // After compaction
```

### 4.2 Project Indexer

```go
// internal/brainstorm/indexer.go

type ProjectIndexer struct {
    llm         llm.Provider
    fileStore   *data.FileStore
}

// Indexing Configuration
type IndexConfig struct {
    IncludePatterns  []string  // e.g., ["*.go", "*.md", "*.yaml"]
    ExcludePatterns  []string  // e.g., ["vendor/*", "node_modules/*"]
    MaxFileSize      int64     // Skip files larger than this
    MaxDepth         int       // Directory traversal depth
}

// The "Prep Prompt" - System prompt for indexing
const PrepPrompt = `
Analyze all the files in this folder to understand:
1. What this project is for
2. How it is constructed
3. How the system communicates
4. What the most recent changes have been

Before you start:
1. Create a context markdown file with the goal of this analysis
2. Create a todos markdown file to track files analyzed
3. Create an insights markdown file to iteratively update

As you work:
- Update insights file after processing each component
- Check off items in todos as you complete them
- After any memory compaction, read context and todos before continuing

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

// Public API
func (pi *ProjectIndexer) StartIndex(session *BrainstormSession, config IndexConfig) error
func (pi *ProjectIndexer) GetProgress(sessionID string) (*IndexProgress, error)
func (pi *ProjectIndexer) PauseIndex(sessionID string) error
func (pi *ProjectIndexer) ResumeIndex(sessionID string) error
```

### 4.3 PRD Generator

```go
// internal/brainstorm/prd.go

type PRDGenerator struct {
    llm         llm.Provider
    history     *HistoryService
}

// Public API
func (pg *PRDGenerator) Generate(session *BrainstormSession) (*PRD, error)
func (pg *PRDGenerator) Update(prdID string, changes PRDChanges) (*PRD, error)
func (pg *PRDGenerator) SaveToFile(prd *PRD, path string) error
func (pg *PRDGenerator) LoadFromFile(path string) (*PRD, error)

// CR Generation
func (pg *PRDGenerator) GenerateCRs(prd *PRD) ([]ChangeRequest, error)
func (pg *PRDGenerator) ValidateCR(cr *ChangeRequest) (*ValidationResult, error)
```

### 4.4 Progress Watcher

```go
// internal/brainstorm/progress.go

type ProgressWatcher struct {
    todoParser  *TodoParser
    notifier    *NotificationService
    updateChan  chan ProgressUpdate
}

// Watch a CR execution
func (pw *ProgressWatcher) Watch(cr *ChangeRequest) error
func (pw *ProgressWatcher) Stop(crID string) error
func (pw *ProgressWatcher) GetProgress(crID string) (*CRProgress, error)

// Notification callbacks
type ProgressCallback func(update ProgressUpdate)

type ProgressUpdate struct {
    CRID            string
    PercentComplete float64
    Completed       []string  // Completed TODO items
    InProgress      []string  // Currently in progress
    Pending         []string  // Not yet started
    Timestamp       time.Time
}
```

### 4.5 History Service

```go
// internal/brainstorm/history.go

type HistoryService struct {
    db      *data.Database
    memory  memory.CoreStore
}

// Recording
func (hs *HistoryService) RecordPRDCreation(prd *PRD) error
func (hs *HistoryService) RecordPRDUpdate(prd *PRD, changes string) error
func (hs *HistoryService) RecordCRCreation(cr *ChangeRequest) error
func (hs *HistoryService) RecordCRCompletion(cr *ChangeRequest) error

// Querying
func (hs *HistoryService) Query(filter HistoryFilter) ([]HistoryEntry, error)
func (hs *HistoryService) GetSessionHistory(sessionID string) ([]HistoryEntry, error)
func (hs *HistoryService) GetProjectHistory(projectPath string) ([]HistoryEntry, error)
func (hs *HistoryService) Search(query string) ([]HistoryEntry, error)
```

---

## 5. Storage & Persistence

### 5.1 SQLite Schema Extensions

```sql
-- Brainstorm Sessions
CREATE TABLE IF NOT EXISTS brainstorm_sessions (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    project_path TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'created',
    index_state TEXT,  -- JSON blob
    context_file TEXT,
    todos_file TEXT,
    insights_file TEXT,
    prd_path TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_accessed_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_sessions_status ON brainstorm_sessions(status);
CREATE INDEX idx_sessions_project ON brainstorm_sessions(project_path);

-- Session Messages
CREATE TABLE IF NOT EXISTS brainstorm_messages (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES brainstorm_sessions(id),
    role TEXT NOT NULL,  -- user|assistant
    content TEXT NOT NULL,
    attachments TEXT,  -- JSON array
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_messages_session ON brainstorm_messages(session_id);

-- Artifacts
CREATE TABLE IF NOT EXISTS brainstorm_artifacts (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES brainstorm_sessions(id),
    type TEXT NOT NULL,
    name TEXT NOT NULL,
    content TEXT NOT NULL,
    file_path TEXT,
    iteration INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_artifacts_session ON brainstorm_artifacts(session_id);

-- PRDs
CREATE TABLE IF NOT EXISTS brainstorm_prds (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES brainstorm_sessions(id),
    project_path TEXT NOT NULL,
    version INTEGER DEFAULT 1,
    content TEXT NOT NULL,  -- Full PRD as JSON
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_prds_session ON brainstorm_prds(session_id);
CREATE INDEX idx_prds_project ON brainstorm_prds(project_path);

-- Change Requests
CREATE TABLE IF NOT EXISTS brainstorm_crs (
    id TEXT PRIMARY KEY,  -- CR-XXX
    session_id TEXT NOT NULL REFERENCES brainstorm_sessions(id),
    prd_id TEXT REFERENCES brainstorm_prds(id),
    title TEXT NOT NULL,
    description TEXT,
    content TEXT NOT NULL,  -- Full CR as JSON
    status TEXT NOT NULL DEFAULT 'draft',
    progress TEXT,  -- JSON blob
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    started_at DATETIME,
    completed_at DATETIME
);

CREATE INDEX idx_crs_session ON brainstorm_crs(session_id);
CREATE INDEX idx_crs_status ON brainstorm_crs(status);

-- History
CREATE TABLE IF NOT EXISTS brainstorm_history (
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

CREATE INDEX idx_history_type ON brainstorm_history(type);
CREATE INDEX idx_history_session ON brainstorm_history(session_id);
CREATE INDEX idx_history_project ON brainstorm_history(project_path);
CREATE INDEX idx_history_created ON brainstorm_history(created_at);

-- Full-text search for history
CREATE VIRTUAL TABLE IF NOT EXISTS brainstorm_history_fts USING fts5(
    title, summary, content='brainstorm_history', content_rowid='rowid'
);
```

### 5.2 File Storage Structure

```
~/.cortex/
├── brainstorm/
│   ├── sessions/
│   │   └── {session_id}/
│   │       ├── context.md        # Analysis goal and context
│   │       ├── todos.md          # File analysis checklist
│   │       ├── insights.md       # Iterative insights
│   │       └── artifacts/
│   │           ├── artifact_001.md
│   │           ├── artifact_002.go
│   │           └── ...
│   └── history/
│       └── history.json          # Backup of history entries
│
└── knowledge.db                  # SQLite (existing, extended)
```

### 5.3 Cortex Memory Integration

```go
// Store session state in Cortex memory for user recall
func (sm *SessionManager) StoreInMemory(session *BrainstormSession) error {
    // Store in Personal memory tier
    entry := memory.Entry{
        Type:      "brainstorm_session",
        Content:   session.ToSummary(),
        Tags:      []string{"brainstorm", session.Name, session.ProjectPath},
        Metadata: map[string]interface{}{
            "session_id":   session.ID,
            "project_path": session.ProjectPath,
            "status":       session.Status,
            "cr_count":     len(session.CRs),
        },
    }
    return sm.memory.Store(entry)
}
```

---

## 6. UI/UX Design

### 6.1 TUI Commands

```
BRAINSTORM COMMANDS
───────────────────────────────────────────────────────
  brainstorm new <name>           Create new session
  brainstorm resume [id]          Resume existing session
  brainstorm list                 List all sessions
  brainstorm history              View PRD/CR history
  brainstorm status <cr-id>       Check CR execution status

IN-SESSION COMMANDS
───────────────────────────────────────────────────────
  /attach <file>                  Attach file to context
  /artifact <name>                Save current output as artifact
  /prd create                     Generate PRD from session
  /prd update                     Update existing PRD
  /cr create                      Create Change Request
  /cr validate <id>               Validate CR with Claude Code
  /cr execute <id>                Start CR execution
  /cr status <id>                 Check execution progress
  /export                         Export session to markdown
  /end                            End brainstorm session
```

### 6.2 Progress Display

```
┌─────────────────────────────────────────────────────────────────┐
│  CR-042: Add Voice Latency Optimization                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Progress: [████████████░░░░░░░░] 67%                           │
│                                                                  │
│  ✅ Completed (4/6):                                            │
│     • Create VoiceExecutive struct                              │
│     • Implement Groq provider routing                           │
│     • Add latency budget enforcement                            │
│     • Update configuration schema                               │
│                                                                  │
│  🔄 In Progress (1/6):                                          │
│     • Integrate with existing voice adapter                     │
│                                                                  │
│  ⏳ Pending (1/6):                                              │
│     • Add tests and documentation                               │
│                                                                  │
│  Last Updated: 2 minutes ago                                    │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### 6.3 Notification Banner

```
┌─────────────────────────────────────────────────────────────────┐
│  🎉 CR-042 COMPLETE!                                            │
│                                                                  │
│  Voice Latency Optimization finished successfully.              │
│  6/6 tasks completed in 12 minutes.                             │
│                                                                  │
│  [View Details]  [Start Next CR]  [Dismiss]                     │
└─────────────────────────────────────────────────────────────────┘
```

---

## 7. Integration Points

### 7.1 Claude Code Integration

```go
// internal/brainstorm/executor.go

type ClaudeCodeExecutor struct {
    workDir     string
    todoWatcher *ProgressWatcher
}

// Execute CR using Claude Code CLI
func (cce *ClaudeCodeExecutor) Execute(cr *ChangeRequest) error {
    // 1. Prepare CR prompt
    prompt := cce.buildPrompt(cr)

    // 2. Start Claude Code in the project directory
    cmd := exec.Command("claude", "--prompt", prompt)
    cmd.Dir = cr.ProjectPath

    // 3. Start TODO watcher
    cce.todoWatcher.Watch(cr)

    // 4. Execute and stream output
    return cmd.Run()
}

// Validation before execution
func (cce *ClaudeCodeExecutor) Validate(cr *ChangeRequest) (*ValidationResult, error) {
    // Run Claude Code in validation mode
    // Check if CR is feasible
    // Return any concerns or suggestions
}
```

### 7.2 AutoLLM Integration

```go
// Use Smart Lane for analysis, Fast Lane for simple queries
func (sm *SessionManager) routeQuery(query string, context *SessionContext) llm.RouteHint {
    // Analysis queries → Smart Lane
    if isAnalysisQuery(query) {
        return llm.RouteHint{
            Lane:   llm.SmartLane,
            Reason: "Complex codebase analysis requires frontier model",
        }
    }

    // Simple Q&A → Fast Lane with passive retrieval
    return llm.RouteHint{
        Lane:            llm.FastLane,
        EnableRetrieval: true,
    }
}
```

### 7.3 Knowledge Fabric Integration

```go
// Index project into Knowledge Fabric for fast retrieval
func (pi *ProjectIndexer) indexToKnowledge(session *BrainstormSession, analysis FileAnalysis) error {
    entry := knowledge.Entry{
        Type:    "code_analysis",
        Source:  analysis.Path,
        Content: analysis.Purpose,
        Tags:    analysis.Protocols,
        Metadata: map[string]interface{}{
            "session_id": session.ID,
            "api":        analysis.APIStructure,
            "notes":      analysis.Notes,
        },
    }
    return pi.fabric.Index(entry)
}
```

---

## 8. Implementation Plan

### 8.1 Phased Approach

```
Phase 1: Core Session Management (CR-094)
─────────────────────────────────────────
- [ ] Create internal/brainstorm/ package structure
- [ ] Implement SessionManager (create, resume, list)
- [ ] Add SQLite schema extensions
- [ ] Basic TUI integration (brainstorm command)
- [ ] Session state persistence

Phase 2: Project Indexing (CR-095)
──────────────────────────────────
- [ ] Implement ProjectIndexer with prep prompt
- [ ] Create context.md, todos.md, insights.md workflow
- [ ] File analysis extraction
- [ ] Progress tracking during indexing
- [ ] Memory compaction recovery

Phase 3: Brainstorm Q&A (CR-096)
────────────────────────────────
- [ ] Message handling with full context
- [ ] Attachment support (PDF, TXT, images)
- [ ] Artifact creation and storage
- [ ] Multiple iteration support
- [ ] Session export to markdown

Phase 4: PRD/CR Generation (CR-097)
───────────────────────────────────
- [ ] PRD generator with template
- [ ] prd.json file management
- [ ] CR creation from PRD
- [ ] History tracking for all PRDs and CRs
- [ ] History query interface

Phase 5: Execution & Progress (CR-098)
──────────────────────────────────────
- [ ] Claude Code integration
- [ ] CR validation workflow
- [ ] TODO watcher service
- [ ] Real-time progress display
- [ ] Completion notifications

Phase 6: Polish & Integration (CR-099)
──────────────────────────────────────
- [ ] Cortex memory integration
- [ ] Session recall from memory
- [ ] Knowledge Fabric indexing
- [ ] AutoLLM routing optimization
- [ ] Performance tuning
```

### 8.2 Estimated Effort

| Phase | Effort | Dependencies |
|-------|--------|--------------|
| Phase 1 | Medium (3-5 days) | None |
| Phase 2 | Large (5-7 days) | Phase 1 |
| Phase 3 | Medium (3-5 days) | Phase 2 |
| Phase 4 | Medium (3-5 days) | Phase 3 |
| Phase 5 | Large (5-7 days) | Phase 4 |
| Phase 6 | Medium (3-5 days) | Phase 5 |

**Total Estimated: 22-34 days**

### 8.3 Success Criteria

1. **Session Persistence**: Sessions survive restarts and memory compaction
2. **Context Quality**: AI has full codebase understanding after indexing
3. **PRD Accuracy**: Generated PRDs are actionable and complete
4. **CR Execution**: CRs execute successfully via Claude Code
5. **Progress Visibility**: Real-time TODO tracking works reliably
6. **History Queryable**: All past work is searchable and retrievable

---

## Appendix A: Example Session

```
$ cortex brainstorm new "voice-optimization"

🧠 Creating brainstorm session: voice-optimization

Select project folder:
> /Users/normanking/ServerProjectsMac/Development/cortex-03

📁 Indexing project...
   Creating context.md ✓
   Creating todos.md ✓
   Creating insights.md ✓

   Analyzing files... [████████░░░░░░░░░░░░] 42%
   - internal/voice/executive.go ✓
   - internal/voice/budget.go ✓
   - internal/voice/conversation.go (processing...)

✅ Indexing complete! 127 files analyzed.

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
BRAINSTORM SESSION: voice-optimization
Project: cortex-03
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

You: What if I wanted to add support for multiple simultaneous
     voice conversations?

AI: Based on my analysis of the voice system architecture, here's
    what that would involve...

    [Detailed response with architecture options]

You: /attach requirements.pdf

📎 Attached: requirements.pdf (analyzed, added to context)

You: Can I implement option B while maintaining the latency budget?

AI: Yes, here's how...

You: /prd create

📋 PRD Generated: voice-multi-conversation.json

   Created Change Requests:
   - CR-100: Add conversation multiplexer
   - CR-101: Implement session isolation
   - CR-102: Update latency budgeting

   Validate before execution? [Y/n]

You: Y

🔍 Validating with Claude Code...
   ✅ CR-100: Feasible (estimated 2 hours)
   ✅ CR-101: Feasible (estimated 3 hours)
   ⚠️  CR-102: Needs clarification on budget allocation

You: /cr execute CR-100

🚀 Executing CR-100...

   Progress: [████████░░░░░░░░░░░░] 42%

   ✅ Create ConversationMux struct
   🔄 Implement session routing
   ⏳ Add cleanup on disconnect
   ⏳ Write tests
```

---

## Appendix B: Prep Prompt (Full)

```markdown
# Codebase Analysis System Prompt

I want you to analyze all the files in this folder to understand:
1. What this project is for
2. How it is constructed
3. How the system communicates
4. What the most recent changes have been

## Before You Start

1. **Create context.md** - Document the goal of this analysis
2. **Create todos.md** - Track which files you've analyzed and findings
3. **Create insights.md** - Iteratively update after processing each component

## As You Work

- Iteratively update the insights file after processing each item
- Check off each item in todos as you complete them
- Make sure todos is updated before your memory gets compacted
- After any memory compaction, read context.md and todos.md before continuing

## For Each Item, Extract

- **Exact code purpose**: What does this file/module do?
- **Communication method**: How does it talk to other components?
- **Module affinity**: Which other modules does it work closely with?
- **Messaging structure**: What data structures are passed?
- **Protocols used**: HTTP, gRPC, WebSocket, events, etc.
- **API structure**: Endpoints, methods, signatures
- **Notes**: Important observations
- **Future work**: TODOs, FIXMEs, planned improvements
- **Current state**: Working, broken, in-progress

## Completion

Work through all files until the analysis is complete. Update insights.md
with a final summary when done.
```

---

*End of Design Document*
