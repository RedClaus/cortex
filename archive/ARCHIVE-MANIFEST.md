---
project: Cortex
component: Unknown
phase: Archive
date_created: 2026-02-12T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-12T02:01:56.664743
---

# Cortex Archive Manifest

**Created:** 2026-02-12
**Status:** Documentation of archived projects and cleanup candidates
**Purpose:** Track deprecated, test, and superseded projects for safe archival

---

## Archive Index

### Test Variants (Safe to Archive — Originals Already in cortex/)

These are test copies, git experiments, and UAT environments of projects that have been consolidated into the cortex monorepo.

| Directory | Size | Status | Reason | Original Location |
|-----------|------|--------|--------|-------------------|
| `cortex-brain-test/` | 397M | Dead | Test copy of CortexBrain | `cortex/core/cortex-brain/` |
| `cortex-brain-git/` | 143M | Dead | Git experiment copy of CortexBrain | `cortex/core/cortex-brain/` |
| `Pinky-Test/` | 30M | Dead | Test copy of Pinky assistant | `cortex/core/pinky/` |
| `cortex-gateway-test/` | 232M | Dead | Source already migrated | `cortex/apps/cortex-gateway/` |
| `cortex-coder-agent-test/` | 19M | Dead | Source already migrated | `cortex/apps/cortex-coder-agent/` |
| `Test-UAT/` | 12K | Dead | UAT test environment | — |
| `cortex-teacher/` | 52K | Dead | Experimental, no go.mod | — |
| `claude_code_agent/` | 342M | Dead | Legacy code agent implementation | — |

**Total Size of Test Variants:** ~1.2GB

**Safe to Archive:** Yes. These are clearly test/experiment copies where the actual source code has been migrated to the cortex monorepo.

---

### Earlier Iterations (Superseded by CortexBrain v2+)

These are earlier versions of CortexBrain and related projects that have been superseded by v2+ architecture.

#### Root Level

| Directory | Size | Status | Reason |
|-----------|------|--------|--------|
| `Cortex-v1/` | — | Archived | Predecessor to CortexBrain; deprecated |

**Location:** Already exists at root level.

#### Development Folder (`Development/`)

| Directory | Size | Status | Reason |
|-----------|------|--------|--------|
| `cortex-02/` | 1.5GB+ | Dead | Earlier CortexBrain iteration |
| `cortex-03/` | 1.8GB+ | Dead | Earlier CortexBrain iteration |
| `cortex-workshop/` | — | Dead | Experimental workshop |
| `cortex-unified/` | — | Dead | Merge experiment |
| `cortex-assistant/` | — | Dead | Earlier assistant concept |
| `TEST-Cortex-Assistant/` | — | Dead | Test variant of cortex-assistant |
| `gotui-sandbox/` | — | Dead | TUI experiments (patterns now in CortexBrain) |

**Safe to Archive:** Yes. CortexBrain v2+ in cortex/core/cortex-brain/ is the canonical version.

---

### Already Archived (Pre-existing)

These directories were already archived before this manifest was created.

| Directory | Location |
|-----------|----------|
| `_archive/` | Root level |
| `Archive/` | Root level |

**Action:** No further action needed for these. They are already segregated.

---

### Stays Outside Monorepo (NOT Archived)

These projects have reasons to remain separate and should NOT be moved into cortex/archive/:

| Directory | Reason |
|-----------|--------|
| `Development/dnet/` | Python project (not Go, separate toolchain) |
| `Development/TermAi-archive/` | TypeScript/Electron rewrite (separate ecosystem) |
| `Development/ui-ux-pro-max-skill/` | Design toolkit; referenced as shared skill |
| `DigitalHoldingsGroup/` | Separate business entity |
| `Infrastructure/` | Infrastructure/DevOps configuration |
| `ecosystem-strategy/` | Migration strategy documentation (reference) |
| `CLAUDE.md` | Workspace instructions for Claude Code |

**Action:** None. These serve external purposes or are referenced as tools.

---

### Root Scaffold (Review Recommended)

**Location:** Root files and directories

| Item | Type | Status | Notes |
|------|------|--------|-------|
| `go.mod` (module "changeme") | File | Active | Original Wails template scaffold |
| `main.go` | File | Active | Original Wails template |
| `build/` | Directory | Active | Original Wails build output |
| `examples/` | Directory | Active | Original Wails examples |

**Status:** These are part of the original Wails scaffold and should be reviewed before any action is taken. They may be placeholders or templates.

---

## Consolidation Status by Project

### CortexBrain
- **Current Location:** `cortex/core/cortex-brain/`
- **Test Copies to Archive:** `cortex-brain-test/`, `cortex-brain-git/`
- **Earlier Versions to Archive:** `Development/cortex-02/`, `Development/cortex-03/`
- **Status:** ✓ Consolidated

### Pinky
- **Current Location:** `cortex/core/pinky/`
- **Test Copy to Archive:** `Pinky-Test/`
- **Status:** ✓ Consolidated

### cortex-gateway
- **Current Location:** `cortex/apps/cortex-gateway/`
- **Test Copy to Archive:** `cortex-gateway-test/`
- **Status:** ✓ Consolidated

### cortex-coder-agent
- **Current Location:** `cortex/apps/cortex-coder-agent/`
- **Test Copy to Archive:** `cortex-coder-agent-test/`
- **Status:** ✓ Consolidated

### CortexLab, Salamander, GoMenu, cortex-avatar, cortex-evaluator
- **Current Location:** `cortex/core/` and `cortex/apps/`
- **Earlier Versions to Archive:** Various `Development/` folders
- **Status:** ✓ Consolidated

---

## Archival Strategy

### Phase 1: Documentation (DONE)
This manifest documents all candidates for archival.

### Phase 2: Selective Archival (PENDING)
Copy small test variants into `cortex/archive/` (< 50MB each):
- `Pinky-Test/`
- `Test-UAT/`
- `cortex-teacher/`

**Reason:** These are clearly dead with no active references.

### Phase 3: Large Test Variants (OPTIONAL)
Larger test variants (100MB+) may be archived to reduce root-level clutter:
- `cortex-brain-test/` (397M)
- `cortex-brain-git/` (143M)
- `cortex-gateway-test/` (232M)
- `cortex-coder-agent-test/` (19M)
- `claude_code_agent/` (342M)

**Reason:** Space optimization; originals are in monorepo. **Note:** Requires `cleanup-originals.sh` script for safe deletion.

### Phase 4: Development Folder Archival (OPTIONAL)
Archive earlier iterations from `Development/`:
- `cortex-02/`, `cortex-03/`, `cortex-workshop/`, `cortex-unified/`
- `cortex-assistant/`, `TEST-Cortex-Assistant/`, `gotui-sandbox/`

**Reason:** Superseded by v2+ architecture. **Note:** `Development/` folder size is significant; plan accordingly.

---

## Cleanup Safety Guidelines

1. **Always Verify Original:** Before deleting a test variant, verify the canonical version exists in `cortex/`.
2. **Backup First:** If archiving large directories, ensure backups exist.
3. **Check Git History:** Ensure no active branches reference the deleted paths.
4. **Use Cleanup Script:** Use `cortex/scripts/cleanup-originals.sh` for safe removal.
5. **Test in Stages:** Archive and delete in phases, not all at once.

---

## File Locations

| File | Purpose |
|------|---------|
| `cortex/archive/ARCHIVE-MANIFEST.md` | This file — Archival strategy and tracking |
| `cortex/scripts/cleanup-originals.sh` | Safe deletion script for original directories |

---

## Appendix: Directory Size Summary

### Test Variants Total Size
```
cortex-brain-test     397M
cortex-brain-git      143M
Pinky-Test             30M
cortex-gateway-test   232M
cortex-coder-agent-test 19M
Test-UAT               12K
cortex-teacher         52K
claude_code_agent     342M
─────────────────────────
Total: ~1.2GB
```

### Development Folder Early Versions (Estimated)
```
cortex-02            1.5GB+
cortex-03            1.8GB+
cortex-workshop      100MB+
cortex-unified       100MB+
cortex-assistant      200MB+
gotui-sandbox         100MB+
─────────────────────────
Total: ~5GB+ (estimate)
```

---

## Approval & Tracking

- **Manifest Created:** 2026-02-12
- **Reviewed by:** Claude Code
- **Status:** Ready for Phase 2 (Selective Archival)

Next step: Run selective archival of small test variants using the bash commands provided.
