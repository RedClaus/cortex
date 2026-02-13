---
project: Cortex
component: Docs
phase: Ideation
date_created: 2026-02-10T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-11T01:40:42.698812
---

# CortexBrain Hardware Design Capability PRD

## Executive Summary

CortexBrain now includes **GateFlow**, a SystemVerilog/RTL development capability for designing custom hardware accelerators. This enables the AI swarm to design specialized hardware for voice processing, video analysis, and real-time perception enhancement.

---

## Vision

Enable CortexBrain to move beyond software-only solutions by designing custom hardware:
- **Voice Processing ASICs** - Low-latency audio pipelines
- **Video Processing Units** - Real-time frame analysis
- **Neural Accelerators** - Custom inference engines
- **Sensor Fusion Hardware** - Multi-modal perception processing

---

## GateFlow Integration

### Location
```
CortexBrain/plugins/gateflow/
├── agents/           # 11 specialized RTL experts
├── skills/           # 12 workflow skills
├── commands/         # 7 quick-action commands
├── hooks/            # Automation hooks
├── CLAUDE.md         # SystemVerilog quick reference
└── GATEFLOW-REFERENCE.md  # Comprehensive patterns
```

### Available Agents

| Agent | Expertise | Use Case |
|-------|-----------|----------|
| `sv-codegen` | RTL architect | Generate modules, FSMs, FIFOs |
| `sv-testbench` | Verification engineer | Create comprehensive testbenches |
| `sv-debug` | Debug specialist | Fix X-propagation, timing issues |
| `sv-verification` | Formal verification | Add assertions, coverage |
| `sv-understanding` | RTL analyst | Explain existing designs |
| `sv-planner` | Architecture planner | Design system architecture |
| `sv-refactor` | Code quality | Lint fixes, style cleanup |
| `sv-developer` | Full-stack RTL | Multi-file implementations |
| `sv-viz` | Visualization | Terminal diagrams, hierarchies |
| `sv-tutor` | Education | Teach SystemVerilog concepts |
| `sv-orchestrator` | Coordinator | Multi-agent hardware projects |

### Available Skills

| Skill | Purpose |
|-------|---------|
| `/gf` | Main hardware development workflow |
| `/gf-plan` | Architecture planning with references |
| `/gf-build` | Build and synthesis workflow |
| `/gf-architect` | Codebase mapping and architecture |
| `/gf-lint` | Linting with Verilator/Verible |
| `/gf-sim` | Simulation workflow |
| `/gf-expand` | Expand design from spec |
| `/gf-viz` | Terminal visualization |
| `tb-best-practices` | Testbench patterns |

### Quick Commands

| Command | Action |
|---------|--------|
| `/gf-doctor` | Check toolchain (Verilator, Verible) |
| `/gf-scan` | Discover SV files in project |
| `/gf-map` | Create codebase architecture map |
| `/gf-lint` | Run lint checks |
| `/gf-sim` | Run simulation |
| `/gf-gen` | Generate module from spec |
| `/gf-fix` | Fix lint warnings |

---

## Use Cases for CortexBrain

### 1. Voice Processing Hardware

Design custom audio processing pipelines:
```
User: "Design a low-latency voice preprocessing ASIC"

CortexBrain → GateFlow:
- sv-planner: Architecture design
- sv-codegen: Generate modules
  - Audio FIFO buffer
  - Sample rate converter
  - Noise filter
  - VAD (Voice Activity Detection)
- sv-testbench: Create verification
- sv-verification: Add assertions
```

### 2. Video Frame Analyzer

Real-time video processing for perception:
```
User: "Create hardware for face detection preprocessing"

CortexBrain → GateFlow:
- sv-planner: Frame buffer + convolution pipeline
- sv-codegen: Generate modules
  - Frame buffer with dual-port RAM
  - 3x3 convolution kernel
  - Edge detection filter
  - ROI extractor
- sv-sim: Verify with test frames
```

### 3. Sensor Fusion Accelerator

Multi-modal input processing:
```
User: "Design sensor fusion hardware for audio + video sync"

CortexBrain → GateFlow:
- sv-architect: Map existing designs
- sv-planner: Timestamp alignment + correlation
- sv-developer: Multi-file implementation
  - Timestamp synchronizer
  - Cross-modal correlator
  - Output formatter
```

---

## Integration Points

### From Pinky/Chat Interface

Users can request hardware design through natural language:
```
"Design a FIFO for audio buffering"
→ Route to GateFlow sv-codegen agent

"Explain this Verilog module"
→ Route to GateFlow sv-understanding agent

"Fix the latch warning in my FSM"
→ Route to GateFlow sv-debug agent
```

### From Coding Agents (Codie, Albert, Harold)

Agents can invoke GateFlow for hardware tasks:
```javascript
// In agent code
const hardwareTask = {
    type: "hardware_design",
    agent: "gateflow:sv-codegen",
    spec: "Create a 2-stage pipeline with valid/ready handshake"
};
```

### Tool Detection in pinky_compat.go

Add hardware design detection:
```go
// Detect hardware/RTL queries
if containsAny(inputLower, []string{
    "verilog", "systemverilog", "rtl", "fpga",
    "hardware design", "asic", "fsm", "fifo",
    "synthesis", "testbench"
}) {
    return "gateflow", map[string]string{"query": input}
}
```

---

## Technical Details

### SystemVerilog Patterns (Quick Reference)

**Always Blocks:**
| Purpose | Construct | Assignment |
|---------|-----------|------------|
| Flip-flops | `always_ff @(posedge clk)` | `<=` (non-blocking) |
| Combinational | `always_comb` | `=` (blocking) |

**FSM Template:**
```systemverilog
typedef enum logic [1:0] {IDLE, ACTIVE, DONE} state_t;
state_t state, next_state;

always_ff @(posedge clk or negedge rst_n)
    if (!rst_n) state <= IDLE;
    else        state <= next_state;

always_comb begin
    next_state = state;
    unique case (state)
        IDLE:   if (start) next_state = ACTIVE;
        ACTIVE: if (done)  next_state = DONE;
        DONE:   next_state = IDLE;
        default: next_state = IDLE;
    endcase
end
```

**Valid/Ready Handshake:**
```systemverilog
wire transfer = valid && ready;
always_ff @(posedge clk)
    if (transfer) captured_data <= data_in;
```

### Required Toolchain

| Tool | Purpose | Install |
|------|---------|---------|
| Verilator | Lint & Simulation | `brew install verilator` |
| Verible | Format & Lint | `brew install verible` |

Check with: `/gf-doctor`

---

## Roadmap

### Phase 1: Foundation (Current)
- [x] GateFlow plugin installed in CortexBrain
- [x] PRD documented
- [ ] Add hardware intent detection to pinky_compat
- [ ] Test basic module generation

### Phase 2: Integration
- [ ] Connect GateFlow agents to swarm messaging
- [ ] Add hardware design as CortexBrain tool
- [ ] Enable Codie/Albert/Harold to invoke GateFlow

### Phase 3: Advanced
- [ ] Voice processing hardware library
- [ ] Video processing hardware library
- [ ] Synthesis to actual FPGA (if hardware available)

---

## Success Metrics

| Metric | Target |
|--------|--------|
| Hardware design queries handled | > 80% |
| Lint-clean generated code | 100% |
| Testbench coverage | > 90% |
| Successful simulations | > 95% |

---

## References

- [GateFlow Plugin](../plugins/gateflow/)
- [SystemVerilog Reference](../plugins/gateflow/GATEFLOW-REFERENCE.md)
- [CortexBrain Architecture](../README.md)
- [Emotional Intelligence PRD](./EMOTIONAL-INTELLIGENCE-PRD.md)

---

## Next Steps

1. [ ] Add hardware detection to `pinky_compat.go`
2. [ ] Create test SystemVerilog project
3. [ ] Verify GateFlow agents work via CortexBrain
4. [ ] Document in swarm capabilities
5. [ ] Push to GitHub
