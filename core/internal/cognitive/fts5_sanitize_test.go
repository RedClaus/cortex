package cognitive

import (
	"testing"
)

func TestSanitizeFTS5Query(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Basic functionality
		{
			name:     "simple word",
			input:    "hello",
			expected: `"hello"`,
		},
		{
			name:     "multiple words",
			input:    "hello world",
			expected: `"hello" "world"`,
		},
		{
			name:     "words with numbers",
			input:    "test123 foo456",
			expected: `"test123" "foo456"`,
		},

		// Short word filtering (< 3 chars interpreted as column names)
		{
			name:     "filters short words - al",
			input:    "al test",
			expected: `"test"`,
		},
		{
			name:     "filters short words - cli becomes valid",
			input:    "cli test",
			expected: `"cli" "test"`,
		},
		{
			name:     "filters 2-char words",
			input:    "to be or never to be",
			expected: `"never"`,
		},
		{
			name:     "filters single characters",
			input:    "a b c test",
			expected: `"test"`,
		},
		{
			name:     "only short words returns empty",
			input:    "a to be",
			expected: "",
		},

		// Special character handling
		{
			name:     "removes comma",
			input:    "hello, world",
			expected: `"hello" "world"`,
		},
		{
			name:     "removes period",
			input:    "hello. world",
			expected: `"hello" "world"`,
		},
		{
			name:     "removes question mark",
			input:    "what is this?",
			expected: `"what" "this"`,
		},
		{
			name:     "removes exclamation",
			input:    "hello! world!",
			expected: `"hello" "world"`,
		},
		{
			name:     "removes single quotes",
			input:    "it's a test",
			expected: `"test"`,
		},
		{
			name:     "removes double quotes",
			input:    `"quoted" text`,
			expected: `"quoted" "text"`,
		},
		{
			name:     "removes asterisk",
			input:    "test* wildcard",
			expected: `"test" "wildcard"`,
		},
		{
			name:     "removes hyphen",
			input:    "test-case hyphen",
			expected: `"test" "case" "hyphen"`,
		},
		{
			name:     "removes parentheses",
			input:    "(test) function()",
			expected: `"test" "function"`,
		},
		{
			name:     "removes colon",
			input:    "column:value test",
			expected: `"column" "value" "test"`,
		},
		{
			name:     "removes at symbol",
			input:    "user@email.com test",
			expected: `"user" "email" "com" "test"`,
		},
		{
			name:     "removes caret",
			input:    "boost^2 test",
			expected: `"boost" "test"`,
		},
		{
			name:     "removes tilde",
			input:    "fuzzy~2 test",
			expected: `"fuzzy" "test"`,
		},
		{
			name:     "removes brackets",
			input:    "[array] {object}",
			expected: `"array" "object"`,
		},
		{
			name:     "removes multiple special chars",
			input:    "test!@#$%^&*() query",
			expected: `"test" "query"`,
		},

		// Boolean operator filtering
		{
			name:     "filters AND operator",
			input:    "test AND query",
			expected: `"test" "query"`,
		},
		{
			name:     "filters OR operator",
			input:    "test OR query",
			expected: `"test" "query"`,
		},
		{
			name:     "filters NOT operator",
			input:    "test NOT query",
			expected: `"test" "query"`,
		},
		{
			name:     "filters NEAR operator",
			input:    "test NEAR query",
			expected: `"test" "query"`,
		},
		{
			name:     "filters lowercase operators",
			input:    "test and or not query",
			expected: `"test" "query"`,
		},
		{
			name:     "filters mixed case operators",
			input:    "test And Or Not query",
			expected: `"test" "query"`,
		},

		// Edge cases
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only whitespace",
			input:    "   ",
			expected: "",
		},
		{
			name:     "only special characters",
			input:    "!@#$%^&*()",
			expected: "",
		},
		{
			name:     "only short words and special chars",
			input:    "a, b. c?",
			expected: "",
		},
		{
			name:     "preserves case",
			input:    "Hello World TEST",
			expected: `"Hello" "World" "TEST"`,
		},
		{
			name:     "handles unicode letters",
			input:    "cafe resume test",
			expected: `"cafe" "resume" "test"`,
		},
		{
			name:     "multiple spaces between words",
			input:    "hello    world",
			expected: `"hello" "world"`,
		},
		{
			name:     "leading and trailing spaces",
			input:    "  hello world  ",
			expected: `"hello" "world"`,
		},

		// Real-world error cases from the bug report
		{
			name:     "cli error case",
			input:    "cli",
			expected: `"cli"`,
		},
		{
			name:     "mixed short and long words",
			input:    "use cli to run",
			expected: `"use" "cli" "run"`,
		},
		{
			name:     "sentence with punctuation",
			input:    "What's the best way to do this?",
			expected: `"What" "the" "best" "way" "this"`,
		},
		{
			name:     "code-like input",
			input:    "function(arg1, arg2)",
			expected: `"function" "arg1" "arg2"`,
		},
		{
			name:     "path-like input",
			input:    "/path/to/file.txt",
			expected: `"path" "file" "txt"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeFTS5Query(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeFTS5Query(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestSanitizeFTS5QuerySafety tests that sanitized queries don't contain
// any characters that could cause FTS5 syntax errors.
func TestSanitizeFTS5QuerySafety(t *testing.T) {
	dangerousInputs := []string{
		`SELECT * FROM users`,
		`" OR "1"="1`,
		`test; DROP TABLE templates;`,
		`MATCH "test"`,
		`column:value AND other:thing`,
		`test* OR test?`,
		`(a AND b) OR (c AND d)`,
		`near/5 test`,
		`test^10`,
		`"exact phrase" NOT other`,
	}

	for _, input := range dangerousInputs {
		result := SanitizeFTS5Query(input)
		// Result should either be empty or contain only quoted terms
		// separated by spaces
		if result != "" {
			// Check that each term is properly quoted
			terms := splitQuotedTerms(result)
			for _, term := range terms {
				if len(term) < 2 || term[0] != '"' || term[len(term)-1] != '"' {
					t.Errorf("Term %q in result %q is not properly quoted", term, result)
				}
			}
		}
	}
}

// splitQuotedTerms is a helper to split a string of quoted terms.
func splitQuotedTerms(s string) []string {
	var terms []string
	inQuote := false
	start := 0

	for i, r := range s {
		if r == '"' {
			if !inQuote {
				inQuote = true
				start = i
			} else {
				inQuote = false
				terms = append(terms, s[start:i+1])
			}
		}
	}

	return terms
}
