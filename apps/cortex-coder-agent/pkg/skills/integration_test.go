// Package skills provides an integration test for the skills system
package skills

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_SkillLoading tests the full skill loading and execution pipeline
func TestIntegration_SkillLoading(t *testing.T) {
	// Create a temporary directory for test skills
	tempDir := t.TempDir()

	// Create a test skill file
	skillContent := `
name: test-skill
version: "1.0.0"
description: A test skill for integration testing
triggers:
  - type: selection
template: |
  You selected: {{.Selection}}
  File: {{.FilePath}}
  Language: {{.Language}}
`
	skillPath := tempDir + "/test-skill.yaml"
	err := os.WriteFile(skillPath, []byte(skillContent), 0644)
	require.NoError(t, err)

	// Create registry and load skills
	registry := NewRegistry(tempDir, "", "")
	err = registry.loadFromDir(tempDir)
	require.NoError(t, err)

	// Verify skill was loaded
	assert.Equal(t, 1, registry.Count())

	// Get the skill
	skill, ok := registry.Get("test-skill")
	require.True(t, ok)
	assert.Equal(t, "test-skill", skill.Name)
	assert.Equal(t, "A test skill for integration testing", skill.Description)

	// Test template execution
	engine := registry.GetTemplateEngine()
	ctx := NewContext().
		WithSelection("my code").
		WithFilePath("/path/to/file.go").
		WithLanguage("go")

	result, err := engine.Execute(skill.Template, ctx)
	require.NoError(t, err)
	assert.Contains(t, result, "You selected: my code")
	assert.Contains(t, result, "File: /path/to/file.go")
	assert.Contains(t, result, "Language: go")
}

// TestIntegration_SkillExecution tests the executor
func TestIntegration_SkillExecution(t *testing.T) {
	// Create a test skill
	skill := &Skill{
		Name:        "exec-test",
		Version:     "1.0.0",
		Description: "Executor test",
		Triggers:    []Trigger{{Type: "selection"}},
		Template:    "Process: {{.Selection}}",
	}

	// Create registry and executor
	registry := NewRegistry("", "", "")
	registry.Register(skill)

	executor := NewExecutor(registry, nil)

	// Execute skill
	ctx := context.Background()
	result, err := executor.ExecuteByName(ctx, "exec-test", "test content")

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "exec-test", result.SkillName)
	assert.Equal(t, "Process: test content", result.Prompt)
	assert.NotNil(t, result.Context)
}

// TestIntegration_TemplateFunctions tests all built-in template functions
func TestIntegration_TemplateFunctions(t *testing.T) {
	engine := NewEngine()

	ctx := NewContext().
		WithFilePath("/path/to/myfile.go").
		WithSelection("line1\nline2\nline3")

	// Test basename
	result, err := engine.Execute("{{basename .FilePath}}", ctx)
	require.NoError(t, err)
	assert.Equal(t, "myfile.go", result)

	// Test dirname
	result, err = engine.Execute("{{dirname .FilePath}}", ctx)
	require.NoError(t, err)
	assert.Equal(t, "/path/to", result)

	// Test ext
	result, err = engine.Execute("{{ext .FilePath}}", ctx)
	require.NoError(t, err)
	assert.Equal(t, ".go", result)

	// Test toUpper
	result, err = engine.Execute("{{toUpper \"hello\"}}", ctx)
	require.NoError(t, err)
	assert.Equal(t, "HELLO", result)

	// Test toLower
	result, err = engine.Execute("{{toLower \"HELLO\"}}", ctx)
	require.NoError(t, err)
	assert.Equal(t, "hello", result)

	// Test quote
	result, err = engine.Execute("{{quote \"hello\"}}", ctx)
	require.NoError(t, err)
	assert.Equal(t, "\"hello\"", result)

	// Test indent - use context variable that contains newlines
	indentTmpl := `{{indent "  " .Selection}}`
	result, err = engine.Execute(indentTmpl, ctx)
	require.NoError(t, err)
	assert.Equal(t, "  line1\n  line2\n  line3", result)

	// Test join
	result, err = engine.Execute("{{join \", \" \"a\" \"b\" \"c\"}}", ctx)
	require.NoError(t, err)
	assert.Equal(t, "a, b, c", result)
}

// TestIntegration_ContextBuilder tests the context builder
func TestIntegration_ContextBuilder(t *testing.T) {
	ctx := NewContextBuilder().
		FromSelection("code", 10).
		FromFile("/path/to/file.py").
		FromCommand("explain", nil).
		FromFunction("test_func").
		FromClass("TestClass").
		Build()

	assert.Equal(t, "code", ctx.Selection)
	assert.Equal(t, "code", ctx.Code)
	assert.Equal(t, 10, ctx.LineNumber)
	assert.Equal(t, "/path/to/file.py", ctx.FilePath)
	assert.Equal(t, "python", ctx.Language)
	assert.Equal(t, "explain", ctx.UserQuery)
	assert.Equal(t, "test_func", ctx.FunctionName)
	assert.Equal(t, "TestClass", ctx.ClassName)
}

// TestIntegration_SkillDiscovery tests skill discovery
func TestIntegration_SkillDiscovery(t *testing.T) {
	tempDir := t.TempDir()

	// Create multiple skills
	skills := []struct {
		name    string
		trigger string
	}{
		{"skill1", "explain"},
		{"skill2", "refactor"},
		{"skill3", "test"},
	}

	for _, s := range skills {
		content := `
name: ` + s.name + `
version: "1.0.0"
description: ` + s.name + ` skill
triggers:
  - type: command
    command: ` + s.trigger + `
template: |
  ` + s.trigger + `
`
		err := os.WriteFile(tempDir+"/"+s.name+".yaml", []byte(content), 0644)
		require.NoError(t, err)
	}

	// Load skills
	registry := NewRegistry(tempDir, "", "")
	err := registry.loadFromDir(tempDir)
	require.NoError(t, err)

	// Verify all skills loaded
	assert.Equal(t, 3, registry.Count())

	// Verify command triggers
	for _, s := range skills {
		skill, ok := registry.GetByCommand(s.trigger)
		assert.True(t, ok, "should find skill for command: "+s.trigger)
		if ok {
			assert.Equal(t, s.name, skill.Name)
		}
	}
}

// TestIntegration_ContextBuilder_LanguageDetection tests language detection
func TestIntegration_ContextBuilder_LanguageDetection(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/path/to/main.go", "go"},
		{"/path/to/script.py", "python"},
		{"/path/to/app.js", "javascript"},
		{"/path/to/app.ts", "javascript"},
		{"/path/to/app.tsx", "javascript"},
		{"/path/to/lib.rs", "rust"},
		{"/path/to/Main.java", "java"},
		{"/path/to/module.cpp", "cpp"},
		{"/path/to/file.c", "c"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			ctx := NewContextBuilder().FromFile(tt.path).Build()
			assert.Equal(t, tt.expected, ctx.Language, "for path: %s", tt.path)
		})
	}
}
