package voice

import (
	"context"
	"strings"
	"testing"
)

// ═══════════════════════════════════════════════════════════════════════════════
// MOCK LLM CLIENT FOR TESTING
// ═══════════════════════════════════════════════════════════════════════════════

type mockLLMClient struct {
	response    string
	streamDelay bool
}

func (m *mockLLMClient) ChatStream(ctx context.Context, messages []LLMMessage, systemPrompt string) (<-chan StreamToken, error) {
	ch := make(chan StreamToken, 100)

	go func() {
		defer close(ch)

		// Split response into tokens (words)
		words := strings.Fields(m.response)
		for i, word := range words {
			select {
			case <-ctx.Done():
				ch <- StreamToken{Error: ctx.Err()}
				return
			default:
			}

			if i > 0 {
				ch <- StreamToken{Token: " "}
			}
			ch <- StreamToken{Token: word}
		}

		ch <- StreamToken{Done: true}
	}()

	return ch, nil
}

func (m *mockLLMClient) Chat(ctx context.Context, messages []LLMMessage, systemPrompt string) (string, error) {
	return m.response, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// PROMPT INJECTOR TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestPromptInjector_TextMode(t *testing.T) {
	detector := NewModeDetector()
	injector := NewPromptInjector(detector, "Base prompt")

	// In text mode (default), should return base prompt unchanged
	prompt := injector.BuildSystemPrompt()
	if prompt != "Base prompt" {
		t.Errorf("Expected base prompt in text mode, got: %s", prompt)
	}

	// Few-shot messages should be nil in text mode
	messages := injector.BuildFewShotMessages()
	if messages != nil {
		t.Errorf("Expected nil few-shot messages in text mode, got: %d messages", len(messages))
	}
}

func TestPromptInjector_VoiceMode(t *testing.T) {
	detector := NewModeDetector()
	injector := NewPromptInjector(detector, "Base prompt")

	// Enable voice mode
	detector.SetExplicitMode(ModeVoice)

	// Should include voice system prompt
	prompt := injector.BuildSystemPrompt()
	if !strings.Contains(prompt, "SPOKEN conversation") {
		t.Error("Voice prompt should mention spoken conversation")
	}
	if !strings.Contains(prompt, "Base prompt") {
		t.Error("Voice prompt should include base prompt")
	}
	if !strings.Contains(prompt, "Voice Response Rules") {
		t.Error("Voice prompt should include voice response rules")
	}

	// Few-shot messages should be present in voice mode
	messages := injector.BuildFewShotMessages()
	if len(messages) == 0 {
		t.Error("Expected few-shot messages in voice mode")
	}

	// Verify message structure
	for i, msg := range messages {
		if msg.Role != "user" && msg.Role != "assistant" {
			t.Errorf("Message %d has invalid role: %s", i, msg.Role)
		}
		if msg.Content == "" {
			t.Errorf("Message %d has empty content", i)
		}
	}
}

func TestPromptInjector_ModeSwitching(t *testing.T) {
	detector := NewModeDetector()
	injector := NewPromptInjector(detector, "Test prompt")

	// Start in text mode
	if injector.IsVoiceMode() {
		t.Error("Should start in text mode")
	}

	// Switch to voice mode
	detector.SetExplicitMode(ModeVoice)
	if !injector.IsVoiceMode() {
		t.Error("Should be in voice mode after setting explicit mode")
	}

	// Switch back to text mode
	detector.ClearExplicitMode()
	if injector.IsVoiceMode() {
		t.Error("Should be in text mode after clearing explicit mode")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// VOICE-AWARE LLM TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestVoiceAwareLLM_SetVoiceMode(t *testing.T) {
	client := &mockLLMClient{response: "Hello world."}
	llm := NewVoiceAwareLLM(client, "Base prompt")

	// Should start in text mode
	if llm.IsVoiceMode() {
		t.Error("Should start in text mode")
	}

	// Enable voice mode
	llm.SetVoiceMode(true)
	if !llm.IsVoiceMode() {
		t.Error("Should be in voice mode after SetVoiceMode(true)")
	}

	// Disable voice mode
	llm.SetVoiceMode(false)
	if llm.IsVoiceMode() {
		t.Error("Should be in text mode after SetVoiceMode(false)")
	}
}

func TestVoiceAwareLLM_AutomaticModeDetection(t *testing.T) {
	client := &mockLLMClient{response: "Test response."}
	llm := NewVoiceAwareLLM(client, "Base prompt")

	// Voice mode requires both STT active AND TTS enabled
	llm.OnSTTActive(true)
	if llm.IsVoiceMode() {
		t.Error("Should not be in voice mode with only STT active")
	}

	llm.OnTTSEnabled(true)
	if !llm.IsVoiceMode() {
		t.Error("Should be in voice mode with STT active AND TTS enabled")
	}

	// Disabling either should exit voice mode
	llm.OnSTTActive(false)
	if llm.IsVoiceMode() {
		t.Error("Should not be in voice mode when STT is inactive")
	}
}

func TestVoiceAwareLLM_Chat(t *testing.T) {
	client := &mockLLMClient{response: "This is a test response."}
	llm := NewVoiceAwareLLM(client, "Base prompt")

	ctx := context.Background()
	messages := []LLMMessage{{Role: "user", Content: "Hello"}}

	response, err := llm.Chat(ctx, messages)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if response != "This is a test response." {
		t.Errorf("Unexpected response: %s", response)
	}
}

func TestVoiceAwareLLM_StreamResponse(t *testing.T) {
	client := &mockLLMClient{response: "This is a test. And another sentence."}
	llm := NewVoiceAwareLLM(client, "Base prompt")

	// Enable voice mode for sentence extraction
	llm.SetVoiceMode(true)

	ctx := context.Background()
	messages := []LLMMessage{{Role: "user", Content: "Hello"}}

	visualCh, voiceCh, errCh := llm.StreamResponse(ctx, messages)

	// Collect visual tokens
	var visualTokens []string
	for token := range visualCh {
		visualTokens = append(visualTokens, token)
	}

	// Collect voice sentences
	var voiceSentences []string
	for sentence := range voiceCh {
		voiceSentences = append(voiceSentences, sentence)
	}

	// Check for errors
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	default:
	}

	// Should have received visual tokens
	if len(visualTokens) == 0 {
		t.Error("Expected visual tokens")
	}

	// Should have received voice sentences in voice mode
	if len(voiceSentences) == 0 {
		t.Error("Expected voice sentences in voice mode")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// TEXT PROCESSING TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestSanitizeForTTS(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain text",
			input:    "Hello world",
			expected: "Hello world",
		},
		{
			name:     "remove markdown bold",
			input:    "This is **bold** text",
			expected: "This is bold text",
		},
		{
			name:     "remove markdown italic",
			input:    "This is *italic* text",
			expected: "This is italic text",
		},
		{
			name:     "remove inline code",
			input:    "Use `go build` to compile",
			expected: "Use to compile",
		},
		{
			name:     "remove code blocks",
			input:    "Here is code:\n```go\nfunc main() {}\n```\nEnd",
			expected: "Here is code: End",
		},
		{
			name:     "convert symbols",
			input:    "Temperature is 72°F",
			expected: "Temperature is 72 degrees Fahrenheit",
		},
		{
			name:     "convert percent",
			input:    "Disk is 50% full",
			expected: "Disk is 50 percent full",
		},
		{
			name:     "remove markdown link",
			input:    "Visit [Google](https://google.com)",
			expected: "Visit Google",
		},
		{
			name:     "remove bullet points",
			input:    "- First item\n- Second item",
			expected: "First item Second item",
		},
		{
			name:     "remove headers",
			input:    "## Title\nContent",
			expected: "Title Content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeForTTS(tt.input)
			// Normalize whitespace for comparison
			result = strings.Join(strings.Fields(result), " ")
			expected := strings.Join(strings.Fields(tt.expected), " ")
			if result != expected {
				t.Errorf("SanitizeForTTS(%q) = %q, want %q", tt.input, result, expected)
			}
		})
	}
}

func TestSplitIntoSentences(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single sentence",
			input:    "Hello world.",
			expected: []string{"Hello world."},
		},
		{
			name:     "multiple sentences",
			input:    "First sentence. Second sentence. Third sentence!",
			expected: []string{"First sentence.", "Second sentence.", "Third sentence!"},
		},
		{
			name:     "question marks",
			input:    "What is this? This is that.",
			expected: []string{"What is this?", "This is that."},
		},
		{
			name:     "abbreviations",
			input:    "Mr. Smith went home. Dr. Jones stayed.",
			expected: []string{"Mr. Smith went home.", "Dr. Jones stayed."},
		},
		{
			name:     "no ending punctuation",
			input:    "Hello world",
			expected: []string{"Hello world"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SplitIntoSentences(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("SplitIntoSentences(%q) returned %d sentences, want %d", tt.input, len(result), len(tt.expected))
				return
			}
			for i, s := range result {
				if s != tt.expected[i] {
					t.Errorf("Sentence %d: got %q, want %q", i, s, tt.expected[i])
				}
			}
		})
	}
}

func TestIsReadableText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "plain english",
			input:    "Hello, how are you?",
			expected: true,
		},
		{
			name:     "code snippet",
			input:    "x := map[string][]int{} && arr[0] != nil",
			expected: false,
		},
		{
			name:     "special characters",
			input:    "<<<>>>|||\\\\///",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "numbers and text",
			input:    "The answer is 42.",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsReadableText(tt.input)
			if result != tt.expected {
				t.Errorf("IsReadableText(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// VOICE PROMPTS TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestVoiceFewShotExamples(t *testing.T) {
	if len(VoiceFewShotExamples) == 0 {
		t.Error("VoiceFewShotExamples should not be empty")
	}

	for i, example := range VoiceFewShotExamples {
		if example.Query == "" {
			t.Errorf("Example %d has empty query", i)
		}
		if example.VoiceResponse == "" {
			t.Errorf("Example %d has empty response", i)
		}

		// Voice responses should not contain markdown
		if strings.Contains(example.VoiceResponse, "```") {
			t.Errorf("Example %d response contains code block", i)
		}
		if strings.Contains(example.VoiceResponse, "**") {
			t.Errorf("Example %d response contains bold markdown", i)
		}
	}
}

func TestGetVoiceFewShotMessages(t *testing.T) {
	messages := GetVoiceFewShotMessages()

	// Should return pairs of user/assistant messages
	if len(messages)%2 != 0 {
		t.Errorf("Few-shot messages should be in pairs, got %d messages", len(messages))
	}

	// Verify alternating roles
	for i := 0; i < len(messages); i += 2 {
		if messages[i].Role != "user" {
			t.Errorf("Message %d should be user role, got %s", i, messages[i].Role)
		}
		if i+1 < len(messages) && messages[i+1].Role != "assistant" {
			t.Errorf("Message %d should be assistant role, got %s", i+1, messages[i+1].Role)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// INJECTOR UTILITY FUNCTION TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestInjectVoicePrompt(t *testing.T) {
	basePrompt := "You are a helpful assistant."

	// Text mode - should return base prompt unchanged
	result := InjectVoicePrompt(basePrompt, false)
	if result != basePrompt {
		t.Errorf("InjectVoicePrompt in text mode should return base prompt unchanged")
	}

	// Voice mode - should include voice instructions
	result = InjectVoicePrompt(basePrompt, true)
	if !strings.Contains(result, "SPOKEN conversation") {
		t.Error("Voice prompt should mention spoken conversation")
	}
	if !strings.Contains(result, basePrompt) {
		t.Error("Voice prompt should include base prompt")
	}
}

func TestGetVoicePromptOnly(t *testing.T) {
	prompt := GetVoicePromptOnly()

	if !strings.Contains(prompt, "SPOKEN conversation") {
		t.Error("Voice prompt should mention spoken conversation")
	}
	if !strings.Contains(prompt, "Voice Response Rules") {
		t.Error("Voice prompt should include voice response rules")
	}
	if !strings.Contains(prompt, "NEVER output code blocks") {
		t.Error("Voice prompt should include code guidance")
	}
}
