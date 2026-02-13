// Package voice provides voice-related types and utilities for Cortex.
// audio_enhancer.go provides audio enhancement client for the Voice Box server (CR-012-B).
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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// EnhanceMode specifies the audio enhancement mode.
type EnhanceMode string

const (
	// EnhanceModeNone disables enhancement (passthrough)
	EnhanceModeNone EnhanceMode = "none"

	// EnhanceModeDenoise applies noise reduction only
	EnhanceModeDenoise EnhanceMode = "denoise"

	// EnhanceModeFull applies full enhancement (denoise + normalization + clarity)
	EnhanceModeFull EnhanceMode = "full"

	// EnhanceModeAuto automatically selects mode based on audio analysis
	EnhanceModeAuto EnhanceMode = "auto"
)

// AudioEnhancerConfig holds configuration for the audio enhancer client.
type AudioEnhancerConfig struct {
	// Endpoint is the Voice Box audio enhance API endpoint
	Endpoint string

	// AnalyzeEndpoint is the Voice Box audio analyze API endpoint
	AnalyzeEndpoint string

	// HealthEndpoint is the Voice Box health check endpoint
	HealthEndpoint string

	// Timeout for enhancement requests
	Timeout time.Duration
}

// DefaultAudioEnhancerConfig returns production defaults for the audio enhancer.
func DefaultAudioEnhancerConfig() AudioEnhancerConfig {
	return AudioEnhancerConfig{
		Endpoint:        "http://127.0.0.1:8880/v1/audio/enhance",
		AnalyzeEndpoint: "http://127.0.0.1:8880/v1/audio/analyze",
		HealthEndpoint:  "http://127.0.0.1:8880/health",
		Timeout:         30 * time.Second,
	}
}

// AudioEnhancer provides audio enhancement functionality via Voice Box.
type AudioEnhancer struct {
	config     AudioEnhancerConfig
	httpClient *http.Client
}

// EnhanceResult holds the result of audio enhancement.
type EnhanceResult struct {
	// AudioPath is the path to the enhanced audio file
	AudioPath string `json:"audio_path"`

	// Mode is the enhancement mode that was applied
	Mode EnhanceMode `json:"mode"`

	// ProcessingTimeMs is the time taken to process in milliseconds
	ProcessingTimeMs int64 `json:"processing_time_ms"`

	// OriginalDuration is the duration of the original audio in seconds
	OriginalDuration float64 `json:"original_duration,omitempty"`
}

// AnalyzeResult holds the result of audio analysis.
type AnalyzeResult struct {
	// NoiseLevel is the detected noise level (0.0 to 1.0)
	NoiseLevel float64 `json:"noise_level"`

	// Recommendation is the recommended enhancement mode
	Recommendation EnhanceMode `json:"recommendation"`

	// Reason explains why this mode was recommended
	Reason string `json:"reason"`

	// Duration is the audio duration in seconds
	Duration float64 `json:"duration,omitempty"`

	// SampleRate is the audio sample rate
	SampleRate int `json:"sample_rate,omitempty"`
}

// NewAudioEnhancer creates a new audio enhancer with the given config.
func NewAudioEnhancer(config AudioEnhancerConfig) (*AudioEnhancer, error) {
	if config.Endpoint == "" {
		config = DefaultAudioEnhancerConfig()
	}

	return &AudioEnhancer{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}, nil
}

// checkHealth verifies Voice Box is running and responding.
func (e *AudioEnhancer) checkHealth(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", e.config.HealthEndpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create health request: %w", err)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("voice box unavailable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("voice box unhealthy: status %d", resp.StatusCode)
	}

	return nil
}

// EnhanceFile enhances audio from a file path.
func (e *AudioEnhancer) EnhanceFile(ctx context.Context, audioPath string, mode EnhanceMode) (*EnhanceResult, error) {
	start := time.Now()

	// Check Voice Box is running
	if err := e.checkHealth(ctx); err != nil {
		return nil, err
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

	log.Debug().
		Str("file", audioPath).
		Int64("size", info.Size()).
		Str("mode", string(mode)).
		Msg("enhancing audio file")

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

	// Add mode
	if mode == "" {
		mode = EnhanceModeAuto
	}
	if err := writer.WriteField("mode", string(mode)); err != nil {
		return nil, fmt.Errorf("failed to write mode field: %w", err)
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
		return nil, fmt.Errorf("enhancement request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("enhancement error (%d): %s", resp.StatusCode, string(respBody))
	}

	// Determine output path
	ext := filepath.Ext(audioPath)
	baseName := strings.TrimSuffix(audioPath, ext)
	outputPath := baseName + ".enhanced.wav"

	// Save response body to file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, resp.Body); err != nil {
		return nil, fmt.Errorf("failed to write output file: %w", err)
	}

	// Build result
	result := &EnhanceResult{
		AudioPath:        outputPath,
		Mode:             mode,
		ProcessingTimeMs: time.Since(start).Milliseconds(),
	}

	// Parse metadata from headers if available
	if durationStr := resp.Header.Get("X-Audio-Duration"); durationStr != "" {
		if duration, err := strconv.ParseFloat(durationStr, 64); err == nil {
			result.OriginalDuration = duration
		}
	}

	// Check if server returned the actual mode used (for auto mode)
	if actualMode := resp.Header.Get("X-Enhance-Mode"); actualMode != "" {
		result.Mode = EnhanceMode(actualMode)
	}

	log.Debug().
		Str("output", outputPath).
		Str("mode", string(result.Mode)).
		Int64("processing_ms", result.ProcessingTimeMs).
		Msg("audio enhancement complete")

	return result, nil
}

// AnalyzeFile analyzes audio to determine recommended enhancement.
func (e *AudioEnhancer) AnalyzeFile(ctx context.Context, audioPath string) (*AnalyzeResult, error) {
	// Check Voice Box is running
	if err := e.checkHealth(ctx); err != nil {
		return nil, err
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

	log.Debug().
		Str("file", audioPath).
		Int64("size", info.Size()).
		Msg("analyzing audio file")

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

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", e.config.AnalyzeEndpoint, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("analyze request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("analyze error (%d): %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var result AnalyzeResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	log.Debug().
		Float64("noise_level", result.NoiseLevel).
		Str("recommendation", string(result.Recommendation)).
		Str("reason", result.Reason).
		Msg("audio analysis complete")

	return &result, nil
}

// IsAvailable checks if the audio enhancer is available (Voice Box healthy).
func (e *AudioEnhancer) IsAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	return e.checkHealth(ctx) == nil
}

// ============================================================================
// Global AudioEnhancer Instance
// ============================================================================

var (
	globalAudioEnhancer     *AudioEnhancer
	globalAudioEnhancerOnce sync.Once
	globalAudioEnhancerErr  error
)

// GetAudioEnhancer returns the global audio enhancer instance, creating it if needed.
func GetAudioEnhancer() (*AudioEnhancer, error) {
	globalAudioEnhancerOnce.Do(func() {
		globalAudioEnhancer, globalAudioEnhancerErr = NewAudioEnhancer(DefaultAudioEnhancerConfig())
	})
	if globalAudioEnhancerErr != nil {
		return nil, globalAudioEnhancerErr
	}
	return globalAudioEnhancer, nil
}
