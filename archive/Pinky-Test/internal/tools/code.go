package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// CodeTool executes code snippets
type CodeTool struct {
	pythonPath string
	nodePath   string
	timeout    time.Duration
	maxOutput  int
	tempDir    string
}

// CodeConfig configures the code tool
type CodeConfig struct {
	PythonPath string
	NodePath   string
	Timeout    time.Duration
	MaxOutput  int
	TempDir    string
}

// DefaultCodeConfig returns sensible defaults
func DefaultCodeConfig() *CodeConfig {
	return &CodeConfig{
		PythonPath: "python3",
		NodePath:   "node",
		Timeout:    30 * time.Second,
		MaxOutput:  1024 * 1024, // 1MB
		TempDir:    os.TempDir(),
	}
}

// NewCodeTool creates a new code tool
func NewCodeTool(cfg *CodeConfig) *CodeTool {
	if cfg == nil {
		cfg = DefaultCodeConfig()
	}

	return &CodeTool{
		pythonPath: cfg.PythonPath,
		nodePath:   cfg.NodePath,
		timeout:    cfg.Timeout,
		maxOutput:  cfg.MaxOutput,
		tempDir:    cfg.TempDir,
	}
}

func (t *CodeTool) Name() string           { return "code" }
func (t *CodeTool) Category() ToolCategory { return CategoryCode }
func (t *CodeTool) RiskLevel() RiskLevel   { return RiskHigh }

func (t *CodeTool) Description() string {
	return "Execute code snippets in Python or JavaScript. Useful for calculations, data processing, and scripting."
}

// Spec returns the tool specification for LLM function calling
func (t *CodeTool) Spec() *ToolSpec {
	return &ToolSpec{
		Name:        t.Name(),
		Description: t.Description(),
		Category:    t.Category(),
		RiskLevel:   t.RiskLevel(),
		Parameters: &ParamSchema{
			Type: "object",
			Properties: map[string]*ParamProp{
				"language": {
					Type:        "string",
					Description: "Programming language",
					Enum:        []string{"python", "javascript", "node"},
				},
				"code": {
					Type:        "string",
					Description: "The code to execute",
				},
			},
			Required: []string{"language", "code"},
		},
	}
}

// Validate checks if the input is valid
func (t *CodeTool) Validate(input *ToolInput) error {
	if input == nil {
		return errors.New("input is nil")
	}

	lang, ok := input.Args["language"].(string)
	if !ok || lang == "" {
		return errors.New("language is required")
	}

	// Normalize language
	lang = strings.ToLower(lang)
	if lang != "python" && lang != "javascript" && lang != "node" {
		return fmt.Errorf("unsupported language: %s (use python or javascript)", lang)
	}

	code, ok := input.Args["code"].(string)
	if !ok || code == "" {
		return errors.New("code is required")
	}

	return nil
}

// Execute runs the code
func (t *CodeTool) Execute(ctx context.Context, input *ToolInput) (*ToolOutput, error) {
	lang := strings.ToLower(input.Args["language"].(string))
	code := input.Args["code"].(string)

	// Normalize javascript to node
	if lang == "javascript" {
		lang = "node"
	}

	// Create temp file for code
	var ext string
	var interpreter string

	switch lang {
	case "python":
		ext = ".py"
		interpreter = t.pythonPath
	case "node":
		ext = ".js"
		interpreter = t.nodePath
	}

	// Create temp file
	tempFile, err := os.CreateTemp(t.tempDir, fmt.Sprintf("pinky-code-*%s", ext))
	if err != nil {
		return &ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("failed to create temp file: %v", err),
		}, nil
	}
	defer os.Remove(tempFile.Name())

	if _, err := tempFile.WriteString(code); err != nil {
		tempFile.Close()
		return &ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("failed to write code: %v", err),
		}, nil
	}
	tempFile.Close()

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, interpreter, tempFile.Name())
	if input.WorkingDir != "" {
		cmd.Dir = input.WorkingDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err = cmd.Run()
	duration := time.Since(start)

	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += stderr.String()
	}

	// Truncate if necessary
	if len(output) > t.maxOutput {
		output = output[:t.maxOutput] + "\n... (output truncated)"
	}

	result := &ToolOutput{
		Success:  err == nil,
		Output:   strings.TrimSpace(output),
		Duration: duration,
	}

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.Error = fmt.Sprintf("%s exited with code %d", lang, exitErr.ExitCode())
		} else if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			result.Error = "code execution timed out"
		} else {
			result.Error = err.Error()
		}
	}

	return result, nil
}

