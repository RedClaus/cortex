package lobes

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/normanking/cortex/pkg/brain"
)

type InhibitionLobe struct {
	blockedPatterns []*regexp.Regexp
}

func NewInhibitionLobe() *InhibitionLobe {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)ignore\s+previous`),
		regexp.MustCompile(`(?i)pretend\s+you\s+are`),
		regexp.MustCompile(`(?i)jailbreak`),
		regexp.MustCompile(`(?i)bypass\s+restrictions`),
	}
	return &InhibitionLobe{blockedPatterns: patterns}
}

func (l *InhibitionLobe) ID() brain.LobeID {
	return brain.LobeInhibition
}

func (l *InhibitionLobe) Process(ctx context.Context, input brain.LobeInput, bb *brain.Blackboard) (*brain.LobeResult, error) {
	startTime := time.Now()

	result := InhibitionResult{
		ShouldInhibit:     false,
		InhibitedAreas:    []string{},
		RiskFactors:       []string{},
		RecommendedAction: "proceed",
	}

	// Check for prompt injection attempts
	for _, pattern := range l.blockedPatterns {
		if pattern.MatchString(input.RawInput) {
			result.ShouldInhibit = true
			result.InhibitedAreas = append(result.InhibitedAreas, "prompt_injection")
			result.RiskFactors = append(result.RiskFactors, pattern.String())
			result.RecommendedAction = "block"
		}
	}

	// Check for excessive complexity
	if len(input.RawInput) > 10000 {
		result.RiskFactors = append(result.RiskFactors, "excessive_length")
	}

	return &brain.LobeResult{
		LobeID:        l.ID(),
		Content:       result,
		Confidence:    0.95,
		RequestReplan: result.ShouldInhibit,
		ReplanReason:  "Inhibition triggered: " + strings.Join(result.InhibitedAreas, ", "),
		Meta: brain.LobeMeta{
			StartedAt: startTime,
			Duration:  time.Since(startTime),
		},
	}, nil
}

func (l *InhibitionLobe) CanHandle(input string) float64 {
	for _, pattern := range l.blockedPatterns {
		if pattern.MatchString(input) {
			return 1.0
		}
	}
	return 0.3 // Always run as a safeguard
}

func (l *InhibitionLobe) ResourceEstimate(input brain.LobeInput) brain.ResourceEstimate {
	return brain.ResourceEstimate{
		EstimatedTokens: 0,
		EstimatedTime:   5 * time.Millisecond,
		RequiresGPU:     false,
	}
}

type InhibitionResult struct {
	ShouldInhibit     bool     `json:"should_inhibit"`
	InhibitedAreas    []string `json:"inhibited_areas"`
	RiskFactors       []string `json:"risk_factors"`
	RecommendedAction string   `json:"recommended_action"`
}
