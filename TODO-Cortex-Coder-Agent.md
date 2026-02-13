---
project: Cortex
component: Agents
phase: Ideation
date_created: 2026-02-04T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:29.689526
---

# TODO: Cortex Coder Agent (CCA)

**Based on PRD:** `Cortex/PRDs/PRD-Cortex-Coder-Agent.md`  
**Status:** APPROVED P1 — IMMEDIATE START  
**Priority:** P1 (Highest)  
**Timeline:** 10 weeks (5 phases × 2 weeks)  
**Created:** 2026-02-04  
**Updated:** 2026-02-04 (Priority elevated to P1)

---

## Overview

Swarm-native interactive coding harness in Go — lightweight alternative to pi-coding-agent that delegates to CortexBrain instead of being standalone.

**Key Principle:** Unlike pi-coding-agent (TypeScript, standalone), CCA is:
- Written in **Go** (swarm standard)
- **CortexBrain-integrated** (LLM, memory, tools)
- **Swarm-native** (A2A bridge, Neural Bus, MemCell)

---

## Phase 1: Core Foundation (Weeks 1-2) — START HERE

### Week 1: Project Setup & CortexBrain Client

#### Day 1-2: Repository Setup
- [ ] Create GitHub repo: `github.com/RedClaus/cortex-coder-agent`
- [ ] Initialize Go module (`go mod init github.com/RedClaus/cortex-coder-agent`)
- [ ] Set up project structure:
  ```
  cmd/coder/main.go
  pkg/agent/
  pkg/tui/
  pkg/skills/
  pkg/prompts/
  pkg/extensions/
  pkg/tools/
  pkg/cortexbrain/
  pkg/sdk/
  pkg/rpc/
  skills/builtin/
  extensions/examples/
  ```
- [ ] Add Makefile with targets: build, test, lint, install
- [ ] Configure CI/CD (GitHub Actions for Go 1.21/1.22)
- [ ] Add .gitignore, LICENSE, CODE_OF_CONDUCT
- [ ] Write initial README with quick start

#### Day 3-4: Dependencies & Configuration
- [ ] Add core dependencies to go.mod:
  - github.com/charmbracelet/bubbletea
  - github.com/charmbracelet/lipgloss
  - github.com/charmbracelet/bubbles
  - github.com/gorilla/websocket
  - github.com/alecthomas/chroma/v2
  - github.com/spf13/viper
  - github.com/spf13/cobra
- [ ] Implement configuration system (viper)
  - Config file: `~/.config/cortex-coder/config.yaml`
  - Environment variables
  - CLI flags
- [ ] Default configuration with CortexBrain endpoint

#### Day 5-7: CortexBrain Client
- [ ] HTTP client for CortexBrain API
  - Base client with retries
  - Authentication (token-based)
  - Error handling
- [ ] WebSocket client for Neural Bus
  - Connection management
  - Event streaming
  - Reconnection logic
- [ ] Health check integration
- [ ] Connection status in TUI

#### Day 8-10: Basic CLI Framework
- [ ] Cobra CLI setup
  - `coder` (interactive mode - default)
  - `coder --mode=json`
  - `coder --mode=rpc`
  - `coder session list`
  - `coder skills list`
  - `coder config`
- [ ] CLI help and documentation
- [ ] Version command

**Phase 1 Week 1 Deliverables:**
- [ ] Repository created and public
- [ ] `go build ./...` succeeds
- [ ] Configuration system working
- [ ] CortexBrain client connects successfully
- [ ] CLI framework functional

### Week 2: TUI Foundation & Session Management

#### Day 1-3: BubbleTea App Structure
- [ ] Main TUI application structure
  - Model, Init, Update, View
  - State management
  - Message routing
- [ ] Layout framework
  - Responsive to terminal size
  - Panel management
- [ ] Theme system (Lipgloss)
  - Default theme
  - Dracula theme (match monitor)
  - Custom theme support

#### Day 4-6: File Browser Component
- [ ] Tree view for file system
  - Expand/collapse directories
  - File icons by type
  - Keyboard navigation (j/k, h/l, Enter)
- [ ] Git integration
  - Status indicators (modified, untracked, staged)
  - Branch display
- [ ] File operations
  - Open file (internal or external editor)
  - Preview file content
  - Quick actions

#### Day 7-9: Chat Panel
- [ ] Message display
  - Scrollable history
  - Markdown rendering
  - Code block syntax highlighting
- [ ] Message input
  - Multi-line input
  - History (up/down arrows)
  - Submit on Enter
- [ ] Streaming response display

#### Day 10: Session Management
- [ ] Session structure
  - Session ID, name, timestamp
  - Project path
  - Open files
  - Message history
- [ ] Local file persistence
  - Save to `~/.local/share/cortex-coder/sessions/`
  - JSON format
- [ ] Auto-save (every 30 seconds)
- [ ] Session list/resume

**Phase 1 Week 2 Deliverables:**
- [ ] `cortex-coder` starts in interactive mode
- [ ] Can browse files and view git status
- [ ] Can chat with CortexBrain
- [ ] Sessions saved and resumable
- [ ] All tests passing

---

## Phase 2: Skills System (Weeks 3-4)

### Week 3: Skill Framework

#### Day 1-2: YAML Parser
- [ ] Skill YAML schema definition
- [ ] Parser implementation
- [ ] Validation (schema check)
- [ ] Error reporting

#### Day 3-5: Template Engine
- [ ] Go template integration
- [ ] Context variables:
  - `.Code`, `.FilePath`, `.PackageName`
  - `.ProjectType`, `.GitBranch`
  - `.Selection`, `.LineNumber`
- [ ] Template functions:
  - `readFile`, `gitDiff`, `join`
- [ ] Context builder from session state

#### Day 6-7: Skill Discovery
- [ ] Built-in skills directory
- [ ] User skills: `~/.config/cortex-coder/skills/`
- [ ] Project skills: `./.coder/skills/`
- [ ] Auto-load by file extension
- [ ] Skill registry

#### Day 8-10: Skill Execution
- [ ] Parse prompt template
- [ ] Inject context
- [ ] Send to CortexBrain
- [ ] Handle tool calls
- [ ] Validation hooks

**Phase 2 Week 3 Deliverables:**
- [ ] Skill framework operational
- [ ] Can load and parse skill YAML
- [ ] Can execute skills via CortexBrain

### Week 4: LSP Integration (NEW - Added 2026-02-04)

#### Day 1-2: LSP Client
- [ ] LSP client implementation
  - WebSocket/stdio connection to language servers
  - gopls (Go), rust-analyzer (Rust), pyright (Python)
  - Initialize, shutdown, lifecycle management
- [ ] LSP message types (request/response/notification)
- [ ] Error handling and reconnection

#### Day 3-4: Diagnostics
- [ ] Diagnostics panel in TUI
  - Error/warning counts per file
  - Inline markers in file browser
  - Jump to error location
- [ ] Real-time diagnostic updates
- [ ] Diagnostic filtering (errors only, all)

#### Day 5-6: Code Intelligence
- [ ] Hover information (type info, docs)
- [ ] Go-to-definition (jump to symbol)
- [ ] Find references
- [ ] Symbol outline (file structure)

#### Day 7-8: Completions
- [ ] Autocomplete in chat code blocks
- [ ] Trigger on demand (Ctrl+Space)
- [ ] Context-aware suggestions
- [ ] Snippet support

#### Day 9-10: Format & Refactor
- [ ] Format document/range
- [ ] Rename symbol
- [ ] Code actions (quick fixes)

**Phase 2 Week 4 Deliverables:**
- [ ] LSP client connected to language servers
- [ ] Diagnostics displayed in TUI
- [ ] Hover info and go-to-definition working
- [ ] Autocomplete functional

### Week 5: Built-in Skills (was Week 4)

#### Day 1-2: Code Understanding Skills
- [ ] `explain-code` — Explain selected code
- [ ] `explain-function` — Explain current function
- [ ] `explain-file` — Explain file purpose

#### Day 3-4: Refactoring Skills
- [ ] `refactor-go` — Refactor Go code
- [ ] `refactor-ts` — Refactor TypeScript
- [ ] `optimize-imports` — Clean up imports

#### Day 5-6: Quality Skills
- [ ] `add-tests` — Generate unit tests
- [ ] `fix-lint` — Fix linting errors
- [ ] `error-handling` — Improve error handling

#### Day 7-8: Documentation Skills
- [ ] `generate-docs` — Generate documentation
- [ ] `add-logging` — Add structured logging
- [ ] `add-comments` — Add inline comments

#### Day 9-10: Security & Analysis
- [ ] `security-scan` — Check for security issues
- [ ] `complexity-analysis` — Analyze code complexity
- [ ] Skill documentation and examples

**Phase 2 Week 4 Deliverables:**
- [ ] 10+ built-in skills working
- [ ] Skills auto-load based on file type
- [ ] Users can create custom skills
- [ ] Skill documentation complete

---

## Phase 3: TUI Polish (Weeks 5-6)

### Week 5: Editor & Diff Viewer

#### Day 1-3: Editor Panel
- [ ] Syntax highlighting (Chroma)
- [ ] Line numbers
- [ ] Multiple tabs
- [ ] Tab switching
- [ ] Close tabs

#### Day 4-6: Diff Viewer
- [ ] Side-by-side diff
- [ ] Inline diff
- [ ] Syntax highlighting in diff
- [ ] Navigation (next/prev change)

#### Day 7-9: Change Application
- [ ] Accept/Reject workflow
- [ ] Modify before apply
- [ ] Preview changes
- [ ] Undo support

#### Day 10: Integration
- [ ] Editor + Chat integration
- [ ] File browser + Editor integration
- [ ] Keyboard shortcuts help

**Phase 3 Week 5 Deliverables:**
- [ ] Full editor with syntax highlighting
- [ ] Diff viewer for proposed changes
- [ ] Accept/Reject/Modify workflow

### Week 6: Performance & Polish

#### Day 1-3: Differential Rendering
- [ ] Track last rendered state
- [ ] Calculate diff between frames
- [ ] Only update changed regions
- [ ] Performance benchmarks

#### Day 4-5: CSI 2026 Synchronized Output
- [ ] Implement CSI 2026 escape sequences
- [ ] Atomic screen updates
- [ ] No flicker guarantee

#### Day 6-7: UI Components
- [ ] Command palette (/commands)
- [ ] Notification system
- [ ] Loading indicators
- [ ] Status bar

#### Day 8-10: Polish & Bug Fixes
- [ ] Responsive to all terminal sizes
- [ ] Bracketed paste mode
- [ ] Error handling and recovery
- [ ] Edge case testing

**Phase 3 Week 6 Deliverables:**
- [ ] Smooth, flicker-free TUI
- [ ] Professional polish
- [ ] All major bugs fixed

---

## Phase 4: Advanced Modes (Weeks 7-8)

### Week 7: JSON & RPC Modes

#### Day 1-3: JSON Mode
- [ ] Non-interactive input parsing
- [ ] JSON output format
- [ ] Error handling with JSON errors
- [ ] Batch processing
- [ ] Documentation

#### Day 4-6: RPC Server
- [ ] JSON-RPC 2.0 protocol
- [ ] stdin/stdout transport
- [ ] Method handlers:
  - `initialize`
  - `edit`
  - `apply`
  - `getSession`
  - `listSkills`
- [ ] Error codes and messages

#### Day 7: Testing
- [ ] JSON mode tests
- [ ] RPC mode tests
- [ ] Integration tests

#### Day 8-10: Documentation
- [ ] JSON mode API reference
- [ ] RPC protocol documentation
- [ ] Example scripts

**Phase 4 Week 7 Deliverables:**
- [ ] JSON mode working
- [ ] RPC server operational

### Week 8: SDK Mode

#### Day 1-3: SDK Package
- [ ] Public SDK interface
- [ ] Agent initialization
- [ ] Configuration options
- [ ] Error types

#### Day 2-4: Core SDK Methods
- [ ] `EditFile()`
- [ ] `ApplySkill()`
- [ ] `GetSession()`
- [ ] `ListSkills()`

#### Day 5-6: Advanced SDK Features
- [ ] Streaming responses
- [ ] Context support
- [ ] Cancellation
- [ ] Middleware support

#### Day 7-8: Editor Plugins (Examples)
- [ ] VS Code extension (basic)
- [ ] Vim plugin (basic)
- [ ] Documentation for plugin developers

#### Day 9-10: SDK Documentation
- [ ] Go package documentation
- [ ] Usage examples
- [ ] Best practices guide

**Phase 4 Week 8 Deliverables:**
- [ ] SDK package published
- [ ] Editor plugin examples
- [ ] Complete SDK documentation

---

## Phase 5: Swarm Integration (Weeks 9-10)

### Week 9: A2A Bridge & Memory

#### Day 1-3: A2A Bridge Integration
- [ ] Define CCA message types
- [ ] Session sharing via A2A
- [ ] Skill discovery messages
- [ ] Collaborative session protocol

#### Day 2-4: CortexBrain Memory
- [ ] Store sessions in MemCell
- [ ] Query knowledge via CortexBrain
- [ ] Session history integration
- [ ] Export to memory

#### Day 5: Neural Bus Events
- [ ] `coder:session_start`
- [ ] `coder:prompt_sent`
- [ ] `coder:response_received`
- [ ] `coder:file_modified`
- [ ] `coder:skill_used`
- [ ] `coder:session_end`

#### Day 6-7: Testing
- [ ] A2A integration tests
- [ ] Memory integration tests
- [ ] End-to-end tests

#### Day 8-10: Documentation
- [ ] Swarm integration guide
- [ ] A2A message reference
- [ ] Memory schema documentation

**Phase 5 Week 9 Deliverables:**
- [ ] A2A integration working
- [ ] Sessions stored in CortexBrain memory

### Week 10: Extensions & Final Polish

#### Day 1-3: Extension System
- [ ] Extension interface
- [ ] Plugin loader (.so files)
- [ ] Extension registry
- [ ] Hot-reload (dev mode)

#### Day 2-4: Example Extensions
- [ ] git-ext (enhanced git commands)
- [ ] test-ext (test runner integration)
- [ ] lint-ext (real-time linting)

#### Day 5-6: Final Testing
- [ ] Full test suite
- [ ] Performance benchmarks
- [ ] Security review
- [ ] User acceptance testing

#### Day 7-8: Documentation
- [ ] Complete user guide
- [ ] API reference
- [ ] Tutorial: "Your First Skill"
- [ ] Troubleshooting guide

#### Day 9-10: Release
- [ ] Version 1.0.0 tag
- [ ] Release notes
- [ ] Installation scripts
- [ ] Announcement

**Phase 5 Week 10 Deliverables:**
- [ ] Extension API stable
- [ ] v1.0.0 released
- [ ] Complete documentation
- [ ] Swarm-native integration complete

---

## Success Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| Cold start time | <2 seconds | Timer |
| Task acceptance rate | >95% | Session outcomes |
| Full agent offload | 40% reduction | Task routing logs |
| Skill adoption | 10+ custom skills | Skill registry |
| Daily active users | 5+ (swarm agents) | Usage logs |
| Average session time | <10 min | Session logs |
| Test coverage | >80% | go test -cover |

---

## Dependencies

### External
- Go 1.21+
- Chrome/Chromium (for future features)
- Git

### Internal (Required)
- CortexBrain API (Pink:18892)
- A2A Bridge (Harold:18802)
- Neural Bus (WebSocket)

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| CortexBrain dependency | High | Graceful degradation, local mode |
| TUI complexity | Medium | Start simple, iterate |
| 10-week timeline | Medium | Phased delivery, MVP first |
| Extension security | Medium | Sandboxing, signing (v2) |

---

## Related Work

- **PRD:** `Cortex/PRDs/PRD-Cortex-Coder-Agent.md` (26KB)
- **Analysis:** `Cortex/Analysis/Pi-Mono-Port-Analysis.md`
- **pi-mono:** https://github.com/badlogic/pi-mono (inspiration)
- **WebAPI Harvester:** `Cortex/PRDs/PRD-Cortex-WebAPI-Harvester.md` (P2)

---

## Notes

- **Priority elevated to P1** per Norman's direction (2026-02-04)
- WebAPI Harvester moved to P2, to be delivered after CCA
- Focus on swarm-native integration from day one
- Leverage existing CortexBrain infrastructure
- Pure Go implementation — no TypeScript/JavaScript runtime

---

---

## Phase 6: DeepEval Model Benchmarking (Week 9) — ADDED 2026-02-04

### Overview
Integrate [DeepEval](https://github.com/confident-ai/deepeval) framework for systematic LLM evaluation and model ranking. Create real-time dashboard showing best model per use case.

**PRD:** `Cortex/PRDs/PRD-Cortex-DeepEval-Integration.md` (13KB, DRAFT)

### Week 9: DeepEval Integration

#### Day 1-2: DeepEval Setup
- [ ] Add `pkg/eval/` package structure
- [ ] Python bridge (exec DeepEval from Go)
- [ ] Configuration for evaluation datasets
- [ ] Test dataset: coding tasks, refactoring, explanation

#### Day 3-4: Metrics & Benchmarks
- [ ] G-Eval for code quality scoring
- [ ] Latency metrics (tokens/second)
- [ ] Cost per request tracking
- [ ] Custom metrics: correctness, style, best practices
- [ ] Benchmark runner: `coder benchmark --model <name>`

#### Day 5-6: Model Ranking Engine
- [ ] Score aggregation across metrics
- [ ] Weighted ranking (accuracy vs speed vs cost)
- [ ] Per-task-type rankings (Go, Python, refactoring, etc.)
- [ ] Historical tracking (performance over time)

#### Day 7-8: TUI Dashboard
- [ ] New panel: Model Rankings
- [ ] Real-time comparison table
- [ ] Filter by task type, metric
- [ ] Sort by score, latency, cost
- [ ] Visual indicators (trending up/down)

#### Day 9-10: Smart Model Selection
- [ ] Auto-select best model for current task
- [ ] Fallback chain based on rankings
- [ ] A/B testing mode (compare 2 models side-by-side)
- [ ] Report generation: `coder eval report --output html`

**Phase 6 Week 9 Deliverables:**
- [ ] DeepEval integration operational
- [ ] Custom benchmark dataset for coding tasks
- [ ] Real-time model ranking dashboard
- [ ] Smart auto-selection based on use case
- [ ] Exportable evaluation reports

**Dependencies:**
- DeepEval Python package
- Test dataset (curated from real tasks)
- All models configured in CortexBrain

---

*Last Updated: 2026-02-04 (Added DeepEval Phase 6)*  
*Status: APPROVED P1 — READY FOR IMMEDIATE DEVELOPMENT*
