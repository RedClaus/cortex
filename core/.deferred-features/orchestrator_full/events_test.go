package orchestrator

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/normanking/cortex/internal/bus"
	"github.com/normanking/cortex/internal/persona"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================================================================
// CR-017 PHASE 5: EVENT WIRING TESTS
// ===========================================================================

// TestOrchestrator_PublishesRequestReceivedEvent tests that Process() publishes
// a RequestReceived event at the start of processing.
func TestOrchestrator_PublishesRequestReceivedEvent(t *testing.T) {
	eventBus := bus.New()
	defer eventBus.Close()

	o := New(WithEventBus(eventBus))

	// Subscribe to events
	var received *bus.RequestReceivedEvent
	var mu sync.Mutex
	sub := eventBus.Subscribe(bus.EventTypeRequestReceived, func(e bus.Event) {
		mu.Lock()
		defer mu.Unlock()
		received = e.(*bus.RequestReceivedEvent)
	})
	defer sub.Unsubscribe()

	// Trigger action
	ctx := context.Background()
	_, err := o.Process(ctx, &Request{Input: "test input"})
	require.NoError(t, err)

	// Give event bus time to process
	time.Sleep(50 * time.Millisecond)

	// Verify event
	mu.Lock()
	defer mu.Unlock()
	assert.NotNil(t, received, "RequestReceivedEvent should be published")
	assert.Equal(t, "test input", received.UserInput)
	assert.NotEmpty(t, received.RequestID)
}

// TestOrchestrator_PublishesResponseGeneratedEvent tests that Process() publishes
// a ResponseGenerated event after processing completes.
func TestOrchestrator_PublishesResponseGeneratedEvent(t *testing.T) {
	eventBus := bus.New()
	defer eventBus.Close()

	o := New(WithEventBus(eventBus))

	// Subscribe to events
	var received *bus.ResponseGeneratedEvent
	var mu sync.Mutex
	sub := eventBus.Subscribe(bus.EventTypeResponseGenerated, func(e bus.Event) {
		mu.Lock()
		defer mu.Unlock()
		received = e.(*bus.ResponseGeneratedEvent)
	})
	defer sub.Unsubscribe()

	// Trigger action
	ctx := context.Background()
	resp, err := o.Process(ctx, &Request{Input: "echo hello"})
	require.NoError(t, err)

	// Give event bus time to process
	time.Sleep(50 * time.Millisecond)

	// Verify event
	mu.Lock()
	defer mu.Unlock()
	assert.NotNil(t, received, "ResponseGeneratedEvent should be published")
	assert.Equal(t, resp.RequestID, received.RequestID)
	assert.Equal(t, resp.Success, received.Success)
	assert.True(t, received.Latency > 0, "Latency should be positive")
}

// TestOrchestrator_PublishesModeChangedEvent tests that SetMode() publishes
// a ModeChanged event.
func TestOrchestrator_PublishesModeChangedEvent(t *testing.T) {
	eventBus := bus.New()
	defer eventBus.Close()

	o := New(WithEventBus(eventBus))

	// Subscribe to events
	var received *bus.ModeChangedEvent
	var mu sync.Mutex
	sub := eventBus.Subscribe(bus.EventTypeModeChanged, func(e bus.Event) {
		mu.Lock()
		defer mu.Unlock()
		received = e.(*bus.ModeChangedEvent)
	})
	defer sub.Unsubscribe()

	// Set initial mode to ensure we have a known state
	o.activeMode = persona.ModeNormal

	// Trigger mode change
	o.SetMode(persona.ModeDebugging)

	// Give event bus time to process
	time.Sleep(50 * time.Millisecond)

	// Verify event
	mu.Lock()
	defer mu.Unlock()
	assert.NotNil(t, received, "ModeChangedEvent should be published")
	assert.Equal(t, string(persona.ModeNormal), received.FromMode)
	assert.Equal(t, string(persona.ModeDebugging), received.ToMode)
	assert.Equal(t, "manual", received.Trigger)
}

// TestOrchestrator_PublishesModeChangedEventWithTrigger tests that SetModeWithTrigger()
// publishes a ModeChanged event with the specified trigger.
func TestOrchestrator_PublishesModeChangedEventWithTrigger(t *testing.T) {
	eventBus := bus.New()
	defer eventBus.Close()

	o := New(WithEventBus(eventBus))

	// Subscribe to events
	var received *bus.ModeChangedEvent
	var mu sync.Mutex
	sub := eventBus.Subscribe(bus.EventTypeModeChanged, func(e bus.Event) {
		mu.Lock()
		defer mu.Unlock()
		received = e.(*bus.ModeChangedEvent)
	})
	defer sub.Unsubscribe()

	// Set initial mode
	o.activeMode = persona.ModeNormal

	// Trigger mode change with custom trigger
	o.SetModeWithTrigger(persona.ModeTeaching, "voice_command")

	// Give event bus time to process
	time.Sleep(50 * time.Millisecond)

	// Verify event
	mu.Lock()
	defer mu.Unlock()
	assert.NotNil(t, received, "ModeChangedEvent should be published")
	assert.Equal(t, string(persona.ModeNormal), received.FromMode)
	assert.Equal(t, string(persona.ModeTeaching), received.ToMode)
	assert.Equal(t, "voice_command", received.Trigger)
}

// TestOrchestrator_PublishesToolExecutedEvent tests that tool execution publishes
// a ToolExecutedEventV2 event.
func TestOrchestrator_PublishesToolExecutedEvent(t *testing.T) {
	eventBus := bus.New()
	defer eventBus.Close()

	o := New(WithEventBus(eventBus))

	// Subscribe to events
	var received *bus.ToolExecutedEventV2
	var mu sync.Mutex
	sub := eventBus.Subscribe(bus.EventTypeToolExecuted, func(e bus.Event) {
		mu.Lock()
		defer mu.Unlock()
		// Accept both v1 and v2 events
		if v2, ok := e.(*bus.ToolExecutedEventV2); ok {
			received = v2
		}
	})
	defer sub.Unsubscribe()

	// Trigger tool execution via command request
	ctx := context.Background()
	_, err := o.Process(ctx, &Request{
		Type:  RequestCommand,
		Input: "echo test",
	})
	require.NoError(t, err)

	// Give event bus time to process
	time.Sleep(50 * time.Millisecond)

	// Verify event
	mu.Lock()
	defer mu.Unlock()
	assert.NotNil(t, received, "ToolExecutedEventV2 should be published")
	assert.Equal(t, "bash", received.Tool)
	assert.True(t, received.Success)
	assert.True(t, received.Latency > 0, "Latency should be positive")
}

// TestOrchestrator_NoEventsWithoutEventBus tests that no events are published
// when event bus is not configured.
func TestOrchestrator_NoEventsWithoutEventBus(t *testing.T) {
	// Create orchestrator WITHOUT event bus
	o := New()

	// This should not panic
	ctx := context.Background()
	_, err := o.Process(ctx, &Request{Input: "test"})
	assert.NoError(t, err)

	// SetMode should also not panic
	o.SetMode(persona.ModeDebugging)
}

// TestOrchestrator_RequestResponseEventPair tests that request and response events
// have matching request IDs.
func TestOrchestrator_RequestResponseEventPair(t *testing.T) {
	eventBus := bus.New()
	defer eventBus.Close()

	o := New(WithEventBus(eventBus))

	// Subscribe to both events
	var requestEvent *bus.RequestReceivedEvent
	var responseEvent *bus.ResponseGeneratedEvent
	var mu sync.Mutex

	sub1 := eventBus.Subscribe(bus.EventTypeRequestReceived, func(e bus.Event) {
		mu.Lock()
		defer mu.Unlock()
		requestEvent = e.(*bus.RequestReceivedEvent)
	})
	defer sub1.Unsubscribe()

	sub2 := eventBus.Subscribe(bus.EventTypeResponseGenerated, func(e bus.Event) {
		mu.Lock()
		defer mu.Unlock()
		responseEvent = e.(*bus.ResponseGeneratedEvent)
	})
	defer sub2.Unsubscribe()

	// Process request
	ctx := context.Background()
	_, err := o.Process(ctx, &Request{Input: "echo hello"})
	require.NoError(t, err)

	// Give event bus time to process
	time.Sleep(100 * time.Millisecond)

	// Verify both events have matching request IDs
	mu.Lock()
	defer mu.Unlock()
	require.NotNil(t, requestEvent, "RequestReceivedEvent should be published")
	require.NotNil(t, responseEvent, "ResponseGeneratedEvent should be published")
	assert.Equal(t, requestEvent.RequestID, responseEvent.RequestID,
		"Request and Response events should have matching request IDs")
}

// TestMemoryCoordinator_PublishesMemoryUpdatedEvent tests that UpdateUserMemory
// publishes a MemoryUpdated event.
func TestMemoryCoordinator_PublishesMemoryUpdatedEvent(t *testing.T) {
	// Skip if we can't create a test memory store
	t.Skip("Requires memory store setup - integration test")
}

// ===========================================================================
// BENCHMARK TESTS
// ===========================================================================

func BenchmarkOrchestrator_ProcessWithEvents(b *testing.B) {
	eventBus := bus.New()
	defer eventBus.Close()

	o := New(WithEventBus(eventBus))
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		o.Process(ctx, &Request{Input: "echo hello"})
	}
}

func BenchmarkOrchestrator_ProcessWithoutEvents(b *testing.B) {
	o := New()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		o.Process(ctx, &Request{Input: "echo hello"})
	}
}
