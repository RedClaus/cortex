package voice

import (
	"regexp"
	"strings"
	"sync"
)

// ═══════════════════════════════════════════════════════════════════════════════
// RESPONSE SANITIZER - Text transformation for natural voice output
// Converts text to speech-friendly format with proper symbol and number handling
// ═══════════════════════════════════════════════════════════════════════════════

// ResponseSanitizer transforms text content for natural voice output.
// It removes markdown formatting, converts symbols to words, and cleans up
// content that doesn't translate well to speech.
type ResponseSanitizer struct {
	// Compiled regex patterns for efficiency
	patterns *sanitizerPatterns
	once     sync.Once
}

// sanitizerPatterns holds precompiled regex patterns.
type sanitizerPatterns struct {
	// Markdown patterns
	codeBlock        *regexp.Regexp
	inlineCode       *regexp.Regexp
	header           *regexp.Regexp
	link             *regexp.Regexp
	image            *regexp.Regexp
	boldAsterisk     *regexp.Regexp
	boldUnderscore   *regexp.Regexp
	italicAsterisk   *regexp.Regexp
	italicUnderscore *regexp.Regexp
	strikethrough    *regexp.Regexp
	bulletList       *regexp.Regexp
	numberedList     *regexp.Regexp
	blockquote       *regexp.Regexp
	horizontalRule   *regexp.Regexp
	table            *regexp.Regexp
	tableDivider     *regexp.Regexp

	// Symbol patterns
	currency    *regexp.Regexp
	percentage  *regexp.Regexp
	temperature *regexp.Regexp
	degree      *regexp.Regexp
	arrow       *regexp.Regexp
	doubleArrow *regexp.Regexp
	ellipsis    *regexp.Regexp
	ampersand   *regexp.Regexp

	// Emoji and special character patterns
	emoji *regexp.Regexp

	// Cleanup patterns
	multipleSpaces   *regexp.Regexp
	multipleNewlines *regexp.Regexp
	leadingSpaces    *regexp.Regexp
}

// NewResponseSanitizer creates a new ResponseSanitizer instance.
func NewResponseSanitizer() *ResponseSanitizer {
	return &ResponseSanitizer{}
}

// initPatterns compiles all regex patterns once.
func (s *ResponseSanitizer) initPatterns() {
	s.once.Do(func() {
		s.patterns = &sanitizerPatterns{
			// Markdown patterns
			codeBlock:        regexp.MustCompile("(?s)```[^`]*```"),
			inlineCode:       regexp.MustCompile("`[^`]+`"),
			header:           regexp.MustCompile(`(?m)^#{1,6}\s+`),
			link:             regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`),
			image:            regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`),
			boldAsterisk:     regexp.MustCompile(`\*\*([^*]+)\*\*`),
			boldUnderscore:   regexp.MustCompile(`__([^_]+)__`),
			italicAsterisk:   regexp.MustCompile(`\*([^*]+)\*`),
			italicUnderscore: regexp.MustCompile(`_([^_]+)_`),
			strikethrough:    regexp.MustCompile(`~~([^~]+)~~`),
			bulletList:       regexp.MustCompile(`(?m)^\s*[-*+]\s+`),
			numberedList:     regexp.MustCompile(`(?m)^\s*\d+\.\s+`),
			blockquote:       regexp.MustCompile(`(?m)^>\s*`),
			horizontalRule:   regexp.MustCompile(`(?m)^[-*_]{3,}$`),
			table:            regexp.MustCompile(`\|[^|]+\|`),
			tableDivider:     regexp.MustCompile(`(?m)^\|?[-:| ]+\|?$`),

			// Symbol patterns - capture groups for replacement
			currency:    regexp.MustCompile(`\$(\d+(?:\.\d{1,2})?)`),
			percentage:  regexp.MustCompile(`(\d+(?:\.\d+)?)\s*%`),
			temperature: regexp.MustCompile(`(\d+)\s*°\s*([FfCc])\b`), // Word boundary to avoid matching "clockwise"
			degree:      regexp.MustCompile(`(\d+)\s*°`),
			arrow:       regexp.MustCompile(`\s*->\s*`),
			doubleArrow: regexp.MustCompile(`\s*=>\s*`),
			ellipsis:    regexp.MustCompile(`\.{3,}`),
			ampersand:   regexp.MustCompile(`\s*&\s*`),

			// Emoji patterns (common ranges including variation selectors)
			emoji: regexp.MustCompile(`[\x{1F300}-\x{1F9FF}]|[\x{2600}-\x{26FF}]|[\x{2700}-\x{27BF}]|[\x{1F600}-\x{1F64F}]|[\x{1F680}-\x{1F6FF}]|[\x{1F1E0}-\x{1F1FF}]|[\x{FE00}-\x{FE0F}]`),

			// Cleanup patterns
			multipleSpaces:   regexp.MustCompile(`[ \t]+`),
			multipleNewlines: regexp.MustCompile(`\n{3,}`),
			leadingSpaces:    regexp.MustCompile(`(?m)^[ \t]+`),
		}
	})
}

// Sanitize transforms the input text into speech-friendly format.
// It removes markdown, converts symbols to words, and cleans up formatting.
func (s *ResponseSanitizer) Sanitize(text string) string {
	s.initPatterns()

	// Process in order of priority
	text = s.removeMarkdown(text)
	text = s.convertSymbols(text)
	text = s.removeEmojis(text)
	text = s.cleanupWhitespace(text)

	return strings.TrimSpace(text)
}

// removeMarkdown removes all markdown formatting from the text.
func (s *ResponseSanitizer) removeMarkdown(text string) string {
	p := s.patterns

	// Remove code blocks first (replace with placeholder)
	text = p.codeBlock.ReplaceAllString(text, " [code block] ")

	// Remove inline code
	text = p.inlineCode.ReplaceAllString(text, "")

	// Remove images (before links, as images use similar syntax)
	text = p.image.ReplaceAllString(text, "$1")

	// Convert links to just the text
	text = p.link.ReplaceAllString(text, "$1")

	// Remove headers (but keep the text)
	text = p.header.ReplaceAllString(text, "")

	// Remove bold formatting (keep text)
	text = p.boldAsterisk.ReplaceAllString(text, "$1")
	text = p.boldUnderscore.ReplaceAllString(text, "$1")

	// Remove italic formatting (keep text)
	text = p.italicAsterisk.ReplaceAllString(text, "$1")
	text = p.italicUnderscore.ReplaceAllString(text, "$1")

	// Remove strikethrough (keep text)
	text = p.strikethrough.ReplaceAllString(text, "$1")

	// Remove list markers
	text = p.bulletList.ReplaceAllString(text, "")
	text = p.numberedList.ReplaceAllString(text, "")

	// Remove blockquotes
	text = p.blockquote.ReplaceAllString(text, "")

	// Remove horizontal rules
	text = p.horizontalRule.ReplaceAllString(text, "")

	// Remove table dividers
	text = p.tableDivider.ReplaceAllString(text, "")

	// Simplify table cells (remove pipe characters)
	text = strings.ReplaceAll(text, "|", " ")

	return text
}

// convertSymbols converts common symbols to their spoken equivalents.
func (s *ResponseSanitizer) convertSymbols(text string) string {
	p := s.patterns

	// Currency: $42 → "42 dollars", $42.50 → "42 dollars and 50 cents"
	text = p.currency.ReplaceAllStringFunc(text, func(match string) string {
		// Extract the number
		submatch := p.currency.FindStringSubmatch(match)
		if len(submatch) < 2 {
			return match
		}
		amount := submatch[1]
		if strings.Contains(amount, ".") {
			parts := strings.Split(amount, ".")
			dollars := parts[0]
			cents := parts[1]
			if cents == "00" {
				return dollars + " dollars"
			}
			return dollars + " dollars and " + cents + " cents"
		}
		return amount + " dollars"
	})

	// Percentage: 42% → "42 percent"
	text = p.percentage.ReplaceAllString(text, "$1 percent")

	// Temperature: 42°F → "42 degrees fahrenheit", 42°C → "42 degrees celsius"
	text = p.temperature.ReplaceAllStringFunc(text, func(match string) string {
		submatch := p.temperature.FindStringSubmatch(match)
		if len(submatch) < 3 {
			return match
		}
		temp := submatch[1]
		unit := strings.ToLower(submatch[2])
		if unit == "f" {
			return temp + " degrees fahrenheit"
		}
		return temp + " degrees celsius"
	})

	// Plain degree symbol: 42° → "42 degrees"
	text = p.degree.ReplaceAllString(text, "$1 degrees")

	// Arrows: -> → "to", => → "becomes"
	text = p.doubleArrow.ReplaceAllString(text, " becomes ")
	text = p.arrow.ReplaceAllString(text, " to ")

	// Ellipsis: ... → " "
	text = p.ellipsis.ReplaceAllString(text, " ")

	// Ampersand: & → "and"
	text = p.ampersand.ReplaceAllString(text, " and ")

	// Additional symbol replacements
	text = strings.ReplaceAll(text, "!=", " is not equal to ")
	text = strings.ReplaceAll(text, "==", " equals ")
	text = strings.ReplaceAll(text, ">=", " is greater than or equal to ")
	text = strings.ReplaceAll(text, "<=", " is less than or equal to ")
	text = strings.ReplaceAll(text, "<>", " is not equal to ")

	// Math symbols
	text = strings.ReplaceAll(text, " + ", " plus ")
	text = strings.ReplaceAll(text, " - ", " minus ")
	text = strings.ReplaceAll(text, " * ", " times ")
	text = strings.ReplaceAll(text, " / ", " divided by ")
	text = strings.ReplaceAll(text, " = ", " equals ")

	// Common programming symbols in prose context
	text = strings.ReplaceAll(text, "null", "null value")
	text = strings.ReplaceAll(text, "nil", "nil value")

	return text
}

// removeEmojis removes emoji characters from the text.
func (s *ResponseSanitizer) removeEmojis(text string) string {
	return s.patterns.emoji.ReplaceAllString(text, "")
}

// cleanupWhitespace normalizes whitespace in the text.
func (s *ResponseSanitizer) cleanupWhitespace(text string) string {
	p := s.patterns

	// Remove leading whitespace from lines
	text = p.leadingSpaces.ReplaceAllString(text, "")

	// Replace multiple spaces with single space
	text = p.multipleSpaces.ReplaceAllString(text, " ")

	// Replace multiple newlines with double newline
	text = p.multipleNewlines.ReplaceAllString(text, "\n\n")

	return text
}

// SanitizeForSentence performs minimal sanitization suitable for a single sentence.
// This is faster than full Sanitize() for streaming use cases.
func (s *ResponseSanitizer) SanitizeForSentence(text string) string {
	s.initPatterns()

	// Convert symbols only - skip markdown removal as stream sanitizer handles code blocks
	text = s.convertSymbols(text)
	text = s.removeEmojis(text)

	// Basic whitespace cleanup
	text = s.patterns.multipleSpaces.ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}

// RemoveCodeBlocks removes code blocks and returns both the cleaned text
// and the number of code blocks that were removed.
func (s *ResponseSanitizer) RemoveCodeBlocks(text string) (cleaned string, count int) {
	s.initPatterns()

	matches := s.patterns.codeBlock.FindAllString(text, -1)
	count = len(matches)
	cleaned = s.patterns.codeBlock.ReplaceAllString(text, " ")

	return cleaned, count
}

// ExtractCodeBlocks extracts all code blocks from the text.
// Returns the code blocks in order of appearance.
func (s *ResponseSanitizer) ExtractCodeBlocks(text string) []CodeBlock {
	s.initPatterns()

	var blocks []CodeBlock
	codeBlockWithLang := regexp.MustCompile("(?s)```(\\w*)\\n?([^`]*)```")

	matches := codeBlockWithLang.FindAllStringSubmatch(text, -1)
	for i, match := range matches {
		if len(match) >= 3 {
			blocks = append(blocks, CodeBlock{
				Index:    i,
				Language: match[1],
				Code:     strings.TrimSpace(match[2]),
			})
		}
	}

	return blocks
}

// CodeBlock represents an extracted code block.
type CodeBlock struct {
	Index    int    // Order of appearance (0-based)
	Language string // Language hint (e.g., "go", "python")
	Code     string // The code content
}

// QuickClean performs fast basic cleaning without regex.
// Use this for performance-critical paths where full sanitization isn't needed.
func QuickClean(text string) string {
	// Simple replacements without regex
	text = strings.ReplaceAll(text, "**", "")
	text = strings.ReplaceAll(text, "__", "")
	text = strings.ReplaceAll(text, "```", "")
	text = strings.ReplaceAll(text, "`", "")
	text = strings.ReplaceAll(text, "##", "")
	text = strings.ReplaceAll(text, "#", "")
	text = strings.ReplaceAll(text, "> ", "")

	return strings.TrimSpace(text)
}

// IsCodeHeavy returns true if the text contains significant code content.
// This can be used to decide whether to summarize instead of speaking verbatim.
func (s *ResponseSanitizer) IsCodeHeavy(text string) bool {
	s.initPatterns()

	// Count code blocks
	codeMatches := s.patterns.codeBlock.FindAllString(text, -1)
	if len(codeMatches) >= 2 {
		return true
	}

	// Calculate code percentage
	totalLen := len(text)
	if totalLen == 0 {
		return false
	}

	codeLen := 0
	for _, match := range codeMatches {
		codeLen += len(match)
	}

	// More than 50% code
	return float64(codeLen)/float64(totalLen) > 0.5
}
