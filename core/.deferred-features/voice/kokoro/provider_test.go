package kokoro

import (
	"context"
	"testing"
	"time"

	"github.com/normanking/cortex/internal/voice"
)

func TestNewProvider(t *testing.T) {
	tests := []struct {
		name           string
		config         Config
		expectedURL    string
		expectedVoice  string
		expectedTimeout time.Duration
	}{
		{
			name:           "default config",
			config:         Config{},
			expectedURL:    "http://localhost:8880",
			expectedVoice:  "af_bella",
			expectedTimeout: 5 * time.Second,
		},
		{
			name: "custom config",
			config: Config{
				BaseURL:      "http://custom:9000",
				DefaultVoice: "am_adam",
				Timeout:      10 * time.Second,
			},
			expectedURL:    "http://custom:9000",
			expectedVoice:  "am_adam",
			expectedTimeout: 10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProvider(tt.config)

			if p.config.BaseURL != tt.expectedURL {
				t.Errorf("expected BaseURL %s, got %s", tt.expectedURL, p.config.BaseURL)
			}
			if p.config.DefaultVoice != tt.expectedVoice {
				t.Errorf("expected DefaultVoice %s, got %s", tt.expectedVoice, p.config.DefaultVoice)
			}
			if p.config.Timeout != tt.expectedTimeout {
				t.Errorf("expected Timeout %s, got %s", tt.expectedTimeout, p.config.Timeout)
			}
		})
	}
}

func TestProvider_Name(t *testing.T) {
	p := NewProvider(Config{})
	if p.Name() != "kokoro" {
		t.Errorf("expected name 'kokoro', got '%s'", p.Name())
	}
}

func TestProvider_ListVoices(t *testing.T) {
	p := NewProvider(Config{})
	ctx := context.Background()

	voices, err := p.ListVoices(ctx)
	if err != nil {
		t.Fatalf("ListVoices failed: %v", err)
	}

	if len(voices) != 6 {
		t.Errorf("expected 6 voices, got %d", len(voices))
	}

	// Check for specific voices
	expectedVoices := map[string]voice.Gender{
		"af_bella":    voice.GenderFemale,
		"af_sarah":    voice.GenderFemale,
		"am_adam":     voice.GenderMale,
		"am_michael":  voice.GenderMale,
		"bf_emma":     voice.GenderFemale,
		"bm_george":   voice.GenderMale,
	}

	for _, v := range voices {
		expectedGender, ok := expectedVoices[v.ID]
		if !ok {
			t.Errorf("unexpected voice ID: %s", v.ID)
			continue
		}
		if v.Gender != expectedGender {
			t.Errorf("voice %s: expected gender %s, got %s", v.ID, expectedGender, v.Gender)
		}
		if v.Language != "en" {
			t.Errorf("voice %s: expected language 'en', got '%s'", v.ID, v.Language)
		}
		if v.IsCloned {
			t.Errorf("voice %s: should not be cloned", v.ID)
		}
	}
}

func TestProvider_Capabilities(t *testing.T) {
	p := NewProvider(Config{})
	caps := p.Capabilities()

	if !caps.SupportsStreaming {
		t.Error("expected SupportsStreaming to be true")
	}
	if caps.SupportsCloning {
		t.Error("expected SupportsCloning to be false")
	}
	if caps.RequiresGPU {
		t.Error("expected RequiresGPU to be false")
	}
	if caps.AvgLatencyMs != 250 {
		t.Errorf("expected AvgLatencyMs 250, got %d", caps.AvgLatencyMs)
	}
	if caps.MaxTextLength != 2000 {
		t.Errorf("expected MaxTextLength 2000, got %d", caps.MaxTextLength)
	}

	// Check languages
	if len(caps.Languages) != 1 || caps.Languages[0] != "en" {
		t.Errorf("expected languages ['en'], got %v", caps.Languages)
	}

	// Check supported formats
	if len(caps.SupportedFormats) != 1 || caps.SupportedFormats[0] != voice.FormatWAV {
		t.Errorf("expected formats ['wav'], got %v", caps.SupportedFormats)
	}
}

func TestProvider_isValidVoice(t *testing.T) {
	p := NewProvider(Config{})

	tests := []struct {
		voiceID  string
		expected bool
	}{
		{"af_bella", true},
		{"am_adam", true},
		{"bf_emma", true},
		{"bm_george", true},
		{"invalid_voice", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.voiceID, func(t *testing.T) {
			result := p.isValidVoice(tt.voiceID)
			if result != tt.expected {
				t.Errorf("isValidVoice(%s): expected %v, got %v", tt.voiceID, tt.expected, result)
			}
		})
	}
}

func TestProvider_ValidateVoice(t *testing.T) {
	p := NewProvider(Config{})

	// Test valid voice
	v, err := p.ValidateVoice("af_bella")
	if err != nil {
		t.Errorf("ValidateVoice failed for valid voice: %v", err)
	}
	if v == nil {
		t.Error("expected non-nil voice")
	} else if v.ID != "af_bella" {
		t.Errorf("expected voice ID 'af_bella', got '%s'", v.ID)
	}

	// Test invalid voice
	v, err = p.ValidateVoice("invalid_voice")
	if err != voice.ErrVoiceNotFound {
		t.Errorf("expected ErrVoiceNotFound, got %v", err)
	}
	if v != nil {
		t.Error("expected nil voice for invalid ID")
	}
}

func TestProvider_GetSetDefaultVoice(t *testing.T) {
	p := NewProvider(Config{})

	// Check initial default
	if p.GetDefaultVoice() != "af_bella" {
		t.Errorf("expected default voice 'af_bella', got '%s'", p.GetDefaultVoice())
	}

	// Set valid voice
	err := p.SetDefaultVoice("am_adam")
	if err != nil {
		t.Errorf("SetDefaultVoice failed: %v", err)
	}
	if p.GetDefaultVoice() != "am_adam" {
		t.Errorf("expected default voice 'am_adam', got '%s'", p.GetDefaultVoice())
	}

	// Try to set invalid voice
	err = p.SetDefaultVoice("invalid_voice")
	if err != voice.ErrVoiceNotFound {
		t.Errorf("expected ErrVoiceNotFound, got %v", err)
	}
	// Default should remain unchanged
	if p.GetDefaultVoice() != "am_adam" {
		t.Errorf("default voice should remain 'am_adam', got '%s'", p.GetDefaultVoice())
	}
}

func TestPresetVoices(t *testing.T) {
	// Verify all preset voices are properly defined
	expectedCount := 6
	if len(PresetVoices) != expectedCount {
		t.Errorf("expected %d preset voices, got %d", expectedCount, len(PresetVoices))
	}

	// Verify each voice has required fields
	for _, v := range PresetVoices {
		if v.ID == "" {
			t.Error("voice has empty ID")
		}
		if v.Name == "" {
			t.Error("voice has empty Name")
		}
		if v.Language == "" {
			t.Error("voice has empty Language")
		}
		if v.Gender == "" {
			t.Error("voice has empty Gender")
		}
		if v.IsCloned {
			t.Errorf("voice %s should not be marked as cloned", v.ID)
		}
	}
}

func TestAudioStream(t *testing.T) {
	// Mock reader for testing
	mockReader := &mockReadCloser{
		data: []byte("test audio data"),
	}

	stream := &audioStream{
		reader:     mockReader,
		format:     voice.FormatWAV,
		sampleRate: 22050,
	}

	// Test Format
	if stream.Format() != voice.FormatWAV {
		t.Errorf("expected format WAV, got %s", stream.Format())
	}

	// Test SampleRate
	if stream.SampleRate() != 22050 {
		t.Errorf("expected sample rate 22050, got %d", stream.SampleRate())
	}

	// Test Read
	buf := make([]byte, 100)
	n, err := stream.Read(buf)
	if err != nil {
		t.Errorf("Read failed: %v", err)
	}
	if n != len(mockReader.data) {
		t.Errorf("expected to read %d bytes, got %d", len(mockReader.data), n)
	}

	// Test Close
	if err := stream.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
	if !mockReader.closed {
		t.Error("reader was not closed")
	}
}

// mockReadCloser is a test helper
type mockReadCloser struct {
	data   []byte
	pos    int
	closed bool
}

func (m *mockReadCloser) Read(p []byte) (n int, err error) {
	if m.pos >= len(m.data) {
		return 0, nil
	}
	n = copy(p, m.data[m.pos:])
	m.pos += n
	return n, nil
}

func (m *mockReadCloser) Close() error {
	m.closed = true
	return nil
}
