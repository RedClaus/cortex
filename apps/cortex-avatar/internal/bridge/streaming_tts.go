// Package bridge provides StreamingTTS for handling streaming LLM responses with incremental TTS.
package bridge

import (
	"context"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/normanking/cortexavatar/internal/tts"
	"github.com/rs/zerolog"
)

// StreamingTTS handles streaming response chunks and sends them to TTS incrementally.
// It buffers text until natural breakpoints (sentence boundaries) are detected,
// then sends complete sentences to TTS while continuing to receive more chunks.
type StreamingTTS struct {
	ttsProvider tts.Provider
	logger      zerolog.Logger
	onAudio     func(audio []byte, format string, phonemes []tts.Phoneme)
	onStart     func()
	onStop      func()

	mu     sync.Mutex
	buffer strings.Builder

	// Configuration
	minChunkSize     int           // Minimum chars before considering flush
	maxBufferSize    int           // Force flush at this size
	flushTimeout     time.Duration // Flush after this duration of no new chunks
	commaBreakLength int           // Minimum chars before comma triggers break
	lastChunkTime    time.Time     // Time of last chunk received

	// State
	cancelFn   context.CancelFunc
	isSpeaking bool
	voiceID    string
}

// StreamingTTSConfig holds configuration for StreamingTTS
type StreamingTTSConfig struct {
	MinChunkSize     int           // Minimum chars before considering flush (default: 20)
	MaxBufferSize    int           // Force flush at this size (default: 200)
	FlushTimeout     time.Duration // Flush after silence (default: 500ms)
	VoiceID          string        // Voice to use for TTS
	CommaBreakLength int           // Minimum chars before comma triggers break (default: 40)
}

// DefaultStreamingTTSConfig returns sensible defaults
func DefaultStreamingTTSConfig() *StreamingTTSConfig {
	return &StreamingTTSConfig{
		MinChunkSize:     20,
		MaxBufferSize:    200,
		FlushTimeout:     500 * time.Millisecond,
		VoiceID:          "nova",
		CommaBreakLength: 40, // Comma only triggers break after 40+ chars
	}
}

// NewStreamingTTS creates a new StreamingTTS instance
func NewStreamingTTS(
	provider tts.Provider,
	logger zerolog.Logger,
	config *StreamingTTSConfig,
) *StreamingTTS {
	if config == nil {
		config = DefaultStreamingTTSConfig()
	}

	return &StreamingTTS{
		ttsProvider:      provider,
		logger:           logger.With().Str("component", "streaming-tts").Logger(),
		minChunkSize:     config.MinChunkSize,
		maxBufferSize:    config.MaxBufferSize,
		flushTimeout:     config.FlushTimeout,
		commaBreakLength: config.CommaBreakLength,
		voiceID:          config.VoiceID,
	}
}

// SetCallbacks configures the audio output and state change callbacks
func (s *StreamingTTS) SetCallbacks(
	onAudio func(audio []byte, format string, phonemes []tts.Phoneme),
	onStart func(),
	onStop func(),
) {
	s.onAudio = onAudio
	s.onStart = onStart
	s.onStop = onStop
}

// SetVoice sets the voice ID for TTS
func (s *StreamingTTS) SetVoice(voiceID string) {
	s.mu.Lock()
	s.voiceID = voiceID
	s.mu.Unlock()
}

// ResponseChunk represents a chunk from an LLM streaming response
type ResponseChunk struct {
	Text    string // Text content of this chunk
	IsFinal bool   // True if this is the last chunk
}

// HandleStreamingResponse processes a channel of response chunks and sends them to TTS.
// It runs concurrently: receiving chunks, buffering until sentence boundaries,
// and sending complete sentences to TTS while continuing to receive.
func (s *StreamingTTS) HandleStreamingResponse(ctx context.Context, chunks <-chan ResponseChunk) {
	// Create cancelable context for this streaming session
	streamCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.cancelFn = cancel
	s.buffer.Reset()
	s.lastChunkTime = time.Now()
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.cancelFn = nil
		s.mu.Unlock()
		cancel()
	}()

	// Channel for sentences ready to speak
	sentenceCh := make(chan string, 10)

	// Start speaker goroutine - speaks sentences as they arrive
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.speakSentences(streamCtx, sentenceCh)
	}()

	// Start timeout monitor - flushes buffer after silence
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.monitorFlushTimeout(streamCtx, sentenceCh)
	}()

	// Process incoming chunks
	for {
		select {
		case <-streamCtx.Done():
			s.logger.Debug().Msg("Streaming context canceled")
			close(sentenceCh)
			wg.Wait()
			return

		case chunk, ok := <-chunks:
			if !ok {
				// Channel closed - flush remaining buffer
				s.flushRemaining(sentenceCh)
				close(sentenceCh)
				wg.Wait()
				return
			}

			s.processChunk(streamCtx, chunk, sentenceCh)
		}
	}
}

// processChunk adds a chunk to the buffer and extracts complete sentences
func (s *StreamingTTS) processChunk(ctx context.Context, chunk ResponseChunk, sentenceCh chan<- string) {
	s.mu.Lock()
	s.buffer.WriteString(chunk.Text)
	s.lastChunkTime = time.Now()
	bufferContent := s.buffer.String()
	s.mu.Unlock()

	// Check for complete sentences
	sentences := s.extractCompleteSentences(bufferContent)
	if len(sentences) > 0 {
		s.mu.Lock()
		// Keep the incomplete portion in buffer
		lastSentenceEnd := 0
		for _, sent := range sentences {
			lastSentenceEnd = strings.LastIndex(bufferContent, sent) + len(sent)
		}
		remaining := bufferContent[lastSentenceEnd:]
		s.buffer.Reset()
		s.buffer.WriteString(strings.TrimSpace(remaining))
		s.mu.Unlock()

		// Send complete sentences to speaker
		for _, sentence := range sentences {
			sentence = strings.TrimSpace(sentence)
			if sentence == "" || shouldSkipForTTS(sentence) {
				continue
			}
			select {
			case sentenceCh <- sentence:
				s.logger.Debug().Str("sentence", truncate(sentence, 50)).Msg("Sent sentence to TTS")
			case <-ctx.Done():
				return
			}
		}
	}

	// Force flush if buffer is too large
	s.mu.Lock()
	if s.buffer.Len() >= s.maxBufferSize {
		content := s.buffer.String()
		s.buffer.Reset()
		s.mu.Unlock()

		if content != "" && !shouldSkipForTTS(content) {
			select {
			case sentenceCh <- content:
				s.logger.Debug().Str("text", truncate(content, 50)).Msg("Force flushed large buffer")
			case <-ctx.Done():
			}
		}
	} else {
		s.mu.Unlock()
	}

	// Handle final chunk
	if chunk.IsFinal {
		s.flushRemaining(sentenceCh)
	}
}

// extractCompleteSentences extracts sentences that end with sentence terminators.
// It handles period, question mark, exclamation mark, and newlines as strong breaks.
// Commas are treated as breakpoints only when the accumulated text exceeds commaBreakLength.
func (s *StreamingTTS) extractCompleteSentences(text string) []string {
	if len(text) < s.minChunkSize {
		return nil
	}

	var sentences []string
	var current strings.Builder

	runes := []rune(text)
	for i, r := range runes {
		current.WriteRune(r)
		currentLen := current.Len()

		// Check for sentence boundary (strong terminators: . ! ? \n)
		if isSentenceTerminator(r) {
			// Verify it's actually end of sentence (not abbreviation)
			if i < len(runes)-1 {
				next := runes[i+1]
				// Sentence ends if followed by space/newline and the next char is uppercase or end
				if unicode.IsSpace(next) {
					// Look ahead for uppercase letter
					for j := i + 2; j < len(runes); j++ {
						if unicode.IsLetter(runes[j]) {
							if unicode.IsUpper(runes[j]) {
								sentences = append(sentences, current.String())
								current.Reset()
							}
							break
						} else if !unicode.IsSpace(runes[j]) {
							break
						}
					}
				}
			} else {
				// End of text - this is a complete sentence
				sentences = append(sentences, current.String())
				current.Reset()
			}
		} else if r == ',' && currentLen >= s.commaBreakLength {
			// Comma acts as breakpoint when we have enough accumulated text
			// This creates natural pauses at clause boundaries for better TTS flow
			if i < len(runes)-1 && unicode.IsSpace(runes[i+1]) {
				sentences = append(sentences, current.String())
				current.Reset()
			}
		}
	}

	return sentences
}

// isSentenceTerminator checks if a rune ends a sentence
func isSentenceTerminator(r rune) bool {
	return r == '.' || r == '!' || r == '?' || r == '\n'
}

// flushRemaining sends any remaining buffer content to TTS
func (s *StreamingTTS) flushRemaining(sentenceCh chan<- string) {
	s.mu.Lock()
	remaining := strings.TrimSpace(s.buffer.String())
	s.buffer.Reset()
	s.mu.Unlock()

	if remaining != "" && !shouldSkipForTTS(remaining) {
		select {
		case sentenceCh <- remaining:
			s.logger.Debug().Str("text", truncate(remaining, 50)).Msg("Flushed remaining buffer")
		default:
		}
	}
}

// monitorFlushTimeout periodically checks if buffer should be flushed due to timeout
func (s *StreamingTTS) monitorFlushTimeout(ctx context.Context, sentenceCh chan<- string) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			bufferLen := s.buffer.Len()
			timeSinceChunk := time.Since(s.lastChunkTime)
			s.mu.Unlock()

			// Flush if we have content and haven't received chunks recently
			if bufferLen >= s.minChunkSize && timeSinceChunk >= s.flushTimeout {
				s.mu.Lock()
				content := s.buffer.String()
				s.buffer.Reset()
				s.lastChunkTime = time.Now() // Reset to avoid repeated flush
				s.mu.Unlock()

				if content != "" && !shouldSkipForTTS(content) {
					select {
					case sentenceCh <- content:
						s.logger.Debug().Str("text", truncate(content, 50)).Msg("Timeout flushed buffer")
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}
}

// speakSentences receives sentences and sends them to TTS
func (s *StreamingTTS) speakSentences(ctx context.Context, sentenceCh <-chan string) {
	first := true

	for {
		select {
		case <-ctx.Done():
			if s.isSpeaking && s.onStop != nil {
				s.onStop()
			}
			return

		case sentence, ok := <-sentenceCh:
			if !ok {
				// Channel closed
				if s.isSpeaking && s.onStop != nil {
					s.onStop()
				}
				return
			}

			// Notify start on first sentence
			if first && s.onStart != nil {
				s.onStart()
				s.isSpeaking = true
				first = false
			}

			// Synthesize and emit audio
			s.synthesizeSentence(ctx, sentence)
		}
	}
}

// synthesizeSentence converts a sentence to audio and emits it
func (s *StreamingTTS) synthesizeSentence(ctx context.Context, sentence string) {
	if s.ttsProvider == nil {
		s.logger.Warn().Msg("No TTS provider configured")
		return
	}

	s.mu.Lock()
	voiceID := s.voiceID
	s.mu.Unlock()

	s.logger.Debug().
		Str("voice", voiceID).
		Int("len", len(sentence)).
		Msg("Synthesizing sentence")

	resp, err := s.ttsProvider.Synthesize(ctx, &tts.SynthesizeRequest{
		Text:         sentence,
		VoiceID:      voiceID,
		WithPhonemes: true,
	})

	if err != nil {
		if ctx.Err() != nil {
			s.logger.Debug().Msg("Synthesis canceled")
			return
		}
		s.logger.Error().Err(err).Msg("TTS synthesis failed")
		return
	}

	if len(resp.Audio) > 0 && s.onAudio != nil {
		s.onAudio(resp.Audio, resp.Format, resp.Phonemes)
	}
}

// Cancel stops the current streaming session
func (s *StreamingTTS) Cancel() {
	s.mu.Lock()
	cancel := s.cancelFn
	s.mu.Unlock()

	if cancel != nil {
		cancel()
		s.logger.Debug().Msg("Streaming TTS canceled")
	}
}

// IsSpeaking returns whether TTS is currently speaking
func (s *StreamingTTS) IsSpeaking() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isSpeaking
}

// truncate shortens a string for logging
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
