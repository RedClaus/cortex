package router_test

import (
	"context"
	"fmt"

	"github.com/normanking/cortex/internal/cognitive"
	"github.com/normanking/cortex/internal/cognitive/router"
)

// ExampleRouter demonstrates basic router usage.
func ExampleRouter() {
	// Create embedder (will check Ollama availability)
	embedder := router.NewOllamaEmbedder(&router.OllamaEmbedderConfig{
		Host:     "http://127.0.0.1:11434",
		Model:    "nomic-embed-text",
		AutoPull: false, // Don't auto-pull in example
	})

	// Create router with default thresholds
	r := router.NewRouter(&router.RouterConfig{
		Embedder:   embedder,
		Registry:   &exampleRegistry{},
		Thresholds: router.DefaultThresholds(),
	})

	ctx := context.Background()

	// Initialize router
	if err := r.Initialize(ctx); err != nil {
		fmt.Printf("Failed to initialize: %v\n", err)
		return
	}

	// Route a request
	result, err := r.Route(ctx, "fix authentication bug")
	if err != nil {
		fmt.Printf("Failed to route: %v\n", err)
		return
	}

	// Check the routing decision
	switch result.Decision {
	case cognitive.RouteTemplate:
		fmt.Printf("Matched template: %s (similarity: %.2f)\n",
			result.Match.Template.Name,
			result.Match.SimilarityScore)
		fmt.Printf("Recommended tier: %s\n", result.RecommendedTier)

	case cognitive.RouteNovel:
		fmt.Println("Novel request - using frontier model")
		fmt.Printf("Recommended model: %s\n", result.RecommendedModel)

	case cognitive.RouteFallback:
		fmt.Println("Using fallback (embedder unavailable)")
	}
}

// ExampleEmbeddingIndex demonstrates vector search.
func ExampleEmbeddingIndex() {
	index := router.NewEmbeddingIndex()

	// Create some example embeddings
	e1 := make(cognitive.Embedding, 768)
	e1[0] = 1.0

	e2 := make(cognitive.Embedding, 768)
	e2[1] = 1.0

	e3 := make(cognitive.Embedding, 768)
	e3[0] = 0.9
	e3[1] = 0.1

	// Add to index
	index.Add("doc1", e1, "First document")
	index.Add("doc2", e2, "Second document")
	index.Add("doc3", e3, "Third document")

	// Search for similar documents
	query := make(cognitive.Embedding, 768)
	query[0] = 1.0 // Similar to e1

	results := index.Search(query, 3)

	fmt.Printf("Found %d results:\n", len(results))
	for i, result := range results {
		fmt.Printf("%d. %s (score: %.2f)\n", i+1, result.ID, result.Score)
	}

	// Output:
	// Found 3 results:
	// 1. doc1 (score: 1.00)
	// 2. doc3 (score: 0.99)
	// 3. doc2 (score: 0.00)
}

// ExampleCosineSimilarity demonstrates similarity computation.
func ExampleEmbedding_CosineSimilarity() {
	// Create two embeddings
	a := cognitive.Embedding{1.0, 0.0, 0.0}
	b := cognitive.Embedding{0.9, 0.1, 0.0}

	// Compute similarity
	similarity := a.CosineSimilarity(b)

	fmt.Printf("Similarity: %.2f\n", similarity)
	// Output: Similarity: 0.99
}

// ExampleThresholds demonstrates threshold-based routing.
func ExampleThresholds() {
	// Get default thresholds
	thresholds := router.DefaultThresholds()

	fmt.Printf("High threshold: %.2f\n", thresholds.High)
	fmt.Printf("Medium threshold: %.2f\n", thresholds.Medium)
	fmt.Printf("Low threshold: %.2f\n", thresholds.Low)

	// Classify a similarity score
	score := 0.75

	level := cognitive.GetSimilarityLevel(score)
	fmt.Printf("Score %.2f → %s\n", score, level)

	// Output:
	// High threshold: 0.85
	// Medium threshold: 0.70
	// Low threshold: 0.50
	// Score 0.75 → medium
}

// ═══════════════════════════════════════════════════════════════════════════════
// EXAMPLE REGISTRY
// ═══════════════════════════════════════════════════════════════════════════════

type exampleRegistry struct{}

func (r *exampleRegistry) ListActive(ctx context.Context) ([]*cognitive.Template, error) {
	// Return some example templates
	return []*cognitive.Template{
		{
			ID:              "auth-fix",
			Name:            "Authentication Bug Fix",
			Intent:          "fix authentication or login bug",
			ConfidenceScore: 0.85,
			Status:          cognitive.StatusPromoted,
		},
		{
			ID:              "perf-opt",
			Name:            "Performance Optimization",
			Intent:          "optimize performance or improve speed",
			ConfidenceScore: 0.75,
			Status:          cognitive.StatusValidated,
		},
	}, nil
}

func (r *exampleRegistry) SearchByKeywords(ctx context.Context, query string, limit int) ([]*cognitive.Template, error) {
	return []*cognitive.Template{}, nil
}

func (r *exampleRegistry) Create(ctx context.Context, t *cognitive.Template) error              { return nil }
func (r *exampleRegistry) Get(ctx context.Context, id string) (*cognitive.Template, error)      { return nil, nil }
func (r *exampleRegistry) Update(ctx context.Context, t *cognitive.Template) error              { return nil }
func (r *exampleRegistry) Delete(ctx context.Context, id string) error                          { return nil }
func (r *exampleRegistry) ListAll(ctx context.Context, statuses []cognitive.TemplateStatus) ([]*cognitive.Template, error) {
	return []*cognitive.Template{}, nil
}
func (r *exampleRegistry) ListByTaskType(ctx context.Context, taskType cognitive.TaskType) ([]*cognitive.Template, error) {
	return []*cognitive.Template{}, nil
}
func (r *exampleRegistry) ListByDomain(ctx context.Context, domain string) ([]*cognitive.Template, error) {
	return []*cognitive.Template{}, nil
}
func (r *exampleRegistry) UpdateStatus(ctx context.Context, id string, status cognitive.TemplateStatus) error {
	return nil
}
func (r *exampleRegistry) GetPromotionCandidates(ctx context.Context) ([]*cognitive.Template, error) {
	return []*cognitive.Template{}, nil
}
func (r *exampleRegistry) GetDeprecationCandidates(ctx context.Context) ([]*cognitive.Template, error) {
	return []*cognitive.Template{}, nil
}
func (r *exampleRegistry) ListByStatus(ctx context.Context, status cognitive.TemplateStatus) ([]*cognitive.Template, error) {
	return []*cognitive.Template{}, nil
}
func (r *exampleRegistry) GetTemplateMetrics(ctx context.Context, templateID string) (*cognitive.TemplateMetrics, error) {
	return nil, nil
}
func (r *exampleRegistry) GetCognitiveMetrics(ctx context.Context) (*cognitive.CognitiveMetrics, error) {
	return nil, nil
}
func (r *exampleRegistry) RecordUsage(ctx context.Context, log *cognitive.UsageLog) (int64, error) {
	return 0, nil
}
func (r *exampleRegistry) GetUsageLogs(ctx context.Context, templateID string, limit int) ([]*cognitive.UsageLog, error) {
	return []*cognitive.UsageLog{}, nil
}
func (r *exampleRegistry) RecordGrade(ctx context.Context, result *cognitive.GradingResult) error {
	return nil
}
func (r *exampleRegistry) GetGradingLogs(ctx context.Context, templateID string) ([]*cognitive.GradingResult, error) {
	return []*cognitive.GradingResult{}, nil
}
func (r *exampleRegistry) GetPendingGrades(ctx context.Context, limit int) ([]*cognitive.UsageLog, error) {
	return []*cognitive.UsageLog{}, nil
}
func (r *exampleRegistry) GetMetrics(ctx context.Context, date string) (*cognitive.CognitiveMetrics, error) {
	return nil, nil
}
func (r *exampleRegistry) IncrementMetric(ctx context.Context, metric string) error { return nil }
func (r *exampleRegistry) GetSystemHealth(ctx context.Context) (*cognitive.SystemHealth, error) {
	return nil, nil
}
func (r *exampleRegistry) CacheEmbedding(ctx context.Context, sourceType, sourceID, textHash string, embedding cognitive.Embedding, model string) error {
	return nil
}
func (r *exampleRegistry) GetCachedEmbedding(ctx context.Context, sourceType, sourceID string) (cognitive.Embedding, error) {
	return nil, nil
}
func (r *exampleRegistry) RecordDistillation(ctx context.Context, req *cognitive.DistillationRequest) error {
	return nil
}
func (r *exampleRegistry) GetDistillationHistory(ctx context.Context, limit int) ([]*cognitive.DistillationRequest, error) {
	return []*cognitive.DistillationRequest{}, nil
}
