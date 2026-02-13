package supervision

import (
	"context"
	"sync"
	"time"
)

// Coordinator manages thought supervision for System 3 meta-cognition.
// It provides the main entry point for process-supervised reasoning.
type Coordinator struct {
	config   *Config
	builder  *ThoughtTreeBuilder
	guardian Guardian
	selector Selector

	// Active trees by request ID
	activeTrees sync.Map // map[string]*ThoughtTree

	mu sync.RWMutex
}

// NewCoordinator creates a new supervision coordinator.
func NewCoordinator(cfg *Config) *Coordinator {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	guardian := NewInhibitionGuardian(cfg.GuardianTimeout)
	selector := NewRewardSelector()

	// Create builder with nil expander (will be set per-request)
	builder := NewThoughtTreeBuilder(cfg, guardian, nil, selector)

	return &Coordinator{
		config:   cfg,
		builder:  builder,
		guardian: guardian,
		selector: selector,
	}
}

// Enabled returns whether supervision is enabled.
func (c *Coordinator) Enabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config.Enabled
}

// SetEnabled enables or disables supervision.
func (c *Coordinator) SetEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config.Enabled = enabled
}

// Config returns the current configuration.
func (c *Coordinator) Config() *Config {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config
}

// EvaluateThought evaluates a single thought using the guardian.
// This is useful for quick validation without building a full tree.
func (c *Coordinator) EvaluateThought(ctx context.Context, content string) (*GuardianResult, error) {
	if !c.Enabled() {
		return &GuardianResult{
			Approved:   true,
			Score:      1.0,
			Confidence: 1.0,
			Reason:     "Supervision disabled",
		}, nil
	}

	node := &ThoughtNode{
		ID:        "eval_node",
		Content:   content,
		Action:    ActionThink,
		State:     StateComplete,
		CreatedAt: time.Now(),
	}

	return c.guardian.Evaluate(ctx, node, nil)
}

// BuildTree constructs a thought tree for supervised reasoning.
func (c *Coordinator) BuildTree(ctx context.Context, requestID, query, initialThought string, expander Expander) (*ThoughtTree, error) {
	if !c.Enabled() {
		// Return a simple single-node tree
		return &ThoughtTree{
			Root: &ThoughtNode{
				ID:        "root",
				Content:   initialThought,
				Action:    ActionThink,
				State:     StateComplete,
				Score:     1.0,
				CreatedAt: time.Now(),
			},
			NodeCount: 1,
			Metadata:  map[string]string{"query": query},
		}, nil
	}

	// Create builder with the provided expander
	builder := NewThoughtTreeBuilder(c.config, c.guardian, expander, c.selector)

	tree, err := builder.Build(ctx, query, initialThought)
	if err != nil {
		return nil, err
	}

	// Store active tree
	c.activeTrees.Store(requestID, tree)

	return tree, nil
}

// GetActiveTree retrieves an active tree by request ID.
func (c *Coordinator) GetActiveTree(requestID string) *ThoughtTree {
	if val, ok := c.activeTrees.Load(requestID); ok {
		if tree, ok := val.(*ThoughtTree); ok {
			return tree
		}
	}
	return nil
}

// CompleteTree marks a tree as complete and removes it from active trees.
func (c *Coordinator) CompleteTree(requestID string) *ThoughtTree {
	if val, ok := c.activeTrees.LoadAndDelete(requestID); ok {
		if tree, ok := val.(*ThoughtTree); ok {
			return tree
		}
	}
	return nil
}

// SelectBestPath selects the best reasoning path from a tree.
func (c *Coordinator) SelectBestPath(tree *ThoughtTree) *PathScore {
	if tree == nil || c.selector == nil {
		return nil
	}
	return c.selector.SelectBest(tree)
}

// RankPaths ranks all complete paths in a tree.
func (c *Coordinator) RankPaths(tree *ThoughtTree) []*PathScore {
	if tree == nil || c.selector == nil {
		return nil
	}
	return c.selector.RankPaths(tree)
}

// PruneTree removes low-scoring nodes from a tree.
func (c *Coordinator) PruneTree(tree *ThoughtTree) int {
	if tree == nil {
		return 0
	}
	return c.builder.Prune(tree, c.config.PruneThreshold)
}

// GetStats returns statistics about active supervision.
func (c *Coordinator) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"enabled":        c.Enabled(),
		"max_branches":   c.config.MaxBranches,
		"max_depth":      c.config.MaxDepth,
		"max_nodes":      c.config.MaxNodes,
		"prune_threshold": c.config.PruneThreshold,
	}

	// Count active trees
	activeCount := 0
	c.activeTrees.Range(func(key, value interface{}) bool {
		activeCount++
		return true
	})
	stats["active_trees"] = activeCount

	return stats
}

// SupervisedStep represents a single supervised reasoning step.
type SupervisedStep struct {
	Content     string          `json:"content"`
	Action      NodeAction      `json:"action"`
	ToolName    string          `json:"tool_name,omitempty"`
	ToolInput   string          `json:"tool_input,omitempty"`
	ToolOutput  string          `json:"tool_output,omitempty"`
	Evaluation  *GuardianResult `json:"evaluation"`
	ShouldRetry bool            `json:"should_retry"`
	Alternatives []string       `json:"alternatives,omitempty"`
}

// SuperviseStep evaluates a reasoning step and provides feedback.
// This is the main integration point for step-by-step supervision.
func (c *Coordinator) SuperviseStep(ctx context.Context, step *SupervisedStep) (*SupervisedStep, error) {
	if !c.Enabled() {
		step.Evaluation = &GuardianResult{
			Approved:   true,
			Score:      1.0,
			Confidence: 1.0,
			Reason:     "Supervision disabled",
		}
		return step, nil
	}

	node := &ThoughtNode{
		ID:         "step_node",
		Content:    step.Content,
		Action:     step.Action,
		ToolName:   step.ToolName,
		ToolInput:  step.ToolInput,
		ToolOutput: step.ToolOutput,
		State:      StateComplete,
		CreatedAt:  time.Now(),
	}

	result, err := c.guardian.Evaluate(ctx, node, nil)
	if err != nil {
		return nil, err
	}

	step.Evaluation = result
	step.ShouldRetry = !result.Approved && result.Score >= 0.2 // Retry if close to threshold

	// Generate alternatives if rejected
	if !result.Approved && len(result.Suggestions) > 0 {
		step.Alternatives = result.Suggestions
	}

	return step, nil
}

// BatchSupervise evaluates multiple steps in parallel.
func (c *Coordinator) BatchSupervise(ctx context.Context, steps []*SupervisedStep) ([]*SupervisedStep, error) {
	if !c.Enabled() {
		for _, step := range steps {
			step.Evaluation = &GuardianResult{
				Approved:   true,
				Score:      1.0,
				Confidence: 1.0,
				Reason:     "Supervision disabled",
			}
		}
		return steps, nil
	}

	var wg sync.WaitGroup
	results := make([]*SupervisedStep, len(steps))
	errors := make([]error, len(steps))

	for i, step := range steps {
		wg.Add(1)
		go func(idx int, s *SupervisedStep) {
			defer wg.Done()
			result, err := c.SuperviseStep(ctx, s)
			results[idx] = result
			errors[idx] = err
		}(i, step)
	}

	wg.Wait()

	// Return first error if any
	for _, err := range errors {
		if err != nil {
			return results, err
		}
	}

	return results, nil
}
