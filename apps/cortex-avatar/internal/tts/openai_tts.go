// Package tts provides OpenAI TTS provider for high-quality voice synthesis.
package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog"
)

// OpenAI TTS voices - all very natural sounding
const (
	VoiceAlloy   = "alloy"   // Neutral, balanced
	VoiceEcho    = "echo"    // Male, warm
	VoiceFable   = "fable"   // British, expressive
	VoiceOnyx    = "onyx"    // Male, deep
	VoiceNova    = "nova"    // Female, warm and natural (recommended)
	VoiceShimmer = "shimmer" // Female, clear and bright
)

// OpenAIProvider implements TTS using OpenAI's TTS API
type OpenAIProvider struct {
	apiKey     string
	client     *http.Client
	logger     zerolog.Logger
	config     *OpenAIConfig
}

// OpenAIConfig holds OpenAI TTS configuration
type OpenAIConfig struct {
	APIKey       string        `json:"api_key"`
	Model        string        `json:"model"`         // tts-1 or tts-1-hd
	DefaultVoice string        `json:"default_voice"` // alloy, echo, fable, onyx, nova, shimmer
	Speed        float64       `json:"speed"`         // 0.25 to 4.0
	Timeout      time.Duration `json:"timeout"`
}

// DefaultOpenAIConfig returns sensible defaults
func DefaultOpenAIConfig() *OpenAIConfig {
	return &OpenAIConfig{
		Model:        "tts-1",      // Fast, good quality
		DefaultVoice: VoiceNova,    // Natural female voice
		Speed:        1.0,
		Timeout:      30 * time.Second,
	}
}

// NewOpenAIProvider creates a new OpenAI TTS provider
func NewOpenAIProvider(logger zerolog.Logger, config *OpenAIConfig) *OpenAIProvider {
	if config == nil {
		config = DefaultOpenAIConfig()
	}

	// Get API key from config or environment
	apiKey := config.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	return &OpenAIProvider{
		apiKey: apiKey,
		client: &http.Client{Timeout: config.Timeout},
		logger: logger.With().Str("provider", "openai-tts").Logger(),
		config: config,
	}
}

// Name returns the provider identifier
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// IsAvailable checks if the provider has an API key configured
func (p *OpenAIProvider) IsAvailable() bool {
	return p.apiKey != ""
}

// SetAPIKey sets the API key
func (p *OpenAIProvider) SetAPIKey(key string) {
	p.apiKey = key
	p.logger.Info().Msg("OpenAI API key updated")
}

// openAITTSRequest is the request format for OpenAI TTS API
type openAITTSRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	ResponseFormat string  `json:"response_format,omitempty"`
	Speed          float64 `json:"speed,omitempty"`
}

// Synthesize converts text to audio using OpenAI TTS
func (p *OpenAIProvider) Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key not configured")
	}

	startTime := time.Now()

	// Use default voice if not specified
	voiceID := req.VoiceID
	if voiceID == "" {
		voiceID = p.config.DefaultVoice
	}

	// Map our voice IDs to OpenAI voices
	openAIVoice := p.mapVoice(voiceID)

	// Determine speed
	speed := req.Speed
	if speed == 0 {
		speed = p.config.Speed
	}

	// Create request
	ttsReq := openAITTSRequest{
		Model:          p.config.Model,
		Input:          req.Text,
		Voice:          openAIVoice,
		ResponseFormat: "mp3", // MP3 is efficient and widely supported
		Speed:          speed,
	}

	body, err := json.Marshal(ttsReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/audio/speech", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	p.logger.Debug().
		Str("voice", openAIVoice).
		Str("model", p.config.Model).
		Int("textLen", len(req.Text)).
		Msg("Sending TTS request to OpenAI")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		p.logger.Error().
			Int("status", resp.StatusCode).
			Str("body", string(bodyBytes)).
			Msg("OpenAI TTS request failed")
		return nil, fmt.Errorf("OpenAI TTS error: %s", string(bodyBytes))
	}

	// Read audio data
	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	processingTime := time.Since(startTime)

	p.logger.Info().
		Str("voice", openAIVoice).
		Int("audioBytes", len(audioData)).
		Dur("processingTime", processingTime).
		Msg("OpenAI TTS synthesis complete")

	return &SynthesizeResponse{
		Audio:          audioData,
		Format:         "mp3",
		SampleRate:     24000, // OpenAI TTS uses 24kHz
		ProcessingTime: processingTime,
		VoiceID:        voiceID,
		Provider:       p.Name(),
	}, nil
}

// mapVoice maps our voice IDs to OpenAI voices
func (p *OpenAIProvider) mapVoice(voiceID string) string {
	// Map avatar voices to OpenAI voices
	switch voiceID {
	case "af_bella", "af_sarah", "af_sky":
		return VoiceNova // Female, warm
	case "am_adam", "am_michael":
		return VoiceOnyx // Male, deep
	case "bf_emma":
		return VoiceShimmer // Female, clear
	case "bm_george":
		return VoiceEcho // Male, warm
	default:
		// Check if it's already an OpenAI voice
		switch voiceID {
		case VoiceAlloy, VoiceEcho, VoiceFable, VoiceOnyx, VoiceNova, VoiceShimmer:
			return voiceID
		}
		return p.config.DefaultVoice
	}
}

// SynthesizeStream handles streaming synthesis (OpenAI doesn't support true streaming yet)
func (p *OpenAIProvider) SynthesizeStream(ctx context.Context, req *SynthesizeRequest) (<-chan *AudioChunk, error) {
	chunks := make(chan *AudioChunk, 1)

	go func() {
		defer close(chunks)

		// Synthesize the full audio
		resp, err := p.Synthesize(ctx, req)
		if err != nil {
			p.logger.Error().Err(err).Msg("Stream synthesis failed")
			return
		}

		// Send as a single chunk
		chunks <- &AudioChunk{
			Data:    resp.Audio,
			Index:   0,
			IsFinal: true,
		}
	}()

	return chunks, nil
}

// ListVoices returns available OpenAI voices
func (p *OpenAIProvider) ListVoices(ctx context.Context) ([]Voice, error) {
	return []Voice{
		{ID: VoiceNova, Name: "Nova (Female, Warm)", Language: "en", Gender: "female"},
		{ID: VoiceShimmer, Name: "Shimmer (Female, Clear)", Language: "en", Gender: "female"},
		{ID: VoiceAlloy, Name: "Alloy (Neutral)", Language: "en", Gender: "neutral"},
		{ID: VoiceEcho, Name: "Echo (Male, Warm)", Language: "en", Gender: "male"},
		{ID: VoiceOnyx, Name: "Onyx (Male, Deep)", Language: "en", Gender: "male"},
		{ID: VoiceFable, Name: "Fable (British)", Language: "en", Gender: "neutral"},
	}, nil
}

// Health checks if OpenAI API is available
func (p *OpenAIProvider) Health(ctx context.Context) error {
	if p.apiKey == "" {
		return ErrProviderUnavailable
	}
	return nil
}

// Capabilities returns OpenAI TTS capabilities
func (p *OpenAIProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsStreaming:  false, // OpenAI doesn't support true streaming
		SupportsCloning:    false,
		SupportsPhonemes:   false,
		SupportedLanguages: []string{"en", "es", "fr", "de", "it", "pt", "pl", "ja", "ko", "zh"},
		MaxTextLength:      4096,
		AvgLatencyMs:       500, // ~500ms for short texts
		RequiresGPU:        false,
		IsLocal:            false,
	}
}
