// Package memory provides enhanced memory capabilities for Cortex.
// This file implements the MemCube abstraction for CR-025.
// MemCube is the atomic unit of memory storage, inspired by MemOS (arXiv:2505.22101).
package memory

import (
	"time"

	"github.com/google/uuid"
)

// CubeType categorizes memory content.
type CubeType string

const (
	// CubeTypeText represents plain knowledge/information.
	CubeTypeText CubeType = "text"

	// CubeTypeSkill represents an executable pattern (Voyager-style).
	CubeTypeSkill CubeType = "skill"

	// CubeTypeTool represents a tool execution trajectory.
	CubeTypeTool CubeType = "tool"
)

// ValidCubeTypes returns all valid cube types.
func ValidCubeTypes() []CubeType {
	return []CubeType{CubeTypeText, CubeTypeSkill, CubeTypeTool}
}

// CubeLink represents a relationship between cubes.
type CubeLink struct {
	// TargetID is the ID of the linked MemCube.
	TargetID string `json:"target_id"`

	// RelType describes the relationship: "derived_from", "supports", "contradicts", "related_to".
	RelType string `json:"rel_type"`

	// Confidence is how confident we are in this relationship (0-1).
	Confidence float64 `json:"confidence"`
}

// MemCube is the atomic unit of memory storage, inspired by MemOS.
// It encapsulates content + metadata for unified memory governance.
type MemCube struct {
	// ============================================================================
	// IDENTITY
	// ============================================================================

	// ID is the unique identifier for this cube.
	ID string `json:"id"`

	// Version tracks revisions of this cube (incremented on updates).
	Version int `json:"version"`

	// CreatedAt is when this cube was first created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when this cube was last modified.
	UpdatedAt time.Time `json:"updated_at"`

	// ============================================================================
	// CONTENT PAYLOAD
	// ============================================================================

	// Content is the main content of this memory cube.
	Content string `json:"content"`

	// ContentType categorizes the content: text, skill, or tool.
	ContentType CubeType `json:"content_type"`

	// Embedding is the vector representation for similarity search.
	Embedding []float32 `json:"embedding,omitempty"`

	// ============================================================================
	// PROVENANCE (where did this come from?)
	// ============================================================================

	// Source describes the origin: "conversation", "execution", "synthesis".
	Source string `json:"source"`

	// SessionID links this cube to a specific session.
	SessionID string `json:"session_id"`

	// ParentID references the cube this was derived from (for forked/evolved cubes).
	ParentID string `json:"parent_id,omitempty"`

	// CreatedBy identifies the creator: "user", "system", "lobe:reasoning", etc.
	CreatedBy string `json:"created_by"`

	// ============================================================================
	// QUALITY SIGNALS
	// ============================================================================

	// Confidence is how confident we are in this memory's accuracy (0-1).
	Confidence float64 `json:"confidence"`

	// SuccessCount tracks how many times using this memory led to success.
	SuccessCount int `json:"success_count"`

	// FailureCount tracks how many times using this memory led to failure.
	FailureCount int `json:"failure_count"`

	// LastAccessedAt is when this cube was last retrieved/used.
	LastAccessedAt time.Time `json:"last_accessed_at"`

	// AccessCount tracks total number of times this cube was accessed.
	AccessCount int `json:"access_count"`

	// ============================================================================
	// RELATIONSHIPS
	// ============================================================================

	// Links are connections to other cubes (loaded separately, not persisted inline).
	Links []CubeLink `json:"links,omitempty"`

	// ============================================================================
	// ACCESS CONTROL (for future multi-user support)
	// ============================================================================

	// Scope defines visibility: "personal", "team", "global".
	Scope string `json:"scope"`
}

// NewMemCube creates a new MemCube with sensible defaults.
func NewMemCube(content string, contentType CubeType, source string) *MemCube {
	now := time.Now()
	return &MemCube{
		ID:          uuid.New().String(),
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
		Content:     content,
		ContentType: contentType,
		Source:      source,
		CreatedBy:   "system",
		Confidence:  0.5, // Neutral prior
		Scope:       "personal",
	}
}

// SuccessRate returns the success ratio for this memory.
// Uses a neutral prior (0.5) when there's no evidence.
func (mc *MemCube) SuccessRate() float64 {
	total := mc.SuccessCount + mc.FailureCount
	if total == 0 {
		return 0.5 // Neutral prior
	}
	return float64(mc.SuccessCount) / float64(total)
}

// Touch updates access timestamp and count.
// Call this when the cube is retrieved for use.
func (mc *MemCube) Touch() {
	mc.LastAccessedAt = time.Now()
	mc.AccessCount++
}

// RecordSuccess increments the success count and updates timestamps.
func (mc *MemCube) RecordSuccess() {
	mc.SuccessCount++
	mc.UpdatedAt = time.Now()
}

// RecordFailure increments the failure count and updates timestamps.
func (mc *MemCube) RecordFailure() {
	mc.FailureCount++
	mc.UpdatedAt = time.Now()
}

// Fork creates a new version derived from this cube.
// The new cube has a fresh ID, references this cube as parent,
// and starts with slightly lower confidence.
func (mc *MemCube) Fork(newContent string, source string) *MemCube {
	now := time.Now()
	return &MemCube{
		ID:          uuid.New().String(),
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
		Content:     newContent,
		ContentType: mc.ContentType,
		Source:      source,
		SessionID:   mc.SessionID,
		ParentID:    mc.ID,
		CreatedBy:   "system",
		Confidence:  mc.Confidence * 0.9, // Slightly lower confidence for derived
		Scope:       "personal",
	}
}

// IsReliable returns true if this cube has enough evidence to be considered reliable.
// Requires at least 3 observations and a success rate >= 70%.
func (mc *MemCube) IsReliable() bool {
	total := mc.SuccessCount + mc.FailureCount
	return total >= MinEvidenceForReliable && mc.SuccessRate() >= 0.7
}

// HasEmbedding returns true if this cube has a valid embedding.
func (mc *MemCube) HasEmbedding() bool {
	return len(mc.Embedding) > 0
}

// TotalObservations returns the total number of success + failure observations.
func (mc *MemCube) TotalObservations() int {
	return mc.SuccessCount + mc.FailureCount
}

// AddLink adds a link to another cube.
// This is for in-memory use; links are persisted separately in memcube_links table.
func (mc *MemCube) AddLink(targetID string, relType string, confidence float64) {
	mc.Links = append(mc.Links, CubeLink{
		TargetID:   targetID,
		RelType:    relType,
		Confidence: confidence,
	})
}

// GetLinksOfType returns all links of a specific relationship type.
func (mc *MemCube) GetLinksOfType(relType string) []CubeLink {
	var result []CubeLink
	for _, link := range mc.Links {
		if link.RelType == relType {
			result = append(result, link)
		}
	}
	return result
}
