// Package tools provides the tool execution layer for Cortex.
// This implements the "bash, read, write, edit" tools from the Opencode module
// with security pre-flight checks and sandboxing support.
package tools

import (
	"context"
	"time"
)

// ToolType identifies the kind of tool being executed.
type ToolType string

const (
	ToolBash      ToolType = "bash"
	ToolRead      ToolType = "read"
	ToolWrite     ToolType = "write"
	ToolEdit      ToolType = "edit"
	ToolGlob      ToolType = "glob"
	ToolGrep      ToolType = "grep"
	ToolWebSearch ToolType = "web_search"
)

// RiskLevel indicates how dangerous a tool invocation is.
type RiskLevel int

const (
	RiskNone     RiskLevel = iota // Safe operations (read, list)
	RiskLow                       // Low risk (local file write)
	RiskMedium                    // Medium risk (network calls, process start)
	RiskHigh                      // High risk (system modification, sudo)
	RiskCritical                  // Critical risk (destructive, rm -rf, etc.)
)

// String returns a human-readable risk level.
func (r RiskLevel) String() string {
	switch r {
	case RiskNone:
		return "none"
	case RiskLow:
		return "low"
	case RiskMedium:
		return "medium"
	case RiskHigh:
		return "high"
	case RiskCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// Tool defines the interface for all executable tools.
type Tool interface {
	// Name returns the tool identifier.
	Name() ToolType

	// Execute runs the tool with the given request.
	Execute(ctx context.Context, req *ToolRequest) (*ToolResult, error)

	// Validate checks if the request is valid before execution.
	Validate(req *ToolRequest) error

	// AssessRisk evaluates the risk level of a request.
	AssessRisk(req *ToolRequest) RiskLevel
}

// ToolRequest represents a tool invocation request.
type ToolRequest struct {
	// Tool specifies which tool to invoke.
	Tool ToolType `json:"tool"`

	// Input is the primary input (command for bash, path for read, etc.).
	Input string `json:"input"`

	// Params contains tool-specific parameters.
	Params map[string]interface{} `json:"params,omitempty"`

	// WorkingDir is the directory context for the operation.
	WorkingDir string `json:"working_dir,omitempty"`

	// Timeout overrides the default timeout.
	Timeout time.Duration `json:"timeout,omitempty"`

	// DryRun if true, validates but doesn't execute.
	DryRun bool `json:"dry_run,omitempty"`

	// RequireConfirmation if true, needs user approval.
	RequireConfirmation bool `json:"require_confirmation,omitempty"`
}

// ToolResult represents the outcome of a tool execution.
type ToolResult struct {
	// Tool that was executed.
	Tool ToolType `json:"tool"`

	// Success indicates if the tool completed successfully.
	Success bool `json:"success"`

	// Output contains the tool's output (stdout for bash, content for read, etc.).
	Output string `json:"output,omitempty"`

	// Error contains error details if Success is false.
	Error string `json:"error,omitempty"`

	// ExitCode for bash commands.
	ExitCode int `json:"exit_code,omitempty"`

	// Duration of the execution.
	Duration time.Duration `json:"duration"`

	// RiskLevel that was assessed.
	RiskLevel RiskLevel `json:"risk_level"`

	// Metadata contains tool-specific metadata.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// SecurityPolicy defines what operations are allowed.
type SecurityPolicy struct {
	// AllowedDirs restricts file operations to these directories.
	AllowedDirs []string `json:"allowed_dirs,omitempty"`

	// BlockedCommands are commands that cannot be executed.
	BlockedCommands []string `json:"blocked_commands,omitempty"`

	// BlockedPatterns are regex patterns for blocked operations.
	BlockedPatterns []string `json:"blocked_patterns,omitempty"`

	// MaxTimeout limits execution time.
	MaxTimeout time.Duration `json:"max_timeout,omitempty"`

	// RequireConfirmationAbove requires user approval for risk >= this level.
	RequireConfirmationAbove RiskLevel `json:"require_confirmation_above"`

	// AllowNetwork permits network-related commands.
	AllowNetwork bool `json:"allow_network"`

	// AllowSudo permits sudo commands.
	AllowSudo bool `json:"allow_sudo"`

	// Sandboxed if true, runs in restricted environment.
	Sandboxed bool `json:"sandboxed"`
}

// DefaultSecurityPolicy returns a reasonable default policy.
func DefaultSecurityPolicy() *SecurityPolicy {
	return &SecurityPolicy{
		BlockedCommands: []string{
			"rm -rf /",
			"rm -rf /*",
			":(){ :|:& };:", // Fork bomb
			"mkfs",
			"dd if=/dev/zero",
			"chmod -R 777 /",
		},
		BlockedPatterns: []string{
			`rm\s+-rf?\s+/($|\s)`,     // rm -rf /
			`>\s*/dev/sd[a-z]`,        // Write to block devices
			`curl.*\|\s*(ba)?sh`,      // Pipe curl to shell
			`wget.*\|\s*(ba)?sh`,      // Pipe wget to shell
			`:()\{.*\}.*;`,            // Fork bomb variants
			`/etc/passwd`,             // Sensitive files
			`/etc/shadow`,             // Sensitive files
			`\.ssh/(id_|authorized)`,  // SSH keys
			`\$\(.*\).*\|\s*(ba)?sh`,  // Command substitution to shell
		},
		MaxTimeout:               5 * time.Minute,
		RequireConfirmationAbove: RiskMedium,
		AllowNetwork:             true,
		AllowSudo:                false,
		Sandboxed:                false,
	}
}

// ToolExecutor manages tool execution with security policies.
type ToolExecutor interface {
	// Register adds a tool to the executor.
	Register(tool Tool) error

	// Execute runs a tool request through the security layer.
	Execute(ctx context.Context, req *ToolRequest) (*ToolResult, error)

	// GetTool returns a registered tool by name.
	GetTool(name ToolType) (Tool, bool)

	// SetPolicy updates the security policy.
	SetPolicy(policy *SecurityPolicy)

	// GetPolicy returns the current security policy.
	GetPolicy() *SecurityPolicy
}

// ConfirmationHandler is called when user confirmation is needed.
type ConfirmationHandler func(req *ToolRequest, risk RiskLevel, reason string) (bool, error)

// OutputHandler receives streaming output from long-running tools.
type OutputHandler func(output string, isStderr bool)
