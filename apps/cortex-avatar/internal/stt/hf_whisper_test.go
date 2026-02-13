package stt

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHFWhisperProvider(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("with default config", func(t *testing.T) {
		provider := NewHFWhisperProvider(nil, logger)

		assert.NotNil(t, provider)
		assert.Equal(t, "http://localhost:8899", provider.config.ServiceURL)
		assert.Equal(t, 30, provider.config.Timeout)
		assert.Equal(t, "en", provider.config.Language)
	})

	t.Run("with custom config", func(t *testing.T) {
		config := &HFWhisperConfig{
			ServiceURL: "http://custom:9000",
			Timeout:    60,
			Language:   "fr",
		}
		provider := NewHFWhisperProvider(config, logger)

		assert.NotNil(t, provider)
		assert.Equal(t, "http://custom:9000", provider.config.ServiceURL)
		assert.Equal(t, 60, provider.config.Timeout)
		assert.Equal(t, "fr", provider.config.Language)
	})
}

func TestHFWhisperProvider_Name(t *testing.T) {
	logger := zerolog.Nop()
	provider := NewHFWhisperProvider(nil, logger)

	assert.Equal(t, "hf_whisper", provider.Name())
}

func TestHFWhisperProvider_Health(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		responseBody   string
		wantErr        bool
	}{
		{
			name:           "service healthy",
			responseStatus: http.StatusOK,
			responseBody:   `{"status":"healthy","components":{"stt":"loaded"}}`,
			wantErr:        false,
		},
		{
			name:           "service unavailable",
			responseStatus: http.StatusServiceUnavailable,
			responseBody:   `{"status":"unhealthy"}`,
			wantErr:        true,
		},
		{
			name:           "service error",
			responseStatus: http.StatusInternalServerError,
			responseBody:   `{"error":"internal error"}`,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/health", r.URL.Path)
				w.WriteHeader(tt.responseStatus)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			logger := zerolog.Nop()
			config := &HFWhisperConfig{
				ServiceURL: server.URL,
				Timeout:    5,
				Language:   "en",
			}
			provider := NewHFWhisperProvider(config, logger)

			err := provider.Health(context.Background())

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHFWhisperProvider_Transcribe(t *testing.T) {
	tests := []struct {
		name             string
		request          *TranscribeRequest
		responseStatus   int
		responseBody     string
		expectedText     string
		expectedLanguage string
		expectedConfidence float64
		wantErr          bool
	}{
		{
			name: "successful transcription",
			request: &TranscribeRequest{
				Audio:      []byte("fake audio data"),
				Format:     "wav",
				SampleRate: 16000,
				Channels:   1,
				Language:   "en",
			},
			responseStatus:   http.StatusOK,
			responseBody:     `{"text":"Hello world","language":"en","confidence":0.98,"processing_time_ms":450}`,
			expectedText:     "Hello world",
			expectedLanguage: "en",
			expectedConfidence: 0.98,
			wantErr:          false,
		},
		{
			name: "transcription with default language",
			request: &TranscribeRequest{
				Audio:      []byte("fake audio data"),
				Format:     "wav",
				SampleRate: 16000,
				Channels:   1,
				Language:   "", // Empty, should use default
			},
			responseStatus:   http.StatusOK,
			responseBody:     `{"text":"Bonjour","language":"fr","confidence":0.95,"processing_time_ms":500}`,
			expectedText:     "Bonjour",
			expectedLanguage: "fr",
			expectedConfidence: 0.95,
			wantErr:          false,
		},
		{
			name: "service error",
			request: &TranscribeRequest{
				Audio:      []byte("fake audio data"),
				Format:     "wav",
				SampleRate: 16000,
				Channels:   1,
				Language:   "en",
			},
			responseStatus: http.StatusInternalServerError,
			responseBody:   `{"error":"transcription failed"}`,
			wantErr:        true,
		},
		{
			name: "audio too short",
			request: &TranscribeRequest{
				Audio:      []byte{},
				Format:     "wav",
				SampleRate: 16000,
				Channels:   1,
				Language:   "en",
			},
			responseStatus: http.StatusBadRequest,
			responseBody:   `{"error":"audio too short"}`,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/stt", r.URL.Path)
				assert.Equal(t, "POST", r.Method)

				// Verify multipart form data
				err := r.ParseMultipartForm(10 << 20)
				require.NoError(t, err)

				if len(tt.request.Audio) > 0 {
					file, _, err := r.FormFile("audio")
					require.NoError(t, err)
					audioBytes, err := io.ReadAll(file)
					require.NoError(t, err)
					assert.Equal(t, tt.request.Audio, audioBytes)
				}

				w.WriteHeader(tt.responseStatus)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			logger := zerolog.Nop()
			config := &HFWhisperConfig{
				ServiceURL: server.URL,
				Timeout:    5,
				Language:   "en",
			}
			provider := NewHFWhisperProvider(config, logger)

			result, err := provider.Transcribe(context.Background(), tt.request)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.expectedText, result.Text)
				assert.Equal(t, tt.expectedLanguage, result.Language)
				assert.InDelta(t, tt.expectedConfidence, result.Confidence, 0.01)
				assert.True(t, result.IsFinal)
				assert.Greater(t, result.ProcessingTime, time.Duration(0))
			}
		})
	}
}

func TestHFWhisperProvider_Transcribe_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"text":"Hello","language":"en","confidence":0.9}`))
	}))
	defer server.Close()

	logger := zerolog.Nop()
	config := &HFWhisperConfig{
		ServiceURL: server.URL,
		Timeout:    1, // 1 second timeout
		Language:   "en",
	}
	provider := NewHFWhisperProvider(config, logger)

	req := &TranscribeRequest{
		Audio:      []byte("fake audio"),
		Format:     "wav",
		SampleRate: 16000,
		Channels:   1,
		Language:   "en",
	}

	result, err := provider.Transcribe(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestHFWhisperProvider_Capabilities(t *testing.T) {
	logger := zerolog.Nop()
	provider := NewHFWhisperProvider(nil, logger)

	caps := provider.Capabilities()

	assert.Equal(t, []string{"en", "fr", "es", "zh", "ja", "ko", "auto"}, caps.SupportedLanguages)
	assert.Equal(t, 500, caps.AvgLatencyMs)
	assert.Equal(t, 30, caps.MaxAudioLengthSec)
	assert.True(t, caps.IsLocal)
	assert.False(t, caps.SupportsStreaming)
}

func TestHFWhisperProvider_TranscribeStream(t *testing.T) {
	logger := zerolog.Nop()
	provider := NewHFWhisperProvider(nil, logger)

	// Create an audio stream channel
	audioStream := make(chan []byte, 1)
	audioStream <- []byte("fake audio chunk")
	close(audioStream)

	resultChan, err := provider.TranscribeStream(context.Background(), audioStream)

	// Streaming not supported by HF Whisper provider
	assert.Error(t, err)
	assert.Nil(t, resultChan)
	assert.Contains(t, err.Error(), "not supported")
}
