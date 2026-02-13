---
project: Cortex
component: Memory
phase: Design
date_created: 2026-02-10T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-11T01:40:42.880203
---

# Observational Memory PRD

**Project:** CortexBrain
**Feature:** Observational Memory System
**Priority:** P2 (After vLLM Integration)
**Status:** Approved for Development
**Author:** Codie
**Date:** 2026-02-10

---

## Executive Summary

Implement a three-tier observational memory system for CortexBrain that uses background agents to compress conversation history into dense observations and reflections. This capability will enable long-running agent conversations without context degradation, reduce inference costs by 10x, and provide cross-session memory for swarm agents (Harold, Albert, Codie).

**Decision:** EMBEDDED (not plugin) - memory consolidation is core cognition, like the biological hippocampus.

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

    // Storage backend
    StorageBackend string `yaml:"storage_backend"` // redis, postgres, sqlite
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

#### 4. Storage Backend (Redis)

```go
// pkg/brain/memory/storage_redis.go

package memory

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/redis/go-redis/v9"
)

// RedisStorage implements three-tier storage using Redis.
type RedisStorage struct {
    client *redis.Client
    prefix string
}

// Redis key patterns:
// - {prefix}:messages:{thread_id} - Sorted set of messages by timestamp
// - {prefix}:observations:{resource_id} - Sorted set of observations
// - {prefix}:reflections:{resource_id} - Sorted set of reflections
// - {prefix}:tokens:{thread_id} - Token count tracking

func (r *RedisStorage) StoreMessage(ctx context.Context, msg *Message) error {
    key := fmt.Sprintf("%s:messages:%s", r.prefix, msg.ThreadID)
    data, _ := json.Marshal(msg)
    return r.client.ZAdd(ctx, key, redis.Z{
        Score:  float64(msg.Timestamp.UnixNano()),
        Member: data,
    }).Err()
}

func (r *RedisStorage) StoreObservation(ctx context.Context, obs *Observation) error {
    key := fmt.Sprintf("%s:observations:%s", r.prefix, obs.ResourceID)
    data, _ := json.Marshal(obs)
    return r.client.ZAdd(ctx, key, redis.Z{
        Score:  float64(obs.Timestamp.UnixNano()),
        Member: data,
    }).Err()
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

```go
// agents/lib/observational-memory.js

class ObservationalMemory {
    constructor(redisClient, config = {}) {
        this.redis = redisClient;
        this.config = {
            messageThreshold: config.messageThreshold || 30000,
            observationThreshold: config.observationThreshold || 40000,
            observerModel: config.observerModel || 'gemini-2.5-flash',
            scope: config.scope || 'resource', // Cross-session for swarm
            ...config
        };
    }

    async addMessage(message) {
        const key = `cortex:om:messages:${message.threadId}`;
        await this.redis.zadd(key, message.timestamp, JSON.stringify(message));
        await this.checkThresholds(message.threadId, message.resourceId);
    }

    async getContext(threadId, resourceId) {
        // Get recent messages + observations + reflections
        const messages = await this.getRecentMessages(threadId);
        const observations = await this.getObservations(resourceId);
        const reflections = await this.getReflections(resourceId);

        return {
            messages,
            observations,
            reflections,
            totalTokens: this.countTokens(messages, observations, reflections)
        };
    }
}

module.exports = { ObservationalMemory };
```

---

## Implementation Phases

### Phase 1: Core Infrastructure (Week 1-2)

**Deliverables:**
- [ ] `pkg/brain/memory/observational.go` - Core types and interfaces
- [ ] `pkg/brain/memory/storage_redis.go` - Redis storage backend
- [ ] `pkg/brain/memory/config.go` - Configuration management
- [ ] Unit tests for storage operations

**Acceptance Criteria:**
- Messages can be stored and retrieved
- Token counting is accurate
- Redis keys follow naming convention
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

  # Storage
  storage:
    backend: "redis"
    redis:
      host: "192.168.1.186"
      port: 6379
      prefix: "cortex:om"

  # Background agent intervals
  observer_interval: "10s"
  reflector_interval: "30s"

  # Retention
  max_observations: 1000
  max_reflections: 100
  ttl_days: 90
```

### Per-Agent Override

```yaml
# agents/harold/memory.yaml

observational_memory:
  scope: "resource"           # Harold remembers across all threads
  message_threshold: 50000    # Higher threshold for overseer
  observer_model: "deepseek"  # Cost optimization
```

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
const om = new ObservationalMemory(redisClient, config);

// Add message
await om.addMessage({ role: 'user', content: '...', threadId, resourceId });

// Get context
const context = await om.getContext(threadId, resourceId);

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
| Redis storage exhaustion | Low | Medium | TTL, max entries, auto-trimming |
| Background agent latency | Medium | Low | Async processing, configurable intervals |
| Cross-session conflicts | Low | Medium | Resource-scoped locking |

---

## Dependencies

### Required

- vLLM Integration (P1) - For fast inference on Observer/Reflector
- Redis (existing) - Storage backend
- Pinky Gateway (existing) - Model routing

### Optional

- Prometheus/Grafana - Metrics
- Swarm Dashboard - Visualization

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

- [Mastra Observational Memory](https://mastra.ai/docs/memory/observational-memory)
- [VentureBeat: 10x Cost Reduction](https://venturebeat.com/data/observational-memory-cuts-ai-agent-costs-10x-and-outscores-rag-on-long)
- [Mastra GitHub Implementation](https://github.com/mastra-ai/mastra)
- [CortexBrain Architecture](../README.md)
- [Observational Memory Analysis](./OBSERVATIONAL-MEMORY-ANALYSIS.md)

---

## Approval

| Role | Name | Date | Status |
|------|------|------|--------|
| Author | Codie | 2026-02-10 | ✅ Created |
| Technical Review | - | - | Pending |
| Product Owner | Norman | - | Pending |
