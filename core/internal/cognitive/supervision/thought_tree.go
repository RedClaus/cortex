package supervision

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ThoughtTreeBuilder constructs and expands thought trees.
type ThoughtTreeBuilder struct {
	config   *Config
	guardian Guardian
	expander Expander
	selector Selector

	mu sync.Mutex
}

// NewThoughtTreeBuilder creates a new tree builder.
func NewThoughtTreeBuilder(cfg *Config, guardian Guardian, expander Expander, selector Selector) *ThoughtTreeBuilder {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	return &ThoughtTreeBuilder{
		config:   cfg,
		guardian: guardian,
		expander: expander,
		selector: selector,
	}
}

// Build constructs a thought tree for the given query.
func (b *ThoughtTreeBuilder) Build(ctx context.Context, query string, initialThought string) (*ThoughtTree, error) {
	startTime := time.Now()

	tree := &ThoughtTree{
		Root: &ThoughtNode{
			ID:        "node_" + uuid.New().String()[:8],
			Depth:     0,
			Content:   initialThought,
			Action:    ActionThink,
			State:     StateComplete,
			Score:     1.0,
			Confidence: 1.0,
			CreatedAt: time.Now(),
			Children:  make([]*ThoughtNode, 0),
		},
		NodeCount: 1,
		Metadata:  make(map[string]string),
	}
	tree.Metadata["query"] = query

	// Expand the tree
	if err := b.expand(ctx, tree, tree.Root, query); err != nil {
		tree.Duration = time.Since(startTime)
		return tree, err
	}

	// Select best path
	if b.selector != nil {
		if best := b.selector.SelectBest(tree); best != nil {
			tree.BestPath = best.Path
		}
	}

	tree.Duration = time.Since(startTime)
	return tree, nil
}

// expand recursively expands nodes in the tree.
func (b *ThoughtTreeBuilder) expand(ctx context.Context, tree *ThoughtTree, node *ThoughtNode, query string) error {
	// Check context cancellation
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Check node limit
	b.mu.Lock()
	if tree.NodeCount >= b.config.MaxNodes {
		b.mu.Unlock()
		return nil
	}
	b.mu.Unlock()

	// Check depth limit
	if node.Depth >= b.config.MaxDepth {
		return nil
	}

	// Check if node can be expanded
	if !node.CanExpand(b.config.MaxDepth) {
		return nil
	}

	// Evaluate node with guardian
	if b.guardian != nil {
		guardCtx, cancel := context.WithTimeout(ctx, b.config.GuardianTimeout)
		result, err := b.guardian.Evaluate(guardCtx, node, tree)
		cancel()

		if err != nil {
			node.Error = err.Error()
			return nil
		}

		node.GuardianResult = result
		node.Score = result.Score

		if !result.Approved {
			node.State = StatePruned
			b.mu.Lock()
			tree.PrunedNodes++
			b.mu.Unlock()
			return nil
		}
	}

	// Expand node to get children
	if b.expander == nil {
		return nil
	}

	expandCtx, cancel := context.WithTimeout(ctx, b.config.NodeTimeout)
	defer cancel()

	req := &ExpansionRequest{
		Node:        node,
		Query:       query,
		Context:     b.buildContext(node),
		MaxBranches: b.config.MaxBranches,
	}

	result, err := b.expander.Expand(expandCtx, req)
	if err != nil {
		node.Error = err.Error()
		return nil
	}

	// Add children to node
	for _, child := range result.Branches {
		// Check node limit before adding
		b.mu.Lock()
		if tree.NodeCount >= b.config.MaxNodes {
			b.mu.Unlock()
			break
		}
		tree.NodeCount++
		if child.Depth > tree.MaxDepth {
			tree.MaxDepth = child.Depth
		}
		b.mu.Unlock()

		child.Parent = node
		child.Depth = node.Depth + 1
		if child.ID == "" {
			child.ID = "node_" + uuid.New().String()[:8]
		}
		child.CreatedAt = time.Now()

		node.Children = append(node.Children, child)

		// Recursively expand child
		if err := b.expand(ctx, tree, child, query); err != nil {
			return err
		}
	}

	return nil
}

// buildContext creates context string for expansion.
func (b *ThoughtTreeBuilder) buildContext(node *ThoughtNode) string {
	path := node.GetPath()
	var context string

	for i, n := range path {
		if i > 0 {
			context += "\n"
		}
		context += fmt.Sprintf("Step %d: %s", i+1, n.Content)
		if n.ToolName != "" {
			context += fmt.Sprintf(" [Tool: %s]", n.ToolName)
		}
	}

	return context
}

// Prune removes nodes below the threshold score.
func (b *ThoughtTreeBuilder) Prune(tree *ThoughtTree, threshold float64) int {
	if threshold <= 0 {
		threshold = b.config.PruneThreshold
	}

	pruned := 0
	for _, node := range tree.GetAllNodes() {
		if node.State != StatePruned && node.Score < threshold {
			node.State = StatePruned
			pruned++
		}
	}

	tree.PrunedNodes += pruned
	return pruned
}

// SimpleExpander is a basic expander for testing.
// In production, this would be replaced by an LLM-based expander.
type SimpleExpander struct {
	branchGenerator func(node *ThoughtNode, query string) []*ThoughtNode
}

// NewSimpleExpander creates a simple expander with a custom generator.
func NewSimpleExpander(generator func(node *ThoughtNode, query string) []*ThoughtNode) *SimpleExpander {
	return &SimpleExpander{branchGenerator: generator}
}

// Expand generates child branches for a node.
func (e *SimpleExpander) Expand(ctx context.Context, req *ExpansionRequest) (*ExpansionResult, error) {
	startTime := time.Now()

	if e.branchGenerator == nil {
		return &ExpansionResult{
			Branches: nil,
			Duration: time.Since(startTime),
		}, nil
	}

	branches := e.branchGenerator(req.Node, req.Query)

	// Limit to max branches
	if len(branches) > req.MaxBranches {
		branches = branches[:req.MaxBranches]
	}

	return &ExpansionResult{
		Branches: branches,
		Duration: time.Since(startTime),
	}, nil
}

// GetStats returns statistics about the tree.
func (t *ThoughtTree) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"node_count":    t.NodeCount,
		"max_depth":     t.MaxDepth,
		"pruned_nodes":  t.PrunedNodes,
		"duration_ms":   t.Duration.Milliseconds(),
	}

	// Count nodes by state
	stateCount := make(map[NodeState]int)
	actionCount := make(map[NodeAction]int)

	for _, node := range t.GetAllNodes() {
		stateCount[node.State]++
		actionCount[node.Action]++
	}

	stats["states"] = stateCount
	stats["actions"] = actionCount

	// Calculate average score
	var totalScore float64
	var scoreCount int
	for _, node := range t.GetAllNodes() {
		if node.State == StateComplete {
			totalScore += node.Score
			scoreCount++
		}
	}
	if scoreCount > 0 {
		stats["avg_score"] = totalScore / float64(scoreCount)
	}

	return stats
}
