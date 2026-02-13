// Package memory provides memory management for CortexBrain.
// This file implements the Reflector Agent for observation consolidation.
package memory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/cognitive"
)

// ═══════════════════════════════════════════════════════════════════════════════
// REFLECTOR AGENT
// Consolidates observations into high-level reflections
// ═══════════════════════════════════════════════════════════════════════════════

// ReflectorAgent consolidates observations into reflections.
type ReflectorAgent struct {
	om *ObservationalMemory
}

// NewReflectorAgent creates a new reflector agent.
func NewReflectorAgent(om *ObservationalMemory) *ReflectorAgent {
	return &ReflectorAgent{om: om}
}

// ReflectorSystemPrompt is the system prompt for the Reflector agent.
const ReflectorSystemPrompt = `You are a Memory Reflector. Your job is to consolidate observations into high-level patterns and insights.

When reflecting:
1. Identify recurring patterns across observations
2. Combine related observations into unified insights
3. Preserve the most important context for long-term memory
4. Identify behavioral patterns, preferences, and learned strategies
5. Note any failures or errors that should be remembered

Output format (YAML):
---
pattern_type: <workflow|preference|strategy|error|learning>
insight: <consolidated understanding in 1-2 sentences>
key_details:
  - <important detail 1>
  - <important detail 2>
observations_to_keep: <list of observation IDs that remain relevant>
observations_to_remove: <list of observation IDs that are now redundant>
---

Consolidate the following observations:`

// Run starts the background reflection loop.
func (r *ReflectorAgent) Run(ctx context.Context) {
	ticker := time.NewTicker(r.om.config.ReflectorInterval)
	defer ticker.Stop()

	r.om.log.Info("[ReflectorAgent] Started with interval %v", r.om.config.ReflectorInterval)

	for {
		select {
		case <-ctx.Done():
			r.om.log.Info("[ReflectorAgent] Stopped (context cancelled)")
			return
		case <-r.om.stopCh:
			r.om.log.Info("[ReflectorAgent] Stopped (stop signal)")
			return
		case <-ticker.C:
			r.checkAndReflect(ctx)
		}
	}
}

// checkAndReflect checks all tracked resources and reflects if needed.
func (r *ReflectorAgent) checkAndReflect(ctx context.Context) {
	// For now, this is a placeholder that would iterate over active resources
	r.om.log.Debug("[ReflectorAgent] Check cycle running")
}

// ReflectNow manually triggers reflection for a specific resource.
func (r *ReflectorAgent) ReflectNow(ctx context.Context, resourceID string) error {
	// Check if reflection is needed
	tokens, err := r.om.store.GetObservationTokenCount(ctx, resourceID)
	if err != nil {
		return fmt.Errorf("get observation token count: %w", err)
	}

	if tokens < r.om.config.ObservationThreshold {
		r.om.log.Debug("[ReflectorAgent] Observation tokens (%d) below threshold (%d), skipping reflection",
			tokens, r.om.config.ObservationThreshold)
		return nil
	}

	r.om.log.Info("[ReflectorAgent] Reflecting observations for resource %s (tokens: %d)", resourceID, tokens)

	// Get observations to consolidate
	observations, err := r.om.store.GetObservations(ctx, resourceID, 50)
	if err != nil {
		return fmt.Errorf("get observations: %w", err)
	}

	if len(observations) < 3 {
		return nil // Need multiple observations to reflect
	}

	// Build reflection prompt
	var sb strings.Builder
	for _, obs := range observations {
		sb.WriteString(fmt.Sprintf("[%s] Priority %d - %s\n%s\n\n",
			obs.Timestamp.Format(time.RFC3339),
			obs.Priority,
			obs.TaskState,
			obs.Content))
	}

	// Call LLM to reflect
	chatMessages := []cognitive.ChatMessage{
		{Role: "user", Content: sb.String()},
	}

	response, err := r.om.llm.Chat(ctx, chatMessages, ReflectorSystemPrompt)
	if err != nil {
		return fmt.Errorf("llm reflection failed: %w", err)
	}

	// Parse response and create reflection
	ref := r.parseReflection(response, observations, resourceID)

	// Store reflection
	if err := r.om.store.StoreReflection(ctx, ref); err != nil {
		return fmt.Errorf("store reflection: %w", err)
	}

	// Mark observations as reflected (extract IDs from response or use all)
	obsIDs := make([]string, len(observations))
	for i, obs := range observations {
		obsIDs[i] = obs.ID
	}
	if err := r.om.store.MarkObservationsReflected(ctx, obsIDs, ref.ID); err != nil {
		return fmt.Errorf("mark observations reflected: %w", err)
	}

	r.om.log.Info("[ReflectorAgent] Created reflection %s (consolidated %d observations)",
		ref.ID, len(observations))

	return nil
}

// parseReflection extracts reflection from LLM response.
func (r *ReflectorAgent) parseReflection(response string, observations []*Observation, resourceID string) *Reflection {
	// Extract observation IDs for source
	sourceObs := make([]string, len(observations))
	for i, obs := range observations {
		sourceObs[i] = obs.ID
	}

	// Extract pattern type from response
	patternType := "general"
	patternTypes := []string{"workflow", "preference", "strategy", "error", "learning"}
	for _, pt := range patternTypes {
		if strings.Contains(strings.ToLower(response), "pattern_type: "+pt) {
			patternType = pt
			break
		}
	}

	return &Reflection{
		ID:         generateID(),
		Content:    response,
		Timestamp:  time.Now(),
		Pattern:    patternType,
		SourceObs:  sourceObs,
		ResourceID: resourceID,
		TokenCount: estimateTokens(response),
		Analyzed:   false,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}
