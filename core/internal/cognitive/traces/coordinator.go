package traces

import (
	"context"
	"database/sql"
	"sync"

	"github.com/normanking/cortex/internal/agent"
	"github.com/normanking/cortex/internal/memory"
)

// Coordinator provides the main entry point for trace functionality.
// It manages trace storage, retrieval, and collection in a thread-safe manner.
type Coordinator struct {
	store     *TraceStore
	retriever *TraceRetriever
	scorer    *TraceScorer
	embedder  memory.Embedder

	// Active collectors by request ID
	collectors sync.Map // map[string]*TraceCollector

	// Configuration
	enabled            bool
	minSimilarity      float64
	maxRetrievedTraces int
	autoStore          bool // Automatically store traces on completion
}

// Config configures the trace coordinator.
type Config struct {
	// Enabled controls whether trace capture is active.
	Enabled bool

	// MinSimilarity is the minimum similarity threshold for trace retrieval.
	MinSimilarity float64

	// MaxRetrievedTraces limits how many similar traces to return.
	MaxRetrievedTraces int

	// AutoStore automatically stores traces on successful completion.
	AutoStore bool
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Enabled:            true,
		MinSimilarity:      0.7,
		MaxRetrievedTraces: 5,
		AutoStore:          true,
	}
}

// NewCoordinator creates a new trace coordinator.
func NewCoordinator(db *sql.DB, embedder memory.Embedder, episodes *memory.EpisodeStore, cfg *Config) *Coordinator {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	store := NewTraceStore(db, embedder, episodes)
	retriever := NewTraceRetriever(db, embedder)
	retriever.SetMinSimilarity(cfg.MinSimilarity)
	retriever.SetMaxResults(cfg.MaxRetrievedTraces)

	return &Coordinator{
		store:              store,
		retriever:          retriever,
		scorer:             NewTraceScorer(),
		embedder:           embedder,
		enabled:            cfg.Enabled,
		minSimilarity:      cfg.MinSimilarity,
		maxRetrievedTraces: cfg.MaxRetrievedTraces,
		autoStore:          cfg.AutoStore,
	}
}

// Enabled returns whether trace collection is enabled.
func (c *Coordinator) Enabled() bool {
	return c.enabled
}

// SetEnabled enables or disables trace collection.
func (c *Coordinator) SetEnabled(enabled bool) {
	c.enabled = enabled
}

// BeforeReasoning is called before agent reasoning begins.
// It searches for similar traces that might be reusable.
// Returns the best matching trace if found, or nil.
func (c *Coordinator) BeforeReasoning(ctx context.Context, requestID, query string) (*TraceSimilarity, error) {
	if !c.enabled {
		return nil, nil
	}

	// Find similar traces
	candidates, err := c.retriever.FindSimilar(ctx, query)
	if err != nil {
		return nil, err
	}

	if len(candidates) == 0 {
		return nil, nil
	}

	// Select the best trace
	best := c.scorer.SelectBestTrace(candidates)
	return best, nil
}

// StartCollection begins trace collection for a request.
// Returns a StepCallback that should be passed to the agent.
func (c *Coordinator) StartCollection(requestID, query string, originalCallback agent.StepCallback) agent.StepCallback {
	if !c.enabled {
		return originalCallback
	}

	collector := NewTraceCollector(c.store, c.embedder)
	callback := collector.Start(query, originalCallback)

	c.collectors.Store(requestID, collector)

	return callback
}

// FinishCollection completes trace collection and stores the trace.
// Should be called after agent execution completes.
func (c *Coordinator) FinishCollection(ctx context.Context, requestID string, outcome TraceOutcome) (*ReasoningTrace, error) {
	if !c.enabled {
		return nil, nil
	}

	value, ok := c.collectors.LoadAndDelete(requestID)
	if !ok {
		return nil, nil
	}

	collector, ok := value.(*TraceCollector)
	if !ok {
		return nil, nil
	}

	if c.autoStore {
		return collector.Finish(ctx, outcome)
	}

	// Cancel without storing
	collector.Cancel()
	return nil, nil
}

// CancelCollection cancels trace collection without storing.
func (c *Coordinator) CancelCollection(requestID string) {
	value, ok := c.collectors.LoadAndDelete(requestID)
	if !ok {
		return
	}
	collector, ok := value.(*TraceCollector)
	if !ok {
		return
	}
	collector.Cancel()
}

// SetLobesActivated records which lobes were activated for the current trace.
func (c *Coordinator) SetLobesActivated(requestID string, lobes []string) {
	value, ok := c.collectors.Load(requestID)
	if !ok {
		return
	}
	collector, ok := value.(*TraceCollector)
	if !ok {
		return
	}
	collector.SetLobesActivated(lobes)
}

// SetMetadata adds metadata to the current trace.
func (c *Coordinator) SetMetadata(requestID, key, value string) {
	val, ok := c.collectors.Load(requestID)
	if !ok {
		return
	}
	collector, ok := val.(*TraceCollector)
	if !ok {
		return
	}
	collector.SetMetadata(key, value)
}

// IncrementTraceReuse marks a trace as reused.
func (c *Coordinator) IncrementTraceReuse(ctx context.Context, traceID string) error {
	return c.store.IncrementReuse(ctx, traceID)
}

// GetRecentTraces retrieves recent traces for display.
func (c *Coordinator) GetRecentTraces(ctx context.Context, limit int) ([]*ReasoningTrace, error) {
	return c.store.GetRecentTraces(ctx, limit)
}

// GetStats returns trace statistics.
func (c *Coordinator) GetStats(ctx context.Context) (*TraceStats, error) {
	return c.store.Stats(ctx)
}

// SearchTraces searches for traces by text.
func (c *Coordinator) SearchTraces(ctx context.Context, query string, limit int) ([]*ReasoningTrace, error) {
	return c.retriever.SearchByText(ctx, query, limit)
}

// PruneTraces removes old, low-quality traces.
func (c *Coordinator) PruneTraces(ctx context.Context) (int, error) {
	// Default: 30 days, min score 0.3
	return c.store.PruneOldTraces(ctx, 30*24*3600*1e9, 0.3)
}

// Store returns the underlying trace store for direct access.
func (c *Coordinator) Store() *TraceStore {
	return c.store
}

// Retriever returns the underlying trace retriever for direct access.
func (c *Coordinator) Retriever() *TraceRetriever {
	return c.retriever
}

// Scorer returns the underlying trace scorer for direct access.
func (c *Coordinator) Scorer() *TraceScorer {
	return c.scorer
}
