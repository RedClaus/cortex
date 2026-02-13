// Package xtts implements the voice.Provider interface for Coqui XTTS v2.
// XTTS is the "Smart Lane" TTS engine with voice cloning and high-quality output.
package xtts

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
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/normanking/cortex/internal/voice"
)

// Provider implements voice.Provider for Coqui XTTS v2.
type Provider struct {
	config       Config
	httpClient   *http.Client
	clonedVoices map[string]*ClonedVoice
}

// Config contains XTTS-specific configuration.
type Config struct {
	BaseURL         string        // Default: http://localhost:5002
	DefaultVoice    string        // Default: "default"
	ClonedVoicesDir string        // Default: /data/cloned_voices
	Timeout         time.Duration // Default: 30s
	MaxTextLength   int           // Default: 5000
	GPUEnabled      bool          // Whether GPU acceleration is available
}

// ClonedVoice represents a voice cloned from a reference audio file.
type ClonedVoice struct {
	ID            string
	Name          string
	ReferenceFile string
	Language      string
	CreatedAt     time.Time
}

// NewProvider creates a new XTTS provider.
func NewProvider(config Config) *Provider {
	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:5002"
	}
	if config.DefaultVoice == "" {
		config.DefaultVoice = "default"
	}
	if config.ClonedVoicesDir == "" {
		config.ClonedVoicesDir = "/data/cloned_voices"
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxTextLength == 0 {
		config.MaxTextLength = 5000
	}

	return &Provider{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		clonedVoices: make(map[string]*ClonedVoice),
	}
}

// Name returns the provider identifier.
func (p *Provider) Name() string {
	return "xtts"
}

// XTTS API types
type xttsRequest struct {
	Text        string `json:"text"`
	Language    string `json:"language"`
	SpeakerWav  string `json:"speaker_wav,omitempty"`
	Speed       float64 `json:"speed,omitempty"`
}

type xttsResponse struct {
	Audio       []byte `json:"audio"`
	SampleRate  int    `json:"sample_rate"`
	Duration    float64 `json:"duration"`
	ProcessedMs int64  `json:"processed_ms"`
}

type xttsHealthResponse struct {
	Status  string `json:"status"`
	GPU     bool   `json:"gpu"`
	Version string `json:"version"`
}

// Synthesize sends a synthesis request to XTTS.
func (p *Provider) Synthesize(ctx context.Context, req *voice.SynthesizeRequest) (*voice.SynthesizeResponse, error) {
	start := time.Now()

	if err := voice.ValidateRequest(req); err != nil {
		return nil, err
	}

	if len(req.Text) > p.config.MaxTextLength {
		return nil, voice.ErrTextTooLong
	}

	// Resolve speaker file
	speakerFile, err := p.getSpeakerFile(req.VoiceID, req.CloneFromFile)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve speaker file: %w", err)
	}

	// Determine language (default to English)
	language := "en"
	if req.VoiceID != "" && req.VoiceID != "default" {
		// Extract language from voice ID or cloned voice metadata
		if cloned, ok := p.clonedVoices[req.VoiceID]; ok {
			if cloned.Language != "" {
				language = cloned.Language
			}
		}
	}

	// Build multipart request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add text field
	if err := writer.WriteField("text", req.Text); err != nil {
		return nil, fmt.Errorf("failed to write text field: %w", err)
	}

	// Add language field
	if err := writer.WriteField("language", language); err != nil {
		return nil, fmt.Errorf("failed to write language field: %w", err)
	}

	// Add speed field if specified
	if req.Speed != 0 {
		if err := writer.WriteField("speed", fmt.Sprintf("%.2f", req.Speed)); err != nil {
			return nil, fmt.Errorf("failed to write speed field: %w", err)
		}
	}

	// Add speaker_wav file if specified
	if speakerFile != "" {
		file, err := os.Open(speakerFile)
		if err != nil {
			return nil, fmt.Errorf("failed to open speaker file: %w", err)
		}
		defer file.Close()

		part, err := writer.CreateFormFile("speaker_wav", filepath.Base(speakerFile))
		if err != nil {
			return nil, fmt.Errorf("failed to create form file: %w", err)
		}

		if _, err := io.Copy(part, file); err != nil {
			return nil, fmt.Errorf("failed to copy speaker file: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Send request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.config.BaseURL+"/api/tts", body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, voice.ErrTimeout
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("xtts returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Read audio data
	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// XTTS returns WAV by default
	format := voice.FormatWAV
	if req.Format != "" {
		format = req.Format
	}

	sampleRate := 22050 // XTTS default
	if req.SampleRate != 0 {
		sampleRate = req.SampleRate
	}

	return &voice.SynthesizeResponse{
		Audio:       audioData,
		Format:      format,
		Duration:    0, // Calculate from audio data if needed
		SampleRate:  sampleRate,
		ProcessedMs: time.Since(start).Milliseconds(),
		VoiceID:     req.VoiceID,
		Provider:    p.Name(),
	}, nil
}

// Stream sends a streaming synthesis request to XTTS.
func (p *Provider) Stream(ctx context.Context, req *voice.SynthesizeRequest) (voice.AudioStream, error) {
	if err := voice.ValidateRequest(req); err != nil {
		return nil, err
	}

	if len(req.Text) > p.config.MaxTextLength {
		return nil, voice.ErrTextTooLong
	}

	// Resolve speaker file
	speakerFile, err := p.getSpeakerFile(req.VoiceID, req.CloneFromFile)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve speaker file: %w", err)
	}

	// Determine language
	language := "en"
	if cloned, ok := p.clonedVoices[req.VoiceID]; ok {
		if cloned.Language != "" {
			language = cloned.Language
		}
	}

	// Build multipart request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if err := writer.WriteField("text", req.Text); err != nil {
		return nil, fmt.Errorf("failed to write text field: %w", err)
	}
	if err := writer.WriteField("language", language); err != nil {
		return nil, fmt.Errorf("failed to write language field: %w", err)
	}
	if req.Speed != 0 {
		if err := writer.WriteField("speed", fmt.Sprintf("%.2f", req.Speed)); err != nil {
			return nil, fmt.Errorf("failed to write speed field: %w", err)
		}
	}

	if speakerFile != "" {
		file, err := os.Open(speakerFile)
		if err != nil {
			return nil, fmt.Errorf("failed to open speaker file: %w", err)
		}
		defer file.Close()

		part, err := writer.CreateFormFile("speaker_wav", filepath.Base(speakerFile))
		if err != nil {
			return nil, fmt.Errorf("failed to create form file: %w", err)
		}
		if _, err := io.Copy(part, file); err != nil {
			return nil, fmt.Errorf("failed to copy speaker file: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Send streaming request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.config.BaseURL+"/api/tts-stream", body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("Accept", "audio/wav")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, voice.ErrTimeout
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("xtts returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	format := voice.FormatWAV
	if req.Format != "" {
		format = req.Format
	}

	sampleRate := 22050
	if req.SampleRate != 0 {
		sampleRate = req.SampleRate
	}

	return &audioStream{
		ReadCloser: resp.Body,
		format:     format,
		sampleRate: sampleRate,
	}, nil
}

// ListVoices returns available voices from XTTS.
func (p *Provider) ListVoices(ctx context.Context) ([]voice.Voice, error) {
	voices := []voice.Voice{
		{
			ID:       "default",
			Name:     "Default",
			Language: "en",
			Gender:   voice.GenderNeutral,
			IsCloned: false,
		},
	}

	// Add cloned voices
	for id, cloned := range p.clonedVoices {
		voices = append(voices, voice.Voice{
			ID:       id,
			Name:     cloned.Name,
			Language: cloned.Language,
			Gender:   voice.GenderUnknown,
			IsCloned: true,
			Metadata: map[string]string{
				"reference_file": cloned.ReferenceFile,
				"created_at":     cloned.CreatedAt.Format(time.RFC3339),
			},
		})
	}

	return voices, nil
}

// Health checks if XTTS is available.
func (p *Provider) Health(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", p.config.BaseURL+"/api/health", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return voice.ErrTimeout
		}
		return voice.ErrProviderUnavailable
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status %d", resp.StatusCode)
	}

	var healthResp xttsHealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&healthResp); err != nil {
		return fmt.Errorf("failed to decode health response: %w", err)
	}

	if healthResp.Status != "ok" && healthResp.Status != "ready" {
		return fmt.Errorf("xtts not ready: %s", healthResp.Status)
	}

	// Update GPU status
	p.config.GPUEnabled = healthResp.GPU

	log.Info().
		Str("provider", p.Name()).
		Bool("gpu", healthResp.GPU).
		Str("version", healthResp.Version).
		Msg("XTTS health check passed")

	return nil
}

// Capabilities returns the capabilities of XTTS.
func (p *Provider) Capabilities() voice.ProviderCapabilities {
	return voice.ProviderCapabilities{
		SupportsStreaming: true,
		SupportsCloning:   true,
		Languages: []string{
			"en", "es", "fr", "de", "it", "pt", "pl", "tr",
			"ru", "nl", "cs", "ar", "zh-cn", "ja", "hu", "ko", "hi",
		},
		MaxTextLength: p.config.MaxTextLength,
		RequiresGPU:   true, // XTTS requires GPU for optimal performance
		AvgLatencyMs:  1500, // 1-2s latency
		SupportedFormats: []voice.AudioFormat{
			voice.FormatWAV,
			voice.FormatMP3,
		},
	}
}

// CloneVoice creates a new cloned voice from a reference audio file.
func (p *Provider) CloneVoice(ctx context.Context, name, referenceFile, language string) (*ClonedVoice, error) {
	if name == "" {
		return nil, fmt.Errorf("voice name is required")
	}
	if referenceFile == "" {
		return nil, fmt.Errorf("reference file is required")
	}

	// Verify reference file exists
	if _, err := os.Stat(referenceFile); err != nil {
		return nil, fmt.Errorf("reference file not found: %w", err)
	}

	// Create cloned voice directory if it doesn't exist
	if err := os.MkdirAll(p.config.ClonedVoicesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cloned voices directory: %w", err)
	}

	// Generate unique ID
	id := strings.ToLower(strings.ReplaceAll(name, " ", "_"))
	if _, exists := p.clonedVoices[id]; exists {
		// Add timestamp to make it unique
		id = fmt.Sprintf("%s_%d", id, time.Now().Unix())
	}

	if language == "" {
		language = "en"
	}

	cloned := &ClonedVoice{
		ID:            id,
		Name:          name,
		ReferenceFile: referenceFile,
		Language:      language,
		CreatedAt:     time.Now(),
	}

	p.clonedVoices[id] = cloned

	log.Info().
		Str("provider", p.Name()).
		Str("voice_id", id).
		Str("name", name).
		Str("language", language).
		Msg("Voice cloned successfully")

	return cloned, nil
}

// getSpeakerFile resolves the speaker file path based on voice ID or clone file.
func (p *Provider) getSpeakerFile(voiceID, cloneFromFile string) (string, error) {
	// If clone file is explicitly specified, use it
	if cloneFromFile != "" {
		if _, err := os.Stat(cloneFromFile); err != nil {
			return "", fmt.Errorf("clone file not found: %w", err)
		}
		return cloneFromFile, nil
	}

	// If voice ID is "default", no speaker file needed
	if voiceID == "" || voiceID == "default" {
		return "", nil
	}

	// Look up cloned voice
	if cloned, ok := p.clonedVoices[voiceID]; ok {
		if _, err := os.Stat(cloned.ReferenceFile); err != nil {
			return "", fmt.Errorf("cloned voice reference file not found: %w", err)
		}
		return cloned.ReferenceFile, nil
	}

	return "", voice.ErrVoiceNotFound
}

// audioStream implements voice.AudioStream.
type audioStream struct {
	io.ReadCloser
	format     voice.AudioFormat
	sampleRate int
}

// Format returns the audio format of the stream.
func (s *audioStream) Format() voice.AudioFormat {
	return s.format
}

// SampleRate returns the sample rate in Hz.
func (s *audioStream) SampleRate() int {
	return s.sampleRate
}
