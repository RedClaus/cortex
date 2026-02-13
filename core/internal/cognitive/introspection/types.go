// Package introspection provides metacognitive self-awareness capabilities.
// CR-018: Enables Cortex to reason about its own knowledge, honestly assess gaps,
// and autonomously acquire new knowledge when needed.
package introspection

import (
	"context"
)

// ============================================================================
// LLM PROVIDER INTERFACE
// ============================================================================

// LLMProvider defines the interface for LLM completion calls.
type LLMProvider interface {
	Complete(ctx context.Context, prompt string) (string, error)
}

// ============================================================================
// QUERY TYPES
// ============================================================================

// QueryType represents the type of introspection query.
type QueryType string

const (
	// QueryTypeKnowledgeCheck - "do you know X?", "have you learned X?"
	QueryTypeKnowledgeCheck QueryType = "knowledge_check"

	// QueryTypeCapabilityCheck - "can you help with X?", "are you able to X?"
	QueryTypeCapabilityCheck QueryType = "capability_check"

	// QueryTypeMemoryList - "what do you know?", "list your knowledge"
	QueryTypeMemoryList QueryType = "memory_list"

	// QueryTypeSkillAssessment - "how good are you at X?", "rate your knowledge of X"
	QueryTypeSkillAssessment QueryType = "skill_assessment"

	// QueryTypeNotIntrospective - regular query, not asking about self-knowledge
	QueryTypeNotIntrospective QueryType = "not_introspective"
)

// IntrospectionQuery represents a classified introspection request.
type IntrospectionQuery struct {
	// Type is the classified query type.
	Type QueryType `json:"type"`

	// Subject is the topic the user is asking about (e.g., "linux commands").
	Subject string `json:"subject"`

	// SearchTerms are expanded search terms for inventory queries.
	SearchTerms []string `json:"search_terms"`

	// OriginalQuery is the raw user input.
	OriginalQuery string `json:"original_query"`

	// Confidence is the classification confidence (0-1).
	Confidence float64 `json:"confidence"`

	// Metadata contains additional classification metadata.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ============================================================================
// GAP ANALYSIS
// ============================================================================

// GapSeverity indicates how severe a knowledge gap is.
type GapSeverity string

const (
	// GapSeverityNone - Has good stored knowledge (10+ items).
	GapSeverityNone GapSeverity = "none"

	// GapSeverityMinimal - Has some stored knowledge (1-9 items).
	GapSeverityMinimal GapSeverity = "minimal"

	// GapSeverityModerate - No stored knowledge, but LLM can help.
	GapSeverityModerate GapSeverity = "moderate"

	// GapSeveritySevere - No stored knowledge, LLM has limited capability.
	GapSeveritySevere GapSeverity = "severe"
)

// GapAnalysis represents the result of analyzing a knowledge gap.
type GapAnalysis struct {
	// Subject is the topic being analyzed.
	Subject string `json:"subject"`

	// HasStoredKnowledge indicates if any items were found in memory.
	HasStoredKnowledge bool `json:"has_stored_knowledge"`

	// StoredKnowledgeCount is the number of items found.
	StoredKnowledgeCount int `json:"stored_knowledge_count"`

	// LLMCanAnswer indicates if the LLM can answer from training.
	LLMCanAnswer bool `json:"llm_can_answer"`

	// LLMConfidence is the LLM's self-assessed confidence (0-1).
	LLMConfidence float64 `json:"llm_confidence"`

	// GapSeverity is the overall severity of the knowledge gap.
	GapSeverity GapSeverity `json:"gap_severity"`

	// AcquisitionOptions are ways to fill the knowledge gap.
	AcquisitionOptions []AcquisitionOption `json:"acquisition_options"`

	// RecommendedAction is the suggested next step.
	RecommendedAction string `json:"recommended_action"`
}

// ============================================================================
// ACQUISITION OPTIONS (used by GapAnalysis)
// Note: AcquisitionType, AcquisitionRequest, AcquisitionResult are defined in acquisition.go
// Note: LearningOutcome, TestResult are defined in learning.go
// ============================================================================

// AcquisitionOption represents a way to fill a knowledge gap.
type AcquisitionOption struct {
	// Type is the acquisition method.
	Type AcquisitionType `json:"type"`

	// Description is a human-readable description.
	Description string `json:"description"`

	// Confidence is how likely this method will succeed (0-1).
	Confidence float64 `json:"confidence"`

	// Effort indicates the complexity (low, medium, high).
	Effort string `json:"effort"`
}

// ============================================================================
// RESPONSE GENERATION
// ============================================================================

// ResponseTemplate represents a response template type.
type ResponseTemplate string

const (
	// TemplateKnowledgeFound - found items in memory.
	TemplateKnowledgeFound ResponseTemplate = "knowledge_found"

	// TemplateKnowledgeNotFoundCanAnswer - no stored items, but LLM can help.
	TemplateKnowledgeNotFoundCanAnswer ResponseTemplate = "not_found_can_answer"

	// TemplateKnowledgeNotFoundCannotAnswer - no stored items, LLM limited.
	TemplateKnowledgeNotFoundCannotAnswer ResponseTemplate = "not_found_cannot_answer"

	// TemplateAcquisitionOffer - offer to learn new knowledge.
	TemplateAcquisitionOffer ResponseTemplate = "acquisition_offer"

	// TemplateAcquisitionStarted - acquisition in progress.
	TemplateAcquisitionStarted ResponseTemplate = "acquisition_started"

	// TemplateAcquisitionComplete - acquisition finished successfully.
	TemplateAcquisitionComplete ResponseTemplate = "acquisition_complete"

	// TemplateAcquisitionFailed - acquisition encountered an error.
	TemplateAcquisitionFailed ResponseTemplate = "acquisition_failed"

	// Aliases for backward compatibility
	TemplateNotFoundCanAnswer    = TemplateKnowledgeNotFoundCanAnswer
	TemplateNotFoundCannotAnswer = TemplateKnowledgeNotFoundCannotAnswer
)

// ResponseContext contains data for template rendering.
type ResponseContext struct {
	// Subject is the topic being discussed.
	Subject string

	// MatchCount is the number of items found.
	MatchCount int

	// TopResults are the best matching items.
	TopResults []InventoryItem

	// RelatedTopics are related topic clusters.
	RelatedTopics []string

	// LLMCanAnswer indicates if LLM can help.
	LLMCanAnswer bool

	// LLMConfidence is the LLM's self-assessed confidence.
	LLMConfidence float64

	// AcquisitionOptions are ways to fill gaps.
	AcquisitionOptions []AcquisitionOption

	// AcquisitionType is the type used for acquisition.
	AcquisitionType string

	// ItemsIngested is the count of items learned.
	ItemsIngested int

	// Categories are the knowledge categories.
	Categories []string

	// ErrorMessage is the error if acquisition failed.
	ErrorMessage string
}

// InventoryItem represents a single item in the knowledge inventory.
// This mirrors the structure from internal/memory but avoids circular imports.
type InventoryItem struct {
	// ID is the unique identifier.
	ID string `json:"id"`

	// Source is which memory store it came from.
	Source string `json:"source"`

	// Content is the item content (truncated).
	Content string `json:"content"`

	// Summary is a brief summary.
	Summary string `json:"summary"`

	// Relevance is the relevance score (0-1).
	Relevance float64 `json:"relevance"`

	// Metadata contains additional item metadata.
	Metadata map[string]string `json:"metadata"`
}
