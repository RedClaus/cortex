// Package memory provides enhanced memory capabilities for Cortex.
// This file implements the Voyager-style Skill Library for CR-025 Phase 3.
//
// The Skill Library stores successful execution patterns as reusable skills,
// inspired by the Voyager paper (Wang et al. 2023). Skills are learned only
// from high-confidence successful executions and retrieved based on semantic
// similarity to new tasks.
//
// Brain Alignment: This component mirrors procedural memory in the basal ganglia,
// storing "how to do things" patterns that can be retrieved and applied to new situations.
package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ============================================================================
// SKILL LIBRARY TYPES
// ============================================================================

// Skill represents an executable pattern learned from successful execution.
// Skills are atomic units of procedural knowledge that can be reused.
type Skill struct {
	Name        string   `json:"name"`         // Human-readable skill name
	Description string   `json:"description"`  // What the skill does
	Pattern     string   `json:"pattern"`      // Template or code pattern
	InputSchema string   `json:"input_schema"` // JSON schema for inputs (optional)
	Examples    []string `json:"examples"`     // Example inputs that triggered this skill
	Tags        []string `json:"tags"`         // Semantic tags for categorization
}

// ExecutionTrace captures the details of a successful execution for learning.
// Only high-confidence traces are converted into skills.
type ExecutionTrace struct {
	// Identity
	SessionID string `json:"session_id"`
	TraceID   string `json:"trace_id"`

	// Input/Output
	UserInput     string `json:"user_input"`
	TaskSummary   string `json:"task_summary"`
	GeneratedCode string `json:"generated_code"`

	// Quality Signals
	Success    bool    `json:"success"`
	Confidence float64 `json:"confidence"`
	LatencyMS  int64   `json:"latency_ms"`
	TokensUsed int     `json:"tokens_used"`

	// Metadata
	DetectedTags []string  `json:"detected_tags"`
	CreatedAt    time.Time `json:"created_at"`
}

// StoredSkill represents a skill as stored in the database.
// Wraps Skill with storage metadata for tracking success/failure.
type StoredSkill struct {
	// Identity
	ID        string    `json:"id"`
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Skill Content
	Skill Skill `json:"skill"`

	// Provenance
	Source    string `json:"source"`     // "execution", "manual", "synthesis"
	SessionID string `json:"session_id"` // Source session
	ParentID  string `json:"parent_id"`  // For evolved skills

	// Quality Signals
	Confidence   float64   `json:"confidence"`
	SuccessCount int       `json:"success_count"`
	FailureCount int       `json:"failure_count"`
	Embedding    []float32 `json:"embedding,omitempty"`

	// Access Tracking
	LastAccessedAt time.Time `json:"last_accessed_at"`
	AccessCount    int       `json:"access_count"`
}

// SuccessRate returns the success ratio for this skill.
// Uses Bayesian smoothing to handle low sample counts.
func (ss *StoredSkill) SuccessRate() float64 {
	// Bayesian estimate with Beta(1,1) prior (uniform)
	// Posterior mean = (successes + 1) / (successes + failures + 2)
	return float64(ss.SuccessCount+1) / float64(ss.SuccessCount+ss.FailureCount+2)
}

// Touch updates access timestamp and count.
func (ss *StoredSkill) Touch() {
	ss.LastAccessedAt = time.Now()
	ss.AccessCount++
}

// ============================================================================
// SKILL LIBRARY THRESHOLDS
// ============================================================================

const (
	// MinConfidenceForLearning is the minimum confidence required to learn from an execution.
	// Only high-confidence executions become skills (Voyager pattern).
	MinConfidenceForLearning = 0.8

	// MinSuccessRateForRetrieval is the minimum success rate for a skill to be retrieved.
	// Skills below this threshold are not returned in FindRelevantSkills.
	MinSuccessRateForRetrieval = 0.6

	// MinSimilarityForMatch is the minimum embedding similarity for skill matching.
	MinSimilarityForMatch = 0.7

	// SkillIDPrefix is the prefix for skill IDs.
	SkillIDPrefix = "skill"
)

// ============================================================================
// SKILL LIBRARY IMPLEMENTATION
// ============================================================================

// SkillLibrary manages executable skill patterns (Voyager-style).
// It stores successful execution patterns and retrieves them for reuse.
type SkillLibrary struct {
	db       *sql.DB
	embedder Embedder
}

// NewSkillLibrary creates a new skill library with database and embedder.
func NewSkillLibrary(db *sql.DB, embedder Embedder) *SkillLibrary {
	return &SkillLibrary{
		db:       db,
		embedder: embedder,
	}
}

// LearnFromExecution creates a skill from a successful execution trace.
// Only learns from high-confidence successful executions (Voyager pattern).
//
// Returns nil if the trace doesn't meet learning criteria (not an error).
func (sl *SkillLibrary) LearnFromExecution(ctx context.Context, exec ExecutionTrace) error {
	// Voyager pattern: Only learn from successful, high-confidence executions
	if !exec.Success {
		log.Printf("skill_library: skipping learning - execution not successful")
		return nil
	}
	if exec.Confidence < MinConfidenceForLearning {
		log.Printf("skill_library: skipping learning - confidence %.2f < threshold %.2f",
			exec.Confidence, MinConfidenceForLearning)
		return nil
	}

	// Generate skill name from execution
	skillName := sl.generateSkillName(exec)

	// Synthesize skill from trace
	skill := Skill{
		Name:        skillName,
		Description: exec.TaskSummary,
		Pattern:     exec.GeneratedCode,
		Examples:    []string{exec.UserInput},
		Tags:        exec.DetectedTags,
	}

	// Create stored skill with metadata
	now := time.Now()
	stored := &StoredSkill{
		ID:           GenerateID(SkillIDPrefix, uuid.New().String()),
		Version:      1,
		CreatedAt:    now,
		UpdatedAt:    now,
		Skill:        skill,
		Source:       "execution",
		SessionID:    exec.SessionID,
		Confidence:   exec.Confidence,
		SuccessCount: 1, // First success (the execution we learned from)
		FailureCount: 0,
	}

	// Generate embedding for retrieval
	if sl.embedder != nil {
		embeddingText := skill.Name + " " + skill.Description
		if len(skill.Tags) > 0 {
			embeddingText += " " + strings.Join(skill.Tags, " ")
		}
		embedding, err := sl.embedder.Embed(ctx, embeddingText)
		if err != nil {
			// Log but don't fail - embedding is optional for storage
			log.Printf("skill_library: embed failed for %s: %v", stored.ID, err)
		} else {
			stored.Embedding = embedding
		}
	}

	// Save to database
	return sl.save(ctx, stored)
}

// FindRelevantSkills retrieves skills matching the current task.
// Only returns skills with success rate >= MinSuccessRateForRetrieval.
func (sl *SkillLibrary) FindRelevantSkills(ctx context.Context, taskDescription string, limit int) ([]Skill, error) {
	if limit <= 0 {
		limit = 5
	}

	// First, try semantic search if embedder is available
	if sl.embedder != nil {
		skills, err := sl.searchByEmbedding(ctx, taskDescription, limit)
		if err == nil && len(skills) > 0 {
			return skills, nil
		}
		// Fall through to text search if embedding search fails
		if err != nil {
			log.Printf("skill_library: embedding search failed, falling back to text: %v", err)
		}
	}

	// Fallback: text-based search
	return sl.searchByText(ctx, taskDescription, limit)
}

// searchByEmbedding performs semantic similarity search using embeddings.
func (sl *SkillLibrary) searchByEmbedding(ctx context.Context, query string, limit int) ([]Skill, error) {
	// Generate query embedding
	queryEmbedding, err := sl.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	// Fetch all skills with embeddings
	storedSkills, err := sl.getAllWithEmbeddings(ctx)
	if err != nil {
		return nil, fmt.Errorf("get skills with embeddings: %w", err)
	}

	// Calculate similarity scores
	type scoredSkill struct {
		skill      *StoredSkill
		similarity float64
	}

	var scored []scoredSkill
	for _, stored := range storedSkills {
		if len(stored.Embedding) == 0 {
			continue
		}

		similarity := CosineSimilarity(queryEmbedding, stored.Embedding)

		// Filter by similarity threshold and success rate
		if similarity >= MinSimilarityForMatch && stored.SuccessRate() >= MinSuccessRateForRetrieval {
			scored = append(scored, scoredSkill{
				skill:      stored,
				similarity: similarity,
			})
		}
	}

	// Sort by similarity descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].similarity > scored[j].similarity
	})

	// Return top N skills
	var skills []Skill
	for i := 0; i < len(scored) && i < limit; i++ {
		stored := scored[i].skill
		stored.Touch()
		// Update access tracking in background (don't block)
		go func(id string) {
			updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			sl.updateAccessTime(updateCtx, id)
		}(stored.ID)

		skills = append(skills, stored.Skill)
	}

	return skills, nil
}

// searchByText performs text-based search using LIKE matching.
func (sl *SkillLibrary) searchByText(ctx context.Context, query string, limit int) ([]Skill, error) {
	// Build search pattern
	searchPattern := "%" + query + "%"

	sqlQuery := `
		SELECT id, version, skill_json, source, session_id, parent_id,
		       confidence, success_count, failure_count, embedding,
		       created_at, updated_at, last_accessed_at, access_count
		FROM skills
		WHERE (skill_json LIKE ? OR skill_json LIKE ?)
		  AND (CAST(success_count AS REAL) + 1) / (success_count + failure_count + 2) >= ?
		ORDER BY success_count DESC, confidence DESC
		LIMIT ?
	`

	// Search in name and description (within JSON)
	namePattern := "%\"name\":\"%"
	rows, err := sl.db.QueryContext(ctx, sqlQuery, searchPattern, namePattern, MinSuccessRateForRetrieval, limit)
	if err != nil {
		return nil, fmt.Errorf("search skills: %w", err)
	}
	defer rows.Close()

	storedSkills, err := sl.scanSkills(rows)
	if err != nil {
		return nil, err
	}

	// Extract skills from stored wrappers
	var skills []Skill
	for _, stored := range storedSkills {
		skills = append(skills, stored.Skill)
	}

	return skills, nil
}

// RecordOutcome updates skill success/failure counts based on usage outcome.
func (sl *SkillLibrary) RecordOutcome(ctx context.Context, skillID string, success bool) error {
	now := time.Now().Format(time.RFC3339)

	var query string
	if success {
		query = `
			UPDATE skills
			SET success_count = success_count + 1,
			    updated_at = ?,
			    last_accessed_at = ?
			WHERE id = ?
		`
	} else {
		query = `
			UPDATE skills
			SET failure_count = failure_count + 1,
			    updated_at = ?,
			    last_accessed_at = ?
			WHERE id = ?
		`
	}

	result, err := sl.db.ExecContext(ctx, query, now, now, skillID)
	if err != nil {
		return fmt.Errorf("record outcome: update failed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("record outcome: rows affected check: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("record outcome: skill not found: %s", skillID)
	}

	return nil
}

// ============================================================================
// PRIVATE METHODS
// ============================================================================

// save stores a skill in the database.
func (sl *SkillLibrary) save(ctx context.Context, stored *StoredSkill) error {
	// Serialize skill to JSON
	skillJSON, err := serializeSkill(stored.Skill)
	if err != nil {
		return fmt.Errorf("save skill: serialize failed: %w", err)
	}

	// Convert embedding to bytes
	embeddingBytes := Float32SliceToBytes(stored.Embedding)

	query := `
		INSERT INTO skills (
			id, version, skill_json, source, session_id, parent_id,
			confidence, success_count, failure_count, embedding,
			created_at, updated_at, last_accessed_at, access_count
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			version = version + 1,
			skill_json = excluded.skill_json,
			confidence = excluded.confidence,
			success_count = excluded.success_count,
			failure_count = excluded.failure_count,
			embedding = excluded.embedding,
			updated_at = excluded.updated_at,
			last_accessed_at = excluded.last_accessed_at,
			access_count = excluded.access_count
	`

	_, err = sl.db.ExecContext(ctx, query,
		stored.ID,
		stored.Version,
		skillJSON,
		stored.Source,
		stored.SessionID,
		stored.ParentID,
		stored.Confidence,
		stored.SuccessCount,
		stored.FailureCount,
		embeddingBytes,
		stored.CreatedAt.Format(time.RFC3339),
		stored.UpdatedAt.Format(time.RFC3339),
		stored.LastAccessedAt.Format(time.RFC3339),
		stored.AccessCount,
	)
	if err != nil {
		return fmt.Errorf("save skill: insert failed: %w", err)
	}

	return nil
}

// GetByID retrieves a skill by its ID.
func (sl *SkillLibrary) GetByID(ctx context.Context, id string) (*StoredSkill, error) {
	query := `
		SELECT id, version, skill_json, source, session_id, parent_id,
		       confidence, success_count, failure_count, embedding,
		       created_at, updated_at, last_accessed_at, access_count
		FROM skills
		WHERE id = ?
	`

	row := sl.db.QueryRowContext(ctx, query, id)
	return sl.scanSkill(row)
}

// getAllWithEmbeddings returns all skills that have embeddings.
func (sl *SkillLibrary) getAllWithEmbeddings(ctx context.Context) ([]*StoredSkill, error) {
	query := `
		SELECT id, version, skill_json, source, session_id, parent_id,
		       confidence, success_count, failure_count, embedding,
		       created_at, updated_at, last_accessed_at, access_count
		FROM skills
		WHERE embedding IS NOT NULL AND length(embedding) > 0
	`

	rows, err := sl.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get skills with embeddings: %w", err)
	}
	defer rows.Close()

	return sl.scanSkills(rows)
}

// updateAccessTime updates the last_accessed_at timestamp for a skill.
func (sl *SkillLibrary) updateAccessTime(ctx context.Context, id string) {
	now := time.Now().Format(time.RFC3339)
	query := `
		UPDATE skills
		SET last_accessed_at = ?,
		    access_count = access_count + 1
		WHERE id = ?
	`
	_, _ = sl.db.ExecContext(ctx, query, now, id) // Ignore errors for background update
}

// GetSuccessfulSkills retrieves the most successful skills.
func (sl *SkillLibrary) GetSuccessfulSkills(ctx context.Context, limit int) ([]*StoredSkill, error) {
	query := `
		SELECT id, version, skill_json, source, session_id, parent_id,
		       confidence, success_count, failure_count, embedding,
		       created_at, updated_at, last_accessed_at, access_count
		FROM skills
		WHERE (success_count + failure_count) >= 3
		  AND (CAST(success_count AS REAL) + 1) / (success_count + failure_count + 2) >= ?
		ORDER BY success_count DESC, confidence DESC
		LIMIT ?
	`

	rows, err := sl.db.QueryContext(ctx, query, MinSuccessRateForRetrieval, limit)
	if err != nil {
		return nil, fmt.Errorf("get successful skills: %w", err)
	}
	defer rows.Close()

	return sl.scanSkills(rows)
}

// ============================================================================
// SCANNING HELPERS
// ============================================================================

// scanSkill scans a single row into a StoredSkill.
func (sl *SkillLibrary) scanSkill(row *sql.Row) (*StoredSkill, error) {
	var stored StoredSkill
	var skillJSON string
	var sessionID, parentID sql.NullString
	var embeddingBytes []byte
	var createdAt, updatedAt, lastAccessedAt string

	err := row.Scan(
		&stored.ID,
		&stored.Version,
		&skillJSON,
		&stored.Source,
		&sessionID,
		&parentID,
		&stored.Confidence,
		&stored.SuccessCount,
		&stored.FailureCount,
		&embeddingBytes,
		&createdAt,
		&updatedAt,
		&lastAccessedAt,
		&stored.AccessCount,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("skill not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scan skill: %w", err)
	}

	// Parse skill JSON
	skill, err := deserializeSkill(skillJSON)
	if err != nil {
		return nil, fmt.Errorf("deserialize skill: %w", err)
	}
	stored.Skill = skill

	// Parse nullable strings
	stored.SessionID = sessionID.String
	stored.ParentID = parentID.String

	// Parse embedding
	stored.Embedding = BytesToFloat32Slice(embeddingBytes)

	// Parse timestamps
	if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
		stored.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
		stored.UpdatedAt = t
	}
	if t, err := time.Parse(time.RFC3339, lastAccessedAt); err == nil {
		stored.LastAccessedAt = t
	}

	return &stored, nil
}

// scanSkills scans multiple rows into a slice of StoredSkill.
func (sl *SkillLibrary) scanSkills(rows *sql.Rows) ([]*StoredSkill, error) {
	var skills []*StoredSkill

	for rows.Next() {
		var stored StoredSkill
		var skillJSON string
		var sessionID, parentID sql.NullString
		var embeddingBytes []byte
		var createdAt, updatedAt, lastAccessedAt string

		err := rows.Scan(
			&stored.ID,
			&stored.Version,
			&skillJSON,
			&stored.Source,
			&sessionID,
			&parentID,
			&stored.Confidence,
			&stored.SuccessCount,
			&stored.FailureCount,
			&embeddingBytes,
			&createdAt,
			&updatedAt,
			&lastAccessedAt,
			&stored.AccessCount,
		)
		if err != nil {
			return nil, fmt.Errorf("scan skill row: %w", err)
		}

		// Parse skill JSON
		skill, err := deserializeSkill(skillJSON)
		if err != nil {
			log.Printf("skill_library: failed to deserialize skill %s: %v", stored.ID, err)
			continue // Skip malformed skills
		}
		stored.Skill = skill

		// Parse nullable strings
		stored.SessionID = sessionID.String
		stored.ParentID = parentID.String

		// Parse embedding
		stored.Embedding = BytesToFloat32Slice(embeddingBytes)

		// Parse timestamps
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			stored.CreatedAt = t
		}
		if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
			stored.UpdatedAt = t
		}
		if t, err := time.Parse(time.RFC3339, lastAccessedAt); err == nil {
			stored.LastAccessedAt = t
		}

		skills = append(skills, &stored)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate skill rows: %w", err)
	}

	return skills, nil
}

// ============================================================================
// SERIALIZATION HELPERS
// ============================================================================

// serializeSkill converts a Skill to JSON string for storage.
func serializeSkill(skill Skill) (string, error) {
	data, err := json.Marshal(skill)
	if err != nil {
		return "", fmt.Errorf("marshal skill: %w", err)
	}
	return string(data), nil
}

// deserializeSkill converts a JSON string back to a Skill.
func deserializeSkill(data string) (Skill, error) {
	var skill Skill
	if err := json.Unmarshal([]byte(data), &skill); err != nil {
		return Skill{}, fmt.Errorf("unmarshal skill: %w", err)
	}
	return skill, nil
}

// ============================================================================
// SKILL NAME GENERATION
// ============================================================================

// generateSkillName creates a descriptive name for a skill from execution trace.
// Uses a simple heuristic based on task summary and tags.
func (sl *SkillLibrary) generateSkillName(exec ExecutionTrace) string {
	// Start with task summary if available
	if exec.TaskSummary != "" {
		// Extract first sentence or phrase (up to 50 chars)
		name := exec.TaskSummary
		if len(name) > 50 {
			// Find a natural break point
			breakPoints := []string{". ", ", ", " - ", ": "}
			for _, bp := range breakPoints {
				if idx := strings.Index(name[:50], bp); idx > 0 {
					name = name[:idx]
					break
				}
			}
			if len(name) > 50 {
				name = name[:47] + "..."
			}
		}
		return name
	}

	// Fall back to tags if available
	if len(exec.DetectedTags) > 0 {
		// Combine first 3 tags
		tags := exec.DetectedTags
		if len(tags) > 3 {
			tags = tags[:3]
		}
		return strings.Join(tags, "_") + "_skill"
	}

	// Last resort: generate from user input
	if exec.UserInput != "" {
		words := strings.Fields(exec.UserInput)
		if len(words) > 5 {
			words = words[:5]
		}
		return strings.Join(words, "_") + "_skill"
	}

	// Fallback: anonymous skill with timestamp
	return fmt.Sprintf("skill_%d", time.Now().Unix())
}

// ============================================================================
// STATISTICS AND ANALYTICS
// ============================================================================

// SkillStats provides aggregate statistics about the skill library.
type SkillStats struct {
	TotalSkills       int     `json:"total_skills"`
	SuccessfulSkills  int     `json:"successful_skills"` // Success rate >= 0.6
	AvgSuccessRate    float64 `json:"avg_success_rate"`
	TotalExecutions   int     `json:"total_executions"` // Sum of success + failure counts
	TopTags           []string `json:"top_tags"`
	MostUsedSkillID   string  `json:"most_used_skill_id"`
	MostUsedSkillName string  `json:"most_used_skill_name"`
}

// GetStats returns aggregate statistics about the skill library.
func (sl *SkillLibrary) GetStats(ctx context.Context) (*SkillStats, error) {
	// Get basic counts
	var stats SkillStats

	query := `
		SELECT
			COUNT(*) as total,
			SUM(CASE WHEN (CAST(success_count AS REAL) + 1) / (success_count + failure_count + 2) >= ? THEN 1 ELSE 0 END) as successful,
			COALESCE(AVG((CAST(success_count AS REAL) + 1) / (success_count + failure_count + 2)), 0) as avg_rate,
			COALESCE(SUM(success_count + failure_count), 0) as total_executions
		FROM skills
	`

	row := sl.db.QueryRowContext(ctx, query, MinSuccessRateForRetrieval)
	err := row.Scan(&stats.TotalSkills, &stats.SuccessfulSkills, &stats.AvgSuccessRate, &stats.TotalExecutions)
	if err != nil {
		return nil, fmt.Errorf("get stats: %w", err)
	}

	// Get most used skill
	mostUsedQuery := `
		SELECT id, skill_json
		FROM skills
		ORDER BY access_count DESC
		LIMIT 1
	`
	var skillJSON string
	row = sl.db.QueryRowContext(ctx, mostUsedQuery)
	err = row.Scan(&stats.MostUsedSkillID, &skillJSON)
	if err == nil && skillJSON != "" {
		if skill, err := deserializeSkill(skillJSON); err == nil {
			stats.MostUsedSkillName = skill.Name
		}
	}

	return &stats, nil
}

// ============================================================================
// SKILL EVOLUTION
// ============================================================================

// EvolveSkill creates a new version of a skill with updated pattern.
// The new skill starts fresh with success/failure counts but inherits lineage.
func (sl *SkillLibrary) EvolveSkill(ctx context.Context, parentID string, newPattern string, reason string) (*StoredSkill, error) {
	// Get parent skill
	parent, err := sl.GetByID(ctx, parentID)
	if err != nil {
		return nil, fmt.Errorf("evolve skill: parent not found: %w", err)
	}

	// Create evolved skill
	now := time.Now()
	evolved := &StoredSkill{
		ID:        GenerateID(SkillIDPrefix, uuid.New().String()),
		Version:   parent.Version + 1,
		CreatedAt: now,
		UpdatedAt: now,
		Skill: Skill{
			Name:        parent.Skill.Name + " (v" + fmt.Sprintf("%d", parent.Version+1) + ")",
			Description: parent.Skill.Description + " [Evolved: " + reason + "]",
			Pattern:     newPattern,
			InputSchema: parent.Skill.InputSchema,
			Examples:    parent.Skill.Examples,
			Tags:        parent.Skill.Tags,
		},
		Source:       "evolution",
		SessionID:    parent.SessionID,
		ParentID:     parentID,
		Confidence:   parent.Confidence * 0.9, // Slightly lower confidence for evolved version
		SuccessCount: 0,                       // Start fresh
		FailureCount: 0,
	}

	// Generate new embedding
	if sl.embedder != nil {
		embeddingText := evolved.Skill.Name + " " + evolved.Skill.Description
		embedding, err := sl.embedder.Embed(ctx, embeddingText)
		if err != nil {
			log.Printf("skill_library: embed failed for evolved skill: %v", err)
		} else {
			evolved.Embedding = embedding
		}
	}

	if err := sl.save(ctx, evolved); err != nil {
		return nil, fmt.Errorf("evolve skill: save failed: %w", err)
	}

	return evolved, nil
}

// GetEvolutionHistory retrieves the full evolution chain for a skill.
func (sl *SkillLibrary) GetEvolutionHistory(ctx context.Context, id string) ([]*StoredSkill, error) {
	var history []*StoredSkill

	current, err := sl.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get evolution history: %w", err)
	}

	// Walk up the parent chain
	for current != nil && current.ParentID != "" {
		parent, err := sl.GetByID(ctx, current.ParentID)
		if err != nil {
			break // Parent may have been deleted
		}
		history = append([]*StoredSkill{parent}, history...) // Prepend
		current = parent
	}

	// Add the original skill at the end
	skill, _ := sl.GetByID(ctx, id)
	if skill != nil {
		history = append(history, skill)
	}

	return history, nil
}
