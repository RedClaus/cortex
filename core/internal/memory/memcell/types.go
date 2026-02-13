// Package memcell provides atomic, structured memory units for CortexBrain.
// Inspired by EverMemOS, MemCells transform flat storage into a hierarchically
// organized, relationally-linked knowledge graph aligned with hippocampal
// memory encoding principles.
//
// CR-027: MemCell Atomic Memory Structure
package memcell

import (
	"context"
	"time"
)

// ══════════════════════════════════════════════════════════════════════════════
// MEMORY TYPE TAXONOMY
// 15 types across 5 categories: episodic, semantic, personal, strategic, contextual
// ══════════════════════════════════════════════════════════════════════════════

// MemoryType categorizes the nature of a memory.
type MemoryType string

const (
	// Episodic memories - what happened
	MemTypeEpisode     MemoryType = "episode"     // Conversation segment
	MemTypeEvent       MemoryType = "event"       // Discrete occurrence
	MemTypeInteraction MemoryType = "interaction" // User interaction

	// Semantic memories - what we know
	MemTypeFact      MemoryType = "fact"      // Verified information
	MemTypeKnowledge MemoryType = "knowledge" // Domain knowledge
	MemTypeProcedure MemoryType = "procedure" // How-to information

	// Personal memories - who the user is
	MemTypePreference   MemoryType = "preference"   // User preferences
	MemTypeProfile      MemoryType = "profile"      // User characteristics
	MemTypeRelationship MemoryType = "relationship" // User relationships

	// Strategic memories - guiding principles
	MemTypePrinciple MemoryType = "principle" // Learned principle
	MemTypeLesson    MemoryType = "lesson"    // Failure-derived insight
	MemTypeGoal      MemoryType = "goal"      // User/system goals

	// Contextual memories - situational awareness
	MemTypeContext MemoryType = "context" // Situational context
	MemTypeProject MemoryType = "project" // Project-specific
	MemTypeMood    MemoryType = "mood"    // Emotional state
)

// String returns the string representation of the memory type.
func (t MemoryType) String() string {
	return string(t)
}

// Category returns the high-level category of this memory type.
func (t MemoryType) Category() string {
	switch t {
	case MemTypeEpisode, MemTypeEvent, MemTypeInteraction:
		return "episodic"
	case MemTypeFact, MemTypeKnowledge, MemTypeProcedure:
		return "semantic"
	case MemTypePreference, MemTypeProfile, MemTypeRelationship:
		return "personal"
	case MemTypePrinciple, MemTypeLesson, MemTypeGoal:
		return "strategic"
	case MemTypeContext, MemTypeProject, MemTypeMood:
		return "contextual"
	default:
		return "unknown"
	}
}

// IsValid returns true if the memory type is recognized.
func (t MemoryType) IsValid() bool {
	switch t {
	case MemTypeEpisode, MemTypeEvent, MemTypeInteraction,
		MemTypeFact, MemTypeKnowledge, MemTypeProcedure,
		MemTypePreference, MemTypeProfile, MemTypeRelationship,
		MemTypePrinciple, MemTypeLesson, MemTypeGoal,
		MemTypeContext, MemTypeProject, MemTypeMood:
		return true
	default:
		return false
	}
}

// AllMemoryTypes returns all valid memory types.
func AllMemoryTypes() []MemoryType {
	return []MemoryType{
		MemTypeEpisode, MemTypeEvent, MemTypeInteraction,
		MemTypeFact, MemTypeKnowledge, MemTypeProcedure,
		MemTypePreference, MemTypeProfile, MemTypeRelationship,
		MemTypePrinciple, MemTypeLesson, MemTypeGoal,
		MemTypeContext, MemTypeProject, MemTypeMood,
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// RELATION TYPE
// ══════════════════════════════════════════════════════════════════════════════

// RelationType defines the nature of a relationship between MemCells.
type RelationType string

const (
	RelTypeRelated     RelationType = "related"     // Semantic similarity
	RelTypeContradicts RelationType = "contradicts" // Conflicting information
	RelTypeSupports    RelationType = "supports"    // Supporting evidence
	RelTypeChild       RelationType = "child"       // Hierarchical
	RelTypeCauses      RelationType = "causes"      // Causal relationship
	RelTypePrecedes    RelationType = "precedes"    // Temporal ordering
	RelTypeElaborates  RelationType = "elaborates"  // More detail on same topic
)

// Relation represents a link between two MemCells.
type Relation struct {
	FromID   string       `json:"from_id" db:"from_id"`
	ToID     string       `json:"to_id" db:"to_id"`
	Type     RelationType `json:"relation_type" db:"relation_type"`
	Strength float64      `json:"strength" db:"strength"` // 0 to 1
}

// ══════════════════════════════════════════════════════════════════════════════
// SCOPE
// ══════════════════════════════════════════════════════════════════════════════

// Scope defines the visibility/ownership of a memory.
type Scope string

const (
	ScopePersonal Scope = "personal" // User-specific
	ScopeTeam     Scope = "team"     // Shared within team
	ScopeGlobal   Scope = "global"   // System-wide knowledge
)

// ══════════════════════════════════════════════════════════════════════════════
// MEMCELL - THE ATOMIC MEMORY UNIT
// ══════════════════════════════════════════════════════════════════════════════

// MemCell is the atomic unit of memory in CortexBrain.
// It consists of 5 layers: Identity, Content, Classification, Relational, Context.
type MemCell struct {
	// ─────────────────────────────────────────────────────────────────────────
	// Identity Layer
	// ─────────────────────────────────────────────────────────────────────────
	ID           string    `json:"id" db:"id"`
	SourceID     string    `json:"source_id,omitempty" db:"source_id"`     // Original message/doc ID
	Version      int       `json:"version" db:"version"`                   // For versioning
	CreatedAt    time.Time `json:"created_at" db:"created_at"`             // Creation timestamp
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`             // Last modified
	LastAccessAt time.Time `json:"last_access_at,omitempty" db:"last_access_at"` // Last retrieval
	AccessCount  int       `json:"access_count" db:"access_count"`         // Retrieval count

	// ─────────────────────────────────────────────────────────────────────────
	// Content Layer
	// ─────────────────────────────────────────────────────────────────────────
	RawContent string    `json:"raw_content" db:"raw_content"` // Original text
	Summary    string    `json:"summary,omitempty" db:"summary"` // Compressed summary
	Embedding  []float32 `json:"embedding,omitempty"`          // Vector (stored in LEANN)
	Entities   []string  `json:"entities,omitempty"`           // Extracted entities
	KeyPhrases []string  `json:"key_phrases,omitempty"`        // Key phrases
	Sentiment  float64   `json:"sentiment" db:"sentiment"`     // -1 to 1

	// ─────────────────────────────────────────────────────────────────────────
	// Classification Layer
	// ─────────────────────────────────────────────────────────────────────────
	MemoryType MemoryType `json:"memory_type" db:"memory_type"` // One of 15 types
	Confidence float64    `json:"confidence" db:"confidence"`   // Classification confidence (0-1)
	Importance float64    `json:"importance" db:"importance"`   // Importance score (0-1)
	Topics     []string   `json:"topics,omitempty"`             // Topic tags
	Scope      Scope      `json:"scope" db:"scope"`             // Visibility scope

	// ─────────────────────────────────────────────────────────────────────────
	// Relational Layer
	// ─────────────────────────────────────────────────────────────────────────
	ParentID      string             `json:"parent_id,omitempty" db:"parent_id"`       // Parent cell
	ChildIDs      []string           `json:"child_ids,omitempty"`                      // Child cells
	RelatedIDs    []string           `json:"related_ids,omitempty"`                    // Related cells
	ContradictsIDs []string          `json:"contradicts_ids,omitempty"`                // Contradicting cells
	SupportsIDs   []string           `json:"supports_ids,omitempty"`                   // Supporting cells
	SupersedesID  string             `json:"supersedes_id,omitempty" db:"supersedes_id"` // Cell this replaces
	EpisodeID     string             `json:"episode_id,omitempty" db:"episode_id"`     // Episode grouping
	LinkStrengths map[string]float64 `json:"link_strengths,omitempty"`                 // ID -> strength

	// ─────────────────────────────────────────────────────────────────────────
	// Context Layer
	// ─────────────────────────────────────────────────────────────────────────
	EventBoundary  bool   `json:"event_boundary" db:"event_boundary"`           // Episode boundary marker
	PrecedingCtx   string `json:"preceding_ctx,omitempty" db:"preceding_ctx"`   // Context before
	FollowingCtx   string `json:"following_ctx,omitempty" db:"following_ctx"`   // Context after
	ConversationID string `json:"conversation_id,omitempty" db:"conversation_id"` // Conversation
	TurnNumber     int    `json:"turn_number,omitempty" db:"turn_number"`       // Turn in conversation
	UserState      string `json:"user_state,omitempty" db:"user_state"`         // JSON state snapshot
}

// ══════════════════════════════════════════════════════════════════════════════
// CONVERSATION TURN (Input to Extractor)
// ══════════════════════════════════════════════════════════════════════════════

// ConversationTurn represents a single turn in a conversation.
type ConversationTurn struct {
	ID             string    `json:"id"`
	Role           string    `json:"role"` // "user" or "assistant"
	Content        string    `json:"content"`
	Timestamp      time.Time `json:"timestamp"`
	ConversationID string    `json:"conversation_id"`
	TurnNumber     int       `json:"turn_number"`
}

// ══════════════════════════════════════════════════════════════════════════════
// SEARCH OPTIONS
// ══════════════════════════════════════════════════════════════════════════════

// SearchOptions configures MemCell retrieval.
type SearchOptions struct {
	TopK           int          `json:"top_k"`            // Max results
	MinImportance  float64      `json:"min_importance"`   // Importance threshold
	MinConfidence  float64      `json:"min_confidence"`   // Confidence threshold
	MemoryTypes    []MemoryType `json:"memory_types"`     // Filter by types
	Scope          Scope        `json:"scope"`            // Filter by scope
	EpisodeID      string       `json:"episode_id"`       // Filter by episode
	ConversationID string       `json:"conversation_id"`  // Filter by conversation
	IncludeRelated bool         `json:"include_related"`  // Expand with relations
	RelationDepth  int          `json:"relation_depth"`   // How deep to traverse
	SinceTime      time.Time    `json:"since_time"`       // Time filter
}

// DefaultSearchOptions returns sensible defaults for search.
func DefaultSearchOptions() SearchOptions {
	return SearchOptions{
		TopK:           10,
		MinImportance:  0.0,
		MinConfidence:  0.0,
		IncludeRelated: false,
		RelationDepth:  1,
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// SEARCH RESULT
// ══════════════════════════════════════════════════════════════════════════════

// SearchResult wraps a MemCell with search metadata.
type SearchResult struct {
	Cell      *MemCell  `json:"cell"`
	Score     float64   `json:"score"`      // Relevance score (0-1)
	MatchType string    `json:"match_type"` // "semantic", "keyword", "hybrid"
	Relations []Relation `json:"relations,omitempty"` // Related links
}

// ══════════════════════════════════════════════════════════════════════════════
// EXTRACTOR INTERFACE
// ══════════════════════════════════════════════════════════════════════════════

// Extractor extracts MemCells from raw content.
type Extractor interface {
	// Extract creates MemCells from conversation turns.
	Extract(ctx context.Context, turns []ConversationTurn) ([]MemCell, error)

	// Classify determines the memory type of content.
	Classify(ctx context.Context, content string) (MemoryType, float64, error)

	// DetectBoundary checks if content marks an event boundary.
	DetectBoundary(ctx context.Context, prev, current string) (bool, error)

	// ExtractEntities extracts named entities from content.
	ExtractEntities(ctx context.Context, content string) ([]string, error)

	// ComputeImportance calculates importance score.
	ComputeImportance(ctx context.Context, cell *MemCell) (float64, error)

	// GenerateSummary creates a compressed summary.
	GenerateSummary(ctx context.Context, content string) (string, error)
}

// ══════════════════════════════════════════════════════════════════════════════
// STORE INTERFACE
// ══════════════════════════════════════════════════════════════════════════════

// Store provides CRUD operations for MemCells.
type Store interface {
	// Create stores a new MemCell.
	Create(ctx context.Context, cell *MemCell) error

	// Get retrieves a MemCell by ID.
	Get(ctx context.Context, id string) (*MemCell, error)

	// Update modifies an existing MemCell.
	Update(ctx context.Context, cell *MemCell) error

	// Delete removes a MemCell.
	Delete(ctx context.Context, id string) error

	// Search finds MemCells by query.
	Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error)

	// GetRelated retrieves related MemCells to a given depth.
	GetRelated(ctx context.Context, id string, depth int) ([]MemCell, error)

	// AddRelation creates a link between two MemCells.
	AddRelation(ctx context.Context, from, to string, relType RelationType, strength float64) error

	// GetByEpisode retrieves all MemCells in an episode.
	GetByEpisode(ctx context.Context, episodeID string) ([]MemCell, error)

	// GetByType retrieves MemCells of a specific type.
	GetByType(ctx context.Context, memType MemoryType, opts SearchOptions) ([]MemCell, error)

	// RecordAccess updates access statistics.
	RecordAccess(ctx context.Context, id string) error
}

// ══════════════════════════════════════════════════════════════════════════════
// BOUNDARY DETECTION CONFIG
// ══════════════════════════════════════════════════════════════════════════════

// BoundaryConfig configures event boundary detection.
type BoundaryConfig struct {
	// EmbeddingThreshold: if cosine distance exceeds this, it's a boundary
	EmbeddingThreshold float64 `json:"embedding_threshold"`

	// TimeGap: if gap between turns exceeds this, it's a boundary
	TimeGap time.Duration `json:"time_gap"`

	// TransitionPatterns: phrases that indicate topic change
	TransitionPatterns []string `json:"transition_patterns"`
}

// DefaultBoundaryConfig returns sensible defaults for boundary detection.
func DefaultBoundaryConfig() BoundaryConfig {
	return BoundaryConfig{
		EmbeddingThreshold: 0.4, // 40% dissimilarity = boundary
		TimeGap:            30 * time.Minute,
		TransitionPatterns: []string{
			"anyway", "by the way", "speaking of", "now",
			"that reminds me", "on another note", "so",
			"moving on", "let's talk about", "changing topic",
		},
	}
}
