package cognitive

import (
	"context"
)

// ═══════════════════════════════════════════════════════════════════════════════
// TEMPLATE REGISTRY INTERFACE
// ═══════════════════════════════════════════════════════════════════════════════

// Registry defines the interface for template storage and retrieval.
// Implementations handle persistence, caching, and lifecycle management.
type Registry interface {
	// ═══ CRUD Operations ═══

	// Create stores a new template in the registry.
	// The template ID must be unique. Returns error if ID already exists.
	Create(ctx context.Context, template *Template) error

	// Get retrieves a template by ID.
	// Returns nil and error if not found.
	Get(ctx context.Context, id string) (*Template, error)

	// Update modifies an existing template.
	// Returns error if template doesn't exist.
	Update(ctx context.Context, template *Template) error

	// Delete removes a template from the registry.
	// This is a hard delete - use UpdateStatus for soft deprecation.
	Delete(ctx context.Context, id string) error

	// ═══ Query Operations ═══

	// ListAll returns all templates, optionally filtered by status.
	// Pass nil for statuses to return all templates.
	ListAll(ctx context.Context, statuses []TemplateStatus) ([]*Template, error)

	// ListByTaskType returns templates matching a specific task type.
	ListByTaskType(ctx context.Context, taskType TaskType) ([]*Template, error)

	// ListByDomain returns templates for a specific domain.
	ListByDomain(ctx context.Context, domain string) ([]*Template, error)

	// ListActive returns all templates that can be used for routing.
	// This includes templates with status 'promoted' or 'validated'.
	ListActive(ctx context.Context) ([]*Template, error)

	// SearchByKeywords performs FTS search on template intents.
	// Used as fallback when embedding search is unavailable.
	SearchByKeywords(ctx context.Context, query string, limit int) ([]*Template, error)

	// ═══ Lifecycle Operations ═══

	// UpdateStatus changes a template's lifecycle status.
	// Handles timestamp updates (promoted_at, deprecated_at).
	UpdateStatus(ctx context.Context, id string, status TemplateStatus) error

	// GetPromotionCandidates returns templates ready for promotion.
	// Criteria: probation status, 3+ grades, 90%+ pass rate.
	GetPromotionCandidates(ctx context.Context) ([]*Template, error)

	// GetDeprecationCandidates returns templates at risk of deprecation.
	// Criteria: probation status, 3+ grades, 50%+ fail rate.
	GetDeprecationCandidates(ctx context.Context) ([]*Template, error)

	// ═══ Status Queries ═══

	// ListByStatus returns templates with a specific status.
	ListByStatus(ctx context.Context, status TemplateStatus) ([]*Template, error)

	// GetTemplateMetrics returns usage metrics for a specific template.
	GetTemplateMetrics(ctx context.Context, templateID string) (*TemplateMetrics, error)

	// GetCognitiveMetrics returns aggregate cognitive system metrics.
	GetCognitiveMetrics(ctx context.Context) (*CognitiveMetrics, error)

	// ═══ Usage Tracking ═══

	// RecordUsage logs a template use event.
	// This updates use_count, success_count, failure_count, and last_used_at.
	// Returns the log ID for tracking.
	RecordUsage(ctx context.Context, log *UsageLog) (int64, error)

	// GetUsageLogs retrieves usage history for a template.
	GetUsageLogs(ctx context.Context, templateID string, limit int) ([]*UsageLog, error)

	// ═══ Grading ═══

	// RecordGrade logs a grading result and updates confidence score.
	RecordGrade(ctx context.Context, result *GradingResult) error

	// GetGradingLogs retrieves grading history for a template.
	GetGradingLogs(ctx context.Context, templateID string) ([]*GradingResult, error)

	// GetPendingGrades returns usage logs that need grading.
	// These are logs where success is NULL.
	GetPendingGrades(ctx context.Context, limit int) ([]*UsageLog, error)

	// ═══ Metrics ═══

	// GetMetrics retrieves cognitive metrics for a date.
	GetMetrics(ctx context.Context, date string) (*CognitiveMetrics, error)

	// IncrementMetric atomically increments a metric counter.
	IncrementMetric(ctx context.Context, metric string) error

	// GetSystemHealth returns overall system health stats.
	GetSystemHealth(ctx context.Context) (*SystemHealth, error)

	// ═══ Embedding Cache ═══

	// CacheEmbedding stores an embedding for later retrieval.
	CacheEmbedding(ctx context.Context, sourceType, sourceID, textHash string, embedding Embedding, model string) error

	// GetCachedEmbedding retrieves a cached embedding.
	// Returns nil if not found.
	GetCachedEmbedding(ctx context.Context, sourceType, sourceID string) (Embedding, error)

	// ═══ Distillation Tracking ═══

	// RecordDistillation logs a distillation request and its outcome.
	RecordDistillation(ctx context.Context, req *DistillationRequest) error

	// GetDistillationHistory retrieves recent distillation requests.
	GetDistillationHistory(ctx context.Context, limit int) ([]*DistillationRequest, error)
}

// ═══════════════════════════════════════════════════════════════════════════════
// FILTER AND OPTIONS TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// ListOptions configures template listing operations.
type ListOptions struct {
	// Status filters - only return templates with these statuses.
	Statuses []TemplateStatus

	// TaskType filter - only return templates of this type.
	TaskType *TaskType

	// Domain filter - only return templates for this domain.
	Domain string

	// Limit caps the number of results.
	Limit int

	// Offset for pagination.
	Offset int

	// OrderBy specifies sort order.
	OrderBy string // "confidence", "use_count", "created_at", "updated_at"

	// Descending reverses sort order.
	Descending bool
}

// DefaultListOptions returns sensible default options.
func DefaultListOptions() *ListOptions {
	return &ListOptions{
		Limit:      100,
		OrderBy:    "confidence",
		Descending: true,
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// REGISTRY EVENTS
// ═══════════════════════════════════════════════════════════════════════════════

// RegistryEventType identifies types of registry events.
type RegistryEventType string

const (
	EventTemplateCreated   RegistryEventType = "template_created"
	EventTemplateUpdated   RegistryEventType = "template_updated"
	EventTemplatePromoted  RegistryEventType = "template_promoted"
	EventTemplateDeprecated RegistryEventType = "template_deprecated"
	EventUsageRecorded     RegistryEventType = "usage_recorded"
	EventGradeRecorded     RegistryEventType = "grade_recorded"
)

// RegistryEvent represents an event that occurred in the registry.
type RegistryEvent struct {
	Type       RegistryEventType `json:"type"`
	TemplateID string            `json:"template_id"`
	Payload    interface{}       `json:"payload,omitempty"`
}

// RegistryEventHandler handles registry events.
type RegistryEventHandler func(event *RegistryEvent)

// ═══════════════════════════════════════════════════════════════════════════════
// OBSERVABLE REGISTRY
// ═══════════════════════════════════════════════════════════════════════════════

// ObservableRegistry extends Registry with event subscription.
type ObservableRegistry interface {
	Registry

	// Subscribe registers a handler for registry events.
	// Returns an unsubscribe function.
	Subscribe(handler RegistryEventHandler) func()
}
