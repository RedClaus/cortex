package xtts

import (
	"context"
	"testing"
	"time"

	"github.com/normanking/cortex/internal/voice"
)

func TestNewProvider(t *testing.T) {
	config := Config{
		BaseURL:         "http://localhost:5002",
		DefaultVoice:    "default",
		ClonedVoicesDir: "/tmp/cloned_voices",
		Timeout:         30 * time.Second,
		MaxTextLength:   5000,
		GPUEnabled:      true,
	}

	provider := NewProvider(config)

	if provider.Name() != "xtts" {
		t.Errorf("Expected provider name 'xtts', got '%s'", provider.Name())
	}

	if provider.config.BaseURL != config.BaseURL {
		t.Errorf("Expected BaseURL '%s', got '%s'", config.BaseURL, provider.config.BaseURL)
	}
}

func TestProviderDefaults(t *testing.T) {
	provider := NewProvider(Config{})

	if provider.config.BaseURL != "http://localhost:5002" {
		t.Errorf("Expected default BaseURL 'http://localhost:5002', got '%s'", provider.config.BaseURL)
	}

	if provider.config.DefaultVoice != "default" {
		t.Errorf("Expected default voice 'default', got '%s'", provider.config.DefaultVoice)
	}

	if provider.config.Timeout != 30*time.Second {
		t.Errorf("Expected default timeout 30s, got %v", provider.config.Timeout)
	}

	if provider.config.MaxTextLength != 5000 {
		t.Errorf("Expected default max text length 5000, got %d", provider.config.MaxTextLength)
	}
}

func TestCapabilities(t *testing.T) {
	provider := NewProvider(Config{})
	caps := provider.Capabilities()

	if !caps.SupportsStreaming {
		t.Error("XTTS should support streaming")
	}

	if !caps.SupportsCloning {
		t.Error("XTTS should support voice cloning")
	}

	if !caps.RequiresGPU {
		t.Error("XTTS requires GPU")
	}

	if caps.MaxTextLength != 5000 {
		t.Errorf("Expected max text length 5000, got %d", caps.MaxTextLength)
	}

	expectedLanguages := []string{
		"en", "es", "fr", "de", "it", "pt", "pl", "tr",
		"ru", "nl", "cs", "ar", "zh-cn", "ja", "hu", "ko", "hi",
	}

	if len(caps.Languages) != len(expectedLanguages) {
		t.Errorf("Expected %d languages, got %d", len(expectedLanguages), len(caps.Languages))
	}

	// Check if all expected languages are present
	languageMap := make(map[string]bool)
	for _, lang := range caps.Languages {
		languageMap[lang] = true
	}

	for _, lang := range expectedLanguages {
		if !languageMap[lang] {
			t.Errorf("Expected language '%s' not found in capabilities", lang)
		}
	}

	// Check supported formats
	if len(caps.SupportedFormats) < 1 {
		t.Error("XTTS should support at least one audio format")
	}

	hasWAV := false
	for _, format := range caps.SupportedFormats {
		if format == voice.FormatWAV {
			hasWAV = true
			break
		}
	}
	if !hasWAV {
		t.Error("XTTS should support WAV format")
	}
}

func TestListVoices(t *testing.T) {
	provider := NewProvider(Config{})

	ctx := context.Background()
	voices, err := provider.ListVoices(ctx)
	if err != nil {
		t.Fatalf("ListVoices failed: %v", err)
	}

	if len(voices) == 0 {
		t.Error("Expected at least one voice (default)")
	}

	// Check for default voice
	hasDefault := false
	for _, v := range voices {
		if v.ID == "default" {
			hasDefault = true
			if v.Language != "en" {
				t.Errorf("Expected default voice language 'en', got '%s'", v.Language)
			}
			if v.IsCloned {
				t.Error("Default voice should not be marked as cloned")
			}
		}
	}

	if !hasDefault {
		t.Error("Default voice not found in voice list")
	}
}

func TestCloneVoice(t *testing.T) {
	provider := NewProvider(Config{
		ClonedVoicesDir: "/tmp/test_voices",
	})

	// Create a temporary reference file for testing
	// Note: In real usage, this would be an actual audio file
	tempFile := "/tmp/test_reference.wav"

	// Test cloning (without actual file - will fail in real usage)
	_, err := provider.CloneVoice(context.Background(), "", tempFile, "en")
	if err == nil {
		t.Error("Expected error for empty voice name")
	}

	_, err = provider.CloneVoice(context.Background(), "Test Voice", "", "en")
	if err == nil {
		t.Error("Expected error for empty reference file")
	}
}

func TestGetSpeakerFile(t *testing.T) {
	provider := NewProvider(Config{})

	// Test default voice (no speaker file)
	speakerFile, err := provider.getSpeakerFile("", "")
	if err != nil {
		t.Errorf("Unexpected error for default voice: %v", err)
	}
	if speakerFile != "" {
		t.Errorf("Expected empty speaker file for default voice, got '%s'", speakerFile)
	}

	speakerFile, err = provider.getSpeakerFile("default", "")
	if err != nil {
		t.Errorf("Unexpected error for default voice: %v", err)
	}
	if speakerFile != "" {
		t.Errorf("Expected empty speaker file for default voice, got '%s'", speakerFile)
	}

	// Test non-existent cloned voice
	_, err = provider.getSpeakerFile("nonexistent", "")
	if err != voice.ErrVoiceNotFound {
		t.Errorf("Expected ErrVoiceNotFound, got %v", err)
	}
}

func TestValidateRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     *voice.SynthesizeRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: &voice.SynthesizeRequest{
				Text:    "Hello, world!",
				VoiceID: "default",
			},
			wantErr: false,
		},
		{
			name: "empty text",
			req: &voice.SynthesizeRequest{
				Text:    "",
				VoiceID: "default",
			},
			wantErr: true,
		},
		{
			name: "empty voice ID",
			req: &voice.SynthesizeRequest{
				Text:    "Hello",
				VoiceID: "",
			},
			wantErr: true,
		},
		{
			name: "invalid speed (too low)",
			req: &voice.SynthesizeRequest{
				Text:    "Hello",
				VoiceID: "default",
				Speed:   0.3,
			},
			wantErr: true,
		},
		{
			name: "invalid speed (too high)",
			req: &voice.SynthesizeRequest{
				Text:    "Hello",
				VoiceID: "default",
				Speed:   2.5,
			},
			wantErr: true,
		},
		{
			name: "valid speed",
			req: &voice.SynthesizeRequest{
				Text:    "Hello",
				VoiceID: "default",
				Speed:   1.5,
			},
			wantErr: false,
		},
		{
			name: "invalid pitch (too low)",
			req: &voice.SynthesizeRequest{
				Text:    "Hello",
				VoiceID: "default",
				Pitch:   -1.5,
			},
			wantErr: true,
		},
		{
			name: "invalid pitch (too high)",
			req: &voice.SynthesizeRequest{
				Text:    "Hello",
				VoiceID: "default",
				Pitch:   1.5,
			},
			wantErr: true,
		},
		{
			name: "valid pitch",
			req: &voice.SynthesizeRequest{
				Text:    "Hello",
				VoiceID: "default",
				Pitch:   0.5,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := voice.ValidateRequest(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestSynthesizeValidation tests that the Synthesize method validates requests
func TestSynthesizeValidation(t *testing.T) {
	provider := NewProvider(Config{
		MaxTextLength: 100,
	})

	ctx := context.Background()

	// Test text too long
	_, err := provider.Synthesize(ctx, &voice.SynthesizeRequest{
		Text:    string(make([]byte, 101)),
		VoiceID: "default",
	})
	if err != voice.ErrTextTooLong {
		t.Errorf("Expected ErrTextTooLong for text exceeding max length, got %v", err)
	}
}
