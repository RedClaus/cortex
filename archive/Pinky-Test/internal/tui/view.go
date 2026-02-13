package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View implements tea.Model
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var sections []string

	// Header
	sections = append(sections, m.renderHeader())

	// Main content area
	if m.showHelp {
		sections = append(sections, m.renderHelp())
	} else {
		sections = append(sections, m.renderMainContent())
	}

	// Status bar
	sections = append(sections, m.renderStatusBar())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderHeader renders the top header bar
func (m Model) renderHeader() string {
	title := "Pinky v1.0.0"
	if m.verboseMode {
		title += " [Verbose]"
	}

	return m.styles.Header.Width(m.width).Render(title)
}

// renderMainContent renders the chat and approval areas
func (m Model) renderMainContent() string {
	var content string

	// Chat viewport
	chatBox := m.styles.ChatPane.
		Width(m.width - 4).
		Height(m.viewport.Height + 2).
		Render(m.viewport.View())

	content = chatBox

	// Overlay settings panel if active
	if m.focus == FocusSettings && m.settingsPanel != nil {
		settingsDialog := m.settingsPanel.View()
		content = m.overlayDialog(content, settingsDialog)
	}

	// Overlay approval dialog if pending
	if m.pendingApproval != nil && m.focus == FocusApproval {
		approvalDialog := m.renderApprovalDialog()
		content = m.overlayDialog(content, approvalDialog)
	}

	// Input area
	inputBox := m.styles.InputBox.
		Width(m.width - 4).
		Render(m.textarea.View())

	return lipgloss.JoinVertical(lipgloss.Left,
		content,
		"",
		inputBox,
	)
}

// renderApprovalDialog renders the approval request dialog
func (m Model) renderApprovalDialog() string {
	if m.pendingApproval == nil {
		return ""
	}

	var sections []string

	// Title
	title := m.styles.Approval.Title.Render("Tool Approval Required")
	sections = append(sections, title)
	sections = append(sections, "")

	// Description
	sections = append(sections, "Pinky wants to execute:")
	sections = append(sections, "")

	// Command box
	command := m.styles.Approval.Command.Render("$ " + m.pendingApproval.Command)
	sections = append(sections, command)
	sections = append(sections, "")

	// Info
	info := fmt.Sprintf("Tool: %s (%s Risk)", m.pendingApproval.Tool, m.pendingApproval.RiskLevel)
	if m.pendingApproval.WorkingDir != "" {
		info += fmt.Sprintf("\nDir: %s", m.pendingApproval.WorkingDir)
	}
	if m.pendingApproval.Reason != "" {
		info += fmt.Sprintf("\nReason: %s", m.pendingApproval.Reason)
	}
	sections = append(sections, m.styles.Approval.Info.Render(info))
	sections = append(sections, "")

	// Checkboxes
	alwaysCheck := "[ ]"
	if m.alwaysAllow {
		alwaysCheck = "[x]"
	}
	sections = append(sections, m.styles.Approval.Checkbox.Render(
		alwaysCheck+" Always allow \""+m.pendingApproval.Tool+"\" commands"))

	dirCheck := "[ ]"
	if m.allowDir {
		dirCheck = "[x]"
	}
	sections = append(sections, m.styles.Approval.Checkbox.Render(
		dirCheck+" Always allow in this directory"))

	sections = append(sections, "")

	// Buttons
	buttons := lipgloss.JoinHorizontal(lipgloss.Center,
		m.styles.Approval.Deny.Render("[d]eny"),
		"  ",
		m.styles.Approval.Approve.Render("[a]pprove"),
		"  ",
		"[A]lways",
		"  ",
		"[e]dit",
	)
	sections = append(sections, m.styles.Approval.Buttons.Render(buttons))

	// Join all sections
	dialogContent := lipgloss.JoinVertical(lipgloss.Left, sections...)

	return m.styles.Approval.Container.Render(dialogContent)
}

// overlayDialog centers a dialog over the content
func (m Model) overlayDialog(background, dialog string) string {
	bgLines := strings.Split(background, "\n")
	dialogLines := strings.Split(dialog, "\n")

	// Calculate dialog dimensions
	dialogWidth := lipgloss.Width(dialog)
	dialogHeight := len(dialogLines)

	// Calculate position (center)
	startRow := (len(bgLines) - dialogHeight) / 2
	startCol := (m.width - dialogWidth) / 2

	if startRow < 0 {
		startRow = 0
	}
	if startCol < 0 {
		startCol = 0
	}

	// Overlay dialog onto background
	result := make([]string, len(bgLines))
	for i, line := range bgLines {
		if i >= startRow && i < startRow+dialogHeight {
			dialogIdx := i - startRow
			if dialogIdx < len(dialogLines) {
				// Insert dialog line at proper column
				padding := strings.Repeat(" ", startCol)
				result[i] = padding + dialogLines[dialogIdx]
			} else {
				result[i] = line
			}
		} else {
			result[i] = line
		}
	}

	return strings.Join(result, "\n")
}

// renderStatusBar renders the bottom status bar
func (m Model) renderStatusBar() string {
	// Channel status
	var channels []string
	for name, connected := range m.channelStatus {
		icon := "○"
		if connected {
			icon = "●"
		}
		channels = append(channels, fmt.Sprintf("%s%s", name, icon))
	}

	// Default TUI as connected
	if len(channels) == 0 {
		channels = append(channels, "TUI●")
	}

	channelStr := "Channels: " + strings.Join(channels, " ")

	// Memory
	memoryStr := fmt.Sprintf("Memory: %d", m.memoryCount)

	// Build status bar
	left := channelStr
	right := memoryStr

	// Calculate spacing
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 4
	if gap < 1 {
		gap = 1
	}

	status := left + strings.Repeat(" ", gap) + right

	return m.styles.StatusBar.Width(m.width).Render(status)
}

// renderHelp renders the help screen
func (m Model) renderHelp() string {
	var sections []string

	sections = append(sections, m.styles.Header.Render("Pinky Help"))
	sections = append(sections, "")

	// Key bindings
	sections = append(sections, "Key Bindings:")
	sections = append(sections, "")

	bindings := []struct {
		key  string
		desc string
	}{
		{"Enter", "Send message"},
		{"Ctrl+Q", "Quit"},
		{"Ctrl+C/Esc", "Cancel"},
		{"Ctrl+L", "Clear chat"},
		{"Ctrl+V", "Toggle verbose mode"},
		{"Ctrl+P", "Change persona"},
		{"Tab", "Cycle panels"},
		{"?", "Toggle help"},
		{"", ""},
		{"Commands:", ""},
		{"/settings", "Open inference settings"},
		{"/lanes", "Show lane status"},
		{"/help", "Show help"},
		{"", ""},
		{"Approval Mode:", ""},
		{"a/y", "Approve"},
		{"d/n", "Deny"},
		{"A", "Always allow"},
		{"e", "Edit command"},
		{"Space", "Toggle checkboxes"},
	}

	for _, b := range bindings {
		if b.key == "" {
			sections = append(sections, "")
		} else if b.desc == "" {
			sections = append(sections, m.styles.UserMsg.Render(b.key))
		} else {
			sections = append(sections, fmt.Sprintf("  %-12s %s", b.key, b.desc))
		}
	}

	sections = append(sections, "")
	sections = append(sections, m.styles.ToolStatus.Render("Press ? or Esc to close help"))

	helpContent := lipgloss.JoinVertical(lipgloss.Left, sections...)

	return m.styles.ChatPane.
		Width(m.width - 4).
		Height(m.viewport.Height + 2).
		Render(helpContent)
}
