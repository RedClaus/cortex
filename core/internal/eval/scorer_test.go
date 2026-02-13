package eval

import (
	"fmt"
	"testing"
)

func TestCapabilityScorer(t *testing.T) {
	scorer := NewCapabilityScorer()

	// Test known models from registry
	tests := []struct {
		provider string
		model    string
		minScore int
		maxScore int
	}{
		{"anthropic", "claude-opus-4-20250514", 95, 100},
		{"anthropic", "claude-sonnet-4-20250514", 88, 96},
		{"anthropic", "claude-3-5-haiku-20241022", 55, 70},
		{"openai", "gpt-4o", 85, 95},
		{"openai", "gpt-4o-mini", 50, 65},
		{"ollama", "llama3.2:1b", 25, 40},
		{"ollama", "mistral:7b", 45, 60},
		{"ollama", "qwen2.5-coder:14b", 65, 80},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s/%s", tc.provider, tc.model), func(t *testing.T) {
			score := scorer.Score(tc.provider, tc.model)
			if score == nil {
				t.Fatal("Score returned nil")
			}
			if score.Overall < tc.minScore || score.Overall > tc.maxScore {
				t.Errorf("Score %d not in expected range [%d, %d]",
					score.Overall, tc.minScore, tc.maxScore)
			}
		})
	}
}

func TestCapabilityScorerHeuristic(t *testing.T) {
	scorer := NewCapabilityScorer()

	// Test unknown models (should use heuristic)
	tests := []struct {
		provider string
		model    string
		minScore int
		maxScore int
	}{
		{"ollama", "unknown-model:7b", 40, 65},
		{"ollama", "unknown-model:70b", 75, 90},
		{"ollama", "unknown-model:1b", 20, 45},
	}

	for _, tc := range tests {
		t.Run(tc.model, func(t *testing.T) {
			score := scorer.Score(tc.provider, tc.model)
			if score == nil {
				t.Fatal("Score returned nil")
			}
			if score.Source != ScoreSourceHeuristic {
				t.Errorf("Expected heuristic source, got %s", score.Source)
			}
			if score.Overall < tc.minScore || score.Overall > tc.maxScore {
				t.Errorf("Heuristic score %d not in expected range [%d, %d]",
					score.Overall, tc.minScore, tc.maxScore)
			}
		})
	}
}

func TestProviderDetection(t *testing.T) {
	scorer := NewCapabilityScorer()

	tests := []struct {
		modelID  string
		expected string
	}{
		{"claude-sonnet-4", "anthropic"},
		{"gpt-4o", "openai"},
		{"o1", "openai"},
		{"gemini-1.5-pro", "gemini"},
		{"llama3:8b", "ollama"},
		{"mistral:7b", "ollama"},
		{"qwen2.5-coder:14b", "ollama"},
	}

	for _, tc := range tests {
		t.Run(tc.modelID, func(t *testing.T) {
			detected := scorer.DetectProvider(tc.modelID)
			if detected != tc.expected {
				t.Errorf("DetectProvider(%q) = %q, want %q",
					tc.modelID, detected, tc.expected)
			}
		})
	}
}

func TestRegistrySize(t *testing.T) {
	registry := DefaultRegistry()
	size := registry.Size()
	if size < 40 {
		t.Errorf("Registry has %d models, expected at least 40", size)
	}
	t.Logf("Registry contains %d models", size)
}

func TestTierFromScore(t *testing.T) {
	tests := []struct {
		score int
		tier  ModelTier
	}{
		{20, TierSmall},
		{35, TierSmall},
		{36, TierMedium},
		{55, TierMedium},
		{56, TierLarge},
		{75, TierLarge},
		{76, TierXL},
		{89, TierXL},
		{90, TierFrontier},
		{100, TierFrontier},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("score_%d", tc.score), func(t *testing.T) {
			tier := TierFromScore(tc.score)
			if tier != tc.tier {
				t.Errorf("TierFromScore(%d) = %q, want %q", tc.score, tier, tc.tier)
			}
		})
	}
}
