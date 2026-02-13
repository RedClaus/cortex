package autollm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/eval"
	"github.com/normanking/cortex/internal/platform"
)

// ═══════════════════════════════════════════════════════════════════════════════
// MODEL SELECTOR
// ═══════════════════════════════════════════════════════════════════════════════

// ModelSelector automatically picks the best models on startup based on
// availability and scoring. Prioritizes models that can meet timeout constraints.
type ModelSelector struct {
	mlxHost        string  // MLX-LM server endpoint (OpenAI-compatible)
	ollamaHost     string
	httpClient     *http.Client
	timeoutSecs    int     // Target timeout (e.g., 120 seconds)
	maxModelSizeGB float64 // Maximum model size based on system RAM

	// Dynamic inventory for runtime model discovery and scoring (optional)
	inventory *eval.DynamicInventory
}

// ModelSelection contains the auto-selected models for a session.
type ModelSelection struct {
	// Primary local model (highest score among available that can meet timeout)
	LocalModel  string
	LocalReason string

	// Fallback chain in priority order
	Fallbacks []FallbackSelection

	// All available models that were considered
	Candidates []ModelCandidate
}

// FallbackSelection represents a selected fallback provider.
type FallbackSelection struct {
	Provider string
	Model    string
	Reason   string
}

// ModelWeight represents how resource-intensive a model is.
type ModelWeight string

const (
	WeightLight  ModelWeight = "light"  // < 4GB, very fast
	WeightMedium ModelWeight = "medium" // 4-12GB, balanced
	WeightHeavy  ModelWeight = "heavy"  // > 12GB, slow but powerful
)

// ModelCandidate represents a model considered during selection.
type ModelCandidate struct {
	Name       string
	Provider   string
	Tier       eval.ModelTier
	SizeGB     float64
	Weight     ModelWeight // light/medium/heavy classification
	Score      int         // Combined score (speed + capability)
	SpeedScore int         // Higher = faster (favored)
	QualScore  int         // Higher = more capable
	Available  bool
	Reason     string // Why selected/rejected
}

// NewModelSelector creates a new model selector.
func NewModelSelector(ollamaHost string, timeoutSecs int) *ModelSelector {
	return NewModelSelectorWithMLX("", ollamaHost, timeoutSecs)
}

// NewModelSelectorWithMLX creates a model selector with explicit MLX endpoint.
func NewModelSelectorWithMLX(mlxHost, ollamaHost string, timeoutSecs int) *ModelSelector {
	if mlxHost == "" {
		mlxHost = "http://127.0.0.1:8081" // Default mlx-lm port
	}
	if ollamaHost == "" {
		ollamaHost = "http://127.0.0.1:11434"
	}
	if timeoutSecs <= 0 {
		timeoutSecs = 120
	}

	maxModelGB := 5.0
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if platformInfo, err := platform.DetectPlatform(ctx); err == nil {
		maxModelGB = platformInfo.MaxModelGB
	}

	return &ModelSelector{
		mlxHost:        mlxHost,
		ollamaHost:     ollamaHost,
		httpClient:     &http.Client{Timeout: 5 * time.Second},
		timeoutSecs:    timeoutSecs,
		maxModelSizeGB: maxModelGB,
	}
}

// SetInventory sets the dynamic inventory for improved model scoring.
// When set, the selector will use cached/online scores instead of heuristics.
func (s *ModelSelector) SetInventory(inv *eval.DynamicInventory) {
	s.inventory = inv
}

// ═══════════════════════════════════════════════════════════════════════════════
// AUTO-SELECTION
// ═══════════════════════════════════════════════════════════════════════════════

// Select queries available models and returns the best selections.
// Priority: MLX (fastest on Apple Silicon) > Ollama
// If DynamicInventory is set, it refreshes the inventory first for accurate scoring.
func (s *ModelSelector) Select(ctx context.Context) (*ModelSelection, error) {
	selection := &ModelSelection{}

	// Refresh dynamic inventory if available (for accurate scoring)
	if s.inventory != nil {
		if err := s.inventory.RefreshInventory(ctx); err != nil {
			// Log but don't fail - we can still use heuristics
			fmt.Printf("[ModelSelector] Inventory refresh failed: %v\n", err)
		}
	}

	// 1. Try MLX first (5-10x faster on Apple Silicon)
	mlxModels, mlxErr := s.queryMLXModels(ctx)
	if mlxErr == nil && len(mlxModels) > 0 {
		// MLX is available - use it
		candidates := s.scoreMLXModels(mlxModels)
		selection.Candidates = candidates

		for _, c := range candidates {
			if c.Available {
				selection.LocalModel = c.Name
				selection.LocalReason = c.Reason + " (MLX: 5-10x faster)"
				break
			}
		}

		if selection.LocalModel != "" {
			selection.Fallbacks = s.selectCloudFallbacks()
			return selection, nil
		}
	}

	// 2. Fall back to Ollama
	ollamaModels, err := s.queryOllamaModels(ctx)
	if err != nil {
		// Neither MLX nor Ollama available
		selection.LocalModel = ""
		selection.LocalReason = "No local backends available (MLX, Ollama offline)"
		selection.Fallbacks = s.selectCloudFallbacks()
		return selection, nil
	}

	{
		// Score and rank Ollama models
		candidates := s.scoreModels(ollamaModels)
		selection.Candidates = candidates

		for _, c := range candidates {
			if c.Available {
				selection.LocalModel = c.Name
				selection.LocalReason = c.Reason
				break
			}
		}

		// Fallback to any available model if nothing passes quality bar
		if selection.LocalModel == "" && len(candidates) > 0 {
			for _, c := range candidates {
				selection.LocalModel = c.Name
				selection.LocalReason = "Only available model (may not be optimal)"
				break
			}
		}
	}

	// 3. Select cloud fallbacks
	selection.Fallbacks = s.selectCloudFallbacks()

	return selection, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// MLX QUERY
// ═══════════════════════════════════════════════════════════════════════════════

// mlxModelInfo represents MLX model metadata from OpenAI-compatible API.
type mlxModelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// queryMLXModels fetches available models from MLX-LM server.
func (s *ModelSelector) queryMLXModels(ctx context.Context) ([]mlxModelInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", s.mlxHost+"/v1/models", nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []mlxModelInfo `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// scoreMLXModels evaluates and ranks MLX models.
// MLX models use HuggingFace naming (e.g., mlx-community/Llama-3.2-3B-Instruct-4bit)
func (s *ModelSelector) scoreMLXModels(models []mlxModelInfo) []ModelCandidate {
	candidates := make([]ModelCandidate, 0, len(models))

	for _, m := range models {
		nameLower := strings.ToLower(m.ID)

		// Skip if "default" placeholder (mlx-lm returns this)
		if m.ID == "default" || m.ID == "" {
			continue
		}

		// Skip vision models - these should only be used for CortexEyes, not chat
		// Vision models have "-vl", "-vision", "llava", "moondream" in their name
		if strings.Contains(nameLower, "-vl") ||
			strings.Contains(nameLower, "-vision") ||
			strings.Contains(nameLower, "llava") ||
			strings.Contains(nameLower, "moondream") ||
			strings.Contains(nameLower, "minicpm-v") {
			continue
		}

		// MLX models are typically 4-bit quantized, estimate size from name
		sizeGB := s.estimateMLXModelSize(m.ID)
		tier := s.classifyMLXModelTier(m.ID)
		weight := classifyWeight(sizeGB)

		// MLX models get a speed bonus (5-10x faster than Ollama)
		speedScore := s.calculateSpeedScore(sizeGB) + 30 // MLX bonus

		// Quality score based on model name
		qualScore := s.calculateMLXQualityScore(m.ID, tier)

		meetsQualityBar := qualScore >= MinQualityForAgentic
		exceedsRAM := sizeGB > s.maxModelSizeGB

		var combinedScore int
		var reason string

		if exceedsRAM {
			combinedScore = 0
			reason = fmt.Sprintf("Model too large (%.1fGB) for system RAM", sizeGB)
		} else if !meetsQualityBar {
			combinedScore = qualScore
			reason = "Model too small for reliable tool use"
		} else {
			baseScore := int(float64(qualScore)*0.5 + float64(speedScore)*0.5)

			switch weight {
			case WeightMedium:
				combinedScore = baseScore + 20 // Extra bonus for MLX medium models
				if strings.Contains(nameLower, "instruct") {
					reason = "MLX optimized, instruction-tuned"
				} else {
					reason = "MLX optimized, good balance"
				}
			case WeightLight:
				combinedScore = baseScore + 15
				reason = "MLX optimized, fast inference"
			case WeightHeavy:
				combinedScore = baseScore
				reason = "MLX optimized, high quality"
			}
		}

		candidates = append(candidates, ModelCandidate{
			Name:       m.ID,
			Provider:   "mlx",
			Tier:       tier,
			SizeGB:     sizeGB,
			Weight:     weight,
			Score:      combinedScore,
			SpeedScore: speedScore,
			QualScore:  qualScore,
			Available:  meetsQualityBar && !exceedsRAM,
			Reason:     reason,
		})
	}

	// Sort by score descending
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	return candidates
}

// estimateMLXModelSize estimates model size in GB from the name.
// MLX models typically include size hints like "3B", "7B", "4bit", etc.
func (s *ModelSelector) estimateMLXModelSize(name string) float64 {
	nameLower := strings.ToLower(name)

	// Check for parameter counts in name
	if strings.Contains(nameLower, "1b") || strings.Contains(nameLower, "1.5b") {
		return 1.0 // 4-bit: ~0.5-1GB
	}
	if strings.Contains(nameLower, "3b") {
		return 2.0 // 4-bit: ~1.5-2GB
	}
	if strings.Contains(nameLower, "7b") || strings.Contains(nameLower, "8b") {
		return 4.5 // 4-bit: ~4-5GB
	}
	if strings.Contains(nameLower, "13b") || strings.Contains(nameLower, "14b") {
		return 8.0 // 4-bit: ~7-9GB
	}
	if strings.Contains(nameLower, "32b") || strings.Contains(nameLower, "34b") {
		return 18.0 // 4-bit: ~16-20GB
	}
	if strings.Contains(nameLower, "70b") {
		return 40.0 // 4-bit: ~35-45GB
	}

	// Default to medium size
	return 4.0
}

// classifyMLXModelTier determines model tier from name.
func (s *ModelSelector) classifyMLXModelTier(name string) eval.ModelTier {
	nameLower := strings.ToLower(name)

	if strings.Contains(nameLower, "70b") {
		return eval.TierXL
	}
	if strings.Contains(nameLower, "32b") || strings.Contains(nameLower, "34b") ||
		strings.Contains(nameLower, "13b") || strings.Contains(nameLower, "14b") {
		return eval.TierLarge
	}
	if strings.Contains(nameLower, "7b") || strings.Contains(nameLower, "8b") ||
		strings.Contains(nameLower, "9b") {
		return eval.TierMedium
	}
	if strings.Contains(nameLower, "3b") || strings.Contains(nameLower, "4b") {
		return eval.TierSmall
	}
	if strings.Contains(nameLower, "1b") || strings.Contains(nameLower, "1.5b") {
		return eval.TierSmall
	}

	return eval.TierMedium // Default
}

// calculateMLXQualityScore scores MLX models based on name.
func (s *ModelSelector) calculateMLXQualityScore(name string, tier eval.ModelTier) int {
	nameLower := strings.ToLower(name)

	// Base score by tier
	baseScore := map[eval.ModelTier]int{
		eval.TierSmall:    35,
		eval.TierMedium:   60,
		eval.TierLarge:    80,
		eval.TierXL:       95,
		eval.TierFrontier: 100,
	}

	score := baseScore[tier]
	if score == 0 {
		score = 55
	}

	// Bonuses for known good models
	if strings.Contains(nameLower, "qwen") {
		score += 20 // Excellent tool calling
	}
	if strings.Contains(nameLower, "llama") {
		score += 15 // Strong general capability
	}
	if strings.Contains(nameLower, "mistral") {
		score += 10 // Good coding
	}
	if strings.Contains(nameLower, "instruct") {
		score += 10 // Instruction-tuned
	}
	if strings.Contains(nameLower, "coder") || strings.Contains(nameLower, "code") {
		score += 15 // Coding focused
	}

	return score
}

// ═══════════════════════════════════════════════════════════════════════════════
// OLLAMA QUERY
// ═══════════════════════════════════════════════════════════════════════════════

// ollamaModelInfo represents Ollama's model metadata.
type ollamaModelInfo struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	Details struct {
		ParameterSize string `json:"parameter_size"`
		Family        string `json:"family"`
	} `json:"details"`
}

// queryOllamaModels fetches available models from Ollama with size info.
func (s *ModelSelector) queryOllamaModels(ctx context.Context) ([]ollamaModelInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", s.ollamaHost+"/api/tags", nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Models []ollamaModelInfo `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Models, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// MODEL SCORING
// ═══════════════════════════════════════════════════════════════════════════════

// MinQualityForAgentic is the minimum quality score required for agentic tasks.
// Models below this threshold (typically <7B params) cannot reliably follow
// tool-use instructions and are excluded from automatic selection.
const MinQualityForAgentic = 50

// classifyWeight determines the weight category based on model size.
func classifyWeight(sizeGB float64) ModelWeight {
	switch {
	case sizeGB < 4:
		return WeightLight // < 4GB: tiny/small models (1b-3b)
	case sizeGB < 12:
		return WeightMedium // 4-12GB: medium models (7b-8b)
	default:
		return WeightHeavy // > 12GB: large models (14b+)
	}
}

// scoreModels evaluates and ranks available models.
// For agentic tasks, models must meet a minimum quality threshold.
// STRATEGY: Prefer MEDIUM weight models with high quality scores.
// Heavy models are penalized even if they have higher raw quality,
// because they're too slow on most hardware.
func (s *ModelSelector) scoreModels(models []ollamaModelInfo) []ModelCandidate {
	candidates := make([]ModelCandidate, 0, len(models))

	for _, m := range models {
		nameLower := strings.ToLower(m.Name)

		// CRITICAL: Skip embedding models - they don't support chat!
		if strings.Contains(nameLower, "embed") || strings.Contains(nameLower, "nomic") ||
			strings.Contains(nameLower, "mxbai") || strings.Contains(nameLower, "bge-") ||
			strings.Contains(nameLower, "e5-") || strings.Contains(nameLower, "gte-") {
			continue // Skip embedding models entirely
		}

		// Skip vision models - these should only be used for CortexEyes, not chat
		// Vision models have "-vl", "-vision", "llava", "moondream" in their name
		if strings.Contains(nameLower, "-vl") ||
			strings.Contains(nameLower, "-vision") ||
			strings.Contains(nameLower, "llava") ||
			strings.Contains(nameLower, "moondream") ||
			strings.Contains(nameLower, "minicpm-v") {
			continue // Skip vision models - use CortexEyes for vision
		}

		sizeGB := float64(m.Size) / (1024 * 1024 * 1024)
		tier := eval.ClassifyModelTier("ollama", m.Name)
		weight := classifyWeight(sizeGB)

		// Speed score: smaller = faster = higher score
		// Scale: 100 for tiny (<2GB), down to 20 for huge (>30GB)
		speedScore := s.calculateSpeedScore(sizeGB)

		// Quality score: based on tier
		qualScore := s.calculateQualityScore(tier, m.Name)

		// CRITICAL: Check minimum quality threshold for agentic tasks
		// Models below this threshold (typically <7B) cannot reliably use tools
		meetsQualityBar := qualScore >= MinQualityForAgentic

		var combinedScore int
		var suitable bool
		var reason string

		exceedsRAM := sizeGB > s.maxModelSizeGB

		if exceedsRAM {
			combinedScore = 0
			suitable = false
			reason = fmt.Sprintf("Model too large (%.1fGB) for system RAM (max %.1fGB)", sizeGB, s.maxModelSizeGB)
		} else if !meetsQualityBar {
			combinedScore = qualScore
			suitable = false
			reason = "Model too small for reliable tool use (< 7B params)"
		} else {
			// Apply weight-based scoring strategy:
			// - Medium weight models get a BONUS (best balance)
			// - Heavy models get a PENALTY (too slow for most hardware)
			// - Light models use normal scoring

			baseScore := int(float64(qualScore)*0.6 + float64(speedScore)*0.4)

			switch weight {
			case WeightMedium:
				// BONUS for medium weight - this is the sweet spot
				combinedScore = baseScore + 15
				reason = s.determineReason(m.Name, sizeGB, speedScore, qualScore, true)
				if qualScore >= 70 {
					reason = "Best balance: high quality, medium weight"
				}
			case WeightHeavy:
				// PENALTY for heavy models - too slow on most hardware
				combinedScore = baseScore - 20
				reason = "High quality but heavy - may be slow"
			case WeightLight:
				// Light models - fast but may lack capability
				combinedScore = baseScore
				reason = s.determineReason(m.Name, sizeGB, speedScore, qualScore, true)
			}

			_ = suitable // Mark as used
		}

		candidates = append(candidates, ModelCandidate{
			Name:       m.Name,
			Provider:   "ollama",
			Tier:       tier,
			SizeGB:     sizeGB,
			Weight:     weight,
			Score:      combinedScore,
			SpeedScore: speedScore,
			QualScore:  qualScore,
			Available:  meetsQualityBar && !exceedsRAM,
			Reason:     reason,
		})
	}

	// Sort by combined score descending (medium-weight high-quality models first)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	return candidates
}

// calculateSpeedScore returns a speed score based on model size.
// Smaller models get higher scores (faster inference).
func (s *ModelSelector) calculateSpeedScore(sizeGB float64) int {
	// Speed tiers based on typical inference times
	switch {
	case sizeGB < 2:
		return 100 // Tiny models: very fast
	case sizeGB < 5:
		return 85 // Small models: fast
	case sizeGB < 8:
		return 70 // Medium models: acceptable
	case sizeGB < 15:
		return 50 // Large models: borderline for timeout
	case sizeGB < 30:
		return 30 // XL models: likely to timeout
	default:
		return 15 // Huge models: will timeout
	}
}

// calculateQualityScore returns a quality score based on tier and model name.
// Qwen models receive a bonus due to superior tool-calling accuracy.
func (s *ModelSelector) calculateQualityScore(tier eval.ModelTier, name string) int {
	// Priority 1: Use dynamic inventory scores if available (online → cached → registry)
	if s.inventory != nil {
		if cap := s.inventory.GetModelScore("ollama", name); cap != nil {
			// Use the score from inventory (includes any online/cached data)
			return cap.Score.Overall
		}
	}

	// Priority 2: Fall back to heuristic scoring
	nameLower := strings.ToLower(name)

	// PENALTY for reasoning/thinking models - they use chain-of-thought
	// which makes them 3-10x slower for simple tasks
	reasoningPenalty := 0
	if strings.Contains(nameLower, "-r1") || strings.Contains(nameLower, "-o1") ||
		strings.Contains(nameLower, "deepseek-r1") || strings.Contains(nameLower, "qwq") {
		reasoningPenalty = -50 // Heavy penalty - too slow for interactive use
	}

	// PENALTY for qwen3 - uses thinking mode by default (adds 3-5x latency)
	// qwen3 is accurate but the thinking overhead makes it too slow for fast lane
	thinkingPenalty := 0
	if strings.Contains(nameLower, "qwen3") {
		thinkingPenalty = -25 // Moderate penalty - good quality but slow due to thinking
	}

	// Bonus for coding-focused models
	codingBonus := 0
	if strings.Contains(nameLower, "coder") || strings.Contains(nameLower, "code") {
		codingBonus = 15
	}

	// Bonus for Qwen models (superior tool/function calling accuracy)
	// Note: qwen3 gets this bonus but also gets thinkingPenalty which partially offsets it
	qwenBonus := 0
	if strings.Contains(nameLower, "qwen") {
		qwenBonus = 20
	}

	// Base score by tier
	baseScore := map[eval.ModelTier]int{
		eval.TierSmall:    30,
		eval.TierMedium:   55,
		eval.TierLarge:    75,
		eval.TierXL:       90,
		eval.TierFrontier: 100,
	}

	score := baseScore[tier]
	if score == 0 {
		score = 50
	}

	return score + codingBonus + qwenBonus + reasoningPenalty + thinkingPenalty
}

// determineReason explains why a model was scored as it was.
func (s *ModelSelector) determineReason(name string, sizeGB float64, speedScore, qualScore int, suitable bool) string {
	if !suitable {
		return "Too slow for timeout constraints"
	}

	nameLower := strings.ToLower(name)

	// Best picks
	if speedScore >= 70 && qualScore >= 60 {
		if strings.Contains(nameLower, "coder") {
			return "Best balance: fast coding model"
		}
		return "Best balance: fast and capable"
	}

	if speedScore >= 85 {
		return "Fastest available, good for quick tasks"
	}

	if qualScore >= 75 {
		return "High quality, may be slower"
	}

	return "Available and suitable"
}

// ═══════════════════════════════════════════════════════════════════════════════
// CLOUD FALLBACK SELECTION
// ═══════════════════════════════════════════════════════════════════════════════

// selectCloudFallbacks returns recommended fallback providers with optimal models.
// Order: Grok (primary cloud) → Anthropic → OpenAI
func (s *ModelSelector) selectCloudFallbacks() []FallbackSelection {
	return []FallbackSelection{
		{
			Provider: "grok",
			Model:    "grok-3",
			Reason:   "Primary cloud fallback, excellent reasoning",
		},
		{
			Provider: "anthropic",
			Model:    "claude-sonnet-4-20250514",
			Reason:   "Secondary fallback, excellent tool use",
		},
		{
			Provider: "openai",
			Model:    "gpt-4o",
			Reason:   "Tertiary fallback, strong all-around capability",
		},
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// PREFERRED MODEL LISTS
// ═══════════════════════════════════════════════════════════════════════════════

// GetSpeedFirstModels returns models ordered by speed with Qwen prioritized per tier.
func GetSpeedFirstModels() []string {
	return []string{
		// Tiny: Always fast
		"qwen2.5:1.5b",
		"llama3.2:1b",
		"gemma2:2b",
		"phi3:mini",
		"tinyllama:latest",

		// Small: Fast
		"qwen2.5:3b",
		"llama3.2:3b",
		"phi3:medium",

		// Medium: Qwen coder preferred for tool calling
		"qwen2.5-coder:7b",
		"qwen2.5:7b",
		"llama3:8b",
		"llama3.1:8b",
		"mistral:7b",
		"codellama:7b",
		"deepseek-coder:6.7b",
		"deepseek-r1:8b",
		"dolphin3:8b",

		// Large: May timeout on slower hardware
		"qwen2.5-coder:14b",
		"qwen2.5:14b",
		"codellama:13b",

		// XL: Likely to timeout unless on fast hardware
		"qwen2.5-coder:32b",
		"mixtral:8x7b",
		"codellama:34b",
		"llama3.1:70b",
	}
}

// GetAgenticDefaultModel returns the recommended default for agentic tasks.
// Qwen models are preferred for superior tool-calling accuracy.
func GetAgenticDefaultModel() string {
	return "qwen2.5-coder:7b"
}

// GetSafeMinimalModel returns the absolute safest choice (fastest, smallest).
func GetSafeMinimalModel() string {
	return "llama3.2:1b"
}
