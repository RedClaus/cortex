---
project: Cortex
component: Docs
phase: Design
date_created: 2026-01-07T22:04:47
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:20:10.551133
---

# CortexAvatar

The Face, Eyes, and Ears of CortexBrain.

CortexAvatar is a companion desktop application that provides a visual, voice-interactive avatar interface for CortexBrain. It communicates exclusively through the A2A Protocol v0.3.0.

## Features

- **3D VRM Avatar** - Real-time lip-sync with 15-Oculus visemes, emotion detection
- **Streaming Voice Pipeline** - Sub-500ms latency STT → LLM → TTS
- **Voice Input** - Groq Whisper (default) or Deepgram streaming STT (~150ms)
- **Voice Output** - OpenAI TTS (default) or Cartesia Sonic streaming TTS (~75ms)
- **Vision Input** - Camera and screen capture for visual context
- **A2A Protocol** - Communicates with CortexBrain via JSON-RPC 2.0 + SSE streaming

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        CortexAvatar                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────┐    ┌─────────────────┐                    │
│  │   Svelte UI     │    │   Go Backend    │                    │
│  │   (WebView)     │◄──►│   (Wails)       │                    │
│  └─────────────────┘    └────────┬────────┘                    │
│                                  │                              │
│  ┌───────────┬───────────┬───────┴───────┬───────────┐         │
│  │   Audio   │   Vision  │    Avatar     │   A2A     │         │
│  │  Pipeline │  Pipeline │  Controller   │  Client   │         │
│  └───────────┴───────────┴───────────────┴─────┬─────┘         │
│                                                 │               │
└─────────────────────────────────────────────────┼───────────────┘
                                                  │
                                    A2A Protocol v0.3.0
                                    (JSON-RPC + SSE)
                                                  │
                                                  ▼
                                    ┌─────────────────────┐
                                    │    CortexBrain      │
                                    │    A2A Server       │
                                    └─────────────────────┘
```

## Requirements

- Go 1.22+
- Node.js 18+
- Wails CLI v2.9+
- CortexBrain running on `http://localhost:8080`

## Installation

```bash
# Install Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Install frontend dependencies
cd frontend && npm install && cd ..

# Build the application
wails build

# Or run in development mode
wails dev
```

## Environment Variables

API keys can be set as environment variables or in `~/.cortex/.env`:

```bash
# Required for voice input (free tier available)
GROQ_API_KEY=gsk_...

# Required for OpenAI TTS (default voice output)
OPENAI_API_KEY=sk-...

# Optional: Cartesia Sonic for ultra-low-latency TTS with lip-sync
CARTESIA_API_KEY=...

# Optional: Deepgram Nova-2 for streaming STT
DEEPGRAM_API_KEY=...
```

## Configuration

Configuration is stored in `~/.cortexavatar/config.yaml`:

```yaml
a2a:
  server_url: "http://localhost:8080"
  timeout: 30s

user:
  id: "default-user"
  persona_id: "hannah"

audio:
  vad_threshold: 0.5

stt:
  provider: "groq"              # whisper, groq, deepgram
  enable_streaming: false       # Enable WebSocket streaming (deepgram)
  interim_results: true         # Show partial transcriptions

tts:
  provider: "openai"            # openai, cartesia, piper, macos
  voice_id: "nova"
  enable_lip_sync: true         # Generate viseme timeline for 3D avatar
  cartesia_voice_id: "a0e99841-438c-4a64-b679-ae501e7d6091"

avatar:
  theme: "default"
  persona: "hannah"
```

### Low-Latency Mode

For the fastest voice conversations (~500ms end-to-end), use streaming providers:

```yaml
stt:
  provider: "deepgram"
  enable_streaming: true

tts:
  provider: "cartesia"
  enable_lip_sync: true
```

Requires `DEEPGRAM_API_KEY` and `CARTESIA_API_KEY` environment variables.

## Usage

1. Start CortexBrain's A2A server:
   ```bash
   cd /path/to/cortex-brain
   cortex-server --port 8080
   ```

2. Launch CortexAvatar:
   ```bash
   ./build/bin/CortexAvatar
   ```

3. The avatar will connect to CortexBrain and be ready for interaction.

## Controls

- **Microphone** - Toggle voice input
- **Speaker** - Toggle voice output
- **Camera** - Enable camera for visual context
- **Screen** - Share screen for visual context

## Development

```bash
# Run in development mode with hot reload
wails dev

# Build for production
wails build

# Generate Wails bindings
wails generate module
```

## Project Structure

```
CortexAvatar/
├── main.go                 # Application entry point
├── internal/
│   ├── a2a/                # A2A Protocol client (JSON-RPC + SSE)
│   ├── audio/              # Audio capture/playback
│   ├── avatar/             # Avatar state machine
│   ├── bridge/             # Wails Go-JS bridges
│   │   ├── audio_bridge.go # Audio/TTS/STT coordination
│   │   └── streaming_orchestrator.go  # STT→LLM→TTS pipeline
│   ├── config/             # Configuration management
│   ├── stt/                # Speech-to-text providers
│   │   ├── groq_whisper.go # Groq Whisper (default)
│   │   └── deepgram_streaming.go  # Deepgram streaming
│   ├── tts/                # Text-to-speech providers
│   │   ├── openai_tts.go   # OpenAI TTS (default)
│   │   ├── cartesia_tts.go # Cartesia Sonic streaming
│   │   └── viseme_timeline.go  # Lip-sync viseme generation
│   └── vision/             # Camera/screen capture
├── frontend/               # Svelte frontend
│   ├── src/
│   │   ├── lib/
│   │   │   ├── Avatar3D.svelte  # 3D VRM avatar with lip-sync
│   │   │   └── avatar/     # Avatar utilities (visemes, emotions)
│   │   └── stores/         # State stores
│   └── assets/             # VRM models, textures
└── scripts/                # Diagnostic scripts
```

## License

MIT License - see [LICENSE](LICENSE) for details.
