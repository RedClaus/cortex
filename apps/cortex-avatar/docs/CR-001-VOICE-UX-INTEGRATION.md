---
project: Cortex
component: Docs
phase: Design
date_created: 2026-01-16T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:20:39.157453
---

# CR-001: Voice UX Integration - CortexAvatar Voice Mode

## Overview

This CR defines the CortexAvatar-side changes to support the Voice Executive architecture in CortexBrain. CortexAvatar will request voice-optimized responses from CortexBrain, filter STT input to reduce noise, and provide seamless voice UX.

## Problem Statement

Current voice interactions suffer from:
1. **STT Noise**: Filler words ("um", "you know") sent as full requests
2. **No Voice Mode**: A2A requests don't indicate voice context
3. **Timeout Issues**: 30s timeout insufficient for cognitive pipeline
4. **No Streaming**: Responses arrive all-at-once instead of streaming

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          CORTEXAVATAR                                    │
│                                                                         │
│  ┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐   │
│  │   STT Filter    │     │   Voice Mode    │     │   Streaming     │   │
│  │   (new)         │     │   A2A Client    │     │   TTS Handler   │   │
│  │                 │     │   (modified)    │     │   (enhanced)    │   │
│  └────────┬────────┘     └────────┬────────┘     └────────┬────────┘   │
│           │                       │                       │            │
│           └───────────────────────┼───────────────────────┘            │
│                                   │                                     │
│                         ┌─────────▼─────────┐                          │
│                         │   Audio Bridge    │                          │
│                         │   (modified)      │                          │
│                         └─────────┬─────────┘                          │
│                                   │                                     │
└───────────────────────────────────┼─────────────────────────────────────┘
                                    │
                          A2A + mode: "voice"
                                    │
                                    ▼
                          ┌─────────────────────┐
                          │    CortexBrain      │
                          │   Voice Executive   │
                          └─────────────────────┘
```

## Components

### 1. STT Filter (`internal/stt/filter.go`)

Filters transcription output before sending to CortexBrain:

```go
type STTFilter struct {
    minWords        int           // Minimum words to accept (default: 2)
    minConfidence   float64       // Minimum confidence threshold (default: 0.7)
    fillerWords     []string      // Words to strip: "um", "uh", "like", "you know"
    fragmentTimeout time.Duration // Wait for more speech before sending fragments
}

func (f *STTFilter) ShouldSend(transcript string, confidence float64) bool
func (f *STTFilter) Clean(transcript string) string
```

**Filter rules**:
- Skip single-word utterances (except commands like "stop", "cancel")
- Remove filler words from beginning/end
- Require minimum confidence
- Accumulate fragments with timeout

### 2. Voice Mode A2A Client (`internal/a2a/client.go`)

Modified A2A client to request voice mode:

```go
type SendMessageOptions struct {
    Mode     string // "voice" for voice-optimized responses
    Persona  string // "hannah" or "henry"
    Timeout  time.Duration
    Stream   bool   // Request streaming response
}

func (c *Client) SendMessageWithOptions(text string, opts SendMessageOptions) (*Response, error)
```

**A2A Request format**:
```json
{
    "jsonrpc": "2.0",
    "method": "message/send",
    "params": {
        "mode": "voice",
        "persona": "hannah",
        "stream": true,
        "message": {
            "role": "user",
            "parts": [{"kind": "text", "text": "Hello Hannah"}]
        }
    }
}
```

### 3. Streaming TTS Handler (`internal/bridge/audio_bridge.go`)

Enhanced TTS handling for streaming responses:

```go
type StreamingTTS struct {
    ttsProvider TTSProvider
    chunkBuffer strings.Builder
    speakChan   chan string
}

func (s *StreamingTTS) HandleStreamingResponse(responseChan <-chan string) error
func (s *StreamingTTS) SpeakAtBreakpoint() // Speak when hitting sentence boundary
```

**Streaming flow**:
1. Receive response chunks from A2A
2. Buffer until natural breakpoint (sentence end)
3. Send to TTS while continuing to receive
4. Reduces perceived latency significantly

### 4. Conversation Manager (`internal/voice/conversation.go`)

Client-side conversation state for context:

```go
type ConversationManager struct {
    exchanges    []Exchange
    maxHistory   int
    lastActivity time.Time
}

func (c *ConversationManager) AddExchange(user, assistant string)
func (c *ConversationManager) GetContext() string
func (c *ConversationManager) IsFollowUp(text string) bool
```

## Configuration

```yaml
# ~/.cortexavatar/config.yaml (new section)
voice:
  mode_enabled: true
  default_persona: "hannah"
  stt_filter:
    min_words: 2
    min_confidence: 0.7
    filler_words: ["um", "uh", "like", "you know", "basically"]
    fragment_timeout_ms: 500
  streaming:
    enabled: true
    chunk_timeout_ms: 100
  timeouts:
    fast_path_ms: 1000
    warm_path_ms: 2000
    deep_path_ms: 5000
```

## File Structure

```
internal/
├── stt/
│   ├── filter.go          # STT output filter (new)
│   └── filter_test.go     # Filter tests (new)
├── a2a/
│   └── client.go          # Modified for voice mode
├── bridge/
│   └── audio_bridge.go    # Modified for streaming TTS
└── voice/
    ├── conversation.go    # Conversation state (new)
    └── config.go          # Voice config (new)
```

## Success Criteria

| Metric | Target |
|--------|--------|
| STT false positives | <5% (filler words not sent) |
| Voice mode adoption | 100% of voice requests use mode |
| Streaming latency | First audio <1s after response starts |
| End-to-end latency | <3s for simple queries |

## User Stories

| ID | Title | Priority |
|----|-------|----------|
| US-001 | STT Filter - Filler Word Removal | P0 |
| US-002 | STT Filter - Fragment Accumulation | P1 |
| US-003 | Voice Mode A2A - Mode Parameter | P0 |
| US-004 | Voice Mode A2A - Streaming Support | P1 |
| US-005 | Streaming TTS - Chunk Handler | P1 |
| US-006 | Streaming TTS - Breakpoint Detection | P2 |
| US-007 | Conversation Manager - Context Tracking | P2 |
| US-008 | Configuration - Voice Settings | P1 |

## Dependencies

- CortexBrain CR-093 (Voice Executive) must be implemented
- Existing STT provider (Groq Whisper)
- Existing TTS provider (ElevenLabs/Piper)
- Existing A2A client infrastructure

## Implementation Order

1. **STT Filter** - Stop sending garbage to brain
2. **Voice Mode A2A** - Request optimized responses
3. **Streaming TTS** - Reduce perceived latency
4. **Conversation Manager** - Context awareness
5. **Configuration** - Make it tunable

## Testing Strategy

### Unit Tests
- STT filter rules
- A2A voice mode serialization
- Streaming chunk handling
- Conversation state management

### Integration Tests
- Full voice flow: STT → Filter → A2A (voice mode) → Streaming TTS
- Timeout handling
- Error recovery

### Manual Tests
- Filler word filtering ("um, so, like, what's the weather")
- Streaming response feel
- Persona switching

## Related Documents

- CR-093 (CortexBrain): Voice Executive architecture
- `internal/bridge/audio_bridge.go`: Current audio handling
- `internal/a2a/client.go`: Current A2A client

---

**Author**: Sisyphus (AI Architect)
**Date**: January 16, 2026
**Status**: PROPOSED
