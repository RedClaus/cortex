package eval

import (
	"fmt"
)

// ExampleCapabilityScorer_Score demonstrates basic score lookup.
func ExampleCapabilityScorer_Score() {
	scorer := NewCapabilityScorer()

	// Get score for Claude Opus 4
	score := scorer.Score("anthropic", "claude-opus-4")
	fmt.Printf("Overall: %d\n", score.Overall)
	fmt.Printf("Source: %s\n", score.Source)

	// Output:
	// Overall: 98
	// Source: registry
}

// ExampleCapabilityScorer_GetCapabilities demonstrates full capability lookup.
func ExampleCapabilityScorer_GetCapabilities() {
	scorer := NewCapabilityScorer()

	// Get full capabilities for GPT-4o
	cap := scorer.GetCapabilities("openai", "gpt-4o")
	fmt.Printf("Model: %s\n", cap.DisplayName)
	fmt.Printf("Tier: %s\n", cap.Tier)
	fmt.Printf("Score: %d\n", cap.Score.Overall)
	fmt.Printf("Vision: %v\n", cap.Capabilities.Vision)

	// Output:
	// Model: GPT-4o
	// Tier: frontier
	// Score: 90
	// Vision: true
}

// ExampleCapabilityScorer_DetectProvider demonstrates provider auto-detection.
func ExampleCapabilityScorer_DetectProvider() {
	scorer := NewCapabilityScorer()

	// Detect provider from model name
	provider1 := scorer.DetectProvider("claude-sonnet-4")
	provider2 := scorer.DetectProvider("llama3:8b")
	provider3 := scorer.DetectProvider("gemini-1.5-pro")

	fmt.Printf("claude-sonnet-4: %s\n", provider1)
	fmt.Printf("llama3:8b: %s\n", provider2)
	fmt.Printf("gemini-1.5-pro: %s\n", provider3)

	// Output:
	// claude-sonnet-4: anthropic
	// llama3:8b: ollama
	// gemini-1.5-pro: gemini
}

// ExampleModelRegistry_List demonstrates listing models.
func ExampleModelRegistry_List() {
	registry := DefaultRegistry()

	// List all Anthropic models
	models := registry.List("anthropic")
	fmt.Printf("Anthropic models: %d\n", len(models))

	// Output:
	// Anthropic models: 6
}

// ExampleModelRegistry_ListByTier demonstrates tier filtering.
func ExampleModelRegistry_ListByTier() {
	registry := DefaultRegistry()

	// Get all frontier-tier models
	frontier := registry.ListByTier(TierFrontier)
	fmt.Printf("Frontier models: %d\n", len(frontier))

	// Output:
	// Frontier models: 5
}

// ExampleTierFromScore demonstrates score to tier conversion.
func ExampleTierFromScore() {
	tier1 := TierFromScore(98)
	tier2 := TierFromScore(82)
	tier3 := TierFromScore(55)
	tier4 := TierFromScore(30)

	fmt.Printf("Score 98: %s\n", tier1)
	fmt.Printf("Score 82: %s\n", tier2)
	fmt.Printf("Score 55: %s\n", tier3)
	fmt.Printf("Score 30: %s\n", tier4)

	// Output:
	// Score 98: frontier
	// Score 82: xl
	// Score 55: medium
	// Score 30: small
}

// ExampleFormatScore demonstrates score formatting.
func ExampleFormatScore() {
	label1 := FormatScore(95)
	label2 := FormatScore(80)
	label3 := FormatScore(60)
	label4 := FormatScore(40)

	fmt.Printf("95: %s\n", label1)
	fmt.Printf("80: %s\n", label2)
	fmt.Printf("60: %s\n", label3)
	fmt.Printf("40: %s\n", label4)

	// Output:
	// 95: Expert
	// 80: Advanced
	// 60: Strong
	// 40: Moderate
}

// ExampleCapabilityScorer_heuristic demonstrates heuristic scoring for unknown models.
func ExampleCapabilityScorer_heuristic() {
	scorer := NewCapabilityScorer()

	// Score unknown models (will use heuristics)
	score7b := scorer.Score("ollama", "unknown-model:7b")
	score70b := scorer.Score("ollama", "unknown-model:70b")

	fmt.Printf("7B model score: %d-%d range\n", score7b.Overall-5, score7b.Overall+5)
	fmt.Printf("70B model score: %d-%d range\n", score70b.Overall-5, score70b.Overall+5)
	fmt.Printf("Source: %s\n", score7b.Source)

	// Output:
	// 7B model score: 47-57 range
	// 70B model score: 77-87 range
	// Source: heuristic
}

// ExampleCapabilityScorer_CompareModels demonstrates model comparison.
func ExampleCapabilityScorer_CompareModels() {
	scorer := NewCapabilityScorer()

	// Compare models
	cmp1 := scorer.CompareModels("anthropic", "claude-opus-4", "openai", "gpt-4o")
	cmp2 := scorer.CompareModels("ollama", "llama3:8b", "ollama", "llama3:70b")

	if cmp1 > 0 {
		fmt.Println("claude-opus-4 > gpt-4o")
	}
	if cmp2 < 0 {
		fmt.Println("llama3:8b < llama3:70b")
	}

	// Output:
	// claude-opus-4 > gpt-4o
	// llama3:8b < llama3:70b
}

// ExampleCapabilityScorer_RecommendForComplexity demonstrates complexity-based recommendations.
func ExampleCapabilityScorer_RecommendForComplexity() {
	scorer := NewCapabilityScorer()

	// Get models suitable for complexity 70 (prefer local)
	candidates := scorer.RecommendForComplexity(70, true)

	fmt.Printf("Found %d+ capable local models\n", len(candidates))
	if len(candidates) > 0 {
		fmt.Printf("Smallest capable: %s tier\n", candidates[0].Tier)
	}

	// Output:
	// Found 21+ capable local models
	// Smallest capable: large tier
}
