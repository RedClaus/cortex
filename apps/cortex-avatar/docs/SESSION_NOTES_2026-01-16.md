---
project: Cortex
component: Docs
phase: Design
date_created: 2026-01-16T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:20:39.830470
---

# CortexAvatar Session Notes
**Date**: January 16, 2026
**Session Focus**: Voice latency investigation (cortex-03 backend)

---

## Executive Summary

CortexAvatar connects to CortexBrain (cortex-03) for voice processing. Voice latency issues traced to the backend - not CortexAvatar itself. Fix implemented in cortex-03: dedicated Groq provider for voice mode.

---

## Current Status

### CortexAvatar (This Repo)
- **Build Status**: Stable
- **Core Functionality**: Working
- **Voice I/O**: Functional but latency dependent on backend

### CortexBrain Backend (cortex-03)
- **Issue Identified**: Voice mode using slow local LLM (Ollama)
- **Fix Applied**: Groq integration for voice inference
- **Status**: Code complete, awaiting test

---

## Integration Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        CORTEXAVATAR                              │
│                                                                  │
│  Frontend (Svelte)              Backend (Go)                     │
│  ┌─────────────────┐           ┌─────────────────┐              │
│  │ Avatar UI       │◄─────────►│ AudioBridge     │              │
│  │ Chat Interface  │  Events   │ A2A Client      │              │
│  │ Settings Panel  │           │ TTS/STT Mgmt    │              │
│  └─────────────────┘           └────────┬────────┘              │
│                                         │                        │
└─────────────────────────────────────────┼────────────────────────┘
                                          │ A2A/JSON-RPC
                                          ▼
                              ┌───────────────────────┐
                              │     CortexBrain       │
                              │   (localhost:8080)    │◄── Groq (voice)
                              │                       │◄── Ollama (chat)
                              └───────────────────────┘
```

---

## Voice Latency Analysis

### Observed Latency (Before Fix)
- Voice mode: 3273ms - 4188ms (target: 500ms)
- Root cause: Ollama llama3.2:3b inference (~2.5-3.5s)

### Expected Latency (After Fix)
- Voice mode: ~200-500ms with Groq
- Groq model: `llama-3.3-70b-versatile`

### Fix Location
- **File**: `cortex-03/cmd/cortex-server/main.go`
- **Change**: Dedicated Groq provider for VoiceExecutive

---

## CortexAvatar Components Status

| Component | Status | Notes |
|-----------|--------|-------|
| AudioBridge | Working | TTS/STT routing functional |
| A2A Client | Working | Connects to localhost:8080 |
| TTS (OpenAI) | Working | Using `nova` voice |
| STT (Groq Whisper) | Working | Free tier, fast |
| Avatar Animation | Working | Svelte frontend |
| Settings Panel | Working | Voice selection UI |

---

## Known Issues

### Backend (cortex-03)
1. Voice latency with local LLM - **FIX APPLIED**
2. STT noise detection - picking up ambient noise
3. Non-voice chat slow (22-34s) - brain processing

### CortexAvatar
1. TTS duplication - historical issue, monitor
2. A2A error handling - could be improved
3. dnet integration - not yet connected

---

## Testing Required

### After cortex-03 Restart
1. Launch CortexAvatar: `wails dev`
2. Verify A2A connection to localhost:8080
3. Test voice input/output latency
4. Check logs for "Groq" provider selection

### Verification Steps
```bash
# Check cortex-server is using Groq for voice
grep -i groq /tmp/cortex-server.log

# Expected log:
# [Main] Voice Executive using Groq (fast cloud) for voice mode
```

---

## Configuration

### Required Environment
```bash
# ~/.cortex/.env
GROQ_API_KEY=gsk_***  # Required for voice (STT + LLM)
OPENAI_API_KEY=sk-*** # Required for TTS
```

### Server Endpoints
| Service | Endpoint | Purpose |
|---------|----------|---------|
| CortexBrain | localhost:8080 | A2A server |
| Ollama | localhost:11434 | Local LLM |
| MLX | localhost:8081 | Apple Silicon LLM |

---

## Development Commands

```bash
# CortexAvatar
cd /Users/normanking/ServerProjectsMac/Development/cortex-avatar
wails dev

# CortexBrain (backend)
cd /Users/normanking/ServerProjectsMac/Development/cortex-03
go build -o /tmp/cortex-server ./cmd/cortex-server
source ~/.cortex/.env && /tmp/cortex-server
```

---

## Session Continuity

### What Was Done (Jan 16)
1. Analyzed voice latency logs
2. Identified Ollama as bottleneck
3. Implemented Groq integration in cortex-03
4. Built cortex-server with fix

### What Needs Testing
1. Restart cortex-server
2. Test voice latency (~300-500ms expected)
3. Verify CortexAvatar voice interaction

### Reference Documents
- `cortex-03/SESSION_NOTES_2026-01-16_VoiceLatency.md` - Detailed technical analysis
- `cortex-03/internal/voice/executive.go` - VoiceExecutive implementation
- `cortex-03/cmd/cortex-server/main.go` - Server configuration
