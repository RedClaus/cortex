package e2e

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/normanking/cortexavatar/internal/audio"
	"github.com/normanking/cortexavatar/internal/stt"
	"github.com/normanking/cortexavatar/internal/tts"
	"github.com/normanking/cortexavatar/tests/testutil"
)

// TestVoicePipelineE2E tests the complete voice interaction cycle:
// Audio Input → VAD → STT → LLM (mocked) → TTS → Audio Output
func TestVoicePipelineE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Check if HF service is required
	requireHFService := os.Getenv("E2E_REQUIRE_HF_SERVICE") == "true"

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	// Create mock HF service
	mockService := testutil.CreateMockHFService(t)
	defer mockService.Close()

	// Initialize clients
	vadClient := audio.NewHFVADClient(mockService.URL, logger)
	sttProvider := stt.NewHFWhisperProvider(&stt.HFWhisperConfig{
		ServiceURL: mockService.URL,
		Timeout:    30,
		Language:   "en",
	}, logger)
	ttsProvider := tts.NewHFMeloProvider(&tts.HFMeloConfig{
		ServiceURL:   mockService.URL,
		Timeout:      30,
		DefaultVoice: "EN",
		DefaultSpeed: 1.0,
	}, logger)

	// Run the full pipeline
	t.Run("FullVoiceInteractionCycle", func(t *testing.T) {
		ctx := context.Background()
		startTime := time.Now()

		// Step 1: VAD - Voice Activity Detection
		t.Log("Step 1: Testing Voice Activity Detection...")
		vadStart := time.Now()

		audioData := testutil.GenerateTestAudio(t, 5*time.Second)
		vadResult, err := vadClient.DetectSpeech(ctx, audioData)
		vadLatency := time.Since(vadStart)

		require.NoError(t, err)
		require.NotNil(t, vadResult)
		assert.True(t, vadResult.IsSpeech, "Should detect speech in test audio")
		assert.Greater(t, vadResult.Confidence, 0.5, "Confidence should be > 0.5")

		t.Logf("✓ VAD completed in %v (confidence: %.2f)", vadLatency, vadResult.Confidence)

		// Step 2: STT - Speech to Text
		t.Log("Step 2: Testing Speech-to-Text transcription...")
		sttStart := time.Now()

		transcribeReq := &stt.TranscribeRequest{
			Audio:      audioData,
			Format:     "wav",
			SampleRate: 16000,
			Channels:   1,
			Language:   "en",
		}

		transcribeResp, err := sttProvider.Transcribe(ctx, transcribeReq)
		sttLatency := time.Since(sttStart)

		require.NoError(t, err)
		require.NotNil(t, transcribeResp)
		assert.NotEmpty(t, transcribeResp.Text, "Transcription should not be empty")
		assert.True(t, transcribeResp.IsFinal, "Transcription should be final")

		t.Logf("✓ STT completed in %v", sttLatency)
		t.Logf("  Transcribed text: %q", transcribeResp.Text)

		// Step 3: LLM Processing (Mocked)
		t.Log("Step 3: Testing LLM response generation (mocked)...")
		llmStart := time.Now()

		llmResponse := mockLLMResponse(transcribeResp.Text)
		llmLatency := time.Since(llmStart)

		assert.NotEmpty(t, llmResponse, "LLM response should not be empty")
		t.Logf("✓ LLM completed in %v", llmLatency)
		t.Logf("  LLM response: %q", llmResponse)

		// Step 4: TTS - Text to Speech
		t.Log("Step 4: Testing Text-to-Speech synthesis...")
		ttsStart := time.Now()

		synthesizeReq := &tts.SynthesizeRequest{
			Text:    llmResponse,
			VoiceID: "en",
			Speed:   1.0,
		}

		synthesizeResp, err := ttsProvider.Synthesize(ctx, synthesizeReq)
		ttsLatency := time.Since(ttsStart)

		require.NoError(t, err)
		require.NotNil(t, synthesizeResp)
		assert.NotEmpty(t, synthesizeResp.Audio, "Audio should not be empty")
		assert.Equal(t, "wav", synthesizeResp.Format)
		assert.Equal(t, 16000, synthesizeResp.SampleRate)

		t.Logf("✓ TTS completed in %v", ttsLatency)
		t.Logf("  Generated %d bytes of audio", len(synthesizeResp.Audio))

		// Calculate total latency
		totalLatency := time.Since(startTime)

		t.Log("\n=== E2E Pipeline Summary ===")
		t.Logf("VAD Latency:   %v", vadLatency)
		t.Logf("STT Latency:   %v", sttLatency)
		t.Logf("LLM Latency:   %v", llmLatency)
		t.Logf("TTS Latency:   %v", ttsLatency)
		t.Logf("Total Latency: %v", totalLatency)
		t.Log("===========================")

		// Verify latency targets (excluding LLM for mock)
		pipelineLatency := vadLatency + sttLatency + ttsLatency
		t.Logf("\nPipeline latency (without LLM): %v", pipelineLatency)

		// With mock HF service, we expect very fast responses
		// With real HF service, target is <2s total (excluding LLM)
		if !requireHFService {
			assert.Less(t, pipelineLatency.Seconds(), 1.0,
				"Mock pipeline should complete in <1s")
		} else {
			assert.Less(t, pipelineLatency.Seconds(), 2.0,
				"Real pipeline should complete in <2s (excluding LLM)")
		}
	})

	// Test error scenarios
	t.Run("ErrorScenarios", func(t *testing.T) {
		ctx := context.Background()

		t.Run("EmptyAudio", func(t *testing.T) {
			result, err := vadClient.DetectSpeech(ctx, []byte{})
			assert.Error(t, err)
			assert.Nil(t, result)
		})

		t.Run("InvalidAudioFormat", func(t *testing.T) {
			invalidAudio := []byte("not valid audio data")
			_, err := sttProvider.Transcribe(ctx, &stt.TranscribeRequest{
				Audio:      invalidAudio,
				Format:     "wav",
				SampleRate: 16000,
				Channels:   1,
				Language:   "en",
			})
			// Should handle gracefully (may or may not error depending on service)
			if err != nil {
				t.Logf("Expected error for invalid audio: %v", err)
			}
		})

		t.Run("TextTooLong", func(t *testing.T) {
			longText := string(make([]byte, 1000)) // 1000 characters
			_, err := ttsProvider.Synthesize(ctx, &tts.SynthesizeRequest{
				Text:    longText,
				VoiceID: "en",
				Speed:   1.0,
			})
			// Should handle gracefully or error appropriately
			if err != nil {
				t.Logf("Handled long text: %v", err)
			}
		})
	})
}

// TestVoicePipelineStreaming tests streaming TTS
func TestVoicePipelineStreaming(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	mockService := testutil.CreateMockHFService(t)
	defer mockService.Close()

	ttsProvider := tts.NewHFMeloProvider(&tts.HFMeloConfig{
		ServiceURL:   mockService.URL,
		Timeout:      30,
		DefaultVoice: "EN",
		DefaultSpeed: 1.0,
	}, logger)

	t.Run("StreamingTTS", func(t *testing.T) {
		ctx := context.Background()

		synthesizeReq := &tts.SynthesizeRequest{
			Text:    "This is a test of the streaming TTS system.",
			VoiceID: "en",
			Speed:   1.0,
		}

		audioChan, err := ttsProvider.SynthesizeStream(ctx, synthesizeReq)
		require.NoError(t, err)
		require.NotNil(t, audioChan)

		chunks := 0
		totalBytes := 0
		var lastChunk *tts.AudioChunk

		for chunk := range audioChan {
			chunks++
			totalBytes += len(chunk.Data)
			lastChunk = chunk
			t.Logf("Received chunk %d: %d bytes (final: %v)", chunk.Index, len(chunk.Data), chunk.IsFinal)
		}

		assert.Greater(t, chunks, 0, "Should receive at least one chunk")
		assert.Greater(t, totalBytes, 0, "Should receive audio data")

		if lastChunk != nil {
			assert.True(t, lastChunk.IsFinal, "Last chunk should be marked as final")
		}

		t.Logf("Received %d chunks, %d total bytes", chunks, totalBytes)
	})
}

// mockLLMResponse provides mock LLM responses for testing
func mockLLMResponse(input string) string {
	responses := map[string]string{
		"hello":     "Hello! How can I help you today?",
		"test":      "This is a test response from the mock LLM.",
		"weather":   "I'm sorry, I don't have access to weather information.",
		"default":   "I understand. How else can I assist you?",
	}

	// Simple keyword matching
	for keyword, response := range responses {
		if contains(input, keyword) {
			return response
		}
	}

	return responses["default"]
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		len(s) > len(substr)*2))
}
