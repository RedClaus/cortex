package bus

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

const (
	// DefaultHistorySize is the number of recent events to retain for replay.
	DefaultHistorySize = 1000

	// DefaultChannelBuffer is the buffer size for subscriber channels.
	DefaultChannelBuffer = 100
)

// SubscriptionID is a unique identifier for event subscriptions.
type SubscriptionID string

// Subscription represents a single event subscription.
type Subscription struct {
	ID        SubscriptionID
	EventType EventType
	Handler   func(Event)
	Channel   chan Event
	done      chan struct{}
}

// Bus is the core Neural Bus that enables communication between CortexBrain lobes.
// It provides thread-safe pub/sub with wildcard support and event history.
type Bus struct {
	// Core state
	subscriptions     map[SubscriptionID]*Subscription
	subscriptionsMu   sync.RWMutex
	subCounter        uint64

	// Event type to subscription mapping for fast lookup
	typedSubs         map[EventType]map[SubscriptionID]*Subscription
	typedSubsMu       sync.RWMutex

	// Wildcard subscribers (receive all events)
	wildcardSubs      map[SubscriptionID]*Subscription
	wildcardSubsMu    sync.RWMutex

	// Event history for replay
	history           []Event
	historyMu         sync.RWMutex
	historySize       int

	// Control
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup
	closed            atomic.Bool
}

// NewBus creates a new Neural Bus with default configuration.
func NewBus() *Bus {
	return NewBusWithConfig(DefaultHistorySize)
}

// NewBusWithConfig creates a new Neural Bus with custom history size.
func NewBusWithConfig(historySize int) *Bus {
	ctx, cancel := context.WithCancel(context.Background())
	
	bus := &Bus{
		subscriptions:  make(map[SubscriptionID]*Subscription),
		typedSubs:      make(map[EventType]map[SubscriptionID]*Subscription),
		wildcardSubs:   make(map[SubscriptionID]*Subscription),
		history:        make([]Event, 0, historySize),
		historySize:    historySize,
		ctx:            ctx,
		cancel:         cancel,
	}

	return bus
}

// Subscribe registers a handler for a specific event type.
// Use EventType("") to subscribe to all events (wildcard).
// Returns a subscription ID that can be used to unsubscribe.
func (b *Bus) Subscribe(eventType EventType, handler func(Event)) SubscriptionID {
	if b.closed.Load() {
		return ""
	}

	b.subCounter++
	id := SubscriptionID(fmt.Sprintf("sub_%d", b.subCounter))

	sub := &Subscription{
		ID:        id,
		EventType: eventType,
		Handler:   handler,
		Channel:   make(chan Event, DefaultChannelBuffer),
		done:      make(chan struct{}),
	}

	// Register subscription
	b.subscriptionsMu.Lock()
	b.subscriptions[id] = sub
	b.subscriptionsMu.Unlock()

	if eventType == "" {
		// Wildcard subscription
		b.wildcardSubsMu.Lock()
		b.wildcardSubs[id] = sub
		b.wildcardSubsMu.Unlock()
	} else {
		// Typed subscription
		b.typedSubsMu.Lock()
		if b.typedSubs[eventType] == nil {
			b.typedSubs[eventType] = make(map[SubscriptionID]*Subscription)
		}
		b.typedSubs[eventType][id] = sub
		b.typedSubsMu.Unlock()
	}

	// Start goroutine to handle events for this subscription
	b.wg.Add(1)
	go b.handleSubscription(sub)

	return id
}

// handleSubscription processes events for a single subscription.
func (b *Bus) handleSubscription(sub *Subscription) {
	defer b.wg.Done()
	
	for {
		select {
		case event := <-sub.Channel:
			sub.Handler(event)
		case <-sub.done:
			return
		case <-b.ctx.Done():
			return
		}
	}
}

// Unsubscribe removes a subscription by ID.
func (b *Bus) Unsubscribe(id SubscriptionID) error {
	if b.closed.Load() {
		return fmt.Errorf("bus is closed")
	}

	b.subscriptionsMu.Lock()
	sub, exists := b.subscriptions[id]
	if !exists {
		b.subscriptionsMu.Unlock()
		return fmt.Errorf("subscription %s not found", id)
	}
	delete(b.subscriptions, id)
	b.subscriptionsMu.Unlock()

	// Remove from typed or wildcard maps
	if sub.EventType == "" {
		b.wildcardSubsMu.Lock()
		delete(b.wildcardSubs, id)
		b.wildcardSubsMu.Unlock()
	} else {
		b.typedSubsMu.Lock()
		if subs, ok := b.typedSubs[sub.EventType]; ok {
			delete(subs, id)
			if len(subs) == 0 {
				delete(b.typedSubs, sub.EventType)
			}
		}
		b.typedSubsMu.Unlock()
	}

	// Signal the subscription to stop
	close(sub.done)

	return nil
}

// Publish sends an event to all matching subscribers.
func (b *Bus) Publish(event Event) error {
	if b.closed.Load() {
		return fmt.Errorf("bus is closed")
	}

	// Add to history
	b.addToHistory(event)

	// Send to wildcard subscribers (all events)
	b.wildcardSubsMu.RLock()
	for _, sub := range b.wildcardSubs {
		select {
		case sub.Channel <- event:
		default:
			// Channel full, drop event for this subscriber
		}
	}
	b.wildcardSubsMu.RUnlock()

	// Send to typed subscribers
	b.typedSubsMu.RLock()
	if subs, ok := b.typedSubs[event.Type]; ok {
		for _, sub := range subs {
			select {
			case sub.Channel <- event:
			default:
				// Channel full, drop event for this subscriber
			}
		}
	}
	b.typedSubsMu.RUnlock()

	return nil
}

// addToHistory safely appends an event to the history buffer.
func (b *Bus) addToHistory(event Event) {
	b.historyMu.Lock()
	defer b.historyMu.Unlock()

	b.history = append(b.history, event)
	
	// Trim if exceeds size
	if len(b.history) > b.historySize {
		b.history = b.history[len(b.history)-b.historySize:]
	}
}

// GetHistory returns a copy of the recent event history.
func (b *Bus) GetHistory() []Event {
	b.historyMu.RLock()
	defer b.historyMu.RUnlock()

	result := make([]Event, len(b.history))
	copy(result, b.history)
	return result
}

// GetHistorySlice returns a slice of recent events (last n events).
func (b *Bus) GetHistorySlice(n int) []Event {
	b.historyMu.RLock()
	defer b.historyMu.RUnlock()

	if n > len(b.history) {
		n = len(b.history)
	}

	start := len(b.history) - n
	result := make([]Event, n)
	copy(result, b.history[start:])
	return result
}

// SubscriptionsCount returns the total number of active subscriptions.
func (b *Bus) SubscriptionsCount() int {
	b.subscriptionsMu.RLock()
	defer b.subscriptionsMu.RUnlock()
	return len(b.subscriptions)
}

// TypedSubscriptionsCount returns the number of subscriptions for a specific event type.
func (b *Bus) TypedSubscriptionsCount(eventType EventType) int {
	b.typedSubsMu.RLock()
	defer b.typedSubsMu.RUnlock()
	
	if subs, ok := b.typedSubs[eventType]; ok {
		return len(subs)
	}
	return 0
}

// WildcardSubscriptionsCount returns the number of wildcard subscriptions.
func (b *Bus) WildcardSubscriptionsCount() int {
	b.wildcardSubsMu.RLock()
	defer b.wildcardSubsMu.RUnlock()
	return len(b.wildcardSubs)
}

// Close shuts down the bus and all subscriptions.
func (b *Bus) Close() error {
	if !b.closed.CompareAndSwap(false, true) {
		return fmt.Errorf("bus already closed")
	}

	b.cancel()
	
	// Wait for all goroutines to finish
	b.wg.Wait()

	// Close all subscription channels
	b.subscriptionsMu.Lock()
	for _, sub := range b.subscriptions {
		close(sub.Channel)
	}
	b.subscriptions = make(map[SubscriptionID]*Subscription)
	b.subscriptionsMu.Unlock()

	// Clear typed and wildcard maps
	b.typedSubsMu.Lock()
	b.typedSubs = make(map[EventType]map[SubscriptionID]*Subscription)
	b.typedSubsMu.Unlock()

	b.wildcardSubsMu.Lock()
	b.wildcardSubs = make(map[SubscriptionID]*Subscription)
	b.wildcardSubsMu.Unlock()

	return nil
}
