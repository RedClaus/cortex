// Package cognitive provides the cognitive architecture layer for Cortex.
// This file defines types for the Dynamic Skill Registry and Failure Library.
package cognitive

import (
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// DYNAMIC SKILL TYPES
// Skills auto-generated through Skill Distillation from Observational Memory
// ═══════════════════════════════════════════════════════════════════════════════

// SkillType represents the skill hierarchy level (inspired by SkillRL).
type SkillType string

const (
	// SkillTypeGeneral represents universal patterns applicable to any task (L1).
	SkillTypeGeneral SkillType = "GENERAL"

	// SkillTypeTaskSpecific represents category-level patterns (L2).
	SkillTypeTaskSpecific SkillType = "TASK_SPECIFIC"

	// SkillTypeAgentSpecific represents patterns only relevant to one agent (not shared).
	SkillTypeAgentSpecific SkillType = "AGENT_SPECIFIC"
)

// SkillSource indicates where the skill originated.
type SkillSource string

const (
	// SkillSourceStatic indicates skill from config/skills/*.md (original 43 Octopus skills).
	SkillSourceStatic SkillSource = "STATIC"

	// SkillSourceDistilled indicates auto-generated skill from Skill Distillation.
	SkillSourceDistilled SkillSource = "DISTILLED"

	// SkillSourceMerged indicates skill combined from multiple similar skills.
	SkillSourceMerged SkillSource = "MERGED"

	// SkillSourceShared indicates skill adopted from another agent via cross-agent learning.
	SkillSourceShared SkillSource = "SHARED"
)

// SkillStatus represents the lifecycle state of a dynamic skill.
type SkillStatus string

const (
	// SkillStatusProbation indicates newly created skill needs validation.
	SkillStatusProbation SkillStatus = "probation"

	// SkillStatusActive indicates validated, usable skill.
	SkillStatusActive SkillStatus = "active"

	// SkillStatusDeprecated indicates skill removed due to low performance.
	SkillStatusDeprecated SkillStatus = "deprecated"
)

// DynamicSkill represents an auto-generated skill from experience distillation.
type DynamicSkill struct {
	// Identity
	ID       string      `json:"id" yaml:"id"`
	Name     string      `json:"name" yaml:"name"`
	Type     SkillType   `json:"type" yaml:"type"`
	Category string      `json:"category,omitempty" yaml:"category,omitempty"` // For TASK_SPECIFIC
	Source   SkillSource `json:"source" yaml:"source"`
	Status   SkillStatus `json:"status" yaml:"status"`

	// Content
	Description string   `json:"description" yaml:"description"`
	WhenToApply string   `json:"when_to_apply" yaml:"when_to_apply"`
	Steps       []string `json:"steps,omitempty" yaml:"steps,omitempty"`
	Examples    []string `json:"examples,omitempty" yaml:"examples,omitempty"`

	// Matching (for routing)
	Intent          string    `json:"intent,omitempty" yaml:"intent,omitempty"`
	IntentEmbedding Embedding `json:"-" yaml:"-"` // Not serialized to YAML
	Keywords        []string  `json:"keywords,omitempty" yaml:"keywords,omitempty"`

	// Provenance
	SourceReflections []string  `json:"source_reflections" yaml:"source_reflections"`
	SourceAgents      []string  `json:"source_agents" yaml:"source_agents"`
	Confidence        float64   `json:"confidence" yaml:"confidence"`
	CreatedAt         time.Time `json:"created_at" yaml:"created_at"`
	UpdatedAt         time.Time `json:"updated_at" yaml:"updated_at"`

	// Usage tracking
	UsageCount int       `json:"usage_count" yaml:"usage_count"`
	LastUsed   time.Time `json:"last_used,omitempty" yaml:"last_used,omitempty"`
	Successes  int       `json:"successes" yaml:"successes"`
	Failures   int       `json:"failures" yaml:"failures"`

	// Version control
	Version  int    `json:"version" yaml:"version"`
	ParentID string `json:"parent_id,omitempty" yaml:"parent_id,omitempty"` // If evolved from another
}

// SuccessRate returns the success rate as a percentage (0-100).
func (s *DynamicSkill) SuccessRate() float64 {
	total := s.Successes + s.Failures
	if total == 0 {
		return 50.0 // Default to 50% when no data
	}
	return float64(s.Successes) / float64(total) * 100.0
}

// IsActive returns true if the skill should be used for routing.
func (s *DynamicSkill) IsActive() bool {
	return s.Status == SkillStatusActive || s.Status == SkillStatusProbation
}

// ═══════════════════════════════════════════════════════════════════════════════
// FAILURE PATTERN TYPES
// Anti-patterns extracted from failed interactions
// ═══════════════════════════════════════════════════════════════════════════════

// FailurePattern represents a "what NOT to do" rule.
type FailurePattern struct {
	// Identity
	ID       string    `json:"id" yaml:"id"`
	Name     string    `json:"name" yaml:"name"`
	Type     SkillType `json:"type" yaml:"type"`
	Category string    `json:"category,omitempty" yaml:"category,omitempty"`

	// Content
	Description    string `json:"description" yaml:"description"`
	ErrorSignature string `json:"error_signature" yaml:"error_signature"` // How to detect
	Recovery       string `json:"recovery" yaml:"recovery"`               // How to fix
	Prevention     string `json:"prevention" yaml:"prevention"`           // How to avoid

	// Matching
	Keywords        []string  `json:"keywords,omitempty" yaml:"keywords,omitempty"`
	ErrorEmbedding  Embedding `json:"-" yaml:"-"` // For semantic matching

	// Provenance
	SourceReflections []string  `json:"source_reflections" yaml:"source_reflections"`
	SourceAgents      []string  `json:"source_agents" yaml:"source_agents"`
	Confidence        float64   `json:"confidence" yaml:"confidence"`
	CreatedAt         time.Time `json:"created_at" yaml:"created_at"`

	// Tracking
	TimesTriggered int `json:"times_triggered" yaml:"times_triggered"`
	TimesPrevented int `json:"times_prevented" yaml:"times_prevented"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// SKILL MATCH TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// SkillMatch represents a matched dynamic skill with its similarity score.
type SkillMatch struct {
	Skill           *DynamicSkill   `json:"skill"`
	SimilarityScore float64         `json:"similarity_score"`
	SimilarityLevel SimilarityLevel `json:"similarity_level"`
	MatchMethod     string          `json:"match_method"` // "embedding", "keyword", "category"
}

// FailureWarning represents a detected potential failure pattern.
type FailureWarning struct {
	Pattern    *FailurePattern `json:"pattern"`
	Confidence float64         `json:"confidence"`
	Triggered  string          `json:"triggered"` // What triggered the match
}

// SkillMatchResult contains results from skill matching.
type SkillMatchResult struct {
	// Best matching skills (sorted by score)
	Skills []*SkillMatch `json:"skills"`

	// Detected failure patterns (warnings)
	Warnings []*FailureWarning `json:"warnings"`

	// Whether any high-confidence match was found
	HasMatch bool `json:"has_match"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// DISTILLATION TYPES
// Types for Skill Distillation Agent output
// ═══════════════════════════════════════════════════════════════════════════════

// DistillationOutput represents the output from analyzing reflections.
type DistillationOutput struct {
	SuccessPatterns []*SuccessPattern `json:"success_patterns"`
	FailureLessons  []*FailureLesson  `json:"failure_lessons"`
}

// SuccessPattern represents a success pattern extracted from reflections.
type SuccessPattern struct {
	Name        string    `json:"name"`
	Type        SkillType `json:"type"`
	Category    string    `json:"category,omitempty"`
	Description string    `json:"description"`
	WhenToApply string    `json:"when_to_apply"`
	Steps       []string  `json:"steps,omitempty"`
	Confidence  float64   `json:"confidence"`
	SourceRefs  []string  `json:"source_refs"` // Reflection IDs
}

// FailureLesson represents a failure lesson extracted from reflections.
type FailureLesson struct {
	Name           string    `json:"name"`
	Type           SkillType `json:"type"`
	Category       string    `json:"category,omitempty"`
	Description    string    `json:"description"`
	ErrorSignature string    `json:"error_signature"`
	Recovery       string    `json:"recovery"`
	Prevention     string    `json:"prevention"`
	Confidence     float64   `json:"confidence"`
	SourceRefs     []string  `json:"source_refs"` // Reflection IDs
}

// ═══════════════════════════════════════════════════════════════════════════════
// CONFIGURATION
// ═══════════════════════════════════════════════════════════════════════════════

// SkillDistillationConfig configures the skill distillation system.
type SkillDistillationConfig struct {
	// Minimum reflections before distillation attempt
	MinReflections int `yaml:"min_reflections"` // Default: 5

	// Distillation interval
	Interval time.Duration `yaml:"interval"` // Default: 1h

	// Model for pattern extraction
	Model string `yaml:"model"` // Default: gemini-2.5-flash

	// Confidence threshold for skill creation
	ConfidenceThreshold float64 `yaml:"confidence_threshold"` // Default: 0.7

	// Cross-agent learning
	CrossAgentLearning bool    `yaml:"cross_agent_learning"` // Default: true
	ShareThreshold     float64 `yaml:"share_threshold"`      // Default: 0.8

	// Storage paths
	SkillOutputDir   string `yaml:"skill_output_dir"`   // Default: ./config/skills/auto
	FailureOutputDir string `yaml:"failure_output_dir"` // Default: ./config/skills/failures

	// Limits
	MaxSkillsPerAgent   int `yaml:"max_skills_per_agent"`   // Default: 100
	MaxFailuresPerAgent int `yaml:"max_failures_per_agent"` // Default: 50
}

// DefaultSkillDistillationConfig returns sensible defaults.
func DefaultSkillDistillationConfig() *SkillDistillationConfig {
	return &SkillDistillationConfig{
		MinReflections:      5,
		Interval:            time.Hour,
		Model:               "gemini-2.5-flash",
		ConfidenceThreshold: 0.7,
		CrossAgentLearning:  true,
		ShareThreshold:      0.8,
		SkillOutputDir:      "./config/skills/auto",
		FailureOutputDir:    "./config/skills/failures",
		MaxSkillsPerAgent:   100,
		MaxFailuresPerAgent: 50,
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// REGISTRY STATS
// ═══════════════════════════════════════════════════════════════════════════════

// DynamicRegistryStats contains statistics about the skill registry.
type DynamicRegistryStats struct {
	// Template counts (existing cognitive system)
	StaticTemplates  int `json:"static_templates"`
	DynamicTemplates int `json:"dynamic_templates"`

	// Skill counts (new skill distillation)
	StaticSkills    int `json:"static_skills"`    // Original 43 Octopus skills
	DynamicSkills   int `json:"dynamic_skills"`   // Auto-generated
	SharedSkills    int `json:"shared_skills"`    // From cross-agent learning
	FailurePatterns int `json:"failure_patterns"`

	// Totals
	TotalSkills int `json:"total_skills"`

	// Health metrics
	AvgSkillConfidence float64 `json:"avg_skill_confidence"`
	AvgSuccessRate     float64 `json:"avg_success_rate"`
}
