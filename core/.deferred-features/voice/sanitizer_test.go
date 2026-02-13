package voice

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ═══════════════════════════════════════════════════════════════════════════════
// RESPONSE SANITIZER TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestNewResponseSanitizer(t *testing.T) {
	s := NewResponseSanitizer()
	require.NotNil(t, s)
}

// Markdown removal tests
func TestSanitizeMarkdownHeaders(t *testing.T) {
	s := NewResponseSanitizer()

	tests := []struct {
		input    string
		expected string
	}{
		{"# Header 1", "Header 1"},
		{"## Header 2", "Header 2"},
		{"### Header 3", "Header 3"},
		{"#### Header 4", "Header 4"},
		{"##### Header 5", "Header 5"},
		{"###### Header 6", "Header 6"},
	}

	for _, tt := range tests {
		result := s.Sanitize(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}

func TestSanitizeMarkdownBold(t *testing.T) {
	s := NewResponseSanitizer()

	tests := []struct {
		input    string
		expected string
	}{
		{"This is **bold** text", "This is bold text"},
		{"This is __bold__ text", "This is bold text"},
		{"**All bold**", "All bold"},
	}

	for _, tt := range tests {
		result := s.Sanitize(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}

func TestSanitizeMarkdownItalic(t *testing.T) {
	s := NewResponseSanitizer()

	tests := []struct {
		input    string
		expected string
	}{
		{"This is *italic* text", "This is italic text"},
		{"This is _italic_ text", "This is italic text"},
	}

	for _, tt := range tests {
		result := s.Sanitize(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}

func TestSanitizeMarkdownStrikethrough(t *testing.T) {
	s := NewResponseSanitizer()

	result := s.Sanitize("This is ~~deleted~~ text")
	assert.Equal(t, "This is deleted text", result)
}

func TestSanitizeMarkdownLinks(t *testing.T) {
	s := NewResponseSanitizer()

	tests := []struct {
		input    string
		expected string
	}{
		{"Check [this link](https://example.com)", "Check this link"},
		{"See [docs](./docs.md) for more", "See docs for more"},
	}

	for _, tt := range tests {
		result := s.Sanitize(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}

func TestSanitizeMarkdownImages(t *testing.T) {
	s := NewResponseSanitizer()

	result := s.Sanitize("Here is ![an image](./image.png)")
	assert.Equal(t, "Here is an image", result)
}

func TestSanitizeMarkdownCodeBlocks(t *testing.T) {
	s := NewResponseSanitizer()

	input := "Here is code:\n```go\nfunc main() {}\n```\nEnd."
	result := s.Sanitize(input)

	assert.Contains(t, result, "[code block]")
	assert.NotContains(t, result, "func main")
}

func TestSanitizeMarkdownInlineCode(t *testing.T) {
	s := NewResponseSanitizer()

	result := s.Sanitize("Use the `fmt.Println` function")
	assert.NotContains(t, result, "`")
	assert.NotContains(t, result, "fmt.Println")
}

func TestSanitizeMarkdownLists(t *testing.T) {
	s := NewResponseSanitizer()

	// Bullet lists
	input := "- Item 1\n- Item 2\n* Item 3\n+ Item 4"
	result := s.Sanitize(input)
	assert.NotContains(t, result, "- ")
	assert.NotContains(t, result, "* ")
	assert.NotContains(t, result, "+ ")

	// Numbered lists
	input = "1. First\n2. Second\n3. Third"
	result = s.Sanitize(input)
	assert.NotContains(t, result, "1.")
	assert.NotContains(t, result, "2.")
}

func TestSanitizeMarkdownBlockquotes(t *testing.T) {
	s := NewResponseSanitizer()

	result := s.Sanitize("> This is a quote")
	assert.NotContains(t, result, ">")
	assert.Contains(t, result, "This is a quote")
}

// Symbol conversion tests
func TestSanitizeCurrency(t *testing.T) {
	s := NewResponseSanitizer()

	tests := []struct {
		input    string
		expected string
	}{
		{"It costs $42", "It costs 42 dollars"},
		{"Price is $42.50", "Price is 42 dollars and 50 cents"},
		{"Only $5.00 left", "Only 5 dollars left"},
		{"$100 dollars", "100 dollars dollars"}, // Intentional - user error
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := s.Sanitize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizePercentage(t *testing.T) {
	s := NewResponseSanitizer()

	tests := []struct {
		input    string
		expected string
	}{
		{"About 42%", "About 42 percent"},
		{"100% complete", "100 percent complete"},
		{"Only 5.5%", "Only 5.5 percent"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := s.Sanitize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeTemperature(t *testing.T) {
	s := NewResponseSanitizer()

	tests := []struct {
		input    string
		expected string
	}{
		{"It's 72°F today", "It's 72 degrees fahrenheit today"},
		{"Set to 20°C", "Set to 20 degrees celsius"},
		{"Temperature: 98°F", "Temperature: 98 degrees fahrenheit"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := s.Sanitize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeDegree(t *testing.T) {
	s := NewResponseSanitizer()

	result := s.Sanitize("Rotate 90° clockwise")
	assert.Equal(t, "Rotate 90 degrees clockwise", result)
}

func TestSanitizeArrows(t *testing.T) {
	s := NewResponseSanitizer()

	tests := []struct {
		input    string
		expected string
	}{
		{"A -> B", "A to B"},
		{"input => output", "input becomes output"},
		{"x->y", "x to y"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := s.Sanitize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeAmpersand(t *testing.T) {
	s := NewResponseSanitizer()

	result := s.Sanitize("Tom & Jerry")
	assert.Equal(t, "Tom and Jerry", result)
}

func TestSanitizeMathOperators(t *testing.T) {
	s := NewResponseSanitizer()

	tests := []struct {
		input    string
		expected string
	}{
		{"2 + 2", "2 plus 2"},
		{"5 - 3", "5 minus 3"},
		{"4 * 2", "4 times 2"},
		{"8 / 4", "8 divided by 4"},
		{"x = 5", "x equals 5"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := s.Sanitize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeComparisonOperators(t *testing.T) {
	s := NewResponseSanitizer()

	tests := []struct {
		input    string
		expected string
	}{
		{"a != b", "a is not equal to b"},
		{"x == y", "x equals y"},
		{"n >= 5", "n is greater than or equal to 5"},
		{"n <= 5", "n is less than or equal to 5"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := s.Sanitize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeEllipsis(t *testing.T) {
	s := NewResponseSanitizer()

	result := s.Sanitize("Wait... thinking...")
	assert.NotContains(t, result, "...")
}

// Emoji removal tests
func TestSanitizeEmojis(t *testing.T) {
	s := NewResponseSanitizer()

	tests := []struct {
		input    string
		expected string
	}{
		{"Hello 👋 World", "Hello World"},
		{"Great! 🎉", "Great!"},
		{"🚀 Launch", "Launch"},
		{"Fire 🔥 and ice ❄️", "Fire and ice"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := s.Sanitize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Whitespace cleanup tests
func TestSanitizeMultipleSpaces(t *testing.T) {
	s := NewResponseSanitizer()

	result := s.Sanitize("Too    many    spaces")
	assert.Equal(t, "Too many spaces", result)
}

func TestSanitizeMultipleNewlines(t *testing.T) {
	s := NewResponseSanitizer()

	result := s.Sanitize("Line 1\n\n\n\nLine 2")
	assert.Equal(t, "Line 1\n\nLine 2", result)
}

func TestSanitizeLeadingWhitespace(t *testing.T) {
	s := NewResponseSanitizer()

	result := s.Sanitize("    Indented text")
	assert.Equal(t, "Indented text", result)
}

// SanitizeForSentence tests
func TestSanitizeForSentence(t *testing.T) {
	s := NewResponseSanitizer()

	// Should convert symbols but skip markdown
	result := s.SanitizeForSentence("Price is $42 👋")
	assert.Contains(t, result, "42 dollars")
	assert.NotContains(t, result, "👋")
}

// RemoveCodeBlocks tests
func TestRemoveCodeBlocks(t *testing.T) {
	s := NewResponseSanitizer()

	input := "Text\n```go\ncode\n```\nMore\n```python\ncode2\n```\nEnd"
	cleaned, count := s.RemoveCodeBlocks(input)

	assert.Equal(t, 2, count)
	assert.NotContains(t, cleaned, "```")
	assert.Contains(t, cleaned, "Text")
	assert.Contains(t, cleaned, "End")
}

// ExtractCodeBlocks tests
func TestExtractCodeBlocks(t *testing.T) {
	s := NewResponseSanitizer()

	input := "```go\nfunc main() {}\n```\n```python\nprint('hi')\n```"
	blocks := s.ExtractCodeBlocks(input)

	require.Len(t, blocks, 2)

	assert.Equal(t, 0, blocks[0].Index)
	assert.Equal(t, "go", blocks[0].Language)
	assert.Contains(t, blocks[0].Code, "func main")

	assert.Equal(t, 1, blocks[1].Index)
	assert.Equal(t, "python", blocks[1].Language)
	assert.Contains(t, blocks[1].Code, "print")
}

// QuickClean tests
func TestQuickClean(t *testing.T) {
	input := "**Bold** and `code` with ## header"
	result := QuickClean(input)

	assert.NotContains(t, result, "**")
	assert.NotContains(t, result, "`")
	assert.NotContains(t, result, "##")
	assert.Contains(t, result, "Bold")
}

// IsCodeHeavy tests
func TestIsCodeHeavy(t *testing.T) {
	s := NewResponseSanitizer()

	// Text-heavy content
	textHeavy := "This is a paragraph about programming. It explains concepts."
	assert.False(t, s.IsCodeHeavy(textHeavy))

	// Code-heavy content
	codeHeavy := "```go\nfunc main() {\n\tfmt.Println(\"Hello\")\n}\n```\n```go\nfunc other() {}\n```"
	assert.True(t, s.IsCodeHeavy(codeHeavy))

	// Mixed but mostly code
	mixed := "Here:\n```go\n" + strings.Repeat("code\n", 100) + "```"
	assert.True(t, s.IsCodeHeavy(mixed))
}

func TestIsCodeHeavyEmpty(t *testing.T) {
	s := NewResponseSanitizer()
	assert.False(t, s.IsCodeHeavy(""))
}

// Complex input tests
func TestSanitizeComplexInput(t *testing.T) {
	s := NewResponseSanitizer()

	input := `# Welcome 👋

Here's a **quick** example:

` + "```go" + `
func main() {
    fmt.Println("Hello")
}
` + "```" + `

The cost is $42.50 (about 50% off). Temperature is 72°F.

- Item 1
- Item 2

Check [our docs](https://example.com) for more info!

> This is important

Use ` + "`config.json`" + ` for settings.`

	result := s.Sanitize(input)

	// Headers removed
	assert.NotContains(t, result, "#")

	// Emojis removed
	assert.NotContains(t, result, "👋")

	// Bold removed
	assert.NotContains(t, result, "**")
	assert.Contains(t, result, "quick")

	// Code block replaced
	assert.Contains(t, result, "[code block]")
	assert.NotContains(t, result, "fmt.Println")

	// Currency converted
	assert.Contains(t, result, "42 dollars")

	// Percentage converted
	assert.Contains(t, result, "50 percent")

	// Temperature converted
	assert.Contains(t, result, "72 degrees fahrenheit")

	// List markers removed
	assert.NotContains(t, result, "- Item")

	// Link converted
	assert.Contains(t, result, "our docs")
	assert.NotContains(t, result, "https://")

	// Blockquote marker removed
	assert.NotContains(t, result, "> ")

	// Inline code removed
	assert.NotContains(t, result, "`")
}

// Performance test
func TestSanitizePerformance(t *testing.T) {
	s := NewResponseSanitizer()

	// Generate moderately complex input
	input := strings.Repeat("**Bold** text with $42 and 50% emoji 👋\n", 100)

	iterations := 1000
	start := time.Now()

	for i := 0; i < iterations; i++ {
		_ = s.Sanitize(input)
	}

	elapsed := time.Since(start)
	avgPerOp := elapsed / time.Duration(iterations)

	// Should be reasonably fast
	assert.Less(t, avgPerOp, 10*time.Millisecond,
		"Sanitize took %v per operation", avgPerOp)

	t.Logf("Sanitize: %v per operation (%d iterations)", avgPerOp, iterations)
}

// Pattern initialization is thread-safe
func TestSanitizerPatternInitialization(t *testing.T) {
	s := NewResponseSanitizer()

	// Call Sanitize from multiple goroutines
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_ = s.Sanitize("Test **input** with $42")
			done <- true
		}()
	}

	// Wait for all to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

// Edge cases
func TestSanitizeEmptyInput(t *testing.T) {
	s := NewResponseSanitizer()
	result := s.Sanitize("")
	assert.Empty(t, result)
}

func TestSanitizeWhitespaceOnly(t *testing.T) {
	s := NewResponseSanitizer()
	result := s.Sanitize("   \n\t  ")
	assert.Empty(t, result)
}

func TestSanitizeNoChangesNeeded(t *testing.T) {
	s := NewResponseSanitizer()
	input := "Plain text without any special formatting"
	result := s.Sanitize(input)
	assert.Equal(t, input, result)
}

func TestExtractCodeBlocksEmpty(t *testing.T) {
	s := NewResponseSanitizer()
	blocks := s.ExtractCodeBlocks("No code here")
	assert.Empty(t, blocks)
}

func TestRemoveCodeBlocksNone(t *testing.T) {
	s := NewResponseSanitizer()
	cleaned, count := s.RemoveCodeBlocks("No code blocks")
	assert.Equal(t, 0, count)
	assert.Equal(t, "No code blocks", cleaned)
}
