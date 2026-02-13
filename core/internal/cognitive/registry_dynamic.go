// Package cognitive provides the cognitive architecture layer for Cortex.
// This file implements the Dynamic Skill Registry for auto-generated skills.
package cognitive

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// DYNAMIC SKILL REGISTRY
// Extends SQLiteRegistry with dynamic skill and failure pattern support
// ═══════════════════════════════════════════════════════════════════════════════

// DynamicSkillRegistry manages both templates and auto-generated skills.
type DynamicSkillRegistry struct {
	*SQLiteRegistry // Embed for template support
	db              *sql.DB
	mu              sync.RWMutex
}

// NewDynamicSkillRegistry creates a registry supporting both templates and skills.
func NewDynamicSkillRegistry(db *sql.DB) *DynamicSkillRegistry {
	return &DynamicSkillRegistry{
		SQLiteRegistry: NewSQLiteRegistry(db),
		db:             db,
	}
}

// InitDynamicSchema creates the dynamic skill tables.
func (r *DynamicSkillRegistry) InitDynamicSchema(ctx context.Context) error {
	schema := `
		-- Dynamic Skills (auto-generated from experience)
		CREATE TABLE IF NOT EXISTS dynamic_skills (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			type TEXT NOT NULL, -- GENERAL, TASK_SPECIFIC, AGENT_SPECIFIC
			category TEXT,
			source TEXT NOT NULL, -- STATIC, DISTILLED, MERGED, SHARED
			status TEXT DEFAULT 'probation', -- probation, active, deprecated

			-- Content
			description TEXT NOT NULL,
			when_to_apply TEXT,
			steps TEXT, -- JSON array
			examples TEXT, -- JSON array

			-- Matching
			intent TEXT,
			intent_embedding BLOB,
			keywords TEXT, -- JSON array

			-- Provenance
			source_reflections TEXT, -- JSON array of reflection IDs
			source_agents TEXT, -- JSON array of agent IDs
			confidence REAL DEFAULT 0.5,

			-- Usage tracking
			usage_count INTEGER DEFAULT 0,
			last_used DATETIME,
			successes INTEGER DEFAULT 0,
			failures INTEGER DEFAULT 0,

			-- Version control
			version INTEGER DEFAULT 1,
			parent_id TEXT,

			-- Timestamps
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_skills_type ON dynamic_skills(type);
		CREATE INDEX IF NOT EXISTS idx_skills_category ON dynamic_skills(category);
		CREATE INDEX IF NOT EXISTS idx_skills_source ON dynamic_skills(source);
		CREATE INDEX IF NOT EXISTS idx_skills_status ON dynamic_skills(status);
		CREATE INDEX IF NOT EXISTS idx_skills_confidence ON dynamic_skills(confidence DESC);

		-- Failure Patterns (what NOT to do)
		CREATE TABLE IF NOT EXISTS failure_patterns (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			type TEXT NOT NULL, -- GENERAL, TASK_SPECIFIC
			category TEXT,

			-- Content
			description TEXT NOT NULL,
			error_signature TEXT, -- How to detect
			recovery TEXT, -- How to fix
			prevention TEXT, -- How to avoid

			-- Matching
			keywords TEXT, -- JSON array
			error_embedding BLOB,

			-- Provenance
			source_reflections TEXT, -- JSON array
			source_agents TEXT, -- JSON array
			confidence REAL DEFAULT 0.5,

			-- Tracking
			times_triggered INTEGER DEFAULT 0,
			times_prevented INTEGER DEFAULT 0,

			-- Timestamps
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_failures_type ON failure_patterns(type);
		CREATE INDEX IF NOT EXISTS idx_failures_category ON failure_patterns(category);

		-- FTS for skill search
		CREATE VIRTUAL TABLE IF NOT EXISTS dynamic_skills_fts USING fts5(
			name, description, when_to_apply, intent, keywords,
			content=dynamic_skills, content_rowid=rowid
		);

		-- FTS for failure pattern search
		CREATE VIRTUAL TABLE IF NOT EXISTS failure_patterns_fts USING fts5(
			name, description, error_signature, prevention,
			content=failure_patterns, content_rowid=rowid
		);
	`

	_, err := r.db.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("init dynamic skill schema: %w", err)
	}

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// DYNAMIC SKILL CRUD
// ═══════════════════════════════════════════════════════════════════════════════

// CreateSkill stores a new dynamic skill.
func (r *DynamicSkillRegistry) CreateSkill(ctx context.Context, skill *DynamicSkill) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	stepsJSON, _ := json.Marshal(skill.Steps)
	examplesJSON, _ := json.Marshal(skill.Examples)
	keywordsJSON, _ := json.Marshal(skill.Keywords)
	sourceRefsJSON, _ := json.Marshal(skill.SourceReflections)
	sourceAgentsJSON, _ := json.Marshal(skill.SourceAgents)

	query := `
		INSERT INTO dynamic_skills (
			id, name, type, category, source, status,
			description, when_to_apply, steps, examples,
			intent, intent_embedding, keywords,
			source_reflections, source_agents, confidence,
			usage_count, successes, failures,
			version, parent_id,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	_, err := r.db.ExecContext(ctx, query,
		skill.ID, skill.Name, string(skill.Type), skill.Category,
		string(skill.Source), string(skill.Status),
		skill.Description, skill.WhenToApply,
		string(stepsJSON), string(examplesJSON),
		skill.Intent, skill.IntentEmbedding.ToBytes(), string(keywordsJSON),
		string(sourceRefsJSON), string(sourceAgentsJSON), skill.Confidence,
		skill.UsageCount, skill.Successes, skill.Failures,
		skill.Version, nullString(skill.ParentID),
		now, now,
	)

	if err != nil {
		return fmt.Errorf("insert dynamic skill: %w", err)
	}

	return nil
}

// GetSkill retrieves a dynamic skill by ID.
func (r *DynamicSkillRegistry) GetSkill(ctx context.Context, id string) (*DynamicSkill, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := `
		SELECT id, name, type, category, source, status,
		       description, when_to_apply, steps, examples,
		       intent, intent_embedding, keywords,
		       source_reflections, source_agents, confidence,
		       usage_count, last_used, successes, failures,
		       version, parent_id,
		       created_at, updated_at
		FROM dynamic_skills
		WHERE id = ?
	`

	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanSkill(row)
}

// UpdateSkill updates an existing dynamic skill.
func (r *DynamicSkillRegistry) UpdateSkill(ctx context.Context, skill *DynamicSkill) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	stepsJSON, _ := json.Marshal(skill.Steps)
	examplesJSON, _ := json.Marshal(skill.Examples)
	keywordsJSON, _ := json.Marshal(skill.Keywords)
	sourceRefsJSON, _ := json.Marshal(skill.SourceReflections)
	sourceAgentsJSON, _ := json.Marshal(skill.SourceAgents)

	query := `
		UPDATE dynamic_skills SET
			name = ?, type = ?, category = ?, status = ?,
			description = ?, when_to_apply = ?, steps = ?, examples = ?,
			intent = ?, intent_embedding = ?, keywords = ?,
			source_reflections = ?, source_agents = ?, confidence = ?,
			usage_count = ?, last_used = ?, successes = ?, failures = ?,
			version = ?, updated_at = ?
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query,
		skill.Name, string(skill.Type), skill.Category, string(skill.Status),
		skill.Description, skill.WhenToApply,
		string(stepsJSON), string(examplesJSON),
		skill.Intent, skill.IntentEmbedding.ToBytes(), string(keywordsJSON),
		string(sourceRefsJSON), string(sourceAgentsJSON), skill.Confidence,
		skill.UsageCount, skill.LastUsed, skill.Successes, skill.Failures,
		skill.Version, time.Now(),
		skill.ID,
	)

	if err != nil {
		return fmt.Errorf("update dynamic skill: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("skill not found: %s", skill.ID)
	}

	return nil
}

// DeleteSkill removes a dynamic skill (soft delete via status).
func (r *DynamicSkillRegistry) DeleteSkill(ctx context.Context, id string) error {
	return r.UpdateSkillStatus(ctx, id, SkillStatusDeprecated)
}

// UpdateSkillStatus changes a skill's lifecycle status.
func (r *DynamicSkillRegistry) UpdateSkillStatus(ctx context.Context, id string, status SkillStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	query := `UPDATE dynamic_skills SET status = ?, updated_at = ? WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query, string(status), time.Now(), id)
	if err != nil {
		return fmt.Errorf("update skill status: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("skill not found: %s", id)
	}

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// SKILL QUERIES
// ═══════════════════════════════════════════════════════════════════════════════

// ListSkills returns skills filtered by type and status.
func (r *DynamicSkillRegistry) ListSkills(ctx context.Context, skillType *SkillType, status *SkillStatus) ([]*DynamicSkill, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := `
		SELECT id, name, type, category, source, status,
		       description, when_to_apply, steps, examples,
		       intent, intent_embedding, keywords,
		       source_reflections, source_agents, confidence,
		       usage_count, last_used, successes, failures,
		       version, parent_id,
		       created_at, updated_at
		FROM dynamic_skills
		WHERE 1=1
	`
	args := []interface{}{}

	if skillType != nil {
		query += " AND type = ?"
		args = append(args, string(*skillType))
	}

	if status != nil {
		query += " AND status = ?"
		args = append(args, string(*status))
	}

	query += " ORDER BY confidence DESC, usage_count DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}
	defer rows.Close()

	var skills []*DynamicSkill
	for rows.Next() {
		skill, err := r.scanSkillRows(rows)
		if err != nil {
			return nil, err
		}
		skills = append(skills, skill)
	}

	return skills, rows.Err()
}

// ListActiveSkills returns all skills usable for routing.
func (r *DynamicSkillRegistry) ListActiveSkills(ctx context.Context) ([]*DynamicSkill, error) {
	probation := SkillStatusProbation
	active := SkillStatusActive

	// Get both probation and active skills
	probationSkills, err := r.ListSkills(ctx, nil, &probation)
	if err != nil {
		return nil, err
	}

	activeSkills, err := r.ListSkills(ctx, nil, &active)
	if err != nil {
		return nil, err
	}

	return append(activeSkills, probationSkills...), nil
}

// ListSkillsByCategory returns skills in a specific category.
func (r *DynamicSkillRegistry) ListSkillsByCategory(ctx context.Context, category string) ([]*DynamicSkill, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := `
		SELECT id, name, type, category, source, status,
		       description, when_to_apply, steps, examples,
		       intent, intent_embedding, keywords,
		       source_reflections, source_agents, confidence,
		       usage_count, last_used, successes, failures,
		       version, parent_id,
		       created_at, updated_at
		FROM dynamic_skills
		WHERE category = ?
		  AND status IN ('active', 'probation')
		ORDER BY confidence DESC
	`

	rows, err := r.db.QueryContext(ctx, query, category)
	if err != nil {
		return nil, fmt.Errorf("list skills by category: %w", err)
	}
	defer rows.Close()

	var skills []*DynamicSkill
	for rows.Next() {
		skill, err := r.scanSkillRows(rows)
		if err != nil {
			return nil, err
		}
		skills = append(skills, skill)
	}

	return skills, rows.Err()
}

// SearchSkills performs keyword search on skills.
func (r *DynamicSkillRegistry) SearchSkills(ctx context.Context, query string, limit int) ([]*DynamicSkill, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	sanitized := SanitizeFTS5Query(query)
	if sanitized == "" {
		return []*DynamicSkill{}, nil
	}

	sqlQuery := `
		SELECT s.id, s.name, s.type, s.category, s.source, s.status,
		       s.description, s.when_to_apply, s.steps, s.examples,
		       s.intent, s.intent_embedding, s.keywords,
		       s.source_reflections, s.source_agents, s.confidence,
		       s.usage_count, s.last_used, s.successes, s.failures,
		       s.version, s.parent_id,
		       s.created_at, s.updated_at
		FROM dynamic_skills s
		JOIN dynamic_skills_fts fts ON s.rowid = fts.rowid
		WHERE dynamic_skills_fts MATCH ?
		  AND s.status IN ('active', 'probation')
		ORDER BY fts.rank, s.confidence DESC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, sqlQuery, sanitized, limit)
	if err != nil {
		return nil, fmt.Errorf("search skills: %w", err)
	}
	defer rows.Close()

	var skills []*DynamicSkill
	for rows.Next() {
		skill, err := r.scanSkillRows(rows)
		if err != nil {
			return nil, err
		}
		skills = append(skills, skill)
	}

	return skills, rows.Err()
}

// ═══════════════════════════════════════════════════════════════════════════════
// SKILL USAGE TRACKING
// ═══════════════════════════════════════════════════════════════════════════════

// RecordSkillUsage logs a skill use and updates counters.
func (r *DynamicSkillRegistry) RecordSkillUsage(ctx context.Context, skillID string, success bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var query string
	if success {
		query = `
			UPDATE dynamic_skills SET
				usage_count = usage_count + 1,
				successes = successes + 1,
				last_used = ?,
				updated_at = ?
			WHERE id = ?
		`
	} else {
		query = `
			UPDATE dynamic_skills SET
				usage_count = usage_count + 1,
				failures = failures + 1,
				last_used = ?,
				updated_at = ?
			WHERE id = ?
		`
	}

	now := time.Now()
	_, err := r.db.ExecContext(ctx, query, now, now, skillID)
	if err != nil {
		return fmt.Errorf("record skill usage: %w", err)
	}

	return nil
}

// GetPromotionCandidateSkills returns skills ready for promotion.
func (r *DynamicSkillRegistry) GetPromotionCandidateSkills(ctx context.Context) ([]*DynamicSkill, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Criteria: probation, 5+ uses, 80%+ success rate, 0.7+ confidence
	query := `
		SELECT id, name, type, category, source, status,
		       description, when_to_apply, steps, examples,
		       intent, intent_embedding, keywords,
		       source_reflections, source_agents, confidence,
		       usage_count, last_used, successes, failures,
		       version, parent_id,
		       created_at, updated_at
		FROM dynamic_skills
		WHERE status = 'probation'
		  AND usage_count >= 5
		  AND confidence >= 0.7
		  AND (successes * 100.0 / (successes + failures + 1)) >= 80
		ORDER BY confidence DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get promotion candidates: %w", err)
	}
	defer rows.Close()

	var skills []*DynamicSkill
	for rows.Next() {
		skill, err := r.scanSkillRows(rows)
		if err != nil {
			return nil, err
		}
		skills = append(skills, skill)
	}

	return skills, rows.Err()
}

// GetDeprecationCandidateSkills returns skills at risk of deprecation.
func (r *DynamicSkillRegistry) GetDeprecationCandidateSkills(ctx context.Context) ([]*DynamicSkill, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Criteria: probation, 5+ uses, <50% success rate
	query := `
		SELECT id, name, type, category, source, status,
		       description, when_to_apply, steps, examples,
		       intent, intent_embedding, keywords,
		       source_reflections, source_agents, confidence,
		       usage_count, last_used, successes, failures,
		       version, parent_id,
		       created_at, updated_at
		FROM dynamic_skills
		WHERE status = 'probation'
		  AND usage_count >= 5
		  AND (successes * 100.0 / (successes + failures + 1)) < 50
		ORDER BY created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get deprecation candidates: %w", err)
	}
	defer rows.Close()

	var skills []*DynamicSkill
	for rows.Next() {
		skill, err := r.scanSkillRows(rows)
		if err != nil {
			return nil, err
		}
		skills = append(skills, skill)
	}

	return skills, rows.Err()
}

// ═══════════════════════════════════════════════════════════════════════════════
// FAILURE PATTERN OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════════

// CreateFailurePattern stores a new failure pattern.
func (r *DynamicSkillRegistry) CreateFailurePattern(ctx context.Context, fp *FailurePattern) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	keywordsJSON, _ := json.Marshal(fp.Keywords)
	sourceRefsJSON, _ := json.Marshal(fp.SourceReflections)
	sourceAgentsJSON, _ := json.Marshal(fp.SourceAgents)

	query := `
		INSERT INTO failure_patterns (
			id, name, type, category,
			description, error_signature, recovery, prevention,
			keywords, error_embedding,
			source_reflections, source_agents, confidence,
			times_triggered, times_prevented,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		fp.ID, fp.Name, string(fp.Type), fp.Category,
		fp.Description, fp.ErrorSignature, fp.Recovery, fp.Prevention,
		string(keywordsJSON), fp.ErrorEmbedding.ToBytes(),
		string(sourceRefsJSON), string(sourceAgentsJSON), fp.Confidence,
		fp.TimesTriggered, fp.TimesPrevented,
		time.Now(),
	)

	if err != nil {
		return fmt.Errorf("insert failure pattern: %w", err)
	}

	return nil
}

// GetFailurePattern retrieves a failure pattern by ID.
func (r *DynamicSkillRegistry) GetFailurePattern(ctx context.Context, id string) (*FailurePattern, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := `
		SELECT id, name, type, category,
		       description, error_signature, recovery, prevention,
		       keywords, error_embedding,
		       source_reflections, source_agents, confidence,
		       times_triggered, times_prevented,
		       created_at
		FROM failure_patterns
		WHERE id = ?
	`

	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanFailurePattern(row)
}

// ListFailurePatterns returns all failure patterns.
func (r *DynamicSkillRegistry) ListFailurePatterns(ctx context.Context, category *string) ([]*FailurePattern, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := `
		SELECT id, name, type, category,
		       description, error_signature, recovery, prevention,
		       keywords, error_embedding,
		       source_reflections, source_agents, confidence,
		       times_triggered, times_prevented,
		       created_at
		FROM failure_patterns
	`
	args := []interface{}{}

	if category != nil {
		query += " WHERE category = ?"
		args = append(args, *category)
	}

	query += " ORDER BY confidence DESC, times_triggered DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list failure patterns: %w", err)
	}
	defer rows.Close()

	var patterns []*FailurePattern
	for rows.Next() {
		fp, err := r.scanFailurePatternRows(rows)
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, fp)
	}

	return patterns, rows.Err()
}

// SearchFailurePatterns searches for matching failure patterns.
func (r *DynamicSkillRegistry) SearchFailurePatterns(ctx context.Context, query string, limit int) ([]*FailurePattern, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	sanitized := SanitizeFTS5Query(query)
	if sanitized == "" {
		return []*FailurePattern{}, nil
	}

	sqlQuery := `
		SELECT f.id, f.name, f.type, f.category,
		       f.description, f.error_signature, f.recovery, f.prevention,
		       f.keywords, f.error_embedding,
		       f.source_reflections, f.source_agents, f.confidence,
		       f.times_triggered, f.times_prevented,
		       f.created_at
		FROM failure_patterns f
		JOIN failure_patterns_fts fts ON f.rowid = fts.rowid
		WHERE failure_patterns_fts MATCH ?
		ORDER BY fts.rank, f.confidence DESC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, sqlQuery, sanitized, limit)
	if err != nil {
		return nil, fmt.Errorf("search failure patterns: %w", err)
	}
	defer rows.Close()

	var patterns []*FailurePattern
	for rows.Next() {
		fp, err := r.scanFailurePatternRows(rows)
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, fp)
	}

	return patterns, rows.Err()
}

// RecordFailureTriggered increments the triggered counter.
func (r *DynamicSkillRegistry) RecordFailureTriggered(ctx context.Context, patternID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	query := `UPDATE failure_patterns SET times_triggered = times_triggered + 1 WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, patternID)
	return err
}

// RecordFailurePrevented increments the prevented counter.
func (r *DynamicSkillRegistry) RecordFailurePrevented(ctx context.Context, patternID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	query := `UPDATE failure_patterns SET times_prevented = times_prevented + 1 WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, patternID)
	return err
}

// ═══════════════════════════════════════════════════════════════════════════════
// STATISTICS
// ═══════════════════════════════════════════════════════════════════════════════

// GetDynamicRegistryStats returns statistics about dynamic skills.
func (r *DynamicSkillRegistry) GetDynamicRegistryStats(ctx context.Context) (*DynamicRegistryStats, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := &DynamicRegistryStats{}

	// Count templates
	r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM templates WHERE source_type = 'static'
	`).Scan(&stats.StaticTemplates)

	r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM templates WHERE source_type = 'distilled'
	`).Scan(&stats.DynamicTemplates)

	// Count skills by source
	r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM dynamic_skills WHERE source = 'STATIC'
	`).Scan(&stats.StaticSkills)

	r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM dynamic_skills WHERE source IN ('DISTILLED', 'MERGED')
	`).Scan(&stats.DynamicSkills)

	r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM dynamic_skills WHERE source = 'SHARED'
	`).Scan(&stats.SharedSkills)

	r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM failure_patterns
	`).Scan(&stats.FailurePatterns)

	stats.TotalSkills = stats.StaticSkills + stats.DynamicSkills + stats.SharedSkills

	// Average confidence and success rate
	r.db.QueryRowContext(ctx, `
		SELECT COALESCE(AVG(confidence), 0) FROM dynamic_skills WHERE status = 'active'
	`).Scan(&stats.AvgSkillConfidence)

	r.db.QueryRowContext(ctx, `
		SELECT COALESCE(AVG(
			CASE WHEN successes + failures > 0
			THEN successes * 100.0 / (successes + failures)
			ELSE 50 END
		), 50) FROM dynamic_skills WHERE status = 'active'
	`).Scan(&stats.AvgSuccessRate)

	return stats, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// SCAN HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

func (r *DynamicSkillRegistry) scanSkill(row *sql.Row) (*DynamicSkill, error) {
	var skill DynamicSkill
	var skillType, source, status string
	var category, intent, parentID sql.NullString
	var stepsJSON, examplesJSON, keywordsJSON, sourceRefsJSON, sourceAgentsJSON string
	var embedding []byte
	var lastUsed sql.NullTime

	err := row.Scan(
		&skill.ID, &skill.Name, &skillType, &category, &source, &status,
		&skill.Description, &skill.WhenToApply, &stepsJSON, &examplesJSON,
		&intent, &embedding, &keywordsJSON,
		&sourceRefsJSON, &sourceAgentsJSON, &skill.Confidence,
		&skill.UsageCount, &lastUsed, &skill.Successes, &skill.Failures,
		&skill.Version, &parentID,
		&skill.CreatedAt, &skill.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("skill not found")
		}
		return nil, fmt.Errorf("scan skill: %w", err)
	}

	skill.Type = SkillType(skillType)
	skill.Source = SkillSource(source)
	skill.Status = SkillStatus(status)

	if category.Valid {
		skill.Category = category.String
	}
	if intent.Valid {
		skill.Intent = intent.String
	}
	if parentID.Valid {
		skill.ParentID = parentID.String
	}
	if lastUsed.Valid {
		skill.LastUsed = lastUsed.Time
	}

	skill.IntentEmbedding = EmbeddingFromBytes(embedding)

	json.Unmarshal([]byte(stepsJSON), &skill.Steps)
	json.Unmarshal([]byte(examplesJSON), &skill.Examples)
	json.Unmarshal([]byte(keywordsJSON), &skill.Keywords)
	json.Unmarshal([]byte(sourceRefsJSON), &skill.SourceReflections)
	json.Unmarshal([]byte(sourceAgentsJSON), &skill.SourceAgents)

	return &skill, nil
}

func (r *DynamicSkillRegistry) scanSkillRows(rows *sql.Rows) (*DynamicSkill, error) {
	var skill DynamicSkill
	var skillType, source, status string
	var category, intent, parentID sql.NullString
	var stepsJSON, examplesJSON, keywordsJSON, sourceRefsJSON, sourceAgentsJSON string
	var embedding []byte
	var lastUsed sql.NullTime

	err := rows.Scan(
		&skill.ID, &skill.Name, &skillType, &category, &source, &status,
		&skill.Description, &skill.WhenToApply, &stepsJSON, &examplesJSON,
		&intent, &embedding, &keywordsJSON,
		&sourceRefsJSON, &sourceAgentsJSON, &skill.Confidence,
		&skill.UsageCount, &lastUsed, &skill.Successes, &skill.Failures,
		&skill.Version, &parentID,
		&skill.CreatedAt, &skill.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan skill: %w", err)
	}

	skill.Type = SkillType(skillType)
	skill.Source = SkillSource(source)
	skill.Status = SkillStatus(status)

	if category.Valid {
		skill.Category = category.String
	}
	if intent.Valid {
		skill.Intent = intent.String
	}
	if parentID.Valid {
		skill.ParentID = parentID.String
	}
	if lastUsed.Valid {
		skill.LastUsed = lastUsed.Time
	}

	skill.IntentEmbedding = EmbeddingFromBytes(embedding)

	json.Unmarshal([]byte(stepsJSON), &skill.Steps)
	json.Unmarshal([]byte(examplesJSON), &skill.Examples)
	json.Unmarshal([]byte(keywordsJSON), &skill.Keywords)
	json.Unmarshal([]byte(sourceRefsJSON), &skill.SourceReflections)
	json.Unmarshal([]byte(sourceAgentsJSON), &skill.SourceAgents)

	return &skill, nil
}

func (r *DynamicSkillRegistry) scanFailurePattern(row *sql.Row) (*FailurePattern, error) {
	var fp FailurePattern
	var fpType string
	var category sql.NullString
	var keywordsJSON, sourceRefsJSON, sourceAgentsJSON string
	var embedding []byte

	err := row.Scan(
		&fp.ID, &fp.Name, &fpType, &category,
		&fp.Description, &fp.ErrorSignature, &fp.Recovery, &fp.Prevention,
		&keywordsJSON, &embedding,
		&sourceRefsJSON, &sourceAgentsJSON, &fp.Confidence,
		&fp.TimesTriggered, &fp.TimesPrevented,
		&fp.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("failure pattern not found")
		}
		return nil, fmt.Errorf("scan failure pattern: %w", err)
	}

	fp.Type = SkillType(fpType)
	if category.Valid {
		fp.Category = category.String
	}

	fp.ErrorEmbedding = EmbeddingFromBytes(embedding)

	json.Unmarshal([]byte(keywordsJSON), &fp.Keywords)
	json.Unmarshal([]byte(sourceRefsJSON), &fp.SourceReflections)
	json.Unmarshal([]byte(sourceAgentsJSON), &fp.SourceAgents)

	return &fp, nil
}

func (r *DynamicSkillRegistry) scanFailurePatternRows(rows *sql.Rows) (*FailurePattern, error) {
	var fp FailurePattern
	var fpType string
	var category sql.NullString
	var keywordsJSON, sourceRefsJSON, sourceAgentsJSON string
	var embedding []byte

	err := rows.Scan(
		&fp.ID, &fp.Name, &fpType, &category,
		&fp.Description, &fp.ErrorSignature, &fp.Recovery, &fp.Prevention,
		&keywordsJSON, &embedding,
		&sourceRefsJSON, &sourceAgentsJSON, &fp.Confidence,
		&fp.TimesTriggered, &fp.TimesPrevented,
		&fp.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan failure pattern: %w", err)
	}

	fp.Type = SkillType(fpType)
	if category.Valid {
		fp.Category = category.String
	}

	fp.ErrorEmbedding = EmbeddingFromBytes(embedding)

	json.Unmarshal([]byte(keywordsJSON), &fp.Keywords)
	json.Unmarshal([]byte(sourceRefsJSON), &fp.SourceReflections)
	json.Unmarshal([]byte(sourceAgentsJSON), &fp.SourceAgents)

	return &fp, nil
}
