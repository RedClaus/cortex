// Package compaction provides context size reduction for the attention-aware blackboard.
//
// Per YAGNI principles (Architect validation), only the Prune method is implemented.
// Future methods (Summarize, Archive, Migrate) can be added when proven necessary.
package compaction

import (
	"sort"
	"time"

	"github.com/normanking/cortex/pkg/brain/context"
)

// Compactor provides context compaction operations.
type Compactor struct {
	config CompactionConfig
}

// CompactionConfig defines compaction behavior.
type CompactionConfig struct {
	// TargetUtilization is the desired utilization after compaction (0.0-1.0).
	// Default: 0.7 (70%)
	TargetUtilization float64

	// MinPruneCount is the minimum number of items to prune per compaction.
	// Prevents micro-compactions. Default: 3
	MinPruneCount int

	// ProtectHighPriority prevents pruning items with priority >= this threshold.
	// Default: 0.9
	ProtectHighPriority float64

	// PreferSupportingZone prioritizes pruning from Supporting (middle) zone.
	// This aligns with "Lost in Middle" - middle content is least attended.
	// Default: true
	PreferSupportingZone bool
}

// DefaultCompactionConfig returns sensible defaults.
func DefaultCompactionConfig() CompactionConfig {
	return CompactionConfig{
		TargetUtilization:    0.7,
		MinPruneCount:        3,
		ProtectHighPriority:  0.9,
		PreferSupportingZone: true,
	}
}

// NewCompactor creates a new compactor with default config.
func NewCompactor() *Compactor {
	return &Compactor{config: DefaultCompactionConfig()}
}

// NewCompactorWithConfig creates a compactor with custom config.
func NewCompactorWithConfig(config CompactionConfig) *Compactor {
	return &Compactor{config: config}
}

// PruneResult contains the outcome of a prune operation.
type PruneResult struct {
	// Items that were pruned
	PrunedItems []*context.ContextItem

	// Token counts
	TokensBefore int
	TokensAfter  int
	TokensFreed  int

	// Utilization changes
	UtilizationBefore float64
	UtilizationAfter  float64

	// Timing
	Duration time.Duration

	// Zone breakdown
	PrunedFromCritical   int
	PrunedFromSupporting int
	PrunedFromActionable int
}

// Prune removes low-priority items until target utilization is reached.
//
// The algorithm:
// 1. If utilization is below target, return (no work needed)
// 2. Collect all pruneable items (priority < ProtectHighPriority)
// 3. Sort by priority (lowest first), preferring Supporting zone
// 4. Prune items until target utilization or MinPruneCount reached
// 5. Return result with pruned items and metrics
func (c *Compactor) Prune(bb *context.AttentionBlackboard) *PruneResult {
	start := time.Now()

	statsBefore := bb.Stats()
	result := &PruneResult{
		PrunedItems:       make([]*context.ContextItem, 0),
		TokensBefore:      statsBefore.TotalTokens,
		UtilizationBefore: statsBefore.Utilization,
	}

	// Check if compaction needed
	if statsBefore.Utilization <= c.config.TargetUtilization {
		result.TokensAfter = statsBefore.TotalTokens
		result.UtilizationAfter = statsBefore.Utilization
		result.Duration = time.Since(start)
		return result
	}

	// Calculate tokens to free
	targetTokens := int(float64(statsBefore.BudgetLimit) * c.config.TargetUtilization)
	tokensToFree := statsBefore.TotalTokens - targetTokens

	// Collect pruneable candidates
	candidates := c.collectCandidates(bb)

	// Prune until target reached or min count satisfied
	pruneCount := 0
	tokensFreed := 0

	for _, candidate := range candidates {
		// Stop if we've reached target AND min prune count
		if tokensFreed >= tokensToFree && pruneCount >= c.config.MinPruneCount {
			break
		}

		// Remove the item
		if bb.Remove(candidate.ID) {
			result.PrunedItems = append(result.PrunedItems, candidate)
			tokensFreed += candidate.TokenCount
			pruneCount++

			// Track zone
			switch candidate.Zone {
			case context.ZoneCritical:
				result.PrunedFromCritical++
			case context.ZoneSupporting:
				result.PrunedFromSupporting++
			case context.ZoneActionable:
				result.PrunedFromActionable++
			}
		}
	}

	// Final stats
	statsAfter := bb.Stats()
	result.TokensAfter = statsAfter.TotalTokens
	result.TokensFreed = tokensFreed
	result.UtilizationAfter = statsAfter.Utilization
	result.Duration = time.Since(start)

	return result
}

// PruneToTarget prunes until a specific token count is reached.
func (c *Compactor) PruneToTarget(bb *context.AttentionBlackboard, targetTokens int) *PruneResult {
	start := time.Now()

	statsBefore := bb.Stats()
	result := &PruneResult{
		PrunedItems:       make([]*context.ContextItem, 0),
		TokensBefore:      statsBefore.TotalTokens,
		UtilizationBefore: statsBefore.Utilization,
	}

	// Check if already at target
	if statsBefore.TotalTokens <= targetTokens {
		result.TokensAfter = statsBefore.TotalTokens
		result.UtilizationAfter = statsBefore.Utilization
		result.Duration = time.Since(start)
		return result
	}

	// Collect and prune
	candidates := c.collectCandidates(bb)
	tokensToFree := statsBefore.TotalTokens - targetTokens
	tokensFreed := 0

	for _, candidate := range candidates {
		if tokensFreed >= tokensToFree {
			break
		}

		if bb.Remove(candidate.ID) {
			result.PrunedItems = append(result.PrunedItems, candidate)
			tokensFreed += candidate.TokenCount

			switch candidate.Zone {
			case context.ZoneCritical:
				result.PrunedFromCritical++
			case context.ZoneSupporting:
				result.PrunedFromSupporting++
			case context.ZoneActionable:
				result.PrunedFromActionable++
			}
		}
	}

	statsAfter := bb.Stats()
	result.TokensAfter = statsAfter.TotalTokens
	result.TokensFreed = tokensFreed
	result.UtilizationAfter = statsAfter.Utilization
	result.Duration = time.Since(start)

	return result
}

// collectCandidates gathers items eligible for pruning, sorted by priority.
func (c *Compactor) collectCandidates(bb *context.AttentionBlackboard) []*context.ContextItem {
	candidates := make([]*context.ContextItem, 0)

	// Collect from all zones
	for _, zone := range context.ZoneOrder() {
		items := bb.GetZone(zone)
		for _, item := range items {
			// Skip high-priority items
			if item.Priority >= c.config.ProtectHighPriority {
				continue
			}
			candidates = append(candidates, item)
		}
	}

	// Sort candidates: lowest priority first, preferring Supporting zone
	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]

		// If preferring Supporting zone, sort Supporting items first
		if c.config.PreferSupportingZone {
			aSupporting := a.Zone == context.ZoneSupporting
			bSupporting := b.Zone == context.ZoneSupporting
			if aSupporting && !bSupporting {
				return true
			}
			if !aSupporting && bSupporting {
				return false
			}
		}

		// Then sort by priority (lowest first)
		return a.EffectivePriority() < b.EffectivePriority()
	})

	return candidates
}

// NeedsPrune returns true if the blackboard should be compacted.
func (c *Compactor) NeedsPrune(bb *context.AttentionBlackboard) bool {
	stats := bb.Stats()
	return stats.Utilization > c.config.TargetUtilization
}

// EstimatePrune returns estimated results without actually pruning.
func (c *Compactor) EstimatePrune(bb *context.AttentionBlackboard) *PruneResult {
	stats := bb.Stats()
	result := &PruneResult{
		TokensBefore:      stats.TotalTokens,
		UtilizationBefore: stats.Utilization,
	}

	if stats.Utilization <= c.config.TargetUtilization {
		result.TokensAfter = stats.TotalTokens
		result.UtilizationAfter = stats.Utilization
		return result
	}

	// Estimate what would be pruned
	targetTokens := int(float64(stats.BudgetLimit) * c.config.TargetUtilization)
	tokensToFree := stats.TotalTokens - targetTokens

	candidates := c.collectCandidates(bb)
	tokensFreed := 0
	pruneCount := 0

	for _, candidate := range candidates {
		if tokensFreed >= tokensToFree && pruneCount >= c.config.MinPruneCount {
			break
		}
		result.PrunedItems = append(result.PrunedItems, candidate)
		tokensFreed += candidate.TokenCount
		pruneCount++

		switch candidate.Zone {
		case context.ZoneCritical:
			result.PrunedFromCritical++
		case context.ZoneSupporting:
			result.PrunedFromSupporting++
		case context.ZoneActionable:
			result.PrunedFromActionable++
		}
	}

	result.TokensFreed = tokensFreed
	result.TokensAfter = stats.TotalTokens - tokensFreed
	result.UtilizationAfter = float64(result.TokensAfter) / float64(stats.BudgetLimit)

	return result
}
