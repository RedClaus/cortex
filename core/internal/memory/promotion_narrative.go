// Package memory provides enhanced memory capabilities for Cortex.
// This file implements promotion narratives for CR-025-LITE.
package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PromotionNarrative explains why a memory's tier changed.
// This provides human-readable context for tier transitions.
type PromotionNarrative struct {
	ID        string     `json:"id"`
	MemoryID  string     `json:"memory_id"`
	FromTier  MemoryTier `json:"from_tier"`
	ToTier    MemoryTier `json:"to_tier"`
	Reason    string     `json:"reason"`  // Human-readable explanation
	Metrics   string     `json:"metrics"` // JSON of triggering metrics
	CreatedAt time.Time  `json:"created_at"`
}

// PromotionMetrics captures the metrics that triggered a promotion.
type PromotionMetrics struct {
	ApplyCount    int     `json:"apply_count"`
	SuccessRate   float64 `json:"success_rate"`
	SessionCount  int     `json:"session_count"`
	AgeDays       int     `json:"age_days"`
	SuccessCount  int     `json:"success_count"`
	FailureCount  int     `json:"failure_count"`
	Confidence    float64 `json:"confidence"`
}

// PromoteIfEligibleWithNarrative checks and promotes a memory, generating a narrative.
// Returns whether the memory was promoted, the narrative, and any error.
func (s *StrategicMemoryStore) PromoteIfEligibleWithNarrative(ctx context.Context, id string, thresholds TierPromotionThresholds) (promoted bool, narrative *PromotionNarrative, err error) {
	mem, err := s.Get(ctx, id)
	if err != nil {
		return false, nil, fmt.Errorf("promote with narrative: %w", err)
	}

	eligibleTier := s.CalculateEligibleTier(mem, thresholds)

	tierOrder := map[MemoryTier]int{
		TierTentative: 0,
		TierCandidate: 1,
		TierProven:    2,
		TierIdentity:  3,
	}

	currentOrder := tierOrder[mem.Tier]
	eligibleOrder := tierOrder[eligibleTier]

	if eligibleOrder <= currentOrder {
		return false, nil, nil
	}

	// Generate narrative
	narrative = s.generatePromotionNarrative(mem, mem.Tier, eligibleTier, thresholds)

	// Perform promotion
	if err := s.UpdateTier(ctx, id, eligibleTier); err != nil {
		return false, nil, fmt.Errorf("promote with narrative: update tier failed: %w", err)
	}

	// Log the narrative
	if err := s.logPromotionNarrative(ctx, narrative); err != nil {
		// Log but don't fail - promotion succeeded
	}

	return true, narrative, nil
}

// generatePromotionNarrative creates a human-readable explanation for a tier change.
func (s *StrategicMemoryStore) generatePromotionNarrative(mem *StrategicMemory, fromTier, toTier MemoryTier, thresholds TierPromotionThresholds) *PromotionNarrative {
	var reason string
	ageDays := int(time.Since(mem.CreatedAt).Hours() / 24)

	switch toTier {
	case TierCandidate:
		reason = fmt.Sprintf("Applied %d times (threshold: %d). This principle is showing early promise as an emerging pattern.",
			mem.ApplyCount, thresholds.MinApplyCountForCandidate)
	case TierProven:
		reason = fmt.Sprintf("Applied %d times with %.1f%% success rate (thresholds: %d applications, %.0f%% success). "+
			"This principle has proven reliable across multiple uses.",
			mem.ApplyCount, mem.SuccessRate*100, thresholds.MinApplyCountForProven, thresholds.MinSuccessRateForProven*100)
	case TierIdentity:
		reason = fmt.Sprintf("Applied %d times with %.1f%% success across %d sessions over %d days "+
			"(thresholds: %d applications, %.0f%% success, %d sessions, %d days). "+
			"This principle has become a core part of the system's identity.",
			mem.ApplyCount, mem.SuccessRate*100, len(mem.SourceSessions), ageDays,
			thresholds.MinApplyCountForIdentity, thresholds.MinSuccessRateForIdentity*100,
			thresholds.MinUniqueSessionsForIdentity, int(thresholds.MinAgeForIdentity.Hours()/24))
	}

	metrics := PromotionMetrics{
		ApplyCount:   mem.ApplyCount,
		SuccessRate:  mem.SuccessRate,
		SessionCount: len(mem.SourceSessions),
		AgeDays:      ageDays,
		SuccessCount: mem.SuccessCount,
		FailureCount: mem.FailureCount,
		Confidence:   mem.Confidence,
	}
	metricsJSON, _ := json.Marshal(metrics)

	return &PromotionNarrative{
		ID:        "prom_" + uuid.New().String(),
		MemoryID:  mem.ID,
		FromTier:  fromTier,
		ToTier:    toTier,
		Reason:    reason,
		Metrics:   string(metricsJSON),
		CreatedAt: time.Now(),
	}
}

// logPromotionNarrative persists a promotion narrative to the database.
func (s *StrategicMemoryStore) logPromotionNarrative(ctx context.Context, narrative *PromotionNarrative) error {
	query := `
		INSERT INTO promotion_narratives (id, memory_id, from_tier, to_tier, reason, metrics, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.ExecContext(ctx, query,
		narrative.ID, narrative.MemoryID, string(narrative.FromTier), string(narrative.ToTier),
		narrative.Reason, narrative.Metrics, narrative.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("log promotion narrative: %w", err)
	}
	return nil
}

// GetPromotionHistory returns the promotion narrative history for a memory.
func (s *StrategicMemoryStore) GetPromotionHistory(ctx context.Context, memoryID string) ([]PromotionNarrative, error) {
	query := `
		SELECT id, memory_id, from_tier, to_tier, reason, metrics, created_at
		FROM promotion_narratives
		WHERE memory_id = ?
		ORDER BY created_at ASC
	`
	rows, err := s.db.QueryContext(ctx, query, memoryID)
	if err != nil {
		return nil, fmt.Errorf("get promotion history: %w", err)
	}
	defer rows.Close()

	return scanPromotionNarratives(rows)
}

// GetRecentPromotions returns recent tier promotions across all memories.
func (s *StrategicMemoryStore) GetRecentPromotions(ctx context.Context, limit int) ([]PromotionNarrative, error) {
	query := `
		SELECT id, memory_id, from_tier, to_tier, reason, metrics, created_at
		FROM promotion_narratives
		ORDER BY created_at DESC
		LIMIT ?
	`
	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("get recent promotions: %w", err)
	}
	defer rows.Close()

	return scanPromotionNarratives(rows)
}

// GetPromotionsByTier returns promotions to a specific tier.
func (s *StrategicMemoryStore) GetPromotionsByTier(ctx context.Context, toTier MemoryTier, limit int) ([]PromotionNarrative, error) {
	query := `
		SELECT id, memory_id, from_tier, to_tier, reason, metrics, created_at
		FROM promotion_narratives
		WHERE to_tier = ?
		ORDER BY created_at DESC
		LIMIT ?
	`
	rows, err := s.db.QueryContext(ctx, query, string(toTier), limit)
	if err != nil {
		return nil, fmt.Errorf("get promotions by tier: %w", err)
	}
	defer rows.Close()

	return scanPromotionNarratives(rows)
}

// GetPromotionStats returns statistics about promotions.
func (s *StrategicMemoryStore) GetPromotionStats(ctx context.Context, since time.Time) (map[string]int, error) {
	query := `
		SELECT to_tier, COUNT(*) as count
		FROM promotion_narratives
		WHERE created_at >= ?
		GROUP BY to_tier
	`
	rows, err := s.db.QueryContext(ctx, query, since.Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("get promotion stats: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var tier string
		var count int
		if err := rows.Scan(&tier, &count); err != nil {
			continue
		}
		stats[tier] = count
	}

	return stats, rows.Err()
}

// RecordManualPromotion records a promotion that was triggered manually (e.g., by admin).
func (s *StrategicMemoryStore) RecordManualPromotion(ctx context.Context, memoryID string, fromTier, toTier MemoryTier, reason string) error {
	narrative := &PromotionNarrative{
		ID:        "prom_" + uuid.New().String(),
		MemoryID:  memoryID,
		FromTier:  fromTier,
		ToTier:    toTier,
		Reason:    fmt.Sprintf("[MANUAL] %s", reason),
		Metrics:   "{}",
		CreatedAt: time.Now(),
	}

	if err := s.UpdateTier(ctx, memoryID, toTier); err != nil {
		return fmt.Errorf("record manual promotion: update tier failed: %w", err)
	}

	return s.logPromotionNarrative(ctx, narrative)
}

// scanPromotionNarratives scans rows into PromotionNarrative structs.
func scanPromotionNarratives(rows *sql.Rows) ([]PromotionNarrative, error) {
	var narratives []PromotionNarrative
	for rows.Next() {
		var n PromotionNarrative
		var fromTier, toTier, createdAt string
		err := rows.Scan(&n.ID, &n.MemoryID, &fromTier, &toTier, &n.Reason, &n.Metrics, &createdAt)
		if err != nil {
			return nil, fmt.Errorf("scan promotion narrative: %w", err)
		}
		n.FromTier = MemoryTier(fromTier)
		n.ToTier = MemoryTier(toTier)
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			n.CreatedAt = t
		}
		narratives = append(narratives, n)
	}
	return narratives, rows.Err()
}
