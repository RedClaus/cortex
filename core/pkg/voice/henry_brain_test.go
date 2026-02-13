package voice

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockAudioPlayerForBrain implements AudioPlayerInterface for testing HenryBrain.
type MockAudioPlayerForBrain struct {
	playedBytes   [][]byte
	stopCalled    bool
	isPlayingFlag bool
	callbacks     PlaybackCallbacks
}

func (m *MockAudioPlayerForBrain) Stop() {
	m.stopCalled = true
	m.isPlayingFlag = false
}

func (m *MockAudioPlayerForBrain) IsPlaying() bool {
	return m.isPlayingFlag
}

func (m *MockAudioPlayerForBrain) PlayBytes(data []byte) error {
	m.playedBytes = append(m.playedBytes, data)
	return nil
}

func (m *MockAudioPlayerForBrain) SetCallbacks(callbacks PlaybackCallbacks) {
	m.callbacks = callbacks
}

// MockTTSGeneratorForBrain implements TTSGenerator for testing.
type MockTTSGeneratorForBrain struct {
	synthesizedTexts []string
}

func (m *MockTTSGeneratorForBrain) SynthesizeToFile(ctx context.Context, text, outputPath, voiceID string) error {
	m.synthesizedTexts = append(m.synthesizedTexts, text)
	return nil
}

func TestNewHenryBrain(t *testing.T) {
	config := DefaultHenryBrainConfig()
	brain, err := NewHenryBrain(config, nil, nil)

	require.NoError(t, err)
	assert.NotNil(t, brain)
	assert.NotNil(t, brain.stateManager)
	assert.NotNil(t, brain.responsePool)
	assert.NotNil(t, brain.audioCache)
}

func TestHenryBrain_HandleWakeWord(t *testing.T) {
	player := &MockAudioPlayerForBrain{}
	config := DefaultHenryBrainConfig()
	brain, err := NewHenryBrain(config, nil, player)
	require.NoError(t, err)

	// Initial state should be Cold
	assert.Equal(t, StateCold, brain.GetState())

	// Handle wake word
	ctx := context.Background()
	response, err := brain.HandleWakeWord(ctx)

	require.NoError(t, err)
	assert.NotEmpty(t, response)

	// State should transition (recorded interaction)
	assert.NotEqual(t, StateCold, brain.GetState())
}

func TestHenryBrain_HandleWakeWord_MultipleTimes(t *testing.T) {
	player := &MockAudioPlayerForBrain{}
	config := DefaultHenryBrainConfig()
	brain, err := NewHenryBrain(config, nil, player)
	require.NoError(t, err)

	ctx := context.Background()

	// First wake word - Cold -> Warm
	resp1, _ := brain.HandleWakeWord(ctx)
	state1 := brain.GetState()
	assert.NotEmpty(t, resp1)

	// Second wake word - stays Warm or -> Active
	resp2, _ := brain.HandleWakeWord(ctx)
	state2 := brain.GetState()
	assert.NotEmpty(t, resp2)

	// States should have progressed
	assert.NotEqual(t, StateCold, state1)
	assert.NotEqual(t, StateCold, state2)
}

func TestHenryBrain_HandleFarewell(t *testing.T) {
	player := &MockAudioPlayerForBrain{}
	config := DefaultHenryBrainConfig()
	brain, err := NewHenryBrain(config, nil, player)
	require.NoError(t, err)

	ctx := context.Background()

	// First have some interaction
	brain.HandleWakeWord(ctx)
	assert.NotEqual(t, StateCold, brain.GetState())

	// Handle farewell
	response, err := brain.HandleFarewell(ctx)

	require.NoError(t, err)
	assert.NotEmpty(t, response)

	// State should be Cold after farewell
	assert.Equal(t, StateCold, brain.GetState())
}

func TestHenryBrain_HandleUserSpeechStart_StopsPlayback(t *testing.T) {
	player := &MockAudioPlayerForBrain{isPlayingFlag: true}
	config := DefaultHenryBrainConfig()
	brain, err := NewHenryBrain(config, nil, player)
	require.NoError(t, err)

	// Simulate user starting to speak (barge-in)
	brain.HandleUserSpeechStart()

	// Player should have been stopped
	assert.True(t, player.stopCalled, "Player should be stopped on user speech start (barge-in)")
}

func TestHenryBrain_HandleLowConfidenceSpeech(t *testing.T) {
	player := &MockAudioPlayerForBrain{}
	config := DefaultHenryBrainConfig()
	config.ConfidenceThreshold = 0.6
	brain, err := NewHenryBrain(config, nil, player)
	require.NoError(t, err)

	ctx := context.Background()

	// Low confidence (below threshold)
	response, err := brain.HandleLowConfidenceSpeech(ctx, 0.3)
	require.NoError(t, err)
	assert.NotEmpty(t, response, "Should get confused response for low confidence")

	// High confidence (above threshold)
	response2, err := brain.HandleLowConfidenceSpeech(ctx, 0.8)
	require.NoError(t, err)
	assert.Empty(t, response2, "Should not get response for high confidence")
}

func TestHenryBrain_ShouldBackchannel(t *testing.T) {
	player := &MockAudioPlayerForBrain{}
	config := DefaultHenryBrainConfig()
	config.MinSpeechForBackchannel = 500 * time.Millisecond
	config.ConfidenceThreshold = 0.6
	brain, err := NewHenryBrain(config, nil, player)
	require.NoError(t, err)

	// Short speech, low confidence - should not backchannel (Risk A)
	assert.False(t, brain.ShouldBackchannel(100*time.Millisecond, 0.3))

	// Must be in Active state for backchanneling to work
	// Backchannel requires: Active state + speech > 5s + high confidence + random chance
	// With random chance (30%), we can't guarantee true, so just verify it doesn't crash
	// and the false cases work correctly

	// Short speech, high confidence - should not (speech too short)
	assert.False(t, brain.ShouldBackchannel(100*time.Millisecond, 0.9))

	// Even with correct conditions, state must be Active (which it isn't by default)
	assert.False(t, brain.ShouldBackchannel(6*time.Second, 0.9))
}

func TestHenryBrain_GetConversationContext(t *testing.T) {
	player := &MockAudioPlayerForBrain{}
	config := DefaultHenryBrainConfig()
	brain, err := NewHenryBrain(config, nil, player)
	require.NoError(t, err)

	ctx := brain.GetConversationContext()

	assert.NotNil(t, ctx)
	assert.Contains(t, ctx, "state")
	assert.Contains(t, ctx, "turn_count")
	assert.Contains(t, ctx, "is_first_turn")
	assert.Contains(t, ctx, "formality")
}

func TestHenryBrain_GetFormality(t *testing.T) {
	player := &MockAudioPlayerForBrain{}
	config := DefaultHenryBrainConfig()
	brain, err := NewHenryBrain(config, nil, player)
	require.NoError(t, err)

	// Initial state should be formal
	formality := brain.GetFormality()
	assert.NotEmpty(t, formality)
}

func TestHenryBrain_OnUIEvent(t *testing.T) {
	player := &MockAudioPlayerForBrain{}
	config := DefaultHenryBrainConfig()
	brain, err := NewHenryBrain(config, nil, player)
	require.NoError(t, err)

	var receivedEvent UIEvent
	brain.OnUIEvent(func(event UIEvent) {
		receivedEvent = event
	})

	ctx := context.Background()
	brain.HandleWakeWord(ctx)

	// Should have received UI event
	// Note: The event might be async, so we wait briefly
	time.Sleep(10 * time.Millisecond)
	assert.NotEqual(t, UIEvent{}, receivedEvent)
}

func TestHenryBrain_OnStateChange(t *testing.T) {
	player := &MockAudioPlayerForBrain{}
	config := DefaultHenryBrainConfig()
	brain, err := NewHenryBrain(config, nil, player)
	require.NoError(t, err)

	// Use channel to safely pass state change data
	stateChangeChan := make(chan struct {
		old ConversationState
		new ConversationState
	}, 1)

	brain.OnStateChange(func(old, new ConversationState) {
		stateChangeChan <- struct {
			old ConversationState
			new ConversationState
		}{old, new}
	})

	ctx := context.Background()
	brain.HandleWakeWord(ctx)

	// Wait for state change callback
	select {
	case change := <-stateChangeChan:
		// oldState was Cold, newState should be different
		assert.Equal(t, StateCold, change.old)
		assert.NotEqual(t, StateCold, change.new)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timed out waiting for state change callback")
	}
}

func TestHenryBrain_GetCacheStats(t *testing.T) {
	player := &MockAudioPlayerForBrain{}
	config := DefaultHenryBrainConfig()
	config.CacheDir = t.TempDir() // Use temp dir for test
	brain, err := NewHenryBrain(config, nil, player)
	require.NoError(t, err)

	fileCount, memSize, diskSize := brain.GetCacheStats()

	// Initial cache should be empty
	assert.Equal(t, 0, fileCount)
	assert.Equal(t, int64(0), memSize)
	assert.Equal(t, int64(0), diskSize)
}

func TestHenryBrain_HandleAcknowledge(t *testing.T) {
	player := &MockAudioPlayerForBrain{}
	config := DefaultHenryBrainConfig()
	brain, err := NewHenryBrain(config, nil, player)
	require.NoError(t, err)

	ctx := context.Background()
	response, err := brain.HandleAcknowledge(ctx)

	require.NoError(t, err)
	assert.NotEmpty(t, response)
}

func TestHenryBrain_Backchannel(t *testing.T) {
	player := &MockAudioPlayerForBrain{}
	config := DefaultHenryBrainConfig()
	brain, err := NewHenryBrain(config, nil, player)
	require.NoError(t, err)

	response := brain.Backchannel()
	assert.NotEmpty(t, response)
}

func TestHenryBrain_PlaybackCallbacksRegistered(t *testing.T) {
	player := &MockAudioPlayerForBrain{}
	config := DefaultHenryBrainConfig()
	config.VAD.Enabled = true
	brain, err := NewHenryBrain(config, nil, player)
	require.NoError(t, err)

	// Initialize VAD which should set up playback callbacks
	ctx := context.Background()
	_ = brain.InitializeVAD(ctx) // Will fail to connect but callbacks should be set

	// Verify callbacks were registered
	assert.NotNil(t, player.callbacks.OnPlaybackStart, "OnPlaybackStart callback should be registered")
	assert.NotNil(t, player.callbacks.OnPlaybackEnd, "OnPlaybackEnd callback should be registered")
}

func TestHenryBrain_HandleVADInterrupt(t *testing.T) {
	player := &MockAudioPlayerForBrain{isPlayingFlag: true}
	config := DefaultHenryBrainConfig()
	config.VAD.Enabled = true
	brain, err := NewHenryBrain(config, nil, player)
	require.NoError(t, err)

	// Track if speech detected callback was called
	var receivedAudio []byte
	var receivedDuration int
	brain.OnSpeechDetected(func(audioData []byte, durationMs int) {
		receivedAudio = audioData
		receivedDuration = durationMs
	})

	// Simulate an interrupt event
	event := VADEvent{
		Type:       "interrupt",
		Confidence: 0.95,
		DurationMs: 500,
	}
	testAudio := []byte("test audio data")

	// Call the interrupt handler directly
	brain.handleVADInterrupt(event, testAudio)

	// Verify playback was stopped
	assert.True(t, player.stopCalled, "Audio player should be stopped on interrupt")
	assert.False(t, player.isPlayingFlag, "Player should not be playing after stop")

	// Wait briefly for async callback
	time.Sleep(20 * time.Millisecond)

	// Verify speech detected callback was called with the interrupt audio
	assert.Equal(t, testAudio, receivedAudio, "Interrupt audio should be passed to speech callback")
	assert.Equal(t, 500, receivedDuration, "Duration should match event duration")
}

func TestHenryBrain_VADModesExist(t *testing.T) {
	// Verify VAD mode constants are correctly defined
	assert.Equal(t, VADMode("FULL"), VADModeFull)
	assert.Equal(t, VADMode("PLAYBACK"), VADModePlayback)
}
