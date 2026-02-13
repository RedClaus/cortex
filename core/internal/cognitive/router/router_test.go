package router

import (
	"context"
	"testing"

	"github.com/normanking/cortex/internal/cognitive"
)

// ═══════════════════════════════════════════════════════════════════════════════
// ROUTER TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestEmbeddingIndex(t *testing.T) {
	index := NewEmbeddingIndex()

	// Add some test embeddings
	e1 := make(cognitive.Embedding, 10)
	for i := range e1 {
		e1[i] = float32(i) / 10.0
	}

	e2 := make(cognitive.Embedding, 10)
	for i := range e2 {
		e2[i] = float32(i) / 5.0
	}

	index.Add("test1", e1, "metadata1")
	index.Add("test2", e2, "metadata2")

	if index.Size() != 2 {
		t.Errorf("Expected size 2, got %d", index.Size())
	}

	// Test search
	results := index.Search(e1, 2)
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// First result should be exact match
	if results[0].ID != "test1" {
		t.Errorf("Expected test1 to be top result, got %s", results[0].ID)
	}

	if results[0].Score < 0.99 {
		t.Errorf("Expected perfect similarity for exact match, got %.2f", results[0].Score)
	}

	// Test removal
	if !index.Remove("test1") {
		t.Error("Expected successful removal")
	}

	if index.Size() != 1 {
		t.Errorf("Expected size 1 after removal, got %d", index.Size())
	}

	// Test clear
	index.Clear()
	if index.Size() != 0 {
		t.Errorf("Expected size 0 after clear, got %d", index.Size())
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a        cognitive.Embedding
		b        cognitive.Embedding
		expected float64
	}{
		{
			name:     "identical vectors",
			a:        cognitive.Embedding{1, 0, 0},
			b:        cognitive.Embedding{1, 0, 0},
			expected: 1.0,
		},
		{
			name:     "orthogonal vectors",
			a:        cognitive.Embedding{1, 0, 0},
			b:        cognitive.Embedding{0, 1, 0},
			expected: 0.0,
		},
		{
			name:     "opposite vectors",
			a:        cognitive.Embedding{1, 0, 0},
			b:        cognitive.Embedding{-1, 0, 0},
			expected: -1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.a.CosineSimilarity(tt.b)
			if result < tt.expected-0.01 || result > tt.expected+0.01 {
				t.Errorf("Expected %.2f, got %.2f", tt.expected, result)
			}
		})
	}
}

func TestNormalize(t *testing.T) {
	e := cognitive.Embedding{3, 4, 0}
	normalized := e.Normalize()

	// Should be unit length (magnitude = 1)
	var magnitude float64
	for _, v := range normalized {
		magnitude += float64(v) * float64(v)
	}

	if magnitude < 0.99 || magnitude > 1.01 {
		t.Errorf("Expected magnitude ~1.0, got %.2f", magnitude)
	}
}

func TestRouterWithNullEmbedder(t *testing.T) {
	embedder := NewNullEmbedder()
	mockRegistry := &mockRegistry{}

	router := NewRouter(&RouterConfig{
		Embedder: embedder,
		Registry: mockRegistry,
	})

	ctx := context.Background()

	// Initialize (should work even with null embedder)
	if err := router.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Route should fall back to keyword search, and since mock returns no results,
	// it routes to novel (frontier model)
	result, err := router.Route(ctx, "test input")
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	// When embedder is unavailable and no keyword matches exist,
	// the decision should be novel (not fallback)
	if result.Decision != cognitive.RouteNovel {
		t.Errorf("Expected novel decision, got %s", result.Decision)
	}

	if result.EmbeddingFailed != true {
		t.Error("Expected EmbeddingFailed to be true")
	}

	// Should route to frontier tier
	if result.RecommendedTier != cognitive.TierFrontier {
		t.Errorf("Expected frontier tier, got %s", result.RecommendedTier)
	}
}

func TestSelectModelTier(t *testing.T) {
	router := NewRouter(&RouterConfig{
		Embedder:   NewNullEmbedder(),
		Registry:   &mockRegistry{},
		Thresholds: DefaultThresholds(),
	})

	tests := []struct {
		name       string
		similarity float64
		confidence float64
		expected   cognitive.ModelTier
	}{
		{
			name:       "high similarity + high confidence",
			similarity: 0.90,
			confidence: 0.85,
			expected:   cognitive.TierLocal,
		},
		{
			name:       "high similarity + medium confidence",
			similarity: 0.90,
			confidence: 0.65,
			expected:   cognitive.TierMid,
		},
		{
			name:       "medium similarity",
			similarity: 0.75,
			confidence: 0.85,
			expected:   cognitive.TierMid,
		},
		{
			name:       "low similarity",
			similarity: 0.55,
			confidence: 0.85,
			expected:   cognitive.TierAdvanced,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := &cognitive.Template{
				ConfidenceScore: tt.confidence,
			}
			result := router.selectModelTier(template, tt.similarity)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestEmbeddingSerialize(t *testing.T) {
	original := cognitive.Embedding{0.1, 0.2, 0.3, 0.4, 0.5}

	// Serialize to bytes
	bytes := original.ToBytes()
	if len(bytes) != len(original)*4 {
		t.Errorf("Expected %d bytes, got %d", len(original)*4, len(bytes))
	}

	// Deserialize back
	deserialized := cognitive.EmbeddingFromBytes(bytes)
	if len(deserialized) != len(original) {
		t.Errorf("Expected length %d, got %d", len(original), len(deserialized))
	}

	// Check values
	for i := range original {
		if deserialized[i] != original[i] {
			t.Errorf("Value mismatch at index %d: expected %.2f, got %.2f", i, original[i], deserialized[i])
		}
	}
}

func TestGetSimilarityLevel(t *testing.T) {
	tests := []struct {
		score    float64
		expected cognitive.SimilarityLevel
	}{
		{0.90, cognitive.SimilarityHigh},
		{0.85, cognitive.SimilarityHigh},
		{0.80, cognitive.SimilarityMedium},
		{0.70, cognitive.SimilarityMedium},
		{0.60, cognitive.SimilarityLow},
		{0.50, cognitive.SimilarityLow},
		{0.40, cognitive.SimilarityNoMatch},
	}

	for _, tt := range tests {
		result := cognitive.GetSimilarityLevel(tt.score)
		if result != tt.expected {
			t.Errorf("Score %.2f: expected %s, got %s", tt.score, tt.expected, result)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// MOCK REGISTRY
// ═══════════════════════════════════════════════════════════════════════════════

type mockRegistry struct{}

// CRUD Operations
func (m *mockRegistry) Create(ctx context.Context, t *cognitive.Template) error {
	return nil
}

func (m *mockRegistry) Get(ctx context.Context, id string) (*cognitive.Template, error) {
	return nil, nil
}

func (m *mockRegistry) Update(ctx context.Context, t *cognitive.Template) error {
	return nil
}

func (m *mockRegistry) Delete(ctx context.Context, id string) error {
	return nil
}

// Query Operations
func (m *mockRegistry) ListAll(ctx context.Context, statuses []cognitive.TemplateStatus) ([]*cognitive.Template, error) {
	return []*cognitive.Template{}, nil
}

func (m *mockRegistry) ListByTaskType(ctx context.Context, taskType cognitive.TaskType) ([]*cognitive.Template, error) {
	return []*cognitive.Template{}, nil
}

func (m *mockRegistry) ListByDomain(ctx context.Context, domain string) ([]*cognitive.Template, error) {
	return []*cognitive.Template{}, nil
}

func (m *mockRegistry) ListActive(ctx context.Context) ([]*cognitive.Template, error) {
	return []*cognitive.Template{}, nil
}

func (m *mockRegistry) SearchByKeywords(ctx context.Context, query string, limit int) ([]*cognitive.Template, error) {
	return []*cognitive.Template{}, nil
}

// Lifecycle Operations
func (m *mockRegistry) UpdateStatus(ctx context.Context, id string, status cognitive.TemplateStatus) error {
	return nil
}

func (m *mockRegistry) GetPromotionCandidates(ctx context.Context) ([]*cognitive.Template, error) {
	return []*cognitive.Template{}, nil
}

func (m *mockRegistry) GetDeprecationCandidates(ctx context.Context) ([]*cognitive.Template, error) {
	return []*cognitive.Template{}, nil
}

// Status Queries
func (m *mockRegistry) ListByStatus(ctx context.Context, status cognitive.TemplateStatus) ([]*cognitive.Template, error) {
	return []*cognitive.Template{}, nil
}

func (m *mockRegistry) GetTemplateMetrics(ctx context.Context, templateID string) (*cognitive.TemplateMetrics, error) {
	return nil, nil
}

func (m *mockRegistry) GetCognitiveMetrics(ctx context.Context) (*cognitive.CognitiveMetrics, error) {
	return nil, nil
}

// Usage Tracking
func (m *mockRegistry) RecordUsage(ctx context.Context, log *cognitive.UsageLog) (int64, error) {
	return 0, nil
}

func (m *mockRegistry) GetUsageLogs(ctx context.Context, templateID string, limit int) ([]*cognitive.UsageLog, error) {
	return []*cognitive.UsageLog{}, nil
}

// Grading
func (m *mockRegistry) RecordGrade(ctx context.Context, result *cognitive.GradingResult) error {
	return nil
}

func (m *mockRegistry) GetGradingLogs(ctx context.Context, templateID string) ([]*cognitive.GradingResult, error) {
	return []*cognitive.GradingResult{}, nil
}

func (m *mockRegistry) GetPendingGrades(ctx context.Context, limit int) ([]*cognitive.UsageLog, error) {
	return []*cognitive.UsageLog{}, nil
}

// Metrics
func (m *mockRegistry) GetMetrics(ctx context.Context, date string) (*cognitive.CognitiveMetrics, error) {
	return nil, nil
}

func (m *mockRegistry) IncrementMetric(ctx context.Context, metric string) error {
	return nil
}

func (m *mockRegistry) GetSystemHealth(ctx context.Context) (*cognitive.SystemHealth, error) {
	return nil, nil
}

// Embedding Cache
func (m *mockRegistry) CacheEmbedding(ctx context.Context, sourceType, sourceID, textHash string, embedding cognitive.Embedding, model string) error {
	return nil
}

func (m *mockRegistry) GetCachedEmbedding(ctx context.Context, sourceType, sourceID string) (cognitive.Embedding, error) {
	return nil, nil
}

// Distillation Tracking
func (m *mockRegistry) RecordDistillation(ctx context.Context, req *cognitive.DistillationRequest) error {
	return nil
}

func (m *mockRegistry) GetDistillationHistory(ctx context.Context, limit int) ([]*cognitive.DistillationRequest, error) {
	return []*cognitive.DistillationRequest{}, nil
}
