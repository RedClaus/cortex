// Package memory provides memory management for CortexBrain.
// This file implements the Observer Agent for message compression.
package memory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/cognitive"
)

// ═══════════════════════════════════════════════════════════════════════════════
// OBSERVER AGENT
// Compresses messages into observations when token threshold is exceeded
// ═══════════════════════════════════════════════════════════════════════════════

// ObserverAgent monitors and compresses message history.
type ObserverAgent struct {
	om *ObservationalMemory
}

// NewObserverAgent creates a new observer agent.
func NewObserverAgent(om *ObservationalMemory) *ObserverAgent {
	return &ObserverAgent{om: om}
}

// ObserverSystemPrompt is the system prompt for the Observer agent.
const ObserverSystemPrompt = `You are a Memory Observer. Your job is to compress conversation history into dense observations.

When compressing messages:
1. Preserve critical information: decisions made, tasks completed, errors encountered
2. Note the current task state and any pending work
3. Include timestamps for temporal context
4. Assign priority (1-5) based on importance for future context:
   - 5 = Critical decisions, errors, blockers
   - 4 = Important task completions, key learnings
   - 3 = Normal progress updates
   - 2 = Context that might be useful
   - 1 = Low-importance details
5. Remove redundant information and filler

Output format (YAML):
---
task_state: <current task description>
priority: <1-5>
observations:
  - <key observation 1>
  - <key observation 2>
  - <key observation 3>
suggested_context: <what the agent should remember going forward>
---

Compress the following messages into observations:`

// Run starts the background observation loop.
func (o *ObserverAgent) Run(ctx context.Context) {
	ticker := time.NewTicker(o.om.config.ObserverInterval)
	defer ticker.Stop()

	o.om.log.Info("[ObserverAgent] Started with interval %v", o.om.config.ObserverInterval)

	for {
		select {
		case <-ctx.Done():
			o.om.log.Info("[ObserverAgent] Stopped (context cancelled)")
			return
		case <-o.om.stopCh:
			o.om.log.Info("[ObserverAgent] Stopped (stop signal)")
			return
		case <-ticker.C:
			o.checkAndCompress(ctx)
		}
	}
}

// checkAndCompress checks all tracked resources and compresses if needed.
func (o *ObserverAgent) checkAndCompress(ctx context.Context) {
	// For now, this is a placeholder that would iterate over active resources
	// In practice, you'd track active thread/resource pairs
	o.om.log.Debug("[ObserverAgent] Check cycle running")
}

// CompressNow manually triggers compression for a specific thread/resource.
func (o *ObserverAgent) CompressNow(ctx context.Context, threadID, resourceID string) error {
	// Check if compression is needed
	tokens, err := o.om.store.GetMessageTokenCount(ctx, threadID, resourceID)
	if err != nil {
		return fmt.Errorf("get token count: %w", err)
	}

	if tokens < o.om.config.MessageThreshold {
		o.om.log.Debug("[ObserverAgent] Tokens (%d) below threshold (%d), skipping compression",
			tokens, o.om.config.MessageThreshold)
		return nil
	}

	o.om.log.Info("[ObserverAgent] Compressing messages for resource %s (tokens: %d)", resourceID, tokens)

	// Get messages to compress (keep recent, compress oldest)
	targetReduction := tokens - o.om.config.MessageThreshold/2
	messages, err := o.om.store.GetMessages(ctx, threadID, resourceID, 100)
	if err != nil {
		return fmt.Errorf("get messages: %w", err)
	}

	// Find messages to compress (oldest first until we hit target)
	var toCompress []*Message
	var compressedTokens int
	for _, msg := range messages {
		if compressedTokens >= targetReduction {
			break
		}
		toCompress = append(toCompress, msg)
		compressedTokens += msg.TokenCount
	}

	if len(toCompress) == 0 {
		return nil
	}

	// Build compression prompt
	var sb strings.Builder
	for _, msg := range toCompress {
		sb.WriteString(fmt.Sprintf("[%s] %s: %s\n",
			msg.Timestamp.Format(time.RFC3339), msg.Role, msg.Content))
	}

	// Call LLM to compress
	chatMessages := []cognitive.ChatMessage{
		{Role: "user", Content: sb.String()},
	}

	response, err := o.om.llm.Chat(ctx, chatMessages, ObserverSystemPrompt)
	if err != nil {
		return fmt.Errorf("llm compression failed: %w", err)
	}

	// Parse response and create observation
	obs := o.parseObservation(response, toCompress, threadID, resourceID)

	// Store observation
	if err := o.om.store.StoreObservation(ctx, obs); err != nil {
		return fmt.Errorf("store observation: %w", err)
	}

	// Mark messages as compressed
	messageIDs := make([]string, len(toCompress))
	for i, m := range toCompress {
		messageIDs[i] = m.ID
	}
	if err := o.om.store.MarkMessagesCompressed(ctx, messageIDs, obs.ID); err != nil {
		return fmt.Errorf("mark messages compressed: %w", err)
	}

	o.om.log.Info("[ObserverAgent] Created observation %s (compressed %d messages, %d tokens)",
		obs.ID, len(toCompress), compressedTokens)

	return nil
}

// parseObservation extracts observation from LLM response.
func (o *ObserverAgent) parseObservation(response string, messages []*Message, threadID, resourceID string) *Observation {
	// Extract message IDs for source range
	sourceRange := make([]string, len(messages))
	for i, m := range messages {
		sourceRange[i] = m.ID
	}

	// Parse priority from response (default to 3 if not found)
	priority := ObservationPriority(3)
	if strings.Contains(response, "priority: 5") {
		priority = PriorityCritical
	} else if strings.Contains(response, "priority: 4") {
		priority = PriorityHigh
	} else if strings.Contains(response, "priority: 2") {
		priority = PriorityLow
	} else if strings.Contains(response, "priority: 1") {
		priority = PriorityLow
	}

	// Extract task state
	taskState := ""
	if idx := strings.Index(response, "task_state:"); idx != -1 {
		endIdx := strings.Index(response[idx:], "\n")
		if endIdx != -1 {
			taskState = strings.TrimSpace(response[idx+11 : idx+endIdx])
		}
	}

	return &Observation{
		ID:          generateID(),
		Content:     response,
		Timestamp:   time.Now(),
		Priority:    priority,
		TaskState:   taskState,
		SourceRange: sourceRange,
		ThreadID:    threadID,
		ResourceID:  resourceID,
		TokenCount:  estimateTokens(response),
		Analyzed:    false,
		Reflected:   false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}
