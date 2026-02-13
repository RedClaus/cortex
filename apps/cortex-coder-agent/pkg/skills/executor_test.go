package skills

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutor_ExecuteByName(t *testing.T) {
	registry := NewRegistry("", "", "")
	skill := &Skill{
		Name:        "test-skill",
		Version:     "1.0.0",
		Description: "A test skill",
		Triggers:    []Trigger{{Type: "selection"}},
		Template:    "Hello {{.Selection}}!",
	}
	registry.Register(skill)

	executor := NewExecutor(registry, nil)

	ctx := context.Background()
	result, err := executor.ExecuteByName(ctx, "test-skill", "World")
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "Hello World!", result.Prompt)
	assert.Equal(t, "test-skill", result.SkillName)
}

func TestExecutor_ExecuteByCommand(t *testing.T) {
	registry := NewRegistry("", "", "")
	skill := &Skill{
		Name:        "explain-skill",
		Version:     "1.0.0",
		Description: "Explain skill",
		Triggers: []Trigger{
			{Type: "command", Command: "explain"},
		},
		Template: "Explain: {{.UserQuery}}",
	}
	registry.Register(skill)

	executor := NewExecutor(registry, nil)

	ctx := context.Background()
	result, err := executor.ExecuteByCommand(ctx, "explain", []string{"code"})
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "Explain: code", result.Prompt)
}

func TestExecutor_ExecuteSelection(t *testing.T) {
	registry := NewRegistry("", "", "")
	skill := &Skill{
		Name:        "selection-skill",
		Version:     "1.0.0",
		Description: "Selection skill",
		Triggers:    []Trigger{{Type: "selection"}},
		Template:    "Selected: {{.Selection}} on line {{.LineNumber}}",
	}
	registry.Register(skill)

	executor := NewExecutor(registry, nil)

	ctx := context.Background()
	result, err := executor.ExecuteSelection(ctx, "my code", "/path/file.go", 42)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "Selected: my code on line 42", result.Prompt)
	assert.Equal(t, 42, result.Context.LineNumber)
	assert.Equal(t, "/path/file.go", result.Context.FilePath)
}

func TestExecutor_Execute_SkillNotFound(t *testing.T) {
	registry := NewRegistry("", "", "")
	executor := NewExecutor(registry, nil)

	ctx := context.Background()
	_, err := executor.ExecuteByName(ctx, "nonexistent", "")
	assert.Error(t, err)
}

func TestExecutor_Execute_CommandNotFound(t *testing.T) {
	registry := NewRegistry("", "", "")
	executor := NewExecutor(registry, nil)

	ctx := context.Background()
	_, err := executor.ExecuteByCommand(ctx, "unknown", nil)
	assert.Error(t, err)
}

func TestExecutor_Execute_TemplateError(t *testing.T) {
	registry := NewRegistry("", "", "")
	skill := &Skill{
		Name:        "broken-skill",
		Version:     "1.0.0",
		Description: "Broken skill",
		Triggers:    []Trigger{{Type: "selection"}},
		Template:    "{{.NonExistent}}",
	}
	registry.Register(skill)

	executor := NewExecutor(registry, nil)

	ctx := context.Background()
	result, err := executor.ExecuteByName(ctx, "broken-skill", "")
	assert.Error(t, err)
	assert.False(t, result.Success)
}

func TestExecutor_ExecuteWithFileContext(t *testing.T) {
	registry := NewRegistry("", "", "")
	skill := &Skill{
		Name:        "file-skill",
		Version:     "1.0.0",
		Description: "File skill",
		Triggers:    []Trigger{{Type: "selection"}},
		Template:    "File: {{.FilePath}}, Lang: {{.Language}}, Package: {{.PackageName}}",
	}
	registry.Register(skill)

	executor := NewExecutor(registry, nil)

	ctx := context.Background()
	result, err := executor.Execute(ctx, ExecutionOptions{
		SkillName: "file-skill",
		Selection: "code",
		FilePath:  "/path/to/main.go",
	})
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.Prompt, "File: /path/to/main.go")
	assert.Contains(t, result.Prompt, "Lang: go")
}

func TestExecutor_GetRegistry(t *testing.T) {
	registry := NewRegistry("", "", "")
	executor := NewExecutor(registry, nil)

	assert.Equal(t, registry, executor.GetRegistry())
}

func TestExecutor_SetClient(t *testing.T) {
	registry := NewRegistry("", "", "")
	executor := NewExecutor(registry, nil)

	// Initially client is nil
	assert.Nil(t, executor.client)
}

func TestExecutionResult_Structure(t *testing.T) {
	result := &ExecutionResult{
		SkillName: "test",
		Prompt:    "prompt",
		Response:  "response",
		Success:   true,
		Metadata:  map[string]interface{}{"key": "value"},
	}

	assert.Equal(t, "test", result.SkillName)
	assert.Equal(t, "prompt", result.Prompt)
	assert.Equal(t, "response", result.Response)
	assert.True(t, result.Success)
	assert.Equal(t, "value", result.Metadata["key"])
}
