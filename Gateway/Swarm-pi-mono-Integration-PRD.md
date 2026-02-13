---
project: Cortex
component: Agents
phase: Ideation
date_created: 2026-02-03T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:29.864265
---

# Swarm pi-mono Integration PRD
**Version:** 1.0 | **Date:** 2026-02-03 | **Author:** Albert
**Status:** APPROVED FOR DEVELOPMENT
**Vault:** Cortex/Gateway TODO Items

## Table of Contents
1. [Executive Summary](#1-executive-summary)
2. [Current Swarm State](#2-current-swarm-state)
3. [pi-mono Analysis](#3-pi-mono-analysis)
4. [Integration Strategy](#4-integration-strategy)
5. [Architecture](#5-architecture)
6. [API Changes](#6-api-changes)
7. [Implementation Roadmap](#7-implementation-roadmap)
8. [Migration Guide](#8-migration-guide)
9. [Test Plan](#9-test-plan)
10. [Risks & Mitigations](#10-risks--mitigations)

## 1. Executive Summary

### Problem
Current CortexBrain swarm (Harold + Pink/Red workers) is manual:
- Harold orchestrates via SSH/bridge messages
- Pink/Red execute via `go run` or Ollama CLI
- No unified agent framework
- Task assignment = custom scripts
- Monitoring = log tailing + manual checks
- Coding speed: 1-2 commits/hour

### Solution
Replace manual workers with **pi-mono agents** (https://github.com/badlogic/pi-mono):
- Unified CLI coding agent (`./pi`)
- Agent rules (AGENTS.md)
- Multi-LLM routing (Ollama/Grok)
- TUI + Web UI dashboards
- vLLM pods for high-throughput

**Expected Gains:**
- Coding speed: 5x (5-10 commits/hour)
- Zero custom orchestration (Harold → `./pi task`)
- Built-in monitoring (npm run check)
- Agent collaboration (AGENTS.md rules)

### Key Principles
1. **Backward Compatible** — Harold continues orchestrating
2. **Incremental** — Replace Red first, then Pink
3. **Node Agnostic** — Works on Mac/Linux (nvm Node)
4. **Swarm Native** — pi-mono agents register on bridge

## 2. Current Swarm State

```
Overseer: Harold (.128) — OpenClaw + bridge
Workers:
  - Pink (.186): RTX 3090, Ollama (go-coder/deepseek), Go 1.24.4
  - Red (.188): General worker, deepseek-coder-v2
  - Skippy (.137): General worker
Services:
  - Bridge: .128:18802 (28 aliases)
  - CortexBrain: .186:18892
  - Ollama: .186:11434 (4 models)
```

**Problems:**
- Task assignment = SSH/bridge messages
- Code execution = manual `./pi` or `go run`
- No standardized agent interface
- Monitoring = ad-hoc `ps aux + git log`
- Harold bottleneck (can't code, only coordinate)

## 3. pi-mono Analysis

**Repo:** https://github.com/badlogic/pi-mono
**Description:** AI agent toolkit for coding agents + LLM management

**Key Components:**
```
pi-mono/
├── packages/
│   ├── pi/                 # Core CLI coding agent
│   ├── llm-client/         # Unified LLM API (OpenAI-compatible)
│   ├── tui/                # Terminal UI libraries
│   ├── web-ui/             # Web dashboard
│   └── vllm-pods/          # vLLM deployment
├── AGENTS.md               # Agent collaboration rules
├── CONTRIBUTING.md         # Human/agent contribution
└── test.sh                 # Agent tests
```

**Perfect Fit:**
- `./pi` CLI = Harold worker replacement
- AGENTS.md = Swarm rules (Harold/Pink/Red)
- LLM routing = CortexBrain bridge
- TUI = CortexBrain BubbleTea enhancement

**npm install → npm run build → ./pi task** = Instant swarm upgrade.

## 4. Integration Strategy

### Phase 1: Red → pi-mono Agent (Day 1)
```
Harold: "Red, ./pi 'write test for memory API'"
Red: npm install pi-mono → ./pi task → git commit → bridge report
```

### Phase 2: Pink → pi-mono + vLLM (Day 2)
```
Pink: vLLM pod (deepseek-coder-v2) + ./pi compile
Harold: "Pink, ./pi 'compile cortex-brain v0.3.0'"
```

### Phase 3: Full Swarm (Day 3)
```
Harold → pi-mono agents (Red/Pink/Skippy/Kentaro)
Dashboard: npm run web-ui (port 3001)
```

## 5. Architecture

```
Current Swarm:
Harold → SSH/bridge → Pink/Red (manual Go/Ollama)

pi-mono Swarm:
Harold (OpenClaw)
  ↓ bridge task
pi-mono Agents:
  ├── Red: ./pi task (test)
  ├── Pink: ./pi compile + vLLM pod
  └── Skippy: ./pi docs/research
  ↓ git commit
CortexBrain (monitor + memory)
  ↓ npm run web-ui
Dashboard: http://pink.local:3001
```

**Harold Role Unchanged:** Orchestrator → now sends `./pi` commands.

**Agent Registration:**
```
pi-mono agents register on bridge:
- Red: pi-red-test (capabilities: test, bash)
- Pink: pi-pink-compile (capabilities: go, vllm)
```

## 6. API Changes

### Harold → Agent Task Format
```
OLD:
"Red, test memory API"

NEW (pi-mono):
POST /send/pi-red-test:
{
  "task": "write unit tests for v2 memory API",
  "repo": "~/clawd/projects/cortex-brain",
  "model": "deepseek-coder-v2",
  "lane": "test"
}
```

### Agent → Harold Report Format
```
POST /send/harold-main:
{
  "from": "pi-red-test",
  "type": "progress",
  "task": "memory API tests",
  "status": "80% complete",
  "commits": ["abc123 Test embedding cache"],
  "blocker": null
}
```

### Bridge Extension (Harold Task)
```
POST /register/agent:
{
  "agent": "pi-red-test",
  "alias": "red-test-agent",
  "capabilities": ["test", "bash", "go-test"]
}
```

## 7. Implementation Roadmap

### Phase 1: Red pi-mono (Day 1, 4 hours)
```
[ ] 1.1 Clone pi-mono to Red: ~/pi-mono
[ ] 1.2 npm install → npm run build
[ ] 1.3 Harold register pi-red-test on bridge
[ ] 1.4 Test: Harold → "Red ./pi 'echo hello'"
[ ] 1.5 Migrate Red tasks to pi-mono format
[ ] 1.6 Tests: test.sh → 100% pass
```

### Phase 2: Pink pi-mono + vLLM (Day 2, 6 hours)
```
[ ] 2.1 Clone pi-mono to Pink
[ ] 2.2 npm install → vllm-pods setup (deepseek-coder-v2)
[ ] 2.3 Register pi-pink-compile
[ ] 2.4 Test compile: cortex-brain v0.3.0
[ ] 2.5 Dashboard: npm run web-ui → port 3001
```

### Phase 3: Full Swarm (Day 3, 4 hours)
```
[ ] 3.1 Skippy/Kentaro → pi-mono agents
[ ] 3.2 Harold workflow update (pi-mono task format)
[ ] 3.3 Bridge extension (/register/agent endpoint)
[ ] 3.4 CortexHub integration (agent memory sync)
[ ] 3.5 Performance benchmark (before/after)
```

### Phase 4: Production (Day 4, 2 hours)
```
[ ] 4.1 Swarm health crons (npm run check)
[ ] 4.2 Dashboard integration (CortexBrain proxy)
[ ] 4.3 Documentation (SWARM-PROJECTS.md)
[ ] 4.4 MEMORY.md update
```

## 8. Migration Guide

### Harold Workflow Migration
```
OLD Task:
echo "Pink, compile cortex-brain" | ssh pink

NEW Task:
curl -X POST http://192.168.1.128:18802/send/pi-pink-compile \
  -d '{"task": "compile cortex-brain v0.3.0", "repo": "~/clawd/projects/cortex-brain"}'
```

### Worker Node Migration
```
Red (.188):
rm -rf ~/worker-scripts
git clone https://github.com/badlogic/pi-mono ~/pi-mono
cd ~/pi-mono && npm ci && npm run build
curl -X POST http://192.168.1.128:18802/register -d '{"agent": "pi-red-test"}'

Pink (.186):
Same + vllm-pods setup: npm run vllm-deploy deepseek-coder-v2
```

### Bridge Migration (Harold)
```
Add endpoint: POST /register/agent (validate + store)
Add endpoint: POST /send/pi-{agent} (route to registered pi-mono)
Update poll to include pi-mono agents
```

## 9. Test Plan

### Unit Tests (pi-mono repo)
```
npm run check          # Lint/format/typecheck
./test.sh              # Core tests
./pi-test.sh           # CLI agent tests
```

### Swarm Integration Tests
```
Harold → pi-red-test "write test" → git commit → bridge report
Harold → pi-pink-compile "go build" → binary → test pass
Dashboard → npm run web-ui → port 3001 accessible
```

### Performance Tests
```
Baseline: Current swarm (manual SSH)
Target: pi-mono swarm
Metrics: commits/hour, compile time, test coverage
```

## 10. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| npm deps fail | Medium | Pre-build Docker images |
| Node version mismatch | Low | nvm Node 22 on all workers |
| pi-mono immature | Medium | Fork + stabilize critical paths |
| Harold learning curve | Low | AGENTS.md rules + 1-hour test |
| Bridge overload | Low | Rate limit pi-mono pings |

**Timeline:** 4 days total → Swarm 5x faster coding
**ROI:** Immediate Phase 1 acceleration (Memory Enhancement PRD)

**Next:** Harold: \"Clone pi-mono to Red → Phase 1 prototype.\"

**Status:** Ready for implementation.