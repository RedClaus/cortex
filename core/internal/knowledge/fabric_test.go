// Package knowledge provides integration tests for the Knowledge Fabric.
package knowledge

import (
	"context"
	"testing"
	"time"

	"github.com/normanking/cortex/pkg/types"
)

// ═══════════════════════════════════════════════════════════════════════════════
// MOCK IMPLEMENTATIONS FOR TESTING
// ═══════════════════════════════════════════════════════════════════════════════

// mockStore implements Store interface for testing.
type mockStore struct {
	items map[string]*types.KnowledgeItem
}

func newMockStore() *mockStore {
	return &mockStore{
		items: make(map[string]*types.KnowledgeItem),
	}
}

func (m *mockStore) Create(ctx context.Context, item *types.KnowledgeItem) error {
	m.items[item.ID] = item
	return nil
}

func (m *mockStore) Update(ctx context.Context, item *types.KnowledgeItem) error {
	if _, exists := m.items[item.ID]; !exists {
		return errNotFound
	}
	m.items[item.ID] = item
	return nil
}

func (m *mockStore) Delete(ctx context.Context, id string) error {
	if item, exists := m.items[id]; exists {
		now := time.Now()
		item.DeletedAt = &now
		return nil
	}
	return errNotFound
}

func (m *mockStore) GetByID(ctx context.Context, id string) (*types.KnowledgeItem, error) {
	item, exists := m.items[id]
	if !exists || item.DeletedAt != nil {
		return nil, errNotFound
	}
	return item, nil
}

func (m *mockStore) GetByScope(ctx context.Context, scope types.Scope) ([]*types.KnowledgeItem, error) {
	var results []*types.KnowledgeItem
	for _, item := range m.items {
		if item.Scope == scope && item.DeletedAt == nil {
			results = append(results, item)
		}
	}
	return results, nil
}

func (m *mockStore) SearchByTags(ctx context.Context, tags []string, scopes []types.Scope) ([]*types.KnowledgeItem, error) {
	var results []*types.KnowledgeItem
	for _, item := range m.items {
		if item.DeletedAt != nil {
			continue
		}
		if hasAllTags(item.Tags, tags) {
			if len(scopes) == 0 || containsScope(scopes, item.Scope) {
				results = append(results, item)
			}
		}
	}
	return results, nil
}

func (m *mockStore) IncrementAccessCount(ctx context.Context, id string) error {
	if item, exists := m.items[id]; exists {
		item.AccessCount++
		return nil
	}
	return errNotFound
}

func (m *mockStore) UpdateTrustScore(ctx context.Context, id string, successCount, failureCount int) error {
	if item, exists := m.items[id]; exists {
		item.SuccessCount = successCount
		item.FailureCount = failureCount
		prior := 2.0
		total := float64(successCount + failureCount)
		item.TrustScore = (float64(successCount) + prior) / (total + prior*2)
		return nil
	}
	return errNotFound
}

// mockSearcher implements Searcher interface for testing.
type mockSearcher struct {
	items []*types.KnowledgeItem
}

func newMockSearcher(items []*types.KnowledgeItem) *mockSearcher {
	return &mockSearcher{items: items}
}

func (m *mockSearcher) Search(ctx context.Context, query string, opts types.SearchOptions) ([]*ScoredItem, error) {
	var results []*ScoredItem
	for _, item := range m.items {
		if item.DeletedAt != nil {
			continue
		}
		// Simple keyword matching for tests
		if contains(item.Title, query) || contains(item.Content, query) {
			results = append(results, &ScoredItem{
				Item:      item,
				Relevance: 0.8,
			})
		}
	}
	return results, nil
}

func (m *mockSearcher) Index(ctx context.Context) error {
	return nil
}

// mockMerger implements MergeStrategy interface for testing.
type mockMerger struct{}

func (m *mockMerger) Resolve(ctx context.Context, local, remote *types.KnowledgeItem) (*types.MergeResult, error) {
	if local.TrustScore >= remote.TrustScore {
		return &types.MergeResult{
			Winner:     local,
			Resolution: "local_wins",
			Reason:     "Local has higher or equal trust",
		}, nil
	}
	return &types.MergeResult{
		Winner:     remote,
		Resolution: "remote_wins",
		Reason:     "Remote has higher trust",
	}, nil
}

// Helper functions
func hasAllTags(itemTags, searchTags []string) bool {
	tagSet := make(map[string]bool)
	for _, t := range itemTags {
		tagSet[t] = true
	}
	for _, t := range searchTags {
		if !tagSet[t] {
			return false
		}
	}
	return true
}

func containsScope(scopes []types.Scope, scope types.Scope) bool {
	for _, s := range scopes {
		if s == scope {
			return true
		}
	}
	return false
}

func contains(text, query string) bool {
	return len(query) > 0 && len(text) > 0 && (text == query || len(text) > len(query))
}

// Custom error for not found
type notFoundError struct{}

func (e notFoundError) Error() string { return "not found" }

var errNotFound = notFoundError{}

// ═══════════════════════════════════════════════════════════════════════════════
// FABRIC TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestFabricCreate(t *testing.T) {
	store := newMockStore()
	searcher := newMockSearcher(nil)
	merger := &mockMerger{}
	fabric := NewFabric(store, searcher, merger)
	ctx := context.Background()

	t.Run("creates valid item", func(t *testing.T) {
		item := &types.KnowledgeItem{
			ID:       "test-1",
			Type:     types.TypeSOP,
			Content:  "Test content",
			Scope:    types.ScopePersonal,
			AuthorID: "user-1",
			Tags:     []string{"test"},
		}

		err := fabric.Create(ctx, item)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		// Verify timestamps were set
		if item.CreatedAt.IsZero() {
			t.Error("CreatedAt should be set")
		}
		if item.UpdatedAt.IsZero() {
			t.Error("UpdatedAt should be set")
		}

		// Verify defaults
		if item.TrustScore != 0.5 {
			t.Errorf("expected default TrustScore 0.5, got %f", item.TrustScore)
		}
		if item.Confidence != 0.5 {
			t.Errorf("expected default Confidence 0.5, got %f", item.Confidence)
		}
	})

	t.Run("rejects nil item", func(t *testing.T) {
		err := fabric.Create(ctx, nil)
		if err == nil {
			t.Error("expected error for nil item")
		}
	})

	t.Run("rejects empty content", func(t *testing.T) {
		item := &types.KnowledgeItem{
			ID:       "test-empty",
			Type:     types.TypeSOP,
			Content:  "",
			Scope:    types.ScopePersonal,
			AuthorID: "user-1",
		}

		err := fabric.Create(ctx, item)
		if err == nil {
			t.Error("expected error for empty content")
		}
	})

	t.Run("rejects missing author", func(t *testing.T) {
		item := &types.KnowledgeItem{
			ID:       "test-no-author",
			Type:     types.TypeSOP,
			Content:  "Content",
			Scope:    types.ScopePersonal,
			AuthorID: "",
		}

		err := fabric.Create(ctx, item)
		if err == nil {
			t.Error("expected error for missing author")
		}
	})

	t.Run("rejects invalid scope", func(t *testing.T) {
		item := &types.KnowledgeItem{
			ID:       "test-bad-scope",
			Type:     types.TypeSOP,
			Content:  "Content",
			Scope:    "invalid",
			AuthorID: "user-1",
		}

		err := fabric.Create(ctx, item)
		if err == nil {
			t.Error("expected error for invalid scope")
		}
	})
}

func TestFabricGetByID(t *testing.T) {
	store := newMockStore()
	searcher := newMockSearcher(nil)
	merger := &mockMerger{}
	fabric := NewFabric(store, searcher, merger)
	ctx := context.Background()

	// Setup: create test item
	item := &types.KnowledgeItem{
		ID:         "get-test-1",
		Type:       types.TypeSOP,
		Content:    "Test content",
		Scope:      types.ScopePersonal,
		AuthorID:   "user-1",
		Tags:       []string{},
		TrustScore: 0.5,
		Confidence: 0.5,
	}
	store.Create(ctx, item)

	t.Run("retrieves existing item", func(t *testing.T) {
		retrieved, err := fabric.GetByID(ctx, "get-test-1")
		if err != nil {
			t.Fatalf("GetByID failed: %v", err)
		}

		if retrieved.ID != "get-test-1" {
			t.Errorf("expected ID 'get-test-1', got '%s'", retrieved.ID)
		}
	})

	t.Run("returns error for empty ID", func(t *testing.T) {
		_, err := fabric.GetByID(ctx, "")
		if err == nil {
			t.Error("expected error for empty ID")
		}
	})

	t.Run("returns error for non-existent ID", func(t *testing.T) {
		_, err := fabric.GetByID(ctx, "non-existent")
		if err == nil {
			t.Error("expected error for non-existent ID")
		}
	})
}

func TestFabricGetByScope(t *testing.T) {
	store := newMockStore()
	searcher := newMockSearcher(nil)
	merger := &mockMerger{}
	fabric := NewFabric(store, searcher, merger)
	ctx := context.Background()

	// Setup: create items in different scopes
	items := []*types.KnowledgeItem{
		{ID: "scope-1", Type: types.TypeSOP, Content: "P1", Scope: types.ScopePersonal, AuthorID: "u1", Tags: []string{}, TrustScore: 0.5, Confidence: 0.5},
		{ID: "scope-2", Type: types.TypeSOP, Content: "P2", Scope: types.ScopePersonal, AuthorID: "u1", Tags: []string{}, TrustScore: 0.5, Confidence: 0.5},
		{ID: "scope-3", Type: types.TypeSOP, Content: "T1", Scope: types.ScopeTeam, AuthorID: "u1", Tags: []string{}, TrustScore: 0.5, Confidence: 0.5},
		{ID: "scope-4", Type: types.TypeSOP, Content: "G1", Scope: types.ScopeGlobal, AuthorID: "admin", Tags: []string{}, TrustScore: 0.5, Confidence: 0.5},
	}
	for _, item := range items {
		store.Create(ctx, item)
	}

	t.Run("retrieves personal scope", func(t *testing.T) {
		results, err := fabric.GetByScope(ctx, types.ScopePersonal)
		if err != nil {
			t.Fatalf("GetByScope failed: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("expected 2 personal items, got %d", len(results))
		}
	})

	t.Run("retrieves team scope", func(t *testing.T) {
		results, err := fabric.GetByScope(ctx, types.ScopeTeam)
		if err != nil {
			t.Fatalf("GetByScope failed: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("expected 1 team item, got %d", len(results))
		}
	})

	t.Run("retrieves global scope", func(t *testing.T) {
		results, err := fabric.GetByScope(ctx, types.ScopeGlobal)
		if err != nil {
			t.Fatalf("GetByScope failed: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("expected 1 global item, got %d", len(results))
		}
	})

	t.Run("rejects invalid scope", func(t *testing.T) {
		_, err := fabric.GetByScope(ctx, "invalid")
		if err == nil {
			t.Error("expected error for invalid scope")
		}
	})
}

func TestFabricUpdate(t *testing.T) {
	store := newMockStore()
	searcher := newMockSearcher(nil)
	merger := &mockMerger{}
	fabric := NewFabric(store, searcher, merger)
	ctx := context.Background()

	// Setup
	item := &types.KnowledgeItem{
		ID:         "update-test-1",
		Type:       types.TypeSOP,
		Content:    "Original",
		Scope:      types.ScopePersonal,
		AuthorID:   "user-1",
		Tags:       []string{},
		TrustScore: 0.5,
		Confidence: 0.5,
		SyncStatus: "synced",
		Version:    1,
	}
	store.Create(ctx, item)

	t.Run("updates item successfully", func(t *testing.T) {
		item.Content = "Updated content"

		err := fabric.Update(ctx, item)
		if err != nil {
			t.Fatalf("Update failed: %v", err)
		}

		// Verify version incremented
		if item.Version != 2 {
			t.Errorf("expected version 2, got %d", item.Version)
		}

		// Verify sync status changed
		if item.SyncStatus != "pending" {
			t.Errorf("expected sync_status 'pending', got '%s'", item.SyncStatus)
		}
	})

	t.Run("rejects nil item", func(t *testing.T) {
		err := fabric.Update(ctx, nil)
		if err == nil {
			t.Error("expected error for nil item")
		}
	})

	t.Run("rejects empty content", func(t *testing.T) {
		item.Content = ""
		err := fabric.Update(ctx, item)
		if err == nil {
			t.Error("expected error for empty content")
		}
	})
}

func TestFabricDelete(t *testing.T) {
	store := newMockStore()
	searcher := newMockSearcher(nil)
	merger := &mockMerger{}
	fabric := NewFabric(store, searcher, merger)
	ctx := context.Background()

	t.Run("deletes personal item", func(t *testing.T) {
		item := &types.KnowledgeItem{
			ID:         "delete-test-1",
			Type:       types.TypeSOP,
			Content:    "Delete me",
			Scope:      types.ScopePersonal,
			AuthorID:   "user-1",
			Tags:       []string{},
			TrustScore: 0.5,
			Confidence: 0.5,
		}
		store.Create(ctx, item)

		err := fabric.Delete(ctx, "delete-test-1")
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		// Verify item is soft-deleted
		_, err = fabric.GetByID(ctx, "delete-test-1")
		if err == nil {
			t.Error("expected error when retrieving deleted item")
		}
	})

	t.Run("prevents deletion of global items", func(t *testing.T) {
		globalItem := &types.KnowledgeItem{
			ID:         "delete-global-1",
			Type:       types.TypeSOP,
			Content:    "Global policy",
			Scope:      types.ScopeGlobal,
			AuthorID:   "admin",
			Tags:       []string{},
			TrustScore: 0.5,
			Confidence: 0.5,
		}
		store.Create(ctx, globalItem)

		err := fabric.Delete(ctx, "delete-global-1")
		if err == nil {
			t.Error("expected error when deleting global item")
		}
	})

	t.Run("returns error for empty ID", func(t *testing.T) {
		err := fabric.Delete(ctx, "")
		if err == nil {
			t.Error("expected error for empty ID")
		}
	})
}

func TestFabricRecordSuccess(t *testing.T) {
	store := newMockStore()
	searcher := newMockSearcher(nil)
	merger := &mockMerger{}
	fabric := NewFabric(store, searcher, merger)
	ctx := context.Background()

	// Setup
	item := &types.KnowledgeItem{
		ID:           "success-test-1",
		Type:         types.TypeSOP,
		Content:      "Test",
		Scope:        types.ScopePersonal,
		AuthorID:     "user-1",
		Tags:         []string{},
		TrustScore:   0.5,
		Confidence:   0.5,
		SuccessCount: 0,
		FailureCount: 0,
	}
	store.Create(ctx, item)

	t.Run("increments success and updates trust", func(t *testing.T) {
		err := fabric.RecordSuccess(ctx, "success-test-1")
		if err != nil {
			t.Fatalf("RecordSuccess failed: %v", err)
		}

		updated, _ := store.GetByID(ctx, "success-test-1")
		if updated.SuccessCount != 1 {
			t.Errorf("expected SuccessCount 1, got %d", updated.SuccessCount)
		}

		// Trust should increase with success
		// (1+2)/(1+4) = 3/5 = 0.6
		expectedTrust := 0.6
		if updated.TrustScore < expectedTrust-0.01 || updated.TrustScore > expectedTrust+0.01 {
			t.Errorf("expected TrustScore ~%f, got %f", expectedTrust, updated.TrustScore)
		}
	})
}

func TestFabricRecordFailure(t *testing.T) {
	store := newMockStore()
	searcher := newMockSearcher(nil)
	merger := &mockMerger{}
	fabric := NewFabric(store, searcher, merger)
	ctx := context.Background()

	// Setup
	item := &types.KnowledgeItem{
		ID:           "failure-test-1",
		Type:         types.TypeSOP,
		Content:      "Test",
		Scope:        types.ScopePersonal,
		AuthorID:     "user-1",
		Tags:         []string{},
		TrustScore:   0.5,
		Confidence:   0.5,
		SuccessCount: 5,
		FailureCount: 0,
	}
	store.Create(ctx, item)

	t.Run("increments failure and updates trust", func(t *testing.T) {
		err := fabric.RecordFailure(ctx, "failure-test-1")
		if err != nil {
			t.Fatalf("RecordFailure failed: %v", err)
		}

		updated, _ := store.GetByID(ctx, "failure-test-1")
		if updated.FailureCount != 1 {
			t.Errorf("expected FailureCount 1, got %d", updated.FailureCount)
		}

		// Trust should decrease with failure
		// (5+2)/(6+4) = 7/10 = 0.7
		expectedTrust := 0.7
		if updated.TrustScore < expectedTrust-0.01 || updated.TrustScore > expectedTrust+0.01 {
			t.Errorf("expected TrustScore ~%f, got %f", expectedTrust, updated.TrustScore)
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// RETRIEVAL TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestFilterByConfidence(t *testing.T) {
	items := []*types.KnowledgeItem{
		{ID: "1", Confidence: 0.9},
		{ID: "2", Confidence: 0.7},
		{ID: "3", Confidence: 0.5},
		{ID: "4", Confidence: 0.85},
	}

	t.Run("filters by threshold", func(t *testing.T) {
		filtered := filterByConfidence(items, 0.8)
		if len(filtered) != 2 {
			t.Errorf("expected 2 items with confidence >= 0.8, got %d", len(filtered))
		}
	})

	t.Run("returns empty for high threshold", func(t *testing.T) {
		filtered := filterByConfidence(items, 0.95)
		if len(filtered) != 0 {
			t.Errorf("expected 0 items with confidence >= 0.95, got %d", len(filtered))
		}
	})
}

func TestCalculateAverageConfidence(t *testing.T) {
	t.Run("calculates average", func(t *testing.T) {
		items := []*types.KnowledgeItem{
			{Confidence: 0.8},
			{Confidence: 0.6},
			{Confidence: 1.0},
		}

		avg := calculateAverageConfidence(items)
		expected := 0.8 // (0.8 + 0.6 + 1.0) / 3
		// Use epsilon comparison for floating point
		epsilon := 0.0001
		if avg < expected-epsilon || avg > expected+epsilon {
			t.Errorf("expected average ~%f, got %f", expected, avg)
		}
	})

	t.Run("returns 0 for empty slice", func(t *testing.T) {
		avg := calculateAverageConfidence([]*types.KnowledgeItem{})
		if avg != 0 {
			t.Errorf("expected 0 for empty slice, got %f", avg)
		}
	})
}

func TestScopePriority(t *testing.T) {
	tests := []struct {
		scope    types.Scope
		expected int
	}{
		{types.ScopePersonal, 3},
		{types.ScopeTeam, 2},
		{types.ScopeGlobal, 1},
		{"unknown", 0},
	}

	for _, tc := range tests {
		t.Run(string(tc.scope), func(t *testing.T) {
			priority := scopePriority(tc.scope)
			if priority != tc.expected {
				t.Errorf("expected priority %d for %s, got %d", tc.expected, tc.scope, priority)
			}
		})
	}
}

func TestHashContent(t *testing.T) {
	t.Run("normalizes whitespace", func(t *testing.T) {
		content := "hello   world\n\ttab"
		hash := hashContent(content)
		expected := "hello world tab"
		if hash != expected {
			t.Errorf("expected '%s', got '%s'", expected, hash)
		}
	})

	t.Run("truncates long content", func(t *testing.T) {
		content := "this is a very long piece of content that should be truncated at 50 characters"
		hash := hashContent(content)
		if len(hash) != 50 {
			t.Errorf("expected hash length 50, got %d", len(hash))
		}
	})
}

func TestMin(t *testing.T) {
	tests := []struct {
		a, b, expected int
	}{
		{1, 2, 1},
		{5, 3, 3},
		{4, 4, 4},
		{0, 10, 0},
	}

	for _, tc := range tests {
		result := min(tc.a, tc.b)
		if result != tc.expected {
			t.Errorf("min(%d, %d) = %d, expected %d", tc.a, tc.b, result, tc.expected)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// MERGE STRATEGY TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestTrustWeightedMerge(t *testing.T) {
	merger := NewTrustWeightedMerge()
	ctx := context.Background()

	t.Run("global scope - remote wins", func(t *testing.T) {
		local := &types.KnowledgeItem{
			ID:         "merge-1",
			Scope:      types.ScopeGlobal,
			TrustScore: 0.9,
		}
		remote := &types.KnowledgeItem{
			ID:         "merge-1",
			Scope:      types.ScopeGlobal,
			TrustScore: 0.5,
		}

		result, err := merger.Resolve(ctx, local, remote)
		if err != nil {
			t.Fatalf("Resolve failed: %v", err)
		}

		if result.Resolution != "remote_wins" {
			t.Errorf("expected 'remote_wins' for global scope, got '%s'", result.Resolution)
		}
	})

	t.Run("personal scope - local wins", func(t *testing.T) {
		local := &types.KnowledgeItem{
			ID:         "merge-2",
			Scope:      types.ScopePersonal,
			TrustScore: 0.5,
		}
		remote := &types.KnowledgeItem{
			ID:         "merge-2",
			Scope:      types.ScopePersonal,
			TrustScore: 0.9,
		}

		result, err := merger.Resolve(ctx, local, remote)
		if err != nil {
			t.Fatalf("Resolve failed: %v", err)
		}

		if result.Resolution != "local_wins" {
			t.Errorf("expected 'local_wins' for personal scope, got '%s'", result.Resolution)
		}
	})

	t.Run("team scope - higher trust wins", func(t *testing.T) {
		local := &types.KnowledgeItem{
			ID:         "merge-3",
			Scope:      types.ScopeTeam,
			TrustScore: 0.85,
			UpdatedAt:  time.Now(),
		}
		remote := &types.KnowledgeItem{
			ID:         "merge-3",
			Scope:      types.ScopeTeam,
			TrustScore: 0.6,
			UpdatedAt:  time.Now(),
		}

		result, err := merger.Resolve(ctx, local, remote)
		if err != nil {
			t.Fatalf("Resolve failed: %v", err)
		}

		if result.Resolution != "local_wins" {
			t.Errorf("expected 'local_wins' for higher trust, got '%s'", result.Resolution)
		}
	})
}

func TestIsContentDifferent(t *testing.T) {
	t.Run("same content", func(t *testing.T) {
		local := &types.KnowledgeItem{
			Title:   "Test",
			Content: "Content",
			Tags:    []string{"a", "b"},
		}
		remote := &types.KnowledgeItem{
			Title:   "Test",
			Content: "Content",
			Tags:    []string{"b", "a"}, // Same tags, different order
		}

		if IsContentDifferent(local, remote) {
			t.Error("expected content to be same")
		}
	})

	t.Run("different title", func(t *testing.T) {
		local := &types.KnowledgeItem{Title: "Title A", Content: "Same", Tags: []string{}}
		remote := &types.KnowledgeItem{Title: "Title B", Content: "Same", Tags: []string{}}

		if !IsContentDifferent(local, remote) {
			t.Error("expected content to be different (title)")
		}
	})

	t.Run("different tags", func(t *testing.T) {
		local := &types.KnowledgeItem{Title: "Same", Content: "Same", Tags: []string{"a"}}
		remote := &types.KnowledgeItem{Title: "Same", Content: "Same", Tags: []string{"b"}}

		if !IsContentDifferent(local, remote) {
			t.Error("expected content to be different (tags)")
		}
	})
}
