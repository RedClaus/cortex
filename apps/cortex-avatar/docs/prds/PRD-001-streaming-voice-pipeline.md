---
project: Cortex
component: Docs
phase: Ideation
date_created: 2026-02-06T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T14:12:41.708523
---

# PRD-001: Streaming Voice Pipeline Integration

**Status:** 📋 Draft
**Priority:** P1 - High
**Owner:** TBD
**Created:** 2026-02-06
**Target Release:** v2.4.0

---

## Problem Statement

CortexAvatar currently suffers from critical voice interaction issues that degrade user experience:

1. **TTS Duplication Bug:** Audio responses play twice, causing confusion and poor UX
2. **Non-Streaming Audio:** Users must wait for complete audio generation before playback starts, creating perceived latency
3. **Limited Voice Features:** Missing modern voice interaction capabilities (VAD, optimized STT, streaming TTS)
4. **Slow Response Times:** Without dnet integration, LLM responses are slower than optimal

**Impact:**
- Poor user experience with voice interactions
- Unusable voice mode due to duplication bug
- Higher perceived latency (>3-5 seconds for responses)
- Falling behind modern voice AI expectations (ChatGPT Voice, etc.)

---

## Goals & Success Metrics

### Primary Goals

1. **Fix TTS Duplication Bug** - Eliminate audio playback issues
2. **Implement Streaming TTS** - Reduce perceived latency by 50%+
3. **Add Production-Ready Voice Pipeline** - VAD, STT, TTS with Mac optimization
4. **Maintain CortexBrain Integration** - Keep A2A protocol, leverage AutoLLM routing

### Success Metrics

| Metric | Current | Target | Measurement |
|--------|---------|--------|-------------|
| **End-to-End Latency** | >3 seconds | <2 seconds | Time from user speech end → audio start |
| **TTS Duplication Rate** | 100% (always duplicates) | 0% | Bug fixed |
| **User Satisfaction** | 3/10 (estimated) | 8/10 | User feedback survey |
| **Memory Usage** | ~2GB | <4GB | Activity Monitor during voice session |
| **Audio Quality** | 7/10 (XTTS) | ≥7/10 | Subjective audio quality assessment |
| **Uptime** | 95% | 99% | Service availability |

---

## User Stories

### Epic 1: Voice Input & Output

**As a** CortexAvatar user
**I want** to speak naturally to my AI assistant
**So that** I can interact hands-free and get immediate responses

**Acceptance Criteria:**
- Audio input is captured from microphone
- Voice Activity Detection filters silence
- Speech is converted to text with 95%+ accuracy
- Text is sent to CortexBrain via A2A
- Response audio starts playing within 2 seconds

---

### Epic 2: Streaming Audio Responses

**As a** CortexAvatar user
**I want** to hear responses as they're being generated
**So that** I don't have to wait for complete generation

**Acceptance Criteria:**
- Audio playback begins <500ms after first chunk ready
- No audio duplication
- Smooth, uninterrupted playback
- Can interrupt mid-response if needed

---

### Epic 3: Mac-Optimized Performance

**As a** Mac user
**I want** voice features optimized for Apple Silicon
**So that** I get fast responses without draining resources

**Acceptance Criteria:**
- Uses MPS acceleration where available
- Memory usage <4GB for voice pipeline
- CPU usage <50% during active voice session
- Battery impact minimal (<10% increase in drain rate)

---

## Technical Specifications

### Architecture

```
┌─────────────────────────────────────────────────────────┐
│              CortexAvatar (Wails v2)                     │
│              Go Backend + Svelte Frontend                │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  User Microphone Input                                  │
│      │                                                   │
│      ▼                                                   │
│  ┌────────────────────────────────────┐                 │
│  │  Audio Capture (Frontend)          │                 │
│  │  - WebAudio API                    │                 │
│  │  - Format: 16kHz, mono, WAV        │                 │
│  └───────────┬────────────────────────┘                 │
│              │                                           │
│              ▼                                           │
│  ┌────────────────────────────────────┐                 │
│  │  Go Backend Handler                │                 │
│  │  - Receive audio chunks            │                 │
│  │  - Forward to HF Service           │                 │
│  └───────────┬────────────────────────┘                 │
│              │                                           │
└──────────────┼──────────────────────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────────────────────┐
│         HF Speech Service (Docker/Python)                │
│         Port: 8899                                       │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  POST /vad                                              │
│  ├─► Silero VAD v5                                     │
│  └─► Returns: speech segments, silence filtered        │
│                                                          │
│  POST /stt                                              │
│  ├─► Lightning Whisper MLX (Mac optimized)             │
│  └─► Returns: transcribed text                         │
│                                                          │
│  POST /tts (streaming)                                  │
│  ├─► Parler-TTS mini v1 (English)                      │
│  ├─► MeloTTS (multilingual fallback)                   │
│  └─► Returns: audio stream (chunked)                   │
│                                                          │
└──────────────┬──────────────────────────────────────────┘
               │
               │ (Skip HF LLM, use existing A2A)
               │
               ▼
┌─────────────────────────────────────────────────────────┐
│              CortexBrain (Existing)                      │
│              Port: 8080 (A2A Protocol)                   │
├─────────────────────────────────────────────────────────┤
│  - AutoLLM routing (local + cloud)                      │
│  - 20 cognitive lobes processing                        │
│  - Context management                                   │
│  - Knowledge retrieval                                  │
└─────────────────────────────────────────────────────────┘
```

---

### Component Specifications

#### 1. HF Speech Service (New)

**Technology:** Python 3.11, FastAPI, Docker
**Dependencies:** transformers, torch, silero-vad, whisper, parler-tts
**Hardware:** Mac (MPS), CPU fallback

**Endpoints:**

```python
POST /vad
Content-Type: audio/wav
Response: {
  "segments": [
    {"start": 0.0, "end": 2.5, "confidence": 0.98},
    {"start": 3.0, "end": 5.2, "confidence": 0.95}
  ]
}

POST /stt
Content-Type: audio/wav
Response: {
  "text": "transcribed speech text",
  "language": "en",
  "confidence": 0.96
}

POST /tts
Content-Type: application/json
Body: {"text": "response text", "language": "en"}
Response: audio/wav (streaming, chunked transfer)
```

**Resource Requirements:**
- Memory: 2-4GB
- Disk: 8GB (models)
- CPU: 2-4 cores
- GPU: Optional (MPS on Mac)

---

#### 2. CortexAvatar Audio Handler (Modified)

**Technology:** Go 1.21+, Wails v2 bindings
**Location:** `internal/audio/`

**New Go Methods:**

```go
type AudioHandler struct {
    hfServiceURL string // http://localhost:8899
    a2aClient    *A2AClient
    audioQueue   chan []byte
}

// VoiceActivityDetection filters silence from audio
func (h *AudioHandler) VoiceActivityDetection(audioData []byte) ([]AudioSegment, error)

// SpeechToText converts audio to text via HF service
func (h *AudioHandler) SpeechToText(audioData []byte) (string, error)

// TextToSpeech converts text to audio (streaming)
func (h *AudioHandler) TextToSpeech(ctx context.Context, text string) (io.ReadCloser, error)

// HandleVoiceInput orchestrates full voice interaction
func (h *AudioHandler) HandleVoiceInput(ctx context.Context, audioData []byte) error {
    // 1. VAD: Filter silence
    segments, err := h.VoiceActivityDetection(audioData)

    // 2. STT: Convert to text
    text, err := h.SpeechToText(segments)

    // 3. A2A: Send to CortexBrain (existing code)
    response, err := h.a2aClient.SendMessage(ctx, text)

    // 4. TTS: Stream audio response
    audioStream, err := h.TextToSpeech(ctx, response.Text)

    // 5. Play audio (streaming)
    return h.PlayAudioStream(audioStream)
}
```

**Fallback Logic:**

```go
// If HF service unavailable, fallback to existing TTS
func (h *AudioHandler) TextToSpeechWithFallback(ctx context.Context, text string) (io.ReadCloser, error) {
    // Try HF service first
    stream, err := h.TextToSpeech(ctx, text)
    if err == nil {
        return stream, nil
    }

    // Fallback to existing XTTS (if available)
    log.Warn("HF service unavailable, falling back to XTTS")
    return h.legacyTTS.Generate(text)
}
```

---

#### 3. Frontend Audio Components (Modified)

**Technology:** Svelte, WebAudio API
**Location:** `frontend/src/lib/audio/`

**AudioCapture.svelte:**

```svelte
<script>
  import { onMount } from 'svelte';
  import { SendAudioToBackend } from '../../wailsjs/go/main/App';

  let mediaRecorder;
  let audioChunks = [];
  let isRecording = false;

  async function startRecording() {
    const stream = await navigator.mediaDevices.getUserMedia({
      audio: {
        sampleRate: 16000,
        channelCount: 1,
        echoCancellation: true,
        noiseSuppression: true
      }
    });

    mediaRecorder = new MediaRecorder(stream, {
      mimeType: 'audio/webm;codecs=opus'
    });

    mediaRecorder.ondataavailable = (event) => {
      audioChunks.push(event.data);
    };

    mediaRecorder.onstop = async () => {
      const audioBlob = new Blob(audioChunks, { type: 'audio/wav' });
      const arrayBuffer = await audioBlob.arrayBuffer();
      const uint8Array = new Uint8Array(arrayBuffer);

      // Send to Go backend
      await SendAudioToBackend(Array.from(uint8Array));
      audioChunks = [];
    };

    mediaRecorder.start();
    isRecording = true;
  }

  function stopRecording() {
    if (mediaRecorder && isRecording) {
      mediaRecorder.stop();
      isRecording = false;
    }
  }
</script>

<button on:click={isRecording ? stopRecording : startRecording}>
  {isRecording ? '🔴 Stop' : '🎤 Speak'}
</button>
```

**StreamingAudioPlayer.svelte:**

```svelte
<script>
  import { onMount } from 'svelte';

  export let audioStreamURL;

  let audioContext;
  let sourceNode;
  let isPlaying = false;

  onMount(() => {
    audioContext = new AudioContext({ sampleRate: 16000 });
  });

  async function playStreamingAudio(url) {
    const response = await fetch(url);
    const reader = response.body.getReader();

    isPlaying = true;

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      // Decode and play audio chunk
      const audioBuffer = await audioContext.decodeAudioData(value.buffer);
      const source = audioContext.createBufferSource();
      source.buffer = audioBuffer;
      source.connect(audioContext.destination);
      source.start();
    }

    isPlaying = false;
  }

  $: if (audioStreamURL) {
    playStreamingAudio(audioStreamURL);
  }
</script>

{#if isPlaying}
  <div class="audio-visualizer">
    <span>🔊 Speaking...</span>
  </div>
{/if}
```

---

### Data Flow

**Voice Input → Response (Full Cycle):**

```
1. User speaks into microphone
   │
   ├─► Frontend captures audio (WebAudio API)
   │
   ├─► Sends WAV bytes to Go backend
   │
   ├─► Go calls HF Service /vad
   │   └─► Silero VAD filters silence (50ms)
   │
   ├─► Go calls HF Service /stt
   │   └─► Lightning Whisper MLX transcribes (200-500ms)
   │
   ├─► Go sends text to CortexBrain A2A
   │   └─► AutoLLM processes request (500-2000ms)
   │
   ├─► Go receives response text
   │
   ├─► Go calls HF Service /tts (streaming)
   │   └─► Parler-TTS generates audio chunks (start <100ms)
   │
   └─► Frontend plays audio stream (immediate playback)

Total latency: <2 seconds (target)
Current latency: >3 seconds
```

---

## Implementation Phases

### Phase 0: Prerequisites (1 day)

**Goal:** Set up development environment

**Tasks:**
- [ ] Clone HF speech-to-speech repo locally
- [ ] Install Python 3.11 + uv package manager
- [ ] Install requirements: `uv pip install -r requirements_mac.txt`
- [ ] Download unidic: `python -m unidic download`
- [ ] Verify models download correctly

**Deliverables:**
- Working HF pipeline on local Mac
- Documentation of setup process

---

### Phase 1: Proof of Concept (3-5 days)

**Goal:** Validate HF pipeline performance on Mac

**Tasks:**
- [ ] Run basic HF pipeline test
- [ ] Measure end-to-end latency (target: <2s)
- [ ] Measure memory usage (target: <4GB)
- [ ] Test audio quality vs existing XTTS
- [ ] Test with different model sizes (tiny/base/small)
- [ ] Document findings in POC report

**Deliverables:**
- POC performance report
- Latency benchmarks
- Memory usage analysis
- Go/No-Go decision document

**Success Criteria:**
- End-to-end latency <2 seconds
- Memory usage <4GB
- Audio quality ≥ existing XTTS
- No blocking issues discovered

**Exit Criteria:**
- If latency >3 seconds, abort and explore alternatives
- If memory >6GB, try smaller models or abort
- If audio quality significantly worse, abort

---

### Phase 2: HTTP API Service (1 week)

**Goal:** Create containerized HF service with HTTP API

**Tasks:**
- [ ] Design FastAPI endpoints (/vad, /stt, /tts)
- [ ] Implement /vad endpoint (Silero VAD)
- [ ] Implement /stt endpoint (Lightning Whisper MLX)
- [ ] Implement /tts endpoint (Parler-TTS, streaming)
- [ ] Add health check endpoint
- [ ] Add metrics endpoint (latency, requests/sec)
- [ ] Write Dockerfile
- [ ] Write docker-compose.yml
- [ ] Test all endpoints with curl/Postman
- [ ] Document API in OpenAPI spec

**Deliverables:**
- `hf-voice-service/` directory
- FastAPI application
- Dockerfile + docker-compose.yml
- API documentation (OpenAPI/Swagger)
- Unit tests for each endpoint

**Success Criteria:**
- All endpoints return correct responses
- Service starts in <30 seconds
- Average response time <500ms per request
- Docker image size <2GB

---

### Phase 3: Go Client Integration (3-4 days)

**Goal:** Integrate HF service into CortexAvatar Go backend

**Tasks:**
- [ ] Create `internal/audio/hf_client.go`
- [ ] Implement VoiceActivityDetection() method
- [ ] Implement SpeechToText() method
- [ ] Implement TextToSpeech() method with streaming
- [ ] Add fallback logic (HF unavailable → legacy TTS)
- [ ] Add retry logic (3 attempts with exponential backoff)
- [ ] Add timeout handling (5s max per request)
- [ ] Write unit tests
- [ ] Write integration tests (require running HF service)

**Deliverables:**
- `internal/audio/hf_client.go`
- `internal/audio/hf_client_test.go`
- Test coverage >80%

**Success Criteria:**
- All Go tests pass
- Client handles HF service downtime gracefully
- Streaming TTS works end-to-end
- Fallback logic verified

---

### Phase 4: Frontend Audio Components (3-4 days)

**Goal:** Build Svelte components for voice interaction

**Tasks:**
- [ ] Create `AudioCapture.svelte` component
- [ ] Create `StreamingAudioPlayer.svelte` component
- [ ] Create `VoiceButton.svelte` (push-to-talk)
- [ ] Add WebAudio API integration
- [ ] Add audio visualization (waveform/levels)
- [ ] Handle browser permissions (microphone access)
- [ ] Add error handling (mic not available, etc.)
- [ ] Write component tests (Vitest)

**Deliverables:**
- 3 new Svelte components
- Component documentation
- Unit tests for each component

**Success Criteria:**
- Audio capture works in Chrome, Firefox, Safari
- Streaming playback works smoothly
- Visual feedback during recording/playback
- Graceful permission denial handling

---

### Phase 5: Bug Fixes & E2E Testing (1 week)

**Goal:** Fix TTS duplication bug, validate full pipeline

**Tasks:**
- [ ] Identify root cause of TTS duplication
- [ ] Remove duplicate audio playback code
- [ ] Implement single audio queue
- [ ] End-to-end test: speak → response → audio
- [ ] Test edge cases (interruption, long responses, errors)
- [ ] Test with dnet integration (fast LLM responses)
- [ ] Performance testing (10+ consecutive interactions)
- [ ] Memory leak testing (1-hour voice session)
- [ ] Fix any discovered bugs

**Deliverables:**
- TTS duplication bug fixed (verified)
- E2E test suite
- Performance test results
- Bug fixes documented

**Success Criteria:**
- Zero audio duplication in 20+ tests
- <2 second latency in 90% of interactions
- No memory leaks after 1-hour session
- All edge cases handled gracefully

---

### Phase 6: Documentation & Deployment (2-3 days)

**Goal:** Prepare for production deployment

**Tasks:**
- [ ] Write user documentation (how to use voice features)
- [ ] Write developer documentation (architecture, APIs)
- [ ] Create deployment guide (Docker setup)
- [ ] Write troubleshooting guide
- [ ] Update README.md
- [ ] Create demo video
- [ ] Prepare release notes (v2.4.0)

**Deliverables:**
- `docs/voice-features.md`
- `docs/deployment.md`
- `docs/troubleshooting.md`
- Demo video (2-3 minutes)
- Release notes

**Success Criteria:**
- Documentation clear enough for new contributors
- Deployment guide works on fresh Mac
- Demo video showcases key features

---

## Dependencies

### Internal Dependencies

| Dependency | Status | Blocker? | Notes |
|------------|--------|----------|-------|
| CortexBrain A2A API | ✅ Available | No | Already integrated, stable |
| dnet Integration | ⚠️ Pending | No | Nice-to-have, not required for v1 |
| Wails v2 | ✅ Available | No | Current version stable |

### External Dependencies

| Dependency | Version | License | Criticality |
|------------|---------|---------|-------------|
| Hugging Face speech-to-speech | latest | Apache 2.0 | Critical |
| Python | 3.11+ | PSF | Critical |
| FastAPI | 0.104+ | MIT | Critical |
| Docker | 24.0+ | Apache 2.0 | High |
| Whisper | latest | MIT | High |
| Parler-TTS | latest | Apache 2.0 | High |
| Silero VAD | v5 | MIT | Medium |

---

## Risks & Mitigations

### High Risks

**Risk 1: HF Service Memory Usage Exceeds 4GB**

**Impact:** Service crashes or slows down Mac
**Likelihood:** Medium
**Mitigation:**
- Use smaller Whisper models (tiny/base instead of large)
- Profile memory usage during POC
- Set Docker memory limits (4GB max)
- Add memory monitoring/alerts

---

**Risk 2: Latency Target Not Met (<2s)**

**Impact:** Poor user experience, feature not usable
**Likelihood:** Low
**Mitigation:**
- Benchmark during POC phase
- Use Mac-optimized models (Lightning Whisper MLX)
- Enable MPS acceleration
- Stream TTS to reduce perceived latency
- If target missed, re-evaluate model choices

---

**Risk 3: Docker Service Adds Deployment Complexity**

**Impact:** Harder to deploy, more moving parts
**Likelihood:** Medium
**Mitigation:**
- Provide one-command deployment (docker-compose up)
- Include auto-start scripts
- Add health checks and auto-restart
- Document troubleshooting steps

---

### Medium Risks

**Risk 4: Python Dependency Conflicts**

**Impact:** Service fails to start
**Likelihood:** Low
**Mitigation:**
- Use Docker for isolation
- Pin all dependency versions
- Test on clean Mac environment

---

**Risk 5: Audio Quality Degradation**

**Impact:** Users prefer old TTS
**Likelihood:** Low
**Mitigation:**
- A/B test during POC
- Keep fallback to existing TTS
- Allow users to choose TTS engine

---

**Risk 6: Multilingual Support Limited**

**Impact:** Non-English users can't use voice
**Likelihood:** Medium
**Mitigation:**
- Use MeloTTS for multilingual (6 languages)
- Document language limitations
- Wait for Parler-TTS multilingual release

---

## Open Questions

1. **Model Size Trade-offs:** Which Whisper variant (tiny/base/small) provides best speed/accuracy balance for our use case?
   - **Decision needed by:** End of POC phase
   - **Impact:** Memory usage, latency, accuracy

2. **Deployment Strategy:** Should HF service run on same machine as CortexAvatar or separate server?
   - **Decision needed by:** Phase 2
   - **Impact:** Architecture, latency, resource usage

3. **Fallback Strategy:** If HF service fails, should we fallback to existing TTS or fail gracefully?
   - **Decision needed by:** Phase 3
   - **Impact:** User experience, code complexity

4. **Voice Activation:** Should we support wake word ("Hey Cortex") or stick with push-to-talk?
   - **Decision needed by:** Phase 4 planning
   - **Impact:** Feature scope, timeline (+1-2 weeks)

5. **Multi-language Support:** Which languages should we prioritize after English?
   - **Decision needed by:** Phase 6
   - **Impact:** Model selection, testing effort

---

## Success Criteria (Final)

**Must Have (MVP):**
- ✅ TTS duplication bug fixed (100% of tests)
- ✅ End-to-end latency <2 seconds (90% of interactions)
- ✅ Memory usage <4GB during voice session
- ✅ Audio quality ≥ existing XTTS
- ✅ Works on Mac (primary platform)
- ✅ Graceful fallback if HF service unavailable
- ✅ Documentation complete

**Should Have:**
- ✅ Streaming TTS (immediate audio playback)
- ✅ VAD (silence filtering)
- ✅ Mac-optimized models (Lightning Whisper MLX)
- ✅ Docker containerization
- ✅ Health checks and monitoring

**Nice to Have:**
- Wake word activation
- Multi-language support (6+ languages)
- Voice cloning (custom TTS voices)
- Emotion detection in speech
- Real-time transcription display

---

## Rollout Plan

### Alpha Release (Internal Testing)

**Audience:** Development team only
**Duration:** 1 week
**Criteria:**
- POC successful
- Basic E2E working
- Known bugs documented

---

### Beta Release (Limited Users)

**Audience:** 5-10 volunteer users
**Duration:** 2 weeks
**Criteria:**
- All Phase 1-5 tasks complete
- TTS duplication bug fixed
- Latency target met
- Feedback collection process ready

**Feedback Collection:**
- User survey (10 questions)
- Usage metrics (latency, error rate)
- Bug reports

---

### Production Release (v2.4.0)

**Audience:** All CortexAvatar users
**Duration:** Ongoing
**Criteria:**
- Beta feedback incorporated
- All success criteria met
- Documentation complete
- Demo video published
- Release notes finalized

**Monitoring:**
- Service uptime (target: 99%)
- Average latency (target: <2s)
- Error rate (target: <1%)
- User satisfaction (target: 8/10)

---

## Timeline

**Total Duration:** 6-7 weeks

```
Week 1: POC + Performance Testing
├─ Day 1-2: Setup + initial testing
├─ Day 3-4: Benchmarking
└─ Day 5: Go/No-Go decision

Week 2-3: HTTP API Service
├─ Week 2: FastAPI implementation + Dockerization
└─ Week 3: Testing + documentation

Week 4: Go Client Integration
├─ Day 1-3: Client implementation
└─ Day 4-5: Testing + fallback logic

Week 5: Frontend Components
├─ Day 1-3: Svelte components
└─ Day 4-5: Integration testing

Week 6: Bug Fixes + E2E Testing
├─ Fix TTS duplication bug
├─ E2E testing
└─ Performance validation

Week 7: Documentation + Deployment
├─ Write docs
├─ Create demo
└─ Production release
```

---

## Appendix

### A. Related Documents

- Evaluation: `/Users/normanking/ServerProjectsMac/cortex-gateway-test/SPEECH-TO-SPEECH-EVALUATION.md`
- HF Repo: https://github.com/huggingface/speech-to-speech
- CortexAvatar Issues: See CLAUDE.md (TTS duplication, dnet integration)

### B. Glossary

- **VAD:** Voice Activity Detection - filters silence from audio
- **STT:** Speech-to-Text - converts audio to text
- **TTS:** Text-to-Speech - converts text to audio
- **A2A:** Agent-to-Agent protocol - CortexAvatar ↔ CortexBrain communication
- **MPS:** Metal Performance Shaders - Apple's GPU acceleration
- **HF:** Hugging Face
- **POC:** Proof of Concept

### C. References

- Whisper: https://github.com/openai/whisper
- Parler-TTS: https://github.com/huggingface/parler-tts
- Silero VAD: https://github.com/snakers4/silero-vad
- FastAPI: https://fastapi.tiangolo.com/

---

**PRD Approval:**

- [ ] Product Owner
- [ ] Technical Lead
- [ ] UX Designer
- [ ] QA Lead

**Next Steps:**
1. Review PRD with stakeholders
2. Get approval signatures
3. Start Phase 0: Prerequisites
4. Create Jira/Linear tickets for all tasks

---

**Last Updated:** 2026-02-06
**Version:** 1.0
**Status:** 📋 Awaiting Approval
