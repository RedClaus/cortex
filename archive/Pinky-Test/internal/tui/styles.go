// Package tui provides the terminal user interface for Pinky
package tui

import "github.com/charmbracelet/lipgloss"

// Color palette
var (
	colorPrimary   = lipgloss.Color("#7D56F4")
	colorSecondary = lipgloss.Color("#5DADE2")
	colorSuccess   = lipgloss.Color("#2ECC71")
	colorWarning   = lipgloss.Color("#F39C12")
	colorDanger    = lipgloss.Color("#E74C3C")
	colorMuted     = lipgloss.Color("#7F8C8D")
	colorBorder    = lipgloss.Color("#3D3D3D")
	colorBg        = lipgloss.Color("#1E1E1E")
	colorBgAlt     = lipgloss.Color("#2D2D2D")
)

// Styles holds all TUI styles
type Styles struct {
	App        lipgloss.Style
	Header     lipgloss.Style
	StatusBar  lipgloss.Style
	ChatPane   lipgloss.Style
	InputBox   lipgloss.Style
	UserMsg    lipgloss.Style
	BotMsg     lipgloss.Style
	ToolStatus lipgloss.Style
	Approval   ApprovalStyles
	Help       lipgloss.Style
}

// ApprovalStyles for the approval dialog
type ApprovalStyles struct {
	Container lipgloss.Style
	Title     lipgloss.Style
	Command   lipgloss.Style
	Info      lipgloss.Style
	Checkbox  lipgloss.Style
	Buttons   lipgloss.Style
	Approve   lipgloss.Style
	Deny      lipgloss.Style
}

// DefaultStyles returns the default style configuration
func DefaultStyles() Styles {
	return Styles{
		App: lipgloss.NewStyle().
			Background(colorBg),

		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(colorBorder).
			Padding(0, 1),

		StatusBar: lipgloss.NewStyle().
			Foreground(colorMuted).
			Background(colorBgAlt).
			Padding(0, 1),

		ChatPane: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(1),

		InputBox: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(0, 1),

		UserMsg: lipgloss.NewStyle().
			Foreground(colorSecondary).
			Bold(true),

		BotMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")),

		ToolStatus: lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true),

		Approval: ApprovalStyles{
			Container: lipgloss.NewStyle().
				BorderStyle(lipgloss.DoubleBorder()).
				BorderForeground(colorWarning).
				Padding(1, 2).
				Background(colorBgAlt),

			Title: lipgloss.NewStyle().
				Bold(true).
				Foreground(colorWarning),

			Command: lipgloss.NewStyle().
				Foreground(colorSecondary).
				Background(colorBg).
				Padding(0, 1),

			Info: lipgloss.NewStyle().
				Foreground(colorMuted),

			Checkbox: lipgloss.NewStyle().
				Foreground(colorMuted),

			Buttons: lipgloss.NewStyle().
				MarginTop(1),

			Approve: lipgloss.NewStyle().
				Bold(true).
				Foreground(colorBg).
				Background(colorSuccess).
				Padding(0, 2),

			Deny: lipgloss.NewStyle().
				Bold(true).
				Foreground(colorBg).
				Background(colorDanger).
				Padding(0, 2),
		},

		Help: lipgloss.NewStyle().
			Foreground(colorMuted),
	}
}
