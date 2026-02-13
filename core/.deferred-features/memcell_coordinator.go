// Package orchestrator provides the central coordination layer for Cortex.
package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/normanking/cortex/internal/bus"
	"github.com/normanking/cortex/internal/logging"
	"github.com/normanking/cortex/internal/memory/memcell"
)

// ══════════════════════════════════════════════════════════════════════════════
// MEMCELL COORDINATOR
// CR-027: Atomic Memory Extraction Pipeline
// Integrates MemCell extraction into the orchestrator response pipeline.
// ══════════════════════════════════════════════════════════════════════════════

// MemCellSystem defines the interface for MemCell operations.
type MemCellSystem interface {
	// ExtractFromTurn extracts MemCells from a conversation turn.
	ExtractFromTurn(ctx context.Context, turn memcell.ConversationTurn) error

	// ExtractFromConversation extracts MemCells from multiple turns.
	ExtractFromConversation(ctx context.Context, turns []memcell.ConversationTurn) error

	// GetCurrentEpisode returns the current active episode ID.
	GetCurrentEpisode() string

	// Search searches for MemCells by query.
	Search(ctx context.Context, query string, opts memcell.SearchOptions) ([]memcell.SearchResult, error)

	// GetRelatedMemories returns memories related to a given ID.
	GetRelatedMemories(ctx context.Context, id string, depth int) ([]memcell.MemCell, error)

	// Stats returns MemCell statistics.
	Stats() *MemCellStats
}

// MemCellStats contains statistics about the MemCell subsystem.
type MemCellStats struct {
	TotalMemCells    int       `json:"total_memcells"`
	EpisodeCount     int       `json:"episode_count"`
	CurrentEpisodeID string    `json:"current_episode_id"`
	LastExtraction   time.Time `json:"last_extraction"`
	ExtractorType    string    `json:"extractor_type"`
}

// ══════════════════════════════════════════════════════════════════════════════
// MEMCELL COORDINATOR IMPLEMENTATION
// ══════════════════════════════════════════════════════════════════════════════

// MemCellCoordinator manages MemCell extraction and storage.
// It integrates with the orchestrator to extract atomic memories from conversations.
type MemCellCoordinator struct {
	// Core components
	store     memcell.Store
	extractor memcell.Extractor

	// Episode tracking
	episodeManager *memcell.EpisodeManager

	// Event bus for publishing extraction events
	eventBus *bus.EventBus

	// Configuration
	config *MemCellCoordinatorConfig
	log    *logging.Logger

	// State
	mu              sync.RWMutex
	enabled         bool
	lastExtraction  time.Time
	extractionQueue chan extractionJob
	stopChan        chan struct{}
	wg              sync.WaitGroup
}

// MemCellCoordinatorConfig configures the MemCellCoordinator.
type MemCellCoordinatorConfig struct {
	// Store is the MemCell storage backend.
	Store memcell.Store

	// Extractor extracts MemCells from conversation turns.
	Extractor memcell.Extractor

	// EmbedderFunc for computing embeddings (used by BoundaryDetector).
	Embedder memcell.EmbedderFunc

	// BoundaryConfig for episode boundary detection.
	BoundaryConfig memcell.BoundaryConfig

	// EventBus for publishing extraction events.
	EventBus *bus.EventBus

	// Enabled controls whether extraction is active.
	Enabled bool

	// AsyncExtraction enables background extraction (non-blocking).
	AsyncExtraction bool

	// QueueSize is the size of the async extraction queue.
	QueueSize int
}

// extractionJob represents an async extraction task.
type extractionJob struct {
	ctx   context.Context
	turns []memcell.ConversationTurn
}

// NewMemCellCoordinator creates a new MemCell coordinator.
func NewMemCellCoordinator(cfg *MemCellCoordinatorConfig) *MemCellCoordinator {
	if cfg == nil {
		cfg = &MemCellCoordinatorConfig{}
	}

	// Set defaults
	if cfg.QueueSize == 0 {
		cfg.QueueSize = 100
	}

	// Create boundary detector with embedder
	boundaryDetector := memcell.NewBoundaryDetectorWithConfig(cfg.Embedder, cfg.BoundaryConfig)

	mc := &MemCellCoordinator{
		store:           cfg.Store,
		extractor:       cfg.Extractor,
		episodeManager:  memcell.NewEpisodeManager(boundaryDetector),
		eventBus:        cfg.EventBus,
		config:          cfg,
		log:             logging.Global(),
		enabled:         cfg.Enabled,
		extractionQueue: make(chan extractionJob, cfg.QueueSize),
		stopChan:        make(chan struct{}),
	}

	// Start async extraction worker if enabled
	if cfg.AsyncExtraction {
		mc.startWorker()
	}

	return mc
}

// Verify MemCellCoordinator implements MemCellSystem at compile time.
var _ MemCellSystem = (*MemCellCoordinator)(nil)

// ══════════════════════════════════════════════════════════════════════════════
// EXTRACTION METHODS
// ══════════════════════════════════════════════════════════════════════════════

// ExtractFromTurn extracts MemCells from a single conversation turn.
func (mc *MemCellCoordinator) ExtractFromTurn(ctx context.Context, turn memcell.ConversationTurn) error {
	return mc.ExtractFromConversation(ctx, []memcell.ConversationTurn{turn})
}

// ExtractFromConversation extracts MemCells from conversation turns.
func (mc *MemCellCoordinator) ExtractFromConversation(ctx context.Context, turns []memcell.ConversationTurn) error {
	mc.mu.RLock()
	enabled := mc.enabled
	async := mc.config.AsyncExtraction
	mc.mu.RUnlock()

	if !enabled {
		return nil
	}

	// Async extraction - queue for background processing
	if async {
		select {
		case mc.extractionQueue <- extractionJob{ctx: ctx, turns: turns}:
			return nil
		default:
			// Queue full, fall through to sync
			mc.log.Debug("MemCell extraction queue full, processing synchronously")
		}
	}

	// Synchronous extraction
	return mc.doExtraction(ctx, turns)
}

// doExtraction performs the actual extraction work.
func (mc *MemCellCoordinator) doExtraction(ctx context.Context, turns []memcell.ConversationTurn) error {
	if mc.extractor == nil || mc.store == nil {
		return nil
	}

	mc.mu.Lock()
	mc.lastExtraction = time.Now()
	mc.mu.Unlock()

	// Process episode boundaries
	for _, turn := range turns {
		episodeID, isBoundary := mc.episodeManager.ProcessTurn(ctx, turn)
		if isBoundary && mc.eventBus != nil {
			mc.eventBus.Publish(NewMemCellEpisodeBoundaryEvent(episodeID, turn.Role))
		}
	}

	// Extract MemCells
	cells, err := mc.extractor.Extract(ctx, turns)
	if err != nil {
		mc.log.Error("[MemCell] Extraction failed: %v", err)
		return fmt.Errorf("memcell extraction: %w", err)
	}

	// Store extracted cells
	for _, cell := range cells {
		// Set episode ID from manager
		if mc.episodeManager.CurrentEpisode() != nil {
			cell.EpisodeID = mc.episodeManager.CurrentEpisode().ID
		}

		if err := mc.store.Create(ctx, &cell); err != nil {
			mc.log.Error("[MemCell] Failed to store cell %s: %v", cell.ID, err)
			continue
		}

		// Publish creation event
		if mc.eventBus != nil {
			mc.eventBus.Publish(NewMemCellCreatedEvent(cell.ID, string(cell.MemoryType), cell.Importance))
		}
	}

	mc.log.Debug("[MemCell] Extracted %d cells", len(cells))
	return nil
}

// ══════════════════════════════════════════════════════════════════════════════
// QUERY METHODS
// ══════════════════════════════════════════════════════════════════════════════

// GetCurrentEpisode returns the current active episode ID.
func (mc *MemCellCoordinator) GetCurrentEpisode() string {
	if ep := mc.episodeManager.CurrentEpisode(); ep != nil {
		return ep.ID
	}
	return ""
}

// Search searches for MemCells by query.
func (mc *MemCellCoordinator) Search(ctx context.Context, query string, opts memcell.SearchOptions) ([]memcell.SearchResult, error) {
	if mc.store == nil {
		return nil, fmt.Errorf("memcell store not configured")
	}
	return mc.store.Search(ctx, query, opts)
}

// GetRelatedMemories returns memories related to a given ID.
func (mc *MemCellCoordinator) GetRelatedMemories(ctx context.Context, id string, depth int) ([]memcell.MemCell, error) {
	if mc.store == nil {
		return nil, fmt.Errorf("memcell store not configured")
	}
	return mc.store.GetRelated(ctx, id, depth)
}

// ══════════════════════════════════════════════════════════════════════════════
// STATISTICS
// ══════════════════════════════════════════════════════════════════════════════

// Stats returns MemCell statistics.
func (mc *MemCellCoordinator) Stats() *MemCellStats {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	stats := &MemCellStats{
		LastExtraction: mc.lastExtraction,
	}

	if ep := mc.episodeManager.CurrentEpisode(); ep != nil {
		stats.CurrentEpisodeID = ep.ID
	}

	stats.EpisodeCount = len(mc.episodeManager.Episodes())

	// Determine extractor type
	switch mc.extractor.(type) {
	case *memcell.LLMExtractor:
		stats.ExtractorType = "llm"
	case *memcell.SimpleExtractor:
		stats.ExtractorType = "simple"
	default:
		stats.ExtractorType = "unknown"
	}

	return stats
}

// ══════════════════════════════════════════════════════════════════════════════
// LIFECYCLE
// ══════════════════════════════════════════════════════════════════════════════

// Enable enables MemCell extraction.
func (mc *MemCellCoordinator) Enable() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.enabled = true
}

// Disable disables MemCell extraction.
func (mc *MemCellCoordinator) Disable() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.enabled = false
}

// IsEnabled returns whether extraction is enabled.
func (mc *MemCellCoordinator) IsEnabled() bool {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.enabled
}

// Close shuts down the coordinator.
func (mc *MemCellCoordinator) Close() error {
	close(mc.stopChan)
	mc.wg.Wait()
	close(mc.extractionQueue)
	return nil
}

// startWorker starts the async extraction worker.
func (mc *MemCellCoordinator) startWorker() {
	mc.wg.Add(1)
	go func() {
		defer mc.wg.Done()
		for {
			select {
			case <-mc.stopChan:
				return
			case job := <-mc.extractionQueue:
				if err := mc.doExtraction(job.ctx, job.turns); err != nil {
					mc.log.Error("[MemCell] Async extraction failed: %v", err)
				}
			}
		}
	}()
}

// SetEventBus sets the event bus for publishing events.
func (mc *MemCellCoordinator) SetEventBus(eb *bus.EventBus) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.eventBus = eb
}

// ══════════════════════════════════════════════════════════════════════════════
// MEMCELL EVENTS
// ══════════════════════════════════════════════════════════════════════════════

// Event type constants for MemCell events.
const (
	EventTypeMemCellCreated         = "memcell.created"
	EventTypeMemCellUpdated         = "memcell.updated"
	EventTypeMemCellEpisodeBoundary = "memcell.episode_boundary"
)

// MemCellCreatedEvent is emitted when a MemCell is created.
type MemCellCreatedEvent struct {
	bus.BaseEvent
	CellID     string  `json:"cell_id"`
	MemoryType string  `json:"memory_type"`
	Importance float64 `json:"importance"`
}

// NewMemCellCreatedEvent creates a new MemCell created event.
func NewMemCellCreatedEvent(cellID, memoryType string, importance float64) *MemCellCreatedEvent {
	return &MemCellCreatedEvent{
		BaseEvent:  bus.NewBaseEvent(EventTypeMemCellCreated),
		CellID:     cellID,
		MemoryType: memoryType,
		Importance: importance,
	}
}

// MemCellEpisodeBoundaryEvent is emitted when an episode boundary is detected.
type MemCellEpisodeBoundaryEvent struct {
	bus.BaseEvent
	EpisodeID string `json:"episode_id"`
	TriggerBy string `json:"triggered_by"` // user, assistant
}

// NewMemCellEpisodeBoundaryEvent creates a new episode boundary event.
func NewMemCellEpisodeBoundaryEvent(episodeID, triggeredBy string) *MemCellEpisodeBoundaryEvent {
	return &MemCellEpisodeBoundaryEvent{
		BaseEvent: bus.NewBaseEvent(EventTypeMemCellEpisodeBoundary),
		EpisodeID: episodeID,
		TriggerBy: triggeredBy,
	}
}
