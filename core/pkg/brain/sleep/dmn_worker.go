// Package sleep implements the sleep cycle self-improvement system.
// This file implements the DMN (Default Mode Network) Worker for cold-path learning.
// The DMN runs during sleep cycles to aggregate routing outcomes and promote strategic memories.
package sleep

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// DMNConfig holds configuration for the DMN Worker.
type DMNConfig struct {
	// EnableOutcomeAggregation controls whether routing outcome aggregation runs.
	EnableOutcomeAggregation bool `yaml:"enable_outcome_aggregation"`

	// EnableTierPromotion controls whether strategic memory tier promotion runs.
	EnableTierPromotion bool `yaml:"enable_tier_promotion"`

	// OutcomeAggregationBatchSize is the max number of task types to process per cycle.
	OutcomeAggregationBatchSize int `yaml:"outcome_aggregation_batch_size"`

	// TierPromotionBatchSize is the max number of memories to check per cycle.
	TierPromotionBatchSize int `yaml:"tier_promotion_batch_size"`

	// MinSamplesForAggregation is the minimum samples needed for model ranking.
	MinSamplesForAggregation int `yaml:"min_samples_for_aggregation"`

	// TierPromotionThresholds defines when memories are promoted between tiers.
	TierPromotionThresholds TierPromotionConfig `yaml:"tier_promotion_thresholds"`
}

// TierPromotionConfig mirrors the strategic memory promotion thresholds.
type TierPromotionConfig struct {
	MinApplyCountForCandidate    int           `yaml:"min_apply_count_candidate"`
	MinApplyCountForProven       int           `yaml:"min_apply_count_proven"`
	MinSuccessRateForProven      float64       `yaml:"min_success_rate_proven"`
	MinApplyCountForIdentity     int           `yaml:"min_apply_count_identity"`
	MinSuccessRateForIdentity    float64       `yaml:"min_success_rate_identity"`
	MinUniqueSessionsForIdentity int           `yaml:"min_unique_sessions_identity"`
	MinAgeForIdentity            time.Duration `yaml:"min_age_identity"`
}

// DefaultDMNConfig returns sensible defaults for the DMN Worker.
func DefaultDMNConfig() DMNConfig {
	return DMNConfig{
		EnableOutcomeAggregation:    true,
		EnableTierPromotion:         true,
		OutcomeAggregationBatchSize: 50,
		TierPromotionBatchSize:      100,
		MinSamplesForAggregation:    5,
		TierPromotionThresholds: TierPromotionConfig{
			MinApplyCountForCandidate:    3,
			MinApplyCountForProven:       10,
			MinSuccessRateForProven:      0.80,
			MinApplyCountForIdentity:     25,
			MinSuccessRateForIdentity:    0.90,
			MinUniqueSessionsForIdentity: 5,
			MinAgeForIdentity:            30 * 24 * time.Hour,
		},
	}
}

// DMNResult holds the results of a DMN Worker cycle.
type DMNResult struct {
	// Outcome aggregation results
	TaskTypesProcessed int              `json:"task_types_processed"`
	ModelRankings      []ModelRanking   `json:"model_rankings,omitempty"`
	AggregationErrors  []string         `json:"aggregation_errors,omitempty"`

	// Tier promotion results
	MemoriesChecked  int                `json:"memories_checked"`
	MemoriesPromoted int                `json:"memories_promoted"`
	Promotions       []PromotionRecord  `json:"promotions,omitempty"`
	PromotionErrors  []string           `json:"promotion_errors,omitempty"`

	// Timing
	Duration time.Duration `json:"duration"`
}

// ModelRanking represents the ranking of a model for a specific task type.
type ModelRanking struct {
	TaskType        string  `json:"task_type"`
	Rank            int     `json:"rank"`
	Provider        string  `json:"provider"`
	Model           string  `json:"model"`
	SuccessRate     float64 `json:"success_rate"`
	SampleCount     int     `json:"sample_count"`
	AvgLatencyMs    int     `json:"avg_latency_ms"`
}

// PromotionRecord records a memory tier promotion event.
type PromotionRecord struct {
	MemoryID string `json:"memory_id"`
	OldTier  string `json:"old_tier"`
	NewTier  string `json:"new_tier"`
	Reason   string `json:"reason"`
}

// DMNWorker performs cold-path learning tasks during sleep cycles.
// It aggregates routing outcomes and promotes strategic memories based on performance.
type DMNWorker struct {
	config       DMNConfig
	db           *sql.DB
	log          Logger

	// Optional stores for advanced functionality
	strategicStore StrategicStore
	outcomeLogger  OutcomeLogger
	linkStore      LinkStore
}

// StrategicStore interface for strategic memory operations.
// This allows the DMN worker to operate without direct memory package dependency.
type StrategicStore interface {
	// GetByTier returns strategic memories filtered by tier.
	GetByTier(ctx context.Context, tier string, limit int) ([]StrategicMemoryInfo, error)

	// PromoteIfEligible checks and promotes a memory if it meets threshold criteria.
	PromoteIfEligible(ctx context.Context, id string, thresholds TierPromotionConfig) (promoted bool, newTier string, err error)
}

// StrategicMemoryInfo is a minimal view of strategic memory for DMN operations.
type StrategicMemoryInfo struct {
	ID           string    `json:"id"`
	Principle    string    `json:"principle"`
	Tier         string    `json:"tier"`
	ApplyCount   int       `json:"apply_count"`
	SuccessRate  float64   `json:"success_rate"`
	CreatedAt    time.Time `json:"created_at"`
}

// OutcomeLogger interface for routing outcome operations.
type OutcomeLogger interface {
	// GetTopPerformingModels returns the best performing models for a task type.
	GetTopPerformingModels(ctx context.Context, taskType string, limit int) ([]ModelRankingInfo, error)

	// GetLaneDistribution returns the distribution of requests across lanes.
	GetLaneDistribution(ctx context.Context, days int) (map[string]LaneStatsInfo, error)
}

// ModelRankingInfo is model performance data from the outcome logger.
type ModelRankingInfo struct {
	Rank         int     `json:"rank"`
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	SampleCount  int     `json:"sample_count"`
	AvgScore     float64 `json:"avg_score"`
	AvgLatencyMs int     `json:"avg_latency_ms"`
}

// LaneStatsInfo is routing lane statistics.
type LaneStatsInfo struct {
	Total        int     `json:"total"`
	SuccessCount int     `json:"success_count"`
	SuccessRate  float64 `json:"success_rate"`
	AvgScore     float64 `json:"avg_score"`
	ForcedCount  int     `json:"forced_count"`
}

// LinkStore interface for routing edge operations.
type LinkStore interface {
	// GetDistinctTaskTypes returns all unique task types with routing data.
	GetDistinctTaskTypes(ctx context.Context) ([]string, error)

	// GetRoutingKnowledge aggregates routing knowledge for a task type.
	GetRoutingKnowledge(ctx context.Context, taskType string) (*RoutingKnowledgeInfo, error)

	// UpdateRoutingEdge updates stats for an existing edge.
	UpdateRoutingEdge(ctx context.Context, provider, model, taskType string, success bool, latencyMs int) error
}

// RoutingKnowledgeInfo aggregates routing performance data for a task type.
type RoutingKnowledgeInfo struct {
	TaskType        string  `json:"task_type"`
	BestModel       string  `json:"best_model"`
	BestProvider    string  `json:"best_provider"`
	BestSuccessRate float64 `json:"best_success_rate"`
	TotalSamples    int     `json:"total_samples"`
}

// NewDMNWorker creates a new DMN Worker instance.
func NewDMNWorker(config DMNConfig, db *sql.DB, log Logger) *DMNWorker {
	if log == nil {
		log = &noopLogger{}
	}
	return &DMNWorker{
		config: config,
		db:     db,
		log:    log,
	}
}

// SetStrategicStore sets the strategic memory store for tier promotion.
func (w *DMNWorker) SetStrategicStore(store StrategicStore) {
	w.strategicStore = store
}

// SetOutcomeLogger sets the outcome logger for routing analysis.
func (w *DMNWorker) SetOutcomeLogger(logger OutcomeLogger) {
	w.outcomeLogger = logger
}

// SetLinkStore sets the link store for routing edge operations.
func (w *DMNWorker) SetLinkStore(store LinkStore) {
	w.linkStore = store
}

// Run executes all DMN Worker tasks.
// This is the main entry point called by the SleepManager during sleep cycles.
// COLD PATH ONLY - never called in hot path.
func (w *DMNWorker) Run(ctx context.Context) (*DMNResult, error) {
	startTime := time.Now()
	result := &DMNResult{}

	w.log.Debug("[DMN] Starting DMN Worker cycle")

	// Check for cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Phase 1: Aggregate routing outcomes (lower priority)
	if w.config.EnableOutcomeAggregation {
		w.log.Debug("[DMN] Phase 1: Aggregating routing outcomes")
		if err := w.aggregateRoutingOutcomes(ctx, result); err != nil {
			if ctx.Err() != nil {
				return result, ctx.Err()
			}
			w.log.Warn("[DMN] Outcome aggregation had errors: %v", err)
			result.AggregationErrors = append(result.AggregationErrors, err.Error())
		}
	}

	// Check for cancellation between phases
	select {
	case <-ctx.Done():
		result.Duration = time.Since(startTime)
		return result, ctx.Err()
	default:
	}

	// Phase 2: Promote eligible strategic memories (lower priority)
	if w.config.EnableTierPromotion {
		w.log.Debug("[DMN] Phase 2: Promoting eligible strategic memories")
		if err := w.promoteEligibleMemories(ctx, result); err != nil {
			if ctx.Err() != nil {
				result.Duration = time.Since(startTime)
				return result, ctx.Err()
			}
			w.log.Warn("[DMN] Tier promotion had errors: %v", err)
			result.PromotionErrors = append(result.PromotionErrors, err.Error())
		}
	}

	result.Duration = time.Since(startTime)

	w.log.Info("[DMN] DMN Worker cycle complete: task_types=%d, memories_checked=%d, memories_promoted=%d, duration=%v",
		result.TaskTypesProcessed,
		result.MemoriesChecked,
		result.MemoriesPromoted,
		result.Duration)

	return result, nil
}

// aggregateRoutingOutcomes aggregates routing performance data from conversation logs.
// It calculates model rankings per task type and updates routing knowledge in the KG.
// COLD PATH ONLY - batch operation for efficiency.
func (w *DMNWorker) aggregateRoutingOutcomes(ctx context.Context, result *DMNResult) error {
	// Get distinct task types from routing data
	taskTypes, err := w.getDistinctTaskTypes(ctx)
	if err != nil {
		return fmt.Errorf("get distinct task types: %w", err)
	}

	w.log.Debug("[DMN] Found %d distinct task types for aggregation", len(taskTypes))

	// Limit to batch size
	if len(taskTypes) > w.config.OutcomeAggregationBatchSize {
		taskTypes = taskTypes[:w.config.OutcomeAggregationBatchSize]
	}

	for _, taskType := range taskTypes {
		// Check for cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Get model rankings for this task type
		rankings, err := w.getModelRankingsForTask(ctx, taskType)
		if err != nil {
			w.log.Warn("[DMN] Failed to get rankings for task type %s: %v", taskType, err)
			result.AggregationErrors = append(result.AggregationErrors,
				fmt.Sprintf("task %s: %v", taskType, err))
			continue
		}

		// Filter out rankings with insufficient samples
		var validRankings []ModelRanking
		for _, r := range rankings {
			if r.SampleCount >= w.config.MinSamplesForAggregation {
				validRankings = append(validRankings, r)
			}
		}

		if len(validRankings) > 0 {
			result.ModelRankings = append(result.ModelRankings, validRankings...)
			w.log.Debug("[DMN] Task type %s: %d valid model rankings", taskType, len(validRankings))
		}

		result.TaskTypesProcessed++
	}

	return nil
}

// getDistinctTaskTypes returns all unique task types with routing data.
// Falls back to routing_lane if task_type is not populated.
func (w *DMNWorker) getDistinctTaskTypes(ctx context.Context) ([]string, error) {
	// First try the LinkStore interface
	if w.linkStore != nil {
		return w.linkStore.GetDistinctTaskTypes(ctx)
	}

	// Fallback to direct SQL query
	if w.db == nil {
		return nil, fmt.Errorf("no database connection available")
	}

	// First try routing_edges table
	query := `
		SELECT DISTINCT task_type
		FROM routing_edges
		WHERE task_type IS NOT NULL AND task_type != ''
		ORDER BY task_type
	`

	rows, err := w.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query distinct task types: %w", err)
	}
	defer rows.Close()

	var taskTypes []string
	for rows.Next() {
		var taskType string
		if err := rows.Scan(&taskType); err != nil {
			continue
		}
		taskTypes = append(taskTypes, taskType)
	}

	// If no task types found, fall back to routing_lane from conversation_logs
	if len(taskTypes) == 0 {
		laneQuery := `
			SELECT DISTINCT routing_lane
			FROM conversation_logs
			WHERE routing_lane IS NOT NULL AND routing_lane != ''
			ORDER BY routing_lane
		`
		laneRows, err := w.db.QueryContext(ctx, laneQuery)
		if err != nil {
			return taskTypes, nil // Return empty, not an error
		}
		defer laneRows.Close()

		for laneRows.Next() {
			var lane string
			if err := laneRows.Scan(&lane); err != nil {
				continue
			}
			taskTypes = append(taskTypes, lane)
		}
	}

	return taskTypes, rows.Err()
}

// getModelRankingsForTask returns model performance rankings for a specific task type.
// taskType can be an actual task type or a routing_lane (fast/smart).
func (w *DMNWorker) getModelRankingsForTask(ctx context.Context, taskType string) ([]ModelRanking, error) {
	// First try the OutcomeLogger interface
	if w.outcomeLogger != nil {
		infos, err := w.outcomeLogger.GetTopPerformingModels(ctx, taskType, 10)
		if err != nil {
			return nil, err
		}

		var rankings []ModelRanking
		for i, info := range infos {
			rankings = append(rankings, ModelRanking{
				TaskType:     taskType,
				Rank:         i + 1,
				Provider:     info.Provider,
				Model:        info.Model,
				SuccessRate:  info.AvgScore,
				SampleCount:  info.SampleCount,
				AvgLatencyMs: info.AvgLatencyMs,
			})
		}
		return rankings, nil
	}

	// Fallback to direct SQL query
	if w.db == nil {
		return nil, fmt.Errorf("no database connection available")
	}

	// First try routing_edges table
	query := `
		SELECT
			provider, model,
			success_count, failure_count, total_latency_ms
		FROM routing_edges
		WHERE task_type = ?
		ORDER BY
			CAST(success_count AS REAL) / NULLIF(success_count + failure_count, 0) DESC,
			CAST(total_latency_ms AS REAL) / NULLIF(success_count + failure_count, 0) ASC
		LIMIT 10
	`

	rows, err := w.db.QueryContext(ctx, query, taskType)
	if err != nil {
		return nil, fmt.Errorf("query model rankings: %w", err)
	}
	defer rows.Close()

	var rankings []ModelRanking
	rank := 1
	for rows.Next() {
		var provider, model string
		var successCount, failureCount, totalLatencyMs int

		if err := rows.Scan(&provider, &model, &successCount, &failureCount, &totalLatencyMs); err != nil {
			continue
		}

		sampleCount := successCount + failureCount
		var successRate float64
		var avgLatencyMs int
		if sampleCount > 0 {
			successRate = float64(successCount) / float64(sampleCount)
			avgLatencyMs = totalLatencyMs / sampleCount
		}

		rankings = append(rankings, ModelRanking{
			TaskType:     taskType,
			Rank:         rank,
			Provider:     provider,
			Model:        model,
			SuccessRate:  successRate,
			SampleCount:  sampleCount,
			AvgLatencyMs: avgLatencyMs,
		})
		rank++
	}

	// If no results from routing_edges, try conversation_logs directly
	// This handles the case where taskType is actually a routing_lane (fast/smart)
	if len(rankings) == 0 {
		convQuery := `
			SELECT
				provider, model,
				COUNT(*) as sample_count,
				SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END) as success_count,
				AVG(duration_ms) as avg_latency_ms,
				AVG(outcome_score) as avg_score
			FROM conversation_logs
			WHERE routing_lane = ? AND provider IS NOT NULL AND provider != '' AND model IS NOT NULL AND model != ''
			GROUP BY provider, model
			ORDER BY avg_score DESC, avg_latency_ms ASC
			LIMIT 10
		`

		convRows, err := w.db.QueryContext(ctx, convQuery, taskType)
		if err != nil {
			return rankings, nil // Return empty, not an error
		}
		defer convRows.Close()

		for convRows.Next() {
			var provider, model string
			var sampleCount, successCount int
			var avgLatencyMs, avgScore float64

			if err := convRows.Scan(&provider, &model, &sampleCount, &successCount, &avgLatencyMs, &avgScore); err != nil {
				continue
			}

			var successRate float64
			if sampleCount > 0 {
				successRate = float64(successCount) / float64(sampleCount)
			}

			rankings = append(rankings, ModelRanking{
				TaskType:     taskType,
				Rank:         rank,
				Provider:     provider,
				Model:        model,
				SuccessRate:  successRate,
				SampleCount:  sampleCount,
				AvgLatencyMs: int(avgLatencyMs),
			})
			rank++
		}
	}

	return rankings, rows.Err()
}

// promoteEligibleMemories checks strategic memories in lower tiers and promotes eligible ones.
// This implements tier progression: tentative -> candidate -> proven -> identity
// COLD PATH ONLY - batch operation for efficiency.
func (w *DMNWorker) promoteEligibleMemories(ctx context.Context, result *DMNResult) error {
	// Define tiers to check in order (skip identity tier as it's already highest)
	tiersToCheck := []string{"tentative", "candidate", "proven"}

	for _, tier := range tiersToCheck {
		// Check for cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Get memories in this tier
		memories, err := w.getMemoriesByTier(ctx, tier, w.config.TierPromotionBatchSize)
		if err != nil {
			w.log.Warn("[DMN] Failed to get %s tier memories: %v", tier, err)
			result.PromotionErrors = append(result.PromotionErrors,
				fmt.Sprintf("tier %s: %v", tier, err))
			continue
		}

		w.log.Debug("[DMN] Checking %d memories in %s tier", len(memories), tier)

		for _, mem := range memories {
			result.MemoriesChecked++

			// Check for cancellation
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// Attempt promotion
			promoted, newTier, err := w.checkAndPromote(ctx, mem)
			if err != nil {
				w.log.Debug("[DMN] Failed to check promotion for %s: %v", mem.ID, err)
				continue
			}

			if promoted {
				result.MemoriesPromoted++
				result.Promotions = append(result.Promotions, PromotionRecord{
					MemoryID: mem.ID,
					OldTier:  tier,
					NewTier:  newTier,
					Reason:   fmt.Sprintf("Met promotion criteria for %s tier", newTier),
				})
				w.log.Info("[DMN] Promoted memory %s: %s -> %s", mem.ID, tier, newTier)
			}
		}
	}

	return nil
}

// getMemoriesByTier returns strategic memories in a specific tier.
func (w *DMNWorker) getMemoriesByTier(ctx context.Context, tier string, limit int) ([]StrategicMemoryInfo, error) {
	// First try the StrategicStore interface
	if w.strategicStore != nil {
		return w.strategicStore.GetByTier(ctx, tier, limit)
	}

	// Fallback to direct SQL query
	if w.db == nil {
		return nil, fmt.Errorf("no database connection available")
	}

	query := `
		SELECT id, principle, tier, apply_count, success_rate, created_at
		FROM strategic_memory
		WHERE tier = ?
		ORDER BY apply_count DESC, created_at ASC
		LIMIT ?
	`

	rows, err := w.db.QueryContext(ctx, query, tier, limit)
	if err != nil {
		return nil, fmt.Errorf("query memories by tier: %w", err)
	}
	defer rows.Close()

	var memories []StrategicMemoryInfo
	for rows.Next() {
		var mem StrategicMemoryInfo
		var createdAtStr string

		if err := rows.Scan(&mem.ID, &mem.Principle, &mem.Tier, &mem.ApplyCount, &mem.SuccessRate, &createdAtStr); err != nil {
			continue
		}

		// Parse created_at
		mem.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
		if mem.CreatedAt.IsZero() {
			mem.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr)
		}

		memories = append(memories, mem)
	}

	return memories, rows.Err()
}

// checkAndPromote checks if a memory should be promoted and performs the promotion.
func (w *DMNWorker) checkAndPromote(ctx context.Context, mem StrategicMemoryInfo) (bool, string, error) {
	// First try the StrategicStore interface
	if w.strategicStore != nil {
		return w.strategicStore.PromoteIfEligible(ctx, mem.ID, w.config.TierPromotionThresholds)
	}

	// Fallback to direct SQL-based promotion
	if w.db == nil {
		return false, "", fmt.Errorf("no database connection available")
	}

	// Get full memory details for promotion check
	query := `
		SELECT id, tier, apply_count, success_rate, source_sessions, created_at
		FROM strategic_memory
		WHERE id = ?
	`

	var id, tier string
	var applyCount int
	var successRate float64
	var sourceSessionsJSON sql.NullString
	var createdAtStr string

	err := w.db.QueryRowContext(ctx, query, mem.ID).Scan(
		&id, &tier, &applyCount, &successRate, &sourceSessionsJSON, &createdAtStr,
	)
	if err != nil {
		return false, "", fmt.Errorf("get memory details: %w", err)
	}

	// Parse created_at
	var createdAt time.Time
	createdAt, _ = time.Parse(time.RFC3339, createdAtStr)
	if createdAt.IsZero() {
		createdAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr)
	}

	// Count unique sessions
	uniqueSessions := 0
	if sourceSessionsJSON.Valid && sourceSessionsJSON.String != "" {
		// Simple count of sessions in JSON array
		uniqueSessions = countJSONArrayElements(sourceSessionsJSON.String)
	}

	// Calculate eligible tier
	eligibleTier := w.calculateEligibleTier(applyCount, successRate, uniqueSessions, createdAt)

	// Check if eligible tier is higher than current
	tierOrder := map[string]int{
		"tentative": 0,
		"candidate": 1,
		"proven":    2,
		"identity":  3,
	}

	currentOrder := tierOrder[tier]
	eligibleOrder := tierOrder[eligibleTier]

	if eligibleOrder <= currentOrder {
		return false, tier, nil
	}

	// Perform promotion
	updateQuery := `
		UPDATE strategic_memory
		SET tier = ?, updated_at = ?
		WHERE id = ?
	`

	_, err = w.db.ExecContext(ctx, updateQuery, eligibleTier, time.Now().Format(time.RFC3339), mem.ID)
	if err != nil {
		return false, "", fmt.Errorf("update tier: %w", err)
	}

	return true, eligibleTier, nil
}

// calculateEligibleTier determines what tier a memory should be promoted to.
func (w *DMNWorker) calculateEligibleTier(applyCount int, successRate float64, uniqueSessions int, createdAt time.Time) string {
	thresholds := w.config.TierPromotionThresholds

	// Check for Identity tier eligibility (highest tier)
	if applyCount >= thresholds.MinApplyCountForIdentity &&
		successRate >= thresholds.MinSuccessRateForIdentity &&
		uniqueSessions >= thresholds.MinUniqueSessionsForIdentity &&
		time.Since(createdAt) >= thresholds.MinAgeForIdentity {
		return "identity"
	}

	// Check for Proven tier eligibility
	if applyCount >= thresholds.MinApplyCountForProven &&
		successRate >= thresholds.MinSuccessRateForProven {
		return "proven"
	}

	// Check for Candidate tier eligibility
	if applyCount >= thresholds.MinApplyCountForCandidate {
		return "candidate"
	}

	// Default to Tentative
	return "tentative"
}

// countJSONArrayElements is a simple helper to count elements in a JSON array string.
func countJSONArrayElements(jsonStr string) int {
	if jsonStr == "" || jsonStr == "[]" || jsonStr == "null" {
		return 0
	}
	// Simple count of commas + 1 for non-empty arrays
	// This is a rough estimate, proper JSON parsing would be more accurate
	count := 1
	for _, c := range jsonStr {
		if c == ',' {
			count++
		}
	}
	return count
}
