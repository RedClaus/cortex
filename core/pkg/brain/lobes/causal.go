package lobes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/llm"
	"github.com/normanking/cortex/pkg/brain"
)

type CausalLobe struct {
	llm LLMProvider
}

func NewCausalLobe(llm LLMProvider) *CausalLobe {
	return &CausalLobe{llm: llm}
}

func (l *CausalLobe) ID() brain.LobeID {
	return brain.LobeCausal
}

func (l *CausalLobe) Process(ctx context.Context, input brain.LobeInput, bb *brain.Blackboard) (*brain.LobeResult, error) {
	startTime := time.Now()

	systemPrompt := `You are an expert in causal reasoning. Your role is to:
1. Identify cause-and-effect relationships
2. Distinguish correlation from causation
3. Trace chains of causality
4. Predict consequences of actions
5. Analyze root causes of problems`

	var promptBuilder strings.Builder
	promptBuilder.WriteString("Query: " + input.RawInput + "\n\n")
	promptBuilder.WriteString("Analyze the causal relationships in this query.")

	req := &llm.ChatRequest{
		Model:        "", // Use provider's default model
		SystemPrompt: systemPrompt,
		Messages:     []llm.Message{{Role: "user", Content: promptBuilder.String()}},
		Temperature:  0.3,
	}

	resp, err := l.llm.Chat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("causal processing failed: %w", err)
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

func (l *CausalLobe) CanHandle(input string) float64 {
	lowerInput := strings.ToLower(input)
	keywords := []string{"cause", "effect", "because", "result", "consequence", "lead to", "root cause", "impact", "due to"}
	for _, kw := range keywords {
		if strings.Contains(lowerInput, kw) {
			return 0.9
		}
	}
	return 0.1
}

func (l *CausalLobe) ResourceEstimate(input brain.LobeInput) brain.ResourceEstimate {
	return brain.ResourceEstimate{
		EstimatedTokens: 1000,
		EstimatedTime:   2 * time.Second,
		RequiresGPU:     true,
	}
}
