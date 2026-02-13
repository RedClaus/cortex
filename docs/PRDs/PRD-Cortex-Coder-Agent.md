---
project: Cortex
component: Agents
phase: Ideation
date_created: 2026-02-04T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:29.779371
---

# Product Requirements Document: Cortex Coder Agent

**Document ID:** PRD-CORTEX-CODER-AGENT-001  
**Version:** 1.0  
**Date:** 2026-02-04  
**Status:** DRAFT → READY FOR REVIEW  
**Owner:** Albert  
**Inspiration:** pi-coding-agent (https://github.com/badlogic/pi-mono)  
**Language:** Go 1.21+  
**Target:** Swarm-native coding harness

---

## 1. Executive Summary

### 1.1 Problem Statement
Current coding workflows require either:
- **Full CortexBrain** (heavy, multi-lobe orchestration) for complex tasks
- **Direct CLI tools** (manual, no context) for quick edits

There's no middle ground for "quick coding tasks" — interactive exploration, file navigation, rapid prototyping — that leverages swarm intelligence without the overhead of full agent orchestration.

### 1.2 Solution
Cortex Coder Agent (CCA) is a lightweight, interactive coding harness that combines:
- **Terminal UI** for interactive exploration (like pi-coding-agent)
- **CortexBrain integration** for intelligence (unlike pi's standalone approach)
- **Swarm distribution** via A2A bridge
- **Skills system** for reusable coding patterns

**Key Differentiator:** Unlike pi-coding-agent (standalone TypeScript), CCA is **swarm-native Go** that delegates to CortexBrain for LLM inference, memory, and tool execution.

### 1.3 Success Criteria

| Metric | Target |
|--------|--------|
| Cold start time | <2 seconds |
| Task acceptance rate | >95% for quick coding tasks |
| User satisfaction | Reduce "full agent" invocations by 40% |
| Skill adoption | 10+ skills in first month |
| Swarm integration | 100% A2A bridge compatible |

### 1.4 Strategic Alignment
- ✅ **Go ecosystem** — Fits swarm infrastructure
- ✅ **CortexBrain leverage** — No duplicate LLM routing
- ✅ **Lightweight** — Fast startup, minimal resources
- ✅ **Extensible** — Skills, templates, extensions
- ✅ **Multi-mode** — Interactive, JSON, RPC, SDK

---

## 2. User Stories

### 2.1 Primary Users

**As Norman (Developer)**
- I want to explore a codebase quickly without heavy agent setup
- I need to make rapid edits with AI assistance
- I want to save reusable coding patterns as skills
- I need to switch between interactive and automated modes

**As Albert (AI Agent)**
- I want a lightweight tool for quick coding tasks
- I need to delegate to CortexBrain for complex reasoning
- I want to share coding sessions via A2A bridge
- I need to use pre-built skills for common patterns

**As Harold (Swarm Foreman)**
- I want to distribute coding tasks to workers efficiently
- I need to track coding session outcomes
- I want skills shared across the swarm

### 2.2 Use Cases

| ID | Use Case | Actor | Priority |
|----|----------|-------|----------|
| UC-001 | Interactive file exploration | Norman | P0 |
| UC-002 | Quick code edits with AI | Norman | P0 |
| UC-003 | Batch file operations | Norman | P1 |
| UC-004 | Create custom skill | Norman | P1 |
| UC-005 | Use skill from marketplace | Norman | P2 |
| UC-006 | Run in JSON mode for automation | Albert | P1 |
| UC-007 | SDK integration in other tools | Albert | P2 |
| UC-008 | RPC mode for editor plugins | Albert | P3 |
| UC-009 | Share session via A2A | Albert | P2 |
| UC-010 | Collaborative coding session | Norman/Albert | P3 |

---

## 3. Functional Requirements

### 3.1 Core Agent (F-AGENT)

**F-AGENT-001: Multi-Mode Operation**
- **Interactive Mode:** Full TUI with editor, file browser, chat
- **Print/JSON Mode:** Non-interactive, scriptable output
- **RPC Mode:** JSON-RPC over stdin/stdout for editor integration
- **SDK Mode:** Go library for embedding in other applications

**F-AGENT-002: CortexBrain Integration**
- Connect to CortexBrain API (configurable endpoint)
- Use Neural Bus for event streaming
- Store sessions in MemCell (episodic memory)
- Query knowledge via CortexBrain search
- Delegate tool execution to CortexBrain lobes

**F-AGENT-003: Session Management**
- Named sessions with persistence
- Resume previous sessions
- Export session to CortexBrain memory
- Share session state via A2A bridge

**F-AGENT-004: Context Awareness**
- Auto-detect project type (Go, Python, JavaScript, etc.)
- Read project configuration files
- Understand git state (branch, uncommitted changes)
- Load relevant skills based on context

### 3.2 Interactive TUI (F-TUI)

**F-TUI-001: Layout**
```
┌─────────────────────────────────────────────────────────────┐
│  Session: my-project          Mode: INTERACTIVE    [? Help] │
├──────────────────┬──────────────────────────────────────────┤
│                  │                                          │
│  FILE BROWSER    │           EDITOR / CHAT                  │
│  (tree view)     │                                          │
│                  │  ┌────────────────────────────────────┐  │
│  📁 src/         │  │ > Explain this function           │  │
│  📄 main.go      │  │                                    │  │
│  📄 utils.go  ●  │  │ Assistant: This function...       │  │
│  📁 internal/    │  │                                    │  │
│  📄 api.go       │  │ [1] Apply suggestion              │  │
│                  │  │ [2] Refactor differently          │  │
│                  │  │ [3] Explain more                  │  │
│                  │  └────────────────────────────────────┘  │
│                  │                                          │
├──────────────────┴──────────────────────────────────────────┤
│  > _                                                        │
│  [Cmd: /help /skills /save /share /quit]                    │
└─────────────────────────────────────────────────────────────┘
```

**F-TUI-002: File Browser**
- Tree view with expand/collapse
- File icons by type
- Git status indicators (modified, untracked)
- Keyboard navigation (vim-style: j/k, h/l)
- Open file in editor or external editor

**F-TUI-003: Editor Panel**
- Syntax highlighting (via Chroma or similar)
- Line numbers
- Diff view for proposed changes
- Accept/Reject/Modify workflow
- Multiple editor tabs

**F-TUI-004: Chat Interface**
- Message history with scrollback
- Syntax-highlighted code blocks
- Action buttons inline ([Apply], [Refactor], etc.)
- Command palette (/help, /skills, /save, etc.)
- Markdown rendering

**F-TUI-005: Command Palette**
```
Commands:
  /help              Show help
  /skills            List available skills
  /skill <name>      Use specific skill
  /save <name>       Save current session
  /load <name>       Load saved session
  /share             Share via A2A bridge
  /mode [json|rpc]   Switch mode
  /quit              Exit
```

**F-TUI-006: Optimizations**
- Differential rendering (only update changed regions)
- CSI 2026 synchronized output (no flicker)
- Bracketed paste mode for large inputs
- Responsive to terminal resize

### 3.3 Skills System (F-SKILL)

**F-SKILL-001: Skill Definition**
```yaml
# ~/.config/cortex-coder/skills/refactor-go.yaml
name: refactor-go
description: Refactor Go code following best practices
trigger: go  # Auto-load for Go files

prompt_template: |
  You are an expert Go developer. Refactor the following code
  to follow Go best practices, idiomatic patterns, and improve
  readability.
  
  Code:
  ```go
  {{ .Code }}
  ```
  
  Context:
  - File: {{ .FilePath }}
  - Package: {{ .PackageName }}
  
  Provide:
  1. Explanation of changes
  2. Refactored code
  3. Any trade-offs considered

tools:
  - file_read
  - file_write
  - go_fmt
  - go_vet

validation:
  - go build ./...
  - go test ./...
```

**F-SKILL-002: Skill Discovery**
- Built-in skills (20+ common patterns)
- User-defined skills in `~/.config/cortex-coder/skills/`
- Project-specific skills in `.coder/skills/`
- Auto-load based on file extension/project type
- Skill marketplace via GitHub (future)

**F-SKILL-003: Built-in Skills**
| Skill | Description | Trigger |
|-------|-------------|---------|
| explain-code | Explain selected code | Any |
| refactor-go | Refactor Go code | .go |
| refactor-ts | Refactor TypeScript | .ts |
| add-tests | Generate unit tests | Any |
| fix-lint | Fix linting errors | Any |
| generate-docs | Generate documentation | Any |
| optimize-imports | Clean up imports | .go, .py, .ts |
| error-handling | Improve error handling | .go |
| add-logging | Add structured logging | Any |
| security-scan | Check for security issues | Any |

**F-SKILL-004: Skill Execution**
- Parse prompt template with Go template engine
- Inject context (file path, code, project info)
- Send to CortexBrain for processing
- Apply tool calls (file operations, commands)
- Validate results (run tests, compilation)

### 3.4 Prompt Templates (F-PROMPT)

**F-PROMPT-001: Template Variables**
```go
type PromptContext struct {
    Code        string
    FilePath    string
    PackageName string
    ProjectType string
    GitBranch   string
    Selection   string
    LineNumber  int
    ContextLines []string  // Surrounding lines
    Skills      []string   // Active skills
}
```

**F-PROMPT-002: Template Functions**
- `{{ .Code }}` — Selected or current file content
- `{{ .FilePath }}` — Relative file path
- `{{ readFile "path/to/file" }}` — Read another file
- `{{ gitDiff }}` — Current uncommitted changes
- `{{ .Skills | join ", " }}` — List active skills

**F-PROMPT-003: Context Management**
- Automatic context window management
- Trim long files intelligently
- Include related files (imports, same package)
- User-defined context rules

### 3.5 Extensions System (F-EXT)

**F-EXT-001: Extension Architecture**
```go
// Extension interface
type Extension interface {
    Name() string
    Initialize(agent *Agent) error
    Commands() []Command
    Hooks() []Hook
}

type Command struct {
    Name        string
    Description string
    Handler     func(args []string) error
}

type Hook interface {
    Event() string      // "before_prompt", "after_response", etc.
    Handler   func(ctx context.Context) error
}
```

**F-EXT-002: Extension Loading**
- Go plugins (`.so` files) in `~/.config/cortex-coder/extensions/`
- Auto-discovery on startup
- Hot-reload in development mode
- Official extensions repository (future)

**F-EXT-003: Example Extensions**
- **git-ext:** Enhanced git commands (blame, history)
- **docker-ext:** Container management
- **k8s-ext:** Kubernetes resource editing
- **test-ext:** Test runner integration
- **lint-ext:** Real-time linting

### 3.6 Tool Integration (F-TOOL)

**F-TOOL-001: Tool Calling**
- Delegate to CortexBrain tool system
- Local tools for fast operations
- Hybrid: local for simple, CortexBrain for complex

**F-TOOL-002: Local Tools**
```go
// Fast, don't need LLM
var LocalTools = map[string]Tool{
    "file_read":   FileReadTool,
    "file_write":  FileWriteTool,
    "file_list":   FileListTool,
    "git_status":  GitStatusTool,
    "git_diff":    GitDiffTool,
    "go_build":    GoBuildTool,
    "go_test":     GoTestTool,
}
```

**F-TOOL-003: CortexBrain Tools**
- Complex reasoning tasks
- Multi-file refactoring
- Cross-project analysis
- Long-running operations

### 3.7 Non-Interactive Modes (F-NONINT)

**F-NONINT-001: JSON Mode**
```bash
# Input via stdin or file
echo '{"task": "refactor", "file": "main.go"}' | \
  cortex-coder --mode=json --skill=refactor-go

# Output:
{
  "success": true,
  "changes": [
    {"file": "main.go", "diff": "...", "applied": true}
  ],
  "explanation": "Improved error handling...",
  "session_id": "uuid"
}
```

**F-NONINT-002: RPC Mode**
```bash
# Start RPC server
cortex-coder --mode=rpc

# JSON-RPC over stdin/stdout
{"jsonrpc": "2.0", "method": "edit", "params": {...}}
```

**F-NONINT-003: SDK Mode**
```go
// Use as library in other Go applications
import "github.com/RedClaus/cortex-coder-agent/pkg/sdk"

agent := sdk.New(sdk.Config{
    CortexBrainURL: "http://192.168.1.186:18892",
    Skills: []string{"refactor-go"},
})

result, err := agent.EditFile(ctx, "main.go", sdk.EditRequest{
    Instruction: "Add error handling",
})
```

---

## 4. Non-Functional Requirements

### 4.1 Performance

**NF-PERF-001: Startup Time**
- Cold start: <2 seconds
- Warm start (resumed session): <500ms
- Skill loading: <100ms per skill

**NF-PERF-002: UI Responsiveness**
- Key input latency: <16ms (60fps)
- File browser scroll: no stuttering
- Chat message streaming: smooth

**NF-PERF-003: Memory Usage**
- Base: <50MB RAM
- Per open file: +5MB
- Long sessions: <200MB

### 4.2 Reliability

**NF-REL-001: Error Recovery**
- Graceful handling of CortexBrain disconnections
- Auto-reconnect with exponential backoff
- Session auto-save every 30 seconds
- Crash recovery on restart

**NF-REL-002: Data Integrity**
- Atomic file writes
- Backup before modifications
- Git integration for rollback

### 4.3 Security

**NF-SEC-001: Safe Execution**
- Confirm destructive operations
- Sandboxed tool execution
- No credential logging

**NF-SEC-002: Network Security**
- HTTPS for CortexBrain connection
- Certificate validation
- No external API calls (except CortexBrain)

---

## 5. Technical Architecture

### 5.1 Component Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                    CORTEX CODER AGENT                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                      CLI / SDK                            │  │
│  │  interactive │ json │ rpc │ sdk                           │  │
│  └───────────────────────────────────────────────────────────┘  │
│                              │                                   │
│  ┌───────────────────────────┼───────────────────────────────┐  │
│  │                           ▼                               │  │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  │  │
│  │  │   TUI    │  │  Skills  │  │  Session │  │  Tools   │  │  │
│  │  │ Engine   │  │  Manager │  │ Manager  │  │  Router  │  │  │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘  │  │
│  │       │              │              │             │       │  │
│  └───────┼──────────────┼──────────────┼─────────────┼───────┘  │
│          │              │              │             │          │
│  ┌───────▼──────────────▼──────────────▼─────────────▼───────┐  │
│  │                    Core Engine                             │  │
│  │  - Template rendering    - Event bus                       │  │
│  │  - Context management    - Extension loader                │  │
│  └───────────────────────────┬───────────────────────────────┘  │
│                              │                                   │
│  ┌───────────────────────────▼───────────────────────────────┐  │
│  │                  CortexBrain Client                        │  │
│  │  - Neural Bus (WebSocket)   - Memory API                   │  │
│  │  - Tool delegation          - Knowledge search             │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### 5.2 Module Structure

```
cortex-coder-agent/
├── cmd/
│   └── coder/
│       └── main.go              # CLI entry point
├── pkg/
│   ├── agent/
│   │   ├── agent.go             # Core agent logic
│   │   ├── config.go            # Configuration
│   │   └── options.go           # Agent options
│   ├── tui/
│   │   ├── app.go               # BubbleTea app
│   │   ├── browser.go           # File browser component
│   │   ├── editor.go            # Editor component
│   │   ├── chat.go              # Chat component
│   │   ├── diff.go              # Diff viewer
│   │   └── styles.go            # Lipgloss styles
│   ├── skills/
│   │   ├── manager.go           # Skill loading/management
│   │   ├── executor.go          # Skill execution
│   │   └── builtin/             # Built-in skills
│   │       ├── explain.go
│   │       ├── refactor.go
│   │       └── testgen.go
│   ├── prompts/
│   │   ├── template.go          # Template engine
│   │   ├── context.go           # Context building
│   │   └── functions.go         # Template functions
│   ├── extensions/
│   │   ├── loader.go            # Extension loading
│   │   └── registry.go          # Extension registry
│   ├── tools/
│   │   ├── local/               # Local fast tools
│   │   │   ├── file.go
│   │   │   ├── git.go
│   │   │   └── go.go
│   │   └── cortexbrain.go       # CortexBrain delegation
│   ├── cortexbrain/
│   │   ├── client.go            # HTTP/WebSocket client
│   │   ├── bus.go               # Neural Bus integration
│   │   └── memory.go            # MemCell integration
│   ├── sdk/
│   │   └── sdk.go               # Public SDK
│   └── rpc/
│       └── server.go            # JSON-RPC server
├── internal/
│   └── util/
│       └── helpers.go
├── skills/                      # Built-in skill definitions
│   ├── explain-code.yaml
│   ├── refactor-go.yaml
│   └── add-tests.yaml
├── extensions/                  # Example extensions
│   └── git-ext/
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

### 5.3 Technology Stack

| Component | Technology | Rationale |
|-----------|-----------|-----------|
| **Language** | Go 1.21+ | Swarm standard |
| **TUI Framework** | BubbleTea + Lipgloss | Proven, Go-native |
| **HTTP Client** | net/http + gorilla/websocket | Standard + reliable |
| **Template Engine** | Go text/template | Native, fast |
| **Syntax Highlight** | Chroma | Pure Go, supports many langs |
| **Diff Viewer** | diffmatchpatch | Google's diff library |
| **YAML Parsing** | gopkg.in/yaml.v3 | Standard |
| **Configuration** | Viper | Flexible config management |
| **Testing** | testify | Assertions, mocks |

### 5.4 Key Dependencies

```go
// go.mod
require (
    github.com/charmbracelet/bubbletea v0.25.0
    github.com/charmbracelet/lipgloss v0.9.1
    github.com/charmbracelet/bubbles v0.18.0
    github.com/gorilla/websocket v1.5.1
    github.com/alecthomas/chroma/v2 v2.12.0
    github.com/sergi/go-diff v1.3.1
    github.com/spf13/viper v1.18.2
    github.com/stretchr/testify v1.8.4
    gopkg.in/yaml.v3 v3.0.1
)
```

---

## 6. Integration Points

### 6.1 CortexBrain Integration

**Connection:**
```go
type CortexBrainClient struct {
    BaseURL    string
    WSURL      string
    AuthToken  string
    HTTPClient *http.Client
    WSConn     *websocket.Conn
}

func (c *CortexBrainClient) SendPrompt(ctx context.Context, req PromptRequest) (chan Event, error)
func (c *CortexBrainClient) SearchKnowledge(query string) ([]KnowledgeEntry, error)
func (c *CortexBrainClient) StoreSession(session Session) error
```

**Neural Bus Events:**
- `coder:session_start` — New coding session
- `coder:prompt_sent` — Prompt submitted
- `coder:response_received` — Response received
- `coder:file_modified` — File changed
- `coder:skill_used` — Skill executed
- `coder:session_end` — Session ended

### 6.2 A2A Bridge Integration

**Message Types:**
```go
type CoderSessionShare struct {
    SessionID   string    `json:"session_id"`
    ProjectPath string    `json:"project_path"`
    Files       []string  `json:"files"`
    SkillsUsed  []string  `json:"skills_used"`
    SharedBy    string    `json:"shared_by"`
    SharedAt    time.Time `json:"shared_at"`
}
```

### 6.3 Memory Integration

**Session Storage:**
```go
// Stored in CortexBrain MemCell
type CoderSessionMemory struct {
    SessionID    string            `json:"session_id"`
    StartTime    time.Time         `json:"start_time"`
    EndTime      *time.Time        `json:"end_time,omitempty"`
    ProjectPath  string            `json:"project_path"`
    FilesOpened  []string          `json:"files_opened"`
    FilesModified []string         `json:"files_modified"`
    Prompts      []PromptRecord    `json:"prompts"`
    SkillsUsed   []string          `json:"skills_used"`
    Outcome      string            `json:"outcome"` // "success", "aborted", "error"
}
```

---

## 7. API Specification

### 7.1 CLI Commands

```bash
# Interactive mode (default)
cortex-coder

# With specific skill
cortex-coder --skill=refactor-go

# JSON mode
cortex-coder --mode=json < request.json

# RPC mode
cortex-coder --mode=rpc

# SDK mode (library usage)
# See pkg/sdk documentation

# Session management
cortex-coder session list
cortex-coder session load <name>
cortex-coder session delete <name>

# Skills
cortex-coder skills list
cortex-coder skills install <url>
cortex-coder skills create <name>

# Extensions
cortex-coder extensions list
cortex-coder extensions install <path>
```

### 7.2 Configuration File

```yaml
# ~/.config/cortex-coder/config.yaml
version: "1.0"

agent:
  name: "default"
  mode: "interactive"  # interactive, json, rpc
  
cortexbrain:
  url: "http://192.168.1.186:18892"
  ws_url: "ws://192.168.1.186:18892/bus"
  auth_token: "${CORTEXBRAIN_TOKEN}"
  
tui:
  theme: "dracula"  # dracula, default, custom
  show_line_numbers: true
  word_wrap: true
  tab_size: 4
  
skills:
  auto_load: true
  directories:
    - "~/.config/cortex-coder/skills/"
    - "./.coder/skills/"
  
extensions:
  enabled: true
  directory: "~/.config/cortex-coder/extensions/"
  
session:
  auto_save: true
  save_interval: "30s"
  history_size: 100

a2a:
  bridge_url: "http://192.168.1.128:18802"
  enabled: true
```

---

## 8. Implementation Phases

### Phase 1: Core Foundation (Weeks 1-2)

**Goal:** Basic interactive mode working

**Tasks:**
- [ ] Project scaffolding (repo, CI, docs)
- [ ] CortexBrain client (HTTP + WebSocket)
- [ ] Basic TUI layout (file browser + chat)
- [ ] Simple prompt → response flow
- [ ] Session persistence (local file)
- [ ] Configuration system

**Deliverables:**
- `cortex-coder` starts in interactive mode
- Can browse files and chat
- Responses from CortexBrain displayed
- Sessions saved locally

### Phase 2: Skills System (Weeks 3-4)

**Goal:** Skill loading and execution

**Tasks:**
- [ ] Skill YAML parser
- [ ] Template engine with context
- [ ] 10 built-in skills
- [ ] Skill auto-loading by file type
- [ ] Skill creation CLI

**Deliverables:**
- Skills load automatically
- `refactor-go` skill works end-to-end
- Users can create custom skills

### Phase 3: TUI Polish (Weeks 5-6)

**Goal:** Production-ready interactive mode

**Tasks:**
- [ ] Editor panel with syntax highlighting
- [ ] Diff viewer for changes
- [ ] Differential rendering optimization
- [ ] CSI 2026 synchronized output
- [ ] Command palette
- [ ] Help system

**Deliverables:**
- Smooth, flicker-free TUI
- Full editor capabilities
- Professional polish

### Phase 4: Advanced Modes (Weeks 7-8)

**Goal:** JSON, RPC, and SDK modes

**Tasks:**
- [ ] JSON mode implementation
- [ ] RPC server (JSON-RPC 2.0)
- [ ] SDK package
- [ ] Editor plugin examples (VS Code, Vim)
- [ ] Documentation

**Deliverables:**
- All four modes working
- SDK documentation
- Editor integration examples

### Phase 5: Swarm Integration (Weeks 9-10)

**Goal:** Full swarm integration

**Tasks:**
- [ ] A2A bridge message types
- [ ] Session sharing
- [ ] CortexBrain memory integration
- [ ] Collaborative session support
- [ ] Extension system

**Deliverables:**
- Sessions shareable via A2A
- Stored in CortexBrain memory
- Extension API stable

---

## 9. Comparison: CCA vs pi-coding-agent vs CortexBrain

| Feature | CCA (Go) | pi-coding-agent (TS) | CortexBrain |
|---------|----------|---------------------|-------------|
| **Language** | Go | TypeScript | Go |
| **Weight** | Lightweight | Medium | Heavy |
| **Startup** | <2s | <3s | N/A (always on) |
| **LLM Routing** | CortexBrain | Self-contained | Native |
| **Memory** | CortexBrain MemCell | Self-managed | Native |
| **Tools** | CortexBrain + local | Self-contained | Full lobe system |
| **TUI** | BubbleTea | Custom | Web UI |
| **Use Case** | Quick tasks | Quick tasks | Complex orchestration |
| **Swarm Integration** | Native | None | Native |

---

## 10. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| **CortexBrain dependency** | High | Graceful degradation, local mode |
| **TUI complexity** | Medium | Start simple, iterate |
| **Skill fragmentation** | Low | Built-in skills + templates |
| **Performance with large files** | Medium | Streaming, pagination |
| **Extension security** | Medium | Sandboxing, signing |

---

## 11. Success Metrics & KPIs

| Metric | Baseline | Target | Measurement |
|--------|----------|--------|-------------|
| **Daily active users** | 0 | 5+ (swarm agents) | Usage logs |
| **Task completion rate** | N/A | >90% | Session outcomes |
| **Average session time** | N/A | <10 min for quick tasks | Session logs |
| **Skill adoption** | 0 | 5+ custom skills/month | Skill registry |
| **CortexBrain offload** | N/A | 40% of tasks via CCA | Task routing logs |

---

## 12. Appendix

### A. References

- pi-coding-agent: https://github.com/badlogic/pi-mono/tree/main/packages/coding-agent
- BubbleTea: https://github.com/charmbracelet/bubbletea
- CortexBrain PRD: `cortex-brain/docs/` (existing)
- A2A Protocol: `memory/a2a-migration-plan.md`

### B. Related Documents

- `Cortex/PRDs/PRD-Cortex-WebAPI-Harvester.md` — Companion project
- `Cortex/Analysis/Pi-Mono-Port-Analysis.md` — Full pi-mono analysis
- `MEMORY.md` — Swarm architecture

### C. Glossary

| Term | Definition |
|------|------------|
| **CCA** | Cortex Coder Agent |
| **Skill** | Reusable coding pattern/template |
| **TUI** | Terminal User Interface |
| **Neural Bus** | CortexBrain event system |
| **MemCell** | CortexBrain memory storage |

---

**Document Control:**
- **Version:** 1.0
- **Last Updated:** 2026-02-04
- **Status:** ✅ APPROVED FOR DEVELOPMENT
- **Next Review:** Phase 1 completion

**Signatures:**
- [ ] Norman (Product Owner)
- [ ] Albert (Technical Lead)
- [ ] Harold (Swarm Foreman)

---

*This PRD replaces the need to port pi-mono. Instead, we build swarm-native CCA that leverages existing infrastructure.*
