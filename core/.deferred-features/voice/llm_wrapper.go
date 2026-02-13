// Package voice provides voice processing capabilities for Cortex.
package voice

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"unicode"
)

// ═══════════════════════════════════════════════════════════════════════════════
// VOICE-AWARE LLM WRAPPER
// FR-002: Voice-optimized prompt injection
// FR-003: Process LLM output as stream, not blocking
// ═══════════════════════════════════════════════════════════════════════════════

// StreamingLLMClient defines the interface for an LLM that supports streaming.
type StreamingLLMClient interface {
	// ChatStream sends a message and returns a channel of tokens.
	// The channel is closed when the response is complete.
	ChatStream(ctx context.Context, messages []LLMMessage, systemPrompt string) (<-chan StreamToken, error)

	// Chat sends a message and returns the complete response.
	Chat(ctx context.Context, messages []LLMMessage, systemPrompt string) (string, error)
}

// LLMMessage represents a conversation message for the LLM.
type LLMMessage struct {
	Role    string `json:"role"`    // "user", "assistant", "system"
	Content string `json:"content"` // Message content
}

// StreamToken represents a single token from a streaming response.
type StreamToken struct {
	Token string // The token text
	Done  bool   // True if this is the last token
	Error error  // Non-nil if an error occurred
}

// VoiceAwareLLM wraps an LLM client to provide voice-aware streaming.
// It handles voice-optimized prompt injection and splits streaming output
// into channels for visual display and TTS synthesis.
type VoiceAwareLLM struct {
	client       StreamingLLMClient
	modeDetector *ModeDetector
	injector     *PromptInjector

	mu         sync.RWMutex
	voiceMode  bool // Explicit voice mode override
	sttActive  bool // STT is currently active
	ttsEnabled bool // TTS is enabled
}

// NewVoiceAwareLLM creates a new VoiceAwareLLM with the given LLM client and base system prompt.
func NewVoiceAwareLLM(client StreamingLLMClient, baseSystemPrompt string) *VoiceAwareLLM {
	modeDetector := NewModeDetector()
	injector := NewPromptInjector(modeDetector, baseSystemPrompt)

	return &VoiceAwareLLM{
		client:       client,
		modeDetector: modeDetector,
		injector:     injector,
		voiceMode:    false,
		sttActive:    false,
		ttsEnabled:   false,
	}
}

// SetVoiceMode explicitly enables or disables voice mode.
// When enabled, prompts will be augmented with voice-specific instructions.
func (v *VoiceAwareLLM) SetVoiceMode(enabled bool) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.voiceMode = enabled
	if enabled {
		v.modeDetector.SetExplicitMode(ModeVoice)
	} else {
		v.modeDetector.ClearExplicitMode()
	}
}

// OnSTTActive should be called when speech-to-text becomes active or inactive.
// This affects automatic voice mode detection.
func (v *VoiceAwareLLM) OnSTTActive(active bool) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.sttActive = active
	v.modeDetector.SetSTTActive(active)
}

// OnTTSEnabled should be called when TTS availability changes.
// This affects automatic voice mode detection.
func (v *VoiceAwareLLM) OnTTSEnabled(enabled bool) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.ttsEnabled = enabled
	v.modeDetector.SetTTSEnabled(enabled)
}

// IsVoiceMode returns whether voice mode is currently active.
func (v *VoiceAwareLLM) IsVoiceMode() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.modeDetector.IsVoiceMode()
}

// ModeDetector returns the underlying mode detector.
func (v *VoiceAwareLLM) ModeDetector() *ModeDetector {
	return v.modeDetector
}

// Injector returns the underlying prompt injector.
func (v *VoiceAwareLLM) Injector() *PromptInjector {
	return v.injector
}

// StreamResponse sends a request to the LLM and returns dual channels:
// - visualCh: All tokens for visual display in the terminal
// - voiceCh: Sanitized, complete sentences for TTS synthesis
// - errCh: Any errors that occur during streaming
//
// The visual channel receives tokens immediately as they arrive.
// The voice channel receives complete, sanitized sentences suitable for TTS.
// Both channels are closed when the response is complete.
func (v *VoiceAwareLLM) StreamResponse(ctx context.Context, messages []LLMMessage) (
	visualCh <-chan string,
	voiceCh <-chan string,
	errCh <-chan error,
) {
	// Create output channels
	visual := make(chan string, 100)
	voice := make(chan string, 20)
	errs := make(chan error, 1)

	go v.streamWorker(ctx, messages, visual, voice, errs)

	return visual, voice, errs
}

// streamWorker handles the streaming response and splits output to channels.
func (v *VoiceAwareLLM) streamWorker(
	ctx context.Context,
	messages []LLMMessage,
	visualCh chan<- string,
	voiceCh chan<- string,
	errCh chan<- error,
) {
	defer close(visualCh)
	defer close(voiceCh)
	defer close(errCh)

	// Build the system prompt (with voice injection if in voice mode)
	systemPrompt := v.injector.BuildSystemPrompt()

	// Prepend few-shot examples if in voice mode
	allMessages := messages
	if fewShot := v.injector.BuildFewShotMessages(); len(fewShot) > 0 {
		converted := make([]LLMMessage, len(fewShot))
		for i, m := range fewShot {
			converted[i] = LLMMessage{Role: m.Role, Content: m.Content}
		}
		allMessages = append(converted, messages...)
	}

	// Start streaming from the LLM
	tokenCh, err := v.client.ChatStream(ctx, allMessages, systemPrompt)
	if err != nil {
		errCh <- err
		return
	}

	// Process tokens
	isVoice := v.IsVoiceMode()
	var sentenceBuffer strings.Builder

	for token := range tokenCh {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			errCh <- ctx.Err()
			return
		default:
		}

		// Handle errors
		if token.Error != nil {
			errCh <- token.Error
			return
		}

		// Send to visual channel immediately
		visualCh <- token.Token

		// If voice mode, buffer for sentence extraction
		if isVoice {
			sentenceBuffer.WriteString(token.Token)

			// Extract complete sentences from buffer
			sentences := extractSentences(&sentenceBuffer)
			for _, sentence := range sentences {
				sanitized := sanitizeForTTS(sentence)
				if sanitized != "" {
					voiceCh <- sanitized
				}
			}
		}
	}

	// Flush any remaining content in voice mode
	if isVoice && sentenceBuffer.Len() > 0 {
		remaining := strings.TrimSpace(sentenceBuffer.String())
		if remaining != "" {
			sanitized := sanitizeForTTS(remaining)
			if sanitized != "" {
				voiceCh <- sanitized
			}
		}
	}
}

// Chat sends a non-streaming request to the LLM.
// This is a convenience method that returns the complete response.
func (v *VoiceAwareLLM) Chat(ctx context.Context, messages []LLMMessage) (string, error) {
	// Build the system prompt (with voice injection if in voice mode)
	systemPrompt := v.injector.BuildSystemPrompt()

	// Prepend few-shot examples if in voice mode
	allMessages := messages
	if fewShot := v.injector.BuildFewShotMessages(); len(fewShot) > 0 {
		converted := make([]LLMMessage, len(fewShot))
		for i, m := range fewShot {
			converted[i] = LLMMessage{Role: m.Role, Content: m.Content}
		}
		allMessages = append(converted, messages...)
	}

	return v.client.Chat(ctx, allMessages, systemPrompt)
}

// ═══════════════════════════════════════════════════════════════════════════════
// TEXT PROCESSING UTILITIES
// ═══════════════════════════════════════════════════════════════════════════════

// sentenceEndPattern matches sentence-ending punctuation followed by whitespace or end.
var sentenceEndPattern = regexp.MustCompile(`[.!?]+\s*`)

// extractSentences extracts complete sentences from the buffer,
// leaving incomplete content in the buffer.
func extractSentences(buffer *strings.Builder) []string {
	text := buffer.String()
	sentences := []string{}

	// Find sentence boundaries
	matches := sentenceEndPattern.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return sentences
	}

	// Extract complete sentences
	lastEnd := 0
	for _, match := range matches {
		end := match[1]
		sentence := strings.TrimSpace(text[lastEnd : match[0]+1]) // Include the punctuation
		if sentence != "" {
			sentences = append(sentences, sentence)
		}
		lastEnd = end
	}

	// Keep remaining incomplete content in buffer
	buffer.Reset()
	if lastEnd < len(text) {
		buffer.WriteString(text[lastEnd:])
	}

	return sentences
}

// sanitizeForTTS cleans text for text-to-speech synthesis.
// It removes markdown, code blocks, and converts symbols to speakable text.
func sanitizeForTTS(text string) string {
	// Remove code blocks
	codeBlockPattern := regexp.MustCompile("```[\\s\\S]*?```")
	text = codeBlockPattern.ReplaceAllString(text, "")

	// Remove inline code
	inlineCodePattern := regexp.MustCompile("`[^`]+`")
	text = inlineCodePattern.ReplaceAllString(text, "")

	// Remove markdown formatting
	text = strings.ReplaceAll(text, "**", "")
	text = strings.ReplaceAll(text, "__", "")
	text = strings.ReplaceAll(text, "*", "")
	text = strings.ReplaceAll(text, "_", " ")

	// Remove markdown headers
	headerPattern := regexp.MustCompile(`(?m)^#{1,6}\s*`)
	text = headerPattern.ReplaceAllString(text, "")

	// Remove markdown links, keep text
	linkPattern := regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	text = linkPattern.ReplaceAllString(text, "$1")

	// Remove bullet points and list markers
	bulletPattern := regexp.MustCompile(`(?m)^[\s]*[-*+]\s+`)
	text = bulletPattern.ReplaceAllString(text, "")

	// Remove numbered list markers
	numberedPattern := regexp.MustCompile(`(?m)^[\s]*\d+\.\s+`)
	text = numberedPattern.ReplaceAllString(text, "")

	// Convert common symbols to words
	text = convertSymbolsToWords(text)

	// Normalize whitespace
	whitespacePattern := regexp.MustCompile(`\s+`)
	text = whitespacePattern.ReplaceAllString(text, " ")

	// Trim and return
	return strings.TrimSpace(text)
}

// convertSymbolsToWords converts common symbols to speakable words.
func convertSymbolsToWords(text string) string {
	// Order matters! More specific patterns must come before less specific ones.
	// Using a slice to guarantee order (maps have random iteration order in Go).
	replacements := []struct {
		symbol string
		word   string
	}{
		// Temperature units first (before plain degree symbol)
		{"°F", " degrees Fahrenheit "},
		{"°C", " degrees Celsius "},
		{"°", " degrees "},
		// Multi-character operators before single character
		{">=", " greater than or equal to "},
		{"<=", " less than or equal to "},
		{"!=", " not equal to "},
		{"==", " equals "},
		{"=>", " arrow "},
		{"->", " arrow "},
		{"↔", " between "},
		{"→", " to "},
		{"←", " from "},
		// Single characters
		{"&", " and "},
		{"@", " at "},
		{"#", " number "},
		{"%", " percent "},
		{"~", " approximately "},
		{"/", " slash "},
	}

	for _, r := range replacements {
		text = strings.ReplaceAll(text, r.symbol, r.word)
	}

	// Handle isolated special characters that might remain
	// Remove them if they're surrounded by spaces
	isolatedPattern := regexp.MustCompile(`\s[<>|\\^]+\s`)
	text = isolatedPattern.ReplaceAllString(text, " ")

	return text
}

// SanitizeForTTS is exported for use by other packages.
func SanitizeForTTS(text string) string {
	return sanitizeForTTS(text)
}

// ═══════════════════════════════════════════════════════════════════════════════
// SENTENCE SPLITTER FOR TTS
// ═══════════════════════════════════════════════════════════════════════════════

// SplitIntoSentences splits text into sentences for TTS processing.
// This is useful for processing complete responses.
func SplitIntoSentences(text string) []string {
	// Pre-process to handle abbreviations
	abbrevs := []string{"Mr.", "Mrs.", "Ms.", "Dr.", "Prof.", "Sr.", "Jr.", "vs.", "etc.", "i.e.", "e.g."}
	placeholders := make(map[string]string)

	for i, abbrev := range abbrevs {
		placeholder := string(rune(0x0001 + i))
		placeholders[placeholder] = abbrev
		text = strings.ReplaceAll(text, abbrev, placeholder)
	}

	// Split on sentence boundaries
	var sentences []string
	var current strings.Builder

	for _, r := range text {
		current.WriteRune(r)

		// Check for sentence end
		if r == '.' || r == '!' || r == '?' {
			sentence := strings.TrimSpace(current.String())
			if sentence != "" {
				// Restore abbreviations
				for placeholder, abbrev := range placeholders {
					sentence = strings.ReplaceAll(sentence, placeholder, abbrev)
				}
				sentences = append(sentences, sentence)
			}
			current.Reset()
		}
	}

	// Add any remaining content
	if current.Len() > 0 {
		sentence := strings.TrimSpace(current.String())
		if sentence != "" {
			for placeholder, abbrev := range placeholders {
				sentence = strings.ReplaceAll(sentence, placeholder, abbrev)
			}
			sentences = append(sentences, sentence)
		}
	}

	return sentences
}

// IsReadableText returns true if the text contains mostly readable content
// (letters, numbers, spaces) and not code or special characters.
func IsReadableText(text string) bool {
	if text == "" {
		return false
	}

	readable := 0
	total := 0

	for _, r := range text {
		total++
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) ||
			r == '.' || r == ',' || r == '!' || r == '?' || r == '\'' || r == '"' {
			readable++
		}
	}

	// Text is readable if at least 80% of characters are readable
	return float64(readable)/float64(total) >= 0.8
}
