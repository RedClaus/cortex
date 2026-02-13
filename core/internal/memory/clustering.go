// Package memory provides enhanced memory capabilities for Cortex.
// This file implements DBSCAN-based topic clustering for automatic
// grouping of related memories.
package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// ============================================================================
// TOPIC TYPES
// ============================================================================

// Topic represents a cluster of related memories.
type Topic struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	Keywords          []string  `json:"keywords"`
	CentroidEmbedding []float32 `json:"centroid_embedding,omitempty"`
	MemberCount       int       `json:"member_count"`
	LastActiveAt      time.Time `json:"last_active_at"`
	CreatedAt         time.Time `json:"created_at"`
	IsActive          bool      `json:"is_active"`
}

// TopicMember represents a memory's membership in a topic.
type TopicMember struct {
	TopicID        string     `json:"topic_id"`
	MemoryID       string     `json:"memory_id"`
	MemoryType     MemoryType `json:"memory_type"`
	AddedAt        time.Time  `json:"added_at"`
	RelevanceScore float64    `json:"relevance_score"`
}

// ClusterConfig configures the DBSCAN clustering algorithm.
type ClusterConfig struct {
	// Epsilon is the maximum cosine distance for points to be considered neighbors.
	// Lower values create tighter clusters. Default: 0.3
	Epsilon float64 `json:"epsilon"`

	// MinPoints is the minimum number of memories required to form a cluster.
	// Default: 3
	MinPoints int `json:"min_points"`

	// LookbackDays is how many days of memories to consider for clustering.
	// Default: 7
	LookbackDays int `json:"lookback_days"`
}

// DefaultClusterConfig returns sensible clustering defaults.
func DefaultClusterConfig() ClusterConfig {
	return ClusterConfig{
		Epsilon:      0.3,
		MinPoints:    3,
		LookbackDays: 7,
	}
}

// VectorPoint represents a memory as a point in vector space for clustering.
type VectorPoint struct {
	ID        string     `json:"id"`
	Content   string     `json:"content"`
	Type      MemoryType `json:"type"`
	Embedding []float32  `json:"embedding"`
	ClusterID int        `json:"cluster_id"` // -1 = unassigned, -2 = noise
	Visited   bool       `json:"visited"`
}

// Cluster ID constants
const (
	ClusterUnassigned = -1
	ClusterNoise      = -2
)

// ============================================================================
// TOPIC STORE
// ============================================================================

// TopicStore manages topic clustering and retrieval.
type TopicStore struct {
	db       *sql.DB
	embedder Embedder
	llm      LLMProvider
}

// NewTopicStore creates a new topic store.
func NewTopicStore(db *sql.DB, embedder Embedder, llm LLMProvider) *TopicStore {
	return &TopicStore{
		db:       db,
		embedder: embedder,
		llm:      llm,
	}
}

// ============================================================================
// PUBLIC METHODS
// ============================================================================

// RunClustering executes the full clustering pipeline.
// It fetches recent memories, runs DBSCAN, and creates/updates topics.
func (ts *TopicStore) RunClustering(ctx context.Context, config ClusterConfig) ([]Topic, error) {
	log.Info().
		Float64("epsilon", config.Epsilon).
		Int("min_points", config.MinPoints).
		Int("lookback_days", config.LookbackDays).
		Msg("starting topic clustering")

	// Step 1: Fetch recent vectors
	points, err := ts.fetchRecentVectors(ctx, config.LookbackDays)
	if err != nil {
		return nil, fmt.Errorf("fetch vectors: %w", err)
	}

	if len(points) < config.MinPoints {
		log.Info().
			Int("points", len(points)).
			Int("min_required", config.MinPoints).
			Msg("not enough points for clustering")
		return nil, nil
	}

	log.Debug().Int("points", len(points)).Msg("fetched vectors for clustering")

	// Step 2: Run DBSCAN
	clusters := ts.dbscan(points, config.Epsilon, config.MinPoints)

	log.Info().
		Int("clusters_found", len(clusters)).
		Msg("DBSCAN clustering complete")

	// Step 3: Create/update topics for each cluster
	var topics []Topic
	for clusterID, members := range clusters {
		if clusterID < 0 {
			continue // Skip noise and unassigned
		}

		topic, err := ts.createOrUpdateTopic(ctx, members)
		if err != nil {
			log.Error().Err(err).Int("cluster_id", clusterID).Msg("failed to create topic")
			continue
		}

		topics = append(topics, *topic)
		log.Debug().
			Str("topic_id", topic.ID).
			Str("name", topic.Name).
			Int("members", len(members)).
			Msg("topic created/updated")
	}

	return topics, nil
}

// GetActiveTopic finds the most relevant active topic for a query.
func (ts *TopicStore) GetActiveTopic(ctx context.Context, query string) (*Topic, []TopicMember, error) {
	// Generate embedding for query
	queryEmb, err := ts.embedder.Embed(ctx, query)
	if err != nil {
		return nil, nil, fmt.Errorf("embed query: %w", err)
	}

	// Get active topics
	topics, err := ts.GetActiveTopics(ctx, 50) // Get up to 50 active topics
	if err != nil {
		return nil, nil, fmt.Errorf("get active topics: %w", err)
	}

	if len(topics) == 0 {
		return nil, nil, nil
	}

	// Find best matching topic by centroid similarity
	var bestTopic *Topic
	bestScore := float64(-1)

	for i := range topics {
		if topics[i].CentroidEmbedding == nil {
			continue
		}

		similarity := CosineSimilarity(queryEmb, topics[i].CentroidEmbedding)
		if similarity > bestScore {
			bestScore = similarity
			bestTopic = &topics[i]
		}
	}

	// Require minimum similarity
	if bestTopic == nil || bestScore < SimilarityThreshold {
		return nil, nil, nil
	}

	// Get topic members
	members, err := ts.getTopicMembers(ctx, bestTopic.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("get topic members: %w", err)
	}

	// Update last active timestamp
	if err := ts.UpdateLastActive(ctx, bestTopic.ID); err != nil {
		log.Warn().Err(err).Str("topic_id", bestTopic.ID).Msg("failed to update last active")
	}

	return bestTopic, members, nil
}

// LoadTopicContext loads all memories for a topic, grouped by type.
func (ts *TopicStore) LoadTopicContext(ctx context.Context, topicID string) (map[string][]GenericMemory, error) {
	result := make(map[string][]GenericMemory)

	members, err := ts.getTopicMembers(ctx, topicID)
	if err != nil {
		return nil, fmt.Errorf("get topic members: %w", err)
	}

	for _, member := range members {
		memory, err := ts.loadMemoryByTypeAndID(ctx, member.MemoryType, member.MemoryID)
		if err != nil {
			log.Warn().
				Err(err).
				Str("memory_id", member.MemoryID).
				Str("type", string(member.MemoryType)).
				Msg("failed to load topic member")
			continue
		}

		typeKey := string(member.MemoryType)
		result[typeKey] = append(result[typeKey], *memory)
	}

	return result, nil
}

// GetTopic retrieves a topic by ID.
func (ts *TopicStore) GetTopic(ctx context.Context, id string) (*Topic, error) {
	row := ts.db.QueryRowContext(ctx, `
		SELECT id, name, description, keywords, centroid_embedding,
		       member_count, last_active_at, created_at, is_active
		FROM memory_topics
		WHERE id = ?
	`, id)

	return ts.scanTopic(row)
}

// GetActiveTopics retrieves active topics ordered by last activity.
func (ts *TopicStore) GetActiveTopics(ctx context.Context, limit int) ([]Topic, error) {
	rows, err := ts.db.QueryContext(ctx, `
		SELECT id, name, description, keywords, centroid_embedding,
		       member_count, last_active_at, created_at, is_active
		FROM memory_topics
		WHERE is_active = 1
		ORDER BY last_active_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query active topics: %w", err)
	}
	defer rows.Close()

	var topics []Topic
	for rows.Next() {
		topic, err := ts.scanTopicRow(rows)
		if err != nil {
			return nil, err
		}
		topics = append(topics, *topic)
	}

	return topics, rows.Err()
}

// DeactivateStaleTopics marks topics as inactive if they haven't been used recently.
func (ts *TopicStore) DeactivateStaleTopics(ctx context.Context, staleDays int) (int, error) {
	staleTime := time.Now().AddDate(0, 0, -staleDays)

	result, err := ts.db.ExecContext(ctx, `
		UPDATE memory_topics
		SET is_active = 0
		WHERE is_active = 1 AND last_active_at < ?
	`, staleTime.Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("deactivate stale topics: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected > 0 {
		log.Info().Int64("deactivated", affected).Msg("deactivated stale topics")
	}

	return int(affected), nil
}

// UpdateLastActive updates the last_active_at timestamp for a topic.
func (ts *TopicStore) UpdateLastActive(ctx context.Context, topicID string) error {
	_, err := ts.db.ExecContext(ctx, `
		UPDATE memory_topics
		SET last_active_at = datetime('now')
		WHERE id = ?
	`, topicID)
	return err
}

// ============================================================================
// DBSCAN IMPLEMENTATION
// ============================================================================

// dbscan implements the DBSCAN clustering algorithm.
// Returns a map of cluster ID -> members.
func (ts *TopicStore) dbscan(points []VectorPoint, epsilon float64, minPoints int) map[int][]VectorPoint {
	clusterID := 0

	for i := range points {
		if points[i].Visited {
			continue
		}

		points[i].Visited = true
		neighbors := ts.regionQuery(points, i, epsilon)

		if len(neighbors) < minPoints {
			points[i].ClusterID = ClusterNoise
		} else {
			points[i].ClusterID = clusterID
			ts.expandCluster(points, i, neighbors, clusterID, epsilon, minPoints)
			clusterID++
		}
	}

	// Group points by cluster
	clusters := make(map[int][]VectorPoint)
	for _, p := range points {
		clusters[p.ClusterID] = append(clusters[p.ClusterID], p)
	}

	return clusters
}

// regionQuery finds all points within epsilon distance of the given point.
func (ts *TopicStore) regionQuery(points []VectorPoint, pointIdx int, epsilon float64) []int {
	var neighbors []int
	targetEmb := points[pointIdx].Embedding

	for i := range points {
		if i == pointIdx {
			continue
		}
		distance := CosineDistance(targetEmb, points[i].Embedding)
		if distance <= epsilon {
			neighbors = append(neighbors, i)
		}
	}

	return neighbors
}

// expandCluster expands a cluster by adding density-reachable points.
func (ts *TopicStore) expandCluster(
	points []VectorPoint,
	pointIdx int,
	neighbors []int,
	clusterID int,
	epsilon float64,
	minPoints int,
) {
	// Use a queue to process neighbors
	queue := make([]int, len(neighbors))
	copy(queue, neighbors)

	for len(queue) > 0 {
		// Pop first element
		neighborIdx := queue[0]
		queue = queue[1:]

		// Change noise to border point
		if points[neighborIdx].ClusterID == ClusterNoise {
			points[neighborIdx].ClusterID = clusterID
		}

		if points[neighborIdx].Visited {
			continue
		}

		points[neighborIdx].Visited = true
		points[neighborIdx].ClusterID = clusterID

		// Find neighbors of this neighbor
		neighborNeighbors := ts.regionQuery(points, neighborIdx, epsilon)

		if len(neighborNeighbors) >= minPoints {
			// Add new neighbors to queue
			queue = append(queue, neighborNeighbors...)
		}
	}
}

// ============================================================================
// TOPIC CREATION & MANAGEMENT
// ============================================================================

// createOrUpdateTopic creates a new topic or updates an existing similar one.
func (ts *TopicStore) createOrUpdateTopic(ctx context.Context, members []VectorPoint) (*Topic, error) {
	// Calculate centroid
	embeddings := make([][]float32, len(members))
	for i, m := range members {
		embeddings[i] = m.Embedding
	}
	centroid := CalculateCentroid(embeddings)

	// Check for existing similar topic
	existingTopic, err := ts.findSimilarTopic(ctx, centroid, SimilarityThreshold)
	if err != nil {
		return nil, fmt.Errorf("find similar topic: %w", err)
	}

	if existingTopic != nil {
		// Update existing topic
		existingTopic.CentroidEmbedding = centroid
		if err := ts.saveTopic(ctx, existingTopic, members); err != nil {
			return nil, fmt.Errorf("update topic: %w", err)
		}
		return existingTopic, nil
	}

	// Generate topic name and description
	name, description, keywords := ts.nameTopic(ctx, members)

	topic := &Topic{
		ID:                fmt.Sprintf("topic_%s", uuid.New().String()[:8]),
		Name:              name,
		Description:       description,
		Keywords:          keywords,
		CentroidEmbedding: centroid,
		MemberCount:       len(members),
		LastActiveAt:      time.Now(),
		CreatedAt:         time.Now(),
		IsActive:          true,
	}

	if err := ts.saveTopic(ctx, topic, members); err != nil {
		return nil, fmt.Errorf("save new topic: %w", err)
	}

	return topic, nil
}

// TopicNamingPrompt is the prompt template for topic name generation.
const TopicNamingPrompt = `Based on these related memories, suggest a topic name:

SAMPLES:
%s

Respond in this format:
NAME: [Short topic name, e.g., "Docker Debugging", "Auth Service Refactor"]
DESCRIPTION: [One sentence description]
KEYWORDS: [comma-separated keywords]`

// nameTopic generates a name, description, and keywords for a topic using LLM.
func (ts *TopicStore) nameTopic(ctx context.Context, members []VectorPoint) (name, description string, keywords []string) {
	// Prepare samples (max 5)
	var samples []string
	maxSamples := 5
	if len(members) < maxSamples {
		maxSamples = len(members)
	}

	for i := 0; i < maxSamples; i++ {
		samples = append(samples, fmt.Sprintf("- %s", truncateString(members[i].Content, 200)))
	}

	// Try LLM naming if available
	if ts.llm != nil {
		prompt := fmt.Sprintf(TopicNamingPrompt, strings.Join(samples, "\n"))
		response, err := ts.llm.Complete(ctx, prompt)

		if err == nil {
			name = ExtractField(response, "NAME:")
			description = ExtractField(response, "DESCRIPTION:")
			keywordsStr := ExtractField(response, "KEYWORDS:")
			keywords = ParseKeywords(keywordsStr)

			if name != "" {
				return name, description, keywords
			}
		} else {
			log.Warn().Err(err).Msg("LLM topic naming failed, using fallback")
		}
	}

	// Fallback: generate from first member
	name = fmt.Sprintf("Topic %s", time.Now().Format("2006-01-02"))
	if len(members) > 0 {
		content := members[0].Content
		if len(content) > 50 {
			content = content[:50] + "..."
		}
		description = fmt.Sprintf("Cluster around: %s", content)
	}

	return name, description, nil
}

// findSimilarTopic searches for an existing topic with similar centroid.
func (ts *TopicStore) findSimilarTopic(ctx context.Context, centroid []float32, threshold float64) (*Topic, error) {
	topics, err := ts.GetActiveTopics(ctx, 100)
	if err != nil {
		return nil, err
	}

	for i := range topics {
		if topics[i].CentroidEmbedding == nil {
			continue
		}

		similarity := CosineSimilarity(centroid, topics[i].CentroidEmbedding)
		if similarity >= threshold {
			return &topics[i], nil
		}
	}

	return nil, nil
}

// CreateTopic creates a new topic without members (for CR-018 introspection).
// This is a simplified version of saveTopic used by the acquisition engine.
func (ts *TopicStore) CreateTopic(ctx context.Context, topic *Topic) error {
	if topic == nil {
		return fmt.Errorf("topic is nil")
	}

	// Set defaults if not provided
	if topic.ID == "" {
		topic.ID = "topic_" + uuid.New().String()
	}
	if topic.CreatedAt.IsZero() {
		topic.CreatedAt = time.Now()
	}
	if topic.LastActiveAt.IsZero() {
		topic.LastActiveAt = time.Now()
	}
	topic.IsActive = true

	// Encode keywords as JSON
	keywordsJSON, err := json.Marshal(topic.Keywords)
	if err != nil {
		keywordsJSON = []byte("[]")
	}

	// Encode centroid as bytes (may be nil for newly created topics)
	centroidBytes := Float32SliceToBytes(topic.CentroidEmbedding)

	// Insert topic
	_, err = ts.db.ExecContext(ctx, `
		INSERT INTO memory_topics (
			id, name, description, keywords, centroid_embedding,
			member_count, last_active_at, created_at, is_active
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			keywords = excluded.keywords,
			centroid_embedding = excluded.centroid_embedding,
			member_count = excluded.member_count,
			last_active_at = excluded.last_active_at,
			is_active = excluded.is_active
	`, topic.ID, topic.Name, topic.Description, string(keywordsJSON),
		centroidBytes, topic.MemberCount, topic.LastActiveAt.Format(time.RFC3339),
		topic.CreatedAt.Format(time.RFC3339), topic.IsActive)

	if err != nil {
		return fmt.Errorf("create topic: %w", err)
	}

	return nil
}

// saveTopic persists a topic and its members to the database.
func (ts *TopicStore) saveTopic(ctx context.Context, topic *Topic, members []VectorPoint) error {
	tx, err := ts.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Encode keywords as JSON
	keywordsJSON, err := json.Marshal(topic.Keywords)
	if err != nil {
		keywordsJSON = []byte("[]")
	}

	// Encode centroid as bytes
	centroidBytes := Float32SliceToBytes(topic.CentroidEmbedding)

	// Upsert topic
	_, err = tx.ExecContext(ctx, `
		INSERT INTO memory_topics (
			id, name, description, keywords, centroid_embedding,
			last_active_at, created_at, is_active
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			keywords = excluded.keywords,
			centroid_embedding = excluded.centroid_embedding,
			last_active_at = excluded.last_active_at,
			is_active = excluded.is_active
	`, topic.ID, topic.Name, topic.Description, string(keywordsJSON),
		centroidBytes, topic.LastActiveAt.Format(time.RFC3339),
		topic.CreatedAt.Format(time.RFC3339), topic.IsActive)
	if err != nil {
		return fmt.Errorf("upsert topic: %w", err)
	}

	// Clear existing members and add new ones
	_, err = tx.ExecContext(ctx, `DELETE FROM memory_topic_members WHERE topic_id = ?`, topic.ID)
	if err != nil {
		return fmt.Errorf("clear members: %w", err)
	}

	// Insert members
	for _, member := range members {
		// Calculate relevance score based on distance to centroid
		relevance := CosineSimilarity(topic.CentroidEmbedding, member.Embedding)

		_, err = tx.ExecContext(ctx, `
			INSERT INTO memory_topic_members (topic_id, memory_id, memory_type, relevance_score)
			VALUES (?, ?, ?, ?)
		`, topic.ID, member.ID, string(member.Type), relevance)
		if err != nil {
			log.Warn().Err(err).Str("member_id", member.ID).Msg("failed to insert member")
		}
	}

	return tx.Commit()
}

// getTopicMembers retrieves all members of a topic.
func (ts *TopicStore) getTopicMembers(ctx context.Context, topicID string) ([]TopicMember, error) {
	rows, err := ts.db.QueryContext(ctx, `
		SELECT topic_id, memory_id, memory_type, added_at, relevance_score
		FROM memory_topic_members
		WHERE topic_id = ?
		ORDER BY relevance_score DESC
	`, topicID)
	if err != nil {
		return nil, fmt.Errorf("query topic members: %w", err)
	}
	defer rows.Close()

	var members []TopicMember
	for rows.Next() {
		var m TopicMember
		var memType string
		var addedAt string

		if err := rows.Scan(&m.TopicID, &m.MemoryID, &memType, &addedAt, &m.RelevanceScore); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}

		m.MemoryType = MemoryType(memType)
		if t, err := time.Parse(time.RFC3339, addedAt); err == nil {
			m.AddedAt = t
		} else if t, err := time.Parse("2006-01-02 15:04:05", addedAt); err == nil {
			m.AddedAt = t
		}

		members = append(members, m)
	}

	return members, rows.Err()
}

// ============================================================================
// DATA FETCHING
// ============================================================================

// fetchRecentVectors retrieves recent memories with embeddings for clustering.
func (ts *TopicStore) fetchRecentVectors(ctx context.Context, days int) ([]VectorPoint, error) {
	cutoff := time.Now().AddDate(0, 0, -days)

	// Query strategic_memory table
	rows, err := ts.db.QueryContext(ctx, `
		SELECT id, principle, embedding, created_at
		FROM strategic_memory
		WHERE embedding IS NOT NULL
		  AND created_at >= ?
		ORDER BY created_at DESC
	`, cutoff.Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("query strategic memory: %w", err)
	}
	defer rows.Close()

	var points []VectorPoint
	for rows.Next() {
		var id, content string
		var embBytes []byte
		var createdAt string

		if err := rows.Scan(&id, &content, &embBytes, &createdAt); err != nil {
			log.Warn().Err(err).Msg("failed to scan vector")
			continue
		}

		embedding := BytesToFloat32Slice(embBytes)
		if embedding == nil || len(embedding) == 0 {
			continue
		}

		points = append(points, VectorPoint{
			ID:        id,
			Content:   content,
			Type:      MemoryTypeStrategic,
			Embedding: embedding,
			ClusterID: ClusterUnassigned,
			Visited:   false,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}

	return points, nil
}

// loadMemoryByTypeAndID loads a memory by its type and ID.
func (ts *TopicStore) loadMemoryByTypeAndID(ctx context.Context, memType MemoryType, memID string) (*GenericMemory, error) {
	switch memType {
	case MemoryTypeStrategic:
		return ts.loadStrategicMemory(ctx, memID)
	default:
		return nil, fmt.Errorf("unsupported memory type: %s", memType)
	}
}

// loadStrategicMemory loads a strategic memory by ID.
func (ts *TopicStore) loadStrategicMemory(ctx context.Context, id string) (*GenericMemory, error) {
	row := ts.db.QueryRowContext(ctx, `
		SELECT id, principle, category, trigger_pattern, embedding
		FROM strategic_memory
		WHERE id = ?
	`, id)

	var memID, principle string
	var category, triggerPattern sql.NullString
	var embBytes []byte

	if err := row.Scan(&memID, &principle, &category, &triggerPattern, &embBytes); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("strategic memory not found: %s", id)
		}
		return nil, fmt.Errorf("scan strategic memory: %w", err)
	}

	embedding := BytesToFloat32Slice(embBytes)

	metadata := make(map[string]any)
	if category.Valid {
		metadata["category"] = category.String
	}
	if triggerPattern.Valid {
		metadata["trigger_pattern"] = triggerPattern.String
	}

	return &GenericMemory{
		ID:        memID,
		Type:      MemoryTypeStrategic,
		Content:   principle,
		Embedding: embedding,
		Metadata:  metadata,
	}, nil
}

// ============================================================================
// HELPERS
// ============================================================================

// scanTopic scans a single topic row.
func (ts *TopicStore) scanTopic(row *sql.Row) (*Topic, error) {
	var topic Topic
	var keywordsJSON, description sql.NullString
	var centroidBytes []byte
	var lastActiveAt, createdAt string
	var isActive int

	err := row.Scan(
		&topic.ID, &topic.Name, &description, &keywordsJSON,
		&centroidBytes, &topic.MemberCount, &lastActiveAt, &createdAt, &isActive,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan topic: %w", err)
	}

	topic.Description = description.String
	topic.IsActive = isActive == 1
	topic.CentroidEmbedding = BytesToFloat32Slice(centroidBytes)

	// Parse keywords
	if keywordsJSON.Valid && keywordsJSON.String != "" {
		json.Unmarshal([]byte(keywordsJSON.String), &topic.Keywords)
	}

	// Parse timestamps
	if t, err := time.Parse(time.RFC3339, lastActiveAt); err == nil {
		topic.LastActiveAt = t
	} else if t, err := time.Parse("2006-01-02 15:04:05", lastActiveAt); err == nil {
		topic.LastActiveAt = t
	}

	if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
		topic.CreatedAt = t
	} else if t, err := time.Parse("2006-01-02 15:04:05", createdAt); err == nil {
		topic.CreatedAt = t
	}

	return &topic, nil
}

// scanTopicRow scans a topic from rows iterator.
func (ts *TopicStore) scanTopicRow(rows *sql.Rows) (*Topic, error) {
	var topic Topic
	var keywordsJSON, description sql.NullString
	var centroidBytes []byte
	var lastActiveAt, createdAt string
	var isActive int

	err := rows.Scan(
		&topic.ID, &topic.Name, &description, &keywordsJSON,
		&centroidBytes, &topic.MemberCount, &lastActiveAt, &createdAt, &isActive,
	)
	if err != nil {
		return nil, fmt.Errorf("scan topic row: %w", err)
	}

	topic.Description = description.String
	topic.IsActive = isActive == 1
	topic.CentroidEmbedding = BytesToFloat32Slice(centroidBytes)

	// Parse keywords
	if keywordsJSON.Valid && keywordsJSON.String != "" {
		json.Unmarshal([]byte(keywordsJSON.String), &topic.Keywords)
	}

	// Parse timestamps
	if t, err := time.Parse(time.RFC3339, lastActiveAt); err == nil {
		topic.LastActiveAt = t
	} else if t, err := time.Parse("2006-01-02 15:04:05", lastActiveAt); err == nil {
		topic.LastActiveAt = t
	}

	if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
		topic.CreatedAt = t
	} else if t, err := time.Parse("2006-01-02 15:04:05", createdAt); err == nil {
		topic.CreatedAt = t
	}

	return &topic, nil
}

// truncateString truncates a string to maxLen with ellipsis.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
