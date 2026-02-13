// Package theme provides theme parsing and Lipgloss style generation for Salamander.
//
// Themes define the visual appearance of Salamander TUI applications through
// color palettes and component-specific styles. This package converts YAML theme
// configurations into ready-to-use Lipgloss styles.
//
// Usage:
//
//	// From configuration
//	theme := theme.NewTheme(cfg.Theme)
//
//	// Or use a builtin theme
//	theme := theme.GetTheme("tokyo_night")
//
//	// Apply styles
//	fmt.Println(theme.Title.Render("Hello World"))
//	fmt.Println(theme.Error.Render("Something went wrong"))
package theme

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/normanking/salamander/pkg/schema"
)

// Theme holds parsed Lipgloss styles ready for use in rendering.
type Theme struct {
	// Name of the theme
	Name string

	// Mode is "dark" or "light"
	Mode string

	// Colors holds the raw color palette
	Colors schema.ColorPalette

	// Message styles
	ChatMessage      lipgloss.Style
	UserMessage      lipgloss.Style
	AssistantMessage lipgloss.Style

	// Input styles
	Input            lipgloss.Style
	InputFocused     lipgloss.Style
	InputPlaceholder lipgloss.Style

	// Menu styles
	Menu                lipgloss.Style
	MenuItem            lipgloss.Style
	MenuItemSelected    lipgloss.Style
	MenuItemDescription lipgloss.Style

	// Status bar styles
	StatusBar     lipgloss.Style
	StatusBarItem lipgloss.Style

	// Common element styles
	Border   lipgloss.Style
	Title    lipgloss.Style
	Subtitle lipgloss.Style

	// Semantic styles
	Error   lipgloss.Style
	Success lipgloss.Style
	Warning lipgloss.Style
	Info    lipgloss.Style

	// Color-based styles
	Muted     lipgloss.Style
	Primary   lipgloss.Style
	Secondary lipgloss.Style
	Accent    lipgloss.Style
}

// NewTheme creates a Theme from a schema.ThemeConfig.
func NewTheme(cfg schema.ThemeConfig) *Theme {
	c := cfg.Colors

	// Apply defaults if colors are empty
	if c.Primary == "" {
		c = defaultDarkPalette()
	}

	return buildTheme(cfg.Name, cfg.Mode, c)
}

// DefaultTheme returns the default dark theme.
func DefaultTheme() *Theme {
	return buildTheme("default", "dark", defaultDarkPalette())
}

// buildTheme constructs a Theme from a color palette.
func buildTheme(name, mode string, c schema.ColorPalette) *Theme {
	t := &Theme{
		Name:   name,
		Mode:   mode,
		Colors: c,
	}

	// Message styles
	t.ChatMessage = lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Text))

	t.UserMessage = lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Primary)).
		Bold(true)

	t.AssistantMessage = lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Text))

	// Input styles
	t.Input = lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Text)).
		Background(lipgloss.Color(c.Surface)).
		Padding(0, 1)

	t.InputFocused = lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Text)).
		Background(lipgloss.Color(c.Surface)).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(c.Primary)).
		Padding(0, 1)

	t.InputPlaceholder = lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.TextMuted)).
		Italic(true)

	// Menu styles
	t.Menu = lipgloss.NewStyle().
		Background(lipgloss.Color(c.Surface)).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(c.Border)).
		Padding(0, 1)

	t.MenuItem = lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Text)).
		Padding(0, 2)

	t.MenuItemSelected = lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Background)).
		Background(lipgloss.Color(c.Primary)).
		Bold(true).
		Padding(0, 2)

	t.MenuItemDescription = lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.TextMuted)).
		Italic(true)

	// Status bar styles
	t.StatusBar = lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Text)).
		Background(lipgloss.Color(c.Surface))

	t.StatusBarItem = lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Text)).
		Background(lipgloss.Color(c.Surface)).
		Padding(0, 1)

	// Common element styles
	t.Border = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(c.Border))

	t.Title = lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Primary)).
		Bold(true)

	t.Subtitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Secondary)).
		Italic(true)

	// Semantic styles
	t.Error = lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Error)).
		Bold(true)

	t.Success = lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Success)).
		Bold(true)

	t.Warning = lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Warning)).
		Bold(true)

	t.Info = lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Info))

	// Color-based styles
	t.Muted = lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.TextMuted))

	t.Primary = lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Primary))

	t.Secondary = lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Secondary))

	t.Accent = lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Accent))

	return t
}

// defaultDarkPalette returns the default dark color palette.
func defaultDarkPalette() schema.ColorPalette {
	return schema.ColorPalette{
		Primary:    "#8B5CF6", // Violet
		Secondary:  "#10B981", // Emerald
		Background: "#0F172A", // Slate-900
		Surface:    "#1E293B", // Slate-800
		Text:       "#F1F5F9", // Slate-100
		TextMuted:  "#94A3B8", // Slate-400
		Border:     "#334155", // Slate-700
		Error:      "#EF4444", // Red-500
		Success:    "#22C55E", // Green-500
		Warning:    "#F59E0B", // Amber-500
		Info:       "#3B82F6", // Blue-500
		Accent:     "#EC4899", // Pink-500
	}
}

// BuiltinThemes returns all built-in themes.
func BuiltinThemes() map[string]*Theme {
	return map[string]*Theme{
		"default":      defaultDark(),
		"violet_dark":  defaultDark(),
		"tokyo_night":  tokyoNight(),
		"dracula":      dracula(),
		"nord":         nord(),
		"violet_light": defaultLight(),
		"github_light": githubLight(),
	}
}

// GetTheme returns a builtin theme by name.
// Returns the default theme if name is not found.
func GetTheme(name string) *Theme {
	themes := BuiltinThemes()
	if t, ok := themes[name]; ok {
		return t
	}
	return DefaultTheme()
}

// defaultDark is the default dark theme - Violet/Emerald on Slate.
func defaultDark() *Theme {
	return buildTheme("violet_dark", "dark", schema.ColorPalette{
		Primary:    "#8B5CF6", // Violet-500
		Secondary:  "#10B981", // Emerald-500
		Background: "#0F172A", // Slate-900
		Surface:    "#1E293B", // Slate-800
		Text:       "#F1F5F9", // Slate-100
		TextMuted:  "#94A3B8", // Slate-400
		Border:     "#334155", // Slate-700
		Error:      "#EF4444", // Red-500
		Success:    "#22C55E", // Green-500
		Warning:    "#F59E0B", // Amber-500
		Info:       "#3B82F6", // Blue-500
		Accent:     "#EC4899", // Pink-500
	})
}

// tokyoNight implements the Tokyo Night color scheme.
func tokyoNight() *Theme {
	return buildTheme("tokyo_night", "dark", schema.ColorPalette{
		Primary:    "#7AA2F7", // Blue
		Secondary:  "#9ECE6A", // Green
		Background: "#1A1B26", // Background
		Surface:    "#24283B", // Surface
		Text:       "#C0CAF5", // Foreground
		TextMuted:  "#565F89", // Comment
		Border:     "#414868", // Border
		Error:      "#F7768E", // Red
		Success:    "#9ECE6A", // Green
		Warning:    "#E0AF68", // Yellow
		Info:       "#7DCFFF", // Cyan
		Accent:     "#BB9AF7", // Magenta
	})
}

// dracula implements the Dracula color scheme.
func dracula() *Theme {
	return buildTheme("dracula", "dark", schema.ColorPalette{
		Primary:    "#BD93F9", // Purple
		Secondary:  "#50FA7B", // Green
		Background: "#282A36", // Background
		Surface:    "#44475A", // Current Line
		Text:       "#F8F8F2", // Foreground
		TextMuted:  "#6272A4", // Comment
		Border:     "#44475A", // Selection
		Error:      "#FF5555", // Red
		Success:    "#50FA7B", // Green
		Warning:    "#FFB86C", // Orange
		Info:       "#8BE9FD", // Cyan
		Accent:     "#FF79C6", // Pink
	})
}

// nord implements the Nord color scheme.
func nord() *Theme {
	return buildTheme("nord", "dark", schema.ColorPalette{
		Primary:    "#88C0D0", // Nord8 - Frost
		Secondary:  "#A3BE8C", // Nord14 - Aurora Green
		Background: "#2E3440", // Nord0 - Polar Night
		Surface:    "#3B4252", // Nord1
		Text:       "#ECEFF4", // Nord6 - Snow Storm
		TextMuted:  "#4C566A", // Nord3
		Border:     "#434C5E", // Nord2
		Error:      "#BF616A", // Nord11 - Aurora Red
		Success:    "#A3BE8C", // Nord14 - Aurora Green
		Warning:    "#EBCB8B", // Nord13 - Aurora Yellow
		Info:       "#81A1C1", // Nord9 - Frost
		Accent:     "#B48EAD", // Nord15 - Aurora Purple
	})
}

// defaultLight is the light version of the default theme.
func defaultLight() *Theme {
	return buildTheme("violet_light", "light", schema.ColorPalette{
		Primary:    "#7C3AED", // Violet-600
		Secondary:  "#059669", // Emerald-600
		Background: "#FFFFFF", // White
		Surface:    "#F8FAFC", // Slate-50
		Text:       "#1E293B", // Slate-800
		TextMuted:  "#64748B", // Slate-500
		Border:     "#CBD5E1", // Slate-300
		Error:      "#DC2626", // Red-600
		Success:    "#16A34A", // Green-600
		Warning:    "#D97706", // Amber-600
		Info:       "#2563EB", // Blue-600
		Accent:     "#DB2777", // Pink-600
	})
}

// githubLight implements the GitHub light color scheme.
func githubLight() *Theme {
	return buildTheme("github_light", "light", schema.ColorPalette{
		Primary:    "#0969DA", // Blue
		Secondary:  "#1A7F37", // Green
		Background: "#FFFFFF", // White
		Surface:    "#F6F8FA", // Gray background
		Text:       "#1F2328", // Foreground
		TextMuted:  "#656D76", // Muted
		Border:     "#D0D7DE", // Border
		Error:      "#CF222E", // Red
		Success:    "#1A7F37", // Green
		Warning:    "#9A6700", // Yellow
		Info:       "#0969DA", // Blue
		Accent:     "#8250DF", // Purple
	})
}
