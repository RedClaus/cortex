// Package voice provides voice-related types and utilities for Cortex.
// conversation_state_test.go contains tests for the conversational state machine (CR-012-C).
package voice

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConversationStateManager_InitialState(t *testing.T) {
	config := DefaultStateConfig()
	mgr := NewConversationStateManager(config)

	assert.Equal(t, StateCold, mgr.GetState())
	assert.Equal(t, 0, mgr.GetTurnCount())
	assert.True(t, mgr.IsFirstInteraction())
}

func TestConversationStateManager_ColdToWarm(t *testing.T) {
	config := DefaultStateConfig()
	mgr := NewConversationStateManager(config)

	// Initial state should be Cold
	assert.Equal(t, StateCold, mgr.GetState())

	// After interaction (not user speaking), should transition to Warm
	newState := mgr.RecordInteraction(false)
	assert.Equal(t, StateWarm, newState)
	assert.Equal(t, StateWarm, mgr.GetState())
}

func TestConversationStateManager_WarmToActive(t *testing.T) {
	config := DefaultStateConfig()
	mgr := NewConversationStateManager(config)

	// Move to Warm
	mgr.RecordInteraction(false)
	assert.Equal(t, StateWarm, mgr.GetState())

	// User speaking should transition to Active
	newState := mgr.RecordInteraction(true)
	assert.Equal(t, StateActive, newState)
	assert.Equal(t, StateActive, mgr.GetState())
}

func TestConversationStateManager_TimeoutToWarm(t *testing.T) {
	config := StateConfig{
		WarmTimeout:   100 * time.Millisecond,
		ActiveTimeout: 50 * time.Millisecond,
	}
	mgr := NewConversationStateManager(config)

	// Move to Active
	mgr.RecordInteraction(false) // Cold -> Warm
	mgr.RecordInteraction(true)  // Warm -> Active
	assert.Equal(t, StateActive, mgr.GetState())

	// Wait for Active timeout
	time.Sleep(60 * time.Millisecond)
	assert.Equal(t, StateWarm, mgr.GetState())
}

func TestConversationStateManager_TimeoutToCold(t *testing.T) {
	config := StateConfig{
		WarmTimeout:   100 * time.Millisecond,
		ActiveTimeout: 50 * time.Millisecond,
	}
	mgr := NewConversationStateManager(config)

	// Move to Warm
	mgr.RecordInteraction(false)
	assert.Equal(t, StateWarm, mgr.GetState())

	// Wait for Warm timeout
	time.Sleep(150 * time.Millisecond)
	assert.Equal(t, StateCold, mgr.GetState())
}

func TestConversationStateManager_EndConversation(t *testing.T) {
	config := DefaultStateConfig()
	mgr := NewConversationStateManager(config)

	// Build up state
	mgr.RecordInteraction(false)
	mgr.RecordInteraction(true)
	assert.Equal(t, StateActive, mgr.GetState())
	assert.Equal(t, 1, mgr.GetTurnCount())

	// End conversation
	mgr.EndConversation()
	assert.Equal(t, StateCold, mgr.GetState())
	assert.Equal(t, 0, mgr.GetTurnCount())
	assert.True(t, mgr.IsFirstInteraction())
}

func TestConversationStateManager_TurnCount(t *testing.T) {
	config := DefaultStateConfig()
	mgr := NewConversationStateManager(config)

	assert.Equal(t, 0, mgr.GetTurnCount())

	// Non-user interaction shouldn't increment turn count
	mgr.RecordInteraction(false)
	assert.Equal(t, 0, mgr.GetTurnCount())

	// User speaking should increment turn count
	mgr.RecordInteraction(true)
	assert.Equal(t, 1, mgr.GetTurnCount())

	mgr.RecordInteraction(true)
	assert.Equal(t, 2, mgr.GetTurnCount())
}

func TestConversationStateManager_ShouldBackchannel_NotActive(t *testing.T) {
	config := DefaultStateConfig()
	mgr := NewConversationStateManager(config)

	// Should NOT backchannel in Cold state
	assert.False(t, mgr.ShouldTriggerBackchannel(1*time.Second, 0.9))

	// Move to Warm
	mgr.RecordInteraction(false)

	// Should NOT backchannel in Warm state
	assert.False(t, mgr.ShouldTriggerBackchannel(1*time.Second, 0.9))
}

func TestConversationStateManager_ShouldBackchannel_ShortSpeech(t *testing.T) {
	config := DefaultStateConfig()
	config.MinSpeechForBackchannel = 500 * time.Millisecond
	mgr := NewConversationStateManager(config)

	// Move to Active
	mgr.RecordInteraction(false)
	mgr.RecordInteraction(true)

	// Should NOT backchannel with short speech (Risk A)
	assert.False(t, mgr.ShouldTriggerBackchannel(100*time.Millisecond, 0.9))
}

func TestConversationStateManager_ShouldBackchannel_LowConfidence(t *testing.T) {
	config := DefaultStateConfig()
	config.ConfidenceThreshold = 0.6
	mgr := NewConversationStateManager(config)

	// Move to Active
	mgr.RecordInteraction(false)
	mgr.RecordInteraction(true)

	// Should NOT backchannel with low confidence (Risk A)
	assert.False(t, mgr.ShouldTriggerBackchannel(1*time.Second, 0.3))
}

func TestConversationStateManager_IsLowConfidence(t *testing.T) {
	config := DefaultStateConfig()
	config.ConfidenceThreshold = 0.6
	mgr := NewConversationStateManager(config)

	assert.True(t, mgr.IsLowConfidence(0.3))
	assert.True(t, mgr.IsLowConfidence(0.5))
	assert.False(t, mgr.IsLowConfidence(0.6))
	assert.False(t, mgr.IsLowConfidence(0.9))
	assert.False(t, mgr.IsLowConfidence(0)) // 0 is treated as "no confidence data"
}

func TestConversationStateManager_GetFormality(t *testing.T) {
	config := DefaultStateConfig()
	mgr := NewConversationStateManager(config)

	// Cold = formal
	assert.Equal(t, "formal", mgr.GetFormality())

	// Warm with few turns = casual
	mgr.RecordInteraction(false)
	assert.Equal(t, "casual", mgr.GetFormality())

	// Warm with many turns = familiar
	mgr.RecordInteraction(true)
	mgr.RecordInteraction(true)
	mgr.RecordInteraction(true)
	// Still in active state now
	assert.Equal(t, "engaged", mgr.GetFormality())
}

func TestConversationStateManager_StateChangeCallback(t *testing.T) {
	config := DefaultStateConfig()
	mgr := NewConversationStateManager(config)

	var oldState, newState ConversationState
	called := make(chan bool, 1)

	mgr.OnStateChange(func(old, new ConversationState) {
		oldState = old
		newState = new
		called <- true
	})

	mgr.RecordInteraction(false)

	select {
	case <-called:
		assert.Equal(t, StateCold, oldState)
		assert.Equal(t, StateWarm, newState)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("callback not called")
	}
}

func TestConversationStateManager_UIEventCallback(t *testing.T) {
	config := DefaultStateConfig()
	mgr := NewConversationStateManager(config)

	var receivedEvent UIEvent
	called := make(chan bool, 1)

	mgr.OnUIEvent(func(event UIEvent) {
		receivedEvent = event
		called <- true
	})

	mgr.EmitUIEvent(UIEventListening)

	select {
	case <-called:
		assert.Equal(t, UIEventListening, receivedEvent.Type)
		assert.Equal(t, StateCold, receivedEvent.State)
		assert.False(t, receivedEvent.Timestamp.IsZero())
	case <-time.After(100 * time.Millisecond):
		t.Fatal("callback not called")
	}
}

func TestConversationStateManager_InterruptionHandling(t *testing.T) {
	config := DefaultStateConfig()
	mgr := NewConversationStateManager(config)

	// Create mock audio player
	mockPlayer := &MockAudioPlayer{playing: true}
	mgr.SetAudioPlayer(mockPlayer)

	// User speaking while audio is playing should stop it
	mgr.RecordInteraction(true)

	assert.True(t, mockPlayer.stopCalled)
	assert.False(t, mockPlayer.playing)
}

func TestConversationStateManager_StopAudioIfPlaying(t *testing.T) {
	config := DefaultStateConfig()
	mgr := NewConversationStateManager(config)

	// No player set
	assert.False(t, mgr.StopAudioIfPlaying())

	// Player not playing
	mockPlayer := &MockAudioPlayer{playing: false}
	mgr.SetAudioPlayer(mockPlayer)
	assert.False(t, mgr.StopAudioIfPlaying())

	// Player is playing
	mockPlayer.playing = true
	assert.True(t, mgr.StopAudioIfPlaying())
	assert.True(t, mockPlayer.stopCalled)
}

func TestConversationStateManager_SessionDuration(t *testing.T) {
	config := DefaultStateConfig()
	mgr := NewConversationStateManager(config)

	time.Sleep(10 * time.Millisecond)

	duration := mgr.GetSessionDuration()
	require.True(t, duration >= 10*time.Millisecond)
}

// MockAudioPlayer implements AudioPlayerInterface for testing.
type MockAudioPlayer struct {
	playing    bool
	stopCalled bool
	playData   []byte
	callbacks  PlaybackCallbacks
}

func (m *MockAudioPlayer) Stop() {
	m.stopCalled = true
	m.playing = false
}

func (m *MockAudioPlayer) IsPlaying() bool {
	return m.playing
}

func (m *MockAudioPlayer) PlayBytes(data []byte) error {
	m.playData = data
	m.playing = true
	return nil
}

func (m *MockAudioPlayer) SetCallbacks(callbacks PlaybackCallbacks) {
	m.callbacks = callbacks
}
