package lobes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/llm"
	"github.com/normanking/cortex/pkg/brain"
)

type TemporalLobe struct {
	llm LLMProvider
}

func NewTemporalLobe(llm LLMProvider) *TemporalLobe {
	return &TemporalLobe{llm: llm}
}

func (l *TemporalLobe) ID() brain.LobeID {
	return brain.LobeTemporal
}

func (l *TemporalLobe) Process(ctx context.Context, input brain.LobeInput, bb *brain.Blackboard) (*brain.LobeResult, error) {
	startTime := time.Now()

	systemPrompt := `You are an expert in temporal reasoning. Your role is to:
1. Understand time-based relationships and sequences
2. Reason about schedules, deadlines, and durations
3. Handle temporal logic (before, after, during, until)
4. Project future states based on current trends
5. Analyze historical patterns and timelines`

	var promptBuilder strings.Builder
	promptBuilder.WriteString("Query: " + input.RawInput + "\n\n")
	promptBuilder.WriteString("Current time context: " + time.Now().Format(time.RFC3339) + "\n")
	promptBuilder.WriteString("Analyze the temporal aspects of this query.")

	req := &llm.ChatRequest{
		Model:        "", // Use provider's default model
		SystemPrompt: systemPrompt,
		Messages:     []llm.Message{{Role: "user", Content: promptBuilder.String()}},
		Temperature:  0.3,
	}

	resp, err := l.llm.Chat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("temporal processing failed: %w", err)
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

func (l *TemporalLobe) CanHandle(input string) float64 {
	lowerInput := strings.ToLower(input)
	keywords := []string{"when", "schedule", "deadline", "before", "after", "during", "timeline", "history", "future", "date", "time"}
	for _, kw := range keywords {
		if strings.Contains(lowerInput, kw) {
			return 0.85
		}
	}
	return 0.1
}

func (l *TemporalLobe) ResourceEstimate(input brain.LobeInput) brain.ResourceEstimate {
	return brain.ResourceEstimate{
		EstimatedTokens: 800,
		EstimatedTime:   2 * time.Second,
		RequiresGPU:     true,
	}
}
