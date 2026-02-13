// Package data provides the unified data access layer for all Cortex operations.
package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/normanking/cortex/pkg/types"
)

// ═══════════════════════════════════════════════════════════════════════════════
// KNOWLEDGE OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════════

// CreateKnowledge inserts a new knowledge item into the database.
// The ID must be unique. Use ULID or UUID for generation.
func (s *Store) CreateKnowledge(ctx context.Context, item *types.KnowledgeItem) error {
	if item.ID == "" {
		return fmt.Errorf("knowledge item ID cannot be empty")
	}

	// Set default sync status if not provided
	syncStatus := item.SyncStatus
	if syncStatus == "" {
		syncStatus = "pending"
	}

	// Marshal tags to JSON
	tagsJSON, err := json.Marshal(item.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}

	// Handle nullable time fields
	var lastAccessedAt, deletedAt, lastSyncedAt interface{}
	if item.LastAccessedAt != nil {
		lastAccessedAt = item.LastAccessedAt
	}
	if item.DeletedAt != nil {
		deletedAt = item.DeletedAt
	}
	if !item.LastSyncedAt.IsZero() {
		lastSyncedAt = item.LastSyncedAt
	}

	query := `
		INSERT INTO knowledge_items (
			id, type, content, title, tags,
			scope, team_id, author_id, author_name,
			confidence, trust_score, success_count, failure_count,
			access_count, version, remote_id, sync_status,
			last_synced_at, created_at, updated_at,
			last_accessed_at, deleted_at
		) VALUES (
			?, ?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?,
			?, ?
		)
	`

	_, err = s.db.ExecContext(ctx, query,
		item.ID, item.Type, item.Content, nullString(item.Title), string(tagsJSON),
		item.Scope, nullString(item.TeamID), item.AuthorID, nullString(item.AuthorName),
		item.Confidence, item.TrustScore, item.SuccessCount, item.FailureCount,
		item.AccessCount, item.Version, nullString(item.RemoteID), syncStatus,
		lastSyncedAt, time.Now(), time.Now(),
		lastAccessedAt, deletedAt,
	)

	if err != nil {
		return fmt.Errorf("insert knowledge item: %w", err)
	}

	return nil
}

// GetKnowledge retrieves a knowledge item by ID.
// Returns sql.ErrNoRows if not found.
func (s *Store) GetKnowledge(ctx context.Context, id string) (*types.KnowledgeItem, error) {
	query := `
		SELECT
			id, type, content, title, tags,
			scope, team_id, author_id, author_name,
			confidence, trust_score, success_count, failure_count,
			access_count, version, remote_id, sync_status,
			last_synced_at, created_at, updated_at,
			last_accessed_at, deleted_at
		FROM knowledge_items
		WHERE id = ? AND deleted_at IS NULL
	`

	var item types.KnowledgeItem
	var title, teamID, authorName, remoteID sql.NullString
	var tagsJSON string
	var lastAccessedAt, deletedAt, lastSyncedAt sql.NullTime

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&item.ID, &item.Type, &item.Content, &title, &tagsJSON,
		&item.Scope, &teamID, &item.AuthorID, &authorName,
		&item.Confidence, &item.TrustScore, &item.SuccessCount, &item.FailureCount,
		&item.AccessCount, &item.Version, &remoteID, &item.SyncStatus,
		&lastSyncedAt, &item.CreatedAt, &item.UpdatedAt,
		&lastAccessedAt, &deletedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("knowledge item not found: %s", id)
		}
		return nil, fmt.Errorf("query knowledge item: %w", err)
	}

	// Unmarshal tags
	if err := json.Unmarshal([]byte(tagsJSON), &item.Tags); err != nil {
		return nil, fmt.Errorf("unmarshal tags: %w", err)
	}

	// Set nullable fields
	if title.Valid {
		item.Title = title.String
	}
	if teamID.Valid {
		item.TeamID = teamID.String
	}
	if authorName.Valid {
		item.AuthorName = authorName.String
	}
	if remoteID.Valid {
		item.RemoteID = remoteID.String
	}
	if lastAccessedAt.Valid {
		item.LastAccessedAt = &lastAccessedAt.Time
	}
	if deletedAt.Valid {
		item.DeletedAt = &deletedAt.Time
	}
	if lastSyncedAt.Valid {
		item.LastSyncedAt = lastSyncedAt.Time
	}

	// Update last accessed timestamp
	go s.updateLastAccessed(id)

	return &item, nil
}

// updateLastAccessed updates the last_accessed_at timestamp for a knowledge item.
// This runs asynchronously to avoid blocking reads.
func (s *Store) updateLastAccessed(id string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `UPDATE knowledge_items SET last_accessed_at = ? WHERE id = ?`
	s.db.ExecContext(ctx, query, time.Now(), id)
}

// UpdateKnowledge updates an existing knowledge item.
// The ID field is used to identify the item.
func (s *Store) UpdateKnowledge(ctx context.Context, item *types.KnowledgeItem) error {
	if item.ID == "" {
		return fmt.Errorf("knowledge item ID cannot be empty")
	}

	// Set default sync status if not provided
	syncStatus := item.SyncStatus
	if syncStatus == "" {
		syncStatus = "pending"
	}

	// Marshal tags to JSON
	tagsJSON, err := json.Marshal(item.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}

	query := `
		UPDATE knowledge_items SET
			type = ?,
			content = ?,
			title = ?,
			tags = ?,
			scope = ?,
			team_id = ?,
			author_id = ?,
			author_name = ?,
			confidence = ?,
			trust_score = ?,
			success_count = ?,
			failure_count = ?,
			access_count = ?,
			remote_id = ?,
			sync_status = ?,
			updated_at = ?
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := s.db.ExecContext(ctx, query,
		item.Type, item.Content, nullString(item.Title), string(tagsJSON),
		item.Scope, nullString(item.TeamID), item.AuthorID, nullString(item.AuthorName),
		item.Confidence, item.TrustScore, item.SuccessCount, item.FailureCount,
		item.AccessCount, nullString(item.RemoteID), syncStatus,
		time.Now(), item.ID,
	)

	if err != nil {
		return fmt.Errorf("update knowledge item: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("knowledge item not found: %s", item.ID)
	}

	return nil
}

// DeleteKnowledge soft-deletes a knowledge item by setting deleted_at.
// This allows the item to be synced as deleted before permanent removal.
func (s *Store) DeleteKnowledge(ctx context.Context, id string) error {
	query := `
		UPDATE knowledge_items
		SET deleted_at = ?, sync_status = 'pending'
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := s.db.ExecContext(ctx, query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("delete knowledge item: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("knowledge item not found or already deleted: %s", id)
	}

	return nil
}

// ListKnowledge retrieves knowledge items based on search options.
// Returns an empty slice if no items match the criteria.
func (s *Store) ListKnowledge(ctx context.Context, opts types.SearchOptions) ([]*types.KnowledgeItem, error) {
	// Build dynamic query based on options
	query := `
		SELECT
			id, type, content, title, tags,
			scope, team_id, author_id, author_name,
			confidence, trust_score, success_count, failure_count,
			access_count, version, remote_id, sync_status,
			last_synced_at, created_at, updated_at,
			last_accessed_at, deleted_at
		FROM knowledge_items
		WHERE deleted_at IS NULL
	`

	var args []interface{}

	// Filter by scope/tiers
	if len(opts.Tiers) > 0 {
		query += " AND scope IN ("
		for i, tier := range opts.Tiers {
			if i > 0 {
				query += ","
			}
			query += "?"
			args = append(args, tier)
		}
		query += ")"
	}

	// Filter by type
	if len(opts.Types) > 0 {
		query += " AND type IN ("
		for i, t := range opts.Types {
			if i > 0 {
				query += ","
			}
			query += "?"
			args = append(args, t)
		}
		query += ")"
	}

	// Filter by minimum trust score
	if opts.MinTrust > 0 {
		query += " AND trust_score >= ?"
		args = append(args, opts.MinTrust)
	}

	// Order by trust score and confidence
	query += " ORDER BY trust_score DESC, confidence DESC, updated_at DESC"

	// Apply limit
	limit := opts.Limit
	if limit <= 0 {
		limit = 10 // Default limit
	}
	query += " LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query knowledge items: %w", err)
	}
	defer rows.Close()

	var items []*types.KnowledgeItem
	for rows.Next() {
		var item types.KnowledgeItem
		var title, teamID, authorName, remoteID sql.NullString
		var tagsJSON string
		var lastAccessedAt, deletedAt, lastSyncedAt sql.NullTime

		err := rows.Scan(
			&item.ID, &item.Type, &item.Content, &title, &tagsJSON,
			&item.Scope, &teamID, &item.AuthorID, &authorName,
			&item.Confidence, &item.TrustScore, &item.SuccessCount, &item.FailureCount,
			&item.AccessCount, &item.Version, &remoteID, &item.SyncStatus,
			&lastSyncedAt, &item.CreatedAt, &item.UpdatedAt,
			&lastAccessedAt, &deletedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("scan knowledge item: %w", err)
		}

		// Unmarshal tags
		if err := json.Unmarshal([]byte(tagsJSON), &item.Tags); err != nil {
			return nil, fmt.Errorf("unmarshal tags: %w", err)
		}

		// Set nullable fields
		if title.Valid {
			item.Title = title.String
		}
		if teamID.Valid {
			item.TeamID = teamID.String
		}
		if authorName.Valid {
			item.AuthorName = authorName.String
		}
		if remoteID.Valid {
			item.RemoteID = remoteID.String
		}
		if lastAccessedAt.Valid {
			item.LastAccessedAt = &lastAccessedAt.Time
		}
		if deletedAt.Valid {
			item.DeletedAt = &deletedAt.Time
		}
		if lastSyncedAt.Valid {
			item.LastSyncedAt = lastSyncedAt.Time
		}

		items = append(items, &item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}

	return items, nil
}

// SearchKnowledgeFTS performs full-text search on knowledge items.
// Uses SQLite's FTS5 for fast, fuzzy matching.
func (s *Store) SearchKnowledgeFTS(ctx context.Context, query string, limit int) ([]*types.KnowledgeItem, error) {
	if limit <= 0 {
		limit = 10
	}

	// FTS search with ranking
	ftsQuery := `
		SELECT
			k.id, k.type, k.content, k.title, k.tags,
			k.scope, k.team_id, k.author_id, k.author_name,
			k.confidence, k.trust_score, k.success_count, k.failure_count,
			k.access_count, k.version, k.remote_id, k.sync_status,
			k.last_synced_at, k.created_at, k.updated_at,
			k.last_accessed_at, k.deleted_at,
			fts.rank
		FROM knowledge_fts fts
		JOIN knowledge_items k ON k.rowid = fts.rowid
		WHERE knowledge_fts MATCH ?
			AND k.deleted_at IS NULL
		ORDER BY fts.rank, k.trust_score DESC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, ftsQuery, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search knowledge items: %w", err)
	}
	defer rows.Close()

	var items []*types.KnowledgeItem
	for rows.Next() {
		var item types.KnowledgeItem
		var title, teamID, authorName, remoteID sql.NullString
		var tagsJSON string
		var lastAccessedAt, deletedAt, lastSyncedAt sql.NullTime
		var rank float64

		err := rows.Scan(
			&item.ID, &item.Type, &item.Content, &title, &tagsJSON,
			&item.Scope, &teamID, &item.AuthorID, &authorName,
			&item.Confidence, &item.TrustScore, &item.SuccessCount, &item.FailureCount,
			&item.AccessCount, &item.Version, &remoteID, &item.SyncStatus,
			&lastSyncedAt, &item.CreatedAt, &item.UpdatedAt,
			&lastAccessedAt, &deletedAt,
			&rank,
		)

		if err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}

		// Unmarshal tags
		if err := json.Unmarshal([]byte(tagsJSON), &item.Tags); err != nil {
			return nil, fmt.Errorf("unmarshal tags: %w", err)
		}

		// Set nullable fields
		if title.Valid {
			item.Title = title.String
		}
		if teamID.Valid {
			item.TeamID = teamID.String
		}
		if authorName.Valid {
			item.AuthorName = authorName.String
		}
		if remoteID.Valid {
			item.RemoteID = remoteID.String
		}
		if lastAccessedAt.Valid {
			item.LastAccessedAt = &lastAccessedAt.Time
		}
		if deletedAt.Valid {
			item.DeletedAt = &deletedAt.Time
		}
		if lastSyncedAt.Valid {
			item.LastSyncedAt = lastSyncedAt.Time
		}

		items = append(items, &item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate search results: %w", err)
	}

	return items, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// KNOWLEDGE STORE INTERFACE METHODS (implements knowledge.Store)
// ═══════════════════════════════════════════════════════════════════════════════

// Create implements knowledge.Store interface by delegating to CreateKnowledge.
func (s *Store) Create(ctx context.Context, item *types.KnowledgeItem) error {
	return s.CreateKnowledge(ctx, item)
}

// Update implements knowledge.Store interface by delegating to UpdateKnowledge.
func (s *Store) Update(ctx context.Context, item *types.KnowledgeItem) error {
	return s.UpdateKnowledge(ctx, item)
}

// Delete implements knowledge.Store interface by delegating to DeleteKnowledge.
func (s *Store) Delete(ctx context.Context, id string) error {
	return s.DeleteKnowledge(ctx, id)
}

// GetByID implements knowledge.Store interface by delegating to GetKnowledge.
func (s *Store) GetByID(ctx context.Context, id string) (*types.KnowledgeItem, error) {
	return s.GetKnowledge(ctx, id)
}

// GetByScope retrieves all knowledge items for a specific tier (personal, team, global).
// Implements knowledge.Store interface.
func (s *Store) GetByScope(ctx context.Context, scope types.Scope) ([]*types.KnowledgeItem, error) {
	query := `
		SELECT
			id, type, content, title, tags,
			scope, team_id, author_id, author_name,
			confidence, trust_score, success_count, failure_count,
			access_count, version, remote_id, sync_status,
			last_synced_at, created_at, updated_at,
			last_accessed_at, deleted_at
		FROM knowledge_items
		WHERE deleted_at IS NULL AND scope = ?
		ORDER BY trust_score DESC, updated_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, scope)
	if err != nil {
		return nil, fmt.Errorf("query knowledge by scope: %w", err)
	}
	defer rows.Close()

	return s.scanKnowledgeItems(rows)
}

// SearchByTags performs an exact tag match search within specified tiers.
// Returns items that contain ALL specified tags.
// Implements knowledge.Store interface.
func (s *Store) SearchByTags(ctx context.Context, tags []string, scopes []types.Scope) ([]*types.KnowledgeItem, error) {
	if len(tags) == 0 {
		return nil, fmt.Errorf("at least one tag is required")
	}

	// Build query with tag matching
	// SQLite JSON: We'll use LIKE on the JSON string since tags are stored as JSON array
	query := `
		SELECT
			id, type, content, title, tags,
			scope, team_id, author_id, author_name,
			confidence, trust_score, success_count, failure_count,
			access_count, version, remote_id, sync_status,
			last_synced_at, created_at, updated_at,
			last_accessed_at, deleted_at
		FROM knowledge_items
		WHERE deleted_at IS NULL
	`

	var args []interface{}

	// Add scope filter
	if len(scopes) > 0 {
		query += " AND scope IN ("
		for i, scope := range scopes {
			if i > 0 {
				query += ","
			}
			query += "?"
			args = append(args, scope)
		}
		query += ")"
	}

	// Add tag filter - must contain ALL specified tags
	for _, tag := range tags {
		query += " AND tags LIKE ?"
		// Use JSON-aware matching: look for the tag within the JSON array
		args = append(args, "%\""+tag+"\"%")
	}

	query += " ORDER BY trust_score DESC, updated_at DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search by tags: %w", err)
	}
	defer rows.Close()

	return s.scanKnowledgeItems(rows)
}

// IncrementAccessCount updates the access counter for a knowledge item.
// Implements knowledge.Store interface.
func (s *Store) IncrementAccessCount(ctx context.Context, id string) error {
	query := `
		UPDATE knowledge_items
		SET access_count = access_count + 1, last_accessed_at = ?
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := s.db.ExecContext(ctx, query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("increment access count: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("knowledge item not found: %s", id)
	}

	return nil
}

// UpdateTrustScore recalculates and updates the trust score for a knowledge item
// based on success/failure counts.
// Implements knowledge.Store interface.
func (s *Store) UpdateTrustScore(ctx context.Context, id string, successCount, failureCount int) error {
	// Calculate Bayesian trust score
	// Formula: (successes + prior) / (total + prior * 2)
	// Prior of 2 gives a slight benefit of the doubt for new items
	const prior = 2.0
	total := float64(successCount + failureCount)
	trustScore := (float64(successCount) + prior) / (total + prior*2)

	query := `
		UPDATE knowledge_items
		SET success_count = ?, failure_count = ?, trust_score = ?, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := s.db.ExecContext(ctx, query, successCount, failureCount, trustScore, time.Now(), id)
	if err != nil {
		return fmt.Errorf("update trust score: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("knowledge item not found: %s", id)
	}

	return nil
}

// scanKnowledgeItems is a helper function to scan multiple knowledge items from rows.
func (s *Store) scanKnowledgeItems(rows *sql.Rows) ([]*types.KnowledgeItem, error) {
	var items []*types.KnowledgeItem
	for rows.Next() {
		var item types.KnowledgeItem
		var title, teamID, authorName, remoteID sql.NullString
		var tagsJSON string
		var lastAccessedAt, deletedAt, lastSyncedAt sql.NullTime

		err := rows.Scan(
			&item.ID, &item.Type, &item.Content, &title, &tagsJSON,
			&item.Scope, &teamID, &item.AuthorID, &authorName,
			&item.Confidence, &item.TrustScore, &item.SuccessCount, &item.FailureCount,
			&item.AccessCount, &item.Version, &remoteID, &item.SyncStatus,
			&lastSyncedAt, &item.CreatedAt, &item.UpdatedAt,
			&lastAccessedAt, &deletedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("scan knowledge item: %w", err)
		}

		// Unmarshal tags
		if err := json.Unmarshal([]byte(tagsJSON), &item.Tags); err != nil {
			return nil, fmt.Errorf("unmarshal tags: %w", err)
		}

		// Set nullable fields
		if title.Valid {
			item.Title = title.String
		}
		if teamID.Valid {
			item.TeamID = teamID.String
		}
		if authorName.Valid {
			item.AuthorName = authorName.String
		}
		if remoteID.Valid {
			item.RemoteID = remoteID.String
		}
		if lastAccessedAt.Valid {
			item.LastAccessedAt = &lastAccessedAt.Time
		}
		if deletedAt.Valid {
			item.DeletedAt = &deletedAt.Time
		}
		if lastSyncedAt.Valid {
			item.LastSyncedAt = lastSyncedAt.Time
		}

		items = append(items, &item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}

	return items, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// TRUST PROFILE OPERATIONS (User/Author Trust)
// ═══════════════════════════════════════════════════════════════════════════════

// GetTrustProfile retrieves a user's trust profile for a specific domain.
// Returns a default profile if not found.
func (s *Store) GetTrustProfile(ctx context.Context, userID, domain string) (*types.TrustProfile, error) {
	query := `
		SELECT
			id, user_id, domain, score, success_count, failure_count,
			last_activity, created_at, updated_at
		FROM trust_profiles
		WHERE user_id = ? AND domain = ?
	`

	var profile types.TrustProfile
	var lastActivity sql.NullTime

	err := s.db.QueryRowContext(ctx, query, userID, domain).Scan(
		&profile.ID, &profile.UserID, &profile.Domain, &profile.Score,
		&profile.SuccessCount, &profile.FailureCount,
		&lastActivity, &profile.CreatedAt, &profile.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		// Return default profile
		return &types.TrustProfile{
			UserID:       userID,
			Domain:       domain,
			Score:        0.5,
			SuccessCount: 0,
			FailureCount: 0,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("query trust profile: %w", err)
	}

	if lastActivity.Valid {
		profile.LastActivity = lastActivity.Time
	}

	return &profile, nil
}

// UpdateUserTrustScore updates a user's trust score for a specific domain based on task outcome.
// Uses exponential moving average to weight recent performance.
// Note: This is different from UpdateTrustScore which updates knowledge item trust.
func (s *Store) UpdateUserTrustScore(ctx context.Context, userID, domain string, success bool) error {
	// Get current profile
	profile, err := s.GetTrustProfile(ctx, userID, domain)
	if err != nil {
		return fmt.Errorf("get trust profile: %w", err)
	}

	// Update counts
	if success {
		profile.SuccessCount++
	} else {
		profile.FailureCount++
	}

	// Calculate new score using exponential moving average
	// Alpha = 0.1 (gives more weight to historical performance)
	alpha := 0.1
	total := float64(profile.SuccessCount + profile.FailureCount)
	rawScore := float64(profile.SuccessCount) / total
	profile.Score = profile.Score*(1-alpha) + rawScore*alpha

	// Clamp to [0, 1]
	if profile.Score < 0 {
		profile.Score = 0
	} else if profile.Score > 1 {
		profile.Score = 1
	}

	// Upsert profile
	query := `
		INSERT INTO trust_profiles (
			user_id, domain, score, success_count, failure_count, last_activity
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, domain) DO UPDATE SET
			score = excluded.score,
			success_count = excluded.success_count,
			failure_count = excluded.failure_count,
			last_activity = excluded.last_activity,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err = s.db.ExecContext(ctx, query,
		userID, domain, profile.Score, profile.SuccessCount, profile.FailureCount, time.Now(),
	)

	if err != nil {
		return fmt.Errorf("update trust profile: %w", err)
	}

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// SESSION OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════════

// CreateSession creates a new conversation session.
func (s *Store) CreateSession(ctx context.Context, session *types.Session) error {
	if session.ID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	query := `
		INSERT INTO sessions (
			id, user_id, title, working_directory,
			platform_vendor, platform_name, platform_version,
			status, started_at, last_activity_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query,
		session.ID, session.UserID, nullString(session.Title), session.CWD,
		nullString(session.PlatformVendor), nullString(session.PlatformName),
		nullString(session.PlatformVersion), session.Status,
		session.StartedAt, session.LastActivityAt,
	)

	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	return nil
}

// GetSession retrieves a session by ID.
func (s *Store) GetSession(ctx context.Context, id string) (*types.Session, error) {
	query := `
		SELECT
			id, user_id, title, working_directory,
			platform_vendor, platform_name, platform_version,
			status, started_at, ended_at, last_activity_at
		FROM sessions
		WHERE id = ?
	`

	var session types.Session
	var title, platformVendor, platformName, platformVersion sql.NullString
	var endedAt sql.NullTime

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&session.ID, &session.UserID, &title, &session.CWD,
		&platformVendor, &platformName, &platformVersion,
		&session.Status, &session.StartedAt, &endedAt, &session.LastActivityAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found: %s", id)
		}
		return nil, fmt.Errorf("query session: %w", err)
	}

	// Set nullable fields
	if title.Valid {
		session.Title = title.String
	}
	if platformVendor.Valid {
		session.PlatformVendor = platformVendor.String
	}
	if platformName.Valid {
		session.PlatformName = platformName.String
	}
	if platformVersion.Valid {
		session.PlatformVersion = platformVersion.String
	}
	if endedAt.Valid {
		session.EndedAt = &endedAt.Time
	}

	return &session, nil
}

// AddMessage adds a message to a session.
func (s *Store) AddMessage(ctx context.Context, msg *types.SessionMessage) error {
	if msg.SessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	query := `
		INSERT INTO session_messages (
			session_id, role, content,
			tool_name, tool_input, tool_output, tool_success,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	var toolSuccess interface{}
	if msg.ToolSuccess != nil {
		if *msg.ToolSuccess {
			toolSuccess = 1
		} else {
			toolSuccess = 0
		}
	}

	_, err := s.db.ExecContext(ctx, query,
		msg.SessionID, msg.Role, msg.Content,
		nullString(msg.ToolName), nullString(msg.ToolInput),
		nullString(msg.ToolOutput), toolSuccess,
		msg.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("add message: %w", err)
	}

	// Update session last activity
	updateQuery := `UPDATE sessions SET last_activity_at = ? WHERE id = ?`
	_, err = s.db.ExecContext(ctx, updateQuery, time.Now(), msg.SessionID)
	if err != nil {
		return fmt.Errorf("update session activity: %w", err)
	}

	return nil
}

// GetSessionMessages retrieves all messages for a session in chronological order.
func (s *Store) GetSessionMessages(ctx context.Context, sessionID string) ([]*types.SessionMessage, error) {
	query := `
		SELECT
			id, session_id, role, content,
			tool_name, tool_input, tool_output, tool_success,
			created_at
		FROM session_messages
		WHERE session_id = ?
		ORDER BY created_at ASC
	`

	rows, err := s.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("query session messages: %w", err)
	}
	defer rows.Close()

	var messages []*types.SessionMessage
	for rows.Next() {
		var msg types.SessionMessage
		var toolName, toolInput, toolOutput sql.NullString
		var toolSuccess sql.NullInt64

		err := rows.Scan(
			&msg.ID, &msg.SessionID, &msg.Role, &msg.Content,
			&toolName, &toolInput, &toolOutput, &toolSuccess,
			&msg.CreatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}

		// Set nullable fields
		if toolName.Valid {
			msg.ToolName = toolName.String
		}
		if toolInput.Valid {
			msg.ToolInput = toolInput.String
		}
		if toolOutput.Valid {
			msg.ToolOutput = toolOutput.String
		}
		if toolSuccess.Valid {
			success := toolSuccess.Int64 == 1
			msg.ToolSuccess = &success
		}

		messages = append(messages, &msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate messages: %w", err)
	}

	return messages, nil
}

// UpdateSessionStatus updates a session's status and optionally sets ended_at.
func (s *Store) UpdateSessionStatus(ctx context.Context, sessionID, status string) error {
	var endedAt interface{}
	if status == "completed" || status == "abandoned" {
		endedAt = time.Now()
	}

	query := `
		UPDATE sessions
		SET status = ?, ended_at = ?, last_activity_at = ?
		WHERE id = ?
	`

	result, err := s.db.ExecContext(ctx, query, status, endedAt, time.Now(), sessionID)
	if err != nil {
		return fmt.Errorf("update session status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPER FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════════

// nullString converts a string to sql.NullString.
// Returns NULL if the string is empty.
func nullString(s string) sql.NullString {
	return sql.NullString{
		String: s,
		Valid:  s != "",
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// SYNC OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════════

// SyncOperation represents a pending sync operation.
type SyncOperation struct {
	ID            int        `json:"id"`
	ItemID        string     `json:"item_id"`
	ItemType      string     `json:"item_type"`
	Operation     string     `json:"operation"`
	Priority      int        `json:"priority"`
	QueuedAt      time.Time  `json:"queued_at"`
	Attempts      int        `json:"attempts"`
	MaxAttempts   int        `json:"max_attempts"`
	LastAttemptAt *time.Time `json:"last_attempt_at,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
}

// SyncState tracks sync progress for a scope.
type SyncState struct {
	Scope          types.Scope `json:"scope"`
	TeamID         string      `json:"team_id,omitempty"`
	LastSyncAt     *time.Time  `json:"last_sync_at,omitempty"`
	LastSyncCursor string      `json:"last_sync_cursor,omitempty"`
	ItemsSynced    int         `json:"items_synced"`
	ErrorsCount    int         `json:"errors_count"`
}

// SyncConflict represents a conflict that needs resolution.
type SyncConflict struct {
	ID             int        `json:"id"`
	ItemID         string     `json:"item_id"`
	LocalVersion   int        `json:"local_version"`
	RemoteVersion  int        `json:"remote_version"`
	LocalContent   string     `json:"local_content"`
	RemoteContent  string     `json:"remote_content"`
	Resolution     string     `json:"resolution"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	ResolvedBy     string     `json:"resolved_by,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// GetPendingSyncOps retrieves pending sync operations up to the given limit.
func (s *Store) GetPendingSyncOps(ctx context.Context, limit int) ([]*SyncOperation, error) {
	query := `
		SELECT id, item_id, item_type, operation, priority,
		       queued_at, attempts, max_attempts, last_attempt_at, last_error
		FROM sync_queue
		WHERE attempts < max_attempts
		ORDER BY priority ASC, queued_at ASC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("query pending sync ops: %w", err)
	}
	defer rows.Close()

	var ops []*SyncOperation
	for rows.Next() {
		var op SyncOperation
		var lastAttemptAt sql.NullTime
		var lastError sql.NullString

		err := rows.Scan(
			&op.ID, &op.ItemID, &op.ItemType, &op.Operation, &op.Priority,
			&op.QueuedAt, &op.Attempts, &op.MaxAttempts, &lastAttemptAt, &lastError,
		)
		if err != nil {
			return nil, fmt.Errorf("scan sync op: %w", err)
		}

		if lastAttemptAt.Valid {
			op.LastAttemptAt = &lastAttemptAt.Time
		}
		if lastError.Valid {
			op.LastError = lastError.String
		}

		ops = append(ops, &op)
	}

	return ops, nil
}

// RecordSyncAttempt records a failed sync attempt.
func (s *Store) RecordSyncAttempt(ctx context.Context, opID int, syncErr error) error {
	var errMsg string
	if syncErr != nil {
		errMsg = syncErr.Error()
	}

	query := `
		UPDATE sync_queue
		SET attempts = attempts + 1,
		    last_attempt_at = ?,
		    last_error = ?
		WHERE id = ?
	`

	_, err := s.db.ExecContext(ctx, query, time.Now(), errMsg, opID)
	if err != nil {
		return fmt.Errorf("record sync attempt: %w", err)
	}

	return nil
}

// CompleteSyncOp removes a completed sync operation from the queue.
func (s *Store) CompleteSyncOp(ctx context.Context, opID int) error {
	query := `DELETE FROM sync_queue WHERE id = ?`

	_, err := s.db.ExecContext(ctx, query, opID)
	if err != nil {
		return fmt.Errorf("complete sync op: %w", err)
	}

	return nil
}

// QueueForSync adds an item to the sync queue.
func (s *Store) QueueForSync(ctx context.Context, itemID, itemType, operation string) error {
	query := `
		INSERT OR REPLACE INTO sync_queue (item_id, item_type, operation, priority, queued_at)
		VALUES (?, ?, ?, 5, ?)
	`

	_, err := s.db.ExecContext(ctx, query, itemID, itemType, operation, time.Now())
	if err != nil {
		return fmt.Errorf("queue for sync: %w", err)
	}

	return nil
}

// GetSyncState retrieves the sync state for a scope.
func (s *Store) GetSyncState(ctx context.Context, scope types.Scope) (*SyncState, error) {
	query := `
		SELECT scope, team_id, last_sync_at, last_sync_cursor, items_synced, errors_count
		FROM sync_state
		WHERE scope = ?
	`

	var state SyncState
	var teamID, cursor sql.NullString
	var lastSyncAt sql.NullTime

	err := s.db.QueryRowContext(ctx, query, scope).Scan(
		&state.Scope, &teamID, &lastSyncAt, &cursor,
		&state.ItemsSynced, &state.ErrorsCount,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			// Return empty state for new scope
			return &SyncState{Scope: scope}, nil
		}
		return nil, fmt.Errorf("get sync state: %w", err)
	}

	if teamID.Valid {
		state.TeamID = teamID.String
	}
	if lastSyncAt.Valid {
		state.LastSyncAt = &lastSyncAt.Time
	}
	if cursor.Valid {
		state.LastSyncCursor = cursor.String
	}

	return &state, nil
}

// SetSyncState updates the sync state for a scope.
func (s *Store) SetSyncState(ctx context.Context, state *SyncState) error {
	query := `
		INSERT OR REPLACE INTO sync_state (scope, team_id, last_sync_at, last_sync_cursor, items_synced, errors_count)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query,
		state.Scope, nullString(state.TeamID),
		state.LastSyncAt, nullString(state.LastSyncCursor),
		state.ItemsSynced, state.ErrorsCount,
	)

	if err != nil {
		return fmt.Errorf("set sync state: %w", err)
	}

	return nil
}

// RecordConflict stores a sync conflict for manual resolution.
func (s *Store) RecordConflict(ctx context.Context, conflict *SyncConflict) error {
	query := `
		INSERT INTO sync_conflicts (item_id, local_version, remote_version, local_content, remote_content, resolution, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query,
		conflict.ItemID, conflict.LocalVersion, conflict.RemoteVersion,
		conflict.LocalContent, conflict.RemoteContent,
		conflict.Resolution, time.Now(),
	)

	if err != nil {
		return fmt.Errorf("record conflict: %w", err)
	}

	return nil
}

// GetPendingConflicts retrieves conflicts awaiting resolution.
func (s *Store) GetPendingConflicts(ctx context.Context) ([]*SyncConflict, error) {
	query := `
		SELECT id, item_id, local_version, remote_version, local_content, remote_content,
		       resolution, resolved_at, resolved_by, created_at
		FROM sync_conflicts
		WHERE resolution = 'pending'
		ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query pending conflicts: %w", err)
	}
	defer rows.Close()

	var conflicts []*SyncConflict
	for rows.Next() {
		var c SyncConflict
		var resolvedAt sql.NullTime
		var resolvedBy sql.NullString

		err := rows.Scan(
			&c.ID, &c.ItemID, &c.LocalVersion, &c.RemoteVersion,
			&c.LocalContent, &c.RemoteContent, &c.Resolution,
			&resolvedAt, &resolvedBy, &c.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan conflict: %w", err)
		}

		if resolvedAt.Valid {
			c.ResolvedAt = &resolvedAt.Time
		}
		if resolvedBy.Valid {
			c.ResolvedBy = resolvedBy.String
		}

		conflicts = append(conflicts, &c)
	}

	return conflicts, nil
}

// ResolveConflict marks a conflict as resolved.
func (s *Store) ResolveConflict(ctx context.Context, conflictID int, resolution, resolvedBy string) error {
	query := `
		UPDATE sync_conflicts
		SET resolution = ?, resolved_at = ?, resolved_by = ?
		WHERE id = ?
	`

	result, err := s.db.ExecContext(ctx, query, resolution, time.Now(), resolvedBy, conflictID)
	if err != nil {
		return fmt.Errorf("resolve conflict: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("conflict not found: %d", conflictID)
	}

	return nil
}
