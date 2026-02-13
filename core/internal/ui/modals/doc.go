// Package modals provides reusable modal overlay components for the Cortex TUI.
//
// The modals package implements a complete modal overlay system using the
// Charmbracelet Bubble Tea framework. All modals support:
//   - Rounded borders
//   - Escape key to dismiss
//   - Proper title styling
//   - Centered overlay on dimmed background
//
// Available Modals:
//
// HelpModal - Displays keyboard shortcuts organized by category
//
//	keyMap := DefaultKeyMap()
//	help := NewHelpModal(keyMap)
//	view := help.View()
//
// ModelSelector - List-based modal for selecting AI models
//
//	models := []ui.ModelInfo{...}
//	selector := NewModelSelector(models)
//	// Returns ModelSelectedMsg when a model is chosen
//
// ThemeSelector - List-based modal for selecting color themes
//
//	selector := NewThemeSelector("tokyo_night")
//	// Returns ThemeSelectedMsg when a theme is chosen
//
// ConfirmModal - Yes/No confirmation dialog for dangerous operations
//
//	modal := NewConfirmModal("Delete File?", "This cannot be undone", fileData)
//	// Returns ConfirmMsg with Yes/No result
//
// Overlay System:
//
// Use OverlayModal to render any modal on top of the main UI with a dimmed background:
//
//	dimColor := lipgloss.Color("#000000")
//	screen := OverlayModal(mainUIView, modalView, width, height, dimColor)
//
// All modals implement the tea.Model interface and can be integrated into
// existing Bubble Tea applications.
package modals
