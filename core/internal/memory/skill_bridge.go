// Package memory provides memory management for CortexBrain.
// This file provides the bridge between Observational Memory and Skill Distillation.
package memory

import (
	"context"
	"time"

	"github.com/normanking/cortex/internal/cognitive"
	"github.com/normanking/cortex/internal/cognitive/distillation"
	"github.com/normanking/cortex/internal/logging"
)

// ═══════════════════════════════════════════════════════════════════════════════
// MEMORY-SKILL BRIDGE
// Connects Observational Memory reflections to Skill Distillation
// ═══════════════════════════════════════════════════════════════════════════════

// SkillBridge connects observational memory to skill distillation.
type SkillBridge struct {
	om        *ObservationalMemory
	distiller *distillation.SkillDistiller
	config    *SkillBridgeConfig
	log       *logging.Logger

	running bool
	stopCh  chan struct{}
}

// SkillBridgeConfig configures the memory-skill bridge.
type SkillBridgeConfig struct {
	// Minimum reflections before triggering distillation
	MinReflections int `yaml:"min_reflections"` // Default: 5

	// Check interval
	CheckInterval time.Duration `yaml:"check_interval"` // Default: 5m

	// Agent ID for provenance tracking
	AgentID string `yaml:"agent_id"`
}

// DefaultSkillBridgeConfig returns sensible defaults.
func DefaultSkillBridgeConfig() *SkillBridgeConfig {
	return &SkillBridgeConfig{
		MinReflections: 5,
		CheckInterval:  5 * time.Minute,
		AgentID:        "default",
	}
}

// NewSkillBridge creates a new memory-skill bridge.
func NewSkillBridge(om *ObservationalMemory, distiller *distillation.SkillDistiller, config *SkillBridgeConfig) *SkillBridge {
	if config == nil {
		config = DefaultSkillBridgeConfig()
	}

	return &SkillBridge{
		om:        om,
		distiller: distiller,
		config:    config,
		log:       logging.Global(),
		stopCh:    make(chan struct{}),
	}
}

// Start begins the background distillation check loop.
func (sb *SkillBridge) Start(ctx context.Context) {
	if sb.running {
		return
	}

	sb.running = true
	ticker := time.NewTicker(sb.config.CheckInterval)
	defer ticker.Stop()

	sb.log.Info("[SkillBridge] Started with interval %v", sb.config.CheckInterval)

	for {
		select {
		case <-ctx.Done():
			sb.running = false
			sb.log.Info("[SkillBridge] Stopped (context cancelled)")
			return
		case <-sb.stopCh:
			sb.running = false
			sb.log.Info("[SkillBridge] Stopped (stop signal)")
			return
		case <-ticker.C:
			sb.checkAndDistill(ctx)
		}
	}
}

// Stop gracefully shuts down the bridge.
func (sb *SkillBridge) Stop() {
	if sb.running {
		close(sb.stopCh)
	}
}

// checkAndDistill checks for unanalyzed reflections and triggers distillation.
func (sb *SkillBridge) checkAndDistill(ctx context.Context) {
	// Get unanalyzed reflections
	reflections, err := sb.om.store.GetUnanalyzedReflections(ctx, sb.config.AgentID, sb.config.MinReflections)
	if err != nil {
		sb.log.Warn("[SkillBridge] Failed to get reflections: %v", err)
		return
	}

	if len(reflections) < sb.config.MinReflections {
		sb.log.Debug("[SkillBridge] Not enough reflections (%d < %d), skipping",
			len(reflections), sb.config.MinReflections)
		return
	}

	sb.log.Info("[SkillBridge] Found %d reflections for distillation", len(reflections))

	// Convert to distillation input
	inputs := make([]*distillation.ReflectionInput, len(reflections))
	refIDs := make([]string, len(reflections))
	for i, ref := range reflections {
		inputs[i] = &distillation.ReflectionInput{
			ID:        ref.ID,
			Content:   ref.Content,
			Timestamp: ref.Timestamp,
			Pattern:   ref.Pattern,
		}
		refIDs[i] = ref.ID
	}

	// Trigger distillation
	output, err := sb.distiller.DistillFromReflections(ctx, inputs, sb.config.AgentID)
	if err != nil {
		sb.log.Warn("[SkillBridge] Distillation failed: %v", err)
		return
	}

	// Mark reflections as analyzed
	if err := sb.om.store.MarkReflectionsAnalyzed(ctx, refIDs, ""); err != nil {
		sb.log.Warn("[SkillBridge] Failed to mark reflections analyzed: %v", err)
	}

	sb.log.Info("[SkillBridge] Distillation complete: %d skills, %d failure patterns",
		len(output.SuccessPatterns), len(output.FailureLessons))
}

// ForceDistill manually triggers distillation for a resource.
func (sb *SkillBridge) ForceDistill(ctx context.Context, resourceID string) (*cognitive.DistillationOutput, error) {
	// Get all unanalyzed reflections for this resource
	reflections, err := sb.om.store.GetUnanalyzedReflections(ctx, resourceID, 1)
	if err != nil {
		return nil, err
	}

	if len(reflections) == 0 {
		return &cognitive.DistillationOutput{}, nil
	}

	// Convert to distillation input
	inputs := make([]*distillation.ReflectionInput, len(reflections))
	refIDs := make([]string, len(reflections))
	for i, ref := range reflections {
		inputs[i] = &distillation.ReflectionInput{
			ID:        ref.ID,
			Content:   ref.Content,
			Timestamp: ref.Timestamp,
			Pattern:   ref.Pattern,
		}
		refIDs[i] = ref.ID
	}

	// Trigger distillation
	output, err := sb.distiller.DistillFromReflections(ctx, inputs, resourceID)
	if err != nil {
		return nil, err
	}

	// Mark reflections as analyzed
	if err := sb.om.store.MarkReflectionsAnalyzed(ctx, refIDs, ""); err != nil {
		sb.log.Warn("[SkillBridge] Failed to mark reflections analyzed: %v", err)
	}

	return output, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// MEMORY-ENHANCED SKILL MATCHING
// Combines skill registry with observational context
// ═══════════════════════════════════════════════════════════════════════════════

// EnhancedSkillMatch contains skill match with memory context.
type EnhancedSkillMatch struct {
	*cognitive.SkillMatchResult

	// Related observations from memory
	RelatedObservations []*Observation `json:"related_observations"`

	// Related reflections from memory
	RelatedReflections []*Reflection `json:"related_reflections"`

	// Memory-enhanced confidence adjustment
	MemoryBoost float64 `json:"memory_boost"`
}

// MatchSkillsWithMemory matches skills and enhances with memory context.
func (sb *SkillBridge) MatchSkillsWithMemory(ctx context.Context, userInput, resourceID string) (*EnhancedSkillMatch, error) {
	// Get skill matches
	skillMatch, err := sb.distiller.MatchSkills(ctx, userInput, 5)
	if err != nil {
		return nil, err
	}

	result := &EnhancedSkillMatch{
		SkillMatchResult: skillMatch,
	}

	// Search memory for related context
	memCtx, err := sb.om.SearchMemory(ctx, resourceID, userInput, 5)
	if err == nil && memCtx != nil {
		result.RelatedObservations = memCtx.Observations
		result.RelatedReflections = memCtx.Reflections

		// Calculate memory boost based on related context
		if len(memCtx.Observations) > 0 || len(memCtx.Reflections) > 0 {
			// Memory context increases confidence in skill matches
			result.MemoryBoost = 0.1 * float64(len(memCtx.Observations)+len(memCtx.Reflections))
			if result.MemoryBoost > 0.3 {
				result.MemoryBoost = 0.3 // Cap at 30% boost
			}
		}
	}

	return result, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// STATISTICS
// ═══════════════════════════════════════════════════════════════════════════════

// SkillBridgeStats contains bridge statistics.
type SkillBridgeStats struct {
	// Memory stats
	TotalMessages     int `json:"total_messages"`
	TotalObservations int `json:"total_observations"`
	TotalReflections  int `json:"total_reflections"`
	UnanalyzedReflections int `json:"unanalyzed_reflections"`

	// Skill stats
	SkillsCreated   int `json:"skills_created"`
	FailuresCreated int `json:"failures_created"`

	// Performance
	LastDistillation time.Time `json:"last_distillation"`
}

// GetStats returns bridge statistics.
func (sb *SkillBridge) GetStats(ctx context.Context, resourceID string) (*SkillBridgeStats, error) {
	stats := &SkillBridgeStats{}

	// Get memory stats
	memStats, err := sb.om.GetStats(ctx, resourceID)
	if err == nil {
		stats.TotalMessages = memStats.MessageCount
		stats.TotalObservations = memStats.ObservationCount
		stats.TotalReflections = memStats.ReflectionCount
	}

	// Count unanalyzed reflections
	unanalyzed, err := sb.om.store.GetUnanalyzedReflections(ctx, resourceID, 0)
	if err == nil {
		stats.UnanalyzedReflections = len(unanalyzed)
	}

	return stats, nil
}
