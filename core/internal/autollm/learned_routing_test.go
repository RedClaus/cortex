package autollm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ═══════════════════════════════════════════════════════════════════════════════
// MOCK OUTCOME STORE
// ═══════════════════════════════════════════════════════════════════════════════

// mockOutcomeStore implements OutcomeStore for testing.
type mockOutcomeStore struct {
	modelSuccessRates map[string]mockSuccessRate
	laneSuccessRates  map[string]mockSuccessRate
	recordedOutcomes  []*RoutingOutcomeRecord
}

type mockSuccessRate struct {
	rate  float64
	count int
}

func newMockOutcomeStore() *mockOutcomeStore {
	return &mockOutcomeStore{
		modelSuccessRates: make(map[string]mockSuccessRate),
		laneSuccessRates:  make(map[string]mockSuccessRate),
	}
}

func (m *mockOutcomeStore) SetModelSuccessRate(provider, model, taskType string, rate float64, count int) {
	key := provider + "/" + model + "/" + taskType
	m.modelSuccessRates[key] = mockSuccessRate{rate: rate, count: count}
}

func (m *mockOutcomeStore) SetLaneSuccessRate(lane, taskType string, rate float64, count int) {
	key := lane + "/" + taskType
	m.laneSuccessRates[key] = mockSuccessRate{rate: rate, count: count}
}

func (m *mockOutcomeStore) GetModelSuccessRate(ctx context.Context, provider, model, taskType string) (float64, int, error) {
	key := provider + "/" + model + "/" + taskType
	if sr, ok := m.modelSuccessRates[key]; ok {
		return sr.rate, sr.count, nil
	}
	return 0, 0, nil
}

func (m *mockOutcomeStore) GetLaneSuccessRate(ctx context.Context, lane, taskType string) (float64, int, error) {
	key := lane + "/" + taskType
	if sr, ok := m.laneSuccessRates[key]; ok {
		return sr.rate, sr.count, nil
	}
	return 0, 0, nil
}

func (m *mockOutcomeStore) RecordOutcome(ctx context.Context, outcome *RoutingOutcomeRecord) error {
	m.recordedOutcomes = append(m.recordedOutcomes, outcome)
	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// CONFIDENCE CALCULATION TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestCalculateLearnedConfidence(t *testing.T) {
	ctx := context.Background()
	config := DefaultLearnedRoutingConfig()

	t.Run("nil store returns base confidence", func(t *testing.T) {
		result := CalculateLearnedConfidence(ctx, 0.7, "ollama", "llama3:8b", "chat", nil, config)

		assert.Equal(t, 0.7, result.BaseConfidence)
		assert.Equal(t, 0.7, result.LearnedConfidence)
		assert.Equal(t, 0.7, result.AdjustedConfidence)
		assert.Equal(t, 0, result.SampleCount)
	})

	t.Run("insufficient samples returns base confidence", func(t *testing.T) {
		store := newMockOutcomeStore()
		store.SetModelSuccessRate("ollama", "llama3:8b", "chat", 0.9, 3) // Only 3 samples, min is 5

		result := CalculateLearnedConfidence(ctx, 0.7, "ollama", "llama3:8b", "chat", store, config)

		assert.Equal(t, 0.7, result.BaseConfidence)
		assert.Equal(t, 0.7, result.AdjustedConfidence) // Not adjusted due to low sample count
		assert.Equal(t, 3, result.SampleCount)
	})

	t.Run("high success rate boosts confidence", func(t *testing.T) {
		store := newMockOutcomeStore()
		store.SetModelSuccessRate("ollama", "llama3:8b", "chat", 0.95, 10) // 95% success, 10 samples

		result := CalculateLearnedConfidence(ctx, 0.5, "ollama", "llama3:8b", "chat", store, config)

		assert.Equal(t, 0.5, result.BaseConfidence)
		assert.Equal(t, 0.95, result.LearnedConfidence)
		assert.Greater(t, result.AdjustedConfidence, 0.5) // Should be boosted
		assert.LessOrEqual(t, result.AdjustedConfidence, 0.8) // Should not exceed base + maxAdjustment
		assert.Equal(t, 10, result.SampleCount)
	})

	t.Run("low success rate penalizes confidence", func(t *testing.T) {
		store := newMockOutcomeStore()
		store.SetModelSuccessRate("ollama", "llama3:8b", "chat", 0.2, 10) // 20% success, 10 samples

		result := CalculateLearnedConfidence(ctx, 0.5, "ollama", "llama3:8b", "chat", store, config)

		assert.Equal(t, 0.5, result.BaseConfidence)
		assert.Equal(t, 0.2, result.LearnedConfidence)
		assert.Less(t, result.AdjustedConfidence, 0.5) // Should be penalized
		assert.GreaterOrEqual(t, result.AdjustedConfidence, 0.2) // Should not go below base - maxAdjustment
		assert.Equal(t, 10, result.SampleCount)
	})

	t.Run("neutral success rate leaves confidence unchanged", func(t *testing.T) {
		store := newMockOutcomeStore()
		store.SetModelSuccessRate("ollama", "llama3:8b", "chat", 0.6, 10) // 60% is in neutral zone

		result := CalculateLearnedConfidence(ctx, 0.5, "ollama", "llama3:8b", "chat", store, config)

		assert.Equal(t, 0.5, result.BaseConfidence)
		assert.Equal(t, 0.6, result.LearnedConfidence)
		assert.Equal(t, 0.5, result.AdjustedConfidence) // No adjustment in neutral zone
		assert.Equal(t, 10, result.SampleCount)
	})
}

func TestCalculateLaneConfidence(t *testing.T) {
	ctx := context.Background()
	config := DefaultLearnedRoutingConfig()

	t.Run("fast lane with high success rate", func(t *testing.T) {
		store := newMockOutcomeStore()
		store.SetLaneSuccessRate("fast", "chat", 0.9, 20)

		result := CalculateLaneConfidence(ctx, 0.5, LaneFast, "chat", store, config)

		assert.Equal(t, 0.9, result.LearnedConfidence)
		assert.Greater(t, result.AdjustedConfidence, 0.5)
		assert.Equal(t, 20, result.SampleCount)
	})

	t.Run("smart lane with low success rate", func(t *testing.T) {
		store := newMockOutcomeStore()
		store.SetLaneSuccessRate("smart", "coding", 0.3, 15)

		result := CalculateLaneConfidence(ctx, 0.5, LaneSmart, "coding", store, config)

		assert.Equal(t, 0.3, result.LearnedConfidence)
		assert.Less(t, result.AdjustedConfidence, 0.5)
		assert.Equal(t, 15, result.SampleCount)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// MODEL PREFERENCE TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestShouldPreferModel(t *testing.T) {
	ctx := context.Background()
	config := DefaultLearnedRoutingConfig()

	t.Run("prefer model with high success rate", func(t *testing.T) {
		store := newMockOutcomeStore()
		store.SetModelSuccessRate("ollama", "qwen2.5:7b", "coding", 0.95, 20)

		prefer, confidence := ShouldPreferModel(ctx, "ollama", "qwen2.5:7b", "coding", store, config)

		assert.True(t, prefer)
		assert.Greater(t, confidence.AdjustedConfidence, 0.6)
	})

	t.Run("do not prefer model with low success rate", func(t *testing.T) {
		store := newMockOutcomeStore()
		store.SetModelSuccessRate("ollama", "llama3:8b", "coding", 0.4, 20)

		prefer, _ := ShouldPreferModel(ctx, "ollama", "llama3:8b", "coding", store, config)

		assert.False(t, prefer)
	})

	t.Run("do not prefer model with insufficient samples", func(t *testing.T) {
		store := newMockOutcomeStore()
		store.SetModelSuccessRate("ollama", "qwen2.5:7b", "coding", 0.95, 3) // Only 3 samples

		prefer, confidence := ShouldPreferModel(ctx, "ollama", "qwen2.5:7b", "coding", store, config)

		assert.False(t, prefer)
		assert.Equal(t, 3, confidence.SampleCount)
	})
}

func TestShouldAvoidModel(t *testing.T) {
	ctx := context.Background()
	config := DefaultLearnedRoutingConfig()

	t.Run("avoid model with very low success rate", func(t *testing.T) {
		store := newMockOutcomeStore()
		store.SetModelSuccessRate("ollama", "badmodel:7b", "chat", 0.15, 20)

		avoid, confidence := ShouldAvoidModel(ctx, "ollama", "badmodel:7b", "chat", store, config)

		assert.True(t, avoid)
		assert.Less(t, confidence.AdjustedConfidence, 0.4)
	})

	t.Run("do not avoid model with good success rate", func(t *testing.T) {
		store := newMockOutcomeStore()
		store.SetModelSuccessRate("ollama", "qwen2.5:7b", "chat", 0.85, 20)

		avoid, _ := ShouldAvoidModel(ctx, "ollama", "qwen2.5:7b", "chat", store, config)

		assert.False(t, avoid)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// CONFIDENCE ADJUSTMENT EDGE CASES
// ═══════════════════════════════════════════════════════════════════════════════

func TestConfidenceAdjustmentEdgeCases(t *testing.T) {
	ctx := context.Background()
	config := DefaultLearnedRoutingConfig()

	t.Run("confidence clamped to 0 minimum", func(t *testing.T) {
		store := newMockOutcomeStore()
		store.SetModelSuccessRate("ollama", "llama3:8b", "chat", 0.0, 10) // 0% success

		result := CalculateLearnedConfidence(ctx, 0.1, "ollama", "llama3:8b", "chat", store, config)

		assert.GreaterOrEqual(t, result.AdjustedConfidence, 0.0)
	})

	t.Run("confidence clamped to 1 maximum", func(t *testing.T) {
		store := newMockOutcomeStore()
		store.SetModelSuccessRate("ollama", "llama3:8b", "chat", 1.0, 10) // 100% success

		result := CalculateLearnedConfidence(ctx, 0.9, "ollama", "llama3:8b", "chat", store, config)

		assert.LessOrEqual(t, result.AdjustedConfidence, 1.0)
	})

	t.Run("handles perfect success rate at threshold boundary", func(t *testing.T) {
		store := newMockOutcomeStore()
		store.SetModelSuccessRate("ollama", "llama3:8b", "chat", 0.85, 10) // Exactly at boost threshold

		result := CalculateLearnedConfidence(ctx, 0.5, "ollama", "llama3:8b", "chat", store, config)

		// Should have minimal or no boost since it's exactly at threshold
		assert.InDelta(t, 0.5, result.AdjustedConfidence, 0.01)
	})

	t.Run("handles penalty threshold boundary", func(t *testing.T) {
		store := newMockOutcomeStore()
		store.SetModelSuccessRate("ollama", "llama3:8b", "chat", 0.4, 10) // Exactly at penalty threshold

		result := CalculateLearnedConfidence(ctx, 0.5, "ollama", "llama3:8b", "chat", store, config)

		// Should have minimal or no penalty since it's exactly at threshold
		assert.InDelta(t, 0.5, result.AdjustedConfidence, 0.01)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// DEFAULT CONFIG TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestDefaultLearnedRoutingConfig(t *testing.T) {
	config := DefaultLearnedRoutingConfig()

	assert.Equal(t, 5, config.MinSamplesForConfidence)
	assert.Equal(t, 0.85, config.ConfidenceBoostThreshold)
	assert.Equal(t, 0.4, config.ConfidencePenaltyThreshold)
	assert.Equal(t, 0.3, config.MaxConfidenceAdjustment)
	assert.Equal(t, 0.95, config.DecayFactor)
}
