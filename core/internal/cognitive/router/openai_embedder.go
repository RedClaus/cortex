// Package router provides semantic routing capabilities for the cognitive architecture.
package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/normanking/cortex/internal/cognitive"
	"github.com/normanking/cortex/internal/logging"
)

// OpenAI embedding models and their dimensions
const (
	OpenAIEmbeddingModelSmall = "text-embedding-3-small" // 1536 dims, cheaper
	OpenAIEmbeddingModelLarge = "text-embedding-3-large" // 3072 dims, better quality
	OpenAIEmbeddingModelAda   = "text-embedding-ada-002" // 1536 dims, legacy

	OpenAISmallDimension = 1536
	OpenAILargeDimension = 3072
)

// OpenAIEmbedder generates embeddings using OpenAI's API.
// This serves as a fallback when local embedders (Ollama/MLX) aren't available.
type OpenAIEmbedder struct {
	apiKey    string
	model     string
	dimension int
	client    *http.Client
	log       *logging.Logger

	// Configuration
	timeout     time.Duration
	fastTimeout time.Duration

	// State
	available       bool
	availableMu     sync.RWMutex
	quotaExceeded   bool          // True when 429 quota error received
	quotaResetTime  time.Time     // When to retry after quota exceeded

	// Embedding cache (reuse from Ollama embedder)
	cache        *embeddingCache
	cacheEnabled bool
}

// OpenAIEmbedderConfig configures the OpenAI embedder.
type OpenAIEmbedderConfig struct {
	APIKey      string        // OpenAI API key (or uses OPENAI_API_KEY env var)
	Model       string        // Embedding model (default: text-embedding-3-small)
	Timeout     time.Duration // HTTP request timeout (default: 30s)
	FastTimeout time.Duration // Fast path timeout (default: 5s)

	// Cache configuration
	CacheEnabled bool          // Enable embedding cache (default: true)
	CacheMaxSize int           // Maximum cache entries (default: 1000)
	CacheTTL     time.Duration // Cache entry TTL (default: 1 hour)
}

// NewOpenAIEmbedder creates a new OpenAI-based embedder.
func NewOpenAIEmbedder(cfg *OpenAIEmbedderConfig) *OpenAIEmbedder {
	if cfg == nil {
		cfg = &OpenAIEmbedderConfig{}
	}

	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	model := cfg.Model
	if model == "" {
		model = OpenAIEmbeddingModelSmall
	}

	// Determine dimension based on model
	dimension := OpenAISmallDimension
	if model == OpenAIEmbeddingModelLarge {
		dimension = OpenAILargeDimension
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	fastTimeout := cfg.FastTimeout
	if fastTimeout == 0 {
		fastTimeout = 5 * time.Second
	}

	// Cache configuration
	cacheEnabled := true
	if cfg.CacheMaxSize < 0 {
		cacheEnabled = false
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

	e := &OpenAIEmbedder{
		apiKey:       apiKey,
		model:        model,
		dimension:    dimension,
		client:       &http.Client{Timeout: timeout},
		log:          logging.Global(),
		timeout:      timeout,
		fastTimeout:  fastTimeout,
		available:    apiKey != "",
		cache:        cache,
		cacheEnabled: cacheEnabled,
	}

	if e.available {
		e.log.Debug("[OpenAI Embedder] Initialized with model: %s (dim=%d)", model, dimension)
	}

	return e
}

// Embed generates an embedding for a single text.
func (e *OpenAIEmbedder) Embed(ctx context.Context, text string) (cognitive.Embedding, error) {
	if !e.Available() {
		return nil, fmt.Errorf("OpenAI embedder not available (no API key)")
	}

	// Check cache first
	if e.cacheEnabled && e.cache != nil {
		if cached := e.cache.get(text); cached != nil {
			e.log.Debug("[OpenAI Embedder] Cache HIT for text (len=%d)", len(text))
			return cached, nil
		}
	}

	embedding, err := e.doEmbedRequest(ctx, text)
	if err != nil {
		return nil, err
	}

	// Store in cache
	if e.cacheEnabled && e.cache != nil {
		e.cache.put(text, embedding)
	}

	return embedding, nil
}

// EmbedFast generates an embedding with the fast timeout.
func (e *OpenAIEmbedder) EmbedFast(ctx context.Context, text string) (cognitive.Embedding, error) {
	if !e.Available() {
		return nil, fmt.Errorf("OpenAI embedder not available")
	}

	// Check cache first
	if e.cacheEnabled && e.cache != nil {
		if cached := e.cache.get(text); cached != nil {
			return cached, nil
		}
	}

	fastCtx, cancel := context.WithTimeout(ctx, e.fastTimeout)
	defer cancel()

	embedding, err := e.doEmbedRequest(fastCtx, text)
	if err != nil {
		if IsTimeoutError(err) {
			return nil, ErrEmbeddingTimeout
		}
		return nil, err
	}

	if e.cacheEnabled && e.cache != nil {
		e.cache.put(text, embedding)
	}

	return embedding, nil
}

// EmbedBatch generates embeddings for multiple texts.
func (e *OpenAIEmbedder) EmbedBatch(ctx context.Context, texts []string) ([]cognitive.Embedding, error) {
	if !e.Available() {
		return nil, fmt.Errorf("OpenAI embedder not available")
	}

	// OpenAI supports batch embeddings natively
	return e.doEmbedBatchRequest(ctx, texts)
}

// doEmbedRequest performs a single embedding request.
func (e *OpenAIEmbedder) doEmbedRequest(ctx context.Context, text string) (cognitive.Embedding, error) {
	embeddings, err := e.doEmbedBatchRequest(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return embeddings[0], nil
}

// doEmbedBatchRequest performs a batch embedding request.
func (e *OpenAIEmbedder) doEmbedBatchRequest(ctx context.Context, texts []string) ([]cognitive.Embedding, error) {
	reqBody := map[string]interface{}{
		"input": texts,
		"model": e.model,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/embeddings", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			} `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)

		// Handle quota exceeded (429) - disable embedder for 1 hour
		// These errors should NOT be retried - they indicate billing/limit issues
		if resp.StatusCode == http.StatusTooManyRequests {
			e.setQuotaExceeded(1 * time.Hour)
			return nil, fmt.Errorf("OpenAI quota exceeded (status 429): %s - embedder disabled for 1 hour", errResp.Error.Message)
		}

		return nil, fmt.Errorf("OpenAI error (status %d): %s", resp.StatusCode, errResp.Error.Message)
	}

	var result struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Sort by index and convert to float32
	embeddings := make([]cognitive.Embedding, len(texts))
	for _, item := range result.Data {
		if item.Index >= len(embeddings) {
			continue
		}
		embedding := make(cognitive.Embedding, len(item.Embedding))
		for i, v := range item.Embedding {
			embedding[i] = float32(v)
		}
		embeddings[item.Index] = embedding
	}

	// Update dimension if different
	if len(result.Data) > 0 && len(result.Data[0].Embedding) != e.dimension {
		e.dimension = len(result.Data[0].Embedding)
	}

	return embeddings, nil
}

// Dimension returns the embedding dimension.
func (e *OpenAIEmbedder) Dimension() int {
	return e.dimension
}

// ModelName returns the name of the embedding model.
func (e *OpenAIEmbedder) ModelName() string {
	return e.model
}

// Available returns true if the embedder is ready to use.
// Returns false if API key is missing or quota is exceeded.
func (e *OpenAIEmbedder) Available() bool {
	e.availableMu.RLock()
	defer e.availableMu.RUnlock()

	if !e.available {
		return false
	}

	// Check if quota was exceeded and if reset time has passed
	if e.quotaExceeded {
		if time.Now().Before(e.quotaResetTime) {
			return false // Still in cooldown period
		}
		// Reset time passed, allow retry
		// Note: actual reset happens in setQuotaExceeded with write lock
	}

	return true
}

// setQuotaExceeded marks the embedder as having exceeded quota.
// It will be unavailable for the specified duration.
func (e *OpenAIEmbedder) setQuotaExceeded(duration time.Duration) {
	e.availableMu.Lock()
	defer e.availableMu.Unlock()
	e.quotaExceeded = true
	e.quotaResetTime = time.Now().Add(duration)
	e.log.Warn("[OpenAI Embedder] Quota exceeded, disabled for %v", duration)
}

// resetQuotaExceeded clears the quota exceeded state.
func (e *OpenAIEmbedder) resetQuotaExceeded() {
	e.availableMu.Lock()
	defer e.availableMu.Unlock()
	if e.quotaExceeded && time.Now().After(e.quotaResetTime) {
		e.quotaExceeded = false
		e.log.Info("[OpenAI Embedder] Quota cooldown expired, re-enabling")
	}
}

// FastTimeout returns the configured fast path timeout.
func (e *OpenAIEmbedder) FastTimeout() time.Duration {
	return e.fastTimeout
}

// GetCacheStats returns current embedding cache statistics.
func (e *OpenAIEmbedder) GetCacheStats() CacheStats {
	if !e.cacheEnabled || e.cache == nil {
		return CacheStats{}
	}
	return e.cache.stats()
}
