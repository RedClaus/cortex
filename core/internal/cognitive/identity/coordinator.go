package identity

import (
	"context"
	"sync"
)

// Coordinator is the main entry point for identity management.
// It coordinates creed management, drift detection, validation, and repair.
type Coordinator struct {
	config   *Config
	creed    *CreedManager
	detector *DriftDetector
	guardian *Guardian
	repairer *Repairer
	embedder Embedder

	mu sync.RWMutex
}

// NewCoordinator creates a new identity coordinator.
func NewCoordinator(cfg *Config, embedder Embedder) *Coordinator {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	creed := NewCreedManager(embedder)
	detector := NewDriftDetector(cfg, creed)
	guardian := NewGuardian(creed, embedder, cfg)
	repairer := NewRepairer(creed, cfg)

	return &Coordinator{
		config:   cfg,
		creed:    creed,
		detector: detector,
		guardian: guardian,
		repairer: repairer,
		embedder: embedder,
	}
}

// Initialize sets up the identity system with creed statements.
func (c *Coordinator) Initialize(statements []string, version string) error {
	return c.creed.Initialize(statements, version)
}

// InitializeWithDefaults sets up with the default Cortex creed.
func (c *Coordinator) InitializeWithDefaults() error {
	return c.creed.Initialize(DefaultCortexCreed(), "1.0.0")
}

// InitializeWithCreed sets up with a pre-computed creed.
func (c *Coordinator) InitializeWithCreed(creed *Creed) error {
	return c.creed.InitializeWithEmbeddings(creed)
}

// Enabled returns whether identity checking is enabled.
func (c *Coordinator) Enabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config.Enabled
}

// SetEnabled enables or disables identity checking.
func (c *Coordinator) SetEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config.Enabled = enabled
}

// RecordResponse records a response for drift tracking.
// Call this after each response is generated.
func (c *Coordinator) RecordResponse(ctx context.Context, response string) error {
	if !c.Enabled() {
		return nil
	}

	var embedding []float32
	if c.embedder != nil {
		var err error
		embedding, err = c.embedder.Embed(response)
		if err != nil {
			// Continue without embedding - pattern-based detection still works
			embedding = nil
		}
	}

	c.detector.RecordResponse(response, embedding)
	return nil
}

// RecordResponseWithEmbedding records a response with a pre-computed embedding.
// Use this when you already have the embedding from another source.
func (c *Coordinator) RecordResponseWithEmbedding(response string, embedding []float32) {
	if !c.Enabled() {
		return
	}
	c.detector.RecordResponse(response, embedding)
}

// ShouldCheckDrift returns true if it's time to check for drift.
func (c *Coordinator) ShouldCheckDrift() bool {
	if !c.Enabled() {
		return false
	}
	return c.detector.ShouldCheck()
}

// CheckDrift performs drift detection.
// This is deterministic and completes in <10ms.
func (c *Coordinator) CheckDrift() *DriftAnalysis {
	if !c.Enabled() {
		return &DriftAnalysis{}
	}
	return c.detector.DetectDrift()
}

// ValidateResponse checks if a response is consistent with the creed.
// Returns validation result with any violations identified.
func (c *Coordinator) ValidateResponse(ctx context.Context, response string) (*ValidationResult, error) {
	if !c.Enabled() {
		return &ValidationResult{Valid: true, Similarity: 1.0}, nil
	}

	var embedding []float32
	if c.embedder != nil {
		var err error
		embedding, err = c.embedder.Embed(response)
		if err != nil {
			embedding = nil
		}
	}

	return c.guardian.ValidateResponse(response, embedding), nil
}

// ValidateResponseWithEmbedding validates with a pre-computed embedding.
func (c *Coordinator) ValidateResponseWithEmbedding(response string, embedding []float32) *ValidationResult {
	if !c.Enabled() {
		return &ValidationResult{Valid: true, Similarity: 1.0}
	}
	return c.guardian.ValidateResponse(response, embedding)
}

// QuickValidate performs pattern-only validation (no embedding).
func (c *Coordinator) QuickValidate(response string) *ValidationResult {
	if !c.Enabled() {
		return &ValidationResult{Valid: true, Similarity: 1.0}
	}
	return c.guardian.QuickValidate(response)
}

// GenerateRepairPlan creates a repair plan for detected drift.
func (c *Coordinator) GenerateRepairPlan(analysis *DriftAnalysis) *RepairPlan {
	if !c.Enabled() || analysis == nil {
		return nil
	}

	if analysis.OverallDrift <= c.config.DriftThreshold {
		return nil
	}

	return c.repairer.GenerateRepairPlan(analysis)
}

// ApplyRepair applies the first action from a repair plan to a response.
func (c *Coordinator) ApplyRepair(plan *RepairPlan, response string) string {
	if plan == nil || len(plan.Actions) == 0 {
		return response
	}

	result := c.repairer.ApplyRepairAction(&plan.Actions[0], response)
	c.detector.RecordRepair()
	return result
}

// GetCreed returns the current creed.
func (c *Coordinator) GetCreed() *Creed {
	return c.creed.GetCreed()
}

// GetCreedStatements returns the creed statements.
func (c *Coordinator) GetCreedStatements() []string {
	return c.creed.GetStatements()
}

// GetDriftScore returns the most recent drift score.
func (c *Coordinator) GetDriftScore() float64 {
	return c.detector.GetRecentDriftScore()
}

// GetAverageDrift returns the running average drift score.
func (c *Coordinator) GetAverageDrift() float64 {
	return c.detector.GetAverageDrift()
}

// GetDriftHistory returns recent drift events.
func (c *Coordinator) GetDriftHistory(limit int) []DriftEvent {
	return c.detector.GetDriftHistory(limit)
}

// GetStats returns identity management statistics.
func (c *Coordinator) GetStats() *IdentityStats {
	return c.detector.GetStats()
}

// Config returns the current configuration.
func (c *Coordinator) Config() *Config {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config
}

// FullCheck performs a complete identity check cycle:
// 1. Check if drift detection is due
// 2. Run drift detection
// 3. Generate repair plan if needed
// Returns the analysis and optional repair plan.
func (c *Coordinator) FullCheck() (*DriftAnalysis, *RepairPlan) {
	if !c.Enabled() {
		return &DriftAnalysis{}, nil
	}

	if !c.detector.ShouldCheck() {
		return nil, nil
	}

	analysis := c.detector.DetectDrift()
	if analysis.OverallDrift <= c.config.DriftThreshold {
		return analysis, nil
	}

	plan := c.repairer.GenerateRepairPlan(analysis)
	return analysis, plan
}

// ProcessResponse is a convenience method that:
// 1. Records the response
// 2. Checks for drift if needed
// 3. Returns any repair actions needed
func (c *Coordinator) ProcessResponse(ctx context.Context, response string) (*ValidationResult, *RepairPlan, error) {
	if !c.Enabled() {
		return &ValidationResult{Valid: true, Similarity: 1.0}, nil, nil
	}

	// Validate response
	validation, err := c.ValidateResponse(ctx, response)
	if err != nil {
		return nil, nil, err
	}

	// Record for drift tracking
	if err := c.RecordResponse(ctx, response); err != nil {
		return validation, nil, err
	}

	// Check drift if needed
	var plan *RepairPlan
	if c.detector.ShouldCheck() {
		analysis := c.detector.DetectDrift()
		if analysis.OverallDrift > c.config.DriftThreshold {
			plan = c.repairer.GenerateRepairPlan(analysis)
		}
	}

	return validation, plan, nil
}
