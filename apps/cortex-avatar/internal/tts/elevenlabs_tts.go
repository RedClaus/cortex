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

const (
	ElevenLabsAPIEndpoint  = "https://api.elevenlabs.io/v1"
	ElevenLabsDefaultVoice = "21m00Tcm4TlvDq8ikWAM" // Rachel - calm, natural female
)

type ElevenLabsProvider struct {
	apiKey string
	logger zerolog.Logger
	config *ElevenLabsConfig
	client *http.Client
}

type ElevenLabsConfig struct {
	APIKey       string  `json:"api_key"`
	DefaultVoice string  `json:"default_voice"`
	ModelID      string  `json:"model_id"`
	Stability    float64 `json:"stability"`
	Similarity   float64 `json:"similarity_boost"`
}

func DefaultElevenLabsConfig() *ElevenLabsConfig {
	return &ElevenLabsConfig{
		DefaultVoice: ElevenLabsDefaultVoice,
		ModelID:      "eleven_monolingual_v1",
		Stability:    0.5,
		Similarity:   0.75,
	}
}

func NewElevenLabsProvider(logger zerolog.Logger, config *ElevenLabsConfig) *ElevenLabsProvider {
	if config == nil {
		config = DefaultElevenLabsConfig()
	}

	apiKey := config.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("ELEVENLABS_API_KEY")
	}

	return &ElevenLabsProvider{
		apiKey: apiKey,
		logger: logger.With().Str("provider", "elevenlabs-tts").Logger(),
		config: config,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *ElevenLabsProvider) Name() string {
	return "elevenlabs"
}

func (p *ElevenLabsProvider) IsAvailable() bool {
	return p.apiKey != ""
}

func (p *ElevenLabsProvider) SetAPIKey(key string) {
	p.apiKey = key
}

var elevenLabsVoiceMap = map[string]string{
	"nova":    "21m00Tcm4TlvDq8ikWAM", // Rachel
	"shimmer": "EXAVITQu4vr4xnSDxMaL", // Bella
	"alloy":   "MF3mGyEYCl7XYWbV9V6O", // Emily
	"echo":    "VR6AewLTigWG4xSOukaG", // Arnold
	"onyx":    "ErXwobaYiN019PkySvjV", // Antoni
	"fable":   "TxGEqnHWrfWFTfGW9XjX", // Josh
}

func (p *ElevenLabsProvider) Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	if !p.IsAvailable() {
		return nil, fmt.Errorf("ElevenLabs API key not set")
	}

	startTime := time.Now()

	voiceID := req.VoiceID
	if voiceID == "" {
		voiceID = p.config.DefaultVoice
	}
	if mapped, ok := elevenLabsVoiceMap[voiceID]; ok {
		voiceID = mapped
	}

	payload := map[string]any{
		"text":     req.Text,
		"model_id": p.config.ModelID,
		"voice_settings": map[string]float64{
			"stability":        p.config.Stability,
			"similarity_boost": p.config.Similarity,
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/text-to-speech/%s", ElevenLabsAPIEndpoint, voiceID)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("xi-api-key", p.apiKey)
	httpReq.Header.Set("Accept", "audio/mpeg")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ElevenLabs API error %d: %s", resp.StatusCode, string(body))
	}

	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read audio: %w", err)
	}

	processingTime := time.Since(startTime)

	p.logger.Info().
		Str("voice", voiceID).
		Int("audioBytes", len(audioData)).
		Dur("processingTime", processingTime).
		Msg("ElevenLabs TTS synthesis complete")

	return &SynthesizeResponse{
		Audio:          audioData,
		Format:         "mp3",
		SampleRate:     22050,
		ProcessingTime: processingTime,
		VoiceID:        voiceID,
		Provider:       p.Name(),
	}, nil
}

func (p *ElevenLabsProvider) SynthesizeStream(ctx context.Context, req *SynthesizeRequest) (<-chan *AudioChunk, error) {
	chunks := make(chan *AudioChunk, 1)

	go func() {
		defer close(chunks)

		resp, err := p.Synthesize(ctx, req)
		if err != nil {
			p.logger.Error().Err(err).Msg("Stream synthesis failed")
			return
		}

		chunks <- &AudioChunk{
			Data:    resp.Audio,
			Index:   0,
			IsFinal: true,
		}
	}()

	return chunks, nil
}

func (p *ElevenLabsProvider) ListVoices(ctx context.Context) ([]Voice, error) {
	return []Voice{
		{ID: "21m00Tcm4TlvDq8ikWAM", Name: "Rachel (Female)", Language: "en-US", Gender: "female"},
		{ID: "EXAVITQu4vr4xnSDxMaL", Name: "Bella (Female)", Language: "en-US", Gender: "female"},
		{ID: "MF3mGyEYCl7XYWbV9V6O", Name: "Emily (Female)", Language: "en-US", Gender: "female"},
		{ID: "ErXwobaYiN019PkySvjV", Name: "Antoni (Male)", Language: "en-US", Gender: "male"},
		{ID: "VR6AewLTigWG4xSOukaG", Name: "Arnold (Male)", Language: "en-US", Gender: "male"},
		{ID: "TxGEqnHWrfWFTfGW9XjX", Name: "Josh (Male)", Language: "en-US", Gender: "male"},
	}, nil
}

func (p *ElevenLabsProvider) Health(ctx context.Context) error {
	if !p.IsAvailable() {
		return ErrProviderUnavailable
	}
	return nil
}

func (p *ElevenLabsProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsStreaming:  true,
		SupportsCloning:    true,
		SupportsPhonemes:   false,
		SupportedLanguages: []string{"en", "de", "pl", "es", "it", "fr", "pt", "hi"},
		MaxTextLength:      5000,
		AvgLatencyMs:       500,
		RequiresGPU:        false,
		IsLocal:            false,
	}
}
