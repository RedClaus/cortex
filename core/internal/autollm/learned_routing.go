package autollm

import (
	"context"
)

// ═══════════════════════════════════════════════════════════════════════════════
// PHASE 2.5: LEARNED CONFIDENCE-BASED ROUTING
// ═══════════════════════════════════════════════════════════════════════════════
//
// This phase sits between Phase 2 (User Intent) and Phase 3 (Default Fast Lane).
// It adjusts routing confidence based on historical outcome data from OutcomeStore.
//
// Algorithm:
// 1. Query historical success rate for the candidate model + task type
// 2. If sample count >= MinSamplesForConfidence, adjust base confidence:
//    - High success rate (> ConfidenceBoostThreshold): boost confidence
//    - Low success rate (< ConfidencePenaltyThreshold): penalize confidence
// 3. Return adjusted confidence for routing decisions

// CalculateLearnedConfidence adjusts base confidence using historical outcomes.
// Returns a RoutingConfidence struct with base, learned, and adjusted values.
//
// Parameters:
//   - ctx: Context for cancellation
//   - baseConfidence: Heuristic-based confidence (0-1)
//   - provider: Model provider (e.g., "ollama", "anthropic")
//   - model: Model identifier
//   - taskType: Task category for lookup
//   - store: OutcomeStore for historical data (can be nil for graceful degradation)
//   - config: Thresholds and limits for confidence adjustment
//
// Returns:
//   - RoutingConfidence with base, learned, and adjusted confidence values
func CalculateLearnedConfidence(
	ctx context.Context,
	baseConfidence float64,
	provider, model, taskType string,
	store OutcomeStore,
	config LearnedRoutingConfig,
) RoutingConfidence {
	result := RoutingConfidence{
		BaseConfidence:     baseConfidence,
		LearnedConfidence:  baseConfidence, // Default to base if no learning data
		SampleCount:        0,
		AdjustedConfidence: baseConfidence,
	}

	// Graceful degradation: if no outcome store, return base confidence
	if store == nil {
		return result
	}

	// Query historical success rate for this model and task type
	successRate, sampleCount, err := store.GetModelSuccessRate(ctx, provider, model, taskType)
	if err != nil {
		// Error querying store - return base confidence
		return result
	}

	result.SampleCount = sampleCount

	// Not enough samples to make a confident adjustment
	if sampleCount < config.MinSamplesForConfidence {
		return result
	}

	// Calculate confidence adjustment based on success rate
	adjustment := calculateConfidenceAdjustment(successRate, config)

	// Apply adjustment with bounds
	adjustedConfidence := baseConfidence + adjustment
	adjustedConfidence = clampConfidence(adjustedConfidence)

	result.LearnedConfidence = successRate
	result.AdjustedConfidence = adjustedConfidence

	return result
}

// calculateConfidenceAdjustment determines how much to adjust base confidence.
// Returns a value in the range [-MaxConfidenceAdjustment, +MaxConfidenceAdjustment].
func calculateConfidenceAdjustment(successRate float64, config LearnedRoutingConfig) float64 {
	// High success rate: boost confidence proportionally
	if successRate > config.ConfidenceBoostThreshold {
		// Scale boost: 0 at threshold, max at 1.0
		boostRange := 1.0 - config.ConfidenceBoostThreshold
		if boostRange <= 0 {
			return config.MaxConfidenceAdjustment
		}
		boostFactor := (successRate - config.ConfidenceBoostThreshold) / boostRange
		return boostFactor * config.MaxConfidenceAdjustment
	}

	// Low success rate: penalize confidence proportionally
	if successRate < config.ConfidencePenaltyThreshold {
		// Scale penalty: 0 at threshold, -max at 0.0
		if config.ConfidencePenaltyThreshold <= 0 {
			return -config.MaxConfidenceAdjustment
		}
		penaltyFactor := (config.ConfidencePenaltyThreshold - successRate) / config.ConfidencePenaltyThreshold
		return -penaltyFactor * config.MaxConfidenceAdjustment
	}

	// Success rate in neutral zone: no adjustment
	return 0
}

// clampConfidence ensures confidence stays within valid range [0, 1].
func clampConfidence(confidence float64) float64 {
	if confidence < 0 {
		return 0
	}
	if confidence > 1 {
		return 1
	}
	return confidence
}

// ═══════════════════════════════════════════════════════════════════════════════
// LANE-LEVEL CONFIDENCE
// ═══════════════════════════════════════════════════════════════════════════════

// CalculateLaneConfidence returns confidence for routing to a specific lane.
// This is used when deciding between Fast and Smart lanes based on historical data.
func CalculateLaneConfidence(
	ctx context.Context,
	baseLaneConfidence float64,
	lane Lane,
	taskType string,
	store OutcomeStore,
	config LearnedRoutingConfig,
) RoutingConfidence {
	result := RoutingConfidence{
		BaseConfidence:     baseLaneConfidence,
		LearnedConfidence:  baseLaneConfidence,
		SampleCount:        0,
		AdjustedConfidence: baseLaneConfidence,
	}

	if store == nil {
		return result
	}

	successRate, sampleCount, err := store.GetLaneSuccessRate(ctx, string(lane), taskType)
	if err != nil {
		return result
	}

	result.SampleCount = sampleCount

	if sampleCount < config.MinSamplesForConfidence {
		return result
	}

	adjustment := calculateConfidenceAdjustment(successRate, config)
	adjustedConfidence := baseLaneConfidence + adjustment
	adjustedConfidence = clampConfidence(adjustedConfidence)

	result.LearnedConfidence = successRate
	result.AdjustedConfidence = adjustedConfidence

	return result
}

// ═══════════════════════════════════════════════════════════════════════════════
// MODEL SCORING WITH LEARNED CONFIDENCE
// ═══════════════════════════════════════════════════════════════════════════════

// ShouldPreferModel returns true if learned confidence suggests preferring
// the given model over alternatives for the specified task type.
//
// This is used in Phase 2.5 to potentially prefer a model that has shown
// consistently good performance for specific task types.
func ShouldPreferModel(
	ctx context.Context,
	provider, model, taskType string,
	store OutcomeStore,
	config LearnedRoutingConfig,
) (prefer bool, confidence RoutingConfidence) {
	// Calculate learned confidence with a base of 0.5 (neutral)
	confidence = CalculateLearnedConfidence(ctx, 0.5, provider, model, taskType, store, config)

	// Prefer if:
	// 1. We have enough samples
	// 2. Adjusted confidence is significantly above neutral (0.5 + half of max adjustment)
	preferThreshold := 0.5 + (config.MaxConfidenceAdjustment / 2)
	prefer = confidence.SampleCount >= config.MinSamplesForConfidence &&
		confidence.AdjustedConfidence >= preferThreshold

	return prefer, confidence
}

// ShouldAvoidModel returns true if learned confidence suggests avoiding
// the given model for the specified task type due to poor historical performance.
func ShouldAvoidModel(
	ctx context.Context,
	provider, model, taskType string,
	store OutcomeStore,
	config LearnedRoutingConfig,
) (avoid bool, confidence RoutingConfidence) {
	confidence = CalculateLearnedConfidence(ctx, 0.5, provider, model, taskType, store, config)

	// Avoid if:
	// 1. We have enough samples
	// 2. Adjusted confidence is significantly below neutral (0.5 - half of max adjustment)
	avoidThreshold := 0.5 - (config.MaxConfidenceAdjustment / 2)
	avoid = confidence.SampleCount >= config.MinSamplesForConfidence &&
		confidence.AdjustedConfidence <= avoidThreshold

	return avoid, confidence
}
