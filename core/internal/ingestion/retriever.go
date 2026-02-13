package ingestion

import (
	"context"
	gosql "database/sql"
	"fmt"
	"strings"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// KNOWLEDGE RETRIEVER
// ═══════════════════════════════════════════════════════════════════════════════

// Retriever searches the knowledge base for relevant chunks.
type Retriever struct {
	db     *gosql.DB
	config *RetrieverConfig
}

// NewRetriever creates a new knowledge retriever.
func NewRetriever(db *gosql.DB, config *RetrieverConfig) *Retriever {
	if config == nil {
		config = DefaultRetrieverConfig()
	}
	return &Retriever{
		db:     db,
		config: config,
	}
}

// Search performs a full-text search on the knowledge base.
// Returns chunks matching the query, ranked by relevance.
func (r *Retriever) Search(ctx context.Context, query string) (*RetrievalResult, error) {
	startTime := time.Now()

	// Use FTS5 for full-text search
	// The knowledge_chunks_fts table indexes: content, title, section_path, commands, keywords
	sql := `
		SELECT
			kc.id,
			kc.source_id,
			kc.content,
			kc.content_type,
			kc.title,
			kc.section_path,
			kc.commands,
			kc.keywords,
			kc.token_count,
			kc.quality_score,
			ks.name as source_name,
			ks.category
		FROM knowledge_chunks_fts fts
		JOIN knowledge_chunks kc ON fts.rowid = kc.rowid
		JOIN knowledge_sources ks ON kc.source_id = ks.id
		WHERE knowledge_chunks_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`

	// Prepare FTS query - escape special characters and add wildcards
	ftsQuery := prepareFTSQuery(query)

	rows, err := r.db.QueryContext(ctx, sql, ftsQuery, r.config.MaxResults)
	if err != nil {
		// If FTS fails (e.g., no matches), try a simple LIKE search as fallback
		return r.fallbackSearch(ctx, query, startTime)
	}
	defer rows.Close()

	var chunks []*ChunkResult
	for rows.Next() {
		var (
			id          string
			sourceID    string
			content     string
			contentType string
			title       gosql.NullString
			sectionPath gosql.NullString
			commands    gosql.NullString
			keywords    gosql.NullString
			tokenCount  int
			quality     float64
			sourceName  string
			category    gosql.NullString
		)

		if err := rows.Scan(
			&id, &sourceID, &content, &contentType,
			&title, &sectionPath, &commands, &keywords,
			&tokenCount, &quality, &sourceName, &category,
		); err != nil {
			continue
		}

		chunk := &Chunk{
			ID:          id,
			SourceID:    sourceID,
			Content:     content,
			ContentType: contentType,
			TokenCount:  tokenCount,
			QualityScore: quality,
		}
		if title.Valid {
			chunk.Title = title.String
		}
		if sectionPath.Valid {
			chunk.SectionPath = sectionPath.String
		}

		// Calculate a simple relevance score based on quality
		similarity := quality * 0.8 // Use quality as proxy for now

		chunks = append(chunks, &ChunkResult{
			Chunk:      chunk,
			Similarity: similarity,
			MatchType:  "keyword",
		})
	}

	// If no FTS results, try fallback
	if len(chunks) == 0 {
		return r.fallbackSearch(ctx, query, startTime)
	}

	// Calculate total tokens
	totalTokens := 0
	for _, cr := range chunks {
		totalTokens += cr.Chunk.TokenCount
	}

	return &RetrievalResult{
		Chunks:      chunks,
		TotalTokens: totalTokens,
		LatencyMs:   time.Since(startTime).Milliseconds(),
	}, nil
}

// fallbackSearch uses LIKE queries when FTS doesn't match.
func (r *Retriever) fallbackSearch(ctx context.Context, query string, startTime time.Time) (*RetrievalResult, error) {
	// Simple LIKE search on content
	sql := `
		SELECT
			kc.id,
			kc.source_id,
			kc.content,
			kc.content_type,
			kc.title,
			kc.section_path,
			kc.token_count,
			kc.quality_score,
			ks.name as source_name
		FROM knowledge_chunks kc
		JOIN knowledge_sources ks ON kc.source_id = ks.id
		WHERE kc.content LIKE ? OR kc.title LIKE ? OR kc.keywords LIKE ?
		ORDER BY kc.quality_score DESC
		LIMIT ?
	`

	pattern := "%" + query + "%"
	rows, err := r.db.QueryContext(ctx, sql, pattern, pattern, pattern, r.config.MaxResults)
	if err != nil {
		return nil, fmt.Errorf("fallback search: %w", err)
	}
	defer rows.Close()

	var chunks []*ChunkResult
	for rows.Next() {
		var (
			id          string
			sourceID    string
			content     string
			contentType string
			title       gosql.NullString
			sectionPath gosql.NullString
			tokenCount  int
			quality     float64
			sourceName  string
		)

		if err := rows.Scan(
			&id, &sourceID, &content, &contentType,
			&title, &sectionPath, &tokenCount, &quality, &sourceName,
		); err != nil {
			continue
		}

		chunk := &Chunk{
			ID:          id,
			SourceID:    sourceID,
			Content:     content,
			ContentType: contentType,
			TokenCount:  tokenCount,
			QualityScore: quality,
		}
		if title.Valid {
			chunk.Title = title.String
		}
		if sectionPath.Valid {
			chunk.SectionPath = sectionPath.String
		}

		chunks = append(chunks, &ChunkResult{
			Chunk:      chunk,
			Similarity: quality * 0.7,
			MatchType:  "keyword",
		})
	}

	totalTokens := 0
	for _, cr := range chunks {
		totalTokens += cr.Chunk.TokenCount
	}

	return &RetrievalResult{
		Chunks:      chunks,
		TotalTokens: totalTokens,
		LatencyMs:   time.Since(startTime).Milliseconds(),
	}, nil
}

// prepareFTSQuery converts a natural language query to FTS5 syntax.
func prepareFTSQuery(query string) string {
	// Split into words and join with OR for broader matching
	words := strings.Fields(query)
	if len(words) == 0 {
		return query
	}

	// Escape special FTS characters
	var escaped []string
	for _, word := range words {
		// Remove FTS special chars
		word = strings.ReplaceAll(word, "\"", "")
		word = strings.ReplaceAll(word, "*", "")
		word = strings.ReplaceAll(word, "(", "")
		word = strings.ReplaceAll(word, ")", "")
		if word != "" {
			escaped = append(escaped, word+"*") // Add prefix matching
		}
	}

	if len(escaped) == 0 {
		return query
	}

	// Join with OR for broader matching
	return strings.Join(escaped, " OR ")
}

// GetStats returns statistics about the knowledge base.
func (r *Retriever) GetStats(ctx context.Context) (*IngestionStats, error) {
	stats := &IngestionStats{
		SourcesByFormat: make(map[string]int),
		ChunksByType:    make(map[string]int),
	}

	// Count sources
	row := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM knowledge_sources WHERE status = 'active'")
	row.Scan(&stats.TotalSources)

	// Count chunks
	row = r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM knowledge_chunks")
	row.Scan(&stats.TotalChunks)

	// Average quality
	row = r.db.QueryRowContext(ctx, "SELECT AVG(quality_score) FROM knowledge_chunks")
	row.Scan(&stats.AvgQualityScore)

	// Sources by format
	rows, err := r.db.QueryContext(ctx, "SELECT format, COUNT(*) FROM knowledge_sources GROUP BY format")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var format string
			var count int
			if rows.Scan(&format, &count) == nil {
				stats.SourcesByFormat[format] = count
			}
		}
	}

	// Chunks by type
	rows, err = r.db.QueryContext(ctx, "SELECT content_type, COUNT(*) FROM knowledge_chunks GROUP BY content_type")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var ctype string
			var count int
			if rows.Scan(&ctype, &count) == nil {
				stats.ChunksByType[ctype] = count
			}
		}
	}

	return stats, nil
}
