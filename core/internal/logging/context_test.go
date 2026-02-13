package logging

import (
	"context"
	"testing"
	"time"
)

func TestDetachContext_SurvivesCancellation(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	detached := DetachContext(parent)

	cancel() // Cancel parent

	if parent.Err() == nil {
		t.Error("parent should be cancelled")
	}
	if detached.Err() != nil {
		t.Errorf("detached should survive cancellation, got error: %v", detached.Err())
	}
}

func TestDetachContextWithTimeout_SurvivesParentCancellation(t *testing.T) {
	parent, parentCancel := context.WithCancel(context.Background())
	detached, detachedCancel := DetachContextWithTimeout(parent, 100*time.Millisecond)
	defer detachedCancel()

	parentCancel() // Cancel parent immediately

	// Parent should be cancelled
	if parent.Err() == nil {
		t.Error("parent should be cancelled")
	}

	// Detached should NOT be cancelled yet (it has its own timeout)
	if detached.Err() != nil {
		t.Errorf("detached should not be cancelled yet, got error: %v", detached.Err())
	}

	// Wait for detached timeout
	time.Sleep(150 * time.Millisecond)

	// Now detached should be cancelled due to its own timeout
	if detached.Err() != context.DeadlineExceeded {
		t.Errorf("detached should have deadline exceeded error, got: %v", detached.Err())
	}
}

func TestDetachContextWithTimeout_HasOwnDeadline(t *testing.T) {
	parent := context.Background()
	timeout := 50 * time.Millisecond
	detached, cancel := DetachContextWithTimeout(parent, timeout)
	defer cancel()

	// Check that detached has a deadline
	deadline, ok := detached.Deadline()
	if !ok {
		t.Error("detached context should have a deadline")
	}

	// Deadline should be approximately timeout from now
	expectedDeadline := time.Now().Add(timeout)
	diff := deadline.Sub(expectedDeadline)
	if diff < -10*time.Millisecond || diff > 10*time.Millisecond {
		t.Errorf("deadline should be ~%v from now, got diff: %v", timeout, diff)
	}

	// Wait for timeout
	<-detached.Done()

	if detached.Err() != context.DeadlineExceeded {
		t.Errorf("expected deadline exceeded, got: %v", detached.Err())
	}
}

func TestDetachContext_PreservesValues(t *testing.T) {
	type key string
	testKey := key("test")
	testValue := "value"

	parent := context.WithValue(context.Background(), testKey, testValue)
	detached := DetachContext(parent)

	// Values should be preserved
	if v := detached.Value(testKey); v != testValue {
		t.Errorf("expected value %v, got %v", testValue, v)
	}
}
