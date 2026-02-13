package brain

import (
	"context"
	"fmt"
)

// Executive is the main entry point for the brain's cognitive processing.
type Executive struct {
	classifier *ExecutiveClassifier
	executor   *PhaseExecutor
	registry   *Registry
	monitor    *SystemMonitor
}

// ExecutiveConfig holds configuration for the Executive.
type ExecutiveConfig struct {
	Embedder  Embedder
	LLMClient LLMClient
	Cache     ClassificationCache
}

// NewExecutive creates a new Executive with the given configuration.
func NewExecutive(cfg ExecutiveConfig) *Executive {
	registry := NewRegistry()
	monitor := NewSystemMonitor(5000000000) // 5 seconds
	classifier := NewExecutiveClassifier(cfg.Embedder, cfg.LLMClient, cfg.Cache)
	executor := NewPhaseExecutor(registry, monitor)

	return &Executive{
		classifier: classifier,
		executor:   executor,
		registry:   registry,
		monitor:    monitor,
	}
}

// RegisterLobe adds a lobe to the executive's registry.
func (e *Executive) RegisterLobe(lobe Lobe) {
	e.registry.Register(lobe)
}

// Start begins background monitoring.
func (e *Executive) Start() {
	e.monitor.Start()
}

// Stop halts background monitoring.
func (e *Executive) Stop() {
	e.monitor.Stop()
}

// Process handles an input through the full cognitive pipeline.
func (e *Executive) Process(ctx context.Context, input string) (*ExecutionResult, error) {
	// Step 1: Classify the input
	classification, err := e.classifier.Classify(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("classification failed: %w", err)
	}

	// Step 2: Select strategy based on classification
	strategy := e.selectStrategy(classification)

	// Step 3: Execute the strategy
	lobeInput := LobeInput{
		RawInput: input,
		Strategy: &strategy,
	}

	result, err := e.executor.Execute(ctx, lobeInput, &strategy)
	if err != nil {
		return nil, fmt.Errorf("execution failed: %w", err)
	}

	result.Classification = classification

	return result, nil
}

// selectStrategy chooses the appropriate thinking strategy based on classification.
func (e *Executive) selectStrategy(c *ClassificationResult) ThinkingStrategy {
	// Safety-critical requests get safety-first strategy
	if c.RiskLevel == RiskHigh || c.RiskLevel == RiskCritical {
		return SafetyFirstStrategy()
	}

	// Route based on primary lobe
	switch c.PrimaryLobe {
	case LobeCoding:
		return CodingStrategy()
	case LobeCreativity:
		return CreativeStrategy()
	case LobePlanning:
		return DeepReasoningStrategy()
	case LobeReasoning, LobeLogic, LobeCausal:
		return DeepReasoningStrategy()
	default:
		return QuickAnswerStrategy()
	}
}

// GetMetrics returns current system metrics.
func (e *Executive) GetMetrics() SystemMetrics {
	return e.monitor.GetMetrics()
}

// Registry returns the lobe registry for external registration.
func (e *Executive) Registry() *Registry {
	return e.registry
}
