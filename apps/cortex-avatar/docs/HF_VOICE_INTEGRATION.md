---
project: Cortex
component: Docs
phase: Design
date_created: 2026-02-06T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T19:14:00.662429
---

# HF Voice Service Integration Guide

**Status:** ✅ Phase 3 Complete - Go clients implemented
**Date:** 2026-02-06
**Version:** CortexAvatar v2.4.0

---

## Overview

CortexAvatar now integrates with the HF Voice Service microservice for Mac-optimized speech processing:

- **VAD (Voice Activity Detection):** Silero VAD v5
- **STT (Speech-to-Text):** Lightning Whisper MLX
- **TTS (Text-to-Speech):** MeloTTS (multilingual)

## Architecture

```
┌─────────────────────┐
│   CortexAvatar      │
│   (Wails App)       │
├─────────────────────┤
│ • HFVADClient       │──┐
│ • HFWhisperProvider │──┼─── HTTP (localhost:8899)
│ • HFMeloProvider    │──┘
└─────────────────────┘
         │
         ▼
┌─────────────────────┐
│  HF Voice Service   │
│  (FastAPI/Docker)   │
├─────────────────────┤
│ • /vad endpoint     │
│ • /stt endpoint     │
│ • /tts endpoint     │
│ • /health endpoint  │
└─────────────────────┘
```

## Go Client Implementation

### 1. VAD Client

**Location:** `internal/audio/hf_vad.go`

**Usage:**
```go
import "github.com/normanking/cortexavatar/internal/audio"

// Create client
vadClient := audio.NewHFVADClient("http://localhost:8899", logger)

// Check health
if err := vadClient.Health(ctx); err != nil {
    log.Error("HF service unavailable:", err)
}

// Detect speech
result, err := vadClient.DetectSpeech(ctx, audioData)
if err != nil {
    log.Error("VAD failed:", err)
}

if result.IsSpeech {
    log.Info("Speech detected (confidence: %.2f)", result.Confidence)
}
```

### 2. STT Provider

**Location:** `internal/stt/hf_whisper.go`

**Usage:**
```go
import "github.com/normanking/cortexavatar/internal/stt"

// Create provider
config := stt.DefaultHFWhisperConfig()
config.ServiceURL = "http://localhost:8899"
config.Language = "en"

provider := stt.NewHFWhisperProvider(config, logger)

// Transcribe audio
req := &stt.TranscribeRequest{
    Audio:      audioData,
    Format:     "wav",
    SampleRate: 16000,
    Channels:   1,
    Language:   "en",
}

resp, err := provider.Transcribe(ctx, req)
if err != nil {
    log.Error("Transcription failed:", err)
}

log.Info("Transcribed:", resp.Text)
log.Info("Confidence:", resp.Confidence)
log.Info("Processing time:", resp.ProcessingTime)
```

**Capabilities:**
- Supports 7 languages: en, fr, es, zh, ja, ko, auto
- Max audio: 30 seconds
- Avg latency: 500ms
- Local processing (no API costs)
- MPS acceleration (Apple Silicon)

### 3. TTS Provider

**Location:** `internal/tts/hf_melo.go`

**Usage:**
```go
import "github.com/normanking/cortexavatar/internal/tts"

// Create provider
config := tts.DefaultHFMeloConfig()
config.ServiceURL = "http://localhost:8899"
config.DefaultVoice = "EN"
config.DefaultSpeed = 1.0

provider := tts.NewHFMeloProvider(config, logger)

// Synthesize text
req := &tts.SynthesizeRequest{
    Text:    "Hello, how can I help you?",
    VoiceID: "en",  // or "EN", "FR", "ES", "ZH", "JA", "KO"
    Speed:   1.0,
}

resp, err := provider.Synthesize(ctx, req)
if err != nil {
    log.Error("Synthesis failed:", err)
}

// resp.Audio contains WAV bytes (16kHz mono)
audioBytes := resp.Audio
```

**Streaming TTS:**
```go
// Stream synthesis in chunks (8KB chunks)
audioChan, err := provider.SynthesizeStream(ctx, req)
if err != nil {
    log.Error("Streaming failed:", err)
}

for chunk := range audioChan {
    // chunk.Data contains audio bytes
    // chunk.IsFinal indicates last chunk
    playAudio(chunk.Data)

    if chunk.IsFinal {
        break
    }
}
```

**Capabilities:**
- Supports 6 languages: EN, FR, ES, ZH, JA, KO
- Max text: 500 characters
- Avg latency: 700ms
- Streaming supported (chunked)
- Local processing
- MPS acceleration

## Fallback Logic

All providers implement health checks for fallback handling:

```go
// Check if HF service is available
if err := provider.Health(ctx); err != nil {
    log.Warn("HF service unavailable, falling back to alternative")

    // Fall back to legacy provider
    provider = getLegacyProvider()
}

// Proceed with transcription/synthesis
resp, err := provider.Transcribe(ctx, req)
```

## Configuration

### Environment Variables

```bash
# HF Service URL (default: http://localhost:8899)
export HF_VOICE_SERVICE_URL="http://localhost:8899"

# Timeout in seconds (default: 30)
export HF_VOICE_TIMEOUT=30

# Default language (default: en)
export HF_STT_LANGUAGE=en

# Default TTS voice (default: EN)
export HF_TTS_VOICE=EN

# Default speech speed (default: 1.0)
export HF_TTS_SPEED=1.0
```

### CortexAvatar Config

Add to your `cortex-avatar` configuration:

```yaml
audio:
  vad:
    provider: "hf"          # Use HF VAD
    service_url: "http://localhost:8899"

  stt:
    provider: "hf_whisper"  # Use HF Whisper
    service_url: "http://localhost:8899"
    language: "en"
    timeout: 30

  tts:
    provider: "hf_melo"     # Use HF Melo
    service_url: "http://localhost:8899"
    default_voice: "EN"
    speed: 1.0
    timeout: 30

fallback:
  stt_provider: "groq_whisper"  # Fallback if HF unavailable
  tts_provider: "openai_tts"    # Fallback if HF unavailable
```

## Running the HF Service

### Docker Compose (Recommended)

```bash
cd ~/Projects/cortex-voice-poc/hf-voice-service

# Start service
docker-compose up -d

# Check logs
docker-compose logs -f

# Stop service
docker-compose down
```

### Local Development

```bash
cd ~/Projects/cortex-voice-poc/hf-voice-service

# Install dependencies
pip install uv
uv pip install -r requirements.txt

# Run service
uvicorn app.main:app --host 0.0.0.0 --port 8899 --reload
```

### Health Check

```bash
# Check service status
curl http://localhost:8899/health

# Expected response:
{
  "status": "healthy",
  "components": {
    "vad": "loaded",
    "stt": "loaded",
    "tts": "loaded"
  },
  "uptime_seconds": 123.45
}
```

## Error Handling

All providers return standard Go errors:

```go
resp, err := provider.Transcribe(ctx, req)
if err != nil {
    switch {
    case errors.Is(err, stt.ErrProviderUnavailable):
        // Service unreachable - use fallback
        log.Warn("STT service unavailable, using fallback")

    case errors.Is(err, stt.ErrTimeout):
        // Request timed out
        log.Error("STT timeout, retrying...")

    case errors.Is(err, stt.ErrAudioTooShort):
        // Audio too short for transcription
        log.Debug("Audio too short, skipping transcription")

    default:
        // Unknown error
        log.Error("STT error:", err)
    }
}
```

## Performance Benchmarks

From POC testing (Phase 1):

| Component | Load Time | Memory | Latency (5s audio) |
|-----------|-----------|--------|-------------------|
| VAD | ~1s | ~50 MB | ~50ms |
| STT | ~2s (cached) | 219 MB | ~500ms |
| TTS | ~1s | ~500 MB | ~700ms |

**Total Memory:** ~770 MB (without LLM)
**E2E Latency:** ~1.25s (VAD + STT + TTS, excluding LLM)

## Integration Checklist

- [x] Phase 0: POC environment setup
- [x] Phase 1: Performance validation (<2s latency, <4GB memory)
- [x] Phase 2: FastAPI service with Docker
- [x] Phase 3: Go clients (VAD, STT, TTS)
- [ ] Phase 4: Svelte audio components
- [ ] Phase 5: E2E testing and TTS duplication fix
- [ ] Phase 6: Documentation and release

## Troubleshooting

### Service Won't Start

```bash
# Check port availability
lsof -i :8899

# Check Docker logs
docker-compose logs hf-voice-service

# Ensure models downloaded
ls ~/Projects/cortex-voice-poc/hf-voice-service/models_cache
```

### High Latency

- First request per model is slow (model loading)
- Subsequent requests much faster (models cached in memory)
- Ensure HF service running locally (not network latency)

### Out of Memory

- Models consume ~1.1GB total
- Ensure Docker has ≥4GB memory allocation
- Consider using smaller Whisper model (tiny instead of distil-large-v3)

## Next Steps

1. **Phase 4:** Build Svelte audio components
   - AudioCapture component
   - StreamingAudioPlayer component
   - VoiceButton component

2. **Phase 5:** Testing and bug fixes
   - E2E voice interaction tests
   - Fix TTS duplication bug
   - Performance validation

3. **Phase 6:** Documentation and release
   - User guide
   - Developer docs
   - Demo video
   - v2.4.0 release notes

---

**Related Files:**
- `/Users/normanking/ServerProjectsMac/Development/cortex-avatar/internal/audio/hf_vad.go`
- `/Users/normanking/ServerProjectsMac/Development/cortex-avatar/internal/stt/hf_whisper.go`
- `/Users/normanking/ServerProjectsMac/Development/cortex-avatar/internal/tts/hf_melo.go`
- `/Users/normanking/Projects/cortex-voice-poc/hf-voice-service/`

**Last Updated:** 2026-02-06
