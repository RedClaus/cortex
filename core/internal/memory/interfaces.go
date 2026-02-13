// Package memory provides enhanced memory capabilities for Cortex.
// This file defines the core interfaces used by the memory enhancement components.
package memory

import (
	"context"
)

// Embedder generates vector embeddings for text.
// Implementations should use a consistent embedding model (e.g., nomic-embed-text).
type Embedder interface {
	// Embed generates a vector embedding for the given text.
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedFast generates an embedding with a short timeout (5 seconds).
	// Returns an error if embedding takes too long, allowing fallback to FTS.
	EmbedFast(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch generates embeddings for multiple texts efficiently.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// Dimension returns the embedding dimension (e.g., 768 for nomic-embed-text).
	Dimension() int

	// ModelName returns the name of the embedding model.
	ModelName() string
}

// LLMProvider makes completion calls for extraction and classification.
// Used for principle extraction, topic naming, and link classification.
type LLMProvider interface {
	// Complete sends a prompt and returns the completion.
	Complete(ctx context.Context, prompt string) (string, error)
}

// MemoryType represents the type of memory for categorization.
type MemoryType string

const (
	// MemoryTypeEpisodic represents episodic memories (what happened).
	MemoryTypeEpisodic MemoryType = "episodic"

	// MemoryTypeProcedural represents procedural memories (how to do it).
	MemoryTypeProcedural MemoryType = "procedural"

	// MemoryTypeStrategic represents strategic memories (why/when to apply).
	MemoryTypeStrategic MemoryType = "strategic"

	// MemoryTypeSemantic represents semantic memories (general knowledge).
	MemoryTypeSemantic MemoryType = "semantic"
)

// LinkType represents the type of relationship between memories.
type LinkType string

const (
	// LinkContradicts indicates new info contradicts old info.
	LinkContradicts LinkType = "contradicts"

	// LinkSupports indicates evidence strengthening a fact.
	LinkSupports LinkType = "supports"

	// LinkEvolvedFrom indicates an updated version of a fact.
	LinkEvolvedFrom LinkType = "evolved_from"

	// LinkRelatedTo indicates a general topical relationship.
	LinkRelatedTo LinkType = "related_to"

	// LinkCausedBy indicates a causal relationship.
	LinkCausedBy LinkType = "caused_by"

	// LinkLeadsTo indicates a sequential relationship.
	LinkLeadsTo LinkType = "leads_to"

	// LinkRoutingDecision indicates query pattern to model success relationship.
	LinkRoutingDecision LinkType = "routing_decision"

	// LinkCapabilityScore indicates model to task capability relationship.
	LinkCapabilityScore LinkType = "capability_score"

	// LinkContextWindow indicates user context to preference relationship.
	LinkContextWindow LinkType = "context_window"

	// LinkTaskAffinity indicates task type to model affinity relationship.
	LinkTaskAffinity LinkType = "task_affinity"
)

// ValidLinkTypes returns all valid link types.
func ValidLinkTypes() []LinkType {
	return []LinkType{
		LinkContradicts,
		LinkSupports,
		LinkEvolvedFrom,
		LinkRelatedTo,
		LinkCausedBy,
		LinkLeadsTo,
		LinkRoutingDecision,
		LinkCapabilityScore,
		LinkContextWindow,
		LinkTaskAffinity,
	}
}

// SessionOutcome represents the outcome of a session for learning.
type SessionOutcome struct {
	// SessionID is the unique identifier for the session.
	SessionID string

	// Goal is what the user was trying to accomplish.
	Goal string

	// GoalCompleted indicates whether the goal was achieved.
	GoalCompleted bool

	// Commands is the list of commands executed.
	Commands []string

	// Errors is the list of error messages encountered.
	Errors []string

	// Outcome is a description of the final state.
	Outcome string

	// Sentiment is the overall sentiment of the session (-1 to 1).
	Sentiment float64

	// PrinciplesApplied is the list of principle IDs that were followed.
	PrinciplesApplied []string

	// PrinciplesIgnored is the list of principle IDs that were ignored.
	PrinciplesIgnored []string
}

// GenericMemory is a unified representation for linking across memory types.
type GenericMemory struct {
	// ID is the unique identifier.
	ID string

	// Type is the memory type (episodic, procedural, strategic).
	Type MemoryType

	// Content is the text content of the memory.
	Content string

	// Embedding is the vector representation.
	Embedding []float32

	// Metadata contains type-specific additional data.
	Metadata map[string]any
}
