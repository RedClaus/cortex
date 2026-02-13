package memory

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupEvolutionTestDB creates a test database with all required tables.
func setupEvolutionTestDB(t *testing.T) (*sql.DB, func()) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	// Create all required tables
	_, err = db.Exec(`
		CREATE TABLE strategic_memory (
			id TEXT PRIMARY KEY,
			principle TEXT NOT NULL,
			category TEXT,
			trigger_pattern TEXT,
			tier TEXT DEFAULT 'tentative',
			success_count INTEGER DEFAULT 0,
			failure_count INTEGER DEFAULT 0,
			apply_count INTEGER DEFAULT 0,
			success_rate REAL GENERATED ALWAYS AS (
				CASE WHEN (success_count + failure_count) > 0
				THEN CAST(success_count AS REAL) / (success_count + failure_count)
				ELSE 0 END
			) STORED,
			confidence REAL DEFAULT 0.5,
			source_sessions TEXT DEFAULT '[]',
			embedding BLOB,
			created_at TEXT DEFAULT (datetime('now')),
			updated_at TEXT DEFAULT (datetime('now')),
			last_applied_at TEXT,
			version INTEGER DEFAULT 1,
			parent_id TEXT,
			evolution_chain TEXT DEFAULT '[]'
		);

		CREATE TABLE memory_attributions (
			id TEXT PRIMARY KEY,
			memory_id TEXT NOT NULL,
			query_id TEXT NOT NULL,
			query_text TEXT,
			outcome TEXT CHECK (outcome IN ('success', 'failure', 'partial')),
			contribution REAL DEFAULT 0.5,
			created_at TEXT DEFAULT (datetime('now')),
			session_id TEXT
		);

		CREATE TABLE activation_logs (
			id TEXT PRIMARY KEY,
			query_id TEXT NOT NULL,
			query_text TEXT,
			memories_found TEXT,
			retrieval_type TEXT,
			latency_ms INTEGER,
			tokens_used INTEGER,
			lane TEXT,
			created_at TEXT DEFAULT (datetime('now')),
			session_id TEXT
		);

		CREATE TABLE promotion_narratives (
			id TEXT PRIMARY KEY,
			memory_id TEXT NOT NULL,
			from_tier TEXT,
			to_tier TEXT NOT NULL,
			reason TEXT NOT NULL,
			metrics TEXT,
			created_at TEXT DEFAULT (datetime('now'))
		);
	`)
	require.NoError(t, err)

	return db, func() { db.Close() }
}

// ============================================================================
// Evolution Tests
// ============================================================================

func TestCreateEvolution(t *testing.T) {
	db, cleanup := setupEvolutionTestDB(t)
	defer cleanup()

	store := NewStrategicMemoryStore(db, nil)
	ctx := context.Background()

	// Create parent memory
	parent := &StrategicMemory{
		Principle:      "Use dependency injection for testability",
		Category:       "architecture",
		TriggerPattern: "when writing service code",
		Tier:           TierProven,
		Confidence:     0.85,
		SourceSessions: []string{"session1", "session2"},
	}
	err := store.Create(ctx, parent)
	require.NoError(t, err)

	// Create evolved version
	child, err := store.CreateEvolution(ctx, parent.ID, "Prefer constructor injection over setter injection", "Refined based on new patterns")
	require.NoError(t, err)

	// Verify child properties
	assert.Equal(t, 2, child.Version)
	assert.Equal(t, parent.ID, child.ParentID)
	assert.Contains(t, child.EvolutionChain, parent.ID)
	assert.Equal(t, TierTentative, child.Tier) // New versions start tentative
	assert.Equal(t, parent.Confidence*0.9, child.Confidence)
	assert.Equal(t, parent.Category, child.Category)
}

func TestGetEvolutionHistory(t *testing.T) {
	db, cleanup := setupEvolutionTestDB(t)
	defer cleanup()

	store := NewStrategicMemoryStore(db, nil)
	ctx := context.Background()

	// Create chain: v1 -> v2 -> v3
	v1 := &StrategicMemory{Principle: "Version 1", Category: "test"}
	err := store.Create(ctx, v1)
	require.NoError(t, err)

	v2, err := store.CreateEvolution(ctx, v1.ID, "Version 2", "improvement")
	require.NoError(t, err)

	v3, err := store.CreateEvolution(ctx, v2.ID, "Version 3", "further improvement")
	require.NoError(t, err)

	// Get history from v3
	history, err := store.GetEvolutionHistory(ctx, v3.ID)
	require.NoError(t, err)

	assert.Len(t, history, 3)
	assert.Equal(t, "Version 1", history[0].Principle)
	assert.Equal(t, "Version 2", history[1].Principle)
	assert.Equal(t, "Version 3", history[2].Principle)
}

func TestGetDescendants(t *testing.T) {
	db, cleanup := setupEvolutionTestDB(t)
	defer cleanup()

	store := NewStrategicMemoryStore(db, nil)
	ctx := context.Background()

	// Create parent with multiple children
	parent := &StrategicMemory{Principle: "Parent principle", Category: "test"}
	err := store.Create(ctx, parent)
	require.NoError(t, err)

	_, err = store.CreateEvolution(ctx, parent.ID, "Child 1", "reason")
	require.NoError(t, err)
	_, err = store.CreateEvolution(ctx, parent.ID, "Child 2", "reason")
	require.NoError(t, err)

	descendants, err := store.GetDescendants(ctx, parent.ID)
	require.NoError(t, err)
	assert.Len(t, descendants, 2)
}

// ============================================================================
// Outcome Attribution Tests
// ============================================================================

func TestRecordAttribution(t *testing.T) {
	db, cleanup := setupEvolutionTestDB(t)
	defer cleanup()

	store := NewStrategicMemoryStore(db, nil)
	ctx := context.Background()

	// Create memory
	mem := &StrategicMemory{Principle: "Test principle", Category: "test"}
	err := store.Create(ctx, mem)
	require.NoError(t, err)

	// Record attribution
	attr := &OutcomeAttribution{
		MemoryID:     mem.ID,
		QueryID:      "query_123",
		QueryText:    "How do I improve performance?",
		Outcome:      "success",
		Contribution: 0.75,
		SessionID:    "session_abc",
	}
	err = store.RecordAttribution(ctx, attr)
	require.NoError(t, err)

	// Verify attribution was recorded
	attrs, err := store.GetAttributionsForMemory(ctx, mem.ID, 10)
	require.NoError(t, err)
	assert.Len(t, attrs, 1)
	assert.Equal(t, "success", attrs[0].Outcome)
	assert.Equal(t, 0.75, attrs[0].Contribution)
}

func TestRecordAttributions(t *testing.T) {
	db, cleanup := setupEvolutionTestDB(t)
	defer cleanup()

	store := NewStrategicMemoryStore(db, nil)
	ctx := context.Background()

	// Create memories
	mem1 := &StrategicMemory{Principle: "Principle 1", Category: "test"}
	mem2 := &StrategicMemory{Principle: "Principle 2", Category: "test"}
	store.Create(ctx, mem1)
	store.Create(ctx, mem2)

	// Record batch attributions
	err := store.RecordAttributions(ctx, "query_456", "Some query", []string{mem1.ID, mem2.ID}, "success", "session_xyz")
	require.NoError(t, err)

	// Verify both were recorded with equal contribution
	attrs1, _ := store.GetAttributionsForMemory(ctx, mem1.ID, 10)
	attrs2, _ := store.GetAttributionsForMemory(ctx, mem2.ID, 10)

	assert.Len(t, attrs1, 1)
	assert.Len(t, attrs2, 1)
	assert.Equal(t, 0.5, attrs1[0].Contribution) // 1/2 = 0.5
	assert.Equal(t, 0.5, attrs2[0].Contribution)
}

func TestCalculateMemoryImpact(t *testing.T) {
	db, cleanup := setupEvolutionTestDB(t)
	defer cleanup()

	store := NewStrategicMemoryStore(db, nil)
	ctx := context.Background()

	// Create memory
	mem := &StrategicMemory{Principle: "Impactful principle", Category: "test"}
	store.Create(ctx, mem)

	// Record multiple attributions
	for i := 0; i < 7; i++ {
		store.RecordAttribution(ctx, &OutcomeAttribution{
			MemoryID: mem.ID,
			QueryID:  "query_" + string(rune('a'+i)),
			Outcome:  "success",
		})
	}
	for i := 0; i < 3; i++ {
		store.RecordAttribution(ctx, &OutcomeAttribution{
			MemoryID: mem.ID,
			QueryID:  "query_fail_" + string(rune('a'+i)),
			Outcome:  "failure",
		})
	}

	// Calculate impact
	impact, err := store.CalculateMemoryImpact(ctx, mem.ID)
	require.NoError(t, err)

	assert.Equal(t, 10, impact.TotalUses)
	assert.Equal(t, 7, impact.Successes)
	assert.Equal(t, 3, impact.Failures)
	assert.Equal(t, 0.7, impact.SuccessRate)
}

// ============================================================================
// Activation Log Tests
// ============================================================================

func TestLogActivation(t *testing.T) {
	db, cleanup := setupEvolutionTestDB(t)
	defer cleanup()

	store := NewStrategicMemoryStore(db, nil)
	ctx := context.Background()

	log := &ActivationLog{
		QueryID:       "query_789",
		QueryText:     "What's the best approach for caching?",
		MemoriesFound: []string{"mem_1", "mem_2", "mem_3"},
		RetrievalType: "similarity",
		LatencyMs:     25,
		TokensUsed:    150,
		Lane:          "fast",
		SessionID:     "session_test",
	}
	err := store.LogActivation(ctx, log)
	require.NoError(t, err)

	// Verify log was recorded
	logs, err := store.GetRecentActivations(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, logs, 1)
	assert.Equal(t, "similarity", logs[0].RetrievalType)
	assert.Len(t, logs[0].MemoriesFound, 3)
	assert.Equal(t, int64(25), logs[0].LatencyMs)
}

func TestGetActivationStats(t *testing.T) {
	db, cleanup := setupEvolutionTestDB(t)
	defer cleanup()

	store := NewStrategicMemoryStore(db, nil)
	ctx := context.Background()

	// Log several activations
	for i := 0; i < 5; i++ {
		store.LogActivation(ctx, &ActivationLog{
			QueryID:       "q_" + string(rune('a'+i)),
			MemoriesFound: []string{"m1", "m2"},
			RetrievalType: "similarity",
			LatencyMs:     int64(20 + i*5),
			TokensUsed:    100,
			Lane:          "fast",
		})
	}
	for i := 0; i < 3; i++ {
		store.LogActivation(ctx, &ActivationLog{
			QueryID:       "q_smart_" + string(rune('a'+i)),
			MemoriesFound: []string{"m1"},
			RetrievalType: "fts",
			LatencyMs:     50,
			TokensUsed:    200,
			Lane:          "smart",
		})
	}

	stats, err := store.GetActivationStats(ctx, time.Now().Add(-1*time.Hour))
	require.NoError(t, err)

	assert.Equal(t, 8, stats.TotalQueries)
	assert.Equal(t, 5, stats.FastLaneCount)
	assert.Equal(t, 3, stats.SmartLaneCount)
	assert.Equal(t, 5, stats.SimilarityCount)
	assert.Equal(t, 3, stats.FTSCount)
}

func TestGetSlowActivations(t *testing.T) {
	db, cleanup := setupEvolutionTestDB(t)
	defer cleanup()

	store := NewStrategicMemoryStore(db, nil)
	ctx := context.Background()

	// Log activations with varying latencies
	store.LogActivation(ctx, &ActivationLog{QueryID: "fast", LatencyMs: 10, RetrievalType: "similarity", Lane: "fast"})
	store.LogActivation(ctx, &ActivationLog{QueryID: "medium", LatencyMs: 50, RetrievalType: "similarity", Lane: "fast"})
	store.LogActivation(ctx, &ActivationLog{QueryID: "slow1", LatencyMs: 100, RetrievalType: "similarity", Lane: "fast"})
	store.LogActivation(ctx, &ActivationLog{QueryID: "slow2", LatencyMs: 150, RetrievalType: "similarity", Lane: "fast"})

	// Get activations slower than 75ms
	slow, err := store.GetSlowActivations(ctx, 75, 10)
	require.NoError(t, err)

	assert.Len(t, slow, 2)
	assert.Equal(t, "slow2", slow[0].QueryID) // Sorted by latency DESC
}

// ============================================================================
// Promotion Narrative Tests
// ============================================================================

func TestPromoteIfEligibleWithNarrative(t *testing.T) {
	db, cleanup := setupEvolutionTestDB(t)
	defer cleanup()

	store := NewStrategicMemoryStore(db, nil)
	ctx := context.Background()

	// Create memory with enough stats for promotion
	mem := &StrategicMemory{
		Principle:      "Test principle",
		Category:       "test",
		Tier:           TierTentative,
		ApplyCount:     5,
		SuccessCount:   4,
		FailureCount:   1,
		SourceSessions: []string{"s1", "s2"},
	}
	err := store.Create(ctx, mem)
	require.NoError(t, err)

	// Attempt promotion
	thresholds := DefaultTierPromotionThresholds()
	thresholds.MinApplyCountForCandidate = 3 // Lower threshold for test
	promoted, narrative, err := store.PromoteIfEligibleWithNarrative(ctx, mem.ID, thresholds)
	require.NoError(t, err)

	assert.True(t, promoted)
	assert.NotNil(t, narrative)
	assert.Equal(t, TierTentative, narrative.FromTier)
	assert.Equal(t, TierCandidate, narrative.ToTier)
	assert.Contains(t, narrative.Reason, "Applied 5 times")
}

func TestGetPromotionHistory(t *testing.T) {
	db, cleanup := setupEvolutionTestDB(t)
	defer cleanup()

	store := NewStrategicMemoryStore(db, nil)
	ctx := context.Background()

	// Create memory
	mem := &StrategicMemory{
		Principle: "Principle for promotion",
		Category:  "test",
		Tier:      TierTentative,
	}
	store.Create(ctx, mem)

	// Simulate multiple promotions
	thresholds := DefaultTierPromotionThresholds()

	// First promotion: tentative -> candidate
	mem.ApplyCount = 5
	store.db.Exec(`UPDATE strategic_memory SET apply_count = 5 WHERE id = ?`, mem.ID)
	store.PromoteIfEligibleWithNarrative(ctx, mem.ID, thresholds)

	// Second promotion: candidate -> proven
	store.db.Exec(`UPDATE strategic_memory SET apply_count = 15, success_count = 13, failure_count = 2 WHERE id = ?`, mem.ID)
	store.PromoteIfEligibleWithNarrative(ctx, mem.ID, thresholds)

	// Get history
	history, err := store.GetPromotionHistory(ctx, mem.ID)
	require.NoError(t, err)

	assert.Len(t, history, 2)
	assert.Equal(t, TierCandidate, history[0].ToTier)
	assert.Equal(t, TierProven, history[1].ToTier)
}

func TestGetRecentPromotions(t *testing.T) {
	db, cleanup := setupEvolutionTestDB(t)
	defer cleanup()

	store := NewStrategicMemoryStore(db, nil)
	ctx := context.Background()

	// Create and promote multiple memories
	thresholds := DefaultTierPromotionThresholds()
	thresholds.MinApplyCountForCandidate = 3
	for i := 0; i < 3; i++ {
		mem := &StrategicMemory{
			Principle:  "Principle " + string(rune('A'+i)),
			Category:   "test",
			Tier:       TierTentative,
			ApplyCount: 5,
		}
		store.Create(ctx, mem)
		store.PromoteIfEligibleWithNarrative(ctx, mem.ID, thresholds)
	}

	recent, err := store.GetRecentPromotions(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, recent, 3)
}

// ============================================================================
// Integration Test
// ============================================================================

func TestStrategicEvolutionIntegration(t *testing.T) {
	db, cleanup := setupEvolutionTestDB(t)
	defer cleanup()

	store := NewStrategicMemoryStore(db, nil)
	ctx := context.Background()

	// 1. Create initial memory
	mem := &StrategicMemory{
		Principle:      "Always validate input at boundaries",
		Category:       "security",
		TriggerPattern: "when accepting external data",
		Tier:           TierTentative,
	}
	err := store.Create(ctx, mem)
	require.NoError(t, err)

	// 2. Log some activations (memory being used)
	for i := 0; i < 3; i++ {
		store.LogActivation(ctx, &ActivationLog{
			QueryID:       "query_" + string(rune('a'+i)),
			QueryText:     "How to handle user input?",
			MemoriesFound: []string{mem.ID},
			RetrievalType: "similarity",
			LatencyMs:     20,
			TokensUsed:    50,
			Lane:          "fast",
			SessionID:     "session_" + string(rune('a'+i)),
		})
	}

	// 3. Record attributions with outcomes
	store.RecordAttributions(ctx, "query_a", "How to handle user input?", []string{mem.ID}, "success", "session_a")
	store.RecordAttributions(ctx, "query_b", "Input validation approach?", []string{mem.ID}, "success", "session_b")
	store.RecordAttributions(ctx, "query_c", "Security best practices?", []string{mem.ID}, "success", "session_c")

	// 4. Sync memory stats from attributions
	err = store.SyncFromAttributions(ctx, mem.ID)
	require.NoError(t, err)

	// Verify updated stats
	updated, _ := store.Get(ctx, mem.ID)
	assert.Equal(t, 3, updated.SuccessCount)

	// 5. Attempt promotion
	thresholds := DefaultTierPromotionThresholds()
	thresholds.MinApplyCountForCandidate = 3 // Lower threshold for test
	promoted, narrative, err := store.PromoteIfEligibleWithNarrative(ctx, mem.ID, thresholds)
	require.NoError(t, err)
	assert.True(t, promoted)
	assert.Equal(t, TierCandidate, narrative.ToTier)

	// 6. Create an evolved version
	evolved, err := store.CreateEvolution(ctx, mem.ID, "Validate and sanitize input at boundaries", "Added sanitization requirement")
	require.NoError(t, err)
	assert.Equal(t, 2, evolved.Version)

	// 7. Get full evolution history
	history, _ := store.GetEvolutionHistory(ctx, evolved.ID)
	assert.Len(t, history, 2)

	// 8. Get impact analysis
	impact, _ := store.CalculateMemoryImpact(ctx, mem.ID)
	assert.Equal(t, 3, impact.TotalUses)
	assert.Equal(t, 1.0, impact.SuccessRate)

	// 9. Get activation stats
	stats, _ := store.GetActivationStats(ctx, time.Now().Add(-1*time.Hour))
	assert.Equal(t, 3, stats.TotalQueries)

	// 10. Get promotion history
	promotions, _ := store.GetPromotionHistory(ctx, mem.ID)
	assert.Len(t, promotions, 1)
}
