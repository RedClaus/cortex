package skills

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewContext(t *testing.T) {
	ctx := NewContext()
	assert.NotNil(t, ctx)
	assert.NotNil(t, ctx.Metadata)
	assert.Empty(t, ctx.Code)
	assert.Empty(t, ctx.FilePath)
}

func TestContext_Builders(t *testing.T) {
	ctx := NewContext().
		WithCode("test code").
		WithFilePath("/path/to/file.go").
		WithLineNumber(42).
		WithLanguage("go").
		WithProjectPath("/project").
		WithGitBranch("main").
		WithUserQuery("explain this").
		WithFunctionName("TestFunction").
		WithClassName("TestClass").
		WithCommandArgs([]string{"arg1", "arg2"}).
		SetMetadata("key", "value")

	assert.Equal(t, "test code", ctx.Code)
	assert.Equal(t, "test code", ctx.Selection)
	assert.Equal(t, "/path/to/file.go", ctx.FilePath)
	assert.Equal(t, 42, ctx.LineNumber)
	assert.Equal(t, "go", ctx.Language)
	assert.Equal(t, "/project", ctx.ProjectPath)
	assert.Equal(t, "main", ctx.GitBranch)
	assert.Equal(t, "explain this", ctx.UserQuery)
	assert.Equal(t, "TestFunction", ctx.FunctionName)
	assert.Equal(t, "TestClass", ctx.ClassName)
	assert.Equal(t, []string{"arg1", "arg2"}, ctx.CommandArgs)
	assert.Equal(t, "value", ctx.Metadata["key"])
}

func TestContextBuilder_FromSelection(t *testing.T) {
	ctx := NewContextBuilder().
		FromSelection("selected code", 10).
		Build()

	assert.Equal(t, "selected code", ctx.Selection)
	assert.Equal(t, "selected code", ctx.Code)
	assert.Equal(t, 10, ctx.LineNumber)
}

func TestContextBuilder_FromFile(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/path/to/file.go", "go"},
		{"/path/to/script.py", "python"},
		{"/path/to/app.js", "javascript"},
		{"/path/to/main.rs", "rust"},
		{"/path/to/Main.java", "java"},
	}

	for _, tt := range tests {
		ctx := NewContextBuilder().
			FromFile(tt.path).
			Build()
		assert.Equal(t, tt.expected, ctx.Language, "for path: %s", tt.path)
	}
}

func TestContextBuilder_FromCommand(t *testing.T) {
	ctx := NewContextBuilder().
		FromCommand("refactor this", []string{"--dry-run"}).
		Build()

	assert.Equal(t, "refactor this", ctx.UserQuery)
	assert.Equal(t, []string{"--dry-run"}, ctx.CommandArgs)
}

func TestContextBuilder_Clone(t *testing.T) {
	originalBuilder := NewContextBuilder().
		FromSelection("code", 1).
		WithMetadata("key", "original")

	clone := originalBuilder.Clone()

	// Modify clone
	clone.ctx.SetMetadata("key", "modified")

	// Clone should have modified value
	assert.Equal(t, "modified", clone.ctx.Metadata["key"])

	// Selection should be copied
	assert.Equal(t, "code", clone.ctx.Selection)
}

func TestContextBuilder_GetOS(t *testing.T) {
	builder := NewContextBuilder()
	os := builder.GetOS()
	assert.NotEmpty(t, os)
}

func TestContextBuilder_GetArch(t *testing.T) {
	builder := NewContextBuilder()
	arch := builder.GetArch()
	assert.NotEmpty(t, arch)
}

func TestContextBuilder_DetectLanguage(t *testing.T) {
	tests := []struct {
		content  string
		initial  string
		expected string
	}{
		{"func main() {}", "", "go"},
		{"def hello(): pass", "", "python"},
		{"function test() {}", "", "javascript"},
		{"struct Foo {}", "", "rust"},
		{"public class Main {}", "", "java"},
		{"unknown code", "", ""}, // Unknown without initial context returns empty
		{"unknown code", "python", "python"}, // Keep initial if unknown
	}

	for _, tt := range tests {
		t.Run(tt.content, func(t *testing.T) {
			builder := NewContextBuilder()
			if tt.initial != "" {
				builder.ctx.Language = tt.initial
			}
			result := builder.DetectLanguage(tt.content)
			t.Logf("content=%q initial=%q expected=%q got=%q", tt.content, tt.initial, tt.expected, result)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContextBuilder_BuildDefault(t *testing.T) {
	ctx := ContextBuilder{}.BuildDefault()

	assert.NotNil(t, ctx)
	assert.NotNil(t, ctx.ProjectPath)
	// Language and project type may be empty depending on cwd
}
