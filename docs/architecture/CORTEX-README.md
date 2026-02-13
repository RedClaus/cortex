---
project: Cortex
component: Docs
phase: Ideation
date_created: 2026-02-06T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T11:39:28.863900
---

# Cortex Ecosystem Documentation Index

**Last Updated:** 2026-02-06

---

## 🗂️ Documentation Structure

This folder contains comprehensive organizational documentation for the Cortex ecosystem:

```
ServerProjectsMac/
├── CORTEX-README.md                    ← You are here (start here!)
├── CORTEX-ORGANIZATION-SUMMARY.md      ← Executive summary & immediate actions
├── CORTEX-DEVELOPMENT-PLANS.md         ← Detailed plans for all 11 projects
├── CORTEX-ECOSYSTEM-ROADMAP.md         ← 6-month timeline & milestones
├── CLAUDE.md                           ← Claude Code project guidance
│
├── CortexBrain/                        ← P0: Core AI with 25 cognitive lobes
├── cortex-gateway-test/                ← P0: REST API gateway
├── cortex-coder-agent-test/            ← P1: Code intelligence agent
├── Development/
│   ├── cortex-avatar/                  ← P1: Desktop companion (Wails)
│   ├── Salamander/                     ← P2: YAML-driven TUI framework
│   ├── dnet/                           ← P2: Distributed LLM inference
│   ├── CortexLab/                      ← P1: Component incubator
│   └── CortexIntegrations/             ← P3: External API connectors
├── GoMenu/                             ← P3: macOS menu bar launcher
└── ... (33 archived projects)
```

---

## 📖 How to Use This Documentation

### If You Want to...

#### **Get Started Quickly**
→ Read: [`CORTEX-ORGANIZATION-SUMMARY.md`](./CORTEX-ORGANIZATION-SUMMARY.md)
- Quick overview of all projects
- Immediate action items for this week
- Decision points that need your input
- Quick wins (5-10 hours of high-impact work)

#### **Understand a Specific Project**
→ Read: [`CORTEX-DEVELOPMENT-PLANS.md`](./CORTEX-DEVELOPMENT-PLANS.md)
- Individual plans for all 11 active projects
- Phase-by-phase implementation details
- Success metrics and dependencies
- Estimated effort (weeks/months)

#### **Plan Long-Term Development**
→ Read: [`CORTEX-ECOSYSTEM-ROADMAP.md`](./CORTEX-ECOSYSTEM-ROADMAP.md)
- 6-month timeline (Feb-Jul 2026)
- 3 phases: Foundation, Features, Ecosystem
- Critical path analysis
- Resource allocation strategy
- Risk assessment & mitigation

#### **Configure Claude Code**
→ Read: [`CLAUDE.md`](./CLAUDE.md)
- Project relationships and architecture
- Build/test/run commands for each project
- Data locations and configuration
- Skills system reference

---

## 🎯 Quick Reference: Project Priorities

| Priority | Projects | Status | Action This Week |
|----------|----------|--------|------------------|
| **P0** | CortexBrain, cortex-gateway-test | 95-100% Production | Fix swarm, add tests |
| **P1** | CortexAvatar, CortexLab, cortex-coder-agent | 70-95% Production | Fix TTS bug, extract components |
| **P2** | Salamander, dnet, TermAi-archive | 75-85% Active Dev | Polish, optimize |
| **P3** | GoMenu, CortexIntegrations | 20-90% Low Priority | Defer or maintain |

---

## 🚀 This Week's Action Items

Based on [`CORTEX-ORGANIZATION-SUMMARY.md`](./CORTEX-ORGANIZATION-SUMMARY.md#-quick-start-what-to-do-first):

### Monday (Today)
- [ ] Review all documentation
- [ ] Make decisions on 5 decision points
- [ ] Fix cortex-gateway swarm health checks

### Tuesday-Wednesday
- [ ] Run CortexBrain test suite (establish baseline)
- [ ] Verify Memory API working across all agents
- [ ] Fix CortexAvatar TTS duplication bug

### Thursday-Friday
- [ ] Set up CI/CD for CortexBrain
- [ ] Document current state of all P0 projects
- [ ] Archive old projects (optional)

**Estimated Time:** 10-15 hours this week

---

## 📊 Critical Path (MVP in 10-12 Weeks)

```
Weeks 1-2:  CortexBrain Testing (80%+ coverage)
Weeks 2-3:  cortex-gateway Swarm Fix
Weeks 5-6:  CortexBrain Documentation
Weeks 9-10: VoiceBox Integration (CR-012)
Weeks 10-11: SenseVoice Integration (CR-021)
Weeks 11-12: CortexAvatar TTS Fix + dnet

Total: 10-12 weeks to production-ready MVP
```

---

## 🧭 Navigation Guide

### By Role

**If you're the Architect/Lead:**
1. Start with [`CORTEX-ECOSYSTEM-ROADMAP.md`](./CORTEX-ECOSYSTEM-ROADMAP.md) - Understand full vision
2. Review [`CORTEX-DEVELOPMENT-PLANS.md`](./CORTEX-DEVELOPMENT-PLANS.md) - Deep dive each project
3. Use [`CORTEX-ORGANIZATION-SUMMARY.md`](./CORTEX-ORGANIZATION-SUMMARY.md) - Make decisions

**If you're a Developer:**
1. Start with [`CORTEX-ORGANIZATION-SUMMARY.md`](./CORTEX-ORGANIZATION-SUMMARY.md) - Get context
2. Review your project's section in [`CORTEX-DEVELOPMENT-PLANS.md`](./CORTEX-DEVELOPMENT-PLANS.md)
3. Check [`CLAUDE.md`](./CLAUDE.md) for build commands

**If you're a Contributor:**
1. Read [`CORTEX-ORGANIZATION-SUMMARY.md`](./CORTEX-ORGANIZATION-SUMMARY.md) - Quick overview
2. Find a project that interests you in [`CORTEX-DEVELOPMENT-PLANS.md`](./CORTEX-DEVELOPMENT-PLANS.md)
3. Check project's README for setup instructions

---

## 🔑 Key Concepts

### Core Projects
- **CortexBrain:** Local-first AI with 25 cognitive lobes, emulating human brain processing
- **cortex-gateway:** REST API gateway for unified memory access and swarm coordination
- **CortexAvatar:** Desktop companion with voice, eyes, ears (Wails v2 + Svelte)

### Technologies
- **A2A Protocol:** Agent-to-Agent communication (JSON-RPC 2.0)
- **Neural Bus:** Message-passing system for cognitive lobe coordination
- **AutoLLM:** Intelligent model routing (Fast/Smart lanes)
- **dnet:** Distributed LLM inference for Apple Silicon clusters (MLX backend)

### Architecture
```
┌───────────────┐
│   GoMenu      │ Menu bar launcher
└───────┬───────┘
        │
        v
┌───────────────┐     A2A      ┌───────────────┐     Ollama    ┌───────────────┐
│ CortexAvatar  │◄────────────►│  CortexBrain  │◄─────────────►│     dnet      │
│  (Desktop)    │              │   (AI Core)   │               │  (Cluster)    │
└───────────────┘              └───────┬───────┘               └───────────────┘
                                       │
                                       v
                               ┌───────────────┐
                               │ cortex-gateway│
                               │  (REST API)   │
                               └───────────────┘
```

---

## 📚 Related Documentation

### Project-Specific
- [`CortexBrain/README.md`](./CortexBrain/README.md) - Core AI system
- [`cortex-gateway-test/README.md`](./cortex-gateway-test/README.md) - API gateway
- [`cortex-gateway-test/AGENT-MEMORY-INTEGRATION.md`](./cortex-gateway-test/AGENT-MEMORY-INTEGRATION.md) - Memory API migration guide
- [`Documents/CortexBrain Vault/A2A-Memory-Bridge-Migration.md`](../Documents/CortexBrain\ Vault/A2A-Memory-Bridge-Migration.md) - Complete migration writeup

### Skills Reference
- [`.shared-skills/`](./.shared-skills/) - Specialized knowledge modules
- Skills include: architect, fixit, charm-tui-mastery, a2a-protocol, autollm-routing, and more

---

## 🎓 Getting Started (New to Cortex?)

### 1. Understand the Vision
Read the "Overview" section in [`CLAUDE.md`](./CLAUDE.md) to understand:
- Why Cortex exists (emulate human brain thinking)
- How projects relate to each other
- Core architecture principles

### 2. Set Up Development Environment
```bash
# Clone repository (if not already done)
cd /Users/normanking/ServerProjectsMac

# CortexBrain (Go)
cd CortexBrain
go build -o /tmp/cortex ./cmd/cortex
/tmp/cortex

# cortex-gateway (Go)
cd ../cortex-gateway-test
go build -o cortex-gateway ./cmd/cortex-gateway
./cortex-gateway

# CortexAvatar (Wails)
cd ../Development/cortex-avatar
wails dev

# dnet (Python + MLX)
cd ../dnet
uv sync --extra mac --extra dev
make init
```

### 3. Verify Everything Works
```bash
# Test Memory API
curl http://localhost:8080/api/v1/memories/stats

# Test agents
cd ~/.openclaw/workspace/scripts
./memory-reflect.sh stats
python3 ch stats

# Test A2A bridge
cd ~/.openclaw/workspace
python3 a2a-memory-bridge.py
# In another terminal:
curl http://localhost:18801/.well-known/agent-card.json
```

### 4. Pick Your First Task
See "Quick Wins" in [`CORTEX-ORGANIZATION-SUMMARY.md`](./CORTEX-ORGANIZATION-SUMMARY.md#-quick-wins-do-these-first)

---

## 🤝 Contributing

### Before You Start
1. Read [`CORTEX-ORGANIZATION-SUMMARY.md`](./CORTEX-ORGANIZATION-SUMMARY.md) - Understand current state
2. Review the project's section in [`CORTEX-DEVELOPMENT-PLANS.md`](./CORTEX-DEVELOPMENT-PLANS.md) - Understand roadmap
3. Check [`CLAUDE.md`](./CLAUDE.md) - Follow project patterns

### Development Workflow
1. **Branching:** Create feature branches from `main`
2. **Testing:** Ensure tests pass (`go test ./...`, `pytest`, etc.)
3. **Documentation:** Update README and relevant docs
4. **Code Review:** Follow patterns in existing code
5. **Commit Messages:** Use Conventional Commits (`feat:`, `fix:`, `docs:`)

---

## 📞 Support & Questions

### Documentation Issues
- If documentation is unclear, create an issue
- If you find errors, submit a PR with corrections

### Technical Questions
- Check project-specific README files first
- Review [`CLAUDE.md`](./CLAUDE.md) for architecture guidance
- Search existing issues for similar problems

---

## 📝 Document Changelog

| Date | Version | Changes |
|------|---------|---------|
| 2026-02-06 | 1.0 | Initial documentation system created |

---

## 🔗 Quick Links

| Document | Purpose | Read Time |
|----------|---------|-----------|
| [CORTEX-ORGANIZATION-SUMMARY.md](./CORTEX-ORGANIZATION-SUMMARY.md) | Quick overview & immediate actions | 10 mins |
| [CORTEX-DEVELOPMENT-PLANS.md](./CORTEX-DEVELOPMENT-PLANS.md) | Detailed project plans | 30 mins |
| [CORTEX-ECOSYSTEM-ROADMAP.md](./CORTEX-ECOSYSTEM-ROADMAP.md) | 6-month timeline & strategy | 20 mins |
| [CLAUDE.md](./CLAUDE.md) | Project guidance for Claude Code | 15 mins |

**Total reading time:** ~75 minutes for complete understanding

---

**Start here:** [`CORTEX-ORGANIZATION-SUMMARY.md`](./CORTEX-ORGANIZATION-SUMMARY.md) → **Then:** Make decisions on 5 decision points → **Finally:** Execute this week's action items

*Good luck building the Cortex ecosystem!* 🧠✨
