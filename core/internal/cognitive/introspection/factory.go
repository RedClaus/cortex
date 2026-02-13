package introspection

import (
	"github.com/normanking/cortex/internal/llm"
	"github.com/normanking/cortex/internal/memory"
)

// Config holds configuration for the introspection system.
type Config struct {
	// LLMProvider for classification and gap analysis.
	LLMProvider LLMProvider

	// FullLLMProvider is the full llm.Provider for acquisition engine.
	// If nil, acquisition will not be available.
	FullLLMProvider llm.Provider

	// Inventory for searching stored knowledge.
	Inventory *memory.KnowledgeInventory

	// KnowledgeFabric for storing acquired knowledge.
	KnowledgeFabric KnowledgeFabricCreator

	// TopicStore for creating topic clusters.
	TopicStore TopicStoreCreator

	// StrategicStore for recording meta-learning patterns.
	StrategicStore StrategicMemoryCreator

	// WebSearcher for web-based knowledge acquisition.
	WebSearcher WebSearcher

	// EventBus for publishing introspection events.
	EventBus EventPublisher

	// Enabled controls whether introspection is active.
	Enabled bool
}

// System holds all introspection components.
type System struct {
	Classifier  *Classifier
	Analyzer    *GapAnalyzer
	Responder   *MetacognitiveResponder
	Acquisition *AcquisitionEngine
	Learning    *LearningConfirmation
	Enabled     bool
}

// NewSystem creates a fully-wired introspection system.
func NewSystem(cfg *Config) *System {
	if cfg == nil {
		return &System{Enabled: false}
	}

	sys := &System{
		Classifier: NewClassifier(cfg.LLMProvider, nil),
		Analyzer:   NewGapAnalyzer(cfg.LLMProvider),
		Responder:  NewMetacognitiveResponder(),
		Enabled:    cfg.Enabled,
	}

	// Create acquisition engine if all dependencies are available
	if cfg.WebSearcher != nil && cfg.KnowledgeFabric != nil && cfg.TopicStore != nil && cfg.FullLLMProvider != nil {
		sys.Acquisition = NewAcquisitionEngine(
			cfg.KnowledgeFabric,
			cfg.TopicStore,
			cfg.WebSearcher,
			cfg.EventBus,
			cfg.FullLLMProvider,
		)
	}

	// Create learning confirmation if dependencies are available
	if cfg.Inventory != nil && cfg.StrategicStore != nil {
		sys.Learning = NewLearningConfirmation(
			cfg.Inventory,
			cfg.StrategicStore,
			cfg.LLMProvider,
		)
	}

	return sys
}

// DefaultConfig returns a default configuration (disabled).
func DefaultConfig() *Config {
	return &Config{
		Enabled: false,
	}
}
