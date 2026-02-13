package traces

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/normanking/cortex/internal/memory"
)

// TraceRetriever finds similar reasoning traces for reuse.
type TraceRetriever struct {
	db           *sql.DB
	embedder     memory.Embedder
	minSimilarity float64 // Minimum similarity threshold (default: 0.7)
	maxResults    int     // Maximum traces to return (default: 5)
}

// NewTraceRetriever creates a new trace retriever.
func NewTraceRetriever(db *sql.DB, embedder memory.Embedder) *TraceRetriever {
	return &TraceRetriever{
		db:           db,
		embedder:     embedder,
		minSimilarity: 0.7,
		maxResults:    5,
	}
}

// SetMinSimilarity sets the minimum similarity threshold.
func (tr *TraceRetriever) SetMinSimilarity(threshold float64) {
	if threshold > 0 && threshold <= 1 {
		tr.minSimilarity = threshold
	}
}

// SetMaxResults sets the maximum number of results to return.
func (tr *TraceRetriever) SetMaxResults(max int) {
	if max > 0 {
		tr.maxResults = max
	}
}

// FindSimilar finds traces similar to the given query.
// Returns traces sorted by similarity score (highest first).
func (tr *TraceRetriever) FindSimilar(ctx context.Context, query string) ([]TraceSimilarity, error) {
	if tr.embedder == nil {
		return nil, nil
	}

	// Generate query embedding
	queryEmb, err := tr.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	return tr.FindSimilarByEmbedding(ctx, queryEmb)
}

// FindSimilarByEmbedding finds traces similar to the given embedding.
func (tr *TraceRetriever) FindSimilarByEmbedding(ctx context.Context, queryEmb []float32) ([]TraceSimilarity, error) {
	if len(queryEmb) == 0 {
		return nil, nil
	}

	// Query all traces with embeddings
	rows, err := tr.db.QueryContext(ctx, `
		SELECT id, query, query_embedding, approach, outcome, success_score,
		       reused_count, tools_used, total_duration_ms, created_at
		FROM reasoning_traces
		WHERE query_embedding IS NOT NULL
		  AND outcome = 'success'
		  AND success_score >= 0.5
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candidates []TraceSimilarity

	for rows.Next() {
		var trace ReasoningTrace
		var embBlob []byte
		var outcomeStr, toolsJSON, createdAt string
		var durationMs int64

		if err := rows.Scan(
			&trace.ID, &trace.Query, &embBlob, &trace.Approach, &outcomeStr,
			&trace.SuccessScore, &trace.ReusedCount, &toolsJSON, &durationMs, &createdAt,
		); err != nil {
			continue
		}

		traceEmb := memory.BytesToFloat32Slice(embBlob)
		if traceEmb == nil {
			continue
		}

		// Calculate cosine similarity
		sim := memory.CosineSimilarity(queryEmb, traceEmb)
		if sim >= tr.minSimilarity {
			trace.Outcome = TraceOutcome(outcomeStr)
			trace.TotalDuration = time.Duration(durationMs) * time.Millisecond
			trace.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
			json.Unmarshal([]byte(toolsJSON), &trace.ToolsUsed)

			candidates = append(candidates, TraceSimilarity{
				Trace:      &trace,
				Similarity: sim,
			})
		}
	}

	// Sort by similarity (descending)
	for i := 0; i < len(candidates)-1; i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].Similarity > candidates[i].Similarity {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	// Limit results
	if len(candidates) > tr.maxResults {
		candidates = candidates[:tr.maxResults]
	}

	return candidates, nil
}

// FindByTools finds traces that used specific tools.
func (tr *TraceRetriever) FindByTools(ctx context.Context, tools []string) ([]*ReasoningTrace, error) {
	if len(tools) == 0 {
		return nil, nil
	}

	// Build query to find traces containing any of the tools
	rows, err := tr.db.QueryContext(ctx, `
		SELECT id, query, approach, outcome, success_score, reused_count,
		       tools_used, total_duration_ms, created_at
		FROM reasoning_traces
		WHERE outcome = 'success'
		ORDER BY success_score DESC
		LIMIT 50
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*ReasoningTrace

	for rows.Next() {
		var trace ReasoningTrace
		var outcomeStr, toolsJSON, createdAt string
		var durationMs int64

		if err := rows.Scan(
			&trace.ID, &trace.Query, &trace.Approach, &outcomeStr,
			&trace.SuccessScore, &trace.ReusedCount, &toolsJSON, &durationMs, &createdAt,
		); err != nil {
			continue
		}

		json.Unmarshal([]byte(toolsJSON), &trace.ToolsUsed)

		// Check if trace uses any of the requested tools
		for _, reqTool := range tools {
			for _, traceTool := range trace.ToolsUsed {
				if reqTool == traceTool {
					trace.Outcome = TraceOutcome(outcomeStr)
					trace.TotalDuration = time.Duration(durationMs) * time.Millisecond
					trace.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
					results = append(results, &trace)
					goto nextRow
				}
			}
		}
	nextRow:
	}

	return results, nil
}

// SearchByText performs full-text search on trace queries.
func (tr *TraceRetriever) SearchByText(ctx context.Context, searchText string, limit int) ([]*ReasoningTrace, error) {
	if limit <= 0 {
		limit = 10
	}

	rows, err := tr.db.QueryContext(ctx, `
		SELECT t.id, t.query, t.approach, t.outcome, t.success_score,
		       t.reused_count, t.tools_used, t.total_duration_ms, t.created_at
		FROM reasoning_traces t
		JOIN reasoning_traces_fts fts ON t.rowid = fts.rowid
		WHERE reasoning_traces_fts MATCH ?
		ORDER BY t.success_score DESC
		LIMIT ?
	`, searchText, limit)
	if err != nil {
		// FTS table might not exist yet, fall back to LIKE
		rows, err = tr.db.QueryContext(ctx, `
			SELECT id, query, approach, outcome, success_score,
			       reused_count, tools_used, total_duration_ms, created_at
			FROM reasoning_traces
			WHERE query LIKE ? OR approach LIKE ?
			ORDER BY success_score DESC
			LIMIT ?
		`, "%"+searchText+"%", "%"+searchText+"%", limit)
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()

	var traces []*ReasoningTrace
	for rows.Next() {
		var trace ReasoningTrace
		var outcomeStr, toolsJSON, createdAt string
		var durationMs int64

		if err := rows.Scan(
			&trace.ID, &trace.Query, &trace.Approach, &outcomeStr,
			&trace.SuccessScore, &trace.ReusedCount, &toolsJSON, &durationMs, &createdAt,
		); err != nil {
			continue
		}

		trace.Outcome = TraceOutcome(outcomeStr)
		trace.TotalDuration = time.Duration(durationMs) * time.Millisecond
		trace.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		json.Unmarshal([]byte(toolsJSON), &trace.ToolsUsed)

		traces = append(traces, &trace)
	}

	return traces, nil
}
