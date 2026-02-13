package eval

import (
	_ "embed"
	"fmt"
	"sync"

	"gopkg.in/yaml.v3"
)

// ═══════════════════════════════════════════════════════════════════════════════
// YAML MODEL REGISTRY LOADER
// Uses go:embed to load model definitions from models.yaml at compile time.
// ═══════════════════════════════════════════════════════════════════════════════

//go:embed models.yaml
var modelsYAML []byte

// yamlModelRegistry holds the parsed YAML data
type yamlModelRegistry struct {
	Models map[string][]yamlModelEntry `yaml:"models"`
}

// yamlModelEntry represents a single model in YAML
type yamlModelEntry struct {
	ID            string           `yaml:"id"`
	Model         string           `yaml:"model"`
	DisplayName   string           `yaml:"display_name"`
	Tier          string           `yaml:"tier"`
	Score         yamlScore        `yaml:"score"`
	Capabilities  yamlCapabilities `yaml:"capabilities"`
	Pricing       *yamlPricing     `yaml:"pricing,omitempty"`
	ContextWindow int              `yaml:"context_window"`
	Aliases       []string         `yaml:"aliases,omitempty"`
}

// yamlScore represents capability scores in YAML
type yamlScore struct {
	Overall     int     `yaml:"overall"`
	Reasoning   int     `yaml:"reasoning"`
	Coding      int     `yaml:"coding"`
	Instruction int     `yaml:"instruction"`
	Speed       int     `yaml:"speed"`
	Confidence  float64 `yaml:"confidence"`
}

// yamlCapabilities represents capability flags in YAML
type yamlCapabilities struct {
	Vision          bool `yaml:"vision"`
	FunctionCalling bool `yaml:"function_calling"`
	JSONMode        bool `yaml:"json_mode"`
	Streaming       bool `yaml:"streaming"`
	SystemPrompt    bool `yaml:"system_prompt"`
}

// yamlPricing represents pricing info in YAML
type yamlPricing struct {
	InputPer1M  float64 `yaml:"input_per_1m"`
	OutputPer1M float64 `yaml:"output_per_1m"`
}

var (
	loadedModels   []*ModelCapability
	loadModelsOnce sync.Once
	loadModelsErr  error
)

// loadModelsFromYAML parses the embedded YAML and returns model capabilities.
// This is called once and cached.
func loadModelsFromYAML() ([]*ModelCapability, error) {
	loadModelsOnce.Do(func() {
		var registry yamlModelRegistry
		if err := yaml.Unmarshal(modelsYAML, &registry); err != nil {
			loadModelsErr = fmt.Errorf("parse models.yaml: %w", err)
			return
		}

		// Convert YAML entries to ModelCapability structs
		for provider, entries := range registry.Models {
			for _, entry := range entries {
				cap := convertYAMLEntry(provider, entry)
				loadedModels = append(loadedModels, cap)
			}
		}
	})

	return loadedModels, loadModelsErr
}

// convertYAMLEntry converts a yamlModelEntry to a ModelCapability
func convertYAMLEntry(provider string, entry yamlModelEntry) *ModelCapability {
	// Convert tier string to ModelTier
	tier := parseTier(entry.Tier)

	// Build the capability struct
	cap := &ModelCapability{
		ID:          entry.ID,
		Provider:    provider,
		Model:       entry.Model,
		DisplayName: entry.DisplayName,
		Tier:        tier,
		Score: UnifiedCapabilityScore{
			Overall:     entry.Score.Overall,
			Reasoning:   entry.Score.Reasoning,
			Coding:      entry.Score.Coding,
			Instruction: entry.Score.Instruction,
			Speed:       entry.Score.Speed,
			Confidence:  entry.Score.Confidence,
			Source:      ScoreSourceRegistry,
		},
		Capabilities: CapabilityFlags{
			Vision:          entry.Capabilities.Vision,
			FunctionCalling: entry.Capabilities.FunctionCalling,
			JSONMode:        entry.Capabilities.JSONMode,
			Streaming:       entry.Capabilities.Streaming,
			SystemPrompt:    entry.Capabilities.SystemPrompt,
		},
		ContextWindow: entry.ContextWindow,
		Aliases:       entry.Aliases,
	}

	// Add pricing if present
	if entry.Pricing != nil {
		cap.Pricing = &PricingInfo{
			InputPer1MTokens:  entry.Pricing.InputPer1M,
			OutputPer1MTokens: entry.Pricing.OutputPer1M,
		}
	}

	return cap
}

// parseTier converts a tier string to ModelTier
func parseTier(s string) ModelTier {
	switch s {
	case "frontier":
		return TierFrontier
	case "xl":
		return TierXL
	case "large":
		return TierLarge
	case "medium":
		return TierMedium
	case "small":
		return TierSmall
	default:
		return TierMedium
	}
}
