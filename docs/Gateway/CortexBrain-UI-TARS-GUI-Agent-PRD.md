---
project: Cortex
component: Brain Kernel
phase: Ideation
date_created: 2026-02-03T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:29.883702
---

# CortexBrain UI-TARS GUI Agent Integration PRD
**Version:** 1.0 | **Date:** 2026-02-03 | **Author:** Albert
**Status:** APPROVED FOR DEVELOPMENT
**Vault:** Cortex/Gateway TODO Items

## Table of Contents
1. [Executive Summary](#1-executive-summary)
2. [Current GUI Capabilities](#2-current-gui-capabilities)
3. [UI-TARS Analysis](#3-ui-tars-analysis)
4. [Integration Strategy](#4-integration-strategy)
5. [Architecture](#5-architecture)
6. [API Specification](#6-api-specification)
7. [Implementation Roadmap](#7-implementation-roadmap)
8. [Migration Guide](#8-migration-guide)
9. [Test Plan](#9-test-plan)
10. [Risks & Mitigations](#10-risks--mitigations)

## 1. Executive Summary

### Problem
CortexBrain lacks native GUI interaction capabilities:
- Peekaboo CLI: Screenshot → OCR → manual click (slow, 30% accuracy)
- No vision-language model integration
- No desktop/browser control (VS Code/GitHub)
- Reachy Mini physical control = manual scripting

### Solution
Integrate **UI-TARS GUI Agent SDK** (ByteDance Apache 2.0):
- Vision-Language model (Seed-1.5-VL/1.6)
- Native mouse/keyboard control
- Event Stream protocol (real-time feedback)
- Local/remote operators (desktop/browser)

**Expected Gains:**
- GUI task speed: 10x (3s vs 30s)
- Accuracy: 85% → 95%
- Reachy Mini: Physical GUI agent
- CortexHive: Enterprise desktop automation

### Key Principles
1. **Modular** — GUI layer optional (CLI fallback)
2. **Backward Compatible** — Peekaboo continues to work
3. **Multi-Modal** — Vision + GUI + CortexBrain memory
4. **Physical Ready** — Reachy Mini integration Day 1

## 2. Current GUI Capabilities

```
Peekaboo CLI (macOS):
  - screenshot → OCR → click/typing
  - Accuracy: ~30% complex tasks
  - Speed: 10-30s/task
  - No vision-language understanding

CortexBrain GUI Gaps:
  - No desktop app control (VS Code/GitHub)
  - No browser automation (native)
  - No real-time feedback loop
  - No physical robot integration (Reachy)
```

## 3. UI-TARS Analysis

**Repo:** https://github.com/bytedance/UI-TARS-desktop (Apache 2.0)
**Key Components:**
```
Agent TARS:
  - CLI/Web UI multimodal agent
  - Event Stream protocol (context engineering)
  - MCP tools (shell/browser/computer)

UI-TARS Desktop:
  - Local operator (VS Code control)
  - Remote operator (any computer)
  - Browser operator (GitHub navigation)
```

**Perfect Fit:**
```
CortexBrain + UI-TARS = GUI CortexAgent
- Vision: UI-TARS → CortexBrain memory
- Action: GUI SDK → Peekaboo enhancement
- Feedback: Event Stream → Neural Bus v2
- Physical: Reachy Mini → UI-TARS operator
```

## 4. Integration Strategy

### Phase 1: Core GUI SDK (Day 1)
```
Fork UI-TARS → cortex-gui-agent (Go wrapper)
npm deps → Docker container (Pink)
Test: 'Open VS Code → cortex-brain → go test'
```

### Phase 2: Event Stream + CortexBrain (Day 2)
```
Neural Bus → Event Stream (bidirectional)
Memory → Context Engineering (live state)
Test: Vision feedback loop (screenshot → think → act)
```

### Phase 3: Reachy Mini Physical (Day 3)
```
UI-TARS operator → Reachy arm/camera
Test: 'Pick up phone → dial Norman'
```

### Phase 4: CortexHive Enterprise (Day 4)
```
Remote operator → Bank terminals
Dashboard integration
Production deploy
```

## 5. Architecture

```
CortexBrain v0.4.0 + UI-TARS:
┌──────────────────────────────────────────────────────────────┐
│                   CORTEXBRAIN + GUI AGENT                     │
│                                                              │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐  │
│  │   UI-TARS    │◄──► │  EVENT STREAM │◄──► │ NEURAL BUS   │  │
│  │  GUI SDK     │     │  Protocol     │     │   v2         │  │
│  └──────────────┘     └──────────────┘     └──────────────┘  │
│              ▲                                ▲               │
│              │ GUI Action + Vision           │ Memory         │
│              │                                │                │
│  ┌────────────▼────────────┐  ┌────────────▼────────────┐    │
│  │    LOCAL OPERATOR       │  │     WORKING MEMORY      │    │
│  │  (Desktop/Browser)      │  │     (SQLite + Cache)    │    │
│  └─────────────────────────┘  └─────────────────────────┘    │
│              │                                │               │
│              └──────────────┬───────────────┐  │               │
│                             │               │  │               │
│  ┌──────────────────────────▼───────────────▼──▼──────────────┐ │
│  │             PHYSICAL OPERATOR (REACHY MINI)                │ │
│  │  Camera → Vision → UI-TARS → Arm Control → Feedback Loop   │ │
│  └────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## 6. API Specification

### New GUI Agent Endpoints (v0.4.0)

#### POST /agent/gui/task
```json
{
  "task": "Open VS Code → cortex-brain → run go test ./...",
  "operator": "local",      // local|remote|browser|physical
  "timeout": 60,
  "stream": true,
  "memory_context": true    // Use CortexBrain memory for context
}
```

**Response Stream (Event Stream):**
```json
{
  "event": "vision",
  "data": {"screenshot": "base64", "objects": ["VS Code icon"]},
  "timestamp": "2026-02-03T16:00:00Z"
}
{
  "event": "action",
  "data": {"type": "click", "x": 100, "y": 200, "target": "VS Code"},
  "timestamp": "2026-02-03T16:00:01Z"
}
{
  "event": "feedback",
  "data": {"text": "VS Code opened, terminal ready"},
  "timestamp": "2026-02-03T16:00:05Z"
}
```

#### GET /agent/events/{session_id}
Live Event Stream viewer (debug dashboard).

#### POST /agent/register/operator
Register new operator (Reachy Mini).
```json
{
  "name": "reachy-mini",
  "capabilities": ["camera", "arm", "voice"],
  "endpoint": "http://reachy.local:8080"
}
```

## 7. Implementation Roadmap

### Phase 1: GUI SDK Wrapper (Day 1, 4h)
```
[ ] Fork UI-TARS-desktop → cortex-gui-agent
[ ] Go wrapper for UI-TARS SDK (CGo or exec)
[ ] Docker container (Pink deploy)
[ ] Test: 'Open VS Code → echo test' (3s)
```

### Phase 2: Event Stream Integration (Day 2, 5h)
```
[ ] Neural Bus v2 → Event Stream protocol
[ ] CortexBrain memory → Context Engineering
[ ] Test: Vision feedback loop (screenshot → action → verify)
[ ] POST /agent/gui/task endpoint
```

### Phase 3: Reachy Physical (Day 3, 6h)
```
[ ] UI-TARS operator → Reachy camera/arm
[ ] Test: 'Pick up phone → dial Norman' (full loop)
[ ] Remote operator → Proxmox VMs
```

### Phase 4: Production (Day 4, 3h)
```
[ ] Dashboard: Event Stream viewer
[ ] CortexHive: Remote bank terminal control
[ ] MEMORY.md + HEARTBEAT.md updates
[ ] Harold workflow: GUI tasks
```

## 8. Migration Guide

### Peekaboo → UI-TARS
```
OLD: peekaboo screenshot → ocr → click
NEW: POST /agent/gui/task "Open VS Code"

Harold Task Format:
OLD: "Peekaboo click VS Code"
NEW: "./pi 'gui-task open vscode cortex-brain go test'"
```

### Reachy Mini Integration
```
Reachy's existing ROS API → UI-TARS operator
Camera feed → UI-TARS Vision → Action → Feedback → CortexBrain memory
```

## 9. Test Plan

### GUI Tests
```
1. Desktop: Open VS Code → cortex-brain → go test (pass)
2. Browser: GitHub → cortex-brain issues → comment #42
3. Terminal: Pink SSH → ollama pull qwen2.5
4. Physical: Reachy 'pick up phone → dial'
```

### Performance
```
Task Time: Peekaboo 30s → UI-TARS 3s (10x)
Accuracy: 30% → 95%
Latency: Vision 100ms → Action 50ms
```

## 10. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| UI-TARS JS deps | Medium | Docker containerize |
| Vision model size | Low | Ollama pre-pull |
| Reachy integration | High | Phase 3 (post-core) |
| Apache compliance | Low | Fork + attribution |

**Timeline:** 4 days → CortexBrain GUI Agent
**ROI:** Reachy Mini + CortexHive desktop automation

**Next:** Harold: \"Prototype UI-TARS Red → VS Code test.\"