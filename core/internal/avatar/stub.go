// Package avatar provides stub types for avatar-related functionality.
// This is a minimal stub to allow compilation. Full implementation TBD.
package avatar

import "time"

// Phoneme represents a phoneme for lip sync.
type Phoneme string

// EmotionState represents emotional state.
type EmotionState struct {
	Primary string
	Valence float64
	Arousal float64
}

// GazeState represents gaze direction.
type GazeState struct {
	X         float64
	Y         float64
	BlinkRate float64
}

// State represents the current avatar state.
type State struct {
	Phoneme    Phoneme
	Emotion    EmotionState
	Gaze       GazeState
	IsSpeaking bool
	IsThinking bool
}

// StateManager manages avatar state.
type StateManager struct{}

// NewStateManager creates a new state manager.
func NewStateManager() *StateManager { return &StateManager{} }

// SetEmotion sets the current emotion.
func (m *StateManager) SetEmotion(emotion string) {}

// GetCurrentState returns the current avatar state.
func (m *StateManager) GetCurrentState() *State { return &State{} }

// Subscribe subscribes to state updates.
func (m *StateManager) Subscribe() (<-chan *State, func()) {
	ch := make(chan *State)
	return ch, func() { close(ch) }
}

// PhonemeData represents phoneme timing data.
type PhonemeData struct {
	Phoneme   Phoneme
	StartMs   int64
	EndMs     int64
	Intensity float64
}

// PhonemeExtractor extracts phonemes from text.
type PhonemeExtractor interface {
	ExtractPhonemes(text string, durationMs int64) []PhonemeData
	PhonemeToViseme(p Phoneme) string
}

// AdvancedPhonemeExtractor implements PhonemeExtractor.
type AdvancedPhonemeExtractor struct{}

// NewAdvancedPhonemeExtractor creates a new extractor.
func NewAdvancedPhonemeExtractor() *AdvancedPhonemeExtractor { return &AdvancedPhonemeExtractor{} }

// ExtractPhonemes extracts phonemes from text.
func (e *AdvancedPhonemeExtractor) ExtractPhonemes(text string, durationMs int64) []PhonemeData {
	return nil
}

// PhonemeToViseme converts a phoneme to a viseme.
func (e *AdvancedPhonemeExtractor) PhonemeToViseme(p Phoneme) string { return "" }

// Used for compile but not implemented
var _ = time.Now
