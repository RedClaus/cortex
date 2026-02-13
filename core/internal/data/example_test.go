// Package data_test demonstrates usage of the Cortex data layer.
package data_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/normanking/cortex/internal/data"
	"github.com/normanking/cortex/pkg/types"
)

// ExampleNewDB demonstrates the basic data layer API.
func ExampleNewDB() {
	// 1. Initialize database
	homeDir, _ := os.UserHomeDir()
	dataDir := filepath.Join(homeDir, ".cortex-test")
	defer os.RemoveAll(dataDir) // Cleanup

	store, err := data.NewDB(dataDir)
	if err != nil {
		panic(err)
	}
	defer store.Close()

	ctx := context.Background()

	// 2. Create a knowledge item
	item := &types.KnowledgeItem{
		ID:         "lesson-001",
		Type:       types.TypeLesson,
		Title:      "Always check disk space before deploying",
		Content:    "When: Deploying to production\nDo: Run 'df -h' first\nAvoid: Assuming space is available\nBecause: Out of disk causes silent failures",
		Tags:       []string{"deployment", "linux", "troubleshooting"},
		Scope:      types.ScopePersonal,
		AuthorID:   "user-123",
		AuthorName: "Alice",
		Confidence: 0.85,
		TrustScore: 0.75,
		SyncStatus: "pending",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := store.CreateKnowledge(ctx, item); err != nil {
		panic(err)
	}

	// 3. Retrieve the item
	retrieved, err := store.GetKnowledge(ctx, "lesson-001")
	if err != nil {
		panic(err)
	}
	_ = retrieved

	// 4. Search for items
	results, err := store.ListKnowledge(ctx, types.SearchOptions{
		Types:    []string{string(types.TypeLesson)},
		MinTrust: 0.5,
		Limit:    10,
	})
	if err != nil {
		panic(err)
	}
	_ = results

	// 5. Full-text search
	ftsResults, err := store.SearchKnowledgeFTS(ctx, "deployment disk", 5)
	if err != nil {
		panic(err)
	}
	_ = ftsResults

	// 6. Update user trust score
	if err := store.UpdateUserTrustScore(ctx, "user-123", "linux", true); err != nil {
		panic(err)
	}

	// 7. Create a session
	session := &types.Session{
		ID:             "session-001",
		UserID:         "user-123",
		Title:          "Deployment troubleshooting",
		CWD:            "/home/user/project",
		PlatformVendor: "linux",
		PlatformName:   "ubuntu",
		Status:         "active",
		StartedAt:      time.Now(),
		LastActivityAt: time.Now(),
	}

	if err := store.CreateSession(ctx, session); err != nil {
		panic(err)
	}

	// 8. Add messages to session
	msg := &types.SessionMessage{
		SessionID: "session-001",
		Role:      "user",
		Content:   "Why is my deployment failing?",
		CreatedAt: time.Now(),
	}

	if err := store.AddMessage(ctx, msg); err != nil {
		panic(err)
	}

	// Output: Example completed successfully
	fmt.Println("Example completed successfully")
}

// TestDatabaseLifecycle verifies basic database operations.
func TestDatabaseLifecycle(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	store, err := data.NewDB(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Test health check
	if err := store.Health(); err != nil {
		t.Fatalf("Health check failed: %v", err)
	}

	// Test knowledge CRUD
	item := &types.KnowledgeItem{
		ID:         "test-001",
		Type:       types.TypeSOP,
		Title:      "Test SOP",
		Content:    "Test content",
		Tags:       []string{"test"},
		Scope:      types.ScopePersonal,
		AuthorID:   "test-user",
		Confidence: 0.9,
		TrustScore: 0.8,
		SyncStatus: "local_only",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Create
	if err := store.CreateKnowledge(ctx, item); err != nil {
		t.Fatalf("Failed to create knowledge: %v", err)
	}

	// Read
	retrieved, err := store.GetKnowledge(ctx, "test-001")
	if err != nil {
		t.Fatalf("Failed to get knowledge: %v", err)
	}

	if retrieved.ID != item.ID || retrieved.Title != item.Title {
		t.Errorf("Retrieved item doesn't match: got %+v", retrieved)
	}

	// Update
	item.Title = "Updated Test SOP"
	item.Confidence = 0.95
	if err := store.UpdateKnowledge(ctx, item); err != nil {
		t.Fatalf("Failed to update knowledge: %v", err)
	}

	updated, err := store.GetKnowledge(ctx, "test-001")
	if err != nil {
		t.Fatalf("Failed to get updated knowledge: %v", err)
	}

	if updated.Title != "Updated Test SOP" || updated.Confidence != 0.95 {
		t.Errorf("Update didn't persist: got %+v", updated)
	}

	// Delete (soft delete)
	if err := store.DeleteKnowledge(ctx, "test-001"); err != nil {
		t.Fatalf("Failed to delete knowledge: %v", err)
	}

	// Verify soft delete - item should not be retrievable
	_, err = store.GetKnowledge(ctx, "test-001")
	if err == nil {
		t.Error("Expected error when getting deleted item")
	}
}

// TestTrustProfile verifies trust score calculations.
func TestTrustProfile(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := data.NewDB(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Update user trust scores
	for i := 0; i < 10; i++ {
		if err := store.UpdateUserTrustScore(ctx, "user-1", "linux", true); err != nil {
			t.Fatalf("Failed to update trust score: %v", err)
		}
	}

	for i := 0; i < 2; i++ {
		if err := store.UpdateUserTrustScore(ctx, "user-1", "linux", false); err != nil {
			t.Fatalf("Failed to update trust score: %v", err)
		}
	}

	// Get profile
	profile, err := store.GetTrustProfile(ctx, "user-1", "linux")
	if err != nil {
		t.Fatalf("Failed to get trust profile: %v", err)
	}

	if profile.SuccessCount != 10 {
		t.Errorf("Expected 10 successes, got %d", profile.SuccessCount)
	}

	if profile.FailureCount != 2 {
		t.Errorf("Expected 2 failures, got %d", profile.FailureCount)
	}

	// Score should be high (10 successes / 12 total = ~0.83)
	if profile.Score < 0.7 {
		t.Errorf("Expected high trust score, got %.2f", profile.Score)
	}
}

// TestSessionOperations verifies session and message operations.
func TestSessionOperations(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := data.NewDB(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create session
	session := &types.Session{
		ID:             "sess-001",
		UserID:         "user-1",
		Title:          "Test Session",
		CWD:            "/home/test",
		PlatformVendor: "linux",
		Status:         "active",
		StartedAt:      time.Now(),
		LastActivityAt: time.Now(),
	}

	if err := store.CreateSession(ctx, session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Add messages
	messages := []*types.SessionMessage{
		{
			SessionID: "sess-001",
			Role:      "user",
			Content:   "Hello",
			CreatedAt: time.Now(),
		},
		{
			SessionID: "sess-001",
			Role:      "assistant",
			Content:   "Hi! How can I help?",
			CreatedAt: time.Now().Add(1 * time.Second),
		},
	}

	for _, msg := range messages {
		if err := store.AddMessage(ctx, msg); err != nil {
			t.Fatalf("Failed to add message: %v", err)
		}
	}

	// Retrieve messages
	retrieved, err := store.GetSessionMessages(ctx, "sess-001")
	if err != nil {
		t.Fatalf("Failed to get messages: %v", err)
	}

	if len(retrieved) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(retrieved))
	}

	// Verify order
	if retrieved[0].Role != "user" || retrieved[1].Role != "assistant" {
		t.Error("Messages not in correct order")
	}

	// Update session status
	if err := store.UpdateSessionStatus(ctx, "sess-001", "completed"); err != nil {
		t.Fatalf("Failed to update session status: %v", err)
	}

	// Verify status update
	updated, err := store.GetSession(ctx, "sess-001")
	if err != nil {
		t.Fatalf("Failed to get updated session: %v", err)
	}

	if updated.Status != "completed" {
		t.Errorf("Expected status 'completed', got '%s'", updated.Status)
	}

	if updated.EndedAt == nil {
		t.Error("Expected ended_at to be set")
	}
}

// TestFullTextSearch verifies FTS functionality.
func TestFullTextSearch(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := data.NewDB(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create test items
	items := []*types.KnowledgeItem{
		{
			ID:         "fts-001",
			Type:       types.TypeLesson,
			Title:      "Docker disk space issues",
			Content:    "Clean up unused Docker images with 'docker system prune'",
			Tags:       []string{"docker", "troubleshooting"},
			Scope:      types.ScopePersonal,
			AuthorID:   "test-user",
			Confidence: 0.9,
			TrustScore: 0.85,
			SyncStatus: "local_only",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
		{
			ID:         "fts-002",
			Type:       types.TypeSOP,
			Title:      "Kubernetes deployment checklist",
			Content:    "1. Check cluster health\n2. Verify namespace\n3. Apply manifests",
			Tags:       []string{"kubernetes", "deployment"},
			Scope:      types.ScopeTeam,
			AuthorID:   "test-user",
			Confidence: 0.95,
			TrustScore: 0.9,
			SyncStatus: "synced",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
	}

	for _, item := range items {
		if err := store.CreateKnowledge(ctx, item); err != nil {
			t.Fatalf("Failed to create item: %v", err)
		}
	}

	// Search for "docker"
	results, err := store.SearchKnowledgeFTS(ctx, "docker", 10)
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result for 'docker', got %d", len(results))
	}

	if len(results) > 0 && results[0].ID != "fts-001" {
		t.Errorf("Expected fts-001, got %s", results[0].ID)
	}

	// Search for "deployment"
	results, err = store.SearchKnowledgeFTS(ctx, "deployment", 10)
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result for 'deployment', got %d", len(results))
	}
}
