package orchestrator

import (
	"context"

	"github.com/normanking/cortex/internal/cognitive/supervision"
)

// SupervisionCoordinator defines the interface for process-supervised thought search.
// It wraps the supervision.Coordinator to provide orchestrator-level integration.
type SupervisionCoordinator interface {
	// Enabled returns whether supervision is enabled.
	Enabled() bool

	// SetEnabled enables or disables supervision.
	SetEnabled(enabled bool)

	// EvaluateThought evaluates a single thought using the guardian.
	// Returns approval status, score, and any suggestions.
	EvaluateThought(ctx context.Context, content string) (*supervision.GuardianResult, error)

	// BuildTree constructs a thought tree for supervised reasoning.
	// This enables Tree-of-Thought exploration with guardian critique.
	BuildTree(ctx context.Context, requestID, query, initialThought string, expander supervision.Expander) (*supervision.ThoughtTree, error)

	// SuperviseStep evaluates a single reasoning step and provides feedback.
	// This is the main integration point for step-by-step supervision.
	SuperviseStep(ctx context.Context, step *supervision.SupervisedStep) (*supervision.SupervisedStep, error)

	// BatchSupervise evaluates multiple steps in parallel.
	BatchSupervise(ctx context.Context, steps []*supervision.SupervisedStep) ([]*supervision.SupervisedStep, error)

	// SelectBestPath selects the best reasoning path from a tree.
	SelectBestPath(tree *supervision.ThoughtTree) *supervision.PathScore

	// RankPaths ranks all complete paths in a tree.
	RankPaths(tree *supervision.ThoughtTree) []*supervision.PathScore

	// GetActiveTree retrieves an active tree by request ID.
	GetActiveTree(requestID string) *supervision.ThoughtTree

	// CompleteTree marks a tree as complete and removes it from active trees.
	CompleteTree(requestID string) *supervision.ThoughtTree

	// PruneTree removes low-scoring nodes from a tree.
	PruneTree(tree *supervision.ThoughtTree) int

	// GetStats returns statistics about active supervision.
	GetStats() map[string]interface{}

	// Config returns the current configuration.
	Config() *supervision.Config
}

// SupervisionCoordinatorConfig configures the supervision coordinator.
type SupervisionCoordinatorConfig struct {
	// Enabled controls whether supervision is active
	Enabled bool

	// MaxBranches is the maximum branches per node (default: 3)
	MaxBranches int

	// MaxDepth is the maximum tree depth (default: 4)
	MaxDepth int

	// MaxNodes is the absolute maximum nodes in the tree (default: 20)
	MaxNodes int

	// PruneThreshold is the minimum score to keep a branch (default: 0.3)
	PruneThreshold float64
}

// DefaultSupervisionCoordinatorConfig returns sensible defaults.
func DefaultSupervisionCoordinatorConfig() *SupervisionCoordinatorConfig {
	return &SupervisionCoordinatorConfig{
		Enabled:        true,
		MaxBranches:    3,
		MaxDepth:       4,
		MaxNodes:       20,
		PruneThreshold: 0.3,
	}
}

// defaultSupervisionCoordinator wraps the supervision.Coordinator.
type defaultSupervisionCoordinator struct {
	coord *supervision.Coordinator
}

// NewSupervisionCoordinator creates a new supervision coordinator.
func NewSupervisionCoordinator(cfg *SupervisionCoordinatorConfig) SupervisionCoordinator {
	if cfg == nil {
		cfg = DefaultSupervisionCoordinatorConfig()
	}

	// Convert to internal config
	supCfg := &supervision.Config{
		Enabled:        cfg.Enabled,
		MaxBranches:    cfg.MaxBranches,
		MaxDepth:       cfg.MaxDepth,
		MaxNodes:       cfg.MaxNodes,
		PruneThreshold: cfg.PruneThreshold,
	}

	coord := supervision.NewCoordinator(supCfg)
	return &defaultSupervisionCoordinator{coord: coord}
}

// NoopSupervisionCoordinator returns a disabled supervisor.
func NoopSupervisionCoordinator() SupervisionCoordinator {
	return &noopSupervisionCoordinator{}
}

// Compile-time interface verification
var _ SupervisionCoordinator = (*defaultSupervisionCoordinator)(nil)
var _ SupervisionCoordinator = (*noopSupervisionCoordinator)(nil)

// defaultSupervisionCoordinator implementation

func (d *defaultSupervisionCoordinator) Enabled() bool {
	return d.coord.Enabled()
}

func (d *defaultSupervisionCoordinator) SetEnabled(enabled bool) {
	d.coord.SetEnabled(enabled)
}

func (d *defaultSupervisionCoordinator) EvaluateThought(ctx context.Context, content string) (*supervision.GuardianResult, error) {
	return d.coord.EvaluateThought(ctx, content)
}

func (d *defaultSupervisionCoordinator) BuildTree(ctx context.Context, requestID, query, initialThought string, expander supervision.Expander) (*supervision.ThoughtTree, error) {
	return d.coord.BuildTree(ctx, requestID, query, initialThought, expander)
}

func (d *defaultSupervisionCoordinator) SuperviseStep(ctx context.Context, step *supervision.SupervisedStep) (*supervision.SupervisedStep, error) {
	return d.coord.SuperviseStep(ctx, step)
}

func (d *defaultSupervisionCoordinator) BatchSupervise(ctx context.Context, steps []*supervision.SupervisedStep) ([]*supervision.SupervisedStep, error) {
	return d.coord.BatchSupervise(ctx, steps)
}

func (d *defaultSupervisionCoordinator) SelectBestPath(tree *supervision.ThoughtTree) *supervision.PathScore {
	return d.coord.SelectBestPath(tree)
}

func (d *defaultSupervisionCoordinator) RankPaths(tree *supervision.ThoughtTree) []*supervision.PathScore {
	return d.coord.RankPaths(tree)
}

func (d *defaultSupervisionCoordinator) GetActiveTree(requestID string) *supervision.ThoughtTree {
	return d.coord.GetActiveTree(requestID)
}

func (d *defaultSupervisionCoordinator) CompleteTree(requestID string) *supervision.ThoughtTree {
	return d.coord.CompleteTree(requestID)
}

func (d *defaultSupervisionCoordinator) PruneTree(tree *supervision.ThoughtTree) int {
	return d.coord.PruneTree(tree)
}

func (d *defaultSupervisionCoordinator) GetStats() map[string]interface{} {
	return d.coord.GetStats()
}

func (d *defaultSupervisionCoordinator) Config() *supervision.Config {
	return d.coord.Config()
}

// noopSupervisionCoordinator provides a disabled supervisor.
type noopSupervisionCoordinator struct{}

func (n *noopSupervisionCoordinator) Enabled() bool {
	return false
}

func (n *noopSupervisionCoordinator) SetEnabled(enabled bool) {}

func (n *noopSupervisionCoordinator) EvaluateThought(ctx context.Context, content string) (*supervision.GuardianResult, error) {
	return &supervision.GuardianResult{
		Approved:   true,
		Score:      1.0,
		Confidence: 1.0,
		Reason:     "Supervision disabled",
	}, nil
}

func (n *noopSupervisionCoordinator) BuildTree(ctx context.Context, requestID, query, initialThought string, expander supervision.Expander) (*supervision.ThoughtTree, error) {
	return nil, nil
}

func (n *noopSupervisionCoordinator) SuperviseStep(ctx context.Context, step *supervision.SupervisedStep) (*supervision.SupervisedStep, error) {
	step.Evaluation = &supervision.GuardianResult{
		Approved:   true,
		Score:      1.0,
		Confidence: 1.0,
		Reason:     "Supervision disabled",
	}
	return step, nil
}

func (n *noopSupervisionCoordinator) BatchSupervise(ctx context.Context, steps []*supervision.SupervisedStep) ([]*supervision.SupervisedStep, error) {
	for _, step := range steps {
		step.Evaluation = &supervision.GuardianResult{
			Approved:   true,
			Score:      1.0,
			Confidence: 1.0,
			Reason:     "Supervision disabled",
		}
	}
	return steps, nil
}

func (n *noopSupervisionCoordinator) SelectBestPath(tree *supervision.ThoughtTree) *supervision.PathScore {
	return nil
}

func (n *noopSupervisionCoordinator) RankPaths(tree *supervision.ThoughtTree) []*supervision.PathScore {
	return nil
}

func (n *noopSupervisionCoordinator) GetActiveTree(requestID string) *supervision.ThoughtTree {
	return nil
}

func (n *noopSupervisionCoordinator) CompleteTree(requestID string) *supervision.ThoughtTree {
	return nil
}

func (n *noopSupervisionCoordinator) PruneTree(tree *supervision.ThoughtTree) int {
	return 0
}

func (n *noopSupervisionCoordinator) GetStats() map[string]interface{} {
	return map[string]interface{}{"enabled": false}
}

func (n *noopSupervisionCoordinator) Config() *supervision.Config {
	return nil
}

// WithSupervisionCoordinator sets the supervision coordinator.
func WithSupervisionCoordinator(sc SupervisionCoordinator) Option {
	return func(o *Orchestrator) {
		o.supervision = sc
	}
}

// Supervision returns the supervision coordinator.
func (o *Orchestrator) Supervision() SupervisionCoordinator {
	if o.supervision == nil {
		return NoopSupervisionCoordinator()
	}
	return o.supervision
}
