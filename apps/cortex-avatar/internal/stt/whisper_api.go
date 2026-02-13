package stt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog"
)

// WhisperAPIProvider implements STT using OpenAI's Whisper API
type WhisperAPIProvider struct {
	apiKey  string
	client  *http.Client
	logger  zerolog.Logger
	config  *WhisperAPIConfig
}

// WhisperAPIConfig holds Whisper API configuration
type WhisperAPIConfig struct {
	APIKey   string        `json:"api_key"`
	Model    string        `json:"model"`    // "whisper-1"
	Language string        `json:"language"` // Optional language hint
	Timeout  time.Duration `json:"timeout"`
}

// DefaultWhisperAPIConfig returns sensible defaults
func DefaultWhisperAPIConfig() *WhisperAPIConfig {
	return &WhisperAPIConfig{
		Model:    "whisper-1",
		Language: "", // Auto-detect
		Timeout:  30 * time.Second,
	}
}

// NewWhisperAPIProvider creates a new OpenAI Whisper API provider
func NewWhisperAPIProvider(logger zerolog.Logger, config *WhisperAPIConfig) *WhisperAPIProvider {
	if config == nil {
		config = DefaultWhisperAPIConfig()
	}

	// Try to get API key from config, then environment
	apiKey := config.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	return &WhisperAPIProvider{
		apiKey: apiKey,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		logger: logger.With().Str("provider", "whisper-api").Logger(),
		config: config,
	}
}

// Name returns the provider identifier
func (p *WhisperAPIProvider) Name() string {
	return "whisper-api"
}

// SetAPIKey sets the API key (for runtime configuration)
func (p *WhisperAPIProvider) SetAPIKey(apiKey string) {
	p.apiKey = apiKey
}

// Transcribe sends audio to OpenAI Whisper API for transcription
func (p *WhisperAPIProvider) Transcribe(ctx context.Context, req *TranscribeRequest) (*TranscribeResponse, error) {
	startTime := time.Now()

	if p.apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key not configured")
	}

	if len(req.Audio) == 0 {
		return nil, ErrAudioTooShort
	}

	// Create WAV file from PCM data
	wavData := p.createWAVHeader(req.Audio, req.SampleRate, req.Channels)

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add file
	part, err := writer.CreateFormFile("file", "audio.wav")
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := part.Write(wavData); err != nil {
		return nil, fmt.Errorf("failed to write audio data: %w", err)
	}

	// Add model
	if err := writer.WriteField("model", p.config.Model); err != nil {
		return nil, fmt.Errorf("failed to write model field: %w", err)
	}

	// Add language hint if specified
	if p.config.Language != "" {
		if err := writer.WriteField("language", p.config.Language); err != nil {
			return nil, fmt.Errorf("failed to write language field: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Create request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/audio/transcriptions", &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		p.logger.Error().Int("status", resp.StatusCode).Str("body", string(body)).Msg("Whisper API error")
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	// Parse response
	var result struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	processingTime := time.Since(startTime)
	p.logger.Info().Str("text", result.Text).Dur("time", processingTime).Msg("Transcription complete")

	return &TranscribeResponse{
		Text:           result.Text,
		Confidence:     0.95, // Whisper doesn't provide confidence
		Language:       req.Language,
		ProcessingTime: processingTime,
		IsFinal:        true,
	}, nil
}

// createWAVHeader creates a WAV header for PCM data
func (p *WhisperAPIProvider) createWAVHeader(pcmData []byte, sampleRate, channels int) []byte {
	if sampleRate == 0 {
		sampleRate = 16000
	}
	if channels == 0 {
		channels = 1
	}

	bitsPerSample := 16
	byteRate := sampleRate * channels * bitsPerSample / 8
	blockAlign := channels * bitsPerSample / 8
	dataSize := len(pcmData)
	fileSize := 36 + dataSize

	header := make([]byte, 44)
	// RIFF header
	copy(header[0:4], "RIFF")
	header[4] = byte(fileSize & 0xff)
	header[5] = byte((fileSize >> 8) & 0xff)
	header[6] = byte((fileSize >> 16) & 0xff)
	header[7] = byte((fileSize >> 24) & 0xff)
	copy(header[8:12], "WAVE")

	// fmt subchunk
	copy(header[12:16], "fmt ")
	header[16] = 16 // Subchunk1Size
	header[17] = 0
	header[18] = 0
	header[19] = 0
	header[20] = 1 // AudioFormat (PCM)
	header[21] = 0
	header[22] = byte(channels)
	header[23] = 0
	header[24] = byte(sampleRate & 0xff)
	header[25] = byte((sampleRate >> 8) & 0xff)
	header[26] = byte((sampleRate >> 16) & 0xff)
	header[27] = byte((sampleRate >> 24) & 0xff)
	header[28] = byte(byteRate & 0xff)
	header[29] = byte((byteRate >> 8) & 0xff)
	header[30] = byte((byteRate >> 16) & 0xff)
	header[31] = byte((byteRate >> 24) & 0xff)
	header[32] = byte(blockAlign)
	header[33] = 0
	header[34] = byte(bitsPerSample)
	header[35] = 0

	// data subchunk
	copy(header[36:40], "data")
	header[40] = byte(dataSize & 0xff)
	header[41] = byte((dataSize >> 8) & 0xff)
	header[42] = byte((dataSize >> 16) & 0xff)
	header[43] = byte((dataSize >> 24) & 0xff)

	return append(header, pcmData...)
}

// TranscribeStream handles streaming transcription (batched for Whisper API)
func (p *WhisperAPIProvider) TranscribeStream(ctx context.Context, audioStream <-chan []byte) (<-chan *TranscribeResponse, error) {
	results := make(chan *TranscribeResponse, 10)

	go func() {
		defer close(results)

		var buffer []byte
		for {
			select {
			case <-ctx.Done():
				return
			case audio, ok := <-audioStream:
				if !ok {
					// Stream ended, transcribe remaining
					if len(buffer) > 16000*2 { // At least 1 second
						resp, err := p.Transcribe(ctx, &TranscribeRequest{
							Audio:      buffer,
							Format:     "pcm",
							SampleRate: 16000,
							Channels:   1,
						})
						if err == nil {
							resp.IsFinal = true
							results <- resp
						}
					}
					return
				}
				buffer = append(buffer, audio...)

				// Transcribe every ~3 seconds of audio
				if len(buffer) > 16000*2*3 {
					resp, err := p.Transcribe(ctx, &TranscribeRequest{
						Audio:      buffer,
						Format:     "pcm",
						SampleRate: 16000,
						Channels:   1,
					})
					if err == nil {
						resp.IsFinal = false
						results <- resp
					}
					buffer = buffer[:0]
				}
			}
		}
	}()

	return results, nil
}

// Health checks if the API is available
func (p *WhisperAPIProvider) Health(ctx context.Context) error {
	if p.apiKey == "" {
		return fmt.Errorf("OpenAI API key not configured")
	}
	return nil
}

// Capabilities returns provider capabilities
func (p *WhisperAPIProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsStreaming:  false, // Batched only
		SupportsTimestamps: true,
		SupportedLanguages: []string{"en", "es", "fr", "de", "it", "pt", "zh", "ja", "ko", "ru", "ar"},
		MaxAudioLengthSec:  300, // 5 minutes
		AvgLatencyMs:       1000,
		RequiresGPU:        false,
		IsLocal:            false,
	}
}
