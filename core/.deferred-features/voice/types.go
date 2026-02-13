package voice

import "time"

// TranscriptionRequest represents a speech-to-text transcription request.
type TranscriptionRequest struct {
	// AudioData is the raw audio bytes (WAV, MP3, WebM, etc.)
	AudioData []byte `json:"-"`

	// Language is the language code (e.g., "en", "es", "zh")
	// Leave empty for auto-detection
	Language string `json:"language,omitempty"`

	// ModelSize specifies the whisper model size ("tiny", "base", "small", "medium", "large")
	// Defaults to "base" if not specified
	ModelSize string `json:"model_size,omitempty"`

	// Format specifies the audio format ("wav", "mp3", "webm")
	Format string `json:"format,omitempty"`
}

// TranscriptionResponse represents the result of a speech-to-text transcription.
type TranscriptionResponse struct {
	// Text is the transcribed text
	Text string `json:"text"`

	// Confidence is the overall confidence score (0.0 to 1.0)
	Confidence float64 `json:"confidence"`

	// Language is the detected or specified language
	Language string `json:"language"`

	// Duration is how long the audio was (in seconds)
	Duration float64 `json:"duration"`

	// ProcessingTime is how long transcription took
	ProcessingTime time.Duration `json:"processing_time"`

	// Segments contains detailed transcription segments (optional)
	Segments []TranscriptionSegment `json:"segments,omitempty"`

	// Error contains any error message
	Error string `json:"error,omitempty"`
}

// TranscriptionSegment represents a time-aligned transcription segment.
type TranscriptionSegment struct {
	ID         int     `json:"id"`
	Start      float64 `json:"start"`      // Start time in seconds
	End        float64 `json:"end"`        // End time in seconds
	Text       string  `json:"text"`       // Segment text
	Confidence float64 `json:"confidence"` // Segment confidence
}

// WhisperConfig holds configuration for the Whisper service.
type WhisperConfig struct {
	// ModelPath is the path to whisper.cpp models directory
	ModelPath string

	// ExecutablePath is the path to whisper.cpp main executable
	ExecutablePath string

	// DefaultModelSize is the default model to use ("tiny", "base", "small", "medium", "large")
	DefaultModelSize string

	// MaxAudioSize is the maximum audio file size in bytes (default: 25MB)
	MaxAudioSize int64

	// TempDir is where temporary audio files are stored
	TempDir string

	// EnableGPU enables GPU acceleration if available
	EnableGPU bool

	// NumThreads specifies the number of CPU threads to use
	NumThreads int
}
