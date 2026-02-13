package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// TTSConfig holds configuration for the TTS engine.
type TTSConfig struct {
	// Endpoint is the TTS API endpoint (e.g., http://localhost:8880/v1/audio/speech)
	Endpoint string

	// VoiceID is the default voice to use (e.g., "am_adam")
	VoiceID string

	// Model is the TTS model name (e.g., "kokoro")
	Model string

	// ResponseFormat is the audio output format (e.g., "wav")
	ResponseFormat string

	// Speed is the playback speed multiplier (0.5-2.0)
	Speed float64

	// Timeout is the HTTP request timeout
	Timeout time.Duration

	// QueueSize is the maximum number of audio jobs to queue
	QueueSize int
}

// DefaultTTSConfig returns sensible defaults for TTS configuration.
func DefaultTTSConfig() TTSConfig {
	return TTSConfig{
		Endpoint:       "http://localhost:8880/v1/audio/speech",
		VoiceID:        "am_adam",
		Model:          "kokoro",
		ResponseFormat: "wav",
		Speed:          1.0,
		Timeout:        60 * time.Second, // Longer timeout for first model download
		QueueSize:      100,
	}
}

// audioJob represents a queued synthesis and playback job.
type audioJob struct {
	text      string
	voiceID   string
	speed     float64
	audio     []byte
	errCh     chan error
	ctx       context.Context
	cancelFn  context.CancelFunc
	processed bool
}

// TTSEngineState represents the current state of the TTS engine.
type TTSEngineState int32

const (
	// TTSStateIdle indicates the engine is ready but not playing.
	TTSStateIdle TTSEngineState = iota

	// TTSStatePlaying indicates the engine is actively playing audio.
	TTSStatePlaying

	// TTSStatePaused indicates the engine is paused (barge-in occurred).
	TTSStatePaused

	// TTSStateStopped indicates the engine has been stopped.
	TTSStateStopped
)

// String returns a string representation of the state.
func (s TTSEngineState) String() string {
	switch s {
	case TTSStateIdle:
		return "idle"
	case TTSStatePlaying:
		return "playing"
	case TTSStatePaused:
		return "paused"
	case TTSStateStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// TTSEngine provides high-performance text-to-speech with:
// - Async queueing for non-blocking speech
// - Persistent audio player for gap-free playback
// - Barge-in support for user interruption
// - Health checking and graceful shutdown
// - Auto-launch of Voice Box sidecar (CR-012)
type TTSEngine struct {
	config     TTSConfig
	httpClient *http.Client

	// Voice Box launcher for auto-start (CR-012)
	launcher *VoiceBoxLauncher

	// Audio job queue
	audioQueue chan *audioJob

	// State management
	state    atomic.Int32
	stateMu  sync.RWMutex
	pausedMu sync.Mutex

	// Interrupt/barge-in control
	interruptCh chan struct{}
	interruptMu sync.Mutex
	interrupted atomic.Bool

	// Lifecycle management
	ctx       context.Context
	cancelFn  context.CancelFunc
	wg        sync.WaitGroup
	startOnce sync.Once
	stopOnce  sync.Once

	// Metrics
	synthesisCount  atomic.Int64
	synthesisErrors atomic.Int64
	playbackCount   atomic.Int64
	interruptCount  atomic.Int64
	totalLatencyMs  atomic.Int64

	// Callback for audio playback (set by integrator)
	playbackFn func(audio []byte, format AudioFormat) error
}

// NewTTSEngine creates a new TTS engine with the given configuration.
// The engine starts background workers for processing the audio queue.
// If no endpoint is configured, it will use the VoiceBoxLauncher (CR-012).
func NewTTSEngine(config TTSConfig) *TTSEngine {
	if config.Endpoint == "" {
		config = DefaultTTSConfig()
	}
	if config.QueueSize == 0 {
		config.QueueSize = 100
	}
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Initialize Voice Box launcher for auto-start (CR-012)
	launcher := GetVoiceBoxLauncher()

	// If using default endpoint, use launcher's endpoint
	if config.Endpoint == DefaultTTSConfig().Endpoint {
		config.Endpoint = launcher.SpeechEndpoint()
	}

	engine := &TTSEngine{
		config:   config,
		launcher: launcher,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		audioQueue:  make(chan *audioJob, config.QueueSize),
		interruptCh: make(chan struct{}, 1),
		ctx:         ctx,
		cancelFn:    cancel,
	}

	engine.state.Store(int32(TTSStateIdle))

	// Start the audio processing worker
	engine.startOnce.Do(func() {
		engine.wg.Add(1)
		go engine.processQueue()
	})

	log.Info().
		Str("endpoint", config.Endpoint).
		Str("voice", config.VoiceID).
		Str("model", config.Model).
		Bool("voicebox_installed", launcher.IsInstalled()).
		Msg("TTS engine initialized")

	return engine
}

// SetPlaybackFunc sets the function used to play audio.
// This allows integration with different audio playback systems.
func (e *TTSEngine) SetPlaybackFunc(fn func(audio []byte, format AudioFormat) error) {
	e.playbackFn = fn
}

// Speak queues text for synthesis and playback without blocking.
// Returns immediately after queueing. Use SpeakSync for blocking playback.
func (e *TTSEngine) Speak(text string) error {
	return e.SpeakWithVoice(text, e.config.VoiceID, e.config.Speed)
}

// SpeakWithVoice queues text for synthesis with a specific voice.
func (e *TTSEngine) SpeakWithVoice(text, voiceID string, speed float64) error {
	if e.State() == TTSStateStopped {
		return fmt.Errorf("TTS engine is stopped")
	}

	if text == "" {
		return nil
	}

	if voiceID == "" {
		voiceID = e.config.VoiceID
	}
	if speed == 0 {
		speed = e.config.Speed
	}

	ctx, cancel := context.WithCancel(e.ctx)
	job := &audioJob{
		text:     text,
		voiceID:  voiceID,
		speed:    speed,
		errCh:    make(chan error, 1),
		ctx:      ctx,
		cancelFn: cancel,
	}

	select {
	case e.audioQueue <- job:
		log.Debug().
			Str("text", truncateText(text, 50)).
			Str("voice", voiceID).
			Msg("queued TTS job")
		return nil
	default:
		cancel()
		return fmt.Errorf("audio queue full")
	}
}

// SpeakSync synthesizes and plays text synchronously, blocking until complete.
func (e *TTSEngine) SpeakSync(ctx context.Context, text string) error {
	return e.SpeakSyncWithVoice(ctx, text, e.config.VoiceID, e.config.Speed)
}

// SpeakSyncWithVoice synthesizes and plays text with a specific voice, blocking until complete.
func (e *TTSEngine) SpeakSyncWithVoice(ctx context.Context, text, voiceID string, speed float64) error {
	if e.State() == TTSStateStopped {
		return fmt.Errorf("TTS engine is stopped")
	}

	if text == "" {
		return nil
	}

	if voiceID == "" {
		voiceID = e.config.VoiceID
	}
	if speed == 0 {
		speed = e.config.Speed
	}

	start := time.Now()

	// Synthesize audio
	audio, err := e.synthesize(ctx, text, voiceID, speed)
	if err != nil {
		e.synthesisErrors.Add(1)
		return err
	}

	e.synthesisCount.Add(1)
	e.totalLatencyMs.Add(time.Since(start).Milliseconds())

	// Play audio
	err = e.playAudio(ctx, audio)
	if err == nil {
		e.playbackCount.Add(1)
	}
	return err
}

// StopSpeaking immediately stops all audio playback and drains the queue.
// This implements BARGE-IN functionality for user interruption.
func (e *TTSEngine) StopSpeaking() {
	e.interruptMu.Lock()
	defer e.interruptMu.Unlock()

	// Signal interrupt
	e.interrupted.Store(true)
	e.interruptCount.Add(1)

	// Send interrupt signal (non-blocking)
	select {
	case e.interruptCh <- struct{}{}:
	default:
	}

	// Drain the queue
	drainCount := 0
	for {
		select {
		case job := <-e.audioQueue:
			job.cancelFn()
			job.errCh <- fmt.Errorf("interrupted by barge-in")
			drainCount++
		default:
			goto done
		}
	}
done:

	e.state.Store(int32(TTSStatePaused))

	log.Info().
		Int("drained", drainCount).
		Msg("TTS playback stopped (barge-in)")
}

// Resume allows playback to continue after an interruption.
func (e *TTSEngine) Resume() {
	e.interruptMu.Lock()
	defer e.interruptMu.Unlock()

	e.interrupted.Store(false)

	// Clear any pending interrupt signals
	select {
	case <-e.interruptCh:
	default:
	}

	e.state.Store(int32(TTSStateIdle))

	log.Debug().Msg("TTS playback resumed")
}

// IsAvailable checks if the TTS service is available and responding.
// CR-012: Also checks if Voice Box is installed when using native sidecar.
func (e *TTSEngine) IsAvailable() bool {
	// CR-012: Only use launcher checks if we're using the launcher's endpoint
	usingLauncherEndpoint := e.launcher != nil && e.config.Endpoint == e.launcher.SpeechEndpoint()

	if usingLauncherEndpoint {
		// Check if Voice Box is installed
		if !e.launcher.IsInstalled() {
			return false
		}
		// If launcher is healthy, we're available
		if e.launcher.IsHealthy() {
			return true
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Try to reach the health endpoint
	healthURL := e.config.Endpoint[:len(e.config.Endpoint)-len("/v1/audio/speech")] + "/health"

	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		return false
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// State returns the current engine state.
func (e *TTSEngine) State() TTSEngineState {
	return TTSEngineState(e.state.Load())
}

// QueueLength returns the number of jobs waiting in the queue.
func (e *TTSEngine) QueueLength() int {
	return len(e.audioQueue)
}

// Metrics returns engine performance metrics.
func (e *TTSEngine) Metrics() TTSMetrics {
	count := e.synthesisCount.Load()
	avgLatency := int64(0)
	if count > 0 {
		avgLatency = e.totalLatencyMs.Load() / count
	}

	return TTSMetrics{
		SynthesisCount:   count,
		SynthesisErrors:  e.synthesisErrors.Load(),
		PlaybackCount:    e.playbackCount.Load(),
		InterruptCount:   e.interruptCount.Load(),
		QueueLength:      len(e.audioQueue),
		AverageLatencyMs: avgLatency,
		State:            e.State().String(),
	}
}

// TTSMetrics contains performance metrics for the TTS engine.
type TTSMetrics struct {
	SynthesisCount   int64  `json:"synthesis_count"`
	SynthesisErrors  int64  `json:"synthesis_errors"`
	PlaybackCount    int64  `json:"playback_count"`
	InterruptCount   int64  `json:"interrupt_count"`
	QueueLength      int    `json:"queue_length"`
	AverageLatencyMs int64  `json:"average_latency_ms"`
	State            string `json:"state"`
}

// Stop gracefully shuts down the TTS engine.
func (e *TTSEngine) Stop() {
	e.stopOnce.Do(func() {
		log.Info().Msg("stopping TTS engine")

		// Stop accepting new jobs
		e.state.Store(int32(TTSStateStopped))

		// Cancel context to stop workers
		e.cancelFn()

		// Drain remaining jobs
		close(e.audioQueue)
		for job := range e.audioQueue {
			job.cancelFn()
		}

		// Wait for workers to finish
		e.wg.Wait()

		log.Info().Msg("TTS engine stopped")
	})
}

// processQueue is the background worker that processes audio jobs.
func (e *TTSEngine) processQueue() {
	defer e.wg.Done()

	for {
		select {
		case <-e.ctx.Done():
			return

		case job, ok := <-e.audioQueue:
			if !ok {
				return
			}

			// Skip if interrupted
			if e.interrupted.Load() {
				job.cancelFn()
				job.errCh <- fmt.Errorf("skipped due to interrupt")
				continue
			}

			// Process the job
			e.processJob(job)
		}
	}
}

// processJob handles synthesis and playback for a single job.
func (e *TTSEngine) processJob(job *audioJob) {
	start := time.Now()

	// Synthesize if not already processed
	if !job.processed {
		audio, err := e.synthesize(job.ctx, job.text, job.voiceID, job.speed)
		if err != nil {
			e.synthesisErrors.Add(1)
			job.errCh <- err
			job.cancelFn()
			return
		}
		job.audio = audio
		job.processed = true
	}

	e.synthesisCount.Add(1)
	e.totalLatencyMs.Add(time.Since(start).Milliseconds())

	// Check for interrupt before playback
	if e.interrupted.Load() {
		job.errCh <- fmt.Errorf("interrupted before playback")
		job.cancelFn()
		return
	}

	// Play audio
	e.state.Store(int32(TTSStatePlaying))
	err := e.playAudio(job.ctx, job.audio)
	e.state.Store(int32(TTSStateIdle))

	if err != nil {
		job.errCh <- err
	} else {
		e.playbackCount.Add(1)
		job.errCh <- nil
	}

	job.cancelFn()
}

// synthesize calls the TTS API to convert text to audio.
// CR-012: Auto-starts Voice Box if not running.
func (e *TTSEngine) synthesize(ctx context.Context, text, voiceID string, speed float64) ([]byte, error) {
	start := time.Now()

	// CR-012: Ensure Voice Box is running (lazy start)
	if e.launcher != nil {
		if err := e.launcher.EnsureRunning(ctx); err != nil {
			log.Warn().Err(err).Msg("Voice Box unavailable, synthesis may fail")
			// Continue anyway - might be using a different TTS backend
		}
	}

	// Build request body (OpenAI-compatible format)
	reqBody := ttsRequest{
		Model:          e.config.Model,
		Input:          text,
		Voice:          voiceID,
		ResponseFormat: e.config.ResponseFormat,
		Speed:          speed,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.config.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("request cancelled: %w", ctx.Err())
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("TTS API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio data: %w", err)
	}

	latency := time.Since(start)
	log.Debug().
		Str("text", truncateText(text, 30)).
		Str("voice", voiceID).
		Int("audio_bytes", len(audio)).
		Dur("latency", latency).
		Msg("TTS synthesis complete")

	return audio, nil
}

// playAudio plays the synthesized audio.
func (e *TTSEngine) playAudio(ctx context.Context, audio []byte) error {
	if e.playbackFn == nil {
		// No playback function set - just log
		log.Debug().
			Int("audio_bytes", len(audio)).
			Msg("audio playback skipped (no playback function)")
		return nil
	}

	// Check for interrupt
	select {
	case <-e.interruptCh:
		return fmt.Errorf("playback interrupted")
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	return e.playbackFn(audio, AudioFormat(e.config.ResponseFormat))
}

// ttsRequest represents the OpenAI-compatible TTS API request format.
type ttsRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	ResponseFormat string  `json:"response_format,omitempty"`
	Speed          float64 `json:"speed,omitempty"`
}

// truncateText truncates text to maxLen characters with ellipsis.
func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen-3] + "..."
}

// ══════════════════════════════════════════════════════════════════════════════
// Convenience functions for quick integration
// ══════════════════════════════════════════════════════════════════════════════

// NewTTSEngineWithPersona creates a TTS engine configured for a specific persona.
func NewTTSEngineWithPersona(personaName string, endpoint string) *TTSEngine {
	persona := GetPersona(personaName)

	config := DefaultTTSConfig()
	config.VoiceID = persona.VoiceID
	config.Speed = persona.Speed
	if endpoint != "" {
		config.Endpoint = endpoint
	}

	return NewTTSEngine(config)
}

// SpeakWithPersona creates a one-shot synthesis using a persona.
// Returns audio data without playback.
func SpeakWithPersona(ctx context.Context, text, personaName, endpoint string) ([]byte, error) {
	persona := GetPersona(personaName)

	config := DefaultTTSConfig()
	config.VoiceID = persona.VoiceID
	config.Speed = persona.Speed
	if endpoint != "" {
		config.Endpoint = endpoint
	}

	engine := NewTTSEngine(config)
	defer engine.Stop()

	return engine.synthesize(ctx, text, persona.VoiceID, persona.Speed)
}
