package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// BashTool executes shell commands with security controls.
type BashTool struct {
	// Shell to use (default: /bin/bash or /bin/sh)
	shell string

	// Environment variables to inject
	env []string

	// OutputHandler for streaming output
	outputHandler OutputHandler

	// MaxOutputSize limits output capture (default: 10MB)
	maxOutputSize int

	// Patterns for risk assessment
	destructivePatterns []*regexp.Regexp
	networkPatterns     []*regexp.Regexp
	systemPatterns      []*regexp.Regexp
}

// BashOption configures the BashTool.
type BashOption func(*BashTool)

// WithShell sets the shell executable.
func WithShell(shell string) BashOption {
	return func(b *BashTool) {
		b.shell = shell
	}
}

// WithEnvironment adds environment variables.
func WithEnvironment(env []string) BashOption {
	return func(b *BashTool) {
		b.env = append(b.env, env...)
	}
}

// WithOutputHandler sets the streaming output handler.
func WithOutputHandler(handler OutputHandler) BashOption {
	return func(b *BashTool) {
		b.outputHandler = handler
	}
}

// WithMaxOutputSize sets the maximum output buffer size.
func WithMaxOutputSize(size int) BashOption {
	return func(b *BashTool) {
		b.maxOutputSize = size
	}
}

// NewBashTool creates a new bash tool.
func NewBashTool(opts ...BashOption) *BashTool {
	b := &BashTool{
		shell:         findShell(),
		maxOutputSize: 10 * 1024 * 1024, // 10MB default
	}

	// Compile risk assessment patterns
	b.compilePatterns()

	// Apply options
	for _, opt := range opts {
		opt(b)
	}

	return b
}

// findShell locates an available shell.
func findShell() string {
	shells := []string{"/bin/bash", "/bin/sh", "/usr/bin/bash", "/usr/bin/sh"}
	for _, shell := range shells {
		if _, err := os.Stat(shell); err == nil {
			return shell
		}
	}
	return "/bin/sh"
}

// compilePatterns prepares regex patterns for risk assessment.
func (b *BashTool) compilePatterns() {
	// Destructive patterns (High/Critical risk)
	b.destructivePatterns = []*regexp.Regexp{
		regexp.MustCompile(`rm\s+-[rf]*\s+`),          // rm with flags
		regexp.MustCompile(`rmdir\s+`),                // rmdir
		regexp.MustCompile(`>\s*/`),                   // Redirect to root paths
		regexp.MustCompile(`dd\s+`),                   // dd command
		regexp.MustCompile(`mkfs\b`),                  // Format filesystem
		regexp.MustCompile(`fdisk\b`),                 // Partition tool
		regexp.MustCompile(`chmod\s+-R\s+`),           // Recursive chmod
		regexp.MustCompile(`chown\s+-R\s+`),           // Recursive chown
		regexp.MustCompile(`truncate\s+`),             // Truncate files
		regexp.MustCompile(`shred\s+`),                // Secure delete
		regexp.MustCompile(`>\s*\|?\s*/dev/(sd|hd)`),  // Write to block devices
		regexp.MustCompile(`:()\s*\{\s*:\|:&\s*\}\s*;`), // Fork bomb
	}

	// Network patterns (Medium risk)
	b.networkPatterns = []*regexp.Regexp{
		regexp.MustCompile(`curl\s+`),
		regexp.MustCompile(`wget\s+`),
		regexp.MustCompile(`nc\s+`),       // netcat
		regexp.MustCompile(`ncat\s+`),
		regexp.MustCompile(`ssh\s+`),
		regexp.MustCompile(`scp\s+`),
		regexp.MustCompile(`rsync\s+`),
		regexp.MustCompile(`ftp\s+`),
		regexp.MustCompile(`sftp\s+`),
		regexp.MustCompile(`telnet\s+`),
		regexp.MustCompile(`ping\s+`),
		regexp.MustCompile(`traceroute\s+`),
		regexp.MustCompile(`nmap\s+`),
	}

	// System modification patterns (High risk)
	b.systemPatterns = []*regexp.Regexp{
		regexp.MustCompile(`sudo\s+`),
		regexp.MustCompile(`su\s+`),
		regexp.MustCompile(`systemctl\s+`),
		regexp.MustCompile(`service\s+`),
		regexp.MustCompile(`apt(-get)?\s+`),
		regexp.MustCompile(`yum\s+`),
		regexp.MustCompile(`dnf\s+`),
		regexp.MustCompile(`brew\s+`),
		regexp.MustCompile(`npm\s+install\s+-g`),
		regexp.MustCompile(`pip\s+install\s+`),
		regexp.MustCompile(`mount\s+`),
		regexp.MustCompile(`umount\s+`),
		regexp.MustCompile(`kill\s+`),
		regexp.MustCompile(`pkill\s+`),
		regexp.MustCompile(`killall\s+`),
		regexp.MustCompile(`reboot\b`),
		regexp.MustCompile(`shutdown\b`),
		regexp.MustCompile(`halt\b`),
		regexp.MustCompile(`poweroff\b`),
	}
}

// Name returns the tool identifier.
func (b *BashTool) Name() ToolType {
	return ToolBash
}

// Validate checks if the request is valid.
func (b *BashTool) Validate(req *ToolRequest) error {
	if req.Tool != ToolBash {
		return fmt.Errorf("wrong tool type: expected %s, got %s", ToolBash, req.Tool)
	}

	if strings.TrimSpace(req.Input) == "" {
		return fmt.Errorf("command cannot be empty")
	}

	// Check for working directory validity
	if req.WorkingDir != "" {
		if !filepath.IsAbs(req.WorkingDir) {
			return fmt.Errorf("working directory must be absolute path")
		}

		info, err := os.Stat(req.WorkingDir)
		if err != nil {
			return fmt.Errorf("working directory does not exist: %s", req.WorkingDir)
		}
		if !info.IsDir() {
			return fmt.Errorf("working directory is not a directory: %s", req.WorkingDir)
		}
	}

	return nil
}

// AssessRisk evaluates the risk level of a command.
func (b *BashTool) AssessRisk(req *ToolRequest) RiskLevel {
	cmd := strings.ToLower(req.Input)

	// Check for critical operations first (rm -rf / as standalone, not as path prefix)
	// Match: "rm -rf /" or "rm -rf /*" but not "rm -rf /tmp"
	if strings.HasSuffix(strings.TrimSpace(cmd), "rm -rf /") ||
		strings.Contains(cmd, "rm -rf / ") ||
		strings.Contains(cmd, "rm -rf /*") {
		return RiskCritical
	}

	// Check for pipe to shell (High) - before network patterns
	if strings.Contains(cmd, "| sh") || strings.Contains(cmd, "| bash") ||
		strings.Contains(cmd, "|sh") || strings.Contains(cmd, "|bash") {
		return RiskHigh
	}

	// Check for destructive patterns (High)
	for _, pattern := range b.destructivePatterns {
		if pattern.MatchString(cmd) {
			return RiskHigh
		}
	}

	// Check for system patterns (High)
	for _, pattern := range b.systemPatterns {
		if pattern.MatchString(cmd) {
			return RiskHigh
		}
	}

	// Check for network patterns (Medium)
	for _, pattern := range b.networkPatterns {
		if pattern.MatchString(cmd) {
			return RiskMedium
		}
	}

	// Check for file write operations (Low)
	if strings.Contains(cmd, ">") || strings.Contains(cmd, ">>") {
		return RiskLow
	}

	// Default: safe read operations
	return RiskNone
}

// Execute runs the bash command.
func (b *BashTool) Execute(ctx context.Context, req *ToolRequest) (*ToolResult, error) {
	start := time.Now()

	// Create the command
	cmd := exec.CommandContext(ctx, b.shell, "-c", req.Input)

	// Set working directory
	if req.WorkingDir != "" {
		cmd.Dir = req.WorkingDir
	}

	// Set environment
	cmd.Env = append(os.Environ(), b.env...)

	// Add safe defaults
	cmd.Env = append(cmd.Env,
		"TERM=xterm-256color",
		"LC_ALL=en_US.UTF-8",
	)

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute
	err := cmd.Run()

	// Build result
	result := &ToolResult{
		Tool:     ToolBash,
		Duration: time.Since(start),
		Metadata: make(map[string]interface{}),
	}

	// Combine stdout and stderr (stderr first for visibility)
	var output strings.Builder
	if stderr.Len() > 0 {
		output.WriteString(stderr.String())
	}
	if stdout.Len() > 0 {
		if output.Len() > 0 {
			output.WriteString("\n")
		}
		output.WriteString(stdout.String())
	}
	result.Output = strings.TrimRight(output.String(), "\n")

	// Truncate if needed
	if len(result.Output) > b.maxOutputSize {
		result.Output = result.Output[:b.maxOutputSize] + "\n... [output truncated]"
	}

	// Handle exit code
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
		result.Metadata["pid"] = cmd.ProcessState.Pid()
		result.Metadata["system_time"] = cmd.ProcessState.SystemTime().String()
		result.Metadata["user_time"] = cmd.ProcessState.UserTime().String()
	}

	// Determine success
	if err != nil {
		result.Success = false
		result.Error = err.Error()

		// Check for specific errors
		if ctx.Err() == context.DeadlineExceeded {
			result.Error = "command timed out"
		} else if ctx.Err() == context.Canceled {
			result.Error = "command cancelled"
		}
	} else {
		result.Success = result.ExitCode == 0
		if !result.Success && result.Error == "" {
			result.Error = fmt.Sprintf("command exited with code %d", result.ExitCode)
		}
	}

	// Notify output handler if set
	if b.outputHandler != nil {
		if stdout.Len() > 0 {
			b.outputHandler(stdout.String(), false)
		}
		if stderr.Len() > 0 {
			b.outputHandler(stderr.String(), true)
		}
	}

	return result, err
}

// ParseCommand extracts the base command name for logging.
func ParseCommand(input string) string {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return ""
	}

	cmd := parts[0]
	// Handle pipes
	if strings.Contains(cmd, "|") {
		parts := strings.Split(cmd, "|")
		cmd = strings.TrimSpace(parts[0])
	}

	return filepath.Base(cmd)
}

// SplitPipeline breaks a command into pipeline stages.
func SplitPipeline(input string) []string {
	// Simple split - doesn't handle quoted pipes
	parts := strings.Split(input, "|")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
