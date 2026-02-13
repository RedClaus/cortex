// Package memory implements Pinky's memory system with temporal awareness.
package memory

import (
	"context"
	"time"

	"github.com/normanking/pinky/internal/brain"
)

// StoreAdapter adapts SQLiteStore to implement brain.MemoryStore.
type StoreAdapter struct {
	store *SQLiteStore
}

// NewStoreAdapter creates a new adapter wrapping an SQLiteStore.
func NewStoreAdapter(store *SQLiteStore) *StoreAdapter {
	return &StoreAdapter{store: store}
}

// Store saves a memory to the store.
func (a *StoreAdapter) Store(ctx context.Context, mem *brain.Memory) error {
	return a.store.Store(ctx, mem)
}

// Recall retrieves memories matching a query with temporal awareness.
func (a *StoreAdapter) Recall(ctx context.Context, query string, opts brain.MemoryRecallOptions) ([]brain.Memory, error) {
	// Convert brain.MemoryRecallOptions to memory.RecallOptions
	recallOpts := RecallOptions{
		UserID:        opts.UserID,
		Limit:         opts.Limit,
		MinImportance: opts.MinImportance,
		Types:         opts.Types,
		Since:         opts.Since,
		Until:         opts.Until,
	}

	// Handle temporal context if provided
	if opts.TimeContext != nil {
		if tc, ok := opts.TimeContext.(*TemporalContext); ok {
			recallOpts.TimeContext = tc
		}
	}

	// Parse temporal context from the query if not explicitly provided
	if recallOpts.TimeContext == nil {
		recallOpts.TimeContext = ParseTemporalContext(query, time.Now())
	}

	return a.store.Recall(ctx, query, recallOpts)
}

// GetRecent retrieves the most recent memories for a user.
func (a *StoreAdapter) GetRecent(ctx context.Context, userID string, limit int) ([]brain.Memory, error) {
	return a.store.GetRecent(ctx, userID, limit)
}

// Close closes the underlying store.
func (a *StoreAdapter) Close() error {
	return a.store.Close()
}
