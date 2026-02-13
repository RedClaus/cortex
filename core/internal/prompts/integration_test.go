package prompts

import (
	"testing"
)

func TestTemplateProvider_GetSystemPrompt(t *testing.T) {
	store := Load()
	provider := NewTemplateProvider(store)

	tests := []struct {
		name        string
		task        string
		modelParams int64
		wantTier    string
	}{
		{
			name:        "small model uses small tier",
			task:        "terminal_error_diagnosis",
			modelParams: 7_000_000_000, // 7B
			wantTier:    "small",
		},
		{
			name:        "large model uses large tier",
			task:        "terminal_error_diagnosis",
			modelParams: 70_000_000_000, // 70B
			wantTier:    "large",
		},
		{
			name:        "boundary case - 13B uses small",
			task:        "command_suggestion",
			modelParams: 13_000_000_000, // 13B
			wantTier:    "small",
		},
		{
			name:        "boundary case - 14B uses large",
			task:        "command_suggestion",
			modelParams: 14_000_000_000, // 14B
			wantTier:    "large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := provider.GetSystemPrompt(tt.task, tt.modelParams)
			if prompt == "" {
				t.Errorf("GetSystemPrompt() returned empty prompt for task=%s params=%d",
					tt.task, tt.modelParams)
			}

			// Verify we can get the same prompt using GetPromptTemplate
			directPrompt := provider.GetPromptTemplate(tt.task, tt.wantTier)
			if prompt != directPrompt {
				t.Errorf("GetSystemPrompt() and GetPromptTemplate() returned different prompts\nGot: %s\nWant: %s",
					prompt, directPrompt)
			}
		})
	}
}

func TestTemplateProvider_ListTasks(t *testing.T) {
	store := Load()
	provider := NewTemplateProvider(store)

	tasks := provider.ListTasks()
	if len(tasks) == 0 {
		t.Fatal("ListTasks() returned no tasks")
	}

	// Check that tasks are sorted
	for i := 1; i < len(tasks); i++ {
		if tasks[i-1] >= tasks[i] {
			t.Errorf("ListTasks() returned unsorted tasks: %v", tasks)
			break
		}
	}

	// Check that all tasks can be retrieved
	for _, task := range tasks {
		if !provider.HasTask(task) {
			t.Errorf("HasTask(%q) returned false but task is in ListTasks()", task)
		}

		prompt := provider.GetSystemPrompt(task, 7_000_000_000)
		if prompt == "" {
			t.Errorf("GetSystemPrompt(%q) returned empty prompt", task)
		}
	}
}

func TestTemplateProvider_RegisterCustomPrompt(t *testing.T) {
	store := Load()
	provider := NewTemplateProvider(store)

	customTask := "custom_test_task"
	customPrompt := "This is a custom test prompt"

	// Register custom prompt
	provider.RegisterCustomPrompt(customTask, "small", customPrompt)

	// Verify it's retrievable
	prompt := provider.GetSystemPrompt(customTask, 7_000_000_000)
	if prompt != customPrompt {
		t.Errorf("GetSystemPrompt() after RegisterCustomPrompt() = %q, want %q", prompt, customPrompt)
	}

	// Verify it appears in ListTasks
	tasks := provider.ListTasks()
	found := false
	for _, task := range tasks {
		if task == customTask {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Custom task %q not found in ListTasks()", customTask)
	}

	// Remove and verify it's gone
	provider.RemoveCustomPrompt(customTask, "small")
	prompt = provider.GetSystemPrompt(customTask, 7_000_000_000)
	if prompt != "" {
		t.Errorf("GetSystemPrompt() after RemoveCustomPrompt() = %q, want empty", prompt)
	}
}

func TestTemplateProvider_GetTiers(t *testing.T) {
	store := Load()
	provider := NewTemplateProvider(store)

	// Test existing task
	tiers := provider.GetTiers("terminal_error_diagnosis")
	if len(tiers) == 0 {
		t.Fatal("GetTiers() returned no tiers for existing task")
	}

	// Check that tiers are sorted
	for i := 1; i < len(tiers); i++ {
		if tiers[i-1] >= tiers[i] {
			t.Errorf("GetTiers() returned unsorted tiers: %v", tiers)
			break
		}
	}

	// Test non-existent task
	tiers = provider.GetTiers("nonexistent_task")
	if len(tiers) != 0 {
		t.Errorf("GetTiers() for nonexistent task returned %v, want empty slice", tiers)
	}
}

func TestTemplateProvider_ClearCustomPrompts(t *testing.T) {
	store := Load()
	provider := NewTemplateProvider(store)

	// Add some custom prompts
	provider.RegisterCustomPrompt("custom1", "small", "prompt1")
	provider.RegisterCustomPrompt("custom2", "large", "prompt2")

	// Clear all custom prompts
	provider.ClearCustomPrompts()

	// Verify they're gone
	if provider.GetSystemPrompt("custom1", 7_000_000_000) != "" {
		t.Error("Custom prompt still exists after ClearCustomPrompts()")
	}
	if provider.GetSystemPrompt("custom2", 70_000_000_000) != "" {
		t.Error("Custom prompt still exists after ClearCustomPrompts()")
	}
}

func TestTemplateProvider_CustomPromptPrecedence(t *testing.T) {
	store := Load()
	provider := NewTemplateProvider(store)

	// Get original prompt
	task := "terminal_error_diagnosis"
	originalPrompt := provider.GetSystemPrompt(task, 7_000_000_000)

	// Register custom prompt for same task
	customPrompt := "CUSTOM OVERRIDE PROMPT"
	provider.RegisterCustomPrompt(task, "small", customPrompt)

	// Verify custom prompt takes precedence
	prompt := provider.GetSystemPrompt(task, 7_000_000_000)
	if prompt != customPrompt {
		t.Errorf("Custom prompt did not take precedence. Got %q, want %q", prompt, customPrompt)
	}

	// Remove custom prompt
	provider.RemoveCustomPrompt(task, "small")

	// Verify we fall back to original
	prompt = provider.GetSystemPrompt(task, 7_000_000_000)
	if prompt != originalPrompt {
		t.Errorf("After removing custom prompt, did not fall back to original. Got %q, want %q",
			prompt, originalPrompt)
	}
}
