---
project: Cortex
component: Docs
phase: Ideation
date_created: 2026-02-06T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T11:39:28.841831
---

# Cortex Ecosystem Development Plans

**Generated:** 2026-02-06
**Purpose:** Individual development plans for all 11 active Cortex projects

---

## Project 1: CortexBrain (P0 - Critical)

**Current Status:** 95% Production (v2.3.0)
**Location:** `/Users/normanking/ServerProjectsMac/CortexBrain/`
**Priority:** P0 - Critical Path

### Objectives
- Maintain stability as core ecosystem hub
- Expand cognitive capabilities with voice features
- Improve test coverage from 15% to 80%+
- Enhance metacognitive self-awareness

### Development Phases

#### Phase 1: Stability & Testing (2-3 weeks)
**Goal:** Achieve production-grade reliability

- [ ] **Unit Tests** - Target 80%+ coverage
  - Write tests for all 25 cognitive lobes
  - Test AutoLLM routing logic
  - Test A2A protocol handlers
  - Test Neural Bus message passing
- [ ] **Integration Tests**
  - End-to-end lobe coordination tests
  - Memory system integration tests
  - External LLM provider failover tests
- [ ] **Performance Benchmarks**
  - Profile cognitive loop latency (<500ms target)
  - Memory query performance (<50ms target)
  - Concurrent task handling (10+ tasks)

#### Phase 2: Voice Features (3-4 weeks)
**Goal:** Complete VoiceBox and SenseVoice integration

- [ ] **VoiceBox Integration (CR-012)**
  - Implement TTS output lobe
  - Add voice response formatting
  - Integrate with CortexAvatar voice pipeline
  - Test voice quality and responsiveness
- [ ] **SenseVoice Integration (CR-021)**
  - Implement STT input lobe
  - Add voice command parsing
  - Integrate wakeword detection
  - Test recognition accuracy
- [ ] **Voice Pipeline**
  - Full voice conversation loop
  - Voice interruption handling
  - Background listening mode

#### Phase 3: Metacognitive Self-Awareness (4-6 weeks)
**Goal:** Enable self-improvement through introspection

- [ ] **Sleep Cycle Implementation**
  - Nightly self-analysis routine
  - Performance metrics collection
  - Cognitive pattern analysis
  - Self-optimization suggestions
- [ ] **Meta-Learning**
  - Track successful vs failed cognitive paths
  - Identify underutilized lobes
  - Optimize routing heuristics
  - Generate self-improvement plans

#### Phase 4: Documentation & Polish (1-2 weeks)
- [ ] Complete API documentation (GoDoc)
- [ ] Create cognitive lobe architecture guide
- [ ] Write deployment guide for production
- [ ] Create troubleshooting playbook

### Key Metrics
- Test coverage: 15% → 80%+
- Cognitive loop latency: <500ms
- Memory query speed: <50ms
- Concurrent tasks: 10+
- Voice recognition accuracy: >95%

### Dependencies
- cortex-gateway (memory API)
- CortexAvatar (voice I/O)
- dnet or Ollama (local LLM)

---

## Project 2: cortex-gateway-test (P0 - Critical)

**Current Status:** 100% Production
**Location:** `/Users/normanking/ServerProjectsMac/cortex-gateway-test/`
**Priority:** P0 - Critical Infrastructure

### Objectives
- Fix swarm health check issues
- Improve bridge registration reliability
- Add comprehensive monitoring
- Document swarm configuration

### Development Phases

#### Phase 1: Swarm Connectivity Fix (1 week)
**Goal:** Resolve "down" status for harold, pink, red, kentaro

- [ ] **Diagnose Bridge Registration 500 Errors**
  - Review bridge registration logs
  - Check DNS discovery implementation
  - Verify health check endpoints on all bridges
  - Test network connectivity between nodes
- [ ] **Fix Health Ring Implementation**
  - Correct health check intervals
  - Implement proper timeout handling
  - Add retry logic for failed checks
  - Fix status reporting to dashboard
- [ ] **Test Swarm Coordination**
  - Verify all bridges register successfully
  - Confirm health checks return "up"
  - Test task distribution across swarm
  - Validate failover behavior

#### Phase 2: Monitoring & Observability (1-2 weeks)
- [ ] **Metrics Collection**
  - Add Prometheus metrics endpoints
  - Track request latency, error rates, throughput
  - Monitor memory API performance
  - Track bridge health status
- [ ] **Logging Improvements**
  - Structured JSON logging
  - Request ID tracing
  - Error stack traces
  - Performance logging
- [ ] **Alerting**
  - Bridge down alerts
  - High error rate alerts
  - Performance degradation alerts

#### Phase 3: Documentation (1 week)
- [ ] Swarm configuration guide
- [ ] Bridge registration troubleshooting
- [ ] API documentation with OpenAPI spec
- [ ] Deployment guide (Docker + Kubernetes)

### Key Metrics
- Bridge registration success rate: 100%
- Health check success rate: >99%
- API latency p95: <100ms
- Error rate: <0.1%

### Dependencies
- Harold Bridge (A2A router)
- DNS discovery service
- Health ring implementation

---

## Project 3: cortex-coder-agent-test (P1 - High)

**Current Status:** 95% Production-Ready
**Location:** `/Users/normanking/ServerProjectsMac/cortex-coder-agent-test/`
**Priority:** P1 - High Value

### Objectives
- Complete missing test files
- Implement advanced code analysis features
- Integrate with CortexBrain cognitive architecture
- Add multi-language support

### Development Phases

#### Phase 1: Test Completion (1 week)
- [ ] Write tests for `internal/agent/agent.go` (currently 0 tests)
- [ ] Write tests for `internal/coder/analyzer.go` (currently 0 tests)
- [ ] Write tests for `internal/coder/generator.go` (currently 0 tests)
- [ ] Achieve 80%+ test coverage

#### Phase 2: Advanced Features (2-3 weeks)
- [ ] **Code Intelligence**
  - AST-based analysis for Go, Python, JavaScript
  - Symbol resolution and cross-references
  - Code quality metrics (complexity, maintainability)
  - Security vulnerability scanning
- [ ] **Multi-Language Support**
  - Expand beyond Go (Python, TypeScript, Rust)
  - Language-specific best practices
  - Cross-language refactoring patterns
- [ ] **Code Generation**
  - Template-based code generation
  - Test generation from interfaces
  - Documentation generation

#### Phase 3: CortexBrain Integration (1-2 weeks)
- [ ] Implement A2A protocol for CortexBrain communication
- [ ] Add coder-specific cognitive lobe in CortexBrain
- [ ] Enable code analysis via Neural Bus
- [ ] Test end-to-end code assistance workflow

### Key Metrics
- Test coverage: 60% → 80%+
- Code analysis latency: <2s
- Multi-language support: 4+ languages
- CortexBrain integration: Full A2A

---

## Project 4: CortexAvatar (P1 - High)

**Current Status:** 80% Production (Phase 6 Complete)
**Location:** `/Users/normanking/ServerProjectsMac/Development/cortex-avatar/`
**Priority:** P1 - High (User-Facing)

### Objectives
- Fix TTS duplication bug
- Improve A2A error handling
- Integrate dnet for faster responses
- Add custom avatar animations

### Development Phases

#### Phase 1: Bug Fixes (1 week)
- [ ] **Fix TTS Duplication**
  - Identify root cause (double audio playback)
  - Implement proper audio queue management
  - Test voice output pipeline
  - Verify no duplicate playback
- [ ] **A2A Error Handling**
  - Add proper error recovery
  - Implement connection retry logic
  - Display user-friendly error messages
  - Test disconnection scenarios

#### Phase 2: Performance (1-2 weeks)
- [ ] **dnet Integration**
  - Switch from Ollama to dnet for local inference
  - Test latency improvements
  - Implement streaming response handling
  - Benchmark response times
- [ ] **Response Optimization**
  - Reduce time-to-first-token
  - Optimize avatar animation triggers
  - Cache common responses

#### Phase 3: Enhanced Avatar (2-3 weeks)
- [ ] **Custom Animations**
  - Thinking animation
  - Speaking animation with lip-sync
  - Listening animation
  - Error/confused animation
- [ ] **Personality Expressions**
  - Mood indicators
  - Confidence levels
  - Processing complexity visualization

#### Phase 4: Advanced Features (2-3 weeks)
- [ ] **Screen & Camera Capture**
  - Implement screenshot analysis
  - Add webcam input for visual tasks
  - Context-aware visual assistance
- [ ] **Multi-Modal Input**
  - Voice + screen + camera
  - Drag-and-drop file analysis
  - Clipboard integration

### Key Metrics
- TTS duplication: 0 occurrences
- Response latency: <2s (dnet)
- A2A error recovery: 100%
- Avatar animation smoothness: 60fps

### Dependencies
- CortexBrain (A2A server)
- dnet (local LLM inference)
- Wails v2 framework

---

## Project 5: Salamander (P2 - Medium)

**Current Status:** 75% Active Development
**Location:** `/Users/normanking/ServerProjectsMac/Development/Salamander/`
**Priority:** P2 - Medium

### Objectives
- Complete YAML agent configuration system
- Add more theme options (target: 20 themes)
- Implement advanced TUI features
- Create production-ready agent templates

### Development Phases

#### Phase 1: YAML System Completion (2-3 weeks)
- [ ] **Agent Configuration**
  - Complete YAML schema documentation
  - Add validation for agent configs
  - Implement hot-reload for config changes
  - Create agent template library
- [ ] **Personality System**
  - Define personality trait schema
  - Implement trait-based response modification
  - Add personality presets (professional, casual, technical)
  - Test personality consistency

#### Phase 2: UI/UX Polish (2 weeks)
- [ ] **Theme Expansion**
  - Add 9 more themes (target: 20 total)
  - Create theme preview gallery
  - Implement theme hot-switching
  - Test themes across terminals
- [ ] **Advanced TUI Features**
  - Split-pane layouts
  - Tab navigation
  - Markdown rendering improvements
  - Syntax highlighting for code blocks

#### Phase 3: Production Readiness (1-2 weeks)
- [ ] **Performance Optimization**
  - Profile TUI rendering performance
  - Optimize viewport scrolling
  - Reduce memory footprint
  - Test with large conversation histories
- [ ] **Testing**
  - Unit tests for core components
  - Integration tests for A2A protocol
  - UI snapshot tests
  - Performance benchmarks

#### Phase 4: Agent Templates (1 week)
- [ ] Create 10+ production-ready agent templates
- [ ] Document agent creation workflow
- [ ] Add example agents for common tasks
- [ ] Create agent marketplace concept

### Key Metrics
- Total themes: 11 → 20
- Agent templates: 10+
- TUI rendering: 60fps
- A2A protocol compliance: 100%

---

## Project 6: dnet (P2 - Medium)

**Current Status:** 85% Active Development
**Location:** `/Users/normanking/ServerProjectsMac/Development/dnet/`
**Priority:** P2 - Medium (Infrastructure)

### Objectives
- Support long context (>128K tokens)
- Implement tensor parallelism
- Expand to non-Apple platforms
- Improve cluster management

### Development Phases

#### Phase 1: Long Context Support (3-4 weeks)
- [ ] **Context Window Expansion**
  - Implement RoPE scaling for >128K tokens
  - Add sparse attention mechanisms
  - Optimize memory usage for long contexts
  - Test with 256K+ token contexts
- [ ] **Context Management**
  - Implement context caching
  - Add sliding window support
  - Optimize KV cache memory
  - Test context coherence

#### Phase 2: Tensor Parallelism (4-6 weeks)
- [ ] **Distributed Inference**
  - Implement model sharding across GPUs
  - Add inter-node communication (gRPC)
  - Optimize data transfer overhead
  - Test with large models (70B+ parameters)
- [ ] **Load Balancing**
  - Dynamic shard assignment
  - Automatic failover
  - Performance monitoring per shard
  - Optimize shard placement

#### Phase 3: Platform Expansion (3-4 weeks)
- [ ] **Non-Apple Silicon Support**
  - Add CUDA backend for NVIDIA GPUs
  - Add ROCm backend for AMD GPUs
  - Abstract hardware-specific code
  - Test on different platforms
- [ ] **Containerization**
  - Create optimized Docker images
  - Add Kubernetes manifests
  - Implement auto-scaling
  - Test deployment scenarios

#### Phase 4: Cluster Management (2-3 weeks)
- [ ] **Management Dashboard**
  - Web UI for cluster status
  - Real-time metrics visualization
  - Shard management interface
  - Log aggregation
- [ ] **Monitoring**
  - Prometheus metrics export
  - Grafana dashboards
  - Alerting for failures
  - Performance profiling

### Key Metrics
- Context window: 8K → 256K tokens
- Tensor parallelism: Support 70B+ models
- Platform support: 3+ platforms
- Inference latency: <100ms (first token)

---

## Project 7: GoMenu (P3 - Low)

**Current Status:** 90% Functional
**Location:** `/Users/normanking/ServerProjectsMac/GoMenu/`
**Priority:** P3 - Low (Utility)

### Objectives
- Fix/remove broken CortexBrain JSON-RPC integration
- Add keyboard shortcuts
- Improve menu organization
- Add menu item categories

### Development Phases

#### Phase 1: Bug Fixes (1 week)
- [ ] **CortexBrain Integration**
  - Test current JSON-RPC integration
  - Fix connection issues or remove if obsolete
  - Document integration status
  - Decide: keep or remove integration
- [ ] **Stability**
  - Fix menu crashes if any
  - Improve error handling
  - Add proper logging

#### Phase 2: Feature Enhancements (1-2 weeks)
- [ ] **Keyboard Shortcuts**
  - Global hotkey for menu activation
  - Quick command shortcuts
  - Configurable keybindings
  - Display shortcuts in menu
- [ ] **Menu Organization**
  - Add categories/sections
  - Implement favorites
  - Add search functionality
  - Recent commands history
- [ ] **Configuration**
  - YAML-based menu configuration
  - Hot-reload menu items
  - Custom icons
  - Dynamic menu items

### Key Metrics
- Launch latency: <100ms
- Menu items: Organized in categories
- Keyboard shortcuts: 5+ global shortcuts

---

## Project 8: CortexLab (P1 - High)

**Current Status:** 70% Stub (Needs Expansion)
**Location:** `/Users/normanking/ServerProjectsMac/Development/CortexLab/`
**Priority:** P1 - High (R&D Infrastructure)

### Objectives
- Expand pkg/ exports for reusable components
- Extract components from CortexBrain for testing
- Create component incubation workflow
- Build component documentation

### Development Phases

#### Phase 1: Component Extraction (2-3 weeks)
- [ ] **Identify Reusable Components in CortexBrain**
  - Neural Bus messaging
  - AutoLLM routing
  - Memory system interfaces
  - TUI components
  - A2A protocol handlers
- [ ] **Extract to CortexLab**
  - Move components to CortexLab/pkg/
  - Add comprehensive tests
  - Document APIs
  - Ensure zero CortexBrain dependencies

#### Phase 2: Component Library (2-3 weeks)
- [ ] **Core Packages**
  - `pkg/neuralbus` - Message passing
  - `pkg/autollm` - Model routing
  - `pkg/memory` - Memory interfaces
  - `pkg/a2a` - A2A protocol
  - `pkg/tui` - BubbleTea components
- [ ] **Documentation**
  - GoDoc for all packages
  - Usage examples
  - Integration guides
  - Testing guides

#### Phase 3: Integration Workflow (1 week)
- [ ] **Local Replace Pattern**
  - Document go.mod replace directive usage
  - Create testing workflow
  - Add CI/CD for CortexLab
  - Test components in isolation
- [ ] **Graduation Process**
  - Define criteria for CortexBrain integration
  - Create PR template
  - Add versioning strategy

### Key Metrics
- Exported packages: 5+ in pkg/
- Test coverage: >90% for all packages
- Zero external dependencies for core packages

---

## Project 9: CortexIntegrations (P3 - Low)

**Current Status:** 20% Early Stage
**Location:** `/Users/normanking/ServerProjectsMac/Development/CortexIntegrations/`
**Priority:** P3 - Low (Future Expansion)

### Objectives
- Define integration patterns
- Add more connectors (GitHub, Slack, Notion)
- Create integration framework
- Build integration marketplace

### Development Phases

#### Phase 1: Integration Framework (2-3 weeks)
- [ ] **Core Framework**
  - Define integration interface
  - Create authentication system (OAuth, API keys)
  - Implement webhook handling
  - Add rate limiting
  - Create integration registry
- [ ] **Testing Framework**
  - Mock integration responses
  - Integration test harness
  - End-to-end testing

#### Phase 2: Core Integrations (3-4 weeks)
- [ ] **GitHub Integration**
  - Repository operations
  - Issue/PR management
  - Code review automation
  - Workflow triggers
- [ ] **Slack Integration**
  - Message sending
  - Channel management
  - Bot commands
  - Event handling
- [ ] **Notion Integration**
  - Page creation/updates
  - Database queries
  - Content sync
  - Template management

#### Phase 3: Integration Marketplace (2-3 weeks)
- [ ] **Marketplace Concept**
  - Web UI for browsing integrations
  - One-click installation
  - Configuration management
  - Usage analytics
- [ ] **Documentation**
  - Integration development guide
  - API reference
  - Example integrations

### Key Metrics
- Total integrations: 1 → 5+
- Integration framework: Complete
- Marketplace: Beta launch

---

## Project 10: Cortex-v1 (Deprecated)

**Current Status:** 90% Stable (Archived)
**Location:** `/Users/normanking/ServerProjectsMac/Cortex-01/`
**Priority:** Deprecated

### Objectives
- **No active development**
- Archive properly with documentation
- Extract any useful patterns for CortexBrain
- Maintain for reference only

### Preservation Tasks
- [ ] Document what was learned from v1
- [ ] Extract unique patterns not in v2
- [ ] Add README with deprecation notice
- [ ] Point users to CortexBrain

---

## Project 11: TermAi-archive (P2 - Medium)

**Current Status:** 85% Major Rewrite in Progress
**Location:** `/Users/normanking/ServerProjectsMac/Development/TermAi-archive/`
**Priority:** P2 - Medium (Parallel Development)

### Objectives
- Complete Electron migration
- Implement monorepo pnpm workspaces
- Apply Apple Design System
- Share CortexLab components

### Development Phases

#### Phase 1: Electron Migration (3-4 weeks)
- [ ] **Architecture Migration**
  - Convert from terminal to Electron app
  - Implement IPC communication
  - Add native menus and dialogs
  - Test cross-platform (macOS, Windows, Linux)
- [ ] **UI Rebuild**
  - Apply Apple Design System
  - Implement main window layout
  - Add preferences window
  - Create about/help screens

#### Phase 2: Monorepo Setup (1-2 weeks)
- [ ] **Workspace Configuration**
  - Set up pnpm workspaces
  - Define packages structure
  - Configure shared dependencies
  - Set up build orchestration
- [ ] **Package Structure**
  - `packages/main` - Electron main process
  - `packages/renderer` - React UI
  - `packages/shared` - Shared utilities
  - `packages/types` - TypeScript types

#### Phase 3: CortexLab Integration (1-2 weeks)
- [ ] Identify shareable components
- [ ] Add Go backend using CortexLab packages
- [ ] Implement A2A protocol support
- [ ] Test integration with CortexBrain

### Key Metrics
- Electron migration: 100% complete
- Monorepo setup: pnpm workspaces
- CortexLab components: 3+ shared
- Apple Design System: Full compliance

---

## Summary: Active Development Priorities

| Priority | Projects | Total Effort | Focus |
|----------|----------|--------------|-------|
| **P0** | CortexBrain, cortex-gateway-test | 6-8 weeks | Stability, testing, swarm fixes |
| **P1** | CortexAvatar, cortex-coder-agent-test, CortexLab | 8-10 weeks | Features, components, integration |
| **P2** | Salamander, dnet, TermAi-archive | 12-16 weeks | Polish, performance, expansion |
| **P3** | GoMenu, CortexIntegrations | 4-6 weeks | Utilities, future features |

**Total estimated effort across all projects:** 30-40 weeks (7-10 months)

With parallel development across multiple projects, the critical path can be reduced to **3-4 months** for P0-P1 priorities.
