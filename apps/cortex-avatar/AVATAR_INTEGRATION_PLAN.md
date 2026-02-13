---
project: Cortex
component: Unknown
phase: Ideation
date_created: 2026-01-07T08:20:17
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:20:10.560905
---

# Cortex Avatar 3D Integration Plan

**Version:** 1.0.0  
**Status:** ARCHITECT APPROVED  
**Last Updated:** January 7, 2025

---

## Executive Summary

This plan integrates advanced 3D avatar rendering (Hannah/Henry) into the existing cortex-avatar project. The current codebase uses **Wails (Go + Svelte WebView)** for a 2D avatar interface. This plan adds a **native OpenGL 4.1 rendering mode** for high-fidelity 3D avatars while preserving the existing Wails interface as a fallback.

### Key Architectural Decision

**Hybrid Approach Selected**: Rather than replacing the existing Wails app, we create a **separate 3D renderer executable** that can:
1. Run standalone for maximum performance
2. Be embedded into Wails via a native window overlay (future)
3. Share the same Cortex bridge infrastructure

This preserves existing functionality while enabling advanced 3D rendering.

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        CORTEX AVATAR SYSTEM                                  │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌────────────────────┐         ┌────────────────────┐                      │
│  │   Wails App        │         │   3D Renderer      │                      │
│  │   (Existing)       │         │   (New)            │                      │
│  │   ┌──────────────┐ │         │   ┌──────────────┐ │                      │
│  │   │ Svelte UI    │ │         │   │ OpenGL 4.1   │ │                      │
│  │   │ 2D Avatar    │ │         │   │ Hannah/Henry │ │                      │
│  │   └──────────────┘ │         │   │ 60+ FPS      │ │                      │
│  └─────────┬──────────┘         └─────────┬────────┘                        │
│            │                              │                                  │
│            └──────────┬───────────────────┘                                  │
│                       │                                                      │
│            ┌──────────▼──────────┐                                          │
│            │   Shared Bridge     │                                          │
│            │   internal/bridge/  │                                          │
│            └──────────┬──────────┘                                          │
│                       │                                                      │
└───────────────────────┼──────────────────────────────────────────────────────┘
                        │
                        ▼
              ┌───────────────────┐
              │    Cortex-02      │
              │    (localhost)    │
              └───────────────────┘
```

---

## Project Structure (New Additions)

```
cortex-avatar/
├── cmd/
│   ├── cortexavatar/           # Existing Wails app
│   └── avatar3d/               # NEW: Standalone 3D renderer
│       └── main.go
│
├── internal/
│   ├── bridge/                 # EXISTING: Shared bridge code
│   │   ├── cortex_bridge.go    # NEW: Cortex protocol bridge
│   │   └── state_mapper.go     # NEW: State → Expression mapping
│   │
│   ├── renderer/               # NEW: OpenGL renderer
│   │   ├── renderer.go
│   │   ├── shader.go
│   │   ├── mesh.go
│   │   ├── texture.go
│   │   ├── camera.go
│   │   ├── lighting.go
│   │   └── postprocess.go
│   │
│   ├── avatar3d/               # NEW: 3D avatar system
│   │   ├── avatar.go
│   │   ├── blendshape.go
│   │   ├── expression.go
│   │   ├── eye_controller.go
│   │   ├── idle.go
│   │   └── lipsync.go
│   │
│   └── gaussian/               # FUTURE: 3D Gaussian Splatting
│       ├── splat.go
│       └── rasterizer.go
│
├── assets/
│   ├── avatars/                # Existing 2D assets
│   ├── models/                 # NEW: 3D models
│   │   ├── hannah/
│   │   │   ├── hannah.glb
│   │   │   └── textures/
│   │   └── henry/
│   │       ├── henry.glb
│   │       └── textures/
│   ├── shaders/                # NEW: GLSL shaders
│   │   ├── skin.vert
│   │   ├── skin.frag
│   │   ├── eye.vert
│   │   ├── eye.frag
│   │   ├── hair.vert
│   │   ├── hair.frag
│   │   └── postprocess.frag
│   └── textures/               # NEW: Shared textures
│       └── skin_lut.png
│
├── tools/
│   └── blender_export/         # NEW: Asset pipeline scripts
│
└── docs/
    ├── ARCHITECTURE.md         # NEW: 3D system architecture
    └── CORTEX_PROTOCOL.md      # NEW: Protocol documentation
```

---

## Phase Breakdown

### Phase 0: Foundation Setup (2 days)
**Owner:** Sisyphus (main agent)

#### Deliverables
- [ ] Create directory structure for new modules
- [ ] Add OpenGL dependencies to go.mod
- [ ] Copy dev guide example code as starting templates
- [ ] Verify OpenGL compiles on macOS
- [ ] Create Makefile targets for 3D renderer

#### Dependencies
- None (starting point)

#### Verification
- `go build ./cmd/avatar3d` succeeds
- Basic GLFW window opens and closes

#### Agent Assignment
| Task | Agent |
|------|-------|
| Directory setup | Sisyphus |
| go.mod updates | Sisyphus |
| Template copying | Sisyphus |
| macOS verification | Sisyphus |

---

### Phase 1: Renderer Foundation (1 week)
**Owner:** avatar-graphics-engineer (via oracle)

#### Deliverables
- [ ] `internal/renderer/renderer.go` - OpenGL 4.1 context, GLFW window
- [ ] `internal/renderer/shader.go` - Shader compilation with hot-reload
- [ ] `internal/renderer/camera.go` - Fixed conversation camera (24mm FOV)
- [ ] `internal/renderer/lighting.go` - Studio 3-point lighting
- [ ] `internal/renderer/postprocess.go` - HDR framebuffer, tone mapping
- [ ] Test cube rendering at 60 FPS

#### Dependencies
- Phase 0 complete

#### Verification
- Colored cube renders with PBR lighting
- Frame time < 16ms sustained
- Shader hot-reload works (fsnotify)
- Window resizes correctly

#### Agent Assignment
| Task | Agent |
|------|-------|
| Renderer architecture | avatar-graphics-engineer |
| Shader system | avatar-graphics-engineer |
| Camera/lighting | avatar-graphics-engineer |
| Integration testing | Sisyphus |

#### Risk Mitigation
- **Risk:** OpenGL 4.1 limited on macOS (no compute shaders)
- **Mitigation:** CPU blendshapes, deferred GPU features to Phase 6+

---

### Phase 2: Mesh & Blendshape System (1 week)
**Owner:** avatar-graphics-engineer

#### Deliverables
- [ ] `internal/renderer/mesh.go` - glTF 2.0 loader with morph targets
- [ ] `internal/avatar3d/blendshape.go` - 52 ARKit blendshapes
- [ ] `internal/avatar3d/avatar.go` - Avatar container struct
- [ ] CPU blendshape computation (SIMD-friendly)
- [ ] GPU buffer double-buffering for smooth updates
- [ ] Placeholder head mesh renders with manual blendshape control

#### Dependencies
- Phase 1 complete
- Placeholder mesh asset (can use simple sphere initially)

#### Verification
- glTF model loads successfully
- All 52 blendshapes addressable
- Blendshape CPU compute < 1.5ms
- Manual slider control demonstrates shape morphing

#### Agent Assignment
| Task | Agent |
|------|-------|
| glTF loader | avatar-graphics-engineer + librarian (qmuntal/gltf docs) |
| Blendshape math | avatar-graphics-engineer |
| Buffer management | avatar-graphics-engineer |
| Testing | Sisyphus |

#### Risk Mitigation
- **Risk:** glTF morph target format variations
- **Mitigation:** Validate with multiple test models, create asset validator tool

---

### Phase 3: Skin & Eye Rendering (1 week)
**Owner:** avatar-graphics-engineer

#### Deliverables
- [ ] `assets/shaders/skin.vert/frag` - PBR + Subsurface Scattering
- [ ] `assets/shaders/eye.vert/frag` - Refraction, iris detail
- [ ] Pre-integrated skin LUT texture generation
- [ ] Normal mapping pipeline
- [ ] `internal/renderer/texture.go` - Texture management
- [ ] Photorealistic skin on static mesh

#### Dependencies
- Phase 2 complete

#### Verification
- SSS visible on thin areas (ears, nose, fingers)
- Eye reflections match environment
- No visible polygon edges at 1080p
- Skin shader < 3ms GPU time

#### Agent Assignment
| Task | Agent |
|------|-------|
| Skin shader | avatar-graphics-engineer |
| Eye shader | avatar-graphics-engineer |
| LUT generation | avatar-graphics-engineer |
| Visual QA | Sisyphus |

#### Risk Mitigation
- **Risk:** SSS performance on older GPUs
- **Mitigation:** Single-pass pre-integrated approach, fallback to simple diffuse

---

### Phase 4: Expression System (1 week)
**Owner:** avatar-graphics-engineer

#### Deliverables
- [ ] `internal/avatar3d/expression.go` - Expression controller
  - Presets: neutral, attentive, thinking, concerned, confident
  - Interpolation: linear, ease-in-out, spring
  - Layer system for additive blending
- [ ] `internal/avatar3d/eye_controller.go` - Gaze targeting, blink logic
- [ ] `internal/avatar3d/idle.go` - Breathing, micro-movements
- [ ] Expression transition testing tool

#### Dependencies
- Phase 3 complete

#### Verification
- Smooth transitions between all presets (no popping)
- Blink timing natural (3-5s interval)
- Gaze tracks target point smoothly
- Idle animation subtle but perceptible

#### Agent Assignment
| Task | Agent |
|------|-------|
| Expression controller | avatar-graphics-engineer |
| Eye controller | avatar-graphics-engineer |
| Idle system | avatar-graphics-engineer |
| Tuning & QA | Sisyphus |

---

### Phase 5: Cortex Integration (1 week)
**Owner:** Sisyphus + avatar-graphics-engineer

#### Deliverables
- [ ] `internal/bridge/cortex_bridge.go` - Unix socket + TCP connection
  - Reconnection logic with exponential backoff
  - State interpolation buffer (50ms)
  - Async state polling
- [ ] `internal/bridge/state_mapper.go` - Cognitive state → Expression
  - Valence/arousal mapping
  - Mode-based base expressions
  - Confidence scaling
- [ ] `internal/avatar3d/lipsync.go` - Viseme processing
  - 15 viseme shapes (silence to vowels)
  - Coarticulation blending
  - Audio timing synchronization
- [ ] MockCortexBridge for standalone testing

#### Dependencies
- Phase 4 complete
- Cortex-02 protocol documentation

#### Verification
- Expression updates within 150ms of state change
- Automatic reconnection after disconnect
- Graceful degradation without Cortex (uses mock)
- Lip-sync matches audio within 100ms
- Mode transitions (idle→speaking→thinking) smooth

#### Agent Assignment
| Task | Agent |
|------|-------|
| Cortex bridge | Sisyphus (uses existing bridge patterns) |
| State mapper | avatar-graphics-engineer |
| Lip-sync | avatar-graphics-engineer |
| Protocol testing | explore (find existing patterns) |

#### Risk Mitigation
- **Risk:** Cortex protocol changes
- **Mitigation:** Version-aware protocol handling, mock bridge for testing

---

### Phase 6: Hair & Polish (1 week)
**Owner:** avatar-graphics-engineer

#### Deliverables
- [ ] `assets/shaders/hair.vert/frag` - Kajiya-Kay anisotropic shader
- [ ] Post-processing pipeline
  - ACES tone mapping
  - FXAA anti-aliasing
  - Optional bloom
- [ ] Performance profiling and optimization
- [ ] Hannah asset integration
- [ ] Henry asset integration
- [ ] Cross-platform testing (macOS primary)

#### Dependencies
- Phase 5 complete
- Hannah/Henry 3D assets (see Asset Strategy)

#### Verification
- 60 FPS sustained for 10 minutes
- Frame time variance < 2ms std dev
- No memory leaks (stable after 1 hour)
- GPU utilization < 80%
- Both avatars render correctly

#### Agent Assignment
| Task | Agent |
|------|-------|
| Hair shader | avatar-graphics-engineer |
| Post-processing | avatar-graphics-engineer |
| Performance optimization | avatar-graphics-engineer + explore (find bottlenecks) |
| Asset integration | Sisyphus |

---

## Asset Strategy

### Avatar Creation Pipeline

```
┌─────────────────────────────────────────────────────────────────┐
│                  AVATAR ASSET PIPELINE                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  [1] BASE MESH                                                  │
│      Option A: Commission from 3D artist                        │
│      Option B: Use open-source base (Manuel Bastioni, MBLab)    │
│      Option C: Generate from MetaHuman-like service             │
│                                                                  │
│  [2] BLENDSHAPES                                                │
│      ├── Sculpt 52 ARKit shapes in Blender                      │
│      ├── Use ARKit blendshape plugin                            │
│      └── Export as glTF morph targets                           │
│                                                                  │
│  [3] TEXTURING                                                  │
│      ├── Albedo (4K, sRGB)                                      │
│      ├── Normal (4K, linear)                                    │
│      ├── Roughness/Metallic (2K)                                │
│      ├── SSS Thickness (2K)                                     │
│      └── AO (2K)                                                │
│                                                                  │
│  [4] EXPORT                                                      │
│      ├── glTF 2.0 binary (.glb)                                 │
│      ├── Draco compression (optional)                            │
│      └── Validate with asset_validator tool                      │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Recommended Asset Approach

**Phase 1-5:** Use placeholder mesh (geometric head or open-source model)  
**Phase 6:** Integrate production Hannah/Henry assets

**Asset Options (Ranked by Quality/Effort):**
1. **Commission from 3D artist** - Best quality, highest cost ($500-2000)
2. **Modify open-source base** - Good quality, moderate effort (free)
3. **Generate from AI service** - Variable quality, fast iteration (varies)

---

## Risk Register

| ID | Risk | Probability | Impact | Mitigation |
|----|------|-------------|--------|------------|
| R1 | OpenGL 4.1 limitations on macOS | High | Medium | CPU blendshapes, defer compute shaders |
| R2 | Asset creation delays | Medium | High | Use placeholders, decouple from rendering |
| R3 | Expression uncanny valley | Medium | High | Subtle micro-expressions, avoid over-animation |
| R4 | Performance on older GPUs | Medium | Medium | LOD system, quality presets |
| R5 | Cortex protocol changes | Low | Medium | Version-aware handling, mock bridge |
| R6 | glTF morph target incompatibility | Low | Medium | Asset validator, multiple test models |
| R7 | Build complexity (CGO, OpenGL) | Medium | Medium | Docker build environment, CI/CD |

---

## Acceptance Criteria Summary

### Visual Quality
- [ ] No visible polygon edges at 1080p
- [ ] Skin SSS visible on thin areas
- [ ] Eye reflections match environment
- [ ] Expression transitions smooth (no popping)
- [ ] Blink timing natural (3-5s interval)
- [ ] Hair silhouette clean against background

### Performance
- [ ] 60 FPS sustained for 10 minutes
- [ ] Frame time variance < 2ms std dev
- [ ] No memory leaks (stable after 1 hour)
- [ ] GPU utilization < 80%

### Cortex Integration
- [ ] Expression updates within 150ms of state change
- [ ] Reconnection automatic after disconnect
- [ ] Graceful degradation without Cortex
- [ ] Lip-sync matches audio within 100ms
- [ ] Mode transitions (idle→speaking→thinking) smooth

---

## Dependencies (go.mod additions)

```go
require (
    // Existing...
    
    // NEW: OpenGL rendering
    github.com/go-gl/gl v0.0.0-20231021071112-07e5d0ea2e71
    github.com/go-gl/glfw/v3.3/glfw v0.0.0-20240506104042-037f3cc74f2a
    github.com/go-gl/mathgl v1.1.0
    
    // NEW: Asset loading
    github.com/qmuntal/gltf v0.27.0
    
    // Existing fsnotify can be used for shader hot-reload
)
```

---

## Timeline Summary

| Phase | Duration | Start | End | Owner |
|-------|----------|-------|-----|-------|
| Phase 0 | 2 days | Week 1 | Week 1 | Sisyphus |
| Phase 1 | 1 week | Week 1 | Week 2 | avatar-graphics-engineer |
| Phase 2 | 1 week | Week 2 | Week 3 | avatar-graphics-engineer |
| Phase 3 | 1 week | Week 3 | Week 4 | avatar-graphics-engineer |
| Phase 4 | 1 week | Week 4 | Week 5 | avatar-graphics-engineer |
| Phase 5 | 1 week | Week 5 | Week 6 | Sisyphus + avatar-graphics-engineer |
| Phase 6 | 1 week | Week 6 | Week 7 | avatar-graphics-engineer |

**Total Estimated Duration:** 7 weeks

---

## Future Phases (Post-MVP)

### Phase 7: 3D Gaussian Splatting Hair
- Replace anisotropic hair with 3DGS for photorealistic strands
- Requires Vulkan or OpenGL compute (not macOS compatible)
- ~50K Gaussians per avatar

### Phase 8: Wails Integration
- Embed OpenGL window into Wails app
- Shared memory for texture streaming
- Unified settings UI

### Phase 9: AR/VR Support
- OpenXR integration
- Stereoscopic rendering
- Hand tracking for gestures

---

## Approval

**Architect Decision:** APPROVED

**Rationale:**
1. Hybrid approach preserves existing functionality
2. Clear separation of concerns (renderer vs avatar vs bridge)
3. Realistic timeline with buffer for asset delays
4. Risk mitigations address all high-impact scenarios
5. Phased approach allows early demos with placeholders

**Conditions:**
1. Phase 0 must verify OpenGL builds before proceeding
2. Asset strategy decision required before Phase 6
3. Performance profiling mandatory at each phase end

---

## Next Steps

1. **Execute Phase 0** - Setup directory structure and dependencies
2. **Invoke avatar-graphics-engineer** for Phase 1 implementation
3. **Begin asset sourcing** in parallel with development
4. **Create todo list** tracking all deliverables

---

*Document generated by Sisyphus | Architect approval: Oracle*
