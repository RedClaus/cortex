// Package audio provides audio capture, playback, and VAD for CortexAvatar.
package audio

import (
	"errors"
	"time"
)

// Common errors
var (
	ErrDeviceNotFound     = errors.New("audio device not found")
	ErrCaptureNotStarted  = errors.New("capture not started")
	ErrPlaybackNotStarted = errors.New("playback not started")
	ErrInvalidFormat      = errors.New("invalid audio format")
	ErrBufferFull         = errors.New("audio buffer full")
)

// AudioFormat represents audio encoding format
type AudioFormat string

const (
	FormatWAV  AudioFormat = "wav"
	FormatPCM  AudioFormat = "pcm"
	FormatWebM AudioFormat = "webm"
	FormatOpus AudioFormat = "opus"
	FormatMP3  AudioFormat = "mp3"
)

// AudioState represents the current audio system state
type AudioState string

const (
	StateIdle      AudioState = "idle"
	StateListening AudioState = "listening"
	StateSpeaking  AudioState = "speaking"
	StateThinking  AudioState = "thinking"
)

// AudioConfig holds audio system configuration
type AudioConfig struct {
	// Input settings
	InputDevice    string  `json:"input_device"`
	SampleRate     int     `json:"sample_rate"`      // Default: 16000 Hz for STT
	Channels       int     `json:"channels"`         // Default: 1 (mono)
	BitDepth       int     `json:"bit_depth"`        // Default: 16
	ChunkDurationMs int    `json:"chunk_duration_ms"` // Default: 100ms

	// VAD settings
	VADEnabled    bool    `json:"vad_enabled"`
	VADThreshold  float64 `json:"vad_threshold"`   // Default: 0.5
	VADPaddingMs  int     `json:"vad_padding_ms"`  // Silence padding before/after speech
	VADMinSpeechMs int    `json:"vad_min_speech_ms"` // Minimum speech duration

	// Output settings
	OutputDevice  string `json:"output_device"`
	OutputVolume  float64 `json:"output_volume"` // 0.0 to 1.0
}

// DefaultAudioConfig returns sensible defaults
func DefaultAudioConfig() *AudioConfig {
	return &AudioConfig{
		InputDevice:     "default",
		SampleRate:      16000,
		Channels:        1,
		BitDepth:        16,
		ChunkDurationMs: 100,
		VADEnabled:      true,
		VADThreshold:    0.5,
		VADPaddingMs:    300,
		VADMinSpeechMs:  250,
		OutputDevice:    "default",
		OutputVolume:    0.8,
	}
}

// AudioChunk represents a chunk of audio data
type AudioChunk struct {
	Data       []byte        `json:"data"`        // Raw audio bytes
	Format     AudioFormat   `json:"format"`      // Audio format
	SampleRate int           `json:"sample_rate"` // Sample rate in Hz
	Channels   int           `json:"channels"`    // Number of channels
	Duration   time.Duration `json:"duration"`    // Duration of this chunk
	Timestamp  time.Time     `json:"timestamp"`   // When this chunk was captured
	IsSpeech   bool          `json:"is_speech"`   // VAD result
	RMS        float64       `json:"rms"`         // Root mean square (volume level)
}

// VADResult represents the result of voice activity detection
type VADResult struct {
	IsSpeech   bool    `json:"is_speech"`
	Confidence float64 `json:"confidence"`
	RMS        float64 `json:"rms"`
}

// SpeechSegment represents a complete speech segment
type SpeechSegment struct {
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Duration  time.Duration `json:"duration"`
	Audio     []byte        `json:"audio"`
	Format    AudioFormat   `json:"format"`
}
