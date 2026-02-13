package brain

import (
	"time"
)

// StrategyBuilder helps construct thinking strategies.
type StrategyBuilder struct {
	strategy ThinkingStrategy
}

// NewStrategyBuilder creates a new strategy builder.
func NewStrategyBuilder(name string) *StrategyBuilder {
	return &StrategyBuilder{
		strategy: ThinkingStrategy{
			Name:   name,
			Phases: []ExecutionPhase{},
		},
	}
}

// WithComputeTier sets the compute tier.
func (b *StrategyBuilder) WithComputeTier(tier ComputeTier) *StrategyBuilder {
	b.strategy.ComputeTier = tier
	return b
}

// AddPhase adds an execution phase.
func (b *StrategyBuilder) AddPhase(name string, lobes []LobeID, parallel bool, timeout time.Duration, canReplan bool) *StrategyBuilder {
	phase := ExecutionPhase{
		Name:      name,
		Lobes:     lobes,
		Parallel:  parallel,
		TimeoutMS: int(timeout.Milliseconds()),
		CanReplan: canReplan,
	}
	b.strategy.Phases = append(b.strategy.Phases, phase)
	return b
}

// Build returns the completed strategy.
func (b *StrategyBuilder) Build() ThinkingStrategy {
	return b.strategy
}

// Pre-built strategy factories:

// QuickAnswerStrategy returns a simple single-phase strategy for fast responses.
// Note: Cloud APIs like Anthropic/OpenAI typically need 10-30 seconds for a response.
func QuickAnswerStrategy() ThinkingStrategy {
	return NewStrategyBuilder("QuickAnswer").
		WithComputeTier(ComputeFast).
		AddPhase("Response", []LobeID{LobeReasoning}, false, 60*time.Second, false).
		Build()
}

// DeepReasoningStrategy returns a multi-phase strategy for complex problems.
func DeepReasoningStrategy() ThinkingStrategy {
	return NewStrategyBuilder("DeepReasoning").
		WithComputeTier(ComputeDeep).
		AddPhase("Understand", []LobeID{LobeTextParsing, LobeAttention}, true, 30*time.Second, true).
		AddPhase("Reason", []LobeID{LobeReasoning, LobeLogic, LobeCausal}, true, 60*time.Second, true).
		AddPhase("Synthesize", []LobeID{LobeReasoning}, false, 30*time.Second, false).
		Build()
}

// CodingStrategy returns a strategy optimized for code-related tasks.
func CodingStrategy() ThinkingStrategy {
	return NewStrategyBuilder("Coding").
		WithComputeTier(ComputeHybrid).
		AddPhase("Planning", []LobeID{LobePlanning, LobeReasoning}, true, 45*time.Second, true).
		AddPhase("Coding", []LobeID{LobeCoding}, false, 90*time.Second, true).
		AddPhase("Review", []LobeID{LobeCoding, LobeSafety}, true, 45*time.Second, true).
		Build()
}

// CreativeStrategy returns a strategy for brainstorming and ideation.
func CreativeStrategy() ThinkingStrategy {
	return NewStrategyBuilder("Creative").
		WithComputeTier(ComputeDeep).
		AddPhase("Brainstorm", []LobeID{LobeCreativity}, false, 45*time.Second, true).
		AddPhase("Filter", []LobeID{LobeReasoning, LobeInhibition}, true, 30*time.Second, false).
		AddPhase("Refine", []LobeID{LobeCreativity, LobeTextParsing}, true, 45*time.Second, true).
		Build()
}

// SafetyFirstStrategy returns a strategy that prioritizes safety checks.
func SafetyFirstStrategy() ThinkingStrategy {
	return NewStrategyBuilder("SafetyFirst").
		WithComputeTier(ComputeDeep).
		AddPhase("Safety Check", []LobeID{LobeSafety}, false, 30*time.Second, false).
		AddPhase("Standard Execution", []LobeID{LobeReasoning, LobePlanning}, true, 60*time.Second, true).
		Build()
}
