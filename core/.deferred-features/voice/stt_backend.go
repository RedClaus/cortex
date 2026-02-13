// Package voice provides voice processing capabilities for Cortex.
// stt_backend.go defines the STTBackend interface for speech-to-text providers.
package voice

import (
	"context"
	"time"
)

// STTBackend defines the interface for speech-to-text providers.
// Brain Alignment: Multiple backends allow graceful degradation and specialization,
// similar to how the brain uses multiple pathways for auditory processing.
type STTBackend interface {
	// Name returns the backend identifier (e.g., "whisper", "sensevoice")
	Name() string

	// IsAvailable checks if the backend is ready to process requests
	IsAvailable(ctx context.Context) bool

	// Transcribe converts audio to text
	Transcribe(ctx context.Context, req *STTRequest) (*STTResult, error)

	// SupportsEmotion indicates if this backend provides emotion detection
	SupportsEmotion() bool

	// SupportsStreaming indicates if this backend supports streaming transcription
	SupportsStreaming() bool
}

// STTRequest contains parameters for a transcription request.
type STTRequest struct {
	// Audio data (raw bytes)
	AudioData []byte

	// AudioPath is the path to an audio file (alternative to AudioData)
	AudioPath string

	// AudioFormat specifies the audio format (e.g., "wav", "mp3")
	AudioFormat string

	// Language hint for transcription (e.g., "en", "zh", "auto")
	Language string

	// IncludeEmotion requests emotion detection if supported
	IncludeEmotion bool

	// IncludeEvents requests audio event detection if supported
	IncludeEvents bool
}

// STTResult contains the transcription output.
type STTResult struct {
	// Text is the transcribed text
	Text string `json:"text"`

	// Language detected or used for transcription
	Language string `json:"language,omitempty"`

	// Confidence in the transcription (0.0-1.0)
	Confidence float64 `json:"confidence,omitempty"`

	// Latency is the time taken for transcription
	Latency time.Duration `json:"latency_ns,omitempty"`

	// Backend identifies which STT backend was used
	Backend string `json:"backend"`

	// Emotion contains voice-based emotion detection (if requested and supported)
	Emotion *VoiceEmotionData `json:"emotion,omitempty"`

	// AudioEvents contains detected audio events (if requested and supported)
	AudioEvents []AudioEvent `json:"audio_events,omitempty"`

	// IsFinal indicates if this is a final transcription (for streaming)
	IsFinal bool `json:"is_final"`
}

// VoiceEmotionData represents detected emotions from voice.
// Brain Alignment: Maps to the Emotion Lobe's input signals, providing
// multimodal emotion detection (voice + text).
type VoiceEmotionData struct {
	// Primary is the dominant detected emotion
	Primary string `json:"primary"`

	// Confidence in the primary emotion detection (0.0-1.0)
	Confidence float64 `json:"confidence"`

	// All contains confidence scores for all detected emotions
	All map[string]float64 `json:"all,omitempty"`
}

// AudioEvent represents a detected audio event.
type AudioEvent struct {
	// Type of audio event (e.g., "speech", "music", "laughter")
	Type string `json:"type"`

	// Confidence in the event detection (0.0-1.0)
	Confidence float64 `json:"confidence"`

	// StartTime is the event start timestamp in seconds (optional)
	StartTime float64 `json:"start_time,omitempty"`

	// EndTime is the event end timestamp in seconds (optional)
	EndTime float64 `json:"end_time,omitempty"`
}

// EmotionLabel represents standard emotion labels supported by SenseVoice.
// These align with the Emotion Lobe's emotion categories.
type EmotionLabel string

const (
	EmotionHappy     EmotionLabel = "happy"
	EmotionSad       EmotionLabel = "sad"
	EmotionAngry     EmotionLabel = "angry"
	EmotionSurprised EmotionLabel = "surprised"
	EmotionFearful   EmotionLabel = "fearful"
	EmotionDisgusted EmotionLabel = "disgusted"
	EmotionNeutral   EmotionLabel = "neutral"
)

// AllEmotionLabels returns all supported emotion labels.
func AllEmotionLabels() []EmotionLabel {
	return []EmotionLabel{
		EmotionHappy,
		EmotionSad,
		EmotionAngry,
		EmotionSurprised,
		EmotionFearful,
		EmotionDisgusted,
		EmotionNeutral,
	}
}

// AudioEventType represents standard audio event types.
type AudioEventType string

const (
	AudioEventSpeech   AudioEventType = "speech"
	AudioEventMusic    AudioEventType = "music"
	AudioEventLaughter AudioEventType = "laughter"
	AudioEventApplause AudioEventType = "applause"
	AudioEventCrying   AudioEventType = "crying"
	AudioEventCoughing AudioEventType = "coughing"
)

// STTBackendConfig contains configuration for an STT backend.
type STTBackendConfig struct {
	// Enabled indicates if this backend should be used
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Endpoint is the API endpoint URL
	Endpoint string `yaml:"endpoint" json:"endpoint"`

	// Priority determines selection order (lower = higher priority)
	Priority int `yaml:"priority" json:"priority"`

	// EnableEmotion enables emotion detection for this backend
	EnableEmotion bool `yaml:"enable_emotion" json:"enable_emotion"`

	// EnableEvents enables audio event detection for this backend
	EnableEvents bool `yaml:"enable_events" json:"enable_events"`

	// Timeout for requests to this backend
	Timeout time.Duration `yaml:"timeout" json:"timeout"`
}

// DefaultSTTBackendConfig returns default configuration for an STT backend.
func DefaultSTTBackendConfig(backend string) *STTBackendConfig {
	switch backend {
	case "sensevoice":
		return &STTBackendConfig{
			Enabled:       true,
			Endpoint:      "http://127.0.0.1:8881",
			Priority:      1, // Higher priority for emotion
			EnableEmotion: true,
			EnableEvents:  true,
			Timeout:       30 * time.Second,
		}
	case "whisper", "voicebox":
		return &STTBackendConfig{
			Enabled:       true,
			Endpoint:      "http://127.0.0.1:8880",
			Priority:      2,
			EnableEmotion: false,
			EnableEvents:  false,
			Timeout:       60 * time.Second,
		}
	default:
		return &STTBackendConfig{
			Enabled:  false,
			Priority: 99,
			Timeout:  30 * time.Second,
		}
	}
}
