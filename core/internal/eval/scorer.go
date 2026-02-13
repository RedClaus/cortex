package eval

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ═══════════════════════════════════════════════════════════════════════════════
// CAPABILITY SCORER
// Unified LLM capability scoring with registry lookup and heuristic fallback.
// Principle: LOOKUP, DON'T COMPUTE - No benchmarking, no API calls.
// ═══════════════════════════════════════════════════════════════════════════════

// CapabilityScorer provides model capability scoring.
type CapabilityScorer struct {
	registry ModelRegistry
}

// NewCapabilityScorer creates a scorer with the default registry.
func NewCapabilityScorer() *CapabilityScorer {
	return &CapabilityScorer{
		registry: DefaultRegistry(),
	}
}

// NewCapabilityScorerWithRegistry creates a scorer with a custom registry.
func NewCapabilityScorerWithRegistry(reg ModelRegistry) *CapabilityScorer {
	return &CapabilityScorer{
		registry: reg,
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// MAIN API
// ═══════════════════════════════════════════════════════════════════════════════

// Score returns the capability score for a model.
// Uses registry lookup first, falls back to heuristics for unknown models.
func (s *CapabilityScorer) Score(provider, model string) *UnifiedCapabilityScore {
	// Try registry lookup
	if cap, ok := s.registry.Get(provider, model); ok {
		return &cap.Score
	}

	// Fall back to heuristics
	return s.heuristicScore(provider, model, 0)
}

// ScoreWithSize returns the score, using size hint for better heuristics.
func (s *CapabilityScorer) ScoreWithSize(provider, model string, sizeBytes int64) *UnifiedCapabilityScore {
	// Try registry lookup
	if cap, ok := s.registry.Get(provider, model); ok {
		return &cap.Score
	}

	// Fall back to heuristics with size info
	return s.heuristicScore(provider, model, sizeBytes)
}

// GetCapabilities returns the full capability info for a model.
// Returns nil if the model is not in the registry.
func (s *CapabilityScorer) GetCapabilities(provider, model string) *ModelCapability {
	if cap, ok := s.registry.Get(provider, model); ok {
		return cap
	}

	// For unknown models, build a capability from heuristics
	score := s.heuristicScore(provider, model, 0)
	return &ModelCapability{
		ID:          provider + "/" + model,
		Provider:    provider,
		Model:       model,
		DisplayName: model,
		Tier:        TierFromScore(score.Overall),
		Score:       *score,
		Capabilities: CapabilityFlags{
			Vision:          false,
			FunctionCalling: false,
			JSONMode:        true,
			Streaming:       true,
			SystemPrompt:    true,
		},
		ContextWindow: 8192,
	}
}

// GetCapabilitiesWithSize returns capability info with size-based heuristics.
func (s *CapabilityScorer) GetCapabilitiesWithSize(provider, model string, sizeBytes int64) *ModelCapability {
	if cap, ok := s.registry.Get(provider, model); ok {
		return cap
	}

	score := s.heuristicScore(provider, model, sizeBytes)
	return &ModelCapability{
		ID:          provider + "/" + model,
		Provider:    provider,
		Model:       model,
		DisplayName: model,
		Tier:        TierFromScore(score.Overall),
		Score:       *score,
		Capabilities: CapabilityFlags{
			Vision:          false,
			FunctionCalling: false,
			JSONMode:        true,
			Streaming:       true,
			SystemPrompt:    true,
		},
		ContextWindow: estimateContextWindow(model, sizeBytes),
	}
}

// DetectProvider infers the provider from a model ID.
func (s *CapabilityScorer) DetectProvider(modelID string) string {
	return s.registry.DetectProvider(modelID)
}

// ListModels returns all models in the registry.
func (s *CapabilityScorer) ListModels(provider string) []*ModelCapability {
	return s.registry.List(provider)
}

// ═══════════════════════════════════════════════════════════════════════════════
// HEURISTIC SCORING
// Used for models not in the registry.
// ═══════════════════════════════════════════════════════════════════════════════

var (
	// Pattern to extract parameter count from model name
	paramPattern = regexp.MustCompile(`(\d+\.?\d*)b`, )
)

// heuristicScore generates an estimated score for unknown models.
func (s *CapabilityScorer) heuristicScore(provider, model string, sizeBytes int64) *UnifiedCapabilityScore {
	score := &UnifiedCapabilityScore{
		Confidence: 0.50, // Lower confidence for heuristic
		Source:     ScoreSourceHeuristic,
	}

	// Start with tier-based estimation
	tier := ClassifyModelTier(provider, model)
	baseScore := tierToBaseScore(tier)
	score.Overall = baseScore

	// Extract parameter count from model name
	params := extractParams(model)
	if params == 0 && sizeBytes > 0 {
		// Estimate params from size (rough: 2 bytes per param for Q8)
		params = float64(sizeBytes) / (2 * 1024 * 1024 * 1024)
	}

	// Adjust score based on parameter count
	if params > 0 {
		score.Overall = paramsToScore(params)
	}

	// Apply family-specific adjustments
	modelLower := strings.ToLower(model)
	score = applyFamilyBonus(score, modelLower)

	// Apply quantization penalty
	score = applyQuantizationPenalty(score, modelLower)

	// Clamp to valid range
	score.Overall = clamp(score.Overall, 10, 95)

	// Set sub-scores based on overall
	score.Reasoning = score.Overall
	score.Coding = score.Overall
	score.Instruction = score.Overall
	score.Speed = estimateSpeed(params, modelLower)

	// Apply coding bonus for code-focused models
	if isCodingModel(modelLower) {
		score.Coding = clamp(score.Coding+12, 10, 95)
		score.Reasoning = clamp(score.Reasoning-5, 10, 95)
	}

	// Apply reasoning bonus for reasoning-focused models
	if isReasoningModel(modelLower) {
		score.Reasoning = clamp(score.Reasoning+15, 10, 95)
	}

	return score
}

// tierToBaseScore converts a tier to a base score.
func tierToBaseScore(tier ModelTier) int {
	scores := map[ModelTier]int{
		TierSmall:    30,
		TierMedium:   48,
		TierLarge:    65,
		TierXL:       80,
		TierFrontier: 92,
	}
	if s, ok := scores[tier]; ok {
		return s
	}
	return 48
}

// paramsToScore converts parameter count (billions) to a score.
func paramsToScore(params float64) int {
	switch {
	case params >= 70:
		return 82
	case params >= 30:
		return 75
	case params >= 20:
		return 70
	case params >= 13:
		return 62
	case params >= 7:
		return 52
	case params >= 3:
		return 40
	case params >= 1:
		return 30
	default:
		return 25
	}
}

// extractParams extracts parameter count from model name.
func extractParams(model string) float64 {
	matches := paramPattern.FindStringSubmatch(strings.ToLower(model))
	if len(matches) >= 2 {
		if p, err := strconv.ParseFloat(matches[1], 64); err == nil {
			return p
		}
	}
	return 0
}

// applyFamilyBonus applies bonuses for well-known model families.
func applyFamilyBonus(score *UnifiedCapabilityScore, model string) *UnifiedCapabilityScore {
	bonuses := map[string]int{
		"llama3":    3, // Llama 3 series is generally strong
		"llama3.1":  5, // Llama 3.1 even better
		"llama3.2":  4,
		"qwen2.5":   4, // Qwen 2.5 series is competitive
		"mistral":   2, // Mistral generally solid
		"mixtral":   5, // MoE models punch above weight
		"deepseek":  3,
		"gemma2":    4, // Gemma 2 improved over 1
		"phi3":      6, // Phi-3 punches above weight class
	}

	for family, bonus := range bonuses {
		if strings.Contains(model, family) {
			score.Overall = clamp(score.Overall+bonus, 10, 95)
			break
		}
	}

	return score
}

// applyQuantizationPenalty reduces score for quantized models.
func applyQuantizationPenalty(score *UnifiedCapabilityScore, model string) *UnifiedCapabilityScore {
	penalties := map[string]int{
		"q2":   -20, // Very low quality
		"q3":   -15,
		"q4":   -10, // Common, moderate quality loss
		"q5":   -6,
		"q6":   -4,
		"q8":   -2, // High quality
		"2bit": -20,
		"4bit": -10,
		"8bit": -2,
	}

	for pattern, penalty := range penalties {
		if strings.Contains(model, pattern) {
			score.Overall = clamp(score.Overall+penalty, 10, 95)
			score.Confidence *= 0.9 // Lower confidence for quantized
			break
		}
	}

	return score
}

// estimateSpeed estimates relative speed (0-100) based on size.
func estimateSpeed(params float64, model string) int {
	// Smaller = faster
	if params == 0 {
		return 80 // Unknown, assume medium
	}

	switch {
	case params >= 70:
		return 25
	case params >= 30:
		return 40
	case params >= 13:
		return 60
	case params >= 7:
		return 80
	case params >= 3:
		return 92
	default:
		return 98
	}
}

// isCodingModel checks if model is code-focused.
func isCodingModel(model string) bool {
	patterns := []string{"code", "coder", "codellama", "starcoder", "codestral"}
	for _, p := range patterns {
		if strings.Contains(model, p) {
			return true
		}
	}
	return false
}

// isReasoningModel checks if model is reasoning-focused.
func isReasoningModel(model string) bool {
	patterns := []string{"deepseek-r1", "o1", "reasoning"}
	for _, p := range patterns {
		if strings.Contains(model, p) {
			return true
		}
	}
	return false
}

// estimateContextWindow estimates context based on model info.
func estimateContextWindow(model string, sizeBytes int64) int {
	modelLower := strings.ToLower(model)

	// Known context windows by family
	if strings.Contains(modelLower, "llama3.1") || strings.Contains(modelLower, "llama3.2") {
		return 128000
	}
	if strings.Contains(modelLower, "mixtral") {
		return 32768
	}
	if strings.Contains(modelLower, "qwen2.5") {
		return 32768
	}
	if strings.Contains(modelLower, "command-r") {
		return 128000
	}

	// Default based on size
	if sizeBytes > 30*1024*1024*1024 {
		return 8192
	}
	return 4096
}

// clamp constrains a value to a range.
func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// ═══════════════════════════════════════════════════════════════════════════════
// COMPARISON AND RECOMMENDATION
// ═══════════════════════════════════════════════════════════════════════════════

// CompareModels compares two models.
// Returns -1 if a < b, 0 if equal, 1 if a > b.
func (s *CapabilityScorer) CompareModels(providerA, modelA, providerB, modelB string) int {
	scoreA := s.Score(providerA, modelA).Overall
	scoreB := s.Score(providerB, modelB).Overall

	switch {
	case scoreA < scoreB:
		return -1
	case scoreA > scoreB:
		return 1
	default:
		return 0
	}
}

// RecommendForComplexity suggests models for a given complexity score (0-100).
func (s *CapabilityScorer) RecommendForComplexity(complexity int, preferLocal bool) []*ModelCapability {
	// Determine minimum score needed
	minScore := complexity - 10 // Some headroom
	if minScore < 20 {
		minScore = 20
	}

	var candidates []*ModelCapability

	// Get all models
	all := s.registry.List("")
	for _, cap := range all {
		if cap.Score.Overall >= minScore {
			if preferLocal && cap.Provider != "ollama" {
				continue
			}
			candidates = append(candidates, cap)
		}
	}

	// Sort by score (ascending) to get smallest capable model first
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score.Overall < candidates[j].Score.Overall
	})

	return candidates
}

// ═══════════════════════════════════════════════════════════════════════════════
// FORMATTING HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

// FormatScore returns a human-readable score string.
func FormatScore(score int) string {
	switch {
	case score >= 90:
		return "Expert"
	case score >= 76:
		return "Advanced"
	case score >= 56:
		return "Strong"
	case score >= 36:
		return "Moderate"
	default:
		return "Basic"
	}
}

// FormatScoreEmoji returns an emoji for the score tier.
func FormatScoreEmoji(score int) string {
	switch {
	case score >= 90:
		return "🔴" // Frontier/Expert
	case score >= 76:
		return "🟠" // XL/Advanced
	case score >= 56:
		return "🟡" // Large/Strong
	case score >= 36:
		return "🟢" // Medium/Moderate
	default:
		return "🔵" // Small/Basic
	}
}

// FormatCapabilities returns a summary of capabilities.
func FormatCapabilities(caps CapabilityFlags) string {
	var parts []string
	if caps.Vision {
		parts = append(parts, "Vision")
	}
	if caps.FunctionCalling {
		parts = append(parts, "Tools")
	}
	if caps.JSONMode {
		parts = append(parts, "JSON")
	}
	if len(parts) == 0 {
		return "Basic"
	}
	return strings.Join(parts, ", ")
}

// FormatPricing returns a pricing summary string.
func FormatPricing(pricing *PricingInfo) string {
	if pricing == nil {
		return "Free (local)"
	}
	return "$" + formatFloat(pricing.InputPer1MTokens) + "/$" + formatFloat(pricing.OutputPer1MTokens) + " per 1M"
}

func formatFloat(f float64) string {
	if f >= 1 {
		return strconv.FormatFloat(f, 'f', 2, 64)
	}
	return strconv.FormatFloat(f, 'f', 3, 64)
}
