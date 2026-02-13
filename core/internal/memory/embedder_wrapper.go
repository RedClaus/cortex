// Package memory provides enhanced memory capabilities for Cortex.
// This file implements embedding caching to avoid redundant embedding generation.
package memory

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"time"

	"github.com/rs/zerolog/log"
)

// EmbeddingCache wraps an Embedder with SQLite-backed caching.
// It uses the content_embedding_cache table (migration 011) to store embeddings
// by content hash, avoiding redundant embedding API calls.
//
// Brain Alignment: Like the brain's priming system - recently accessed patterns
// are readily available without full recomputation.
type EmbeddingCache struct {
	embedder Embedder
	db       *sql.DB
	modelID  string

	// Stats for monitoring
	cacheHits   int64
	cacheMisses int64
}

// NewEmbeddingCache creates a new embedding cache wrapping the given embedder.
// The cache uses the content_embedding_cache table in the provided database.
func NewEmbeddingCache(embedder Embedder, db *sql.DB) *EmbeddingCache {
	modelID := "unknown"
	if embedder != nil {
		modelID = embedder.ModelName()
	}

	cache := &EmbeddingCache{
		embedder: embedder,
		db:       db,
		modelID:  modelID,
	}

	log.Info().
		Str("model", modelID).
		Msg("embedding cache initialized")

	return cache
}

// Embed generates an embedding for the given text, checking the cache first.
// If the embedding is cached, it's returned immediately. Otherwise, it's generated
// and cached asynchronously for future use.
func (c *EmbeddingCache) Embed(ctx context.Context, text string) ([]float32, error) {
	if c.embedder == nil {
		return nil, nil
	}

	// Generate content hash for cache lookup
	hash := c.hashContent(text)

	// Try cache first
	if cached, found := c.getFromCache(ctx, hash); found {
		c.cacheHits++
		log.Debug().
			Str("hash", hash[:8]).
			Msg("embedding cache hit")
		return cached, nil
	}

	c.cacheMisses++

	// Generate embedding
	embedding, err := c.embedder.Embed(ctx, text)
	if err != nil {
		return nil, err
	}

	// Cache asynchronously to not block the caller
	go c.cacheAsync(hash, embedding)

	log.Debug().
		Str("hash", hash[:8]).
		Int("dim", len(embedding)).
		Msg("embedding generated and caching")

	return embedding, nil
}

// EmbedBatch generates embeddings for multiple texts efficiently.
// Cached embeddings are used where available, and only missing ones are generated.
func (c *EmbeddingCache) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if c.embedder == nil || len(texts) == 0 {
		return nil, nil
	}

	results := make([][]float32, len(texts))
	toGenerate := make([]int, 0)      // Indices of texts that need generation
	toGenerateTexts := make([]string, 0)

	// Check cache for each text
	for i, text := range texts {
		hash := c.hashContent(text)
		if cached, found := c.getFromCache(ctx, hash); found {
			results[i] = cached
			c.cacheHits++
		} else {
			toGenerate = append(toGenerate, i)
			toGenerateTexts = append(toGenerateTexts, text)
			c.cacheMisses++
		}
	}

	// Generate missing embeddings
	if len(toGenerateTexts) > 0 {
		generated, err := c.embedder.EmbedBatch(ctx, toGenerateTexts)
		if err != nil {
			return nil, err
		}

		// Fill in results and cache
		for j, idx := range toGenerate {
			if j < len(generated) {
				results[idx] = generated[j]
				// Cache asynchronously
				hash := c.hashContent(texts[idx])
				go c.cacheAsync(hash, generated[j])
			}
		}
	}

	log.Debug().
		Int("total", len(texts)).
		Int("cached", len(texts)-len(toGenerate)).
		Int("generated", len(toGenerate)).
		Msg("batch embedding completed")

	return results, nil
}

// Dimension returns the embedding dimension from the underlying embedder.
func (c *EmbeddingCache) Dimension() int {
	if c.embedder == nil {
		return 0
	}
	return c.embedder.Dimension()
}

// ModelName returns the model name from the underlying embedder.
func (c *EmbeddingCache) ModelName() string {
	return c.modelID
}

// Stats returns cache statistics.
func (c *EmbeddingCache) Stats() (hits, misses int64) {
	return c.cacheHits, c.cacheMisses
}

// HitRate returns the cache hit rate as a percentage.
func (c *EmbeddingCache) HitRate() float64 {
	total := c.cacheHits + c.cacheMisses
	if total == 0 {
		return 0
	}
	return float64(c.cacheHits) / float64(total) * 100
}

// hashContent generates a SHA256 hash of the content for cache lookup.
func (c *EmbeddingCache) hashContent(text string) string {
	hash := sha256.Sum256([]byte(text))
	return hex.EncodeToString(hash[:])
}

// getFromCache attempts to retrieve an embedding from the cache.
func (c *EmbeddingCache) getFromCache(ctx context.Context, hash string) ([]float32, bool) {
	if c.db == nil {
		return nil, false
	}

	var embeddingBlob []byte
	var dimension int

	err := c.db.QueryRowContext(ctx, `
		SELECT embedding, dimension
		FROM content_embedding_cache
		WHERE content_hash = ?
	`, hash).Scan(&embeddingBlob, &dimension)

	if err != nil {
		if err != sql.ErrNoRows {
			log.Debug().Err(err).Str("hash", hash[:8]).Msg("cache lookup error")
		}
		return nil, false
	}

	// Convert bytes to float32 slice
	embedding := BytesToFloat32Slice(embeddingBlob)
	if embedding == nil || len(embedding) != dimension {
		log.Warn().
			Str("hash", hash[:8]).
			Int("expected_dim", dimension).
			Int("actual_dim", len(embedding)).
			Msg("embedding dimension mismatch, treating as cache miss")
		return nil, false
	}

	// Update last_used_at and use_count asynchronously
	go c.updateCacheStats(hash)

	return embedding, true
}

// cacheAsync stores an embedding in the cache asynchronously.
func (c *EmbeddingCache) cacheAsync(hash string, embedding []float32) {
	if c.db == nil || embedding == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	embeddingBlob := Float32SliceToBytes(embedding)

	_, err := c.db.ExecContext(ctx, `
		INSERT INTO content_embedding_cache (content_hash, embedding, dimension, model_id, created_at, last_used_at, use_count)
		VALUES (?, ?, ?, ?, datetime('now'), datetime('now'), 1)
		ON CONFLICT(content_hash) DO UPDATE SET
			last_used_at = datetime('now'),
			use_count = use_count + 1
	`, hash, embeddingBlob, len(embedding), c.modelID)

	if err != nil {
		log.Debug().Err(err).Str("hash", hash[:8]).Msg("failed to cache embedding")
	}
}

// updateCacheStats updates last_used_at and use_count for a cache entry.
func (c *EmbeddingCache) updateCacheStats(hash string) {
	if c.db == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := c.db.ExecContext(ctx, `
		UPDATE content_embedding_cache
		SET last_used_at = datetime('now'), use_count = use_count + 1
		WHERE content_hash = ?
	`, hash)

	if err != nil {
		log.Debug().Err(err).Str("hash", hash[:8]).Msg("failed to update cache stats")
	}
}

// CacheCount returns the number of cached embeddings.
func (c *EmbeddingCache) CacheCount(ctx context.Context) (int, error) {
	if c.db == nil {
		return 0, nil
	}

	var count int
	err := c.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM content_embedding_cache
	`).Scan(&count)

	return count, err
}

// EvictStale removes cache entries not used in the given number of days.
// Returns the number of entries evicted.
func (c *EmbeddingCache) EvictStale(ctx context.Context, staleDays int) (int64, error) {
	if c.db == nil {
		return 0, nil
	}

	result, err := c.db.ExecContext(ctx, `
		DELETE FROM content_embedding_cache
		WHERE last_used_at < datetime('now', '-' || ? || ' days')
	`, staleDays)

	if err != nil {
		return 0, err
	}

	count, _ := result.RowsAffected()

	if count > 0 {
		log.Info().
			Int64("evicted", count).
			Int("stale_days", staleDays).
			Msg("evicted stale embedding cache entries")
	}

	return count, nil
}

// Precompute generates and caches embeddings for a list of texts.
// Useful for batch preprocessing known content.
func (c *EmbeddingCache) Precompute(ctx context.Context, texts []string) error {
	if len(texts) == 0 {
		return nil
	}

	// Use batch embedding which handles caching
	_, err := c.EmbedBatch(ctx, texts)
	return err
}
