// Package memory provides memory management for CortexBrain.
// This file implements the ObservationalMemory system (three-tier compression).
package memory

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/normanking/cortex/internal/cognitive"
	"github.com/normanking/cortex/internal/logging"
)

// ═══════════════════════════════════════════════════════════════════════════════
// OBSERVATIONAL MEMORY
// Central coordinator for three-tier memory compression
// ═══════════════════════════════════════════════════════════════════════════════

// ObservationalMemory manages the three-tier memory compression system.
type ObservationalMemory struct {
	// Storage backend (SQLite working memory + Memvid episodic)
	store ObservationalStore

	// Background agents
	observer  *ObserverAgent
	reflector *ReflectorAgent

	// LLM provider for background agents
	llm cognitive.SimpleChatProvider

	// Configuration
	config *ObservationalMemoryConfig

	// State
	running bool
	stopCh  chan struct{}
	mu      sync.RWMutex

	// Logging
	log *logging.Logger
}

// NewObservationalMemory creates a new observational memory system.
func NewObservationalMemory(store ObservationalStore, llm cognitive.SimpleChatProvider, config *ObservationalMemoryConfig) *ObservationalMemory {
	if config == nil {
		config = DefaultObservationalMemoryConfig()
	}

	om := &ObservationalMemory{
		store:  store,
		llm:    llm,
		config: config,
		stopCh: make(chan struct{}),
		log:    logging.Global(),
	}

	// Create background agents
	om.observer = NewObserverAgent(om)
	om.reflector = NewReflectorAgent(om)

	return om
}

// generateID creates a random ID for tracking.
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ═══════════════════════════════════════════════════════════════════════════════
// LIFECYCLE
// ═══════════════════════════════════════════════════════════════════════════════

// Start begins the background observer and reflector agents.
func (om *ObservationalMemory) Start(ctx context.Context) {
	om.mu.Lock()
	defer om.mu.Unlock()

	if om.running {
		return
	}

	om.running = true
	om.log.Info("[ObservationalMemory] Starting background agents")

	// Start observer agent
	go om.observer.Run(ctx)

	// Start reflector agent
	go om.reflector.Run(ctx)
}

// Stop gracefully shuts down the background agents.
func (om *ObservationalMemory) Stop() {
	om.mu.Lock()
	defer om.mu.Unlock()

	if !om.running {
		return
	}

	om.log.Info("[ObservationalMemory] Stopping background agents")
	close(om.stopCh)
	om.running = false
}

// ═══════════════════════════════════════════════════════════════════════════════
// MESSAGE OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════════

// AddMessage stores a new message and checks for compression thresholds.
func (om *ObservationalMemory) AddMessage(ctx context.Context, msg *Message) error {
	if msg.ID == "" {
		msg.ID = generateID()
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	// Estimate token count if not set
	if msg.TokenCount == 0 {
		msg.TokenCount = estimateTokens(msg.Content)
	}

	return om.store.StoreMessage(ctx, msg)
}

// GetContext retrieves the current observational context for prompt injection.
func (om *ObservationalMemory) GetContext(ctx context.Context, threadID, resourceID string) (*ObservationalContext, error) {
	oc := &ObservationalContext{}

	// Get recent messages (Tier 1)
	messages, err := om.store.GetMessages(ctx, threadID, resourceID, 50)
	if err != nil {
		om.log.Warn("[ObservationalMemory] Failed to get messages: %v", err)
	} else {
		oc.Messages = messages
		for _, m := range messages {
			oc.MessageTokens += m.TokenCount
		}
	}

	// Get observations (Tier 2)
	observations, err := om.store.GetObservations(ctx, resourceID, 20)
	if err != nil {
		om.log.Warn("[ObservationalMemory] Failed to get observations: %v", err)
	} else {
		oc.Observations = observations
		for _, o := range observations {
			oc.ObservationTokens += o.TokenCount
		}
	}

	// Get reflections (Tier 3)
	reflections, err := om.store.GetReflections(ctx, resourceID, 10)
	if err != nil {
		om.log.Warn("[ObservationalMemory] Failed to get reflections: %v", err)
	} else {
		oc.Reflections = reflections
		for _, r := range reflections {
			oc.ReflectionTokens += r.TokenCount
		}
	}

	oc.TotalTokens = oc.MessageTokens + oc.ObservationTokens + oc.ReflectionTokens

	return oc, nil
}

// SearchMemory performs semantic search across all memory tiers.
func (om *ObservationalMemory) SearchMemory(ctx context.Context, resourceID, query string, limit int) (*ObservationalContext, error) {
	return om.store.SearchMemory(ctx, resourceID, query, limit)
}

// GetTimeline returns memory at a specific point in time (time-travel).
func (om *ObservationalMemory) GetTimeline(ctx context.Context, resourceID string, from, to time.Time) (*ObservationalContext, error) {
	return om.store.GetTimeline(ctx, resourceID, from, to)
}

// ═══════════════════════════════════════════════════════════════════════════════
// MANUAL TRIGGERS
// ═══════════════════════════════════════════════════════════════════════════════

// ForceObserve manually triggers observation compression.
func (om *ObservationalMemory) ForceObserve(ctx context.Context, threadID, resourceID string) error {
	return om.observer.CompressNow(ctx, threadID, resourceID)
}

// ForceReflect manually triggers reflection consolidation.
func (om *ObservationalMemory) ForceReflect(ctx context.Context, resourceID string) error {
	return om.reflector.ReflectNow(ctx, resourceID)
}

// ═══════════════════════════════════════════════════════════════════════════════
// STATISTICS
// ═══════════════════════════════════════════════════════════════════════════════

// Stats returns observational memory statistics.
type ObservationalStats struct {
	MessageCount     int `json:"message_count"`
	ObservationCount int `json:"observation_count"`
	ReflectionCount  int `json:"reflection_count"`
	MessageTokens    int `json:"message_tokens"`
	ObservationTokens int `json:"observation_tokens"`
	ReflectionTokens int `json:"reflection_tokens"`
	TotalTokens      int `json:"total_tokens"`
	CompressionRatio float64 `json:"compression_ratio"`
}

// GetStats returns memory statistics for a resource.
func (om *ObservationalMemory) GetStats(ctx context.Context, resourceID string) (*ObservationalStats, error) {
	stats := &ObservationalStats{}

	// Get token counts
	msgTokens, err := om.store.GetMessageTokenCount(ctx, "", resourceID)
	if err == nil {
		stats.MessageTokens = msgTokens
	}

	obsTokens, err := om.store.GetObservationTokenCount(ctx, resourceID)
	if err == nil {
		stats.ObservationTokens = obsTokens
	}

	// Get context for counts
	oc, err := om.GetContext(ctx, "", resourceID)
	if err == nil {
		stats.MessageCount = len(oc.Messages)
		stats.ObservationCount = len(oc.Observations)
		stats.ReflectionCount = len(oc.Reflections)
		stats.ReflectionTokens = oc.ReflectionTokens
	}

	stats.TotalTokens = stats.MessageTokens + stats.ObservationTokens + stats.ReflectionTokens

	// Calculate compression ratio (original / compressed)
	if stats.ObservationTokens+stats.ReflectionTokens > 0 {
		// Estimate original size from what we compressed
		originalEstimate := stats.MessageTokens + stats.ObservationTokens*10 // Rough estimate
		stats.CompressionRatio = float64(originalEstimate) / float64(stats.TotalTokens)
	}

	return stats, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPER FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════════

// estimateTokens is defined in context_builder.go
