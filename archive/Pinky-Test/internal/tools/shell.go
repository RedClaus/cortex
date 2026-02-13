package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// ShellTool executes shell commands
type ShellTool struct {
	allowedShells []string
	defaultShell  string
	timeout       time.Duration
	maxOutput     int
	env           []string
}

// ShellConfig configures the shell tool
type ShellConfig struct {
	AllowedShells []string
	DefaultShell  string
	Timeout       time.Duration
	MaxOutput     int
	Env           []string
}

// DefaultShellConfig returns platform-appropriate defaults
func DefaultShellConfig() *ShellConfig {
	shells := []string{"bash", "sh"}
	defaultShell := "bash"

	if runtime.GOOS == "windows" {
		shells = []string{"cmd", "powershell"}
		defaultShell = "cmd"
	}

	return &ShellConfig{
		AllowedShells: shells,
		DefaultShell:  defaultShell,
		Timeout:       2 * time.Minute,
		MaxOutput:     1024 * 1024, // 1MB
	}
}

// NewShellTool creates a new shell tool
func NewShellTool(cfg *ShellConfig) *ShellTool {
	if cfg == nil {
		cfg = DefaultShellConfig()
	}

	return &ShellTool{
		allowedShells: cfg.AllowedShells,
		defaultShell:  cfg.DefaultShell,
		timeout:       cfg.Timeout,
		maxOutput:     cfg.MaxOutput,
		env:           cfg.Env,
	}
}

func (t *ShellTool) Name() string           { return "shell" }
func (t *ShellTool) Category() ToolCategory { return CategoryShell }
func (t *ShellTool) RiskLevel() RiskLevel   { return RiskHigh }

func (t *ShellTool) Description() string {
	return "Execute shell commands. Supports bash, sh, and platform-specific shells."
}

// Spec returns the tool specification for LLM function calling
func (t *ShellTool) Spec() *ToolSpec {
	return &ToolSpec{
		Name:        t.Name(),
		Description: t.Description(),
		Category:    t.Category(),
		RiskLevel:   t.RiskLevel(),
		Parameters: &ParamSchema{
			Type: "object",
			Properties: map[string]*ParamProp{
				"command": {
					Type:        "string",
					Description: "The shell command to execute",
				},
				"shell": {
					Type:        "string",
					Description: "Shell to use (bash, sh, etc.). Defaults to bash.",
					Enum:        t.allowedShells,
					Default:     t.defaultShell,
				},
				"working_dir": {
					Type:        "string",
					Description: "Working directory for command execution",
				},
			},
			Required: []string{"command"},
		},
	}
}

// Validate checks if the input is valid
func (t *ShellTool) Validate(input *ToolInput) error {
	if input == nil {
		return errors.New("input is nil")
	}
	if input.Command == "" {
		return errors.New("command is required")
	}

	// Check if specified shell is allowed
	if shell, ok := input.Args["shell"].(string); ok && shell != "" {
		if !t.isShellAllowed(shell) {
			return fmt.Errorf("shell %q is not allowed", shell)
		}
	}

	return nil
}

// Execute runs the shell command
func (t *ShellTool) Execute(ctx context.Context, input *ToolInput) (*ToolOutput, error) {
	shell := t.defaultShell
	if s, ok := input.Args["shell"].(string); ok && s != "" {
		shell = s
	}

	// Determine shell flags
	var shellArgs []string
	switch shell {
	case "bash", "sh", "zsh":
		shellArgs = []string{"-c", input.Command}
	case "cmd":
		shellArgs = []string{"/C", input.Command}
	case "powershell":
		shellArgs = []string{"-Command", input.Command}
	default:
		shellArgs = []string{"-c", input.Command}
	}

	cmd := exec.CommandContext(ctx, shell, shellArgs...)

	// Set working directory
	if input.WorkingDir != "" {
		if _, err := os.Stat(input.WorkingDir); err != nil {
			return &ToolOutput{
				Success: false,
				Error:   fmt.Sprintf("working directory does not exist: %s", input.WorkingDir),
			}, nil
		}
		cmd.Dir = input.WorkingDir
	}

	// Set environment
	cmd.Env = os.Environ()
	if len(t.env) > 0 {
		cmd.Env = append(cmd.Env, t.env...)
	}

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	// Combine stdout and stderr
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
			result.Error = fmt.Sprintf("exit code %d", exitErr.ExitCode())
		} else if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			result.Error = "command timed out"
		} else {
			result.Error = err.Error()
		}
	}

	return result, nil
}

func (t *ShellTool) isShellAllowed(shell string) bool {
	for _, allowed := range t.allowedShells {
		if allowed == shell {
			return true
		}
	}
	return false
}
