package traces

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/normanking/cortex/internal/agent"
	"github.com/normanking/cortex/internal/memory"
)

// TraceCollector captures reasoning steps during agent execution.
// It implements agent.StepCallback to receive step events.
type TraceCollector struct {
	mu           sync.Mutex
	store        *TraceStore
	embedder     memory.Embedder
	currentTrace *ReasoningTrace
	startTime    time.Time
	stepTimes    []time.Time
	originalCallback agent.StepCallback
}

// NewTraceCollector creates a new trace collector.
func NewTraceCollector(store *TraceStore, embedder memory.Embedder) *TraceCollector {
	return &TraceCollector{
		store:    store,
		embedder: embedder,
	}
}

// Start begins collecting a new trace for the given query.
// Returns a StepCallback that should be passed to the agent.
func (tc *TraceCollector) Start(query string, originalCallback agent.StepCallback) agent.StepCallback {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.currentTrace = &ReasoningTrace{
		ID:        "trace_" + uuid.New().String(),
		Query:     query,
		Steps:     make([]ReasoningStep, 0),
		ToolsUsed: make([]string, 0),
		Outcome:   OutcomeSuccess, // Assume success, update on failure
		CreatedAt: time.Now(),
		Metadata:  make(map[string]string),
	}
	tc.startTime = time.Now()
	tc.stepTimes = []time.Time{tc.startTime}
	tc.originalCallback = originalCallback

	return tc.onStep
}

// onStep handles step events from the agent.
func (tc *TraceCollector) onStep(event *agent.StepEvent) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if tc.currentTrace == nil {
		return
	}

	// Record step timing
	now := time.Now()
	var duration time.Duration
	if len(tc.stepTimes) > 0 {
		duration = now.Sub(tc.stepTimes[len(tc.stepTimes)-1])
	}
	tc.stepTimes = append(tc.stepTimes, now)

	// Convert to reasoning step
	step := StepEventToReasoningStep(event)
	step.Duration = duration
	tc.currentTrace.Steps = append(tc.currentTrace.Steps, step)

	// Track tools used
	if event.ToolName != "" {
		found := false
		for _, t := range tc.currentTrace.ToolsUsed {
			if t == event.ToolName {
				found = true
				break
			}
		}
		if !found {
			tc.currentTrace.ToolsUsed = append(tc.currentTrace.ToolsUsed, event.ToolName)
		}
	}

	// Check for errors
	if event.Type == agent.EventError || (event.Error != "" && !event.Success) {
		tc.currentTrace.Outcome = OutcomeFailed
	}

	// Call original callback if set
	if tc.originalCallback != nil {
		tc.originalCallback(event)
	}
}

// Finish completes the current trace and stores it.
// Returns the completed trace or nil if nothing was collected.
func (tc *TraceCollector) Finish(ctx context.Context, outcome TraceOutcome) (*ReasoningTrace, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if tc.currentTrace == nil || len(tc.currentTrace.Steps) == 0 {
		return nil, nil
	}

	trace := tc.currentTrace
	trace.Outcome = outcome
	trace.TotalDuration = time.Since(tc.startTime)

	// Calculate success score based on outcome and step efficiency
	trace.SuccessScore = tc.calculateSuccessScore(trace)

	// Estimate token usage (rough: ~4 chars per token)
	totalChars := len(trace.Query)
	for _, step := range trace.Steps {
		totalChars += len(step.Content) + len(step.ToolInput) + len(step.ToolOutput)
	}
	trace.TokensUsed = totalChars / 4

	// Generate approach summary from first thinking step
	for _, step := range trace.Steps {
		if step.Action == ActionThink && len(step.Content) > 0 {
			trace.Approach = truncate(step.Content, 200)
			break
		}
	}

	// Generate query embedding
	if tc.embedder != nil {
		emb, err := tc.embedder.Embed(ctx, trace.Query)
		if err == nil {
			trace.QueryEmbedding = emb
		}
	}

	// Store the trace
	if err := tc.store.StoreTrace(ctx, trace); err != nil {
		return trace, err
	}

	// Clear current trace
	tc.currentTrace = nil

	return trace, nil
}

// Cancel aborts the current trace collection without storing.
func (tc *TraceCollector) Cancel() {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.currentTrace = nil
}

// SetLobesActivated records which brain lobes were activated during processing.
func (tc *TraceCollector) SetLobesActivated(lobes []string) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	if tc.currentTrace != nil {
		tc.currentTrace.LobesActivated = lobes
	}
}

// SetMetadata adds metadata to the current trace.
func (tc *TraceCollector) SetMetadata(key, value string) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	if tc.currentTrace != nil {
		tc.currentTrace.Metadata[key] = value
	}
}

// calculateSuccessScore computes a success score for the trace.
func (tc *TraceCollector) calculateSuccessScore(trace *ReasoningTrace) float64 {
	if trace.Outcome == OutcomeFailed {
		return 0.0
	}

	score := 0.5 // Base score

	// Bonus for successful completion
	if trace.Outcome == OutcomeSuccess {
		score += 0.3
	} else if trace.Outcome == OutcomePartial {
		score += 0.1
	}

	// Bonus for efficiency (fewer steps = better)
	stepCount := len(trace.Steps)
	if stepCount <= 3 {
		score += 0.2
	} else if stepCount <= 5 {
		score += 0.1
	} else if stepCount > 10 {
		score -= 0.1
	}

	// Cap at 1.0
	if score > 1.0 {
		score = 1.0
	}
	if score < 0.0 {
		score = 0.0
	}

	return score
}

// CurrentTraceID returns the ID of the current trace being collected.
func (tc *TraceCollector) CurrentTraceID() string {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	if tc.currentTrace != nil {
		return tc.currentTrace.ID
	}
	return ""
}

// IsCollecting returns true if a trace is currently being collected.
func (tc *TraceCollector) IsCollecting() bool {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	return tc.currentTrace != nil
}
