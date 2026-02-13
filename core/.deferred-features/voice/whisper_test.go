package voice

import (
	"os"
	"testing"
)

func TestWhisperService_New(t *testing.T) {
	tests := []struct {
		name        string
		config      WhisperConfig
		expectError bool
	}{
		{
			name: "valid config with defaults",
			config: WhisperConfig{
				ExecutablePath: "echo", // Use echo as a mock executable
			},
			expectError: false,
		},
		{
			name: "invalid executable path",
			config: WhisperConfig{
				ExecutablePath: "/nonexistent/whisper",
			},
			expectError: true,
		},
		{
			name: "invalid model path",
			config: WhisperConfig{
				ModelPath:      "/nonexistent/models",
				ExecutablePath: "echo",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewWhisperService(tt.config)
			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		filename    string
		contentType string
		expected    string
	}{
		{"audio.wav", "", "wav"},
		{"audio.mp3", "", "mp3"},
		{"audio.webm", "", "webm"},
		{"recording.WAV", "", "wav"},
		{"", "audio/wav", "wav"},
		{"", "audio/mpeg", "mp3"},
		{"", "audio/webm", "webm"},
		{"unknown.xyz", "application/octet-stream", "wav"},
	}

	for _, tt := range tests {
		t.Run(tt.filename+"/"+tt.contentType, func(t *testing.T) {
			result := detectFormat(tt.filename, tt.contentType)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"00:00:00,000", 0.0},
		{"00:00:01,500", 1.5},
		{"00:01:30,250", 90.25},
		{"01:00:00,000", 3600.0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseTimestamp(tt.input)
			if result != tt.expected {
				t.Errorf("expected %f, got %f", tt.expected, result)
			}
		})
	}
}

func TestCreateTempAudioFile(t *testing.T) {
	service := &WhisperService{
		config: WhisperConfig{
			TempDir: os.TempDir(),
		},
	}

	audioData := []byte("fake audio data")
	tempFile, err := service.createTempAudioFile(audioData, "wav")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile)

	// Verify file exists
	if _, err := os.Stat(tempFile); os.IsNotExist(err) {
		t.Error("temp file was not created")
	}

	// Verify content
	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("failed to read temp file: %v", err)
	}
	if string(content) != string(audioData) {
		t.Error("temp file content does not match")
	}
}

func TestTranscribeRequest_Validation(t *testing.T) {
	service, err := NewWhisperService(WhisperConfig{
		ExecutablePath: "echo",
		MaxAudioSize:   1024, // 1KB limit for testing
	})
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	// Test empty audio data
	req := TranscriptionRequest{
		AudioData: []byte{},
	}
	resp, err := service.Transcribe(req)
	if err == nil {
		t.Error("expected error for empty audio data")
	}
	if resp.Error == "" {
		t.Error("expected error message in response")
	}

	// Test audio too large
	req = TranscriptionRequest{
		AudioData: make([]byte, 2048), // Exceeds 1KB limit
	}
	resp, err = service.Transcribe(req)
	if err == nil {
		t.Error("expected error for oversized audio")
	}
	if resp.Error == "" {
		t.Error("expected error message in response")
	}
}
