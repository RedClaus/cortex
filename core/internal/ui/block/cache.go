// Package block provides render caching for the Cortex TUI block system.
package block

import (
	"sync"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// RENDER CACHE
// ═══════════════════════════════════════════════════════════════════════════════

// RenderCache provides efficient caching of rendered block content.
// It tracks what blocks need re-rendering and evicts stale cache entries.
type RenderCache struct {
	entries    map[string]*CacheEntry
	maxSize    int
	maxAge     time.Duration
	hits       uint64
	misses     uint64
	evictions  uint64
	mu         sync.RWMutex
}

// CacheEntry represents a single cached render result.
type CacheEntry struct {
	// Content is the rendered string content
	Content string

	// Width is the width used for rendering
	Width int

	// Version tracks the block version when cached
	Version uint64

	// CreatedAt is when this entry was cached
	CreatedAt time.Time

	// LastAccessed tracks last read time for LRU eviction
	LastAccessed time.Time

	// Size is the approximate memory size of this entry
	Size int
}

// NewRenderCache creates a new render cache with the specified max size and age.
func NewRenderCache(maxSize int, maxAge time.Duration) *RenderCache {
	if maxSize <= 0 {
		maxSize = 1000 // Default to 1000 entries
	}
	if maxAge <= 0 {
		maxAge = 5 * time.Minute // Default to 5 minutes
	}

	return &RenderCache{
		entries: make(map[string]*CacheEntry),
		maxSize: maxSize,
		maxAge:  maxAge,
	}
}

// DefaultRenderCache creates a cache with sensible defaults.
func DefaultRenderCache() *RenderCache {
	return NewRenderCache(1000, 5*time.Minute)
}

// ═══════════════════════════════════════════════════════════════════════════════
// CACHE OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════════

// Get retrieves a cached render for a block.
// Returns (content, true) if cached and valid, ("", false) otherwise.
func (c *RenderCache) Get(blockID string, width int, version uint64) (string, bool) {
	c.mu.RLock()
	entry, exists := c.entries[blockID]
	c.mu.RUnlock()

	if !exists {
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
		return "", false
	}

	// Check if entry is valid
	if entry.Width != width || entry.Version != version {
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
		return "", false
	}

	// Check if entry is too old
	if time.Since(entry.CreatedAt) > c.maxAge {
		c.mu.Lock()
		delete(c.entries, blockID)
		c.misses++
		c.mu.Unlock()
		return "", false
	}

	// Update last accessed time
	c.mu.Lock()
	entry.LastAccessed = time.Now()
	c.hits++
	c.mu.Unlock()

	return entry.Content, true
}

// Set stores a rendered block in the cache.
func (c *RenderCache) Set(blockID string, content string, width int, version uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict if at capacity
	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}

	c.entries[blockID] = &CacheEntry{
		Content:      content,
		Width:        width,
		Version:      version,
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
		Size:         len(content),
	}
}

// Invalidate removes a specific block from the cache.
func (c *RenderCache) Invalidate(blockID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, blockID)
}

// InvalidateAll clears the entire cache.
func (c *RenderCache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*CacheEntry)
}

// InvalidateByPrefix removes all entries with IDs starting with the prefix.
// Useful for invalidating all children of a parent block.
func (c *RenderCache) InvalidateByPrefix(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for id := range c.entries {
		if len(id) >= len(prefix) && id[:len(prefix)] == prefix {
			delete(c.entries, id)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// EVICTION
// ═══════════════════════════════════════════════════════════════════════════════

// evictOldest removes the least recently accessed entry.
// Must be called with lock held.
func (c *RenderCache) evictOldest() {
	if len(c.entries) == 0 {
		return
	}

	var oldestID string
	var oldestTime time.Time

	for id, entry := range c.entries {
		if oldestID == "" || entry.LastAccessed.Before(oldestTime) {
			oldestID = id
			oldestTime = entry.LastAccessed
		}
	}

	if oldestID != "" {
		delete(c.entries, oldestID)
		c.evictions++
	}
}

// evictStale removes all entries older than maxAge.
func (c *RenderCache) evictStale() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for id, entry := range c.entries {
		if now.Sub(entry.CreatedAt) > c.maxAge {
			delete(c.entries, id)
			c.evictions++
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// STATISTICS
// ═══════════════════════════════════════════════════════════════════════════════

// CacheStats provides statistics about cache performance.
type CacheStats struct {
	Size      int
	MaxSize   int
	Hits      uint64
	Misses    uint64
	HitRate   float64
	Evictions uint64
	TotalSize int // Approximate memory usage in bytes
}

// Stats returns current cache statistics.
func (c *RenderCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	totalSize := 0
	for _, entry := range c.entries {
		totalSize += entry.Size
	}

	total := c.hits + c.misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(c.hits) / float64(total)
	}

	return CacheStats{
		Size:      len(c.entries),
		MaxSize:   c.maxSize,
		Hits:      c.hits,
		Misses:    c.misses,
		HitRate:   hitRate,
		Evictions: c.evictions,
		TotalSize: totalSize,
	}
}

// ResetStats resets the hit/miss counters.
func (c *RenderCache) ResetStats() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hits = 0
	c.misses = 0
	c.evictions = 0
}

// ═══════════════════════════════════════════════════════════════════════════════
// CACHE-AWARE BLOCK RENDERER
// ═══════════════════════════════════════════════════════════════════════════════

// CachedBlockRenderer wraps BlockRenderer with caching.
type CachedBlockRenderer struct {
	renderer *BlockRenderer
	cache    *RenderCache
}

// NewCachedBlockRenderer creates a renderer with caching.
func NewCachedBlockRenderer(styles BlockStyles, width int) *CachedBlockRenderer {
	return &CachedBlockRenderer{
		renderer: NewBlockRenderer(styles, width),
		cache:    DefaultRenderCache(),
	}
}

// RenderBlock renders a block, using cache when possible.
func (r *CachedBlockRenderer) RenderBlock(b *Block) string {
	if b == nil {
		return ""
	}

	// Don't cache streaming blocks
	if b.State == BlockStateStreaming {
		return r.renderer.RenderBlock(b)
	}

	// Get block version for cache validation
	// Using NeedsRender as a simple version indicator
	version := uint64(0)
	if !b.NeedsRender {
		version = 1
	}

	// Check cache
	if cached, ok := r.cache.Get(b.ID, r.renderer.width, version); ok {
		return cached
	}

	// Render and cache
	rendered := r.renderer.RenderBlock(b)
	r.cache.Set(b.ID, rendered, r.renderer.width, version)

	return rendered
}

// RenderBlockList renders a list of blocks with caching.
func (r *CachedBlockRenderer) RenderBlockList(blocks []*Block) string {
	// Use the underlying renderer's list rendering
	// Individual blocks will be cached via RenderBlock calls
	return r.renderer.RenderBlockList(blocks)
}

// SetWidth updates the width and invalidates cache.
func (r *CachedBlockRenderer) SetWidth(width int) {
	if r.renderer.width != width {
		r.renderer.SetWidth(width)
		r.cache.InvalidateAll() // Width change invalidates all cached renders
	}
}

// InvalidateBlock marks a specific block for re-rendering.
func (r *CachedBlockRenderer) InvalidateBlock(blockID string) {
	r.cache.Invalidate(blockID)
}

// Stats returns cache statistics.
func (r *CachedBlockRenderer) Stats() CacheStats {
	return r.cache.Stats()
}
