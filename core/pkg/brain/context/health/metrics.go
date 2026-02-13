package health

import (
	"time"

	"github.com/normanking/cortex/pkg/brain/context"
)

// HealthChecker performs health assessments on the AttentionBlackboard.
type HealthChecker struct {
	detector *Detector
	triggers *TriggerManager
	config   HealthConfig
}

// NewHealthChecker creates a new health checker with default configuration.
func NewHealthChecker() *HealthChecker {
	return NewHealthCheckerWithConfig(DefaultHealthConfig())
}

// NewHealthCheckerWithConfig creates a health checker with custom configuration.
func NewHealthCheckerWithConfig(config HealthConfig) *HealthChecker {
	return &HealthChecker{
		detector: NewDetector(config),
		triggers: NewTriggerManager(),
		config:   config,
	}
}

// Check performs a health assessment on the blackboard.
// Returns a comprehensive health report.
func (hc *HealthChecker) Check(bb *context.AttentionBlackboard) HealthReport {
	start := time.Now()

	// Get stats
	stats := bb.Stats()

	// Detect patterns
	patterns := hc.detector.Detect(bb)

	// Calculate score
	score := hc.calculateScore(stats, patterns)

	// Generate recommendations
	recommendations := hc.detector.GenerateRecommendations(patterns)

	// Determine status
	status := hc.scoreToStatus(score)

	return HealthReport{
		Status:          status,
		Score:           score,
		Patterns:        patterns,
		Recommendations: recommendations,
		Stats:           stats,
		Timestamp:       time.Now(),
		Duration:        time.Since(start),
	}
}

// calculateScore computes an overall health score (0-100).
// Higher is better.
func (hc *HealthChecker) calculateScore(stats context.BlackboardStats, patterns []DetectedPattern) int {
	score := 100.0

	// Deduct for detected patterns (max 50 points)
	patternPenalty := 0.0
	for _, pattern := range patterns {
		patternPenalty += pattern.Severity * 25 // Each pattern can cost up to 25 points
	}
	if patternPenalty > 50 {
		patternPenalty = 50
	}
	score -= patternPenalty

	// Deduct for high utilization (max 20 points)
	if stats.Utilization > 0.9 {
		score -= 20
	} else if stats.Utilization > 0.75 {
		score -= 10
	} else if stats.Utilization > 0.5 {
		score -= 5
	}

	// Deduct for zone imbalance (max 15 points)
	if stats.TotalTokens > 0 {
		supportingRatio := float64(stats.SupportingTokens) / float64(stats.TotalTokens)
		if supportingRatio > 0.7 {
			score -= 15 // Too much in middle
		} else if supportingRatio > 0.6 {
			score -= 8
		}
	}

	// Deduct for too few items in critical/actionable (max 15 points)
	if stats.CriticalItems == 0 && stats.TotalItems > 0 {
		score -= 10 // No system context
	}
	if stats.ActionableItems == 0 && stats.TotalItems > 3 {
		score -= 5 // No actionable items
	}

	// Clamp to 0-100
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return int(score)
}

// scoreToStatus converts a numeric score to a status.
func (hc *HealthChecker) scoreToStatus(score int) HealthStatus {
	if score < hc.config.CriticalThreshold {
		return StatusCritical
	}
	if score < hc.config.DegradationThreshold {
		return StatusDegraded
	}
	return StatusHealthy
}

// Triggers returns the trigger manager for registering callbacks.
func (hc *HealthChecker) Triggers() *TriggerManager {
	return hc.triggers
}

// Detector returns the detector for direct access.
func (hc *HealthChecker) Detector() *Detector {
	return hc.detector
}

// QuickCheck performs a fast check without full pattern detection.
// Use for frequent checks where full analysis is too expensive.
func (hc *HealthChecker) QuickCheck(bb *context.AttentionBlackboard) (HealthStatus, int) {
	stats := bb.Stats()

	// Quick score based on utilization and balance only
	score := 100

	// Utilization penalty
	if stats.Utilization > 0.9 {
		score -= 30
	} else if stats.Utilization > 0.75 {
		score -= 15
	}

	// Balance penalty
	if stats.TotalTokens > 0 {
		supportingRatio := float64(stats.SupportingTokens) / float64(stats.TotalTokens)
		if supportingRatio > 0.7 {
			score -= 20
		}
	}

	return hc.scoreToStatus(score), score
}

// NeedsCompaction returns true if the blackboard would benefit from compaction.
func (hc *HealthChecker) NeedsCompaction(bb *context.AttentionBlackboard) bool {
	stats := bb.Stats()

	// High utilization
	if stats.Utilization > 0.85 {
		return true
	}

	// Pattern detected with high severity
	patterns := hc.detector.Detect(bb)
	for _, p := range patterns {
		if p.Severity > 0.5 {
			return true
		}
	}

	return false
}

// CompactionPriority returns how urgently compaction is needed (0.0-1.0).
// Higher means more urgent.
func (hc *HealthChecker) CompactionPriority(bb *context.AttentionBlackboard) float64 {
	stats := bb.Stats()
	priority := 0.0

	// Utilization contributes up to 0.5
	if stats.Utilization > 0.9 {
		priority += 0.5
	} else if stats.Utilization > 0.75 {
		priority += 0.3
	} else if stats.Utilization > 0.5 {
		priority += 0.1
	}

	// Patterns contribute up to 0.5
	patterns := hc.detector.Detect(bb)
	maxSeverity := 0.0
	for _, p := range patterns {
		if p.Severity > maxSeverity {
			maxSeverity = p.Severity
		}
	}
	priority += maxSeverity * 0.5

	if priority > 1.0 {
		priority = 1.0
	}

	return priority
}
