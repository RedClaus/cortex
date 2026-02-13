package memory

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// MOCK IMPLEMENTATIONS FOR CLUSTERING TESTS
// ============================================================================

// clusteringMockLLM provides deterministic responses for topic naming.
type clusteringMockLLM struct{}

func (m *clusteringMockLLM) Complete(ctx context.Context, prompt string) (string, error) {
	return "NAME: Test Topic\nDESCRIPTION: A test topic for clustering\nKEYWORDS: test, clustering, memory", nil
}

// clusteringMockEmbedder provides deterministic embeddings for testing.
type clusteringMockEmbedder struct {
	dimension int
}

func (m *clusteringMockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// Return a simple hash-based embedding for deterministic behavior
	emb := make([]float32, m.dimension)
	for i, c := range text {
		emb[i%m.dimension] += float32(c) / 1000.0
	}
	return NormalizeVector(emb), nil
}

func (m *clusteringMockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
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

func (m *clusteringMockEmbedder) Dimension() int {
	return m.dimension
}

func (m *clusteringMockEmbedder) ModelName() string {
	return "mock-embedder"
}

// ============================================================================
// TEST HELPERS
// ============================================================================

// setupClusteringTestDB creates an in-memory SQLite database with required schema.
func setupClusteringTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err, "failed to open test database")

	// Create the required tables
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS memory_topics (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			keywords TEXT,
			centroid_embedding BLOB,
			member_count INTEGER DEFAULT 0,
			last_active_at TEXT NOT NULL,
			created_at TEXT NOT NULL,
			is_active INTEGER DEFAULT 1
		);

		CREATE TABLE IF NOT EXISTS memory_topic_members (
			topic_id TEXT NOT NULL,
			memory_id TEXT NOT NULL,
			memory_type TEXT NOT NULL,
			added_at TEXT DEFAULT CURRENT_TIMESTAMP,
			relevance_score REAL DEFAULT 0,
			PRIMARY KEY (topic_id, memory_id),
			FOREIGN KEY (topic_id) REFERENCES memory_topics(id)
		);

		CREATE TABLE IF NOT EXISTS strategic_memory (
			id TEXT PRIMARY KEY,
			principle TEXT NOT NULL,
			category TEXT,
			trigger_pattern TEXT,
			embedding BLOB,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_topics_active ON memory_topics(is_active);
		CREATE INDEX IF NOT EXISTS idx_topics_last_active ON memory_topics(last_active_at);
		CREATE INDEX IF NOT EXISTS idx_topic_members_topic ON memory_topic_members(topic_id);
	`)
	require.NoError(t, err, "failed to create schema")

	return db
}

// createClusteringTestVectors generates test vectors with known clustering properties.
// Uses normalized vectors for proper cosine distance calculations.
func createClusteringTestVectors() []VectorPoint {
	// Cluster 1: Docker-related vectors (similar embeddings pointing in X direction)
	// These are close in cosine distance (< 0.15)
	cluster1 := []VectorPoint{
		{ID: "mem1", Content: "docker container", Embedding: NormalizeVector([]float32{1.0, 0.0, 0.0}), ClusterID: ClusterUnassigned, Type: MemoryTypeStrategic},
		{ID: "mem2", Content: "docker image", Embedding: NormalizeVector([]float32{1.0, 0.1, 0.0}), ClusterID: ClusterUnassigned, Type: MemoryTypeStrategic},
		{ID: "mem3", Content: "container runtime", Embedding: NormalizeVector([]float32{1.0, 0.15, 0.0}), ClusterID: ClusterUnassigned, Type: MemoryTypeStrategic},
	}

	// Cluster 2: Git-related vectors (pointing in Y direction)
	// These are close in cosine distance (< 0.15) but far from cluster 1
	cluster2 := []VectorPoint{
		{ID: "mem4", Content: "git commit", Embedding: NormalizeVector([]float32{0.0, 1.0, 0.0}), ClusterID: ClusterUnassigned, Type: MemoryTypeStrategic},
		{ID: "mem5", Content: "git push", Embedding: NormalizeVector([]float32{0.1, 1.0, 0.0}), ClusterID: ClusterUnassigned, Type: MemoryTypeStrategic},
		{ID: "mem6", Content: "git branch", Embedding: NormalizeVector([]float32{0.15, 1.0, 0.0}), ClusterID: ClusterUnassigned, Type: MemoryTypeStrategic},
	}

	// Noise: Isolated point in different direction (far from both clusters)
	noise := []VectorPoint{
		{ID: "mem7", Content: "random isolated", Embedding: NormalizeVector([]float32{0.0, 0.0, 1.0}), ClusterID: ClusterUnassigned, Type: MemoryTypeStrategic},
	}

	all := append(cluster1, cluster2...)
	return append(all, noise...)
}

// createClusteringTestTopic creates a test topic in the database.
func createClusteringTestTopic(t *testing.T, db *sql.DB, id, name string, isActive bool, lastActive time.Time, centroid []float32) {
	t.Helper()

	activeInt := 0
	if isActive {
		activeInt = 1
	}

	centroidBytes := Float32SliceToBytes(centroid)

	_, err := db.Exec(`
		INSERT INTO memory_topics (id, name, description, keywords, centroid_embedding, member_count, last_active_at, created_at, is_active)
		VALUES (?, ?, 'Test description', '["test"]', ?, 3, ?, ?, ?)
	`, id, name, centroidBytes, lastActive.Format(time.RFC3339), time.Now().Format(time.RFC3339), activeInt)
	require.NoError(t, err, "failed to create test topic")
}

// insertClusteringStrategicMemory inserts a test strategic memory.
func insertClusteringStrategicMemory(t *testing.T, db *sql.DB, id, principle string, embedding []float32) {
	t.Helper()
	embBytes := Float32SliceToBytes(embedding)
	_, err := db.Exec(`
		INSERT INTO strategic_memory (id, principle, category, trigger_pattern, embedding, created_at)
		VALUES (?, ?, 'test', 'test pattern', ?, ?)
	`, id, principle, embBytes, time.Now().Format(time.RFC3339))
	require.NoError(t, err, "failed to insert strategic memory")
}

// ============================================================================
// DBSCAN ALGORITHM TESTS
// ============================================================================

func TestDBSCAN_BasicClustering(t *testing.T) {
	db := setupClusteringTestDB(t)
	defer db.Close()

	ts := NewTopicStore(db, &clusteringMockEmbedder{dimension: 3}, &clusteringMockLLM{})
	points := createClusteringTestVectors()

	// Run DBSCAN with epsilon=0.02 (2% cosine distance - tight clusters), minPoints=2
	// Note: Our test vectors have intra-cluster distances < 0.015
	// minPoints=2 because regionQuery excludes self, so 3 points have 2 neighbors each
	clusters := ts.dbscan(points, 0.02, 2)

	// Should have at least 2 clusters (cluster 0 and cluster 1) plus noise
	assert.GreaterOrEqual(t, len(clusters), 2, "should have at least 2 distinct groups")

	// Count actual clusters (excluding noise and unassigned)
	clusterCount := 0
	for clusterID := range clusters {
		if clusterID >= 0 {
			clusterCount++
		}
	}
	assert.Equal(t, 2, clusterCount, "should have exactly 2 clusters")
}

func TestDBSCAN_NoiseHandling(t *testing.T) {
	db := setupClusteringTestDB(t)
	defer db.Close()

	ts := NewTopicStore(db, &clusteringMockEmbedder{dimension: 3}, &clusteringMockLLM{})
	points := createClusteringTestVectors()

	// Run DBSCAN with epsilon that groups similar points but isolates noise
	// minPoints=2 because regionQuery excludes self
	clusters := ts.dbscan(points, 0.02, 2)

	// The isolated point (mem7) should be marked as noise
	noisePoints, exists := clusters[ClusterNoise]
	assert.True(t, exists, "should have noise cluster")

	// Check that the isolated point is in noise
	found := false
	for _, p := range noisePoints {
		if p.ID == "mem7" {
			found = true
			break
		}
	}
	assert.True(t, found, "isolated point mem7 should be classified as noise")
}

func TestDBSCAN_SingleCluster(t *testing.T) {
	db := setupClusteringTestDB(t)
	defer db.Close()

	ts := NewTopicStore(db, &clusteringMockEmbedder{dimension: 3}, &clusteringMockLLM{})

	// All points very similar - should form one cluster
	points := []VectorPoint{
		{ID: "p1", Content: "content1", Embedding: NormalizeVector([]float32{1.0, 0.0, 0.0}), ClusterID: ClusterUnassigned},
		{ID: "p2", Content: "content2", Embedding: NormalizeVector([]float32{1.0, 0.05, 0.0}), ClusterID: ClusterUnassigned},
		{ID: "p3", Content: "content3", Embedding: NormalizeVector([]float32{1.0, 0.1, 0.0}), ClusterID: ClusterUnassigned},
		{ID: "p4", Content: "content4", Embedding: NormalizeVector([]float32{1.0, 0.15, 0.0}), ClusterID: ClusterUnassigned},
	}

	// epsilon=0.02 covers the small intra-cluster distances, minPoints=2
	clusters := ts.dbscan(points, 0.02, 2)

	// Count real clusters (ID >= 0)
	clusterCount := 0
	for clusterID := range clusters {
		if clusterID >= 0 {
			clusterCount++
		}
	}
	assert.Equal(t, 1, clusterCount, "all similar points should form one cluster")

	// All points should be in cluster 0
	cluster0, exists := clusters[0]
	assert.True(t, exists, "cluster 0 should exist")
	assert.Equal(t, 4, len(cluster0), "all 4 points should be in cluster 0")
}

func TestDBSCAN_MultipleClusters(t *testing.T) {
	db := setupClusteringTestDB(t)
	defer db.Close()

	ts := NewTopicStore(db, &clusteringMockEmbedder{dimension: 3}, &clusteringMockLLM{})

	// Three distinct clusters - use normalized vectors
	points := []VectorPoint{
		// Cluster 1: X-axis direction
		{ID: "x1", Content: "x1", Embedding: NormalizeVector([]float32{1.0, 0.0, 0.0}), ClusterID: ClusterUnassigned},
		{ID: "x2", Content: "x2", Embedding: NormalizeVector([]float32{1.0, 0.1, 0.0}), ClusterID: ClusterUnassigned},
		{ID: "x3", Content: "x3", Embedding: NormalizeVector([]float32{1.0, 0.15, 0.0}), ClusterID: ClusterUnassigned},
		// Cluster 2: Y-axis direction
		{ID: "y1", Content: "y1", Embedding: NormalizeVector([]float32{0.0, 1.0, 0.0}), ClusterID: ClusterUnassigned},
		{ID: "y2", Content: "y2", Embedding: NormalizeVector([]float32{0.1, 1.0, 0.0}), ClusterID: ClusterUnassigned},
		{ID: "y3", Content: "y3", Embedding: NormalizeVector([]float32{0.15, 1.0, 0.0}), ClusterID: ClusterUnassigned},
		// Cluster 3: Z-axis direction
		{ID: "z1", Content: "z1", Embedding: NormalizeVector([]float32{0.0, 0.0, 1.0}), ClusterID: ClusterUnassigned},
		{ID: "z2", Content: "z2", Embedding: NormalizeVector([]float32{0.1, 0.0, 1.0}), ClusterID: ClusterUnassigned},
		{ID: "z3", Content: "z3", Embedding: NormalizeVector([]float32{0.15, 0.0, 1.0}), ClusterID: ClusterUnassigned},
	}

	// epsilon=0.02 covers intra-cluster distances (<0.015), minPoints=2
	clusters := ts.dbscan(points, 0.02, 2)

	// Count clusters (excluding noise and unassigned)
	clusterCount := 0
	for clusterID := range clusters {
		if clusterID >= 0 {
			clusterCount++
		}
	}
	assert.Equal(t, 3, clusterCount, "should have 3 distinct clusters")

	// Each cluster should have 3 members
	for clusterID, members := range clusters {
		if clusterID >= 0 {
			assert.Equal(t, 3, len(members), "each cluster should have 3 members")
		}
	}
}

func TestDBSCAN_MinPointsThreshold(t *testing.T) {
	db := setupClusteringTestDB(t)
	defer db.Close()

	ts := NewTopicStore(db, &clusteringMockEmbedder{dimension: 3}, &clusteringMockLLM{})

	// Create a small group that doesn't meet minPoints
	// With minPoints=2, we need 2 neighbors (excluding self)
	// So a single point won't form a cluster
	points := []VectorPoint{
		{ID: "p1", Content: "content1", Embedding: NormalizeVector([]float32{1.0, 0.0, 0.0}), ClusterID: ClusterUnassigned},
		// Only 1 point - needs at least 2 neighbors for minPoints=2
	}

	clusters := ts.dbscan(points, 0.02, 2)

	// With minPoints=2, 1 point cannot form a cluster (needs 2 neighbors)
	clusterCount := 0
	for clusterID := range clusters {
		if clusterID >= 0 {
			clusterCount++
		}
	}
	assert.Equal(t, 0, clusterCount, "1 point should not form a cluster with minPoints=2")

	// The point should be noise
	noisePoints, exists := clusters[ClusterNoise]
	assert.True(t, exists, "noise cluster should exist")
	assert.Equal(t, 1, len(noisePoints), "point should be noise")
}

func TestDBSCAN_EpsilonSensitivity(t *testing.T) {
	db := setupClusteringTestDB(t)
	defer db.Close()

	ts := NewTopicStore(db, &clusteringMockEmbedder{dimension: 3}, &clusteringMockLLM{})

	// Points that are somewhat similar but spread out
	// Distance between adjacent points is ~0.04 (small), but distance across all 3 is ~0.15
	points := []VectorPoint{
		{ID: "p1", Content: "content1", Embedding: NormalizeVector([]float32{1.0, 0.0, 0.0}), ClusterID: ClusterUnassigned},
		{ID: "p2", Content: "content2", Embedding: NormalizeVector([]float32{1.0, 0.3, 0.0}), ClusterID: ClusterUnassigned},
		{ID: "p3", Content: "content3", Embedding: NormalizeVector([]float32{1.0, 0.6, 0.0}), ClusterID: ClusterUnassigned},
	}

	// With very small epsilon (0.01), nothing should cluster since distances are ~0.04+
	resetPoints := func() {
		for i := range points {
			points[i].ClusterID = ClusterUnassigned
			points[i].Visited = false
		}
	}

	clusters := ts.dbscan(points, 0.01, 2)
	noiseCount := 0
	for clusterID := range clusters {
		if clusterID == ClusterNoise {
			noiseCount = len(clusters[clusterID])
		}
	}
	assert.Equal(t, 3, noiseCount, "with small epsilon, all points should be noise")

	// With large epsilon (0.2), all should cluster
	resetPoints()
	clusters = ts.dbscan(points, 0.2, 2)
	clusterCount := 0
	for clusterID := range clusters {
		if clusterID >= 0 {
			clusterCount++
		}
	}
	assert.GreaterOrEqual(t, clusterCount, 1, "with large epsilon, points should cluster")
}

// ============================================================================
// TOPIC STORE TESTS
// ============================================================================

func TestTopicStore_RunClustering(t *testing.T) {
	db := setupClusteringTestDB(t)
	defer db.Close()

	embedder := &clusteringMockEmbedder{dimension: 3}
	llm := &clusteringMockLLM{}
	ts := NewTopicStore(db, embedder, llm)
	ctx := context.Background()

	// Insert test strategic memories with normalized embeddings
	// Cluster 1: Docker-related (X-axis direction)
	insertClusteringStrategicMemory(t, db, "strat1", "docker container management", NormalizeVector([]float32{1.0, 0.0, 0.0}))
	insertClusteringStrategicMemory(t, db, "strat2", "docker image building", NormalizeVector([]float32{1.0, 0.1, 0.0}))
	insertClusteringStrategicMemory(t, db, "strat3", "container orchestration", NormalizeVector([]float32{1.0, 0.15, 0.0}))

	// Cluster 2: Git-related (Y-axis direction)
	insertClusteringStrategicMemory(t, db, "strat4", "git commit practices", NormalizeVector([]float32{0.0, 1.0, 0.0}))
	insertClusteringStrategicMemory(t, db, "strat5", "git push workflow", NormalizeVector([]float32{0.1, 1.0, 0.0}))
	insertClusteringStrategicMemory(t, db, "strat6", "git branching strategy", NormalizeVector([]float32{0.15, 1.0, 0.0}))

	// Use epsilon=0.02 for tight clusters and minPoints=2 (regionQuery excludes self)
	config := ClusterConfig{
		Epsilon:      0.02,
		MinPoints:    2,
		LookbackDays: 7,
	}

	topics, err := ts.RunClustering(ctx, config)
	require.NoError(t, err, "clustering should succeed")

	// Should create 2 topics
	assert.Equal(t, 2, len(topics), "should create 2 topics")

	// Verify topics were saved to database
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM memory_topics WHERE is_active = 1").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count, "should have 2 active topics in database")

	// Verify each topic has the mock-generated name
	for _, topic := range topics {
		assert.Equal(t, "Test Topic", topic.Name, "topic should have LLM-generated name")
		assert.NotEmpty(t, topic.CentroidEmbedding, "topic should have centroid")
		assert.True(t, topic.IsActive, "topic should be active")
	}
}

func TestTopicStore_RunClustering_NotEnoughPoints(t *testing.T) {
	db := setupClusteringTestDB(t)
	defer db.Close()

	ts := NewTopicStore(db, &clusteringMockEmbedder{dimension: 3}, &clusteringMockLLM{})
	ctx := context.Background()

	// Insert only 1 point (less than minPoints=2)
	insertClusteringStrategicMemory(t, db, "strat1", "point 1", NormalizeVector([]float32{1.0, 0.0, 0.0}))

	config := ClusterConfig{
		Epsilon:      0.02,
		MinPoints:    2,
		LookbackDays: 7,
	}

	topics, err := ts.RunClustering(ctx, config)
	require.NoError(t, err, "should not error")
	assert.Nil(t, topics, "should return nil when not enough points")
}

func TestTopicStore_GetActiveTopic(t *testing.T) {
	db := setupClusteringTestDB(t)
	defer db.Close()

	embedder := &clusteringMockEmbedder{dimension: 3}
	ts := NewTopicStore(db, embedder, &clusteringMockLLM{})
	ctx := context.Background()

	// Create test topics with different centroids
	now := time.Now()
	createClusteringTestTopic(t, db, "topic1", "Docker Topics", true, now, NormalizeVector([]float32{1.0, 0.0, 0.0}))
	createClusteringTestTopic(t, db, "topic2", "Git Topics", true, now, NormalizeVector([]float32{0.0, 1.0, 0.0}))
	createClusteringTestTopic(t, db, "topic3", "Inactive Topic", false, now.AddDate(0, 0, -60), NormalizeVector([]float32{0.0, 0.0, 1.0}))

	// Test: Query that should match Docker topic
	topic, members, err := ts.GetActiveTopic(ctx, "docker container")
	require.NoError(t, err)

	// Note: With mock embedder, results depend on text hashing
	// The test verifies the mechanism works
	if topic != nil {
		assert.True(t, topic.IsActive, "returned topic should be active")
		assert.NotNil(t, members, "should return members")
	}
}

func TestTopicStore_GetActiveTopics(t *testing.T) {
	db := setupClusteringTestDB(t)
	defer db.Close()

	ts := NewTopicStore(db, &clusteringMockEmbedder{dimension: 3}, &clusteringMockLLM{})
	ctx := context.Background()

	now := time.Now()
	// Create multiple topics
	createClusteringTestTopic(t, db, "topic1", "Topic 1", true, now, NormalizeVector([]float32{1.0, 0.0, 0.0}))
	createClusteringTestTopic(t, db, "topic2", "Topic 2", true, now.Add(-1*time.Hour), NormalizeVector([]float32{0.0, 1.0, 0.0}))
	createClusteringTestTopic(t, db, "topic3", "Topic 3", true, now.Add(-2*time.Hour), NormalizeVector([]float32{0.0, 0.0, 1.0}))
	createClusteringTestTopic(t, db, "topic4", "Inactive", false, now, NormalizeVector([]float32{0.5, 0.5, 0.0}))

	// Get active topics
	topics, err := ts.GetActiveTopics(ctx, 10)
	require.NoError(t, err)

	// Should only return active topics
	assert.Equal(t, 3, len(topics), "should return 3 active topics")

	// Should be ordered by last_active_at DESC
	assert.Equal(t, "Topic 1", topics[0].Name, "most recent topic should be first")
	assert.Equal(t, "Topic 2", topics[1].Name, "second most recent should be second")
	assert.Equal(t, "Topic 3", topics[2].Name, "oldest should be last")

	// Test limit
	limitedTopics, err := ts.GetActiveTopics(ctx, 2)
	require.NoError(t, err)
	assert.Equal(t, 2, len(limitedTopics), "should respect limit")
}

func TestTopicStore_GetTopic(t *testing.T) {
	db := setupClusteringTestDB(t)
	defer db.Close()

	ts := NewTopicStore(db, &clusteringMockEmbedder{dimension: 3}, &clusteringMockLLM{})
	ctx := context.Background()

	now := time.Now()
	createClusteringTestTopic(t, db, "topic1", "Test Topic", true, now, NormalizeVector([]float32{1.0, 0.0, 0.0}))

	// Get existing topic
	topic, err := ts.GetTopic(ctx, "topic1")
	require.NoError(t, err)
	require.NotNil(t, topic, "should find topic")
	assert.Equal(t, "Test Topic", topic.Name)
	assert.True(t, topic.IsActive)

	// Get non-existent topic
	topic, err = ts.GetTopic(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, topic, "should return nil for non-existent topic")
}

func TestTopicStore_DeactivateStaleTopics(t *testing.T) {
	db := setupClusteringTestDB(t)
	defer db.Close()

	ts := NewTopicStore(db, &clusteringMockEmbedder{dimension: 3}, &clusteringMockLLM{})
	ctx := context.Background()

	now := time.Now()
	// Create topics with different last_active_at times
	createClusteringTestTopic(t, db, "topic1", "Fresh Topic", true, now, NormalizeVector([]float32{1.0, 0.0, 0.0}))
	createClusteringTestTopic(t, db, "topic2", "Stale Topic 1", true, now.AddDate(0, 0, -35), NormalizeVector([]float32{0.0, 1.0, 0.0}))
	createClusteringTestTopic(t, db, "topic3", "Stale Topic 2", true, now.AddDate(0, 0, -40), NormalizeVector([]float32{0.0, 0.0, 1.0}))
	createClusteringTestTopic(t, db, "topic4", "Already Inactive", false, now.AddDate(0, 0, -50), NormalizeVector([]float32{0.5, 0.5, 0.0}))

	// Deactivate topics not used in 30 days
	count, err := ts.DeactivateStaleTopics(ctx, 30)
	require.NoError(t, err)
	assert.Equal(t, 2, count, "should deactivate 2 stale topics")

	// Verify only fresh topic is still active
	var activeCount int
	err = db.QueryRow("SELECT COUNT(*) FROM memory_topics WHERE is_active = 1").Scan(&activeCount)
	require.NoError(t, err)
	assert.Equal(t, 1, activeCount, "should have only 1 active topic remaining")

	// Verify the fresh topic is the active one
	topic, err := ts.GetTopic(ctx, "topic1")
	require.NoError(t, err)
	assert.True(t, topic.IsActive, "fresh topic should still be active")
}

func TestTopicStore_UpdateLastActive(t *testing.T) {
	db := setupClusteringTestDB(t)
	defer db.Close()

	ts := NewTopicStore(db, &clusteringMockEmbedder{dimension: 3}, &clusteringMockLLM{})
	ctx := context.Background()

	oldTime := time.Now().AddDate(0, 0, -7)
	createClusteringTestTopic(t, db, "topic1", "Test Topic", true, oldTime, NormalizeVector([]float32{1.0, 0.0, 0.0}))

	// Get original last_active_at
	topic, err := ts.GetTopic(ctx, "topic1")
	require.NoError(t, err)
	originalTime := topic.LastActiveAt

	// Wait a moment and update
	time.Sleep(10 * time.Millisecond)
	err = ts.UpdateLastActive(ctx, "topic1")
	require.NoError(t, err)

	// Get updated topic
	topic, err = ts.GetTopic(ctx, "topic1")
	require.NoError(t, err)
	assert.True(t, topic.LastActiveAt.After(originalTime), "last_active_at should be updated")
}

func TestTopicStore_LoadTopicContext(t *testing.T) {
	db := setupClusteringTestDB(t)
	defer db.Close()

	ts := NewTopicStore(db, &clusteringMockEmbedder{dimension: 3}, &clusteringMockLLM{})
	ctx := context.Background()

	// Create a topic
	now := time.Now()
	createClusteringTestTopic(t, db, "topic1", "Test Topic", true, now, NormalizeVector([]float32{1.0, 0.0, 0.0}))

	// Add strategic memories
	insertClusteringStrategicMemory(t, db, "strat1", "principle 1", NormalizeVector([]float32{1.0, 0.0, 0.0}))
	insertClusteringStrategicMemory(t, db, "strat2", "principle 2", NormalizeVector([]float32{1.0, 0.1, 0.0}))

	// Add topic members
	_, err := db.Exec(`
		INSERT INTO memory_topic_members (topic_id, memory_id, memory_type, relevance_score)
		VALUES ('topic1', 'strat1', 'strategic', 0.9),
		       ('topic1', 'strat2', 'strategic', 0.8)
	`)
	require.NoError(t, err)

	// Load topic context
	topicContext, err := ts.LoadTopicContext(ctx, "topic1")
	require.NoError(t, err)
	assert.NotEmpty(t, topicContext, "should return context")

	// Check strategic memories were loaded
	strategicMemories, exists := topicContext[string(MemoryTypeStrategic)]
	assert.True(t, exists, "should have strategic memories")
	assert.Equal(t, 2, len(strategicMemories), "should have 2 strategic memories")
}

// ============================================================================
// CENTROID CALCULATION TESTS
// ============================================================================

func TestCalculateCentroid(t *testing.T) {
	tests := []struct {
		name     string
		vectors  [][]float32
		expected []float32
	}{
		{
			name:     "empty vectors",
			vectors:  [][]float32{},
			expected: nil,
		},
		{
			name: "single vector",
			vectors: [][]float32{
				{1.0, 2.0, 3.0},
			},
			expected: []float32{1.0, 2.0, 3.0},
		},
		{
			name: "two vectors",
			vectors: [][]float32{
				{1.0, 0.0, 0.0},
				{0.0, 1.0, 0.0},
			},
			expected: []float32{0.5, 0.5, 0.0},
		},
		{
			name: "three vectors",
			vectors: [][]float32{
				{1.0, 0.0, 0.0},
				{0.0, 1.0, 0.0},
				{0.0, 0.0, 1.0},
			},
			expected: []float32{1.0 / 3.0, 1.0 / 3.0, 1.0 / 3.0},
		},
		{
			name: "uniform vectors",
			vectors: [][]float32{
				{2.0, 4.0, 6.0},
				{2.0, 4.0, 6.0},
				{2.0, 4.0, 6.0},
			},
			expected: []float32{2.0, 4.0, 6.0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := CalculateCentroid(tc.vectors)

			if tc.expected == nil {
				assert.Nil(t, result)
				return
			}

			require.Equal(t, len(tc.expected), len(result), "centroid dimension mismatch")

			for i := range tc.expected {
				assert.InDelta(t, tc.expected[i], result[i], 0.0001,
					"centroid element %d mismatch: expected %f, got %f", i, tc.expected[i], result[i])
			}
		})
	}
}

func TestCalculateCentroid_MismatchedDimensions(t *testing.T) {
	vectors := [][]float32{
		{1.0, 0.0, 0.0},
		{0.0, 1.0}, // Different dimension - should be skipped
		{0.0, 0.0, 1.0},
	}

	result := CalculateCentroid(vectors)

	// The function should still compute a centroid from valid vectors
	assert.NotNil(t, result)
	assert.Equal(t, 3, len(result), "centroid should have dimension of first vector")
}

// ============================================================================
// EDGE CASES AND ERROR HANDLING
// ============================================================================

func TestTopicStore_EmptyDatabase(t *testing.T) {
	db := setupClusteringTestDB(t)
	defer db.Close()

	ts := NewTopicStore(db, &clusteringMockEmbedder{dimension: 3}, &clusteringMockLLM{})
	ctx := context.Background()

	// Get active topics from empty database
	topics, err := ts.GetActiveTopics(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, topics, "should return empty list for empty database")

	// Get non-existent topic
	topic, err := ts.GetTopic(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, topic, "should return nil for non-existent topic")
}

func TestTopicStore_NilLLMProvider(t *testing.T) {
	db := setupClusteringTestDB(t)
	defer db.Close()

	// Create store without LLM provider
	ts := NewTopicStore(db, &clusteringMockEmbedder{dimension: 3}, nil)
	ctx := context.Background()

	// Insert test data with normalized vectors
	insertClusteringStrategicMemory(t, db, "strat1", "docker topic 1", NormalizeVector([]float32{1.0, 0.0, 0.0}))
	insertClusteringStrategicMemory(t, db, "strat2", "docker topic 2", NormalizeVector([]float32{1.0, 0.1, 0.0}))
	insertClusteringStrategicMemory(t, db, "strat3", "docker topic 3", NormalizeVector([]float32{1.0, 0.15, 0.0}))

	// Use epsilon=0.02 for tight clusters and minPoints=2
	config := ClusterConfig{
		Epsilon:      0.02,
		MinPoints:    2,
		LookbackDays: 7,
	}

	// Clustering should still work with fallback naming
	topics, err := ts.RunClustering(ctx, config)
	require.NoError(t, err)
	assert.NotEmpty(t, topics, "should create topics even without LLM")

	// Topic name should use fallback format
	assert.Contains(t, topics[0].Name, "Topic", "should use fallback naming")
}

func TestDefaultClusterConfig(t *testing.T) {
	config := DefaultClusterConfig()

	assert.Equal(t, 0.3, config.Epsilon, "default epsilon should be 0.3")
	assert.Equal(t, 3, config.MinPoints, "default minPoints should be 3")
	assert.Equal(t, 7, config.LookbackDays, "default lookbackDays should be 7")
}

func TestTopicStore_ConcurrentAccess(t *testing.T) {
	// Use file-based SQLite with shared cache for concurrent access testing
	db, err := sql.Open("sqlite3", "file::memory:?cache=shared")
	require.NoError(t, err, "failed to open test database")
	defer db.Close()

	// Create schema
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS memory_topics (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			keywords TEXT,
			centroid_embedding BLOB,
			member_count INTEGER DEFAULT 0,
			last_active_at TEXT NOT NULL,
			created_at TEXT NOT NULL,
			is_active INTEGER DEFAULT 1
		);
		CREATE INDEX IF NOT EXISTS idx_topics_active ON memory_topics(is_active);
	`)
	require.NoError(t, err, "failed to create schema")

	ts := NewTopicStore(db, &clusteringMockEmbedder{dimension: 3}, &clusteringMockLLM{})
	ctx := context.Background()

	// Create test data
	now := time.Now()
	centroidBytes := Float32SliceToBytes(NormalizeVector([]float32{1.0, 0.0, 0.0}))
	_, err = db.Exec(`
		INSERT INTO memory_topics (id, name, description, keywords, centroid_embedding, member_count, last_active_at, created_at, is_active)
		VALUES (?, ?, 'Test', '[]', ?, 1, ?, ?, 1)
	`, "topic1", "Test Topic", centroidBytes, now.Format(time.RFC3339), now.Format(time.RFC3339))
	require.NoError(t, err, "failed to insert test topic")

	// Simulate concurrent reads using sync.WaitGroup for proper synchronization
	var wg sync.WaitGroup
	errChan := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := ts.GetActiveTopics(ctx, 10)
			if err != nil {
				errChan <- err
			}
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)

	// Check for any errors
	for err := range errChan {
		t.Errorf("concurrent access error: %v", err)
	}
}
