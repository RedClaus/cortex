package traces

import (
	"time"
)

// TraceScorer evaluates trace quality and reuse potential.
type TraceScorer struct {
	// Weights for different scoring factors
	OutcomeWeight    float64 // Weight for successful outcome (default: 0.4)
	EfficiencyWeight float64 // Weight for step efficiency (default: 0.2)
	RecencyWeight    float64 // Weight for recency (default: 0.15)
	ReuseWeight      float64 // Weight for reuse count (default: 0.15)
	DurationWeight   float64 // Weight for fast execution (default: 0.1)
}

// NewTraceScorer creates a scorer with default weights.
func NewTraceScorer() *TraceScorer {
	return &TraceScorer{
		OutcomeWeight:    0.4,
		EfficiencyWeight: 0.2,
		RecencyWeight:    0.15,
		ReuseWeight:      0.15,
		DurationWeight:   0.1,
	}
}

// Score calculates a comprehensive quality score for a trace.
// Returns a value between 0 and 1.
func (s *TraceScorer) Score(trace *ReasoningTrace) float64 {
	if trace == nil {
		return 0.0
	}

	score := 0.0

	// Outcome score (0-1)
	outcomeScore := s.scoreOutcome(trace.Outcome)
	score += outcomeScore * s.OutcomeWeight

	// Efficiency score (fewer steps = better, 0-1)
	efficiencyScore := s.scoreEfficiency(len(trace.Steps))
	score += efficiencyScore * s.EfficiencyWeight

	// Recency score (more recent = better, 0-1)
	recencyScore := s.scoreRecency(trace.CreatedAt, trace.LastUsedAt)
	score += recencyScore * s.RecencyWeight

	// Reuse score (more reuses = proven value, 0-1)
	reuseScore := s.scoreReuse(trace.ReusedCount)
	score += reuseScore * s.ReuseWeight

	// Duration score (faster = better, 0-1)
	durationScore := s.scoreDuration(trace.TotalDuration)
	score += durationScore * s.DurationWeight

	// Ensure bounds
	if score > 1.0 {
		score = 1.0
	}
	if score < 0.0 {
		score = 0.0
	}

	return score
}

// scoreOutcome converts outcome to a score.
func (s *TraceScorer) scoreOutcome(outcome TraceOutcome) float64 {
	switch outcome {
	case OutcomeSuccess:
		return 1.0
	case OutcomePartial:
		return 0.5
	case OutcomeInterrupted:
		return 0.3
	case OutcomeFailed:
		return 0.0
	default:
		return 0.5
	}
}

// scoreEfficiency rewards traces with fewer steps.
func (s *TraceScorer) scoreEfficiency(stepCount int) float64 {
	if stepCount <= 1 {
		return 1.0
	}
	if stepCount <= 3 {
		return 0.9
	}
	if stepCount <= 5 {
		return 0.7
	}
	if stepCount <= 10 {
		return 0.5
	}
	if stepCount <= 15 {
		return 0.3
	}
	return 0.1
}

// scoreRecency rewards more recent traces.
func (s *TraceScorer) scoreRecency(createdAt, lastUsedAt time.Time) float64 {
	// Use the more recent of created or last used
	relevantTime := createdAt
	if lastUsedAt.After(createdAt) {
		relevantTime = lastUsedAt
	}

	age := time.Since(relevantTime)

	if age < 24*time.Hour {
		return 1.0
	}
	if age < 7*24*time.Hour {
		return 0.8
	}
	if age < 30*24*time.Hour {
		return 0.6
	}
	if age < 90*24*time.Hour {
		return 0.4
	}
	return 0.2
}

// scoreReuse rewards traces that have been successfully reused.
func (s *TraceScorer) scoreReuse(reuseCount int) float64 {
	if reuseCount >= 10 {
		return 1.0
	}
	if reuseCount >= 5 {
		return 0.9
	}
	if reuseCount >= 3 {
		return 0.7
	}
	if reuseCount >= 1 {
		return 0.5
	}
	return 0.0 // Never reused yet
}

// scoreDuration rewards faster traces.
func (s *TraceScorer) scoreDuration(duration time.Duration) float64 {
	if duration < 1*time.Second {
		return 1.0
	}
	if duration < 5*time.Second {
		return 0.8
	}
	if duration < 15*time.Second {
		return 0.6
	}
	if duration < 30*time.Second {
		return 0.4
	}
	if duration < 60*time.Second {
		return 0.2
	}
	return 0.1
}

// ShouldPrune determines if a trace should be pruned based on its score and age.
func (s *TraceScorer) ShouldPrune(trace *ReasoningTrace, maxAge time.Duration, minScore float64) bool {
	// Never prune traces that have been reused
	if trace.ReusedCount > 0 {
		return false
	}

	// Never prune recent traces
	if time.Since(trace.CreatedAt) < 24*time.Hour {
		return false
	}

	// Check age threshold
	if time.Since(trace.CreatedAt) > maxAge {
		return true
	}

	// Check score threshold
	score := s.Score(trace)
	if score < minScore {
		return true
	}

	return false
}

// RankTraces sorts traces by their scores (highest first).
func (s *TraceScorer) RankTraces(traces []*ReasoningTrace) []*ReasoningTrace {
	if len(traces) <= 1 {
		return traces
	}

	// Calculate scores
	type scored struct {
		trace *ReasoningTrace
		score float64
	}
	scoredTraces := make([]scored, len(traces))
	for i, t := range traces {
		scoredTraces[i] = scored{trace: t, score: s.Score(t)}
	}

	// Sort by score descending
	for i := 0; i < len(scoredTraces)-1; i++ {
		for j := i + 1; j < len(scoredTraces); j++ {
			if scoredTraces[j].score > scoredTraces[i].score {
				scoredTraces[i], scoredTraces[j] = scoredTraces[j], scoredTraces[i]
			}
		}
	}

	// Extract sorted traces
	result := make([]*ReasoningTrace, len(traces))
	for i, st := range scoredTraces {
		result[i] = st.trace
	}

	return result
}

// SelectBestTrace chooses the best trace from candidates for reuse.
// Considers both similarity and quality score.
func (s *TraceScorer) SelectBestTrace(candidates []TraceSimilarity) *TraceSimilarity {
	if len(candidates) == 0 {
		return nil
	}

	var best *TraceSimilarity
	var bestCombined float64

	for i := range candidates {
		c := &candidates[i]
		qualityScore := s.Score(c.Trace)

		// Combined score: weighted average of similarity and quality
		// Similarity is more important (0.6) but quality matters too (0.4)
		combined := c.Similarity*0.6 + qualityScore*0.4

		if combined > bestCombined {
			best = c
			bestCombined = combined
		}
	}

	return best
}
