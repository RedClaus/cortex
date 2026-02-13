// Package memory provides memory management for CortexBrain.
// This file implements SQLite storage for the Observational Memory system.
package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// SQLITE OBSERVATIONAL STORE
// Implements ObservationalStore interface using SQLite for working memory
// ═══════════════════════════════════════════════════════════════════════════════

// SQLiteObservationalStore implements ObservationalStore using SQLite.
type SQLiteObservationalStore struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewSQLiteObservationalStore creates a new SQLite-backed observational store.
func NewSQLiteObservationalStore(db *sql.DB) *SQLiteObservationalStore {
	return &SQLiteObservationalStore{db: db}
}

// InitSchema creates the observational memory tables if they don't exist.
func (s *SQLiteObservationalStore) InitSchema(ctx context.Context) error {
	schema := `
		-- Messages (Tier 1: Working Memory)
		CREATE TABLE IF NOT EXISTS om_messages (
			id TEXT PRIMARY KEY,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			timestamp DATETIME NOT NULL,
			thread_id TEXT NOT NULL,
			resource_id TEXT NOT NULL,
			token_count INTEGER DEFAULT 0,
			compressed INTEGER DEFAULT 0,
			obs_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_om_messages_thread_resource
			ON om_messages(thread_id, resource_id);
		CREATE INDEX IF NOT EXISTS idx_om_messages_timestamp
			ON om_messages(timestamp);
		CREATE INDEX IF NOT EXISTS idx_om_messages_compressed
			ON om_messages(compressed);

		-- Observations (Tier 2: Compressed Memory)
		CREATE TABLE IF NOT EXISTS om_observations (
			id TEXT PRIMARY KEY,
			content TEXT NOT NULL,
			timestamp DATETIME NOT NULL,
			priority INTEGER DEFAULT 3,
			task_state TEXT,
			source_range TEXT, -- JSON array of message IDs
			thread_id TEXT NOT NULL,
			resource_id TEXT NOT NULL,
			token_count INTEGER DEFAULT 0,
			analyzed INTEGER DEFAULT 0,
			reflected INTEGER DEFAULT 0,
			ref_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_om_observations_resource
			ON om_observations(resource_id);
		CREATE INDEX IF NOT EXISTS idx_om_observations_timestamp
			ON om_observations(timestamp);
		CREATE INDEX IF NOT EXISTS idx_om_observations_priority
			ON om_observations(priority DESC);
		CREATE INDEX IF NOT EXISTS idx_om_observations_analyzed
			ON om_observations(analyzed);
		CREATE INDEX IF NOT EXISTS idx_om_observations_reflected
			ON om_observations(reflected);

		-- Reflections (Tier 3: High-Level Patterns)
		CREATE TABLE IF NOT EXISTS om_reflections (
			id TEXT PRIMARY KEY,
			content TEXT NOT NULL,
			timestamp DATETIME NOT NULL,
			pattern TEXT,
			source_obs TEXT, -- JSON array of observation IDs
			resource_id TEXT NOT NULL,
			token_count INTEGER DEFAULT 0,
			analyzed INTEGER DEFAULT 0,
			skill_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_om_reflections_resource
			ON om_reflections(resource_id);
		CREATE INDEX IF NOT EXISTS idx_om_reflections_timestamp
			ON om_reflections(timestamp);
		CREATE INDEX IF NOT EXISTS idx_om_reflections_pattern
			ON om_reflections(pattern);
		CREATE INDEX IF NOT EXISTS idx_om_reflections_analyzed
			ON om_reflections(analyzed);

		-- FTS5 virtual tables for semantic search
		CREATE VIRTUAL TABLE IF NOT EXISTS om_messages_fts USING fts5(
			content, thread_id, resource_id,
			content=om_messages, content_rowid=rowid
		);

		CREATE VIRTUAL TABLE IF NOT EXISTS om_observations_fts USING fts5(
			content, task_state, resource_id,
			content=om_observations, content_rowid=rowid
		);

		CREATE VIRTUAL TABLE IF NOT EXISTS om_reflections_fts USING fts5(
			content, pattern, resource_id,
			content=om_reflections, content_rowid=rowid
		);
	`

	_, err := s.db.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("init observational schema: %w", err)
	}

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// MESSAGE OPERATIONS (Tier 1)
// ═══════════════════════════════════════════════════════════════════════════════

// StoreMessage stores a new message.
func (s *SQLiteObservationalStore) StoreMessage(ctx context.Context, msg *Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
		INSERT INTO om_messages (
			id, role, content, timestamp, thread_id, resource_id,
			token_count, compressed, obs_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query,
		msg.ID, msg.Role, msg.Content, msg.Timestamp,
		msg.ThreadID, msg.ResourceID,
		msg.TokenCount, msg.Compressed, nullStr(msg.ObsID),
	)

	if err != nil {
		return fmt.Errorf("insert message: %w", err)
	}

	return nil
}

// GetMessages retrieves recent messages for a thread/resource.
func (s *SQLiteObservationalStore) GetMessages(ctx context.Context, threadID, resourceID string, limit int) ([]*Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT id, role, content, timestamp, thread_id, resource_id,
		       token_count, compressed, obs_id
		FROM om_messages
		WHERE resource_id = ?
		  AND (thread_id = ? OR ? = '')
		  AND compressed = 0
		ORDER BY timestamp ASC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, resourceID, threadID, threadID, limit)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	var messages []*Message
	for rows.Next() {
		msg, err := s.scanMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, rows.Err()
}

// GetMessageTokenCount returns total token count for uncompressed messages.
func (s *SQLiteObservationalStore) GetMessageTokenCount(ctx context.Context, threadID, resourceID string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT COALESCE(SUM(token_count), 0)
		FROM om_messages
		WHERE resource_id = ?
		  AND (thread_id = ? OR ? = '')
		  AND compressed = 0
	`

	var count int
	err := s.db.QueryRowContext(ctx, query, resourceID, threadID, threadID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("get message token count: %w", err)
	}

	return count, nil
}

// MarkMessagesCompressed marks messages as compressed into an observation.
func (s *SQLiteObservationalStore) MarkMessagesCompressed(ctx context.Context, messageIDs []string, obsID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(messageIDs) == 0 {
		return nil
	}

	// Build placeholders
	query := "UPDATE om_messages SET compressed = 1, obs_id = ? WHERE id IN ("
	args := []interface{}{obsID}
	for i, id := range messageIDs {
		if i > 0 {
			query += ","
		}
		query += "?"
		args = append(args, id)
	}
	query += ")"

	_, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("mark messages compressed: %w", err)
	}

	return nil
}

// DeleteMessages removes messages by ID.
func (s *SQLiteObservationalStore) DeleteMessages(ctx context.Context, messageIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(messageIDs) == 0 {
		return nil
	}

	query := "DELETE FROM om_messages WHERE id IN ("
	args := make([]interface{}, len(messageIDs))
	for i, id := range messageIDs {
		if i > 0 {
			query += ","
		}
		query += "?"
		args[i] = id
	}
	query += ")"

	_, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("delete messages: %w", err)
	}

	return nil
}

func (s *SQLiteObservationalStore) scanMessage(rows *sql.Rows) (*Message, error) {
	var msg Message
	var obsID sql.NullString

	err := rows.Scan(
		&msg.ID, &msg.Role, &msg.Content, &msg.Timestamp,
		&msg.ThreadID, &msg.ResourceID,
		&msg.TokenCount, &msg.Compressed, &obsID,
	)
	if err != nil {
		return nil, fmt.Errorf("scan message: %w", err)
	}

	if obsID.Valid {
		msg.ObsID = obsID.String
	}

	return &msg, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// OBSERVATION OPERATIONS (Tier 2)
// ═══════════════════════════════════════════════════════════════════════════════

// StoreObservation stores a new observation.
func (s *SQLiteObservationalStore) StoreObservation(ctx context.Context, obs *Observation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sourceRangeJSON, err := json.Marshal(obs.SourceRange)
	if err != nil {
		return fmt.Errorf("marshal source range: %w", err)
	}

	query := `
		INSERT INTO om_observations (
			id, content, timestamp, priority, task_state, source_range,
			thread_id, resource_id, token_count, analyzed, reflected, ref_id,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	_, err = s.db.ExecContext(ctx, query,
		obs.ID, obs.Content, obs.Timestamp, int(obs.Priority),
		obs.TaskState, string(sourceRangeJSON),
		obs.ThreadID, obs.ResourceID, obs.TokenCount,
		obs.Analyzed, obs.Reflected, nullStr(obs.RefID),
		now, now,
	)

	if err != nil {
		return fmt.Errorf("insert observation: %w", err)
	}

	return nil
}

// GetObservations retrieves observations for a resource.
func (s *SQLiteObservationalStore) GetObservations(ctx context.Context, resourceID string, limit int) ([]*Observation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}

	query := `
		SELECT id, content, timestamp, priority, task_state, source_range,
		       thread_id, resource_id, token_count, analyzed, reflected, ref_id,
		       created_at, updated_at
		FROM om_observations
		WHERE resource_id = ?
		  AND reflected = 0
		ORDER BY priority DESC, timestamp DESC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, resourceID, limit)
	if err != nil {
		return nil, fmt.Errorf("query observations: %w", err)
	}
	defer rows.Close()

	var observations []*Observation
	for rows.Next() {
		obs, err := s.scanObservation(rows)
		if err != nil {
			return nil, err
		}
		observations = append(observations, obs)
	}

	return observations, rows.Err()
}

// GetObservationTokenCount returns total token count for unreflected observations.
func (s *SQLiteObservationalStore) GetObservationTokenCount(ctx context.Context, resourceID string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT COALESCE(SUM(token_count), 0)
		FROM om_observations
		WHERE resource_id = ?
		  AND reflected = 0
	`

	var count int
	err := s.db.QueryRowContext(ctx, query, resourceID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("get observation token count: %w", err)
	}

	return count, nil
}

// GetUnanalyzedObservations returns observations not yet processed by distillation.
func (s *SQLiteObservationalStore) GetUnanalyzedObservations(ctx context.Context, resourceID string, minCount int) ([]*Observation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// First check if we have enough
	countQuery := `
		SELECT COUNT(*) FROM om_observations
		WHERE resource_id = ? AND analyzed = 0
	`
	var count int
	err := s.db.QueryRowContext(ctx, countQuery, resourceID).Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("count unanalyzed: %w", err)
	}

	if count < minCount {
		return nil, nil // Not enough yet
	}

	query := `
		SELECT id, content, timestamp, priority, task_state, source_range,
		       thread_id, resource_id, token_count, analyzed, reflected, ref_id,
		       created_at, updated_at
		FROM om_observations
		WHERE resource_id = ?
		  AND analyzed = 0
		ORDER BY timestamp ASC
	`

	rows, err := s.db.QueryContext(ctx, query, resourceID)
	if err != nil {
		return nil, fmt.Errorf("query unanalyzed observations: %w", err)
	}
	defer rows.Close()

	var observations []*Observation
	for rows.Next() {
		obs, err := s.scanObservation(rows)
		if err != nil {
			return nil, err
		}
		observations = append(observations, obs)
	}

	return observations, rows.Err()
}

// MarkObservationsAnalyzed marks observations as processed by distillation.
func (s *SQLiteObservationalStore) MarkObservationsAnalyzed(ctx context.Context, obsIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(obsIDs) == 0 {
		return nil
	}

	query := "UPDATE om_observations SET analyzed = 1, updated_at = ? WHERE id IN ("
	args := []interface{}{time.Now()}
	for i, id := range obsIDs {
		if i > 0 {
			query += ","
		}
		query += "?"
		args = append(args, id)
	}
	query += ")"

	_, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("mark observations analyzed: %w", err)
	}

	return nil
}

// MarkObservationsReflected marks observations as consolidated into a reflection.
func (s *SQLiteObservationalStore) MarkObservationsReflected(ctx context.Context, obsIDs []string, refID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(obsIDs) == 0 {
		return nil
	}

	query := "UPDATE om_observations SET reflected = 1, ref_id = ?, updated_at = ? WHERE id IN ("
	args := []interface{}{refID, time.Now()}
	for i, id := range obsIDs {
		if i > 0 {
			query += ","
		}
		query += "?"
		args = append(args, id)
	}
	query += ")"

	_, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("mark observations reflected: %w", err)
	}

	return nil
}

func (s *SQLiteObservationalStore) scanObservation(rows *sql.Rows) (*Observation, error) {
	var obs Observation
	var sourceRangeJSON string
	var refID sql.NullString
	var priority int

	err := rows.Scan(
		&obs.ID, &obs.Content, &obs.Timestamp, &priority,
		&obs.TaskState, &sourceRangeJSON,
		&obs.ThreadID, &obs.ResourceID, &obs.TokenCount,
		&obs.Analyzed, &obs.Reflected, &refID,
		&obs.CreatedAt, &obs.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan observation: %w", err)
	}

	obs.Priority = ObservationPriority(priority)

	if refID.Valid {
		obs.RefID = refID.String
	}

	if err := json.Unmarshal([]byte(sourceRangeJSON), &obs.SourceRange); err != nil {
		obs.SourceRange = nil
	}

	return &obs, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// REFLECTION OPERATIONS (Tier 3)
// ═══════════════════════════════════════════════════════════════════════════════

// StoreReflection stores a new reflection.
func (s *SQLiteObservationalStore) StoreReflection(ctx context.Context, ref *Reflection) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sourceObsJSON, err := json.Marshal(ref.SourceObs)
	if err != nil {
		return fmt.Errorf("marshal source obs: %w", err)
	}

	query := `
		INSERT INTO om_reflections (
			id, content, timestamp, pattern, source_obs, resource_id,
			token_count, analyzed, skill_id, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	_, err = s.db.ExecContext(ctx, query,
		ref.ID, ref.Content, ref.Timestamp, ref.Pattern,
		string(sourceObsJSON), ref.ResourceID, ref.TokenCount,
		ref.Analyzed, nullStr(ref.SkillID),
		now, now,
	)

	if err != nil {
		return fmt.Errorf("insert reflection: %w", err)
	}

	return nil
}

// GetReflections retrieves reflections for a resource.
func (s *SQLiteObservationalStore) GetReflections(ctx context.Context, resourceID string, limit int) ([]*Reflection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT id, content, timestamp, pattern, source_obs, resource_id,
		       token_count, analyzed, skill_id, created_at, updated_at
		FROM om_reflections
		WHERE resource_id = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, resourceID, limit)
	if err != nil {
		return nil, fmt.Errorf("query reflections: %w", err)
	}
	defer rows.Close()

	var reflections []*Reflection
	for rows.Next() {
		ref, err := s.scanReflection(rows)
		if err != nil {
			return nil, err
		}
		reflections = append(reflections, ref)
	}

	return reflections, rows.Err()
}

// GetUnanalyzedReflections returns reflections not yet distilled into skills.
func (s *SQLiteObservationalStore) GetUnanalyzedReflections(ctx context.Context, resourceID string, minCount int) ([]*Reflection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// First check if we have enough
	countQuery := `
		SELECT COUNT(*) FROM om_reflections
		WHERE resource_id = ? AND analyzed = 0
	`
	var count int
	err := s.db.QueryRowContext(ctx, countQuery, resourceID).Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("count unanalyzed reflections: %w", err)
	}

	if count < minCount {
		return nil, nil // Not enough yet
	}

	query := `
		SELECT id, content, timestamp, pattern, source_obs, resource_id,
		       token_count, analyzed, skill_id, created_at, updated_at
		FROM om_reflections
		WHERE resource_id = ?
		  AND analyzed = 0
		ORDER BY timestamp ASC
	`

	rows, err := s.db.QueryContext(ctx, query, resourceID)
	if err != nil {
		return nil, fmt.Errorf("query unanalyzed reflections: %w", err)
	}
	defer rows.Close()

	var reflections []*Reflection
	for rows.Next() {
		ref, err := s.scanReflection(rows)
		if err != nil {
			return nil, err
		}
		reflections = append(reflections, ref)
	}

	return reflections, rows.Err()
}

// MarkReflectionsAnalyzed marks reflections as distilled into skills.
func (s *SQLiteObservationalStore) MarkReflectionsAnalyzed(ctx context.Context, refIDs []string, skillID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(refIDs) == 0 {
		return nil
	}

	query := "UPDATE om_reflections SET analyzed = 1, skill_id = ?, updated_at = ? WHERE id IN ("
	args := []interface{}{nullStr(skillID), time.Now()}
	for i, id := range refIDs {
		if i > 0 {
			query += ","
		}
		query += "?"
		args = append(args, id)
	}
	query += ")"

	_, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("mark reflections analyzed: %w", err)
	}

	return nil
}

func (s *SQLiteObservationalStore) scanReflection(rows *sql.Rows) (*Reflection, error) {
	var ref Reflection
	var sourceObsJSON string
	var skillID sql.NullString

	err := rows.Scan(
		&ref.ID, &ref.Content, &ref.Timestamp, &ref.Pattern,
		&sourceObsJSON, &ref.ResourceID, &ref.TokenCount,
		&ref.Analyzed, &skillID,
		&ref.CreatedAt, &ref.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan reflection: %w", err)
	}

	if skillID.Valid {
		ref.SkillID = skillID.String
	}

	if err := json.Unmarshal([]byte(sourceObsJSON), &ref.SourceObs); err != nil {
		ref.SourceObs = nil
	}

	return &ref, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// SEARCH OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════════

// SearchMemory performs semantic search across all memory tiers.
func (s *SQLiteObservationalStore) SearchMemory(ctx context.Context, resourceID, query string, limit int) (*ObservationalContext, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	oc := &ObservationalContext{}

	// Search messages
	msgQuery := `
		SELECT m.id, m.role, m.content, m.timestamp, m.thread_id, m.resource_id,
		       m.token_count, m.compressed, m.obs_id
		FROM om_messages m
		JOIN om_messages_fts fts ON m.rowid = fts.rowid
		WHERE om_messages_fts MATCH ?
		  AND m.resource_id = ?
		LIMIT ?
	`
	msgRows, err := s.db.QueryContext(ctx, msgQuery, query, resourceID, limit)
	if err == nil {
		defer msgRows.Close()
		for msgRows.Next() {
			msg, err := s.scanMessage(msgRows)
			if err == nil {
				oc.Messages = append(oc.Messages, msg)
				oc.MessageTokens += msg.TokenCount
			}
		}
	}

	// Search observations
	obsQuery := `
		SELECT o.id, o.content, o.timestamp, o.priority, o.task_state, o.source_range,
		       o.thread_id, o.resource_id, o.token_count, o.analyzed, o.reflected, o.ref_id,
		       o.created_at, o.updated_at
		FROM om_observations o
		JOIN om_observations_fts fts ON o.rowid = fts.rowid
		WHERE om_observations_fts MATCH ?
		  AND o.resource_id = ?
		LIMIT ?
	`
	obsRows, err := s.db.QueryContext(ctx, obsQuery, query, resourceID, limit)
	if err == nil {
		defer obsRows.Close()
		for obsRows.Next() {
			obs, err := s.scanObservation(obsRows)
			if err == nil {
				oc.Observations = append(oc.Observations, obs)
				oc.ObservationTokens += obs.TokenCount
			}
		}
	}

	// Search reflections
	refQuery := `
		SELECT r.id, r.content, r.timestamp, r.pattern, r.source_obs, r.resource_id,
		       r.token_count, r.analyzed, r.skill_id, r.created_at, r.updated_at
		FROM om_reflections r
		JOIN om_reflections_fts fts ON r.rowid = fts.rowid
		WHERE om_reflections_fts MATCH ?
		  AND r.resource_id = ?
		LIMIT ?
	`
	refRows, err := s.db.QueryContext(ctx, refQuery, query, resourceID, limit)
	if err == nil {
		defer refRows.Close()
		for refRows.Next() {
			ref, err := s.scanReflection(refRows)
			if err == nil {
				oc.Reflections = append(oc.Reflections, ref)
				oc.ReflectionTokens += ref.TokenCount
			}
		}
	}

	oc.TotalTokens = oc.MessageTokens + oc.ObservationTokens + oc.ReflectionTokens
	return oc, nil
}

// GetTimeline returns memory at a specific point in time.
func (s *SQLiteObservationalStore) GetTimeline(ctx context.Context, resourceID string, from, to time.Time) (*ObservationalContext, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	oc := &ObservationalContext{}

	// Get messages in range
	msgQuery := `
		SELECT id, role, content, timestamp, thread_id, resource_id,
		       token_count, compressed, obs_id
		FROM om_messages
		WHERE resource_id = ?
		  AND timestamp >= ?
		  AND timestamp <= ?
		ORDER BY timestamp ASC
	`
	msgRows, err := s.db.QueryContext(ctx, msgQuery, resourceID, from, to)
	if err == nil {
		defer msgRows.Close()
		for msgRows.Next() {
			msg, err := s.scanMessage(msgRows)
			if err == nil {
				oc.Messages = append(oc.Messages, msg)
				oc.MessageTokens += msg.TokenCount
			}
		}
	}

	// Get observations in range
	obsQuery := `
		SELECT id, content, timestamp, priority, task_state, source_range,
		       thread_id, resource_id, token_count, analyzed, reflected, ref_id,
		       created_at, updated_at
		FROM om_observations
		WHERE resource_id = ?
		  AND timestamp >= ?
		  AND timestamp <= ?
		ORDER BY timestamp ASC
	`
	obsRows, err := s.db.QueryContext(ctx, obsQuery, resourceID, from, to)
	if err == nil {
		defer obsRows.Close()
		for obsRows.Next() {
			obs, err := s.scanObservation(obsRows)
			if err == nil {
				oc.Observations = append(oc.Observations, obs)
				oc.ObservationTokens += obs.TokenCount
			}
		}
	}

	// Get reflections in range
	refQuery := `
		SELECT id, content, timestamp, pattern, source_obs, resource_id,
		       token_count, analyzed, skill_id, created_at, updated_at
		FROM om_reflections
		WHERE resource_id = ?
		  AND timestamp >= ?
		  AND timestamp <= ?
		ORDER BY timestamp ASC
	`
	refRows, err := s.db.QueryContext(ctx, refQuery, resourceID, from, to)
	if err == nil {
		defer refRows.Close()
		for refRows.Next() {
			ref, err := s.scanReflection(refRows)
			if err == nil {
				oc.Reflections = append(oc.Reflections, ref)
				oc.ReflectionTokens += ref.TokenCount
			}
		}
	}

	oc.TotalTokens = oc.MessageTokens + oc.ObservationTokens + oc.ReflectionTokens
	return oc, nil
}

// ExportMemory exports all memory for a resource.
// Returns path to exported file (placeholder for Memvid integration).
func (s *SQLiteObservationalStore) ExportMemory(ctx context.Context, resourceID string) (string, error) {
	// This is a placeholder - actual Memvid export will be implemented
	// in a separate memvid_store.go when that integration is built
	return "", fmt.Errorf("memvid export not yet implemented")
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPER FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════════

// nullStr converts string to sql.NullString.
func nullStr(s string) sql.NullString {
	return sql.NullString{
		String: s,
		Valid:  s != "",
	}
}
