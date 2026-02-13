package introspection

import (
	"context"
	"fmt"
	"testing"

	"github.com/normanking/cortex/internal/memory"
)

func TestIntrospectionDemo(t *testing.T) {
	ctx := context.Background()

	// Create components (no LLM for fast test)
	classifier := NewClassifier(nil, nil)
	analyzer := NewGapAnalyzer(nil)
	responder := NewMetacognitiveResponder()

	// Test queries
	queries := []string{
		"Do you know about Docker containers?",
		"What do you know about kubernetes?",
		"Can you help me with Python scripting?",
		"How good are you at Go programming?",
		"What do you know?",
		"List all files in the directory", // Not introspective
	}

	fmt.Println("")
	fmt.Println("=== CR-018 Introspection Demo ===")

	for _, q := range queries {
		fmt.Printf("Query: %q\n", q)

		// Step 1: Classify
		result, err := classifier.Classify(ctx, q)
		if err != nil {
			fmt.Printf("  Error: %v\n\n", err)
			continue
		}
		fmt.Printf("  Type: %s\n", result.Type)
		fmt.Printf("  Subject: %q\n", result.Subject)
		fmt.Printf("  Confidence: %.2f\n", result.Confidence)

		if result.Type == QueryTypeNotIntrospective {
			fmt.Println("  → Not an introspection query, passing through")
			fmt.Println("")
			continue
		}

		// Step 2: Simulate inventory (empty for this test)
		inventory := &memory.InventoryResult{
			Subject:      result.Subject,
			TotalMatches: 0,
			TopResults:   []memory.InventoryItem{},
		}

		// Step 3: Analyze gap
		analysis, err := analyzer.Analyze(ctx, result, inventory)
		if err != nil {
			fmt.Printf("  Analysis Error: %v\n\n", err)
			continue
		}
		fmt.Printf("  Gap Severity: %s\n", analysis.GapSeverity)
		fmt.Printf("  Has Stored: %v\n", analysis.HasStoredKnowledge)
		fmt.Printf("  LLM Can Answer: %v (%.0f%%)\n", analysis.LLMCanAnswer, analysis.LLMConfidence*100)
		fmt.Printf("  Recommended: %s\n", analysis.RecommendedAction)

		// Step 4: Generate response
		response, err := responder.GenerateFromAnalysis(analysis, inventory)
		if err != nil {
			fmt.Printf("  Response Error: %v\n\n", err)
			continue
		}
		// Truncate for display
		if len(response) > 150 {
			response = response[:150] + "..."
		}
		fmt.Printf("  Response: %s\n\n", response)
	}

	// Test with simulated stored knowledge
	fmt.Println("")
	fmt.Println("=== Test with Stored Knowledge ===")

	q := "Do you know about Git workflows?"
	fmt.Printf("Query: %q\n", q)

	result, _ := classifier.Classify(ctx, q)
	fmt.Printf("  Type: %s, Subject: %q\n", result.Type, result.Subject)

	// Simulate finding 5 items
	inventory := &memory.InventoryResult{
		Subject:      result.Subject,
		TotalMatches: 5,
		TopResults: []memory.InventoryItem{
			{Summary: "Git branching strategies", Source: "knowledge_fabric"},
			{Summary: "Git merge vs rebase", Source: "strategic_memory"},
		},
		RelatedTopics: []string{"version control", "GitHub"},
	}

	analysis, _ := analyzer.Analyze(ctx, result, inventory)
	analysis.HasStoredKnowledge = true
	analysis.StoredKnowledgeCount = 5

	fmt.Printf("  Gap Severity: %s\n", analysis.GapSeverity)
	fmt.Printf("  Has Stored: %v (%d items)\n", analysis.HasStoredKnowledge, analysis.StoredKnowledgeCount)

	response, _ := responder.GenerateFromAnalysis(analysis, inventory)
	fmt.Printf("\n  Response:\n%s\n", response)
}
