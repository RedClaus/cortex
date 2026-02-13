// Package ui provides view rendering functions for the Cortex TUI.
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ═══════════════════════════════════════════════════════════════════════════════
// MAIN VIEW FUNCTION
// ═══════════════════════════════════════════════════════════════════════════════

// view renders the complete TUI interface.
// This is the main rendering function called by Model.View().
func view(m Model) string {
	// Show initialization message until terminal is sized
	if !m.ready {
		return m.styles.ChatArea.Render("Initializing Cortex...")
	}

	// Build the main interface components
	header := viewHeader(m)
	chat := viewChat(m)
	input := viewInput(m)
	footer := viewFooter(m)

	// Compose the vertical layout
	main := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		chat,
		input,
		footer,
	)

	// Overlay modal if one is active
	if m.activeModal != ModalNone {
		main = overlayModal(m, main)
	}

	return main
}

// ═══════════════════════════════════════════════════════════════════════════════
// HEADER VIEW
// ═══════════════════════════════════════════════════════════════════════════════

// viewHeader renders the top header bar with logo, context, and model info.
func viewHeader(m Model) string {
	// Use the model's styles for consistent rendering
	headerStyle := m.styles.Header.Copy().Width(m.width)

	// Logo with brain emoji
	logo := m.styles.Logo.Render("🧠 CORTEX")

	// Working directory (placeholder - will be populated when available)
	// TODO: Add workingDir field to Model struct
	workingDir := m.styles.HeaderContext.Render("~/projects")

	// Model status indicator
	modelStatus := viewModelStatus(m)

	// Layout: logo on left, model status on right, working dir in middle
	leftSection := lipgloss.JoinHorizontal(lipgloss.Left, logo, "  ", workingDir)

	// Calculate spacing to push model status to the right
	leftWidth := lipgloss.Width(leftSection)
	rightWidth := lipgloss.Width(modelStatus)
	spacerWidth := m.width - leftWidth - rightWidth - 4 // 4 for padding
	if spacerWidth < 1 {
		spacerWidth = 1
	}
	spacer := strings.Repeat(" ", spacerWidth)

	headerContent := lipgloss.JoinHorizontal(lipgloss.Left, leftSection, spacer, modelStatus)

	return headerStyle.Render(headerContent)
}

// viewModelStatus renders the current model/provider status indicator.
func viewModelStatus(m Model) string {
	if m.currentModel == "" {
		return m.styles.HeaderContext.Render("[No Model]")
	}

	// Show provider and model name
	statusText := fmt.Sprintf("[%s: %s]", m.currentProvider, m.currentModel)

	// Use HeaderStatus style for model status
	return m.styles.HeaderStatus.Render(statusText)
}

// ═══════════════════════════════════════════════════════════════════════════════
// CHAT VIEW
// ═══════════════════════════════════════════════════════════════════════════════

// viewChat renders the chat viewport containing the message history.
func viewChat(m Model) string {
	// The viewport component handles scrolling and rendering
	// We just need to ensure it contains the rendered messages
	return m.viewport.View()
}

// ═══════════════════════════════════════════════════════════════════════════════
// INPUT VIEW
// ═══════════════════════════════════════════════════════════════════════════════

// viewInput renders the input area with textarea and streaming indicator.
func viewInput(m Model) string {
	// Input container style
	inputStyle := m.styles.InputArea.Copy().Width(m.width - 2) // Account for border

	// Get the textarea view
	textareaView := m.input.View()

	// If streaming, add spinner to the right of the input
	if m.isStreaming {
		spinnerView := m.spinner.View()
		statusText := m.styles.Spinner.Render(fmt.Sprintf(" %s Streaming...", spinnerView))

		// Join textarea and spinner horizontally
		textareaView = lipgloss.JoinHorizontal(lipgloss.Left, textareaView, statusText)
	}

	return inputStyle.Render(textareaView)
}

// ═══════════════════════════════════════════════════════════════════════════════
// FOOTER VIEW
// ═══════════════════════════════════════════════════════════════════════════════

// viewFooter renders the bottom footer with mode indicator and help keybindings.
func viewFooter(m Model) string {
	// Footer style
	footerStyle := m.styles.Footer.Copy().Width(m.width)

	// Mode indicator with color coding
	modeText := viewModeIndicator(m)

	// Help keybindings
	helpText := viewHelpText(m)

	// Layout: mode on left, help on right
	leftSection := modeText
	rightSection := helpText

	// Calculate spacing
	leftWidth := lipgloss.Width(leftSection)
	rightWidth := lipgloss.Width(rightSection)
	spacerWidth := m.width - leftWidth - rightWidth - 4 // 4 for padding
	if spacerWidth < 1 {
		spacerWidth = 1
	}
	spacer := strings.Repeat(" ", spacerWidth)

	footerContent := lipgloss.JoinHorizontal(lipgloss.Left, leftSection, spacer, rightSection)

	return footerStyle.Render(footerContent)
}

// viewModeIndicator renders the current operational mode with color coding.
func viewModeIndicator(m Model) string {
	var modeStyle lipgloss.Style
	var modeText string

	switch m.mode {
	case ModeNormal:
		modeStyle = m.styles.FooterNormal
		modeText = "● NORMAL"

	case ModeYolo:
		modeStyle = m.styles.FooterYolo
		modeText = "⚡ YOLO"

	case ModePlan:
		modeStyle = m.styles.FooterPlan
		modeText = "📋 PLAN"

	default:
		modeStyle = m.styles.FooterNormal
		modeText = "● UNKNOWN"
	}

	return modeStyle.Render(modeText)
}

// viewHelpText renders abbreviated help keybindings.
func viewHelpText(m Model) string {
	// Show different help text based on active modal
	if m.activeModal != ModalNone {
		return m.styles.FooterNormal.Render("ESC: close • ↑/↓: navigate • ENTER: select")
	}

	// Block mode has different keybindings
	if m.useBlockSystem {
		return m.styles.FooterNormal.Render("j/k: nav • c: copy • t: toggle • ?: help • ^C: quit")
	}

	// Default keybindings for normal mode
	return m.styles.FooterNormal.Render("^M: model • ^T: theme • ?: help • ^C: quit")
}

// ═══════════════════════════════════════════════════════════════════════════════
// MODAL OVERLAY
// ═══════════════════════════════════════════════════════════════════════════════

// overlayModal renders a modal dialog on top of the main view.
func overlayModal(m Model, main string) string {
	// Modal content (will be replaced with actual modal rendering)
	var modalContent string

	switch m.activeModal {
	case ModalHelp:
		modalContent = renderHelpModal(m)
	case ModalModel:
		modalContent = renderModelModal(m)
	case ModalTheme:
		modalContent = renderThemeModal(m)
	case ModalSession:
		modalContent = renderSessionModal(m)
	case ModalConfirm:
		modalContent = renderConfirmModal(m)
	case ModalDevice:
		modalContent = renderDeviceModal(m)
	default:
		modalContent = "Unknown modal"
	}

	// Modal box style
	modalBoxStyle := m.styles.ModalBorder.Copy().
		Width(60). // Fixed width for modals
		MaxWidth(m.width - 10)

	// Render the modal box
	modalBox := modalBoxStyle.Render(modalContent)

	// Create semi-transparent overlay using lipgloss.Place
	// This centers the modal in the terminal
	overlay := lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		modalBox,
		lipgloss.WithWhitespaceChars("░"), // Semi-transparent overlay effect
	)

	return overlay
}

// ═══════════════════════════════════════════════════════════════════════════════
// MODAL RENDERING FUNCTIONS (Placeholders)
// ═══════════════════════════════════════════════════════════════════════════════

// renderHelpModal renders the help/keybindings modal.
func renderHelpModal(m Model) string {
	title := m.styles.ModalTitle.Render("⌨️  KEYBOARD SHORTCUTS\n\n")

	// Core keybindings
	coreHelp := "CORE ACTIONS\n" +
		"  Enter     - Send Message\n" +
		"  Ctrl+C    - Cancel/Quit\n" +
		"  Esc       - Close Modal\n\n"

	// Navigation keybindings
	navHelp := "NAVIGATION\n" +
		"  ↑/k       - Scroll Up\n" +
		"  ↓/j       - Scroll Down\n" +
		"  PgUp      - Page Up\n" +
		"  PgDn      - Page Down\n" +
		"  Home/g    - Top\n" +
		"  End/G     - Bottom\n\n"

	// Modals keybindings
	modalsHelp := "MODALS\n" +
		"  Ctrl+M    - Select Model\n" +
		"  Ctrl+T    - Select Theme\n" +
		"  Ctrl+S    - Sessions\n" +
		"  Ctrl+A    - Audio Devices\n" +
		"  ?/F1      - Help\n\n"

	// Mode keybindings
	modeHelp := "MODES\n" +
		"  Ctrl+N    - Normal Mode\n" +
		"  Ctrl+Y    - YOLO Mode\n" +
		"  Ctrl+P    - Plan Mode\n" +
		"  Ctrl+L    - Clear History\n\n"

	helpContent := coreHelp + navHelp + modalsHelp + modeHelp

	// Add block keybindings when block system is enabled
	if m.useBlockSystem {
		blockNavHelp := "BLOCK NAVIGATION\n" +
			"  j/↓       - Next Block\n" +
			"  k/↑       - Previous Block\n" +
			"  l/→       - Expand/Enter\n" +
			"  h/←       - Collapse/Parent\n\n"

		blockActionHelp := "BLOCK ACTIONS\n" +
			"  c         - Copy Block\n" +
			"  t         - Toggle Collapse\n" +
			"  b         - Toggle Bookmark\n" +
			"  r         - Regenerate\n" +
			"  e         - Edit Block\n" +
			"  n         - Next Bookmark\n" +
			"  N         - Prev Bookmark\n"

		helpContent += blockNavHelp + blockActionHelp
	}

	return title + helpContent
}

// renderModelModal renders the model selection modal.
func renderModelModal(m Model) string {
	return m.styles.ModalTitle.Render("🤖 MODEL SELECTOR\n\n") +
		"(Model selection UI will be implemented here)"
}

// renderThemeModal renders the theme selection modal.
func renderThemeModal(m Model) string {
	return m.styles.ModalTitle.Render("🎨 THEME SELECTOR\n\n") +
		"(Theme selection UI will be implemented here)"
}

// renderSessionModal renders the session management modal.
func renderSessionModal(m Model) string {
	return m.styles.ModalTitle.Render("💬 SESSIONS\n\n") +
		"(Session management UI will be implemented here)"
}

// renderConfirmModal renders a confirmation dialog.
func renderConfirmModal(m Model) string {
	return m.styles.ModalTitle.Render("⚠️  CONFIRM\n\n") +
		"(Confirmation dialog will be implemented here)"
}

// renderDeviceModal renders the audio device selection modal.
func renderDeviceModal(m Model) string {
	// Use the device selector component if available
	if m.deviceSelector != nil {
		return m.deviceSelector.View()
	}

	// Fallback if device selector not initialized
	title := m.styles.ModalTitle.Render("🎙️ AUDIO DEVICE SETTINGS\n\n")

	instructions := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7aa2f7")).
		Render("Loading audio devices from voice orchestrator...\n\n")

	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#565f89")).
		Render("Press Ctrl+A to refresh • ESC to close\n\n")

	// Show instructions for manual API access
	apiInfo := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9ece6a")).
		Render("Voice orchestrator API:\n") +
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("#c0caf5")).
			Render("  GET  http://localhost:8765/devices\n") +
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("#c0caf5")).
			Render("  POST http://localhost:8765/devices/input/{index}\n") +
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("#c0caf5")).
			Render("  POST http://localhost:8765/devices/output/{index}\n")

	return title + instructions + help + apiInfo
}
