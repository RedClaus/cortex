// Package voice provides unified interfaces for TTS providers.
// It abstracts Kokoro (Fast Lane) and XTTS (Smart Lane) behind a common interface.
package voice

import (
	"context"
	"errors"
	"io"
	"time"
)

// Common errors
var (
	ErrProviderUnavailable = errors.New("provider unavailable")
	ErrVoiceNotFound       = errors.New("voice not found")
	ErrTextTooLong         = errors.New("text exceeds maximum length")
	ErrTimeout             = errors.New("request timeout")
	ErrInvalidAudioFormat  = errors.New("invalid audio format")
	ErrCloningNotSupported = errors.New("voice cloning not supported by this provider")
	ErrStreamingNotSupported = errors.New("streaming not supported by this provider")
)

// Provider is the interface all TTS providers must implement.
type Provider interface {
	// Name returns the provider identifier (e.g., "kokoro", "xtts")
	Name() string

	// Synthesize sends a synthesis request and returns the full audio response.
	Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error)

	// Stream sends a streaming synthesis request.
	// Returns an audio stream that can be read incrementally.
	Stream(ctx context.Context, req *SynthesizeRequest) (AudioStream, error)

	// ListVoices returns available voices from this provider.
	ListVoices(ctx context.Context) ([]Voice, error)

	// Health checks if the provider is available and configured.
	Health(ctx context.Context) error

	// Capabilities returns the provider's feature set.
	Capabilities() ProviderCapabilities
}

// SynthesizeRequest represents a request to a TTS provider.
type SynthesizeRequest struct {
	Text          string      `json:"text"`
	VoiceID       string      `json:"voice_id"`
	Speed         float64     `json:"speed,omitempty"`          // Playback speed multiplier (0.5-2.0)
	Pitch         float64     `json:"pitch,omitempty"`          // Pitch adjustment (-1.0 to 1.0)
	CloneFromFile string      `json:"clone_from_file,omitempty"` // Path to audio file for voice cloning
	Format        AudioFormat `json:"format,omitempty"`         // Desired output format
	SampleRate    int         `json:"sample_rate,omitempty"`    // Desired sample rate (Hz)
}

// SynthesizeResponse represents a response from a TTS provider.
type SynthesizeResponse struct {
	Audio       []byte        `json:"audio"`         // Raw audio data
	Format      AudioFormat   `json:"format"`        // Audio format
	Duration    time.Duration `json:"duration"`      // Audio duration
	SampleRate  int           `json:"sample_rate"`   // Sample rate in Hz
	ProcessedMs int64         `json:"processed_ms"`  // Processing time in milliseconds
	VoiceID     string        `json:"voice_id"`      // Voice used for synthesis
	Provider    string        `json:"provider"`      // Provider name
}

// AudioStream represents a streaming audio response.
type AudioStream interface {
	io.ReadCloser

	// Format returns the audio format of the stream.
	Format() AudioFormat

	// SampleRate returns the sample rate in Hz.
	SampleRate() int
}

// Voice represents an available TTS voice.
type Voice struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Language string            `json:"language"`      // ISO 639-1 language code (e.g., "en", "es")
	Gender   Gender            `json:"gender"`
	Preview  string            `json:"preview,omitempty"`  // URL to preview audio
	IsCloned bool              `json:"is_cloned"`          // True if this is a cloned voice
	Metadata map[string]string `json:"metadata,omitempty"` // Additional provider-specific metadata
}

// Gender represents voice gender.
type Gender string

const (
	GenderMale    Gender = "male"
	GenderFemale  Gender = "female"
	GenderNeutral Gender = "neutral"
	GenderUnknown Gender = "unknown"
)

// ProviderCapabilities describes what features a provider supports.
type ProviderCapabilities struct {
	SupportsStreaming bool     `json:"supports_streaming"`
	SupportsCloning   bool     `json:"supports_cloning"`
	Languages         []string `json:"languages"`         // Supported language codes
	MaxTextLength     int      `json:"max_text_length"`   // Maximum characters per request
	RequiresGPU       bool     `json:"requires_gpu"`      // True if GPU is required
	AvgLatencyMs      int      `json:"avg_latency_ms"`    // Average latency in milliseconds
	SupportedFormats  []AudioFormat `json:"supported_formats"` // Supported audio formats
}

// Note: AudioFormat, FormatWAV, FormatMP3, FormatOGG, FormatPCM, FormatOpus, and IsValid() are defined in cache.go

// ProviderConfig contains common configuration for providers.
type ProviderConfig struct {
	BaseURL      string        `json:"base_url,omitempty"`
	APIKey       string        `json:"api_key,omitempty"`
	Timeout      time.Duration `json:"timeout,omitempty"`
	MaxRetries   int           `json:"max_retries,omitempty"`
	DefaultVoice string        `json:"default_voice,omitempty"`
	GPUEnabled   bool          `json:"gpu_enabled,omitempty"`
}

// DefaultConfig returns sensible default configuration.
func DefaultConfig() ProviderConfig {
	return ProviderConfig{
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		GPUEnabled: false,
	}
}

// ValidateRequest validates a synthesis request.
func ValidateRequest(req *SynthesizeRequest) error {
	if req.Text == "" {
		return errors.New("text cannot be empty")
	}
	if req.VoiceID == "" {
		return errors.New("voice_id is required")
	}
	if req.Speed != 0 && (req.Speed < 0.5 || req.Speed > 2.0) {
		return errors.New("speed must be between 0.5 and 2.0")
	}
	if req.Pitch != 0 && (req.Pitch < -1.0 || req.Pitch > 1.0) {
		return errors.New("pitch must be between -1.0 and 1.0")
	}
	if req.Format != "" && !req.Format.IsValid() {
		return ErrInvalidAudioFormat
	}
	if req.SampleRate != 0 && (req.SampleRate < 8000 || req.SampleRate > 48000) {
		return errors.New("sample_rate must be between 8000 and 48000 Hz")
	}
	return nil
}
