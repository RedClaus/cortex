---
project: Cortex
component: Agents
phase: Ideation
date_created: 2026-02-07T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-07T15:26:37.876846
---

# Pinky - Product Requirements Document

**Version:** 1.0.0
**Date:** 2026-02-07
**Status:** Approved
**Tagline:** *"The same thing we do every night, Brain—try to take over the world."*

---

## Table of Contents

1. [Overview & Vision](#1-overview--vision)
2. [Architecture](#2-architecture)
3. [Tool Execution Framework](#3-tool-execution-framework)
4. [Channels & Messaging](#4-channels--messaging)
5. [Agent Loop & Brain Integration](#5-agent-loop--brain-integration)
6. [Memory & User Identity](#6-memory--user-identity)
7. [User Interfaces](#7-user-interfaces-tui--webui)
8. [Implementation Phases](#8-implementation-phases)
9. [Technical Specifications](#9-technical-specifications)
10. [Appendix](#10-appendix)

---

## 1. Overview & Vision

### What is Pinky?

Pinky is a self-hosted AI agent gateway that lets you command an intelligent assistant from anywhere—your terminal, a web browser, Telegram, Discord, or Slack. Unlike simple chatbots, Pinky can *act*: run shell commands, manage files, execute code, interact with Git, and call APIs on your behalf.

### Core Principles

1. **Standalone & Flexible** - Runs as a single binary with embedded brain, or connects to external CortexBrain service. Your choice at deployment.

2. **Message from Anywhere** - Unified agent accessible from TUI, WebUI, and messaging platforms. Same brain, same memory, everywhere.

3. **Act, Don't Just Chat** - Full tool execution framework with shell, files, git, code, web, and system integration.

4. **User in Control** - Tiered permission system (unrestricted/some/restricted) with approval workflows. You decide what Pinky can do autonomously.

5. **Know Your User** - Cross-channel user identity linking. Pinky remembers *you*, not just conversations.

6. **Personality Matters** - Configurable personas from templates or custom definitions. Pinky's tone and style adapt to your preference.

### Target Users

- Developers who want a personal AI assistant they can message from anywhere
- Power users seeking automation without cloud dependencies
- Teams wanting a shared AI agent across communication platforms

### Relationship to CortexBrain

Pinky integrates with the Cortex ecosystem, specifically CortexBrain (the cognitive engine with 20 lobes emulating human brain processing). Pinky serves as the gateway layer—handling channels, tools, and user interaction—while CortexBrain provides the intelligence.

### Comparison to OpenClaw

| Feature | OpenClaw | Pinky |
|---------|----------|-------|
| Multi-channel messaging | ✓ | ✓ |
| Tool execution | ✓ | ✓ |
| Self-hosted | ✓ | ✓ |
| Embedded brain option | ✗ | ✓ |
| Cognitive architecture | ✗ | ✓ (via CortexBrain) |
| Persona system | ✗ | ✓ |
| Tiered permissions | ✗ | ✓ |

---

## 2. Architecture

### 2.1 Deployment Modes

Pinky supports two deployment modes, selectable at build time or via configuration:

#### Mode A: Embedded Brain (Single Binary)

```
┌─────────────────────────────────────┐
│            Pinky Binary             │
│  ┌─────────────┬─────────────────┐  │
│  │   Gateway   │  Embedded Brain │  │
│  │  (channels, │  (inference,    │  │
│  │   tools,    │   memory,       │  │
│  │   routing)  │   lobes)        │  │
│  └─────────────┴─────────────────┘  │
└─────────────────────────────────────┘
```

- Single process, ~50MB binary
- Built with `go build -tags embedded`
- Best for: Personal machines, simple deployments
- No external dependencies (except LLM API keys)

#### Mode B: Remote Brain (Distributed)

```
┌─────────────────┐      HTTP/WS      ┌─────────────────┐
│  Pinky Gateway  │◄────────────────►│   CortexBrain   │
│  (channels,     │                   │   (inference,   │
│   tools, UI)    │                   │    memory,      │
│                 │                   │    lobes)       │
└─────────────────┘                   └─────────────────┘
```

- Two processes, can be on different machines
- Built with `go build` (default)
- Best for: Teams, high-availability, GPU separation
- CortexBrain source included in Pinky repo for deployment

### 2.2 Core Components

| Component | Purpose | Location |
|-----------|---------|----------|
| `cmd/pinky` | Main binary entry point | `cmd/pinky/main.go` |
| `internal/brain` | Brain interface + embedded/remote implementations | `internal/brain/` |
| `internal/gateway` | Channel routing, session management | `internal/gateway/` |
| `internal/channels` | Telegram, Discord, Slack, WebChat adapters | `internal/channels/` |
| `internal/tools` | Tool registry and executors | `internal/tools/` |
| `internal/permissions` | Approval workflow, permission tiers | `internal/permissions/` |
| `internal/memory` | User-linked cross-channel memory | `internal/memory/` |
| `internal/persona` | Personality templates and customization | `internal/persona/` |
| `internal/identity` | Cross-channel user identity | `internal/identity/` |
| `internal/tui` | BubbleTea terminal interface | `internal/tui/` |
| `internal/webui` | React web dashboard | `internal/webui/` |
| `internal/wizard` | First-run configuration wizard | `internal/wizard/` |
| `cortexbrain/` | Embedded CortexBrain source | `cortexbrain/` |

### 2.3 System Architecture Diagram

```
                                    ┌─────────────────────────────────────┐
                                    │            User Interfaces          │
                                    │  ┌─────────┐  ┌─────────────────┐   │
                                    │  │   TUI   │  │     WebUI       │   │
                                    │  │(Bubble  │  │  (React/Svelte) │   │
                                    │  │  Tea)   │  │                 │   │
                                    │  └────┬────┘  └────────┬────────┘   │
                                    └───────┼────────────────┼────────────┘
                                            │                │
┌───────────────────────────────────────────┼────────────────┼────────────────────────────────────┐
│                                           ▼                ▼                                    │
│                                    ┌─────────────────────────────┐                              │
│    ┌──────────────┐               │        HTTP Server          │                              │
│    │   Telegram   │◄──────────────┤   (REST + WebSocket)        │                              │
│    └──────────────┘               └──────────────┬──────────────┘                              │
│                                                   │                                             │
│    ┌──────────────┐               ┌──────────────▼──────────────┐                              │
│    │   Discord    │◄──────────────┤      Channel Router         │                              │
│    └──────────────┘               │   (unified message flow)    │                              │
│                                    └──────────────┬──────────────┘                              │
│    ┌──────────────┐                              │                                             │
│    │    Slack     │◄─────────────────────────────┤                                             │
│    └──────────────┘                              │                                             │
│                                    ┌──────────────▼──────────────┐                              │
│                                    │      Identity Service       │                              │
│                                    │  (cross-channel user ID)    │                              │
│                                    └──────────────┬──────────────┘                              │
│                                                   │                                             │
│                                    ┌──────────────▼──────────────┐      ┌──────────────────┐   │
│                                    │        Agent Loop           │◄────►│  Memory Store    │   │
│                                    │  (context, reasoning, tools)│      │  (SQLite + Vec)  │   │
│                                    └──────────────┬──────────────┘      └──────────────────┘   │
│                                                   │                                             │
│                         ┌─────────────────────────┼─────────────────────────┐                  │
│                         │                         │                         │                  │
│                         ▼                         ▼                         ▼                  │
│              ┌──────────────────┐    ┌──────────────────┐    ┌──────────────────┐             │
│              │  Tool Executor   │    │   Brain Client   │    │ Permission Svc   │             │
│              │  (7 categories)  │    │(embedded/remote) │    │  (approval flow) │             │
│              └──────────────────┘    └────────┬─────────┘    └──────────────────┘             │
│                                                │                                               │
│                                    ┌───────────┴───────────┐                                   │
│                                    │                       │                                   │
│                                    ▼                       ▼                                   │
│                         ┌──────────────────┐    ┌──────────────────┐                          │
│                         │  Embedded Brain  │    │   Remote Brain   │                          │
│                         │  (CortexBrain)   │    │  (HTTP Client)   │                          │
│                         └──────────────────┘    └──────────────────┘                          │
│                                                                                                │
│                                           PINKY                                                │
└────────────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Tool Execution Framework

### 3.1 Tool Categories

| Category | Tools | Risk Level | MVP |
|----------|-------|------------|-----|
| **Shell** | `bash`, `zsh`, execute scripts | High | ✓ |
| **Files** | read, write, search, delete, move | Medium-High | ✓ |
| **Web** | fetch URL, parse HTML, download | Low | ✓ |
| **API** | REST calls, webhook triggers | Low-Medium | ✓ |
| **Git** | status, add, commit, push, PR, clone | Medium | ✓ |
| **Code** | run Python, run Node, evaluate | High | ✓ |
| **System** | notifications, clipboard, open apps | Medium | ✓ |
| **Browser** | Playwright automation | High | Phase 2 |

### 3.2 Permission Tiers

#### Unrestricted Mode
- All tools auto-execute
- No approval prompts
- Best for: Trusted personal machines, experienced users

#### Some Restrictions Mode
- Low-risk tools (web, API) auto-execute
- Medium/High-risk tools require approval
- "Always allow" option available per tool
- **Default mode**

#### Restricted Mode
- Every tool execution requires approval
- Full visibility before any action
- Best for: Shared machines, cautious users

### 3.3 Approval Workflow

```
┌─ Tool Approval Required ─────────────────────────────┐
│                                                      │
│  Pinky wants to execute:                             │
│  ┌────────────────────────────────────────────────┐  │
│  │  $ rm -rf ./build && npm run build             │  │
│  └────────────────────────────────────────────────┘  │
│                                                      │
│  Tool: shell (High Risk)                             │
│  Working Dir: /Users/you/project                     │
│                                                      │
│  ☐ Always allow "shell" commands                     │
│  ☐ Always allow in this directory                    │
│                                                      │
│  [Deny]  [Modify]  [Approve ✓]                       │
└──────────────────────────────────────────────────────┘
```

### 3.4 Tool Interface

```go
type Tool interface {
    Name() string
    Description() string
    Category() ToolCategory
    RiskLevel() RiskLevel  // Low, Medium, High
    Execute(ctx context.Context, input ToolInput) (ToolOutput, error)
    Validate(input ToolInput) error  // Pre-execution validation
}

type ToolInput struct {
    Command    string            // Primary command/action
    Args       map[string]any    // Structured arguments
    WorkingDir string            // Execution context
    UserID     string            // For audit trail
}

type ToolOutput struct {
    Success   bool
    Output    string
    Error     string
    Duration  time.Duration
    Artifacts []string  // Created files, URLs, etc.
}
```

### 3.5 Approval Persistence

Approvals are stored per-user in `~/.pinky/approvals.yaml`:

```yaml
user_abc123:
  shell:
    always_allow: false
    allowed_patterns:
      - "git *"
      - "npm run *"
      - "go build *"
    denied_patterns:
      - "rm -rf /*"
      - "sudo *"
  files:
    always_allow: true
    allowed_directories:
      - "/Users/you/projects"
      - "/tmp"
    denied_directories:
      - "/etc"
      - "/usr"
  git:
    always_allow: true
  web:
    always_allow: true
  api:
    always_allow: false
    allowed_domains:
      - "api.github.com"
      - "api.openai.com"
```

### 3.6 Tool Implementations

#### Shell Tool
```go
type ShellTool struct {
    allowedShells []string  // ["bash", "zsh", "sh"]
    timeout       time.Duration
    maxOutput     int  // Truncate output beyond this
}

func (t *ShellTool) Execute(ctx context.Context, input ToolInput) (ToolOutput, error) {
    // 1. Validate command against denied patterns
    // 2. Set up environment and working directory
    // 3. Execute with timeout
    // 4. Capture stdout/stderr
    // 5. Return structured output
}
```

#### Files Tool
```go
type FilesTool struct {
    allowedPaths []string
    deniedPaths  []string
    maxFileSize  int64
}

// Operations: read, write, append, delete, move, copy, search, list
```

#### Git Tool
```go
type GitTool struct {
    allowPush     bool
    allowForce    bool
    defaultBranch string
}

// Operations: status, add, commit, push, pull, clone, branch, checkout, diff, log, pr
```

#### Code Tool
```go
type CodeTool struct {
    pythonPath string
    nodePath   string
    timeout    time.Duration
    sandbox    bool  // Future: containerized execution
}

// Operations: run_python, run_node, run_script
```

---

## 4. Channels & Messaging

### 4.1 Channel Priorities

#### MVP Channels
| Channel | Library | Status |
|---------|---------|--------|
| **TUI** | BubbleTea | Core |
| **WebUI** | React | Core |
| **Telegram** | go-telegram-bot-api | MVP |
| **Discord** | discordgo | MVP |
| **Slack** | slack-go/slack | MVP |

#### Phase 2 Channels
| Channel | Library | Notes |
|---------|---------|-------|
| **WhatsApp** | whatsmeow | Go native, QR pairing |
| **iMessage** | BlueBubbles API | macOS only |

### 4.2 Unified Channel Interface

```go
type Channel interface {
    // Lifecycle
    Name() string
    Start(ctx context.Context) error
    Stop() error
    IsEnabled() bool

    // Messaging
    SendMessage(userID string, msg *OutboundMessage) error
    Incoming() <-chan *InboundMessage

    // Pinky-specific
    SendApprovalRequest(userID string, req *ApprovalRequest) error
    SendToolOutput(userID string, output *ToolOutput) error

    // Capabilities
    SupportsMedia() bool
    SupportsButtons() bool  // For inline approval buttons
    SupportsThreading() bool
}

type InboundMessage struct {
    ID          string
    UserID      string
    ChannelName string    // "telegram", "discord", "slack"
    ChannelID   string    // Platform-specific channel/chat ID
    Content     string
    Media       []Media   // Images, files, audio
    ReplyTo     string    // Threading support
    Metadata    map[string]string
    ReceivedAt  time.Time
}

type OutboundMessage struct {
    Content  string
    Media    []Media
    Buttons  []Button  // Approve/Deny buttons where supported
    ReplyTo  string
    Format   MessageFormat  // Plain, Markdown, Code
}

type ApprovalRequest struct {
    ID          string
    Tool        string
    Command     string
    RiskLevel   RiskLevel
    WorkingDir  string
    Reason      string  // Why the agent wants to run this
}
```

### 4.3 Cross-Channel Message Flow

```
User sends "deploy my app" on Telegram
              │
              ▼
┌─────────────────────────────────┐
│       Channel Router            │
│  - Identify user across channels│
│  - Load user memory/context     │
│  - Route to Agent Loop          │
└───────────────┬─────────────────┘
                │
                ▼
┌─────────────────────────────────┐
│         Agent Loop              │
│  - Build context + memory       │
│  - Call Brain (embedded/remote) │
│  - Execute tools (with approval)│
│  - Stream response back         │
└───────────────┬─────────────────┘
                │
                ▼
Response sent back to Telegram
(or wherever user is active)
```

### 4.4 User Identity Linking

Users can link their accounts across channels:

```
┌─ Link Your Accounts ─────────────────────────────┐
│                                                  │
│  Your Pinky code: BRAIN-7X2K                     │
│                                                  │
│  Send this command in other channels:            │
│  /link BRAIN-7X2K                                │
│                                                  │
│  Linked accounts:                                │
│  ✓ TUI (primary)                                 │
│  ✓ Telegram @yourname                            │
│  ○ Discord (not linked)                          │
│  ○ Slack (not linked)                            │
│                                                  │
└──────────────────────────────────────────────────┘
```

Commands available in all channels:
- `/link <code>` - Link this account to your Pinky identity
- `/unlink` - Unlink this account
- `/whoami` - Show linked accounts
- `/persona <name>` - Switch persona
- `/permission <level>` - Change permission tier

---

## 5. Agent Loop & Brain Integration

### 5.1 Enhanced Agent Loop

```
┌─────────────────────────────────────────────────────────────────┐
│                        PINKY AGENT LOOP                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  1. RECEIVE      Inbound message from any channel               │
│       │                                                         │
│       ▼                                                         │
│  2. IDENTIFY     Resolve user identity across channels          │
│       │          Load user memory, preferences, persona         │
│       ▼                                                         │
│  3. CONTEXTUALIZE  Build prompt with:                           │
│       │            - System prompt + persona                    │
│       │            - Relevant memories (semantic search)        │
│       │            - Temporal memories (if time referenced)     │
│       │            - Recent conversation history                │
│       │            - Available tools description                │
│       ▼                                                         │
│  4. REASON       Send to Brain (embedded or remote)             │
│       │          Brain returns: response OR tool_calls          │
│       ▼                                                         │
│  5. TOOL LOOP    If tool_calls:                                 │
│       │            - Check permissions                          │
│       │            - Request approval if needed                 │
│       │            - Execute approved tools                     │
│       │            - Feed results back to Brain                 │
│       │            - Repeat until no more tool_calls            │
│       ▼                                                         │
│  6. RESPOND      Stream final response to user                  │
│       │          Update memory with interaction                 │
│       ▼                                                         │
│  7. PERSIST      Store conversation, tool results, learnings    │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 5.2 Brain Interface

```go
type Brain interface {
    // Core inference
    Think(ctx context.Context, req *ThinkRequest) (*ThinkResponse, error)

    // Streaming support
    ThinkStream(ctx context.Context, req *ThinkRequest) (<-chan *ThinkChunk, error)

    // Memory operations (for embedded mode)
    Remember(ctx context.Context, memory *Memory) error
    Recall(ctx context.Context, query string, limit int) ([]Memory, error)

    // Health
    Ping(ctx context.Context) error

    // Info
    Mode() BrainMode  // Embedded or Remote
}

type ThinkRequest struct {
    UserID       string
    Persona      *Persona
    Messages     []Message       // Conversation history
    Memories     []Memory        // Relevant recalled memories
    Tools        []ToolSpec      // Available tools
    MaxTokens    int
    Temperature  float64
    Stream       bool
}

type ThinkResponse struct {
    Content    string            // Text response (if no tool calls)
    ToolCalls  []ToolCall        // Tools the brain wants to execute
    Reasoning  string            // Visible thinking (if verbose mode)
    Usage      TokenUsage
    Done       bool              // For streaming
}

type ToolCall struct {
    ID       string
    Tool     string              // Tool name
    Input    map[string]any      // Arguments
    Reason   string              // Why this tool (for approval UI)
}
```

### 5.3 Brain Implementations

#### Embedded Brain

```go
type EmbeddedBrain struct {
    router    *inference.Router   // Multi-lane LLM routing
    memory    *memory.Store       // Local memory store
    lobes     *cognitive.Lobes    // CortexBrain cognitive modules
    config    *BrainConfig
}

func NewEmbeddedBrain(cfg *config.Config) (*EmbeddedBrain, error) {
    // Initialize inference router (Ollama, OpenAI, etc.)
    // Initialize memory store
    // Initialize cognitive lobes
    // Return embedded brain
}

func (b *EmbeddedBrain) Think(ctx context.Context, req *ThinkRequest) (*ThinkResponse, error) {
    // 1. Apply persona to system prompt
    // 2. Inject relevant memories
    // 3. Format tools for LLM
    // 4. Call inference router
    // 5. Parse response for tool calls
    // 6. Return structured response
}
```

#### Remote Brain

```go
type RemoteBrain struct {
    baseURL   string
    client    *http.Client
    token     string              // JWT auth
}

func NewRemoteBrain(url, token string) (*RemoteBrain, error) {
    return &RemoteBrain{
        baseURL: url,
        client:  &http.Client{Timeout: 30 * time.Second},
        token:   token,
    }, nil
}

func (b *RemoteBrain) Think(ctx context.Context, req *ThinkRequest) (*ThinkResponse, error) {
    // 1. Serialize request to JSON
    // 2. POST to CortexBrain /api/v1/think
    // 3. Deserialize response
    // 4. Return structured response
}
```

### 5.4 Context Window Management

```go
type ContextBuilder struct {
    maxTokens     int    // Model's context limit
    reserveOutput int    // Reserve for response (default: 4096)
}

func (cb *ContextBuilder) Build(req *ThinkRequest) []Message {
    budget := cb.maxTokens - cb.reserveOutput

    // Priority order (highest to lowest):
    // 1. System prompt + persona (always included)
    // 2. Tool definitions (always included)
    // 3. Current message (always included)
    // 4. Recent conversation (sliding window)
    // 5. Relevant memories (semantic search, fill remaining)

    // Truncate from lowest priority if over budget
    // Return optimized message list
}
```

---

## 6. Memory & User Identity

### 6.1 User Identity System

```go
type User struct {
    ID             string              // Internal UUID
    PrimaryName    string              // Display name
    LinkedAccounts []LinkedAccount     // Cross-channel identities
    Persona        string              // Selected persona ID
    Permissions    PermissionTier      // unrestricted/some/restricted
    Approvals      map[string]Approval // Persisted tool approvals
    Preferences    UserPreferences     // Verbosity, timezone, etc.
    CreatedAt      time.Time
    LastSeenAt     time.Time
}

type LinkedAccount struct {
    Channel     string    // "telegram", "discord", "slack", "tui", "webui"
    ExternalID  string    // Platform-specific user ID
    Username    string    // @handle or display name
    LinkedAt    time.Time
    Verified    bool
    Primary     bool      // Primary account for notifications
}

type UserPreferences struct {
    Verbosity   VerbosityLevel  // minimal, normal, verbose
    Timezone    string          // For temporal memory
    Language    string          // Response language
    CodeStyle   string          // Preferred code formatting
}
```

### 6.2 Memory Store

```go
type Memory struct {
    ID           string
    UserID       string              // Owner
    Type         MemoryType          // episodic, semantic, procedural
    Content      string              // The actual memory
    Embedding    []float64           // Vector for semantic search
    Importance   float64             // 0.0 - 1.0
    Source       string              // Channel where learned
    Context      map[string]string   // Additional metadata
    TemporalTags []TemporalTag       // Time references
    CreatedAt    time.Time
    AccessedAt   time.Time           // For decay/relevance
    AccessCount  int                 // Frequency weighting
}

type MemoryType string
const (
    MemoryEpisodic   MemoryType = "episodic"    // Events, conversations
    MemorySemantic   MemoryType = "semantic"    // Facts, knowledge
    MemoryProcedural MemoryType = "procedural"  // How to do things
)

type TemporalTag struct {
    Type   string  // "relative", "absolute", "recurring"
    Value  string  // "yesterday", "2024-01-15", "every monday"
}
```

### 6.3 Memory Operations

```go
type MemoryStore interface {
    // Write
    Store(ctx context.Context, mem *Memory) error

    // Read
    Recall(ctx context.Context, query string, opts RecallOptions) ([]Memory, error)
    GetRecent(ctx context.Context, userID string, limit int) ([]Memory, error)

    // Search
    SemanticSearch(ctx context.Context, embedding []float64, limit int) ([]Memory, error)
    TemporalSearch(ctx context.Context, userID string, temporal *TemporalContext) ([]Memory, error)

    // Maintenance
    Decay(ctx context.Context) error         // Reduce importance of old unused memories
    Consolidate(ctx context.Context) error   // Merge similar memories
    Prune(ctx context.Context, maxAge time.Duration) error  // Remove old low-importance memories
}

type RecallOptions struct {
    UserID        string
    Limit         int
    MinImportance float64
    Types         []MemoryType
    Since         time.Time
    Until         time.Time
    TimeContext   *TemporalContext  // Parsed from user query
}
```

### 6.4 Temporal Memory System

```go
type TemporalContext struct {
    HasTimeReference bool
    RelativeTime     string    // "yesterday", "last week", "2 hours ago"
    AbsoluteTime     time.Time // Parsed absolute time
    TimeRange        *TimeRange // "between Monday and Friday"
    Recurrence       string    // "every morning", "on Fridays"
}

// Detects time references in user queries
func ParseTemporalContext(query string) *TemporalContext {
    // Patterns detected:
    // - "yesterday", "last week", "2 days ago"
    // - "on Monday", "in January", "last Friday"
    // - "this morning", "tonight", "earlier today"
    // - "before the meeting", "after lunch"
    // - "the deployment from last week"
    // - "what did I say on January 5th"
}

// Score memories with temporal awareness
func (ms *MemoryStore) ScoreMemory(mem *Memory, query string, temporal *TemporalContext) float64 {
    score := mem.Importance

    // Boost for semantic match
    score += semanticSimilarity(mem.Content, query) * 0.3

    // Boost for temporal match
    if temporal != nil && temporal.HasTimeReference {
        timeDistance := temporal.AbsoluteTime.Sub(mem.CreatedAt).Abs()
        if timeDistance < 24*time.Hour {
            score += 0.4  // Strong boost for exact day match
        } else if timeDistance < 7*24*time.Hour {
            score += 0.2  // Moderate boost for same week
        }
    }

    // Recency boost
    daysSinceAccess := time.Since(mem.AccessedAt).Hours() / 24
    score += 0.1 / (1 + daysSinceAccess*0.1)

    return score
}
```

### 6.5 Storage Backend

MVP uses SQLite for simplicity:

```
~/.pinky/
├── pinky.db           # SQLite: users, memories, sessions, approvals
├── config.yaml        # Main configuration
├── approvals.yaml     # Tool approval rules
├── personas/          # Custom persona definitions
│   ├── professional.yaml
│   ├── casual.yaml
│   └── custom.yaml
├── embeddings/        # Cached vector embeddings
└── logs/              # Execution logs
    └── 2024-01-15.log
```

Database schema:

```sql
-- Users table
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    primary_name TEXT NOT NULL,
    persona TEXT DEFAULT 'professional',
    permission_tier TEXT DEFAULT 'some',
    preferences JSON,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_seen_at TIMESTAMP
);

-- Linked accounts
CREATE TABLE linked_accounts (
    id TEXT PRIMARY KEY,
    user_id TEXT REFERENCES users(id),
    channel TEXT NOT NULL,
    external_id TEXT NOT NULL,
    username TEXT,
    linked_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    verified BOOLEAN DEFAULT FALSE,
    is_primary BOOLEAN DEFAULT FALSE,
    UNIQUE(channel, external_id)
);

-- Memories
CREATE TABLE memories (
    id TEXT PRIMARY KEY,
    user_id TEXT REFERENCES users(id),
    type TEXT NOT NULL,
    content TEXT NOT NULL,
    embedding BLOB,  -- Serialized float64 array
    importance REAL DEFAULT 0.5,
    source TEXT,
    context JSON,
    temporal_tags JSON,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    accessed_at TIMESTAMP,
    access_count INTEGER DEFAULT 0
);

-- Sessions
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT REFERENCES users(id),
    channel TEXT NOT NULL,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_activity TIMESTAMP,
    message_count INTEGER DEFAULT 0,
    context JSON  -- Conversation history
);
```

---

## 7. User Interfaces (TUI & WebUI)

### 7.1 TUI (Terminal User Interface)

Built with BubbleTea, the TUI is the power-user interface:

```
┌─ Pinky v1.0.0 ──────────────────────────── [Verbose Mode] ─┐
│                                                            │
│  ┌─ Chat ─────────────────────────────────────────────┐   │
│  │ You: Deploy the app to staging                     │   │
│  │                                                     │   │
│  │ Pinky: I'll deploy to staging. Let me:             │   │
│  │   1. Check git status                               │   │
│  │   2. Run tests                                      │   │
│  │   3. Build and deploy                               │   │
│  │                                                     │   │
│  │ [Tool: git status] ✓ Clean working tree            │   │
│  │ [Tool: npm test] ✓ 42 tests passed                 │   │
│  │ [Tool: shell] ⏳ Awaiting approval...              │   │
│  │                                                     │   │
│  │ > _                                                 │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                            │
│  ┌─ Approval ──────────────────────────────────────────┐  │
│  │ Pinky wants to run:                                 │  │
│  │ $ ./deploy.sh staging                               │  │
│  │                                                     │  │
│  │ [a]pprove  [d]eny  [A]lways allow  [e]dit          │  │
│  └─────────────────────────────────────────────────────┘  │
│                                                            │
├────────────────────────────────────────────────────────────┤
│ Channels: TUI● Telegram● Discord○ | Memory: 847 | CPU: 12%│
└────────────────────────────────────────────────────────────┘
```

#### TUI Key Bindings

| Key | Action |
|-----|--------|
| `Enter` | Send message |
| `Tab` | Cycle panels (Chat → Thinking → Tools) |
| `Ctrl+V` | Toggle verbose/minimal mode |
| `Ctrl+P` | Change persona |
| `Ctrl+L` | Clear chat |
| `a/d/A/e` | Approval actions (when prompted) |
| `Ctrl+C` | Cancel current operation |
| `?` | Help |
| `Ctrl+Q` | Quit |

### 7.2 WebUI Layout

```
┌──────────────────────────────────────────────────────────────────────────┐
│  🧠 Pinky                                    [user@email] [⚙ Settings]   │
├──────────────┬───────────────────────────────┬───────────────────────────┤
│ ◀ Config     │                               │ Agent Thinking            │
│              │   Chat                        │ ───────────────           │
│ ▼ Channels   │   ───────────────             │ Planning deployment...    │
│   ☑ Telegram │                               │ Step 1: git status ✓      │
│   ☑ Discord  │   You                         │ Step 2: npm test ✓        │
│   ☐ Slack    │   Deploy to staging           │ Step 3: deploy ⏳         │
│              │                               │                           │
│ ▼ Persona    │   Pinky                       ├───────────────────────────┤
│   ◉ Profess. │   On it! Running checks...   │ Tool Execution            │
│   ○ Casual   │                               │ ───────────────           │
│   ○ Custom   │   ┌─ Approval Required ────┐  │ > ./deploy.sh staging     │
│              │   │ $ ./deploy.sh staging  │  │                           │
│ ▼ Permission │   │                        │  │ Output:                   │
│   ○ Open     │   │ [Deny] [Approve ✓]     │  │ Building...               │
│   ◉ Some     │   └────────────────────────┘  │ Uploading...              │
│   ○ Restrict │                               │                           │
│              │   ┌──────────────────────┐    ├───────────────────────────┤
│ ▼ Memory     │   │ Type a message...    │    │ Memory Context            │
│   847 items  │   └──────────────────────┘    │ ───────────────           │
│   [Search]   │                               │ • Last deploy: 2 days ago │
│              │                               │ • Staging: stage.app.com  │
│ ▼ Sessions   │                               │ • Prefers: run tests first│
│   • TUI (now)│                               │                           │
│   • Telegram │                               │                           │
└──────────────┴───────────────────────────────┴───────────────────────────┘
```

#### WebUI Features

| Section | Features |
|---------|----------|
| **Config Sidebar** | Collapsible, channel toggles, persona selector, permission tier, memory browser, active sessions |
| **Chat Panel** | Message history, inline tool results, approval dialogs, file/image upload |
| **Thinking Panel** | Real-time reasoning display (verbose mode), step progress, toggleable |
| **Tool Panel** | Live command output, execution history, logs |
| **Memory Panel** | Relevant memories for current context, searchable |

### 7.3 Startup Wizard

Both TUI and WebUI share the same wizard flow:

```
┌─ Welcome to Pinky ─────────────────────────────────────────┐
│                                                            │
│  Step 1 of 5: Brain Mode                                   │
│  ════════════════════════════════════════                  │
│                                                            │
│  How should Pinky think?                                   │
│                                                            │
│  ● Embedded (single binary, runs locally)                  │
│    Best for: Personal use, simple setup                    │
│                                                            │
│  ○ Remote (connect to CortexBrain server)                  │
│    Best for: Teams, separate GPU server                    │
│    Requires: CortexBrain running at URL                    │
│                                                            │
│                              [Back] [Next →]               │
└────────────────────────────────────────────────────────────┘
```

#### Wizard Steps

1. **Brain Mode** - Embedded vs Remote
2. **Channels** - Enable Telegram, Discord, Slack (configure tokens)
3. **Permission Level** - Unrestricted / Some Restrictions / Restricted
4. **Persona Selection** - Choose from templates or create custom
5. **Confirmation** - Review settings and start Pinky

### 7.4 Persona System

#### Built-in Templates

```yaml
# ~/.pinky/personas/professional.yaml
id: professional
name: Professional
description: Clear, concise, formal responses
traits:
  formality: high
  verbosity: medium
  emoji_usage: none
  humor: minimal
system_prompt: |
  You are Pinky, a professional AI assistant. Be clear, concise, and helpful.
  Use formal language. Focus on efficiency and accuracy.
  Avoid unnecessary pleasantries.

# ~/.pinky/personas/casual.yaml
id: casual
name: Casual
description: Friendly, conversational tone
traits:
  formality: low
  verbosity: medium
  emoji_usage: occasional
  humor: moderate
system_prompt: |
  You are Pinky, a friendly AI assistant. Be helpful and conversational.
  It's okay to be casual and use occasional humor.
  Make the interaction feel natural and comfortable.

# ~/.pinky/personas/mentor.yaml
id: mentor
name: Mentor
description: Patient, educational, explains concepts
traits:
  formality: medium
  verbosity: high
  emoji_usage: none
  humor: minimal
system_prompt: |
  You are Pinky, an educational AI mentor. When helping with tasks,
  take time to explain concepts and reasoning. Be patient and thorough.
  Help the user learn, not just complete tasks.
```

#### Custom Persona Creation

Users can create custom personas via WebUI or by editing YAML files:

```yaml
# ~/.pinky/personas/custom.yaml
id: my-persona
name: My Custom Persona
description: My personalized assistant
traits:
  formality: medium
  verbosity: low
  emoji_usage: moderate
  humor: high
system_prompt: |
  You are Pinky, my personal AI assistant.
  [User's custom instructions here]
```

### 7.5 Reasoning Visibility Toggle

Users can switch between minimal and verbose modes:

**Minimal Mode:**
```
You: Deploy to staging
Pinky: Done! Deployed to staging successfully.
```

**Verbose Mode:**
```
You: Deploy to staging

┌─ Pinky's Thinking ─────────────────────────────┐
│ Planning: User wants to deploy to staging      │
│ Step 1: Check git status for clean tree        │
│ Step 2: Run tests to ensure nothing broken     │
│ Step 3: Execute deployment script              │
└────────────────────────────────────────────────┘

[Tool: git status] ✓ Clean working tree
[Tool: npm test] ✓ All 42 tests passed
[Tool: shell ./deploy.sh staging] ✓ Deployed

Pinky: Done! Deployed to staging successfully.
       - Git: clean tree
       - Tests: 42 passed
       - Deploy: completed in 45s
```

---

## 8. Implementation Phases

### Phase 1: Foundation (MVP Core)

**Goal:** Pinky works locally via TUI and WebUI with tool execution

| Task | Description | Effort |
|------|-------------|--------|
| Project scaffold | New repo, Go modules, folder structure | 1 day |
| Brain interface | `Brain` interface + embedded/remote implementations | 2 days |
| Tool framework | Registry, executor, 7 tool categories | 3 days |
| Permission system | 3 tiers, approval workflow, persistence | 2 days |
| Agent loop | Full agentic loop with tool calling | 2 days |
| TUI | Enhanced BubbleTea with approval prompts | 2 days |
| WebUI | React dashboard with config sidebar | 3 days |
| Startup wizard | First-run configuration flow | 1 day |
| Persona system | Templates + custom persona support | 1 day |

**Phase 1 Deliverable:** ~2-3 weeks
- `pinky` binary works standalone
- TUI and WebUI fully functional
- All 7 tool categories working
- Permission tiers and approvals working
- Persona system operational

---

### Phase 2: Channels (MVP Complete)

**Goal:** Message Pinky from Telegram, Discord, Slack

| Task | Description | Effort |
|------|-------------|--------|
| Channel interface | Unified interface with approval support | 1 day |
| Telegram adapter | Port existing + add approval buttons | 1 day |
| Discord adapter | Port existing + add approval buttons | 1 day |
| Slack adapter | New implementation (slack-go library) | 2 days |
| User identity | Cross-channel linking, `/link` command | 2 days |
| Memory system | SQLite store, semantic search, temporal | 3 days |
| Channel router | Route messages, maintain sessions | 1 day |

**Phase 2 Deliverable:** ~2 weeks
- All MVP channels working (TUI, WebUI, Telegram, Discord, Slack)
- Cross-channel identity linking
- User-linked memory across channels
- Temporal memory recall

---

### Phase 3: Polish & Production

**Goal:** Production-ready deployment

| Task | Description | Effort |
|------|-------------|--------|
| Docker packaging | Multi-stage builds, compose file | 1 day |
| systemd/launchd | Service files for Linux/macOS | 0.5 day |
| Metrics | Prometheus instrumentation | 1 day |
| Logging | Structured logging, log rotation | 0.5 day |
| Documentation | README, setup guide, API docs | 2 days |
| Testing | Unit tests, integration tests | 3 days |
| Security audit | Review permissions, sanitize inputs | 1 day |

**Phase 3 Deliverable:** ~1.5 weeks
- Production-ready binary
- Docker deployment option
- Full documentation
- Test coverage > 70%

---

### Phase 4: Extended Channels (Post-MVP)

| Task | Description | Effort |
|------|-------------|--------|
| WhatsApp | whatsmeow integration, QR pairing | 1 week |
| iMessage | BlueBubbles REST API (macOS only) | 3 days |
| Browser automation | Playwright tool integration | 1 week |

---

### Timeline Summary

| Phase | Duration | Cumulative |
|-------|----------|------------|
| Phase 1 (Foundation) | 2-3 weeks | 2-3 weeks |
| Phase 2 (Channels) | 2 weeks | 4-5 weeks |
| Phase 3 (Polish) | 1.5 weeks | 5.5-6.5 weeks |
| **MVP Complete** | **~6 weeks** | |
| Phase 4 (Extended) | 2.5 weeks | 8-9 weeks |

---

## 9. Technical Specifications

### 9.1 Technology Stack

| Component | Technology |
|-----------|------------|
| Language | Go 1.24+ |
| TUI Framework | Charm (BubbleTea, Lipgloss, Bubbles) |
| WebUI Framework | React 18 + TypeScript |
| Database | SQLite 3 |
| Vector Search | SQLite + custom embedding |
| HTTP Server | Go stdlib `net/http` |
| WebSocket | `gorilla/websocket` |
| Configuration | YAML (`gopkg.in/yaml.v3`) |
| Logging | `slog` (stdlib) |
| Metrics | Prometheus (`client_golang`) |

### 9.2 External Dependencies

| Dependency | Purpose |
|------------|---------|
| `github.com/charmbracelet/bubbletea` | TUI framework |
| `github.com/charmbracelet/lipgloss` | TUI styling |
| `github.com/go-telegram-bot-api/telegram-bot-api/v5` | Telegram |
| `github.com/bwmarrin/discordgo` | Discord |
| `github.com/slack-go/slack` | Slack |
| `github.com/gorilla/websocket` | WebSocket |
| `github.com/mattn/go-sqlite3` | SQLite driver |
| `github.com/prometheus/client_golang` | Metrics |

### 9.3 API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/api/v1/chat` | POST | Send message, receive response |
| `/api/v1/chat/stream` | WS | Streaming chat |
| `/api/v1/approve/{id}` | POST | Approve/deny tool execution |
| `/api/v1/users/me` | GET | Current user info |
| `/api/v1/users/link` | POST | Link channel account |
| `/api/v1/memory/search` | GET | Search memories |
| `/api/v1/config` | GET/PUT | Configuration |
| `/api/v1/personas` | GET | List personas |
| `/api/v1/channels` | GET | Channel status |

### 9.4 Configuration File

```yaml
# ~/.pinky/config.yaml
version: 1

# Brain configuration
brain:
  mode: embedded  # embedded | remote
  remote_url: http://localhost:18892  # If mode=remote
  remote_token: ""  # JWT for remote auth

# Inference (for embedded mode)
inference:
  default_lane: fast
  lanes:
    fast:
      engine: ollama
      model: llama3:8b
    smart:
      engine: openai
      model: gpt-4o

# Server
server:
  host: 127.0.0.1
  port: 18800
  webui_port: 18801

# Channels
channels:
  telegram:
    enabled: true
    token: ${TELEGRAM_BOT_TOKEN}
  discord:
    enabled: true
    token: ${DISCORD_BOT_TOKEN}
  slack:
    enabled: false
    token: ${SLACK_BOT_TOKEN}
    app_token: ${SLACK_APP_TOKEN}

# Permissions
permissions:
  default_tier: some  # unrestricted | some | restricted

# Persona
persona:
  default: professional

# Logging
logging:
  level: info
  format: text  # text | json
  file: ~/.pinky/logs/pinky.log
```

### 9.5 Build Commands

```bash
# Development build (remote brain mode)
go build -o pinky ./cmd/pinky

# Production build with embedded brain
go build -tags embedded -o pinky ./cmd/pinky

# Build for Linux
GOOS=linux GOARCH=amd64 go build -o pinky-linux ./cmd/pinky

# Build for macOS
GOOS=darwin GOARCH=arm64 go build -o pinky-macos ./cmd/pinky

# Run tests
go test ./...

# Run with coverage
go test -cover ./...
```

---

## 10. Appendix

### 10.1 Glossary

| Term | Definition |
|------|------------|
| **Brain** | The cognitive engine that processes requests and generates responses |
| **Channel** | A messaging platform (Telegram, Discord, Slack, etc.) |
| **Tool** | An executable capability (shell, files, git, etc.) |
| **Persona** | A personality configuration affecting response style |
| **Memory** | Stored knowledge about users and interactions |
| **Agent Loop** | The processing cycle: receive → think → act → respond |

### 10.2 Related Projects

- **CortexBrain** - Core cognitive engine with 20 lobes
- **OpenClaw** - Similar open-source AI gateway (inspiration)
- **Claude Code** - Anthropic's CLI agent (UI/UX reference)

### 10.3 Success Metrics

| Metric | Target |
|--------|--------|
| Response latency (p95) | < 5s for simple queries |
| Tool execution success rate | > 95% |
| Memory recall accuracy | > 80% relevant results |
| Cross-channel message delivery | > 99% |
| User satisfaction (qualitative) | Positive feedback |

### 10.4 Security Considerations

- Tool execution runs in user's context (not root/admin)
- Approval workflow prevents unintended actions
- Sensitive data (tokens, keys) stored securely
- Input sanitization for all external inputs
- Rate limiting on API endpoints
- Audit logging for all tool executions

### 10.5 Future Considerations

- Mobile companion app (iOS/Android)
- Voice input/output
- Multi-user support with access control
- Plugin system for custom tools
- Scheduled tasks / cron-like automation
- Integration with CI/CD pipelines

---

## Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-02-07 | Norman King + Claude | Initial PRD |

---

*"Gee, Brain, what do you want to do tonight?"*
*"The same thing we do every night, Pinky—try to take over the world!"*
