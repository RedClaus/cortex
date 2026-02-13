// Package orchestrator provides the enhanced memory integration for CR-015.
// This file adds support for Strategic Memory, Topic Clustering, Orientation,
// and Memory Links to the orchestrator pipeline.
package orchestrator

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/logging"
	"github.com/normanking/cortex/internal/memory"
)

// ============================================================================
// ENHANCED MEMORY STORES (CR-015)
// ============================================================================

// EnhancedMemoryStores holds the CR-015 memory enhancement components.
type EnhancedMemoryStores struct {
	Strategic   *memory.StrategicMemoryStore
	Topics      *memory.TopicStore
	Links       *memory.LinkStore
	Orientation *memory.OrientationEngine
	Jobs        *memory.MemoryJobs
}

// EnhancedMemoryConfig configures the enhanced memory system.
type EnhancedMemoryConfig struct {
	// Enabled controls whether enhanced memory is active.
	Enabled bool

	// AutoStartJobs starts background maintenance jobs automatically.
	AutoStartJobs bool

	// JobConfig configures background job parameters.
	JobConfig memory.JobConfig

	// OrientationEnabled loads agent identity on session start.
	OrientationEnabled bool

	// TopPrinciplesLimit is how many principles to load at orientation.
	TopPrinciplesLimit int

	// ActiveTopicsLimit is how many topics to load at orientation.
	ActiveTopicsLimit int
}

// DefaultEnhancedMemoryConfig returns sensible defaults.
func DefaultEnhancedMemoryConfig() EnhancedMemoryConfig {
	return EnhancedMemoryConfig{
		Enabled:            true,
		AutoStartJobs:      true,
		JobConfig:          memory.DefaultJobConfig(),
		OrientationEnabled: true,
		TopPrinciplesLimit: 5,
		ActiveTopicsLimit:  5,
	}
}

// enhancedMemory holds the orchestrator's enhanced memory state.
type enhancedMemory struct {
	stores       *EnhancedMemoryStores
	config       EnhancedMemoryConfig
	sessionCtx   *memory.OrientationContext
	sessionStart time.Time
	enabled      bool
}

// ============================================================================
// ORCHESTRATOR INTEGRATION OPTIONS
// ============================================================================

// WithEnhancedMemory configures the enhanced memory system.
func WithEnhancedMemory(db *sql.DB, embedder memory.Embedder, llm memory.LLMProvider, config EnhancedMemoryConfig) Option {
	return func(o *Orchestrator) {
		if db == nil || !config.Enabled {
			return
		}

		stores := &EnhancedMemoryStores{}

		stores.Strategic = memory.NewStrategicMemoryStore(db, embedder)
		stores.Topics = memory.NewTopicStore(db, embedder, llm)
		stores.Links = memory.NewLinkStore(db, embedder, llm)
		stores.Orientation = memory.NewOrientationEngine(db, stores.Topics, stores.Strategic)

		jobLogger := &orchestratorJobLogger{}
		stores.Jobs = memory.NewMemoryJobs(
			db,
			stores.Topics,
			stores.Strategic,
			stores.Links,
			embedder,
			config.JobConfig,
			jobLogger,
		)

		o.enhancedMem = &enhancedMemory{
			stores:  stores,
			config:  config,
			enabled: true,
		}
	}
}

// WithEnhancedMemoryStores configures the enhanced memory system with pre-created stores.
// This is useful when stores need to be shared with other components (e.g., CR-018 introspection).
func WithEnhancedMemoryStores(stores *EnhancedMemoryStores, config EnhancedMemoryConfig) Option {
	return func(o *Orchestrator) {
		if stores == nil || !config.Enabled {
			return
		}

		o.enhancedMem = &enhancedMemory{
			stores:  stores,
			config:  config,
			enabled: true,
		}
	}
}

// CreateEnhancedMemoryStores creates the CR-015 memory stores independently.
// Use WithEnhancedMemoryStores to pass them to the orchestrator after creation.
func CreateEnhancedMemoryStores(db *sql.DB, embedder memory.Embedder, llm memory.LLMProvider, config EnhancedMemoryConfig) *EnhancedMemoryStores {
	if db == nil || !config.Enabled {
		return nil
	}

	stores := &EnhancedMemoryStores{}

	stores.Strategic = memory.NewStrategicMemoryStore(db, embedder)
	stores.Topics = memory.NewTopicStore(db, embedder, llm)
	stores.Links = memory.NewLinkStore(db, embedder, llm)
	stores.Orientation = memory.NewOrientationEngine(db, stores.Topics, stores.Strategic)

	jobLogger := &orchestratorJobLogger{}
	stores.Jobs = memory.NewMemoryJobs(
		db,
		stores.Topics,
		stores.Strategic,
		stores.Links,
		embedder,
		config.JobConfig,
		jobLogger,
	)

	return stores
}

// orchestratorJobLogger implements memory.Logger for background jobs.
type orchestratorJobLogger struct{}

func (l *orchestratorJobLogger) Info(msg string, args ...any) {
	log := logging.Global()
	log.Info("[MemoryJobs] "+msg, args...)
}

func (l *orchestratorJobLogger) Error(msg string, args ...any) {
	log := logging.Global()
	log.Error("[MemoryJobs] "+msg, args...)
}

// ============================================================================
// SESSION LIFECYCLE (CR-015)
// ============================================================================

// StartEnhancedSession loads orientation context for a new session.
// Call this at the beginning of a conversation.
func StartEnhancedSession(ctx context.Context, stores *EnhancedMemoryStores) (*memory.OrientationContext, error) {
	if stores == nil || stores.Orientation == nil {
		return nil, fmt.Errorf("enhanced memory stores not initialized")
	}

	orientation, err := stores.Orientation.WakeUp(ctx)
	if err != nil {
		return nil, fmt.Errorf("orientation wakeup failed: %w", err)
	}

	return orientation, nil
}

// EndEnhancedSession records session outcome and updates state.
// Call this at the end of a conversation.
func EndEnhancedSession(ctx context.Context, stores *EnhancedMemoryStores, outcome memory.SessionOutcome) error {
	if stores == nil || stores.Orientation == nil {
		return nil
	}

	log := logging.Global()

	// Update mood based on session sentiment
	if err := stores.Orientation.UpdateMood(ctx, outcome.Sentiment); err != nil {
		log.Warn("[EnhancedMemory] Failed to update mood: %v", err)
	}

	// Update current goal if completed
	if outcome.GoalCompleted {
		if err := stores.Orientation.ClearCurrentGoal(ctx); err != nil {
			log.Warn("[EnhancedMemory] Failed to clear goal: %v", err)
		}
	}

	// Record principle successes/failures
	for _, principleID := range outcome.PrinciplesApplied {
		if err := stores.Strategic.RecordSuccess(ctx, principleID); err != nil {
			log.Warn("[EnhancedMemory] Failed to record principle success %s: %v", principleID, err)
		}
	}

	for _, principleID := range outcome.PrinciplesIgnored {
		if err := stores.Strategic.RecordFailure(ctx, principleID); err != nil {
			log.Warn("[EnhancedMemory] Failed to record principle failure %s: %v", principleID, err)
		}
	}

	return nil
}

// ============================================================================
// SYSTEM PROMPT ENHANCEMENT (CR-015)
// ============================================================================

// BuildEnhancedSystemPreamble generates a system prompt preamble from orientation.
// This should be prepended to the base system prompt.
func BuildEnhancedSystemPreamble(orientation *memory.OrientationContext) string {
	if orientation == nil {
		return ""
	}

	var sb strings.Builder

	// Identity
	sb.WriteString(fmt.Sprintf("You are %s, a terminal AI assistant.\n\n", orientation.Identity.Name))

	// Core values
	if len(orientation.Identity.CoreValues) > 0 {
		sb.WriteString("## Core Values\n")
		for _, v := range orientation.Identity.CoreValues {
			sb.WriteString(fmt.Sprintf("- %s\n", v))
		}
		sb.WriteString("\n")
	}

	// Current goal
	if orientation.Identity.CurrentGoal != "" {
		sb.WriteString(fmt.Sprintf("## Current Focus\n%s\n\n", orientation.Identity.CurrentGoal))
	}

	// Active topics
	if len(orientation.ActiveTopics) > 0 {
		sb.WriteString("## Active Topics\n")
		for _, t := range orientation.ActiveTopics {
			if t.Description != "" {
				sb.WriteString(fmt.Sprintf("- %s: %s\n", t.Name, t.Description))
			} else {
				sb.WriteString(fmt.Sprintf("- %s\n", t.Name))
			}
		}
		sb.WriteString("\n")
	}

	// Strategic principles
	if len(orientation.TopPrinciples) > 0 {
		sb.WriteString("## Guiding Principles\n")
		for _, p := range orientation.TopPrinciples {
			successPct := int(p.SuccessRate * 100)
			sb.WriteString(fmt.Sprintf("- %s (%d%% success)\n", p.Principle, successPct))
		}
		sb.WriteString("\n")
	}

	// Recent context
	if len(orientation.SessionHistory) > 0 {
		sb.WriteString("## Recent Sessions\n")
		for _, s := range orientation.SessionHistory {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", s.Goal, s.Outcome))
		}
		sb.WriteString("\n")
	}

	// Custom persona
	if orientation.Identity.PersonaPrompt != "" {
		sb.WriteString(fmt.Sprintf("## Personality\n%s\n\n", orientation.Identity.PersonaPrompt))
	}

	return sb.String()
}

// ============================================================================
// PRINCIPLE EXTRACTION (CR-015)
// ============================================================================

// ExtractPrincipleFromFailure analyzes a failed session and extracts a principle.
// This should be called when a session fails to help the system learn.
func ExtractPrincipleFromFailure(ctx context.Context, stores *EnhancedMemoryStores, llm memory.LLMProvider, outcome memory.SessionOutcome) (*memory.StrategicMemory, error) {
	if stores == nil || stores.Strategic == nil {
		return nil, fmt.Errorf("strategic memory store not initialized")
	}

	log := logging.Global()

	// Build extraction prompt
	prompt := fmt.Sprintf(`Analyze this failed session and extract a high-level principle that could prevent similar failures.

SESSION:
User Goal: %s
Commands Tried: %v
Error Messages: %v
Final Outcome: %s

Extract a principle in this format:
PRINCIPLE: [One sentence rule, e.g., "Always check X before doing Y"]
CATEGORY: [One word: debugging, docker, git, network, config, etc.]
TRIGGER: [When to apply this, e.g., "User mentions connection issues"]

Be concise and actionable.`,
		outcome.Goal,
		outcome.Commands,
		outcome.Errors,
		outcome.Outcome,
	)

	response, err := llm.Complete(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("llm completion failed: %w", err)
	}

	// Parse response
	principle := memory.ExtractField(response, "PRINCIPLE:")
	category := memory.ExtractField(response, "CATEGORY:")
	trigger := memory.ExtractField(response, "TRIGGER:")

	if principle == "" {
		return nil, fmt.Errorf("could not extract principle from response")
	}

	// Check for existing similar principle
	existing, err := stores.Strategic.SearchSimilar(ctx, principle, 1)
	if err == nil && len(existing) > 0 {
		// Calculate similarity - if very similar, update existing instead
		// For now, we'll just increment failure count on the existing one
		similarity := 0.0 // Would calculate actual similarity here
		if similarity > memory.DeduplicationThreshold {
			log.Info("[EnhancedMemory] Found similar principle %s, incrementing failure count", existing[0].ID)
			stores.Strategic.RecordFailure(ctx, existing[0].ID)
			return &existing[0], nil
		}
	}

	// Create new principle
	mem := &memory.StrategicMemory{
		Principle:      principle,
		Category:       category,
		TriggerPattern: trigger,
		SourceSessions: []string{outcome.SessionID},
		Confidence:     0.5, // Start neutral
		FailureCount:   1,   // This came from a failure
	}

	err = stores.Strategic.Create(ctx, mem)
	if err != nil {
		return nil, fmt.Errorf("failed to create principle: %w", err)
	}

	log.Info("[EnhancedMemory] Created new principle: %s (category: %s)", principle, category)
	return mem, nil
}

// ============================================================================
// CONTEXT-AWARE RETRIEVAL (CR-015)
// ============================================================================

// RetrieveWithContradictions retrieves a memory with its relationships flagged.
// Use this when retrieving knowledge to show contradictions.
func RetrieveWithContradictions(ctx context.Context, stores *EnhancedMemoryStores, memoryID string, memoryType memory.MemoryType) (*memory.MemoryWithContext, error) {
	if stores == nil || stores.Links == nil {
		return nil, fmt.Errorf("link store not initialized")
	}

	return stores.Links.RetrieveWithContext(ctx, memoryID, memoryType)
}

// GetRelevantPrinciples finds principles relevant to the current query.
func GetRelevantPrinciples(ctx context.Context, stores *EnhancedMemoryStores, query string, limit int) ([]memory.StrategicMemory, error) {
	if stores == nil || stores.Strategic == nil {
		return nil, nil
	}

	return stores.Strategic.SearchSimilar(ctx, query, limit)
}

// GetRelevantTopic finds the most relevant topic for the current query.
func GetRelevantTopic(ctx context.Context, stores *EnhancedMemoryStores, query string) (*memory.Topic, []memory.TopicMember, error) {
	if stores == nil || stores.Topics == nil {
		return nil, nil, nil
	}

	return stores.Topics.GetActiveTopic(ctx, query)
}

// ============================================================================
// BACKGROUND JOBS MANAGEMENT
// ============================================================================

// StartMemoryJobs starts the background maintenance jobs.
func StartMemoryJobs(stores *EnhancedMemoryStores) {
	if stores == nil || stores.Jobs == nil {
		return
	}
	stores.Jobs.Start()
}

// StopMemoryJobs stops the background maintenance jobs.
func StopMemoryJobs(stores *EnhancedMemoryStores) {
	if stores == nil || stores.Jobs == nil {
		return
	}
	stores.Jobs.Stop()
}

// RunMemoryJobsNow executes all maintenance jobs immediately.
func RunMemoryJobsNow(ctx context.Context, stores *EnhancedMemoryStores) error {
	if stores == nil || stores.Jobs == nil {
		return nil
	}
	return stores.Jobs.RunNow(ctx)
}
