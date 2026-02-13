---
project: Cortex
component: Docs
phase: Build
date_created: 2026-01-16T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:20:39.923798
---

# Real-Time 3D Avatar with Streaming

This document describes the new TalkingHead-style avatar system with lip-sync, emotions, and streaming support.

## Phase 1 Complete: Frontend Avatar System

### New Files Created

```
frontend/src/lib/avatar/
├── TalkingHeadController.ts   # Main avatar controller
├── AvatarScene.svelte         # Svelte wrapper component
├── visemes.ts                 # Viseme types and mappings
├── emotions.ts                # Emotion detection and mapping
└── index.ts                   # Exports
```

### Features Implemented

1. **VRM Model Loading** - Uses @pixiv/three-vrm
2. **15 Oculus Visemes** - Full lip-sync support
3. **Emotion System** - 8 emotions with VRM expression mapping
4. **Idle Animations** - Blinking, head movement
5. **Audio-Reactive Fallback** - Web Audio API analysis when no viseme data
6. **Text-Based Emotion Detection** - Keyword analysis

### How to Use

The existing Avatar3D.svelte continues to work. The new TalkingHeadController provides an alternative approach:

```typescript
import { TalkingHeadController } from './lib/avatar';

const controller = new TalkingHeadController(container, {
  modelUrl: '/models/hannah.vrm',
  defaultEmotion: 'neutral',
  enableIdleAnimations: true
});

await controller.loadModel('/models/hannah.vrm');
controller.setEmotion('happy', 0.7);
controller.playVisemeTimeline(visemes, duration);
```

## Remaining Work (Phase 2-3)

### Backend Streaming (Not Yet Implemented)

- [ ] Cartesia TTS provider (`internal/tts/cartesia.go`)
- [ ] Deepgram STT provider (`internal/stt/deepgram.go`)
- [ ] Streaming orchestrator (`internal/bridge/streaming_orchestrator.go`)
- [ ] A2A streaming client update

### CortexBrain Changes (Not Yet Implemented)

- [ ] SSE streaming endpoint
- [ ] LLM provider streaming support

### Environment Variables Needed

```bash
# For Phase 2 (streaming providers)
export CARTESIA_API_KEY="your-key"
export DEEPGRAM_API_KEY="your-key"
```

## Testing the Current Implementation

1. Build and run:
   ```bash
   wails dev
   ```

2. The avatar should load with existing lip-sync (timer-based)

3. Test emotions by sending messages that trigger detection:
   - "This is amazing!" → happy
   - "I'm sorry to hear that" → sad
   - "Wow, really?" → surprised

## VRM Models

Available models in `frontend/public/models/`:
- `avatar.vrm` - Default avatar
- `hannah.vrm` - Female character
- `henry.vrm` - Male character

Get more from:
- VRoid Hub: https://hub.vroid.com/
- VRoid Studio: Free avatar creator
