package cognitive

import "fmt"

// PipelineFactory creates pipeline instances with configured providers.
type PipelineFactory struct {
	config PipelineConfig
}

// NewPipelineFactory creates a new pipeline factory.
func NewPipelineFactory(cfg PipelineConfig) *PipelineFactory {
	return &PipelineFactory{config: cfg}
}

// CreatePipeline creates a new pipeline with the given provider configurations.
func (f *PipelineFactory) CreatePipeline(
	ollamaEndpoint string,
	claudeAPIKey string,
	modeTracker *ModeTracker,
) (*Pipeline, error) {
	// Validate inputs
	if ollamaEndpoint == "" {
		ollamaEndpoint = "http://127.0.0.1:11434"
	}

	// Create fast lane provider (Ollama)
	fastLLM := NewOllamaProvider(ollamaEndpoint, f.config.FastModel)

	// Create smart lane provider (Claude)
	if claudeAPIKey == "" {
		return nil, fmt.Errorf("claude API key is required for smart lane")
	}
	smartLLM := NewClaudeProvider(claudeAPIKey, f.config.SmartModel)

	// Create pipeline
	return NewPipeline(fastLLM, smartLLM, modeTracker, f.config), nil
}

// CreatePipelineWithFallback creates a pipeline with fallback to fast lane only if smart lane unavailable.
func (f *PipelineFactory) CreatePipelineWithFallback(
	ollamaEndpoint string,
	claudeAPIKey string,
	modeTracker *ModeTracker,
) (*Pipeline, error) {
	// Validate inputs
	if ollamaEndpoint == "" {
		ollamaEndpoint = "http://127.0.0.1:11434"
	}

	// Create fast lane provider (Ollama)
	fastLLM := NewOllamaProvider(ollamaEndpoint, f.config.FastModel)

	// Create smart lane provider (Claude) if key is available
	var smartLLM LLMProvider
	if claudeAPIKey != "" {
		smartLLM = NewClaudeProvider(claudeAPIKey, f.config.SmartModel)
	} else {
		// Fallback: use a larger Ollama model for smart lane
		smartLLM = NewOllamaProvider(ollamaEndpoint, "llama3:8b")
	}

	// Create pipeline
	return NewPipeline(fastLLM, smartLLM, modeTracker, f.config), nil
}
