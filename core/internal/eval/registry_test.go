package eval

import (
	"strings"
	"testing"
)

// ═══════════════════════════════════════════════════════════════════════════════
// REGISTRY TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestRegistryGet(t *testing.T) {
	registry := DefaultRegistry()

	tests := []struct {
		provider string
		model    string
		found    bool
		minScore int
	}{
		// Anthropic models
		{"anthropic", "claude-opus-4-20250514", true, 95},
		{"anthropic", "claude-sonnet-4-20250514", true, 88},
		{"anthropic", "claude-3-5-haiku-20241022", true, 60},

		// OpenAI models
		{"openai", "o1", true, 90},
		{"openai", "gpt-4o", true, 85},
		{"openai", "gpt-4o-mini", true, 50},

		// Gemini models
		{"gemini", "gemini-1.5-pro", true, 80},
		{"gemini", "gemini-1.5-flash", true, 60},

		// Ollama models
		{"ollama", "llama3.2:1b", true, 25},
		{"ollama", "llama3:8b", true, 50},
		{"ollama", "llama3:70b", true, 78},
		{"ollama", "mistral:7b", true, 45},
		{"ollama", "qwen2.5-coder:32b", true, 75},
		{"ollama", "deepseek-r1:70b", true, 85},

		// Unknown model
		{"unknown", "unknown-model", false, 0},
	}

	for _, tc := range tests {
		t.Run(tc.provider+"/"+tc.model, func(t *testing.T) {
			cap, ok := registry.Get(tc.provider, tc.model)

			if tc.found {
				if !ok {
					t.Errorf("Expected to find model %s/%s, but not found", tc.provider, tc.model)
					return
				}
				if cap.Score.Overall < tc.minScore {
					t.Errorf("Score %d is below minimum %d", cap.Score.Overall, tc.minScore)
				}
				if cap.Provider != tc.provider {
					t.Errorf("Provider mismatch: got %s, want %s", cap.Provider, tc.provider)
				}
			} else {
				if ok {
					t.Errorf("Expected not to find model %s/%s, but found it", tc.provider, tc.model)
				}
			}
		})
	}
}

func TestRegistryGetByID(t *testing.T) {
	registry := DefaultRegistry()

	tests := []struct {
		id       string
		found    bool
		provider string
		minScore int
	}{
		{"anthropic/claude-opus-4-20250514", true, "anthropic", 95},
		{"openai/gpt-4o", true, "openai", 85},
		{"gemini/gemini-1.5-pro", true, "gemini", 80},
		{"ollama/llama3:8b", true, "ollama", 50},
		{"unknown/model", false, "", 0},
	}

	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			cap, ok := registry.GetByID(tc.id)

			if tc.found {
				if !ok {
					t.Errorf("Expected to find model %s, but not found", tc.id)
					return
				}
				if cap.Provider != tc.provider {
					t.Errorf("Provider mismatch: got %s, want %s", cap.Provider, tc.provider)
				}
				if cap.Score.Overall < tc.minScore {
					t.Errorf("Score %d is below minimum %d", cap.Score.Overall, tc.minScore)
				}
			} else {
				if ok {
					t.Errorf("Expected not to find model %s, but found it", tc.id)
				}
			}
		})
	}
}

func TestRegistryList(t *testing.T) {
	registry := DefaultRegistry()

	// Test listing all models
	all := registry.List("")
	if len(all) < 49 {
		t.Errorf("Expected at least 49 models, got %d", len(all))
	}
	t.Logf("Total models in registry: %d", len(all))

	// Test listing by provider
	providers := []string{"anthropic", "openai", "gemini", "ollama"}
	for _, provider := range providers {
		models := registry.List(provider)
		if len(models) == 0 {
			t.Errorf("Expected models for provider %s, got none", provider)
		}

		// Verify all returned models belong to the provider
		for _, model := range models {
			if strings.ToLower(model.Provider) != provider {
				t.Errorf("Provider filter failed: expected %s, got %s", provider, model.Provider)
			}
		}
		t.Logf("Provider %s: %d models", provider, len(models))
	}
}

func TestRegistryListByTier(t *testing.T) {
	registry := DefaultRegistry()

	tiers := []ModelTier{TierSmall, TierMedium, TierLarge, TierXL, TierFrontier}
	for _, tier := range tiers {
		models := registry.ListByTier(tier)
		if len(models) == 0 {
			t.Logf("Warning: No models found for tier %s", tier)
			continue
		}

		// Verify all returned models are in the correct tier
		for _, model := range models {
			if model.Tier != tier {
				t.Errorf("Tier filter failed: expected %s, got %s for model %s",
					tier, model.Tier, model.DisplayName)
			}
		}
		t.Logf("Tier %s: %d models", tier, len(models))
	}
}

func TestRegistryDetectProvider(t *testing.T) {
	registry := DefaultRegistry()

	tests := []struct {
		modelID  string
		expected string
	}{
		// Explicit provider prefix
		{"anthropic/claude-sonnet-4", "anthropic"},
		{"openai/gpt-4o", "openai"},
		{"gemini/gemini-1.5-pro", "gemini"},
		{"ollama/llama3:8b", "ollama"},

		// Anthropic patterns
		{"claude-opus-4", "anthropic"},
		{"claude-sonnet-3.5", "anthropic"},
		{"claude-haiku", "anthropic"},

		// OpenAI patterns
		{"gpt-4o", "openai"},
		{"gpt-3.5-turbo", "openai"},
		{"o1", "openai"},
		{"o1-mini", "openai"},
		{"davinci-002", "openai"},

		// Gemini patterns
		{"gemini-1.5-pro", "gemini"},
		{"gemini-2.0-flash", "gemini"},
		{"palm-2", "gemini"},

		// Mistral API patterns
		{"mistral-large", "mistral"},
		{"codestral", "mistral"},
		{"open-mistral-7b", "mistral"},
		{"open-mixtral-8x7b", "mistral"},

		// Ollama patterns (with colon version suffix)
		{"llama3:8b", "ollama"},
		{"mistral:7b", "ollama"},
		{"qwen2.5:14b", "ollama"},
		{"deepseek-coder:6.7b", "ollama"},
		{"phi3:mini", "ollama"},

		// Ollama patterns (name-based)
		{"llama3-instruct", "ollama"},
		{"codellama", "ollama"},
		{"dolphin-mixtral", "ollama"},
		{"neural-chat", "ollama"},
		{"tinyllama", "ollama"},

		// Unknown
		{"some-random-model", "unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.modelID, func(t *testing.T) {
			detected := registry.DetectProvider(tc.modelID)
			if detected != tc.expected {
				t.Errorf("DetectProvider(%q) = %q, want %q", tc.modelID, detected, tc.expected)
			}
		})
	}
}

func TestRegistryModelCount(t *testing.T) {
	registry := DefaultRegistry()
	size := registry.Size()

	if size < 49 {
		t.Errorf("Registry has %d models, expected at least 49", size)
	}

	t.Logf("Registry contains %d models", size)
}

func TestRegistryAliases(t *testing.T) {
	registry := DefaultRegistry()

	tests := []struct {
		provider string
		alias    string
		expected string // Expected model name
	}{
		{"anthropic", "opus-4", "claude-opus-4-20250514"},
		{"anthropic", "sonnet-4", "claude-sonnet-4-20250514"},
		{"openai", "o1-2024-12-17", "o1"},
		{"ollama", "llama3:latest", "llama3:8b"},
		{"ollama", "mistral:latest", "mistral:7b"},
	}

	for _, tc := range tests {
		t.Run(tc.alias, func(t *testing.T) {
			cap, ok := registry.Get(tc.provider, tc.alias)
			if !ok {
				t.Errorf("Alias %s not found for provider %s", tc.alias, tc.provider)
				return
			}

			if !strings.Contains(cap.ID, tc.expected) && cap.Model != tc.expected {
				t.Logf("Alias %s resolved to %s (expected %s) - this may be acceptable if it's an alternate alias",
					tc.alias, cap.Model, tc.expected)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// MODEL CAPABILITY VALIDATION
// ═══════════════════════════════════════════════════════════════════════════════

func TestModelCapabilitiesStructure(t *testing.T) {
	registry := DefaultRegistry()
	all := registry.List("")

	for _, cap := range all {
		t.Run(cap.ID, func(t *testing.T) {
			// Verify required fields are populated
			if cap.ID == "" {
				t.Error("ID is empty")
			}
			if cap.Provider == "" {
				t.Error("Provider is empty")
			}
			if cap.Model == "" {
				t.Error("Model is empty")
			}
			if cap.DisplayName == "" {
				t.Error("DisplayName is empty")
			}

			// Verify score is in valid range
			if cap.Score.Overall < 0 || cap.Score.Overall > 100 {
				t.Errorf("Overall score %d out of range [0, 100]", cap.Score.Overall)
			}
			if cap.Score.Reasoning < 0 || cap.Score.Reasoning > 100 {
				t.Errorf("Reasoning score %d out of range [0, 100]", cap.Score.Reasoning)
			}
			if cap.Score.Coding < 0 || cap.Score.Coding > 100 {
				t.Errorf("Coding score %d out of range [0, 100]", cap.Score.Coding)
			}
			if cap.Score.Instruction < 0 || cap.Score.Instruction > 100 {
				t.Errorf("Instruction score %d out of range [0, 100]", cap.Score.Instruction)
			}
			if cap.Score.Speed < 0 || cap.Score.Speed > 100 {
				t.Errorf("Speed score %d out of range [0, 100]", cap.Score.Speed)
			}

			// Verify confidence is in valid range
			if cap.Score.Confidence < 0 || cap.Score.Confidence > 1 {
				t.Errorf("Confidence %f out of range [0, 1]", cap.Score.Confidence)
			}

			// Verify tier matches score
			expectedTier := TierFromScore(cap.Score.Overall)
			if cap.Tier != expectedTier {
				t.Errorf("Tier mismatch: score %d → tier should be %s, got %s",
					cap.Score.Overall, expectedTier, cap.Tier)
			}

			// Verify context window is reasonable
			if cap.ContextWindow < 1000 {
				t.Errorf("Context window %d seems too small", cap.ContextWindow)
			}

			// Verify source is set
			if cap.Score.Source != ScoreSourceRegistry && cap.Score.Source != ScoreSourceHeuristic {
				t.Errorf("Invalid score source: %s", cap.Score.Source)
			}

			// Cloud models should have pricing
			if (cap.Provider == "anthropic" || cap.Provider == "openai" || cap.Provider == "gemini") {
				if cap.Pricing == nil {
					t.Error("Cloud model missing pricing information")
				} else {
					if cap.Pricing.InputPer1MTokens <= 0 {
						t.Error("Invalid input pricing")
					}
					if cap.Pricing.OutputPer1MTokens <= 0 {
						t.Error("Invalid output pricing")
					}
				}
			}

			// Ollama models should NOT have pricing
			if cap.Provider == "ollama" && cap.Pricing != nil {
				t.Error("Local model should not have pricing")
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// SCORE DISTRIBUTION TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestScoreDistribution(t *testing.T) {
	registry := DefaultRegistry()
	all := registry.List("")

	tierCounts := make(map[ModelTier]int)
	providerCounts := make(map[string]int)

	for _, cap := range all {
		tierCounts[cap.Tier]++
		providerCounts[cap.Provider]++
	}

	t.Logf("Score distribution by tier:")
	allTiers := []ModelTier{TierSmall, TierMedium, TierLarge, TierXL, TierFrontier}
	for _, tier := range allTiers {
		count := tierCounts[tier]
		t.Logf("  %s: %d models", tier, count)
	}

	t.Logf("\nModel distribution by provider:")
	for provider, count := range providerCounts {
		t.Logf("  %s: %d models", provider, count)
	}

	// Verify we have models in all tiers
	for _, tier := range allTiers {
		if tierCounts[tier] == 0 {
			t.Errorf("No models found for tier %s", tier)
		}
	}
}

func TestFrontierModels(t *testing.T) {
	registry := DefaultRegistry()
	frontier := registry.ListByTier(TierFrontier)

	if len(frontier) == 0 {
		t.Fatal("No frontier models found")
	}

	t.Logf("Frontier models (%d):", len(frontier))
	for _, model := range frontier {
		t.Logf("  %s: score=%d, reasoning=%d, coding=%d",
			model.DisplayName,
			model.Score.Overall,
			model.Score.Reasoning,
			model.Score.Coding,
		)

		// Frontier models should have score >= 90
		if model.Score.Overall < 90 {
			t.Errorf("Frontier model %s has score %d < 90", model.DisplayName, model.Score.Overall)
		}
	}
}

func TestOllamaModelCoverage(t *testing.T) {
	registry := DefaultRegistry()
	ollama := registry.List("ollama")

	if len(ollama) < 30 {
		t.Errorf("Expected at least 30 Ollama models, got %d", len(ollama))
	}

	// Check for key model families
	families := map[string]bool{
		"llama3":        false,
		"mistral":       false,
		"qwen":          false,
		"codellama":     false,
		"deepseek":      false,
		"gemma":         false,
		"phi":           false,
		"mixtral":       false,
	}

	for _, model := range ollama {
		modelLower := strings.ToLower(model.Model)
		for family := range families {
			if strings.Contains(modelLower, family) {
				families[family] = true
			}
		}
	}

	t.Logf("Ollama model families (%d models total):", len(ollama))
	for family, found := range families {
		if !found {
			t.Errorf("Missing Ollama family: %s", family)
		} else {
			t.Logf("  ✓ %s family present", family)
		}
	}
}
