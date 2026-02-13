// Package knowledge provides the Knowledge Fabric - a three-tier knowledge retrieval
// and management system for Cortex AI assistant.
package knowledge

import (
	"context"

	"github.com/normanking/cortex/pkg/types"
)

// Store defines the data layer interface for knowledge persistence.
// This interface will be implemented by the SQLite data layer.
type Store interface {
	// Create inserts a new knowledge item.
	Create(ctx context.Context, item *types.KnowledgeItem) error

	// Update modifies an existing knowledge item.
	Update(ctx context.Context, item *types.KnowledgeItem) error

	// Delete soft-deletes a knowledge item by setting DeletedAt.
	Delete(ctx context.Context, id string) error

	// GetByID retrieves a single knowledge item by its ID.
	GetByID(ctx context.Context, id string) (*types.KnowledgeItem, error)

	// GetByScope retrieves all knowledge items for a specific tier (personal, team, global).
	GetByScope(ctx context.Context, scope types.Scope) ([]*types.KnowledgeItem, error)

	// SearchByTags performs an exact tag match search within specified tiers.
	// Returns items that contain ALL specified tags.
	SearchByTags(ctx context.Context, tags []string, scopes []types.Scope) ([]*types.KnowledgeItem, error)

	// IncrementAccessCount updates the access counter for a knowledge item.
	IncrementAccessCount(ctx context.Context, id string) error

	// UpdateTrustScore recalculates and updates the trust score based on success/failure counts.
	UpdateTrustScore(ctx context.Context, id string, successCount, failureCount int) error
}

// Searcher defines the full-text search interface using SQLite's FTS5.
// This handles fuzzy/semantic search beyond exact tag matching.
type Searcher interface {
	// Search performs a full-text search across title, content, and tags.
	// Returns items ranked by relevance score, filtered by the provided options.
	Search(ctx context.Context, query string, opts types.SearchOptions) ([]*ScoredItem, error)

	// Index rebuilds the FTS5 index for all knowledge items.
	// This is typically called after bulk imports or database migrations.
	Index(ctx context.Context) error
}

// ScoredItem wraps a KnowledgeItem with its search relevance score.
type ScoredItem struct {
	Item      *types.KnowledgeItem `json:"item"`
	Relevance float64              `json:"relevance"` // FTS5 rank score (0.0 - 1.0)
}

// MergeStrategy defines how conflicts are resolved when synchronizing knowledge items.
type MergeStrategy interface {
	// Resolve determines the winner when a local and remote version of the same
	// knowledge item diverge.
	Resolve(ctx context.Context, local, remote *types.KnowledgeItem) (*types.MergeResult, error)
}
