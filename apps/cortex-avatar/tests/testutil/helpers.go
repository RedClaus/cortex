package testutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// CreateMockHFService creates a mock HF service for testing
func CreateMockHFService(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":     "healthy",
				"components": map[string]string{"vad": "loaded", "stt": "loaded", "tts": "loaded"},
			})

		case "/vad":
			// Parse multipart form to validate audio
			err := r.ParseMultipartForm(10 << 20) // 10 MB max
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error": "failed to parse multipart form",
				})
				return
			}

			file, _, err := r.FormFile("audio")
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error": "audio file is required",
				})
				return
			}
			defer file.Close()

			// Read audio data
			audioData := make([]byte, 1024*1024) // 1MB buffer
			n, _ := file.Read(audioData)
			audioData = audioData[:n]

			// Validate audio is not empty
			if len(audioData) == 0 {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error": "audio data is empty",
				})
				return
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"has_speech": true,
				"confidence": 0.95,
			})

		case "/stt":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"text":               "Hello, this is a test transcription",
				"language":           "en",
				"confidence":         0.98,
				"processing_time_ms": 450,
			})

		case "/tts":
			// Generate fake WAV audio (44 bytes WAV header + 1000 bytes of silence)
			w.Header().Set("Content-Type", "audio/wav")
			w.WriteHeader(http.StatusOK)

			// Minimal WAV header
			wavHeader := []byte{
				0x52, 0x49, 0x46, 0x46, // "RIFF"
				0xE8, 0x03, 0x00, 0x00, // File size
				0x57, 0x41, 0x56, 0x45, // "WAVE"
				0x66, 0x6D, 0x74, 0x20, // "fmt "
				0x10, 0x00, 0x00, 0x00, // Chunk size
				0x01, 0x00,             // Audio format (PCM)
				0x01, 0x00,             // Channels (mono)
				0x80, 0x3E, 0x00, 0x00, // Sample rate (16000)
				0x00, 0x7D, 0x00, 0x00, // Byte rate
				0x02, 0x00,             // Block align
				0x10, 0x00,             // Bits per sample
				0x64, 0x61, 0x74, 0x61, // "data"
				0xC4, 0x03, 0x00, 0x00, // Data size
			}
			w.Write(wavHeader)
			w.Write(make([]byte, 1000)) // Silent audio data

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// GenerateTestAudio generates test audio (silent WAV) with specified duration
func GenerateTestAudio(t *testing.T, duration time.Duration) []byte {
	sampleRate := 16000
	channels := 1
	bitsPerSample := 16

	numSamples := int(duration.Seconds() * float64(sampleRate))
	dataSize := numSamples * channels * (bitsPerSample / 8)

	// WAV header
	header := []byte{
		0x52, 0x49, 0x46, 0x46, // "RIFF"
		0x00, 0x00, 0x00, 0x00, // File size (placeholder)
		0x57, 0x41, 0x56, 0x45, // "WAVE"
		0x66, 0x6D, 0x74, 0x20, // "fmt "
		0x10, 0x00, 0x00, 0x00, // Chunk size
		0x01, 0x00,             // Audio format (PCM)
		byte(channels), 0x00,   // Channels
		0x80, 0x3E, 0x00, 0x00, // Sample rate (16000)
		0x00, 0x7D, 0x00, 0x00, // Byte rate
		0x02, 0x00,             // Block align
		byte(bitsPerSample), 0x00, // Bits per sample
		0x64, 0x61, 0x74, 0x61, // "data"
		0x00, 0x00, 0x00, 0x00, // Data size (placeholder)
	}

	// Update file size and data size
	fileSize := uint32(len(header) + dataSize - 8)
	header[4] = byte(fileSize)
	header[5] = byte(fileSize >> 8)
	header[6] = byte(fileSize >> 16)
	header[7] = byte(fileSize >> 24)

	header[len(header)-4] = byte(dataSize)
	header[len(header)-3] = byte(dataSize >> 8)
	header[len(header)-2] = byte(dataSize >> 16)
	header[len(header)-1] = byte(dataSize >> 24)

	// Combine header and silent data
	audio := make([]byte, len(header)+dataSize)
	copy(audio, header)

	return audio
}
