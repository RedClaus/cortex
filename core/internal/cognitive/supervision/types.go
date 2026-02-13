// Package supervision provides process-supervised thought search for System 3 meta-cognition.
// It implements Tree-of-Thought expansion with guardian critique, enabling improved reasoning
// quality through branch exploration and pruning (per Sophia paper arXiv:2512.18202).
package supervision

import (
	"context"
	"time"
)

// Config configures the thought supervision system.
type Config struct {
	// MaxBranches is the maximum branches per node (default: 3)
	MaxBranches int

	// MaxDepth is the maximum tree depth (default: 4)
	MaxDepth int

	// MaxNodes is the absolute maximum nodes in the tree (default: 20)
	// This is a safety limit to prevent runaway expansion.
	MaxNodes int

	// PruneThreshold is the minimum score to keep a branch (default: 0.3)
	PruneThreshold float64

	// GuardianTimeout is the max time for guardian critique per node (default: 200ms)
	GuardianTimeout time.Duration

	// NodeTimeout is the max time for node expansion (default: 5s)
	NodeTimeout time.Duration

	// Enabled controls whether supervision is active
	Enabled bool
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		MaxBranches:     3,
		MaxDepth:        4,
		MaxNodes:        20,
		PruneThreshold:  0.3,
		GuardianTimeout: 200 * time.Millisecond,
		NodeTimeout:     5 * time.Second,
		Enabled:         true,
	}
}

// ThoughtTree represents a tree of reasoning paths.
type ThoughtTree struct {
	Root       *ThoughtNode          `json:"root"`
	NodeCount  int                   `json:"node_count"`
	MaxDepth   int                   `json:"max_depth_reached"`
	BestPath   []*ThoughtNode        `json:"best_path"`
	PrunedNodes int                  `json:"pruned_nodes"`
	Duration   time.Duration         `json:"duration"`
	Metadata   map[string]string     `json:"metadata"`
}

// ThoughtNode represents a single node in the thought tree.
type ThoughtNode struct {
	ID          string         `json:"id"`
	Depth       int            `json:"depth"`
	Content     string         `json:"content"`      // The thought/reasoning at this node
	Action      NodeAction     `json:"action"`       // Type of action
	ToolName    string         `json:"tool_name"`    // Tool called (if any)
	ToolInput   string         `json:"tool_input"`   // Tool input (if any)
	ToolOutput  string         `json:"tool_output"`  // Tool output (if any)

	// Scoring
	Score       float64        `json:"score"`        // Combined score (0-1)
	Confidence  float64        `json:"confidence"`   // Node confidence (0-1)

	// Guardian evaluation
	GuardianResult *GuardianResult `json:"guardian_result"`

	// Tree structure
	Parent      *ThoughtNode   `json:"-"`            // Parent node (nil for root)
	Children    []*ThoughtNode `json:"children"`     // Child branches

	// State
	State       NodeState      `json:"state"`
	Error       string         `json:"error,omitempty"`

	// Timing
	CreatedAt   time.Time      `json:"created_at"`
	Duration    time.Duration  `json:"duration"`
}

// NodeAction represents the type of action at a node.
type NodeAction string

const (
	ActionThink      NodeAction = "think"       // Reasoning step
	ActionToolCall   NodeAction = "tool_call"   // Tool invocation
	ActionToolResult NodeAction = "tool_result" // Tool result processing
	ActionConclude   NodeAction = "conclude"    // Final conclusion
	ActionReplan     NodeAction = "replan"      // Replanning step
)

// NodeState represents the state of a node.
type NodeState string

const (
	StatePending   NodeState = "pending"   // Not yet processed
	StateExpanding NodeState = "expanding" // Being expanded
	StateComplete  NodeState = "complete"  // Successfully completed
	StatePruned    NodeState = "pruned"    // Pruned by guardian
	StateFailed    NodeState = "failed"    // Failed to process
	StateTimeout   NodeState = "timeout"   // Timed out
)

// GuardianResult represents the evaluation by the guardian.
type GuardianResult struct {
	Approved      bool          `json:"approved"`
	Score         float64       `json:"score"`          // 0-1 quality score
	Confidence    float64       `json:"confidence"`     // Guardian's confidence
	Reason        string        `json:"reason"`         // Explanation
	RiskFactors   []string      `json:"risk_factors"`   // Identified risks
	Suggestions   []string      `json:"suggestions"`    // Improvement suggestions
	Duration      time.Duration `json:"duration"`
}

// RewardSignal represents a reward/penalty signal for a node.
type RewardSignal struct {
	Source    string  `json:"source"`    // What generated this signal
	Value     float64 `json:"value"`     // -1 to 1 (negative = penalty)
	Weight    float64 `json:"weight"`    // Importance weight
	Reason    string  `json:"reason"`    // Explanation
}

// PathScore represents the score for a complete path through the tree.
type PathScore struct {
	Path          []*ThoughtNode `json:"path"`
	TotalScore    float64        `json:"total_score"`
	AvgScore      float64        `json:"avg_score"`
	MinScore      float64        `json:"min_score"`
	Depth         int            `json:"depth"`
	ToolsUsed     []string       `json:"tools_used"`
	HasConclusion bool           `json:"has_conclusion"`
}

// ExpansionRequest represents a request to expand a node.
type ExpansionRequest struct {
	Node       *ThoughtNode
	Query      string
	Context    string
	MaxBranches int
}

// ExpansionResult represents the result of expanding a node.
type ExpansionResult struct {
	Branches   []*ThoughtNode
	Error      error
	Duration   time.Duration
}

// Guardian defines the interface for thought evaluation.
type Guardian interface {
	// Evaluate assesses a thought node and returns approval/rejection.
	Evaluate(ctx context.Context, node *ThoughtNode, tree *ThoughtTree) (*GuardianResult, error)
}

// Expander defines the interface for generating new thought branches.
type Expander interface {
	// Expand generates child branches for a node.
	Expand(ctx context.Context, req *ExpansionRequest) (*ExpansionResult, error)
}

// Selector defines the interface for selecting the best path.
type Selector interface {
	// SelectBest chooses the best path through the tree.
	SelectBest(tree *ThoughtTree) *PathScore

	// RankPaths ranks all complete paths in the tree.
	RankPaths(tree *ThoughtTree) []*PathScore
}

// GetAllNodes returns all nodes in the tree via BFS.
func (t *ThoughtTree) GetAllNodes() []*ThoughtNode {
	if t.Root == nil {
		return nil
	}

	var nodes []*ThoughtNode
	queue := []*ThoughtNode{t.Root}

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		nodes = append(nodes, node)
		queue = append(queue, node.Children...)
	}

	return nodes
}

// GetLeafNodes returns all leaf nodes (nodes with no children).
func (t *ThoughtTree) GetLeafNodes() []*ThoughtNode {
	var leaves []*ThoughtNode
	for _, node := range t.GetAllNodes() {
		if len(node.Children) == 0 && node.State != StatePruned {
			leaves = append(leaves, node)
		}
	}
	return leaves
}

// GetPath returns the path from root to the given node.
func (n *ThoughtNode) GetPath() []*ThoughtNode {
	var path []*ThoughtNode
	current := n
	for current != nil {
		path = append([]*ThoughtNode{current}, path...)
		current = current.Parent
	}
	return path
}

// IsTerminal returns true if this node is a terminal node.
func (n *ThoughtNode) IsTerminal() bool {
	return n.Action == ActionConclude || n.State == StatePruned || n.State == StateFailed
}

// CanExpand returns true if this node can be expanded.
func (n *ThoughtNode) CanExpand(maxDepth int) bool {
	return n.State == StateComplete &&
		   !n.IsTerminal() &&
		   n.Depth < maxDepth &&
		   (n.GuardianResult == nil || n.GuardianResult.Approved)
}
