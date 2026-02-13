// Package cognitive implements the Cortex cognitive architecture including
// behavioral mode tracking and lane-gated processing pipelines.
package cognitive

import (
	"strings"
	"sync"
	"time"

	"github.com/normanking/cortex/internal/facets"
)

// ModeTracker manages behavioral mode state per conversation.
// Mode transitions are triggered by keyword matching (cheap, no LLM calls).
type ModeTracker struct {
	mu       sync.RWMutex
	states   map[string]*ModeState             // conversationID -> state
	personas map[string]*facets.PersonaCore    // personaID -> persona
}

// ModeState tracks the current mode for a conversation.
type ModeState struct {
	ConversationID string
	PersonaID      string
	CurrentMode    string
	EnteredAt      time.Time
	MessageCount   int
	PreviousMode   string // For debugging/analytics
}

// ModeTransition represents a mode change event.
type ModeTransition struct {
	From        string    `json:"from"`
	To          string    `json:"to"`
	Trigger     string    `json:"trigger"`      // What triggered the transition
	TriggerType string    `json:"trigger_type"` // "keyword", "manual", "exit", "reset"
	Timestamp   time.Time `json:"timestamp"`
}

// NewModeTracker creates a new mode tracker instance.
func NewModeTracker() *ModeTracker {
	return &ModeTracker{
		states:   make(map[string]*ModeState),
		personas: make(map[string]*facets.PersonaCore),
	}
}

// RegisterPersona adds a persona to the tracker.
func (t *ModeTracker) RegisterPersona(persona *facets.PersonaCore) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.personas[persona.ID] = persona
}

// UnregisterPersona removes a persona from the tracker.
func (t *ModeTracker) UnregisterPersona(personaID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.personas, personaID)
}

// UpdateMode checks for mode transitions based on user message.
// Returns the active mode and any transition that occurred.
// This is O(keywords) - very fast, no LLM calls.
func (t *ModeTracker) UpdateMode(
	conversationID string,
	personaID string,
	message string,
) (*facets.BehavioralMode, *ModeTransition) {
	t.mu.Lock()
	defer t.mu.Unlock()

	persona, ok := t.personas[personaID]
	if !ok {
		return nil, nil
	}

	// Get or create state
	state, ok := t.states[conversationID]
	if !ok {
		state = &ModeState{
			ConversationID: conversationID,
			PersonaID:      personaID,
			CurrentMode:    persona.DefaultMode,
			EnteredAt:      time.Now(),
		}
		t.states[conversationID] = state
	}

	// Update persona ID if changed (conversation switched persona)
	if state.PersonaID != personaID {
		state.PersonaID = personaID
		state.CurrentMode = persona.DefaultMode
		state.EnteredAt = time.Now()
		state.MessageCount = 0
	}

	state.MessageCount++
	msgLower := strings.ToLower(message)

	currentMode := findMode(persona.Modes, state.CurrentMode)

	// Check for manual trigger first (highest priority)
	for _, mode := range persona.Modes {
		if mode.ManualTrigger != "" && strings.Contains(msgLower, strings.ToLower(mode.ManualTrigger)) {
			if mode.ID != state.CurrentMode {
				transition := t.transitionTo(state, mode.ID, mode.ManualTrigger, "manual")
				return &mode, transition
			}
			return &mode, nil
		}
	}

	// Check for exit from current mode
	if currentMode != nil && currentMode.ID != persona.DefaultMode {
		for _, keyword := range currentMode.ExitKeywords {
			if containsWord(msgLower, strings.ToLower(keyword)) {
				transition := t.transitionTo(state, persona.DefaultMode, keyword, "exit")
				return findMode(persona.Modes, persona.DefaultMode), transition
			}
		}
	}

	// Check for entry into new mode
	for _, mode := range persona.Modes {
		if mode.ID == state.CurrentMode {
			continue
		}

		for _, keyword := range mode.EntryKeywords {
			if containsWord(msgLower, strings.ToLower(keyword)) {
				transition := t.transitionTo(state, mode.ID, keyword, "keyword")
				modeCopy := mode // avoid pointer to loop variable
				return &modeCopy, transition
			}
		}
	}

	return currentMode, nil
}

// GetCurrentMode returns the current mode without checking for transitions.
func (t *ModeTracker) GetCurrentMode(conversationID string) *facets.BehavioralMode {
	t.mu.RLock()
	defer t.mu.RUnlock()

	state, ok := t.states[conversationID]
	if !ok {
		return nil
	}

	persona, ok := t.personas[state.PersonaID]
	if !ok {
		return nil
	}

	return findMode(persona.Modes, state.CurrentMode)
}

// GetModeState returns the full mode state for a conversation.
func (t *ModeTracker) GetModeState(conversationID string) *ModeState {
	t.mu.RLock()
	defer t.mu.RUnlock()

	state, ok := t.states[conversationID]
	if !ok {
		return nil
	}

	// Return a copy to avoid race conditions
	stateCopy := *state
	return &stateCopy
}

// ResetMode manually resets to default mode.
func (t *ModeTracker) ResetMode(conversationID string) *ModeTransition {
	t.mu.Lock()
	defer t.mu.Unlock()

	state, ok := t.states[conversationID]
	if !ok {
		return nil
	}

	persona, ok := t.personas[state.PersonaID]
	if !ok {
		return nil
	}

	if state.CurrentMode == persona.DefaultMode {
		return nil
	}

	return t.transitionTo(state, persona.DefaultMode, "manual_reset", "reset")
}

// SetMode manually sets a specific mode.
func (t *ModeTracker) SetMode(conversationID string, modeID string) *ModeTransition {
	t.mu.Lock()
	defer t.mu.Unlock()

	state, ok := t.states[conversationID]
	if !ok {
		return nil
	}

	persona, ok := t.personas[state.PersonaID]
	if !ok {
		return nil
	}

	// Verify mode exists
	mode := findMode(persona.Modes, modeID)
	if mode == nil {
		return nil
	}

	if state.CurrentMode == modeID {
		return nil
	}

	return t.transitionTo(state, modeID, "manual_set", "manual")
}

// ClearConversation removes state for a conversation.
func (t *ModeTracker) ClearConversation(conversationID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.states, conversationID)
}

// transitionTo updates state and returns transition record.
func (t *ModeTracker) transitionTo(state *ModeState, newMode, trigger, triggerType string) *ModeTransition {
	transition := &ModeTransition{
		From:        state.CurrentMode,
		To:          newMode,
		Trigger:     trigger,
		TriggerType: triggerType,
		Timestamp:   time.Now(),
	}

	state.PreviousMode = state.CurrentMode
	state.CurrentMode = newMode
	state.EnteredAt = time.Now()
	state.MessageCount = 0

	return transition
}

// Helper functions

func findMode(modes []facets.BehavioralMode, id string) *facets.BehavioralMode {
	for i := range modes {
		if modes[i].ID == id {
			return &modes[i]
		}
	}
	return nil
}

// containsWord checks for whole word match (not substring).
// This prevents "error" from matching "errorhandling" or "terror".
func containsWord(text, word string) bool {
	// Simple word boundary check
	idx := strings.Index(text, word)
	if idx < 0 {
		return false
	}

	// Check left boundary
	if idx > 0 {
		c := text[idx-1]
		if isWordChar(c) {
			return false
		}
	}

	// Check right boundary
	endIdx := idx + len(word)
	if endIdx < len(text) {
		c := text[endIdx]
		if isWordChar(c) {
			return false
		}
	}

	return true
}

func isWordChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}
