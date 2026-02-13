---
project: Cortex
component: Docs
phase: Ideation
date_created: 2026-02-12T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-12T02:01:56.693274
---

# Migration Map: ServerProjectsMac → cortex/

Date: 2026-02-12
Status: Phase 2 Complete (Migration Active)

## Overview

This document serves as the definitive reference for the ServerProjectsMac → cortex/ monorepo migration. It maps every original location to its new home in the consolidated workspace.

---

## Source → Destination Table

### Core Engine

| Original Location | New Location | Module | Status |
|---|---|---|---|
| `CortexBrain/` | `cortex/core/` | `github.com/normanking/cortex` | ✅ Migrated |

---

### P0/P1 Applications

| Original Location | New Location | Module | Status |
|---|---|---|---|
| `Pinky/` | `cortex/apps/pinky/` | `github.com/normanking/pinky` | ✅ Migrated |
| `cortex-gateway-test/` | `cortex/apps/cortex-gateway/` | `github.com/cortexhub/cortex-gateway` | ✅ Migrated |
| `cortex-coder-agent-test/` | `cortex/apps/cortex-coder-agent/` | `github.com/RedClaus/cortex-coder-agent` | ✅ Migrated |
| `Development/cortex-avatar/` | `cortex/apps/cortex-avatar/` | `github.com/normanking/cortexavatar` | ✅ Migrated |
| `Development/CortexLab/` | `cortex/apps/cortex-lab/` | `github.com/normanking/cortexlab` | ✅ Migrated |
| `Production/cortex-key-vault/` | `cortex/apps/cortex-key-vault/` | `github.com/normanking/cortex-key-vault` | ✅ Migrated |

---

### P2/P3 Applications

| Original Location | New Location | Module | Status |
|---|---|---|---|
| `Development/Salamander/` | `cortex/apps/salamander/` | `github.com/normanking/salamander` | ✅ Migrated |
| `Development/go-menu/` | `cortex/apps/go-menu/` | `gomenu` | ✅ Migrated |
| `Development/cortex-evaluator/` | `cortex/apps/cortex-evaluator/` | `github.com/cortex-evaluator/cortex-evaluator` | ✅ Migrated |

---

## Documentation Migration

### Architecture & Strategy

| Original Location | New Location | Notes |
|---|---|---|
| `CORTEX-README.md` | `cortex/docs/architecture/CORTEX-README.md` | Ecosystem overview |
| `CORTEX-ECOSYSTEM-ROADMAP.md` | `cortex/docs/architecture/CORTEX-ECOSYSTEM-ROADMAP.md` | 6-month roadmap |
| `CORTEX-DEVELOPMENT-PLANS.md` | `cortex/docs/architecture/CORTEX-DEVELOPMENT-PLANS.md` | Per-project plans |
| `CORTEX-ORGANIZATION-SUMMARY.md` | `cortex/docs/architecture/CORTEX-ORGANIZATION-SUMMARY.md` | Executive summary |
| `CORTEX-DOCS.md` | `cortex/docs/architecture/CORTEX-DOCS.md` | Documentation index |
| `CORTEX-AI-DOCUMENTATION-PLAN.md` | `cortex/docs/architecture/CORTEX-AI-DOCUMENTATION-PLAN.md` | Documentation strategy |
| `CORTEX.rules` | `cortex/docs/architecture/CORTEX.rules` | Project rules |
| `SSH-BLOCKED-CRITICAL.md` | `cortex/docs/architecture/SSH-BLOCKED-CRITICAL.md` | Infrastructure notes |
| `BACKUP-LOCATION.md` | `cortex/docs/architecture/BACKUP-LOCATION.md` | Backup strategy |
| `ServerProjectsMac_Taxonomy.md` | `cortex/docs/architecture/ServerProjectsMac_Taxonomy.md` | Project taxonomy |
| `LIBRARIAN-DOCUMENTATION-POLICY.md` | `cortex/docs/architecture/LIBRARIAN-DOCUMENTATION-POLICY.md` | Documentation policy |
| `Cortex/Brain-Overview.md` | `cortex/docs/architecture/Brain-Overview.md` | Brain architecture |
| `Cortex/AGENT-CODING-GUIDELINES.md` | `cortex/docs/architecture/AGENT-CODING-GUIDELINES.md` | Coding standards |

### Research & Implementation

| Original Location | New Location | Notes |
|---|---|---|
| `OLLAMA-*.md` (9 files) | `cortex/research/implementation/ollama/` | Ollama integration research |
| `PERFORMANCE_*.md` | `cortex/research/implementation/performance/` | Performance research |
| `OPTIMIZATION_GUIDE.md` | `cortex/research/implementation/performance/OPTIMIZATION_GUIDE.md` | Optimization guide |
| `INTELLIGENT-MODEL-PICKER-PROPOSAL.md` | `cortex/research/implementation/INTELLIGENT-MODEL-PICKER-PROPOSAL.md` | AutoLLM routing research |
| `RTX3090-CODING-MODEL-RESEARCH.md` | `cortex/research/implementation/RTX3090-CODING-MODEL-RESEARCH.md` | Hardware/model research |

### Domain-Specific Documentation

| Original Location | New Location | Notes |
|---|---|---|
| `Cortex/Analysis/` | `cortex/docs/Analysis/` | Analysis documents |
| `Cortex/Design/` | `cortex/docs/Design/` | Design documents |
| `Cortex/Evaluations/` | `cortex/docs/Evaluations/` | Evaluation reports |
| `Cortex/Gateway/` | `cortex/docs/Gateway/` | Gateway architecture & planning |
| `Cortex/PRDs/` | `cortex/docs/PRDs/` | Product requirements documents |

---

## Archived Directories

These directories are preserved in `cortex/archive/` for historical reference and debugging purposes:

| Original Location | Archive Location | Reason |
|---|---|---|
| `cortex-brain-test/` | `cortex/archive/cortex-brain-test/` | Test variant of CortexBrain |
| `cortex-brain-git/` | `cortex/archive/cortex-brain-git/` | Git experiment variant |
| `Pinky-Test/` | `cortex/archive/Pinky-Test/` | Test variant of Pinky |
| `Test-UAT/` | `cortex/archive/Test-UAT/` | UAT test environment |
| `cortex-teacher/` | `cortex/archive/cortex-teacher/` | Experimental, never shipped |

### Not Copied (Size/Deprecation)

| Original Location | Reason | Action |
|---|---|---|
| `Cortex-v1/` | Deprecated predecessor (too large) | Reference via git tags if needed |
| `Development/cortex-02/` | Earlier iteration (too large) | Archive in git history |
| `Development/cortex-03/` | Earlier iteration (too large) | Archive in git history |
| `Development/cortex-workshop/` | Experimental workshop | Reference notes in ADR docs |
| `Development/cortex-unified/` | Merge experiment | Architecture decisions captured |
| `Development/cortex-assistant/` | Earlier concept | Design captured in docs |
| `Development/TEST-Cortex-Assistant/` | Test variant | Not critical path |
| `Development/gotui-sandbox/` | TUI experiments | Code patterns referenced |

---

## Stays Outside Monorepo

These remain separate repositories due to toolchain, ownership, or strategic reasons:

| Directory | Reason | Status |
|---|---|---|
| `Development/dnet/` | Python/MLX project, separate CI/CD | Maintain as independent repo |
| `Development/TermAi-archive/` | TypeScript/Electron, separate stack | Reference via git submodule if needed |
| `ui-ux-pro-max-skill/` | Standalone design toolkit | Maintain as independent repo |
| `DigitalHoldingsGroup/` | Separate business entity | Outside scope |
| `Infrastructure/` | Infrastructure configuration | Separate Terraform/IaC repo |
| `ecosystem-strategy/` | Strategic planning docs | Maintain separately |
| `CLAUDE.md` | Workspace-level AI instructions | Root-level file |

---

## Directory Structure (cortex/)

```
cortex/
├── README.md                          # Monorepo entry point
├── go.work                            # Go 1.25.5 workspace file
├── go.work.sum
│
├── core/                              # CortexBrain (P0)
│   ├── go.mod
│   ├── go.sum
│   ├── cmd/
│   ├── internal/
│   └── pkg/
│
├── apps/                              # P0/P1/P2 Applications
│   ├── pinky/                         # P0 - Assistant
│   │   ├── go.mod
│   │   └── ...
│   ├── cortex-gateway/                # P0 - API Gateway
│   ├── cortex-coder-agent/            # P0 - Code Agent
│   ├── cortex-avatar/                 # P1 - Desktop Companion
│   ├── cortex-lab/                    # P1 - Component Incubator
│   ├── cortex-key-vault/              # P1 - Secrets Management
│   ├── salamander/                    # P2 - YAML TUI Framework
│   ├── go-menu/                       # P2 - Menu Bar App
│   └── cortex-evaluator/              # P2 - Evaluation Suite
│
├── docs/
│   ├── MIGRATION-MAP.md               # THIS FILE
│   ├── architecture/                  # Strategic & architectural docs
│   │   ├── CORTEX-README.md
│   │   ├── CORTEX-ECOSYSTEM-ROADMAP.md
│   │   ├── CORTEX-DEVELOPMENT-PLANS.md
│   │   ├── Brain-Overview.md
│   │   ├── AGENT-CODING-GUIDELINES.md
│   │   └── ...
│   ├── Analysis/                      # Analysis documents
│   ├── Design/                        # Design specifications
│   ├── Evaluations/                   # Evaluation reports
│   ├── Gateway/                       # Gateway planning
│   ├── PRDs/                          # Product requirements
│   └── README.md                      # Docs index
│
├── research/
│   ├── implementation/
│   │   ├── ollama/                    # Ollama integration research
│   │   ├── performance/               # Performance research
│   │   ├── INTELLIGENT-MODEL-PICKER-PROPOSAL.md
│   │   └── RTX3090-CODING-MODEL-RESEARCH.md
│   └── README.md
│
├── archive/                           # Historical/test variants
│   ├── cortex-brain-test/
│   ├── cortex-brain-git/
│   ├── Pinky-Test/
│   ├── Test-UAT/
│   └── cortex-teacher/
│
├── scripts/
│   ├── cleanup-originals.sh           # Safe removal of originals
│   ├── verify-migration.sh            # Validation script
│   └── ...
│
└── .shared-skills/                    # Symlinked to all Go projects
    └── (see CLAUDE.md in parent)
```

---

## go.work Module Map

The `go.work` file coordinates all Go modules:

```go
go 1.25.5

use (
	./core
	./apps/pinky
	./apps/cortex-gateway
	./apps/cortex-coder-agent
	./apps/cortex-avatar
	./apps/cortex-lab
	./apps/cortex-key-vault
	./apps/salamander
	./apps/go-menu
	./apps/cortex-evaluator
)
```

Each module maintains its own `go.mod`:

| Module | Import Path | Location |
|--------|-------------|----------|
| cortex (core) | `github.com/normanking/cortex` | `cortex/core/go.mod` |
| Pinky | `github.com/normanking/pinky` | `cortex/apps/pinky/go.mod` |
| Cortex Gateway | `github.com/cortexhub/cortex-gateway` | `cortex/apps/cortex-gateway/go.mod` |
| Cortex Coder Agent | `github.com/RedClaus/cortex-coder-agent` | `cortex/apps/cortex-coder-agent/go.mod` |
| CortexAvatar | `github.com/normanking/cortexavatar` | `cortex/apps/cortex-avatar/go.mod` |
| CortexLab | `github.com/normanking/cortexlab` | `cortex/apps/cortex-lab/go.mod` |
| Cortex Key Vault | `github.com/normanking/cortex-key-vault` | `cortex/apps/cortex-key-vault/go.mod` |
| Salamander | `github.com/normanking/salamander` | `cortex/apps/salamander/go.mod` |
| GoMenu | `gomenu` | `cortex/apps/go-menu/go.mod` |
| Cortex Evaluator | `github.com/cortex-evaluator/cortex-evaluator` | `cortex/apps/cortex-evaluator/go.mod` |

---

## Migration Verification

After the migration, verify correctness:

```bash
# Check all modules can be resolved
cd cortex
go work sync

# Build all applications
go build -v ./core/cmd/cortex
go build -v ./apps/pinky/...
go build -v ./apps/cortex-gateway/...
# ... etc for all apps

# Run tests
go test -v ./core/...
go test -v ./apps/pinky/...
# ... etc for all apps
```

---

## Cleanup Instructions

**After confirming the monorepo works correctly**, remove the original directories:

```bash
# This script safely removes migrated directories
./cortex/scripts/cleanup-originals.sh

# Or manually (be careful!):
rm -rf /path/to/original/CortexBrain
rm -rf /path/to/original/Pinky
rm -rf /path/to/original/Development/cortex-avatar
# ... etc
```

**WARNING:** Do not run cleanup until you have verified:
1. All applications build successfully from new locations
2. All tests pass
3. All git history is preserved (if using git)
4. Backups exist

---

## FAQ

### Q: Where do I find CortexBrain code?
**A:** `cortex/core/` (formerly `CortexBrain/`)

### Q: Where do I find Pinky code?
**A:** `cortex/apps/pinky/` (formerly `Pinky/`)

### Q: Where is the documentation?
**A:** `cortex/docs/architecture/` for strategic docs, `cortex/research/` for research

### Q: Can I still access old code?
**A:** Yes, test/archived variants are in `cortex/archive/`. Fully deprecated projects (Cortex-v1, etc.) are not included but can be referenced via git history.

### Q: How do I build a specific app?
**A:** From `cortex/` root:
```bash
go build -o /tmp/app-name ./apps/app-name/cmd/...
```

Or use the app's README in its own directory.

### Q: What about dnet and TermAi?
**A:** These stay as separate repositories due to different toolchains (Python/MLX, TypeScript/Electron). Reference them as external dependencies or git submodules as needed.

### Q: How do imports work now?
**A:** Use the module paths in the `go.mod` files. Cross-app imports work via `go.work` coordination.

Example:
```go
import "github.com/normanking/cortex/pkg/somepackage"
```

---

## Contact & Questions

For questions about this migration or finding specific code:
- Check the `cortex/docs/architecture/` directory for strategic context
- Review individual app READMEs in `cortex/apps/*/`
- Consult `CORTEX-DOCS.md` for documentation strategy
- See `CORTEX.rules` for development standards
