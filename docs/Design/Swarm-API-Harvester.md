---
project: Cortex
component: Agents
phase: Ideation
date_created: 2026-02-04T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:29.728529
---

# Design: Swarm API Harvester — Open-Source Unbrowse Alternative

**Date:** 2026-02-04  
**Status:** DESIGN PROPOSAL  
**Objective:** Build internal API reverse-engineering capability without proprietary/crypto dependencies

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                     SWARM API HARVESTER                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐      │
│  │   CAPTURE    │───▶│   ANALYZE    │───▶│   GENERATE   │      │
│  └──────────────┘    └──────────────┘    └──────────────┘      │
│         │                   │                   │               │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐      │
│  │ Chrome CDP   │    │ Pattern      │    │ Skill        │      │
│  │ HAR files    │    │ Detection    │    │ Templates    │      │
│  │ mitmproxy    │    │ Auth Extract │    │ Go/TS/JS     │      │
│  └──────────────┘    └──────────────┘    └──────────────┘      │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     STORAGE & DISTRIBUTION                      │
├─────────────────────────────────────────────────────────────────┤
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐      │
│  │ CortexBrain  │    │ GitHub Repo  │    │ A2A Bridge   │      │
│  │ Knowledge    │    │ Private      │    │ Skill Share  │      │
│  └──────────────┘    └──────────────┘    └──────────────┘      │
└─────────────────────────────────────────────────────────────────┘
```

---

## Component Breakdown

### 1. CAPTURE Layer (Multiple Options)

#### Option A: Chrome DevTools Protocol (CDP) — RECOMMENDED
```bash
# Chrome launched with remote debugging
chrome --remote-debugging-port=9222

# Go CDP client connects and captures:
# - Network.requestWillBeSent
# - Network.responseReceived
# - Network.loadingFinished
```

**Go Library:** `github.com/chromedp/chromedp` (already used in swarm)

**Pros:**
- No proxy needed
- Access to request/response bodies
- Works with existing browser automation

**Cons:**
- Chrome-specific

#### Option B: HAR File Import
```bash
# Browser exports HAR file manually
# Or Chrome DevTools "Save all as HAR"

# Tool parses HAR JSON:
harvest import ~/Downloads/site.har --name="polymarket"
```

**Pros:**
- Universal (all browsers)
- No runtime integration needed
- Manual audit before processing

**Cons:**
- Manual step required
- No real-time capture

#### Option C: mitmproxy (Man-in-the-Middle)
```bash
# Proxy captures all HTTP(S) traffic
mitmproxy --mode regular --listen-port 8080

# SSL certs installed on system
# All traffic decrypted and logged
```

**Go Library:** `github.com/mitmproxy/mitmproxy`

**Pros:**
- Captures all apps (not just browser)
- Full request/response visibility

**Cons:**
- Certificate management complexity
- System-wide impact

---

### 2. ANALYZE Layer (Pattern Detection)

```go
package harvester

// APIEndpoint represents a discovered API
type APIEndpoint struct {
    ID          string            `json:"id"`
    Host        string            `json:"host"`
    Path        string            `json:"path"`
    Method      string            `json:"method"`
    Parameters  []Parameter       `json:"parameters"`
    AuthType    AuthType          `json:"auth_type"`
    ResponseSchema ResponseSchema `json:"response_schema"`
    SampleCalls []APICall         `json:"sample_calls"`
    Confidence  float64           `json:"confidence"`
}

type Analyzer struct {
    minConfidence float64
    ignorePatterns []regexp.Regexp
}

func (a *Analyzer) Analyze(requests []NetworkRequest) []APIEndpoint {
    // Group by endpoint (host + path + method)
    // Detect parameter types (path, query, body)
    // Extract auth patterns (Bearer, cookies, API keys)
    // Infer response schema from samples
    // Calculate confidence score
}
```

**Detection Rules:**
```yaml
# Static assets to ignore
ignore:
  - "*.js"
  - "*.css"
  - "*.png"
  - "*.jpg"
  - "*.woff2"
  - "analytics.*"
  - "tracking.*"

# Auth patterns
auth:
  bearer:
    header: "Authorization"
    pattern: "Bearer (.+)"
  api_key:
    header: "X-API-Key"
  cookie:
    header: "Cookie"
    pattern: "session=([^;]+)"
```

---

### 3. GENERATE Layer (Skill Templates)

#### Go Skill Template
```go
// Auto-generated: {{.SiteName}} API Client
// Generated: {{.Timestamp}}
// Source: {{.SourceURL}}

package skills

import (
    "encoding/json"
    "fmt"
    "net/http"
)

type {{.ClientName}} struct {
    baseURL    string
    httpClient *http.Client
    {{range .AuthFields}}
    {{.Name}} string{{end}}
}

{{range .Endpoints}}
// {{.Description}}
func (c *{{.ClientName}}) {{.FunctionName}}({{range .Parameters}}{{.Name}} {{.Type}}, {{end}}) ({{.ReturnType}}, error) {
    url := fmt.Sprintf("%s{{.Path}}", c.baseURL{{range .PathParams}}, {{.Name}}{{end}})
    {{if .HasQueryParams}}
    req, err := http.NewRequest("{{.Method}}", url, nil)
    if err != nil {
        return nil, err
    }
    q := req.URL.Query()
    {{range .QueryParams}}
    q.Add("{{.Key}}", {{.Value}})
    {{end}}
    req.URL.RawQuery = q.Encode()
    {{end}}
    // ... implementation
}
{{end}}
```

#### TypeScript/JavaScript Skill Template
```typescript
// Auto-generated: {{siteName}} API Client

interface {{clientName}}Config {
  baseURL: string;
  {{#authFields}}
  {{name}}: string;
  {{/authFields}}
}

{{#endpoints}}
/**
 * {{description}}
 */
export async function {{functionName}}(
  config: {{clientName}}Config,
  {{#parameters}}
  {{name}}: {{type}},
  {{/parameters}}
): Promise<{{returnType}}> {
  const url = new URL(`{{path}}`, config.baseURL);
  {{#queryParams}}
  url.searchParams.set('{{key}}', {{value}});
  {{/queryParams}}
  
  const response = await fetch(url.toString(), {
    method: '{{method}}',
    headers: {
      {{#auth}}
      '{{header}}': `{{prefix}}${config.{{field}}}`,
      {{/auth}}
      'Content-Type': 'application/json',
    },
    {{#body}}
    body: JSON.stringify({{{body}}}),
    {{/body}}
  });
  
  if (!response.ok) {
    throw new Error(`HTTP ${response.status}: ${response.statusText}`);
  }
  return response.json();
}
{{/endpoints}}
```

---

## Implementation Plan

### Phase 1: Core Harvester (1 week)
```bash
# New repository
cortex-harvester/
├── cmd/harvest/           # CLI tool
├── pkg/capture/           # CDP/HAR/mitmproxy
├── pkg/analyze/           # Pattern detection
├── pkg/generate/          # Code generation
├── pkg/storage/           # CortexBrain/GitHub integration
└── templates/             # Go/TS/JS templates
```

**CLI Commands:**
```bash
# Start capture session
harvest start --browser=chrome --output=./session/

# Import HAR file
harvest import ./site.har --name="polymarket"

# Analyze captured traffic
harvest analyze ./session/ --min-confidence=0.8

# Generate skill
harvest generate --format=go --output=./skills/

# Publish to swarm
harvest publish --site="polymarket" --repo=github.com/RedClaus/swarm-skills
```

### Phase 2: Integration (1 week)
- OpenClaw plugin wrapper
- A2A bridge skill sharing
- CortexBrain knowledge storage

### Phase 3: Swarm Distribution (1 week)
- Skill registry in CortexBrain
- Auto-discovery by agents
- Version management

---

## Storage Strategy

### Local Storage (Vault)
```
~/ServerProjectsMac/Cortex/Harvester/
├── sessions/
│   └── 2026-02-04-polymarket/
│       ├── requests.jsonl
│       └── analysis.json
├── skills/
│   └── polymarket/
│       ├── client.go
│       └── client_test.go
└── index.json
```

### CortexBrain Knowledge
```json
{
  "type": "api_skill",
  "site": "polymarket",
  "endpoints": [
    {
      "path": "/api/markets",
      "method": "GET",
      "function": "GetMarkets",
      "latency_ms": 200,
      "reliability": 0.95
    }
  ],
  "generated_at": "2026-02-04T11:30:00Z",
  "vault_path": "Cortex/Harvester/skills/polymarket/"
}
```

### GitHub Distribution
```bash
# Private repo for swarm skills
github.com/RedClaus/swarm-skills/
├── polymarket/
│   ├── v1.0.0/
│   │   ├── client.go
│   │   ├── client.ts
│   │   └── README.md
│   └── latest -> v1.0.0
└── index.json
```

---

## Comparison: Swarm Harvester vs Unbrowse

| Feature | Unbrowse | Swarm Harvester |
|---------|----------|-----------------|
| **License** | Proprietary | ✅ MIT/Open Source |
| **Payments** | x402/USDC | ✅ None |
| **Self-hosted** | ❌ Cloud-dependent | ✅ Fully local |
| **Marketplace** | Centralized | ✅ GitHub/A2A distributed |
| **Crypto** | Required | ✅ None |
| **Skill formats** | TypeScript only | ✅ Go/TS/JS |
| **Integration** | OpenClaw plugin | ✅ Native swarm tools |
| **Speedup** | 100x | ✅ 100x (same approach) |
| **Setup** | npm install | ✅ Go binary |

---

## Advantages Over Unbrowse

1. **No Financial Risk** — No crypto, no payments, no private keys
2. **Fully Open Source** — Auditable, modifiable, no vendor lock-in
3. **Swarm Native** — Integrates with A2A, CortexBrain, existing tools
4. **Multi-language** — Generate Go for backend, TS for frontend
5. **Privacy First** — No external services, data stays in swarm
6. **Git-based Distribution** — Skills versioned, reviewed, shared via GitHub

---

## Key Technical Decisions

### 1. CDP vs HAR vs Proxy
**Decision:** Support all three
- **CDP:** Default for real-time (best for automation)
- **HAR:** Import for manual audit (best for security)
- **Proxy:** Optional for system-wide (best for mobile apps)

### 2. Go vs TypeScript
**Decision:** Go for harvester, templates for all languages
- Harvester CLI in Go (fits swarm)
- Templates generate Go/TS/JS skills

### 3. Storage
**Decision:** Vault + CortexBrain + GitHub
- Vault: Source of truth (files)
- CortexBrain: Searchable index
- GitHub: Distribution to all agents

---

## Integration with Existing Tools

### Browser Automation
```go
// Current browser tool
browser open https://polymarket.com
browser click "Markets"

// With harvester running:
// Automatically captures API calls
// Generates skill after session

// Future: Use generated skill
skill polymarket getMarkets  // 200ms instead of 12s
```

### A2A Bridge
```json
{
  "type": "skill_discovered",
  "site": "polymarket",
  "endpoints": 12,
  "skill_path": "github.com/RedClaus/swarm-skills/polymarket",
  "discovered_by": "albert",
  "confidence": 0.92
}
```

### CortexBrain
```
POST /api/v1/knowledge
{
  "category": "api_skill",
  "content": "Polymarket internal API endpoints",
  "metadata": {
    "endpoints": [...],
    "vault_path": "..."
  }
}
```

---

## Success Metrics

| Metric | Target |
|--------|--------|
| API detection accuracy | >90% |
| Generated skill compile rate | >95% |
| Speedup vs browser | 50x minimum |
| False positive rate | <5% |
| Time to skill generation | <30s |

---

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| ToS violations | Only use on sites you own/permission |
| Auth token leakage | Store in keychain, never in generated code |
| API changes | Version skills, detect drift |
| PII capture | Filtering rules, manual review |

---

## Next Steps

1. **Create repo:** `github.com/RedClaus/cortex-harvester`
2. **Phase 1:** CDP capture + basic analysis (week 1)
3. **Phase 2:** Go skill generation + CLI (week 2)
4. **Phase 3:** Integration + testing (week 3)
5. **Pilot:** Target 3-5 common sites for initial skills

---

**Estimated Effort:** 3 weeks (1 person)  
**Value:** 100x speedup on web automation, swarm-native, no external dependencies  
**Priority:** P2 — High value, medium effort, aligns with swarm autonomy goals

---

**References:**
- chromedp: https://github.com/chromedp/chromedp
- mitmproxy: https://mitmproxy.org
- HAR spec: http://www.softwareishard.com/blog/har-12-spec/
- Unbrowse evaluation: `Cortex/Evaluations/Unbrowse-OpenClaw-Evaluation.md`
