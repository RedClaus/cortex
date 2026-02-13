---
project: Cortex
component: Unknown
phase: Ideation
date_created: 2026-02-01T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:29.799710
---

# CortexHub — Unified Ecosystem Plan

**Created:** 2026-02-01
**Status:** IN PROGRESS

## Vision
CortexHub is the unified ecosystem where CortexBrain serves as the **central memory and cognitive layer** for all agents: Albert (OpenClaw), Harold (Swarm Overseer), and every worker in the swarm.

## Architecture

```
┌─────────────────────────────────────────────────┐
│                   CortexHub                      │
│                                                  │
│  ┌──────────┐    ┌──────────────┐    ┌────────┐ │
│  │  Albert   │◄──►│ CortexBrain  │◄──►│ Harold │ │
│  │ (OpenClaw)│    │ (Pink:18892) │    │(229)   │ │
│  └──────────┘    └──────┬───────┘    └────────┘ │
│                         │                        │
│              ┌──────────┼──────────┐             │
│              │          │          │              │
│         ┌────┴───┐ ┌───┴────┐ ┌──┴─────┐       │
│         │Workers │ │Health  │ │  CCP   │       │
│         │Pink/Red│ │Doc/etc │ │Cluster │       │
│         └────────┘ └────────┘ └────────┘       │
│                         │                        │
│              ┌──────────┴──────────┐             │
│              │    A2A Bridge       │             │
│              │  Harold:18802       │             │
│              └─────────────────────┘             │
└─────────────────────────────────────────────────┘
```

## CortexBrain Services (Pink:18892)
- **Memory API** — episodic, semantic, procedural memories
- **Knowledge API** — facts, architecture notes, preferences
- **Auth** — JWT, user-scoped memory isolation
- **25 Cognitive Lobes** — planning, reasoning, emotion, attention, etc.
- **Ollama Integration** — deepseek-coder-v2, 101ms latency
- **A2A Protocol** — JSON-RPC 2.0, registered on bridge

## Users/Agents
| Agent | Username | Role |
|-------|----------|------|
| Albert | albert | Primary AI partner (OpenClaw) |
| Harold | harold | Swarm overseer/coordinator |
| System | system | Internal/automated processes |

## Migration Plan

### Phase 1: Connect & Migrate (TODAY) ✅ IN PROGRESS
- [x] Deploy full CortexBrain binary (25 lobes, SQLite, pure-Go)
- [x] Connect to A2A Bridge (Harold:18802)
- [x] Register Albert user
- [ ] Register Harold user
- [ ] Migrate Albert's MEMORY.md → CortexBrain memories
- [ ] Migrate daily logs (memory/*.md) → CortexBrain episodic memories
- [ ] Create Albert sync script (OpenClaw → CortexBrain)

### Phase 2: Integration Hooks
- [ ] Albert heartbeat writes to CortexBrain
- [ ] Harold queries CortexBrain for shared state
- [ ] Worker agents report via bridge → CortexBrain stores
- [ ] Sleep Cycle activation (memory consolidation)

### Phase 3: Full CortexHub
- [ ] Dashboard/Web UI for CortexBrain
- [ ] Real-time memory search across all agents
- [ ] Knowledge graph connections
- [ ] Conversation history persistence
- [ ] Agent personality persistence

## Credentials
- CortexBrain: Pink:18892
- Auth: albert / CortexBrain2026!
- JWT Secret: CortexHub2026SuperSecretKeyForJWT!x
- Bridge: Harold:18802
