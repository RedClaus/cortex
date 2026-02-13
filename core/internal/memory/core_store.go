package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// CoreMemoryStore manages persistent core memory in SQLite.
// Per CR-003 v1.1: No in-memory cache. SQLite is fast enough for single-user local app.
type CoreMemoryStore struct {
	db       *sql.DB
	config   CoreMemoryConfig
	embedder *EmbeddingCache // Optional embedding cache for semantic search
}

// CoreMemoryConfig configures the core memory store.
type CoreMemoryConfig struct {
	// Token budgets per lane
	FastLaneMaxTokens  int `json:"fast_lane_max_tokens"`  // ~400 tokens
	SmartLaneMaxTokens int `json:"smart_lane_max_tokens"` // ~2000 tokens

	// Limits
	MaxUserFacts      int  `json:"max_user_facts"`
	MaxPreferences    int  `json:"max_preferences"`
	AllowLLMUserWrite bool `json:"allow_llm_user_write"` // Can LLM write to user memory?
}

// DefaultCoreMemoryConfig returns sensible defaults.
func DefaultCoreMemoryConfig() CoreMemoryConfig {
	return CoreMemoryConfig{
		FastLaneMaxTokens:  400,  // Minimal context for fast responses
		SmartLaneMaxTokens: 2000, // Full context for complex queries
		MaxUserFacts:       20,   // Prevent unbounded growth
		MaxPreferences:     15,
		AllowLLMUserWrite:  true,
	}
}

// NewCoreMemoryStore creates a new core memory store.
func NewCoreMemoryStore(db *sql.DB, config CoreMemoryConfig) (*CoreMemoryStore, error) {
	store := &CoreMemoryStore{
		db:     db,
		config: config,
	}

	if err := store.migrate(); err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	log.Info().Msg("core memory store initialized")
	return store, nil
}

// SetEmbedder sets the embedding cache for semantic operations.
// This should be called after construction to enable embedding generation for facts.
func (s *CoreMemoryStore) SetEmbedder(ec *EmbeddingCache) {
	s.embedder = ec
	if ec != nil {
		log.Info().Msg("core memory store: embedder configured")
	}
}

// Embedder returns the configured embedding cache.
func (s *CoreMemoryStore) Embedder() *EmbeddingCache {
	return s.embedder
}

// migrate creates the required database tables.
func (s *CoreMemoryStore) migrate() error {
	migrations := []string{
		// User memory table
		`CREATE TABLE IF NOT EXISTS user_memory (
			user_id TEXT PRIMARY KEY,
			name TEXT,
			role TEXT,
			experience TEXT,
			os TEXT,
			shell TEXT,
			editor TEXT,
			preferences_json TEXT,
			custom_facts_json TEXT,
			prefers_concise INTEGER DEFAULT 0,
			prefers_verbose INTEGER DEFAULT 0,
			last_updated DATETIME DEFAULT CURRENT_TIMESTAMP,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Changelog for audit trail
		`CREATE TABLE IF NOT EXISTS user_memory_changelog (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			field_changed TEXT NOT NULL,
			old_value TEXT,
			new_value TEXT,
			change_source TEXT NOT NULL,
			changed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Index for efficient changelog queries
		`CREATE INDEX IF NOT EXISTS idx_changelog_user
		ON user_memory_changelog(user_id, changed_at DESC)`,

		// Project memory table
		`CREATE TABLE IF NOT EXISTS project_memory (
			project_id TEXT PRIMARY KEY,
			name TEXT,
			path TEXT,
			type TEXT,
			tech_stack_json TEXT,
			conventions_json TEXT,
			git_branch TEXT,
			metadata_json TEXT,
			last_updated DATETIME DEFAULT CURRENT_TIMESTAMP,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Strategic memory table (CR-015)
		`CREATE TABLE IF NOT EXISTS strategic_memory (
			id TEXT PRIMARY KEY,
			principle TEXT NOT NULL,
			category TEXT,
			trigger_pattern TEXT,
			embedding BLOB,
			success_count INTEGER DEFAULT 0,
			failure_count INTEGER DEFAULT 0,
			apply_count INTEGER DEFAULT 0,
			success_rate REAL DEFAULT 0.0,
			confidence REAL DEFAULT 0.5,
			source_sessions TEXT,
			last_used DATETIME,
			last_applied_at TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Memory topics table (CR-015)
		`CREATE TABLE IF NOT EXISTS memory_topics (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			keywords TEXT,
			centroid_embedding BLOB,
			member_count INTEGER DEFAULT 0,
			last_active_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			is_active INTEGER DEFAULT 1
		)`,

		// Note: idx_topics_active index is created after ALTER TABLE migration
		// to ensure is_active column exists for existing databases

		// Topic-memory associations
		`CREATE TABLE IF NOT EXISTS topic_memories (
			topic_id TEXT NOT NULL,
			memory_id TEXT NOT NULL,
			relevance_score REAL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (topic_id, memory_id),
			FOREIGN KEY (topic_id) REFERENCES memory_topics(id),
			FOREIGN KEY (memory_id) REFERENCES strategic_memory(id)
		)`,

		// Memory links table (CR-015)
		`CREATE TABLE IF NOT EXISTS memory_links (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source_id TEXT NOT NULL,
			target_id TEXT NOT NULL,
			link_type TEXT NOT NULL,
			strength REAL DEFAULT 0.5,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (source_id) REFERENCES strategic_memory(id),
			FOREIGN KEY (target_id) REFERENCES strategic_memory(id)
		)`,

		// Index for efficient link queries
		`CREATE INDEX IF NOT EXISTS idx_memory_links_source ON memory_links(source_id)`,
		`CREATE INDEX IF NOT EXISTS idx_memory_links_target ON memory_links(target_id)`,
	}

	for _, migration := range migrations {
		if _, err := s.db.Exec(migration); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	// Schema upgrades for existing tables (ALTER TABLE statements)
	// These add missing columns to tables that already exist
	alterMigrations := []struct {
		table  string
		column string
		sql    string
	}{
		// Strategic memory columns
		{
			table:  "strategic_memory",
			column: "last_applied_at",
			sql:    "ALTER TABLE strategic_memory ADD COLUMN last_applied_at TEXT",
		},
		{
			table:  "strategic_memory",
			column: "apply_count",
			sql:    "ALTER TABLE strategic_memory ADD COLUMN apply_count INTEGER DEFAULT 0",
		},
		{
			table:  "strategic_memory",
			column: "success_rate",
			sql:    "ALTER TABLE strategic_memory ADD COLUMN success_rate REAL DEFAULT 0.0",
		},
		{
			table:  "strategic_memory",
			column: "source_sessions",
			sql:    "ALTER TABLE strategic_memory ADD COLUMN source_sessions TEXT",
		},
		// Memory topics columns
		{
			table:  "memory_topics",
			column: "is_active",
			sql:    "ALTER TABLE memory_topics ADD COLUMN is_active INTEGER DEFAULT 1",
		},
		{
			table:  "memory_topics",
			column: "centroid_embedding",
			sql:    "ALTER TABLE memory_topics ADD COLUMN centroid_embedding BLOB",
		},
		{
			table:  "memory_topics",
			column: "last_active_at",
			sql:    "ALTER TABLE memory_topics ADD COLUMN last_active_at DATETIME",
		},
	}

	for _, alter := range alterMigrations {
		// Check if column exists
		var count int
		err := s.db.QueryRow(
			"SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?",
			alter.table, alter.column,
		).Scan(&count)
		if err != nil {
			log.Warn().Err(err).Str("table", alter.table).Msg("failed to check column existence")
			continue
		}
		if count == 0 {
			// Column doesn't exist, add it
			if _, err := s.db.Exec(alter.sql); err != nil {
				log.Warn().Err(err).Str("table", alter.table).Str("column", alter.column).Msg("failed to add column")
			} else {
				log.Info().Str("table", alter.table).Str("column", alter.column).Msg("added missing column")
			}
		}
	}

	// Create indexes after ALTER TABLE ensures columns exist
	postAlterIndexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_topics_active ON memory_topics(is_active)`,
	}
	for _, idx := range postAlterIndexes {
		if _, err := s.db.Exec(idx); err != nil {
			log.Warn().Err(err).Msg("failed to create post-migration index")
		}
	}

	log.Debug().Msg("core memory migrations applied")

	// Try to create FTS5 index for strategic memory (optional - may not be available in all SQLite builds)
	_, ftsErr := s.db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS strategic_memory_fts USING fts5(
		id, principle, category, content='strategic_memory', content_rowid='rowid'
	)`)
	if ftsErr != nil {
		log.Debug().Err(ftsErr).Msg("FTS5 not available for strategic memory search (optional feature)")
	}

	return nil
}

// GetUserMemory retrieves user memory for a given user ID.
// Returns empty memory if user doesn't exist yet.
func (s *CoreMemoryStore) GetUserMemory(ctx context.Context, userID string) (*UserMemory, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT name, role, experience, os, shell, editor,
		       preferences_json, custom_facts_json,
		       prefers_concise, prefers_verbose, last_updated
		FROM user_memory WHERE user_id = ?
	`, userID)

	var mem UserMemory
	var name, role, experience, osField, shell, editor sql.NullString
	var prefsJSON, factsJSON sql.NullString
	var prefersConcise, prefersVerbose sql.NullInt64
	var lastUpdated sql.NullTime

	err := row.Scan(
		&name, &role, &experience,
		&osField, &shell, &editor,
		&prefsJSON, &factsJSON,
		&prefersConcise, &prefersVerbose,
		&lastUpdated,
	)

	if err == sql.ErrNoRows {
		log.Debug().Str("user_id", userID).Msg("no user memory found, returning empty")
		return NewUserMemory(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	// Map nullable fields to struct
	mem.Name = name.String
	mem.Role = role.String
	mem.Experience = experience.String
	mem.OS = osField.String
	mem.Shell = shell.String
	mem.Editor = editor.String
	if lastUpdated.Valid {
		mem.LastUpdated = lastUpdated.Time
	} else {
		mem.LastUpdated = time.Now()
	}

	// Unmarshal JSON fields
	if prefsJSON.Valid && prefsJSON.String != "" {
		if err := json.Unmarshal([]byte(prefsJSON.String), &mem.Preferences); err != nil {
			log.Warn().Err(err).Msg("failed to unmarshal preferences")
			mem.Preferences = []UserPreference{}
		}
	} else {
		mem.Preferences = []UserPreference{}
	}

	if factsJSON.Valid && factsJSON.String != "" {
		if err := json.Unmarshal([]byte(factsJSON.String), &mem.CustomFacts); err != nil {
			log.Warn().Err(err).Msg("failed to unmarshal custom facts")
			mem.CustomFacts = []UserFact{}
		}
	} else {
		mem.CustomFacts = []UserFact{}
	}

	mem.PrefersConcise = prefersConcise.Valid && prefersConcise.Int64 == 1
	mem.PrefersVerbose = prefersVerbose.Valid && prefersVerbose.Int64 == 1

	log.Debug().
		Str("user_id", userID).
		Int("preferences", len(mem.Preferences)).
		Int("facts", len(mem.CustomFacts)).
		Msg("user memory loaded")

	return &mem, nil
}

// allowedUserFields defines the exact SQL column names that can be updated.
// This is used to prevent SQL injection by only allowing known-safe field names.
var allowedUserFields = map[string]string{
	"name":           "name",
	"role":           "role",
	"experience":     "experience",
	"os":             "os",
	"shell":          "shell",
	"editor":         "editor",
	"prefers_concise": "prefers_concise",
	"prefers_verbose": "prefers_verbose",
}

// UpdateUserField updates a single field in user memory.
// Only allowed fields can be updated to prevent arbitrary writes.
func (s *CoreMemoryStore) UpdateUserField(
	ctx context.Context,
	userID string,
	field string,
	value interface{},
	source string,
) error {
	// Validate field against allowlist and get the safe column name
	safeColumn, ok := allowedUserFields[field]
	if !ok {
		return fmt.Errorf("field %q not allowed for direct update", field)
	}

	// Get current value for changelog
	current, err := s.GetUserMemory(ctx, userID)
	if err != nil {
		return err
	}
	oldValue := s.getFieldValue(current, field)

	// Use switch to select pre-defined SQL queries (no string interpolation)
	// This eliminates any possibility of SQL injection
	var query string
	switch safeColumn {
	case "name":
		query = `INSERT INTO user_memory (user_id, name, last_updated) VALUES (?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(user_id) DO UPDATE SET name = excluded.name, last_updated = CURRENT_TIMESTAMP`
	case "role":
		query = `INSERT INTO user_memory (user_id, role, last_updated) VALUES (?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(user_id) DO UPDATE SET role = excluded.role, last_updated = CURRENT_TIMESTAMP`
	case "experience":
		query = `INSERT INTO user_memory (user_id, experience, last_updated) VALUES (?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(user_id) DO UPDATE SET experience = excluded.experience, last_updated = CURRENT_TIMESTAMP`
	case "os":
		query = `INSERT INTO user_memory (user_id, os, last_updated) VALUES (?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(user_id) DO UPDATE SET os = excluded.os, last_updated = CURRENT_TIMESTAMP`
	case "shell":
		query = `INSERT INTO user_memory (user_id, shell, last_updated) VALUES (?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(user_id) DO UPDATE SET shell = excluded.shell, last_updated = CURRENT_TIMESTAMP`
	case "editor":
		query = `INSERT INTO user_memory (user_id, editor, last_updated) VALUES (?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(user_id) DO UPDATE SET editor = excluded.editor, last_updated = CURRENT_TIMESTAMP`
	case "prefers_concise":
		query = `INSERT INTO user_memory (user_id, prefers_concise, last_updated) VALUES (?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(user_id) DO UPDATE SET prefers_concise = excluded.prefers_concise, last_updated = CURRENT_TIMESTAMP`
	case "prefers_verbose":
		query = `INSERT INTO user_memory (user_id, prefers_verbose, last_updated) VALUES (?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(user_id) DO UPDATE SET prefers_verbose = excluded.prefers_verbose, last_updated = CURRENT_TIMESTAMP`
	default:
		return fmt.Errorf("field %q not allowed for direct update", field)
	}

	_, err = s.db.ExecContext(ctx, query, userID, value)
	if err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	// Record change for audit trail
	s.recordChange(ctx, userID, field, oldValue, fmt.Sprintf("%v", value), source)

	log.Info().
		Str("user_id", userID).
		Str("field", field).
		Str("source", source).
		Msg("user memory field updated")

	return nil
}

// AppendUserFact adds a custom fact about the user.
// Enforces MaxUserFacts limit by removing oldest facts.
func (s *CoreMemoryStore) AppendUserFact(
	ctx context.Context,
	userID string,
	fact string,
	source string,
) error {
	mem, err := s.GetUserMemory(ctx, userID)
	if err != nil {
		return err
	}

	// Check for duplicate
	for _, f := range mem.CustomFacts {
		if f.Fact == fact {
			log.Debug().Str("fact", fact).Msg("duplicate fact, skipping")
			return nil
		}
	}

	// Enforce limit by removing oldest
	if len(mem.CustomFacts) >= s.config.MaxUserFacts {
		mem.CustomFacts = mem.CustomFacts[1:] // Remove oldest
		log.Debug().Msg("removed oldest fact to enforce limit")
	}

	// Add new fact
	mem.CustomFacts = append(mem.CustomFacts, UserFact{
		Fact:      fact,
		Source:    source,
		CreatedAt: time.Now(),
	})

	// Persist
	factsJSON, err := json.Marshal(mem.CustomFacts)
	if err != nil {
		return fmt.Errorf("failed to marshal facts: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO user_memory (user_id, custom_facts_json, last_updated)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id) DO UPDATE SET
			custom_facts_json = excluded.custom_facts_json,
			last_updated = CURRENT_TIMESTAMP
	`, userID, string(factsJSON))

	if err != nil {
		return fmt.Errorf("failed to save facts: %w", err)
	}

	s.recordChange(ctx, userID, "custom_facts", "", fact, source)

	// Generate embedding asynchronously if embedder is available
	// The embedding will be cached in content_embedding_cache for semantic search
	if s.embedder != nil {
		go func(factText string) {
			embedCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			_, err := s.embedder.Embed(embedCtx, factText)
			if err != nil {
				log.Debug().Err(err).Str("fact", factText[:min(50, len(factText))]).Msg("failed to generate fact embedding")
			} else {
				log.Debug().Str("fact", factText[:min(50, len(factText))]).Msg("fact embedding generated and cached")
			}
		}(fact)
	}

	log.Info().
		Str("user_id", userID).
		Str("fact", fact).
		Str("source", source).
		Msg("user fact appended")

	return nil
}

// AppendUserPreference adds or updates a preference.
func (s *CoreMemoryStore) AppendUserPreference(
	ctx context.Context,
	userID string,
	pref UserPreference,
) error {
	mem, err := s.GetUserMemory(ctx, userID)
	if err != nil {
		return err
	}

	// Update existing or add new
	found := false
	for i, p := range mem.Preferences {
		if p.Category == pref.Category {
			mem.Preferences[i] = pref
			found = true
			break
		}
	}

	if !found {
		// Add the new preference first
		mem.Preferences = append(mem.Preferences, pref)

		// Enforce limit by removing lowest confidence if over limit
		if len(mem.Preferences) > s.config.MaxPreferences {
			// Remove lowest confidence preference
			minIdx := 0
			minConf := mem.Preferences[0].Confidence
			for i, p := range mem.Preferences {
				if p.Confidence < minConf {
					minIdx = i
					minConf = p.Confidence
				}
			}
			mem.Preferences = append(mem.Preferences[:minIdx], mem.Preferences[minIdx+1:]...)
		}
	}

	// Persist
	prefsJSON, err := json.Marshal(mem.Preferences)
	if err != nil {
		return fmt.Errorf("failed to marshal preferences: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO user_memory (user_id, preferences_json, last_updated)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id) DO UPDATE SET
			preferences_json = excluded.preferences_json,
			last_updated = CURRENT_TIMESTAMP
	`, userID, string(prefsJSON))

	if err != nil {
		return fmt.Errorf("failed to save preferences: %w", err)
	}

	log.Info().
		Str("user_id", userID).
		Str("category", pref.Category).
		Str("preference", pref.Preference).
		Msg("user preference saved")

	return nil
}

// GetProjectMemory retrieves project memory for a given project ID.
func (s *CoreMemoryStore) GetProjectMemory(ctx context.Context, projectID string) (*ProjectMemory, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT name, path, type, tech_stack_json, conventions_json,
		       git_branch, metadata_json, last_updated
		FROM project_memory WHERE project_id = ?
	`, projectID)

	var mem ProjectMemory
	var techStackJSON, conventionsJSON, metadataJSON sql.NullString

	err := row.Scan(
		&mem.Name, &mem.Path, &mem.Type,
		&techStackJSON, &conventionsJSON,
		&mem.GitBranch, &metadataJSON,
		&mem.LastUpdated,
	)

	if err == sql.ErrNoRows {
		return NewProjectMemory(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	// Unmarshal JSON fields
	if techStackJSON.Valid && techStackJSON.String != "" {
		json.Unmarshal([]byte(techStackJSON.String), &mem.TechStack)
	}
	if conventionsJSON.Valid && conventionsJSON.String != "" {
		json.Unmarshal([]byte(conventionsJSON.String), &mem.Conventions)
	}
	if metadataJSON.Valid && metadataJSON.String != "" {
		json.Unmarshal([]byte(metadataJSON.String), &mem.Metadata)
	}

	// Ensure non-nil slices/maps
	if mem.TechStack == nil {
		mem.TechStack = []string{}
	}
	if mem.Conventions == nil {
		mem.Conventions = []string{}
	}
	if mem.Metadata == nil {
		mem.Metadata = make(map[string]string)
	}

	return &mem, nil
}

// SaveProjectMemory saves or updates project memory.
func (s *CoreMemoryStore) SaveProjectMemory(ctx context.Context, projectID string, mem *ProjectMemory) error {
	techStackJSON, _ := json.Marshal(mem.TechStack)
	conventionsJSON, _ := json.Marshal(mem.Conventions)
	metadataJSON, _ := json.Marshal(mem.Metadata)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO project_memory (
			project_id, name, path, type, tech_stack_json,
			conventions_json, git_branch, metadata_json, last_updated
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(project_id) DO UPDATE SET
			name = excluded.name,
			path = excluded.path,
			type = excluded.type,
			tech_stack_json = excluded.tech_stack_json,
			conventions_json = excluded.conventions_json,
			git_branch = excluded.git_branch,
			metadata_json = excluded.metadata_json,
			last_updated = CURRENT_TIMESTAMP
	`, projectID, mem.Name, mem.Path, mem.Type,
		string(techStackJSON), string(conventionsJSON),
		mem.GitBranch, string(metadataJSON))

	if err != nil {
		return fmt.Errorf("failed to save project memory: %w", err)
	}

	log.Info().
		Str("project_id", projectID).
		Str("name", mem.Name).
		Msg("project memory saved")

	return nil
}

// GetChangelog returns recent changes to user memory.
func (s *CoreMemoryStore) GetChangelog(ctx context.Context, userID string, limit int) ([]MemoryChange, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT field_changed, old_value, new_value, change_source, changed_at
		FROM user_memory_changelog
		WHERE user_id = ?
		ORDER BY changed_at DESC
		LIMIT ?
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var changes []MemoryChange
	for rows.Next() {
		var c MemoryChange
		if err := rows.Scan(&c.Field, &c.OldValue, &c.NewValue, &c.Source, &c.ChangedAt); err != nil {
			return nil, err
		}
		changes = append(changes, c)
	}

	return changes, nil
}

// MemoryChange represents a change to user memory.
type MemoryChange struct {
	Field     string    `json:"field"`
	OldValue  string    `json:"old_value"`
	NewValue  string    `json:"new_value"`
	Source    string    `json:"source"`
	ChangedAt time.Time `json:"changed_at"`
}

// recordChange logs a change to the changelog table.
func (s *CoreMemoryStore) recordChange(ctx context.Context, userID, field, oldVal, newVal, source string) {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_memory_changelog (user_id, field_changed, old_value, new_value, change_source)
		VALUES (?, ?, ?, ?, ?)
	`, userID, field, oldVal, newVal, source)

	if err != nil {
		log.Warn().Err(err).Msg("failed to record memory change")
	}
}

// getFieldValue extracts a field value from UserMemory for changelog.
func (s *CoreMemoryStore) getFieldValue(mem *UserMemory, field string) string {
	if mem == nil {
		return ""
	}
	switch field {
	case "name":
		return mem.Name
	case "role":
		return mem.Role
	case "experience":
		return mem.Experience
	case "os":
		return mem.OS
	case "shell":
		return mem.Shell
	case "editor":
		return mem.Editor
	case "prefers_concise":
		if mem.PrefersConcise {
			return "true"
		}
		return "false"
	case "prefers_verbose":
		if mem.PrefersVerbose {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

// Config returns the store configuration.
func (s *CoreMemoryStore) Config() CoreMemoryConfig {
	return s.config
}

// ExtractNameFromFacts attempts to extract the user's name from stored facts.
// This is used to populate the name field when facts mention a name but profile is empty.
// Returns the extracted name, or empty string if no name could be extracted.
func (s *CoreMemoryStore) ExtractNameFromFacts(ctx context.Context, userID string) (string, error) {
	mem, err := s.GetUserMemory(ctx, userID)
	if err != nil {
		return "", err
	}

	// If name is already set, no need to extract
	if mem.Name != "" {
		return mem.Name, nil
	}

	// Look for patterns in facts that reveal the user's name
	// Common patterns: "Norman is a...", "Norman works...", "The user Norman..."
	for _, fact := range mem.CustomFacts {
		name := extractNameFromText(fact.Fact)
		if name != "" {
			log.Info().
				Str("user_id", userID).
				Str("extracted_name", name).
				Str("from_fact", fact.Fact).
				Msg("extracted user name from facts")
			return name, nil
		}
	}

	return "", nil
}

// extractNameFromText attempts to extract a first name from a fact string.
// Looks for patterns like "Name is a...", "Name works...", "Name's...".
func extractNameFromText(text string) string {
	// Simple heuristic: First word before verb patterns
	patterns := []string{
		" is a ",
		" is an ",
		" is the ",
		" is interested ",
		" is planning ",
		" is currently ",
		" is learning ",
		" is working ",
		" is located ",
		" works ",
		" worked ",
		" has ",
		" had ",
		" lives ",
		" lived ",
		" uses ",
		" prefers ",
		" successfully ",
		" will ",
		"'s ",
	}

	lowerText := strings.ToLower(text)
	for _, pattern := range patterns {
		idx := strings.Index(lowerText, pattern)
		if idx > 0 {
			// Extract the word before the pattern
			prefix := strings.TrimSpace(text[:idx])
			// Get the last word (in case there's "The user Norman")
			words := strings.Fields(prefix)
			if len(words) > 0 {
				name := words[len(words)-1]
				// Basic validation: starts with capital, 2-15 chars
				if len(name) >= 2 && len(name) <= 15 && name[0] >= 'A' && name[0] <= 'Z' {
					// Skip common non-names
					skip := map[string]bool{
						"The": true, "He": true, "She": true, "They": true,
						"It": true, "User": true, "This": true, "That": true,
					}
					if !skip[name] {
						return name
					}
				}
			}
		}
	}
	return ""
}

// AutoPopulateProfile extracts profile fields from facts if they're empty.
// Call this on startup or when loading user memory.
func (s *CoreMemoryStore) AutoPopulateProfile(ctx context.Context, userID string) error {
	name, err := s.ExtractNameFromFacts(ctx, userID)
	if err != nil {
		return err
	}

	if name != "" {
		mem, _ := s.GetUserMemory(ctx, userID)
		if mem.Name == "" {
			log.Info().
				Str("user_id", userID).
				Str("name", name).
				Msg("auto-populating user name from facts")
			return s.UpdateUserField(ctx, userID, "name", name, "auto_extracted")
		}
	}

	return nil
}
