package kokoro

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/normanking/cortex/internal/voice"
	"github.com/rs/zerolog/log"
)

// Provider implements voice.Provider for Kokoro TTS (Fast Lane).
// Kokoro is a lightweight, CPU-only TTS engine with <300ms latency.
// 82M parameters, preset voices only (no cloning).
type Provider struct {
	config     Config
	httpClient *http.Client
}

// Config holds Kokoro-specific configuration.
type Config struct {
	BaseURL       string        // Default: http://localhost:8880
	Timeout       time.Duration // Default: 5s (fast lane)
	DefaultVoice  string        // Default: "af_bella"
	MaxTextLength int           // Default: 2000
}

// NewProvider creates a new Kokoro provider with the given config.
func NewProvider(config Config) *Provider {
	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:8880"
	}
	if config.Timeout == 0 {
		config.Timeout = 5 * time.Second
	}
	if config.DefaultVoice == "" {
		config.DefaultVoice = "af_bella"
	}
	if config.MaxTextLength == 0 {
		config.MaxTextLength = 2000
	}

	return &Provider{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Name returns the provider identifier.
func (p *Provider) Name() string {
	return "kokoro"
}

// PresetVoices defines the available Kokoro voices.
var PresetVoices = []voice.Voice{
	// Female US voices
	{
		ID:       "af_bella",
		Name:     "Bella (US Female)",
		Language: "en",
		Gender:   voice.GenderFemale,
		IsCloned: false,
		Metadata: map[string]string{
			"style":  "natural",
			"region": "us",
		},
	},
	{
		ID:       "af_sarah",
		Name:     "Sarah (US Female)",
		Language: "en",
		Gender:   voice.GenderFemale,
		IsCloned: false,
		Metadata: map[string]string{
			"style":  "professional",
			"region": "us",
		},
	},
	// Male US voices
	{
		ID:       "am_adam",
		Name:     "Adam (US Male)",
		Language: "en",
		Gender:   voice.GenderMale,
		IsCloned: false,
		Metadata: map[string]string{
			"style":  "casual",
			"region": "us",
		},
	},
	{
		ID:       "am_michael",
		Name:     "Michael (US Male)",
		Language: "en",
		Gender:   voice.GenderMale,
		IsCloned: false,
		Metadata: map[string]string{
			"style":  "authoritative",
			"region": "us",
		},
	},
	// British voices
	{
		ID:       "bf_emma",
		Name:     "Emma (British Female)",
		Language: "en",
		Gender:   voice.GenderFemale,
		IsCloned: false,
		Metadata: map[string]string{
			"style":  "refined",
			"region": "uk",
		},
	},
	{
		ID:       "bm_george",
		Name:     "George (British Male)",
		Language: "en",
		Gender:   voice.GenderMale,
		IsCloned: false,
		Metadata: map[string]string{
			"style":  "distinguished",
			"region": "uk",
		},
	},
}

// kokoroRequest represents the API request format for Kokoro-FastAPI (OpenAI-compatible).
type kokoroRequest struct {
	Model string  `json:"model"`           // Model name (always "kokoro")
	Input string  `json:"input"`           // Text to synthesize
	Voice string  `json:"voice"`           // Voice ID
	Speed float64 `json:"speed,omitempty"` // Speed multiplier (0.5-2.0)
}

// kokoroResponse represents the API response format.
type kokoroResponse struct {
	AudioData  []byte `json:"audio_data,omitempty"` // Base64 or raw bytes
	Format     string `json:"format,omitempty"`
	Duration   int64  `json:"duration_ms,omitempty"`
	SampleRate int    `json:"sample_rate,omitempty"`
}

// Synthesize sends a synthesis request to Kokoro and returns the full audio response.
func (p *Provider) Synthesize(ctx context.Context, req *voice.SynthesizeRequest) (*voice.SynthesizeResponse, error) {
	start := time.Now()

	if err := voice.ValidateRequest(req); err != nil {
		return nil, err
	}

	// Check text length
	if len(req.Text) > p.config.MaxTextLength {
		return nil, voice.ErrTextTooLong
	}

	// Use default voice if not specified
	voiceID := req.VoiceID
	if voiceID == "" {
		voiceID = p.config.DefaultVoice
	}

	// Validate voice exists
	if !p.isValidVoice(voiceID) {
		return nil, voice.ErrVoiceNotFound
	}

	// Build request
	speed := req.Speed
	if speed == 0 {
		speed = 1.0
	}

	kokoroReq := kokoroRequest{
		Model: "kokoro",
		Input: req.Text,
		Voice: voiceID,
		Speed: speed,
	}

	body, err := json.Marshal(kokoroReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.config.BaseURL+"/v1/audio/speech", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, voice.ErrTimeout
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("kokoro returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Read audio data - Kokoro returns raw WAV data
	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio data: %w", err)
	}

	processingTime := time.Since(start).Milliseconds()

	return &voice.SynthesizeResponse{
		Audio:       audioData,
		Format:      voice.FormatWAV,
		Duration:    0,     // Would need to parse WAV header for accurate duration
		SampleRate:  22050, // Kokoro default sample rate
		ProcessedMs: processingTime,
		VoiceID:     voiceID,
		Provider:    p.Name(),
	}, nil
}

// Stream sends a streaming synthesis request to Kokoro.
func (p *Provider) Stream(ctx context.Context, req *voice.SynthesizeRequest) (voice.AudioStream, error) {
	if err := voice.ValidateRequest(req); err != nil {
		return nil, err
	}

	// Check text length
	if len(req.Text) > p.config.MaxTextLength {
		return nil, voice.ErrTextTooLong
	}

	// Use default voice if not specified
	voiceID := req.VoiceID
	if voiceID == "" {
		voiceID = p.config.DefaultVoice
	}

	// Validate voice exists
	if !p.isValidVoice(voiceID) {
		return nil, voice.ErrVoiceNotFound
	}

	// Build request
	speed := req.Speed
	if speed == 0 {
		speed = 1.0
	}

	kokoroReq := kokoroRequest{
		Model: "kokoro",
		Input: req.Text,
		Voice: voiceID,
		Speed: speed,
	}

	body, err := json.Marshal(kokoroReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.config.BaseURL+"/v1/audio/speech/stream", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, voice.ErrTimeout
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("kokoro returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return &audioStream{
		reader:     resp.Body,
		format:     voice.FormatWAV,
		sampleRate: 22050,
	}, nil
}

// audioStream implements voice.AudioStream interface.
type audioStream struct {
	reader     io.ReadCloser
	format     voice.AudioFormat
	sampleRate int
}

func (s *audioStream) Read(p []byte) (n int, err error) {
	return s.reader.Read(p)
}

func (s *audioStream) Close() error {
	return s.reader.Close()
}

func (s *audioStream) Format() voice.AudioFormat {
	return s.format
}

func (s *audioStream) SampleRate() int {
	return s.sampleRate
}

// ListVoices returns available voices from Kokoro.
// Kokoro has a fixed set of preset voices.
func (p *Provider) ListVoices(ctx context.Context) ([]voice.Voice, error) {
	// Kokoro has preset voices - no need to query the API
	return PresetVoices, nil
}

// Health checks if Kokoro is available.
func (p *Provider) Health(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", p.config.BaseURL+"/health", nil)
	if err != nil {
		return err
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return voice.ErrProviderUnavailable
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("kokoro health check failed: status %d", resp.StatusCode)
	}

	return nil
}

// Capabilities returns Kokoro's feature set.
func (p *Provider) Capabilities() voice.ProviderCapabilities {
	return voice.ProviderCapabilities{
		SupportsStreaming: true,
		SupportsCloning:   false,          // Kokoro does not support voice cloning
		Languages:         []string{"en"}, // English only
		MaxTextLength:     p.config.MaxTextLength,
		RequiresGPU:       false, // CPU-only
		AvgLatencyMs:      250,   // <300ms typical
		SupportedFormats: []voice.AudioFormat{
			voice.FormatWAV,
		},
	}
}

// isValidVoice checks if a voice ID exists in the preset voices.
func (p *Provider) isValidVoice(voiceID string) bool {
	for _, v := range PresetVoices {
		if v.ID == voiceID {
			return true
		}
	}
	return false
}

// getVoiceByID returns a voice by its ID.
func (p *Provider) getVoiceByID(voiceID string) *voice.Voice {
	for _, v := range PresetVoices {
		if v.ID == voiceID {
			return &v
		}
	}
	return nil
}

// ValidateVoice checks if a voice ID is valid and returns the voice details.
func (p *Provider) ValidateVoice(voiceID string) (*voice.Voice, error) {
	v := p.getVoiceByID(voiceID)
	if v == nil {
		return nil, voice.ErrVoiceNotFound
	}
	return v, nil
}

// GetDefaultVoice returns the default voice configuration.
func (p *Provider) GetDefaultVoice() string {
	return p.config.DefaultVoice
}

// SetDefaultVoice updates the default voice.
func (p *Provider) SetDefaultVoice(voiceID string) error {
	if !p.isValidVoice(voiceID) {
		return voice.ErrVoiceNotFound
	}
	p.config.DefaultVoice = voiceID
	log.Info().Str("voice", voiceID).Msg("default voice updated")
	return nil
}
