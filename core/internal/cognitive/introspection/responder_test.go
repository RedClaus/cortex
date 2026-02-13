package introspection

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetacognitiveResponder_SelectTemplate(t *testing.T) {
	responder := NewMetacognitiveResponder()

	tests := []struct {
		name         string
		analysis     *GapAnalysis
		expectedTmpl ResponseTemplate
	}{
		{
			name: "knowledge found",
			analysis: &GapAnalysis{
				HasStoredKnowledge: true,
				LLMCanAnswer:       true,
				LLMConfidence:      0.9,
			},
			expectedTmpl: TemplateKnowledgeFound,
		},
		{
			name: "not found but can answer",
			analysis: &GapAnalysis{
				HasStoredKnowledge: false,
				LLMCanAnswer:       true,
				LLMConfidence:      0.8,
			},
			expectedTmpl: TemplateKnowledgeNotFoundCanAnswer,
		},
		{
			name: "not found cannot answer",
			analysis: &GapAnalysis{
				HasStoredKnowledge: false,
				LLMCanAnswer:       false,
				LLMConfidence:      0.3,
			},
			expectedTmpl: TemplateKnowledgeNotFoundCannotAnswer,
		},
		{
			name: "not found low confidence",
			analysis: &GapAnalysis{
				HasStoredKnowledge: false,
				LLMCanAnswer:       true,
				LLMConfidence:      0.5,
			},
			expectedTmpl: TemplateKnowledgeNotFoundCannotAnswer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := responder.SelectTemplate(tt.analysis)
			assert.Equal(t, tt.expectedTmpl, tmpl)
		})
	}
}

func TestMetacognitiveResponder_Generate(t *testing.T) {
	responder := NewMetacognitiveResponder()

	tests := []struct {
		name          string
		template      ResponseTemplate
		context       *ResponseContext
		shouldContain []string
	}{
		{
			name:     "knowledge found response",
			template: TemplateKnowledgeFound,
			context: &ResponseContext{
				Subject:    "Docker",
				MatchCount: 5,
				TopResults: []InventoryItem{
					{Summary: "Docker container basics", Source: "knowledge_fabric"},
				},
			},
			shouldContain: []string{"Docker", "5 items"},
		},
		{
			name:     "not found can answer response",
			template: TemplateKnowledgeNotFoundCanAnswer,
			context: &ResponseContext{
				Subject:       "Kubernetes",
				LLMCanAnswer:  true,
				LLMConfidence: 0.85,
			},
			shouldContain: []string{"Kubernetes", "0 items", "85%"},
		},
		{
			name:     "not found cannot answer response",
			template: TemplateKnowledgeNotFoundCannotAnswer,
			context: &ResponseContext{
				Subject: "ObscureTopic",
				AcquisitionOptions: []AcquisitionOption{
					{Type: "web_search", Description: "Search the web"},
				},
			},
			shouldContain: []string{"ObscureTopic", "Search the web"},
		},
		{
			name:     "acquisition started response",
			template: TemplateAcquisitionStarted,
			context: &ResponseContext{
				Subject:         "Python",
				AcquisitionType: "documentation_crawl",
			},
			shouldContain: []string{"Python", "documentation_crawl"},
		},
		{
			name:     "acquisition complete response",
			template: TemplateAcquisitionComplete,
			context: &ResponseContext{
				Subject:       "Go",
				ItemsIngested: 15,
				Categories:    []string{"programming", "concurrency"},
			},
			shouldContain: []string{"Go", "15", "programming"},
		},
		{
			name:     "acquisition failed response",
			template: TemplateAcquisitionFailed,
			context: &ResponseContext{
				Subject:      "FailingTopic",
				ErrorMessage: "connection timeout",
			},
			shouldContain: []string{"FailingTopic", "connection timeout"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := responder.Generate(tt.template, tt.context)
			require.NoError(t, err)
			for _, expected := range tt.shouldContain {
				assert.Contains(t, response, expected)
			}
		})
	}
}

func TestMetacognitiveResponder_Generate_InvalidTemplate(t *testing.T) {
	responder := NewMetacognitiveResponder()
	_, err := responder.Generate(ResponseTemplate("invalid"), &ResponseContext{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "template not found")
}
