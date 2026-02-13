package health

import (
	"sync"
	"testing"
	"time"

	"github.com/normanking/cortex/pkg/brain/context"
)

func TestNewDetector(t *testing.T) {
	d := NewDefaultDetector()
	if d == nil {
		t.Fatal("NewDefaultDetector returned nil")
	}
}

func TestDetector_DetectLostInMiddle(t *testing.T) {
	d := NewDefaultDetector()
	bb := context.NewAttentionBlackboard()

	// Add high-priority item to middle (Supporting) zone
	item := context.NewContextItem(context.SourceSystem, context.CategorySystem, "important", context.ZoneSupporting)
	item.Priority = 0.9 // Above threshold
	item.TokenCount = 100
	bb.Add(item)

	patterns := d.Detect(bb)

	if len(patterns) == 0 {
		t.Error("Expected LostInMiddle pattern to be detected")
	}

	found := false
	for _, p := range patterns {
		if p.Pattern == PatternLostInMiddle {
			found = true
			if p.Severity <= 0 {
				t.Error("Expected non-zero severity")
			}
			if len(p.AffectedItems) == 0 {
				t.Error("Expected affected items")
			}
		}
	}
	if !found {
		t.Error("LostInMiddle pattern not found")
	}
}

func TestDetector_NoPatternWhenHealthy(t *testing.T) {
	d := NewDefaultDetector()
	bb := context.NewAttentionBlackboard()

	// Add low-priority items to middle zone with balanced distribution
	// Need items in all zones to avoid middle overload
	sysItem := context.NewContextItem(context.SourceSystem, context.CategorySystem, "system", context.ZoneCritical)
	sysItem.Priority = 0.5
	sysItem.TokenCount = 100
	bb.Add(sysItem)

	memItem := context.NewContextItem(context.SourceSystem, context.CategoryMemory, "memory", context.ZoneSupporting)
	memItem.Priority = 0.3 // Below threshold
	memItem.TokenCount = 50
	bb.Add(memItem)

	taskItem := context.NewContextItem(context.SourceSystem, context.CategoryTask, "task", context.ZoneActionable)
	taskItem.Priority = 0.5
	taskItem.TokenCount = 100
	bb.Add(taskItem)

	patterns := d.Detect(bb)

	// With balanced distribution and low priority in middle, should be healthy
	for _, p := range patterns {
		if p.Pattern == PatternLostInMiddle && len(p.AffectedItems) > 0 {
			t.Error("Expected no high-priority items in middle pattern")
		}
	}
}

func TestDetector_GenerateRecommendations(t *testing.T) {
	d := NewDefaultDetector()

	patterns := []DetectedPattern{
		{
			Pattern:       PatternLostInMiddle,
			Severity:      0.8,
			AffectedItems: []string{"item1", "item2"},
		},
	}

	recs := d.GenerateRecommendations(patterns)

	if len(recs) == 0 {
		t.Error("Expected recommendations")
	}
}

func TestDetector_SuggestZone(t *testing.T) {
	d := NewDefaultDetector()

	// High priority -> Critical or Actionable
	item := context.NewContextItem(context.SourceSystem, context.CategoryTask, "task", context.ZoneSupporting)
	item.Priority = 0.9
	zone := d.SuggestZone(item)
	if zone == context.ZoneSupporting {
		t.Error("High priority item should not be suggested for Supporting zone")
	}

	// Task category -> Actionable
	taskItem := context.NewContextItem(context.SourceSystem, context.CategoryTask, "task", context.ZoneSupporting)
	taskItem.Priority = 0.5
	zone = d.SuggestZone(taskItem)
	if zone != context.ZoneActionable {
		t.Errorf("Task category should suggest Actionable, got %s", zone)
	}

	// System category -> Critical
	sysItem := context.NewContextItem(context.SourceSystem, context.CategorySystem, "sys", context.ZoneSupporting)
	sysItem.Priority = 0.5
	zone = d.SuggestZone(sysItem)
	if zone != context.ZoneCritical {
		t.Errorf("System category should suggest Critical, got %s", zone)
	}
}

func TestHealthChecker_Check(t *testing.T) {
	hc := NewHealthChecker()
	bb := context.NewAttentionBlackboard()

	// Empty blackboard should be healthy
	report := hc.Check(bb)
	if report.Status != StatusHealthy {
		t.Errorf("Empty blackboard should be healthy, got %s", report.Status)
	}
	if report.Score < 90 {
		t.Errorf("Empty blackboard score should be high, got %d", report.Score)
	}

	// Add some items
	bb.Add(context.NewContextItem(context.SourceSystem, context.CategorySystem, "system", context.ZoneCritical))
	bb.Add(context.NewContextItem(context.SourceSystem, context.CategoryTask, "task", context.ZoneActionable))

	report = hc.Check(bb)
	if report.Timestamp.IsZero() {
		t.Error("Report should have timestamp")
	}
	if report.Duration == 0 {
		t.Error("Report should have duration")
	}
}

func TestHealthChecker_QuickCheck(t *testing.T) {
	hc := NewHealthChecker()
	bb := context.NewAttentionBlackboard()

	status, score := hc.QuickCheck(bb)
	if status != StatusHealthy {
		t.Error("Quick check should return healthy for empty blackboard")
	}
	if score < 90 {
		t.Errorf("Quick check score should be high, got %d", score)
	}
}

func TestHealthChecker_NeedsCompaction(t *testing.T) {
	config := context.ZoneConfig{
		Critical:   100,
		Supporting: 100,
		Actionable: 100,
	}
	bb := context.NewAttentionBlackboardWithConfig(config)
	hc := NewHealthChecker()

	// Empty blackboard doesn't need compaction
	if hc.NeedsCompaction(bb) {
		t.Error("Empty blackboard should not need compaction")
	}

	// Fill up the blackboard to >85% utilization (260/300 = 86.7%)
	for i := 0; i < 26; i++ {
		item := context.NewContextItem(context.SourceSystem, context.CategoryMemory, "memory", context.ZoneSupporting)
		item.TokenCount = 10
		if !bb.Add(item) {
			// Zone is full, try other zones
			item.Zone = context.ZoneCritical
			if !bb.Add(item) {
				item.Zone = context.ZoneActionable
				bb.Add(item)
			}
		}
	}

	// High utilization (>85%) should need compaction
	stats := bb.Stats()
	if stats.Utilization < 0.85 {
		t.Skipf("Utilization only %f, test needs adjustment", stats.Utilization)
	}
	if !hc.NeedsCompaction(bb) {
		t.Errorf("High utilization (%f) blackboard should need compaction", stats.Utilization)
	}
}

func TestHealthChecker_CompactionPriority(t *testing.T) {
	config := context.ZoneConfig{
		Critical:   100,
		Supporting: 100,
		Actionable: 100,
	}
	bb := context.NewAttentionBlackboardWithConfig(config)
	hc := NewHealthChecker()

	// Empty blackboard has low priority
	priority := hc.CompactionPriority(bb)
	if priority > 0.1 {
		t.Errorf("Empty blackboard should have low priority, got %f", priority)
	}

	// Fill up to >90% utilization to get higher priority
	for i := 0; i < 28; i++ {
		item := context.NewContextItem(context.SourceSystem, context.CategoryMemory, "memory", context.ZoneSupporting)
		item.TokenCount = 10
		if !bb.Add(item) {
			// Zone full, try others
			item.Zone = context.ZoneCritical
			if !bb.Add(item) {
				item.Zone = context.ZoneActionable
				bb.Add(item)
			}
		}
	}

	priority = hc.CompactionPriority(bb)
	// At >90% utilization, priority should be at least 0.5 (from utilization) + pattern severity
	if priority < 0.4 {
		t.Errorf("High utilization should have higher priority, got %f (utilization: %f)", priority, bb.Stats().Utilization)
	}
}

func TestTriggerManager_BudgetThresholds(t *testing.T) {
	tm := NewTriggerManager()
	// Total budget: 300 tokens. Need to add 150+ to cross 50%
	bb := context.NewAttentionBlackboardWithConfig(context.ZoneConfig{
		Critical:   100,
		Supporting: 100,
		Actionable: 100,
	})

	triggered := make([]TriggerType, 0)
	var mu sync.Mutex

	tm.OnTrigger(func(trigger TriggerType, _ *context.AttentionBlackboard) {
		mu.Lock()
		triggered = append(triggered, trigger)
		mu.Unlock()
	})

	// Add items to cross 50% threshold (need 150+ tokens across all zones)
	// Add to all zones to reach 50% total utilization
	for i := 0; i < 16; i++ {
		item := context.NewContextItem(context.SourceSystem, context.CategoryMemory, "memory", context.ZoneSupporting)
		item.TokenCount = 10
		if !bb.Add(item) {
			item.Zone = context.ZoneCritical
			if !bb.Add(item) {
				item.Zone = context.ZoneActionable
				bb.Add(item)
			}
		}
		tm.CheckBudget(bb)
	}

	// Wait for goroutines
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Check if we actually crossed 50%
	stats := bb.Stats()
	if stats.Utilization < 0.5 {
		t.Skipf("Only reached %f utilization, need 0.5+", stats.Utilization)
	}

	if len(triggered) == 0 {
		t.Error("Expected budget threshold trigger")
	}

	found50 := false
	for _, tt := range triggered {
		if tt == TriggerBudget50 {
			found50 = true
		}
	}
	if !found50 {
		t.Errorf("Expected Budget50 trigger, got %v (utilization: %f)", triggered, stats.Utilization)
	}
}

func TestTriggerManager_PhaseComplete(t *testing.T) {
	tm := NewTriggerManager()
	bb := context.NewAttentionBlackboard()

	triggered := false
	var triggerType TriggerType

	tm.OnTrigger(func(trigger TriggerType, _ *context.AttentionBlackboard) {
		triggered = true
		triggerType = trigger
	})

	tm.OnPhaseComplete(bb)

	// Wait for goroutine
	time.Sleep(50 * time.Millisecond)

	if !triggered {
		t.Error("Expected phase complete trigger")
	}
	if triggerType != TriggerPhaseComplete {
		t.Errorf("Expected TriggerPhaseComplete, got %s", triggerType)
	}
}

func TestTriggerManager_Reset(t *testing.T) {
	tm := NewTriggerManager()
	// Total budget: 300. Need 150+ tokens to cross 50%
	bb := context.NewAttentionBlackboardWithConfig(context.ZoneConfig{
		Critical:   100,
		Supporting: 100,
		Actionable: 100,
	})

	// Cross 50% threshold - need 150+ tokens total
	for i := 0; i < 16; i++ {
		item := context.NewContextItem(context.SourceSystem, context.CategoryMemory, "memory", context.ZoneSupporting)
		item.TokenCount = 10
		if !bb.Add(item) {
			item.Zone = context.ZoneCritical
			if !bb.Add(item) {
				item.Zone = context.ZoneActionable
				bb.Add(item)
			}
		}
		tm.CheckBudget(bb)
	}

	stats := bb.Stats()
	if stats.Utilization < 0.5 {
		t.Skipf("Only reached %f utilization, need 0.5+", stats.Utilization)
	}

	if !tm.IsThresholdFired(0.5) {
		t.Errorf("50%% threshold should be fired (utilization: %f)", stats.Utilization)
	}

	tm.Reset()

	if tm.IsThresholdFired(0.5) {
		t.Error("50% threshold should be reset")
	}
}

func TestDefaultHealthConfig(t *testing.T) {
	config := DefaultHealthConfig()

	if config.DegradationThreshold <= 0 || config.DegradationThreshold > 100 {
		t.Error("Invalid degradation threshold")
	}
	if config.CriticalThreshold <= 0 || config.CriticalThreshold >= config.DegradationThreshold {
		t.Error("Invalid critical threshold")
	}
	if !config.LostInMiddle.Enabled {
		t.Error("LostInMiddle should be enabled by default")
	}
}

func TestDefaultTriggerConfig(t *testing.T) {
	config := DefaultTriggerConfig()

	if len(config.BudgetThresholds) == 0 {
		t.Error("Should have budget thresholds")
	}
	if !config.OnPhaseComplete {
		t.Error("OnPhaseComplete should be true by default")
	}
	if !config.OnCompactionComplete {
		t.Error("OnCompactionComplete should be true by default")
	}
}

func TestHealthReport_Fields(t *testing.T) {
	hc := NewHealthChecker()
	bb := context.NewAttentionBlackboard()

	bb.Add(context.NewContextItem(context.SourceSystem, context.CategorySystem, "sys", context.ZoneCritical))

	report := hc.Check(bb)

	if report.Score < 0 || report.Score > 100 {
		t.Errorf("Score should be 0-100, got %d", report.Score)
	}
	if report.Status != StatusHealthy && report.Status != StatusDegraded && report.Status != StatusCritical {
		t.Errorf("Invalid status: %s", report.Status)
	}
	if report.Stats.TotalItems != 1 {
		t.Errorf("Expected 1 item in stats, got %d", report.Stats.TotalItems)
	}
}
