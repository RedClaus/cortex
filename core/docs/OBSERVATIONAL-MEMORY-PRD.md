---
project: Cortex
component: Memory
phase: Design
date_created: 2026-02-10T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-11T01:40:42.644879
---

# Observational Memory PRD

**Project:** CortexBrain
**Feature:** Observational Memory System
**Priority:** P2 (After vLLM Integration)
**Status:** Approved for Development
**Author:** Codie
**Date:** 2026-02-10
**Updated:** 2026-02-11 (Memvid storage backend)

---

## Executive Summary

Implement a three-tier observational memory system for CortexBrain that uses background agents to compress conversation history into dense observations and reflections. This capability will enable long-running agent conversations without context degradation, reduce inference costs by 10x, and provide cross-session memory for swarm agents (Harold, Albert, Codie).

**Decision:** EMBEDDED (not plugin) - memory consolidation is core cognition, like the biological hippocampus.

**Storage Backend:** Memvid .mv2 files (per the Memory Enhancement PRD) - portable, single-file storage with time-travel queries and built-in semantic search.

---

## Problem Statement

### Current Pain Points

| Problem | Impact | Affected |
|---------|--------|----------|
| **Context Rot** | Important details drowned by newer messages | All agents |
| **Token Waste** | Same history resent repeatedly, burning costs | Multi-agent swarm |
| **Context Limits** | Conversations hit limits and lose coherence | Long tasks |
| **No Cross-Session Memory** | Agents forget between sessions | Harold, Albert, Codie |
| **RAG Limitations** | Retrieval adds latency, misses context | Knowledge queries |

### Business Case

- **10x cost reduction** via prompt caching (stable context window)
- **94.87% accuracy** on LongMemEval benchmarks (vs 80% RAG)
- **5-40x compression** of conversation history
- **Infinite conversation length** with preserved context

---

## Goals & Success Metrics

### Primary Goals

1. Enable swarm agents to maintain memory across sessions
2. Reduce inference costs for multi-agent workloads by 10x
3. Preserve critical context in long-running conversations
4. Outperform RAG on long-context recall benchmarks

### Success Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| Cost Reduction | ≥ 5x | Token usage before/after |
| LongMemEval Score | ≥ 85% | Benchmark suite |
| Compression Ratio | 5-40x | Tokens in vs tokens out |
| Context Stability | 95% cache hit | Prompt cache metrics |
| Cross-Session Recall | ≥ 90% | Manual evaluation |

---

## Technical Architecture

### Three-Tier Memory Hierarchy

```
┌─────────────────────────────────────────────────────────────────────────┐
│                     Observational Memory System                          │
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │                    Tier 1: Recent Messages                       │    │
│  │  • Raw conversation history                                      │    │
│  │  • Full fidelity, no compression                                 │    │
│  │  • Threshold: 30,000 tokens (configurable)                       │    │
│  └──────────────────────────┬──────────────────────────────────────┘    │
│                             │                                            │
│                             ▼  @ threshold                               │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │                     Observer Agent                               │    │
│  │  • Background goroutine                                          │    │
│  │  • Model: Gemini 2.5 Flash (1M context)                          │    │
│  │  • Compresses messages → timestamped observations                │    │
│  │  • Preserves: task state, priority events, decisions             │    │
│  │  • Target: 5-40x compression                                     │    │
│  └──────────────────────────┬──────────────────────────────────────┘    │
│                             │                                            │
│                             ▼                                            │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │                    Tier 2: Observations                          │    │
│  │  • Compressed notes from Observer                                │    │
│  │  • Timestamped with priority levels                              │    │
│  │  • Threshold: 40,000 tokens (configurable)                       │    │
│  └──────────────────────────┬──────────────────────────────────────┘    │
│                             │                                            │
│                             ▼  @ threshold                               │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │                    Reflector Agent                               │    │
│  │  • Background goroutine                                          │    │
│  │  • Model: Gemini 2.5 Flash                                       │    │
│  │  • Combines related observations                                 │    │
│  │  • Identifies patterns across time                               │    │
│  │  • Garbage collects irrelevant observations                      │    │
│  └──────────────────────────┬──────────────────────────────────────┘    │
│                             │                                            │
│                             ▼                                            │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │                    Tier 3: Reflections                           │    │
│  │  • High-level patterns and insights                              │    │
│  │  • Long-term memory (weeks/months)                               │    │
│  │  • Cross-session persistence                                     │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### Component Design

#### 1. ObservationalMemory Core

```go
// pkg/brain/memory/observational.go

package memory

import (
    "context"
    "sync"
    "time"
)

// ObservationalMemory manages the three-tier memory system.
type ObservationalMemory struct {
    // Storage tiers
    messages     *MessageStore      // Tier 1: Recent messages
    observations *ObservationStore  // Tier 2: Compressed observations
    reflections  *ReflectionStore   // Tier 3: High-level patterns

    // Background agents
    observer  *ObserverAgent
    reflector *ReflectorAgent

    // Configuration
    config Config

    // Concurrency
    mu sync.RWMutex
}

// Config configures the observational memory system.
type Config struct {
    // Token thresholds
    MessageThreshold     int `yaml:"message_threshold"`     // Default: 30000
    ObservationThreshold int `yaml:"observation_threshold"` // Default: 40000

    // Model configuration
    ObserverModel  string `yaml:"observer_model"`  // Default: gemini-2.5-flash
    ReflectorModel string `yaml:"reflector_model"` // Default: gemini-2.5-flash

    // Scope: "thread" (per-conversation) or "resource" (per-user)
    Scope string `yaml:"scope"` // Default: thread

    // Token budget sharing
    ShareTokenBudget bool `yaml:"share_token_budget"` // Default: false

    // Storage backend (per Memory Enhancement PRD)
    // Working Memory: SQLite (fast, ephemeral)
    // Episodic Memory: Memvid .mv2 files (long-term, portable, time-travel)
    WorkingStorage string `yaml:"working_storage"` // sqlite (default)
    EpisodicStorage string `yaml:"episodic_storage"` // memvid (default)

    // Memvid configuration
    MemvidDataDir   string `yaml:"memvid_data_dir"`   // Default: ./data/episodic
    MemvidEmbedding string `yaml:"memvid_embedding"`  // Default: bge-small-en-v1.5
}

// Message represents a conversation message.
type Message struct {
    ID        string    `json:"id"`
    Role      string    `json:"role"`      // user, assistant, system
    Content   string    `json:"content"`
    Timestamp time.Time `json:"timestamp"`
    ThreadID  string    `json:"thread_id"`
    ResourceID string   `json:"resource_id"` // User/agent identifier
    TokenCount int      `json:"token_count"`
}

// Observation represents a compressed memory unit.
type Observation struct {
    ID          string    `json:"id"`
    Content     string    `json:"content"`     // Compressed observation text
    Timestamp   time.Time `json:"timestamp"`
    Priority    int       `json:"priority"`    // 1-5, higher = more important
    TaskState   string    `json:"task_state"`  // Current task context
    SourceRange []string  `json:"source_range"` // Message IDs that were compressed
    ThreadID    string    `json:"thread_id"`
    ResourceID  string    `json:"resource_id"`
    TokenCount  int       `json:"token_count"`
}

// Reflection represents a high-level pattern or insight.
type Reflection struct {
    ID          string    `json:"id"`
    Content     string    `json:"content"`
    Timestamp   time.Time `json:"timestamp"`
    Pattern     string    `json:"pattern"`      // Type of pattern identified
    SourceObs   []string  `json:"source_obs"`   // Observation IDs consolidated
    ResourceID  string    `json:"resource_id"`
    TokenCount  int       `json:"token_count"`
}
```

#### 2. Observer Agent

```go
// pkg/brain/memory/observer.go

package memory

import (
    "context"
    "time"
)

// ObserverAgent monitors and compresses message history.
type ObserverAgent struct {
    model       LLMClient
    store       *MessageStore
    obsStore    *ObservationStore
    config      Config
    running     bool
    stopCh      chan struct{}
}

// ObserverPrompt is the system prompt for the Observer agent.
const ObserverPrompt = `You are a Memory Observer. Your job is to compress conversation history into dense observations.

When compressing messages:
1. Preserve critical information: decisions made, tasks completed, errors encountered
2. Note the current task state and any pending work
3. Include timestamps for temporal context
4. Assign priority (1-5) based on importance for future context
5. Remove redundant information and filler

Output format:
---
[TIMESTAMP] Priority: X
Task State: <current task description>
Observations:
- <key observation 1>
- <key observation 2>
Suggested Context: <what the agent should remember>
---

Compress the following messages into observations:`

// Start begins the background observation loop.
func (o *ObserverAgent) Start(ctx context.Context) {
    o.running = true
    go o.observeLoop(ctx)
}

// observeLoop continuously monitors message count.
func (o *ObserverAgent) observeLoop(ctx context.Context) {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-o.stopCh:
            return
        case <-ticker.C:
            o.checkAndCompress(ctx)
        }
    }
}

// checkAndCompress compresses messages if threshold exceeded.
func (o *ObserverAgent) checkAndCompress(ctx context.Context) error {
    tokens := o.store.TotalTokens()
    if tokens < o.config.MessageThreshold {
        return nil
    }

    // Get messages to compress (oldest first, keep recent)
    messages, err := o.store.GetOldestMessages(ctx, tokens - o.config.MessageThreshold/2)
    if err != nil {
        return err
    }

    // Call LLM to compress
    observation, err := o.compress(ctx, messages)
    if err != nil {
        return err
    }

    // Store observation and remove compressed messages
    if err := o.obsStore.Store(ctx, observation); err != nil {
        return err
    }

    return o.store.RemoveMessages(ctx, messages)
}
```

#### 3. Reflector Agent

```go
// pkg/brain/memory/reflector.go

package memory

// ReflectorAgent consolidates observations into reflections.
type ReflectorAgent struct {
    model    LLMClient
    obsStore *ObservationStore
    refStore *ReflectionStore
    config   Config
    running  bool
    stopCh   chan struct{}
}

// ReflectorPrompt is the system prompt for the Reflector agent.
const ReflectorPrompt = `You are a Memory Reflector. Your job is to consolidate observations into high-level patterns and insights.

When reflecting:
1. Identify recurring patterns across observations
2. Combine related observations into unified insights
3. Preserve the most important context for long-term memory
4. Garbage collect observations that are no longer relevant
5. Note any behavioral patterns or preferences

Output format:
---
Pattern: <pattern type>
Insight: <consolidated understanding>
Preserved Observations: <IDs to keep>
Garbage Collect: <IDs to remove>
---

Consolidate the following observations:`

// reflectLoop continuously monitors observation count.
func (r *ReflectorAgent) reflectLoop(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-r.stopCh:
            return
        case <-ticker.C:
            r.checkAndReflect(ctx)
        }
    }
}
```

#### 4. Storage Backend (Memvid)

Per the **Memory Enhancement PRD**, the storage architecture uses:
- **Working Memory (SQLite):** Fast, ephemeral storage for recent messages (<1ms latency)
- **Episodic Memory (Memvid .mv2):** Long-term archival with semantic + lexical search

**Why Memvid?**
| Feature | Benefit |
|---------|---------|
| Single file per agent | Portable, easy backup/sync |
| Time-travel queries | "What did I know on January 15th?" |
| Built-in vector index | Semantic search without external DB |
| Crash-safe WAL | Append-only, no data loss |
| 88-92% compression | Storage efficient |
| 0.025ms retrieval | Fast semantic recall |

**File Layout:**
```
data/episodic/
├── albert.mv2       # Albert's observations + reflections
├── harold.mv2       # Harold's observations + reflections
├── codie.mv2        # Codie's observations + reflections
├── shared.mv2       # Cross-agent reflections
└── system.mv2       # System-level patterns
```

```go
// pkg/brain/memory/storage_memvid.go

package memory

import (
    "context"
    "encoding/json"
    "os"
    "path/filepath"
    "time"
)

// MemvidStorage implements three-tier storage using Memvid .mv2 files.
// Uses SQLite for working memory (Tier 1) and Memvid for episodic (Tiers 2-3).
type MemvidStorage struct {
    dataDir    string
    embedding  string // bge-small-en-v1.5 or nomic-embed-text

    // Working memory (SQLite)
    workingDB  *sql.DB

    // Per-agent .mv2 files (lazy-loaded)
    agentStores map[string]*MemvidFile
}

// MemvidFile wraps a single .mv2 file with semantic + lexical search.
type MemvidFile struct {
    path     string
    writer   *MemvidWriter  // Append-only writer
    searcher *MemvidSearcher // Hybrid BM25 + vector search
}

// File patterns per Memory Enhancement PRD:
// - data/episodic/{agent}.mv2 - Per-agent episodic memory
// - Each .mv2 contains: observations, reflections, vector index, time index

func NewMemvidStorage(dataDir, embedding string) (*MemvidStorage, error) {
    if err := os.MkdirAll(dataDir, 0755); err != nil {
        return nil, err
    }

    return &MemvidStorage{
        dataDir:     dataDir,
        embedding:   embedding,
        agentStores: make(map[string]*MemvidFile),
    }, nil
}

// StoreObservation writes an observation to the agent's .mv2 file.
func (m *MemvidStorage) StoreObservation(ctx context.Context, obs *Observation) error {
    store, err := m.getOrCreateStore(obs.ResourceID)
    if err != nil {
        return err
    }

    // Frame format: timestamp + priority + content as JSON
    frame := map[string]interface{}{
        "type":        "observation",
        "id":          obs.ID,
        "content":     obs.Content,
        "timestamp":   obs.Timestamp.Unix(),
        "priority":    obs.Priority,
        "task_state":  obs.TaskState,
        "source_range": obs.SourceRange,
        "tokens":      obs.TokenCount,
    }

    data, _ := json.Marshal(frame)
    return store.writer.Put(ctx, data)
}

// StoreReflection writes a reflection to the agent's .mv2 file.
func (m *MemvidStorage) StoreReflection(ctx context.Context, ref *Reflection) error {
    store, err := m.getOrCreateStore(ref.ResourceID)
    if err != nil {
        return err
    }

    frame := map[string]interface{}{
        "type":       "reflection",
        "id":         ref.ID,
        "content":    ref.Content,
        "timestamp":  ref.Timestamp.Unix(),
        "pattern":    ref.Pattern,
        "source_obs": ref.SourceObs,
        "tokens":     ref.TokenCount,
    }

    data, _ := json.Marshal(frame)
    return store.writer.Put(ctx, data)
}

// SearchMemory performs hybrid search across observations and reflections.
func (m *MemvidStorage) SearchMemory(ctx context.Context, resourceID, query string, limit int) ([]MemoryResult, error) {
    store, err := m.getOrCreateStore(resourceID)
    if err != nil {
        return nil, err
    }

    // Memvid hybrid search: BM25 + vector similarity
    return store.searcher.Find(ctx, query, limit)
}

// GetTimeline returns chronological memory (time-travel).
func (m *MemvidStorage) GetTimeline(ctx context.Context, resourceID string, from, to time.Time) ([]MemoryResult, error) {
    store, err := m.getOrCreateStore(resourceID)
    if err != nil {
        return nil, err
    }

    return store.searcher.Timeline(ctx, from.Unix(), to.Unix())
}

// getOrCreateStore returns or creates a .mv2 file for the agent.
func (m *MemvidStorage) getOrCreateStore(resourceID string) (*MemvidFile, error) {
    if store, ok := m.agentStores[resourceID]; ok {
        return store, nil
    }

    path := filepath.Join(m.dataDir, resourceID+".mv2")
    store, err := NewMemvidFile(path, m.embedding)
    if err != nil {
        return nil, err
    }

    m.agentStores[resourceID] = store
    return store, nil
}

// Export returns the .mv2 file for backup/sync.
func (m *MemvidStorage) Export(resourceID string) (string, error) {
    return filepath.Join(m.dataDir, resourceID+".mv2"), nil
}
```

### Working Memory (SQLite)

Recent messages (Tier 1) stay in SQLite for fast access:

```go
// pkg/brain/memory/working_memory.go

package memory

// WorkingMemory manages recent messages in SQLite.
// Aligned with existing CortexBrain memory tables.
type WorkingMemory struct {
    db *sql.DB
}

// Schema (extends existing memories table)
const workingMemorySchema = `
ALTER TABLE memories ADD COLUMN om_tier INTEGER DEFAULT 1;
ALTER TABLE memories ADD COLUMN om_compressed INTEGER DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_memories_om ON memories(om_tier, om_compressed);
`

// StoreMessage adds a message to working memory.
func (w *WorkingMemory) StoreMessage(ctx context.Context, msg *Message) error {
    query := `
    INSERT INTO memories (id, owner_id, type, content, metadata, created_at, om_tier)
    VALUES (?, ?, 'episodic', ?, ?, ?, 1)
    `
    metadata, _ := json.Marshal(map[string]interface{}{
        "thread_id":  msg.ThreadID,
        "role":       msg.Role,
        "tokens":     msg.TokenCount,
    })

    _, err := w.db.ExecContext(ctx, query, msg.ID, msg.ResourceID, msg.Content, metadata, msg.Timestamp)
    return err
}

// GetMessagesToCompress returns messages ready for Observer.
func (w *WorkingMemory) GetMessagesToCompress(ctx context.Context, threshold int) ([]*Message, error) {
    // Get oldest messages exceeding token threshold
    // ...
}

// MarkCompressed marks messages as compressed (ready for deletion).
func (w *WorkingMemory) MarkCompressed(ctx context.Context, messageIDs []string) error {
    // ...
}
```

### Integration with CortexBrain

```go
// pkg/brain/brain.go (additions)

type Brain struct {
    // ... existing fields ...

    // Observational Memory
    observationalMemory *memory.ObservationalMemory
}

// Process now includes observational memory context.
func (b *Brain) Process(ctx context.Context, input Input) (*Output, error) {
    // Add message to observational memory
    if err := b.observationalMemory.AddMessage(ctx, input.ToMessage()); err != nil {
        b.log.Warn("failed to add message to observational memory", "error", err)
    }

    // Get memory context (observations + reflections)
    memoryContext, err := b.observationalMemory.GetContext(ctx, input.ThreadID, input.ResourceID)
    if err != nil {
        b.log.Warn("failed to get memory context", "error", err)
    }

    // Include memory context in processing
    input.MemoryContext = memoryContext

    // ... rest of processing ...
}
```

### Swarm Integration

The swarm agents (Harold, Albert, Codie) use Memvid for cross-session memory. Each agent has its own `.mv2` file that can be synced across machines.

```javascript
// agents/lib/observational-memory.js

const { MemvidClient } = require('./memvid-client');

class ObservationalMemory {
    constructor(config = {}) {
        this.config = {
            messageThreshold: config.messageThreshold || 30000,
            observationThreshold: config.observationThreshold || 40000,
            observerModel: config.observerModel || 'gemini-2.5-flash',
            scope: config.scope || 'resource', // Cross-session for swarm
            dataDir: config.dataDir || '/home/normanking/clawd/cortex-brain/data/episodic',
            embedding: config.embedding || 'bge-small-en-v1.5',
            ...config
        };

        // Memvid client for .mv2 file operations
        this.memvid = new MemvidClient(this.config.dataDir, this.config.embedding);

        // SQLite for working memory (recent messages)
        this.workingDB = config.workingDB || null;
    }

    async addMessage(message) {
        // Store in working memory (SQLite)
        await this.storeWorkingMessage(message);
        await this.checkThresholds(message.threadId, message.resourceId);
    }

    async storeObservation(observation) {
        // Store in Memvid .mv2 file
        const mv2Path = `${this.config.dataDir}/${observation.resourceId}.mv2`;
        await this.memvid.put(mv2Path, {
            type: 'observation',
            ...observation,
            timestamp: Date.now()
        });
    }

    async storeReflection(reflection) {
        // Store in Memvid .mv2 file
        const mv2Path = `${this.config.dataDir}/${reflection.resourceId}.mv2`;
        await this.memvid.put(mv2Path, {
            type: 'reflection',
            ...reflection,
            timestamp: Date.now()
        });
    }

    async getContext(threadId, resourceId) {
        // Get recent messages from SQLite (working memory)
        const messages = await this.getRecentMessages(threadId);

        // Get observations + reflections from Memvid
        const mv2Path = `${this.config.dataDir}/${resourceId}.mv2`;
        const observations = await this.memvid.find(mv2Path, { type: 'observation' }, 20);
        const reflections = await this.memvid.find(mv2Path, { type: 'reflection' }, 10);

        return {
            messages,
            observations,
            reflections,
            totalTokens: this.countTokens(messages, observations, reflections)
        };
    }

    async searchMemory(resourceId, query, limit = 10) {
        // Hybrid search across .mv2 file (BM25 + vector)
        const mv2Path = `${this.config.dataDir}/${resourceId}.mv2`;
        return await this.memvid.search(mv2Path, query, { limit, mode: 'hybrid' });
    }

    async getTimeline(resourceId, fromTs, toTs) {
        // Time-travel query: "what did I know at time X?"
        const mv2Path = `${this.config.dataDir}/${resourceId}.mv2`;
        return await this.memvid.timeline(mv2Path, fromTs, toTs);
    }

    async exportMemory(resourceId) {
        // Export .mv2 file for backup/sync
        return `${this.config.dataDir}/${resourceId}.mv2`;
    }
}

module.exports = { ObservationalMemory };
```

**Cross-Machine Sync:**
```bash
# Sync agent memory from Pink to Harold
rsync -avz pink:~/cortex-brain/data/episodic/codie.mv2 ~/clawd/cortex-brain/data/episodic/

# Sync agent memory from Harold to Pink
rsync -avz ~/clawd/cortex-brain/data/episodic/harold.mv2 pink:~/cortex-brain/data/episodic/
```

---

## Implementation Phases

### Phase 1: Core Infrastructure (Week 1-2)

**Deliverables:**
- [ ] `pkg/brain/memory/observational.go` - Core types and interfaces
- [ ] `pkg/brain/memory/storage_memvid.go` - Memvid .mv2 storage backend
- [ ] `pkg/brain/memory/working_memory.go` - SQLite working memory
- [ ] `pkg/brain/memory/config.go` - Configuration management
- [ ] Unit tests for storage operations

**Acceptance Criteria:**
- Messages can be stored and retrieved from SQLite
- Observations/reflections stored in .mv2 files
- Token counting is accurate
- Per-agent .mv2 files created on first write
- Time-travel queries work correctly
- Configuration is loaded from YAML

### Phase 2: Observer Agent (Week 2-3)

**Deliverables:**
- [ ] `pkg/brain/memory/observer.go` - Observer agent implementation
- [ ] Observer system prompt optimized for compression
- [ ] Background goroutine for monitoring
- [ ] Integration with Gemini 2.5 Flash via Pinky

**Acceptance Criteria:**
- Messages are compressed when threshold exceeded
- Compression achieves 5-40x ratio
- Priority tagging works correctly
- Task state is preserved

### Phase 3: Reflector Agent (Week 3-4)

**Deliverables:**
- [ ] `pkg/brain/memory/reflector.go` - Reflector agent implementation
- [ ] Reflector system prompt for pattern recognition
- [ ] Garbage collection of stale observations
- [ ] Cross-session memory persistence

**Acceptance Criteria:**
- Observations are consolidated into reflections
- Patterns are correctly identified
- Stale observations are garbage collected
- Memory persists across sessions

### Phase 4: Integration & Testing (Week 4-5)

**Deliverables:**
- [ ] Integration with Brain.Process()
- [ ] Integration with swarm agents (Harold, Albert, Codie)
- [ ] `agents/lib/observational-memory.js` for Node.js agents
- [ ] LongMemEval benchmark suite
- [ ] Cost tracking and comparison

**Acceptance Criteria:**
- ≥85% on LongMemEval benchmark
- ≥5x cost reduction measured
- Swarm agents maintain cross-session memory
- No performance regression in normal operations

### Phase 5: Documentation & Rollout (Week 5-6)

**Deliverables:**
- [ ] API documentation
- [ ] Configuration guide
- [ ] Migration guide for existing memory
- [ ] Monitoring dashboard updates

---

## Configuration

### Default Configuration

```yaml
# config/observational-memory.yaml

observational_memory:
  enabled: true

  # Token thresholds
  message_threshold: 30000      # Trigger Observer at 30k tokens
  observation_threshold: 40000  # Trigger Reflector at 40k tokens

  # Model configuration
  observer_model: "gemini-2.5-flash"
  reflector_model: "gemini-2.5-flash"

  # Scope: "thread" (per-conversation) or "resource" (per-user/agent)
  scope: "resource"  # Cross-session for swarm

  # Token budget sharing
  share_token_budget: false

  # Storage (per Memory Enhancement PRD)
  storage:
    # Working Memory: SQLite (fast, ephemeral)
    working:
      backend: "sqlite"
      path: "./cortex-brain.db"  # Existing CortexBrain database

    # Episodic Memory: Memvid .mv2 files (long-term, portable)
    episodic:
      backend: "memvid"
      data_dir: "./data/episodic"     # Per-agent .mv2 files
      embedding: "bge-small-en-v1.5"  # Built-in to Memvid
      # File layout:
      # - data/episodic/{agent}.mv2 - Per-agent memory
      # - data/episodic/shared.mv2  - Cross-agent patterns

  # Background agent intervals
  observer_interval: "10s"
  reflector_interval: "30s"

  # Retention
  max_observations_per_agent: 1000
  max_reflections_per_agent: 100
  ttl_days: 90

  # Memvid-specific settings
  memvid:
    compress: true            # Enable compression (88-92%)
    time_index: true          # Enable time-travel queries
    vector_index: true        # Enable semantic search
    lexical_index: true       # Enable BM25 search
```

### Per-Agent Override

```yaml
# agents/harold/memory.yaml

observational_memory:
  scope: "resource"           # Harold remembers across all threads
  message_threshold: 50000    # Higher threshold for overseer
  observer_model: "deepseek"  # Cost optimization
  storage:
    episodic:
      data_dir: "/home/normanking/clawd/cortex-brain/data/episodic"
      # harold.mv2 created automatically on first observation
```

### Memvid File Layout

```
data/episodic/
├── albert.mv2       # Albert's observations + reflections (OpenClaw)
├── harold.mv2       # Harold's observations + reflections (Swarm Overseer)
├── codie.mv2        # Codie's observations + reflections (Claude Code)
├── shared.mv2       # Cross-agent insights (automatically populated)
└── system.mv2       # System-level patterns
```

Each `.mv2` file is self-contained with:
- Frame data (observations/reflections as JSON)
- Lexical index (BM25, always available)
- Vector index (bge-small embeddings)
- Time index (chronological ordering for time-travel)
- WAL (write-ahead log for crash safety)

---

## API Design

### Go API

```go
// Create observational memory
om, err := memory.NewObservationalMemory(config)

// Add message
err = om.AddMessage(ctx, message)

// Get context for prompt
context, err := om.GetContext(ctx, threadID, resourceID)

// Force observation (manual trigger)
err = om.ForceObserve(ctx, threadID)

// Get statistics
stats := om.Stats()
```

### Node.js API (Swarm Agents)

```javascript
const om = new ObservationalMemory({
    dataDir: '/home/normanking/clawd/cortex-brain/data/episodic',
    embedding: 'bge-small-en-v1.5'
});

// Add message (stored in SQLite working memory)
await om.addMessage({ role: 'user', content: '...', threadId, resourceId });

// Get context (working + episodic from .mv2)
const context = await om.getContext(threadId, resourceId);

// Search across agent's memory (Memvid hybrid search)
const results = await om.searchMemory(resourceId, 'deployment issues', 10);

// Time-travel: what did agent know on specific date?
const pastKnowledge = await om.getTimeline(resourceId, new Date('2026-02-01'), new Date('2026-02-05'));

// Export .mv2 for backup/sync
const mv2Path = await om.exportMemory(resourceId);

// Use in prompt
const prompt = `
${context.reflections.map(r => r.content).join('\n')}
${context.observations.map(o => o.content).join('\n')}

Recent messages:
${context.messages.map(m => `${m.role}: ${m.content}`).join('\n')}
`;
```

---

## Model Selection

### Recommended Models

| Model | Context | Cost | Speed | Notes |
|-------|---------|------|-------|-------|
| **Gemini 2.5 Flash** | 1M | Low | Fast | Default, tested by Mastra |
| DeepSeek | 64k | Very Low | Fast | Budget option |
| Qwen3 | 128k | Low | Fast | Alternative |
| GLM-4.7 | 128k | Low | Medium | Chinese support |

### Not Recommended

| Model | Reason |
|-------|--------|
| Claude 4.5 | Poor performance as Observer/Reflector (per Mastra) |
| GPT-4o | High cost for background agents |

---

## Monitoring & Observability

### Metrics

```
# Token metrics
cortex_om_messages_tokens_total{thread_id, resource_id}
cortex_om_observations_tokens_total{resource_id}
cortex_om_reflections_tokens_total{resource_id}

# Compression metrics
cortex_om_compression_ratio{resource_id}
cortex_om_observations_created_total{resource_id}
cortex_om_reflections_created_total{resource_id}

# Performance metrics
cortex_om_observer_duration_seconds{resource_id}
cortex_om_reflector_duration_seconds{resource_id}

# Cost metrics
cortex_om_observer_cost_total{resource_id}
cortex_om_reflector_cost_total{resource_id}
cortex_om_cost_savings_total{resource_id}
```

### Dashboard

Add to swarm dashboard (192.168.1.128:18870):
- Observational memory token usage by agent
- Compression ratios over time
- Cost savings graph
- Memory tier distribution

---

## Risks & Mitigations

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Observer loses important info | Medium | High | Priority tagging, human review option |
| Model incompatibility | Low | Medium | Use tested models (Gemini, DeepSeek) |
| Memvid .mv2 file corruption | Very Low | Medium | WAL for crash safety, periodic backups |
| .mv2 file size growth | Low | Low | 88-92% compression, auto-trimming |
| Background agent latency | Medium | Low | Async processing, configurable intervals |
| Cross-session conflicts | Low | Medium | Resource-scoped .mv2 files, file locking |
| Memvid SDK not available | Low | Medium | Fallback to CLI, Python subprocess |

---

## Dependencies

### Required

- vLLM Integration (P1) - For fast inference on Observer/Reflector
- Memvid (per Memory Enhancement PRD) - Episodic .mv2 storage backend
- SQLite (existing) - Working memory storage
- Pinky Gateway (existing) - Model routing

### Integration with Memory Enhancement PRD

This PRD builds on the dual-layer memory architecture defined in the **Memory Enhancement PRD**:

| Layer | Storage | Purpose | This PRD |
|-------|---------|---------|----------|
| Working Memory | SQLite | Fast, ephemeral | Tier 1: Recent messages |
| Episodic Memory | Memvid .mv2 | Long-term, semantic | Tiers 2-3: Observations + Reflections |

**Key Alignment:**
- Uses same `.mv2` file format per agent
- Integrates with existing consolidation engine
- Shares embedding model (bge-small-en-v1.5)
- Time-travel queries work across both systems

### Optional

- Prometheus/Grafana - Metrics
- Swarm Dashboard - Visualization
- Redis (optional) - Can use for cross-machine sync coordination

---

## Timeline

| Phase | Duration | Dependencies |
|-------|----------|--------------|
| Phase 1: Core Infrastructure | 2 weeks | None |
| Phase 2: Observer Agent | 1 week | Phase 1, vLLM |
| Phase 3: Reflector Agent | 1 week | Phase 2 |
| Phase 4: Integration | 1 week | Phase 3 |
| Phase 5: Documentation | 1 week | Phase 4 |
| **Total** | **6 weeks** | After vLLM P1 |

---

## References

### Internal Documents
- [Memory Enhancement PRD](../../Cortex/Gateway/CortexBrain-Memory-Enhancement-PRD.md) - Defines Memvid integration
- [CortexBrain Architecture](../README.md)
- [Observational Memory Analysis](./OBSERVATIONAL-MEMORY-ANALYSIS.md)

### External Sources
- [Mastra Observational Memory](https://mastra.ai/docs/memory/observational-memory)
- [VentureBeat: 10x Cost Reduction](https://venturebeat.com/data/observational-memory-cuts-ai-agent-costs-10x-and-outscores-rag-on-long)
- [Mastra GitHub Implementation](https://github.com/mastra-ai/mastra)
- [Memvid: Video-based AI Memory](https://github.com/olow/memvid) - .mv2 format specification

---

## Approval

| Role | Name | Date | Status |
|------|------|------|--------|
| Author | Codie | 2026-02-10 | ✅ Created |
| Technical Review | - | - | Pending |
| Product Owner | Norman | - | Pending |
