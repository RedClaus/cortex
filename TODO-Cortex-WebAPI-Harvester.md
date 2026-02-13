---
project: Cortex
component: Unknown
phase: Ideation
date_created: 2026-02-04T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:29.698619
---

# TODO: Cortex WebAPI Harvester

**Based on PRD:** `Cortex/PRDs/PRD-Cortex-WebAPI-Harvester.md`  
**Status:** APPROVED P2 — 4 Week Timeline (Deliver after CCA)  
**Priority:** P2 (after Cortex Coder Agent)  
**Created:** 2026-02-04  
**Updated:** 2026-02-04 (Priority changed to P2 per Norman)  

---

## Phase 1: Foundation (Week 1) — IN PROGRESS

### Setup & Scaffolding
- [ ] Create repository: `github.com/RedClaus/cortex-webapi-harvester`
- [ ] Set up Go project structure (cmd/, pkg/, templates/)
- [ ] Configure CI/CD (GitHub Actions)
- [ ] Add Makefile with build/test targets
- [ ] Write initial README and CONTRIBUTING

### Capture Engine
- [ ] Implement Chrome CDP connection (chromedp)
- [ ] Network request interception
- [ ] Filter out static assets (JS, CSS, images)
- [ ] Session management (start, pause, resume, stop)
- [ ] HAR file import support
- [ ] Save capture data to vault

### CLI Framework
- [ ] Set up Cobra CLI framework
- [ ] Implement `harvest start` command
- [ ] Implement `harvest stop` command
- [ ] Implement `harvest import` command
- [ ] Configuration file support (viper)

**Phase 1 Deliverables:**
- [ ] Capture files saved to `~/ServerProjectsMac/Cortex/Harvester/sessions/`
- [ ] Unit tests passing
- [ ] CLI commands functional

---

## Phase 2: Generation (Week 2)

### Analysis Engine
- [ ] Endpoint clustering by (host + path + method)
- [ ] Path parameter detection (`{id}`, `{marketId}`)
- [ ] Query parameter extraction
- [ ] Header parameter detection
- [ ] Authentication pattern detection (Bearer, API key, Cookie)
- [ ] Schema inference from JSON samples
- [ ] Confidence scoring algorithm

### Go Skill Generation
- [ ] Design Go client template
- [ ] Generate struct types from schemas
- [ ] Generate methods per endpoint
- [ ] HTTP client with configurable timeout
- [ ] Error handling with typed errors
- [ ] Auth injection from environment
- [ ] Generate unit tests

### TypeScript Skill Generation
- [ ] Design TypeScript client template
- [ ] Generate interfaces from schemas
- [ ] Generate async functions
- [ ] Fetch-based HTTP client
- [ ] Config via constructor options
- [ ] Generate TypeScript tests

**Phase 2 Deliverables:**
- [ ] `harvest generate --format=go` produces working code
- [ ] `harvest generate --format=typescript` produces working code
- [ ] Generated skills compile and pass tests
- [ ] Sample skills for 2-3 pilot sites

---

## Phase 3: Distribution (Week 3)

### Storage Integration
- [ ] Vault file storage structure
- [ ] CortexBrain knowledge API integration
- [ ] Store skill metadata in CortexBrain
- [ ] Link vault files to knowledge entries

### GitHub Integration
- [ ] Create `github.com/RedClaus/swarm-skills` repository
- [ ] Implement `harvest publish` command
- [ ] Version tagging (semantic versioning)
- [ ] Generate skill index JSON

### A2A Bridge Integration
- [ ] Publish skill_discovered events to bridge
- [ ] Implement `harvest install` command
- [ ] Download skills from GitHub
- [ ] Version resolution and updates

**Phase 3 Deliverables:**
- [ ] Skills published to GitHub
- [ ] A2A distribution working
- [ ] Cross-agent skill sharing functional
- [ ] End-to-end workflow complete

---

## Phase 4: Polish (Week 4)

### Performance & Security
- [ ] Performance benchmarks
- [ ] Optimize capture throughput (1000+ req/min)
- [ ] PII detection and redaction
- [ ] Security audit of generated code
- [ ] Auth token protection review

### Documentation
- [ ] Complete user guide
- [ ] API reference documentation
- [ ] Tutorial: "Your First Skill"
- [ ] Troubleshooting guide
- [ ] Architecture documentation

### Testing & Hardening
- [ ] Integration tests
- [ ] End-to-end CLI tests (Bats)
- [ ] Test with 5+ real websites
- [ ] Error handling edge cases
- [ ] User feedback integration

**Phase 4 Deliverables:**
- [ ] v1.0.0 release tagged
- [ ] Production deployment guide
- [ ] Training materials for swarm agents
- [ ] Demo video or screencast

---

## Pilot Sites (To Be Determined)

Candidates for initial skill generation:
- [ ] **Polymarket** — Prediction markets API
- [ ] **GitHub** — Internal API for advanced features
- [ ] **Weather Service** — Reliable, simple API
- [ ] **News API** — Content aggregation
- [ ] **Financial Data** — Stock/crypto prices

**Selection Criteria:**
- High value (frequent use by agents)
- No official API available
- Reasonable complexity (5-20 endpoints)
- Terms of Service permitting automation

---

## Open Questions / Decisions Needed

- [ ] **Q1:** Which 3-5 sites for initial pilot? (Owner: Norman) — Due: Week 1
- [ ] **Q2:** Go vs TypeScript priority? (Owner: Albert) — Due: Week 1
- [ ] **Q3:** Do we need JavaScript output? (Owner: Albert) — Due: Week 2
- [ ] **Q4:** GitHub repo: new or existing swarm-skills? (Owner: Harold) — Due: Week 1
- [ ] **Q5:** CortexBrain knowledge schema finalization (Owner: Albert) — Due: Week 2

---

## Success Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| API detection accuracy | >90% | Manual verification on 10 sites |
| Generated skill compile rate | >95% | Automated CI checks |
| Speedup vs browser | 50x+ | Benchmark comparison |
| Time to skill generation | <30s | Timer measurement |
| False positive rate | <5% | Manual review |
| Agent adoption | 3+ agents | Active usage tracking |

---

## Dependencies

### External
- Chrome/Chromium browser
- GitHub CLI (optional, for publishing)
- mitmproxy (optional, for proxy capture)

### Internal
- CortexBrain API (Pink:18892)
- A2A Bridge (Harold:18802)
- Vault directory access
- GitHub repository access

---

## Related Work

- **Design Document:** `Cortex/Design/Swarm-API-Harvester.md`
- **PRD:** `Cortex/PRDs/PRD-Cortex-WebAPI-Harvester.md`
- **Unbrowse Evaluation:** `Cortex/Evaluations/Unbrowse-OpenClaw-Evaluation.md`
- **UI-TARS Evaluation:** `Cortex/Evaluations/UI-TARS-desktop-Evaluation.md` (GUI agent patterns)
- **A2A Migration:** `memory/a2a-migration-plan.md` (protocol integration)

---

## Notes

- This replaces proprietary Unbrowse with open-source swarm-native alternative
- No crypto payments, no external services, fully self-hosted
- Aligns with swarm autonomy and privacy-first principles
- Estimated effort: 4 weeks (1 developer)
- High impact: 100x speedup on web automation tasks

---

## Notes

- **Priority changed to P2** per Norman's direction (2026-02-04)
- Cortex Coder Agent is now P1, to be delivered first
- WebAPI Harvester to be developed after CCA completion
- This replaces proprietary Unbrowse with open-source swarm-native alternative
- No crypto payments, no external services, fully self-hosted

---

*Last Updated: 2026-02-04*  
*Status: APPROVED P2 — Deliver after CCA completion*
