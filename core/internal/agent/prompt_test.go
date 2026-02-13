package agent

import (
	"strings"
	"testing"

	"github.com/normanking/cortex/internal/prompts"
)

func TestPromptomatixIntegration(t *testing.T) {
	// Verify that Promptomatix store loads correctly
	store := prompts.Load()

	// Check that agentic_tool_use exists
	if !store.Has("agentic_tool_use") {
		t.Fatal("Promptomatix store missing 'agentic_tool_use' task")
	}

	// Get the small tier prompt (used for local models)
	smallPrompt := store.GetTier("agentic_tool_use", "small")
	if smallPrompt == "" {
		t.Fatal("Promptomatix 'agentic_tool_use' missing 'small' tier")
	}

	// Verify key elements of the optimized prompt
	expectedElements := []string{
		"CRITICAL",
		"<tool>",
		"</tool>",
		"<params>",
		"run_command",
		"list_directory",
		"read_file",
		"NEVER just describe",
	}

	for _, elem := range expectedElements {
		if !strings.Contains(smallPrompt, elem) {
			t.Errorf("Promptomatix small prompt missing expected element: %q", elem)
		}
	}

	// Get the large tier prompt (used for cloud models)
	largePrompt := store.GetTier("agentic_tool_use", "large")
	if largePrompt == "" {
		t.Fatal("Promptomatix 'agentic_tool_use' missing 'large' tier")
	}

	// Large prompts should be shorter (more concise)
	if len(largePrompt) >= len(smallPrompt) {
		t.Logf("Warning: large prompt (%d chars) not shorter than small prompt (%d chars)",
			len(largePrompt), len(smallPrompt))
	}
}

func TestSystemPromptFullUsesPromptomatix(t *testing.T) {
	// Get the system prompt
	prompt := SystemPromptFull("/test/dir", nil, false)

	// Should contain Promptomatix-specific content (EXECUTE, not just describe)
	// Note: Identity ("You are Cortex") is now injected separately via persona
	// Promptomatix handles TASK instructions, not IDENTITY
	if !strings.Contains(prompt, "EXECUTE") && !strings.Contains(prompt, "tool") {
		t.Error("SystemPromptFull should use Promptomatix task instructions")
	}

	// Should contain the working directory
	if !strings.Contains(prompt, "/test/dir") {
		t.Error("SystemPromptFull should include working directory")
	}

	// Should contain tool format
	if !strings.Contains(prompt, "<tool>") {
		t.Error("SystemPromptFull should include tool format from Promptomatix")
	}
}

func TestSystemPromptForTierSelectsCorrectTier(t *testing.T) {
	// Test small model tier (< 14B params)
	smallPrompt := SystemPromptForTier("/test", nil, false, 7_000_000_000) // 7B
	if !strings.Contains(smallPrompt, "CRITICAL") && !strings.Contains(smallPrompt, "EXECUTE") {
		t.Error("Small tier prompt should contain verbose task instructions")
	}

	// Test large model tier (>= 14B params)
	// Note: Identity is now injected separately via persona, not Promptomatix
	largePrompt := SystemPromptForTier("/test", nil, false, 70_000_000_000) // 70B
	if !strings.Contains(largePrompt, "tool") && !strings.Contains(largePrompt, "Execute") {
		t.Error("Large tier prompt should contain tool instructions")
	}

	// Large prompt should generally be shorter
	if len(largePrompt) >= len(smallPrompt) {
		t.Logf("Note: large prompt (%d chars) not shorter than small (%d chars)",
			len(largePrompt), len(smallPrompt))
	}
}

func TestSystemPromptWithPersona(t *testing.T) {
	// Test without persona - should use default "You are Cortex" identity
	promptNoPersona := SystemPromptWithPersona("/test", nil, false, "", "")
	if !strings.Contains(promptNoPersona, "You are Cortex") {
		t.Error("Prompt without persona should have default Cortex identity")
	}
	if !strings.Contains(promptNoPersona, "## IDENTITY") {
		t.Error("Prompt should have IDENTITY section")
	}

	// Test with custom persona - should use the custom identity
	customPersona := "You are Hannah, a friendly AI assistant who loves tea."
	promptWithPersona := SystemPromptWithPersona("/test", nil, false, "", customPersona)
	if !strings.Contains(promptWithPersona, "Hannah") {
		t.Error("Prompt with persona should contain persona name")
	}
	if strings.Contains(promptWithPersona, "You are Cortex") {
		t.Error("Prompt with persona should NOT contain default Cortex identity")
	}

	// Both should contain task instructions (from Promptomatix)
	if !strings.Contains(promptNoPersona, "EXECUTE") && !strings.Contains(promptNoPersona, "tool") {
		t.Error("Prompt should contain Promptomatix task instructions")
	}
	if !strings.Contains(promptWithPersona, "EXECUTE") && !strings.Contains(promptWithPersona, "tool") {
		t.Error("Prompt with persona should contain Promptomatix task instructions")
	}
}
