package lobes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/llm"
	"github.com/normanking/cortex/pkg/brain"
)

type LogicLobe struct {
	llm LLMProvider
}

func NewLogicLobe(llm LLMProvider) *LogicLobe {
	return &LogicLobe{llm: llm}
}

func (l *LogicLobe) ID() brain.LobeID {
	return brain.LobeLogic
}

func (l *LogicLobe) Process(ctx context.Context, input brain.LobeInput, bb *brain.Blackboard) (*brain.LobeResult, error) {
	startTime := time.Now()

	systemPrompt := `You are a formal logic and mathematics expert. Your role is to:
1. Identify logical structures and relationships
2. Apply formal reasoning rules
3. Detect logical fallacies or inconsistencies
4. Provide step-by-step proofs when applicable
5. Use mathematical notation where helpful`

	var promptBuilder strings.Builder
	promptBuilder.WriteString("Problem: " + input.RawInput + "\n\n")
	promptBuilder.WriteString("Apply formal logic and reasoning to analyze this problem.")

	req := &llm.ChatRequest{
		Model:        "", // Use provider's default model
		SystemPrompt: systemPrompt,
		Messages:     []llm.Message{{Role: "user", Content: promptBuilder.String()}},
		Temperature:  0.1,
	}

	resp, err := l.llm.Chat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("logic processing failed: %w", err)
	}

	return &brain.LobeResult{
		LobeID:     l.ID(),
		Content:    resp.Content,
		Confidence: 1.0,
		Meta: brain.LobeMeta{
			StartedAt:  startTime,
			Duration:   time.Since(startTime),
			TokensUsed: resp.TokensUsed,
			ModelUsed:  resp.Model,
		},
	}, nil
}

func (l *LogicLobe) CanHandle(input string) float64 {
	lowerInput := strings.ToLower(input)
	keywords := []string{"prove", "logic", "therefore", "implies", "if then", "valid", "invalid", "fallacy", "theorem", "axiom"}
	for _, kw := range keywords {
		if strings.Contains(lowerInput, kw) {
			return 0.9
		}
	}
	return 0.1
}

func (l *LogicLobe) ResourceEstimate(input brain.LobeInput) brain.ResourceEstimate {
	return brain.ResourceEstimate{
		EstimatedTokens: 1000,
		EstimatedTime:   2 * time.Second,
		RequiresGPU:     true,
	}
}
