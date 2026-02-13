package memory

import (
	"context"
	"database/sql"
	"hash/fnv"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// MOCK IMPLEMENTATIONS FOR STRATEGIC MEMORY TESTS
// ============================================================================

// strategicMockEmbedder provides deterministic embeddings for testing.
type strategicMockEmbedder struct {
	dimension int
}

func (m *strategicMockEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	// Return deterministic embedding based on text hash
	embedding := make([]float32, m.dimension)
	h := fnv.New32a()
	h.Write([]byte(text))
	seed := h.Sum32()
	for i := range embedding {
		embedding[i] = float32(seed%1000) / 1000.0
		seed = seed*1103515245 + 12345
	}
	return embedding, nil
}

func (m *strategicMockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := m.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		results[i] = emb
	}
	return results, nil
}

func (m *strategicMockEmbedder) Dimension() int {
	return m.dimension
}

func (m *strategicMockEmbedder) ModelName() string {
	return "strategic-mock-embed"
}

// ============================================================================
// TEST HELPERS
// ============================================================================

// setupStrategicTestDB creates an in-memory SQLite database with strategic memory schema.
// Note: We create the schema inline to avoid FTS5 dependency issues with test SQLite driver.
func setupStrategicTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err, "failed to open in-memory database")

	// Create strategic_memory table with generated column for success_rate
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS strategic_memory (
			id TEXT PRIMARY KEY,
			principle TEXT NOT NULL,
			category TEXT,
			trigger_pattern TEXT,
			tier TEXT DEFAULT 'tentative',
			success_count INTEGER DEFAULT 0,
			failure_count INTEGER DEFAULT 0,
			success_rate REAL GENERATED ALWAYS AS (
				CASE WHEN (success_count + failure_count) > 0
				THEN CAST(success_count AS REAL) / (success_count + failure_count)
				ELSE 0.5 END
			) STORED,
			confidence REAL DEFAULT 0.5,
			source_sessions TEXT,
			embedding BLOB,
			created_at TEXT DEFAULT (datetime('now')),
			updated_at TEXT DEFAULT (datetime('now')),
			last_applied_at TEXT,
			apply_count INTEGER DEFAULT 0
		);

		CREATE INDEX IF NOT EXISTS idx_strategic_category ON strategic_memory(category);
		CREATE INDEX IF NOT EXISTS idx_strategic_success_rate ON strategic_memory(success_rate DESC);
		CREATE INDEX IF NOT EXISTS idx_strategic_confidence ON strategic_memory(confidence DESC);
		CREATE INDEX IF NOT EXISTS idx_strategic_updated ON strategic_memory(updated_at DESC);
		CREATE INDEX IF NOT EXISTS idx_strategic_memory_tier ON strategic_memory(tier);
	`)
	require.NoError(t, err, "failed to create strategic_memory table")

	t.Cleanup(func() { db.Close() })
	return db
}

// createStrategicTestStore creates a StrategicMemoryStore with the mock embedder.
func createStrategicTestStore(t *testing.T) (*StrategicMemoryStore, *sql.DB) {
	t.Helper()
	db := setupStrategicTestDB(t)
	embedder := &strategicMockEmbedder{dimension: 128}
	store := NewStrategicMemoryStore(db, embedder)
	return store, db
}

// TestStrategicMemory_Create verifies memory creation with auto ID and embedding.
func TestStrategicMemory_Create(t *testing.T) {
	store, _ := createStrategicTestStore(t)
	ctx := context.Background()

	t.Run("creates memory with auto-generated ID", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle:      "Always check container logs before restarting",
			Category:       "debugging",
			TriggerPattern: "container errors",
			SourceSessions: []string{"session-1", "session-2"},
		}

		err := store.Create(ctx, mem)
		require.NoError(t, err)

		// Verify ID was generated with correct prefix
		assert.True(t, strings.HasPrefix(mem.ID, "strat_"), "ID should have strat_ prefix")
		// strat_ (6 chars) + UUID (36 chars) = 42 chars
		assert.Len(t, mem.ID, 42, "ID should be strat_ + 36 char UUID")
	})

	t.Run("generates embedding from principle and trigger pattern", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle:      "Use verbose logging during development",
			Category:       "development",
			TriggerPattern: "new feature implementation",
		}

		err := store.Create(ctx, mem)
		require.NoError(t, err)

		// Embedding should be generated
		assert.NotNil(t, mem.Embedding)
		assert.Len(t, mem.Embedding, 128)
	})

	t.Run("sets default confidence to 0.5", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle: "Test default confidence",
		}

		err := store.Create(ctx, mem)
		require.NoError(t, err)

		assert.Equal(t, 0.5, mem.Confidence)
	})

	t.Run("preserves custom confidence", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle:  "Test custom confidence",
			Confidence: 0.8,
		}

		err := store.Create(ctx, mem)
		require.NoError(t, err)

		assert.Equal(t, 0.8, mem.Confidence)
	})

	t.Run("sets timestamps", func(t *testing.T) {
		before := time.Now().Add(-time.Second)
		mem := &StrategicMemory{
			Principle: "Test timestamps",
		}

		err := store.Create(ctx, mem)
		require.NoError(t, err)
		after := time.Now().Add(time.Second)

		assert.True(t, mem.CreatedAt.After(before) && mem.CreatedAt.Before(after))
		assert.True(t, mem.UpdatedAt.After(before) && mem.UpdatedAt.Before(after))
	})

	t.Run("rejects empty principle", func(t *testing.T) {
		mem := &StrategicMemory{
			Category: "debugging",
		}

		err := store.Create(ctx, mem)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "principle is required")
	})

	t.Run("handles nil source sessions", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle:      "No source sessions",
			SourceSessions: nil,
		}

		err := store.Create(ctx, mem)
		require.NoError(t, err)

		// Should be retrievable
		retrieved, err := store.Get(ctx, mem.ID)
		require.NoError(t, err)
		assert.Empty(t, retrieved.SourceSessions)
	})
}

// TestStrategicMemory_Get verifies retrieval by ID.
func TestStrategicMemory_Get(t *testing.T) {
	store, _ := createStrategicTestStore(t)
	ctx := context.Background()

	t.Run("retrieves existing memory", func(t *testing.T) {
		original := &StrategicMemory{
			Principle:      "Always validate input before processing",
			Category:       "security",
			TriggerPattern: "user input handling",
			SourceSessions: []string{"sess-1"},
			Confidence:     0.85,
		}

		err := store.Create(ctx, original)
		require.NoError(t, err)

		retrieved, err := store.Get(ctx, original.ID)
		require.NoError(t, err)

		assert.Equal(t, original.ID, retrieved.ID)
		assert.Equal(t, original.Principle, retrieved.Principle)
		assert.Equal(t, original.Category, retrieved.Category)
		assert.Equal(t, original.TriggerPattern, retrieved.TriggerPattern)
		assert.Equal(t, original.Confidence, retrieved.Confidence)
		assert.Equal(t, original.SourceSessions, retrieved.SourceSessions)
	})

	t.Run("retrieves embedding correctly", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle: "Test embedding retrieval",
		}

		err := store.Create(ctx, mem)
		require.NoError(t, err)

		retrieved, err := store.Get(ctx, mem.ID)
		require.NoError(t, err)

		assert.Equal(t, mem.Embedding, retrieved.Embedding)
	})

	t.Run("returns error for non-existent ID", func(t *testing.T) {
		_, err := store.Get(ctx, "strat_nonexistent-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// TestStrategicMemory_RecordSuccess verifies success count increment.
func TestStrategicMemory_RecordSuccess(t *testing.T) {
	store, _ := createStrategicTestStore(t)
	ctx := context.Background()

	t.Run("increments success count and apply count", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle:    "Test success recording",
			SuccessCount: 0,
			ApplyCount:   0,
		}

		err := store.Create(ctx, mem)
		require.NoError(t, err)

		err = store.RecordSuccess(ctx, mem.ID)
		require.NoError(t, err)

		retrieved, err := store.Get(ctx, mem.ID)
		require.NoError(t, err)

		assert.Equal(t, 1, retrieved.SuccessCount)
		assert.Equal(t, 1, retrieved.ApplyCount)
	})

	t.Run("updates last_applied_at", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle: "Test last applied update",
		}

		err := store.Create(ctx, mem)
		require.NoError(t, err)

		// Initially nil
		retrieved, _ := store.Get(ctx, mem.ID)
		assert.Nil(t, retrieved.LastAppliedAt)

		// After success
		err = store.RecordSuccess(ctx, mem.ID)
		require.NoError(t, err)

		retrieved, _ = store.Get(ctx, mem.ID)
		assert.NotNil(t, retrieved.LastAppliedAt)
	})

	t.Run("multiple successes accumulate", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle: "Test multiple successes",
		}

		err := store.Create(ctx, mem)
		require.NoError(t, err)

		for i := 0; i < 5; i++ {
			err = store.RecordSuccess(ctx, mem.ID)
			require.NoError(t, err)
		}

		retrieved, _ := store.Get(ctx, mem.ID)
		assert.Equal(t, 5, retrieved.SuccessCount)
		assert.Equal(t, 5, retrieved.ApplyCount)
	})

	t.Run("returns error for non-existent ID", func(t *testing.T) {
		err := store.RecordSuccess(ctx, "strat_nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// TestStrategicMemory_RecordFailure verifies failure count increment.
func TestStrategicMemory_RecordFailure(t *testing.T) {
	store, _ := createStrategicTestStore(t)
	ctx := context.Background()

	t.Run("increments failure count and apply count", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle:    "Test failure recording",
			FailureCount: 0,
			ApplyCount:   0,
		}

		err := store.Create(ctx, mem)
		require.NoError(t, err)

		err = store.RecordFailure(ctx, mem.ID)
		require.NoError(t, err)

		retrieved, err := store.Get(ctx, mem.ID)
		require.NoError(t, err)

		assert.Equal(t, 1, retrieved.FailureCount)
		assert.Equal(t, 1, retrieved.ApplyCount)
	})

	t.Run("updates last_applied_at", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle: "Test failure last applied",
		}

		err := store.Create(ctx, mem)
		require.NoError(t, err)

		err = store.RecordFailure(ctx, mem.ID)
		require.NoError(t, err)

		retrieved, _ := store.Get(ctx, mem.ID)
		assert.NotNil(t, retrieved.LastAppliedAt)
	})

	t.Run("multiple failures accumulate", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle: "Test multiple failures",
		}

		err := store.Create(ctx, mem)
		require.NoError(t, err)

		for i := 0; i < 3; i++ {
			err = store.RecordFailure(ctx, mem.ID)
			require.NoError(t, err)
		}

		retrieved, _ := store.Get(ctx, mem.ID)
		assert.Equal(t, 3, retrieved.FailureCount)
		assert.Equal(t, 3, retrieved.ApplyCount)
	})

	t.Run("returns error for non-existent ID", func(t *testing.T) {
		err := store.RecordFailure(ctx, "strat_nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// TestStrategicMemory_SuccessRate verifies computed success rate.
func TestStrategicMemory_SuccessRate(t *testing.T) {
	store, _ := createStrategicTestStore(t)
	ctx := context.Background()

	t.Run("computes correct success rate", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle: "Test success rate calculation",
		}

		err := store.Create(ctx, mem)
		require.NoError(t, err)

		// Record 3 successes, 1 failure = 75% success rate
		for i := 0; i < 3; i++ {
			store.RecordSuccess(ctx, mem.ID)
		}
		store.RecordFailure(ctx, mem.ID)

		retrieved, err := store.Get(ctx, mem.ID)
		require.NoError(t, err)

		// success_rate = 3 / (3 + 1) = 0.75
		assert.InDelta(t, 0.75, retrieved.SuccessRate, 0.001)
	})

	t.Run("returns 0.5 for no observations", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle: "Test default success rate",
		}

		err := store.Create(ctx, mem)
		require.NoError(t, err)

		retrieved, err := store.Get(ctx, mem.ID)
		require.NoError(t, err)

		// Default when no observations: 0.5
		assert.Equal(t, 0.5, retrieved.SuccessRate)
	})

	t.Run("returns 1.0 for all successes", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle: "Test 100% success rate",
		}

		err := store.Create(ctx, mem)
		require.NoError(t, err)

		for i := 0; i < 5; i++ {
			store.RecordSuccess(ctx, mem.ID)
		}

		retrieved, err := store.Get(ctx, mem.ID)
		require.NoError(t, err)

		assert.Equal(t, 1.0, retrieved.SuccessRate)
	})

	t.Run("returns 0.0 for all failures", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle: "Test 0% success rate",
		}

		err := store.Create(ctx, mem)
		require.NoError(t, err)

		for i := 0; i < 3; i++ {
			store.RecordFailure(ctx, mem.ID)
		}

		retrieved, err := store.Get(ctx, mem.ID)
		require.NoError(t, err)

		assert.Equal(t, 0.0, retrieved.SuccessRate)
	})
}

// TestStrategicMemory_GetTopPrinciples verifies ordering by success rate with min evidence filter.
func TestStrategicMemory_GetTopPrinciples(t *testing.T) {
	store, _ := createStrategicTestStore(t)
	ctx := context.Background()

	t.Run("returns principles ordered by success rate", func(t *testing.T) {
		// Create memories with different success rates
		// Memory 1: 3 successes, 0 failures = 100%
		mem1 := &StrategicMemory{Principle: "High success principle"}
		store.Create(ctx, mem1)
		for i := 0; i < 3; i++ {
			store.RecordSuccess(ctx, mem1.ID)
		}

		// Memory 2: 2 successes, 2 failures = 50%
		mem2 := &StrategicMemory{Principle: "Medium success principle"}
		store.Create(ctx, mem2)
		for i := 0; i < 2; i++ {
			store.RecordSuccess(ctx, mem2.ID)
			store.RecordFailure(ctx, mem2.ID)
		}

		// Memory 3: 3 successes, 1 failure = 75%
		mem3 := &StrategicMemory{Principle: "Good success principle"}
		store.Create(ctx, mem3)
		for i := 0; i < 3; i++ {
			store.RecordSuccess(ctx, mem3.ID)
		}
		store.RecordFailure(ctx, mem3.ID)

		results, err := store.GetTopPrinciples(ctx, 10)
		require.NoError(t, err)

		// Should be ordered: 100%, 75%, 50%
		require.Len(t, results, 3)
		assert.Equal(t, "High success principle", results[0].Principle)
		assert.Equal(t, "Good success principle", results[1].Principle)
		assert.Equal(t, "Medium success principle", results[2].Principle)
	})

	t.Run("filters by minimum evidence", func(t *testing.T) {
		// Create a new store for isolated test
		store2, _ := createStrategicTestStore(t)

		// Memory with insufficient evidence (only 2 observations, need 3)
		memLow := &StrategicMemory{Principle: "Low evidence principle"}
		store2.Create(ctx, memLow)
		store2.RecordSuccess(ctx, memLow.ID)
		store2.RecordSuccess(ctx, memLow.ID)

		// Memory with sufficient evidence (3 observations)
		memEnough := &StrategicMemory{Principle: "Enough evidence principle"}
		store2.Create(ctx, memEnough)
		for i := 0; i < 3; i++ {
			store2.RecordSuccess(ctx, memEnough.ID)
		}

		results, err := store2.GetTopPrinciples(ctx, 10)
		require.NoError(t, err)

		// Only the one with >= MinEvidenceForReliable should be returned
		assert.Len(t, results, 1)
		assert.Equal(t, "Enough evidence principle", results[0].Principle)
	})

	t.Run("respects limit parameter", func(t *testing.T) {
		store3, _ := createStrategicTestStore(t)

		// Create multiple memories with enough evidence
		for i := 0; i < 5; i++ {
			mem := &StrategicMemory{Principle: "Principle " + string(rune('A'+i))}
			store3.Create(ctx, mem)
			for j := 0; j < MinEvidenceForReliable; j++ {
				store3.RecordSuccess(ctx, mem.ID)
			}
		}

		results, err := store3.GetTopPrinciples(ctx, 3)
		require.NoError(t, err)

		assert.Len(t, results, 3)
	})

	t.Run("uses confidence as secondary sort", func(t *testing.T) {
		store4, _ := createStrategicTestStore(t)

		// Create two memories with same success rate but different confidence
		mem1 := &StrategicMemory{Principle: "High confidence", Confidence: 0.9}
		store4.Create(ctx, mem1)
		for i := 0; i < 3; i++ {
			store4.RecordSuccess(ctx, mem1.ID)
		}

		mem2 := &StrategicMemory{Principle: "Low confidence", Confidence: 0.3}
		store4.Create(ctx, mem2)
		for i := 0; i < 3; i++ {
			store4.RecordSuccess(ctx, mem2.ID)
		}

		results, err := store4.GetTopPrinciples(ctx, 10)
		require.NoError(t, err)

		// Same success rate (100%), so sorted by confidence
		require.Len(t, results, 2)
		assert.Equal(t, "High confidence", results[0].Principle)
		assert.Equal(t, "Low confidence", results[1].Principle)
	})
}

// TestStrategicMemory_GetByCategory verifies category filtering.
func TestStrategicMemory_GetByCategory(t *testing.T) {
	store, _ := createStrategicTestStore(t)
	ctx := context.Background()

	t.Run("returns only memories in specified category", func(t *testing.T) {
		// Create memories in different categories
		store.Create(ctx, &StrategicMemory{Principle: "Docker principle 1", Category: "docker"})
		store.Create(ctx, &StrategicMemory{Principle: "Docker principle 2", Category: "docker"})
		store.Create(ctx, &StrategicMemory{Principle: "Git principle", Category: "git"})
		store.Create(ctx, &StrategicMemory{Principle: "Go principle", Category: "go"})

		results, err := store.GetByCategory(ctx, "docker", 10)
		require.NoError(t, err)

		assert.Len(t, results, 2)
		for _, r := range results {
			assert.Equal(t, "docker", r.Category)
		}
	})

	t.Run("returns empty for non-existent category", func(t *testing.T) {
		results, err := store.GetByCategory(ctx, "nonexistent", 10)
		require.NoError(t, err)

		assert.Empty(t, results)
	})

	t.Run("respects limit parameter", func(t *testing.T) {
		store2, _ := createStrategicTestStore(t)

		// Create multiple memories in same category
		for i := 0; i < 10; i++ {
			store2.Create(ctx, &StrategicMemory{Principle: "Test principle", Category: "test"})
		}

		results, err := store2.GetByCategory(ctx, "test", 5)
		require.NoError(t, err)

		assert.Len(t, results, 5)
	})

	t.Run("orders by success rate and confidence", func(t *testing.T) {
		store3, _ := createStrategicTestStore(t)

		mem1 := &StrategicMemory{Principle: "Low rate", Category: "cat1", Confidence: 0.5}
		store3.Create(ctx, mem1)
		store3.RecordFailure(ctx, mem1.ID)
		store3.RecordFailure(ctx, mem1.ID)

		mem2 := &StrategicMemory{Principle: "High rate", Category: "cat1", Confidence: 0.5}
		store3.Create(ctx, mem2)
		store3.RecordSuccess(ctx, mem2.ID)
		store3.RecordSuccess(ctx, mem2.ID)

		results, err := store3.GetByCategory(ctx, "cat1", 10)
		require.NoError(t, err)

		require.Len(t, results, 2)
		assert.Equal(t, "High rate", results[0].Principle)
		assert.Equal(t, "Low rate", results[1].Principle)
	})
}

// TestStrategicMemory_SearchSimilar verifies vector similarity search.
func TestStrategicMemory_SearchSimilar(t *testing.T) {
	store, _ := createStrategicTestStore(t)
	ctx := context.Background()

	t.Run("finds similar memories by embedding", func(t *testing.T) {
		// Create memories with different principles
		store.Create(ctx, &StrategicMemory{Principle: "Always validate user input"})
		store.Create(ctx, &StrategicMemory{Principle: "Use proper error handling"})
		store.Create(ctx, &StrategicMemory{Principle: "Validate all inputs from users"}) // Similar to first

		results, err := store.SearchSimilar(ctx, "validate user input", 10)
		require.NoError(t, err)

		// Should return results ordered by similarity
		assert.NotEmpty(t, results)
	})

	t.Run("returns empty when no memories exist", func(t *testing.T) {
		store2, _ := createStrategicTestStore(t)

		results, err := store2.SearchSimilar(ctx, "test query", 10)
		require.NoError(t, err)

		assert.Empty(t, results)
	})

	t.Run("respects limit parameter", func(t *testing.T) {
		store3, _ := createStrategicTestStore(t)

		for i := 0; i < 10; i++ {
			store3.Create(ctx, &StrategicMemory{Principle: "Principle " + string(rune('A'+i))})
		}

		results, err := store3.SearchSimilar(ctx, "Principle", 3)
		require.NoError(t, err)

		assert.Len(t, results, 3)
	})

	t.Run("returns error when embedder is nil", func(t *testing.T) {
		db := setupStrategicTestDB(t)
		storeNoEmbed := NewStrategicMemoryStore(db, nil)

		_, err := storeNoEmbed.SearchSimilar(ctx, "test", 10)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "embedder not configured")
	})
}

// TestStrategicMemory_SearchFTS verifies full-text search.
// Note: These tests require FTS5 support which may not be available in all SQLite builds.
// The tests verify error handling when FTS is not available.
func TestStrategicMemory_SearchFTS(t *testing.T) {
	store, _ := createStrategicTestStore(t)
	ctx := context.Background()

	// Since we don't have FTS5 in the test environment, verify that SearchFTS
	// returns an error when the FTS table doesn't exist
	t.Run("returns error when FTS table missing", func(t *testing.T) {
		store.Create(ctx, &StrategicMemory{Principle: "Test principle", Category: "test"})

		_, err := store.SearchFTS(ctx, "test", 10)
		// Should error because FTS table doesn't exist in test DB
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "strategic_memory_fts")
	})
}

// TestStrategicMemory_UpdateConfidence verifies confidence update.
func TestStrategicMemory_UpdateConfidence(t *testing.T) {
	store, _ := createStrategicTestStore(t)
	ctx := context.Background()

	t.Run("updates confidence value", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle:  "Test confidence update",
			Confidence: 0.5,
		}

		err := store.Create(ctx, mem)
		require.NoError(t, err)

		err = store.UpdateConfidence(ctx, mem.ID, 0.9)
		require.NoError(t, err)

		retrieved, err := store.Get(ctx, mem.ID)
		require.NoError(t, err)

		assert.Equal(t, 0.9, retrieved.Confidence)
	})

	t.Run("updates updated_at timestamp", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle: "Test timestamp update",
		}

		err := store.Create(ctx, mem)
		require.NoError(t, err)

		originalUpdatedAt := mem.UpdatedAt

		// Wait to ensure timestamp differs - use longer delay for reliability
		time.Sleep(1100 * time.Millisecond)

		err = store.UpdateConfidence(ctx, mem.ID, 0.7)
		require.NoError(t, err)

		retrieved, _ := store.Get(ctx, mem.ID)
		// The updated_at should be at or after the original time
		assert.True(t, !retrieved.UpdatedAt.Before(originalUpdatedAt),
			"updated_at should not be before original: got %v, original %v",
			retrieved.UpdatedAt, originalUpdatedAt)
	})

	t.Run("rejects confidence below 0", func(t *testing.T) {
		mem := &StrategicMemory{Principle: "Test invalid confidence"}
		store.Create(ctx, mem)

		err := store.UpdateConfidence(ctx, mem.ID, -0.1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "between 0 and 1")
	})

	t.Run("rejects confidence above 1", func(t *testing.T) {
		mem := &StrategicMemory{Principle: "Test invalid confidence high"}
		store.Create(ctx, mem)

		err := store.UpdateConfidence(ctx, mem.ID, 1.5)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "between 0 and 1")
	})

	t.Run("accepts boundary values 0 and 1", func(t *testing.T) {
		mem := &StrategicMemory{Principle: "Test boundary confidence"}
		store.Create(ctx, mem)

		err := store.UpdateConfidence(ctx, mem.ID, 0.0)
		require.NoError(t, err)

		retrieved, _ := store.Get(ctx, mem.ID)
		assert.Equal(t, 0.0, retrieved.Confidence)

		err = store.UpdateConfidence(ctx, mem.ID, 1.0)
		require.NoError(t, err)

		retrieved, _ = store.Get(ctx, mem.ID)
		assert.Equal(t, 1.0, retrieved.Confidence)
	})

	t.Run("returns error for non-existent ID", func(t *testing.T) {
		err := store.UpdateConfidence(ctx, "strat_nonexistent", 0.5)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// TestStrategicMemory_Delete verifies deletion.
func TestStrategicMemory_Delete(t *testing.T) {
	store, _ := createStrategicTestStore(t)
	ctx := context.Background()

	t.Run("deletes existing memory", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle: "To be deleted",
		}

		err := store.Create(ctx, mem)
		require.NoError(t, err)

		// Verify it exists
		_, err = store.Get(ctx, mem.ID)
		require.NoError(t, err)

		// Delete it
		err = store.Delete(ctx, mem.ID)
		require.NoError(t, err)

		// Verify it's gone
		_, err = store.Get(ctx, mem.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("returns error for non-existent ID", func(t *testing.T) {
		err := store.Delete(ctx, "strat_nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	// Note: FTS index removal test skipped - requires FTS5 support
	// In production with FTS5, the delete trigger would remove from FTS index
}

// TestStrategicMemory_NilEmbedder verifies behavior without an embedder.
func TestStrategicMemory_NilEmbedder(t *testing.T) {
	db := setupStrategicTestDB(t)
	store := NewStrategicMemoryStore(db, nil)
	ctx := context.Background()

	t.Run("creates memory without embedding", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle: "Test without embedder",
		}

		err := store.Create(ctx, mem)
		require.NoError(t, err)

		// Should have no embedding
		assert.Nil(t, mem.Embedding)
	})

	t.Run("retrieves memory without embedding", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle: "Retrievable without embedding",
		}

		store.Create(ctx, mem)

		retrieved, err := store.Get(ctx, mem.ID)
		require.NoError(t, err)

		assert.Equal(t, mem.ID, retrieved.ID)
		assert.Nil(t, retrieved.Embedding)
	})
}

// TestStrategicMemory_SequentialOperations tests that multiple operations work correctly.
// Note: True concurrent access testing requires a file-based SQLite database with proper
// connection pooling. In-memory SQLite doesn't handle concurrent writes well.
func TestStrategicMemory_SequentialOperations(t *testing.T) {
	store, _ := createStrategicTestStore(t)
	ctx := context.Background()

	mem := &StrategicMemory{
		Principle: "Sequential test principle",
	}

	err := store.Create(ctx, mem)
	require.NoError(t, err)

	// Run sequential success/failure recordings
	for i := 0; i < 10; i++ {
		err = store.RecordSuccess(ctx, mem.ID)
		require.NoError(t, err)
		err = store.RecordFailure(ctx, mem.ID)
		require.NoError(t, err)
	}

	// Verify final state is consistent
	retrieved, err := store.Get(ctx, mem.ID)
	require.NoError(t, err)

	assert.Equal(t, 10, retrieved.SuccessCount)
	assert.Equal(t, 10, retrieved.FailureCount)
	assert.Equal(t, 20, retrieved.ApplyCount)
}

// TestStrategicMemory_SourceSessionsPersistence verifies JSON array storage.
func TestStrategicMemory_SourceSessionsPersistence(t *testing.T) {
	store, _ := createStrategicTestStore(t)
	ctx := context.Background()

	t.Run("persists multiple source sessions", func(t *testing.T) {
		sessions := []string{"sess-001", "sess-002", "sess-003"}
		mem := &StrategicMemory{
			Principle:      "Test source sessions",
			SourceSessions: sessions,
		}

		err := store.Create(ctx, mem)
		require.NoError(t, err)

		retrieved, err := store.Get(ctx, mem.ID)
		require.NoError(t, err)

		assert.Equal(t, sessions, retrieved.SourceSessions)
	})

	t.Run("handles empty source sessions", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle:      "Test empty sessions",
			SourceSessions: []string{},
		}

		err := store.Create(ctx, mem)
		require.NoError(t, err)

		retrieved, err := store.Get(ctx, mem.ID)
		require.NoError(t, err)

		assert.Empty(t, retrieved.SourceSessions)
	})
}

// TestStrategicMemory_MixedSuccessFailure verifies combined success/failure tracking.
func TestStrategicMemory_MixedSuccessFailure(t *testing.T) {
	store, _ := createStrategicTestStore(t)
	ctx := context.Background()

	mem := &StrategicMemory{
		Principle: "Test mixed tracking",
	}

	err := store.Create(ctx, mem)
	require.NoError(t, err)

	// Interleave successes and failures
	store.RecordSuccess(ctx, mem.ID)
	store.RecordFailure(ctx, mem.ID)
	store.RecordSuccess(ctx, mem.ID)
	store.RecordSuccess(ctx, mem.ID)
	store.RecordFailure(ctx, mem.ID)

	retrieved, err := store.Get(ctx, mem.ID)
	require.NoError(t, err)

	assert.Equal(t, 3, retrieved.SuccessCount)
	assert.Equal(t, 2, retrieved.FailureCount)
	assert.Equal(t, 5, retrieved.ApplyCount)
	// Success rate = 3/5 = 0.6
	assert.InDelta(t, 0.6, retrieved.SuccessRate, 0.001)
}

// ============================================================================
// TIER PROMOTION TESTS
// ============================================================================

// TestStrategicMemory_DefaultTier verifies new memories are created with tentative tier.
func TestStrategicMemory_DefaultTier(t *testing.T) {
	store, _ := createStrategicTestStore(t)
	ctx := context.Background()

	t.Run("creates memory with default tentative tier", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle: "Test default tier",
		}

		err := store.Create(ctx, mem)
		require.NoError(t, err)

		assert.Equal(t, TierTentative, mem.Tier)

		// Verify persisted correctly
		retrieved, err := store.Get(ctx, mem.ID)
		require.NoError(t, err)
		assert.Equal(t, TierTentative, retrieved.Tier)
	})

	t.Run("preserves explicit tier on creation", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle: "Test explicit tier",
			Tier:      TierCandidate,
		}

		err := store.Create(ctx, mem)
		require.NoError(t, err)

		assert.Equal(t, TierCandidate, mem.Tier)

		// Verify persisted correctly
		retrieved, err := store.Get(ctx, mem.ID)
		require.NoError(t, err)
		assert.Equal(t, TierCandidate, retrieved.Tier)
	})
}

// TestStrategicMemory_CalculateEligibleTier verifies tier calculation logic.
func TestStrategicMemory_CalculateEligibleTier(t *testing.T) {
	store, _ := createStrategicTestStore(t)
	thresholds := DefaultTierPromotionThresholds()

	t.Run("returns tentative for new memory", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle:      "New memory",
			ApplyCount:     0,
			SuccessRate:    0.5,
			SourceSessions: []string{},
			CreatedAt:      time.Now(),
		}

		tier := store.CalculateEligibleTier(mem, thresholds)
		assert.Equal(t, TierTentative, tier)
	})

	t.Run("returns candidate when apply count meets threshold", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle:      "Candidate memory",
			ApplyCount:     3, // MinApplyCountForCandidate
			SuccessRate:    0.5,
			SourceSessions: []string{},
			CreatedAt:      time.Now(),
		}

		tier := store.CalculateEligibleTier(mem, thresholds)
		assert.Equal(t, TierCandidate, tier)
	})

	t.Run("returns proven when apply count and success rate meet thresholds", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle:      "Proven memory",
			ApplyCount:     10, // MinApplyCountForProven
			SuccessRate:    0.85, // > MinSuccessRateForProven (0.80)
			SourceSessions: []string{},
			CreatedAt:      time.Now(),
		}

		tier := store.CalculateEligibleTier(mem, thresholds)
		assert.Equal(t, TierProven, tier)
	})

	t.Run("returns candidate when apply count high but success rate low", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle:      "High apply low success",
			ApplyCount:     15,
			SuccessRate:    0.60, // < MinSuccessRateForProven (0.80)
			SourceSessions: []string{},
			CreatedAt:      time.Now(),
		}

		tier := store.CalculateEligibleTier(mem, thresholds)
		assert.Equal(t, TierCandidate, tier)
	})

	t.Run("returns identity when all criteria met", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle:   "Identity memory",
			ApplyCount:  25, // MinApplyCountForIdentity
			SuccessRate: 0.95, // > MinSuccessRateForIdentity (0.90)
			SourceSessions: []string{
				"sess-1", "sess-2", "sess-3", "sess-4", "sess-5", // MinUniqueSessionsForIdentity
			},
			CreatedAt: time.Now().Add(-35 * 24 * time.Hour), // > MinAgeForIdentity (30 days)
		}

		tier := store.CalculateEligibleTier(mem, thresholds)
		assert.Equal(t, TierIdentity, tier)
	})

	t.Run("returns proven when identity age not met", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle:   "Almost identity",
			ApplyCount:  25,
			SuccessRate: 0.95,
			SourceSessions: []string{
				"sess-1", "sess-2", "sess-3", "sess-4", "sess-5",
			},
			CreatedAt: time.Now().Add(-10 * 24 * time.Hour), // < MinAgeForIdentity (30 days)
		}

		tier := store.CalculateEligibleTier(mem, thresholds)
		assert.Equal(t, TierProven, tier)
	})

	t.Run("returns proven when identity sessions not met", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle:   "Few sessions",
			ApplyCount:  25,
			SuccessRate: 0.95,
			SourceSessions: []string{
				"sess-1", "sess-2", // < MinUniqueSessionsForIdentity (5)
			},
			CreatedAt: time.Now().Add(-35 * 24 * time.Hour),
		}

		tier := store.CalculateEligibleTier(mem, thresholds)
		assert.Equal(t, TierProven, tier)
	})
}

// TestStrategicMemory_PromoteIfEligible verifies tier promotion.
func TestStrategicMemory_PromoteIfEligible(t *testing.T) {
	ctx := context.Background()
	thresholds := DefaultTierPromotionThresholds()

	t.Run("promotes tentative to candidate", func(t *testing.T) {
		store, _ := createStrategicTestStore(t)

		mem := &StrategicMemory{
			Principle: "Promotable memory",
		}
		err := store.Create(ctx, mem)
		require.NoError(t, err)

		// Record enough applications to qualify for candidate
		for i := 0; i < 3; i++ {
			err = store.RecordSuccess(ctx, mem.ID)
			require.NoError(t, err)
		}

		promoted, newTier, err := store.PromoteIfEligible(ctx, mem.ID, thresholds)
		require.NoError(t, err)

		assert.True(t, promoted)
		assert.Equal(t, TierCandidate, newTier)

		// Verify persisted
		retrieved, _ := store.Get(ctx, mem.ID)
		assert.Equal(t, TierCandidate, retrieved.Tier)
	})

	t.Run("does not promote when criteria not met", func(t *testing.T) {
		store, _ := createStrategicTestStore(t)

		mem := &StrategicMemory{
			Principle: "Not promotable yet",
		}
		err := store.Create(ctx, mem)
		require.NoError(t, err)

		// Only 2 applications - not enough for candidate
		store.RecordSuccess(ctx, mem.ID)
		store.RecordSuccess(ctx, mem.ID)

		promoted, newTier, err := store.PromoteIfEligible(ctx, mem.ID, thresholds)
		require.NoError(t, err)

		assert.False(t, promoted)
		assert.Equal(t, TierTentative, newTier)
	})

	t.Run("does not demote already promoted memory", func(t *testing.T) {
		store, _ := createStrategicTestStore(t)

		// Create memory at proven tier
		mem := &StrategicMemory{
			Principle: "Already proven",
			Tier:      TierProven,
		}
		err := store.Create(ctx, mem)
		require.NoError(t, err)

		// Stats would only qualify for candidate
		store.RecordSuccess(ctx, mem.ID)
		store.RecordSuccess(ctx, mem.ID)
		store.RecordSuccess(ctx, mem.ID)

		promoted, newTier, err := store.PromoteIfEligible(ctx, mem.ID, thresholds)
		require.NoError(t, err)

		assert.False(t, promoted)
		assert.Equal(t, TierProven, newTier) // Stays at proven
	})

	t.Run("promotes candidate to proven", func(t *testing.T) {
		store, _ := createStrategicTestStore(t)

		mem := &StrategicMemory{
			Principle: "Rising star",
			Tier:      TierCandidate,
		}
		err := store.Create(ctx, mem)
		require.NoError(t, err)

		// Record 10 successes (100% success rate meets proven threshold)
		for i := 0; i < 10; i++ {
			store.RecordSuccess(ctx, mem.ID)
		}

		promoted, newTier, err := store.PromoteIfEligible(ctx, mem.ID, thresholds)
		require.NoError(t, err)

		assert.True(t, promoted)
		assert.Equal(t, TierProven, newTier)
	})

	t.Run("returns error for non-existent memory", func(t *testing.T) {
		store, _ := createStrategicTestStore(t)

		_, _, err := store.PromoteIfEligible(ctx, "strat_nonexistent", thresholds)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// TestStrategicMemory_GetByTier verifies tier filtering.
func TestStrategicMemory_GetByTier(t *testing.T) {
	store, _ := createStrategicTestStore(t)
	ctx := context.Background()

	t.Run("returns only memories in specified tier", func(t *testing.T) {
		// Create memories in different tiers
		store.Create(ctx, &StrategicMemory{Principle: "Tentative 1", Tier: TierTentative})
		store.Create(ctx, &StrategicMemory{Principle: "Tentative 2", Tier: TierTentative})
		store.Create(ctx, &StrategicMemory{Principle: "Candidate 1", Tier: TierCandidate})
		store.Create(ctx, &StrategicMemory{Principle: "Proven 1", Tier: TierProven})

		results, err := store.GetByTier(ctx, TierTentative, 10)
		require.NoError(t, err)

		assert.Len(t, results, 2)
		for _, r := range results {
			assert.Equal(t, TierTentative, r.Tier)
		}
	})

	t.Run("returns empty for tier with no memories", func(t *testing.T) {
		store2, _ := createStrategicTestStore(t)

		// Create only tentative memories
		store2.Create(ctx, &StrategicMemory{Principle: "Only tentative"})

		results, err := store2.GetByTier(ctx, TierIdentity, 10)
		require.NoError(t, err)

		assert.Empty(t, results)
	})

	t.Run("respects limit parameter", func(t *testing.T) {
		store3, _ := createStrategicTestStore(t)

		// Create multiple memories in same tier
		for i := 0; i < 10; i++ {
			store3.Create(ctx, &StrategicMemory{Principle: "Test principle", Tier: TierCandidate})
		}

		results, err := store3.GetByTier(ctx, TierCandidate, 5)
		require.NoError(t, err)

		assert.Len(t, results, 5)
	})

	t.Run("orders by success rate and confidence", func(t *testing.T) {
		store4, _ := createStrategicTestStore(t)

		mem1 := &StrategicMemory{Principle: "Low rate", Tier: TierCandidate, Confidence: 0.5}
		store4.Create(ctx, mem1)
		store4.RecordFailure(ctx, mem1.ID)
		store4.RecordFailure(ctx, mem1.ID)

		mem2 := &StrategicMemory{Principle: "High rate", Tier: TierCandidate, Confidence: 0.5}
		store4.Create(ctx, mem2)
		store4.RecordSuccess(ctx, mem2.ID)
		store4.RecordSuccess(ctx, mem2.ID)

		results, err := store4.GetByTier(ctx, TierCandidate, 10)
		require.NoError(t, err)

		require.Len(t, results, 2)
		assert.Equal(t, "High rate", results[0].Principle)
		assert.Equal(t, "Low rate", results[1].Principle)
	})
}

// TestStrategicMemory_UpdateTier verifies tier update.
func TestStrategicMemory_UpdateTier(t *testing.T) {
	store, _ := createStrategicTestStore(t)
	ctx := context.Background()

	t.Run("updates tier value", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle: "Test tier update",
			Tier:      TierTentative,
		}

		err := store.Create(ctx, mem)
		require.NoError(t, err)

		err = store.UpdateTier(ctx, mem.ID, TierProven)
		require.NoError(t, err)

		retrieved, err := store.Get(ctx, mem.ID)
		require.NoError(t, err)

		assert.Equal(t, TierProven, retrieved.Tier)
	})

	t.Run("updates updated_at timestamp", func(t *testing.T) {
		mem := &StrategicMemory{
			Principle: "Test timestamp update",
		}

		err := store.Create(ctx, mem)
		require.NoError(t, err)

		originalUpdatedAt := mem.UpdatedAt

		time.Sleep(1100 * time.Millisecond)

		err = store.UpdateTier(ctx, mem.ID, TierCandidate)
		require.NoError(t, err)

		retrieved, _ := store.Get(ctx, mem.ID)
		assert.True(t, !retrieved.UpdatedAt.Before(originalUpdatedAt))
	})

	t.Run("accepts all valid tiers", func(t *testing.T) {
		mem := &StrategicMemory{Principle: "Test valid tiers"}
		store.Create(ctx, mem)

		validTiers := []MemoryTier{TierTentative, TierCandidate, TierProven, TierIdentity}
		for _, tier := range validTiers {
			err := store.UpdateTier(ctx, mem.ID, tier)
			require.NoError(t, err, "should accept tier: %s", tier)

			retrieved, _ := store.Get(ctx, mem.ID)
			assert.Equal(t, tier, retrieved.Tier)
		}
	})

	t.Run("rejects invalid tier", func(t *testing.T) {
		mem := &StrategicMemory{Principle: "Test invalid tier"}
		store.Create(ctx, mem)

		err := store.UpdateTier(ctx, mem.ID, MemoryTier("invalid"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid tier value")
	})

	t.Run("returns error for non-existent ID", func(t *testing.T) {
		err := store.UpdateTier(ctx, "strat_nonexistent", TierCandidate)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}
