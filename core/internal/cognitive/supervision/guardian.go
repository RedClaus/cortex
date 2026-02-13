package supervision

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/normanking/cortex/pkg/brain"
	"github.com/normanking/cortex/pkg/brain/lobes"
)

// InhibitionGuardian uses the Inhibition lobe for thought evaluation.
// It provides deterministic, fast (<200ms) evaluation without LLM calls.
type InhibitionGuardian struct {
	inhibitionLobe *lobes.InhibitionLobe
	timeout        time.Duration

	// Additional patterns for thought-specific validation
	invalidPatterns   []*regexp.Regexp
	lowQualityPatterns []*regexp.Regexp
}

// NewInhibitionGuardian creates a guardian that wraps the Inhibition lobe.
func NewInhibitionGuardian(timeout time.Duration) *InhibitionGuardian {
	if timeout <= 0 {
		timeout = 200 * time.Millisecond
	}

	return &InhibitionGuardian{
		inhibitionLobe: lobes.NewInhibitionLobe(),
		timeout:        timeout,
		invalidPatterns: []*regexp.Regexp{
			// Circular reasoning
			regexp.MustCompile(`(?i)as\s+i\s+(just\s+)?said|as\s+mentioned\s+(above|before|earlier)`),
			// Hallucination indicators
			regexp.MustCompile(`(?i)i\s+believe|i\s+think\s+that|probably|maybe|perhaps`),
			// Contradiction markers
			regexp.MustCompile(`(?i)actually,?\s+no|wait,?\s+that'?s\s+wrong|on\s+second\s+thought`),
		},
		lowQualityPatterns: []*regexp.Regexp{
			// Vague responses
			regexp.MustCompile(`(?i)^(i\s+don'?t\s+know|i'?m\s+not\s+sure|it\s+depends)`),
			// Refusals without explanation
			regexp.MustCompile(`(?i)^(i\s+can'?t|i\s+cannot|i'?m\s+unable\s+to)$`),
			// Empty or minimal content
			regexp.MustCompile(`^\s*$`),
		},
	}
}

// Evaluate assesses a thought node using the Inhibition lobe.
// This is deterministic and fast (<200ms guaranteed via timeout).
func (g *InhibitionGuardian) Evaluate(ctx context.Context, node *ThoughtNode, tree *ThoughtTree) (*GuardianResult, error) {
	startTime := time.Now()

	// Create timeout context
	evalCtx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	result := &GuardianResult{
		Approved:    true,
		Score:       1.0,
		Confidence:  0.9,
		RiskFactors: []string{},
		Suggestions: []string{},
	}

	// Channel for async evaluation
	done := make(chan struct{})
	go func() {
		defer close(done)
		g.evaluateNode(node, tree, result)
	}()

	// Wait for evaluation or timeout
	select {
	case <-done:
		// Evaluation completed
	case <-evalCtx.Done():
		// Timeout - auto-approve with lower confidence
		result.Approved = true
		result.Confidence = 0.5
		result.Reason = "Guardian timeout - auto-approved"
		result.Suggestions = append(result.Suggestions, "Consider simplifying the thought")
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

// evaluateNode performs the actual evaluation logic.
func (g *InhibitionGuardian) evaluateNode(node *ThoughtNode, tree *ThoughtTree, result *GuardianResult) {
	content := node.Content

	// 1. Check via Inhibition lobe (prompt injection, etc.)
	lobeInput := brain.LobeInput{
		RawInput: content,
	}
	bb := brain.NewBlackboard()

	lobeResult, err := g.inhibitionLobe.Process(context.Background(), lobeInput, bb)
	if err == nil && lobeResult.Content != nil {
		if inhibResult, ok := lobeResult.Content.(lobes.InhibitionResult); ok {
			if inhibResult.ShouldInhibit {
				result.Approved = false
				result.Score = 0.0
				result.Reason = "Inhibition triggered: " + inhibResult.RecommendedAction
				result.RiskFactors = append(result.RiskFactors, inhibResult.RiskFactors...)
				return
			}
		}
	}

	// 2. Check for invalid patterns
	for _, pattern := range g.invalidPatterns {
		if pattern.MatchString(content) {
			result.Score -= 0.2
			result.RiskFactors = append(result.RiskFactors, "invalid_pattern: "+pattern.String())
		}
	}

	// 3. Check for low quality patterns
	for _, pattern := range g.lowQualityPatterns {
		if pattern.MatchString(content) {
			result.Score -= 0.3
			result.RiskFactors = append(result.RiskFactors, "low_quality: "+pattern.String())
		}
	}

	// 4. Check content length
	if len(content) < 10 {
		result.Score -= 0.2
		result.RiskFactors = append(result.RiskFactors, "content_too_short")
	}
	if len(content) > 5000 {
		result.Score -= 0.1
		result.RiskFactors = append(result.RiskFactors, "content_too_long")
	}

	// 5. Check for consistency with parent
	if node.Parent != nil {
		consistency := g.checkConsistency(node, node.Parent)
		if consistency < 0.5 {
			result.Score -= 0.2
			result.RiskFactors = append(result.RiskFactors, "inconsistent_with_parent")
		}
	}

	// 6. Check tree-level constraints
	if tree != nil && node.Parent != nil {
		// Avoid repetition with siblings
		for _, sibling := range node.Parent.Children {
			if sibling != node && g.isSimilar(node.Content, sibling.Content) {
				result.Score -= 0.3
				result.RiskFactors = append(result.RiskFactors, "similar_to_sibling")
				break
			}
		}
	}

	// 7. Evaluate action-specific quality
	switch node.Action {
	case ActionToolCall:
		if node.ToolName == "" {
			result.Score -= 0.4
			result.RiskFactors = append(result.RiskFactors, "tool_call_without_tool")
		}
	case ActionConclude:
		if len(content) < 20 {
			result.Score -= 0.2
			result.RiskFactors = append(result.RiskFactors, "conclusion_too_short")
		}
	}

	// Clamp score
	if result.Score < 0 {
		result.Score = 0
	}
	if result.Score > 1 {
		result.Score = 1
	}

	// Set approval based on score threshold
	result.Approved = result.Score >= 0.3

	// Generate reason
	if !result.Approved {
		result.Reason = "Score below threshold: " + strings.Join(result.RiskFactors, ", ")
	} else if len(result.RiskFactors) > 0 {
		result.Reason = "Approved with concerns: " + strings.Join(result.RiskFactors, ", ")
	} else {
		result.Reason = "Approved"
	}

	// Add suggestions based on issues
	if contains(result.RiskFactors, "content_too_short") {
		result.Suggestions = append(result.Suggestions, "Expand on the reasoning")
	}
	if contains(result.RiskFactors, "inconsistent_with_parent") {
		result.Suggestions = append(result.Suggestions, "Ensure logical flow from previous step")
	}
}

// checkConsistency checks semantic consistency between two nodes.
func (g *InhibitionGuardian) checkConsistency(node, parent *ThoughtNode) float64 {
	// Simple heuristic: check for contradictory markers
	contradictions := []string{
		"but actually", "however", "on the contrary",
		"that's wrong", "not true", "incorrect",
	}

	lowerContent := strings.ToLower(node.Content)
	for _, c := range contradictions {
		if strings.Contains(lowerContent, c) {
			return 0.3
		}
	}

	// Check if node references parent content
	parentWords := strings.Fields(strings.ToLower(parent.Content))
	nodeContent := strings.ToLower(node.Content)
	matches := 0
	for _, word := range parentWords {
		if len(word) > 4 && strings.Contains(nodeContent, word) {
			matches++
		}
	}

	if len(parentWords) > 0 {
		return float64(matches) / float64(len(parentWords)) * 0.5 + 0.5
	}

	return 0.7 // Default reasonable consistency
}

// isSimilar checks if two content strings are too similar.
func (g *InhibitionGuardian) isSimilar(a, b string) bool {
	if a == b {
		return true
	}

	// Jaccard similarity on words
	wordsA := strings.Fields(strings.ToLower(a))
	wordsB := strings.Fields(strings.ToLower(b))

	if len(wordsA) == 0 || len(wordsB) == 0 {
		return false
	}

	setA := make(map[string]bool)
	for _, w := range wordsA {
		setA[w] = true
	}

	intersection := 0
	for _, w := range wordsB {
		if setA[w] {
			intersection++
		}
	}

	union := len(setA)
	for _, w := range wordsB {
		if !setA[w] {
			union++
		}
	}

	similarity := float64(intersection) / float64(union)
	return similarity > 0.8
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.Contains(s, item) {
			return true
		}
	}
	return false
}
