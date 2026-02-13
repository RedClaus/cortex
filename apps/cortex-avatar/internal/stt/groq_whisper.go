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

// GroqWhisperProvider implements STT using Groq's free Whisper API
type GroqWhisperProvider struct {
	apiKey string
	client *http.Client
	logger zerolog.Logger
	config *GroqWhisperConfig
}

// GroqWhisperConfig holds Groq Whisper configuration
type GroqWhisperConfig struct {
	APIKey   string        `json:"api_key"`
	Model    string        `json:"model"`    // "whisper-large-v3" or "whisper-large-v3-turbo"
	Language string        `json:"language"` // Optional language hint
	Timeout  time.Duration `json:"timeout"`
}

// DefaultGroqWhisperConfig returns sensible defaults
func DefaultGroqWhisperConfig() *GroqWhisperConfig {
	return &GroqWhisperConfig{
		Model:    "whisper-large-v3-turbo", // Faster, free tier
		Language: "",                        // Auto-detect
		Timeout:  30 * time.Second,
	}
}

// NewGroqWhisperProvider creates a new Groq Whisper provider
func NewGroqWhisperProvider(logger zerolog.Logger, config *GroqWhisperConfig) *GroqWhisperProvider {
	if config == nil {
		config = DefaultGroqWhisperConfig()
	}

	// Try to get API key from config, then environment
	apiKey := config.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("GROQ_API_KEY")
	}

	return &GroqWhisperProvider{
		apiKey: apiKey,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		logger: logger.With().Str("provider", "groq-whisper").Logger(),
		config: config,
	}
}

// Name returns the provider identifier
func (p *GroqWhisperProvider) Name() string {
	return "groq-whisper"
}

// SetAPIKey sets the API key
func (p *GroqWhisperProvider) SetAPIKey(apiKey string) {
	p.apiKey = apiKey
}

// IsAvailable returns true if the provider has a valid API key
func (p *GroqWhisperProvider) IsAvailable() bool {
	return p.apiKey != ""
}

// Transcribe sends audio to Groq's Whisper API
func (p *GroqWhisperProvider) Transcribe(ctx context.Context, req *TranscribeRequest) (*TranscribeResponse, error) {
	startTime := time.Now()

	if p.apiKey == "" {
		return nil, fmt.Errorf("Groq API key not configured. Get a free key at https://console.groq.com")
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

	// Request word timestamps
	if err := writer.WriteField("response_format", "verbose_json"); err != nil {
		return nil, fmt.Errorf("failed to write response_format field: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Create request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.groq.com/openai/v1/audio/transcriptions", &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request
	p.logger.Debug().Int("audioBytes", len(req.Audio)).Msg("Sending audio to Groq Whisper")
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
		p.logger.Error().Int("status", resp.StatusCode).Str("body", string(body)).Msg("Groq API error")
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	// Parse response (verbose_json format)
	var result struct {
		Text     string  `json:"text"`
		Language string  `json:"language"`
		Duration float64 `json:"duration"`
		Words    []struct {
			Word  string  `json:"word"`
			Start float64 `json:"start"`
			End   float64 `json:"end"`
		} `json:"words"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		// Try simple format
		var simple struct {
			Text string `json:"text"`
		}
		if err2 := json.Unmarshal(body, &simple); err2 == nil {
			result.Text = simple.Text
		} else {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
	}

	processingTime := time.Since(startTime)
	p.logger.Info().Str("text", result.Text).Dur("time", processingTime).Msg("Transcription complete")

	response := &TranscribeResponse{
		Text:           result.Text,
		Confidence:     0.95,
		Language:       result.Language,
		Duration:       time.Duration(result.Duration * float64(time.Second)),
		ProcessingTime: processingTime,
		IsFinal:        true,
	}

	// Convert words to our format
	for _, w := range result.Words {
		response.Words = append(response.Words, Word{
			Word:       w.Word,
			Start:      time.Duration(w.Start * float64(time.Second)),
			End:        time.Duration(w.End * float64(time.Second)),
			Confidence: 0.95,
		})
	}

	return response, nil
}

// createWAVHeader creates a WAV header for PCM data
func (p *GroqWhisperProvider) createWAVHeader(pcmData []byte, sampleRate, channels int) []byte {
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
	copy(header[0:4], "RIFF")
	header[4] = byte(fileSize & 0xff)
	header[5] = byte((fileSize >> 8) & 0xff)
	header[6] = byte((fileSize >> 16) & 0xff)
	header[7] = byte((fileSize >> 24) & 0xff)
	copy(header[8:12], "WAVE")
	copy(header[12:16], "fmt ")
	header[16] = 16
	header[20] = 1 // PCM
	header[22] = byte(channels)
	header[24] = byte(sampleRate & 0xff)
	header[25] = byte((sampleRate >> 8) & 0xff)
	header[26] = byte((sampleRate >> 16) & 0xff)
	header[27] = byte((sampleRate >> 24) & 0xff)
	header[28] = byte(byteRate & 0xff)
	header[29] = byte((byteRate >> 8) & 0xff)
	header[30] = byte((byteRate >> 16) & 0xff)
	header[31] = byte((byteRate >> 24) & 0xff)
	header[32] = byte(blockAlign)
	header[34] = byte(bitsPerSample)
	copy(header[36:40], "data")
	header[40] = byte(dataSize & 0xff)
	header[41] = byte((dataSize >> 8) & 0xff)
	header[42] = byte((dataSize >> 16) & 0xff)
	header[43] = byte((dataSize >> 24) & 0xff)

	return append(header, pcmData...)
}

// TranscribeStream handles streaming transcription (batched)
func (p *GroqWhisperProvider) TranscribeStream(ctx context.Context, audioStream <-chan []byte) (<-chan *TranscribeResponse, error) {
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
					if len(buffer) > 16000*2 {
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

				// Transcribe every ~2 seconds
				if len(buffer) > 16000*2*2 {
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

// Health checks if the provider is available
func (p *GroqWhisperProvider) Health(ctx context.Context) error {
	if p.apiKey == "" {
		return fmt.Errorf("Groq API key not configured")
	}
	return nil
}

// Capabilities returns provider capabilities
func (p *GroqWhisperProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsStreaming:  false,
		SupportsTimestamps: true,
		SupportedLanguages: []string{"en", "es", "fr", "de", "it", "pt", "zh", "ja", "ko", "ru", "ar"},
		MaxAudioLengthSec:  300,
		AvgLatencyMs:       500, // Groq is very fast
		RequiresGPU:        false,
		IsLocal:            false,
	}
}
