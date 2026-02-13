package router

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/normanking/cortex/internal/cognitive"
	"github.com/normanking/cortex/internal/logging"
)

// ═══════════════════════════════════════════════════════════════════════════════
// SEMANTIC ROUTER
// ═══════════════════════════════════════════════════════════════════════════════

// Router performs semantic similarity routing for user requests.
// It matches incoming requests against a template index to determine
// the best template or if the request is novel.
type Router struct {
	embedder Embedder
	index    *EmbeddingIndex
	registry cognitive.Registry
	log      *logging.Logger

	// Configuration
	thresholds *Thresholds

	// State
	mu            sync.RWMutex
	initialized   bool
	lastRefresh   time.Time
	refreshPeriod time.Duration
}

// Thresholds configures similarity thresholds for routing decisions.
type Thresholds struct {
	High   float64 // >= High: Strong match, use template directly
	Medium float64 // >= Medium: Moderate match, use template with caution
	Low    float64 // >= Low: Weak match, consider template but may need adjustment
	// Below Low: No match, route to frontier for novel handling
}

// DefaultThresholds returns the default similarity thresholds.
func DefaultThresholds() *Thresholds {
	return &Thresholds{
		High:   cognitive.ThresholdHigh,   // 0.85
		Medium: cognitive.ThresholdMedium, // 0.70
		Low:    cognitive.ThresholdLow,    // 0.50
	}
}

// RouterConfig configures the semantic router.
type RouterConfig struct {
	Embedder      Embedder
	Registry      cognitive.Registry
	Thresholds    *Thresholds
	RefreshPeriod time.Duration // How often to refresh index from registry
}

// NewRouter creates a new semantic router.
func NewRouter(cfg *RouterConfig) *Router {
	if cfg.Thresholds == nil {
		cfg.Thresholds = DefaultThresholds()
	}

	refreshPeriod := cfg.RefreshPeriod
	if refreshPeriod == 0 {
		refreshPeriod = 5 * time.Minute
	}

	r := &Router{
		embedder:      cfg.Embedder,
		index:         NewEmbeddingIndex(),
		registry:      cfg.Registry,
		log:           logging.Global(),
		thresholds:    cfg.Thresholds,
		refreshPeriod: refreshPeriod,
	}

	return r
}

// Initialize loads active templates into the index.
// Should be called once at startup.
func (r *Router) Initialize(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.refreshIndexLocked(ctx)
}

// refreshIndexLocked reloads templates into the index.
// Caller must hold r.mu.
func (r *Router) refreshIndexLocked(ctx context.Context) error {
	// Get all active templates
	templates, err := r.registry.ListActive(ctx)
	if err != nil {
		return fmt.Errorf("list active templates: %w", err)
	}

	r.log.Info("[Router] Loading %d active templates into index", len(templates))

	// Clear and rebuild index
	r.index.Clear()

	for _, t := range templates {
		if len(t.IntentEmbedding) > 0 {
			r.index.Add(t.ID, t.IntentEmbedding, t)
		}
	}

	r.initialized = true
	r.lastRefresh = time.Now()

	r.log.Info("[Router] Index ready with %d templates", r.index.Size())
	return nil
}

// RefreshIfNeeded refreshes the index if the refresh period has elapsed.
func (r *Router) RefreshIfNeeded(ctx context.Context) error {
	r.mu.RLock()
	needsRefresh := time.Since(r.lastRefresh) > r.refreshPeriod
	r.mu.RUnlock()

	if needsRefresh {
		r.mu.Lock()
		defer r.mu.Unlock()
		// Double-check after acquiring write lock
		if time.Since(r.lastRefresh) > r.refreshPeriod {
			return r.refreshIndexLocked(ctx)
		}
	}

	return nil
}

// Route performs semantic routing for a user request.
// Uses fast path embedding with immediate FTS fallback on timeout.
func (r *Router) Route(ctx context.Context, input string) (*cognitive.RoutingResult, error) {
	start := time.Now()

	result := &cognitive.RoutingResult{}

	// Check if embedder is available
	if !r.embedder.Available() {
		r.log.Debug("[Router] Embedder unavailable, falling back to keyword search")
		return r.fallbackRoute(ctx, input, start)
	}

	// Refresh index if needed
	if err := r.RefreshIfNeeded(ctx); err != nil {
		r.log.Warn("[Router] Failed to refresh index: %v", err)
	}

	// Generate embedding using fast path (5s timeout, no retries)
	// This ensures the main request path is never blocked for long
	embedding, err := r.embedder.EmbedFast(ctx, input)
	if err != nil {
		// Check if this was a timeout - use fast FTS fallback
		if err == ErrEmbeddingTimeout || IsTimeoutError(err) {
			r.log.Info("[Router] Embedding timed out, using FTS-only search (fast path)")
		} else {
			r.log.Warn("[Router] Failed to generate embedding: %v", err)
		}
		result.EmbeddingFailed = true
		return r.fallbackRoute(ctx, input, start)
	}

	result.InputEmbedding = embedding

	// Search for similar templates
	r.mu.RLock()
	searchResults := r.index.Search(embedding, 5) // Get top 5
	r.mu.RUnlock()

	if len(searchResults) == 0 {
		// No templates in index at all
		result.Decision = cognitive.RouteNovel
		result.RecommendedTier = cognitive.TierFrontier
		result.ProcessingMs = int(time.Since(start).Milliseconds())
		return result, nil
	}

	// Get the best match
	best := searchResults[0]
	similarity := best.Score

	// Determine routing based on similarity
	if similarity >= r.thresholds.Low {
		// Template match found
		template := best.Metadata.(*cognitive.Template)

		result.Decision = cognitive.RouteTemplate
		result.Match = &cognitive.TemplateMatch{
			Template:        template,
			SimilarityScore: similarity,
			SimilarityLevel: cognitive.GetSimilarityLevel(similarity),
			MatchMethod:     "embedding",
		}

		// Determine model tier based on confidence and similarity
		result.RecommendedTier = r.selectModelTier(template, similarity)
		result.RecommendedModel = r.selectModel(result.RecommendedTier)
	} else {
		// Novel request - needs frontier model + distillation
		result.Decision = cognitive.RouteNovel
		result.RecommendedTier = cognitive.TierFrontier
		result.RecommendedModel = r.selectModel(cognitive.TierFrontier)
	}

	result.ProcessingMs = int(time.Since(start).Milliseconds())
	return result, nil
}

// AsyncRoutingResult wraps a routing result with optional async embedding improvement.
type AsyncRoutingResult struct {
	// Initial result (may be FTS-only)
	Result *cognitive.RoutingResult

	// Channel to receive improved result when embedding completes (optional)
	// Will be nil if embedding succeeded synchronously
	ImprovedResult <-chan *cognitive.RoutingResult
}

// RouteAsync performs routing with immediate FTS results and optional async embedding.
// Returns FTS results immediately, then improves results when embedding completes.
func (r *Router) RouteAsync(ctx context.Context, input string) (*AsyncRoutingResult, error) {
	start := time.Now()

	// Check if embedder is available
	if !r.embedder.Available() {
		r.log.Debug("[Router] Embedder unavailable, falling back to keyword search")
		result, err := r.fallbackRoute(ctx, input, start)
		return &AsyncRoutingResult{Result: result}, err
	}

	// Refresh index if needed (non-blocking)
	if err := r.RefreshIfNeeded(ctx); err != nil {
		r.log.Warn("[Router] Failed to refresh index: %v", err)
	}

	// Try fast embedding first
	embedding, err := r.embedder.EmbedFast(ctx, input)
	if err == nil {
		// Embedding succeeded quickly - use synchronous path
		result, err := r.routeWithEmbedding(ctx, input, embedding, start)
		return &AsyncRoutingResult{Result: result}, err
	}

	// Embedding timed out or failed - return FTS immediately, improve async
	if err == ErrEmbeddingTimeout || IsTimeoutError(err) {
		r.log.Info("[Router] Embedding timed out, returning FTS results immediately (async improvement pending)")
	} else {
		r.log.Warn("[Router] Fast embedding failed: %v, falling back to FTS", err)
	}

	// Get FTS results immediately
	ftsResult, err := r.fallbackRoute(ctx, input, start)
	if err != nil {
		return &AsyncRoutingResult{Result: ftsResult}, err
	}

	// Start async embedding improvement
	improvedCh := make(chan *cognitive.RoutingResult, 1)
	go func() {
		defer close(improvedCh)

		// Use full timeout for background embedding
		bgCtx, cancel := context.WithTimeout(context.Background(), r.embedder.FastTimeout()*6) // 30s
		defer cancel()

		embedding, err := r.embedder.Embed(bgCtx, input)
		if err != nil {
			r.log.Debug("[Router] Async embedding failed: %v", err)
			return
		}

		// Route with the completed embedding
		improved, err := r.routeWithEmbedding(bgCtx, input, embedding, start)
		if err != nil {
			r.log.Debug("[Router] Async routing failed: %v", err)
			return
		}

		r.log.Debug("[Router] Async embedding completed, improved result available")
		improvedCh <- improved
	}()

	return &AsyncRoutingResult{
		Result:         ftsResult,
		ImprovedResult: improvedCh,
	}, nil
}

// routeWithEmbedding performs routing using a pre-computed embedding.
func (r *Router) routeWithEmbedding(ctx context.Context, input string, embedding cognitive.Embedding, start time.Time) (*cognitive.RoutingResult, error) {
	result := &cognitive.RoutingResult{
		InputEmbedding: embedding,
	}

	// Search for similar templates
	r.mu.RLock()
	searchResults := r.index.Search(embedding, 5) // Get top 5
	r.mu.RUnlock()

	if len(searchResults) == 0 {
		// No templates in index at all
		result.Decision = cognitive.RouteNovel
		result.RecommendedTier = cognitive.TierFrontier
		result.ProcessingMs = int(time.Since(start).Milliseconds())
		return result, nil
	}

	// Get the best match
	best := searchResults[0]
	similarity := best.Score

	// Determine routing based on similarity
	if similarity >= r.thresholds.Low {
		// Template match found
		template := best.Metadata.(*cognitive.Template)

		result.Decision = cognitive.RouteTemplate
		result.Match = &cognitive.TemplateMatch{
			Template:        template,
			SimilarityScore: similarity,
			SimilarityLevel: cognitive.GetSimilarityLevel(similarity),
			MatchMethod:     "embedding",
		}

		// Determine model tier based on confidence and similarity
		result.RecommendedTier = r.selectModelTier(template, similarity)
		result.RecommendedModel = r.selectModel(result.RecommendedTier)
	} else {
		// Novel request - needs frontier model + distillation
		result.Decision = cognitive.RouteNovel
		result.RecommendedTier = cognitive.TierFrontier
		result.RecommendedModel = r.selectModel(cognitive.TierFrontier)
	}

	result.ProcessingMs = int(time.Since(start).Milliseconds())
	return result, nil
}

// fallbackRoute uses keyword search when embeddings are unavailable.
func (r *Router) fallbackRoute(ctx context.Context, input string, start time.Time) (*cognitive.RoutingResult, error) {
	result := &cognitive.RoutingResult{
		Decision:        cognitive.RouteFallback,
		EmbeddingFailed: true,
	}

	// Try FTS search
	templates, err := r.registry.SearchByKeywords(ctx, input, 5)
	if err != nil {
		r.log.Warn("[Router] FTS search failed: %v", err)
		// Fall through to novel handling
	}

	if len(templates) > 0 {
		// Found a keyword match
		result.Decision = cognitive.RouteTemplate
		result.Match = &cognitive.TemplateMatch{
			Template:        templates[0],
			SimilarityScore: 0.5, // Default score for keyword matches
			SimilarityLevel: cognitive.SimilarityLow,
			MatchMethod:     "keyword",
		}
		result.RecommendedTier = cognitive.TierMid // Use mid-tier for keyword matches
	} else {
		// No match - route to frontier
		result.Decision = cognitive.RouteNovel
		result.RecommendedTier = cognitive.TierFrontier
	}

	result.RecommendedModel = r.selectModel(result.RecommendedTier)
	result.ProcessingMs = int(time.Since(start).Milliseconds())

	return result, nil
}

// selectModelTier determines the appropriate model tier based on template and similarity.
func (r *Router) selectModelTier(t *cognitive.Template, similarity float64) cognitive.ModelTier {
	// High similarity + high confidence = local model
	if similarity >= r.thresholds.High && t.ConfidenceScore >= 0.8 {
		return cognitive.TierLocal
	}

	// High similarity + medium confidence = mid-tier
	if similarity >= r.thresholds.High && t.ConfidenceScore >= 0.6 {
		return cognitive.TierMid
	}

	// Medium similarity = mid-tier
	if similarity >= r.thresholds.Medium {
		return cognitive.TierMid
	}

	// Low similarity = advanced tier
	return cognitive.TierAdvanced
}

// selectModel returns the recommended model for a tier.
func (r *Router) selectModel(tier cognitive.ModelTier) string {
	switch tier {
	case cognitive.TierLocal:
		return "llama3.2:latest" // Default Ollama model
	case cognitive.TierMid:
		return "claude-3-5-haiku-latest"
	case cognitive.TierAdvanced:
		return "claude-sonnet-4-20250514"
	case cognitive.TierFrontier:
		return "claude-sonnet-4-20250514" // Use Sonnet for distillation
	default:
		return "claude-sonnet-4-20250514"
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// INDEX MANAGEMENT
// ═══════════════════════════════════════════════════════════════════════════════

// AddTemplate adds or updates a template in the index.
// Use this for real-time updates when a new template is created.
func (r *Router) AddTemplate(ctx context.Context, t *cognitive.Template) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Generate embedding if not present
	if len(t.IntentEmbedding) == 0 {
		if !r.embedder.Available() {
			return fmt.Errorf("embedder unavailable")
		}

		embedding, err := r.embedder.Embed(ctx, t.Intent)
		if err != nil {
			return fmt.Errorf("generate embedding: %w", err)
		}
		t.IntentEmbedding = embedding
	}

	r.index.Add(t.ID, t.IntentEmbedding, t)
	r.log.Debug("[Router] Added template %s to index", t.ID)

	return nil
}

// RemoveTemplate removes a template from the index.
func (r *Router) RemoveTemplate(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.index.Remove(id) {
		r.log.Debug("[Router] Removed template %s from index", id)
	}
}

// IndexSize returns the number of templates in the index.
func (r *Router) IndexSize() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.index.Size()
}

// EmbedderAvailable returns true if the embedder is ready.
func (r *Router) EmbedderAvailable() bool {
	return r.embedder.Available()
}

// ═══════════════════════════════════════════════════════════════════════════════
// STATISTICS
// ═══════════════════════════════════════════════════════════════════════════════

// RouterStats contains router performance statistics.
type RouterStats struct {
	IndexSize          int       `json:"index_size"`
	EmbedderAvailable  bool      `json:"embedder_available"`
	EmbeddingModel     string    `json:"embedding_model"`
	EmbeddingDimension int       `json:"embedding_dimension"`
	LastRefresh        time.Time `json:"last_refresh"`
	Initialized        bool      `json:"initialized"`
}

// Stats returns current router statistics.
func (r *Router) Stats() *RouterStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return &RouterStats{
		IndexSize:          r.index.Size(),
		EmbedderAvailable:  r.embedder.Available(),
		EmbeddingModel:     r.embedder.ModelName(),
		EmbeddingDimension: r.embedder.Dimension(),
		LastRefresh:        r.lastRefresh,
		Initialized:        r.initialized,
	}
}
