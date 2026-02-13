---
project: Cortex
component: Docs
phase: Ideation
date_created: 2026-02-06T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T11:39:28.821704
---

# Cortex Documentation Index

**Documentation Location:** All Cortex documentation is stored in `/Users/normanking/ServerProjectsMac/`

**Last Updated:** 2026-02-06

---

## 📚 Available Documentation

All comprehensive Cortex ecosystem documentation:

### Core Organizational Documents

| Document | Description |
|----------|-------------|
| **CORTEX-README.md** | Master navigation index - Start here! |
| **CORTEX-ORGANIZATION-SUMMARY.md** | Executive summary & immediate actions (this week) |
| **CORTEX-DEVELOPMENT-PLANS.md** | Detailed development plans for all 11 projects |
| **CORTEX-ECOSYSTEM-ROADMAP.md** | 6-month timeline with phases & milestones |
| **OpenClaw-Memory-Integration-Status.md** | OpenClaw agents REST API integration guide |

### Technical Documentation

| Document | Description |
|----------|-------------|
| **CLAUDE.md** | Claude Code project guidance & patterns |
| **AGENT-MEMORY-INTEGRATION.md** | Memory API integration guide (in cortex-gateway-test/) |

---

## 🎯 Quick Access

```bash
# Navigate to documentation
cd /Users/normanking/ServerProjectsMac

# List all CORTEX documentation
ls -la CORTEX-*.md

# View master index
cat CORTEX-README.md

# Open in editor
code /Users/normanking/ServerProjectsMac
```

---

## 📝 Document Creation Policy

**IMPORTANT:** All Cortex documentation must be saved to `/Users/normanking/ServerProjectsMac/`

### For AI Assistants (Claude Code, etc.)

When creating Cortex-related documentation:

1. **Save location:** `/Users/normanking/ServerProjectsMac/`
2. **Naming convention:** `CORTEX-<topic>.md` or descriptive names
3. **Format:** Markdown with clear headings
4. **Metadata:** Include creation date and author in frontmatter

### Example Frontmatter
```markdown
---
project: CortexBrain
component: <Component Name>
phase: <Development Phase>
date_created: 2026-02-06
author: <Author Name>
tags: cortex, <relevant tags>
---
```

---

## 🔗 Related Systems

### CortexBrain Memory API
- **Location:** `~/.cortex/memory/`
- **Purpose:** Daily episodic and knowledge memories (separate from project docs)
- **Format:** Dated markdown files (2026-02-06.md, knowledge.md)
- **Access:** Via REST API at `http://localhost:8080/api/v1/memories`
- **Documentation:** See AGENT-MEMORY-INTEGRATION.md in cortex-gateway-test/

### CortexBrain Vault (Obsidian)
- **Location:** `/Users/normanking/Documents/CortexBrain Vault/`
- **Purpose:** Personal knowledge base, research notes, external documentation
- **Format:** Obsidian-compatible markdown
- **Use:** Reference material, not active project documentation

---

## 📖 Documentation Types by Location

### ServerProjectsMac (Project Documentation)
- ✅ Development roadmaps
- ✅ Architecture decisions
- ✅ Implementation plans
- ✅ Integration guides
- ✅ API documentation
- ✅ Status reports
- ✅ README files
- ✅ Configuration guides

### ~/.cortex/memory/ (Memory API)
- Daily episodic memories
- Knowledge entries
- Agent interactions
- Auto-logged events

### CortexBrain Vault (Personal Notes)
- Research articles
- Learning resources
- External documentation references
- Personal brainstorming

---

## 🔍 Finding Documentation

### List All Cortex Docs
```bash
ls /Users/normanking/ServerProjectsMac/CORTEX-*.md
```

### Search Documentation
```bash
grep -r "keyword" /Users/normanking/ServerProjectsMac/*.md
```

### Open Project Directory
```bash
cd /Users/normanking/ServerProjectsMac
code .  # Open in VS Code
```

---

## ✅ Current Documentation Status

| Document | Size | Last Modified | Status |
|----------|------|---------------|--------|
| CORTEX-README.md | ~8KB | 2026-02-06 | ✅ Complete |
| CORTEX-ORGANIZATION-SUMMARY.md | ~12KB | 2026-02-06 | ✅ Complete |
| CORTEX-DEVELOPMENT-PLANS.md | ~45KB | 2026-02-06 | ✅ Complete |
| CORTEX-ECOSYSTEM-ROADMAP.md | ~38KB | 2026-02-06 | ✅ Complete |
| OpenClaw-Memory-Integration-Status.md | ~18KB | 2026-02-06 | ✅ Complete |

**Total Documentation:** 5 major documents, ~121KB

---

## 🎓 Reading Order

**For Quick Overview:**
1. CORTEX-ORGANIZATION-SUMMARY.md (10 mins)
2. CORTEX-README.md (5 mins)

**For Complete Understanding:**
1. CORTEX-README.md - Navigation & structure
2. CORTEX-ORGANIZATION-SUMMARY.md - Immediate actions & decisions
3. CORTEX-DEVELOPMENT-PLANS.md - Detailed project plans
4. CORTEX-ECOSYSTEM-ROADMAP.md - Long-term strategy
5. OpenClaw-Memory-Integration-Status.md - Technical integration

**Total Time:** ~75 minutes for complete read-through

---

**Remember:** All project documentation lives in ServerProjectsMac. The vault is for personal notes only.

**Last Updated:** 2026-02-06
