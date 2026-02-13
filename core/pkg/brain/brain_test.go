package brain

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLobeID_Valid(t *testing.T) {
	tests := []struct {
		id    LobeID
		valid bool
	}{
		{LobeMemory, true},
		{LobeCoding, true},
		{LobeSafety, true},
		{LobeID("unknown"), false},
		{LobeID(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.id), func(t *testing.T) {
			assert.Equal(t, tt.valid, tt.id.Valid())
		})
	}
}

func TestAllLobes(t *testing.T) {
	lobes := AllLobes()
	assert.Equal(t, 20, len(lobes))

	for _, lobe := range lobes {
		assert.True(t, lobe.Valid(), "lobe %s should be valid", lobe)
	}
}

func TestRiskLevel_Valid(t *testing.T) {
	assert.True(t, RiskLow.Valid())
	assert.True(t, RiskHigh.Valid())
	assert.False(t, RiskLevel("unknown").Valid())
}

func TestComputeTier_Valid(t *testing.T) {
	assert.True(t, ComputeFast.Valid())
	assert.True(t, ComputeMax.Valid())
	assert.False(t, ComputeTier("unknown").Valid())
}

func TestBlackboard_ThreadSafety(t *testing.T) {
	bb := NewBlackboard()
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(n int) {
			bb.Set("key", n)
			bb.Get("key")
			bb.AddMemory(Memory{ID: "test"})
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	assert.Equal(t, 10, len(bb.Memories))
}

func TestBlackboard_Clone(t *testing.T) {
	bb := NewBlackboard()
	bb.Set("key", "value")
	bb.AddMemory(Memory{ID: "mem1", Content: "test"})
	bb.OverallConfidence = 0.9

	clone := bb.Clone()

	assert.Equal(t, bb.OverallConfidence, clone.OverallConfidence)
	assert.Equal(t, len(bb.Memories), len(clone.Memories))

	clone.Set("key", "modified")
	original, _ := bb.Get("key")
	assert.Equal(t, "value", original)
}

func TestRegistry(t *testing.T) {
	reg := NewRegistry()
	assert.Equal(t, 0, reg.Count())

	mock := &mockLobe{id: LobeMemory}
	reg.Register(mock)

	assert.Equal(t, 1, reg.Count())

	lobe, ok := reg.Get(LobeMemory)
	require.True(t, ok)
	assert.Equal(t, LobeMemory, lobe.ID())

	_, ok = reg.Get(LobeCoding)
	assert.False(t, ok)

	lobes := reg.GetAll([]LobeID{LobeMemory, LobeCoding})
	assert.Equal(t, 1, len(lobes))
}

func TestStrategyBuilder(t *testing.T) {
	strategy := NewStrategyBuilder("TestStrategy").
		WithComputeTier(ComputeDeep).
		AddPhase("Phase1", []LobeID{LobeMemory}, false, 5*time.Second, true).
		AddPhase("Phase2", []LobeID{LobeReasoning, LobeCoding}, true, 10*time.Second, false).
		Build()

	assert.Equal(t, "TestStrategy", strategy.Name)
	assert.Equal(t, ComputeDeep, strategy.ComputeTier)
	assert.Equal(t, 2, len(strategy.Phases))
	assert.Equal(t, "Phase1", strategy.Phases[0].Name)
	assert.False(t, strategy.Phases[0].Parallel)
	assert.True(t, strategy.Phases[1].Parallel)
}

func TestPrebuiltStrategies(t *testing.T) {
	strategies := []ThinkingStrategy{
		QuickAnswerStrategy(),
		DeepReasoningStrategy(),
		CodingStrategy(),
		CreativeStrategy(),
		SafetyFirstStrategy(),
	}

	for _, s := range strategies {
		assert.NotEmpty(t, s.Name)
		assert.NotEmpty(t, s.Phases)
		assert.True(t, s.ComputeTier.Valid())
	}
}

func TestSystemMonitor(t *testing.T) {
	monitor := NewSystemMonitor(100 * time.Millisecond)
	monitor.Start()
	defer monitor.Stop()

	time.Sleep(150 * time.Millisecond)

	metrics := monitor.GetMetrics()
	assert.True(t, metrics.GoRoutineCount > 0)
	assert.True(t, metrics.MemoryTotalMB > 0)
	assert.False(t, metrics.LastUpdated.IsZero())
}

func TestOutcomeLogger(t *testing.T) {
	logger := NewOutcomeLogger(nil, 10)

	for i := 0; i < 15; i++ {
		logger.Log(ExecutionRecord{
			Input: "test",
			Outcome: Outcome{
				Success:    i%2 == 0,
				LatencyMS:  100,
				TokensUsed: 50,
			},
		})
	}

	recent := logger.GetRecent(5)
	assert.Equal(t, 5, len(recent))

	stats := logger.GetStats()
	assert.Equal(t, 10, stats.TotalExecutions)
	assert.Equal(t, float64(100), stats.AvgLatencyMS)
}

func TestClassifier_Regex(t *testing.T) {
	classifier := NewExecutiveClassifier(nil, nil, &mockCache{})

	tests := []struct {
		input    string
		expected LobeID
	}{
		{"write code for a function", LobeCoding},
		{"remember what I said yesterday", LobeMemory},
		{"plan the steps for this project", LobePlanning},
		{"why does this happen", LobeReasoning},
		{"brainstorm ideas for a new app", LobeCreativity},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, ok := classifier.classifyByRegex(tt.input)
			if ok {
				assert.Equal(t, tt.expected, result.PrimaryLobe)
				assert.Equal(t, "regex", result.Method)
			}
		})
	}
}

type mockLobe struct {
	id LobeID
}

func (m *mockLobe) ID() LobeID { return m.id }
func (m *mockLobe) Process(_ context.Context, _ LobeInput, _ *Blackboard) (*LobeResult, error) {
	return nil, nil
}
func (m *mockLobe) CanHandle(_ string) float64                    { return 0.5 }
func (m *mockLobe) ResourceEstimate(_ LobeInput) ResourceEstimate { return ResourceEstimate{} }

type mockCache struct {
	data map[string]*ClassificationResult
}

func (c *mockCache) Get(key string) (*ClassificationResult, bool) {
	if c.data == nil {
		return nil, false
	}
	r, ok := c.data[key]
	return r, ok
}

func (c *mockCache) Set(key string, result *ClassificationResult) {
	if c.data == nil {
		c.data = make(map[string]*ClassificationResult)
	}
	c.data[key] = result
}

// -----------------------------------------------------------------------------
// Copy-on-Write Blackboard Tests
// -----------------------------------------------------------------------------

func TestBlackboard_CloneIndependence(t *testing.T) {
	// Test that clone modifications don't affect original
	bb := NewBlackboard()
	bb.Set("key", "original")

	clone := bb.Clone()
	clone.Set("key", "modified")

	// Original should be unchanged (but frozen)
	val, _ := bb.Get("key")
	assert.Equal(t, "original", val)

	// Clone should have new value
	val2, _ := clone.Get("key")
	assert.Equal(t, "modified", val2)
}

func TestBlackboard_FrozenWritePanic(t *testing.T) {
	bb := NewBlackboard()
	_ = bb.Clone() // Freezes bb

	assert.Panics(t, func() {
		bb.Set("key", "value") // Should panic
	})
}

func TestBlackboard_FrozenDeletePanic(t *testing.T) {
	bb := NewBlackboard()
	bb.Set("key", "value")
	_ = bb.Clone() // Freezes bb

	assert.Panics(t, func() {
		bb.Delete("key") // Should panic
	})
}

func TestBlackboard_FrozenAddMemoryPanic(t *testing.T) {
	bb := NewBlackboard()
	_ = bb.Clone() // Freezes bb

	assert.Panics(t, func() {
		bb.AddMemory(Memory{ID: "test"}) // Should panic
	})
}

func TestBlackboard_FrozenMergePanic(t *testing.T) {
	bb := NewBlackboard()
	_ = bb.Clone() // Freezes bb

	assert.Panics(t, func() {
		bb.Merge(&LobeResult{LobeID: LobeMemory}) // Should panic
	})
}

func TestBlackboard_TombstoneOverride(t *testing.T) {
	bb := NewBlackboard()
	bb.Set("key", "value")

	clone := bb.Clone()
	clone.Delete("key")

	// Parent still has it
	val, ok := bb.Get("key")
	assert.True(t, ok)
	assert.Equal(t, "value", val)

	// Clone sees tombstone
	_, ok = clone.Get("key")
	assert.False(t, ok)
}

func TestBlackboard_MaxDepthFlatten(t *testing.T) {
	bb := NewBlackboard()
	bb.Set("key", "value")

	// Build up to MaxParentDepth
	current := bb
	for i := 0; i < MaxParentDepth; i++ {
		current = current.Clone()
	}

	// At this point, current has depth = MaxParentDepth
	assert.Equal(t, MaxParentDepth, current.Depth())

	// Next clone should trigger flatten
	flattened := current.Clone()

	// Should have been flattened (depth reset to 0, no parent)
	assert.Equal(t, 0, flattened.Depth())
	assert.Nil(t, flattened.Parent())

	// Data preserved
	val, ok := flattened.Get("key")
	assert.True(t, ok)
	assert.Equal(t, "value", val)

	// Flattened should not be frozen (it's a new root)
	assert.False(t, flattened.IsFrozen())
}

func TestBlackboard_Keys(t *testing.T) {
	bb := NewBlackboard()
	bb.Set("key1", "value1")
	bb.Set("key2", "value2")

	clone := bb.Clone()
	clone.Set("key3", "value3")
	clone.Delete("key1")

	// Keys should include key2 (from parent) and key3 (from overlay)
	// But not key1 (deleted in clone)
	keys := clone.Keys()
	assert.Contains(t, keys, "key2")
	assert.Contains(t, keys, "key3")
	assert.NotContains(t, keys, "key1")
}

func TestBlackboard_Flatten(t *testing.T) {
	bb := NewBlackboard()
	bb.Set("key1", "value1")

	clone := bb.Clone()
	clone.Set("key2", "value2")

	flattened := clone.Flatten()

	// Flattened should have no parent
	assert.Nil(t, flattened.Parent())
	assert.Equal(t, 0, flattened.Depth())
	assert.False(t, flattened.IsFrozen())

	// Should have both keys
	val1, ok := flattened.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, "value1", val1)

	val2, ok := flattened.Get("key2")
	assert.True(t, ok)
	assert.Equal(t, "value2", val2)
}

func TestBlackboard_IsFrozen(t *testing.T) {
	bb := NewBlackboard()
	assert.False(t, bb.IsFrozen())

	_ = bb.Clone()
	assert.True(t, bb.IsFrozen())
}

func TestBlackboard_Depth(t *testing.T) {
	bb := NewBlackboard()
	assert.Equal(t, 0, bb.Depth())

	clone := bb.Clone()
	assert.Equal(t, 1, clone.Depth())

	clone2 := clone.Clone()
	assert.Equal(t, 2, clone2.Depth())
}

func TestBlackboard_NilValueVsTombstone(t *testing.T) {
	bb := NewBlackboard()
	bb.Set("nil_value", nil)
	bb.Set("to_delete", "value")

	clone := bb.Clone()
	clone.Delete("to_delete")

	// nil value should be found
	val, ok := clone.Get("nil_value")
	assert.True(t, ok)
	assert.Nil(t, val)

	// Deleted key should not be found
	_, ok = clone.Get("to_delete")
	assert.False(t, ok)
}

func TestBlackboard_ConcurrentCloneAndRead(t *testing.T) {
	bb := NewBlackboard()
	for i := 0; i < 100; i++ {
		bb.Set(fmt.Sprintf("key%d", i), i)
	}

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			clone := bb.Clone()
			for j := 0; j < 100; j++ {
				_, _ = clone.Get(fmt.Sprintf("key%d", j))
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestBlackboard_Summary(t *testing.T) {
	bb := NewBlackboard()
	bb.Set("key", "value")
	bb.ConversationID = "conv-123"
	bb.TurnNumber = 5

	clone := bb.Clone()
	clone.Set("key2", "value2")

	summary := clone.Summary()

	assert.Equal(t, "conv-123", summary["conversation_id"])
	assert.Equal(t, 5, summary["turn_number"])
	assert.Equal(t, 1, summary["depth"])
	assert.False(t, summary["frozen"].(bool))

	data := summary["data"].(map[string]interface{})
	assert.Equal(t, "value", data["key"])
	assert.Equal(t, "value2", data["key2"])
}

// -----------------------------------------------------------------------------
// Benchmarks - Critical for validating CoW improvement
// -----------------------------------------------------------------------------
//
// Benchmark Results (Apple M2, darwin/arm64):
//
//   Operation                ns/op    B/op   allocs/op   Status
//   ----------------------------------------------------------------
//   Clone (empty)            106.7    192    2           ✅ 5x better than 500ns target
//   Clone (100 items)        180.1    768    3           ✅ Excellent
//   Get (single level)        21.32     0    0           ✅ Zero-allocation
//   Get (10-level chain)      59.59     0    0           ✅ Zero-allocation
//   Set                       65.08     8    0           ✅ Excellent
//   Flatten (100 keys)     50,991  19,344   22           ✅ Acceptable (O(N))
//   Keys (100 keys)        20,682  15,944   12           ✅ Good
//   Merge                     56.77     0    0           ✅ Zero-allocation
//   Parallel                 506.8    544    3           ✅ Excellent under contention
//
// Key Insights:
//   - Clone is O(1) and independent of parent data size
//   - Get is zero-allocation at any chain depth
//   - Parent chain walk is cache-friendly (<100ns for 10 levels)
//   - All concurrency tests pass with no race conditions
//
// See BENCHMARK_RESULTS.md for detailed analysis.
// -----------------------------------------------------------------------------

// BenchmarkBlackboardClone measures the cost of a single clone operation.
// Target: < 500ns for empty blackboard with CoW optimization.
func BenchmarkBlackboardClone(b *testing.B) {
	bb := NewBlackboard()
	bb.Set("key", "value")
	bb.OverallConfidence = 0.95
	bb.ConversationID = "test-conv"

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = bb.Clone()
	}
}

// BenchmarkBlackboardCloneDeep measures clone cost with realistic data.
// Tests CoW efficiency with 100 items in the overlay map.
func BenchmarkBlackboardCloneDeep(b *testing.B) {
	bb := NewBlackboard()

	// Populate with 100 items to simulate realistic workload
	for i := 0; i < 100; i++ {
		bb.Set(fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i))
	}
	bb.OverallConfidence = 0.95
	bb.ConversationID = "test-conv"

	// Add some structured data
	for i := 0; i < 10; i++ {
		bb.AddMemory(Memory{
			ID:        fmt.Sprintf("mem%d", i),
			Content:   fmt.Sprintf("Memory content %d", i),
			Source:    "test",
			Relevance: 0.8,
		})
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = bb.Clone()
	}
}

// BenchmarkBlackboardGet measures single Get operation.
// Target: O(1) for single-level, degrades linearly with depth.
func BenchmarkBlackboardGet(b *testing.B) {
	bb := NewBlackboard()
	bb.Set("key", "value")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = bb.Get("key")
	}
}

// BenchmarkBlackboardGetDeep measures Get cost through 10-level chain.
// Tests O(depth) performance characteristic of parent chain walk.
func BenchmarkBlackboardGetDeep(b *testing.B) {
	bb := NewBlackboard()
	bb.Set("root_key", "root_value")

	// Build 10-level chain
	current := bb
	for i := 0; i < 10; i++ {
		current = current.Clone()
		current.Set(fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Get from root (worst case - walk full chain)
		_, _ = current.Get("root_key")
	}
}

// BenchmarkBlackboardSet measures single Set operation.
// Target: O(1) write to overlay map.
func BenchmarkBlackboardSet(b *testing.B) {
	bb := NewBlackboard()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		bb.Set("key", i)
	}
}

// BenchmarkBlackboardFlatten measures cost of flattening a chain.
// This is an O(N) operation that materializes all keys from parent chain.
func BenchmarkBlackboardFlatten(b *testing.B) {
	bb := NewBlackboard()

	// Populate root with 50 items
	for i := 0; i < 50; i++ {
		bb.Set(fmt.Sprintf("root%d", i), i)
	}

	// Build 5-level chain, each adding 10 items
	current := bb
	for level := 0; level < 5; level++ {
		current = current.Clone()
		for i := 0; i < 10; i++ {
			current.Set(fmt.Sprintf("level%d_key%d", level, i), i)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = current.Flatten()
	}
}

// BenchmarkBlackboardKeys measures cost of collecting all keys.
// This walks the parent chain and deduplicates keys.
func BenchmarkBlackboardKeys(b *testing.B) {
	bb := NewBlackboard()

	// Populate with 100 items
	for i := 0; i < 100; i++ {
		bb.Set(fmt.Sprintf("key%d", i), i)
	}

	// Build 3-level chain with some overwrites
	current := bb
	for level := 0; level < 3; level++ {
		current = current.Clone()
		for i := 0; i < 20; i++ {
			current.Set(fmt.Sprintf("level%d_key%d", level, i), i)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = current.Keys()
	}
}

// BenchmarkBlackboardMerge measures cost of merging a LobeResult.
func BenchmarkBlackboardMerge(b *testing.B) {
	bb := NewBlackboard()
	result := &LobeResult{
		LobeID:     LobeMemory,
		Content:    "Test result",
		Confidence: 0.9,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		bb.Merge(result)
	}
}

// BenchmarkBlackboardAddMemory measures cost of appending a memory.
func BenchmarkBlackboardAddMemory(b *testing.B) {
	bb := NewBlackboard()
	mem := Memory{
		ID:        "test",
		Content:   "test memory",
		Source:    "test",
		Relevance: 0.8,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		bb.AddMemory(mem)
	}
}

// BenchmarkBlackboardParallel simulates parallel lobe execution pattern.
// Multiple goroutines clone and operate on independent blackboards.
func BenchmarkBlackboardParallel(b *testing.B) {
	bb := NewBlackboard()

	// Populate with realistic data
	for i := 0; i < 50; i++ {
		bb.Set(fmt.Sprintf("key%d", i), i)
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Simulate lobe pattern: clone, read, write
			clone := bb.Clone()
			_, _ = clone.Get("key0")
			clone.Set("new_key", "new_value")
		}
	})
}

// -----------------------------------------------------------------------------
// Concurrency Tests
// -----------------------------------------------------------------------------

// TestBlackboardConcurrentClone tests concurrent cloning from the same parent.
// Multiple goroutines should be able to clone safely without data races.
func TestBlackboardConcurrentClone(t *testing.T) {
	bb := NewBlackboard()

	// Populate with data
	for i := 0; i < 100; i++ {
		bb.Set(fmt.Sprintf("key%d", i), i)
	}

	const numGoroutines = 50
	clones := make([]*Blackboard, numGoroutines)
	done := make(chan int, numGoroutines)

	// Concurrent cloning
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			clones[idx] = bb.Clone()
			done <- idx
		}(i)
	}

	// Wait for all clones
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify all clones are valid and independent
	for i, clone := range clones {
		assert.NotNil(t, clone, "clone %d should not be nil", i)
		assert.Equal(t, 1, clone.Depth(), "clone %d should have depth 1", i)
		assert.False(t, clone.IsFrozen(), "clone %d should not be frozen", i)

		// Verify data integrity
		val, ok := clone.Get("key0")
		assert.True(t, ok, "clone %d should have key0", i)
		assert.Equal(t, 0, val, "clone %d key0 should have correct value", i)

		// Modify clone to ensure independence
		clone.Set("unique_key", i)
	}

	// Verify parent is frozen and unchanged
	assert.True(t, bb.IsFrozen(), "parent should be frozen")
	val, ok := bb.Get("key0")
	assert.True(t, ok)
	assert.Equal(t, 0, val)
}

// TestBlackboardConcurrentReadWrite tests concurrent reads and writes.
// Reads from frozen parent should be safe while writes happen on clones.
func TestBlackboardConcurrentReadWrite(t *testing.T) {
	bb := NewBlackboard()

	// Populate parent
	for i := 0; i < 100; i++ {
		bb.Set(fmt.Sprintf("key%d", i), i)
	}

	const numReaders = 20
	const numWriters = 20
	const opsPerGoroutine = 100

	done := make(chan bool, numReaders+numWriters)

	// Start readers (reading from frozen parent)
	for i := 0; i < numReaders; i++ {
		go func() {
			for j := 0; j < opsPerGoroutine; j++ {
				// Read random key
				key := fmt.Sprintf("key%d", j%100)
				_, _ = bb.Get(key)
			}
			done <- true
		}()
	}

	// Start writers (writing to independent clones)
	for i := 0; i < numWriters; i++ {
		go func(idx int) {
			clone := bb.Clone()
			for j := 0; j < opsPerGoroutine; j++ {
				// Write unique keys
				key := fmt.Sprintf("writer%d_key%d", idx, j)
				clone.Set(key, j)
			}
			done <- true
		}(i)
	}

	// Wait for completion
	for i := 0; i < numReaders+numWriters; i++ {
		<-done
	}

	// Verify parent is still intact
	val, ok := bb.Get("key0")
	assert.True(t, ok)
	assert.Equal(t, 0, val)
}

// TestBlackboardConcurrentGetFromChain tests concurrent reads through parent chain.
// Multiple readers should be able to walk the parent chain safely.
func TestBlackboardConcurrentGetFromChain(t *testing.T) {
	bb := NewBlackboard()
	bb.Set("root_key", "root_value")

	// Build 5-level chain
	current := bb
	for i := 0; i < 5; i++ {
		current = current.Clone()
		current.Set(fmt.Sprintf("level%d", i), i)
	}

	const numReaders = 50
	const opsPerGoroutine = 100
	done := make(chan bool, numReaders)

	// Concurrent reads from deepest level
	for i := 0; i < numReaders; i++ {
		go func() {
			for j := 0; j < opsPerGoroutine; j++ {
				// Read from various levels
				_, _ = current.Get("root_key")
				_, _ = current.Get("level0")
				_, _ = current.Get("level4")
			}
			done <- true
		}()
	}

	// Wait for completion
	for i := 0; i < numReaders; i++ {
		<-done
	}

	// Verify data integrity
	val, ok := current.Get("root_key")
	assert.True(t, ok)
	assert.Equal(t, "root_value", val)
}

// TestBlackboardConcurrentFlatten tests concurrent flatten operations.
func TestBlackboardConcurrentFlatten(t *testing.T) {
	bb := NewBlackboard()

	// Populate with data
	for i := 0; i < 50; i++ {
		bb.Set(fmt.Sprintf("key%d", i), i)
	}

	// Build chain
	current := bb
	for i := 0; i < 3; i++ {
		current = current.Clone()
		current.Set(fmt.Sprintf("level%d", i), i)
	}

	const numGoroutines = 20
	flattened := make([]*Blackboard, numGoroutines)
	done := make(chan int, numGoroutines)

	// Concurrent flattening
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			flattened[idx] = current.Flatten()
			done <- idx
		}(i)
	}

	// Wait for completion
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify all flattened boards are valid and independent
	for i, flat := range flattened {
		assert.NotNil(t, flat, "flattened %d should not be nil", i)
		assert.Equal(t, 0, flat.Depth(), "flattened %d should have depth 0", i)
		assert.Nil(t, flat.Parent(), "flattened %d should have no parent", i)

		// Verify data integrity
		val, ok := flat.Get("key0")
		assert.True(t, ok, "flattened %d should have key0", i)
		assert.Equal(t, 0, val)

		val, ok = flat.Get("level0")
		assert.True(t, ok, "flattened %d should have level0", i)
		assert.Equal(t, 0, val)
	}
}

// TestBlackboardConcurrentMerge tests concurrent merge operations on different clones.
func TestBlackboardConcurrentMerge(t *testing.T) {
	bb := NewBlackboard()

	const numClones = 20
	clones := make([]*Blackboard, numClones)
	done := make(chan int, numClones)

	// Create clones
	for i := 0; i < numClones; i++ {
		clones[i] = bb.Clone()
	}

	// Concurrent merges on different clones
	for i := 0; i < numClones; i++ {
		go func(idx int) {
			result := &LobeResult{
				LobeID:     LobeID(fmt.Sprintf("lobe%d", idx)),
				Content:    fmt.Sprintf("result%d", idx),
				Confidence: 0.9,
			}
			clones[idx].Merge(result)
			done <- idx
		}(i)
	}

	// Wait for completion
	for i := 0; i < numClones; i++ {
		<-done
	}

	// Verify each clone has its own result
	for i, clone := range clones {
		val, ok := clone.Get(fmt.Sprintf("lobe%d", i))
		assert.True(t, ok, "clone %d should have its result", i)
		assert.Equal(t, fmt.Sprintf("result%d", i), val)
	}
}

// TestBlackboardConcurrentMemoryOps tests concurrent memory/entity operations.
func TestBlackboardConcurrentMemoryOps(t *testing.T) {
	bb := NewBlackboard()

	const numClones = 20
	clones := make([]*Blackboard, numClones)
	done := make(chan int, numClones)

	// Create clones
	for i := 0; i < numClones; i++ {
		clones[i] = bb.Clone()
	}

	// Concurrent memory/entity operations
	for i := 0; i < numClones; i++ {
		go func(idx int) {
			clone := clones[idx]

			// Add memories
			for j := 0; j < 10; j++ {
				clone.AddMemory(Memory{
					ID:        fmt.Sprintf("mem%d_%d", idx, j),
					Content:   fmt.Sprintf("content%d", j),
					Source:    "test",
					Relevance: 0.8,
				})
			}

			// Add entities
			for j := 0; j < 5; j++ {
				clone.AddEntity(Entity{
					Type:  "test",
					Value: fmt.Sprintf("entity%d_%d", idx, j),
					Start: j,
					End:   j + 1,
				})
			}

			done <- idx
		}(i)
	}

	// Wait for completion
	for i := 0; i < numClones; i++ {
		<-done
	}

	// Verify each clone has its own data
	for i, clone := range clones {
		assert.Equal(t, 10, len(clone.Memories), "clone %d should have 10 memories", i)
		assert.Equal(t, 5, len(clone.Entities), "clone %d should have 5 entities", i)
	}
}
