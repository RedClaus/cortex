---
project: Cortex
component: Docs
phase: Ideation
date_created: 2026-02-02T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:29.874240
---

# Cortex-Gateway — Product Requirements Document
**Version:** 1.0 | **Date:** 2026-02-02 | **Authors:** Norman King + Albert (Architect)
**Status:** DRAFT — Awaiting Norman's Review
**Input Sources:** OpenClaw codebase, Nanobot codebase, CortexBrain architecture, Container Framework Plan, Mission Control Plan, CortexHub Plan

---

## 1. Executive Summary

### Problem Statement
The Cortex ecosystem currently relies on OpenClaw as its gateway layer — a 430K+ line TypeScript monolith that we don't control, can't deeply integrate, and can't containerize cleanly. CortexBrain (138K lines Go, 25 cognitive lobes, SQLite, REST API) already provides the cognitive substrate but lacks a native channel-facing gateway. The result is two systems fighting over routing, sessions, and memory.

### Solution
Build **Cortex-Gateway** — a purpose-built Go service that acts as the native channel-facing layer for CortexBrain, replacing OpenClaw's gateway role while incorporating the best architectural patterns from both OpenClaw and Nanobot.

### Vision
A single `docker compose up` deploys the entire Cortex ecosystem — gateway, brain, inference, and state — anywhere Docker runs. No OS dependencies, no missing binaries, no SSH management. The gateway IS CortexBrain's voice to the world.

### Key Architectural Decision
**CortexBrain is NOT wrapped in a foreign control plane.** Instead, Cortex-Gateway extends CortexBrain natively — using the Neural Bus as the message bus, MemCell as session storage, Blackboard as shared context, and the lane-based inference router for model selection.

---

## 2. Product Goals & Success Criteria

### Primary Goals
| ID | Goal | Measurable Outcome |
|----|------|--------------------|
| G1 | **Native CortexBrain integration** | Zero duplicate subsystems (no parallel message bus, session store, or routing) |
| G2 | **Container-first deployment** | `docker compose up -d` from cold start to operational in <60 seconds |
| G3 | **Deploy anywhere** | Proven on Linux (Proxmox), macOS (Harold's Mac), Docker Desktop (Windows WSL2) |
| G4 | **Channel parity** | WhatsApp + Telegram + Discord operational at launch; others addable |
| G5 | **Stability** | 99.9% uptime (8.7 hours max monthly downtime), auto-recovery from crashes |
| G6 | **Operational simplicity** | Single YAML config, one log stream, health endpoint, no SSH management |

### Success Criteria (Definition of Done)
- [ ] Full conversation loop: User sends WhatsApp message → Gateway routes → CortexBrain processes → response delivered
- [ ] Containerized stack deploys on fresh Ubuntu 24.04 with only Docker installed
- [ ] Backup on Pink → restore on Proxmox CT → fully operational
- [ ] Agent crash → auto-restart → session continuity (no lost context)
- [ ] Sustained 10 concurrent conversations without degradation
- [ ] Harold can manage the gateway via API (no SSH)

---

## 3. Architecture Overview

### First Principles (from Container Framework Plan)

1. **OS-Agnostic** — Runs on Linux, macOS, Windows. Backup on one, restore on another.
2. **API-Managed** — Every lifecycle operation via REST API.
3. **Self-Healing** — Crashed containers auto-restart. No human intervention.
4. **Portable State** — All state in mounted volumes. Backup = tar the volume.
5. **Image = Truth** — Docker image contains everything needed. No missing binaries.
6. **Service Discovery** — Docker DNS. No hardcoded IPs/ports.
7. **GPU Passthrough** — Containers access GPU when available.
8. **Secure by Default** — Rootless containers, isolated networks, secret management.

### System Architecture

```
                    ┌─────────────────────────────────────────────────────────────┐
                    │              CORTEX-GATEWAY (Go binary)                      │
                    │                                                              │
                    │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐       │
                    │  │ WhatsApp │ │ Telegram │ │ Discord  │ │ WebChat  │       │
                    │  │ Adapter  │ │ Adapter  │ │ Adapter  │ │ Adapter  │       │
                    │  └────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘       │
                    │       │            │            │            │              │
                    │       └────────────┴────────────┴────────────┘              │
                    │                         │                                   │
                    │                ┌────────▼────────┐                          │
                    │                │  Channel Router  │                          │
                    │                │  (DM policy,     │                          │
                    │                │   pairing, ACL)  │                          │
                    │                └────────┬────────┘                          │
                    │                         │                                   │
                    │                ┌────────▼────────┐                          │
                    │                │  Session Manager │──── MemCell API         │
                    │                │  (context, hist) │     (CortexBrain)       │
                    │                └────────┬────────┘                          │
                    │                         │                                   │
                    │                ┌────────▼────────┐                          │
                    │                │   Agent Loop     │                          │
                    │                │  (LLM call →     │                          │
                    │                │   tool exec →    │                          │
                    │                │   response)      │                          │
                    │                └────────┬────────┘                          │
                    │                         │                                   │
                    │          ┌──────────────┼──────────────┐                    │
                    │          │              │              │                    │
                    │   ┌──────▼──────┐ ┌────▼─────┐ ┌─────▼────┐              │
                    │   │ Tool Engine │ │Neural Bus│ │Blackboard│              │
                    │   │ (exec,web,  │ │(pub/sub) │ │(shared   │              │
                    │   │  file,etc)  │ │          │ │ state)   │              │
                    │   └─────────────┘ └──────────┘ └──────────┘              │
                    │                         │                                   │
                    │                ┌────────▼────────┐                          │
                    │                │ Inference Router │                          │
                    │                │ local → Ollama   │                          │
                    │                │ cloud → Grok/API │                          │
                    │                │ reason → Grok-4  │                          │
                    │                └─────────────────┘                          │
                    └─────────────────────────────────────────────────────────────┘

                    Deploys as: Docker container(s)
                    Config: Single YAML
                    State: SQLite (CortexBrain) + Redis (cache)
```

### Container Composition

```yaml
services:
  cortex-gateway:
    image: cortex/gateway:latest
    ports:
      - "18789:18789"    # HTTP/WS API
      - "18793:18793"    # WebChat UI
    environment:
      - CORTEXBRAIN_URL=http://cortex-brain:18892
      - OLLAMA_URL=http://ollama:11434
      - REDIS_URL=redis://redis:6379
    volumes:
      - ./config.yaml:/app/config.yaml
      - ./data/gateway:/app/data
    depends_on:
      - cortex-brain
      - redis
    restart: always
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:18789/health"]
      interval: 30s
      retries: 3

  cortex-brain:
    image: cortex/brain:latest
    ports:
      - "18892:18892"
    volumes:
      - ./data/brain:/app/data
    environment:
      - OLLAMA_URL=http://ollama:11434
      - JWT_SECRET=${JWT_SECRET}
    restart: always

  ollama:
    image: ollama/ollama:latest
    ports:
      - "11434:11434"
    deploy:
      resources:
        reservations:
          devices:
            - capabilities: [gpu]
    volumes:
      - ollama-models:/root/.ollama
    restart: always

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis-data:/data
    restart: always

  bridge:
    image: cortex/bridge:latest
    ports:
      - "18802:18802"
    depends_on:
      - cortex-brain
    restart: always
```

---

## 4. Functional Requirements

### FR-100: Channel Adapters

| ID | Requirement | Priority | Source |
|----|-------------|----------|--------|
| FR-101 | System SHALL accept inbound messages from WhatsApp via Baileys protocol | P0 | OpenClaw pattern |
| FR-102 | System SHALL accept inbound messages from Telegram via Bot API | P0 | Nanobot pattern |
| FR-103 | System SHALL accept inbound messages from Discord via Gateway API | P1 | OpenClaw pattern |
| FR-104 | System SHALL support adding new channel adapters without modifying core | P0 | Design principle |
| FR-105 | Each channel adapter SHALL implement a common `ChannelAdapter` interface | P0 | Nanobot pattern |
| FR-106 | System SHALL handle media messages (images, audio, video, documents) | P1 | OpenClaw pattern |
| FR-107 | System SHALL support group chat messages with mention-gating | P1 | OpenClaw pattern |
| FR-108 | System SHALL maintain per-channel message formatting (Markdown, bold, etc.) | P1 | OpenClaw pattern |
| FR-109 | WhatsApp adapter SHALL maintain a single session per gateway instance | P0 | OpenClaw invariant |
| FR-110 | System SHALL support Signal channel adapter | P2 | Future |
| FR-111 | System SHALL support Slack channel adapter | P2 | Future |
| FR-112 | System SHALL serve WebChat UI from built-in HTTP server | P1 | OpenClaw pattern |

### FR-200: Message Routing & Session Management

| ID | Requirement | Priority | Source |
|----|-------------|----------|--------|
| FR-201 | System SHALL route inbound messages through the Neural Bus | P0 | CortexBrain native |
| FR-202 | System SHALL create/retrieve sessions via CortexBrain MemCell API | P0 | CortexBrain native |
| FR-203 | System SHALL maintain per-channel, per-chat session isolation | P0 | Both patterns |
| FR-204 | System SHALL support DM pairing for unknown senders | P1 | OpenClaw pattern |
| FR-205 | System SHALL support allowlist-based access control per channel | P0 | Both patterns |
| FR-206 | System SHALL queue messages when agent is processing (no drops) | P0 | Design principle |
| FR-207 | System SHALL deduplicate retried messages via idempotency keys | P1 | OpenClaw pattern |
| FR-208 | System SHALL support multi-agent routing (route channels to different agents) | P2 | OpenClaw pattern |
| FR-209 | System SHALL persist session history across gateway restarts | P0 | Design principle |
| FR-210 | Session context SHALL be stored in CortexBrain (NOT flat files) | P0 | CortexBrain Primary Memory decision |

### FR-300: Agent Loop & LLM Integration

| ID | Requirement | Priority | Source |
|----|-------------|----------|--------|
| FR-301 | System SHALL implement an agentic loop: receive → build context → call LLM → exec tools → respond | P0 | Both patterns |
| FR-302 | Agent loop SHALL support max iteration limits to prevent runaway | P0 | Nanobot pattern |
| FR-303 | System SHALL use CortexBrain's lane-based routing for model selection | P0 | CortexBrain native |
| FR-304 | Lane routing: `local` → Ollama, `cloud` → Grok/Claude API, `reasoning` → Grok-4 | P0 | CortexBrain native |
| FR-305 | System SHALL support model override per session | P1 | OpenClaw pattern |
| FR-306 | System SHALL support model failover (primary fails → fallback) | P1 | OpenClaw pattern |
| FR-307 | System SHALL support streaming responses with chunked delivery | P1 | OpenClaw pattern |
| FR-308 | System SHALL pass conversation history as context to LLM | P0 | Both patterns |
| FR-309 | System SHALL inject system prompt, workspace files, and skills into context | P0 | Both patterns |
| FR-310 | System SHALL support sub-agent spawning for background tasks | P1 | Both patterns |

### FR-400: Tool Framework

| ID | Requirement | Priority | Source |
|----|-------------|----------|--------|
| FR-401 | System SHALL provide a tool registry for registering/discovering tools | P0 | Nanobot pattern |
| FR-402 | Built-in tools: file read/write/edit, shell exec, web search, web fetch | P0 | Both patterns |
| FR-403 | Built-in tools: message send (cross-channel), session management | P0 | OpenClaw pattern |
| FR-404 | System SHALL support CortexBrain-native tools (memory store/recall, blackboard read/write, lobe invoke) | P0 | CortexBrain native |
| FR-405 | Tool execution SHALL be sandboxed (configurable permissions) | P1 | OpenClaw pattern |
| FR-406 | System SHALL support external tool loading (skills/plugins) | P2 | OpenClaw pattern |
| FR-407 | System SHALL support browser automation via CDP/Playwright | P2 | OpenClaw pattern |

### FR-500: Scheduling & Automation

| ID | Requirement | Priority | Source |
|----|-------------|----------|--------|
| FR-501 | System SHALL support cron-based scheduled tasks | P0 | Both patterns |
| FR-502 | System SHALL support heartbeat/proactive wake-ups | P1 | OpenClaw pattern |
| FR-503 | Scheduled tasks SHALL be able to inject messages into sessions | P0 | OpenClaw pattern |
| FR-504 | System SHALL integrate with CortexBrain Sleep Cycle for nightly maintenance | P1 | CortexBrain native |
| FR-505 | System SHALL support webhook-triggered actions | P2 | OpenClaw pattern |

### FR-600: Administration & Monitoring

| ID | Requirement | Priority | Source |
|----|-------------|----------|--------|
| FR-601 | System SHALL expose REST API for lifecycle management (start/stop/status/config) | P0 | Container Framework principle |
| FR-602 | System SHALL expose /health endpoint for container orchestration | P0 | Container Framework principle |
| FR-603 | System SHALL provide real-time metrics (message count, latency, token usage) | P1 | OpenClaw pattern |
| FR-604 | System SHALL serve a web-based admin dashboard | P2 | OpenClaw pattern |
| FR-605 | System SHALL support configuration hot-reload without restart | P1 | OpenClaw pattern |
| FR-606 | System SHALL log structured JSON for aggregation/search | P0 | Design principle |
| FR-607 | System SHALL integrate with CortexBrain Neural Monitor for cognitive visualization | P2 | CortexBrain native |

### FR-700: Cortex-TUI (Terminal Interface)

| ID | Requirement | Priority | Source |
|----|-------------|----------|--------|
| FR-701 | System SHALL provide a BubbleTea-based terminal UI as a built-in channel | P1 | CortexBrain TUI-Redesign |
| FR-702 | TUI SHALL connect to Cortex-Gateway via the same WS/HTTP API as messaging channels | P0 | Design principle — TUI is just another channel |
| FR-703 | TUI SHALL display real-time conversation with the agent (send/receive) | P0 | Core functionality |
| FR-704 | TUI SHALL display Neural Bus event stream (lobe activations, cognitive events) | P1 | CortexBrain native |
| FR-705 | TUI SHALL display system health dashboard (services, GPU, memory, uptime) | P1 | Operational visibility |
| FR-706 | TUI SHALL support memory search interface (query CortexBrain MemCell) | P2 | CortexBrain native |
| FR-707 | TUI SHALL display Blackboard state (shared context, active tasks) | P2 | Mission Control plan |
| FR-708 | TUI SHALL be launchable as `cortex-gateway tui` subcommand (same binary) | P0 | Single binary principle |
| FR-709 | TUI SHALL use the established Cortex visual theme (teal #0d7377, off-white #f8f7f4) | P1 | Brand consistency |
| FR-710 | TUI SHALL support keyboard shortcuts for common operations (send, search, switch panels) | P1 | Usability |
| FR-711 | TUI SHALL work over SSH sessions (no graphics dependencies) | P0 | Remote access |
| FR-712 | TUI components SHALL reuse existing scaffolding in CortexBrain/internal/tui/ | P1 | Code reuse |

### FR-800: Cortex Web UI

| ID | Requirement | Priority | Source |
|----|-------------|----------|--------|
| FR-801 | System SHALL serve a web-based UI from the gateway's built-in HTTP server | P0 | Design principle |
| FR-802 | Web UI SHALL provide a chat interface (send/receive messages, conversation history) | P0 | OpenClaw WebChat pattern |
| FR-803 | Web UI SHALL provide a Neural Monitor panel (3D brain view, lobe activations, EEG-style traces) | P1 | Existing cortex-monitor (React/Three.js/R3F) |
| FR-804 | Web UI SHALL provide a system health dashboard (services, GPU, memory, agents, uptime) | P1 | Operational visibility |
| FR-805 | Web UI SHALL provide a memory explorer (search, browse, inspect CortexBrain memories) | P1 | CortexBrain native |
| FR-806 | Web UI SHALL provide a Blackboard inspector (shared state, active tasks, Mission Control view) | P2 | Mission Control plan |
| FR-807 | Web UI SHALL provide a session manager (list, inspect, switch conversations) | P1 | OpenClaw pattern |
| FR-808 | Web UI SHALL provide an agent configuration panel (model selection, channel config, tool permissions) | P2 | Admin functionality |
| FR-809 | Web UI SHALL provide a log viewer (real-time structured log streaming) | P2 | Operational visibility |
| FR-810 | Web UI SHALL connect via WebSocket to the gateway for real-time updates | P0 | OpenClaw pattern |
| FR-811 | Web UI SHALL be responsive (desktop + tablet + mobile) | P1 | Access anywhere |
| FR-812 | Web UI SHALL use the Cortex visual theme (teal #0d7377, off-white #f8f7f4, dark mode #0a0a0f) | P1 | Brand consistency |
| FR-813 | Web UI SHALL be a static SPA bundled into the gateway binary (no separate server) | P0 | Single deployment unit |
| FR-814 | Web UI SHALL support authentication (JWT token from CortexBrain auth) | P0 | Security |
| FR-815 | Web UI SHALL integrate the existing Neural Monitor (EEG-style visualization) | P1 | Existing asset: neural-monitor.html |
| FR-816 | Web UI SHALL integrate the existing cortex-monitor (3D brain viewer via React/Three.js) | P2 | Existing asset: Production/cortex-monitor |
| FR-817 | Web UI SHALL provide a Sleep Cycle dashboard (last run, dedup stats, consolidation history) | P2 | CortexBrain native |

### FR-900: A2A Bridge Integration

| ID | Requirement | Priority | Source |
|----|-------------|----------|--------|
| FR-901 | System SHALL register on the A2A Bridge as a named agent | P0 | Existing infrastructure |
| FR-902 | System SHALL accept task assignments from Bridge (Harold/CCP) | P1 | Swarm V2 pattern |
| FR-903 | System SHALL report task completion/status back to Bridge | P1 | Swarm V2 pattern |
| FR-904 | System SHALL support cross-agent messaging via Bridge | P1 | Existing infrastructure |

---

## 5. Non-Functional Requirements

### NFR-100: Performance

| ID | Requirement | Target | Rationale |
|----|-------------|--------|-----------|
| NFR-101 | Message-to-first-token latency | <2 seconds (local), <5 seconds (cloud) | User experience |
| NFR-102 | Concurrent conversation support | ≥10 simultaneous | Multi-channel usage |
| NFR-103 | Memory footprint (gateway binary) | <100 MB RSS | Container efficiency |
| NFR-104 | Cold start time | <10 seconds | Container restart recovery |
| NFR-105 | Message throughput | ≥50 messages/minute sustained | Peak usage |
| NFR-106 | Tool execution latency (local) | <500ms for file/shell ops | Responsiveness |

### NFR-200: Reliability & Availability

| ID | Requirement | Target | Rationale |
|----|-------------|--------|-----------|
| NFR-201 | Uptime | 99.9% (8.7h max downtime/month) | Personal assistant expectation |
| NFR-202 | Crash recovery | Auto-restart within 30 seconds | Container restart policy |
| NFR-203 | Session durability | Zero message loss on crash/restart | State in CortexBrain |
| NFR-204 | Graceful degradation | If Ollama down, fall back to cloud API | Lane routing |
| NFR-205 | If cloud API down, queue messages and retry | Max 5 minutes queue | Reliability |
| NFR-206 | Channel reconnection | Auto-reconnect within 60 seconds | WhatsApp/Telegram resilience |

### NFR-300: Portability & Deployment

| ID | Requirement | Target | Rationale |
|----|-------------|--------|-----------|
| NFR-301 | Container runtime | Docker Engine ≥24.0 | Standard |
| NFR-302 | Host OS support | Linux (x86_64, arm64), macOS, Windows (WSL2) | Deploy anywhere |
| NFR-303 | Single-command deployment | `docker compose up -d` | Operational simplicity |
| NFR-304 | Backup portability | Backup on OS A → Restore on OS B | Container Framework principle |
| NFR-305 | Configuration | Single YAML file + environment variables | Simplicity |
| NFR-306 | Zero external dependencies beyond Docker | No Node.js, Python, Go runtime needed on host | Image = Truth |
| NFR-307 | Binary size | <50 MB (Go static binary) | Container efficiency |

### NFR-400: Security

| ID | Requirement | Target | Rationale |
|----|-------------|--------|-----------|
| NFR-401 | DM access control | Allowlist + pairing for unknown senders | OpenClaw security model |
| NFR-402 | API authentication | JWT tokens for all API access | CortexBrain existing |
| NFR-403 | Secret management | Environment variables or mounted secrets | Container security |
| NFR-404 | Network isolation | Separate Docker networks per concern | Container Framework principle |
| NFR-405 | Container runtime | Non-root user inside containers | Security best practice |
| NFR-406 | Tool sandboxing | Configurable exec permissions per agent | Safety |

### NFR-500: Maintainability & Extensibility

| ID | Requirement | Target | Rationale |
|----|-------------|--------|-----------|
| NFR-501 | Language | Go (match CortexBrain) | Single runtime, no IPC overhead |
| NFR-502 | Codebase size target | <15K lines (gateway only) | Maintainability |
| NFR-503 | Channel adapter interface | Common interface, one file per adapter | Extensibility |
| NFR-504 | Tool plugin interface | Common interface, loadable at runtime | Extensibility |
| NFR-505 | Test coverage | ≥70% for core routing, session, agent loop | Quality |
| NFR-506 | Documentation | Architecture doc, API spec, deployment guide | Operability |

### NFR-600: Observability

| ID | Requirement | Target | Rationale |
|----|-------------|--------|-----------|
| NFR-601 | Structured logging | JSON format, configurable level | Aggregation |
| NFR-602 | Health endpoint | /health with component status | Container orchestration |
| NFR-603 | Metrics | Prometheus-compatible /metrics endpoint | Monitoring |
| NFR-604 | Distributed tracing | Request ID propagation through Neural Bus | Debugging |

---

## 6. User Stories

### Epic 1: Core Messaging (Channel User)

| ID | User Story | Acceptance Criteria |
|----|-----------|-------------------|
| US-101 | As a **WhatsApp user**, I want to send a message to the Cortex bot and receive an intelligent response, so that I can interact with my AI assistant from my phone. | Message delivered, CortexBrain processes, response appears in WhatsApp within 10 seconds |
| US-102 | As a **Telegram user**, I want to chat with the Cortex bot in a Telegram DM, so that I can use my preferred messaging platform. | Bot responds to DMs from allowlisted users |
| US-103 | As a **Discord user**, I want to interact with the Cortex bot in my server, so that I can get AI assistance in my team context. | Bot responds to mentions in configured channels |
| US-104 | As a **user**, I want my conversation history to persist across sessions, so that the bot remembers previous context. | History survives gateway restart; bot references earlier messages |
| US-105 | As a **user**, I want to send images/documents and get analysis, so that the bot can help with visual content. | Media forwarded to vision-capable model, analysis returned |
| US-106 | As a **user in a group chat**, I want the bot to only respond when mentioned, so that it doesn't spam the group. | Bot silent unless @mentioned or explicitly addressed |
| US-107 | As a **new user**, I want a pairing flow that verifies my identity, so that unauthorized people can't use the bot. | Unknown sender gets pairing code; admin approves; sender added to allowlist |

### Epic 2: Agent Capabilities (AI User)

| ID | User Story | Acceptance Criteria |
|----|-----------|-------------------|
| US-201 | As a **user**, I want the bot to execute shell commands on my behalf, so that I can automate tasks remotely. | `exec` tool runs commands, output returned in chat |
| US-202 | As a **user**, I want the bot to search the web and summarize results, so that I get quick research answers. | Web search results summarized coherently |
| US-203 | As a **user**, I want to set reminders and scheduled tasks, so that the bot proactively alerts me. | Cron job fires at specified time, message delivered to channel |
| US-204 | As a **user**, I want the bot to read and write files in my workspace, so that I can manage documents remotely. | File operations succeed, content returned/confirmed |
| US-205 | As a **user**, I want the bot to use CortexBrain's memory to recall past conversations and facts, so that it has long-term continuity. | Memory search returns relevant past context; bot uses it in responses |
| US-206 | As a **user**, I want to spawn background tasks, so that long-running operations don't block my conversation. | Sub-agent runs in isolation, result announced back to chat |
| US-207 | As a **user**, I want the bot to use different models for different tasks (fast model for simple questions, reasoning model for complex ones), so that I get optimal performance. | Lane routing selects appropriate model based on task complexity |

### Epic 3: Operations & Administration (Admin)

| ID | User Story | Acceptance Criteria |
|----|-----------|-------------------|
| US-301 | As an **admin**, I want to deploy the entire Cortex stack with one command, so that setup is trivial. | `docker compose up -d` → all services healthy within 60 seconds |
| US-302 | As an **admin**, I want to back up the entire system and restore it on a different machine, so that I have disaster recovery. | Tar backup → restore on fresh host → fully operational |
| US-303 | As an **admin**, I want the gateway to auto-restart on crash, so that I don't need to babysit it. | Simulated crash → container restarts → sessions resume |
| US-304 | As an **admin**, I want to check system health via API, so that I can monitor without SSH. | GET /health returns component status, uptime, version |
| US-305 | As an **admin**, I want to update configuration without restarting, so that changes take effect immediately. | Config hot-reload via API or SIGHUP |
| US-306 | As an **admin**, I want structured logs, so that I can search and analyze issues. | JSON logs with timestamp, level, component, request ID |
| US-307 | As an **admin**, I want Harold to manage the gateway via API, so that the swarm is self-managing. | Harold calls REST endpoints to check status, restart services, deploy updates |
| US-308 | As an **admin**, I want GPU passthrough for Ollama containers, so that local inference is fast. | Ollama container accesses RTX 3090, models load and infer correctly |

### Epic 4: Multi-Agent Coordination (Swarm)

| ID | User Story | Acceptance Criteria |
|----|-----------|-------------------|
| US-401 | As **Harold** (overseer), I want to send tasks to the gateway via the A2A Bridge, so that I can coordinate work. | Task assignment message → gateway accepts → agent processes |
| US-402 | As **Harold**, I want to check gateway status via Bridge, so that I can monitor the swarm. | Health query via Bridge → status response |
| US-403 | As a **worker agent** (Pink/Red), I want to receive coding tasks routed through CortexBrain, so that the brain coordinates all work. | Task routed via Blackboard → worker picks up → result stored |
| US-404 | As **CortexBrain**, I want the Sleep Cycle to consolidate daily conversation data, so that memory stays clean and searchable. | Nightly cycle: dedup, decay, consolidation runs automatically |

### Epic 5: Cortex-TUI (Terminal Interface)

| ID | User Story | Acceptance Criteria |
|----|-----------|-------------------|
| US-501 | As a **developer**, I want a terminal UI to chat with CortexBrain directly, so that I can interact without needing a phone or browser. | BubbleTea TUI launches, sends messages, receives responses |
| US-502 | As a **developer**, I want to see Neural Bus events in real-time in the TUI, so that I can observe which cognitive lobes are activating. | Event panel shows lobe activations with timestamps |
| US-503 | As an **admin**, I want a system health panel in the TUI, so that I can monitor infrastructure at a glance. | Health panel shows service status, GPU usage, memory, uptime |
| US-504 | As a **user**, I want to search CortexBrain memories from the TUI, so that I can recall past conversations and facts. | Memory search panel with query input and results display |
| US-505 | As a **developer**, I want the TUI to work over SSH, so that I can manage remote systems. | TUI renders correctly in SSH terminal sessions |
| US-506 | As a **developer**, I want to run the TUI as `cortex-gateway tui`, so that I don't need a separate binary. | Single binary, subcommand launches TUI mode |

### Epic 6: Cortex Web UI

| ID | User Story | Acceptance Criteria |
|----|-----------|-------------------|
| US-601 | As a **user**, I want to chat with CortexBrain through a web browser, so that I can interact from any device without installing apps. | Web chat loads, messages send/receive, history persists |
| US-602 | As a **user**, I want to see a 3D brain visualization showing which lobes are active, so that I can understand how CortexBrain processes my requests. | Neural Monitor shows lobe activations in real-time via WebSocket |
| US-603 | As an **admin**, I want a web dashboard showing system health, so that I can monitor from anywhere. | Dashboard shows all services, GPU stats, memory, uptime |
| US-604 | As a **user**, I want to search and browse CortexBrain's memory from the web UI, so that I can explore what the system knows. | Memory explorer with search, filters, detail view |
| US-605 | As an **admin**, I want to manage agent configuration from the web UI, so that I don't need to edit YAML files. | Config panel for models, channels, tools with save/apply |
| US-606 | As an **admin**, I want to view real-time logs in the web UI, so that I can debug issues without SSH. | Log viewer with level filtering, search, auto-scroll |
| US-607 | As a **user**, I want the web UI to work on my phone, so that I can check on the system from anywhere. | Responsive layout renders correctly on mobile browsers |
| US-608 | As an **admin**, I want the web UI to require login, so that unauthorized users can't access the system. | JWT auth gate before any content loads |

### Epic 7: CortexBrain Integration (Cognitive)

| ID | User Story | Acceptance Criteria |
|----|-----------|-------------------|
| US-701 | As the **gateway**, I want to route messages through the Neural Bus, so that all 25 cognitive lobes can process them. | Inbound message → Neural Bus event → relevant lobes activate |
| US-702 | As the **gateway**, I want to store conversation context in MemCell, so that CortexBrain IS the session store. | Session CRUD via MemCell API; no separate session DB |
| US-703 | As the **gateway**, I want to read/write the Blackboard for shared state, so that multiple agents have consistent context. | Blackboard operations succeed; state visible to all connected agents |
| US-704 | As the **gateway**, I want to invoke specific cognitive lobes for specialized tasks, so that I can leverage the full brain architecture. | Coding lobe for code tasks, planning lobe for project management, etc. |
| US-705 | As the **gateway**, I want the Metacognition lobe to evaluate response quality, so that bad responses are caught. | Quality check before delivery; low-confidence responses flagged |

---

## 7. Implementation Phases

### Phase 1: Foundation (Week 1-2)
**Goal:** Minimal viable gateway — one channel, basic agent loop, CortexBrain integration

| Task | Description | Effort |
|------|------------|--------|
| 1.1 | Go project scaffold (cmd/cortex-gateway, internal/, config/) | 2 hours |
| 1.2 | Configuration loader (YAML + env vars) | 4 hours |
| 1.3 | HTTP/WebSocket server with /health endpoint | 4 hours |
| 1.4 | Channel adapter interface + Telegram adapter | 2 days |
| 1.5 | CortexBrain client (REST API wrapper for MemCell, Blackboard, Neural Bus) | 1 day |
| 1.6 | Session manager (backed by MemCell) | 1 day |
| 1.7 | Basic agent loop (context → LLM call → response) | 2 days |
| 1.8 | Inference router (Ollama client + cloud API client) | 1 day |
| 1.9 | Dockerfile + docker-compose.yml | 4 hours |
| 1.10 | End-to-end test: Telegram message → CortexBrain → response | 4 hours |

**Deliverable:** Working Telegram bot powered by CortexBrain, deployable via Docker.

### Phase 2: Tools & WhatsApp (Week 3)
**Goal:** Core tool framework + WhatsApp channel

| Task | Description | Effort |
|------|------------|--------|
| 2.1 | Tool registry (register/discover/execute pattern) | 1 day |
| 2.2 | Built-in tools: file ops, shell exec, web search, web fetch | 2 days |
| 2.3 | CortexBrain tools: memory store/recall, blackboard, lobe invoke | 1 day |
| 2.4 | WhatsApp adapter (Baileys Go port or CGo bridge) | 3 days |
| 2.5 | Message tool (cross-channel delivery) | 4 hours |
| 2.6 | Media pipeline (images, audio, documents) | 1 day |

**Deliverable:** Full tool-using agent on WhatsApp + Telegram.

### Phase 3: Scheduling & Stability (Week 4)
**Goal:** Cron, heartbeats, reliability hardening

| Task | Description | Effort |
|------|------------|--------|
| 3.1 | Cron scheduler (at/every/cron expressions) | 1 day |
| 3.2 | Heartbeat/proactive wake-up system | 1 day |
| 3.3 | Sleep Cycle integration | 4 hours |
| 3.4 | Message queuing + retry logic | 1 day |
| 3.5 | Graceful shutdown + session persistence | 4 hours |
| 3.6 | Channel reconnection logic | 1 day |
| 3.7 | Integration tests (crash recovery, failover) | 1 day |

**Deliverable:** Stable, self-healing gateway with scheduling.

### Phase 4: Cortex-TUI & Discord (Week 5)
**Goal:** Terminal interface, additional channels, swarm integration

| Task | Description | Effort |
|------|------------|--------|
| 4.1 | BubbleTea TUI scaffold (conversation panel, input, status bar) | 2 days |
| 4.2 | TUI Neural Bus event panel (real-time lobe activations) | 1 day |
| 4.3 | TUI health dashboard panel (services, GPU, memory) | 1 day |
| 4.4 | TUI memory search panel (MemCell query interface) | 4 hours |
| 4.5 | TUI theming (teal #0d7377, off-white #f8f7f4, SSH-compatible) | 4 hours |
| 4.6 | Discord adapter | 2 days |
| 4.7 | A2A Bridge integration (register, task accept/complete) | 1 day |

**Deliverable:** Native terminal interface + Discord channel.

### Phase 4.5: Cortex Web UI (Week 5-6)
**Goal:** Full web interface — chat, neural monitor, admin dashboard

| Task | Description | Effort |
|------|------------|--------|
| 4.8 | Web UI scaffold (Vite + React + Tailwind, embedded in Go binary) | 1 day |
| 4.9 | Web Chat panel (conversation interface, WebSocket real-time) | 2 days |
| 4.10 | Neural Monitor integration (port existing EEG monitor + 3D brain viewer) | 2 days |
| 4.11 | System Health dashboard (services, GPU, memory, agents, uptime) | 1 day |
| 4.12 | Memory Explorer (search, browse, inspect CortexBrain memories) | 1 day |
| 4.13 | Session Manager (list, inspect, switch conversations) | 1 day |
| 4.14 | Admin REST API (status, config, restart) | 1 day |
| 4.15 | Admin Config panel (model selection, channels, tools) | 1 day |
| 4.16 | Log Viewer (real-time structured log streaming) | 4 hours |
| 4.17 | JWT authentication gate | 4 hours |
| 4.18 | DM pairing + allowlist management | 1 day |
| 4.19 | Responsive design (mobile/tablet) | 1 day |
| 4.20 | Cortex theme (teal/off-white light, dark mode) | 4 hours |

**Deliverable:** Complete web UI with chat, neural visualization, admin tools, and monitoring.

### Phase 5: Polish & Migration (Week 6)
**Goal:** Production readiness, documentation, migration from OpenClaw

| Task | Description | Effort |
|------|------------|--------|
| 5.1 | Backup/restore tooling + cross-OS testing | 1 day |
| 5.2 | Prometheus metrics endpoint | 4 hours |
| 5.3 | Structured logging (JSON) | 4 hours |
| 5.4 | Architecture documentation + API spec | 1 day |
| 5.5 | Deployment guide (Linux, macOS, Docker Desktop) | 4 hours |
| 5.6 | Migration runbook: OpenClaw → Cortex-Gateway | 1 day |
| 5.7 | Norman acceptance testing | 2 days |

**Deliverable:** Production-ready Cortex-Gateway with full documentation.

---

## 7.5. UI Coverage Matrix

Cortex-Gateway provides **complete UI coverage** across four interface types:

| Capability | Messaging Channels | Cortex-TUI (Terminal) | Cortex Web UI | WebChat (Lightweight) |
|------------|-------------------|----------------------|---------------|----------------------|
| **Chat / Conversation** | ✅ WhatsApp, Telegram, Discord | ✅ Full | ✅ Full | ✅ Full |
| **Neural Monitor** | — | ✅ Event stream | ✅ 3D brain + EEG | — |
| **System Health** | — | ✅ Dashboard panel | ✅ Dashboard page | — |
| **Memory Search** | ✅ Via conversation | ✅ Search panel | ✅ Memory Explorer | — |
| **Blackboard / Tasks** | ✅ Via conversation | ✅ State panel | ✅ Mission Control view | — |
| **Admin / Config** | — | — | ✅ Full admin panel | — |
| **Log Viewer** | — | — | ✅ Real-time logs | — |
| **Session Management** | — | — | ✅ Session manager | — |
| **Mobile Access** | ✅ Native apps | — | ✅ Responsive | ✅ Responsive |
| **SSH Access** | — | ✅ Works over SSH | — | — |
| **No Install Required** | ✅ Existing apps | ✅ Terminal only | ✅ Any browser | ✅ Any browser |

### Existing Assets to Integrate

| Asset | Location | Stack | Integration Plan |
|-------|----------|-------|-----------------|
| **Neural Monitor (EEG)** | `~/.openclaw/workspace/neural-monitor.html` | Vanilla JS, single file | Embed directly or port to React component |
| **Cortex Monitor (3D)** | `Production/cortex-monitor/` | React, Three.js (R3F), Zustand, Recharts, Tailwind | Port as Web UI component, connect to gateway WS |
| **TUI Scaffold** | `CortexBrain/internal/tui/` | Go (empty scaffold: block/, components/, renderer/, styles/) | Build out with BubbleTea/Charm |
| **TUI Designer** | `CortexBrain/TUI-Redesign/cortex-tui-designer/` | Component + service definitions | Reference for TUI layout |
| **Avatar Web** | `Archive/cortex-avatar-web/` | Web-based avatar interface | Future integration (Phase 6+) |

### Interface Architecture

```
┌────────────────────────────────────────────────────────────────┐
│                    CORTEX-GATEWAY (Go binary)                   │
│                                                                 │
│  ┌─────────────────────────── HTTP Server ───────────────────┐ │
│  │                                                            │ │
│  │  /api/*        → REST API (admin, health, config)         │ │
│  │  /ws           → WebSocket (chat, events, logs)           │ │
│  │  /ui/*         → Cortex Web UI (React SPA, embedded)      │ │
│  │  /chat/*       → WebChat (lightweight, embedded)          │ │
│  │  /metrics      → Prometheus metrics                       │ │
│  │  /health       → Health check                             │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                 │
│  ┌─────── Channel Adapters ──────┐  ┌──── Cortex-TUI ────┐   │
│  │ WhatsApp │ Telegram │ Discord │  │ BubbleTea terminal  │   │
│  └──────────┴──────────┴─────────┘  └─────────────────────┘   │
│                                                                 │
│                    ┌── CortexBrain Core ──┐                    │
│                    │ Neural Bus + MemCell  │                    │
│                    │ + Blackboard + Lobes  │                    │
│                    └──────────────────────┘                    │
└────────────────────────────────────────────────────────────────┘
```

---

## 8. What We're Stealing (and from Where)

### From OpenClaw (Patterns, Not Code)
| Pattern | Why | Adaptation |
|---------|-----|------------|
| Baileys WhatsApp integration | Battle-tested, single-session invariant | Go port or Node.js sidecar |
| DM pairing security model | Smart access control for untrusted surfaces | Implement in Go |
| Cron scheduling (at/every/cron) | Flexible, proven pattern | Reimplement in Go |
| Skills/tool loading pattern | Clean extensibility | Go plugin interface |
| WebChat static UI concept | Simple, embeddable | Serve from gateway HTTP |
| Idempotency keys for dedup | Prevents duplicate processing | Implement in Go |
| Channel chunking & routing | Handles long responses gracefully | Implement in Go |
| Model failover chains | Reliability for inference | Use CortexBrain lane routing |

### From Nanobot (Architecture, Port to Go)
| Pattern | Why | Adaptation |
|---------|-----|------------|
| MessageBus pub-sub (asyncio.Queue) | Clean decoupling | Go channels + goroutines |
| AgentLoop (LLM ↔ tool iteration) | Clear, testable pattern | Direct Go implementation |
| ToolRegistry (register/execute) | Extensible, clean | Go interface pattern |
| LLMProvider abstraction | Pluggable backends | CortexBrain's lane router |
| SubagentManager | Background task isolation | Goroutine-based |
| JSONL session persistence | Simple, debuggable | MemCell API (CortexBrain) |
| Configuration simplicity | One JSON file | One YAML file |

### Native to CortexBrain (No Porting Needed)
| Subsystem | Role in Gateway |
|-----------|----------------|
| Neural Bus | IS the message bus (replaces MessageBus) |
| 25 Cognitive Lobes | IS the agent intelligence (replaces AgentLoop's LLM) |
| MemCell | IS the session store (replaces SessionManager files) |
| Blackboard | IS the shared context (replaces workspace files) |
| Sleep Cycle | IS the maintenance cron (replaces cleanup jobs) |
| Lane Routing | IS the provider router (replaces model selection) |
| State Store + Raft | IS the consistency layer (replaces distributed state) |
| JWT Auth | IS the security layer (replaces gateway auth) |

---

## 9. Risk Assessment

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| WhatsApp adapter complexity (Baileys is Node.js) | High | Medium | Option A: Go port of Baileys. Option B: Node.js sidecar with gRPC. Option C: Use whatsmeow (Go native) |
| CortexBrain API may need new endpoints | Medium | High | Plan endpoint additions as part of Phase 1; CortexBrain is ours to modify |
| Ollama container GPU passthrough issues | Medium | Low | Well-documented; NVIDIA Container Toolkit is mature |
| Channel-specific edge cases (rate limits, message size, formatting) | Medium | High | Address iteratively; start with happy path |
| Harold/swarm coordination during migration | Low | Medium | Run both systems in parallel during transition |
| Performance regression vs. OpenClaw | Medium | Low | Go is significantly faster than Node.js for this workload |

---

## 10. Decisions (Approved by Norman — 2026-02-02)

| # | Question | Decision | Notes |
|---|----------|----------|-------|
| 1 | **WhatsApp strategy** | **(a) whatsmeow — Go native** | Pure Go, no Node.js dependency, active maintenance |
| 2 | **Migration strategy** | **(c) Hybrid** — Shadow mode → Telegram cutover → WhatsApp cutover → full retirement | See Migration Plan below |
| 3 | **First deployment target** | **(a) Pink** | CortexBrain + GPU already there |
| 4 | **Timeline priority** | **(b) Quality — 6 weeks** | Critical infrastructure, no rushing |
| 5 | **Who builds it?** | **(b) Albert + Harold + Swarm** | Albert architects, Harold + swarm handle infra/deployment |
| 6 | **A2A Bridge** | **(a) Keep current Bridge** + plan Go replacement in later phase | Bridge works; rebuild is a future phase |
| 7 | **CortexBrain modifications** | **(a) Minimal** — use existing APIs, add new endpoints as needed | Architecture is solid |

### Migration Plan: Hybrid Strategy

**Rationale:** WhatsApp physically cannot run in parallel (single session per phone number). Telegram and Discord can be swapped independently. Shadow mode validates the system before any real traffic touches it.

```
Week 4: SHADOW MODE
  └── Cortex-Gateway receives message copies, processes but doesn't respond
  └── Validates routing, sessions, tools, response quality vs OpenClaw
  └── Fix all issues discovered

Week 5: TELEGRAM CUTOVER (Monday)
  └── Swap bot token → Cortex-Gateway
  └── Monitor 48 hours → fix issues → confirm stable by Friday

Week 6: WHATSAPP CUTOVER (Monday)  
  └── Stop OpenClaw WhatsApp → Start whatsmeow → QR scan
  └── Monitor 48 hours → confirm stable by Wednesday

Week 6: DISCORD + FULL RETIREMENT (Thursday)
  └── Move Discord → Stop OpenClaw → Archive config
  └── Cortex-Gateway is sole gateway ✅
```

**Rollback plan:** OpenClaw config preserved. Any channel can be rolled back within minutes (Telegram: swap token; WhatsApp: re-scan QR on OpenClaw; Discord: swap bot token).

---

## 11. Glossary

| Term | Definition |
|------|-----------|
| **Cortex-Gateway** | The new Go-based gateway service that replaces OpenClaw's control plane |
| **CortexBrain** | The 138K-line Go cognitive engine with 25 lobes, Neural Bus, MemCell, Blackboard |
| **Neural Bus** | CortexBrain's internal pub/sub event system (Go channels + WebSocket) |
| **MemCell** | CortexBrain's memory subsystem (episodic, semantic, procedural memories via SQLite) |
| **Blackboard** | CortexBrain's shared key-value state store accessible by all lobes |
| **Sleep Cycle** | CortexBrain's nightly maintenance routine (dedup, decay, consolidation) |
| **Lane Routing** | CortexBrain's model selection: local→Ollama, cloud→Grok/Claude, reasoning→Grok-4 |
| **A2A Bridge** | The agent-to-agent communication bridge (currently at Harold:18802) |
| **Channel Adapter** | A module that translates between a messaging platform's API and the gateway's internal format |
| **Agent Loop** | The iterative process: receive message → build context → call LLM → execute tools → respond |

---

## 12. Input Status

### Consulted Sources ✅
- [x] **OpenClaw codebase** — 1,743 JS modules, architecture docs, gateway protocol
- [x] **Nanobot codebase** — 4K LOC Python, agent loop, bus, session, tool patterns
- [x] **CortexBrain** — 138K LOC Go, 25 lobes, REST API, running on Pink:18892
- [x] **Container Framework Plan** — CONTAINER-FRAMEWORK-PLAN.md (15.4KB)
- [x] **Mission Control Plan** — CORTEXBRAIN-MISSION-CONTROL-PLAN.md (13.8KB)
- [x] **CortexHub Plan** — CORTEXHUB-PLAN.md (3.8KB)
- [x] **Primary Memory Plan** — CORTEXBRAIN-PRIMARY-MEMORY-PLAN.md (6.1KB)
- [x] **Swarm V2 Architecture** — memory archives, phase plans, tracker

### Pending Input ⏳
- [ ] **Harold** — OFFLINE (IP changed .229→.128, currently unreachable). Key questions for Harold:
  - Infrastructure requirements for hosting containerized gateway?
  - Preferred deployment target (Proxmox CT vs. Pink vs. dedicated)?
  - Bridge integration requirements?
  - Self-healing/monitoring preferences?
- [ ] **Pink** — CortexBrain running, Ollama operational. Key questions for Pink infrastructure:
  - CortexBrain API endpoints that need extension?
  - GPU resource allocation between Ollama and gateway?
  - Volume layout preferences for shared CortexBrain data?

> **Note:** Harold input will be incorporated when he comes back online. The PRD is structured to accept additive feedback without restructuring. Pink's infrastructure questions are captured as open items for Phase 1.

---

## 13. Alignment with First Principles

### Container Framework Plan Alignment
| Principle | How Cortex-Gateway Honors It |
|-----------|------------------------------|
| OS-Agnostic | Go static binary, Docker deployment |
| API-Managed | REST API for all lifecycle operations |
| Self-Healing | `restart: always` + health checks |
| Portable State | All state in CortexBrain (SQLite volumes) |
| Image = Truth | Single Dockerfile, no host dependencies |
| Service Discovery | Docker DNS (bridge, ollama, cortex-brain) |
| GPU Passthrough | Ollama container with nvidia device reservation |
| Secure by Default | Non-root containers, isolated networks, JWT auth |

### CortexBrain Architecture Alignment
| CortexBrain Subsystem | Gateway Integration | Status |
|----------------------|--------------------|---------| 
| Neural Bus | Gateway publishes inbound messages as Neural Bus events | Native — no adapter needed |
| 25 Cognitive Lobes | Gateway delegates processing to appropriate lobes | Native — REST API |
| MemCell | Gateway uses MemCell as session/memory store | Native — replaces flat files |
| Blackboard | Gateway reads/writes shared state | Native — REST API |
| Sleep Cycle | Gateway triggers nightly via cron | Native — existing schedule |
| Lane Routing | Gateway uses lanes for model selection | Native — no duplicate router |
| State Store + Raft | Gateway leverages for distributed consistency | Native — existing consensus |
| JWT Auth | Gateway authenticates via CortexBrain auth | Native — existing flow |

### Mission Control Plan Alignment
| Mission Control Concept | Cortex-Gateway Role |
|------------------------|---------------------|
| Blackboard = Task Board | Gateway writes tasks for Harold/workers |
| Neural Bus = Notifications | Gateway subscribes to agent notifications |
| MemCell = Shared Docs | Gateway stores conversation context here |
| Sleep Cycle = Auto-Standup | Gateway benefits from nightly consolidation |
| Executive = Task Routing | Gateway leverages for multi-agent delegation |

---

*"The gateway is CortexBrain's voice to the world. Not a wrapper — an extension."*

---

**Document Status:** Ready for Norman's review.
**Next:** Incorporate Harold's input when he comes online. Begin Phase 1 scaffolding upon approval.
