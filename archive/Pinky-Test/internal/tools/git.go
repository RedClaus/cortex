// Package tools provides the tool execution framework for Pinky
package tools

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Git operation names
const (
	GitOpStatus   = "status"
	GitOpAdd      = "add"
	GitOpCommit   = "commit"
	GitOpPush     = "push"
	GitOpPull     = "pull"
	GitOpClone    = "clone"
	GitOpBranch   = "branch"
	GitOpCheckout = "checkout"
	GitOpDiff     = "diff"
	GitOpLog      = "log"
	GitOpPR       = "pr"
)

// GitConfig holds configuration for the git tool
type GitConfig struct {
	AllowPush     bool   // Whether push operations are allowed
	AllowForce    bool   // Whether force operations are allowed
	DefaultBranch string // Default branch name (main/master)
}

// DefaultGitConfig returns sensible defaults
func DefaultGitConfig() *GitConfig {
	return &GitConfig{
		AllowPush:     true,
		AllowForce:    false,
		DefaultBranch: "main",
	}
}

// GitTool implements the Tool interface for git operations
type GitTool struct {
	config *GitConfig
}

// NewGitTool creates a new git tool with the given configuration
func NewGitTool(config *GitConfig) *GitTool {
	if config == nil {
		config = DefaultGitConfig()
	}
	return &GitTool{config: config}
}

// Name returns the tool identifier
func (g *GitTool) Name() string {
	return "git"
}

// Description returns a human-readable description
func (g *GitTool) Description() string {
	return "Git version control operations: status, add, commit, push, pull, clone, branch, checkout, diff, log, pr"
}

// Category returns the tool category
func (g *GitTool) Category() ToolCategory {
	return CategoryGit
}

// RiskLevel returns the base risk level for git operations
// Individual operations may have higher risk levels
func (g *GitTool) RiskLevel() RiskLevel {
	return RiskMedium
}

// OperationRiskLevel returns the risk level for a specific git operation
func (g *GitTool) OperationRiskLevel(operation string, args map[string]any) RiskLevel {
	switch operation {
	case GitOpStatus, GitOpDiff, GitOpLog:
		// Read-only operations are low risk
		return RiskLow
	case GitOpAdd, GitOpBranch, GitOpCheckout:
		// Local-only modifications are medium risk
		return RiskMedium
	case GitOpCommit:
		// Commits are medium risk (local only)
		return RiskMedium
	case GitOpPush:
		// Push can be high risk if force is involved
		if force, ok := args["force"].(bool); ok && force {
			return RiskHigh
		}
		return RiskMedium
	case GitOpPull, GitOpClone:
		// Network operations that modify local state
		return RiskMedium
	case GitOpPR:
		// Creating PRs involves network but is generally safe
		return RiskMedium
	default:
		return RiskMedium
	}
}

// Common validation errors
var (
	ErrMissingOperation   = errors.New("operation is required")
	ErrInvalidOperation   = errors.New("invalid git operation")
	ErrMissingFiles       = errors.New("files argument is required for add operation")
	ErrMissingMessage     = errors.New("message argument is required for commit operation")
	ErrMissingRepoURL     = errors.New("url argument is required for clone operation")
	ErrMissingBranchName  = errors.New("name argument is required for branch operation")
	ErrPushNotAllowed     = errors.New("push operations are disabled")
	ErrForceNotAllowed    = errors.New("force operations are disabled")
	ErrNotGitRepository   = errors.New("not a git repository")
)

// Validate checks if the input is valid for execution
func (g *GitTool) Validate(input *ToolInput) error {
	if input == nil {
		return errors.New("input is nil")
	}

	// Support both input.Command and input.Args["operation"] for flexibility
	operation := input.Command
	if operation == "" {
		if op, ok := input.Args["operation"].(string); ok {
			operation = op
		}
	}

	if operation == "" {
		return ErrMissingOperation
	}

	switch operation {
	case GitOpStatus, GitOpDiff, GitOpLog, GitOpPull:
		// No required arguments
		return nil

	case GitOpAdd:
		if _, ok := input.Args["files"]; !ok {
			return ErrMissingFiles
		}
		return nil

	case GitOpCommit:
		if _, ok := input.Args["message"]; !ok {
			return ErrMissingMessage
		}
		return nil

	case GitOpPush:
		if !g.config.AllowPush {
			return ErrPushNotAllowed
		}
		if force, ok := input.Args["force"].(bool); ok && force && !g.config.AllowForce {
			return ErrForceNotAllowed
		}
		return nil

	case GitOpClone:
		if _, ok := input.Args["url"]; !ok {
			return ErrMissingRepoURL
		}
		return nil

	case GitOpBranch:
		// Branch without arguments lists branches (valid)
		return nil

	case GitOpCheckout:
		if _, ok := input.Args["ref"]; !ok {
			return ErrMissingBranchName
		}
		return nil

	case GitOpPR:
		// PR helper - validation depends on sub-operation
		return nil

	default:
		return fmt.Errorf("%w: %s", ErrInvalidOperation, operation)
	}
}

// getOperation extracts the operation from input (checks both Command and Args["operation"])
func (g *GitTool) getOperation(input *ToolInput) string {
	if input == nil {
		return ""
	}
	if input.Command != "" {
		return input.Command
	}
	if op, ok := input.Args["operation"].(string); ok {
		return op
	}
	return ""
}

// Execute runs the git operation
func (g *GitTool) Execute(ctx context.Context, input *ToolInput) (*ToolOutput, error) {
	if err := g.Validate(input); err != nil {
		return &ToolOutput{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	workingDir := input.WorkingDir
	if workingDir == "" {
		workingDir = "."
	}

	var output *ToolOutput
	var err error

	operation := g.getOperation(input)
	switch operation {
	case GitOpStatus:
		output, err = g.executeStatus(ctx, workingDir)
	case GitOpAdd:
		output, err = g.executeAdd(ctx, workingDir, input.Args)
	case GitOpCommit:
		output, err = g.executeCommit(ctx, workingDir, input.Args)
	case GitOpPush:
		output, err = g.executePush(ctx, workingDir, input.Args)
	case GitOpPull:
		output, err = g.executePull(ctx, workingDir, input.Args)
	case GitOpClone:
		output, err = g.executeClone(ctx, input.Args)
	case GitOpBranch:
		output, err = g.executeBranch(ctx, workingDir, input.Args)
	case GitOpCheckout:
		output, err = g.executeCheckout(ctx, workingDir, input.Args)
	case GitOpDiff:
		output, err = g.executeDiff(ctx, workingDir, input.Args)
	case GitOpLog:
		output, err = g.executeLog(ctx, workingDir, input.Args)
	case GitOpPR:
		output, err = g.executePR(ctx, workingDir, input.Args)
	default:
		return &ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("unknown operation: %s", operation),
		}, ErrInvalidOperation
	}

	return output, err
}

// executeStatus runs git status
func (g *GitTool) executeStatus(ctx context.Context, workingDir string) (*ToolOutput, error) {
	return g.runGitCommand(ctx, workingDir, "status", "--porcelain=v2", "--branch")
}

// executeAdd runs git add with the specified files
func (g *GitTool) executeAdd(ctx context.Context, workingDir string, args map[string]any) (*ToolOutput, error) {
	files := g.extractFiles(args["files"])
	if len(files) == 0 {
		return &ToolOutput{
			Success: false,
			Error:   "no files specified",
		}, ErrMissingFiles
	}

	cmdArgs := []string{"add"}
	cmdArgs = append(cmdArgs, files...)

	return g.runGitCommand(ctx, workingDir, cmdArgs...)
}

// executeCommit runs git commit
func (g *GitTool) executeCommit(ctx context.Context, workingDir string, args map[string]any) (*ToolOutput, error) {
	message, _ := args["message"].(string)
	if message == "" {
		return &ToolOutput{
			Success: false,
			Error:   "commit message is required",
		}, ErrMissingMessage
	}

	// SECURITY: Sanitize commit message to prevent command injection
	message = SanitizeCommitMessage(message)

	cmdArgs := []string{"commit", "-m", message}

	// Optional: amend
	if amend, ok := args["amend"].(bool); ok && amend {
		cmdArgs = append(cmdArgs, "--amend")
	}

	// Optional: allow empty
	if allowEmpty, ok := args["allow_empty"].(bool); ok && allowEmpty {
		cmdArgs = append(cmdArgs, "--allow-empty")
	}

	return g.runGitCommand(ctx, workingDir, cmdArgs...)
}

// executePush runs git push
func (g *GitTool) executePush(ctx context.Context, workingDir string, args map[string]any) (*ToolOutput, error) {
	cmdArgs := []string{"push"}

	// Optional: remote
	if remote, ok := args["remote"].(string); ok && remote != "" {
		cmdArgs = append(cmdArgs, remote)
	}

	// Optional: branch
	if branch, ok := args["branch"].(string); ok && branch != "" {
		cmdArgs = append(cmdArgs, branch)
	}

	// Optional: set-upstream
	if setUpstream, ok := args["set_upstream"].(bool); ok && setUpstream {
		cmdArgs = append(cmdArgs, "-u")
	}

	// Optional: force (requires explicit enable in config)
	if force, ok := args["force"].(bool); ok && force {
		if !g.config.AllowForce {
			return &ToolOutput{
				Success: false,
				Error:   "force push is disabled",
			}, ErrForceNotAllowed
		}
		cmdArgs = append(cmdArgs, "--force")
	}

	return g.runGitCommand(ctx, workingDir, cmdArgs...)
}

// executePull runs git pull
func (g *GitTool) executePull(ctx context.Context, workingDir string, args map[string]any) (*ToolOutput, error) {
	cmdArgs := []string{"pull"}

	// Optional: remote
	if remote, ok := args["remote"].(string); ok && remote != "" {
		cmdArgs = append(cmdArgs, remote)
	}

	// Optional: branch
	if branch, ok := args["branch"].(string); ok && branch != "" {
		cmdArgs = append(cmdArgs, branch)
	}

	// Optional: rebase
	if rebase, ok := args["rebase"].(bool); ok && rebase {
		cmdArgs = append(cmdArgs, "--rebase")
	}

	return g.runGitCommand(ctx, workingDir, cmdArgs...)
}

// executeClone runs git clone
func (g *GitTool) executeClone(ctx context.Context, args map[string]any) (*ToolOutput, error) {
	url, _ := args["url"].(string)
	if url == "" {
		return &ToolOutput{
			Success: false,
			Error:   "repository URL is required",
		}, ErrMissingRepoURL
	}

	cmdArgs := []string{"clone", url}

	// Optional: destination directory
	if dest, ok := args["destination"].(string); ok && dest != "" {
		cmdArgs = append(cmdArgs, dest)
	}

	// Optional: depth for shallow clone
	if depth, ok := args["depth"].(int); ok && depth > 0 {
		cmdArgs = append(cmdArgs, "--depth", fmt.Sprintf("%d", depth))
	}

	// Optional: branch to clone
	if branch, ok := args["branch"].(string); ok && branch != "" {
		cmdArgs = append(cmdArgs, "-b", branch)
	}

	return g.runGitCommand(ctx, ".", cmdArgs...)
}

// executeBranch runs git branch operations
func (g *GitTool) executeBranch(ctx context.Context, workingDir string, args map[string]any) (*ToolOutput, error) {
	cmdArgs := []string{"branch"}

	// If name provided, create new branch
	if name, ok := args["name"].(string); ok && name != "" {
		// Check for delete operation
		if del, ok := args["delete"].(bool); ok && del {
			cmdArgs = append(cmdArgs, "-d", name)
		} else {
			cmdArgs = append(cmdArgs, name)
			// Optional: start point
			if startPoint, ok := args["start_point"].(string); ok && startPoint != "" {
				cmdArgs = append(cmdArgs, startPoint)
			}
		}
	} else {
		// List branches
		if all, ok := args["all"].(bool); ok && all {
			cmdArgs = append(cmdArgs, "-a")
		}
		if remote, ok := args["remote"].(bool); ok && remote {
			cmdArgs = append(cmdArgs, "-r")
		}
	}

	return g.runGitCommand(ctx, workingDir, cmdArgs...)
}

// executeCheckout runs git checkout
func (g *GitTool) executeCheckout(ctx context.Context, workingDir string, args map[string]any) (*ToolOutput, error) {
	ref, _ := args["ref"].(string)
	if ref == "" {
		return &ToolOutput{
			Success: false,
			Error:   "branch or ref is required",
		}, ErrMissingBranchName
	}

	cmdArgs := []string{"checkout", ref}

	// Optional: create new branch
	if create, ok := args["create"].(bool); ok && create {
		cmdArgs = []string{"checkout", "-b", ref}
	}

	return g.runGitCommand(ctx, workingDir, cmdArgs...)
}

// executeDiff runs git diff
func (g *GitTool) executeDiff(ctx context.Context, workingDir string, args map[string]any) (*ToolOutput, error) {
	cmdArgs := []string{"diff"}

	// Optional: staged changes
	if staged, ok := args["staged"].(bool); ok && staged {
		cmdArgs = append(cmdArgs, "--staged")
	}

	// Optional: specific commit/ref
	if ref, ok := args["ref"].(string); ok && ref != "" {
		cmdArgs = append(cmdArgs, ref)
	}

	// Optional: specific files
	if files := g.extractFiles(args["files"]); len(files) > 0 {
		cmdArgs = append(cmdArgs, "--")
		cmdArgs = append(cmdArgs, files...)
	}

	// Optional: stat only
	if stat, ok := args["stat"].(bool); ok && stat {
		cmdArgs = append(cmdArgs, "--stat")
	}

	return g.runGitCommand(ctx, workingDir, cmdArgs...)
}

// executeLog runs git log
func (g *GitTool) executeLog(ctx context.Context, workingDir string, args map[string]any) (*ToolOutput, error) {
	cmdArgs := []string{"log"}

	// Optional: limit
	limit := 10 // default
	if n, ok := args["limit"].(int); ok && n > 0 {
		limit = n
	}
	cmdArgs = append(cmdArgs, fmt.Sprintf("-n%d", limit))

	// Optional: oneline format
	if oneline, ok := args["oneline"].(bool); ok && oneline {
		cmdArgs = append(cmdArgs, "--oneline")
	} else {
		cmdArgs = append(cmdArgs, "--format=%H|%an|%ae|%s|%ci")
	}

	// Optional: specific ref
	if ref, ok := args["ref"].(string); ok && ref != "" {
		cmdArgs = append(cmdArgs, ref)
	}

	// Optional: specific files
	if files := g.extractFiles(args["files"]); len(files) > 0 {
		cmdArgs = append(cmdArgs, "--")
		cmdArgs = append(cmdArgs, files...)
	}

	return g.runGitCommand(ctx, workingDir, cmdArgs...)
}

// executePR handles pull request operations (using gh CLI)
func (g *GitTool) executePR(ctx context.Context, workingDir string, args map[string]any) (*ToolOutput, error) {
	subOp, _ := args["action"].(string)
	if subOp == "" {
		subOp = "status" // default to showing PR status
	}

	switch subOp {
	case "status":
		return g.runCommand(ctx, workingDir, "gh", "pr", "status")
	case "list":
		cmdArgs := []string{"pr", "list"}
		if state, ok := args["state"].(string); ok && state != "" {
			cmdArgs = append(cmdArgs, "--state", state)
		}
		return g.runCommand(ctx, workingDir, "gh", cmdArgs...)
	case "view":
		prNum, _ := args["number"].(string)
		if prNum == "" {
			return g.runCommand(ctx, workingDir, "gh", "pr", "view")
		}
		return g.runCommand(ctx, workingDir, "gh", "pr", "view", prNum)
	case "create":
		cmdArgs := []string{"pr", "create"}
		if title, ok := args["title"].(string); ok && title != "" {
			cmdArgs = append(cmdArgs, "--title", title)
		}
		if body, ok := args["body"].(string); ok && body != "" {
			cmdArgs = append(cmdArgs, "--body", body)
		}
		if draft, ok := args["draft"].(bool); ok && draft {
			cmdArgs = append(cmdArgs, "--draft")
		}
		return g.runCommand(ctx, workingDir, "gh", cmdArgs...)
	case "checkout":
		prNum, _ := args["number"].(string)
		if prNum == "" {
			return &ToolOutput{
				Success: false,
				Error:   "PR number is required for checkout",
			}, errors.New("pr number required")
		}
		return g.runCommand(ctx, workingDir, "gh", "pr", "checkout", prNum)
	default:
		return &ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("unknown PR action: %s", subOp),
		}, errors.New("unknown pr action")
	}
}

// runGitCommand executes a git command
func (g *GitTool) runGitCommand(ctx context.Context, workingDir string, args ...string) (*ToolOutput, error) {
	return g.runCommand(ctx, workingDir, "git", args...)
}

// runCommand executes a command with the given arguments
func (g *GitTool) runCommand(ctx context.Context, workingDir string, name string, args ...string) (*ToolOutput, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = workingDir

	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))

	if err != nil {
		// Check for common git errors
		if isNotGitRepo(outputStr) {
			return &ToolOutput{
				Success: false,
				Output:  outputStr,
				Error:   "not a git repository",
			}, ErrNotGitRepository
		}

		return &ToolOutput{
			Success: false,
			Output:  outputStr,
			Error:   err.Error(),
		}, err
	}

	return &ToolOutput{
		Success: true,
		Output:  outputStr,
	}, nil
}

// extractFiles converts various file input formats to a slice of strings
func (g *GitTool) extractFiles(filesArg any) []string {
	switch f := filesArg.(type) {
	case string:
		// Single file or space-separated list
		return strings.Fields(f)
	case []string:
		return f
	case []any:
		var files []string
		for _, item := range f {
			if s, ok := item.(string); ok {
				files = append(files, s)
			}
		}
		return files
	default:
		return nil
	}
}

// isNotGitRepo checks if the error indicates not a git repository
func isNotGitRepo(output string) bool {
	patterns := []string{
		"not a git repository",
		"fatal: not a git repository",
	}
	lower := strings.ToLower(output)
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// GetCurrentBranch returns the current git branch
func (g *GitTool) GetCurrentBranch(ctx context.Context, workingDir string) (string, error) {
	output, err := g.runGitCommand(ctx, workingDir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output.Output), nil
}

// IsClean returns true if the working directory has no uncommitted changes
func (g *GitTool) IsClean(ctx context.Context, workingDir string) (bool, error) {
	output, err := g.runGitCommand(ctx, workingDir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output.Output) == "", nil
}

// GetRemoteURL returns the URL of the specified remote
func (g *GitTool) GetRemoteURL(ctx context.Context, workingDir, remote string) (string, error) {
	if remote == "" {
		remote = "origin"
	}
	output, err := g.runGitCommand(ctx, workingDir, "remote", "get-url", remote)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output.Output), nil
}

// SanitizeCommitMessage removes potentially dangerous characters from commit messages
func SanitizeCommitMessage(msg string) string {
	// Remove null bytes and other control characters
	re := regexp.MustCompile(`[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]`)
	return re.ReplaceAllString(msg, "")
}

// ParseGitStatus parses git status --porcelain=v2 output into structured data
type GitStatusEntry struct {
	Status     string // M, A, D, R, C, U, ?
	Path       string
	OrigPath   string // For renames
	Staged     bool
	Worktree   bool
}

func ParseGitStatus(output string) []GitStatusEntry {
	var entries []GitStatusEntry
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Porcelain v2 format parsing
		if strings.HasPrefix(line, "1 ") || strings.HasPrefix(line, "2 ") {
			parts := strings.Fields(line)
			if len(parts) >= 9 {
				xy := parts[1] // XY status
				path := parts[8]
				entries = append(entries, GitStatusEntry{
					Status:   xy,
					Path:     path,
					Staged:   xy[0] != '.',
					Worktree: xy[1] != '.',
				})
			}
		} else if strings.HasPrefix(line, "? ") {
			// Untracked file
			path := strings.TrimPrefix(line, "? ")
			entries = append(entries, GitStatusEntry{
				Status:   "?",
				Path:     path,
				Worktree: true,
			})
		}
	}
	return entries
}

// AbsolutePath resolves a path relative to the git working directory
func (g *GitTool) AbsolutePath(workingDir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(workingDir, path)
}
