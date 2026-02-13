---
project: Cortex
component: UI
phase: Design
date_created: 2026-01-08T12:37:35
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:29.529458
---

# CortexBrain Performance Optimization Guide

**Implementation Guide for High-Priority Optimizations**

---

## Optimization #1: Copy-on-Write for Blackboard.Clone()

**Severity:** High
**Estimated Impact:** 70-80% latency reduction on parallel branches
**Estimated Effort:** 1-2 days
**Files to Modify:**
- `/Users/normanking/ServerProjectsMac/cortex-brain-main/pkg/brain/blackboard.go`
- `/Users/normanking/ServerProjectsMac/cortex-brain-main/pkg/brain/parallel_executor.go` (call site)

### Current Implementation Issues

```go
// Current: Deep copy every time
func (b *Blackboard) Clone() *Blackboard {
    b.mu.RLock()
    defer b.mu.RUnlock()

    newBB := NewBlackboard()
    // ... copy all maps and slices ...

    // Deep copy of slices - O(n) allocations
    if len(b.Memories) > 0 {
        newBB.Memories = make([]Memory, len(b.Memories))
        copy(newBB.Memories, b.Memories)
    }
    // Cost: 3 clones × 20 memories = 60 allocations per parallel execution
    return newBB
}
```

### Proposed Solution: Lazy Copy-on-Write

**Step 1: Modify Blackboard struct**

```go
type Blackboard struct {
    mu sync.RWMutex

    // Core data
    data map[string]interface{}
    Memories  []Memory
    Entities  []Entity
    UserState *UserState

    // CoW tracking
    original *Blackboard  // Ref to parent (nil if original)
    writes   sync.Map     // Track modified keys for lazy materialization

    // Session info
    ConversationID string
    TurnNumber     int
    OverallConfidence float64
}
```

**Step 2: Implement CloneCoW**

```go
// Clone creates a copy-on-write view of the blackboard
func (b *Blackboard) Clone() *Blackboard {
    b.mu.RLock()
    defer b.mu.RUnlock()

    return &Blackboard{
        data:              b.data,      // Shared reference
        Memories:          b.Memories,  // Shared reference
        Entities:          b.Entities,  // Shared reference
        UserState:         b.UserState, // Shared reference
        ConversationID:    b.ConversationID,
        TurnNumber:        b.TurnNumber,
        OverallConfidence: b.OverallConfidence,
        original:          b,           // Track parent
        writes:            sync.Map{},  // Empty initially
    }
}
```

**Step 3: Lazy materialization on Set**

```go
func (b *Blackboard) Set(key string, value interface{}) {
    b.mu.Lock()
    defer b.mu.Unlock()

    // If this is a CoW clone and we haven't materialized yet
    if b.original != nil {
        // Mark as written to track modifications
        b.writes.Store(key, true)

        // Lazy materialize data map on first write
        if b.data == b.original.data {
            newData := make(map[string]interface{})
            b.original.mu.RLock()
            for k, v := range b.original.data {
                newData[k] = v
            }
            b.original.mu.RUnlock()
            b.data = newData
        }
    }

    b.data[key] = value
}
```

**Step 4: Memory operations**

```go
func (b *Blackboard) AddMemory(mem Memory) {
    b.mu.Lock()
    defer b.mu.Unlock()

    if b.original != nil {
        // Lazy materialize Memories slice
        if b.Memories == b.original.Memories {
            newMemories := make([]Memory, len(b.original.Memories), len(b.original.Memories)+10)
            copy(newMemories, b.original.Memories)
            b.Memories = newMemories
        }
    }

    b.Memories = append(b.Memories, mem)
}
```

### Performance Comparison

**Before:**
```
Clone cost: 3 × (1 map copy + 1 memory slice copy + 1 entity slice copy)
         = 3 × (O(n) + O(m))
         ≈ 2-5ms per parallel execution
```

**After (CoW):**
```
Clone cost: O(1) - just reference sharing
Add memory: O(1) amortized (lazy materialize only on first write)
         ≈ 0.1-0.2ms per parallel execution
         = 95% faster for read-heavy branches
```

### Testing Strategy

```go
func TestBlackboardCoW(t *testing.T) {
    bb := NewBlackboard()
    bb.Set("test", "value")
    bb.AddMemory(Memory{ID: "mem1", Content: "content"})

    // Clone should not copy data
    clone := bb.Clone()

    // Should share reference initially
    val, _ := clone.Get("test")
    assert.Equal(t, "value", val)

    // After write, should materialize
    clone.Set("new_key", "new_value")

    // Original should not be affected
    _, exists := bb.Get("new_key")
    assert.False(t, exists)
}
```

---

## Optimization #2: Top-K Vector Search using Min-Heap

**Severity:** High
**Estimated Impact:** 50-70% faster for large result sets (1000+ candidates)
**Estimated Effort:** 1 day
**Files to Modify:**
- `/Users/normanking/ServerProjectsMac/cortex-brain-main/internal/memory/vector_index.go`

### Current Issues

```go
// Current: Unbounded append + sort
var candidates []ScoredItem[GenericMemory]  // Empty, grows dynamically

for _, bucketID := range allBuckets {
    rows, err := vi.db.QueryContext(...)
    for rows.Next() {
        // ...
        candidates = append(candidates, item)  // O(n) worst case
    }
}

sort.Slice(candidates, ...)  // O(n log n) on FULL list
if len(candidates) > limit {
    candidates = candidates[:limit]  // Wasteful truncation
}
```

**Problems:**
1. No pre-allocation → multiple slice growths
2. Sort before truncate → O(n log n) instead of O(n log k)
3. Memory waste on unused items

### Proposed Solution: Min-Heap Priority Queue

**Step 1: Implement MinHeap**

```go
// minHeap implements a min-heap for top-K selection
type minHeap struct {
    items   []interface{}
    compare func(a, b interface{}) bool  // true if a < b
    maxSize int
}

func newMinHeap(maxSize int, compare func(a, b interface{}) bool) *minHeap {
    return &minHeap{
        items:   make([]interface{}, 0, maxSize),
        compare: compare,
        maxSize: maxSize,
    }
}

func (h *minHeap) Push(item interface{}) {
    if len(h.items) < h.maxSize {
        h.items = append(h.items, item)
        h.upHeap(len(h.items) - 1)
    } else if h.compare(item, h.items[0]) {
        // Item is larger than min, replace root
        h.items[0] = item
        h.downHeap(0)
    }
}

func (h *minHeap) ExtractAll() []interface{} {
    result := make([]interface{}, len(h.items))
    copy(result, h.items)
    sort.Slice(result, func(i, j int) bool {
        return !h.compare(result[i], result[j])  // Reverse order
    })
    return result
}

// Helper methods: upHeap, downHeap, etc.
```

**Step 2: Refactor SearchSimilar**

```go
func (vi *VectorIndex) SearchSimilar(ctx context.Context, queryEmb []float32, limit int, threshold float64) ([]ScoredItem[GenericMemory], error) {
    // Create min-heap to track top-K
    pq := newMinHeap(limit, func(a, b interface{}) bool {
        // a < b means a is SMALLER score (for min-heap)
        return a.(ScoredItem[GenericMemory]).Score < b.(ScoredItem[GenericMemory]).Score
    })

    primaryBucket := vi.computeBucketID(queryEmb)
    adjacentBuckets := vi.getAdjacentBuckets(primaryBucket)
    allBuckets := append([]string{primaryBucket}, adjacentBuckets...)

    for _, bucketID := range allBuckets {
        rows, err := vi.db.QueryContext(ctx, `
            SELECT eb.memory_id, eb.memory_type
            FROM embedding_buckets eb
            WHERE eb.bucket_id = ?
        `, bucketID)
        if err != nil {
            continue
        }

        for rows.Next() {
            var memID, memType string
            if err := rows.Scan(&memID, &memType); err != nil {
                continue
            }

            emb, content, err := vi.getMemoryEmbedding(ctx, memID, MemoryType(memType))
            if err != nil || emb == nil {
                continue
            }

            sim := CosineSimilarity(queryEmb, emb)
            if sim >= threshold {
                // Add to heap (doesn't sort entire list)
                pq.Push(ScoredItem[GenericMemory]{
                    Item: GenericMemory{
                        ID:        memID,
                        Type:      MemoryType(memType),
                        Content:   content,
                        Embedding: emb,
                    },
                    Score: sim,
                })
            }
        }
        rows.Close()
    }

    // Extract top-K in order
    results := pq.ExtractAll()
    typed := make([]ScoredItem[GenericMemory], len(results))
    for i, item := range results {
        typed[i] = item.(ScoredItem[GenericMemory])
    }
    return typed, nil
}
```

### Performance Comparison

**Before:**
```
Time: O(n log n) for full sort + O(n) for append operations
Space: O(n) for candidates list

Example: 1000 candidates, limit=10
- 1000 appends: ~5ms (amortized)
- 1000 elements sort: ~10ms
- Total: 15ms
```

**After:**
```
Time: O(n log k) for heap operations + O(k log k) for final sort
Space: O(k) for heap

Example: 1000 candidates, limit=10
- 1000 heap pushes: ~1ms (n log k = 1000 × log 10)
- 10 element sort: <0.1ms
- Total: 1.1ms
= 93% faster
```

---

## Optimization #3: Prevent Streaming Goroutine Leaks

**Severity:** Medium
**Estimated Impact:** Prevents rare goroutine leaks (error cases)
**Estimated Effort:** 2 hours
**Files to Modify:**
- `/Users/normanking/ServerProjectsMac/cortex-brain-main/internal/orchestrator/streaming.go`

### Current Issue

```go
func (o *Orchestrator) ProcessStream(ctx context.Context, req *Request) (<-chan *StreamChunk, error) {
    chunkCh := make(chan *StreamChunk, 100)

    go o.runStreamingPipeline(ctx, req, chunkCh)
    // ↑ If caller abandons channel, goroutine blocks on channel write
    return chunkCh, nil
}
```

### Fix: Detect Early Channel Abandonment

**Option 1: Wrapper Goroutine (Safe)**

```go
func (o *Orchestrator) ProcessStream(ctx context.Context, req *Request) (<-chan *StreamChunk, error) {
    chunkCh := make(chan *StreamChunk, 100)

    go func() {
        defer close(chunkCh)
        o.runStreamingPipeline(ctx, req, chunkCh)
    }()

    return chunkCh, nil
}
```

**Then in runStreamingPipeline, add context check:**

```go
func (o *Orchestrator) runStreamingPipeline(ctx context.Context, req *Request, chunkCh chan *StreamChunk) {
    // Check if context is cancelled before starting
    select {
    case <-ctx.Done():
        return
    default:
    }

    // ... pipeline implementation ...

    // When sending chunks, check context
    select {
    case chunkCh <- chunk:
    case <-ctx.Done():
        return  // Unblock if caller cancels
    }
}
```

**Option 2: Timeout Wrapper (for extra safety)**

```go
func (o *Orchestrator) ProcessStreamWithTimeout(ctx context.Context, req *Request, timeout time.Duration) (<-chan *StreamChunk, error) {
    // Create new context with timeout
    ctx, cancel := context.WithTimeout(ctx, timeout)
    chunkCh := make(chan *StreamChunk, 100)

    go func() {
        defer close(chunkCh)
        defer cancel()
        o.runStreamingPipeline(ctx, req, chunkCh)
    }()

    return chunkCh, nil
}
```

### Testing

```go
func TestStreamingLeakPrevention(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())

    // Start streaming
    chunks, err := o.ProcessStream(ctx, req)
    assert.NoError(t, err)

    // Cancel context (simulating caller abandonment)
    cancel()

    // Goroutine should exit gracefully
    time.Sleep(100 * time.Millisecond)

    // Read goroutine count - should not leak
    finalGoroutines := runtime.NumGoroutine()
    assert.Less(t, finalGoroutines, initialGoroutines + 5)
}
```

---

## Optimization #4: EventBus Metrics & Monitoring

**Severity:** Low (observability)
**Estimated Impact:** Better debugging and tuning
**Estimated Effort:** 4 hours
**Files to Modify:**
- `/Users/normanking/ServerProjectsMac/cortex-brain-main/internal/bus/bus.go`

### Current Issue

```go
func (b *EventBus) PublishAsync(event Event) {
    // ...
    select {
    case b.asyncCh <- event:
    default:
        // Buffer full, event dropped - SILENT
    }
}
```

### Solution: Add Metrics

```go
type EventBus struct {
    mu            sync.RWMutex
    subscriptions map[string][]*subscription
    allHandlers   []*subscription
    nextID        uint64
    asyncCh       chan Event
    closed        atomic.Bool

    // NEW: Metrics
    metrics struct {
        asyncPublished int64
        asyncDropped   int64
        dispatchErrors int64
        bufferSize     int
    }
}

func (b *EventBus) PublishAsync(event Event) {
    if b.closed.Load() {
        return
    }
    select {
    case b.asyncCh <- event:
        atomic.AddInt64(&b.metrics.asyncPublished, 1)
    default:
        atomic.AddInt64(&b.metrics.asyncDropped, 1)
        // Log warning
        if b.metrics.asyncDropped % 100 == 0 {
            log.Warn().
                Int64("dropped_count", atomic.LoadInt64(&b.metrics.asyncDropped)).
                Msg("EventBus async buffer overflow")
        }
    }
}

func (b *EventBus) Metrics() map[string]int64 {
    return map[string]int64{
        "async_published": atomic.LoadInt64(&b.metrics.asyncPublished),
        "async_dropped":   atomic.LoadInt64(&b.metrics.asyncDropped),
        "dispatch_errors": atomic.LoadInt64(&b.metrics.dispatchErrors),
    }
}
```

---

## Optimization #5: Parallel Vector Bucket Queries

**Severity:** Medium
**Estimated Impact:** 30-50% faster vector search
**Estimated Effort:** 1 day
**Files to Modify:**
- `/Users/normanking/ServerProjectsMac/cortex-brain-main/internal/memory/vector_index.go`

### Current Implementation (Serial)

```go
for _, bucketID := range allBuckets {
    rows, err := vi.db.QueryContext(ctx, `...`)
    // Process rows serially
}
```

### Parallel Implementation

```go
func (vi *VectorIndex) SearchSimilarParallel(ctx context.Context, queryEmb []float32, limit int, threshold float64) ([]ScoredItem[GenericMemory], error) {
    primaryBucket := vi.computeBucketID(queryEmb)
    adjacentBuckets := vi.getAdjacentBuckets(primaryBucket)
    allBuckets := append([]string{primaryBucket}, adjacentBuckets...)

    // Parallel bucket queries
    type bucketResult struct {
        items []ScoredItem[GenericMemory]
        err   error
    }

    resultsCh := make(chan bucketResult, len(allBuckets))
    var wg sync.WaitGroup

    for _, bucketID := range allBuckets {
        wg.Add(1)
        go func(bID string) {
            defer wg.Done()
            items, err := vi.querySimilarInBucket(ctx, bID, queryEmb, threshold)
            resultsCh <- bucketResult{items, err}
        }(bucketID)
    }

    go func() {
        wg.Wait()
        close(resultsCh)
    }()

    // Aggregate and select top-K
    pq := newMinHeap(limit, scoreCompare)
    for result := range resultsCh {
        if result.err != nil {
            continue
        }
        for _, item := range result.items {
            pq.Push(item)
        }
    }

    return pq.ExtractAll(), nil
}

func (vi *VectorIndex) querySimilarInBucket(ctx context.Context, bucketID string, queryEmb []float32, threshold float64) ([]ScoredItem[GenericMemory], error) {
    rows, err := vi.db.QueryContext(ctx, `
        SELECT eb.memory_id, eb.memory_type
        FROM embedding_buckets eb
        WHERE eb.bucket_id = ?
    `, bucketID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var items []ScoredItem[GenericMemory]
    for rows.Next() {
        var memID, memType string
        if err := rows.Scan(&memID, &memType); err != nil {
            continue
        }

        emb, content, err := vi.getMemoryEmbedding(ctx, memID, MemoryType(memType))
        if err != nil || emb == nil {
            continue
        }

        sim := CosineSimilarity(queryEmb, emb)
        if sim >= threshold {
            items = append(items, ScoredItem[GenericMemory]{
                Item: GenericMemory{
                    ID:      memID,
                    Type:    MemoryType(memType),
                    Content: content,
                    // Omit Embedding for memory efficiency
                },
                Score: sim,
            })
        }
    }

    return items, rows.Err()
}
```

**Tradeoff:** Slightly higher memory/concurrency for faster throughput

---

## Implementation Checklist

### Week 1: High-Priority Optimizations

- [ ] Implement Blackboard CoW (2 days)
  - [ ] Modify struct definition
  - [ ] Implement lazy materialization
  - [ ] Add comprehensive tests
  - [ ] Benchmark vs. current approach

- [ ] Implement Vector Search Top-K (1 day)
  - [ ] Create MinHeap utility
  - [ ] Refactor SearchSimilar
  - [ ] Add benchmarks
  - [ ] Verify correctness on large result sets

### Week 2: Medium-Priority Optimizations

- [ ] Fix streaming goroutine leak (2-4 hours)
  - [ ] Add wrapper goroutine
  - [ ] Add context checks in pipeline
  - [ ] Add leak detection test

- [ ] Add EventBus metrics (4 hours)
  - [ ] Add metrics fields to struct
  - [ ] Implement metrics tracking
  - [ ] Add metrics export endpoint
  - [ ] Set up alerts for drops

- [ ] Implement parallel vector queries (1 day)
  - [ ] Extract querySimilarInBucket function
  - [ ] Implement parallel pattern
  - [ ] Benchmark multi-bucket performance
  - [ ] Tune max concurrency

### Week 3+: Monitoring & Testing

- [ ] Add performance monitoring
  - [ ] Clone latency histograms
  - [ ] Vector search percentiles
  - [ ] Goroutine leak detection

- [ ] Load testing
  - [ ] Run parallel executor stress tests
  - [ ] Memory profiling after optimizations
  - [ ] CPU profiling with pprof

---

## Benchmarking Commands

```bash
# Run benchmarks for parallel executor
go test -bench=BenchmarkParallelExecutor -benchmem -cpuprofile=cpu.prof ./pkg/brain/
go tool pprof cpu.prof

# Memory profiling
go test -bench=BenchmarkBlackboardClone -benchmem -memprofile=mem.prof ./pkg/brain/
go tool pprof mem.prof

# Goroutine leak detection
go test -bench=. -race -run=TestStreamingLeak ./internal/orchestrator/

# Vector search benchmarks
go test -bench=BenchmarkVectorSearch -benchmem ./internal/memory/
```

---

## Monitoring Metrics

Add these to your metrics dashboard:

```go
// Metrics to track
metrics := map[string]interface{}{
    // Latency
    "blackboard.clone_latency_ms":      float64,
    "vector_search.latency_p99":        float64,
    "orchestrator.process_latency_ms":  float64,

    // Memory
    "blackboard.clone_allocations":     int64,
    "vector_search.candidates_count":   int64,
    "eventbus.async_dropped_total":     int64,

    // Concurrency
    "goroutine_count":                  int,
    "db_connection_pool.active":        int,
    "db_connection_pool.idle":          int,
}
```

---

## Risk Mitigation

### Blackboard CoW Risks

- **Risk:** Breaking existing code that modifies clones
- **Mitigation:** Add comprehensive test suite, run existing tests
- **Rollback:** Keep original Clone() as CloneDeep(), gradual migration

### Vector Search Top-K Risks

- **Risk:** MinHeap correctness with edge cases
- **Mitigation:** Property-based testing, fuzz testing
- **Rollback:** Keep old SearchSimilar as SearchSimilarBruteForce()

### Parallel Vector Queries Risks

- **Risk:** Database connection pool exhaustion
- **Mitigation:** Limit concurrent queries, monitor pool usage
- **Rollback:** Keep serial implementation, runtime flag to switch

---

## Expected Improvements

After implementing all 5 optimizations:

| Metric | Before | After | Improvement |
|--------|--------|-------|-------------|
| Parallel request latency | 20-30ms | 10-15ms | 50% |
| Vector search (1000 items) | 15ms | 3-5ms | 70% |
| Memory allocations per request | 500+ | 100-150 | 75% |
| Goroutine leaks/week | ~5 | 0 | 100% |
| EventBus observability | None | Full metrics | - |

---

## Next Steps

1. **Review & Approve:** Get code review on optimization designs
2. **Benchmark Baseline:** Run benchmarks on main branch first
3. **Implement P1:** Start with Blackboard CoW + Vector Top-K
4. **Measure:** Compare latencies before/after
5. **Iterate:** Move to P2 optimizations based on profiling results
