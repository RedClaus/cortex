// Package bridge provides integration between the CR-022 AttentionBlackboard
// and the existing context building system.
//
// This bridge enables gradual migration from the legacy ContextBuilder
// to the new attention-aware context engineering system.
package bridge

import (
	"context"
	"strings"

	ctxpkg "github.com/normanking/cortex/pkg/brain/context"
	"github.com/normanking/cortex/pkg/brain/context/compaction"
	"github.com/normanking/cortex/pkg/brain/context/health"
	"github.com/normanking/cortex/pkg/brain/context/masks"
)

// AttentionContextBuilder wraps the AttentionBlackboard for context building.
// It provides a higher-level API for assembling context for LLM requests.
type AttentionContextBuilder struct {
	// Blackboard is the underlying attention-aware storage.
	blackboard *ctxpkg.AttentionBlackboard

	// Registry of lobe masks for filtering.
	maskRegistry *masks.MaskRegistry

	// Health checker for monitoring.
	healthChecker *health.HealthChecker

	// Compactor for automatic pruning.
	compactor *compaction.Compactor

	// Config for builder behavior.
	config AttentionContextConfig
}

// AttentionContextConfig configures the attention-aware context builder.
type AttentionContextConfig struct {
	// ZoneConfig defines token budgets per zone.
	ZoneConfig ctxpkg.ZoneConfig

	// EnableAutoCompaction enables automatic pruning when threshold exceeded.
	EnableAutoCompaction bool

	// CompactionThreshold triggers compaction at this utilization (0.0-1.0).
	CompactionThreshold float64

	// EnableHealthMonitoring enables health checking.
	EnableHealthMonitoring bool

	// EnableMasking enables per-lobe context filtering.
	EnableMasking bool
}

// DefaultAttentionContextConfig returns sensible defaults.
func DefaultAttentionContextConfig() AttentionContextConfig {
	return AttentionContextConfig{
		ZoneConfig:             ctxpkg.DefaultZoneConfig(),
		EnableAutoCompaction:   true,
		CompactionThreshold:    0.85,
		EnableHealthMonitoring: true,
		EnableMasking:          true,
	}
}

// NewAttentionContextBuilder creates a new attention-aware context builder.
func NewAttentionContextBuilder() *AttentionContextBuilder {
	return NewAttentionContextBuilderWithConfig(DefaultAttentionContextConfig())
}

// NewAttentionContextBuilderWithConfig creates a builder with custom config.
func NewAttentionContextBuilderWithConfig(config AttentionContextConfig) *AttentionContextBuilder {
	return &AttentionContextBuilder{
		blackboard:    ctxpkg.NewAttentionBlackboardWithConfig(config.ZoneConfig),
		maskRegistry:  masks.NewMaskRegistry(),
		healthChecker: health.NewHealthChecker(),
		compactor:     compaction.NewCompactor(),
		config:        config,
	}
}

// AddSystemContext adds system-level context to the Critical zone.
func (b *AttentionContextBuilder) AddSystemContext(content string, priority float64) {
	item := ctxpkg.NewContextItem(
		ctxpkg.SourceSystem,
		ctxpkg.CategorySystem,
		content,
		ctxpkg.ZoneCritical,
	).WithPriority(priority)
	b.blackboard.AddForce(item)
}

// AddUserContext adds user profile context to the Critical zone.
func (b *AttentionContextBuilder) AddUserContext(content string, priority float64) {
	item := ctxpkg.NewContextItem(
		ctxpkg.SourceSystem,
		ctxpkg.CategoryUser,
		content,
		ctxpkg.ZoneCritical,
	).WithPriority(priority)
	b.blackboard.Add(item)
}

// AddMemoryContext adds retrieved memories to the Supporting zone.
func (b *AttentionContextBuilder) AddMemoryContext(id, content string, relevance float64) {
	item := ctxpkg.NewContextItem(
		ctxpkg.SourceMemoryLobe,
		ctxpkg.CategoryMemory,
		content,
		ctxpkg.ZoneSupporting,
	).WithPriority(relevance)
	item.ID = id
	b.blackboard.Add(item)
}

// AddTaskContext adds current task to the Actionable zone.
func (b *AttentionContextBuilder) AddTaskContext(content string, priority float64) {
	item := ctxpkg.NewContextItem(
		ctxpkg.SourcePlanningLobe,
		ctxpkg.CategoryTask,
		content,
		ctxpkg.ZoneActionable,
	).WithPriority(priority)
	b.blackboard.AddForce(item)
}

// AddCodeContext adds code-related context.
func (b *AttentionContextBuilder) AddCodeContext(content string, priority float64, zone ctxpkg.AttentionZone) {
	item := ctxpkg.NewContextItem(
		ctxpkg.SourceCodingLobe,
		ctxpkg.CategoryCode,
		content,
		zone,
	).WithPriority(priority)
	b.blackboard.Add(item)
}

// AddLobeOutput adds output from a specific lobe.
func (b *AttentionContextBuilder) AddLobeOutput(lobeID ctxpkg.LobeID, category, content string, priority float64, zone ctxpkg.AttentionZone) {
	item := ctxpkg.NewContextItem(
		lobeID,
		category,
		content,
		zone,
	).WithPriority(priority)
	b.blackboard.Add(item)
}

// Build assembles the final context for LLM submission.
// Returns the context string and metadata.
func (b *AttentionContextBuilder) Build(ctx context.Context) (*BuiltContext, error) {
	// Auto-compact if needed
	if b.config.EnableAutoCompaction && b.healthChecker.NeedsCompaction(b.blackboard) {
		b.compactor.Prune(b.blackboard)
	}

	// Get all items in attention-optimized order
	items := b.blackboard.GetAll()

	// Build the context string
	var parts []string
	for _, item := range items {
		if content := item.ContentString(); content != "" {
			parts = append(parts, content)
		}
	}

	stats := b.blackboard.Stats()
	result := &BuiltContext{
		Content:     strings.Join(parts, "\n\n"),
		TokenCount:  stats.TotalTokens,
		TokenBudget: stats.BudgetLimit,
		Utilization: stats.Utilization,
		ItemCount:   stats.TotalItems,
		ZoneStats: ZoneStats{
			CriticalItems:    stats.CriticalItems,
			CriticalTokens:   stats.CriticalTokens,
			SupportingItems:  stats.SupportingItems,
			SupportingTokens: stats.SupportingTokens,
			ActionableItems:  stats.ActionableItems,
			ActionableTokens: stats.ActionableTokens,
		},
	}

	// Add health status if monitoring enabled
	if b.config.EnableHealthMonitoring {
		status, score := b.healthChecker.QuickCheck(b.blackboard)
		result.HealthStatus = string(status)
		result.HealthScore = score
	}

	return result, nil
}

// BuildForLobe assembles context filtered for a specific lobe.
func (b *AttentionContextBuilder) BuildForLobe(ctx context.Context, lobeID ctxpkg.LobeID) (*BuiltContext, error) {
	if !b.config.EnableMasking {
		return b.Build(ctx)
	}

	// Get filtered view for this lobe
	items := b.maskRegistry.FilteredView(lobeID, b.blackboard)

	// Build the context string
	var parts []string
	var tokenCount int
	for _, item := range items {
		if content := item.ContentString(); content != "" {
			parts = append(parts, content)
			tokenCount += item.TokenCount
		}
	}

	return &BuiltContext{
		Content:     strings.Join(parts, "\n\n"),
		TokenCount:  tokenCount,
		TokenBudget: b.blackboard.Stats().BudgetLimit,
		Utilization: float64(tokenCount) / float64(b.blackboard.Stats().BudgetLimit),
		ItemCount:   len(items),
		FilteredFor: string(lobeID),
	}, nil
}

// BuiltContext is the assembled context ready for LLM submission.
type BuiltContext struct {
	// Content is the assembled context string.
	Content string

	// TokenCount is the total tokens in the context.
	TokenCount int

	// TokenBudget is the maximum allowed tokens.
	TokenBudget int

	// Utilization is TokenCount/TokenBudget.
	Utilization float64

	// ItemCount is the number of context items.
	ItemCount int

	// ZoneStats breaks down items by zone.
	ZoneStats ZoneStats

	// HealthStatus is the current health status (if monitoring enabled).
	HealthStatus string

	// HealthScore is 0-100 health score (if monitoring enabled).
	HealthScore int

	// FilteredFor indicates if this was filtered for a specific lobe.
	FilteredFor string
}

// ZoneStats provides per-zone statistics.
type ZoneStats struct {
	CriticalItems    int
	CriticalTokens   int
	SupportingItems  int
	SupportingTokens int
	ActionableItems  int
	ActionableTokens int
}

// Clear resets the builder for a new request.
func (b *AttentionContextBuilder) Clear() {
	b.blackboard.Clear()
}

// Blackboard returns the underlying blackboard for direct access.
func (b *AttentionContextBuilder) Blackboard() *ctxpkg.AttentionBlackboard {
	return b.blackboard
}

// MaskRegistry returns the mask registry for customization.
func (b *AttentionContextBuilder) MaskRegistry() *masks.MaskRegistry {
	return b.maskRegistry
}

// HealthChecker returns the health checker for monitoring.
func (b *AttentionContextBuilder) HealthChecker() *health.HealthChecker {
	return b.healthChecker
}

// Stats returns current blackboard statistics.
func (b *AttentionContextBuilder) Stats() ctxpkg.BlackboardStats {
	return b.blackboard.Stats()
}

// LegacyBridge helps migrate from the old ContextBuilder to AttentionContextBuilder.
type LegacyBridge struct {
	builder *AttentionContextBuilder
}

// NewLegacyBridge creates a bridge for migration.
func NewLegacyBridge() *LegacyBridge {
	return &LegacyBridge{
		builder: NewAttentionContextBuilder(),
	}
}

// ImportLaneContext converts a legacy LaneContext to AttentionBlackboard items.
// This allows gradual migration by importing existing context into the new system.
func (lb *LegacyBridge) ImportLaneContext(systemPrompt string, passiveResults []PassiveResultLegacy) error {
	lb.builder.Clear()

	// Add system prompt as Critical
	if systemPrompt != "" {
		lb.builder.AddSystemContext(systemPrompt, 1.0)
	}

	// Add passive results as Supporting
	for _, pr := range passiveResults {
		lb.builder.AddMemoryContext(pr.ID, pr.Summary, pr.Confidence)
	}

	return nil
}

// PassiveResultLegacy mirrors the legacy PassiveResult structure.
type PassiveResultLegacy struct {
	ID         string
	Summary    string
	Confidence float64
}

// Build returns the context using the new attention-aware system.
func (lb *LegacyBridge) Build(ctx context.Context) (*BuiltContext, error) {
	return lb.builder.Build(ctx)
}

// Builder returns the underlying builder for advanced usage.
func (lb *LegacyBridge) Builder() *AttentionContextBuilder {
	return lb.builder
}
