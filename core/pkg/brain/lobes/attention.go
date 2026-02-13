package lobes

import (
	"context"
	"strings"
	"time"

	"github.com/normanking/cortex/pkg/brain"
)

type AttentionLobe struct{}

func NewAttentionLobe() *AttentionLobe {
	return &AttentionLobe{}
}

func (l *AttentionLobe) ID() brain.LobeID {
	return brain.LobeAttention
}

func (l *AttentionLobe) Process(ctx context.Context, input brain.LobeInput, bb *brain.Blackboard) (*brain.LobeResult, error) {
	startTime := time.Now()

	// Attention lobe prioritizes and filters information
	result := AttentionResult{
		FocusAreas:       l.identifyFocusAreas(input.RawInput),
		Priority:         l.calculatePriority(input.RawInput),
		Distractors:      l.identifyDistractors(input.RawInput),
		RecommendedLobes: l.recommendLobes(input.RawInput),
	}

	return &brain.LobeResult{
		LobeID:     l.ID(),
		Content:    result,
		Confidence: 0.9,
		Meta: brain.LobeMeta{
			StartedAt: startTime,
			Duration:  time.Since(startTime),
		},
	}, nil
}

func (l *AttentionLobe) identifyFocusAreas(input string) []string {
	var areas []string
	lowerInput := strings.ToLower(input)

	if strings.Contains(lowerInput, "urgent") || strings.Contains(lowerInput, "important") {
		areas = append(areas, "high_priority")
	}
	if strings.Contains(lowerInput, "code") || strings.Contains(lowerInput, "implement") {
		areas = append(areas, "technical")
	}
	if strings.Contains(lowerInput, "help") || strings.Contains(lowerInput, "question") {
		areas = append(areas, "assistance")
	}
	if len(areas) == 0 {
		areas = append(areas, "general")
	}
	return areas
}

func (l *AttentionLobe) calculatePriority(input string) float64 {
	lowerInput := strings.ToLower(input)
	priority := 0.5

	if strings.Contains(lowerInput, "urgent") {
		priority += 0.3
	}
	if strings.Contains(lowerInput, "asap") || strings.Contains(lowerInput, "immediately") {
		priority += 0.2
	}
	if priority > 1.0 {
		priority = 1.0
	}
	return priority
}

func (l *AttentionLobe) identifyDistractors(input string) []string {
	return []string{} // Placeholder for noise filtering
}

func (l *AttentionLobe) recommendLobes(input string) []brain.LobeID {
	var lobes []brain.LobeID
	lowerInput := strings.ToLower(input)

	if strings.Contains(lowerInput, "code") {
		lobes = append(lobes, brain.LobeCoding)
	}
	if strings.Contains(lowerInput, "remember") {
		lobes = append(lobes, brain.LobeMemory)
	}
	if strings.Contains(lowerInput, "plan") {
		lobes = append(lobes, brain.LobePlanning)
	}
	return lobes
}

func (l *AttentionLobe) CanHandle(input string) float64 {
	return 0.7 // Attention is always somewhat relevant
}

func (l *AttentionLobe) ResourceEstimate(input brain.LobeInput) brain.ResourceEstimate {
	return brain.ResourceEstimate{
		EstimatedTokens: 0,
		EstimatedTime:   10 * time.Millisecond,
		RequiresGPU:     false,
	}
}

type AttentionResult struct {
	FocusAreas       []string       `json:"focus_areas"`
	Priority         float64        `json:"priority"`
	Distractors      []string       `json:"distractors"`
	RecommendedLobes []brain.LobeID `json:"recommended_lobes"`
}
