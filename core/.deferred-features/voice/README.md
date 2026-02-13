---
project: Cortex
component: Docs
phase: Archive
date_created: 2025-12-18T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:51.531472
---

# Voice Package - Whisper.cpp Integration

This package provides server-side speech-to-text (STT) functionality using whisper.cpp.

## Overview

The voice package wraps whisper.cpp to provide REST API endpoints for audio transcription. It supports multiple audio formats (WAV, MP3, WebM, OGG, M4A) and provides configurable model sizes, multi-language support, and GPU acceleration.

## Architecture

```
internal/voice/
├── types.go          # Data types (TranscriptionRequest, TranscriptionResponse)
├── whisper.go        # WhisperService (core transcription logic)
├── handler.go        # HTTP handlers for REST API
├── whisper_test.go   # Unit tests
└── README.md         # This file
```

## Installation

### 1. Install whisper.cpp

```bash
# Clone whisper.cpp repository
git clone https://github.com/ggerganov/whisper.cpp.git
cd whisper.cpp

# Build the main executable
make

# Download a model (base model recommended for most use cases)
bash ./models/download-ggml-model.sh base

# Move models to default location
mkdir -p ~/.whisper
cp models/ggml-*.bin ~/.whisper/
```

### 2. Add whisper.cpp to PATH

```bash
# Option 1: Add whisper.cpp to PATH
export PATH=$PATH:/path/to/whisper.cpp

# Option 2: Symlink the executable
sudo ln -s /path/to/whisper.cpp/main /usr/local/bin/whisper
```

## Usage

### Basic Integration

```go
package main

import (
    "log"
    "net/http"
    "github.com/normanking/cortex/internal/voice"
)

func main() {
    // Configure Whisper service
    config := voice.WhisperConfig{
        ModelPath:      "",              // Uses ~/.whisper/ by default
        ExecutablePath: "whisper",       // Or full path to whisper.cpp/main
        DefaultModelSize: "base",        // "tiny", "base", "small", "medium", "large"
        MaxAudioSize:   25 * 1024 * 1024, // 25MB
        NumThreads:     4,
        EnableGPU:      false,
    }

    // Create Whisper service
    service, err := voice.NewWhisperService(config)
    if err != nil {
        log.Fatal(err)
    }

    // Create HTTP handler
    handler := voice.NewHandler(service)

    // Register routes
    mux := http.NewServeMux()
    handler.RegisterRoutes(mux)

    // Start server
    log.Println("Voice API listening on :8080")
    http.ListenAndServe(":8080", mux)
}
```

### API Endpoints

#### POST /api/v1/voice/transcribe

Transcribes an audio file to text.

**Request:**
```bash
curl -X POST http://localhost:8080/api/v1/voice/transcribe \
  -F "audio=@recording.wav" \
  -F "language=en" \
  -F "model_size=base"
```

**Parameters:**
- `audio` (required): Audio file (WAV, MP3, WebM, OGG, M4A)
- `language` (optional): Language code (e.g., "en", "es", "zh")
- `model_size` (optional): "tiny", "base", "small", "medium", "large" (default: "base")

**Response:**
```json
{
  "text": "Hello, this is a test transcription.",
  "confidence": 0.92,
  "language": "en",
  "duration": 3.5,
  "processing_time": "1.2s",
  "segments": [
    {
      "id": 0,
      "start": 0.0,
      "end": 1.5,
      "text": "Hello, this is a test",
      "confidence": 0.93
    },
    {
      "id": 1,
      "start": 1.5,
      "end": 3.5,
      "text": "transcription.",
      "confidence": 0.91
    }
  ]
}
```

#### GET /api/v1/voice/health

Health check endpoint.

**Response:**
```json
{
  "status": "ok",
  "service": "whisper"
}
```

## Model Sizes

| Model  | Size   | Speed  | Accuracy | Use Case |
|--------|--------|--------|----------|----------|
| tiny   | 75MB   | Fastest| Low      | Quick tests, low-resource devices |
| base   | 142MB  | Fast   | Good     | **Recommended for most use cases** |
| small  | 466MB  | Medium | Better   | Higher accuracy needed |
| medium | 1.5GB  | Slow   | High     | Professional transcription |
| large  | 2.9GB  | Slowest| Highest  | Maximum accuracy, research |

## Supported Languages

Whisper supports 99+ languages. Common language codes:

- `en` - English
- `es` - Spanish
- `fr` - French
- `de` - German
- `it` - Italian
- `pt` - Portuguese
- `zh` - Chinese
- `ja` - Japanese
- `ko` - Korean
- `ar` - Arabic
- `ru` - Russian
- `hi` - Hindi

## Configuration Options

```go
type WhisperConfig struct {
    // ModelPath is the directory containing whisper models
    // Default: ~/.whisper/
    ModelPath string

    // ExecutablePath is the path to whisper.cpp main executable
    // Default: searches for "whisper" in PATH
    ExecutablePath string

    // DefaultModelSize is the default model to use
    // Options: "tiny", "base", "small", "medium", "large"
    // Default: "base"
    DefaultModelSize string

    // MaxAudioSize is the maximum audio file size in bytes
    // Default: 25MB
    MaxAudioSize int64

    // TempDir is where temporary audio files are stored
    // Default: os.TempDir()
    TempDir string

    // EnableGPU enables GPU acceleration if available
    // Default: false
    EnableGPU bool

    // NumThreads specifies the number of CPU threads to use
    // Default: 4
    NumThreads int
}
```

## Performance Optimization

### GPU Acceleration

For CUDA-enabled GPUs:
```bash
# Build whisper.cpp with CUDA support
cd whisper.cpp
make clean
WHISPER_CUDA=1 make
```

Then enable GPU in config:
```go
config := voice.WhisperConfig{
    EnableGPU: true,
}
```

### Thread Configuration

Adjust threads based on CPU cores:
```go
config := voice.WhisperConfig{
    NumThreads: runtime.NumCPU(), // Use all CPU cores
}
```

## Error Handling

The service provides detailed error messages:

```go
resp, err := service.Transcribe(req)
if err != nil {
    // Check response error field
    if resp.Error != "" {
        log.Printf("Transcription failed: %s", resp.Error)
    }
}
```

Common errors:
- "empty audio data" - No audio provided
- "audio file too large" - Exceeds MaxAudioSize
- "whisper executable not found" - whisper.cpp not installed
- "model path does not exist" - Invalid model directory

## Integration with Prism Frontend

The frontend already has browser-native Web Speech API integration (see `prism/VOICE_INTEGRATION.md`). This server-side Whisper integration can be used for:

1. **Offline transcription** - When internet is unavailable
2. **Higher accuracy** - Whisper models are more accurate than browser APIs
3. **Multi-language support** - 99+ languages vs. browser's limited set
4. **Long audio files** - Browser APIs timeout after ~60 seconds
5. **Batch processing** - Transcribe multiple files server-side

### Example Frontend Integration

```typescript
// prism/src/api/voice.ts
export async function transcribeAudio(audioBlob: Blob, language = 'en'): Promise<string> {
  const formData = new FormData();
  formData.append('audio', audioBlob, 'recording.webm');
  formData.append('language', language);
  formData.append('model_size', 'base');

  const response = await fetch('/api/v1/voice/transcribe', {
    method: 'POST',
    body: formData,
  });

  const result = await response.json();
  return result.text;
}
```

## Testing

Run the test suite:
```bash
go test ./internal/voice/... -v
```

Test coverage:
- ✅ WhisperService creation and validation
- ✅ Audio format detection
- ✅ Temporary file creation
- ✅ Timestamp parsing
- ✅ Request validation (empty data, size limits)

## Troubleshooting

### "whisper executable not found"

Ensure whisper.cpp is installed and in PATH:
```bash
which whisper
# Or specify full path in config
ExecutablePath: "/path/to/whisper.cpp/main"
```

### "model path does not exist"

Download models to the correct location:
```bash
mkdir -p ~/.whisper
cd whisper.cpp
bash ./models/download-ggml-model.sh base
cp models/ggml-base.bin ~/.whisper/
```

### Slow transcription

1. Use smaller model (tiny or base)
2. Increase thread count
3. Enable GPU acceleration
4. Reduce audio quality before upload

### High memory usage

Use smaller model sizes or reduce thread count.

## License

This package wraps whisper.cpp (MIT License):
https://github.com/ggerganov/whisper.cpp

## References

- [whisper.cpp GitHub](https://github.com/ggerganov/whisper.cpp)
- [OpenAI Whisper](https://github.com/openai/whisper)
- [Whisper Paper](https://arxiv.org/abs/2212.04356)
