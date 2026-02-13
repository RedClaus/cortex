// Package voice provides voice-related types and utilities for Cortex.
// conversation_state.go implements the conversational state machine for human-like interaction (CR-012-C).
package voice

import (
	"math/rand"
	"sync"
	"time"
)

// ConversationState represents Henry's current conversational mode.
type ConversationState string

const (
	// StateCold indicates no recent interaction - use formal greetings.
	StateCold ConversationState = "cold"
	// StateWarm indicates recent interaction - use casual responses.
	StateWarm ConversationState = "warm"
	// StateActive indicates mid-conversation - use engaged, minimal responses.
	StateActive ConversationState = "active"
)

// UIEventType represents types of UI events for visual feedback sync.
type UIEventType string

const (
	UIEventListening  UIEventType = "listening"
	UIEventProcessing UIEventType = "processing"
	UIEventSpeaking   UIEventType = "speaking"
	UIEventIdle       UIEventType = "idle"
)

// UIEvent represents a UI synchronization event.
type UIEvent struct {
	Type      UIEventType
	State     ConversationState
	Timestamp time.Time
}

// StateConfig holds timing configuration for state transitions.
type StateConfig struct {
	// WarmTimeout is the duration before Warm → Cold transition.
	WarmTimeout time.Duration
	// ActiveTimeout is the duration before Active → Warm transition.
	ActiveTimeout time.Duration
	// MinSpeechForBackchannel is the minimum speech duration to trigger backchanneling (Risk A).
	MinSpeechForBackchannel time.Duration
	// ConfidenceThreshold is the minimum confidence for clear speech (Risk A).
	ConfidenceThreshold float64
	// BackchannelCooldown is the minimum time between backchannels.
	BackchannelCooldown time.Duration
}

// DefaultStateConfig returns production defaults.
func DefaultStateConfig() StateConfig {
	return StateConfig{
		WarmTimeout:             2 * time.Minute,
		ActiveTimeout:           30 * time.Second,
		MinSpeechForBackchannel: 500 * time.Millisecond,
		ConfidenceThreshold:     0.6,
		BackchannelCooldown:     2 * time.Second,
	}
}

// PlaybackCallbacks for coordinating with other systems.
// These callbacks enable UI synchronization and state management.
type PlaybackCallbacks struct {
	// OnPlaybackStart is called when audio starts playing.
	OnPlaybackStart func()
	// OnPlaybackEnd is called when audio finishes or is stopped.
	OnPlaybackEnd func()
}

// AudioPlayerInterface defines the interface for audio playback control.
// Used for interruption handling (Risk C).
type AudioPlayerInterface interface {
	// Stop stops current audio playback immediately.
	Stop()
	// IsPlaying returns true if audio is currently playing.
	IsPlaying() bool
	// PlayBytes plays raw audio data.
	PlayBytes(data []byte) error
	// SetCallbacks registers playback event callbacks.
	SetCallbacks(callbacks PlaybackCallbacks)
}

// ConversationStateManager tracks and manages conversation state.
type ConversationStateManager struct {
	mu sync.RWMutex

	config          StateConfig
	currentState    ConversationState
	lastInteraction time.Time
	lastBackchannel time.Time
	turnCount       int
	sessionStart    time.Time

	// Audio player reference for interruption handling (Risk C)
	audioPlayer AudioPlayerInterface

	// Callbacks
	onStateChange func(old, new ConversationState)
	onUIEvent     func(UIEvent)
}

// NewConversationStateManager creates a new state manager with the given config.
func NewConversationStateManager(config StateConfig) *ConversationStateManager {
	return &ConversationStateManager{
		config:       config,
		currentState: StateCold,
		sessionStart: time.Now(),
	}
}

// SetAudioPlayer sets the audio player for interruption handling (Risk C).
func (m *ConversationStateManager) SetAudioPlayer(player AudioPlayerInterface) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.audioPlayer = player
}

// GetState returns the current conversation state, accounting for timeouts.
func (m *ConversationStateManager) GetState() ConversationState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.calculateCurrentState()
}

// calculateCurrentState determines state based on time elapsed (must hold lock).
func (m *ConversationStateManager) calculateCurrentState() ConversationState {
	if m.lastInteraction.IsZero() {
		return StateCold
	}

	elapsed := time.Since(m.lastInteraction)

	switch m.currentState {
	case StateActive:
		if elapsed > m.config.WarmTimeout {
			return StateCold
		}
		if elapsed > m.config.ActiveTimeout {
			return StateWarm
		}
		return StateActive

	case StateWarm:
		if elapsed > m.config.WarmTimeout {
			return StateCold
		}
		return StateWarm

	default:
		return StateCold
	}
}

// RecordInteraction records a new interaction and updates state.
// Returns the new state after the interaction.
func (m *ConversationStateManager) RecordInteraction(isUserSpeaking bool) ConversationState {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldState := m.currentState
	m.lastInteraction = time.Now()

	// Risk C: Stop playback if Henry is speaking and user interrupts
	if isUserSpeaking && m.audioPlayer != nil && m.audioPlayer.IsPlaying() {
		m.audioPlayer.Stop()
	}

	// Determine new state
	var newState ConversationState
	switch oldState {
	case StateCold:
		newState = StateWarm
	case StateWarm:
		if isUserSpeaking {
			newState = StateActive
		} else {
			newState = StateWarm
		}
	case StateActive:
		newState = StateActive
	default:
		newState = StateWarm
	}

	// Track turn count
	if isUserSpeaking {
		m.turnCount++
	}

	// Update state and fire callback if changed
	if newState != oldState {
		m.currentState = newState
		if m.onStateChange != nil {
			go m.onStateChange(oldState, newState)
		}
	} else {
		m.currentState = newState
	}

	return newState
}

// EndConversation explicitly ends the conversation (farewell).
func (m *ConversationStateManager) EndConversation() {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldState := m.currentState
	m.currentState = StateCold
	m.turnCount = 0
	m.sessionStart = time.Now()
	m.lastInteraction = time.Time{}

	if m.onStateChange != nil && oldState != StateCold {
		go m.onStateChange(oldState, StateCold)
	}
}

// GetTurnCount returns the number of conversational turns.
func (m *ConversationStateManager) GetTurnCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.turnCount
}

// GetSessionDuration returns how long the current session has been active.
func (m *ConversationStateManager) GetSessionDuration() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return time.Since(m.sessionStart)
}

// OnStateChange sets a callback for state transitions.
func (m *ConversationStateManager) OnStateChange(fn func(old, new ConversationState)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onStateChange = fn
}

// OnUIEvent sets a callback for UI synchronization.
func (m *ConversationStateManager) OnUIEvent(fn func(UIEvent)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onUIEvent = fn
}

// EmitUIEvent sends a UI event for visual feedback.
func (m *ConversationStateManager) EmitUIEvent(eventType UIEventType) {
	m.mu.RLock()
	callback := m.onUIEvent
	state := m.currentState
	m.mu.RUnlock()

	if callback != nil {
		callback(UIEvent{
			Type:      eventType,
			State:     state,
			Timestamp: time.Now(),
		})
	}
}

// IsFirstInteraction returns true if this is the first turn.
func (m *ConversationStateManager) IsFirstInteraction() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.turnCount == 0
}

// ShouldTriggerBackchannel checks if backchannel is appropriate (Risk A protection).
// Requires minimum speech duration and confidence threshold.
func (m *ConversationStateManager) ShouldTriggerBackchannel(speechDuration time.Duration, confidence float64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Only backchannel in Active state
	if m.currentState != StateActive {
		return false
	}

	// Risk A: Minimum duration to avoid false triggers
	if speechDuration < m.config.MinSpeechForBackchannel {
		return false
	}

	// Risk A: Confidence threshold
	if confidence < m.config.ConfidenceThreshold {
		return false
	}

	// Cooldown to avoid spam
	if time.Since(m.lastBackchannel) < m.config.BackchannelCooldown {
		return false
	}

	// Random chance to avoid predictability (every 5-8 seconds of speech)
	if speechDuration > 5*time.Second {
		return rand.Float64() < 0.3 // 30% chance
	}

	return false
}

// RecordBackchannel records that a backchannel was triggered.
func (m *ConversationStateManager) RecordBackchannel() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastBackchannel = time.Now()
}

// IsLowConfidence checks if we should respond with confusion.
func (m *ConversationStateManager) IsLowConfidence(confidence float64) bool {
	return confidence > 0 && confidence < m.config.ConfidenceThreshold
}

// GetFormality returns the appropriate formality level based on state and turn count.
func (m *ConversationStateManager) GetFormality() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	switch m.currentState {
	case StateCold:
		return "formal"
	case StateWarm:
		if m.turnCount < 3 {
			return "casual"
		}
		return "familiar"
	case StateActive:
		return "engaged"
	default:
		return "neutral"
	}
}

// StopAudioIfPlaying stops audio playback if currently playing (Risk C).
func (m *ConversationStateManager) StopAudioIfPlaying() bool {
	m.mu.RLock()
	player := m.audioPlayer
	m.mu.RUnlock()

	if player != nil && player.IsPlaying() {
		player.Stop()
		return true
	}
	return false
}
