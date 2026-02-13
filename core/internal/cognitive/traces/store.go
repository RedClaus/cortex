package traces

import (
	"bytes"
	"compress/zlib"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/normanking/cortex/internal/memory"
)

// TraceStore manages reasoning trace persistence.
// It extends the Episode system for trace storage with additional metadata.
type TraceStore struct {
	db       *sql.DB
	embedder memory.Embedder
	episodes *memory.EpisodeStore
}

// NewTraceStore creates a new trace store.
func NewTraceStore(db *sql.DB, embedder memory.Embedder, episodes *memory.EpisodeStore) *TraceStore {
	return &TraceStore{
		db:       db,
		embedder: embedder,
		episodes: episodes,
	}
}

// StoreTrace persists a reasoning trace.
func (ts *TraceStore) StoreTrace(ctx context.Context, trace *ReasoningTrace) error {
	if trace.ID == "" {
		trace.ID = "trace_" + uuid.New().String()
	}
	if trace.CreatedAt.IsZero() {
		trace.CreatedAt = time.Now()
	}
	trace.LastUsedAt = trace.CreatedAt

	// Generate embedding if not present
	if len(trace.QueryEmbedding) == 0 && ts.embedder != nil {
		emb, err := ts.embedder.Embed(ctx, trace.Query)
		if err == nil {
			trace.QueryEmbedding = emb
		}
	}

	// Compress steps JSON
	stepsJSON, err := json.Marshal(trace.Steps)
	if err != nil {
		return fmt.Errorf("marshal steps: %w", err)
	}
	compressedSteps, err := compressData(stepsJSON)
	if err != nil {
		compressedSteps = stepsJSON // Fallback to uncompressed
	}

	// Prepare metadata
	metaJSON, _ := json.Marshal(trace.Metadata)
	toolsJSON, _ := json.Marshal(trace.ToolsUsed)
	lobesJSON, _ := json.Marshal(trace.LobesActivated)
	embeddingBlob := memory.Float32SliceToBytes(trace.QueryEmbedding)

	// Insert into reasoning_traces table (extends episodes concept)
	_, err = ts.db.ExecContext(ctx, `
		INSERT INTO reasoning_traces (
			id, query, query_embedding, approach, steps_json,
			outcome, success_score, reused_count,
			tools_used, lobes_activated, total_duration_ms, tokens_used,
			created_at, last_used_at, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			success_score = excluded.success_score,
			reused_count = excluded.reused_count,
			last_used_at = excluded.last_used_at
	`,
		trace.ID, trace.Query, embeddingBlob, trace.Approach, compressedSteps,
		string(trace.Outcome), trace.SuccessScore, trace.ReusedCount,
		string(toolsJSON), string(lobesJSON), trace.TotalDuration.Milliseconds(), trace.TokensUsed,
		trace.CreatedAt.Format(time.RFC3339), trace.LastUsedAt.Format(time.RFC3339), string(metaJSON),
	)
	if err != nil {
		return fmt.Errorf("insert trace: %w", err)
	}

	return nil
}

// GetTrace retrieves a trace by ID.
func (ts *TraceStore) GetTrace(ctx context.Context, id string) (*ReasoningTrace, error) {
	trace := &ReasoningTrace{}
	var (
		embBlob, stepsBlob              []byte
		outcomeStr, metaJSON            string
		toolsJSON, lobesJSON            string
		createdAt, lastUsedAt           string
		durationMs                      int64
	)

	err := ts.db.QueryRowContext(ctx, `
		SELECT id, query, query_embedding, approach, steps_json,
		       outcome, success_score, reused_count,
		       tools_used, lobes_activated, total_duration_ms, tokens_used,
		       created_at, last_used_at, metadata
		FROM reasoning_traces WHERE id = ?
	`, id).Scan(
		&trace.ID, &trace.Query, &embBlob, &trace.Approach, &stepsBlob,
		&outcomeStr, &trace.SuccessScore, &trace.ReusedCount,
		&toolsJSON, &lobesJSON, &durationMs, &trace.TokensUsed,
		&createdAt, &lastUsedAt, &metaJSON,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get trace: %w", err)
	}

	// Decompress steps
	decompressed, err := decompressData(stepsBlob)
	if err != nil {
		decompressed = stepsBlob // Try uncompressed
	}
	json.Unmarshal(decompressed, &trace.Steps)

	// Parse other fields
	trace.QueryEmbedding = memory.BytesToFloat32Slice(embBlob)
	trace.Outcome = TraceOutcome(outcomeStr)
	trace.TotalDuration = time.Duration(durationMs) * time.Millisecond
	trace.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	trace.LastUsedAt, _ = time.Parse(time.RFC3339, lastUsedAt)
	json.Unmarshal([]byte(metaJSON), &trace.Metadata)
	json.Unmarshal([]byte(toolsJSON), &trace.ToolsUsed)
	json.Unmarshal([]byte(lobesJSON), &trace.LobesActivated)

	return trace, nil
}

// IncrementReuse marks a trace as reused and updates last_used_at.
func (ts *TraceStore) IncrementReuse(ctx context.Context, id string) error {
	_, err := ts.db.ExecContext(ctx, `
		UPDATE reasoning_traces
		SET reused_count = reused_count + 1, last_used_at = ?
		WHERE id = ?
	`, time.Now().Format(time.RFC3339), id)
	return err
}

// GetRecentTraces retrieves the most recent traces.
func (ts *TraceStore) GetRecentTraces(ctx context.Context, limit int) ([]*ReasoningTrace, error) {
	rows, err := ts.db.QueryContext(ctx, `
		SELECT id, query, approach, outcome, success_score, reused_count,
		       tools_used, total_duration_ms, created_at
		FROM reasoning_traces
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var traces []*ReasoningTrace
	for rows.Next() {
		trace := &ReasoningTrace{}
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

		traces = append(traces, trace)
	}

	return traces, nil
}

// PruneOldTraces removes traces older than maxAge with low success scores.
func (ts *TraceStore) PruneOldTraces(ctx context.Context, maxAge time.Duration, minScore float64) (int, error) {
	cutoff := time.Now().Add(-maxAge).Format(time.RFC3339)
	result, err := ts.db.ExecContext(ctx, `
		DELETE FROM reasoning_traces
		WHERE created_at < ? AND success_score < ? AND reused_count = 0
	`, cutoff, minScore)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

// Stats returns statistics about stored traces.
func (ts *TraceStore) Stats(ctx context.Context) (*TraceStats, error) {
	stats := &TraceStats{}

	ts.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM reasoning_traces`).Scan(&stats.TotalTraces)
	ts.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM reasoning_traces WHERE outcome = 'success'`).Scan(&stats.SuccessfulTraces)
	ts.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM reasoning_traces WHERE reused_count > 0`).Scan(&stats.ReusedTraces)
	ts.db.QueryRowContext(ctx, `SELECT COALESCE(AVG(success_score), 0) FROM reasoning_traces`).Scan(&stats.AvgSuccessScore)
	ts.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(reused_count), 0) FROM reasoning_traces`).Scan(&stats.TotalReuses)

	// Calculate average steps per trace (requires decompressing, so estimate from token count)
	var avgTokens float64
	ts.db.QueryRowContext(ctx, `SELECT COALESCE(AVG(tokens_used), 0) FROM reasoning_traces`).Scan(&avgTokens)
	stats.AvgStepsPerTrace = avgTokens / 500 // Rough estimate: ~500 tokens per step

	return stats, nil
}

// compressData compresses data using zlib.
func compressData(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	_, err := w.Write(data)
	if err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// decompressData decompresses zlib data.
func decompressData(data []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}
