---
project: Cortex
component: Agents
phase: Ideation
date_created: 2026-02-08T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-08T12:13:22.995409
---

# Pinky A2A Integration

**Date:** 2026-02-08
**Status:** Complete
**Type:** Integration
**Projects:** Pinky, CortexBrain

## Overview

Pinky now supports Agent-to-Agent (A2A) protocol communication with CortexBrain, enabling Pinky to serve as a frontend gateway while CortexBrain handles cognitive processing.

## Architecture

```
┌─────────────────────────┐         ┌─────────────────────────┐
│         Pinky           │         │      CortexBrain        │
│  (Gateway/Frontend)     │  A2A    │    (Cognitive Engine)   │
├─────────────────────────┤ ◄─────► ├─────────────────────────┤
│  - TUI (BubbleTea)      │ HTTP    │  - 20 Cognitive Lobes   │
│  - WebUI (React)        │ JSON-RPC│  - PhaseExecutor        │
│  - Tool Execution       │         │  - Blackboard           │
│  - Permission System    │         │  - ThinkingStrategy     │
└─────────────────────────┘         └─────────────────────────┘
```

## Integration Points

### CortexBrain A2A Server (Port 8080)

Located: `CortexBrain/internal/a2a/`

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/health` | GET | Health check |
| `/v1/think` | POST | Process thinking request |
| `/v1/memory` | POST | Store memory |
| `/v1/memory/search` | POST | Search memories |

### Pinky RemoteBrain Client

Located: `Pinky/internal/brain/remote.go`

Configuration:
```yaml
brain:
  mode: remote
  remote_url: http://localhost:8080
  remote_token: ""  # Optional auth
```

## Related Plans

### Model Picker Implementation
**Location:** `Pinky/docs/plans/Pinky-model-picker.md`

Adds model selection and auto-routing settings to Pinky:
- Auto-routing toggle (AutoLLM on/off)
- Model picker for cloud providers (Anthropic, OpenAI, Groq)
- Model picker for local models (Ollama)
- Available in wizard and TUI settings panel

## Files Modified

### CortexBrain
- `internal/a2a/server.go` - A2A server implementation
- `internal/a2a/pinky_compat.go` - Pinky-compatible endpoints
- `pkg/brain/executor.go` - Fixed Blackboard frozen panic

### Pinky
- `internal/brain/remote.go` - RemoteBrain client
- `internal/brain/factory.go` - Brain factory (embedded/remote)
- `cmd/pinky/main.go` - Factory integration

## Testing

```bash
# Start CortexBrain A2A server
cd CortexBrain && go run ./cmd/cortex-server --port 8080

# Start Pinky in remote mode
cd Pinky && ./pinky --tui
# With config: brain.mode: remote, brain.remote_url: http://localhost:8080
```

## Future Enhancements

1. **Streaming Support** - SSE for real-time responses
2. **Authentication** - Token-based auth for remote brain
3. **Memory Sync** - Bidirectional memory sharing
4. **Multi-Brain** - Support multiple CortexBrain instances
