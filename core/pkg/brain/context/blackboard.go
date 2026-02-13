package context

import (
	"sort"
	"sync"
	"time"
)

// AttentionBlackboard is an attention-aware shared memory space for cognitive processing.
// It organizes context into three zones (Critical, Supporting, Actionable) based on
// LLM attention patterns from "Lost in the Middle" research.
//
// Key features:
//   - Zone-based organization for optimal attention allocation
//   - Token budget enforcement per zone
//   - Priority-based retrieval and compaction
//   - Thread-safe operations
//
// This implements a superset of the original Blackboard interface for backward compatibility.
type AttentionBlackboard struct {
	mu sync.RWMutex

	// Zone-organized items
	critical   []*ContextItem
	supporting []*ContextItem
	actionable []*ContextItem

	// Quick lookup by ID
	itemIndex map[string]*ContextItem

	// Zone budgets
	budgets map[AttentionZone]*TokenBudget

	// Token estimator
	estimator *TokenEstimator

	// Configuration
	config ZoneConfig

	// Metrics
	metrics *BlackboardMetrics
}

// BlackboardMetrics tracks usage statistics.
type BlackboardMetrics struct {
	TotalAdds       int64
	TotalPrunes     int64
	BudgetViolations int64
	LastCompaction  time.Time
}

// NewAttentionBlackboard creates a new attention-aware blackboard with default configuration.
func NewAttentionBlackboard() *AttentionBlackboard {
	return NewAttentionBlackboardWithConfig(DefaultZoneConfig())
}

// NewAttentionBlackboardWithConfig creates a blackboard with custom zone configuration.
func NewAttentionBlackboardWithConfig(config ZoneConfig) *AttentionBlackboard {
	bb := &AttentionBlackboard{
		critical:   make([]*ContextItem, 0),
		supporting: make([]*ContextItem, 0),
		actionable: make([]*ContextItem, 0),
		itemIndex:  make(map[string]*ContextItem),
		budgets:    make(map[AttentionZone]*TokenBudget),
		estimator:  DefaultTokenEstimator(),
		config:     config,
		metrics:    &BlackboardMetrics{},
	}

	// Initialize zone budgets
	bb.budgets[ZoneCritical] = NewTokenBudget(config.Critical)
	bb.budgets[ZoneSupporting] = NewTokenBudget(config.Supporting)
	bb.budgets[ZoneActionable] = NewTokenBudget(config.Actionable)

	return bb
}

// Add inserts a context item into the appropriate zone.
// Returns false if the item couldn't fit within budget.
func (bb *AttentionBlackboard) Add(item *ContextItem) bool {
	bb.mu.Lock()
	defer bb.mu.Unlock()

	// Estimate tokens if not already set
	if item.TokenCount == 0 {
		item.TokenCount = bb.estimator.Estimate(item.Content)
	}

	// Check budget
	budget := bb.budgets[item.Zone]
	if budget == nil {
		budget = bb.budgets[ZoneSupporting] // Default to supporting
		item.Zone = ZoneSupporting
	}

	if !budget.CanFit(item.TokenCount) {
		bb.metrics.BudgetViolations++
		return false
	}

	// Add to zone
	switch item.Zone {
	case ZoneCritical:
		bb.critical = append(bb.critical, item)
	case ZoneActionable:
		bb.actionable = append(bb.actionable, item)
	default:
		bb.supporting = append(bb.supporting, item)
	}

	// Update index and budget
	bb.itemIndex[item.ID] = item
	budget.Use(item.TokenCount)
	bb.metrics.TotalAdds++

	return true
}

// AddForce inserts an item, pruning lower-priority items if necessary.
// Use for critical items that must be added.
func (bb *AttentionBlackboard) AddForce(item *ContextItem) {
	bb.mu.Lock()
	defer bb.mu.Unlock()

	// Estimate tokens if not already set
	if item.TokenCount == 0 {
		item.TokenCount = bb.estimator.Estimate(item.Content)
	}

	budget := bb.budgets[item.Zone]
	if budget == nil {
		budget = bb.budgets[ZoneSupporting]
		item.Zone = ZoneSupporting
	}

	// Prune until we have space
	for !budget.CanFit(item.TokenCount) {
		pruned := bb.pruneLowestPriority(item.Zone)
		if !pruned {
			break // Can't prune anymore
		}
	}

	// Add the item
	switch item.Zone {
	case ZoneCritical:
		bb.critical = append(bb.critical, item)
	case ZoneActionable:
		bb.actionable = append(bb.actionable, item)
	default:
		bb.supporting = append(bb.supporting, item)
	}

	bb.itemIndex[item.ID] = item
	budget.Use(item.TokenCount)
	bb.metrics.TotalAdds++
}

// pruneLowestPriority removes the lowest priority item from a zone.
// Must be called with lock held. Returns false if zone is empty.
func (bb *AttentionBlackboard) pruneLowestPriority(zone AttentionZone) bool {
	var items *[]*ContextItem
	switch zone {
	case ZoneCritical:
		items = &bb.critical
	case ZoneActionable:
		items = &bb.actionable
	default:
		items = &bb.supporting
	}

	if len(*items) == 0 {
		return false
	}

	// Find lowest priority item
	lowestIdx := 0
	lowestPriority := (*items)[0].EffectivePriority()
	for i, item := range *items {
		if item.EffectivePriority() < lowestPriority {
			lowestIdx = i
			lowestPriority = item.EffectivePriority()
		}
	}

	// Remove it
	removed := (*items)[lowestIdx]
	*items = append((*items)[:lowestIdx], (*items)[lowestIdx+1:]...)
	delete(bb.itemIndex, removed.ID)
	bb.budgets[zone].Release(removed.TokenCount)
	bb.metrics.TotalPrunes++

	return true
}

// Get retrieves an item by ID.
func (bb *AttentionBlackboard) Get(id string) (*ContextItem, bool) {
	bb.mu.RLock()
	defer bb.mu.RUnlock()
	item, ok := bb.itemIndex[id]
	return item, ok
}

// GetByCategory returns all items matching the category.
func (bb *AttentionBlackboard) GetByCategory(category string) []*ContextItem {
	bb.mu.RLock()
	defer bb.mu.RUnlock()

	var result []*ContextItem
	for _, item := range bb.itemIndex {
		if item.Category == category {
			result = append(result, item)
		}
	}
	return result
}

// GetBySource returns all items from a specific source lobe.
func (bb *AttentionBlackboard) GetBySource(source LobeID) []*ContextItem {
	bb.mu.RLock()
	defer bb.mu.RUnlock()

	var result []*ContextItem
	for _, item := range bb.itemIndex {
		if item.Source == source {
			result = append(result, item)
		}
	}
	return result
}

// GetZone returns all items in a specific zone, ordered by priority (highest first).
func (bb *AttentionBlackboard) GetZone(zone AttentionZone) []*ContextItem {
	bb.mu.RLock()
	defer bb.mu.RUnlock()

	var items []*ContextItem
	switch zone {
	case ZoneCritical:
		items = make([]*ContextItem, len(bb.critical))
		copy(items, bb.critical)
	case ZoneActionable:
		items = make([]*ContextItem, len(bb.actionable))
		copy(items, bb.actionable)
	default:
		items = make([]*ContextItem, len(bb.supporting))
		copy(items, bb.supporting)
	}

	// Sort by priority (highest first)
	sort.Slice(items, func(i, j int) bool {
		return items[i].Priority > items[j].Priority
	})

	return items
}

// GetAll returns all items in context order (Critical → Supporting → Actionable).
// This is the order to send to the LLM for optimal attention.
func (bb *AttentionBlackboard) GetAll() []*ContextItem {
	bb.mu.RLock()
	defer bb.mu.RUnlock()

	result := make([]*ContextItem, 0, len(bb.itemIndex))

	// Sort each zone by priority before concatenating
	for _, zone := range ZoneOrder() {
		items := bb.getZoneUnsorted(zone)
		sort.Slice(items, func(i, j int) bool {
			return items[i].Priority > items[j].Priority
		})
		result = append(result, items...)
	}

	return result
}

// getZoneUnsorted returns zone items without sorting. Must be called with lock held.
func (bb *AttentionBlackboard) getZoneUnsorted(zone AttentionZone) []*ContextItem {
	switch zone {
	case ZoneCritical:
		items := make([]*ContextItem, len(bb.critical))
		copy(items, bb.critical)
		return items
	case ZoneActionable:
		items := make([]*ContextItem, len(bb.actionable))
		copy(items, bb.actionable)
		return items
	default:
		items := make([]*ContextItem, len(bb.supporting))
		copy(items, bb.supporting)
		return items
	}
}

// Remove deletes an item by ID.
func (bb *AttentionBlackboard) Remove(id string) bool {
	bb.mu.Lock()
	defer bb.mu.Unlock()

	item, ok := bb.itemIndex[id]
	if !ok {
		return false
	}

	// Remove from zone slice
	switch item.Zone {
	case ZoneCritical:
		bb.critical = removeFromSlice(bb.critical, id)
	case ZoneActionable:
		bb.actionable = removeFromSlice(bb.actionable, id)
	default:
		bb.supporting = removeFromSlice(bb.supporting, id)
	}

	// Update index and budget
	delete(bb.itemIndex, id)
	bb.budgets[item.Zone].Release(item.TokenCount)

	return true
}

// removeFromSlice removes an item by ID from a slice.
func removeFromSlice(items []*ContextItem, id string) []*ContextItem {
	for i, item := range items {
		if item.ID == id {
			return append(items[:i], items[i+1:]...)
		}
	}
	return items
}

// Clear removes all items from the blackboard.
func (bb *AttentionBlackboard) Clear() {
	bb.mu.Lock()
	defer bb.mu.Unlock()

	bb.critical = make([]*ContextItem, 0)
	bb.supporting = make([]*ContextItem, 0)
	bb.actionable = make([]*ContextItem, 0)
	bb.itemIndex = make(map[string]*ContextItem)

	// Reset budgets
	for zone, budget := range bb.budgets {
		bb.budgets[zone] = NewTokenBudget(budget.Limit)
	}
}

// Stats returns current blackboard statistics.
func (bb *AttentionBlackboard) Stats() BlackboardStats {
	bb.mu.RLock()
	defer bb.mu.RUnlock()

	return BlackboardStats{
		TotalItems:     len(bb.itemIndex),
		CriticalItems:  len(bb.critical),
		SupportingItems: len(bb.supporting),
		ActionableItems: len(bb.actionable),
		CriticalTokens:  bb.budgets[ZoneCritical].Used,
		SupportingTokens: bb.budgets[ZoneSupporting].Used,
		ActionableTokens: bb.budgets[ZoneActionable].Used,
		TotalTokens:     bb.totalTokensUnsafe(),
		BudgetLimit:     bb.config.Total(),
		Utilization:     float64(bb.totalTokensUnsafe()) / float64(bb.config.Total()),
	}
}

// totalTokensUnsafe returns total tokens. Must be called with lock held.
func (bb *AttentionBlackboard) totalTokensUnsafe() int {
	return bb.budgets[ZoneCritical].Used +
		bb.budgets[ZoneSupporting].Used +
		bb.budgets[ZoneActionable].Used
}

// BlackboardStats contains blackboard usage statistics.
type BlackboardStats struct {
	TotalItems       int
	CriticalItems    int
	SupportingItems  int
	ActionableItems  int
	CriticalTokens   int
	SupportingTokens int
	ActionableTokens int
	TotalTokens      int
	BudgetLimit      int
	Utilization      float64 // 0.0-1.0
}

// ZoneUtilization returns the token utilization for a specific zone.
func (bb *AttentionBlackboard) ZoneUtilization(zone AttentionZone) float64 {
	bb.mu.RLock()
	defer bb.mu.RUnlock()

	budget := bb.budgets[zone]
	if budget == nil {
		return 0
	}
	return budget.Utilization()
}

// Clone creates a deep copy of the blackboard.
func (bb *AttentionBlackboard) Clone() *AttentionBlackboard {
	bb.mu.RLock()
	defer bb.mu.RUnlock()

	clone := NewAttentionBlackboardWithConfig(bb.config)

	// Copy items
	for _, item := range bb.critical {
		cloneItem := *item
		clone.critical = append(clone.critical, &cloneItem)
		clone.itemIndex[item.ID] = &cloneItem
		clone.budgets[ZoneCritical].Use(item.TokenCount)
	}
	for _, item := range bb.supporting {
		cloneItem := *item
		clone.supporting = append(clone.supporting, &cloneItem)
		clone.itemIndex[item.ID] = &cloneItem
		clone.budgets[ZoneSupporting].Use(item.TokenCount)
	}
	for _, item := range bb.actionable {
		cloneItem := *item
		clone.actionable = append(clone.actionable, &cloneItem)
		clone.itemIndex[item.ID] = &cloneItem
		clone.budgets[ZoneActionable].Use(item.TokenCount)
	}

	return clone
}

// Metrics returns blackboard metrics for monitoring.
func (bb *AttentionBlackboard) Metrics() BlackboardMetrics {
	bb.mu.RLock()
	defer bb.mu.RUnlock()
	return *bb.metrics
}
