// Package tui provides the interactive terminal user interface for Cortex Evaluator.
package tui

import "github.com/charmbracelet/lipgloss"

// Color palette
var (
	primaryColor   = lipgloss.Color("#7C3AED") // Purple
	secondaryColor = lipgloss.Color("#10B981") // Green
	accentColor    = lipgloss.Color("#F59E0B") // Amber
	errorColor     = lipgloss.Color("#EF4444") // Red
	dimColor       = lipgloss.Color("#6B7280") // Gray
	bgColor        = lipgloss.Color("#1F2937") // Dark gray
	textColor      = lipgloss.Color("#F9FAFB") // Light
)

// UI Styles
var (
	// Title and headers
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(dimColor).
			Italic(true)

	// Session list
	sessionListStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(primaryColor).
				Padding(1, 2)

	selectedSessionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(primaryColor).
				Background(lipgloss.Color("#374151"))

	normalSessionStyle = lipgloss.NewStyle().
				Foreground(textColor)

	archivedSessionStyle = lipgloss.NewStyle().
				Foreground(dimColor).
				Strikethrough(true)

	// Chat area
	chatBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(secondaryColor).
			Padding(1, 2)

	userMessageStyle = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true)

	assistantMessageStyle = lipgloss.NewStyle().
				Foreground(secondaryColor)

	// Input area
	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accentColor).
			Padding(0, 1)

	inputPromptStyle = lipgloss.NewStyle().
				Foreground(accentColor).
				Bold(true)

	// Status bar
	statusBarStyle = lipgloss.NewStyle().
			Background(bgColor).
			Foreground(textColor).
			Padding(0, 1)

	statusKeyStyle = lipgloss.NewStyle().
			Background(primaryColor).
			Foreground(textColor).
			Padding(0, 1).
			Bold(true)

	statusValueStyle = lipgloss.NewStyle().
				Background(bgColor).
				Foreground(dimColor).
				Padding(0, 1)

	// Help
	helpStyle = lipgloss.NewStyle().
			Foreground(dimColor)

	// Dim text
	dimStyle = lipgloss.NewStyle().
			Foreground(dimColor)

	// Errors and success
	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true)

	// Info boxes
	infoBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(dimColor).
			Padding(1, 2)

	// Context panel
	contextPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(dimColor).
				Padding(0, 1)

	contextHeaderStyle = lipgloss.NewStyle().
				Foreground(accentColor).
				Bold(true)

	// File reference
	fileRefStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Underline(true)
)

// Logo returns the ASCII art logo
func Logo() string {
	logo := "" +
		"   ____          _              \n" +
		"  / ___|___  _ _| |_ _____  __  \n" +
		" | |   / _ \\| '_| __/ _ \\ \\/ / \n" +
		" | |__| (_) | | | ||  __/>  <   \n" +
		"  \\____\\___/|_|  \\__\\___/_/\\_\\ \n" +
		"    ___            _           _\n" +
		"   | __|_ __  __ _| |_  _ __ _| |_ ___ _ _\n" +
		"   | _|\\ V / / _` | | || / _` |  _/ _ \\ '_|\n" +
		"   |___|  \\/  \\__,_|_|\\_,_\\__,_|\\__\\___/_|\n"
	return titleStyle.Render(logo)
}
