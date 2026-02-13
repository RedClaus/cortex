package web

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/normanking/pinky/internal/tools"
)

// ParseHTMLTool extracts content from HTML using CSS selectors.
type ParseHTMLTool struct{}

// NewParseHTMLTool creates a new ParseHTMLTool.
func NewParseHTMLTool() *ParseHTMLTool {
	return &ParseHTMLTool{}
}

func (p *ParseHTMLTool) Name() string {
	return "web_parse_html"
}

func (p *ParseHTMLTool) Description() string {
	return "Parse HTML content and extract data using CSS selectors. Returns matched elements as text or attributes."
}

func (p *ParseHTMLTool) Category() tools.ToolCategory {
	return tools.CategoryWeb
}

func (p *ParseHTMLTool) RiskLevel() tools.RiskLevel {
	return tools.RiskLow
}

func (p *ParseHTMLTool) Validate(input *tools.ToolInput) error {
	html, ok := input.Args["html"].(string)
	if !ok || html == "" {
		return fmt.Errorf("html content is required")
	}

	selector, ok := input.Args["selector"].(string)
	if !ok || selector == "" {
		return fmt.Errorf("CSS selector is required")
	}

	return nil
}

func (p *ParseHTMLTool) Execute(ctx context.Context, input *tools.ToolInput) (*tools.ToolOutput, error) {
	start := time.Now()

	html := input.Args["html"].(string)
	selector := input.Args["selector"].(string)

	// Parse the HTML
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return &tools.ToolOutput{
			Success:  false,
			Error:    fmt.Sprintf("failed to parse HTML: %v", err),
			Duration: time.Since(start),
		}, nil
	}

	// Determine what to extract
	extractAttr := ""
	if attr, ok := input.Args["attribute"].(string); ok {
		extractAttr = attr
	}

	// Find matching elements
	var results []string
	doc.Find(selector).Each(func(i int, s *goquery.Selection) {
		if extractAttr != "" {
			// Extract specific attribute
			if val, exists := s.Attr(extractAttr); exists {
				results = append(results, val)
			}
		} else {
			// Extract text content
			results = append(results, strings.TrimSpace(s.Text()))
		}
	})

	// Check for limit
	limit := -1
	if l, ok := input.Args["limit"].(float64); ok {
		limit = int(l)
	} else if l, ok := input.Args["limit"].(int); ok {
		limit = l
	}

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	// Format output
	var output string
	if len(results) == 0 {
		output = "No matches found"
	} else if len(results) == 1 {
		output = results[0]
	} else {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Found %d matches:\n", len(results)))
		for i, r := range results {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, r))
		}
		output = sb.String()
	}

	return &tools.ToolOutput{
		Success:  len(results) > 0,
		Output:   output,
		Duration: time.Since(start),
	}, nil
}
