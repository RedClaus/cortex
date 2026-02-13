package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	Teal     = lipgloss.Color("#0d7377")
	OffWhite = lipgloss.Color("#f8f7f4")
	DarkGray = lipgloss.Color("#333333")

	// Styles
	AppStyle = lipgloss.NewStyle().
		Background(DarkGray).
		Foreground(OffWhite)

	StatusBarStyle = lipgloss.NewStyle().
		Background(Teal).
		Foreground(OffWhite).
		Bold(true).
		Padding(0, 1)

	ChatPanelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Teal).
		Padding(1)

	StatusPanelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Teal).
		Padding(1)

	NeuralPanelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Teal).
		Padding(1)

	InputBarStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Teal).
		Padding(0, 1)

	UserMessageStyle = lipgloss.NewStyle().
		Foreground(OffWhite).
		Bold(true)

	AssistantMessageStyle = lipgloss.NewStyle().
		Foreground(Teal)

	EventStyle = lipgloss.NewStyle().
		Foreground(OffWhite)
)
