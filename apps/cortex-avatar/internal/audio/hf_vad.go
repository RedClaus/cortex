package audio

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

// HFVADClient calls the HF Voice Service for voice activity detection
type HFVADClient struct {
	serviceURL string
	httpClient *http.Client
	logger     zerolog.Logger
}

// NewHFVADClient creates a new HF VAD client
func NewHFVADClient(serviceURL string, logger zerolog.Logger) *HFVADClient {
	if serviceURL == "" {
		serviceURL = "http://localhost:8899"
	}

	return &HFVADClient{
		serviceURL: serviceURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger.With().Str("component", "hf_vad").Logger(),
	}
}

// DetectSpeech sends audio to HF service for VAD analysis
func (c *HFVADClient) DetectSpeech(ctx context.Context, audioData []byte) (*VADResult, error) {
	// Prepare multipart form data
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add audio file
	part, err := writer.CreateFormFile("audio", "audio.wav")
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := part.Write(audioData); err != nil {
		return nil, fmt.Errorf("failed to write audio data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Build request URL
	url := fmt.Sprintf("%s/vad", c.serviceURL)

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request
	c.logger.Debug().Str("url", url).Msg("Sending VAD request to HF service")

	resp, err := c.httpClient.Do(httpReq)
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
	var vadResp struct {
		HasSpeech        bool    `json:"has_speech"`
		Confidence       float64 `json:"confidence"`
		ProcessingTimeMs float64 `json:"processing_time_ms"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&vadResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Debug().
		Bool("has_speech", vadResp.HasSpeech).
		Float64("confidence", vadResp.Confidence).
		Float64("processing_ms", vadResp.ProcessingTimeMs).
		Msg("VAD result received")

	return &VADResult{
		IsSpeech:   vadResp.HasSpeech,
		Confidence: vadResp.Confidence,
		RMS:        0, // Not provided by HF service
	}, nil
}

// Health checks if the HF service is available
func (c *HFVADClient) Health(ctx context.Context) error {
	url := fmt.Sprintf("%s/health", c.serviceURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HF service unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HF service unhealthy (status %d)", resp.StatusCode)
	}

	c.logger.Debug().Msg("HF service health check passed")
	return nil
}
