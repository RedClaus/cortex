// Package bus provides the Neural Bus - the core event distribution system for CortexBrain.
// This is the nervous system that enables communication between the 20 lobes of the
// cognitive architecture.
package bus

import (
	"fmt"
	"time"
)

// EventType represents the type of cognitive event flowing through the neural bus.
type EventType string

// Event types for the 20-lobe cognitive architecture.
const (
	// Lobe lifecycle events
	EventLobeStart     EventType = "lobe_start"
	EventLobeComplete  EventType = "lobe_complete"
	EventLobeError     EventType = "lobe_error"

	// Phase events
	EventPhaseStart    EventType = "phase_start"
	EventPhaseComplete EventType = "phase_complete"

	// Inter-lobe communication
	EventPathway       EventType = "pathway"
	EventBlackboard    EventType = "blackboard"

	// I/O events
	EventMessageIn     EventType = "message_in"
	EventMessageOut    EventType = "message_out"

	// System events
	EventHeartbeat     EventType = "heartbeat"

	// LLM integration events
	EventLLMRequest    EventType = "llm_request"
	EventLLMResponse   EventType = "llm_response"
	EventLLMError      EventType = "llm_error"
)

// Event represents a single cognitive event in the CortexBrain architecture.
// Events flow through the Neural Bus to coordinate activity across all 20 lobes.
type Event struct {
	// Core identification
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Type      EventType `json:"type"`

	// Request tracking
	RequestID string `json:"request_id,omitempty"`

	// Lobe context
	Lobe      string `json:"lobe,omitempty"`
	LobeState string `json:"lobe_state,omitempty"`

	// Phase information
	Phase string `json:"phase,omitempty"`

	// Performance metrics
	Confidence float64 `json:"confidence,omitempty"`
	DurationMs int64   `json:"duration_ms,omitempty"`

	// Content
	Content string `json:"content,omitempty"`
	Details string `json:"details,omitempty"`

	// Pathway tracking (activation sequence)
	Pathway []string `json:"pathway,omitempty"`

	// Source/target for inter-lobe communication
	SourceLobe string `json:"source_lobe,omitempty"`
	TargetLobe string `json:"target_lobe,omitempty"`

	// Shared state
	Blackboard map[string]any `json:"blackboard,omitempty"`

	// Error information
	Error string `json:"error,omitempty"`

	// LLM context
	Model string `json:"model,omitempty"`
}

// LobeNames defines the 20 lobes in the 5-layer cognitive architecture.
var LobeNames = []string{
	// Perception Layer (3)
	"Vision", "Audition", "TextParsing",
	// Cognitive Layer (4)
	"Memory", "Planning", "Creativity", "Reasoning",
	// Social Layer (3)
	"Emotion", "TheoryOfMind", "Rapport",
	// Specialized Layer (6)
	"Coding", "Logic", "Temporal", "Spatial", "Causal", "Knowledge",
	// Executive Layer (4)
	"Attention", "Metacognition", "Inhibition", "Self",
}

// LobeLayer maps each lobe to its cognitive layer.
var LobeLayer = map[string]string{
	"Vision":       "Perception",
	"Audition":     "Perception",
	"TextParsing":  "Perception",
	"Memory":       "Cognitive",
	"Planning":     "Cognitive",
	"Creativity":   "Cognitive",
	"Reasoning":    "Cognitive",
	"Emotion":      "Social",
	"TheoryOfMind": "Social",
	"Rapport":      "Social",
	"Coding":       "Specialized",
	"Logic":        "Specialized",
	"Temporal":     "Specialized",
	"Spatial":      "Specialized",
	"Causal":       "Specialized",
	"Knowledge":    "Specialized",
	"Attention":    "Executive",
	"Metacognition": "Executive",
	"Inhibition":   "Executive",
	"Self":         "Executive",
}

// LayerOrder defines the typical activation sequence through the architecture.
var LayerOrder = []string{
	"Perception",
	"Cognitive",
	"Social",
	"Specialized",
	"Executive",
}

// GetLayerForLobe returns the cognitive layer for a given lobe name.
func GetLayerForLobe(lobe string) string {
	if layer, ok := LobeLayer[lobe]; ok {
		return layer
	}
	return "Unknown"
}

// eventIDCounter for generating unique event IDs.
var eventIDCounter uint64

// generateEventID creates a unique event identifier.
func generateEventID() string {
	eventIDCounter++
	return fmt.Sprintf("evt_%d_%d", time.Now().UnixNano(), eventIDCounter)
}

// NewEvent creates a new event with the current timestamp and generated ID.
func NewEvent(eventType EventType) Event {
	return Event{
		ID:        generateEventID(),
		Timestamp: time.Now().UTC(),
		Type:      eventType,
	}
}
