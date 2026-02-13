package orchestrator

import (
	"context"

	"github.com/normanking/cortex/internal/cognitive/identity"
)

// IdentityCoordinator defines the interface for identity management.
// It wraps the identity.Coordinator to provide orchestrator-level integration.
type IdentityCoordinator interface {
	// Enabled returns whether identity checking is enabled.
	Enabled() bool

	// SetEnabled enables or disables identity checking.
	SetEnabled(enabled bool)

	// Initialize sets up the identity system with creed statements.
	Initialize(statements []string, version string) error

	// InitializeWithDefaults sets up with the default Cortex creed.
	InitializeWithDefaults() error

	// RecordResponse records a response for drift tracking.
	RecordResponse(ctx context.Context, response string) error

	// RecordResponseWithEmbedding records a response with a pre-computed embedding.
	RecordResponseWithEmbedding(response string, embedding []float32)

	// ValidateResponse checks if a response is consistent with the creed.
	ValidateResponse(ctx context.Context, response string) (*identity.ValidationResult, error)

	// QuickValidate performs pattern-only validation (no embedding).
	QuickValidate(response string) *identity.ValidationResult

	// ShouldCheckDrift returns true if it's time to check for drift.
	ShouldCheckDrift() bool

	// CheckDrift performs drift detection.
	CheckDrift() *identity.DriftAnalysis

	// GenerateRepairPlan creates a repair plan for detected drift.
	GenerateRepairPlan(analysis *identity.DriftAnalysis) *identity.RepairPlan

	// ApplyRepair applies the first action from a repair plan to a response.
	ApplyRepair(plan *identity.RepairPlan, response string) string

	// ProcessResponse validates, records, and checks drift in one call.
	ProcessResponse(ctx context.Context, response string) (*identity.ValidationResult, *identity.RepairPlan, error)

	// GetCreed returns the current creed.
	GetCreed() *identity.Creed

	// GetCreedStatements returns the creed statements.
	GetCreedStatements() []string

	// GetDriftScore returns the most recent drift score.
	GetDriftScore() float64

	// GetAverageDrift returns the running average drift score.
	GetAverageDrift() float64

	// GetDriftHistory returns recent drift events.
	GetDriftHistory(limit int) []identity.DriftEvent

	// GetStats returns identity management statistics.
	GetStats() *identity.IdentityStats

	// Config returns the current configuration.
	Config() *identity.Config
}

// IdentityCoordinatorConfig configures the identity coordinator.
type IdentityCoordinatorConfig struct {
	// Enabled controls whether identity checking is active
	Enabled bool

	// DriftThreshold is the maximum allowed drift score (0-1)
	DriftThreshold float64

	// CheckInterval is the number of responses between drift checks
	CheckInterval int

	// AutoRepair enables automatic drift correction
	AutoRepair bool

	// WindowSize is the number of recent responses to analyze
	WindowSize int

	// Embedder for computing response embeddings
	Embedder identity.Embedder
}

// DefaultIdentityCoordinatorConfig returns sensible defaults.
func DefaultIdentityCoordinatorConfig() *IdentityCoordinatorConfig {
	return &IdentityCoordinatorConfig{
		Enabled:        true,
		DriftThreshold: 0.3,
		CheckInterval:  100,
		AutoRepair:     false,
		WindowSize:     10,
	}
}

// defaultIdentityCoordinator wraps the identity.Coordinator.
type defaultIdentityCoordinator struct {
	coord *identity.Coordinator
}

// NewIdentityCoordinator creates a new identity coordinator.
func NewIdentityCoordinator(cfg *IdentityCoordinatorConfig) IdentityCoordinator {
	if cfg == nil {
		cfg = DefaultIdentityCoordinatorConfig()
	}

	// Convert to internal config
	idCfg := &identity.Config{
		Enabled:        cfg.Enabled,
		DriftThreshold: cfg.DriftThreshold,
		CheckInterval:  cfg.CheckInterval,
		AutoRepair:     cfg.AutoRepair,
		WindowSize:     cfg.WindowSize,
	}

	coord := identity.NewCoordinator(idCfg, cfg.Embedder)
	return &defaultIdentityCoordinator{coord: coord}
}

// NoopIdentityCoordinator returns a disabled identity coordinator.
func NoopIdentityCoordinator() IdentityCoordinator {
	return &noopIdentityCoordinator{}
}

// Compile-time interface verification
var _ IdentityCoordinator = (*defaultIdentityCoordinator)(nil)
var _ IdentityCoordinator = (*noopIdentityCoordinator)(nil)

// defaultIdentityCoordinator implementation

func (d *defaultIdentityCoordinator) Enabled() bool {
	return d.coord.Enabled()
}

func (d *defaultIdentityCoordinator) SetEnabled(enabled bool) {
	d.coord.SetEnabled(enabled)
}

func (d *defaultIdentityCoordinator) Initialize(statements []string, version string) error {
	return d.coord.Initialize(statements, version)
}

func (d *defaultIdentityCoordinator) InitializeWithDefaults() error {
	return d.coord.InitializeWithDefaults()
}

func (d *defaultIdentityCoordinator) RecordResponse(ctx context.Context, response string) error {
	return d.coord.RecordResponse(ctx, response)
}

func (d *defaultIdentityCoordinator) RecordResponseWithEmbedding(response string, embedding []float32) {
	d.coord.RecordResponseWithEmbedding(response, embedding)
}

func (d *defaultIdentityCoordinator) ValidateResponse(ctx context.Context, response string) (*identity.ValidationResult, error) {
	return d.coord.ValidateResponse(ctx, response)
}

func (d *defaultIdentityCoordinator) QuickValidate(response string) *identity.ValidationResult {
	return d.coord.QuickValidate(response)
}

func (d *defaultIdentityCoordinator) ShouldCheckDrift() bool {
	return d.coord.ShouldCheckDrift()
}

func (d *defaultIdentityCoordinator) CheckDrift() *identity.DriftAnalysis {
	return d.coord.CheckDrift()
}

func (d *defaultIdentityCoordinator) GenerateRepairPlan(analysis *identity.DriftAnalysis) *identity.RepairPlan {
	return d.coord.GenerateRepairPlan(analysis)
}

func (d *defaultIdentityCoordinator) ApplyRepair(plan *identity.RepairPlan, response string) string {
	return d.coord.ApplyRepair(plan, response)
}

func (d *defaultIdentityCoordinator) ProcessResponse(ctx context.Context, response string) (*identity.ValidationResult, *identity.RepairPlan, error) {
	return d.coord.ProcessResponse(ctx, response)
}

func (d *defaultIdentityCoordinator) GetCreed() *identity.Creed {
	return d.coord.GetCreed()
}

func (d *defaultIdentityCoordinator) GetCreedStatements() []string {
	return d.coord.GetCreedStatements()
}

func (d *defaultIdentityCoordinator) GetDriftScore() float64 {
	return d.coord.GetDriftScore()
}

func (d *defaultIdentityCoordinator) GetAverageDrift() float64 {
	return d.coord.GetAverageDrift()
}

func (d *defaultIdentityCoordinator) GetDriftHistory(limit int) []identity.DriftEvent {
	return d.coord.GetDriftHistory(limit)
}

func (d *defaultIdentityCoordinator) GetStats() *identity.IdentityStats {
	return d.coord.GetStats()
}

func (d *defaultIdentityCoordinator) Config() *identity.Config {
	return d.coord.Config()
}

// noopIdentityCoordinator provides a disabled identity coordinator.
type noopIdentityCoordinator struct{}

func (n *noopIdentityCoordinator) Enabled() bool {
	return false
}

func (n *noopIdentityCoordinator) SetEnabled(enabled bool) {}

func (n *noopIdentityCoordinator) Initialize(statements []string, version string) error {
	return nil
}

func (n *noopIdentityCoordinator) InitializeWithDefaults() error {
	return nil
}

func (n *noopIdentityCoordinator) RecordResponse(ctx context.Context, response string) error {
	return nil
}

func (n *noopIdentityCoordinator) RecordResponseWithEmbedding(response string, embedding []float32) {}

func (n *noopIdentityCoordinator) ValidateResponse(ctx context.Context, response string) (*identity.ValidationResult, error) {
	return &identity.ValidationResult{Valid: true, Similarity: 1.0}, nil
}

func (n *noopIdentityCoordinator) QuickValidate(response string) *identity.ValidationResult {
	return &identity.ValidationResult{Valid: true, Similarity: 1.0}
}

func (n *noopIdentityCoordinator) ShouldCheckDrift() bool {
	return false
}

func (n *noopIdentityCoordinator) CheckDrift() *identity.DriftAnalysis {
	return &identity.DriftAnalysis{}
}

func (n *noopIdentityCoordinator) GenerateRepairPlan(analysis *identity.DriftAnalysis) *identity.RepairPlan {
	return nil
}

func (n *noopIdentityCoordinator) ApplyRepair(plan *identity.RepairPlan, response string) string {
	return response
}

func (n *noopIdentityCoordinator) ProcessResponse(ctx context.Context, response string) (*identity.ValidationResult, *identity.RepairPlan, error) {
	return &identity.ValidationResult{Valid: true, Similarity: 1.0}, nil, nil
}

func (n *noopIdentityCoordinator) GetCreed() *identity.Creed {
	return nil
}

func (n *noopIdentityCoordinator) GetCreedStatements() []string {
	return nil
}

func (n *noopIdentityCoordinator) GetDriftScore() float64 {
	return 0.0
}

func (n *noopIdentityCoordinator) GetAverageDrift() float64 {
	return 0.0
}

func (n *noopIdentityCoordinator) GetDriftHistory(limit int) []identity.DriftEvent {
	return nil
}

func (n *noopIdentityCoordinator) GetStats() *identity.IdentityStats {
	return &identity.IdentityStats{Enabled: false}
}

func (n *noopIdentityCoordinator) Config() *identity.Config {
	return nil
}

// WithIdentityCoordinator sets the identity coordinator.
func WithIdentityCoordinator(ic IdentityCoordinator) Option {
	return func(o *Orchestrator) {
		o.identity = ic
	}
}

// Identity returns the identity coordinator.
func (o *Orchestrator) Identity() IdentityCoordinator {
	if o.identity == nil {
		return NoopIdentityCoordinator()
	}
	return o.identity
}
