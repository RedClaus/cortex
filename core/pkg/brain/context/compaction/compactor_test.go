package compaction

import (
	"testing"

	"github.com/normanking/cortex/pkg/brain/context"
)

func TestNewCompactor(t *testing.T) {
	c := NewCompactor()
	if c == nil {
		t.Fatal("NewCompactor returned nil")
	}
	if c.config.TargetUtilization != 0.7 {
		t.Errorf("Expected target 0.7, got %f", c.config.TargetUtilization)
	}
}

func TestDefaultCompactionConfig(t *testing.T) {
	config := DefaultCompactionConfig()

	if config.TargetUtilization != 0.7 {
		t.Errorf("Expected target 0.7, got %f", config.TargetUtilization)
	}
	if config.MinPruneCount != 3 {
		t.Errorf("Expected min prune 3, got %d", config.MinPruneCount)
	}
	if config.ProtectHighPriority != 0.9 {
		t.Errorf("Expected protect threshold 0.9, got %f", config.ProtectHighPriority)
	}
	if !config.PreferSupportingZone {
		t.Error("Expected PreferSupportingZone to be true")
	}
}

func TestPrune_NoWorkNeeded(t *testing.T) {
	// Small budget to make testing easier
	bb := context.NewAttentionBlackboardWithConfig(context.ZoneConfig{
		Critical:   100,
		Supporting: 200,
		Actionable: 100,
	})
	c := NewCompactor()

	// Add minimal items (well under 70% target)
	item := context.NewContextItem(context.SourceSystem, context.CategorySystem, "test", context.ZoneCritical)
	item.TokenCount = 10
	bb.Add(item)

	result := c.Prune(bb)

	if len(result.PrunedItems) != 0 {
		t.Errorf("Expected no pruning, got %d items pruned", len(result.PrunedItems))
	}
	if result.TokensBefore != result.TokensAfter {
		t.Error("Tokens should not change when no pruning needed")
	}
}

func TestPrune_BasicPruning(t *testing.T) {
	// Total: 300 tokens. Target 70% = 210. Need to be above that.
	bb := context.NewAttentionBlackboardWithConfig(context.ZoneConfig{
		Critical:   100,
		Supporting: 100,
		Actionable: 100,
	})
	c := NewCompactor()

	// Add items to exceed 70% utilization (need >210 tokens)
	// Add 10 items at 25 tokens each = 250 tokens (83%)
	for i := 0; i < 10; i++ {
		item := context.NewContextItem(context.SourceMemoryLobe, context.CategoryMemory, "memory", context.ZoneSupporting)
		item.TokenCount = 25
		item.Priority = 0.3 // Low priority, eligible for pruning
		if !bb.Add(item) {
			// Zone full, try other zones
			item.Zone = context.ZoneCritical
			if !bb.Add(item) {
				item.Zone = context.ZoneActionable
				bb.Add(item)
			}
		}
	}

	statsBefore := bb.Stats()
	if statsBefore.Utilization < 0.7 {
		t.Skipf("Utilization %f is below target, can't test pruning", statsBefore.Utilization)
	}

	result := c.Prune(bb)

	if len(result.PrunedItems) == 0 {
		t.Error("Expected items to be pruned")
	}
	if result.TokensAfter >= result.TokensBefore {
		t.Error("Expected tokens to decrease after pruning")
	}
	if result.UtilizationAfter > c.config.TargetUtilization+0.05 {
		t.Errorf("Expected utilization near target, got %f", result.UtilizationAfter)
	}
}

func TestPrune_ProtectsHighPriority(t *testing.T) {
	bb := context.NewAttentionBlackboardWithConfig(context.ZoneConfig{
		Critical:   100,
		Supporting: 200,
		Actionable: 100,
	})
	c := NewCompactor()

	// Add high-priority items
	for i := 0; i < 8; i++ {
		item := context.NewContextItem(context.SourceSystem, context.CategorySystem, "important", context.ZoneSupporting)
		item.TokenCount = 30
		item.Priority = 0.95 // Above protection threshold
		bb.Add(item)
	}

	result := c.Prune(bb)

	// High priority items should NOT be pruned
	for _, pruned := range result.PrunedItems {
		if pruned.Priority >= 0.9 {
			t.Errorf("High priority item (%.2f) should not be pruned", pruned.Priority)
		}
	}
}

func TestPrune_PrefersSupportingZone(t *testing.T) {
	bb := context.NewAttentionBlackboardWithConfig(context.ZoneConfig{
		Critical:   100,
		Supporting: 200,
		Actionable: 100,
	})
	config := DefaultCompactionConfig()
	config.PreferSupportingZone = true
	config.MinPruneCount = 1 // Allow single prunes for this test
	c := NewCompactorWithConfig(config)

	// Add low-priority items to all zones
	criticalItem := context.NewContextItem(context.SourceSystem, context.CategorySystem, "critical", context.ZoneCritical)
	criticalItem.TokenCount = 40
	criticalItem.Priority = 0.3
	bb.Add(criticalItem)

	supportingItem := context.NewContextItem(context.SourceMemoryLobe, context.CategoryMemory, "supporting", context.ZoneSupporting)
	supportingItem.TokenCount = 150
	supportingItem.Priority = 0.3
	bb.Add(supportingItem)

	actionableItem := context.NewContextItem(context.SourcePlanningLobe, context.CategoryTask, "actionable", context.ZoneActionable)
	actionableItem.TokenCount = 40
	actionableItem.Priority = 0.3
	bb.Add(actionableItem)

	// Total: 230/400 = 57.5% - add more to exceed target
	for i := 0; i < 3; i++ {
		item := context.NewContextItem(context.SourceMemoryLobe, context.CategoryMemory, "extra", context.ZoneSupporting)
		item.TokenCount = 20
		item.Priority = 0.2
		bb.Add(item)
	}

	result := c.Prune(bb)

	// Supporting zone should be pruned first
	if len(result.PrunedItems) > 0 && result.PrunedFromSupporting == 0 {
		t.Error("Expected Supporting zone to be pruned first")
	}
}

func TestPruneToTarget(t *testing.T) {
	bb := context.NewAttentionBlackboardWithConfig(context.ZoneConfig{
		Critical:   100,
		Supporting: 200,
		Actionable: 100,
	})
	c := NewCompactor()

	// Add items totaling 250 tokens
	for i := 0; i < 10; i++ {
		item := context.NewContextItem(context.SourceMemoryLobe, context.CategoryMemory, "mem", context.ZoneSupporting)
		item.TokenCount = 20
		item.Priority = 0.3
		if !bb.Add(item) {
			item.Zone = context.ZoneCritical
			bb.Add(item)
		}
	}

	// Prune to target of 150 tokens
	result := c.PruneToTarget(bb, 150)

	if result.TokensAfter > 150 {
		t.Errorf("Expected tokens <= 150, got %d", result.TokensAfter)
	}
	if len(result.PrunedItems) == 0 {
		t.Error("Expected items to be pruned")
	}
}

func TestPruneToTarget_AlreadyAtTarget(t *testing.T) {
	bb := context.NewAttentionBlackboardWithConfig(context.ZoneConfig{
		Critical:   100,
		Supporting: 100,
		Actionable: 100,
	})
	c := NewCompactor()

	item := context.NewContextItem(context.SourceSystem, context.CategorySystem, "test", context.ZoneCritical)
	item.TokenCount = 50
	bb.Add(item)

	// Target higher than current
	result := c.PruneToTarget(bb, 200)

	if len(result.PrunedItems) != 0 {
		t.Error("Expected no pruning when already below target")
	}
}

func TestNeedsPrune(t *testing.T) {
	bb := context.NewAttentionBlackboardWithConfig(context.ZoneConfig{
		Critical:   100,
		Supporting: 100,
		Actionable: 100,
	})
	c := NewCompactor() // Target 70%

	// Empty blackboard
	if c.NeedsPrune(bb) {
		t.Error("Empty blackboard should not need pruning")
	}

	// Add items to exceed 70% (need >210 tokens)
	for i := 0; i < 9; i++ {
		item := context.NewContextItem(context.SourceMemoryLobe, context.CategoryMemory, "mem", context.ZoneSupporting)
		item.TokenCount = 10
		if !bb.Add(item) {
			item.Zone = context.ZoneCritical
			if !bb.Add(item) {
				item.Zone = context.ZoneActionable
				bb.Add(item)
			}
		}
	}

	stats := bb.Stats()
	if stats.Utilization > 0.7 && !c.NeedsPrune(bb) {
		t.Errorf("Should need pruning at %.2f utilization", stats.Utilization)
	}
}

func TestEstimatePrune(t *testing.T) {
	bb := context.NewAttentionBlackboardWithConfig(context.ZoneConfig{
		Critical:   100,
		Supporting: 100,
		Actionable: 100,
	})
	c := NewCompactor()

	// Add items to exceed target
	for i := 0; i < 10; i++ {
		item := context.NewContextItem(context.SourceMemoryLobe, context.CategoryMemory, "mem", context.ZoneSupporting)
		item.TokenCount = 25
		item.Priority = 0.3
		if !bb.Add(item) {
			item.Zone = context.ZoneCritical
			if !bb.Add(item) {
				item.Zone = context.ZoneActionable
				bb.Add(item)
			}
		}
	}

	statsBefore := bb.Stats()
	if statsBefore.Utilization <= 0.7 {
		t.Skip("Need higher utilization to test estimation")
	}

	estimate := c.EstimatePrune(bb)

	// Verify estimate doesn't actually change blackboard
	statsAfter := bb.Stats()
	if statsAfter.TotalTokens != statsBefore.TotalTokens {
		t.Error("Estimate should not modify blackboard")
	}

	// Verify estimate predicts reduction
	if estimate.TokensAfter >= estimate.TokensBefore {
		t.Error("Estimate should predict token reduction")
	}
}

func TestPrune_MinPruneCount(t *testing.T) {
	bb := context.NewAttentionBlackboardWithConfig(context.ZoneConfig{
		Critical:   100,
		Supporting: 200,
		Actionable: 100,
	})

	// Config with high min prune count
	config := CompactionConfig{
		TargetUtilization:    0.7,
		MinPruneCount:        5,
		ProtectHighPriority:  0.9,
		PreferSupportingZone: true,
	}
	c := NewCompactorWithConfig(config)

	// Add items just above target
	for i := 0; i < 12; i++ {
		item := context.NewContextItem(context.SourceMemoryLobe, context.CategoryMemory, "mem", context.ZoneSupporting)
		item.TokenCount = 15
		item.Priority = 0.3
		if !bb.Add(item) {
			item.Zone = context.ZoneCritical
			if !bb.Add(item) {
				item.Zone = context.ZoneActionable
				bb.Add(item)
			}
		}
	}

	stats := bb.Stats()
	if stats.Utilization <= 0.7 {
		t.Skip("Need higher utilization for this test")
	}

	result := c.Prune(bb)

	// Should prune at least MinPruneCount items
	if len(result.PrunedItems) < config.MinPruneCount {
		t.Errorf("Expected at least %d pruned items, got %d", config.MinPruneCount, len(result.PrunedItems))
	}
}

func TestPrune_Duration(t *testing.T) {
	bb := context.NewAttentionBlackboardWithConfig(context.ZoneConfig{
		Critical:   100,
		Supporting: 100,
		Actionable: 100,
	})
	c := NewCompactor()

	// Add some items
	for i := 0; i < 10; i++ {
		item := context.NewContextItem(context.SourceMemoryLobe, context.CategoryMemory, "mem", context.ZoneSupporting)
		item.TokenCount = 25
		item.Priority = 0.3
		if !bb.Add(item) {
			item.Zone = context.ZoneCritical
			bb.Add(item)
		}
	}

	result := c.Prune(bb)

	if result.Duration == 0 {
		t.Error("Duration should be set")
	}
}

func TestPrune_ZoneBreakdown(t *testing.T) {
	bb := context.NewAttentionBlackboardWithConfig(context.ZoneConfig{
		Critical:   100,
		Supporting: 100,
		Actionable: 100,
	})
	config := DefaultCompactionConfig()
	config.PreferSupportingZone = false // Don't prefer any zone
	c := NewCompactorWithConfig(config)

	// Add low-priority items to all zones
	for i := 0; i < 4; i++ {
		cItem := context.NewContextItem(context.SourceSystem, context.CategorySystem, "crit", context.ZoneCritical)
		cItem.TokenCount = 20
		cItem.Priority = 0.1
		bb.Add(cItem)
	}

	for i := 0; i < 4; i++ {
		sItem := context.NewContextItem(context.SourceMemoryLobe, context.CategoryMemory, "supp", context.ZoneSupporting)
		sItem.TokenCount = 20
		sItem.Priority = 0.2
		bb.Add(sItem)
	}

	for i := 0; i < 4; i++ {
		aItem := context.NewContextItem(context.SourcePlanningLobe, context.CategoryTask, "act", context.ZoneActionable)
		aItem.TokenCount = 20
		aItem.Priority = 0.3
		bb.Add(aItem)
	}

	result := c.Prune(bb)

	// With PreferSupportingZone=false, lowest priority items should be pruned first
	// Critical zone has priority 0.1, so should be pruned first
	totalFromZones := result.PrunedFromCritical + result.PrunedFromSupporting + result.PrunedFromActionable
	if totalFromZones != len(result.PrunedItems) {
		t.Errorf("Zone counts (%d) don't match pruned items (%d)",
			totalFromZones, len(result.PrunedItems))
	}
}
