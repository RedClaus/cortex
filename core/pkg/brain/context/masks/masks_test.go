package masks

import (
	"testing"

	"github.com/normanking/cortex/pkg/brain/context"
)

func TestContextMask_Matches_Category(t *testing.T) {
	mask := &ContextMask{
		IncludeCategories: []string{context.CategoryCode, context.CategoryError},
	}

	// Should match included category
	codeItem := context.NewContextItem(context.SourceCodingLobe, context.CategoryCode, "code", context.ZoneSupporting)
	if !mask.Matches(codeItem) {
		t.Error("Should match included category 'code'")
	}

	// Should not match excluded category
	emotionItem := context.NewContextItem(context.SourceEmotionLobe, context.CategoryEmotion, "emotion", context.ZoneSupporting)
	if mask.Matches(emotionItem) {
		t.Error("Should not match category 'emotion' (not in include list)")
	}
}

func TestContextMask_Matches_ExcludeCategory(t *testing.T) {
	mask := &ContextMask{
		ExcludeCategories: []string{context.CategoryEmotion},
	}

	// Should not match excluded category
	emotionItem := context.NewContextItem(context.SourceEmotionLobe, context.CategoryEmotion, "emotion", context.ZoneSupporting)
	if mask.Matches(emotionItem) {
		t.Error("Should not match excluded category")
	}

	// Should match non-excluded category
	codeItem := context.NewContextItem(context.SourceCodingLobe, context.CategoryCode, "code", context.ZoneSupporting)
	if !mask.Matches(codeItem) {
		t.Error("Should match non-excluded category")
	}
}

func TestContextMask_Matches_Zone(t *testing.T) {
	mask := &ContextMask{
		IncludeZones: []context.AttentionZone{context.ZoneCritical, context.ZoneActionable},
	}

	// Should match included zones
	criticalItem := context.NewContextItem(context.SourceSystem, context.CategorySystem, "sys", context.ZoneCritical)
	if !mask.Matches(criticalItem) {
		t.Error("Should match Critical zone")
	}

	actionableItem := context.NewContextItem(context.SourceSystem, context.CategoryTask, "task", context.ZoneActionable)
	if !mask.Matches(actionableItem) {
		t.Error("Should match Actionable zone")
	}

	// Should not match excluded zone
	supportingItem := context.NewContextItem(context.SourceMemoryLobe, context.CategoryMemory, "mem", context.ZoneSupporting)
	if mask.Matches(supportingItem) {
		t.Error("Should not match Supporting zone (not in include list)")
	}
}

func TestContextMask_Matches_Priority(t *testing.T) {
	mask := &ContextMask{
		MinPriority: 0.5,
	}

	// Should match high priority
	highPri := context.NewContextItem(context.SourceSystem, context.CategorySystem, "high", context.ZoneCritical)
	highPri.Priority = 0.8
	if !mask.Matches(highPri) {
		t.Error("Should match high priority item")
	}

	// Should not match low priority
	lowPri := context.NewContextItem(context.SourceSystem, context.CategorySystem, "low", context.ZoneCritical)
	lowPri.Priority = 0.3
	if mask.Matches(lowPri) {
		t.Error("Should not match low priority item")
	}
}

func TestContextMask_Matches_Source(t *testing.T) {
	mask := &ContextMask{
		IncludeSources: []context.LobeID{context.SourceCodingLobe, context.SourceMemoryLobe},
	}

	// Should match included source
	codingItem := context.NewContextItem(context.SourceCodingLobe, context.CategoryCode, "code", context.ZoneSupporting)
	if !mask.Matches(codingItem) {
		t.Error("Should match included source")
	}

	// Should not match excluded source
	emotionItem := context.NewContextItem(context.SourceEmotionLobe, context.CategoryEmotion, "emotion", context.ZoneSupporting)
	if mask.Matches(emotionItem) {
		t.Error("Should not match source not in include list")
	}
}

func TestContextMask_Matches_ExcludeSource(t *testing.T) {
	mask := &ContextMask{
		ExcludeSources: []context.LobeID{context.SourceEmotionLobe},
	}

	// Should not match excluded source
	emotionItem := context.NewContextItem(context.SourceEmotionLobe, context.CategoryEmotion, "emotion", context.ZoneSupporting)
	if mask.Matches(emotionItem) {
		t.Error("Should not match excluded source")
	}

	// Should match non-excluded source
	codingItem := context.NewContextItem(context.SourceCodingLobe, context.CategoryCode, "code", context.ZoneSupporting)
	if !mask.Matches(codingItem) {
		t.Error("Should match non-excluded source")
	}
}

func TestContextMask_Apply(t *testing.T) {
	bb := context.NewAttentionBlackboardWithConfig(context.ZoneConfig{
		Critical:   200,
		Supporting: 400,
		Actionable: 200,
	})

	// Add various items
	codeItem := context.NewContextItem(context.SourceCodingLobe, context.CategoryCode, "code", context.ZoneSupporting)
	codeItem.TokenCount = 50
	bb.Add(codeItem)

	errorItem := context.NewContextItem(context.SourceCodingLobe, context.CategoryError, "error", context.ZoneActionable)
	errorItem.TokenCount = 50
	bb.Add(errorItem)

	emotionItem := context.NewContextItem(context.SourceEmotionLobe, context.CategoryEmotion, "emotion", context.ZoneSupporting)
	emotionItem.TokenCount = 50
	bb.Add(emotionItem)

	// Create coding mask
	mask := &ContextMask{
		IncludeCategories: []string{context.CategoryCode, context.CategoryError},
		ExcludeCategories: []string{context.CategoryEmotion},
		MaxTokens:         1000,
	}

	result := mask.Apply(bb)

	// Should only include code and error items
	if len(result) != 2 {
		t.Errorf("Expected 2 items, got %d", len(result))
	}

	for _, item := range result {
		if item.Category == context.CategoryEmotion {
			t.Error("Should not include emotion items")
		}
	}
}

func TestContextMask_Apply_TokenLimit(t *testing.T) {
	bb := context.NewAttentionBlackboardWithConfig(context.ZoneConfig{
		Critical:   200,
		Supporting: 400,
		Actionable: 200,
	})

	// Add items that total more than mask limit
	for i := 0; i < 5; i++ {
		item := context.NewContextItem(context.SourceCodingLobe, context.CategoryCode, "code", context.ZoneSupporting)
		item.TokenCount = 50
		bb.Add(item)
	}

	mask := &ContextMask{
		MaxTokens: 120, // Only enough for 2 items (50*2=100, but not 50*3=150)
	}

	result := mask.Apply(bb)

	// Should stop at token limit
	if len(result) > 2 {
		t.Errorf("Expected at most 2 items due to token limit, got %d", len(result))
	}
}

func TestMaskRegistry(t *testing.T) {
	reg := NewMaskRegistry()

	// Should have masks for all lobes
	masks := reg.List()
	if len(masks) != 20 {
		t.Errorf("Expected 20 masks, got %d", len(masks))
	}

	// Should be able to get specific masks
	codingMask := reg.Get(context.SourceCodingLobe)
	if codingMask == nil {
		t.Error("Should have CodingLobe mask")
	}

	safetyMask := reg.Get(context.SourceSafetyLobe)
	if safetyMask == nil {
		t.Error("Should have SafetyLobe mask")
	}
}

func TestMaskRegistry_FilteredView(t *testing.T) {
	reg := NewMaskRegistry()

	bb := context.NewAttentionBlackboardWithConfig(context.ZoneConfig{
		Critical:   200,
		Supporting: 400,
		Actionable: 200,
	})

	// Add code and emotion items
	codeItem := context.NewContextItem(context.SourceCodingLobe, context.CategoryCode, "code", context.ZoneSupporting)
	codeItem.TokenCount = 50
	bb.Add(codeItem)

	emotionItem := context.NewContextItem(context.SourceEmotionLobe, context.CategoryEmotion, "emotion", context.ZoneSupporting)
	emotionItem.TokenCount = 50
	bb.Add(emotionItem)

	// Coding lobe should not see emotion items
	codingView := reg.FilteredView(context.SourceCodingLobe, bb)

	for _, item := range codingView {
		if item.Category == context.CategoryEmotion {
			t.Error("CodingLobe should not see emotion items")
		}
	}
}

func TestMaskRegistry_FilteredView_UnregisteredLobe(t *testing.T) {
	reg := NewMaskRegistry()

	bb := context.NewAttentionBlackboardWithConfig(context.ZoneConfig{
		Critical:   200,
		Supporting: 200,
		Actionable: 200,
	})

	bb.Add(context.NewContextItem(context.SourceSystem, context.CategorySystem, "sys", context.ZoneCritical))
	bb.Add(context.NewContextItem(context.SourceSystem, context.CategoryTask, "task", context.ZoneActionable))

	// Unknown lobe should see everything
	view := reg.FilteredView("unknown_lobe", bb)
	if len(view) != 2 {
		t.Errorf("Unknown lobe should see all items, got %d", len(view))
	}
}

func TestAllLobeMasks(t *testing.T) {
	masks := AllLobeMasks()

	if len(masks) != 20 {
		t.Errorf("Expected 20 lobe masks, got %d", len(masks))
	}

	// Verify each mask has a LobeID
	for _, mask := range masks {
		if mask.LobeID == "" {
			t.Error("Mask should have LobeID")
		}
		if mask.Description == "" {
			t.Errorf("Mask for %s should have description", mask.LobeID)
		}
	}

	// Verify key masks have expected properties
	for _, mask := range masks {
		switch mask.LobeID {
		case context.SourceCodingLobe:
			// Coding should exclude emotion
			found := false
			for _, cat := range mask.ExcludeCategories {
				if cat == context.CategoryEmotion {
					found = true
					break
				}
			}
			if !found {
				t.Error("CodingLobe mask should exclude emotion")
			}

		case context.SourceSafetyLobe:
			// Safety should have min priority
			if mask.MinPriority == 0 {
				t.Error("SafetyLobe mask should have MinPriority")
			}
		}
	}
}

func TestGetMaskForLobe(t *testing.T) {
	mask := GetMaskForLobe(context.SourceCodingLobe)
	if mask == nil {
		t.Fatal("Should find CodingLobe mask")
	}
	if mask.LobeID != context.SourceCodingLobe {
		t.Errorf("Expected CodingLobe, got %s", mask.LobeID)
	}

	// Unknown lobe should return nil
	unknown := GetMaskForLobe("unknown")
	if unknown != nil {
		t.Error("Unknown lobe should return nil")
	}
}

func TestDefaultMask(t *testing.T) {
	mask := DefaultMask()
	if mask == nil {
		t.Fatal("DefaultMask should not be nil")
	}

	// Default mask should match everything
	item := context.NewContextItem(context.SourceCodingLobe, context.CategoryCode, "test", context.ZoneSupporting)
	if !mask.Matches(item) {
		t.Error("Default mask should match all items")
	}
}
