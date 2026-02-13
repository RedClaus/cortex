package brain

import (
	"context"
	"fmt"
	stdlog "log"
	"strings"
	"sync"
	"time"
)

// truncate shortens a string for logging
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// PhaseExecutor runs lobes according to a thinking strategy.
type PhaseExecutor struct {
	registry   LobeRegistry
	monitor    *SystemMonitor
	maxReplans int
}

// LobeRegistry provides access to registered lobes.
type LobeRegistry interface {
	Get(id LobeID) (Lobe, bool)
	GetAll(ids []LobeID) []Lobe
}

// ExecutionContext holds state for a single execution run.
type ExecutionContext struct {
	Blackboard        *Blackboard
	Strategy          *ThinkingStrategy
	Results           []*LobeResult
	ReplanCount       int
	StartTime         time.Time
	CurrentPhaseIndex int
}

// ExecutionResult is the final output of running a strategy.
type ExecutionResult struct {
	LobeResults    []*LobeResult         `json:"lobe_results"`
	FinalContent   interface{}           `json:"final_content"`
	TotalTime      time.Duration         `json:"total_time"`
	ReplanCount    int                   `json:"replan_count"`
	Phases         []PhaseResult         `json:"phases"`
	Strategy       *ThinkingStrategy     `json:"strategy,omitempty"`
	Classification *ClassificationResult `json:"classification,omitempty"`
}

// PhaseResult captures the outcome of a single phase.
type PhaseResult struct {
	PhaseName   string        `json:"phase_name"`
	LobeResults []*LobeResult `json:"lobe_results"`
	Duration    time.Duration `json:"duration"`
	Replanned   bool          `json:"replanned"`
}

// NewPhaseExecutor creates an executor with the given registry and monitor.
func NewPhaseExecutor(registry LobeRegistry, monitor *SystemMonitor) *PhaseExecutor {
	return &PhaseExecutor{
		registry:   registry,
		monitor:    monitor,
		maxReplans: 3,
	}
}

// Execute runs the complete strategy and returns aggregated results.
func (e *PhaseExecutor) Execute(ctx context.Context, input LobeInput, strategy *ThinkingStrategy) (*ExecutionResult, error) {
	startTime := time.Now()

	stdlog.Printf("[Brain] Cognitive processing started: strategy=%s phases=%d", strategy.Name, len(strategy.Phases))
	stdlog.Printf("[Brain] Input: %s", truncate(input.RawInput, 100))

	// Initialize Blackboard if not present in input (LobeInput doesn't have it, so we create one)
	// We populate it with initial data.
	bb := NewBlackboard()
	bb.Set("raw_input", input.RawInput)
	if input.ParsedIntent != nil {
		bb.Set("intent", input.ParsedIntent)
	}
	if input.PhaseConfig != nil {
		for k, v := range input.PhaseConfig {
			bb.Set(k, v)
		}
	}

	ec := &ExecutionContext{
		Blackboard:        bb,
		Strategy:          strategy,
		Results:           make([]*LobeResult, 0),
		ReplanCount:       0,
		StartTime:         startTime,
		CurrentPhaseIndex: 0,
	}

	var phaseResults []PhaseResult

	// Iterate phases. Use index because strategy.Phases might grow during execution.
	for i := 0; i < len(ec.Strategy.Phases); i++ {
		ec.CurrentPhaseIndex = i
		phase := ec.Strategy.Phases[i]

		stdlog.Printf("[Brain] Phase %d/%d: %s (lobes: %v)", i+1, len(ec.Strategy.Phases), phase.Name, phase.Lobes)

		// Execute phase
		pResult, err := e.executePhase(ctx, phase, input, ec.Blackboard)
		if err != nil {
			stdlog.Printf("[Brain] Phase %s FAILED: %v", phase.Name, err)
			return nil, fmt.Errorf("failed to execute phase %s: %w", phase.Name, err)
		}

		stdlog.Printf("[Brain] Phase %s completed: %d lobe results", phase.Name, len(pResult.LobeResults))

		// Handle replanning
		if phase.CanReplan {
			for _, res := range pResult.LobeResults {
				if res.RequestReplan {
					didReplan, err := e.handleReplan(ctx, res, ec)
					if err != nil {
						// We treat replan errors as non-fatal for the whole execution,
						// but maybe we should log it. For now, we just proceed.
						// Or return error if strict.
						return nil, fmt.Errorf("replan failed: %w", err)
					}
					if didReplan {
						pResult.Replanned = true
					}
				}
			}
		}

		phaseResults = append(phaseResults, *pResult)
		ec.Results = append(ec.Results, pResult.LobeResults...)
	}

	executionResult := &ExecutionResult{
		LobeResults:  ec.Results,
		FinalContent: e.aggregateResults(ec.Results),
		TotalTime:    time.Since(startTime),
		ReplanCount:  ec.ReplanCount,
		Phases:       phaseResults,
		Strategy:     strategy,
	}

	return executionResult, nil
}

// executePhase runs a single phase (parallel or sequential).
func (e *PhaseExecutor) executePhase(ctx context.Context, phase ExecutionPhase, input LobeInput, bb *Blackboard) (*PhaseResult, error) {
	startTime := time.Now()

	// Resolve lobes
	var lobes []Lobe
	for _, id := range phase.Lobes {
		if lobe, ok := e.registry.Get(id); ok {
			lobes = append(lobes, lobe)
		}
		// If lobe not found, we skip it.
	}

	// Calculate timeout
	timeout := time.Duration(phase.TimeoutMS) * time.Millisecond
	if timeout == 0 {
		timeout = 5 * time.Minute // Default timeout - increased for local LLM inference
	}

	var results []*LobeResult
	var err error

	if phase.Parallel {
		results, err = e.executeParallel(ctx, lobes, input, bb, timeout)
	} else {
		results, err = e.executeSequential(ctx, lobes, input, bb, timeout)
	}

	if err != nil {
		return nil, err
	}

	return &PhaseResult{
		PhaseName:   phase.Name,
		LobeResults: results,
		Duration:    time.Since(startTime),
		Replanned:   false,
	}, nil
}

// executeParallel runs lobes concurrently.
// Each lobe gets its own clone of the blackboard to prevent frozen-blackboard panics.
func (e *PhaseExecutor) executeParallel(ctx context.Context, lobes []Lobe, input LobeInput, bb *Blackboard, timeout time.Duration) ([]*LobeResult, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var wg sync.WaitGroup
	resultsChan := make(chan *LobeResult, len(lobes))
	errChan := make(chan error, len(lobes))

	for _, lobe := range lobes {
		wg.Add(1)
		go func(l Lobe) {
			defer wg.Done()
			if ctx.Err() != nil {
				return
			}

			// Clone the blackboard for each parallel lobe.
			// This prevents lobes that call Clone() from freezing the shared parent.
			lobeBB := bb.Clone()

			// Process with the isolated clone
			res, err := l.Process(ctx, input, lobeBB)
			if err != nil {
				errChan <- fmt.Errorf("lobe %s failed: %w", l.ID(), err)
				return
			}
			resultsChan <- res
		}(lobe)
	}

	wg.Wait()
	close(resultsChan)
	close(errChan)

	// Collect results into a slice first
	var results []*LobeResult
	for res := range resultsChan {
		results = append(results, res)
	}

	// Merge all results into a working copy of the blackboard.
	// The original bb is now frozen from Clone() calls above, so we work on a clone.
	workingBB := bb.Clone()
	for _, res := range results {
		// Clone again before each merge to handle any freezing from previous merges
		nextBB := workingBB.Clone()
		nextBB.Merge(res)
		workingBB = nextBB
	}

	// Check for errors
	// If any lobe failed, we report it.
	if len(errChan) > 0 {
		return results, <-errChan
	}

	return results, nil
}

// executeSequential runs lobes one after another.
func (e *PhaseExecutor) executeSequential(ctx context.Context, lobes []Lobe, input LobeInput, bb *Blackboard, timeout time.Duration) ([]*LobeResult, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var results []*LobeResult

	// Create a working copy of the blackboard for sequential execution.
	// This prevents lobes that call Clone() from freezing our main blackboard.
	workingBB := bb.Clone()

	for _, lobe := range lobes {
		if ctx.Err() != nil {
			return results, ctx.Err()
		}

		// Pass the working blackboard to the lobe
		res, err := lobe.Process(ctx, input, workingBB)
		if err != nil {
			return results, fmt.Errorf("lobe %s failed: %w", lobe.ID(), err)
		}

		results = append(results, res)

		// Create a new working copy for the next lobe (in case this lobe froze it).
		// Clone first, then merge result into the new clone.
		nextBB := workingBB.Clone()
		nextBB.Merge(res)
		workingBB = nextBB
	}

	return results, nil
}

// handleReplan processes replan requests from lobes.
func (e *PhaseExecutor) handleReplan(ctx context.Context, result *LobeResult, ec *ExecutionContext) (bool, error) {
	if ec.ReplanCount >= e.maxReplans {
		return false, nil
	}

	if len(result.SuggestLobes) == 0 {
		return false, nil
	}

	ec.ReplanCount++
	nextIdx := ec.CurrentPhaseIndex + 1

	if nextIdx < len(ec.Strategy.Phases) {
		// Add to next phase
		ec.Strategy.Phases[nextIdx].Lobes = append(ec.Strategy.Phases[nextIdx].Lobes, result.SuggestLobes...)
		return true, nil
	}

	// Create new phase if at end
	newPhase := ExecutionPhase{
		Name:      "Replan Extension",
		Lobes:     result.SuggestLobes,
		Parallel:  true, // Default to parallel
		TimeoutMS: 30000,
		CanReplan: true,
	}
	ec.Strategy.Phases = append(ec.Strategy.Phases, newPhase)
	return true, nil
}

// aggregateResults combines all lobe results into final output.
func (e *PhaseExecutor) aggregateResults(results []*LobeResult) interface{} {
	if len(results) == 0 {
		return nil
	}

	// Check if all contents are strings
	allStrings := true
	for _, res := range results {
		if _, ok := res.Content.(string); !ok {
			allStrings = false
			break
		}
	}

	if allStrings {
		var sb strings.Builder
		for i, res := range results {
			if i > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(res.Content.(string))
		}
		return sb.String()
	}

	// Fallback: Return the content of the last result
	return results[len(results)-1].Content
}
