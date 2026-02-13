// Package orchestrator provides the trace coordinator for CR-024.
// This coordinator handles reasoning trace storage and retrieval for System 3 meta-cognition.
package orchestrator

import (
	"context"
	"database/sql"

	"github.com/normanking/cortex/internal/agent"
	"github.com/normanking/cortex/internal/cognitive/traces"
	"github.com/normanking/cortex/internal/memory"
)

// TraceCoordinator defines the interface for reasoning trace operations.
// CR-024: System 3 Meta-Cognitive Enhancement
type TraceCoordinator interface {
	// Enabled returns whether trace collection is enabled.
	Enabled() bool

	// SetEnabled enables or disables trace collection.
	SetEnabled(enabled bool)

	// BeforeReasoning is called before agent reasoning begins.
	// Returns the best matching trace if found, or nil.
	BeforeReasoning(ctx context.Context, requestID, query string) (*traces.TraceSimilarity, error)

	// StartCollection begins trace collection for a request.
	// Returns a StepCallback that wraps the original callback.
	StartCollection(requestID, query string, originalCallback agent.StepCallback) agent.StepCallback

	// FinishCollection completes trace collection and stores the trace.
	FinishCollection(ctx context.Context, requestID string, outcome traces.TraceOutcome) (*traces.ReasoningTrace, error)

	// CancelCollection cancels trace collection without storing.
	CancelCollection(requestID string)

	// SetLobesActivated records which lobes were activated.
	SetLobesActivated(requestID string, lobes []string)

	// SetMetadata adds metadata to the current trace.
	SetMetadata(requestID, key, value string)

	// IncrementTraceReuse marks a trace as reused.
	IncrementTraceReuse(ctx context.Context, traceID string) error

	// GetRecentTraces retrieves recent traces.
	GetRecentTraces(ctx context.Context, limit int) ([]*traces.ReasoningTrace, error)

	// GetStats returns trace statistics.
	GetStats(ctx context.Context) (*traces.TraceStats, error)

	// SearchTraces searches for traces by text.
	SearchTraces(ctx context.Context, query string, limit int) ([]*traces.ReasoningTrace, error)

	// PruneTraces removes old, low-quality traces.
	PruneTraces(ctx context.Context) (int, error)
}

// TraceCoordinatorConfig holds configuration for the trace coordinator.
type TraceCoordinatorConfig struct {
	// Enabled controls whether trace capture is active.
	Enabled bool

	// MinSimilarity is the minimum similarity threshold for retrieval.
	MinSimilarity float64

	// MaxRetrievedTraces limits how many similar traces to return.
	MaxRetrievedTraces int

	// AutoStore automatically stores traces on completion.
	AutoStore bool
}

// DefaultTraceCoordinatorConfig returns sensible defaults.
func DefaultTraceCoordinatorConfig() *TraceCoordinatorConfig {
	return &TraceCoordinatorConfig{
		Enabled:            true,
		MinSimilarity:      0.7,
		MaxRetrievedTraces: 5,
		AutoStore:          true,
	}
}

// defaultTraceCoordinator wraps traces.Coordinator.
type defaultTraceCoordinator struct {
	coord *traces.Coordinator
}

// NewTraceCoordinator creates a new trace coordinator.
func NewTraceCoordinator(db *sql.DB, embedder memory.Embedder, episodes *memory.EpisodeStore, cfg *TraceCoordinatorConfig) TraceCoordinator {
	if cfg == nil {
		cfg = DefaultTraceCoordinatorConfig()
	}

	traceCfg := &traces.Config{
		Enabled:            cfg.Enabled,
		MinSimilarity:      cfg.MinSimilarity,
		MaxRetrievedTraces: cfg.MaxRetrievedTraces,
		AutoStore:          cfg.AutoStore,
	}

	coord := traces.NewCoordinator(db, embedder, episodes, traceCfg)

	return &defaultTraceCoordinator{coord: coord}
}

func (d *defaultTraceCoordinator) Enabled() bool {
	return d.coord.Enabled()
}

func (d *defaultTraceCoordinator) SetEnabled(enabled bool) {
	d.coord.SetEnabled(enabled)
}

func (d *defaultTraceCoordinator) BeforeReasoning(ctx context.Context, requestID, query string) (*traces.TraceSimilarity, error) {
	return d.coord.BeforeReasoning(ctx, requestID, query)
}

func (d *defaultTraceCoordinator) StartCollection(requestID, query string, originalCallback agent.StepCallback) agent.StepCallback {
	return d.coord.StartCollection(requestID, query, originalCallback)
}

func (d *defaultTraceCoordinator) FinishCollection(ctx context.Context, requestID string, outcome traces.TraceOutcome) (*traces.ReasoningTrace, error) {
	return d.coord.FinishCollection(ctx, requestID, outcome)
}

func (d *defaultTraceCoordinator) CancelCollection(requestID string) {
	d.coord.CancelCollection(requestID)
}

func (d *defaultTraceCoordinator) SetLobesActivated(requestID string, lobes []string) {
	d.coord.SetLobesActivated(requestID, lobes)
}

func (d *defaultTraceCoordinator) SetMetadata(requestID, key, value string) {
	d.coord.SetMetadata(requestID, key, value)
}

func (d *defaultTraceCoordinator) IncrementTraceReuse(ctx context.Context, traceID string) error {
	return d.coord.IncrementTraceReuse(ctx, traceID)
}

func (d *defaultTraceCoordinator) GetRecentTraces(ctx context.Context, limit int) ([]*traces.ReasoningTrace, error) {
	return d.coord.GetRecentTraces(ctx, limit)
}

func (d *defaultTraceCoordinator) GetStats(ctx context.Context) (*traces.TraceStats, error) {
	return d.coord.GetStats(ctx)
}

func (d *defaultTraceCoordinator) SearchTraces(ctx context.Context, query string, limit int) ([]*traces.ReasoningTrace, error) {
	return d.coord.SearchTraces(ctx, query, limit)
}

func (d *defaultTraceCoordinator) PruneTraces(ctx context.Context) (int, error) {
	return d.coord.PruneTraces(ctx)
}

// WithTraceCoordinator sets the trace coordinator.
// CR-024: System 3 Meta-Cognitive Enhancement
func WithTraceCoordinator(tc TraceCoordinator) Option {
	return func(o *Orchestrator) {
		o.traces = tc
	}
}

// noopTraceCoordinator is a disabled trace coordinator.
type noopTraceCoordinator struct{}

func (n *noopTraceCoordinator) Enabled() bool                            { return false }
func (n *noopTraceCoordinator) SetEnabled(enabled bool)                  {}
func (n *noopTraceCoordinator) BeforeReasoning(ctx context.Context, requestID, query string) (*traces.TraceSimilarity, error) {
	return nil, nil
}
func (n *noopTraceCoordinator) StartCollection(requestID, query string, originalCallback agent.StepCallback) agent.StepCallback {
	return originalCallback
}
func (n *noopTraceCoordinator) FinishCollection(ctx context.Context, requestID string, outcome traces.TraceOutcome) (*traces.ReasoningTrace, error) {
	return nil, nil
}
func (n *noopTraceCoordinator) CancelCollection(requestID string)                  {}
func (n *noopTraceCoordinator) SetLobesActivated(requestID string, lobes []string) {}
func (n *noopTraceCoordinator) SetMetadata(requestID, key, value string)           {}
func (n *noopTraceCoordinator) IncrementTraceReuse(ctx context.Context, traceID string) error {
	return nil
}
func (n *noopTraceCoordinator) GetRecentTraces(ctx context.Context, limit int) ([]*traces.ReasoningTrace, error) {
	return nil, nil
}
func (n *noopTraceCoordinator) GetStats(ctx context.Context) (*traces.TraceStats, error) {
	return &traces.TraceStats{}, nil
}
func (n *noopTraceCoordinator) SearchTraces(ctx context.Context, query string, limit int) ([]*traces.ReasoningTrace, error) {
	return nil, nil
}
func (n *noopTraceCoordinator) PruneTraces(ctx context.Context) (int, error) { return 0, nil }

// NoopTraceCoordinator returns a disabled trace coordinator.
func NoopTraceCoordinator() TraceCoordinator {
	return &noopTraceCoordinator{}
}
