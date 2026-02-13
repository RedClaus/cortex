package decomposer_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/normanking/cortex/internal/cognitive"
	"github.com/normanking/cortex/internal/cognitive/decomposer"
)

// Example: Basic complexity scoring
func ExampleScoreComplexity() {
	input := "Create a new user authentication system with email verification"
	score := decomposer.ScoreComplexity(input)
	fmt.Printf("Complexity score: %d\n", score)
	// Output will vary based on input analysis
}

// Example: Complexity scoring with task type
func ExampleScoreComplexityWithType() {
	input := "Refactor the database layer to support multiple backends"
	score := decomposer.ScoreComplexityWithType(input, cognitive.TaskRefactor)
	fmt.Printf("Refactor task complexity: %d\n", score)
	// Refactor tasks get a 1.5x multiplier, so score will be higher
}

// Example: Decomposition decision
func ExampleShouldDecompose() {
	complexScore := 75
	templateMatch := 0.60

	if decomposer.ShouldDecompose(complexScore, templateMatch) {
		fmt.Println("Task should be decomposed into steps")
	} else {
		fmt.Println("Task can be handled directly")
	}
	// Output: Task should be decomposed into steps
}

// Example: Building a workflow
func ExampleWorkflowBuilder() {
	workflow := decomposer.NewWorkflow("Deploy Application").
		WithDescription("Deploy the application to production").
		WithEstimatedTime("15-20 minutes").
		AddLLMStep("Analyze deployment requirements", "What needs to be deployed?").
		AddToolStep("Run tests", "bash", "medium", "step1").
		AddApprovalStep("Approve deployment", "step2").
		AddToolStep("Deploy to production", "bash", "high", "step3").
		Build()

	fmt.Printf("Workflow: %s with %d steps\n", workflow.Name, len(workflow.Steps))
	// Output: Workflow: Deploy Application with 4 steps
}

// Mock LLM provider for testing
type mockLLM struct{}

func (m *mockLLM) Chat(ctx context.Context, messages []cognitive.ChatMessage, systemPrompt string) (string, error) {
	return `{
		"steps": [
			{
				"id": "step1",
				"description": "Analyze the requirements",
				"type": "llm",
				"risk_level": "low"
			},
			{
				"id": "step2",
				"description": "Implement the solution",
				"type": "tool",
				"tool": "edit",
				"risk_level": "medium",
				"depends_on": ["step1"]
			}
		],
		"estimated_time": "10 minutes",
		"requires_approval": false
	}`, nil
}

// Example: Full decomposition workflow
func TestDecompositionWorkflow(t *testing.T) {
	// Create decomposer with mock LLM
	llm := &mockLLM{}
	dec := decomposer.NewDecomposer(llm)

	// Complex task that needs decomposition
	input := "Build a microservices architecture with API gateway, authentication service, and user service. Deploy to Kubernetes with monitoring and logging."
	taskType := cognitive.TaskInfrastructure

	// Analyze complexity
	result := dec.Analyze(input, taskType)
	t.Logf("Complexity: %d (%s)", result.Complexity.Score, result.Complexity.Level)
	t.Logf("Needs decomposition: %v", result.Complexity.NeedsDecomp)
	t.Logf("Factors: %v", result.Complexity.Factors)

	// If complex, decompose
	if result.Complexity.NeedsDecomp {
		ctx := context.Background()
		decomposed, err := dec.Decompose(ctx, input, taskType)
		if err != nil {
			t.Fatalf("Decomposition failed: %v", err)
		}

		t.Logf("Decomposed into %d steps:", len(decomposed.Steps))
		for i, step := range decomposed.Steps {
			t.Logf("  %d. [%s] %s (type: %s, risk: %s)",
				i+1, step.ID, step.Description, step.Type, step.RiskLevel)
		}
	}
}

// Example: Workflow validation
func TestWorkflowValidation(t *testing.T) {
	// Valid workflow
	validWorkflow := decomposer.NewWorkflow("Test").
		AddLLMStep("First step", "Do something").
		AddToolStep("Second step", "bash", "low", "step1").
		Build()

	if err := decomposer.ValidateWorkflow(validWorkflow); err != nil {
		t.Errorf("Valid workflow failed validation: %v", err)
	}

	// Invalid workflow with circular dependency
	invalidWorkflow := &decomposer.Workflow{
		Steps: []decomposer.Step{
			{ID: "step1", DependsOn: []string{"step2"}},
			{ID: "step2", DependsOn: []string{"step1"}},
		},
	}

	if err := decomposer.ValidateWorkflow(invalidWorkflow); err == nil {
		t.Error("Circular dependency not detected")
	} else {
		t.Logf("Correctly detected circular dependency: %v", err)
	}
}

// Benchmark complexity scoring
func BenchmarkComplexityScoring(b *testing.B) {
	input := "Create a distributed system with load balancing, caching, and auto-scaling capabilities"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		decomposer.ScoreComplexity(input)
	}
}
