---
project: Cortex
component: Docs
phase: Ideation
date_created: 2026-02-06T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T11:39:28.875428
---

# Cortex Ecosystem Development Roadmap

**Generated:** 2026-02-06
**Version:** 1.0
**Timeline:** 6 months (Feb 2026 - Jul 2026)

---

## Executive Summary

This roadmap defines the development strategy for the Cortex ecosystem, prioritizing stability, testing, and feature completeness across 11 active projects. The approach focuses on:

1. **Foundation First** (Months 1-2): Stabilize core infrastructure (CortexBrain, cortex-gateway)
2. **Feature Expansion** (Months 3-4): Enhance user-facing capabilities (CortexAvatar, voice features)
3. **Ecosystem Growth** (Months 5-6): Expand supporting infrastructure (dnet, Salamander, integrations)

**Critical Path:** CortexBrain → cortex-gateway → CortexAvatar (10-12 weeks)

---

## Roadmap Overview

```
Month 1-2: Foundation          Month 3-4: Features           Month 5-6: Ecosystem
═══════════════════════        ═══════════════════════       ═══════════════════════
┌─────────────────────┐        ┌─────────────────────┐       ┌─────────────────────┐
│  CortexBrain        │───────>│  Voice Features     │       │  Metacognition      │
│  Testing & Docs     │        │  VoiceBox/SenseVoice│       │  Self-Improvement   │
└─────────────────────┘        └─────────────────────┘       └─────────────────────┘
           │                              │                              │
           v                              v                              v
┌─────────────────────┐        ┌─────────────────────┐       ┌─────────────────────┐
│  cortex-gateway     │───────>│  CortexAvatar       │       │  Advanced Features  │
│  Swarm Fix          │        │  TTS Fix + dnet     │       │  Multi-Modal Input  │
└─────────────────────┘        └─────────────────────┘       └─────────────────────┘
           │                              │                              │
           v                              v                              v
┌─────────────────────┐        ┌─────────────────────┐       ┌─────────────────────┐
│  CortexLab          │───────>│  Component Library  │       │  Production Ready   │
│  Extract Components │        │  5+ Packages        │       │  90%+ Test Coverage │
└─────────────────────┘        └─────────────────────┘       └─────────────────────┘
```

---

## Phase 1: Foundation (Weeks 1-8)

**Focus:** Stability, Testing, Core Infrastructure
**Goal:** Production-grade reliability for P0 projects

### Month 1 (Weeks 1-4): Critical Path Stabilization

#### Week 1-2: CortexBrain Testing Blitz
**Owner:** Primary Developer
**Project:** CortexBrain (P0)

**Deliverables:**
- [ ] 80%+ test coverage for all 25 cognitive lobes
- [ ] Unit tests for AutoLLM routing
- [ ] Integration tests for Neural Bus
- [ ] Performance benchmarks (<500ms cognitive loop)

**Success Criteria:**
- `go test ./...` passes with 0 failures
- Test coverage report shows >80%
- All lobes have documented test cases
- CI/CD pipeline running tests automatically

#### Week 2-3: cortex-gateway Swarm Fix
**Owner:** Infrastructure Developer
**Project:** cortex-gateway-test (P0)

**Deliverables:**
- [ ] Diagnose and fix bridge registration 500 errors
- [ ] Resolve health check failures (harold, pink, red, kentaro)
- [ ] Implement proper retry logic
- [ ] Add Prometheus metrics

**Success Criteria:**
- All bridges show "up" status
- Health checks succeed >99% of the time
- Dashboard shows all nodes healthy
- Zero 500 errors in logs

#### Week 3-4: CortexLab Component Extraction
**Owner:** Architecture Lead
**Project:** CortexLab (P1)

**Deliverables:**
- [ ] Extract Neural Bus to `pkg/neuralbus`
- [ ] Extract AutoLLM router to `pkg/autollm`
- [ ] Extract Memory interfaces to `pkg/memory`
- [ ] Add comprehensive tests for each package

**Success Criteria:**
- 5+ packages in CortexLab/pkg/
- >90% test coverage for each package
- Zero CortexBrain dependencies
- Documentation complete

### Month 2 (Weeks 5-8): Documentation & Polish

#### Week 5-6: CortexBrain Documentation
**Owner:** Documentation Lead
**Project:** CortexBrain (P0)

**Deliverables:**
- [ ] Complete GoDoc for all packages
- [ ] Cognitive lobe architecture guide
- [ ] Deployment guide for production
- [ ] Troubleshooting playbook

**Success Criteria:**
- Every public function has GoDoc
- Architecture diagram complete
- Deployment tested on clean system
- Common issues documented

#### Week 6-7: cortex-gateway Monitoring
**Owner:** Infrastructure Developer
**Project:** cortex-gateway-test (P0)

**Deliverables:**
- [ ] Prometheus metrics endpoints
- [ ] Grafana dashboards
- [ ] Structured logging (JSON)
- [ ] Request tracing (request IDs)

**Success Criteria:**
- Metrics exported and scraped
- 3+ Grafana dashboards
- All errors logged with context
- Request IDs in all logs

#### Week 7-8: cortex-coder-agent Tests
**Owner:** Testing Lead
**Project:** cortex-coder-agent-test (P1)

**Deliverables:**
- [ ] Tests for internal/agent/agent.go
- [ ] Tests for internal/coder/analyzer.go
- [ ] Tests for internal/coder/generator.go
- [ ] 80%+ test coverage

**Success Criteria:**
- All core packages have tests
- Coverage >80%
- CI/CD passing
- Documentation updated

### Phase 1 Milestone: Foundation Complete

**Exit Criteria:**
- ✅ CortexBrain test coverage >80%
- ✅ cortex-gateway swarm healthy
- ✅ CortexLab has 5+ packages
- ✅ Documentation complete for P0 projects
- ✅ All P0/P1 projects have CI/CD

**Review Date:** End of Week 8
**Go/No-Go Decision:** Proceed to Phase 2 if all exit criteria met

---

## Phase 2: Features (Weeks 9-16)

**Focus:** User-Facing Capabilities, Voice Features, Advanced AI
**Goal:** Enhance user experience and expand capabilities

### Month 3 (Weeks 9-12): Voice & Audio

#### Week 9-10: VoiceBox Integration (CR-012)
**Owner:** AI Features Developer
**Project:** CortexBrain (P0)

**Deliverables:**
- [ ] Implement TTS output lobe
- [ ] Add voice response formatting
- [ ] Integrate with CortexAvatar voice pipeline
- [ ] Test voice quality and responsiveness

**Success Criteria:**
- Voice responses working end-to-end
- Latency <2s from request to speech
- Voice quality acceptable
- No audio artifacts

#### Week 10-11: SenseVoice Integration (CR-021)
**Owner:** AI Features Developer
**Project:** CortexBrain (P0)

**Deliverables:**
- [ ] Implement STT input lobe
- [ ] Add voice command parsing
- [ ] Integrate wakeword detection
- [ ] Test recognition accuracy

**Success Criteria:**
- Voice recognition accuracy >95%
- Wakeword detection working
- Commands parsed correctly
- Low false positive rate

#### Week 11-12: CortexAvatar TTS Fix & dnet
**Owner:** Desktop Application Developer
**Project:** CortexAvatar (P1)

**Deliverables:**
- [ ] Fix TTS duplication bug
- [ ] Integrate dnet for local inference
- [ ] Improve A2A error handling
- [ ] Optimize response latency

**Success Criteria:**
- Zero TTS duplication occurrences
- dnet integration working
- Response latency <2s
- Graceful error recovery

### Month 4 (Weeks 13-16): Advanced Capabilities

#### Week 13-14: cortex-coder-agent Advanced Features
**Owner:** Code Intelligence Developer
**Project:** cortex-coder-agent-test (P1)

**Deliverables:**
- [ ] AST-based code analysis
- [ ] Symbol resolution
- [ ] Code quality metrics
- [ ] Multi-language support (Python, TypeScript, Rust)

**Success Criteria:**
- 4+ languages supported
- AST analysis working
- Quality metrics calculated
- Accurate symbol resolution

#### Week 14-15: Salamander Production Polish
**Owner:** TUI Developer
**Project:** Salamander (P2)

**Deliverables:**
- [ ] Complete YAML configuration system
- [ ] Add 9 more themes (target: 20 total)
- [ ] Performance optimization (60fps)
- [ ] 10+ production agent templates

**Success Criteria:**
- 20 themes available
- Smooth 60fps rendering
- 10+ agent templates
- YAML validation working

#### Week 15-16: CortexAvatar Enhanced Avatar
**Owner:** Desktop Application Developer
**Project:** CortexAvatar (P1)

**Deliverables:**
- [ ] Custom avatar animations
- [ ] Thinking/speaking/listening states
- [ ] Personality expressions
- [ ] Mood indicators

**Success Criteria:**
- 4+ avatar states
- Smooth animation transitions
- Personality visible
- 60fps animation

### Phase 2 Milestone: Features Complete

**Exit Criteria:**
- ✅ Voice features working end-to-end
- ✅ CortexAvatar TTS fixed and performant
- ✅ cortex-coder-agent multi-language
- ✅ Salamander production-ready
- ✅ User feedback collected and positive

**Review Date:** End of Week 16
**Go/No-Go Decision:** Proceed to Phase 3 if exit criteria met

---

## Phase 3: Ecosystem (Weeks 17-24)

**Focus:** Infrastructure Expansion, Self-Improvement, Polish
**Goal:** Complete ecosystem with advanced features

### Month 5 (Weeks 17-20): Infrastructure & Intelligence

#### Week 17-18: CortexBrain Metacognition
**Owner:** AI Architecture Lead
**Project:** CortexBrain (P0)

**Deliverables:**
- [ ] Implement sleep cycle self-analysis
- [ ] Performance metrics collection
- [ ] Cognitive pattern analysis
- [ ] Self-optimization engine

**Success Criteria:**
- Nightly self-analysis runs
- Metrics tracked over time
- Optimization suggestions generated
- Measurable improvements

#### Week 18-19: dnet Long Context
**Owner:** ML Infrastructure Developer
**Project:** dnet (P2)

**Deliverables:**
- [ ] RoPE scaling for >128K tokens
- [ ] Context caching implementation
- [ ] Memory optimization
- [ ] Test with 256K tokens

**Success Criteria:**
- 256K+ token contexts working
- Context coherence maintained
- Memory usage acceptable
- Performance benchmarked

#### Week 19-20: CortexAvatar Multi-Modal
**Owner:** Desktop Application Developer
**Project:** CortexAvatar (P1)

**Deliverables:**
- [ ] Screenshot analysis
- [ ] Webcam input
- [ ] Drag-and-drop files
- [ ] Clipboard integration

**Success Criteria:**
- Screenshot analysis working
- Webcam capture functional
- File analysis integrated
- Clipboard context used

### Month 6 (Weeks 21-24): Polish & Expansion

#### Week 21-22: dnet Tensor Parallelism
**Owner:** ML Infrastructure Developer
**Project:** dnet (P2)

**Deliverables:**
- [ ] Model sharding across GPUs
- [ ] Inter-node gRPC communication
- [ ] Load balancing
- [ ] Test with 70B+ models

**Success Criteria:**
- 70B+ models running distributed
- <100ms first token latency
- Automatic failover working
- Performance scalable

#### Week 22-23: CortexIntegrations Framework
**Owner:** Integration Developer
**Project:** CortexIntegrations (P3)

**Deliverables:**
- [ ] Integration framework
- [ ] GitHub integration
- [ ] Slack integration
- [ ] Notion integration

**Success Criteria:**
- 3+ integrations working
- Framework reusable
- Authentication system
- Webhook handling

#### Week 23-24: Final Polish & Launch Prep
**Owner:** All Teams

**Deliverables:**
- [ ] Performance optimization across all projects
- [ ] Security audit
- [ ] Final documentation review
- [ ] User testing feedback incorporated

**Success Criteria:**
- All P0-P1 projects >90% complete
- Security vulnerabilities addressed
- Documentation comprehensive
- User testing positive

### Phase 3 Milestone: Ecosystem Complete

**Exit Criteria:**
- ✅ CortexBrain metacognitive
- ✅ dnet production-ready with long context
- ✅ CortexAvatar multi-modal
- ✅ 3+ external integrations working
- ✅ All documentation complete
- ✅ Security audit passed

**Review Date:** End of Week 24
**Launch Decision:** Ready for production release

---

## Dependencies & Critical Path

### Dependency Graph

```
CortexBrain Testing (Week 1-2)
    │
    ├──> cortex-gateway Fix (Week 2-3)
    │        │
    │        └──> Monitoring (Week 6-7)
    │
    ├──> VoiceBox (Week 9-10)
    │        │
    │        └──> SenseVoice (Week 10-11)
    │                 │
    │                 └──> Metacognition (Week 17-18)
    │
    └──> Documentation (Week 5-6)

CortexLab Extraction (Week 3-4)
    │
    ├──> Component Library (Week 13-14)
    │
    └──> CortexBrain Integration (Week 5-8)

CortexAvatar TTS Fix (Week 11-12)
    │
    ├──> dnet Integration (Week 11-12)
    │        │
    │        └──> Long Context (Week 18-19)
    │                 │
    │                 └──> Tensor Parallelism (Week 21-22)
    │
    └──> Multi-Modal (Week 19-20)

cortex-coder-agent Tests (Week 7-8)
    │
    └──> Advanced Features (Week 13-14)

Salamander Polish (Week 14-15)
CortexIntegrations (Week 22-23)
```

### Critical Path (Longest Dependency Chain)
1. CortexBrain Testing (2 weeks)
2. VoiceBox Integration (2 weeks)
3. SenseVoice Integration (2 weeks)
4. CortexBrain Metacognition (2 weeks)
5. Final Polish (2 weeks)

**Total Critical Path:** 10 weeks (2.5 months)

---

## Resource Allocation

### Team Structure (Recommended)

| Role | Primary Projects | Allocation |
|------|------------------|------------|
| **Architecture Lead** | CortexBrain, CortexLab | 100% |
| **Infrastructure Dev** | cortex-gateway, dnet | 100% |
| **Desktop App Dev** | CortexAvatar | 100% |
| **Code Intelligence Dev** | cortex-coder-agent | 50% |
| **TUI Dev** | Salamander | 50% |
| **Integration Dev** | CortexIntegrations | 50% |
| **Documentation Lead** | All projects | 50% |
| **Testing Lead** | All projects | 50% |

**Total:** 5.5 FTE (Full-Time Equivalent)

### Parallel Development Strategy

**Phase 1 (Weeks 1-8):**
- Team A: CortexBrain + cortex-gateway (3 devs)
- Team B: CortexLab + cortex-coder-agent (2 devs)
- Team C: Documentation + Testing (1 dev)

**Phase 2 (Weeks 9-16):**
- Team A: Voice features (2 devs)
- Team B: CortexAvatar + dnet (2 devs)
- Team C: Salamander + cortex-coder (2 devs)

**Phase 3 (Weeks 17-24):**
- Team A: Metacognition + dnet advanced (2 devs)
- Team B: CortexAvatar multi-modal (2 devs)
- Team C: Integrations + polish (2 devs)

---

## Milestones & Deliverables

### Milestone 1: Foundation Complete (End of Week 8)

**Deliverables:**
- CortexBrain with 80%+ test coverage
- cortex-gateway with healthy swarm
- CortexLab with 5+ packages
- Complete documentation for P0 projects

**Success Metrics:**
- All tests passing
- Zero critical bugs
- Documentation complete
- CI/CD operational

### Milestone 2: Features Complete (End of Week 16)

**Deliverables:**
- Voice features (VoiceBox + SenseVoice)
- CortexAvatar with TTS fix and dnet
- cortex-coder-agent with multi-language
- Salamander production-ready

**Success Metrics:**
- Voice accuracy >95%
- TTS duplication = 0
- 4+ languages supported
- User feedback positive

### Milestone 3: Ecosystem Complete (End of Week 24)

**Deliverables:**
- CortexBrain metacognitive self-improvement
- dnet with long context and tensor parallelism
- CortexAvatar multi-modal input
- 3+ external integrations

**Success Metrics:**
- Self-optimization working
- 256K+ token contexts
- Multi-modal features functional
- Integrations production-ready

---

## Risk Assessment

### High Risk

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| **Voice recognition accuracy <95%** | High | Medium | Early testing, fallback to text, multiple STT providers |
| **dnet long context memory issues** | High | Medium | Incremental testing, memory profiling, fallback to shorter contexts |
| **cortex-gateway swarm instability** | High | Low | Thorough testing, health check redundancy, automatic failover |

### Medium Risk

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| **CortexAvatar TTS duplication recurrence** | Medium | Low | Comprehensive testing, audio pipeline audit, monitoring |
| **Test coverage targets not met** | Medium | Medium | Dedicated testing sprints, automated coverage reports, blockers for low coverage |
| **Integration dependencies delayed** | Medium | Medium | Parallel development, mock integrations, decouple features |

### Low Risk

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| **Salamander theme expansion delayed** | Low | Low | Themes non-critical, can defer, community contributions |
| **GoMenu integration issues** | Low | Low | Optional feature, can deprecate if needed |

---

## Success Criteria

### Phase 1 Success (Weeks 1-8)
- [ ] CortexBrain: 80%+ test coverage, all tests passing
- [ ] cortex-gateway: All nodes healthy, zero 500 errors
- [ ] CortexLab: 5+ packages extracted, >90% coverage each
- [ ] Documentation: Complete for all P0 projects

### Phase 2 Success (Weeks 9-16)
- [ ] Voice features: >95% accuracy, <2s latency
- [ ] CortexAvatar: TTS fixed, dnet integrated, <2s responses
- [ ] cortex-coder-agent: 4+ languages, AST analysis working
- [ ] Salamander: 20 themes, 60fps, 10+ templates

### Phase 3 Success (Weeks 17-24)
- [ ] Metacognition: Self-optimization measurable
- [ ] dnet: 256K+ contexts, 70B+ models distributed
- [ ] CortexAvatar: Multi-modal working
- [ ] Integrations: 3+ production-ready

### Overall Success (End of 6 Months)
- [ ] All P0 projects: 100% production-ready
- [ ] All P1 projects: >90% complete
- [ ] All P2 projects: >75% complete
- [ ] Security: Audit passed, vulnerabilities addressed
- [ ] Documentation: Comprehensive, up-to-date
- [ ] User testing: Positive feedback, <5 critical bugs

---

## Communication & Reporting

### Weekly Status Updates
- **When:** Every Monday
- **Format:** Written status report + 30-min standup
- **Content:**
  - Progress on current week's deliverables
  - Blockers and dependencies
  - Next week's plan
  - Risk updates

### Monthly Reviews
- **When:** Last Friday of each month
- **Format:** 2-hour review meeting
- **Content:**
  - Phase progress vs. plan
  - Milestone completion
  - Risk assessment
  - Budget and resource review
  - Go/No-Go decisions

### Quarterly Planning
- **When:** End of Month 2 and Month 4
- **Format:** Half-day planning session
- **Content:**
  - Retrospective on previous phase
  - Adjust priorities for next phase
  - Resource reallocation if needed
  - Roadmap updates

---

## Metrics & KPIs

### Development Metrics
- **Velocity:** Story points completed per week
- **Test Coverage:** % of code covered by tests (target: >80%)
- **Bug Rate:** Critical bugs per 1000 lines of code (target: <1)
- **Build Success Rate:** % of CI/CD builds passing (target: >95%)

### Product Metrics
- **Response Latency:** Average response time (target: <2s)
- **Error Rate:** % of requests failing (target: <0.1%)
- **Uptime:** % of time services available (target: >99.9%)
- **User Satisfaction:** NPS score from user testing (target: >70)

### Quality Metrics
- **Code Review Coverage:** % of PRs reviewed (target: 100%)
- **Documentation Coverage:** % of public APIs documented (target: 100%)
- **Security Vulnerabilities:** Number of critical/high vulns (target: 0)
- **Technical Debt:** Story points of debt identified (track trend)

---

## Appendix A: Project Priority Matrix

| Project | Priority | Foundation | Features | Ecosystem | Total Weeks |
|---------|----------|------------|----------|-----------|-------------|
| CortexBrain | P0 | 4 weeks | 4 weeks | 2 weeks | 10 weeks |
| cortex-gateway | P0 | 3 weeks | 0 weeks | 0 weeks | 3 weeks |
| CortexLab | P1 | 2 weeks | 2 weeks | 0 weeks | 4 weeks |
| CortexAvatar | P1 | 0 weeks | 4 weeks | 2 weeks | 6 weeks |
| cortex-coder-agent | P1 | 2 weeks | 2 weeks | 0 weeks | 4 weeks |
| Salamander | P2 | 0 weeks | 2 weeks | 0 weeks | 2 weeks |
| dnet | P2 | 0 weeks | 0 weeks | 4 weeks | 4 weeks |
| GoMenu | P3 | 0 weeks | 0 weeks | 1 week | 1 week |
| CortexIntegrations | P3 | 0 weeks | 0 weeks | 2 weeks | 2 weeks |
| TermAi-archive | P2 | 0 weeks | 0 weeks | 3 weeks | 3 weeks |

---

## Appendix B: Technology Stack

### Core Technologies
- **Languages:** Go 1.24+, Python 3.12+, TypeScript 5+
- **Frameworks:** BubbleTea (TUI), Wails v2 (Desktop), Electron (TermAi)
- **ML/AI:** MLX (Apple Silicon), Ollama (local LLM), OpenAI/Anthropic (cloud)
- **Infrastructure:** Docker, Kubernetes, Prometheus, Grafana
- **Protocols:** A2A (JSON-RPC 2.0), REST, gRPC, SSE

### Development Tools
- **Package Managers:** Go modules, uv (Python), pnpm (Node)
- **Testing:** go test, pytest, vitest
- **Linting:** golangci-lint, ruff, eslint
- **CI/CD:** GitHub Actions, Docker Compose
- **Monitoring:** Prometheus, Grafana, structured logging

---

## Appendix C: Glossary

| Term | Definition |
|------|------------|
| **A2A** | Agent-to-Agent protocol (JSON-RPC 2.0) for inter-agent communication |
| **AutoLLM** | Intelligent model routing system (Fast/Smart lanes) |
| **Cognitive Lobe** | Independent processing module in CortexBrain (25 lobes total) |
| **Neural Bus** | Message-passing system for lobe coordination |
| **Swarm** | Distributed cortex-gateway cluster for load balancing |
| **dnet** | Distributed inference system for Apple Silicon clusters |
| **MLX** | Apple's machine learning framework for Apple Silicon |
| **Metacognition** | Self-awareness and self-improvement capabilities |
| **TUI** | Text User Interface (terminal-based UI) |

---

**Document Version:** 1.0
**Last Updated:** 2026-02-06
**Next Review:** 2026-02-13 (Week 2)

*This roadmap is a living document and will be updated as the project progresses.*
