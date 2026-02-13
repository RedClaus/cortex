---
project: Cortex
component: UI
phase: Design
date_created: 2026-02-07T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-07T15:26:36.558850
---

# HF Voice Pipeline - Developer Guide

**Version:** 2.4.0
**Last Updated:** 2026-02-07
**Audience:** Developers integrating voice features

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [API Reference](#api-reference)
3. [Integration Patterns](#integration-patterns)
4. [Code Examples](#code-examples)
5. [Testing Guide](#testing-guide)
6. [Best Practices](#best-practices)

---

## Architecture Overview

### System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     CortexAvatar (Wails)                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │   Frontend   │  │   Go Bridge  │  │  LLM Engine  │     │
│  │   (Svelte)   │◄─┤  (internal/) ├─►│              │     │
│  └──────┬───────┘  └──────┬───────┘  └──────────────┘     │
│         │                  │                                │
│    ┌────▼────────────────▼─────┐                          │
│    │  Audio/STT/TTS Providers  │                          │
│    └────┬──────────────────────┘                          │
└─────────┼───────────────────────────────────────────────────┘
          │ HTTP
          ▼
┌─────────────────────────────────────────────────────────────┐
│              HF Voice Service (FastAPI)                      │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                 │
│  │   VAD    │  │   STT    │  │   TTS    │                 │
│  │ (Silero) │  │(Whisper) │  │  (Melo)  │                 │
│  └──────────┘  └──────────┘  └──────────┘                 │
└─────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Location |
|-----------|---------------|----------|
| **VoiceButton.svelte** | UI control for voice input | `frontend/src/lib/` |
| **AudioCapture.svelte** | Microphone access & recording | `frontend/src/lib/` |
| **StreamingAudioPlayer.svelte** | Audio playback & visualization | `frontend/src/lib/` |
| **HFVADClient** | Voice activity detection | `internal/audio/` |
| **HFWhisperProvider** | Speech-to-text | `internal/stt/` |
| **HFMeloProvider** | Text-to-speech | `internal/tts/` |
| **AudioBridge** | Frontend ↔ Backend bridge | `internal/bridge/` |

---

## API Reference

### HF Service REST API

#### Base URL
```
http://localhost:8899
```

#### Health Check

```http
GET /health
```

**Response:**
```json
{
  "status": "healthy",
  "components": {
    "vad": "loaded",
    "stt": "loaded",
    "tts": "loaded"
  }
}
```

#### Voice Activity Detection

```http
POST /vad
Content-Type: multipart/form-data

audio: <binary WAV data>
```

**Response:**
```json
{
  "has_speech": true,
  "confidence": 0.95
}
```

#### Speech-to-Text

```http
POST /stt?language=en
Content-Type: multipart/form-data

audio: <binary WAV data>
```

**Response:**
```json
{
  "text": "Hello, how can I help you?",
  "language": "en",
  "confidence": 0.98,
  "processing_time_ms": 450
}
```

**Supported Languages:**
- `en` - English
- `fr` - French
- `es` - Spanish
- `zh` - Chinese
- `ja` - Japanese
- `ko` - Korean

#### Text-to-Speech

```http
POST /tts
Content-Type: application/json

{
  "text": "Hello, how can I help you?",
  "language": "EN",
  "speed": 1.0
}
```

**Response:**
- Content-Type: `audio/wav`
- Binary WAV audio data (16kHz, mono, 16-bit PCM)

**Supported Voices:**
- `EN` - English (US)
- `EN-BR` - English (British)
- `FR` - French
- `ES` - Spanish
- `ZH` - Chinese
- `JA` - Japanese
- `KO` - Korean

---

### Go Client API

#### VAD Client

```go
import "github.com/normanking/cortexavatar/internal/audio"

// Create client
client := audio.NewHFVADClient("http://localhost:8899", logger)

// Detect speech
result, err := client.DetectSpeech(ctx, audioData)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Has speech: %v (confidence: %.2f)\n",
    result.IsSpeech, result.Confidence)
```

**Types:**
```go
type VADResult struct {
    IsSpeech   bool
    Confidence float64
}
```

#### STT Provider

```go
import "github.com/normanking/cortexavatar/internal/stt"

// Create provider
provider := stt.NewHFWhisperProvider(&stt.HFWhisperConfig{
    ServiceURL: "http://localhost:8899",
    Timeout:    30,
    Language:   "en",
}, logger)

// Transcribe audio
request := &stt.TranscribeRequest{
    Audio:      audioData,
    Format:     "wav",
    SampleRate: 16000,
    Channels:   1,
    Language:   "en",
}

response, err := provider.Transcribe(ctx, request)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Transcription: %s\n", response.Text)
```

**Types:**
```go
type TranscribeRequest struct {
    Audio      []byte
    Format     string  // "wav", "mp3", etc.
    SampleRate int     // 16000 recommended
    Channels   int     // 1 (mono) recommended
    Language   string  // "en", "fr", etc.
}

type TranscribeResponse struct {
    Text           string
    Language       string
    Confidence     float64
    IsFinal        bool
    ProcessingTime float64  // milliseconds
}
```

#### TTS Provider

```go
import "github.com/normanking/cortexavatar/internal/tts"

// Create provider
provider := tts.NewHFMeloProvider(&tts.HFMeloConfig{
    ServiceURL:   "http://localhost:8899",
    Timeout:      30,
    DefaultVoice: "EN",
    DefaultSpeed: 1.0,
}, logger)

// Synthesize speech
request := &tts.SynthesizeRequest{
    Text:    "Hello, how can I help you?",
    VoiceID: "EN",
    Speed:   1.0,
}

response, err := provider.Synthesize(ctx, request)
if err != nil {
    log.Fatal(err)
}

// Play audio
playAudio(response.Audio)
```

**Streaming TTS:**
```go
// Synthesize with streaming
audioChan, err := provider.SynthesizeStream(ctx, request)
if err != nil {
    log.Fatal(err)
}

// Process audio chunks as they arrive
for chunk := range audioChan {
    if chunk.Error != nil {
        log.Printf("Error: %v", chunk.Error)
        continue
    }

    playAudioChunk(chunk.Data)

    if chunk.IsFinal {
        break
    }
}
```

**Types:**
```go
type SynthesizeRequest struct {
    Text    string
    VoiceID string  // "EN", "FR", etc.
    Speed   float64 // 0.5 - 2.0
}

type SynthesizeResponse struct {
    Audio        []byte
    Format       string  // "wav"
    SampleRate   int     // 16000
    Channels     int     // 1
    Duration     float64 // seconds
}

type AudioChunk struct {
    Data    []byte
    Index   int
    IsFinal bool
    Error   error
}
```

---

## Integration Patterns

### Pattern 1: Simple Voice Command

```go
func handleVoiceCommand(audioData []byte) error {
    ctx := context.Background()

    // 1. Detect speech
    vadResult, err := vadClient.DetectSpeech(ctx, audioData)
    if err != nil {
        return fmt.Errorf("VAD failed: %w", err)
    }

    if !vadResult.IsSpeech {
        return errors.New("no speech detected")
    }

    // 2. Transcribe
    transcribeReq := &stt.TranscribeRequest{
        Audio:      audioData,
        Format:     "wav",
        SampleRate: 16000,
        Channels:   1,
        Language:   "en",
    }

    transcribeResp, err := sttProvider.Transcribe(ctx, transcribeReq)
    if err != nil {
        return fmt.Errorf("transcription failed: %w", err)
    }

    // 3. Process command
    response := processCommand(transcribeResp.Text)

    // 4. Synthesize response
    synthesizeReq := &tts.SynthesizeRequest{
        Text:    response,
        VoiceID: "EN",
        Speed:   1.0,
    }

    synthesizeResp, err := ttsProvider.Synthesize(ctx, synthesizeReq)
    if err != nil {
        return fmt.Errorf("synthesis failed: %w", err)
    }

    // 5. Play audio
    return playAudio(synthesizeResp.Audio)
}
```

### Pattern 2: Streaming Voice Interaction

```go
func handleStreamingVoiceInteraction(audioStream chan []byte) error {
    ctx := context.Background()

    // Collect audio chunks
    var audioBuffer []byte
    for chunk := range audioStream {
        audioBuffer = append(audioBuffer, chunk...)
    }

    // Transcribe
    transcribeResp, err := sttProvider.Transcribe(ctx, &stt.TranscribeRequest{
        Audio:      audioBuffer,
        Format:     "wav",
        SampleRate: 16000,
        Channels:   1,
        Language:   "en",
    })
    if err != nil {
        return err
    }

    // Get LLM response
    llmResponse := getLLMResponse(transcribeResp.Text)

    // Stream TTS audio
    audioChan, err := ttsProvider.SynthesizeStream(ctx, &tts.SynthesizeRequest{
        Text:    llmResponse,
        VoiceID: "EN",
        Speed:   1.0,
    })
    if err != nil {
        return err
    }

    // Play audio chunks as they arrive
    for chunk := range audioChan {
        if chunk.Error != nil {
            log.Printf("TTS error: %v", chunk.Error)
            continue
        }
        playAudioChunk(chunk.Data)
    }

    return nil
}
```

### Pattern 3: Fallback Strategy

```go
type VoiceProcessor struct {
    hfTTS      *tts.HFMeloProvider
    fallbackTTS *tts.ElevenLabsProvider
}

func (vp *VoiceProcessor) Synthesize(ctx context.Context, text string) ([]byte, error) {
    // Try HF TTS first
    req := &tts.SynthesizeRequest{
        Text:    text,
        VoiceID: "EN",
        Speed:   1.0,
    }

    resp, err := vp.hfTTS.Synthesize(ctx, req)
    if err == nil {
        return resp.Audio, nil
    }

    log.Printf("HF TTS failed (%v), falling back to ElevenLabs", err)

    // Fallback to ElevenLabs
    return vp.fallbackTTS.Synthesize(ctx, text)
}
```

### Pattern 4: Retry with Exponential Backoff

```go
func synthesizeWithRetry(ctx context.Context, provider *tts.HFMeloProvider, req *tts.SynthesizeRequest, maxRetries int) (*tts.SynthesizeResponse, error) {
    var lastErr error

    for attempt := 0; attempt < maxRetries; attempt++ {
        resp, err := provider.Synthesize(ctx, req)
        if err == nil {
            return resp, nil
        }

        lastErr = err

        // Exponential backoff
        backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
        log.Printf("Attempt %d failed: %v. Retrying in %v", attempt+1, err, backoff)

        select {
        case <-time.After(backoff):
            continue
        case <-ctx.Done():
            return nil, ctx.Err()
        }
    }

    return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}
```

---

## Code Examples

### Frontend: Voice Button Integration

```svelte
<script lang="ts">
  import VoiceButton from '$lib/VoiceButton.svelte';
  import AudioCapture from '$lib/AudioCapture.svelte';
  import StreamingAudioPlayer from '$lib/StreamingAudioPlayer.svelte';

  let isRecording = false;
  let transcription = '';
  let audioCapture: AudioCapture;
  let audioPlayer: StreamingAudioPlayer;

  async function handleVoiceInput() {
    isRecording = true;

    try {
      // Capture audio
      const audioBlob = await audioCapture.startRecording();

      // Send to backend
      const response = await fetch('/api/voice/process', {
        method: 'POST',
        body: audioBlob
      });

      const data = await response.json();
      transcription = data.transcription;

      // Play response audio
      if (data.audioUrl) {
        await audioPlayer.playAudioUrl(data.audioUrl);
      }
    } catch (error) {
      console.error('Voice input failed:', error);
    } finally {
      isRecording = false;
    }
  }
</script>

<div class="voice-interface">
  <AudioCapture bind:this={audioCapture} />
  <StreamingAudioPlayer bind:this={audioPlayer} />

  <VoiceButton
    on:press={handleVoiceInput}
    disabled={isRecording}
  />

  {#if transcription}
    <p class="transcription">{transcription}</p>
  {/if}
</div>
```

### Backend: Voice Processing Handler

```go
func (b *AudioBridge) ProcessVoiceInput(audioData []byte) (*VoiceResponse, error) {
    ctx := context.Background()
    startTime := time.Now()

    // Step 1: VAD
    vadResult, err := b.vadClient.DetectSpeech(ctx, audioData)
    if err != nil {
        return nil, fmt.Errorf("VAD failed: %w", err)
    }

    if !vadResult.IsSpeech {
        return nil, errors.New("no speech detected")
    }

    // Step 2: STT
    transcribeReq := &stt.TranscribeRequest{
        Audio:      audioData,
        Format:     "wav",
        SampleRate: 16000,
        Channels:   1,
        Language:   "en",
    }

    transcribeResp, err := b.sttProvider.Transcribe(ctx, transcribeReq)
    if err != nil {
        return nil, fmt.Errorf("transcription failed: %w", err)
    }

    // Step 3: LLM Processing
    llmResponse, err := b.llmEngine.Process(transcribeResp.Text)
    if err != nil {
        return nil, fmt.Errorf("LLM processing failed: %w", err)
    }

    // Step 4: TTS
    synthesizeReq := &tts.SynthesizeRequest{
        Text:    llmResponse,
        VoiceID: "EN",
        Speed:   1.0,
    }

    synthesizeResp, err := b.ttsProvider.Synthesize(ctx, synthesizeReq)
    if err != nil {
        return nil, fmt.Errorf("synthesis failed: %w", err)
    }

    latency := time.Since(startTime)

    return &VoiceResponse{
        Transcription: transcribeResp.Text,
        Response:      llmResponse,
        Audio:         synthesizeResp.Audio,
        Latency:       latency,
    }, nil
}
```

---

## Testing Guide

### Unit Testing

```go
func TestHFVADClient_DetectSpeech(t *testing.T) {
    // Create mock server
    mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "has_speech": true,
            "confidence": 0.95,
        })
    }))
    defer mockServer.Close()

    // Create client
    client := audio.NewHFVADClient(mockServer.URL, zerolog.Nop())

    // Test detection
    audioData := generateTestAudio(t, 3*time.Second)
    result, err := client.DetectSpeech(context.Background(), audioData)

    require.NoError(t, err)
    assert.True(t, result.IsSpeech)
    assert.Equal(t, 0.95, result.Confidence)
}
```

### Integration Testing

```go
func TestVoicePipeline_E2E(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping E2E test")
    }

    // Initialize real clients
    vadClient := audio.NewHFVADClient("http://localhost:8899", logger)
    sttProvider := stt.NewHFWhisperProvider(&stt.HFWhisperConfig{
        ServiceURL: "http://localhost:8899",
        Timeout:    30,
        Language:   "en",
    }, logger)
    ttsProvider := tts.NewHFMeloProvider(&tts.HFMeloConfig{
        ServiceURL:   "http://localhost:8899",
        Timeout:      30,
        DefaultVoice: "EN",
        DefaultSpeed: 1.0,
    }, logger)

    // Generate test audio
    audioData := generateTestAudio(t, 5*time.Second)

    // Run full pipeline
    ctx := context.Background()

    // VAD
    vadResult, err := vadClient.DetectSpeech(ctx, audioData)
    require.NoError(t, err)
    assert.True(t, vadResult.IsSpeech)

    // STT
    transcribeResp, err := sttProvider.Transcribe(ctx, &stt.TranscribeRequest{
        Audio:      audioData,
        Format:     "wav",
        SampleRate: 16000,
        Channels:   1,
        Language:   "en",
    })
    require.NoError(t, err)
    assert.NotEmpty(t, transcribeResp.Text)

    // TTS
    synthesizeResp, err := ttsProvider.Synthesize(ctx, &tts.SynthesizeRequest{
        Text:    "Test response",
        VoiceID: "EN",
        Speed:   1.0,
    })
    require.NoError(t, err)
    assert.NotEmpty(t, synthesizeResp.Audio)
}
```

### Performance Testing

```bash
# Run performance benchmarks
go test -v ./tests/performance/... -run TestVoicePipelinePerformance

# Expected output:
# ✅ Success Rate: 100.00% (100/100)
# ✅ E2E P95 Latency: 526.75µs (target: <2s)
# ✅ Memory Growth: 46.88% (target: <50%)
```

---

## Best Practices

### 1. Error Handling

```go
// ✅ Good: Structured error handling with context
func processVoice(audioData []byte) error {
    vadResult, err := vadClient.DetectSpeech(ctx, audioData)
    if err != nil {
        if errors.Is(err, context.DeadlineExceeded) {
            return fmt.Errorf("VAD timeout: %w", err)
        }
        return fmt.Errorf("VAD failed: %w", err)
    }

    // ... rest of processing
}

// ❌ Bad: Generic error messages
func processVoice(audioData []byte) error {
    _, err := vadClient.DetectSpeech(ctx, audioData)
    if err != nil {
        return err  // No context!
    }
}
```

### 2. Context Management

```go
// ✅ Good: Timeout contexts
func synthesize(text string) ([]byte, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    return ttsProvider.Synthesize(ctx, &tts.SynthesizeRequest{
        Text:    text,
        VoiceID: "EN",
        Speed:   1.0,
    })
}

// ❌ Bad: No timeout
func synthesize(text string) ([]byte, error) {
    return ttsProvider.Synthesize(context.Background(), req)  // Could hang forever!
}
```

### 3. Resource Cleanup

```go
// ✅ Good: Proper cleanup
func streamAudio(text string) error {
    ctx := context.Background()

    audioChan, err := ttsProvider.SynthesizeStream(ctx, req)
    if err != nil {
        return err
    }

    defer func() {
        // Drain channel to prevent goroutine leak
        for range audioChan {
        }
    }()

    for chunk := range audioChan {
        if chunk.Error != nil {
            return chunk.Error
        }
        playChunk(chunk.Data)
    }

    return nil
}
```

### 4. Logging

```go
// ✅ Good: Structured logging with context
log.Info().
    Str("operation", "transcribe").
    Int("audio_size", len(audioData)).
    Float64("latency_ms", latency.Milliseconds()).
    Str("language", "en").
    Msg("Transcription completed")

// ❌ Bad: Unstructured logging
log.Printf("Transcription done")
```

### 5. Configuration

```go
// ✅ Good: Configuration struct
type VoiceConfig struct {
    HFServiceURL  string
    Timeout       time.Duration
    Language      string
    RetryAttempts int
}

// Load from config file
config := loadConfig("config.yaml")
provider := stt.NewHFWhisperProvider(&stt.HFWhisperConfig{
    ServiceURL: config.HFServiceURL,
    Timeout:    config.Timeout,
    Language:   config.Language,
}, logger)

// ❌ Bad: Hardcoded values
provider := stt.NewHFWhisperProvider(&stt.HFWhisperConfig{
    ServiceURL: "http://localhost:8899",  // Hardcoded!
    Timeout:    30,
    Language:   "en",
}, logger)
```

---

## Performance Optimization

### 1. Connection Pooling

```go
// Create HTTP client with connection pooling
httpClient := &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
    },
}

// Use in providers
provider := stt.NewHFWhisperProviderWithClient(config, logger, httpClient)
```

### 2. Audio Compression

```go
// Compress audio before sending
func compressAudio(audioData []byte) ([]byte, error) {
    // Use Opus codec for better compression
    // 16kHz mono PCM → Opus reduces size by ~10x
    return opus.Encode(audioData, 16000, 1)
}
```

### 3. Batch Processing

```go
// Process multiple requests in batch
func processVoiceBatch(audioChunks [][]byte) ([]*VoiceResponse, error) {
    results := make([]*VoiceResponse, len(audioChunks))

    // Process in parallel
    var wg sync.WaitGroup
    for i, audio := range audioChunks {
        wg.Add(1)
        go func(idx int, data []byte) {
            defer wg.Done()
            results[idx], _ = processVoice(data)
        }(i, audio)
    }

    wg.Wait()
    return results, nil
}
```

---

## Security Considerations

### 1. Input Validation

```go
func validateAudioInput(audioData []byte) error {
    // Check size limits
    maxSize := 10 * 1024 * 1024 // 10MB
    if len(audioData) > maxSize {
        return fmt.Errorf("audio too large: %d bytes (max: %d)", len(audioData), maxSize)
    }

    // Check format
    if !isValidWAV(audioData) {
        return errors.New("invalid audio format (must be WAV)")
    }

    return nil
}
```

### 2. Rate Limiting

```go
import "golang.org/x/time/rate"

// Create rate limiter (10 requests per second)
limiter := rate.NewLimiter(10, 1)

func handleVoiceRequest(audioData []byte) error {
    if !limiter.Allow() {
        return errors.New("rate limit exceeded")
    }

    return processVoice(audioData)
}
```

### 3. Authentication

```go
// Add API key to requests
func (p *HFWhisperProvider) transcribe(ctx context.Context, audioData []byte) error {
    req, err := http.NewRequestWithContext(ctx, "POST", p.serviceURL+"/stt", body)
    if err != nil {
        return err
    }

    // Add authentication
    req.Header.Set("Authorization", "Bearer "+p.apiKey)

    // ... send request
}
```

---

## Migration Guide

### From Legacy TTS to HF Pipeline

**Before:**
```go
// Old ElevenLabs TTS
ttsProvider := tts.NewElevenLabsProvider(apiKey, logger)
audio, err := ttsProvider.Synthesize("Hello world")
```

**After:**
```go
// New HF TTS with fallback
hfProvider := tts.NewHFMeloProvider(&tts.HFMeloConfig{
    ServiceURL:   "http://localhost:8899",
    Timeout:      30,
    DefaultVoice: "EN",
    DefaultSpeed: 1.0,
}, logger)

// Try HF first
response, err := hfProvider.Synthesize(ctx, &tts.SynthesizeRequest{
    Text:    "Hello world",
    VoiceID: "EN",
    Speed:   1.0,
})
if err != nil {
    // Fallback to ElevenLabs
    audio, err = elevenlabsProvider.Synthesize("Hello world")
}
```

---

## Resources

- **API Documentation:** [HF_VOICE_INTEGRATION.md](HF_VOICE_INTEGRATION.md)
- **User Guide:** [HF_VOICE_USER_GUIDE.md](HF_VOICE_USER_GUIDE.md)
- **Deployment Guide:** [HF_VOICE_DEPLOYMENT.md](HF_VOICE_DEPLOYMENT.md)
- **GitHub Repository:** https://github.com/normanking/cortex-avatar
- **Issue Tracker:** https://github.com/normanking/cortex-avatar/issues

---

**Need Help?**
- Email: dev-support@cortexavatar.com
- Discord: https://discord.gg/cortexavatar-dev
- Stack Overflow: [cortex-avatar] tag
