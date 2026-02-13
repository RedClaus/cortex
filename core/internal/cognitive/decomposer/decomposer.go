package decomposer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/cognitive"
	"github.com/normanking/cortex/internal/logging"
)

// ═══════════════════════════════════════════════════════════════════════════════
// TASK DECOMPOSER
// ═══════════════════════════════════════════════════════════════════════════════

// StepType categorizes the type of workflow step.
type StepType string

const (
	StepTool     StepType = "tool"     // Execute a tool
	StepTemplate StepType = "template" // Execute via template engine
	StepLLM      StepType = "llm"      // Query LLM for guidance
	StepApproval StepType = "approval" // Require user approval
)

// Step represents a single step in a decomposed task.
type Step struct {
	ID          string                 `json:"id"`
	Description string                 `json:"description"`
	Type        StepType               `json:"type"`
	Tool        string                 `json:"tool,omitempty"`        // For StepTool
	TemplateID  string                 `json:"template_id,omitempty"` // For StepTemplate
	Prompt      string                 `json:"prompt,omitempty"`      // For StepLLM
	Variables   map[string]interface{} `json:"variables,omitempty"`
	DependsOn   []string               `json:"depends_on,omitempty"` // Step IDs this depends on
	Optional    bool                   `json:"optional"`
	RiskLevel   string                 `json:"risk_level"` // "low", "medium", "high"
}

// DecompositionResult contains the breakdown of a complex task.
type DecompositionResult struct {
	OriginalInput string            `json:"original_input"`
	Complexity    *ComplexityResult `json:"complexity"`
	Steps         []Step            `json:"steps"`
	EstimatedTime string            `json:"estimated_time,omitempty"`
	RequiresApproval bool           `json:"requires_approval"`
}

// Decomposer breaks complex tasks into manageable steps.
type Decomposer struct {
	llm    cognitive.SimpleChatProvider
	scorer *Scorer
	log    *logging.Logger
}

// NewDecomposer creates a new task decomposer.
func NewDecomposer(llm cognitive.SimpleChatProvider) *Decomposer {
	return &Decomposer{
		llm:    llm,
		scorer: NewScorer(),
		log:    logging.Global(),
	}
}

// DecompositionSystemPrompt is the prompt for task decomposition.
const DecompositionSystemPrompt = `You are a task decomposition assistant. Your job is to break complex tasks into simple, executable steps.

When given a task, analyze it and break it down into a sequence of steps. Each step should be:
1. Atomic - does one thing
2. Clear - easily understood
3. Executable - can be performed with available tools

For each step, specify:
- description: What this step does
- type: "tool" (use a tool), "template" (use a template), "llm" (ask AI), or "approval" (get user confirmation)
- tool: If type is "tool", which tool to use (read, write, edit, bash, glob, grep)
- risk_level: "low", "medium", or "high"

Output your analysis as JSON:
{
  "steps": [
    {
      "id": "step1",
      "description": "What this step does",
      "type": "tool",
      "tool": "read",
      "risk_level": "low",
      "depends_on": []
    },
    {
      "id": "step2",
      "description": "Another step",
      "type": "tool",
      "tool": "edit",
      "risk_level": "medium",
      "depends_on": ["step1"]
    }
  ],
  "estimated_time": "5-10 minutes",
  "requires_approval": false
}

Guidelines:
- High-risk operations (delete, deploy, execute) should have approval steps before them
- Group related operations when possible
- Consider rollback strategies for destructive operations
- Keep the number of steps reasonable (3-10 typically)`

// Analyze evaluates a task and determines if decomposition is needed.
func (d *Decomposer) Analyze(input string, taskType cognitive.TaskType) *DecompositionResult {
	complexity := d.scorer.Score(input, taskType)

	result := &DecompositionResult{
		OriginalInput: input,
		Complexity:    complexity,
		Steps:         make([]Step, 0),
	}

	// Only decompose if complexity warrants it
	if !complexity.NeedsDecomp {
		// Return single step for simple tasks
		result.Steps = append(result.Steps, Step{
			ID:          "single",
			Description: "Execute the request directly",
			Type:        StepLLM,
			Prompt:      input,
			RiskLevel:   "low",
		})
		return result
	}

	return result
}

// Decompose breaks a complex task into executable steps using LLM.
func (d *Decomposer) Decompose(ctx context.Context, input string, taskType cognitive.TaskType) (*DecompositionResult, error) {
	// First analyze complexity
	result := d.Analyze(input, taskType)

	// If simple, no need to call LLM
	if !result.Complexity.NeedsDecomp {
		d.log.Debug("[Decomposer] Task is simple, skipping decomposition")
		return result, nil
	}

	d.log.Info("[Decomposer] Decomposing complex task (score: %d)", result.Complexity.Score)

	// Call LLM for decomposition
	messages := []cognitive.ChatMessage{
		{Role: "user", Content: fmt.Sprintf("Decompose this task into steps:\n\n%s", input)},
	}

	response, err := d.llm.Chat(ctx, messages, DecompositionSystemPrompt)
	if err != nil {
		return nil, fmt.Errorf("decomposition failed: %w", err)
	}

	// Parse the response
	steps, err := parseDecompositionResponse(response)
	if err != nil {
		d.log.Warn("[Decomposer] Failed to parse response: %v", err)
		// Fall back to single step
		result.Steps = []Step{{
			ID:          "fallback",
			Description: "Execute via LLM (decomposition failed)",
			Type:        StepLLM,
			Prompt:      input,
			RiskLevel:   "medium",
		}}
		return result, nil
	}

	result.Steps = steps.Steps
	result.EstimatedTime = steps.EstimatedTime
	result.RequiresApproval = steps.RequiresApproval

	// Check for high-risk operations
	for _, step := range result.Steps {
		if step.RiskLevel == "high" {
			result.RequiresApproval = true
			break
		}
	}

	d.log.Info("[Decomposer] Decomposed into %d steps (requires approval: %v)", len(result.Steps), result.RequiresApproval)

	return result, nil
}

// parseDecompositionResponse extracts steps from LLM response.
func parseDecompositionResponse(response string) (*DecompositionResult, error) {
	// Find JSON in response (LLM might wrap it in markdown code blocks)
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found in response")
	}

	// Parse the JSON structure
	var parsed struct {
		Steps            []Step `json:"steps"`
		EstimatedTime    string `json:"estimated_time"`
		RequiresApproval bool   `json:"requires_approval"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	if len(parsed.Steps) == 0 {
		return nil, fmt.Errorf("no steps found in decomposition")
	}

	result := &DecompositionResult{
		Steps:            parsed.Steps,
		EstimatedTime:    parsed.EstimatedTime,
		RequiresApproval: parsed.RequiresApproval,
	}

	return result, nil
}

// extractJSON finds and extracts JSON from a response that may contain markdown.
func extractJSON(response string) string {
	// Try to find JSON in markdown code blocks first
	codeBlockStart := "```json"
	codeBlockEnd := "```"

	if idx := findIgnoreCase(response, codeBlockStart); idx != -1 {
		start := idx + len(codeBlockStart)
		if end := findFrom(response, codeBlockEnd, start); end != -1 {
			return strings.TrimSpace(response[start:end])
		}
	}

	// Try plain ``` blocks
	if idx := findString(response, "```"); idx != -1 {
		start := idx + 3
		if end := findFrom(response, "```", start); end != -1 {
			content := strings.TrimSpace(response[start:end])
			if strings.HasPrefix(content, "{") {
				return content
			}
		}
	}

	// Find raw JSON by brace matching
	startIdx := -1
	endIdx := -1
	depth := 0

	for i, c := range response {
		if c == '{' {
			if depth == 0 {
				startIdx = i
			}
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 && startIdx != -1 {
				endIdx = i + 1
				break
			}
		}
	}

	if startIdx != -1 && endIdx != -1 {
		return response[startIdx:endIdx]
	}

	return ""
}

// findIgnoreCase finds a substring case-insensitively.
func findIgnoreCase(s, substr string) int {
	lower := strings.ToLower(s)
	lowerSubstr := strings.ToLower(substr)
	return strings.Index(lower, lowerSubstr)
}

// findString finds a substring in a string.
func findString(s, substr string) int {
	return strings.Index(s, substr)
}

// findFrom finds a substring starting from a specific index.
func findFrom(s, substr string, start int) int {
	if start >= len(s) {
		return -1
	}
	if idx := strings.Index(s[start:], substr); idx != -1 {
		return start + idx
	}
	return -1
}

// ═══════════════════════════════════════════════════════════════════════════════
// WORKFLOW EXECUTION
// ═══════════════════════════════════════════════════════════════════════════════

// StepResult contains the outcome of executing a step.
type StepResult struct {
	StepID    string        `json:"step_id"`
	Success   bool          `json:"success"`
	Output    string        `json:"output,omitempty"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration"`
	Skipped   bool          `json:"skipped"`
	Timestamp time.Time     `json:"timestamp"`
}

// WorkflowResult contains the outcome of executing a workflow.
type WorkflowResult struct {
	Success      bool          `json:"success"`
	StepResults  []StepResult  `json:"step_results"`
	TotalSteps   int           `json:"total_steps"`
	CompletedSteps int         `json:"completed_steps"`
	FailedSteps  int           `json:"failed_steps"`
	SkippedSteps int           `json:"skipped_steps"`
	TotalDuration time.Duration `json:"total_duration"`
	FinalOutput  string        `json:"final_output,omitempty"`
}

// StepCallback is called for each step during workflow execution.
type StepCallback func(step *Step, result *StepResult)

// WorkflowExecutor runs decomposed workflows.
type WorkflowExecutor struct {
	llm cognitive.SimpleChatProvider
	log *logging.Logger
}

// NewWorkflowExecutor creates a new workflow executor.
func NewWorkflowExecutor(llm cognitive.SimpleChatProvider) *WorkflowExecutor {
	return &WorkflowExecutor{
		llm: llm,
		log: logging.Global(),
	}
}

// Execute runs a decomposed workflow.
func (e *WorkflowExecutor) Execute(ctx context.Context, workflow *DecompositionResult, callback StepCallback) (*WorkflowResult, error) {
	start := time.Now()

	result := &WorkflowResult{
		Success:     true,
		StepResults: make([]StepResult, 0),
		TotalSteps:  len(workflow.Steps),
	}

	// Track completed steps for dependency resolution
	completed := make(map[string]bool)

	for _, step := range workflow.Steps {
		// Check dependencies
		depsOK := true
		for _, dep := range step.DependsOn {
			if !completed[dep] {
				depsOK = false
				break
			}
		}

		stepResult := StepResult{
			StepID:    step.ID,
			Timestamp: time.Now(),
		}

		if !depsOK {
			// Skip if dependencies not met
			stepResult.Skipped = true
			stepResult.Error = "dependencies not met"
			result.SkippedSteps++
		} else {
			// Execute the step
			stepStart := time.Now()
			output, err := e.executeStep(ctx, &step, workflow.OriginalInput)
			stepResult.Duration = time.Since(stepStart)

			if err != nil {
				stepResult.Success = false
				stepResult.Error = err.Error()
				result.FailedSteps++
				result.Success = false
			} else {
				stepResult.Success = true
				stepResult.Output = output
				result.CompletedSteps++
				completed[step.ID] = true
			}
		}

		result.StepResults = append(result.StepResults, stepResult)

		// Call the callback
		if callback != nil {
			callback(&step, &stepResult)
		}

		// Stop on failure (could be configurable)
		if !stepResult.Success && !step.Optional {
			break
		}
	}

	result.TotalDuration = time.Since(start)

	// Compile final output from step outputs
	if len(result.StepResults) > 0 {
		lastSuccess := result.StepResults[len(result.StepResults)-1]
		if lastSuccess.Success {
			result.FinalOutput = lastSuccess.Output
		}
	}

	return result, nil
}

// executeStep runs a single workflow step.
func (e *WorkflowExecutor) executeStep(ctx context.Context, step *Step, originalInput string) (string, error) {
	e.log.Debug("[Workflow] Executing step %s: %s", step.ID, step.Description)

	switch step.Type {
	case StepLLM:
		// Call LLM with the step prompt or original input
		prompt := step.Prompt
		if prompt == "" {
			prompt = originalInput
		}
		messages := []cognitive.ChatMessage{
			{Role: "user", Content: prompt},
		}
		return e.llm.Chat(ctx, messages, "")

	case StepTool:
		// Tool execution would integrate with the agent's tool system
		return fmt.Sprintf("Tool %s execution not implemented in decomposer", step.Tool), nil

	case StepTemplate:
		// Template execution would integrate with template engine
		return fmt.Sprintf("Template %s execution not implemented in decomposer", step.TemplateID), nil

	case StepApproval:
		// Approval steps would pause and wait for user input
		return "Approval granted", nil

	default:
		return "", fmt.Errorf("unknown step type: %s", step.Type)
	}
}
