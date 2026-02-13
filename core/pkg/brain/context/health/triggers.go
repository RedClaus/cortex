package health

import (
	"sync"

	"github.com/normanking/cortex/pkg/brain/context"
)

// TriggerManager manages event-driven health check triggers.
// It tracks utilization thresholds and fires callbacks when conditions are met.
//
// Unlike fixed-interval polling, this is event-driven:
// - Checks fire when budget crosses 50%, 75%, 90% thresholds
// - Checks fire when phases complete
// - Checks fire after compaction
//
// This is more efficient than polling every 5 seconds.
type TriggerManager struct {
	mu sync.RWMutex

	config    TriggerConfig
	callbacks []TriggerCallback

	// Track which thresholds have fired to avoid duplicate triggers
	firedThresholds map[float64]bool

	// Last known utilization
	lastUtilization float64
}

// TriggerCallback is called when a trigger fires.
type TriggerCallback func(trigger TriggerType, bb *context.AttentionBlackboard)

// NewTriggerManager creates a new trigger manager with default configuration.
func NewTriggerManager() *TriggerManager {
	return NewTriggerManagerWithConfig(DefaultTriggerConfig())
}

// NewTriggerManagerWithConfig creates a trigger manager with custom configuration.
func NewTriggerManagerWithConfig(config TriggerConfig) *TriggerManager {
	return &TriggerManager{
		config:          config,
		callbacks:       make([]TriggerCallback, 0),
		firedThresholds: make(map[float64]bool),
	}
}

// OnTrigger registers a callback to be called when any trigger fires.
func (tm *TriggerManager) OnTrigger(callback TriggerCallback) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.callbacks = append(tm.callbacks, callback)
}

// CheckBudget checks if any budget thresholds have been crossed.
// Should be called after items are added to the blackboard.
func (tm *TriggerManager) CheckBudget(bb *context.AttentionBlackboard) {
	stats := bb.Stats()
	currentUtilization := stats.Utilization

	tm.mu.Lock()
	defer tm.mu.Unlock()

	for _, threshold := range tm.config.BudgetThresholds {
		// Check if we crossed this threshold upward
		if currentUtilization >= threshold && tm.lastUtilization < threshold {
			if !tm.firedThresholds[threshold] {
				tm.firedThresholds[threshold] = true
				triggerType := thresholdToTrigger(threshold)
				tm.fireCallbacksLocked(triggerType, bb)
			}
		}
		// Reset if we dropped below threshold
		if currentUtilization < threshold {
			tm.firedThresholds[threshold] = false
		}
	}

	tm.lastUtilization = currentUtilization
}

// thresholdToTrigger converts a threshold percentage to a trigger type.
func thresholdToTrigger(threshold float64) TriggerType {
	switch {
	case threshold >= 0.90:
		return TriggerBudget90
	case threshold >= 0.75:
		return TriggerBudget75
	case threshold >= 0.50:
		return TriggerBudget50
	default:
		return TriggerManual
	}
}

// OnPhaseComplete should be called when a processing phase completes.
func (tm *TriggerManager) OnPhaseComplete(bb *context.AttentionBlackboard) {
	if !tm.config.OnPhaseComplete {
		return
	}

	tm.mu.RLock()
	defer tm.mu.RUnlock()
	tm.fireCallbacksLocked(TriggerPhaseComplete, bb)
}

// OnCompactionComplete should be called after compaction runs.
func (tm *TriggerManager) OnCompactionComplete(bb *context.AttentionBlackboard) {
	if !tm.config.OnCompactionComplete {
		return
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Reset threshold tracking after compaction (utilization may have dropped)
	for threshold := range tm.firedThresholds {
		tm.firedThresholds[threshold] = false
	}
	tm.lastUtilization = bb.Stats().Utilization

	tm.fireCallbacksLocked(TriggerCompactionComplete, bb)
}

// Manual triggers a manual health check.
func (tm *TriggerManager) Manual(bb *context.AttentionBlackboard) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	tm.fireCallbacksLocked(TriggerManual, bb)
}

// fireCallbacksLocked calls all registered callbacks. Must be called with lock held.
func (tm *TriggerManager) fireCallbacksLocked(trigger TriggerType, bb *context.AttentionBlackboard) {
	for _, callback := range tm.callbacks {
		// Call in goroutine to avoid blocking
		go callback(trigger, bb)
	}
}

// Reset clears all trigger state.
func (tm *TriggerManager) Reset() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.firedThresholds = make(map[float64]bool)
	tm.lastUtilization = 0
}

// IsThresholdFired returns true if the given threshold has already fired.
func (tm *TriggerManager) IsThresholdFired(threshold float64) bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.firedThresholds[threshold]
}

// PendingTriggers returns which thresholds are close to firing.
// Useful for proactive monitoring.
func (tm *TriggerManager) PendingTriggers(utilization float64) []float64 {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var pending []float64
	for _, threshold := range tm.config.BudgetThresholds {
		if utilization < threshold && !tm.firedThresholds[threshold] {
			// This threshold hasn't fired and we haven't crossed it yet
			if utilization >= threshold-0.1 {
				// Within 10% of threshold
				pending = append(pending, threshold)
			}
		}
	}
	return pending
}
