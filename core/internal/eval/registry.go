package eval

import (
	"strings"
	"sync"
)

// ═══════════════════════════════════════════════════════════════════════════════
// MODEL REGISTRY INTERFACE
// ═══════════════════════════════════════════════════════════════════════════════

// ModelRegistry provides lookup access to model capability information.
type ModelRegistry interface {
	// Get retrieves capability info for a specific model.
	Get(provider, model string) (*ModelCapability, bool)

	// GetByID retrieves by full ID (e.g., "openai/gpt-4o").
	GetByID(id string) (*ModelCapability, bool)

	// List returns all models, optionally filtered by provider.
	// Pass empty string to get all models.
	List(provider string) []*ModelCapability

	// ListByTier returns models at a specific tier.
	ListByTier(tier ModelTier) []*ModelCapability

	// DetectProvider infers provider from model ID or name.
	DetectProvider(modelID string) string

	// Size returns the number of models in the registry.
	Size() int
}

// ═══════════════════════════════════════════════════════════════════════════════
// DEFAULT REGISTRY IMPLEMENTATION
// ═══════════════════════════════════════════════════════════════════════════════

// defaultRegistry implements ModelRegistry with a static model database.
type defaultRegistry struct {
	models  map[string]*ModelCapability // key: "provider/model"
	aliases map[string]string           // alias -> canonical ID
}

var (
	registryOnce     sync.Once
	registryInstance *defaultRegistry
)

// DefaultRegistry returns the singleton registry instance.
func DefaultRegistry() ModelRegistry {
	registryOnce.Do(func() {
		registryInstance = newRegistry()
	})
	return registryInstance
}

// newRegistry creates and populates the registry.
func newRegistry() *defaultRegistry {
	r := &defaultRegistry{
		models:  make(map[string]*ModelCapability),
		aliases: make(map[string]string),
	}
	r.loadModels()
	return r
}

// loadModels populates the registry from static data.
func (r *defaultRegistry) loadModels() {
	// Load all provider models
	allModels := getAllModels()

	for _, m := range allModels {
		// Store by canonical ID
		r.models[m.ID] = m

		// Also store by just model name for easier lookup
		key := m.Provider + "/" + strings.ToLower(m.Model)
		if key != m.ID {
			r.aliases[key] = m.ID
		}

		// Register aliases
		for _, alias := range m.Aliases {
			aliasKey := m.Provider + "/" + strings.ToLower(alias)
			r.aliases[aliasKey] = m.ID
		}
	}
}

// Get retrieves capability info for a specific model.
func (r *defaultRegistry) Get(provider, model string) (*ModelCapability, bool) {
	provider = strings.ToLower(provider)
	model = strings.ToLower(model)

	// Try direct lookup
	id := provider + "/" + model
	if cap, ok := r.models[id]; ok {
		return cap, true
	}

	// Try alias
	if canonicalID, ok := r.aliases[id]; ok {
		if cap, ok := r.models[canonicalID]; ok {
			return cap, true
		}
	}

	// Try partial match (e.g., "claude-sonnet-4" matches "claude-sonnet-4-20250514")
	for key, cap := range r.models {
		if strings.HasPrefix(key, provider+"/") {
			modelPart := strings.TrimPrefix(key, provider+"/")
			if strings.HasPrefix(modelPart, model) || strings.HasPrefix(model, modelPart) {
				return cap, true
			}
		}
	}

	return nil, false
}

// GetByID retrieves by full ID (e.g., "openai/gpt-4o").
func (r *defaultRegistry) GetByID(id string) (*ModelCapability, bool) {
	id = strings.ToLower(id)

	if cap, ok := r.models[id]; ok {
		return cap, true
	}

	// Try alias
	if canonicalID, ok := r.aliases[id]; ok {
		if cap, ok := r.models[canonicalID]; ok {
			return cap, true
		}
	}

	return nil, false
}

// List returns all models, optionally filtered by provider.
func (r *defaultRegistry) List(provider string) []*ModelCapability {
	var result []*ModelCapability
	provider = strings.ToLower(provider)

	for _, cap := range r.models {
		if provider == "" || strings.ToLower(cap.Provider) == provider {
			result = append(result, cap)
		}
	}

	return result
}

// ListByTier returns models at a specific tier.
func (r *defaultRegistry) ListByTier(tier ModelTier) []*ModelCapability {
	var result []*ModelCapability

	for _, cap := range r.models {
		if cap.Tier == tier {
			result = append(result, cap)
		}
	}

	return result
}

// DetectProvider infers provider from model ID or name.
func (r *defaultRegistry) DetectProvider(modelID string) string {
	modelLower := strings.ToLower(modelID)

	// Check for explicit provider prefix
	if idx := strings.Index(modelLower, "/"); idx > 0 {
		return modelLower[:idx]
	}

	// Anthropic patterns
	if strings.HasPrefix(modelLower, "claude") {
		return "anthropic"
	}

	// OpenAI patterns
	if strings.HasPrefix(modelLower, "gpt-") ||
		strings.HasPrefix(modelLower, "o1") ||
		strings.HasPrefix(modelLower, "davinci") ||
		strings.HasPrefix(modelLower, "curie") {
		return "openai"
	}

	// Google/Gemini patterns
	if strings.HasPrefix(modelLower, "gemini") ||
		strings.HasPrefix(modelLower, "palm") {
		return "gemini"
	}

	// xAI Grok patterns
	if strings.HasPrefix(modelLower, "grok") {
		return "grok"
	}

	// Mistral API patterns
	if strings.HasPrefix(modelLower, "mistral-") ||
		strings.HasPrefix(modelLower, "codestral") ||
		strings.HasPrefix(modelLower, "open-mistral") ||
		strings.HasPrefix(modelLower, "open-mixtral") {
		return "mistral"
	}

	// Ollama/local model patterns
	ollamaPatterns := []string{
		"llama", "mistral", "codellama", "deepseek", "phi",
		"gemma", "dolphin", "qwen", "mixtral", "tinyllama",
		"vicuna", "orca", "neural-chat", "starling",
	}
	for _, pattern := range ollamaPatterns {
		if strings.Contains(modelLower, pattern) {
			return "ollama"
		}
	}

	// Check for version suffix (common in Ollama: "model:7b")
	if strings.Contains(modelLower, ":") {
		return "ollama"
	}

	return "unknown"
}

// Size returns the number of models in the registry.
func (r *defaultRegistry) Size() int {
	return len(r.models)
}
