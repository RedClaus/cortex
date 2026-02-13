package context

import (
	"time"

	"github.com/google/uuid"
)

// LobeID identifies the source lobe of a context item.
// Matches the LobeID type in pkg/brain/lobe.go.
type LobeID string

// ContextItem represents a single piece of context in the attention-aware blackboard.
// This is the SIMPLIFIED version per Architect validation (8 fields, not 14).
//
// Removed fields (add when proven necessary):
//   - LastAccessed, AccessCount (tracking adds complexity)
//   - ExpiresAt (TTL rarely used)
//   - Tags (rarely needed for filtering)
//   - Confidence (can be derived from source lobe)
//   - ContentText (Content is sufficient)
type ContextItem struct {
	// ID uniquely identifies this context item
	ID string `json:"id"`

	// Source identifies which lobe or system produced this item
	Source LobeID `json:"source"`

	// Category classifies the content type for filtering
	// Examples: "code", "emotion", "memory", "task", "user", "system"
	Category string `json:"category"`

	// Content holds the actual data (string, struct, or any serializable type)
	Content interface{} `json:"content"`

	// TokenCount is the estimated token count for this item's content
	TokenCount int `json:"token_count"`

	// Priority indicates importance for compaction decisions (0.0-1.0)
	// Higher priority items are kept longer during pruning
	Priority float64 `json:"priority"`

	// Zone determines where this item appears in the context window
	Zone AttentionZone `json:"zone"`

	// CreatedAt records when this item was added
	CreatedAt time.Time `json:"created_at"`
}

// NewContextItem creates a new context item with a generated ID and current timestamp.
func NewContextItem(source LobeID, category string, content interface{}, zone AttentionZone) *ContextItem {
	return &ContextItem{
		ID:        uuid.New().String(),
		Source:    source,
		Category:  category,
		Content:   content,
		Zone:      zone,
		Priority:  0.5, // Default priority
		CreatedAt: time.Now(),
	}
}

// WithPriority sets the priority and returns the item for chaining.
func (c *ContextItem) WithPriority(p float64) *ContextItem {
	if p < 0 {
		p = 0
	} else if p > 1 {
		p = 1
	}
	c.Priority = p
	return c
}

// WithTokenCount sets the token count and returns the item for chaining.
func (c *ContextItem) WithTokenCount(count int) *ContextItem {
	c.TokenCount = count
	return c
}

// EffectivePriority combines item priority with zone priority.
// Used for compaction decisions.
func (c *ContextItem) EffectivePriority() float64 {
	zonePriority := ZonePriority(c.Zone)
	return (c.Priority + zonePriority) / 2
}

// Age returns how long ago this item was created.
func (c *ContextItem) Age() time.Duration {
	return time.Since(c.CreatedAt)
}

// ContentString attempts to return content as a string.
// Returns empty string if content is not a string type.
func (c *ContextItem) ContentString() string {
	if s, ok := c.Content.(string); ok {
		return s
	}
	return ""
}

// Common category constants for filtering
const (
	// Core categories
	CategorySystem    = "system"    // System prompts, instructions
	CategoryUser      = "user"      // User profile, preferences
	CategoryMemory    = "memory"    // Retrieved memories
	CategoryTask      = "task"      // Current task, action items
	CategoryCode      = "code"      // Code snippets, files
	CategoryEmotion   = "emotion"   // Emotional context
	CategoryTechnical = "technical" // Technical documentation
	CategoryError     = "error"     // Errors, issues
	CategoryOutput    = "output"    // Lobe outputs

	// Perception categories
	CategoryVisual = "visual" // Images, screenshots
	CategoryVoice  = "voice"  // Voice transcripts
	CategoryAudio  = "audio"  // Audio signals
	CategoryText   = "text"   // Raw text input
	CategoryFile   = "file"   // File references

	// Cognitive categories
	CategoryGoal         = "goal"         // Goals and objectives
	CategoryPlan         = "plan"         // Plans and strategies
	CategoryIdea         = "idea"         // Creative ideas
	CategoryCreative     = "creative"     // Creative content
	CategoryAnalysis     = "analysis"     // Analytical content
	CategoryEvidence     = "evidence"     // Supporting evidence
	CategoryProof        = "proof"        // Logical proofs
	CategoryIntent       = "intent"       // User intent
	CategoryConversation = "conversation" // Conversation history
	CategoryProject      = "project"      // Project context

	// Temporal/Spatial categories
	CategoryTime     = "time"     // Time-related context
	CategorySchedule = "schedule" // Schedules and events
	CategorySpatial  = "spatial"  // Spatial information
	CategoryLayout   = "layout"   // Layout information

	// Causal categories
	CategoryCause  = "cause"  // Cause information
	CategoryEffect = "effect" // Effect information

	// Executive categories
	CategoryStrategy    = "strategy"    // Strategic context
	CategoryReflection  = "reflection"  // Reflective content
	CategorySafety      = "safety"      // Safety information
	CategoryConstraint  = "constraint"  // Constraints
	CategoryRisk        = "risk"        // Risk information
	CategoryPersonality = "personality" // Personality traits
	CategoryCapability  = "capability"  // Capabilities
)

// Source lobe IDs for all 20 lobes
const (
	// Perception layer
	SourceVisionLobe      LobeID = "vision"
	SourceAuditionLobe    LobeID = "audition"
	SourceTextParsingLobe LobeID = "text_parsing"

	// Cognitive layer
	SourceMemoryLobe     LobeID = "memory"
	SourcePlanningLobe   LobeID = "planning"
	SourceCreativityLobe LobeID = "creativity"
	SourceReasoningLobe  LobeID = "reasoning"

	// Social-Emotional layer
	SourceEmotionLobe       LobeID = "emotion"
	SourceTheoryOfMindLobe  LobeID = "theory_of_mind"
	SourceRapportLobe       LobeID = "rapport"

	// Specialized layer
	SourceCodingLobe   LobeID = "coding"
	SourceLogicLobe    LobeID = "logic"
	SourceTemporalLobe LobeID = "temporal"
	SourceSpatialLobe  LobeID = "spatial"
	SourceCausalLobe   LobeID = "causal"

	// Executive layer
	SourceAttentionLobe     LobeID = "attention"
	SourceMetacognitionLobe LobeID = "metacognition"
	SourceInhibitionLobe    LobeID = "inhibition"
	SourceSelfKnowledgeLobe LobeID = "self_knowledge"
	SourceSafetyLobe        LobeID = "safety"

	// System source (not a lobe, but used for system-generated items)
	SourceSystem LobeID = "system"
)
