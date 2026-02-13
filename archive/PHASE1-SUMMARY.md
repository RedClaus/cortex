---
project: Cortex
component: Unknown
phase: Archive
date_created: 2026-02-12T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-12T02:01:56.672542
---

# Phase 1 Archival Summary

**Completed:** 2026-02-12  
**Phase:** Selective Archival of Small Test Variants

## Archived Directories

Successfully copied the following small test variants into `cortex/archive/`:

| Directory | Size | Status | Original |
|-----------|------|--------|----------|
| `Pinky-Test/` | 30M | ✓ Archived | `cortex/core/pinky/` |
| `Test-UAT/` | 12K | ✓ Archived | — |
| `cortex-teacher/` | 44K | ✓ Archived | — |

**Total Archived:** 30M (30.056 MB)

## What Happened

1. Created `ARCHIVE-MANIFEST.md` documenting all candidates for archival
2. Created `cortex/scripts/cleanup-originals.sh` for safe deletion of originals
3. Copied small test variants (< 50MB) into `cortex/archive/`
4. Excluded `.git`, `node_modules`, and `.DS_Store` during copy

## Next Steps

### Option A: Delete Original Test Directories (Immediate)
Use the cleanup script to safely remove original directories:

```bash
cd /sessions/wizardly-inspiring-archimedes/mnt/ServerProjectsMac
chmod +x cortex/scripts/cleanup-originals.sh
./cortex/scripts/cleanup-originals.sh
```

The script will:
- Confirm each deletion before proceeding
- Use `trash` command if available (macOS)
- Otherwise use `rm -rf` for permanent deletion
- Provide rollback suggestions

### Option B: Large Test Variants (Optional)
Additional test variants are candidates for archival if space is a concern:

- `cortex-brain-test/` (397M)
- `cortex-brain-git/` (143M)
- `cortex-gateway-test/` (232M)
- `cortex-coder-agent-test/` (19M)
- `claude_code_agent/` (342M)

**Estimated Total:** ~1.1GB

### Option C: Development Folder Early Versions (Optional)
Archive earlier CortexBrain iterations:

- `Development/cortex-02/` (1.5GB+)
- `Development/cortex-03/` (1.8GB+)
- `Development/cortex-workshop/`, `cortex-unified/`, `cortex-assistant/`, etc.

**Estimated Total:** ~5GB+

See `ARCHIVE-MANIFEST.md` for full details.

## Safety Checklist

Before deleting originals:

- [ ] Confirmed cortex monorepo is building: `cd cortex && make build`
- [ ] Tested core applications: CortexBrain, Pinky, Gateway, etc.
- [ ] Verified no active Git branches reference deleted paths
- [ ] Backed up important directories if needed
- [ ] Checked that `cortex/core/` contains all consolidated projects

## Files Created

| File | Purpose |
|------|---------|
| `cortex/archive/ARCHIVE-MANIFEST.md` | Complete archival documentation |
| `cortex/scripts/cleanup-originals.sh` | Safe deletion script (executable) |
| `cortex/archive/PHASE1-SUMMARY.md` | This file — Phase 1 completion summary |
| `cortex/archive/Pinky-Test/` | Archived test copy of Pinky |
| `cortex/archive/Test-UAT/` | Archived UAT test environment |
| `cortex/archive/cortex-teacher/` | Archived experimental cortex-teacher |

## Current Status

- Phase 1 (Documentation & Selective Archival): **✓ COMPLETE**
- Phase 2 (Delete Test Directories): **PENDING** — Use `cleanup-originals.sh`
- Phase 3 (Development Folder): **OPTIONAL** — See manifest for details
- Phase 4 (Root Level Projects): **OPTIONAL** — See manifest for details

---

**Questions?** See `ARCHIVE-MANIFEST.md` for comprehensive documentation.
