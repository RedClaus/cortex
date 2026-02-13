package eval

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// OUTCOME LOGGER INTERFACE
// ═══════════════════════════════════════════════════════════════════════════════

// OutcomeLogger provides logging and querying of routing outcomes for RoamPal learning.
type OutcomeLogger interface {
	// LogOutcome records a routing decision outcome.
	LogOutcome(ctx context.Context, outcome *RoutingOutcome, provider, model, taskType string, latencyMs int) error

	// GetModelStats retrieves success rate and sample count for a model/task combination.
	GetModelStats(ctx context.Context, provider, model, taskType string) (successRate float64, count int, err error)

	// GetLaneStats retrieves success rate and sample count for a lane/task combination.
	GetLaneStats(ctx context.Context, lane, taskType string) (successRate float64, count int, err error)

	// GetRecentOutcomes retrieves recent routing outcomes for analysis.
	GetRecentOutcomes(ctx context.Context, limit int) ([]*RoutingOutcomeWithMeta, error)
}

// RoutingOutcomeWithMeta extends RoutingOutcome with metadata for queries.
type RoutingOutcomeWithMeta struct {
	RoutingOutcome
	Provider  string    `json:"provider"`
	Model     string    `json:"model"`
	TaskType  string    `json:"task_type"`
	RequestID string    `json:"request_id"`
	CreatedAt time.Time `json:"created_at"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// SQLITE OUTCOME LOGGER IMPLEMENTATION
// ═══════════════════════════════════════════════════════════════════════════════

// SQLiteOutcomeLogger implements OutcomeLogger and autollm.OutcomeStore using SQLite.
// It uses the conversation_logs table with routing columns from migration 015.
type SQLiteOutcomeLogger struct {
	db *sql.DB
}

// NewSQLiteOutcomeLogger creates a new SQLite-backed outcome logger.
func NewSQLiteOutcomeLogger(db *sql.DB) *SQLiteOutcomeLogger {
	return &SQLiteOutcomeLogger{
		db: db,
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// OUTCOME LOGGER INTERFACE IMPLEMENTATION
// ═══════════════════════════════════════════════════════════════════════════════

// LogOutcome records a routing decision outcome.
// This updates the routing columns on an existing conversation_log entry.
func (l *SQLiteOutcomeLogger) LogOutcome(ctx context.Context, outcome *RoutingOutcome, provider, model, taskType string, latencyMs int) error {
	if outcome == nil {
		return fmt.Errorf("log outcome: outcome is nil")
	}

	// Find the most recent log entry for this provider/model combination
	// and update it with routing outcome data
	query := `
		UPDATE conversation_logs SET
			routing_lane = ?,
			routing_reason = ?,
			routing_forced = ?,
			routing_constraint = ?,
			outcome_score = ?
		WHERE id = (
			SELECT id FROM conversation_logs
			WHERE provider = ? AND model = ?
			ORDER BY created_at DESC
			LIMIT 1
		)
	`

	result, err := l.db.ExecContext(ctx, query,
		nullString(outcome.Lane),
		nullString(outcome.Reason),
		boolToInt(outcome.Forced),
		nullString(outcome.Constraint),
		outcome.OutcomeScore,
		provider,
		model,
	)

	if err != nil {
		return fmt.Errorf("log outcome: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("log outcome check rows: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("log outcome: no matching conversation log found for %s/%s", provider, model)
	}

	return nil
}

// GetModelStats retrieves success rate and sample count for a model/task combination.
func (l *SQLiteOutcomeLogger) GetModelStats(ctx context.Context, provider, model, taskType string) (float64, int, error) {
	var query string
	var args []interface{}

	if taskType != "" {
		query = `
			SELECT
				COUNT(*) as total,
				SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END) as success_count,
				AVG(CASE WHEN outcome_score IS NOT NULL THEN outcome_score ELSE CASE WHEN success = 1 THEN 1.0 ELSE 0.0 END END) as avg_score
			FROM conversation_logs
			WHERE provider = ? AND model = ? AND task_type = ?
			AND routing_lane IS NOT NULL
		`
		args = []interface{}{provider, model, taskType}
	} else {
		query = `
			SELECT
				COUNT(*) as total,
				SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END) as success_count,
				AVG(CASE WHEN outcome_score IS NOT NULL THEN outcome_score ELSE CASE WHEN success = 1 THEN 1.0 ELSE 0.0 END END) as avg_score
			FROM conversation_logs
			WHERE provider = ? AND model = ?
			AND routing_lane IS NOT NULL
		`
		args = []interface{}{provider, model}
	}

	var total, successCount int
	var avgScore sql.NullFloat64

	err := l.db.QueryRowContext(ctx, query, args...).Scan(&total, &successCount, &avgScore)
	if err != nil {
		return 0, 0, fmt.Errorf("get model stats: %w", err)
	}

	if total == 0 {
		return 0, 0, nil
	}

	// Use outcome_score average if available, otherwise use success rate
	successRate := float64(successCount) / float64(total)
	if avgScore.Valid {
		successRate = avgScore.Float64
	}

	return successRate, total, nil
}

// GetLaneStats retrieves success rate and sample count for a lane/task combination.
func (l *SQLiteOutcomeLogger) GetLaneStats(ctx context.Context, lane, taskType string) (float64, int, error) {
	var query string
	var args []interface{}

	if taskType != "" {
		query = `
			SELECT
				COUNT(*) as total,
				SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END) as success_count,
				AVG(CASE WHEN outcome_score IS NOT NULL THEN outcome_score ELSE CASE WHEN success = 1 THEN 1.0 ELSE 0.0 END END) as avg_score
			FROM conversation_logs
			WHERE routing_lane = ? AND task_type = ?
		`
		args = []interface{}{lane, taskType}
	} else {
		query = `
			SELECT
				COUNT(*) as total,
				SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END) as success_count,
				AVG(CASE WHEN outcome_score IS NOT NULL THEN outcome_score ELSE CASE WHEN success = 1 THEN 1.0 ELSE 0.0 END END) as avg_score
			FROM conversation_logs
			WHERE routing_lane = ?
		`
		args = []interface{}{lane}
	}

	var total, successCount int
	var avgScore sql.NullFloat64

	err := l.db.QueryRowContext(ctx, query, args...).Scan(&total, &successCount, &avgScore)
	if err != nil {
		return 0, 0, fmt.Errorf("get lane stats: %w", err)
	}

	if total == 0 {
		return 0, 0, nil
	}

	// Use outcome_score average if available, otherwise use success rate
	successRate := float64(successCount) / float64(total)
	if avgScore.Valid {
		successRate = avgScore.Float64
	}

	return successRate, total, nil
}

// GetRecentOutcomes retrieves recent routing outcomes for analysis.
func (l *SQLiteOutcomeLogger) GetRecentOutcomes(ctx context.Context, limit int) ([]*RoutingOutcomeWithMeta, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT
			request_id, provider, model, task_type,
			routing_lane, routing_reason, routing_forced, routing_constraint,
			success, outcome_score, duration_ms,
			created_at
		FROM conversation_logs
		WHERE routing_lane IS NOT NULL
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := l.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("get recent outcomes: %w", err)
	}
	defer rows.Close()

	var outcomes []*RoutingOutcomeWithMeta
	for rows.Next() {
		var outcome RoutingOutcomeWithMeta
		var taskType, lane, reason, constraint sql.NullString
		var outcomeScore sql.NullFloat64
		var routingForced, success, durationMs int

		err := rows.Scan(
			&outcome.RequestID,
			&outcome.Provider,
			&outcome.Model,
			&taskType,
			&lane,
			&reason,
			&routingForced,
			&constraint,
			&success,
			&outcomeScore,
			&durationMs,
			&outcome.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan outcome: %w", err)
		}

		outcome.TaskType = taskType.String
		outcome.Lane = lane.String
		outcome.Reason = reason.String
		outcome.Constraint = constraint.String
		outcome.Forced = routingForced == 1
		outcome.OutcomeSuccess = success == 1
		outcome.LatencyMs = durationMs
		outcome.ModelSelected = outcome.Model

		if outcomeScore.Valid {
			outcome.OutcomeScore = outcomeScore.Float64
		} else if success == 1 {
			outcome.OutcomeScore = 1.0
		}

		outcomes = append(outcomes, &outcome)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outcomes: %w", err)
	}

	return outcomes, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// AUTOLLM OUTCOME STORE INTERFACE IMPLEMENTATION
// ═══════════════════════════════════════════════════════════════════════════════

// RoutingOutcomeRecord is a single routing outcome for storage.
// This type mirrors autollm.RoutingOutcomeRecord to avoid circular imports.
// SQLiteOutcomeLogger implements autollm.OutcomeStore interface with this type.
type RoutingOutcomeRecord struct {
	Timestamp    time.Time
	Provider     string
	Model        string
	Lane         string
	TaskType     string
	Success      bool
	Score        float64
	LatencyMs    int
	WasEscalated bool
}

// GetModelSuccessRate returns the success rate for a model on a task type.
// Returns: successRate (0-1), sampleCount, error
// This method implements autollm.OutcomeStore.GetModelSuccessRate.
func (l *SQLiteOutcomeLogger) GetModelSuccessRate(ctx context.Context, provider, model, taskType string) (float64, int, error) {
	return l.GetModelStats(ctx, provider, model, taskType)
}

// GetLaneSuccessRate returns the success rate for a lane on a task type.
// This method implements autollm.OutcomeStore.GetLaneSuccessRate.
func (l *SQLiteOutcomeLogger) GetLaneSuccessRate(ctx context.Context, lane, taskType string) (float64, int, error) {
	return l.GetLaneStats(ctx, lane, taskType)
}

// RecordOutcome records a routing outcome for future learning.
// This method implements autollm.OutcomeStore.RecordOutcome.
// Note: The outcome parameter type mirrors autollm.RoutingOutcomeRecord.
func (l *SQLiteOutcomeLogger) RecordOutcome(ctx context.Context, outcome *RoutingOutcomeRecord) error {
	if outcome == nil {
		return fmt.Errorf("record outcome: outcome is nil")
	}

	// Insert a new log entry specifically for routing outcome tracking,
	// or update the most recent matching entry
	query := `
		UPDATE conversation_logs SET
			routing_lane = ?,
			outcome_score = ?
		WHERE id = (
			SELECT id FROM conversation_logs
			WHERE provider = ? AND model = ?
			AND task_type = COALESCE(?, task_type)
			ORDER BY created_at DESC
			LIMIT 1
		)
	`

	result, err := l.db.ExecContext(ctx, query,
		outcome.Lane,
		outcome.Score,
		outcome.Provider,
		outcome.Model,
		nullString(outcome.TaskType),
	)

	if err != nil {
		return fmt.Errorf("record outcome: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("record outcome check rows: %w", err)
	}

	// If no existing log was found, we can optionally insert a lightweight record
	// for pure routing tracking (without full conversation data)
	if rowsAffected == 0 {
		insertQuery := `
			INSERT INTO conversation_logs (
				request_id, provider, model, task_type,
				routing_lane, outcome_score, success,
				prompt, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, '', ?)
		`

		requestID := fmt.Sprintf("outcome-%d", outcome.Timestamp.UnixNano())
		successInt := 0
		if outcome.Success {
			successInt = 1
		}

		_, err = l.db.ExecContext(ctx, insertQuery,
			requestID,
			outcome.Provider,
			outcome.Model,
			nullString(outcome.TaskType),
			outcome.Lane,
			outcome.Score,
			successInt,
			outcome.Timestamp,
		)

		if err != nil {
			return fmt.Errorf("record outcome insert: %w", err)
		}
	}

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// ADVANCED QUERIES FOR ROAMPAL LEARNING
// ═══════════════════════════════════════════════════════════════════════════════

// GetModelPerformanceHistory returns time-series performance data for a model.
// This can be used for trend analysis and detecting model degradation.
func (l *SQLiteOutcomeLogger) GetModelPerformanceHistory(ctx context.Context, provider, model string, days int) ([]ModelDailyStats, error) {
	if days <= 0 {
		days = 30
	}

	query := `
		SELECT
			DATE(created_at) as date,
			COUNT(*) as total_requests,
			SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END) as success_count,
			AVG(CASE WHEN outcome_score IS NOT NULL THEN outcome_score END) as avg_score,
			AVG(duration_ms) as avg_latency_ms
		FROM conversation_logs
		WHERE provider = ? AND model = ?
		AND created_at >= DATE('now', '-' || ? || ' days')
		AND routing_lane IS NOT NULL
		GROUP BY DATE(created_at)
		ORDER BY date DESC
	`

	rows, err := l.db.QueryContext(ctx, query, provider, model, days)
	if err != nil {
		return nil, fmt.Errorf("get model performance history: %w", err)
	}
	defer rows.Close()

	var stats []ModelDailyStats
	for rows.Next() {
		var s ModelDailyStats
		var avgScore sql.NullFloat64
		var avgLatency sql.NullFloat64

		err := rows.Scan(&s.Date, &s.TotalRequests, &s.SuccessCount, &avgScore, &avgLatency)
		if err != nil {
			return nil, fmt.Errorf("scan model stats: %w", err)
		}

		if s.TotalRequests > 0 {
			s.SuccessRate = float64(s.SuccessCount) / float64(s.TotalRequests)
		}
		if avgScore.Valid {
			s.AvgScore = avgScore.Float64
		}
		if avgLatency.Valid {
			s.AvgLatencyMs = int(avgLatency.Float64)
		}

		stats = append(stats, s)
	}

	return stats, nil
}

// GetLaneDistribution returns the distribution of requests across lanes.
func (l *SQLiteOutcomeLogger) GetLaneDistribution(ctx context.Context, days int) (map[string]LaneStats, error) {
	if days <= 0 {
		days = 30
	}

	query := `
		SELECT
			routing_lane,
			COUNT(*) as total,
			SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END) as success_count,
			AVG(CASE WHEN outcome_score IS NOT NULL THEN outcome_score END) as avg_score,
			SUM(CASE WHEN routing_forced = 1 THEN 1 ELSE 0 END) as forced_count
		FROM conversation_logs
		WHERE routing_lane IS NOT NULL
		AND created_at >= DATE('now', '-' || ? || ' days')
		GROUP BY routing_lane
	`

	rows, err := l.db.QueryContext(ctx, query, days)
	if err != nil {
		return nil, fmt.Errorf("get lane distribution: %w", err)
	}
	defer rows.Close()

	result := make(map[string]LaneStats)
	for rows.Next() {
		var lane string
		var stats LaneStats
		var avgScore sql.NullFloat64

		err := rows.Scan(&lane, &stats.Total, &stats.SuccessCount, &avgScore, &stats.ForcedCount)
		if err != nil {
			return nil, fmt.Errorf("scan lane stats: %w", err)
		}

		if stats.Total > 0 {
			stats.SuccessRate = float64(stats.SuccessCount) / float64(stats.Total)
		}
		if avgScore.Valid {
			stats.AvgScore = avgScore.Float64
		}

		result[lane] = stats
	}

	return result, nil
}

// GetTopPerformingModels returns the best performing models for a task type.
func (l *SQLiteOutcomeLogger) GetTopPerformingModels(ctx context.Context, taskType string, limit int) ([]ModelRanking, error) {
	if limit <= 0 {
		limit = 10
	}

	var query string
	var args []interface{}

	if taskType != "" {
		query = `
			SELECT
				provider, model,
				COUNT(*) as total,
				AVG(CASE WHEN outcome_score IS NOT NULL THEN outcome_score ELSE CASE WHEN success = 1 THEN 1.0 ELSE 0.0 END END) as avg_score,
				AVG(duration_ms) as avg_latency_ms
			FROM conversation_logs
			WHERE task_type = ?
			AND routing_lane IS NOT NULL
			GROUP BY provider, model
			HAVING COUNT(*) >= 5
			ORDER BY avg_score DESC, avg_latency_ms ASC
			LIMIT ?
		`
		args = []interface{}{taskType, limit}
	} else {
		query = `
			SELECT
				provider, model,
				COUNT(*) as total,
				AVG(CASE WHEN outcome_score IS NOT NULL THEN outcome_score ELSE CASE WHEN success = 1 THEN 1.0 ELSE 0.0 END END) as avg_score,
				AVG(duration_ms) as avg_latency_ms
			FROM conversation_logs
			WHERE routing_lane IS NOT NULL
			GROUP BY provider, model
			HAVING COUNT(*) >= 5
			ORDER BY avg_score DESC, avg_latency_ms ASC
			LIMIT ?
		`
		args = []interface{}{limit}
	}

	rows, err := l.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get top performing models: %w", err)
	}
	defer rows.Close()

	var rankings []ModelRanking
	rank := 1
	for rows.Next() {
		var r ModelRanking
		var avgLatency sql.NullFloat64

		err := rows.Scan(&r.Provider, &r.Model, &r.SampleCount, &r.AvgScore, &avgLatency)
		if err != nil {
			return nil, fmt.Errorf("scan model ranking: %w", err)
		}

		r.Rank = rank
		if avgLatency.Valid {
			r.AvgLatencyMs = int(avgLatency.Float64)
		}
		rank++

		rankings = append(rankings, r)
	}

	return rankings, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// SUPPORTING TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// ModelDailyStats contains daily aggregated stats for a model.
type ModelDailyStats struct {
	Date          string  `json:"date"`
	TotalRequests int     `json:"total_requests"`
	SuccessCount  int     `json:"success_count"`
	SuccessRate   float64 `json:"success_rate"`
	AvgScore      float64 `json:"avg_score"`
	AvgLatencyMs  int     `json:"avg_latency_ms"`
}

// LaneStats contains aggregated stats for a routing lane.
type LaneStats struct {
	Total        int     `json:"total"`
	SuccessCount int     `json:"success_count"`
	SuccessRate  float64 `json:"success_rate"`
	AvgScore     float64 `json:"avg_score"`
	ForcedCount  int     `json:"forced_count"`
}

// ModelRanking contains performance ranking for a model.
type ModelRanking struct {
	Rank        int     `json:"rank"`
	Provider    string  `json:"provider"`
	Model       string  `json:"model"`
	SampleCount int     `json:"sample_count"`
	AvgScore    float64 `json:"avg_score"`
	AvgLatencyMs int    `json:"avg_latency_ms"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

// nullString converts an empty string to sql.NullString for proper NULL handling.
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// boolToInt converts a boolean to 0/1 for SQLite.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
