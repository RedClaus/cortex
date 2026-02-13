package decomposer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/normanking/cortex/internal/cognitive"
	"github.com/normanking/cortex/internal/logging"
)

// ═══════════════════════════════════════════════════════════════════════════════
// WORKFLOW TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// Workflow represents a multi-step execution plan.
type Workflow struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description,omitempty"`
	Steps         []Step                 `json:"steps"`
	Context       map[string]interface{} `json:"context"`         // Shared state between steps
	CreatedAt     time.Time              `json:"created_at"`
	EstimatedTime string                 `json:"estimated_time,omitempty"`
}

// WorkflowState tracks the execution state of a workflow.
type WorkflowState string

const (
	WorkflowStatePending   WorkflowState = "pending"
	WorkflowStateRunning   WorkflowState = "running"
	WorkflowStateCompleted WorkflowState = "completed"
	WorkflowStateFailed    WorkflowState = "failed"
	WorkflowStatePaused    WorkflowState = "paused"
	WorkflowStateCancelled WorkflowState = "cancelled"
)

// WorkflowExecution tracks a running workflow.
type WorkflowExecution struct {
	WorkflowID    string                 `json:"workflow_id"`
	State         WorkflowState          `json:"state"`
	CurrentStep   int                    `json:"current_step"`
	StepResults   []StepResult           `json:"step_results"`
	Context       map[string]interface{} `json:"context"`
	StartedAt     time.Time              `json:"started_at"`
	CompletedAt   *time.Time             `json:"completed_at,omitempty"`
	Error         string                 `json:"error,omitempty"`
	TotalDuration time.Duration          `json:"total_duration"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// WORKFLOW BUILDER
// ═══════════════════════════════════════════════════════════════════════════════

// WorkflowBuilder provides a fluent API for building workflows.
type WorkflowBuilder struct {
	workflow *Workflow
}

// NewWorkflow creates a new workflow builder.
func NewWorkflow(name string) *WorkflowBuilder {
	return &WorkflowBuilder{
		workflow: &Workflow{
			ID:        uuid.New().String(),
			Name:      name,
			Steps:     make([]Step, 0),
			Context:   make(map[string]interface{}),
			CreatedAt: time.Now(),
		},
	}
}

// WithDescription sets the workflow description.
func (wb *WorkflowBuilder) WithDescription(desc string) *WorkflowBuilder {
	wb.workflow.Description = desc
	return wb
}

// WithEstimatedTime sets the estimated execution time.
func (wb *WorkflowBuilder) WithEstimatedTime(duration string) *WorkflowBuilder {
	wb.workflow.EstimatedTime = duration
	return wb
}

// WithContext adds context variables to the workflow.
func (wb *WorkflowBuilder) WithContext(ctx map[string]interface{}) *WorkflowBuilder {
	for k, v := range ctx {
		wb.workflow.Context[k] = v
	}
	return wb
}

// AddStep adds a step to the workflow.
func (wb *WorkflowBuilder) AddStep(step Step) *WorkflowBuilder {
	// Auto-generate ID if not provided
	if step.ID == "" {
		step.ID = fmt.Sprintf("step%d", len(wb.workflow.Steps)+1)
	}
	wb.workflow.Steps = append(wb.workflow.Steps, step)
	return wb
}

// AddLLMStep adds an LLM query step.
func (wb *WorkflowBuilder) AddLLMStep(description, prompt string, deps ...string) *WorkflowBuilder {
	return wb.AddStep(Step{
		Description: description,
		Type:        StepLLM,
		Prompt:      prompt,
		RiskLevel:   "low",
		DependsOn:   deps,
	})
}

// AddToolStep adds a tool execution step.
func (wb *WorkflowBuilder) AddToolStep(description, tool string, riskLevel string, deps ...string) *WorkflowBuilder {
	return wb.AddStep(Step{
		Description: description,
		Type:        StepTool,
		Tool:        tool,
		RiskLevel:   riskLevel,
		DependsOn:   deps,
	})
}

// AddApprovalStep adds a user approval step.
func (wb *WorkflowBuilder) AddApprovalStep(description string, deps ...string) *WorkflowBuilder {
	return wb.AddStep(Step{
		Description: description,
		Type:        StepApproval,
		RiskLevel:   "high",
		DependsOn:   deps,
	})
}

// Build returns the constructed workflow.
func (wb *WorkflowBuilder) Build() *Workflow {
	return wb.workflow
}

// ═══════════════════════════════════════════════════════════════════════════════
// ENHANCED WORKFLOW EXECUTOR
// ═══════════════════════════════════════════════════════════════════════════════

// ToolExecutor defines the interface for tool execution.
type ToolExecutor interface {
	ExecuteTool(ctx context.Context, toolName string, variables map[string]interface{}) (string, error)
}

// TemplateExecutor defines the interface for template execution.
type TemplateExecutor interface {
	ExecuteTemplate(ctx context.Context, templateID string, variables map[string]interface{}) (string, error)
}

// ApprovalHandler defines the interface for user approval.
type ApprovalHandler interface {
	RequestApproval(ctx context.Context, step *Step) (bool, error)
}

// EnhancedWorkflowExecutor provides advanced workflow execution with dependency resolution.
type EnhancedWorkflowExecutor struct {
	llm         cognitive.SimpleChatProvider
	toolExec    ToolExecutor
	templateExec TemplateExecutor
	approvalHandler ApprovalHandler
	log         *logging.Logger
}

// ExecutorConfig configures the workflow executor.
type ExecutorConfig struct {
	LLM         cognitive.SimpleChatProvider
	ToolExec    ToolExecutor
	TemplateExec TemplateExecutor
	ApprovalHandler ApprovalHandler
}

// NewEnhancedExecutor creates a new enhanced workflow executor.
func NewEnhancedExecutor(cfg ExecutorConfig) *EnhancedWorkflowExecutor {
	return &EnhancedWorkflowExecutor{
		llm:         cfg.LLM,
		toolExec:    cfg.ToolExec,
		templateExec: cfg.TemplateExec,
		approvalHandler: cfg.ApprovalHandler,
		log:         logging.Global(),
	}
}

// Execute runs a workflow with full dependency resolution and state management.
func (e *EnhancedWorkflowExecutor) Execute(ctx context.Context, workflow *Workflow, callback StepCallback) (*WorkflowResult, error) {
	e.log.Info("[Workflow] Starting execution: %s (%d steps)", workflow.Name, len(workflow.Steps))
	start := time.Now()

	result := &WorkflowResult{
		Success:     true,
		StepResults: make([]StepResult, 0, len(workflow.Steps)),
		TotalSteps:  len(workflow.Steps),
	}

	// Initialize execution context with workflow context
	execContext := make(map[string]interface{})
	for k, v := range workflow.Context {
		execContext[k] = v
	}

	// Track completed steps for dependency resolution
	completed := make(map[string]*StepResult)
	stepOutputs := make(map[string]string)

	for i, step := range workflow.Steps {
		e.log.Debug("[Workflow] Processing step %d/%d: %s", i+1, len(workflow.Steps), step.ID)

		// Check context cancellation
		select {
		case <-ctx.Done():
			result.Success = false
			return result, fmt.Errorf("workflow cancelled: %w", ctx.Err())
		default:
		}

		// Check dependencies
		depsOK, missingDeps := e.checkDependencies(step, completed)

		stepResult := StepResult{
			StepID:    step.ID,
			Timestamp: time.Now(),
		}

		if !depsOK {
			stepResult.Skipped = true
			stepResult.Error = fmt.Sprintf("missing dependencies: %v", missingDeps)
			result.SkippedSteps++
			e.log.Warn("[Workflow] Step %s skipped: missing dependencies %v", step.ID, missingDeps)
		} else {
			// Execute the step
			stepStart := time.Now()

			// Prepare step variables with context
			stepVars := make(map[string]interface{})
			for k, v := range step.Variables {
				stepVars[k] = v
			}
			// Add outputs from dependent steps
			for _, depID := range step.DependsOn {
				if output, ok := stepOutputs[depID]; ok {
					stepVars[depID+"_output"] = output
				}
			}
			// Add global context
			for k, v := range execContext {
				if _, exists := stepVars[k]; !exists {
					stepVars[k] = v
				}
			}

			output, err := e.executeStep(ctx, &step, stepVars)
			stepResult.Duration = time.Since(stepStart)

			if err != nil {
				stepResult.Success = false
				stepResult.Error = err.Error()
				result.FailedSteps++
				result.Success = false
				e.log.Error("[Workflow] Step %s failed: %v", step.ID, err)
			} else {
				stepResult.Success = true
				stepResult.Output = output
				result.CompletedSteps++
				completed[step.ID] = &stepResult
				stepOutputs[step.ID] = output

				// Update execution context
				execContext[step.ID+"_output"] = output

				e.log.Info("[Workflow] Step %s completed in %v", step.ID, stepResult.Duration)
			}
		}

		result.StepResults = append(result.StepResults, stepResult)

		// Call the callback
		if callback != nil {
			callback(&step, &stepResult)
		}

		// Stop on failure for non-optional steps
		if !stepResult.Success && !step.Optional {
			e.log.Error("[Workflow] Workflow failed on required step: %s", step.ID)
			break
		}
	}

	result.TotalDuration = time.Since(start)

	// Set final output from last successful step
	for i := len(result.StepResults) - 1; i >= 0; i-- {
		if result.StepResults[i].Success && result.StepResults[i].Output != "" {
			result.FinalOutput = result.StepResults[i].Output
			break
		}
	}

	e.log.Info("[Workflow] Execution completed: %d/%d steps successful in %v",
		result.CompletedSteps, result.TotalSteps, result.TotalDuration)

	return result, nil
}

// checkDependencies verifies all dependencies are satisfied.
func (e *EnhancedWorkflowExecutor) checkDependencies(step Step, completed map[string]*StepResult) (bool, []string) {
	missing := make([]string, 0)

	for _, depID := range step.DependsOn {
		result, ok := completed[depID]
		if !ok || !result.Success {
			missing = append(missing, depID)
		}
	}

	return len(missing) == 0, missing
}

// executeStep runs a single workflow step.
func (e *EnhancedWorkflowExecutor) executeStep(ctx context.Context, step *Step, variables map[string]interface{}) (string, error) {
	e.log.Debug("[Workflow] Executing step %s: %s (type: %s)", step.ID, step.Description, step.Type)

	switch step.Type {
	case StepLLM:
		return e.executeLLMStep(ctx, step, variables)

	case StepTool:
		return e.executeToolStep(ctx, step, variables)

	case StepTemplate:
		return e.executeTemplateStep(ctx, step, variables)

	case StepApproval:
		return e.executeApprovalStep(ctx, step)

	default:
		return "", fmt.Errorf("unknown step type: %s", step.Type)
	}
}

// executeLLMStep executes an LLM query step.
func (e *EnhancedWorkflowExecutor) executeLLMStep(ctx context.Context, step *Step, variables map[string]interface{}) (string, error) {
	if e.llm == nil {
		return "", fmt.Errorf("LLM provider not configured")
	}

	// Substitute variables in prompt
	prompt := step.Prompt
	for key, value := range variables {
		placeholder := fmt.Sprintf("{{%s}}", key)
		prompt = strings.ReplaceAll(prompt, placeholder, fmt.Sprint(value))
	}

	messages := []cognitive.ChatMessage{
		{Role: "user", Content: prompt},
	}

	response, err := e.llm.Chat(ctx, messages, "")
	if err != nil {
		return "", fmt.Errorf("LLM chat failed: %w", err)
	}

	return response, nil
}

// executeToolStep executes a tool step.
func (e *EnhancedWorkflowExecutor) executeToolStep(ctx context.Context, step *Step, variables map[string]interface{}) (string, error) {
	if e.toolExec == nil {
		return fmt.Sprintf("[Tool %s would execute here]", step.Tool), nil
	}

	output, err := e.toolExec.ExecuteTool(ctx, step.Tool, variables)
	if err != nil {
		return "", fmt.Errorf("tool %s failed: %w", step.Tool, err)
	}

	return output, nil
}

// executeTemplateStep executes a template step.
func (e *EnhancedWorkflowExecutor) executeTemplateStep(ctx context.Context, step *Step, variables map[string]interface{}) (string, error) {
	if e.templateExec == nil {
		return fmt.Sprintf("[Template %s would execute here]", step.TemplateID), nil
	}

	output, err := e.templateExec.ExecuteTemplate(ctx, step.TemplateID, variables)
	if err != nil {
		return "", fmt.Errorf("template %s failed: %w", step.TemplateID, err)
	}

	return output, nil
}

// executeApprovalStep requests user approval.
func (e *EnhancedWorkflowExecutor) executeApprovalStep(ctx context.Context, step *Step) (string, error) {
	if e.approvalHandler == nil {
		// Auto-approve if no handler configured
		e.log.Warn("[Workflow] No approval handler configured, auto-approving step %s", step.ID)
		return "Auto-approved (no handler)", nil
	}

	approved, err := e.approvalHandler.RequestApproval(ctx, step)
	if err != nil {
		return "", fmt.Errorf("approval request failed: %w", err)
	}

	if !approved {
		return "", fmt.Errorf("user rejected approval")
	}

	return "Approved", nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// WORKFLOW DAG VALIDATION
// ═══════════════════════════════════════════════════════════════════════════════

// ValidateWorkflow checks for circular dependencies and invalid references.
func ValidateWorkflow(workflow *Workflow) error {
	// Build step ID set
	stepIDs := make(map[string]bool)
	for _, step := range workflow.Steps {
		if step.ID == "" {
			return fmt.Errorf("step has empty ID")
		}
		if stepIDs[step.ID] {
			return fmt.Errorf("duplicate step ID: %s", step.ID)
		}
		stepIDs[step.ID] = true
	}

	// Validate dependencies
	for _, step := range workflow.Steps {
		for _, depID := range step.DependsOn {
			if !stepIDs[depID] {
				return fmt.Errorf("step %s references non-existent dependency: %s", step.ID, depID)
			}
		}
	}

	// Check for circular dependencies
	if err := checkCircularDeps(workflow.Steps); err != nil {
		return err
	}

	return nil
}

// checkCircularDeps detects circular dependencies using DFS.
func checkCircularDeps(steps []Step) error {
	// Build adjacency list
	graph := make(map[string][]string)
	for _, step := range steps {
		graph[step.ID] = step.DependsOn
	}

	// Track visited nodes
	visited := make(map[string]bool)
	recursionStack := make(map[string]bool)

	var dfs func(string) error
	dfs = func(nodeID string) error {
		visited[nodeID] = true
		recursionStack[nodeID] = true

		for _, depID := range graph[nodeID] {
			if !visited[depID] {
				if err := dfs(depID); err != nil {
					return err
				}
			} else if recursionStack[depID] {
				return fmt.Errorf("circular dependency detected: %s -> %s", nodeID, depID)
			}
		}

		recursionStack[nodeID] = false
		return nil
	}

	for _, step := range steps {
		if !visited[step.ID] {
			if err := dfs(step.ID); err != nil {
				return err
			}
		}
	}

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// WORKFLOW SERIALIZATION
// ═══════════════════════════════════════════════════════════════════════════════

// ToJSON serializes a workflow to JSON.
func (w *Workflow) ToJSON() ([]byte, error) {
	return json.MarshalIndent(w, "", "  ")
}

// FromJSON deserializes a workflow from JSON.
func FromJSON(data []byte) (*Workflow, error) {
	var workflow Workflow
	if err := json.Unmarshal(data, &workflow); err != nil {
		return nil, fmt.Errorf("failed to parse workflow JSON: %w", err)
	}

	// Validate the workflow
	if err := ValidateWorkflow(&workflow); err != nil {
		return nil, fmt.Errorf("invalid workflow: %w", err)
	}

	return &workflow, nil
}
