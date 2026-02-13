package styles

import (
	"github.com/charmbracelet/lipgloss"
)

// Colors for the vault theme - using AdaptiveColor for light/dark terminal support
var (
	// Primary colors (work on both light and dark)
	Primary     = lipgloss.Color("#7C3AED") // Purple
	PrimaryDark = lipgloss.Color("#5B21B6")
	Secondary   = lipgloss.Color("#06B6D4") // Cyan
	Accent      = lipgloss.Color("#F59E0B") // Amber

	// Status colors
	Success = lipgloss.Color("#10B981") // Green
	Warning = lipgloss.Color("#F59E0B") // Amber
	Error   = lipgloss.Color("#EF4444") // Red
	Info    = lipgloss.Color("#3B82F6") // Blue

	// Neutral colors - adaptive for light/dark terminals
	Background    = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#0F0F0F"}
	Surface       = lipgloss.AdaptiveColor{Light: "#F5F5F5", Dark: "#1A1A1A"}
	SurfaceLight  = lipgloss.AdaptiveColor{Light: "#E5E5E5", Dark: "#262626"}
	Border        = lipgloss.AdaptiveColor{Light: "#D4D4D4", Dark: "#333333"}
	BorderFocused = lipgloss.AdaptiveColor{Light: "#A3A3A3", Dark: "#525252"}

	// Text colors - adaptive for light/dark terminals
	TextPrimary   = lipgloss.AdaptiveColor{Light: "#171717", Dark: "#FAFAFA"}
	TextSecondary = lipgloss.AdaptiveColor{Light: "#525252", Dark: "#A3A3A3"}
	TextMuted     = lipgloss.AdaptiveColor{Light: "#737373", Dark: "#737373"}
	TextDisabled  = lipgloss.AdaptiveColor{Light: "#A3A3A3", Dark: "#525252"}

	// Secret type colors (these work on both)
	APIKeyColor      = lipgloss.Color("#D97706") // Amber/Gold (darker for light bg)
	SSHKeyColor      = lipgloss.Color("#0891B2") // Cyan
	PasswordColor    = lipgloss.Color("#DC2626") // Red
	CertificateColor = lipgloss.Color("#059669") // Green
)

// Base styles
var (
	// Container styles
	AppContainer = lipgloss.NewStyle().
			Background(Background)

	// Header styles
	Header = lipgloss.NewStyle().
		Foreground(TextPrimary).
		Bold(true).
		Padding(0, 1)

	HeaderTitle = lipgloss.NewStyle().
			Foreground(Primary).
			Bold(true)

	// Panel styles - use visible border color
	Panel = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#888888")).
		Padding(1)

	PanelFocused = Panel.
			BorderForeground(Primary)

	// Sidebar styles - use visible border color
	Sidebar = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#888888")).
		Padding(0, 1)

	SidebarItem = lipgloss.NewStyle().
			Foreground(TextSecondary).
			Padding(0, 1)

	SidebarItemSelected = lipgloss.NewStyle().
				Foreground(TextPrimary).
				Background(SurfaceLight).
				Bold(true).
				Padding(0, 1)

	SidebarItemCount = lipgloss.NewStyle().
				Foreground(TextMuted)

	// List item styles
	ListItem = lipgloss.NewStyle().
			Foreground(TextPrimary).
			Padding(0, 1)

	ListItemSelected = lipgloss.NewStyle().
				Foreground(TextPrimary).
				Background(Primary).
				Bold(true).
				Padding(0, 1)

	ListItemSubtext = lipgloss.NewStyle().
			Foreground(TextMuted).
			Padding(0, 1)

	// Input styles
	Input = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Border).
		Padding(0, 1)

	InputFocused = Input.
			BorderForeground(Primary)

	InputLabel = lipgloss.NewStyle().
			Foreground(TextSecondary).
			Bold(true)

	InputPlaceholder = lipgloss.NewStyle().
				Foreground(TextMuted)

	// Button styles
	Button = lipgloss.NewStyle().
		Foreground(TextPrimary).
		Background(SurfaceLight).
		Padding(0, 2).
		Bold(true)

	ButtonPrimary = lipgloss.NewStyle().
			Foreground(TextPrimary).
			Background(Primary).
			Padding(0, 2).
			Bold(true)

	ButtonDanger = lipgloss.NewStyle().
			Foreground(TextPrimary).
			Background(Error).
			Padding(0, 2).
			Bold(true)

	// Status bar
	StatusBar = lipgloss.NewStyle().
			Foreground(TextMuted).
			Background(Surface).
			Padding(0, 1)

	StatusBarKey = lipgloss.NewStyle().
			Foreground(Primary).
			Bold(true)

	// Help text
	HelpText = lipgloss.NewStyle().
			Foreground(TextMuted)

	HelpKey = lipgloss.NewStyle().
		Foreground(Secondary).
		Bold(true)

	// Error/Success messages
	ErrorText = lipgloss.NewStyle().
			Foreground(Error).
			Bold(true)

	SuccessText = lipgloss.NewStyle().
			Foreground(Success).
			Bold(true)

	WarningText = lipgloss.NewStyle().
			Foreground(Warning).
			Bold(true)

	// Logo style
	Logo = lipgloss.NewStyle().
		Foreground(Primary).
		Bold(true)

	// Masked text (for passwords)
	MaskedText = lipgloss.NewStyle().
			Foreground(TextMuted)

	// Tag style
	Tag = lipgloss.NewStyle().
		Foreground(TextPrimary).
		Background(SurfaceLight).
		Padding(0, 1)

	// Divider - use visible color
	Divider = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888"))

	// Modal overlay
	Modal = lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(Primary).
		Padding(1, 2).
		Background(Surface)

	ModalTitle = lipgloss.NewStyle().
			Foreground(TextPrimary).
			Bold(true).
			Padding(0, 0, 1, 0)

	// Text styles (for rendering text with colors)
	TextPrimaryStyle = lipgloss.NewStyle().
				Foreground(TextPrimary)

	TextSecondaryStyle = lipgloss.NewStyle().
				Foreground(TextSecondary)

	TextMutedStyle = lipgloss.NewStyle().
			Foreground(TextMuted)

	TextDisabledStyle = lipgloss.NewStyle().
				Foreground(TextDisabled)
)

// Helper functions for dynamic styling

// SecretTypeStyle returns the color for a secret type
func SecretTypeStyle(secretType string) lipgloss.Style {
	switch secretType {
	case "api_key":
		return lipgloss.NewStyle().Foreground(APIKeyColor)
	case "ssh_key":
		return lipgloss.NewStyle().Foreground(SSHKeyColor)
	case "password":
		return lipgloss.NewStyle().Foreground(PasswordColor)
	case "certificate":
		return lipgloss.NewStyle().Foreground(CertificateColor)
	default:
		return lipgloss.NewStyle().Foreground(TextSecondary)
	}
}

// TruncateWithEllipsis truncates a string and adds ellipsis if needed
func TruncateWithEllipsis(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// RenderKeybind renders a keybind in consistent style
func RenderKeybind(key, description string) string {
	return HelpKey.Render("["+key+"]") + " " + HelpText.Render(description)
}
