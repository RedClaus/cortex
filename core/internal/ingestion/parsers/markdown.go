package parsers

import (
	"bufio"
	"regexp"
	"strings"
)

// MarkdownParser parses Markdown documents into structured sections.
type MarkdownParser struct {
	// Configuration
	preserveCodeBlocks bool
	preserveLinks      bool
}

// NewMarkdownParser creates a new Markdown parser.
func NewMarkdownParser() *MarkdownParser {
	return &MarkdownParser{
		preserveCodeBlocks: true,
		preserveLinks:      true,
	}
}

// Parse parses Markdown content into a structured document.
func (p *MarkdownParser) Parse(content string) (*ParsedDocument, error) {
	doc := &ParsedDocument{
		Sections: make([]*Section, 0),
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	var currentSection *Section
	var sectionStack []*Section
	var contentBuffer strings.Builder
	var inCodeBlock bool
	var currentCodeBlock *CodeBlock
	lineNum := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		// Detect code block boundaries
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if !inCodeBlock {
				// Start of code block
				inCodeBlock = true
				lang := strings.TrimPrefix(strings.TrimSpace(line), "```")
				currentCodeBlock = &CodeBlock{
					Language: lang,
					Line:     lineNum,
				}
				continue
			} else {
				// End of code block
				inCodeBlock = false
				if currentCodeBlock != nil && currentSection != nil {
					currentSection.CodeBlocks = append(currentSection.CodeBlocks, currentCodeBlock)
				}
				currentCodeBlock = nil
				continue
			}
		}

		// If we're in a code block, accumulate code
		if inCodeBlock {
			if currentCodeBlock != nil {
				if currentCodeBlock.Code != "" {
					currentCodeBlock.Code += "\n"
				}
				currentCodeBlock.Code += line
			}
			continue
		}

		// Detect headers
		if headerMatch := regexp.MustCompile(`^(#{1,6})\s+(.+)`).FindStringSubmatch(line); headerMatch != nil {
			// Flush current section content
			if currentSection != nil {
				currentSection.Content = strings.TrimSpace(contentBuffer.String())
				contentBuffer.Reset()
			}

			level := len(headerMatch[1])
			title := strings.TrimSpace(headerMatch[2])

			// Set document title from first H1
			if level == 1 && doc.Title == "" {
				doc.Title = title
			}

			// Create new section
			newSection := &Section{
				Level:       level,
				Title:       title,
				StartLine:   lineNum,
				CodeBlocks:  make([]*CodeBlock, 0),
				Subsections: make([]*Section, 0),
			}

			// Pop stack until we find the parent level
			for len(sectionStack) > 0 && sectionStack[len(sectionStack)-1].Level >= level {
				popped := sectionStack[len(sectionStack)-1]
				popped.EndLine = lineNum - 1
				sectionStack = sectionStack[:len(sectionStack)-1]
			}

			// Add to parent or document root
			if len(sectionStack) > 0 {
				parent := sectionStack[len(sectionStack)-1]
				parent.Subsections = append(parent.Subsections, newSection)
			} else {
				doc.Sections = append(doc.Sections, newSection)
			}

			// Push new section onto stack
			sectionStack = append(sectionStack, newSection)
			currentSection = newSection
			continue
		}

		// Accumulate regular content
		if currentSection != nil {
			if contentBuffer.Len() > 0 {
				contentBuffer.WriteString("\n")
			}
			contentBuffer.WriteString(line)
		}
	}

	// Flush final section content
	if currentSection != nil {
		currentSection.Content = strings.TrimSpace(contentBuffer.String())
		currentSection.EndLine = lineNum
	}

	// Close remaining sections
	for _, section := range sectionStack {
		if section.EndLine == 0 {
			section.EndLine = lineNum
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return doc, nil
}

// Format returns the format identifier.
func (p *MarkdownParser) Format() string {
	return "markdown"
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPER FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════════

// extractTitle attempts to extract a title from Markdown content.
func extractTitle(content string) string {
	// Look for first H1
	re := regexp.MustCompile(`(?m)^#\s+(.+)$`)
	if matches := re.FindStringSubmatch(content); matches != nil {
		return strings.TrimSpace(matches[1])
	}

	// Fallback: first non-empty line
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			if len(line) > 100 {
				return line[:100] + "..."
			}
			return line
		}
	}

	return "Untitled Document"
}
