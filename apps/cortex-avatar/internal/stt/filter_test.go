package stt

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestNewSTTFilter_DefaultFillerWords(t *testing.T) {
	f := NewSTTFilter(nil)

	words := f.GetFillerWords()
	if len(words) == 0 {
		t.Error("expected default filler words, got empty list")
	}

	// Check some expected defaults
	wordSet := make(map[string]struct{})
	for _, w := range words {
		wordSet[w] = struct{}{}
	}

	expected := []string{"um", "uh", "like", "you know", "basically"}
	for _, e := range expected {
		if _, ok := wordSet[e]; !ok {
			t.Errorf("expected default filler word %q not found", e)
		}
	}
}

func TestNewSTTFilter_CustomFillerWords(t *testing.T) {
	custom := []string{"foo", "bar", "baz"}
	f := NewSTTFilter(custom)

	words := f.GetFillerWords()
	if len(words) != 3 {
		t.Errorf("expected 3 filler words, got %d", len(words))
	}
}

func TestSTTFilter_Clean_RemovesFillers(t *testing.T) {
	f := NewSTTFilter(nil)

	tests := []struct {
		name        string
		input       string
		wantCleaned string
		wantHas     bool
	}{
		{
			name:        "simple filler removal",
			input:       "um what is the weather",
			wantCleaned: "what is the weather",
			wantHas:     true,
		},
		{
			name:        "multiple fillers",
			input:       "um like what is uh the weather you know",
			wantCleaned: "what is the weather",
			wantHas:     true,
		},
		{
			name:        "filler only",
			input:       "um uh like",
			wantCleaned: "",
			wantHas:     false,
		},
		{
			name:        "empty string",
			input:       "",
			wantCleaned: "",
			wantHas:     false,
		},
		{
			name:        "no fillers",
			input:       "what is the weather today",
			wantCleaned: "what is the weather today",
			wantHas:     true,
		},
		{
			name:        "case insensitive",
			input:       "UM what is UH the weather",
			wantCleaned: "what is the weather",
			wantHas:     true,
		},
		{
			name:        "filler in middle of word preserved",
			input:       "I like umbrella",
			wantCleaned: "I umbrella",
			wantHas:     true,
		},
		{
			name:        "multi-word filler",
			input:       "so you know I think basically it works",
			wantCleaned: "I think it works",
			wantHas:     true,
		},
		{
			name:        "extra whitespace normalized",
			input:       "um   what   is   the   weather",
			wantCleaned: "what is the weather",
			wantHas:     true,
		},
		{
			name:        "punctuation only after cleaning",
			input:       "um, uh.",
			wantCleaned: "",
			wantHas:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleaned, has := f.Clean(tt.input)
			if cleaned != tt.wantCleaned {
				t.Errorf("Clean(%q) = %q, want %q", tt.input, cleaned, tt.wantCleaned)
			}
			if has != tt.wantHas {
				t.Errorf("Clean(%q) hasMeaningful = %v, want %v", tt.input, has, tt.wantHas)
			}
		})
	}
}

func TestSTTFilter_IsFillerOnly(t *testing.T) {
	f := NewSTTFilter(nil)

	tests := []struct {
		input string
		want  bool
	}{
		{"um uh like", true},
		{"um hello", false},
		{"hello world", false},
		{"", true},
		{"   ", true},
		{"um, uh.", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := f.IsFillerOnly(tt.input)
			if got != tt.want {
				t.Errorf("IsFillerOnly(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSTTFilter_AddRemoveFillerWord(t *testing.T) {
	f := NewSTTFilter([]string{"um"})

	// Initially only "um" is filtered
	cleaned, _ := f.Clean("um foo bar")
	if cleaned != "foo bar" {
		t.Errorf("expected 'foo bar', got %q", cleaned)
	}

	// "baz" is not filtered
	cleaned, _ = f.Clean("baz foo bar")
	if cleaned != "baz foo bar" {
		t.Errorf("expected 'baz foo bar', got %q", cleaned)
	}

	// Add "baz" as filler
	f.AddFillerWord("baz")
	cleaned, _ = f.Clean("baz foo bar")
	if cleaned != "foo bar" {
		t.Errorf("after AddFillerWord, expected 'foo bar', got %q", cleaned)
	}

	// Remove "um"
	f.RemoveFillerWord("um")
	cleaned, _ = f.Clean("um foo bar")
	if cleaned != "um foo bar" {
		t.Errorf("after RemoveFillerWord, expected 'um foo bar', got %q", cleaned)
	}
}

func TestSTTFilter_SetFillerWords(t *testing.T) {
	f := NewSTTFilter(nil)

	// Replace with custom list
	f.SetFillerWords([]string{"xyz", "abc"})

	words := f.GetFillerWords()
	if len(words) != 2 {
		t.Errorf("expected 2 words after SetFillerWords, got %d", len(words))
	}

	// Old defaults should no longer work
	cleaned, has := f.Clean("um what is the weather")
	if cleaned != "um what is the weather" {
		t.Errorf("expected 'um' to not be filtered, got %q", cleaned)
	}
	if !has {
		t.Error("expected hasMeaningful=true")
	}

	// New fillers should work
	cleaned, _ = f.Clean("xyz what abc is the weather")
	if cleaned != "what is the weather" {
		t.Errorf("expected 'what is the weather', got %q", cleaned)
	}
}

func TestSTTFilter_FilterResponse(t *testing.T) {
	f := NewSTTFilter(nil)

	tests := []struct {
		name     string
		input    *TranscribeResponse
		wantOK   bool
		wantText string
	}{
		{
			name:     "nil response",
			input:    nil,
			wantOK:   false,
			wantText: "",
		},
		{
			name:     "meaningful content",
			input:    &TranscribeResponse{Text: "um what is the weather"},
			wantOK:   true,
			wantText: "what is the weather",
		},
		{
			name:     "filler only",
			input:    &TranscribeResponse{Text: "um uh like"},
			wantOK:   false,
			wantText: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok := f.FilterResponse(tt.input)
			if ok != tt.wantOK {
				t.Errorf("FilterResponse() = %v, want %v", ok, tt.wantOK)
			}
			if tt.input != nil && tt.input.Text != tt.wantText {
				t.Errorf("FilterResponse() text = %q, want %q", tt.input.Text, tt.wantText)
			}
		})
	}
}

func TestSTTFilter_EmptyFillerList(t *testing.T) {
	f := NewSTTFilter([]string{})

	// With no fillers, nothing should be removed
	cleaned, has := f.Clean("um what is the weather")
	if cleaned != "um what is the weather" {
		t.Errorf("expected no filtering with empty list, got %q", cleaned)
	}
	if !has {
		t.Error("expected hasMeaningful=true")
	}
}

func TestSTTFilter_ConcurrentAccess(t *testing.T) {
	f := NewSTTFilter(nil)

	// Test concurrent reads and writes
	done := make(chan bool)

	// Concurrent reads
	for range 10 {
		go func() {
			for range 100 {
				f.Clean("um what is the weather")
				f.GetFillerWords()
			}
			done <- true
		}()
	}

	// Concurrent writes
	for i := range 5 {
		go func(n int) {
			for range 50 {
				f.AddFillerWord("test" + string(rune('a'+n)))
				f.RemoveFillerWord("test" + string(rune('a'+n)))
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for range 15 {
		<-done
	}
}

// =============================================================================
// FragmentBuffer Tests
// =============================================================================

func TestNewFragmentBuffer_Defaults(t *testing.T) {
	fb := NewFragmentBuffer(nil)

	cfg := fb.GetConfig()
	if cfg.TimeoutMs != 500 {
		t.Errorf("expected default TimeoutMs=500, got %d", cfg.TimeoutMs)
	}
	if cfg.MinWordCount != 2 {
		t.Errorf("expected default MinWordCount=2, got %d", cfg.MinWordCount)
	}
}

func TestNewFragmentBuffer_CustomConfig(t *testing.T) {
	fb := NewFragmentBuffer(&FragmentBufferConfig{
		TimeoutMs:    1000,
		MinWordCount: 5,
	})

	cfg := fb.GetConfig()
	if cfg.TimeoutMs != 1000 {
		t.Errorf("expected TimeoutMs=1000, got %d", cfg.TimeoutMs)
	}
	if cfg.MinWordCount != 5 {
		t.Errorf("expected MinWordCount=5, got %d", cfg.MinWordCount)
	}
}

func TestFragmentBuffer_Add(t *testing.T) {
	fb := NewFragmentBuffer(nil)

	tests := []struct {
		name      string
		fragment  string
		wantAdded bool
		wantPeek  string
		wantWords int
	}{
		{"add single word", "hello", true, "hello", 1},
		{"add second fragment", "world", true, "hello world", 2},
		{"empty string", "", false, "hello world", 2},
		{"whitespace only", "   ", false, "hello world", 2},
		{"add with trim", "  test  ", true, "hello world test", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			added := fb.Add(tt.fragment)
			if added != tt.wantAdded {
				t.Errorf("Add(%q) = %v, want %v", tt.fragment, added, tt.wantAdded)
			}
			if got := fb.Peek(); got != tt.wantPeek {
				t.Errorf("Peek() = %q, want %q", got, tt.wantPeek)
			}
			if got := fb.WordCount(); got != tt.wantWords {
				t.Errorf("WordCount() = %d, want %d", got, tt.wantWords)
			}
		})
	}
}

func TestFragmentBuffer_ShouldSend_MinWordCount(t *testing.T) {
	fb := NewFragmentBuffer(&FragmentBufferConfig{
		TimeoutMs:    10000, // Long timeout so we test word count only
		MinWordCount: 2,
	})

	// Empty buffer - should not send
	if fb.ShouldSend() {
		t.Error("empty buffer should not trigger send")
	}

	// Add one word - still below threshold
	fb.Add("hello")
	if fb.ShouldSend() {
		t.Error("single word should not trigger send")
	}

	// Add second word - meets threshold
	fb.Add("world")
	if !fb.ShouldSend() {
		t.Error("two words should trigger send")
	}
}

func TestFragmentBuffer_ShouldSend_Timeout(t *testing.T) {
	// Create mock time
	var mockTime int64 = 1000000000 // 1 second in nanoseconds

	fb := NewFragmentBuffer(&FragmentBufferConfig{
		TimeoutMs:    500,
		MinWordCount: 10, // High threshold so we test timeout only
	})

	// Override time provider
	fb.timeProvider = func() int64 {
		return atomic.LoadInt64(&mockTime)
	}

	// Add a fragment (below word threshold)
	fb.Add("hello")

	// Immediately after - should not send (timeout not elapsed)
	if fb.ShouldSend() {
		t.Error("should not send immediately after adding")
	}

	// Advance time by 400ms - still not enough
	atomic.StoreInt64(&mockTime, 1400000000) // 1.4 seconds
	if fb.ShouldSend() {
		t.Error("should not send before timeout")
	}

	// Advance time past 500ms timeout
	atomic.StoreInt64(&mockTime, 1600000000) // 1.6 seconds (600ms elapsed)
	if !fb.ShouldSend() {
		t.Error("should send after timeout elapsed")
	}
}

func TestFragmentBuffer_Flush(t *testing.T) {
	fb := NewFragmentBuffer(nil)

	fb.Add("hello")
	fb.Add("world")

	result := fb.Flush()
	if result != "hello world" {
		t.Errorf("Flush() = %q, want %q", result, "hello world")
	}

	// Buffer should be empty after flush
	if !fb.IsEmpty() {
		t.Error("buffer should be empty after Flush()")
	}
	if fb.WordCount() != 0 {
		t.Errorf("word count should be 0 after Flush(), got %d", fb.WordCount())
	}

	// Flush on empty buffer returns empty string
	result = fb.Flush()
	if result != "" {
		t.Errorf("Flush() on empty buffer = %q, want empty", result)
	}
}

func TestFragmentBuffer_Peek(t *testing.T) {
	fb := NewFragmentBuffer(nil)

	fb.Add("hello")
	fb.Add("world")

	// Peek should return content without clearing
	peek1 := fb.Peek()
	peek2 := fb.Peek()

	if peek1 != peek2 {
		t.Error("Peek() should not modify buffer")
	}
	if peek1 != "hello world" {
		t.Errorf("Peek() = %q, want %q", peek1, "hello world")
	}
}

func TestFragmentBuffer_IsEmpty(t *testing.T) {
	fb := NewFragmentBuffer(nil)

	if !fb.IsEmpty() {
		t.Error("new buffer should be empty")
	}

	fb.Add("test")
	if fb.IsEmpty() {
		t.Error("buffer with content should not be empty")
	}

	fb.Flush()
	if !fb.IsEmpty() {
		t.Error("buffer after Flush should be empty")
	}
}

func TestFragmentBuffer_SetTimeout(t *testing.T) {
	fb := NewFragmentBuffer(nil)

	fb.SetTimeout(1000)
	cfg := fb.GetConfig()
	if cfg.TimeoutMs != 1000 {
		t.Errorf("SetTimeout(1000) failed, got %d", cfg.TimeoutMs)
	}
}

func TestFragmentBuffer_SetMinWordCount(t *testing.T) {
	fb := NewFragmentBuffer(nil)

	fb.SetMinWordCount(5)
	cfg := fb.GetConfig()
	if cfg.MinWordCount != 5 {
		t.Errorf("SetMinWordCount(5) failed, got %d", cfg.MinWordCount)
	}

	// Invalid value should be ignored
	fb.SetMinWordCount(0)
	cfg = fb.GetConfig()
	if cfg.MinWordCount != 5 {
		t.Errorf("SetMinWordCount(0) should be ignored, got %d", cfg.MinWordCount)
	}
}

func TestFragmentBuffer_ConcurrentAccess(t *testing.T) {
	fb := NewFragmentBuffer(&FragmentBufferConfig{
		TimeoutMs:    100,
		MinWordCount: 5,
	})

	var wg sync.WaitGroup

	// Concurrent adds
	for i := range 10 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for range 50 {
				fb.Add("word")
				fb.ShouldSend()
				fb.Peek()
				fb.WordCount()
			}
		}(i)
	}

	// Concurrent config changes
	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 20 {
				fb.SetTimeout(200)
				fb.SetMinWordCount(3)
				fb.GetConfig()
			}
		}()
	}

	wg.Wait()
}

func TestCountWords(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"   ", 0},
		{"hello", 1},
		{"hello world", 2},
		{"  hello   world  ", 2},
		{"one two three four", 4},
		{"word", 1},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := countWords(tt.input)
			if got != tt.want {
				t.Errorf("countWords(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestFragmentBuffer_AccumulatesShortUtterances(t *testing.T) {
	// Test the main use case: accumulating short utterances until threshold
	var mockTime int64 = 1000000000

	fb := NewFragmentBuffer(&FragmentBufferConfig{
		TimeoutMs:    500,
		MinWordCount: 3,
	})

	fb.timeProvider = func() int64 {
		return atomic.LoadInt64(&mockTime)
	}

	// User says "um" (filtered out upstream, but could pass as "")
	// Then says "what"
	fb.Add("what")
	if fb.ShouldSend() {
		t.Error("should not send after 1 word")
	}

	// Then says "is"
	atomic.AddInt64(&mockTime, 100000000) // 100ms later
	fb.Add("is")
	if fb.ShouldSend() {
		t.Error("should not send after 2 words")
	}

	// Then says "the weather" (2 words) - now at 4 words total
	atomic.AddInt64(&mockTime, 150000000) // 150ms later
	fb.Add("the weather")
	if !fb.ShouldSend() {
		t.Error("should send after 4 words (above threshold)")
	}

	result := fb.Flush()
	if result != "what is the weather" {
		t.Errorf("expected 'what is the weather', got %q", result)
	}
}

func TestFragmentBuffer_SendsAfterPause(t *testing.T) {
	// Test: even with 1 word, send after pause (timeout)
	var mockTime int64 = 1000000000

	fb := NewFragmentBuffer(&FragmentBufferConfig{
		TimeoutMs:    500,
		MinWordCount: 3,
	})

	fb.timeProvider = func() int64 {
		return atomic.LoadInt64(&mockTime)
	}

	// User says just "hi"
	fb.Add("hi")
	if fb.ShouldSend() {
		t.Error("should not send single word immediately")
	}

	// User pauses for 500ms
	atomic.AddInt64(&mockTime, 500000000) // 500ms later
	if !fb.ShouldSend() {
		t.Error("should send after pause timeout even with 1 word")
	}
}

func TestDefaultFragmentConfig(t *testing.T) {
	cfg := DefaultFragmentConfig()
	if cfg.TimeoutMs != 500 {
		t.Errorf("DefaultFragmentConfig TimeoutMs = %d, want 500", cfg.TimeoutMs)
	}
	if cfg.MinWordCount != 2 {
		t.Errorf("DefaultFragmentConfig MinWordCount = %d, want 2", cfg.MinWordCount)
	}
}
