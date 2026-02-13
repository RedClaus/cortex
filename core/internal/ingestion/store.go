package ingestion

import (
	"context"
	gosql "database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// KNOWLEDGE STORE
// ═══════════════════════════════════════════════════════════════════════════════

// Store handles persistence of knowledge sources and chunks.
type Store struct {
	db *gosql.DB
}

// NewStore creates a new knowledge store.
func NewStore(db *gosql.DB) *Store {
	return &Store{db: db}
}

// SaveSource saves a knowledge source to the database.
func (s *Store) SaveSource(ctx context.Context, result *IngestionResult, req *IngestionRequest) error {
	// Generate content hash
	contentHash := hashContent(req.Content)

	// Convert tags to JSON
	tagsJSON, _ := json.Marshal(req.Tags)

	sql := `
		INSERT INTO knowledge_sources (
			id, name, description, source_type, source_path, format,
			category, tags, platform, content_hash, status, chunk_count,
			quality_score, ingested_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			chunk_count = excluded.chunk_count,
			quality_score = excluded.quality_score,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := s.db.ExecContext(ctx, sql,
		result.SourceID,
		req.Name,
		req.Description,
		req.SourceType,
		req.SourcePath,
		detectFormatFromPath(req.SourcePath),
		req.Category,
		string(tagsJSON),
		req.Platform,
		contentHash,
		result.ChunksCreated,
		result.QualityScore,
	)

	return err
}

// SaveChunks saves chunks to the database.
func (s *Store) SaveChunks(ctx context.Context, chunks []*Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO knowledge_chunks (
			id, source_id, content, content_type, content_hash,
			parent_chunk_id, position, depth, title, section_path,
			embedding, embedding_model, embedding_dim,
			commands, keywords, start_offset, end_offset,
			token_count, quality_score, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(content_hash) DO UPDATE SET
			retrieval_count = knowledge_chunks.retrieval_count
	`)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, chunk := range chunks {
		// Generate content hash for CAS
		contentHash := hashContent(chunk.Content)

		// Convert arrays to JSON
		commandsJSON, _ := json.Marshal(chunk.Commands)
		keywordsJSON, _ := json.Marshal(chunk.Keywords)

		// Convert embedding to blob (if present)
		var embeddingBlob []byte
		if len(chunk.Embedding) > 0 {
			embeddingBlob = embeddingToBytes(chunk.Embedding)
		}

		embeddingDim := 768
		if len(chunk.Embedding) > 0 {
			embeddingDim = len(chunk.Embedding)
		}

		var parentID interface{}
		if chunk.ParentChunkID != nil {
			parentID = *chunk.ParentChunkID
		}

		_, err := stmt.ExecContext(ctx,
			chunk.ID,
			chunk.SourceID,
			chunk.Content,
			chunk.ContentType,
			contentHash,
			parentID,
			chunk.Position,
			chunk.Depth,
			chunk.Title,
			chunk.SectionPath,
			embeddingBlob,
			chunk.EmbeddingModel,
			embeddingDim,
			string(commandsJSON),
			string(keywordsJSON),
			chunk.StartOffset,
			chunk.EndOffset,
			chunk.TokenCount,
			chunk.QualityScore,
		)
		if err != nil {
			return fmt.Errorf("insert chunk %s: %w", chunk.ID, err)
		}
	}

	return tx.Commit()
}

// embeddingToBytes converts a float32 slice to bytes for SQLite BLOB storage.
func embeddingToBytes(embedding []float32) []byte {
	// Store as raw float32 bytes (4 bytes per float) using IEEE 754
	bytes := make([]byte, len(embedding)*4)
	for i, f := range embedding {
		bits := math.Float32bits(f)
		binary.LittleEndian.PutUint32(bytes[i*4:], bits)
	}
	return bytes
}

// GetStats returns statistics about the knowledge base.
func (s *Store) GetStats(ctx context.Context) (*IngestionStats, error) {
	stats := &IngestionStats{
		SourcesByFormat: make(map[string]int),
		ChunksByType:    make(map[string]int),
	}

	// Count sources
	row := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM knowledge_sources WHERE status = 'active'")
	row.Scan(&stats.TotalSources)

	// Count chunks
	row = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM knowledge_chunks")
	row.Scan(&stats.TotalChunks)

	// Average quality
	row = s.db.QueryRowContext(ctx, "SELECT COALESCE(AVG(quality_score), 0) FROM knowledge_chunks")
	row.Scan(&stats.AvgQualityScore)

	return stats, nil
}

// DeleteSource removes a source and its chunks.
func (s *Store) DeleteSource(ctx context.Context, sourceID string) error {
	// Chunks are deleted via CASCADE
	_, err := s.db.ExecContext(ctx, "DELETE FROM knowledge_sources WHERE id = ?", sourceID)
	return err
}

// ListSources returns all knowledge sources.
func (s *Store) ListSources(ctx context.Context) ([]*SourceHealth, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			ks.id, ks.name, ks.category, ks.format, ks.chunk_count, ks.status,
			COALESCE(SUM(kc.retrieval_count), 0) as total_retrievals,
			COALESCE(AVG(kc.avg_relevance_score), 0) as avg_relevance,
			COALESCE(AVG(kc.quality_score), 0) as avg_quality,
			MAX(kc.last_retrieved_at) as last_used
		FROM knowledge_sources ks
		LEFT JOIN knowledge_chunks kc ON ks.id = kc.source_id
		GROUP BY ks.id
		ORDER BY ks.ingested_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []*SourceHealth
	for rows.Next() {
		var (
			sh       SourceHealth
			category gosql.NullString
			lastUsed gosql.NullTime
		)
		err := rows.Scan(
			&sh.SourceID, &sh.Name, &category, &sh.Format,
			&sh.ChunkCount, &sh.Status, &sh.TotalRetrievals,
			&sh.AvgRelevance, &sh.AvgQuality, &lastUsed,
		)
		if err != nil {
			continue
		}
		if category.Valid {
			sh.Category = category.String
		}
		if lastUsed.Valid {
			sh.LastUsed = lastUsed.Time
		}
		sources = append(sources, &sh)
	}

	return sources, nil
}

// Ensure NullTime exists
type nullTime struct {
	Time  time.Time
	Valid bool
}

func (nt *nullTime) Scan(value interface{}) error {
	if value == nil {
		nt.Time, nt.Valid = time.Time{}, false
		return nil
	}
	switch v := value.(type) {
	case time.Time:
		nt.Time, nt.Valid = v, true
	case string:
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			t, err = time.Parse("2006-01-02 15:04:05", v)
		}
		if err == nil {
			nt.Time, nt.Valid = t, true
		}
	}
	return nil
}
