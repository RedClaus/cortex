---
project: Cortex
component: Unknown
phase: Ideation
date_created: 2026-01-16T11:58:54
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:18:06.665738
---

# CLAUDE.md

This file provides guidance to Claude Code when working with the CortexLab codebase.

---

## Project Overview

[Project description - to be filled in]

---

## Development Commands

```bash
# Build
[build commands]

# Test
[test commands]

# Run
[run commands]
```

---

## Code Patterns

[Key patterns and conventions for this project]

---

## Software Manufacturing Process

This project follows a structured software manufacturing process to ensure quality and consistency across the full development lifecycle.

**Full specification:** See [`.claude/process/PROCESS.md`](.claude/process/PROCESS.md)

### Quick Reference

| Phase | Purpose | Key Artifacts |
|-------|---------|---------------|
| Discovery | Understand the problem | requirements.md, success-criteria.md |
| Design | Define the solution | architecture.md, API contracts, ADRs |
| Implementation | Build the solution | Source code, unit tests |
| Testing | Verify correctness | Test plan, coverage reports |
| Deployment | Release safely | Release notes, rollback plan |
| Operations | Monitor and maintain | Incident logs, retrospectives |

### Process Commands

| Command | Action |
|---------|--------|
| `process:status` | Show current phase and gate status |
| `process:advance` | Validate gates and transition to next phase |
| `process:init [name]` | Initialize directory structure for new feature |
| `process:gate-check` | Audit artifacts against current phase |

### State Tracking

Process state is maintained in `.claude/process/state.json`. Always check current phase status before beginning work on a feature.
