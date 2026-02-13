package router

import (
	"testing"
	"time"

	"github.com/normanking/cortex/internal/cognitive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"lowercase", "hello world", "hello world"},
		{"uppercase", "HELLO WORLD", "hello world"},
		{"mixed case", "Hello World", "hello world"},
		{"with whitespace", "  hello world  ", "hello world"},
		{"mixed case with whitespace", "  Hello World  ", "hello world"},
		{"empty", "", ""},
		{"only whitespace", "   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeKey(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEmbeddingCache_BasicOperations(t *testing.T) {
	cache := newEmbeddingCache(10, time.Hour)

	// Test initial state
	stats := cache.stats()
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(0), stats.Misses)
	assert.Equal(t, 0, stats.Size)

	// Test cache miss
	result := cache.get("hello")
	assert.Nil(t, result)
	stats = cache.stats()
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(1), stats.Misses)

	// Test cache put and hit
	embedding := cognitive.Embedding{0.1, 0.2, 0.3}
	cache.put("hello", embedding)

	result = cache.get("hello")
	require.NotNil(t, result)
	assert.Equal(t, embedding, result)
	stats = cache.stats()
	assert.Equal(t, int64(1), stats.Hits)
	assert.Equal(t, int64(1), stats.Misses)
	assert.Equal(t, 1, stats.Size)
}

func TestEmbeddingCache_KeyNormalization(t *testing.T) {
	cache := newEmbeddingCache(10, time.Hour)

	embedding := cognitive.Embedding{0.1, 0.2, 0.3}

	// Store with one variant
	cache.put("hello world", embedding)

	// Retrieve with different case
	result := cache.get("HELLO WORLD")
	require.NotNil(t, result)
	assert.Equal(t, embedding, result)

	// Retrieve with whitespace
	result = cache.get("  Hello World  ")
	require.NotNil(t, result)
	assert.Equal(t, embedding, result)

	// Verify only one entry
	stats := cache.stats()
	assert.Equal(t, 1, stats.Size)
}

func TestEmbeddingCache_TTLExpiration(t *testing.T) {
	// Create cache with very short TTL
	cache := newEmbeddingCache(10, 50*time.Millisecond)

	embedding := cognitive.Embedding{0.1, 0.2, 0.3}
	cache.put("hello", embedding)

	// Immediate retrieval should work
	result := cache.get("hello")
	require.NotNil(t, result)
	assert.Equal(t, embedding, result)

	// Wait for TTL to expire
	time.Sleep(60 * time.Millisecond)

	// Should be expired now
	result = cache.get("hello")
	assert.Nil(t, result)

	// Entry should be removed
	stats := cache.stats()
	assert.Equal(t, 0, stats.Size)
}

func TestEmbeddingCache_LRUEviction(t *testing.T) {
	// Small cache to test eviction
	cache := newEmbeddingCache(3, time.Hour)

	// Add 3 entries
	cache.put("a", cognitive.Embedding{1})
	cache.put("b", cognitive.Embedding{2})
	cache.put("c", cognitive.Embedding{3})

	stats := cache.stats()
	assert.Equal(t, 3, stats.Size)

	// Access "a" to make it most recently used
	cache.get("a")

	// Add a 4th entry - should evict "b" (oldest accessed)
	cache.put("d", cognitive.Embedding{4})

	stats = cache.stats()
	assert.Equal(t, 3, stats.Size)

	// "b" should be evicted
	result := cache.get("b")
	assert.Nil(t, result)

	// "a", "c", "d" should still exist
	assert.NotNil(t, cache.get("a"))
	assert.NotNil(t, cache.get("c"))
	assert.NotNil(t, cache.get("d"))
}

func TestEmbeddingCache_UpdateExisting(t *testing.T) {
	cache := newEmbeddingCache(10, time.Hour)

	// Add initial entry
	cache.put("hello", cognitive.Embedding{0.1})

	// Update with new embedding
	newEmbedding := cognitive.Embedding{0.9}
	cache.put("hello", newEmbedding)

	// Should still have only one entry
	stats := cache.stats()
	assert.Equal(t, 1, stats.Size)

	// Should return new embedding
	result := cache.get("hello")
	require.NotNil(t, result)
	assert.Equal(t, newEmbedding, result)
}

func TestEmbeddingCache_HitRate(t *testing.T) {
	cache := newEmbeddingCache(10, time.Hour)

	// Add an entry
	cache.put("hello", cognitive.Embedding{0.1})

	// 3 misses
	cache.get("miss1")
	cache.get("miss2")
	cache.get("miss3")

	// 1 hit
	cache.get("hello")

	stats := cache.stats()
	assert.Equal(t, int64(1), stats.Hits)
	assert.Equal(t, int64(3), stats.Misses)
	assert.InDelta(t, 25.0, stats.HitRate, 0.01) // 1/4 = 25%
}

func TestEmbeddingCache_ConcurrentAccess(t *testing.T) {
	cache := newEmbeddingCache(1000, time.Hour)
	done := make(chan bool)

	// Concurrent writers
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := string(rune('a'+id)) + string(rune('0'+j%10))
				cache.put(key, cognitive.Embedding{float32(id), float32(j)})
			}
			done <- true
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := string(rune('a'+id)) + string(rune('0'+j%10))
				cache.get(key)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}

	// Should not panic and stats should be consistent
	stats := cache.stats()
	assert.True(t, stats.Hits+stats.Misses > 0)
	assert.True(t, stats.Size <= 1000)
}

func TestOllamaEmbedder_CacheEnabled(t *testing.T) {
	// Test default config - cache should be enabled
	cfg := &OllamaEmbedderConfig{}
	embedder := NewOllamaEmbedder(cfg)
	assert.True(t, embedder.CacheEnabled())
	assert.NotNil(t, embedder.cache)

	// Test explicitly enabled
	cfg = &OllamaEmbedderConfig{
		CacheEnabled: true,
		CacheMaxSize: 500,
		CacheTTL:     30 * time.Minute,
	}
	embedder = NewOllamaEmbedder(cfg)
	assert.True(t, embedder.CacheEnabled())
	assert.NotNil(t, embedder.cache)
	stats := embedder.GetCacheStats()
	assert.Equal(t, 500, stats.MaxSize)

	// Test disabled with negative max size
	cfg = &OllamaEmbedderConfig{
		CacheMaxSize: -1,
	}
	embedder = NewOllamaEmbedder(cfg)
	assert.False(t, embedder.CacheEnabled())
	assert.Nil(t, embedder.cache)
	stats = embedder.GetCacheStats()
	assert.Equal(t, CacheStats{}, stats)
}

func TestOllamaEmbedder_GetCacheStats(t *testing.T) {
	cfg := &OllamaEmbedderConfig{
		CacheEnabled: true,
		CacheMaxSize: 100,
		CacheTTL:     time.Hour,
	}
	embedder := NewOllamaEmbedder(cfg)

	stats := embedder.GetCacheStats()
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(0), stats.Misses)
	assert.Equal(t, 0, stats.Size)
	assert.Equal(t, 100, stats.MaxSize)
	assert.Equal(t, 0.0, stats.HitRate)
}
