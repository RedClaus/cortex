---
project: Cortex
component: Docs
phase: Ideation
date_created: 2025-01-16T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:18:25.832081
---

# Software Manufacturing Process

**Version:** 1.0.0

> This process ensures quality and consistency across the full development lifecycle.

---

## Process Philosophy

Software manufacturing is a disciplined pipeline where each phase produces artifacts that gate the next.

---

## Phase State Machine

\`\`\`
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  DISCOVERY  │────▶│   DESIGN    │────▶│IMPLEMENTATION│
└─────────────┘     └─────────────┘     └──────┬──────┘
                                               │
┌─────────────┐     ┌─────────────┐     ┌──────▼──────┐
│ OPERATIONS  │◀────│  DEPLOYMENT │◀────│   TESTING   │
└─────────────┘     └─────────────┘     └─────────────┘
       │                                       ▲
       └───────────── FEEDBACK ────────────────┘
\`\`\`

Current phase is tracked in \`.claude/process/state.json\`

---

## Phase Definitions

### Phase 0: Discovery

**Purpose:** Understand the problem before solving it.

**Required Artifacts:**
| Artifact | Location | Description |
|----------|----------|-------------|
| Requirements | \`00-discovery/requirements.md\` | Problem statement, user needs, constraints |
| Success Criteria | \`00-discovery/success-criteria.md\` | Measurable outcomes (SMART goals) |
| Scope | \`00-discovery/scope.md\` | What's in, what's out |

**Gate Checklist:**
- [ ] Problem statement is clear and specific
- [ ] At least 3 measurable success criteria defined
- [ ] Scope boundaries explicitly stated

---

### Phase 1: Design

**Purpose:** Define how to solve the problem before writing code.

**Required Artifacts:**
| Artifact | Location | Description |
|----------|----------|-------------|
| Architecture | \`01-design/architecture.md\` | Components, boundaries, data flow |
| API Contracts | \`01-design/api-contracts/\` | Interface specifications |
| ADRs | \`01-design/decisions/\` | Architecture Decision Records |

**Gate Checklist:**
- [ ] Architecture diagram with component boundaries
- [ ] All external interfaces documented
- [ ] At least one ADR documenting a key decision

---

### Phase 2: Implementation

**Purpose:** Write the code according to design specifications.

**Required Artifacts:**
| Artifact | Location | Description |
|----------|----------|-------------|
| Source Code | Project src | The actual implementation |
| Unit Tests | Alongside source | Tests for each module |

**Gate Checklist:**
- [ ] All code follows project conventions
- [ ] Unit test coverage > 80%
- [ ] Code compiles/builds without warnings

---

### Phase 3: Testing

**Purpose:** Verify the implementation meets requirements.

**Required Artifacts:**
| Artifact | Location | Description |
|----------|----------|-------------|
| Test Plan | \`03-testing/test-plan.md\` | What to test, how, acceptance criteria |
| Coverage Report | \`03-testing/coverage/\` | Code coverage analysis |

**Gate Checklist:**
- [ ] Test plan approved
- [ ] All critical paths have test cases
- [ ] Coverage report generated

---

### Phase 4: Deployment

**Purpose:** Release the software safely.

**Required Artifacts:**
| Artifact | Location | Description |
|----------|----------|-------------|
| Release Notes | \`04-deployment/RELEASE_NOTES.md\` | What changed, migration steps |
| Rollback Plan | \`04-deployment/ROLLBACK.md\` | How to undo if needed |

**Gate Checklist:**
- [ ] Release notes complete
- [ ] Version bumped
- [ ] Rollback procedure documented

---

### Phase 5: Operations

**Purpose:** Monitor and maintain the running system.

**Required Artifacts:**
| Artifact | Location | Description |
|----------|----------|-------------|
| Incident Log | \`05-operations/incidents/\` | What went wrong |
| Retrospectives | \`05-operations/retros/\` | Lessons learned |

---

## Process Commands

| Command | Action |
|---------|--------|
| \`process:status\` | Show current phase, completed gates, blockers |
| \`process:advance\` | Validate gates and transition to next phase |
| \`process:gate-check\` | Audit artifacts against current phase requirements |
| \`process:init [feature-name]\` | Initialize directory structure for new feature |

---

## State File Schema

\`.claude/process/state.json\`:
\`\`\`json
{
  "feature": "feature-name",
  "currentPhase": "design",
  "startedAt": "2025-01-16T10:00:00Z",
  "phaseHistory": [],
  "gates": {}
}
\`\`\`

---

## Directory Structure for Features

When \`process:init\` is run:

\`\`\`
feature-name/
├── 00-discovery/
│   ├── requirements.md
│   ├── success-criteria.md
│   └── scope.md
├── 01-design/
│   ├── architecture.md
│   ├── api-contracts/
│   └── decisions/
├── 02-implementation/
├── 03-testing/
│   ├── test-plan.md
│   └── coverage/
├── 04-deployment/
│   ├── RELEASE_NOTES.md
│   └── ROLLBACK.md
└── 05-operations/
    ├── incidents/
    └── retros/
\`\`\`
