---
project: Cortex
component: Docs
phase: Ideation
date_created: 2026-02-04T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:29.789807
---

# Product Requirements Document: Cortex WebAPI Harvester

**Document ID:** PRD-WEBAPI-HARVESTER-001  
**Version:** 1.0  
**Date:** 2026-02-04  
**Status:** DRAFT → READY FOR REVIEW  
**Owner:** Albert  
**Stakeholders:** Norman, Harold (Swarm Foreman), CortexBrain Team

---

## 1. Executive Summary

### 1.1 Problem Statement
AI agents are limited by browser automation speed. Loading pages, waiting for JavaScript, and DOM manipulation takes 10-45 seconds per operation. Meanwhile, modern web apps load data via internal APIs in <200ms. Agents don't know these APIs exist.

### 1.2 Solution
Cortex WebAPI Harvester captures internal APIs during normal browsing and auto-generates reusable skills. This provides **100x speed improvement** (200ms vs 12s) without proprietary dependencies or crypto payments.

### 1.3 Success Criteria
| Metric | Target |
|--------|--------|
| API detection accuracy | >90% |
| Generated skill compile rate | >95% |
| Speedup vs browser automation | 50x minimum |
| Time to skill generation | <30 seconds |
| False positive rate | <5% |

### 1.4 Strategic Alignment
- ✅ Swarm autonomy (no external services)
- ✅ Privacy-first (LAN-only operation)
- ✅ Financial safety (no crypto/payments)
- ✅ Open source (MIT license)
- ✅ Multi-language support (Go/TypeScript/JavaScript)

---

## 2. User Stories

### 2.1 Primary Users

**As Albert (AI Agent)**
- I want to automate web tasks 100x faster
- I need to discover APIs without manual reverse engineering
- I want skills that compile and work immediately

**As Harold (Swarm Foreman)**
- I need to distribute discovered skills to all workers
- I want versioned, tested skills in our GitHub
- I need to track which APIs are reliable

**As Norman (Human Operator)**
- I want to review APIs before agents use them
- I need to know which sites have been harvested
- I want to audit generated code for security

### 2.2 Use Cases

| ID | Use Case | Actor | Priority |
|----|----------|-------|----------|
| UC-001 | Capture APIs while browsing | Albert | P0 |
| UC-002 | Import HAR file from browser | Norman | P1 |
| UC-003 | Generate Go skill from APIs | Albert | P0 |
| UC-004 | Generate TypeScript skill | Albert | P1 |
| UC-005 | Publish skill to GitHub | Harold | P1 |
| UC-006 | Search CortexBrain for existing skills | Albert | P2 |
| UC-007 | Update skill when API changes | Harold | P2 |
| UC-008 | Share skill via A2A bridge | Harold | P2 |

---

## 3. Functional Requirements

### 3.1 Capture (F-CAP)

**F-CAP-001: Chrome DevTools Protocol Capture**
- Connect to Chrome remote debugging port (9222)
- Intercept all network requests/responses
- Capture: URL, method, headers, query params, body, response
- Filter out static assets (JS, CSS, images, fonts)
- Real-time streaming to analyzer

**F-CAP-002: HAR File Import**
- Import HAR 1.2 format files
- Parse entries into internal request format
- Support multiple browser exports
- Validate HAR structure

**F-CAP-003: mitmproxy Integration (Future)**
- Optional system-wide proxy capture
- SSL certificate management
- Mobile app API capture

**F-CAP-004: Session Management**
- Named sessions: `harvest start --name="polymarket-session"`
- Pause/resume capture
- Session metadata (start time, site URL, agent)
- Export raw capture data

### 3.2 Analyze (F-ANZ)

**F-ANZ-001: Endpoint Clustering**
- Group requests by (host + path template + method)
- Detect path parameters: `/api/markets/{marketId}`
- Normalize URLs for comparison

**F-ANZ-002: Parameter Detection**
- Query parameters: `?limit=100&offset=0`
- Body parameters (JSON form fields)
- Header parameters (auth tokens, content-type)
- Detect required vs optional parameters

**F-ANZ-003: Authentication Extraction**
- Detect auth patterns:
  - Bearer token: `Authorization: Bearer {token}`
  - API key: `X-API-Key: {key}`
  - Cookie: `session={value}`
  - Query param: `?api_key={key}`
- Extract token sources (login response, localStorage, etc.)
- Mark sensitive fields (never hardcode in generated code)

**F-ANZ-004: Schema Inference**
- Infer request body schema from samples
- Infer response schema from JSON samples
- Detect common types (string, number, boolean, array, object)
- Generate TypeScript interfaces

**F-ANZ-005: Confidence Scoring**
- Calculate confidence per endpoint (0.0 - 1.0)
- Factors: sample count, consistency, auth presence
- Configurable threshold: `--min-confidence=0.8`
- Flag low-confidence endpoints for manual review

**F-ANZ-006: Pattern Filtering**
- Ignore patterns: analytics, tracking, CDNs
- Configurable filter list
- Machine learning for auto-filtering (Future)

### 3.3 Generate (F-GEN)

**F-GEN-001: Go Skill Generation**
- Generate Go client struct
- Methods per endpoint with proper types
- HTTP client with timeouts
- Error handling with typed errors
- Configurable base URL
- Auth injection (from environment/config)
- Generated tests with mock server

**F-GEN-002: TypeScript Skill Generation**
- Generate TypeScript interfaces
- Async functions with proper return types
- Fetch-based HTTP client
- Config via constructor options
- Type guards for runtime validation

**F-GEN-003: JavaScript Skill Generation**
- ES6+ module format
- JSDoc type annotations
- Promise-based API
- Configurable via options object

**F-GEN-004: Documentation Generation**
- Markdown README per skill
- Endpoint reference table
- Usage examples
- Auth setup instructions

**F-GEN-005: Test Generation**
- Unit tests for each endpoint
- Mock server responses
- Error case coverage
- Integration test scaffold

### 3.4 Store (F-STOR)

**F-STOR-001: Vault Storage**
- Save to `~/ServerProjectsMac/Cortex/Harvester/`
- Structure:
  ```
  sessions/{date}-{name}/
    capture.jsonl
    analysis.json
  skills/{site-name}/
    go/
      client.go
      client_test.go
    ts/
      client.ts
      client.test.ts
    README.md
  ```

**F-STOR-002: CortexBrain Integration**
- Store as knowledge entries
- Category: `api_skill`
- Metadata: endpoints, auth type, reliability score
- Link to vault file path

**F-STOR-003: GitHub Repository**
- Target: `github.com/RedClaus/swarm-skills/`
- Versioned releases (git tags)
- Semantic versioning
- Private repository (security)

**F-STOR-004: Skill Index**
- JSON index of all skills
- Searchable metadata
- Usage statistics
- Reliability ratings

### 3.5 Share (F-SHR)

**F-SHR-001: A2A Bridge Distribution**
- Publish skill_discovered event
- Include: site, endpoints, vault path, GitHub URL
- Workers subscribe and download

**F-SHR-002: CLI Installation**
- `harvest install polymarket` → downloads from GitHub
- Version pinning: `harvest install polymarket@v1.2.0`
- Dependency resolution

**F-SHR-003: Update Notifications**
- Detect API changes (drift detection)
- Notify when site updates endpoints
- Migration guides for breaking changes

---

## 4. Non-Functional Requirements

### 4.1 Performance

**NF-PERF-001: Capture Throughput**
- Handle 1000+ requests/minute without dropping
- Memory usage <500MB during capture
- Non-blocking I/O

**NF-PERF-002: Analysis Speed**
- Analyze 10,000 requests in <5 seconds
- Parallel endpoint clustering
- Incremental analysis (add requests without reprocessing)

**NF-PERF-003: Generation Speed**
- Generate Go skill in <5 seconds
- Generate TypeScript skill in <5 seconds
- Parallel template rendering

### 4.2 Security

**NF-SEC-001: Auth Token Protection**
- Never hardcode tokens in generated code
- Use environment variables: `POLYMARKET_API_KEY`
- Mark sensitive fields in analysis

**NF-SEC-002: PII Filtering**
- Detect and redact PII in captures
- Email, phone, SSN patterns
- Configurable PII rules

**NF-SEC-003: Access Control**
- Vault files readable only by owner
- GitHub repo private
- CortexBrain auth required

### 4.3 Reliability

**NF-REL-001: Error Handling**
- Graceful handling of malformed HAR files
- Retry on CDP connection failures
- Clear error messages

**NF-REL-002: Data Integrity**
- Checksums for capture files
- Atomic writes to prevent corruption
- Backup before overwrite

### 4.4 Usability

**NF-USE-001: CLI Interface**
- Intuitive commands: `start`, `stop`, `analyze`, `generate`
- Progress indicators for long operations
- Colored output for readability
- Help text for all commands

**NF-USE-002: Configuration**
- Config file: `~/.harvester/config.yaml`
- Environment variable overrides
- Sensible defaults

**NF-USE-003: Documentation**
- Comprehensive README
- Tutorial: "Your First Skill"
- Troubleshooting guide
- API reference

---

## 5. Technical Architecture

### 5.1 Component Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                     CORTEX WEBAPI HARVESTER                 │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                    CLI Layer                        │   │
│  │  harvest start │ harvest analyze │ harvest generate  │   │
│  └─────────────────────────────────────────────────────┘   │
│                           │                                 │
│  ┌────────────────────────┼─────────────────────────────┐   │
│  │                        ▼                             │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌─────────────┐│   │
│  │  │   Capture    │  │   Analyze    │  │   Generate  ││   │
│  │  │   Engine     │──│    Engine    │──│   Engine    ││   │
│  │  └──────────────┘  └──────────────┘  └─────────────┘│   │
│  │         │                  │                  │      │   │
│  └─────────┼──────────────────┼──────────────────┼──────┘   │
│            │                  │                  │           │
│  ┌─────────▼──────────────────▼──────────────────▼──────┐   │
│  │                    Storage Layer                     │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────────────┐   │   │
│  │  │   Vault  │  │CortexBrain│  │     GitHub       │   │   │
│  │  │  (Files) │  │ (Knowledge)│  │  (Distribution)  │   │   │
│  │  └──────────┘  └──────────┘  └──────────────────┘   │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 5.2 Module Breakdown

```
cortex-webapi-harvester/
├── cmd/
│   └── harvest/
│       └── main.go              # CLI entry point
├── pkg/
│   ├── capture/
│   │   ├── cdp.go               # Chrome DevTools Protocol
│   │   ├── har.go               # HAR file import
│   │   └── session.go           # Session management
│   ├── analyze/
│   │   ├── cluster.go           # Endpoint clustering
│   │   ├── param.go             # Parameter detection
│   │   ├── auth.go              # Auth extraction
│   │   ├── schema.go            # Schema inference
│   │   └── confidence.go        # Scoring
│   ├── generate/
│   │   ├── go.go                # Go template
│   │   ├── typescript.go        # TypeScript template
│   │   ├── javascript.go        # JavaScript template
│   │   ├── docs.go              # Documentation
│   │   └── test.go              # Test generation
│   ├── storage/
│   │   ├── vault.go             # Vault file storage
│   │   ├── cortexbrain.go       # CortexBrain API
│   │   └── github.go            # GitHub integration
│   ├── share/
│   │   ├── a2a.go               # A2A bridge events
│   │   └── install.go           # CLI install command
│   └── types/
│       ├── request.go           # Internal request types
│       ├── endpoint.go          # Endpoint types
│       └── skill.go             # Skill metadata
├── templates/
│   ├── go/
│   │   ├── client.tmpl
│   │   ├── types.tmpl
│   │   └── test.tmpl
│   ├── typescript/
│   │   ├── client.tmpl
│   │   └── test.tmpl
│   └── javascript/
│       └── client.tmpl
├── config/
│   └── default.yaml
├── scripts/
│   └── install.sh
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

### 5.3 Technology Stack

| Component | Technology | Rationale |
|-----------|-----------|-----------|
| **Language** | Go 1.21+ | Swarm standard, fast, single binary |
| **CDP Client** | chromedp | Production-tested, actively maintained |
| **Templates** | Go text/template | Native, fast, no dependencies |
| **HTTP Client** | net/http | Standard library, sufficient |
| **CLI** | cobra | Industry standard, great UX |
| **Config** | viper | Supports YAML/JSON/env |
| **Logging** | slog | Go 1.21 standard |
| **Testing** | testify | Assertions, mocks |

### 5.4 Dependencies

**Required:**
- Chrome/Chromium (for CDP capture)
- Go 1.21+ (for building)

**Optional:**
- mitmproxy (for proxy capture)
- GitHub CLI (for publishing)

---

## 6. API Specification

### 6.1 CLI Commands

```bash
# Session management
harvest start --name="polymarket" --browser=chrome
harvest pause --name="polymarket"
harvest resume --name="polymarket"
harvest stop --name="polymarket"

# Analysis
harvest analyze --session="polymarket" --min-confidence=0.8
harvest import --file="polymarket.har" --name="polymarket"

# Generation
harvest generate --session="polymarket" --format=go --output=./skills/
harvest generate --session="polymarket" --format=typescript --output=./skills/

# Distribution
harvest publish --site="polymarket" --version="1.0.0"
harvest install --site="polymarket"
harvest update --site="polymarket"

# Discovery
harvest list                          # List all skills
harvest search "prediction market"    # Search by keyword
harvest info "polymarket"             # Show skill details

# Utilities
harvest validate --skill=./skills/polymarket/
harvest test --skill=./skills/polymarket/
harvest config --set="browser.port=9222"
```

### 6.2 Configuration Schema

```yaml
# ~/.harvester/config.yaml
version: "1.0"

browser:
  port: 9222
  headless: false
  user_data_dir: "~/.harvester/chrome-profile"

capture:
  ignore_patterns:
    - "*.js"
    - "*.css"
    - "*.png"
    - "*.jpg"
    - "analytics.*"
    - "tracking.*"
  max_request_size: "10MB"
  max_response_size: "10MB"

analysis:
  min_confidence: 0.8
  min_sample_count: 3
  enable_pii_filter: true

storage:
  vault_dir: "~/ServerProjectsMac/Cortex/Harvester/"
  cortexbrain_url: "http://192.168.1.186:18892"
  github_repo: "github.com/RedClaus/swarm-skills"

generation:
  default_format: "go"
  include_tests: true
  include_docs: true
  auth_env_prefix: "HARVESTER_"

share:
  a2a_bridge_url: "http://192.168.1.128:18802"
  auto_publish: false
```

---

## 7. Data Models

### 7.1 Capture Session

```go
type Session struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    SiteURL   string    `json:"site_url"`
    StartedAt time.Time `json:"started_at"`
    EndedAt   *time.Time `json:"ended_at,omitempty"`
    Status    string    `json:"status"` // "capturing", "paused", "completed"
    Requests  []Request `json:"requests"`
    Metadata  Metadata  `json:"metadata"`
}

type Request struct {
    ID        string            `json:"id"`
    Timestamp time.Time         `json:"timestamp"`
    Method    string            `json:"method"`
    URL       string            `json:"url"`
    Headers   map[string]string `json:"headers"`
    Body      []byte            `json:"body,omitempty"`
    Response  *Response         `json:"response,omitempty"`
}

type Response struct {
    StatusCode int               `json:"status_code"`
    Headers    map[string]string `json:"headers"`
    Body       []byte            `json:"body,omitempty"`
    Duration   time.Duration     `json:"duration"`
}
```

### 7.2 API Endpoint

```go
type Endpoint struct {
    ID             string      `json:"id"`
    Host           string      `json:"host"`
    PathTemplate   string      `json:"path_template"` // "/api/markets/{id}"
    Method         string      `json:"method"`
    Parameters     []Parameter `json:"parameters"`
    AuthType       string      `json:"auth_type"` // "bearer", "api_key", "cookie"
    RequestSchema  *Schema     `json:"request_schema,omitempty"`
    ResponseSchema *Schema     `json:"response_schema,omitempty"`
    SampleCalls    []Call      `json:"sample_calls"`
    Confidence     float64     `json:"confidence"`
    Tags           []string    `json:"tags"`
}

type Parameter struct {
    Name     string `json:"name"`
    Location string `json:"location"` // "path", "query", "header", "body"
    Type     string `json:"type"`     // "string", "number", "boolean"
    Required bool   `json:"required"`
    Sensitive bool  `json:"sensitive"` // Don't hardcode
}

type Schema struct {
    Type       string            `json:"type"`
    Properties map[string]Schema `json:"properties,omitempty"`
    Items      *Schema           `json:"items,omitempty"`
    Required   []string          `json:"required,omitempty"`
}
```

### 7.3 Skill Metadata

```go
type Skill struct {
    Name        string     `json:"name"`
    SiteURL     string     `json:"site_url"`
    Version     string     `json:"version"`
    Description string     `json:"description"`
    Endpoints   []Endpoint `json:"endpoints"`
    AuthType    string     `json:"auth_type"`
    Languages   []string   `json:"languages"` // ["go", "typescript"]
    GeneratedAt time.Time  `json:"generated_at"`
    GeneratedBy string     `json:"generated_by"`
    Reliability float64    `json:"reliability"` // 0.0-1.0
    VaultPath   string     `json:"vault_path"`
    GitHubURL   string     `json:"github_url,omitempty"`
}
```

---

## 8. Security & Privacy

### 8.1 Data Handling

| Data Type | Storage | Encryption | Retention |
|-----------|---------|------------|-----------|
| Capture sessions | Vault | File permissions | 90 days |
| Auth tokens | Keychain | macOS keychain | Persistent |
| Generated skills | GitHub | Repository private | Indefinite |
| CortexBrain entries | CortexBrain | HTTPS | Indefinite |

### 8.2 Sensitive Data Rules

1. **Never hardcode:** Auth tokens, API keys, passwords
2. **Always use:** Environment variables or secure config
3. **Mark clearly:** Sensitive fields in analysis
4. **Redact by default:** PII in captures unless explicitly enabled

### 8.3 Audit Trail

```go
type AuditLog struct {
    Timestamp   time.Time `json:"timestamp"`
    Action      string    `json:"action"`      // "capture_started", "skill_generated"
    SessionID   string    `json:"session_id"`
    SiteURL     string    `json:"site_url"`
    Endpoints   int       `json:"endpoints"`
    GeneratedBy string    `json:"generated_by"`
}
```

---

## 9. Testing Strategy

### 9.1 Test Levels

| Level | Coverage | Tools |
|-------|----------|-------|
| **Unit** | Individual functions | Go test, testify |
| **Integration** | Capture → Analyze → Generate | Test fixtures |
| **E2E** | Full CLI workflow | Bats (bash tests) |
| **Performance** | 1000+ requests | Benchmarks |

### 9.2 Test Fixtures

```
testdata/
├── har/
│   ├── polymarket.har
│   └── github.har
├── captured/
│   └── sample-session.json
└── expected/
    ├── polymarket-go/
    └── polymarket-ts/
```

### 9.3 CI/CD Pipeline

```yaml
# .github/workflows/test.yml
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
      - run: make test
      - run: make test-integration
      - run: make test-e2e
```

---

## 10. Implementation Phases

### Phase 1: Foundation (Week 1)
**Goal:** Core capture and analysis

**Tasks:**
- [ ] Project scaffolding (repo, CI, docs)
- [ ] CDP capture engine
- [ ] Basic endpoint clustering
- [ ] Session management
- [ ] CLI framework

**Deliverables:**
- `harvest start` and `harvest stop` work
- Capture files saved to vault
- Unit tests passing

### Phase 2: Generation (Week 2)
**Goal:** Skill generation in Go and TypeScript

**Tasks:**
- [ ] Go template and generation
- [ ] TypeScript template and generation
- [ ] Parameter detection improvements
- [ ] Auth extraction
- [ ] Test generation

**Deliverables:**
- `harvest generate --format=go` produces working code
- Generated skills compile and pass tests

### Phase 3: Distribution (Week 3)
**Goal:** Storage and sharing

**Tasks:**
- [ ] CortexBrain integration
- [ ] GitHub publishing
- [ ] A2A bridge events
- [ ] `harvest install` command
- [ ] Skill search and listing

**Deliverables:**
- Skills published to GitHub
- A2A distribution working
- End-to-end workflow complete

### Phase 4: Polish (Week 4)
**Goal:** Production readiness

**Tasks:**
- [ ] Performance optimization
- [ ] Security audit
- [ ] Documentation complete
- [ ] Error handling robustness
- [ ] User feedback integration

**Deliverables:**
- v1.0.0 release
- Production deployment guide
- Training materials

---

## 11. Open Questions

| Question | Priority | Owner | Target Date |
|----------|----------|-------|-------------|
| Which 3-5 sites for initial pilot? | P0 | Norman | Week 1 |
| Go vs TypeScript priority for generation? | P0 | Albert | Week 1 |
| Do we need JavaScript output? | P1 | Albert | Week 2 |
| GitHub repo: new or existing? | P0 | Harold | Week 1 |
| CortexBrain knowledge schema finalization | P1 | Albert | Week 2 |
| Rate limiting for generated clients? | P2 | — | Week 3 |

---

## 12. Appendix

### A. Glossary

| Term | Definition |
|------|------------|
| **API Harvester** | Tool that captures and reverse-engineers web APIs |
| **CDP** | Chrome DevTools Protocol |
| **HAR** | HTTP Archive format |
| **Skill** | Generated code client for a specific API |
| **Endpoint** | Specific API URL + method combination |
| **Session** | Capture period with metadata |

### B. References

- Chrome DevTools Protocol: https://chromedevtools.github.io/devtools-protocol/
- HAR 1.2 Spec: http://www.softwareishard.com/blog/har-12-spec/
- chromedp: https://github.com/chromedp/chromedp
- Unbrowse evaluation: `Cortex/Evaluations/Unbrowse-OpenClaw-Evaluation.md`

### C. Related Documents

- `Cortex/Design/Swarm-API-Harvester.md` — Initial design
- `Cortex/TODO-Cortex-Avatar.md` — Related GUI agent work
- `memory/a2a-migration-plan.md` — A2A protocol integration

---

**Document Control:**
- **Version:** 1.0
- **Last Updated:** 2026-02-04
- **Next Review:** Phase 1 completion (Week 1)
- **Status:** ✅ APPROVED FOR DEVELOPMENT

**Signatures:**
- [ ] Norman (Product Owner)
- [ ] Albert (Technical Lead)
- [ ] Harold (Swarm Foreman)
