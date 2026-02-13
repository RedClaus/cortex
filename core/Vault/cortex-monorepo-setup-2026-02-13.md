# Cortex Monorepo Setup

**Date:** 2026-02-13
**Status:** Complete
**GitHub:** https://github.com/RedClaus/cortex

## Overview

Consolidated the Cortex ecosystem into a unified monorepo at `/Users/normanking/ServerProjectsMac/cortex/`. All other projects under ServerProjectsMac are now archived.

## Monorepo Structure

```
/Users/normanking/ServerProjectsMac/cortex/
├── go.work              # Go workspace linking all modules
├── Makefile             # Build orchestration
├── .gitignore           # Excludes venv, node_modules, secrets
├── core/                # CortexBrain engine
│   ├── pkg/agent/       # Agent Layer (BrainInterface, Router, etc.)
│   ├── pkg/brain/       # Brain architecture (lobes, executive)
│   ├── internal/a2a/    # A2A server (Pinky compatibility)
│   ├── internal/memory/ # Memory systems
│   └── Vault/           # Knowledge vault
├── apps/                # Application suite
│   ├── pinky/           # AI assistant TUI
│   ├── cortex-gateway/  # API gateway
│   ├── cortex-coder-agent/
│   ├── cortex-avatar/
│   ├── cortex-lab/
│   ├── cortex-key-vault/
│   ├── salamander/
│   ├── go-menu/
│   └── cortex-evaluator/
├── docs/                # Documentation
├── PRDs/                # Product Requirements
├── Gateway/             # Gateway planning docs
├── Design/              # Design documents
└── Evaluations/         # Tool evaluations
```

## Key References

| Item | Path |
|------|------|
| CortexBrain | `/Users/normanking/ServerProjectsMac/cortex/core/` |
| Agent Layer | `/Users/normanking/ServerProjectsMac/cortex/core/pkg/agent/` |
| Vault | `/Users/normanking/ServerProjectsMac/cortex/core/Vault/` |
| Monorepo Root | `/Users/normanking/ServerProjectsMac/cortex/` |
| Go Workspace | `/Users/normanking/ServerProjectsMac/cortex/go.work` |

## Git Setup

- **Remote:** `origin` → `https://github.com/RedClaus/cortex.git`
- **Branch:** `main`
- **Initial Commit:** `1dac2db` - 2,887 files, 635,829 insertions

## Notes

- Config files with API keys excluded from git (`.gitignore`)
- Python venv and node_modules excluded
- Go workspace (`go.work`) links all modules for unified development

## Related Documents

- [Agent Layer Implementation](./agent-layer-implementation-2026-02-13.md)
- [PRD: Agentic Brain System](./PRD-AGENTIC-BRAIN-SYSTEM.md)
