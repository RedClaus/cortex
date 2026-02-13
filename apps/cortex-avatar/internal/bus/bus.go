// Package bus provides an internal event bus for component communication
package bus

import (
	"sync"
)

// EventType identifies different event types
type EventType string

// Event types for CortexAvatar
const (
	// Connection events
	EventTypeConnected    EventType = "connection.connected"
	EventTypeDisconnected EventType = "connection.disconnected"
	EventTypeError        EventType = "connection.error"

	// Audio events
	EventTypeListeningStarted EventType = "audio.listening_started"
	EventTypeListeningStopped EventType = "audio.listening_stopped"
	EventTypeSpeakingStarted  EventType = "audio.speaking_started"
	EventTypeSpeakingStopped  EventType = "audio.speaking_stopped"
	EventTypeVADActive        EventType = "audio.vad_active"
	EventTypeVADInactive      EventType = "audio.vad_inactive"
	EventTypeTranscript       EventType = "audio.transcript"

	// Avatar events
	EventTypeAvatarStateChanged EventType = "avatar.state_changed"
	EventTypeEmotionChanged     EventType = "avatar.emotion_changed"
	EventTypeMouthShapeChanged  EventType = "avatar.mouth_shape_changed"

	// Vision events
	EventTypeCameraEnabled       EventType = "vision.camera_enabled"
	EventTypeCameraDisabled      EventType = "vision.camera_disabled"
	EventTypeScreenShareEnabled  EventType = "vision.screen_enabled"
	EventTypeScreenShareDisabled EventType = "vision.screen_disabled"
	EventTypeFrameCaptured       EventType = "vision.frame_captured"

	// A2A events
	EventTypeTaskStarted   EventType = "a2a.task_started"
	EventTypeTaskCompleted EventType = "a2a.task_completed"
	EventTypeTaskFailed    EventType = "a2a.task_failed"
	EventTypeArtifact      EventType = "a2a.artifact"

	// TTS events
	EventTypeTTSStarted   EventType = "tts.started"
	EventTypeTTSCompleted EventType = "tts.completed"
	EventTypeTTSPhoneme   EventType = "tts.phoneme"

	// Audio state events
	EventTypeAudioStateChanged EventType = "audio.state_changed"
	EventTypeSpeechStart       EventType = "audio.speech_start"
	EventTypeSpeechEnd         EventType = "audio.speech_end"

	// STT events
	EventTypeSTTResult     EventType = "stt.result"
	EventTypeSTTPartial    EventType = "stt.partial"
)

// Event represents a bus event
type Event struct {
	Type EventType
	Data map[string]any
}

// Handler is a function that handles events
type Handler func(Event)

// EventBus is a simple pub/sub event bus
type EventBus struct {
	mu       sync.RWMutex
	handlers map[EventType][]Handler
}

// NewEventBus creates a new event bus
func NewEventBus() *EventBus {
	return &EventBus{
		handlers: make(map[EventType][]Handler),
	}
}

// Subscribe adds a handler for an event type
func (b *EventBus) Subscribe(eventType EventType, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

// SubscribeMultiple adds a handler for multiple event types
func (b *EventBus) SubscribeMultiple(eventTypes []EventType, handler Handler) {
	for _, et := range eventTypes {
		b.Subscribe(et, handler)
	}
}

// Publish sends an event to all subscribed handlers
func (b *EventBus) Publish(event Event) {
	b.mu.RLock()
	handlers := make([]Handler, len(b.handlers[event.Type]))
	copy(handlers, b.handlers[event.Type])
	b.mu.RUnlock()

	for _, handler := range handlers {
		// Call handlers in goroutines to avoid blocking
		go handler(event)
	}
}

// PublishSync sends an event and waits for all handlers to complete
func (b *EventBus) PublishSync(event Event) {
	b.mu.RLock()
	handlers := make([]Handler, len(b.handlers[event.Type]))
	copy(handlers, b.handlers[event.Type])
	b.mu.RUnlock()

	var wg sync.WaitGroup
	for _, handler := range handlers {
		wg.Add(1)
		go func(h Handler) {
			defer wg.Done()
			h(event)
		}(handler)
	}
	wg.Wait()
}

// Clear removes all handlers
func (b *EventBus) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers = make(map[EventType][]Handler)
}
