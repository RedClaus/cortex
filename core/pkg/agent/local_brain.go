package agent

import (
	"context"
	"time"

	"github.com/normanking/cortex/pkg/brain"
)

// LocalBrain wraps the existing Brain (Executive) to implement BrainInterface.
type LocalBrain struct {
	executive *brain.Executive
	available bool
}

// NewLocalBrain creates a LocalBrain from an existing Executive.
func NewLocalBrain(exec *brain.Executive) *LocalBrain {
	return &LocalBrain{
		executive: exec,
		available: exec != nil,
	}
}

// Type returns "local".
func (l *LocalBrain) Type() string {
	return "local"
}

// Available returns true if the executive is configured.
func (l *LocalBrain) Available() bool {
	return l.available && l.executive != nil
}

// Process sends a request through the lobe-based brain.
func (l *LocalBrain) Process(ctx context.Context, input *BrainInput) (*BrainResult, error) {
	startTime := time.Now()

	result := &BrainResult{
		Source: "local",
	}

	if !l.Available() {
		result.Success = false
		result.Error = "local brain not available"
		result.Latency = time.Since(startTime)
		return result, nil
	}

	// Build the prompt for the executive
	prompt := input.Query
	if input.SystemPrompt != "" {
		prompt = input.SystemPrompt + "\n\n" + prompt
	}

	// Process through the executive (lobe-based brain)
	execResult, err := l.executive.Process(ctx, prompt)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		result.Latency = time.Since(startTime)
		return result, err
	}

	// Extract content from result
	result.Content = extractContent(execResult.FinalContent)
	result.Success = true
	result.Confidence = calculateConfidence(execResult)
	result.Latency = time.Since(startTime)

	// Add model info if available
	if execResult.Classification != nil {
		result.Model = string(execResult.Classification.PrimaryLobe)
	}

	return result, nil
}

// extractContent converts ExecutionResult content to string.
func extractContent(content interface{}) string {
	if content == nil {
		return ""
	}
	switch v := content.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return ""
	}
}

// calculateConfidence derives a confidence score from the execution result.
func calculateConfidence(result *brain.ExecutionResult) float64 {
	if result == nil {
		return 0.5
	}

	// Start with base confidence
	confidence := 0.7

	// Adjust based on phase results
	if len(result.Phases) > 0 {
		totalConf := 0.0
		lobeCount := 0
		for _, pr := range result.Phases {
			for _, lr := range pr.LobeResults {
				totalConf += lr.Confidence
				lobeCount++
			}
		}
		if lobeCount > 0 {
			confidence = totalConf / float64(lobeCount)
		}
	}

	// Cap at reasonable bounds
	if confidence < 0.3 {
		confidence = 0.3
	}
	if confidence > 0.95 {
		confidence = 0.95
	}

	return confidence
}
