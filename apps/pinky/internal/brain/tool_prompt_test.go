package brain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToolsDescription(t *testing.T) {
	tests := []struct {
		name     string
		tools    []ToolSpec
		contains []string
	}{
		{
			name:     "empty tools",
			tools:    nil,
			contains: nil,
		},
		{
			name: "single tool",
			tools: []ToolSpec{
				{
					Name:        "read_file",
					Description: "Read the contents of a file",
					Parameters: map[string]ParameterSpec{
						"path": {
							Type:        "string",
							Description: "The file path to read",
							Required:    true,
						},
					},
				},
			},
			contains: []string{
				"Available Tools",
				"<tool>tool_name</tool><params>",
				"read_file",
				"Read the contents of a file",
				"path",
				"(required)",
			},
		},
		{
			name: "multiple tools",
			tools: []ToolSpec{
				{
					Name:        "bash",
					Description: "Execute a bash command",
				},
				{
					Name:        "write_file",
					Description: "Write content to a file",
				},
			},
			contains: []string{
				"bash",
				"write_file",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToolsDescription(tt.tools)
			for _, expected := range tt.contains {
				assert.Contains(t, result, expected)
			}
		})
	}
}

func TestParseToolCalls(t *testing.T) {
	tests := []struct {
		name            string
		response        string
		expectedCalls   int
		expectedTool    string
		expectedParams  map[string]any
		expectedContent string
	}{
		{
			name:            "no tool call",
			response:        "Hello, how can I help you?",
			expectedCalls:   0,
			expectedContent: "Hello, how can I help you?",
		},
		{
			name:            "single tool call",
			response:        `Let me read that file for you. <tool>read_file</tool><params>{"path": "/tmp/test.txt"}</params>`,
			expectedCalls:   1,
			expectedTool:    "read_file",
			expectedParams:  map[string]any{"path": "/tmp/test.txt"},
			expectedContent: "Let me read that file for you.",
		},
		{
			name:            "tool call with no surrounding text",
			response:        `<tool>bash</tool><params>{"command": "ls -la"}</params>`,
			expectedCalls:   1,
			expectedTool:    "bash",
			expectedParams:  map[string]any{"command": "ls -la"},
			expectedContent: "",
		},
		{
			name:            "tool call with text after",
			response:        `<tool>list_files</tool><params>{"directory": "."}</params> I'll check the directory.`,
			expectedCalls:   1,
			expectedTool:    "list_files",
			expectedParams:  map[string]any{"directory": "."},
			expectedContent: "I'll check the directory.",
		},
		{
			name:            "multiple tool calls - only first extracted",
			response:        `<tool>first</tool><params>{"a": 1}</params> Some text <tool>second</tool><params>{"b": 2}</params>`,
			expectedCalls:   2,
			expectedTool:    "first",
			expectedParams:  map[string]any{"a": float64(1)},
			expectedContent: "Some text",
		},
		{
			name:            "invalid json in params",
			response:        `<tool>bad</tool><params>{invalid}</params> Still here`,
			expectedCalls:   0,
			expectedContent: "Still here",
		},
		{
			name:            "missing params tag",
			response:        `<tool>orphan</tool> No params here`,
			expectedCalls:   0,
			expectedContent: "No params here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls, content := ParseToolCalls(tt.response)

			assert.Len(t, calls, tt.expectedCalls)
			assert.Equal(t, tt.expectedContent, content)

			if tt.expectedCalls > 0 {
				assert.Equal(t, tt.expectedTool, calls[0].Tool)
				if tt.expectedParams != nil {
					for k, v := range tt.expectedParams {
						assert.Equal(t, v, calls[0].Input[k], "param %s mismatch", k)
					}
				}
			}
		})
	}
}

func TestFormatToolResult(t *testing.T) {
	t.Run("success result", func(t *testing.T) {
		result := ToolResult{
			ToolCallID: "call_1",
			Success:    true,
			Output:     "file contents here",
		}
		formatted := FormatToolResult("read_file", result)
		assert.Contains(t, formatted, "[Tool Result: read_file]")
		assert.Contains(t, formatted, "file contents here")
	})

	t.Run("error result", func(t *testing.T) {
		result := ToolResult{
			ToolCallID: "call_2",
			Success:    false,
			Error:      "file not found",
		}
		formatted := FormatToolResult("read_file", result)
		assert.Contains(t, formatted, "[Tool Result: read_file]")
		assert.Contains(t, formatted, "Error: file not found")
	})
}
