package tools

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"
)

// Executor implements ToolExecutor with security policies.
type Executor struct {
	mu      sync.RWMutex
	tools   map[ToolType]Tool
	policy  *SecurityPolicy
	confirm ConfirmationHandler

	// Compiled blocked patterns for performance
	blockedPatterns []*regexp.Regexp

	// Statistics
	stats ExecutorStats
}

// ExecutorStats tracks tool execution metrics.
type ExecutorStats struct {
	TotalExecutions  int64
	SuccessCount     int64
	FailureCount     int64
	BlockedCount     int64
	ConfirmationReqs int64
	TotalDuration    time.Duration

	mu sync.Mutex
}

// ExecutorOption configures the Executor.
type ExecutorOption func(*Executor)

// WithPolicy sets a custom security policy.
func WithPolicy(policy *SecurityPolicy) ExecutorOption {
	return func(e *Executor) {
		e.policy = policy
		e.compilePatterns()
	}
}

// WithConfirmationHandler sets the confirmation callback.
func WithConfirmationHandler(handler ConfirmationHandler) ExecutorOption {
	return func(e *Executor) {
		e.confirm = handler
	}
}

// NewExecutor creates a new tool executor.
func NewExecutor(opts ...ExecutorOption) *Executor {
	e := &Executor{
		tools:  make(map[ToolType]Tool),
		policy: DefaultSecurityPolicy(),
	}

	// Compile default patterns
	e.compilePatterns()

	// Apply options
	for _, opt := range opts {
		opt(e)
	}

	return e
}

// compilePatterns pre-compiles blocked patterns for performance.
func (e *Executor) compilePatterns() {
	e.blockedPatterns = nil
	for _, pattern := range e.policy.BlockedPatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			e.blockedPatterns = append(e.blockedPatterns, re)
		}
	}
}

// Register adds a tool to the executor.
func (e *Executor) Register(tool Tool) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	name := tool.Name()
	if _, exists := e.tools[name]; exists {
		return fmt.Errorf("tool %s already registered", name)
	}

	e.tools[name] = tool
	return nil
}

// GetTool returns a registered tool by name.
func (e *Executor) GetTool(name ToolType) (Tool, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	tool, ok := e.tools[name]
	return tool, ok
}

// SetPolicy updates the security policy.
func (e *Executor) SetPolicy(policy *SecurityPolicy) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.policy = policy
	e.compilePatterns()
}

// GetPolicy returns the current security policy.
func (e *Executor) GetPolicy() *SecurityPolicy {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.policy
}

// Execute runs a tool request through the security layer.
func (e *Executor) Execute(ctx context.Context, req *ToolRequest) (*ToolResult, error) {
	start := time.Now()

	// Get the tool
	tool, ok := e.GetTool(req.Tool)
	if !ok {
		return &ToolResult{
			Tool:      req.Tool,
			Success:   false,
			Error:     fmt.Sprintf("unknown tool: %s", req.Tool),
			Duration:  time.Since(start),
			RiskLevel: RiskNone,
		}, fmt.Errorf("unknown tool: %s", req.Tool)
	}

	// Validate request
	if err := tool.Validate(req); err != nil {
		return &ToolResult{
			Tool:      req.Tool,
			Success:   false,
			Error:     fmt.Sprintf("validation failed: %v", err),
			Duration:  time.Since(start),
			RiskLevel: RiskNone,
		}, err
	}

	// Security pre-flight check
	if blocked, reason := e.isBlocked(req); blocked {
		e.stats.mu.Lock()
		e.stats.BlockedCount++
		e.stats.mu.Unlock()

		return &ToolResult{
			Tool:      req.Tool,
			Success:   false,
			Error:     fmt.Sprintf("blocked by security policy: %s", reason),
			Duration:  time.Since(start),
			RiskLevel: RiskCritical,
		}, fmt.Errorf("blocked: %s", reason)
	}

	// Assess risk
	risk := tool.AssessRisk(req)

	// Check if confirmation is required
	if risk >= e.policy.RequireConfirmationAbove && e.confirm != nil && !req.RequireConfirmation {
		e.stats.mu.Lock()
		e.stats.ConfirmationReqs++
		e.stats.mu.Unlock()

		approved, err := e.confirm(req, risk, fmt.Sprintf("risk level: %s", risk.String()))
		if err != nil {
			return &ToolResult{
				Tool:      req.Tool,
				Success:   false,
				Error:     fmt.Sprintf("confirmation failed: %v", err),
				Duration:  time.Since(start),
				RiskLevel: risk,
			}, err
		}
		if !approved {
			return &ToolResult{
				Tool:      req.Tool,
				Success:   false,
				Error:     "operation cancelled by user",
				Duration:  time.Since(start),
				RiskLevel: risk,
			}, fmt.Errorf("cancelled by user")
		}
	}

	// Handle dry run
	if req.DryRun {
		return &ToolResult{
			Tool:      req.Tool,
			Success:   true,
			Output:    "[DRY RUN] Would execute: " + req.Input,
			Duration:  time.Since(start),
			RiskLevel: risk,
			Metadata: map[string]interface{}{
				"dry_run": true,
			},
		}, nil
	}

	// Apply timeout
	timeout := req.Timeout
	if timeout == 0 {
		timeout = e.policy.MaxTimeout
	}
	if timeout > e.policy.MaxTimeout {
		timeout = e.policy.MaxTimeout
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute the tool
	e.stats.mu.Lock()
	e.stats.TotalExecutions++
	e.stats.mu.Unlock()

	result, err := tool.Execute(execCtx, req)

	// Update stats
	e.stats.mu.Lock()
	e.stats.TotalDuration += time.Since(start)
	if result != nil && result.Success {
		e.stats.SuccessCount++
	} else {
		e.stats.FailureCount++
	}
	e.stats.mu.Unlock()

	// Ensure result has risk level
	if result != nil {
		result.RiskLevel = risk
		result.Duration = time.Since(start)
	}

	return result, err
}

// isBlocked checks if the request matches blocked patterns.
func (e *Executor) isBlocked(req *ToolRequest) (bool, string) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Check blocked commands (exact match)
	for _, cmd := range e.policy.BlockedCommands {
		if req.Input == cmd {
			return true, fmt.Sprintf("exact match: %s", cmd)
		}
	}

	// Check blocked patterns (regex)
	for _, pattern := range e.blockedPatterns {
		if pattern.MatchString(req.Input) {
			return true, fmt.Sprintf("pattern match: %s", pattern.String())
		}
	}

	// Check sudo restriction
	if !e.policy.AllowSudo {
		sudoPattern := regexp.MustCompile(`^\s*sudo\s+`)
		if sudoPattern.MatchString(req.Input) {
			return true, "sudo not allowed"
		}
	}

	return false, ""
}

// Stats returns execution statistics.
func (e *Executor) Stats() ExecutorStats {
	e.stats.mu.Lock()
	defer e.stats.mu.Unlock()

	return ExecutorStats{
		TotalExecutions:  e.stats.TotalExecutions,
		SuccessCount:     e.stats.SuccessCount,
		FailureCount:     e.stats.FailureCount,
		BlockedCount:     e.stats.BlockedCount,
		ConfirmationReqs: e.stats.ConfirmationReqs,
		TotalDuration:    e.stats.TotalDuration,
	}
}

// SuccessRate returns the success rate as a percentage.
func (s *ExecutorStats) SuccessRate() float64 {
	if s.TotalExecutions == 0 {
		return 0
	}
	return float64(s.SuccessCount) / float64(s.TotalExecutions) * 100
}

// AvgDuration returns the average execution duration.
func (s *ExecutorStats) AvgDuration() time.Duration {
	if s.TotalExecutions == 0 {
		return 0
	}
	return time.Duration(int64(s.TotalDuration) / s.TotalExecutions)
}
