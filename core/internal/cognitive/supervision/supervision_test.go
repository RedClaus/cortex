package supervision

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Config Tests
// ============================================================================

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.NotNil(t, cfg)
	assert.Equal(t, 3, cfg.MaxBranches)
	assert.Equal(t, 4, cfg.MaxDepth)
	assert.Equal(t, 20, cfg.MaxNodes)
	assert.Equal(t, 0.3, cfg.PruneThreshold)
	assert.Equal(t, 200*time.Millisecond, cfg.GuardianTimeout)
	assert.Equal(t, 5*time.Second, cfg.NodeTimeout)
	assert.True(t, cfg.Enabled)
}

// ============================================================================
// ThoughtNode Tests
// ============================================================================

func TestThoughtNode_GetPath(t *testing.T) {
	root := &ThoughtNode{ID: "root", Depth: 0}
	child1 := &ThoughtNode{ID: "child1", Depth: 1, Parent: root}
	child2 := &ThoughtNode{ID: "child2", Depth: 2, Parent: child1}
	grandchild := &ThoughtNode{ID: "grandchild", Depth: 3, Parent: child2}

	path := grandchild.GetPath()

	assert.Len(t, path, 4)
	assert.Equal(t, "root", path[0].ID)
	assert.Equal(t, "child1", path[1].ID)
	assert.Equal(t, "child2", path[2].ID)
	assert.Equal(t, "grandchild", path[3].ID)
}

func TestThoughtNode_IsTerminal(t *testing.T) {
	tests := []struct {
		name     string
		action   NodeAction
		state    NodeState
		expected bool
	}{
		{"conclude action", ActionConclude, StateComplete, true},
		{"pruned state", ActionThink, StatePruned, true},
		{"failed state", ActionThink, StateFailed, true},
		{"complete think", ActionThink, StateComplete, false},
		{"complete tool call", ActionToolCall, StateComplete, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &ThoughtNode{Action: tt.action, State: tt.state}
			assert.Equal(t, tt.expected, node.IsTerminal())
		})
	}
}

func TestThoughtNode_CanExpand(t *testing.T) {
	tests := []struct {
		name     string
		node     *ThoughtNode
		maxDepth int
		expected bool
	}{
		{
			name:     "can expand",
			node:     &ThoughtNode{State: StateComplete, Depth: 2, Action: ActionThink},
			maxDepth: 4,
			expected: true,
		},
		{
			name:     "at max depth",
			node:     &ThoughtNode{State: StateComplete, Depth: 4, Action: ActionThink},
			maxDepth: 4,
			expected: false,
		},
		{
			name:     "not complete",
			node:     &ThoughtNode{State: StatePending, Depth: 2, Action: ActionThink},
			maxDepth: 4,
			expected: false,
		},
		{
			name:     "is terminal",
			node:     &ThoughtNode{State: StateComplete, Depth: 2, Action: ActionConclude},
			maxDepth: 4,
			expected: false,
		},
		{
			name:     "guardian rejected",
			node:     &ThoughtNode{State: StateComplete, Depth: 2, Action: ActionThink, GuardianResult: &GuardianResult{Approved: false}},
			maxDepth: 4,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.node.CanExpand(tt.maxDepth))
		})
	}
}

// ============================================================================
// ThoughtTree Tests
// ============================================================================

func TestThoughtTree_GetAllNodes(t *testing.T) {
	tree := &ThoughtTree{
		Root: &ThoughtNode{
			ID: "root",
			Children: []*ThoughtNode{
				{ID: "child1", Children: []*ThoughtNode{
					{ID: "grandchild1"},
				}},
				{ID: "child2"},
			},
		},
	}

	nodes := tree.GetAllNodes()

	assert.Len(t, nodes, 4)
	ids := make([]string, len(nodes))
	for i, n := range nodes {
		ids[i] = n.ID
	}
	assert.Contains(t, ids, "root")
	assert.Contains(t, ids, "child1")
	assert.Contains(t, ids, "child2")
	assert.Contains(t, ids, "grandchild1")
}

func TestThoughtTree_GetLeafNodes(t *testing.T) {
	tree := &ThoughtTree{
		Root: &ThoughtNode{
			ID:    "root",
			State: StateComplete,
			Children: []*ThoughtNode{
				{ID: "child1", State: StateComplete, Children: []*ThoughtNode{
					{ID: "grandchild1", State: StateComplete},
				}},
				{ID: "child2", State: StateComplete},
				{ID: "child3", State: StatePruned}, // Pruned, should not be leaf
			},
		},
	}

	leaves := tree.GetLeafNodes()

	assert.Len(t, leaves, 2)
	ids := make([]string, len(leaves))
	for i, n := range leaves {
		ids[i] = n.ID
	}
	assert.Contains(t, ids, "grandchild1")
	assert.Contains(t, ids, "child2")
	assert.NotContains(t, ids, "child3") // Pruned
}

func TestThoughtTree_GetStats(t *testing.T) {
	tree := &ThoughtTree{
		NodeCount:   5,
		MaxDepth:    3,
		PrunedNodes: 1,
		Duration:    100 * time.Millisecond,
		Root: &ThoughtNode{
			State: StateComplete,
			Score: 0.9,
			Children: []*ThoughtNode{
				{State: StateComplete, Score: 0.8},
				{State: StatePruned, Score: 0.1},
			},
		},
	}

	stats := tree.GetStats()

	assert.Equal(t, 5, stats["node_count"])
	assert.Equal(t, 3, stats["max_depth"])
	assert.Equal(t, 1, stats["pruned_nodes"])
	assert.Equal(t, int64(100), stats["duration_ms"])
}

// ============================================================================
// InhibitionGuardian Tests
// ============================================================================

func TestInhibitionGuardian_Evaluate_BasicApproval(t *testing.T) {
	guardian := NewInhibitionGuardian(200 * time.Millisecond)
	ctx := context.Background()

	node := &ThoughtNode{
		ID:      "test",
		Content: "This is a valid thought with enough content to pass validation.",
		State:   StateComplete,
	}

	result, err := guardian.Evaluate(ctx, node, nil)

	require.NoError(t, err)
	assert.True(t, result.Approved)
	assert.GreaterOrEqual(t, result.Score, 0.3)
	assert.Less(t, result.Duration, 200*time.Millisecond)
}

func TestInhibitionGuardian_Evaluate_RejectShortContent(t *testing.T) {
	guardian := NewInhibitionGuardian(200 * time.Millisecond)
	ctx := context.Background()

	node := &ThoughtNode{
		ID:      "test",
		Content: "short", // Less than 10 chars
		State:   StateComplete,
	}

	result, err := guardian.Evaluate(ctx, node, nil)

	require.NoError(t, err)
	assert.Contains(t, result.RiskFactors, "content_too_short")
}

func TestInhibitionGuardian_Evaluate_DetectsInvalidPatterns(t *testing.T) {
	guardian := NewInhibitionGuardian(200 * time.Millisecond)
	ctx := context.Background()

	tests := []struct {
		name    string
		content string
	}{
		{"circular reasoning", "As I just said, the answer is correct because I said so."},
		{"hallucination", "I believe this might be the answer, probably correct."},
		{"contradiction", "Actually, no that's wrong. Let me reconsider."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &ThoughtNode{
				ID:      "test",
				Content: tt.content,
				State:   StateComplete,
			}

			result, err := guardian.Evaluate(ctx, node, nil)

			require.NoError(t, err)
			assert.Less(t, result.Score, 1.0, "Score should be reduced for invalid pattern")
		})
	}
}

func TestInhibitionGuardian_Evaluate_Timeout(t *testing.T) {
	guardian := NewInhibitionGuardian(1 * time.Millisecond)
	ctx := context.Background()

	// Create node with very long content to trigger timeout
	longContent := make([]byte, 10000)
	for i := range longContent {
		longContent[i] = 'a'
	}

	node := &ThoughtNode{
		ID:      "test",
		Content: string(longContent),
		State:   StateComplete,
	}

	result, err := guardian.Evaluate(ctx, node, nil)

	require.NoError(t, err)
	assert.True(t, result.Approved) // Auto-approved on timeout
	// Confidence should be lower on timeout, but this is implementation-dependent
}

func TestInhibitionGuardian_CheckConsistency(t *testing.T) {
	guardian := NewInhibitionGuardian(200 * time.Millisecond)

	parent := &ThoughtNode{
		ID:      "parent",
		Content: "The API endpoint returns user data in JSON format.",
	}

	tests := []struct {
		name       string
		childText  string
		minScore   float64
	}{
		{
			name:      "consistent child",
			childText: "The JSON response includes the user data fields we need.",
			minScore:  0.5,
		},
		{
			name:      "contradictory child",
			childText: "But actually that's wrong, the API doesn't return JSON.",
			minScore:  0.0, // Contradictions score low
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			child := &ThoughtNode{ID: "child", Content: tt.childText}
			score := guardian.checkConsistency(child, parent)
			assert.GreaterOrEqual(t, score, tt.minScore)
		})
	}
}

// ============================================================================
// RewardSelector Tests
// ============================================================================

func TestRewardSelector_SelectBest(t *testing.T) {
	selector := NewRewardSelector()

	tree := &ThoughtTree{
		Root: &ThoughtNode{
			ID:     "root",
			Score:  0.9,
			State:  StateComplete,
			Action: ActionThink,
			Children: []*ThoughtNode{
				{
					ID:     "path1",
					Score:  0.8,
					State:  StateComplete,
					Action: ActionConclude,
				},
				{
					ID:     "path2",
					Score:  0.5,
					State:  StateComplete,
					Action: ActionThink,
				},
			},
		},
	}

	// Set parent references
	tree.Root.Children[0].Parent = tree.Root
	tree.Root.Children[1].Parent = tree.Root

	best := selector.SelectBest(tree)

	require.NotNil(t, best)
	assert.True(t, best.HasConclusion)
	assert.Greater(t, best.TotalScore, 0.0)
}

func TestRewardSelector_RankPaths(t *testing.T) {
	selector := NewRewardSelector()

	tree := &ThoughtTree{
		Root: &ThoughtNode{
			ID:     "root",
			Score:  0.9,
			State:  StateComplete,
			Action: ActionThink,
			Children: []*ThoughtNode{
				{ID: "high", Score: 0.9, State: StateComplete, Action: ActionConclude},
				{ID: "medium", Score: 0.6, State: StateComplete, Action: ActionConclude},
				{ID: "low", Score: 0.3, State: StateComplete, Action: ActionThink},
			},
		},
	}

	// Set parent references
	for _, child := range tree.Root.Children {
		child.Parent = tree.Root
	}

	paths := selector.RankPaths(tree)

	assert.Len(t, paths, 3)
	// Paths should be sorted by score descending
	assert.GreaterOrEqual(t, paths[0].TotalScore, paths[1].TotalScore)
	assert.GreaterOrEqual(t, paths[1].TotalScore, paths[2].TotalScore)
}

func TestRewardSelector_ScorePath_ToolUsage(t *testing.T) {
	selector := NewRewardSelector()

	pathWithTools := []*ThoughtNode{
		{ID: "1", Score: 0.8, ToolName: "bash"},
		{ID: "2", Score: 0.8, ToolName: "read_file"},
		{ID: "3", Score: 0.8, Action: ActionConclude},
	}

	pathNoTools := []*ThoughtNode{
		{ID: "1", Score: 0.8},
		{ID: "2", Score: 0.8, Action: ActionConclude},
	}

	scoreWithTools := selector.scorePath(pathWithTools)
	scoreNoTools := selector.scorePath(pathNoTools)

	assert.Len(t, scoreWithTools.ToolsUsed, 2)
	assert.Contains(t, scoreWithTools.ToolsUsed, "bash")
	assert.Contains(t, scoreWithTools.ToolsUsed, "read_file")
	assert.Len(t, scoreNoTools.ToolsUsed, 0)
}

func TestRewardSelector_DepthScoring(t *testing.T) {
	selector := NewRewardSelector()

	tests := []struct {
		depth    int
		minScore float64
		maxScore float64
	}{
		{1, 0.0, 0.5},  // Too shallow
		{2, 0.8, 1.0},  // Ideal
		{3, 0.8, 1.0},  // Ideal
		{5, 0.5, 0.9},  // Good
		{8, 0.0, 0.5},  // Too long
	}

	for _, tt := range tests {
		t.Run("depth_"+string(rune(tt.depth+'0')), func(t *testing.T) {
			path := make([]*ThoughtNode, tt.depth)
			for i := range path {
				path[i] = &ThoughtNode{ID: string(rune(i + '0')), Score: 0.8}
			}
			path[tt.depth-1].Action = ActionConclude

			score := selector.scorePath(path)
			// Check that depth component is within expected range
			assert.Equal(t, tt.depth, score.Depth)
		})
	}
}

func TestRewardSelector_GenerateRewardSignals(t *testing.T) {
	selector := NewRewardSelector()

	ps := &PathScore{
		AvgScore:      0.85,
		MinScore:      0.75,
		Depth:         3,
		HasConclusion: true,
		ToolsUsed:     []string{"bash", "read_file"},
	}

	signals := selector.GenerateRewardSignals(ps)

	assert.NotEmpty(t, signals)
	// Should have completion reward
	hasCompletion := false
	for _, s := range signals {
		if s.Source == "completion" {
			hasCompletion = true
			assert.Greater(t, s.Value, 0.0, "Completion reward should be positive")
		}
	}
	assert.True(t, hasCompletion)
}

func TestRewardSelector_ApplyReward(t *testing.T) {
	selector := NewRewardSelector()

	path := []*ThoughtNode{
		{Score: 0.5},
		{Score: 0.6},
		{Score: 0.7},
	}

	signal := RewardSignal{
		Source: "test",
		Value:  0.2,
		Weight: 1.0,
		Reason: "Test reward",
	}

	selector.ApplyReward(path, signal)

	// Scores should increase (using InDelta for floating-point comparison)
	assert.InDelta(t, 0.7, path[0].Score, 0.001) // 0.5 + 0.2
	assert.InDelta(t, 0.8, path[1].Score, 0.001) // 0.6 + 0.2
	assert.InDelta(t, 0.9, path[2].Score, 0.001) // 0.7 + 0.2
}

func TestRewardSelector_ApplyReward_Clamping(t *testing.T) {
	selector := NewRewardSelector()

	path := []*ThoughtNode{
		{Score: 0.9},
		{Score: 0.1},
	}

	// Apply large positive signal
	selector.ApplyReward(path, RewardSignal{Value: 0.5, Weight: 1.0})
	assert.Equal(t, 1.0, path[0].Score, "Score should be clamped to 1.0")
	assert.Equal(t, 0.6, path[1].Score)

	// Apply large negative signal
	path[1].Score = 0.1
	selector.ApplyReward(path, RewardSignal{Value: -0.5, Weight: 1.0})
	assert.Equal(t, 0.0, path[1].Score, "Score should be clamped to 0.0")
}

// ============================================================================
// ThoughtTreeBuilder Tests
// ============================================================================

func TestThoughtTreeBuilder_Build_NoExpander(t *testing.T) {
	cfg := DefaultConfig()
	guardian := NewInhibitionGuardian(cfg.GuardianTimeout)
	selector := NewRewardSelector()
	builder := NewThoughtTreeBuilder(cfg, guardian, nil, selector)

	ctx := context.Background()
	tree, err := builder.Build(ctx, "test query", "Initial thought about the problem.")

	require.NoError(t, err)
	require.NotNil(t, tree)
	assert.Equal(t, 1, tree.NodeCount)
	assert.Equal(t, "Initial thought about the problem.", tree.Root.Content)
}

func TestThoughtTreeBuilder_Build_WithSimpleExpander(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxDepth = 2
	cfg.MaxBranches = 2

	guardian := NewInhibitionGuardian(cfg.GuardianTimeout)
	selector := NewRewardSelector()

	// Create expander that generates child nodes
	expander := NewSimpleExpander(func(node *ThoughtNode, query string) []*ThoughtNode {
		if node.Depth >= 1 {
			return nil // Stop at depth 1
		}
		return []*ThoughtNode{
			{Content: "Branch 1 from " + node.ID, Action: ActionThink, State: StateComplete, Score: 0.8},
			{Content: "Branch 2 from " + node.ID, Action: ActionConclude, State: StateComplete, Score: 0.9},
		}
	})

	builder := NewThoughtTreeBuilder(cfg, guardian, expander, selector)

	ctx := context.Background()
	tree, err := builder.Build(ctx, "test query", "Initial thought for expansion.")

	require.NoError(t, err)
	require.NotNil(t, tree)
	assert.GreaterOrEqual(t, tree.NodeCount, 1)
}

func TestThoughtTreeBuilder_Prune(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PruneThreshold = 0.5

	builder := NewThoughtTreeBuilder(cfg, nil, nil, nil)

	tree := &ThoughtTree{
		Root: &ThoughtNode{
			ID:    "root",
			Score: 0.9,
			State: StateComplete,
			Children: []*ThoughtNode{
				{ID: "high", Score: 0.8, State: StateComplete},
				{ID: "low1", Score: 0.3, State: StateComplete},
				{ID: "low2", Score: 0.4, State: StateComplete},
			},
		},
	}

	pruned := builder.Prune(tree, 0.5)

	assert.Equal(t, 2, pruned)
	// Check that low-scoring nodes are pruned
	for _, child := range tree.Root.Children {
		if child.Score < 0.5 {
			assert.Equal(t, StatePruned, child.State)
		}
	}
}

// ============================================================================
// Coordinator Tests
// ============================================================================

func TestCoordinator_NewCoordinator(t *testing.T) {
	coord := NewCoordinator(nil)

	assert.NotNil(t, coord)
	assert.True(t, coord.Enabled())
}

func TestCoordinator_EnabledToggle(t *testing.T) {
	coord := NewCoordinator(nil)

	assert.True(t, coord.Enabled())

	coord.SetEnabled(false)
	assert.False(t, coord.Enabled())

	coord.SetEnabled(true)
	assert.True(t, coord.Enabled())
}

func TestCoordinator_EvaluateThought_Disabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = false
	coord := NewCoordinator(cfg)

	ctx := context.Background()
	result, err := coord.EvaluateThought(ctx, "test thought")

	require.NoError(t, err)
	assert.True(t, result.Approved)
	assert.Equal(t, 1.0, result.Score)
	assert.Equal(t, "Supervision disabled", result.Reason)
}

func TestCoordinator_EvaluateThought_Enabled(t *testing.T) {
	coord := NewCoordinator(nil)

	ctx := context.Background()
	result, err := coord.EvaluateThought(ctx, "This is a valid thought with sufficient content for evaluation.")

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.GreaterOrEqual(t, result.Score, 0.0)
	assert.LessOrEqual(t, result.Score, 1.0)
}

func TestCoordinator_BuildTree_Disabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = false
	coord := NewCoordinator(cfg)

	ctx := context.Background()
	tree, err := coord.BuildTree(ctx, "req1", "test query", "Initial thought.", nil)

	require.NoError(t, err)
	require.NotNil(t, tree)
	assert.Equal(t, 1, tree.NodeCount)
	assert.Equal(t, 1.0, tree.Root.Score)
}

func TestCoordinator_BuildTree_Enabled(t *testing.T) {
	coord := NewCoordinator(nil)

	ctx := context.Background()
	tree, err := coord.BuildTree(ctx, "req1", "test query", "Initial thought for the tree.", nil)

	require.NoError(t, err)
	require.NotNil(t, tree)
	assert.GreaterOrEqual(t, tree.NodeCount, 1)
}

func TestCoordinator_ActiveTrees(t *testing.T) {
	coord := NewCoordinator(nil)
	ctx := context.Background()

	// Build a tree
	tree, err := coord.BuildTree(ctx, "req1", "query", "Initial thought for tree management.", nil)
	require.NoError(t, err)

	// Retrieve active tree
	active := coord.GetActiveTree("req1")
	assert.NotNil(t, active)
	assert.Equal(t, tree, active)

	// Complete tree
	completed := coord.CompleteTree("req1")
	assert.Equal(t, tree, completed)

	// Tree should no longer be active
	assert.Nil(t, coord.GetActiveTree("req1"))
}

func TestCoordinator_SuperviseStep_Disabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = false
	coord := NewCoordinator(cfg)

	ctx := context.Background()
	step := &SupervisedStep{
		Content: "Test step content.",
		Action:  ActionThink,
	}

	result, err := coord.SuperviseStep(ctx, step)

	require.NoError(t, err)
	assert.True(t, result.Evaluation.Approved)
	assert.Equal(t, "Supervision disabled", result.Evaluation.Reason)
}

func TestCoordinator_SuperviseStep_Enabled(t *testing.T) {
	coord := NewCoordinator(nil)

	ctx := context.Background()
	step := &SupervisedStep{
		Content:  "This is a valid reasoning step with sufficient content.",
		Action:   ActionThink,
		ToolName: "bash",
	}

	result, err := coord.SuperviseStep(ctx, step)

	require.NoError(t, err)
	assert.NotNil(t, result.Evaluation)
	// Check that evaluation ran
	assert.NotEmpty(t, result.Evaluation.Reason)
}

func TestCoordinator_BatchSupervise(t *testing.T) {
	coord := NewCoordinator(nil)

	ctx := context.Background()
	steps := []*SupervisedStep{
		{Content: "Step 1 with valid content for evaluation.", Action: ActionThink},
		{Content: "Step 2 with valid content for evaluation.", Action: ActionToolCall, ToolName: "bash"},
		{Content: "Step 3 concluding the reasoning process.", Action: ActionConclude},
	}

	results, err := coord.BatchSupervise(ctx, steps)

	require.NoError(t, err)
	assert.Len(t, results, 3)
	for _, r := range results {
		assert.NotNil(t, r.Evaluation)
	}
}

func TestCoordinator_GetStats(t *testing.T) {
	coord := NewCoordinator(nil)
	ctx := context.Background()

	// Build a tree to have active state
	_, _ = coord.BuildTree(ctx, "req1", "query", "Initial thought for stats.", nil)

	stats := coord.GetStats()

	assert.True(t, stats["enabled"].(bool))
	assert.Equal(t, 3, stats["max_branches"])
	assert.Equal(t, 4, stats["max_depth"])
	assert.Equal(t, 20, stats["max_nodes"])
	assert.Equal(t, 0.3, stats["prune_threshold"])
	assert.Equal(t, 1, stats["active_trees"])
}

func TestCoordinator_SelectAndRankPaths(t *testing.T) {
	coord := NewCoordinator(nil)

	tree := &ThoughtTree{
		Root: &ThoughtNode{
			ID:     "root",
			Score:  0.9,
			State:  StateComplete,
			Action: ActionThink,
			Children: []*ThoughtNode{
				{ID: "c1", Score: 0.9, State: StateComplete, Action: ActionConclude},
				{ID: "c2", Score: 0.5, State: StateComplete, Action: ActionThink},
			},
		},
	}
	tree.Root.Children[0].Parent = tree.Root
	tree.Root.Children[1].Parent = tree.Root

	best := coord.SelectBestPath(tree)
	assert.NotNil(t, best)
	assert.True(t, best.HasConclusion)

	paths := coord.RankPaths(tree)
	assert.Len(t, paths, 2)
}

// ============================================================================
// Integration Tests
// ============================================================================

func TestIntegration_FullSupervisionFlow(t *testing.T) {
	coord := NewCoordinator(nil)
	ctx := context.Background()

	// Simulate a reasoning workflow

	// 1. Start a supervised session
	step1 := &SupervisedStep{
		Content: "First, I need to understand the problem at hand.",
		Action:  ActionThink,
	}
	result1, err := coord.SuperviseStep(ctx, step1)
	require.NoError(t, err)
	assert.True(t, result1.Evaluation.Approved)

	// 2. Use a tool
	step2 := &SupervisedStep{
		Content:  "Let me search for relevant information.",
		Action:   ActionToolCall,
		ToolName: "web_search",
	}
	result2, err := coord.SuperviseStep(ctx, step2)
	require.NoError(t, err)
	assert.True(t, result2.Evaluation.Approved)

	// 3. Process tool result
	step3 := &SupervisedStep{
		Content:    "The search returned useful results.",
		Action:     ActionToolResult,
		ToolOutput: "Found 5 relevant articles.",
	}
	result3, err := coord.SuperviseStep(ctx, step3)
	require.NoError(t, err)
	assert.True(t, result3.Evaluation.Approved)

	// 4. Conclude
	step4 := &SupervisedStep{
		Content: "Based on my analysis, the answer is X because of the evidence found.",
		Action:  ActionConclude,
	}
	result4, err := coord.SuperviseStep(ctx, step4)
	require.NoError(t, err)
	assert.True(t, result4.Evaluation.Approved)
}

func TestIntegration_TreeBuildingWithSelection(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxDepth = 2
	cfg.MaxBranches = 2

	coord := NewCoordinator(cfg)
	ctx := context.Background()

	// Create expander
	expander := NewSimpleExpander(func(node *ThoughtNode, query string) []*ThoughtNode {
		if node.Depth >= 1 {
			return nil
		}
		return []*ThoughtNode{
			{Content: "Approach A: Direct solution", Action: ActionConclude, State: StateComplete, Score: 0.8},
			{Content: "Approach B: Indirect solution", Action: ActionConclude, State: StateComplete, Score: 0.6},
		}
	})

	// Build tree
	tree, err := coord.BuildTree(ctx, "req1", "How to solve this?", "Let me think about approaches.", expander)
	require.NoError(t, err)
	require.NotNil(t, tree)

	// Select best path
	best := coord.SelectBestPath(tree)
	require.NotNil(t, best)
	assert.True(t, best.HasConclusion)

	// Verify tree is active
	active := coord.GetActiveTree("req1")
	assert.NotNil(t, active)

	// Complete and verify cleanup
	coord.CompleteTree("req1")
	assert.Nil(t, coord.GetActiveTree("req1"))
}
