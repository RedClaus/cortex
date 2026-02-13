package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRegistry(t *testing.T) {
	r := NewRegistry()

	// Test empty registry
	tools := r.List()
	if len(tools) != 0 {
		t.Errorf("expected empty registry, got %d tools", len(tools))
	}

	// Register a tool
	shell := NewShellTool(nil)
	r.Register(shell)

	// Test Get
	got, ok := r.Get("shell")
	if !ok {
		t.Error("expected to find shell tool")
	}
	if got.Name() != "shell" {
		t.Errorf("expected shell, got %s", got.Name())
	}

	// Test List
	tools = r.List()
	if len(tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(tools))
	}

	// Test ListByCategory
	shellTools := r.ListByCategory(CategoryShell)
	if len(shellTools) != 1 {
		t.Errorf("expected 1 shell tool, got %d", len(shellTools))
	}

	webTools := r.ListByCategory(CategoryWeb)
	if len(webTools) != 0 {
		t.Errorf("expected 0 web tools, got %d", len(webTools))
	}

	// Test ListByRisk
	highRisk := r.ListByRisk(RiskHigh)
	if len(highRisk) != 1 {
		t.Errorf("expected 1 high risk tool, got %d", len(highRisk))
	}
}

func TestDefaultRegistry(t *testing.T) {
	r := NewDefaultRegistry(nil)

	// Should have all 8 tools (including web_search)
	tools := r.List()
	if len(tools) != 8 {
		t.Errorf("expected 8 tools, got %d", len(tools))
	}

	// Check each tool exists
	expectedTools := []string{"shell", "files", "web", "web_search", "git", "code", "system", "api"}
	for _, name := range expectedTools {
		if _, ok := r.Get(name); !ok {
			t.Errorf("expected to find %s tool", name)
		}
	}
}

func TestShellTool(t *testing.T) {
	shell := NewShellTool(nil)

	// Test metadata
	if shell.Name() != "shell" {
		t.Errorf("expected name shell, got %s", shell.Name())
	}
	if shell.Category() != CategoryShell {
		t.Errorf("expected category shell, got %s", shell.Category())
	}
	if shell.RiskLevel() != RiskHigh {
		t.Errorf("expected high risk, got %s", shell.RiskLevel())
	}

	// Test Validate
	if err := shell.Validate(nil); err == nil {
		t.Error("expected error for nil input")
	}

	if err := shell.Validate(&ToolInput{}); err == nil {
		t.Error("expected error for empty command")
	}

	if err := shell.Validate(&ToolInput{Command: "echo hello"}); err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Test Execute
	ctx := context.Background()
	output, err := shell.Execute(ctx, &ToolInput{Command: "echo hello"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !output.Success {
		t.Errorf("expected success, got error: %s", output.Error)
	}
	if output.Output != "hello" {
		t.Errorf("expected 'hello', got '%s'", output.Output)
	}
}

func TestFilesTool(t *testing.T) {
	files := NewFilesTool(nil)

	// Test metadata
	if files.Name() != "files" {
		t.Errorf("expected name files, got %s", files.Name())
	}
	if files.Category() != CategoryFiles {
		t.Errorf("expected category files, got %s", files.Category())
	}

	// Create temp dir for testing
	tmpDir := t.TempDir()

	// Test write
	ctx := context.Background()
	input := &ToolInput{
		Args: map[string]any{
			"operation": "write",
			"path":      filepath.Join(tmpDir, "test.txt"),
			"content":   "hello world",
		},
	}

	output, err := files.Execute(ctx, input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !output.Success {
		t.Errorf("expected success, got error: %s", output.Error)
	}

	// Test read
	input = &ToolInput{
		Args: map[string]any{
			"operation": "read",
			"path":      filepath.Join(tmpDir, "test.txt"),
		},
	}

	output, err = files.Execute(ctx, input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if output.Output != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", output.Output)
	}

	// Test exists
	input = &ToolInput{
		Args: map[string]any{
			"operation": "exists",
			"path":      filepath.Join(tmpDir, "test.txt"),
		},
	}

	output, err = files.Execute(ctx, input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if output.Output == "false" {
		t.Error("expected file to exist")
	}

	// Test list
	input = &ToolInput{
		Args: map[string]any{
			"operation": "list",
			"path":      tmpDir,
		},
	}

	output, err = files.Execute(ctx, input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !output.Success {
		t.Errorf("expected success, got error: %s", output.Error)
	}

	// Test delete
	input = &ToolInput{
		Args: map[string]any{
			"operation": "delete",
			"path":      filepath.Join(tmpDir, "test.txt"),
		},
	}

	output, err = files.Execute(ctx, input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !output.Success {
		t.Errorf("expected success, got error: %s", output.Error)
	}

	// Verify deletion
	if _, err := os.Stat(filepath.Join(tmpDir, "test.txt")); !os.IsNotExist(err) {
		t.Error("expected file to be deleted")
	}
}

func TestGitTool(t *testing.T) {
	git := NewGitTool(nil)

	// Test metadata
	if git.Name() != "git" {
		t.Errorf("expected name git, got %s", git.Name())
	}
	if git.Category() != CategoryGit {
		t.Errorf("expected category git, got %s", git.Category())
	}

	// Test Validate
	if err := git.Validate(nil); err == nil {
		t.Error("expected error for nil input")
	}

	if err := git.Validate(&ToolInput{Args: map[string]any{}}); err == nil {
		t.Error("expected error for missing operation")
	}

	if err := git.Validate(&ToolInput{Args: map[string]any{"operation": "invalid"}}); err == nil {
		t.Error("expected error for invalid operation")
	}

	if err := git.Validate(&ToolInput{Args: map[string]any{"operation": "status"}}); err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Test commit requires message
	if err := git.Validate(&ToolInput{Args: map[string]any{"operation": "commit"}}); err == nil {
		t.Error("expected error for commit without message")
	}
}

func TestCodeTool(t *testing.T) {
	code := NewCodeTool(nil)

	// Test metadata
	if code.Name() != "code" {
		t.Errorf("expected name code, got %s", code.Name())
	}
	if code.RiskLevel() != RiskHigh {
		t.Errorf("expected high risk, got %s", code.RiskLevel())
	}

	// Test Validate
	if err := code.Validate(&ToolInput{Args: map[string]any{"language": "python"}}); err == nil {
		t.Error("expected error for missing code")
	}

	if err := code.Validate(&ToolInput{Args: map[string]any{"language": "ruby", "code": "puts 'hi'"}}); err == nil {
		t.Error("expected error for unsupported language")
	}

	// Test Python execution (if python3 is available)
	ctx := context.Background()
	input := &ToolInput{
		Args: map[string]any{
			"language": "python",
			"code":     "print(2 + 2)",
		},
	}

	output, err := code.Execute(ctx, input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// Note: This will fail if python3 is not installed
	if output.Success && output.Output != "4" {
		t.Errorf("expected '4', got '%s'", output.Output)
	}
}

func TestToolSpec(t *testing.T) {
	shell := NewShellTool(nil)
	spec := GetSpec(shell)

	if spec.Name != "shell" {
		t.Errorf("expected name shell, got %s", spec.Name)
	}
	if spec.Category != CategoryShell {
		t.Errorf("expected category shell, got %s", spec.Category)
	}
	if spec.RiskLevel != RiskHigh {
		t.Errorf("expected high risk, got %s", spec.RiskLevel)
	}
	if spec.Parameters == nil {
		t.Error("expected parameters to be set")
	}
	if spec.Parameters.Type != "object" {
		t.Errorf("expected type object, got %s", spec.Parameters.Type)
	}

	// Test JSON serialization
	json, err := spec.ToJSON()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(json) == 0 {
		t.Error("expected non-empty JSON")
	}
}

func TestExecutor(t *testing.T) {
	registry := NewDefaultRegistry(nil)

	// Create executor without permission checker (all allowed)
	executor := NewExecutor(registry, nil, nil, nil)

	ctx := context.Background()

	// Test simple execution
	result, err := executor.Execute(ctx, &ExecuteRequest{
		Tool: "shell",
		Input: &ToolInput{
			Command: "echo test",
		},
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Output.Output != "test" {
		t.Errorf("expected 'test', got '%s'", result.Output.Output)
	}

	// Test tool not found
	_, err = executor.Execute(ctx, &ExecuteRequest{
		Tool:  "nonexistent",
		Input: &ToolInput{},
	})
	if err == nil {
		t.Error("expected error for nonexistent tool")
	}

	// Test timeout - use a command that definitely takes longer than timeout
	shortExecutor := NewExecutor(registry, nil, nil, &ExecutorConfig{
		DefaultTimeout: 50 * time.Millisecond,
		MaxConcurrent:  10,
		MaxOutputSize:  1024,
	})

	result, err = shortExecutor.Execute(ctx, &ExecuteRequest{
		Tool: "shell",
		Input: &ToolInput{
			Command: "sleep 5", // 5 seconds, but timeout is 50ms
		},
		Timeout: 50 * time.Millisecond, // Explicit timeout
	})
	// The shell tool catches the timeout and returns it in output.Error
	// So we check the output for the timeout message
	if err != nil {
		// Direct error from executor (context cancelled)
		if err != ErrExecutionTimeout && err != context.DeadlineExceeded {
			t.Errorf("expected timeout-related error, got %v", err)
		}
	} else if result.Output.Success {
		t.Error("expected command to fail due to timeout")
	} else if result.Output.Error != "command timed out" && result.Output.Error != "" {
		// Should have a timeout error message
		t.Logf("timeout test: success=%v, error=%s", result.Output.Success, result.Output.Error)
	}
}

func TestExecutorShutdown(t *testing.T) {
	registry := NewDefaultRegistry(nil)
	executor := NewExecutor(registry, nil, nil, nil)

	ctx := context.Background()

	// Shutdown
	if err := executor.Shutdown(ctx); err != nil {
		t.Errorf("unexpected shutdown error: %v", err)
	}

	// Try to execute after shutdown
	_, err := executor.Execute(ctx, &ExecuteRequest{
		Tool:  "shell",
		Input: &ToolInput{Command: "echo hello"},
	})
	if err != ErrExecutorShutdown {
		t.Errorf("expected shutdown error, got %v", err)
	}
}

// MockPermissionChecker for testing
type mockPermissionChecker struct {
	needsApproval bool
}

func (m *mockPermissionChecker) NeedsApproval(userID, tool, command, riskLevel string) bool {
	return m.needsApproval
}

func TestExecutorWithPermissions(t *testing.T) {
	registry := NewDefaultRegistry(nil)

	// Test with permissions that always require approval
	executor := NewExecutor(registry, &mockPermissionChecker{needsApproval: true}, nil, nil)

	ctx := context.Background()

	// Should fail because approval is required but no handler
	_, err := executor.Execute(ctx, &ExecuteRequest{
		Tool: "shell",
		Input: &ToolInput{
			Command: "echo test",
			UserID:  "test-user",
		},
	})
	if err != ErrApprovalRequired {
		t.Errorf("expected approval required error, got %v", err)
	}

	// With skip approval flag
	result, err := executor.Execute(ctx, &ExecuteRequest{
		Tool: "shell",
		Input: &ToolInput{
			Command: "echo test",
		},
		SkipApproval: true,
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Output.Output != "test" {
		t.Errorf("expected 'test', got '%s'", result.Output.Output)
	}
}

func TestSystemTool(t *testing.T) {
	sys := NewSystemTool(nil)

	// Test metadata
	if sys.Name() != "system" {
		t.Errorf("expected name system, got %s", sys.Name())
	}
	if sys.Category() != CategorySystem {
		t.Errorf("expected category system, got %s", sys.Category())
	}
	if sys.RiskLevel() != RiskMedium {
		t.Errorf("expected medium risk, got %s", sys.RiskLevel())
	}

	// Test Validate
	if err := sys.Validate(nil); err == nil {
		t.Error("expected error for nil input")
	}

	if err := sys.Validate(&ToolInput{Args: map[string]any{}}); err == nil {
		t.Error("expected error for missing operation")
	}

	if err := sys.Validate(&ToolInput{Args: map[string]any{"operation": "invalid"}}); err == nil {
		t.Error("expected error for invalid operation")
	}

	// notify requires message
	if err := sys.Validate(&ToolInput{Args: map[string]any{"operation": "notify"}}); err == nil {
		t.Error("expected error for notify without message")
	}
	if err := sys.Validate(&ToolInput{Args: map[string]any{"operation": "notify", "message": "test"}}); err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// clipboard_read needs no extra args
	if err := sys.Validate(&ToolInput{Args: map[string]any{"operation": "clipboard_read"}}); err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// clipboard_write requires content
	if err := sys.Validate(&ToolInput{Args: map[string]any{"operation": "clipboard_write"}}); err == nil {
		t.Error("expected error for clipboard_write without content")
	}
	if err := sys.Validate(&ToolInput{Args: map[string]any{"operation": "clipboard_write", "content": "test"}}); err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// open requires target
	if err := sys.Validate(&ToolInput{Args: map[string]any{"operation": "open"}}); err == nil {
		t.Error("expected error for open without target")
	}
	if err := sys.Validate(&ToolInput{Args: map[string]any{"operation": "open", "target": "https://example.com"}}); err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Test Spec
	spec := sys.Spec()
	if spec.Name != "system" {
		t.Errorf("expected spec name system, got %s", spec.Name)
	}
	if spec.Parameters == nil {
		t.Error("expected parameters to be set")
	}
	if _, ok := spec.Parameters.Properties["operation"]; !ok {
		t.Error("expected operation property in spec")
	}
}
