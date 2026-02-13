package audio

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

func TestNewHFVADClient(t *testing.T) {
	logger := zerolog.Nop()
	client := NewHFVADClient("http://localhost:8899", logger)

	assert.NotNil(t, client)
	assert.Equal(t, "http://localhost:8899", client.serviceURL)
	assert.NotNil(t, client.httpClient)
	assert.Equal(t, 10*time.Second, client.httpClient.Timeout)
}

func TestHFVADClient_Health(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		responseBody   string
		wantErr        bool
	}{
		{
			name:           "service healthy",
			responseStatus: http.StatusOK,
			responseBody:   `{"status":"healthy","components":{"vad":"loaded"}}`,
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
			// Create mock HTTP server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/health", r.URL.Path)
				w.WriteHeader(tt.responseStatus)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			logger := zerolog.Nop()
			client := NewHFVADClient(server.URL, logger)

			err := client.Health(context.Background())

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHFVADClient_DetectSpeech(t *testing.T) {
	tests := []struct {
		name             string
		audioData        []byte
		responseStatus   int
		responseBody     string
		expectedSpeech   bool
		expectedConfidence float64
		wantErr          bool
	}{
		{
			name:           "speech detected",
			audioData:      []byte("fake audio data"),
			responseStatus: http.StatusOK,
			responseBody:   `{"has_speech":true,"confidence":0.95}`,
			expectedSpeech: true,
			expectedConfidence: 0.95,
			wantErr:        false,
		},
		{
			name:           "no speech detected",
			audioData:      []byte("fake audio data"),
			responseStatus: http.StatusOK,
			responseBody:   `{"has_speech":false,"confidence":0.15}`,
			expectedSpeech: false,
			expectedConfidence: 0.15,
			wantErr:        false,
		},
		{
			name:           "service error",
			audioData:      []byte("fake audio data"),
			responseStatus: http.StatusInternalServerError,
			responseBody:   `{"error":"internal error"}`,
			wantErr:        true,
		},
		{
			name:           "invalid audio",
			audioData:      []byte{},
			responseStatus: http.StatusBadRequest,
			responseBody:   `{"error":"audio data required"}`,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock HTTP server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/vad", r.URL.Path)
				assert.Equal(t, "POST", r.Method)

				// Verify multipart form data
				err := r.ParseMultipartForm(10 << 20) // 10MB
				require.NoError(t, err)

				file, _, err := r.FormFile("audio")
				if len(tt.audioData) > 0 {
					require.NoError(t, err)
					audioBytes, err := io.ReadAll(file)
					require.NoError(t, err)
					assert.Equal(t, tt.audioData, audioBytes)
				}

				w.WriteHeader(tt.responseStatus)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			logger := zerolog.Nop()
			client := NewHFVADClient(server.URL, logger)

			result, err := client.DetectSpeech(context.Background(), tt.audioData)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.expectedSpeech, result.IsSpeech)
				assert.InDelta(t, tt.expectedConfidence, result.Confidence, 0.01)
			}
		})
	}
}

func TestHFVADClient_DetectSpeech_Timeout(t *testing.T) {
	// Create server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"has_speech":true,"confidence":0.95}`))
	}))
	defer server.Close()

	logger := zerolog.Nop()
	client := NewHFVADClient(server.URL, logger)
	client.httpClient.Timeout = 100 * time.Millisecond

	ctx := context.Background()
	result, err := client.DetectSpeech(ctx, []byte("fake audio"))

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "deadline exceeded")
}

func TestHFVADClient_DetectSpeech_ContextCancellation(t *testing.T) {
	// Create server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"has_speech":true,"confidence":0.95}`))
	}))
	defer server.Close()

	logger := zerolog.Nop()
	client := NewHFVADClient(server.URL, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result, err := client.DetectSpeech(ctx, []byte("fake audio"))

	assert.Error(t, err)
	assert.Nil(t, result)
}

