// Package voice provides voice processing capabilities for Cortex.
// sensevoice.go implements the SenseVoice STT client with emotion detection.
package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
)

// SenseVoiceClient implements STTBackend for the SenseVoice sidecar.
// Brain Alignment: SenseVoice provides voice emotion detection, enriching
// the Emotion Lobe with multimodal emotional signals.
type SenseVoiceClient struct {
	config     *STTBackendConfig
	httpClient *http.Client
}

// NewSenseVoiceClient creates a new SenseVoice STT client.
func NewSenseVoiceClient(config *STTBackendConfig) *SenseVoiceClient {
	if config == nil {
		config = DefaultSTTBackendConfig("sensevoice")
	}

	return &SenseVoiceClient{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Name returns "sensevoice".
func (c *SenseVoiceClient) Name() string {
	return "sensevoice"
}

// IsAvailable checks if the SenseVoice sidecar is running.
func (c *SenseVoiceClient) IsAvailable(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", c.config.Endpoint+"/health", nil)
	if err != nil {
		return false
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// Transcribe performs speech-to-text with optional emotion detection.
func (c *SenseVoiceClient) Transcribe(ctx context.Context, req *STTRequest) (*STTResult, error) {
	startTime := time.Now()

	// Determine endpoint based on emotion request
	endpoint := c.config.Endpoint + "/v1/audio/transcriptions"
	if req.IncludeEmotion && c.config.EnableEmotion {
		endpoint = c.config.Endpoint + "/v1/audio/transcriptions/emotion"
	}

	// Build multipart request
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add audio file
	if req.AudioPath != "" {
		if err := c.addAudioFile(writer, req.AudioPath); err != nil {
			return nil, fmt.Errorf("add audio file: %w", err)
		}
	} else if len(req.AudioData) > 0 {
		if err := c.addAudioData(writer, req.AudioData, req.AudioFormat); err != nil {
			return nil, fmt.Errorf("add audio data: %w", err)
		}
	} else {
		return nil, fmt.Errorf("no audio provided")
	}

	// Add language parameter
	if req.Language != "" {
		if err := writer.WriteField("language", req.Language); err != nil {
			return nil, fmt.Errorf("write language field: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close writer: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, &body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sensevoice error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var result *STTResult
	if req.IncludeEmotion && c.config.EnableEmotion {
		result, err = c.parseEmotionResponse(resp.Body)
	} else {
		result, err = c.parseStandardResponse(resp.Body)
	}

	if err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	result.Latency = time.Since(startTime)
	result.Backend = "sensevoice"
	result.IsFinal = true

	log.Debug().
		Str("backend", "sensevoice").
		Str("text", truncateString(result.Text, 50)).
		Dur("latency", result.Latency).
		Msg("transcription complete")

	return result, nil
}

// SupportsEmotion returns true - SenseVoice provides voice emotion detection.
func (c *SenseVoiceClient) SupportsEmotion() bool {
	return c.config.EnableEmotion
}

// SupportsStreaming returns false - SenseVoice does not support streaming.
func (c *SenseVoiceClient) SupportsStreaming() bool {
	return false
}

// addAudioFile adds an audio file to the multipart request.
func (c *SenseVoiceClient) addAudioFile(writer *multipart.Writer, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	filename := filepath.Base(path)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return fmt.Errorf("create form file: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("copy file: %w", err)
	}

	return nil
}

// addAudioData adds raw audio data to the multipart request.
func (c *SenseVoiceClient) addAudioData(writer *multipart.Writer, data []byte, format string) error {
	if format == "" {
		format = "wav"
	}

	filename := "audio." + format
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return fmt.Errorf("create form file: %w", err)
	}

	if _, err := part.Write(data); err != nil {
		return fmt.Errorf("write data: %w", err)
	}

	return nil
}

// senseVoiceStandardResponse represents the standard transcription response.
type senseVoiceStandardResponse struct {
	Text     string `json:"text"`
	Language string `json:"language"`
	Backend  string `json:"backend"`
}

// senseVoiceEmotionResponse represents the emotion transcription response.
type senseVoiceEmotionResponse struct {
	Text     string `json:"text"`
	Language string `json:"language"`
	Backend  string `json:"backend"`
	Emotion  *struct {
		Primary    string             `json:"primary"`
		Confidence float64            `json:"confidence"`
		All        map[string]float64 `json:"all"`
	} `json:"emotion"`
	Events []struct {
		Type       string  `json:"type"`
		Confidence float64 `json:"confidence"`
	} `json:"events"`
}

// parseStandardResponse parses the standard transcription response.
func (c *SenseVoiceClient) parseStandardResponse(body io.Reader) (*STTResult, error) {
	var resp senseVoiceStandardResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &STTResult{
		Text:     resp.Text,
		Language: resp.Language,
		Backend:  resp.Backend,
	}, nil
}

// parseEmotionResponse parses the emotion transcription response.
func (c *SenseVoiceClient) parseEmotionResponse(body io.Reader) (*STTResult, error) {
	var resp senseVoiceEmotionResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	result := &STTResult{
		Text:     resp.Text,
		Language: resp.Language,
		Backend:  resp.Backend,
	}

	// Parse emotion data
	if resp.Emotion != nil {
		result.Emotion = &VoiceEmotionData{
			Primary:    resp.Emotion.Primary,
			Confidence: resp.Emotion.Confidence,
			All:        resp.Emotion.All,
		}
	}

	// Parse audio events
	if len(resp.Events) > 0 {
		result.AudioEvents = make([]AudioEvent, len(resp.Events))
		for i, ev := range resp.Events {
			result.AudioEvents[i] = AudioEvent{
				Type:       ev.Type,
				Confidence: ev.Confidence,
			}
		}
	}

	return result, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Health and Status
// ─────────────────────────────────────────────────────────────────────────────

// SenseVoiceHealth represents the health check response from SenseVoice.
type SenseVoiceHealth struct {
	Status        string   `json:"status"`
	Version       string   `json:"version"`
	Platform      string   `json:"platform"`
	Device        string   `json:"device"`
	EmotionLabels []string `json:"emotion_labels"`
	Model         struct {
		Name     string   `json:"name"`
		Loaded   bool     `json:"loaded"`
		Features []string `json:"features"`
	} `json:"model"`
}

// GetHealth fetches detailed health information from SenseVoice.
func (c *SenseVoiceClient) GetHealth(ctx context.Context) (*SenseVoiceHealth, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.config.Endpoint+"/health", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	var health SenseVoiceHealth
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return nil, fmt.Errorf("decode health response: %w", err)
	}

	return &health, nil
}

// Config returns the client configuration.
func (c *SenseVoiceClient) Config() *STTBackendConfig {
	return c.config
}
