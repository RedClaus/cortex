package introspection

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifier_Classify_PatternMatching(t *testing.T) {
	classifier := NewClassifier(nil, nil)

	tests := []struct {
		name          string
		input         string
		expectedType  QueryType
		expectedSubj  string
		minConfidence float64
	}{
		{
			name:          "knowledge check - do you know",
			input:         "Do you know about Docker containers?",
			expectedType:  QueryTypeKnowledgeCheck,
			expectedSubj:  "Docker containers",
			minConfidence: 0.8,
		},
		{
			name:          "knowledge check - what do you know",
			input:         "What do you know about kubernetes?",
			expectedType:  QueryTypeKnowledgeCheck,
			expectedSubj:  "kubernetes",
			minConfidence: 0.8,
		},
		{
			name:          "knowledge check - is in memory",
			input:         "Is golang in your memory?",
			expectedType:  QueryTypeKnowledgeCheck,
			expectedSubj:  "",
			minConfidence: 0.8,
		},
		{
			name:          "knowledge check - have you learned",
			input:         "Have you learned about Python?",
			expectedType:  QueryTypeKnowledgeCheck,
			expectedSubj:  "Python",
			minConfidence: 0.8,
		},
		{
			name:          "capability check - can you help",
			input:         "Can you help with debugging?",
			expectedType:  QueryTypeCapabilityCheck,
			expectedSubj:  "debugging",
			minConfidence: 0.8,
		},
		{
			name:          "capability check - are you able",
			input:         "Are you able to write unit tests?",
			expectedType:  QueryTypeCapabilityCheck,
			expectedSubj:  "write unit tests",
			minConfidence: 0.8,
		},
		{
			name:          "memory list - what do you know",
			input:         "What do you know?",
			expectedType:  QueryTypeMemoryList,
			expectedSubj:  "",
			minConfidence: 0.8,
		},
		{
			name:          "memory list - list knowledge",
			input:         "List your knowledge",
			expectedType:  QueryTypeMemoryList,
			expectedSubj:  "",
			minConfidence: 0.8,
		},
		{
			name:          "skill assessment - how good",
			input:         "How good are you at Go programming?",
			expectedType:  QueryTypeSkillAssessment,
			expectedSubj:  "Go programming",
			minConfidence: 0.8,
		},
		{
			name:          "not introspective - regular question",
			input:         "What is the capital of France?",
			expectedType:  QueryTypeNotIntrospective,
			expectedSubj:  "",
			minConfidence: 0.9,
		},
		{
			name:          "not introspective - command",
			input:         "ls -la",
			expectedType:  QueryTypeNotIntrospective,
			expectedSubj:  "",
			minConfidence: 0.9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := classifier.Classify(context.Background(), tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedType, result.Type)
			if tt.expectedSubj != "" {
				assert.Contains(t, result.Subject, tt.expectedSubj)
			}
			assert.GreaterOrEqual(t, result.Confidence, tt.minConfidence)
			assert.Equal(t, tt.input, result.OriginalQuery)
		})
	}
}

func TestExpandSearchTerms(t *testing.T) {
	tests := []struct {
		subject       string
		shouldContain []string
	}{
		{
			subject:       "linux commands",
			shouldContain: []string{"linux commands", "bash", "shell", "unix"},
		},
		{
			subject:       "docker setup",
			shouldContain: []string{"docker setup", "container"},
		},
		{
			subject:       "git workflow",
			shouldContain: []string{"git workflow", "version control"},
		},
		{
			subject:       "python scripting",
			shouldContain: []string{"python scripting", "py", "python3"},
		},
		{
			subject:       "kubernetes deployment",
			shouldContain: []string{"kubernetes deployment", "k8s", "kube"},
		},
		{
			subject:       "random topic",
			shouldContain: []string{"random topic"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.subject, func(t *testing.T) {
			terms := expandSearchTerms(tt.subject)
			for _, expected := range tt.shouldContain {
				assert.Contains(t, terms, expected)
			}
		})
	}
}

func TestSeemsPotentiallyIntrospective(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"do you know about X", true},
		{"what do you think", true},
		{"have you learned", true},
		{"can you help", true},
		{"your memory contains", true},
		{"ls -la", false},
		{"compile the code", false},
		{"run tests", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := seemsPotentiallyIntrospective(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
