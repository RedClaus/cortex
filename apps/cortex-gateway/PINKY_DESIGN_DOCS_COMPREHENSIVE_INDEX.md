---
project: Cortex-Gateway
component: Agents
phase: Ideation
date_created: 2026-02-08T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-08T14:46:20.368703
---

# Pinky Design Documents - Comprehensive Index

**Project:** Pinky - Self-hosted AI Agent Gateway
**Created:** 2026-02-08
**Location:** ~/ServerProjectsMac/Pinky/
**Status:** Active Development (Phase: Ideation/Design)

---

## Executive Summary

Pinky is a **self-hosted AI agent gateway** that provides multi-channel access to an intelligent AI assistant with tool execution capabilities. Unlike Pink (the infrastructure server at 192.168.1.186), Pinky is a standalone application project.

**Core Features:**
- Multi-channel messaging (TUI, WebUI, Telegram, Discord, Slack)
- Tool execution (shell, files, Git, code, web, APIs)
- Tiered permissions (unrestricted/some/restricted) with approval workflows
- Dual brain mode (embedded single binary or remote CortexBrain)
- Cross-channel user identity linking
- Configurable personas
- Temporal memory system

**Relationship to Cortex Ecosystem:**
Pinky serves as a gateway layer while CortexBrain provides the cognitive intelligence (20-lobe brain architecture).

---

## Document Categories

### 1. Core Design Documents

#### 1.1 Product Requirements Document (PRD)
**File:** `docs/plans/PINKY-PRD.md`
**Size:** 54,892 bytes
**Version:** 1.0.0
**Date:** 2026-02-07
**Status:** Approved

**Table of Contents:**
1. Overview & Vision
2. Architecture (Embedded vs Remote deployment modes)
3. Tool Execution Framework
4. Channels & Messaging
5. Agent Loop & Brain Integration
6. Memory & User Identity
7. User Interfaces (TUI & WebUI)
8. Implementation Phases
9. Technical Specifications
10. Appendix

**Key Architecture Decisions:**
- **Mode A: Embedded Brain** - Single binary (~50MB), built with `go build -tags embedded`
- **Mode B: Remote Brain** - Gateway process connecting to external CortexBrain service
- Tool framework with 8 categories: Shell, Files, Git, Code, Web, Database, System, API
- Permission tiers: Unrestricted, Some Restrictions, Restricted
- Approval workflow with "Always allow" memory

**Comparison to OpenClaw:**
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

#### 1.2 Model Picker & Auto-Routing Design
**File:** `docs/plans/Pinky-model-picker.md`
**Size:** 18,085 bytes
**Date:** 2026-02-08
**Status:** Planned
**Priority:** P1 - High

**Goals:**
1. Auto-routing toggle for automatic lane selection based on task complexity
2. Model picker for cloud providers (Anthropic/OpenAI/Groq)
3. Model picker for local models (Ollama)
4. Two access points: Setup wizard AND TUI settings panel

**Current State Analysis:**
- Config structure already supports `AutoLLM` bool field
- EmbeddedBrain APIs partially implemented:
  - ✓ `SetAutoLLM(enabled bool)` / `GetAutoLLM() bool`
  - ✓ `SetLane(name string) error` / `GetLane() string`
  - ✓ `GetLanes() []LaneInfo`
  - ✗ Missing: `SetModel(lane, model string) error`
  - ✗ Missing: `GetAvailableModels(engine string) ([]string, error)`

**Wizard Enhancement:**
- New step between StepAPIKeys and StepChannels
- Model picker step with auto-routing toggle

**TUI Enhancement:**
- New `FocusSettings` focus state
- Settings panel with:
  - Auto-routing on/off toggle
  - Fast lane model picker
  - Smart lane model picker

**Implementation Details:**
- Ollama model fetching via HTTP GET `/api/tags`
- Cloud provider model lists via SDK APIs
- Model validation before saving to config
- Settings persistence to `~/.pinky/config.yaml`

---

### 2. Security & Audit

#### 2.1 Security Audit Report
**File:** `docs/SECURITY_AUDIT.md`
**Size:** 7,020 bytes
**Date:** 2026-02-07
**Auditor:** Phase 3 Security Review
**Status:** Critical issues found and fixed

**Summary:**
- **12 vulnerabilities identified:**
  - 4 Critical (remote code execution, command injection)
  - 4 High (SSRF, path traversal, permission bypass)
  - 4 Medium (CORS misconfiguration, missing size limits)

**Critical Vulnerabilities (All Fixed):**

1. **AppleScript Injection in System Tool**
   - File: `internal/tools/system.go:373-378`
   - Type: Command Injection (CWE-78)
   - Attack: Insufficient escaping of backslashes/quotes
   - Fix: Enhanced `escapeAppleScript` with proper character escaping

2. **PowerShell Injection via Notification**
   - File: `internal/tools/system.go:217-226`
   - Type: Command Injection (CWE-78)
   - Attack: String interpolation with insufficient escaping
   - Fix: Improved `escapePowerShell` to handle newlines and embedded quotes

3. **Command Injection in Shell Tool**
   - Attack: Direct user input to `exec.Command`
   - Fix: Argument validation and shell escaping

4. **SSRF in Web Fetch Tool**
   - Attack: Unrestricted URL fetching
   - Fix: URL validation, IP blocklists, DNS rebinding protection

**High Severity Issues (All Fixed):**
- Path traversal in file operations
- Permission bypass in approval workflow
- Insecure credential storage
- Missing rate limiting

**Recommendations Implemented:**
- Security-focused code review
- Input validation on all external inputs
- Principle of least privilege
- Comprehensive audit logging

---

### 3. User Documentation

#### 3.1 Main README
**File:** `README.md`
**Size:** 8,574 bytes
**Date:** Updated 2026-02-08

**Sections:**
- What is Pinky? (tagline: "The same thing we do every night, Brain—try to take over the world!")
- Features overview
- Quick Start guide
- Architecture diagram
- Configuration (`~/.pinky/config.yaml`)
- Permission tiers explanation
- Personas (Professional, Casual, Mentor, Minimalist)
- Development commands
- Project structure
- Roadmap (4 phases)
- License (MIT)

**Quick Start:**
```bash
# Build Pinky
go build -o pinky ./cmd/pinky

# Run setup wizard
./pinky --wizard

# Start in TUI mode
./pinky --tui

# Or start server mode (for WebUI and channels)
./pinky
```

**Architecture:**
```
┌─────────────────────────────────────────────────────────────────┐
│                           PINKY                                  │
├──────────────┬──────────────┬──────────────┬───────────────────┤
│     TUI      │    WebUI     │   Telegram   │   Discord/Slack   │
├──────────────┴──────────────┴──────────────┴───────────────────┤
│                      Channel Router                              │
├─────────────────────────────────────────────────────────────────┤
│                      Agent Loop                                  │
│  (context building, tool calling, memory integration)            │
├──────────────┬──────────────┬──────────────────────────────────┤
│    Brain     │    Tools     │          Permissions              │
│ (embedded or │ (shell, git, │  (approval workflow, tiers)       │
│   remote)    │  files, etc) │                                   │
├──────────────┴──────────────┴──────────────────────────────────┤
│              Memory & Identity (SQLite + Vector)                 │
└─────────────────────────────────────────────────────────────────┘
```

**Roadmap:**
- [x] PRD and architecture design
- [ ] Phase 1: Foundation (Brain, Tools, TUI, WebUI)
- [ ] Phase 2: Channels (Telegram, Discord, Slack)
- [ ] Phase 3: Polish (Docker, docs, tests)
- [ ] Phase 4: Extended (WhatsApp, iMessage, Browser automation)

---

#### 3.2 Deployment Guide
**File:** `deploy/README.md`
**Size:** 4,955 bytes
**Date:** 2026-02-07

**Deployment Options:**
- Linux systemd (user service or system service)
- macOS launchd (LaunchAgent or LaunchDaemon)

**Quick Start:**
```bash
# Build Pinky first
go build -o pinky ./cmd/pinky

# Install as user service (recommended for personal use)
./deploy/install-service.sh

# Or install as system service (for servers)
./deploy/install-service.sh --system
```

**Directory Structure:**
```
deploy/
├── README.md
├── install-service.sh
├── uninstall-service.sh
├── systemd/
│   ├── pinky.service            # User-level
│   └── pinky-system.service     # System-level
└── launchd/
    ├── com.pinky.agent.plist    # macOS user (login)
    └── com.pinky.daemon.plist   # macOS system (boot)
```

**Linux User Service Commands:**
```bash
systemctl --user start pinky
systemctl --user stop pinky
systemctl --user restart pinky
systemctl --user status pinky
journalctl --user -u pinky -f
```

**macOS User Agent Commands:**
```bash
launchctl start com.pinky.agent
launchctl stop com.pinky.agent
launchctl list | grep pinky
tail -f ~/.pinky/logs/pinky.log
```

**Configuration Locations:**
- User service: `~/.config/pinky/env` (Linux) or edit plist (macOS)
- System service: `/etc/pinky/env` (Linux) or `/etc/pinky/` (macOS)
- Main config: `~/.pinky/config.yaml` (user) or `/etc/pinky/config.yaml` (system)

**Troubleshooting:**
- Service won't start (check binary, logs, config)
- Permission denied (user vs system permissions)
- Port already in use (`lsof -i :18800`)
- macOS security prompts (grant permissions on first run)

---

### 4. Agent Workflow Instructions

#### 4.1 Agent Workflow (bd/beads)
**File:** `AGENTS.md`
**Size:** ~1,500 bytes
**Date:** 2026-02-07

**Purpose:** Instructions for AI agents working on Pinky using `bd` (beads) issue tracking.

**Quick Reference:**
```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --status in_progress  # Claim work
bd close <id>         # Complete work
bd sync               # Sync with git
```

**Landing the Plane (Session Completion) - MANDATORY WORKFLOW:**

When ending a work session, ALL steps below MUST be completed:

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd sync
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds

---

#### 4.2 Claude/Refinery Context
**File:** `CLAUDE.md`
**Size:** ~400 bytes
**Date:** 2026-02-07

**Purpose:** Minimal instructions for Refinery (Gastown multi-agent system) context recovery.

**Content:**
```markdown
# Refinery Context (pinky)

> **Recovery**: Run `gt prime` after compaction, clear, or new session

Full context is injected by `gt prime` at session start.

## Quick Reference

- Check MQ: `gt mq list`
- Process next: `gt mq process`
```

**Note:** This file is used by the Gastown multi-agent orchestrator when working on Pinky.

---

## Project Structure

```
Pinky/
├── cmd/pinky/          # Main entry point
├── internal/
│   ├── brain/          # Brain interface (embedded/remote)
│   ├── channels/       # Telegram, Discord, Slack, WebChat
│   ├── tools/          # Tool execution framework
│   ├── permissions/    # Approval workflow
│   ├── memory/         # Memory store with temporal search
│   ├── identity/       # Cross-channel user identity
│   ├── persona/        # Personality system
│   ├── tui/            # BubbleTea terminal UI
│   ├── webui/          # React web dashboard
│   └── wizard/         # First-run setup
├── cortexbrain/        # CortexBrain source (for embedded mode)
├── web/                # WebUI frontend source
├── docs/               # Documentation
│   ├── plans/          # PRD and design docs
│   │   ├── PINKY-PRD.md
│   │   └── Pinky-model-picker.md
│   └── SECURITY_AUDIT.md
├── deploy/             # Service deployment files
│   ├── README.md
│   ├── systemd/        # Linux systemd services
│   └── launchd/        # macOS launchd plists
├── assets/             # Logo and static assets
├── README.md           # Main project documentation
├── AGENTS.md           # Agent workflow instructions
└── CLAUDE.md           # Refinery context
```

---

## Configuration

**Main Config:** `~/.pinky/config.yaml`

```yaml
version: 1

brain:
  mode: embedded  # or "remote"
  remote_url: http://localhost:18892

server:
  host: 127.0.0.1
  port: 18800
  webui_port: 18801

channels:
  telegram:
    enabled: true
    token: ${TELEGRAM_BOT_TOKEN}
  discord:
    enabled: true
    token: ${DISCORD_BOT_TOKEN}
  slack:
    enabled: false

permissions:
  default_tier: some  # unrestricted | some | restricted

persona:
  default: professional
```

---

## Development Status

### Current Phase
**Phase:** Ideation/Design
**Progress:** ~40% - Core design complete, implementation starting

### Completed Work
- ✅ PRD (PINKY-PRD.md) - Approved
- ✅ Architecture design (embedded vs remote)
- ✅ Tool execution framework design
- ✅ Security audit (12 vulnerabilities fixed)
- ✅ Deployment infrastructure (systemd/launchd)
- ✅ Model picker & routing design (Pinky-model-picker.md)

### In Progress
- 🔄 Phase 1: Foundation implementation
  - Brain interface (embedded/remote)
  - Tool execution framework
  - TUI with BubbleTea
  - WebUI with React

### Planned
- 📋 Phase 2: Channels (Telegram, Discord, Slack)
- 📋 Phase 3: Polish (Docker, comprehensive docs, tests)
- 📋 Phase 4: Extended (WhatsApp, iMessage, browser automation)

---

## Key Technologies

| Component | Technology | Purpose |
|-----------|-----------|---------|
| **Backend** | Go 1.21+ | Main application logic |
| **TUI** | BubbleTea | Terminal user interface |
| **WebUI** | React + Vite | Web dashboard |
| **Brain** | CortexBrain | Cognitive intelligence (20 lobes) |
| **Local LLM** | Ollama | Embedded inference option |
| **Memory** | SQLite + Vector | Context & identity storage |
| **Messaging** | WebSocket + SSE | Real-time communication |
| **Deployment** | systemd/launchd | Service management |

---

## Vault Status

**Vault Location:** `/Users/normanking/Documents/CortexBrain Vault/`
**Status:** ⚠️ **NOT YET INDEXED**

**Note:** A search for "Pinky" in the CortexBrain Vault returned no results. Pinky design documents have not yet been copied to the vault for knowledge base integration.

**Recommendation:** Copy Pinky design documents to vault structure:
```
CortexBrain Vault/
└── Projects/
    └── Pinky/
        ├── README.md
        ├── PINKY-PRD.md
        ├── Pinky-model-picker.md
        ├── SECURITY_AUDIT.md
        ├── deploy-README.md
        └── AGENTS.md
```

---

## Related Projects

### CortexBrain
- **Relationship:** Provides cognitive intelligence (20-lobe brain)
- **Integration:** Pinky connects as A2A client or embeds CortexBrain
- **Port:** 8080 (A2A server)

### Pink Server
- **IMPORTANT:** Pink (192.168.1.186) is **NOT** Pinky
- Pink = Infrastructure node hosting CortexBrain, Ollama, Redis
- Pinky = AI Agent Gateway application (this project)

### Cortex Swarm
- **Relationship:** Pinky could potentially integrate with swarm for distributed agent execution
- **Current Status:** No direct integration planned in Phase 1-3

---

## Quick Links

| Document | Path | Purpose |
|----------|------|---------|
| **PRD** | `docs/plans/PINKY-PRD.md` | Complete product requirements |
| **Model Picker** | `docs/plans/Pinky-model-picker.md` | Model selection & routing design |
| **Security Audit** | `docs/SECURITY_AUDIT.md` | Vulnerability findings & fixes |
| **README** | `README.md` | Quick start & overview |
| **Deployment** | `deploy/README.md` | Service installation guide |
| **Agent Workflow** | `AGENTS.md` | bd/beads workflow for AI agents |
| **Refinery Context** | `CLAUDE.md` | Gastown integration |

---

## Contact & Attribution

**Project Lead:** Norman King
**Development Team:** Claude Opus 4.5 (AI Pair Programmer)
**License:** MIT
**Repository:** ~/ServerProjectsMac/Pinky/
**Part of:** Cortex Ecosystem

---

**Index Created:** 2026-02-08
**Last Updated:** 2026-02-08
**Version:** 1.0.0
**Status:** Complete

---

## Summary

This index catalogs **7 primary design documents** for the Pinky AI Agent Gateway project:

1. **PINKY-PRD.md** (54,892 bytes) - Complete product requirements
2. **Pinky-model-picker.md** (18,085 bytes) - Model selection & routing
3. **SECURITY_AUDIT.md** (7,020 bytes) - Security vulnerabilities & fixes
4. **README.md** (8,574 bytes) - Quick start & overview
5. **deploy/README.md** (4,955 bytes) - Service deployment guide
6. **AGENTS.md** (~1,500 bytes) - Agent workflow instructions
7. **CLAUDE.md** (~400 bytes) - Refinery context

**Total Documentation:** ~94,426 bytes across 7 core documents

**Status:** Design phase complete, implementation Phase 1 in progress
