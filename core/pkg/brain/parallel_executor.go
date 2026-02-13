package brain

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// BranchStatus represents the state of an execution branch.
type BranchStatus string

const (
	// BranchPending indicates the branch has not started execution.
	BranchPending BranchStatus = "pending"
	// BranchRunning indicates the branch is currently executing.
	BranchRunning BranchStatus = "running"
	// BranchCompleted indicates the branch finished successfully.
	BranchCompleted BranchStatus = "completed"
	// BranchFailed indicates the branch encountered an error.
	BranchFailed BranchStatus = "failed"
	// BranchPruned indicates the branch was pruned due to low confidence.
	BranchPruned BranchStatus = "pruned"
)

const (
	// DefaultMaxBranches is the default number of parallel branches to execute.
	DefaultMaxBranches = 3
	// DefaultPruneThreshold is the minimum confidence to keep a branch.
	DefaultPruneThreshold = 0.3
)

// ErrNoBranchesCompleted indicates all branches failed or were pruned.
var ErrNoBranchesCompleted = errors.New("parallel_executor: no branches completed successfully")

// ExecutionBranch represents a single parallel execution path.
type ExecutionBranch struct {
	// ID uniquely identifies this branch.
	ID string `json:"id"`
	// Status indicates the current state of the branch.
	Status BranchStatus `json:"status"`
	// Confidence is the branch's confidence score (0.0-1.0).
	Confidence float64 `json:"confidence"`
	// ActiveLobes are the lobes used in this branch.
	ActiveLobes []LobeID `json:"active_lobes"`
	// StartedAt records when execution began.
	StartedAt time.Time `json:"started_at"`
	// Duration records how long execution took.
	Duration time.Duration `json:"duration"`
	// Error holds any error that occurred.
	Error error `json:"-"`
	// Result holds the branch's execution result.
	Result *LobeResult `json:"result,omitempty"`
}

// ToResult converts the branch to a ParallelExecutionResult.
func (b *ExecutionBranch) ToResult() *ParallelExecutionResult {
	var content interface{}
	if b.Result != nil {
		content = b.Result.Content
	}
	return &ParallelExecutionResult{
		Content:     content,
		Confidence:  b.Confidence,
		BranchCount: 1,
		BranchID:    b.ID,
		Duration:    b.Duration,
	}
}

// ParallelExecutionResult is the final output from parallel execution.
type ParallelExecutionResult struct {
	// Content is the final response content.
	Content interface{} `json:"content"`
	// Confidence is the aggregated confidence score.
	Confidence float64 `json:"confidence"`
	// BranchCount is how many branches completed successfully.
	BranchCount int `json:"branch_count"`
	// BranchID identifies which branch produced the result.
	BranchID string `json:"branch_id"`
	// Duration is how long execution took.
	Duration time.Duration `json:"duration"`
}

// SkillLibraryInterface defines the interface for skill retrieval.
// This allows Phase 3's SkillLibrary to be injected when available.
type SkillLibraryInterface interface {
	// FindRelevantSkills retrieves skills matching the task description.
	FindRelevantSkills(ctx context.Context, taskDescription string, limit int) ([]Skill, error)
}

// Skill represents an executable pattern learned from successful execution.
// This mirrors the Skill type from internal/memory/skill_library.go.
type Skill struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Pattern     string   `json:"pattern"`
	InputSchema string   `json:"input_schema"`
	Examples    []string `json:"examples"`
	Tags        []string `json:"tags"`
}

// NextScenePredictorInterface defines the interface for memory prediction.
// This allows Phase 2's NextScenePredictor to be injected when available.
type NextScenePredictorInterface interface {
	// Predict returns predicted relevant memory cubes for the input.
	Predict(ctx context.Context, partialInput string) []PredictedMemory
}

// PredictedMemory represents a memory cube predicted to be relevant.
type PredictedMemory struct {
	ID         string  `json:"id"`
	Content    string  `json:"content"`
	Confidence float64 `json:"confidence"`
}

// ParallelExecutor manages multiple solution branches for enhanced reasoning.
// It implements the Mind Evolution pattern: parallel divergent/convergent thinking.
//
// Brain Alignment: Like the prefrontal cortex coordinating multiple cognitive
// processes simultaneously before converging on a decision.
type ParallelExecutor struct {
	// maxBranches is the maximum number of parallel branches to execute.
	maxBranches int
	// pruneThreshold is the minimum confidence to keep a branch.
	pruneThreshold float64
	// skillLibrary provides reusable execution patterns (Phase 3).
	skillLibrary SkillLibraryInterface
	// predictor provides proactive memory loading (Phase 2).
	predictor NextScenePredictorInterface
	// executor runs the actual lobe execution.
	executor *PhaseExecutor
	// branchStrategies defines different strategies to try in parallel.
	branchStrategies []ThinkingStrategy
}

// ParallelExecutorOption configures the ParallelExecutor.
type ParallelExecutorOption func(*ParallelExecutor)

// WithMaxBranches sets the maximum number of parallel branches.
func WithMaxBranches(n int) ParallelExecutorOption {
	return func(pe *ParallelExecutor) {
		if n > 0 {
			pe.maxBranches = n
		}
	}
}

// WithPruneThreshold sets the minimum confidence threshold.
func WithPruneThreshold(threshold float64) ParallelExecutorOption {
	return func(pe *ParallelExecutor) {
		if threshold >= 0 && threshold <= 1 {
			pe.pruneThreshold = threshold
		}
	}
}

// WithSkillLibrary injects the skill library for pattern reuse.
func WithSkillLibrary(sl SkillLibraryInterface) ParallelExecutorOption {
	return func(pe *ParallelExecutor) {
		pe.skillLibrary = sl
	}
}

// WithPredictor injects the next-scene predictor for proactive memory loading.
func WithPredictor(p NextScenePredictorInterface) ParallelExecutorOption {
	return func(pe *ParallelExecutor) {
		pe.predictor = p
	}
}

// WithPhaseExecutor injects the executor for running lobes.
func WithPhaseExecutor(e *PhaseExecutor) ParallelExecutorOption {
	return func(pe *ParallelExecutor) {
		pe.executor = e
	}
}

// WithBranchStrategies sets custom strategies for parallel branches.
func WithBranchStrategies(strategies []ThinkingStrategy) ParallelExecutorOption {
	return func(pe *ParallelExecutor) {
		pe.branchStrategies = strategies
	}
}

// NewParallelExecutor creates a new ParallelExecutor with the given options.
func NewParallelExecutor(opts ...ParallelExecutorOption) *ParallelExecutor {
	pe := &ParallelExecutor{
		maxBranches:    DefaultMaxBranches,
		pruneThreshold: DefaultPruneThreshold,
	}

	for _, opt := range opts {
		opt(pe)
	}

	// Initialize default branch strategies if none provided
	if len(pe.branchStrategies) == 0 {
		pe.branchStrategies = defaultBranchStrategies()
	}

	return pe
}

// defaultBranchStrategies returns a set of diverse thinking strategies
// for parallel exploration (divergent thinking).
func defaultBranchStrategies() []ThinkingStrategy {
	return []ThinkingStrategy{
		// Branch 1: Quick reasoning (fast path)
		QuickAnswerStrategy(),
		// Branch 2: Deep reasoning (thorough path)
		DeepReasoningStrategy(),
		// Branch 3: Creative approach (lateral thinking)
		CreativeStrategy(),
	}
}

// Execute runs parallel branches with skill injection and consensus.
//
// The execution flow:
// 1. Predictor preloads relevant memories (if available)
// 2. Skill library provides reusable patterns (if available)
// 3. Branches execute in parallel with different strategies
// 4. Results are aggregated via confidence-based selection
func (pe *ParallelExecutor) Execute(ctx context.Context, input LobeInput, bb *Blackboard) (*ParallelExecutionResult, error) {
	startTime := time.Now()

	// Phase 1: Proactive memory loading (Next-Scene Prediction)
	if pe.predictor != nil {
		predictedCubes := pe.predictor.Predict(ctx, input.RawInput)
		for _, cube := range predictedCubes {
			// Inject predicted memories into the blackboard
			bb.AddMemory(Memory{
				ID:        cube.ID,
				Content:   cube.Content,
				Source:    "prediction",
				Relevance: cube.Confidence,
			})
		}
	}

	// Phase 2: Find relevant skills for potential reuse
	if pe.skillLibrary != nil {
		skills, err := pe.skillLibrary.FindRelevantSkills(ctx, input.RawInput, 3)
		if err == nil && len(skills) > 0 {
			bb.Set("available_skills", skills)
		}
	}

	// Phase 3: Generate and execute branches in parallel
	branches := pe.generateBranches(input)

	var wg sync.WaitGroup
	results := make(chan *ExecutionBranch, len(branches))

	for _, branch := range branches {
		wg.Add(1)
		go func(b *ExecutionBranch) {
			defer wg.Done()
			// Check for context cancellation before starting
			select {
			case <-ctx.Done():
				b.Status = BranchFailed
				b.Error = ctx.Err()
				results <- b
				return
			default:
			}
			// Clone blackboard for isolated branch execution
			branchBB := bb.Clone()
			pe.executeBranch(ctx, b, input, branchBB)
			results <- b
		}(branch)
	}

	// Close results channel when all branches complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Phase 4: Aggregate results via weighted consensus
	result, err := pe.aggregate(results)
	if err != nil {
		return nil, err
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

// generateBranches creates execution branches with different strategies.
func (pe *ParallelExecutor) generateBranches(input LobeInput) []*ExecutionBranch {
	numBranches := pe.maxBranches
	if numBranches > len(pe.branchStrategies) {
		numBranches = len(pe.branchStrategies)
	}

	branches := make([]*ExecutionBranch, numBranches)
	for i := 0; i < numBranches; i++ {
		strategy := pe.branchStrategies[i]
		branches[i] = &ExecutionBranch{
			ID:          fmt.Sprintf("branch_%d_%s", i, strategy.Name),
			Status:      BranchPending,
			Confidence:  0.5, // Neutral prior
			ActiveLobes: extractLobes(strategy),
		}
	}

	return branches
}

// extractLobes gets all unique lobes from a strategy.
func extractLobes(strategy ThinkingStrategy) []LobeID {
	lobeSet := make(map[LobeID]struct{})
	for _, phase := range strategy.Phases {
		for _, lobe := range phase.Lobes {
			lobeSet[lobe] = struct{}{}
		}
	}

	lobes := make([]LobeID, 0, len(lobeSet))
	for lobe := range lobeSet {
		lobes = append(lobes, lobe)
	}
	return lobes
}

// executeBranch runs a single branch with its associated strategy.
func (pe *ParallelExecutor) executeBranch(ctx context.Context, branch *ExecutionBranch, input LobeInput, bb *Blackboard) {
	branch.StartedAt = time.Now()
	branch.Status = BranchRunning

	// Find the strategy for this branch
	var strategy *ThinkingStrategy
	for i, s := range pe.branchStrategies {
		if fmt.Sprintf("branch_%d_%s", i, s.Name) == branch.ID {
			strategy = &pe.branchStrategies[i]
			break
		}
	}

	if strategy == nil {
		branch.Status = BranchFailed
		branch.Error = fmt.Errorf("no strategy found for branch %s", branch.ID)
		branch.Duration = time.Since(branch.StartedAt)
		return
	}

	// Execute using PhaseExecutor if available
	if pe.executor != nil {
		result, err := pe.executor.Execute(ctx, input, strategy)
		if err != nil {
			branch.Status = BranchFailed
			branch.Error = err
			branch.Duration = time.Since(branch.StartedAt)
			return
		}

		// Convert ExecutionResult to LobeResult
		branch.Result = &LobeResult{
			LobeID:     LobeReasoning, // Primary lobe
			Content:    result.FinalContent,
			Confidence: bb.OverallConfidence,
			Meta: LobeMeta{
				StartedAt: branch.StartedAt,
				Duration:  result.TotalTime,
			},
		}
		branch.Confidence = bb.OverallConfidence
		branch.Status = BranchCompleted
		branch.Duration = time.Since(branch.StartedAt)
		return
	}

	// Fallback: Create a simple result if no executor
	branch.Result = &LobeResult{
		LobeID:     LobeReasoning,
		Content:    "Branch executed without phase executor",
		Confidence: 0.5,
		Meta: LobeMeta{
			StartedAt: branch.StartedAt,
			Duration:  time.Since(branch.StartedAt),
		},
	}
	branch.Confidence = 0.5
	branch.Status = BranchCompleted
	branch.Duration = time.Since(branch.StartedAt)
}

// aggregate combines branch results via confidence-based selection.
// This implements a simplified consensus: select the highest confidence branch.
//
// Brain Alignment: Like the brain's decision-making process where the strongest
// signal (highest confidence) wins out over weaker alternatives.
func (pe *ParallelExecutor) aggregate(branches <-chan *ExecutionBranch) (*ParallelExecutionResult, error) {
	var best *ExecutionBranch
	var branchCount int

	for b := range branches {
		// Skip branches that failed or didn't complete
		if b.Status != BranchCompleted {
			continue
		}

		// Skip branches below the prune threshold
		if b.Confidence < pe.pruneThreshold {
			continue
		}

		branchCount++

		// Select the branch with highest confidence
		if best == nil || b.Confidence > best.Confidence {
			best = b
		}
	}

	if best == nil {
		return nil, ErrNoBranchesCompleted
	}

	result := best.ToResult()
	result.BranchCount = branchCount
	return result, nil
}
