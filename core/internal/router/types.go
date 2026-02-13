// Package router implements the Fast/Slow request classification system.
// It routes user requests to appropriate handlers based on intent classification.
package router

import (
	"time"
)

// TaskType represents the classification of a user request.
type TaskType string

const (
	// TaskGeneral is the default task type for unclassified requests.
	TaskGeneral TaskType = "general"
	// TaskCodeGen is for code generation and writing tasks.
	TaskCodeGen TaskType = "code_generation"
	// TaskDebug is for debugging and error fixing tasks.
	TaskDebug TaskType = "debug"
	// TaskReview is for code review and audit tasks.
	TaskReview TaskType = "review"
	// TaskPlanning is for architecture and design planning tasks.
	TaskPlanning TaskType = "planning"
	// TaskInfrastructure is for DevOps, networking, and infrastructure tasks.
	TaskInfrastructure TaskType = "infrastructure"
	// TaskExplain is for explanation and documentation tasks.
	TaskExplain TaskType = "explain"
	// TaskRefactor is for code refactoring tasks.
	TaskRefactor TaskType = "refactor"
)

// AllTaskTypes returns all valid task types for validation.
func AllTaskTypes() []TaskType {
	return []TaskType{
		TaskGeneral,
		TaskCodeGen,
		TaskDebug,
		TaskReview,
		TaskPlanning,
		TaskInfrastructure,
		TaskExplain,
		TaskRefactor,
	}
}

// String returns the string representation of a TaskType.
func (t TaskType) String() string {
	return string(t)
}

// IsValid checks if a TaskType is a known valid type.
func (t TaskType) IsValid() bool {
	for _, valid := range AllTaskTypes() {
		if t == valid {
			return true
		}
	}
	return false
}

// RiskLevel indicates the potential risk of executing a task.
type RiskLevel int

const (
	// RiskLow indicates safe operations (read-only, non-destructive).
	RiskLow RiskLevel = iota
	// RiskMedium indicates operations that modify files or state.
	RiskMedium
	// RiskHigh indicates potentially dangerous operations (system commands, deletions).
	RiskHigh
	// RiskCritical indicates operations requiring explicit user confirmation.
	RiskCritical
)

// String returns a human-readable risk level.
func (r RiskLevel) String() string {
	switch r {
	case RiskLow:
		return "low"
	case RiskMedium:
		return "medium"
	case RiskHigh:
		return "high"
	case RiskCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// ClassificationPath indicates whether fast or slow classification was used.
type ClassificationPath string

const (
	// PathFast indicates regex-based classification was used.
	PathFast ClassificationPath = "fast"
	// PathSlow indicates semantic (LLM) classification was used.
	PathSlow ClassificationPath = "slow"
	// PathExplicit indicates the user explicitly specified the task type via @mention.
	PathExplicit ClassificationPath = "explicit"
	// PathContext indicates classification was based on context (e.g., platform detection).
	PathContext ClassificationPath = "context"
)

// RoutingDecision contains the result of request classification.
type RoutingDecision struct {
	// TaskType is the classified task category.
	TaskType TaskType `json:"task_type"`

	// Input is the original user input (potentially cleaned).
	Input string `json:"input"`

	// Confidence is the classification confidence (0.0 to 1.0).
	Confidence float64 `json:"confidence"`

	// Path indicates which classification method was used.
	Path ClassificationPath `json:"path"`

	// RiskLevel indicates the potential risk of this task.
	RiskLevel RiskLevel `json:"risk_level"`

	// Specialist is the name of the specialist to handle this task (optional).
	Specialist string `json:"specialist,omitempty"`

	// Metadata contains additional context from classification.
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// ClassifiedAt is when the classification was made.
	ClassifiedAt time.Time `json:"classified_at"`

	// ClassificationDuration is how long classification took.
	ClassificationDuration time.Duration `json:"classification_duration"`
}

// ProcessContext provides additional context for routing decisions.
type ProcessContext struct {
	// SessionID identifies the current session.
	SessionID string

	// Platform contains detected platform information (optional).
	Platform *PlatformInfo

	// WorkingDir is the current working directory.
	WorkingDir string

	// RecentCommands contains recently executed commands (for context).
	RecentCommands []string

	// ActiveFile is the currently focused file, if any.
	ActiveFile string
}

// PlatformInfo contains detected platform/environment details.
type PlatformInfo struct {
	// Vendor is the platform vendor (e.g., "cisco", "linux", "windows").
	Vendor string `json:"vendor"`

	// Name is the platform name (e.g., "ubuntu", "ios-xe").
	Name string `json:"name"`

	// Version is the platform version.
	Version string `json:"version"`

	// DetectedFrom indicates how the platform was detected.
	DetectedFrom string `json:"detected_from"`
}

// RouterStats tracks routing statistics for monitoring and tuning.
type RouterStats struct {
	// FastHits is the number of requests classified via fast path.
	FastHits int64 `json:"fast_hits"`

	// SlowHits is the number of requests classified via slow path.
	SlowHits int64 `json:"slow_hits"`

	// ExplicitHits is the number of explicit @mention classifications.
	ExplicitHits int64 `json:"explicit_hits"`

	// ContextHits is the number of context-based classifications.
	ContextHits int64 `json:"context_hits"`

	// AmbiguousCount is the number of ambiguous requests that needed slow path.
	AmbiguousCount int64 `json:"ambiguous_count"`

	// TotalRequests is the total number of routing requests.
	TotalRequests int64 `json:"total_requests"`

	// AverageConfidence is the running average confidence score.
	AverageConfidence float64 `json:"average_confidence"`

	// TaskTypeDistribution tracks how often each task type is classified.
	TaskTypeDistribution map[TaskType]int64 `json:"task_type_distribution"`
}

// FastPathRatio returns the percentage of requests handled by the fast path.
func (s *RouterStats) FastPathRatio() float64 {
	if s.TotalRequests == 0 {
		return 0
	}
	return float64(s.FastHits) / float64(s.TotalRequests) * 100
}

// LLMRouter is the interface for semantic classification via LLM.
type LLMRouter interface {
	// Classify uses an LLM to semantically classify a user request.
	Classify(input string) (TaskType, float64, error)
}

// Specialist represents a specialized handler for a task type.
type Specialist struct {
	// Name is the specialist's identifier.
	Name string `json:"name"`

	// TaskTypes is the list of task types this specialist handles.
	TaskTypes []TaskType `json:"task_types"`

	// Description explains what this specialist does.
	Description string `json:"description"`

	// SystemPrompt is the custom system prompt for this specialist.
	SystemPrompt string `json:"system_prompt,omitempty"`
}
