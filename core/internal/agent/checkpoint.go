package agent

import (
	"context"
	"fmt"
)

// ═══════════════════════════════════════════════════════════════════════════════
// SUPERVISED AGENTIC MODE - CHECKPOINT SYSTEM
// ═══════════════════════════════════════════════════════════════════════════════
//
// Instead of hard-stopping on loops or step limits, supervised mode pauses
// and asks the user what to do. This enables complex tasks while keeping
// the user in control.

// CheckpointReason identifies why the agent paused.
type CheckpointReason string

const (
	CheckpointLoopDetected  CheckpointReason = "loop_detected"   // Repeated identical tool call
	CheckpointStepLimit     CheckpointReason = "step_limit"      // Reached max steps
	CheckpointToolError     CheckpointReason = "tool_error"      // Tool execution failed
	CheckpointEmptyResults  CheckpointReason = "empty_results"   // Multiple tools returned no useful data
	CheckpointComplexTask   CheckpointReason = "complex_task"    // Task detected as complex upfront
	CheckpointCostThreshold CheckpointReason = "cost_threshold"  // Token/cost limit approaching
	CheckpointUserRequested CheckpointReason = "user_requested"  // User manually requested pause
	CheckpointLongRunning   CheckpointReason = "long_running"    // Task taking longer than expected (>60s)
)

// CheckpointAction is the user's response to a checkpoint.
type CheckpointAction string

const (
	CheckpointContinue       CheckpointAction = "continue"        // Continue with N more steps
	CheckpointGuide          CheckpointAction = "guide"           // User provides guidance/correction
	CheckpointSimplify       CheckpointAction = "simplify"        // Break into smaller tasks
	CheckpointAbort          CheckpointAction = "abort"           // Stop and show progress
	CheckpointEscalate       CheckpointAction = "escalate"        // Switch to frontier model
	CheckpointRetry          CheckpointAction = "retry"           // Retry the failed step
	CheckpointWait           CheckpointAction = "wait"            // Keep waiting for current approach
	CheckpointDifferentApproach CheckpointAction = "different"    // Try a different approach
)

// CheckpointOption represents a choice the user can make at a checkpoint.
type CheckpointOption struct {
	Action      CheckpointAction `json:"action"`
	Key         string           `json:"key"`         // Keyboard shortcut (e.g., "c", "g")
	Label       string           `json:"label"`       // Display label (e.g., "Continue")
	Description string           `json:"description"` // Explanation of what this does
}

// Checkpoint represents a pause point where the agent needs user input.
type Checkpoint struct {
	Reason        CheckpointReason  `json:"reason"`
	ReasonMessage string            `json:"reason_message"` // Human-readable explanation
	TaskSummary   string            `json:"task_summary"`   // What the agent was trying to do
	StepsRun      int               `json:"steps_run"`
	StepsMax      int               `json:"steps_max"`
	LastAction    string            `json:"last_action"`    // Last tool call or action
	LastError     string            `json:"last_error"`     // Error if tool failed
	Progress      []string          `json:"progress"`       // Summary of steps completed
	ToolsUsed     []string          `json:"tools_used"`
	TokensUsed    int               `json:"tokens_used"`
	Options       []CheckpointOption `json:"options"`
}

// CheckpointResponse is the user's answer to a checkpoint.
type CheckpointResponse struct {
	Action         CheckpointAction `json:"action"`
	Guidance       string           `json:"guidance"`        // User guidance if Action == ActionGuide
	AdditionalSteps int             `json:"additional_steps"` // Extra steps if Action == ActionContinue
}

// DefaultCheckpointOptions returns the standard options for a checkpoint.
func DefaultCheckpointOptions(reason CheckpointReason) []CheckpointOption {
	options := []CheckpointOption{
		{
			Action:      CheckpointContinue,
			Key:         "c",
			Label:       "Continue",
			Description: "Allow 5 more steps to complete the task",
		},
		{
			Action:      CheckpointGuide,
			Key:         "g",
			Label:       "Guide",
			Description: "Provide a hint or correction to help the agent",
		},
		{
			Action:      CheckpointAbort,
			Key:         "a",
			Label:       "Abort",
			Description: "Stop and show what was accomplished",
		},
	}

	// Add context-specific options
	switch reason {
	case CheckpointToolError:
		options = append([]CheckpointOption{{
			Action:      CheckpointRetry,
			Key:         "r",
			Label:       "Retry",
			Description: "Retry the failed operation",
		}}, options...)
	case CheckpointLoopDetected, CheckpointEmptyResults:
		options = append(options, CheckpointOption{
			Action:      CheckpointEscalate,
			Key:         "e",
			Label:       "Escalate",
			Description: "Switch to a more capable model",
		})
	case CheckpointStepLimit:
		options[0].Description = "Allow 10 more steps to complete the task"
	case CheckpointLongRunning:
		// Special options for long-running tasks
		options = []CheckpointOption{
			{
				Action:      CheckpointWait,
				Key:         "w",
				Label:       "Wait",
				Description: "Keep waiting - the task is still progressing",
			},
			{
				Action:      CheckpointDifferentApproach,
				Key:         "d",
				Label:       "Different approach",
				Description: "Interrupt and try a different strategy",
			},
			{
				Action:      CheckpointEscalate,
				Key:         "e",
				Label:       "Escalate",
				Description: "Switch to a faster/smarter model",
			},
			{
				Action:      CheckpointAbort,
				Key:         "a",
				Label:       "Abort",
				Description: "Stop and show what was accomplished so far",
			},
		}
	}

	return options
}

// NewCheckpoint creates a checkpoint with standard options.
func NewCheckpoint(reason CheckpointReason, reasonMsg string, stepsRun, stepsMax int) *Checkpoint {
	return &Checkpoint{
		Reason:        reason,
		ReasonMessage: reasonMsg,
		StepsRun:      stepsRun,
		StepsMax:      stepsMax,
		Progress:      make([]string, 0),
		ToolsUsed:     make([]string, 0),
		Options:       DefaultCheckpointOptions(reason),
	}
}

// AgenticMode determines how the agent handles checkpoints.
type AgenticMode string

const (
	AgenticModeSupervised AgenticMode = "supervised" // Pause and ask user at checkpoints
	AgenticModeAutonomous AgenticMode = "autonomous" // Run to completion (current behavior)
	AgenticModeDisabled   AgenticMode = "disabled"   // No agentic execution
)

// SupervisedConfig configures supervised agentic mode.
type SupervisedConfig struct {
	Mode                  AgenticMode `yaml:"mode" json:"mode"`
	StepLimit             int         `yaml:"step_limit" json:"step_limit"`
	CheckpointOnLoop      bool        `yaml:"checkpoint_on_loop" json:"checkpoint_on_loop"`
	CheckpointOnError     bool        `yaml:"checkpoint_on_error" json:"checkpoint_on_error"`
	CheckpointOnStepLimit bool        `yaml:"checkpoint_on_step_limit" json:"checkpoint_on_step_limit"`
	AutoEscalateOnLoop    bool        `yaml:"auto_escalate_on_loop" json:"auto_escalate_on_loop"`
	CostCheckpointTokens  int         `yaml:"cost_checkpoint_tokens" json:"cost_checkpoint_tokens"` // Pause after N tokens
	LongRunningTimeout    int         `yaml:"long_running_timeout" json:"long_running_timeout"`     // Seconds before long-running checkpoint (default: 60)
}

// DefaultSupervisedConfig returns sensible defaults.
func DefaultSupervisedConfig() SupervisedConfig {
	return SupervisedConfig{
		Mode:                  AgenticModeSupervised,
		StepLimit:             10,
		CheckpointOnLoop:      true,
		CheckpointOnError:     true,
		CheckpointOnStepLimit: true,
		AutoEscalateOnLoop:    false,
		CostCheckpointTokens:  10000, // ~$0.01 for most models
		LongRunningTimeout:    60,    // 60 seconds before asking user
	}
}

// CheckpointHandler is called when the agent needs user input.
// It should block until the user responds.
// Returns nil to use default behavior (abort for supervised, continue for autonomous).
type CheckpointHandler func(ctx context.Context, checkpoint *Checkpoint) (*CheckpointResponse, error)

// FormatCheckpoint creates a human-readable checkpoint message.
func FormatCheckpoint(cp *Checkpoint) string {
	header := fmt.Sprintf("⚠️  AGENTIC CHECKPOINT: %s\n", cp.ReasonMessage)

	progress := fmt.Sprintf("\nProgress: %d/%d steps", cp.StepsRun, cp.StepsMax)
	if len(cp.ToolsUsed) > 0 {
		progress += fmt.Sprintf(" | Tools: %v", cp.ToolsUsed)
	}
	if cp.TokensUsed > 0 {
		progress += fmt.Sprintf(" | Tokens: %d", cp.TokensUsed)
	}

	var details string
	if cp.LastAction != "" {
		details += fmt.Sprintf("\nLast action: %s", cp.LastAction)
	}
	if cp.LastError != "" {
		details += fmt.Sprintf("\nError: %s", cp.LastError)
	}

	options := "\n\nWhat would you like to do?\n"
	for _, opt := range cp.Options {
		options += fmt.Sprintf("  [%s] %s - %s\n", opt.Key, opt.Label, opt.Description)
	}

	return header + progress + details + options
}
