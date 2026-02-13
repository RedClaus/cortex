package context

import (
	"testing"
	"time"
)

func TestNewAttentionBlackboard(t *testing.T) {
	bb := NewAttentionBlackboard()
	if bb == nil {
		t.Fatal("NewAttentionBlackboard returned nil")
	}

	stats := bb.Stats()
	if stats.TotalItems != 0 {
		t.Errorf("Expected 0 items, got %d", stats.TotalItems)
	}
	if stats.BudgetLimit != DefaultZoneConfig().Total() {
		t.Errorf("Expected budget %d, got %d", DefaultZoneConfig().Total(), stats.BudgetLimit)
	}
}

func TestAttentionBlackboard_Add(t *testing.T) {
	bb := NewAttentionBlackboard()

	item := NewContextItem(SourceSystem, CategorySystem, "test content", ZoneCritical)
	ok := bb.Add(item)

	if !ok {
		t.Error("Add returned false")
	}

	stats := bb.Stats()
	if stats.TotalItems != 1 {
		t.Errorf("Expected 1 item, got %d", stats.TotalItems)
	}
	if stats.CriticalItems != 1 {
		t.Errorf("Expected 1 critical item, got %d", stats.CriticalItems)
	}
}

func TestAttentionBlackboard_Get(t *testing.T) {
	bb := NewAttentionBlackboard()

	item := NewContextItem(SourceMemoryLobe, CategoryMemory, "memory content", ZoneSupporting)
	bb.Add(item)

	retrieved, ok := bb.Get(item.ID)
	if !ok {
		t.Error("Get returned false for existing item")
	}
	if retrieved.ContentString() != "memory content" {
		t.Errorf("Expected 'memory content', got '%s'", retrieved.ContentString())
	}

	_, ok = bb.Get("nonexistent")
	if ok {
		t.Error("Get returned true for nonexistent item")
	}
}

func TestAttentionBlackboard_GetByCategory(t *testing.T) {
	bb := NewAttentionBlackboard()

	bb.Add(NewContextItem(SourceSystem, CategoryCode, "code 1", ZoneSupporting))
	bb.Add(NewContextItem(SourceSystem, CategoryCode, "code 2", ZoneSupporting))
	bb.Add(NewContextItem(SourceSystem, CategoryMemory, "memory", ZoneSupporting))

	codeItems := bb.GetByCategory(CategoryCode)
	if len(codeItems) != 2 {
		t.Errorf("Expected 2 code items, got %d", len(codeItems))
	}

	memoryItems := bb.GetByCategory(CategoryMemory)
	if len(memoryItems) != 1 {
		t.Errorf("Expected 1 memory item, got %d", len(memoryItems))
	}
}

func TestAttentionBlackboard_GetBySource(t *testing.T) {
	bb := NewAttentionBlackboard()

	bb.Add(NewContextItem(SourceCodingLobe, CategoryCode, "code", ZoneActionable))
	bb.Add(NewContextItem(SourceCodingLobe, CategoryCode, "more code", ZoneActionable))
	bb.Add(NewContextItem(SourceEmotionLobe, CategoryEmotion, "emotion", ZoneSupporting))

	codingItems := bb.GetBySource(SourceCodingLobe)
	if len(codingItems) != 2 {
		t.Errorf("Expected 2 coding lobe items, got %d", len(codingItems))
	}
}

func TestAttentionBlackboard_GetZone(t *testing.T) {
	bb := NewAttentionBlackboard()

	bb.Add(NewContextItem(SourceSystem, CategorySystem, "critical", ZoneCritical).WithPriority(0.8))
	bb.Add(NewContextItem(SourceSystem, CategorySystem, "critical2", ZoneCritical).WithPriority(0.3))
	bb.Add(NewContextItem(SourceSystem, CategoryMemory, "supporting", ZoneSupporting))

	critical := bb.GetZone(ZoneCritical)
	if len(critical) != 2 {
		t.Errorf("Expected 2 critical items, got %d", len(critical))
	}

	// Should be sorted by priority (highest first)
	if critical[0].Priority < critical[1].Priority {
		t.Error("Critical items not sorted by priority")
	}
}

func TestAttentionBlackboard_GetAll(t *testing.T) {
	bb := NewAttentionBlackboard()

	bb.Add(NewContextItem(SourceSystem, CategorySystem, "critical", ZoneCritical))
	bb.Add(NewContextItem(SourceSystem, CategoryMemory, "supporting", ZoneSupporting))
	bb.Add(NewContextItem(SourceSystem, CategoryTask, "actionable", ZoneActionable))

	all := bb.GetAll()
	if len(all) != 3 {
		t.Errorf("Expected 3 items, got %d", len(all))
	}

	// Should be in zone order: Critical -> Supporting -> Actionable
	if all[0].Zone != ZoneCritical {
		t.Errorf("First item should be Critical, got %s", all[0].Zone)
	}
	if all[1].Zone != ZoneSupporting {
		t.Errorf("Second item should be Supporting, got %s", all[1].Zone)
	}
	if all[2].Zone != ZoneActionable {
		t.Errorf("Third item should be Actionable, got %s", all[2].Zone)
	}
}

func TestAttentionBlackboard_Remove(t *testing.T) {
	bb := NewAttentionBlackboard()

	item := NewContextItem(SourceSystem, CategorySystem, "to remove", ZoneCritical)
	bb.Add(item)

	ok := bb.Remove(item.ID)
	if !ok {
		t.Error("Remove returned false")
	}

	stats := bb.Stats()
	if stats.TotalItems != 0 {
		t.Errorf("Expected 0 items after remove, got %d", stats.TotalItems)
	}

	ok = bb.Remove("nonexistent")
	if ok {
		t.Error("Remove returned true for nonexistent item")
	}
}

func TestAttentionBlackboard_BudgetEnforcement(t *testing.T) {
	config := ZoneConfig{
		Critical:   100,
		Supporting: 100,
		Actionable: 100,
	}
	bb := NewAttentionBlackboardWithConfig(config)

	// Add items that fit
	item1 := NewContextItem(SourceSystem, CategorySystem, "short", ZoneCritical)
	item1.TokenCount = 50
	ok := bb.Add(item1)
	if !ok {
		t.Error("First item should fit")
	}

	// Add item that doesn't fit
	item2 := NewContextItem(SourceSystem, CategorySystem, "too long content that exceeds budget", ZoneCritical)
	item2.TokenCount = 100 // Would exceed 100 limit
	ok = bb.Add(item2)
	if ok {
		t.Error("Second item should not fit")
	}
}

func TestAttentionBlackboard_AddForce(t *testing.T) {
	config := ZoneConfig{
		Critical:   50,
		Supporting: 100,
		Actionable: 50,
	}
	bb := NewAttentionBlackboardWithConfig(config)

	// Fill critical zone
	item1 := NewContextItem(SourceSystem, CategorySystem, "first", ZoneCritical).WithPriority(0.3)
	item1.TokenCount = 30
	bb.Add(item1)

	// Force add should prune the lower priority item
	item2 := NewContextItem(SourceSystem, CategorySystem, "second", ZoneCritical).WithPriority(0.8)
	item2.TokenCount = 40
	bb.AddForce(item2)

	stats := bb.Stats()
	if stats.CriticalItems != 1 {
		t.Errorf("Expected 1 critical item after force add, got %d", stats.CriticalItems)
	}

	// The remaining item should be the higher priority one
	critical := bb.GetZone(ZoneCritical)
	if len(critical) != 1 || critical[0].Priority != 0.8 {
		t.Error("Wrong item kept after force add")
	}
}

func TestAttentionBlackboard_Clone(t *testing.T) {
	bb := NewAttentionBlackboard()

	bb.Add(NewContextItem(SourceSystem, CategorySystem, "item1", ZoneCritical))
	bb.Add(NewContextItem(SourceSystem, CategoryMemory, "item2", ZoneSupporting))

	clone := bb.Clone()

	// Modify original
	bb.Add(NewContextItem(SourceSystem, CategoryTask, "item3", ZoneActionable))

	// Clone should not have the new item
	cloneStats := clone.Stats()
	bbStats := bb.Stats()

	if cloneStats.TotalItems != 2 {
		t.Errorf("Clone should have 2 items, got %d", cloneStats.TotalItems)
	}
	if bbStats.TotalItems != 3 {
		t.Errorf("Original should have 3 items, got %d", bbStats.TotalItems)
	}
}

func TestAttentionBlackboard_Clear(t *testing.T) {
	bb := NewAttentionBlackboard()

	bb.Add(NewContextItem(SourceSystem, CategorySystem, "item1", ZoneCritical))
	bb.Add(NewContextItem(SourceSystem, CategoryMemory, "item2", ZoneSupporting))
	bb.Add(NewContextItem(SourceSystem, CategoryTask, "item3", ZoneActionable))

	bb.Clear()

	stats := bb.Stats()
	if stats.TotalItems != 0 {
		t.Errorf("Expected 0 items after clear, got %d", stats.TotalItems)
	}
	if stats.TotalTokens != 0 {
		t.Errorf("Expected 0 tokens after clear, got %d", stats.TotalTokens)
	}
}

func TestAttentionBlackboard_ZoneUtilization(t *testing.T) {
	config := ZoneConfig{
		Critical:   100,
		Supporting: 100,
		Actionable: 100,
	}
	bb := NewAttentionBlackboardWithConfig(config)

	item := NewContextItem(SourceSystem, CategorySystem, "half", ZoneCritical)
	item.TokenCount = 50
	bb.Add(item)

	util := bb.ZoneUtilization(ZoneCritical)
	if util != 0.5 {
		t.Errorf("Expected 0.5 utilization, got %f", util)
	}
}

func TestContextItem_EffectivePriority(t *testing.T) {
	item := NewContextItem(SourceSystem, CategorySystem, "test", ZoneCritical)
	item.Priority = 0.8

	// Critical zone has priority 1.0
	// Effective = (0.8 + 1.0) / 2 = 0.9
	effective := item.EffectivePriority()
	if effective != 0.9 {
		t.Errorf("Expected 0.9 effective priority, got %f", effective)
	}
}

func TestContextItem_Age(t *testing.T) {
	item := NewContextItem(SourceSystem, CategorySystem, "test", ZoneCritical)

	// Wait a tiny bit
	time.Sleep(10 * time.Millisecond)

	age := item.Age()
	if age < 10*time.Millisecond {
		t.Errorf("Age should be at least 10ms, got %v", age)
	}
}

func TestTokenEstimator(t *testing.T) {
	estimator := DefaultTokenEstimator()

	// Test basic estimation
	tokens := estimator.Estimate("Hello world")
	if tokens < 2 || tokens > 5 {
		t.Errorf("Unexpected token count for 'Hello world': %d", tokens)
	}

	// Empty string should return 0
	tokens = estimator.Estimate("")
	if tokens != 0 {
		t.Errorf("Empty string should have 0 tokens, got %d", tokens)
	}

	// Nil should return 0
	tokens = estimator.Estimate(nil)
	if tokens != 0 {
		t.Errorf("Nil should have 0 tokens, got %d", tokens)
	}
}

func TestTokenBudget(t *testing.T) {
	budget := NewTokenBudget(100)

	if budget.Available() != 100 {
		t.Errorf("Expected 100 available, got %d", budget.Available())
	}

	if !budget.CanFit(50) {
		t.Error("Should be able to fit 50")
	}

	ok := budget.Use(50)
	if !ok {
		t.Error("Use should succeed")
	}

	if budget.Available() != 50 {
		t.Errorf("Expected 50 available, got %d", budget.Available())
	}

	if budget.CanFit(60) {
		t.Error("Should not fit 60")
	}

	budget.Release(30)
	if budget.Available() != 80 {
		t.Errorf("Expected 80 available, got %d", budget.Available())
	}

	if budget.Utilization() != 0.2 {
		t.Errorf("Expected 0.2 utilization, got %f", budget.Utilization())
	}
}

func TestZoneConfig(t *testing.T) {
	config := DefaultZoneConfig()

	total := config.Total()
	expected := config.Critical + config.Supporting + config.Actionable
	if total != expected {
		t.Errorf("Total() should be %d, got %d", expected, total)
	}
}

func TestZoneOrder(t *testing.T) {
	order := ZoneOrder()

	if len(order) != 3 {
		t.Errorf("Expected 3 zones, got %d", len(order))
	}
	if order[0] != ZoneCritical {
		t.Error("First zone should be Critical")
	}
	if order[1] != ZoneSupporting {
		t.Error("Second zone should be Supporting")
	}
	if order[2] != ZoneActionable {
		t.Error("Third zone should be Actionable")
	}
}

func TestZonePriority(t *testing.T) {
	if ZonePriority(ZoneCritical) != 1.0 {
		t.Error("Critical zone should have priority 1.0")
	}
	if ZonePriority(ZoneActionable) != 0.9 {
		t.Error("Actionable zone should have priority 0.9")
	}
	if ZonePriority(ZoneSupporting) != 0.6 {
		t.Error("Supporting zone should have priority 0.6")
	}
}
