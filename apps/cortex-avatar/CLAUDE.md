---
project: Cortex
component: Unknown
phase: Ideation
date_created: 2026-01-16T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:20:10.577554
---

# CLAUDE.md - CortexAvatar

This file provides guidance to Claude Code when working with the CortexAvatar codebase.

---

## Current Session State (January 16, 2026)

### Recent Work
- **Voice latency analysis** - Identified Ollama as bottleneck (3-4s latency)
- **Groq integration** - Implemented in cortex-03 for fast voice (~300-500ms)
- **Status**: Code complete in cortex-03, awaiting test

### Next Steps
1. Restart cortex-server with Groq provider
2. Test voice latency in CortexAvatar
3. Verify STT noise filtering

### Key Session Docs
- `docs/SESSION_NOTES_2026-01-16.md` - CortexAvatar session notes
- `cortex-03/SESSION_NOTES_2026-01-16_VoiceLatency.md` - Technical analysis

---

## Project Overview

**CortexAvatar** is a desktop companion application that provides voice, eyes, and ears for CortexBrain. Built with:
- **Wails v2** - Go + Svelte desktop framework
- **Go Backend** - Audio processing, A2A client, TTS/STT providers
- **Svelte Frontend** - Animated avatar, chat UI, settings

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        CORTEXAVATAR                              │
│                                                                 │
│  Frontend (Svelte)              Backend (Go)                    │
│  ┌─────────────────┐           ┌─────────────────┐             │
│  │ Avatar UI       │◄─────────►│ AudioBridge     │             │
│  │ Chat Interface  │  Events   │ A2A Client      │             │
│  │ Settings Panel  │           │ TTS/STT Mgmt    │             │
│  └─────────────────┘           └────────┬────────┘             │
│                                         │                       │
└─────────────────────────────────────────┼───────────────────────┘
                                          │ A2A/JSON-RPC
                                          ▼
                              ┌───────────────────────┐
                              │     CortexBrain       │
                              │   (localhost:8080)    │
                              └───────────────────────┘
```

## Key Files

| Path | Purpose |
|------|---------|
| `main.go` | Application entry point |
| `internal/bridge/audio_bridge.go` | Audio/voice handling, TTS, STT |
| `internal/a2a/client.go` | A2A protocol client |
| `internal/tts/` | TTS providers (OpenAI, Piper, macOS) |
| `internal/stt/` | STT providers (Groq Whisper) |
| `internal/avatar/controller.go` | Avatar state management |
| `frontend/src/stores/audio.ts` | Frontend audio state |
| `frontend/src/App.svelte` | Main UI component |

## Development Commands

```bash
# Development mode
wails dev

# Build production
CGO_ENABLED=1 GOTOOLCHAIN=local ~/go/bin/wails build

# Run diagnostics
./scripts/diagnose.sh

# Run specific diagnostic
./scripts/diagnose.sh stt    # Voice input issues
./scripts/diagnose.sh tts    # Voice output issues
./scripts/diagnose.sh a2a    # Protocol issues
```

## Fixit Agent Instructions

When diagnosing issues in CortexAvatar, follow this systematic approach:

### Step 1: Run Diagnostics First
```bash
./scripts/diagnose.sh all
```

### Step 2: Check Logs
```bash
# Server logs
tail -50 /tmp/cortex-server.log

# Look for errors
grep -i "error\|fail" /tmp/cortex-server.log | tail -20
```

### Step 3: Common Issues & Fixes

#### Voice Input Not Working
1. Check GROQ_API_KEY is set in `~/.cortex/.env`
2. Verify microphone permissions in System Settings
3. Check `frontend/src/stores/audio.ts` for STT errors

#### Voice Output Issues (Double Speech, No Sound)
1. TTS is handled in `internal/bridge/audio_bridge.go:speakText()`
2. Frontend should NOT call SpeakText - backend handles it
3. Check `cortex:response` event handler doesn't duplicate TTS

#### A2A Connection Issues
1. Verify CortexBrain is running: `pgrep -f cortex-server`
2. Test endpoint: `curl http://localhost:8080/.well-known/agent-card.json`
3. Check `internal/a2a/client.go` for connection errors

#### Slow Responses
1. Check if dnet is running: `curl http://localhost:9080/health`
2. Verify model is loaded in dnet cluster
3. Check CortexBrain logs for processing times

#### Voice Latency (January 2026 Fix)
Voice was slow (3-4 seconds) because CortexBrain used local LLM (Ollama) for voice inference.
**Fix applied in cortex-03**: Dedicated Groq provider for voice mode (~300-500ms).

To verify fix is active:
```bash
# Check cortex-server log for Groq
grep -i "groq" /tmp/cortex-server.log
# Should see: "[Main] Voice Executive using Groq (fast cloud) for voice mode"

# If you see this warning, voice will be slow:
# "[Main] Voice Executive using local provider - voice will be slow"
```

**Resolution**: Ensure `GROQ_API_KEY` is set in `~/.cortex/.env`

### Step 4: Fix Patterns

**Audio Bridge Issues:**
- Always use `cancelOngoingSpeech()` before new TTS
- Use `ttsMu` mutex for thread-safe TTS access
- Filter system messages with `shouldSkipTTS()`

**Frontend Audio Issues:**
- TTS is controlled by Go backend, not frontend
- Frontend only handles playback via `audio:playback` event
- Don't call `SpeakText()` from multiple places

**A2A Protocol Issues:**
- Use correct message format: `{"kind":"text","text":"..."}`
- Handle streaming responses properly
- Check task state transitions

### Step 5: Consult Troubleshooting Guide

For detailed fixes, see: `docs/TROUBLESHOOTING.md`

## Configuration

| Setting | Location | Default |
|---------|----------|---------|
| Server URL | `internal/config/config.go` | `http://localhost:8080` |
| TTS Voice | `~/.cortex/config.yaml` | `nova` |
| STT Provider | `internal/stt/` | Groq Whisper |
| Sample Rate | Audio config | 16000 Hz |

## Dependencies

- CortexBrain server must be running on port 8080
- Optional: dnet cluster on port 9080 for local LLM
- Optional: Piper TTS for high-quality local voice
- Required: GROQ_API_KEY for voice input (free tier)


---

## Workspace Standards Reference

This project inherits the **Cortex Project Rules & Standards** from the workspace root.

**See:** [ServerProjectsMac/CLAUDE.md](../CLAUDE.md#cortex-project-rules--standards) for:
- Software Engineering Principles (SOLID, DRY, naming conventions)
- Testing & Quality Control (TDD, coverage, edge cases)
- Bug Fixing & Diagnostics (root cause analysis, regression tests)
- Security & Reliability (input validation, secrets management)
- Documentation & Workflow (conventional commits, self-correction)
- Language-Specific Standards (linting, testing, package managers)


---

## Software Manufacturing Process

This project follows a structured software manufacturing process to ensure quality and consistency across the full development lifecycle.

**Full specification:** See [`.claude/process/PROCESS.md`](.claude/process/PROCESS.md)

### Quick Reference

| Phase | Purpose | Key Artifacts |
|-------|---------|---------------|
| Discovery | Understand the problem | requirements.md, success-criteria.md |
| Design | Define the solution | architecture.md, API contracts, ADRs |
| Implementation | Build the solution | Source code, unit tests |
| Testing | Verify correctness | Test plan, coverage reports |
| Deployment | Release safely | Release notes, rollback plan |
| Operations | Monitor and maintain | Incident logs, retrospectives |

### Process Commands

| Command | Action |
|---------|--------|
| `process:status` | Show current phase and gate status |
| `process:advance` | Validate gates and transition to next phase |
| `process:init [name]` | Initialize directory structure for new feature |
| `process:gate-check` | Audit artifacts against current phase |

### State Tracking

Process state is maintained in `.claude/process/state.json`. Always check current phase status before beginning work on a feature.
