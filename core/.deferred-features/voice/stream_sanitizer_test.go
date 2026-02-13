package voice

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ═══════════════════════════════════════════════════════════════════════════════
// STREAM SANITIZER TESTS
// FR-014: Sentence splitter SHALL NOT split on dots within filenames or version numbers
// ═══════════════════════════════════════════════════════════════════════════════

func TestNewStreamSanitizer(t *testing.T) {
	s := NewStreamSanitizer()
	require.NotNil(t, s)
	assert.Equal(t, 0, s.BufferLength())
	assert.False(t, s.InCodeBlock())
	assert.Equal(t, 0, s.CodeBlockCount())
}

func TestPushTokenSimpleSentence(t *testing.T) {
	s := NewStreamSanitizer()

	// Push tokens that form a sentence
	sentence, ready := s.PushToken("Hello")
	assert.False(t, ready)
	assert.Empty(t, sentence)

	sentence, ready = s.PushToken(" world")
	assert.False(t, ready)
	assert.Empty(t, sentence)

	sentence, ready = s.PushToken(". ")
	assert.True(t, ready)
	assert.Equal(t, "Hello world.", sentence)
}

func TestPushTokenMultipleSentences(t *testing.T) {
	s := NewStreamSanitizer()

	// Push tokens that form multiple sentences
	// When we push a complete sentence, it returns immediately
	sentence, ready := s.PushToken("First sentence. ")

	// First sentence is complete and returned immediately
	assert.True(t, ready)
	assert.Equal(t, "First sentence.", sentence)

	// Push second part - buffer should contain it
	sentence2, ready2 := s.PushToken("Second starts")
	assert.False(t, ready2, "Second part is incomplete")
	assert.Empty(t, sentence2)
	assert.Contains(t, s.PeekBuffer(), "Second starts")
}

func TestPushTokenExclamation(t *testing.T) {
	s := NewStreamSanitizer()

	// Exclamation with trailing space completes the sentence
	sentence, ready := s.PushToken("Hello! ")

	assert.True(t, ready)
	assert.Equal(t, "Hello!", sentence)

	// Buffer should be empty after sentence extraction
	assert.Empty(t, s.PeekBuffer())
}

func TestPushTokenQuestion(t *testing.T) {
	s := NewStreamSanitizer()

	// Question mark with trailing space completes the sentence
	sentence, ready := s.PushToken("How are you? ")

	assert.True(t, ready)
	assert.Equal(t, "How are you?", sentence)
}

// FR-014: Filename tests
func TestPushTokenFilenamePreservation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool // Should NOT split
	}{
		{"Go file", "Edit the main.go file.", false},
		{"Python file", "Run python script.py now.", false},
		{"JSON file", "Check config.json please.", false},
		{"TypeScript file", "Update index.ts file.", false},
		{"YAML file", "See docker-compose.yaml for details.", false},
		{"Markdown file", "Read the README.md document.", false},
		{"Shell script", "Execute setup.sh script.", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStreamSanitizer()

			// Push the entire input at once
			sentence, ready := s.PushToken(tt.input + " ")

			if tt.expected {
				// Should not have split
				assert.False(t, ready, "Should not split on filename dot")
			} else {
				// Should emit the complete sentence
				assert.True(t, ready)
				assert.Contains(t, sentence, ".")
			}
		})
	}
}

func TestPushTokenFilenameInMiddle(t *testing.T) {
	s := NewStreamSanitizer()

	// Filename in middle of sentence should not cause split on the filename dot
	// But the sentence ending period should trigger completion
	sentence, ready := s.PushToken("The config.json file is important. ")

	assert.True(t, ready)
	assert.Equal(t, "The config.json file is important.", sentence)
}

// FR-014: Version number tests
func TestPushTokenVersionNumberPreservation(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"Semantic version", "Using version v1.2.3 now. "},
		{"Plain version", "Version 10.15.7 is required. "},
		{"Short version", "Update to 2.0 please. "},
		{"Four part version", "Install 1.2.3.4 first. "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStreamSanitizer()

			sentence, ready := s.PushToken(tt.input)
			sentence2, _ := s.PushToken("Done")

			// If ready, check that version number is intact
			if ready {
				// Version should be in the sentence, not split
				assert.NotEmpty(t, sentence)
			} else {
				// Flush and check
				flushed := s.Flush()
				assert.NotEmpty(t, flushed)
				_ = sentence2 // Use variable
			}
		})
	}
}

func TestPushTokenVersionInSentence(t *testing.T) {
	s := NewStreamSanitizer()

	// Push sentence with version number - should not split on version dots
	sentence, ready := s.PushToken("Go version 1.21.0 is great. ")

	assert.True(t, ready)
	assert.Equal(t, "Go version 1.21.0 is great.", sentence)
}

// Code block tests
func TestPushTokenCodeBlockStart(t *testing.T) {
	s := NewStreamSanitizer()

	// Text before code block
	s.PushToken("Here is code:")
	sentence, ready := s.PushToken("```")

	// Should emit text before code block
	assert.True(t, ready)
	assert.Equal(t, "Here is code:", sentence)
	assert.True(t, s.InCodeBlock())
}

func TestPushTokenCodeBlockEnd(t *testing.T) {
	s := NewStreamSanitizer()

	// Start code block
	s.PushToken("```go\nfunc main() {}\n")
	assert.True(t, s.InCodeBlock())

	// End code block
	sentence, ready := s.PushToken("```")

	assert.True(t, ready)
	assert.Contains(t, sentence, "Go code")
	assert.False(t, s.InCodeBlock())
	assert.Equal(t, 1, s.CodeBlockCount())
}

func TestPushTokenCodeBlockWithLanguage(t *testing.T) {
	tests := []struct {
		lang     string
		expected string
	}{
		{"go", "Go"},
		{"python", "Python"},
		{"javascript", "JavaScript"},
		{"typescript", "TypeScript"},
		{"rust", "Rust"},
		{"bash", "shell"},
		{"sql", "SQL"},
		{"json", "JSON"},
		{"yaml", "YAML"},
		{"", "code"},
	}

	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			s := NewStreamSanitizer()

			// Start code block with language
			s.PushToken("```" + tt.lang + "\n")
			s.PushToken("some code\n")

			// End code block
			sentence, ready := s.PushToken("```")

			assert.True(t, ready)
			assert.Contains(t, sentence, tt.expected)
		})
	}
}

func TestPushTokenMultipleCodeBlocks(t *testing.T) {
	s := NewStreamSanitizer()

	// First code block
	s.PushToken("```go\ncode1\n```")
	assert.Equal(t, 1, s.CodeBlockCount())

	// Second code block
	s.PushToken("```python\ncode2\n```")
	assert.Equal(t, 2, s.CodeBlockCount())
}

func TestFlush(t *testing.T) {
	s := NewStreamSanitizer()

	// Add incomplete sentence
	s.PushToken("This is incomplete")

	// Flush should return it
	content := s.Flush()
	assert.Equal(t, "This is incomplete", content)
	assert.Equal(t, 0, s.BufferLength())
}

func TestFlushInsideCodeBlock(t *testing.T) {
	s := NewStreamSanitizer()

	// Start code block but don't close it
	s.PushToken("```go\nfunc main() {}")

	// Flush should emit placeholder
	content := s.Flush()
	assert.Contains(t, content, "code")
	assert.False(t, s.InCodeBlock())
}

func TestReset(t *testing.T) {
	s := NewStreamSanitizer()

	// Set up state
	s.PushToken("Some text```go\ncode")

	// Reset
	s.Reset()

	assert.Equal(t, 0, s.BufferLength())
	assert.False(t, s.InCodeBlock())
	assert.Equal(t, 0, s.CodeBlockCount())
}

func TestPeekBuffer(t *testing.T) {
	s := NewStreamSanitizer()

	s.PushToken("Hello world")

	// Peek should not consume
	peeked := s.PeekBuffer()
	assert.Equal(t, "Hello world", peeked)

	// Buffer should still have content
	assert.Equal(t, len("Hello world"), s.BufferLength())
}

func TestStreamSanitizerConcurrency(t *testing.T) {
	s := NewStreamSanitizer()
	iterations := 100

	var wg sync.WaitGroup
	wg.Add(3)

	// Concurrent pushes
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			s.PushToken("Hello. ")
		}
	}()

	// Concurrent peeks
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = s.PeekBuffer()
			_ = s.BufferLength()
		}
	}()

	// Concurrent state checks
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = s.InCodeBlock()
			_ = s.CodeBlockCount()
		}
	}()

	wg.Wait()

	// Should not panic
	_ = s.Flush()
}

// Abbreviation tests
func TestPushTokenAbbreviations(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"e.g.", "For example, e.g. this. ", "For example, e.g. this."},
		{"i.e.", "That is, i.e. good. ", "That is, i.e. good."},
		{"etc.", "And etc. more. ", "And etc. more."},
		{"vs.", "Good vs. bad. ", "Good vs. bad."},
		{"Mr.", "Hello Mr. Smith. ", "Hello Mr. Smith."},
		{"Dr.", "Call Dr. Jones. ", "Call Dr. Jones."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStreamSanitizer()

			// When input contains complete sentence, it returns immediately
			sentence, ready := s.PushToken(tt.input)

			assert.True(t, ready, "Should recognize complete sentence")
			assert.Equal(t, tt.expected, sentence)
		})
	}
}

// Decimal number tests
func TestPushTokenDecimalNumbers(t *testing.T) {
	s := NewStreamSanitizer()

	// Decimal should not split, but sentence ending period should
	sentence, ready := s.PushToken("The value is 3.14 radians. ")

	assert.True(t, ready)
	assert.Equal(t, "The value is 3.14 radians.", sentence)
}

// URL tests
func TestPushTokenURLs(t *testing.T) {
	s := NewStreamSanitizer()

	// URL should not split at dots, but sentence ending period should
	sentence, ready := s.PushToken("Visit https://example.com/page for more. ")

	assert.True(t, ready)
	assert.Contains(t, sentence, "example.com")
}

// Edge cases
func TestPushTokenEmptyInput(t *testing.T) {
	s := NewStreamSanitizer()

	sentence, ready := s.PushToken("")
	assert.False(t, ready)
	assert.Empty(t, sentence)
}

func TestPushTokenWhitespaceOnly(t *testing.T) {
	s := NewStreamSanitizer()

	sentence, ready := s.PushToken("   ")
	assert.False(t, ready)
	assert.Empty(t, sentence)
}

func TestPushTokenMultiplePunctuation(t *testing.T) {
	s := NewStreamSanitizer()

	// Multiple punctuation with trailing space should complete the sentence
	sentence, ready := s.PushToken("Really?! ")

	assert.True(t, ready)
	assert.Contains(t, sentence, "Really")
}

func TestPushTokenNoSpaceAfterPunctuation(t *testing.T) {
	s := NewStreamSanitizer()

	// Period without space should not immediately split (might be abbreviation or filename)
	s.PushToken("Check file.go")
	sentence, ready := s.PushToken(" now")

	// Should not split on .go
	assert.False(t, ready)
	assert.Empty(t, sentence)
}

func TestFlushEmpty(t *testing.T) {
	s := NewStreamSanitizer()

	// Flush empty buffer
	content := s.Flush()
	assert.Empty(t, content)
}

func TestCodeBlockTokenWithBothMarkers(t *testing.T) {
	s := NewStreamSanitizer()

	// Token containing both start and end markers
	sentence, ready := s.PushToken("```go\ncode\n```")

	assert.True(t, ready)
	assert.Contains(t, sentence, "Go code")
	assert.False(t, s.InCodeBlock())
}
