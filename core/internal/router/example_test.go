package router_test

import (
	"fmt"

	"github.com/normanking/cortex/internal/router"
)

// ExampleNewSmartRouter demonstrates basic router usage.
func ExampleNewSmartRouter() {
	// Create a router without LLM (fast path only)
	r := router.NewSmartRouter(nil)

	// Route a user request
	decision := r.RouteSimple("Fix the bug in the login function")

	fmt.Printf("Task Type: %s\n", decision.TaskType)
	fmt.Printf("Confidence: %.2f\n", decision.Confidence)
	fmt.Printf("Path: %s\n", decision.Path)

	// Output:
	// Task Type: debug
	// Confidence: 1.00
	// Path: fast
}

// ExampleSmartRouter_Route_explicit demonstrates explicit @mention routing.
func ExampleSmartRouter_Route_explicit() {
	r := router.NewSmartRouter(nil)

	// Explicit @mentions always route to the specified task type
	decision := r.RouteSimple("@review check this code for issues")

	fmt.Printf("Task Type: %s\n", decision.TaskType)
	fmt.Printf("Path: %s\n", decision.Path)
	fmt.Printf("Input: %s\n", decision.Input)

	// Output:
	// Task Type: review
	// Path: explicit
	// Input: check this code for issues
}

// ExampleSmartRouter_Route_context demonstrates context-based routing.
func ExampleSmartRouter_Route_context() {
	r := router.NewSmartRouter(nil)

	// Platform context forces infrastructure classification
	ctx := &router.ProcessContext{
		Platform: &router.PlatformInfo{
			Vendor: "cisco",
			Name:   "ios-xe",
		},
	}

	decision := r.Route("show running-config", ctx)

	fmt.Printf("Task Type: %s\n", decision.TaskType)
	fmt.Printf("Path: %s\n", decision.Path)

	// Output:
	// Task Type: infrastructure
	// Path: context
}

// ExampleFastClassifier_Classify demonstrates the fast classifier.
func ExampleFastClassifier_Classify() {
	classifier := router.NewFastClassifier()

	// Classify various inputs
	inputs := []string{
		"Fix this error in my code",
		"Write a function to parse JSON",
		"Review this pull request",
		"Deploy to production",
	}

	for _, input := range inputs {
		taskType, confidence := classifier.Classify(input)
		fmt.Printf("%s -> %s (%.2f)\n", input, taskType, confidence)
	}

	// Output:
	// Fix this error in my code -> debug (1.00)
	// Write a function to parse JSON -> code_generation (1.00)
	// Review this pull request -> review (1.00)
	// Deploy to production -> infrastructure (1.00)
}

// ExampleExtractMention demonstrates @mention extraction.
func ExampleExtractMention() {
	inputs := []string{
		"@debug fix this error",
		"@review check the code",
		"no mention here",
	}

	for _, input := range inputs {
		mention, remaining := router.ExtractMention(input)
		if mention != "" {
			fmt.Printf("Mention: @%s, Remaining: %s\n", mention, remaining)
		} else {
			fmt.Printf("No mention in: %s\n", input)
		}
	}

	// Output:
	// Mention: @debug, Remaining: fix this error
	// Mention: @review, Remaining: check the code
	// No mention in: no mention here
}

// ExampleRouterStats demonstrates statistics tracking.
func ExampleRouterStats() {
	r := router.NewSmartRouter(nil)

	// Make some routing decisions
	r.RouteSimple("Fix this bug")
	r.RouteSimple("Write a function")
	r.RouteSimple("@review check this")

	stats := r.Stats()

	fmt.Printf("Total Requests: %d\n", stats.TotalRequests)
	fmt.Printf("Fast Path Ratio: %.1f%%\n", stats.FastPathRatio())
	fmt.Printf("Explicit Hits: %d\n", stats.ExplicitHits)

	// Output:
	// Total Requests: 3
	// Fast Path Ratio: 66.7%
	// Explicit Hits: 1
}

// ExampleMockLLMRouter demonstrates using the mock LLM for testing.
func ExampleMockLLMRouter() {
	// Create a mock LLM with predefined responses
	mock := router.NewMockLLMRouter().
		WithResponse("complex query", router.TaskPlanning)

	// Use it with the router
	r := router.NewSmartRouter(mock, router.WithConfidenceThreshold(0.99))

	decision := r.RouteSimple("complex query requiring semantic analysis")

	fmt.Printf("Task Type: %s\n", decision.TaskType)

	// Output:
	// Task Type: planning
}
