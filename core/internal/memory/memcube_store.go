// Package memory provides enhanced memory capabilities for Cortex.
// This file implements the MemCubeStore for CR-025, managing MemCube persistence and retrieval.
package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/rs/zerolog/log"
)

// MemCubeStore manages MemCube persistence and retrieval.
type MemCubeStore struct {
	db        *sql.DB
	embedder  Embedder
	vectorIdx *VectorIndex
}

// NewMemCubeStore creates a new MemCubeStore.
func NewMemCubeStore(db *sql.DB, embedder Embedder, vectorIdx *VectorIndex) *MemCubeStore {
	return &MemCubeStore{
		db:        db,
		embedder:  embedder,
		vectorIdx: vectorIdx,
	}
}

// Save stores a MemCube, creating a new version if it already exists.
// Generates embedding if not present and embedder is available.
func (s *MemCubeStore) Save(ctx context.Context, cube *MemCube) error {
	if cube == nil {
		return fmt.Errorf("save memcube: cube is nil")
	}
	if cube.ID == "" {
		return fmt.Errorf("save memcube: cube ID is required")
	}

	// Generate embedding if not present
	if len(cube.Embedding) == 0 && cube.Content != "" && s.embedder != nil {
		emb, err := s.embedder.Embed(ctx, cube.Content)
		if err != nil {
			// Log but don't fail - embedding is optional for storage
			log.Warn().
				Err(err).
				Str("cube_id", cube.ID).
				Msg("memcube: embedding generation failed")
		} else {
			cube.Embedding = emb
		}
	}

	// Increment version if updating existing cube
	existing, _ := s.GetByID(ctx, cube.ID)
	if existing != nil {
		cube.Version = existing.Version + 1
	}
	cube.UpdatedAt = time.Now()

	// Convert embedding to bytes for storage
	embeddingBytes := Float32SliceToBytes(cube.Embedding)

	// Store in database
	query := `
		INSERT INTO memcubes (
			id, version, content, content_type, embedding,
			source, session_id, parent_id, created_by,
			confidence, success_count, failure_count,
			created_at, updated_at, last_accessed_at, access_count, scope
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			version = excluded.version,
			content = excluded.content,
			embedding = excluded.embedding,
			confidence = excluded.confidence,
			success_count = excluded.success_count,
			failure_count = excluded.failure_count,
			updated_at = excluded.updated_at,
			last_accessed_at = excluded.last_accessed_at,
			access_count = excluded.access_count
	`

	var lastAccessedAt interface{}
	if !cube.LastAccessedAt.IsZero() {
		lastAccessedAt = cube.LastAccessedAt.Format(time.RFC3339)
	}

	_, err := s.db.ExecContext(ctx, query,
		cube.ID,
		cube.Version,
		cube.Content,
		string(cube.ContentType),
		embeddingBytes,
		cube.Source,
		cube.SessionID,
		cube.ParentID,
		cube.CreatedBy,
		cube.Confidence,
		cube.SuccessCount,
		cube.FailureCount,
		cube.CreatedAt.Format(time.RFC3339),
		cube.UpdatedAt.Format(time.RFC3339),
		lastAccessedAt,
		cube.AccessCount,
		cube.Scope,
	)
	if err != nil {
		return fmt.Errorf("save memcube: %w", err)
	}

	// Update vector index for fast similarity search
	if len(cube.Embedding) > 0 && s.vectorIdx != nil {
		if err := s.vectorIdx.IndexMemory(ctx, cube.ID, MemoryTypeMemCube, cube.Embedding); err != nil {
			log.Warn().
				Err(err).
				Str("cube_id", cube.ID).
				Msg("memcube: vector index update failed")
		}
	}

	log.Debug().
		Str("cube_id", cube.ID).
		Int("version", cube.Version).
		Str("content_type", string(cube.ContentType)).
		Msg("memcube saved")

	return nil
}

// GetByID retrieves a MemCube by its ID.
func (s *MemCubeStore) GetByID(ctx context.Context, id string) (*MemCube, error) {
	query := `
		SELECT id, version, content, content_type, embedding,
		       source, session_id, parent_id, created_by,
		       confidence, success_count, failure_count,
		       created_at, updated_at, last_accessed_at, access_count, scope
		FROM memcubes
		WHERE id = ?
	`

	row := s.db.QueryRowContext(ctx, query, id)
	cube, err := scanMemCube(row)
	if err == sql.ErrNoRows {
		return nil, nil // Not found, return nil without error
	}
	if err != nil {
		return nil, fmt.Errorf("get memcube by id: %w", err)
	}

	// Load links
	links, err := s.GetLinks(ctx, id)
	if err == nil {
		cube.Links = links
	}

	return cube, nil
}

// Delete removes a MemCube by ID.
func (s *MemCubeStore) Delete(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM memcubes WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete memcube: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete memcube: check rows: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("delete memcube: not found: %s", id)
	}

	// Remove from vector index
	if s.vectorIdx != nil {
		s.vectorIdx.RemoveMemory(ctx, id)
	}

	return nil
}

// SearchSimilar finds cubes similar to query, optionally filtered by type.
func (s *MemCubeStore) SearchSimilar(ctx context.Context, query string, cubeType CubeType, limit int) ([]*MemCube, error) {
	if s.embedder == nil {
		return nil, fmt.Errorf("search similar: embedder not configured")
	}

	// Embed query
	queryEmb, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("search similar: embed query: %w", err)
	}

	return s.SearchSimilarByEmbedding(ctx, queryEmb, cubeType, limit)
}

// SearchSimilarByEmbedding finds cubes similar to the given embedding.
func (s *MemCubeStore) SearchSimilarByEmbedding(ctx context.Context, queryEmb []float32, cubeType CubeType, limit int) ([]*MemCube, error) {
	// Fetch all cubes with embeddings (could be optimized with vector index)
	sqlQuery := `
		SELECT id, version, content, content_type, embedding,
		       source, session_id, parent_id, created_by,
		       confidence, success_count, failure_count,
		       created_at, updated_at, last_accessed_at, access_count, scope
		FROM memcubes
		WHERE embedding IS NOT NULL
	`

	args := []interface{}{}
	if cubeType != "" {
		sqlQuery += " AND content_type = ?"
		args = append(args, string(cubeType))
	}

	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("search similar: query: %w", err)
	}
	defer rows.Close()

	cubes, err := scanMemCubes(rows)
	if err != nil {
		return nil, fmt.Errorf("search similar: scan: %w", err)
	}

	// Calculate similarity scores
	type scoredCube struct {
		cube  *MemCube
		score float64
	}

	scored := make([]scoredCube, 0, len(cubes))
	for _, cube := range cubes {
		if len(cube.Embedding) > 0 {
			similarity := CosineSimilarity(queryEmb, cube.Embedding)
			if similarity >= SimilarityThreshold {
				scored = append(scored, scoredCube{cube: cube, score: similarity})
			}
		}
	}

	// Sort by similarity descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Return top N, updating access times
	result := make([]*MemCube, 0, limit)
	for i := 0; i < len(scored) && i < limit; i++ {
		cube := scored[i].cube
		cube.Touch()
		result = append(result, cube)
	}

	return result, nil
}

// GetSuccessfulSkills retrieves high-success-rate skill cubes.
// Requires at least 3 observations and 70%+ success rate.
func (s *MemCubeStore) GetSuccessfulSkills(ctx context.Context, limit int) ([]*MemCube, error) {
	query := `
		SELECT id, version, content, content_type, embedding,
		       source, session_id, parent_id, created_by,
		       confidence, success_count, failure_count,
		       created_at, updated_at, last_accessed_at, access_count, scope
		FROM memcubes
		WHERE content_type = 'skill'
		  AND (success_count + failure_count) >= 3
		  AND CAST(success_count AS REAL) / (success_count + failure_count) >= 0.7
		ORDER BY success_count DESC
		LIMIT ?
	`

	return s.queryMemCubes(ctx, query, limit)
}

// GetByType retrieves cubes of a specific type, ordered by recency.
func (s *MemCubeStore) GetByType(ctx context.Context, cubeType CubeType, limit int) ([]*MemCube, error) {
	query := `
		SELECT id, version, content, content_type, embedding,
		       source, session_id, parent_id, created_by,
		       confidence, success_count, failure_count,
		       created_at, updated_at, last_accessed_at, access_count, scope
		FROM memcubes
		WHERE content_type = ?
		ORDER BY updated_at DESC
		LIMIT ?
	`

	return s.queryMemCubes(ctx, query, string(cubeType), limit)
}

// GetBySession retrieves cubes from a specific session.
func (s *MemCubeStore) GetBySession(ctx context.Context, sessionID string, limit int) ([]*MemCube, error) {
	query := `
		SELECT id, version, content, content_type, embedding,
		       source, session_id, parent_id, created_by,
		       confidence, success_count, failure_count,
		       created_at, updated_at, last_accessed_at, access_count, scope
		FROM memcubes
		WHERE session_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`

	return s.queryMemCubes(ctx, query, sessionID, limit)
}

// GetDescendants finds all cubes that were forked from this one.
func (s *MemCubeStore) GetDescendants(ctx context.Context, parentID string) ([]*MemCube, error) {
	query := `
		SELECT id, version, content, content_type, embedding,
		       source, session_id, parent_id, created_by,
		       confidence, success_count, failure_count,
		       created_at, updated_at, last_accessed_at, access_count, scope
		FROM memcubes
		WHERE parent_id = ?
		ORDER BY version DESC
	`

	return s.queryMemCubes(ctx, query, parentID)
}

// RecordSuccess increments success_count for a cube.
func (s *MemCubeStore) RecordSuccess(ctx context.Context, id string) error {
	now := time.Now().Format(time.RFC3339)
	query := `
		UPDATE memcubes
		SET success_count = success_count + 1,
		    updated_at = ?
		WHERE id = ?
	`

	result, err := s.db.ExecContext(ctx, query, now, id)
	if err != nil {
		return fmt.Errorf("record success: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("record success: cube not found: %s", id)
	}

	return nil
}

// RecordFailure increments failure_count for a cube.
func (s *MemCubeStore) RecordFailure(ctx context.Context, id string) error {
	now := time.Now().Format(time.RFC3339)
	query := `
		UPDATE memcubes
		SET failure_count = failure_count + 1,
		    updated_at = ?
		WHERE id = ?
	`

	result, err := s.db.ExecContext(ctx, query, now, id)
	if err != nil {
		return fmt.Errorf("record failure: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("record failure: cube not found: %s", id)
	}

	return nil
}

// ============================================================================
// LINK MANAGEMENT
// ============================================================================

// SaveLink creates or updates a link between cubes.
func (s *MemCubeStore) SaveLink(ctx context.Context, sourceID, targetID, relType string, confidence float64) error {
	query := `
		INSERT INTO memcube_links (source_id, target_id, rel_type, confidence, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(source_id, target_id, rel_type) DO UPDATE SET
			confidence = excluded.confidence
	`

	_, err := s.db.ExecContext(ctx, query, sourceID, targetID, relType, confidence, time.Now().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("save link: %w", err)
	}

	return nil
}

// GetLinks retrieves all links from a cube.
func (s *MemCubeStore) GetLinks(ctx context.Context, sourceID string) ([]CubeLink, error) {
	query := `
		SELECT target_id, rel_type, confidence
		FROM memcube_links
		WHERE source_id = ?
	`

	rows, err := s.db.QueryContext(ctx, query, sourceID)
	if err != nil {
		return nil, fmt.Errorf("get links: %w", err)
	}
	defer rows.Close()

	var links []CubeLink
	for rows.Next() {
		var link CubeLink
		if err := rows.Scan(&link.TargetID, &link.RelType, &link.Confidence); err != nil {
			return nil, fmt.Errorf("get links: scan: %w", err)
		}
		links = append(links, link)
	}

	return links, rows.Err()
}

// DeleteLink removes a link between cubes.
func (s *MemCubeStore) DeleteLink(ctx context.Context, sourceID, targetID, relType string) error {
	query := `DELETE FROM memcube_links WHERE source_id = ? AND target_id = ? AND rel_type = ?`
	_, err := s.db.ExecContext(ctx, query, sourceID, targetID, relType)
	if err != nil {
		return fmt.Errorf("delete link: %w", err)
	}
	return nil
}

// ============================================================================
// STATISTICS
// ============================================================================

// Stats returns statistics about the MemCube store.
func (s *MemCubeStore) Stats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total cubes
	var total int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memcubes`).Scan(&total)
	stats["total_cubes"] = total

	// Count by type
	var textCount, skillCount, toolCount int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memcubes WHERE content_type = 'text'`).Scan(&textCount)
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memcubes WHERE content_type = 'skill'`).Scan(&skillCount)
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memcubes WHERE content_type = 'tool'`).Scan(&toolCount)
	stats["text_cubes"] = textCount
	stats["skill_cubes"] = skillCount
	stats["tool_cubes"] = toolCount

	// Cubes with embeddings
	var withEmbedding int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memcubes WHERE embedding IS NOT NULL`).Scan(&withEmbedding)
	stats["with_embedding"] = withEmbedding

	// Reliable cubes (3+ observations, 70%+ success)
	var reliable int
	s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM memcubes
		WHERE (success_count + failure_count) >= 3
		  AND CAST(success_count AS REAL) / (success_count + failure_count) >= 0.7
	`).Scan(&reliable)
	stats["reliable_cubes"] = reliable

	// Total links
	var links int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memcube_links`).Scan(&links)
	stats["total_links"] = links

	return stats, nil
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// queryMemCubes is a helper that executes a query and scans results into MemCubes.
func (s *MemCubeStore) queryMemCubes(ctx context.Context, query string, args ...interface{}) ([]*MemCube, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanMemCubes(rows)
}

// rawMemCubeRow holds scanned values before processing.
type rawMemCubeRow struct {
	id, content            string
	version                int
	contentType            string
	embeddingBytes         []byte
	source, sessionID      sql.NullString
	parentID, createdBy    sql.NullString
	confidence             float64
	successCount           int
	failureCount           int
	createdAt, updatedAt   string
	lastAccessedAt         sql.NullString
	accessCount            int
	scope                  sql.NullString
}

// toMemCube converts raw scanned values to a MemCube struct.
func (r *rawMemCubeRow) toMemCube() *MemCube {
	cube := &MemCube{
		ID:           r.id,
		Version:      r.version,
		Content:      r.content,
		ContentType:  CubeType(r.contentType),
		Embedding:    BytesToFloat32Slice(r.embeddingBytes),
		Source:       r.source.String,
		SessionID:    r.sessionID.String,
		ParentID:     r.parentID.String,
		CreatedBy:    r.createdBy.String,
		Confidence:   r.confidence,
		SuccessCount: r.successCount,
		FailureCount: r.failureCount,
		AccessCount:  r.accessCount,
		Scope:        r.scope.String,
	}

	// Default scope if empty
	if cube.Scope == "" {
		cube.Scope = "personal"
	}

	// Parse timestamps
	if t, err := time.Parse(time.RFC3339, r.createdAt); err == nil {
		cube.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339, r.updatedAt); err == nil {
		cube.UpdatedAt = t
	}
	if r.lastAccessedAt.Valid && r.lastAccessedAt.String != "" {
		if t, err := time.Parse(time.RFC3339, r.lastAccessedAt.String); err == nil {
			cube.LastAccessedAt = t
		}
	}

	return cube
}

// scanMemCube scans a single row into a MemCube.
func scanMemCube(row *sql.Row) (*MemCube, error) {
	var r rawMemCubeRow

	err := row.Scan(
		&r.id, &r.version, &r.content, &r.contentType, &r.embeddingBytes,
		&r.source, &r.sessionID, &r.parentID, &r.createdBy,
		&r.confidence, &r.successCount, &r.failureCount,
		&r.createdAt, &r.updatedAt, &r.lastAccessedAt, &r.accessCount, &r.scope,
	)
	if err != nil {
		return nil, err
	}

	return r.toMemCube(), nil
}

// scanMemCubes scans multiple rows into a slice of MemCubes.
func scanMemCubes(rows *sql.Rows) ([]*MemCube, error) {
	var cubes []*MemCube

	for rows.Next() {
		var r rawMemCubeRow

		err := rows.Scan(
			&r.id, &r.version, &r.content, &r.contentType, &r.embeddingBytes,
			&r.source, &r.sessionID, &r.parentID, &r.createdBy,
			&r.confidence, &r.successCount, &r.failureCount,
			&r.createdAt, &r.updatedAt, &r.lastAccessedAt, &r.accessCount, &r.scope,
		)
		if err != nil {
			return nil, fmt.Errorf("scan memcube: %w", err)
		}

		cubes = append(cubes, r.toMemCube())
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan memcubes: rows error: %w", err)
	}

	return cubes, nil
}

// ============================================================================
// JSON SERIALIZATION (for MemCube content storage)
// ============================================================================

// SerializeMetadata converts metadata map to JSON string.
func SerializeMetadata(metadata map[string]interface{}) string {
	if metadata == nil {
		return "{}"
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// DeserializeMetadata converts JSON string to metadata map.
func DeserializeMetadata(jsonStr string) map[string]interface{} {
	if jsonStr == "" {
		return make(map[string]interface{})
	}
	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &metadata); err != nil {
		return make(map[string]interface{})
	}
	return metadata
}

// MemoryTypeMemCube is the memory type constant for MemCubes in the vector index.
const MemoryTypeMemCube MemoryType = "memcube"
