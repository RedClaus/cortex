---
project: Cortex
component: Unknown
phase: Design
date_created: 2026-01-17T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:18:14.488088
---

# CR-099: Polish and Cortex Integration

**Status:** Draft
**Phase:** 6
**Priority:** P1
**Estimated Effort:** 3-5 days
**Created:** 2026-01-17
**Depends On:** CR-094, CR-095, CR-096, CR-097, CR-098

---

## Summary

Final polish phase that integrates Cortex Evaluator with the Cortex-03 memory system, adds session recall from brain queries, optimizes AutoLLM routing for analysis queries, and ensures production-ready quality.

---

## Requirements

### Functional Requirements

1. **Cortex Memory Integration**
   - Store session summaries in Cortex personal memory tier
   - Store artifacts in retrievable format
   - Enable session recall via brain queries
   - Sync session state with Cortex memory

2. **Session Recall**
   - Query past sessions from Cortex brain
   - Recall session by name, project, or content
   - Resume any recalled session with full context
   - Display session history from memory

3. **AutoLLM Routing**
   - Route analysis queries to Smart Lane
   - Route simple Q&A to Fast Lane with retrieval
   - Optimize for codebase context queries
   - Integrate with Cortex-03 routing infrastructure

4. **Knowledge Fabric Integration**
   - Index session insights into Knowledge Fabric
   - Enable semantic search across sessions
   - Link related sessions and artifacts
   - Support three-tier retrieval

5. **Production Polish**
   - Comprehensive error handling
   - Performance optimization
   - Logging and debugging
   - Configuration management
   - Documentation

---

## Technical Design

### Cortex Memory Integration

```go
// internal/memory/cortex.go

type CortexMemoryAdapter interface {
    // Store session in Cortex memory
    StoreSession(session *Session) error

    // Store artifact in Cortex memory
    StoreArtifact(artifact *Artifact) error

    // Recall sessions by query
    RecallSessions(query string) ([]SessionSummary, error)

    // Get session from memory
    GetSession(sessionID string) (*Session, error)

    // Sync session state
    SyncSession(session *Session) error
}

type SessionSummary struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    ProjectPath string    `json:"project_path"`
    Summary     string    `json:"summary"`
    Tags        []string  `json:"tags"`
    CreatedAt   time.Time `json:"created_at"`
    LastAccess  time.Time `json:"last_access"`
}
```

### Memory Entry Format

```go
// Entry stored in Cortex personal memory tier
type MemoryEntry struct {
    Type      string                 `json:"type"`     // "brainstorm_session"
    Content   string                 `json:"content"`  // Session summary
    Tags      []string               `json:"tags"`     // ["brainstorm", name, project]
    Metadata  map[string]interface{} `json:"metadata"`
}

// Metadata fields
// - session_id: UUID
// - project_path: /path/to/project
// - status: ready|archived
// - cr_count: number of CRs created
// - artifact_count: number of artifacts
// - prd_id: associated PRD if any
// - key_insights: top 3 insights from session
```

### AutoLLM Integration

```go
// internal/routing/router.go

type EvaluatorRouter interface {
    // Route query based on type and context
    Route(query string, context *SessionContext) (*RouteDecision, error)
}

type RouteDecision struct {
    Lane         string  // "fast" or "smart"
    Model        string  // Recommended model
    Reason       string  // Why this route
    EnableRetrieval bool // Whether to enable passive retrieval
}

// Routing logic:
// - "What if" questions → Smart Lane (complex reasoning)
// - "Can I" questions → Smart Lane (feasibility analysis)
// - Comparison questions → Smart Lane (multi-source analysis)
// - Simple lookups → Fast Lane with retrieval
// - Code generation → Smart Lane (quality matters)
```

### Knowledge Fabric Integration

```go
// internal/knowledge/fabric.go

type KnowledgeFabricAdapter interface {
    // Index session insights
    IndexSession(session *Session) error

    // Index file analysis
    IndexFileAnalysis(analysis *FileAnalysis) error

    // Search across sessions
    SearchSessions(query string) ([]SearchResult, error)

    // Get related sessions
    GetRelated(sessionID string) ([]SessionSummary, error)
}

type SearchResult struct {
    SessionID   string
    Relevance   float64
    Snippet     string
    MatchedIn   string  // "insights", "artifacts", "messages"
}
```

### Package Structure

```
internal/
├── memory/
│   ├── cortex.go       # Cortex memory adapter
│   ├── sync.go         # Session sync logic
│   └── recall.go       # Session recall from memory
├── routing/
│   ├── router.go       # AutoLLM routing
│   └── classifier.go   # Query classification
├── knowledge/
│   └── fabric.go       # Knowledge Fabric adapter
├── config/
│   ├── config.go       # Configuration management
│   └── defaults.go     # Default values
```

### Configuration

```yaml
# ~/.cortex-evaluator/config.yaml

cortex:
  # Path to Cortex-03 installation
  path: /Users/normanking/ServerProjectsMac/Development/cortex-03
  # Memory integration
  memory:
    enabled: true
    tier: personal  # personal, team, global
  # Knowledge fabric
  knowledge:
    enabled: true
    index_sessions: true
    index_artifacts: true

autollm:
  # Routing preferences
  analysis_queries: smart  # Use smart lane for analysis
  simple_queries: fast     # Use fast lane for simple Q&A
  code_generation: smart   # Use smart lane for code

indexer:
  # Default include patterns
  include:
    - "*.go"
    - "*.py"
    - "*.js"
    - "*.ts"
    - "*.md"
    - "*.yaml"
    - "*.json"
  # Default exclude patterns
  exclude:
    - "vendor/*"
    - "node_modules/*"
    - ".git/*"
    - "*.test.go"
  # Max file size (bytes)
  max_file_size: 1048576  # 1MB

notifications:
  # Notification preferences
  banner: true
  system: true  # macOS notifications
  sound: true

logging:
  level: info  # debug, info, warn, error
  file: ~/.cortex-evaluator/evaluator.log
```

---

## Implementation Tasks

### Cortex Integration
- [ ] Implement internal/memory/cortex.go adapter
- [ ] Implement session storage in Cortex memory
- [ ] Implement artifact storage in Cortex memory
- [ ] Implement internal/memory/recall.go for session recall
- [ ] Implement internal/memory/sync.go for state sync
- [ ] Add `brain recall` command for session lookup

### AutoLLM Routing
- [ ] Implement internal/routing/classifier.go for query classification
- [ ] Implement internal/routing/router.go for routing decisions
- [ ] Integrate with Cortex-03 AutoLLM infrastructure
- [ ] Add routing hints to brainstorm engine

### Knowledge Fabric
- [ ] Implement internal/knowledge/fabric.go adapter
- [ ] Index sessions on creation/update
- [ ] Enable semantic search across sessions
- [ ] Add `search` command for cross-session search

### Configuration
- [ ] Implement internal/config/config.go
- [ ] Create default configuration file
- [ ] Add config validation
- [ ] Add `config` command for settings

### Polish
- [ ] Add comprehensive error handling throughout
- [ ] Add structured logging with zerolog
- [ ] Performance profiling and optimization
- [ ] Memory usage optimization
- [ ] Add graceful shutdown handling
- [ ] Create README.md with usage guide
- [ ] Add CHANGELOG.md
- [ ] Add LICENSE file

### Testing
- [ ] Write integration tests for Cortex memory
- [ ] Write integration tests for routing
- [ ] Write end-to-end tests for full workflow
- [ ] Performance benchmarks

---

## Files to Create/Modify

| File | Action | Description |
|------|--------|-------------|
| `internal/memory/cortex.go` | Create | Cortex memory adapter |
| `internal/memory/sync.go` | Create | Session sync logic |
| `internal/memory/recall.go` | Create | Session recall |
| `internal/routing/router.go` | Create | AutoLLM routing |
| `internal/routing/classifier.go` | Create | Query classification |
| `internal/knowledge/fabric.go` | Create | Knowledge adapter |
| `internal/config/config.go` | Create | Configuration |
| `internal/config/defaults.go` | Create | Default values |
| `README.md` | Create | Usage documentation |
| `CHANGELOG.md` | Create | Version history |
| `LICENSE` | Create | License file |
| `cmd/evaluator/main.go` | Modify | Add new commands |

---

## Acceptance Criteria

### Cortex Integration
- [ ] Sessions are stored in Cortex personal memory
- [ ] Artifacts are stored and retrievable
- [ ] User can recall sessions with `evaluator brain recall <query>`
- [ ] Recalled sessions can be resumed with full context

### AutoLLM Routing
- [ ] Analysis queries route to Smart Lane
- [ ] Simple queries route to Fast Lane
- [ ] Routing decisions are logged for debugging
- [ ] Performance meets latency targets

### Knowledge Fabric
- [ ] Sessions are indexed on creation
- [ ] User can search across sessions with `evaluator search <query>`
- [ ] Related sessions are suggested

### Production Quality
- [ ] All errors are handled gracefully with helpful messages
- [ ] Logs are structured and useful for debugging
- [ ] Configuration is validated on startup
- [ ] Memory usage stays within bounds
- [ ] Application starts in < 1 second
- [ ] Documentation is complete and accurate

---

## Dependencies

- CR-094 through CR-098 (all previous CRs)
- Cortex-03 installation with memory APIs
- Cortex-03 Knowledge Fabric
- Cortex-03 AutoLLM Router

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Cortex-03 API changes | High | Version pinning, adapter pattern |
| Memory quota exceeded | Medium | Cleanup old entries, compression |
| Routing incorrect | Low | Fallback logic, user override |
| Performance regression | Medium | Benchmarks, profiling |

---

## Post-Launch

After this CR is complete, Cortex Evaluator will be production-ready with:

1. ✅ Persistent brainstorming sessions
2. ✅ Codebase indexing and analysis
3. ✅ Context-aware Q&A
4. ✅ PRD and CR generation
5. ✅ Code execution with progress monitoring
6. ✅ Full Cortex-03 integration
7. ✅ Production-quality polish

Future enhancements (not in scope):
- Multi-user collaboration
- Cloud sync
- Plugin system
- IDE integrations
