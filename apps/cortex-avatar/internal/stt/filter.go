// Package stt provides Speech-to-Text transcription services for CortexAvatar.
package stt

import (
	"regexp"
	"strings"
	"sync"
	"time"
)

// DefaultFillerWords contains common English filler words to remove from transcripts.
var DefaultFillerWords = []string{
	"um", "uh", "uhh", "umm",
	"like", "you know", "basically",
	"actually", "literally", "so",
	"er", "ah", "hmm", "mm",
	"well", "right", "okay",
}

// STTFilter filters filler words and noise from STT transcripts.
type STTFilter struct {
	mu          sync.RWMutex
	fillerWords map[string]struct{}
	pattern     *regexp.Regexp
}

// NewSTTFilter creates a new filter with the given filler words.
// If fillerWords is nil, DefaultFillerWords is used.
func NewSTTFilter(fillerWords []string) *STTFilter {
	if fillerWords == nil {
		fillerWords = DefaultFillerWords
	}

	f := &STTFilter{
		fillerWords: make(map[string]struct{}, len(fillerWords)),
	}

	for _, word := range fillerWords {
		f.fillerWords[strings.ToLower(word)] = struct{}{}
	}

	f.buildPattern()
	return f
}

// buildPattern constructs a regex pattern from the filler words.
func (f *STTFilter) buildPattern() {
	if len(f.fillerWords) == 0 {
		f.pattern = nil
		return
	}

	// Build pattern with word boundaries
	var patterns []string
	for word := range f.fillerWords {
		// Escape special regex characters and add word boundaries
		escaped := regexp.QuoteMeta(word)
		patterns = append(patterns, `\b`+escaped+`\b`)
	}

	// Join with OR and compile (case-insensitive)
	patternStr := `(?i)(` + strings.Join(patterns, `|`) + `)`
	f.pattern = regexp.MustCompile(patternStr)
}

// AddFillerWord adds a word to the filler list.
func (f *STTFilter) AddFillerWord(word string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.fillerWords[strings.ToLower(word)] = struct{}{}
	f.buildPattern()
}

// RemoveFillerWord removes a word from the filler list.
func (f *STTFilter) RemoveFillerWord(word string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.fillerWords, strings.ToLower(word))
	f.buildPattern()
}

// SetFillerWords replaces the entire filler word list.
func (f *STTFilter) SetFillerWords(words []string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.fillerWords = make(map[string]struct{}, len(words))
	for _, word := range words {
		f.fillerWords[strings.ToLower(word)] = struct{}{}
	}
	f.buildPattern()
}

// GetFillerWords returns a copy of the current filler word list.
func (f *STTFilter) GetFillerWords() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	words := make([]string, 0, len(f.fillerWords))
	for word := range f.fillerWords {
		words = append(words, word)
	}
	return words
}

// Clean removes filler words from the transcript and normalizes whitespace.
// Returns the cleaned text and a boolean indicating if the result has meaningful content.
func (f *STTFilter) Clean(text string) (cleaned string, hasMeaningfulContent bool) {
	if text == "" {
		return "", false
	}

	f.mu.RLock()
	pattern := f.pattern
	f.mu.RUnlock()

	cleaned = text

	// Remove filler words if pattern exists
	if pattern != nil {
		cleaned = pattern.ReplaceAllString(cleaned, "")
	}

	// Normalize whitespace: collapse multiple spaces to one
	spacePattern := regexp.MustCompile(`\s+`)
	cleaned = spacePattern.ReplaceAllString(cleaned, " ")

	// Trim leading/trailing whitespace
	cleaned = strings.TrimSpace(cleaned)

	// Remove standalone punctuation that might remain
	punctPattern := regexp.MustCompile(`^[.,!?;:\s]+$`)
	if punctPattern.MatchString(cleaned) {
		cleaned = ""
	}

	hasMeaningfulContent = len(cleaned) > 0
	return cleaned, hasMeaningfulContent
}

// IsFillerOnly returns true if the text contains only filler words.
func (f *STTFilter) IsFillerOnly(text string) bool {
	cleaned, hasMeaningful := f.Clean(text)
	return !hasMeaningful || cleaned == ""
}

// FilterResponse filters a TranscribeResponse, updating the Text field.
// Returns false if the response contains only filler words and should be discarded.
func (f *STTFilter) FilterResponse(resp *TranscribeResponse) bool {
	if resp == nil {
		return false
	}

	cleaned, hasMeaningful := f.Clean(resp.Text)
	resp.Text = cleaned

	return hasMeaningful
}

// FragmentBuffer accumulates speech fragments until a pause is detected.
// It prevents sending incomplete thoughts to the brain by waiting for
// sufficient content before triggering a send.
type FragmentBuffer struct {
	mu            sync.Mutex
	buffer        strings.Builder
	lastAddTime   int64 // Unix nanoseconds
	timeoutNs     int64 // Timeout in nanoseconds
	minWordCount  int
	currentWords  int
	timeProvider  func() int64 // For testing - returns current time in nanoseconds
}

// FragmentBufferConfig holds configuration for FragmentBuffer.
type FragmentBufferConfig struct {
	TimeoutMs    int64 // Timeout in milliseconds (default 500)
	MinWordCount int   // Minimum word count to send (default 2)
}

// DefaultFragmentConfig returns sensible defaults for fragment accumulation.
func DefaultFragmentConfig() FragmentBufferConfig {
	return FragmentBufferConfig{
		TimeoutMs:    500,
		MinWordCount: 2,
	}
}

// NewFragmentBuffer creates a new FragmentBuffer with the given configuration.
// If config is nil, defaults are used.
func NewFragmentBuffer(config *FragmentBufferConfig) *FragmentBuffer {
	cfg := DefaultFragmentConfig()
	if config != nil {
		if config.TimeoutMs > 0 {
			cfg.TimeoutMs = config.TimeoutMs
		}
		if config.MinWordCount > 0 {
			cfg.MinWordCount = config.MinWordCount
		}
	}

	return &FragmentBuffer{
		timeoutNs:    cfg.TimeoutMs * 1e6, // Convert ms to ns
		minWordCount: cfg.MinWordCount,
		timeProvider: timeNowNano,
	}
}

// timeNowNano returns current time in nanoseconds.
// This is a package-level variable to allow mocking in tests.
var timeNowNano = func() int64 {
	return time.Now().UnixNano()
}

// Add appends a fragment to the buffer.
// Returns true if the fragment was added (non-empty after trimming).
func (fb *FragmentBuffer) Add(fragment string) bool {
	fragment = strings.TrimSpace(fragment)
	if fragment == "" {
		return false
	}

	fb.mu.Lock()
	defer fb.mu.Unlock()

	// Add space separator if buffer is not empty
	if fb.buffer.Len() > 0 {
		fb.buffer.WriteString(" ")
	}
	fb.buffer.WriteString(fragment)

	// Count words in the added fragment
	fb.currentWords += countWords(fragment)

	// Update timestamp
	fb.lastAddTime = fb.timeProvider()

	return true
}

// countWords counts the number of words in a string.
func countWords(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	return len(strings.Fields(s))
}

// ShouldSend returns true if the buffer contains enough content to send.
// This is true when:
// 1. Word count >= minWordCount, OR
// 2. Timeout has elapsed since the last fragment was added
func (fb *FragmentBuffer) ShouldSend() bool {
	fb.mu.Lock()
	defer fb.mu.Unlock()

	if fb.buffer.Len() == 0 {
		return false
	}

	// Check minimum word count
	if fb.currentWords >= fb.minWordCount {
		return true
	}

	// Check timeout (pause detection)
	if fb.lastAddTime > 0 {
		elapsed := fb.timeProvider() - fb.lastAddTime
		if elapsed >= fb.timeoutNs {
			return true
		}
	}

	return false
}

// Flush returns the accumulated text and clears the buffer.
// Returns empty string if buffer is empty.
func (fb *FragmentBuffer) Flush() string {
	fb.mu.Lock()
	defer fb.mu.Unlock()

	result := fb.buffer.String()
	fb.buffer.Reset()
	fb.currentWords = 0
	fb.lastAddTime = 0

	return result
}

// Peek returns the current buffer content without clearing it.
func (fb *FragmentBuffer) Peek() string {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	return fb.buffer.String()
}

// WordCount returns the current word count in the buffer.
func (fb *FragmentBuffer) WordCount() int {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	return fb.currentWords
}

// IsEmpty returns true if the buffer contains no content.
func (fb *FragmentBuffer) IsEmpty() bool {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	return fb.buffer.Len() == 0
}

// SetTimeout updates the timeout in milliseconds.
func (fb *FragmentBuffer) SetTimeout(timeoutMs int64) {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	fb.timeoutNs = timeoutMs * 1e6
}

// SetMinWordCount updates the minimum word count threshold.
func (fb *FragmentBuffer) SetMinWordCount(count int) {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	if count > 0 {
		fb.minWordCount = count
	}
}

// GetConfig returns the current configuration.
func (fb *FragmentBuffer) GetConfig() FragmentBufferConfig {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	return FragmentBufferConfig{
		TimeoutMs:    fb.timeoutNs / 1e6,
		MinWordCount: fb.minWordCount,
	}
}
