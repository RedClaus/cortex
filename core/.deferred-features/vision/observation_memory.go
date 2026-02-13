// Package vision provides unified interfaces for vision/image analysis providers.
package vision

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ObservationMemory stores screen observations for learning.
// It uses SQLite for persistent storage and supports pattern analysis.
//
// CR-023: CortexEyes - Screen Awareness & Contextual Learning
type ObservationMemory struct {
	db           *sql.DB
	maxRetention time.Duration
	mu           sync.RWMutex

	// In-memory stats
	totalObservations int64
	sessionID         string
}

// Observation represents a single screen observation.
type Observation struct {
	ID            string       `json:"id"`
	SessionID     string       `json:"session_id"`
	Context       *UserContext `json:"context"`
	Summary       string       `json:"summary"`
	Insights      []string     `json:"insights,omitempty"`
	Timestamp     time.Time    `json:"timestamp"`
	CreatedAt     time.Time    `json:"created_at"`
}

// ObservationMemoryConfig configures the observation memory.
type ObservationMemoryConfig struct {
	DBPath       string        // Path to SQLite database
	MaxRetention time.Duration // How long to keep observations (default: 30 days)
	SessionID    string        // Current session ID (auto-generated if empty)
}

// NewObservationMemory creates a new observation memory.
func NewObservationMemory(db *sql.DB, config *ObservationMemoryConfig) (*ObservationMemory, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection required")
	}

	maxRetention := 30 * 24 * time.Hour // 30 days default
	sessionID := uuid.New().String()

	if config != nil {
		if config.MaxRetention > 0 {
			maxRetention = config.MaxRetention
		}
		if config.SessionID != "" {
			sessionID = config.SessionID
		}
	}

	om := &ObservationMemory{
		db:           db,
		maxRetention: maxRetention,
		sessionID:    sessionID,
	}

	// Initialize schema
	if err := om.initSchema(); err != nil {
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return om, nil
}

// initSchema creates the necessary tables if they don't exist.
func (om *ObservationMemory) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS cortex_observations (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		activity TEXT NOT NULL,
		application TEXT,
		domain TEXT,
		content_type TEXT,
		focus_area TEXT,
		summary TEXT,
		insights TEXT,
		confidence REAL,
		timestamp DATETIME NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_observations_timestamp ON cortex_observations(timestamp);
	CREATE INDEX IF NOT EXISTS idx_observations_session ON cortex_observations(session_id);
	CREATE INDEX IF NOT EXISTS idx_observations_activity ON cortex_observations(activity);
	CREATE INDEX IF NOT EXISTS idx_observations_application ON cortex_observations(application);

	CREATE TABLE IF NOT EXISTS cortex_patterns (
		id TEXT PRIMARY KEY,
		pattern_type TEXT NOT NULL,
		time_context TEXT,
		common_apps TEXT,
		common_tasks TEXT,
		typical_flow TEXT,
		confidence REAL,
		observation_count INTEGER,
		last_updated DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS cortex_insights (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		title TEXT NOT NULL,
		content TEXT,
		relevance REAL,
		based_on TEXT,
		action_type TEXT,
		action_payload TEXT,
		shown_to_user INTEGER DEFAULT 0,
		user_accepted INTEGER,
		timestamp DATETIME NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`

	_, err := om.db.Exec(schema)
	return err
}

// Store saves an observation to the database.
func (om *ObservationMemory) Store(ctx context.Context, obs *Observation) error {
	om.mu.Lock()
	defer om.mu.Unlock()

	if obs.ID == "" {
		obs.ID = uuid.New().String()
	}
	if obs.SessionID == "" {
		obs.SessionID = om.sessionID
	}
	if obs.Timestamp.IsZero() {
		obs.Timestamp = time.Now()
	}
	if obs.CreatedAt.IsZero() {
		obs.CreatedAt = time.Now()
	}

	// Serialize insights to JSON
	insightsJSON, _ := json.Marshal(obs.Insights)

	query := `
	INSERT INTO cortex_observations
		(id, session_id, activity, application, domain, content_type, focus_area, summary, insights, confidence, timestamp, created_at)
	VALUES
		(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var activity, application, domain, contentType, focusArea string
	var confidence float64

	if obs.Context != nil {
		activity = obs.Context.Activity
		application = obs.Context.Application
		domain = obs.Context.Domain
		contentType = obs.Context.ContentType
		focusArea = obs.Context.FocusArea
		confidence = obs.Context.Confidence
	}

	_, err := om.db.ExecContext(ctx, query,
		obs.ID,
		obs.SessionID,
		activity,
		application,
		domain,
		contentType,
		focusArea,
		obs.Summary,
		string(insightsJSON),
		confidence,
		obs.Timestamp,
		obs.CreatedAt,
	)

	if err == nil {
		om.totalObservations++
	}

	return err
}

// QueryRelevant finds observations relevant to the given query.
func (om *ObservationMemory) QueryRelevant(ctx context.Context, query string, limit int) ([]*Observation, error) {
	om.mu.RLock()
	defer om.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	// Simple keyword search - a real implementation would use vector similarity
	sqlQuery := `
	SELECT id, session_id, activity, application, domain, content_type, focus_area, summary, insights, confidence, timestamp, created_at
	FROM cortex_observations
	WHERE activity LIKE ? OR application LIKE ? OR domain LIKE ? OR focus_area LIKE ? OR summary LIKE ?
	ORDER BY timestamp DESC
	LIMIT ?
	`

	searchTerm := "%" + query + "%"
	rows, err := om.db.QueryContext(ctx, sqlQuery, searchTerm, searchTerm, searchTerm, searchTerm, searchTerm, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return om.scanObservations(rows)
}

// GetRecent returns recent observations.
func (om *ObservationMemory) GetRecent(ctx context.Context, limit int) ([]*Observation, error) {
	om.mu.RLock()
	defer om.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	query := `
	SELECT id, session_id, activity, application, domain, content_type, focus_area, summary, insights, confidence, timestamp, created_at
	FROM cortex_observations
	ORDER BY timestamp DESC
	LIMIT ?
	`

	rows, err := om.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return om.scanObservations(rows)
}

// GetBySession returns observations for a specific session.
func (om *ObservationMemory) GetBySession(ctx context.Context, sessionID string, limit int) ([]*Observation, error) {
	om.mu.RLock()
	defer om.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	query := `
	SELECT id, session_id, activity, application, domain, content_type, focus_area, summary, insights, confidence, timestamp, created_at
	FROM cortex_observations
	WHERE session_id = ?
	ORDER BY timestamp DESC
	LIMIT ?
	`

	rows, err := om.db.QueryContext(ctx, query, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return om.scanObservations(rows)
}

// GetByTimeRange returns observations within a time range.
func (om *ObservationMemory) GetByTimeRange(ctx context.Context, start, end time.Time, limit int) ([]*Observation, error) {
	om.mu.RLock()
	defer om.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	query := `
	SELECT id, session_id, activity, application, domain, content_type, focus_area, summary, insights, confidence, timestamp, created_at
	FROM cortex_observations
	WHERE timestamp >= ? AND timestamp <= ?
	ORDER BY timestamp DESC
	LIMIT ?
	`

	rows, err := om.db.QueryContext(ctx, query, start, end, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return om.scanObservations(rows)
}

// scanObservations scans rows into Observation structs.
func (om *ObservationMemory) scanObservations(rows *sql.Rows) ([]*Observation, error) {
	var observations []*Observation

	for rows.Next() {
		var (
			id, sessionID, activity, application, domain, contentType, focusArea, summary, insightsJSON string
			confidence                                                                                   float64
			timestamp, createdAt                                                                         time.Time
		)

		err := rows.Scan(&id, &sessionID, &activity, &application, &domain, &contentType, &focusArea, &summary, &insightsJSON, &confidence, &timestamp, &createdAt)
		if err != nil {
			return nil, err
		}

		var insights []string
		if insightsJSON != "" {
			json.Unmarshal([]byte(insightsJSON), &insights)
		}

		obs := &Observation{
			ID:        id,
			SessionID: sessionID,
			Context: &UserContext{
				Activity:    activity,
				Application: application,
				Domain:      domain,
				ContentType: contentType,
				FocusArea:   focusArea,
				Confidence:  confidence,
				Timestamp:   timestamp,
			},
			Summary:   summary,
			Insights:  insights,
			Timestamp: timestamp,
			CreatedAt: createdAt,
		}

		observations = append(observations, obs)
	}

	return observations, rows.Err()
}

// GetActivityStats returns activity statistics.
func (om *ObservationMemory) GetActivityStats(ctx context.Context, since time.Time) (map[string]int, error) {
	om.mu.RLock()
	defer om.mu.RUnlock()

	query := `
	SELECT activity, COUNT(*) as count
	FROM cortex_observations
	WHERE timestamp >= ?
	GROUP BY activity
	ORDER BY count DESC
	`

	rows, err := om.db.QueryContext(ctx, query, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var activity string
		var count int
		if err := rows.Scan(&activity, &count); err != nil {
			return nil, err
		}
		stats[activity] = count
	}

	return stats, rows.Err()
}

// GetAppStats returns application usage statistics.
func (om *ObservationMemory) GetAppStats(ctx context.Context, since time.Time) (map[string]int, error) {
	om.mu.RLock()
	defer om.mu.RUnlock()

	query := `
	SELECT application, COUNT(*) as count
	FROM cortex_observations
	WHERE timestamp >= ?
	GROUP BY application
	ORDER BY count DESC
	`

	rows, err := om.db.QueryContext(ctx, query, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var app string
		var count int
		if err := rows.Scan(&app, &count); err != nil {
			return nil, err
		}
		stats[app] = count
	}

	return stats, rows.Err()
}

// Cleanup removes old observations beyond retention period.
func (om *ObservationMemory) Cleanup(ctx context.Context) (int64, error) {
	om.mu.Lock()
	defer om.mu.Unlock()

	cutoff := time.Now().Add(-om.maxRetention)

	result, err := om.db.ExecContext(ctx, `DELETE FROM cortex_observations WHERE timestamp < ?`, cutoff)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// Count returns the total number of observations.
func (om *ObservationMemory) Count(ctx context.Context) (int64, error) {
	om.mu.RLock()
	defer om.mu.RUnlock()

	var count int64
	err := om.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cortex_observations`).Scan(&count)
	return count, err
}

// TodayCount returns the number of observations today.
func (om *ObservationMemory) TodayCount(ctx context.Context) (int64, error) {
	om.mu.RLock()
	defer om.mu.RUnlock()

	today := time.Now().Truncate(24 * time.Hour)

	var count int64
	err := om.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cortex_observations WHERE timestamp >= ?`, today).Scan(&count)
	return count, err
}

// SessionID returns the current session ID.
func (om *ObservationMemory) SessionID() string {
	return om.sessionID
}

// Stats returns memory statistics.
type ObservationMemoryStats struct {
	TotalObservations int64  `json:"total_observations"`
	SessionID         string `json:"session_id"`
	MaxRetentionDays  int    `json:"max_retention_days"`
}

func (om *ObservationMemory) Stats() ObservationMemoryStats {
	return ObservationMemoryStats{
		TotalObservations: om.totalObservations,
		SessionID:         om.sessionID,
		MaxRetentionDays:  int(om.maxRetention.Hours() / 24),
	}
}

// StoredPattern represents a persistent pattern stored in the database.
type StoredPattern struct {
	ID               string    `json:"id"`
	PatternType      string    `json:"pattern_type"` // "daily", "weekly", "app_sequence"
	TimeContext      string    `json:"time_context"` // morning, afternoon, evening
	CommonApps       []string  `json:"common_apps"`
	CommonTasks      []string  `json:"common_tasks"`
	TypicalFlow      []string  `json:"typical_flow"`
	Confidence       float64   `json:"confidence"`
	ObservationCount int       `json:"observation_count"`
	LastUpdated      time.Time `json:"last_updated"`
	CreatedAt        time.Time `json:"created_at"`
}

// StorePattern saves or updates a detected pattern.
func (om *ObservationMemory) StorePattern(ctx context.Context, pattern *StoredPattern) error {
	om.mu.Lock()
	defer om.mu.Unlock()

	if pattern.ID == "" {
		pattern.ID = uuid.New().String()
	}
	pattern.LastUpdated = time.Now()
	if pattern.CreatedAt.IsZero() {
		pattern.CreatedAt = time.Now()
	}

	commonAppsJSON, _ := json.Marshal(pattern.CommonApps)
	commonTasksJSON, _ := json.Marshal(pattern.CommonTasks)
	typicalFlowJSON, _ := json.Marshal(pattern.TypicalFlow)

	query := `
	INSERT OR REPLACE INTO cortex_patterns
		(id, pattern_type, time_context, common_apps, common_tasks, typical_flow, confidence, observation_count, last_updated, created_at)
	VALUES
		(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := om.db.ExecContext(ctx, query,
		pattern.ID,
		pattern.PatternType,
		pattern.TimeContext,
		string(commonAppsJSON),
		string(commonTasksJSON),
		string(typicalFlowJSON),
		pattern.Confidence,
		pattern.ObservationCount,
		pattern.LastUpdated,
		pattern.CreatedAt,
	)

	return err
}

// GetPatterns retrieves patterns optionally filtered by type and time context.
func (om *ObservationMemory) GetPatterns(ctx context.Context, patternType, timeContext string) ([]*StoredPattern, error) {
	om.mu.RLock()
	defer om.mu.RUnlock()

	query := `
	SELECT id, pattern_type, time_context, common_apps, common_tasks, typical_flow, confidence, observation_count, last_updated, created_at
	FROM cortex_patterns
	WHERE 1=1
	`
	args := []interface{}{}

	if patternType != "" {
		query += " AND pattern_type = ?"
		args = append(args, patternType)
	}
	if timeContext != "" {
		query += " AND time_context = ?"
		args = append(args, timeContext)
	}

	query += " ORDER BY confidence DESC, observation_count DESC"

	rows, err := om.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var patterns []*StoredPattern
	for rows.Next() {
		var (
			id, patternType, timeContext                     string
			commonAppsJSON, commonTasksJSON, typicalFlowJSON string
			confidence                                       float64
			obsCount                                         int
			lastUpdated, createdAt                           time.Time
		)

		err := rows.Scan(&id, &patternType, &timeContext, &commonAppsJSON, &commonTasksJSON, &typicalFlowJSON, &confidence, &obsCount, &lastUpdated, &createdAt)
		if err != nil {
			return nil, err
		}

		var commonApps, commonTasks, typicalFlow []string
		json.Unmarshal([]byte(commonAppsJSON), &commonApps)
		json.Unmarshal([]byte(commonTasksJSON), &commonTasks)
		json.Unmarshal([]byte(typicalFlowJSON), &typicalFlow)

		patterns = append(patterns, &StoredPattern{
			ID:               id,
			PatternType:      patternType,
			TimeContext:      timeContext,
			CommonApps:       commonApps,
			CommonTasks:      commonTasks,
			TypicalFlow:      typicalFlow,
			Confidence:       confidence,
			ObservationCount: obsCount,
			LastUpdated:      lastUpdated,
			CreatedAt:        createdAt,
		})
	}

	return patterns, rows.Err()
}

// DetectPatterns analyzes recent observations and returns detected patterns.
func (om *ObservationMemory) DetectPatterns(ctx context.Context, window time.Duration, minObservations int) ([]*StoredPattern, error) {
	// Get observations within window
	since := time.Now().Add(-window)
	observations, err := om.GetByTimeRange(ctx, since, time.Now(), 500)
	if err != nil {
		return nil, err
	}

	if len(observations) < minObservations {
		return nil, nil // Not enough data
	}

	patterns := make([]*StoredPattern, 0)

	// Detect daily patterns by time of day
	timePatterns := om.detectTimePatterns(observations)
	patterns = append(patterns, timePatterns...)

	// Detect app sequence patterns
	seqPatterns := om.detectSequencePatterns(observations)
	patterns = append(patterns, seqPatterns...)

	return patterns, nil
}

// detectTimePatterns finds patterns based on time of day.
func (om *ObservationMemory) detectTimePatterns(observations []*Observation) []*StoredPattern {
	patterns := make([]*StoredPattern, 0)

	// Group by time of day
	timeGroups := map[string][]*Observation{
		"morning":   {},
		"afternoon": {},
		"evening":   {},
		"night":     {},
	}

	for _, obs := range observations {
		hour := obs.Timestamp.Hour()
		var timeCtx string
		switch {
		case hour >= 5 && hour < 12:
			timeCtx = "morning"
		case hour >= 12 && hour < 17:
			timeCtx = "afternoon"
		case hour >= 17 && hour < 21:
			timeCtx = "evening"
		default:
			timeCtx = "night"
		}
		timeGroups[timeCtx] = append(timeGroups[timeCtx], obs)
	}

	// Create patterns for each time group with enough data
	for timeCtx, group := range timeGroups {
		if len(group) < 5 {
			continue
		}

		appCounts := make(map[string]int)
		taskCounts := make(map[string]int)

		for _, obs := range group {
			if obs.Context != nil {
				appCounts[obs.Context.Application]++
				taskCounts[obs.Context.Activity]++
			}
		}

		commonApps := topNKeys(appCounts, 5)
		commonTasks := topNKeys(taskCounts, 5)

		if len(commonApps) > 0 {
			pattern := &StoredPattern{
				ID:               uuid.New().String(),
				PatternType:      "daily",
				TimeContext:      timeCtx,
				CommonApps:       commonApps,
				CommonTasks:      commonTasks,
				Confidence:       float64(len(group)) / float64(len(observations)),
				ObservationCount: len(group),
			}
			patterns = append(patterns, pattern)
		}
	}

	return patterns
}

// detectSequencePatterns finds common app/task sequences.
func (om *ObservationMemory) detectSequencePatterns(observations []*Observation) []*StoredPattern {
	patterns := make([]*StoredPattern, 0)

	// Track transitions
	transitions := make(map[string]int) // "app1->app2" -> count

	for i := 1; i < len(observations); i++ {
		prev := observations[i-1]
		curr := observations[i]

		if prev.Context != nil && curr.Context != nil {
			prevApp := prev.Context.Application
			currApp := curr.Context.Application

			if prevApp != currApp && prevApp != "" && currApp != "" {
				key := prevApp + "->" + currApp
				transitions[key]++
			}
		}
	}

	// Find frequent transitions
	commonTransitions := topNKeys(transitions, 5)
	if len(commonTransitions) >= 3 {
		pattern := &StoredPattern{
			ID:               uuid.New().String(),
			PatternType:      "app_sequence",
			TypicalFlow:      commonTransitions,
			Confidence:       0.7,
			ObservationCount: len(observations),
		}
		patterns = append(patterns, pattern)
	}

	return patterns
}

// GetDomainStats returns domain usage statistics.
func (om *ObservationMemory) GetDomainStats(ctx context.Context, since time.Time) (map[string]int, error) {
	om.mu.RLock()
	defer om.mu.RUnlock()

	query := `
	SELECT domain, COUNT(*) as count
	FROM cortex_observations
	WHERE timestamp >= ? AND domain != ''
	GROUP BY domain
	ORDER BY count DESC
	`

	rows, err := om.db.QueryContext(ctx, query, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var domain string
		var count int
		if err := rows.Scan(&domain, &count); err != nil {
			return nil, err
		}
		stats[domain] = count
	}

	return stats, rows.Err()
}

// topNKeys returns the top N keys from a map sorted by value.
func topNKeys(m map[string]int, n int) []string {
	type kv struct {
		key   string
		value int
	}

	sorted := make([]kv, 0, len(m))
	for k, v := range m {
		sorted = append(sorted, kv{k, v})
	}

	// Simple bubble sort for small n
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].value > sorted[i].value {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	result := make([]string, 0, n)
	for i := 0; i < len(sorted) && i < n; i++ {
		result = append(result, sorted[i].key)
	}
	return result
}
