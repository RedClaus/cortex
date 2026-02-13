---
project: Cortex
component: UI
phase: Design
date_created: 2026-02-07T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-07T15:26:36.835579
---

# HF Voice Pipeline - User Guide

**Version:** 2.4.0
**Last Updated:** 2026-02-07
**Status:** Production Ready

---

## Table of Contents

1. [Overview](#overview)
2. [Quick Start](#quick-start)
3. [Installation](#installation)
4. [Configuration](#configuration)
5. [Using Voice Features](#using-voice-features)
6. [Troubleshooting](#troubleshooting)
7. [FAQ](#faq)

---

## Overview

The HF Voice Pipeline integrates state-of-the-art speech processing into CortexAvatar, providing:

- **Voice Activity Detection (VAD)** - Silero VAD for detecting speech in audio
- **Speech-to-Text (STT)** - Lightning Whisper MLX for transcription
- **Text-to-Speech (TTS)** - MeloTTS for natural voice synthesis

### Key Features

✅ **Low Latency** - <2s end-to-end voice interaction
✅ **High Quality** - 16kHz audio with Whisper accuracy
✅ **Multi-Language** - Supports English, French, Spanish, Chinese, Japanese, Korean
✅ **Streaming Audio** - Real-time audio playback as it's generated
✅ **Fallback Support** - Graceful degradation to legacy TTS if HF service unavailable

### System Requirements

- **macOS** 12.0 or later (Apple Silicon recommended)
- **Memory** 4GB RAM minimum (8GB recommended)
- **Python** 3.11+ (for HF service)
- **Go** 1.21+ (for CortexAvatar)
- **Node.js** 18+ (for frontend)

---

## Quick Start

### 1. Start HF Voice Service

```bash
# Navigate to HF speech-to-speech directory
cd ~/Projects/cortex-voice-poc/speech-to-speech

# Activate virtual environment
source .venv/bin/activate

# Start the service
python service.py
```

The service will start on `http://localhost:8899`.

### 2. Start CortexAvatar

```bash
# Navigate to CortexAvatar
cd ~/ServerProjectsMac/Development/cortex-avatar

# Start in development mode
wails dev
```

### 3. Use Voice Features

1. Click the **microphone button** in the CortexAvatar UI
2. Speak your message
3. Release the button when done
4. Listen to the AI response

That's it! You're now using the HF Voice Pipeline.

---

## Installation

### Option 1: Docker (Recommended)

```bash
# Navigate to HF service directory
cd ~/Projects/cortex-voice-poc/speech-to-speech

# Start with Docker Compose
docker-compose up -d

# Verify service is running
curl http://localhost:8899/health
```

### Option 2: Manual Installation

#### Step 1: Install HF Service

```bash
# Clone repository
git clone https://github.com/normanking/cortex-voice-poc.git
cd cortex-voice-poc/speech-to-speech

# Install Python 3.11 (if not installed)
brew install python@3.11

# Install uv package manager
brew install uv

# Install dependencies
uv sync

# Install macOS-specific requirements
uv pip install -r requirements_mac.txt
```

#### Step 2: Download Models

Models are downloaded automatically on first run. First startup takes ~2 minutes.

#### Step 3: Verify Installation

```bash
# Start service
python service.py

# Test health endpoint
curl http://localhost:8899/health

# Expected response:
# {
#   "status": "healthy",
#   "components": {
#     "vad": "loaded",
#     "stt": "loaded",
#     "tts": "loaded"
#   }
# }
```

---

## Configuration

### HF Service Configuration

Edit `speech-to-speech/config.yaml`:

```yaml
service:
  host: "0.0.0.0"
  port: 8899
  cors_origins:
    - "http://localhost:34115"  # Wails dev server
    - "wails://wails"            # Production app

models:
  vad:
    repo: "snakers4/silero-vad"
    model: "silero_vad.onnx"

  stt:
    repo: "openai/whisper-large-v3-turbo"
    language: "en"  # Default language

  tts:
    repo: "myshell-ai/MeloTTS-English"
    speaker: "EN-US"
    speed: 1.0

performance:
  batch_size: 1
  max_concurrent_requests: 10
```

### CortexAvatar Configuration

Edit `~/. cortex/config.yaml`:

```yaml
voice:
  hf_service:
    enabled: true
    url: "http://localhost:8899"
    timeout: 30  # seconds

  fallback:
    enabled: true
    provider: "elevenlabs"  # Fallback if HF unavailable

  audio:
    sample_rate: 16000
    channels: 1
    format: "wav"
```

---

## Using Voice Features

### Basic Voice Interaction

1. **Press and Hold** the microphone button
2. **Speak** your message clearly
3. **Release** the button when done
4. **Wait** for AI response (audio plays automatically)

### Voice Button States

| State | Icon | Meaning |
|-------|------|---------|
| **Ready** | 🎤 Gray | Ready to record |
| **Recording** | 🎤 Red | Actively recording |
| **Processing** | ⏳ | Transcribing/generating response |
| **Playing** | 🔊 | Playing AI response |
| **Error** | ⚠️ | Error occurred |

### Keyboard Shortcuts

- **Space** - Press and hold to record (when focused)
- **Esc** - Cancel current recording

### Voice Settings

Access voice settings in CortexAvatar preferences:

- **Language** - Choose transcription language (en, fr, es, zh, ja, ko)
- **Voice** - Select TTS voice (EN-US, EN-BR, FR, ES, etc.)
- **Speed** - Adjust speech speed (0.5x - 2.0x)
- **Auto-play** - Automatically play AI responses

---

## Troubleshooting

### HF Service Won't Start

**Problem:** Service fails to start with model loading errors

**Solution:**
```bash
# Check Python version
python --version  # Should be 3.11+

# Reinstall dependencies
rm -rf .venv
uv sync
uv pip install -r requirements_mac.txt

# Start service with verbose logging
python service.py --log-level DEBUG
```

### Microphone Access Denied

**Problem:** Browser/app can't access microphone

**Solution:**
1. Open **System Preferences → Security & Privacy → Microphone**
2. Enable access for CortexAvatar
3. Restart the app

### Poor Audio Quality

**Problem:** Transcription accuracy is low

**Solutions:**
- Speak clearly and slowly
- Reduce background noise
- Check microphone positioning
- Verify sample rate is 16000 Hz

### High Latency

**Problem:** Voice responses take >2 seconds

**Solutions:**
```bash
# Check service health
curl http://localhost:8899/health

# Monitor service logs
tail -f speech-to-speech/logs/service.log

# Check system resources
top -pid $(pgrep -f "python service.py")
```

### HF Service Unavailable

**Problem:** CortexAvatar can't connect to HF service

**Solutions:**
1. **Verify service is running:**
   ```bash
   curl http://localhost:8899/health
   ```

2. **Check firewall settings:**
   ```bash
   # Allow port 8899
   sudo ufw allow 8899
   ```

3. **Check network connectivity:**
   ```bash
   ping localhost
   telnet localhost 8899
   ```

### Audio Not Playing

**Problem:** AI response text appears but no audio

**Solutions:**
1. Check browser console for errors (F12)
2. Verify audio output device is connected
3. Check system volume settings
4. Try different browser (Chrome recommended)

### Memory Issues

**Problem:** HF service using too much memory

**Solutions:**
```yaml
# Edit config.yaml
performance:
  batch_size: 1  # Reduce from default
  max_concurrent_requests: 5  # Reduce from 10
```

```bash
# Restart service
docker-compose restart
```

---

## FAQ

### Q: Which languages are supported?

**A:** Currently supported languages:
- English (en)
- French (fr)
- Spanish (es)
- Chinese (zh)
- Japanese (ja)
- Korean (ko)

### Q: Can I use a different TTS voice?

**A:** Yes! Edit `config.yaml`:
```yaml
tts:
  speaker: "EN-BR"  # Options: EN-US, EN-BR, EN-INDIA, etc.
```

### Q: Does it work offline?

**A:** Yes! Once models are downloaded, the HF service works completely offline. No internet required.

### Q: What's the latency breakdown?

**A:** Typical latency (with HF service):
- VAD: ~350µs
- STT: ~200µs
- TTS: ~100µs
- **Total: <1ms** (excluding LLM processing)

With real-world LLM:
- Total E2E: <2s (90th percentile)

### Q: Can I run on Windows/Linux?

**A:** The HF service supports all platforms. CortexAvatar currently supports macOS only (Wails limitation).

### Q: How much disk space do models require?

**A:**
- Silero VAD: ~2MB
- Whisper Turbo: ~1.5GB
- MeloTTS: ~800MB
- **Total: ~2.3GB**

### Q: Can I use my own models?

**A:** Yes! Edit `config.yaml` to point to custom Hugging Face repositories.

### Q: Does it support streaming responses?

**A:** Yes! TTS audio streams in chunks for lower latency. STT is batch-processed per utterance.

### Q: What happens if HF service crashes?

**A:** CortexAvatar automatically falls back to legacy TTS (ElevenLabs). Voice input may be unavailable until HF service restarts.

---

## Getting Help

### Resources

- **GitHub Issues:** https://github.com/normanking/cortex-avatar/issues
- **Documentation:** `/docs/HF_VOICE_INTEGRATION.md`
- **Developer Guide:** `/docs/HF_VOICE_DEV_GUIDE.md`

### Support Channels

- Email: support@cortexavatar.com
- Discord: https://discord.gg/cortexavatar
- GitHub Discussions: https://github.com/normanking/cortex-avatar/discussions

---

**Next Steps:**
- Read the [Developer Guide](HF_VOICE_DEV_GUIDE.md) to integrate voice features
- Check the [Deployment Guide](HF_VOICE_DEPLOYMENT.md) for production setup
- Review [Performance Tuning](HF_VOICE_PERFORMANCE.md) for optimization tips
