// Package chatterbox implements the voice.Provider interface for Chatterbox TTS.
// CR-004-A: Replaces XTTS v2 with MIT-licensed Chatterbox from Resemble AI.
//
// Key differences from XTTS:
// - License: MIT (XTTS was CPML with commercial restrictions)
// - API: OpenAI-compatible (field is "input" not "text")
// - Voice cloning: Upload once to library, reference by name forever (vs WAV per request)
// - Emotion control: Exaggeration, CFG weight, temperature parameters
// - Port: 4123 (XTTS was 5002)
package chatterbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/normanking/cortex/internal/voice"
)

// Provider implements voice.Provider for Chatterbox TTS.
type Provider struct {
	config     Config
	httpClient *http.Client
	healthy    atomic.Bool
}

// Config contains Chatterbox-specific configuration.
type Config struct {
	BaseURL       string        // Default: http://localhost:4123
	DefaultVoice  string        // Default: "default"
	Timeout       time.Duration // Default: 30s
	MaxTextLength int           // Default: 5000

	// Emotion/Style Controls (unique to Chatterbox)
	Exaggeration float64 // 0.25-2.0: emotion intensity (default: 0.5)
	CFGWeight    float64 // 0.0-1.0: pace control (default: 0.5)
	Temperature  float64 // 0.05-5.0: sampling temperature (default: 0.8)
}

// DefaultConfig returns sensible defaults for Chatterbox.
func DefaultConfig() Config {
	return Config{
		BaseURL:       "http://localhost:4123",
		DefaultVoice:  "default",
		Timeout:       30 * time.Second,
		MaxTextLength: 5000,
		Exaggeration:  0.5, // Natural speech
		CFGWeight:     0.5, // Balanced pace
		Temperature:   0.8, // Moderate creativity
	}
}

// NewProvider creates a new Chatterbox provider.
func NewProvider(config Config) *Provider {
	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:4123"
	}
	if config.DefaultVoice == "" {
		config.DefaultVoice = "default"
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxTextLength == 0 {
		config.MaxTextLength = 5000
	}
	if config.Exaggeration == 0 {
		config.Exaggeration = 0.5
	}
	if config.CFGWeight == 0 {
		config.CFGWeight = 0.5
	}
	if config.Temperature == 0 {
		config.Temperature = 0.8
	}

	return &Provider{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// NewProviderWithDefaults creates a provider with default configuration.
func NewProviderWithDefaults() *Provider {
	return NewProvider(DefaultConfig())
}

// Name returns the provider identifier.
func (p *Provider) Name() string {
	return "chatterbox"
}

// ═══════════════════════════════════════════════════════════════════════════
// API TYPES (OpenAI-compatible)
// ═══════════════════════════════════════════════════════════════════════════

// SpeechRequest matches the OpenAI-compatible /v1/audio/speech endpoint.
// IMPORTANT: Field is "input" NOT "text" (OpenAI compatibility).
type SpeechRequest struct {
	Model string `json:"model"`           // "chatterbox-tts-1" or "tts-1"
	Input string `json:"input"`           // Text to speak (NOT "text"!)
	Voice string `json:"voice"`           // Voice name from library or "default"

	// Optional parameters
	Speed        float64 `json:"speed,omitempty"`        // Speech speed multiplier
	Exaggeration float64 `json:"exaggeration,omitempty"` // Emotion intensity (0.25-2.0)
	CFGWeight    float64 `json:"cfg_weight,omitempty"`   // Pace control (0.0-1.0)
	Temperature  float64 `json:"temperature,omitempty"`  // Sampling temperature (0.05-5.0)
}

// ErrorResponse from Chatterbox API.
type ErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// HealthResponse from GET /health.
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
}

// ═══════════════════════════════════════════════════════════════════════════
// CORE SYNTHESIS METHODS
// ═══════════════════════════════════════════════════════════════════════════

// Synthesize sends a synthesis request to Chatterbox.
// This implements the voice.Provider interface.
func (p *Provider) Synthesize(ctx context.Context, req *voice.SynthesizeRequest) (*voice.SynthesizeResponse, error) {
	start := time.Now()

	if err := voice.ValidateRequest(req); err != nil {
		return nil, err
	}

	if len(req.Text) > p.config.MaxTextLength {
		return nil, voice.ErrTextTooLong
	}

	// Build Chatterbox request
	voiceID := req.VoiceID
	if voiceID == "" {
		voiceID = p.config.DefaultVoice
	}

	speed := req.Speed
	if speed == 0 {
		speed = 1.0
	}

	speechReq := SpeechRequest{
		Model:        "chatterbox-tts-1",
		Input:        req.Text, // Note: "input" not "text" for OpenAI compat
		Voice:        voiceID,
		Speed:        speed,
		Exaggeration: p.config.Exaggeration,
		CFGWeight:    p.config.CFGWeight,
		Temperature:  p.config.Temperature,
	}

	audioData, err := p.doSynthesis(ctx, &speechReq)
	if err != nil {
		return nil, err
	}

	format := voice.FormatWAV
	if req.Format != "" {
		format = req.Format
	}

	sampleRate := 24000 // Chatterbox default
	if req.SampleRate != 0 {
		sampleRate = req.SampleRate
	}

	return &voice.SynthesizeResponse{
		Audio:       audioData,
		Format:      format,
		Duration:    0, // Calculate from audio data if needed
		SampleRate:  sampleRate,
		ProcessedMs: time.Since(start).Milliseconds(),
		VoiceID:     voiceID,
		Provider:    p.Name(),
	}, nil
}

// SynthesizeWithEmotion synthesizes speech with custom emotion settings.
// This is a Chatterbox-specific method that allows fine-grained control.
func (p *Provider) SynthesizeWithEmotion(ctx context.Context, text, voiceID string, exaggeration, cfgWeight float64) ([]byte, error) {
	if voiceID == "" {
		voiceID = p.config.DefaultVoice
	}

	// Clamp values to valid ranges
	if exaggeration < 0.25 {
		exaggeration = 0.25
	} else if exaggeration > 2.0 {
		exaggeration = 2.0
	}

	if cfgWeight < 0.0 {
		cfgWeight = 0.0
	} else if cfgWeight > 1.0 {
		cfgWeight = 1.0
	}

	req := SpeechRequest{
		Model:        "chatterbox-tts-1",
		Input:        text,
		Voice:        voiceID,
		Speed:        1.0,
		Exaggeration: exaggeration,
		CFGWeight:    cfgWeight,
		Temperature:  p.config.Temperature,
	}

	return p.doSynthesis(ctx, &req)
}

// doSynthesis is the internal method that makes the HTTP call.
func (p *Provider) doSynthesis(ctx context.Context, req *SpeechRequest) ([]byte, error) {
	// Set defaults if not specified
	if req.Model == "" {
		req.Model = "chatterbox-tts-1"
	}
	if req.Exaggeration == 0 {
		req.Exaggeration = p.config.Exaggeration
	}
	if req.CFGWeight == 0 {
		req.CFGWeight = p.config.CFGWeight
	}
	if req.Temperature == 0 {
		req.Temperature = p.config.Temperature
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		p.config.BaseURL+"/v1/audio/speech", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		p.healthy.Store(false)
		if ctx.Err() != nil {
			return nil, voice.ErrTimeout
		}
		return nil, fmt.Errorf("chatterbox request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		p.healthy.Store(false)

		// Parse error response for better messages
		var errResp ErrorResponse
		if json.Unmarshal(bodyBytes, &errResp) == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("chatterbox error: %s", errResp.Error.Message)
		}

		return nil, fmt.Errorf("chatterbox error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	p.healthy.Store(true)
	return io.ReadAll(resp.Body)
}

// Stream sends a streaming synthesis request.
// Note: Chatterbox streaming support depends on the API version.
func (p *Provider) Stream(ctx context.Context, req *voice.SynthesizeRequest) (voice.AudioStream, error) {
	// For now, fall back to full synthesis and wrap in a stream
	// Chatterbox API may support streaming in future versions
	resp, err := p.Synthesize(ctx, req)
	if err != nil {
		return nil, err
	}

	format := voice.FormatWAV
	if req.Format != "" {
		format = req.Format
	}

	sampleRate := 24000
	if req.SampleRate != 0 {
		sampleRate = req.SampleRate
	}

	return &audioStream{
		ReadCloser: io.NopCloser(bytes.NewReader(resp.Audio)),
		format:     format,
		sampleRate: sampleRate,
	}, nil
}

// ═══════════════════════════════════════════════════════════════════════════
// HEALTH & CAPABILITIES
// ═══════════════════════════════════════════════════════════════════════════

// Health checks if Chatterbox is available.
func (p *Provider) Health(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", p.config.BaseURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		p.healthy.Store(false)
		if ctx.Err() != nil {
			return voice.ErrTimeout
		}
		return voice.ErrProviderUnavailable
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		p.healthy.Store(false)
		return fmt.Errorf("health check failed with status %d", resp.StatusCode)
	}

	p.healthy.Store(true)

	log.Info().
		Str("provider", p.Name()).
		Str("base_url", p.config.BaseURL).
		Msg("Chatterbox health check passed")

	return nil
}

// IsHealthy returns cached health status.
func (p *Provider) IsHealthy() bool {
	return p.healthy.Load()
}

// ListVoices returns available voices from Chatterbox.
func (p *Provider) ListVoices(ctx context.Context) ([]voice.Voice, error) {
	// Get voices from library
	library, err := p.GetVoiceLibrary(ctx)
	if err != nil {
		// Return default voice if library unavailable
		return []voice.Voice{
			{
				ID:       "default",
				Name:     "Default",
				Language: "en",
				Gender:   voice.GenderNeutral,
				IsCloned: false,
			},
		}, nil
	}

	voices := []voice.Voice{
		{
			ID:       "default",
			Name:     "Default",
			Language: "en",
			Gender:   voice.GenderNeutral,
			IsCloned: false,
		},
	}

	// Add library voices
	for _, v := range library.Voices {
		voices = append(voices, voice.Voice{
			ID:       v.Name,
			Name:     v.Name,
			Language: "en", // Chatterbox supports multiple but defaults to English
			Gender:   voice.GenderUnknown,
			IsCloned: true,
			Metadata: map[string]string{
				"filename":   v.Filename,
				"file_size":  fmt.Sprintf("%d", v.FileSize),
				"upload_date": v.UploadDate,
			},
		})
	}

	return voices, nil
}

// Capabilities returns the capabilities of Chatterbox.
func (p *Provider) Capabilities() voice.ProviderCapabilities {
	return voice.ProviderCapabilities{
		SupportsStreaming: false, // Full synthesis then stream
		SupportsCloning:   true,  // Voice library with permanent storage
		Languages: []string{
			"en", "es", "fr", "de", "it", "pt", "pl", "tr",
			"ru", "nl", "cs", "ar", "zh", "ja", "hu", "ko",
			"hi", "sv", "da", "fi", "no", "el", "he", // 23 languages
		},
		MaxTextLength: p.config.MaxTextLength,
		RequiresGPU:   true, // Optimal performance requires GPU
		AvgLatencyMs:  400,  // ~400ms (vs XTTS ~1500ms)
		SupportedFormats: []voice.AudioFormat{
			voice.FormatWAV,
		},
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// EMOTION CONTROL HELPERS
// ═══════════════════════════════════════════════════════════════════════════

// SetExaggeration updates the default emotion intensity.
func (p *Provider) SetExaggeration(value float64) {
	if value >= 0.25 && value <= 2.0 {
		p.config.Exaggeration = value
	}
}

// SetCFGWeight updates the default pace control.
func (p *Provider) SetCFGWeight(value float64) {
	if value >= 0.0 && value <= 1.0 {
		p.config.CFGWeight = value
	}
}

// SetTemperature updates the default sampling temperature.
func (p *Provider) SetTemperature(value float64) {
	if value >= 0.05 && value <= 5.0 {
		p.config.Temperature = value
	}
}

// GetConfig returns the current configuration.
func (p *Provider) GetConfig() Config {
	return p.config
}

// ═══════════════════════════════════════════════════════════════════════════
// AUDIO STREAM IMPLEMENTATION
// ═══════════════════════════════════════════════════════════════════════════

// audioStream implements voice.AudioStream.
type audioStream struct {
	io.ReadCloser
	format     voice.AudioFormat
	sampleRate int
}

// Format returns the audio format of the stream.
func (s *audioStream) Format() voice.AudioFormat {
	return s.format
}

// SampleRate returns the sample rate in Hz.
func (s *audioStream) SampleRate() int {
	return s.sampleRate
}
