package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// ═══════════════════════════════════════════════════════════════════════════════
// MOCK BACKEND FOR TESTING
// ═══════════════════════════════════════════════════════════════════════════════

type MockBackend struct {
	models   []ModelInfo
	sessions []SessionInfo
}

func (m *MockBackend) SendMessage(content string) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk)
	close(ch)
	return ch, nil
}

func (m *MockBackend) StreamChannel() <-chan StreamChunk {
	ch := make(chan StreamChunk)
	close(ch)
	return ch
}

func (m *MockBackend) CancelStream() error {
	return nil
}

func (m *MockBackend) GetModels() ([]ModelInfo, error) {
	return m.models, nil
}

func (m *MockBackend) GetSessions() ([]SessionInfo, error) {
	return m.sessions, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// COMMAND ROUTER TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestHandleCommand_Help(t *testing.T) {
	backend := &MockBackend{}

	tests := []struct {
		input string
		want  string
	}{
		{"/help", "ShowHelpMsg"},
		{"/h", "ShowHelpMsg"},
		{"/?", "ShowHelpMsg"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			cmd := HandleCommand(tt.input, backend)
			msg := cmd()

			if _, ok := msg.(ShowHelpMsg); !ok {
				t.Errorf("HandleCommand(%q) returned %T, want ShowHelpMsg", tt.input, msg)
			}
		})
	}
}

func TestHandleCommand_Clear(t *testing.T) {
	backend := &MockBackend{}

	tests := []string{"/clear", "/c"}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			cmd := HandleCommand(input, backend)
			msg := cmd()

			if _, ok := msg.(ClearHistoryMsg); !ok {
				t.Errorf("HandleCommand(%q) returned %T, want ClearHistoryMsg", input, msg)
			}
		})
	}
}

func TestHandleCommand_Yolo(t *testing.T) {
	backend := &MockBackend{}

	cmd := HandleCommand("/yolo", backend)
	msg := cmd()

	if _, ok := msg.(ToggleYoloMsg); !ok {
		t.Errorf("HandleCommand(/yolo) returned %T, want ToggleYoloMsg", msg)
	}
}

func TestHandleCommand_Plan(t *testing.T) {
	backend := &MockBackend{}

	cmd := HandleCommand("/plan", backend)
	msg := cmd()

	if _, ok := msg.(TogglePlanMsg); !ok {
		t.Errorf("HandleCommand(/plan) returned %T, want TogglePlanMsg", msg)
	}
}

func TestHandleCommand_Quit(t *testing.T) {
	backend := &MockBackend{}

	tests := []string{"/quit", "/q", "/exit"}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			cmd := HandleCommand(input, backend)
			msg := cmd()

			// tea.Quit returns tea.QuitMsg
			if _, ok := msg.(tea.QuitMsg); !ok {
				t.Errorf("HandleCommand(%q) returned %T, want tea.QuitMsg", input, msg)
			}
		})
	}
}

func TestHandleCommand_Unknown(t *testing.T) {
	backend := &MockBackend{}

	tests := []string{"/unknown", "/invalid", "/xyz"}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			cmd := HandleCommand(input, backend)
			msg := cmd()

			cmdErr, ok := msg.(CommandErrorMsg)
			if !ok {
				t.Errorf("HandleCommand(%q) returned %T, want CommandErrorMsg", input, msg)
				return
			}

			if !strings.Contains(cmdErr.Error, "Unknown command") {
				t.Errorf("Expected error to contain 'Unknown command', got: %s", cmdErr.Error)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// MODEL COMMAND TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestHandleCommand_Model_NoArgs(t *testing.T) {
	backend := &MockBackend{}

	cmd := HandleCommand("/model", backend)
	msg := cmd()

	if _, ok := msg.(ShowModelSelectorMsg); !ok {
		t.Errorf("HandleCommand(/model) returned %T, want ShowModelSelectorMsg", msg)
	}
}

func TestHandleCommand_Model_WithValidModel(t *testing.T) {
	backend := &MockBackend{
		models: []ModelInfo{
			{ID: "gpt-4", Name: "GPT-4", Provider: "openai"},
			{ID: "claude-opus", Name: "Claude Opus", Provider: "anthropic"},
		},
	}

	cmd := HandleCommand("/model gpt-4", backend)
	msg := cmd()

	modelMsg, ok := msg.(ModelSelectedMsg)
	if !ok {
		t.Errorf("HandleCommand(/model gpt-4) returned %T, want ModelSelectedMsg", msg)
		return
	}

	if modelMsg.Model.ID != "gpt-4" {
		t.Errorf("Expected model ID 'gpt-4', got '%s'", modelMsg.Model.ID)
	}
}

func TestHandleCommand_Model_PartialMatch(t *testing.T) {
	backend := &MockBackend{
		models: []ModelInfo{
			{ID: "gpt-4-turbo", Name: "GPT-4 Turbo", Provider: "openai"},
			{ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo", Provider: "openai"},
		},
	}

	// Partial match should find gpt-4-turbo
	cmd := HandleCommand("/model gpt-4", backend)
	msg := cmd()

	modelMsg, ok := msg.(ModelSelectedMsg)
	if !ok {
		t.Errorf("HandleCommand(/model gpt-4) returned %T, want ModelSelectedMsg", msg)
		return
	}

	// Should match gpt-4-turbo
	if !strings.Contains(modelMsg.Model.ID, "gpt-4") {
		t.Errorf("Expected model containing 'gpt-4', got '%s'", modelMsg.Model.ID)
	}
}

func TestHandleCommand_Model_NotFound(t *testing.T) {
	backend := &MockBackend{
		models: []ModelInfo{
			{ID: "gpt-4", Name: "GPT-4", Provider: "openai"},
		},
	}

	cmd := HandleCommand("/model invalid-model", backend)
	msg := cmd()

	cmdErr, ok := msg.(CommandErrorMsg)
	if !ok {
		t.Errorf("HandleCommand(/model invalid-model) returned %T, want CommandErrorMsg", msg)
		return
	}

	if !strings.Contains(cmdErr.Error, "not found") {
		t.Errorf("Expected error to contain 'not found', got: %s", cmdErr.Error)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// THEME COMMAND TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestHandleCommand_Theme_NoArgs(t *testing.T) {
	backend := &MockBackend{}

	cmd := HandleCommand("/theme", backend)
	msg := cmd()

	if _, ok := msg.(ShowThemeSelectorMsg); !ok {
		t.Errorf("HandleCommand(/theme) returned %T, want ShowThemeSelectorMsg", msg)
	}
}

func TestHandleCommand_Theme_Valid(t *testing.T) {
	backend := &MockBackend{}

	tests := []string{"default", "dracula", "nord", "gruvbox"}

	for _, theme := range tests {
		t.Run(theme, func(t *testing.T) {
			cmd := HandleCommand("/theme "+theme, backend)
			msg := cmd()

			themeMsg, ok := msg.(ThemeSelectedMsg)
			if !ok {
				t.Errorf("HandleCommand(/theme %s) returned %T, want ThemeSelectedMsg", theme, msg)
				return
			}

			if themeMsg.ThemeName != theme {
				t.Errorf("Expected theme '%s', got '%s'", theme, themeMsg.ThemeName)
			}
		})
	}
}

func TestHandleCommand_Theme_Invalid(t *testing.T) {
	backend := &MockBackend{}

	cmd := HandleCommand("/theme invalid-theme", backend)
	msg := cmd()

	cmdErr, ok := msg.(CommandErrorMsg)
	if !ok {
		t.Errorf("HandleCommand(/theme invalid-theme) returned %T, want CommandErrorMsg", msg)
		return
	}

	if !strings.Contains(cmdErr.Error, "not found") {
		t.Errorf("Expected error to contain 'not found', got: %s", cmdErr.Error)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// SHELL ESCAPE TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestHandleShellEscape_Success(t *testing.T) {
	cmd := HandleShellEscape("!echo test")
	msg := cmd()

	shellMsg, ok := msg.(ShellCommandMsg)
	if !ok {
		t.Errorf("HandleShellEscape returned %T, want ShellCommandMsg", msg)
		return
	}

	if !strings.Contains(shellMsg.Output, "test") {
		t.Errorf("Expected output to contain 'test', got: %s", shellMsg.Output)
	}

	if shellMsg.Error != nil {
		t.Errorf("Expected no error, got: %v", shellMsg.Error)
	}
}

func TestHandleShellEscape_Empty(t *testing.T) {
	cmd := HandleShellEscape("!")
	msg := cmd()

	cmdErr, ok := msg.(CommandErrorMsg)
	if !ok {
		t.Errorf("HandleShellEscape(!) returned %T, want CommandErrorMsg", msg)
		return
	}

	if !strings.Contains(cmdErr.Error, "No shell command") {
		t.Errorf("Expected error about empty command, got: %s", cmdErr.Error)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// AUTOCOMPLETE TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestGetCommandSuggestions_Empty(t *testing.T) {
	suggestions := GetCommandSuggestions("")

	if len(suggestions) == 0 {
		t.Error("Expected suggestions for empty input, got none")
	}

	// Should include all commands
	expected := []string{"/help", "/model", "/theme", "/clear", "/yolo"}
	for _, exp := range expected {
		found := false
		for _, sug := range suggestions {
			if sug == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected suggestions to include %s", exp)
		}
	}
}

func TestGetCommandSuggestions_Partial(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"/h", []string{"/help", "/h"}},
		{"/mo", []string{"/model", "/m"}},
		{"/th", []string{"/theme", "/t"}},
		{"/q", []string{"/quit", "/q"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			suggestions := GetCommandSuggestions(tt.input)

			for _, exp := range tt.expected {
				found := false
				for _, sug := range suggestions {
					if sug == exp {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("GetCommandSuggestions(%q) should include %s", tt.input, exp)
				}
			}
		})
	}
}

func TestGetCommandHelp(t *testing.T) {
	help := GetCommandHelp()

	if len(help) == 0 {
		t.Error("Expected command help map, got empty")
	}

	// Should include all major commands
	expectedKeys := []string{
		"/help, /h, /?",
		"/model, /m [name]",
		"/theme, /t [name]",
		"/clear, /c",
		"/yolo",
	}

	for _, key := range expectedKeys {
		if _, ok := help[key]; !ok {
			t.Errorf("Expected help to include key: %s", key)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// BENCHMARKS
// ═══════════════════════════════════════════════════════════════════════════════

func BenchmarkHandleCommand(b *testing.B) {
	backend := &MockBackend{
		models: []ModelInfo{
			{ID: "gpt-4", Name: "GPT-4", Provider: "openai"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := HandleCommand("/help", backend)
		_ = cmd()
	}
}

func BenchmarkGetCommandSuggestions(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GetCommandSuggestions("/h")
	}
}
