package bridge

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/normanking/cortexavatar/internal/tts"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTTSProvider implements tts.Provider for testing
type mockTTSProvider struct {
	synthesizeCalls []string
	mu              sync.Mutex
	delay           time.Duration
	failOn          map[string]error
}

func newMockTTSProvider() *mockTTSProvider {
	return &mockTTSProvider{
		synthesizeCalls: make([]string, 0),
		failOn:          make(map[string]error),
	}
}

func (m *mockTTSProvider) Name() string { return "mock" }

func (m *mockTTSProvider) Synthesize(ctx context.Context, req *tts.SynthesizeRequest) (*tts.SynthesizeResponse, error) {
	m.mu.Lock()
	m.synthesizeCalls = append(m.synthesizeCalls, req.Text)
	failErr := m.failOn[req.Text]
	m.mu.Unlock()

	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if failErr != nil {
		return nil, failErr
	}

	return &tts.SynthesizeResponse{
		Audio:      []byte("audio-data"),
		Format:     "pcm",
		SampleRate: 22050,
		VoiceID:    req.VoiceID,
		Provider:   "mock",
	}, nil
}

func (m *mockTTSProvider) SynthesizeStream(ctx context.Context, req *tts.SynthesizeRequest) (<-chan *tts.AudioChunk, error) {
	return nil, nil
}

func (m *mockTTSProvider) ListVoices(ctx context.Context) ([]tts.Voice, error) {
	return nil, nil
}

func (m *mockTTSProvider) Health(ctx context.Context) error {
	return nil
}

func (m *mockTTSProvider) Capabilities() tts.ProviderCapabilities {
	return tts.ProviderCapabilities{SupportsStreaming: true}
}

func (m *mockTTSProvider) getSynthesizeCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.synthesizeCalls))
	copy(result, m.synthesizeCalls)
	return result
}

// TestStreamingTTS_BasicSentenceExtraction tests that complete sentences are extracted
func TestStreamingTTS_BasicSentenceExtraction(t *testing.T) {
	logger := zerolog.Nop()
	provider := newMockTTSProvider()

	config := &StreamingTTSConfig{
		MinChunkSize:  5,
		MaxBufferSize: 500,
		FlushTimeout:  100 * time.Millisecond,
		VoiceID:       "test-voice",
	}

	stts := NewStreamingTTS(provider, logger, config)

	var audioReceived [][]byte
	var mu sync.Mutex

	stts.SetCallbacks(
		func(audio []byte, format string, phonemes []tts.Phoneme) {
			mu.Lock()
			audioReceived = append(audioReceived, audio)
			mu.Unlock()
		},
		func() {},
		func() {},
	)

	chunks := make(chan ResponseChunk)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go stts.HandleStreamingResponse(ctx, chunks)

	// Send chunks that form two sentences
	chunks <- ResponseChunk{Text: "Hello there. ", IsFinal: false}
	chunks <- ResponseChunk{Text: "How are you?", IsFinal: true}
	close(chunks)

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	calls := provider.getSynthesizeCalls()
	assert.GreaterOrEqual(t, len(calls), 1, "Should have synthesized at least one sentence")

	// Verify sentences were extracted
	found := false
	for _, call := range calls {
		if call == "Hello there." || call == "How are you?" {
			found = true
			break
		}
	}
	assert.True(t, found, "Should have synthesized a complete sentence")
}

// TestStreamingTTS_BufferAccumulatesUntilBreakpoint tests buffering behavior
func TestStreamingTTS_BufferAccumulatesUntilBreakpoint(t *testing.T) {
	logger := zerolog.Nop()
	provider := newMockTTSProvider()

	config := &StreamingTTSConfig{
		MinChunkSize:  10,
		MaxBufferSize: 500,
		FlushTimeout:  1 * time.Second, // Long timeout to test accumulation
		VoiceID:       "test-voice",
	}

	stts := NewStreamingTTS(provider, logger, config)
	stts.SetCallbacks(nil, nil, nil)

	chunks := make(chan ResponseChunk)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go stts.HandleStreamingResponse(ctx, chunks)

	// Send word-by-word chunks (no sentence terminator yet)
	chunks <- ResponseChunk{Text: "The ", IsFinal: false}
	chunks <- ResponseChunk{Text: "quick ", IsFinal: false}
	chunks <- ResponseChunk{Text: "brown ", IsFinal: false}

	// Brief wait - should NOT have synthesized yet (no sentence boundary)
	time.Sleep(100 * time.Millisecond)
	calls := provider.getSynthesizeCalls()
	assert.Equal(t, 0, len(calls), "Should not synthesize without sentence boundary")

	// Now complete the sentence
	chunks <- ResponseChunk{Text: "fox. ", IsFinal: false}
	chunks <- ResponseChunk{Text: "The end.", IsFinal: true}
	close(chunks)

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	calls = provider.getSynthesizeCalls()
	assert.GreaterOrEqual(t, len(calls), 1, "Should have synthesized after sentence boundary")
}

// TestStreamingTTS_ConcurrentReceiveAndSpeak tests that we can receive while speaking
func TestStreamingTTS_ConcurrentReceiveAndSpeak(t *testing.T) {
	logger := zerolog.Nop()
	provider := newMockTTSProvider()
	provider.delay = 50 * time.Millisecond // Simulate TTS taking time

	config := &StreamingTTSConfig{
		MinChunkSize:  5,
		MaxBufferSize: 500,
		FlushTimeout:  50 * time.Millisecond,
		VoiceID:       "test-voice",
	}

	stts := NewStreamingTTS(provider, logger, config)

	startCalled := false
	stopCalled := false
	audioCount := 0
	var mu sync.Mutex

	stts.SetCallbacks(
		func(audio []byte, format string, phonemes []tts.Phoneme) {
			mu.Lock()
			audioCount++
			mu.Unlock()
		},
		func() {
			mu.Lock()
			startCalled = true
			mu.Unlock()
		},
		func() {
			mu.Lock()
			stopCalled = true
			mu.Unlock()
		},
	)

	chunks := make(chan ResponseChunk, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go stts.HandleStreamingResponse(ctx, chunks)

	// Send multiple sentences rapidly
	chunks <- ResponseChunk{Text: "First sentence. ", IsFinal: false}
	chunks <- ResponseChunk{Text: "Second sentence. ", IsFinal: false}
	chunks <- ResponseChunk{Text: "Third sentence.", IsFinal: true}
	close(chunks)

	// Wait for all processing
	time.Sleep(1 * time.Second)

	mu.Lock()
	assert.True(t, startCalled, "onStart should have been called")
	assert.True(t, stopCalled, "onStop should have been called")
	assert.GreaterOrEqual(t, audioCount, 1, "Should have received audio")
	mu.Unlock()

	calls := provider.getSynthesizeCalls()
	assert.GreaterOrEqual(t, len(calls), 2, "Should have synthesized multiple sentences")
}

// TestStreamingTTS_Cancellation tests that cancellation stops processing
func TestStreamingTTS_Cancellation(t *testing.T) {
	logger := zerolog.Nop()
	provider := newMockTTSProvider()
	provider.delay = 500 * time.Millisecond // Slow TTS

	config := &StreamingTTSConfig{
		MinChunkSize:  5,
		MaxBufferSize: 500,
		FlushTimeout:  50 * time.Millisecond,
		VoiceID:       "test-voice",
	}

	stts := NewStreamingTTS(provider, logger, config)

	stopCalled := make(chan struct{})
	stts.SetCallbacks(
		func(audio []byte, format string, phonemes []tts.Phoneme) {},
		func() {},
		func() {
			close(stopCalled)
		},
	)

	chunks := make(chan ResponseChunk, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go stts.HandleStreamingResponse(ctx, chunks)

	// Send a sentence
	chunks <- ResponseChunk{Text: "Hello world.", IsFinal: false}

	// Wait a bit then cancel
	time.Sleep(100 * time.Millisecond)
	stts.Cancel()

	// Verify stop was called
	select {
	case <-stopCalled:
		// Success
	case <-time.After(1 * time.Second):
		// May not be called if cancel happened before speaking started
	}

	// Additional chunks should be ignored
	select {
	case chunks <- ResponseChunk{Text: "This should be ignored.", IsFinal: true}:
	default:
	}
	close(chunks)

	// Verify only initial sentence was attempted
	time.Sleep(200 * time.Millisecond)
	calls := provider.getSynthesizeCalls()
	assert.LessOrEqual(t, len(calls), 1, "Should not process chunks after cancellation")
}

// TestStreamingTTS_FlushTimeout tests that buffer flushes after timeout
func TestStreamingTTS_FlushTimeout(t *testing.T) {
	logger := zerolog.Nop()
	provider := newMockTTSProvider()

	config := &StreamingTTSConfig{
		MinChunkSize:  5,
		MaxBufferSize: 500,
		FlushTimeout:  150 * time.Millisecond,
		VoiceID:       "test-voice",
	}

	stts := NewStreamingTTS(provider, logger, config)
	stts.SetCallbacks(nil, nil, nil)

	chunks := make(chan ResponseChunk)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go stts.HandleStreamingResponse(ctx, chunks)

	// Send incomplete sentence (no terminator)
	chunks <- ResponseChunk{Text: "Hello world without period", IsFinal: false}

	// Wait for flush timeout
	time.Sleep(400 * time.Millisecond)

	calls := provider.getSynthesizeCalls()
	assert.GreaterOrEqual(t, len(calls), 1, "Should have flushed after timeout")
	if len(calls) > 0 {
		assert.Contains(t, calls[0], "Hello world", "Should contain the buffered text")
	}

	close(chunks)
}

// TestStreamingTTS_MaxBufferForceFlush tests that large buffers are force-flushed
func TestStreamingTTS_MaxBufferForceFlush(t *testing.T) {
	logger := zerolog.Nop()
	provider := newMockTTSProvider()

	config := &StreamingTTSConfig{
		MinChunkSize:  5,
		MaxBufferSize: 50, // Small max to trigger force flush
		FlushTimeout:  10 * time.Second,
		VoiceID:       "test-voice",
	}

	stts := NewStreamingTTS(provider, logger, config)
	stts.SetCallbacks(nil, nil, nil)

	chunks := make(chan ResponseChunk)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go stts.HandleStreamingResponse(ctx, chunks)

	// Send a long chunk without sentence boundary
	longText := "This is a very long text without any sentence boundary that exceeds the maximum buffer size"
	chunks <- ResponseChunk{Text: longText, IsFinal: true}
	close(chunks)

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	calls := provider.getSynthesizeCalls()
	assert.GreaterOrEqual(t, len(calls), 1, "Should have force-flushed large buffer")
}

// TestStreamingTTS_SkipsCodeBlocks tests that code blocks are filtered out
func TestStreamingTTS_SkipsCodeBlocks(t *testing.T) {
	logger := zerolog.Nop()
	provider := newMockTTSProvider()

	config := &StreamingTTSConfig{
		MinChunkSize:  5,
		MaxBufferSize: 500,
		FlushTimeout:  100 * time.Millisecond,
		VoiceID:       "test-voice",
	}

	stts := NewStreamingTTS(provider, logger, config)
	stts.SetCallbacks(nil, nil, nil)

	chunks := make(chan ResponseChunk)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go stts.HandleStreamingResponse(ctx, chunks)

	// Send a code block (should be skipped)
	chunks <- ResponseChunk{Text: "```python\nprint('hello')\n```", IsFinal: false}
	chunks <- ResponseChunk{Text: "Normal text here.", IsFinal: true}
	close(chunks)

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	calls := provider.getSynthesizeCalls()
	// Should only synthesize the normal text, not the code block
	for _, call := range calls {
		assert.NotContains(t, call, "```", "Should not synthesize code blocks")
	}
}

// TestStreamingTTS_ExtractCompleteSentences tests sentence extraction logic
func TestStreamingTTS_ExtractCompleteSentences(t *testing.T) {
	logger := zerolog.Nop()
	stts := NewStreamingTTS(nil, logger, &StreamingTTSConfig{MinChunkSize: 5, CommaBreakLength: 40})

	tests := []struct {
		name     string
		input    string
		expected int // number of complete sentences
	}{
		{
			name:     "single complete sentence",
			input:    "Hello world.",
			expected: 1,
		},
		{
			name:     "two sentences",
			input:    "Hello world. How are you?",
			expected: 2,
		},
		{
			name:     "incomplete sentence",
			input:    "Hello world",
			expected: 0,
		},
		{
			name:     "exclamation",
			input:    "Hello world!",
			expected: 1,
		},
		{
			name:     "question",
			input:    "How are you?",
			expected: 1,
		},
		{
			name:     "sentence followed by incomplete",
			input:    "First sentence. Second part",
			expected: 1,
		},
		{
			name:     "short text under threshold",
			input:    "Hi.",
			expected: 0, // Below minChunkSize
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sentences := stts.extractCompleteSentences(tt.input)
			assert.Equal(t, tt.expected, len(sentences), "Unexpected number of sentences for: %s", tt.input)
		})
	}
}

// TestStreamingTTS_CommaBreakpoint tests comma breakpoint detection with length threshold
func TestStreamingTTS_CommaBreakpoint(t *testing.T) {
	logger := zerolog.Nop()

	tests := []struct {
		name             string
		input            string
		commaBreakLength int
		minChunkSize     int
		expected         int
		expectedTexts    []string
	}{
		{
			name:             "comma with sufficient length triggers break",
			input:            "This is a very long clause that exceeds the threshold, and this is another clause",
			commaBreakLength: 40,
			minChunkSize:     5,
			expected:         1,
			expectedTexts:    []string{"This is a very long clause that exceeds the threshold,"},
		},
		{
			name:             "short comma clause does not trigger break",
			input:            "Hello, world",
			commaBreakLength: 40,
			minChunkSize:     5,
			expected:         0,
			expectedTexts:    nil,
		},
		{
			name:             "comma without trailing space does not trigger",
			input:            "This is exactly forty characters long,continued without space",
			commaBreakLength: 30,
			minChunkSize:     5,
			expected:         0,
			expectedTexts:    nil,
		},
		{
			name:             "multiple comma breaks with long clauses",
			input:            "First clause that is long enough for comma break, second clause also exceeds the threshold, and more",
			commaBreakLength: 30,
			minChunkSize:     5,
			expected:         2,
		},
		{
			name:             "period followed by long comma clause",
			input:            "A long sentence that ends with period. Then this second part is also quite long with a comma, here",
			commaBreakLength: 20,
			minChunkSize:     5,
			expected:         2, // period break + comma break after long clause
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &StreamingTTSConfig{
				MinChunkSize:     tt.minChunkSize,
				CommaBreakLength: tt.commaBreakLength,
			}
			stts := NewStreamingTTS(nil, logger, config)
			sentences := stts.extractCompleteSentences(tt.input)
			assert.Equal(t, tt.expected, len(sentences), "Unexpected number of breaks for: %s", tt.input)

			if tt.expectedTexts != nil {
				for i, expected := range tt.expectedTexts {
					if i < len(sentences) {
						assert.Equal(t, expected, sentences[i], "Unexpected sentence text at index %d", i)
					}
				}
			}
		})
	}
}

// TestStreamingTTS_CommaBreakIntegration tests comma breaks in streaming context
func TestStreamingTTS_CommaBreakIntegration(t *testing.T) {
	logger := zerolog.Nop()
	provider := newMockTTSProvider()

	config := &StreamingTTSConfig{
		MinChunkSize:     5,
		MaxBufferSize:    500,
		FlushTimeout:     100 * time.Millisecond,
		VoiceID:          "test-voice",
		CommaBreakLength: 30, // Low threshold for testing
	}

	stts := NewStreamingTTS(provider, logger, config)
	stts.SetCallbacks(nil, nil, nil)

	chunks := make(chan ResponseChunk)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go stts.HandleStreamingResponse(ctx, chunks)

	// Send long clause with comma
	chunks <- ResponseChunk{Text: "This is a long clause that should trigger, ", IsFinal: false}
	chunks <- ResponseChunk{Text: "and then we continue with more text.", IsFinal: true}
	close(chunks)

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	calls := provider.getSynthesizeCalls()
	// Should have at least 2 calls: one at comma break, one at end
	assert.GreaterOrEqual(t, len(calls), 2, "Should have multiple TTS calls due to comma break")
}

// TestStreamingTTS_IsSentenceTerminator tests boundary detection
func TestStreamingTTS_IsSentenceTerminator(t *testing.T) {
	tests := []struct {
		r        rune
		expected bool
	}{
		{'.', true},
		{'!', true},
		{'?', true},
		{'\n', true},
		{',', false},
		{';', false},
		{':', false},
		{' ', false},
		{'a', false},
	}

	for _, tt := range tests {
		t.Run(string(tt.r), func(t *testing.T) {
			result := isSentenceTerminator(tt.r)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestStreamingTTS_SetVoice tests voice change
func TestStreamingTTS_SetVoice(t *testing.T) {
	logger := zerolog.Nop()
	provider := newMockTTSProvider()

	stts := NewStreamingTTS(provider, logger, nil)
	stts.SetVoice("new-voice")

	stts.mu.Lock()
	assert.Equal(t, "new-voice", stts.voiceID)
	stts.mu.Unlock()
}

// TestNewStreamingTTS_DefaultConfig tests default configuration
func TestNewStreamingTTS_DefaultConfig(t *testing.T) {
	logger := zerolog.Nop()
	stts := NewStreamingTTS(nil, logger, nil)

	require.NotNil(t, stts)
	assert.Equal(t, 20, stts.minChunkSize)
	assert.Equal(t, 200, stts.maxBufferSize)
	assert.Equal(t, 500*time.Millisecond, stts.flushTimeout)
	assert.Equal(t, 40, stts.commaBreakLength)
	assert.Equal(t, "nova", stts.voiceID)
}

// TestStreamingTTS_IsSpeaking tests speaking state
func TestStreamingTTS_IsSpeaking(t *testing.T) {
	logger := zerolog.Nop()
	provider := newMockTTSProvider()

	stts := NewStreamingTTS(provider, logger, nil)

	// Initially not speaking
	assert.False(t, stts.IsSpeaking())

	// Manually set speaking state for test
	stts.mu.Lock()
	stts.isSpeaking = true
	stts.mu.Unlock()

	assert.True(t, stts.IsSpeaking())
}
