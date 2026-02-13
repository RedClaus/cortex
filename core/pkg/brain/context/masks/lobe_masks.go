package masks

import (
	"github.com/normanking/cortex/pkg/brain/context"
)

// AllLobeMasks returns context masks for all 20 lobes.
// These masks define what context each lobe should receive,
// enabling efficient context tailoring based on lobe specialization.
//
// Masks are organized by brain layer:
// - Perception: Vision, Audition, TextParsing
// - Cognitive: Memory, Planning, Creativity, Reasoning
// - Social-Emotional: Emotion, TheoryOfMind, Rapport
// - Specialized: Coding, Logic, Temporal, Spatial, Causal
// - Executive: Attention, Metacognition, Inhibition, SelfKnowledge, Safety
func AllLobeMasks() []*ContextMask {
	return []*ContextMask{
		// =====================================================================
		// PERCEPTION LAYER
		// =====================================================================

		// VisionLobe - processes visual input (images, screenshots)
		{
			LobeID:            context.SourceVisionLobe,
			IncludeCategories: []string{context.CategoryVisual, context.CategoryFile},
			ExcludeCategories: []string{context.CategoryVoice},
			MaxTokens:         4000,
			Description:       "Vision lobe focuses on visual content and file references",
		},

		// AuditionLobe - processes audio input (voice, sounds)
		{
			LobeID:            context.SourceAuditionLobe,
			IncludeCategories: []string{context.CategoryVoice, context.CategoryAudio},
			MaxTokens:         2000,
			Description:       "Audition lobe focuses on audio and voice content",
		},

		// TextParsingLobe - parses and structures text input
		{
			LobeID:            context.SourceTextParsingLobe,
			IncludeCategories: []string{context.CategoryText, context.CategoryCode, context.CategoryMemory},
			MaxTokens:         8000,
			Description:       "Text parsing sees text, code, and memory for structural analysis",
		},

		// =====================================================================
		// COGNITIVE LAYER
		// =====================================================================

		// MemoryLobe - retrieval and storage of memories
		{
			LobeID:            context.SourceMemoryLobe,
			IncludeCategories: []string{context.CategoryMemory, context.CategoryUser, context.CategoryProject},
			ExcludeCategories: []string{context.CategoryVisual, context.CategoryVoice},
			MaxTokens:         6000,
			Description:       "Memory lobe focuses on stored knowledge, user, and project context",
		},

		// PlanningLobe - creates and manages plans
		{
			LobeID:            context.SourcePlanningLobe,
			IncludeCategories: []string{context.CategoryTask, context.CategoryGoal, context.CategoryPlan, context.CategoryMemory},
			IncludeZones:      []context.AttentionZone{context.ZoneCritical, context.ZoneActionable},
			MaxTokens:         5000,
			Description:       "Planning focuses on tasks, goals, and actionable context",
		},

		// CreativityLobe - generates creative solutions
		{
			LobeID:            context.SourceCreativityLobe,
			IncludeCategories: []string{context.CategoryIdea, context.CategoryCreative, context.CategoryMemory},
			MinPriority:       0.3, // Focus on higher-value items
			MaxTokens:         4000,
			Description:       "Creativity lobe draws from ideas, creative content, and memories",
		},

		// ReasoningLobe - logical reasoning and problem solving
		{
			LobeID:            context.SourceReasoningLobe,
			IncludeCategories: []string{context.CategoryAnalysis, context.CategoryEvidence, context.CategoryMemory, context.CategoryCode},
			MaxTokens:         8000,
			Description:       "Reasoning sees analytical content, evidence, and code for problem solving",
		},

		// =====================================================================
		// SOCIAL-EMOTIONAL LAYER
		// =====================================================================

		// EmotionLobe - processes emotional signals
		{
			LobeID:            context.SourceEmotionLobe,
			IncludeCategories: []string{context.CategoryEmotion, context.CategoryUser, context.CategoryVoice},
			ExcludeCategories: []string{context.CategoryCode, context.CategoryFile},
			MaxTokens:         3000,
			Description:       "Emotion lobe focuses on emotional signals and user context",
		},

		// TheoryOfMindLobe - models user mental state
		{
			LobeID:            context.SourceTheoryOfMindLobe,
			IncludeCategories: []string{context.CategoryUser, context.CategoryIntent, context.CategoryEmotion, context.CategoryMemory},
			MaxTokens:         4000,
			Description:       "Theory of mind models user intent and mental state",
		},

		// RapportLobe - builds and maintains relationship
		{
			LobeID:            context.SourceRapportLobe,
			IncludeCategories: []string{context.CategoryUser, context.CategoryConversation, context.CategoryEmotion},
			ExcludeCategories: []string{context.CategoryCode, context.CategoryFile},
			MaxTokens:         3000,
			Description:       "Rapport focuses on user relationship and conversation history",
		},

		// =====================================================================
		// SPECIALIZED LAYER
		// =====================================================================

		// CodingLobe - writes and analyzes code
		{
			LobeID:            context.SourceCodingLobe,
			IncludeCategories: []string{context.CategoryCode, context.CategoryError, context.CategoryFile, context.CategoryProject},
			ExcludeCategories: []string{context.CategoryEmotion, context.CategoryVoice},
			MaxTokens:         12000, // Large context for code
			Description:       "Coding lobe needs code, errors, files, and project context",
		},

		// LogicLobe - formal logic and proofs
		{
			LobeID:            context.SourceLogicLobe,
			IncludeCategories: []string{context.CategoryAnalysis, context.CategoryEvidence, context.CategoryProof},
			MaxTokens:         6000,
			Description:       "Logic lobe focuses on analytical content and proofs",
		},

		// TemporalLobe - time-based reasoning
		{
			LobeID:            context.SourceTemporalLobe,
			IncludeCategories: []string{context.CategoryTime, context.CategorySchedule, context.CategoryMemory},
			MaxTokens:         3000,
			Description:       "Temporal lobe focuses on time-related context and schedules",
		},

		// SpatialLobe - spatial reasoning and layouts
		{
			LobeID:            context.SourceSpatialLobe,
			IncludeCategories: []string{context.CategorySpatial, context.CategoryVisual, context.CategoryLayout},
			MaxTokens:         3000,
			Description:       "Spatial lobe focuses on layouts and spatial relationships",
		},

		// CausalLobe - cause and effect reasoning
		{
			LobeID:            context.SourceCausalLobe,
			IncludeCategories: []string{context.CategoryCause, context.CategoryEffect, context.CategoryAnalysis},
			MaxTokens:         4000,
			Description:       "Causal lobe focuses on cause-effect relationships",
		},

		// =====================================================================
		// EXECUTIVE LAYER
		// =====================================================================

		// AttentionLobe - manages focus and priority
		{
			LobeID:            context.SourceAttentionLobe,
			IncludeZones:      []context.AttentionZone{context.ZoneCritical, context.ZoneActionable},
			MinPriority:       0.5, // Only high-priority items
			MaxTokens:         2000,
			Description:       "Attention focuses on critical and actionable high-priority items",
		},

		// MetacognitionLobe - monitors cognitive processes
		{
			LobeID:            context.SourceMetacognitionLobe,
			IncludeCategories: []string{context.CategorySystem, context.CategoryStrategy, context.CategoryReflection},
			MaxTokens:         2000,
			Description:       "Metacognition sees system state and strategic context",
		},

		// InhibitionLobe - stops inappropriate actions
		{
			LobeID:            context.SourceInhibitionLobe,
			IncludeCategories: []string{context.CategorySystem, context.CategorySafety, context.CategoryConstraint},
			IncludeZones:      []context.AttentionZone{context.ZoneCritical},
			MaxTokens:         1000,
			Description:       "Inhibition focuses on safety constraints and critical context",
		},

		// SelfKnowledgeLobe - introspective awareness
		{
			LobeID:            context.SourceSelfKnowledgeLobe,
			IncludeCategories: []string{context.CategorySystem, context.CategoryPersonality, context.CategoryCapability},
			MaxTokens:         2000,
			Description:       "Self-knowledge sees system identity and capabilities",
		},

		// SafetyLobe - safety and risk assessment
		{
			LobeID:            context.SourceSafetyLobe,
			IncludeCategories: []string{context.CategorySafety, context.CategoryRisk, context.CategoryConstraint, context.CategorySystem},
			IncludeZones:      []context.AttentionZone{context.ZoneCritical, context.ZoneActionable},
			MaxTokens:         2000,
			MinPriority:       0.7, // Only high-priority for safety decisions
			Description:       "Safety lobe sees safety-relevant, high-priority context",
		},
	}
}

// GetMaskForLobe returns the mask for a specific lobe ID.
// Convenience function for quick access without registry.
func GetMaskForLobe(lobeID context.LobeID) *ContextMask {
	for _, mask := range AllLobeMasks() {
		if mask.LobeID == lobeID {
			return mask
		}
	}
	return nil
}

// DefaultMask returns a permissive mask that allows all content.
// Used when a lobe doesn't have a specific mask.
func DefaultMask() *ContextMask {
	return &ContextMask{
		LobeID:      "",
		MaxTokens:   0, // Use full budget
		Description: "Default permissive mask - allows all content",
	}
}
