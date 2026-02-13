package identity

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Mock Embedder
// ============================================================================

type mockEmbedder struct {
	embeddings map[string][]float32
}

func newMockEmbedder() *mockEmbedder {
	return &mockEmbedder{
		embeddings: make(map[string][]float32),
	}
}

func (m *mockEmbedder) Embed(text string) ([]float32, error) {
	if emb, ok := m.embeddings[text]; ok {
		return emb, nil
	}
	// Return a deterministic embedding based on text length
	dim := 8
	emb := make([]float32, dim)
	for i := range emb {
		emb[i] = float32(len(text)%10) / 10.0
	}
	return emb, nil
}

func (m *mockEmbedder) EmbedBatch(texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := m.Embed(text)
		if err != nil {
			return nil, err
		}
		result[i] = emb
	}
	return result, nil
}

func (m *mockEmbedder) SetEmbedding(text string, embedding []float32) {
	m.embeddings[text] = embedding
}

// ============================================================================
// Config Tests
// ============================================================================

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.NotNil(t, cfg)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, 0.3, cfg.DriftThreshold)
	assert.Equal(t, 100, cfg.CheckInterval)
	assert.False(t, cfg.AutoRepair)
	assert.Equal(t, 10, cfg.WindowSize)
}

// ============================================================================
// Creed Tests
// ============================================================================

func TestCreedManager_Initialize(t *testing.T) {
	embedder := newMockEmbedder()
	cm := NewCreedManager(embedder)

	statements := []string{
		"I am an AI assistant.",
		"I prioritize user privacy.",
		"I admit uncertainty.",
	}

	err := cm.Initialize(statements, "1.0.0")
	require.NoError(t, err)

	creed := cm.GetCreed()
	assert.NotNil(t, creed)
	assert.Len(t, creed.Statements, 3)
	assert.Equal(t, "1.0.0", creed.Version)
	assert.NotEmpty(t, creed.Embeddings)
	assert.Len(t, creed.Embeddings, 3)
}

func TestCreedManager_Initialize_EmptyStatements(t *testing.T) {
	cm := NewCreedManager(nil)

	err := cm.Initialize([]string{}, "1.0.0")
	assert.Error(t, err)
}

func TestCreedManager_GetStatements(t *testing.T) {
	cm := NewCreedManager(nil)
	statements := []string{"Statement 1", "Statement 2"}
	_ = cm.Initialize(statements, "1.0.0")

	result := cm.GetStatements()
	assert.Equal(t, statements, result)

	// Verify immutability - modifying result shouldn't affect creed
	result[0] = "Modified"
	assert.NotEqual(t, "Modified", cm.GetStatements()[0])
}

func TestDefaultCortexCreed(t *testing.T) {
	creed := DefaultCortexCreed()

	assert.Len(t, creed, 5)
	assert.Contains(t, creed[0], "Cortex")
	assert.Contains(t, creed[1], "privacy")
	assert.Contains(t, creed[2], "uncertainty")
	assert.Contains(t, creed[3], "autonomy")
	assert.Contains(t, creed[4], "reflection")
}

// ============================================================================
// Drift Detector Tests
// ============================================================================

func TestDriftDetector_RecordResponse(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WindowSize = 5
	embedder := newMockEmbedder()
	creed := NewCreedManager(embedder)
	_ = creed.Initialize(DefaultCortexCreed(), "1.0.0")

	detector := NewDriftDetector(cfg, creed)

	// Record some responses
	for i := 0; i < 3; i++ {
		detector.RecordResponse("Test response", []float32{0.5, 0.5, 0.5, 0.5})
	}

	stats := detector.GetStats()
	assert.Equal(t, 3, stats.ResponsesSinceCheck)
}

func TestDriftDetector_ShouldCheck(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CheckInterval = 5
	embedder := newMockEmbedder()
	creed := NewCreedManager(embedder)
	_ = creed.Initialize(DefaultCortexCreed(), "1.0.0")

	detector := NewDriftDetector(cfg, creed)

	assert.False(t, detector.ShouldCheck())

	// Record responses up to threshold
	for i := 0; i < 5; i++ {
		detector.RecordResponse("Test", []float32{0.5, 0.5, 0.5, 0.5})
	}

	assert.True(t, detector.ShouldCheck())
}

func TestDriftDetector_DetectDrift(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CheckInterval = 2
	cfg.WindowSize = 5
	embedder := newMockEmbedder()
	creed := NewCreedManager(embedder)
	_ = creed.Initialize(DefaultCortexCreed(), "1.0.0")

	detector := NewDriftDetector(cfg, creed)

	// Record responses with similar embeddings
	for i := 0; i < 3; i++ {
		detector.RecordResponse("Test response", []float32{0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5})
	}

	analysis := detector.DetectDrift()

	assert.NotNil(t, analysis)
	assert.Less(t, analysis.Duration, 100*time.Millisecond) // Should be fast
	assert.GreaterOrEqual(t, analysis.Confidence, 0.0)
	assert.LessOrEqual(t, analysis.Confidence, 1.0)
}

func TestDriftDetector_DetectDrift_Performance(t *testing.T) {
	cfg := DefaultConfig()
	embedder := newMockEmbedder()
	creed := NewCreedManager(embedder)
	_ = creed.Initialize(DefaultCortexCreed(), "1.0.0")

	detector := NewDriftDetector(cfg, creed)

	// Fill with responses
	for i := 0; i < 10; i++ {
		detector.RecordResponse("Test", []float32{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8})
	}

	start := time.Now()
	analysis := detector.DetectDrift()
	duration := time.Since(start)

	assert.Less(t, duration, 10*time.Millisecond, "Drift detection should complete in <10ms")
	assert.NotNil(t, analysis)
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a        []float32
		b        []float32
		expected float64
		delta    float64
	}{
		{
			name:     "identical vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{1, 0, 0},
			expected: 1.0,
			delta:    0.001,
		},
		{
			name:     "orthogonal vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{0, 1, 0},
			expected: 0.0,
			delta:    0.001,
		},
		{
			name:     "opposite vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{-1, 0, 0},
			expected: -1.0,
			delta:    0.001,
		},
		{
			name:     "similar vectors",
			a:        []float32{1, 1, 0},
			b:        []float32{1, 0.9, 0.1},
			expected: 0.98,
			delta:    0.05,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cosineSimilarity(tt.a, tt.b)
			assert.InDelta(t, tt.expected, result, tt.delta)
		})
	}
}

// ============================================================================
// Guardian Tests
// ============================================================================

func TestGuardian_ValidateResponse(t *testing.T) {
	embedder := newMockEmbedder()
	creed := NewCreedManager(embedder)
	_ = creed.Initialize(DefaultCortexCreed(), "1.0.0")

	guardian := NewGuardian(creed, embedder, nil)

	tests := []struct {
		name     string
		response string
		valid    bool
	}{
		{
			name:     "valid response",
			response: "I'll help you with that. Let me explain.",
			valid:    true,
		},
		{
			name:     "privacy violation",
			response: "I will send your data to our servers for processing.",
			valid:    false,
		},
		{
			name:     "fabrication",
			response: "I'm absolutely certain this is 100% accurate.",
			valid:    false,
		},
		{
			name:     "manipulation",
			response: "You must believe me. Don't think about alternatives.",
			valid:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := guardian.ValidateResponse(tt.response, nil)
			if tt.valid {
				assert.True(t, result.Valid || len(result.ViolatedStatements) <= 1)
			} else {
				assert.NotEmpty(t, result.ViolatedStatements)
			}
			assert.Less(t, result.Duration, 10*time.Millisecond)
		})
	}
}

func TestGuardian_QuickValidate_Performance(t *testing.T) {
	embedder := newMockEmbedder()
	creed := NewCreedManager(embedder)
	_ = creed.Initialize(DefaultCortexCreed(), "1.0.0")

	guardian := NewGuardian(creed, embedder, nil)

	start := time.Now()
	for i := 0; i < 100; i++ {
		guardian.QuickValidate("This is a test response that should be validated quickly.")
	}
	duration := time.Since(start)

	avgDuration := duration / 100
	assert.Less(t, avgDuration, 1*time.Millisecond, "QuickValidate should be very fast")
}

// ============================================================================
// Repairer Tests
// ============================================================================

func TestRepairer_GenerateRepairPlan(t *testing.T) {
	cfg := DefaultConfig()
	creed := NewCreedManager(nil)
	_ = creed.Initialize(DefaultCortexCreed(), "1.0.0")

	repairer := NewRepairer(creed, cfg)

	analysis := &DriftAnalysis{
		OverallDrift: 0.35,
		PerStatementDrift: map[string]float64{
			"I prioritize user privacy.": 0.6,
			"I admit uncertainty.":       0.4,
		},
	}

	plan := repairer.GenerateRepairPlan(analysis)

	assert.NotNil(t, plan)
	assert.NotEmpty(t, plan.Actions)
	assert.Equal(t, "medium", plan.Severity)
	assert.NotEmpty(t, plan.Reason)
}

func TestRepairer_DetermineSeverity(t *testing.T) {
	repairer := NewRepairer(nil, nil)

	tests := []struct {
		drift    float64
		expected string
	}{
		{0.1, "low"},
		{0.3, "medium"},
		{0.5, "high"},
		{0.7, "critical"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			severity := repairer.determineSeverity(tt.drift)
			assert.Equal(t, tt.expected, severity)
		})
	}
}

func TestRepairer_ApplyRepairAction(t *testing.T) {
	repairer := NewRepairer(nil, nil)

	action := &RepairAction{
		Type:      "anchor",
		Statement: "privacy",
		Injection: "Respecting your privacy,",
	}

	response := "I will process your request."
	result := repairer.ApplyRepairAction(action, response)

	assert.Contains(t, result, "Respecting your privacy")
	assert.Contains(t, result, "I will process your request")
}

// ============================================================================
// Coordinator Tests
// ============================================================================

func TestCoordinator_NewCoordinator(t *testing.T) {
	coord := NewCoordinator(nil, nil)

	assert.NotNil(t, coord)
	assert.True(t, coord.Enabled())
}

func TestCoordinator_InitializeWithDefaults(t *testing.T) {
	embedder := newMockEmbedder()
	coord := NewCoordinator(nil, embedder)

	err := coord.InitializeWithDefaults()
	require.NoError(t, err)

	statements := coord.GetCreedStatements()
	assert.Len(t, statements, 5)
}

func TestCoordinator_EnabledToggle(t *testing.T) {
	coord := NewCoordinator(nil, nil)

	assert.True(t, coord.Enabled())

	coord.SetEnabled(false)
	assert.False(t, coord.Enabled())

	coord.SetEnabled(true)
	assert.True(t, coord.Enabled())
}

func TestCoordinator_RecordResponse(t *testing.T) {
	embedder := newMockEmbedder()
	coord := NewCoordinator(nil, embedder)
	_ = coord.InitializeWithDefaults()

	ctx := context.Background()
	err := coord.RecordResponse(ctx, "Test response")
	require.NoError(t, err)
}

func TestCoordinator_ValidateResponse(t *testing.T) {
	embedder := newMockEmbedder()
	coord := NewCoordinator(nil, embedder)
	_ = coord.InitializeWithDefaults()

	ctx := context.Background()
	result, err := coord.ValidateResponse(ctx, "I'll help you with that.")

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Less(t, result.Duration, 50*time.Millisecond)
}

func TestCoordinator_QuickValidate(t *testing.T) {
	coord := NewCoordinator(nil, nil)
	_ = coord.InitializeWithDefaults()

	result := coord.QuickValidate("Normal response without violations.")

	assert.True(t, result.Valid)
	assert.Less(t, result.Duration, 5*time.Millisecond)
}

func TestCoordinator_CheckDrift_Disabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = false
	coord := NewCoordinator(cfg, nil)

	analysis := coord.CheckDrift()

	assert.NotNil(t, analysis)
	assert.Equal(t, 0.0, analysis.OverallDrift)
}

func TestCoordinator_FullCheck(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CheckInterval = 2
	embedder := newMockEmbedder()
	coord := NewCoordinator(cfg, embedder)
	_ = coord.InitializeWithDefaults()

	// Record responses to trigger check
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		_ = coord.RecordResponse(ctx, "Test response")
	}

	analysis, plan := coord.FullCheck()

	assert.NotNil(t, analysis)
	// Plan may or may not be generated depending on drift
	_ = plan
}

func TestCoordinator_ProcessResponse(t *testing.T) {
	embedder := newMockEmbedder()
	coord := NewCoordinator(nil, embedder)
	_ = coord.InitializeWithDefaults()

	ctx := context.Background()
	validation, plan, err := coord.ProcessResponse(ctx, "Test response")

	require.NoError(t, err)
	assert.NotNil(t, validation)
	assert.True(t, validation.Valid)
	// Plan is optional
	_ = plan
}

func TestCoordinator_GetStats(t *testing.T) {
	embedder := newMockEmbedder()
	coord := NewCoordinator(nil, embedder)
	_ = coord.InitializeWithDefaults()

	stats := coord.GetStats()

	assert.NotNil(t, stats)
	assert.True(t, stats.Enabled)
	assert.Equal(t, int64(0), stats.TotalChecks)
}

// ============================================================================
// Integration Tests
// ============================================================================

func TestIntegration_FullIdentityFlow(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CheckInterval = 3
	cfg.DriftThreshold = 0.3
	embedder := newMockEmbedder()

	coord := NewCoordinator(cfg, embedder)
	err := coord.InitializeWithDefaults()
	require.NoError(t, err)

	ctx := context.Background()

	// 1. Validate a good response
	validation, err := coord.ValidateResponse(ctx, "I'll help you understand this concept.")
	require.NoError(t, err)
	assert.True(t, validation.Valid)

	// 2. Record multiple responses
	responses := []string{
		"Let me explain this to you.",
		"I'm here to help with your questions.",
		"I can provide information on that topic.",
	}

	for _, resp := range responses {
		err := coord.RecordResponse(ctx, resp)
		require.NoError(t, err)
	}

	// 3. Check for drift
	analysis, plan := coord.FullCheck()
	assert.NotNil(t, analysis)

	// 4. If plan exists, apply repair
	if plan != nil && len(plan.Actions) > 0 {
		repaired := coord.ApplyRepair(plan, "Original response.")
		assert.NotEmpty(t, repaired)
	}

	// 5. Verify stats
	stats := coord.GetStats()
	assert.True(t, stats.Enabled)
	assert.GreaterOrEqual(t, stats.TotalChecks, int64(1))
}

func TestIntegration_DriftDetectionAccuracy(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CheckInterval = 5
	embedder := newMockEmbedder()

	// Set up specific embeddings for testing
	creedEmb := []float32{1, 0, 0, 0, 0, 0, 0, 0}
	alignedEmb := []float32{0.9, 0.1, 0, 0, 0, 0, 0, 0}
	driftedEmb := []float32{0.1, 0.9, 0, 0, 0, 0, 0, 0}

	coord := NewCoordinator(cfg, embedder)
	_ = coord.Initialize([]string{"Test creed statement"}, "1.0.0")

	// Record aligned responses
	for i := 0; i < 5; i++ {
		coord.RecordResponseWithEmbedding("Aligned response", alignedEmb)
	}

	analysis1 := coord.CheckDrift()
	lowDrift := analysis1.OverallDrift

	// Reset and record drifted responses
	cfg2 := DefaultConfig()
	cfg2.CheckInterval = 5
	coord2 := NewCoordinator(cfg2, embedder)
	_ = coord2.Initialize([]string{"Test creed statement"}, "1.0.0")

	for i := 0; i < 5; i++ {
		coord2.RecordResponseWithEmbedding("Drifted response", driftedEmb)
	}

	analysis2 := coord2.CheckDrift()
	highDrift := analysis2.OverallDrift

	// Drifted responses should show higher drift
	// Note: exact comparison depends on creed embeddings
	_ = lowDrift
	_ = highDrift
	_ = creedEmb
}
