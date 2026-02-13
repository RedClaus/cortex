package memory

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/normanking/pinky/internal/brain"
)

func TestSQLiteStore_StoreAndRecall(t *testing.T) {
	// Create a temporary database
	tmpFile, err := os.CreateTemp("", "pinky-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	store, err := NewSQLiteStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Store a memory
	mem := &brain.Memory{
		ID:         "test-1",
		UserID:     "user-1",
		Type:       brain.MemoryEpisodic,
		Content:    "Had a meeting with the team about the project",
		Importance: 0.7,
		Source:     "chat",
		CreatedAt:  time.Now(),
		AccessedAt: time.Now(),
	}

	if err := store.Store(ctx, mem); err != nil {
		t.Fatalf("Failed to store memory: %v", err)
	}

	// Recall by keyword
	results, err := store.Recall(ctx, "meeting team", RecallOptions{
		UserID: "user-1",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("Failed to recall: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if results[0].ID != "test-1" {
		t.Errorf("Expected ID test-1, got %s", results[0].ID)
	}
}

func TestSQLiteStore_TemporalSearch(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "pinky-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	store, err := NewSQLiteStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now()

	// Store memories at different times
	memories := []struct {
		id      string
		content string
		time    time.Time
	}{
		{"today-1", "Today's standup meeting", now},
		{"yesterday-1", "Yesterday's code review", now.AddDate(0, 0, -1)},
		{"last-week-1", "Last week's planning", now.AddDate(0, 0, -7)},
		{"old-1", "Old discussion from months ago", now.AddDate(0, -2, 0)},
	}

	for _, m := range memories {
		mem := &brain.Memory{
			ID:         m.id,
			UserID:     "user-1",
			Type:       brain.MemoryEpisodic,
			Content:    m.content,
			Importance: 0.5,
			CreatedAt:  m.time,
			AccessedAt: m.time,
		}
		if err := store.Store(ctx, mem); err != nil {
			t.Fatalf("Failed to store memory %s: %v", m.id, err)
		}
	}

	// Test temporal search for "yesterday"
	temporal := &TemporalContext{
		HasTimeReference: true,
		RelativeTime:     "yesterday",
		AbsoluteTime:     now.AddDate(0, 0, -1),
	}

	results, err := store.TemporalSearch(ctx, "user-1", temporal)
	if err != nil {
		t.Fatalf("Temporal search failed: %v", err)
	}

	// Should find the yesterday memory
	found := false
	for _, r := range results {
		if r.ID == "yesterday-1" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected to find 'yesterday-1' in temporal search results")
	}
}

func TestSQLiteStore_GetRecent(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "pinky-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	store, err := NewSQLiteStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now()

	// Store several memories
	for i := 0; i < 5; i++ {
		mem := &brain.Memory{
			ID:         fmt.Sprintf("mem-%d", i),
			UserID:     "user-1",
			Type:       brain.MemoryEpisodic,
			Content:    fmt.Sprintf("Memory number %d", i),
			Importance: 0.5,
			CreatedAt:  now.Add(-time.Duration(i) * time.Hour),
			AccessedAt: now,
		}
		if err := store.Store(ctx, mem); err != nil {
			t.Fatalf("Failed to store memory: %v", err)
		}
	}

	// Get recent with limit
	results, err := store.GetRecent(ctx, "user-1", 3)
	if err != nil {
		t.Fatalf("GetRecent failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// Most recent should be first
	if results[0].ID != "mem-0" {
		t.Errorf("Expected most recent memory first, got %s", results[0].ID)
	}
}

func TestSQLiteStore_ScoringWithTemporalContext(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "pinky-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	store, err := NewSQLiteStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now()

	// Store memories with same keywords but different times
	memories := []struct {
		id      string
		content string
		time    time.Time
	}{
		{"meeting-today", "Team meeting discussion", now},
		{"meeting-week", "Team meeting notes", now.AddDate(0, 0, -5)},
		{"meeting-old", "Team meeting summary", now.AddDate(0, -1, 0)},
	}

	for _, m := range memories {
		mem := &brain.Memory{
			ID:         m.id,
			UserID:     "user-1",
			Type:       brain.MemoryEpisodic,
			Content:    m.content,
			Importance: 0.5,
			CreatedAt:  m.time,
			AccessedAt: m.time,
		}
		if err := store.Store(ctx, mem); err != nil {
			t.Fatalf("Failed to store memory: %v", err)
		}
	}

	// Search with temporal context for "today"
	temporal := ParseTemporalContext("What was discussed in the meeting today?", now)

	results, err := store.Recall(ctx, "team meeting", RecallOptions{
		UserID:      "user-1",
		Limit:       10,
		TimeContext: temporal,
	})
	if err != nil {
		t.Fatalf("Recall failed: %v", err)
	}

	// The "today" meeting should be ranked first due to temporal boost
	if len(results) > 0 && results[0].ID != "meeting-today" {
		t.Errorf("Expected 'meeting-today' to be first due to temporal boost, got %s", results[0].ID)
	}
}

func TestSQLiteStore_Decay(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "pinky-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	store, err := NewSQLiteStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now()

	// Store an old memory that hasn't been accessed
	mem := &brain.Memory{
		ID:         "old-mem",
		UserID:     "user-1",
		Type:       brain.MemoryEpisodic,
		Content:    "Old memory",
		Importance: 0.8,
		CreatedAt:  now.AddDate(0, -2, 0),
		AccessedAt: now.AddDate(0, -2, 0), // Not accessed in 2 months
	}
	if err := store.Store(ctx, mem); err != nil {
		t.Fatalf("Failed to store memory: %v", err)
	}

	// Run decay
	if err := store.Decay(ctx); err != nil {
		t.Fatalf("Decay failed: %v", err)
	}

	// The memory should still exist (decay doesn't delete)
	results, err := store.GetRecent(ctx, "user-1", 10)
	if err != nil {
		t.Fatalf("GetRecent failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result after decay, got %d", len(results))
	}

	// Importance should be reduced (0.8 * 0.9 = 0.72)
	if results[0].Importance >= 0.8 {
		t.Errorf("Expected importance to be reduced from 0.8, got %f", results[0].Importance)
	}
}

func TestSQLiteStore_Prune(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "pinky-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	store, err := NewSQLiteStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now()

	// Store memories with different ages and importance
	memories := []struct {
		id         string
		importance float64
		time       time.Time
	}{
		{"keep-high-imp", 0.8, now.AddDate(-1, 0, 0)},   // Old but important
		{"keep-recent", 0.1, now},                       // Recent but low importance
		{"prune-old-low", 0.1, now.AddDate(-1, 0, 0)},   // Old and low importance
	}

	for _, m := range memories {
		mem := &brain.Memory{
			ID:         m.id,
			UserID:     "user-1",
			Type:       brain.MemoryEpisodic,
			Content:    "Test memory",
			Importance: m.importance,
			CreatedAt:  m.time,
			AccessedAt: m.time,
		}
		if err := store.Store(ctx, mem); err != nil {
			t.Fatalf("Failed to store memory: %v", err)
		}
	}

	// Prune memories older than 6 months with low importance
	if err := store.Prune(ctx, 6*30*24*time.Hour); err != nil {
		t.Fatalf("Prune failed: %v", err)
	}

	results, err := store.GetRecent(ctx, "user-1", 10)
	if err != nil {
		t.Fatalf("GetRecent failed: %v", err)
	}

	// Should have pruned "prune-old-low"
	if len(results) != 2 {
		t.Errorf("Expected 2 memories after prune, got %d", len(results))
	}

	for _, r := range results {
		if r.ID == "prune-old-low" {
			t.Error("Expected 'prune-old-low' to be pruned")
		}
	}
}
