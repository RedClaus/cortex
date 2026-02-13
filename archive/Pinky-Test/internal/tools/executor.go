// Package tools provides the tool execution framework for Pinky
package tools

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Common errors for tool execution
var (
	ErrToolNotFound      = errors.New("tool not found")
	ErrApprovalRequired  = errors.New("tool execution requires approval")
	ErrApprovalDenied    = errors.New("tool execution was denied")
	ErrExecutionTimeout  = errors.New("tool execution timed out")
	ErrValidationFailed  = errors.New("tool input validation failed")
	ErrExecutorShutdown  = errors.New("executor is shutting down")
)

// PermissionChecker is the interface for permission checking
type PermissionChecker interface {
	NeedsApproval(userID, tool, command string, riskLevel string) bool
}

// ApprovalHandler handles requesting and receiving user approval
type ApprovalHandler interface {
	RequestApproval(ctx context.Context, req *ApprovalRequest) (*ApprovalResponse, error)
}

// ApprovalRequest represents a pending approval request
type ApprovalRequest struct {
	ID         string
	UserID     string
	Tool       string
	Command    string
	Args       map[string]any
	WorkingDir string
	RiskLevel  RiskLevel
	Reason     string
}

// ApprovalResponse is the user's decision
type ApprovalResponse struct {
	Approved    bool
	AlwaysAllow bool
	AllowDir    bool
	Modified    string
}

// ExecutorConfig configures the tool executor
type ExecutorConfig struct {
	DefaultTimeout time.Duration // Default execution timeout
	MaxConcurrent  int           // Maximum concurrent executions
	MaxOutputSize  int           // Maximum output size in bytes
}

// DefaultExecutorConfig returns sensible defaults
func DefaultExecutorConfig() *ExecutorConfig {
	return &ExecutorConfig{
		DefaultTimeout: 2 * time.Minute,
		MaxConcurrent:  10,
		MaxOutputSize:  1024 * 1024, // 1MB
	}
}

// Executor coordinates tool execution with permission checks
type Executor struct {
	registry    *Registry
	permissions PermissionChecker
	approval    ApprovalHandler
	config      *ExecutorConfig

	// Concurrency control
	semaphore chan struct{}

	// Shutdown handling
	mu       sync.RWMutex
	shutdown bool
	wg       sync.WaitGroup

	// Execution tracking
	execMu     sync.Mutex
	executions map[string]*Execution
}

// Execution tracks a running tool execution
type Execution struct {
	ID        string
	Tool      string
	Input     *ToolInput
	StartTime time.Time
	Cancel    context.CancelFunc
}

// ExecuteRequest is the input to Execute
type ExecuteRequest struct {
	Tool       string
	Input      *ToolInput
	Timeout    time.Duration // Override default timeout
	SkipApproval bool        // Skip permission check (internal use only)
	Reason     string        // Why the agent wants to run this
}

// ExecuteResult is the output from Execute
type ExecuteResult struct {
	Output      *ToolOutput
	Tool        string
	ExecutionID string
	Approved    bool
	ApprovedBy  string
}

// NewExecutor creates a new tool executor
func NewExecutor(registry *Registry, permissions PermissionChecker, approval ApprovalHandler, config *ExecutorConfig) *Executor {
	if config == nil {
		config = DefaultExecutorConfig()
	}

	return &Executor{
		registry:    registry,
		permissions: permissions,
		approval:    approval,
		config:      config,
		semaphore:   make(chan struct{}, config.MaxConcurrent),
		executions:  make(map[string]*Execution),
	}
}

// Execute runs a tool with permission checks and timeout handling
func (e *Executor) Execute(ctx context.Context, req *ExecuteRequest) (*ExecuteResult, error) {
	// Check if shutting down
	e.mu.RLock()
	if e.shutdown {
		e.mu.RUnlock()
		return nil, ErrExecutorShutdown
	}
	e.mu.RUnlock()

	// Get the tool
	tool, ok := e.registry.Get(req.Tool)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrToolNotFound, req.Tool)
	}

	// Validate input
	if err := tool.Validate(req.Input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidationFailed, err)
	}

	// Check permissions (unless explicitly skipped)
	if !req.SkipApproval && e.permissions != nil {
		needsApproval := e.permissions.NeedsApproval(
			req.Input.UserID,
			req.Tool,
			req.Input.Command,
			string(tool.RiskLevel()),
		)

		if needsApproval {
			if e.approval == nil {
				return nil, ErrApprovalRequired
			}

			// Request approval
			approvalReq := &ApprovalRequest{
				ID:         generateID(),
				UserID:     req.Input.UserID,
				Tool:       req.Tool,
				Command:    req.Input.Command,
				Args:       req.Input.Args,
				WorkingDir: req.Input.WorkingDir,
				RiskLevel:  tool.RiskLevel(),
				Reason:     req.Reason,
			}

			resp, err := e.approval.RequestApproval(ctx, approvalReq)
			if err != nil {
				return nil, fmt.Errorf("approval request failed: %w", err)
			}

			if !resp.Approved {
				return nil, ErrApprovalDenied
			}

			// Apply modifications if user edited the command
			if resp.Modified != "" {
				req.Input.Command = resp.Modified
			}
		}
	}

	// Acquire semaphore slot
	select {
	case e.semaphore <- struct{}{}:
		defer func() { <-e.semaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Track execution
	e.wg.Add(1)
	defer e.wg.Done()

	// Set up timeout
	timeout := req.Timeout
	if timeout == 0 {
		timeout = e.config.DefaultTimeout
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	execID := generateID()
	exec := &Execution{
		ID:        execID,
		Tool:      req.Tool,
		Input:     req.Input,
		StartTime: time.Now(),
		Cancel:    cancel,
	}

	e.execMu.Lock()
	e.executions[execID] = exec
	e.execMu.Unlock()

	defer func() {
		e.execMu.Lock()
		delete(e.executions, execID)
		e.execMu.Unlock()
	}()

	// Execute the tool
	start := time.Now()
	output, err := tool.Execute(execCtx, req.Input)
	duration := time.Since(start)

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrExecutionTimeout
		}
		return nil, err
	}

	// Update duration in output
	if output != nil {
		output.Duration = duration

		// Truncate output if too large
		if len(output.Output) > e.config.MaxOutputSize {
			output.Output = output.Output[:e.config.MaxOutputSize] + "\n... (output truncated)"
		}
	}

	return &ExecuteResult{
		Output:      output,
		Tool:        req.Tool,
		ExecutionID: execID,
		Approved:    true,
	}, nil
}

// CancelExecution cancels a running execution by ID
func (e *Executor) CancelExecution(execID string) bool {
	e.execMu.Lock()
	defer e.execMu.Unlock()

	if exec, ok := e.executions[execID]; ok {
		exec.Cancel()
		return true
	}
	return false
}

// ListExecutions returns currently running executions
func (e *Executor) ListExecutions() []*Execution {
	e.execMu.Lock()
	defer e.execMu.Unlock()

	execs := make([]*Execution, 0, len(e.executions))
	for _, exec := range e.executions {
		execs = append(execs, exec)
	}
	return execs
}

// Shutdown gracefully shuts down the executor
func (e *Executor) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	e.shutdown = true
	e.mu.Unlock()

	// Cancel all running executions
	e.execMu.Lock()
	for _, exec := range e.executions {
		exec.Cancel()
	}
	e.execMu.Unlock()

	// Wait for all executions to complete
	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// generateID creates a simple unique ID
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
