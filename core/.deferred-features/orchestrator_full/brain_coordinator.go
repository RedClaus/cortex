package orchestrator

import (
	"context"
	"fmt"
	"sync"

	brainadapter "github.com/normanking/cortex/internal/brain"
	"github.com/normanking/cortex/internal/cognitive/router"
	"github.com/normanking/cortex/internal/llm"
	"github.com/normanking/cortex/internal/logging"
	"github.com/normanking/cortex/internal/registrar"
	"github.com/normanking/cortex/pkg/brain"
)

type BrainSystem interface {
	Process(ctx context.Context, input string) (*brain.ExecutionResult, error)
	GetMetrics() brain.SystemMetrics
	ListLobes() []brain.LobeID
	Enabled() bool
	SetEnabled(enabled bool)
	Stats() *BrainStats
}

type BrainStats struct {
	Enabled        bool     `json:"enabled"`
	LobeCount      int      `json:"lobe_count"`
	ProcessedCount int64    `json:"processed_count"`
	AvailableLobes []string `json:"available_lobes"`
}

type BrainCoordinatorConfig struct {
	LLMProvider  llm.Provider
	MemorySystem MemorySystem
	UserID       string
	Enabled      bool
	Embedder     router.Embedder
	Registrar    *registrar.Registrar // CR-027: For lobe capability registration
}

type BrainCoordinator struct {
	executive *brain.Executive
	config    *BrainCoordinatorConfig
	log       *logging.Logger

	mu             sync.RWMutex
	enabled        bool
	processedCount int64
}

func NewBrainCoordinator(cfg *BrainCoordinatorConfig) *BrainCoordinator {
	if cfg == nil {
		cfg = &BrainCoordinatorConfig{}
	}

	bc := &BrainCoordinator{
		config:  cfg,
		enabled: cfg.Enabled,
		log:     logging.Global(),
	}

	bc.log.Info("[BrainCoordinator] Creating coordinator enabled=%v", cfg.Enabled)

	if cfg.LLMProvider != nil {
		bc.log.Debug("[BrainCoordinator] Initializing Brain Executive")
		bc.executive = brainadapter.NewExecutive(brainadapter.FactoryConfig{
			LLMProvider:  cfg.LLMProvider,
			MemorySystem: cfg.MemorySystem,
			UserID:       cfg.UserID,
			Embedder:     cfg.Embedder,
		})
		bc.executive.Start()
		bc.log.Info("[BrainCoordinator] Brain Executive started")

		// CR-027: Register lobes with capability registrar
		if cfg.Registrar != nil {
			if err := brainadapter.RegisterLobesWithRegistrar(bc.executive, cfg.Registrar); err != nil {
				bc.log.Warn("[BrainCoordinator] Failed to register lobes with registrar: %v", err)
			} else {
				bc.log.Info("[BrainCoordinator] Lobes registered with capability registrar")
			}
		}
	} else {
		bc.log.Warn("[BrainCoordinator] No LLM provider, Brain Executive not initialized")
	}

	return bc
}

var _ BrainSystem = (*BrainCoordinator)(nil)

func (bc *BrainCoordinator) Process(ctx context.Context, input string) (*brain.ExecutionResult, error) {
	bc.mu.RLock()
	enabled := bc.enabled
	exec := bc.executive
	bc.mu.RUnlock()

	bc.log.Debug("[BrainCoordinator] Process called input_length=%d enabled=%v", len(input), enabled)

	if !enabled {
		bc.log.Warn("[BrainCoordinator] Process rejected: brain processing is disabled")
		return nil, fmt.Errorf("brain processing is disabled")
	}

	if exec == nil {
		bc.log.Error("[BrainCoordinator] Process rejected: executive not initialized")
		return nil, fmt.Errorf("brain executive not initialized")
	}

	bc.log.Info("[BrainCoordinator] Processing input through Brain Executive")
	result, err := exec.Process(ctx, input)
	if err != nil {
		bc.log.Error("[BrainCoordinator] Brain process failed: %v", err)
		return nil, fmt.Errorf("brain process: %w", err)
	}

	bc.mu.Lock()
	bc.processedCount++
	count := bc.processedCount
	bc.mu.Unlock()

	bc.log.Info("[BrainCoordinator] Process complete lobe=%s strategy=%s total_requests=%d",
		result.Classification.PrimaryLobe, result.Strategy.Name, count)

	return result, nil
}

func (bc *BrainCoordinator) GetMetrics() brain.SystemMetrics {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	if bc.executive == nil {
		return brain.SystemMetrics{}
	}
	return bc.executive.GetMetrics()
}

func (bc *BrainCoordinator) ListLobes() []brain.LobeID {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	if bc.executive == nil {
		return nil
	}

	registry := bc.executive.Registry()
	if registry == nil {
		return nil
	}

	lobes := registry.All()
	ids := make([]brain.LobeID, len(lobes))
	for i, l := range lobes {
		ids[i] = l.ID()
	}
	return ids
}

func (bc *BrainCoordinator) Enabled() bool {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.enabled
}

func (bc *BrainCoordinator) SetEnabled(enabled bool) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.log.Info("[BrainCoordinator] SetEnabled: %v -> %v", bc.enabled, enabled)
	bc.enabled = enabled
}

func (bc *BrainCoordinator) Stats() *BrainStats {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	lobes := bc.ListLobes()
	lobeNames := make([]string, len(lobes))
	for i, l := range lobes {
		lobeNames[i] = string(l)
	}

	return &BrainStats{
		Enabled:        bc.enabled,
		LobeCount:      len(lobes),
		ProcessedCount: bc.processedCount,
		AvailableLobes: lobeNames,
	}
}

func (bc *BrainCoordinator) Stop() {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	bc.log.Info("[BrainCoordinator] Stopping Brain Executive")
	if bc.executive != nil {
		bc.executive.Stop()
		bc.log.Info("[BrainCoordinator] Brain Executive stopped")
	}
}
