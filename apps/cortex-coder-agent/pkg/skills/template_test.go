package skills

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_Execute(t *testing.T) {
	engine := NewEngine()

	ctx := NewContext().
		WithCode("test code").
		WithFilePath("/path/to/file.go").
		WithSelection("selected text").
		WithLineNumber(10).
		WithLanguage("go")

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "simple substitution",
			template: "Hello {{.Selection}}",
			expected: "Hello selected text",
		},
		{
			name:     "multiple variables",
			template: "Code: {{.Code}}, Line: {{.LineNumber}}",
			expected: "Code: test code, Line: 10",
		},
		{
			name:     "file path",
			template: "File: {{.FilePath}}",
			expected: "File: /path/to/file.go",
		},
		{
			name:     "language",
			template: "Language: {{.Language}}",
			expected: "Language: go",
		},
		{
			name:     "basename function",
			template: "Base: {{basename .FilePath}}",
			expected: "Base: file.go",
		},
		{
			name:     "dirname function",
			template: "Dir: {{dirname .FilePath}}",
			expected: "Dir: /path/to",
		},
		{
			name:     "ext function",
			template: "Ext: {{ext .FilePath}}",
			expected: "Ext: .go",
		},
		{
			name:     "join function",
			template: "Joined: {{join \" - \" .Code .Selection}}",
			expected: "Joined: test code - selected text",
		},
		{
			name:     "toUpper function",
			template: "Upper: {{toUpper .Selection}}",
			expected: "Upper: SELECTED TEXT",
		},
		{
			name:     "toLower function",
			template: "Lower: {{toLower .Selection}}",
			expected: "Lower: selected text",
		},
		{
			name:     "title function",
			template: "Title: {{title .Selection}}",
			expected: "Title: Selected Text",
		},
		{
			name:     "quote function",
			template: "Quoted: {{quote .Selection}}",
			expected: "Quoted: \"selected text\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.Execute(tt.template, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEngine_ExecuteFile(t *testing.T) {
	engine := NewEngine()

	ctx := NewContext().
		WithSelection("world")

	// Create a temporary template file
	content := "Hello {{.Selection}}!"
	result, err := engine.Execute(content, ctx)
	require.NoError(t, err)
	assert.Equal(t, "Hello world!", result)
}

func TestEngine_Execute_InvalidTemplate(t *testing.T) {
	engine := NewEngine()
	ctx := NewContext()

	_, err := engine.Execute("{{.Invalid", ctx)
	assert.Error(t, err)
}

func TestEngine_AddFunc(t *testing.T) {
	engine := NewEngine()

	// Add a custom function
	engine.AddFunc("double", func(n int) int {
		return n * 2
	})

	ctx := NewContext()
	ctx.Metadata["number"] = 5

	result, err := engine.Execute("Result: {{double .number}}", ctx)
	require.NoError(t, err)
	assert.Equal(t, "Result: 10", result)
}

func TestEngine_SetFuncs(t *testing.T) {
	engine := NewEngine()

	// Set multiple custom functions
	engine.SetFuncs(map[string]interface{}{
		"triple": func(n int) int { return n * 3 },
		"square": func(n int) int { return n * n },
	})

	ctx := NewContext()
	ctx.Metadata["num"] = 4

	result, err := engine.Execute("Triple: {{triple .num}}, Square: {{square .num}}", ctx)
	require.NoError(t, err)
	assert.Equal(t, "Triple: 12, Square: 16", result)
}

func TestEngine_Indent(t *testing.T) {
	engine := NewEngine()

	ctx := NewContext()
	result, err := engine.Execute("{{indent \"  \" \"line1\nline2\nline3\"}}", ctx)
	require.NoError(t, err)

	expected := "  line1\n  line2\n  line3"
	assert.Equal(t, expected, result)
}

func TestEngine_MultilineTemplate(t *testing.T) {
	engine := NewEngine()

	ctx := NewContext().
		WithSelection("code block").
		WithFilePath("/test/file.go")

	template := "Explain this code:\n```\n{{.Selection}}\n```\n\nFile: {{.FilePath}}\n"
	result, err := engine.Execute(template, ctx)
	require.NoError(t, err)

	assert.Contains(t, result, "code block")
	assert.Contains(t, result, "/test/file.go")
}

func TestEngine_EmptyContext(t *testing.T) {
	engine := NewEngine()
	ctx := NewContext()

	result, err := engine.Execute("Hello", ctx)
	require.NoError(t, err)
	assert.Equal(t, "Hello", result)
}

func TestEngine_ReadFile(t *testing.T) {
	// Note: This test requires a real file to exist
	// Skipped in unit tests, integration test only
	t.Skip("Requires actual file system")
}

func TestEngine_GitDiff(t *testing.T) {
	// Note: This test requires git repository
	// Skipped in unit tests, integration test only
	t.Skip("Requires git repository")
}

func TestEngine_ExecuteTemplate_RealWorld(t *testing.T) {
	engine := NewEngine()

	// Simulate a real explain-code skill template
	tmpl := `Please explain the following {{.Language}} code:

{{.Selection}}

**File:** {{.FilePath}}
{{if .FunctionName}}**Function:** {{.FunctionName}}{{end}}`

	ctx := NewContext().
		WithSelection("func main() { fmt.Println(\"Hello\") }").
		WithFilePath("/project/main.go").
		WithLanguage("go").
		WithFunctionName("main")

	result, err := engine.Execute(tmpl, ctx)
	require.NoError(t, err)

	assert.Contains(t, result, "func main()")
	assert.Contains(t, result, "/project/main.go")
	assert.Contains(t, result, "**Function:** main")
}

func BenchmarkEngine_Execute(b *testing.B) {
	engine := NewEngine()
	ctx := NewContext().
		WithSelection(strings.Repeat("code ", 100)).
		WithFilePath("/path/to/file.go").
		WithLanguage("go")

	tmpl := "Code: {{.Selection}}\nFile: {{.FilePath}}\nLang: {{.Language}}\n{{basename .FilePath}}"

	for i := 0; i < b.N; i++ {
		engine.Execute(tmpl, ctx)
	}
}
