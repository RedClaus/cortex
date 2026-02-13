// Package traces provides reasoning trace storage and retrieval for System 3 meta-cognition.
// Traces capture successful reasoning paths for reuse, enabling 80% step reduction
// on recurring problems (per Sophia paper arXiv:2512.18202).
package traces

import (
	"time"

	"github.com/normanking/cortex/internal/agent"
	"github.com/normanking/cortex/internal/memory"
)

// EpisodeTypeReasoningTrace is the episode type for reasoning traces.
const EpisodeTypeReasoningTrace = "reasoning_trace"

// TraceOutcome represents the result of a reasoning trace.
type TraceOutcome string

const (
	OutcomeSuccess    TraceOutcome = "success"     // Task completed successfully
	OutcomePartial    TraceOutcome = "partial"     // Partial completion
	OutcomeFailed     TraceOutcome = "failed"      // Task failed
	OutcomeInterrupted TraceOutcome = "interrupted" // User interrupted
)

// ReasoningTrace captures a complete reasoning session for future reuse.
type ReasoningTrace struct {
	ID             string            `json:"id"`
	Query          string            `json:"query"`            // Original user query
	QueryEmbedding []float32         `json:"query_embedding"`  // For similarity search
	Approach       string            `json:"approach"`         // High-level strategy description
	Steps          []ReasoningStep   `json:"steps"`            // Individual reasoning steps
	Outcome        TraceOutcome      `json:"outcome"`          // Success/failure
	SuccessScore   float64           `json:"success_score"`    // 0-1 quality score
	ReusedCount    int               `json:"reused_count"`     // Times this trace was reused
	ToolsUsed      []string          `json:"tools_used"`       // Tools invoked during trace
	LobesActivated []string          `json:"lobes_activated"`  // Brain lobes that participated
	TotalDuration  time.Duration     `json:"total_duration"`   // Total execution time
	TokensUsed     int               `json:"tokens_used"`      // Estimated token consumption
	CreatedAt      time.Time         `json:"created_at"`
	LastUsedAt     time.Time         `json:"last_used_at"`
	Metadata       map[string]string `json:"metadata"`
}

// ReasoningStep captures a single step in the reasoning process.
type ReasoningStep struct {
	StepNum    int               `json:"step_num"`
	Action     StepAction        `json:"action"`       // Type of action taken
	Content    string            `json:"content"`      // Thinking/output content
	ToolName   string            `json:"tool_name"`    // Tool called (if any)
	ToolInput  string            `json:"tool_input"`   // Tool input (if any)
	ToolOutput string            `json:"tool_output"`  // Tool output (if any)
	Success    bool              `json:"success"`      // Step succeeded
	Error      string            `json:"error"`        // Error message (if any)
	Duration   time.Duration     `json:"duration"`     // Step duration
	Timestamp  time.Time         `json:"timestamp"`
}

// StepAction represents the type of reasoning step.
type StepAction string

const (
	ActionThink    StepAction = "think"      // Agent reasoning
	ActionToolCall StepAction = "tool_call"  // Tool invocation
	ActionToolResult StepAction = "tool_result" // Tool result processing
	ActionConclude StepAction = "conclude"   // Final conclusion
)

// TraceSimilarity represents a similar trace with its match score.
type TraceSimilarity struct {
	Trace      *ReasoningTrace `json:"trace"`
	Similarity float64         `json:"similarity"` // 0-1 cosine similarity
}

// TraceStats provides statistics about stored traces.
type TraceStats struct {
	TotalTraces      int     `json:"total_traces"`
	SuccessfulTraces int     `json:"successful_traces"`
	ReusedTraces     int     `json:"reused_traces"`
	AvgSuccessScore  float64 `json:"avg_success_score"`
	AvgStepsPerTrace float64 `json:"avg_steps_per_trace"`
	TotalReuses      int     `json:"total_reuses"`
}

// StepEventToReasoningStep converts an agent StepEvent to a ReasoningStep.
func StepEventToReasoningStep(event *agent.StepEvent) ReasoningStep {
	action := ActionThink
	switch event.Type {
	case agent.EventThinking:
		action = ActionThink
	case agent.EventToolCall:
		action = ActionToolCall
	case agent.EventToolResult:
		action = ActionToolResult
	case agent.EventComplete:
		action = ActionConclude
	}

	return ReasoningStep{
		StepNum:    event.Step,
		Action:     action,
		Content:    event.Message,
		ToolName:   event.ToolName,
		ToolInput:  event.ToolInput,
		ToolOutput: event.Output,
		Success:    event.Success,
		Error:      event.Error,
		Timestamp:  time.Now(),
	}
}

// ToEpisode converts a ReasoningTrace to a memory Episode for storage.
func (t *ReasoningTrace) ToEpisode() *memory.Episode {
	return &memory.Episode{
		ID:               t.ID,
		EpisodeType:      EpisodeTypeReasoningTrace,
		StartedAt:        t.CreatedAt,
		Title:            truncate(t.Query, 100),
		Summary:          t.Approach,
		SummaryEmbedding: t.QueryEmbedding,
		MemoryCount:      len(t.Steps),
		TokenEstimate:    t.TokensUsed,
		Metadata:         t.Metadata,
		IsActive:         false, // Traces are always closed
	}
}

// truncate shortens a string to maxLen, adding ellipsis if needed.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
