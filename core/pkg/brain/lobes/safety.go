package lobes

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/normanking/cortex/pkg/brain"
)

// SafetyResult contains the analysis from the SafetyLobe.
type SafetyResult struct {
	IsSafe          bool            `json:"is_safe"`
	RiskLevel       brain.RiskLevel `json:"risk_level"`
	Concerns        []string        `json:"concerns"`
	Recommendations []string        `json:"recommendations"`
}

// SafetyLobe handles safety checks and harm prevention.
type SafetyLobe struct {
	dangerPatterns []*regexp.Regexp
}

// NewSafetyLobe creates a safety lobe with default danger patterns.
func NewSafetyLobe() *SafetyLobe {
	patterns := []string{
		"rm -rf",
		"sudo",
		"password",
		"credential",
		"secret",
		"api.key",
		"delete.*database",
		"drop.*table",
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if re, err := regexp.Compile("(?i)" + p); err == nil {
			compiled = append(compiled, re)
		}
	}

	return &SafetyLobe{
		dangerPatterns: compiled,
	}
}

// ID returns brain.LobeSafety
func (l *SafetyLobe) ID() brain.LobeID {
	return brain.LobeSafety
}

// Process checks input for safety concerns and flags issues.
func (l *SafetyLobe) Process(ctx context.Context, input brain.LobeInput, bb *brain.Blackboard) (*brain.LobeResult, error) {
	start := time.Now()
	content := input.RawInput

	result := SafetyResult{
		IsSafe:          true,
		RiskLevel:       brain.RiskLow,
		Concerns:        []string{},
		Recommendations: []string{},
	}

	for _, re := range l.dangerPatterns {
		if re.MatchString(content) {
			result.IsSafe = false
			result.Concerns = append(result.Concerns, "Matched dangerous pattern: "+re.String())
		}
	}

	// Calculate risk level based on concerns
	if !result.IsSafe {
		if len(result.Concerns) > 0 {
			result.RiskLevel = brain.RiskHigh
			result.Recommendations = append(result.Recommendations, "Review the command for potential destructive actions.")
			result.Recommendations = append(result.Recommendations, "Ensure appropriate permissions and backups.")
		}
	}

	lobeResult := &brain.LobeResult{
		LobeID:  l.ID(),
		Content: result,
		Meta: brain.LobeMeta{
			StartedAt:  start,
			Duration:   time.Since(start),
			TokensUsed: 10, // Estimate
			ModelUsed:  "regex-engine",
		},
		Confidence: 1.0, // Deterministic check
	}

	if !result.IsSafe {
		lobeResult.RequestReplan = true
		lobeResult.ReplanReason = "Safety violation detected: " + strings.Join(result.Concerns, ", ")
	}

	return lobeResult, nil
}

// CanHandle returns confidence based on presence of dangerous patterns.
func (l *SafetyLobe) CanHandle(input string) float64 {
	for _, re := range l.dangerPatterns {
		if re.MatchString(input) {
			return 1.0
		}
	}
	return 0.1 // Always run as sanity check
}

// ResourceEstimate returns minimal resource requirements (regex only).
func (l *SafetyLobe) ResourceEstimate(input brain.LobeInput) brain.ResourceEstimate {
	return brain.ResourceEstimate{
		EstimatedTokens: 10,
		EstimatedTime:   time.Millisecond,
		RequiresGPU:     false,
	}
}
