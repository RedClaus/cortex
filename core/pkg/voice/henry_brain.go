// Package voice provides voice-related types and utilities for Cortex.
// henry_brain.go integrates state management with response generation for human-like interaction (CR-012-C).
package voice

import (
	"context"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// AudioSource represents a source of audio frames for continuous listening.
type AudioSource interface {
	// ReadFrame reads a single audio frame (typically 20-30ms of audio).
	// Returns the frame data or an error (io.EOF when done).
	ReadFrame() ([]byte, error)
	// Close releases any resources held by the audio source.
	Close() error
}

// HenryBrain integrates state management, response pools, and audio cache.
// It provides a unified interface for human-like voice interaction.
type HenryBrain struct {
	stateManager *ConversationStateManager
	responsePool *WakeResponsePool
	audioCache   *AudioCache
	audioPlayer  AudioPlayerInterface
	tts          TTSGenerator
	config       HenryBrainConfig

	// VAD-related fields (CR-013)
	vadClient      *VADClient
	vadMu          sync.RWMutex
	vadListening   bool
	vadCancelFunc  context.CancelFunc
	vadAudioBuffer []byte // Buffer for captured speech audio

	// CR-015: Wake word client for pre-STT hotword detection
	wakeWordClient *WakeWordClient

	// Callbacks for VAD events
	onSpeechDetected func(audioData []byte, durationMs int)

	// CR-015: Callback for wake word detection
	onWakeWordDetected func(wakeWord string, confidence float64)
}

// HenryBrainConfig holds configuration for HenryBrain.
type HenryBrainConfig struct {
	// CacheDir is the directory for pre-generated audio cache.
	CacheDir string
	// VoiceID is the TTS voice to use.
	VoiceID string
	// PersonaName is the assistant's name (e.g., "Henry" for male, "Hannah" for female).
	PersonaName string
	// WarmTimeout is the duration before Warm → Cold transition.
	WarmTimeout time.Duration
	// ActiveTimeout is the duration before Active → Warm transition.
	ActiveTimeout time.Duration
	// PreloadAudio enables preloading all audio into memory.
	PreloadAudio bool
	// MinSpeechForBackchannel is the minimum speech duration for backchanneling.
	MinSpeechForBackchannel time.Duration
	// ConfidenceThreshold is the minimum confidence for clear speech.
	ConfidenceThreshold float64

	// VAD Configuration (CR-013)
	VAD VADSettings `mapstructure:"vad" yaml:"vad"`

	// WakeWord Configuration (CR-015)
	WakeWord WakeWordSettings `mapstructure:"wake_word" yaml:"wake_word"`
}

// VADSettings holds VAD-specific configuration for HenryBrain.
type VADSettings struct {
	// Enabled enables VAD for continuous listening mode.
	Enabled bool `mapstructure:"enabled" yaml:"enabled"`
	// Endpoint is the Voice Box VAD WebSocket endpoint.
	Endpoint string `mapstructure:"endpoint" yaml:"endpoint"`
	// Mode is the VAD mode: "silero" (ML-based) or "energy" (simple threshold).
	Mode string `mapstructure:"mode" yaml:"mode"`
	// Threshold is the speech probability threshold (0.0-1.0).
	Threshold float64 `mapstructure:"threshold" yaml:"threshold"`
	// MinSpeechMs is the minimum speech duration to trigger (milliseconds).
	MinSpeechMs int `mapstructure:"min_speech_ms" yaml:"min_speech_ms"`
	// MinSilenceMs is the minimum silence to end speech (milliseconds).
	MinSilenceMs int `mapstructure:"min_silence_ms" yaml:"min_silence_ms"`
}

// WakeWordSettings holds wake word detection configuration for HenryBrain (CR-015).
type WakeWordSettings struct {
	// Enabled enables pre-STT wake word detection.
	Enabled bool `mapstructure:"enabled" yaml:"enabled"`
	// WakeWords is the list of wake words to detect.
	WakeWords []string `mapstructure:"wake_words" yaml:"wake_words"`
	// Threshold is the detection threshold (0.0-1.0).
	Threshold float64 `mapstructure:"threshold" yaml:"threshold"`
	// ModelDir is the directory containing custom wake word models.
	ModelDir string `mapstructure:"model_dir" yaml:"model_dir"`
}

// DefaultHenryBrainConfig returns production defaults.
func DefaultHenryBrainConfig() HenryBrainConfig {
	return HenryBrainConfig{
		CacheDir:                "~/.cortex/voicebox/audio_cache",
		VoiceID:                 "am_adam",
		PersonaName:             "Henry",
		WarmTimeout:             2 * time.Minute,
		ActiveTimeout:           30 * time.Second,
		PreloadAudio:            true,
		MinSpeechForBackchannel: 500 * time.Millisecond,
		ConfidenceThreshold:     0.6,
		VAD: VADSettings{
			Enabled:      false,
			Endpoint:     "ws://localhost:8880/v1/vad/stream",
			Mode:         "silero",
			Threshold:    0.5,
			MinSpeechMs:  250,
			MinSilenceMs: 300,
		},
		WakeWord: WakeWordSettings{
			Enabled:   true,
			WakeWords: []string{"hey_cortex", "hey_henry", "hey_hannah", "cortex", "henry", "hannah"},
			Threshold: 0.5,
			ModelDir:  "~/.cortex/voicebox/wake_word_models",
		},
	}
}

// NewHenryBrain creates a new HenryBrain instance.
func NewHenryBrain(config HenryBrainConfig, tts TTSGenerator, player AudioPlayerInterface) (*HenryBrain, error) {
	stateConfig := StateConfig{
		WarmTimeout:             config.WarmTimeout,
		ActiveTimeout:           config.ActiveTimeout,
		MinSpeechForBackchannel: config.MinSpeechForBackchannel,
		ConfidenceThreshold:     config.ConfidenceThreshold,
		BackchannelCooldown:     2 * time.Second,
	}

	stateManager := NewConversationStateManager(stateConfig)
	stateManager.SetAudioPlayer(player) // Enable interruption handling (Risk C)

	cacheConfig := AudioCacheConfig{
		VoiceID:    config.VoiceID,
		Model:      "kokoro",
		Speed:      1.0,
		SampleRate: 24000,
	}

	audioCache := NewAudioCache(config.CacheDir, cacheConfig)

	brain := &HenryBrain{
		stateManager: stateManager,
		responsePool: DefaultWakeResponsePool(),
		audioCache:   audioCache,
		audioPlayer:  player,
		tts:          tts,
		config:       config,
	}

	// Preload audio if configured
	if config.PreloadAudio {
		_ = audioCache.PreloadAll() // Non-fatal: will load on demand
	}

	return brain, nil
}

// HandleWakeWord handles wake word detection ("Hey Henry").
// Returns the response text and plays audio immediately for < 200ms latency.
func (h *HenryBrain) HandleWakeWord(ctx context.Context) (string, error) {
	// Check if this is the very first interaction (before recording)
	isFirstInteraction := h.stateManager.IsFirstInteraction()

	// Get current state BEFORE recording interaction
	state := h.stateManager.GetState()

	// Risk C: Stop any current playback (user is interrupting)
	h.stateManager.StopAudioIfPlaying()

	// Record the interaction (transitions state)
	h.stateManager.RecordInteraction(false)

	// Emit UI event for visual feedback sync
	h.stateManager.EmitUIEvent(UIEventListening)

	// Get appropriate response
	var response WakeResponse
	if isFirstInteraction && state == StateCold {
		// Very first interaction - use introduction with persona name
		response = h.responsePool.GetIntroductionResponse()
	} else {
		// Subsequent interactions - use state-appropriate response
		response = h.responsePool.GetWakeResponse(state)
	}

	// Replace {name} placeholder with actual persona name
	responseText := strings.Replace(response.Text, "{name}", h.config.PersonaName, -1)

	// Try to play pre-generated audio (fast path)
	if response.AudioFile != "" {
		audioData, err := h.audioCache.GetAudio(response.AudioFile)
		if err == nil && h.audioPlayer != nil {
			// Play immediately (< 100ms)
			go func() {
				h.stateManager.EmitUIEvent(UIEventSpeaking)
				_ = h.audioPlayer.PlayBytes(audioData)
				h.stateManager.EmitUIEvent(UIEventListening)
			}()
			return responseText, nil
		}
		// Fall through to TTS if cache miss
	}

	// Fallback: generate on-the-fly (slow path, ~1-2s)
	if h.tts != nil {
		go func() {
			h.stateManager.EmitUIEvent(UIEventSpeaking)
			// TTS would generate and play here
			h.stateManager.EmitUIEvent(UIEventListening)
		}()
	}

	return responseText, nil
}

// HandleLowConfidenceSpeech handles unclear/low-confidence input.
func (h *HenryBrain) HandleLowConfidenceSpeech(ctx context.Context, confidence float64) (string, error) {
	if !h.stateManager.IsLowConfidence(confidence) {
		return "", nil // Not confused, no response needed
	}

	response := h.responsePool.GetConfusedResponse()

	// Play pre-generated audio
	if response.AudioFile != "" {
		audioData, err := h.audioCache.GetAudio(response.AudioFile)
		if err == nil && h.audioPlayer != nil {
			go func() {
				h.stateManager.EmitUIEvent(UIEventSpeaking)
				_ = h.audioPlayer.PlayBytes(audioData)
				h.stateManager.EmitUIEvent(UIEventListening)
			}()
			return response.Text, nil
		}
	}

	return response.Text, nil
}

// HandleUserSpeechStart called when user begins speaking.
// Implements Risk C: stops Henry if he's speaking.
func (h *HenryBrain) HandleUserSpeechStart() {
	// Risk C: Stop Henry if he's speaking
	h.stateManager.StopAudioIfPlaying()
	h.stateManager.RecordInteraction(true)
	h.stateManager.EmitUIEvent(UIEventListening)
}

// HandleUserSpeechEnd called when user finishes speaking.
func (h *HenryBrain) HandleUserSpeechEnd() ConversationState {
	h.stateManager.EmitUIEvent(UIEventProcessing)
	return h.stateManager.GetState()
}

// HandleFarewell handles explicit goodbye.
func (h *HenryBrain) HandleFarewell(ctx context.Context) (string, error) {
	h.stateManager.EndConversation()

	farewell := h.responsePool.GetFarewellResponse()

	if farewell.AudioFile != "" {
		audioData, err := h.audioCache.GetAudio(farewell.AudioFile)
		if err == nil && h.audioPlayer != nil {
			go func() {
				h.stateManager.EmitUIEvent(UIEventSpeaking)
				_ = h.audioPlayer.PlayBytes(audioData)
				h.stateManager.EmitUIEvent(UIEventIdle)
			}()
			return farewell.Text, nil
		}
	}

	h.stateManager.EmitUIEvent(UIEventIdle)
	return farewell.Text, nil
}

// HandleAcknowledge plays an acknowledgment when starting a task.
func (h *HenryBrain) HandleAcknowledge(ctx context.Context) (string, error) {
	ack := h.responsePool.GetAcknowledgeResponse()

	if ack.AudioFile != "" {
		audioData, err := h.audioCache.GetAudio(ack.AudioFile)
		if err == nil && h.audioPlayer != nil {
			go func() {
				h.stateManager.EmitUIEvent(UIEventSpeaking)
				_ = h.audioPlayer.PlayBytes(audioData)
				h.stateManager.EmitUIEvent(UIEventProcessing)
			}()
			return ack.Text, nil
		}
	}

	return ack.Text, nil
}

// ShouldBackchannel determines if Henry should make an acknowledgment sound.
// Includes Risk A protections (duration + confidence thresholds).
func (h *HenryBrain) ShouldBackchannel(speechDuration time.Duration, confidence float64) bool {
	return h.stateManager.ShouldTriggerBackchannel(speechDuration, confidence)
}

// Backchannel plays a brief acknowledgment sound.
func (h *HenryBrain) Backchannel() string {
	response := h.responsePool.GetBackchannelResponse()

	if response.AudioFile != "" {
		audioData, err := h.audioCache.GetAudio(response.AudioFile)
		if err == nil && h.audioPlayer != nil {
			_ = h.audioPlayer.PlayBytes(audioData)
		}
	}

	h.stateManager.RecordBackchannel()
	return response.Text
}

// GetConversationContext returns context for LLM prompt enrichment.
func (h *HenryBrain) GetConversationContext() map[string]interface{} {
	state := h.stateManager.GetState()
	turns := h.stateManager.GetTurnCount()
	duration := h.stateManager.GetSessionDuration()
	formality := h.stateManager.GetFormality()

	return map[string]interface{}{
		"state":            string(state),
		"turn_count":       turns,
		"session_duration": duration.String(),
		"is_first_turn":    h.stateManager.IsFirstInteraction(),
		"formality":        formality,
	}
}

// GetState returns the current conversation state.
func (h *HenryBrain) GetState() ConversationState {
	return h.stateManager.GetState()
}

// GetFormality returns the current formality level.
func (h *HenryBrain) GetFormality() string {
	return h.stateManager.GetFormality()
}

// SetPersonaName updates the persona name (e.g., when voice changes).
func (h *HenryBrain) SetPersonaName(name string) {
	h.config.PersonaName = name
}

// GetPersonaName returns the current persona name.
func (h *HenryBrain) GetPersonaName() string {
	return h.config.PersonaName
}

// OnUIEvent registers a callback for UI synchronization.
func (h *HenryBrain) OnUIEvent(fn func(UIEvent)) {
	h.stateManager.OnUIEvent(fn)
}

// OnStateChange registers a callback for state transitions.
func (h *HenryBrain) OnStateChange(fn func(old, new ConversationState)) {
	h.stateManager.OnStateChange(fn)
}

// EnsureCacheReady generates audio cache if needed.
func (h *HenryBrain) EnsureCacheReady(ctx context.Context) error {
	if h.tts == nil {
		return nil
	}
	return h.audioCache.EnsureGenerated(ctx, h.tts)
}

// GetCacheStats returns audio cache statistics.
func (h *HenryBrain) GetCacheStats() (fileCount int, memorySize, diskSize int64) {
	return h.audioCache.FileCount(), h.audioCache.CacheSize(), h.audioCache.DiskSize()
}

// ClearCache clears the audio cache.
func (h *HenryBrain) ClearCache() error {
	return h.audioCache.Clear()
}

// ============================================================================
// VAD Integration Methods (CR-013)
// ============================================================================

// InitializeVAD creates and configures the VAD client with event callbacks.
// This connects VAD events to the conversation state machine.
func (h *HenryBrain) InitializeVAD(ctx context.Context) error {
	h.vadMu.Lock()
	defer h.vadMu.Unlock()

	if !h.config.VAD.Enabled {
		log.Debug().Msg("[HenryBrain] VAD not enabled, skipping initialization")
		return nil
	}

	if h.vadClient != nil {
		log.Debug().Msg("[HenryBrain] VAD client already initialized")
		return nil
	}

	// Create VAD client config
	vadConfig := VADClientConfig{
		Endpoint:      h.config.VAD.Endpoint,
		ReconnectWait: 2 * time.Second,
		MaxReconnects: 10,
		PingInterval:  30 * time.Second,
	}

	h.vadClient = NewVADClient(vadConfig)

	// Set up VAD event callbacks connected to state machine
	h.vadClient.OnSpeechStart = h.handleVADSpeechStart
	h.vadClient.OnSpeechEnd = h.handleVADSpeechEnd
	h.vadClient.OnInterrupt = h.handleVADInterrupt
	h.vadClient.OnError = h.handleVADError

	// Set up playback callbacks to coordinate VAD mode with TTS playback
	h.setupPlaybackCallbacks()

	log.Info().
		Str("endpoint", h.config.VAD.Endpoint).
		Str("mode", h.config.VAD.Mode).
		Float64("threshold", h.config.VAD.Threshold).
		Msg("[HenryBrain] VAD client initialized with playback coordination")

	return nil
}

// handleVADSpeechStart is called when VAD detects speech start.
// Implements Risk C: stops audio playback when user starts speaking.
func (h *HenryBrain) handleVADSpeechStart(event VADEvent) {
	log.Debug().
		Float64("confidence", event.Confidence).
		Msg("[HenryBrain] VAD speech_start detected")

	// Risk C: Stop any current audio playback (user is interrupting)
	h.stateManager.StopAudioIfPlaying()

	// Emit UI event for visual feedback
	h.stateManager.EmitUIEvent(UIEventListening)

	// Clear audio buffer for new speech segment
	h.vadMu.Lock()
	h.vadAudioBuffer = nil
	h.vadMu.Unlock()

	// Call existing speech start handler
	h.HandleUserSpeechStart()
}

// handleVADSpeechEnd is called when VAD detects speech end.
// Triggers STT processing with the buffered audio.
func (h *HenryBrain) handleVADSpeechEnd(event VADEvent, audioData []byte) {
	durationMs := int(event.DurationMs)
	log.Debug().
		Float64("confidence", event.Confidence).
		Int("duration_ms", durationMs).
		Int("audio_bytes", len(audioData)).
		Msg("[HenryBrain] VAD speech_end detected")

	// Emit processing state
	h.stateManager.EmitUIEvent(UIEventProcessing)

	// Call speech end handler to update state
	h.HandleUserSpeechEnd()

	// Trigger speech detected callback with audio for STT
	h.vadMu.RLock()
	callback := h.onSpeechDetected
	h.vadMu.RUnlock()

	if callback != nil && len(audioData) > 0 {
		go callback(audioData, durationMs)
	}
}

// handleVADError is called when the VAD client encounters an error.
func (h *HenryBrain) handleVADError(err error) {
	log.Error().Err(err).Msg("[HenryBrain] VAD client error")
}

// handleVADInterrupt is called when VAD detects user speech during TTS playback.
// This implements barge-in: the user wants to interrupt and take over the conversation.
func (h *HenryBrain) handleVADInterrupt(event VADEvent, audioData []byte) {
	log.Info().
		Float64("confidence", event.Confidence).
		Float64("duration_ms", event.DurationMs).
		Int("audio_bytes", len(audioData)).
		Msg("[HenryBrain] User interrupt detected, stopping TTS playback")

	// Stop current audio playback immediately
	if h.audioPlayer != nil {
		h.audioPlayer.Stop()
	}

	// Switch VAD back to full sensitivity mode
	h.vadMu.RLock()
	client := h.vadClient
	h.vadMu.RUnlock()

	if client != nil {
		if err := client.SetMode(VADModeFull); err != nil {
			log.Warn().Err(err).Msg("[HenryBrain] Failed to switch VAD to full mode after interrupt")
		} else {
			log.Debug().Msg("[HenryBrain] VAD mode switched to FULL after interrupt")
		}
	}

	// Emit UI event showing we're now listening
	h.stateManager.EmitUIEvent(UIEventListening)

	// Clear audio buffer for new speech segment
	h.vadMu.Lock()
	h.vadAudioBuffer = nil
	h.vadMu.Unlock()

	// Record user interaction
	h.stateManager.RecordInteraction(true)

	// Process the interrupt speech audio through the normal speech callback
	// The audio from the interrupt event contains the speech that caused the interrupt
	durationMs := int(event.DurationMs)

	h.vadMu.RLock()
	callback := h.onSpeechDetected
	h.vadMu.RUnlock()

	if callback != nil && len(audioData) > 0 {
		log.Debug().
			Int("duration_ms", durationMs).
			Int("audio_bytes", len(audioData)).
			Msg("[HenryBrain] Processing interrupt speech for STT")
		go callback(audioData, durationMs)
	}
}

// setupPlaybackCallbacks registers callbacks with the audio player to coordinate
// VAD mode changes with TTS playback state. This enables interrupt detection.
func (h *HenryBrain) setupPlaybackCallbacks() {
	if h.audioPlayer == nil {
		log.Debug().Msg("[HenryBrain] No audio player, skipping playback callbacks")
		return
	}

	h.audioPlayer.SetCallbacks(PlaybackCallbacks{
		OnPlaybackStart: func() {
			log.Debug().Msg("[HenryBrain] TTS playback started, switching VAD to playback mode")

			h.vadMu.RLock()
			client := h.vadClient
			h.vadMu.RUnlock()

			if client != nil && client.IsConnected() {
				if err := client.SetMode(VADModePlayback); err != nil {
					log.Warn().Err(err).Msg("[HenryBrain] Failed to set VAD playback mode")
				} else {
					log.Info().
						Str("mode", string(VADModePlayback)).
						Msg("[HenryBrain] VAD mode changed for TTS playback")
				}
			}
		},
		OnPlaybackEnd: func() {
			log.Debug().Msg("[HenryBrain] TTS playback ended, switching VAD to full mode")

			h.vadMu.RLock()
			client := h.vadClient
			h.vadMu.RUnlock()

			if client != nil && client.IsConnected() {
				if err := client.SetMode(VADModeFull); err != nil {
					log.Warn().Err(err).Msg("[HenryBrain] Failed to set VAD full mode")
				} else {
					log.Info().
						Str("mode", string(VADModeFull)).
						Msg("[HenryBrain] VAD mode changed back to full sensitivity")
				}
			}
		},
	})

	log.Debug().Msg("[HenryBrain] Playback callbacks registered for VAD coordination")
}

// StartListening starts continuous VAD-based listening mode.
// Audio frames from the source are sent to the VAD service for speech detection.
func (h *HenryBrain) StartListening(ctx context.Context, audioSource AudioSource) error {
	h.vadMu.Lock()

	if h.vadListening {
		h.vadMu.Unlock()
		return nil // Already listening
	}

	if h.vadClient == nil {
		h.vadMu.Unlock()
		// Initialize VAD if not done
		if err := h.InitializeVAD(ctx); err != nil {
			return err
		}
		h.vadMu.Lock()
	}

	// Connect to VAD service if not connected
	if !h.vadClient.IsConnected() {
		if err := h.vadClient.Connect(ctx); err != nil {
			h.vadMu.Unlock()
			return err
		}
	}

	// Create cancellable context for listening goroutine
	listenCtx, cancel := context.WithCancel(ctx)
	h.vadCancelFunc = cancel
	h.vadListening = true
	h.vadMu.Unlock()

	// Emit UI event
	h.stateManager.EmitUIEvent(UIEventListening)

	log.Info().Msg("[HenryBrain] Started continuous VAD listening")

	// Start audio capture goroutine
	go h.audioCaptureLop(listenCtx, audioSource)

	return nil
}

// audioCaptureLop reads audio frames from the source and sends to VAD.
func (h *HenryBrain) audioCaptureLop(ctx context.Context, source AudioSource) {
	defer func() {
		h.vadMu.Lock()
		h.vadListening = false
		h.vadMu.Unlock()
		log.Debug().Msg("[HenryBrain] Audio capture loop stopped")
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Read audio frame from source
		frame, err := source.ReadFrame()
		if err != nil {
			if err == io.EOF {
				log.Debug().Msg("[HenryBrain] Audio source EOF")
				return
			}
			log.Error().Err(err).Msg("[HenryBrain] Error reading audio frame")
			continue
		}

		// Send frame to VAD client
		h.vadMu.RLock()
		client := h.vadClient
		h.vadMu.RUnlock()

		if client != nil && client.IsConnected() {
			if err := client.SendAudioFrame(frame); err != nil {
				log.Debug().Err(err).Msg("[HenryBrain] Error sending audio frame to VAD")
			}
		}
	}
}

// StopListening stops the continuous VAD listening mode.
// The VAD connection is optionally kept open for quick restart.
func (h *HenryBrain) StopListening() error {
	h.vadMu.Lock()
	defer h.vadMu.Unlock()

	if !h.vadListening {
		return nil // Not listening
	}

	// Cancel the listening goroutine
	if h.vadCancelFunc != nil {
		h.vadCancelFunc()
		h.vadCancelFunc = nil
	}

	h.vadListening = false

	// Emit idle state
	h.stateManager.EmitUIEvent(UIEventIdle)

	log.Info().Msg("[HenryBrain] Stopped continuous VAD listening")
	return nil
}

// IsListening returns true if VAD continuous listening is active.
func (h *HenryBrain) IsListening() bool {
	h.vadMu.RLock()
	defer h.vadMu.RUnlock()
	return h.vadListening
}

// OnSpeechDetected registers a callback for when complete speech is detected.
// The callback receives the captured audio data and duration for STT processing.
func (h *HenryBrain) OnSpeechDetected(fn func(audioData []byte, durationMs int)) {
	h.vadMu.Lock()
	defer h.vadMu.Unlock()
	h.onSpeechDetected = fn
}

// GetVADClient returns the underlying VAD client for advanced usage.
func (h *HenryBrain) GetVADClient() *VADClient {
	h.vadMu.RLock()
	defer h.vadMu.RUnlock()
	return h.vadClient
}

// IsVADEnabled returns true if VAD is enabled in config.
func (h *HenryBrain) IsVADEnabled() bool {
	return h.config.VAD.Enabled
}

// CloseVAD closes the VAD client connection.
func (h *HenryBrain) CloseVAD() error {
	h.vadMu.Lock()
	defer h.vadMu.Unlock()

	// Stop listening first
	if h.vadListening && h.vadCancelFunc != nil {
		h.vadCancelFunc()
		h.vadCancelFunc = nil
		h.vadListening = false
	}

	// Close VAD client
	if h.vadClient != nil {
		err := h.vadClient.Close()
		h.vadClient = nil
		return err
	}

	return nil
}

// ============================================================================
// Wake Word Integration Methods (CR-015)
// ============================================================================

// InitializeWakeWord creates and configures the wake word client.
func (h *HenryBrain) InitializeWakeWord(ctx context.Context) error {
	h.vadMu.Lock()
	defer h.vadMu.Unlock()

	if !h.config.WakeWord.Enabled {
		log.Debug().Msg("[HenryBrain] Wake word detection not enabled")
		return nil
	}

	if h.wakeWordClient != nil {
		log.Debug().Msg("[HenryBrain] Wake word client already initialized")
		return nil
	}

	// Create wake word client config
	wakeWordConfig := WakeWordClientConfig{
		WakeWords: h.config.WakeWord.WakeWords,
		Threshold: h.config.WakeWord.Threshold,
		Enabled:   h.config.WakeWord.Enabled,
	}

	h.wakeWordClient = NewWakeWordClient(wakeWordConfig)

	// Set up callback
	h.wakeWordClient.OnWakeWord = h.handleWakeWordDetected

	log.Info().
		Strs("wake_words", h.config.WakeWord.WakeWords).
		Float64("threshold", h.config.WakeWord.Threshold).
		Msg("[HenryBrain] Wake word client initialized")

	return nil
}

// handleWakeWordDetected is called when a wake word is detected.
func (h *HenryBrain) handleWakeWordDetected(event WakeWordEvent) {
	log.Info().
		Str("wake_word", event.WakeWord).
		Float64("confidence", event.Confidence).
		Msg("[HenryBrain] Wake word detected via pre-STT hotword")

	// Map wake word to persona if applicable
	persona := h.mapWakeWordToPersona(event.WakeWord)
	if persona != "" && persona != h.config.PersonaName {
		log.Info().
			Str("old_persona", h.config.PersonaName).
			Str("new_persona", persona).
			Msg("[HenryBrain] Switching persona based on wake word")
		h.config.PersonaName = persona
	}

	// Emit UI event for visual feedback
	h.stateManager.EmitUIEvent(UIEventListening)

	// Fire external callback
	h.vadMu.RLock()
	callback := h.onWakeWordDetected
	h.vadMu.RUnlock()

	if callback != nil {
		go callback(event.WakeWord, event.Confidence)
	}
}

// mapWakeWordToPersona maps a wake word to a persona name.
func (h *HenryBrain) mapWakeWordToPersona(wakeWord string) string {
	switch wakeWord {
	case "hey_henry", "henry":
		return "Henry"
	case "hey_hannah", "hannah":
		return "Hannah"
	case "hey_cortex", "cortex":
		return "" // Keep current persona
	default:
		return ""
	}
}

// OnWakeWordDetected registers a callback for wake word detection events.
func (h *HenryBrain) OnWakeWordDetected(fn func(wakeWord string, confidence float64)) {
	h.vadMu.Lock()
	defer h.vadMu.Unlock()
	h.onWakeWordDetected = fn
}

// GetWakeWordClient returns the underlying wake word client for advanced usage.
func (h *HenryBrain) GetWakeWordClient() *WakeWordClient {
	h.vadMu.RLock()
	defer h.vadMu.RUnlock()
	return h.wakeWordClient
}

// IsWakeWordEnabled returns true if wake word detection is enabled.
func (h *HenryBrain) IsWakeWordEnabled() bool {
	return h.config.WakeWord.Enabled
}

// HandleWakeWordEvent processes a wake word event from the voice bridge.
// This is called when a wake_word message is received via WebSocket.
func (h *HenryBrain) HandleWakeWordEvent(wakeWord string, confidence float64) (string, error) {
	// Record interaction (transitions state)
	h.stateManager.RecordInteraction(false)

	// Get wake word response based on current state
	state := h.stateManager.GetState()
	response := h.responsePool.GetWakeResponse(state)

	log.Info().
		Str("wake_word", wakeWord).
		Float64("confidence", confidence).
		Str("state", string(state)).
		Str("response", response.Text).
		Msg("[HenryBrain] Handling wake word event")

	// Try to play pre-generated audio (fast path)
	if response.AudioFile != "" {
		audioData, err := h.audioCache.GetAudio(response.AudioFile)
		if err == nil && h.audioPlayer != nil {
			go func() {
				h.stateManager.EmitUIEvent(UIEventSpeaking)
				_ = h.audioPlayer.PlayBytes(audioData)
				h.stateManager.EmitUIEvent(UIEventListening)
			}()
			return response.Text, nil
		}
	}

	// Fallback: TTS would generate on-the-fly
	if h.tts != nil {
		go func() {
			h.stateManager.EmitUIEvent(UIEventSpeaking)
			h.stateManager.EmitUIEvent(UIEventListening)
		}()
	}

	return response.Text, nil
}
