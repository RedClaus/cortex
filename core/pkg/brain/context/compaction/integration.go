package compaction

import (
	"github.com/normanking/cortex/internal/bus"
	"github.com/normanking/cortex/pkg/brain/context"
	"github.com/normanking/cortex/pkg/brain/context/health"
)

// AutoCompactor integrates compaction with health monitoring and event bus.
type AutoCompactor struct {
	compactor     *Compactor
	healthChecker *health.HealthChecker
	eventBus      *bus.EventBus
}

// NewAutoCompactor creates an integrated compactor.
func NewAutoCompactor(eventBus *bus.EventBus) *AutoCompactor {
	return &AutoCompactor{
		compactor:     NewCompactor(),
		healthChecker: health.NewHealthChecker(),
		eventBus:      eventBus,
	}
}

// NewAutoCompactorWithConfig creates an integrated compactor with custom config.
func NewAutoCompactorWithConfig(config CompactionConfig, eventBus *bus.EventBus) *AutoCompactor {
	return &AutoCompactor{
		compactor:     NewCompactorWithConfig(config),
		healthChecker: health.NewHealthChecker(),
		eventBus:      eventBus,
	}
}

// CompactIfNeeded checks if compaction is needed and performs it if so.
// Returns the result if compaction was performed, nil if not needed.
func (ac *AutoCompactor) CompactIfNeeded(bb *context.AttentionBlackboard) *PruneResult {
	if !ac.healthChecker.NeedsCompaction(bb) {
		return nil
	}

	// Fire compaction needed event
	if ac.eventBus != nil {
		priority := ac.healthChecker.CompactionPriority(bb)
		utilization := bb.Stats().Utilization
		ac.eventBus.Publish(bus.NewContextCompactionNeededEvent(priority, utilization, "utilization threshold exceeded"))
	}

	// Perform compaction
	result := ac.compactor.Prune(bb)

	// Fire compaction complete event
	if ac.eventBus != nil && len(result.PrunedItems) > 0 {
		ac.eventBus.Publish(bus.NewContextCompactionDoneEvent(
			result.TokensBefore,
			result.TokensAfter,
			len(result.PrunedItems),
			result.Duration,
		))
	}

	return result
}

// CompactWithPriority performs compaction based on health-determined priority.
// Higher priority = more aggressive compaction (lower target utilization).
func (ac *AutoCompactor) CompactWithPriority(bb *context.AttentionBlackboard) *PruneResult {
	priority := ac.healthChecker.CompactionPriority(bb)

	// Adjust target based on priority
	// Priority 0.0-0.5: target 0.7 (normal)
	// Priority 0.5-0.8: target 0.6 (moderate)
	// Priority 0.8-1.0: target 0.5 (aggressive)
	var targetUtilization float64
	switch {
	case priority >= 0.8:
		targetUtilization = 0.5
	case priority >= 0.5:
		targetUtilization = 0.6
	default:
		targetUtilization = 0.7
	}

	// Create temporary compactor with adjusted config
	config := ac.compactor.config
	config.TargetUtilization = targetUtilization
	tempCompactor := NewCompactorWithConfig(config)

	return tempCompactor.Prune(bb)
}

// SetupTriggers connects to health triggers for automatic compaction.
func (ac *AutoCompactor) SetupTriggers(triggers *health.TriggerManager) {
	triggers.OnTrigger(func(tt health.TriggerType, bb *context.AttentionBlackboard) {
		// Compact on high budget thresholds
		switch tt {
		case health.TriggerBudget90:
			// Urgent compaction at 90%
			ac.CompactIfNeeded(bb)
		case health.TriggerBudget75:
			// Consider compaction at 75%
			if ac.healthChecker.NeedsCompaction(bb) {
				ac.CompactIfNeeded(bb)
			}
		case health.TriggerPhaseComplete:
			// Good opportunity to compact between phases
			ac.CompactIfNeeded(bb)
		}
	})
}

// HealthCheck returns current health status.
func (ac *AutoCompactor) HealthCheck(bb *context.AttentionBlackboard) health.HealthReport {
	return ac.healthChecker.Check(bb)
}

// QuickHealthCheck returns quick status without full report.
func (ac *AutoCompactor) QuickHealthCheck(bb *context.AttentionBlackboard) (health.HealthStatus, int) {
	return ac.healthChecker.QuickCheck(bb)
}

// Compactor returns the underlying compactor for direct access.
func (ac *AutoCompactor) Compactor() *Compactor {
	return ac.compactor
}

// HealthChecker returns the underlying health checker for direct access.
func (ac *AutoCompactor) HealthChecker() *health.HealthChecker {
	return ac.healthChecker
}
