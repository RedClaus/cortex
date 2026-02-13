package memory

import (
	"context"
	"fmt"
	"strings"

	"github.com/normanking/cortex/internal/persona"
	"github.com/normanking/cortex/pkg/types"
	"github.com/rs/zerolog/log"
)

// LaneType represents which processing lane is being used.
type LaneType string

const (
	// LaneFast is for simple queries: <500ms, minimal context, passive retrieval only
	LaneFast LaneType = "fast"

	// LaneSmart is for complex queries: up to 15s, full context, active tool use
	LaneSmart LaneType = "smart"

	// CharsPerToken re-exports the shared constant for backward compatibility.
	CharsPerToken = types.CharsPerToken
)

// ContextBuilder creates lane-appropriate context for LLM requests.
// This is the core of CR-003's Dynamic Context Injection strategy.
type ContextBuilder struct {
	coreStore *CoreMemoryStore
	config    ContextBuilderConfig
}

// ContextBuilderConfig configures the context builder.
type ContextBuilderConfig struct {
	// Token budgets
	FastLaneMaxTokens  int // ~400 tokens for minimal context
	SmartLaneMaxTokens int // ~2000 tokens for full context

	// What to include
	IncludePersonaInFast    bool // Include minimal persona in fast lane
	IncludePrefsInFast      bool // Include user preferences in fast lane
	IncludeTechStackFast    bool // Include project tech stack in fast lane
	IncludeConventions      bool // Include conventions (smart lane only)
	IncludeToolInstructions bool // Include memory tool instructions
}

// DefaultContextBuilderConfig returns sensible defaults.
func DefaultContextBuilderConfig() ContextBuilderConfig {
	return ContextBuilderConfig{
		FastLaneMaxTokens:       400,
		SmartLaneMaxTokens:      2000,
		IncludePersonaInFast:    true,
		IncludePrefsInFast:      true,
		IncludeTechStackFast:    true,
		IncludeConventions:      true,
		IncludeToolInstructions: true,
	}
}

// LaneContext is the assembled context for a request.
type LaneContext struct {
	SystemPrompt   string          `json:"system_prompt"`
	TokenCount     int             `json:"token_count"`
	Lane           LaneType        `json:"lane"`
	PassiveResults []PassiveResult `json:"passive_results,omitempty"` // From passive retrieval (Fast Lane)
}

// PassiveResult is a pre-fetched knowledge item injected into Fast Lane context.
type PassiveResult struct {
	ID         string  `json:"id"`
	Summary    string  `json:"summary"`
	Confidence float64 `json:"confidence"`
}

// NewContextBuilder creates a new context builder.
func NewContextBuilder(coreStore *CoreMemoryStore, config ContextBuilderConfig) *ContextBuilder {
	return &ContextBuilder{
		coreStore: coreStore,
		config:    config,
	}
}

// BuildForLane creates context appropriate for the specified lane.
// This is the main entry point for context generation.
func (cb *ContextBuilder) BuildForLane(
	ctx context.Context,
	lane LaneType,
	userID string,
	personaCore *persona.PersonaCore,
	project *ProjectMemory,
	activeMode *persona.BehavioralMode,
) (*LaneContext, error) {
	// Try to auto-populate profile fields from facts (e.g., extract name from "Norman is a developer")
	// This is a no-op if name is already set, so safe to call every time
	if err := cb.coreStore.AutoPopulateProfile(ctx, userID); err != nil {
		log.Warn().Err(err).Msg("failed to auto-populate profile from facts")
	}

	userMem, err := cb.coreStore.GetUserMemory(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user memory: %w", err)
	}

	switch lane {
	case LaneFast:
		return cb.buildFastLaneContext(userMem, personaCore, project, activeMode)
	case LaneSmart:
		return cb.buildSmartLaneContext(userMem, personaCore, project, activeMode)
	default:
		// Default to fast lane for safety
		return cb.buildFastLaneContext(userMem, personaCore, project, activeMode)
	}
}

// buildFastLaneContext creates minimal context (~400 tokens).
// Optimized for speed - only essential information.
func (cb *ContextBuilder) buildFastLaneContext(
	user *UserMemory,
	personaCore *persona.PersonaCore,
	project *ProjectMemory,
	mode *persona.BehavioralMode,
) (*LaneContext, error) {
	var sb strings.Builder

	// Minimal persona (just name, role, and current mode)
	if cb.config.IncludePersonaInFast && personaCore != nil {
		sb.WriteString(fmt.Sprintf("You are %s, a %s.\n",
			personaCore.Identity.Name,
			personaCore.Identity.Role))

		if mode != nil && mode.Type != persona.ModeNormal {
			sb.WriteString(fmt.Sprintf("[Mode: %s]\n", mode.Type))
		}
		sb.WriteString("\n")
	}

	// User preferences ONLY (not full facts or history)
	if cb.config.IncludePrefsInFast && user != nil {
		hasUserContext := false

		if user.PrefersConcise {
			sb.WriteString("User prefers concise responses.\n")
			hasUserContext = true
		}
		if user.PrefersVerbose {
			sb.WriteString("User prefers detailed responses.\n")
			hasUserContext = true
		}
		if user.Shell != "" {
			sb.WriteString(fmt.Sprintf("User's shell: %s\n", user.Shell))
			hasUserContext = true
		}
		if user.OS != "" {
			sb.WriteString(fmt.Sprintf("User's OS: %s\n", user.OS))
			hasUserContext = true
		}

		if hasUserContext {
			sb.WriteString("\n")
		}

		// Include user identity and custom facts (essential for personalization)
		if user.Name != "" {
			sb.WriteString(fmt.Sprintf("User's name: %s\n", user.Name))
		}
		if user.Role != "" {
			sb.WriteString(fmt.Sprintf("User's role: %s\n", user.Role))
		}

		// Include custom facts (critical for personalized responses)
		if len(user.CustomFacts) > 0 {
			sb.WriteString("Known facts about user:\n")
			for _, f := range user.CustomFacts {
				sb.WriteString(fmt.Sprintf("  - %s\n", f.Fact))
			}
			sb.WriteString("\n")
		}
	}

	// Project tech stack ONLY (not full conventions)
	if cb.config.IncludeTechStackFast && project != nil && len(project.TechStack) > 0 {
		sb.WriteString(fmt.Sprintf("Project tech: %s\n", strings.Join(project.TechStack, ", ")))
		sb.WriteString("\n")
	}

	// Placeholder for passive retrieval results
	// This will be replaced by PassiveRetriever.InjectIntoContext()
	sb.WriteString("{{PASSIVE_RETRIEVAL}}\n")

	// Fast lane response guidelines
	sb.WriteString(`## Response Guidelines (FAST LANE)
- Be extremely concise (1-3 sentences max)
- No explanations unless specifically asked
- If you need to provide code, just provide the code
- Do not think out loud or reason through problems
- Provide direct, actionable answers only
`)

	prompt := sb.String()
	tokens := estimateTokens(prompt)

	log.Debug().
		Int("tokens", tokens).
		Int("max_tokens", cb.config.FastLaneMaxTokens).
		Msg("fast lane context built")

	return &LaneContext{
		SystemPrompt: prompt,
		TokenCount:   tokens,
		Lane:         LaneFast,
	}, nil
}

// buildSmartLaneContext creates full context (~2000 tokens).
// Includes everything needed for complex reasoning.
func (cb *ContextBuilder) buildSmartLaneContext(
	user *UserMemory,
	personaCore *persona.PersonaCore,
	project *ProjectMemory,
	mode *persona.BehavioralMode,
) (*LaneContext, error) {
	var sb strings.Builder

	// Full persona from CR-002
	if personaCore != nil {
		// Use persona's built-in system prompt builder
		basePrompt := personaCore.BuildSystemPrompt(&persona.SessionContext{})
		sb.WriteString(basePrompt)
		sb.WriteString("\n\n")
	}

	// Mode augmentation
	if mode != nil && mode.Type != persona.ModeNormal {
		sb.WriteString(mode.GetInstructions())
		sb.WriteString("\n\n")
	}

	// Full user memory
	if user != nil {
		sb.WriteString("<user_memory>\n")

		if user.Name != "" {
			sb.WriteString(fmt.Sprintf("Name: %s\n", user.Name))
		}
		if user.Role != "" {
			sb.WriteString(fmt.Sprintf("Role: %s\n", user.Role))
		}
		if user.Experience != "" {
			sb.WriteString(fmt.Sprintf("Experience level: %s\n", user.Experience))
		}
		if user.OS != "" || user.Shell != "" {
			sb.WriteString(fmt.Sprintf("Environment: %s / %s\n", user.OS, user.Shell))
		}
		if user.Editor != "" {
			sb.WriteString(fmt.Sprintf("Preferred editor: %s\n", user.Editor))
		}

		// Include all preferences
		if len(user.Preferences) > 0 {
			sb.WriteString("\nPreferences:\n")
			for _, p := range user.Preferences {
				sb.WriteString(fmt.Sprintf("  - [%s] %s (confidence: %.0f%%)\n",
					p.Category, p.Preference, p.Confidence*100))
			}
		}

		// Include custom facts
		if len(user.CustomFacts) > 0 {
			sb.WriteString("\nKnown facts:\n")
			for _, f := range user.CustomFacts {
				sb.WriteString(fmt.Sprintf("  - %s\n", f.Fact))
			}
		}

		sb.WriteString("</user_memory>\n\n")
	}

	// Full project context
	if project != nil && project.Name != "" {
		sb.WriteString("<project>\n")
		sb.WriteString(fmt.Sprintf("Name: %s\n", project.Name))

		if project.Path != "" {
			sb.WriteString(fmt.Sprintf("Path: %s\n", project.Path))
		}
		if project.Type != "" {
			sb.WriteString(fmt.Sprintf("Type: %s\n", project.Type))
		}
		if len(project.TechStack) > 0 {
			sb.WriteString(fmt.Sprintf("Tech Stack: %s\n", strings.Join(project.TechStack, ", ")))
		}
		if project.GitBranch != "" {
			sb.WriteString(fmt.Sprintf("Git Branch: %s\n", project.GitBranch))
		}

		// Include conventions (smart lane only)
		if cb.config.IncludeConventions && len(project.Conventions) > 0 {
			sb.WriteString("Conventions:\n")
			for _, c := range project.Conventions {
				sb.WriteString(fmt.Sprintf("  - %s\n", c))
			}
		}

		sb.WriteString("</project>\n\n")
	}

	// Memory tool instructions (Smart Lane only)
	if cb.config.IncludeToolInstructions {
		sb.WriteString(memoryToolInstructions)
		sb.WriteString("\n")
	}

	// Smart lane response guidelines
	sb.WriteString(`## Response Guidelines (SMART LANE)
- You may think through problems step by step
- Explain your reasoning when helpful
- Consider multiple approaches before deciding
- Provide comprehensive but focused responses
- Use code examples with explanations when relevant
- Use memory tools when you need context not in the current conversation
`)

	prompt := sb.String()
	tokens := estimateTokens(prompt)

	log.Debug().
		Int("tokens", tokens).
		Int("max_tokens", cb.config.SmartLaneMaxTokens).
		Msg("smart lane context built")

	return &LaneContext{
		SystemPrompt: prompt,
		TokenCount:   tokens,
		Lane:         LaneSmart,
	}, nil
}

// InjectPassiveResults replaces the placeholder with actual passive retrieval results.
func (cb *ContextBuilder) InjectPassiveResults(laneCtx *LaneContext, results []PassiveResult) {
	if len(results) == 0 {
		// Remove placeholder entirely
		laneCtx.SystemPrompt = strings.Replace(laneCtx.SystemPrompt, "{{PASSIVE_RETRIEVAL}}\n", "", 1)
		return
	}

	var sb strings.Builder
	sb.WriteString("<relevant_knowledge>\n")
	for _, r := range results {
		sb.WriteString(fmt.Sprintf("• %s (confidence: %.0f%%)\n", r.Summary, r.Confidence*100))
	}
	sb.WriteString("</relevant_knowledge>\n")

	laneCtx.SystemPrompt = strings.Replace(laneCtx.SystemPrompt, "{{PASSIVE_RETRIEVAL}}\n", sb.String(), 1)
	laneCtx.PassiveResults = results

	// Update token count
	laneCtx.TokenCount = estimateTokens(laneCtx.SystemPrompt)
}

// estimateTokens provides a rough token estimate.
// Wraps types.EstimateTokens for package-level convenience.
func estimateTokens(text string) int {
	return types.EstimateTokens(text)
}

// Memory tool instructions for Smart Lane.
const memoryToolInstructions = `## Memory Tools

You have access to memory tools for managing long-term context:

**Search & Retrieve:**
- recall_memory_search: Search past conversations for relevant context
- core_memory_read: Read persistent facts about user or project
- archival_memory_search: Search knowledge base for lessons and solutions

**Store & Remember:**
- core_memory_update: Update user's profile (name, role, os, shell, editor). IMPORTANT: Use this when you first learn the user's name!
- core_memory_append: Remember a new fact about the user
- archival_memory_insert: Store a lesson learned for future reference

**Guidelines:**
- Use tools when you need context not in the current conversation
- Do NOT use tools for simple questions you can answer directly
- When you learn the user's name for the first time, use core_memory_update to save it
- When you learn something about the user, consider storing it
- After solving a complex problem, consider archiving the solution`
