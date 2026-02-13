// Package memory provides enhanced memory capabilities for Cortex.
// This file implements background jobs for memory maintenance tasks.
package memory

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// Logger interface for job logging.
type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

// JobConfig configures the memory maintenance jobs.
type JobConfig struct {
	// Interval is how often to run maintenance jobs. Default: 24h
	Interval time.Duration `json:"interval"`

	// DecayRate is the daily confidence decay factor (0-1). Default: 0.99
	DecayRate float64 `json:"decay_rate"`

	// StaleTopicDays is days until a topic is considered inactive. Default: 30
	StaleTopicDays int `json:"stale_topic_days"`

	// AutoLinkBatchSize is max memories to auto-link per run. Default: 50
	AutoLinkBatchSize int `json:"auto_link_batch_size"`

	// ClusterConfig contains DBSCAN clustering parameters.
	ClusterConfig ClusterConfig `json:"cluster_config"`

	// NeighborhoodBatchSize is max neighborhoods to refresh per run. Default: 100
	NeighborhoodBatchSize int `json:"neighborhood_batch_size"`

	// RebuildVectorIndex triggers full index rebuild if true. Default: false
	RebuildVectorIndex bool `json:"rebuild_vector_index"`
}

// DefaultJobConfig returns sensible defaults for maintenance jobs.
func DefaultJobConfig() JobConfig {
	return JobConfig{
		Interval:              24 * time.Hour,
		DecayRate:             DefaultDecayRate,
		StaleTopicDays:        StaleTopicDays,
		AutoLinkBatchSize:     50,
		ClusterConfig:         DefaultClusterConfig(),
		NeighborhoodBatchSize: 100,
		RebuildVectorIndex:    false,
	}
}

// MemoryJobs manages scheduled maintenance tasks for the memory system.
type MemoryJobs struct {
	db                *sql.DB
	topicStore        *TopicStore
	strategicStore    *StrategicMemoryStore
	linkStore         *LinkStore
	neighborhoodStore *NeighborhoodStore
	episodeStore      *EpisodeStore
	vectorIndex       *VectorIndex
	embedder          Embedder
	config            JobConfig
	stopCh            chan struct{}
	logger            Logger
	once              sync.Once
	running           bool
	mu                sync.Mutex
}

// NewMemoryJobs creates a new MemoryJobs instance.
func NewMemoryJobs(
	db *sql.DB,
	topicStore *TopicStore,
	strategicStore *StrategicMemoryStore,
	linkStore *LinkStore,
	embedder Embedder,
	config JobConfig,
	logger Logger,
) *MemoryJobs {
	return &MemoryJobs{
		db:             db,
		topicStore:     topicStore,
		strategicStore: strategicStore,
		linkStore:      linkStore,
		embedder:       embedder,
		config:         config,
		stopCh:         make(chan struct{}, 1),
		logger:         logger,
	}
}

func (j *MemoryJobs) SetNeighborhoodStore(ns *NeighborhoodStore) {
	j.neighborhoodStore = ns
}

func (j *MemoryJobs) SetEpisodeStore(es *EpisodeStore) {
	j.episodeStore = es
}

func (j *MemoryJobs) SetVectorIndex(vi *VectorIndex) {
	j.vectorIndex = vi
}

// Start launches the background job goroutine.
func (j *MemoryJobs) Start() {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.running {
		return
	}

	j.running = true
	go j.runLoop()
	j.logger.Info("memory jobs started", "interval", j.config.Interval.String())
}

// Stop cleanly shuts down the background jobs.
func (j *MemoryJobs) Stop() {
	j.once.Do(func() {
		j.mu.Lock()
		defer j.mu.Unlock()

		if !j.running {
			return
		}

		close(j.stopCh)
		j.running = false
	})
}

// RunNow executes all maintenance jobs immediately (useful for testing).
func (j *MemoryJobs) RunNow(ctx context.Context) error {
	j.runAllJobs(ctx)
	return nil
}

// runLoop is the main background loop with ticker.
func (j *MemoryJobs) runLoop() {
	// Run immediately on start with 30-second timeout per job cycle
	// This prevents memory jobs from blocking the database indefinitely
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	j.runAllJobs(ctx)
	cancel()

	ticker := time.NewTicker(j.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Use timeout context to prevent long-running jobs from blocking
			jobCtx, jobCancel := context.WithTimeout(context.Background(), 30*time.Second)
			j.runAllJobs(jobCtx)
			jobCancel()
		case <-j.stopCh:
			j.logger.Info("memory jobs stopped")
			return
		}
	}
}

// runAllJobs executes all maintenance tasks.
// Errors are logged but don't stop subsequent jobs.
func (j *MemoryJobs) runAllJobs(ctx context.Context) {
	j.logger.Info("running memory maintenance jobs")

	// Run clustering
	if err := j.runClustering(ctx); err != nil {
		j.logger.Error("clustering job failed", "error", err.Error())
	}

	// Run confidence decay
	if err := j.runDecay(ctx); err != nil {
		j.logger.Error("decay job failed", "error", err.Error())
	}

	// Run stale topic cleanup
	if err := j.runStaleTopicCleanup(ctx); err != nil {
		j.logger.Error("stale topic cleanup job failed", "error", err.Error())
	}

	// Run auto-linking
	if err := j.runAutoLinking(ctx); err != nil {
		j.logger.Error("auto-linking job failed", "error", err.Error())
	}

	// Run neighborhood refresh
	if err := j.runNeighborhoodRefresh(ctx); err != nil {
		j.logger.Error("neighborhood refresh job failed", "error", err.Error())
	}

	// Run vector index maintenance
	if err := j.runVectorIndexMaintenance(ctx); err != nil {
		j.logger.Error("vector index job failed", "error", err.Error())
	}

	j.logger.Info("memory maintenance jobs complete")
}

// runClustering executes the topic clustering job.
func (j *MemoryJobs) runClustering(ctx context.Context) error {
	if j.topicStore == nil {
		return nil
	}

	topics, err := j.topicStore.RunClustering(ctx, j.config.ClusterConfig)
	if err != nil {
		return fmt.Errorf("clustering: %w", err)
	}

	j.logger.Info("clustering complete", "topics_found", len(topics))
	return nil
}

// runDecay applies time-based confidence decay to strategic memories.
func (j *MemoryJobs) runDecay(ctx context.Context) error {
	if j.strategicStore == nil {
		return nil
	}

	// Find strategic memories not applied in last 7 days
	query := `
		SELECT id, confidence, 
		       CAST((julianday('now') - julianday(last_applied_at)) AS INTEGER) as days_since
		FROM strategic_memory
		WHERE last_applied_at IS NOT NULL 
		  AND julianday('now') - julianday(last_applied_at) > 7
	`

	rows, err := j.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("query stale memories: %w", err)
	}
	defer rows.Close()

	var updated int
	for rows.Next() {
		var id string
		var confidence float64
		var daysSince int

		if err := rows.Scan(&id, &confidence, &daysSince); err != nil {
			j.logger.Error("scan decay row failed", "error", err.Error())
			continue
		}

		// Calculate new confidence using helper
		newConfidence := DecayConfidence(confidence, daysSince, j.config.DecayRate)

		// Only update if confidence actually changed
		if newConfidence != confidence {
			if err := j.strategicStore.UpdateConfidence(ctx, id, newConfidence); err != nil {
				j.logger.Error("update confidence failed", "id", id, "error", err.Error())
				continue
			}
			updated++
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate decay rows: %w", err)
	}

	j.logger.Info("decay complete", "memories_updated", updated)
	return nil
}

// runStaleTopicCleanup deactivates topics that haven't been used recently.
func (j *MemoryJobs) runStaleTopicCleanup(ctx context.Context) error {
	if j.topicStore == nil {
		return nil
	}

	count, err := j.topicStore.DeactivateStaleTopics(ctx, j.config.StaleTopicDays)
	if err != nil {
		return fmt.Errorf("deactivate stale topics: %w", err)
	}

	j.logger.Info("stale topic cleanup", "deactivated", count)
	return nil
}

// runAutoLinking automatically creates links for recent unlinked memories.
func (j *MemoryJobs) runAutoLinking(ctx context.Context) error {
	if j.linkStore == nil {
		return nil
	}

	// Find recent memories without links
	query := `
		SELECT id, principle as content, embedding
		FROM strategic_memory
		WHERE id NOT IN (SELECT DISTINCT source_id FROM memory_links)
		  AND embedding IS NOT NULL
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := j.db.QueryContext(ctx, query, j.config.AutoLinkBatchSize)
	if err != nil {
		return fmt.Errorf("query unlinked memories: %w", err)
	}
	defer rows.Close()

	var linked int
	for rows.Next() {
		var id, content string
		var embeddingBytes []byte

		if err := rows.Scan(&id, &content, &embeddingBytes); err != nil {
			j.logger.Error("scan unlinked memory failed", "error", err.Error())
			continue
		}

		embedding := BytesToFloat32Slice(embeddingBytes)
		if embedding == nil {
			continue
		}

		// Create GenericMemory for auto-linking
		memory := GenericMemory{
			ID:        id,
			Type:      MemoryTypeStrategic,
			Content:   content,
			Embedding: embedding,
		}

		// Call auto-link
		links, err := j.linkStore.AutoLinkMemory(ctx, memory)
		if err != nil {
			j.logger.Error("auto-link failed", "id", id, "error", err.Error())
			continue
		}

		linked += len(links)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate unlinked rows: %w", err)
	}

	j.logger.Info("auto-linking complete", "links_created", linked)
	return nil
}

func (j *MemoryJobs) runNeighborhoodRefresh(ctx context.Context) error {
	if j.neighborhoodStore == nil {
		return nil
	}

	refreshed, err := j.neighborhoodStore.RefreshStaleNeighborhoods(ctx, j.config.NeighborhoodBatchSize)
	if err != nil {
		return fmt.Errorf("refresh neighborhoods: %w", err)
	}

	j.logger.Info("neighborhood refresh complete", "refreshed", refreshed)
	return nil
}

func (j *MemoryJobs) runVectorIndexMaintenance(ctx context.Context) error {
	if j.vectorIndex == nil {
		return nil
	}

	if j.config.RebuildVectorIndex {
		if err := j.vectorIndex.RebuildIndex(ctx); err != nil {
			return fmt.Errorf("rebuild vector index: %w", err)
		}
		j.logger.Info("vector index rebuilt")
		return nil
	}

	stats, err := j.vectorIndex.Stats(ctx)
	if err != nil {
		return fmt.Errorf("get vector index stats: %w", err)
	}

	j.logger.Info("vector index stats", "indexed", stats["total_indexed"], "buckets", stats["unique_buckets"])
	return nil
}
