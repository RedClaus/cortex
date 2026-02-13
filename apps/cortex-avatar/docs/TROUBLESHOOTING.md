---
project: Cortex
component: Docs
phase: Ideation
date_created: 2026-01-01T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:20:39.178185
---

# CortexAvatar Troubleshooting Guide for Claude Code

> **Purpose**: This guide helps Claude Code systematically diagnose and fix voice recognition, conversation flow, and memory issues in CortexAvatar - the voice/eyes/ears interface for Cortex.

---

## Table of Contents

1. [System Overview](#1-system-overview)
2. [Quick Diagnostic Commands](#2-quick-diagnostic-commands)
3. [Voice Input (STT) Issues](#3-voice-input-stt-issues)
4. [Voice Output (TTS) Issues](#4-voice-output-tts-issues)
5. [A2A Protocol Issues](#5-a2a-protocol-issues)
6. [ACP Protocol Issues](#6-acp-protocol-issues)
7. [Conversation Flow Issues](#7-conversation-flow-issues)
8. [Memory & Learning Issues](#8-memory--learning-issues)
9. [Context Retrieval Issues](#9-context-retrieval-issues)
10. [Bi-directional Communication Issues](#10-bi-directional-communication-issues)
11. [Performance Issues](#11-performance-issues)
12. [Integration Test Scenarios](#12-integration-test-scenarios)

---

## 1. System Overview

### Architecture Map

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           CORTEXAVATAR                                       │
│                    (Voice, Eyes, Ears for Cortex)                           │
│                                                                             │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐                     │
│  │    STT      │    │   Avatar    │    │    TTS      │                     │
│  │  (Whisper)  │───▶│   Brain     │───▶│  (Kokoro)   │                     │
│  │             │    │             │    │             │                     │
│  │ Microphone  │    │ Personality │    │  Speaker    │                     │
│  │ VAD/Wake    │    │ State Mgmt  │    │  Streaming  │                     │
│  └─────────────┘    └──────┬──────┘    └─────────────┘                     │
│                            │                                                │
│                    ┌───────┴───────┐                                        │
│                    │  A2A / ACP    │                                        │
│                    │   Gateway     │                                        │
│                    └───────┬───────┘                                        │
└────────────────────────────┼────────────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                             CORTEX                                           │
│                                                                             │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │                         COGNITIVE PLANE                               │   │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐                  │   │
│  │  │ Frontal │  │Parietal │  │ Limbic  │  │Temporal │                  │   │
│  │  │Reasoning│  │UserModel│  │ Safety  │  │ Memory  │                  │   │
│  │  └─────────┘  └─────────┘  └─────────┘  └─────────┘                  │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │                         MEMORY SYSTEMS                                │   │
│  │  ┌──────────────────┐  ┌──────────────────┐                          │   │
│  │  │ Knowledge Fabric │  │    Hindsight     │                          │   │
│  │  │ (Procedural)     │  │  (Declarative)   │                          │   │
│  │  │ Lessons/Patterns │  │ Facts/Experience │                          │   │
│  │  └──────────────────┘  └──────────────────┘                          │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Key Data Flows

1. **Voice Input → Cortex**: Mic → STT → A2A Signal → Cortex Sensory → Processing
2. **Cortex → Voice Output**: Cortex Motor → ACP Response → TTS → Speaker
3. **Memory Loop**: User Info → Temporal Lobe → Knowledge Fabric → Future Context
4. **Learning Loop**: Interaction → Outcome → Lesson → Embedding → Retrieval

---

## 2. Quick Diagnostic Commands

Run these first to establish baseline system health:

```bash
# ============================================================================
# SYSTEM HEALTH CHECK
# ============================================================================

# Check if Cortex is running
pgrep -f cortex && echo "✓ Cortex running" || echo "✗ Cortex not found"

# Check if CortexAvatar is running  
pgrep -f cortexavatar && echo "✓ Avatar running" || echo "✗ Avatar not found"

# Check Ollama status (for STT/TTS models)
curl -s http://localhost:11434/api/tags | jq '.models[].name' 2>/dev/null || echo "✗ Ollama not responding"

# Check Kokoro TTS container
docker ps | grep kokoro && echo "✓ Kokoro running" || echo "✗ Kokoro not found"

# Check A2A endpoint
curl -s http://localhost:8080/.well-known/agent.json | jq '.name' 2>/dev/null || echo "✗ A2A endpoint not responding"

# Check ACP endpoint
curl -s http://localhost:8080/acp/capabilities | jq '.' 2>/dev/null || echo "✗ ACP endpoint not responding"

# Check audio devices
# macOS
system_profiler SPAudioDataType 2>/dev/null | grep -A5 "Input" || echo "Check audio input"
# Linux
arecord -l 2>/dev/null || pactl list sources short 2>/dev/null || echo "Check audio input"

# Check memory database
ls -la ~/.cortex/knowledge.db 2>/dev/null && echo "✓ Knowledge DB exists" || echo "✗ Knowledge DB missing"
ls -la ~/.cortex/hindsight.db 2>/dev/null && echo "✓ Hindsight DB exists" || echo "✗ Hindsight DB missing"

# Check for recent logs
tail -20 ~/.cortex/logs/cortex.log 2>/dev/null || echo "No Cortex logs found"
tail -20 ~/.cortex/logs/avatar.log 2>/dev/null || echo "No Avatar logs found"
```

### Quick Status Script

Create this as `cortex-health.sh`:

```bash
#!/bin/bash
# Cortex + CortexAvatar Health Check

echo "═══════════════════════════════════════════════════════════════"
echo "                 CORTEX SYSTEM HEALTH CHECK"
echo "═══════════════════════════════════════════════════════════════"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

check() {
    if eval "$2" > /dev/null 2>&1; then
        echo -e "${GREEN}✓${NC} $1"
        return 0
    else
        echo -e "${RED}✗${NC} $1"
        return 1
    fi
}

warn() {
    echo -e "${YELLOW}⚠${NC} $1"
}

echo ""
echo "▸ Core Services"
check "Cortex process" "pgrep -f cortex"
check "CortexAvatar process" "pgrep -f cortexavatar"
check "Ollama service" "curl -s http://localhost:11434/api/tags"
check "Kokoro TTS container" "docker ps | grep -q kokoro"

echo ""
echo "▸ Protocol Endpoints"
check "A2A Agent Card" "curl -s http://localhost:8080/.well-known/agent.json | jq -e '.name'"
check "ACP Capabilities" "curl -s http://localhost:8080/acp/capabilities | jq -e '.'"

echo ""
echo "▸ Audio Subsystem"
if [[ "$OSTYPE" == "darwin"* ]]; then
    check "Audio input device" "system_profiler SPAudioDataType | grep -q 'Input'"
else
    check "Audio input device" "arecord -l 2>/dev/null || pactl list sources short"
fi

echo ""
echo "▸ Memory Systems"
check "Knowledge Fabric DB" "test -f ~/.cortex/knowledge.db"
check "Hindsight DB" "test -f ~/.cortex/hindsight.db"

echo ""
echo "▸ Model Availability"
MODELS=$(curl -s http://localhost:11434/api/tags 2>/dev/null | jq -r '.models[].name' 2>/dev/null)
if echo "$MODELS" | grep -q "whisper"; then
    echo -e "${GREEN}✓${NC} Whisper STT model loaded"
else
    warn "Whisper model not found - STT may fail"
fi
if echo "$MODELS" | grep -q "llama"; then
    echo -e "${GREEN}✓${NC} LLaMA model loaded"
else
    warn "LLaMA model not found - reasoning may fail"
fi

echo ""
echo "═══════════════════════════════════════════════════════════════"
```

---

## 3. Voice Input (STT) Issues

### Symptom Categories

| Symptom | Likely Cause | Section |
|---------|--------------|---------|
| No response to voice | Mic not capturing | 3.1 |
| Garbled transcription | STT model issue | 3.2 |
| High latency on speech | Processing bottleneck | 3.3 |
| Wake word not detected | VAD configuration | 3.4 |
| Partial transcription | Buffer/timeout issue | 3.5 |

### 3.1 Microphone Not Capturing

**Diagnostic Steps:**

```bash
# 1. Test raw audio capture
# macOS
rec -c 1 -r 16000 test.wav trim 0 5
play test.wav

# Linux
arecord -d 5 -f S16_LE -r 16000 test.wav
aplay test.wav

# 2. Check audio permissions (macOS)
tccutil reset Microphone
# Then re-grant permission to CortexAvatar

# 3. Check audio levels
# macOS
osascript -e "input volume of (get volume settings)"

# 4. Verify CortexAvatar has mic access
# Check logs for permission errors
grep -i "microphone\|audio\|permission" ~/.cortex/logs/avatar.log
```

**Common Fixes:**

```go
// avatar/audio/capture.go

// Issue: Audio capture not initializing
// Fix: Add explicit device selection

func NewAudioCapture(cfg AudioConfig) (*AudioCapture, error) {
    // List available devices
    devices, err := portaudio.Devices()
    if err != nil {
        return nil, fmt.Errorf("failed to enumerate audio devices: %w", err)
    }
    
    // Find the default input device or specified device
    var inputDevice *portaudio.DeviceInfo
    for _, d := range devices {
        if d.MaxInputChannels > 0 {
            if cfg.DeviceName == "" || strings.Contains(d.Name, cfg.DeviceName) {
                inputDevice = d
                break
            }
        }
    }
    
    if inputDevice == nil {
        return nil, fmt.Errorf("no suitable input device found")
    }
    
    log.Printf("Using audio input: %s", inputDevice.Name)
    
    // Initialize stream with explicit parameters
    params := portaudio.StreamParameters{
        Input: portaudio.StreamDeviceParameters{
            Device:   inputDevice,
            Channels: 1,
            Latency:  inputDevice.DefaultLowInputLatency,
        },
        SampleRate:      float64(cfg.SampleRate),
        FramesPerBuffer: cfg.BufferSize,
    }
    
    // ... rest of initialization
}
```

### 3.2 STT Model Issues

**Diagnostic Steps:**

```bash
# 1. Test Whisper directly
curl -X POST http://localhost:11434/api/generate \
  -d '{"model": "whisper", "prompt": "transcribe", "audio": "<base64_audio>"}' \
  | jq '.response'

# 2. Check model is loaded
curl -s http://localhost:11434/api/ps | jq '.models[] | select(.name | contains("whisper"))'

# 3. Check for model errors
grep -i "whisper\|stt\|transcri" ~/.cortex/logs/avatar.log | tail -20

# 4. Verify audio format matches model expectations
ffprobe test.wav 2>&1 | grep -E "Stream|Duration|Sample"
```

**Common Fixes:**

```go
// avatar/stt/whisper.go

// Issue: Whisper returning empty or garbage
// Fix: Ensure correct audio preprocessing

func (w *WhisperClient) Transcribe(ctx context.Context, audio []byte) (string, error) {
    // Validate audio format
    if len(audio) < 1000 {
        return "", fmt.Errorf("audio too short: %d bytes", len(audio))
    }
    
    // Ensure 16kHz mono PCM
    processed, err := w.preprocessAudio(audio)
    if err != nil {
        return "", fmt.Errorf("audio preprocessing failed: %w", err)
    }
    
    // Add silence detection - skip if too quiet
    if w.calculateRMS(processed) < w.config.MinRMSThreshold {
        return "", ErrAudioTooQuiet
    }
    
    // Call Whisper with timeout
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()
    
    result, err := w.client.Transcribe(ctx, TranscribeRequest{
        Audio:    processed,
        Language: w.config.Language,  // "en" or "auto"
        Task:     "transcribe",       // not "translate"
    })
    
    if err != nil {
        return "", fmt.Errorf("whisper transcription failed: %w", err)
    }
    
    // Post-process: clean up common artifacts
    text := w.cleanTranscription(result.Text)
    
    return text, nil
}

func (w *WhisperClient) preprocessAudio(audio []byte) ([]byte, error) {
    // Convert to 16kHz mono if needed
    reader := bytes.NewReader(audio)
    
    // Detect format
    format, err := detectAudioFormat(reader)
    if err != nil {
        return nil, err
    }
    
    // Resample if needed
    if format.SampleRate != 16000 {
        audio, err = resample(audio, format.SampleRate, 16000)
        if err != nil {
            return nil, err
        }
    }
    
    // Convert to mono if stereo
    if format.Channels > 1 {
        audio = toMono(audio, format.Channels)
    }
    
    return audio, nil
}
```

### 3.3 High Latency on Speech Recognition

**Diagnostic Steps:**

```bash
# 1. Measure STT latency
time curl -X POST http://localhost:11434/api/generate \
  -d '{"model": "whisper", "prompt": "transcribe", "audio": "'$(base64 -i test.wav)'"}'

# 2. Check for model swapping (VRAM thrashing)
watch -n 0.5 'curl -s http://localhost:11434/api/ps | jq ".models[].name"'

# 3. Profile CPU/GPU during transcription
# macOS
sudo powermetrics --samplers gpu_power -i 500 -n 10

# 4. Check for queuing delays
grep -i "queue\|wait\|latency" ~/.cortex/logs/avatar.log
```

**Common Fixes:**

```go
// avatar/stt/pipeline.go

// Issue: Latency spikes due to model loading
// Fix: Keep Whisper warm with periodic pings

type STTPipeline struct {
    whisper    *WhisperClient
    warmTicker *time.Ticker
    warmCtx    context.Context
    warmCancel context.CancelFunc
}

func NewSTTPipeline(whisper *WhisperClient) *STTPipeline {
    ctx, cancel := context.WithCancel(context.Background())
    
    p := &STTPipeline{
        whisper:    whisper,
        warmTicker: time.NewTicker(30 * time.Second),
        warmCtx:    ctx,
        warmCancel: cancel,
    }
    
    // Start warm-keeping goroutine
    go p.keepWarm()
    
    return p
}

func (p *STTPipeline) keepWarm() {
    // Send minimal audio to keep model in VRAM
    silentAudio := make([]byte, 16000*2) // 1 second of silence
    
    for {
        select {
        case <-p.warmCtx.Done():
            return
        case <-p.warmTicker.C:
            ctx, cancel := context.WithTimeout(p.warmCtx, 5*time.Second)
            _, _ = p.whisper.Transcribe(ctx, silentAudio)
            cancel()
        }
    }
}

// Issue: Blocking on transcription
// Fix: Implement streaming/chunked transcription

func (p *STTPipeline) TranscribeStreaming(ctx context.Context, audioStream <-chan []byte) (<-chan string, error) {
    results := make(chan string, 10)
    
    go func() {
        defer close(results)
        
        var buffer []byte
        flushTicker := time.NewTicker(500 * time.Millisecond)
        defer flushTicker.Stop()
        
        for {
            select {
            case <-ctx.Done():
                return
                
            case chunk, ok := <-audioStream:
                if !ok {
                    // Stream ended, transcribe remaining
                    if len(buffer) > 0 {
                        text, _ := p.whisper.Transcribe(ctx, buffer)
                        if text != "" {
                            results <- text
                        }
                    }
                    return
                }
                buffer = append(buffer, chunk...)
                
            case <-flushTicker.C:
                // Transcribe accumulated audio
                if len(buffer) >= 16000*2 { // At least 1 second
                    text, err := p.whisper.Transcribe(ctx, buffer)
                    if err == nil && text != "" {
                        results <- text
                    }
                    buffer = buffer[:0]
                }
            }
        }
    }()
    
    return results, nil
}
```

### 3.4 Wake Word Detection Issues

**Diagnostic Steps:**

```bash
# 1. Check VAD (Voice Activity Detection) settings
grep -i "vad\|wake\|trigger" ~/.cortex/config/avatar.yaml

# 2. Test wake word sensitivity
# Record yourself saying the wake word
rec -c 1 -r 16000 wake_test.wav trim 0 3
# Analyze energy levels
sox wake_test.wav -n stat 2>&1 | grep -E "RMS|Maximum"

# 3. Check for false negatives in logs
grep -i "wake\|trigger\|listen" ~/.cortex/logs/avatar.log | tail -50
```

**Common Fixes:**

```go
// avatar/audio/vad.go

// Issue: Wake word not detected
// Fix: Implement adaptive threshold VAD

type AdaptiveVAD struct {
    config       VADConfig
    noiseFloor   float64
    speechThresh float64
    history      *RingBuffer[float64]
    mu           sync.RWMutex
}

type VADConfig struct {
    InitialNoiseFloor  float64       // Starting noise estimate
    AdaptationRate     float64       // How fast to adapt (0.01-0.1)
    SpeechMultiplier   float64       // Noise * this = speech threshold
    MinSpeechDuration  time.Duration // Min duration to count as speech
    HangoverDuration   time.Duration // Keep listening after speech stops
}

func NewAdaptiveVAD(cfg VADConfig) *AdaptiveVAD {
    return &AdaptiveVAD{
        config:       cfg,
        noiseFloor:   cfg.InitialNoiseFloor,
        speechThresh: cfg.InitialNoiseFloor * cfg.SpeechMultiplier,
        history:      NewRingBuffer[float64](100),
    }
}

func (v *AdaptiveVAD) ProcessFrame(frame []byte) VADResult {
    energy := v.calculateEnergy(frame)
    
    v.mu.Lock()
    defer v.mu.Unlock()
    
    // Adapt noise floor during silence
    if energy < v.speechThresh {
        v.noiseFloor = v.noiseFloor*(1-v.config.AdaptationRate) + 
                       energy*v.config.AdaptationRate
        v.speechThresh = v.noiseFloor * v.config.SpeechMultiplier
    }
    
    v.history.Push(energy)
    
    // Detect speech
    isSpeech := energy > v.speechThresh
    
    return VADResult{
        IsSpeech:     isSpeech,
        Energy:       energy,
        NoiseFloor:   v.noiseFloor,
        Threshold:    v.speechThresh,
        Confidence:   v.calculateConfidence(energy),
    }
}

// Issue: Wake word false positives/negatives
// Fix: Implement proper wake word detection

type WakeWordDetector struct {
    vad        *AdaptiveVAD
    keywords   []string
    stt        STTClient
    buffer     *AudioBuffer
    listening  bool
    mu         sync.Mutex
}

func (w *WakeWordDetector) ProcessAudio(audio []byte) *WakeWordEvent {
    vadResult := w.vad.ProcessFrame(audio)
    
    w.mu.Lock()
    defer w.mu.Unlock()
    
    // Always buffer recent audio
    w.buffer.Write(audio)
    
    if vadResult.IsSpeech {
        w.listening = true
        return nil // Keep collecting
    }
    
    if w.listening && !vadResult.IsSpeech {
        // Speech ended, check for wake word
        w.listening = false
        
        audioData := w.buffer.Read()
        if len(audioData) < 8000 { // Too short
            return nil
        }
        
        // Quick transcription for wake word check
        ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
        defer cancel()
        
        text, err := w.stt.TranscribeQuick(ctx, audioData)
        if err != nil {
            return nil
        }
        
        // Check for wake word
        textLower := strings.ToLower(text)
        for _, keyword := range w.keywords {
            if strings.Contains(textLower, keyword) {
                return &WakeWordEvent{
                    Keyword:    keyword,
                    FullText:   text,
                    Audio:      audioData,
                    Confidence: vadResult.Confidence,
                }
            }
        }
    }
    
    return nil
}
```

### 3.5 Partial Transcription Issues

**Diagnostic Steps:**

```bash
# 1. Check audio buffer configuration
grep -i "buffer\|chunk\|timeout" ~/.cortex/config/avatar.yaml

# 2. Look for timeout errors
grep -i "timeout\|truncat\|partial" ~/.cortex/logs/avatar.log

# 3. Check for audio discontinuities
# Record and analyze
rec -c 1 -r 16000 long_test.wav trim 0 30
sox long_test.wav -n spectrogram -o spectrogram.png
```

**Common Fixes:**

```go
// avatar/stt/buffer.go

// Issue: Long utterances getting cut off
// Fix: Implement proper sentence boundary detection

type UtteranceBuffer struct {
    chunks         [][]byte
    totalDuration  time.Duration
    maxDuration    time.Duration
    silenceTimeout time.Duration
    lastActivity   time.Time
    vad            *AdaptiveVAD
}

func (u *UtteranceBuffer) AddChunk(chunk []byte, duration time.Duration) BufferStatus {
    u.chunks = append(u.chunks, chunk)
    u.totalDuration += duration
    
    vadResult := u.vad.ProcessFrame(chunk)
    
    if vadResult.IsSpeech {
        u.lastActivity = time.Now()
    }
    
    // Check if we should flush
    silenceDuration := time.Since(u.lastActivity)
    
    // Natural sentence boundary: pause after speech
    if silenceDuration > u.silenceTimeout && len(u.chunks) > 0 {
        return BufferStatusReadyToFlush
    }
    
    // Hard limit: max duration reached
    if u.totalDuration > u.maxDuration {
        return BufferStatusForceFlush
    }
    
    return BufferStatusCollecting
}

func (u *UtteranceBuffer) Flush() []byte {
    if len(u.chunks) == 0 {
        return nil
    }
    
    // Concatenate all chunks
    totalSize := 0
    for _, c := range u.chunks {
        totalSize += len(c)
    }
    
    result := make([]byte, 0, totalSize)
    for _, c := range u.chunks {
        result = append(result, c...)
    }
    
    // Reset
    u.chunks = u.chunks[:0]
    u.totalDuration = 0
    
    return result
}
```

---

## 4. Voice Output (TTS) Issues

### Symptom Categories

| Symptom | Likely Cause | Section |
|---------|--------------|---------|
| No audio output | Speaker/route issue | 4.1 |
| Robotic/garbled voice | Model quality | 4.2 |
| Slow TTS response | Generation latency | 4.3 |
| Audio cuts off | Streaming issue | 4.4 |
| Wrong voice/personality | Persona config | 4.5 |

### 4.1 No Audio Output

**Diagnostic Steps:**

```bash
# 1. Test Kokoro TTS directly
curl -X POST http://localhost:5000/tts \
  -H "Content-Type: application/json" \
  -d '{"text": "Hello, this is a test", "voice": "default"}' \
  --output test_tts.wav
play test_tts.wav

# 2. Check Kokoro container logs
docker logs kokoro-tts 2>&1 | tail -50

# 3. Check audio output device
# macOS
system_profiler SPAudioDataType | grep -A10 "Output"
# Linux
pactl list sinks short

# 4. Check for audio routing issues
# macOS - check if sound is going to right device
osascript -e "output volume of (get volume settings)"
```

**Common Fixes:**

```go
// avatar/tts/speaker.go

// Issue: Audio not playing
// Fix: Explicit output device selection and error handling

type Speaker struct {
    stream     *portaudio.Stream
    device     *portaudio.DeviceInfo
    buffer     chan []float32
    errorChan  chan error
    playing    atomic.Bool
}

func NewSpeaker(deviceName string) (*Speaker, error) {
    if err := portaudio.Initialize(); err != nil {
        return nil, fmt.Errorf("portaudio init failed: %w", err)
    }
    
    // Find output device
    devices, err := portaudio.Devices()
    if err != nil {
        return nil, fmt.Errorf("failed to list devices: %w", err)
    }
    
    var outputDevice *portaudio.DeviceInfo
    for _, d := range devices {
        if d.MaxOutputChannels > 0 {
            if deviceName == "" {
                outputDevice = d
                break
            }
            if strings.Contains(strings.ToLower(d.Name), strings.ToLower(deviceName)) {
                outputDevice = d
                break
            }
        }
    }
    
    if outputDevice == nil {
        return nil, fmt.Errorf("no suitable output device found")
    }
    
    log.Printf("Using audio output: %s", outputDevice.Name)
    
    s := &Speaker{
        device:    outputDevice,
        buffer:    make(chan []float32, 100),
        errorChan: make(chan error, 10),
    }
    
    // Test output by playing silence
    if err := s.testOutput(); err != nil {
        return nil, fmt.Errorf("output test failed: %w", err)
    }
    
    return s, nil
}

func (s *Speaker) testOutput() error {
    silence := make([]float32, 1024)
    
    stream, err := portaudio.OpenStream(portaudio.StreamParameters{
        Output: portaudio.StreamDeviceParameters{
            Device:   s.device,
            Channels: 1,
            Latency:  s.device.DefaultLowOutputLatency,
        },
        SampleRate:      24000,
        FramesPerBuffer: 1024,
    }, silence)
    
    if err != nil {
        return err
    }
    
    if err := stream.Start(); err != nil {
        stream.Close()
        return err
    }
    
    time.Sleep(100 * time.Millisecond)
    stream.Stop()
    stream.Close()
    
    return nil
}
```

### 4.2 Voice Quality Issues

**Diagnostic Steps:**

```bash
# 1. Check Kokoro model configuration
docker exec kokoro-tts cat /app/config.yaml

# 2. Test with different voice settings
curl -X POST http://localhost:5000/tts \
  -H "Content-Type: application/json" \
  -d '{"text": "Testing voice quality", "voice": "default", "speed": 1.0, "pitch": 1.0}' \
  --output quality_test.wav

# Analyze
sox quality_test.wav -n stat 2>&1

# 3. Check for sample rate mismatches
ffprobe quality_test.wav 2>&1 | grep "Stream"
```

**Common Fixes:**

```go
// avatar/tts/kokoro.go

// Issue: Poor voice quality
// Fix: Proper audio processing and parameter tuning

type KokoroClient struct {
    baseURL     string
    voiceConfig VoiceConfig
    httpClient  *http.Client
}

type VoiceConfig struct {
    VoiceID     string  `json:"voice_id"`
    Speed       float64 `json:"speed"`       // 0.5 - 2.0
    Pitch       float64 `json:"pitch"`       // 0.5 - 2.0
    Energy      float64 `json:"energy"`      // 0.0 - 1.0
    SampleRate  int     `json:"sample_rate"` // 22050 or 24000
    
    // Persona-specific settings
    Warmth      float64 `json:"warmth"`      // Custom: affects tone
    Stability   float64 `json:"stability"`   // Consistency
}

func (k *KokoroClient) Synthesize(ctx context.Context, text string) ([]byte, error) {
    // Preprocess text for better synthesis
    text = k.preprocessText(text)
    
    // Split into sentences for more natural prosody
    sentences := k.splitIntoSentences(text)
    
    var audioChunks [][]byte
    
    for _, sentence := range sentences {
        chunk, err := k.synthesizeSentence(ctx, sentence)
        if err != nil {
            return nil, err
        }
        audioChunks = append(audioChunks, chunk)
    }
    
    // Concatenate with small pauses between sentences
    return k.concatenateWithPauses(audioChunks, 150*time.Millisecond), nil
}

func (k *KokoroClient) preprocessText(text string) string {
    // Expand abbreviations for better pronunciation
    replacements := map[string]string{
        "Dr.":  "Doctor",
        "Mr.":  "Mister",
        "Mrs.": "Missus",
        "vs.":  "versus",
        "etc.": "etcetera",
        "i.e.": "that is",
        "e.g.": "for example",
    }
    
    for abbr, expansion := range replacements {
        text = strings.ReplaceAll(text, abbr, expansion)
    }
    
    // Handle numbers
    text = k.expandNumbers(text)
    
    // Add SSML-like hints for emphasis
    text = k.addProsodyHints(text)
    
    return text
}

func (k *KokoroClient) synthesizeSentence(ctx context.Context, text string) ([]byte, error) {
    req := TTSRequest{
        Text:       text,
        VoiceID:    k.voiceConfig.VoiceID,
        Speed:      k.voiceConfig.Speed,
        Pitch:      k.voiceConfig.Pitch,
        SampleRate: k.voiceConfig.SampleRate,
    }
    
    body, _ := json.Marshal(req)
    
    httpReq, err := http.NewRequestWithContext(ctx, "POST", 
        k.baseURL+"/tts", bytes.NewReader(body))
    if err != nil {
        return nil, err
    }
    
    httpReq.Header.Set("Content-Type", "application/json")
    
    resp, err := k.httpClient.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("TTS request failed: %w", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("TTS error %d: %s", resp.StatusCode, body)
    }
    
    return io.ReadAll(resp.Body)
}
```

### 4.3 TTS Latency Issues

**Diagnostic Steps:**

```bash
# 1. Measure TTS generation time
time curl -X POST http://localhost:5000/tts \
  -d '{"text": "This is a test sentence for measuring latency."}' \
  --output /dev/null

# 2. Check Kokoro GPU utilization
docker exec kokoro-tts nvidia-smi 2>/dev/null || echo "No GPU in container"

# 3. Check for queuing
docker exec kokoro-tts cat /app/logs/tts.log | grep -i "queue\|wait"

# 4. Profile the full pipeline
# Add timing to avatar logs
grep -E "tts_start|tts_end|speak_start|speak_end" ~/.cortex/logs/avatar.log
```

**Common Fixes:**

```go
// avatar/tts/streaming.go

// Issue: Long wait before speech starts
// Fix: Implement streaming TTS with chunked playback

type StreamingTTS struct {
    kokoro     *KokoroClient
    speaker    *Speaker
    bufferSize int
}

func (s *StreamingTTS) SpeakStreaming(ctx context.Context, text string) error {
    // Split text into speakable chunks
    chunks := s.splitForStreaming(text)
    
    // Create pipeline: generate -> buffer -> play
    audioChan := make(chan []byte, 3) // Buffer 3 chunks ahead
    errChan := make(chan error, 1)
    
    // Generator goroutine
    go func() {
        defer close(audioChan)
        for _, chunk := range chunks {
            audio, err := s.kokoro.Synthesize(ctx, chunk)
            if err != nil {
                errChan <- err
                return
            }
            select {
            case audioChan <- audio:
            case <-ctx.Done():
                return
            }
        }
    }()
    
    // Player goroutine - starts as soon as first chunk is ready
    go func() {
        for audio := range audioChan {
            if err := s.speaker.Play(audio); err != nil {
                errChan <- err
                return
            }
        }
    }()
    
    // Wait for completion or error
    select {
    case err := <-errChan:
        return err
    case <-ctx.Done():
        return ctx.Err()
    }
}

func (s *StreamingTTS) splitForStreaming(text string) []string {
    // Split at natural boundaries for streaming
    // Aim for ~2-3 second chunks
    var chunks []string
    
    sentences := splitSentences(text)
    
    var current strings.Builder
    wordCount := 0
    
    for _, sentence := range sentences {
        words := strings.Fields(sentence)
        
        if wordCount+len(words) > 20 { // ~20 words ≈ 3 seconds
            if current.Len() > 0 {
                chunks = append(chunks, current.String())
                current.Reset()
                wordCount = 0
            }
        }
        
        if current.Len() > 0 {
            current.WriteString(" ")
        }
        current.WriteString(sentence)
        wordCount += len(words)
    }
    
    if current.Len() > 0 {
        chunks = append(chunks, current.String())
    }
    
    return chunks
}
```

### 4.4 Audio Cutoff Issues

**Diagnostic Steps:**

```bash
# 1. Check for buffer underruns
grep -i "underrun\|buffer\|xrun" ~/.cortex/logs/avatar.log

# 2. Monitor audio stream status
# During playback, check for errors
dmesg | grep -i audio  # Linux
log show --predicate 'subsystem == "com.apple.audio"' --last 5m  # macOS

# 3. Check audio file integrity
ffprobe problematic_audio.wav 2>&1 | grep -i "error\|invalid"
```

**Common Fixes:**

```go
// avatar/tts/playback.go

// Issue: Audio cutting off prematurely
// Fix: Proper stream management and draining

type PlaybackManager struct {
    speaker    *Speaker
    mutex      sync.Mutex
    isPlaying  bool
    stopChan   chan struct{}
}

func (p *PlaybackManager) Play(audio []byte) error {
    p.mutex.Lock()
    defer p.mutex.Unlock()
    
    // Stop any current playback
    if p.isPlaying {
        close(p.stopChan)
        time.Sleep(50 * time.Millisecond) // Allow cleanup
    }
    
    p.stopChan = make(chan struct{})
    p.isPlaying = true
    
    // Decode audio
    samples, sampleRate, err := decodeAudio(audio)
    if err != nil {
        p.isPlaying = false
        return fmt.Errorf("audio decode failed: %w", err)
    }
    
    // Add fade-out to prevent clicks
    samples = addFadeOut(samples, 100) // 100 sample fade
    
    // Play with proper draining
    err = p.playWithDrain(samples, sampleRate)
    p.isPlaying = false
    
    return err
}

func (p *PlaybackManager) playWithDrain(samples []float32, sampleRate int) error {
    stream, err := p.speaker.CreateStream(sampleRate, len(samples))
    if err != nil {
        return err
    }
    defer stream.Close()
    
    if err := stream.Start(); err != nil {
        return err
    }
    
    // Write samples in chunks
    chunkSize := 1024
    for i := 0; i < len(samples); i += chunkSize {
        select {
        case <-p.stopChan:
            // Interrupted - fade out quickly
            stream.Stop()
            return nil
        default:
        }
        
        end := i + chunkSize
        if end > len(samples) {
            end = len(samples)
        }
        
        if err := stream.Write(samples[i:end]); err != nil {
            return err
        }
    }
    
    // CRITICAL: Wait for buffer to drain
    // This prevents audio cutoff
    bufferDuration := time.Duration(float64(chunkSize) / float64(sampleRate) * float64(time.Second))
    time.Sleep(bufferDuration * 2)
    
    stream.Stop()
    return nil
}

func addFadeOut(samples []float32, fadeLength int) []float32 {
    if len(samples) < fadeLength {
        return samples
    }
    
    result := make([]float32, len(samples))
    copy(result, samples)
    
    startFade := len(result) - fadeLength
    for i := 0; i < fadeLength; i++ {
        factor := float32(fadeLength-i) / float32(fadeLength)
        result[startFade+i] *= factor
    }
    
    return result
}
```

### 4.5 Persona Voice Configuration

**Diagnostic Steps:**

```bash
# 1. Check current persona configuration
cat ~/.cortex/config/persona.yaml | grep -A20 "voice:"

# 2. Verify voice ID exists in Kokoro
curl -s http://localhost:5000/voices | jq '.voices[].id'

# 3. Test persona-specific voice
curl -X POST http://localhost:5000/tts \
  -d '{"text": "Hello, I am Henry.", "voice": "henry-assistant-v1"}' \
  --output henry_test.wav
```

**Common Fixes:**

```go
// avatar/persona/voice_adapter.go

// Issue: Wrong voice for persona
// Fix: Proper persona-to-voice mapping

type VoiceAdapter struct {
    personaManager *persona.Manager
    kokoroClient   *KokoroClient
    voiceMap       map[persona.PersonaID]VoiceConfig
}

func NewVoiceAdapter(pm *persona.Manager, kc *KokoroClient) *VoiceAdapter {
    return &VoiceAdapter{
        personaManager: pm,
        kokoroClient:   kc,
        voiceMap: map[persona.PersonaID]VoiceConfig{
            persona.PersonaHenry: {
                VoiceID:    "henry-assistant-v1",
                Speed:      1.05,
                Pitch:      0.95,
                Warmth:     0.6,
                Stability:  0.8,
            },
            persona.PersonaHannah: {
                VoiceID:    "hannah-assistant-v1",
                Speed:      0.95,
                Pitch:      1.05,
                Warmth:     0.9,
                Stability:  0.7,
            },
        },
    }
}

func (v *VoiceAdapter) GetCurrentVoice() VoiceConfig {
    activePersona := v.personaManager.Active()
    
    if config, ok := v.voiceMap[activePersona.ID]; ok {
        return config
    }
    
    // Fallback to default
    return VoiceConfig{
        VoiceID: "default",
        Speed:   1.0,
        Pitch:   1.0,
    }
}

func (v *VoiceAdapter) SynthesizeWithPersona(ctx context.Context, text string) ([]byte, error) {
    voice := v.GetCurrentVoice()
    
    // Apply persona voice config to Kokoro
    v.kokoroClient.voiceConfig = voice
    
    return v.kokoroClient.Synthesize(ctx, text)
}
```

---

## 5. A2A Protocol Issues

### Symptom Categories

| Symptom | Likely Cause | Section |
|---------|--------------|---------|
| Agent card not found | Endpoint config | 5.1 |
| Task submission fails | Protocol mismatch | 5.2 |
| No task updates | SSE connection | 5.3 |
| Authentication errors | Credential issue | 5.4 |

### 5.1 Agent Card Issues

**Diagnostic Steps:**

```bash
# 1. Check agent card endpoint
curl -v http://localhost:8080/.well-known/agent.json

# 2. Validate agent card format
curl -s http://localhost:8080/.well-known/agent.json | python3 -m json.tool

# 3. Check required fields
curl -s http://localhost:8080/.well-known/agent.json | jq '{
  name: .name,
  url: .url,
  version: .version,
  capabilities: .capabilities
}'
```

**Common Fixes:**

```go
// avatar/a2a/agent_card.go

// Issue: Invalid or missing agent card
// Fix: Proper agent card implementation

type AgentCard struct {
    Name            string            `json:"name"`
    Description     string            `json:"description"`
    URL             string            `json:"url"`
    Version         string            `json:"version"`
    Protocol        string            `json:"protocol"`
    Capabilities    []string          `json:"capabilities"`
    Authentication  *AuthConfig       `json:"authentication,omitempty"`
    Skills          []SkillDefinition `json:"skills"`
    DefaultInputModes  []string       `json:"defaultInputModes"`
    DefaultOutputModes []string       `json:"defaultOutputModes"`
}

type SkillDefinition struct {
    ID          string   `json:"id"`
    Name        string   `json:"name"`
    Description string   `json:"description"`
    InputModes  []string `json:"inputModes"`
    OutputModes []string `json:"outputModes"`
}

func NewCortexAgentCard(baseURL string) *AgentCard {
    return &AgentCard{
        Name:        "Cortex",
        Description: "AI assistant with voice, memory, and learning capabilities",
        URL:         baseURL,
        Version:     "1.0.0",
        Protocol:    "a2a/1.0",
        Capabilities: []string{
            "streaming",
            "push-notifications",
            "multi-turn",
        },
        Skills: []SkillDefinition{
            {
                ID:          "conversation",
                Name:        "Natural Conversation",
                Description: "Engage in natural language conversation with memory",
                InputModes:  []string{"text", "audio"},
                OutputModes: []string{"text", "audio"},
            },
            {
                ID:          "task-execution",
                Name:        "Task Execution",
                Description: "Execute tasks with planning and safety checks",
                InputModes:  []string{"text"},
                OutputModes: []string{"text", "structured"},
            },
            {
                ID:          "memory-query",
                Name:        "Memory Query",
                Description: "Query learned information about the user",
                InputModes:  []string{"text"},
                OutputModes: []string{"text", "structured"},
            },
        },
        DefaultInputModes:  []string{"text", "audio"},
        DefaultOutputModes: []string{"text", "audio"},
    }
}

// Serve agent card
func (s *A2AServer) handleAgentCard(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("Access-Control-Allow-Origin", "*")
    
    card := NewCortexAgentCard(s.baseURL)
    
    if err := json.NewEncoder(w).Encode(card); err != nil {
        http.Error(w, "Failed to encode agent card", http.StatusInternalServerError)
        return
    }
}
```

### 5.2 Task Submission Issues

**Diagnostic Steps:**

```bash
# 1. Test task submission
curl -X POST http://localhost:8080/a2a/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "id": "test-task-1",
    "message": {
      "role": "user",
      "parts": [{"text": "Hello, how are you?"}]
    }
  }'

# 2. Check task status
curl http://localhost:8080/a2a/tasks/test-task-1

# 3. Look for validation errors
grep -i "task\|validation\|parse" ~/.cortex/logs/avatar.log | tail -20
```

**Common Fixes:**

```go
// avatar/a2a/task_handler.go

// Issue: Task submission failing
// Fix: Robust task parsing and validation

type TaskRequest struct {
    ID             string          `json:"id"`
    SessionID      string          `json:"sessionId,omitempty"`
    Message        Message         `json:"message"`
    PushNotification *PushConfig   `json:"pushNotification,omitempty"`
    HistoryLength  int             `json:"historyLength,omitempty"`
}

type Message struct {
    Role     string   `json:"role"`     // "user" or "agent"
    Parts    []Part   `json:"parts"`
    Metadata Metadata `json:"metadata,omitempty"`
}

type Part struct {
    Type     string `json:"type,omitempty"` // "text", "audio", "file"
    Text     string `json:"text,omitempty"`
    Audio    *Audio `json:"audio,omitempty"`
    MimeType string `json:"mimeType,omitempty"`
    Data     string `json:"data,omitempty"` // base64
}

func (h *TaskHandler) handleTaskSubmit(w http.ResponseWriter, r *http.Request) {
    var req TaskRequest
    
    // Parse with size limit
    r.Body = http.MaxBytesReader(w, r.Body, 10*1024*1024) // 10MB max
    
    decoder := json.NewDecoder(r.Body)
    decoder.DisallowUnknownFields() // Strict parsing
    
    if err := decoder.Decode(&req); err != nil {
        h.sendError(w, http.StatusBadRequest, "invalid_request", 
            fmt.Sprintf("Failed to parse request: %v", err))
        return
    }
    
    // Validate required fields
    if err := h.validateTaskRequest(&req); err != nil {
        h.sendError(w, http.StatusBadRequest, "validation_error", err.Error())
        return
    }
    
    // Generate ID if not provided
    if req.ID == "" {
        req.ID = uuid.New().String()
    }
    
    // Create task
    task, err := h.createTask(&req)
    if err != nil {
        h.sendError(w, http.StatusInternalServerError, "task_creation_failed", err.Error())
        return
    }
    
    // Send to Cortex via EventBus
    h.submitToCortex(task)
    
    // Return task info
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(TaskResponse{
        ID:     task.ID,
        Status: "submitted",
    })
}

func (h *TaskHandler) validateTaskRequest(req *TaskRequest) error {
    if req.Message.Role == "" {
        return fmt.Errorf("message.role is required")
    }
    
    if req.Message.Role != "user" && req.Message.Role != "agent" {
        return fmt.Errorf("message.role must be 'user' or 'agent'")
    }
    
    if len(req.Message.Parts) == 0 {
        return fmt.Errorf("message.parts cannot be empty")
    }
    
    for i, part := range req.Message.Parts {
        if part.Text == "" && part.Audio == nil && part.Data == "" {
            return fmt.Errorf("message.parts[%d] has no content", i)
        }
    }
    
    return nil
}
```

### 5.3 SSE Connection Issues

**Diagnostic Steps:**

```bash
# 1. Test SSE endpoint
curl -N http://localhost:8080/a2a/tasks/test-task-1/events

# 2. Check for connection resets
grep -i "sse\|stream\|connection" ~/.cortex/logs/avatar.log

# 3. Monitor active connections
netstat -an | grep 8080 | grep ESTABLISHED
```

**Common Fixes:**

```go
// avatar/a2a/sse_handler.go

// Issue: SSE not sending updates
// Fix: Proper SSE implementation with keep-alive

type SSEHandler struct {
    tasks    *TaskStore
    clients  map[string]map[chan TaskEvent]bool
    mu       sync.RWMutex
}

func (h *SSEHandler) handleTaskEvents(w http.ResponseWriter, r *http.Request) {
    taskID := chi.URLParam(r, "taskId")
    
    // Check task exists
    task, err := h.tasks.Get(taskID)
    if err != nil {
        http.Error(w, "Task not found", http.StatusNotFound)
        return
    }
    
    // Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering
    
    // Create client channel
    events := make(chan TaskEvent, 100)
    h.addClient(taskID, events)
    defer h.removeClient(taskID, events)
    
    // Flush initial headers
    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
        return
    }
    flusher.Flush()
    
    // Send initial state
    h.sendEvent(w, flusher, TaskEvent{
        Type: "status",
        Data: task.Status,
    })
    
    // Keep-alive ticker
    keepAlive := time.NewTicker(15 * time.Second)
    defer keepAlive.Stop()
    
    ctx := r.Context()
    
    for {
        select {
        case <-ctx.Done():
            return
            
        case event := <-events:
            h.sendEvent(w, flusher, event)
            
            // If task completed/failed, close after sending final event
            if event.Type == "complete" || event.Type == "error" {
                return
            }
            
        case <-keepAlive.C:
            // Send keep-alive comment
            fmt.Fprintf(w, ": keep-alive\n\n")
            flusher.Flush()
        }
    }
}

func (h *SSEHandler) sendEvent(w http.ResponseWriter, f http.Flusher, event TaskEvent) {
    data, _ := json.Marshal(event.Data)
    
    fmt.Fprintf(w, "event: %s\n", event.Type)
    fmt.Fprintf(w, "data: %s\n\n", data)
    f.Flush()
}

func (h *SSEHandler) BroadcastTaskUpdate(taskID string, event TaskEvent) {
    h.mu.RLock()
    clients, ok := h.clients[taskID]
    h.mu.RUnlock()
    
    if !ok {
        return
    }
    
    for client := range clients {
        select {
        case client <- event:
        default:
            // Client buffer full, skip
        }
    }
}
```

---

## 6. ACP Protocol Issues

### Symptom Categories

| Symptom | Likely Cause | Section |
|---------|--------------|---------|
| Capabilities not listed | Registration issue | 6.1 |
| Capability call fails | Handler error | 6.2 |
| Response format wrong | Serialization issue | 6.3 |

### 6.1 Capability Registration

**Diagnostic Steps:**

```bash
# 1. List all capabilities
curl -s http://localhost:8080/acp/capabilities | jq '.'

# 2. Get specific capability info
curl -s http://localhost:8080/acp/capabilities/memory-query | jq '.'

# 3. Check registration logs
grep -i "capability\|register" ~/.cortex/logs/cortex.log
```

**Common Fixes:**

```go
// avatar/acp/registry.go

// Issue: Capabilities not appearing
// Fix: Proper capability registration

type CapabilityRegistry struct {
    capabilities map[string]Capability
    mu           sync.RWMutex
}

type Capability struct {
    ID          string                 `json:"id"`
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    Version     string                 `json:"version"`
    Schema      *JSONSchema            `json:"schema,omitempty"`
    Handler     CapabilityHandler      `json:"-"`
}

type CapabilityHandler func(ctx context.Context, req CapabilityRequest) (*CapabilityResponse, error)

func NewCapabilityRegistry() *CapabilityRegistry {
    r := &CapabilityRegistry{
        capabilities: make(map[string]Capability),
    }
    
    // Register built-in capabilities
    r.registerBuiltins()
    
    return r
}

func (r *CapabilityRegistry) registerBuiltins() {
    // Memory query capability
    r.Register(Capability{
        ID:          "memory-query",
        Name:        "Memory Query",
        Description: "Query Cortex's memory about the user",
        Version:     "1.0.0",
        Schema: &JSONSchema{
            Type: "object",
            Properties: map[string]JSONSchema{
                "query": {Type: "string", Description: "Natural language query"},
                "limit": {Type: "integer", Description: "Max results", Default: 5},
            },
            Required: []string{"query"},
        },
        Handler: r.handleMemoryQuery,
    })
    
    // Memory store capability
    r.Register(Capability{
        ID:          "memory-store",
        Name:        "Memory Store",
        Description: "Store information about the user",
        Version:     "1.0.0",
        Schema: &JSONSchema{
            Type: "object",
            Properties: map[string]JSONSchema{
                "fact":       {Type: "string", Description: "Fact to remember"},
                "category":   {Type: "string", Description: "Category (preference, fact, etc.)"},
                "confidence": {Type: "number", Description: "Confidence 0-1"},
            },
            Required: []string{"fact"},
        },
        Handler: r.handleMemoryStore,
    })
    
    // Context query capability
    r.Register(Capability{
        ID:          "context-query",
        Name:        "Context Query",
        Description: "Get relevant context for a query",
        Version:     "1.0.0",
        Handler:     r.handleContextQuery,
    })
    
    // User model capability
    r.Register(Capability{
        ID:          "user-model",
        Name:        "User Model",
        Description: "Get current understanding of user",
        Version:     "1.0.0",
        Handler:     r.handleUserModel,
    })
}

func (r *CapabilityRegistry) Register(cap Capability) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    if _, exists := r.capabilities[cap.ID]; exists {
        return fmt.Errorf("capability %s already registered", cap.ID)
    }
    
    r.capabilities[cap.ID] = cap
    log.Printf("Registered capability: %s", cap.ID)
    
    return nil
}
```

### 6.2 Capability Call Handling

**Diagnostic Steps:**

```bash
# 1. Call a capability directly
curl -X POST http://localhost:8080/acp/capabilities/memory-query/invoke \
  -H "Content-Type: application/json" \
  -d '{"query": "What do you know about me?"}'

# 2. Check for errors
grep -i "capability\|invoke\|error" ~/.cortex/logs/avatar.log | tail -20

# 3. Validate request format
curl -s http://localhost:8080/acp/capabilities/memory-query | jq '.schema'
```

**Common Fixes:**

```go
// avatar/acp/handler.go

// Issue: Capability calls failing
// Fix: Robust request handling and error recovery

func (h *ACPHandler) handleCapabilityInvoke(w http.ResponseWriter, r *http.Request) {
    capID := chi.URLParam(r, "capabilityId")
    
    // Get capability
    cap, err := h.registry.Get(capID)
    if err != nil {
        h.sendError(w, http.StatusNotFound, "capability_not_found", 
            fmt.Sprintf("Capability %s not found", capID))
        return
    }
    
    // Parse request
    var req CapabilityRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        h.sendError(w, http.StatusBadRequest, "invalid_request", err.Error())
        return
    }
    
    // Validate against schema if present
    if cap.Schema != nil {
        if err := h.validateSchema(req.Params, cap.Schema); err != nil {
            h.sendError(w, http.StatusBadRequest, "validation_error", err.Error())
            return
        }
    }
    
    // Create context with timeout
    ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
    defer cancel()
    
    // Call handler with panic recovery
    var resp *CapabilityResponse
    func() {
        defer func() {
            if r := recover(); r != nil {
                err = fmt.Errorf("capability panic: %v", r)
                log.Printf("Capability %s panicked: %v\n%s", capID, r, debug.Stack())
            }
        }()
        resp, err = cap.Handler(ctx, req)
    }()
    
    if err != nil {
        h.sendError(w, http.StatusInternalServerError, "execution_error", err.Error())
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
}

// Memory query handler
func (r *CapabilityRegistry) handleMemoryQuery(ctx context.Context, req CapabilityRequest) (*CapabilityResponse, error) {
    query, ok := req.Params["query"].(string)
    if !ok || query == "" {
        return nil, fmt.Errorf("query parameter is required")
    }
    
    limit := 5
    if l, ok := req.Params["limit"].(float64); ok {
        limit = int(l)
    }
    
    // Query temporal lobe
    memories, err := r.temporalLobe.Query(ctx, query, limit)
    if err != nil {
        return nil, fmt.Errorf("memory query failed: %w", err)
    }
    
    return &CapabilityResponse{
        Success: true,
        Data: map[string]interface{}{
            "memories": memories,
            "count":    len(memories),
            "query":    query,
        },
    }, nil
}
```

---

## 7. Conversation Flow Issues

### Symptom Categories

| Symptom | Likely Cause | Section |
|---------|--------------|---------|
| No response to query | Pipeline blocked | 7.1 |
| Context lost between turns | Session management | 7.2 |
| Wrong personality/tone | Persona not applied | 7.3 |
| Repetitive responses | History issue | 7.4 |

### 7.1 Pipeline Blockage

**Diagnostic Steps:**

```bash
# 1. Trace a message through the system
# Add trace ID to request and follow logs
TRACE_ID=$(uuidgen)
curl -X POST http://localhost:8080/a2a/tasks \
  -H "X-Trace-ID: $TRACE_ID" \
  -d '{"message": {"role": "user", "parts": [{"text": "test"}]}}'

grep "$TRACE_ID" ~/.cortex/logs/*.log

# 2. Check EventBus channels
curl http://localhost:8080/debug/eventbus | jq '.channel_depths'

# 3. Look for blocked goroutines
curl http://localhost:8080/debug/pprof/goroutine?debug=2 | head -100
```

**Common Fixes:**

```go
// avatar/pipeline/conversation.go

// Issue: Messages not flowing through
// Fix: Add instrumentation and timeout handling

type ConversationPipeline struct {
    stt           *STTPipeline
    llm           *LLMClient
    tts           *StreamingTTS
    eventBus      *kernel.EventBus
    metrics       *Metrics
    
    activeConversations sync.Map // sessionID -> *Conversation
}

type Conversation struct {
    ID           string
    SessionID    string
    Messages     []Message
    State        ConversationState
    StartedAt    time.Time
    LastActivity time.Time
}

func (p *ConversationPipeline) ProcessInput(ctx context.Context, input Input) (*Output, error) {
    start := time.Now()
    
    // Create span for tracing
    span := trace.StartSpan(ctx, "conversation.process")
    defer span.End()
    
    // 1. Transcribe if audio
    var text string
    if input.Audio != nil {
        transcribeStart := time.Now()
        var err error
        text, err = p.stt.Transcribe(ctx, input.Audio)
        p.metrics.RecordLatency("stt", time.Since(transcribeStart))
        
        if err != nil {
            return nil, fmt.Errorf("transcription failed: %w", err)
        }
        
        if text == "" {
            return nil, fmt.Errorf("transcription returned empty")
        }
    } else {
        text = input.Text
    }
    
    span.AddEvent("transcription_complete", trace.WithAttributes(
        attribute.String("text", text),
    ))
    
    // 2. Get or create conversation
    conv := p.getOrCreateConversation(input.SessionID)
    
    // 3. Build context from memory
    contextStart := time.Now()
    cortexCtx, err := p.buildContext(ctx, conv, text)
    p.metrics.RecordLatency("context_build", time.Since(contextStart))
    
    if err != nil {
        log.Printf("Warning: context build failed: %v", err)
        // Continue with minimal context
    }
    
    // 4. Send to Cortex via EventBus with timeout
    responseCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()
    
    llmStart := time.Now()
    response, err := p.sendToCortex(responseCtx, cortexCtx, text)
    p.metrics.RecordLatency("llm", time.Since(llmStart))
    
    if err != nil {
        if errors.Is(err, context.DeadlineExceeded) {
            return nil, fmt.Errorf("cortex response timeout after 30s")
        }
        return nil, fmt.Errorf("cortex processing failed: %w", err)
    }
    
    // 5. Update conversation history
    conv.Messages = append(conv.Messages,
        Message{Role: "user", Content: text},
        Message{Role: "assistant", Content: response.Text},
    )
    conv.LastActivity = time.Now()
    
    // 6. Synthesize speech
    var audio []byte
    if input.WantsAudio {
        ttsStart := time.Now()
        audio, err = p.tts.Synthesize(ctx, response.Text)
        p.metrics.RecordLatency("tts", time.Since(ttsStart))
        
        if err != nil {
            log.Printf("Warning: TTS failed: %v", err)
            // Return text response even if TTS fails
        }
    }
    
    p.metrics.RecordLatency("total", time.Since(start))
    
    return &Output{
        Text:      response.Text,
        Audio:     audio,
        SessionID: conv.SessionID,
    }, nil
}
```

### 7.2 Session Context Issues

**Diagnostic Steps:**

```bash
# 1. Check session storage
curl http://localhost:8080/debug/sessions | jq '.'

# 2. Verify session continuity
# Make two requests with same session ID
SESSION_ID="test-session-$(date +%s)"

curl -X POST http://localhost:8080/a2a/tasks \
  -d "{\"sessionId\": \"$SESSION_ID\", \"message\": {\"role\": \"user\", \"parts\": [{\"text\": \"My name is Alice\"}]}}"

sleep 2

curl -X POST http://localhost:8080/a2a/tasks \
  -d "{\"sessionId\": \"$SESSION_ID\", \"message\": {\"role\": \"user\", \"parts\": [{\"text\": \"What is my name?\"}]}}"

# 3. Check session expiry
grep -i "session\|expire\|timeout" ~/.cortex/logs/avatar.log
```

**Common Fixes:**

```go
// avatar/session/manager.go

// Issue: Context lost between turns
// Fix: Proper session management with persistence

type SessionManager struct {
    sessions    sync.Map
    store       SessionStore  // For persistence
    maxHistory  int
    expiry      time.Duration
    cleanupTick *time.Ticker
}

type Session struct {
    ID           string
    UserID       string
    ConvHistory  []Message
    ContextCache *ContextCache
    CreatedAt    time.Time
    LastActivity time.Time
    Metadata     map[string]interface{}
}

func NewSessionManager(store SessionStore, maxHistory int, expiry time.Duration) *SessionManager {
    sm := &SessionManager{
        store:       store,
        maxHistory:  maxHistory,
        expiry:      expiry,
        cleanupTick: time.NewTicker(5 * time.Minute),
    }
    
    go sm.cleanupLoop()
    
    return sm
}

func (sm *SessionManager) GetOrCreate(sessionID string) (*Session, error) {
    // Try memory first
    if sess, ok := sm.sessions.Load(sessionID); ok {
        session := sess.(*Session)
        session.LastActivity = time.Now()
        return session, nil
    }
    
    // Try persistent store
    session, err := sm.store.Load(sessionID)
    if err == nil && session != nil {
        session.LastActivity = time.Now()
        sm.sessions.Store(sessionID, session)
        return session, nil
    }
    
    // Create new session
    session = &Session{
        ID:           sessionID,
        ConvHistory:  make([]Message, 0),
        ContextCache: NewContextCache(),
        CreatedAt:    time.Now(),
        LastActivity: time.Now(),
        Metadata:     make(map[string]interface{}),
    }
    
    sm.sessions.Store(sessionID, session)
    
    return session, nil
}

func (sm *SessionManager) AddMessage(sessionID string, msg Message) error {
    session, err := sm.GetOrCreate(sessionID)
    if err != nil {
        return err
    }
    
    session.ConvHistory = append(session.ConvHistory, msg)
    
    // Trim to max history
    if len(session.ConvHistory) > sm.maxHistory {
        session.ConvHistory = session.ConvHistory[len(session.ConvHistory)-sm.maxHistory:]
    }
    
    session.LastActivity = time.Now()
    
    // Persist async
    go func() {
        if err := sm.store.Save(session); err != nil {
            log.Printf("Failed to persist session %s: %v", sessionID, err)
        }
    }()
    
    return nil
}

func (sm *SessionManager) GetHistory(sessionID string, limit int) ([]Message, error) {
    session, err := sm.GetOrCreate(sessionID)
    if err != nil {
        return nil, err
    }
    
    history := session.ConvHistory
    if limit > 0 && len(history) > limit {
        history = history[len(history)-limit:]
    }
    
    return history, nil
}
```

### 7.3 Persona Not Applied

**Diagnostic Steps:**

```bash
# 1. Check active persona
curl http://localhost:8080/debug/persona | jq '.'

# 2. Verify persona in response
# Ask about identity
curl -X POST http://localhost:8080/a2a/tasks \
  -d '{"message": {"role": "user", "parts": [{"text": "Who are you? What is your name?"}]}}'

# 3. Check persona loading
grep -i "persona\|henry\|hannah" ~/.cortex/logs/cortex.log
```

**Common Fixes:**

```go
// avatar/persona/integration.go

// Issue: Persona not reflected in responses
// Fix: Ensure persona is injected into system prompt

type PersonaIntegration struct {
    manager     *PersonaManager
    promptCache sync.Map
}

func (p *PersonaIntegration) BuildSystemPrompt(ctx context.Context, session *Session) string {
    persona := p.manager.Active()
    
    // Check cache
    cacheKey := fmt.Sprintf("%s-%s-%d", persona.ID, session.ID, len(session.ConvHistory))
    if cached, ok := p.promptCache.Load(cacheKey); ok {
        return cached.(string)
    }
    
    // Build prompt with persona
    var sb strings.Builder
    
    // Identity section
    sb.WriteString(persona.Prompts.Identity)
    sb.WriteString("\n\n---\n\n")
    
    // Core directives
    sb.WriteString(persona.Prompts.CoreDirectives)
    sb.WriteString("\n\n---\n\n")
    
    // Style guide
    sb.WriteString(persona.Prompts.StyleGuide)
    sb.WriteString("\n\n---\n\n")
    
    // Memory usage
    sb.WriteString(persona.Prompts.MemoryUsage)
    sb.WriteString("\n\n---\n\n")
    
    // Constraints
    sb.WriteString(persona.Prompts.Constraints)
    sb.WriteString("\n\n---\n\n")
    
    // Add conversation context
    if len(session.ConvHistory) > 0 {
        sb.WriteString("## Recent Conversation\n\n")
        for _, msg := range session.ConvHistory {
            sb.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
        }
        sb.WriteString("\n---\n\n")
    }
    
    // Add user context if available
    if userCtx := session.ContextCache.GetUserContext(); userCtx != "" {
        sb.WriteString("## User Context\n\n")
        sb.WriteString(userCtx)
        sb.WriteString("\n\n")
    }
    
    prompt := sb.String()
    p.promptCache.Store(cacheKey, prompt)
    
    return prompt
}
```

---

## 8. Memory & Learning Issues

### Symptom Categories

| Symptom | Likely Cause | Section |
|---------|--------------|---------|
| Not remembering user info | Storage issue | 8.1 |
| Not learning from corrections | Learning disabled | 8.2 |
| Retrieving wrong memories | Search issue | 8.3 |
| Memory not persisting | Database issue | 8.4 |

### 8.1 User Information Not Stored

**Diagnostic Steps:**

```bash
# 1. Check knowledge database
sqlite3 ~/.cortex/knowledge.db "SELECT * FROM knowledge_items WHERE type='user_fact' ORDER BY created_at DESC LIMIT 10;"

# 2. Check Hindsight database
sqlite3 ~/.cortex/hindsight.db "SELECT * FROM facts ORDER BY created_at DESC LIMIT 10;"

# 3. Look for storage errors
grep -i "store\|save\|insert" ~/.cortex/logs/cortex.log | grep -i error

# 4. Test manual storage
curl -X POST http://localhost:8080/acp/capabilities/memory-store/invoke \
  -d '{"fact": "User prefers dark mode", "category": "preference", "confidence": 0.9}'
```

**Common Fixes:**

```go
// cortex/lobes/temporal/learning.go

// Issue: Facts not being extracted and stored
// Fix: Implement proper fact extraction pipeline

type FactExtractor struct {
    llm          llm.Client
    store        *knowledge.Store
    hindsight    *hindsight.Client
}

type ExtractedFact struct {
    Content    string
    Category   string   // preference, fact, opinion, relationship
    Confidence float64
    Source     string
    Timestamp  time.Time
}

func (f *FactExtractor) ExtractAndStore(ctx context.Context, conversation []Message) error {
    // Only process if there's meaningful content
    if len(conversation) < 2 {
        return nil
    }
    
    // Build extraction prompt
    prompt := f.buildExtractionPrompt(conversation)
    
    // Call LLM to extract facts
    response, err := f.llm.Complete(ctx, llm.Request{
        Prompt:      prompt,
        Model:       "llama3.2",
        Temperature: 0.3, // Low for consistent extraction
        MaxTokens:   500,
    })
    
    if err != nil {
        return fmt.Errorf("fact extraction failed: %w", err)
    }
    
    // Parse extracted facts
    facts, err := f.parseFacts(response.Content)
    if err != nil {
        log.Printf("Warning: fact parsing failed: %v", err)
        return nil
    }
    
    // Store each fact
    for _, fact := range facts {
        if fact.Confidence < 0.5 {
            continue // Skip low confidence facts
        }
        
        // Check for duplicates
        if f.isDuplicate(ctx, fact) {
            continue
        }
        
        // Store in appropriate system
        if err := f.storeFact(ctx, fact); err != nil {
            log.Printf("Warning: failed to store fact: %v", err)
        }
    }
    
    return nil
}

func (f *FactExtractor) buildExtractionPrompt(conversation []Message) string {
    var sb strings.Builder
    
    sb.WriteString(`Extract factual information about the user from this conversation.

For each fact, provide:
- fact: The information itself
- category: preference | fact | opinion | relationship
- confidence: 0.0-1.0

Only extract explicit, clear information. Do not infer or guess.

Conversation:
`)
    
    for _, msg := range conversation {
        sb.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
    }
    
    sb.WriteString(`
Respond in JSON format:
{"facts": [{"fact": "...", "category": "...", "confidence": 0.9}]}
`)
    
    return sb.String()
}

func (f *FactExtractor) storeFact(ctx context.Context, fact ExtractedFact) error {
    // Generate embedding
    embedding, err := f.generateEmbedding(fact.Content)
    if err != nil {
        return err
    }
    
    // Store in Knowledge Fabric
    item := knowledge.Item{
        Type:      "user_fact",
        Content:   fact.Content,
        Embedding: embedding,
        Metadata: map[string]interface{}{
            "category":   fact.Category,
            "confidence": fact.Confidence,
            "source":     fact.Source,
        },
        CreatedAt: time.Now(),
    }
    
    if err := f.store.Add(ctx, item); err != nil {
        return fmt.Errorf("knowledge store failed: %w", err)
    }
    
    // Also store in Hindsight for declarative memory
    hindsightFact := hindsight.Fact{
        Content:    fact.Content,
        Category:   fact.Category,
        Confidence: fact.Confidence,
        Timestamp:  time.Now(),
    }
    
    if err := f.hindsight.StoreFact(ctx, hindsightFact); err != nil {
        log.Printf("Warning: hindsight store failed: %v", err)
    }
    
    return nil
}
```

### 8.2 Learning Disabled

**Diagnostic Steps:**

```bash
# 1. Check learning configuration
cat ~/.cortex/config/cortex.yaml | grep -A10 "learning:"

# 2. Check if learning is enabled
curl http://localhost:8080/debug/config | jq '.learning'

# 3. Look for learning activity
grep -i "learn\|lesson\|extract" ~/.cortex/logs/cortex.log | tail -20

# 4. Check dreamer/consolidation status
curl http://localhost:8080/debug/temporal | jq '.dreamer_status'
```

**Common Fixes:**

```go
// cortex/lobes/temporal/dreamer.go

// Issue: Learning not happening
// Fix: Ensure dreamer is running and processing

type Dreamer struct {
    procedural   *knowledge.Store
    declarative  *hindsight.Client
    llm          llm.Client
    
    isActive     atomic.Bool
    lastRun      time.Time
    idleThresh   time.Duration
    
    ticker       *time.Ticker
    stopChan     chan struct{}
}

func NewDreamer(proc *knowledge.Store, decl *hindsight.Client, llm llm.Client) *Dreamer {
    d := &Dreamer{
        procedural:  proc,
        declarative: decl,
        llm:         llm,
        idleThresh:  5 * time.Minute,
        stopChan:    make(chan struct{}),
    }
    
    return d
}

func (d *Dreamer) Start() {
    d.ticker = time.NewTicker(1 * time.Minute)
    
    go func() {
        for {
            select {
            case <-d.stopChan:
                return
            case <-d.ticker.C:
                d.maybeConsolidate()
            }
        }
    }()
}

func (d *Dreamer) maybeConsolidate() {
    // Check if system is idle
    if !d.isSystemIdle() {
        return
    }
    
    // Don't run too frequently
    if time.Since(d.lastRun) < 30*time.Minute {
        return
    }
    
    d.isActive.Store(true)
    defer d.isActive.Store(false)
    
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()
    
    log.Println("Starting memory consolidation...")
    
    // 1. Consolidate recent experiences into lessons
    if err := d.consolidateLessons(ctx); err != nil {
        log.Printf("Lesson consolidation failed: %v", err)
    }
    
    // 2. Prune low-value memories
    if err := d.pruneMemories(ctx); err != nil {
        log.Printf("Memory pruning failed: %v", err)
    }
    
    // 3. Strengthen frequently-accessed memories
    if err := d.strengthenMemories(ctx); err != nil {
        log.Printf("Memory strengthening failed: %v", err)
    }
    
    d.lastRun = time.Now()
    log.Println("Memory consolidation complete")
}

func (d *Dreamer) consolidateLessons(ctx context.Context) error {
    // Get recent experiences that haven't been processed
    experiences, err := d.declarative.GetUnprocessedExperiences(ctx, 50)
    if err != nil {
        return err
    }
    
    if len(experiences) < 5 {
        return nil // Not enough to consolidate
    }
    
    // Group by topic/pattern
    groups := d.groupExperiences(experiences)
    
    for topic, group := range groups {
        if len(group) < 3 {
            continue
        }
        
        // Generate lesson from group
        lesson, err := d.generateLesson(ctx, topic, group)
        if err != nil {
            log.Printf("Lesson generation failed for %s: %v", topic, err)
            continue
        }
        
        // Store lesson
        if err := d.storeLesson(ctx, lesson); err != nil {
            log.Printf("Lesson storage failed: %v", err)
            continue
        }
        
        // Mark experiences as processed
        for _, exp := range group {
            d.declarative.MarkProcessed(ctx, exp.ID)
        }
    }
    
    return nil
}
```

### 8.3 Wrong Memories Retrieved

**Diagnostic Steps:**

```bash
# 1. Test memory search directly
curl -X POST http://localhost:8080/acp/capabilities/memory-query/invoke \
  -d '{"query": "user preferences"}' | jq '.data.memories'

# 2. Check embedding similarity
sqlite3 ~/.cortex/knowledge.db "
  SELECT content, 
         (SELECT content FROM knowledge_items WHERE id = 'test') as query,
         embedding 
  FROM knowledge_items 
  WHERE type = 'user_fact' 
  LIMIT 5;
"

# 3. Verify embedding model
curl http://localhost:11434/api/embeddings \
  -d '{"model": "nomic-embed-text", "prompt": "test query"}'
```

**Common Fixes:**

```go
// cortex/lobes/temporal/search.go

// Issue: Irrelevant memories returned
// Fix: Improve search with hybrid approach

type MemorySearch struct {
    store         *knowledge.Store
    embedder      Embedder
    reranker      *Reranker
}

type SearchResult struct {
    Item       knowledge.Item
    Score      float64
    MatchType  string // "semantic", "keyword", "exact"
}

func (s *MemorySearch) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
    var results []SearchResult
    
    // 1. Semantic search (embedding similarity)
    semanticResults, err := s.semanticSearch(ctx, query, opts.Limit*2)
    if err != nil {
        log.Printf("Semantic search failed: %v", err)
    } else {
        for _, r := range semanticResults {
            results = append(results, SearchResult{
                Item:      r.Item,
                Score:     r.Score,
                MatchType: "semantic",
            })
        }
    }
    
    // 2. Keyword search (FTS)
    keywordResults, err := s.keywordSearch(ctx, query, opts.Limit)
    if err != nil {
        log.Printf("Keyword search failed: %v", err)
    } else {
        for _, r := range keywordResults {
            // Check if already in results
            if !s.containsItem(results, r.Item.ID) {
                results = append(results, SearchResult{
                    Item:      r.Item,
                    Score:     r.Score * 0.8, // Slightly lower weight
                    MatchType: "keyword",
                })
            }
        }
    }
    
    // 3. Exact match (for specific queries like names)
    if isSpecificQuery(query) {
        exactResults, err := s.exactSearch(ctx, query)
        if err == nil {
            for _, r := range exactResults {
                if !s.containsItem(results, r.Item.ID) {
                    results = append(results, SearchResult{
                        Item:      r.Item,
                        Score:     1.0, // Highest score for exact
                        MatchType: "exact",
                    })
                }
            }
        }
    }
    
    // 4. Rerank results
    if s.reranker != nil && len(results) > 1 {
        results, err = s.reranker.Rerank(ctx, query, results)
        if err != nil {
            log.Printf("Reranking failed: %v", err)
        }
    }
    
    // 5. Sort by score and limit
    sort.Slice(results, func(i, j int) bool {
        return results[i].Score > results[j].Score
    })
    
    if len(results) > opts.Limit {
        results = results[:opts.Limit]
    }
    
    // 6. Filter by minimum score
    filtered := make([]SearchResult, 0, len(results))
    for _, r := range results {
        if r.Score >= opts.MinScore {
            filtered = append(filtered, r)
        }
    }
    
    return filtered, nil
}

func (s *MemorySearch) semanticSearch(ctx context.Context, query string, limit int) ([]SearchResult, error) {
    // Generate query embedding
    embedding, err := s.embedder.Embed(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("embedding failed: %w", err)
    }
    
    // Search by cosine similarity
    items, err := s.store.SearchByEmbedding(ctx, embedding, limit)
    if err != nil {
        return nil, err
    }
    
    results := make([]SearchResult, len(items))
    for i, item := range items {
        results[i] = SearchResult{
            Item:  item.Item,
            Score: item.Similarity,
        }
    }
    
    return results, nil
}
```

### 8.4 Database Issues

**Diagnostic Steps:**

```bash
# 1. Check database integrity
sqlite3 ~/.cortex/knowledge.db "PRAGMA integrity_check;"
sqlite3 ~/.cortex/hindsight.db "PRAGMA integrity_check;"

# 2. Check database size and table counts
sqlite3 ~/.cortex/knowledge.db "
  SELECT 'knowledge_items' as table_name, COUNT(*) as count FROM knowledge_items
  UNION ALL
  SELECT 'embeddings', COUNT(*) FROM embeddings;
"

# 3. Check for lock issues
lsof ~/.cortex/*.db

# 4. Check write permissions
ls -la ~/.cortex/
touch ~/.cortex/test_write && rm ~/.cortex/test_write && echo "Write OK"
```

**Common Fixes:**

```go
// cortex/knowledge/store.go

// Issue: Database errors
// Fix: Robust database handling with WAL and proper locking

type Store struct {
    db        *sql.DB
    dbPath    string
    writeMu   sync.Mutex
    embedder  Embedder
}

func NewStore(dbPath string) (*Store, error) {
    // Ensure directory exists
    dir := filepath.Dir(dbPath)
    if err := os.MkdirAll(dir, 0755); err != nil {
        return nil, fmt.Errorf("failed to create db directory: %w", err)
    }
    
    // Open with WAL mode for better concurrency
    db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON")
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %w", err)
    }
    
    // Set connection pool settings
    db.SetMaxOpenConns(1)  // SQLite is single-writer
    db.SetMaxIdleConns(1)
    db.SetConnMaxLifetime(0)
    
    // Initialize schema
    if err := initSchema(db); err != nil {
        db.Close()
        return nil, fmt.Errorf("schema initialization failed: %w", err)
    }
    
    // Verify database is writable
    if _, err := db.Exec("INSERT INTO _health_check (ts) VALUES (?)", time.Now().Unix()); err != nil {
        db.Close()
        return nil, fmt.Errorf("database write test failed: %w", err)
    }
    
    return &Store{
        db:     db,
        dbPath: dbPath,
    }, nil
}

func initSchema(db *sql.DB) error {
    schema := `
        CREATE TABLE IF NOT EXISTS knowledge_items (
            id TEXT PRIMARY KEY,
            type TEXT NOT NULL,
            content TEXT NOT NULL,
            metadata TEXT,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
        );
        
        CREATE INDEX IF NOT EXISTS idx_knowledge_type ON knowledge_items(type);
        CREATE INDEX IF NOT EXISTS idx_knowledge_created ON knowledge_items(created_at);
        
        CREATE TABLE IF NOT EXISTS embeddings (
            item_id TEXT PRIMARY KEY REFERENCES knowledge_items(id) ON DELETE CASCADE,
            embedding BLOB NOT NULL
        );
        
        CREATE VIRTUAL TABLE IF NOT EXISTS knowledge_fts USING fts5(
            content,
            content_rowid='rowid'
        );
        
        CREATE TABLE IF NOT EXISTS _health_check (
            id INTEGER PRIMARY KEY,
            ts INTEGER
        );
    `
    
    _, err := db.Exec(schema)
    return err
}

func (s *Store) Add(ctx context.Context, item Item) error {
    s.writeMu.Lock()
    defer s.writeMu.Unlock()
    
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("begin transaction failed: %w", err)
    }
    defer tx.Rollback()
    
    // Generate ID if not set
    if item.ID == "" {
        item.ID = uuid.New().String()
    }
    
    // Serialize metadata
    metadataJSON, err := json.Marshal(item.Metadata)
    if err != nil {
        return fmt.Errorf("metadata serialization failed: %w", err)
    }
    
    // Insert item
    _, err = tx.ExecContext(ctx, `
        INSERT INTO knowledge_items (id, type, content, metadata, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?)
    `, item.ID, item.Type, item.Content, metadataJSON, time.Now(), time.Now())
    
    if err != nil {
        return fmt.Errorf("item insert failed: %w", err)
    }
    
    // Generate and store embedding
    if s.embedder != nil && item.Embedding == nil {
        embedding, err := s.embedder.Embed(ctx, item.Content)
        if err != nil {
            log.Printf("Warning: embedding generation failed: %v", err)
        } else {
            item.Embedding = embedding
        }
    }
    
    if item.Embedding != nil {
        embeddingBlob := s.serializeEmbedding(item.Embedding)
        _, err = tx.ExecContext(ctx, `
            INSERT INTO embeddings (item_id, embedding)
            VALUES (?, ?)
        `, item.ID, embeddingBlob)
        
        if err != nil {
            return fmt.Errorf("embedding insert failed: %w", err)
        }
    }
    
    // Update FTS
    _, err = tx.ExecContext(ctx, `
        INSERT INTO knowledge_fts (rowid, content)
        VALUES ((SELECT rowid FROM knowledge_items WHERE id = ?), ?)
    `, item.ID, item.Content)
    
    if err != nil {
        log.Printf("Warning: FTS update failed: %v", err)
    }
    
    return tx.Commit()
}
```

---

## 9. Context Retrieval Issues

### Symptom Categories

| Symptom | Likely Cause | Section |
|---------|--------------|---------|
| Missing relevant context | Search failure | 9.1 |
| Too much irrelevant context | Filtering issue | 9.2 |
| Slow context building | Performance | 9.3 |

### 9.1 Context Not Retrieved

**Diagnostic Steps:**

```bash
# 1. Test context building directly
curl -X POST http://localhost:8080/debug/context \
  -d '{"query": "What do you know about me?"}' | jq '.'

# 2. Check context sources
curl http://localhost:8080/debug/context/sources | jq '.'

# 3. Verify embeddings exist
sqlite3 ~/.cortex/knowledge.db "SELECT COUNT(*) FROM embeddings;"
```

**Common Fixes:**

```go
// cortex/lobes/parietal/context_builder.go

// Issue: Context not including relevant information
// Fix: Comprehensive context building with fallbacks

type ContextBuilder struct {
    temporal     *TemporalLobe
    knowledge    *knowledge.Store
    userModel    *UserModel
    maxTokens    int
}

type BuiltContext struct {
    SystemPrompt    string
    UserContext     string
    RelevantMemories []Memory
    ConvHistory     []Message
    TotalTokens     int
}

func (c *ContextBuilder) Build(ctx context.Context, query string, session *Session) (*BuiltContext, error) {
    built := &BuiltContext{}
    tokenBudget := c.maxTokens
    
    // 1. Get relevant memories (highest priority)
    memories, err := c.retrieveRelevantMemories(ctx, query, session)
    if err != nil {
        log.Printf("Warning: memory retrieval failed: %v", err)
        // Continue without memories
    } else {
        built.RelevantMemories = memories
        tokenBudget -= c.estimateTokens(memories)
    }
    
    // 2. Get user context
    userCtx, err := c.getUserContext(ctx, session)
    if err != nil {
        log.Printf("Warning: user context failed: %v", err)
    } else {
        built.UserContext = userCtx
        tokenBudget -= c.estimateTokens(userCtx)
    }
    
    // 3. Include conversation history (fit what we can)
    if len(session.ConvHistory) > 0 {
        historyTokens := tokenBudget / 2 // Reserve half for history
        built.ConvHistory = c.fitHistory(session.ConvHistory, historyTokens)
    }
    
    // 4. Build final context string
    built.SystemPrompt = c.assemblePrompt(built)
    built.TotalTokens = c.estimateTokens(built.SystemPrompt)
    
    return built, nil
}

func (c *ContextBuilder) retrieveRelevantMemories(ctx context.Context, query string, session *Session) ([]Memory, error) {
    var memories []Memory
    
    // Search procedural memory (lessons/patterns)
    lessons, err := c.temporal.QueryLessons(ctx, query, 5)
    if err == nil {
        for _, l := range lessons {
            memories = append(memories, Memory{
                Type:    "lesson",
                Content: l.Content,
                Score:   l.Score,
            })
        }
    }
    
    // Search declarative memory (facts about user)
    facts, err := c.temporal.QueryFacts(ctx, query, 5)
    if err == nil {
        for _, f := range facts {
            memories = append(memories, Memory{
                Type:    "fact",
                Content: f.Content,
                Score:   f.Score,
            })
        }
    }
    
    // Include session-specific context
    if sessionCtx := session.ContextCache.Get(query); sessionCtx != nil {
        memories = append(memories, Memory{
            Type:    "session_context",
            Content: sessionCtx.Content,
            Score:   1.0, // Session context is always relevant
        })
    }
    
    // Sort by score
    sort.Slice(memories, func(i, j int) bool {
        return memories[i].Score > memories[j].Score
    })
    
    return memories, nil
}

func (c *ContextBuilder) getUserContext(ctx context.Context, session *Session) (string, error) {
    var parts []string
    
    // User preferences
    if prefs := c.userModel.GetPreferences(); len(prefs) > 0 {
        parts = append(parts, "User preferences: "+strings.Join(prefs, ", "))
    }
    
    // User expertise
    if exp := c.userModel.GetExpertise(); len(exp) > 0 {
        expStr := make([]string, 0, len(exp))
        for domain, level := range exp {
            expStr = append(expStr, fmt.Sprintf("%s: %s", domain, level))
        }
        parts = append(parts, "Expertise: "+strings.Join(expStr, ", "))
    }
    
    // Current state
    state := c.userModel.GetCurrentState()
    if state.Frustration > 0.5 {
        parts = append(parts, "User appears frustrated")
    }
    if state.Urgency > 0.7 {
        parts = append(parts, "User seems to be in a hurry")
    }
    
    // Known facts about user
    knownFacts, err := c.temporal.GetUserFacts(ctx, 10)
    if err == nil && len(knownFacts) > 0 {
        factStrs := make([]string, len(knownFacts))
        for i, f := range knownFacts {
            factStrs[i] = f.Content
        }
        parts = append(parts, "Known about user: "+strings.Join(factStrs, "; "))
    }
    
    return strings.Join(parts, "\n"), nil
}
```

---

## 10. Bi-directional Communication Issues

### Symptom Categories

| Symptom | Likely Cause | Section |
|---------|--------------|---------|
| One-way only | Channel setup | 10.1 |
| Barge-in not working | Interrupt handling | 10.2 |
| Echo/feedback | Audio routing | 10.3 |

### 10.1 Channel Setup Issues

**Diagnostic Steps:**

```bash
# 1. Check both channels are open
curl http://localhost:8080/debug/channels | jq '.'

# 2. Test bidirectional flow
# Terminal 1: Listen for responses
curl -N http://localhost:8080/a2a/tasks/test/events &

# Terminal 2: Send message
curl -X POST http://localhost:8080/a2a/tasks \
  -d '{"id": "test", "message": {"role": "user", "parts": [{"text": "ping"}]}}'

# 3. Check EventBus routing
grep -i "route\|channel\|bus" ~/.cortex/logs/cortex.log | tail -20
```

**Common Fixes:**

```go
// avatar/comm/bidirectional.go

// Issue: Communication only flows one way
// Fix: Proper bidirectional channel setup

type BidirectionalComm struct {
    // Inbound: Avatar -> Cortex
    inbound   chan Message
    
    // Outbound: Cortex -> Avatar
    outbound  chan Message
    
    // Event subscriptions
    subs      map[string]chan Event
    subsMu    sync.RWMutex
    
    // Connection state
    connected atomic.Bool
}

func NewBidirectionalComm() *BidirectionalComm {
    bc := &BidirectionalComm{
        inbound:  make(chan Message, 100),
        outbound: make(chan Message, 100),
        subs:     make(map[string]chan Event),
    }
    
    // Start router
    go bc.routeMessages()
    
    return bc
}

func (bc *BidirectionalComm) routeMessages() {
    for {
        select {
        case msg := <-bc.outbound:
            // Route to all subscribers
            bc.subsMu.RLock()
            for _, sub := range bc.subs {
                select {
                case sub <- Event{Type: "message", Data: msg}:
                default:
                    // Subscriber buffer full, skip
                }
            }
            bc.subsMu.RUnlock()
        }
    }
}

func (bc *BidirectionalComm) Subscribe(id string) <-chan Event {
    ch := make(chan Event, 50)
    
    bc.subsMu.Lock()
    bc.subs[id] = ch
    bc.subsMu.Unlock()
    
    return ch
}

func (bc *BidirectionalComm) Unsubscribe(id string) {
    bc.subsMu.Lock()
    if ch, ok := bc.subs[id]; ok {
        close(ch)
        delete(bc.subs, id)
    }
    bc.subsMu.Unlock()
}

func (bc *BidirectionalComm) SendToCortex(msg Message) error {
    select {
    case bc.inbound <- msg:
        return nil
    default:
        return fmt.Errorf("inbound channel full")
    }
}

func (bc *BidirectionalComm) SendToAvatar(msg Message) error {
    select {
    case bc.outbound <- msg:
        return nil
    default:
        return fmt.Errorf("outbound channel full")
    }
}
```

### 10.2 Barge-in Issues

**Diagnostic Steps:**

```bash
# 1. Check interrupt handling config
cat ~/.cortex/config/avatar.yaml | grep -A10 "barge_in:"

# 2. Test interrupt signal
# While Cortex is speaking, send interrupt
curl -X POST http://localhost:8080/avatar/interrupt

# 3. Check interrupt logs
grep -i "interrupt\|barge\|cancel" ~/.cortex/logs/avatar.log
```

**Common Fixes:**

```go
// avatar/audio/barge_in.go

// Issue: User can't interrupt Cortex while speaking
// Fix: Implement proper barge-in detection

type BargeInDetector struct {
    vad            *AdaptiveVAD
    speaker        *Speaker
    threshold      float64
    debounceTime   time.Duration
    
    lastBargeIn    time.Time
    interruptChan  chan struct{}
}

func NewBargeInDetector(vad *AdaptiveVAD, speaker *Speaker) *BargeInDetector {
    return &BargeInDetector{
        vad:           vad,
        speaker:       speaker,
        threshold:     0.6,
        debounceTime:  500 * time.Millisecond,
        interruptChan: make(chan struct{}, 1),
    }
}

func (b *BargeInDetector) ProcessAudio(audio []byte) {
    // Only check for barge-in if we're currently speaking
    if !b.speaker.IsPlaying() {
        return
    }
    
    // Check if user is speaking
    vadResult := b.vad.ProcessFrame(audio)
    
    if vadResult.IsSpeech && vadResult.Confidence > b.threshold {
        // Debounce
        if time.Since(b.lastBargeIn) < b.debounceTime {
            return
        }
        
        b.lastBargeIn = time.Now()
        
        // Signal interrupt
        select {
        case b.interruptChan <- struct{}{}:
            log.Println("Barge-in detected, interrupting playback")
        default:
            // Already signaled
        }
    }
}

func (b *BargeInDetector) InterruptChannel() <-chan struct{} {
    return b.interruptChan
}

// Integration in conversation pipeline
func (p *ConversationPipeline) speakWithBargeIn(ctx context.Context, text string) error {
    // Start speaking
    speakCtx, cancel := context.WithCancel(ctx)
    defer cancel()
    
    // Start TTS in background
    speakDone := make(chan error, 1)
    go func() {
        speakDone <- p.tts.SpeakStreaming(speakCtx, text)
    }()
    
    // Monitor for barge-in
    for {
        select {
        case <-p.bargeIn.InterruptChannel():
            // User interrupted
            cancel() // Stop speaking
            p.speaker.Stop()
            
            // Clear any buffered audio
            p.speaker.Flush()
            
            return ErrBargeIn
            
        case err := <-speakDone:
            return err
            
        case <-ctx.Done():
            return ctx.Err()
        }
    }
}
```

### 10.3 Audio Echo/Feedback Issues

**Diagnostic Steps:**

```bash
# 1. Check for echo cancellation
cat ~/.cortex/config/avatar.yaml | grep -A5 "echo_cancellation:"

# 2. Test audio isolation
# Record while playing back
rec -c 1 -r 16000 echo_test.wav &
play some_audio.wav
# Analyze echo_test.wav for feedback

# 3. Check audio routing
# macOS
system_profiler SPAudioDataType
```

**Common Fixes:**

```go
// avatar/audio/echo_cancel.go

// Issue: Audio feedback loop
// Fix: Implement echo cancellation / suppression

type EchoCanceller struct {
    // Reference buffer (what we're playing)
    referenceBuffer *RingBuffer[float32]
    
    // Input buffer (what we're recording)
    inputBuffer     *RingBuffer[float32]
    
    // Adaptive filter
    filter          *AdaptiveFilter
    
    // Playback state
    isPlaying       atomic.Bool
    playbackDelay   time.Duration
}

func NewEchoCanceller(sampleRate int, filterLength int) *EchoCanceller {
    return &EchoCanceller{
        referenceBuffer: NewRingBuffer[float32](sampleRate * 2), // 2 second buffer
        inputBuffer:     NewRingBuffer[float32](sampleRate * 2),
        filter:          NewAdaptiveFilter(filterLength),
        playbackDelay:   50 * time.Millisecond, // Typical system delay
    }
}

// Called when we output audio
func (e *EchoCanceller) FeedReference(samples []float32) {
    e.isPlaying.Store(true)
    e.referenceBuffer.Write(samples)
}

// Called when we receive input audio
func (e *EchoCanceller) Process(input []float32) []float32 {
    // If not playing, pass through
    if !e.isPlaying.Load() {
        return input
    }
    
    // Get reference signal with appropriate delay
    delayedSamples := int(float64(e.playbackDelay) / float64(time.Second) * 16000)
    reference := e.referenceBuffer.ReadDelayed(len(input), delayedSamples)
    
    if len(reference) == 0 {
        return input
    }
    
    // Apply adaptive filter to estimate echo
    echoEstimate := e.filter.Process(reference)
    
    // Subtract echo estimate from input
    output := make([]float32, len(input))
    for i := range input {
        if i < len(echoEstimate) {
            output[i] = input[i] - echoEstimate[i]
        } else {
            output[i] = input[i]
        }
    }
    
    // Update filter based on residual
    e.filter.Update(reference, output)
    
    return output
}

// Simpler approach: Gate input while playing
type SimpleEchoSuppressor struct {
    isPlaying    atomic.Bool
    gateDelay    time.Duration
    playbackEnd  time.Time
    mu           sync.Mutex
}

func (s *SimpleEchoSuppressor) StartPlayback() {
    s.isPlaying.Store(true)
}

func (s *SimpleEchoSuppressor) StopPlayback() {
    s.mu.Lock()
    s.isPlaying.Store(false)
    s.playbackEnd = time.Now()
    s.mu.Unlock()
}

func (s *SimpleEchoSuppressor) ShouldCapture() bool {
    if s.isPlaying.Load() {
        return false // Don't capture while playing
    }
    
    s.mu.Lock()
    defer s.mu.Unlock()
    
    // Add delay after playback stops
    if time.Since(s.playbackEnd) < s.gateDelay {
        return false
    }
    
    return true
}
```

---

## 11. Performance Issues

### Quick Performance Check

```bash
# Create performance diagnostic script
cat > cortex-perf.sh << 'EOF'
#!/bin/bash

echo "=== CORTEX PERFORMANCE DIAGNOSTICS ==="

echo ""
echo "▸ Latency Measurements"

# STT latency
echo -n "STT latency: "
time (curl -s -X POST http://localhost:11434/api/generate \
  -d '{"model": "whisper", "prompt": "test"}' > /dev/null) 2>&1 | grep real

# LLM latency
echo -n "LLM latency: "
time (curl -s -X POST http://localhost:11434/api/generate \
  -d '{"model": "llama3.2", "prompt": "Say hello"}' > /dev/null) 2>&1 | grep real

# TTS latency  
echo -n "TTS latency: "
time (curl -s -X POST http://localhost:5000/tts \
  -d '{"text": "Hello world"}' > /dev/null) 2>&1 | grep real

# Memory search latency
echo -n "Memory search: "
time (curl -s -X POST http://localhost:8080/acp/capabilities/memory-query/invoke \
  -d '{"query": "test"}' > /dev/null) 2>&1 | grep real

echo ""
echo "▸ Resource Usage"
ps aux | grep -E "cortex|ollama|kokoro" | awk '{print $11, "CPU:", $3"%", "MEM:", $4"%"}'

echo ""
echo "▸ Model Status"
curl -s http://localhost:11434/api/ps | jq '.models[] | {name: .name, size: .size}'

echo ""
echo "▸ Database Stats"
sqlite3 ~/.cortex/knowledge.db "SELECT 'Items:', COUNT(*) FROM knowledge_items;"
sqlite3 ~/.cortex/knowledge.db "SELECT 'DB Size:', page_count * page_size FROM pragma_page_count(), pragma_page_size();"
EOF
chmod +x cortex-perf.sh
```

---

## 12. Integration Test Scenarios

### Test Suite for Voice + Memory

```go
// test/integration/voice_memory_test.go

func TestVoiceMemoryIntegration(t *testing.T) {
    ctx := context.Background()
    
    // Setup
    avatar := setupTestAvatar(t)
    defer avatar.Cleanup()
    
    // Test 1: Store information via voice
    t.Run("StoreViaVoice", func(t *testing.T) {
        // Simulate voice input
        response, err := avatar.ProcessVoice(ctx, "My name is Alice and I prefer dark mode")
        require.NoError(t, err)
        
        // Check response acknowledges
        assert.Contains(t, response.Text, "Alice")
        
        // Wait for storage
        time.Sleep(2 * time.Second)
        
        // Verify stored
        memories, err := avatar.QueryMemory(ctx, "user name")
        require.NoError(t, err)
        assert.True(t, containsMemory(memories, "Alice"))
    })
    
    // Test 2: Recall information
    t.Run("RecallViaVoice", func(t *testing.T) {
        response, err := avatar.ProcessVoice(ctx, "What is my name?")
        require.NoError(t, err)
        
        assert.Contains(t, response.Text, "Alice")
    })
    
    // Test 3: Contextual question
    t.Run("ContextualQuestion", func(t *testing.T) {
        response, err := avatar.ProcessVoice(ctx, "What theme should you use when showing me things?")
        require.NoError(t, err)
        
        assert.Contains(t, strings.ToLower(response.Text), "dark")
    })
    
    // Test 4: Multi-turn with memory
    t.Run("MultiTurnWithMemory", func(t *testing.T) {
        sessionID := "test-session"
        
        // Turn 1
        _, err := avatar.ProcessVoiceWithSession(ctx, sessionID, "I'm working on a Go project called Cortex")
        require.NoError(t, err)
        
        // Turn 2 - should remember context
        response, err := avatar.ProcessVoiceWithSession(ctx, sessionID, "What language is it written in?")
        require.NoError(t, err)
        
        assert.Contains(t, strings.ToLower(response.Text), "go")
    })
    
    // Test 5: Barge-in
    t.Run("BargeIn", func(t *testing.T) {
        // Start long response
        done := make(chan struct{})
        go func() {
            defer close(done)
            avatar.ProcessVoice(ctx, "Tell me a long story about programming")
        }()
        
        // Interrupt after 1 second
        time.Sleep(1 * time.Second)
        err := avatar.Interrupt()
        require.NoError(t, err)
        
        // Verify interrupted
        select {
        case <-done:
            // Good, interrupted
        case <-time.After(2 * time.Second):
            t.Fatal("Barge-in did not interrupt")
        }
    })
}
```

---

## Quick Reference Card

### Most Common Issues & Quick Fixes

| Issue | Quick Fix |
|-------|-----------|
| No audio input | Check mic permissions, run `rec test.wav trim 0 3` |
| STT not responding | Restart Ollama: `ollama serve` |
| TTS silent | Check Kokoro: `docker restart kokoro-tts` |
| Memory not saving | Check DB write: `sqlite3 ~/.cortex/knowledge.db "PRAGMA integrity_check;"` |
| Slow responses | Check model switching: `curl localhost:11434/api/ps` |
| Lost context | Check session: `curl localhost:8080/debug/sessions` |
| A2A not working | Check agent card: `curl localhost:8080/.well-known/agent.json` |

### Emergency Recovery

```bash
# Full system restart
pkill -f cortex
pkill -f cortexavatar
docker restart kokoro-tts
ollama serve &
sleep 5
./cortex &
./cortexavatar &
```

---

**Document Version**: 1.0.0  
**Last Updated**: 2024-01-15  
**For**: Claude Code debugging CortexAvatar + Cortex integration
