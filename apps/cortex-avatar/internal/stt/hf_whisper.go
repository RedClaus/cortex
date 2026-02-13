package stt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// HFWhisperProvider uses the HF Voice Service (Lightning Whisper MLX) for STT
type HFWhisperProvider struct {
	config     *HFWhisperConfig
	httpClient *http.Client
	logger     zerolog.Logger
}

// HFWhisperConfig holds configuration for the HF Whisper provider
type HFWhisperConfig struct {
	ServiceURL string `json:"service_url"` // e.g., "http://localhost:8899"
	Timeout    int    `json:"timeout_sec"` // HTTP timeout in seconds
	Language   string `json:"language"`    // Default language (e.g., "en")
}

// DefaultHFWhisperConfig returns sensible defaults
func DefaultHFWhisperConfig() *HFWhisperConfig {
	return &HFWhisperConfig{
		ServiceURL: "http://localhost:8899",
		Timeout:    30,
		Language:   "en",
	}
}

// NewHFWhisperProvider creates a new HF Whisper STT provider
func NewHFWhisperProvider(config *HFWhisperConfig, logger zerolog.Logger) *HFWhisperProvider {
	if config == nil {
		config = DefaultHFWhisperConfig()
	}

	return &HFWhisperProvider{
		config: config,
		httpClient: &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Second,
		},
		logger: logger.With().Str("provider", "hf_whisper").Logger(),
	}
}

// Name returns the provider identifier
func (p *HFWhisperProvider) Name() string {
	return "hf_whisper"
}

// Transcribe converts audio to text using HF Voice Service
func (p *HFWhisperProvider) Transcribe(ctx context.Context, req *TranscribeRequest) (*TranscribeResponse, error) {
	startTime := time.Now()

	// Prepare multipart form data
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add audio file
	part, err := writer.CreateFormFile("audio", "audio.wav")
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := part.Write(req.Audio); err != nil {
		return nil, fmt.Errorf("failed to write audio data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Determine language
	language := req.Language
	if language == "" {
		language = p.config.Language
	}

	// Build request URL
	url := fmt.Sprintf("%s/stt?language=%s", p.config.ServiceURL, language)

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request
	p.logger.Debug().Str("url", url).Str("language", language).Msg("Sending STT request to HF service")

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

	// Parse response
	var sttResp struct {
		Text            string  `json:"text"`
		Language        string  `json:"language"`
		Confidence      float64 `json:"confidence"`
		ProcessingTimeMs float64 `json:"processing_time_ms"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&sttResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	processingTime := time.Since(startTime)

	p.logger.Info().
		Str("text", sttResp.Text).
		Str("language", sttResp.Language).
		Float64("confidence", sttResp.Confidence).
		Dur("processing_time", processingTime).
		Msg("STT transcription complete")

	return &TranscribeResponse{
		Text:           sttResp.Text,
		Confidence:     sttResp.Confidence,
		Language:       sttResp.Language,
		Duration:       0, // Not provided by HF service
		ProcessingTime: processingTime,
		IsFinal:        true,
	}, nil
}

// TranscribeStream is not supported by HF Whisper (batch processing only)
func (p *HFWhisperProvider) TranscribeStream(ctx context.Context, audioStream <-chan []byte) (<-chan *TranscribeResponse, error) {
	return nil, fmt.Errorf("streaming not supported by HF Whisper provider")
}

// Health checks if the HF service is available
func (p *HFWhisperProvider) Health(ctx context.Context) error {
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
func (p *HFWhisperProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsStreaming:  false, // Batch processing only
		SupportsTimestamps: false, // Not exposed by HF service
		SupportedLanguages: []string{"en", "fr", "es", "zh", "ja", "ko", "auto"},
		MaxAudioLengthSec:  30,   // Lightning Whisper MLX optimized for short clips
		AvgLatencyMs:       500,  // ~500ms for short utterances (from POC)
		RequiresGPU:        false, // MPS acceleration, but optional
		IsLocal:            true,  // Runs locally via microservice
	}
}
