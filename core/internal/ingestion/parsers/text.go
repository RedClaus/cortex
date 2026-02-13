package parsers

import (
	"bufio"
	"strings"
)

// TextParser parses plain text documents.
type TextParser struct{}

// NewTextParser creates a new text parser.
func NewTextParser() *TextParser {
	return &TextParser{}
}

// Parse parses plain text content into a structured document.
// Since plain text has no inherent structure, we create a single section.
func (p *TextParser) Parse(content string) (*ParsedDocument, error) {
	doc := &ParsedDocument{
		Title: extractTextTitle(content),
		Sections: []*Section{
			{
				Level:       1,
				Title:       "Content",
				Content:     strings.TrimSpace(content),
				CodeBlocks:  make([]*CodeBlock, 0),
				Subsections: make([]*Section, 0),
				StartLine:   1,
				EndLine:     countLines(content),
			},
		},
	}

	return doc, nil
}

// Format returns the format identifier.
func (p *TextParser) Format() string {
	return "text"
}

// extractTextTitle attempts to extract a title from the first line.
func extractTextTitle(content string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	if scanner.Scan() {
		firstLine := strings.TrimSpace(scanner.Text())
		if firstLine != "" {
			if len(firstLine) > 100 {
				return firstLine[:100] + "..."
			}
			return firstLine
		}
	}
	return "Text Document"
}

// countLines counts the number of lines in the content.
func countLines(content string) int {
	count := 0
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		count++
	}
	return count
}
