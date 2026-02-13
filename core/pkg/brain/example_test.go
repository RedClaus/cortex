package brain_test

import (
	"context"
	"fmt"
	"time"

	"github.com/normanking/cortex/pkg/brain"
	"github.com/normanking/cortex/pkg/brain/lobes"
)

func Example_basicUsage() {
	// Create executive with nil providers (uses regex classification only)
	exec := brain.NewExecutive(brain.ExecutiveConfig{})

	// Register lobes that don't need LLM
	exec.RegisterLobe(lobes.NewSafetyLobe())
	exec.RegisterLobe(lobes.NewTextParsingLobe())
	exec.RegisterLobe(lobes.NewAttentionLobe())
	exec.RegisterLobe(lobes.NewMetacognitionLobe())
	exec.RegisterLobe(lobes.NewInhibitionLobe())
	exec.RegisterLobe(lobes.NewSelfKnowledgeLobe())

	// Start monitoring
	exec.Start()
	defer exec.Stop()

	// Check metrics
	metrics := exec.GetMetrics()
	fmt.Printf("Goroutines: %d\n", metrics.GoRoutineCount)
	fmt.Printf("Memory: %d MB\n", metrics.MemoryUsedMB)

	// Output will vary based on system state
}

func Example_classifier() {
	cache := &simpleCache{data: make(map[string]*brain.ClassificationResult)}
	classifier := brain.NewExecutiveClassifier(nil, nil, cache)

	ctx := context.Background()

	testInputs := []string{
		"write a function to sort an array",
		"remember what we discussed yesterday",
		"plan the steps to deploy this app",
		"why does the sky appear blue",
		"brainstorm ideas for a startup",
	}

	for _, input := range testInputs {
		result, err := classifier.Classify(ctx, input)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		fmt.Printf("Input: %q → Lobe: %s (method: %s)\n",
			input[:20]+"...", result.PrimaryLobe, result.Method)
	}

	// Output:
	// Input: "write a function to ..." → Lobe: coding (method: regex)
	// Input: "remember what we dis..." → Lobe: memory (method: regex)
	// Input: "plan the steps to de..." → Lobe: planning (method: regex)
	// Input: "why does the sky app..." → Lobe: reasoning (method: regex)
	// Input: "brainstorm ideas for..." → Lobe: creativity (method: regex)
}

func Example_strategies() {
	strategies := []brain.ThinkingStrategy{
		brain.QuickAnswerStrategy(),
		brain.DeepReasoningStrategy(),
		brain.CodingStrategy(),
		brain.CreativeStrategy(),
		brain.SafetyFirstStrategy(),
	}

	for _, s := range strategies {
		fmt.Printf("Strategy: %s, Phases: %d, Tier: %s\n",
			s.Name, len(s.Phases), s.ComputeTier)
	}

	// Output:
	// Strategy: QuickAnswer, Phases: 1, Tier: fast
	// Strategy: DeepReasoning, Phases: 3, Tier: deep
	// Strategy: Coding, Phases: 3, Tier: hybrid
	// Strategy: Creative, Phases: 3, Tier: deep
	// Strategy: SafetyFirst, Phases: 2, Tier: deep
}

func Example_blackboard() {
	bb := brain.NewBlackboard()
	bb.ConversationID = "conv-123"
	bb.TurnNumber = 1

	// Add memories
	bb.AddMemory(brain.Memory{
		ID:        "mem-1",
		Content:   "User prefers Go for backend development",
		Relevance: 0.9,
	})

	// Add entities
	bb.AddEntity(brain.Entity{
		Type:  "language",
		Value: "Go",
	})

	// Set user state
	bb.SetUserState(&brain.UserState{
		ExpertiseLevel: "advanced",
		EstimatedMood:  "focused",
	})

	summary := bb.Summary()
	fmt.Printf("Memories: %d, Entities: %d, Confidence: %.1f\n",
		summary["memory_count"], summary["entity_count"], summary["overall_confidence"])

	// Output:
	// Memories: 1, Entities: 1, Confidence: 1.0
}

func Example_monitor() {
	monitor := brain.NewSystemMonitor(100 * time.Millisecond)
	monitor.Start()
	defer monitor.Stop()

	time.Sleep(150 * time.Millisecond)

	tier := monitor.SuggestComputeTier()
	constrained := monitor.IsSystemConstrained()

	fmt.Printf("Suggested tier: %s\n", tier)
	fmt.Printf("System constrained: %v\n", constrained)

	// Output will vary based on system load
}

func Example_outcomeLogger() {
	logger := brain.NewOutcomeLogger(nil, 100)

	// Log some executions
	for i := 0; i < 5; i++ {
		logger.Log(brain.ExecutionRecord{
			Input: fmt.Sprintf("test input %d", i),
			Classification: brain.ClassificationResult{
				PrimaryLobe: brain.LobeCoding,
			},
			Outcome: brain.Outcome{
				Success:    i%2 == 0,
				LatencyMS:  int64(100 + i*10),
				TokensUsed: 500 + i*50,
			},
		})
	}

	// Add feedback to latest
	logger.AddFeedback(5, "Great response!")

	stats := logger.GetStats()
	fmt.Printf("Total: %d, Success Rate: %.0f%%, Avg Latency: %.0fms\n",
		stats.TotalExecutions, stats.SuccessRate*100, stats.AvgLatencyMS)

	// Output:
	// Total: 5, Success Rate: 60%, Avg Latency: 120ms
}

type simpleCache struct {
	data map[string]*brain.ClassificationResult
}

func (c *simpleCache) Get(key string) (*brain.ClassificationResult, bool) {
	r, ok := c.data[key]
	return r, ok
}

func (c *simpleCache) Set(key string, result *brain.ClassificationResult) {
	c.data[key] = result
}
