package lobes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/llm"
	"github.com/normanking/cortex/pkg/brain"
)

// ReasoningLobe handles logical deduction and analysis.
type ReasoningLobe struct {
	llm LLMProvider
}

// NewReasoningLobe creates a new reasoning lobe.
func NewReasoningLobe(llm LLMProvider) *ReasoningLobe {
	return &ReasoningLobe{
		llm: llm,
	}
}

// ID returns brain.LobeReasoning
func (l *ReasoningLobe) ID() brain.LobeID {
	return brain.LobeReasoning
}

// Process performs reasoning on the input using LLM.
func (l *ReasoningLobe) Process(ctx context.Context, input brain.LobeInput, bb *brain.Blackboard) (*brain.LobeResult, error) {
	startTime := time.Now()

	// Use Clone to safely access blackboard data without race conditions
	safeBB := bb.Clone()

	// Construct system prompt
	systemPrompt := "You are the Reasoning Lobe of the Cortex brain. Your job is to perform deep logical deduction, analysis, and reasoning based on the input and provided context. Be objective, thorough, and analytical."

	// Construct user prompt with context
	var promptBuilder strings.Builder
	promptBuilder.WriteString("Input: " + input.RawInput + "\n\n")

	if safeBB != nil {
		promptBuilder.WriteString("Context:\n")

		// Add memories
		if len(safeBB.Memories) > 0 {
			promptBuilder.WriteString("Memories:\n")
			for _, mem := range safeBB.Memories {
				promptBuilder.WriteString(fmt.Sprintf("- %s (Relevance: %.2f)\n", mem.Content, mem.Relevance))
			}
			promptBuilder.WriteString("\n")
		}

		// Add entities
		if len(safeBB.Entities) > 0 {
			promptBuilder.WriteString("Entities:\n")
			for _, ent := range safeBB.Entities {
				promptBuilder.WriteString(fmt.Sprintf("- %s: %s\n", ent.Type, ent.Value))
			}
			promptBuilder.WriteString("\n")
		}

		// Add user state
		if safeBB.UserState != nil {
			promptBuilder.WriteString("User State:\n")
			promptBuilder.WriteString(fmt.Sprintf("- Expertise: %s\n", safeBB.UserState.ExpertiseLevel))
			promptBuilder.WriteString(fmt.Sprintf("- Mood: %s\n", safeBB.UserState.EstimatedMood))
			promptBuilder.WriteString("\n")
		}
	}

	promptBuilder.WriteString("Analyze the input given the context and provide a reasoned response.")

	req := &llm.ChatRequest{
		Model:        "", // Use provider's default model
		SystemPrompt: systemPrompt,
		Messages: []llm.Message{
			{Role: "user", Content: promptBuilder.String()},
		},
		Temperature: 0.3,
	}

	resp, err := l.llm.Chat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("reasoning generation failed: %w", err)
	}

	result := &brain.LobeResult{
		LobeID:     l.ID(),
		Content:    resp.Content,
		Confidence: 1.0,
		Meta: brain.LobeMeta{
			StartedAt:  startTime,
			Duration:   time.Since(startTime),
			TokensUsed: resp.TokensUsed,
			ModelUsed:  resp.Model,
		},
	}

	return result, nil
}

// CanHandle returns confidence for reasoning tasks.
func (l *ReasoningLobe) CanHandle(input string) float64 {
	lowerInput := strings.ToLower(input)
	keywords := []string{"why", "explain", "analyze", "compare", "evaluate", "reason", "think"}

	for _, keyword := range keywords {
		if strings.Contains(lowerInput, keyword) {
			return 0.9
		}
	}

	return 0.1
}

// ResourceEstimate returns moderate resource requirements.
func (l *ReasoningLobe) ResourceEstimate(input brain.LobeInput) brain.ResourceEstimate {
	return brain.ResourceEstimate{
		EstimatedTokens: 1000,
		EstimatedTime:   2 * time.Second,
		RequiresGPU:     true,
	}
}
