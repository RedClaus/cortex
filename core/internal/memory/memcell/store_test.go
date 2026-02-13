package memcell

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ══════════════════════════════════════════════════════════════════════════════
// TEST HELPERS
// ══════════════════════════════════════════════════════════════════════════════

func setupTestDB(t *testing.T) (*sql.DB, func()) {
	// Create temp file
	tmpFile, err := os.CreateTemp("", "memcell_test_*.db")
	require.NoError(t, err)
	tmpFile.Close()

	// Open database
	db, err := sql.Open("sqlite3", tmpFile.Name()+"?_journal_mode=WAL")
	require.NoError(t, err)

	// Apply schema (without FTS5 for test compatibility)
	schema := `
		CREATE TABLE IF NOT EXISTS migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS memcells (
			id TEXT PRIMARY KEY,
			source_id TEXT,
			version INTEGER DEFAULT 1,
			created_at TEXT DEFAULT (datetime('now')),
			updated_at TEXT DEFAULT (datetime('now')),
			last_access_at TEXT,
			access_count INTEGER DEFAULT 0,
			raw_content TEXT NOT NULL,
			summary TEXT,
			entities TEXT,
			key_phrases TEXT,
			sentiment REAL DEFAULT 0,
			memory_type TEXT NOT NULL,
			confidence REAL DEFAULT 0.5,
			importance REAL DEFAULT 0.5,
			topics TEXT,
			scope TEXT DEFAULT 'personal',
			parent_id TEXT,
			supersedes_id TEXT,
			episode_id TEXT,
			event_boundary INTEGER DEFAULT 0,
			preceding_ctx TEXT,
			following_ctx TEXT,
			conversation_id TEXT,
			turn_number INTEGER,
			user_state TEXT
		);

		CREATE TABLE IF NOT EXISTS memcell_relations (
			from_id TEXT NOT NULL,
			to_id TEXT NOT NULL,
			relation_type TEXT NOT NULL,
			strength REAL DEFAULT 0.5,
			created_at TEXT DEFAULT (datetime('now')),
			PRIMARY KEY (from_id, to_id, relation_type)
		);
	`

	_, err = db.Exec(schema)
	require.NoError(t, err)

	// Try to create FTS5 table (may fail if FTS5 not available)
	ftsSchema := `
		CREATE VIRTUAL TABLE IF NOT EXISTS memcells_fts USING fts5(
			raw_content,
			summary,
			entities,
			key_phrases,
			content='memcells',
			content_rowid='rowid'
		);

		CREATE TRIGGER IF NOT EXISTS memcells_ai AFTER INSERT ON memcells BEGIN
			INSERT INTO memcells_fts(rowid, raw_content, summary, entities, key_phrases)
			VALUES (new.rowid, new.raw_content, new.summary, new.entities, new.key_phrases);
		END;
	`
	// Ignore FTS5 errors - tests will skip FTS-dependent functionality
	db.Exec(ftsSchema)

	cleanup := func() {
		db.Close()
		os.Remove(tmpFile.Name())
	}

	return db, cleanup
}

func createTestCell(id string) *MemCell {
	return &MemCell{
		ID:          id,
		RawContent:  "Test content for " + id,
		MemoryType:  MemTypeKnowledge,
		Confidence:  0.8,
		Importance:  0.7,
		Scope:       ScopePersonal,
		Entities:    []string{"entity1", "entity2"},
		KeyPhrases:  []string{"phrase1"},
		Topics:      []string{"topic1"},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		AccessCount: 0,
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// STORE TESTS
// ══════════════════════════════════════════════════════════════════════════════

func TestSQLiteStore_Create(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewSQLiteStore(db)
	ctx := context.Background()

	cell := createTestCell("test-1")
	err := store.Create(ctx, cell)
	require.NoError(t, err)

	// Verify created
	retrieved, err := store.Get(ctx, cell.ID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, cell.ID, retrieved.ID)
	assert.Equal(t, cell.RawContent, retrieved.RawContent)
	assert.Equal(t, cell.MemoryType, retrieved.MemoryType)
}

func TestSQLiteStore_CreateAutoID(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewSQLiteStore(db)
	ctx := context.Background()

	cell := &MemCell{
		RawContent: "Content without ID",
		MemoryType: MemTypeFact,
		Scope:      ScopePersonal,
	}

	err := store.Create(ctx, cell)
	require.NoError(t, err)
	assert.NotEmpty(t, cell.ID, "ID should be auto-generated")
}

func TestSQLiteStore_Get_NotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewSQLiteStore(db)
	ctx := context.Background()

	retrieved, err := store.Get(ctx, "non-existent")
	require.NoError(t, err)
	assert.Nil(t, retrieved)
}

func TestSQLiteStore_Update(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewSQLiteStore(db)
	ctx := context.Background()

	// Create
	cell := createTestCell("test-update")
	err := store.Create(ctx, cell)
	require.NoError(t, err)

	// Update
	cell.RawContent = "Updated content"
	cell.Importance = 0.9
	err = store.Update(ctx, cell)
	require.NoError(t, err)

	// Verify
	retrieved, err := store.Get(ctx, cell.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated content", retrieved.RawContent)
	assert.Equal(t, 0.9, retrieved.Importance)
	assert.Equal(t, 2, retrieved.Version) // Version incremented
}

func TestSQLiteStore_Delete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewSQLiteStore(db)
	ctx := context.Background()

	// Create
	cell := createTestCell("test-delete")
	err := store.Create(ctx, cell)
	require.NoError(t, err)

	// Delete
	err = store.Delete(ctx, cell.ID)
	require.NoError(t, err)

	// Verify deleted
	retrieved, err := store.Get(ctx, cell.ID)
	require.NoError(t, err)
	assert.Nil(t, retrieved)
}

func TestSQLiteStore_Search(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewSQLiteStore(db)
	ctx := context.Background()

	// Create test cells
	cells := []*MemCell{
		{ID: "search-1", RawContent: "Go programming language", MemoryType: MemTypeKnowledge, Importance: 0.8, Scope: ScopePersonal},
		{ID: "search-2", RawContent: "Python programming basics", MemoryType: MemTypeKnowledge, Importance: 0.6, Scope: ScopePersonal},
		{ID: "search-3", RawContent: "JavaScript is versatile", MemoryType: MemTypeFact, Importance: 0.7, Scope: ScopePersonal},
	}

	for _, cell := range cells {
		err := store.Create(ctx, cell)
		require.NoError(t, err)
	}

	// Search - may fail if FTS5 not available
	results, err := store.Search(ctx, "programming", SearchOptions{TopK: 10})
	if err != nil {
		t.Skip("FTS5 not available, skipping search test")
	}
	assert.Len(t, results, 2) // Only 2 contain "programming"
}

func TestSQLiteStore_SearchWithFilters(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewSQLiteStore(db)
	ctx := context.Background()

	// Create test cells
	cells := []*MemCell{
		{ID: "filter-1", RawContent: "Important principle", MemoryType: MemTypePrinciple, Importance: 0.9, Scope: ScopePersonal},
		{ID: "filter-2", RawContent: "Less important fact", MemoryType: MemTypeFact, Importance: 0.3, Scope: ScopePersonal},
		{ID: "filter-3", RawContent: "Important knowledge", MemoryType: MemTypeKnowledge, Importance: 0.8, Scope: ScopeTeam},
	}

	for _, cell := range cells {
		err := store.Create(ctx, cell)
		require.NoError(t, err)
	}

	// Search with importance filter - may fail if FTS5 not available
	results, err := store.Search(ctx, "important", SearchOptions{
		TopK:          10,
		MinImportance: 0.7,
	})
	if err != nil {
		t.Skip("FTS5 not available, skipping search test")
	}
	assert.Len(t, results, 2) // Only high importance

	// Search with type filter
	results, err = store.Search(ctx, "important", SearchOptions{
		TopK:        10,
		MemoryTypes: []MemoryType{MemTypePrinciple},
	})
	if err != nil {
		t.Skip("FTS5 not available, skipping search test")
	}
	assert.Len(t, results, 1)
}

func TestSQLiteStore_Relations(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewSQLiteStore(db)
	ctx := context.Background()

	// Create cells
	cell1 := createTestCell("rel-1")
	cell2 := createTestCell("rel-2")
	cell3 := createTestCell("rel-3")

	for _, cell := range []*MemCell{cell1, cell2, cell3} {
		err := store.Create(ctx, cell)
		require.NoError(t, err)
	}

	// Add relations
	err := store.AddRelation(ctx, cell1.ID, cell2.ID, RelTypeRelated, 0.8)
	require.NoError(t, err)
	err = store.AddRelation(ctx, cell1.ID, cell3.ID, RelTypeSupports, 0.9)
	require.NoError(t, err)

	// Get related
	related, err := store.GetRelated(ctx, cell1.ID, 1)
	require.NoError(t, err)
	assert.Len(t, related, 2)
}

func TestSQLiteStore_GetByEpisode(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewSQLiteStore(db)
	ctx := context.Background()

	episodeID := "episode-123"

	// Create cells in episode
	cells := []*MemCell{
		{ID: "ep-1", RawContent: "First turn", MemoryType: MemTypeInteraction, EpisodeID: episodeID, TurnNumber: 1, Scope: ScopePersonal},
		{ID: "ep-2", RawContent: "Second turn", MemoryType: MemTypeInteraction, EpisodeID: episodeID, TurnNumber: 2, Scope: ScopePersonal},
		{ID: "ep-3", RawContent: "Different episode", MemoryType: MemTypeInteraction, EpisodeID: "other", TurnNumber: 1, Scope: ScopePersonal},
	}

	for _, cell := range cells {
		err := store.Create(ctx, cell)
		require.NoError(t, err)
	}

	// Get by episode
	results, err := store.GetByEpisode(ctx, episodeID)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "ep-1", results[0].ID)
	assert.Equal(t, "ep-2", results[1].ID)
}

func TestSQLiteStore_GetByType(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewSQLiteStore(db)
	ctx := context.Background()

	// Create cells of different types
	cells := []*MemCell{
		{ID: "type-1", RawContent: "Principle 1", MemoryType: MemTypePrinciple, Importance: 0.9, Scope: ScopePersonal},
		{ID: "type-2", RawContent: "Principle 2", MemoryType: MemTypePrinciple, Importance: 0.7, Scope: ScopePersonal},
		{ID: "type-3", RawContent: "Fact 1", MemoryType: MemTypeFact, Importance: 0.8, Scope: ScopePersonal},
	}

	for _, cell := range cells {
		err := store.Create(ctx, cell)
		require.NoError(t, err)
	}

	// Get principles
	results, err := store.GetByType(ctx, MemTypePrinciple, SearchOptions{TopK: 10})
	require.NoError(t, err)
	assert.Len(t, results, 2)

	// Verify ordered by importance
	assert.Equal(t, "type-1", results[0].ID) // 0.9 importance
	assert.Equal(t, "type-2", results[1].ID) // 0.7 importance
}

func TestSQLiteStore_RecordAccess(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewSQLiteStore(db)
	ctx := context.Background()

	// Create
	cell := createTestCell("access-test")
	err := store.Create(ctx, cell)
	require.NoError(t, err)

	// Record access
	err = store.RecordAccess(ctx, cell.ID)
	require.NoError(t, err)

	// Verify access count was incremented
	retrieved, err := store.Get(ctx, cell.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, retrieved.AccessCount)
	// Note: LastAccessAt may not parse correctly in tests, just verify count
}

// ══════════════════════════════════════════════════════════════════════════════
// TYPE TESTS
// ══════════════════════════════════════════════════════════════════════════════

func TestMemoryType_Category(t *testing.T) {
	tests := []struct {
		memType  MemoryType
		expected string
	}{
		{MemTypeEpisode, "episodic"},
		{MemTypeEvent, "episodic"},
		{MemTypeFact, "semantic"},
		{MemTypeKnowledge, "semantic"},
		{MemTypePreference, "personal"},
		{MemTypePrinciple, "strategic"},
		{MemTypeContext, "contextual"},
	}

	for _, tt := range tests {
		t.Run(string(tt.memType), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.memType.Category())
		})
	}
}

func TestMemoryType_IsValid(t *testing.T) {
	// Valid types
	for _, mt := range AllMemoryTypes() {
		assert.True(t, mt.IsValid(), "Should be valid: %s", mt)
	}

	// Invalid type
	invalid := MemoryType("invalid")
	assert.False(t, invalid.IsValid())
}

// ══════════════════════════════════════════════════════════════════════════════
// CLASSIFIER TESTS
// ══════════════════════════════════════════════════════════════════════════════

func TestClassifier_PatternBased(t *testing.T) {
	classifier := NewClassifier(nil, nil)
	ctx := context.Background()

	tests := []struct {
		content  string
		expected MemoryType
	}{
		{"Always should validate user input before processing", MemTypePrinciple},
		{"I learned that caching improves performance", MemTypeLesson},
		{"Goal: finish the project by Friday", MemTypeGoal},
		{"I prefer using Go for backend development", MemTypePreference},
		{"To do this, run npm install first", MemTypeProcedure},
		{"HTTP status code 404 means that resource was not found", MemTypeFact}, // "means that" matches fact pattern
		{"Just a regular message", MemTypeInteraction},
	}

	for _, tt := range tests {
		name := tt.content
		if len(name) > 20 {
			name = name[:20]
		}
		t.Run(name, func(t *testing.T) {
			memType, confidence, err := classifier.Classify(ctx, tt.content)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, memType, "Content: %s", tt.content)
			assert.Greater(t, confidence, 0.0)
		})
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// EXTRACTOR TESTS
// ══════════════════════════════════════════════════════════════════════════════

func TestSimpleExtractor_Extract(t *testing.T) {
	extractor := NewSimpleExtractor(nil)
	ctx := context.Background()

	turns := []ConversationTurn{
		{ID: "1", Role: "user", Content: "Help me with Go programming", TurnNumber: 1, ConversationID: "conv-1", Timestamp: time.Now()},
		{ID: "2", Role: "assistant", Content: "Here's how to do it", TurnNumber: 2, ConversationID: "conv-1", Timestamp: time.Now()},
		{ID: "3", Role: "user", Content: "Anyway, what's for lunch?", TurnNumber: 3, ConversationID: "conv-1", Timestamp: time.Now()},
	}

	cells, err := extractor.Extract(ctx, turns)
	require.NoError(t, err)
	assert.Len(t, cells, 3)

	// Check first cell
	assert.Equal(t, "1", cells[0].SourceID)
	assert.Equal(t, "conv-1", cells[0].ConversationID)
	assert.Equal(t, 1, cells[0].TurnNumber)

	// Third cell should be event boundary (starts with "anyway")
	assert.True(t, cells[2].EventBoundary, "Should detect topic change starting with 'anyway'")
}

func TestSimpleExtractor_DetectBoundary(t *testing.T) {
	extractor := NewSimpleExtractor(nil)
	ctx := context.Background()

	tests := []struct {
		name     string
		prev     string
		current  string
		expected bool
	}{
		{"no_boundary", "That was interesting", "Tell me more", false},
		{"prev_thanks", "Thanks for the help!", "What else can we do?", true}, // prev contains "thanks"
		{"anyway_boundary", "Working on this", "anyway, let's discuss the meeting", true}, // starts with "anyway"
		{"continuation", "Step 1 begins", "Step 2 is next", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isBoundary, err := extractor.DetectBoundary(ctx, tt.prev, tt.current)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, isBoundary, "prev=%q current=%q", tt.prev, tt.current)
		})
	}
}

func TestSimpleExtractor_ExtractEntities(t *testing.T) {
	extractor := NewSimpleExtractor(nil)
	ctx := context.Background()

	content := `Check the CamelCaseFunction in /path/to/file.go and see https://example.com for more info. Also look at "MyProject" config.`

	entities, err := extractor.ExtractEntities(ctx, content)
	require.NoError(t, err)
	assert.NotEmpty(t, entities)

	// Should contain code pattern
	found := false
	for _, e := range entities {
		if e == "CamelCaseFunction" {
			found = true
			break
		}
	}
	assert.True(t, found, "Should extract CamelCaseFunction")
}
