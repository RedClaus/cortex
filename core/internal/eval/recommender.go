package eval

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/platform"
)

// ═══════════════════════════════════════════════════════════════════════════════
// MODEL RECOMMENDER
// ═══════════════════════════════════════════════════════════════════════════════

// ModelRecommender suggests model upgrades based on capability assessment.
type ModelRecommender struct {
	availableModels []ModelInfo
	cloudProvider   string
	scorer          *CapabilityScorer
	maxModelSizeGB  float64
}

// ModelFetcher is an interface for fetching available models.
// This is implemented by the LLM package to avoid circular dependencies.
type ModelFetcher interface {
	// FetchModels returns available models for the given provider.
	FetchModels(ctx context.Context, provider, endpoint string) ([]ModelInfo, error)
}

// NewModelRecommender creates a new model recommender.
func NewModelRecommender() *ModelRecommender {
	maxModelGB := 5.0
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if platformInfo, err := platform.DetectPlatform(ctx); err == nil {
		maxModelGB = platformInfo.MaxModelGB
	}

	return &ModelRecommender{
		cloudProvider:  "anthropic",
		scorer:         NewCapabilityScorer(),
		maxModelSizeGB: maxModelGB,
	}
}

// WithCloudProvider sets the preferred cloud provider.
func (r *ModelRecommender) WithCloudProvider(provider string) *ModelRecommender {
	r.cloudProvider = provider
	return r
}

// SetAvailableModels sets the available models for recommendations.
func (r *ModelRecommender) SetAvailableModels(models []ModelInfo) {
	r.availableModels = models
}

// isModelAvailable checks if a model name exists in the available models list.
// Returns true if availableModels is empty (no validation possible).
func (r *ModelRecommender) isModelAvailable(modelName string) bool {
	// If no available models are set, skip validation (backward compatibility)
	if len(r.availableModels) == 0 {
		return true
	}

	for _, m := range r.availableModels {
		if m.Name == modelName {
			return true
		}
	}
	return false
}

// GetScorer returns the capability scorer for external use.
func (r *ModelRecommender) GetScorer() *CapabilityScorer {
	return r.scorer
}

// ScoreModel returns the capability score for a model.
func (r *ModelRecommender) ScoreModel(provider, model string) *UnifiedCapabilityScore {
	return r.scorer.Score(provider, model)
}

// GetModelCapability returns full capability info for a model.
func (r *ModelRecommender) GetModelCapability(provider, model string) *ModelCapability {
	return r.scorer.GetCapabilities(provider, model)
}

// ═══════════════════════════════════════════════════════════════════════════════
// RECOMMENDATION LOGIC
// ═══════════════════════════════════════════════════════════════════════════════

// Recommend returns a model recommendation based on the assessment.
func (r *ModelRecommender) Recommend(
	ctx context.Context,
	currentProvider string,
	currentModel string,
	assessment *Assessment,
	complexityScore int,
) *Recommendation {
	// Classify current model tier
	currentTier := ClassifyModelTier(currentProvider, currentModel)

	// Determine required tier based on issues and complexity
	requiredTier := r.determineRequiredTier(currentTier, assessment.Issues, complexityScore)

	// If current tier is sufficient, no recommendation needed
	if CompareTiers(currentTier, requiredTier) >= 0 {
		return nil
	}

	recommendedModel, recommendedProvider := r.findModelForTier(requiredTier, currentProvider)
	if recommendedModel == "" {
		if requiredTier == TierFrontier {
			recommendedModel = GetCloudModelForTier(r.cloudProvider, requiredTier)
			recommendedProvider = r.cloudProvider
		} else {
			recommendedModel = r.getFallbackModelWithRAM(requiredTier)
			recommendedProvider = "ollama"
		}
	}

	if recommendedModel == "" {
		return nil
	}

	// Build reason string
	reason := r.buildReasonString(assessment.Issues, currentTier, requiredTier, complexityScore)

	return &Recommendation{
		CurrentModel:        currentModel,
		CurrentProvider:     currentProvider,
		CurrentTier:         currentTier,
		RecommendedModel:    recommendedModel,
		RecommendedProvider: recommendedProvider,
		RecommendedTier:     requiredTier,
		Reason:              reason,
		Confidence:          assessment.Confidence,
		Issues:              assessment.Issues,
	}
}

// determineRequiredTier calculates the minimum tier needed.
func (r *ModelRecommender) determineRequiredTier(current ModelTier, issues []Issue, complexity int) ModelTier {
	required := current

	// Check for issues that warrant upgrade
	hasTimeout := false
	hasRepetition := false
	hasToolFailure := false
	hasTruncation := false
	highSeverityCount := 0

	for _, issue := range issues {
		switch issue.Type {
		case IssueTimeout:
			hasTimeout = true
		case IssueRepetition:
			hasRepetition = true
		case IssueToolFailure:
			hasToolFailure = true
		case IssueTruncation:
			hasTruncation = true
		}
		if issue.Severity == SeverityHigh {
			highSeverityCount++
		}
	}

	// Rule 1: Any issue bumps to at least next tier
	if len(issues) > 0 {
		required = MaxTier(required, NextTier(current))
	}

	// Rule 2: Repetition + high complexity → Large tier minimum
	if hasRepetition && complexity > 60 {
		required = MaxTier(required, TierLarge)
	}

	// Rule 3: Timeout + very high complexity → XL tier
	if hasTimeout && complexity > 80 {
		required = MaxTier(required, TierXL)
	}

	// Rule 4: Tool failure often indicates model can't handle structured output
	if hasToolFailure && complexity > 50 {
		required = MaxTier(required, TierLarge)
	}

	// Rule 5: Multiple high-severity issues → bump two tiers
	if highSeverityCount >= 2 {
		required = MaxTier(required, NextTier(NextTier(current)))
	}

	// Rule 6: Truncation with high complexity → needs larger context
	if hasTruncation && complexity > 70 {
		required = MaxTier(required, TierXL)
	}

	// Rule 7: Very high complexity without issues but near tier boundary
	if complexity > 85 && TierOrder(current) < TierOrder(TierLarge) {
		required = MaxTier(required, TierLarge)
	}

	return required
}

// findModelForTier finds the best available model for a given tier.
func (r *ModelRecommender) findModelForTier(tier ModelTier, currentProvider string) (string, string) {
	if tier == TierFrontier {
		model := GetCloudModelForTier(r.cloudProvider, tier)
		if model != "" {
			return model, r.cloudProvider
		}
	}

	const GB = 1024 * 1024 * 1024
	maxSizeBytes := int64(r.maxModelSizeGB * float64(GB))

	var candidates []ModelInfo
	for _, model := range r.availableModels {
		if model.Tier == tier && model.SizeBytes <= maxSizeBytes {
			candidates = append(candidates, model)
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].SizeBytes < candidates[j].SizeBytes
	})

	for _, model := range candidates {
		if model.Provider == currentProvider {
			return model.Name, model.Provider
		}
	}

	if len(candidates) > 0 {
		return candidates[0].Name, candidates[0].Provider
	}

	return "", ""
}

// buildReasonString constructs a human-readable explanation.
func (r *ModelRecommender) buildReasonString(issues []Issue, current, required ModelTier, complexity int) string {
	var parts []string

	// Add issue descriptions
	for _, issue := range issues {
		switch issue.Type {
		case IssueTimeout:
			parts = append(parts, fmt.Sprintf("response time exceeded threshold (%s)", issue.Evidence))
		case IssueRepetition:
			parts = append(parts, "repetitive output detected")
		case IssueToolFailure:
			parts = append(parts, "tool execution failure")
		case IssueTruncation:
			parts = append(parts, "response appears truncated")
		case IssueJSONError:
			parts = append(parts, "malformed JSON in response")
		}
	}

	// Add complexity context
	if complexity > 70 {
		parts = append(parts, fmt.Sprintf("high task complexity (%d/100)", complexity))
	}

	// Build final string
	if len(parts) == 0 {
		return fmt.Sprintf("Upgrade from %s to %s tier recommended for better performance",
			TierDescription(current), TierDescription(required))
	}

	return fmt.Sprintf("Issues detected: %s. Consider upgrading from %s to %s model.",
		strings.Join(parts, ", "),
		current.String(),
		required.String())
}

// ═══════════════════════════════════════════════════════════════════════════════
// QUICK RECOMMENDATION
// ═══════════════════════════════════════════════════════════════════════════════

func (r *ModelRecommender) getFallbackModelWithRAM(tier ModelTier) string {
	type modelSize struct {
		model  string
		sizeGB float64
	}
	fallbacks := map[ModelTier][]modelSize{
		TierSmall:  {{model: "llama3.2:1b", sizeGB: 1.3}},
		TierMedium: {{model: "llama3.2:3b", sizeGB: 2.0}, {model: "llama3:8b", sizeGB: 4.7}},
		TierLarge:  {{model: "qwen2.5-coder:7b", sizeGB: 4.7}, {model: "qwen2.5-coder:14b", sizeGB: 9.0}},
		TierXL:     {{model: "qwen2.5-coder:14b", sizeGB: 9.0}, {model: "qwen2.5-coder:32b", sizeGB: 18.0}},
	}

	tierOrder := []ModelTier{tier, TierLarge, TierMedium, TierSmall}

	for _, t := range tierOrder {
		candidates, ok := fallbacks[t]
		if !ok {
			continue
		}
		for _, c := range candidates {
			if c.sizeGB <= r.maxModelSizeGB && r.isModelAvailable(c.model) {
				return c.model
			}
		}
	}

	for _, m := range r.availableModels {
		sizeGB := float64(m.SizeBytes) / (1024 * 1024 * 1024)
		if sizeGB <= r.maxModelSizeGB {
			return m.Name
		}
	}

	return ""
}

// QuickRecommend generates a recommendation without full assessment.
// Useful for real-time feedback during streaming responses.
func (r *ModelRecommender) QuickRecommend(
	provider string,
	model string,
	durationMs int,
	hasTimeout bool,
	hasRepetition bool,
) *Recommendation {
	currentTier := ClassifyModelTier(provider, model)

	// Simple rules for quick recommendation
	var requiredTier ModelTier
	switch {
	case hasTimeout && hasRepetition:
		requiredTier = NextTier(NextTier(currentTier))
	case hasTimeout || hasRepetition:
		requiredTier = NextTier(currentTier)
	case durationMs > 45000: // 45 seconds
		requiredTier = NextTier(currentTier)
	default:
		return nil // No recommendation needed
	}

	if CompareTiers(currentTier, requiredTier) >= 0 {
		return nil
	}

	var recommendedModel string
	var recommendedProvider string
	if requiredTier == TierFrontier {
		recommendedProvider = r.cloudProvider
		recommendedModel = GetCloudModelForTier(r.cloudProvider, requiredTier)
	} else {
		recommendedModel = r.getFallbackModelWithRAM(requiredTier)
		recommendedProvider = "ollama"
	}

	if recommendedModel == "" {
		return nil
	}

	var issues []Issue
	if hasTimeout {
		issues = append(issues, Issue{
			Type:        IssueTimeout,
			Severity:    SeverityMedium,
			Description: "Response time exceeded threshold",
		})
	}
	if hasRepetition {
		issues = append(issues, Issue{
			Type:        IssueRepetition,
			Severity:    SeverityHigh,
			Description: "Repetitive output detected",
		})
	}

	reason := "Model performance issues detected"
	if hasRepetition {
		reason = "This model may be too small for the work you are asking it to do"
	}

	return &Recommendation{
		CurrentModel:        model,
		CurrentProvider:     provider,
		CurrentTier:         currentTier,
		RecommendedModel:    recommendedModel,
		RecommendedProvider: recommendedProvider,
		RecommendedTier:     requiredTier,
		Reason:              reason,
		Confidence:          0.7,
		Issues:              issues,
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

// ShouldWarnUser returns true if the recommendation warrants a user warning.
func ShouldWarnUser(rec *Recommendation) bool {
	if rec == nil {
		return false
	}

	// Warn if confidence is high enough
	if rec.Confidence < 0.5 {
		return false
	}

	// Warn if tier jump is significant
	tierDiff := TierOrder(rec.RecommendedTier) - TierOrder(rec.CurrentTier)
	if tierDiff >= 1 {
		return true
	}

	// Warn if any high severity issues
	for _, issue := range rec.Issues {
		if issue.Severity == SeverityHigh {
			return true
		}
	}

	return false
}

// FormatRecommendationMessage creates a user-friendly warning message.
func FormatRecommendationMessage(rec *Recommendation) string {
	if rec == nil {
		return ""
	}

	return fmt.Sprintf(
		"⚠️  Model Capability Warning\n"+
			"Current: %s (%s)\n"+
			"Suggested: %s (%s)\n"+
			"Reason: %s\n\n"+
			"Use /model to switch models",
		rec.CurrentModel,
		rec.CurrentTier.String(),
		rec.RecommendedModel,
		rec.RecommendedTier.String(),
		rec.Reason,
	)
}
