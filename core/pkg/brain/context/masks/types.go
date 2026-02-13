// Package masks provides per-lobe context filtering for the attention-aware blackboard.
//
// Context masks define what categories of context each lobe should see,
// enabling efficient context tailoring based on lobe specialization.
// This aligns with the biological metaphor where different brain regions
// receive filtered sensory input relevant to their function.
package masks

import (
	"github.com/normanking/cortex/pkg/brain/context"
)

// ContextMask defines filtering rules for a lobe's context view.
type ContextMask struct {
	// LobeID identifies which lobe this mask is for.
	LobeID context.LobeID

	// IncludeCategories specifies which categories this lobe should see.
	// Empty means include all (no category filtering).
	IncludeCategories []string

	// ExcludeCategories specifies which categories to hide from this lobe.
	// Exclusions take precedence over inclusions.
	ExcludeCategories []string

	// IncludeSources specifies which source lobes to include.
	// Empty means include all sources.
	IncludeSources []context.LobeID

	// ExcludeSources specifies which source lobes to exclude.
	ExcludeSources []context.LobeID

	// IncludeZones specifies which zones this lobe should see.
	// Empty means include all zones.
	IncludeZones []context.AttentionZone

	// MaxTokens limits the total tokens for this lobe's view.
	// 0 means use the default budget.
	MaxTokens int

	// MinPriority filters out items below this priority threshold.
	// 0.0 means include all priorities.
	MinPriority float64

	// Description explains what this mask does (for debugging/docs).
	Description string
}

// Matches returns true if the item passes this mask's filters.
func (m *ContextMask) Matches(item *context.ContextItem) bool {
	// Check zone inclusion
	if len(m.IncludeZones) > 0 {
		zoneMatch := false
		for _, zone := range m.IncludeZones {
			if item.Zone == zone {
				zoneMatch = true
				break
			}
		}
		if !zoneMatch {
			return false
		}
	}

	// Check category exclusion (exclusions take precedence)
	for _, cat := range m.ExcludeCategories {
		if item.Category == cat {
			return false
		}
	}

	// Check source exclusion
	for _, src := range m.ExcludeSources {
		if item.Source == src {
			return false
		}
	}

	// Check category inclusion (if specified)
	if len(m.IncludeCategories) > 0 {
		catMatch := false
		for _, cat := range m.IncludeCategories {
			if item.Category == cat {
				catMatch = true
				break
			}
		}
		if !catMatch {
			return false
		}
	}

	// Check source inclusion (if specified)
	if len(m.IncludeSources) > 0 {
		srcMatch := false
		for _, src := range m.IncludeSources {
			if item.Source == src {
				srcMatch = true
				break
			}
		}
		if !srcMatch {
			return false
		}
	}

	// Check priority threshold
	if m.MinPriority > 0 && item.Priority < m.MinPriority {
		return false
	}

	return true
}

// Apply filters items from a blackboard according to this mask.
// Returns items that pass the mask, respecting MaxTokens limit.
func (m *ContextMask) Apply(bb *context.AttentionBlackboard) []*context.ContextItem {
	allItems := bb.GetAll()
	result := make([]*context.ContextItem, 0)
	tokenCount := 0
	maxTokens := m.MaxTokens
	if maxTokens == 0 {
		maxTokens = bb.Stats().BudgetLimit // Use full budget
	}

	for _, item := range allItems {
		if !m.Matches(item) {
			continue
		}

		// Check token budget
		if tokenCount+item.TokenCount > maxTokens {
			break // Stop when budget exhausted
		}

		result = append(result, item)
		tokenCount += item.TokenCount
	}

	return result
}

// MaskRegistry provides access to lobe-specific context masks.
type MaskRegistry struct {
	masks map[context.LobeID]*ContextMask
}

// NewMaskRegistry creates a registry with all default lobe masks.
func NewMaskRegistry() *MaskRegistry {
	reg := &MaskRegistry{
		masks: make(map[context.LobeID]*ContextMask),
	}

	// Register all lobe masks
	for _, mask := range AllLobeMasks() {
		reg.masks[mask.LobeID] = mask
	}

	return reg
}

// Get returns the mask for a specific lobe.
// Returns nil if no mask is registered.
func (r *MaskRegistry) Get(lobeID context.LobeID) *ContextMask {
	return r.masks[lobeID]
}

// Register adds or updates a mask for a lobe.
func (r *MaskRegistry) Register(mask *ContextMask) {
	r.masks[mask.LobeID] = mask
}

// List returns all registered masks.
func (r *MaskRegistry) List() []*ContextMask {
	masks := make([]*ContextMask, 0, len(r.masks))
	for _, mask := range r.masks {
		masks = append(masks, mask)
	}
	return masks
}

// FilteredView returns a filtered context view for a specific lobe.
func (r *MaskRegistry) FilteredView(lobeID context.LobeID, bb *context.AttentionBlackboard) []*context.ContextItem {
	mask := r.Get(lobeID)
	if mask == nil {
		// No mask = see everything
		return bb.GetAll()
	}
	return mask.Apply(bb)
}
