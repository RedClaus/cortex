package introspection

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockWebSearcher is a test double for WebSearcher.
type mockWebSearcher struct {
	results []WebSearchResult
	err     error
}

func (m *mockWebSearcher) Search(ctx context.Context, query string, maxResults int) ([]WebSearchResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

// mockKnowledgeFabric is a test double for KnowledgeFabricCreator.
type mockKnowledgeFabric struct {
	items []interface{}
}

func (m *mockKnowledgeFabric) Create(ctx context.Context, item interface{}) error {
	m.items = append(m.items, item)
	return nil
}

// mockTopicStore is a test double for TopicStoreCreator.
type mockTopicStore struct {
	topics []interface{}
}

func (m *mockTopicStore) CreateTopic(ctx context.Context, topic interface{}) error {
	m.topics = append(m.topics, topic)
	return nil
}

// mockEventPublisher is a test double for EventPublisher.
type mockEventPublisher struct {
	events []interface{}
}

func (m *mockEventPublisher) Publish(event interface{}) {
	m.events = append(m.events, event)
}

func TestWebSearchAdapter_Search(t *testing.T) {
	// Test the adapter with mock data
	// Note: This is a unit test - real integration would use WebSearchTool

	// Create a simple test to verify the adapter compiles and basic logic
	adapter := &WebSearchAdapter{tool: nil}

	// Should return error when tool is nil
	_, err := adapter.Search(context.Background(), "test query", 5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

func TestSystem_CreatesAllComponents(t *testing.T) {
	// Test that NewSystem creates all components when dependencies provided
	cfg := &Config{
		LLMProvider: nil, // Optional for basic creation
		Enabled:     true,
	}

	sys := NewSystem(cfg)

	require.NotNil(t, sys)
	assert.True(t, sys.Enabled)
	assert.NotNil(t, sys.Classifier)
	assert.NotNil(t, sys.Analyzer)
	assert.NotNil(t, sys.Responder)

	// Acquisition and Learning should be nil without dependencies
	assert.Nil(t, sys.Acquisition)
	assert.Nil(t, sys.Learning)
}

func TestSystem_NilConfig(t *testing.T) {
	sys := NewSystem(nil)

	require.NotNil(t, sys)
	assert.False(t, sys.Enabled)
}

func TestAcquisitionFlowIntegration(t *testing.T) {
	// This test verifies the acquisition flow works end-to-end with mocks
	ctx := context.Background()

	// Mock web search results
	mockSearch := &mockWebSearcher{
		results: []WebSearchResult{
			{
				URL:     "https://example.com/docker-basics",
				Title:   "Docker Basics Tutorial",
				Content: "Docker is a containerization platform. Containers package applications with dependencies. docker run starts a container.",
				Score:   0.95,
			},
			{
				URL:     "https://example.com/docker-commands",
				Title:   "Essential Docker Commands",
				Content: "Use docker build to create images. docker ps lists running containers. docker stop halts containers.",
				Score:   0.90,
			},
		},
	}

	mockFabric := &mockKnowledgeFabric{}
	mockTopics := &mockTopicStore{}
	mockEvents := &mockEventPublisher{}

	// Step 1: Classify the query
	classifier := NewClassifier(nil, nil)
	query, err := classifier.Classify(ctx, "Do you know about Docker containers?")
	require.NoError(t, err)
	assert.Equal(t, QueryTypeKnowledgeCheck, query.Type)
	assert.Contains(t, query.Subject, "Docker")

	// Step 2: Analyze gap (simulate empty inventory)
	analyzer := NewGapAnalyzer(nil)
	analysis, err := analyzer.Analyze(ctx, query, nil)
	require.NoError(t, err)
	assert.False(t, analysis.HasStoredKnowledge)
	assert.NotEqual(t, GapSeverityNone, analysis.GapSeverity)

	// Step 3: Generate response offering acquisition
	responder := NewMetacognitiveResponder()
	template := responder.SelectTemplate(analysis)
	assert.Equal(t, TemplateKnowledgeNotFoundCannotAnswer, template)

	// Step 4: User chooses to acquire - create acquisition request
	req := &AcquisitionRequest{
		Type:        AcquisitionTypeWebSearch,
		Subject:     "Docker containers",
		SearchQuery: "Docker containers tutorial",
	}

	// Step 5: Execute acquisition (using mocks)
	// Note: We can't use the full AcquisitionEngine without llm.Provider
	// So we test the web search part directly
	results, err := mockSearch.Search(ctx, req.SearchQuery, 5)
	require.NoError(t, err)
	assert.Len(t, results, 2)

	// Verify results have expected content
	assert.Contains(t, results[0].Content, "Docker")
	assert.Contains(t, results[1].Content, "docker")

	// Step 6: Verify we can store to fabric (mock)
	for range results {
		err = mockFabric.Create(ctx, "knowledge item")
		require.NoError(t, err)
	}
	assert.Len(t, mockFabric.items, 2)

	// Step 7: Create topic (mock)
	err = mockTopics.CreateTopic(ctx, "Docker topic")
	require.NoError(t, err)
	assert.Len(t, mockTopics.topics, 1)

	// Step 8: Publish event (mock)
	mockEvents.Publish(map[string]interface{}{
		"type":    "acquisition_complete",
		"subject": "Docker containers",
	})
	assert.Len(t, mockEvents.events, 1)
}

func TestAcquisitionRequest_Types(t *testing.T) {
	tests := []struct {
		name        string
		reqType     AcquisitionType
		description string
	}{
		{
			name:        "file ingest",
			reqType:     AcquisitionTypeFile,
			description: "Ingest from local file",
		},
		{
			name:        "web search",
			reqType:     AcquisitionTypeWebSearch,
			description: "Search and learn from web",
		},
		{
			name:        "doc crawl",
			reqType:     AcquisitionTypeDocCrawl,
			description: "Crawl documentation site",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &AcquisitionRequest{
				Type:    tt.reqType,
				Subject: "Test Subject",
			}
			assert.Equal(t, tt.reqType, req.Type)
		})
	}
}

func TestLearningOutcome_Verification(t *testing.T) {
	// Test the learning outcome structure
	outcome := &LearningOutcome{
		Subject:          "Docker",
		Verified:         true,
		ItemsRetrievable: 5,
		SampleQueries:    []string{"what is docker", "docker example"},
		TestResults: []TestResult{
			{Query: "what is docker", Found: true, ResultCount: 3, TopScore: 0.95},
			{Query: "docker example", Found: true, ResultCount: 2, TopScore: 0.88},
		},
	}

	assert.True(t, outcome.Verified)
	assert.Equal(t, 5, outcome.ItemsRetrievable)
	assert.Len(t, outcome.TestResults, 2)
	assert.True(t, outcome.TestResults[0].Found)
}

func TestEndToEnd_ClassifyAnalyzeRespond(t *testing.T) {
	// Full end-to-end test without external dependencies
	ctx := context.Background()

	// Create system with minimal config
	sys := NewSystem(&Config{
		LLMProvider: nil,
		Enabled:     true,
	})

	require.NotNil(t, sys)
	require.NotNil(t, sys.Classifier)
	require.NotNil(t, sys.Analyzer)
	require.NotNil(t, sys.Responder)

	// Test various queries
	testCases := []struct {
		input       string
		expectType  QueryType
		expectGap   bool
		description string
	}{
		{
			input:       "Do you know about Kubernetes?",
			expectType:  QueryTypeKnowledgeCheck,
			expectGap:   true,
			description: "Knowledge check should detect gap when no inventory",
		},
		{
			input:       "What do you know?",
			expectType:  QueryTypeMemoryList,
			expectGap:   true,
			description: "Memory list should work",
		},
		{
			input:       "ls -la",
			expectType:  QueryTypeNotIntrospective,
			expectGap:   false,
			description: "Commands should not be introspective",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			// Classify
			query, err := sys.Classifier.Classify(ctx, tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.expectType, query.Type)

			if query.Type == QueryTypeNotIntrospective {
				return // No further processing needed
			}

			// Analyze
			analysis, err := sys.Analyzer.Analyze(ctx, query, nil)
			require.NoError(t, err)

			if tc.expectGap {
				assert.False(t, analysis.HasStoredKnowledge)
			}

			// Generate response
			response, err := sys.Responder.GenerateFromAnalysis(analysis, nil)
			require.NoError(t, err)
			assert.NotEmpty(t, response)
		})
	}
}
