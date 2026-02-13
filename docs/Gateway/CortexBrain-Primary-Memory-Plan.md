---
project: Cortex
component: Brain Kernel
phase: Ideation
date_created: 2026-02-01T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:29.809402
---

# CortexBrain Primary Memory Migration Plan
**Author:** Albert | **Date:** 2026-02-01 | **Status:** DRAFT — Awaiting Harold Review

## Objective
Promote CortexBrain (Pink:18892) from backup copy-store to **primary memory system** for Albert and Harold. Flat files become the disaster recovery export, not the source of truth.

---

## Phase 1: Real-Time Write-Through (Harold Implements)

### 1.1 — Upgrade `ch` CLI with Write-Through Commands
Current `ch store/pin/learn` already writes to CortexBrain. Needs:
- **New command: `ch log <content>`** — Stores episodic memory AND appends to flat file backup simultaneously
- **New command: `ch export`** — Dumps CortexBrain memories → flat files (reverse of current sync)
- **Verify idempotency** — Re-storing the same content shouldn't create duplicates

### 1.2 — Reverse the Sync Direction
Current: Flat files → CortexBrain (every 30 min)
New: CortexBrain → Flat files (export cron, every 30 min)

**Tasks:**
- Create `scripts/cortexhub-export.py` — Pulls all memories from CortexBrain API, writes to `memory/YYYY-MM-DD.md` and `MEMORY.md`
- Modify existing `cortexhub-sync.py` → rename to `cortexhub-export.py` with reversed flow
- Update Albert's cron job to run export instead of sync

### 1.3 — Verify CortexBrain API Completeness
Confirm these endpoints work correctly on the full brain (currently running):
- `POST /api/auth/login` — Auth ✅ (verified)
- `POST /api/memories` — Store episodic memory
- `GET /api/memories/search?q=<query>` — Semantic search
- `GET /api/memories/stats` — Stats
- `GET /api/admin/memories/all` — Full dump
- `POST /api/knowledge` — Store knowledge
- `GET /api/knowledge` — List knowledge
- `GET /api/admin/knowledge/all` — Full knowledge dump
- `DELETE /api/memories/{id}` — Delete specific memory
- `POST /api/auth/refresh` — Token refresh
- `GET /api/auth/me` — Current user info

---

## Phase 2: Recall-First Workflow (Harold Implements)

### 2.1 — Upgrade `ch recall` for Conversational Context
Current: Returns raw text snippets
Needed:
- **Ranked results** with importance scores visible
- **Cross-agent results** — Show if memory is from Albert or Harold
- **Recency weighting** — Recent memories should rank higher for the same relevance
- **Return format** optimized for LLM context injection (clean, concise)

### 2.2 — Create `ch context <topic>` Command
New compound command that:
1. Searches episodic memories for `<topic>`
2. Searches knowledge entries for `<topic>`
3. Combines and deduplicates
4. Returns a ranked, formatted context block ready for injection

### 2.3 — Test Deduplication
CortexBrain currently has duplicate entries from repeated syncs. Harold must:
- Audit current memory store for duplicates
- Run dedup pass (use Sleep Cycle or manual)
- Verify dedup works correctly before we rely on it

---

## Phase 3: Validation & Testing (Harold Implements, Albert Verifies)

### 3.1 — API Endpoint Tests (Harold)
For each endpoint listed in 1.3:
- Test with valid input → expect success
- Test with invalid input → expect proper error
- Test with expired token → expect 401
- Test with wrong user → expect 403
- Measure response time

### 3.2 — Memory Store Tests (Harold)
- Store 10 unique episodic memories with varying importance (0.1 to 1.0)
- Store 5 knowledge entries across different categories
- Verify all stored correctly via `GET /api/admin/memories/all`
- Verify search returns them ranked by relevance
- Verify importance scoring affects ranking
- Delete a memory and verify it's gone

### 3.3 — Recall Accuracy Tests (Albert)
Once Harold reports ready, Albert will:
- Query every existing memory category and verify relevant results return
- Test cross-agent recall (can Albert find Harold's memories?)
- Test temporal queries ("what happened today?", "what did we do last week?")
- Test project-specific queries ("CortexBrain status", "container framework plan")
- Test people queries ("Joshua", "Hamako", "Patrick")
- Test infrastructure queries ("Pink IP", "Harold services", "DHCP setup")
- Conversational test: Ask CortexBrain questions and evaluate quality of context returned

### 3.4 — Export Verification (Harold)
- Run CortexBrain → flat file export
- Compare exported files against current flat files
- Verify no data loss in the round-trip
- Verify format is consistent and readable

---

## Phase 4: Cutover (Albert + Harold)

### 4.1 — Update Albert's Workflow
- AGENTS.md: Change recall instructions to use `ch recall` / `ch context` first
- TOOLS.md: Update CortexHub section with new primary-memory workflow
- Cron: Switch sync cron to export cron
- Stop injecting MEMORY.md into context (save tokens)

### 4.2 — Update Harold's Workflow
- Harold should also use CortexBrain as primary recall
- Harold's sync script should write-through, not batch sync

### 4.3 — Monitoring
- Alert if CortexBrain goes down (critical now — it's the primary memory)
- Health check in self-healing crons
- Backup exports run even if agents are down

---

## Success Criteria
- [ ] All API endpoints tested and passing (100%)
- [ ] Memory store/recall round-trip works for all data types
- [ ] Search returns relevant results for all existing memory categories
- [ ] Cross-agent memory works (Albert ↔ Harold)
- [ ] Deduplication verified — no redundant entries
- [ ] Export to flat files produces identical content
- [ ] Response times < 500ms for recall queries
- [ ] Albert's conversational test passes (subjective quality assessment)

---

## Risk Mitigation
- **CortexBrain goes down:** Flat file exports exist as backup, self-healing cron restarts it
- **Data loss:** SQLite DB backed up in `~/backups/`, export cron writes flat files
- **Quality regression:** If recall quality is poor, we can fall back to flat files instantly

---

## Timeline
- Phase 1-2: Harold executes (target: 2-3 hours)
- Phase 3: Harold tests + Albert verifies (target: 1-2 hours)  
- Phase 4: Cutover after all tests pass

**This plan requires Harold's review before execution.**
