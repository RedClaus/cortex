// Package stt provides Speech-to-Text transcription services for CortexAvatar.
package stt

import (
	"context"
	"errors"
	"time"
)

// Common errors
var (
	ErrProviderUnavailable = errors.New("STT provider unavailable")
	ErrAudioTooShort       = errors.New("audio too short for transcription")
	ErrAudioTooLong        = errors.New("audio exceeds maximum length")
	ErrInvalidFormat       = errors.New("invalid audio format")
	ErrTimeout             = errors.New("transcription timeout")
)

// Provider is the interface all STT providers must implement
type Provider interface {
	// Name returns the provider identifier (e.g., "whisper", "a2a")
	Name() string

	// Transcribe converts audio to text
	Transcribe(ctx context.Context, req *TranscribeRequest) (*TranscribeResponse, error)

	// TranscribeStream handles streaming transcription (if supported)
	TranscribeStream(ctx context.Context, audioStream <-chan []byte) (<-chan *TranscribeResponse, error)

	// Health checks if the provider is available
	Health(ctx context.Context) error

	// Capabilities returns the provider's feature set
	Capabilities() ProviderCapabilities
}

// TranscribeRequest represents a transcription request
type TranscribeRequest struct {
	Audio      []byte `json:"-"`                   // Raw audio data
	Format     string `json:"format,omitempty"`    // Audio format (wav, pcm, webm)
	SampleRate int    `json:"sample_rate"`         // Sample rate in Hz
	Channels   int    `json:"channels"`            // Number of channels
	Language   string `json:"language,omitempty"`  // Language code (e.g., "en")
	ModelSize  string `json:"model_size,omitempty"` // Model size (tiny, base, small, medium, large)
}

// TranscribeResponse represents a transcription result
type TranscribeResponse struct {
	Text           string                `json:"text"`                      // Transcribed text
	Confidence     float64               `json:"confidence"`                // Overall confidence (0-1)
	Language       string                `json:"language"`                  // Detected/specified language
	Duration       time.Duration         `json:"duration"`                  // Audio duration
	ProcessingTime time.Duration         `json:"processing_time"`           // How long transcription took
	Segments       []TranscribeSegment   `json:"segments,omitempty"`        // Time-aligned segments
	Words          []Word                `json:"words,omitempty"`           // Word-level timestamps
	IsFinal        bool                  `json:"is_final"`                  // True if this is a final result
	Error          string                `json:"error,omitempty"`           // Error message if any
}

// TranscribeSegment represents a time-aligned transcription segment
type TranscribeSegment struct {
	ID         int           `json:"id"`
	Start      time.Duration `json:"start"`
	End        time.Duration `json:"end"`
	Text       string        `json:"text"`
	Confidence float64       `json:"confidence"`
}

// Word represents a word with timestamp
type Word struct {
	Word       string        `json:"word"`
	Start      time.Duration `json:"start"`
	End        time.Duration `json:"end"`
	Confidence float64       `json:"confidence"`
}

// ProviderCapabilities describes what features a provider supports
type ProviderCapabilities struct {
	SupportsStreaming  bool     `json:"supports_streaming"`
	SupportsTimestamps bool     `json:"supports_timestamps"`
	SupportedLanguages []string `json:"supported_languages"`
	MaxAudioLengthSec  int      `json:"max_audio_length_sec"`
	AvgLatencyMs       int      `json:"avg_latency_ms"`
	RequiresGPU        bool     `json:"requires_gpu"`
	IsLocal            bool     `json:"is_local"` // True if runs locally
}

// Config holds STT configuration
type Config struct {
	Provider     string `json:"provider"`      // Provider name (whisper, a2a)
	ModelSize    string `json:"model_size"`    // Model size
	Language     string `json:"language"`      // Default language
	EnableStream bool   `json:"enable_stream"` // Enable streaming transcription
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Provider:     "a2a", // Use CortexBrain via A2A by default
		ModelSize:    "base",
		Language:     "en",
		EnableStream: false,
	}
}
