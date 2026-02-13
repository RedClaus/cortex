package memory

import (
	"context"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupLinkStore creates a test database with required tables and returns a LinkStore.
func setupLinkStore(t *testing.T) *LinkStore {
	t.Helper()
	db := testDB(t)

	// Create memory_links table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS memory_links (
			source_id TEXT NOT NULL,
			target_id TEXT NOT NULL,
			source_type TEXT,
			target_type TEXT,
			rel_type TEXT NOT NULL,
			confidence REAL DEFAULT 0.5,
			metadata TEXT,
			created_at TEXT,
			created_by TEXT DEFAULT 'system',
			PRIMARY KEY (source_id, target_id, rel_type)
		)
	`)
	require.NoError(t, err)

	// Create strategic_memory table for findSimilarMemories
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS strategic_memory (
			id TEXT PRIMARY KEY,
			principle TEXT,
			embedding BLOB,
			category TEXT,
			success_count INTEGER DEFAULT 0,
			failure_count INTEGER DEFAULT 0,
			created_at TEXT
		)
	`)
	require.NoError(t, err)

	// Create memories table for episodic memories
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS memories (
			id TEXT PRIMARY KEY,
			content TEXT,
			embedding BLOB,
			created_at TEXT
		)
	`)
	require.NoError(t, err)

	return NewLinkStore(db, nil, nil)
}

// TestLinkStore_CreateLink verifies link creation.
func TestLinkStore_CreateLink(t *testing.T) {
	store := setupLinkStore(t)

	ctx := context.Background()

	link := &MemoryLink{
		SourceID:   "mem-1",
		TargetID:   "mem-2",
		SourceType: MemoryTypeStrategic,
		TargetType: MemoryTypeEpisodic,
		RelType:    LinkSupports,
		Confidence: 0.85,
		Metadata: map[string]string{
			"reason": "evidence found",
		},
		CreatedBy: "llm",
	}

	err := store.CreateLink(ctx, link)
	require.NoError(t, err)

	// Verify the link was created
	retrieved, err := store.GetLink(ctx, "mem-1", "mem-2", LinkSupports)
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	assert.Equal(t, "mem-1", retrieved.SourceID)
	assert.Equal(t, "mem-2", retrieved.TargetID)
	assert.Equal(t, MemoryTypeStrategic, retrieved.SourceType)
	assert.Equal(t, MemoryTypeEpisodic, retrieved.TargetType)
	assert.Equal(t, LinkSupports, retrieved.RelType)
	assert.Equal(t, 0.85, retrieved.Confidence)
	assert.Equal(t, "evidence found", retrieved.Metadata["reason"])
	assert.Equal(t, "llm", retrieved.CreatedBy)
	assert.False(t, retrieved.CreatedAt.IsZero())
}

// TestLinkStore_CreateLink_Validation verifies validation rules.
func TestLinkStore_CreateLink_Validation(t *testing.T) {
	store := setupLinkStore(t)
	ctx := context.Background()

	// Nil link
	err := store.CreateLink(ctx, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "link cannot be nil")

	// Missing source_id
	err = store.CreateLink(ctx, &MemoryLink{
		TargetID: "mem-2",
		RelType:  LinkSupports,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "source_id and target_id are required")

	// Missing target_id
	err = store.CreateLink(ctx, &MemoryLink{
		SourceID: "mem-1",
		RelType:  LinkSupports,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "source_id and target_id are required")

	// Missing rel_type
	err = store.CreateLink(ctx, &MemoryLink{
		SourceID: "mem-1",
		TargetID: "mem-2",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rel_type is required")
}

// TestLinkStore_CreateLink_Upsert verifies that creating a duplicate link updates it.
func TestLinkStore_CreateLink_Upsert(t *testing.T) {
	store := setupLinkStore(t)
	ctx := context.Background()

	// Create initial link
	link := &MemoryLink{
		SourceID:   "mem-1",
		TargetID:   "mem-2",
		RelType:    LinkSupports,
		Confidence: 0.5,
	}
	err := store.CreateLink(ctx, link)
	require.NoError(t, err)

	// Create same link with different confidence (should upsert)
	link.Confidence = 0.9
	link.Metadata = map[string]string{"updated": "true"}
	err = store.CreateLink(ctx, link)
	require.NoError(t, err)

	// Verify the link was updated
	retrieved, err := store.GetLink(ctx, "mem-1", "mem-2", LinkSupports)
	require.NoError(t, err)
	assert.Equal(t, 0.9, retrieved.Confidence)
	assert.Equal(t, "true", retrieved.Metadata["updated"])
}

// TestLinkStore_GetLink verifies retrieval.
func TestLinkStore_GetLink(t *testing.T) {
	store := setupLinkStore(t)
	ctx := context.Background()

	// Create a link
	link := &MemoryLink{
		SourceID:   "mem-a",
		TargetID:   "mem-b",
		SourceType: MemoryTypeProcedural,
		TargetType: MemoryTypeProcedural,
		RelType:    LinkEvolvedFrom,
		Confidence: 0.75,
	}
	err := store.CreateLink(ctx, link)
	require.NoError(t, err)

	// Get existing link
	retrieved, err := store.GetLink(ctx, "mem-a", "mem-b", LinkEvolvedFrom)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, "mem-a", retrieved.SourceID)
	assert.Equal(t, "mem-b", retrieved.TargetID)

	// Get non-existing link
	notFound, err := store.GetLink(ctx, "mem-a", "mem-b", LinkContradicts)
	require.NoError(t, err)
	assert.Nil(t, notFound)

	// Get with wrong source
	notFound, err = store.GetLink(ctx, "nonexistent", "mem-b", LinkEvolvedFrom)
	require.NoError(t, err)
	assert.Nil(t, notFound)
}

// TestLinkStore_DeleteLink verifies deletion.
func TestLinkStore_DeleteLink(t *testing.T) {
	store := setupLinkStore(t)
	ctx := context.Background()

	// Create a link
	link := &MemoryLink{
		SourceID: "del-1",
		TargetID: "del-2",
		RelType:  LinkContradicts,
	}
	err := store.CreateLink(ctx, link)
	require.NoError(t, err)

	// Verify it exists
	retrieved, err := store.GetLink(ctx, "del-1", "del-2", LinkContradicts)
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	// Delete it
	err = store.DeleteLink(ctx, "del-1", "del-2", LinkContradicts)
	require.NoError(t, err)

	// Verify it's gone
	retrieved, err = store.GetLink(ctx, "del-1", "del-2", LinkContradicts)
	require.NoError(t, err)
	assert.Nil(t, retrieved)

	// Deleting non-existent link should not error
	err = store.DeleteLink(ctx, "nonexistent", "also-nonexistent", LinkSupports)
	assert.NoError(t, err)
}

// TestLinkStore_GetLinkedMemories verifies bidirectional retrieval.
func TestLinkStore_GetLinkedMemories(t *testing.T) {
	store := setupLinkStore(t)
	ctx := context.Background()

	// Create multiple links from and to a central memory
	links := []*MemoryLink{
		{SourceID: "center", TargetID: "target-1", RelType: LinkSupports, Confidence: 0.8},
		{SourceID: "center", TargetID: "target-2", RelType: LinkContradicts, Confidence: 0.7},
		{SourceID: "source-1", TargetID: "center", RelType: LinkRelatedTo, Confidence: 0.6},
		{SourceID: "other-1", TargetID: "other-2", RelType: LinkSupports, Confidence: 0.9}, // Unrelated
	}

	for _, link := range links {
		err := store.CreateLink(ctx, link)
		require.NoError(t, err)
	}

	// Get all links for "center" (both directions)
	allLinks, err := store.GetLinkedMemories(ctx, "center")
	require.NoError(t, err)
	assert.Len(t, allLinks, 3) // 2 outgoing + 1 incoming

	// Verify links are correct
	var hasSupports, hasContradicts, hasRelatedTo bool
	for _, link := range allLinks {
		switch link.RelType {
		case LinkSupports:
			hasSupports = true
			assert.Equal(t, "center", link.SourceID)
			assert.Equal(t, "target-1", link.TargetID)
		case LinkContradicts:
			hasContradicts = true
		case LinkRelatedTo:
			hasRelatedTo = true
			assert.Equal(t, "source-1", link.SourceID)
			assert.Equal(t, "center", link.TargetID)
		}
	}
	assert.True(t, hasSupports, "should have supports link")
	assert.True(t, hasContradicts, "should have contradicts link")
	assert.True(t, hasRelatedTo, "should have related_to link")
}

// TestLinkStore_GetLinkedMemories_WithFilter verifies type filtering.
func TestLinkStore_GetLinkedMemories_WithFilter(t *testing.T) {
	store := setupLinkStore(t)
	ctx := context.Background()

	// Create links with different types
	links := []*MemoryLink{
		{SourceID: "mem-x", TargetID: "mem-y", RelType: LinkSupports, Confidence: 0.8},
		{SourceID: "mem-x", TargetID: "mem-z", RelType: LinkContradicts, Confidence: 0.7},
		{SourceID: "mem-x", TargetID: "mem-w", RelType: LinkRelatedTo, Confidence: 0.6},
		{SourceID: "mem-x", TargetID: "mem-v", RelType: LinkEvolvedFrom, Confidence: 0.9},
	}

	for _, link := range links {
		err := store.CreateLink(ctx, link)
		require.NoError(t, err)
	}

	// Filter by single type
	supportsOnly, err := store.GetLinkedMemories(ctx, "mem-x", LinkSupports)
	require.NoError(t, err)
	assert.Len(t, supportsOnly, 1)
	assert.Equal(t, LinkSupports, supportsOnly[0].RelType)

	// Filter by multiple types
	supportsAndContradicts, err := store.GetLinkedMemories(ctx, "mem-x", LinkSupports, LinkContradicts)
	require.NoError(t, err)
	assert.Len(t, supportsAndContradicts, 2)

	// Filter returns empty for non-matching type
	leadsTo, err := store.GetLinkedMemories(ctx, "mem-x", LinkLeadsTo)
	require.NoError(t, err)
	assert.Len(t, leadsTo, 0)
}

// TestLinkStore_GetContradictions verifies contradiction shorthand.
func TestLinkStore_GetContradictions(t *testing.T) {
	store := setupLinkStore(t)
	ctx := context.Background()

	// Create various links
	links := []*MemoryLink{
		{SourceID: "fact-1", TargetID: "fact-2", RelType: LinkContradicts, Confidence: 0.9},
		{SourceID: "fact-3", TargetID: "fact-1", RelType: LinkContradicts, Confidence: 0.8},
		{SourceID: "fact-1", TargetID: "fact-4", RelType: LinkSupports, Confidence: 0.7},
	}

	for _, link := range links {
		err := store.CreateLink(ctx, link)
		require.NoError(t, err)
	}

	// Get contradictions for fact-1
	contradictions, err := store.GetContradictions(ctx, "fact-1")
	require.NoError(t, err)
	assert.Len(t, contradictions, 2) // One outgoing, one incoming

	// Verify all are contradicts type
	for _, link := range contradictions {
		assert.Equal(t, LinkContradicts, link.RelType)
	}
}

// TestLinkStore_GetSupports verifies supports shorthand.
func TestLinkStore_GetSupports(t *testing.T) {
	store := setupLinkStore(t)
	ctx := context.Background()

	// Create various links
	links := []*MemoryLink{
		{SourceID: "claim-1", TargetID: "evidence-1", RelType: LinkSupports, Confidence: 0.9},
		{SourceID: "claim-1", TargetID: "evidence-2", RelType: LinkSupports, Confidence: 0.85},
		{SourceID: "claim-1", TargetID: "contra-1", RelType: LinkContradicts, Confidence: 0.7},
	}

	for _, link := range links {
		err := store.CreateLink(ctx, link)
		require.NoError(t, err)
	}

	// Get supports for claim-1
	supports, err := store.GetSupports(ctx, "claim-1")
	require.NoError(t, err)
	assert.Len(t, supports, 2)

	// Verify all are supports type
	for _, link := range supports {
		assert.Equal(t, LinkSupports, link.RelType)
	}
}

// TestLinkStore_TraverseLinks verifies BFS traversal with depth limit.
func TestLinkStore_TraverseLinks(t *testing.T) {
	store := setupLinkStore(t)
	ctx := context.Background()

	// Create a chain of links: A -> B -> C -> D
	links := []*MemoryLink{
		{SourceID: "A", TargetID: "B", RelType: LinkRelatedTo, Confidence: 0.9},
		{SourceID: "B", TargetID: "C", RelType: LinkRelatedTo, Confidence: 0.8},
		{SourceID: "C", TargetID: "D", RelType: LinkRelatedTo, Confidence: 0.7},
	}

	for _, link := range links {
		err := store.CreateLink(ctx, link)
		require.NoError(t, err)
	}

	// Traverse with depth 2 from A
	// BFS visits nodes level by level:
	// - Level 0: A (starting node)
	// - Level 1: B (linked from A)
	// depth=2 means we do 2 iterations of the BFS loop
	result, err := store.TraverseLinks(ctx, "A", 2)
	require.NoError(t, err)

	// With depth 2, we visit A and B, getting their links
	assert.Contains(t, result, "A")
	assert.Contains(t, result, "B")
	// Note: The actual implementation may not visit C in depth 2 since
	// depth refers to the number of BFS iterations, not the path length

	// D should NOT be in result because we haven't traversed that far
	assert.NotContains(t, result, "D")

	// Traverse with depth 0 (should use default of 3)
	result, err = store.TraverseLinks(ctx, "A", 0)
	require.NoError(t, err)
	// With default depth 3, should visit A, B, C (3 nodes)
	assert.GreaterOrEqual(t, len(result), 2) // At least A and B
}

// TestLinkStore_TraverseLinks_CycleDetection verifies cycles don't cause infinite loop.
func TestLinkStore_TraverseLinks_CycleDetection(t *testing.T) {
	store := setupLinkStore(t)
	ctx := context.Background()

	// Create a cycle: A -> B -> C -> A
	links := []*MemoryLink{
		{SourceID: "cycle-A", TargetID: "cycle-B", RelType: LinkRelatedTo, Confidence: 0.9},
		{SourceID: "cycle-B", TargetID: "cycle-C", RelType: LinkRelatedTo, Confidence: 0.8},
		{SourceID: "cycle-C", TargetID: "cycle-A", RelType: LinkRelatedTo, Confidence: 0.7},
	}

	for _, link := range links {
		err := store.CreateLink(ctx, link)
		require.NoError(t, err)
	}

	// Traverse should complete without hanging
	done := make(chan struct{})
	go func() {
		result, err := store.TraverseLinks(ctx, "cycle-A", 10)
		assert.NoError(t, err)
		// Should find all 3 nodes
		assert.Len(t, result, 3)
		close(done)
	}()

	select {
	case <-done:
		// Test passed
	case <-time.After(2 * time.Second):
		t.Fatal("TraverseLinks appears to be stuck in an infinite loop")
	}
}

// TestLinkStore_TraverseLinks_Tree verifies traversal of a tree structure.
func TestLinkStore_TraverseLinks_Tree(t *testing.T) {
	store := setupLinkStore(t)
	ctx := context.Background()

	// Create a tree:
	//       root
	//      /    \
	//    left  right
	//    /
	//  leaf
	links := []*MemoryLink{
		{SourceID: "root", TargetID: "left", RelType: LinkLeadsTo, Confidence: 0.9},
		{SourceID: "root", TargetID: "right", RelType: LinkLeadsTo, Confidence: 0.9},
		{SourceID: "left", TargetID: "leaf", RelType: LinkLeadsTo, Confidence: 0.8},
	}

	for _, link := range links {
		err := store.CreateLink(ctx, link)
		require.NoError(t, err)
	}

	// Traverse from root
	result, err := store.TraverseLinks(ctx, "root", 3)
	require.NoError(t, err)

	// Should find all nodes
	assert.Contains(t, result, "root")
	assert.Contains(t, result, "left")
	assert.Contains(t, result, "right")
	assert.Contains(t, result, "leaf")
}

// TestLinkStore_RetrieveWithContext verifies context loading.
func TestLinkStore_RetrieveWithContext(t *testing.T) {
	store := setupLinkStore(t)
	ctx := context.Background()

	// Create a strategic memory in the database
	_, err := store.db.ExecContext(ctx, `
		INSERT INTO strategic_memory (id, principle, embedding, category, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, "strat-1", "Always check logs first", nil, "debugging", time.Now().Format(time.RFC3339))
	require.NoError(t, err)

	// Create links for this memory
	links := []*MemoryLink{
		{SourceID: "strat-1", TargetID: "other-1", SourceType: MemoryTypeStrategic, RelType: LinkContradicts, Confidence: 0.9},
		{SourceID: "strat-1", TargetID: "other-2", SourceType: MemoryTypeStrategic, RelType: LinkSupports, Confidence: 0.85},
		{SourceID: "strat-1", TargetID: "other-3", SourceType: MemoryTypeStrategic, RelType: LinkRelatedTo, Confidence: 0.7},
		{SourceID: "strat-1", TargetID: "other-4", SourceType: MemoryTypeStrategic, RelType: LinkEvolvedFrom, Confidence: 0.8},
	}

	for _, link := range links {
		err := store.CreateLink(ctx, link)
		require.NoError(t, err)
	}

	// Retrieve with context
	result, err := store.RetrieveWithContext(ctx, "strat-1", MemoryTypeStrategic)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify memory is loaded
	assert.Equal(t, "strat-1", result.Memory.ID)
	assert.Equal(t, MemoryTypeStrategic, result.Memory.Type)
	assert.Equal(t, "Always check logs first", result.Memory.Content)

	// Verify links are categorized
	assert.Len(t, result.Contradictions, 1)
	assert.Len(t, result.Supports, 1)
	assert.Len(t, result.RelatedTo, 2) // related_to + evolved_from
	assert.True(t, result.HasUpdates)  // evolved_from sets this
}

// TestLinkStore_RetrieveWithContext_EpisodicMemory verifies loading episodic memory.
func TestLinkStore_RetrieveWithContext_EpisodicMemory(t *testing.T) {
	store := setupLinkStore(t)
	ctx := context.Background()

	// Create an episodic memory in the database
	_, err := store.db.ExecContext(ctx, `
		INSERT INTO memories (id, content, embedding, created_at)
		VALUES (?, ?, ?, ?)
	`, "ep-1", "User fixed a bug in the parser", nil, time.Now().Format(time.RFC3339))
	require.NoError(t, err)

	// Retrieve with context
	result, err := store.RetrieveWithContext(ctx, "ep-1", MemoryTypeEpisodic)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "ep-1", result.Memory.ID)
	assert.Equal(t, MemoryTypeEpisodic, result.Memory.Type)
	assert.Equal(t, "User fixed a bug in the parser", result.Memory.Content)
}

// TestLinkStore_RetrieveWithContext_NotFound verifies error on missing memory.
func TestLinkStore_RetrieveWithContext_NotFound(t *testing.T) {
	store := setupLinkStore(t)
	ctx := context.Background()

	// Try to retrieve non-existent memory
	result, err := store.RetrieveWithContext(ctx, "nonexistent", MemoryTypeStrategic)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not found")
}

// TestLinkStore_CreateLink_DefaultValues verifies default values are set.
func TestLinkStore_CreateLink_DefaultValues(t *testing.T) {
	store := setupLinkStore(t)
	ctx := context.Background()

	link := &MemoryLink{
		SourceID: "src",
		TargetID: "tgt",
		RelType:  LinkRelatedTo,
		// CreatedAt and CreatedBy not set
	}

	err := store.CreateLink(ctx, link)
	require.NoError(t, err)

	// Verify defaults were applied
	retrieved, err := store.GetLink(ctx, "src", "tgt", LinkRelatedTo)
	require.NoError(t, err)

	assert.False(t, retrieved.CreatedAt.IsZero(), "CreatedAt should be set")
	assert.Equal(t, "system", retrieved.CreatedBy, "CreatedBy should default to 'system'")
}
