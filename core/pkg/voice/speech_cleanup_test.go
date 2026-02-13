// Package voice provides voice-related types and utilities for Cortex.
// speech_cleanup_test.go provides tests for STT output normalization.
package voice

import (
	"strings"
	"testing"
)

func TestSpeechCleaner_Clean(t *testing.T) {
	cleaner := NewSpeechCleaner()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "artifact prefix removal",
			input:    "very internet and get me the latest headlines from new york city",
			expected: "Get me the latest headlines from new york city",
		},
		{
			name:     "wake word extraction",
			input:    "hey henry what's the weather in miami",
			expected: "What's the weather in miami",
		},
		{
			name:     "filler word removal",
			input:    "um uh so like can you help me with this",
			expected: "Can you help me with this",
		},
		{
			name:     "repeated word cleanup",
			input:    "the the weather today",
			expected: "Weather today",
		},
		{
			name:     "clean input unchanged",
			input:    "get me the latest headlines",
			expected: "Get me the latest headlines",
		},
		{
			name:     "okay prefix removal",
			input:    "okay so get me the news",
			expected: "Get me the news",
		},
		{
			name:     "multiple artifacts",
			input:    "uh very and get me the stock prices",
			expected: "Get me the stock prices",
		},
		{
			name:     "command starter extraction",
			input:    "noise noise show me the calendar",
			expected: "Show me the calendar",
		},
		{
			name:     "and connector removal",
			input:    "something and find me a restaurant",
			expected: "Find me a restaurant",
		},
		{
			name:     "question preservation",
			input:    "what is the capital of france",
			expected: "What is the capital of france",
		},
		{
			name:     "cortex wake word",
			input:    "hey cortex, tell me a joke",
			expected: "Tell me a joke",
		},
		{
			name:     "berry misheard as very",
			input:    "berry get me the headlines",
			expected: "Get me the headlines",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only filler words",
			input:    "um uh like so and",
			expected: "",
		},
		{
			name:     "mixed case preservation",
			input:    "tell me about NEW YORK",
			expected: "Tell me about NEW YORK",
		},
		{
			name:     "sorry and prefix",
			input:    "sorry and can you help me",
			expected: "Can you help me",
		},
		{
			name:     "alright prefix",
			input:    "alright please find me a hotel",
			expected: "Please find me a hotel",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleaner.Clean(tt.input)
			if result != tt.expected {
				t.Errorf("Clean(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSpeechCleaner_RealWorldExamples(t *testing.T) {
	cleaner := NewSpeechCleaner()

	// Real-world STT artifacts observed in production
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "user reported issue",
			input:    "very internet and get me the latest headlines from new york city",
			expected: "Get me the latest headlines from new york city",
		},
		{
			name:     "background noise start",
			input:    "and and what time is it",
			expected: "What time is it",
		},
		{
			name:     "false start with retry",
			input:    "no wait search for pizza places nearby",
			expected: "Search for pizza places nearby",
		},
		{
			name:     "thinking sounds",
			input:    "hmm let me see um find me flights to chicago",
			expected: "Find me flights to chicago",
		},
		{
			name:     "very prefix",
			input:    "very get me news",
			expected: "Get me news",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleaner.Clean(tt.input)
			if result != tt.expected {
				t.Errorf("Clean(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSpeechCleaner_PreservesValidInput(t *testing.T) {
	cleaner := NewSpeechCleaner()

	// These should be preserved (not over-cleaned)
	tests := []struct {
		name  string
		input string
	}{
		{"simple question", "What is the weather"},
		{"polite request", "Please help me with this"},
		{"search query", "Search for restaurants in boston"},
		{"imperative", "Open my calendar"},
		{"how question", "How do I reset my password"},
		{"why question", "Why is the sky blue"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleaner.Clean(tt.input)
			// Result should be similar (just capitalization fixed)
			if len(result) < len(tt.input)/2 {
				t.Errorf("Clean(%q) = %q - too much was removed", tt.input, result)
			}
		})
	}
}

func TestSpeechCleaner_AddCustomPatterns(t *testing.T) {
	cleaner := NewSpeechCleaner()

	// Add custom artifact
	cleaner.AddArtifactPrefix("blah blah")

	result := cleaner.Clean("blah blah show me the news")
	expected := "Show me the news"

	if result != expected {
		t.Errorf("Custom artifact not handled: got %q, want %q", result, expected)
	}

	// Add custom wake word
	cleaner.AddWakeWord("yo assistant")

	result = cleaner.Clean("yo assistant what time is it")
	expected = "What time is it"

	if result != expected {
		t.Errorf("Custom wake word not handled: got %q, want %q", result, expected)
	}
}

func TestCleanTranscription(t *testing.T) {
	// Test convenience function
	result := CleanTranscription("very internet and get me the news")
	if result != "Get me the news" {
		t.Errorf("CleanTranscription failed: got %q", result)
	}
}

func TestSpeechCleaner_WakeWords(t *testing.T) {
	cleaner := NewSpeechCleaner()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "hey henry wake word",
			input:    "hey henry what time is it",
			expected: "What time is it",
		},
		{
			name:     "hey hannah wake word",
			input:    "hey hannah what's the weather",
			expected: "What's the weather",
		},
		{
			name:     "hi henry wake word",
			input:    "hi henry show me the calendar",
			expected: "Show me the calendar",
		},
		{
			name:     "hi hannah wake word",
			input:    "hi hannah find me a restaurant",
			expected: "Find me a restaurant",
		},
		{
			name:     "hannah only",
			input:    "hannah get me the news",
			expected: "Get me the news",
		},
		{
			name:     "henry only",
			input:    "henry search for flights",
			expected: "Search for flights",
		},
		{
			name:     "cortex wake word still works",
			input:    "hey cortex help me with this",
			expected: "Help me with this",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleaner.Clean(tt.input)
			if result != tt.expected {
				t.Errorf("Clean(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestSpeechCleaner_MisheardCorrections tests CR-012-C misheard word corrections
func TestSpeechCleaner_MisheardCorrections(t *testing.T) {
	cleaner := NewSpeechCleaner()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Single-word corrections only apply to short inputs (<=3 words)
		{
			name:     "yes misheard as guess (short input)",
			input:    "guess",
			expected: "Yes",
		},
		{
			name:     "stop misheard as stock (short input)",
			input:    "stock",
			expected: "Stop",
		},
		{
			name:     "stock prices should NOT be corrected (long input)",
			input:    "get me the stock prices please",
			expected: "Get me the stock prices please",
		},
		// Short multi-word phrases get corrections
		{
			name:     "go head correction (2 words)",
			input:    "go head",
			expected: "Go ahead",
		},
		// Single word with punctuation preserved
		{
			name:     "stop with period",
			input:    "stock.",
			expected: "Stop.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleaner.Clean(tt.input)
			if result != tt.expected {
				t.Errorf("Clean(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}

	// Test that multi-word corrections are applied (check result contains correction)
	result := cleaner.Clean("i never seen")
	if !strings.Contains(strings.ToLower(result), "proceed") {
		t.Errorf("Multi-word correction not working: Clean(\"i never seen\") = %q, should contain 'proceed'", result)
	}

	// Test AddMisheardCorrection
	cleaner.AddMisheardCorrection("custom phrase", "fixed")
	result = cleaner.Clean("custom phrase")
	if !strings.Contains(strings.ToLower(result), "fixed") {
		t.Errorf("AddMisheardCorrection not working: got %q", result)
	}
}

func BenchmarkSpeechCleaner_Clean(b *testing.B) {
	cleaner := NewSpeechCleaner()
	input := "very internet and get me the latest headlines from new york city"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cleaner.Clean(input)
	}
}
