---
project: Cortex
component: Docs
phase: Design
date_created: 2026-02-10T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-11T01:40:42.862261
---

# CortexBrain Octopus Integration PRD

## Executive Summary

CortexBrain now integrates **Claude Octopus** capabilities as core cognitive extensions. This includes 29 expert personas, 43 specialized skills, and the Double Diamond workflow methodology - all embedded into the brain's lobe architecture.

---

## What Was Integrated

### 29 Expert Personas

| Category | Personas | Phase |
|----------|----------|-------|
| **Research** | research-synthesizer, business-analyst, ai-engineer, strategy-analyst | Discover |
| **Architecture** | backend-architect, frontend-developer, database-architect, cloud-architect, graphql-architect | Define |
| **Development** | tdd-orchestrator, debugger, python-pro, typescript-pro, devops-troubleshooter | Develop |
| **Delivery** | code-reviewer, security-auditor, test-automator, performance-engineer, deployment-engineer | Deliver |
| **Support** | docs-architect, mermaid-expert, incident-responder, context-manager, thought-partner | All |
| **Writing** | academic-writer, product-writer, exec-communicator, content-analyst, ux-researcher | Various |

### 43 Specialized Skills

| Category | Skills |
|----------|--------|
| **Workflow Flows** | flow-discover, flow-define, flow-develop, flow-deliver |
| **Core Skills** | skill-architecture, skill-tdd, skill-debug, skill-code-review |
| **Security** | skill-security-audit, skill-adversarial-security |
| **Research** | skill-deep-research, skill-brainstorm |
| **Documentation** | skill-doc-delivery, skill-prd |
| **Validation** | skill-validate, skill-verify, skill-rollback |
| **Collaboration** | skill-debate, skill-thought-partner |

### Double Diamond Workflow

```
   DISCOVER      DEFINE       DEVELOP      DELIVER
  (diverge)   (converge)   (diverge)   (converge)

    🔍            🎯           🛠️           ✅
   Probe        Grasp       Tangle        Ink

  Research → Requirements → Build → Validate
```

---

## Architecture

```
CortexBrain/
├── pkg/brain/
│   ├── personas/
│   │   └── router.go           # Persona routing and selection
│   ├── skills/
│   │   └── registry.go         # Skill registry and matching
│   └── workflow/
│       └── double_diamond.go   # Workflow phase management
├── config/
│   ├── personas/               # 29 persona definitions (markdown)
│   │   ├── backend-architect.md
│   │   ├── security-auditor.md
│   │   └── ...
│   ├── skills/                 # 43 skill definitions (markdown)
│   │   ├── skill-architecture.md
│   │   ├── skill-tdd.md
│   │   └── ...
│   └── workflows/
│       └── double-diamond.md   # Workflow methodology
```

---

## How It Works

### 1. Persona Router

The persona router automatically selects the best expert persona based on the task:

```go
router := personas.NewRouter()
persona := router.Route("design the API for user authentication")
// Returns: backend-architect persona
```

**Routing Logic:**
- Pattern matching on task keywords
- Phase-aware persona selection
- Expertise-based ranking

### 2. Skills Registry

The skills registry matches tasks to cognitive workflows:

```go
registry := skills.NewRegistry()
skill := registry.Match("debug the authentication error")
// Returns: skill-debug workflow
```

**Workflow Steps:**
1. Reproduce the issue
2. Gather error information
3. Form hypothesis
4. Test hypothesis
5. Implement fix
6. Verify fix

### 3. Double Diamond Workflow

The workflow manager tracks progress through the 4 phases:

```go
wm := workflow.NewWorkflowManager()
ws := wm.StartWorkflow("feature-123")
// Starts in Discover phase

wm.AdvancePhase("feature-123", true) // Move to Define
wm.AdvancePhase("feature-123", true) // Move to Develop
wm.AdvancePhase("feature-123", true) // Move to Deliver
```

---

## Integration Points

### With Pinky (via pinky_compat.go)

Queries are routed through personas:

```go
// In pinky_compat.go
router := personas.NewRouter()
persona := router.Route(userQuery)

// Adjust response based on persona expertise
if persona.Name == "security-auditor" {
    // Apply security-focused analysis
}
```

### With Emotion and Intent

Personas can be combined with emotional intelligence:

```
User Query: "I'm frustrated, the auth is broken again"

EmotionLobe: frustration detected
PersonaRouter: debugger persona
SkillRegistry: skill-debug workflow

Response: Acknowledges frustration + systematic debugging
```

### IDE Agnostic

Because personas and skills are embedded, they work regardless of IDE:
- Claude Code
- VS Code
- Cursor
- Vim/Neovim
- Any terminal

---

## Usage Examples

### Automatic Persona Selection

```
User: "Design the database schema for user accounts"
→ Routed to: database-architect persona
→ Uses: skill-architecture workflow
→ Phase: Define (Grasp)
```

### Explicit Phase Selection

```
User: "/discover OAuth best practices"
→ Phase: Discover (Probe)
→ Persona: research-synthesizer
→ Skill: skill-deep-research
```

### Multi-Phase Workflow

```
User: "Build user authentication feature"
→ Phase 1 (Discover): Research auth patterns
→ Phase 2 (Define): Design auth architecture
→ Phase 3 (Develop): Implement with TDD
→ Phase 4 (Deliver): Security review and ship
```

---

## Quality Gates

Each phase includes quality checks:

| Phase | Quality Gate |
|-------|--------------|
| Discover | Research synthesis complete |
| Define | Requirements documented, consensus achieved |
| Develop | Tests pass, security reviewed |
| Deliver | Final validation passed |

---

## First Principles Alignment

**Why EMBEDDED (not plugin)?**

| Aspect | GateFlow (Plugin) | Octopus (Embedded) |
|--------|-------------------|-------------------|
| Purpose | Domain tool (RTL) | Cognitive mode |
| Scope | Specific domain | General cognition |
| Required | Only for hardware | Always useful |
| Nature | What to do | How to think |

CortexBrain is modeled after a biological brain. Personas are *modes of cognition* - how the brain thinks about different types of problems. They're not external tools, they're core capabilities.

---

## Files Created

| File | Purpose |
|------|---------|
| `pkg/brain/personas/router.go` | Persona routing logic |
| `pkg/brain/skills/registry.go` | Skill matching logic |
| `pkg/brain/workflow/double_diamond.go` | Workflow management |
| `config/personas/*.md` | 29 persona definitions |
| `config/skills/*.md` | 43 skill definitions |
| `config/workflows/double-diamond.md` | Workflow methodology |
| `docs/OCTOPUS-INTEGRATION-ANALYSIS.md` | First principles analysis |
| `docs/OCTOPUS-INTEGRATION-PRD.md` | This document |

---

## Next Steps

1. [x] Persona router implementation
2. [x] Skills registry implementation
3. [x] Double Diamond workflow manager
4. [x] Copy persona/skill definitions
5. [ ] Integrate with pinky_compat.go
6. [ ] Add to blackboard system
7. [ ] Test with Pinky WebUI

---

## References

- [Claude Octopus](https://github.com/nyldn/claude-octopus)
- [Integration Analysis](./OCTOPUS-INTEGRATION-ANALYSIS.md)
- [CortexBrain Architecture](../README.md)
- [Plugin System](./PLUGIN-SYSTEM-PRD.md)
