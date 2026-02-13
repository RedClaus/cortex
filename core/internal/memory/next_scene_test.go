package memory

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockMemCubeSearcher implements MemCubeSearcher for testing.
type mockMemCubeSearcher struct {
	searchCalls int32
	delay       time.Duration
	results     map[CubeType][]*MemCube
}

func newMockSearcher() *mockMemCubeSearcher {
	return &mockMemCubeSearcher{
		results: make(map[CubeType][]*MemCube),
	}
}

func (m *mockMemCubeSearcher) SearchSimilar(ctx context.Context, query string, cubeType CubeType, limit int) ([]*MemCube, error) {
	atomic.AddInt32(&m.searchCalls, 1)

	// Simulate network delay
	if m.delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(m.delay):
		}
	}

	// Check for cancellation after delay
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if cubes, ok := m.results[cubeType]; ok {
		if len(cubes) > limit {
			return cubes[:limit], nil
		}
		return cubes, nil
	}
	return nil, nil
}

func (m *mockMemCubeSearcher) addResult(cubeType CubeType, cube *MemCube) {
	m.results[cubeType] = append(m.results[cubeType], cube)
}

func (m *mockMemCubeSearcher) getSearchCalls() int32 {
	return atomic.LoadInt32(&m.searchCalls)
}

// ============================================================================
// TESTS
// ============================================================================

func TestNextScenePredictor_NewAndClose(t *testing.T) {
	searcher := newMockSearcher()
	predictor := NewNextScenePredictor(searcher)
	require.NotNil(t, predictor)
	require.NotNil(t, predictor.cache)
	require.NotNil(t, predictor.prefetchCh)
	require.NotNil(t, predictor.closeCh)

	// Close should not panic or hang
	predictor.Close()
}

func TestNextScenePredictor_ShortInput(t *testing.T) {
	searcher := newMockSearcher()
	predictor := NewNextScenePredictor(searcher)
	defer predictor.Close()

	ctx := context.Background()

	// Input < 10 chars should return nil without searching
	result := predictor.Predict(ctx, "short")
	assert.Nil(t, result)
	assert.Equal(t, int32(0), searcher.getSearchCalls())

	// Exactly 9 chars should also return nil
	result = predictor.Predict(ctx, "123456789")
	assert.Nil(t, result)
	assert.Equal(t, int32(0), searcher.getSearchCalls())

	// 10 chars should trigger search
	result = predictor.Predict(ctx, "1234567890")
	assert.Equal(t, int32(1), searcher.getSearchCalls()) // At least knowledge search
}

func TestNextScenePredictor_BasicPrediction(t *testing.T) {
	searcher := newMockSearcher()
	searcher.addResult(CubeTypeText, &MemCube{
		ID:          "cube1",
		Content:     "Test knowledge content",
		ContentType: CubeTypeText,
		Confidence:  0.8,
	})

	predictor := NewNextScenePredictor(searcher)
	defer predictor.Close()

	ctx := context.Background()
	result := predictor.Predict(ctx, "What is this test about?")

	require.NotNil(t, result)
	assert.Len(t, result, 1)
	assert.Equal(t, "cube1", result[0].ID)
	assert.Equal(t, "Test knowledge content", result[0].Content)
}

func TestNextScenePredictor_CodeIntent(t *testing.T) {
	searcher := newMockSearcher()
	searcher.addResult(CubeTypeText, &MemCube{
		ID:          "knowledge1",
		Content:     "General knowledge",
		ContentType: CubeTypeText,
	})
	searcher.addResult(CubeTypeSkill, &MemCube{
		ID:          "skill1",
		Content:     "Coding skill pattern",
		ContentType: CubeTypeSkill,
	})

	predictor := NewNextScenePredictor(searcher)
	defer predictor.Close()

	ctx := context.Background()

	// Query with code intent should search skills + knowledge
	result := predictor.Predict(ctx, "implement a function to sort arrays")

	require.NotNil(t, result)
	// Should have both knowledge and skill results
	assert.GreaterOrEqual(t, len(result), 2)

	// Verify we got both types
	hasSkill := false
	hasKnowledge := false
	for _, cube := range result {
		if cube.ContentType == CubeTypeSkill {
			hasSkill = true
		}
		if cube.ContentType == CubeTypeText {
			hasKnowledge = true
		}
	}
	assert.True(t, hasSkill, "Should include skill results for code intent")
	assert.True(t, hasKnowledge, "Should include knowledge results")
}

func TestNextScenePredictor_ToolIntent(t *testing.T) {
	searcher := newMockSearcher()
	searcher.addResult(CubeTypeText, &MemCube{
		ID:          "knowledge1",
		Content:     "General knowledge",
		ContentType: CubeTypeText,
	})
	searcher.addResult(CubeTypeTool, &MemCube{
		ID:          "tool1",
		Content:     "Tool execution pattern",
		ContentType: CubeTypeTool,
	})

	predictor := NewNextScenePredictor(searcher)
	defer predictor.Close()

	ctx := context.Background()

	// Query with tool intent should search tools + knowledge
	result := predictor.Predict(ctx, "run the command to deploy")

	require.NotNil(t, result)
	// Should have both knowledge and tool results
	assert.GreaterOrEqual(t, len(result), 2)

	// Verify we got both types
	hasTool := false
	hasKnowledge := false
	for _, cube := range result {
		if cube.ContentType == CubeTypeTool {
			hasTool = true
		}
		if cube.ContentType == CubeTypeText {
			hasKnowledge = true
		}
	}
	assert.True(t, hasTool, "Should include tool results for tool intent")
	assert.True(t, hasKnowledge, "Should include knowledge results")
}

func TestNextScenePredictor_SlashCommand(t *testing.T) {
	searcher := newMockSearcher()
	searcher.addResult(CubeTypeTool, &MemCube{
		ID:          "tool1",
		Content:     "Tool pattern",
		ContentType: CubeTypeTool,
	})

	predictor := NewNextScenePredictor(searcher)
	defer predictor.Close()

	ctx := context.Background()

	// Slash commands should trigger tool intent
	result := predictor.Predict(ctx, "/help with this task")

	require.NotNil(t, result)

	hasTool := false
	for _, cube := range result {
		if cube.ContentType == CubeTypeTool {
			hasTool = true
			break
		}
	}
	assert.True(t, hasTool, "Slash commands should search for tool patterns")
}

func TestNextScenePredictor_Caching(t *testing.T) {
	searcher := newMockSearcher()
	searcher.addResult(CubeTypeText, &MemCube{
		ID:      "cube1",
		Content: "Test content",
	})

	predictor := NewNextScenePredictor(searcher)
	defer predictor.Close()

	ctx := context.Background()
	input := "test query for caching behavior"

	// First call should search
	result1 := predictor.Predict(ctx, input)
	require.NotNil(t, result1)
	firstCalls := searcher.getSearchCalls()

	// Second call with same input should use cache
	result2 := predictor.Predict(ctx, input)
	require.NotNil(t, result2)
	assert.Equal(t, firstCalls, searcher.getSearchCalls(), "Should use cached result")

	// Results should be the same
	assert.Equal(t, len(result1), len(result2))
}

func TestNextScenePredictor_ClearCache(t *testing.T) {
	searcher := newMockSearcher()
	searcher.addResult(CubeTypeText, &MemCube{
		ID:      "cube1",
		Content: "Test content",
	})

	predictor := NewNextScenePredictor(searcher)
	defer predictor.Close()

	ctx := context.Background()
	input := "test query for cache clearing"

	// First call
	predictor.Predict(ctx, input)
	firstCalls := searcher.getSearchCalls()

	// Clear cache
	predictor.ClearCache()

	// Second call should search again
	predictor.Predict(ctx, input)
	assert.Greater(t, searcher.getSearchCalls(), firstCalls, "Should search again after cache clear")
}

func TestNextScenePredictor_Timeout(t *testing.T) {
	searcher := newMockSearcher()
	searcher.delay = 100 * time.Millisecond // Delay longer than 50ms timeout
	searcher.addResult(CubeTypeText, &MemCube{
		ID:      "cube1",
		Content: "Test content",
	})

	predictor := NewNextScenePredictor(searcher)
	defer predictor.Close()

	ctx := context.Background()
	start := time.Now()
	result := predictor.Predict(ctx, "test query with timeout")
	elapsed := time.Since(start)

	// Should complete within ~60ms (50ms timeout + some overhead)
	// Even though searcher has 100ms delay
	assert.Less(t, elapsed, 80*time.Millisecond, "Should respect 50ms timeout")

	// Results may be empty due to timeout
	_ = result
}

func TestNextScenePredictor_ContextCancellation(t *testing.T) {
	searcher := newMockSearcher()
	searcher.delay = 200 * time.Millisecond
	searcher.addResult(CubeTypeText, &MemCube{
		ID:      "cube1",
		Content: "Test content",
	})

	predictor := NewNextScenePredictor(searcher)
	defer predictor.Close()

	// Create context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	result := predictor.Predict(ctx, "test with cancelled context")
	elapsed := time.Since(start)

	// Should return quickly since context is cancelled
	assert.Less(t, elapsed, 20*time.Millisecond)
	assert.Nil(t, result)
}

func TestNextScenePredictor_Prefetch(t *testing.T) {
	searcher := newMockSearcher()
	searcher.addResult(CubeTypeText, &MemCube{
		ID:      "cube1",
		Content: "Test content",
	})

	predictor := NewNextScenePredictor(searcher)
	defer predictor.Close()

	// Prefetch should not block
	predictor.Prefetch("prefetch this query now")

	// Give background worker time to process
	time.Sleep(150 * time.Millisecond)

	// Check that prefetch triggered a search
	assert.Greater(t, searcher.getSearchCalls(), int32(0), "Prefetch should trigger search")
}

func TestNextScenePredictor_PrefetchShortInput(t *testing.T) {
	searcher := newMockSearcher()
	predictor := NewNextScenePredictor(searcher)
	defer predictor.Close()

	// Short input should be ignored
	predictor.Prefetch("short")

	// Give time for potential processing
	time.Sleep(20 * time.Millisecond)

	assert.Equal(t, int32(0), searcher.getSearchCalls(), "Short input should not trigger prefetch")
}

func TestExtractSignals(t *testing.T) {
	searcher := newMockSearcher()
	predictor := NewNextScenePredictor(searcher)
	defer predictor.Close()

	tests := []struct {
		input         string
		wantCode      bool
		wantTool      bool
	}{
		{"implement a function", true, false},
		{"fix the bug", true, false},
		{"error handling", true, false},
		{"class definition", true, false},
		{"method signature", true, false},
		{"run the tests", false, true},
		{"execute command", false, true},
		{"shell script", false, true},
		{"/help me", false, true},
		{"implement and run", true, true},
		{"fix the run command", true, true},
		{"what is the weather", false, false},
		{"explain this concept", false, false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			signals := predictor.extractSignals(tc.input)
			assert.Equal(t, tc.wantCode, signals.hasCodeIntent, "code intent for: %s", tc.input)
			assert.Equal(t, tc.wantTool, signals.hasToolIntent, "tool intent for: %s", tc.input)
		})
	}
}

func TestInjectPredictedCubes_Empty(t *testing.T) {
	result := InjectPredictedCubes(nil)
	assert.Empty(t, result)

	result = InjectPredictedCubes([]*MemCube{})
	assert.Empty(t, result)
}

func TestInjectPredictedCubes_SingleCube(t *testing.T) {
	cubes := []*MemCube{
		{
			ID:          "cube1",
			Content:     "Test content for injection",
			ContentType: CubeTypeText,
		},
	}

	result := InjectPredictedCubes(cubes)

	assert.Contains(t, result, "<predicted_context>")
	assert.Contains(t, result, "</predicted_context>")
	assert.Contains(t, result, "Test content for injection")
	assert.Contains(t, result, "Anticipated relevant information")
}

func TestInjectPredictedCubes_MultipleCubes(t *testing.T) {
	cubes := []*MemCube{
		{ID: "1", Content: "Skill content", ContentType: CubeTypeSkill},
		{ID: "2", Content: "Tool content", ContentType: CubeTypeTool},
		{ID: "3", Content: "Text content", ContentType: CubeTypeText},
	}

	result := InjectPredictedCubes(cubes)

	assert.Contains(t, result, "[Skill]")
	assert.Contains(t, result, "[Tool]")
	assert.Contains(t, result, "Skill content")
	assert.Contains(t, result, "Tool content")
	assert.Contains(t, result, "Text content")
}

func TestInjectPredictedCubes_LongContent(t *testing.T) {
	longContent := ""
	for i := 0; i < 200; i++ {
		longContent += "x"
	}

	cubes := []*MemCube{
		{ID: "1", Content: longContent, ContentType: CubeTypeText},
	}

	result := InjectPredictedCubes(cubes)

	// Should truncate to ~150 chars + "..."
	assert.Contains(t, result, "...")
	assert.Less(t, len(result), len(longContent)+100) // Allow for XML tags
}

func TestInjectPredictedCubes_NilCube(t *testing.T) {
	cubes := []*MemCube{
		nil,
		{ID: "2", Content: "Valid content", ContentType: CubeTypeText},
		nil,
	}

	result := InjectPredictedCubes(cubes)

	assert.Contains(t, result, "Valid content")
	// Should not panic on nil cubes
}

func TestTruncatePredictorInput(t *testing.T) {
	// Short input - unchanged
	assert.Equal(t, "short", truncatePredictorInput("short"))

	// Exactly 40 chars - unchanged
	input40 := "1234567890123456789012345678901234567890"
	assert.Equal(t, input40, truncatePredictorInput(input40))

	// Over 40 chars - truncated
	input50 := "12345678901234567890123456789012345678901234567890"
	result := truncatePredictorInput(input50)
	assert.Equal(t, "1234567890123456789012345678901234567890...", result)
}
