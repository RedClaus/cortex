// Package memory provides memory management for CortexBrain.
// This file contains tests for the Observational Memory system.
package memory

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ═══════════════════════════════════════════════════════════════════════════════
// TEST HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	return db
}

func setupTestStore(t *testing.T) *SQLiteObservationalStore {
	db := setupTestDB(t)
	store := NewSQLiteObservationalStore(db)
	if err := store.InitSchema(context.Background()); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}
	return store
}

// ═══════════════════════════════════════════════════════════════════════════════
// MESSAGE TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestStoreMessage(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	msg := &Message{
		ID:         "msg-001",
		Role:       "user",
		Content:    "Hello, this is a test message",
		Timestamp:  time.Now(),
		ThreadID:   "thread-1",
		ResourceID: "agent-1",
		TokenCount: 10,
	}

	err := store.StoreMessage(ctx, msg)
	if err != nil {
		t.Fatalf("StoreMessage failed: %v", err)
	}

	// Retrieve and verify
	messages, err := store.GetMessages(ctx, "thread-1", "agent-1", 10)
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}

	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}

	if messages[0].Content != msg.Content {
		t.Errorf("Content mismatch: got %q, want %q", messages[0].Content, msg.Content)
	}
}

func TestGetMessageTokenCount(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	// Store multiple messages
	for i := 0; i < 5; i++ {
		msg := &Message{
			ID:         generateID(),
			Role:       "user",
			Content:    "Test message content",
			Timestamp:  time.Now(),
			ThreadID:   "thread-1",
			ResourceID: "agent-1",
			TokenCount: 100,
		}
		store.StoreMessage(ctx, msg)
	}

	count, err := store.GetMessageTokenCount(ctx, "thread-1", "agent-1")
	if err != nil {
		t.Fatalf("GetMessageTokenCount failed: %v", err)
	}

	if count != 500 {
		t.Errorf("Expected token count 500, got %d", count)
	}
}

func TestMarkMessagesCompressed(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	// Store messages
	msgIDs := []string{"msg-1", "msg-2", "msg-3"}
	for _, id := range msgIDs {
		msg := &Message{
			ID:         id,
			Role:       "user",
			Content:    "Test",
			Timestamp:  time.Now(),
			ThreadID:   "thread-1",
			ResourceID: "agent-1",
			TokenCount: 10,
		}
		store.StoreMessage(ctx, msg)
	}

	// Mark as compressed
	err := store.MarkMessagesCompressed(ctx, msgIDs, "obs-001")
	if err != nil {
		t.Fatalf("MarkMessagesCompressed failed: %v", err)
	}

	// Verify compressed messages are not returned
	messages, err := store.GetMessages(ctx, "thread-1", "agent-1", 10)
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}

	if len(messages) != 0 {
		t.Errorf("Expected 0 uncompressed messages, got %d", len(messages))
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// OBSERVATION TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestStoreObservation(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	obs := &Observation{
		ID:          "obs-001",
		Content:     "User completed task successfully",
		Timestamp:   time.Now(),
		Priority:    PriorityHigh,
		TaskState:   "debugging",
		SourceRange: []string{"msg-1", "msg-2"},
		ThreadID:    "thread-1",
		ResourceID:  "agent-1",
		TokenCount:  50,
	}

	err := store.StoreObservation(ctx, obs)
	if err != nil {
		t.Fatalf("StoreObservation failed: %v", err)
	}

	// Retrieve and verify
	observations, err := store.GetObservations(ctx, "agent-1", 10)
	if err != nil {
		t.Fatalf("GetObservations failed: %v", err)
	}

	if len(observations) != 1 {
		t.Errorf("Expected 1 observation, got %d", len(observations))
	}

	if observations[0].Priority != PriorityHigh {
		t.Errorf("Priority mismatch: got %d, want %d", observations[0].Priority, PriorityHigh)
	}
}

func TestObservationPriorityOrdering(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	// Store observations with different priorities
	priorities := []ObservationPriority{PriorityLow, PriorityCritical, PriorityMedium}
	for i, p := range priorities {
		obs := &Observation{
			ID:         generateID(),
			Content:    "Test observation",
			Timestamp:  time.Now().Add(time.Duration(i) * time.Minute),
			Priority:   p,
			ThreadID:   "thread-1",
			ResourceID: "agent-1",
			TokenCount: 10,
		}
		store.StoreObservation(ctx, obs)
	}

	// Retrieve - should be ordered by priority DESC
	observations, err := store.GetObservations(ctx, "agent-1", 10)
	if err != nil {
		t.Fatalf("GetObservations failed: %v", err)
	}

	if len(observations) != 3 {
		t.Fatalf("Expected 3 observations, got %d", len(observations))
	}

	// First should be CRITICAL (highest priority)
	if observations[0].Priority != PriorityCritical {
		t.Errorf("Expected first observation to be CRITICAL, got %d", observations[0].Priority)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// REFLECTION TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestStoreReflection(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	ref := &Reflection{
		ID:         "ref-001",
		Content:    "User prefers step-by-step explanations",
		Timestamp:  time.Now(),
		Pattern:    "preference",
		SourceObs:  []string{"obs-1", "obs-2"},
		ResourceID: "agent-1",
		TokenCount: 30,
	}

	err := store.StoreReflection(ctx, ref)
	if err != nil {
		t.Fatalf("StoreReflection failed: %v", err)
	}

	// Retrieve and verify
	reflections, err := store.GetReflections(ctx, "agent-1", 10)
	if err != nil {
		t.Fatalf("GetReflections failed: %v", err)
	}

	if len(reflections) != 1 {
		t.Errorf("Expected 1 reflection, got %d", len(reflections))
	}

	if reflections[0].Pattern != "preference" {
		t.Errorf("Pattern mismatch: got %q, want %q", reflections[0].Pattern, "preference")
	}
}

func TestUnanalyzedReflections(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	// Store reflections
	for i := 0; i < 5; i++ {
		ref := &Reflection{
			ID:         generateID(),
			Content:    "Test reflection",
			Timestamp:  time.Now(),
			Pattern:    "learning",
			ResourceID: "agent-1",
			TokenCount: 10,
			Analyzed:   false,
		}
		store.StoreReflection(ctx, ref)
	}

	// Get unanalyzed (min 3)
	refs, err := store.GetUnanalyzedReflections(ctx, "agent-1", 3)
	if err != nil {
		t.Fatalf("GetUnanalyzedReflections failed: %v", err)
	}

	if len(refs) != 5 {
		t.Errorf("Expected 5 reflections, got %d", len(refs))
	}

	// Try with min 10 (should return nil)
	refs, err = store.GetUnanalyzedReflections(ctx, "agent-1", 10)
	if err != nil {
		t.Fatalf("GetUnanalyzedReflections failed: %v", err)
	}

	if refs != nil {
		t.Errorf("Expected nil (not enough reflections), got %d", len(refs))
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// TIMELINE TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestGetTimeline(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	baseTime := time.Now().Add(-time.Hour)

	// Store messages at different times
	for i := 0; i < 10; i++ {
		msg := &Message{
			ID:         generateID(),
			Role:       "user",
			Content:    "Test message",
			Timestamp:  baseTime.Add(time.Duration(i*10) * time.Minute),
			ThreadID:   "thread-1",
			ResourceID: "agent-1",
			TokenCount: 10,
		}
		store.StoreMessage(ctx, msg)
	}

	// Query middle range (messages 3-6)
	from := baseTime.Add(25 * time.Minute)
	to := baseTime.Add(65 * time.Minute)

	oc, err := store.GetTimeline(ctx, "agent-1", from, to)
	if err != nil {
		t.Fatalf("GetTimeline failed: %v", err)
	}

	// Should get messages 3, 4, 5, 6 (indices where timestamp falls in range)
	if len(oc.Messages) < 3 || len(oc.Messages) > 5 {
		t.Errorf("Expected 3-5 messages in timeline, got %d", len(oc.Messages))
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// INTEGRATION TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestFullMemoryPipeline(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	resourceID := "test-agent"
	threadID := "thread-1"

	// 1. Store messages
	for i := 0; i < 5; i++ {
		msg := &Message{
			ID:         generateID(),
			Role:       "user",
			Content:    "Test message for compression",
			Timestamp:  time.Now(),
			ThreadID:   threadID,
			ResourceID: resourceID,
			TokenCount: 100,
		}
		store.StoreMessage(ctx, msg)
	}

	// 2. Verify token count
	tokens, _ := store.GetMessageTokenCount(ctx, threadID, resourceID)
	if tokens != 500 {
		t.Errorf("Expected 500 tokens, got %d", tokens)
	}

	// 3. Store observation (simulating compression)
	obs := &Observation{
		ID:         generateID(),
		Content:    "Compressed context: User sent 5 messages about testing",
		Timestamp:  time.Now(),
		Priority:   PriorityMedium,
		TaskState:  "testing",
		ThreadID:   threadID,
		ResourceID: resourceID,
		TokenCount: 20,
	}
	store.StoreObservation(ctx, obs)

	// 4. Store reflection
	ref := &Reflection{
		ID:         generateID(),
		Content:    "User is doing testing-related work",
		Timestamp:  time.Now(),
		Pattern:    "workflow",
		SourceObs:  []string{obs.ID},
		ResourceID: resourceID,
		TokenCount: 10,
	}
	store.StoreReflection(ctx, ref)

	// 5. Verify full context retrieval
	observations, _ := store.GetObservations(ctx, resourceID, 10)
	reflections, _ := store.GetReflections(ctx, resourceID, 10)

	if len(observations) != 1 {
		t.Errorf("Expected 1 observation, got %d", len(observations))
	}

	if len(reflections) != 1 {
		t.Errorf("Expected 1 reflection, got %d", len(reflections))
	}
}
