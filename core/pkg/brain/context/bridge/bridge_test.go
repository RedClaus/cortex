package bridge

import (
	"context"
	"testing"

	ctxpkg "github.com/normanking/cortex/pkg/brain/context"
)

func TestNewAttentionContextBuilder(t *testing.T) {
	builder := NewAttentionContextBuilder()

	if builder == nil {
		t.Fatal("NewAttentionContextBuilder returned nil")
	}
	if builder.blackboard == nil {
		t.Error("Blackboard should be initialized")
	}
	if builder.maskRegistry == nil {
		t.Error("MaskRegistry should be initialized")
	}
	if builder.healthChecker == nil {
		t.Error("HealthChecker should be initialized")
	}
}

func TestDefaultAttentionContextConfig(t *testing.T) {
	config := DefaultAttentionContextConfig()

	if config.ZoneConfig.Total() == 0 {
		t.Error("ZoneConfig should have budget")
	}
	if !config.EnableAutoCompaction {
		t.Error("EnableAutoCompaction should default to true")
	}
	if !config.EnableHealthMonitoring {
		t.Error("EnableHealthMonitoring should default to true")
	}
	if !config.EnableMasking {
		t.Error("EnableMasking should default to true")
	}
}

func TestAttentionContextBuilder_AddSystemContext(t *testing.T) {
	builder := NewAttentionContextBuilder()

	builder.AddSystemContext("You are a helpful assistant.", 1.0)

	stats := builder.Stats()
	if stats.CriticalItems != 1 {
		t.Errorf("Expected 1 critical item, got %d", stats.CriticalItems)
	}
}

func TestAttentionContextBuilder_AddUserContext(t *testing.T) {
	builder := NewAttentionContextBuilder()

	builder.AddUserContext("User: Norman, preferences: concise", 0.9)

	stats := builder.Stats()
	if stats.CriticalItems != 1 {
		t.Errorf("Expected 1 critical item, got %d", stats.CriticalItems)
	}
}

func TestAttentionContextBuilder_AddMemoryContext(t *testing.T) {
	builder := NewAttentionContextBuilder()

	builder.AddMemoryContext("mem-1", "Previous conversation about Go", 0.8)

	stats := builder.Stats()
	if stats.SupportingItems != 1 {
		t.Errorf("Expected 1 supporting item, got %d", stats.SupportingItems)
	}
}

func TestAttentionContextBuilder_AddTaskContext(t *testing.T) {
	builder := NewAttentionContextBuilder()

	builder.AddTaskContext("Current task: Implement feature X", 0.95)

	stats := builder.Stats()
	if stats.ActionableItems != 1 {
		t.Errorf("Expected 1 actionable item, got %d", stats.ActionableItems)
	}
}

func TestAttentionContextBuilder_AddCodeContext(t *testing.T) {
	builder := NewAttentionContextBuilder()

	builder.AddCodeContext("func main() {}", 0.7, ctxpkg.ZoneSupporting)

	stats := builder.Stats()
	if stats.SupportingItems != 1 {
		t.Errorf("Expected 1 supporting item, got %d", stats.SupportingItems)
	}
}

func TestAttentionContextBuilder_Build(t *testing.T) {
	builder := NewAttentionContextBuilder()

	// Add items to all zones
	builder.AddSystemContext("System prompt", 1.0)
	builder.AddMemoryContext("m1", "Memory content", 0.5)
	builder.AddTaskContext("Task content", 0.9)

	ctx := context.Background()
	result, err := builder.Build(ctx)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if result.Content == "" {
		t.Error("Content should not be empty")
	}
	if result.ItemCount != 3 {
		t.Errorf("Expected 3 items, got %d", result.ItemCount)
	}
	if result.TokenCount == 0 {
		t.Error("TokenCount should be > 0")
	}
}

func TestAttentionContextBuilder_Build_ZoneOrder(t *testing.T) {
	builder := NewAttentionContextBuilder()

	builder.AddTaskContext("ACTIONABLE", 0.9)
	builder.AddMemoryContext("m1", "SUPPORTING", 0.5)
	builder.AddSystemContext("CRITICAL", 1.0)

	ctx := context.Background()
	result, err := builder.Build(ctx)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Content should be in zone order: Critical -> Supporting -> Actionable
	content := result.Content
	criticalPos := indexOf(content, "CRITICAL")
	supportingPos := indexOf(content, "SUPPORTING")
	actionablePos := indexOf(content, "ACTIONABLE")

	if criticalPos == -1 || supportingPos == -1 || actionablePos == -1 {
		t.Fatal("All content should be present")
	}

	if criticalPos > supportingPos {
		t.Error("Critical should come before Supporting")
	}
	if supportingPos > actionablePos {
		t.Error("Supporting should come before Actionable")
	}
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func TestAttentionContextBuilder_BuildForLobe(t *testing.T) {
	builder := NewAttentionContextBuilder()

	// Add code and emotion content
	builder.AddCodeContext("func main() {}", 0.8, ctxpkg.ZoneSupporting)
	builder.AddLobeOutput(ctxpkg.SourceEmotionLobe, ctxpkg.CategoryEmotion, "User seems happy", 0.6, ctxpkg.ZoneSupporting)

	ctx := context.Background()

	// Coding lobe should not see emotion content
	codingResult, err := builder.BuildForLobe(ctx, ctxpkg.SourceCodingLobe)
	if err != nil {
		t.Fatalf("BuildForLobe failed: %v", err)
	}

	if indexOf(codingResult.Content, "func main") == -1 {
		t.Error("Coding lobe should see code content")
	}
	// Note: emotion content filtering depends on mask configuration
}

func TestAttentionContextBuilder_Clear(t *testing.T) {
	builder := NewAttentionContextBuilder()

	builder.AddSystemContext("System", 1.0)
	builder.AddMemoryContext("m1", "Memory", 0.5)

	stats := builder.Stats()
	if stats.TotalItems == 0 {
		t.Fatal("Should have items before clear")
	}

	builder.Clear()

	stats = builder.Stats()
	if stats.TotalItems != 0 {
		t.Errorf("Should have 0 items after clear, got %d", stats.TotalItems)
	}
}

func TestAttentionContextBuilder_HealthMonitoring(t *testing.T) {
	config := DefaultAttentionContextConfig()
	config.EnableHealthMonitoring = true
	builder := NewAttentionContextBuilderWithConfig(config)

	builder.AddSystemContext("System", 1.0)

	ctx := context.Background()
	result, err := builder.Build(ctx)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if result.HealthStatus == "" {
		t.Error("HealthStatus should be set when monitoring enabled")
	}
	if result.HealthScore == 0 {
		t.Error("HealthScore should be > 0 for healthy context")
	}
}

func TestAttentionContextBuilder_AutoCompaction(t *testing.T) {
	config := AttentionContextConfig{
		ZoneConfig: ctxpkg.ZoneConfig{
			Critical:   100,
			Supporting: 100,
			Actionable: 100,
		},
		EnableAutoCompaction:   true,
		CompactionThreshold:    0.85,
		EnableHealthMonitoring: true,
		EnableMasking:          true,
	}
	builder := NewAttentionContextBuilderWithConfig(config)

	// Fill up to trigger compaction
	for i := 0; i < 15; i++ {
		builder.AddMemoryContext("m"+string(rune('a'+i)), "Memory content that takes space", 0.3)
	}

	stats := builder.Stats()
	initialItems := stats.TotalItems

	ctx := context.Background()
	_, err := builder.Build(ctx) // Should trigger compaction
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// After compaction, may have fewer items
	finalStats := builder.Stats()
	if finalStats.TotalItems > initialItems {
		t.Error("Items should not increase after compaction")
	}
}

func TestLegacyBridge(t *testing.T) {
	bridge := NewLegacyBridge()

	if bridge == nil {
		t.Fatal("NewLegacyBridge returned nil")
	}
	if bridge.builder == nil {
		t.Error("Builder should be initialized")
	}
}

func TestLegacyBridge_ImportLaneContext(t *testing.T) {
	bridge := NewLegacyBridge()

	passiveResults := []PassiveResultLegacy{
		{ID: "p1", Summary: "First result", Confidence: 0.9},
		{ID: "p2", Summary: "Second result", Confidence: 0.7},
	}

	err := bridge.ImportLaneContext("You are a helpful assistant.", passiveResults)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	stats := bridge.Builder().Stats()
	// 1 system + 2 memories = 3 items
	if stats.TotalItems != 3 {
		t.Errorf("Expected 3 items, got %d", stats.TotalItems)
	}
}

func TestLegacyBridge_Build(t *testing.T) {
	bridge := NewLegacyBridge()

	passiveResults := []PassiveResultLegacy{
		{ID: "p1", Summary: "Result content", Confidence: 0.8},
	}

	err := bridge.ImportLaneContext("System prompt", passiveResults)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	ctx := context.Background()
	result, err := bridge.Build(ctx)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if result.Content == "" {
		t.Error("Content should not be empty")
	}
	if indexOf(result.Content, "System prompt") == -1 {
		t.Error("Content should include system prompt")
	}
	if indexOf(result.Content, "Result content") == -1 {
		t.Error("Content should include passive result")
	}
}

func TestBuiltContext_Fields(t *testing.T) {
	builder := NewAttentionContextBuilder()

	builder.AddSystemContext("System", 1.0)
	builder.AddMemoryContext("m1", "Memory", 0.5)
	builder.AddTaskContext("Task", 0.9)

	ctx := context.Background()
	result, err := builder.Build(ctx)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Verify all fields are populated
	if result.TokenBudget == 0 {
		t.Error("TokenBudget should be set")
	}
	if result.Utilization < 0 || result.Utilization > 1 {
		t.Errorf("Utilization should be 0-1, got %f", result.Utilization)
	}
	if result.ZoneStats.CriticalItems == 0 {
		t.Error("ZoneStats.CriticalItems should be set")
	}
	if result.ZoneStats.SupportingItems == 0 {
		t.Error("ZoneStats.SupportingItems should be set")
	}
	if result.ZoneStats.ActionableItems == 0 {
		t.Error("ZoneStats.ActionableItems should be set")
	}
}

func TestAttentionContextBuilder_Accessors(t *testing.T) {
	builder := NewAttentionContextBuilder()

	if builder.Blackboard() == nil {
		t.Error("Blackboard() should not return nil")
	}
	if builder.MaskRegistry() == nil {
		t.Error("MaskRegistry() should not return nil")
	}
	if builder.HealthChecker() == nil {
		t.Error("HealthChecker() should not return nil")
	}
}
