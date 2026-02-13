package stt

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

const (
	DeepgramWSEndpoint = "wss://api.deepgram.com/v1/listen"
	DeepgramModel      = "nova-2"
)

type DeepgramStreamingProvider struct {
	apiKey string
	logger zerolog.Logger
	config *DeepgramConfig

	conn        *websocket.Conn
	connMu      sync.Mutex
	isConnected bool

	transcriptCh chan *TranscribeResponse
	errorCh      chan error
	closeCh      chan struct{}
}

type DeepgramConfig struct {
	APIKey         string        `json:"api_key"`
	Model          string        `json:"model"`
	Language       string        `json:"language"`
	SampleRate     int           `json:"sample_rate"`
	Encoding       string        `json:"encoding"`
	Channels       int           `json:"channels"`
	InterimResults bool          `json:"interim_results"`
	Punctuate      bool          `json:"punctuate"`
	Timeout        time.Duration `json:"timeout"`
}

func DefaultDeepgramConfig() *DeepgramConfig {
	return &DeepgramConfig{
		Model:          DeepgramModel,
		Language:       "en-US",
		SampleRate:     16000,
		Encoding:       "linear16",
		Channels:       1,
		InterimResults: true,
		Punctuate:      true,
		Timeout:        30 * time.Second,
	}
}

func NewDeepgramStreamingProvider(logger zerolog.Logger, config *DeepgramConfig) *DeepgramStreamingProvider {
	if config == nil {
		config = DefaultDeepgramConfig()
	}

	apiKey := config.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("DEEPGRAM_API_KEY")
	}

	return &DeepgramStreamingProvider{
		apiKey:  apiKey,
		logger:  logger.With().Str("provider", "deepgram-streaming").Logger(),
		config:  config,
		closeCh: make(chan struct{}),
	}
}

func (p *DeepgramStreamingProvider) Name() string {
	return "deepgram-streaming"
}

func (p *DeepgramStreamingProvider) IsAvailable() bool {
	return p.apiKey != ""
}

func (p *DeepgramStreamingProvider) SetAPIKey(apiKey string) {
	p.apiKey = apiKey
}

type deepgramMessage struct {
	Type        string            `json:"type"`
	ChannelIdx  int               `json:"channel_index,omitempty"`
	Duration    float64           `json:"duration,omitempty"`
	Start       float64           `json:"start,omitempty"`
	IsFinal     bool              `json:"is_final,omitempty"`
	SpeechFinal bool              `json:"speech_final,omitempty"`
	Channel     deepgramChannel   `json:"channel,omitempty"`
	Metadata    *deepgramMetadata `json:"metadata,omitempty"`
}

type deepgramChannel struct {
	Alternatives []deepgramAlternative `json:"alternatives,omitempty"`
}

type deepgramAlternative struct {
	Transcript string         `json:"transcript"`
	Confidence float64        `json:"confidence"`
	Words      []deepgramWord `json:"words,omitempty"`
}

type deepgramWord struct {
	Word       string  `json:"word"`
	Start      float64 `json:"start"`
	End        float64 `json:"end"`
	Confidence float64 `json:"confidence"`
}

type deepgramMetadata struct {
	RequestID string  `json:"request_id"`
	ModelInfo string  `json:"model_info"`
	Duration  float64 `json:"duration"`
}

func (p *DeepgramStreamingProvider) connect(ctx context.Context) error {
	p.connMu.Lock()
	defer p.connMu.Unlock()

	if p.isConnected && p.conn != nil {
		return nil
	}

	url := fmt.Sprintf("%s?model=%s&language=%s&encoding=%s&sample_rate=%d&channels=%d&punctuate=%t&interim_results=%t",
		DeepgramWSEndpoint,
		p.config.Model,
		p.config.Language,
		p.config.Encoding,
		p.config.SampleRate,
		p.config.Channels,
		p.config.Punctuate,
		p.config.InterimResults,
	)

	header := http.Header{}
	header.Set("Authorization", "Token "+p.apiKey)

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, resp, err := dialer.DialContext(ctx, url, header)
	if err != nil {
		if resp != nil {
			p.logger.Error().
				Int("status", resp.StatusCode).
				Err(err).
				Msg("Deepgram WebSocket connection failed")
		}
		return fmt.Errorf("websocket dial: %w", err)
	}

	p.conn = conn
	p.isConnected = true
	p.logger.Info().Msg("Connected to Deepgram streaming STT")

	return nil
}

func (p *DeepgramStreamingProvider) StartStreaming(ctx context.Context) (<-chan *TranscribeResponse, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("Deepgram API key not configured")
	}

	if err := p.connect(ctx); err != nil {
		return nil, err
	}

	p.transcriptCh = make(chan *TranscribeResponse, 32)
	p.errorCh = make(chan error, 1)

	go p.readResponses(ctx)

	return p.transcriptCh, nil
}

func (p *DeepgramStreamingProvider) readResponses(ctx context.Context) {
	defer close(p.transcriptCh)

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.closeCh:
			return
		default:
		}

		if p.conn == nil {
			return
		}

		_, message, err := p.conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				p.logger.Debug().Msg("Deepgram connection closed normally")
				return
			}
			p.logger.Error().Err(err).Msg("Error reading Deepgram response")
			return
		}

		var msg deepgramMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			p.logger.Warn().Err(err).Str("message", string(message)).Msg("Failed to parse Deepgram message")
			continue
		}

		switch msg.Type {
		case "Results":
			if len(msg.Channel.Alternatives) > 0 {
				alt := msg.Channel.Alternatives[0]
				if alt.Transcript != "" {
					resp := &TranscribeResponse{
						Text:       alt.Transcript,
						Confidence: alt.Confidence,
						Language:   p.config.Language,
						Duration:   time.Duration(msg.Duration * float64(time.Second)),
						IsFinal:    msg.IsFinal || msg.SpeechFinal,
					}

					for _, w := range alt.Words {
						resp.Words = append(resp.Words, Word{
							Word:       w.Word,
							Start:      time.Duration(w.Start * float64(time.Second)),
							End:        time.Duration(w.End * float64(time.Second)),
							Confidence: w.Confidence,
						})
					}

					select {
					case p.transcriptCh <- resp:
						p.logger.Debug().
							Str("text", alt.Transcript).
							Bool("final", resp.IsFinal).
							Float64("confidence", alt.Confidence).
							Msg("Deepgram transcript")
					default:
						p.logger.Warn().Msg("Transcript channel full, dropping")
					}
				}
			}

		case "Metadata":
			p.logger.Debug().
				Str("requestID", msg.Metadata.RequestID).
				Msg("Deepgram metadata received")

		case "UtteranceEnd":
			p.logger.Debug().Msg("Deepgram utterance end")

		case "Error":
			p.logger.Error().Str("message", string(message)).Msg("Deepgram error")
		}
	}
}

func (p *DeepgramStreamingProvider) SendAudio(audio []byte) error {
	p.connMu.Lock()
	defer p.connMu.Unlock()

	if !p.isConnected || p.conn == nil {
		return fmt.Errorf("not connected")
	}

	return p.conn.WriteMessage(websocket.BinaryMessage, audio)
}

func (p *DeepgramStreamingProvider) StopStreaming() error {
	p.connMu.Lock()
	defer p.connMu.Unlock()

	if p.conn == nil {
		return nil
	}

	closeMsg := []byte(`{"type": "CloseStream"}`)
	if err := p.conn.WriteMessage(websocket.TextMessage, closeMsg); err != nil {
		p.logger.Warn().Err(err).Msg("Failed to send close message")
	}

	err := p.conn.Close()
	p.conn = nil
	p.isConnected = false

	p.logger.Info().Msg("Deepgram streaming stopped")
	return err
}

func (p *DeepgramStreamingProvider) Transcribe(ctx context.Context, req *TranscribeRequest) (*TranscribeResponse, error) {
	transcriptCh, err := p.StartStreaming(ctx)
	if err != nil {
		return nil, err
	}
	defer p.StopStreaming()

	chunkSize := p.config.SampleRate * 2 / 10
	for i := 0; i < len(req.Audio); i += chunkSize {
		end := i + chunkSize
		if end > len(req.Audio) {
			end = len(req.Audio)
		}
		if err := p.SendAudio(req.Audio[i:end]); err != nil {
			return nil, err
		}
		time.Sleep(100 * time.Millisecond)
	}

	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()

	var finalResponse *TranscribeResponse
	for {
		select {
		case resp, ok := <-transcriptCh:
			if !ok {
				if finalResponse != nil {
					return finalResponse, nil
				}
				return nil, fmt.Errorf("no transcription received")
			}
			if resp.IsFinal {
				return resp, nil
			}
			finalResponse = resp

		case <-timeout.C:
			if finalResponse != nil {
				finalResponse.IsFinal = true
				return finalResponse, nil
			}
			return nil, fmt.Errorf("transcription timeout")

		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (p *DeepgramStreamingProvider) TranscribeStream(ctx context.Context, audioStream <-chan []byte) (<-chan *TranscribeResponse, error) {
	transcriptCh, err := p.StartStreaming(ctx)
	if err != nil {
		return nil, err
	}

	resultCh := make(chan *TranscribeResponse, 32)

	go func() {
		defer close(resultCh)
		defer p.StopStreaming()

		for {
			select {
			case <-ctx.Done():
				return
			case audio, ok := <-audioStream:
				if !ok {
					return
				}
				if err := p.SendAudio(audio); err != nil {
					p.logger.Error().Err(err).Msg("Failed to send audio")
					return
				}
			}
		}
	}()

	go func() {
		for resp := range transcriptCh {
			select {
			case resultCh <- resp:
			case <-ctx.Done():
				return
			}
		}
	}()

	return resultCh, nil
}

func (p *DeepgramStreamingProvider) Health(ctx context.Context) error {
	if p.apiKey == "" {
		return fmt.Errorf("Deepgram API key not configured")
	}
	return nil
}

func (p *DeepgramStreamingProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsStreaming:  true,
		SupportsTimestamps: true,
		SupportedLanguages: []string{"en", "es", "fr", "de", "it", "pt", "nl", "ja", "ko", "zh", "ru", "ar", "hi"},
		MaxAudioLengthSec:  0,
		AvgLatencyMs:       150,
		RequiresGPU:        false,
		IsLocal:            false,
	}
}

func (p *DeepgramStreamingProvider) Close() error {
	close(p.closeCh)
	return p.StopStreaming()
}
