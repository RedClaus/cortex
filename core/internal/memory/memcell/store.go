package memcell

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ══════════════════════════════════════════════════════════════════════════════
// MEMCELL STORE IMPLEMENTATION
// ══════════════════════════════════════════════════════════════════════════════

// SQLiteStore implements Store using SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLite-backed MemCell store.
func NewSQLiteStore(db *sql.DB) *SQLiteStore {
	return &SQLiteStore{db: db}
}

// ══════════════════════════════════════════════════════════════════════════════
// CRUD OPERATIONS
// ══════════════════════════════════════════════════════════════════════════════

// Create stores a new MemCell.
func (s *SQLiteStore) Create(ctx context.Context, cell *MemCell) error {
	if cell.ID == "" {
		cell.ID = uuid.New().String()
	}
	if cell.CreatedAt.IsZero() {
		cell.CreatedAt = time.Now()
	}
	cell.UpdatedAt = time.Now()
	if cell.Version == 0 {
		cell.Version = 1
	}

	// Marshal JSON fields
	entitiesJSON, _ := json.Marshal(cell.Entities)
	keyPhrasesJSON, _ := json.Marshal(cell.KeyPhrases)
	topicsJSON, _ := json.Marshal(cell.Topics)

	query := `
		INSERT INTO memcells (
			id, source_id, version, created_at, updated_at, last_access_at, access_count,
			raw_content, summary, entities, key_phrases, sentiment,
			memory_type, confidence, importance, topics, scope,
			parent_id, supersedes_id, episode_id,
			event_boundary, preceding_ctx, following_ctx, conversation_id, turn_number, user_state
		) VALUES (
			?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?,
			?, ?, ?, ?, ?, ?
		)
	`

	_, err := s.db.ExecContext(ctx, query,
		cell.ID, cell.SourceID, cell.Version, cell.CreatedAt.Format(time.RFC3339), cell.UpdatedAt.Format(time.RFC3339),
		nullTimeString(cell.LastAccessAt), cell.AccessCount,
		cell.RawContent, cell.Summary, string(entitiesJSON), string(keyPhrasesJSON), cell.Sentiment,
		string(cell.MemoryType), cell.Confidence, cell.Importance, string(topicsJSON), string(cell.Scope),
		nullString(cell.ParentID), nullString(cell.SupersedesID), nullString(cell.EpisodeID),
		boolToInt(cell.EventBoundary), cell.PrecedingCtx, cell.FollowingCtx,
		nullString(cell.ConversationID), cell.TurnNumber, cell.UserState,
	)
	if err != nil {
		return fmt.Errorf("create memcell: %w", err)
	}

	// Store relations if any
	if err := s.storeRelations(ctx, cell); err != nil {
		return fmt.Errorf("store relations: %w", err)
	}

	return nil
}

// Get retrieves a MemCell by ID.
func (s *SQLiteStore) Get(ctx context.Context, id string) (*MemCell, error) {
	query := `
		SELECT
			id, source_id, version, created_at, updated_at, last_access_at, access_count,
			raw_content, summary, entities, key_phrases, sentiment,
			memory_type, confidence, importance, topics, scope,
			parent_id, supersedes_id, episode_id,
			event_boundary, preceding_ctx, following_ctx, conversation_id, turn_number, user_state
		FROM memcells
		WHERE id = ?
	`

	cell := &MemCell{}
	var (
		createdAt, updatedAt, lastAccessAt sql.NullString
		entities, keyPhrases, topics       string
		parentID, supersedesID, episodeID  sql.NullString
		conversationID                     sql.NullString
		eventBoundary                      int
	)

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&cell.ID, &cell.SourceID, &cell.Version, &createdAt, &updatedAt, &lastAccessAt, &cell.AccessCount,
		&cell.RawContent, &cell.Summary, &entities, &keyPhrases, &cell.Sentiment,
		&cell.MemoryType, &cell.Confidence, &cell.Importance, &topics, &cell.Scope,
		&parentID, &supersedesID, &episodeID,
		&eventBoundary, &cell.PrecedingCtx, &cell.FollowingCtx, &conversationID, &cell.TurnNumber, &cell.UserState,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get memcell: %w", err)
	}

	// Parse timestamps
	cell.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
	cell.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt.String)
	if lastAccessAt.Valid {
		cell.LastAccessAt, _ = time.Parse(time.RFC3339, lastAccessAt.String)
	}

	// Parse JSON arrays
	json.Unmarshal([]byte(entities), &cell.Entities)
	json.Unmarshal([]byte(keyPhrases), &cell.KeyPhrases)
	json.Unmarshal([]byte(topics), &cell.Topics)

	// Parse nullable strings
	cell.ParentID = parentID.String
	cell.SupersedesID = supersedesID.String
	cell.EpisodeID = episodeID.String
	cell.ConversationID = conversationID.String
	cell.EventBoundary = eventBoundary == 1

	// Load relations
	if err := s.loadRelations(ctx, cell); err != nil {
		return nil, fmt.Errorf("load relations: %w", err)
	}

	return cell, nil
}

// Update modifies an existing MemCell.
func (s *SQLiteStore) Update(ctx context.Context, cell *MemCell) error {
	cell.UpdatedAt = time.Now()
	cell.Version++

	entitiesJSON, _ := json.Marshal(cell.Entities)
	keyPhrasesJSON, _ := json.Marshal(cell.KeyPhrases)
	topicsJSON, _ := json.Marshal(cell.Topics)

	query := `
		UPDATE memcells SET
			source_id = ?, version = ?, updated_at = ?,
			raw_content = ?, summary = ?, entities = ?, key_phrases = ?, sentiment = ?,
			memory_type = ?, confidence = ?, importance = ?, topics = ?, scope = ?,
			parent_id = ?, supersedes_id = ?, episode_id = ?,
			event_boundary = ?, preceding_ctx = ?, following_ctx = ?,
			conversation_id = ?, turn_number = ?, user_state = ?
		WHERE id = ?
	`

	_, err := s.db.ExecContext(ctx, query,
		cell.SourceID, cell.Version, cell.UpdatedAt.Format(time.RFC3339),
		cell.RawContent, cell.Summary, string(entitiesJSON), string(keyPhrasesJSON), cell.Sentiment,
		string(cell.MemoryType), cell.Confidence, cell.Importance, string(topicsJSON), string(cell.Scope),
		nullString(cell.ParentID), nullString(cell.SupersedesID), nullString(cell.EpisodeID),
		boolToInt(cell.EventBoundary), cell.PrecedingCtx, cell.FollowingCtx,
		nullString(cell.ConversationID), cell.TurnNumber, cell.UserState,
		cell.ID,
	)
	if err != nil {
		return fmt.Errorf("update memcell: %w", err)
	}

	// Update relations
	if err := s.storeRelations(ctx, cell); err != nil {
		return fmt.Errorf("update relations: %w", err)
	}

	return nil
}

// Delete removes a MemCell.
func (s *SQLiteStore) Delete(ctx context.Context, id string) error {
	// Relations are deleted via CASCADE
	_, err := s.db.ExecContext(ctx, "DELETE FROM memcells WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete memcell: %w", err)
	}
	return nil
}

// ══════════════════════════════════════════════════════════════════════════════
// SEARCH OPERATIONS
// ══════════════════════════════════════════════════════════════════════════════

// Search finds MemCells by query using FTS5.
func (s *SQLiteStore) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	if opts.TopK == 0 {
		opts.TopK = 10
	}

	// Build WHERE clauses
	var conditions []string
	var args []interface{}

	// FTS match
	conditions = append(conditions, "m.rowid IN (SELECT rowid FROM memcells_fts WHERE memcells_fts MATCH ?)")
	args = append(args, query)

	// Type filter
	if len(opts.MemoryTypes) > 0 {
		placeholders := make([]string, len(opts.MemoryTypes))
		for i, t := range opts.MemoryTypes {
			placeholders[i] = "?"
			args = append(args, string(t))
		}
		conditions = append(conditions, fmt.Sprintf("m.memory_type IN (%s)", strings.Join(placeholders, ",")))
	}

	// Scope filter
	if opts.Scope != "" {
		conditions = append(conditions, "m.scope = ?")
		args = append(args, string(opts.Scope))
	}

	// Episode filter
	if opts.EpisodeID != "" {
		conditions = append(conditions, "m.episode_id = ?")
		args = append(args, opts.EpisodeID)
	}

	// Conversation filter
	if opts.ConversationID != "" {
		conditions = append(conditions, "m.conversation_id = ?")
		args = append(args, opts.ConversationID)
	}

	// Importance threshold
	if opts.MinImportance > 0 {
		conditions = append(conditions, "m.importance >= ?")
		args = append(args, opts.MinImportance)
	}

	// Confidence threshold
	if opts.MinConfidence > 0 {
		conditions = append(conditions, "m.confidence >= ?")
		args = append(args, opts.MinConfidence)
	}

	// Time filter
	if !opts.SinceTime.IsZero() {
		conditions = append(conditions, "m.created_at >= ?")
		args = append(args, opts.SinceTime.Format(time.RFC3339))
	}

	whereClause := strings.Join(conditions, " AND ")
	args = append(args, opts.TopK)

	searchQuery := fmt.Sprintf(`
		SELECT
			m.id, m.source_id, m.version, m.created_at, m.updated_at, m.last_access_at, m.access_count,
			m.raw_content, m.summary, m.entities, m.key_phrases, m.sentiment,
			m.memory_type, m.confidence, m.importance, m.topics, m.scope,
			m.parent_id, m.supersedes_id, m.episode_id,
			m.event_boundary, m.preceding_ctx, m.following_ctx, m.conversation_id, m.turn_number, m.user_state
		FROM memcells m
		WHERE %s
		ORDER BY m.importance DESC, m.created_at DESC
		LIMIT ?
	`, whereClause)

	rows, err := s.db.QueryContext(ctx, searchQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("search memcells: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		cell := &MemCell{}
		var (
			createdAt, updatedAt, lastAccessAt sql.NullString
			entities, keyPhrases, topics       string
			parentID, supersedesID, episodeID  sql.NullString
			conversationID                     sql.NullString
			eventBoundary                      int
		)

		err := rows.Scan(
			&cell.ID, &cell.SourceID, &cell.Version, &createdAt, &updatedAt, &lastAccessAt, &cell.AccessCount,
			&cell.RawContent, &cell.Summary, &entities, &keyPhrases, &cell.Sentiment,
			&cell.MemoryType, &cell.Confidence, &cell.Importance, &topics, &cell.Scope,
			&parentID, &supersedesID, &episodeID,
			&eventBoundary, &cell.PrecedingCtx, &cell.FollowingCtx, &conversationID, &cell.TurnNumber, &cell.UserState,
		)
		if err != nil {
			continue
		}

		// Parse fields
		cell.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
		cell.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt.String)
		if lastAccessAt.Valid {
			cell.LastAccessAt, _ = time.Parse(time.RFC3339, lastAccessAt.String)
		}
		json.Unmarshal([]byte(entities), &cell.Entities)
		json.Unmarshal([]byte(keyPhrases), &cell.KeyPhrases)
		json.Unmarshal([]byte(topics), &cell.Topics)
		cell.ParentID = parentID.String
		cell.SupersedesID = supersedesID.String
		cell.EpisodeID = episodeID.String
		cell.ConversationID = conversationID.String
		cell.EventBoundary = eventBoundary == 1

		results = append(results, SearchResult{
			Cell:      cell,
			Score:     cell.Importance, // FTS doesn't give score, use importance
			MatchType: "keyword",
		})
	}

	// Optionally expand with relations
	if opts.IncludeRelated && len(results) > 0 {
		for i := range results {
			related, _ := s.GetRelated(ctx, results[i].Cell.ID, opts.RelationDepth)
			for _, r := range related {
				// Avoid duplicates
				exists := false
				for _, res := range results {
					if res.Cell.ID == r.ID {
						exists = true
						break
					}
				}
				if !exists {
					results = append(results, SearchResult{
						Cell:      &r,
						Score:     r.Importance * 0.8, // Discount related
						MatchType: "related",
					})
				}
			}
		}
	}

	return results, nil
}

// GetRelated retrieves related MemCells to a given depth.
func (s *SQLiteStore) GetRelated(ctx context.Context, id string, depth int) ([]MemCell, error) {
	if depth <= 0 {
		depth = 1
	}

	visited := make(map[string]bool)
	visited[id] = true

	var results []MemCell
	toVisit := []string{id}

	for d := 0; d < depth && len(toVisit) > 0; d++ {
		var nextToVisit []string

		for _, cellID := range toVisit {
			// Get relations FROM this cell
			rows, err := s.db.QueryContext(ctx, `
				SELECT to_id FROM memcell_relations WHERE from_id = ?
			`, cellID)
			if err != nil {
				continue
			}

			for rows.Next() {
				var toID string
				if err := rows.Scan(&toID); err != nil {
					continue
				}
				if !visited[toID] {
					visited[toID] = true
					nextToVisit = append(nextToVisit, toID)

					cell, err := s.Get(ctx, toID)
					if err == nil && cell != nil {
						results = append(results, *cell)
					}
				}
			}
			rows.Close()

			// Get relations TO this cell
			rows, err = s.db.QueryContext(ctx, `
				SELECT from_id FROM memcell_relations WHERE to_id = ?
			`, cellID)
			if err != nil {
				continue
			}

			for rows.Next() {
				var fromID string
				if err := rows.Scan(&fromID); err != nil {
					continue
				}
				if !visited[fromID] {
					visited[fromID] = true
					nextToVisit = append(nextToVisit, fromID)

					cell, err := s.Get(ctx, fromID)
					if err == nil && cell != nil {
						results = append(results, *cell)
					}
				}
			}
			rows.Close()
		}

		toVisit = nextToVisit
	}

	return results, nil
}

// AddRelation creates a link between two MemCells.
func (s *SQLiteStore) AddRelation(ctx context.Context, from, to string, relType RelationType, strength float64) error {
	query := `
		INSERT OR REPLACE INTO memcell_relations (from_id, to_id, relation_type, strength, created_at)
		VALUES (?, ?, ?, ?, datetime('now'))
	`
	_, err := s.db.ExecContext(ctx, query, from, to, string(relType), strength)
	if err != nil {
		return fmt.Errorf("add relation: %w", err)
	}
	return nil
}

// GetByEpisode retrieves all MemCells in an episode.
func (s *SQLiteStore) GetByEpisode(ctx context.Context, episodeID string) ([]MemCell, error) {
	query := `
		SELECT
			id, source_id, version, created_at, updated_at, last_access_at, access_count,
			raw_content, summary, entities, key_phrases, sentiment,
			memory_type, confidence, importance, topics, scope,
			parent_id, supersedes_id, episode_id,
			event_boundary, preceding_ctx, following_ctx, conversation_id, turn_number, user_state
		FROM memcells
		WHERE episode_id = ?
		ORDER BY turn_number ASC, created_at ASC
	`

	rows, err := s.db.QueryContext(ctx, query, episodeID)
	if err != nil {
		return nil, fmt.Errorf("get by episode: %w", err)
	}
	defer rows.Close()

	return s.scanCells(rows)
}

// GetByType retrieves MemCells of a specific type.
func (s *SQLiteStore) GetByType(ctx context.Context, memType MemoryType, opts SearchOptions) ([]MemCell, error) {
	if opts.TopK == 0 {
		opts.TopK = 100
	}

	query := `
		SELECT
			id, source_id, version, created_at, updated_at, last_access_at, access_count,
			raw_content, summary, entities, key_phrases, sentiment,
			memory_type, confidence, importance, topics, scope,
			parent_id, supersedes_id, episode_id,
			event_boundary, preceding_ctx, following_ctx, conversation_id, turn_number, user_state
		FROM memcells
		WHERE memory_type = ?
		AND importance >= ?
		AND confidence >= ?
		ORDER BY importance DESC, created_at DESC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, string(memType), opts.MinImportance, opts.MinConfidence, opts.TopK)
	if err != nil {
		return nil, fmt.Errorf("get by type: %w", err)
	}
	defer rows.Close()

	return s.scanCells(rows)
}

// RecordAccess updates access statistics.
func (s *SQLiteStore) RecordAccess(ctx context.Context, id string) error {
	query := `
		UPDATE memcells SET
			last_access_at = datetime('now'),
			access_count = access_count + 1
		WHERE id = ?
	`
	_, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("record access: %w", err)
	}
	return nil
}

// ══════════════════════════════════════════════════════════════════════════════
// HELPER METHODS
// ══════════════════════════════════════════════════════════════════════════════

func (s *SQLiteStore) storeRelations(ctx context.Context, cell *MemCell) error {
	// Clear existing relations from this cell
	_, err := s.db.ExecContext(ctx, "DELETE FROM memcell_relations WHERE from_id = ?", cell.ID)
	if err != nil {
		return err
	}

	// Store child relations
	for _, childID := range cell.ChildIDs {
		if err := s.AddRelation(ctx, cell.ID, childID, RelTypeChild, 1.0); err != nil {
			return err
		}
	}

	// Store related relations
	for _, relID := range cell.RelatedIDs {
		strength := 0.5
		if cell.LinkStrengths != nil {
			if s, ok := cell.LinkStrengths[relID]; ok {
				strength = s
			}
		}
		if err := s.AddRelation(ctx, cell.ID, relID, RelTypeRelated, strength); err != nil {
			return err
		}
	}

	// Store contradicts relations
	for _, cID := range cell.ContradictsIDs {
		if err := s.AddRelation(ctx, cell.ID, cID, RelTypeContradicts, 1.0); err != nil {
			return err
		}
	}

	// Store supports relations
	for _, sID := range cell.SupportsIDs {
		if err := s.AddRelation(ctx, cell.ID, sID, RelTypeSupports, 1.0); err != nil {
			return err
		}
	}

	return nil
}

func (s *SQLiteStore) loadRelations(ctx context.Context, cell *MemCell) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT to_id, relation_type, strength
		FROM memcell_relations
		WHERE from_id = ?
	`, cell.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	cell.LinkStrengths = make(map[string]float64)

	for rows.Next() {
		var toID, relType string
		var strength float64
		if err := rows.Scan(&toID, &relType, &strength); err != nil {
			continue
		}

		cell.LinkStrengths[toID] = strength

		switch RelationType(relType) {
		case RelTypeChild:
			cell.ChildIDs = append(cell.ChildIDs, toID)
		case RelTypeRelated:
			cell.RelatedIDs = append(cell.RelatedIDs, toID)
		case RelTypeContradicts:
			cell.ContradictsIDs = append(cell.ContradictsIDs, toID)
		case RelTypeSupports:
			cell.SupportsIDs = append(cell.SupportsIDs, toID)
		}
	}

	return nil
}

func (s *SQLiteStore) scanCells(rows *sql.Rows) ([]MemCell, error) {
	var cells []MemCell

	for rows.Next() {
		cell := MemCell{}
		var (
			createdAt, updatedAt, lastAccessAt sql.NullString
			entities, keyPhrases, topics       string
			parentID, supersedesID, episodeID  sql.NullString
			conversationID                     sql.NullString
			eventBoundary                      int
		)

		err := rows.Scan(
			&cell.ID, &cell.SourceID, &cell.Version, &createdAt, &updatedAt, &lastAccessAt, &cell.AccessCount,
			&cell.RawContent, &cell.Summary, &entities, &keyPhrases, &cell.Sentiment,
			&cell.MemoryType, &cell.Confidence, &cell.Importance, &topics, &cell.Scope,
			&parentID, &supersedesID, &episodeID,
			&eventBoundary, &cell.PrecedingCtx, &cell.FollowingCtx, &conversationID, &cell.TurnNumber, &cell.UserState,
		)
		if err != nil {
			continue
		}

		cell.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
		cell.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt.String)
		if lastAccessAt.Valid {
			cell.LastAccessAt, _ = time.Parse(time.RFC3339, lastAccessAt.String)
		}
		json.Unmarshal([]byte(entities), &cell.Entities)
		json.Unmarshal([]byte(keyPhrases), &cell.KeyPhrases)
		json.Unmarshal([]byte(topics), &cell.Topics)
		cell.ParentID = parentID.String
		cell.SupersedesID = supersedesID.String
		cell.EpisodeID = episodeID.String
		cell.ConversationID = conversationID.String
		cell.EventBoundary = eventBoundary == 1

		cells = append(cells, cell)
	}

	return cells, nil
}

// ══════════════════════════════════════════════════════════════════════════════
// UTILITY FUNCTIONS
// ══════════════════════════════════════════════════════════════════════════════

func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func nullTimeString(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t.Format(time.RFC3339)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
