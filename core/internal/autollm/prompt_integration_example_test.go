package autollm

import (
	"context"
	"testing"

	"github.com/normanking/cortex/internal/llm"
)

// Example demonstrating prompt optimization integration.
// When a TaskType is provided, the router automatically injects
// the tier-optimized system prompt before making the LLM call.
func ExampleRouter_Complete_withOptimizedPrompt() {
	// Create router
	config := RouterConfig{
		FastModels:  []string{"llama3:8b"},
		SmartModels: []string{"claude-3-5-sonnet"},
	}
	router := NewRouter(config, nil)

	// Create a mock provider map
	providers := map[string]llm.Provider{
		// In real usage, these would be actual provider implementations
	}

	// Request with TaskType specified
	req := Request{
		Prompt:   "Why does my npm install fail with EACCES?",
		TaskType: "terminal_error_diagnosis", // Triggers optimized prompt injection
	}

	// Router will:
	// 1. Route to appropriate model (llama3:8b - small tier)
	// 2. Check if optimized prompt exists for task type
	// 3. Inject tier-specific system prompt (small tier for llama3)
	// 4. Execute the completion
	_, decision, _ := router.Complete(context.Background(), req, providers)

	// The optimized prompt was automatically selected based on model tier
	_ = decision
}

// TestRouter_PromptInjection verifies that optimized prompts are injected
// when TaskType is specified and available in the store.
func TestRouter_PromptInjection(t *testing.T) {
	config := RouterConfig{
		FastModels:  []string{"llama3:8b"},
		SmartModels: []string{"gpt-4o"},
	}
	router := NewRouter(config, nil)

	tests := []struct {
		name         string
		taskType     string
		expectPrompt bool
	}{
		{
			name:         "known task type gets optimized prompt",
			taskType:     "terminal_error_diagnosis",
			expectPrompt: true,
		},
		{
			name:         "unknown task type returns no prompt",
			taskType:     "unknown_task",
			expectPrompt: false,
		},
		{
			name:         "empty task type returns no prompt",
			taskType:     "",
			expectPrompt: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasPrompt := router.promptStore.Has(tt.taskType)
			if hasPrompt != tt.expectPrompt {
				t.Errorf("expected Has(%q) = %v, got %v", tt.taskType, tt.expectPrompt, hasPrompt)
			}
		})
	}
}

// TestRouter_PromptTierMapping verifies that model tiers map correctly
// to prompt tiers (small vs large).
func TestRouter_PromptTierMapping(t *testing.T) {
	config := RouterConfig{
		FastModels:  []string{"llama3:8b"},
		SmartModels: []string{"gpt-4o"},
	}
	router := NewRouter(config, nil)

	tests := []struct {
		name         string
		modelName    string
		expectedTier string
	}{
		{
			name:         "small local model gets small tier",
			modelName:    "llama3:8b",
			expectedTier: "small",
		},
		{
			name:         "large cloud model gets large tier",
			modelName:    "gpt-4o",
			expectedTier: "large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cap := router.getModel(tt.modelName)
			tier := router.getPromptTier(cap)

			if tier != tt.expectedTier {
				t.Errorf("expected tier %q for model %q, got %q", tt.expectedTier, tt.modelName, tier)
			}
		})
	}
}
