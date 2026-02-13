// Package data provides tests for Store operations.
package data

import (
	"context"
	"testing"
	"time"

	"github.com/normanking/cortex/pkg/types"
)

// ═══════════════════════════════════════════════════════════════════════════════
// KNOWLEDGE CRUD TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestCreateKnowledge(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	t.Run("creates item successfully", func(t *testing.T) {
		item := &types.KnowledgeItem{
			ID:       "test-create-1",
			Type:     types.TypeSOP,
			Title:    "Test SOP",
			Content:  "Test content",
			Tags:     []string{"cisco", "network"},
			Scope:    types.ScopePersonal,
			AuthorID: "user-1",
		}

		err := store.CreateKnowledge(ctx, item)
		if err != nil {
			t.Fatalf("CreateKnowledge failed: %v", err)
		}

		// Verify item exists
		retrieved, err := store.GetKnowledge(ctx, "test-create-1")
		if err != nil {
			t.Fatalf("GetKnowledge failed: %v", err)
		}

		if retrieved.Title != "Test SOP" {
			t.Errorf("expected title 'Test SOP', got '%s'", retrieved.Title)
		}
		if len(retrieved.Tags) != 2 {
			t.Errorf("expected 2 tags, got %d", len(retrieved.Tags))
		}
	})

	t.Run("rejects empty ID", func(t *testing.T) {
		item := &types.KnowledgeItem{
			ID:      "",
			Type:    types.TypeSOP,
			Content: "Test",
		}

		err := store.CreateKnowledge(ctx, item)
		if err == nil {
			t.Error("expected error for empty ID")
		}
	})

	t.Run("rejects duplicate ID", func(t *testing.T) {
		item := &types.KnowledgeItem{
			ID:       "test-create-dup",
			Type:     types.TypeSOP,
			Content:  "First",
			Scope:    types.ScopePersonal,
			AuthorID: "user-1",
			Tags:     []string{},
		}

		store.CreateKnowledge(ctx, item)

		// Try to create duplicate
		item2 := &types.KnowledgeItem{
			ID:       "test-create-dup",
			Type:     types.TypeSOP,
			Content:  "Second",
			Scope:    types.ScopePersonal,
			AuthorID: "user-1",
			Tags:     []string{},
		}

		err := store.CreateKnowledge(ctx, item2)
		if err == nil {
			t.Error("expected error for duplicate ID")
		}
	})
}

func TestGetKnowledge(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	// Setup: create test item
	item := &types.KnowledgeItem{
		ID:         "test-get-1",
		Type:       types.TypeLesson,
		Title:      "Test Lesson",
		Content:    "Lesson content",
		Tags:       []string{"learning"},
		Scope:      types.ScopeTeam,
		TeamID:     "team-123",
		AuthorID:   "user-1",
		AuthorName: "Test User",
		Confidence: 0.8,
		TrustScore: 0.75,
	}
	store.CreateKnowledge(ctx, item)

	t.Run("retrieves existing item", func(t *testing.T) {
		retrieved, err := store.GetKnowledge(ctx, "test-get-1")
		if err != nil {
			t.Fatalf("GetKnowledge failed: %v", err)
		}

		if retrieved.Type != types.TypeLesson {
			t.Errorf("expected type Lesson, got %s", retrieved.Type)
		}
		if retrieved.TeamID != "team-123" {
			t.Errorf("expected team_id 'team-123', got '%s'", retrieved.TeamID)
		}
		if retrieved.Confidence != 0.8 {
			t.Errorf("expected confidence 0.8, got %f", retrieved.Confidence)
		}
	})

	t.Run("returns error for non-existent ID", func(t *testing.T) {
		_, err := store.GetKnowledge(ctx, "non-existent-id")
		if err == nil {
			t.Error("expected error for non-existent ID")
		}
	})

	t.Run("excludes soft-deleted items", func(t *testing.T) {
		// Create and delete an item
		toDelete := &types.KnowledgeItem{
			ID:       "test-get-deleted",
			Type:     types.TypeSOP,
			Content:  "Will be deleted",
			Scope:    types.ScopePersonal,
			AuthorID: "user-1",
			Tags:     []string{},
		}
		store.CreateKnowledge(ctx, toDelete)
		store.DeleteKnowledge(ctx, "test-get-deleted")

		// Try to retrieve
		_, err := store.GetKnowledge(ctx, "test-get-deleted")
		if err == nil {
			t.Error("expected error for soft-deleted item")
		}
	})
}

func TestUpdateKnowledge(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	// Setup: create test item
	item := &types.KnowledgeItem{
		ID:       "test-update-1",
		Type:     types.TypeSOP,
		Title:    "Original Title",
		Content:  "Original content",
		Tags:     []string{"original"},
		Scope:    types.ScopePersonal,
		AuthorID: "user-1",
	}
	store.CreateKnowledge(ctx, item)

	t.Run("updates item successfully", func(t *testing.T) {
		item.Title = "Updated Title"
		item.Content = "Updated content"
		item.Tags = []string{"updated", "modified"}

		err := store.UpdateKnowledge(ctx, item)
		if err != nil {
			t.Fatalf("UpdateKnowledge failed: %v", err)
		}

		// Verify changes
		retrieved, _ := store.GetKnowledge(ctx, "test-update-1")
		if retrieved.Title != "Updated Title" {
			t.Errorf("expected title 'Updated Title', got '%s'", retrieved.Title)
		}
		if len(retrieved.Tags) != 2 {
			t.Errorf("expected 2 tags, got %d", len(retrieved.Tags))
		}
	})

	t.Run("returns error for non-existent ID", func(t *testing.T) {
		fake := &types.KnowledgeItem{
			ID:      "non-existent",
			Type:    types.TypeSOP,
			Content: "test",
		}

		err := store.UpdateKnowledge(ctx, fake)
		if err == nil {
			t.Error("expected error for non-existent ID")
		}
	})
}

func TestDeleteKnowledge(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	t.Run("soft deletes item", func(t *testing.T) {
		item := &types.KnowledgeItem{
			ID:       "test-delete-1",
			Type:     types.TypeSOP,
			Content:  "Will be deleted",
			Scope:    types.ScopePersonal,
			AuthorID: "user-1",
			Tags:     []string{},
		}
		store.CreateKnowledge(ctx, item)

		err := store.DeleteKnowledge(ctx, "test-delete-1")
		if err != nil {
			t.Fatalf("DeleteKnowledge failed: %v", err)
		}

		// Verify item is not retrievable
		_, err = store.GetKnowledge(ctx, "test-delete-1")
		if err == nil {
			t.Error("expected error when retrieving deleted item")
		}

		// Verify item still exists in DB with deleted_at set
		var deletedAt *time.Time
		store.db.QueryRow(`
			SELECT deleted_at FROM knowledge_items WHERE id = 'test-delete-1'
		`).Scan(&deletedAt)

		if deletedAt == nil {
			t.Error("expected deleted_at to be set")
		}
	})

	t.Run("returns error for non-existent ID", func(t *testing.T) {
		err := store.DeleteKnowledge(ctx, "non-existent")
		if err == nil {
			t.Error("expected error for non-existent ID")
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// INTERFACE METHOD TESTS (knowledge.Store compatibility)
// ═══════════════════════════════════════════════════════════════════════════════

func TestGetByScope(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	// Setup: create items in different scopes
	items := []*types.KnowledgeItem{
		{ID: "scope-personal-1", Type: types.TypeSOP, Content: "Personal 1", Scope: types.ScopePersonal, AuthorID: "user-1", Tags: []string{}},
		{ID: "scope-personal-2", Type: types.TypeSOP, Content: "Personal 2", Scope: types.ScopePersonal, AuthorID: "user-1", Tags: []string{}},
		{ID: "scope-team-1", Type: types.TypeSOP, Content: "Team 1", Scope: types.ScopeTeam, AuthorID: "user-1", Tags: []string{}},
		{ID: "scope-global-1", Type: types.TypeSOP, Content: "Global 1", Scope: types.ScopeGlobal, AuthorID: "admin", Tags: []string{}},
	}
	for _, item := range items {
		store.CreateKnowledge(ctx, item)
	}

	t.Run("retrieves personal scope items", func(t *testing.T) {
		results, err := store.GetByScope(ctx, types.ScopePersonal)
		if err != nil {
			t.Fatalf("GetByScope failed: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("expected 2 personal items, got %d", len(results))
		}
	})

	t.Run("retrieves team scope items", func(t *testing.T) {
		results, err := store.GetByScope(ctx, types.ScopeTeam)
		if err != nil {
			t.Fatalf("GetByScope failed: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("expected 1 team item, got %d", len(results))
		}
	})

	t.Run("retrieves global scope items", func(t *testing.T) {
		results, err := store.GetByScope(ctx, types.ScopeGlobal)
		if err != nil {
			t.Fatalf("GetByScope failed: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("expected 1 global item, got %d", len(results))
		}
	})
}

func TestSearchByTags(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	// Setup: create items with various tags
	items := []*types.KnowledgeItem{
		{ID: "tag-1", Type: types.TypeSOP, Content: "Cisco SOP", Tags: []string{"cisco", "network", "layer2"}, Scope: types.ScopeTeam, AuthorID: "user-1"},
		{ID: "tag-2", Type: types.TypeSOP, Content: "Cisco Config", Tags: []string{"cisco", "config"}, Scope: types.ScopeTeam, AuthorID: "user-1"},
		{ID: "tag-3", Type: types.TypeSOP, Content: "Juniper SOP", Tags: []string{"juniper", "network"}, Scope: types.ScopePersonal, AuthorID: "user-1"},
		{ID: "tag-4", Type: types.TypeSOP, Content: "General", Tags: []string{"general"}, Scope: types.ScopeGlobal, AuthorID: "admin"},
	}
	for _, item := range items {
		store.CreateKnowledge(ctx, item)
	}

	t.Run("finds items with single tag", func(t *testing.T) {
		results, err := store.SearchByTags(ctx, []string{"cisco"}, nil)
		if err != nil {
			t.Fatalf("SearchByTags failed: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("expected 2 items with 'cisco' tag, got %d", len(results))
		}
	})

	t.Run("finds items with multiple tags (AND logic)", func(t *testing.T) {
		results, err := store.SearchByTags(ctx, []string{"cisco", "network"}, nil)
		if err != nil {
			t.Fatalf("SearchByTags failed: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("expected 1 item with both 'cisco' AND 'network' tags, got %d", len(results))
		}
	})

	t.Run("filters by scope", func(t *testing.T) {
		results, err := store.SearchByTags(ctx, []string{"network"}, []types.Scope{types.ScopeTeam})
		if err != nil {
			t.Fatalf("SearchByTags failed: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("expected 1 team item with 'network' tag, got %d", len(results))
		}
	})

	t.Run("returns empty for non-matching tags", func(t *testing.T) {
		results, err := store.SearchByTags(ctx, []string{"nonexistent"}, nil)
		if err != nil {
			t.Fatalf("SearchByTags failed: %v", err)
		}

		if len(results) != 0 {
			t.Errorf("expected 0 items, got %d", len(results))
		}
	})

	t.Run("requires at least one tag", func(t *testing.T) {
		_, err := store.SearchByTags(ctx, []string{}, nil)
		if err == nil {
			t.Error("expected error for empty tags")
		}
	})
}

func TestIncrementAccessCount(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	// Setup
	item := &types.KnowledgeItem{
		ID:          "access-test-1",
		Type:        types.TypeSOP,
		Content:     "Test",
		Scope:       types.ScopePersonal,
		AuthorID:    "user-1",
		Tags:        []string{},
		AccessCount: 0,
	}
	store.CreateKnowledge(ctx, item)

	t.Run("increments access count", func(t *testing.T) {
		err := store.IncrementAccessCount(ctx, "access-test-1")
		if err != nil {
			t.Fatalf("IncrementAccessCount failed: %v", err)
		}

		// Check count
		var count int
		store.db.QueryRow("SELECT access_count FROM knowledge_items WHERE id = 'access-test-1'").Scan(&count)
		if count != 1 {
			t.Errorf("expected access_count 1, got %d", count)
		}

		// Increment again
		store.IncrementAccessCount(ctx, "access-test-1")
		store.db.QueryRow("SELECT access_count FROM knowledge_items WHERE id = 'access-test-1'").Scan(&count)
		if count != 2 {
			t.Errorf("expected access_count 2, got %d", count)
		}
	})

	t.Run("updates last_accessed_at", func(t *testing.T) {
		before := time.Now().Add(-1 * time.Second)

		store.IncrementAccessCount(ctx, "access-test-1")

		var lastAccessed time.Time
		store.db.QueryRow("SELECT last_accessed_at FROM knowledge_items WHERE id = 'access-test-1'").Scan(&lastAccessed)

		if lastAccessed.Before(before) {
			t.Error("last_accessed_at was not updated")
		}
	})

	t.Run("returns error for non-existent ID", func(t *testing.T) {
		err := store.IncrementAccessCount(ctx, "non-existent")
		if err == nil {
			t.Error("expected error for non-existent ID")
		}
	})
}

func TestUpdateTrustScore(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	// Setup
	item := &types.KnowledgeItem{
		ID:           "trust-test-1",
		Type:         types.TypeSOP,
		Content:      "Test",
		Scope:        types.ScopePersonal,
		AuthorID:     "user-1",
		Tags:         []string{},
		TrustScore:   0.5,
		SuccessCount: 0,
		FailureCount: 0,
	}
	store.CreateKnowledge(ctx, item)

	t.Run("updates trust score with Bayesian formula", func(t *testing.T) {
		// 10 successes, 0 failures should give high trust
		err := store.UpdateTrustScore(ctx, "trust-test-1", 10, 0)
		if err != nil {
			t.Fatalf("UpdateTrustScore failed: %v", err)
		}

		var trustScore float64
		store.db.QueryRow("SELECT trust_score FROM knowledge_items WHERE id = 'trust-test-1'").Scan(&trustScore)

		// With prior=2: (10+2)/(10+4) = 12/14 ≈ 0.857
		if trustScore < 0.8 || trustScore > 0.9 {
			t.Errorf("expected trust_score ~0.857, got %f", trustScore)
		}
	})

	t.Run("updates counts correctly", func(t *testing.T) {
		store.UpdateTrustScore(ctx, "trust-test-1", 5, 3)

		var successCount, failureCount int
		store.db.QueryRow("SELECT success_count, failure_count FROM knowledge_items WHERE id = 'trust-test-1'").Scan(&successCount, &failureCount)

		if successCount != 5 {
			t.Errorf("expected success_count 5, got %d", successCount)
		}
		if failureCount != 3 {
			t.Errorf("expected failure_count 3, got %d", failureCount)
		}
	})

	t.Run("handles zero counts (new items)", func(t *testing.T) {
		newItem := &types.KnowledgeItem{
			ID:       "trust-test-new",
			Type:     types.TypeSOP,
			Content:  "New",
			Scope:    types.ScopePersonal,
			AuthorID: "user-1",
			Tags:     []string{},
		}
		store.CreateKnowledge(ctx, newItem)

		err := store.UpdateTrustScore(ctx, "trust-test-new", 0, 0)
		if err != nil {
			t.Fatalf("UpdateTrustScore failed: %v", err)
		}

		var trustScore float64
		store.db.QueryRow("SELECT trust_score FROM knowledge_items WHERE id = 'trust-test-new'").Scan(&trustScore)

		// With prior=2: (0+2)/(0+4) = 0.5 (neutral)
		if trustScore != 0.5 {
			t.Errorf("expected trust_score 0.5 for new item, got %f", trustScore)
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// FTS5 SEARCH TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestSearchKnowledgeFTS(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	// Setup: create searchable items
	items := []*types.KnowledgeItem{
		{ID: "fts-1", Type: types.TypeSOP, Title: "Cisco VLAN Configuration", Content: "Configure VLANs on Cisco switches", Tags: []string{"cisco"}, Scope: types.ScopeTeam, AuthorID: "user-1", TrustScore: 0.9},
		{ID: "fts-2", Type: types.TypeSOP, Title: "Juniper Network Setup", Content: "Set up Juniper routers", Tags: []string{"juniper"}, Scope: types.ScopeTeam, AuthorID: "user-1", TrustScore: 0.8},
		{ID: "fts-3", Type: types.TypeLesson, Title: "Network Troubleshooting", Content: "Debug network issues on Cisco and Juniper", Tags: []string{"network"}, Scope: types.ScopePersonal, AuthorID: "user-1", TrustScore: 0.7},
	}
	for _, item := range items {
		store.CreateKnowledge(ctx, item)
	}

	// Wait for FTS index to update (triggers are async)
	time.Sleep(100 * time.Millisecond)

	t.Run("finds items by content", func(t *testing.T) {
		results, err := store.SearchKnowledgeFTS(ctx, "cisco", 10)
		if err != nil {
			t.Fatalf("SearchKnowledgeFTS failed: %v", err)
		}

		if len(results) < 1 {
			t.Errorf("expected at least 1 result for 'cisco', got %d", len(results))
		}
	})

	t.Run("finds items by title", func(t *testing.T) {
		results, err := store.SearchKnowledgeFTS(ctx, "VLAN Configuration", 10)
		if err != nil {
			t.Fatalf("SearchKnowledgeFTS failed: %v", err)
		}

		if len(results) < 1 {
			t.Errorf("expected at least 1 result for 'VLAN Configuration', got %d", len(results))
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		results, err := store.SearchKnowledgeFTS(ctx, "network", 1)
		if err != nil {
			t.Fatalf("SearchKnowledgeFTS failed: %v", err)
		}

		if len(results) > 1 {
			t.Errorf("expected max 1 result, got %d", len(results))
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// SESSION TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestSessionOperations(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	t.Run("creates and retrieves session", func(t *testing.T) {
		session := &types.Session{
			ID:             "session-1",
			UserID:         "user-1",
			Title:          "Test Session",
			CWD:            "/home/user",
			Status:         "active",
			StartedAt:      time.Now(),
			LastActivityAt: time.Now(),
		}

		err := store.CreateSession(ctx, session)
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}

		retrieved, err := store.GetSession(ctx, "session-1")
		if err != nil {
			t.Fatalf("GetSession failed: %v", err)
		}

		if retrieved.Title != "Test Session" {
			t.Errorf("expected title 'Test Session', got '%s'", retrieved.Title)
		}
	})

	t.Run("adds and retrieves messages", func(t *testing.T) {
		// Create session first
		session := &types.Session{
			ID:             "session-msg-1",
			UserID:         "user-1",
			Status:         "active",
			StartedAt:      time.Now(),
			LastActivityAt: time.Now(),
		}
		store.CreateSession(ctx, session)

		// Add messages
		msg1 := &types.SessionMessage{
			SessionID: "session-msg-1",
			Role:      "user",
			Content:   "Hello",
			CreatedAt: time.Now(),
		}
		msg2 := &types.SessionMessage{
			SessionID: "session-msg-1",
			Role:      "assistant",
			Content:   "Hi there!",
			CreatedAt: time.Now().Add(1 * time.Second),
		}

		store.AddMessage(ctx, msg1)
		store.AddMessage(ctx, msg2)

		messages, err := store.GetSessionMessages(ctx, "session-msg-1")
		if err != nil {
			t.Fatalf("GetSessionMessages failed: %v", err)
		}

		if len(messages) != 2 {
			t.Errorf("expected 2 messages, got %d", len(messages))
		}
	})

	t.Run("updates session status", func(t *testing.T) {
		session := &types.Session{
			ID:             "session-status-1",
			UserID:         "user-1",
			Status:         "active",
			StartedAt:      time.Now(),
			LastActivityAt: time.Now(),
		}
		store.CreateSession(ctx, session)

		err := store.UpdateSessionStatus(ctx, "session-status-1", "completed")
		if err != nil {
			t.Fatalf("UpdateSessionStatus failed: %v", err)
		}

		updated, _ := store.GetSession(ctx, "session-status-1")
		if updated.Status != "completed" {
			t.Errorf("expected status 'completed', got '%s'", updated.Status)
		}
		if updated.EndedAt == nil {
			t.Error("expected ended_at to be set for completed session")
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// TRUST PROFILE TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestTrustProfileOperations(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	t.Run("returns default profile for new user", func(t *testing.T) {
		profile, err := store.GetTrustProfile(ctx, "new-user", "network")
		if err != nil {
			t.Fatalf("GetTrustProfile failed: %v", err)
		}

		if profile.Score != 0.5 {
			t.Errorf("expected default score 0.5, got %f", profile.Score)
		}
		if profile.SuccessCount != 0 || profile.FailureCount != 0 {
			t.Error("expected zero counts for new profile")
		}
	})

	t.Run("updates user trust score", func(t *testing.T) {
		// Success
		err := store.UpdateUserTrustScore(ctx, "trust-user-1", "network", true)
		if err != nil {
			t.Fatalf("UpdateUserTrustScore failed: %v", err)
		}

		profile, _ := store.GetTrustProfile(ctx, "trust-user-1", "network")
		if profile.SuccessCount != 1 {
			t.Errorf("expected success_count 1, got %d", profile.SuccessCount)
		}

		// Failure
		store.UpdateUserTrustScore(ctx, "trust-user-1", "network", false)
		profile, _ = store.GetTrustProfile(ctx, "trust-user-1", "network")
		if profile.FailureCount != 1 {
			t.Errorf("expected failure_count 1, got %d", profile.FailureCount)
		}
	})

	t.Run("separates domains", func(t *testing.T) {
		store.UpdateUserTrustScore(ctx, "domain-user", "network", true)
		store.UpdateUserTrustScore(ctx, "domain-user", "security", false)

		networkProfile, _ := store.GetTrustProfile(ctx, "domain-user", "network")
		securityProfile, _ := store.GetTrustProfile(ctx, "domain-user", "security")

		if networkProfile.SuccessCount != 1 {
			t.Error("network profile should have 1 success")
		}
		if securityProfile.FailureCount != 1 {
			t.Error("security profile should have 1 failure")
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// PERFORMANCE BENCHMARKS
// ═══════════════════════════════════════════════════════════════════════════════

func BenchmarkCreateKnowledge(b *testing.B) {
	tmpDir := b.TempDir()
	store, _ := NewDB(tmpDir)
	defer store.Close()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		item := &types.KnowledgeItem{
			ID:       "bench-" + string(rune(i)),
			Type:     types.TypeSOP,
			Content:  "Benchmark content",
			Scope:    types.ScopePersonal,
			AuthorID: "user-1",
			Tags:     []string{"benchmark"},
		}
		store.CreateKnowledge(ctx, item)
	}
}

func BenchmarkGetKnowledge(b *testing.B) {
	tmpDir := b.TempDir()
	store, _ := NewDB(tmpDir)
	defer store.Close()
	ctx := context.Background()

	// Setup
	item := &types.KnowledgeItem{
		ID:       "bench-get",
		Type:     types.TypeSOP,
		Content:  "Benchmark content",
		Scope:    types.ScopePersonal,
		AuthorID: "user-1",
		Tags:     []string{"benchmark"},
	}
	store.CreateKnowledge(ctx, item)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.GetKnowledge(ctx, "bench-get")
	}
}

func BenchmarkSearchKnowledgeFTS(b *testing.B) {
	tmpDir := b.TempDir()
	store, _ := NewDB(tmpDir)
	defer store.Close()
	ctx := context.Background()

	// Setup: create 100 searchable items
	for i := 0; i < 100; i++ {
		item := &types.KnowledgeItem{
			ID:       "bench-fts-" + string(rune(i)),
			Type:     types.TypeSOP,
			Title:    "Cisco VLAN Configuration Guide",
			Content:  "This document describes how to configure VLANs on Cisco switches for network segmentation.",
			Scope:    types.ScopeTeam,
			AuthorID: "user-1",
			Tags:     []string{"cisco", "vlan", "network"},
		}
		store.CreateKnowledge(ctx, item)
	}

	time.Sleep(100 * time.Millisecond) // Let FTS index update

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.SearchKnowledgeFTS(ctx, "cisco vlan", 10)
	}
}
