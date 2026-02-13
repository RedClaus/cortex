package audio

import (
	"context"
	"encoding/base64"
	"sync"
	"time"

	"github.com/normanking/cortexavatar/internal/bus"
	"github.com/rs/zerolog"
)

// Manager coordinates audio capture, playback, and VAD.
// Audio I/O happens in the browser; this manages state and coordination.
type Manager struct {
	config     *AudioConfig
	state      AudioState
	stateMu    sync.RWMutex
	eventBus   *bus.EventBus
	logger     zerolog.Logger
	ctx        context.Context
	cancel     context.CancelFunc

	// Speech accumulator
	speechBuffer  []byte
	speechStart   time.Time
	speechActive  bool
	speechMu      sync.Mutex

	// Callbacks
	onSpeechStart   func()
	onSpeechEnd     func(audio []byte, duration time.Duration)
	onAudioChunk    func(chunk *AudioChunk)
	callbackMu      sync.RWMutex

	// Playback queue
	playbackQueue   [][]byte
	playbackMu      sync.Mutex
	isPlaying       bool
}

// NewManager creates a new audio manager
func NewManager(config *AudioConfig, eventBus *bus.EventBus, logger zerolog.Logger) *Manager {
	if config == nil {
		config = DefaultAudioConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		config:        config,
		state:         StateIdle,
		eventBus:      eventBus,
		logger:        logger.With().Str("component", "audio").Logger(),
		ctx:           ctx,
		cancel:        cancel,
		speechBuffer:  make([]byte, 0, 16000*2*10), // 10 seconds at 16kHz mono 16-bit
		playbackQueue: make([][]byte, 0),
	}
}

// Start initializes the audio manager
func (m *Manager) Start() error {
	m.logger.Info().Msg("Audio manager started")
	return nil
}

// Stop shuts down the audio manager
func (m *Manager) Stop() {
	m.cancel()
	m.logger.Info().Msg("Audio manager stopped")
}

// GetState returns the current audio state
func (m *Manager) GetState() AudioState {
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()
	return m.state
}

// SetState updates the audio state and emits an event
func (m *Manager) SetState(state AudioState) {
	m.stateMu.Lock()
	oldState := m.state
	m.state = state
	m.stateMu.Unlock()

	if oldState != state {
		m.logger.Info().Str("old", string(oldState)).Str("new", string(state)).Msg("Audio state changed")
		if m.eventBus != nil {
			m.eventBus.Publish(bus.Event{
				Type: bus.EventTypeAudioStateChanged,
				Data: map[string]any{
					"old_state": string(oldState),
					"new_state": string(state),
				},
			})
		}
	}
}

// StartListening transitions to listening state
func (m *Manager) StartListening() {
	m.SetState(StateListening)
}

// StopListening transitions to idle state
func (m *Manager) StopListening() {
	m.SetState(StateIdle)
}

// StartSpeaking transitions to speaking state
func (m *Manager) StartSpeaking() {
	m.SetState(StateSpeaking)
}

// StopSpeaking transitions back to listening or idle
func (m *Manager) StopSpeaking() {
	// Go back to listening if we were listening before
	m.SetState(StateListening)
}

// OnSpeechStart registers a callback for when speech starts
func (m *Manager) OnSpeechStart(callback func()) {
	m.callbackMu.Lock()
	defer m.callbackMu.Unlock()
	m.onSpeechStart = callback
}

// OnSpeechEnd registers a callback for when speech ends
func (m *Manager) OnSpeechEnd(callback func(audio []byte, duration time.Duration)) {
	m.callbackMu.Lock()
	defer m.callbackMu.Unlock()
	m.onSpeechEnd = callback
}

// OnAudioChunk registers a callback for audio chunks
func (m *Manager) OnAudioChunk(callback func(chunk *AudioChunk)) {
	m.callbackMu.Lock()
	defer m.callbackMu.Unlock()
	m.onAudioChunk = callback
}

// ProcessAudioChunk handles incoming audio data from the frontend
// audioBase64 is base64-encoded PCM audio data
func (m *Manager) ProcessAudioChunk(audioBase64 string, isSpeech bool, rms float64) {
	// Decode audio
	audioData, err := base64.StdEncoding.DecodeString(audioBase64)
	if err != nil {
		m.logger.Error().Err(err).Msg("Failed to decode audio chunk")
		return
	}

	// Create chunk
	chunk := &AudioChunk{
		Data:       audioData,
		Format:     FormatPCM,
		SampleRate: m.config.SampleRate,
		Channels:   m.config.Channels,
		Timestamp:  time.Now(),
		IsSpeech:   isSpeech,
		RMS:        rms,
	}

	// Calculate duration
	bytesPerSample := m.config.BitDepth / 8
	samples := len(audioData) / (bytesPerSample * m.config.Channels)
	chunk.Duration = time.Duration(samples) * time.Second / time.Duration(m.config.SampleRate)

	// Invoke callback
	m.callbackMu.RLock()
	callback := m.onAudioChunk
	m.callbackMu.RUnlock()

	if callback != nil {
		callback(chunk)
	}

	// Handle speech detection
	m.handleSpeechDetection(chunk)
}

// handleSpeechDetection accumulates speech segments
func (m *Manager) handleSpeechDetection(chunk *AudioChunk) {
	m.speechMu.Lock()
	defer m.speechMu.Unlock()

	if chunk.IsSpeech {
		if !m.speechActive {
			// Speech started
			m.speechActive = true
			m.speechStart = chunk.Timestamp
			m.speechBuffer = m.speechBuffer[:0]

			m.logger.Debug().Msg("Speech started")

			// Invoke callback
			m.callbackMu.RLock()
			callback := m.onSpeechStart
			m.callbackMu.RUnlock()

			if callback != nil {
				go callback()
			}

			// Publish event
			if m.eventBus != nil {
				m.eventBus.Publish(bus.Event{
					Type: bus.EventTypeSpeechStart,
					Data: map[string]any{
						"timestamp": chunk.Timestamp,
					},
				})
			}
		}

		// Accumulate audio
		m.speechBuffer = append(m.speechBuffer, chunk.Data...)
	} else if m.speechActive {
		// Speech ended
		m.speechActive = false
		duration := time.Since(m.speechStart)

		// Copy buffer
		audio := make([]byte, len(m.speechBuffer))
		copy(audio, m.speechBuffer)

		m.logger.Debug().Dur("duration", duration).Int("bytes", len(audio)).Msg("Speech ended")

		// Invoke callback
		m.callbackMu.RLock()
		callback := m.onSpeechEnd
		m.callbackMu.RUnlock()

		if callback != nil {
			go callback(audio, duration)
		}

		// Publish event
		if m.eventBus != nil {
			m.eventBus.Publish(bus.Event{
				Type: bus.EventTypeSpeechEnd,
				Data: map[string]any{
					"duration":  duration,
					"audio_len": len(audio),
				},
			})
		}

		// Clear buffer
		m.speechBuffer = m.speechBuffer[:0]
	}
}

// QueuePlayback adds audio to the playback queue
func (m *Manager) QueuePlayback(audioData []byte) {
	m.playbackMu.Lock()
	m.playbackQueue = append(m.playbackQueue, audioData)
	m.playbackMu.Unlock()

	m.logger.Debug().Int("bytes", len(audioData)).Int("queue_len", len(m.playbackQueue)).Msg("Audio queued for playback")
}

// GetNextPlayback returns the next audio chunk to play, or nil if empty
func (m *Manager) GetNextPlayback() []byte {
	m.playbackMu.Lock()
	defer m.playbackMu.Unlock()

	if len(m.playbackQueue) == 0 {
		return nil
	}

	audio := m.playbackQueue[0]
	m.playbackQueue = m.playbackQueue[1:]
	return audio
}

// ClearPlaybackQueue clears all pending audio
func (m *Manager) ClearPlaybackQueue() {
	m.playbackMu.Lock()
	m.playbackQueue = m.playbackQueue[:0]
	m.playbackMu.Unlock()

	m.logger.Debug().Msg("Playback queue cleared")
}

// SetPlaying marks whether audio is currently playing
func (m *Manager) SetPlaying(playing bool) {
	m.playbackMu.Lock()
	m.isPlaying = playing
	m.playbackMu.Unlock()

	if playing {
		m.SetState(StateSpeaking)
	} else {
		m.SetState(StateListening)
	}
}

// IsPlaying returns whether audio is currently playing
func (m *Manager) IsPlaying() bool {
	m.playbackMu.Lock()
	defer m.playbackMu.Unlock()
	return m.isPlaying
}

// GetConfig returns the current audio configuration
func (m *Manager) GetConfig() *AudioConfig {
	return m.config
}

// UpdateConfig updates audio configuration
func (m *Manager) UpdateConfig(config *AudioConfig) {
	m.config = config
	m.logger.Info().Interface("config", config).Msg("Audio config updated")
}
