package tts

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHFMeloProvider(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("with default config", func(t *testing.T) {
		provider := NewHFMeloProvider(nil, logger)

		assert.NotNil(t, provider)
		assert.Equal(t, "http://localhost:8899", provider.config.ServiceURL)
		assert.Equal(t, 30, provider.config.Timeout)
		assert.Equal(t, "EN", provider.config.DefaultVoice)
		assert.Equal(t, 1.0, provider.config.DefaultSpeed)
	})

	t.Run("with custom config", func(t *testing.T) {
		config := &HFMeloConfig{
			ServiceURL:   "http://custom:9000",
			Timeout:      60,
			DefaultVoice: "FR",
			DefaultSpeed: 1.5,
		}
		provider := NewHFMeloProvider(config, logger)

		assert.NotNil(t, provider)
		assert.Equal(t, "http://custom:9000", provider.config.ServiceURL)
		assert.Equal(t, 60, provider.config.Timeout)
		assert.Equal(t, "FR", provider.config.DefaultVoice)
		assert.Equal(t, 1.5, provider.config.DefaultSpeed)
	})
}

func TestHFMeloProvider_Name(t *testing.T) {
	logger := zerolog.Nop()
	provider := NewHFMeloProvider(nil, logger)

	assert.Equal(t, "hf_melo", provider.Name())
}

func TestHFMeloProvider_Health(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		responseBody   string
		wantErr        bool
	}{
		{
			name:           "service healthy",
			responseStatus: http.StatusOK,
			responseBody:   `{"status":"healthy","components":{"tts":"loaded"}}`,
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
			config := &HFMeloConfig{
				ServiceURL:   server.URL,
				Timeout:      5,
				DefaultVoice: "EN",
				DefaultSpeed: 1.0,
			}
			provider := NewHFMeloProvider(config, logger)

			err := provider.Health(context.Background())

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHFMeloProvider_Synthesize(t *testing.T) {
	tests := []struct {
		name           string
		request        *SynthesizeRequest
		responseStatus int
		responseBody   []byte
		wantErr        bool
	}{
		{
			name: "successful synthesis",
			request: &SynthesizeRequest{
				Text:    "Hello world",
				VoiceID: "en",
				Speed:   1.0,
			},
			responseStatus: http.StatusOK,
			responseBody:   []byte("fake wav audio data"),
			wantErr:        false,
		},
		{
			name: "synthesis with default speed",
			request: &SynthesizeRequest{
				Text:    "Bonjour",
				VoiceID: "fr",
				Speed:   0, // Should use default
			},
			responseStatus: http.StatusOK,
			responseBody:   []byte("fake wav audio data"),
			wantErr:        false,
		},
		{
			name: "service error",
			request: &SynthesizeRequest{
				Text:    "Test",
				VoiceID: "en",
				Speed:   1.0,
			},
			responseStatus: http.StatusInternalServerError,
			responseBody:   []byte(`{"error":"synthesis failed"}`),
			wantErr:        true,
		},
		{
			name: "text too long",
			request: &SynthesizeRequest{
				Text:    string(make([]byte, 1000)),
				VoiceID: "en",
				Speed:   1.0,
			},
			responseStatus: http.StatusBadRequest,
			responseBody:   []byte(`{"error":"text too long"}`),
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/tts", r.URL.Path)
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				// Verify request body
				bodyBytes, err := io.ReadAll(r.Body)
				require.NoError(t, err)

				var req map[string]interface{}
				err = json.Unmarshal(bodyBytes, &req)
				require.NoError(t, err)

				assert.Equal(t, tt.request.Text, req["text"])
				assert.NotEmpty(t, req["language"])

				w.WriteHeader(tt.responseStatus)
				w.Write(tt.responseBody)
			}))
			defer server.Close()

			logger := zerolog.Nop()
			config := &HFMeloConfig{
				ServiceURL:   server.URL,
				Timeout:      5,
				DefaultVoice: "EN",
				DefaultSpeed: 1.0,
			}
			provider := NewHFMeloProvider(config, logger)

			result, err := provider.Synthesize(context.Background(), tt.request)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.responseBody, result.Audio)
				assert.Equal(t, "wav", result.Format)
				assert.Equal(t, 16000, result.SampleRate)
			}
		})
	}
}

func TestHFMeloProvider_SynthesizeStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/tts", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		// Simulate chunked response
		w.Header().Set("Content-Type", "audio/wav")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		require.True(t, ok, "ResponseWriter should support flushing")

		// Write audio chunks
		chunks := [][]byte{
			[]byte("chunk1"),
			[]byte("chunk2"),
			[]byte("chunk3"),
		}

		for _, chunk := range chunks {
			w.Write(chunk)
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer server.Close()

	logger := zerolog.Nop()
	config := &HFMeloConfig{
		ServiceURL:   server.URL,
		Timeout:      5,
		DefaultVoice: "EN",
		DefaultSpeed: 1.0,
	}
	provider := NewHFMeloProvider(config, logger)

	req := &SynthesizeRequest{
		Text:    "Hello world",
		VoiceID: "en",
		Speed:   1.0,
	}

	audioChan, err := provider.SynthesizeStream(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, audioChan)

	chunks := []*AudioChunk{}
	for chunk := range audioChan {
		chunks = append(chunks, chunk)
	}

	assert.Greater(t, len(chunks), 0, "Should receive at least one chunk")

	// Last chunk should be marked as final
	if len(chunks) > 0 {
		assert.True(t, chunks[len(chunks)-1].IsFinal)
	}
}

func TestHFMeloProvider_Synthesize_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("audio data"))
	}))
	defer server.Close()

	logger := zerolog.Nop()
	config := &HFMeloConfig{
		ServiceURL:   server.URL,
		Timeout:      1, // 1 second timeout
		DefaultVoice: "EN",
		DefaultSpeed: 1.0,
	}
	provider := NewHFMeloProvider(config, logger)

	req := &SynthesizeRequest{
		Text:    "Hello",
		VoiceID: "en",
		Speed:   1.0,
	}

	result, err := provider.Synthesize(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestHFMeloProvider_Capabilities(t *testing.T) {
	logger := zerolog.Nop()
	provider := NewHFMeloProvider(nil, logger)

	caps := provider.Capabilities()

	assert.Equal(t, []string{"EN", "FR", "ES", "ZH", "JA", "KO"}, caps.SupportedLanguages)
	assert.Equal(t, 700, caps.AvgLatencyMs)
	assert.Equal(t, 500, caps.MaxTextLength)
	assert.True(t, caps.IsLocal)
	assert.True(t, caps.SupportsStreaming)
}

func TestHFMeloProvider_VoiceMapping(t *testing.T) {
	logger := zerolog.Nop()
	provider := NewHFMeloProvider(nil, logger)

	tests := []struct {
		voiceID  string
		expected string
	}{
		{"en", "EN"},
		{"EN", "EN"},
		{"fr", "FR"},
		{"FR", "FR"},
		{"es", "ES"},
		{"ES", "ES"},
		{"zh", "ZH"},
		{"ZH", "ZH"},
		{"ja", "JA"},
		{"JA", "JA"},
		{"ko", "KO"},
		{"KO", "KO"},
		{"unknown", "EN"}, // Should default to EN
	}

	for _, tt := range tests {
		t.Run(tt.voiceID, func(t *testing.T) {
			result := provider.mapVoiceToLanguage(tt.voiceID)
			assert.Equal(t, tt.expected, result)
		})
	}
}
