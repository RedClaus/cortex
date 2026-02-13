// Package metrics provides SQLite-based metrics storage for Cortex usage tracking.
// This is used by Prism to display system metrics on the dashboard.
package metrics

import (
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// METRICS TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// RequestType categorizes the type of AI request.
type RequestType string

const (
	RequestChat    RequestType = "chat"
	RequestAgent   RequestType = "agent"
	RequestSearch  RequestType = "search"
	RequestEmbed   RequestType = "embed"
)

// RequestMetric records a single AI request.
type RequestMetric struct {
	ID          int64       `json:"id"`
	RequestType RequestType `json:"request_type"`
	Provider    string      `json:"provider"`
	Model       string      `json:"model"`
	LatencyMs   int64       `json:"latency_ms"`
	TokensIn    int         `json:"tokens_in"`
	TokensOut   int         `json:"tokens_out"`
	Success     bool        `json:"success"`
	ErrorMsg    string      `json:"error_msg,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
}

// DailyStats contains aggregated metrics for a single day.
type DailyStats struct {
	Date             string  `json:"date"` // YYYY-MM-DD
	TotalRequests    int64   `json:"total_requests"`
	SuccessfulReqs   int64   `json:"successful_requests"`
	FailedReqs       int64   `json:"failed_requests"`
	AvgLatencyMs     float64 `json:"avg_latency_ms"`
	TotalTokensIn    int64   `json:"total_tokens_in"`
	TotalTokensOut   int64   `json:"total_tokens_out"`
	LocalModelRate   float64 `json:"local_model_rate"`   // % requests using local models
	TemplateHitRate  float64 `json:"template_hit_rate"`  // % requests hitting template cache
}

// ProviderStats contains per-provider metrics.
type ProviderStats struct {
	Provider      string  `json:"provider"`
	RequestCount  int64   `json:"request_count"`
	SuccessRate   float64 `json:"success_rate"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	TotalTokensIn int64   `json:"total_tokens_in"`
	TotalTokensOut int64  `json:"total_tokens_out"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// METRICS STORE
// ═══════════════════════════════════════════════════════════════════════════════

// Store provides SQLite-backed metrics storage.
type Store struct {
	db *sql.DB
	mu sync.RWMutex

	// In-memory counters for high-frequency metrics
	requestCount    int64
	successCount    int64
	totalLatencyMs  int64
	templateHits    int64
	templateMisses  int64
	localRequests   int64
}

// NewStore creates a new metrics store using the provided database connection.
func NewStore(db *sql.DB) (*Store, error) {
	s := &Store{db: db}

	if err := s.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize metrics schema: %w", err)
	}

	return s, nil
}

// initSchema creates the metrics tables if they don't exist.
func (s *Store) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS metrics_requests (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		request_type TEXT NOT NULL,
		provider TEXT NOT NULL,
		model TEXT NOT NULL,
		latency_ms INTEGER NOT NULL,
		tokens_in INTEGER DEFAULT 0,
		tokens_out INTEGER DEFAULT 0,
		success BOOLEAN NOT NULL,
		error_msg TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_metrics_created_at ON metrics_requests(created_at);
	CREATE INDEX IF NOT EXISTS idx_metrics_provider ON metrics_requests(provider);

	CREATE TABLE IF NOT EXISTS metrics_daily (
		date TEXT PRIMARY KEY,
		total_requests INTEGER DEFAULT 0,
		successful_reqs INTEGER DEFAULT 0,
		failed_reqs INTEGER DEFAULT 0,
		total_latency_ms INTEGER DEFAULT 0,
		total_tokens_in INTEGER DEFAULT 0,
		total_tokens_out INTEGER DEFAULT 0,
		local_requests INTEGER DEFAULT 0,
		template_hits INTEGER DEFAULT 0,
		template_misses INTEGER DEFAULT 0,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`

	_, err := s.db.Exec(schema)
	return err
}

// ═══════════════════════════════════════════════════════════════════════════════
// RECORDING METHODS
// ═══════════════════════════════════════════════════════════════════════════════

// RecordRequest records a single AI request.
func (s *Store) RecordRequest(metric *RequestMetric) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Insert into requests table
	_, err := s.db.Exec(`
		INSERT INTO metrics_requests (request_type, provider, model, latency_ms, tokens_in, tokens_out, success, error_msg)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, metric.RequestType, metric.Provider, metric.Model, metric.LatencyMs,
		metric.TokensIn, metric.TokensOut, metric.Success, metric.ErrorMsg)

	if err != nil {
		return fmt.Errorf("failed to record request: %w", err)
	}

	// Update in-memory counters
	s.requestCount++
	if metric.Success {
		s.successCount++
	}
	s.totalLatencyMs += metric.LatencyMs

	// Check if local model
	if metric.Provider == "ollama" {
		s.localRequests++
	}

	// Update daily stats
	return s.updateDailyStats(metric)
}

// updateDailyStats updates the daily aggregates.
func (s *Store) updateDailyStats(metric *RequestMetric) error {
	date := time.Now().Format("2006-01-02")

	// Upsert daily stats
	_, err := s.db.Exec(`
		INSERT INTO metrics_daily (date, total_requests, successful_reqs, failed_reqs, total_latency_ms, total_tokens_in, total_tokens_out, local_requests)
		VALUES (?, 1, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(date) DO UPDATE SET
			total_requests = total_requests + 1,
			successful_reqs = successful_reqs + ?,
			failed_reqs = failed_reqs + ?,
			total_latency_ms = total_latency_ms + ?,
			total_tokens_in = total_tokens_in + ?,
			total_tokens_out = total_tokens_out + ?,
			local_requests = local_requests + ?,
			updated_at = CURRENT_TIMESTAMP
	`,
		// Initial insert values
		date, boolToInt(metric.Success), boolToInt(!metric.Success), metric.LatencyMs,
		metric.TokensIn, metric.TokensOut, boolToInt(metric.Provider == "ollama"),
		// Update values
		boolToInt(metric.Success), boolToInt(!metric.Success), metric.LatencyMs,
		metric.TokensIn, metric.TokensOut, boolToInt(metric.Provider == "ollama"),
	)

	return err
}

// RecordTemplateHit increments the template hit counter.
func (s *Store) RecordTemplateHit() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.templateHits++

	// Update daily stats
	date := time.Now().Format("2006-01-02")
	s.db.Exec(`
		INSERT INTO metrics_daily (date, template_hits)
		VALUES (?, 1)
		ON CONFLICT(date) DO UPDATE SET
			template_hits = template_hits + 1,
			updated_at = CURRENT_TIMESTAMP
	`, date)
}

// RecordTemplateMiss increments the template miss counter.
func (s *Store) RecordTemplateMiss() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.templateMisses++

	// Update daily stats
	date := time.Now().Format("2006-01-02")
	s.db.Exec(`
		INSERT INTO metrics_daily (date, template_misses)
		VALUES (?, 1)
		ON CONFLICT(date) DO UPDATE SET
			template_misses = template_misses + 1,
			updated_at = CURRENT_TIMESTAMP
	`, date)
}

// ═══════════════════════════════════════════════════════════════════════════════
// QUERY METHODS
// ═══════════════════════════════════════════════════════════════════════════════

// GetDailyStats returns stats for the specified date.
func (s *Store) GetDailyStats(date string) (*DailyStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &DailyStats{Date: date}

	err := s.db.QueryRow(`
		SELECT total_requests, successful_reqs, failed_reqs, total_latency_ms,
		       total_tokens_in, total_tokens_out, local_requests, template_hits, template_misses
		FROM metrics_daily WHERE date = ?
	`, date).Scan(
		&stats.TotalRequests, &stats.SuccessfulReqs, &stats.FailedReqs,
		&stats.AvgLatencyMs, &stats.TotalTokensIn, &stats.TotalTokensOut,
		&stats.LocalModelRate, &stats.TemplateHitRate, &stats.TemplateHitRate,
	)

	if err == sql.ErrNoRows {
		return stats, nil // Return empty stats
	}
	if err != nil {
		return nil, err
	}

	// Calculate derived metrics
	if stats.TotalRequests > 0 {
		stats.AvgLatencyMs = stats.AvgLatencyMs / float64(stats.TotalRequests)
		stats.LocalModelRate = float64(int64(stats.LocalModelRate)) / float64(stats.TotalRequests) * 100
	}

	totalTemplateReqs := int64(stats.TemplateHitRate) // Actually hits
	totalMisses := s.templateMisses // From counter
	if totalTemplateReqs+totalMisses > 0 {
		stats.TemplateHitRate = float64(totalTemplateReqs) / float64(totalTemplateReqs+totalMisses) * 100
	}

	return stats, nil
}

// GetTodayStats returns stats for today.
func (s *Store) GetTodayStats() (*DailyStats, error) {
	return s.GetDailyStats(time.Now().Format("2006-01-02"))
}

// GetProviderStats returns per-provider statistics for the last N days.
func (s *Store) GetProviderStats(days int) ([]ProviderStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02 15:04:05")

	rows, err := s.db.Query(`
		SELECT provider,
		       COUNT(*) as request_count,
		       SUM(CASE WHEN success THEN 1 ELSE 0 END) * 100.0 / COUNT(*) as success_rate,
		       AVG(latency_ms) as avg_latency,
		       SUM(tokens_in) as total_tokens_in,
		       SUM(tokens_out) as total_tokens_out
		FROM metrics_requests
		WHERE created_at >= ?
		GROUP BY provider
		ORDER BY request_count DESC
	`, since)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []ProviderStats
	for rows.Next() {
		var s ProviderStats
		if err := rows.Scan(&s.Provider, &s.RequestCount, &s.SuccessRate,
			&s.AvgLatencyMs, &s.TotalTokensIn, &s.TotalTokensOut); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}

	return stats, rows.Err()
}

// GetRecentRequests returns the most recent N requests.
func (s *Store) GetRecentRequests(limit int) ([]RequestMetric, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, request_type, provider, model, latency_ms, tokens_in, tokens_out, success, error_msg, created_at
		FROM metrics_requests
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []RequestMetric
	for rows.Next() {
		var m RequestMetric
		var errorMsg sql.NullString
		if err := rows.Scan(&m.ID, &m.RequestType, &m.Provider, &m.Model,
			&m.LatencyMs, &m.TokensIn, &m.TokensOut, &m.Success, &errorMsg, &m.CreatedAt); err != nil {
			return nil, err
		}
		if errorMsg.Valid {
			m.ErrorMsg = errorMsg.String
		}
		metrics = append(metrics, m)
	}

	return metrics, rows.Err()
}

// ═══════════════════════════════════════════════════════════════════════════════
// SUMMARY METHODS
// ═══════════════════════════════════════════════════════════════════════════════

// GetSummary returns a quick summary of current metrics.
func (s *Store) GetSummary() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	avgLatency := float64(0)
	if s.requestCount > 0 {
		avgLatency = float64(s.totalLatencyMs) / float64(s.requestCount)
	}

	successRate := float64(0)
	if s.requestCount > 0 {
		successRate = float64(s.successCount) / float64(s.requestCount) * 100
	}

	localRate := float64(0)
	if s.requestCount > 0 {
		localRate = float64(s.localRequests) / float64(s.requestCount) * 100
	}

	templateHitRate := float64(0)
	total := s.templateHits + s.templateMisses
	if total > 0 {
		templateHitRate = float64(s.templateHits) / float64(total) * 100
	}

	return map[string]interface{}{
		"total_requests":    s.requestCount,
		"success_rate":      successRate,
		"avg_latency_ms":    avgLatency,
		"local_model_rate":  localRate,
		"template_hit_rate": templateHitRate,
		"template_hits":     s.templateHits,
		"template_misses":   s.templateMisses,
	}
}

// Reset clears in-memory counters (for testing).
func (s *Store) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requestCount = 0
	s.successCount = 0
	s.totalLatencyMs = 0
	s.templateHits = 0
	s.templateMisses = 0
	s.localRequests = 0
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
