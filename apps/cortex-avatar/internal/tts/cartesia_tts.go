// Package tts provides Cartesia Sonic TTS provider for ultra-low-latency streaming speech synthesis.
// Cartesia provides 40-90ms latency with word timestamps for lip-sync.
package tts

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

// Cartesia WebSocket endpoint
const (
	CartesiaWSEndpoint   = "wss://api.cartesia.ai/tts/websocket"
	CartesiaAPIVersion   = "2025-04-16"
	CartesiaDefaultModel = "sonic-3"
	CartesiaSampleRate   = 22050 // Cartesia default, good quality
)

// CartesiaProvider implements streaming TTS using Cartesia Sonic API
type CartesiaProvider struct {
	apiKey string
	logger zerolog.Logger
	config *CartesiaConfig

	// WebSocket connection pool (reuse connections for low latency)
	conn     *websocket.Conn
	connMu   sync.Mutex
	lastUsed time.Time

	// Keep connection alive
	keepAlive *time.Ticker
	closeCh   chan struct{}
}

// CartesiaConfig holds Cartesia TTS configuration
type CartesiaConfig struct {
	APIKey       string        `json:"api_key"`
	Model        string        `json:"model"`         // sonic-3 (latest), sonic-2
	DefaultVoice string        `json:"default_voice"` // Voice UUID
	Language     string        `json:"language"`      // en, es, fr, de, etc.
	SampleRate   int           `json:"sample_rate"`   // 8000, 16000, 22050, 24000, 44100
	Encoding     string        `json:"encoding"`      // pcm_s16le, pcm_f32le, pcm_mulaw, pcm_alaw
	Container    string        `json:"container"`     // raw, wav
	Timeout      time.Duration `json:"timeout"`
}

// DefaultCartesiaConfig returns sensible defaults for real-time use
func DefaultCartesiaConfig() *CartesiaConfig {
	return &CartesiaConfig{
		Model:        CartesiaDefaultModel,
		DefaultVoice: "a0e99841-438c-4a64-b679-ae501e7d6091", // Default voice
		Language:     "en",
		SampleRate:   CartesiaSampleRate,
		Encoding:     "pcm_s16le",
		Container:    "raw",
		Timeout:      30 * time.Second,
	}
}

// NewCartesiaProvider creates a new Cartesia TTS provider
func NewCartesiaProvider(logger zerolog.Logger, config *CartesiaConfig) *CartesiaProvider {
	if config == nil {
		config = DefaultCartesiaConfig()
	}

	// Get API key from config or environment
	apiKey := config.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("CARTESIA_API_KEY")
	}

	p := &CartesiaProvider{
		apiKey:  apiKey,
		logger:  logger.With().Str("provider", "cartesia-tts").Logger(),
		config:  config,
		closeCh: make(chan struct{}),
	}

	return p
}

// Name returns the provider identifier
func (p *CartesiaProvider) Name() string {
	return "cartesia"
}

// IsAvailable checks if the provider has an API key configured
func (p *CartesiaProvider) IsAvailable() bool {
	return p.apiKey != ""
}

// SetAPIKey sets the API key
func (p *CartesiaProvider) SetAPIKey(key string) {
	p.apiKey = key
	p.logger.Info().Msg("Cartesia API key updated")
}

// --- WebSocket Messages ---

// CartesiaRequest is the generation request format
type CartesiaRequest struct {
	ModelID       string               `json:"model_id"`
	Transcript    string               `json:"transcript"`
	Voice         CartesiaVoice        `json:"voice"`
	Language      string               `json:"language"`
	ContextID     string               `json:"context_id,omitempty"`
	OutputFormat  CartesiaOutputFormat `json:"output_format"`
	AddTimestamps bool                 `json:"add_timestamps"`
	Continue      bool                 `json:"continue"`
}

// CartesiaVoice specifies the voice to use
type CartesiaVoice struct {
	Mode string `json:"mode"` // "id" or "embedding"
	ID   string `json:"id,omitempty"`
}

// CartesiaOutputFormat specifies the audio output format
type CartesiaOutputFormat struct {
	Container  string `json:"container"`   // raw, wav
	Encoding   string `json:"encoding"`    // pcm_s16le, pcm_f32le, etc.
	SampleRate int    `json:"sample_rate"` // 8000-44100
}

// CartesiaResponse is a generic response that can be any type
type CartesiaResponse struct {
	Type       string `json:"type"`
	Done       bool   `json:"done"`
	StatusCode int    `json:"status_code"`
	ContextID  string `json:"context_id,omitempty"`

	// For type="chunk"
	Data     string `json:"data,omitempty"`
	StepTime int    `json:"step_time,omitempty"` // Processing time in ms

	// For type="timestamps"
	WordTimestamps *CartesiaTimestamps `json:"word_timestamps,omitempty"`

	// For type="error"
	Error string `json:"error,omitempty"`

	// For type="flush_done"
	FlushDone bool `json:"flush_done,omitempty"`
	FlushID   int  `json:"flush_id,omitempty"`
}

// CartesiaTimestamps contains word-level timing data
type CartesiaTimestamps struct {
	Words []string  `json:"words"`
	Start []float64 `json:"start"` // Start times in seconds
	End   []float64 `json:"end"`   // End times in seconds
}

// --- Provider Implementation ---

// connect establishes or reuses a WebSocket connection
func (p *CartesiaProvider) connect(ctx context.Context) (*websocket.Conn, error) {
	p.connMu.Lock()
	defer p.connMu.Unlock()

	// Reuse existing connection if it's recent
	if p.conn != nil && time.Since(p.lastUsed) < 30*time.Second {
		p.lastUsed = time.Now()
		return p.conn, nil
	}

	// Close old connection if exists
	if p.conn != nil {
		p.conn.Close()
		p.conn = nil
	}

	// Build WebSocket URL with auth
	url := fmt.Sprintf("%s?api_key=%s&cartesia_version=%s",
		CartesiaWSEndpoint, p.apiKey, CartesiaAPIVersion)

	p.logger.Debug().Str("url", CartesiaWSEndpoint).Msg("Connecting to Cartesia WebSocket")

	// Connect with timeout from context
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, resp, err := dialer.DialContext(ctx, url, http.Header{})
	if err != nil {
		if resp != nil {
			p.logger.Error().
				Int("status", resp.StatusCode).
				Err(err).
				Msg("Cartesia WebSocket connection failed")
		}
		return nil, fmt.Errorf("websocket dial: %w", err)
	}

	p.conn = conn
	p.lastUsed = time.Now()

	p.logger.Info().Msg("Connected to Cartesia WebSocket")
	return conn, nil
}

// generateContextID creates a unique context ID for a request
func (p *CartesiaProvider) generateContextID() string {
	return fmt.Sprintf("cortex-%d", time.Now().UnixNano())
}

// Synthesize converts text to audio (non-streaming, collects all chunks)
func (p *CartesiaProvider) Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("Cartesia API key not configured")
	}

	startTime := time.Now()

	// Use default voice if not specified
	voiceID := req.VoiceID
	if voiceID == "" {
		voiceID = p.config.DefaultVoice
	}

	// Connect to WebSocket
	conn, err := p.connect(ctx)
	if err != nil {
		return nil, err
	}

	contextID := p.generateContextID()

	// Build request
	cartesiaReq := CartesiaRequest{
		ModelID:    p.config.Model,
		Transcript: req.Text,
		Voice: CartesiaVoice{
			Mode: "id",
			ID:   voiceID,
		},
		Language:  p.config.Language,
		ContextID: contextID,
		OutputFormat: CartesiaOutputFormat{
			Container:  p.config.Container,
			Encoding:   p.config.Encoding,
			SampleRate: p.config.SampleRate,
		},
		AddTimestamps: req.WithPhonemes, // Get timestamps for lip-sync
		Continue:      false,
	}

	// Send request
	if err := conn.WriteJSON(cartesiaReq); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	p.logger.Debug().
		Str("voice", voiceID).
		Str("contextID", contextID).
		Int("textLen", len(req.Text)).
		Bool("timestamps", req.WithPhonemes).
		Msg("Sent TTS request to Cartesia")

	// Collect response chunks
	var audioData []byte
	var timestamps *CartesiaTimestamps
	var stepTimes []int

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		var resp CartesiaResponse
		if err := conn.ReadJSON(&resp); err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}

		// Handle different response types
		switch resp.Type {
		case "chunk":
			// Decode base64 audio data
			audioBytes, err := base64.StdEncoding.DecodeString(resp.Data)
			if err != nil {
				p.logger.Warn().Err(err).Msg("Failed to decode audio chunk")
				continue
			}
			audioData = append(audioData, audioBytes...)
			if resp.StepTime > 0 {
				stepTimes = append(stepTimes, resp.StepTime)
			}

		case "timestamps":
			if resp.WordTimestamps != nil {
				timestamps = resp.WordTimestamps
				p.logger.Debug().
					Int("words", len(timestamps.Words)).
					Msg("Received word timestamps")
			}

		case "done":
			// Generation complete
			p.logger.Debug().
				Int("audioBytes", len(audioData)).
				Msg("Cartesia generation complete")
			goto done

		case "error":
			return nil, fmt.Errorf("Cartesia error: %s", resp.Error)

		case "flush_done":
			// Flush completed, continue reading
			continue
		}
	}

done:
	processingTime := time.Since(startTime)

	// Calculate average step time
	avgStepTime := 0
	if len(stepTimes) > 0 {
		total := 0
		for _, t := range stepTimes {
			total += t
		}
		avgStepTime = total / len(stepTimes)
	}

	// Convert timestamps to phonemes for lip-sync
	var phonemes []Phoneme
	if timestamps != nil {
		phonemes = p.timestampsToPhonemes(timestamps)
	}

	p.logger.Info().
		Str("voice", voiceID).
		Int("audioBytes", len(audioData)).
		Int("avgStepTimeMs", avgStepTime).
		Dur("totalTime", processingTime).
		Int("phonemes", len(phonemes)).
		Msg("Cartesia TTS synthesis complete")

	// Determine format for response
	format := "pcm"
	if p.config.Container == "wav" {
		format = "wav"
	}

	return &SynthesizeResponse{
		Audio:          audioData,
		Format:         format,
		SampleRate:     p.config.SampleRate,
		ProcessingTime: processingTime,
		VoiceID:        voiceID,
		Provider:       p.Name(),
		Phonemes:       phonemes,
	}, nil
}

// timestampsToPhonemes converts Cartesia word timestamps to phoneme data for lip-sync
// This uses word-level timing to generate approximate visemes
func (p *CartesiaProvider) timestampsToPhonemes(ts *CartesiaTimestamps) []Phoneme {
	if ts == nil || len(ts.Words) == 0 {
		return nil
	}

	var phonemes []Phoneme

	for i, word := range ts.Words {
		if i >= len(ts.Start) || i >= len(ts.End) {
			break
		}

		startMs := time.Duration(ts.Start[i] * float64(time.Second))
		endMs := time.Duration(ts.End[i] * float64(time.Second))
		duration := endMs - startMs

		// Generate visemes for each word
		wordVisemes := wordToVisemes(word, startMs, duration)
		phonemes = append(phonemes, wordVisemes...)
	}

	return phonemes
}

// wordToVisemes converts a word into a sequence of visemes with timing
// This is a simplified phoneme-to-viseme mapping based on word spelling
func wordToVisemes(word string, startTime, duration time.Duration) []Phoneme {
	if len(word) == 0 {
		return nil
	}

	// Approximate phoneme duration based on word length
	// Average English word has ~3.5 phonemes per word
	estimatedPhonemes := max(1, len(word)*3/4)
	phonemeDuration := duration / time.Duration(estimatedPhonemes)

	var phonemes []Phoneme
	currentTime := startTime

	// Simple letter-to-viseme mapping for English
	// This is an approximation - Cartesia doesn't provide phoneme-level data
	for i, ch := range []byte(word) {
		viseme := letterToViseme(ch)
		if viseme == "" {
			continue
		}

		// Blend with next viseme for smoother transitions
		nextViseme := viseme
		if i < len(word)-1 {
			nextViseme = letterToViseme(word[i+1])
			if nextViseme == "" {
				nextViseme = viseme
			}
		}

		phonemes = append(phonemes, Phoneme{
			Phoneme:  string(ch),
			Viseme:   string(viseme),
			Start:    currentTime,
			End:      currentTime + phonemeDuration,
			Duration: phonemeDuration,
		})

		currentTime += phonemeDuration
	}

	// Add silence at word boundaries
	if len(phonemes) > 0 {
		phonemes = append(phonemes, Phoneme{
			Phoneme:  " ",
			Viseme:   string(VisemeSilent),
			Start:    currentTime,
			End:      currentTime + 50*time.Millisecond,
			Duration: 50 * time.Millisecond,
		})
	}

	return phonemes
}

// letterToViseme maps ASCII letters to approximate visemes
func letterToViseme(ch byte) Viseme {
	switch ch {
	// Vowels - mouth open
	case 'a', 'A':
		return VisemeAA
	case 'e', 'E':
		return VisemeEE
	case 'i', 'I':
		return VisemeII
	case 'o', 'O':
		return VisemeOO
	case 'u', 'U':
		return VisemeUU

	// Lip consonants
	case 'b', 'B', 'm', 'M', 'p', 'P':
		return VisemeMBP

	// Lip-teeth consonants
	case 'f', 'F', 'v', 'V':
		return VisemeFV

	// Tongue-teeth consonants
	case 't', 'T', 'd', 'D', 'n', 'N', 'l', 'L':
		return VisemeLNTD

	// Sibilants
	case 's', 'S', 'z', 'Z':
		return VisemeSZ

	// Back tongue
	case 'k', 'K', 'g', 'G':
		return VisemeKG

	// Affricates
	case 'c', 'C', 'j', 'J':
		return VisemeCHJ

	// R sound
	case 'r', 'R':
		return VisemeR

	// W sound
	case 'w', 'W', 'q', 'Q':
		return VisemeWQ

	// H - open mouth
	case 'h', 'H':
		return VisemeAA

	// Y - ee sound
	case 'y', 'Y':
		return VisemeEE

	// X - ks sound
	case 'x', 'X':
		return VisemeKG

	default:
		return ""
	}
}

// SynthesizeStream handles streaming synthesis with real-time audio chunks
func (p *CartesiaProvider) SynthesizeStream(ctx context.Context, req *SynthesizeRequest) (<-chan *AudioChunk, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("Cartesia API key not configured")
	}

	chunks := make(chan *AudioChunk, 32)

	go func() {
		defer close(chunks)

		// Use default voice if not specified
		voiceID := req.VoiceID
		if voiceID == "" {
			voiceID = p.config.DefaultVoice
		}

		// Connect to WebSocket
		conn, err := p.connect(ctx)
		if err != nil {
			p.logger.Error().Err(err).Msg("Stream connection failed")
			return
		}

		contextID := p.generateContextID()

		// Build request with timestamps enabled
		cartesiaReq := CartesiaRequest{
			ModelID:    p.config.Model,
			Transcript: req.Text,
			Voice: CartesiaVoice{
				Mode: "id",
				ID:   voiceID,
			},
			Language:  p.config.Language,
			ContextID: contextID,
			OutputFormat: CartesiaOutputFormat{
				Container:  p.config.Container,
				Encoding:   p.config.Encoding,
				SampleRate: p.config.SampleRate,
			},
			AddTimestamps: true, // Always get timestamps for streaming
			Continue:      false,
		}

		// Send request
		if err := conn.WriteJSON(cartesiaReq); err != nil {
			p.logger.Error().Err(err).Msg("Failed to send stream request")
			return
		}

		p.logger.Debug().
			Str("voice", voiceID).
			Str("contextID", contextID).
			Msg("Started streaming TTS request")

		var pendingTimestamps *CartesiaTimestamps
		chunkIndex := 0

		for {
			select {
			case <-ctx.Done():
				p.logger.Debug().Msg("Stream context canceled")
				return
			default:
			}

			var resp CartesiaResponse
			if err := conn.ReadJSON(&resp); err != nil {
				p.logger.Error().Err(err).Msg("Stream read error")
				return
			}

			switch resp.Type {
			case "chunk":
				audioBytes, err := base64.StdEncoding.DecodeString(resp.Data)
				if err != nil {
					p.logger.Warn().Err(err).Msg("Failed to decode audio chunk")
					continue
				}

				// Include pending timestamps with this chunk
				var chunkPhonemes []Phoneme
				if pendingTimestamps != nil {
					chunkPhonemes = p.timestampsToPhonemes(pendingTimestamps)
					pendingTimestamps = nil
				}

				chunks <- &AudioChunk{
					Data:     audioBytes,
					Index:    chunkIndex,
					IsFinal:  false,
					Phonemes: chunkPhonemes,
				}
				chunkIndex++

			case "timestamps":
				// Store timestamps to include with next audio chunk
				pendingTimestamps = resp.WordTimestamps

			case "done":
				// Send final marker
				chunks <- &AudioChunk{
					Data:    nil,
					Index:   chunkIndex,
					IsFinal: true,
				}
				p.logger.Debug().Int("chunks", chunkIndex).Msg("Stream complete")
				return

			case "error":
				p.logger.Error().Str("error", resp.Error).Msg("Stream error")
				return

			case "flush_done":
				continue
			}
		}
	}()

	return chunks, nil
}

// ListVoices returns available Cartesia voices
// Note: This would require an HTTP API call to get the full list
func (p *CartesiaProvider) ListVoices(ctx context.Context) ([]Voice, error) {
	// Return some common Cartesia voices
	// For a full list, use Cartesia's Voice API
	return []Voice{
		{ID: "a0e99841-438c-4a64-b679-ae501e7d6091", Name: "Reflection (Female)", Language: "en", Gender: "female"},
		{ID: "79a125e8-cd45-4c13-8a67-188112f4dd22", Name: "British Lady", Language: "en", Gender: "female"},
		{ID: "b7d50908-b17c-442d-ad8d-810c63997ed9", Name: "Confident", Language: "en", Gender: "female"},
		{ID: "694f9389-aac1-45b6-b726-9d9369183238", Name: "Friendly", Language: "en", Gender: "male"},
		{ID: "421b3369-f63f-4b03-8980-37a44df1d4e8", Name: "Newsman", Language: "en", Gender: "male"},
	}, nil
}

// Health checks if Cartesia API is available
func (p *CartesiaProvider) Health(ctx context.Context) error {
	if p.apiKey == "" {
		return ErrProviderUnavailable
	}

	// Try to establish a connection
	_, err := p.connect(ctx)
	if err != nil {
		return fmt.Errorf("cartesia health check failed: %w", err)
	}

	return nil
}

// Capabilities returns Cartesia TTS capabilities
func (p *CartesiaProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsStreaming:  true,
		SupportsCloning:    true, // Cartesia supports voice cloning
		SupportsPhonemes:   true, // Word timestamps for lip-sync
		SupportedLanguages: []string{"en", "es", "fr", "de", "it", "pt", "ja", "ko", "zh", "ar", "hi"},
		MaxTextLength:      10000,
		AvgLatencyMs:       75, // 40-90ms typical
		RequiresGPU:        false,
		IsLocal:            false,
	}
}

// Close closes the provider and any open connections
func (p *CartesiaProvider) Close() error {
	close(p.closeCh)

	p.connMu.Lock()
	defer p.connMu.Unlock()

	if p.conn != nil {
		err := p.conn.Close()
		p.conn = nil
		return err
	}

	return nil
}

// max returns the larger of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
