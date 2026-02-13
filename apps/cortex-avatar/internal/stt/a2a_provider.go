package stt

import (
	"context"
	"encoding/base64"
	"time"

	"github.com/normanking/cortexavatar/internal/a2a"
	"github.com/rs/zerolog"
)

// A2AProvider implements STT by delegating to CortexBrain via A2A protocol.
// This sends audio as base64 in the message and receives transcription in the response.
type A2AProvider struct {
	client *a2a.Client
	logger zerolog.Logger
	config *A2AConfig
}

// A2AConfig holds A2A STT provider configuration
type A2AConfig struct {
	Timeout   time.Duration `json:"timeout"`
	ModelSize string        `json:"model_size"`
	Language  string        `json:"language"`
}

// DefaultA2AConfig returns sensible defaults
func DefaultA2AConfig() *A2AConfig {
	return &A2AConfig{
		Timeout:   30 * time.Second,
		ModelSize: "base",
		Language:  "en",
	}
}

// NewA2AProvider creates a new A2A-based STT provider
func NewA2AProvider(client *a2a.Client, logger zerolog.Logger, config *A2AConfig) *A2AProvider {
	if config == nil {
		config = DefaultA2AConfig()
	}

	return &A2AProvider{
		client: client,
		logger: logger.With().Str("provider", "a2a-stt").Logger(),
		config: config,
	}
}

// Name returns the provider identifier
func (p *A2AProvider) Name() string {
	return "a2a"
}

// Transcribe sends audio to CortexBrain for transcription
func (p *A2AProvider) Transcribe(ctx context.Context, req *TranscribeRequest) (*TranscribeResponse, error) {
	startTime := time.Now()

	// Validate
	if len(req.Audio) == 0 {
		return nil, ErrAudioTooShort
	}

	// Encode audio as base64
	audioBase64 := base64.StdEncoding.EncodeToString(req.Audio)

	// Determine MIME type
	mimeType := "audio/wav"
	switch req.Format {
	case "pcm":
		mimeType = "audio/raw"
	case "webm":
		mimeType = "audio/webm"
	case "opus":
		mimeType = "audio/opus"
	case "mp3":
		mimeType = "audio/mp3"
	}

	// Create a special transcribe request message
	// CortexBrain's A2A handler recognizes [TRANSCRIBE] prefix
	messageText := "[TRANSCRIBE audio=" + mimeType + "] " + audioBase64

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, p.config.Timeout)
	defer cancel()

	// Send to CortexBrain
	response, err := p.client.SendMessage(ctx, messageText)
	if err != nil {
		p.logger.Error().Err(err).Msg("Failed to send audio to CortexBrain")
		return &TranscribeResponse{
			Error:   err.Error(),
			IsFinal: true,
		}, err
	}

	// Extract transcription from response
	transcript := ""
	if response != nil {
		transcript = response.ExtractText()
	}

	processingTime := time.Since(startTime)

	return &TranscribeResponse{
		Text:           transcript,
		Confidence:     0.85, // A2A doesn't provide confidence
		Language:       req.Language,
		ProcessingTime: processingTime,
		IsFinal:        true,
	}, nil
}

// TranscribeStream handles streaming transcription
func (p *A2AProvider) TranscribeStream(ctx context.Context, audioStream <-chan []byte) (<-chan *TranscribeResponse, error) {
	// A2A doesn't support true streaming STT, but we can batch
	results := make(chan *TranscribeResponse, 10)

	go func() {
		defer close(results)

		var buffer []byte
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case audio, ok := <-audioStream:
				if !ok {
					// Stream ended, transcribe remaining buffer
					if len(buffer) > 0 {
						resp, err := p.Transcribe(ctx, &TranscribeRequest{
							Audio:  buffer,
							Format: "pcm",
						})
						if err == nil {
							resp.IsFinal = true
							results <- resp
						}
					}
					return
				}
				buffer = append(buffer, audio...)

			case <-ticker.C:
				// Transcribe accumulated audio periodically
				if len(buffer) > 16000*2 { // At least 1 second of 16kHz mono
					resp, err := p.Transcribe(ctx, &TranscribeRequest{
						Audio:  buffer,
						Format: "pcm",
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

// Health checks if CortexBrain is available for STT
func (p *A2AProvider) Health(ctx context.Context) error {
	// Check if A2A client is connected
	if !p.client.IsConnected() {
		return ErrProviderUnavailable
	}
	return nil
}

// Capabilities returns what this provider supports
func (p *A2AProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsStreaming:  false, // Batched only
		SupportsTimestamps: false, // Depends on CortexBrain
		SupportedLanguages: []string{"en", "es", "fr", "de", "it", "pt", "zh", "ja", "ko"},
		MaxAudioLengthSec:  300, // 5 minutes
		AvgLatencyMs:       500, // Network latency + processing
		RequiresGPU:        false,
		IsLocal:            false,
	}
}
