---
project: Cortex
component: Brain Kernel
phase: Ideation
date_created: 2026-02-02T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:29.828380
---

# CortexBrain Memory Enhancement — Complete Design Document
**Version:** 1.0 | **Date:** 2026-02-02 | **Author:** Albert (Architect) + Norman King (Vision)
**Status:** APPROVED FOR DEVELOPMENT
**Location:** `cortex-brain/docs/MEMORY-ENHANCEMENT-PRD.md`

---

## Table of Contents
1. [Executive Summary](#1-executive-summary)
2. [Current State Analysis](#2-current-state-analysis)
3. [Product Requirements](#3-product-requirements)
4. [Architecture Design](#4-architecture-design)
5. [Functional Specification](#5-functional-specification)
6. [API Specification](#6-api-specification)
7. [Data Model Changes](#7-data-model-changes)
8. [Change Requests](#8-change-requests)
9. [Migration Guide for Downstream Projects](#9-migration-guide)
10. [Test Plan](#10-test-plan)
11. [Implementation Roadmap](#11-implementation-roadmap)
12. [Appendix](#12-appendix)

---

## 1. Executive Summary

### Problem
CortexBrain's memory system is a flat SQLite store with keyword-based text search. It has no semantic understanding, no separation between working and long-term memory, no embedding support, and no portable archival format. Every project using CortexBrain (cortex-gateway, Harold, Albert's sync scripts) directly queries the same SQLite tables with basic SQL LIKE searches.

### Solution
Implement a **dual-layer memory architecture** inspired by neuroscience:
- **Working Memory (Frontal Lobe)** — Fast, ephemeral MemCell store for active context (SQLite, <1ms)
- **Episodic Memory (Temporal Lobe)** — Long-term archival with semantic search (Memvid .mv2 files)
- **Embedding Layer** — Lightweight local embedding model for semantic recall
- **Unified Memory API** — Single interface that queries both layers and ranks results

### Key Principles
1. **Modular** — CortexBrain is enhanced independently; downstream projects receive upgrade instructions
2. **Backward Compatible** — Existing API endpoints continue to work (deprecated, not removed)
3. **Zero External Dependencies** — Embeddings run locally via Ollama or built-in model
4. **Portable** — Each agent's episodic memory is a single .mv2 file, transferable anywhere
5. **Enterprise-Ready** — The memory architecture works for a single user or a fleet of agents

---

## 2. Current State Analysis

### CortexBrain Architecture (v0.2.0)
- **Binary:** Single Go executable (16 MB, compiled with SQLite + auth + memory API + chat routing)
- **Source:** `/home/normanking/clawd/cortex-brain/cortex-brain.go` (190 lines, chat routing only)
- **Compiled Source:** Includes proxy layer with full memory API (StoreMemory, SearchMemories, RecallMemories, StoreKnowledge, etc.)
- **Database:** SQLite at `cortex-brain.db` (1.4 MB)
- **Port:** 18892 (systemd managed)
- **Auth:** JWT with secret `CORTEX_JWT_SECRET`

### Current Database Schema
```sql
-- 549 episodic memories
memories (
    id TEXT PRIMARY KEY,
    owner_id TEXT NOT NULL,           -- user ID or "system"
    type TEXT DEFAULT 'episodic',     -- episodic, semantic, procedural, system
    content TEXT NOT NULL,
    metadata TEXT,                     -- JSON
    importance REAL DEFAULT 0.5,       -- 0.0-1.0
    access_count INTEGER DEFAULT 0,
    created_at DATETIME,
    updated_at DATETIME,
    last_accessed_at DATETIME
)

-- 52 knowledge entries
knowledge (
    id TEXT PRIMARY KEY,
    owner_id TEXT NOT NULL,
    category TEXT NOT NULL,            -- fact, preference, solution, pattern
    content TEXT NOT NULL,
    metadata TEXT,
    confidence REAL DEFAULT 0.8,
    source TEXT DEFAULT 'conversation',
    status TEXT DEFAULT 'active',      -- active, superseded, archived
    created_at DATETIME,
    updated_at DATETIME
)

-- Conversation log
conversations (
    id TEXT PRIMARY KEY,
    owner_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,                -- user, assistant, system
    content TEXT NOT NULL,
    metadata TEXT,
    created_at DATETIME
)

-- Users + sessions + personas (auth layer)
users, user_sessions, user_personas
```

### Current Search Capability
- **SQL LIKE** — `WHERE content LIKE '%keyword%'`
- **No embeddings** — Zero vector/semantic search
- **No ranking** — Results ordered by created_at, not relevance
- **Single store** — All memory types in one table, one access pattern

### Downstream Projects Using CortexBrain Memory API
| Project | How It Uses Memory | Files to Update |
|---------|-------------------|-----------------|
| **cortex-gateway** | `internal/brain/client.go` — RecallMemory, StoreMemory | Update client to use v2 API |
| **Albert (OpenClaw)** | `scripts/ch` — Python CortexHub CLI | Add v2 search methods |
| **Harold** | Bridge + memory sync | Update sync endpoint |
| **cortex-gateway sync** | `scripts/cortexhub-sync.py` | Use new bulk export/import |

---

## 3. Product Requirements

### 3.1 Functional Requirements

| ID | Requirement | Priority |
|----|------------|----------|
| FR-01 | **Dual-layer memory:** Working Memory (fast, ephemeral) + Episodic Memory (persistent, searchable) | P0 |
| FR-02 | **Semantic search:** Query memories by meaning, not just keywords | P0 |
| FR-03 | **Local embeddings:** Generate embeddings using local model (no cloud dependency) | P0 |
| FR-04 | **Per-agent memory files:** Each agent gets its own .mv2 episodic store | P0 |
| FR-05 | **Time-travel queries:** Search memories as they existed at a specific point in time | P1 |
| FR-06 | **Hybrid search:** Combine lexical (BM25) + semantic (vector) for best recall | P1 |
| FR-07 | **Memory consolidation:** Background process that moves working → episodic memory | P1 |
| FR-08 | **Portable export/import:** Export agent memory as single file, import on another instance | P1 |
| FR-09 | **Backward compatibility:** Existing /api/memories and /api/knowledge endpoints still work | P0 |
| FR-10 | **Memory decay:** Reduce importance of unaccessed memories over time (existing sleep cycle) | P2 |
| FR-11 | **Cross-agent search:** Query across multiple agents' episodic stores | P2 |
| FR-12 | **Conversation archival:** Auto-archive completed conversations to episodic memory | P1 |

### 3.2 Non-Functional Requirements

| ID | Requirement | Target |
|----|------------|--------|
| NF-01 | Working memory read latency | < 1ms |
| NF-02 | Semantic search latency (embeddings cached) | < 50ms |
| NF-03 | Semantic search latency (embedding generation) | < 200ms |
| NF-04 | Episodic memory capacity per agent | 1M+ entries |
| NF-05 | Embedding model size | < 300 MB |
| NF-06 | Memory file portability | Single file, any OS |
| NF-07 | Zero external service dependency | Everything runs local |
| NF-08 | Backward API compatibility | 100% for v1 endpoints |

---

## 4. Architecture Design

### 4.1 Dual-Layer Memory Model

```
┌──────────────────────────────────────────────────────────────────┐
│                        CORTEXBRAIN v0.3.0                        │
│                                                                   │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │                 UNIFIED MEMORY API (v2)                     │  │
│  │   POST /api/v2/memory/store                                │  │
│  │   POST /api/v2/memory/search    (hybrid: lex + semantic)   │  │
│  │   GET  /api/v2/memory/recall    (by query, ranked)         │  │
│  │   POST /api/v2/memory/consolidate (working → episodic)     │  │
│  │   GET  /api/v2/memory/export    (download .mv2)            │  │
│  │   POST /api/v2/memory/import    (upload .mv2)              │  │
│  └──────────────┬────────────────────┬────────────────────────┘  │
│                  │                    │                            │
│  ┌───────────────▼──────┐  ┌────────▼───────────────────────┐   │
│  │   WORKING MEMORY      │  │    EPISODIC MEMORY              │   │
│  │   (Frontal Lobe)      │  │    (Temporal Lobe)              │   │
│  │                       │  │                                  │   │
│  │  SQLite (existing)    │  │  Memvid .mv2 files              │   │
│  │  • memories table     │  │  • Per-agent: albert.mv2        │   │
│  │  • knowledge table    │  │  • Shared: shared.mv2           │   │
│  │  • conversations      │  │  • Append-only, crash-safe      │   │
│  │  • Fast R/W (<1ms)    │  │  • Time-travel queries          │   │
│  │  • Recent context     │  │  • Semantic + lexical search    │   │
│  │  • Active session     │  │  • Compressed, portable         │   │
│  │  • TTL-based expiry   │  │  • Unlimited retention          │   │
│  └───────────────────────┘  └──────────────────────────────────┘   │
│                  │                    │                            │
│  ┌───────────────▼────────────────────▼──────────────────────┐   │
│  │              EMBEDDING ENGINE                              │   │
│  │                                                            │   │
│  │  Primary: nomic-embed-text (Ollama, 274 MB, 768 dims)     │   │
│  │  Fallback: bge-small-en-v1.5 (built-in, 138 MB, 384 dims)│   │
│  │  Cache: SQLite table (embedding_cache)                     │   │
│  │                                                            │   │
│  │  • Generate on write (async, non-blocking)                 │   │
│  │  • Cache embeddings in SQLite for working memory           │   │
│  │  • Memvid manages its own vectors for episodic             │   │
│  └────────────────────────────────────────────────────────────┘   │
│                                                                   │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │              CONSOLIDATION ENGINE                           │  │
│  │                                                            │  │
│  │  Background goroutine (configurable interval)              │  │
│  │  • Scans working memory for entries older than TTL         │  │
│  │  • Archives to episodic (.mv2) with full metadata          │  │
│  │  • Updates importance scores based on access patterns      │  │
│  │  • Deduplicates near-identical entries (cosine similarity) │  │
│  │  • Runs during "sleep cycle" (existing 3 AM cron)          │  │
│  └────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
```

### 4.2 Data Flow

```
User Message → CortexBrain
    │
    ├─→ Store in Working Memory (SQLite, immediate)
    │   └─→ Generate embedding (async) → embedding_cache table
    │
    ├─→ Search Working Memory (fast, recent context)
    │   └─→ Vector similarity on cached embeddings
    │
    ├─→ Search Episodic Memory (deep, historical)
    │   └─→ Memvid hybrid search (BM25 + vector)
    │
    ├─→ Rank & merge results from both layers
    │   └─→ Return top-K by combined relevance score
    │
    └─→ Consolidation (background)
        └─→ Working Memory entries older than TTL → Episodic .mv2
```

### 4.3 Embedding Strategy

| Scenario | Model | How |
|----------|-------|-----|
| Ollama available | `nomic-embed-text` | HTTP call to Ollama /api/embeddings |
| Ollama unavailable | `bge-small-en-v1.5` | Memvid's built-in (auto-download) |
| No GPU, no network | Lexical search only | BM25 fallback (always works) |

Embeddings are generated **asynchronously** on write — the store operation returns immediately, and a background worker generates and caches the embedding. Search uses cached embeddings when available, falls back to lexical when not.

### 4.4 File Layout

```
/home/normanking/clawd/cortex-brain/
├── cortex-brain              # Binary (enhanced v0.3.0)
├── cortex-brain.db           # Working memory (SQLite)
├── config.yaml               # NEW: configuration file
├── data/
│   └── episodic/
│       ├── albert.mv2        # Albert's episodic memory
│       ├── harold.mv2        # Harold's episodic memory
│       ├── shared.mv2        # Cross-agent knowledge
│       └── system.mv2        # System-level memories
└── models/
    └── bge-small-en-v1.5/    # Fallback embedding model (if no Ollama)
```

---

## 5. Functional Specification

### 5.1 Working Memory (Frontal Lobe)

**Purpose:** Fast access to recent, active, frequently-changing state.

**Behavior:**
- All new memories land here first (existing behavior preserved)
- Entries have a configurable TTL (default: 7 days)
- Entries accessed frequently have TTL extended automatically
- Pinned entries (importance ≥ 0.9) never expire from working memory
- Search returns results ranked by recency + importance + access_count

**New SQLite Tables:**
```sql
-- Embedding cache for working memory semantic search
CREATE TABLE embedding_cache (
    memory_id TEXT PRIMARY KEY,
    embedding BLOB NOT NULL,        -- float32 array, serialized
    model TEXT NOT NULL,             -- "nomic-embed-text" or "bge-small"
    dimensions INTEGER NOT NULL,
    created_at DATETIME NOT NULL,
    FOREIGN KEY (memory_id) REFERENCES memories(id) ON DELETE CASCADE
);

-- Working memory configuration
CREATE TABLE memory_config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at DATETIME NOT NULL
);
```

### 5.2 Episodic Memory (Temporal Lobe)

**Purpose:** Permanent archival of all agent interactions, searchable by meaning.

**Behavior:**
- Receives entries from consolidation (working → episodic)
- Also supports direct writes (bulk import, conversation archival)
- Each agent has its own .mv2 file
- Shared knowledge goes to shared.mv2
- Supports time-travel: "what did I know on January 15th?"
- Memvid handles its own embedding index (bge-small built-in)

**Integration:**
- CortexBrain wraps Memvid CLI or SDK for .mv2 operations
- On Linux (Pink): Use Python SDK (`memvid` 0.1.3, already installed)
- On macOS: Use CLI with lexical mode, or route to Pink
- Future: Go FFI bindings to memvid-core (Rust)

### 5.3 Memory Consolidation

**Purpose:** Automated migration of working memory to episodic archive.

**Process (runs on configurable schedule, default: every 6 hours + nightly sleep cycle):**

1. **Select candidates:** Working memory entries where:
   - `created_at` > TTL (7 days default)
   - `importance` < 0.9 (pinned entries stay)
   - `type` = 'episodic' (knowledge stays in working memory)

2. **Deduplicate:** Compare embedding similarity of candidates against recent episodic entries. Skip if cosine similarity > 0.95 (near-duplicate).

3. **Archive:** Write each candidate to appropriate .mv2 file:
   - Agent-specific entries → `{agent_name}.mv2`
   - System entries → `system.mv2`
   - Cross-agent knowledge → `shared.mv2`

4. **Clean up:** Delete archived entries from working memory SQLite.

5. **Log:** Record consolidation metrics (entries processed, archived, deduped, skipped).

### 5.4 Search Pipeline

When a search query arrives:

```
Query: "How did we fix Harold's SSH issue?"
    │
    ├─1─→ Generate query embedding (nomic-embed-text via Ollama)
    │
    ├─2─→ Search Working Memory
    │     ├── Lexical: SQL LIKE '%Harold%SSH%'
    │     └── Semantic: cosine_similarity(query_emb, cached_emb) > 0.7
    │     → Results: [{content, score, source: "working"}]
    │
    ├─3─→ Search Episodic Memory
    │     └── memvid find --query "Harold SSH fix" --mode auto --top-k 5
    │     → Results: [{content, score, source: "episodic"}]
    │
    ├─4─→ Merge & Rank
    │     ├── Combine results from both layers
    │     ├── Normalize scores to 0.0-1.0
    │     ├── Apply recency boost (working > episodic for recent entries)
    │     ├── Apply importance weighting
    │     └── Deduplicate (cosine similarity > 0.95 → keep highest score)
    │
    └─5─→ Return top-K results with provenance
          [{content, score, source, timestamp, agent, type}]
```

---

## 6. API Specification

### 6.1 New v2 Endpoints

All new endpoints under `/api/v2/memory/`. Existing v1 endpoints (`/api/memories`, `/api/knowledge`) continue to work unchanged.

#### POST /api/v2/memory/store
Store a memory (automatically routed to working memory).
```json
// Request
{
    "content": "Harold's SSH was fixed by updating to .128 IP",
    "type": "episodic",          // episodic|semantic|procedural|system
    "agent": "albert",            // which agent this belongs to
    "importance": 0.7,
    "metadata": {"tags": ["harold", "ssh", "fix"]},
    "pin": false                   // if true, never auto-consolidate
}

// Response
{
    "id": "uuid",
    "layer": "working",
    "embedding_status": "queued",  // queued|ready|failed
    "created_at": "2026-02-02T19:30:00Z"
}
```

#### POST /api/v2/memory/search
Hybrid search across both memory layers.
```json
// Request
{
    "query": "How did we fix Harold?",
    "mode": "hybrid",             // lexical|semantic|hybrid
    "layers": ["working", "episodic"],  // which layers to search
    "agent": "albert",            // filter to agent (optional)
    "top_k": 10,
    "min_score": 0.5,
    "time_range": {               // optional
        "after": "2026-01-01T00:00:00Z",
        "before": "2026-02-03T00:00:00Z"
    }
}

// Response
{
    "query": "How did we fix Harold?",
    "mode": "hybrid",
    "results": [
        {
            "id": "uuid",
            "content": "Harold SSH restored at .128...",
            "score": 0.92,
            "source": "working",      // working|episodic
            "agent": "albert",
            "type": "episodic",
            "importance": 0.8,
            "created_at": "2026-02-02T16:01:00Z",
            "metadata": {"tags": ["harold"]}
        }
    ],
    "search_ms": 45,
    "working_count": 3,
    "episodic_count": 7
}
```

#### POST /api/v2/memory/consolidate
Trigger manual consolidation.
```json
// Request
{
    "agent": "albert",            // optional, all agents if omitted
    "dry_run": true,              // preview what would be archived
    "ttl_override": "24h"         // optional, override default TTL
}

// Response
{
    "candidates": 45,
    "archived": 38,
    "deduplicated": 5,
    "skipped_pinned": 2,
    "duration_ms": 1200
}
```

#### GET /api/v2/memory/export/{agent}
Export agent's episodic memory as downloadable .mv2 file.
```
GET /api/v2/memory/export/albert
→ Content-Type: application/octet-stream
→ Content-Disposition: attachment; filename="albert.mv2"
```

#### POST /api/v2/memory/import
Import episodic memory from .mv2 file.
```
POST /api/v2/memory/import
Content-Type: multipart/form-data
agent=albert
file=@albert.mv2

// Response
{
    "agent": "albert",
    "frames_imported": 549,
    "file_size": "2.4 MB",
    "status": "success"
}
```

#### GET /api/v2/memory/stats
Memory system statistics.
```json
// Response
{
    "working": {
        "total_memories": 549,
        "total_knowledge": 52,
        "total_conversations": 1200,
        "embeddings_cached": 480,
        "db_size_bytes": 1486848
    },
    "episodic": {
        "agents": {
            "albert": {"frames": 0, "file_size": 0},
            "harold": {"frames": 0, "file_size": 0}
        },
        "shared": {"frames": 0, "file_size": 0}
    },
    "embedding": {
        "model": "nomic-embed-text",
        "dimensions": 768,
        "source": "ollama",
        "status": "available"
    }
}
```

#### GET /api/v2/memory/timeline/{agent}
View chronological memory timeline (time-travel).
```json
// Request
GET /api/v2/memory/timeline/albert?from=2026-01-01&to=2026-02-01&limit=50

// Response
{
    "agent": "albert",
    "entries": [
        {"timestamp": "...", "content": "...", "type": "...", "source": "episodic"}
    ]
}
```

### 6.2 Backward Compatible v1 Endpoints

These continue to work **unchanged**. They map to working memory only.

| Existing Endpoint | Behavior | v2 Equivalent |
|-------------------|----------|---------------|
| GET /api/memories | List working memories | GET /api/v2/memory/search (working layer only) |
| POST /api/memories | Store to working memory | POST /api/v2/memory/store |
| GET /api/memories/search?q= | Keyword search (LIKE) | POST /api/v2/memory/search (mode=lexical) |
| GET /api/knowledge | List knowledge | Same (knowledge stays in working memory) |
| POST /api/knowledge | Store knowledge | Same |

**Deprecation:** v1 endpoints will log a deprecation warning. No removal timeline — they work forever, just not enhanced.

---

## 7. Data Model Changes

### 7.1 New SQLite Tables

```sql
-- Embedding cache
CREATE TABLE IF NOT EXISTS embedding_cache (
    memory_id TEXT PRIMARY KEY,
    embedding BLOB NOT NULL,
    model TEXT NOT NULL DEFAULT 'nomic-embed-text',
    dimensions INTEGER NOT NULL DEFAULT 768,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (memory_id) REFERENCES memories(id) ON DELETE CASCADE
);
CREATE INDEX idx_embedding_model ON embedding_cache(model);

-- Consolidation log
CREATE TABLE IF NOT EXISTS consolidation_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent TEXT NOT NULL,
    memories_processed INTEGER,
    memories_archived INTEGER,
    memories_deduped INTEGER,
    memories_skipped INTEGER,
    duration_ms INTEGER,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Memory configuration
CREATE TABLE IF NOT EXISTS memory_config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### 7.2 Schema Modifications to Existing Tables

```sql
-- Add agent field to memories (currently uses owner_id which maps to user UUID)
ALTER TABLE memories ADD COLUMN agent TEXT DEFAULT '';

-- Add TTL expiry field
ALTER TABLE memories ADD COLUMN expires_at DATETIME;

-- Add pinned flag
ALTER TABLE memories ADD COLUMN pinned INTEGER DEFAULT 0;

-- Add embedding status
ALTER TABLE memories ADD COLUMN embedding_status TEXT DEFAULT 'pending';
-- Values: pending, queued, ready, failed

-- Index for consolidation queries
CREATE INDEX IF NOT EXISTS idx_memories_expires ON memories(expires_at);
CREATE INDEX IF NOT EXISTS idx_memories_pinned ON memories(pinned);
CREATE INDEX IF NOT EXISTS idx_memories_embedding ON memories(embedding_status);
```

### 7.3 Memvid File Structure
```
data/episodic/
├── albert.mv2       # Created on first write for agent "albert"
├── harold.mv2       # Created on first write for agent "harold"
├── shared.mv2       # Cross-agent knowledge archive
└── system.mv2       # System-level memories
```

Each .mv2 file is self-contained with:
- Frame data (memory content + metadata as JSON)
- Lexical index (BM25, always available)
- Vector index (bge-small embeddings, built on first semantic query)
- Time index (chronological ordering)
- WAL (crash safety)

---

## 8. Change Requests

### CR-001: Add Embedding Engine to CortexBrain
**Priority:** P0 | **Effort:** 3 hours
- Add Go code to call Ollama `/api/embeddings` endpoint
- Add fallback to direct bge-small loading (via memvid SDK)
- Add embedding_cache SQLite table
- Background worker goroutine for async embedding generation
- Config: `embedding.model`, `embedding.ollama_url`, `embedding.fallback_model`

### CR-002: Add Memvid Integration Layer
**Priority:** P0 | **Effort:** 4 hours
- Go wrapper for Memvid operations (create, put, find, stats, timeline)
- Initially via CLI exec (memvid binary) or Python SDK subprocess
- Future: Go FFI to memvid-core Rust library
- Auto-create .mv2 files per agent on first write
- Config: `episodic.enabled`, `episodic.data_dir`, `episodic.embedding_model`

### CR-003: Implement v2 Memory API
**Priority:** P0 | **Effort:** 4 hours
- Add all v2 endpoints to HTTP handler
- Implement hybrid search (working + episodic, merge + rank)
- Auth: Same JWT as existing v1
- v1 endpoints remain unchanged (backward compat)

### CR-004: Implement Consolidation Engine
**Priority:** P1 | **Effort:** 3 hours
- Background goroutine with configurable interval
- TTL-based selection from working memory
- Deduplication via cosine similarity
- Logging to consolidation_log table
- Integration with existing sleep cycle (3 AM)

### CR-005: Schema Migration
**Priority:** P0 | **Effort:** 1 hour
- Auto-migrate on startup (check schema version)
- Add new columns to existing tables (non-breaking ALTER TABLE)
- Create new tables (embedding_cache, consolidation_log, memory_config)
- Backfill `agent` field from owner_id → username mapping

### CR-006: Configuration File
**Priority:** P1 | **Effort:** 1 hour
- Add `config.yaml` support to CortexBrain
- Sections: server, memory, embedding, episodic, consolidation
- Environment variable overrides (existing behavior preserved)
- Default config generated on first run

### CR-007: Export/Import Endpoints
**Priority:** P1 | **Effort:** 2 hours
- Stream .mv2 files via HTTP for export
- Accept multipart upload for import
- Validate .mv2 integrity before import
- Support bulk working memory → episodic migration via import

---

## 9. Migration Guide for Downstream Projects

### 9.1 cortex-gateway

**File:** `internal/brain/client.go`

**Changes:**
```go
// OLD
func (c *Client) RecallMemory(req *RecallMemoryRequest) (*RecallMemoryResponse, error) {
    // calls GET /api/memories/search?q=...
}

// NEW (add alongside, don't replace)
func (c *Client) SearchMemoryV2(req *SearchMemoryV2Request) (*SearchMemoryV2Response, error) {
    // calls POST /api/v2/memory/search
}

type SearchMemoryV2Request struct {
    Query     string   `json:"query"`
    Mode      string   `json:"mode"`      // "hybrid"|"lexical"|"semantic"
    Layers    []string `json:"layers"`    // ["working", "episodic"]
    Agent     string   `json:"agent"`
    TopK      int      `json:"top_k"`
    MinScore  float64  `json:"min_score"`
}
```

**Migration steps:**
1. Add `SearchMemoryV2` method to brain client
2. Update agent loop to prefer v2 search when available
3. Fallback to v1 if v2 returns 404 (brain not upgraded yet)
4. No breaking changes — old methods continue to work

### 9.2 Albert's CortexHub CLI (`scripts/ch`)

**File:** `scripts/ch` (Python)

**Changes:**
```python
# OLD
def recall(query):
    r = requests.get(f"{BASE}/api/memories/search?q={query}", auth=AUTH)

# NEW (add subcommand)
def search(query, mode="hybrid", layers=None):
    r = requests.post(f"{BASE}/api/v2/memory/search", json={
        "query": query,
        "mode": mode,
        "layers": layers or ["working", "episodic"],
    }, auth=AUTH)
```

**New subcommands:**
- `ch search <query>` — Hybrid search (both layers)
- `ch search --semantic <query>` — Semantic only
- `ch search --episodic <query>` — Episodic only
- `ch consolidate` — Trigger manual consolidation
- `ch export <agent>` — Download .mv2 file
- `ch stats` — Memory system stats (both layers)

### 9.3 Harold (Gateway + Sync)

**Changes:**
- Update bridge-based memory sync to use v2 bulk API
- Harold's own memories go to `harold.mv2`
- Sync script uses `/api/v2/memory/export` for backup

### 9.4 Version Detection Pattern

All downstream projects should detect CortexBrain version before calling v2:

```go
// Check if v2 is available
resp, err := http.Get(brainURL + "/health")
// Parse version from response
// If version >= "0.3.0", use v2 endpoints
// Otherwise, fall back to v1
```

This keeps the system **modular** — each project works with whatever version of CortexBrain is deployed.

---

## 10. Test Plan

### 10.1 Unit Tests

| Test ID | Module | Test | Pass Criteria |
|---------|--------|------|---------------|
| UT-01 | embedding | TestGenerateEmbedding_Ollama | Returns 768-dim float32 array |
| UT-02 | embedding | TestGenerateEmbedding_Fallback | Works without Ollama |
| UT-03 | embedding | TestCacheEmbedding | Stores/retrieves from SQLite |
| UT-04 | embedding | TestCosineSimilarity | Correct similarity for known vectors |
| UT-05 | memcell | TestStoreWorking | Memory stored in SQLite |
| UT-06 | memcell | TestSearchLexical | SQL LIKE search works |
| UT-07 | memcell | TestSearchSemantic | Vector search against cached embeddings |
| UT-08 | episodic | TestCreateMv2 | .mv2 file created for agent |
| UT-09 | episodic | TestStoreMv2 | Frame appended to .mv2 |
| UT-10 | episodic | TestSearchMv2_Lexical | BM25 search returns results |
| UT-11 | episodic | TestSearchMv2_Semantic | Vector search returns results |
| UT-12 | episodic | TestTimeline | Chronological frame listing |
| UT-13 | consolidation | TestSelectCandidates | Correct TTL-based selection |
| UT-14 | consolidation | TestDeduplication | Near-duplicates detected |
| UT-15 | consolidation | TestArchive | Entries moved to .mv2 |
| UT-16 | consolidation | TestPinnedSkipped | Pinned entries not archived |
| UT-17 | api | TestV2Store | POST /api/v2/memory/store works |
| UT-18 | api | TestV2Search_Hybrid | Hybrid search merges layers |
| UT-19 | api | TestV2Search_WorkingOnly | Layer filter works |
| UT-20 | api | TestV1Backward | Old endpoints still work |

### 10.2 Integration Tests

| Test ID | Scenario | Pass Criteria |
|---------|----------|---------------|
| IT-01 | Store → Search cycle | Store memory, search returns it (both lexical and semantic) |
| IT-02 | Consolidation cycle | Working memory → episodic .mv2, verify searchable in both |
| IT-03 | Export → Import | Export agent.mv2, import on fresh instance, search works |
| IT-04 | Cross-layer search | Query returns results from both working and episodic |
| IT-05 | Time-travel | Store entries at different times, query as-of past timestamp |
| IT-06 | Version fallback | cortex-gateway detects v1 brain, uses v1 API correctly |
| IT-07 | Ollama down | Embedding falls back to lexical, no errors |
| IT-08 | Concurrent access | Multiple agents writing/reading simultaneously |

### 10.3 Performance Tests

| Test ID | Metric | Target |
|---------|--------|--------|
| PT-01 | Working memory store | < 1ms |
| PT-02 | Working memory lexical search | < 5ms |
| PT-03 | Working memory semantic search (cached) | < 50ms |
| PT-04 | Embedding generation (nomic) | < 100ms per entry |
| PT-05 | Episodic search (1K entries) | < 200ms |
| PT-06 | Episodic search (100K entries) | < 500ms |
| PT-07 | Consolidation (500 entries) | < 5s |
| PT-08 | Export 10K entries to .mv2 | < 30s |

---

## 11. Implementation Roadmap

### Phase 1: Foundation (Day 1)
- [ ] CR-005: Schema migration (new tables + columns)
- [ ] CR-001: Embedding engine (Ollama + fallback)
- [ ] CR-006: Configuration file support
- [ ] Tests: UT-01 through UT-07

### Phase 2: Episodic Layer (Day 2)
- [ ] CR-002: Memvid integration (create, put, find, stats)
- [ ] Install memvid properly on Pink (Docker or cargo build from source)
- [ ] Pre-download bge-small model
- [ ] Tests: UT-08 through UT-12

### Phase 3: API + Consolidation (Day 3)
- [ ] CR-003: v2 API endpoints
- [ ] CR-004: Consolidation engine
- [ ] CR-007: Export/import
- [ ] Tests: UT-13 through UT-20

### Phase 4: Integration + Migration (Day 4)
- [ ] Integration tests IT-01 through IT-08
- [ ] Performance tests PT-01 through PT-08
- [ ] Update cortex-gateway brain client
- [ ] Update ch CLI script
- [ ] Update sync scripts

### Phase 5: Rollout (Day 5)
- [ ] Build new cortex-brain binary (v0.3.0)
- [ ] Deploy to Pink (systemd restart)
- [ ] Verify v1 backward compatibility
- [ ] Bulk migrate existing 549 memories to episodic
- [ ] Notify downstream projects (cortex-gateway, Harold)
- [ ] Update MEMORY.md and HEARTBEAT.md

---

## 12. Appendix

### A. Embedding Model Comparison

| Model | Dims | Size | MTEB Score | Speed (tok/s) | Notes |
|-------|------|------|------------|---------------|-------|
| nomic-embed-text-v1.5 | 768 | 274 MB | 62.28 | ~3000 | In our Ollama |
| bge-small-en-v1.5 | 384 | 138 MB | 62.17 | ~5000 | Memvid default |
| all-MiniLM-L6-v2 | 384 | 91 MB | 56.26 | ~7000 | Smallest useful |
| snowflake-arctic-embed-xs | 384 | 91 MB | 55.98 | ~7000 | New, fast |
| gte-large | 1024 | 1.3 GB | 63.13 | ~1500 | Best quality |

**Decision:** Use `nomic-embed-text` (already deployed) as primary, `bge-small` as fallback.

### B. Memvid vs. Alternatives

| Feature | Memvid | sqlite-vec | pgvector | Qdrant | ChromaDB |
|---------|--------|-----------|----------|--------|----------|
| Single file | ✅ | ✅ | ❌ | ❌ | ❌ |
| Time-travel | ✅ | ❌ | ❌ | ❌ | ❌ |
| Crash-safe WAL | ✅ | ✅ | ✅ | ✅ | ❌ |
| Go SDK | ❌ | ✅ | ✅ | ✅ | ❌ |
| No server needed | ✅ | ✅ | ❌ | ❌ | ❌ |
| Portable | ✅✅ | ✅ | ❌ | ❌ | ❌ |
| Maturity | Low | Medium | High | High | Medium |

**Decision:** Memvid for episodic (portability + time-travel), SQLite for working (speed + Go native).

### C. Configuration Schema

```yaml
# cortex-brain config.yaml
server:
  port: 18892
  host: "0.0.0.0"
  jwt_secret: "${CORTEX_JWT_SECRET}"

memory:
  working:
    ttl: "7d"                    # Default TTL for working memory
    max_entries: 10000           # Max entries before forced consolidation
  
  episodic:
    enabled: true
    data_dir: "./data/episodic"
    embedding_model: "bge-small" # Model for .mv2 vector index
  
  embedding:
    primary: "ollama"
    ollama_url: "http://localhost:11434"
    ollama_model: "nomic-embed-text"
    fallback: "bge-small"
    cache_enabled: true
    async: true                  # Generate embeddings asynchronously
  
  consolidation:
    enabled: true
    interval: "6h"               # Check every 6 hours
    sleep_cycle: "03:00"         # Nightly deep consolidation
    dedup_threshold: 0.95        # Cosine similarity threshold
    batch_size: 100

inference:
  ollama_url: "http://localhost:11434"
  default_model: "deepseek-coder-v2:latest"
  xai_endpoint: "https://api.x.ai/v1"
  xai_api_key: "${XAI_API_KEY}"

bridge:
  url: "http://192.168.1.128:18802"
  agent_name: "cortex-brain"
```

---

**Document Status:** Complete and ready for implementation.
**Next Action:** Add to cortex-brain project and begin Phase 1.
