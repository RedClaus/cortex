// Package voice provides voice processing capabilities for Cortex.
package voice

import (
	"sync"
)

// ═══════════════════════════════════════════════════════════════════════════════
// MODE DETECTOR - Thread-safe voice/text mode detection
// NFR-002: Mode detection overhead < 1ms
// ═══════════════════════════════════════════════════════════════════════════════

// OutputMode represents the current output mode for responses.
type OutputMode int

const (
	// ModeText indicates text-only output mode.
	ModeText OutputMode = iota

	// ModeVoice indicates voice output mode with TTS synthesis.
	ModeVoice
)

// String returns the string representation of the output mode.
func (m OutputMode) String() string {
	switch m {
	case ModeVoice:
		return "voice"
	default:
		return "text"
	}
}

// ModeDetector provides thread-safe detection of voice vs text mode.
// Voice mode is active when: explicit voice mode is set OR (STT is active AND TTS is enabled).
type ModeDetector struct {
	sttActive    bool       // Whether speech-to-text is currently active (user speaking)
	ttsEnabled   bool       // Whether text-to-speech is available/enabled
	explicitMode OutputMode // Explicitly set mode override
	hasExplicit  bool       // Whether explicit mode has been set
	mu           sync.RWMutex
}

// NewModeDetector creates a new ModeDetector with default settings (text mode).
func NewModeDetector() *ModeDetector {
	return &ModeDetector{
		sttActive:    false,
		ttsEnabled:   false,
		explicitMode: ModeText,
		hasExplicit:  false,
	}
}

// SetSTTActive sets whether speech-to-text transcription is currently active.
// This is typically set to true when the user begins speaking and false when they stop.
func (d *ModeDetector) SetSTTActive(active bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.sttActive = active
}

// STTActive returns whether speech-to-text is currently active.
func (d *ModeDetector) STTActive() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.sttActive
}

// SetTTSEnabled sets whether text-to-speech is available and enabled.
// This is typically set during initialization based on TTS provider availability.
func (d *ModeDetector) SetTTSEnabled(enabled bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ttsEnabled = enabled
}

// TTSEnabled returns whether text-to-speech is enabled.
func (d *ModeDetector) TTSEnabled() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.ttsEnabled
}

// SetExplicitMode sets an explicit mode override.
// When an explicit mode is set, it takes precedence over automatic detection.
func (d *ModeDetector) SetExplicitMode(mode OutputMode) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.explicitMode = mode
	d.hasExplicit = true
}

// ClearExplicitMode clears the explicit mode override, reverting to automatic detection.
func (d *ModeDetector) ClearExplicitMode() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.hasExplicit = false
}

// HasExplicitMode returns whether an explicit mode has been set.
func (d *ModeDetector) HasExplicitMode() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.hasExplicit
}

// CurrentMode returns the current output mode.
// The mode is determined by:
// 1. If an explicit mode is set, return that mode
// 2. Otherwise, return ModeVoice if (STT is active AND TTS is enabled)
// 3. Otherwise, return ModeText
func (d *ModeDetector) CurrentMode() OutputMode {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.currentModeLocked()
}

// currentModeLocked returns the current mode without acquiring the lock.
// Caller must hold at least a read lock.
func (d *ModeDetector) currentModeLocked() OutputMode {
	// Explicit mode takes precedence
	if d.hasExplicit {
		return d.explicitMode
	}

	// Automatic detection: voice mode when STT is active and TTS is enabled
	if d.sttActive && d.ttsEnabled {
		return ModeVoice
	}

	return ModeText
}

// IsVoiceMode returns true if the current mode is voice mode.
// This is a convenience method equivalent to CurrentMode() == ModeVoice.
func (d *ModeDetector) IsVoiceMode() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.currentModeLocked() == ModeVoice
}

// IsTextMode returns true if the current mode is text mode.
// This is a convenience method equivalent to CurrentMode() == ModeText.
func (d *ModeDetector) IsTextMode() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.currentModeLocked() == ModeText
}

// State returns a snapshot of the current detector state.
// This is useful for debugging and testing.
type ModeState struct {
	STTActive    bool       `json:"stt_active"`
	TTSEnabled   bool       `json:"tts_enabled"`
	ExplicitMode OutputMode `json:"explicit_mode"`
	HasExplicit  bool       `json:"has_explicit"`
	CurrentMode  OutputMode `json:"current_mode"`
}

// State returns a snapshot of the current detector state.
func (d *ModeDetector) State() ModeState {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return ModeState{
		STTActive:    d.sttActive,
		TTSEnabled:   d.ttsEnabled,
		ExplicitMode: d.explicitMode,
		HasExplicit:  d.hasExplicit,
		CurrentMode:  d.currentModeLocked(),
	}
}

// Reset clears all mode detection state to defaults.
func (d *ModeDetector) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.sttActive = false
	d.ttsEnabled = false
	d.explicitMode = ModeText
	d.hasExplicit = false
}
