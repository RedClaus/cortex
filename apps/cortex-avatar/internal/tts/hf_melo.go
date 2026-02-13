package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// HFMeloProvider uses the HF Voice Service (MeloTTS) for TTS
type HFMeloProvider struct {
	config     *HFMeloConfig
	httpClient *http.Client
	logger     zerolog.Logger
}

// HFMeloConfig holds configuration for the HF Melo provider
type HFMeloConfig struct {
	ServiceURL   string  `json:"service_url"`    // e.g., "http://localhost:8899"
	Timeout      int     `json:"timeout_sec"`    // HTTP timeout in seconds
	DefaultVoice string  `json:"default_voice"`  // Default language/voice
	DefaultSpeed float64 `json:"default_speed"`  // Speech speed (0.5-2.0)
}

// DefaultHFMeloConfig returns sensible defaults
func DefaultHFMeloConfig() *HFMeloConfig {
	return &HFMeloConfig{
		ServiceURL:   "http://localhost:8899",
		Timeout:      30,
		DefaultVoice: "EN",
		DefaultSpeed: 1.0,
	}
}

// NewHFMeloProvider creates a new HF Melo TTS provider
func NewHFMeloProvider(config *HFMeloConfig, logger zerolog.Logger) *HFMeloProvider {
	if config == nil {
		config = DefaultHFMeloConfig()
	}

	return &HFMeloProvider{
		config: config,
		httpClient: &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Second,
		},
		logger: logger.With().Str("provider", "hf_melo").Logger(),
	}
}

// Name returns the provider identifier
func (p *HFMeloProvider) Name() string {
	return "hf_melo"
}

// Synthesize converts text to audio using HF Voice Service
func (p *HFMeloProvider) Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	startTime := time.Now()

	// Map voice ID to language (MeloTTS uses language codes)
	language := p.mapVoiceToLanguage(req.VoiceID)

	// Determine speed
	speed := req.Speed
	if speed == 0 {
		speed = p.config.DefaultSpeed
	}

	// Build request payload
	ttsReq := map[string]interface{}{
		"text":     req.Text,
		"language": language,
		"speed":    speed,
	}

	payloadBytes, err := json.Marshal(ttsReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build request URL
	url := fmt.Sprintf("%s/tts", p.config.ServiceURL)

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	p.logger.Debug().
		Str("url", url).
		Str("text", req.Text).
		Str("language", language).
		Float64("speed", speed).
		Msg("Sending TTS request to HF service")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HF service returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Read audio data (streaming WAV)
	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio data: %w", err)
	}

	processingTime := time.Since(startTime)

	p.logger.Info().
		Int("audio_bytes", len(audioData)).
		Dur("processing_time", processingTime).
		Msg("TTS synthesis complete")

	return &SynthesizeResponse{
		Audio:          audioData,
		Format:         "wav",
		SampleRate:     16000, // MeloTTS outputs 16kHz
		Duration:       0,     // Not calculated
		ProcessingTime: processingTime,
		VoiceID:        req.VoiceID,
		Provider:       "hf_melo",
	}, nil
}

// SynthesizeStream handles streaming synthesis (HF Melo returns full audio, but we can chunk it)
func (p *HFMeloProvider) SynthesizeStream(ctx context.Context, req *SynthesizeRequest) (<-chan *AudioChunk, error) {
	// HF service returns complete WAV file, not true streaming
	// We'll chunk it for compatibility with streaming interface

	ch := make(chan *AudioChunk, 10)

	go func() {
		defer close(ch)

		resp, err := p.Synthesize(ctx, req)
		if err != nil {
			p.logger.Error().Err(err).Msg("Synthesis failed")
			return
		}

		// Send audio in chunks
		chunkSize := 8192 // 8KB chunks
		audioData := resp.Audio
		totalChunks := (len(audioData) + chunkSize - 1) / chunkSize

		for i := 0; i < totalChunks; i++ {
			start := i * chunkSize
			end := start + chunkSize
			if end > len(audioData) {
				end = len(audioData)
			}

			chunk := &AudioChunk{
				Data:    audioData[start:end],
				Index:   i,
				IsFinal: i == totalChunks-1,
			}

			select {
			case ch <- chunk:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

// ListVoices returns available voices (MeloTTS languages)
func (p *HFMeloProvider) ListVoices(ctx context.Context) ([]Voice, error) {
	// MeloTTS supports 6 languages with default speakers
	voices := []Voice{
		{ID: "en", Name: "English (US)", Language: "en", Gender: "neutral"},
		{ID: "fr", Name: "French", Language: "fr", Gender: "neutral"},
		{ID: "es", Name: "Spanish", Language: "es", Gender: "neutral"},
		{ID: "zh", Name: "Chinese (Mandarin)", Language: "zh", Gender: "neutral"},
		{ID: "ja", Name: "Japanese", Language: "ja", Gender: "neutral"},
		{ID: "ko", Name: "Korean", Language: "ko", Gender: "neutral"},
	}

	return voices, nil
}

// Health checks if the HF service is available
func (p *HFMeloProvider) Health(ctx context.Context) error {
	url := fmt.Sprintf("%s/health", p.config.ServiceURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HF service unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HF service unhealthy (status %d)", resp.StatusCode)
	}

	p.logger.Debug().Msg("HF service health check passed")
	return nil
}

// Capabilities returns the provider's feature set
func (p *HFMeloProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsStreaming:  true, // Chunked streaming supported
		SupportsCloning:    false,
		SupportsPhonemes:   false, // MeloTTS doesn't provide phoneme data
		SupportedLanguages: []string{"EN", "FR", "ES", "ZH", "JA", "KO"},
		MaxTextLength:      500,  // Reasonable limit for sentence-level synthesis
		AvgLatencyMs:       700,  // ~700ms from POC benchmarks
		RequiresGPU:        false, // MPS acceleration, but optional
		IsLocal:            true,  // Runs locally via microservice
	}
}

// mapVoiceToLanguage maps voice IDs to MeloTTS language codes
func (p *HFMeloProvider) mapVoiceToLanguage(voiceID string) string {
	// If voice ID is already a language code, use it directly
	validLanguages := map[string]bool{
		"EN": true, "en": true,
		"FR": true, "fr": true,
		"ES": true, "es": true,
		"ZH": true, "zh": true,
		"JA": true, "ja": true,
		"KO": true, "ko": true,
	}

	if validLanguages[voiceID] {
		// Normalize to uppercase
		switch voiceID {
		case "en":
			return "EN"
		case "fr":
			return "FR"
		case "es":
			return "ES"
		case "zh":
			return "ZH"
		case "ja":
			return "JA"
		case "ko":
			return "KO"
		default:
			return voiceID
		}
	}

	// Default to configured voice
	return p.config.DefaultVoice
}
