// Package modals provides reusable modal overlay components for the Cortex TUI.
// Modals are rendered on top of the main UI with dimmed backgrounds for focus.
package modals

import (
	"github.com/charmbracelet/lipgloss"
)

// OverlayModal centers a modal on a dimmed background.
// It uses lipgloss.Place to center the modal content and renders a dimmed
// background using whitespace characters.
//
// Parameters:
//   - background: The rendered main UI content (will be dimmed)
//   - modal: The rendered modal content to overlay
//   - width: Total terminal width
//   - height: Total terminal height
//   - dimColor: Color to use for the dimmed background overlay
//
// Returns:
//   - A string containing the full screen with the modal overlaid and centered
//
// Example:
//
//	content := lipgloss.Place(
//	    60, 10,
//	    lipgloss.Center, lipgloss.Center,
//	    "Modal content here",
//	)
//	dimColor := lipgloss.Color("#000000")
//	screen := OverlayModal(mainUI, content, 80, 24, dimColor)
func OverlayModal(background, modal string, width, height int, dimColor lipgloss.Color) string {
	return lipgloss.Place(
		width, height,
		lipgloss.Center, lipgloss.Center,
		modal,
		lipgloss.WithWhitespaceChars("░"),
		lipgloss.WithWhitespaceForeground(dimColor),
	)
}
