package bridge

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/normanking/cortexavatar/internal/stt"
	"github.com/normanking/cortexavatar/internal/tts"
	"github.com/rs/zerolog"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type StreamingOrchestrator struct {
	ctx         context.Context
	logger      zerolog.Logger
	sttProvider *stt.DeepgramStreamingProvider
	ttsProvider *tts.CartesiaProvider
	sendMessage func(ctx context.Context, text string) (string, error)

	mu          sync.Mutex
	isStreaming bool
	audioCh     chan []byte
	cancelFn    context.CancelFunc

	sentenceBuffer strings.Builder
	lastEmitTime   time.Time
}

func NewStreamingOrchestrator(
	ctx context.Context,
	logger zerolog.Logger,
	sttProvider *stt.DeepgramStreamingProvider,
	ttsProvider *tts.CartesiaProvider,
	sendMessage func(ctx context.Context, text string) (string, error),
) *StreamingOrchestrator {
	return &StreamingOrchestrator{
		ctx:         ctx,
		logger:      logger.With().Str("component", "streaming-orchestrator").Logger(),
		sttProvider: sttProvider,
		ttsProvider: ttsProvider,
		sendMessage: sendMessage,
	}
}

func (o *StreamingOrchestrator) Start() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.isStreaming {
		return nil
	}

	if o.sttProvider == nil || !o.sttProvider.IsAvailable() {
		o.logger.Warn().Msg("Deepgram STT not available, streaming disabled")
		return nil
	}

	ctx, cancel := context.WithCancel(o.ctx)
	o.cancelFn = cancel
	o.audioCh = make(chan []byte, 100)
	o.isStreaming = true

	transcriptCh, err := o.sttProvider.TranscribeStream(ctx, o.audioCh)
	if err != nil {
		o.isStreaming = false
		cancel()
		return err
	}

	go o.processTranscripts(ctx, transcriptCh)

	o.logger.Info().Msg("Streaming orchestrator started")
	return nil
}

func (o *StreamingOrchestrator) Stop() {
	o.mu.Lock()
	defer o.mu.Unlock()

	if !o.isStreaming {
		return
	}

	if o.cancelFn != nil {
		o.cancelFn()
	}

	if o.audioCh != nil {
		close(o.audioCh)
		o.audioCh = nil
	}

	o.isStreaming = false
	o.logger.Info().Msg("Streaming orchestrator stopped")
}

func (o *StreamingOrchestrator) SendAudio(audio []byte) {
	o.mu.Lock()
	ch := o.audioCh
	o.mu.Unlock()

	if ch != nil {
		select {
		case ch <- audio:
		default:
			o.logger.Warn().Msg("Audio channel full, dropping chunk")
		}
	}
}

func (o *StreamingOrchestrator) processTranscripts(ctx context.Context, transcriptCh <-chan *stt.TranscribeResponse) {
	for {
		select {
		case <-ctx.Done():
			return
		case resp, ok := <-transcriptCh:
			if !ok {
				return
			}

			if resp.Text == "" {
				continue
			}

			runtime.EventsEmit(o.ctx, "stt:interim", map[string]any{
				"text":    resp.Text,
				"isFinal": resp.IsFinal,
			})

			if resp.IsFinal {
				o.handleFinalTranscript(ctx, resp.Text)
			}
		}
	}
}

func (o *StreamingOrchestrator) handleFinalTranscript(ctx context.Context, text string) {
	o.logger.Info().Str("text", text).Msg("Final transcript received")

	runtime.EventsEmit(o.ctx, "audio:transcript", text)

	go func() {
		response, err := o.sendMessage(ctx, text)
		if err != nil {
			o.logger.Error().Err(err).Msg("Failed to send message")
			return
		}

		runtime.EventsEmit(o.ctx, "cortex:response", response)

		if o.ttsProvider != nil && o.ttsProvider.IsAvailable() {
			o.speakWithStreaming(ctx, response)
		}
	}()
}

func (o *StreamingOrchestrator) speakWithStreaming(ctx context.Context, text string) {
	sentences := splitIntoSentences(text)
	if len(sentences) == 0 {
		return
	}

	runtime.EventsEmit(o.ctx, "audio:speaking", true)
	defer runtime.EventsEmit(o.ctx, "audio:speaking", false)

	for _, sentence := range sentences {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if shouldSkipForTTS(sentence) {
			continue
		}

		chunkCh, err := o.ttsProvider.SynthesizeStream(ctx, &tts.SynthesizeRequest{
			Text:         sentence,
			WithPhonemes: true,
		})
		if err != nil {
			o.logger.Error().Err(err).Msg("TTS streaming failed")
			continue
		}

		for chunk := range chunkCh {
			if chunk.IsFinal {
				break
			}

			if len(chunk.Data) > 0 {
				runtime.EventsEmit(o.ctx, "audio:chunk", map[string]any{
					"data":  chunk.Data,
					"index": chunk.Index,
				})
			}

			if len(chunk.Phonemes) > 0 {
				timeline := tts.GenerateVisemeTimeline(chunk.Phonemes)
				runtime.EventsEmit(o.ctx, "viseme:timeline", timeline.ConvertToFrontendFormat())
			}
		}
	}
}

func splitIntoSentences(text string) []string {
	var sentences []string
	var current strings.Builder

	for i, ch := range text {
		current.WriteRune(ch)

		if ch == '.' || ch == '!' || ch == '?' {
			if i < len(text)-1 {
				next := text[i+1]
				if next == ' ' || next == '\n' || next == '\r' {
					s := strings.TrimSpace(current.String())
					if len(s) > 0 {
						sentences = append(sentences, s)
					}
					current.Reset()
				}
			} else {
				s := strings.TrimSpace(current.String())
				if len(s) > 0 {
					sentences = append(sentences, s)
				}
				current.Reset()
			}
		}
	}

	remaining := strings.TrimSpace(current.String())
	if len(remaining) > 0 {
		sentences = append(sentences, remaining)
	}

	return sentences
}

func shouldSkipForTTS(text string) bool {
	lower := strings.ToLower(text)

	if strings.Contains(text, "```") {
		return true
	}
	if strings.HasPrefix(text, "#") {
		return true
	}
	if strings.HasPrefix(text, "-") || strings.HasPrefix(text, "*") {
		return true
	}
	if strings.Contains(lower, "<thinking>") || strings.Contains(lower, "</thinking>") {
		return true
	}
	if len(text) > 500 {
		return true
	}

	return false
}

func (o *StreamingOrchestrator) IsStreaming() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.isStreaming
}
