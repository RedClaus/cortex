// Package bus provides compatibility aliases for the Neural Bus.
// These aliases exist to support code that uses the legacy EventBus naming.
package bus

import (
	"fmt"
	"time"
)

// EventBus is an alias for Bus for backward compatibility.
type EventBus = Bus

// BaseEvent provides a base struct that can be embedded in custom events.
// It contains the common fields shared by all events.
type BaseEvent struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Type      EventType `json:"type"`
	RequestID string    `json:"request_id,omitempty"`
}

// NewBaseEvent creates a new base event with the given type.
func NewBaseEvent(eventType EventType) BaseEvent {
	return BaseEvent{
		ID:        generateEventID(),
		Timestamp: time.Now(),
		Type:      eventType,
	}
}

// NewMemoryUpdatedEvent creates a memory updated event.
func NewMemoryUpdatedEvent(userID, field, source string) Event {
	return Event{
		ID:        generateEventID(),
		Timestamp: time.Now(),
		Type:      EventType("memory_updated"),
		Details:   fmt.Sprintf("user=%s field=%s source=%s", userID, field, source),
	}
}

// NewRequestReceivedEvent creates a request received event.
func NewRequestReceivedEvent(requestID, content string) Event {
	return Event{
		ID:        generateEventID(),
		Timestamp: time.Now(),
		Type:      EventType("request_received"),
		RequestID: requestID,
		Content:   content,
	}
}

// NewResponseGeneratedEvent creates a response generated event.
func NewResponseGeneratedEvent(requestID, content string) Event {
	return Event{
		ID:        generateEventID(),
		Timestamp: time.Now(),
		Type:      EventType("response_generated"),
		RequestID: requestID,
		Content:   content,
	}
}

// NewInterruptEvent creates an interrupt event.
func NewInterruptEvent(reason string) Event {
	return Event{
		ID:        generateEventID(),
		Timestamp: time.Now(),
		Type:      EventType("interrupt"),
		Details:   reason,
	}
}
