package sleep

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// SleepConfig holds configuration for the sleep cycle.
type SleepConfig struct {
	Mode            ImprovementMode `yaml:"mode"`
	IdleTimeout     time.Duration   `yaml:"idle_timeout"`
	Schedule        string          `yaml:"schedule"` // Cron expression
	MinInteractions int             `yaml:"min_interactions"`

	// DMN Worker configuration (optional, cold-path learning)
	DMN DMNConfig `yaml:"dmn"`
}

// DefaultSleepConfig returns sensible defaults.
func DefaultSleepConfig() SleepConfig {
	return SleepConfig{
		Mode:            ImprovementSupervised,
		IdleTimeout:     30 * time.Minute,
		Schedule:        "0 3 * * *", // 3 AM daily
		MinInteractions: 10,
		DMN:             DefaultDMNConfig(),
	}
}

// Logger interface for dependency injection.
// This allows the sleep package to work with any logging implementation.
type Logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
}

// noopLogger is a no-op logger for when no logger is provided.
type noopLogger struct{}

func (n *noopLogger) Debug(format string, args ...interface{}) {}
func (n *noopLogger) Info(format string, args ...interface{})  {}
func (n *noopLogger) Warn(format string, args ...interface{})  {}
func (n *noopLogger) Error(format string, args ...interface{}) {}

// SleepManager orchestrates the sleep cycle and self-improvement.
type SleepManager struct {
	mu          sync.RWMutex
	config      SleepConfig
	personality *PersonalityStore
	memory      MemoryStore
	log         Logger

	// DMN Worker for cold-path learning (optional)
	dmnWorker *DMNWorker

	// State
	lastSleep       time.Time
	isSleeping      bool
	pendingWake     *WakeReport
	interactionCnt  int
	lastInteraction time.Time

	// DMN results from last sleep cycle
	lastDMNResult *DMNResult
}

// NewSleepManager creates a new sleep manager.
func NewSleepManager(config SleepConfig, personality *PersonalityStore, memory MemoryStore, log Logger) *SleepManager {
	if log == nil {
		log = &noopLogger{}
	}
	return &SleepManager{
		config:          config,
		personality:     personality,
		memory:          memory,
		log:             log,
		lastSleep:       time.Now(),
		lastInteraction: time.Now(),
	}
}

// ShouldSleep checks if conditions are met to enter sleep.
func (sm *SleepManager) ShouldSleep() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.config.Mode == ImprovementOff {
		return false
	}

	if sm.isSleeping {
		return false
	}

	if sm.interactionCnt < sm.config.MinInteractions {
		return false
	}

	idleDuration := time.Since(sm.lastInteraction)
	return idleDuration >= sm.config.IdleTimeout
}

// RecordInteraction increments the interaction counter.
func (sm *SleepManager) RecordInteraction() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.interactionCnt++
	sm.lastInteraction = time.Now()
}

// GetInteractionCount returns the current interaction count.
func (sm *SleepManager) GetInteractionCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.interactionCnt
}

// GetIdleDuration returns how long since the last interaction.
func (sm *SleepManager) GetIdleDuration() time.Duration {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return time.Since(sm.lastInteraction)
}

// EnterSleep initiates the sleep cycle.
// BRAIN AUDIT FIX: Added timeout protection and proper context usage.
func (sm *SleepManager) EnterSleep(ctx context.Context) (*WakeReport, error) {
	sm.mu.Lock()
	if sm.isSleeping {
		sm.mu.Unlock()
		return nil, ErrAlreadySleeping
	}
	sm.isSleeping = true
	sm.mu.Unlock()

	defer func() {
		sm.mu.Lock()
		sm.isSleeping = false
		sm.lastSleep = time.Now()
		sm.interactionCnt = 0
		sm.mu.Unlock()
	}()

	// BRAIN AUDIT FIX: Add timeout to prevent indefinite blocking
	// Sleep cycle should complete within 10 minutes max
	sleepCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	sm.log.Info("[Sleep] Entering sleep cycle")
	startTime := time.Now()

	// Phase 1: Consolidation (N3 "deep sleep")
	sm.log.Debug("[Sleep] Phase 1: Consolidating memories")
	consolidated, err := sm.consolidateMemories(sleepCtx)
	if err != nil {
		if sleepCtx.Err() == context.DeadlineExceeded {
			sm.log.Warn("[Sleep] Consolidation timed out after %v", time.Since(startTime))
			return nil, fmt.Errorf("sleep cycle timed out during consolidation")
		}
		if sleepCtx.Err() == context.Canceled {
			sm.log.Info("[Sleep] Sleep cycle cancelled during consolidation")
			return nil, ctx.Err()
		}
		sm.log.Error("[Sleep] Consolidation failed: %v", err)
		return nil, fmt.Errorf("consolidation failed: %w", err)
	}

	// Phase 2: Reflection (REM "dreaming")
	sm.log.Debug("[Sleep] Phase 2: Reflecting on patterns")
	insights, err := sm.reflectOnPatterns(sleepCtx, consolidated)
	if err != nil {
		if sleepCtx.Err() != nil {
			sm.log.Warn("[Sleep] Reflection interrupted: %v", sleepCtx.Err())
			return nil, sleepCtx.Err()
		}
		sm.log.Error("[Sleep] Reflection failed: %v", err)
		return nil, fmt.Errorf("reflection failed: %w", err)
	}

	// Phase 3: Proposal Generation
	sm.log.Debug("[Sleep] Phase 3: Generating proposals")
	proposals, err := sm.generateProposals(sleepCtx, insights)
	if err != nil {
		if sleepCtx.Err() != nil {
			sm.log.Warn("[Sleep] Proposal generation interrupted: %v", sleepCtx.Err())
			return nil, sleepCtx.Err()
		}
		sm.log.Error("[Sleep] Proposal generation failed: %v", err)
		return nil, fmt.Errorf("proposal generation failed: %w", err)
	}

	// Phase 4: DMN Worker (Cold-path learning - lower priority)
	// This phase runs outcome aggregation and tier promotion during sleep
	var dmnResult *DMNResult
	if sm.dmnWorker != nil {
		sm.log.Debug("[Sleep] Phase 4: Running DMN Worker (cold-path learning)")
		dmnResult, err = sm.dmnWorker.Run(sleepCtx)
		if err != nil {
			if sleepCtx.Err() != nil {
				sm.log.Warn("[Sleep] DMN Worker interrupted: %v", sleepCtx.Err())
				// DMN is lower priority, continue with report generation
			} else {
				sm.log.Warn("[Sleep] DMN Worker failed (non-fatal): %v", err)
			}
			// DMN failures are non-fatal - continue with the sleep cycle
		} else {
			sm.log.Debug("[Sleep] DMN Worker complete: task_types=%d, memories_promoted=%d",
				dmnResult.TaskTypesProcessed, dmnResult.MemoriesPromoted)
		}

		// Store DMN result
		sm.mu.Lock()
		sm.lastDMNResult = dmnResult
		sm.mu.Unlock()
	}

	report := sm.prepareWakeReport(consolidated, insights, proposals, dmnResult, time.Since(startTime))

	sm.mu.Lock()
	sm.pendingWake = report
	sm.mu.Unlock()

	// Update personality last sleep time
	p, err := sm.personality.Load()
	if err == nil {
		p.LastSleepCycle = time.Now()
		_ = sm.personality.Save(p)
	}

	sm.log.Info("[Sleep] Sleep cycle complete: interactions=%d, insights=%d, proposals=%d, auto_applied=%d, pending=%d, duration=%v",
		report.InteractionsReviewed,
		len(report.Insights),
		len(report.Proposals),
		len(report.AutoApplied),
		len(report.PendingApproval),
		report.SleepDuration)

	return report, nil
}

// prepareWakeReport creates the final wake report.
func (sm *SleepManager) prepareWakeReport(
	consolidated *ConsolidationResult,
	insights []ReflectionInsight,
	proposals []PersonalityProposal,
	dmnResult *DMNResult,
	duration time.Duration,
) *WakeReport {
	report := &WakeReport{
		SleepDuration:        duration,
		InteractionsReviewed: consolidated.InteractionCount,
		PatternsFound:        len(consolidated.Patterns),
		Insights:             insights,
		Proposals:            proposals,
		AutoApplied:          []PersonalityProposal{},
		PendingApproval:      []PersonalityProposal{},
		DMNResult:            dmnResult,
	}

	switch sm.config.Mode {
	case ImprovementAuto:
		// Auto-apply safe changes with high confidence
		for _, p := range proposals {
			if p.RiskLevel == RiskSafe && p.Confidence > 0.8 {
				if err := sm.applyProposal(p); err != nil {
					sm.log.Warn("[Sleep] Failed to auto-apply proposal %s: %v", p.ID, err)
					report.PendingApproval = append(report.PendingApproval, p)
				} else {
					report.AutoApplied = append(report.AutoApplied, p)
				}
			} else {
				report.PendingApproval = append(report.PendingApproval, p)
			}
		}

	case ImprovementSupervised:
		// Queue all for user approval
		report.PendingApproval = proposals
	}

	return report
}

// GetMode returns the current improvement mode.
func (sm *SleepManager) GetMode() ImprovementMode {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.config.Mode
}

// SetMode updates the improvement mode.
func (sm *SleepManager) SetMode(mode ImprovementMode) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.config.Mode = mode
	sm.log.Info("[Sleep] Improvement mode updated to: %s", mode.String())
}

// GetPendingWakeReport returns any pending wake report.
func (sm *SleepManager) GetPendingWakeReport() *WakeReport {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.pendingWake
}

// ClearPendingWakeReport clears the pending wake report.
func (sm *SleepManager) ClearPendingWakeReport() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.pendingWake = nil
}

// IsSleeping returns whether a sleep cycle is in progress.
func (sm *SleepManager) IsSleeping() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.isSleeping
}

// GetLastSleep returns when the last sleep cycle occurred.
func (sm *SleepManager) GetLastSleep() time.Time {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.lastSleep
}

// GetPersonality returns the current personality.
func (sm *SleepManager) GetPersonality() (*Personality, error) {
	return sm.personality.Load()
}

// GetPersonalityStore returns the personality store.
func (sm *SleepManager) GetPersonalityStore() *PersonalityStore {
	return sm.personality
}

// SetDMNWorker sets the DMN Worker for cold-path learning during sleep cycles.
// This is optional - if not set, DMN tasks will be skipped.
func (sm *SleepManager) SetDMNWorker(worker *DMNWorker) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.dmnWorker = worker
}

// GetDMNWorker returns the current DMN Worker, if set.
func (sm *SleepManager) GetDMNWorker() *DMNWorker {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.dmnWorker
}

// GetLastDMNResult returns the results from the last DMN Worker cycle.
func (sm *SleepManager) GetLastDMNResult() *DMNResult {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.lastDMNResult
}
