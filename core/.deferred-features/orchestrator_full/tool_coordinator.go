// Package orchestrator provides the central coordination layer for Cortex.
package orchestrator

import (
	"context"
	"fmt"
	"sync"

	"github.com/normanking/cortex/internal/fingerprint"
	"github.com/normanking/cortex/internal/logging"
	"github.com/normanking/cortex/internal/planning/tasks"
	"github.com/normanking/cortex/internal/tools"
)

// ═══════════════════════════════════════════════════════════════════════════════
// TOOL EXECUTOR INTERFACE
// CR-017: Phase 4 - Tool Coordinator Extraction
// ═══════════════════════════════════════════════════════════════════════════════

// ToolDefinition describes a tool's schema for LLM consumption.
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// ToolExecutor defines the interface for tool execution operations.
// It encapsulates tool registration, execution, and validation into a
// single coherent subsystem.
type ToolExecutor interface {
	// Execute runs a tool request through the security layer.
	Execute(ctx context.Context, req *tools.ToolRequest) (*tools.ToolResult, error)

	// ListTools returns all registered tool definitions.
	ListTools() []ToolDefinition

	// GetTool returns a registered tool by name.
	GetTool(name string) (tools.Tool, bool)

	// Register adds a tool to the executor.
	Register(tool tools.Tool)

	// ValidateArgs validates arguments for a specific tool.
	ValidateArgs(toolName string, args map[string]any) error

	// Stats returns tool coordinator statistics.
	Stats() *ToolStats
}

// ToolStats contains statistics about the tool subsystem.
type ToolStats struct {
	ToolCount        int  `json:"tool_count"`
	HasFingerprinter bool `json:"has_fingerprinter"`
	HasTaskManager   bool `json:"has_task_manager"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// TOOL COORDINATOR IMPLEMENTATION
// ═══════════════════════════════════════════════════════════════════════════════

// ToolCoordinator manages all tool-related operations.
// It encapsulates the tool executor, fingerprinter, and task manager into a
// single coherent subsystem.
type ToolCoordinator struct {
	// Tool components
	executor      *tools.Executor
	fingerprinter *fingerprint.Fingerprinter
	taskManager   *tasks.Manager

	// Configuration
	config *ToolCoordinatorConfig
	log    *logging.Logger

	// State
	mu sync.RWMutex
}

// ToolCoordinatorConfig configures the ToolCoordinator.
type ToolCoordinatorConfig struct {
	// Executor is the underlying tool executor.
	Executor *tools.Executor

	// Fingerprinter provides project/platform detection.
	Fingerprinter *fingerprint.Fingerprinter

	// TaskManager provides task management capabilities (optional).
	TaskManager *tasks.Manager

	// SecurityPolicy overrides the default security policy.
	SecurityPolicy *tools.SecurityPolicy
}

// NewToolCoordinator creates a new tool coordinator.
func NewToolCoordinator(cfg *ToolCoordinatorConfig) *ToolCoordinator {
	if cfg == nil {
		cfg = &ToolCoordinatorConfig{}
	}

	tc := &ToolCoordinator{
		executor:      cfg.Executor,
		fingerprinter: cfg.Fingerprinter,
		taskManager:   cfg.TaskManager,
		config:        cfg,
		log:           logging.Global(),
	}

	// Create default executor if not provided
	if tc.executor == nil {
		tc.executor = tools.NewExecutor()
	}

	// Apply security policy if provided
	if cfg.SecurityPolicy != nil && tc.executor != nil {
		tc.executor.SetPolicy(cfg.SecurityPolicy)
	}

	// Create default fingerprinter if not provided
	if tc.fingerprinter == nil {
		tc.fingerprinter = fingerprint.NewFingerprinter()
	}

	return tc
}

// Verify ToolCoordinator implements ToolExecutor at compile time.
var _ ToolExecutor = (*ToolCoordinator)(nil)

// ═══════════════════════════════════════════════════════════════════════════════
// TOOL EXECUTION
// ═══════════════════════════════════════════════════════════════════════════════

// Execute runs a tool request through the security layer.
func (tc *ToolCoordinator) Execute(ctx context.Context, req *tools.ToolRequest) (*tools.ToolResult, error) {
	if tc.executor == nil {
		return nil, fmt.Errorf("executor not configured")
	}

	tc.log.Debug("[ToolCoordinator] Executing tool: %s", req.Tool)

	result, err := tc.executor.Execute(ctx, req)
	if err != nil {
		tc.log.Debug("[ToolCoordinator] Tool execution failed: %v", err)
		return result, err
	}

	tc.log.Debug("[ToolCoordinator] Tool execution completed: success=%v, duration=%v", result.Success, result.Duration)
	return result, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// TOOL REGISTRATION
// ═══════════════════════════════════════════════════════════════════════════════

// Register adds a tool to the executor.
func (tc *ToolCoordinator) Register(tool tools.Tool) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if tc.executor == nil {
		tc.log.Warn("[ToolCoordinator] Cannot register tool: executor not configured")
		return
	}

	if err := tc.executor.Register(tool); err != nil {
		tc.log.Warn("[ToolCoordinator] Failed to register tool %s: %v", tool.Name(), err)
		return
	}

	tc.log.Debug("[ToolCoordinator] Registered tool: %s", tool.Name())
}

// GetTool returns a registered tool by name.
func (tc *ToolCoordinator) GetTool(name string) (tools.Tool, bool) {
	if tc.executor == nil {
		return nil, false
	}

	return tc.executor.GetTool(tools.ToolType(name))
}

// ListTools returns all registered tool definitions.
func (tc *ToolCoordinator) ListTools() []ToolDefinition {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	if tc.executor == nil {
		return nil
	}

	// Get all registered tools from executor
	// We need to iterate through known tool types since executor doesn't expose a list method
	knownTools := []tools.ToolType{
		tools.ToolBash,
		tools.ToolRead,
		tools.ToolWrite,
		tools.ToolEdit,
		tools.ToolGlob,
		tools.ToolGrep,
		tools.ToolWebSearch,
	}

	definitions := make([]ToolDefinition, 0)
	for _, toolType := range knownTools {
		if tool, ok := tc.executor.GetTool(toolType); ok {
			definitions = append(definitions, ToolDefinition{
				Name:        string(tool.Name()),
				Description: tc.getToolDescription(tool.Name()),
			})
		}
	}

	return definitions
}

// getToolDescription returns a description for a tool type.
func (tc *ToolCoordinator) getToolDescription(toolType tools.ToolType) string {
	descriptions := map[tools.ToolType]string{
		tools.ToolBash:      "Execute shell commands with security pre-flight checks",
		tools.ToolRead:      "Read file contents from the filesystem",
		tools.ToolWrite:     "Write content to a file",
		tools.ToolEdit:      "Edit file contents with search and replace",
		tools.ToolGlob:      "Find files matching glob patterns",
		tools.ToolGrep:      "Search file contents using regular expressions",
		tools.ToolWebSearch: "Search the web for information",
	}

	if desc, ok := descriptions[toolType]; ok {
		return desc
	}
	return "Tool: " + string(toolType)
}

// ═══════════════════════════════════════════════════════════════════════════════
// ARGUMENT VALIDATION
// ═══════════════════════════════════════════════════════════════════════════════

// ValidateArgs validates arguments for a specific tool.
func (tc *ToolCoordinator) ValidateArgs(toolName string, args map[string]any) error {
	tool, ok := tc.GetTool(toolName)
	if !ok {
		return fmt.Errorf("unknown tool: %s", toolName)
	}

	// Build a ToolRequest from args for validation
	req := &tools.ToolRequest{
		Tool:   tools.ToolType(toolName),
		Params: args,
	}

	// Extract input from args if present
	if input, ok := args["input"].(string); ok {
		req.Input = input
	}
	if input, ok := args["command"].(string); ok {
		req.Input = input
	}
	if input, ok := args["path"].(string); ok {
		req.Input = input
	}

	// Use the tool's Validate method
	return tool.Validate(req)
}

// ═══════════════════════════════════════════════════════════════════════════════
// FINGERPRINTING
// ═══════════════════════════════════════════════════════════════════════════════

// DetectProject analyzes the current directory for project type.
func (tc *ToolCoordinator) DetectProject(ctx context.Context, dir string) (*fingerprint.Fingerprint, error) {
	if tc.fingerprinter == nil {
		return nil, fmt.Errorf("fingerprinter not configured")
	}

	return tc.fingerprinter.DetectProject(ctx, dir)
}

// Fingerprint returns the underlying fingerprinter.
func (tc *ToolCoordinator) Fingerprint() *fingerprint.Fingerprinter {
	return tc.fingerprinter
}

// ═══════════════════════════════════════════════════════════════════════════════
// TASK MANAGEMENT
// ═══════════════════════════════════════════════════════════════════════════════

// TaskManager returns the underlying task manager (may be nil).
func (tc *ToolCoordinator) TaskManager() *tasks.Manager {
	return tc.taskManager
}

// ═══════════════════════════════════════════════════════════════════════════════
// SECURITY POLICY
// ═══════════════════════════════════════════════════════════════════════════════

// SetSecurityPolicy updates the executor's security policy.
func (tc *ToolCoordinator) SetSecurityPolicy(policy *tools.SecurityPolicy) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if tc.executor == nil {
		tc.log.Warn("[ToolCoordinator] Cannot set security policy: executor not configured")
		return
	}

	tc.executor.SetPolicy(policy)
	tc.log.Info("[ToolCoordinator] Security policy updated")
}

// GetSecurityPolicy returns the current security policy.
func (tc *ToolCoordinator) GetSecurityPolicy() *tools.SecurityPolicy {
	if tc.executor == nil {
		return nil
	}

	return tc.executor.GetPolicy()
}

// ═══════════════════════════════════════════════════════════════════════════════
// STATISTICS
// ═══════════════════════════════════════════════════════════════════════════════

// Stats returns tool coordinator statistics.
func (tc *ToolCoordinator) Stats() *ToolStats {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	stats := &ToolStats{
		HasFingerprinter: tc.fingerprinter != nil,
		HasTaskManager:   tc.taskManager != nil,
	}

	if tc.executor != nil {
		// Count registered tools
		knownTools := []tools.ToolType{
			tools.ToolBash,
			tools.ToolRead,
			tools.ToolWrite,
			tools.ToolEdit,
			tools.ToolGlob,
			tools.ToolGrep,
			tools.ToolWebSearch,
		}

		for _, toolType := range knownTools {
			if _, ok := tc.executor.GetTool(toolType); ok {
				stats.ToolCount++
			}
		}
	}

	return stats
}

// ExecutorStats returns the underlying executor's statistics.
func (tc *ToolCoordinator) ExecutorStats() tools.ExecutorStats {
	if tc.executor == nil {
		return tools.ExecutorStats{}
	}

	return tc.executor.Stats()
}

// ═══════════════════════════════════════════════════════════════════════════════
// COMPONENT ACCESS (for legacy compatibility)
// ═══════════════════════════════════════════════════════════════════════════════

// Executor returns the underlying tool executor.
func (tc *ToolCoordinator) Executor() *tools.Executor {
	return tc.executor
}
