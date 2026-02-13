// Package memory provides enhanced memory capabilities for Cortex.
// This file implements the Strategic Memory Store for CR-015.
package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// MemoryTier represents the confidence/permanence level of a strategic memory
type MemoryTier string

const (
	TierTentative MemoryTier = "tentative" // New patterns, <3 applications
	TierCandidate MemoryTier = "candidate" // Emerging patterns, 3-10 applications
	TierProven    MemoryTier = "proven"    // High-confidence patterns, >10 applications, >80% success
	TierIdentity  MemoryTier = "identity"  // Core patterns, permanent, >90% success, many sessions
)

// TierPromotionThresholds defines when memories are promoted between tiers
type TierPromotionThresholds struct {
	// Tentative -> Candidate
	MinApplyCountForCandidate int

	// Candidate -> Proven
	MinApplyCountForProven  int
	MinSuccessRateForProven float64

	// Proven -> Identity
	MinApplyCountForIdentity     int
	MinSuccessRateForIdentity    float64
	MinUniqueSessionsForIdentity int
	MinAgeForIdentity            time.Duration
}

// DefaultTierPromotionThresholds returns sensible defaults
func DefaultTierPromotionThresholds() TierPromotionThresholds {
	return TierPromotionThresholds{
		MinApplyCountForCandidate:    3,
		MinApplyCountForProven:       10,
		MinSuccessRateForProven:      0.80,
		MinApplyCountForIdentity:     25,
		MinSuccessRateForIdentity:    0.90,
		MinUniqueSessionsForIdentity: 5,
		MinAgeForIdentity:            30 * 24 * time.Hour, // 30 days
	}
}

// StrategicMemory represents a high-level principle or heuristic
// derived from patterns of success and failure.
type StrategicMemory struct {
	ID             string     `json:"id"`
	Principle      string     `json:"principle"`       // The rule/heuristic
	Category       string     `json:"category"`        // e.g., "debugging", "docker", "git"
	TriggerPattern string     `json:"trigger_pattern"` // When to apply this principle
	Tier           MemoryTier `json:"tier"`            // Confidence/permanence level
	SuccessCount   int        `json:"success_count"`   // Times following this worked
	FailureCount   int        `json:"failure_count"`   // Times ignoring this failed
	ApplyCount     int        `json:"apply_count"`     // Total times applied
	SuccessRate    float64    `json:"success_rate"`    // Auto-computed by SQLite
	Confidence     float64    `json:"confidence"`      // How confident we are (0-1)
	SourceSessions []string   `json:"source_sessions"` // Session IDs that formed this
	Embedding      []float32  `json:"embedding,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	LastAppliedAt  *time.Time `json:"last_applied_at,omitempty"`

	// CR-025-LITE: Evolution tracking
	Version        int      `json:"version"`         // Increment on significant changes
	ParentID       string   `json:"parent_id"`       // Previous version's ID (empty for originals)
	EvolutionChain []string `json:"evolution_chain"` // Full ancestry [oldest...newest]
}

// StrategicMemoryStore manages strategic memory persistence in SQLite.
type StrategicMemoryStore struct {
	db       *sql.DB
	embedder Embedder
}

// NewStrategicMemoryStore creates a new strategic memory store.
func NewStrategicMemoryStore(db *sql.DB, embedder Embedder) *StrategicMemoryStore {
	return &StrategicMemoryStore{
		db:       db,
		embedder: embedder,
	}
}

// Create inserts a new strategic memory with auto-generated ID and embedding.
func (s *StrategicMemoryStore) Create(ctx context.Context, mem *StrategicMemory) error {
	if mem.Principle == "" {
		return fmt.Errorf("create strategic memory: principle is required")
	}

	// Generate ID with prefix
	mem.ID = "strat_" + uuid.New().String()

	// Generate embedding for the principle
	if s.embedder != nil {
		embeddingText := mem.Principle
		if mem.TriggerPattern != "" {
			embeddingText = mem.Principle + " " + mem.TriggerPattern
		}
		embedding, err := s.embedder.Embed(ctx, embeddingText)
		if err != nil {
			return fmt.Errorf("create strategic memory: embed failed: %w", err)
		}
		mem.Embedding = embedding
	}

	// Marshal source sessions to JSON
	sourceSessionsJSON, err := json.Marshal(mem.SourceSessions)
	if err != nil {
		return fmt.Errorf("create strategic memory: marshal source_sessions: %w", err)
	}

	// Convert embedding to bytes
	embeddingBytes := Float32SliceToBytes(mem.Embedding)

	// Set timestamps
	now := time.Now()
	mem.CreatedAt = now
	mem.UpdatedAt = now

	// Set default confidence if not provided
	if mem.Confidence == 0 {
		mem.Confidence = 0.5
	}

	// Set default tier to tentative
	if mem.Tier == "" {
		mem.Tier = TierTentative
	}

	// CR-025-LITE: Set default version
	if mem.Version == 0 {
		mem.Version = 1
	}

	// Marshal evolution chain to JSON
	evolutionChainJSON, err := json.Marshal(mem.EvolutionChain)
	if err != nil {
		evolutionChainJSON = []byte("[]")
	}

	query := `
		INSERT INTO strategic_memory (
			id, principle, category, trigger_pattern, tier,
			success_count, failure_count, apply_count,
			confidence, source_sessions, embedding,
			created_at, updated_at, last_applied_at,
			version, parent_id, evolution_chain
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var lastAppliedAt interface{}
	if mem.LastAppliedAt != nil {
		lastAppliedAt = mem.LastAppliedAt.Format(time.RFC3339)
	}

	_, err = s.db.ExecContext(ctx, query,
		mem.ID,
		mem.Principle,
		mem.Category,
		mem.TriggerPattern,
		string(mem.Tier),
		mem.SuccessCount,
		mem.FailureCount,
		mem.ApplyCount,
		mem.Confidence,
		string(sourceSessionsJSON),
		embeddingBytes,
		mem.CreatedAt.Format(time.RFC3339),
		mem.UpdatedAt.Format(time.RFC3339),
		lastAppliedAt,
		mem.Version,
		mem.ParentID,
		string(evolutionChainJSON),
	)
	if err != nil {
		return fmt.Errorf("create strategic memory: insert failed: %w", err)
	}

	return nil
}

// Get retrieves a strategic memory by ID.
func (s *StrategicMemoryStore) Get(ctx context.Context, id string) (*StrategicMemory, error) {
	query := `
		SELECT id, principle, category, trigger_pattern, tier,
		       success_count, failure_count, apply_count,
		       success_rate, confidence, source_sessions, embedding,
		       created_at, updated_at, last_applied_at,
		       COALESCE(version, 1), COALESCE(parent_id, ''), COALESCE(evolution_chain, '[]')
		FROM strategic_memory
		WHERE id = ?
	`

	row := s.db.QueryRowContext(ctx, query, id)
	return scanStrategicMemory(row)
}

// RecordSuccess increments success_count and updates timestamps.
func (s *StrategicMemoryStore) RecordSuccess(ctx context.Context, id string) error {
	now := time.Now().Format(time.RFC3339)
	query := `
		UPDATE strategic_memory
		SET success_count = success_count + 1,
		    apply_count = apply_count + 1,
		    updated_at = ?,
		    last_applied_at = ?
		WHERE id = ?
	`

	result, err := s.db.ExecContext(ctx, query, now, now, id)
	if err != nil {
		return fmt.Errorf("record success: update failed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("record success: rows affected check: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("record success: memory not found: %s", id)
	}

	return nil
}

// RecordFailure increments failure_count and updates timestamp.
func (s *StrategicMemoryStore) RecordFailure(ctx context.Context, id string) error {
	now := time.Now().Format(time.RFC3339)
	query := `
		UPDATE strategic_memory
		SET failure_count = failure_count + 1,
		    apply_count = apply_count + 1,
		    updated_at = ?,
		    last_applied_at = ?
		WHERE id = ?
	`

	result, err := s.db.ExecContext(ctx, query, now, now, id)
	if err != nil {
		return fmt.Errorf("record failure: update failed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("record failure: rows affected check: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("record failure: memory not found: %s", id)
	}

	return nil
}

// GetTopPrinciples returns the most successful principles.
// Only includes principles with at least MinEvidenceForReliable observations.
func (s *StrategicMemoryStore) GetTopPrinciples(ctx context.Context, limit int) ([]StrategicMemory, error) {
	query := `
		SELECT id, principle, category, trigger_pattern, tier,
		       success_count, failure_count, apply_count,
		       success_rate, confidence, source_sessions, embedding,
		       created_at, updated_at, last_applied_at,
		       COALESCE(version, 1), COALESCE(parent_id, ''), COALESCE(evolution_chain, '[]')
		FROM strategic_memory
		WHERE (success_count + failure_count) >= ?
		ORDER BY success_rate DESC, confidence DESC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, MinEvidenceForReliable, limit)
	if err != nil {
		return nil, fmt.Errorf("get top principles: query failed: %w", err)
	}
	defer rows.Close()

	return scanStrategicMemories(rows)
}

// GetByCategory returns strategic memories filtered by category.
func (s *StrategicMemoryStore) GetByCategory(ctx context.Context, category string, limit int) ([]StrategicMemory, error) {
	query := `
		SELECT id, principle, category, trigger_pattern, tier,
		       success_count, failure_count, apply_count,
		       success_rate, confidence, source_sessions, embedding,
		       created_at, updated_at, last_applied_at,
		       COALESCE(version, 1), COALESCE(parent_id, ''), COALESCE(evolution_chain, '[]')
		FROM strategic_memory
		WHERE category = ?
		ORDER BY success_rate DESC, confidence DESC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, category, limit)
	if err != nil {
		return nil, fmt.Errorf("get by category: query failed: %w", err)
	}
	defer rows.Close()

	return scanStrategicMemories(rows)
}

// SearchSimilar finds strategic memories similar to the query using vector similarity.
func (s *StrategicMemoryStore) SearchSimilar(ctx context.Context, query string, limit int) ([]StrategicMemory, error) {
	if s.embedder == nil {
		return nil, fmt.Errorf("search similar: embedder not configured")
	}

	// Generate embedding for query using fast path (5-second timeout)
	// This is critical for responsiveness - we don't want to block on slow embeddings
	queryEmbedding, err := s.embedder.EmbedFast(ctx, query)
	if err != nil {
		// Fall back to FTS search if embedding fails
		return s.SearchFTS(ctx, query, limit)
	}

	// Fetch top memories by confidence/success, limited to prevent slow scans
	// ORDER BY confidence DESC prioritizes high-quality memories
	// LIMIT 100 prevents scanning all memories (425+ rows)
	sqlQuery := `
		SELECT id, principle, category, trigger_pattern, tier,
		       success_count, failure_count, apply_count,
		       success_rate, confidence, source_sessions, embedding,
		       created_at, updated_at, last_applied_at,
		       COALESCE(version, 1), COALESCE(parent_id, ''), COALESCE(evolution_chain, '[]')
		FROM strategic_memory
		WHERE embedding IS NOT NULL
		ORDER BY confidence DESC, success_rate DESC
		LIMIT 100
	`

	rows, err := s.db.QueryContext(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("search similar: query failed: %w", err)
	}
	defer rows.Close()

	memories, err := scanStrategicMemories(rows)
	if err != nil {
		return nil, fmt.Errorf("search similar: scan failed: %w", err)
	}

	// Calculate similarity scores using generic ScoredItem
	scored := make([]ScoredItem[StrategicMemory], 0, len(memories))
	for _, mem := range memories {
		if len(mem.Embedding) > 0 {
			similarity := CosineSimilarity(queryEmbedding, mem.Embedding)
			scored = append(scored, ScoredItem[StrategicMemory]{Item: mem, Score: similarity})
		}
	}

	// Use min-heap for efficient top-K selection: O(n log k) vs O(n log n)
	topK := TopKWithScores(scored, limit)

	// Extract memories from scored items
	result := make([]StrategicMemory, len(topK))
	for i, item := range topK {
		result[i] = item.Item
	}

	return result, nil
}

// SearchFTS performs a full-text search on strategic memories.
func (s *StrategicMemoryStore) SearchFTS(ctx context.Context, query string, limit int) ([]StrategicMemory, error) {
	// Convert natural language query to FTS5 search terms
	// Extract significant words and join with OR for broader matching
	ftsQuery := s.buildFTSQuery(query)

	// Use the FTS5 table for full-text search
	sqlQuery := `
		SELECT sm.id, sm.principle, sm.category, sm.trigger_pattern, sm.tier,
		       sm.success_count, sm.failure_count, sm.apply_count,
		       sm.success_rate, sm.confidence, sm.source_sessions, sm.embedding,
		       sm.created_at, sm.updated_at, sm.last_applied_at,
		       COALESCE(sm.version, 1), COALESCE(sm.parent_id, ''), COALESCE(sm.evolution_chain, '[]')
		FROM strategic_memory sm
		JOIN strategic_memory_fts fts ON sm.id = fts.id
		WHERE strategic_memory_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, sqlQuery, ftsQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("search fts: query failed: %w", err)
	}
	defer rows.Close()

	return scanStrategicMemories(rows)
}

// buildFTSQuery converts a natural language query to FTS5 format
func (s *StrategicMemoryStore) buildFTSQuery(query string) string {
	// Common stop words to filter out
	stopWords := map[string]bool{
		"a": true, "an": true, "the": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "must": true, "shall": true, "can": true,
		"to": true, "of": true, "in": true, "for": true, "on": true, "with": true,
		"at": true, "by": true, "from": true, "up": true, "about": true, "into": true,
		"through": true, "during": true, "before": true, "after": true, "above": true,
		"below": true, "between": true, "under": true, "again": true, "further": true,
		"then": true, "once": true, "here": true, "there": true, "when": true,
		"where": true, "why": true, "how": true, "all": true, "each": true,
		"few": true, "more": true, "most": true, "other": true, "some": true,
		"such": true, "no": true, "nor": true, "not": true, "only": true,
		"own": true, "same": true, "so": true, "than": true, "too": true,
		"very": true, "just": true, "and": true, "but": true, "if": true,
		"or": true, "because": true, "as": true, "until": true, "while": true,
		"this": true, "that": true, "these": true, "those": true, "am": true,
		"its": true, "it": true, "i": true, "me": true, "my": true, "you": true,
		"your": true, "he": true, "she": true, "we": true, "they": true,
		"what": true, "which": true, "who": true, "whom": true,
	}

	// Split query into words and filter
	words := strings.Fields(strings.ToLower(query))
	var terms []string
	for _, word := range words {
		// Remove punctuation
		word = strings.Trim(word, ".,?!:;\"'()[]{}")
		// Skip stop words and short words
		if len(word) >= 3 && !stopWords[word] {
			// Add asterisk for prefix matching
			terms = append(terms, word+"*")
		}
	}

	if len(terms) == 0 {
		// Fallback to original query if no terms extracted
		return query
	}

	// Join with OR for broader matching
	return strings.Join(terms, " OR ")
}

// UpdateConfidence updates the confidence score for a strategic memory.
func (s *StrategicMemoryStore) UpdateConfidence(ctx context.Context, id string, confidence float64) error {
	if confidence < 0 || confidence > 1 {
		return fmt.Errorf("update confidence: value must be between 0 and 1, got %f", confidence)
	}

	now := time.Now().Format(time.RFC3339)
	query := `
		UPDATE strategic_memory
		SET confidence = ?,
		    updated_at = ?
		WHERE id = ?
	`

	result, err := s.db.ExecContext(ctx, query, confidence, now, id)
	if err != nil {
		return fmt.Errorf("update confidence: update failed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update confidence: rows affected check: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("update confidence: memory not found: %s", id)
	}

	return nil
}

// Delete removes a strategic memory by ID.
func (s *StrategicMemoryStore) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM strategic_memory WHERE id = ?`

	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete strategic memory: delete failed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete strategic memory: rows affected check: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("delete strategic memory: memory not found: %s", id)
	}

	return nil
}

// rawStrategicRow holds scanned values before processing.
type rawStrategicRow struct {
	id, principle                   string
	category, triggerPattern        sql.NullString
	tier                            sql.NullString
	successCount, failureCount      int
	applyCount                      int
	successRate, confidence         float64
	sourceSessionsJSON              sql.NullString
	embeddingBytes                  []byte
	createdAt, updatedAt            string
	lastAppliedAt                   sql.NullString
	// CR-025-LITE: Evolution fields
	version            int
	parentID           string
	evolutionChainJSON string
}

// toStrategicMemory converts raw scanned values to a StrategicMemory struct.
func (r *rawStrategicRow) toStrategicMemory() *StrategicMemory {
	mem := &StrategicMemory{
		ID:             r.id,
		Principle:      r.principle,
		Category:       r.category.String,
		TriggerPattern: r.triggerPattern.String,
		Tier:           TierTentative, // Default tier
		SuccessCount:   r.successCount,
		FailureCount:   r.failureCount,
		ApplyCount:     r.applyCount,
		SuccessRate:    r.successRate,
		Confidence:     r.confidence,
		// CR-025-LITE: Evolution fields
		Version:  r.version,
		ParentID: r.parentID,
	}

	// Parse tier from database
	if r.tier.Valid && r.tier.String != "" {
		mem.Tier = MemoryTier(r.tier.String)
	}

	// Parse source sessions JSON
	if r.sourceSessionsJSON.Valid && r.sourceSessionsJSON.String != "" {
		if err := json.Unmarshal([]byte(r.sourceSessionsJSON.String), &mem.SourceSessions); err != nil {
			mem.SourceSessions = []string{}
		}
	} else {
		mem.SourceSessions = []string{}
	}

	// CR-025-LITE: Parse evolution chain JSON
	if r.evolutionChainJSON != "" {
		if err := json.Unmarshal([]byte(r.evolutionChainJSON), &mem.EvolutionChain); err != nil {
			mem.EvolutionChain = []string{}
		}
	} else {
		mem.EvolutionChain = []string{}
	}

	// Convert embedding bytes to float32 slice
	mem.Embedding = BytesToFloat32Slice(r.embeddingBytes)

	// Parse timestamps
	if t, err := time.Parse(time.RFC3339, r.createdAt); err == nil {
		mem.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339, r.updatedAt); err == nil {
		mem.UpdatedAt = t
	}
	if r.lastAppliedAt.Valid && r.lastAppliedAt.String != "" {
		if t, err := time.Parse(time.RFC3339, r.lastAppliedAt.String); err == nil {
			mem.LastAppliedAt = &t
		}
	}

	// Default version if not set
	if mem.Version == 0 {
		mem.Version = 1
	}

	return mem
}

// scanStrategicMemory scans a single row into a StrategicMemory struct.
func scanStrategicMemory(row *sql.Row) (*StrategicMemory, error) {
	var r rawStrategicRow

	err := row.Scan(
		&r.id, &r.principle, &r.category, &r.triggerPattern, &r.tier,
		&r.successCount, &r.failureCount, &r.applyCount,
		&r.successRate, &r.confidence, &r.sourceSessionsJSON,
		&r.embeddingBytes, &r.createdAt, &r.updatedAt, &r.lastAppliedAt,
		&r.version, &r.parentID, &r.evolutionChainJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("strategic memory not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scan strategic memory: %w", err)
	}

	return r.toStrategicMemory(), nil
}

// scanStrategicMemories scans multiple rows into a slice of StrategicMemory.
func scanStrategicMemories(rows *sql.Rows) ([]StrategicMemory, error) {
	var memories []StrategicMemory

	for rows.Next() {
		var r rawStrategicRow

		err := rows.Scan(
			&r.id, &r.principle, &r.category, &r.triggerPattern, &r.tier,
			&r.successCount, &r.failureCount, &r.applyCount,
			&r.successRate, &r.confidence, &r.sourceSessionsJSON,
			&r.embeddingBytes, &r.createdAt, &r.updatedAt, &r.lastAppliedAt,
			&r.version, &r.parentID, &r.evolutionChainJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("scan strategic memories: %w", err)
		}

		memories = append(memories, *r.toStrategicMemory())
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan strategic memories: rows error: %w", err)
	}

	return memories, nil
}

// ============================================================================
// TIER PROMOTION METHODS
// ============================================================================

// CalculateEligibleTier determines what tier a memory should be promoted to
// based on its current stats and the provided thresholds.
func (s *StrategicMemoryStore) CalculateEligibleTier(mem *StrategicMemory, thresholds TierPromotionThresholds) MemoryTier {
	// Check for Identity tier eligibility (highest tier)
	if mem.ApplyCount >= thresholds.MinApplyCountForIdentity &&
		mem.SuccessRate >= thresholds.MinSuccessRateForIdentity &&
		len(mem.SourceSessions) >= thresholds.MinUniqueSessionsForIdentity &&
		time.Since(mem.CreatedAt) >= thresholds.MinAgeForIdentity {
		return TierIdentity
	}

	// Check for Proven tier eligibility
	if mem.ApplyCount >= thresholds.MinApplyCountForProven &&
		mem.SuccessRate >= thresholds.MinSuccessRateForProven {
		return TierProven
	}

	// Check for Candidate tier eligibility
	if mem.ApplyCount >= thresholds.MinApplyCountForCandidate {
		return TierCandidate
	}

	// Default to Tentative
	return TierTentative
}

// PromoteIfEligible checks and promotes a memory if it meets threshold criteria.
// Returns whether the memory was promoted, the new tier, and any error.
func (s *StrategicMemoryStore) PromoteIfEligible(ctx context.Context, id string, thresholds TierPromotionThresholds) (promoted bool, newTier MemoryTier, err error) {
	// Get current memory state
	mem, err := s.Get(ctx, id)
	if err != nil {
		return false, "", fmt.Errorf("promote if eligible: %w", err)
	}

	// Calculate eligible tier
	eligibleTier := s.CalculateEligibleTier(mem, thresholds)

	// Define tier order for comparison
	tierOrder := map[MemoryTier]int{
		TierTentative: 0,
		TierCandidate: 1,
		TierProven:    2,
		TierIdentity:  3,
	}

	currentOrder := tierOrder[mem.Tier]
	eligibleOrder := tierOrder[eligibleTier]

	// Only promote if eligible tier is higher than current
	if eligibleOrder <= currentOrder {
		return false, mem.Tier, nil
	}

	// Perform promotion
	err = s.UpdateTier(ctx, id, eligibleTier)
	if err != nil {
		return false, "", fmt.Errorf("promote if eligible: update tier failed: %w", err)
	}

	return true, eligibleTier, nil
}

// GetByTier returns strategic memories filtered by tier.
func (s *StrategicMemoryStore) GetByTier(ctx context.Context, tier MemoryTier, limit int) ([]StrategicMemory, error) {
	query := `
		SELECT id, principle, category, trigger_pattern, tier,
		       success_count, failure_count, apply_count,
		       success_rate, confidence, source_sessions, embedding,
		       created_at, updated_at, last_applied_at,
		       COALESCE(version, 1), COALESCE(parent_id, ''), COALESCE(evolution_chain, '[]')
		FROM strategic_memory
		WHERE tier = ?
		ORDER BY success_rate DESC, confidence DESC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, string(tier), limit)
	if err != nil {
		return nil, fmt.Errorf("get by tier: query failed: %w", err)
	}
	defer rows.Close()

	return scanStrategicMemories(rows)
}

// List returns all strategic memories up to a limit.
func (s *StrategicMemoryStore) List(ctx context.Context, limit int) ([]StrategicMemory, error) {
	query := `
		SELECT id, principle, category, trigger_pattern, tier,
		       success_count, failure_count, apply_count,
		       success_rate, confidence, source_sessions, embedding,
		       created_at, updated_at, last_applied_at,
		       COALESCE(version, 1), COALESCE(parent_id, ''), COALESCE(evolution_chain, '[]')
		FROM strategic_memory
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list: query failed: %w", err)
	}
	defer rows.Close()

	return scanStrategicMemories(rows)
}

// UpdateTier updates the tier for a strategic memory.
func (s *StrategicMemoryStore) UpdateTier(ctx context.Context, id string, tier MemoryTier) error {
	// Validate tier
	switch tier {
	case TierTentative, TierCandidate, TierProven, TierIdentity:
		// Valid tier
	default:
		return fmt.Errorf("update tier: invalid tier value: %s", tier)
	}

	now := time.Now().Format(time.RFC3339)
	query := `
		UPDATE strategic_memory
		SET tier = ?,
		    updated_at = ?
		WHERE id = ?
	`

	result, err := s.db.ExecContext(ctx, query, string(tier), now, id)
	if err != nil {
		return fmt.Errorf("update tier: update failed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update tier: rows affected check: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("update tier: memory not found: %s", id)
	}

	return nil
}

// ============================================================================
// CR-025-LITE: EVOLUTION METHODS
// ============================================================================

// CreateEvolution creates a new version of a memory, preserving the lineage.
// The new memory starts at TierTentative with slightly reduced confidence.
func (s *StrategicMemoryStore) CreateEvolution(ctx context.Context, parentID string, newPrinciple string, reason string) (*StrategicMemory, error) {
	parent, err := s.Get(ctx, parentID)
	if err != nil {
		return nil, fmt.Errorf("create evolution: parent not found: %w", err)
	}

	// Build evolution chain: append parent to its chain
	chain := make([]string, len(parent.EvolutionChain), len(parent.EvolutionChain)+1)
	copy(chain, parent.EvolutionChain)
	chain = append(chain, parentID)

	child := &StrategicMemory{
		Principle:      newPrinciple,
		Category:       parent.Category,
		TriggerPattern: parent.TriggerPattern,
		Tier:           TierTentative, // New versions start tentative
		Version:        parent.Version + 1,
		ParentID:       parentID,
		EvolutionChain: chain,
		SourceSessions: parent.SourceSessions,
		Confidence:     parent.Confidence * 0.9, // Slightly lower confidence for evolved version
	}

	if err := s.Create(ctx, child); err != nil {
		return nil, fmt.Errorf("create evolution: %w", err)
	}

	return child, nil
}

// GetEvolutionHistory retrieves the full evolution chain for a memory.
// Returns ancestors from oldest to newest, including the current memory.
func (s *StrategicMemoryStore) GetEvolutionHistory(ctx context.Context, id string) ([]StrategicMemory, error) {
	mem, err := s.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get evolution history: %w", err)
	}

	var history []StrategicMemory
	for _, ancestorID := range mem.EvolutionChain {
		ancestor, err := s.Get(ctx, ancestorID)
		if err == nil {
			history = append(history, *ancestor)
		}
		// Skip ancestors that no longer exist (deleted)
	}
	history = append(history, *mem) // Include current

	return history, nil
}

// GetDescendants finds all memories that evolved from this one.
func (s *StrategicMemoryStore) GetDescendants(ctx context.Context, id string) ([]StrategicMemory, error) {
	query := `
		SELECT id, principle, category, trigger_pattern, tier,
		       success_count, failure_count, apply_count,
		       success_rate, confidence, source_sessions, embedding,
		       created_at, updated_at, last_applied_at,
		       COALESCE(version, 1), COALESCE(parent_id, ''), COALESCE(evolution_chain, '[]')
		FROM strategic_memory
		WHERE parent_id = ?
		ORDER BY version DESC
	`

	rows, err := s.db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("get descendants: query failed: %w", err)
	}
	defer rows.Close()

	return scanStrategicMemories(rows)
}
