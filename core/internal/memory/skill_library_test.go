package memory

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// createTestDB creates an in-memory SQLite database for testing.
func createTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)

	// Create the skills table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS skills (
			id TEXT PRIMARY KEY,
			version INTEGER NOT NULL DEFAULT 1,
			skill_json TEXT NOT NULL,
			source TEXT NOT NULL DEFAULT 'execution',
			session_id TEXT,
			parent_id TEXT,
			confidence REAL DEFAULT 0.5,
			success_count INTEGER DEFAULT 0,
			failure_count INTEGER DEFAULT 0,
			embedding BLOB,
			created_at TEXT DEFAULT (datetime('now')),
			updated_at TEXT DEFAULT (datetime('now')),
			last_accessed_at TEXT,
			access_count INTEGER DEFAULT 0,
			FOREIGN KEY (parent_id) REFERENCES skills(id) ON DELETE SET NULL
		)
	`)
	require.NoError(t, err)

	return db
}

func TestNewSkillLibrary(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	sl := NewSkillLibrary(db, nil)
	assert.NotNil(t, sl)
	assert.Equal(t, db, sl.db)
}

func TestLearnFromExecution_Success(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	sl := NewSkillLibrary(db, nil)
	ctx := context.Background()

	trace := ExecutionTrace{
		SessionID:     "session-123",
		TraceID:       "trace-456",
		UserInput:     "Write a function to sort a list",
		TaskSummary:   "Implement quicksort algorithm",
		GeneratedCode: "func quicksort(arr []int) []int { ... }",
		Success:       true,
		Confidence:    0.9, // Above threshold
		DetectedTags:  []string{"sorting", "algorithm", "go"},
		CreatedAt:     time.Now(),
	}

	err := sl.LearnFromExecution(ctx, trace)
	require.NoError(t, err)

	// Verify skill was created
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM skills").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify skill content
	var skillJSON string
	err = db.QueryRow("SELECT skill_json FROM skills").Scan(&skillJSON)
	require.NoError(t, err)
	assert.Contains(t, skillJSON, "quicksort")
	assert.Contains(t, skillJSON, "sorting")
}

func TestLearnFromExecution_SkipsLowConfidence(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	sl := NewSkillLibrary(db, nil)
	ctx := context.Background()

	trace := ExecutionTrace{
		SessionID:   "session-123",
		UserInput:   "Do something",
		TaskSummary: "Low confidence task",
		Success:     true,
		Confidence:  0.5, // Below threshold (0.8)
	}

	err := sl.LearnFromExecution(ctx, trace)
	require.NoError(t, err)

	// Verify no skill was created
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM skills").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestLearnFromExecution_SkipsFailure(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	sl := NewSkillLibrary(db, nil)
	ctx := context.Background()

	trace := ExecutionTrace{
		SessionID:   "session-123",
		UserInput:   "Do something",
		TaskSummary: "Failed task",
		Success:     false, // Not successful
		Confidence:  0.9,
	}

	err := sl.LearnFromExecution(ctx, trace)
	require.NoError(t, err)

	// Verify no skill was created
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM skills").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestRecordOutcome_Success(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	sl := NewSkillLibrary(db, nil)
	ctx := context.Background()

	// Create a skill first
	trace := ExecutionTrace{
		SessionID:     "session-123",
		TaskSummary:   "Test skill",
		GeneratedCode: "test code",
		Success:       true,
		Confidence:    0.9,
	}
	err := sl.LearnFromExecution(ctx, trace)
	require.NoError(t, err)

	// Get the skill ID
	var skillID string
	err = db.QueryRow("SELECT id FROM skills").Scan(&skillID)
	require.NoError(t, err)

	// Record success
	err = sl.RecordOutcome(ctx, skillID, true)
	require.NoError(t, err)

	// Verify success count increased
	var successCount int
	err = db.QueryRow("SELECT success_count FROM skills WHERE id = ?", skillID).Scan(&successCount)
	require.NoError(t, err)
	assert.Equal(t, 2, successCount) // Initial 1 + recorded 1

	// Record failure
	err = sl.RecordOutcome(ctx, skillID, false)
	require.NoError(t, err)

	// Verify failure count increased
	var failureCount int
	err = db.QueryRow("SELECT failure_count FROM skills WHERE id = ?", skillID).Scan(&failureCount)
	require.NoError(t, err)
	assert.Equal(t, 1, failureCount)
}

func TestRecordOutcome_NotFound(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	sl := NewSkillLibrary(db, nil)
	ctx := context.Background()

	err := sl.RecordOutcome(ctx, "nonexistent-id", true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestStoredSkill_SuccessRate(t *testing.T) {
	tests := []struct {
		name         string
		successCount int
		failureCount int
		expectedRate float64
	}{
		{
			name:         "no observations",
			successCount: 0,
			failureCount: 0,
			expectedRate: 0.5, // Bayesian prior
		},
		{
			name:         "all success",
			successCount: 10,
			failureCount: 0,
			expectedRate: 11.0 / 12.0, // (10+1)/(10+0+2)
		},
		{
			name:         "all failure",
			successCount: 0,
			failureCount: 10,
			expectedRate: 1.0 / 12.0, // (0+1)/(0+10+2)
		},
		{
			name:         "balanced",
			successCount: 5,
			failureCount: 5,
			expectedRate: 6.0 / 12.0, // (5+1)/(5+5+2) = 0.5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ss := &StoredSkill{
				SuccessCount: tt.successCount,
				FailureCount: tt.failureCount,
			}
			rate := ss.SuccessRate()
			assert.InDelta(t, tt.expectedRate, rate, 0.001)
		})
	}
}

func TestGenerateSkillName(t *testing.T) {
	sl := NewSkillLibrary(nil, nil)

	tests := []struct {
		name     string
		trace    ExecutionTrace
		expected string
	}{
		{
			name: "from task summary",
			trace: ExecutionTrace{
				TaskSummary: "Implement quicksort algorithm",
			},
			expected: "Implement quicksort algorithm",
		},
		{
			name: "long task summary truncated",
			trace: ExecutionTrace{
				TaskSummary: "This is a very long task summary that should be truncated at some natural break point. It has multiple sentences.",
			},
			expected: "This is a very long task summary that should be...",
		},
		{
			name: "from tags",
			trace: ExecutionTrace{
				DetectedTags: []string{"sorting", "algorithm", "go"},
			},
			expected: "sorting_algorithm_go_skill",
		},
		{
			name: "from user input",
			trace: ExecutionTrace{
				UserInput: "Write a function to calculate fibonacci numbers recursively",
			},
			expected: "Write_a_function_to_calculate_skill",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := sl.generateSkillName(tt.trace)
			assert.Equal(t, tt.expected, name)
		})
	}
}

func TestSerializeDeserializeSkill(t *testing.T) {
	original := Skill{
		Name:        "Test Skill",
		Description: "A test skill for verification",
		Pattern:     "func test() {}",
		InputSchema: `{"type": "object"}`,
		Examples:    []string{"example1", "example2"},
		Tags:        []string{"test", "verification"},
	}

	// Serialize
	jsonStr, err := serializeSkill(original)
	require.NoError(t, err)
	assert.Contains(t, jsonStr, "Test Skill")
	assert.Contains(t, jsonStr, "verification")

	// Deserialize
	restored, err := deserializeSkill(jsonStr)
	require.NoError(t, err)
	assert.Equal(t, original.Name, restored.Name)
	assert.Equal(t, original.Description, restored.Description)
	assert.Equal(t, original.Pattern, restored.Pattern)
	assert.Equal(t, original.InputSchema, restored.InputSchema)
	assert.Equal(t, original.Examples, restored.Examples)
	assert.Equal(t, original.Tags, restored.Tags)
}

func TestGetByID(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	sl := NewSkillLibrary(db, nil)
	ctx := context.Background()

	// Create a skill
	trace := ExecutionTrace{
		SessionID:     "session-123",
		TaskSummary:   "Test retrieval skill",
		GeneratedCode: "func retrieve() {}",
		Success:       true,
		Confidence:    0.95,
		DetectedTags:  []string{"retrieval", "test"},
	}
	err := sl.LearnFromExecution(ctx, trace)
	require.NoError(t, err)

	// Get the skill ID
	var skillID string
	err = db.QueryRow("SELECT id FROM skills").Scan(&skillID)
	require.NoError(t, err)

	// Retrieve by ID
	stored, err := sl.GetByID(ctx, skillID)
	require.NoError(t, err)
	assert.Equal(t, skillID, stored.ID)
	assert.Equal(t, "Test retrieval skill", stored.Skill.Description)
	assert.Equal(t, "session-123", stored.SessionID)
	assert.Equal(t, 0.95, stored.Confidence)
}

func TestGetByID_NotFound(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	sl := NewSkillLibrary(db, nil)
	ctx := context.Background()

	_, err := sl.GetByID(ctx, "nonexistent-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestEvolveSkill(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	sl := NewSkillLibrary(db, nil)
	ctx := context.Background()

	// Create parent skill
	trace := ExecutionTrace{
		SessionID:     "session-123",
		TaskSummary:   "Original skill",
		GeneratedCode: "func original() {}",
		Success:       true,
		Confidence:    0.9,
	}
	err := sl.LearnFromExecution(ctx, trace)
	require.NoError(t, err)

	// Get parent ID
	var parentID string
	err = db.QueryRow("SELECT id FROM skills").Scan(&parentID)
	require.NoError(t, err)

	// Evolve the skill
	newPattern := "func improved() { // better implementation }"
	evolved, err := sl.EvolveSkill(ctx, parentID, newPattern, "improved efficiency")
	require.NoError(t, err)

	assert.NotEqual(t, parentID, evolved.ID)
	assert.Equal(t, parentID, evolved.ParentID)
	assert.Equal(t, 2, evolved.Version)
	assert.Equal(t, newPattern, evolved.Skill.Pattern)
	assert.Contains(t, evolved.Skill.Description, "improved efficiency")
	assert.Equal(t, 0.9*0.9, evolved.Confidence) // 10% confidence reduction
	assert.Equal(t, 0, evolved.SuccessCount)     // Fresh start
	assert.Equal(t, 0, evolved.FailureCount)
}

func TestGetStats(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	sl := NewSkillLibrary(db, nil)
	ctx := context.Background()

	// Create multiple skills with different success rates
	traces := []ExecutionTrace{
		{SessionID: "s1", TaskSummary: "Skill 1", GeneratedCode: "code1", Success: true, Confidence: 0.9},
		{SessionID: "s2", TaskSummary: "Skill 2", GeneratedCode: "code2", Success: true, Confidence: 0.85},
		{SessionID: "s3", TaskSummary: "Skill 3", GeneratedCode: "code3", Success: true, Confidence: 0.95},
	}

	for _, trace := range traces {
		err := sl.LearnFromExecution(ctx, trace)
		require.NoError(t, err)
	}

	// Record some outcomes
	rows, err := db.Query("SELECT id FROM skills")
	require.NoError(t, err)
	var ids []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()

	// Add extra successes and failures
	for i, id := range ids {
		for j := 0; j < i+1; j++ {
			sl.RecordOutcome(ctx, id, true)
		}
		if i%2 == 0 {
			sl.RecordOutcome(ctx, id, false)
		}
	}

	stats, err := sl.GetStats(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, stats.TotalSkills)
	assert.Greater(t, stats.AvgSuccessRate, 0.5)
	assert.Greater(t, stats.TotalExecutions, 0)
}
