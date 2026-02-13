package prompts_test

import (
	"fmt"

	"github.com/normanking/cortex/internal/cognitive"
	"github.com/normanking/cortex/internal/prompts"
)

// Example demonstrates basic usage of the prompts integration
func Example_basicUsage() {
	// Load the default prompts store
	store := prompts.Load()
	provider := prompts.NewTemplateProvider(store)

	// Get a prompt for a small model (7B)
	smallPrompt := provider.GetSystemPrompt("terminal_error_diagnosis", 7_000_000_000)
	fmt.Printf("Small model prompt length: %d\n", len(smallPrompt))

	// Get a prompt for a large model (70B)
	largePrompt := provider.GetSystemPrompt("terminal_error_diagnosis", 70_000_000_000)
	fmt.Printf("Large model prompt length: %d\n", len(largePrompt))

	// List all available tasks
	tasks := provider.ListTasks()
	fmt.Printf("Available tasks: %d\n", len(tasks))

	// Output:
	// Small model prompt length: 528
	// Large model prompt length: 130
	// Available tasks: 12
}

// Example demonstrates using the PromptManager with cognitive templates
func Example_cognitiveIntegration() {
	// Create a PromptManager
	pm := cognitive.NewPromptManager()

	// Get optimized prompt for a task
	task := "command_suggestion"
	modelParams := int64(7_000_000_000) // 7B model

	prompt := pm.GetOptimizedPrompt(task, modelParams)
	fmt.Printf("Got prompt for %s: %d bytes\n", task, len(prompt))

	// Apply to a template context
	ctx := pm.ApplyToTemplate(task, modelParams, map[string]interface{}{
		"user_query": "how to list files",
	})

	systemPrompt := ctx["system_prompt"].(string)
	fmt.Printf("Template context has system_prompt: %v\n", len(systemPrompt) > 0)

	// Output:
	// Got prompt for command_suggestion: 402 bytes
	// Template context has system_prompt: true
}

// Example demonstrates custom prompt registration
func Example_customPrompts() {
	// Create a provider
	provider := prompts.NewTemplateProvider(prompts.Load())

	// Register a custom prompt
	customTask := "my_custom_task"
	customPrompt := "You are a helpful assistant specialized in custom tasks."
	provider.RegisterCustomPrompt(customTask, "small", customPrompt)

	// Retrieve the custom prompt
	retrieved := provider.GetSystemPrompt(customTask, 7_000_000_000)
	fmt.Printf("Custom prompt matches: %v\n", retrieved == customPrompt)

	// Custom prompts appear in ListTasks
	tasks := provider.ListTasks()
	hasCustom := false
	for _, task := range tasks {
		if task == customTask {
			hasCustom = true
			break
		}
	}
	fmt.Printf("Custom task in list: %v\n", hasCustom)

	// Output:
	// Custom prompt matches: true
	// Custom task in list: true
}

// Example demonstrates model tier mapping
func Example_modelTierMapping() {
	// Map model sizes to cognitive tiers
	tiers := []struct {
		params int64
		tier   cognitive.ModelTier
	}{
		{1_000_000_000, cognitive.TierLocal},    // 1B
		{7_000_000_000, cognitive.TierMid},      // 7B
		{14_000_000_000, cognitive.TierAdvanced}, // 14B
		{70_000_000_000, cognitive.TierFrontier}, // 70B
	}

	for _, t := range tiers {
		tier := cognitive.GetModelTierFromParams(t.params)
		fmt.Printf("%dB -> %s\n", t.params/1_000_000_000, tier)
	}

	// Output:
	// 1B -> local
	// 7B -> mid
	// 14B -> advanced
	// 70B -> frontier
}

// Example demonstrates enriching template context with prompt metadata
func Example_enrichTemplateContext() {
	pm := cognitive.NewPromptManager()

	task := "tui_troubleshooting"
	modelParams := int64(14_000_000_000) // 14B model

	// Enrich context with prompt and metadata
	ctx := pm.EnrichTemplateContext(task, modelParams, map[string]interface{}{
		"error_message": "TUI rendering issue",
	})

	// Check what was added
	fmt.Printf("Has system_prompt: %v\n", ctx["system_prompt"] != nil)
	fmt.Printf("Has prompt_tier: %v\n", ctx["prompt_tier"] != nil)
	fmt.Printf("Has model_tier: %v\n", ctx["model_tier"] != nil)
	fmt.Printf("Original data preserved: %v\n", ctx["error_message"] == "TUI rendering issue")
	fmt.Printf("Model tier: %s\n", ctx["model_tier"])
	fmt.Printf("Prompt tier: %s\n", ctx["prompt_tier"])

	// Output:
	// Has system_prompt: true
	// Has prompt_tier: true
	// Has model_tier: true
	// Original data preserved: true
	// Model tier: advanced
	// Prompt tier: large
}
