package lobes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/llm"
	"github.com/normanking/cortex/pkg/brain"
)

type SpatialLobe struct {
	llm LLMProvider
}

func NewSpatialLobe(llm LLMProvider) *SpatialLobe {
	return &SpatialLobe{llm: llm}
}

func (l *SpatialLobe) ID() brain.LobeID {
	return brain.LobeSpatial
}

func (l *SpatialLobe) Process(ctx context.Context, input brain.LobeInput, bb *brain.Blackboard) (*brain.LobeResult, error) {
	startTime := time.Now()

	systemPrompt := `You are an expert in spatial reasoning. Your role is to:
1. Understand physical layouts and arrangements
2. Reason about positions, distances, and directions
3. Visualize 3D structures and relationships
4. Analyze architectural and geographic concepts
5. Handle navigation and pathfinding problems`

	var promptBuilder strings.Builder
	promptBuilder.WriteString("Query: " + input.RawInput + "\n\n")
	promptBuilder.WriteString("Analyze the spatial aspects of this query.")

	req := &llm.ChatRequest{
		Model:        "", // Use provider's default model
		SystemPrompt: systemPrompt,
		Messages:     []llm.Message{{Role: "user", Content: promptBuilder.String()}},
		Temperature:  0.3,
	}

	resp, err := l.llm.Chat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("spatial processing failed: %w", err)
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

func (l *SpatialLobe) CanHandle(input string) float64 {
	lowerInput := strings.ToLower(input)
	keywords := []string{"where", "location", "position", "distance", "direction", "layout", "map", "navigate", "above", "below", "left", "right"}
	for _, kw := range keywords {
		if strings.Contains(lowerInput, kw) {
			return 0.85
		}
	}
	return 0.1
}

func (l *SpatialLobe) ResourceEstimate(input brain.LobeInput) brain.ResourceEstimate {
	return brain.ResourceEstimate{
		EstimatedTokens: 800,
		EstimatedTime:   2 * time.Second,
		RequiresGPU:     true,
	}
}
