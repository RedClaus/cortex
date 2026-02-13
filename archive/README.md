---
project: Cortex
component: Docs
phase: Archive
date_created: 2026-02-12T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-12T02:01:56.680349
---

# Cortex Archive

Central repository for archived projects, test variants, and deprecated code.

## Quick Navigation

### Essential Documentation

- **[ARCHIVE-MANIFEST.md](ARCHIVE-MANIFEST.md)** — Complete archival strategy
  - Full inventory of what was archived and why
  - 4-phase archival plan (Phases 1-4)
  - Safety guidelines and cleanup procedures

- **[PHASE1-SUMMARY.md](PHASE1-SUMMARY.md)** — Phase 1 completion status
  - What was archived: Pinky-Test/, Test-UAT/, cortex-teacher/
  - What's pending: Cleanup of original directories
  - Optional phases (2-4) for additional archival

### Cleanup & Maintenance

- **Script:** `../scripts/cleanup-originals.sh`
  - Safe deletion of original directories
  - Interactive confirmations for each deletion
  - Uses `trash` on macOS, `rm -rf` on Linux
  - Executable: `chmod +x cortex/scripts/cleanup-originals.sh`

### Archived Directories

| Directory | Size | Status |
|-----------|------|--------|
| `Pinky-Test/` | 30M | ✓ Archived Phase 1 |
| `Test-UAT/` | 12K | ✓ Archived Phase 1 |
| `cortex-teacher/` | 44K | ✓ Archived Phase 1 |

**Total Archived:** 30M

---

## Archival Status

### Phase 1: Complete ✓
Small test variants safely archived:
- Pinky-Test (30M)
- Test-UAT (12K)
- cortex-teacher (44K)

### Phase 2: Pending
Large test variants (optional):
- cortex-brain-test (397M)
- cortex-brain-git (143M)
- cortex-gateway-test (232M)
- cortex-coder-agent-test (19M)
- claude_code_agent (342M)

**Total Candidates:** ~1.1GB

### Phase 3: Pending
Development folder early versions (optional):
- cortex-02, cortex-03, cortex-workshop, cortex-unified, cortex-assistant, TEST-Cortex-Assistant, gotui-sandbox

**Total Candidates:** ~5GB+

### Phase 4: Pending
Root level old projects (optional):
- CortexBrain/
- Pinky/
- Related consolidations

---

## Key Facts

### What's Archived
- **Test variants** of projects now in cortex/core/ and cortex/apps/
- **Earlier iterations** of CortexBrain (v1, v2, v3)
- **Experimental projects** with no go.mod or active development
- **UAT environments** and git experiments

### What's NOT Archived
These projects stay outside the archive (separate purposes):
- `dnet/` — Python project, separate toolchain
- `ui-ux-pro-max-skill/` — Shared design toolkit
- `TermAi-archive/` — TypeScript/Electron ecosystem
- `DigitalHoldingsGroup/` — Separate business entity
- `Infrastructure/` — DevOps configuration
- `ecosystem-strategy/` — Reference documentation

### Why Archive?
- **Reduce clutter** in root workspace
- **Improve build times** (fewer dirs to scan)
- **Preserve history** (originals are backed up)
- **Safer cleanup** (clear inventory before deletion)

---

## Usage

### Review Before Cleanup
```bash
cd /path/to/ServerProjectsMac
cat cortex/archive/ARCHIVE-MANIFEST.md
```

### Delete Original Test Directories
```bash
cd /path/to/ServerProjectsMac
chmod +x cortex/scripts/cleanup-originals.sh
./cortex/scripts/cleanup-originals.sh
```

The script will:
1. List each directory to be deleted
2. Show its size and purpose
3. Ask for confirmation before deletion
4. Provide progress feedback

### Add More to Archive
Future phases can archive additional directories:
- Phase 2: Large test variants (use cleanup script)
- Phase 3: Development folder iterations (use cleanup script)
- Phase 4: Root level old projects (use cleanup script)

See `ARCHIVE-MANIFEST.md` for complete details.

---

## Safety Checklist

Before running the cleanup script:

- [ ] Reviewed `ARCHIVE-MANIFEST.md`
- [ ] Confirmed cortex monorepo builds: `cd cortex && make build`
- [ ] Tested core applications (CortexBrain, Pinky, etc.)
- [ ] Verified no active Git branches reference deleted paths
- [ ] Backed up important directories if needed
- [ ] Checked that `cortex/core/` contains all consolidated projects

---

## Support

**Questions?**
1. Check `ARCHIVE-MANIFEST.md` for comprehensive documentation
2. Run `./cortex/scripts/cleanup-originals.sh` with `--help` (if available)
3. Review `PHASE1-SUMMARY.md` for quick status

**Issues?**
- The cleanup script has confirmations for each deletion
- Deleted files moved to Trash on macOS (recoverable)
- Permanently deleted on Linux (use with caution)
- See safety guidelines in `ARCHIVE-MANIFEST.md`

---

**Last Updated:** 2026-02-12  
**Phase:** Phase 1 Complete, Phases 2-4 Pending
