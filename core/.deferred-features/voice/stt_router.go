// Package voice provides voice processing capabilities for Cortex.
// stt_router.go routes STT requests to the best available backend.
package voice

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// STTRouter routes transcription requests to available STT backends.
// Brain Alignment: Like the auditory cortex routing signals to appropriate
// processing regions, this router selects optimal backends based on
// capabilities and requirements.
type STTRouter struct {
	mu       sync.RWMutex
	backends map[string]STTBackend
	config   STTRouterConfig

	// Cached availability status
	availabilityCache map[string]bool
	lastCheck         time.Time
	checkInterval     time.Duration
}

// STTRouterConfig configures the STT router behavior.
type STTRouterConfig struct {
	// PreferredBackend specifies which backend to prefer ("auto", "sensevoice", "whisper")
	PreferredBackend string `yaml:"preferred_backend" json:"preferred_backend"`

	// PreferEmotionCapable prioritizes backends with emotion detection
	PreferEmotionCapable bool `yaml:"prefer_emotion" json:"prefer_emotion"`

	// FallbackEnabled allows falling back to other backends if preferred is unavailable
	FallbackEnabled bool `yaml:"fallback_enabled" json:"fallback_enabled"`

	// AvailabilityCheckInterval determines how often to check backend availability
	AvailabilityCheckInterval time.Duration `yaml:"availability_check_interval" json:"availability_check_interval"`
}

// DefaultSTTRouterConfig returns sensible defaults.
func DefaultSTTRouterConfig() STTRouterConfig {
	return STTRouterConfig{
		PreferredBackend:          "auto",
		PreferEmotionCapable:      true,
		FallbackEnabled:           true,
		AvailabilityCheckInterval: 30 * time.Second,
	}
}

// NewSTTRouter creates a new STT router with the given backends.
func NewSTTRouter(config STTRouterConfig, backends ...STTBackend) *STTRouter {
	router := &STTRouter{
		backends:          make(map[string]STTBackend),
		config:            config,
		availabilityCache: make(map[string]bool),
		checkInterval:     config.AvailabilityCheckInterval,
	}

	for _, backend := range backends {
		router.backends[backend.Name()] = backend
	}

	return router
}

// AddBackend registers a new STT backend.
func (r *STTRouter) AddBackend(backend STTBackend) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.backends[backend.Name()] = backend
}

// RemoveBackend unregisters an STT backend.
func (r *STTRouter) RemoveBackend(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.backends, name)
}

// Transcribe routes the request to the best available backend.
func (r *STTRouter) Transcribe(ctx context.Context, req *STTRequest) (*STTResult, error) {
	backend, err := r.selectBackend(ctx, req)
	if err != nil {
		return nil, err
	}

	log.Debug().
		Str("backend", backend.Name()).
		Bool("emotion_requested", req.IncludeEmotion).
		Msg("routing STT request")

	return backend.Transcribe(ctx, req)
}

// selectBackend chooses the best backend for the request.
func (r *STTRouter) selectBackend(ctx context.Context, req *STTRequest) (STTBackend, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.backends) == 0 {
		return nil, fmt.Errorf("no STT backends registered")
	}

	// Refresh availability cache if needed
	r.refreshAvailability(ctx)

	// If a specific backend is requested and available, use it
	if r.config.PreferredBackend != "" && r.config.PreferredBackend != "auto" {
		if backend, exists := r.backends[r.config.PreferredBackend]; exists {
			if r.isAvailable(r.config.PreferredBackend) {
				return backend, nil
			}
			if !r.config.FallbackEnabled {
				return nil, fmt.Errorf("preferred backend %s is not available", r.config.PreferredBackend)
			}
		}
	}

	// Build list of available backends
	var candidates []STTBackend
	for name, backend := range r.backends {
		if r.isAvailable(name) {
			candidates = append(candidates, backend)
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no STT backends available")
	}

	// Sort candidates by priority
	sort.Slice(candidates, func(i, j int) bool {
		return r.backendScore(candidates[i], req) > r.backendScore(candidates[j], req)
	})

	return candidates[0], nil
}

// backendScore calculates a priority score for a backend given the request.
func (r *STTRouter) backendScore(backend STTBackend, req *STTRequest) int {
	score := 0

	// Emotion capability bonus
	if req.IncludeEmotion && backend.SupportsEmotion() {
		score += 100
	} else if r.config.PreferEmotionCapable && backend.SupportsEmotion() {
		score += 50
	}

	// Streaming capability (if ever implemented)
	if backend.SupportsStreaming() {
		score += 10
	}

	// SenseVoice gets a speed bonus (15x faster than Whisper)
	if backend.Name() == "sensevoice" {
		score += 30
	}

	return score
}

// isAvailable checks cached availability for a backend.
func (r *STTRouter) isAvailable(name string) bool {
	if avail, exists := r.availabilityCache[name]; exists {
		return avail
	}
	return false
}

// refreshAvailability updates the availability cache if stale.
func (r *STTRouter) refreshAvailability(ctx context.Context) {
	if time.Since(r.lastCheck) < r.checkInterval {
		return
	}

	// Check in background to avoid blocking
	go func() {
		r.mu.Lock()
		defer r.mu.Unlock()

		checkCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		for name, backend := range r.backends {
			r.availabilityCache[name] = backend.IsAvailable(checkCtx)
		}
		r.lastCheck = time.Now()
	}()
}

// ForceRefreshAvailability immediately checks all backends.
func (r *STTRouter) ForceRefreshAvailability(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for name, backend := range r.backends {
		r.availabilityCache[name] = backend.IsAvailable(ctx)
	}
	r.lastCheck = time.Now()
}

// GetAvailableBackends returns names of currently available backends.
func (r *STTRouter) GetAvailableBackends() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var available []string
	for name := range r.backends {
		if r.isAvailable(name) {
			available = append(available, name)
		}
	}
	return available
}

// GetBackend returns a specific backend by name.
func (r *STTRouter) GetBackend(name string) (STTBackend, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	backend, exists := r.backends[name]
	return backend, exists
}

// ─────────────────────────────────────────────────────────────────────────────
// Whisper Backend Adapter (for VoiceBox)
// ─────────────────────────────────────────────────────────────────────────────

// WhisperBackend adapts VoiceBoxLauncher as an STT backend.
type WhisperBackend struct {
	launcher *VoiceBoxLauncher
}

// NewWhisperBackend creates a Whisper backend from VoiceBox.
func NewWhisperBackend(launcher *VoiceBoxLauncher) *WhisperBackend {
	return &WhisperBackend{launcher: launcher}
}

// Name returns "whisper".
func (w *WhisperBackend) Name() string {
	return "whisper"
}

// IsAvailable checks if VoiceBox is running.
func (w *WhisperBackend) IsAvailable(ctx context.Context) bool {
	return w.launcher.IsHealthy()
}

// Transcribe delegates to VoiceBox.
// Note: This is a minimal implementation - expand as needed.
func (w *WhisperBackend) Transcribe(ctx context.Context, req *STTRequest) (*STTResult, error) {
	// TODO: Implement VoiceBox transcription call
	// For now, return an error indicating this needs implementation
	return nil, fmt.Errorf("whisper transcription not yet implemented via VoiceBox")
}

// SupportsEmotion returns false - Whisper doesn't provide emotion detection.
func (w *WhisperBackend) SupportsEmotion() bool {
	return false
}

// SupportsStreaming returns false - VoiceBox Whisper doesn't stream.
func (w *WhisperBackend) SupportsStreaming() bool {
	return false
}

// ─────────────────────────────────────────────────────────────────────────────
// Default Router Factory
// ─────────────────────────────────────────────────────────────────────────────

// DefaultSTTRouter creates a router with default backends.
func DefaultSTTRouter() *STTRouter {
	config := DefaultSTTRouterConfig()

	// Create SenseVoice client
	senseVoice := NewSenseVoiceClient(DefaultSTTBackendConfig("sensevoice"))

	// Create Whisper backend from VoiceBox
	whisper := NewWhisperBackend(GetVoiceBoxLauncher())

	return NewSTTRouter(config, senseVoice, whisper)
}

// ─────────────────────────────────────────────────────────────────────────────
// Global Router Singleton
// ─────────────────────────────────────────────────────────────────────────────

var (
	globalSTTRouter     *STTRouter
	globalSTTRouterOnce sync.Once
)

// GetSTTRouter returns the global STT router instance.
func GetSTTRouter() *STTRouter {
	globalSTTRouterOnce.Do(func() {
		globalSTTRouter = DefaultSTTRouter()
	})
	return globalSTTRouter
}
