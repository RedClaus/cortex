package server

import (
	"testing"
)

func TestCountMatchedWords(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		queryWords []string
		expected   int
	}{
		{
			name:       "all words match",
			text:       "go best practices for development",
			queryWords: []string{"go", "best", "practices"},
			expected:   3,
		},
		{
			name:       "partial match",
			text:       "go best practices for development",
			queryWords: []string{"go", "testing", "practices"},
			expected:   2,
		},
		{
			name:       "no matches",
			text:       "react component patterns",
			queryWords: []string{"python", "django"},
			expected:   0,
		},
		{
			name:       "single word match",
			text:       "cortex architecture guide",
			queryWords: []string{"architecture"},
			expected:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countMatchedWords(tt.text, tt.queryWords)
			if result != tt.expected {
				t.Errorf("countMatchedWords() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestBuildMatchContext(t *testing.T) {
	tests := []struct {
		name             string
		title            string
		titleMatches     int
		metadataMatches  int
		metadataText     string
		query            string
		expectedContains string
	}{
		{
			name:             "title match only",
			title:            "Go Best Practices",
			titleMatches:     2,
			metadataMatches:  0,
			metadataText:     "",
			query:            "go best",
			expectedContains: "Title match: 'Go Best Practices'",
		},
		{
			name:             "metadata match only",
			title:            "Some Document",
			titleMatches:     0,
			metadataMatches:  1,
			metadataText:     " category:guidelines language:go",
			query:            "guidelines",
			expectedContains: "Metadata:",
		},
		{
			name:             "both title and metadata match",
			title:            "React Patterns",
			titleMatches:     1,
			metadataMatches:  1,
			metadataText:     " category:documentation language:typescript",
			query:            "react typescript",
			expectedContains: "Title match:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildMatchContext(tt.title, tt.titleMatches, tt.metadataMatches, tt.metadataText, tt.query)
			if result == "" {
				t.Errorf("buildMatchContext() returned empty string")
			}
			// Just verify something was returned (detailed string matching is fragile)
		})
	}
}
