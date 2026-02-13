package modals_test

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/normanking/cortex/internal/ui/modals"
	"github.com/normanking/cortex/internal/ui/types"
	"github.com/normanking/cortex/pkg/theme"
)

// Example_overlayModal demonstrates basic overlay usage.
func Example_overlayModal() {
	// Render main UI content
	mainUI := "Main application content here..."

	// Render modal content
	modalContent := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Render("This is a modal")

	// Overlay the modal on dimmed background
	dimColor := lipgloss.Color("#1a1b26")
	screen := modals.OverlayModal(mainUI, modalContent, 80, 24, dimColor)

	_ = screen // Use the rendered screen
}

// Example_helpModal demonstrates help modal creation and rendering.
func Example_helpModal() {
	// Create custom keybindings
	keyMap := modals.KeyMap{
		Navigation: []modals.KeyBinding{
			{"↑/k", "Scroll up"},
			{"↓/j", "Scroll down"},
			{"g", "Go to top"},
			{"G", "Go to bottom"},
		},
		Chat: []modals.KeyBinding{
			{"enter", "Send message"},
		},
		Settings: []modals.KeyBinding{
			{"/", "Open menu"},
			{"?", "Show help"},
		},
		Other: []modals.KeyBinding{
			{"ctrl+c", "Quit"},
		},
	}

	// Create help modal
	help := modals.NewHelpModal(keyMap)
	help.SetSize(80, 24)

	// Render the modal
	view := help.View()
	_ = view // Use the rendered view
}

// Example_modelSelector demonstrates model selection modal.
func Example_modelSelector() {
	// Define available models
	models := []types.ModelInfo{
		{
			ID:       "gpt-4",
			Name:     "GPT-4",
			Provider: "openai",
			IsLocal:  false,
		},
		{
			ID:       "claude-3-opus",
			Name:     "Claude 3 Opus",
			Provider: "anthropic",
			IsLocal:  false,
		},
		{
			ID:       "llama2",
			Name:     "Llama 2 7B",
			Provider: "ollama",
			IsLocal:  true,
		},
	}

	// Create model selector
	selector := modals.NewModelSelector(models)
	selector.SetSize(80, 24)

	// Render the modal
	view := selector.View()
	_ = view // Use the rendered view

	// In your Update() function, handle the selection:
	// case modals.ModelSelectedMsg:
	//     selectedModel := msg.Model
	//     log.Printf("Selected: %s", selectedModel.Name)
}

// Example_themeSelector demonstrates theme selection modal.
func Example_themeSelector() {
	// Create theme selector with current theme
	currentTheme := "tokyo_night"
	selector := modals.NewThemeSelector(currentTheme)
	selector.SetSize(80, 24)

	// Render the modal
	view := selector.View()
	_ = view // Use the rendered view

	// In your Update() function, handle the selection:
	// case modals.ThemeSelectedMsg:
	//     themeID := msg.ThemeID
	//     palette := msg.Theme
	//     log.Printf("Selected theme: %s", palette.Name)
}

// Example_confirmModal demonstrates confirmation dialog.
func Example_confirmModal() {
	// Create confirmation modal for a dangerous operation
	conversationID := "conv-123"
	modal := modals.NewConfirmModal(
		"Delete Conversation?",
		"This will permanently delete all messages. This action cannot be undone.",
		conversationID, // Payload to identify what's being confirmed
	)
	modal.SetSize(80, 24)

	// Render the modal
	view := modal.View()
	_ = view // Use the rendered view

	// In your Update() function, handle the confirmation:
	// case modals.ConfirmMsg:
	//     if msg.Result == modals.ConfirmResultYes {
	//         conversationID := msg.Payload.(string)
	//         deleteConversation(conversationID)
	//     }
}

// TestKeyMapCreation verifies default keymap is valid.
func TestKeyMapCreation(t *testing.T) {
	keyMap := modals.DefaultKeyMap()

	if len(keyMap.Navigation) == 0 {
		t.Error("Expected navigation keybindings")
	}
	if len(keyMap.Chat) == 0 {
		t.Error("Expected chat keybindings")
	}
	if len(keyMap.Settings) == 0 {
		t.Error("Expected settings keybindings")
	}
	if len(keyMap.Other) == 0 {
		t.Error("Expected other keybindings")
	}
}

// TestHelpModalCreation verifies help modal can be created.
func TestHelpModalCreation(t *testing.T) {
	keyMap := modals.DefaultKeyMap()
	help := modals.NewHelpModal(keyMap)

	if help == nil {
		t.Fatal("Expected non-nil help modal")
	}

	// Set size and render
	help.SetSize(80, 24)
	view := help.View()

	if view == "" {
		t.Error("Expected non-empty view")
	}
}

// TestModelSelectorCreation verifies model selector can be created.
func TestModelSelectorCreation(t *testing.T) {
	models := []types.ModelInfo{
		{
			ID:       "gpt-4",
			Name:     "GPT-4",
			Provider: "openai",
			IsLocal:  false,
		},
	}

	selector := modals.NewModelSelector(models)
	if selector == nil {
		t.Fatal("Expected non-nil model selector")
	}

	selector.SetSize(80, 24)
	view := selector.View()

	if view == "" {
		t.Error("Expected non-empty view")
	}
}

// TestThemeSelectorCreation verifies theme selector can be created.
func TestThemeSelectorCreation(t *testing.T) {
	selector := modals.NewThemeSelector(theme.DefaultTheme)
	if selector == nil {
		t.Fatal("Expected non-nil theme selector")
	}

	selector.SetSize(80, 24)
	view := selector.View()

	if view == "" {
		t.Error("Expected non-empty view")
	}
}

// TestConfirmModalCreation verifies confirm modal can be created.
func TestConfirmModalCreation(t *testing.T) {
	modal := modals.NewConfirmModal(
		"Test Title",
		"Test message",
		"test-payload",
	)

	if modal == nil {
		t.Fatal("Expected non-nil confirm modal")
	}

	modal.SetSize(80, 24)
	view := modal.View()

	if view == "" {
		t.Error("Expected non-empty view")
	}
}

// TestOverlayModal verifies overlay rendering.
func TestOverlayModal(t *testing.T) {
	background := "Background content"
	modal := "Modal content"
	dimColor := lipgloss.Color("#1a1b26")

	result := modals.OverlayModal(background, modal, 80, 24, dimColor)

	if result == "" {
		t.Error("Expected non-empty overlay result")
	}
}
