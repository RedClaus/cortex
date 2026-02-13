package lobes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/llm"
	"github.com/normanking/cortex/pkg/brain"
)

// CreativityLobe handles idea generation and creative thinking.
type CreativityLobe struct {
	llm LLMProvider
}

// NewCreativityLobe creates a new creativity lobe.
func NewCreativityLobe(llm LLMProvider) *CreativityLobe {
	return &CreativityLobe{
		llm: llm,
	}
}

// ID returns brain.LobeCreativity
func (l *CreativityLobe) ID() brain.LobeID {
	return brain.LobeCreativity
}

// Process generates creative ideas based on input.
func (l *CreativityLobe) Process(ctx context.Context, input brain.LobeInput, bb *brain.Blackboard) (*brain.LobeResult, error) {
	startTime := time.Now()

	safeBB := bb.Clone()

	systemPrompt := "You are the Creativity Lobe of the Cortex brain. Your job is to generate novel, divergent, and innovative ideas. Think outside the box, explore multiple possibilities, and embrace unconventional approaches. Be imaginative and inspiring."

	var promptBuilder strings.Builder
	promptBuilder.WriteString("Input: " + input.RawInput + "\n\n")

	if safeBB != nil {
		promptBuilder.WriteString("Context:\n")

		if len(safeBB.Memories) > 0 {
			promptBuilder.WriteString("Memories:\n")
			for _, mem := range safeBB.Memories {
				promptBuilder.WriteString(fmt.Sprintf("- %s (Relevance: %.2f)\n", mem.Content, mem.Relevance))
			}
			promptBuilder.WriteString("\n")
		}

		if len(safeBB.Entities) > 0 {
			promptBuilder.WriteString("Entities:\n")
			for _, ent := range safeBB.Entities {
				promptBuilder.WriteString(fmt.Sprintf("- %s: %s\n", ent.Type, ent.Value))
			}
			promptBuilder.WriteString("\n")
		}

		if safeBB.UserState != nil {
			promptBuilder.WriteString("User State:\n")
			promptBuilder.WriteString(fmt.Sprintf("- Expertise: %s\n", safeBB.UserState.ExpertiseLevel))
			promptBuilder.WriteString(fmt.Sprintf("- Mood: %s\n", safeBB.UserState.EstimatedMood))
			promptBuilder.WriteString("\n")
		}
	}

	promptBuilder.WriteString("Generate creative ideas or content based on the input and context.")

	req := &llm.ChatRequest{
		Model:        "", // Use provider's default model
		SystemPrompt: systemPrompt,
		Messages: []llm.Message{
			{Role: "user", Content: promptBuilder.String()},
		},
		Temperature: 0.7,
	}

	resp, err := l.llm.Chat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("creative generation failed: %w", err)
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

// CanHandle returns confidence for creative tasks.
func (l *CreativityLobe) CanHandle(input string) float64 {
	lowerInput := strings.ToLower(input)
	keywords := []string{"brainstorm", "ideas", "creative", "imagine", "generate", "story", "invent"}

	for _, keyword := range keywords {
		if strings.Contains(lowerInput, keyword) {
			return 0.95
		}
	}

	return 0.1
}

// ResourceEstimate returns moderate resource requirements.
func (l *CreativityLobe) ResourceEstimate(input brain.LobeInput) brain.ResourceEstimate {
	return brain.ResourceEstimate{
		EstimatedTokens: 1200,
		EstimatedTime:   2 * time.Second,
		RequiresGPU:     true,
	}
}
