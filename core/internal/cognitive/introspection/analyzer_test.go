package introspection

import (
	"context"
	"testing"

	"github.com/normanking/cortex/internal/memory"
	"github.com/stretchr/testify/assert"
)

type mockLLMProvider struct {
	response string
	err      error
}

func (m *mockLLMProvider) Complete(ctx context.Context, prompt string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

func TestGapAnalyzer_DetermineGapSeverity(t *testing.T) {
	analyzer := &GapAnalyzer{}

	tests := []struct {
		name          string
		storedCount   int
		llmCanAnswer  bool
		llmConfidence float64
		expectedSev   GapSeverity
	}{
		{
			name:          "none - extensive stored knowledge",
			storedCount:   15,
			llmCanAnswer:  true,
			llmConfidence: 0.9,
			expectedSev:   GapSeverityNone,
		},
		{
			name:          "minimal - some stored knowledge",
			storedCount:   5,
			llmCanAnswer:  true,
			llmConfidence: 0.8,
			expectedSev:   GapSeverityMinimal,
		},
		{
			name:          "moderate - no stored but high LLM confidence",
			storedCount:   0,
			llmCanAnswer:  true,
			llmConfidence: 0.8,
			expectedSev:   GapSeverityModerate,
		},
		{
			name:          "severe - no stored, low LLM confidence",
			storedCount:   0,
			llmCanAnswer:  false,
			llmConfidence: 0.5,
			expectedSev:   GapSeveritySevere,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := &GapAnalysis{
				StoredKnowledgeCount: tt.storedCount,
				HasStoredKnowledge:   tt.storedCount > 0,
				LLMCanAnswer:         tt.llmCanAnswer,
				LLMConfidence:        tt.llmConfidence,
			}
			severity := analyzer.determineGapSeverity(analysis)
			assert.Equal(t, tt.expectedSev, severity)
		})
	}
}

func TestGapAnalyzer_GenerateAcquisitionOptions(t *testing.T) {
	analyzer := &GapAnalyzer{}

	tests := []struct {
		subject       string
		minOptions    int
		expectedTypes []string
	}{
		{
			subject:       "python programming",
			minOptions:    2,
			expectedTypes: []string{"web_search", "documentation_crawl"},
		},
		{
			subject:       "internal project code",
			minOptions:    1,
			expectedTypes: []string{"file_ingest"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.subject, func(t *testing.T) {
			options := analyzer.generateAcquisitionOptions(tt.subject)
			assert.GreaterOrEqual(t, len(options), tt.minOptions)

			optionTypes := make([]string, len(options))
			for i, opt := range options {
				optionTypes[i] = string(opt.Type)
			}
			for _, expected := range tt.expectedTypes {
				assert.Contains(t, optionTypes, expected)
			}
		})
	}
}

func TestGapAnalyzer_DetermineRecommendedAction(t *testing.T) {
	analyzer := &GapAnalyzer{}

	tests := []struct {
		name           string
		severity       GapSeverity
		hasStored      bool
		llmCanAnswer   bool
		expectedAction string
	}{
		{
			name:           "use stored knowledge",
			severity:       GapSeverityNone,
			hasStored:      true,
			llmCanAnswer:   true,
			expectedAction: "use_stored_knowledge",
		},
		{
			name:           "supplement with LLM",
			severity:       GapSeverityMinimal,
			hasStored:      true,
			llmCanAnswer:   true,
			expectedAction: "supplement_with_llm",
		},
		{
			name:           "offer LLM and acquisition",
			severity:       GapSeverityModerate,
			hasStored:      false,
			llmCanAnswer:   true,
			expectedAction: "offer_llm_and_acquisition",
		},
		{
			name:           "recommend acquisition",
			severity:       GapSeveritySevere,
			hasStored:      false,
			llmCanAnswer:   false,
			expectedAction: "recommend_acquisition",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := &GapAnalysis{
				GapSeverity:        tt.severity,
				HasStoredKnowledge: tt.hasStored,
				LLMCanAnswer:       tt.llmCanAnswer,
			}
			action := analyzer.determineRecommendedAction(analysis)
			assert.Equal(t, tt.expectedAction, action)
		})
	}
}

func TestGapAnalyzer_Analyze(t *testing.T) {
	mockLLM := &mockLLMProvider{
		response: `{"can_answer": true, "confidence": 0.8, "explanation": "I have good knowledge of this"}`,
	}
	analyzer := NewGapAnalyzer(mockLLM)

	query := &IntrospectionQuery{
		Type:    QueryTypeKnowledgeCheck,
		Subject: "golang",
	}

	inventory := &memory.InventoryResult{
		TotalMatches: 5,
		TopResults: []memory.InventoryItem{
			{ID: "1", Content: "Go programming basics"},
		},
	}

	analysis, err := analyzer.Analyze(context.Background(), query, inventory)
	assert.NoError(t, err)
	assert.Equal(t, "golang", analysis.Subject)
	assert.Equal(t, 5, analysis.StoredKnowledgeCount)
	assert.True(t, analysis.HasStoredKnowledge)
	assert.True(t, analysis.LLMCanAnswer)
	assert.NotEmpty(t, analysis.RecommendedAction)
}
