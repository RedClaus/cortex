package voice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// CommonPhrases contains frequently used terminal phrases for cache prewarming.
var CommonPhrases = []string{
	"Command executed successfully",
	"Error occurred",
	"Processing request",
	"Task completed",
	"File not found",
	"Permission denied",
	"Compilation failed",
	"Tests passed",
	"Build complete",
	"Deployment started",
	"Server started on port",
	"Connection established",
	"Authentication successful",
	"Request timed out",
	"Invalid input",
	"Operation cancelled",
	"Backup created",
	"Configuration updated",
	"Service restarted",
	"Database connected",
}

// AudioFormat represents the audio encoding format.
type AudioFormat string

const (
	FormatWAV  AudioFormat = "wav"
	FormatMP3  AudioFormat = "mp3"
	FormatOGG  AudioFormat = "ogg"
	FormatPCM  AudioFormat = "pcm"
	FormatOpus AudioFormat = "opus"
)

// IsValid checks if an audio format is valid.
func (f AudioFormat) IsValid() bool {
	switch f {
	case FormatWAV, FormatMP3, FormatOGG, FormatPCM, FormatOpus:
		return true
	}
	return false
}

// cacheEntry represents a single cached audio item.
type cacheEntry struct {
	audio     []byte
	format    AudioFormat
	createdAt time.Time
	lastUsed  time.Time
	useCount  int
}

// AudioCache provides LRU caching for synthesized audio.
type AudioCache struct {
	mu           sync.RWMutex
	entries      map[string]*cacheEntry
	accessOrder  []string // LRU tracking
	maxEntries   int
	maxEntrySize int
	ttl          time.Duration

	// Statistics
	hits   int64
	misses int64
}

// CacheStats contains cache performance metrics.
type CacheStats struct {
	Entries    int     `json:"entries"`
	TotalSize  int64   `json:"total_size"`
	Hits       int64   `json:"hits"`
	Misses     int64   `json:"misses"`
	HitRate    float64 `json:"hit_rate"`
	OldestItem string  `json:"oldest_item,omitempty"`
	NewestItem string  `json:"newest_item,omitempty"`
}

// NewAudioCache creates a new audio cache with default settings.
func NewAudioCache() *AudioCache {
	return NewAudioCacheWithConfig(1000, 512*1024, 24*time.Hour)
}

// NewAudioCacheWithConfig creates a new audio cache with custom settings.
func NewAudioCacheWithConfig(maxEntries, maxEntrySize int, ttl time.Duration) *AudioCache {
	return &AudioCache{
		entries:      make(map[string]*cacheEntry),
		accessOrder:  make([]string, 0, maxEntries),
		maxEntries:   maxEntries,
		maxEntrySize: maxEntrySize,
		ttl:          ttl,
	}
}

// CacheKey generates a SHA256 hash key from text, voiceID, and speed.
func CacheKey(text, voiceID string, speed float64) string {
	data := fmt.Sprintf("%s|%s|%.2f", text, voiceID, speed)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// Get retrieves audio from cache if available and not expired.
func (c *AudioCache) Get(key string) (audio []byte, format AudioFormat, found bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.entries[key]
	if !exists {
		c.misses++
		return nil, "", false
	}

	// Check if expired
	if time.Since(entry.createdAt) > c.ttl {
		// Remove expired entry
		delete(c.entries, key)
		c.removeFromAccessOrder(key)
		c.misses++
		log.Debug().Str("key", key[:16]).Msg("cache entry expired")
		return nil, "", false
	}

	// Update access tracking
	entry.lastUsed = time.Now()
	entry.useCount++
	c.updateAccessOrder(key)
	c.hits++

	log.Debug().
		Str("key", key[:16]).
		Int("use_count", entry.useCount).
		Msg("cache hit")

	return entry.audio, entry.format, true
}

// Set stores audio in the cache with LRU eviction if needed.
func (c *AudioCache) Set(key string, audio []byte, format AudioFormat) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check entry size limit
	if len(audio) > c.maxEntrySize {
		log.Warn().
			Int("size", len(audio)).
			Int("max_size", c.maxEntrySize).
			Msg("audio too large for cache")
		return
	}

	// Evict if at capacity
	if len(c.entries) >= c.maxEntries {
		c.evictLRU()
	}

	// Store new entry
	c.entries[key] = &cacheEntry{
		audio:     audio,
		format:    format,
		createdAt: time.Now(),
		lastUsed:  time.Now(),
		useCount:  0,
	}

	c.accessOrder = append(c.accessOrder, key)

	log.Debug().
		Str("key", key[:16]).
		Int("size", len(audio)).
		Str("format", string(format)).
		Msg("cached audio")
}

// evictLRU removes the least recently used entry.
// Must be called with lock held.
func (c *AudioCache) evictLRU() {
	if len(c.accessOrder) == 0 {
		return
	}

	// Remove oldest entry
	oldestKey := c.accessOrder[0]
	delete(c.entries, oldestKey)
	c.accessOrder = c.accessOrder[1:]

	log.Debug().
		Str("key", oldestKey[:16]).
		Msg("evicted LRU cache entry")
}

// removeFromAccessOrder removes a key from the access order list.
// Must be called with lock held.
func (c *AudioCache) removeFromAccessOrder(key string) {
	for i, k := range c.accessOrder {
		if k == key {
			c.accessOrder = append(c.accessOrder[:i], c.accessOrder[i+1:]...)
			return
		}
	}
}

// updateAccessOrder moves a key to the end (most recently used).
// Must be called with lock held.
func (c *AudioCache) updateAccessOrder(key string) {
	c.removeFromAccessOrder(key)
	c.accessOrder = append(c.accessOrder, key)
}

// Stats returns current cache statistics.
func (c *AudioCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var totalSize int64
	for _, entry := range c.entries {
		totalSize += int64(len(entry.audio))
	}

	var hitRate float64
	totalRequests := c.hits + c.misses
	if totalRequests > 0 {
		hitRate = float64(c.hits) / float64(totalRequests)
	}

	stats := CacheStats{
		Entries:   len(c.entries),
		TotalSize: totalSize,
		Hits:      c.hits,
		Misses:    c.misses,
		HitRate:   hitRate,
	}

	if len(c.accessOrder) > 0 {
		stats.OldestItem = c.accessOrder[0][:16]
		stats.NewestItem = c.accessOrder[len(c.accessOrder)-1][:16]
	}

	return stats
}

// Clear removes all entries from the cache.
func (c *AudioCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*cacheEntry)
	c.accessOrder = make([]string, 0, c.maxEntries)

	log.Info().Msg("cache cleared")
}

// CleanExpired removes all expired entries from the cache.
func (c *AudioCache) CleanExpired() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	expiredKeys := make([]string, 0)

	for key, entry := range c.entries {
		if now.Sub(entry.createdAt) > c.ttl {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, key := range expiredKeys {
		delete(c.entries, key)
		c.removeFromAccessOrder(key)
	}

	if len(expiredKeys) > 0 {
		log.Info().
			Int("count", len(expiredKeys)).
			Msg("cleaned expired cache entries")
	}

	return len(expiredKeys)
}

// VoiceProvider is an interface for TTS providers (used for prewarming).
type VoiceProvider interface {
	Synthesize(ctx context.Context, text, voiceID string, speed float64) ([]byte, AudioFormat, error)
}

// Prewarm preloads common phrases into the cache using the given provider.
func (c *AudioCache) Prewarm(ctx context.Context, provider VoiceProvider, voiceID string, phrases []string) error {
	if provider == nil {
		return fmt.Errorf("provider is nil")
	}

	if len(phrases) == 0 {
		phrases = CommonPhrases
	}

	log.Info().
		Int("phrases", len(phrases)).
		Str("voice_id", voiceID).
		Msg("prewarming audio cache")

	successCount := 0
	errorCount := 0

	for _, phrase := range phrases {
		select {
		case <-ctx.Done():
			log.Warn().
				Err(ctx.Err()).
				Int("completed", successCount).
				Msg("cache prewarm cancelled")
			return ctx.Err()
		default:
		}

		// Generate cache key
		key := CacheKey(phrase, voiceID, 1.0)

		// Skip if already cached
		if _, _, found := c.Get(key); found {
			continue
		}

		// Synthesize audio
		audio, format, err := provider.Synthesize(ctx, phrase, voiceID, 1.0)
		if err != nil {
			log.Warn().
				Err(err).
				Str("phrase", phrase[:min(30, len(phrase))]).
				Msg("failed to prewarm phrase")
			errorCount++
			continue
		}

		// Cache the result
		c.Set(key, audio, format)
		successCount++
	}

	log.Info().
		Int("success", successCount).
		Int("errors", errorCount).
		Msg("cache prewarm complete")

	return nil
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
