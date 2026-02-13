package voice

import (
	"strings"
	"testing"
)

func TestVoiceResponseFormatter_Dates(t *testing.T) {
	f := DefaultVoiceResponseFormatter()

	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "ISO date",
			input:    "Meeting on 2025-01-15",
			contains: "January 15", // Year may be converted by number formatter
		},
		{
			name:     "Slash date",
			input:    "Due by 1/15/25",
			contains: "January 15",
		},
		{
			name:     "24-hour time",
			input:    "Call at 15:30",
			contains: "3:30 PM",
		},
		{
			name:     "Midnight",
			input:    "Deadline is 00:00",
			contains: "12 AM",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := f.Format(tc.input)
			if !strings.Contains(result, tc.contains) {
				t.Errorf("Format(%q) = %q, want to contain %q", tc.input, result, tc.contains)
			}
		})
	}
}

func TestVoiceResponseFormatter_Numbers(t *testing.T) {
	f := DefaultVoiceResponseFormatter()

	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "Thousands",
			input:    "Found 2048 files",
			contains: "thousand",
		},
		{
			name:     "Millions",
			input:    "Size: 1500000 bytes",
			contains: "million",
		},
		{
			name:     "Small numbers unchanged",
			input:    "Only 100 items",
			contains: "100",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := f.Format(tc.input)
			if !strings.Contains(result, tc.contains) {
				t.Errorf("Format(%q) = %q, want to contain %q", tc.input, result, tc.contains)
			}
		})
	}
}

func TestStripMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Bold asterisks",
			input:    "This is **bold** text",
			expected: "This is bold text",
		},
		{
			name:     "Italic asterisks",
			input:    "This is *italic* text",
			expected: "This is italic text",
		},
		{
			name:     "Inline code",
			input:    "Run `git status` now",
			expected: "Run git status now",
		},
		{
			name:     "Headers",
			input:    "## Section Title\nContent here",
			expected: "Section Title\nContent here",
		},
		{
			name:     "Links",
			input:    "Check [this link](https://example.com) out",
			expected: "Check this link out",
		},
		{
			name:     "Bullet points",
			input:    "- Item one\n- Item two",
			expected: "Item one\nItem two",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := StripMarkdown(tc.input)
			if result != tc.expected {
				t.Errorf("StripMarkdown(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestHandleCodeBlocks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "Short code block",
			input:    "Here's the code:\n```bash\ngit status\n```",
			contains: "git status",
		},
		{
			name:     "Long code block",
			input:    "Here's the code:\n```go\nfunc main() {\n    fmt.Println(\"hello\")\n    fmt.Println(\"world\")\n    return\n}\n```",
			contains: "Code block with",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := HandleCodeBlocks(tc.input)
			if !strings.Contains(result, tc.contains) {
				t.Errorf("HandleCodeBlocks() = %q, want to contain %q", result, tc.contains)
			}
		})
	}
}

func TestVoiceResponseFormatter_LimitSentences(t *testing.T) {
	f := DefaultVoiceResponseFormatter()
	f.MaxSentences = 2

	input := "First sentence. Second sentence. Third sentence. Fourth sentence."
	result := f.Format(input)

	// Should only have 2 sentences
	sentences := strings.Split(result, ". ")
	if len(sentences) > 2 {
		t.Errorf("Expected max 2 sentences, got %d: %q", len(sentences), result)
	}
}

func TestVoiceResponseFormatter_AddAcknowledgment(t *testing.T) {
	f := DefaultVoiceResponseFormatter()

	tests := []struct {
		ackType  string
		expected string
	}{
		{"confirm", "Got it. "},
		{"working", "On it. "},
		{"done", "Done. "},
		{"found", "Found it. "},
		{"error", "Hmm. "},
		{"checking", "Let me check. "},
	}

	for _, tc := range tests {
		t.Run(tc.ackType, func(t *testing.T) {
			result := f.AddAcknowledgment("Test message", tc.ackType)
			if !strings.HasPrefix(result, tc.expected) {
				t.Errorf("AddAcknowledgment(%q) = %q, want prefix %q", tc.ackType, result, tc.expected)
			}
		})
	}
}

func TestFormatForTTS(t *testing.T) {
	input := "On **2025-01-15**, we found 2048 files in `/usr/local/bin/app/data/config`."
	result := FormatForTTS(input)

	// Should have markdown stripped
	if strings.Contains(result, "**") {
		t.Error("Expected markdown to be stripped")
	}

	// Should have date converted
	if !strings.Contains(result, "January") {
		t.Error("Expected date to be converted")
	}

	// Should have number converted
	if !strings.Contains(result, "thousand") {
		t.Error("Expected large number to be converted")
	}

	// Should have path simplified
	if strings.Contains(result, "/usr/local/bin") {
		t.Error("Expected long path to be simplified")
	}
}
