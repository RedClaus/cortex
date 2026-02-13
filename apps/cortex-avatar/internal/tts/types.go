// Package tts provides Text-to-Speech synthesis services for CortexAvatar.
package tts

import (
	"context"
	"errors"
	"time"
)

// Common errors
var (
	ErrProviderUnavailable = errors.New("TTS provider unavailable")
	ErrVoiceNotFound       = errors.New("voice not found")
	ErrTextTooLong         = errors.New("text exceeds maximum length")
	ErrTimeout             = errors.New("synthesis timeout")
)

// Provider is the interface all TTS providers must implement
type Provider interface {
	// Name returns the provider identifier (e.g., "kokoro", "a2a")
	Name() string

	// Synthesize converts text to audio
	Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error)

	// SynthesizeStream handles streaming synthesis
	SynthesizeStream(ctx context.Context, req *SynthesizeRequest) (<-chan *AudioChunk, error)

	// ListVoices returns available voices
	ListVoices(ctx context.Context) ([]Voice, error)

	// Health checks if the provider is available
	Health(ctx context.Context) error

	// Capabilities returns the provider's feature set
	Capabilities() ProviderCapabilities
}

// SynthesizeRequest represents a synthesis request
type SynthesizeRequest struct {
	Text      string  `json:"text"`
	VoiceID   string  `json:"voice_id"`
	Speed     float64 `json:"speed,omitempty"`     // 0.5 to 2.0
	Pitch     float64 `json:"pitch,omitempty"`     // -1.0 to 1.0
	Format    string  `json:"format,omitempty"`    // wav, mp3, opus
	WithPhonemes bool `json:"with_phonemes,omitempty"` // Include phoneme data for lip-sync
}

// SynthesizeResponse represents a synthesis result
type SynthesizeResponse struct {
	Audio          []byte        `json:"audio"`           // Raw audio data
	Format         string        `json:"format"`          // Audio format
	SampleRate     int           `json:"sample_rate"`     // Sample rate in Hz
	Duration       time.Duration `json:"duration"`        // Audio duration
	ProcessingTime time.Duration `json:"processing_time"` // How long synthesis took
	VoiceID        string        `json:"voice_id"`        // Voice used
	Provider       string        `json:"provider"`        // Provider name
	Phonemes       []Phoneme     `json:"phonemes,omitempty"` // Phoneme data for lip-sync
	Error          string        `json:"error,omitempty"` // Error message if any
}

// AudioChunk represents a streaming audio chunk
type AudioChunk struct {
	Data       []byte        `json:"data"`       // Audio bytes
	Index      int           `json:"index"`      // Chunk index
	IsFinal    bool          `json:"is_final"`   // True if last chunk
	Phonemes   []Phoneme     `json:"phonemes,omitempty"` // Phonemes for this chunk
}

// Voice represents an available TTS voice
type Voice struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Language string            `json:"language"`
	Gender   string            `json:"gender"` // male, female, neutral
	Preview  string            `json:"preview,omitempty"` // URL to preview audio
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Phoneme represents a phoneme with timing for lip-sync
type Phoneme struct {
	Phoneme   string        `json:"phoneme"`   // IPA phoneme
	Viseme    string        `json:"viseme"`    // Visual phoneme for animation
	Start     time.Duration `json:"start"`     // Start time
	End       time.Duration `json:"end"`       // End time
	Duration  time.Duration `json:"duration"`  // Duration
}

// ProviderCapabilities describes what features a provider supports
type ProviderCapabilities struct {
	SupportsStreaming bool     `json:"supports_streaming"`
	SupportsCloning   bool     `json:"supports_cloning"`
	SupportsPhonemes  bool     `json:"supports_phonemes"` // For lip-sync
	SupportedLanguages []string `json:"supported_languages"`
	MaxTextLength     int      `json:"max_text_length"`
	AvgLatencyMs      int      `json:"avg_latency_ms"`
	RequiresGPU       bool     `json:"requires_gpu"`
	IsLocal           bool     `json:"is_local"`
}

// Config holds TTS configuration
type Config struct {
	Provider     string  `json:"provider"`       // Provider name
	DefaultVoice string  `json:"default_voice"`  // Default voice ID
	Speed        float64 `json:"speed"`          // Default speed
	EnablePhonemes bool  `json:"enable_phonemes"` // Enable phoneme extraction
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Provider:      "a2a",
		DefaultVoice:  "af_bella",
		Speed:         1.0,
		EnablePhonemes: true,
	}
}

// Viseme represents a mouth shape for lip-sync animation
type Viseme string

const (
	VisemeSilent  Viseme = "silent"  // Closed mouth
	VisemeAA      Viseme = "aa"      // Open mouth (father)
	VisemeEE      Viseme = "ee"      // Smile (see)
	VisemeII      Viseme = "ii"      // Narrow (sit)
	VisemeOO      Viseme = "oo"      // Rounded (boot)
	VisemeUU      Viseme = "uu"      // Tight rounded (book)
	VisemeFV      Viseme = "fv"      // Lip on teeth (five)
	VisemeTH      Viseme = "th"      // Tongue between teeth
	VisemeMBP     Viseme = "mbp"     // Closed lips (mother, boy, pan)
	VisemeLNTD    Viseme = "lntd"    // Tongue to roof (love, no, two, day)
	VisemeWQ      Viseme = "wq"      // Puckered (we, queen)
	VisemeSZ      Viseme = "sz"      // Teeth together (see, zoo)
	VisemeKG      Viseme = "kg"      // Back tongue (key, go)
	VisemeCHJ     Viseme = "chj"     // Puckered narrow (church, joy)
	VisemeR       Viseme = "r"       // Slight pucker (run)
)

// PhonemeToViseme maps IPA phonemes to visemes for lip-sync
var PhonemeToViseme = map[string]Viseme{
	// Vowels
	"ɑ": VisemeAA, "æ": VisemeAA, "ʌ": VisemeAA, "ə": VisemeAA,
	"i": VisemeEE, "ɪ": VisemeII, "e": VisemeEE, "ɛ": VisemeEE,
	"u": VisemeOO, "ʊ": VisemeUU, "o": VisemeOO, "ɔ": VisemeOO,

	// Consonants
	"p": VisemeMBP, "b": VisemeMBP, "m": VisemeMBP,
	"f": VisemeFV, "v": VisemeFV,
	"θ": VisemeTH, "ð": VisemeTH,
	"t": VisemeLNTD, "d": VisemeLNTD, "n": VisemeLNTD, "l": VisemeLNTD,
	"s": VisemeSZ, "z": VisemeSZ,
	"ʃ": VisemeCHJ, "ʒ": VisemeCHJ, "tʃ": VisemeCHJ, "dʒ": VisemeCHJ,
	"k": VisemeKG, "g": VisemeKG, "ŋ": VisemeKG,
	"r": VisemeR, "ɹ": VisemeR,
	"w": VisemeWQ,
	"j": VisemeEE,
	"h": VisemeAA,

	// Silence
	"": VisemeSilent, " ": VisemeSilent,
}
