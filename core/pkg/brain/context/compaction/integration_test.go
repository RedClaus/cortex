package compaction

import (
	"sync"
	"testing"
	"time"

	"github.com/normanking/cortex/internal/bus"
	"github.com/normanking/cortex/pkg/brain/context"
	"github.com/normanking/cortex/pkg/brain/context/health"
)

func TestNewAutoCompactor(t *testing.T) {
	eb := bus.New()
	defer eb.Close()
	ac := NewAutoCompactor(eb)

	if ac == nil {
		t.Fatal("NewAutoCompactor returned nil")
	}
	if ac.compactor == nil {
		t.Error("Compactor should be initialized")
	}
	if ac.healthChecker == nil {
		t.Error("HealthChecker should be initialized")
	}
}

func TestAutoCompactor_CompactIfNeeded_NotNeeded(t *testing.T) {
	eb := bus.New()
	defer eb.Close()
	ac := NewAutoCompactor(eb)

	bb := context.NewAttentionBlackboardWithConfig(context.ZoneConfig{
		Critical:   100,
		Supporting: 100,
		Actionable: 100,
	})

	// Empty blackboard doesn't need compaction
	result := ac.CompactIfNeeded(bb)
	if result != nil {
		t.Error("Should return nil when compaction not needed")
	}
}

func TestAutoCompactor_CompactIfNeeded_Needed(t *testing.T) {
	eb := bus.New()
	defer eb.Close()
	ac := NewAutoCompactor(eb)

	bb := context.NewAttentionBlackboardWithConfig(context.ZoneConfig{
		Critical:   100,
		Supporting: 100,
		Actionable: 100,
	})

	// Fill to >85% utilization (compaction threshold)
	for i := 0; i < 26; i++ {
		item := context.NewContextItem(context.SourceMemoryLobe, context.CategoryMemory, "mem", context.ZoneSupporting)
		item.TokenCount = 10
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
	if stats.Utilization < 0.85 {
		t.Skipf("Utilization %f is below compaction threshold", stats.Utilization)
	}

	result := ac.CompactIfNeeded(bb)
	if result == nil {
		t.Error("Should return result when compaction performed")
	}
	if len(result.PrunedItems) == 0 {
		t.Error("Should have pruned items")
	}
}

func TestAutoCompactor_EventBusIntegration(t *testing.T) {
	eb := bus.New()
	defer eb.Close()
	ac := NewAutoCompactor(eb)

	var compactionNeeded, compactionDone bool
	var mu sync.Mutex

	eb.Subscribe(bus.EventTypeContextCompactionNeeded, func(e bus.Event) {
		mu.Lock()
		compactionNeeded = true
		mu.Unlock()
	})

	eb.Subscribe(bus.EventTypeContextCompactionDone, func(e bus.Event) {
		mu.Lock()
		compactionDone = true
		mu.Unlock()
	})

	bb := context.NewAttentionBlackboardWithConfig(context.ZoneConfig{
		Critical:   100,
		Supporting: 100,
		Actionable: 100,
	})

	// Fill to trigger compaction
	for i := 0; i < 28; i++ {
		item := context.NewContextItem(context.SourceMemoryLobe, context.CategoryMemory, "mem", context.ZoneSupporting)
		item.TokenCount = 10
		item.Priority = 0.3
		if !bb.Add(item) {
			item.Zone = context.ZoneCritical
			if !bb.Add(item) {
				item.Zone = context.ZoneActionable
				bb.Add(item)
			}
		}
	}

	ac.CompactIfNeeded(bb)

	// Wait for events
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	stats := bb.Stats()
	if stats.Utilization >= 0.85 {
		// Only check events if we actually triggered compaction
		if !compactionNeeded {
			t.Error("Expected CompactionNeeded event")
		}
		if !compactionDone {
			t.Error("Expected CompactionDone event")
		}
	}
}

func TestAutoCompactor_CompactWithPriority(t *testing.T) {
	eb := bus.New()
	defer eb.Close()
	ac := NewAutoCompactor(eb)

	bb := context.NewAttentionBlackboardWithConfig(context.ZoneConfig{
		Critical:   100,
		Supporting: 100,
		Actionable: 100,
	})

	// Fill to 95%+ for high priority
	for i := 0; i < 29; i++ {
		item := context.NewContextItem(context.SourceMemoryLobe, context.CategoryMemory, "mem", context.ZoneSupporting)
		item.TokenCount = 10
		item.Priority = 0.3
		if !bb.Add(item) {
			item.Zone = context.ZoneCritical
			if !bb.Add(item) {
				item.Zone = context.ZoneActionable
				bb.Add(item)
			}
		}
	}

	result := ac.CompactWithPriority(bb)

	if result == nil {
		t.Fatal("Expected result from priority compaction")
	}

	// High priority should be more aggressive (target 0.5)
	if result.UtilizationAfter > 0.6 {
		t.Errorf("High priority compaction should be aggressive, got %.2f", result.UtilizationAfter)
	}
}

func TestAutoCompactor_SetupTriggers(t *testing.T) {
	eb := bus.New()
	defer eb.Close()
	ac := NewAutoCompactor(eb)
	hc := health.NewHealthChecker()
	triggers := hc.Triggers()

	ac.SetupTriggers(triggers)

	bb := context.NewAttentionBlackboardWithConfig(context.ZoneConfig{
		Critical:   100,
		Supporting: 100,
		Actionable: 100,
	})

	// Fill to 90%+
	for i := 0; i < 28; i++ {
		item := context.NewContextItem(context.SourceMemoryLobe, context.CategoryMemory, "mem", context.ZoneSupporting)
		item.TokenCount = 10
		item.Priority = 0.3
		if !bb.Add(item) {
			item.Zone = context.ZoneCritical
			if !bb.Add(item) {
				item.Zone = context.ZoneActionable
				bb.Add(item)
			}
		}
		triggers.CheckBudget(bb)
	}

	// Wait for trigger processing
	time.Sleep(100 * time.Millisecond)

	// The trigger should have initiated compaction
	// We can't easily verify this without more instrumentation,
	// but at least verify no panic occurred
}

func TestAutoCompactor_HealthCheck(t *testing.T) {
	eb := bus.New()
	defer eb.Close()
	ac := NewAutoCompactor(eb)

	bb := context.NewAttentionBlackboard()

	report := ac.HealthCheck(bb)
	// HealthCheck returns a value, not pointer
	if report.Status != health.StatusHealthy {
		t.Errorf("Empty blackboard should be healthy, got %s", report.Status)
	}
}

func TestAutoCompactor_QuickHealthCheck(t *testing.T) {
	eb := bus.New()
	defer eb.Close()
	ac := NewAutoCompactor(eb)

	bb := context.NewAttentionBlackboard()

	status, score := ac.QuickHealthCheck(bb)
	if status != health.StatusHealthy {
		t.Errorf("Expected healthy, got %s", status)
	}
	if score < 90 {
		t.Errorf("Expected high score, got %d", score)
	}
}

func TestAutoCompactor_Accessors(t *testing.T) {
	eb := bus.New()
	defer eb.Close()
	ac := NewAutoCompactor(eb)

	if ac.Compactor() == nil {
		t.Error("Compactor() should not return nil")
	}
	if ac.HealthChecker() == nil {
		t.Error("HealthChecker() should not return nil")
	}
}

func TestAutoCompactor_NilEventBus(t *testing.T) {
	// Should work without event bus (nil)
	ac := NewAutoCompactor(nil)

	bb := context.NewAttentionBlackboardWithConfig(context.ZoneConfig{
		Critical:   100,
		Supporting: 100,
		Actionable: 100,
	})

	// Fill and compact - should not panic
	for i := 0; i < 28; i++ {
		item := context.NewContextItem(context.SourceMemoryLobe, context.CategoryMemory, "mem", context.ZoneSupporting)
		item.TokenCount = 10
		item.Priority = 0.3
		if !bb.Add(item) {
			item.Zone = context.ZoneCritical
			if !bb.Add(item) {
				item.Zone = context.ZoneActionable
				bb.Add(item)
			}
		}
	}

	// Should not panic even without event bus
	ac.CompactIfNeeded(bb)
}
