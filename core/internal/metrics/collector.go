package metrics

import (
	"sync"
	"time"

	"github.com/normanking/cortex/internal/bus"
)

// Collector subscribes to event bus and aggregates metrics.
type Collector struct {
	bus          *bus.EventBus
	store        *Store
	session      *SessionStats
	recentEvents []bus.Event
	mu           sync.RWMutex
	maxEvents    int
	subs         []bus.Subscription
	stopped      bool
}

// SessionStats holds current session metrics.
type SessionStats struct {
	StartTime      time.Time
	RequestCount   int
	TokensIn       int64
	TokensOut      int64
	ToolCalls      int
	TotalLatencyMs int64
	SuccessCount   int
	FailureCount   int
	ActiveAgents   int
	LastEvent      string
	LastEventTime  time.Time
	LocalRequests  int
}

// NewCollector creates a metrics collector.
func NewCollector(eventBus *bus.EventBus, store *Store) *Collector {
	return &Collector{
		bus:          eventBus,
		store:        store,
		session:      &SessionStats{StartTime: time.Now()},
		recentEvents: make([]bus.Event, 0),
		maxEvents:    50,
		subs:         make([]bus.Subscription, 0),
	}
}

// Start begins listening to event bus.
func (c *Collector) Start() {
	if c.bus == nil {
		return // Graceful handling when bus is nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopped {
		return
	}

	// Subscribe to relevant event types
	c.subs = append(c.subs, c.bus.Subscribe(bus.EventTypeAgentStarted, c.handleEvent))
	c.subs = append(c.subs, c.bus.Subscribe(bus.EventTypeAgentCompleted, c.handleEvent))
	c.subs = append(c.subs, c.bus.Subscribe(bus.EventTypeToolExecuted, c.handleEvent))
	c.subs = append(c.subs, c.bus.Subscribe(bus.EventTypeStreamChunk, c.handleEvent))
}

// Stop stops listening.
func (c *Collector) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopped {
		return
	}

	c.stopped = true

	// Unsubscribe from all events
	for _, sub := range c.subs {
		sub.Unsubscribe()
	}
	c.subs = nil
}

// GetSessionStats returns current session stats (thread-safe).
func (c *Collector) GetSessionStats() *SessionStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy
	stats := *c.session
	return &stats
}

// GetRecentEvents returns recent events for display (thread-safe).
func (c *Collector) GetRecentEvents(n int) []bus.Event {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if n > len(c.recentEvents) {
		n = len(c.recentEvents)
	}

	// Return most recent n events (from end of slice)
	start := len(c.recentEvents) - n
	if start < 0 {
		start = 0
	}

	events := make([]bus.Event, n)
	copy(events, c.recentEvents[start:])
	return events
}

// handleEvent is the central event handler that dispatches to specific handlers.
func (c *Collector) handleEvent(event bus.Event) {
	// Add to recent events
	c.mu.Lock()
	c.recentEvents = append(c.recentEvents, event)
	if len(c.recentEvents) > c.maxEvents {
		c.recentEvents = c.recentEvents[1:]
	}
	c.mu.Unlock()

	// Dispatch to specific handler
	switch e := event.(type) {
	case *bus.AgentStartedEvent:
		c.handleAgentStarted(e)
	case *bus.AgentCompletedEvent:
		c.handleAgentCompleted(e)
	case *bus.ToolExecutedEvent:
		c.handleToolExecuted(e)
	case *bus.StreamChunkEvent:
		c.handleStreamChunk(e)
	}
}

// handleAgentStarted processes agent started events.
func (c *Collector) handleAgentStarted(e *bus.AgentStartedEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.session.ActiveAgents++
	c.session.LastEvent = "agent started: " + e.AgentName
	c.session.LastEventTime = e.Timestamp()
}

// handleAgentCompleted processes agent completed events.
func (c *Collector) handleAgentCompleted(e *bus.AgentCompletedEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.session.ActiveAgents--
	if c.session.ActiveAgents < 0 {
		c.session.ActiveAgents = 0
	}

	c.session.RequestCount++
	c.session.TokensIn += int64(e.TokensUsed)
	c.session.ToolCalls += e.ToolCalls
	c.session.TotalLatencyMs += e.Duration.Milliseconds()

	if e.Success {
		c.session.SuccessCount++
	} else {
		c.session.FailureCount++
	}

	c.session.LastEvent = "agent completed"
	c.session.LastEventTime = e.Timestamp()

	// Check if this is a local model request
	if e.Provider == "ollama" || e.Provider == "local" {
		c.session.LocalRequests++
	}

	// Record to store if available
	if c.store != nil {
		metric := &RequestMetric{
			RequestType: RequestAgent,
			Provider:    e.Provider,
			Model:       e.Model,
			LatencyMs:   e.Duration.Milliseconds(),
			TokensIn:    e.TokensUsed,
			TokensOut:   0, // Not tracked in AgentCompletedEvent
			Success:     e.Success,
			ErrorMsg:    e.Error,
			CreatedAt:   e.Timestamp(),
		}
		c.store.RecordRequest(metric)
	}
}

// handleToolExecuted processes tool executed events.
func (c *Collector) handleToolExecuted(e *bus.ToolExecutedEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.session.ToolCalls++
	c.session.LastEvent = "tool: " + e.ToolName
	c.session.LastEventTime = e.Timestamp()
}

// handleStreamChunk processes stream chunk events.
func (c *Collector) handleStreamChunk(e *bus.StreamChunkEvent) {
	// Stream chunks are high-frequency, we only update last event
	c.mu.Lock()
	defer c.mu.Unlock()

	if e.IsFinal {
		c.session.LastEvent = "stream completed"
		c.session.LastEventTime = e.Timestamp()
	}
}
