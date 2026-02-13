// Package memory implements Pinky's memory system with temporal awareness.
package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/normanking/pinky/internal/brain"

	_ "modernc.org/sqlite"
)

// SQLiteStore implements Store using SQLite for persistence.
type SQLiteStore struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewSQLiteStore creates a new SQLite-backed memory store.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return store, nil
}

// migrate creates the necessary tables if they don't exist.
func (s *SQLiteStore) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS memories (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		type TEXT NOT NULL,
		content TEXT NOT NULL,
		embedding BLOB,
		importance REAL DEFAULT 0.5,
		source TEXT,
		context TEXT,
		temporal_tags TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		accessed_at TIMESTAMP,
		access_count INTEGER DEFAULT 0
	);

	CREATE INDEX IF NOT EXISTS idx_memories_user_id ON memories(user_id);
	CREATE INDEX IF NOT EXISTS idx_memories_created_at ON memories(created_at);
	CREATE INDEX IF NOT EXISTS idx_memories_type ON memories(type);
	CREATE INDEX IF NOT EXISTS idx_memories_importance ON memories(importance);
	`

	_, err := s.db.Exec(schema)
	return err
}

// Store saves a memory to the database.
func (s *SQLiteStore) Store(ctx context.Context, mem *brain.Memory) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Serialize context and temporal tags as JSON
	contextJSON, err := json.Marshal(mem.Context)
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}

	temporalJSON, err := json.Marshal(mem.TemporalTags)
	if err != nil {
		return fmt.Errorf("failed to marshal temporal tags: %w", err)
	}

	// Serialize embedding as JSON
	var embeddingJSON []byte
	if len(mem.Embedding) > 0 {
		embeddingJSON, err = json.Marshal(mem.Embedding)
		if err != nil {
			return fmt.Errorf("failed to marshal embedding: %w", err)
		}
	}

	query := `
		INSERT INTO memories (id, user_id, type, content, embedding, importance, source, context, temporal_tags, created_at, accessed_at, access_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			content = excluded.content,
			importance = excluded.importance,
			context = excluded.context,
			temporal_tags = excluded.temporal_tags,
			accessed_at = excluded.accessed_at,
			access_count = excluded.access_count
	`

	now := time.Now()
	if mem.CreatedAt.IsZero() {
		mem.CreatedAt = now
	}
	if mem.AccessedAt.IsZero() {
		mem.AccessedAt = now
	}

	_, err = s.db.ExecContext(ctx, query,
		mem.ID,
		mem.UserID,
		string(mem.Type),
		mem.Content,
		embeddingJSON,
		mem.Importance,
		mem.Source,
		string(contextJSON),
		string(temporalJSON),
		mem.CreatedAt,
		mem.AccessedAt,
		mem.AccessCount,
	)

	return err
}

// Recall retrieves memories matching a query with temporal awareness.
func (s *SQLiteStore) Recall(ctx context.Context, query string, opts RecallOptions) ([]brain.Memory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Build the SQL query
	sqlQuery := `SELECT id, user_id, type, content, embedding, importance, source, context, temporal_tags, created_at, accessed_at, access_count FROM memories WHERE 1=1`
	args := make([]any, 0)

	if opts.UserID != "" {
		sqlQuery += " AND user_id = ?"
		args = append(args, opts.UserID)
	}

	if opts.MinImportance > 0 {
		sqlQuery += " AND importance >= ?"
		args = append(args, opts.MinImportance)
	}

	if len(opts.Types) > 0 {
		placeholders := make([]string, len(opts.Types))
		for i, t := range opts.Types {
			placeholders[i] = "?"
			args = append(args, string(t))
		}
		sqlQuery += " AND type IN (" + strings.Join(placeholders, ",") + ")"
	}

	if !opts.Since.IsZero() {
		sqlQuery += " AND created_at >= ?"
		args = append(args, opts.Since)
	}

	if !opts.Until.IsZero() {
		sqlQuery += " AND created_at <= ?"
		args = append(args, opts.Until)
	}

	sqlQuery += " ORDER BY created_at DESC"

	// Fetch more than limit to allow for scoring and reranking
	fetchLimit := opts.Limit * 3
	if fetchLimit < 50 {
		fetchLimit = 50
	}
	sqlQuery += fmt.Sprintf(" LIMIT %d", fetchLimit)

	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	memories := make([]brain.Memory, 0)
	for rows.Next() {
		mem, err := s.scanMemory(rows)
		if err != nil {
			return nil, err
		}
		memories = append(memories, *mem)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Score and rank memories
	scored := make([]ScoredMemory, len(memories))
	for i, mem := range memories {
		scored[i] = ScoredMemory{
			Memory: mem,
			Score:  ScoreMemory(&mem, query, opts.TimeContext),
		}
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	// Return top N
	limit := opts.Limit
	if limit <= 0 {
		limit = 10
	}
	if len(scored) > limit {
		scored = scored[:limit]
	}

	result := make([]brain.Memory, len(scored))
	for i, sm := range scored {
		result[i] = sm.Memory
		// Update access time and count
		s.updateAccessStats(ctx, sm.Memory.ID)
	}

	return result, nil
}

// GetRecent retrieves the most recent memories for a user.
func (s *SQLiteStore) GetRecent(ctx context.Context, userID string, limit int) ([]brain.Memory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, user_id, type, content, embedding, importance, source, context, temporal_tags, created_at, accessed_at, access_count
		FROM memories
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT ?`

	rows, err := s.db.QueryContext(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	memories := make([]brain.Memory, 0)
	for rows.Next() {
		mem, err := s.scanMemory(rows)
		if err != nil {
			return nil, err
		}
		memories = append(memories, *mem)
	}

	return memories, rows.Err()
}

// TemporalSearch searches memories with time-aware filtering.
func (s *SQLiteStore) TemporalSearch(ctx context.Context, userID string, temporal *TemporalContext) ([]brain.Memory, error) {
	if temporal == nil || !temporal.HasTimeReference {
		return s.GetRecent(ctx, userID, 10)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Calculate time window around the target time
	targetTime := temporal.AbsoluteTime
	var startTime, endTime time.Time

	// For time ranges, use the range directly
	if temporal.TimeRange != nil {
		startTime = temporal.TimeRange.Start
		endTime = temporal.TimeRange.End
	} else {
		// Default to a window around the target time
		// Start of day for the target time
		startTime = time.Date(targetTime.Year(), targetTime.Month(), targetTime.Day(), 0, 0, 0, 0, targetTime.Location())
		endTime = startTime.Add(24 * time.Hour)
	}

	query := `SELECT id, user_id, type, content, embedding, importance, source, context, temporal_tags, created_at, accessed_at, access_count
		FROM memories
		WHERE user_id = ? AND created_at >= ? AND created_at < ?
		ORDER BY created_at DESC
		LIMIT 50`

	rows, err := s.db.QueryContext(ctx, query, userID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	memories := make([]brain.Memory, 0)
	for rows.Next() {
		mem, err := s.scanMemory(rows)
		if err != nil {
			return nil, err
		}
		memories = append(memories, *mem)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Score and rank by temporal relevance
	scored := make([]ScoredMemory, len(memories))
	for i, mem := range memories {
		scored[i] = ScoredMemory{
			Memory: mem,
			Score:  TemporalDistance(mem.CreatedAt, temporal),
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	if len(scored) > 10 {
		scored = scored[:10]
	}

	result := make([]brain.Memory, len(scored))
	for i, sm := range scored {
		result[i] = sm.Memory
	}

	return result, nil
}

// SemanticSearch searches memories using embedding similarity with cosine distance.
func (s *SQLiteStore) SemanticSearch(ctx context.Context, embedding []float64, limit int) ([]brain.Memory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(embedding) == 0 {
		return s.GetRecent(ctx, "", limit)
	}

	// Get all memories with embeddings
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, type, content, embedding, importance, source, context, temporal_tags, created_at, accessed_at, access_count
		FROM memories
		WHERE embedding IS NOT NULL AND embedding != ''
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Score each memory by cosine similarity
	type scoredMem struct {
		memory brain.Memory
		score  float64
	}
	var scored []scoredMem

	for rows.Next() {
		mem, err := s.scanMemory(rows)
		if err != nil {
			continue
		}

		if len(mem.Embedding) > 0 {
			similarity := cosineSimilarity(embedding, mem.Embedding)
			scored = append(scored, scoredMem{memory: *mem, score: similarity})
		}
	}

	// Sort by similarity descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Return top results
	result := make([]brain.Memory, 0, limit)
	for i := 0; i < len(scored) && i < limit; i++ {
		result = append(result, scored[i].memory)
	}

	return result, nil
}

// cosineSimilarity calculates the cosine similarity between two vectors.
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0.0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// Decay reduces importance of old unused memories.
func (s *SQLiteStore) Decay(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Reduce importance of memories not accessed in the last 30 days
	query := `UPDATE memories
		SET importance = importance * 0.9
		WHERE accessed_at < datetime('now', '-30 days')
		AND importance > 0.1`

	_, err := s.db.ExecContext(ctx, query)
	return err
}

// Consolidate merges similar memories.
// TODO: Implement memory consolidation with LLM assistance.
func (s *SQLiteStore) Consolidate(ctx context.Context) error {
	// Placeholder for future implementation
	return nil
}

// Prune removes old low-importance memories.
func (s *SQLiteStore) Prune(ctx context.Context, maxAge time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	query := `DELETE FROM memories WHERE importance < 0.2 AND created_at < ?`

	_, err := s.db.ExecContext(ctx, query, cutoff)
	return err
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// scanMemory scans a row into a Memory struct.
func (s *SQLiteStore) scanMemory(rows *sql.Rows) (*brain.Memory, error) {
	var mem brain.Memory
	var memType string
	var embeddingJSON, contextJSON, temporalJSON sql.NullString

	err := rows.Scan(
		&mem.ID,
		&mem.UserID,
		&memType,
		&mem.Content,
		&embeddingJSON,
		&mem.Importance,
		&mem.Source,
		&contextJSON,
		&temporalJSON,
		&mem.CreatedAt,
		&mem.AccessedAt,
		&mem.AccessCount,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan memory: %w", err)
	}

	mem.Type = brain.MemoryType(memType)

	// Deserialize embedding
	if embeddingJSON.Valid && embeddingJSON.String != "" {
		if err := json.Unmarshal([]byte(embeddingJSON.String), &mem.Embedding); err != nil {
			// Non-fatal, continue without embedding
			mem.Embedding = nil
		}
	}

	// Deserialize context
	if contextJSON.Valid && contextJSON.String != "" {
		if err := json.Unmarshal([]byte(contextJSON.String), &mem.Context); err != nil {
			mem.Context = nil
		}
	}

	// Deserialize temporal tags
	if temporalJSON.Valid && temporalJSON.String != "" {
		if err := json.Unmarshal([]byte(temporalJSON.String), &mem.TemporalTags); err != nil {
			mem.TemporalTags = nil
		}
	}

	return &mem, nil
}

// updateAccessStats updates the access time and count for a memory.
func (s *SQLiteStore) updateAccessStats(ctx context.Context, memID string) {
	// Run in background to not block the main query
	go func() {
		query := `UPDATE memories SET accessed_at = ?, access_count = access_count + 1 WHERE id = ?`
		s.db.ExecContext(ctx, query, time.Now(), memID)
	}()
}
