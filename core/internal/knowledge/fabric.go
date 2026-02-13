package knowledge

import (
	"context"
	"fmt"
	"time"

	"github.com/normanking/cortex/pkg/types"
)

// KnowledgeFabric is the main interface for knowledge operations in Cortex.
// It provides a unified API for storing, retrieving, and managing knowledge across
// three tiers: Global (read-only) → Team (shared) → Personal (private).
type KnowledgeFabric interface {
	// ═══════════════════════════════════════════════════════════════════════════════
	// RETRIEVAL
	// ═══════════════════════════════════════════════════════════════════════════════

	// Search performs tiered retrieval with the following priority:
	// 1. Tier 1 (STRICT): Exact tag match, confidence > 0.8
	// 2. Tier 2 (FUZZY): FTS5 search + trust-weighted ranking
	// 3. Tier 3 (FALLBACK): Return partial results + flag for LLM generation
	Search(ctx context.Context, query string, opts types.SearchOptions) (*types.RetrievalResult, error)

	// GetByID retrieves a single knowledge item by its ID.
	GetByID(ctx context.Context, id string) (*types.KnowledgeItem, error)

	// GetByScope retrieves all knowledge items for a specific tier.
	GetByScope(ctx context.Context, scope types.Scope) ([]*types.KnowledgeItem, error)

	// ═══════════════════════════════════════════════════════════════════════════════
	// MUTATION
	// ═══════════════════════════════════════════════════════════════════════════════

	// Create adds a new knowledge item to the fabric.
	Create(ctx context.Context, item *types.KnowledgeItem) error

	// Update modifies an existing knowledge item.
	Update(ctx context.Context, item *types.KnowledgeItem) error

	// Delete soft-deletes a knowledge item.
	Delete(ctx context.Context, id string) error

	// ═══════════════════════════════════════════════════════════════════════════════
	// TRUST FEEDBACK
	// ═══════════════════════════════════════════════════════════════════════════════

	// RecordSuccess increments the success counter and recalculates trust score.
	RecordSuccess(ctx context.Context, id string) error

	// RecordFailure increments the failure counter and recalculates trust score.
	RecordFailure(ctx context.Context, id string) error
}

// Fabric implements KnowledgeFabric using dependency injection for testability.
type Fabric struct {
	store    Store         // Data persistence layer
	searcher Searcher      // FTS5 full-text search
	merger   MergeStrategy // Conflict resolution (for sync)
}

// NewFabric creates a new KnowledgeFabric instance with the provided dependencies.
func NewFabric(store Store, searcher Searcher, merger MergeStrategy) KnowledgeFabric {
	return &Fabric{
		store:    store,
		searcher: searcher,
		merger:   merger,
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// RETRIEVAL IMPLEMENTATION
// ═══════════════════════════════════════════════════════════════════════════════

// Search implements the three-tier retrieval strategy.
// See retrieval.go for the detailed logic.
func (f *Fabric) Search(ctx context.Context, query string, opts types.SearchOptions) (*types.RetrievalResult, error) {
	return performTieredRetrieval(ctx, f.store, f.searcher, query, opts)
}

// GetByID retrieves a single knowledge item by its ID.
func (f *Fabric) GetByID(ctx context.Context, id string) (*types.KnowledgeItem, error) {
	if id == "" {
		return nil, fmt.Errorf("knowledge item ID cannot be empty")
	}

	item, err := f.store.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve knowledge item %s: %w", id, err)
	}

	// Increment access count asynchronously (don't block on this)
	go func() {
		ctx := context.Background()
		if err := f.store.IncrementAccessCount(ctx, id); err != nil {
			// Log error but don't fail the request
			// TODO: Add proper logging
			_ = err
		}
	}()

	return item, nil
}

// GetByScope retrieves all knowledge items for a specific tier.
func (f *Fabric) GetByScope(ctx context.Context, scope types.Scope) ([]*types.KnowledgeItem, error) {
	if scope != types.ScopeGlobal && scope != types.ScopeTeam && scope != types.ScopePersonal {
		return nil, fmt.Errorf("invalid scope: %s", scope)
	}

	items, err := f.store.GetByScope(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve knowledge items for scope %s: %w", scope, err)
	}

	return items, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// MUTATION IMPLEMENTATION
// ═══════════════════════════════════════════════════════════════════════════════

// Create adds a new knowledge item to the fabric.
func (f *Fabric) Create(ctx context.Context, item *types.KnowledgeItem) error {
	if item == nil {
		return fmt.Errorf("knowledge item cannot be nil")
	}

	// Validate required fields
	if item.Content == "" {
		return fmt.Errorf("knowledge item content cannot be empty")
	}
	if item.AuthorID == "" {
		return fmt.Errorf("knowledge item must have an author")
	}
	if item.Scope != types.ScopeGlobal && item.Scope != types.ScopeTeam && item.Scope != types.ScopePersonal {
		return fmt.Errorf("invalid scope: %s", item.Scope)
	}

	// Set timestamps
	now := time.Now()
	item.CreatedAt = now
	item.UpdatedAt = now

	// Initialize quality signals if not set
	if item.TrustScore == 0 {
		item.TrustScore = 0.5 // Neutral starting score
	}
	if item.Confidence == 0 {
		item.Confidence = 0.5 // Neutral starting confidence
	}

	// Set sync status
	if item.SyncStatus == "" {
		item.SyncStatus = "local_only"
	}

	// Create in store
	if err := f.store.Create(ctx, item); err != nil {
		return fmt.Errorf("failed to create knowledge item: %w", err)
	}

	return nil
}

// Update modifies an existing knowledge item.
func (f *Fabric) Update(ctx context.Context, item *types.KnowledgeItem) error {
	if item == nil {
		return fmt.Errorf("knowledge item cannot be nil")
	}
	if item.ID == "" {
		return fmt.Errorf("knowledge item ID cannot be empty")
	}

	// Validate required fields
	if item.Content == "" {
		return fmt.Errorf("knowledge item content cannot be empty")
	}

	// Update timestamp and version
	item.UpdatedAt = time.Now()
	item.Version++

	// Mark as needing sync
	if item.SyncStatus == "synced" {
		item.SyncStatus = "pending"
	}

	// Update in store
	if err := f.store.Update(ctx, item); err != nil {
		return fmt.Errorf("failed to update knowledge item: %w", err)
	}

	return nil
}

// Delete soft-deletes a knowledge item by setting DeletedAt.
func (f *Fabric) Delete(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("knowledge item ID cannot be empty")
	}

	// Verify item exists
	item, err := f.store.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to retrieve knowledge item for deletion: %w", err)
	}

	// Prevent deletion of global knowledge (read-only)
	if item.Scope == types.ScopeGlobal {
		return fmt.Errorf("cannot delete global knowledge item (read-only)")
	}

	if err := f.store.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete knowledge item: %w", err)
	}

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// TRUST FEEDBACK IMPLEMENTATION
// ═══════════════════════════════════════════════════════════════════════════════

// RecordSuccess increments the success counter and recalculates trust score.
func (f *Fabric) RecordSuccess(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("knowledge item ID cannot be empty")
	}

	// Retrieve current item
	item, err := f.store.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to retrieve knowledge item: %w", err)
	}

	// Increment success count
	item.SuccessCount++

	// Recalculate trust score using Bayesian approach
	// trust = (successes + prior) / (total_attempts + prior * 2)
	// Prior = 2 (assume 2 successes, 2 failures initially)
	prior := 2.0
	totalAttempts := float64(item.SuccessCount + item.FailureCount)
	item.TrustScore = (float64(item.SuccessCount) + prior) / (totalAttempts + prior*2)

	// Update in store
	if err := f.store.UpdateTrustScore(ctx, id, item.SuccessCount, item.FailureCount); err != nil {
		return fmt.Errorf("failed to update trust score: %w", err)
	}

	return nil
}

// RecordFailure increments the failure counter and recalculates trust score.
func (f *Fabric) RecordFailure(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("knowledge item ID cannot be empty")
	}

	// Retrieve current item
	item, err := f.store.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to retrieve knowledge item: %w", err)
	}

	// Increment failure count
	item.FailureCount++

	// Recalculate trust score
	prior := 2.0
	totalAttempts := float64(item.SuccessCount + item.FailureCount)
	item.TrustScore = (float64(item.SuccessCount) + prior) / (totalAttempts + prior*2)

	// Update in store
	if err := f.store.UpdateTrustScore(ctx, id, item.SuccessCount, item.FailureCount); err != nil {
		return fmt.Errorf("failed to update trust score: %w", err)
	}

	return nil
}
