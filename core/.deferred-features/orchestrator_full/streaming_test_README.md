---
project: Cortex
component: Docs
phase: Archive
date_created: 2026-01-08T13:30:13
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:51.741113
---

# Streaming Tests README

## Overview

This test file validates the streaming goroutine fix that prevents goroutine leaks and deadlocks when contexts are cancelled during streaming operations.

## Quick Start

Run all streaming tests:
```bash
go test -v ./internal/orchestrator -run TestStreaming -timeout 30s
```

## Individual Tests

### Context Cancellation
```bash
go test -v -run TestStreamingContextCancellation
```
Verifies that streaming goroutines exit cleanly when context is cancelled mid-stream.

**What it tests:**
- Stream starts and produces chunks
- Context cancelled after receiving 2 chunks
- Channel closes within 1 second
- No goroutines leak

### Goroutine Leak Detection
```bash
go test -v -run TestStreamingNoLeak
```
Uses `runtime.NumGoroutine()` to detect goroutine leaks across multiple stream cycles.

**What it tests:**
- Creates 10 streaming requests
- Abandons them with rapid cancellations
- Measures goroutine count before/after
- Asserts: leaked ≤ 3 (tolerance for background workers)

### Multiple Cancellations
```bash
go test -v -run TestStreamingMultipleCancellations
```
Tests rapid cancellation/creation cycles (worst case scenario).

**What it tests:**
- 20 iterations of: create stream → immediately cancel
- Verifies no deadlocks occur
- Verifies channels close promptly

### Interrupt Handling
```bash
go test -v -run TestStreamingInterrupt
```
Verifies that `Interrupt()` method cancels active streams.

**What it tests:**
- Stream is active
- `Interrupt()` called
- Stream receives error chunk or closes
- Cancel function is cleared

## Benchmarks

### Throughput
```bash
go test -bench=BenchmarkStreamingThroughput -benchmem
```
Measures tokens/second throughput. Ensures the fix doesn't regress performance.

### Cancellation Overhead
```bash
go test -bench=BenchmarkStreamingCancellation -benchmem
```
Measures the overhead of the context-aware channel send pattern.

### Concurrent Streaming
```bash
go test -bench=BenchmarkStreamingConcurrent -benchmem
```
Measures performance with multiple concurrent streams.

## Expected Results

All tests should pass with:
- ✅ 0 goroutine leaks (or ≤ 3 for background workers)
- ✅ Channel closes within 1 second of cancellation
- ✅ No deadlocks
- ✅ No test timeouts

## Test Patterns

### Goroutine Leak Detection Pattern
```go
runtime.GC()
time.Sleep(100 * time.Millisecond)
baseline := runtime.NumGoroutine()

// ... create and abandon streams ...

runtime.GC()
time.Sleep(200 * time.Millisecond)
final := runtime.NumGoroutine()
leaked := final - baseline

assert.LessOrEqual(t, leaked, 3)
```

### Context Cancellation Pattern
```go
ctx, cancel := context.WithCancel(context.Background())
chunkCh, err := o.ProcessStream(ctx, req)

// Read some chunks
cancel() // Cancel mid-stream

// Verify channel closes within timeout
select {
case _, ok := <-chunkCh:
    if !ok { /* success */ }
case <-time.After(1 * time.Second):
    t.Error("timeout")
}
```

## Troubleshooting

### Test hangs/times out
- Indicates a deadlock or goroutine that doesn't respect context cancellation
- Check that all channel sends use the `select { case <-ctx.Done() }` pattern

### Goroutine leak detection fails
- Some background goroutines from test infrastructure are expected
- Acceptable leak: ≤ 3 goroutines
- If leaked > 10, investigate the code for missing context checks

### Benchmark shows regression
- Compare with baseline: ~500µs per request
- If > 1ms per request, investigate performance impact

## Related Files

- Implementation: `/Users/normanking/ServerProjectsMac/cortex-brain-main/internal/orchestrator/streaming.go`
- Ollama tests: `/Users/normanking/ServerProjectsMac/cortex-brain-main/internal/llm/ollama_test.go`
- Documentation: `/Users/normanking/ServerProjectsMac/cortex-brain-main/docs/TEST_STREAMING_GOROUTINE_FIX.md`
