package cognitive

import (
	"testing"

	"github.com/normanking/cortex/internal/prompts"
)

func TestPromptManager_GetOptimizedPrompt(t *testing.T) {
	pm := NewPromptManager()

	tests := []struct {
		name        string
		task        string
		modelParams int64
		wantEmpty   bool
	}{
		{
			name:        "small model",
			task:        "terminal_error_diagnosis",
			modelParams: 7_000_000_000,
			wantEmpty:   false,
		},
		{
			name:        "large model",
			task:        "terminal_error_diagnosis",
			modelParams: 70_000_000_000,
			wantEmpty:   false,
		},
		{
			name:        "default model params (0)",
			task:        "command_suggestion",
			modelParams: 0, // Should use default (7B)
			wantEmpty:   false,
		},
		{
			name:        "nonexistent task",
			task:        "nonexistent_task",
			modelParams: 7_000_000_000,
			wantEmpty:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := pm.GetOptimizedPrompt(tt.task, tt.modelParams)
			if tt.wantEmpty && prompt != "" {
				t.Errorf("GetOptimizedPrompt() = %q, want empty", prompt)
			}
			if !tt.wantEmpty && prompt == "" {
				t.Errorf("GetOptimizedPrompt() returned empty, want non-empty")
			}
		})
	}
}

func TestPromptManager_ApplyToTemplate(t *testing.T) {
	pm := NewPromptManager()

	task := "terminal_error_diagnosis"
	modelParams := int64(7_000_000_000)

	// Test with nil context
	ctx := pm.ApplyToTemplate(task, modelParams, nil)
	if ctx == nil {
		t.Fatal("ApplyToTemplate() returned nil context")
	}

	systemPrompt, ok := ctx["system_prompt"].(string)
	if !ok {
		t.Fatal("ApplyToTemplate() did not set system_prompt in context")
	}
	if systemPrompt == "" {
		t.Error("ApplyToTemplate() set empty system_prompt")
	}

	// Test with existing context
	existingCtx := map[string]interface{}{
		"existing_key": "existing_value",
	}
	ctx = pm.ApplyToTemplate(task, modelParams, existingCtx)

	if ctx["existing_key"] != "existing_value" {
		t.Error("ApplyToTemplate() overwrote existing context values")
	}

	systemPrompt, ok = ctx["system_prompt"].(string)
	if !ok || systemPrompt == "" {
		t.Error("ApplyToTemplate() did not properly set system_prompt with existing context")
	}
}

func TestPromptManager_EnrichTemplateContext(t *testing.T) {
	pm := NewPromptManager()

	task := "terminal_error_diagnosis"
	modelParams := int64(7_000_000_000)

	ctx := pm.EnrichTemplateContext(task, modelParams, nil)

	// Check all expected fields
	fields := []string{"system_prompt", "prompt_tier", "model_tier"}
	for _, field := range fields {
		if _, ok := ctx[field]; !ok {
			t.Errorf("EnrichTemplateContext() did not set %q field", field)
		}
	}

	// Verify prompt_tier
	promptTier, ok := ctx["prompt_tier"].(string)
	if !ok || promptTier == "" {
		t.Error("EnrichTemplateContext() did not set valid prompt_tier")
	}

	// Verify model_tier
	modelTier, ok := ctx["model_tier"].(string)
	if !ok || modelTier == "" {
		t.Error("EnrichTemplateContext() did not set valid model_tier")
	}
}

func TestGetModelTierFromParams(t *testing.T) {
	tests := []struct {
		name        string
		modelParams int64
		wantTier    ModelTier
	}{
		{
			name:        "unknown (0) -> local",
			modelParams: 0,
			wantTier:    TierLocal,
		},
		{
			name:        "1B -> local",
			modelParams: 1_000_000_000,
			wantTier:    TierLocal,
		},
		{
			name:        "3B -> mid",
			modelParams: 3_000_000_000,
			wantTier:    TierMid,
		},
		{
			name:        "7B -> mid",
			modelParams: 7_000_000_000,
			wantTier:    TierMid,
		},
		{
			name:        "14B -> advanced",
			modelParams: 14_000_000_000,
			wantTier:    TierAdvanced,
		},
		{
			name:        "70B -> frontier",
			modelParams: 70_000_000_000,
			wantTier:    TierFrontier,
		},
		{
			name:        "405B -> frontier",
			modelParams: 405_000_000_000,
			wantTier:    TierFrontier,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tier := GetModelTierFromParams(tt.modelParams)
			if tier != tt.wantTier {
				t.Errorf("GetModelTierFromParams(%d) = %v, want %v",
					tt.modelParams, tier, tt.wantTier)
			}
		})
	}
}

func TestGetPromptTierFromModelTier(t *testing.T) {
	tests := []struct {
		name      string
		modelTier ModelTier
		wantTier  string
	}{
		{
			name:      "local -> small",
			modelTier: TierLocal,
			wantTier:  "small",
		},
		{
			name:      "mid -> small",
			modelTier: TierMid,
			wantTier:  "small",
		},
		{
			name:      "advanced -> large",
			modelTier: TierAdvanced,
			wantTier:  "large",
		},
		{
			name:      "frontier -> large",
			modelTier: TierFrontier,
			wantTier:  "large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tier := GetPromptTierFromModelTier(tt.modelTier)
			if tier != tt.wantTier {
				t.Errorf("GetPromptTierFromModelTier(%v) = %q, want %q",
					tt.modelTier, tier, tt.wantTier)
			}
		})
	}
}

func TestPromptManager_CustomPrompts(t *testing.T) {
	pm := NewPromptManager()

	customTask := "custom_task"
	customPrompt := "Custom test prompt"

	// Register custom prompt
	pm.RegisterCustomPrompt(customTask, "small", customPrompt)

	// Verify it's retrievable
	prompt := pm.GetOptimizedPrompt(customTask, 7_000_000_000)
	if prompt != customPrompt {
		t.Errorf("GetOptimizedPrompt() after RegisterCustomPrompt() = %q, want %q",
			prompt, customPrompt)
	}

	// Remove and verify
	pm.RemoveCustomPrompt(customTask, "small")
	prompt = pm.GetOptimizedPrompt(customTask, 7_000_000_000)
	if prompt != "" {
		t.Errorf("GetOptimizedPrompt() after RemoveCustomPrompt() = %q, want empty", prompt)
	}
}

func TestPromptManager_WithCustomConfig(t *testing.T) {
	// Create a custom store
	store := prompts.Load()
	provider := prompts.NewTemplateProvider(store)

	// Add a custom prompt to the provider
	provider.RegisterCustomPrompt("test_task", "small", "Test prompt")

	// Create PromptManager with custom provider
	cfg := &PromptManagerConfig{
		DefaultModelParams: 3_000_000_000, // 3B
		CustomProvider:     provider,
	}
	pm := NewPromptManagerWithConfig(cfg)

	// Verify the custom prompt is accessible
	prompt := pm.GetOptimizedPrompt("test_task", 0) // Use default (3B)
	if prompt != "Test prompt" {
		t.Errorf("GetOptimizedPrompt() with custom config = %q, want %q",
			prompt, "Test prompt")
	}
}

func TestPromptManager_ListTasks(t *testing.T) {
	pm := NewPromptManager()

	tasks := pm.ListTasks()
	if len(tasks) == 0 {
		t.Fatal("ListTasks() returned no tasks")
	}

	// Verify all tasks can be retrieved
	for _, task := range tasks {
		if !pm.HasTask(task) {
			t.Errorf("HasTask(%q) returned false but task is in ListTasks()", task)
		}

		prompt := pm.GetOptimizedPrompt(task, 7_000_000_000)
		if prompt == "" {
			t.Errorf("GetOptimizedPrompt(%q) returned empty", task)
		}
	}
}

func TestPromptManager_GetTiers(t *testing.T) {
	pm := NewPromptManager()

	// Test with existing task
	tiers := pm.GetTiers("terminal_error_diagnosis")
	if len(tiers) == 0 {
		t.Fatal("GetTiers() returned no tiers for existing task")
	}

	// Verify all tiers can be retrieved
	for _, tier := range tiers {
		prompt := pm.GetPromptForTier("terminal_error_diagnosis", tier)
		if prompt == "" {
			t.Errorf("GetPromptForTier(terminal_error_diagnosis, %q) returned empty", tier)
		}
	}
}

func TestPromptManager_GetOptimizedPromptForTier(t *testing.T) {
	pm := NewPromptManager()

	task := "terminal_error_diagnosis"

	tests := []struct {
		name      string
		tier      ModelTier
		wantEmpty bool
	}{
		{
			name:      "local tier",
			tier:      TierLocal,
			wantEmpty: false,
		},
		{
			name:      "mid tier",
			tier:      TierMid,
			wantEmpty: false,
		},
		{
			name:      "advanced tier",
			tier:      TierAdvanced,
			wantEmpty: false,
		},
		{
			name:      "frontier tier",
			tier:      TierFrontier,
			wantEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := pm.GetOptimizedPromptForTier(task, tt.tier)
			if tt.wantEmpty && prompt != "" {
				t.Errorf("GetOptimizedPromptForTier() = %q, want empty", prompt)
			}
			if !tt.wantEmpty && prompt == "" {
				t.Error("GetOptimizedPromptForTier() returned empty, want non-empty")
			}
		})
	}
}
