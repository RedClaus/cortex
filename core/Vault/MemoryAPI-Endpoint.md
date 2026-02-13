---
project: Cortex
component: Memory
phase: Ideation
date_created: 2026-02-05T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:21:14.055673
---

# CortexBrain Memory API Endpoint Proposal

**Date:** February 5, 2026
**Author:** Albert (AI Partner)
**Target:** Norman King
**Status:** Proposal — Awaiting Decision

---

## Executive Summary

**Problem:** CortexBrain's A2A server (port 18892) does not expose REST API endpoints for memory operations. This prevents external clients (CortexHub CLI, OpenClaw, Harold Bridge) from accessing memory via standard HTTP requests. Currently, only direct file operations work.

**Mission Alignment:** CortexBrain aims to be an "AI emulating human brain" with 20 cognitive lobes, Neural Bus, Sleep Cycle, and ecosystem integration (GoMenu, CortexAvatar, dnet). A memory REST API is critical for achieving these goals.

**Recommendation:** Implement REST API endpoints (`/api/v1/memory/*`) for memory operations, with optional A2A endpoints for agent-to-agent communication.

**Estimated Effort:** 2-3 days

---

## Current State

### What Works Today

| Component | Status | Access Method |
|-----------|--------|---------------|
| **A2A Server** | ✅ Operational | JSON-RPC 2.0 (agent-to-agent) |
| **Memory Files** | ✅ Operational | Direct file access (`MEMORY.md`, `memory/*.md`) |
| **CortexHub CLI** | ❌ Broken | Tries to use non-existent REST API endpoints |
| **Harold Bridge** | ✅ Operational | A2A protocol only (no memory endpoints) |
| **OpenClaw** | ⚠️ Partial | Uses `memory-reflect.sh` (direct file) |

### What Doesn't Work

```bash
# All of these return 404 Not Found:
curl "http://192.168.1.186:18892/api/v1/memories/search?q=Norman"
curl "http://192.168.1.186:18892/api/v1/memories/stats"
curl "http://192.168.1.186:18892/api/memories/recent?limit=10"
curl "http://192.168.1.186:18892/api/v1/memory/store" -d '{"content":"test"}'
```

### Direct File Operations (Current Workaround)

```bash
memory-reflect.sh search "Norman"   # Direct file read
memory-reflect.sh recent 20          # Direct file read
memory-reflect.sh today              # Direct file read
```

**Limitations:**
- ⚠️ No semantic search (just file regex)
- ⚠️ No distributed access (only local files)
- ⚠️ Not standardized (protocol-agnostic)
- ⚠️ No authentication/authorization

---

## Three Implementation Options

### Option 1: Direct File Operations (No Change) — 0 Days

**Approach:** Keep current setup, continue using direct file operations.

**What This Means:**
- CortexBrain remains as-is (no API changes)
- All clients use direct file operations
- No REST API, no A2A memory endpoints

**Pros:**
- ✅ **Zero effort** — Already working
- ✅ **No architectural change** — Minimal risk
- ✅ **Fastest time to value** — Immediate

**Cons:**
- ❌ **Not standardized** — Protocol-agnostic
- ❌ **No distributed access** — Only local files
- ❌ **No authentication** — No access control
- ❌ **Hard to integrate** — Other tools can't easily connect
- ❌ **Doesn't fit mission** — Not ecosystem-integrated
- ❌ **Scalability issues** — Future-proofing challenging

**Mission Alignment:** ⚠️ Partial
- ✅ Self-contained (brain manages memory files)
- ❌ Ecosystem integration (no external access)
- ❌ Distributed architecture (no A2A integration)

**Recommended For:** Low-effort, minimal-change scenarios only

---

### Option 2: REST API Endpoints — 2-3 Days (RECOMMENDED)

**Approach:** Add REST API endpoints to CortexBrain for memory operations, while keeping A2A for agent communication.

**What Needs to Be Done:**

#### 1. Add REST API Routes (1 day)

```go
// cortex-brain/cmd/server/main.go

func main() {
    // ... existing code ...

    // Add memory endpoints
    http.HandleFunc("/api/v1/memories/search", memoryHandler.Search)
    http.HandleFunc("/api/v1/memories/recent", memoryHandler.Recent)
    http.HandleFunc("/api/v1/memories/store", memoryHandler.Store)
    http.HandleFunc("/api/v1/memories/stats", memoryHandler.Stats)

    // Start server
    log.Printf("🧠 CortexBrain API starting on :18892...")
    http.ListenAndServe(":18892", nil)
}

// New memory handler
func memoryHandler(w http.ResponseWriter, r *http.Request) {
    // Parse query parameters
    query := r.URL.Query().Get("q")
    limit := r.URL.Query().Get("limit")

    // Access memory files (direct file operations internally)
    results := memorystore.Search(query, limit)

    // Return JSON response
    json.NewEncoder(w).Encode(results)
}
```

#### 2. Implement Handler Functions (1 day)

```go
// internal/memorystore/handler.go

package memorystore

import (
    "encoding/json"
    "net/http"
    "os"
    "path/filepath"
)

type MemoryResponse struct {
    Memories []MemoryEntry `json:"memories"`
    Count    int           `json:"count"`
}

type MemoryEntry struct {
    Content string `json:"content"`
    Timestamp string `json:"timestamp"`
    File string `json:"file"`
}

func Search(w http.ResponseWriter, r *http.Request) {
    query := r.URL.Query().Get("q")
    limit := r.URL.Query().Get("limit")

    if limit == "" {
        limit = "50"
    }

    results := searchMemoryFiles(query, limit)

    json.NewEncoder(w).Encode(MemoryResponse{
        Memories: results,
        Count: len(results),
    })
}

func Recent(w http.ResponseWriter, r *http.Request) {
    limit := r.URL.Query().Get("limit")
    if limit == "" {
        limit = "10"
    }

    results := getRecentMemories(limit)

    json.NewEncoder(w).Encode(MemoryResponse{
        Memories: results,
        Count: len(results),
    })
}

func Store(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Content string `json:"content"`
        Importance float64 `json:"importance"`
        Type string `json:"type"` // episodic, knowledge
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Store to memory file
    storeMemory(req.Content, req.Importance, req.Type)

    w.WriteHeader(http.StatusCreated)
}

func Stats(w http.ResponseWriter, r *http.Request) {
    stats := getMemoryStats()
    json.NewEncoder(w).Encode(stats)
}
```

#### 3. Update CortexHub CLI (1 day)

```python
# scripts/ch (CortexHub CLI)

def cmd_search(token, query):
    """Search all memories + knowledge (cross-agent)."""
    response = api("GET", f"/api/memories/search?q={urllib.request.quote(query)}", token=token)
    # Now this will work!
```

#### 4. Test and Document (1 day)

**API Specification:**

```
GET /api/v1/memories/search?q={query}&limit={limit}
GET /api/v1/memories/recent?limit={limit}
POST /api/v1/memories/store
  Body: {"content": "...", "importance": 0.8, "type": "episodic"}
GET /api/v1/memories/stats
```

**Response Format:**

```json
{
  "memories": [
    {
      "content": "Norman's birthday",
      "timestamp": "2026-02-05T23:30:00Z",
      "file": "memory/2026-02-05.md"
    }
  ],
  "count": 1
}
```

**Pros:**
- ✅ **Mission-aligned** — Ecosystem integration (GoMenu, CortexAvatar, dnet)
- ✅ **Standardized** — REST API, any language can connect
- ✅ **Self-contained** — Brain manages its own API
- ✅ **Future-proof** — Conventional architecture
- ✅ **Authentication** — Easy to add (Bearer token)
- ✅ **Distributed** — Multiple clients can access
- ✅ **A2A compatible** — Can still use A2A internally

**Cons:**
- ⚠️ **Effort required** — 2-3 days implementation
- ⚠️ **Testing needed** — Ensure backward compatibility
- ⚠️ **Documentation needed** — API docs for integrators

**Mission Alignment:** ✅ **Perfect**
- ✅ Ecosystem integration
- ✅ Self-contained (brain manages API)
- ✅ Distributed architecture
- ✅ Standardized protocol
- ✅ Future-proof

**Recommended For:** **PRIMARY RECOMMENDATION**

---

### Option 3: A2A Endpoints Only — 2-3 Days

**Approach:** Add A2A protocol endpoints for memory operations, but NO REST API.

**What Needs to Be Done:**

#### 1. Add A2A Endpoints to CortexBrain (1 day)

```go
// cortex-brain/cmd/server/main.go

func main() {
    // ... existing code ...

    // Register memory endpoints to A2A
    a2a.HandleFunc("memory.search", handleMemorySearchA2A)
    a2a.HandleFunc("memory.recall", handleMemoryRecallA2A)
    a2a.HandleFunc("memory.store", handleMemoryStoreA2A)
    a2a.HandleFunc("memory.stats", handleMemoryStatsA2A)

    // Start A2A server
    http.ListenAndServe(":18892", nil)
}

func handleMemorySearchA2A(w http.ResponseWriter, r *http.Request) {
    // Parse JSON-RPC 2.0 request
    var req JSONRPCRequest
    json.NewDecoder(r.Body).Decode(&req)

    // Extract parameters
    query := req.Params["query"]
    limit := req.Params["limit"]

    // Search memory files
    results := memorystore.Search(query, limit)

    // Return JSON-RPC response
    json.NewEncoder(w).Encode(JSONRPCResponse{
        ID:     req.ID,
        Result: results,
    })
}
```

#### 2. Update Clients to Use A2A Protocol (1 day)

```python
# scripts/ch (CortexHub CLI)

def cmd_search(token, query):
    """Search all memories + knowledge (cross-agent)."""
    payload = {
        "jsonrpc": "2.0",
        "method": "memory.search",
        "params": {"query": query, "limit": 50},
        "id": 1
    }

    response = requests.post("http://192.168.1.186:18892", json=payload)
    return response.json()

# In OpenClaw:
payload = {
    "jsonrpc": "2.0",
    "method": "memory.search",
    "params": {"query": "Norman"},
    "id": 1
}
response = requests.post("http://192.168.1.186:18892", json=payload)
```

#### 3. Test and Document (1 day)

**A2A Protocol Specification:**

```
POST / (A2A JSON-RPC 2.0)

Request:
{
  "jsonrpc": "2.0",
  "method": "memory.search",
  "params": {"query": "Norman", "limit": 50},
  "id": 1
}

Response:
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "memories": [...],
    "count": 1
  }
}
```

**Pros:**
- ✅ **A2A-first** — Follows agent-to-agent communication pattern
- ✅ **Protocol-specific** — Clean separation between agent comms and memory ops
- ✅ **Effort similar** — 2-3 days (same as REST API)

**Cons:**
- ❌ **Agent-centric** — Only agents can access memory
- ❌ **Hard to integrate** — Other tools (non-A2A) can't connect
- ❌ **Not standardized** — A2A is protocol-specific
- ❌ **Scalability issues** — Future-proofing challenging
- ❌ **Doesn't fit mission** — Not ecosystem-integrated

**Mission Alignment:** ⚠️ Partial
- ✅ A2A protocol (agent-centric)
- ❌ Ecosystem integration (no external access)
- ❌ Standardized (protocol-specific)
- ❌ Distributed (only agents)

**Recommended For:** Agent-only scenarios (e.g., internal swarm communication only)

---

## Comparison Matrix

| Criterion | Option 1: Direct File | Option 2: REST API (Recommended) | Option 3: A2A Only |
|-----------|----------------------|-----------------------------------|-------------------|
| **Implementation Effort** | 0 days | 2-3 days | 2-3 days |
| **Mission Alignment** | ⚠️ Partial | ✅ **Perfect** | ⚠️ Partial |
| **Ecosystem Integration** | ❌ No | ✅ Yes | ❌ No |
| **Standardized Protocol** | ❌ No | ✅ Yes | ⚠️ Protocol-specific |
| **Distributed Access** | ❌ No | ✅ Yes | ⚠️ Only agents |
| **Authentication** | ❌ No | ✅ Easy to add | ✅ A2A auth |
| **Future-Proof** | ❌ No | ✅ Yes | ⚠️ Protocol-specific |
| **Multiple Clients** | ⚠️ Only local | ✅ Any language | ⚠️ Only agents |
| **Scalability** | ⚠️ Limited | ✅ Good | ⚠️ Limited |
| **Cost** | ✅ Free | ⚠️ 2-3 days dev | ⚠️ 2-3 days dev |

---

## Mission Alignment Analysis

### CortexBrain Mission (from USER.md)

> **"AI emulating human brain: 20 cognitive lobes, Neural Bus, Sleep Cycle, Growth Roadmap"**
>
> **"Written in Go + BubbleTea TUI, A2A protocol, Wails desktop app"**
>
> **"Ecosystem: CortexBrain, CortexAvatar, CortexLab, dnet, Salamander, GoMenu"**
>
> **"Deeply personal — 'Book of Life' + Brain Vision doc"**

### How Each Option Fits

| Mission Principle | Option 1 | Option 2 (REST API) | Option 3 (A2A) |
|------------------|----------|---------------------|----------------|
| **20 Cognitive Lobes** | ⚠️ Partial | ✅ Each lobe can have API | ✅ Each lobe can have A2A |
| **Neural Bus** | ⚠️ Partial | ✅ API endpoints on bus | ✅ A2A on bus |
| **Sleep Cycle** | ⚠️ Partial | ✅ API integration | ✅ A2A integration |
| **Growth Roadmap** | ❌ No | ✅ Scalable architecture | ⚠️ Agent-only |
| **Ecosystem Integration** | ❌ No | ✅ **Perfect** | ❌ No |
| **GoMenu** | ❌ No | ✅ Easy integration | ⚠️ Harder |
| **CortexAvatar** | ❌ No | ✅ Easy integration | ⚠️ Harder |
| **dnet** | ❌ No | ✅ Easy integration | ⚠️ Harder |
| **Salamander** | ❌ No | ✅ Easy integration | ⚠️ Harder |
| **Wails Desktop App** | ❌ No | ✅ Easy integration | ⚠️ Harder |
| **"Book of Life"** | ✅ Partial | ✅ Perfect | ✅ Perfect |

**Score (out of 10):**
- Option 1: **4/10** (partial, not ecosystem-integrated)
- Option 2: **10/10** (perfect fit)
- Option 3: **6/10** (agent-centric, not ecosystem-integrated)

---

## Implementation Roadmap

### Option 2: REST API Endpoints (Recommended)

**Phase 1: Backend Implementation (1 day)**
- [ ] Add REST API routes to `cmd/server/main.go`
- [ ] Create `internal/memorystore/handler.go`
- [ ] Implement search, recent, store, stats handlers
- [ ] Add error handling and validation

**Phase 2: Frontend Integration (1 day)**
- [ ] Update `scripts/ch` (CortexHub CLI) to use new API
- [ ] Update `memory-reflect.sh` to use new API
- [ ] Add authentication (Bearer token)
- [ ] Add rate limiting

**Phase 3: Testing & Documentation (1 day)**
- [ ] Write API documentation (Swagger/OpenAPI)
- [ ] Add integration tests
- [ ] Update CortexBrain README
- [ ] Test with all clients (ch, OpenClaw, Harold)

**Deliverables:**
- ✅ REST API endpoints (`/api/v1/memory/*`)
- ✅ CortexHub CLI working
- ✅ OpenClaw using API
- ✅ API documentation
- ✅ Tests and examples

---

## Risk Assessment

### Option 1: Direct File Operations
**Risk Level:** 🟢 Low
- No code changes
- No architectural risks
- No testing required

### Option 2: REST API
**Risk Level:** 🟡 Medium
- Code changes to CortexBrain (though isolated)
- Need to test backward compatibility
- Potential breaking changes (if any)

**Mitigation:**
- Test thoroughly before deployment
- Keep A2A server unchanged
- Add deprecation warnings for old clients
- Rollback plan in place

### Option 3: A2A Only
**Risk Level:** 🟡 Medium
- Similar to REST API
- Protocol-specific (less flexible)

---

## Recommended Approach: **Option 2 — REST API**

### Rationale

1. **Mission Alignment:** Perfect fit for CortexBrain's mission
2. **Ecosystem Integration:** Enables integration with GoMenu, CortexAvatar, dnet, Salamander
3. **Standardized:** REST API is widely understood, any language can connect
4. **Self-contained:** CortexBrain manages its own API
5. **Future-proof:** Conventional architecture, easy to scale
6. **Authentication:** Easy to add access control
7. **Distributed:** Multiple clients can access memory
8. **A2A Compatible:** Can still use A2A internally for agent communication

### Implementation Priority

**High Priority (P1):**
- Memory search endpoint
- Memory stats endpoint
- Update CortexHub CLI

**Medium Priority (P2):**
- Memory store endpoint
- Memory recent endpoint
- Authentication/authorization
- Rate limiting

**Low Priority (P3):**
- Semantic search (using nomic-embed-text)
- Time-travel (memory timeline)
- Memory consolidation endpoints
- Export/import endpoints

---

## Conclusion

**Based on CortexBrain's mission to be an ecosystem-integrated, self-contained cognitive system with 20 cognitive lobes, the REST API approach (Option 2) is the most appropriate choice.**

**Why REST API over A2A only:**
- ✅ Ecosystem integration (GoMenu, CortexAvatar, dnet, Salamander)
- ✅ Standardized protocol (any language can connect)
- ✅ Multiple clients (not agent-only)
- ✅ Authentication (access control)
- ✅ Future-proof (conventional architecture)

**Why not direct file operations (Option 1):**
- ❌ Not ecosystem-integrated
- ❌ No distributed access
- ❌ No authentication
- ❌ Not future-proof

---

## Next Steps

**Awaiting Norman's Decision:**

1. **Option 1 (No Change):** Accept current setup, continue using direct file operations
2. **Option 2 (REST API):** Implement REST API endpoints (2-3 days effort)
3. **Option 3 (A2A Only):** Implement A2A endpoints only (2-3 days effort)

**Please respond with your choice:**
- **"REST API"** — Implement REST API endpoints
- **"A2A Only"** — Implement A2A endpoints only
- **"Direct File"** — Keep current setup (no API)

**Expected Impact:**
- **REST API:** ✅ Full ecosystem integration, multiple clients, authentication
- **A2A Only:** ⚠️ Agent-only, harder integration, protocol-specific
- **Direct File:** ❌ Limited access, no integration, not scalable

---

**Document Version:** 1.0
**Last Updated:** February 5, 2026
**Status:** Awaiting Norman's Decision
