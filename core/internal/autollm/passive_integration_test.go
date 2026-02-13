package autollm

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/normanking/cortex/internal/llm"
	"github.com/normanking/cortex/pkg/types"
)

// mockFabric implements knowledge.KnowledgeFabric for testing
type mockFabric struct {
	items []*types.KnowledgeItem
}

func (m *mockFabric) Search(ctx context.Context, query string, opts types.SearchOptions) (*types.RetrievalResult, error) {
	if len(m.items) == 0 {
		return &types.RetrievalResult{
			Items: nil,
			Tier:  types.TierStrict,
		}, nil
	}

	return &types.RetrievalResult{
		Items: m.items,
		Tier:  types.TierStrict,
	}, nil
}

func (m *mockFabric) GetByID(ctx context.Context, id string) (*types.KnowledgeItem, error) {
	return nil, nil
}

func (m *mockFabric) GetByScope(ctx context.Context, scope types.Scope) ([]*types.KnowledgeItem, error) {
	return nil, nil
}

func (m *mockFabric) Create(ctx context.Context, item *types.KnowledgeItem) error {
	return nil
}

func (m *mockFabric) Update(ctx context.Context, item *types.KnowledgeItem) error {
	return nil
}

func (m *mockFabric) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockFabric) RecordSuccess(ctx context.Context, id string) error {
	return nil
}

func (m *mockFabric) RecordFailure(ctx context.Context, id string) error {
	return nil
}

// mockProvider implements llm.Provider for testing
type mockProvider struct {
	responseContent string
	systemPrompt    string // Capture the system prompt for verification
}

func (m *mockProvider) Chat(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
	// Capture system prompt if present
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			m.systemPrompt = msg.Content
			break
		}
	}

	return &llm.ChatResponse{
		Content:    m.responseContent,
		TokensUsed: 10,
	}, nil
}

func (m *mockProvider) Name() string {
	return "mock"
}

func (m *mockProvider) Available() bool {
	return true
}

func TestPassiveRetrievalIntegration_FastLaneWithKnowledge(t *testing.T) {
	// Create mock fabric with knowledge
	fabric := &mockFabric{
		items: []*types.KnowledgeItem{
			{
				ID:         "test-1",
				Title:      "Fix Docker Build Error",
				Content:    "Run: docker build --no-cache -t myapp .",
				TrustScore: 0.9,
				Confidence: 0.9,
			},
		},
	}

	// Create router with knowledge
	config := RouterConfig{
		FastModels:  []string{"test-model"},
		SmartModels: []string{"smart-model"},
	}
	router := NewRouterWithKnowledge(config, nil, fabric)

	// Create mock provider
	mockProv := &mockProvider{
		responseContent: "Here's the solution",
	}

	// Create request that should route to Fast Lane
	req := Request{
		Prompt: "How do I fix Docker build errors?",
		Mode:   LaneFast,
	}

	// Execute
	providers := map[string]llm.Provider{
		"ollama": mockProv, // Assuming test-model uses ollama provider
	}

	ctx := context.Background()
	resp, decision, err := router.Complete(ctx, req, providers)

	// Verify
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	if decision.Lane != LaneFast {
		t.Errorf("Expected Fast Lane, got %s", decision.Lane)
	}

	if resp == nil {
		t.Fatal("Response is nil")
	}

	// Verify passive retrieval results were injected into system prompt
	if mockProv.systemPrompt == "" {
		t.Error("System prompt was not set")
	}

	// Check for passive retrieval marker (should be replaced with actual results)
	if strings.Contains(mockProv.systemPrompt, "{{PASSIVE_RETRIEVAL}}") {
		t.Error("Passive retrieval placeholder was not replaced")
	}

	// Check that knowledge was injected
	if !strings.Contains(mockProv.systemPrompt, "relevant_knowledge") {
		t.Error("Knowledge section not found in system prompt")
	}

	if !strings.Contains(mockProv.systemPrompt, "Docker") {
		t.Error("Knowledge content not found in system prompt")
	}

	t.Logf("System prompt:\n%s", mockProv.systemPrompt)
}

func TestPassiveRetrievalIntegration_FastLaneNoKnowledge(t *testing.T) {
	// Create mock fabric without knowledge
	fabric := &mockFabric{
		items: nil,
	}

	// Create router with knowledge
	config := RouterConfig{
		FastModels:  []string{"test-model"},
		SmartModels: []string{"smart-model"},
	}
	router := NewRouterWithKnowledge(config, nil, fabric)

	// Create mock provider
	mockProv := &mockProvider{
		responseContent: "Here's the solution",
	}

	// Create request that should route to Fast Lane
	req := Request{
		Prompt: "Simple question",
		Mode:   LaneFast,
	}

	// Execute
	providers := map[string]llm.Provider{
		"ollama": mockProv,
	}

	ctx := context.Background()
	resp, decision, err := router.Complete(ctx, req, providers)

	// Verify
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	if decision.Lane != LaneFast {
		t.Errorf("Expected Fast Lane, got %s", decision.Lane)
	}

	if resp == nil {
		t.Fatal("Response is nil")
	}

	// Verify placeholder was removed (no knowledge to inject)
	if strings.Contains(mockProv.systemPrompt, "{{PASSIVE_RETRIEVAL}}") {
		t.Error("Passive retrieval placeholder should be removed when no results")
	}

	// Should NOT contain knowledge section
	if strings.Contains(mockProv.systemPrompt, "relevant_knowledge") {
		t.Error("Knowledge section should not be present when no results")
	}

	t.Logf("System prompt:\n%s", mockProv.systemPrompt)
}

func TestPassiveRetrievalIntegration_SmartLaneSkipsPassive(t *testing.T) {
	// Create mock fabric with knowledge
	fabric := &mockFabric{
		items: []*types.KnowledgeItem{
			{
				ID:         "test-1",
				Title:      "Knowledge Item",
				Content:    "Some content",
				TrustScore: 0.9,
			},
		},
	}

	// Create router with knowledge
	config := RouterConfig{
		FastModels:  []string{"test-model"},
		SmartModels: []string{"smart-model"},
	}
	router := NewRouterWithKnowledge(config, nil, fabric)

	// Create mock provider
	mockProv := &mockProvider{
		responseContent: "Complex answer",
	}

	// Create request that should route to Smart Lane
	req := Request{
		Prompt: "Complex reasoning task",
		Mode:   LaneSmart, // Force smart lane
	}

	// Execute
	providers := map[string]llm.Provider{
		"ollama": mockProv, // Using ollama provider for test
	}

	ctx := context.Background()
	_, decision, err := router.Complete(ctx, req, providers)

	// Verify
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	if decision.Lane != LaneSmart {
		t.Errorf("Expected Smart Lane, got %s", decision.Lane)
	}

	// Smart Lane should NOT have passive retrieval
	if strings.Contains(mockProv.systemPrompt, "{{PASSIVE_RETRIEVAL}}") {
		t.Error("Smart Lane should not have passive retrieval placeholder")
	}

	if strings.Contains(mockProv.systemPrompt, "relevant_knowledge") {
		t.Error("Smart Lane should not have passive retrieval knowledge injected")
	}

	t.Logf("System prompt (Smart Lane):\n%s", mockProv.systemPrompt)
}

func TestPassiveRetrievalIntegration_Timeout(t *testing.T) {
	// Create slow fabric that simulates timeout
	fabric := &slowFabric{
		delay: 100 * time.Millisecond, // Exceeds 50ms timeout
	}

	config := RouterConfig{
		FastModels:  []string{"test-model"},
		SmartModels: []string{"smart-model"},
	}
	router := NewRouterWithKnowledge(config, nil, fabric)

	mockProv := &mockProvider{
		responseContent: "Response",
	}

	req := Request{
		Prompt: "Question",
		Mode:   LaneFast,
	}

	providers := map[string]llm.Provider{
		"ollama": mockProv,
	}

	ctx := context.Background()
	resp, _, err := router.Complete(ctx, req, providers)

	// Should succeed despite timeout (passive retrieval is optional)
	if err != nil {
		t.Fatalf("Complete should succeed even if passive retrieval times out: %v", err)
	}

	if resp == nil {
		t.Fatal("Response should not be nil")
	}

	// Placeholder should be removed even after timeout
	if strings.Contains(mockProv.systemPrompt, "{{PASSIVE_RETRIEVAL}}") {
		t.Error("Placeholder should be removed after timeout")
	}

	t.Log("Passive retrieval timeout handled gracefully")
}

// slowFabric simulates a slow knowledge fabric
type slowFabric struct {
	delay time.Duration
}

func (s *slowFabric) Search(ctx context.Context, query string, opts types.SearchOptions) (*types.RetrievalResult, error) {
	time.Sleep(s.delay)
	return &types.RetrievalResult{Items: nil, Tier: types.TierStrict}, nil
}

func (s *slowFabric) GetByID(ctx context.Context, id string) (*types.KnowledgeItem, error) {
	return nil, nil
}

func (s *slowFabric) GetByScope(ctx context.Context, scope types.Scope) ([]*types.KnowledgeItem, error) {
	return nil, nil
}

func (s *slowFabric) Create(ctx context.Context, item *types.KnowledgeItem) error {
	return nil
}

func (s *slowFabric) Update(ctx context.Context, item *types.KnowledgeItem) error {
	return nil
}

func (s *slowFabric) Delete(ctx context.Context, id string) error {
	return nil
}

func (s *slowFabric) RecordSuccess(ctx context.Context, id string) error {
	return nil
}

func (s *slowFabric) RecordFailure(ctx context.Context, id string) error {
	return nil
}
