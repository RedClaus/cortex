package tts

import (
	"context"
	"time"

	"github.com/normanking/cortexavatar/internal/a2a"
	"github.com/rs/zerolog"
)

// A2AProvider implements TTS by delegating to CortexBrain via A2A protocol.
type A2AProvider struct {
	client *a2a.Client
	logger zerolog.Logger
	config *A2AConfig
}

// A2AConfig holds A2A TTS provider configuration
type A2AConfig struct {
	Timeout      time.Duration `json:"timeout"`
	DefaultVoice string        `json:"default_voice"`
	Speed        float64       `json:"speed"`
}

// DefaultA2AConfig returns sensible defaults
func DefaultA2AConfig() *A2AConfig {
	return &A2AConfig{
		Timeout:      30 * time.Second,
		DefaultVoice: "af_bella",
		Speed:        1.0,
	}
}

// NewA2AProvider creates a new A2A-based TTS provider
func NewA2AProvider(client *a2a.Client, logger zerolog.Logger, config *A2AConfig) *A2AProvider {
	if config == nil {
		config = DefaultA2AConfig()
	}

	return &A2AProvider{
		client: client,
		logger: logger.With().Str("provider", "a2a-tts").Logger(),
		config: config,
	}
}

// Name returns the provider identifier
func (p *A2AProvider) Name() string {
	return "a2a"
}

// Synthesize sends text to CortexBrain for TTS synthesis
// Note: For now, this just sends the text as a message and CortexBrain will speak it.
// The actual audio is played by CortexBrain's voice system, not returned here.
func (p *A2AProvider) Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	startTime := time.Now()

	// Validate
	if req.Text == "" {
		return nil, ErrTextTooLong
	}

	// Use default voice if not specified
	voiceID := req.VoiceID
	if voiceID == "" {
		voiceID = p.config.DefaultVoice
	}

	// Create a message that CortexBrain will respond to
	// For TTS, we're just sending the text to be spoken
	messageText := req.Text

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, p.config.Timeout)
	defer cancel()

	// Send to CortexBrain
	response, err := p.client.SendMessage(ctx, messageText)
	if err != nil {
		p.logger.Error().Err(err).Msg("Failed to send TTS request to CortexBrain")
		return &SynthesizeResponse{
			Error: err.Error(),
		}, err
	}

	processingTime := time.Since(startTime)

	// Note: The A2A protocol doesn't return audio directly.
	// CortexBrain handles TTS internally and speaks via its own audio system.
	// For CortexAvatar to play audio, we'd need to either:
	// 1. Have CortexBrain return audio in the response
	// 2. Use a direct TTS API (Kokoro)
	// For now, we return the response text
	responseText := ""
	if response != nil {
		responseText = response.ExtractText()
	}

	return &SynthesizeResponse{
		Audio:          nil, // No audio returned via A2A
		Format:         "wav",
		SampleRate:     16000,
		Duration:       0,
		ProcessingTime: processingTime,
		VoiceID:        voiceID,
		Provider:       p.Name(),
		Error:          responseText, // Store response for debugging
	}, nil
}

// SynthesizeStream handles streaming synthesis
func (p *A2AProvider) SynthesizeStream(ctx context.Context, req *SynthesizeRequest) (<-chan *AudioChunk, error) {
	chunks := make(chan *AudioChunk, 10)

	go func() {
		defer close(chunks)

		// For A2A, we don't have audio streaming
		// Just send an empty final chunk
		chunks <- &AudioChunk{
			Data:    nil,
			Index:   0,
			IsFinal: true,
		}
	}()

	return chunks, nil
}

// ListVoices returns available voices from CortexBrain
func (p *A2AProvider) ListVoices(ctx context.Context) ([]Voice, error) {
	// Return common Kokoro voices (CortexBrain uses Kokoro for fast lane)
	return []Voice{
		{ID: "af_bella", Name: "Bella (US Female)", Language: "en", Gender: "female"},
		{ID: "af_sarah", Name: "Sarah (US Female)", Language: "en", Gender: "female"},
		{ID: "am_adam", Name: "Adam (US Male)", Language: "en", Gender: "male"},
		{ID: "am_michael", Name: "Michael (US Male)", Language: "en", Gender: "male"},
		{ID: "bf_emma", Name: "Emma (British Female)", Language: "en", Gender: "female"},
		{ID: "bm_george", Name: "George (British Male)", Language: "en", Gender: "male"},
	}, nil
}

// Health checks if CortexBrain is available for TTS
func (p *A2AProvider) Health(ctx context.Context) error {
	if !p.client.IsConnected() {
		return ErrProviderUnavailable
	}
	return nil
}

// Capabilities returns what this provider supports
func (p *A2AProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsStreaming:  false, // Batched streaming only
		SupportsCloning:    false, // Depends on CortexBrain config
		SupportsPhonemes:   false, // Needs CortexBrain enhancement
		SupportedLanguages: []string{"en"},
		MaxTextLength:      2000,
		AvgLatencyMs:       300, // Kokoro is fast
		RequiresGPU:        false,
		IsLocal:            false,
	}
}
