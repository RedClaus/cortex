package modals

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// KeyBinding represents a single keybinding with its description.
type KeyBinding struct {
	Key  string // The key or key combination (e.g., "ctrl+c", "/", "enter")
	Desc string // Human-readable description of what it does
}

// KeyMap defines all keybindings for the application.
// Organized into logical sections for display.
type KeyMap struct {
	Navigation []KeyBinding
	Chat       []KeyBinding
	Settings   []KeyBinding
	Other      []KeyBinding
}

// DefaultKeyMap returns the standard Cortex keybindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Navigation: []KeyBinding{
			{"↑/k", "Scroll up"},
			{"↓/j", "Scroll down"},
			{"ctrl+u", "Scroll up half page"},
			{"ctrl+d", "Scroll down half page"},
			{"g", "Go to top"},
			{"G", "Go to bottom"},
			{"pgup", "Page up"},
			{"pgdn", "Page down"},
		},
		Chat: []KeyBinding{
			{"enter", "Send message"},
			{"↑/↓", "Navigate history (empty input)"},
			{"ctrl+l", "Clear chat"},
		},
		Settings: []KeyBinding{
			{"/", "Open command menu"},
			{"esc", "Close menu/modal"},
			{"?", "Show this help"},
			{"ctrl+t", "Cycle theme"},
		},
		Other: []KeyBinding{
			{"ctrl+c", "Quit application"},
		},
	}
}

// HelpModal displays all keybindings in an organized modal.
type HelpModal struct {
	keyMap KeyMap
	styles lipgloss.Style
	width  int
	height int
}

// NewHelpModal creates a new help modal with the given keybindings.
func NewHelpModal(keyMap KeyMap) *HelpModal {
	return &HelpModal{
		keyMap: keyMap,
		width:  60, // Fixed width for help modal
	}
}

// SetSize sets the modal dimensions.
func (h *HelpModal) SetSize(width, height int) {
	h.width = width
	h.height = height
}

// SetStyles updates the modal styling.
func (h *HelpModal) SetStyles(style lipgloss.Style) {
	h.styles = style
}

// Init implements tea.Model.
func (h *HelpModal) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (h *HelpModal) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "?", "q":
			// Close modal - parent should handle this
			return h, nil
		}
	}
	return h, nil
}

// View implements tea.Model and renders the help modal content.
func (h *HelpModal) View() string {
	var sections []string

	// Title
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7aa2f7")).
		Align(lipgloss.Center).
		Width(h.width - 4).
		Render("Keyboard Shortcuts")

	sections = append(sections, title)
	sections = append(sections, "")

	// Render each section
	if len(h.keyMap.Navigation) > 0 {
		sections = append(sections, h.renderSection("Navigation", h.keyMap.Navigation))
		sections = append(sections, "")
	}

	if len(h.keyMap.Chat) > 0 {
		sections = append(sections, h.renderSection("Chat Controls", h.keyMap.Chat))
		sections = append(sections, "")
	}

	if len(h.keyMap.Settings) > 0 {
		sections = append(sections, h.renderSection("Settings & UI", h.keyMap.Settings))
		sections = append(sections, "")
	}

	if len(h.keyMap.Other) > 0 {
		sections = append(sections, h.renderSection("Other", h.keyMap.Other))
	}

	content := strings.Join(sections, "\n")

	// Wrap in rounded border box
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7aa2f7")).
		Padding(1, 2).
		Width(h.width).
		Align(lipgloss.Left)

	return boxStyle.Render(content)
}

// renderSection renders a single section of keybindings.
func (h *HelpModal) renderSection(title string, bindings []KeyBinding) string {
	var lines []string

	// Section header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#bb9af7"))
	lines = append(lines, headerStyle.Render(title))

	// Keybindings
	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9ece6a")).
		Width(20).
		Align(lipgloss.Left)

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#c0caf5"))

	for _, binding := range bindings {
		key := keyStyle.Render(binding.Key)
		desc := descStyle.Render(binding.Desc)
		lines = append(lines, fmt.Sprintf("  %s %s", key, desc))
	}

	return strings.Join(lines, "\n")
}
