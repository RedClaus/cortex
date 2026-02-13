package web

import (
	"context"
	"strings"
	"testing"

	"github.com/normanking/pinky/internal/tools"
)

func TestParseHTMLTool_Name(t *testing.T) {
	p := NewParseHTMLTool()
	if p.Name() != "web_parse_html" {
		t.Errorf("expected name 'web_parse_html', got '%s'", p.Name())
	}
}

func TestParseHTMLTool_Category(t *testing.T) {
	p := NewParseHTMLTool()
	if p.Category() != tools.CategoryWeb {
		t.Errorf("expected category 'web', got '%s'", p.Category())
	}
}

func TestParseHTMLTool_RiskLevel(t *testing.T) {
	p := NewParseHTMLTool()
	if p.RiskLevel() != tools.RiskLow {
		t.Errorf("expected risk level 'low', got '%s'", p.RiskLevel())
	}
}

func TestParseHTMLTool_Validate(t *testing.T) {
	p := NewParseHTMLTool()

	tests := []struct {
		name    string
		input   *tools.ToolInput
		wantErr bool
	}{
		{
			name: "valid input",
			input: &tools.ToolInput{
				Args: map[string]any{
					"html":     "<html><body><h1>Test</h1></body></html>",
					"selector": "h1",
				},
			},
			wantErr: false,
		},
		{
			name: "missing html",
			input: &tools.ToolInput{
				Args: map[string]any{
					"selector": "h1",
				},
			},
			wantErr: true,
		},
		{
			name: "missing selector",
			input: &tools.ToolInput{
				Args: map[string]any{
					"html": "<html><body><h1>Test</h1></body></html>",
				},
			},
			wantErr: true,
		},
		{
			name: "empty html",
			input: &tools.ToolInput{
				Args: map[string]any{
					"html":     "",
					"selector": "h1",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := p.Validate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseHTMLTool_Execute_Text(t *testing.T) {
	p := NewParseHTMLTool()

	html := `
		<html>
		<body>
			<h1>Welcome to Pinky</h1>
			<p class="intro">Hello World</p>
			<div class="content">
				<p>Paragraph 1</p>
				<p>Paragraph 2</p>
			</div>
		</body>
		</html>
	`

	tests := []struct {
		name     string
		selector string
		want     string
	}{
		{
			name:     "select h1",
			selector: "h1",
			want:     "Welcome to Pinky",
		},
		{
			name:     "select by class",
			selector: ".intro",
			want:     "Hello World",
		},
		{
			name:     "select multiple",
			selector: ".content p",
			want:     "2 matches",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &tools.ToolInput{
				Args: map[string]any{
					"html":     html,
					"selector": tt.selector,
				},
			}

			output, err := p.Execute(context.Background(), input)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			if !strings.Contains(output.Output, tt.want) {
				t.Errorf("expected output to contain '%s', got '%s'", tt.want, output.Output)
			}
		})
	}
}

func TestParseHTMLTool_Execute_Attribute(t *testing.T) {
	p := NewParseHTMLTool()

	html := `
		<html>
		<body>
			<a href="https://example.com">Link 1</a>
			<a href="https://pinky.dev">Link 2</a>
			<img src="/image.png" alt="Test Image">
		</body>
		</html>
	`

	tests := []struct {
		name      string
		selector  string
		attribute string
		want      string
	}{
		{
			name:      "extract href",
			selector:  "a",
			attribute: "href",
			want:      "https://example.com",
		},
		{
			name:      "extract src",
			selector:  "img",
			attribute: "src",
			want:      "/image.png",
		},
		{
			name:      "extract alt",
			selector:  "img",
			attribute: "alt",
			want:      "Test Image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &tools.ToolInput{
				Args: map[string]any{
					"html":      html,
					"selector":  tt.selector,
					"attribute": tt.attribute,
				},
			}

			output, err := p.Execute(context.Background(), input)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			if !strings.Contains(output.Output, tt.want) {
				t.Errorf("expected output to contain '%s', got '%s'", tt.want, output.Output)
			}
		})
	}
}

func TestParseHTMLTool_Execute_Limit(t *testing.T) {
	p := NewParseHTMLTool()

	html := `
		<ul>
			<li>Item 1</li>
			<li>Item 2</li>
			<li>Item 3</li>
			<li>Item 4</li>
			<li>Item 5</li>
		</ul>
	`

	input := &tools.ToolInput{
		Args: map[string]any{
			"html":     html,
			"selector": "li",
			"limit":    float64(2), // JSON numbers are float64
		},
	}

	output, err := p.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Should only have 2 matches
	if !strings.Contains(output.Output, "2 matches") {
		t.Errorf("expected 2 matches, got: %s", output.Output)
	}
}

func TestParseHTMLTool_Execute_NoMatches(t *testing.T) {
	p := NewParseHTMLTool()

	html := "<html><body><p>Hello</p></body></html>"

	input := &tools.ToolInput{
		Args: map[string]any{
			"html":     html,
			"selector": "h1", // No h1 in the HTML
		},
	}

	output, err := p.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if output.Success {
		t.Error("expected failure when no matches found")
	}

	if !strings.Contains(output.Output, "No matches") {
		t.Errorf("expected 'No matches' message, got: %s", output.Output)
	}
}
