// Package router provides semantic routing capabilities for the cognitive architecture.
// It uses embedding-based similarity search to match user requests to templates.
package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/normanking/cortex/internal/cognitive"
	"github.com/normanking/cortex/internal/logging"
)

// ═══════════════════════════════════════════════════════════════════════════════
// EMBEDDER INTERFACE
// ═══════════════════════════════════════════════════════════════════════════════

// Embedder provides text embedding capabilities.
type Embedder interface {
	// Embed generates an embedding for a single text.
	Embed(ctx context.Context, text string) (cognitive.Embedding, error)

	// EmbedFast generates an embedding with fast timeout (non-blocking path).
	// Returns ErrEmbeddingTimeout if the embedding takes too long.
	EmbedFast(ctx context.Context, text string) (cognitive.Embedding, error)

	// EmbedBatch generates embeddings for multiple texts.
	// More efficient than calling Embed multiple times.
	EmbedBatch(ctx context.Context, texts []string) ([]cognitive.Embedding, error)

	// Dimension returns the embedding dimension (e.g., 768 for nomic-embed-text).
	Dimension() int

	// ModelName returns the name of the embedding model.
	ModelName() string

	// Available returns true if the embedder is ready to use.
	Available() bool

	// FastTimeout returns the configured fast path timeout.
	FastTimeout() time.Duration
}

// ═══════════════════════════════════════════════════════════════════════════════
// OLLAMA EMBEDDER
// ═══════════════════════════════════════════════════════════════════════════════

// DefaultEmbeddingModel is the default model for embeddings.
const DefaultEmbeddingModel = "nomic-embed-text"

// DefaultOllamaHost is the default Ollama API endpoint.
const DefaultOllamaHost = "http://127.0.0.1:11434"

// ErrEmbeddingTimeout is returned when embedding times out on the fast path.
var ErrEmbeddingTimeout = fmt.Errorf("embedding timeout")

// OllamaEmbedder generates embeddings using Ollama's local models.
type OllamaEmbedder struct {
	host      string
	model     string
	dimension int
	client    *http.Client
	log       *logging.Logger

	// Configuration
	timeout     time.Duration
	fastTimeout time.Duration
	maxRetries  int
	retryDelay  time.Duration

	// State
	available     bool
	availableMu   sync.RWMutex
	lastCheck     time.Time
	checkInterval time.Duration

	// Embedding cache
	cache        *embeddingCache
	cacheEnabled bool
}

// OllamaEmbedderConfig configures the Ollama embedder.
type OllamaEmbedderConfig struct {
	Host          string        // Ollama API host (default: http://127.0.0.1:11434)
	Model         string        // Embedding model (default: nomic-embed-text)
	AutoPull      bool          // Auto-pull model if missing
	CheckInterval time.Duration // How often to check availability
	Timeout       time.Duration // HTTP request timeout (default: 30s for retries)
	FastTimeout   time.Duration // Fast path timeout for non-blocking requests (default: 5s)
	MaxRetries    int           // Number of retries on failure (default: 1)
	RetryDelay    time.Duration // Delay between retries (default: 2s)

	// Cache configuration
	CacheEnabled bool          // Enable embedding cache (default: true)
	CacheMaxSize int           // Maximum cache entries (default: 1000)
	CacheTTL     time.Duration // Cache entry TTL (default: 1 hour)
}

// ═══════════════════════════════════════════════════════════════════════════════
// EMBEDDING CACHE
// ═══════════════════════════════════════════════════════════════════════════════

// Default cache configuration values.
const (
	DefaultCacheMaxSize = 1000
	DefaultCacheTTL     = 1 * time.Hour
)

// cacheEntry stores a cached embedding with its timestamp.
type cacheEntry struct {
	embedding cognitive.Embedding
	timestamp time.Time
	key       string
}

// embeddingCache implements an LRU cache for embeddings with TTL support.
type embeddingCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	order   []*cacheEntry // LRU order: oldest at front, newest at back
	maxSize int
	ttl     time.Duration
	hits    int64
	misses  int64
}

// CacheStats contains embedding cache statistics.
type CacheStats struct {
	Hits    int64   `json:"hits"`
	Misses  int64   `json:"misses"`
	Size    int     `json:"size"`
	MaxSize int     `json:"max_size"`
	HitRate float64 `json:"hit_rate"`
}

// newEmbeddingCache creates a new embedding cache.
func newEmbeddingCache(maxSize int, ttl time.Duration) *embeddingCache {
	return &embeddingCache{
		entries: make(map[string]*cacheEntry),
		order:   make([]*cacheEntry, 0, maxSize),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// normalizeKey creates a normalized cache key from text.
func normalizeKey(text string) string {
	return strings.ToLower(strings.TrimSpace(text))
}

// get retrieves an embedding from the cache. Returns nil if not found or expired.
func (c *embeddingCache) get(text string) cognitive.Embedding {
	key := normalizeKey(text)

	c.mu.RLock()
	entry, exists := c.entries[key]
	c.mu.RUnlock()

	if !exists {
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
		return nil
	}

	// Check TTL
	if time.Since(entry.timestamp) > c.ttl {
		c.mu.Lock()
		c.misses++
		// Remove expired entry
		c.removeEntryLocked(key)
		c.mu.Unlock()
		return nil
	}

	c.mu.Lock()
	c.hits++
	// Move to back (most recently used)
	c.moveToBackLocked(entry)
	c.mu.Unlock()

	return entry.embedding
}

// put stores an embedding in the cache.
func (c *embeddingCache) put(text string, embedding cognitive.Embedding) {
	key := normalizeKey(text)

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if key already exists
	if existing, exists := c.entries[key]; exists {
		// Update existing entry
		existing.embedding = embedding
		existing.timestamp = time.Now()
		c.moveToBackLocked(existing)
		return
	}

	// Evict oldest entries if at capacity
	for len(c.entries) >= c.maxSize && len(c.order) > 0 {
		oldest := c.order[0]
		c.removeEntryLocked(oldest.key)
	}

	// Add new entry
	entry := &cacheEntry{
		embedding: embedding,
		timestamp: time.Now(),
		key:       key,
	}
	c.entries[key] = entry
	c.order = append(c.order, entry)
}

// removeEntryLocked removes an entry from the cache. Must be called with lock held.
func (c *embeddingCache) removeEntryLocked(key string) {
	entry, exists := c.entries[key]
	if !exists {
		return
	}

	delete(c.entries, key)

	// Remove from order slice
	for i, e := range c.order {
		if e == entry {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
}

// moveToBackLocked moves an entry to the back of the LRU list. Must be called with lock held.
func (c *embeddingCache) moveToBackLocked(entry *cacheEntry) {
	// Find and remove from current position
	for i, e := range c.order {
		if e == entry {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
	// Add to back
	c.order = append(c.order, entry)
}

// stats returns current cache statistics.
func (c *embeddingCache) stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(c.hits) / float64(total) * 100.0
	}

	return CacheStats{
		Hits:    c.hits,
		Misses:  c.misses,
		Size:    len(c.entries),
		MaxSize: c.maxSize,
		HitRate: hitRate,
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// OLLAMA EMBEDDER IMPLEMENTATION
// ═══════════════════════════════════════════════════════════════════════════════

// NewOllamaEmbedder creates a new Ollama-based embedder.
func NewOllamaEmbedder(cfg *OllamaEmbedderConfig) *OllamaEmbedder {
	if cfg == nil {
		cfg = &OllamaEmbedderConfig{}
	}

	host := cfg.Host
	if host == "" {
		host = DefaultOllamaHost
	}

	model := cfg.Model
	if model == "" {
		model = DefaultEmbeddingModel
	}

	checkInterval := cfg.CheckInterval
	if checkInterval == 0 {
		checkInterval = 5 * time.Minute
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second // Retry path timeout
	}

	fastTimeout := cfg.FastTimeout
	if fastTimeout == 0 {
		fastTimeout = 5 * time.Second // Fast path timeout (non-blocking)
	}

	maxRetries := cfg.MaxRetries
	if maxRetries == 0 {
		maxRetries = 1 // Default: retry once on failure
	}

	retryDelay := cfg.RetryDelay
	if retryDelay == 0 {
		retryDelay = 2 * time.Second
	}

	// Cache configuration - enabled by default unless explicitly disabled
	cacheEnabled := true
	if cfg.CacheMaxSize < 0 {
		// Negative max size explicitly disables cache
		cacheEnabled = false
	} else if cfg.CacheMaxSize == 0 && cfg.CacheTTL == 0 && !cfg.CacheEnabled {
		// Default case: enable cache with defaults
		cacheEnabled = true
	} else {
		cacheEnabled = cfg.CacheEnabled || cfg.CacheMaxSize > 0 || cfg.CacheTTL > 0
	}

	cacheMaxSize := cfg.CacheMaxSize
	if cacheMaxSize <= 0 {
		cacheMaxSize = DefaultCacheMaxSize
	}

	cacheTTL := cfg.CacheTTL
	if cacheTTL == 0 {
		cacheTTL = DefaultCacheTTL
	}

	var cache *embeddingCache
	if cacheEnabled {
		cache = newEmbeddingCache(cacheMaxSize, cacheTTL)
	}

	e := &OllamaEmbedder{
		host:      host,
		model:     model,
		dimension: cognitive.DefaultEmbeddingDim, // 768 for nomic-embed-text
		client: &http.Client{
			// Don't use http.Client.Timeout for streaming - use Transport.ResponseHeaderTimeout instead.
			// Client.Timeout applies to entire request including body reading.
			Transport: &http.Transport{
				ResponseHeaderTimeout: timeout, // Time to receive response headers (allows for model loading)
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
			},
		},
		log:           logging.Global(),
		timeout:       timeout,
		fastTimeout:   fastTimeout,
		maxRetries:    maxRetries,
		retryDelay:    retryDelay,
		checkInterval: checkInterval,
		cache:         cache,
		cacheEnabled:  cacheEnabled,
	}

	// Check availability on startup
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if e.checkAvailability(ctx) {
		e.available = true
	} else if cfg.AutoPull {
		e.log.Info("[Embedder] Model %s not found, attempting to pull...", model)
		if err := e.pullModel(ctx); err != nil {
			e.log.Warn("[Embedder] Failed to pull model: %v", err)
		} else {
			e.available = true
		}
	}

	if cacheEnabled {
		e.log.Debug("[Embedder] Cache enabled with maxSize=%d, ttl=%v", cacheMaxSize, cacheTTL)
	}

	return e
}

// Embed generates an embedding for a single text.
func (e *OllamaEmbedder) Embed(ctx context.Context, text string) (cognitive.Embedding, error) {
	if !e.Available() {
		return nil, fmt.Errorf("embedder not available")
	}

	embeddings, err := e.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	return embeddings[0], nil
}

// EmbedFast generates an embedding with the fast timeout (non-blocking path).
// Returns ErrEmbeddingTimeout if the embedding takes too long.
func (e *OllamaEmbedder) EmbedFast(ctx context.Context, text string) (cognitive.Embedding, error) {
	if !e.Available() {
		return nil, fmt.Errorf("embedder not available")
	}

	// Check cache first - cache hits are always fast
	if e.cacheEnabled && e.cache != nil {
		if cached := e.cache.get(text); cached != nil {
			e.log.Debug("[Embedder] Cache HIT (fast path) for text (len=%d)", len(text))
			return cached, nil
		}
		e.log.Debug("[Embedder] Cache MISS (fast path) for text (len=%d)", len(text))
	}

	// Create a context with the fast timeout
	fastCtx, cancel := context.WithTimeout(ctx, e.fastTimeout)
	defer cancel()

	// Direct embedding call without retries for fast path
	embedding, err := e.doEmbedRequest(fastCtx, text)
	if err != nil {
		// Check if this was a timeout
		if IsTimeoutError(err) {
			e.log.Debug("[Embedder] Fast path timed out after %v", e.fastTimeout)
			return nil, ErrEmbeddingTimeout
		}
		return nil, err
	}

	// Store in cache on success
	if e.cacheEnabled && e.cache != nil {
		e.cache.put(text, embedding)
	}

	return embedding, nil
}

// FastTimeout returns the configured fast path timeout.
func (e *OllamaEmbedder) FastTimeout() time.Duration {
	return e.fastTimeout
}

// EmbedBatch generates embeddings for multiple texts.
func (e *OllamaEmbedder) EmbedBatch(ctx context.Context, texts []string) ([]cognitive.Embedding, error) {
	if !e.Available() {
		return nil, fmt.Errorf("embedder not available")
	}

	embeddings := make([]cognitive.Embedding, len(texts))

	// Ollama doesn't have native batch embedding, so we process sequentially
	// In the future, this could be parallelized with rate limiting
	for i, text := range texts {
		embedding, err := e.embedSingle(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("embed text %d: %w", i, err)
		}
		embeddings[i] = embedding
	}

	return embeddings, nil
}

// embedSingle generates an embedding for a single text via Ollama API.
// Implements retry logic for transient failures (timeouts, connection issues).
func (e *OllamaEmbedder) embedSingle(ctx context.Context, text string) (cognitive.Embedding, error) {
	// Check cache first
	if e.cacheEnabled && e.cache != nil {
		if cached := e.cache.get(text); cached != nil {
			e.log.Debug("[Embedder] Cache HIT for text (len=%d)", len(text))
			return cached, nil
		}
		e.log.Debug("[Embedder] Cache MISS for text (len=%d)", len(text))
	}

	var lastErr error

	for attempt := 0; attempt <= e.maxRetries; attempt++ {
		if attempt > 0 {
			e.log.Info("[Embedder] Retry attempt %d/%d after error: %v", attempt, e.maxRetries, lastErr)
			// Wait before retry (but respect context cancellation)
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("context cancelled during retry: %w", ctx.Err())
			case <-time.After(e.retryDelay):
				// Continue with retry
			}
		}

		embedding, err := e.doEmbedRequest(ctx, text)
		if err == nil {
			if attempt > 0 {
				e.log.Info("[Embedder] Retry succeeded on attempt %d", attempt+1)
			}

			// Store in cache on success
			if e.cacheEnabled && e.cache != nil {
				e.cache.put(text, embedding)
			}

			return embedding, nil
		}

		lastErr = err

		// Check if error is retryable (timeouts, connection errors)
		if !isRetryableError(err) {
			e.log.Debug("[Embedder] Non-retryable error: %v", err)
			break
		}
	}

	return nil, lastErr
}

// isRetryableError determines if an error should trigger a retry.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Retry on timeout or connection errors
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "EOF")
}

// IsTimeoutError checks if an error is a timeout error.
func IsTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") ||
		strings.Contains(errStr, "context canceled")
}

// doEmbedRequest performs the actual HTTP request to Ollama.
func (e *OllamaEmbedder) doEmbedRequest(ctx context.Context, text string) (cognitive.Embedding, error) {
	// Ollama embedding request
	reqBody := map[string]interface{}{
		"model":  e.model,
		"prompt": text,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.host+"/api/embeddings", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		// Mark as unavailable on connection error (will be rechecked later)
		e.setAvailable(false)
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Embedding []float64 `json:"embedding"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Convert float64 to float32
	embedding := make(cognitive.Embedding, len(result.Embedding))
	for i, v := range result.Embedding {
		embedding[i] = float32(v)
	}

	// Update dimension if different (first time)
	if len(embedding) > 0 && e.dimension != len(embedding) {
		e.dimension = len(embedding)
	}

	return embedding, nil
}

// Dimension returns the embedding dimension.
func (e *OllamaEmbedder) Dimension() int {
	return e.dimension
}

// ModelName returns the name of the embedding model.
func (e *OllamaEmbedder) ModelName() string {
	return e.model
}

// Available returns true if the embedder is ready to use.
func (e *OllamaEmbedder) Available() bool {
	e.availableMu.RLock()
	available := e.available
	lastCheck := e.lastCheck
	e.availableMu.RUnlock()

	// Re-check if enough time has passed
	if !available && time.Since(lastCheck) > e.checkInterval {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if e.checkAvailability(ctx) {
			e.setAvailable(true)
		}
	}

	return available
}

// setAvailable updates the availability state.
func (e *OllamaEmbedder) setAvailable(available bool) {
	e.availableMu.Lock()
	e.available = available
	e.lastCheck = time.Now()
	e.availableMu.Unlock()
}

// checkAvailability checks if Ollama is running and the model is available.
func (e *OllamaEmbedder) checkAvailability(ctx context.Context) bool {
	// Check if Ollama is running
	req, err := http.NewRequestWithContext(ctx, "GET", e.host+"/api/tags", nil)
	if err != nil {
		return false
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	// Parse response to check if model exists
	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}

	// Check if our model is in the list
	for _, m := range result.Models {
		// Handle both "nomic-embed-text" and "nomic-embed-text:latest"
		if m.Name == e.model || strings.HasPrefix(m.Name, e.model+":") {
			e.log.Debug("[Embedder] Model %s is available", e.model)
			return true
		}
	}

	e.log.Debug("[Embedder] Model %s not found in Ollama", e.model)
	return false
}

// pullModel attempts to pull the embedding model using Ollama CLI.
func (e *OllamaEmbedder) pullModel(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "ollama", "pull", e.model)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pull failed: %w (output: %s)", err, string(output))
	}

	e.log.Info("[Embedder] Successfully pulled model %s", e.model)
	return nil
}

// Warmup sends a minimal request to pre-load the embedding model into memory.
// This avoids cold start latency (30-90+ seconds) on the first real embedding request.
func (e *OllamaEmbedder) Warmup(ctx context.Context) error {
	if !e.Available() {
		return fmt.Errorf("embedder not available")
	}

	// Send a minimal embedding request to load the model
	_, err := e.doEmbedRequest(ctx, "warmup")
	if err != nil {
		return fmt.Errorf("warmup failed: %w", err)
	}
	return nil
}

// WarmupAsync starts embedding model warmup in the background.
// It returns immediately and does not block. This is useful for pre-loading
// the embedding model before the user needs it.
func (e *OllamaEmbedder) WarmupAsync(ctx context.Context) {
	go func() {
		if err := e.Warmup(ctx); err != nil {
			// Log but don't fail - warmup is optional optimization
			e.log.Debug("[Embedder] Warmup failed: %v", err)
		} else {
			e.log.Debug("[Embedder] Model %s warmed up successfully", e.model)
		}
	}()
}

// GetCacheStats returns current embedding cache statistics.
// Returns zero stats if caching is disabled.
func (e *OllamaEmbedder) GetCacheStats() CacheStats {
	if !e.cacheEnabled || e.cache == nil {
		return CacheStats{}
	}
	return e.cache.stats()
}

// CacheEnabled returns whether the embedding cache is enabled.
func (e *OllamaEmbedder) CacheEnabled() bool {
	return e.cacheEnabled
}

// ═══════════════════════════════════════════════════════════════════════════════
// NULL EMBEDDER (for testing/fallback)
// ═══════════════════════════════════════════════════════════════════════════════

// NullEmbedder is a no-op embedder for testing or when embeddings are unavailable.
type NullEmbedder struct {
	dimension int
}

// NewNullEmbedder creates a new null embedder.
func NewNullEmbedder() *NullEmbedder {
	return &NullEmbedder{dimension: cognitive.DefaultEmbeddingDim}
}

// Embed returns a zero embedding.
func (e *NullEmbedder) Embed(ctx context.Context, text string) (cognitive.Embedding, error) {
	return make(cognitive.Embedding, e.dimension), nil
}

// EmbedFast returns a zero embedding (fast path for null embedder).
func (e *NullEmbedder) EmbedFast(ctx context.Context, text string) (cognitive.Embedding, error) {
	return make(cognitive.Embedding, e.dimension), nil
}

// EmbedBatch returns zero embeddings.
func (e *NullEmbedder) EmbedBatch(ctx context.Context, texts []string) ([]cognitive.Embedding, error) {
	embeddings := make([]cognitive.Embedding, len(texts))
	for i := range embeddings {
		embeddings[i] = make(cognitive.Embedding, e.dimension)
	}
	return embeddings, nil
}

// Dimension returns the embedding dimension.
func (e *NullEmbedder) Dimension() int {
	return e.dimension
}

// ModelName returns "null".
func (e *NullEmbedder) ModelName() string {
	return "null"
}

// Available always returns false.
func (e *NullEmbedder) Available() bool {
	return false
}

// FastTimeout returns the default fast timeout.
func (e *NullEmbedder) FastTimeout() time.Duration {
	return 5 * time.Second
}
