// Package resemble provides Resemble.ai API clients and webhook server for Voice Agents.
// This file implements the tool executor that bridges webhook server to Cortex's actual tools.
package resemble

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/logging"
)

// ═══════════════════════════════════════════════════════════════════════════════
// CONSTANTS
// ═══════════════════════════════════════════════════════════════════════════════

const (
	// MaxFileSize is the maximum file size for read operations (1MB).
	MaxFileSize = 1 * 1024 * 1024

	// MaxOutputSize is the maximum output size for command results (512KB).
	MaxOutputSize = 512 * 1024

	// DefaultCommandTimeout is the default timeout for bash commands.
	DefaultCommandTimeout = 30 * time.Second

	// MaxDirEntries is the maximum number of directory entries to list.
	MaxDirEntries = 1000
)

// ═══════════════════════════════════════════════════════════════════════════════
// TOOL EXECUTOR
// ═══════════════════════════════════════════════════════════════════════════════

// CortexToolExecutor bridges the webhook server to Cortex's actual tool implementations.
// It provides secure execution of tools with path validation and output limiting.
type CortexToolExecutor struct {
	workDir string
	log     *logging.Logger
}

// NewCortexToolExecutor creates a new tool executor with the specified working directory.
func NewCortexToolExecutor(workDir string) *CortexToolExecutor {
	// Normalize and validate work directory
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	// Ensure absolute path
	if !filepath.IsAbs(workDir) {
		absPath, err := filepath.Abs(workDir)
		if err == nil {
			workDir = absPath
		}
	}

	return &CortexToolExecutor{
		workDir: workDir,
		log:     logging.Global().WithComponent("ToolExecutor"),
	}
}

// Execute executes a tool by name with the provided arguments.
// It returns the result or an error if the tool execution fails.
func (e *CortexToolExecutor) Execute(ctx context.Context, toolName string, args map[string]any, sessionID string) (any, error) {
	e.log.Debug("Executing tool: %s (session=%s)", toolName, sessionID)

	switch toolName {
	case "bash":
		command, err := getStringArg(args, "command")
		if err != nil {
			return nil, fmt.Errorf("bash: %w", err)
		}
		return e.executeBash(ctx, command)

	case "read_file":
		path, err := getStringArg(args, "path")
		if err != nil {
			return nil, fmt.Errorf("read_file: %w", err)
		}
		return e.readFile(path)

	case "write_file":
		path, err := getStringArg(args, "path")
		if err != nil {
			return nil, fmt.Errorf("write_file: %w", err)
		}
		content, err := getStringArg(args, "content")
		if err != nil {
			return nil, fmt.Errorf("write_file: %w", err)
		}
		return e.writeFile(path, content)

	case "list_directory":
		path, err := getStringArg(args, "path")
		if err != nil {
			return nil, fmt.Errorf("list_directory: %w", err)
		}
		return e.listDirectory(path)

	case "glob":
		pattern, err := getStringArg(args, "pattern")
		if err != nil {
			return nil, fmt.Errorf("glob: %w", err)
		}
		return e.glob(pattern)

	case "grep":
		pattern, err := getStringArg(args, "pattern")
		if err != nil {
			return nil, fmt.Errorf("grep: %w", err)
		}
		path := getStringArgDefault(args, "path", e.workDir)
		return e.grep(ctx, pattern, path)

	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// BASH EXECUTION
// ═══════════════════════════════════════════════════════════════════════════════

// executeBash runs a shell command with timeout and output limiting.
func (e *CortexToolExecutor) executeBash(ctx context.Context, command string) (any, error) {
	e.log.Debug("Executing bash: %s", command)

	// Validate command is not empty
	if strings.TrimSpace(command) == "" {
		return nil, fmt.Errorf("command cannot be empty")
	}

	// Create context with timeout if not already set
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultCommandTimeout)
		defer cancel()
	}

	// Find available shell
	shell := findShell()

	// Create command
	cmd := exec.CommandContext(ctx, shell, "-c", command)
	cmd.Dir = e.workDir

	// Set environment
	cmd.Env = append(os.Environ(),
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
	result := map[string]any{
		"command": command,
	}

	// Combine stdout and stderr
	output := combineOutput(stdout.String(), stderr.String())
	output = truncateOutput(output, MaxOutputSize)
	result["output"] = output

	// Get exit code
	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	result["exit_code"] = exitCode

	// Handle errors
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result["error"] = "command timed out"
		} else if ctx.Err() == context.Canceled {
			result["error"] = "command cancelled"
		} else {
			result["error"] = err.Error()
		}
		return result, fmt.Errorf("command failed: %w", err)
	}

	if exitCode != 0 {
		result["error"] = fmt.Sprintf("command exited with code %d", exitCode)
		return result, fmt.Errorf("command exited with code %d", exitCode)
	}

	result["success"] = true
	return result, nil
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

// ═══════════════════════════════════════════════════════════════════════════════
// FILE OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════════

// readFile reads the contents of a file with size limiting.
func (e *CortexToolExecutor) readFile(path string) (any, error) {
	e.log.Debug("Reading file: %s", path)

	// Resolve and validate path
	absPath, err := e.resolvePath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	// Check file exists and get info
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("cannot access file: %w", err)
	}

	// Check it's a file
	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory, not a file: %s", path)
	}

	// Check file size
	if info.Size() > MaxFileSize {
		return nil, fmt.Errorf("file too large (%d bytes, max %d)", info.Size(), MaxFileSize)
	}

	// Read file
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return map[string]any{
		"path":    absPath,
		"content": string(content),
		"size":    info.Size(),
	}, nil
}

// writeFile writes content to a file.
func (e *CortexToolExecutor) writeFile(path, content string) (any, error) {
	e.log.Debug("Writing file: %s (%d bytes)", path, len(content))

	// Resolve and validate path
	absPath, err := e.resolvePath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	// Ensure parent directory exists
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	return map[string]any{
		"path":    absPath,
		"written": len(content),
		"success": true,
	}, nil
}

// listDirectory lists the contents of a directory.
func (e *CortexToolExecutor) listDirectory(path string) (any, error) {
	e.log.Debug("Listing directory: %s", path)

	// Resolve and validate path
	absPath, err := e.resolvePath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	// Check directory exists
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("directory not found: %s", path)
		}
		return nil, fmt.Errorf("cannot access directory: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", path)
	}

	// Read directory entries
	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	// Build result
	files := make([]map[string]any, 0, len(entries))
	for i, entry := range entries {
		if i >= MaxDirEntries {
			break
		}

		entryInfo, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, map[string]any{
			"name":     entry.Name(),
			"is_dir":   entry.IsDir(),
			"size":     entryInfo.Size(),
			"mode":     entryInfo.Mode().String(),
			"mod_time": entryInfo.ModTime().Format(time.RFC3339),
		})
	}

	return map[string]any{
		"path":      absPath,
		"entries":   files,
		"count":     len(files),
		"truncated": len(entries) > MaxDirEntries,
	}, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// SEARCH OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════════

// glob finds files matching a pattern.
func (e *CortexToolExecutor) glob(pattern string) (any, error) {
	e.log.Debug("Glob pattern: %s", pattern)

	// If pattern is not absolute, make it relative to workDir
	if !filepath.IsAbs(pattern) {
		pattern = filepath.Join(e.workDir, pattern)
	}

	// Validate the pattern doesn't escape workDir
	if !e.isPathSafe(pattern) {
		return nil, fmt.Errorf("pattern escapes working directory")
	}

	// Find matches
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid glob pattern: %w", err)
	}

	// Filter to ensure all matches are within workDir
	safeMatches := make([]string, 0, len(matches))
	for _, match := range matches {
		if e.isPathSafe(match) {
			// Convert to relative paths for cleaner output
			relPath, err := filepath.Rel(e.workDir, match)
			if err == nil {
				safeMatches = append(safeMatches, relPath)
			} else {
				safeMatches = append(safeMatches, match)
			}
		}
	}

	return map[string]any{
		"pattern": pattern,
		"matches": safeMatches,
		"count":   len(safeMatches),
	}, nil
}

// grep searches file contents using ripgrep or grep.
func (e *CortexToolExecutor) grep(ctx context.Context, pattern, path string) (any, error) {
	e.log.Debug("Grep: pattern=%s path=%s", pattern, path)

	// Resolve and validate path
	absPath, err := e.resolvePath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	// Create context with timeout
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultCommandTimeout)
		defer cancel()
	}

	// Try ripgrep first, fall back to grep
	var cmd *exec.Cmd
	if rgPath, err := exec.LookPath("rg"); err == nil {
		cmd = exec.CommandContext(ctx, rgPath, "-n", "--no-heading", "-m", "100", pattern, absPath)
	} else if grepPath, err := exec.LookPath("grep"); err == nil {
		cmd = exec.CommandContext(ctx, grepPath, "-rn", "-m", "100", pattern, absPath)
	} else {
		return nil, fmt.Errorf("neither ripgrep nor grep available")
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	// grep returns exit code 1 when no matches found (not an error)
	output := stdout.String()
	if err != nil && output == "" {
		// Check if it's just "no matches" (exit code 1)
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return map[string]any{
				"pattern": pattern,
				"path":    absPath,
				"matches": []string{},
				"count":   0,
			}, nil
		}
		return nil, fmt.Errorf("grep failed: %w", err)
	}

	// Parse matches
	lines := strings.Split(strings.TrimSpace(output), "\n")
	matches := make([]string, 0, len(lines))
	for _, line := range lines {
		if line != "" {
			matches = append(matches, line)
		}
	}

	// Limit results
	truncated := false
	if len(matches) > 100 {
		matches = matches[:100]
		truncated = true
	}

	return map[string]any{
		"pattern":   pattern,
		"path":      absPath,
		"matches":   matches,
		"count":     len(matches),
		"truncated": truncated,
	}, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// PATH SECURITY
// ═══════════════════════════════════════════════════════════════════════════════

// resolvePath resolves a path and validates it's within the working directory.
func (e *CortexToolExecutor) resolvePath(path string) (string, error) {
	// Handle empty path
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	// Make absolute if relative
	var absPath string
	if filepath.IsAbs(path) {
		absPath = path
	} else {
		absPath = filepath.Join(e.workDir, path)
	}

	// Clean the path (resolve . and ..)
	absPath = filepath.Clean(absPath)

	// Validate the path doesn't escape workDir
	if !e.isPathSafe(absPath) {
		return "", fmt.Errorf("path escapes working directory: %s", path)
	}

	return absPath, nil
}

// isPathSafe checks if a path is within the working directory.
func (e *CortexToolExecutor) isPathSafe(path string) bool {
	// Clean and absolutize the path
	absPath := filepath.Clean(path)
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(e.workDir, absPath)
		absPath = filepath.Clean(absPath)
	}

	// Check if path starts with workDir
	// We add a separator to prevent /tmp matching /tmpfoo
	workDirWithSep := e.workDir
	if !strings.HasSuffix(workDirWithSep, string(filepath.Separator)) {
		workDirWithSep += string(filepath.Separator)
	}

	// Path is safe if it equals workDir or is under it
	return absPath == e.workDir || strings.HasPrefix(absPath, workDirWithSep)
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

// getStringArg extracts a string argument from args map.
func getStringArg(args map[string]any, key string) (string, error) {
	val, ok := args[key]
	if !ok {
		return "", fmt.Errorf("missing required argument: %s", key)
	}

	str, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("argument %s must be a string", key)
	}

	return str, nil
}

// getStringArgDefault extracts a string argument with a default value.
func getStringArgDefault(args map[string]any, key, defaultVal string) string {
	val, ok := args[key]
	if !ok {
		return defaultVal
	}

	str, ok := val.(string)
	if !ok {
		return defaultVal
	}

	return str
}

// combineOutput combines stdout and stderr.
func combineOutput(stdout, stderr string) string {
	stdout = strings.TrimSpace(stdout)
	stderr = strings.TrimSpace(stderr)

	if stderr != "" && stdout != "" {
		return stderr + "\n" + stdout
	}
	if stderr != "" {
		return stderr
	}
	return stdout
}

// truncateOutput truncates output to maxSize.
func truncateOutput(output string, maxSize int) string {
	if len(output) > maxSize {
		return output[:maxSize] + "\n... [output truncated]"
	}
	return output
}
