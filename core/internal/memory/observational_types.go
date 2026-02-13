// Package memory provides memory management for CortexBrain.
// This file defines types for the Observational Memory system (three-tier compression).
package memory

import (
	"context"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// OBSERVATIONAL MEMORY TYPES
// Three-tier memory compression: Messages → Observations → Reflections
// ═══════════════════════════════════════════════════════════════════════════════

// ObservationPriority indicates the importance of an observation.
type ObservationPriority int

const (
	PriorityLow      ObservationPriority = 1
	PriorityNormal   ObservationPriority = 2
	PriorityMedium   ObservationPriority = 3
	PriorityHigh     ObservationPriority = 4
	PriorityCritical ObservationPriority = 5
)

// Message represents a conversation message (Tier 1 - working memory).
type Message struct {
	ID         string    `json:"id"`
	Role       string    `json:"role"`       // user, assistant, system
	Content    string    `json:"content"`
	Timestamp  time.Time `json:"timestamp"`
	ThreadID   string    `json:"thread_id"`
	ResourceID string    `json:"resource_id"` // User or agent identifier
	TokenCount int       `json:"token_count"`

	// Metadata
	Compressed bool   `json:"compressed"` // True if compressed into observation
	ObsID      string `json:"obs_id,omitempty"` // ID of observation that absorbed this
}

// Observation represents a compressed memory unit (Tier 2).
// Created by the Observer Agent when message token count exceeds threshold.
type Observation struct {
	ID          string              `json:"id"`
	Content     string              `json:"content"`      // Compressed observation text
	Timestamp   time.Time           `json:"timestamp"`
	Priority    ObservationPriority `json:"priority"`     // 1-5, higher = more important
	TaskState   string              `json:"task_state"`   // Current task context
	SourceRange []string            `json:"source_range"` // Message IDs that were compressed
	ThreadID    string              `json:"thread_id"`
	ResourceID  string              `json:"resource_id"`
	TokenCount  int                 `json:"token_count"`

	// Lifecycle
	Analyzed  bool      `json:"analyzed"`            // True if analyzed by Distillation Agent
	Reflected bool      `json:"reflected"`           // True if consolidated into reflection
	RefID     string    `json:"ref_id,omitempty"`    // ID of reflection that absorbed this
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Reflection represents a high-level pattern or insight (Tier 3).
// Created by the Reflector Agent when observation token count exceeds threshold.
type Reflection struct {
	ID         string    `json:"id"`
	Content    string    `json:"content"`
	Timestamp  time.Time `json:"timestamp"`
	Pattern    string    `json:"pattern"`      // Type of pattern identified
	SourceObs  []string  `json:"source_obs"`   // Observation IDs consolidated
	ResourceID string    `json:"resource_id"`
	TokenCount int       `json:"token_count"`

	// Lifecycle
	Analyzed  bool      `json:"analyzed"`   // True if analyzed by Distillation Agent
	SkillID   string    `json:"skill_id,omitempty"` // If distilled into a skill
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// CONFIGURATION
// ═══════════════════════════════════════════════════════════════════════════════

// ObservationalMemoryConfig configures the observational memory system.
type ObservationalMemoryConfig struct {
	// Token thresholds
	MessageThreshold     int `yaml:"message_threshold"`     // Default: 30000
	ObservationThreshold int `yaml:"observation_threshold"` // Default: 40000

	// Model configuration
	ObserverModel  string `yaml:"observer_model"`  // Default: gemini-2.5-flash
	ReflectorModel string `yaml:"reflector_model"` // Default: gemini-2.5-flash

	// Scope: "thread" (per-conversation) or "resource" (per-user/agent)
	Scope string `yaml:"scope"` // Default: resource

	// Background agent intervals
	ObserverInterval  time.Duration `yaml:"observer_interval"`  // Default: 10s
	ReflectorInterval time.Duration `yaml:"reflector_interval"` // Default: 30s

	// Storage backend
	WorkingStorage  string `yaml:"working_storage"`  // sqlite (default)
	EpisodicStorage string `yaml:"episodic_storage"` // memvid (default)

	// Memvid configuration
	MemvidDataDir   string `yaml:"memvid_data_dir"`   // Default: ./data/episodic
	MemvidEmbedding string `yaml:"memvid_embedding"`  // Default: bge-small-en-v1.5

	// Retention
	MaxObservationsPerAgent int `yaml:"max_observations_per_agent"` // Default: 1000
	MaxReflectionsPerAgent  int `yaml:"max_reflections_per_agent"`  // Default: 100
	TTLDays                 int `yaml:"ttl_days"`                   // Default: 90
}

// DefaultObservationalMemoryConfig returns sensible defaults.
func DefaultObservationalMemoryConfig() *ObservationalMemoryConfig {
	return &ObservationalMemoryConfig{
		MessageThreshold:        30000,
		ObservationThreshold:    40000,
		ObserverModel:           "gemini-2.5-flash",
		ReflectorModel:          "gemini-2.5-flash",
		Scope:                   "resource",
		ObserverInterval:        10 * time.Second,
		ReflectorInterval:       30 * time.Second,
		WorkingStorage:          "sqlite",
		EpisodicStorage:         "memvid",
		MemvidDataDir:           "./data/episodic",
		MemvidEmbedding:         "bge-small-en-v1.5",
		MaxObservationsPerAgent: 1000,
		MaxReflectionsPerAgent:  100,
		TTLDays:                 90,
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// MEMORY CONTEXT
// ═══════════════════════════════════════════════════════════════════════════════

// ObservationalContext holds the three tiers for prompt injection.
type ObservationalContext struct {
	// Tier 1: Recent messages (working memory)
	Messages []*Message `json:"messages"`

	// Tier 2: Compressed observations
	Observations []*Observation `json:"observations"`

	// Tier 3: High-level reflections
	Reflections []*Reflection `json:"reflections"`

	// Token counts
	MessageTokens     int `json:"message_tokens"`
	ObservationTokens int `json:"observation_tokens"`
	ReflectionTokens  int `json:"reflection_tokens"`
	TotalTokens       int `json:"total_tokens"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// STORAGE INTERFACE
// ═══════════════════════════════════════════════════════════════════════════════

// ObservationalStore defines the storage interface for observational memory.
type ObservationalStore interface {
	// Messages (Tier 1)
	StoreMessage(ctx context.Context, msg *Message) error
	GetMessages(ctx context.Context, threadID, resourceID string, limit int) ([]*Message, error)
	GetMessageTokenCount(ctx context.Context, threadID, resourceID string) (int, error)
	MarkMessagesCompressed(ctx context.Context, messageIDs []string, obsID string) error
	DeleteMessages(ctx context.Context, messageIDs []string) error

	// Observations (Tier 2)
	StoreObservation(ctx context.Context, obs *Observation) error
	GetObservations(ctx context.Context, resourceID string, limit int) ([]*Observation, error)
	GetObservationTokenCount(ctx context.Context, resourceID string) (int, error)
	GetUnanalyzedObservations(ctx context.Context, resourceID string, minCount int) ([]*Observation, error)
	MarkObservationsAnalyzed(ctx context.Context, obsIDs []string) error
	MarkObservationsReflected(ctx context.Context, obsIDs []string, refID string) error

	// Reflections (Tier 3)
	StoreReflection(ctx context.Context, ref *Reflection) error
	GetReflections(ctx context.Context, resourceID string, limit int) ([]*Reflection, error)
	GetUnanalyzedReflections(ctx context.Context, resourceID string, minCount int) ([]*Reflection, error)
	MarkReflectionsAnalyzed(ctx context.Context, refIDs []string, skillID string) error

	// Search
	SearchMemory(ctx context.Context, resourceID, query string, limit int) (*ObservationalContext, error)

	// Timeline (time-travel queries)
	GetTimeline(ctx context.Context, resourceID string, from, to time.Time) (*ObservationalContext, error)

	// Export
	ExportMemory(ctx context.Context, resourceID string) (string, error) // Returns path to .mv2 file
}
