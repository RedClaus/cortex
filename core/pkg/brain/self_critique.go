package brain

import (
	"context"
	"fmt"
	"time"
)

const (
	// DefaultMaxRetries is the default number of refinement attempts.
	DefaultMaxRetries = 3
	// DefaultMinConfidence is the default minimum confidence threshold.
	DefaultMinConfidence = 0.7
)

// CritiqueResult holds the outcome of self-critique evaluation.
type CritiqueResult struct {
	// Content is the (potentially refined) response content.
	Content interface{} `json:"content"`
	// Confidence is the final confidence score after critique.
	Confidence float64 `json:"confidence"`
	// AttemptCount is how many refinement attempts were made.
	AttemptCount int `json:"attempt_count"`
	// SafetyConcerns lists any safety issues identified.
	SafetyConcerns []string `json:"safety_concerns,omitempty"`
	// Refinements lists suggested improvements.
	Refinements []string `json:"refinements,omitempty"`
	// NeedsReplan indicates if the response requires replanning.
	NeedsReplan bool `json:"needs_replan"`
	// Duration is how long the critique process took.
	Duration time.Duration `json:"duration"`
}

// MetacognitionLobeInterface defines the interface for metacognition.
// This allows the actual MetacognitionLobe from pkg/brain/lobes to be injected.
type MetacognitionLobeInterface interface {
	// ID returns the lobe identifier.
	ID() LobeID
	// Process runs metacognitive assessment.
	Process(ctx context.Context, input LobeInput, bb *Blackboard) (*LobeResult, error)
}

// InhibitionLobeInterface defines the interface for inhibition checks.
// This allows the actual InhibitionLobe from pkg/brain/lobes to be injected.
type InhibitionLobeInterface interface {
	// ID returns the lobe identifier.
	ID() LobeID
	// Process runs safety and inhibition checks.
	Process(ctx context.Context, input LobeInput, bb *Blackboard) (*LobeResult, error)
}

// SelfCritiqueLoop evaluates outputs and requests refinement if needed.
// It implements the Self-Critique Evaluator pattern from Agentic Patterns.
//
// Brain Alignment: Like the prefrontal cortex's monitoring function that
// evaluates actions before they're taken, catching errors and biases.
// The metacognition lobe assesses quality while inhibition catches risks.
type SelfCritiqueLoop struct {
	// metacog provides metacognitive assessment.
	metacog MetacognitionLobeInterface
	// inhibition provides safety and inhibition checks.
	inhibition InhibitionLobeInterface
	// maxRetries is the maximum number of refinement attempts.
	maxRetries int
	// minConfidence is the minimum acceptable confidence score.
	minConfidence float64
}

// SelfCritiqueOption configures the SelfCritiqueLoop.
type SelfCritiqueOption func(*SelfCritiqueLoop)

// WithMaxRetries sets the maximum number of refinement attempts.
func WithMaxRetries(n int) SelfCritiqueOption {
	return func(scl *SelfCritiqueLoop) {
		if n > 0 {
			scl.maxRetries = n
		}
	}
}

// WithMinConfidence sets the minimum acceptable confidence threshold.
func WithMinConfidence(threshold float64) SelfCritiqueOption {
	return func(scl *SelfCritiqueLoop) {
		if threshold >= 0 && threshold <= 1 {
			scl.minConfidence = threshold
		}
	}
}

// NewSelfCritiqueLoop creates a new SelfCritiqueLoop with the given lobes and options.
func NewSelfCritiqueLoop(metacog MetacognitionLobeInterface, inhibition InhibitionLobeInterface, opts ...SelfCritiqueOption) *SelfCritiqueLoop {
	scl := &SelfCritiqueLoop{
		metacog:       metacog,
		inhibition:    inhibition,
		maxRetries:    DefaultMaxRetries,
		minConfidence: DefaultMinConfidence,
	}

	for _, opt := range opts {
		opt(scl)
	}

	return scl
}

// Evaluate runs self-critique on a parallel execution result.
// It combines metacognition assessment with safety checks to determine
// whether the result is acceptable or needs refinement.
//
// The evaluation flow:
// 1. Run metacognition to assess response quality
// 2. Run inhibition to check for safety concerns
// 3. Combine confidence scores with safety penalties
// 4. If confidence meets threshold, return early
// 5. Otherwise, set critique feedback on blackboard for replanning
func (scl *SelfCritiqueLoop) Evaluate(ctx context.Context, result *ParallelExecutionResult, bb *Blackboard) (*CritiqueResult, error) {
	startTime := time.Now()

	// Early return if lobes are not available
	if scl.metacog == nil || scl.inhibition == nil {
		return &CritiqueResult{
			Content:      result.Content,
			Confidence:   result.Confidence,
			AttemptCount: 0,
			Duration:     time.Since(startTime),
		}, nil
	}

	var safetyConcerns []string
	var refinements []string

	for attempt := 0; attempt < scl.maxRetries; attempt++ {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return &CritiqueResult{
				Content:        result.Content,
				Confidence:     result.Confidence,
				AttemptCount:   attempt,
				SafetyConcerns: safetyConcerns,
				Refinements:    refinements,
				Duration:       time.Since(startTime),
			}, ctx.Err()
		default:
		}

		// Step 1: Run metacognition assessment
		contentStr := fmt.Sprintf("%v", result.Content)
		metacogInput := LobeInput{
			RawInput: fmt.Sprintf("Evaluate this response:\n%s", contentStr),
		}

		assessment, err := scl.metacog.Process(ctx, metacogInput, bb)
		if err != nil {
			// If critique fails, return original result
			return &CritiqueResult{
				Content:      result.Content,
				Confidence:   result.Confidence,
				AttemptCount: attempt,
				Duration:     time.Since(startTime),
			}, nil
		}

		// Extract metacognition results if available
		metacogConfidence := assessment.Confidence
		if metaResult, ok := assessment.Content.(MetacognitionResult); ok {
			metacogConfidence = metaResult.ConfidenceAssessment
			refinements = append(refinements, metaResult.SuggestedRefinements...)
		}

		// Step 2: Run inhibition for safety concerns
		inhibInput := LobeInput{
			RawInput: contentStr,
		}

		inhibResult, inhibErr := scl.inhibition.Process(ctx, inhibInput, bb)

		// Calculate combined confidence
		combinedConfidence := (metacogConfidence + result.Confidence) / 2

		// Apply safety penalty if inhibition found issues
		if inhibErr == nil && inhibResult != nil {
			if inhibResult.RequestReplan {
				// Significant penalty for safety concerns
				combinedConfidence *= 0.5

				// Extract safety concerns if available
				if inhibContent, ok := inhibResult.Content.(InhibitionResult); ok {
					safetyConcerns = append(safetyConcerns, inhibContent.RiskFactors...)
				}
			}
		}

		// Step 3: Check if confidence meets threshold
		if combinedConfidence >= scl.minConfidence {
			return &CritiqueResult{
				Content:        result.Content,
				Confidence:     combinedConfidence,
				AttemptCount:   attempt + 1,
				SafetyConcerns: safetyConcerns,
				Refinements:    refinements,
				NeedsReplan:    false,
				Duration:       time.Since(startTime),
			}, nil
		}

		// Step 4: Check if replan was suggested by metacognition
		if !assessment.RequestReplan {
			// Accept result even with lower confidence if no replan suggested
			return &CritiqueResult{
				Content:        result.Content,
				Confidence:     combinedConfidence,
				AttemptCount:   attempt + 1,
				SafetyConcerns: safetyConcerns,
				Refinements:    refinements,
				NeedsReplan:    false,
				Duration:       time.Since(startTime),
			}, nil
		}

		// Step 5: Set critique feedback on blackboard for replanning
		feedbackContent := fmt.Sprintf("%v", assessment.Content)
		bb.Set("critique_feedback", feedbackContent)
		bb.Set("critique_attempt", attempt+1)
		bb.Set("critique_confidence", combinedConfidence)

		// Update result confidence for next iteration
		result.Confidence = combinedConfidence
	}

	// Max retries reached - return best effort result
	return &CritiqueResult{
		Content:        result.Content,
		Confidence:     result.Confidence,
		AttemptCount:   scl.maxRetries,
		SafetyConcerns: safetyConcerns,
		Refinements:    refinements,
		NeedsReplan:    true, // Indicate that replanning might still help
		Duration:       time.Since(startTime),
	}, nil
}

// EvaluateLobeResult is a convenience method for evaluating a LobeResult.
// It wraps the LobeResult in a ParallelExecutionResult for unified processing.
func (scl *SelfCritiqueLoop) EvaluateLobeResult(ctx context.Context, result *LobeResult, bb *Blackboard) (*CritiqueResult, error) {
	parallelResult := &ParallelExecutionResult{
		Content:     result.Content,
		Confidence:  result.Confidence,
		BranchCount: 1,
	}
	return scl.Evaluate(ctx, parallelResult, bb)
}

// MetacognitionResult mirrors the type from lobes/metacognition.go.
// This allows extracting structured data from metacognition assessment.
type MetacognitionResult struct {
	ConfidenceAssessment float64  `json:"confidence_assessment"`
	ProcessingQuality    float64  `json:"processing_quality"`
	SuggestedRefinements []string `json:"suggested_refinements"`
	KnowledgeGaps        []string `json:"knowledge_gaps"`
}

// InhibitionResult mirrors the type from lobes/inhibition.go.
// This allows extracting structured data from inhibition checks.
type InhibitionResult struct {
	ShouldInhibit     bool     `json:"should_inhibit"`
	InhibitedAreas    []string `json:"inhibited_areas"`
	RiskFactors       []string `json:"risk_factors"`
	RecommendedAction string   `json:"recommended_action"`
}
