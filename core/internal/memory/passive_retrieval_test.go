package memory

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/normanking/cortex/pkg/types"
)

// mockFabric implements knowledge.KnowledgeFabric for testing.
type mockFabric struct {
	searchFunc func(ctx context.Context, query string, opts types.SearchOptions) (*types.RetrievalResult, error)
	items      []*types.KnowledgeItem
	delay      time.Duration // Simulate slow search
}

func (m *mockFabric) Search(ctx context.Context, query string, opts types.SearchOptions) (*types.RetrievalResult, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if m.searchFunc != nil {
		return m.searchFunc(ctx, query, opts)
	}

	// Default: return mock items
	result := &types.RetrievalResult{
		Items: m.items,
		Tier:  types.TierFuzzy,
	}
	return result, nil
}

func (m *mockFabric) GetByID(ctx context.Context, id string) (*types.KnowledgeItem, error) {
	return nil, nil
}
func (m *mockFabric) GetByScope(ctx context.Context, scope types.Scope) ([]*types.KnowledgeItem, error) {
	return nil, nil
}
func (m *mockFabric) Create(ctx context.Context, item *types.KnowledgeItem) error { return nil }
func (m *mockFabric) Update(ctx context.Context, item *types.KnowledgeItem) error { return nil }
func (m *mockFabric) Delete(ctx context.Context, id string) error                 { return nil }
func (m *mockFabric) RecordSuccess(ctx context.Context, id string) error          { return nil }
func (m *mockFabric) RecordFailure(ctx context.Context, id string) error          { return nil }

// TestPassiveRetriever_BasicSearch tests basic search functionality.
func TestPassiveRetriever_BasicSearch(t *testing.T) {
	fabric := &mockFabric{
		items: []*types.KnowledgeItem{
			{
				ID:         "item-1",
				Title:      "Kubernetes Ingress Fix",
				Content:    "Use kubectl rollout restart deployment/ingress-nginx to fix 503 errors",
				TrustScore: 0.85,
			},
			{
				ID:         "item-2",
				Title:      "Pod Log Commands",
				Content:    "Use kubectl logs -f <pod> to follow pod logs",
				TrustScore: 0.75,
			},
		},
	}

	retriever := NewPassiveRetriever(fabric, DefaultPassiveRetrievalConfig())
	ctx := context.Background()

	results, err := retriever.Retrieve(ctx, "how to fix kubernetes ingress 503", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	// First result should be the ingress fix
	if !strings.Contains(results[0].Summary, "Kubernetes Ingress Fix") {
		t.Errorf("first result should be ingress fix, got: %s", results[0].Summary)
	}

	// Check confidence is preserved
	if results[0].Confidence != 0.85 {
		t.Errorf("expected confidence 0.85, got %.2f", results[0].Confidence)
	}
}

// TestPassiveRetriever_Disabled tests that disabled retriever returns nothing.
func TestPassiveRetriever_Disabled(t *testing.T) {
	fabric := &mockFabric{
		items: []*types.KnowledgeItem{
			{ID: "item-1", Title: "Test", Content: "Test content", TrustScore: 0.9},
		},
	}

	config := DefaultPassiveRetrievalConfig()
	config.Enabled = false

	retriever := NewPassiveRetriever(fabric, config)
	results, _ := retriever.Retrieve(context.Background(), "test query", "")

	if len(results) != 0 {
		t.Error("disabled retriever should return no results")
	}
}

// TestPassiveRetriever_ShortQuery tests that very short queries are skipped.
func TestPassiveRetriever_ShortQuery(t *testing.T) {
	fabric := &mockFabric{
		items: []*types.KnowledgeItem{
			{ID: "item-1", Title: "Test", Content: "Test content", TrustScore: 0.9},
		},
	}

	retriever := NewPassiveRetriever(fabric, DefaultPassiveRetrievalConfig())

	// Query too short
	results, _ := retriever.Retrieve(context.Background(), "hi", "")
	if len(results) != 0 {
		t.Error("short query should return no results")
	}

	// Empty query
	results, _ = retriever.Retrieve(context.Background(), "", "")
	if len(results) != 0 {
		t.Error("empty query should return no results")
	}

	// Whitespace only
	results, _ = retriever.Retrieve(context.Background(), "   ", "")
	if len(results) != 0 {
		t.Error("whitespace query should return no results")
	}
}

// TestPassiveRetriever_Timeout tests timeout handling.
func TestPassiveRetriever_Timeout(t *testing.T) {
	fabric := &mockFabric{
		items: []*types.KnowledgeItem{
			{ID: "item-1", Title: "Test", Content: "Test content", TrustScore: 0.9},
		},
		delay: 200 * time.Millisecond, // Much longer than timeout
	}

	config := DefaultPassiveRetrievalConfig()
	config.TimeoutMs = 10 // Very short timeout

	retriever := NewPassiveRetriever(fabric, config)
	results, err := retriever.Retrieve(context.Background(), "test query that times out", "")

	// Should fail silently
	if err != nil {
		t.Error("timeout should not return error")
	}
	if len(results) != 0 {
		t.Error("timeout should return no results")
	}

	// Metrics should show timeout
	metrics := retriever.Metrics()
	if metrics.TotalTimeouts != 1 {
		t.Errorf("expected 1 timeout, got %d", metrics.TotalTimeouts)
	}
}

// TestPassiveRetriever_NoResults tests handling of no results.
func TestPassiveRetriever_NoResults(t *testing.T) {
	fabric := &mockFabric{
		items: []*types.KnowledgeItem{}, // Empty results
	}

	retriever := NewPassiveRetriever(fabric, DefaultPassiveRetrievalConfig())
	results, _ := retriever.Retrieve(context.Background(), "query with no matches", "")

	if len(results) != 0 {
		t.Error("should return empty results")
	}

	// Metrics should show miss
	metrics := retriever.Metrics()
	if metrics.TotalMisses != 1 {
		t.Errorf("expected 1 miss, got %d", metrics.TotalMisses)
	}
}

// TestPassiveRetriever_TokenBudget tests token budget enforcement.
func TestPassiveRetriever_TokenBudget(t *testing.T) {
	// Create items with lots of content
	longContent := strings.Repeat("word ", 500) // ~500 words
	fabric := &mockFabric{
		items: []*types.KnowledgeItem{
			{ID: "item-1", Title: "Long Item 1", Content: longContent, TrustScore: 0.9},
			{ID: "item-2", Title: "Long Item 2", Content: longContent, TrustScore: 0.8},
			{ID: "item-3", Title: "Long Item 3", Content: longContent, TrustScore: 0.7},
		},
	}

	config := DefaultPassiveRetrievalConfig()
	config.MaxTokensToAdd = 100 // Low budget - should only fit 1 result

	retriever := NewPassiveRetriever(fabric, config)
	results, _ := retriever.Retrieve(context.Background(), "query with lots of results", "")

	// With low token budget, should only get 1-2 results
	if len(results) > 2 {
		t.Errorf("token budget should limit results, got %d", len(results))
	}
}

// TestPassiveRetriever_ResultFormatting tests different content formats.
func TestPassiveRetriever_ResultFormatting(t *testing.T) {
	fabric := &mockFabric{
		items: []*types.KnowledgeItem{
			{
				ID:         "code-item",
				Title:      "Git Command",
				Content:    "Use this command:\n```bash\ngit stash pop\n```\nto restore stashed changes",
				TrustScore: 0.9,
			},
			{
				ID:         "fix-item",
				Title:      "Fix for CORS Error",
				Content:    "Add Access-Control-Allow-Origin header to your server response",
				TrustScore: 0.85,
			},
			{
				ID:         "normal-item",
				Title:      "Docker Best Practice",
				Content:    "Use multi-stage builds to reduce image size",
				TrustScore: 0.8,
			},
		},
	}

	retriever := NewPassiveRetriever(fabric, DefaultPassiveRetrievalConfig())
	results, _ := retriever.Retrieve(context.Background(), "test formatting", "")

	// Code item should extract code block
	if !strings.Contains(results[0].Summary, "git stash pop") {
		t.Errorf("code block should be extracted, got: %s", results[0].Summary)
	}

	// Fix item should be labeled as "Known fix"
	if !strings.Contains(results[1].Summary, "Known fix") {
		t.Errorf("fix item should be labeled, got: %s", results[1].Summary)
	}
}

// TestPassiveRetriever_InjectIntoContext tests context injection.
func TestPassiveRetriever_InjectIntoContext(t *testing.T) {
	retriever := NewPassiveRetriever(&mockFabric{}, DefaultPassiveRetrievalConfig())

	systemPrompt := `You are an AI assistant.

{{PASSIVE_RETRIEVAL}}

Help the user with their questions.`

	// Test with no results
	result := retriever.InjectIntoContext(systemPrompt, nil)
	if strings.Contains(result, "{{PASSIVE_RETRIEVAL}}") {
		t.Error("placeholder should be removed when no results")
	}
	if strings.Contains(result, "relevant_knowledge") {
		t.Error("should not add knowledge tags when no results")
	}

	// Test with results
	results := []PassiveResult{
		{ID: "1", Summary: "First knowledge item", Confidence: 0.9},
		{ID: "2", Summary: "Second knowledge item", Confidence: 0.8},
	}

	result = retriever.InjectIntoContext(systemPrompt, results)
	if strings.Contains(result, "{{PASSIVE_RETRIEVAL}}") {
		t.Error("placeholder should be replaced")
	}
	if !strings.Contains(result, "<relevant_knowledge>") {
		t.Error("should contain relevant_knowledge tags")
	}
	if !strings.Contains(result, "First knowledge item") {
		t.Error("should contain first result")
	}
	if !strings.Contains(result, "Second knowledge item") {
		t.Error("should contain second result")
	}
}

// TestPassiveRetriever_Metrics tests metrics tracking.
func TestPassiveRetriever_Metrics(t *testing.T) {
	fabric := &mockFabric{
		items: []*types.KnowledgeItem{
			{ID: "item-1", Title: "Test", Content: "Test content", TrustScore: 0.9},
		},
	}

	retriever := NewPassiveRetriever(fabric, DefaultPassiveRetrievalConfig())
	ctx := context.Background()

	// Perform searches
	retriever.Retrieve(ctx, "query with results", "")
	retriever.Retrieve(ctx, "another query with results", "")

	// Remove items to simulate no results
	fabric.items = nil
	retriever.Retrieve(ctx, "query with no results now", "")

	metrics := retriever.Metrics()

	if metrics.TotalSearches != 3 {
		t.Errorf("expected 3 total searches, got %d", metrics.TotalSearches)
	}
	if metrics.TotalHits != 2 {
		t.Errorf("expected 2 hits, got %d", metrics.TotalHits)
	}
	if metrics.TotalMisses != 1 {
		t.Errorf("expected 1 miss, got %d", metrics.TotalMisses)
	}

	// Hit rate should be ~66%
	hitRate := retriever.HitRate()
	if hitRate < 60 || hitRate > 70 {
		t.Errorf("expected hit rate around 66%%, got %.2f%%", hitRate)
	}
}

// TestPassiveRetriever_RetrieveWithSummary tests the debug summary.
func TestPassiveRetriever_RetrieveWithSummary(t *testing.T) {
	fabric := &mockFabric{
		items: []*types.KnowledgeItem{
			{ID: "item-1", Title: "Test Item", Content: "Test content here", TrustScore: 0.9},
		},
	}

	retriever := NewPassiveRetriever(fabric, DefaultPassiveRetrievalConfig())

	results, summary := retriever.RetrieveWithSummary(context.Background(), "test query for summary", "")

	if len(results) == 0 {
		t.Error("should return results")
	}

	if summary.ResultCount != len(results) {
		t.Errorf("summary count mismatch: %d vs %d", summary.ResultCount, len(results))
	}

	if summary.LatencyMs < 0 {
		t.Error("latency should be non-negative")
	}

	if summary.TopConfidence != 0.9 {
		t.Errorf("expected top confidence 0.9, got %.2f", summary.TopConfidence)
	}
}

// TestPassiveRetriever_ContentTruncation tests long content is truncated.
func TestPassiveRetriever_ContentTruncation(t *testing.T) {
	veryLongContent := strings.Repeat("This is a very long sentence. ", 100)
	fabric := &mockFabric{
		items: []*types.KnowledgeItem{
			{
				ID:         "long-item",
				Title:      "Long Content Item",
				Content:    veryLongContent,
				TrustScore: 0.9,
			},
		},
	}

	retriever := NewPassiveRetriever(fabric, DefaultPassiveRetrievalConfig())
	results, _ := retriever.Retrieve(context.Background(), "test long content", "")

	if len(results) == 0 {
		t.Fatal("should return results")
	}

	// Summary should be truncated
	if len(results[0].Summary) > 300 {
		t.Errorf("summary should be truncated, got %d characters", len(results[0].Summary))
	}

	// Should end with "..." if truncated
	if !strings.HasSuffix(results[0].Summary, "...") {
		t.Error("truncated content should end with ...")
	}
}
