package voice

import (
	"regexp"
	"strings"
	"sync"
	"unicode"
)

// ═══════════════════════════════════════════════════════════════════════════════
// STREAM SANITIZER - Real-time token processing for low-latency voice output
// NFR-003: Sanitization SHALL NOT block streaming pipeline
// FR-014: Sentence splitter SHALL NOT split on dots within filenames or version numbers
// ═══════════════════════════════════════════════════════════════════════════════

// StreamSanitizer processes streaming LLM output tokens in real-time,
// buffering them and emitting complete sentences for TTS synthesis.
// It handles code block detection and generates verbal placeholders for code.
type StreamSanitizer struct {
	buffer          strings.Builder
	inCodeBlock     bool   // Currently inside a code block (```)
	codeBlockLang   string // Language of the current code block (if any)
	codeBlockCount  int    // Number of code blocks seen
	sentencePattern *regexp.Regexp
	mu              sync.Mutex
}

// NewStreamSanitizer creates a new StreamSanitizer instance.
func NewStreamSanitizer() *StreamSanitizer {
	return &StreamSanitizer{
		// Pattern for sentence-ending punctuation followed by space or end of string
		sentencePattern: regexp.MustCompile(`[.!?]+\s*$`),
	}
}

// knownExtensions contains file extensions that should not trigger sentence splits.
var knownExtensions = map[string]bool{
	"go": true, "py": true, "js": true, "ts": true, "jsx": true, "tsx": true,
	"json": true, "yaml": true, "yml": true, "toml": true, "xml": true,
	"html": true, "css": true, "scss": true, "less": true,
	"md": true, "txt": true, "log": true, "csv": true,
	"sh": true, "bash": true, "zsh": true, "fish": true,
	"sql": true, "rb": true, "rs": true, "java": true, "kt": true,
	"c": true, "cpp": true, "h": true, "hpp": true,
	"swift": true, "m": true, "mm": true,
	"r": true, "R": true, "rmd": true,
	"php": true, "pl": true, "pm": true,
	"ex": true, "exs": true, "erl": true,
	"hs": true, "elm": true, "ml": true, "mli": true,
	"vue": true, "svelte": true,
	"proto": true, "graphql": true, "gql": true,
	"conf": true, "cfg": true, "ini": true, "env": true,
	"dockerfile": true, "makefile": true,
	"lock": true, "sum": true, "mod": true,
}

// PushToken processes an incoming token and returns a complete sentence if ready.
// Returns the sentence text and true if a sentence is ready to be spoken,
// or empty string and false if more tokens are needed.
func (s *StreamSanitizer) PushToken(token string) (sentence string, ready bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for code block markers
	if strings.Contains(token, "```") {
		return s.handleCodeBlockMarker(token)
	}

	// If inside a code block, accumulate but don't emit
	if s.inCodeBlock {
		s.buffer.WriteString(token)
		return "", false
	}

	// Add token to buffer
	s.buffer.WriteString(token)

	// Check if we have a complete sentence
	return s.checkForSentence()
}

// handleCodeBlockMarker handles code block start/end markers.
func (s *StreamSanitizer) handleCodeBlockMarker(token string) (string, bool) {
	// Count occurrences of ``` in the token
	count := strings.Count(token, "```")

	// Handle each occurrence
	for i := 0; i < count; i++ {
		if s.inCodeBlock {
			// Ending a code block
			s.inCodeBlock = false
			s.codeBlockCount++

			// Emit any buffered content before the code block
			buffered := strings.TrimSpace(s.buffer.String())
			s.buffer.Reset()

			// Generate verbal placeholder for the code
			placeholder := s.generateCodePlaceholder()

			if buffered != "" {
				return buffered + " " + placeholder, true
			}
			return placeholder, true
		} else {
			// Starting a code block
			s.inCodeBlock = true

			// Try to extract language from the marker
			parts := strings.Split(token, "```")
			if len(parts) > 1 && len(parts[1]) > 0 {
				// Extract language hint (e.g., ```go or ```python)
				lang := strings.TrimSpace(strings.Split(parts[1], "\n")[0])
				if lang != "" {
					s.codeBlockLang = lang
				}
			}

			// Emit any buffered sentence before the code block
			buffered := strings.TrimSpace(s.buffer.String())
			s.buffer.Reset()
			if buffered != "" {
				return buffered, true
			}
		}
	}

	return "", false
}

// generateCodePlaceholder generates a verbal placeholder for code blocks.
func (s *StreamSanitizer) generateCodePlaceholder() string {
	lang := s.codeBlockLang
	s.codeBlockLang = "" // Reset for next block

	// Generate language-specific placeholder
	switch strings.ToLower(lang) {
	case "go", "golang":
		return "I've put some Go code on screen for you."
	case "python", "py":
		return "I've put some Python code on screen for you."
	case "javascript", "js":
		return "I've put some JavaScript code on screen for you."
	case "typescript", "ts":
		return "I've put some TypeScript code on screen for you."
	case "rust", "rs":
		return "I've put some Rust code on screen for you."
	case "java":
		return "I've put some Java code on screen for you."
	case "c", "cpp", "c++":
		return "I've put some C code on screen for you."
	case "bash", "sh", "shell", "zsh":
		return "I've put some shell commands on screen for you."
	case "sql":
		return "I've put some SQL on screen for you."
	case "json":
		return "I've put some JSON on screen for you."
	case "yaml", "yml":
		return "I've put some YAML on screen for you."
	case "html":
		return "I've put some HTML on screen for you."
	case "css":
		return "I've put some CSS on screen for you."
	default:
		return "I've put some code on screen for you."
	}
}

// checkForSentence checks if the buffer contains a complete sentence.
func (s *StreamSanitizer) checkForSentence() (string, bool) {
	content := s.buffer.String()
	if content == "" {
		return "", false
	}

	// Look for sentence-ending punctuation followed by whitespace
	for i := 0; i < len(content); i++ {
		ch := content[i]
		if ch == '.' || ch == '!' || ch == '?' {
			// Must be followed by whitespace or end of content to be a potential sentence end
			hasFollowingSpace := (i+1 < len(content) && unicode.IsSpace(rune(content[i+1]))) || i+1 >= len(content)

			// For periods, also check that this is a true sentence end (not filename/version)
			if ch == '.' {
				if hasFollowingSpace && s.isSentenceEnd(content, i) {
					// Extract the sentence (including punctuation)
					sentence := strings.TrimSpace(content[:i+1])
					remaining := strings.TrimLeft(content[i+1:], " \t")

					// Reset buffer with remaining content
					s.buffer.Reset()
					s.buffer.WriteString(remaining)

					if sentence != "" {
						return sentence, true
					}
				}
			} else if hasFollowingSpace {
				// ! and ? are always sentence ends when followed by space
				sentence := strings.TrimSpace(content[:i+1])
				remaining := strings.TrimLeft(content[i+1:], " \t")

				s.buffer.Reset()
				s.buffer.WriteString(remaining)

				if sentence != "" {
					return sentence, true
				}
			}
		}
	}

	return "", false
}

// isSentenceEnd determines if the punctuation at position i is a true sentence end.
// FR-014: Must not split on dots in filenames (config.json, main.go) or version numbers (v1.2.3).
func (s *StreamSanitizer) isSentenceEnd(text string, i int) bool {
	// Must have punctuation at position i
	if i >= len(text) {
		return false
	}

	ch := text[i]
	if ch != '.' && ch != '!' && ch != '?' {
		return false
	}

	// Exclamation and question marks are always sentence ends
	if ch == '!' || ch == '?' {
		return true
	}

	// For periods, check for filename or version patterns

	// Check if followed by a common file extension
	if i+1 < len(text) {
		remaining := text[i+1:]
		// Extract potential extension (up to next non-alphanumeric)
		extEnd := 0
		for extEnd < len(remaining) && (isAlphanumeric(remaining[extEnd]) || remaining[extEnd] == '_') {
			extEnd++
		}
		if extEnd > 0 {
			ext := strings.ToLower(remaining[:extEnd])
			if knownExtensions[ext] {
				return false
			}
		}
	}

	// Check if part of a version number pattern (e.g., v1.2.3, 10.15.7)
	if s.isVersionContext(text, i) {
		return false
	}

	// Check if followed by a digit (likely a decimal or IP address)
	if i+1 < len(text) && unicode.IsDigit(rune(text[i+1])) {
		return false
	}

	// Check if preceded by a single letter or number (likely abbreviation or decimal)
	if i > 0 && i < len(text)-1 {
		prev := text[i-1]
		next := text[i+1]
		// Single letter followed by period then space is likely end of sentence
		// But "v1.2" or "1.5" should not split
		if unicode.IsDigit(rune(prev)) && (unicode.IsDigit(rune(next)) || unicode.IsLetter(rune(next))) {
			return false
		}
	}

	// Check if this is an abbreviation (e.g., "e.g.", "i.e.", "etc.")
	if s.isAbbreviation(text, i) {
		return false
	}

	// Check for URL/path patterns
	if s.isURLOrPath(text, i) {
		return false
	}

	// Check if followed by whitespace or end of string
	if i+1 >= len(text) {
		return true // End of string
	}
	nextChar := text[i+1]
	return unicode.IsSpace(rune(nextChar))
}

// isVersionContext checks if the period at position i is part of a version number.
func (s *StreamSanitizer) isVersionContext(text string, i int) bool {
	// Look backward for version pattern start
	start := i
	for start > 0 && (unicode.IsDigit(rune(text[start-1])) || text[start-1] == '.') {
		start--
	}

	// Check for 'v' prefix (v1.2.3)
	if start > 0 && (text[start-1] == 'v' || text[start-1] == 'V') {
		start--
	}

	// Look forward for version pattern continuation
	end := i + 1
	for end < len(text) && (unicode.IsDigit(rune(text[end])) || text[end] == '.') {
		end++
	}

	// Extract potential version string
	if start < i && end > i+1 {
		version := text[start:end]
		// Count digits and periods
		digits := 0
		periods := 0
		for _, c := range version {
			if unicode.IsDigit(c) {
				digits++
			} else if c == '.' {
				periods++
			}
		}
		// Version numbers have multiple periods and digits
		if periods >= 1 && digits >= 2 {
			return true
		}
	}

	return false
}

// isAbbreviation checks if the period at position i is part of a common abbreviation.
func (s *StreamSanitizer) isAbbreviation(text string, i int) bool {
	// Common abbreviations
	abbreviations := []string{"e.g.", "i.e.", "etc.", "vs.", "mr.", "mrs.", "ms.", "dr.", "jr.", "sr."}

	for _, abbr := range abbreviations {
		abbLen := len(abbr)
		// Check if the period at position i is part of this abbreviation
		for j := 0; j < abbLen; j++ {
			if abbr[j] == '.' {
				checkStart := i - j
				if checkStart >= 0 && checkStart+abbLen <= len(text) {
					if strings.EqualFold(text[checkStart:checkStart+abbLen], abbr) {
						return true
					}
				}
			}
		}
	}
	return false
}

// isURLOrPath checks if the period is part of a URL or file path.
func (s *StreamSanitizer) isURLOrPath(text string, i int) bool {
	// Check for http:// or https:// before this period
	lower := strings.ToLower(text[:i])
	if strings.Contains(lower, "http://") || strings.Contains(lower, "https://") {
		// We're likely in a URL - find the URL boundaries
		// URLs typically end with whitespace
		if i+1 < len(text) && !unicode.IsSpace(rune(text[i+1])) {
			return true
		}
	}

	// Check for path-like patterns (contains /)
	start := i
	for start > 0 && text[start-1] != ' ' && text[start-1] != '\n' {
		start--
	}
	segment := text[start:i]
	if strings.Contains(segment, "/") || strings.Contains(segment, "\\") {
		return true
	}

	return false
}

// isAlphanumeric checks if a byte is alphanumeric.
func isAlphanumeric(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// Flush returns any remaining buffered content and clears the buffer.
// This should be called when the stream ends to get any incomplete sentence.
func (s *StreamSanitizer) Flush() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If we're still in a code block, emit a placeholder
	if s.inCodeBlock {
		s.inCodeBlock = false
		s.codeBlockCount++
		placeholder := s.generateCodePlaceholder()
		s.buffer.Reset()
		return placeholder
	}

	content := strings.TrimSpace(s.buffer.String())
	s.buffer.Reset()
	return content
}

// Reset clears all state for reuse.
func (s *StreamSanitizer) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.buffer.Reset()
	s.inCodeBlock = false
	s.codeBlockLang = ""
	s.codeBlockCount = 0
}

// InCodeBlock returns whether the sanitizer is currently inside a code block.
func (s *StreamSanitizer) InCodeBlock() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inCodeBlock
}

// CodeBlockCount returns the number of code blocks processed.
func (s *StreamSanitizer) CodeBlockCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.codeBlockCount
}

// BufferLength returns the current buffer length.
func (s *StreamSanitizer) BufferLength() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buffer.Len()
}

// PeekBuffer returns a copy of the current buffer content without consuming it.
func (s *StreamSanitizer) PeekBuffer() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buffer.String()
}
