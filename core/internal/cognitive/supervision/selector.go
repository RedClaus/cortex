package supervision

import (
	"sort"
)

// RewardSelector selects the best path using hybrid reward signals.
type RewardSelector struct {
	// Weights for different reward components
	ScoreWeight      float64 // Weight for node scores (default: 0.4)
	DepthWeight      float64 // Weight for path depth (default: 0.1)
	ConclusionWeight float64 // Weight for having a conclusion (default: 0.3)
	ToolWeight       float64 // Weight for tool usage (default: 0.2)
}

// NewRewardSelector creates a selector with default weights.
func NewRewardSelector() *RewardSelector {
	return &RewardSelector{
		ScoreWeight:      0.4,
		DepthWeight:      0.1,
		ConclusionWeight: 0.3,
		ToolWeight:       0.2,
	}
}

// SelectBest chooses the best path through the tree.
func (s *RewardSelector) SelectBest(tree *ThoughtTree) *PathScore {
	paths := s.RankPaths(tree)
	if len(paths) == 0 {
		return nil
	}
	return paths[0]
}

// RankPaths ranks all complete paths in the tree.
func (s *RewardSelector) RankPaths(tree *ThoughtTree) []*PathScore {
	leaves := tree.GetLeafNodes()
	if len(leaves) == 0 {
		return nil
	}

	var paths []*PathScore

	for _, leaf := range leaves {
		path := leaf.GetPath()
		if len(path) == 0 {
			continue
		}

		ps := s.scorePath(path)
		paths = append(paths, ps)
	}

	// Sort by total score descending
	sort.Slice(paths, func(i, j int) bool {
		return paths[i].TotalScore > paths[j].TotalScore
	})

	return paths
}

// scorePath calculates the reward score for a path.
func (s *RewardSelector) scorePath(path []*ThoughtNode) *PathScore {
	ps := &PathScore{
		Path:      path,
		Depth:     len(path),
		ToolsUsed: make([]string, 0),
		MinScore:  1.0,
	}

	// Track tools and calculate scores
	toolSet := make(map[string]bool)
	var totalScore float64

	for _, node := range path {
		// Track minimum score
		if node.Score < ps.MinScore {
			ps.MinScore = node.Score
		}

		// Sum scores
		totalScore += node.Score

		// Track tools
		if node.ToolName != "" && !toolSet[node.ToolName] {
			toolSet[node.ToolName] = true
			ps.ToolsUsed = append(ps.ToolsUsed, node.ToolName)
		}

		// Check for conclusion
		if node.Action == ActionConclude {
			ps.HasConclusion = true
		}
	}

	// Calculate average score
	if len(path) > 0 {
		ps.AvgScore = totalScore / float64(len(path))
	}

	// Calculate total weighted score
	ps.TotalScore = s.calculateTotalScore(ps)

	return ps
}

// calculateTotalScore computes the weighted total score.
func (s *RewardSelector) calculateTotalScore(ps *PathScore) float64 {
	score := 0.0

	// Score component (average of node scores)
	score += ps.AvgScore * s.ScoreWeight

	// Depth component (prefer moderate depth, penalize very short or very long)
	depthScore := 0.0
	switch {
	case ps.Depth <= 1:
		depthScore = 0.3 // Too shallow
	case ps.Depth <= 3:
		depthScore = 1.0 // Ideal
	case ps.Depth <= 5:
		depthScore = 0.8 // Good
	case ps.Depth <= 7:
		depthScore = 0.5 // Getting long
	default:
		depthScore = 0.3 // Too long
	}
	score += depthScore * s.DepthWeight

	// Conclusion component
	if ps.HasConclusion {
		score += 1.0 * s.ConclusionWeight
	}

	// Tool usage component (reward using tools appropriately)
	toolScore := 0.0
	switch len(ps.ToolsUsed) {
	case 0:
		toolScore = 0.5 // No tools might be okay for simple queries
	case 1:
		toolScore = 0.8 // Single tool is focused
	case 2, 3:
		toolScore = 1.0 // Multiple tools shows thorough investigation
	default:
		toolScore = 0.7 // Too many tools might indicate confusion
	}
	score += toolScore * s.ToolWeight

	return score
}

// ApplyReward applies a reward signal to nodes in a path.
func (s *RewardSelector) ApplyReward(path []*ThoughtNode, signal RewardSignal) {
	for _, node := range path {
		adjustment := signal.Value * signal.Weight
		node.Score += adjustment

		// Clamp score
		if node.Score < 0 {
			node.Score = 0
		}
		if node.Score > 1 {
			node.Score = 1
		}
	}
}

// GenerateRewardSignals creates reward signals based on path analysis.
func (s *RewardSelector) GenerateRewardSignals(ps *PathScore) []RewardSignal {
	var signals []RewardSignal

	// Reward for completion
	if ps.HasConclusion {
		signals = append(signals, RewardSignal{
			Source: "completion",
			Value:  0.2,
			Weight: 1.0,
			Reason: "Path reaches a conclusion",
		})
	} else {
		signals = append(signals, RewardSignal{
			Source: "completion",
			Value:  -0.1,
			Weight: 1.0,
			Reason: "Path does not reach a conclusion",
		})
	}

	// Reward for consistency (high min score)
	if ps.MinScore >= 0.7 {
		signals = append(signals, RewardSignal{
			Source: "consistency",
			Value:  0.15,
			Weight: 1.0,
			Reason: "All nodes maintain high quality",
		})
	} else if ps.MinScore < 0.3 {
		signals = append(signals, RewardSignal{
			Source: "consistency",
			Value:  -0.2,
			Weight: 1.0,
			Reason: "Path contains low-quality nodes",
		})
	}

	// Reward for appropriate tool use
	if len(ps.ToolsUsed) >= 1 && len(ps.ToolsUsed) <= 3 {
		signals = append(signals, RewardSignal{
			Source: "tool_use",
			Value:  0.1,
			Weight: 1.0,
			Reason: "Appropriate tool usage",
		})
	}

	// Penalty for excessive depth
	if ps.Depth > 7 {
		signals = append(signals, RewardSignal{
			Source: "efficiency",
			Value:  -0.15,
			Weight: 1.0,
			Reason: "Path is excessively long",
		})
	}

	return signals
}

// PathComparison compares two paths and returns which is better.
type PathComparison struct {
	Better      *PathScore
	Worse       *PathScore
	Margin      float64
	Reasons     []string
}

// Compare compares two paths and explains the difference.
func (s *RewardSelector) Compare(a, b *PathScore) *PathComparison {
	if a == nil || b == nil {
		return nil
	}

	comp := &PathComparison{
		Reasons: make([]string, 0),
	}

	if a.TotalScore >= b.TotalScore {
		comp.Better = a
		comp.Worse = b
	} else {
		comp.Better = b
		comp.Worse = a
	}

	comp.Margin = comp.Better.TotalScore - comp.Worse.TotalScore

	// Explain the differences
	if a.AvgScore > b.AvgScore {
		comp.Reasons = append(comp.Reasons, "Higher average node quality")
	} else if b.AvgScore > a.AvgScore {
		comp.Reasons = append(comp.Reasons, "Lower average node quality")
	}

	if a.HasConclusion && !b.HasConclusion {
		comp.Reasons = append(comp.Reasons, "Has conclusion")
	} else if b.HasConclusion && !a.HasConclusion {
		comp.Reasons = append(comp.Reasons, "Missing conclusion")
	}

	if len(a.ToolsUsed) > len(b.ToolsUsed) {
		comp.Reasons = append(comp.Reasons, "More tool usage")
	}

	return comp
}
