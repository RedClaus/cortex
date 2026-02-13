package voice

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ══════════════════════════════════════════════════════════════════════════════
// Test Helpers
// ══════════════════════════════════════════════════════════════════════════════

// mockTTSServer creates a mock TTS server that returns dummy audio data.
func mockTTSServer(t *testing.T, latency time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}

		if r.URL.Path == "/v1/audio/speech" {
			// Simulate synthesis latency
			if latency > 0 {
				time.Sleep(latency)
			}

			// Parse request to validate format
			var req ttsRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Logf("Failed to decode request: %v", err)
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}

			// Return dummy WAV data (minimal valid WAV header + silence)
			wavHeader := []byte{
				'R', 'I', 'F', 'F', // ChunkID
				0x24, 0x00, 0x00, 0x00, // ChunkSize
				'W', 'A', 'V', 'E', // Format
				'f', 'm', 't', ' ', // Subchunk1ID
				0x10, 0x00, 0x00, 0x00, // Subchunk1Size
				0x01, 0x00, // AudioFormat (PCM)
				0x01, 0x00, // NumChannels
				0x22, 0x56, 0x00, 0x00, // SampleRate (22050)
				0x44, 0xAC, 0x00, 0x00, // ByteRate
				0x02, 0x00, // BlockAlign
				0x10, 0x00, // BitsPerSample
				'd', 'a', 't', 'a', // Subchunk2ID
				0x00, 0x00, 0x00, 0x00, // Subchunk2Size (empty)
			}

			w.Header().Set("Content-Type", "audio/wav")
			w.WriteHeader(http.StatusOK)
			w.Write(wavHeader)
			return
		}

		http.NotFound(w, r)
	}))
}

// ══════════════════════════════════════════════════════════════════════════════
// TTSEngine Tests
// ══════════════════════════════════════════════════════════════════════════════

func TestNewTTSEngine(t *testing.T) {
	config := DefaultTTSConfig()
	engine := NewTTSEngine(config)
	defer engine.Stop()

	assert.NotNil(t, engine)
	assert.Equal(t, TTSStateIdle, engine.State())
	assert.Equal(t, config.VoiceID, engine.config.VoiceID)
}

func TestTTSEngine_DefaultConfig(t *testing.T) {
	config := DefaultTTSConfig()

	assert.Equal(t, "http://localhost:8880/v1/audio/speech", config.Endpoint)
	assert.Equal(t, "am_adam", config.VoiceID)
	assert.Equal(t, "kokoro", config.Model)
	assert.Equal(t, "wav", config.ResponseFormat)
	assert.Equal(t, 1.0, config.Speed)
	assert.Equal(t, 60*time.Second, config.Timeout) // Longer timeout for first model download
}

func TestTTSEngine_Speak_NonBlocking(t *testing.T) {
	server := mockTTSServer(t, 100*time.Millisecond)
	defer server.Close()

	config := DefaultTTSConfig()
	config.Endpoint = server.URL + "/v1/audio/speech"

	engine := NewTTSEngine(config)
	defer engine.Stop()

	// Speak should return immediately (non-blocking)
	start := time.Now()
	err := engine.Speak("Hello, world!")
	elapsed := time.Since(start)

	require.NoError(t, err)
	// Should return quickly, not wait for synthesis
	assert.Less(t, elapsed, 50*time.Millisecond, "Speak should be non-blocking")
	// Note: Queue length check removed - the background worker may have already
	// picked up the job by the time we check. The non-blocking behavior is the
	// key test here.
}

func TestTTSEngine_SpeakSync_Blocking(t *testing.T) {
	server := mockTTSServer(t, 50*time.Millisecond)
	defer server.Close()

	config := DefaultTTSConfig()
	config.Endpoint = server.URL + "/v1/audio/speech"

	engine := NewTTSEngine(config)
	defer engine.Stop()

	// SpeakSync should block until complete
	ctx := context.Background()
	start := time.Now()
	err := engine.SpeakSync(ctx, "Hello, world!")
	elapsed := time.Since(start)

	require.NoError(t, err)
	// Should take at least the simulated latency
	assert.GreaterOrEqual(t, elapsed, 50*time.Millisecond, "SpeakSync should block")
}

func TestTTSEngine_SpeakSync_Cancellation(t *testing.T) {
	server := mockTTSServer(t, 500*time.Millisecond)
	defer server.Close()

	config := DefaultTTSConfig()
	config.Endpoint = server.URL + "/v1/audio/speech"

	engine := NewTTSEngine(config)
	defer engine.Stop()

	// Cancel context quickly
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := engine.SpeakSync(ctx, "This should be cancelled")

	// Should error due to cancellation
	assert.Error(t, err)
}

func TestTTSEngine_StopSpeaking_DrainsQueue(t *testing.T) {
	server := mockTTSServer(t, 200*time.Millisecond)
	defer server.Close()

	config := DefaultTTSConfig()
	config.Endpoint = server.URL + "/v1/audio/speech"

	engine := NewTTSEngine(config)
	defer engine.Stop()

	// Queue multiple items
	for i := 0; i < 5; i++ {
		err := engine.Speak("Message " + string(rune('A'+i)))
		require.NoError(t, err)
	}

	// Wait a moment for queue to fill
	time.Sleep(10 * time.Millisecond)
	initialQueueLen := engine.QueueLength()
	assert.Greater(t, initialQueueLen, 0, "Queue should have items")

	// Stop speaking (barge-in)
	engine.StopSpeaking()

	// Queue should be drained
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 0, engine.QueueLength(), "Queue should be drained after StopSpeaking")
	assert.Equal(t, TTSStatePaused, engine.State())
}

func TestTTSEngine_Resume_AfterInterrupt(t *testing.T) {
	config := DefaultTTSConfig()
	engine := NewTTSEngine(config)
	defer engine.Stop()

	// Stop speaking first
	engine.StopSpeaking()
	assert.Equal(t, TTSStatePaused, engine.State())
	assert.True(t, engine.interrupted.Load())

	// Resume
	engine.Resume()
	assert.Equal(t, TTSStateIdle, engine.State())
	assert.False(t, engine.interrupted.Load())
}

func TestTTSEngine_IsAvailable_Success(t *testing.T) {
	server := mockTTSServer(t, 0)
	defer server.Close()

	config := DefaultTTSConfig()
	config.Endpoint = server.URL + "/v1/audio/speech"

	engine := NewTTSEngine(config)
	defer engine.Stop()

	available := engine.IsAvailable()
	assert.True(t, available)
}

func TestTTSEngine_IsAvailable_Failure(t *testing.T) {
	config := DefaultTTSConfig()
	config.Endpoint = "http://localhost:99999/v1/audio/speech" // Invalid port

	engine := NewTTSEngine(config)
	defer engine.Stop()

	available := engine.IsAvailable()
	assert.False(t, available)
}

func TestTTSEngine_Stop_GracefulShutdown(t *testing.T) {
	server := mockTTSServer(t, 10*time.Millisecond)
	defer server.Close()

	config := DefaultTTSConfig()
	config.Endpoint = server.URL + "/v1/audio/speech"

	engine := NewTTSEngine(config)

	// Queue some work
	err := engine.Speak("Test message")
	require.NoError(t, err)

	// Stop should complete without hanging
	done := make(chan struct{})
	go func() {
		engine.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Good - shutdown completed
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() timed out - possible deadlock")
	}

	assert.Equal(t, TTSStateStopped, engine.State())
}

func TestTTSEngine_Speak_AfterStop_ReturnsError(t *testing.T) {
	config := DefaultTTSConfig()
	engine := NewTTSEngine(config)
	engine.Stop()

	err := engine.Speak("This should fail")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stopped")
}

func TestTTSEngine_Metrics(t *testing.T) {
	server := mockTTSServer(t, 10*time.Millisecond)
	defer server.Close()

	config := DefaultTTSConfig()
	config.Endpoint = server.URL + "/v1/audio/speech"

	engine := NewTTSEngine(config)
	defer engine.Stop()

	// Initial metrics
	metrics := engine.Metrics()
	assert.Equal(t, int64(0), metrics.SynthesisCount)
	assert.Equal(t, "idle", metrics.State)

	// Do some synthesis
	ctx := context.Background()
	err := engine.SpeakSync(ctx, "Test 1")
	require.NoError(t, err)

	err = engine.SpeakSync(ctx, "Test 2")
	require.NoError(t, err)

	// Check updated metrics
	metrics = engine.Metrics()
	assert.Equal(t, int64(2), metrics.SynthesisCount)
	assert.Equal(t, int64(2), metrics.PlaybackCount)
	assert.Equal(t, int64(0), metrics.SynthesisErrors)
}

func TestTTSEngine_EmptyText_NoOp(t *testing.T) {
	config := DefaultTTSConfig()
	engine := NewTTSEngine(config)
	defer engine.Stop()

	// Empty text should be a no-op
	err := engine.Speak("")
	assert.NoError(t, err)
	assert.Equal(t, 0, engine.QueueLength())

	ctx := context.Background()
	err = engine.SpeakSync(ctx, "")
	assert.NoError(t, err)
}

func TestTTSEngine_SpeakWithVoice(t *testing.T) {
	server := mockTTSServer(t, 10*time.Millisecond)
	defer server.Close()

	config := DefaultTTSConfig()
	config.Endpoint = server.URL + "/v1/audio/speech"

	engine := NewTTSEngine(config)
	defer engine.Stop()

	// Use a different voice
	err := engine.SpeakWithVoice("Hello", "bf_emma", 0.9)
	require.NoError(t, err)
}

func TestTTSEngine_ConcurrentSpeak(t *testing.T) {
	server := mockTTSServer(t, 5*time.Millisecond)
	defer server.Close()

	config := DefaultTTSConfig()
	config.Endpoint = server.URL + "/v1/audio/speech"
	config.QueueSize = 100

	engine := NewTTSEngine(config)
	defer engine.Stop()

	// Concurrent speaks should not race
	var wg sync.WaitGroup
	var errors atomic.Int32

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			if err := engine.Speak("Concurrent message"); err != nil {
				errors.Add(1)
			}
		}(i)
	}

	wg.Wait()
	assert.Equal(t, int32(0), errors.Load(), "No errors should occur during concurrent speaks")
}

func TestTTSEngine_QueueFull_ReturnsError(t *testing.T) {
	server := mockTTSServer(t, 500*time.Millisecond) // Slow server
	defer server.Close()

	config := DefaultTTSConfig()
	config.Endpoint = server.URL + "/v1/audio/speech"
	config.QueueSize = 2 // Very small queue

	engine := NewTTSEngine(config)
	defer engine.Stop()

	// Fill the queue
	var overflowErr error
	for i := 0; i < 10; i++ {
		if err := engine.Speak("Message"); err != nil {
			overflowErr = err
			break
		}
	}

	assert.Error(t, overflowErr, "Should get error when queue is full")
	assert.Contains(t, overflowErr.Error(), "queue full")
}

func TestTTSEngine_WithPlaybackFunc(t *testing.T) {
	server := mockTTSServer(t, 10*time.Millisecond)
	defer server.Close()

	config := DefaultTTSConfig()
	config.Endpoint = server.URL + "/v1/audio/speech"

	engine := NewTTSEngine(config)
	defer engine.Stop()

	// Track playback calls
	var playbackCalls atomic.Int32
	var lastFormat AudioFormat

	engine.SetPlaybackFunc(func(audio []byte, format AudioFormat) error {
		playbackCalls.Add(1)
		lastFormat = format
		return nil
	})

	ctx := context.Background()
	err := engine.SpeakSync(ctx, "Test with playback")
	require.NoError(t, err)

	assert.Equal(t, int32(1), playbackCalls.Load())
	assert.Equal(t, AudioFormat("wav"), lastFormat)
}

// ══════════════════════════════════════════════════════════════════════════════
// VoicePersona Tests
// ══════════════════════════════════════════════════════════════════════════════

func TestGetPersona_Exists(t *testing.T) {
	persona := GetPersona("henry")

	assert.Equal(t, "henry", persona.Name)
	assert.Equal(t, "am_adam", persona.VoiceID)
	assert.Equal(t, 1.0, persona.Speed)
	assert.Contains(t, persona.Traits, "authoritative")
}

func TestGetPersona_NotFound_ReturnsDefault(t *testing.T) {
	persona := GetPersona("nonexistent")

	assert.Equal(t, "henry", persona.Name, "Should return default persona")
}

func TestGetPersonaOrNil_Exists(t *testing.T) {
	persona := GetPersonaOrNil("ada")

	require.NotNil(t, persona)
	assert.Equal(t, "ada", persona.Name)
	assert.Equal(t, "af_sarah", persona.VoiceID)
}

func TestGetPersonaOrNil_NotFound(t *testing.T) {
	persona := GetPersonaOrNil("nonexistent")

	assert.Nil(t, persona)
}

func TestListPersonas(t *testing.T) {
	names := ListPersonas()

	assert.GreaterOrEqual(t, len(names), 3, "Should have at least henry, ada, nexus")
	assert.Contains(t, names, "henry")
	assert.Contains(t, names, "ada")
	assert.Contains(t, names, "nexus")
}

func TestListPersonaDetails(t *testing.T) {
	personas := ListPersonaDetails()

	assert.GreaterOrEqual(t, len(personas), 3)

	// Each persona should have required fields
	for _, p := range personas {
		assert.NotEmpty(t, p.Name)
		assert.NotEmpty(t, p.VoiceID)
		assert.NotEmpty(t, p.Description)
		assert.NotEmpty(t, p.Traits)
	}
}

func TestValidatePersona(t *testing.T) {
	assert.True(t, ValidatePersona("henry"))
	assert.True(t, ValidatePersona("ada"))
	assert.True(t, ValidatePersona("nexus"))
	assert.False(t, ValidatePersona("nonexistent"))
}

func TestPersonaForVoice(t *testing.T) {
	persona := PersonaForVoice("am_adam")

	require.NotNil(t, persona)
	assert.Equal(t, "henry", persona.Name)

	// Non-existent voice
	persona = PersonaForVoice("nonexistent_voice")
	assert.Nil(t, persona)
}

func TestVoicePersona_GetTraitsPrompt(t *testing.T) {
	persona := GetPersona("henry")
	prompt := persona.GetTraitsPrompt()

	assert.NotEmpty(t, prompt)
	assert.Contains(t, prompt, "authoritative")
	assert.Contains(t, prompt, "manner")

	// Empty traits
	emptyPersona := VoicePersona{Name: "empty"}
	assert.Empty(t, emptyPersona.GetTraitsPrompt())
}

func TestBuiltInPersonas_VoiceIDs(t *testing.T) {
	// Verify all personas use valid Kokoro voice IDs
	validVoices := map[string]bool{
		"am_adam":    true,
		"am_michael": true,
		"af_sarah":   true,
		"af_nicole":  true,
		"bf_emma":    true,
		"bm_george":  true,
		"af_bella":   true,
	}

	for name, persona := range BuiltInPersonas {
		assert.True(t, validVoices[persona.VoiceID],
			"Persona %s uses invalid voice ID: %s", name, persona.VoiceID)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// Integration Tests
// ══════════════════════════════════════════════════════════════════════════════

func TestNewTTSEngineWithPersona(t *testing.T) {
	server := mockTTSServer(t, 10*time.Millisecond)
	defer server.Close()

	engine := NewTTSEngineWithPersona("ada", server.URL+"/v1/audio/speech")
	defer engine.Stop()

	assert.Equal(t, "af_sarah", engine.config.VoiceID)
	assert.Equal(t, 0.95, engine.config.Speed)
}

func TestSpeakWithPersona(t *testing.T) {
	server := mockTTSServer(t, 10*time.Millisecond)
	defer server.Close()

	ctx := context.Background()
	audio, err := SpeakWithPersona(ctx, "Test message", "nexus", server.URL+"/v1/audio/speech")

	require.NoError(t, err)
	assert.NotEmpty(t, audio)
}
