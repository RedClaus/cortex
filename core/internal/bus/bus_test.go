package bus

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewBus(t *testing.T) {
	bus := NewBus()
	if bus == nil {
		t.Fatal("NewBus returned nil")
	}
	
	if bus.historySize != DefaultHistorySize {
		t.Errorf("Expected history size %d, got %d", DefaultHistorySize, bus.historySize)
	}
	
	bus.Close()
}

func TestNewBusWithConfig(t *testing.T) {
	bus := NewBusWithConfig(500)
	if bus.historySize != 500 {
		t.Errorf("Expected history size 500, got %d", bus.historySize)
	}
	bus.Close()
}

func TestSubscribeAndPublish(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	var received atomic.Bool
	done := make(chan bool, 1)

	handler := func(e Event) {
		if e.Type == EventLobeStart {
			received.Store(true)
			done <- true
		}
	}

	id := bus.Subscribe(EventLobeStart, handler)
	if id == "" {
		t.Fatal("Subscribe returned empty ID")
	}

	event := NewEvent(EventLobeStart)
	event.Lobe = "TestLobe"
	
	if err := bus.Publish(event); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	select {
	case <-done:
		if !received.Load() {
			t.Error("Handler was not called with correct event")
		}
	case <-time.After(time.Second):
		t.Error("Timeout waiting for event")
	}
}

func TestUnsubscribe(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	callCount := atomic.Int32{}
	
	handler := func(e Event) {
		callCount.Add(1)
	}

	id := bus.Subscribe(EventLobeStart, handler)
	
	// Publish and receive
	bus.Publish(NewEvent(EventLobeStart))
	time.Sleep(100 * time.Millisecond)
	
	// Unsubscribe
	if err := bus.Unsubscribe(id); err != nil {
		t.Fatalf("Unsubscribe failed: %v", err)
	}
	
	// Publish again
	bus.Publish(NewEvent(EventLobeStart))
	time.Sleep(100 * time.Millisecond)
	
	if callCount.Load() != 1 {
		t.Errorf("Expected 1 call, got %d", callCount.Load())
	}
}

func TestWildcardSubscription(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	callCount := atomic.Int32{}
	done := make(chan bool, 1)
	
	handler := func(e Event) {
		if callCount.Add(1) == 2 {
			done <- true
		}
	}

	bus.Subscribe(EventType(""), handler)
	
	bus.Publish(NewEvent(EventLobeStart))
	bus.Publish(NewEvent(EventLobeComplete))
	
	select {
	case <-done:
		if callCount.Load() != 2 {
			t.Errorf("Expected 2 calls, got %d", callCount.Load())
		}
	case <-time.After(time.Second):
		t.Error("Timeout waiting for events")
	}
}

func TestTypedAndWildcardSubscriptions(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	typedCount := atomic.Int32{}
	wildcardCount := atomic.Int32{}
	
	bus.Subscribe(EventLobeStart, func(e Event) {
		typedCount.Add(1)
	})
	
	bus.Subscribe(EventType(""), func(e Event) {
		wildcardCount.Add(1)
	})
	
	bus.Publish(NewEvent(EventLobeStart))
	time.Sleep(100 * time.Millisecond)
	
	if typedCount.Load() != 1 {
		t.Errorf("Typed subscriber expected 1 call, got %d", typedCount.Load())
	}
	if wildcardCount.Load() != 1 {
		t.Errorf("Wildcard subscriber expected 1 call, got %d", wildcardCount.Load())
	}
}

func TestEventHistory(t *testing.T) {
	bus := NewBusWithConfig(10)
	defer bus.Close()

	// Publish 5 events
	for i := 0; i < 5; i++ {
		event := NewEvent(EventLobeStart)
		event.Lobe = string(rune('A' + i))
		bus.Publish(event)
	}
	
	history := bus.GetHistory()
	if len(history) != 5 {
		t.Errorf("Expected 5 events in history, got %d", len(history))
	}
	
	// Test slice
	slice := bus.GetHistorySlice(3)
	if len(slice) != 3 {
		t.Errorf("Expected 3 events in slice, got %d", len(slice))
	}
}

func TestHistoryOverflow(t *testing.T) {
	bus := NewBusWithConfig(5)
	defer bus.Close()

	// Publish 10 events (double the capacity)
	for i := 0; i < 10; i++ {
		bus.Publish(NewEvent(EventLobeStart))
	}
	
	history := bus.GetHistory()
	if len(history) != 5 {
		t.Errorf("Expected 5 events in history (max capacity), got %d", len(history))
	}
}

func TestMultipleSubscribers(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	var wg sync.WaitGroup
	counters := [3]*atomic.Int32{{}, {}, {}}
	
	for i := 0; i < 3; i++ {
		wg.Add(1)
		idx := i
		bus.Subscribe(EventLobeStart, func(e Event) {
			counters[idx].Add(1)
			wg.Done()
		})
	}
	
	bus.Publish(NewEvent(EventLobeStart))
	
	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()
	
	select {
	case <-done:
		for i, c := range counters {
			if c.Load() != 1 {
				t.Errorf("Subscriber %d expected 1 call, got %d", i, c.Load())
			}
		}
	case <-time.After(time.Second):
		t.Error("Timeout waiting for all subscribers")
	}
}

func TestConcurrentPublishSubscribe(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	received := atomic.Int32{}
	
	// Create multiple subscribers
	for i := 0; i < 10; i++ {
		bus.Subscribe(EventLobeStart, func(e Event) {
			received.Add(1)
		})
	}
	
	// Concurrent publishes
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bus.Publish(NewEvent(EventLobeStart))
		}()
	}
	
	wg.Wait()
	time.Sleep(200 * time.Millisecond) // Allow handlers to process
	
	expected := int32(100 * 10) // 100 events * 10 subscribers
	if received.Load() != expected {
		t.Errorf("Expected %d total events, got %d", expected, received.Load())
	}
}

func TestPublishAfterClose(t *testing.T) {
	bus := NewBus()
	bus.Close()
	
	err := bus.Publish(NewEvent(EventLobeStart))
	if err == nil {
		t.Error("Expected error when publishing to closed bus")
	}
}

func TestUnsubscribeNonExistent(t *testing.T) {
	bus := NewBus()
	defer bus.Close()
	
	err := bus.Unsubscribe(SubscriptionID("nonexistent"))
	if err == nil {
		t.Error("Expected error when unsubscribing non-existent ID")
	}
}

func TestSubscriptionCounts(t *testing.T) {
	bus := NewBus()
	defer bus.Close()
	
	// Initially empty
	if bus.SubscriptionsCount() != 0 {
		t.Errorf("Expected 0 subscriptions, got %d", bus.SubscriptionsCount())
	}
	
	// Add typed subscriptions
	id1 := bus.Subscribe(EventLobeStart, func(e Event) {})
	id2 := bus.Subscribe(EventLobeComplete, func(e Event) {})
	
	if bus.SubscriptionsCount() != 2 {
		t.Errorf("Expected 2 subscriptions, got %d", bus.SubscriptionsCount())
	}
	
	// Add wildcard
	bus.Subscribe(EventType(""), func(e Event) {})
	
	if bus.SubscriptionsCount() != 3 {
		t.Errorf("Expected 3 subscriptions, got %d", bus.SubscriptionsCount())
	}
	
	if bus.WildcardSubscriptionsCount() != 1 {
		t.Errorf("Expected 1 wildcard subscription, got %d", bus.WildcardSubscriptionsCount())
	}
	
	if bus.TypedSubscriptionsCount(EventLobeStart) != 1 {
		t.Errorf("Expected 1 typed subscription for EventLobeStart, got %d", bus.TypedSubscriptionsCount(EventLobeStart))
	}
	
	// Unsubscribe
	bus.Unsubscribe(id1)
	
	if bus.SubscriptionsCount() != 2 {
		t.Errorf("Expected 2 subscriptions after unsubscribe, got %d", bus.SubscriptionsCount())
	}
	
	// Unsubscribe other
	bus.Unsubscribe(id2)
	
	if bus.TypedSubscriptionsCount(EventLobeComplete) != 0 {
		t.Errorf("Expected 0 typed subscriptions for EventLobeComplete after unsubscribe, got %d", bus.TypedSubscriptionsCount(EventLobeComplete))
	}
}

func TestNewEvent(t *testing.T) {
	event := NewEvent(EventLobeStart)
	
	if event.ID == "" {
		t.Error("NewEvent should generate an ID")
	}
	
	if event.Type != EventLobeStart {
		t.Errorf("Expected type %s, got %s", EventLobeStart, event.Type)
	}
	
	if event.Timestamp.IsZero() {
		t.Error("NewEvent should set a timestamp")
	}
}

func BenchmarkPublish(b *testing.B) {
	bus := NewBus()
	defer bus.Close()
	
	bus.Subscribe(EventLobeStart, func(e Event) {})
	
	event := NewEvent(EventLobeStart)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.Publish(event)
	}
}

func BenchmarkPublishMultipleSubscribers(b *testing.B) {
	bus := NewBus()
	defer bus.Close()
	
	for i := 0; i < 10; i++ {
		bus.Subscribe(EventLobeStart, func(e Event) {})
	}
	
	event := NewEvent(EventLobeStart)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.Publish(event)
	}
}
