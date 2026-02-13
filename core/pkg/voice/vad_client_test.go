package voice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVADModeConstants(t *testing.T) {
	// Verify VAD mode constants are correctly defined
	assert.Equal(t, VADMode("FULL"), VADModeFull)
	assert.Equal(t, VADMode("PLAYBACK"), VADModePlayback)
}

func TestDefaultVADClientConfig(t *testing.T) {
	config := DefaultVADClientConfig()

	assert.Equal(t, "ws://127.0.0.1:8880/v1/vad/stream", config.Endpoint)
	assert.Equal(t, "http://127.0.0.1:8880", config.HTTPBaseURL)
	assert.NotZero(t, config.ReconnectWait)
	assert.NotZero(t, config.MaxReconnects)
	assert.NotZero(t, config.PingInterval)
	assert.NotZero(t, config.HTTPTimeout)
}

func TestNewVADClient(t *testing.T) {
	config := DefaultVADClientConfig()
	client := NewVADClient(config)

	assert.NotNil(t, client)
	assert.Equal(t, config.Endpoint, client.config.Endpoint)
	assert.Equal(t, VADModeFull, client.currentMode)
	assert.NotNil(t, client.httpClient)
}

func TestVADClient_CurrentMode(t *testing.T) {
	config := DefaultVADClientConfig()
	client := NewVADClient(config)

	// Initial mode should be FULL
	assert.Equal(t, VADModeFull, client.CurrentMode())
}

func TestDeriveHTTPBaseURL(t *testing.T) {
	tests := []struct {
		name     string
		wsURL    string
		expected string
	}{
		{
			name:     "standard ws URL",
			wsURL:    "ws://127.0.0.1:8880/v1/vad/stream",
			expected: "http://127.0.0.1:8880",
		},
		{
			name:     "secure wss URL",
			wsURL:    "wss://example.com:8880/v1/vad/stream",
			expected: "https://example.com:8880",
		},
		{
			name:     "URL without path",
			wsURL:    "ws://localhost:8880",
			expected: "http://localhost:8880",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deriveHTTPBaseURL(tt.wsURL)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestVADClient_HandleEvent_Interrupt(t *testing.T) {
	config := DefaultVADClientConfig()
	client := NewVADClient(config)

	// Track if interrupt callback was called
	var receivedEvent VADEvent
	var receivedAudio []byte
	callbackCalled := false

	client.OnInterrupt = func(event VADEvent, audioData []byte) {
		callbackCalled = true
		receivedEvent = event
		receivedAudio = audioData
	}

	// Create an interrupt event
	event := VADEvent{
		Type:       "interrupt",
		Timestamp:  1234567890.123,
		Confidence: 0.92,
		DurationMs: 750,
	}

	// Call handleEvent directly (simulating server message)
	client.handleEvent(event)

	// Verify callback was called
	assert.True(t, callbackCalled, "OnInterrupt callback should be called")
	assert.Equal(t, "interrupt", receivedEvent.Type)
	assert.Equal(t, 0.92, receivedEvent.Confidence)
	assert.Equal(t, float64(750), receivedEvent.DurationMs)
	assert.Nil(t, receivedAudio) // No audio_base64 in this event
}

func TestVADClient_HandleEvent_InterruptWithAudio(t *testing.T) {
	config := DefaultVADClientConfig()
	client := NewVADClient(config)

	// Track callback invocation
	var receivedAudio []byte
	callbackCalled := false

	client.OnInterrupt = func(event VADEvent, audioData []byte) {
		callbackCalled = true
		receivedAudio = audioData
	}

	// Create an interrupt event with base64-encoded audio
	// "test audio" in base64 is "dGVzdCBhdWRpbw=="
	event := VADEvent{
		Type:        "interrupt",
		Timestamp:   1234567890.123,
		Confidence:  0.85,
		DurationMs:  500,
		AudioBase64: "dGVzdCBhdWRpbw==",
	}

	// Call handleEvent
	client.handleEvent(event)

	// Verify callback was called with decoded audio
	assert.True(t, callbackCalled)
	assert.Equal(t, []byte("test audio"), receivedAudio)
}

func TestVADClient_HandleEvent_SpeechStart(t *testing.T) {
	config := DefaultVADClientConfig()
	client := NewVADClient(config)

	var receivedEvent VADEvent
	callbackCalled := false

	client.OnSpeechStart = func(event VADEvent) {
		callbackCalled = true
		receivedEvent = event
	}

	event := VADEvent{
		Type:       "speech_start",
		Timestamp:  1234567890.0,
		Confidence: 0.88,
	}

	client.handleEvent(event)

	assert.True(t, callbackCalled)
	assert.Equal(t, "speech_start", receivedEvent.Type)
	assert.Equal(t, 0.88, receivedEvent.Confidence)
}

func TestVADClient_HandleEvent_SpeechEnd(t *testing.T) {
	config := DefaultVADClientConfig()
	client := NewVADClient(config)

	var receivedEvent VADEvent
	var receivedAudio []byte
	callbackCalled := false

	client.OnSpeechEnd = func(event VADEvent, audioData []byte) {
		callbackCalled = true
		receivedEvent = event
		receivedAudio = audioData
	}

	event := VADEvent{
		Type:        "speech_end",
		Timestamp:   1234567890.0,
		Confidence:  0.95,
		DurationMs:  1500,
		AudioBase64: "c29tZSBhdWRpbyBkYXRh", // "some audio data"
	}

	client.handleEvent(event)

	assert.True(t, callbackCalled)
	assert.Equal(t, "speech_end", receivedEvent.Type)
	assert.Equal(t, float64(1500), receivedEvent.DurationMs)
	assert.Equal(t, []byte("some audio data"), receivedAudio)
}

func TestVADClient_HandleEvent_UnknownType(t *testing.T) {
	config := DefaultVADClientConfig()
	client := NewVADClient(config)

	// Set all callbacks to track if any are called
	speechStartCalled := false
	speechEndCalled := false
	interruptCalled := false
	errorCalled := false

	client.OnSpeechStart = func(event VADEvent) { speechStartCalled = true }
	client.OnSpeechEnd = func(event VADEvent, audioData []byte) { speechEndCalled = true }
	client.OnInterrupt = func(event VADEvent, audioData []byte) { interruptCalled = true }
	client.OnError = func(err error) { errorCalled = true }

	// Unknown event type
	event := VADEvent{
		Type:      "unknown_event",
		Timestamp: 1234567890.0,
	}

	// Should not panic and should not call any callbacks
	client.handleEvent(event)

	assert.False(t, speechStartCalled)
	assert.False(t, speechEndCalled)
	assert.False(t, interruptCalled)
	assert.False(t, errorCalled)
}

func TestVADClient_IsConnected_NotConnected(t *testing.T) {
	config := DefaultVADClientConfig()
	client := NewVADClient(config)

	// Should not be connected when just created
	assert.False(t, client.IsConnected())
}
