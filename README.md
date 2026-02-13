---
project: Cortex
component: Docs
phase: Design
date_created: 2026-02-11T21:01:46
source: ServerProjectsMac
librarian_indexed: 2026-02-11T22:46:24.298266
---

# Cortex Monorepo

> Emulate the human brain's thinking processes through simulated cognitive modules.

## Structure

```
cortex/
├── core/           CortexBrain engine (20 cognitive lobes, AutoLLM, A2A server)
├── apps/           Plugin applications that snap onto CortexBrain
│   └── _template/  Scaffold for new plugins
├── docs/
│   ├── adr/        Architecture Decision Records
│   ├── rfc/        Requests for Comments
│   └── architecture/  System diagrams and overviews
├── research/       Dated research notes (evaluations, benchmarks, prototypes)
├── scripts/        Automation scripts
├── tools/          Internal tooling
├── archive/        Deprecated projects
└── .github/        CI/CD workflows
```

## Quick Start

```bash
# Build everything
make build

# Test everything
make test

# Build CortexBrain binary
make brain

# Scaffold a new plugin
make new-app NAME=cortex-your-name

# Create a new ADR
make new-adr TITLE="your decision title"

# See all commands
make help
```

## First Principles

Every decision must pass these inviolable constraints:

1. **Single Binary** — CortexBrain ships as one executable
2. **Apple Silicon Primary** — Optimized for M-series chips
3. **Local-First Privacy** — No data leaves the machine without consent
4. **Go Only (core)** — Core engine is pure Go; apps may use other languages
5. **Memory < 500MB** — Total runtime memory budget

## Dependency Rules

```
✅  apps/*  →  core/              (allowed)
✅  apps/*  →  apps/cortex-lab    (allowed — shared kernel)
✅  apps/*  →  external libs      (allowed, ADR for significant deps)
❌  apps/*  →  apps/other-app     (never — use A2A protocol)
❌  core/   →  apps/*             (never)
```

## Documentation

- **Architecture Decisions:** `docs/adr/`
- **Research Notes:** `research/`
- **Contributing:** See [CONTRIBUTING.md](../ecosystem-strategy/guides/CONTRIBUTING.md)
- **Roadmap:** See [ROADMAP.md](../ecosystem-strategy/templates/ROADMAP.md)
