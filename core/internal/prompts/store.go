package prompts

import (
	_ "embed"

	"gopkg.in/yaml.v3"
)

//go:embed static/optimized.yaml
var optimizedYAML []byte

// Store provides access to tier-optimized prompts
type Store struct {
	prompts map[string]map[string]string // task -> tier -> prompt
}

type yamlFile struct {
	Prompts map[string]map[string]string `yaml:"prompts"`
}

// Load initializes the prompt store from embedded YAML
func Load() *Store {
	var data yamlFile
	if err := yaml.Unmarshal(optimizedYAML, &data); err != nil {
		// Return empty store - caller handles missing prompts
		return &Store{prompts: make(map[string]map[string]string)}
	}
	return &Store{prompts: data.Prompts}
}

// Get returns the optimized prompt for a task and model
func (s *Store) Get(task string, modelParams int64) string {
	tier := "large"
	if modelParams < 14_000_000_000 { // < 14B
		tier = "small"
	}
	return s.GetTier(task, tier)
}

// GetTier returns prompt for explicit tier
func (s *Store) GetTier(task, tier string) string {
	if taskPrompts, ok := s.prompts[task]; ok {
		if prompt, ok := taskPrompts[tier]; ok {
			return prompt
		}
		// Fallback: return any available tier
		for _, p := range taskPrompts {
			return p
		}
	}
	return "" // Caller handles empty
}

// Has checks if a task exists
func (s *Store) Has(task string) bool {
	_, ok := s.prompts[task]
	return ok
}
