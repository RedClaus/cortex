package lobes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/llm"
	"github.com/normanking/cortex/pkg/brain"
)

// PlanningLobe handles task decomposition and planning.
type PlanningLobe struct {
	llm LLMProvider
}

// NewPlanningLobe creates a new planning lobe.
func NewPlanningLobe(llm LLMProvider) *PlanningLobe {
	return &PlanningLobe{llm: llm}
}

// ID returns brain.LobePlanning
func (l *PlanningLobe) ID() brain.LobeID {
	return brain.LobePlanning
}

// Process decomposes a task into steps.
func (l *PlanningLobe) Process(ctx context.Context, input brain.LobeInput, bb *brain.Blackboard) (*brain.LobeResult, error) {
	startTime := time.Now()

	systemPrompt := `You are the Planning Lobe of the Cortex brain. Your job is to break down complex tasks into clear, actionable steps.
Guidelines:
1. Analyze the task to understand its full scope
2. Identify dependencies between steps
3. Order steps logically
4. Keep steps atomic and actionable
5. Consider potential blockers or risks`

	var promptBuilder strings.Builder
	promptBuilder.WriteString("Task to plan: " + input.RawInput + "\n\n")

	if bb != nil {
		safeBB := bb.Clone()
		if len(safeBB.Memories) > 0 {
			promptBuilder.WriteString("Relevant context:\n")
			for _, mem := range safeBB.Memories {
				promptBuilder.WriteString(fmt.Sprintf("- %s\n", mem.Content))
			}
		}
	}

	promptBuilder.WriteString("\nBreak this down into a clear step-by-step plan.")

	req := &llm.ChatRequest{
		Model:        "", // Use provider's default model
		SystemPrompt: systemPrompt,
		Messages:     []llm.Message{{Role: "user", Content: promptBuilder.String()}},
		Temperature:  0.3,
	}

	resp, err := l.llm.Chat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("planning generation failed: %w", err)
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

// CanHandle returns confidence for planning tasks.
func (l *PlanningLobe) CanHandle(input string) float64 {
	lowerInput := strings.ToLower(input)
	keywords := []string{"plan", "steps", "break down", "how to", "roadmap", "strategy", "outline"}

	for _, kw := range keywords {
		if strings.Contains(lowerInput, kw) {
			return 0.9
		}
	}
	return 0.1
}

// ResourceEstimate returns moderate resource requirements.
func (l *PlanningLobe) ResourceEstimate(input brain.LobeInput) brain.ResourceEstimate {
	return brain.ResourceEstimate{
		EstimatedTokens: 800,
		EstimatedTime:   2 * time.Second,
		RequiresGPU:     true,
	}
}
