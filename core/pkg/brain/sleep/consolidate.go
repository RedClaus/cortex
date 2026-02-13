package sleep

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// consolidateMemories performs Phase 1: Memory Consolidation.
// This is analogous to N3 deep sleep where memories are compressed.
// BRAIN AUDIT FIX: Now properly respects context cancellation.
func (sm *SleepManager) consolidateMemories(ctx context.Context) (*ConsolidationResult, error) {
	// Check for cancellation before starting
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	interactions, err := sm.memory.GetInteractionsSince(sm.lastSleep)
	if err != nil {
		return nil, err
	}

	if len(interactions) == 0 {
		return &ConsolidationResult{
			InteractionCount: 0,
			TimeRange: TimeRange{
				Start: sm.lastSleep,
				End:   time.Now(),
			},
		}, nil
	}

	result := &ConsolidationResult{
		InteractionCount: len(interactions),
		TimeRange: TimeRange{
			Start: sm.lastSleep,
			End:   time.Now(),
		},
	}

	// Check for cancellation between phases
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Extract patterns from interactions (like hippocampal replay)
	result.Patterns = sm.extractPatterns(ctx, interactions)

	// Check for cancellation
	select {
	case <-ctx.Done():
		return result, ctx.Err() // Return partial result
	default:
	}

	// Identify emotional signatures
	result.Emotions = sm.extractEmotions(ctx, interactions)

	// Check for cancellation
	select {
	case <-ctx.Done():
		return result, ctx.Err()
	default:
	}

	// Score interaction outcomes
	result.Outcomes = sm.scoreOutcomes(ctx, interactions)

	// Check for cancellation
	select {
	case <-ctx.Done():
		return result, ctx.Err()
	default:
	}

	// Infer user preferences
	result.Preferences = sm.inferPreferences(ctx, interactions)

	sm.log.Debug("[Sleep] Consolidation complete: interactions=%d, patterns=%d, emotions=%d, outcomes=%d, preferences=%d",
		len(interactions), len(result.Patterns), len(result.Emotions), len(result.Outcomes), len(result.Preferences))

	return result, nil
}

// extractPatterns finds recurring patterns in interactions.
func (sm *SleepManager) extractPatterns(ctx context.Context, interactions []Interaction) []Pattern {
	patterns := []Pattern{}

	// Group interactions by type
	typeGroups := make(map[string][]Interaction)
	for _, i := range interactions {
		typeGroups[i.Type] = append(typeGroups[i.Type], i)
	}

	// Find patterns in each group
	for typeName, group := range typeGroups {
		if len(group) >= 3 { // Minimum for a pattern
			examples := make([]string, 0, 3)
			for i := 0; i < len(group) && i < 3; i++ {
				examples = append(examples, group[i].ID)
			}

			patterns = append(patterns, Pattern{
				ID:          generateID(),
				Type:        "request_type",
				Description: fmt.Sprintf("Frequent %s requests (%d occurrences)", typeName, len(group)),
				Frequency:   len(group),
				Confidence:  float64(len(group)) / float64(len(interactions)),
				Examples:    examples,
			})
		}
	}

	// Find timing patterns
	timePatterns := sm.findTimingPatterns(interactions)
	patterns = append(patterns, timePatterns...)

	return patterns
}

// findTimingPatterns finds patterns related to when interactions occur.
func (sm *SleepManager) findTimingPatterns(interactions []Interaction) []Pattern {
	patterns := []Pattern{}

	// Group by hour of day
	hourCounts := make(map[int]int)
	for _, i := range interactions {
		hour := i.Timestamp.Hour()
		hourCounts[hour]++
	}

	// Find peak hours
	var peakHour int
	var peakCount int
	for hour, count := range hourCounts {
		if count > peakCount {
			peakHour = hour
			peakCount = count
		}
	}

	if peakCount >= 3 && float64(peakCount)/float64(len(interactions)) > 0.3 {
		patterns = append(patterns, Pattern{
			ID:          generateID(),
			Type:        "timing",
			Description: fmt.Sprintf("Peak activity around %d:00 (%d interactions)", peakHour, peakCount),
			Frequency:   peakCount,
			Confidence:  float64(peakCount) / float64(len(interactions)),
		})
	}

	return patterns
}

// extractEmotions detects emotional context in interactions.
func (sm *SleepManager) extractEmotions(ctx context.Context, interactions []Interaction) []EmotionSignature {
	emotions := []EmotionSignature{}

	frustrationIndicators := []string{
		"why doesn't", "not working", "wrong", "error", "bug",
		"frustrated", "annoying", "ugh", "!!!",
	}
	satisfactionIndicators := []string{
		"thanks", "perfect", "great", "awesome", "works",
		"exactly what", "nice", "love it", "excellent",
	}
	confusionIndicators := []string{
		"don't understand", "confused", "what do you mean",
		"unclear", "???", "huh", "i don't get",
	}

	for _, i := range interactions {
		userText := strings.ToLower(i.UserMessage)

		// Check for frustration
		for _, indicator := range frustrationIndicators {
			if strings.Contains(userText, indicator) {
				emotions = append(emotions, EmotionSignature{
					Emotion:        "frustrated",
					Intensity:      0.7,
					Context:        truncateString(i.Summary, 100),
					InteractionIDs: []string{i.ID},
				})
				break
			}
		}

		// Check for satisfaction
		for _, indicator := range satisfactionIndicators {
			if strings.Contains(userText, indicator) {
				emotions = append(emotions, EmotionSignature{
					Emotion:        "satisfied",
					Intensity:      0.8,
					Context:        truncateString(i.Summary, 100),
					InteractionIDs: []string{i.ID},
				})
				break
			}
		}

		// Check for confusion
		for _, indicator := range confusionIndicators {
			if strings.Contains(userText, indicator) {
				emotions = append(emotions, EmotionSignature{
					Emotion:        "confused",
					Intensity:      0.6,
					Context:        truncateString(i.Summary, 100),
					InteractionIDs: []string{i.ID},
				})
				break
			}
		}
	}

	// Aggregate similar emotions
	return aggregateEmotions(emotions)
}

// aggregateEmotions combines similar emotion signatures.
func aggregateEmotions(emotions []EmotionSignature) []EmotionSignature {
	aggregated := make(map[string]*EmotionSignature)

	for _, e := range emotions {
		key := e.Emotion
		if existing, ok := aggregated[key]; ok {
			// Merge
			existing.Intensity = (existing.Intensity + e.Intensity) / 2
			existing.InteractionIDs = append(existing.InteractionIDs, e.InteractionIDs...)
			existing.Context = existing.Context + "; " + e.Context
		} else {
			copy := e
			aggregated[key] = &copy
		}
	}

	result := make([]EmotionSignature, 0, len(aggregated))
	for _, e := range aggregated {
		result = append(result, *e)
	}
	return result
}

// scoreOutcomes evaluates how well each interaction went.
func (sm *SleepManager) scoreOutcomes(ctx context.Context, interactions []Interaction) []InteractionOutcome {
	outcomes := []InteractionOutcome{}

	for _, i := range interactions {
		outcome := InteractionOutcome{
			InteractionID:    i.ID,
			Indicators:       []string{},
			UserFeedback:     i.Feedback,
			FeedbackPositive: i.FeedbackPositive,
		}

		// Check for explicit feedback
		if i.Feedback != "" {
			outcome.Success = i.FeedbackPositive
			outcome.Indicators = append(outcome.Indicators, "explicit_feedback")
		} else {
			// Infer success from behavior
			if i.TaskCompleted {
				outcome.Success = true
				outcome.Indicators = append(outcome.Indicators, "task_completed")
			}

			if i.FollowUpCount == 0 {
				outcome.Success = true
				outcome.Indicators = append(outcome.Indicators, "no_followup_needed")
			} else if i.FollowUpCount > 2 {
				outcome.Success = false
				outcome.Indicators = append(outcome.Indicators, "multiple_followups")
			}

			if i.UserCorrected {
				outcome.Success = false
				outcome.Indicators = append(outcome.Indicators, "user_corrected")
			}
		}

		outcomes = append(outcomes, outcome)
	}

	return outcomes
}

// inferPreferences learns user preferences from interactions.
func (sm *SleepManager) inferPreferences(ctx context.Context, interactions []Interaction) []UserPreference {
	preferences := []UserPreference{}

	// Analyze response length preferences
	if pref := sm.analyzeResponseLengthPreference(interactions); pref != nil {
		preferences = append(preferences, *pref)
	}

	// Analyze code vs explanation preferences
	if pref := sm.analyzeCodePreference(interactions); pref != nil {
		preferences = append(preferences, *pref)
	}

	return preferences
}

// analyzeResponseLengthPreference determines if user prefers short or long responses.
func (sm *SleepManager) analyzeResponseLengthPreference(interactions []Interaction) *UserPreference {
	var shortPreferred, longPreferred int
	var evidence []string

	for _, i := range interactions {
		msg := strings.ToLower(i.UserMessage)

		if containsAny(msg, []string{"briefly", "short", "concise", "tldr", "summary", "quick"}) {
			shortPreferred++
			evidence = append(evidence, i.ID)
		}
		if containsAny(msg, []string{"explain more", "in detail", "elaborate", "tell me more", "step by step"}) {
			longPreferred++
			evidence = append(evidence, i.ID)
		}
	}

	total := shortPreferred + longPreferred
	if total < 3 {
		return nil // Not enough data
	}

	if float64(shortPreferred)/float64(total) > 0.7 {
		return &UserPreference{
			Category:   "response_length",
			Preference: "concise responses preferred",
			Confidence: float64(shortPreferred) / float64(total),
			Evidence:   evidence,
		}
	}

	if float64(longPreferred)/float64(total) > 0.7 {
		return &UserPreference{
			Category:   "response_length",
			Preference: "detailed responses preferred",
			Confidence: float64(longPreferred) / float64(total),
			Evidence:   evidence,
		}
	}

	return nil
}

// analyzeCodePreference determines if user prefers code examples.
func (sm *SleepManager) analyzeCodePreference(interactions []Interaction) *UserPreference {
	var codeRequests int
	var evidence []string

	for _, i := range interactions {
		msg := strings.ToLower(i.UserMessage)

		if containsAny(msg, []string{"show me code", "example", "snippet", "how do i code", "implementation"}) {
			codeRequests++
			evidence = append(evidence, i.ID)
		}
	}

	if codeRequests >= 3 && float64(codeRequests)/float64(len(interactions)) > 0.3 {
		return &UserPreference{
			Category:   "response_style",
			Preference: "prefers code examples over explanations",
			Confidence: float64(codeRequests) / float64(len(interactions)),
			Evidence:   evidence,
		}
	}

	return nil
}

// Utility functions

func generateID() string {
	return uuid.New().String()[:8]
}

func containsAny(s string, substrs []string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
