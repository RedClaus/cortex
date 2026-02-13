package eval

import (
	"strings"
)

// ═══════════════════════════════════════════════════════════════════════════════
// MODEL TIERS
// ═══════════════════════════════════════════════════════════════════════════════

// ModelTier categorizes models by size/capability.
type ModelTier string

const (
	TierSmall    ModelTier = "small"    // < 2GB / < 3B params
	TierMedium   ModelTier = "medium"   // 2-6GB / 3-10B params
	TierLarge    ModelTier = "large"    // 6-15GB / 10-20B params
	TierXL       ModelTier = "xl"       // 15GB+ / 20B+ params
	TierFrontier ModelTier = "frontier" // Cloud frontier models
)

// String returns the string representation of the tier.
func (t ModelTier) String() string {
	return string(t)
}

// ═══════════════════════════════════════════════════════════════════════════════
// MODEL INFO
// ═══════════════════════════════════════════════════════════════════════════════

// ModelInfo contains model details for recommendations.
type ModelInfo struct {
	Provider     string                  `json:"provider"`
	Name         string                  `json:"name"`
	Tier         ModelTier               `json:"tier"`
	SizeBytes    int64                   `json:"size_bytes"`
	Score        *UnifiedCapabilityScore `json:"score,omitempty"`        // Unified 0-100 score
	Capabilities *CapabilityFlags        `json:"capabilities,omitempty"` // Boolean capability flags
}

// GetScore returns the capability score, computing from registry if needed.
func (m *ModelInfo) GetScore() *UnifiedCapabilityScore {
	if m.Score != nil {
		return m.Score
	}
	// Compute from scorer
	scorer := NewCapabilityScorer()
	return scorer.ScoreWithSize(m.Provider, m.Name, m.SizeBytes)
}

// GetCapabilities returns the capability flags, computing from registry if needed.
func (m *ModelInfo) GetCapabilities() *CapabilityFlags {
	if m.Capabilities != nil {
		return m.Capabilities
	}
	// Try to get from registry
	scorer := NewCapabilityScorer()
	cap := scorer.GetCapabilitiesWithSize(m.Provider, m.Name, m.SizeBytes)
	return &cap.Capabilities
}

// ═══════════════════════════════════════════════════════════════════════════════
// TIER CLASSIFICATION
// ═══════════════════════════════════════════════════════════════════════════════

// ClassifyModelTier determines the tier for a model based on provider and name.
func ClassifyModelTier(provider, model string) ModelTier {
	// Frontier cloud models
	if provider == "openai" || provider == "anthropic" || provider == "gemini" || provider == "grok" {
		return TierFrontier
	}

	// For Ollama, classify by known model sizes
	modelLower := strings.ToLower(model)

	// Known model mappings
	knownTiers := map[string]ModelTier{
		"llama3.2:1b":           TierSmall,
		"deepseek-coder:latest": TierSmall,
		"deepseek-coder:1b":     TierSmall,
		"phi:latest":            TierSmall,
		"phi3:mini":             TierSmall,
		"gemma:2b":              TierSmall,
		"tinyllama":             TierSmall,

		"mistral:7b":           TierMedium,
		"mistral:latest":       TierMedium,
		"llama3:8b":            TierMedium,
		"llama3:latest":        TierMedium,
		"llama3.2:3b":          TierMedium,
		"deepseek-r1:8b":       TierMedium,
		"codellama:7b":         TierMedium,
		"gemma:7b":             TierMedium,
		"dolphin3:8b":          TierMedium,
		"qwen2.5-coder:latest": TierMedium,
		"gemma3:latest":        TierMedium,

		"qwen2.5-coder:14b":        TierLarge,
		"deepseek-coder-v2:latest": TierLarge,
		"codellama:13b":            TierLarge,
		"llama2:13b":               TierLarge,
		"mixtral:8x7b":             TierLarge,

		"dolphin-mixtral:latest": TierXL,
		"llama2:70b":             TierXL,
		"codellama:34b":          TierXL,
		"mixtral:8x22b":          TierXL,
		"qwen:72b":               TierXL,
	}

	if tier, ok := knownTiers[modelLower]; ok {
		return tier
	}

	// Fallback: classify by size indicators in model name
	return classifyByNamePattern(modelLower)
}

// classifyByNamePattern attempts to classify a model by patterns in its name.
func classifyByNamePattern(model string) ModelTier {
	// Check for size indicators
	sizePatterns := map[ModelTier][]string{
		TierSmall:  {"1b", "2b", "3b", ":1b", ":2b", ":3b", "-1b", "-2b", "-3b", "mini", "tiny"},
		TierMedium: {"7b", "8b", ":7b", ":8b", "-7b", "-8b"},
		TierLarge:  {"13b", "14b", "15b", ":13b", ":14b", ":15b", "-13b", "-14b", "-15b"},
		TierXL:     {"20b", "34b", "70b", "72b", ":20b", ":34b", ":70b", ":72b", "mixtral"},
	}

	for tier, patterns := range sizePatterns {
		for _, pattern := range patterns {
			if strings.Contains(model, pattern) {
				return tier
			}
		}
	}

	// Default to medium tier if unknown
	return TierMedium
}

// ClassifyBySize classifies a model tier based on its size in bytes.
func ClassifyBySize(sizeBytes int64) ModelTier {
	const (
		GB = 1024 * 1024 * 1024
	)

	sizeGB := float64(sizeBytes) / float64(GB)

	switch {
	case sizeGB < 2:
		return TierSmall
	case sizeGB < 6:
		return TierMedium
	case sizeGB < 15:
		return TierLarge
	default:
		return TierXL
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// TIER ORDERING
// ═══════════════════════════════════════════════════════════════════════════════

// TierOrder returns the numeric order of a tier (higher = more capable).
func TierOrder(tier ModelTier) int {
	order := map[ModelTier]int{
		TierSmall:    1,
		TierMedium:   2,
		TierLarge:    3,
		TierXL:       4,
		TierFrontier: 5,
	}
	if o, ok := order[tier]; ok {
		return o
	}
	return 2 // Default to medium
}

// NextTier returns the next tier up from the given tier.
func NextTier(tier ModelTier) ModelTier {
	switch tier {
	case TierSmall:
		return TierMedium
	case TierMedium:
		return TierLarge
	case TierLarge:
		return TierXL
	case TierXL:
		return TierFrontier
	default:
		return TierFrontier
	}
}

// MaxTier returns the higher of two tiers.
func MaxTier(a, b ModelTier) ModelTier {
	if TierOrder(b) > TierOrder(a) {
		return b
	}
	return a
}

// CompareTiers returns -1 if a < b, 0 if a == b, 1 if a > b.
func CompareTiers(a, b ModelTier) int {
	orderA, orderB := TierOrder(a), TierOrder(b)
	switch {
	case orderA < orderB:
		return -1
	case orderA > orderB:
		return 1
	default:
		return 0
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// KNOWN MODELS BY TIER
// ═══════════════════════════════════════════════════════════════════════════════

// GetFallbackModel returns a known good model for the given tier.
// Note: These are suggestions - the TUI validates availability before switching.
func GetFallbackModel(tier ModelTier) string {
	fallbacks := map[ModelTier]string{
		TierSmall:    "llama3.2:1b",
		TierMedium:   "llama3.2:3b",       // More commonly installed than mistral:7b
		TierLarge:    "qwen2.5-coder:14b", // Common for coding tasks
		TierXL:       "qwen2.5-coder:32b", // More common than dolphin-mixtral
		TierFrontier: "claude-sonnet-4-20250514",
	}

	if model, ok := fallbacks[tier]; ok {
		return model
	}
	return "qwen2.5-coder:14b"
}

// GetCloudModelForTier returns the recommended cloud model for a tier.
func GetCloudModelForTier(provider string, tier ModelTier) string {
	cloudModels := map[string]map[ModelTier]string{
		"openai": {
			TierMedium:   "gpt-4o-mini",
			TierLarge:    "gpt-4o",
			TierXL:       "gpt-4o",
			TierFrontier: "gpt-4o",
		},
		"anthropic": {
			TierMedium:   "claude-3-5-haiku-20241022",
			TierLarge:    "claude-sonnet-4-20250514",
			TierXL:       "claude-sonnet-4-20250514",
			TierFrontier: "claude-opus-4-20250514",
		},
		"gemini": {
			TierMedium:   "gemini-1.5-flash",
			TierLarge:    "gemini-1.5-pro",
			TierXL:       "gemini-1.5-pro",
			TierFrontier: "gemini-1.5-pro",
		},
	}

	if providerModels, ok := cloudModels[provider]; ok {
		if model, ok := providerModels[tier]; ok {
			return model
		}
	}

	return ""
}

// ═══════════════════════════════════════════════════════════════════════════════
// TIER DESCRIPTIONS
// ═══════════════════════════════════════════════════════════════════════════════

// TierDescription returns a human-readable description of a tier.
func TierDescription(tier ModelTier) string {
	descriptions := map[ModelTier]string{
		TierSmall:    "Small (< 3B params) - Fast but limited capability",
		TierMedium:   "Medium (3-10B params) - Good balance of speed and capability",
		TierLarge:    "Large (10-20B params) - High capability, moderate speed",
		TierXL:       "XL (20B+ params) - Very high capability, slower",
		TierFrontier: "Frontier - State-of-the-art cloud models",
	}

	if desc, ok := descriptions[tier]; ok {
		return desc
	}
	return string(tier)
}
