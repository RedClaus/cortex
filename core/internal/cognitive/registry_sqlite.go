package cognitive

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"
)

// ═══════════════════════════════════════════════════════════════════════════════
// SQLITE REGISTRY IMPLEMENTATION
// ═══════════════════════════════════════════════════════════════════════════════

// SQLiteRegistry implements Registry using SQLite for persistence.
type SQLiteRegistry struct {
	db        *sql.DB
	mu        sync.RWMutex
	handlers  []RegistryEventHandler
	handlerMu sync.RWMutex
}

// NewSQLiteRegistry creates a new SQLite-backed registry.
func NewSQLiteRegistry(db *sql.DB) *SQLiteRegistry {
	return &SQLiteRegistry{
		db:       db,
		handlers: make([]RegistryEventHandler, 0),
	}
}

// emit sends an event to all registered handlers.
func (r *SQLiteRegistry) emit(event *RegistryEvent) {
	r.handlerMu.RLock()
	handlers := make([]RegistryEventHandler, len(r.handlers))
	copy(handlers, r.handlers)
	r.handlerMu.RUnlock()

	for _, h := range handlers {
		go h(event)
	}
}

// Subscribe registers a handler for registry events.
func (r *SQLiteRegistry) Subscribe(handler RegistryEventHandler) func() {
	r.handlerMu.Lock()
	r.handlers = append(r.handlers, handler)
	idx := len(r.handlers) - 1
	r.handlerMu.Unlock()

	return func() {
		r.handlerMu.Lock()
		defer r.handlerMu.Unlock()
		if idx < len(r.handlers) {
			r.handlers = append(r.handlers[:idx], r.handlers[idx+1:]...)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// CRUD OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════════

// Create stores a new template in the registry.
func (r *SQLiteRegistry) Create(ctx context.Context, t *Template) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	keywordsJSON, err := json.Marshal(t.IntentKeywords)
	if err != nil {
		return fmt.Errorf("marshal keywords: %w", err)
	}

	query := `
		INSERT INTO templates (
			id, name, description, intent, intent_embedding, intent_keywords,
			template_body, example_output, variable_schema, gbnf_grammar,
			task_type, domain, status, confidence_score, complexity_score,
			use_count, success_count, failure_count,
			source_type, source_model, source_request_id,
			created_at, updated_at
		) VALUES (
			?, ?, ?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?,
			?, ?, ?,
			?, ?
		)
	`

	now := time.Now()
	_, err = r.db.ExecContext(ctx, query,
		t.ID, t.Name, nullString(t.Description),
		t.Intent, t.IntentEmbedding.ToBytes(), string(keywordsJSON),
		t.TemplateBody, nullString(t.ExampleOutput), t.VariableSchema, nullString(t.GBNFGrammar),
		string(t.TaskType), nullString(t.Domain), string(t.Status),
		t.ConfidenceScore, t.ComplexityScore,
		t.UseCount, t.SuccessCount, t.FailureCount,
		string(t.SourceType), nullString(t.SourceModel), nullString(t.SourceRequestID),
		now, now,
	)

	if err != nil {
		return fmt.Errorf("insert template: %w", err)
	}

	r.emit(&RegistryEvent{Type: EventTemplateCreated, TemplateID: t.ID, Payload: t})
	return nil
}

// Get retrieves a template by ID.
func (r *SQLiteRegistry) Get(ctx context.Context, id string) (*Template, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := `
		SELECT
			id, name, description, intent, intent_embedding, intent_keywords,
			template_body, example_output, variable_schema, gbnf_grammar,
			task_type, domain, status, confidence_score, complexity_score,
			use_count, success_count, failure_count,
			source_type, source_model, source_request_id,
			created_at, updated_at, last_used_at, promoted_at, deprecated_at
		FROM templates
		WHERE id = ?
	`

	var t Template
	var description, exampleOutput, gbnfGrammar, domain, sourceModel, sourceRequestID sql.NullString
	var embedding []byte
	var keywordsJSON string
	var lastUsedAt, promotedAt, deprecatedAt sql.NullTime
	var taskType, status, sourceType string

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&t.ID, &t.Name, &description, &t.Intent, &embedding, &keywordsJSON,
		&t.TemplateBody, &exampleOutput, &t.VariableSchema, &gbnfGrammar,
		&taskType, &domain, &status, &t.ConfidenceScore, &t.ComplexityScore,
		&t.UseCount, &t.SuccessCount, &t.FailureCount,
		&sourceType, &sourceModel, &sourceRequestID,
		&t.CreatedAt, &t.UpdatedAt, &lastUsedAt, &promotedAt, &deprecatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("template not found: %s", id)
		}
		return nil, fmt.Errorf("query template: %w", err)
	}

	// Set nullable fields
	if description.Valid {
		t.Description = description.String
	}
	if exampleOutput.Valid {
		t.ExampleOutput = exampleOutput.String
	}
	if gbnfGrammar.Valid {
		t.GBNFGrammar = gbnfGrammar.String
	}
	if domain.Valid {
		t.Domain = domain.String
	}
	if sourceModel.Valid {
		t.SourceModel = sourceModel.String
	}
	if sourceRequestID.Valid {
		t.SourceRequestID = sourceRequestID.String
	}
	if lastUsedAt.Valid {
		t.LastUsedAt = &lastUsedAt.Time
	}
	if promotedAt.Valid {
		t.PromotedAt = &promotedAt.Time
	}
	if deprecatedAt.Valid {
		t.DeprecatedAt = &deprecatedAt.Time
	}

	// Parse enum types
	t.TaskType = TaskType(taskType)
	t.Status = TemplateStatus(status)
	t.SourceType = TemplateSourceType(sourceType)

	// Parse embedding
	t.IntentEmbedding = EmbeddingFromBytes(embedding)

	// Parse keywords
	if err := json.Unmarshal([]byte(keywordsJSON), &t.IntentKeywords); err != nil {
		t.IntentKeywords = nil
	}

	return &t, nil
}

// Update modifies an existing template.
func (r *SQLiteRegistry) Update(ctx context.Context, t *Template) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	keywordsJSON, err := json.Marshal(t.IntentKeywords)
	if err != nil {
		return fmt.Errorf("marshal keywords: %w", err)
	}

	query := `
		UPDATE templates SET
			name = ?, description = ?, intent = ?, intent_embedding = ?, intent_keywords = ?,
			template_body = ?, example_output = ?, variable_schema = ?, gbnf_grammar = ?,
			task_type = ?, domain = ?, confidence_score = ?, complexity_score = ?,
			use_count = ?, success_count = ?, failure_count = ?,
			updated_at = ?
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query,
		t.Name, nullString(t.Description), t.Intent, t.IntentEmbedding.ToBytes(), string(keywordsJSON),
		t.TemplateBody, nullString(t.ExampleOutput), t.VariableSchema, nullString(t.GBNFGrammar),
		string(t.TaskType), nullString(t.Domain), t.ConfidenceScore, t.ComplexityScore,
		t.UseCount, t.SuccessCount, t.FailureCount,
		time.Now(), t.ID,
	)

	if err != nil {
		return fmt.Errorf("update template: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("template not found: %s", t.ID)
	}

	r.emit(&RegistryEvent{Type: EventTemplateUpdated, TemplateID: t.ID, Payload: t})
	return nil
}

// Delete removes a template from the registry.
func (r *SQLiteRegistry) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	result, err := r.db.ExecContext(ctx, "DELETE FROM templates WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete template: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("template not found: %s", id)
	}

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// QUERY OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════════

// ListAll returns all templates, optionally filtered by status.
func (r *SQLiteRegistry) ListAll(ctx context.Context, statuses []TemplateStatus) ([]*Template, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := `
		SELECT
			id, name, description, intent, intent_embedding, intent_keywords,
			template_body, example_output, variable_schema, gbnf_grammar,
			task_type, domain, status, confidence_score, complexity_score,
			use_count, success_count, failure_count,
			source_type, source_model, source_request_id,
			created_at, updated_at, last_used_at, promoted_at, deprecated_at
		FROM templates
	`

	var args []interface{}
	if len(statuses) > 0 {
		query += " WHERE status IN ("
		for i, s := range statuses {
			if i > 0 {
				query += ","
			}
			query += "?"
			args = append(args, string(s))
		}
		query += ")"
	}
	query += " ORDER BY confidence_score DESC, use_count DESC"

	return r.queryTemplates(ctx, query, args...)
}

// ListByTaskType returns templates matching a specific task type.
func (r *SQLiteRegistry) ListByTaskType(ctx context.Context, taskType TaskType) ([]*Template, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := `
		SELECT
			id, name, description, intent, intent_embedding, intent_keywords,
			template_body, example_output, variable_schema, gbnf_grammar,
			task_type, domain, status, confidence_score, complexity_score,
			use_count, success_count, failure_count,
			source_type, source_model, source_request_id,
			created_at, updated_at, last_used_at, promoted_at, deprecated_at
		FROM templates
		WHERE task_type = ?
		ORDER BY confidence_score DESC
	`

	return r.queryTemplates(ctx, query, string(taskType))
}

// ListByDomain returns templates for a specific domain.
func (r *SQLiteRegistry) ListByDomain(ctx context.Context, domain string) ([]*Template, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := `
		SELECT
			id, name, description, intent, intent_embedding, intent_keywords,
			template_body, example_output, variable_schema, gbnf_grammar,
			task_type, domain, status, confidence_score, complexity_score,
			use_count, success_count, failure_count,
			source_type, source_model, source_request_id,
			created_at, updated_at, last_used_at, promoted_at, deprecated_at
		FROM templates
		WHERE domain = ?
		ORDER BY confidence_score DESC
	`

	return r.queryTemplates(ctx, query, domain)
}

// ListActive returns all templates that can be used for routing.
func (r *SQLiteRegistry) ListActive(ctx context.Context) ([]*Template, error) {
	return r.ListAll(ctx, []TemplateStatus{StatusPromoted, StatusValidated})
}

// ListByStatus returns templates with a specific status.
func (r *SQLiteRegistry) ListByStatus(ctx context.Context, status TemplateStatus) ([]*Template, error) {
	return r.ListAll(ctx, []TemplateStatus{status})
}

// GetTemplateMetrics returns usage metrics for a specific template.
func (r *SQLiteRegistry) GetTemplateMetrics(ctx context.Context, templateID string) (*TemplateMetrics, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metrics := &TemplateMetrics{TemplateID: templateID}

	// Get usage counts from template
	query := `
		SELECT use_count, success_count, failure_count
		FROM templates WHERE id = ?
	`
	err := r.db.QueryRowContext(ctx, query, templateID).Scan(
		&metrics.UseCount, &metrics.SuccessCount, &metrics.FailureCount,
	)
	if err != nil {
		return nil, fmt.Errorf("get template metrics: %w", err)
	}

	// Calculate success rate
	if metrics.UseCount > 0 {
		metrics.SuccessRate = float64(metrics.SuccessCount) / float64(metrics.UseCount)
	}

	// Get average latency from usage logs
	latencyQuery := `
		SELECT COALESCE(AVG(latency_ms), 0) FROM template_usage_log
		WHERE template_id = ? AND latency_ms > 0
	`
	r.db.QueryRowContext(ctx, latencyQuery, templateID).Scan(&metrics.AvgLatencyMs)

	// Get grading counts
	gradeQuery := `
		SELECT
			COALESCE(SUM(CASE WHEN grade = 'pass' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN grade = 'fail' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN grade = 'partial' THEN 1 ELSE 0 END), 0)
		FROM template_grading_log
		WHERE template_id = ?
	`
	r.db.QueryRowContext(ctx, gradeQuery, templateID).Scan(
		&metrics.PassCount, &metrics.FailCount, &metrics.PartialCount,
	)

	return metrics, nil
}

// GetCognitiveMetrics returns aggregate cognitive system metrics.
func (r *SQLiteRegistry) GetCognitiveMetrics(ctx context.Context) (*CognitiveMetrics, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metrics := &CognitiveMetrics{
		Date: time.Now().Format("2006-01-02"),
	}

	// Get today's metrics from cognitive_metrics table
	query := `
		SELECT
			COALESCE(SUM(CASE WHEN metric_name = 'total_requests' THEN metric_value ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN metric_name = 'template_hits' THEN metric_value ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN metric_name = 'template_misses' THEN metric_value ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN metric_name = 'local_model_calls' THEN metric_value ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN metric_name = 'frontier_calls' THEN metric_value ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN metric_name = 'distillation_attempts' THEN metric_value ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN metric_name = 'distillation_successes' THEN metric_value ELSE 0 END), 0)
		FROM cognitive_metrics
		WHERE date = ?
	`
	err := r.db.QueryRowContext(ctx, query, metrics.Date).Scan(
		&metrics.TotalRequests,
		&metrics.TemplateHits,
		&metrics.TemplateMisses,
		&metrics.LocalModelCalls,
		&metrics.FrontierCalls,
		&metrics.DistillationAttempts,
		&metrics.DistillationSuccesses,
	)
	if err != nil {
		// If no metrics for today, return empty metrics
		return metrics, nil
	}

	// Calculate rates
	if metrics.TotalRequests > 0 {
		metrics.TemplateHitRate = float64(metrics.TemplateHits) / float64(metrics.TotalRequests)
		metrics.LocalModelRate = float64(metrics.LocalModelCalls) / float64(metrics.TotalRequests)
	}

	// Get grading stats
	gradeQuery := `
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN grade = 'pass' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN grade = 'fail' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN grade = 'partial' THEN 1 ELSE 0 END), 0)
		FROM template_grading_log
		WHERE DATE(created_at) = ?
	`
	r.db.QueryRowContext(ctx, gradeQuery, metrics.Date).Scan(
		&metrics.TotalGrades, &metrics.PassGrades, &metrics.FailGrades, &metrics.PartialGrades,
	)

	// Get latency stats
	latencyQuery := `
		SELECT
			COALESCE(AVG(total_ms), 0),
			COALESCE(MAX(total_ms), 0)
		FROM template_usage_log
		WHERE DATE(created_at) = ? AND total_ms > 0
	`
	r.db.QueryRowContext(ctx, latencyQuery, metrics.Date).Scan(
		&metrics.AvgLatencyMs, &metrics.P95LatencyMs,
	)

	// Get success rate from usage logs
	successQuery := `
		SELECT
			COALESCE(AVG(CASE WHEN success = 1 THEN 1.0 ELSE 0.0 END), 0)
		FROM template_usage_log
		WHERE DATE(created_at) = ?
	`
	r.db.QueryRowContext(ctx, successQuery, metrics.Date).Scan(&metrics.SuccessRate)

	return metrics, nil
}

// SanitizeFTS5Query sanitizes user input for safe FTS5 query execution.
// It removes special characters that have meaning in FTS5 syntax,
// filters out short words that could be misinterpreted as column names,
// and wraps remaining terms in double quotes for exact matching.
// Returns an empty string if no valid search terms remain.
func SanitizeFTS5Query(input string) string {
	// FTS5 special characters that need to be removed or escaped
	// These characters have special meaning in FTS5 syntax:
	// - Quotes: ", '
	// - Operators: *, -, ^, ~
	// - Punctuation that causes parsing issues: ,, ., ?, !, :, @, (, ), [, ], {, }
	// - Boolean operators are handled by removing short words
	specialChars := `"'*-^~,.:?!@()[]{};<>\/|`

	// Replace special characters with spaces
	cleaned := input
	for _, char := range specialChars {
		cleaned = strings.ReplaceAll(cleaned, string(char), " ")
	}

	// Split into words and filter
	words := strings.Fields(cleaned)
	var validTerms []string

	for _, word := range words {
		// Skip empty words
		if word == "" {
			continue
		}

		// Skip words shorter than 3 characters to avoid:
		// 1. Column name interpretation (e.g., "al", "or", "to")
		// 2. FTS5 boolean operators (AND, OR, NOT - but we check length first)
		if len(word) < 3 {
			continue
		}

		// Skip FTS5 boolean operators (case-insensitive)
		upper := strings.ToUpper(word)
		if upper == "AND" || upper == "OR" || upper == "NOT" || upper == "NEAR" {
			continue
		}

		// Ensure the word contains at least one alphanumeric character
		hasAlnum := false
		for _, r := range word {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				hasAlnum = true
				break
			}
		}
		if !hasAlnum {
			continue
		}

		// Wrap each term in double quotes for exact matching
		// This prevents FTS5 from interpreting any remaining special syntax
		validTerms = append(validTerms, `"`+word+`"`)
	}

	// Return empty string if no valid terms
	if len(validTerms) == 0 {
		return ""
	}

	// Join terms with spaces (FTS5 default is AND between terms)
	return strings.Join(validTerms, " ")
}

// SearchByKeywords performs FTS search on template intents.
func (r *SQLiteRegistry) SearchByKeywords(ctx context.Context, queryStr string, limit int) ([]*Template, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	// Sanitize the query string for safe FTS5 execution
	sanitized := SanitizeFTS5Query(queryStr)
	if sanitized == "" {
		// No valid search terms after sanitization, return empty results
		return []*Template{}, nil
	}

	query := `
		SELECT
			t.id, t.name, t.description, t.intent, t.intent_embedding, t.intent_keywords,
			t.template_body, t.example_output, t.variable_schema, t.gbnf_grammar,
			t.task_type, t.domain, t.status, t.confidence_score, t.complexity_score,
			t.use_count, t.success_count, t.failure_count,
			t.source_type, t.source_model, t.source_request_id,
			t.created_at, t.updated_at, t.last_used_at, t.promoted_at, t.deprecated_at
		FROM templates_fts fts
		JOIN templates t ON t.rowid = fts.rowid
		WHERE templates_fts MATCH ?
		  AND t.status IN ('promoted', 'validated')
		ORDER BY fts.rank, t.confidence_score DESC
		LIMIT ?
	`

	return r.queryTemplates(ctx, query, sanitized, limit)
}

// queryTemplates is a helper to scan multiple templates from a query.
func (r *SQLiteRegistry) queryTemplates(ctx context.Context, query string, args ...interface{}) ([]*Template, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query templates: %w", err)
	}
	defer rows.Close()

	var templates []*Template
	for rows.Next() {
		t, err := r.scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		templates = append(templates, t)
	}

	return templates, rows.Err()
}

// scanTemplate scans a single template from a row.
func (r *SQLiteRegistry) scanTemplate(rows *sql.Rows) (*Template, error) {
	var t Template
	var description, exampleOutput, gbnfGrammar, domain, sourceModel, sourceRequestID sql.NullString
	var embedding []byte
	var keywordsJSON string
	var lastUsedAt, promotedAt, deprecatedAt sql.NullTime
	var taskType, status, sourceType string

	err := rows.Scan(
		&t.ID, &t.Name, &description, &t.Intent, &embedding, &keywordsJSON,
		&t.TemplateBody, &exampleOutput, &t.VariableSchema, &gbnfGrammar,
		&taskType, &domain, &status, &t.ConfidenceScore, &t.ComplexityScore,
		&t.UseCount, &t.SuccessCount, &t.FailureCount,
		&sourceType, &sourceModel, &sourceRequestID,
		&t.CreatedAt, &t.UpdatedAt, &lastUsedAt, &promotedAt, &deprecatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("scan template: %w", err)
	}

	// Set nullable fields
	if description.Valid {
		t.Description = description.String
	}
	if exampleOutput.Valid {
		t.ExampleOutput = exampleOutput.String
	}
	if gbnfGrammar.Valid {
		t.GBNFGrammar = gbnfGrammar.String
	}
	if domain.Valid {
		t.Domain = domain.String
	}
	if sourceModel.Valid {
		t.SourceModel = sourceModel.String
	}
	if sourceRequestID.Valid {
		t.SourceRequestID = sourceRequestID.String
	}
	if lastUsedAt.Valid {
		t.LastUsedAt = &lastUsedAt.Time
	}
	if promotedAt.Valid {
		t.PromotedAt = &promotedAt.Time
	}
	if deprecatedAt.Valid {
		t.DeprecatedAt = &deprecatedAt.Time
	}

	// Parse enum types
	t.TaskType = TaskType(taskType)
	t.Status = TemplateStatus(status)
	t.SourceType = TemplateSourceType(sourceType)

	// Parse embedding
	t.IntentEmbedding = EmbeddingFromBytes(embedding)

	// Parse keywords
	if err := json.Unmarshal([]byte(keywordsJSON), &t.IntentKeywords); err != nil {
		t.IntentKeywords = nil
	}

	return &t, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// LIFECYCLE OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════════

// UpdateStatus changes a template's lifecycle status.
func (r *SQLiteRegistry) UpdateStatus(ctx context.Context, id string, status TemplateStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	var query string
	var args []interface{}

	switch status {
	case StatusPromoted:
		query = `UPDATE templates SET status = ?, promoted_at = ?, updated_at = ? WHERE id = ?`
		args = []interface{}{string(status), now, now, id}
	case StatusDeprecated:
		query = `UPDATE templates SET status = ?, deprecated_at = ?, updated_at = ? WHERE id = ?`
		args = []interface{}{string(status), now, now, id}
	default:
		query = `UPDATE templates SET status = ?, updated_at = ? WHERE id = ?`
		args = []interface{}{string(status), now, id}
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("template not found: %s", id)
	}

	// Emit appropriate event
	switch status {
	case StatusPromoted:
		r.emit(&RegistryEvent{Type: EventTemplatePromoted, TemplateID: id})
		r.IncrementMetric(ctx, "promotions")
	case StatusDeprecated:
		r.emit(&RegistryEvent{Type: EventTemplateDeprecated, TemplateID: id})
		r.IncrementMetric(ctx, "deprecations")
	}

	return nil
}

// GetPromotionCandidates returns templates ready for promotion.
func (r *SQLiteRegistry) GetPromotionCandidates(ctx context.Context) ([]*Template, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := `
		SELECT
			t.id, t.name, t.description, t.intent, t.intent_embedding, t.intent_keywords,
			t.template_body, t.example_output, t.variable_schema, t.gbnf_grammar,
			t.task_type, t.domain, t.status, t.confidence_score, t.complexity_score,
			t.use_count, t.success_count, t.failure_count,
			t.source_type, t.source_model, t.source_request_id,
			t.created_at, t.updated_at, t.last_used_at, t.promoted_at, t.deprecated_at
		FROM templates t
		WHERE t.status = 'probation'
		  AND t.id IN (
			SELECT template_id FROM template_grading_log
			GROUP BY template_id
			HAVING COUNT(*) >= 3
			  AND SUM(CASE WHEN grade = 'pass' THEN 1 ELSE 0 END) >= 3
			  AND (SUM(CASE WHEN grade = 'pass' THEN 1.0 ELSE 0.0 END) / COUNT(*)) >= 0.9
		  )
	`

	return r.queryTemplates(ctx, query)
}

// GetDeprecationCandidates returns templates at risk of deprecation.
func (r *SQLiteRegistry) GetDeprecationCandidates(ctx context.Context) ([]*Template, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := `
		SELECT
			t.id, t.name, t.description, t.intent, t.intent_embedding, t.intent_keywords,
			t.template_body, t.example_output, t.variable_schema, t.gbnf_grammar,
			t.task_type, t.domain, t.status, t.confidence_score, t.complexity_score,
			t.use_count, t.success_count, t.failure_count,
			t.source_type, t.source_model, t.source_request_id,
			t.created_at, t.updated_at, t.last_used_at, t.promoted_at, t.deprecated_at
		FROM templates t
		WHERE t.status = 'probation'
		  AND t.id IN (
			SELECT template_id FROM template_grading_log
			GROUP BY template_id
			HAVING COUNT(*) >= 3
			  AND (SUM(CASE WHEN grade = 'fail' THEN 1.0 ELSE 0.0 END) / COUNT(*)) >= 0.5
		  )
	`

	return r.queryTemplates(ctx, query)
}

// ═══════════════════════════════════════════════════════════════════════════════
// USAGE TRACKING
// ═══════════════════════════════════════════════════════════════════════════════

// RecordUsage logs a template use event.
// Returns the log ID for tracking.
func (r *SQLiteRegistry) RecordUsage(ctx context.Context, log *UsageLog) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Insert usage log
	query := `
		INSERT INTO template_usage_log (
			template_id, session_id, request_id,
			user_input, extracted_variables, rendered_output,
			similarity_score, match_method,
			success, error_message,
			latency_ms, extraction_ms, rendering_ms, total_ms,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.ExecContext(ctx, query,
		log.TemplateID, nullString(log.SessionID), nullString(log.RequestID),
		log.UserInput, nullString(log.ExtractedVariables), nullString(log.RenderedOutput),
		log.SimilarityScore, nullString(log.MatchMethod),
		log.Success, nullString(log.ErrorMessage),
		log.LatencyMs, log.ExtractionMs, log.RenderingMs, log.TotalMs,
		time.Now(),
	)

	if err != nil {
		return 0, fmt.Errorf("insert usage log: %w", err)
	}

	// Get the inserted ID
	id, _ := result.LastInsertId()
	log.ID = id

	// Update template usage stats
	updateQuery := `
		UPDATE templates SET
			use_count = use_count + 1,
			success_count = success_count + ?,
			failure_count = failure_count + ?,
			last_used_at = ?,
			updated_at = ?
		WHERE id = ?
	`
	var successInc, failureInc int
	if log.Success {
		successInc = 1
	} else {
		failureInc = 1
	}
	now := time.Now()
	r.db.ExecContext(ctx, updateQuery, successInc, failureInc, now, now, log.TemplateID)

	// Increment metric
	r.IncrementMetric(ctx, "template_hits")

	r.emit(&RegistryEvent{Type: EventUsageRecorded, TemplateID: log.TemplateID, Payload: log})
	return id, nil
}

// GetUsageLogs retrieves usage history for a template.
func (r *SQLiteRegistry) GetUsageLogs(ctx context.Context, templateID string, limit int) ([]*UsageLog, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT
			id, template_id, session_id, request_id,
			user_input, extracted_variables, rendered_output,
			similarity_score, match_method,
			success, user_feedback,
			extraction_ms, rendering_ms, total_ms,
			created_at
		FROM template_usage_log
		WHERE template_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query, templateID, limit)
	if err != nil {
		return nil, fmt.Errorf("query usage logs: %w", err)
	}
	defer rows.Close()

	var logs []*UsageLog
	for rows.Next() {
		var log UsageLog
		var sessionID, requestID, extractedVars, renderedOutput, matchMethod, userFeedback sql.NullString
		var success sql.NullInt64

		err := rows.Scan(
			&log.ID, &log.TemplateID, &sessionID, &requestID,
			&log.UserInput, &extractedVars, &renderedOutput,
			&log.SimilarityScore, &matchMethod,
			&success, &userFeedback,
			&log.ExtractionMs, &log.RenderingMs, &log.TotalMs,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan usage log: %w", err)
		}

		if sessionID.Valid {
			log.SessionID = sessionID.String
		}
		if requestID.Valid {
			log.RequestID = requestID.String
		}
		if extractedVars.Valid {
			log.ExtractedVariables = extractedVars.String
		}
		if renderedOutput.Valid {
			log.RenderedOutput = renderedOutput.String
		}
		if matchMethod.Valid {
			log.MatchMethod = matchMethod.String
		}
		if userFeedback.Valid {
			log.UserFeedback = userFeedback.String
		}
		if success.Valid {
			log.Success = success.Int64 == 1
		}

		logs = append(logs, &log)
	}

	return logs, rows.Err()
}

// ═══════════════════════════════════════════════════════════════════════════════
// GRADING
// ═══════════════════════════════════════════════════════════════════════════════

// RecordGrade logs a grading result and updates confidence score.
func (r *SQLiteRegistry) RecordGrade(ctx context.Context, result *GradingResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Insert grading log
	query := `
		INSERT INTO template_grading_log (
			template_id, usage_log_id, grader_model,
			grade, grade_reason,
			correctness_score, completeness_score,
			confidence_delta, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		result.TemplateID, result.UsageLogID, result.GraderModel,
		string(result.Grade), nullString(result.GradeReason),
		result.CorrectnessScore, result.CompletenessScore,
		result.ConfidenceDelta, time.Now(),
	)

	if err != nil {
		return fmt.Errorf("insert grading log: %w", err)
	}

	// Update template confidence and success/failure counts
	var updateQuery string
	if result.Grade == GradePass {
		updateQuery = `
			UPDATE templates SET
				confidence_score = MIN(1.0, MAX(0.0, confidence_score + ?)),
				success_count = success_count + 1,
				updated_at = ?
			WHERE id = ?
		`
		r.IncrementMetric(ctx, "grading_passes")
	} else {
		updateQuery = `
			UPDATE templates SET
				confidence_score = MIN(1.0, MAX(0.0, confidence_score + ?)),
				failure_count = failure_count + 1,
				updated_at = ?
			WHERE id = ?
		`
		r.IncrementMetric(ctx, "grading_fails")
	}

	r.db.ExecContext(ctx, updateQuery, result.ConfidenceDelta, time.Now(), result.TemplateID)

	// Also update the usage log if provided
	if result.UsageLogID != nil {
		success := result.Grade == GradePass
		r.db.ExecContext(ctx,
			"UPDATE template_usage_log SET success = ? WHERE id = ?",
			success, *result.UsageLogID,
		)
	}

	r.emit(&RegistryEvent{Type: EventGradeRecorded, TemplateID: result.TemplateID, Payload: result})
	return nil
}

// GetGradingLogs retrieves grading history for a template.
func (r *SQLiteRegistry) GetGradingLogs(ctx context.Context, templateID string) ([]*GradingResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := `
		SELECT
			id, template_id, usage_log_id, grader_model,
			grade, grade_reason,
			correctness_score, completeness_score,
			confidence_delta, created_at
		FROM template_grading_log
		WHERE template_id = ?
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, templateID)
	if err != nil {
		return nil, fmt.Errorf("query grading logs: %w", err)
	}
	defer rows.Close()

	var results []*GradingResult
	for rows.Next() {
		var res GradingResult
		var usageLogID sql.NullInt64
		var gradeReason sql.NullString
		var grade string

		err := rows.Scan(
			&res.ID, &res.TemplateID, &usageLogID, &res.GraderModel,
			&grade, &gradeReason,
			&res.CorrectnessScore, &res.CompletenessScore,
			&res.ConfidenceDelta, &res.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan grading log: %w", err)
		}

		res.Grade = GradeType(grade)
		if usageLogID.Valid {
			res.UsageLogID = &usageLogID.Int64
		}
		if gradeReason.Valid {
			res.GradeReason = gradeReason.String
		}

		results = append(results, &res)
	}

	return results, rows.Err()
}

// GetPendingGrades returns usage logs that need grading.
func (r *SQLiteRegistry) GetPendingGrades(ctx context.Context, limit int) ([]*UsageLog, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT
			ul.id, ul.template_id, ul.session_id, ul.request_id,
			ul.user_input, ul.extracted_variables, ul.rendered_output,
			ul.similarity_score, ul.match_method,
			ul.success, ul.user_feedback,
			ul.extraction_ms, ul.rendering_ms, ul.total_ms,
			ul.created_at
		FROM template_usage_log ul
		JOIN templates t ON t.id = ul.template_id
		WHERE ul.success IS NULL
		  AND t.status = 'probation'
		ORDER BY ul.created_at ASC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("query pending grades: %w", err)
	}
	defer rows.Close()

	var logs []*UsageLog
	for rows.Next() {
		var log UsageLog
		var sessionID, requestID, extractedVars, renderedOutput, matchMethod, userFeedback sql.NullString
		var success sql.NullInt64

		err := rows.Scan(
			&log.ID, &log.TemplateID, &sessionID, &requestID,
			&log.UserInput, &extractedVars, &renderedOutput,
			&log.SimilarityScore, &matchMethod,
			&success, &userFeedback,
			&log.ExtractionMs, &log.RenderingMs, &log.TotalMs,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan usage log: %w", err)
		}

		if sessionID.Valid {
			log.SessionID = sessionID.String
		}
		if requestID.Valid {
			log.RequestID = requestID.String
		}
		if extractedVars.Valid {
			log.ExtractedVariables = extractedVars.String
		}
		if renderedOutput.Valid {
			log.RenderedOutput = renderedOutput.String
		}
		if matchMethod.Valid {
			log.MatchMethod = matchMethod.String
		}
		if userFeedback.Valid {
			log.UserFeedback = userFeedback.String
		}

		logs = append(logs, &log)
	}

	return logs, rows.Err()
}

// ═══════════════════════════════════════════════════════════════════════════════
// METRICS
// ═══════════════════════════════════════════════════════════════════════════════

// GetMetrics retrieves cognitive metrics for a date.
func (r *SQLiteRegistry) GetMetrics(ctx context.Context, date string) (*CognitiveMetrics, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := `
		SELECT metric, value FROM cognitive_metrics WHERE date = ?
	`

	rows, err := r.db.QueryContext(ctx, query, date)
	if err != nil {
		return nil, fmt.Errorf("query metrics: %w", err)
	}
	defer rows.Close()

	metrics := &CognitiveMetrics{Date: date}
	for rows.Next() {
		var metric string
		var value float64
		if err := rows.Scan(&metric, &value); err != nil {
			return nil, fmt.Errorf("scan metric: %w", err)
		}

		switch metric {
		case "total_requests":
			metrics.TotalRequests = int64(value)
		case "template_hits":
			metrics.TemplateHits = int64(value)
		case "template_misses":
			metrics.TemplateMisses = int64(value)
		case "local_model_calls":
			metrics.LocalModelCalls = int64(value)
		case "frontier_calls":
			metrics.FrontierCalls = int64(value)
		case "distillation_attempts":
			metrics.DistillationAttempts = int64(value)
		case "distillation_successes":
			metrics.DistillationSuccesses = int64(value)
		case "pass_grades":
			metrics.PassGrades = int(value)
		case "fail_grades":
			metrics.FailGrades = int(value)
		case "partial_grades":
			metrics.PartialGrades = int(value)
		case "promotions":
			metrics.Promotions = int(value)
		case "deprecations":
			metrics.Deprecations = int(value)
		}
	}

	return metrics, rows.Err()
}

// IncrementMetric atomically increments a metric counter.
func (r *SQLiteRegistry) IncrementMetric(ctx context.Context, metric string) error {
	date := time.Now().Format("2006-01-02")

	query := `
		INSERT INTO cognitive_metrics (date, metric, value)
		VALUES (?, ?, 1)
		ON CONFLICT(date, metric) DO UPDATE SET value = value + 1
	`

	_, err := r.db.ExecContext(ctx, query, date, metric)
	if err != nil {
		return fmt.Errorf("increment metric: %w", err)
	}

	return nil
}

// GetSystemHealth returns overall system health stats.
func (r *SQLiteRegistry) GetSystemHealth(ctx context.Context) (*SystemHealth, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	health := &SystemHealth{GeneratedAt: time.Now()}

	// Get template counts by status
	statusQuery := `
		SELECT status, COUNT(*) FROM templates GROUP BY status
	`
	rows, err := r.db.QueryContext(ctx, statusQuery)
	if err != nil {
		return nil, fmt.Errorf("query template counts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			continue
		}
		health.TotalTemplates += count
		switch TemplateStatus(status) {
		case StatusProbation:
			health.ProbationTemplates = count
		case StatusValidated:
			health.ValidatedTemplates = count
		case StatusPromoted:
			health.PromotedTemplates = count
		case StatusDeprecated:
			health.DeprecatedTemplates = count
		}
	}

	// Get average confidence and success rate
	avgQuery := `
		SELECT
			AVG(confidence_score),
			AVG(CASE WHEN success_count + failure_count > 0
				THEN (success_count * 100.0 / (success_count + failure_count))
				ELSE 50.0 END)
		FROM templates
		WHERE status IN ('promoted', 'validated')
	`
	r.db.QueryRowContext(ctx, avgQuery).Scan(&health.AvgConfidenceScore, &health.AvgSuccessRate)

	// Get today's activity
	today := time.Now().Format("2006-01-02")
	todayQuery := `
		SELECT COUNT(*) FROM templates WHERE date(created_at) = ?
	`
	r.db.QueryRowContext(ctx, todayQuery, today).Scan(&health.TemplatesCreatedToday)

	promotedTodayQuery := `
		SELECT COUNT(*) FROM templates WHERE date(promoted_at) = ?
	`
	r.db.QueryRowContext(ctx, promotedTodayQuery, today).Scan(&health.TemplatesPromotedToday)

	return health, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// EMBEDDING CACHE
// ═══════════════════════════════════════════════════════════════════════════════

// CacheEmbedding stores an embedding for later retrieval.
func (r *SQLiteRegistry) CacheEmbedding(ctx context.Context, sourceType, sourceID, textHash string, embedding Embedding, model string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	query := `
		INSERT OR REPLACE INTO embedding_cache (
			source_type, source_id, text_hash, embedding, embedding_model, embedding_dim, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		sourceType, sourceID, textHash,
		embedding.ToBytes(), model, len(embedding),
		time.Now(),
	)

	if err != nil {
		return fmt.Errorf("cache embedding: %w", err)
	}

	return nil
}

// GetCachedEmbedding retrieves a cached embedding.
func (r *SQLiteRegistry) GetCachedEmbedding(ctx context.Context, sourceType, sourceID string) (Embedding, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := `
		SELECT embedding FROM embedding_cache
		WHERE source_type = ? AND source_id = ?
	`

	var data []byte
	err := r.db.QueryRowContext(ctx, query, sourceType, sourceID).Scan(&data)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get cached embedding: %w", err)
	}

	return EmbeddingFromBytes(data), nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// DISTILLATION TRACKING
// ═══════════════════════════════════════════════════════════════════════════════

// RecordDistillation logs a distillation request and its outcome.
func (r *SQLiteRegistry) RecordDistillation(ctx context.Context, req *DistillationRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	query := `
		INSERT INTO distillation_requests (
			id, user_input, task_type,
			similarity_score, route_reason,
			frontier_model, solution,
			template_created, template_id, extraction_error,
			compilation_passed, schema_valid, grammar_generated,
			frontier_ms, extraction_ms, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		req.ID, req.UserInput, string(req.TaskType),
		req.SimilarityScore, nullString(req.RouteReason),
		req.FrontierModel, nullString(req.Solution),
		req.TemplateCreated, nullString(req.TemplateID), nullString(req.ExtractionError),
		req.CompilationPassed, req.SchemaValid, req.GrammarGenerated,
		req.FrontierMs, req.ExtractionMs, time.Now(),
	)

	if err != nil {
		return fmt.Errorf("record distillation: %w", err)
	}

	// Increment metrics
	r.IncrementMetric(ctx, "distillation_requests")
	if req.TemplateCreated {
		r.IncrementMetric(ctx, "distillation_successes")
	}

	return nil
}

// GetDistillationHistory retrieves recent distillation requests.
func (r *SQLiteRegistry) GetDistillationHistory(ctx context.Context, limit int) ([]*DistillationRequest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT
			id, user_input, task_type,
			similarity_score, route_reason,
			frontier_model, solution,
			template_created, template_id, extraction_error,
			compilation_passed, schema_valid, grammar_generated,
			frontier_ms, extraction_ms, created_at
		FROM distillation_requests
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("query distillation history: %w", err)
	}
	defer rows.Close()

	var requests []*DistillationRequest
	for rows.Next() {
		var req DistillationRequest
		var taskType, routeReason, solution, templateID, extractionError sql.NullString

		err := rows.Scan(
			&req.ID, &req.UserInput, &taskType,
			&req.SimilarityScore, &routeReason,
			&req.FrontierModel, &solution,
			&req.TemplateCreated, &templateID, &extractionError,
			&req.CompilationPassed, &req.SchemaValid, &req.GrammarGenerated,
			&req.FrontierMs, &req.ExtractionMs, &req.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan distillation request: %w", err)
		}

		if taskType.Valid {
			req.TaskType = TaskType(taskType.String)
		}
		if routeReason.Valid {
			req.RouteReason = routeReason.String
		}
		if solution.Valid {
			req.Solution = solution.String
		}
		if templateID.Valid {
			req.TemplateID = templateID.String
		}
		if extractionError.Valid {
			req.ExtractionError = extractionError.String
		}

		requests = append(requests, &req)
	}

	return requests, rows.Err()
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPER FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════════

// nullString converts a string to sql.NullString.
func nullString(s string) sql.NullString {
	return sql.NullString{
		String: s,
		Valid:  s != "",
	}
}
