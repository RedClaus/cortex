// Package knowledge provides full-text search and knowledge management functionality.
package knowledge

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/normanking/cortex/pkg/types"
)

// Note: Searcher and ScoredItem interfaces are defined in interfaces.go
// This file provides the FTS5 implementation.

// FTS5Searcher implements Searcher using SQLite FTS5.
type FTS5Searcher struct {
	db *sql.DB
}

// NewFTS5Searcher creates a new FTS5-based searcher.
func NewFTS5Searcher(db *sql.DB) *FTS5Searcher {
	return &FTS5Searcher{db: db}
}

// Search performs full-text search with trust-weighted ranking.
// Implements the Searcher interface defined in interfaces.go.
func (s *FTS5Searcher) Search(ctx context.Context, query string, opts types.SearchOptions) ([]*ScoredItem, error) {
	if query == "" {
		return nil, fmt.Errorf("search query cannot be empty")
	}

	// Set defaults
	if opts.Limit == 0 {
		opts.Limit = 10
	}

	// Escape and prepare FTS5 query
	ftsQuery, err := prepareFTS5Query(query)
	if err != nil {
		return nil, fmt.Errorf("invalid search query: %w", err)
	}

	// Build SQL query
	sqlQuery, args := s.buildSearchQuery(ftsQuery, opts)

	// Execute query
	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("search query failed: %w", err)
	}
	defer rows.Close()

	// Parse results
	results := make([]*ScoredItem, 0, opts.Limit)
	for rows.Next() {
		item, err := s.scanSearchResult(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}
		results = append(results, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating results: %w", err)
	}

	return results, nil
}

// Index rebuilds the FTS5 index for all knowledge items.
// This is typically called after bulk imports or database migrations.
func (s *FTS5Searcher) Index(ctx context.Context) error {
	// FTS5 is automatically maintained by triggers, but we can optimize it
	_, err := s.db.ExecContext(ctx, "INSERT INTO knowledge_fts(knowledge_fts) VALUES('optimize')")
	if err != nil {
		return fmt.Errorf("failed to optimize FTS5 index: %w", err)
	}
	return nil
}

// buildSearchQuery constructs the SQL query with all filters and ranking.
func (s *FTS5Searcher) buildSearchQuery(ftsQuery string, opts types.SearchOptions) (string, []interface{}) {
	args := []interface{}{ftsQuery}

	// Default trust weight for ranking
	trustWeight := 0.3
	if opts.MinTrust > 0.5 {
		// If user filters for high trust, weight it more in ranking
		trustWeight = 0.5
	}

	// Base query with BM25 ranking
	query := `
SELECT
    k.id, k.type, k.title, k.content, k.tags,
    k.scope, k.team_id, k.author_id, k.author_name,
    k.confidence, k.trust_score,
    k.success_count, k.failure_count, k.access_count,
    k.version, k.remote_id, k.sync_status, k.last_synced_at,
    k.created_at, k.updated_at, k.last_accessed_at, k.deleted_at,
    bm25(f) as bm25_score,
    highlight(f, 1, '<mark>', '</mark>') as highlighted_title,
    highlight(f, 2, '<mark>', '</mark>') as highlighted_content
FROM knowledge_fts f
JOIN knowledge_items k ON f.rowid = k.rowid
WHERE f MATCH ?
AND k.deleted_at IS NULL`

	// Add scope filter (renamed from Tiers in SearchOptions)
	if len(opts.Tiers) > 0 {
		placeholders := make([]string, len(opts.Tiers))
		for i, scope := range opts.Tiers {
			placeholders[i] = "?"
			args = append(args, string(scope))
		}
		query += fmt.Sprintf("\nAND k.scope IN (%s)", strings.Join(placeholders, ", "))
	}

	// Add type filter
	if len(opts.Types) > 0 {
		placeholders := make([]string, len(opts.Types))
		for i, typ := range opts.Types {
			placeholders[i] = "?"
			args = append(args, typ)
		}
		query += fmt.Sprintf("\nAND k.type IN (%s)", strings.Join(placeholders, ", "))
	}

	// Add tag filter (must contain ALL specified tags)
	if len(opts.Tags) > 0 {
		for _, tag := range opts.Tags {
			query += "\nAND k.tags LIKE ?"
			args = append(args, "%"+tag+"%")
		}
	}

	// Add trust score filter
	if opts.MinTrust > 0 {
		query += "\nAND k.trust_score >= ?"
		args = append(args, opts.MinTrust)
	}

	// Calculate final score: weighted combination of BM25 and trust
	// BM25 scores are negative (lower is better), so we negate them
	// Final score = (1 - trust_weight) * (-bm25) + trust_weight * trust_score
	query += fmt.Sprintf(`
ORDER BY ((%f * (-bm25(f))) + (%f * k.trust_score)) DESC
LIMIT ?`,
		1.0-trustWeight, trustWeight)

	args = append(args, opts.Limit)

	return query, args
}

// scanSearchResult parses a row into a ScoredItem.
func (s *FTS5Searcher) scanSearchResult(rows *sql.Rows) (*ScoredItem, error) {
	var item types.KnowledgeItem
	var bm25Score float64
	var highlightedTitle, highlightedContent string
	var tagsJSON string
	var lastSyncedAt, lastAccessedAt, deletedAt sql.NullTime

	err := rows.Scan(
		&item.ID, &item.Type, &item.Title, &item.Content, &tagsJSON,
		&item.Scope, &item.TeamID, &item.AuthorID, &item.AuthorName,
		&item.Confidence, &item.TrustScore,
		&item.SuccessCount, &item.FailureCount, &item.AccessCount,
		&item.Version, &item.RemoteID, &item.SyncStatus, &lastSyncedAt,
		&item.CreatedAt, &item.UpdatedAt, &lastAccessedAt, &deletedAt,
		&bm25Score, &highlightedTitle, &highlightedContent,
	)
	if err != nil {
		return nil, err
	}

	// Parse JSON tags
	item.Tags = parseTags(tagsJSON)

	// Handle nullable timestamps
	if lastSyncedAt.Valid {
		item.LastSyncedAt = lastSyncedAt.Time
	}
	if lastAccessedAt.Valid {
		item.LastAccessedAt = &lastAccessedAt.Time
	}
	if deletedAt.Valid {
		item.DeletedAt = &deletedAt.Time
	}

	// Calculate relevance score (normalized 0.0 - 1.0)
	// BM25 is negative (lower is better), convert to positive normalized score
	// Combine with trust score for final relevance
	relevance := calculateRelevance(bm25Score, item.TrustScore)

	return &ScoredItem{
		Item:      &item,
		Relevance: relevance,
	}, nil
}

// calculateRelevance converts BM25 score and trust into a normalized relevance score (0.0 - 1.0)
func calculateRelevance(bm25Score, trustScore float64) float64 {
	// BM25 scores are negative, typically in range [-10, 0]
	// Normalize to [0, 1] assuming -10 is lowest relevance
	const maxBM25 = 10.0
	normalizedBM25 := 1.0 - ((-bm25Score) / maxBM25)
	if normalizedBM25 < 0 {
		normalizedBM25 = 0
	}
	if normalizedBM25 > 1 {
		normalizedBM25 = 1
	}

	// Weighted combination: 70% BM25 relevance, 30% trust
	relevance := (normalizedBM25 * 0.7) + (trustScore * 0.3)
	return relevance
}

// prepareFTS5Query escapes special FTS5 characters and prepares the query.
func prepareFTS5Query(query string) (string, error) {
	// Trim whitespace
	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("query cannot be empty")
	}

	// Handle quoted phrases (preserve them)
	var result strings.Builder
	inQuote := false
	escaped := false

	for i := 0; i < len(query); i++ {
		char := query[i]

		if escaped {
			result.WriteByte(char)
			escaped = false
			continue
		}

		switch char {
		case '\\':
			escaped = true
			result.WriteByte(char)
		case '"':
			inQuote = !inQuote
			result.WriteByte(char)
		case '*', '(', ')', '{', '}', '[', ']', '^', ':':
			// Escape special FTS5 characters when not in quotes
			if !inQuote {
				result.WriteByte('"')
				result.WriteByte(char)
				result.WriteByte('"')
			} else {
				result.WriteByte(char)
			}
		default:
			result.WriteByte(char)
		}
	}

	ftsQuery := result.String()

	// If the query doesn't contain AND/OR operators, use OR by default
	// This provides better recall for multi-term queries
	if !strings.Contains(strings.ToUpper(ftsQuery), " AND ") &&
		!strings.Contains(strings.ToUpper(ftsQuery), " OR ") {
		// Split on spaces and join with OR (unless already quoted)
		terms := strings.Fields(ftsQuery)
		if len(terms) > 1 {
			ftsQuery = strings.Join(terms, " OR ")
		}
	}

	return ftsQuery, nil
}

// extractSearchTerms extracts individual search terms from a query.
func extractSearchTerms(query string) []string {
	// Remove quotes and special operators
	query = strings.ReplaceAll(query, `"`, "")
	query = strings.ToLower(query)

	// Remove operators
	query = strings.ReplaceAll(query, " and ", " ")
	query = strings.ReplaceAll(query, " or ", " ")
	query = strings.ReplaceAll(query, " not ", " ")

	// Split on whitespace
	terms := strings.Fields(query)

	// Filter out very short terms
	filtered := make([]string, 0, len(terms))
	for _, term := range terms {
		if len(term) >= 2 {
			filtered = append(filtered, term)
		}
	}

	return filtered
}

// parseTags converts JSON tags string to string slice.
func parseTags(tagsJSON string) []string {
	tagsJSON = strings.TrimSpace(tagsJSON)
	if tagsJSON == "" || tagsJSON == "[]" || tagsJSON == "null" {
		return nil
	}

	// Simple JSON array parsing (assumes well-formed input)
	tagsJSON = strings.Trim(tagsJSON, "[]")
	if tagsJSON == "" {
		return nil
	}

	parts := strings.Split(tagsJSON, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		tag := strings.Trim(strings.TrimSpace(part), `"`)
		if tag != "" {
			tags = append(tags, tag)
		}
	}

	return tags
}
