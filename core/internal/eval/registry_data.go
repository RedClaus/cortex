package eval

import (
	"log"
)

// ═══════════════════════════════════════════════════════════════════════════════
// MODEL REGISTRY DATA
// Static capability data for all known LLM models.
// Scores are based on public benchmarks and documented capabilities.
// Principle: LOOKUP, DON'T COMPUTE - No API calls or benchmarking needed.
//
// Model data is loaded from models.yaml via go:embed in registry_loader.go.
// ═══════════════════════════════════════════════════════════════════════════════

// getAllModels returns all known models from all providers.
// Models are loaded from the embedded models.yaml file.
func getAllModels() []*ModelCapability {
	models, err := loadModelsFromYAML()
	if err != nil {
		log.Printf("WARN: failed to load models from YAML: %v, using empty registry", err)
		return nil
	}
	return models
}
