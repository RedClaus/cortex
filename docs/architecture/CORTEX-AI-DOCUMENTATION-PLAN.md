---
project: Cortex
component: Docs
phase: Ideation
date_created: 2026-01-15T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:29.659485
---

# Cortex AI-Optimized Documentation System

## Executive Summary

Create a distributed documentation system where **any AI coding agent can read a single file per project** and immediately understand:
- What the project is and does
- Where it fits in the ecosystem
- Current status and version
- Recent changes and history
- How to build/run/test
- What work remains

This enables **instant context loading** for AI agents like Claude Code.

---

## Current Challenges

| Challenge | Impact |
|-----------|--------|
| Single monolithic CLAUDE.md | AI must parse 500+ lines to find relevant project info |
| No per-project context | Agent must explore to understand each project |
| No version tracking | Can't tell what's production-ready vs experimental |
| No change history | Agent doesn't know what was recently modified |
| No promotion workflow | Unclear how projects move from dev to production |

---

## Proposed Solution

### Three-Layer Documentation Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    CORTEX DOCUMENTATION HIERARCHY                            │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Layer 1: WORKSPACE ROOT                                                    │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  CLAUDE.md              - AI agent master context (existing)        │   │
│  │  .cortex-manifest.yaml  - Machine-readable project registry (NEW)   │   │
│  │  ECOSYSTEM-MAP.md       - Visual architecture guide (NEW)           │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│  Layer 2: ENVIRONMENT LEVEL                                                 │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  Production/                                                         │   │
│  │    ├── PRODUCTION-INDEX.md    - Versioned app catalog               │   │
│  │    └── PROMOTION-LOG.md       - History of promotions               │   │
│  │                                                                      │   │
│  │  Development/                                                        │   │
│  │    └── DEVELOPMENT-STATUS.md  - Active work tracker                 │   │
│  │                                                                      │   │
│  │  Test-UAT/                                                           │   │
│  │    └── TEST-QUEUE.md          - Apps awaiting promotion             │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│  Layer 3: PROJECT LEVEL (THE KEY INNOVATION)                                │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  Each project folder contains:                                       │   │
│  │    PROJECT.md           - AI-optimized single-file context (NEW)    │   │
│  │    CHANGELOG.md         - Version history with descriptions (NEW)   │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## PROJECT.md Template (Per-App AI Context)

This is the **single file** an AI agent reads to understand a project.

```markdown
# [Project Name]

> One-line description of what this project does.

## AI Agent Quick Context

| Field | Value |
|-------|-------|
| **Status** | `Development` / `Test-UAT` / `Production` |
| **Version** | `1.0.0` (semver, or `dev` if not released) |
| **Priority** | `P0-Critical` / `P1-High` / `P2-Medium` / `P3-Low` |
| **Location** | `/Users/normanking/ServerProjectsMac/[Environment]/[Project]` |
| **Language** | Go / Python / TypeScript / etc. |
| **Last Updated** | 2026-01-15 |

## What This Project Does

[2-3 paragraphs explaining the purpose, target users, and core functionality]

## Ecosystem Role

```
┌─────────────┐     ┌─────────────┐
│  Depends On │────►│ THIS PROJECT│────►│ Used By     │
│  - ProjectA │     │             │     │  - ProjectX │
│  - ProjectB │     └─────────────┘     │  - ProjectY │
└─────────────┘                         └─────────────┘
```

- **Depends on**: [List upstream dependencies]
- **Used by**: [List downstream consumers]
- **Port**: [If networked service, which port]

## Key Files & Entry Points

| File | Purpose |
|------|---------|
| `cmd/main.go` | Application entry point |
| `internal/service/` | Core business logic |
| `internal/tui/` | Terminal UI components |
| `go.mod` | Dependencies |

## Build, Run, Test

```bash
# Build
go build -o /tmp/[binary] ./cmd/[app]

# Run
/tmp/[binary]

# Test
go test ./...

# Lint
golangci-lint run
```

## Configuration

| Config | Location | Purpose |
|--------|----------|---------|
| `.env` | Project root | API keys, secrets |
| `config.yaml` | `~/.cortex/` | Runtime config |

## Recent Changes (Last 5)

| Date | Version | Change |
|------|---------|--------|
| 2026-01-15 | 1.0.0 | Initial production release |
| 2026-01-14 | 0.9.0 | Added bulk import feature |
| 2026-01-13 | 0.8.0 | Fixed login screen crash |

## Current Work / TODOs

- [ ] Fix sidebar display issue (#123)
- [ ] Add search functionality
- [ ] Improve error handling

## Known Issues

| Issue | Severity | Workaround |
|-------|----------|------------|
| Sidebar not rendering | Medium | Use keyboard navigation |

## For AI Agents

### When Working on This Project

1. **Build first**: Always verify the project builds before making changes
2. **Test commands**: Use `go test ./...` after changes
3. **Key patterns**: [Project-specific patterns to follow]
4. **Avoid**: [Anti-patterns specific to this project]

### Related Documentation

- [Link to CLAUDE.md section]
- [Link to relevant .shared-skills/]
- [External docs]
```

---

## Promotion Workflow

### Development → Test-UAT → Production

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│ Development │───►│  Test-UAT   │───►│ Production  │
│             │    │             │    │             │
│ - Active    │    │ - Verified  │    │ - Stable    │
│ - Unstable  │    │ - Tested    │    │ - Versioned │
│ - Rapid     │    │ - Reviewed  │    │ - Supported │
└─────────────┘    └─────────────┘    └─────────────┘
     ▲                   │                   │
     │                   │                   │
     └───────────────────┴───────────────────┘
              Rollback if issues
```

### Promotion Checklist

Before promoting from Development → Test-UAT:
- [ ] All tests pass
- [ ] PROJECT.md is up to date
- [ ] CHANGELOG.md has new entry
- [ ] No critical bugs open

Before promoting from Test-UAT → Production:
- [ ] UAT testing complete
- [ ] Version number assigned (semver)
- [ ] PRODUCTION-INDEX.md updated
- [ ] start.sh script created/updated
- [ ] _previous-versions/ backup made

### Promotion Command (to be added to Evolve-Now.sh)

```bash
./Evolve-Now.sh promote <project-name> --from development --to test-uat
./Evolve-Now.sh promote <project-name> --from test-uat --to production --version 1.0.0
```

---

## PRODUCTION-INDEX.md Structure

```markdown
# Production Applications Index

> Last updated: 2026-01-15 19:30:00

## Active Production Apps

| App | Version | Promoted | Description | Health |
|-----|---------|----------|-------------|--------|
| cortex-key-vault | 1.0.0 | 2026-01-15 | Secure secrets manager TUI | ✅ Healthy |
| cortex-brain | 2.3.0 | 2026-01-10 | AI assistant core | ✅ Healthy |

## Promotion History

### 2026-01-15
- **cortex-key-vault v1.0.0** promoted from Test-UAT
  - Added bulk import feature
  - Fixed light terminal theme
  - Reviewer: Norman King

### 2026-01-10
- **cortex-brain v2.3.0** promoted from Test-UAT
  - Added voice feature foundation
  - Improved A2A protocol handling
```

---

## .cortex-manifest.yaml (Machine-Readable Registry)

```yaml
# Cortex Ecosystem Manifest
# Machine-readable project registry for automation and AI agents

version: "1.0"
updated: "2026-01-15T19:30:00Z"

environments:
  production:
    path: /Users/normanking/ServerProjectsMac/Production
    apps:
      cortex-key-vault:
        version: "1.0.0"
        promoted: "2026-01-15"
        status: healthy
        port: null
        depends_on: [cortex-brain]
      cortex-brain:
        version: "2.3.0"
        promoted: "2026-01-10"
        status: healthy
        port: 8080
        depends_on: []

  development:
    path: /Users/normanking/ServerProjectsMac/Development
    apps:
      CortexBrain:
        priority: P0
        maturity: 95
        language: go
      dnet:
        priority: P2
        maturity: 85
        language: python

  archive:
    path: /Users/normanking/ServerProjectsMac/Archive
    reason: "deprecated or inactive"

ecosystem:
  core: cortex-brain
  first_principle: "Emulate the human brain's thinking processes"
```

---

## Integration with Claude Code

### How Claude Code Discovers Context

1. **Workspace CLAUDE.md** (existing) - Read automatically at session start
2. **Project PROJECT.md** (new) - Read when entering project directory
3. **Manifest file** (new) - Queryable for project relationships

### Enhanced CLAUDE.md Structure

Add to existing CLAUDE.md:

```markdown
## AI Agent Instructions

When working in this workspace:

1. **Find project context**: Look for `PROJECT.md` in the current directory
2. **Check environment**: Note if you're in Production/, Development/, or Archive/
3. **Verify versions**: Check PRODUCTION-INDEX.md for current production versions
4. **Follow promotion rules**: Never edit Production/ directly; changes go through Development/

### Quick Project Lookup

To understand any project, read its `PROJECT.md`:
```bash
cat Development/CortexBrain/PROJECT.md
```
```

---

## Suggestions & Improvements

### 1. Add Project Health Checks

Each PROJECT.md could include a health check command:

```yaml
health_check:
  command: "go build -o /tmp/test ./cmd/app && rm /tmp/test"
  expected: "exit 0"
```

AI agents can run this to verify project is in working state.

### 2. Standardize Change Categories

Use conventional changelog categories:
- `Added` - New features
- `Changed` - Changes in existing functionality
- `Deprecated` - Soon-to-be removed features
- `Removed` - Removed features
- `Fixed` - Bug fixes
- `Security` - Security fixes

### 3. Add Decision Log

Track architectural decisions per project:

```markdown
## Decision Log

### 2026-01-15: Use macOS Keychain for secrets
**Context**: Need secure storage for sensitive values
**Decision**: Use go-keyring with macOS Keychain backend
**Alternatives**: SQLCipher, environment variables
**Reason**: Native OS security, no additional encryption needed
```

### 4. Create Project Templates

For new projects, provide a template:

```bash
./Evolve-Now.sh new-project --name "my-app" --type go-tui
```

This creates the project with PROJECT.md, CHANGELOG.md, and standard structure.

### 5. Add Dependency Graph Generation

Auto-generate ecosystem visualization:

```bash
./Evolve-Now.sh ecosystem-map --output ECOSYSTEM-MAP.md
```

### 6. Session Context File

Create `.claude-session.md` that tracks:
- What the AI agent worked on last session
- Open tasks/issues
- Suggested next steps

---

## Implementation Plan

### Phase 1: Foundation (Immediate)

1. Create PROJECT.md template
2. Generate PROJECT.md for all existing projects
3. Create PRODUCTION-INDEX.md
4. Create .cortex-manifest.yaml
5. Update Evolve-Now.sh to create these files

### Phase 2: Automation

6. Add `promote` command to Evolve-Now.sh
7. Add `new-project` command
8. Auto-update manifest on changes
9. Add health check system

### Phase 3: Enhancement

10. Add decision log support
11. Create ecosystem map generator
12. Add session context tracking
13. Integrate with git hooks for auto-updates

---

## Files to Create

| File | Location | Purpose |
|------|----------|---------|
| `PROJECT.md` | Every project folder | AI agent context |
| `CHANGELOG.md` | Every project folder | Version history |
| `PRODUCTION-INDEX.md` | Production/ | Production app catalog |
| `PROMOTION-LOG.md` | Production/ | Promotion history |
| `.cortex-manifest.yaml` | Workspace root | Machine-readable registry |
| `DEVELOPMENT-STATUS.md` | Development/ | Active work tracker |
| `TEST-QUEUE.md` | Test-UAT/ | Apps awaiting promotion |

---

## Next Steps

1. **Review this plan** - Approve or suggest changes
2. **Run enhanced Evolve-Now.sh** - Creates all documentation files
3. **Populate PROJECT.md files** - Either auto-generated or manually refined
4. **Test AI agent experience** - Start new Claude Code session and verify context loading

---

*"The best documentation is the documentation that AI agents can actually use."*
