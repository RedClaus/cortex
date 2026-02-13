package feedback

import (
	"context"
	"sort"
	"time"

	"github.com/normanking/cortex/internal/cognitive"
	"github.com/normanking/cortex/internal/logging"
)

// ═══════════════════════════════════════════════════════════════════════════════
// SYSTEM HEALTH METRICS
// ═══════════════════════════════════════════════════════════════════════════════

// SystemHealth contains comprehensive health metrics for the cognitive system.
type SystemHealth struct {
	Timestamp time.Time `json:"timestamp"`

	// Template Statistics
	TotalTemplates     int `json:"total_templates"`
	ProbationTemplates int `json:"probation_templates"`
	ValidatedTemplates int `json:"validated_templates"`
	PromotedTemplates  int `json:"promoted_templates"`
	DeprecatedTemplates int `json:"deprecated_templates"`

	// Routing Statistics
	TotalRoutes          int64   `json:"total_routes"`
	TemplateHitRate      float64 `json:"template_hit_rate"`      // % of requests matched to templates
	LocalModelRate       float64 `json:"local_model_rate"`       // % of requests served by local models
	FrontierEscalations  int64   `json:"frontier_escalations"`   // Count of frontier model calls

	// Performance Metrics
	AvgLatencyMs         float64 `json:"avg_latency_ms"`
	P95LatencyMs         float64 `json:"p95_latency_ms"`
	SuccessRate          float64 `json:"success_rate"`

	// Distillation Statistics
	TotalDistillations   int64   `json:"total_distillations"`
	SuccessfulDistillations int64 `json:"successful_distillations"`
	DistillationSuccessRate float64 `json:"distillation_success_rate"`

	// Grading Statistics
	TotalGrades          int64   `json:"total_grades"`
	PassRate             float64 `json:"pass_rate"`
	FailRate             float64 `json:"fail_rate"`
	PartialRate          float64 `json:"partial_rate"`

	// Learning Velocity
	TemplatesCreatedLast24h int `json:"templates_created_last_24h"`
	TemplatesPromotedLast24h int `json:"templates_promoted_last_24h"`
	TemplatesDeprecatedLast24h int `json:"templates_deprecated_last_24h"`

	// Health Score (0-100)
	HealthScore int `json:"health_score"`
	HealthStatus string `json:"health_status"` // "healthy", "degraded", "unhealthy"
}

// MetricsCollector gathers system health metrics.
type MetricsCollector struct {
	registry cognitive.Registry
	log      *logging.Logger
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector(registry cognitive.Registry) *MetricsCollector {
	return &MetricsCollector{
		registry: registry,
		log:      logging.Global(),
	}
}

// GetSystemHealth collects comprehensive system health metrics.
func (m *MetricsCollector) GetSystemHealth(ctx context.Context) (*SystemHealth, error) {
	health := &SystemHealth{
		Timestamp: time.Now(),
	}

	// Collect template counts by status
	if err := m.collectTemplateCounts(ctx, health); err != nil {
		m.log.Warn("[Metrics] Failed to collect template counts: %v", err)
	}

	// Collect aggregate metrics from registry
	if err := m.collectAggregateMetrics(ctx, health); err != nil {
		m.log.Warn("[Metrics] Failed to collect aggregate metrics: %v", err)
	}

	// Collect learning velocity metrics
	if err := m.collectLearningVelocity(ctx, health); err != nil {
		m.log.Warn("[Metrics] Failed to collect learning velocity: %v", err)
	}

	// Calculate health score
	m.calculateHealthScore(health)

	return health, nil
}

// collectTemplateCounts counts templates by status.
func (m *MetricsCollector) collectTemplateCounts(ctx context.Context, health *SystemHealth) error {
	statuses := []cognitive.TemplateStatus{
		cognitive.StatusProbation,
		cognitive.StatusValidated,
		cognitive.StatusPromoted,
		cognitive.StatusDeprecated,
	}

	for _, status := range statuses {
		templates, err := m.registry.ListByStatus(ctx, status)
		if err != nil {
			return err
		}

		count := len(templates)
		health.TotalTemplates += count

		switch status {
		case cognitive.StatusProbation:
			health.ProbationTemplates = count
		case cognitive.StatusValidated:
			health.ValidatedTemplates = count
		case cognitive.StatusPromoted:
			health.PromotedTemplates = count
		case cognitive.StatusDeprecated:
			health.DeprecatedTemplates = count
		}
	}

	return nil
}

// collectAggregateMetrics collects aggregate metrics from the registry.
func (m *MetricsCollector) collectAggregateMetrics(ctx context.Context, health *SystemHealth) error {
	// Try to get cognitive metrics from registry
	metrics, err := m.registry.GetCognitiveMetrics(ctx)
	if err != nil {
		return err
	}

	if metrics != nil {
		health.TotalRoutes = metrics.TotalRequests
		health.TemplateHitRate = metrics.TemplateHitRate
		health.LocalModelRate = metrics.LocalModelRate
		health.FrontierEscalations = metrics.FrontierCalls
		health.AvgLatencyMs = metrics.AvgLatencyMs
		health.P95LatencyMs = metrics.P95LatencyMs
		health.SuccessRate = metrics.SuccessRate
		health.TotalDistillations = metrics.DistillationAttempts
		health.SuccessfulDistillations = metrics.DistillationSuccesses

		if metrics.DistillationAttempts > 0 {
			health.DistillationSuccessRate = float64(metrics.DistillationSuccesses) / float64(metrics.DistillationAttempts)
		}

		health.TotalGrades = int64(metrics.TotalGrades)
		if metrics.TotalGrades > 0 {
			health.PassRate = float64(metrics.PassGrades) / float64(metrics.TotalGrades)
			health.FailRate = float64(metrics.FailGrades) / float64(metrics.TotalGrades)
			health.PartialRate = float64(metrics.PartialGrades) / float64(metrics.TotalGrades)
		}
	}

	return nil
}

// collectLearningVelocity measures template lifecycle activity in the last 24 hours.
func (m *MetricsCollector) collectLearningVelocity(ctx context.Context, health *SystemHealth) error {
	since := time.Now().Add(-24 * time.Hour)

	// Count recently created templates
	allTemplates, err := m.registry.ListAll(ctx, nil)
	if err != nil {
		return err
	}

	for _, tmpl := range allTemplates {
		if tmpl.CreatedAt.After(since) {
			health.TemplatesCreatedLast24h++
		}

		// Check promotion/deprecation times would require additional tracking
		// For now, we'll use the status and updatedAt as a proxy
		if tmpl.UpdatedAt.After(since) {
			switch tmpl.Status {
			case cognitive.StatusPromoted:
				health.TemplatesPromotedLast24h++
			case cognitive.StatusDeprecated:
				health.TemplatesDeprecatedLast24h++
			}
		}
	}

	return nil
}

// calculateHealthScore computes an overall health score (0-100).
func (m *MetricsCollector) calculateHealthScore(health *SystemHealth) {
	var score float64 = 100

	// Factor 1: Template Maturity (30 points)
	// More promoted templates = better
	if health.TotalTemplates > 0 {
		maturityRatio := float64(health.PromotedTemplates) / float64(health.TotalTemplates)
		score -= 30 * (1 - maturityRatio)
	}

	// Factor 2: Success Rate (30 points)
	// Higher success rate = better
	if health.SuccessRate > 0 {
		score -= 30 * (1 - health.SuccessRate)
	}

	// Factor 3: Local Model Rate (20 points)
	// Higher local model usage = lower cost
	if health.LocalModelRate > 0 {
		score -= 20 * (1 - health.LocalModelRate)
	}

	// Factor 4: Grading Pass Rate (10 points)
	if health.PassRate > 0 {
		score -= 10 * (1 - health.PassRate)
	}

	// Factor 5: Distillation Success Rate (10 points)
	if health.DistillationSuccessRate > 0 {
		score -= 10 * (1 - health.DistillationSuccessRate)
	}

	// Clamp to 0-100
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	health.HealthScore = int(score)

	// Determine health status
	switch {
	case health.HealthScore >= 80:
		health.HealthStatus = "healthy"
	case health.HealthScore >= 50:
		health.HealthStatus = "degraded"
	default:
		health.HealthStatus = "unhealthy"
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// METRIC RECORDING
// ═══════════════════════════════════════════════════════════════════════════════

// RecordRequest records a routing request for metrics.
func (m *MetricsCollector) RecordRequest(ctx context.Context, templateMatch bool, modelTier string, latencyMs int) {
	// This would update counters in the registry
	// For now, this is handled by the orchestrator integration
	m.log.Debug("[Metrics] Request: template_match=%v, tier=%s, latency=%dms", templateMatch, modelTier, latencyMs)
}

// RecordDistillation records a distillation attempt.
func (m *MetricsCollector) RecordDistillation(ctx context.Context, success bool, templateID string) {
	status := "success"
	if !success {
		status = "failure"
	}
	m.log.Debug("[Metrics] Distillation: status=%s, template_id=%s", status, templateID)
}

// ═══════════════════════════════════════════════════════════════════════════════
// DASHBOARD DATA
// ═══════════════════════════════════════════════════════════════════════════════

// DashboardData contains data for a monitoring dashboard.
type DashboardData struct {
	Health         *SystemHealth          `json:"health"`
	TopTemplates   []TemplateStats        `json:"top_templates"`
	RecentActivity []ActivityEntry        `json:"recent_activity"`
}

// TemplateStats contains usage statistics for a template.
type TemplateStats struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Status      string  `json:"status"`
	UseCount    int     `json:"use_count"`
	SuccessRate float64 `json:"success_rate"`
	AvgLatency  float64 `json:"avg_latency"`
}

// ActivityEntry represents a recent activity event.
type ActivityEntry struct {
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"` // "route", "distillation", "promotion", "deprecation"
	TemplateID  string    `json:"template_id,omitempty"`
	Description string    `json:"description"`
}

// GetDashboardData collects all data needed for a monitoring dashboard.
func (m *MetricsCollector) GetDashboardData(ctx context.Context) (*DashboardData, error) {
	data := &DashboardData{
		TopTemplates:   make([]TemplateStats, 0),
		RecentActivity: make([]ActivityEntry, 0),
	}

	// Get health metrics
	health, err := m.GetSystemHealth(ctx)
	if err != nil {
		return nil, err
	}
	data.Health = health

	// Get top templates by usage
	templates, err := m.registry.ListAll(ctx, nil)
	if err != nil {
		return nil, err
	}

	// Collect stats for each template
	for _, tmpl := range templates {
		if tmpl.Status == cognitive.StatusDeprecated {
			continue // Skip deprecated templates
		}

		metrics, err := m.registry.GetTemplateMetrics(ctx, tmpl.ID)
		if err != nil {
			continue
		}

		data.TopTemplates = append(data.TopTemplates, TemplateStats{
			ID:          tmpl.ID,
			Name:        tmpl.Name,
			Status:      string(tmpl.Status),
			UseCount:    metrics.UseCount,
			SuccessRate: metrics.SuccessRate,
			AvgLatency:  metrics.AvgLatencyMs,
		})
	}

	// Sort by use count (descending)
	sort.Slice(data.TopTemplates, func(i, j int) bool {
		return data.TopTemplates[i].UseCount > data.TopTemplates[j].UseCount
	})

	// Limit to top 10
	if len(data.TopTemplates) > 10 {
		data.TopTemplates = data.TopTemplates[:10]
	}

	return data, nil
}
