package health

import (
	"fmt"

	"github.com/normanking/cortex/pkg/brain/context"
)

// Detector analyzes context for degradation patterns.
type Detector struct {
	config HealthConfig
}

// NewDetector creates a new detector with the given configuration.
func NewDetector(config HealthConfig) *Detector {
	return &Detector{config: config}
}

// NewDefaultDetector creates a detector with default configuration.
func NewDefaultDetector() *Detector {
	return NewDetector(DefaultHealthConfig())
}

// Detect runs all enabled detection algorithms on the blackboard.
// Returns detected patterns (may be empty if none found).
func (d *Detector) Detect(bb *context.AttentionBlackboard) []DetectedPattern {
	var patterns []DetectedPattern

	if d.config.LostInMiddle.Enabled {
		if pattern := d.detectLostInMiddle(bb); pattern != nil {
			patterns = append(patterns, *pattern)
		}
	}

	// Future detectors will be added here when proven necessary:
	// if d.config.Poisoning.Enabled { ... }
	// if d.config.Distraction.Enabled { ... }

	return patterns
}

// detectLostInMiddle checks if high-priority items are buried in the middle zone.
//
// The "Lost in the Middle" problem (Liu et al., 2023) shows that LLMs have
// reduced attention to content in the middle of their context window.
// High-priority information should be at the beginning or end.
//
// Detection criteria:
// 1. High-priority items (>0.7) in the Supporting zone
// 2. Supporting zone taking >60% of total tokens
func (d *Detector) detectLostInMiddle(bb *context.AttentionBlackboard) *DetectedPattern {
	stats := bb.Stats()
	config := d.config.LostInMiddle

	// Check 1: Are high-priority items in the middle?
	supportingItems := bb.GetZone(context.ZoneSupporting)
	var highPriorityInMiddle []*context.ContextItem

	for _, item := range supportingItems {
		if item.Priority >= config.HighPriorityThreshold {
			highPriorityInMiddle = append(highPriorityInMiddle, item)
		}
	}

	// Check 2: Is the middle zone overloaded?
	middleRatio := float64(stats.SupportingTokens) / float64(stats.TotalTokens)
	if stats.TotalTokens == 0 {
		middleRatio = 0
	}
	middleOverloaded := middleRatio > config.MaxMiddleRatio

	// Calculate severity
	if len(highPriorityInMiddle) == 0 && !middleOverloaded {
		return nil // No issue detected
	}

	severity := 0.0
	var description string
	var affectedIDs []string

	if len(highPriorityInMiddle) > 0 {
		// Severity increases with number of high-priority items in middle
		severity += float64(len(highPriorityInMiddle)) * 0.2
		if severity > 0.5 {
			severity = 0.5
		}

		for _, item := range highPriorityInMiddle {
			affectedIDs = append(affectedIDs, item.ID)
		}
		description = fmt.Sprintf("%d high-priority items in middle zone (low attention area)", len(highPriorityInMiddle))
	}

	if middleOverloaded {
		overloadSeverity := (middleRatio - config.MaxMiddleRatio) / (1.0 - config.MaxMiddleRatio)
		severity += overloadSeverity * 0.5
		if description != "" {
			description += "; "
		}
		description += fmt.Sprintf("middle zone at %.0f%% (threshold: %.0f%%)",
			middleRatio*100, config.MaxMiddleRatio*100)
	}

	if severity > 1.0 {
		severity = 1.0
	}

	return &DetectedPattern{
		Pattern:       PatternLostInMiddle,
		Severity:      severity,
		Description:   description,
		AffectedItems: affectedIDs,
	}
}

// GenerateRecommendations suggests remediation actions for detected patterns.
func (d *Detector) GenerateRecommendations(patterns []DetectedPattern) []string {
	var recommendations []string

	for _, pattern := range patterns {
		switch pattern.Pattern {
		case PatternLostInMiddle:
			recommendations = append(recommendations, d.lostInMiddleRecommendations(pattern)...)
		}
	}

	return recommendations
}

// lostInMiddleRecommendations generates remediation suggestions for LostInMiddle.
func (d *Detector) lostInMiddleRecommendations(pattern DetectedPattern) []string {
	var recs []string

	if len(pattern.AffectedItems) > 0 {
		recs = append(recs,
			"Move high-priority items from Supporting to Critical or Actionable zone")
	}

	if pattern.Severity > 0.5 {
		recs = append(recs,
			"Consider compacting the Supporting zone to reduce middle content")
	}

	if pattern.Severity > 0.7 {
		recs = append(recs,
			"Prioritize pruning low-priority Supporting zone items")
	}

	return recs
}

// ShouldPromote returns true if an item should be promoted from Supporting zone.
// Used by compaction to decide what to move rather than prune.
func (d *Detector) ShouldPromote(item *context.ContextItem) bool {
	if item.Zone != context.ZoneSupporting {
		return false
	}
	return item.Priority >= d.config.LostInMiddle.HighPriorityThreshold
}

// SuggestZone suggests the optimal zone for an item based on its properties.
func (d *Detector) SuggestZone(item *context.ContextItem) context.AttentionZone {
	// High priority items should be at beginning or end
	if item.Priority >= d.config.LostInMiddle.HighPriorityThreshold {
		// Prefer Actionable (end) for recent items, Critical (beginning) for older
		if item.Age().Minutes() < 5 {
			return context.ZoneActionable
		}
		return context.ZoneCritical
	}

	// Task and output categories should be actionable
	if item.Category == context.CategoryTask || item.Category == context.CategoryOutput {
		return context.ZoneActionable
	}

	// System and user context should be critical
	if item.Category == context.CategorySystem || item.Category == context.CategoryUser {
		return context.ZoneCritical
	}

	// Default to supporting
	return context.ZoneSupporting
}
