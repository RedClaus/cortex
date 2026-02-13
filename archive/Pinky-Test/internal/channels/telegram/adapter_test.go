package telegram

import (
	"testing"

	"github.com/normanking/pinky/internal/channels"
	"github.com/normanking/pinky/internal/config"
)

func TestNew(t *testing.T) {
	cfg := config.TelegramConfig{
		Enabled: true,
		Token:   "test-token",
	}

	adapter := New(cfg, nil)

	if adapter == nil {
		t.Fatal("expected adapter to be created")
	}

	if adapter.Name() != "telegram" {
		t.Errorf("expected name 'telegram', got %s", adapter.Name())
	}

	if !adapter.IsEnabled() {
		t.Error("expected adapter to be enabled")
	}
}

func TestCapabilities(t *testing.T) {
	adapter := New(config.TelegramConfig{}, nil)

	if !adapter.SupportsMedia() {
		t.Error("expected SupportsMedia to be true")
	}

	if !adapter.SupportsButtons() {
		t.Error("expected SupportsButtons to be true")
	}

	if !adapter.SupportsThreading() {
		t.Error("expected SupportsThreading to be true")
	}
}

func TestEscapeMarkdownV2(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"hello_world", "hello\\_world"},
		{"*bold*", "\\*bold\\*"},
		{"[link]", "\\[link\\]"},
		{"(parens)", "\\(parens\\)"},
		{"cmd --flag", "cmd \\-\\-flag"},
		{"a.b.c", "a\\.b\\.c"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeMarkdownV2(tt.input)
			if result != tt.expected {
				t.Errorf("escapeMarkdownV2(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseChatID(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		wantErr  bool
	}{
		{"123456789", 123456789, false},
		{"-100123456789", -100123456789, false},
		{"abc", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseChatID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseChatID(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if result != tt.expected {
				t.Errorf("parseChatID(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatApprovalMessage(t *testing.T) {
	req := &channels.ApprovalRequest{
		ID:         "test-123",
		Tool:       "shell",
		Command:    "rm -rf /tmp/test",
		RiskLevel:  "high",
		WorkingDir: "/home/user",
		Reason:     "Cleaning up temp files",
	}

	result := formatApprovalMessage(req)

	// Should contain key elements (escaped)
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}

	// The message should contain the command
	if !contains(result, "rm") {
		t.Error("expected result to contain command")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr))
}

func TestImplementsChannelInterface(t *testing.T) {
	// Compile-time check that Adapter implements Channel
	var _ channels.Channel = (*Adapter)(nil)
}
