---
project: Cortex
component: Docs
phase: Ideation
date_created: 2026-02-06T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T11:39:28.831506
---

# Librarian Agent Documentation Policy

**Date:** 2026-02-06
**Status:** ✅ Configured
**Agent:** librarian-agent.py v1.0.0

---

## Summary

The librarian agent has been configured to save all Cortex project documentation to `/Users/normanking/ServerProjectsMac/`. This policy is now enforced in code, stored in memory, and documented.

---

## Configuration Changes

### 1. Updated librarian-agent.py

**File:** `~/.openclaw/workspace/agents/librarian-agent.py`

**Changes:**
```python
# DOCUMENTATION POLICY: All Cortex project documentation must be saved to ServerProjectsMac
# The vault at ~/Documents/CortexBrain Vault is for personal notes only
# Memory API logs stay in ~/.cortex/memory/ (separate system)
VAULT_ROOT = Path.home() / "ServerProjectsMac"
```

**Line:** 27-31

**What this means:**
- Librarian scans `~/ServerProjectsMac/` for documentation
- Creates indexes in `~/ServerProjectsMac/`
- Organizes files within `~/ServerProjectsMac/`
- Never touches `~/Documents/CortexBrain Vault/`

---

### 2. Updated README-LIBRARIAN.md

**File:** `~/.openclaw/workspace/agents/README-LIBRARIAN.md`

**Added:**
```markdown
**DOCUMENTATION POLICY:** All Cortex project documentation must be saved to
`/Users/normanking/ServerProjectsMac/`. The vault at `~/Documents/CortexBrain Vault`
is for personal notes only.
```

---

### 3. Created LIBRARIAN-CONFIG.md

**File:** `~/.openclaw/workspace/agents/LIBRARIAN-CONFIG.md`

**Contents:**
- Complete configuration reference
- Documentation policy details
- Directory structure
- Frontmatter schema
- Operations guide
- Deployment instructions
- Troubleshooting
- Policy enforcement examples

---

### 4. Stored in CortexBrain Memory

**Memory Entry:**
```
LIBRARIAN AGENT CONFIGURATION: The librarian-agent.py VAULT_ROOT is set to
~/ServerProjectsMac/. All documentation created, moved, or organized by the
librarian agent must be saved to /Users/normanking/ServerProjectsMac/. This is
the official Cortex project documentation location.
```

**Type:** Knowledge
**Importance:** 0.95 (very high)

**Retrieval:**
```bash
curl "http://localhost:8080/api/v1/memories/search?q=LIBRARIAN+AGENT"
```

---

## Policy Overview

### ✅ Correct Documentation Locations

| Content Type | Location | Purpose |
|--------------|----------|---------|
| **Project Documentation** | `/Users/normanking/ServerProjectsMac/` | All Cortex project docs |
| **Personal Notes** | `~/Documents/CortexBrain Vault/` | Research, learning, brainstorming |
| **Memory API Logs** | `~/.cortex/memory/` | Agent interactions, daily logs |

### 📁 ServerProjectsMac Contents

```
ServerProjectsMac/
├── CORTEX-README.md                          # Master index
├── CORTEX-ORGANIZATION-SUMMARY.md            # Executive summary
├── CORTEX-DEVELOPMENT-PLANS.md               # Project plans
├── CORTEX-ECOSYSTEM-ROADMAP.md               # 6-month roadmap
├── CORTEX-DOCS.md                            # Documentation index
├── OpenClaw-Memory-Integration-Status.md     # Integration guide
├── LIBRARIAN-DOCUMENTATION-POLICY.md         # This file
├── CortexBrain/                              # Core project
├── cortex-gateway-test/                      # API gateway
├── Development/                              # Active projects
└── ... (all Cortex projects)
```

---

## How It Works

### When Librarian Agent Starts

1. **Reads VAULT_ROOT:**
   ```python
   VAULT_ROOT = Path.home() / "ServerProjectsMac"
   self.vault_root = VAULT_ROOT
   ```

2. **Creates vault if needed:**
   ```python
   self.vault_root.mkdir(exist_ok=True)
   ```

3. **Scans for documents:**
   ```python
   for md_file in self.vault_root.rglob("*.md"):
       # Index this file
   ```

4. **Stores activity in memory:**
   ```python
   self.store_memory(
       "Librarian agent initialized, scanning vault at ~/ServerProjectsMac",
       "episodic",
       0.7
   )
   ```

### When Creating/Organizing Documents

**Example operation:**
```python
# Organize a new document
dest_folder = self.vault_root / "CortexBrain" / "Planning" / "Phase1"
dest_folder.mkdir(parents=True, exist_ok=True)
dest_file = dest_folder / "feature-plan.md"

# Move/create file
shutil.move(source_file, dest_file)

# Store memory
self.store_memory(
    f"Organized feature-plan.md → CortexBrain/Planning/Phase1/",
    "episodic",
    0.6
)
```

**Result:** File saved to `/Users/normanking/ServerProjectsMac/CortexBrain/Planning/Phase1/feature-plan.md`

---

## Verification

### Test 1: Check Configuration
```bash
grep "^VAULT_ROOT" ~/.openclaw/workspace/agents/librarian-agent.py
# Expected: VAULT_ROOT = Path.home() / "ServerProjectsMac"
```

### Test 2: Check Memory
```bash
curl -s "http://localhost:8080/api/v1/memories/search?q=LIBRARIAN+VAULT_ROOT" | jq .
# Expected: Returns policy with importance 0.95
```

### Test 3: Check README
```bash
grep -i "documentation policy" ~/.openclaw/workspace/agents/README-LIBRARIAN.md
# Expected: Shows policy about ServerProjectsMac
```

### Test 4: Run Librarian (Dry Run)
```bash
cd ~/.openclaw/workspace/agents
python3 -c "
from librarian_agent import LibrarianAgent
agent = LibrarianAgent()
print(f'VAULT_ROOT: {agent.vault_root}')
"
# Expected: VAULT_ROOT: /Users/normanking/ServerProjectsMac
```

---

## Impact on Operations

### A2A Skills

All A2A skills now operate on ServerProjectsMac:

**1. skills/vault/index**
- Creates `index.md` files in ServerProjectsMac subdirectories
- Never touches CortexBrain Vault

**2. skills/vault/search**
- Searches only ServerProjectsMac
- Ignores CortexBrain Vault

**3. skills/vault/organize**
- Moves files within ServerProjectsMac
- Creates project/component/phase structure in ServerProjectsMac

### Memory Storage

All librarian operations are logged to Memory API:

```bash
# View librarian activity
curl "http://localhost:8080/api/v1/memories/search?q=librarian&limit=20" | jq .

# Recent operations
curl "http://localhost:8080/api/v1/memories/search?q=indexed" | jq .
```

---

## Related Documentation

| Document | Location | Purpose |
|----------|----------|---------|
| **LIBRARIAN-CONFIG.md** | `~/.openclaw/workspace/agents/` | Complete config reference |
| **README-LIBRARIAN.md** | `~/.openclaw/workspace/agents/` | Agent overview & deployment |
| **CORTEX-DOCS.md** | `~/ServerProjectsMac/` | Documentation index |
| **OpenClaw-Memory-Integration-Status.md** | `~/ServerProjectsMac/` | Memory integration guide |

---

## Policy Enforcement

### Automated Checks

**In librarian-agent.py:**
- VAULT_ROOT hardcoded to ServerProjectsMac (line 27)
- All file operations use `self.vault_root`
- No code references to CortexBrain Vault

**In Memory:**
- Policy stored with importance 0.95
- Searchable by all agents
- Referenced in Claude Code auto-sync

**In Documentation:**
- README updated with policy
- LIBRARIAN-CONFIG.md created
- This policy document created

---

## Future Considerations

### If Policy Needs to Change

1. Update `VAULT_ROOT` in librarian-agent.py
2. Update memory entry (new knowledge item)
3. Update all documentation (README, config files)
4. Notify all agents via Memory API
5. Test librarian operations

### Adding New Documentation Locations

If you need to add a secondary documentation location:

1. Add new path constant in librarian-agent.py
2. Update `scan_vault()` to include new path
3. Update policy documentation
4. Store new policy in memory

---

## Troubleshooting

### Problem: Librarian saving to wrong location

**Check:**
```bash
grep "VAULT_ROOT" ~/.openclaw/workspace/agents/librarian-agent.py
```

**Expected:**
```python
VAULT_ROOT = Path.home() / "ServerProjectsMac"
```

**If wrong, fix and restart librarian.**

### Problem: Can't find policy in memory

**Retrieve:**
```bash
curl -s "http://localhost:8080/api/v1/memories/search?q=LIBRARIAN&limit=5" | jq -r '.results[] | select(.importance > 0.8) | .content'
```

**If missing, re-run:**
```bash
curl -X POST http://localhost:8080/api/v1/memories/store \
  -H "Content-Type: application/json" \
  -d '{"content":"LIBRARIAN AGENT CONFIGURATION: VAULT_ROOT set to ~/ServerProjectsMac/","type":"knowledge","importance":0.95}'
```

---

## Summary

✅ **Librarian agent configured:** VAULT_ROOT = ServerProjectsMac
✅ **Policy documented:** 3 files updated/created
✅ **Memory stored:** Importance 0.95 (knowledge)
✅ **Verification passed:** All checks green

**Status:** The librarian agent will now save all documentation to ServerProjectsMac automatically.

**Last Updated:** 2026-02-06
**Configured By:** Claude Code
**Verified By:** Configuration checks, memory verification, README review
