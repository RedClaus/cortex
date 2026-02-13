package lobes

import (
	"context"
	"strings"
	"time"

	"github.com/normanking/cortex/pkg/brain"
)

type SelfKnowledgeLobe struct {
	capabilities map[string]float64
	limitations  []string
}

func NewSelfKnowledgeLobe() *SelfKnowledgeLobe {
	return &SelfKnowledgeLobe{
		capabilities: map[string]float64{
			"coding":       0.9,
			"reasoning":    0.85,
			"creativity":   0.8,
			"memory":       0.9,
			"planning":     0.85,
			"math":         0.7,
			"vision":       0.6,
			"real_time":    0.0,
			"web_browsing": 0.0,
		},
		limitations: []string{
			"Cannot access real-time information",
			"Cannot browse the internet",
			"Cannot execute code in production environments",
			"Knowledge cutoff applies to training data",
		},
	}
}

func (l *SelfKnowledgeLobe) ID() brain.LobeID {
	return brain.LobeSelfKnowledge
}

func (l *SelfKnowledgeLobe) Process(ctx context.Context, input brain.LobeInput, bb *brain.Blackboard) (*brain.LobeResult, error) {
	startTime := time.Now()

	result := SelfKnowledgeResult{
		RelevantCapabilities: l.findRelevantCapabilities(input.RawInput),
		Limitations:          l.limitations,
		ConfidenceInTask:     l.assessTaskConfidence(input.RawInput),
		Recommendations:      l.makeRecommendations(input.RawInput),
	}

	return &brain.LobeResult{
		LobeID:     l.ID(),
		Content:    result,
		Confidence: result.ConfidenceInTask,
		Meta: brain.LobeMeta{
			StartedAt: startTime,
			Duration:  time.Since(startTime),
		},
	}, nil
}

func (l *SelfKnowledgeLobe) findRelevantCapabilities(input string) map[string]float64 {
	relevant := make(map[string]float64)
	lowerInput := strings.ToLower(input)

	if strings.Contains(lowerInput, "code") || strings.Contains(lowerInput, "program") {
		relevant["coding"] = l.capabilities["coding"]
	}
	if strings.Contains(lowerInput, "think") || strings.Contains(lowerInput, "analyze") {
		relevant["reasoning"] = l.capabilities["reasoning"]
	}
	if strings.Contains(lowerInput, "create") || strings.Contains(lowerInput, "imagine") {
		relevant["creativity"] = l.capabilities["creativity"]
	}
	return relevant
}

func (l *SelfKnowledgeLobe) assessTaskConfidence(input string) float64 {
	lowerInput := strings.ToLower(input)

	// Lower confidence for tasks outside capabilities
	if strings.Contains(lowerInput, "current") || strings.Contains(lowerInput, "today") {
		return 0.3 // Real-time info
	}
	if strings.Contains(lowerInput, "browse") || strings.Contains(lowerInput, "website") {
		return 0.2 // Web browsing
	}
	return 0.8 // Default good confidence
}

func (l *SelfKnowledgeLobe) makeRecommendations(input string) []string {
	var recs []string
	lowerInput := strings.ToLower(input)

	if strings.Contains(lowerInput, "current") {
		recs = append(recs, "Consider using web search tool for current information")
	}
	return recs
}

func (l *SelfKnowledgeLobe) CanHandle(input string) float64 {
	lowerInput := strings.ToLower(input)
	if strings.Contains(lowerInput, "can you") || strings.Contains(lowerInput, "are you able") {
		return 0.95
	}
	return 0.4
}

func (l *SelfKnowledgeLobe) ResourceEstimate(input brain.LobeInput) brain.ResourceEstimate {
	return brain.ResourceEstimate{
		EstimatedTokens: 0,
		EstimatedTime:   10 * time.Millisecond,
		RequiresGPU:     false,
	}
}

type SelfKnowledgeResult struct {
	RelevantCapabilities map[string]float64 `json:"relevant_capabilities"`
	Limitations          []string           `json:"limitations"`
	ConfidenceInTask     float64            `json:"confidence_in_task"`
	Recommendations      []string           `json:"recommendations"`
}
