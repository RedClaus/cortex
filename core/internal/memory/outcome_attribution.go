// Package memory provides enhanced memory capabilities for Cortex.
// This file implements outcome attribution for CR-025-LITE.
package memory

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// OutcomeAttribution links a strategic memory to a query outcome.
// This enables bidirectional tracing: which memories helped which queries,
// and which queries used which memories.
type OutcomeAttribution struct {
	ID           string    `json:"id"`
	MemoryID     string    `json:"memory_id"`     // The strategic memory that was used
	QueryID      string    `json:"query_id"`      // The query that used it
	QueryText    string    `json:"query_text"`    // Original query (truncated for storage)
	Outcome      string    `json:"outcome"`       // "success", "failure", "partial"
	Contribution float64   `json:"contribution"`  // How much this memory contributed (0-1)
	CreatedAt    time.Time `json:"created_at"`
	SessionID    string    `json:"session_id"`
}

// MemoryImpact aggregates the impact of a memory across all its attributions.
type MemoryImpact struct {
	MemoryID        string  `json:"memory_id"`
	TotalUses       int     `json:"total_uses"`
	Successes       int     `json:"successes"`
	Failures        int     `json:"failures"`
	Partials        int     `json:"partials"`
	AvgContribution float64 `json:"avg_contribution"`
	SuccessRate     float64 `json:"success_rate"`
}

// RecordAttribution links a memory to a query outcome.
func (s *StrategicMemoryStore) RecordAttribution(ctx context.Context, attr *OutcomeAttribution) error {
	if attr.ID == "" {
		attr.ID = "attr_" + uuid.New().String()
	}
	if attr.CreatedAt.IsZero() {
		attr.CreatedAt = time.Now()
	}

	query := `
		INSERT INTO memory_attributions (id, memory_id, query_id, query_text, outcome, contribution, created_at, session_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.ExecContext(ctx, query,
		attr.ID, attr.MemoryID, attr.QueryID, truncateString(attr.QueryText, 200),
		attr.Outcome, attr.Contribution, attr.CreatedAt.Format(time.RFC3339), attr.SessionID)
	if err != nil {
		return fmt.Errorf("record attribution: %w", err)
	}
	return nil
}

// RecordAttributions records multiple attributions in a batch.
func (s *StrategicMemoryStore) RecordAttributions(ctx context.Context, queryID string, queryText string, memoryIDs []string, outcome string, sessionID string) error {
	if len(memoryIDs) == 0 {
		return nil
	}

	contribution := 1.0 / float64(len(memoryIDs)) // Equal distribution

	for _, memID := range memoryIDs {
		attr := &OutcomeAttribution{
			MemoryID:     memID,
			QueryID:      queryID,
			QueryText:    queryText,
			Outcome:      outcome,
			Contribution: contribution,
			SessionID:    sessionID,
		}
		if err := s.RecordAttribution(ctx, attr); err != nil {
			return err
		}
	}
	return nil
}

// GetAttributionsForMemory returns all queries that used a specific memory.
func (s *StrategicMemoryStore) GetAttributionsForMemory(ctx context.Context, memoryID string, limit int) ([]OutcomeAttribution, error) {
	query := `
		SELECT id, memory_id, query_id, query_text, outcome, contribution, created_at, session_id
		FROM memory_attributions
		WHERE memory_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`
	rows, err := s.db.QueryContext(ctx, query, memoryID, limit)
	if err != nil {
		return nil, fmt.Errorf("get attributions for memory: %w", err)
	}
	defer rows.Close()

	return scanOutcomeAttributions(rows)
}

// GetAttributionsForQuery returns all memories used by a specific query.
func (s *StrategicMemoryStore) GetAttributionsForQuery(ctx context.Context, queryID string) ([]OutcomeAttribution, error) {
	query := `
		SELECT id, memory_id, query_id, query_text, outcome, contribution, created_at, session_id
		FROM memory_attributions
		WHERE query_id = ?
		ORDER BY contribution DESC
	`
	rows, err := s.db.QueryContext(ctx, query, queryID)
	if err != nil {
		return nil, fmt.Errorf("get attributions for query: %w", err)
	}
	defer rows.Close()

	return scanOutcomeAttributions(rows)
}

// CalculateMemoryImpact analyzes the overall impact of a memory.
func (s *StrategicMemoryStore) CalculateMemoryImpact(ctx context.Context, memoryID string) (*MemoryImpact, error) {
	query := `
		SELECT
			COUNT(*) as total_uses,
			SUM(CASE WHEN outcome = 'success' THEN 1 ELSE 0 END) as successes,
			SUM(CASE WHEN outcome = 'failure' THEN 1 ELSE 0 END) as failures,
			SUM(CASE WHEN outcome = 'partial' THEN 1 ELSE 0 END) as partials,
			COALESCE(AVG(contribution), 0) as avg_contribution
		FROM memory_attributions
		WHERE memory_id = ?
	`
	row := s.db.QueryRowContext(ctx, query, memoryID)

	impact := &MemoryImpact{MemoryID: memoryID}
	err := row.Scan(&impact.TotalUses, &impact.Successes, &impact.Failures, &impact.Partials, &impact.AvgContribution)
	if err != nil {
		return nil, fmt.Errorf("calculate memory impact: %w", err)
	}

	if impact.TotalUses > 0 {
		impact.SuccessRate = float64(impact.Successes) / float64(impact.TotalUses)
	}

	return impact, nil
}

// GetRecentlyUsedMemories returns memories that were used in recent attributions.
func (s *StrategicMemoryStore) GetRecentlyUsedMemories(ctx context.Context, since time.Duration, limit int) ([]StrategicMemory, error) {
	sinceTime := time.Now().Add(-since).Format(time.RFC3339)

	query := `
		SELECT DISTINCT sm.id, sm.principle, sm.category, sm.trigger_pattern, sm.tier,
		       sm.success_count, sm.failure_count, sm.apply_count,
		       sm.success_rate, sm.confidence, sm.source_sessions, sm.embedding,
		       sm.created_at, sm.updated_at, sm.last_applied_at,
		       COALESCE(sm.version, 1), COALESCE(sm.parent_id, ''), COALESCE(sm.evolution_chain, '[]')
		FROM strategic_memory sm
		JOIN memory_attributions ma ON sm.id = ma.memory_id
		WHERE ma.created_at >= ?
		ORDER BY ma.created_at DESC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, sinceTime, limit)
	if err != nil {
		return nil, fmt.Errorf("get recently used memories: %w", err)
	}
	defer rows.Close()

	return scanStrategicMemories(rows)
}

// GetMostImpactfulMemories returns memories ranked by their impact.
func (s *StrategicMemoryStore) GetMostImpactfulMemories(ctx context.Context, minUses int, limit int) ([]MemoryImpact, error) {
	query := `
		SELECT
			memory_id,
			COUNT(*) as total_uses,
			SUM(CASE WHEN outcome = 'success' THEN 1 ELSE 0 END) as successes,
			SUM(CASE WHEN outcome = 'failure' THEN 1 ELSE 0 END) as failures,
			SUM(CASE WHEN outcome = 'partial' THEN 1 ELSE 0 END) as partials,
			COALESCE(AVG(contribution), 0) as avg_contribution
		FROM memory_attributions
		GROUP BY memory_id
		HAVING total_uses >= ?
		ORDER BY (CAST(successes AS REAL) / total_uses) DESC, total_uses DESC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, minUses, limit)
	if err != nil {
		return nil, fmt.Errorf("get most impactful memories: %w", err)
	}
	defer rows.Close()

	var impacts []MemoryImpact
	for rows.Next() {
		var impact MemoryImpact
		err := rows.Scan(&impact.MemoryID, &impact.TotalUses, &impact.Successes, &impact.Failures, &impact.Partials, &impact.AvgContribution)
		if err != nil {
			return nil, fmt.Errorf("scan memory impact: %w", err)
		}
		if impact.TotalUses > 0 {
			impact.SuccessRate = float64(impact.Successes) / float64(impact.TotalUses)
		}
		impacts = append(impacts, impact)
	}

	return impacts, rows.Err()
}

// SyncFromAttributions updates a memory's success/failure counts from its attributions.
// This is useful during sleep cycle consolidation.
func (s *StrategicMemoryStore) SyncFromAttributions(ctx context.Context, memoryID string) error {
	impact, err := s.CalculateMemoryImpact(ctx, memoryID)
	if err != nil {
		return err
	}

	if impact.TotalUses == 0 {
		return nil // No attributions to sync from
	}

	query := `
		UPDATE strategic_memory
		SET success_count = ?,
		    failure_count = ?,
		    apply_count = ?,
		    updated_at = ?
		WHERE id = ?
	`
	_, err = s.db.ExecContext(ctx, query,
		impact.Successes, impact.Failures, impact.TotalUses,
		time.Now().Format(time.RFC3339), memoryID)
	if err != nil {
		return fmt.Errorf("sync from attributions: %w", err)
	}

	return nil
}

// scanOutcomeAttributions scans rows into OutcomeAttribution structs.
func scanOutcomeAttributions(rows *sql.Rows) ([]OutcomeAttribution, error) {
	var attrs []OutcomeAttribution
	for rows.Next() {
		var attr OutcomeAttribution
		var createdAt string
		err := rows.Scan(&attr.ID, &attr.MemoryID, &attr.QueryID, &attr.QueryText,
			&attr.Outcome, &attr.Contribution, &createdAt, &attr.SessionID)
		if err != nil {
			return nil, fmt.Errorf("scan outcome attribution: %w", err)
		}
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			attr.CreatedAt = t
		}
		attrs = append(attrs, attr)
	}
	return attrs, rows.Err()
}

// truncateString is defined in clustering.go
