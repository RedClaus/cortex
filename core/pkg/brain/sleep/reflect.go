package sleep

import (
	"context"
	"fmt"
)

// reflectOnPatterns performs Phase 2: Reflection.
// This is analogous to REM sleep where the brain processes and makes sense of experiences.
func (sm *SleepManager) reflectOnPatterns(ctx context.Context, consolidated *ConsolidationResult) ([]ReflectionInsight, error) {
	insights := []ReflectionInsight{}

	// Find strengths (what worked well)
	strengths := sm.identifyStrengths(consolidated)
	insights = append(insights, strengths...)

	// Find weaknesses (what didn't work)
	weaknesses := sm.identifyWeaknesses(consolidated)
	insights = append(insights, weaknesses...)

	// Find opportunities (potential improvements)
	opportunities := sm.identifyOpportunities(consolidated)
	insights = append(insights, opportunities...)

	// Analyze patterns for insights
	patternInsights := sm.analyzePatternInsights(consolidated.Patterns)
	insights = append(insights, patternInsights...)

	sm.log.Debug("[Sleep] Reflection complete: strengths=%d, weaknesses=%d, opportunities=%d, pattern_insights=%d",
		len(strengths), len(weaknesses), len(opportunities), len(patternInsights))

	return insights, nil
}

// identifyStrengths finds things that went well.
func (sm *SleepManager) identifyStrengths(c *ConsolidationResult) []ReflectionInsight {
	insights := []ReflectionInsight{}

	// Calculate success rate
	successful := []InteractionOutcome{}
	for _, o := range c.Outcomes {
		if o.Success {
			successful = append(successful, o)
		}
	}

	if len(c.Outcomes) > 0 {
		successRate := float64(len(successful)) / float64(len(c.Outcomes))
		if successRate > 0.8 {
			evidence := make([]string, 0, 5)
			for i := 0; i < len(successful) && i < 5; i++ {
				evidence = append(evidence, successful[i].InteractionID)
			}

			insights = append(insights, ReflectionInsight{
				ID:          generateID(),
				Category:    "strength",
				Description: fmt.Sprintf("High success rate: %.0f%% of interactions were successful", successRate*100),
				Confidence:  successRate,
				Evidence:    evidence,
			})
		}
	}

	// Find positive emotional responses
	for _, e := range c.Emotions {
		if e.Emotion == "satisfied" && e.Intensity > 0.7 {
			insights = append(insights, ReflectionInsight{
				ID:          generateID(),
				Category:    "strength",
				Description: fmt.Sprintf("User satisfaction detected: %s", e.Context),
				Confidence:  e.Intensity,
				Evidence:    e.InteractionIDs,
			})
		}
	}

	return insights
}

// identifyWeaknesses finds areas that need improvement.
func (sm *SleepManager) identifyWeaknesses(c *ConsolidationResult) []ReflectionInsight {
	insights := []ReflectionInsight{}

	// Find failed interactions
	for _, o := range c.Outcomes {
		if !o.Success {
			if contains(o.Indicators, "user_corrected") {
				insights = append(insights, ReflectionInsight{
					ID:            generateID(),
					Category:      "weakness",
					Description:   "Response required user correction",
					Confidence:    0.9,
					Evidence:      []string{o.InteractionID},
					ActionableFor: []string{"confidence", "accuracy"},
				})
			}

			if contains(o.Indicators, "multiple_followups") {
				insights = append(insights, ReflectionInsight{
					ID:            generateID(),
					Category:      "weakness",
					Description:   "Multiple follow-ups needed - initial response incomplete",
					Confidence:    0.8,
					Evidence:      []string{o.InteractionID},
					ActionableFor: []string{"verbosity", "thoroughness"},
				})
			}
		}
	}

	// Find frustration and confusion patterns
	for _, e := range c.Emotions {
		if e.Emotion == "frustrated" && len(e.InteractionIDs) > 0 {
			insights = append(insights, ReflectionInsight{
				ID:            generateID(),
				Category:      "weakness",
				Description:   fmt.Sprintf("User frustration detected: %s", e.Context),
				Confidence:    e.Intensity,
				Evidence:      e.InteractionIDs,
				ActionableFor: []string{"patience", "clarity"},
			})
		}

		if e.Emotion == "confused" && len(e.InteractionIDs) > 0 {
			insights = append(insights, ReflectionInsight{
				ID:            generateID(),
				Category:      "weakness",
				Description:   fmt.Sprintf("User confusion detected: %s", e.Context),
				Confidence:    e.Intensity,
				Evidence:      e.InteractionIDs,
				ActionableFor: []string{"verbosity", "directness"},
			})
		}
	}

	return insights
}

// identifyOpportunities finds potential improvements from preferences.
func (sm *SleepManager) identifyOpportunities(c *ConsolidationResult) []ReflectionInsight {
	insights := []ReflectionInsight{}

	for _, pref := range c.Preferences {
		insights = append(insights, ReflectionInsight{
			ID:            generateID(),
			Category:      "opportunity",
			Description:   fmt.Sprintf("User preference detected: %s", pref.Preference),
			Confidence:    pref.Confidence,
			Evidence:      pref.Evidence,
			ActionableFor: []string{pref.Category},
		})
	}

	return insights
}

// analyzePatternInsights creates insights from detected patterns.
func (sm *SleepManager) analyzePatternInsights(patterns []Pattern) []ReflectionInsight {
	insights := []ReflectionInsight{}

	for _, p := range patterns {
		if p.Frequency >= 5 && p.Confidence > 0.6 {
			insights = append(insights, ReflectionInsight{
				ID:          generateID(),
				Category:    "pattern",
				Description: p.Description,
				Confidence:  p.Confidence,
				Evidence:    p.Examples,
			})
		}
	}

	return insights
}

// contains checks if a slice contains a string.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
