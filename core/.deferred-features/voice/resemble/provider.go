// Package resemble provides a TTS provider implementation for Resemble.ai
package resemble

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/normanking/cortex/internal/logging"
	"github.com/normanking/cortex/internal/voice"
)

const (
	// SynthesisEndpoint is the Resemble.ai synthesis API endpoint
	SynthesisEndpoint = "https://f.cluster.resemble.ai/synthesize"
	// StreamingEndpoint is the Resemble.ai WebSocket streaming endpoint
	StreamingEndpoint = "wss://f.cluster.resemble.ai/stream"
	// VoicesEndpoint is the Resemble.ai voices API endpoint
	VoicesEndpoint = "https://app.resemble.ai/api/v2/voices"
	// DefaultTimeout is the default request timeout
	DefaultTimeout = 30 * time.Second
	// MaxTextLength is the maximum text length per request
	MaxTextLength = 2000
)

// Provider implements voice.Provider for Resemble.ai
type Provider struct {
	apiKey       string
	client       *http.Client
	defaultVoice string
	sampleRate   int
	log          *logging.Logger
}

// Config holds Resemble provider configuration
type Config struct {
	APIKey       string
	DefaultVoice string // Voice UUID from Resemble
	SampleRate   int    // 8000, 16000, 22050, 32000, 44100, 48000 (default)
	Timeout      time.Duration
}

// DefaultConfig returns sensible defaults for Resemble
func DefaultConfig() Config {
	return Config{
		SampleRate: 48000,
		Timeout:    DefaultTimeout,
	}
}

// NewProvider creates a new Resemble.ai TTS provider
func NewProvider(cfg Config) (*Provider, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("resemble: API key is required")
	}

	if cfg.SampleRate == 0 {
		cfg.SampleRate = 48000
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	return &Provider{
		apiKey:       cfg.APIKey,
		client:       &http.Client{Timeout: timeout},
		defaultVoice: cfg.DefaultVoice,
		sampleRate:   cfg.SampleRate,
		log:          logging.Global(),
	}, nil
}

// Name returns the provider identifier
func (p *Provider) Name() string {
	return "resemble"
}

// synthesizeRequest is the request body for the synthesis API
type synthesizeRequest struct {
	VoiceUUID    string `json:"voice_uuid"`
	Data         string `json:"data"` // Text or SSML
	ProjectUUID  string `json:"project_uuid,omitempty"`
	Title        string `json:"title,omitempty"`
	Precision    string `json:"precision,omitempty"`     // MULAW, PCM_16, PCM_24, PCM_32
	OutputFormat string `json:"output_format,omitempty"` // wav, mp3
	SampleRate   int    `json:"sample_rate,omitempty"`
}

// synthesizeResponse is the response from the synthesis API
type synthesizeResponse struct {
	Success       bool    `json:"success"`
	AudioContent  string  `json:"audio_content"` // Base64 encoded
	Duration      float64 `json:"duration"`
	SynthDuration float64 `json:"synth_duration"`
	OutputFormat  string  `json:"output_format"`
	SampleRate    int     `json:"sample_rate"`
	Issues        []struct {
		Message string `json:"message"`
	} `json:"issues"`
}

// Synthesize sends a synthesis request and returns the full audio response
func (p *Provider) Synthesize(ctx context.Context, req *voice.SynthesizeRequest) (*voice.SynthesizeResponse, error) {
	if err := voice.ValidateRequest(req); err != nil {
		return nil, fmt.Errorf("resemble: %w", err)
	}

	if len(req.Text) > MaxTextLength {
		return nil, voice.ErrTextTooLong
	}

	voiceID := req.VoiceID
	if voiceID == "" {
		voiceID = p.defaultVoice
	}
	if voiceID == "" {
		return nil, errors.New("resemble: voice_id is required (no default voice configured)")
	}

	// Build request
	synthReq := synthesizeRequest{
		VoiceUUID:    voiceID,
		Data:         req.Text,
		OutputFormat: "wav",
		SampleRate:   p.sampleRate,
		Precision:    "PCM_16",
	}

	// Override sample rate if specified
	if req.SampleRate > 0 {
		synthReq.SampleRate = req.SampleRate
	}

	// Override format if specified
	if req.Format == voice.FormatMP3 {
		synthReq.OutputFormat = "mp3"
	}

	body, err := json.Marshal(synthReq)
	if err != nil {
		return nil, fmt.Errorf("resemble: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", SynthesisEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("resemble: failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	// Note: Not using gzip to simplify response handling

	p.log.Info("[Resemble] Synthesizing: voice=%s text_len=%d endpoint=%s", voiceID, len(req.Text), SynthesisEndpoint)

	start := time.Now()
	resp, err := p.client.Do(httpReq)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, voice.ErrTimeout
		}
		return nil, fmt.Errorf("resemble: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("resemble: failed to read response: %w", err)
	}

	p.log.Info("[Resemble] Response: status=%d body_len=%d", resp.StatusCode, len(respBody))

	if resp.StatusCode != http.StatusOK {
		p.log.Error("[Resemble] API error: status=%d body=%s", resp.StatusCode, string(respBody))
		return nil, fmt.Errorf("resemble: API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var synthResp synthesizeResponse
	if err := json.Unmarshal(respBody, &synthResp); err != nil {
		p.log.Error("[Resemble] Failed to parse response: %v", err)
		return nil, fmt.Errorf("resemble: failed to parse response: %w", err)
	}

	p.log.Info("[Resemble] Parsed response: success=%v audio_len=%d issues=%d", synthResp.Success, len(synthResp.AudioContent), len(synthResp.Issues))

	if !synthResp.Success {
		errMsg := "unknown error"
		if len(synthResp.Issues) > 0 {
			errMsg = synthResp.Issues[0].Message
		}
		p.log.Error("[Resemble] Synthesis failed: %s", errMsg)
		return nil, fmt.Errorf("resemble: synthesis failed: %s", errMsg)
	}

	// Decode base64 audio
	audio, err := base64.StdEncoding.DecodeString(synthResp.AudioContent)
	if err != nil {
		p.log.Error("[Resemble] Failed to decode audio: %v", err)
		return nil, fmt.Errorf("resemble: failed to decode audio: %w", err)
	}

	p.log.Info("[Resemble] Successfully decoded %d bytes of audio", len(audio))

	format := voice.FormatWAV
	if synthResp.OutputFormat == "mp3" {
		format = voice.FormatMP3
	}

	return &voice.SynthesizeResponse{
		Audio:       audio,
		Format:      format,
		Duration:    time.Duration(synthResp.Duration * float64(time.Second)),
		SampleRate:  synthResp.SampleRate,
		ProcessedMs: time.Since(start).Milliseconds(),
		VoiceID:     voiceID,
		Provider:    "resemble",
	}, nil
}

// Stream sends a streaming synthesis request via WebSocket.
// Uses wss://f.cluster.resemble.ai/stream for lower latency audio streaming.
func (p *Provider) Stream(ctx context.Context, req *voice.SynthesizeRequest) (voice.AudioStream, error) {
	if err := voice.ValidateRequest(req); err != nil {
		return nil, fmt.Errorf("resemble: %w", err)
	}

	if len(req.Text) > MaxTextLength {
		return nil, voice.ErrTextTooLong
	}

	voiceID := req.VoiceID
	if voiceID == "" {
		voiceID = p.defaultVoice
	}
	if voiceID == "" {
		return nil, errors.New("resemble: voice_id is required (no default voice configured)")
	}

	// Create WebSocket connection with auth header
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	header := http.Header{}
	header.Set("Authorization", "Bearer "+p.apiKey)

	conn, _, err := dialer.DialContext(ctx, StreamingEndpoint, header)
	if err != nil {
		return nil, fmt.Errorf("resemble: failed to connect to streaming endpoint: %w", err)
	}

	// Create the streaming audio stream
	stream := &resembleStream{
		conn:       conn,
		format:     voice.FormatWAV,
		sampleRate: p.sampleRate,
		buffer:     make(chan []byte, 100), // Buffer for audio chunks
		done:       make(chan struct{}),
		log:        p.log,
	}

	// Send synthesis request
	streamReq := streamRequest{
		VoiceUUID:    voiceID,
		Data:         req.Text,
		SampleRate:   p.sampleRate,
		Precision:    "PCM_16",
		OutputFormat: "wav",
	}

	if err := conn.WriteJSON(streamReq); err != nil {
		conn.Close()
		return nil, fmt.Errorf("resemble: failed to send stream request: %w", err)
	}

	// Start reading audio chunks in background
	go stream.readLoop(ctx)

	return stream, nil
}

// streamRequest is the WebSocket request format for streaming synthesis.
type streamRequest struct {
	VoiceUUID    string `json:"voice_uuid"`
	Data         string `json:"data"`
	SampleRate   int    `json:"sample_rate,omitempty"`
	Precision    string `json:"precision,omitempty"`
	OutputFormat string `json:"output_format,omitempty"`
}

// streamChunk represents an audio chunk from the streaming response.
type streamChunk struct {
	Type         string `json:"type"`          // "audio" or "end"
	AudioContent string `json:"audio_content"` // Base64 encoded audio chunk
	Error        string `json:"error,omitempty"`
}

// resembleStream implements voice.AudioStream for Resemble streaming.
type resembleStream struct {
	conn       *websocket.Conn
	format     voice.AudioFormat
	sampleRate int
	buffer     chan []byte
	done       chan struct{}
	current    []byte // Current chunk being read
	offset     int    // Offset into current chunk
	err        error  // Last error encountered
	mu         sync.Mutex
	log        *logging.Logger
}

// readLoop reads audio chunks from WebSocket and buffers them.
func (s *resembleStream) readLoop(ctx context.Context) {
	defer close(s.done)
	defer close(s.buffer)

	for {
		select {
		case <-ctx.Done():
			s.setError(ctx.Err())
			return
		default:
		}

		_, message, err := s.conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				return // Normal close
			}
			s.setError(fmt.Errorf("resemble: WebSocket read error: %w", err))
			return
		}

		var chunk streamChunk
		if err := json.Unmarshal(message, &chunk); err != nil {
			s.setError(fmt.Errorf("resemble: failed to parse stream chunk: %w", err))
			return
		}

		if chunk.Error != "" {
			s.setError(fmt.Errorf("resemble: stream error: %s", chunk.Error))
			return
		}

		if chunk.Type == "end" {
			return // Stream complete
		}

		if chunk.Type == "audio" && chunk.AudioContent != "" {
			audio, err := base64.StdEncoding.DecodeString(chunk.AudioContent)
			if err != nil {
				s.log.Warn("[Resemble] Failed to decode audio chunk: %v", err)
				continue
			}

			select {
			case s.buffer <- audio:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (s *resembleStream) setError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err == nil {
		s.err = err
	}
}

func (s *resembleStream) getError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// Read implements io.Reader for streaming audio data.
func (s *resembleStream) Read(p []byte) (int, error) {
	// Check for errors
	if err := s.getError(); err != nil {
		return 0, err
	}

	// If we have remaining data in current chunk, use it first
	if s.offset < len(s.current) {
		n := copy(p, s.current[s.offset:])
		s.offset += n
		return n, nil
	}

	// Get next chunk from buffer
	select {
	case chunk, ok := <-s.buffer:
		if !ok {
			// Buffer closed, check for error or return EOF
			if err := s.getError(); err != nil {
				return 0, err
			}
			return 0, io.EOF
		}
		s.current = chunk
		s.offset = 0
		n := copy(p, s.current)
		s.offset = n
		return n, nil
	case <-s.done:
		if err := s.getError(); err != nil {
			return 0, err
		}
		return 0, io.EOF
	}
}

// Close closes the streaming connection.
func (s *resembleStream) Close() error {
	return s.conn.Close()
}

// Format returns the audio format of the stream.
func (s *resembleStream) Format() voice.AudioFormat {
	return s.format
}

// SampleRate returns the sample rate in Hz.
func (s *resembleStream) SampleRate() int {
	return s.sampleRate
}

// voicesResponse is the response from the voices API
type voicesResponse struct {
	Success      bool `json:"success"`
	Page         int  `json:"page"`
	TotalResults int  `json:"total_results"`
	Items        []struct {
		UUID            string   `json:"uuid"`
		Name            string   `json:"name"`
		Status          string   `json:"status"`
		DefaultLanguage string   `json:"default_language"`
		Languages       []string `json:"supported_languages"`
	} `json:"items"`
}

// ListVoices returns available voices from Resemble
func (p *Provider) ListVoices(ctx context.Context) ([]voice.Voice, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", VoicesEndpoint+"?page=1&page_size=100", nil)
	if err != nil {
		return nil, fmt.Errorf("resemble: failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("resemble: failed to fetch voices: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("resemble: failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("resemble: API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var voicesResp voicesResponse
	if err := json.Unmarshal(respBody, &voicesResp); err != nil {
		return nil, fmt.Errorf("resemble: failed to parse response: %w", err)
	}

	if !voicesResp.Success {
		return nil, errors.New("resemble: failed to fetch voices")
	}

	voices := make([]voice.Voice, 0, len(voicesResp.Items))
	for _, v := range voicesResp.Items {
		// Resemble API returns "finished" for status and "Ready" for voice_status
		// Accept both "ready" and "finished" as valid states
		if v.Status != "ready" && v.Status != "finished" {
			continue // Skip voices that aren't ready
		}

		lang := v.DefaultLanguage
		if lang == "" && len(v.Languages) > 0 {
			lang = v.Languages[0]
		}

		voices = append(voices, voice.Voice{
			ID:       v.UUID,
			Name:     v.Name,
			Language: lang,
			Gender:   voice.GenderUnknown, // Resemble doesn't expose gender in API
			Metadata: map[string]string{
				"provider": "resemble",
				"status":   v.Status,
			},
		})
	}

	return voices, nil
}

// Health checks if the provider is available
func (p *Provider) Health(ctx context.Context) error {
	// Try to list voices as a health check
	_, err := p.ListVoices(ctx)
	if err != nil {
		return fmt.Errorf("resemble: health check failed: %w", err)
	}
	return nil
}

// Capabilities returns the provider's feature set
func (p *Provider) Capabilities() voice.ProviderCapabilities {
	return voice.ProviderCapabilities{
		SupportsStreaming: true, // WebSocket streaming implemented
		SupportsCloning:   true, // Resemble supports custom voices
		Languages:         []string{"en-US", "es-ES", "fr-FR", "de-DE", "it-IT", "pt-BR", "ja-JP", "ko-KR", "zh-CN"},
		MaxTextLength:     MaxTextLength,
		RequiresGPU:       false, // Cloud service
		AvgLatencyMs:      300,   // Lower latency with streaming
		SupportedFormats:  []voice.AudioFormat{voice.FormatWAV, voice.FormatMP3},
	}
}

// TestSynthesis tests the API with a simple synthesis request
func (p *Provider) TestSynthesis(ctx context.Context, voiceID string) error {
	if voiceID == "" {
		voiceID = p.defaultVoice
	}
	if voiceID == "" {
		// Try to get first available voice
		voices, err := p.ListVoices(ctx)
		if err != nil {
			return fmt.Errorf("cannot test synthesis: %w", err)
		}
		if len(voices) == 0 {
			return errors.New("no voices available for testing")
		}
		voiceID = voices[0].ID
	}

	_, err := p.Synthesize(ctx, &voice.SynthesizeRequest{
		Text:    "Hello, this is a test.",
		VoiceID: voiceID,
	})
	return err
}
