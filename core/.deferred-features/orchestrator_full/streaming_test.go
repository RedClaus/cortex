package orchestrator

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStreamingContextCancellation verifies that streaming goroutines exit cleanly when context is cancelled.
// This tests the fix in streaming.go lines 486-492 (context-aware channel send).
func TestStreamingContextCancellation(t *testing.T) {
	o := New()

	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Create a simple request
	req := &Request{
		Type:  RequestChat,
		Input: "test query",
	}

	// Start streaming
	chunkCh, err := o.ProcessStream(ctx, req)
	require.NoError(t, err, "ProcessStream should not error")
	require.NotNil(t, chunkCh, "chunk channel should be created")

	// Read a few chunks to ensure stream started
	chunksReceived := 0
	timeout := time.After(2 * time.Second)

readLoop:
	for {
		select {
		case chunk, ok := <-chunkCh:
			if !ok {
				// Channel closed normally - this is also valid
				break readLoop
			}
			chunksReceived++
			if chunksReceived >= 2 || chunk.IsFinal {
				// Cancel mid-stream after receiving some chunks
				cancel()
				break readLoop
			}
		case <-timeout:
			t.Fatal("Timeout waiting for initial chunks")
		}
	}

	// Drain remaining chunks with a timeout
	drainStart := time.Now()
	drainTimeout := time.After(1 * time.Second)

drainLoop:
	for {
		select {
		case _, ok := <-chunkCh:
			if !ok {
				// Channel closed - goroutine exited cleanly
				break drainLoop
			}
			// Continue draining
		case <-drainTimeout:
			t.Error("Channel was not closed within timeout after context cancellation")
			break drainLoop
		}
	}

	drainDuration := time.Since(drainStart)
	t.Logf("Channel closed after %v from cancellation", drainDuration)

	// Verify channel is closed
	_, stillOpen := <-chunkCh
	assert.False(t, stillOpen, "chunk channel should be closed after cancellation")
}

// TestStreamingNoLeak verifies that streaming goroutines don't leak.
// Uses runtime.NumGoroutine() to detect goroutine leaks.
func TestStreamingNoLeak(t *testing.T) {
	o := New()

	// Force garbage collection and wait for goroutines to settle
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	// Get baseline goroutine count
	baseline := runtime.NumGoroutine()
	t.Logf("Baseline goroutine count: %d", baseline)

	// Create multiple streams and abandon them (simulate rapid cancellations)
	const numStreams = 10
	var wg sync.WaitGroup

	for i := 0; i < numStreams; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			req := &Request{
				Type:  RequestChat,
				Input: "test query",
			}

			chunkCh, err := o.ProcessStream(ctx, req)
			if err != nil {
				t.Logf("Stream %d: ProcessStream error: %v", idx, err)
				return
			}

			// Read a few chunks then abandon
			count := 0
			for chunk := range chunkCh {
				count++
				if count >= 2 || chunk.IsFinal {
					// Cancel and abandon stream
					cancel()
					break
				}
			}

			// Drain remaining chunks
			for range chunkCh {
				// Drain
			}
		}(i)
	}

	// Wait for all streams to complete
	wg.Wait()

	// Force garbage collection
	runtime.GC()
	time.Sleep(200 * time.Millisecond)

	// Check goroutine count
	final := runtime.NumGoroutine()
	leaked := final - baseline

	t.Logf("Final goroutine count: %d (leaked: %d)", final, leaked)

	// Allow some tolerance for background goroutines (test framework, GC, etc.)
	// but we should not have leaked all numStreams goroutines
	maxAcceptableLeak := 3
	assert.LessOrEqual(t, leaked, maxAcceptableLeak,
		"Should not leak more than %d goroutines (leaked %d)", maxAcceptableLeak, leaked)
}

// TestStreamingMultipleCancellations tests rapid cancellation/creation cycles.
func TestStreamingMultipleCancellations(t *testing.T) {
	o := New()

	const iterations = 20

	for i := 0; i < iterations; i++ {
		ctx, cancel := context.WithCancel(context.Background())

		req := &Request{
			Type:  RequestChat,
			Input: "test query",
		}

		chunkCh, err := o.ProcessStream(ctx, req)
		require.NoError(t, err)

		// Immediately cancel (worst case)
		cancel()

		// Drain channel with timeout
		timeout := time.After(500 * time.Millisecond)
	drainLoop:
		for {
			select {
			case _, ok := <-chunkCh:
				if !ok {
					break drainLoop
				}
			case <-timeout:
				t.Fatalf("Iteration %d: channel not closed after cancellation", i)
			}
		}
	}

	// All iterations should complete without deadlock
	t.Logf("Successfully completed %d cancellation cycles", iterations)
}

// TestStreamingChannelClose verifies that channel is closed when stream completes normally.
func TestStreamingChannelClose(t *testing.T) {
	o := New()

	ctx := context.Background()
	req := &Request{
		Type:  RequestChat,
		Input: "test query",
	}

	chunkCh, err := o.ProcessStream(ctx, req)
	require.NoError(t, err)

	// Read all chunks until channel closes
	timeout := time.After(10 * time.Second)
	receivedFinal := false

	for {
		select {
		case chunk, ok := <-chunkCh:
			if !ok {
				// Channel closed
				if !receivedFinal {
					t.Error("Channel closed without receiving final chunk")
				}
				return
			}
			if chunk.IsFinal {
				receivedFinal = true
			}
		case <-timeout:
			t.Fatal("Timeout waiting for stream completion")
		}
	}
}

// TestStreamingInterrupt verifies that Interrupt() cancels active streams.
func TestStreamingInterrupt(t *testing.T) {
	o := New()

	ctx := context.Background()
	req := &Request{
		Type:  RequestChat,
		Input: "test query",
	}

	chunkCh, err := o.ProcessStream(ctx, req)
	require.NoError(t, err)

	// Read a few chunks
	chunksReceived := 0
	timeout := time.After(2 * time.Second)

readLoop:
	for {
		select {
		case chunk, ok := <-chunkCh:
			if !ok {
				t.Error("Channel closed before interrupt")
				return
			}
			chunksReceived++
			if chunksReceived >= 2 || chunk.IsFinal {
				// Trigger interrupt
				err := o.Interrupt("test_interrupt")
				require.NoError(t, err)
				break readLoop
			}
		case <-timeout:
			t.Fatal("Timeout waiting for chunks")
		}
	}

	// Verify stream was interrupted (channel should close or receive error chunk)
	timeout = time.After(1 * time.Second)
	gotErrorOrClose := false

	for {
		select {
		case chunk, ok := <-chunkCh:
			if !ok {
				// Channel closed due to interrupt
				gotErrorOrClose = true
				goto done
			}
			if chunk.Type == StreamChunkError {
				gotErrorOrClose = true
				goto done
			}
		case <-timeout:
			t.Error("Stream was not interrupted within timeout")
			goto done
		}
	}

done:
	assert.True(t, gotErrorOrClose, "Stream should be interrupted (error chunk or close)")
}

// TestStreamingConcurrentStreams verifies multiple concurrent streams work correctly.
func TestStreamingConcurrentStreams(t *testing.T) {
	o := New()

	const numConcurrent = 5
	var wg sync.WaitGroup
	errors := make(chan error, numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			req := &Request{
				Type:  RequestChat,
				Input: "test query",
			}

			chunkCh, err := o.ProcessStream(ctx, req)
			if err != nil {
				errors <- err
				return
			}

			// Read all chunks
			count := 0
			for chunk := range chunkCh {
				count++
				if chunk.IsFinal {
					break
				}
			}

			if count == 0 {
				errors <- assert.AnError
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent stream error: %v", err)
	}
}

// TestSalamanderOrchestratorContextCancellation verifies Salamander adapter handles cancellation.
// This tests the fix in streaming.go lines 486-492 (context-aware send in conversion goroutine).
func TestSalamanderOrchestratorContextCancellation(t *testing.T) {
	o := New()
	salamander := NewSalamanderOrchestrator(o)

	ctx, cancel := context.WithCancel(context.Background())

	req := &DirectRequest{
		ID:        "test-123",
		Input:     "test query",
		TaskID:    "task-1",
		ContextID: "ctx-1",
	}

	respCh, err := salamander.ProcessStream(ctx, req)
	require.NoError(t, err)

	// Read a few responses
	responsesReceived := 0
	timeout := time.After(2 * time.Second)

readLoop:
	for {
		select {
		case resp, ok := <-respCh:
			if !ok {
				break readLoop
			}
			responsesReceived++
			if responsesReceived >= 2 || resp.IsFinal {
				// Cancel mid-stream
				cancel()
				break readLoop
			}
		case <-timeout:
			t.Fatal("Timeout waiting for initial responses")
		}
	}

	// Drain with timeout
	drainTimeout := time.After(1 * time.Second)
drainLoop:
	for {
		select {
		case _, ok := <-respCh:
			if !ok {
				break drainLoop
			}
		case <-drainTimeout:
			t.Error("Response channel was not closed after cancellation")
			break drainLoop
		}
	}

	// Verify channel is closed
	_, stillOpen := <-respCh
	assert.False(t, stillOpen, "response channel should be closed")
}

// BenchmarkStreamingThroughput measures tokens/second throughput.
// Ensures the fix doesn't regress performance.
func BenchmarkStreamingThroughput(b *testing.B) {
	o := New()
	ctx := context.Background()

	req := &Request{
		Type:  RequestChat,
		Input: "test query",
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		chunkCh, err := o.ProcessStream(ctx, req)
		if err != nil {
			b.Fatalf("ProcessStream error: %v", err)
		}

		// Consume all chunks
		for range chunkCh {
			// Count chunks
		}
	}
}

// BenchmarkStreamingCancellation measures cancellation overhead.
func BenchmarkStreamingCancellation(b *testing.B) {
	o := New()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithCancel(context.Background())

		req := &Request{
			Type:  RequestChat,
			Input: "test query",
		}

		chunkCh, err := o.ProcessStream(ctx, req)
		if err != nil {
			b.Fatalf("ProcessStream error: %v", err)
		}

		// Read one chunk then cancel
		<-chunkCh
		cancel()

		// Drain
		for range chunkCh {
		}
	}
}

// BenchmarkStreamingConcurrent measures concurrent streaming performance.
func BenchmarkStreamingConcurrent(b *testing.B) {
	o := New()
	ctx := context.Background()

	req := &Request{
		Type:  RequestChat,
		Input: "test query",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			chunkCh, err := o.ProcessStream(ctx, req)
			if err != nil {
				b.Errorf("ProcessStream error: %v", err)
				continue
			}

			// Consume all chunks
			for range chunkCh {
			}
		}
	})
}
