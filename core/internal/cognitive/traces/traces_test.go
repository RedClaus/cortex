package traces

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/normanking/cortex/internal/agent"
)

// setupTestDB creates a temporary SQLite database for testing.
func setupTestDB(t *testing.T) *sql.DB {
	tmpFile, err := os.CreateTemp("", "traces_test_*.db")
	require.NoError(t, err)
	tmpFile.Close()

	t.Cleanup(func() {
		os.Remove(tmpFile.Name())
	})

	db, err := sql.Open("sqlite3", tmpFile.Name())
	require.NoError(t, err)

	// Create the reasoning_traces table
	_, err = db.Exec(`
		CREATE TABLE reasoning_traces (
			id TEXT PRIMARY KEY,
			query TEXT NOT NULL,
			query_embedding BLOB,
			approach TEXT,
			steps_json BLOB,
			outcome TEXT DEFAULT 'success',
			success_score REAL DEFAULT 0.0,
			reused_count INTEGER DEFAULT 0,
			tools_used TEXT DEFAULT '[]',
			lobes_activated TEXT DEFAULT '[]',
			total_duration_ms INTEGER DEFAULT 0,
			tokens_used INTEGER DEFAULT 0,
			created_at TEXT NOT NULL,
			last_used_at TEXT NOT NULL,
			metadata TEXT DEFAULT '{}'
		)
	`)
	require.NoError(t, err)

	return db
}

func TestTraceStore_StoreAndRetrieve(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewTraceStore(db, nil, nil)
	ctx := context.Background()

	// Create a trace
	trace := &ReasoningTrace{
		Query:    "How do I list files in Go?",
		Approach: "Use os.ReadDir or filepath.Walk",
		Steps: []ReasoningStep{
			{StepNum: 1, Action: ActionThink, Content: "I need to list files"},
			{StepNum: 2, Action: ActionToolCall, ToolName: "bash", ToolInput: "ls -la"},
			{StepNum: 3, Action: ActionConclude, Content: "Use os.ReadDir"},
		},
		Outcome:       OutcomeSuccess,
		SuccessScore:  0.9,
		ToolsUsed:     []string{"bash"},
		TotalDuration: 5 * time.Second,
		TokensUsed:    500,
	}

	// Store
	err := store.StoreTrace(ctx, trace)
	require.NoError(t, err)
	assert.NotEmpty(t, trace.ID)

	// Retrieve
	retrieved, err := store.GetTrace(ctx, trace.ID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	assert.Equal(t, trace.Query, retrieved.Query)
	assert.Equal(t, trace.Approach, retrieved.Approach)
	assert.Equal(t, trace.Outcome, retrieved.Outcome)
	assert.Equal(t, trace.SuccessScore, retrieved.SuccessScore)
	assert.Len(t, retrieved.Steps, 3)
	assert.Equal(t, "bash", retrieved.Steps[1].ToolName)
}

func TestTraceStore_IncrementReuse(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewTraceStore(db, nil, nil)
	ctx := context.Background()

	trace := &ReasoningTrace{
		Query:        "Test query",
		Outcome:      OutcomeSuccess,
		SuccessScore: 0.8,
	}

	err := store.StoreTrace(ctx, trace)
	require.NoError(t, err)

	// Increment reuse
	err = store.IncrementReuse(ctx, trace.ID)
	require.NoError(t, err)

	// Check reuse count
	retrieved, err := store.GetTrace(ctx, trace.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, retrieved.ReusedCount)

	// Increment again
	err = store.IncrementReuse(ctx, trace.ID)
	require.NoError(t, err)

	retrieved, err = store.GetTrace(ctx, trace.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, retrieved.ReusedCount)
}

func TestTraceStore_GetRecentTraces(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewTraceStore(db, nil, nil)
	ctx := context.Background()

	// Store multiple traces
	for i := 0; i < 5; i++ {
		trace := &ReasoningTrace{
			Query:        "Query " + string(rune('A'+i)),
			Outcome:      OutcomeSuccess,
			SuccessScore: 0.5 + float64(i)*0.1,
		}
		err := store.StoreTrace(ctx, trace)
		require.NoError(t, err)
	}

	// Get recent
	traces, err := store.GetRecentTraces(ctx, 3)
	require.NoError(t, err)
	assert.Len(t, traces, 3)
}

func TestTraceScorer_Score(t *testing.T) {
	scorer := NewTraceScorer()

	tests := []struct {
		name     string
		trace    *ReasoningTrace
		minScore float64
		maxScore float64
	}{
		{
			name: "successful short trace",
			trace: &ReasoningTrace{
				Outcome:       OutcomeSuccess,
				Steps:         make([]ReasoningStep, 2),
				CreatedAt:     time.Now(),
				LastUsedAt:    time.Now(),
				ReusedCount:   5,
				TotalDuration: 2 * time.Second,
			},
			minScore: 0.8,
			maxScore: 1.0,
		},
		{
			name: "failed trace",
			trace: &ReasoningTrace{
				Outcome:       OutcomeFailed,
				Steps:         make([]ReasoningStep, 10),
				CreatedAt:     time.Now().Add(-60 * 24 * time.Hour),
				TotalDuration: 30 * time.Second,
			},
			minScore: 0.0,
			maxScore: 0.3,
		},
		{
			name: "partial with reuses",
			trace: &ReasoningTrace{
				Outcome:       OutcomePartial,
				Steps:         make([]ReasoningStep, 5),
				CreatedAt:     time.Now().Add(-7 * 24 * time.Hour),
				LastUsedAt:    time.Now(),
				ReusedCount:   3,
				TotalDuration: 10 * time.Second,
			},
			minScore: 0.5,
			maxScore: 0.8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := scorer.Score(tt.trace)
			assert.GreaterOrEqual(t, score, tt.minScore, "score should be >= minScore")
			assert.LessOrEqual(t, score, tt.maxScore, "score should be <= maxScore")
		})
	}
}

func TestStepEventToReasoningStep(t *testing.T) {
	event := &agent.StepEvent{
		Type:      agent.EventToolCall,
		Step:      3,
		Message:   "Calling bash tool",
		ToolName:  "bash",
		ToolInput: "ls -la",
		Output:    "file1.go\nfile2.go",
		Success:   true,
	}

	step := StepEventToReasoningStep(event)

	assert.Equal(t, 3, step.StepNum)
	assert.Equal(t, ActionToolCall, step.Action)
	assert.Equal(t, "Calling bash tool", step.Content)
	assert.Equal(t, "bash", step.ToolName)
	assert.Equal(t, "ls -la", step.ToolInput)
	assert.True(t, step.Success)
}

func TestTraceCollector(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewTraceStore(db, nil, nil)
	collector := NewTraceCollector(store, nil)

	ctx := context.Background()

	// Track callback invocations
	callbackCount := 0
	originalCallback := func(event *agent.StepEvent) {
		callbackCount++
	}

	// Start collection
	callback := collector.Start("Test query", originalCallback)
	assert.True(t, collector.IsCollecting())
	assert.NotEmpty(t, collector.CurrentTraceID())

	// Simulate agent steps
	callback(&agent.StepEvent{Type: agent.EventThinking, Step: 1, Message: "Thinking..."})
	callback(&agent.StepEvent{Type: agent.EventToolCall, Step: 2, ToolName: "bash"})
	callback(&agent.StepEvent{Type: agent.EventToolResult, Step: 2, Success: true})
	callback(&agent.StepEvent{Type: agent.EventComplete, Step: 3})

	// Original callback should have been called
	assert.Equal(t, 4, callbackCount)

	// Finish collection
	trace, err := collector.Finish(ctx, OutcomeSuccess)
	require.NoError(t, err)
	require.NotNil(t, trace)

	assert.Equal(t, "Test query", trace.Query)
	assert.Equal(t, OutcomeSuccess, trace.Outcome)
	assert.Len(t, trace.Steps, 4)
	assert.Contains(t, trace.ToolsUsed, "bash")
	assert.False(t, collector.IsCollecting())
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 8, "hello..."},
		{"hi", 2, "hi"},
		{"hello", 4, "h..."},
		{"", 5, ""},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.maxLen)
		assert.Equal(t, tt.expected, result)
	}
}
