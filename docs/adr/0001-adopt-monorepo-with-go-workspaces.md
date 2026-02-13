---
project: Cortex
component: Unknown
phase: Ideation
date_created: 2026-02-11T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-11T22:46:24.317262
---

# ADR-0001: Adopt Monorepo with Go Workspaces

**Date:** 2026-02-11  
**Status:** ACCEPTED  
**Deciders:** Norman

---

## Context

ServerProjectsMac contains 10+ Go projects that form the Cortex AI ecosystem, all serving the founding principle: *"Emulate the human brain's thinking processes through simulated cognitive modules."*

### Current State Problems

1. **Fragmented Repository Structure**
   - CortexBrain (core engine) lives independently
   - Plugin applications (Pinky, cortex-coder-agent, cortex-gateway, CortexAvatar, Salamander, CortexLab, GoMenu, CortexIntegrations) are scattered repos
   - Projects communicate via A2A protocol (JSON-RPC 2.0) but have no workspace coordination

2. **Dependency Management Issues**
   - Several projects use `replace` directives in `go.mod` to reference CortexLab locally
   - No unified dependency tracking or version management
   - Difficult to validate that shared packages (e.g., CortexLab exports) work across all consumers

3. **Operational Friction**
   - No unified CI/CD pipeline—each project has its own test suite
   - Documentation scattered across root CORTEX-*.md files with no clear hierarchy
   - Building/testing the full ecosystem requires manual orchestration
   - No single source of truth for architectural decisions

4. **Developer Experience**
   - Onboarding requires understanding 10+ separate modules and their relationship graph
   - No enforced conventions for commit messages, naming, or code organization
   - Difficult to refactor shared code (e.g., A2A protocol, routing logic) without breaking consumers

---

## Decision

**Adopt a monorepo structure under `cortex/` using Go 1.24 Workspaces (`go.work`)** with the following layout:

```
cortex/
├── go.work                           # Workspace definition
├── core/                             # CortexBrain and shared libraries
│   ├── cortex-brain/                 # Core engine (20 cognitive lobes)
│   ├── cortex-lab/                   # Component incubator & shared pkgs
│   └── a2a-protocol/                 # JSON-RPC 2.0 agent communication
├── apps/                             # Standalone applications (A2A clients)
│   ├── cortex-avatar/
│   ├── pinky/
│   ├── cortex-coder-agent/
│   ├── cortex-gateway/
│   ├── salamander/
│   ├── go-menu/
│   └── cortex-integrations/
├── docs/                             # Architecture, ADRs, guides
│   ├── adr/                          # Architecture Decision Records
│   ├── architecture.md
│   ├── quickstart.md
│   └── contributing.md
├── research/                         # Research papers, neuroscience notes
├── scripts/                          # Shared build/deploy scripts
├── Makefile                          # Cross-module build orchestration
├── go.sum
└── README.md                         # Ecosystem overview
```

### Key Principles

1. **Single Source of Truth**
   - `go.work` manages all module dependencies
   - Eliminate `replace` directives—use direct module references
   - `go work sync` keeps all modules in sync

2. **Clear Dependency Rules**
   - **Core** modules (cortex-brain, cortex-lab, a2a-protocol) can depend on each other
   - **Apps** (cortex-avatar, pinky, etc.) depend on core modules but NOT on each other
   - Non-Go projects (dnet, CortexIntegrations with Python) stay outside go.work

3. **Unified Conventions**
   - Conventional Commits with scope: `feat(cortex-brain)`, `fix(salamander)`, `docs(adr)`
   - Architecture Decision Records (ADRs) in `docs/adr/` for all significant choices
   - Shared Makefile targets: `make build`, `make test`, `make lint`

4. **Documentation as Code**
   - All architectural decisions recorded in ADRs
   - README in each module describes purpose and usage
   - Root README links to architecture guide and project index

---

## Alternatives Considered

### 1. Keep Separate Repos with Git Submodules
**Rejected** because:
- Submodules add complexity (detached HEAD states, recursive clones)
- Version drift: hard to ensure all consumers use compatible module versions
- No atomic commits across core + apps
- CI/CD overhead to test integration points

### 2. Single Go Module with Internal Packages
**Rejected** because:
- Forces tight coupling: all apps would be internal packages of cortex-brain
- No independent versioning: all modules bumped together
- Violates single-responsibility principle: core engine bundles app logic
- Testing becomes monolithic

### 3. Nx/Bazel-Style Polyglot Monorepo
**Rejected** because:
- Over-engineered for a single developer working in Go
- Go Workspaces provide 90% of the benefit with 10% of the complexity
- Bazel has steep learning curve; Nx targets Node.js ecosystem
- Maintenance overhead not justified

### 4. Keep Monorepo, Use Trunks with Dependency Graph Tools
**Rejected** because:
- Requires external tools (Trunks, Lerna, etc.)
- Go Workspaces are the native, first-class solution

---

## Consequences

### Positive

1. **Atomic Commits**
   - Single commit can update CortexBrain API and all consumer apps in tandem
   - No version mismatch between core and apps at rest

2. **Simplified Dependency Management**
   - `go work sync` replaces manual `replace` directives
   - `go mod tidy` validates the entire graph
   - Clear visibility into what each app imports

3. **Unified CI/CD**
   - Single pipeline builds all modules
   - Tests run in the correct order (core first, then apps)
   - No integration bugs slip through gaps between repos

4. **Faster Refactoring**
   - IDE (GoLand, VS Code + Go extension) understands entire workspace
   - "Find usages" works across modules
   - Rename operations update all consumers atomically

5. **Better Onboarding**
   - New contributor clones one repo, sees full ecosystem
   - Makefile and docs provide clear entry points
   - ADRs explain the "why" behind architectural decisions

### Negative

1. **Initial Migration Effort**
   - Estimated 3-4 weeks to move all 10+ projects into workspace
   - Requires updating import paths and go.mod references
   - Testing needed to ensure zero behavioral changes

2. **Larger Repository**
   - Single .git history includes all projects
   - `git clone` pulls ~500MB (estimate) vs. 50MB per individual repo
   - Risk: if security breach occurs, all projects affected

3. **Enforce Dependency Rules**
   - Manual discipline required: apps cannot import each other
   - Linters (e.g., `go-sumtype`, custom rules) needed to prevent circular deps
   - Code reviews must check import boundaries

4. **Coordination Overhead**
   - Changes to core APIs block app PRs until merged
   - Single main branch means less independence
   - Feature branches must be coordinated across modules

---

## Compatibility with First Principles

### ✅ Single Binary
- **Impact:** None
- Each app builds independently via `go build ./apps/cortex-avatar`
- Cortex-brain remains the primary binary
- Other apps are optional companions

### ✅ Apple Silicon Primary
- **Impact:** None
- Go cross-compilation unchanged
- `make build` handles `GOOS=darwin GOARCH=arm64` uniformly

### ✅ Local-First
- **Impact:** None
- No cloud dependencies introduced
- All modules remain self-contained

### ✅ Go Only (Core)
- **Impact:** Reinforced
- Clear boundary: Go modules in `cortex/`, Python/non-Go in parent `ServerProjectsMac/`
- dnet (Python, MLX backend) stays outside go.work
- CortexIntegrations bridges the gap but doesn't force coupling

### ✅ Memory <500MB
- **Impact:** None
- Runtime memory consumption unchanged
- Build artifacts increase, but final binaries unaffected

---

## Implementation

### Phase 1: Skeleton (Week 1)
- Create `cortex/` directory structure
- Initialize root `go.mod` (module: `cortex.local`) and `go.work`
- Create `docs/adr/` and this ADR
- Add root Makefile with targets: `make build`, `make test`, `make lint`

### Phase 2: Core Migration (Week 2)
- Move CortexBrain to `core/cortex-brain/`
- Move CortexLab to `core/cortex-lab/`
- Extract A2A protocol to `core/a2a-protocol/` (if not already extracted)
- Update imports; verify `go work sync` works

### Phase 3: App Migration (Week 3)
- Move priority apps (CortexAvatar P1, Pinky P1) to `apps/*/`
- Update all A2A client code to import from core
- Run `make test` to verify end-to-end

### Phase 4: Polish & Documentation (Week 4)
- Move remaining apps (Salamander, GoMenu, cortex-gateway, cortex-coder-agent, cortex-integrations)
- Write architecture.md explaining the workspace
- Add contributing.md with branch/commit conventions
- Set up CI/CD (GitHub Actions or similar) for `make test` on all branches

### Timeline
- **Start:** 2026-02-11
- **Estimated Completion:** 2026-03-11

### Modules Affected
- cortex-brain
- cortex-lab
- cortex-avatar
- pinky
- cortex-coder-agent
- cortex-gateway
- salamander
- go-menu
- cortex-integrations
- a2a-protocol (new extraction)

### Non-Affected
- dnet (Python, stays in ServerProjectsMac root)
- ui-ux-pro-max-skill (reference only)
- Scripts/ (kept as-is, will add to shared `scripts/` if relevant)

---

## Rollback Plan

If monorepo structure proves untenable:

1. **Revert to multi-repo:** Split `cortex/` back into independent git repos
2. **Timeline:** Can be done in 1-2 weeks if caught early
3. **Loss:** Only ADRs and unified CI history; individual modules retain full commit logs via git submodules or fresh clones

---

## References

- [Go Workspaces Proposal](https://github.com/golang/go/issues/45713) (Go 1.18+)
- [Go Workspaces Tutorial](https://go.dev/blog/go1.18)
- [Monorepo Conventions](https://monorepo.tools/)

---

## Decision Log

| Date | Event |
|------|-------|
| 2026-02-11 | ADR proposed and accepted |
