package lobes

import (
	"context"
	"time"

	"github.com/normanking/cortex/pkg/brain"
)

type MetacognitionLobe struct{}

func NewMetacognitionLobe() *MetacognitionLobe {
	return &MetacognitionLobe{}
}

func (l *MetacognitionLobe) ID() brain.LobeID {
	return brain.LobeMetacognition
}

func (l *MetacognitionLobe) Process(ctx context.Context, input brain.LobeInput, bb *brain.Blackboard) (*brain.LobeResult, error) {
	startTime := time.Now()

	// Metacognition reflects on the thinking process
	result := MetacognitionResult{
		ConfidenceAssessment: bb.OverallConfidence,
		ProcessingQuality:    l.assessQuality(bb),
		SuggestedRefinements: l.suggestRefinements(bb),
		KnowledgeGaps:        l.identifyGaps(bb),
	}

	return &brain.LobeResult{
		LobeID:     l.ID(),
		Content:    result,
		Confidence: 0.85,
		Meta: brain.LobeMeta{
			StartedAt: startTime,
			Duration:  time.Since(startTime),
		},
	}, nil
}

func (l *MetacognitionLobe) assessQuality(bb *brain.Blackboard) float64 {
	if bb == nil {
		return 0.5
	}

	quality := 0.5
	if len(bb.Memories) > 0 {
		quality += 0.2
	}
	if len(bb.Entities) > 0 {
		quality += 0.1
	}
	if bb.OverallConfidence > 0.8 {
		quality += 0.2
	}
	if quality > 1.0 {
		quality = 1.0
	}
	return quality
}

func (l *MetacognitionLobe) suggestRefinements(bb *brain.Blackboard) []string {
	var suggestions []string

	if bb == nil {
		return suggestions
	}

	if len(bb.Memories) == 0 {
		suggestions = append(suggestions, "Consider retrieving relevant memories")
	}
	if bb.OverallConfidence < 0.5 {
		suggestions = append(suggestions, "Low confidence - consider additional verification")
	}
	return suggestions
}

func (l *MetacognitionLobe) identifyGaps(bb *brain.Blackboard) []string {
	return []string{} // Placeholder
}

func (l *MetacognitionLobe) CanHandle(input string) float64 {
	return 0.6 // Metacognition is useful for reflection phases
}

func (l *MetacognitionLobe) ResourceEstimate(input brain.LobeInput) brain.ResourceEstimate {
	return brain.ResourceEstimate{
		EstimatedTokens: 0,
		EstimatedTime:   20 * time.Millisecond,
		RequiresGPU:     false,
	}
}

type MetacognitionResult struct {
	ConfidenceAssessment float64  `json:"confidence_assessment"`
	ProcessingQuality    float64  `json:"processing_quality"`
	SuggestedRefinements []string `json:"suggested_refinements"`
	KnowledgeGaps        []string `json:"knowledge_gaps"`
}
