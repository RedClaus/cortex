// Package voice provides the HenryAdapter for connecting pkg/voice HenryBrain
// to internal/voice TTSEngine for audio playback and synthesis.
package voice

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	pkgvoice "github.com/normanking/cortex/pkg/voice"
	"github.com/rs/zerolog/log"
)

// HenryAdapter bridges pkg/voice.HenryBrain interfaces with internal/voice.TTSEngine.
// It implements both AudioPlayerInterface and TTSGenerator from pkg/voice.
type HenryAdapter struct {
	engine      *TTSEngine
	voiceBridge *VoiceBridge // Optional: for audio playback via orchestrator
	isPlaying   atomic.Bool
	callbacks   pkgvoice.PlaybackCallbacks
	callbacksMu sync.RWMutex
}

// NewHenryAdapter creates a new adapter wrapping a TTSEngine.
func NewHenryAdapter(engine *TTSEngine) *HenryAdapter {
	adapter := &HenryAdapter{
		engine: engine,
	}

	// Set up playback tracking with callback support
	engine.SetPlaybackFunc(func(audio []byte, format AudioFormat) error {
		// Get callbacks before starting playback
		adapter.callbacksMu.RLock()
		onStart := adapter.callbacks.OnPlaybackStart
		onEnd := adapter.callbacks.OnPlaybackEnd
		adapter.callbacksMu.RUnlock()

		// Trigger OnPlaybackStart callback
		if onStart != nil {
			onStart()
		}

		adapter.isPlaying.Store(true)
		defer func() {
			adapter.isPlaying.Store(false)
			// Trigger OnPlaybackEnd callback
			if onEnd != nil {
				onEnd()
			}
		}()

		// For now, we log the playback - in production this would
		// actually play the audio through the audio system
		log.Debug().
			Int("audio_bytes", len(audio)).
			Str("format", string(format)).
			Msg("[HenryAdapter] Would play audio")

		// TODO: Integrate with actual audio playback (portaudio, etc.)
		// For now, synthesis completes but playback is a no-op unless
		// the audio is sent to the voice orchestrator
		return nil
	})

	return adapter
}

// ══════════════════════════════════════════════════════════════════════════════
// AudioPlayerInterface implementation
// ══════════════════════════════════════════════════════════════════════════════

// Stop stops current audio playback immediately.
// Implements pkg/voice.AudioPlayerInterface.
func (a *HenryAdapter) Stop() {
	wasPlaying := a.isPlaying.Load()
	a.engine.StopSpeaking()
	a.isPlaying.Store(false)
	log.Debug().Msg("[HenryAdapter] Stopped playback")

	// Trigger OnPlaybackEnd callback if we were playing
	if wasPlaying {
		a.callbacksMu.RLock()
		onEnd := a.callbacks.OnPlaybackEnd
		a.callbacksMu.RUnlock()
		if onEnd != nil {
			onEnd()
		}
	}
}

// SetCallbacks registers playback event callbacks.
// Implements pkg/voice.AudioPlayerInterface.
func (a *HenryAdapter) SetCallbacks(callbacks pkgvoice.PlaybackCallbacks) {
	a.callbacksMu.Lock()
	defer a.callbacksMu.Unlock()
	a.callbacks = callbacks
	log.Debug().Msg("[HenryAdapter] Playback callbacks registered")
}

// IsPlaying returns true if audio is currently playing.
// Implements pkg/voice.AudioPlayerInterface.
func (a *HenryAdapter) IsPlaying() bool {
	return a.isPlaying.Load() || a.engine.State() == TTSStatePlaying
}

// PlayBytes plays raw audio data.
// Implements pkg/voice.AudioPlayerInterface.
func (a *HenryAdapter) PlayBytes(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	// Trigger OnPlaybackStart callback
	a.callbacksMu.RLock()
	onStart := a.callbacks.OnPlaybackStart
	onEnd := a.callbacks.OnPlaybackEnd
	a.callbacksMu.RUnlock()

	if onStart != nil {
		onStart()
	}

	a.isPlaying.Store(true)
	defer func() {
		a.isPlaying.Store(false)
		if onEnd != nil {
			onEnd()
		}
	}()

	log.Debug().
		Int("audio_bytes", len(data)).
		Msg("[HenryAdapter] Playing pre-cached audio")

	// If VoiceBridge is available, send audio to orchestrator for playback
	if a.voiceBridge != nil && a.voiceBridge.IsConnected() {
		log.Debug().Msg("[HenryAdapter] Sending audio via VoiceBridge")
		return a.voiceBridge.SendAudio(data, "wav")
	}

	// Fallback: log that we can't play (no bridge connected)
	log.Warn().Msg("[HenryAdapter] Cannot play audio - VoiceBridge not connected")
	return nil
}

// ══════════════════════════════════════════════════════════════════════════════
// TTSGenerator implementation
// ══════════════════════════════════════════════════════════════════════════════

// SynthesizeToFile generates audio and saves to file.
// Implements pkg/voice.TTSGenerator.
func (a *HenryAdapter) SynthesizeToFile(ctx context.Context, text, outputPath, voiceID string) error {
	if text == "" {
		return fmt.Errorf("empty text")
	}

	// Ensure output directory exists
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Use the engine's synthesize method (we need to access it)
	// Since synthesize is private, we'll use SpeakSyncWithVoice and capture the audio
	audio, err := a.synthesize(ctx, text, voiceID)
	if err != nil {
		return fmt.Errorf("synthesis failed: %w", err)
	}

	// Write audio to file
	if err := os.WriteFile(outputPath, audio, 0644); err != nil {
		return fmt.Errorf("failed to write audio file: %w", err)
	}

	log.Debug().
		Str("text", truncateText(text, 30)).
		Str("voice", voiceID).
		Str("output", outputPath).
		Int("bytes", len(audio)).
		Msg("[HenryAdapter] Synthesized audio to file")

	return nil
}

// synthesize generates audio bytes from text using the TTS engine.
func (a *HenryAdapter) synthesize(ctx context.Context, text, voiceID string) ([]byte, error) {
	// Create a temporary capture of the audio
	var capturedAudio []byte

	// Store original playback function
	originalFn := a.engine.playbackFn

	// Set temporary capture function
	a.engine.SetPlaybackFunc(func(audio []byte, format AudioFormat) error {
		capturedAudio = make([]byte, len(audio))
		copy(capturedAudio, audio)
		return nil
	})

	// Restore original after
	defer func() {
		a.engine.SetPlaybackFunc(originalFn)
	}()

	// Use the engine to synthesize
	speed := a.engine.config.Speed
	if speed == 0 {
		speed = 1.0
	}
	if voiceID == "" {
		voiceID = a.engine.config.VoiceID
	}

	err := a.engine.SpeakSyncWithVoice(ctx, text, voiceID, speed)
	if err != nil {
		return nil, err
	}

	if len(capturedAudio) == 0 {
		return nil, fmt.Errorf("no audio captured")
	}

	return capturedAudio, nil
}

// ══════════════════════════════════════════════════════════════════════════════
// Additional convenience methods
// ══════════════════════════════════════════════════════════════════════════════

// Speak synthesizes and plays text (convenience wrapper).
func (a *HenryAdapter) Speak(text string) error {
	return a.engine.Speak(text)
}

// SpeakSync synthesizes and plays text, blocking until complete.
func (a *HenryAdapter) SpeakSync(ctx context.Context, text string) error {
	return a.engine.SpeakSync(ctx, text)
}

// IsAvailable checks if TTS is available.
func (a *HenryAdapter) IsAvailable() bool {
	return a.engine.IsAvailable()
}

// Engine returns the underlying TTSEngine.
func (a *HenryAdapter) Engine() *TTSEngine {
	return a.engine
}

// SetVoiceBridge sets the VoiceBridge for audio playback via orchestrator.
// This enables the fast path for pre-cached audio playback.
func (a *HenryAdapter) SetVoiceBridge(bridge *VoiceBridge) {
	a.voiceBridge = bridge
	log.Debug().Msg("[HenryAdapter] VoiceBridge set for audio playback")
}
