// Package voice provides speech-to-text and text-to-speech functionality.
// stt_engine.go implements the STT client for the Voice Box server (CR-012-A).
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

// EnhanceMode specifies the audio enhancement mode for STT preprocessing.
type EnhanceMode string

const (
	// EnhanceModeNone disables enhancement (passthrough)
	EnhanceModeNone EnhanceMode = "none"

	// EnhanceModeDenoise applies noise reduction only
	EnhanceModeDenoise EnhanceMode = "denoise"

	// EnhanceModeFull applies full enhancement (denoise + fidelity restoration)
	EnhanceModeFull EnhanceMode = "full"

	// EnhanceModeAuto automatically selects mode based on audio analysis
	EnhanceModeAuto EnhanceMode = "auto"
)

// STTConfig holds STT engine configuration.
type STTConfig struct {
	// Endpoint is the Voice Box STT API endpoint
	Endpoint string

	// Model is the Whisper model to use (e.g., "whisper-large-v3-turbo")
	Model string

	// Language is the expected language (empty for auto-detect)
	Language string

	// Timeout for transcription requests
	Timeout time.Duration

	// EnhanceMode specifies audio enhancement preprocessing (CR-012-B)
	// Options: "none", "denoise", "full", "auto"
	EnhanceMode EnhanceMode
}

// DefaultSTTConfig returns production defaults for STT.
func DefaultSTTConfig() STTConfig {
	return STTConfig{
		Endpoint:    "http://127.0.0.1:8880/v1/audio/transcriptions",
		Model:       "whisper-large-v3-turbo",
		Language:    "", // Auto-detect
		Timeout:     60 * time.Second,
		EnhanceMode: EnhanceModeNone, // Default: no enhancement for speed
	}
}

// TranscriptionResult holds the result of speech-to-text transcription.
type TranscriptionResult struct {
	// Text is the transcribed text
	Text string `json:"text"`

	// Language is the detected language (if auto-detected)
	Language string `json:"language,omitempty"`

	// Backend is the STT backend used (mlx, faster_whisper, etc.)
	Backend string `json:"backend,omitempty"`

	// Duration is the audio duration in seconds
	Duration float64 `json:"duration,omitempty"`

	// Latency is the transcription time in milliseconds
	Latency int64 `json:"latency_ms,omitempty"`

	// Enhanced indicates if audio was preprocessed with enhancement (CR-012-B)
	Enhanced bool `json:"enhanced,omitempty"`
}

// STTEngine provides speech-to-text functionality via Voice Box.
type STTEngine struct {
	config     STTConfig
	httpClient *http.Client
	launcher   *VoiceBoxLauncher
}

// NewSTTEngine creates a new STT engine with the given config.
func NewSTTEngine(config STTConfig) (*STTEngine, error) {
	// Get the global VoiceBox launcher
	launcher := GetVoiceBoxLauncher()

	if config.Endpoint == "" {
		config.Endpoint = launcher.Endpoint() + "/v1/audio/transcriptions"
	}

	return &STTEngine{
		config:   config,
		launcher: launcher,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}, nil
}

// TranscribeFile transcribes audio from a file path.
func (e *STTEngine) TranscribeFile(ctx context.Context, audioPath string) (*TranscriptionResult, error) {
	start := time.Now()

	// Ensure Voice Box is running
	if err := e.launcher.EnsureRunning(ctx); err != nil {
		return nil, fmt.Errorf("voice box unavailable: %w", err)
	}

	// Open file
	file, err := os.Open(audioPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open audio file: %w", err)
	}
	defer file.Close()

	// Get file info for logging
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat audio file: %w", err)
	}

	// Determine enhance mode
	enhanceMode := e.config.EnhanceMode
	if enhanceMode == "" {
		enhanceMode = EnhanceModeNone
	}

	log.Debug().
		Str("file", audioPath).
		Int64("size", info.Size()).
		Str("model", e.config.Model).
		Str("enhance", string(enhanceMode)).
		Msg("transcribing audio file")

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add file
	part, err := writer.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("failed to copy file data: %w", err)
	}

	// Add model
	if err := writer.WriteField("model", e.config.Model); err != nil {
		return nil, fmt.Errorf("failed to write model field: %w", err)
	}

	// Add language if specified
	if e.config.Language != "" {
		if err := writer.WriteField("language", e.config.Language); err != nil {
			return nil, fmt.Errorf("failed to write language field: %w", err)
		}
	}

	// Add response format
	if err := writer.WriteField("response_format", "json"); err != nil {
		return nil, fmt.Errorf("failed to write response_format field: %w", err)
	}

	// Add enhance mode (CR-012-B)
	if err := writer.WriteField("enhance", string(enhanceMode)); err != nil {
		return nil, fmt.Errorf("failed to write enhance field: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", e.config.Endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("transcription request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("STT error (%d): %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var result TranscriptionResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Add latency
	result.Latency = time.Since(start).Milliseconds()

	log.Debug().
		Str("text", truncateString(result.Text, 50)).
		Str("language", result.Language).
		Str("backend", result.Backend).
		Int64("latency_ms", result.Latency).
		Msg("transcription complete")

	return &result, nil
}

// TranscribeBytes transcribes audio from a byte slice.
func (e *STTEngine) TranscribeBytes(ctx context.Context, audio []byte, format string) (*TranscriptionResult, error) {
	start := time.Now()

	// Ensure Voice Box is running
	if err := e.launcher.EnsureRunning(ctx); err != nil {
		return nil, fmt.Errorf("voice box unavailable: %w", err)
	}

	// Determine file extension
	ext := format
	if ext == "" {
		ext = "wav"
	}
	if ext[0] != '.' {
		ext = "." + ext
	}

	// Determine enhance mode
	enhanceMode := e.config.EnhanceMode
	if enhanceMode == "" {
		enhanceMode = EnhanceModeNone
	}

	log.Debug().
		Int("size", len(audio)).
		Str("format", ext).
		Str("model", e.config.Model).
		Str("enhance", string(enhanceMode)).
		Msg("transcribing audio bytes")

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add file
	part, err := writer.CreateFormFile("file", "audio"+ext)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := part.Write(audio); err != nil {
		return nil, fmt.Errorf("failed to write audio data: %w", err)
	}

	// Add model
	if err := writer.WriteField("model", e.config.Model); err != nil {
		return nil, fmt.Errorf("failed to write model field: %w", err)
	}

	// Add language if specified
	if e.config.Language != "" {
		if err := writer.WriteField("language", e.config.Language); err != nil {
			return nil, fmt.Errorf("failed to write language field: %w", err)
		}
	}

	// Add response format
	if err := writer.WriteField("response_format", "json"); err != nil {
		return nil, fmt.Errorf("failed to write response_format field: %w", err)
	}

	// Add enhance mode (CR-012-B)
	if err := writer.WriteField("enhance", string(enhanceMode)); err != nil {
		return nil, fmt.Errorf("failed to write enhance field: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", e.config.Endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("transcription request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("STT error (%d): %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var result TranscriptionResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Add latency
	result.Latency = time.Since(start).Milliseconds()

	log.Debug().
		Str("text", truncateString(result.Text, 50)).
		Str("language", result.Language).
		Str("backend", result.Backend).
		Int64("latency_ms", result.Latency).
		Msg("transcription complete")

	return &result, nil
}

// IsAvailable checks if STT is available (Voice Box installed and healthy).
func (e *STTEngine) IsAvailable() bool {
	return e.launcher.IsInstalled() && e.launcher.IsHealthy()
}

// GetBackendInfo returns information about the STT backend being used.
func (e *STTEngine) GetBackendInfo(ctx context.Context) (map[string]interface{}, error) {
	// Ensure Voice Box is running
	if err := e.launcher.EnsureRunning(ctx); err != nil {
		return nil, fmt.Errorf("voice box unavailable: %w", err)
	}

	// Get health info
	healthEndpoint := e.launcher.Endpoint() + "/health"
	resp, err := e.httpClient.Get(healthEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to get health info: %w", err)
	}
	defer resp.Body.Close()

	var info map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to parse health info: %w", err)
	}

	return info, nil
}

// truncateString truncates a string to maxLen characters with ellipsis.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ============================================================================
// Global STT Engine Instance
// ============================================================================

var (
	globalSTTEngine *STTEngine
)

// GetSTTEngine returns the global STT engine instance, creating it if needed.
func GetSTTEngine() (*STTEngine, error) {
	if globalSTTEngine == nil {
		var err error
		globalSTTEngine, err = NewSTTEngine(DefaultSTTConfig())
		if err != nil {
			return nil, err
		}
	}
	return globalSTTEngine, nil
}
