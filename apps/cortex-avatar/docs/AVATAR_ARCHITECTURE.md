---
project: Cortex
component: Docs
phase: Design
date_created: 2026-01-07T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:20:39.728508
---

# Cortex Avatar Architecture

## System Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           CORTEX AVATAR SYSTEM                           │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌────────────────┐     ┌────────────────┐     ┌────────────────┐       │
│  │   CORTEX-02    │────►│  AVATAR BRIDGE │────►│   EXPRESSION   │       │
│  │  Cognitive AI  │     │   (Go/Socket)  │     │   CONTROLLER   │       │
│  └────────────────┘     └────────────────┘     └────────────────┘       │
│                                                        │                 │
│                                                        ▼                 │
│                              ┌──────────────────────────────────────┐   │
│                              │         AVATAR INSTANCE              │   │
│                              │  ┌────────────┐  ┌────────────┐     │   │
│                              │  │  Hannah    │  │   Henry    │     │   │
│                              │  │  (Female)  │  │   (Male)   │     │   │
│                              │  └────────────┘  └────────────┘     │   │
│                              └──────────────────────────────────────┘   │
│                                               │                          │
│                                               ▼                          │
│  ┌────────────────┐     ┌────────────────┐     ┌────────────────┐       │
│  │   BLENDSHAPE   │────►│   GPU RENDER   │────►│  POST-PROCESS  │       │
│  │    COMPUTE     │     │   (OpenGL 4.1) │     │  (Tone Map)    │       │
│  └────────────────┘     └────────────────┘     └────────────────┘       │
│                                                        │                 │
│                                                        ▼                 │
│                                               ┌────────────────┐        │
│                                               │    DISPLAY     │        │
│                                               │   (60+ FPS)    │        │
│                                               └────────────────┘        │
└─────────────────────────────────────────────────────────────────────────┘
```

## Module Hierarchy

```
cortex-avatar/
├── cmd/
│   └── avatar/                 # Application entry point
│       └── main.go
│
├── internal/                   # Private packages
│   │
│   ├── renderer/               # OpenGL rendering
│   │   ├── renderer.go         # Context management
│   │   ├── shader.go           # Shader compilation
│   │   ├── mesh.go             # Mesh loading
│   │   ├── texture.go          # Texture management
│   │   ├── camera.go           # Camera system
│   │   ├── lighting.go         # Light definitions
│   │   └── postprocess.go      # Post-processing
│   │
│   ├── avatar/                 # Avatar management
│   │   ├── avatar.go           # Avatar container
│   │   ├── blendshape.go       # Blendshape system
│   │   ├── expression.go       # Expression controller
│   │   ├── eye_controller.go   # Gaze & blink
│   │   ├── idle.go             # Idle animations
│   │   └── lipsync.go          # Viseme processing
│   │
│   ├── bridge/                 # Cortex integration
│   │   ├── cortex_bridge.go    # Socket connection
│   │   └── state_mapper.go     # State → Expression
│   │
│   ├── gaussian/               # 3D Gaussian Splatting
│   │   ├── splat.go            # Gaussian structures
│   │   ├── rasterizer.go       # GPU rasterization
│   │   └── sort.go             # Depth sorting
│   │
│   └── config/                 # Configuration
│       ├── config.go           # Config loading
│       └── profiles.go         # Avatar profiles
│
├── pkg/                        # Public packages
│   └── avatartypes/            # Shared types
│       └── types.go
│
├── assets/                     # Runtime assets
│   ├── models/
│   │   ├── hannah/
│   │   └── henry/
│   ├── shaders/
│   └── textures/
│
└── tools/                      # Build tools
    ├── blender_export/
    └── asset_validator/
```

## Data Flow

### Frame Pipeline

```
Frame N @ 16.6ms (60 FPS)
│
├─► [1] INPUT STAGE (0.5ms)
│   │
│   ├── Poll Cortex state (non-blocking)
│   ├── Read viseme queue
│   └── Update input state
│
├─► [2] EXPRESSION STAGE (1.0ms)
│   │
│   ├── Map cognitive state → expression weights
│   ├── Process active transitions
│   ├── Apply expression layers
│   │   ├── Base expression
│   │   ├── Eye controller (gaze, blink)
│   │   ├── Idle animation
│   │   └── Lip-sync
│   └── Clamp final weights
│
├─► [3] GEOMETRY STAGE (1.5ms)
│   │
│   ├── Compute blended vertex positions
│   │   └── base + Σ(weight[i] * delta[i])
│   ├── Recalculate normals
│   └── Upload to GPU buffers
│
├─► [4] RENDER STAGE (8.0ms)
│   │
│   ├── Shadow Pass (1.0ms)
│   │   └── Render depth from light view
│   │
│   ├── Main Pass (6.0ms)
│   │   ├── Skin (PBR + SSS)
│   │   ├── Eyes (Refraction)
│   │   └── Hair (Anisotropic/3DGS)
│   │
│   └── Post-Process (1.0ms)
│       ├── Tone mapping
│       ├── Bloom (optional)
│       └── FXAA
│
└─► [5] PRESENT (VSync)
    │
    └── Swap buffers
```

### State Propagation

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  Cortex-02  │     │   Bridge    │     │  Avatar     │
│             │     │             │     │             │
│ Mode:       │     │ Buffer:     │     │ Weights:    │
│ "thinking"  │────►│ [state0]    │────►│ browInnerUp │
│             │     │ [state1]    │     │ = 0.25      │
│ Valence:    │     │ [state2]    │     │             │
│ 0.2         │     │             │     │ eyeLookUp   │
│             │     │ Interpolate │     │ = 0.30      │
│ Arousal:    │     │ @ render    │     │             │
│ 0.4         │     │ time        │     │ ...         │
└─────────────┘     └─────────────┘     └─────────────┘
```

## Component Details

### Renderer

```go
type Renderer struct {
    window  *glfw.Window
    config  Config
    
    // Shaders
    skinShader  *Shader    // PBR + SSS
    eyeShader   *Shader    // Refraction
    hairShader  *Shader    // Anisotropic/3DGS
    postShader  *Shader    // Tone mapping
    
    // Camera
    camera  *Camera
    
    // Lights
    lights  []Light
    
    // Framebuffers
    hdrFBO  uint32
}

// Methods
func (r *Renderer) BeginFrame()
func (r *Renderer) EndFrame()
func (r *Renderer) UseSkinShader()
func (r *Renderer) SetModelMatrix(m mgl32.Mat4)
```

### Avatar

```go
type Avatar struct {
    ID  AvatarID
    
    // Meshes
    headMesh  *Mesh
    eyesMesh  *Mesh
    hairMesh  *Mesh
    
    // Materials
    skinMat  *Material
    eyeMat   *Material
    hairMat  *Material
    
    // Blendshapes
    blendshapes     *BlendshapeData
    currentWeights  BlendshapeWeights
    
    // Controllers
    expressionCtrl  *ExpressionController
    eyeController   *EyeController
    idleController  *IdleController
    lipSyncCtrl     *LipSyncController
}

// Methods
func (a *Avatar) ApplyCortexState(state CortexState)
func (a *Avatar) Update(dt float32)
func (a *Avatar) Draw(r *Renderer)
```

### Expression Controller

```go
type ExpressionController struct {
    currentWeights  BlendshapeWeights
    transition      *BlendTransition
    layers          map[string]*ExpressionLayer
}

// Key function: Map cognitive state to expression
func MapCortexState(state CortexState) BlendshapeWeights {
    // 1. Select base expression from mode
    // 2. Apply valence/arousal modulation
    // 3. Scale by confidence
    // 4. Return final weights
}
```

### Cortex Bridge

```go
type CortexBridge struct {
    config  Config
    conn    net.Conn
    
    currentState  CortexState
    stateBuffer   []CortexState
    
    // Callbacks
    onStateChange  func(CortexState)
}

// Methods
func (b *CortexBridge) Start() error
func (b *CortexBridge) Stop()
func (b *CortexBridge) GetState() CortexState
func (b *CortexBridge) GetInterpolatedState(t time.Time) CortexState
```

## Performance Considerations

### CPU Budget

| Component | Budget | Strategy |
|-----------|--------|----------|
| Cortex Poll | 0.5ms | Async, buffered |
| Expression Map | 0.3ms | Direct calculation |
| Blendshape Compute | 1.5ms | SIMD, sparse deltas |
| Frame Overhead | 0.5ms | Minimal allocations |

### GPU Budget

| Pass | Budget | Strategy |
|------|--------|----------|
| Shadow | 1.0ms | Single cascade |
| Skin | 3.0ms | Single-pass SSS |
| Eyes | 1.0ms | Simple refraction |
| Hair | 2.0ms | Anisotropic |
| Post | 1.0ms | ACES + FXAA |

### Memory Budget

| Asset | Budget | Notes |
|-------|--------|-------|
| Meshes | 10MB | Hannah + Henry |
| Textures | 100MB | 4K head, 2K others |
| GPU Buffers | 20MB | Vertex, uniform |
| State | 1MB | Expression, interp |

## Extension Points

### Adding New Avatars

1. Create asset directory: `assets/models/new_avatar/`
2. Export glTF with morph targets
3. Add `AvatarID` constant
4. Test with `go run cmd/avatar -avatar new_avatar`

### Adding Expressions

1. Define preset in `expression.go`
2. Add to `MapCortexState` logic
3. Test with mock bridge

### Adding Effects

1. Create shader in `assets/shaders/`
2. Add loading in `renderer.initShaders()`
3. Add render pass in avatar.Draw()

## Error Handling

| Error | Response |
|-------|----------|
| Cortex disconnect | Continue with last state, reconnect |
| Shader compile fail | Use fallback embedded shader |
| Asset load fail | Fatal error with clear message |
| GPU memory full | Reduce texture resolution |
| Frame overtime | Log warning, continue |

## Testing Strategy

### Unit Tests
- Blendshape interpolation
- Expression mapping
- State serialization

### Integration Tests
- Full render loop
- Cortex mock connection
- Asset loading

### Visual Tests
- Expression coverage
- Transition smoothness
- Performance profiling
