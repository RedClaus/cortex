// Package orchestrator provides the sleep coordinator for CR-020.
// This coordinator handles the sleep cycle self-improvement system,
// allowing Cortex to reflect on interactions during idle periods and
// propose personality/behavior improvements.
package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/normanking/cortex/internal/bus"
	"github.com/normanking/cortex/internal/logging"
	"github.com/normanking/cortex/pkg/brain/sleep"
)

// SleepCoordinator defines the interface for sleep cycle operations.
// CR-020: Sleep Cycle Self-Improvement
type SleepCoordinator interface {
	// RecordInteraction records an interaction for the sleep cycle.
	RecordInteraction()

	// ShouldSleep returns true if conditions are met to enter sleep.
	ShouldSleep() bool

	// EnterSleep initiates a sleep cycle and returns the wake report.
	EnterSleep(ctx context.Context) (*sleep.WakeReport, error)

	// GetPendingProposals returns any pending personality proposals.
	GetPendingProposals() []sleep.PersonalityProposal

	// ApproveProposal approves and applies a personality proposal.
	ApproveProposal(proposalID string) error

	// RejectProposal rejects a personality proposal.
	RejectProposal(proposalID string, feedback string) error

	// ApproveAllSafe approves all safe proposals.
	ApproveAllSafe() (int, error)

	// GetPersonality returns the current personality.
	GetPersonality() (*sleep.Personality, error)

	// GetMode returns the current improvement mode.
	GetMode() sleep.ImprovementMode

	// SetMode updates the improvement mode.
	SetMode(mode sleep.ImprovementMode)

	// GetPersonalityHistory returns available personality backups.
	GetPersonalityHistory() ([]string, error)

	// RevertToHistory reverts personality to a historical version.
	RevertToHistory(filename string) error

	// Enabled returns whether sleep cycle is enabled.
	Enabled() bool

	// IsSleeping returns whether a sleep cycle is in progress.
	IsSleeping() bool

	// GetStats returns sleep cycle statistics.
	GetStats() SleepStats
}

// SleepStats holds statistics about sleep cycles.
type SleepStats struct {
	InteractionCount int
	LastSleep        time.Time
	IdleDuration     time.Duration
	Mode             sleep.ImprovementMode
	PendingProposals int
	IsSleeping       bool
}

// SleepCoordinatorConfig holds configuration for the sleep coordinator.
type SleepCoordinatorConfig struct {
	// PersonalityPath is the directory for personality files.
	PersonalityPath string

	// MemoryStore provides access to interaction history.
	MemoryStore sleep.MemoryStore

	// EventBus for publishing sleep events.
	EventBus *bus.EventBus

	// Config holds sleep cycle configuration.
	Config sleep.SleepConfig

	// Enabled controls whether sleep cycle is active.
	Enabled bool
}

// sleepCoordinatorImpl is the concrete implementation.
type sleepCoordinatorImpl struct {
	manager  *sleep.SleepManager
	eventBus *bus.EventBus
	enabled  bool
}

// NewSleepCoordinator creates a new sleep coordinator.
func NewSleepCoordinator(cfg *SleepCoordinatorConfig) SleepCoordinator {
	log := logging.Global()

	if cfg == nil || !cfg.Enabled {
		log.Info("[Sleep] Sleep coordinator disabled")
		return &sleepCoordinatorImpl{enabled: false}
	}

	// Determine personality path
	personalityPath := cfg.PersonalityPath
	if personalityPath == "" {
		personalityPath = "personality"
	}

	// Create personality store
	personalityStore := sleep.NewPersonalityStore(personalityPath)

	// Use default config if not provided
	sleepConfig := cfg.Config
	if sleepConfig.MinInteractions == 0 {
		sleepConfig = sleep.DefaultSleepConfig()
	}

	// Create memory store adapter if not provided
	memoryStore := cfg.MemoryStore
	if memoryStore == nil {
		memoryStore = &noopMemoryStore{}
	}

	// Create sleep manager
	// The internal/logging.Logger satisfies sleep.Logger interface
	manager := sleep.NewSleepManager(
		sleepConfig,
		personalityStore,
		memoryStore,
		log,
	)

	log.Info("[Sleep] Sleep coordinator initialized (mode=%s, idle_timeout=%v)",
		sleepConfig.Mode.String(), sleepConfig.IdleTimeout)

	return &sleepCoordinatorImpl{
		manager:  manager,
		eventBus: cfg.EventBus,
		enabled:  true,
	}
}

// Enabled returns whether sleep cycle is enabled.
func (sc *sleepCoordinatorImpl) Enabled() bool {
	return sc.enabled && sc.manager != nil
}

// RecordInteraction records an interaction for the sleep cycle.
func (sc *sleepCoordinatorImpl) RecordInteraction() {
	if !sc.enabled || sc.manager == nil {
		return
	}
	sc.manager.RecordInteraction()
}

// ShouldSleep returns true if conditions are met to enter sleep.
func (sc *sleepCoordinatorImpl) ShouldSleep() bool {
	if !sc.enabled || sc.manager == nil {
		return false
	}
	return sc.manager.ShouldSleep()
}

// EnterSleep initiates a sleep cycle and returns the wake report.
func (sc *sleepCoordinatorImpl) EnterSleep(ctx context.Context) (*sleep.WakeReport, error) {
	log := logging.Global()

	if !sc.enabled || sc.manager == nil {
		return nil, fmt.Errorf("sleep coordinator not enabled")
	}

	log.Info("[Sleep] Entering sleep cycle...")

	// Publish start event
	sc.publishEvent("sleep_started", map[string]any{
		"interaction_count": sc.manager.GetInteractionCount(),
		"mode":              sc.manager.GetMode().String(),
	})

	report, err := sc.manager.EnterSleep(ctx)
	if err != nil {
		sc.publishEvent("sleep_failed", map[string]any{
			"error": err.Error(),
		})
		return nil, err
	}

	log.Info("[Sleep] Sleep cycle complete: %d interactions reviewed, %d proposals generated",
		report.InteractionsReviewed, len(report.Proposals))

	// Publish completion event
	sc.publishEvent("sleep_completed", map[string]any{
		"duration_ms":           report.SleepDuration.Milliseconds(),
		"interactions_reviewed": report.InteractionsReviewed,
		"patterns_found":        report.PatternsFound,
		"insights":              len(report.Insights),
		"proposals":             len(report.Proposals),
		"auto_applied":          len(report.AutoApplied),
		"pending_approval":      len(report.PendingApproval),
	})

	return report, nil
}

// GetPendingProposals returns any pending personality proposals.
func (sc *sleepCoordinatorImpl) GetPendingProposals() []sleep.PersonalityProposal {
	if !sc.enabled || sc.manager == nil {
		return nil
	}

	report := sc.manager.GetPendingWakeReport()
	if report == nil {
		return nil
	}

	return report.PendingApproval
}

// ApproveProposal approves and applies a personality proposal.
func (sc *sleepCoordinatorImpl) ApproveProposal(proposalID string) error {
	log := logging.Global()

	if !sc.enabled || sc.manager == nil {
		return fmt.Errorf("sleep coordinator not enabled")
	}

	if err := sc.manager.ApplyProposal(proposalID); err != nil {
		return err
	}

	log.Info("[Sleep] Proposal approved: %s", proposalID)

	sc.publishEvent("proposal_approved", map[string]any{
		"proposal_id": proposalID,
	})

	return nil
}

// RejectProposal rejects a personality proposal.
func (sc *sleepCoordinatorImpl) RejectProposal(proposalID string, feedback string) error {
	log := logging.Global()

	if !sc.enabled || sc.manager == nil {
		return fmt.Errorf("sleep coordinator not enabled")
	}

	if err := sc.manager.RejectProposal(proposalID, feedback); err != nil {
		return err
	}

	log.Info("[Sleep] Proposal rejected: %s (feedback: %s)", proposalID, feedback)

	sc.publishEvent("proposal_rejected", map[string]any{
		"proposal_id": proposalID,
		"feedback":    feedback,
	})

	return nil
}

// ApproveAllSafe approves all safe proposals.
func (sc *sleepCoordinatorImpl) ApproveAllSafe() (int, error) {
	log := logging.Global()

	if !sc.enabled || sc.manager == nil {
		return 0, fmt.Errorf("sleep coordinator not enabled")
	}

	count, err := sc.manager.ApproveAllSafe()
	if err != nil {
		return 0, err
	}

	log.Info("[Sleep] Approved %d safe proposals", count)

	sc.publishEvent("proposals_bulk_approved", map[string]any{
		"count": count,
	})

	return count, nil
}

// GetPersonality returns the current personality.
func (sc *sleepCoordinatorImpl) GetPersonality() (*sleep.Personality, error) {
	if !sc.enabled || sc.manager == nil {
		return nil, fmt.Errorf("sleep coordinator not enabled")
	}

	return sc.manager.GetPersonality()
}

// GetMode returns the current improvement mode.
func (sc *sleepCoordinatorImpl) GetMode() sleep.ImprovementMode {
	if !sc.enabled || sc.manager == nil {
		return sleep.ImprovementOff
	}

	return sc.manager.GetMode()
}

// SetMode updates the improvement mode.
func (sc *sleepCoordinatorImpl) SetMode(mode sleep.ImprovementMode) {
	log := logging.Global()

	if !sc.enabled || sc.manager == nil {
		return
	}

	sc.manager.SetMode(mode)

	log.Info("[Sleep] Improvement mode set to: %s", mode.String())

	sc.publishEvent("mode_changed", map[string]any{
		"mode": mode.String(),
	})
}

// GetPersonalityHistory returns available personality backups.
func (sc *sleepCoordinatorImpl) GetPersonalityHistory() ([]string, error) {
	if !sc.enabled || sc.manager == nil {
		return nil, fmt.Errorf("sleep coordinator not enabled")
	}

	return sc.manager.GetPersonalityStore().GetChangeHistory()
}

// RevertToHistory reverts personality to a historical version.
func (sc *sleepCoordinatorImpl) RevertToHistory(filename string) error {
	log := logging.Global()

	if !sc.enabled || sc.manager == nil {
		return fmt.Errorf("sleep coordinator not enabled")
	}

	// Validate filename to prevent path traversal
	if filepath.Base(filename) != filename {
		return fmt.Errorf("invalid filename")
	}

	if err := sc.manager.RevertChange(filename); err != nil {
		return err
	}

	log.Info("[Sleep] Personality reverted to: %s", filename)

	sc.publishEvent("personality_reverted", map[string]any{
		"filename": filename,
	})

	return nil
}

// IsSleeping returns whether a sleep cycle is in progress.
func (sc *sleepCoordinatorImpl) IsSleeping() bool {
	if !sc.enabled || sc.manager == nil {
		return false
	}

	return sc.manager.IsSleeping()
}

// GetStats returns sleep cycle statistics.
func (sc *sleepCoordinatorImpl) GetStats() SleepStats {
	if !sc.enabled || sc.manager == nil {
		return SleepStats{Mode: sleep.ImprovementOff}
	}

	pendingCount := 0
	if report := sc.manager.GetPendingWakeReport(); report != nil {
		pendingCount = len(report.PendingApproval)
	}

	return SleepStats{
		InteractionCount: sc.manager.GetInteractionCount(),
		LastSleep:        sc.manager.GetLastSleep(),
		IdleDuration:     sc.manager.GetIdleDuration(),
		Mode:             sc.manager.GetMode(),
		PendingProposals: pendingCount,
		IsSleeping:       sc.manager.IsSleeping(),
	}
}

func (sc *sleepCoordinatorImpl) publishEvent(eventType string, data map[string]any) {
	if sc.eventBus == nil {
		return
	}
	sc.eventBus.Publish(bus.NewSleepEvent(eventType, data))
}

// noopMemoryStore is a no-op memory store for when no real store is provided.
type noopMemoryStore struct{}

func (n *noopMemoryStore) GetInteractionsSince(since time.Time) ([]sleep.Interaction, error) {
	return []sleep.Interaction{}, nil
}

// ============================================================================
// ORCHESTRATOR INTEGRATION
// ============================================================================

// WithSleepCoordinator sets the sleep coordinator.
// CR-020: Sleep Cycle Self-Improvement
func WithSleepCoordinator(sc SleepCoordinator) Option {
	return func(o *Orchestrator) {
		o.sleep = sc
	}
}

// Sleep returns the sleep coordinator.
func (o *Orchestrator) Sleep() SleepCoordinator {
	return o.sleep
}

// RecordInteractionForSleep records an interaction for the sleep cycle.
// This should be called after each user interaction.
func (o *Orchestrator) RecordInteractionForSleep() {
	if o.sleep != nil && o.sleep.Enabled() {
		o.sleep.RecordInteraction()
	}
}

// CheckSleepConditions checks if sleep should be triggered and returns wake report if so.
func (o *Orchestrator) CheckSleepConditions(ctx context.Context) (*sleep.WakeReport, error) {
	if o.sleep == nil || !o.sleep.Enabled() {
		return nil, nil
	}

	if !o.sleep.ShouldSleep() {
		return nil, nil
	}

	return o.sleep.EnterSleep(ctx)
}

// EnterSleep initiates a sleep cycle and returns the wake report.
// CR-020: Sleep Cycle Self-Improvement
// This is exposed via the Interface for TUI /sleep command.
func (o *Orchestrator) EnterSleep(ctx context.Context) (*sleep.WakeReport, error) {
	if o.sleep == nil || !o.sleep.Enabled() {
		return nil, fmt.Errorf("sleep coordinator not enabled")
	}
	return o.sleep.EnterSleep(ctx)
}

// ═══════════════════════════════════════════════════════════════════════════════
// PERSONA METHODS
// ═══════════════════════════════════════════════════════════════════════════════

// GetPersona returns the current active persona ID.
func (o *Orchestrator) GetPersona() string {
	// If we have a persona coordinator, use it
	if o.persona != nil {
		if p := o.persona.GetActivePersona(); p != nil {
			return p.ID
		}
	}

	// Fallback to stored ID
	o.mu.RLock()
	defer o.mu.RUnlock()
	if o.activePersonaID != "" {
		return o.activePersonaID
	}
	return "cortex" // Default persona
}
