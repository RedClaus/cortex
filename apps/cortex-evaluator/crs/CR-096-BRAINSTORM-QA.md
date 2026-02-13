---
project: Cortex
component: Brain Kernel
phase: Design
date_created: 2026-01-17T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:18:14.497886
---

# CR-096: Brainstorm Q&A Engine

**Status:** Draft
**Phase:** 3
**Priority:** P0
**Estimated Effort:** 3-5 days
**Created:** 2026-01-17
**Depends On:** CR-094, CR-095

---

## Summary

Implement the interactive brainstorming Q&A engine that allows users to have conversations with the AI using full codebase context. Support file attachments (PDF, TXT, images), artifact creation, multiple iterations, and session export.

---

## Requirements

### Functional Requirements

1. **Context-Aware Q&A**
   - Load indexed codebase context into conversation
   - Maintain conversation history within session
   - Support "what if", "can I", and comparison questions
   - Provide detailed, informed answers based on codebase knowledge

2. **File Attachments**
   - Accept PDF files (extract text)
   - Accept text files (TXT, MD, JSON, YAML)
   - Accept images (for vision-capable models)
   - Add attachment content to conversation context

3. **Artifact Management**
   - Save any response section as an artifact
   - Support artifact types: text, code, markdown, diagram
   - Store artifacts with session
   - Allow artifact retrieval and modification

4. **Multiple Iterations**
   - Support back-and-forth refinement
   - Track iteration count per artifact
   - Allow user to indicate satisfaction

5. **Session Export**
   - Export full session to markdown
   - Include conversation history
   - Include all artifacts
   - Include session metadata

---

## Technical Design

### Data Models

```go
// internal/brainstorm/types.go

type Message struct {
    ID          string       `json:"id"`
    SessionID   string       `json:"session_id"`
    Role        MessageRole  `json:"role"`
    Content     string       `json:"content"`
    Attachments []Attachment `json:"attachments,omitempty"`
    CreatedAt   time.Time    `json:"created_at"`
}

type MessageRole string

const (
    RoleUser      MessageRole = "user"
    RoleAssistant MessageRole = "assistant"
    RoleSystem    MessageRole = "system"
)

type Attachment struct {
    ID        string         `json:"id"`
    Name      string         `json:"name"`
    Type      AttachmentType `json:"type"`
    Size      int64          `json:"size"`
    Content   string         `json:"content"`   // Extracted text or base64
    FilePath  string         `json:"file_path"` // Original path
}

type AttachmentType string

const (
    AttachPDF   AttachmentType = "pdf"
    AttachText  AttachmentType = "text"
    AttachImage AttachmentType = "image"
    AttachCode  AttachmentType = "code"
)

type Artifact struct {
    ID        string       `json:"id"`
    SessionID string       `json:"session_id"`
    Name      string       `json:"name"`
    Type      ArtifactType `json:"type"`
    Content   string       `json:"content"`
    FilePath  *string      `json:"file_path,omitempty"` // If saved to disk
    Iteration int          `json:"iteration"`
    CreatedAt time.Time    `json:"created_at"`
    UpdatedAt time.Time    `json:"updated_at"`
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

### Package Structure

```
internal/
├── brainstorm/
│   ├── engine.go       # Main Q&A engine
│   ├── types.go        # Data types
│   ├── context.go      # Context building from index
│   ├── attachments.go  # File attachment processing
│   ├── artifacts.go    # Artifact management
│   ├── export.go       # Session export
│   └── storage.go      # Message/artifact persistence
```

### API Design

```go
// internal/brainstorm/engine.go

type BrainstormEngine interface {
    // Start a brainstorm session (load context)
    Start(session *Session) error

    // Send a message and get response
    SendMessage(sessionID string, message string, attachments []Attachment) (*Message, error)

    // Get conversation history
    GetHistory(sessionID string) ([]Message, error)

    // Attachment operations
    AddAttachment(sessionID string, filePath string) (*Attachment, error)
    ListAttachments(sessionID string) ([]Attachment, error)

    // Artifact operations
    CreateArtifact(sessionID string, name string, artifactType ArtifactType, content string) (*Artifact, error)
    GetArtifact(sessionID, artifactID string) (*Artifact, error)
    UpdateArtifact(sessionID, artifactID string, content string) (*Artifact, error)
    ListArtifacts(sessionID string) ([]Artifact, error)
    SaveArtifactToFile(sessionID, artifactID, filePath string) error

    // Export
    ExportSession(sessionID string) (string, error) // Returns markdown
}
```

### Context Building

```go
// internal/brainstorm/context.go

type ContextBuilder interface {
    // Build full context from session index
    BuildContext(session *Session) (*ConversationContext, error)

    // Add attachment to context
    AddAttachment(ctx *ConversationContext, attachment *Attachment) error

    // Get token count for context
    TokenCount(ctx *ConversationContext) (int, error)

    // Trim context if too large
    TrimContext(ctx *ConversationContext, maxTokens int) error
}

type ConversationContext struct {
    SystemPrompt   string            // Based on prep prompt insights
    CodebaseInfo   string            // Project purpose, architecture
    FileInsights   []FileInsight     // Relevant file analyses
    Attachments    []AttachmentData  // Processed attachments
    RecentMessages []Message         // Last N messages
}
```

### SQLite Schema

```sql
-- Messages table
CREATE TABLE IF NOT EXISTS brainstorm_messages (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id),
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    attachments TEXT,  -- JSON array
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_messages_session ON brainstorm_messages(session_id);
CREATE INDEX idx_messages_created ON brainstorm_messages(created_at);

-- Artifacts table
CREATE TABLE IF NOT EXISTS brainstorm_artifacts (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id),
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    content TEXT NOT NULL,
    file_path TEXT,
    iteration INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_artifacts_session ON brainstorm_artifacts(session_id);
CREATE INDEX idx_artifacts_type ON brainstorm_artifacts(type);
```

---

## Implementation Tasks

- [ ] Implement internal/brainstorm/types.go with data models
- [ ] Implement internal/brainstorm/storage.go for persistence
- [ ] Implement internal/brainstorm/context.go for context building
- [ ] Implement internal/brainstorm/attachments.go for file processing
- [ ] Implement internal/brainstorm/artifacts.go for artifact management
- [ ] Implement internal/brainstorm/engine.go main coordinator
- [ ] Implement internal/brainstorm/export.go for markdown export
- [ ] Add PDF text extraction (use pdfcpu or similar)
- [ ] Add image processing for vision models
- [ ] Add interactive TUI for brainstorming
- [ ] Add `/attach` command
- [ ] Add `/artifact` command
- [ ] Add `/export` command
- [ ] Write unit tests for context builder
- [ ] Write unit tests for attachment processing
- [ ] Write integration tests for full Q&A flow

---

## Files to Create/Modify

| File | Action | Description |
|------|--------|-------------|
| `internal/brainstorm/types.go` | Create | Data models |
| `internal/brainstorm/storage.go` | Create | Persistence |
| `internal/brainstorm/context.go` | Create | Context building |
| `internal/brainstorm/attachments.go` | Create | Attachment processing |
| `internal/brainstorm/artifacts.go` | Create | Artifact management |
| `internal/brainstorm/engine.go` | Create | Main engine |
| `internal/brainstorm/export.go` | Create | Session export |
| `internal/data/schema.sql` | Modify | Add new tables |
| `cmd/evaluator/main.go` | Modify | Add brainstorm commands |

---

## Acceptance Criteria

- [ ] User can start brainstorm with `evaluator brainstorm <session-id>`
- [ ] AI responds with full codebase context awareness
- [ ] User can attach PDF with `/attach document.pdf`
- [ ] User can attach text files with `/attach notes.txt`
- [ ] User can save artifact with `/artifact save <name>`
- [ ] Artifacts are persisted and retrievable
- [ ] User can export session with `/export`
- [ ] Export includes full conversation and artifacts
- [ ] Context stays within token limits (auto-trimming)
- [ ] Response latency < 5 seconds for typical queries

---

## Dependencies

- CR-094: Core Session Management
- CR-095: Project Indexer
- LLM provider with sufficient context window
- PDF library (pdfcpu or similar)

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Context too large | High | Smart trimming, summarization |
| PDF parsing failures | Medium | Fallback to raw text, error handling |
| Slow responses | Medium | Streaming, progress indicator |
| Lost context | Low | Persist all messages, recovery |
