// Package memory provides enhanced memory capabilities for Cortex.
// This file implements query activation logging for CR-025-LITE.
package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ActivationLog records when and why memories were retrieved.
// This provides a complete audit trail for debugging and learning analysis.
type ActivationLog struct {
	ID            string    `json:"id"`
	QueryID       string    `json:"query_id"`
	QueryText     string    `json:"query_text"`      // Original query (truncated)
	MemoriesFound []string  `json:"memories_found"`  // IDs of retrieved memories
	RetrievalType string    `json:"retrieval_type"`  // "similarity", "fts", "category", "tier"
	LatencyMs     int64     `json:"latency_ms"`
	TokensUsed    int       `json:"tokens_used"`     // Context tokens consumed by these memories
	Lane          string    `json:"lane"`            // "fast" or "smart"
	CreatedAt     time.Time `json:"created_at"`
	SessionID     string    `json:"session_id"`
}

// ActivationStats aggregates activation statistics.
type ActivationStats struct {
	TotalQueries      int     `json:"total_queries"`
	AvgLatencyMs      float64 `json:"avg_latency_ms"`
	TotalTokensUsed   int     `json:"total_tokens_used"`
	FastLaneCount     int     `json:"fast_lane_count"`
	SmartLaneCount    int     `json:"smart_lane_count"`
	AvgMemoriesPerQuery float64 `json:"avg_memories_per_query"`
	// Retrieval type breakdown
	SimilarityCount int `json:"similarity_count"`
	FTSCount        int `json:"fts_count"`
	CategoryCount   int `json:"category_count"`
	TierCount       int `json:"tier_count"`
}

// LogActivation records a retrieval event.
func (s *StrategicMemoryStore) LogActivation(ctx context.Context, log *ActivationLog) error {
	if log.ID == "" {
		log.ID = "act_" + uuid.New().String()
	}
	if log.QueryID == "" {
		log.QueryID = uuid.New().String()
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}

	memoriesJSON, err := json.Marshal(log.MemoriesFound)
	if err != nil {
		memoriesJSON = []byte("[]")
	}

	query := `
		INSERT INTO activation_logs (id, query_id, query_text, memories_found, retrieval_type,
		                             latency_ms, tokens_used, lane, created_at, session_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err = s.db.ExecContext(ctx, query,
		log.ID, log.QueryID, truncateString(log.QueryText, 200), string(memoriesJSON),
		log.RetrievalType, log.LatencyMs, log.TokensUsed, log.Lane,
		log.CreatedAt.Format(time.RFC3339), log.SessionID)
	if err != nil {
		return fmt.Errorf("log activation: %w", err)
	}
	return nil
}

// GetRecentActivations returns recent retrieval events for analysis.
func (s *StrategicMemoryStore) GetRecentActivations(ctx context.Context, limit int) ([]ActivationLog, error) {
	query := `
		SELECT id, query_id, query_text, memories_found, retrieval_type,
		       latency_ms, tokens_used, lane, created_at, session_id
		FROM activation_logs
		ORDER BY created_at DESC
		LIMIT ?
	`
	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("get recent activations: %w", err)
	}
	defer rows.Close()

	return scanActivationLogs(rows)
}

// GetActivationsForSession returns activations for a specific session.
func (s *StrategicMemoryStore) GetActivationsForSession(ctx context.Context, sessionID string) ([]ActivationLog, error) {
	query := `
		SELECT id, query_id, query_text, memories_found, retrieval_type,
		       latency_ms, tokens_used, lane, created_at, session_id
		FROM activation_logs
		WHERE session_id = ?
		ORDER BY created_at ASC
	`
	rows, err := s.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get activations for session: %w", err)
	}
	defer rows.Close()

	return scanActivationLogs(rows)
}

// GetActivationStats returns aggregate statistics for a time period.
func (s *StrategicMemoryStore) GetActivationStats(ctx context.Context, since time.Time) (*ActivationStats, error) {
	query := `
		SELECT
			COUNT(*) as total_queries,
			COALESCE(AVG(latency_ms), 0) as avg_latency,
			COALESCE(SUM(tokens_used), 0) as total_tokens,
			SUM(CASE WHEN lane = 'fast' THEN 1 ELSE 0 END) as fast_lane_count,
			SUM(CASE WHEN lane = 'smart' THEN 1 ELSE 0 END) as smart_lane_count,
			SUM(CASE WHEN retrieval_type = 'similarity' THEN 1 ELSE 0 END) as similarity_count,
			SUM(CASE WHEN retrieval_type = 'fts' THEN 1 ELSE 0 END) as fts_count,
			SUM(CASE WHEN retrieval_type = 'category' THEN 1 ELSE 0 END) as category_count,
			SUM(CASE WHEN retrieval_type = 'tier' THEN 1 ELSE 0 END) as tier_count
		FROM activation_logs
		WHERE created_at >= ?
	`

	row := s.db.QueryRowContext(ctx, query, since.Format(time.RFC3339))

	stats := &ActivationStats{}
	err := row.Scan(&stats.TotalQueries, &stats.AvgLatencyMs, &stats.TotalTokensUsed,
		&stats.FastLaneCount, &stats.SmartLaneCount,
		&stats.SimilarityCount, &stats.FTSCount, &stats.CategoryCount, &stats.TierCount)
	if err != nil {
		return nil, fmt.Errorf("get activation stats: %w", err)
	}

	// Calculate average memories per query
	if stats.TotalQueries > 0 {
		var avgMemories float64
		avgQuery := `
			SELECT AVG(json_array_length(memories_found))
			FROM activation_logs
			WHERE created_at >= ?
		`
		s.db.QueryRowContext(ctx, avgQuery, since.Format(time.RFC3339)).Scan(&avgMemories)
		stats.AvgMemoriesPerQuery = avgMemories
	}

	return stats, nil
}

// GetSlowActivations returns activations that exceeded a latency threshold.
func (s *StrategicMemoryStore) GetSlowActivations(ctx context.Context, thresholdMs int64, limit int) ([]ActivationLog, error) {
	query := `
		SELECT id, query_id, query_text, memories_found, retrieval_type,
		       latency_ms, tokens_used, lane, created_at, session_id
		FROM activation_logs
		WHERE latency_ms > ?
		ORDER BY latency_ms DESC
		LIMIT ?
	`
	rows, err := s.db.QueryContext(ctx, query, thresholdMs, limit)
	if err != nil {
		return nil, fmt.Errorf("get slow activations: %w", err)
	}
	defer rows.Close()

	return scanActivationLogs(rows)
}

// GetMemoryAccessFrequency returns how often each memory is accessed.
func (s *StrategicMemoryStore) GetMemoryAccessFrequency(ctx context.Context, since time.Time, limit int) (map[string]int, error) {
	query := `
		SELECT memories_found
		FROM activation_logs
		WHERE created_at >= ?
	`
	rows, err := s.db.QueryContext(ctx, query, since.Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("get memory access frequency: %w", err)
	}
	defer rows.Close()

	frequency := make(map[string]int)
	for rows.Next() {
		var memoriesJSON string
		if err := rows.Scan(&memoriesJSON); err != nil {
			continue
		}
		var memoryIDs []string
		if err := json.Unmarshal([]byte(memoriesJSON), &memoryIDs); err != nil {
			continue
		}
		for _, id := range memoryIDs {
			frequency[id]++
		}
	}

	return frequency, rows.Err()
}

// CleanupOldActivations removes activation logs older than a duration.
func (s *StrategicMemoryStore) CleanupOldActivations(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan).Format(time.RFC3339)
	query := `DELETE FROM activation_logs WHERE created_at < ?`

	result, err := s.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("cleanup old activations: %w", err)
	}

	return result.RowsAffected()
}

// scanActivationLogs scans rows into ActivationLog structs.
func scanActivationLogs(rows *sql.Rows) ([]ActivationLog, error) {
	var logs []ActivationLog
	for rows.Next() {
		var log ActivationLog
		var memoriesJSON, createdAt string
		err := rows.Scan(&log.ID, &log.QueryID, &log.QueryText, &memoriesJSON,
			&log.RetrievalType, &log.LatencyMs, &log.TokensUsed, &log.Lane,
			&createdAt, &log.SessionID)
		if err != nil {
			return nil, fmt.Errorf("scan activation log: %w", err)
		}
		if err := json.Unmarshal([]byte(memoriesJSON), &log.MemoriesFound); err != nil {
			log.MemoriesFound = []string{}
		}
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			log.CreatedAt = t
		}
		logs = append(logs, log)
	}
	return logs, rows.Err()
}
