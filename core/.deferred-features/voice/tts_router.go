package voice

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// RouterConfig contains configuration for the voice router.
type RouterConfig struct {
	// Enabled controls whether voice synthesis is enabled
	Enabled bool `json:"enabled"`

	// LongTextThreshold is the character count above which Smart Lane is recommended
	LongTextThreshold int `json:"long_text_threshold"`

	// FastLaneDefaultVoice is the default voice ID for Fast Lane (Kokoro)
	FastLaneDefaultVoice string `json:"fast_lane_default_voice"`

	// SmartLaneDefaultVoice is the default voice ID for Smart Lane (XTTS)
	SmartLaneDefaultVoice string `json:"smart_lane_default_voice"`

	// EnableVoiceCloning enables voice cloning features (requires Smart Lane)
	EnableVoiceCloning bool `json:"enable_voice_cloning"`

	// EnableCache enables audio caching
	EnableCache bool `json:"enable_cache"`

	// SmartLaneFormat is the audio format for Smart Lane (default: Opus for compression)
	SmartLaneFormat AudioFormat `json:"smart_lane_format"`

	// FastLaneFormat is the audio format for Fast Lane (default: WAV for speed)
	FastLaneFormat AudioFormat `json:"fast_lane_format"`

	// PrewarmPhrases are phrases to cache on startup
	PrewarmPhrases []string `json:"prewarm_phrases,omitempty"`
}

// DefaultRouterConfig returns sensible defaults for the router.
func DefaultRouterConfig() RouterConfig {
	return RouterConfig{
		Enabled:               true,
		LongTextThreshold:     500,
		FastLaneDefaultVoice:  "af_sky",
		SmartLaneDefaultVoice: "default",
		EnableVoiceCloning:    false,
		EnableCache:           true,
		SmartLaneFormat:       FormatOpus,
		FastLaneFormat:        FormatWAV,
		PrewarmPhrases:        nil, // Use CommonPhrases from cache.go
	}
}

// SpeakRequest represents a high-level voice synthesis request.
type SpeakRequest struct {
	// Text is the content to synthesize
	Text string `json:"text"`

	// Lane specifies which provider lane to use ("fast" or "smart")
	Lane string `json:"lane"`

	// PersonaVoiceID is the voice ID to use (overrides lane defaults)
	PersonaVoiceID string `json:"persona_voice_id,omitempty"`

	// Mode is the synthesis mode (optional, provider-specific)
	Mode string `json:"mode,omitempty"`

	// UserVoicePrefs contains user preferences for speed, pitch, etc.
	UserVoicePrefs *VoicePreferences `json:"user_voice_prefs,omitempty"`

	// PreferredFormat is the desired output format (overrides lane defaults)
	PreferredFormat AudioFormat `json:"preferred_format,omitempty"`
}

// VoicePreferences contains user-specific voice settings.
type VoicePreferences struct {
	Speed float64 `json:"speed,omitempty"` // Playback speed multiplier (0.5-2.0)
	Pitch float64 `json:"pitch,omitempty"` // Pitch adjustment (-1.0 to 1.0)
}

// SpeakResponse represents a voice synthesis response with metadata.
type SpeakResponse struct {
	// Embedded synthesis response
	SynthesizeResponse

	// Provider is the name of the provider that handled the request
	Provider string `json:"provider"`

	// UsedFallback indicates if fallback to Fast Lane was used
	UsedFallback bool `json:"used_fallback"`

	// CacheHit indicates if the response was served from cache
	CacheHit bool `json:"cache_hit"`
}

// HealthStatus represents the health status of a provider.
type HealthStatus struct {
	Available bool      `json:"available"`
	Error     string    `json:"error,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
}

// Router selects the appropriate voice provider based on lane and context.
// It provides automatic fallback when XTTS is unavailable.
type Router struct {
	fastProvider  Provider // Kokoro (always available)
	smartProvider Provider // XTTS (may be nil if GPU unavailable)
	cloudProvider Provider // Resemble.ai (may be nil if no API key)

	cache    *AudioCache
	encoder  *AudioEncoder
	gpuCheck *GPUChecker
	config   RouterConfig

	// Fallback state
	smartAvailable      atomic.Bool
	cloudAvailable      atomic.Bool
	lastHealthCheck     time.Time
	healthCheckInterval time.Duration

	// Active provider preference (can be "local" or "cloud")
	activeProvider string

	mu sync.RWMutex
}

// NewRouter creates a new voice router with the given providers and configuration.
// The smart provider may be nil if GPU is unavailable.
func NewRouter(fast, smart Provider, config RouterConfig) *Router {
	r := &Router{
		fastProvider:        fast,
		smartProvider:       smart,
		config:              config,
		healthCheckInterval: 30 * time.Second,
		activeProvider:      "local", // Default to local providers
	}

	// Initialize cache if enabled
	if config.EnableCache {
		r.cache = NewAudioCache()
	}

	// Initialize encoder (optional, for format conversion)
	if encoder, err := NewAudioEncoder(); err == nil {
		r.encoder = encoder
		log.Debug().Msg("audio encoder initialized")
	} else {
		log.Warn().Err(err).Msg("audio encoder unavailable, format conversion disabled")
	}

	// Initialize GPU checker
	r.gpuCheck = NewGPUChecker()

	// Set initial smart availability
	if smart != nil {
		r.smartAvailable.Store(true)
	} else {
		r.smartAvailable.Store(false)
	}

	return r
}

// SetCloudProvider sets the cloud TTS provider (Resemble.ai).
func (r *Router) SetCloudProvider(cloud Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cloudProvider = cloud
	if cloud != nil {
		r.cloudAvailable.Store(true)
	} else {
		r.cloudAvailable.Store(false)
	}
}

// SetActiveProvider sets the preferred provider type ("local" or "cloud").
func (r *Router) SetActiveProvider(provider string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if provider == "local" || provider == "cloud" {
		r.activeProvider = provider
	}
}

// GetActiveProvider returns the current active provider type.
func (r *Router) GetActiveProvider() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.activeProvider
}

// HasCloudProvider returns true if a cloud provider is configured.
func (r *Router) HasCloudProvider() bool {
	return r.cloudAvailable.Load()
}

// GetCloudProvider returns the cloud provider if available.
func (r *Router) GetCloudProvider() Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cloudProvider
}

// Speak synthesizes audio with automatic fallback and caching.
func (r *Router) Speak(ctx context.Context, req *SpeakRequest) (*SpeakResponse, error) {
	if !r.config.Enabled {
		return nil, fmt.Errorf("voice synthesis is disabled")
	}

	// Check cache first
	if r.cache != nil && req.PersonaVoiceID != "" {
		speed := 1.0
		if req.UserVoicePrefs != nil && req.UserVoicePrefs.Speed != 0 {
			speed = req.UserVoicePrefs.Speed
		}

		cacheKey := CacheKey(req.Text, req.PersonaVoiceID, speed)
		if audio, format, found := r.cache.Get(cacheKey); found {
			log.Debug().
				Str("key", cacheKey[:16]).
				Int("size", len(audio)).
				Msg("cache hit for speak request")

			return &SpeakResponse{
				SynthesizeResponse: SynthesizeResponse{
					Audio:       audio,
					Format:      format,
					VoiceID:     req.PersonaVoiceID,
					Provider:    "cache",
					ProcessedMs: 0,
				},
				Provider: "cache",
				CacheHit: true,
			}, nil
		}
	}

	// Select provider with fallback
	provider, usedFallback, err := r.selectProviderWithFallback(ctx, req.Lane)
	if err != nil {
		return nil, err
	}

	// Build synthesis request
	synthReq := r.buildSynthesizeRequest(req, provider)

	// Attempt synthesis
	synthResp, err := r.attemptSynthesis(ctx, provider, synthReq)
	if err != nil {
		// If smart provider failed and we haven't tried fallback yet, try fast provider
		if !usedFallback && r.fastProvider != nil && provider.Name() != r.fastProvider.Name() {
			log.Warn().
				Err(err).
				Str("provider", provider.Name()).
				Msg("smart provider failed, falling back to fast provider")

			provider = r.fastProvider
			usedFallback = true
			synthReq = r.buildSynthesizeRequest(req, provider)
			synthResp, err = r.attemptSynthesis(ctx, provider, synthReq)
			if err != nil {
				return nil, fmt.Errorf("both providers failed: %w", err)
			}
		} else {
			return nil, err
		}
	}

	// Transcode if needed
	synthResp, err = r.maybeTranscode(synthResp, req)
	if err != nil {
		log.Warn().Err(err).Msg("transcoding failed, returning original format")
	}

	// Cache the result
	if r.cache != nil && req.PersonaVoiceID != "" {
		speed := 1.0
		if req.UserVoicePrefs != nil && req.UserVoicePrefs.Speed != 0 {
			speed = req.UserVoicePrefs.Speed
		}
		cacheKey := CacheKey(req.Text, req.PersonaVoiceID, speed)
		r.cache.Set(cacheKey, synthResp.Audio, synthResp.Format)
	}

	// Build response
	return &SpeakResponse{
		SynthesizeResponse: *synthResp,
		Provider:           provider.Name(),
		UsedFallback:       usedFallback,
		CacheHit:           false,
	}, nil
}

// SpeakStream synthesizes audio as a stream with automatic fallback.
func (r *Router) SpeakStream(ctx context.Context, req *SpeakRequest) (AudioStream, error) {
	if !r.config.Enabled {
		return nil, fmt.Errorf("voice synthesis is disabled")
	}

	// Select provider with fallback
	provider, usedFallback, err := r.selectProviderWithFallback(ctx, req.Lane)
	if err != nil {
		return nil, err
	}

	// Build synthesis request
	synthReq := r.buildSynthesizeRequest(req, provider)

	// Attempt streaming synthesis
	stream, err := provider.Stream(ctx, synthReq)
	if err != nil {
		// If smart provider failed and we haven't tried fallback yet, try fast provider
		if !usedFallback && r.fastProvider != nil && provider.Name() != r.fastProvider.Name() {
			log.Warn().
				Err(err).
				Str("provider", provider.Name()).
				Msg("smart provider stream failed, falling back to fast provider")

			provider = r.fastProvider
			synthReq = r.buildSynthesizeRequest(req, provider)
			stream, err = provider.Stream(ctx, synthReq)
			if err != nil {
				return nil, fmt.Errorf("both providers failed for streaming: %w", err)
			}
		} else {
			return nil, err
		}
	}

	return stream, nil
}

// Health checks the health of all registered providers.
func (r *Router) Health(ctx context.Context) map[string]HealthStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results := make(map[string]HealthStatus)

	// Check fast provider
	if r.fastProvider != nil {
		err := r.fastProvider.Health(ctx)
		results[r.fastProvider.Name()] = HealthStatus{
			Available: err == nil,
			Error:     errToString(err),
			CheckedAt: time.Now(),
		}
	}

	// Check smart provider
	if r.smartProvider != nil {
		err := r.smartProvider.Health(ctx)
		available := err == nil
		results[r.smartProvider.Name()] = HealthStatus{
			Available: available,
			Error:     errToString(err),
			CheckedAt: time.Now(),
		}

		// Update cached availability
		r.smartAvailable.Store(available)
		r.mu.Lock()
		r.lastHealthCheck = time.Now()
		r.mu.Unlock()
	}

	// Check cloud provider
	if r.cloudProvider != nil {
		err := r.cloudProvider.Health(ctx)
		available := err == nil
		results[r.cloudProvider.Name()] = HealthStatus{
			Available: available,
			Error:     errToString(err),
			CheckedAt: time.Now(),
		}

		// Update cached availability
		r.cloudAvailable.Store(available)
	}

	return results
}

// GetProviderForLane returns the provider for the specified lane.
func (r *Router) GetProviderForLane(lane string) Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	switch lane {
	case "cloud":
		if r.cloudProvider != nil && r.cloudAvailable.Load() {
			return r.cloudProvider
		}
		// Fallback to fast provider
		return r.fastProvider
	case "fast":
		return r.fastProvider
	case "smart":
		if r.smartProvider != nil && r.isSmartAvailable() {
			return r.smartProvider
		}
		// Fallback to fast provider
		return r.fastProvider
	default:
		return r.fastProvider
	}
}

// Prewarm preloads common phrases into the cache.
func (r *Router) Prewarm(ctx context.Context) error {
	if r.cache == nil {
		return fmt.Errorf("cache is not enabled")
	}

	// Use Fast Lane provider for prewarming (always available, fast)
	if r.fastProvider == nil {
		return fmt.Errorf("fast provider is not available")
	}

	phrases := r.config.PrewarmPhrases
	if len(phrases) == 0 {
		phrases = CommonPhrases
	}

	voiceID := r.config.FastLaneDefaultVoice

	log.Info().
		Int("phrases", len(phrases)).
		Str("voice_id", voiceID).
		Msg("prewarming voice cache")

	// Create adapter for VoiceProvider interface
	adapter := &prewarmAdapter{provider: r.fastProvider}

	return r.cache.Prewarm(ctx, adapter, voiceID, phrases)
}

// selectProviderWithFallback selects the appropriate provider with automatic fallback.
func (r *Router) selectProviderWithFallback(ctx context.Context, lane string) (Provider, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	usedFallback := false

	// Check if cloud provider is preferred and available
	if r.activeProvider == "cloud" && r.cloudProvider != nil && r.cloudAvailable.Load() {
		return r.cloudProvider, usedFallback, nil
	}

	switch lane {
	case "cloud":
		// Explicitly request cloud provider
		log.Debug().
			Bool("cloudProvider_exists", r.cloudProvider != nil).
			Bool("cloudAvailable", r.cloudAvailable.Load()).
			Msg("checking cloud provider availability")
		if r.cloudProvider != nil && r.cloudAvailable.Load() {
			log.Info().Str("provider", r.cloudProvider.Name()).Msg("using cloud provider")
			return r.cloudProvider, usedFallback, nil
		}
		// Fall back to fast provider
		log.Warn().
			Bool("cloudProvider_exists", r.cloudProvider != nil).
			Bool("cloudAvailable", r.cloudAvailable.Load()).
			Msg("cloud lane requested but unavailable, falling back to fast lane")
		usedFallback = true
		fallthrough

	case "smart":
		// Try smart provider if available
		if r.smartProvider != nil && r.isSmartAvailable() {
			return r.smartProvider, usedFallback, nil
		}

		// Fall back to fast provider
		log.Warn().Msg("smart lane requested but unavailable, falling back to fast lane")
		usedFallback = true
		fallthrough

	case "fast":
		if r.fastProvider != nil {
			return r.fastProvider, usedFallback, nil
		}
		return nil, usedFallback, ErrProviderUnavailable

	default:
		// Unknown lane, default to fast
		log.Warn().Str("lane", lane).Msg("unknown lane, defaulting to fast")
		if r.fastProvider != nil {
			return r.fastProvider, usedFallback, nil
		}
		return nil, usedFallback, ErrProviderUnavailable
	}
}

// isSmartAvailable checks if the smart provider is available with cached health check.
func (r *Router) isSmartAvailable() bool {
	// Check cached health status first
	if time.Since(r.lastHealthCheck) < r.healthCheckInterval {
		return r.smartAvailable.Load()
	}

	// Perform fresh health check asynchronously to avoid blocking
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := r.smartProvider.Health(ctx)
		available := err == nil

		r.smartAvailable.Store(available)
		r.mu.Lock()
		r.lastHealthCheck = time.Now()
		r.mu.Unlock()

		log.Debug().
			Bool("available", available).
			Err(err).
			Msg("smart provider health check completed")
	}()

	// Return cached value while async check runs
	return r.smartAvailable.Load()
}

// buildSynthesizeRequest converts a SpeakRequest to a SynthesizeRequest.
func (r *Router) buildSynthesizeRequest(req *SpeakRequest, provider Provider) *SynthesizeRequest {
	synthReq := &SynthesizeRequest{
		Text:   req.Text,
		Format: r.getOutputFormat(req, provider),
	}

	// Determine voice ID
	if req.PersonaVoiceID != "" {
		synthReq.VoiceID = req.PersonaVoiceID
	} else {
		// Use lane-specific default
		if provider == r.smartProvider {
			synthReq.VoiceID = r.config.SmartLaneDefaultVoice
		} else {
			synthReq.VoiceID = r.config.FastLaneDefaultVoice
		}
	}

	// Apply user preferences
	if req.UserVoicePrefs != nil {
		synthReq.Speed = req.UserVoicePrefs.Speed
		synthReq.Pitch = req.UserVoicePrefs.Pitch
	}

	return synthReq
}

// getOutputFormat determines the output format based on request and provider.
func (r *Router) getOutputFormat(req *SpeakRequest, provider Provider) AudioFormat {
	// Use preferred format if specified
	if req.PreferredFormat != "" {
		return req.PreferredFormat
	}

	// Use lane-specific format
	if provider == r.smartProvider {
		return r.config.SmartLaneFormat
	}

	return r.config.FastLaneFormat
}

// attemptSynthesis attempts synthesis with the given provider.
func (r *Router) attemptSynthesis(ctx context.Context, provider Provider, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	start := time.Now()

	resp, err := provider.Synthesize(ctx, req)
	if err != nil {
		return nil, err
	}

	duration := time.Since(start)
	log.Debug().
		Str("provider", provider.Name()).
		Int("text_len", len(req.Text)).
		Int("audio_size", len(resp.Audio)).
		Dur("duration", duration).
		Msg("synthesis completed")

	return resp, nil
}

// maybeTranscode transcodes audio if encoder is available and format differs from preferred.
func (r *Router) maybeTranscode(resp *SynthesizeResponse, req *SpeakRequest) (*SynthesizeResponse, error) {
	if r.encoder == nil {
		return resp, nil
	}

	// Determine target format
	targetFormat := req.PreferredFormat
	if targetFormat == "" {
		// No transcoding needed
		return resp, nil
	}

	// Skip if already in target format
	if resp.Format == targetFormat {
		return resp, nil
	}

	// Only transcode from WAV for now
	if resp.Format != FormatWAV {
		return resp, nil
	}

	var transcoded []byte
	var err error

	switch targetFormat {
	case FormatOpus:
		transcoded, err = r.encoder.EncodeToOpus(resp.Audio)
	case FormatMP3:
		transcoded, err = r.encoder.EncodeToMP3(resp.Audio)
	default:
		// Unsupported target format
		return resp, nil
	}

	if err != nil {
		return resp, fmt.Errorf("transcoding failed: %w", err)
	}

	log.Debug().
		Str("from", string(resp.Format)).
		Str("to", string(targetFormat)).
		Int("original_size", len(resp.Audio)).
		Int("transcoded_size", len(transcoded)).
		Msg("audio transcoded")

	// Update response with transcoded audio
	resp.Audio = transcoded
	resp.Format = targetFormat

	return resp, nil
}

// prewarmAdapter adapts Provider to VoiceProvider interface for cache prewarming.
type prewarmAdapter struct {
	provider Provider
}

func (a *prewarmAdapter) Synthesize(ctx context.Context, text, voiceID string, speed float64) ([]byte, AudioFormat, error) {
	req := &SynthesizeRequest{
		Text:    text,
		VoiceID: voiceID,
		Speed:   speed,
		Format:  FormatWAV, // Use WAV for cache
	}

	resp, err := a.provider.Synthesize(ctx, req)
	if err != nil {
		return nil, "", err
	}

	return resp.Audio, resp.Format, nil
}

// errToString converts an error to a string, returning empty string for nil.
func errToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
