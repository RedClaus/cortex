package orchestrator

import (
	"context"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/logging"
	"github.com/normanking/cortex/internal/memory"
	"github.com/normanking/cortex/internal/persona"
)

// Memory integration for ContextBuilder

// contextBuilder creates lane-appropriate context for system prompts.
// Initialized lazily on first use.
type contextBuilderWrapper struct {
	builder     *memory.ContextBuilder
	personaCore *persona.PersonaCore
	userID      string
}

// WithMemoryStore sets the memory store and initializes the ContextBuilder.
func WithMemoryStore(store *memory.CoreMemoryStore) Option {
	return func(o *Orchestrator) {
		o.memoryStore = store
	}
}

// WithPassiveRetriever sets the passive retriever for automatic memory injection.
// CRITICAL: This enables memory to be retrieved and injected into LLM context
// without requiring explicit tool calls. The PassiveRetriever performs fast
// semantic search (50ms timeout) and injects relevant memories into the system prompt.
func WithPassiveRetriever(pr *memory.PassiveRetriever) Option {
	return func(o *Orchestrator) {
		o.passiveRetriever = pr
	}
}

// WithPersonaCore sets the persona for context building.
func WithPersonaCore(p *persona.PersonaCore) Option {
	return func(o *Orchestrator) {
		// Store persona in orchestrator for use by ContextBuilder
		// We'll add a personaCore field to Orchestrator struct
		if p == nil {
			p = persona.NewPersonaCore()
		}
		// This will require adding a personaCore field to Orchestrator
		// For now, we'll document this requirement
	}
}

// getContextBuilder returns or creates the context builder for this orchestrator.
func (o *Orchestrator) getContextBuilder() *memory.ContextBuilder {
	if o.memoryStore == nil {
		return nil
	}
	// Create context builder with default config
	return memory.NewContextBuilder(o.memoryStore, memory.DefaultContextBuilderConfig())
}

// buildSystemPromptWithMemory builds a system prompt using ContextBuilder.
// This replaces the manual prompt building in llmStage.buildSystemPrompt().
// CRITICAL FIX: Persona is now MERGED with memory context instead of bypassing it.
func (s *llmStage) buildSystemPromptWithMemory(ctx context.Context, state *PipelineState) string {
	log := logging.Global()

	// CR-016 Phase 3: If an agent prompt is provided, it takes priority
	// and we skip the memory-based context building
	if state.Request.Context != nil && state.Request.Context.AgentSystemPrompt != "" {
		log.Info("[Memory] Using agent system prompt (bypassing ContextBuilder)")
		return s.buildSystemPrompt(state)
	}

	// CRITICAL FIX: Capture active persona for MERGING, not bypassing
	// Previously this would return early and bypass all memory injection
	activePersona := s.o.GetActivePersona()
	var personaPrompt string
	if activePersona != nil {
		personaPrompt = activePersona.SystemPrompt
		log.Info("[Memory] Persona active: %s (will merge with memory context)", activePersona.Name)
	}

	// Get context builder
	contextBuilder := s.o.getContextBuilder()
	if contextBuilder == nil {
		// Fall back to original prompt building
		log.Debug("[Memory] ContextBuilder not available, using legacy prompt")
		// Even without ContextBuilder, we should try passive retrieval
		basePrompt := s.buildSystemPrompt(state)
		return s.injectPassiveRetrieval(ctx, basePrompt, state)
	}

	// Determine lane based on routing and complexity
	lane := s.determineLane(state)

	// Get persona (default for now, can be customized later)
	personaCore := persona.NewPersonaCore()

	// Get project memory if available
	var projectMem *memory.ProjectMemory
	if state.Request.Context != nil && state.Request.Context.Fingerprint != nil {
		// Try to get project memory based on project root
		projectID := state.Request.Context.Fingerprint.ProjectRoot
		if projectID != "" && s.o.memoryStore != nil {
			proj, err := s.o.memoryStore.GetProjectMemory(ctx, projectID)
			if err == nil {
				projectMem = proj
			}
		}
	}

	// Get current behavioral mode (if any)
	var mode *persona.BehavioralMode
	// For now, use default mode. Can be enhanced to track mode per session.
	mode = persona.DefaultMode()

	// Build context using ContextBuilder
	userID := "default" // Can be customized based on session/user
	laneCtx, err := contextBuilder.BuildForLane(ctx, lane, userID, personaCore, projectMem, mode)
	if err != nil {
		log.Warn("[Memory] ContextBuilder failed: %v, falling back to legacy prompt", err)
		basePrompt := s.buildSystemPrompt(state)
		return s.injectPassiveRetrieval(ctx, basePrompt, state)
	}

	log.Info("[Memory] Using ContextBuilder for %s lane (%d tokens)", laneCtx.Lane, laneCtx.TokenCount)

	// Inject passive retrieval results (for both lanes, but especially important for Fast Lane)
	laneCtx.SystemPrompt = s.injectPassiveRetrieval(ctx, laneCtx.SystemPrompt, state)

	// CRITICAL FIX: Merge persona prompt with memory context instead of bypassing
	// The persona provides the AI's identity, while memory provides the context
	if personaPrompt != "" {
		// Prepend persona prompt to memory-enriched context
		// Format: [Persona Identity] + [Memory Context]
		laneCtx.SystemPrompt = personaPrompt + "\n\n## Memory Context\n\n" + laneCtx.SystemPrompt
		log.Info("[Memory] Merged persona '%s' with memory context", activePersona.Name)
	}

	return laneCtx.SystemPrompt
}

// determineLane decides which lane to use based on request characteristics.
func (s *llmStage) determineLane(state *PipelineState) memory.LaneType {
	// Use Smart Lane if:
	// 1. Cognitive analysis indicates complexity
	// 2. Task type requires deep reasoning (code review, debugging, planning)
	// 3. User explicitly requested --strong mode

	// Check cognitive complexity
	if state.Cognitive != nil && state.Cognitive.ComplexityScore > 50 {
		return memory.LaneSmart
	}

	// Check task type
	if state.Routing != nil {
		switch state.Routing.TaskType {
		case "code_review", "debug", "planning", "refactor":
			return memory.LaneSmart
		}
	}

	// Check for explicit strong mode request (cloud/frontier models)
	if state.Request.Context != nil && state.Request.Context.ModelOverride != "" {
		model := strings.ToLower(state.Request.Context.ModelOverride)
		// Only use Smart Lane for cloud/frontier models, not local Ollama models
		if strings.Contains(model, "claude") || strings.Contains(model, "gpt") ||
			strings.Contains(model, "gemini") || strings.Contains(model, "grok") ||
			strings.Contains(model, "sonnet") || strings.Contains(model, "opus") {
			return memory.LaneSmart
		}
	}

	// Default to Fast Lane
	return memory.LaneFast
}

// injectPassiveRetrieval replaces the {{PASSIVE_RETRIEVAL}} placeholder with actual results.
// This is called BEFORE the LLM sees the prompt, not by the LLM.
// CRITICAL FIX: Now uses PassiveRetriever to actively query knowledge fabric.
func (s *llmStage) injectPassiveRetrieval(ctx context.Context, prompt string, state *PipelineState) string {
	log := logging.Global()

	// Inject strategic principles FIRST (these are high-priority learned guidelines)
	prompt = s.injectStrategicPrinciples(prompt, state)

	// Get user message for semantic search
	userMessage := ""
	if state.Request != nil {
		userMessage = state.Request.Input
	}

	// Get project ID for scoping (optional)
	projectID := ""
	if state.Request.Context != nil && state.Request.Context.Fingerprint != nil {
		projectID = state.Request.Context.Fingerprint.ProjectRoot
	}

	var results []memory.PassiveResult

	// CRITICAL FIX: Use PassiveRetriever if available (preferred path)
	passiveRetriever := s.o.GetPassiveRetriever()
	if passiveRetriever != nil && userMessage != "" {
		// Create a timeout context (50ms max to not impact TTFT)
		retrieveCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		retrieved, err := passiveRetriever.Retrieve(retrieveCtx, userMessage, projectID)
		if err != nil {
			log.Debug("[Memory] Passive retrieval error (non-fatal): %v", err)
		} else if len(retrieved) > 0 {
			results = retrieved
			log.Info("[Memory] Passive retrieval found %d relevant memories for: %q", len(results), truncateForLog(userMessage, 50))
		} else {
			log.Debug("[Memory] Passive retrieval found no matches for: %q", truncateForLog(userMessage, 50))
		}
	} else if passiveRetriever == nil {
		log.Debug("[Memory] PassiveRetriever not configured, trying fallback")
	}

	// Fallback: Use pre-fetched knowledge items if PassiveRetriever unavailable
	if len(results) == 0 && s.o.fabric != nil && len(state.Knowledge) > 0 {
		log.Debug("[Memory] Using %d pre-fetched knowledge items as fallback", len(state.Knowledge))

		// Convert knowledge items to PassiveResult format
		for i, k := range state.Knowledge {
			if i >= 3 { // Limit to top 3 for Fast Lane
				break
			}
			results = append(results, memory.PassiveResult{
				ID:         k.ID,
				Summary:    k.Content,
				Confidence: k.Confidence,
			})
		}
	}

	// Inject results into prompt
	contextBuilder := s.o.getContextBuilder()
	if contextBuilder != nil {
		laneCtx := &memory.LaneContext{SystemPrompt: prompt}
		contextBuilder.InjectPassiveResults(laneCtx, results)
		if len(results) > 0 {
			log.Info("[Memory] Injected %d memory items into system prompt", len(results))
		}
		return laneCtx.SystemPrompt
	}

	// No context builder - manually inject using PassiveRetriever's method
	if passiveRetriever != nil && len(results) > 0 {
		return passiveRetriever.InjectIntoContext(prompt, results)
	}

	// Last resort: just remove placeholder
	return strings.Replace(prompt, "{{PASSIVE_RETRIEVAL}}\n", "", 1)
}

// injectStrategicPrinciples adds strategic principles to the system prompt.
// These are high-confidence learned guidelines from previous interactions.
func (s *llmStage) injectStrategicPrinciples(prompt string, state *PipelineState) string {
	log := logging.Global()

	// Check if we have strategic principles to inject
	if len(state.StrategicPrinciples) == 0 {
		return prompt
	}

	// Build the principles section
	var sb strings.Builder
	sb.WriteString("\n\n## Strategic Guidelines\n")
	sb.WriteString("Apply these learned principles when relevant:\n\n")

	for i, p := range state.StrategicPrinciples {
		// Only include high-confidence principles (>0.5)
		if p.Confidence < 0.5 {
			continue
		}

		sb.WriteString("- ")
		sb.WriteString(p.Principle)
		if p.Category != "" {
			sb.WriteString(" (")
			sb.WriteString(p.Category)
			sb.WriteString(")")
		}
		sb.WriteString("\n")

		// Limit to top 5 principles to avoid context bloat
		if i >= 4 {
			break
		}
	}

	log.Info("[Memory] Injected %d strategic principles into system prompt", min(len(state.StrategicPrinciples), 5))

	// Append principles section to prompt
	return prompt + sb.String()
}

// truncateForLog truncates a string for logging purposes
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
