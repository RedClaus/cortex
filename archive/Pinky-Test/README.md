---
project: Cortex
component: Agents
phase: Ideation
date_created: 2026-02-07T15:54:10
source: ServerProjectsMac
librarian_indexed: 2026-02-07T18:01:18.237855
---

# Pinky

<p align="center">
  <img src="assets/pinky-logo.png" alt="Pinky Logo" width="200">
</p>

<p align="center">
  <em>"The same thing we do every night, Brain—try to take over the world!"</em>
</p>

<p align="center">
  <strong>Self-hosted AI Agent Gateway</strong>
</p>

---

## What is Pinky?

Pinky is a self-hosted AI agent gateway that lets you command an intelligent assistant from anywhere—your terminal, a web browser, Telegram, Discord, or Slack. Unlike simple chatbots, Pinky can *act*: run shell commands, manage files, execute code, interact with Git, and call APIs on your behalf.

## Features

- **Multi-Channel** - Control Pinky from TUI, WebUI, Telegram, Discord, or Slack
- **Tool Execution** - Shell, files, Git, code execution, web fetch, APIs, system commands
- **Tiered Permissions** - Unrestricted, Some Restrictions, or Restricted modes
- **Approval Workflow** - Review and approve tool executions before they run
- **Cross-Channel Identity** - Link your accounts, Pinky remembers *you* everywhere
- **Temporal Memory** - Ask about "yesterday" or "last week" and get relevant context
- **Configurable Personas** - Professional, Casual, Mentor, or create your own
- **Dual Brain Mode** - Embedded (single binary) or Remote (distributed)
- **Multi-Model Inference** - Configure multiple AI models with intelligent auto-routing
- **Model Picker** - Interactive model selection in setup wizard and TUI settings

## Quick Start

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

## Architecture

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

## Configuration

Configuration lives in `~/.pinky/config.yaml`:

```yaml
version: 1

brain:
  mode: embedded  # or "remote"
  remote_url: http://localhost:18892

inference:
  default_lane: fast
  autollm: true  # Enable automatic lane selection based on task complexity
  lanes:
    local:
      engine: ollama
      model: llama3.2:3b
      url: http://localhost:11434
    fast:
      engine: groq
      model: llama-3.1-8b-instant
      api_key: ${GROQ_API_KEY}
    smart:
      engine: anthropic
      model: claude-3-5-sonnet-20241022
      api_key: ${ANTHROPIC_API_KEY}

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

## Inference Lanes & Auto-Routing

Pinky supports multiple inference "lanes" for different use cases:

| Lane | Purpose | Default Engine |
|------|---------|----------------|
| **local** | Privacy-sensitive tasks | Ollama (local) |
| **fast** | Quick responses | Groq/OpenAI |
| **smart** | Complex reasoning | Anthropic/OpenAI |

### Auto-Routing

When `autollm: true`, Pinky automatically selects the appropriate lane based on task complexity:
- **Simple queries** → local lane ( lightweight, private)
- **Standard tasks** → fast lane (quick responses)
- **Complex analysis** → smart lane (best reasoning)

Configure models interactively via:
- **Setup Wizard**: `pinky --wizard` → Step 3: Model Selection
- **TUI Settings**: Press `/settings` in TUI mode

### Supported Engines

- **Ollama** - Local inference (fetches available models dynamically)
- **Anthropic** - Claude models (static list)
- **OpenAI** - GPT models (static list)
- **Groq** - Fast inference (static list)

## Permission Tiers

| Tier | Behavior |
|------|----------|
| **Unrestricted** | Execute all tools automatically |
| **Some Restrictions** | Auto-approve low-risk tools, ask for high-risk |
| **Restricted** | Ask before every tool execution |

The approval workflow includes an "Always allow" option that remembers your choices.

## TUI Commands

When using the terminal interface (`pinky --tui`), the following slash commands are available:

| Command | Description |
|---------|-------------|
| `/settings` | Open inference settings panel to configure lanes and models |
| `/lanes` | Show current lane configuration and auto-routing status |
| `/help` | Show help screen with key bindings |
| `/clear` | Clear chat history |

### Settings Panel

The `/settings` panel allows you to:
- Toggle **Auto-routing** (Tab key)
- Select models for each lane (↑/↓ navigate, Enter to select)
- Changes are persisted to `~/.pinky/config.yaml`

## Personas

Built-in personas:
- **Professional** - Clear, concise, formal
- **Casual** - Friendly, conversational
- **Mentor** - Patient, educational
- **Minimalist** - Terse, just the facts

Create custom personas in `~/.pinky/personas/custom.yaml`.

## Development

```bash
# Run tests
go test ./...

# Build for production (embedded brain)
go build -tags embedded -o pinky ./cmd/pinky

# Build for remote brain mode
go build -o pinky ./cmd/pinky
```

## Project Structure

```
pinky/
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
│   └── plans/          # PRD and design docs
└── assets/             # Logo and static assets
```

## Roadmap

- [x] PRD and architecture design
- [ ] Phase 1: Foundation (Brain, Tools, TUI, WebUI)
- [ ] Phase 2: Channels (Telegram, Discord, Slack)
- [ ] Phase 3: Polish (Docker, docs, tests)
- [ ] Phase 4: Extended (WhatsApp, iMessage, Browser automation)

## License

MIT

## Credits

Part of the Cortex ecosystem. Powered by CortexBrain.

---

*"Gee, Brain, what do you want to do tonight?"*
