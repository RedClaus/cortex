---
project: Cortex
component: Brain Kernel
phase: Design
date_created: 2026-02-10T23:32:11
source: ServerProjectsMac
librarian_indexed: 2026-02-11T01:40:42.362200
---

# CortexBrain

**AI Inference and Reasoning Core for the Cortex Ecosystem**

CortexBrain is a brain-inspired AI system that provides intelligent routing, cognitive workflows, and specialized processing for AI agents. It serves as the central reasoning engine for Pinky and other Cortex components.

## Architecture

CortexBrain is modeled after a biological brain with specialized "lobes" for different cognitive functions:

```
CortexBrain/
├── pkg/brain/
│   ├── personas/         # Expert cognitive modes (29 personas)
│   ├── skills/           # Cognitive workflows (43 skills)
│   ├── workflow/         # Double Diamond methodology
│   └── lobes/            # Specialized processing lobes
├── config/
│   ├── personas/         # Persona definitions (markdown)
│   ├── skills/           # Skill definitions (markdown)
│   └── workflows/        # Workflow methodology docs
├── plugins/
│   ├── manager/          # Plugin management system
│   └── gateflow/         # Hardware design plugin
└── internal/
    └── a2a/              # Agent-to-agent communication
```

## Core Capabilities

### 1. Claude Octopus Integration (Embedded)

29 expert personas and 43 specialized skills provide cognitive modes for different task types:

| Category | Personas | Phase |
|----------|----------|-------|
| Research | research-synthesizer, business-analyst, ai-engineer | Discover |
| Architecture | backend-architect, frontend-developer, database-architect | Define |
| Development | tdd-orchestrator, debugger, python-pro, typescript-pro | Develop |
| Delivery | code-reviewer, security-auditor, deployment-engineer | Deliver |

**Why Embedded (not plugin)?** Personas are *modes of cognition* - how the brain thinks about problems. They're not external tools, they're core capabilities.

### 2. Double Diamond Workflow

```
DISCOVER (Probe)  →  DEFINE (Grasp)  →  DEVELOP (Tangle)  →  DELIVER (Ink)
   diverge            converge            diverge             converge
     🔍                 🎯                  🛠️                  ✅
```

Each phase has quality gates and appropriate personas/skills are automatically selected.

### 3. Plugin System

Extensible architecture for domain-specific capabilities:

```bash
# Install from marketplace
cortex-plugin install gateflow

# Search available plugins
cortex-plugin search "hardware design"

# List installed plugins
cortex-plugin list
```

### 4. GateFlow Hardware Design (Plugin)

SystemVerilog/RTL design capability with 11 specialist agents and 12 workflow skills.

## Usage

### Persona Routing

```go
router := personas.NewRouter()
persona := router.Route("design the API for user authentication")
// Returns: backend-architect persona
```

### Skill Matching

```go
registry := skills.NewRegistry()
skill := registry.Match("debug the authentication error")
// Returns: skill-debug workflow with 6 systematic steps
```

### Workflow Management

```go
wm := workflow.NewWorkflowManager()
ws := wm.StartWorkflow("feature-123")
// Starts in Discover phase

wm.AdvancePhase("feature-123", true) // Move to Define
wm.AdvancePhase("feature-123", true) // Move to Develop
wm.AdvancePhase("feature-123", true) // Move to Deliver
```

## Integration

### With Pinky

CortexBrain integrates with Pinky via `pinky_compat.go`:
- Receives queries from Pinky WebUI
- Routes to appropriate personas
- Returns structured responses with workflow guidance

### IDE Agnostic

Because personas and skills are embedded, they work regardless of IDE:
- Claude Code
- VS Code / Cursor
- Vim / Neovim
- Any terminal

## Documentation

### Core Capabilities
- [Octopus Integration PRD](./docs/OCTOPUS-INTEGRATION-PRD.md) - 29 personas, 43 skills
- [Plugin System PRD](./docs/PLUGIN-SYSTEM-PRD.md) - Marketplace architecture
- [Hardware Design PRD](./docs/HARDWARE-DESIGN-PRD.md) - GateFlow plugin

### Planned Capabilities (P2+)
- [Observational Memory PRD](./docs/OBSERVATIONAL-MEMORY-PRD.md) - Three-tier memory compression (P2)
- [Skill Distillation PRD](./docs/SKILL-DISTILLATION-PRD.md) - Auto-evolving skills from experience (P2.5)
- [Emotional Intelligence PRD](./docs/EMOTIONAL-INTELLIGENCE-PRD.md) - Affect-aware responses (P3)

### Reference
- [Vault Index](./Vault/index.md)

## First Principles

CortexBrain follows biological brain architecture:

| Biological | CortexBrain | Purpose |
|------------|-------------|---------|
| Lobes | `pkg/brain/lobes/` | Specialized processing |
| Cognitive Modes | Personas | How to think about problems |
| Learned Skills | Skills Registry | Structured approaches |
| Executive Function | Workflow Manager | Phase coordination |
| Hippocampus | Observational Memory | Experience compression |
| Synaptic Plasticity | Skill Distillation | Learning from experience |

**Key Decisions:**
- External tools (GateFlow) are **plugins**
- Cognitive modes (personas) are **embedded**
- Memory consolidation (Observational Memory) is **embedded**
- Skill evolution (Skill Distillation) is **embedded** — evolution through pattern extraction, not weight training

## License

Part of the Cortex ecosystem.
