// Package ui provides the theming and styling system for Cortex's TUI framework.
// It implements a comprehensive color palette system using Charmbracelet's lipgloss library.
package ui

// ═══════════════════════════════════════════════════════════════════════════════
// THEME DEFINITION
// ═══════════════════════════════════════════════════════════════════════════════

// Theme defines a complete color palette for the Cortex TUI.
// Each theme provides semantic colors that are applied consistently across all UI components.
// Colors are stored as strings (hex codes) for compatibility with lipgloss.Color().
type Theme struct {
	// Metadata
	Name string // Human-readable theme name

	// Base Colors - Foundation of the UI (as hex strings)
	Background string // Main background color
	Foreground string // Primary text color
	Border     string // Borders and dividers

	// Semantic Colors - Component-level meaning
	Primary   string // Primary actions, emphasis, focus
	Secondary string // Supporting elements, subtitles
	Success   string // Success states, confirmations
	Warning   string // Warnings, important notices
	Error     string // Error states, critical alerts
	Muted     string // Dimmed text, placeholders

	// Layout Background Colors - Layered UI elements
	HeaderBg string // Header/title bar background
	FooterBg string // Footer/status bar background
	InputBg  string // Input field background
	ModalBg  string // Modal overlay background

	// Message Colors - Chat message styling
	UserMessageFg      string // User message text color
	UserMessageBg      string // User message background
	AssistantMessageFg string // Assistant message text color
	AssistantMessageBg string // Assistant message background

	// Markdown Rendering
	GlamourStyle string // Glamour theme name for markdown rendering
}

// ═══════════════════════════════════════════════════════════════════════════════
// BUILT-IN THEMES
// ═══════════════════════════════════════════════════════════════════════════════

// ThemeDefault provides a VS Code dark-inspired theme.
// This is the default theme for Cortex, optimized for readability and low eye strain.
var ThemeDefault = Theme{
	Name: "Default (VS Code Dark)",

	// Base Colors
	Background: "#1e1e1e", // VS Code editor background
	Foreground: "#d4d4d4", // Standard text
	Border:     "#3e3e42", // Subtle borders

	// Semantic Colors
	Primary:   "#007acc", // Blue - actions, links
	Secondary: "#9cdcfe", // Cyan - types, secondary info
	Success:   "#4ec9b0", // Teal - success, confirmations
	Warning:   "#dcdcaa", // Yellow - warnings
	Error:     "#f48771", // Salmon - errors
	Muted:     "#6a737d", // Gray - muted text

	// Layout Backgrounds
	HeaderBg: "#252526", // Slightly lighter header
	FooterBg: "#181818", // Slightly darker footer
	InputBg:  "#1e1e1e", // Match main background
	ModalBg:  "#252526", // Elevated modal background

	// Message Colors
	UserMessageFg:      "#d4d4d4", // Same as foreground
	UserMessageBg:      "#2d2d30", // Slightly elevated
	AssistantMessageFg: "#9cdcfe", // Secondary color for AI
	AssistantMessageBg: "#252526", // Match header

	// Markdown
	GlamourStyle: "dark",
}

// ThemeDracula provides the popular Dracula color scheme.
// Known for its vibrant purple and pink tones with excellent contrast.
var ThemeDracula = Theme{
	Name: "Dracula",

	// Base Colors
	Background: "#282a36", // Dracula background
	Foreground: "#f8f8f2", // Dracula foreground
	Border:     "#6272a4", // Dracula comment color

	// Semantic Colors
	Primary:   "#bd93f9", // Purple - main actions
	Secondary: "#8be9fd", // Cyan - secondary info
	Success:   "#50fa7b", // Green - success states
	Warning:   "#f1fa8c", // Yellow - warnings
	Error:     "#ff5555", // Red - errors
	Muted:     "#6272a4", // Comment gray

	// Layout Backgrounds
	HeaderBg: "#21222c", // Slightly darker
	FooterBg: "#191a21", // Even darker
	InputBg:  "#282a36", // Match main background
	ModalBg:  "#343746", // Elevated surface

	// Message Colors
	UserMessageFg:      "#f8f8f2", // Foreground
	UserMessageBg:      "#343746", // Elevated
	AssistantMessageFg: "#8be9fd", // Cyan
	AssistantMessageBg: "#21222c", // Darker

	// Markdown
	GlamourStyle: "dracula",
}

// ThemeNord provides the Nord color palette.
// A arctic, north-bluish theme with excellent readability and low contrast.
var ThemeNord = Theme{
	Name: "Nord",

	// Base Colors - Polar Night palette
	Background: "#2e3440", // Nord0
	Foreground: "#eceff4", // Snow Storm (Nord6)
	Border:     "#4c566a", // Nord3

	// Semantic Colors - Frost & Aurora palettes
	Primary:   "#88c0d0", // Frost cyan (Nord8)
	Secondary: "#81a1c1", // Frost blue (Nord9)
	Success:   "#a3be8c", // Aurora green (Nord14)
	Warning:   "#ebcb8b", // Aurora yellow (Nord13)
	Error:     "#bf616a", // Aurora red (Nord11)
	Muted:     "#4c566a", // Nord3

	// Layout Backgrounds
	HeaderBg: "#3b4252", // Nord1 - lighter
	FooterBg: "#2e3440", // Nord0 - match background
	InputBg:  "#2e3440", // Nord0
	ModalBg:  "#3b4252", // Nord1 - elevated

	// Message Colors
	UserMessageFg:      "#eceff4", // Snow Storm
	UserMessageBg:      "#3b4252", // Nord1
	AssistantMessageFg: "#88c0d0", // Frost cyan
	AssistantMessageBg: "#2e3440", // Nord0

	// Markdown
	GlamourStyle: "dark",
}

// ThemeGruvbox provides the retro groove color scheme.
// Warm, earthy tones inspired by badwolf, jellybeans and retro groove.
var ThemeGruvbox = Theme{
	Name: "Gruvbox Dark",

	// Base Colors
	Background: "#282828", // Gruvbox dark background
	Foreground: "#ebdbb2", // Gruvbox light foreground
	Border:     "#504945", // Gruvbox dark3

	// Semantic Colors
	Primary:   "#fe8019", // Orange - vibrant actions
	Secondary: "#83a598", // Blue - secondary info
	Success:   "#b8bb26", // Bright green - success
	Warning:   "#fabd2f", // Bright yellow - warnings
	Error:     "#fb4934", // Bright red - errors
	Muted:     "#928374", // Gray - muted text

	// Layout Backgrounds
	HeaderBg: "#3c3836", // Dark1 - lighter
	FooterBg: "#1d2021", // Hard contrast - darker
	InputBg:  "#282828", // Match main background
	ModalBg:  "#3c3836", // Dark1 - elevated

	// Message Colors
	UserMessageFg:      "#ebdbb2", // Light foreground
	UserMessageBg:      "#3c3836", // Dark1
	AssistantMessageFg: "#83a598", // Blue
	AssistantMessageBg: "#282828", // Background

	// Markdown
	GlamourStyle: "dark",
}

// ═══════════════════════════════════════════════════════════════════════════════
// THEME REGISTRY
// ═══════════════════════════════════════════════════════════════════════════════

// availableThemes maps theme IDs to their Theme definitions.
var availableThemes = map[string]Theme{
	"default":  ThemeDefault,
	"dracula":  ThemeDracula,
	"nord":     ThemeNord,
	"gruvbox":  ThemeGruvbox,
}

// GetTheme retrieves a theme by ID.
// If the theme doesn't exist, returns ThemeDefault.
func GetTheme(id string) Theme {
	if theme, ok := availableThemes[id]; ok {
		return theme
	}
	return ThemeDefault
}

// ThemeNames returns a list of all available theme IDs.
func ThemeNames() []string {
	names := make([]string, 0, len(availableThemes))
	for name := range availableThemes {
		names = append(names, name)
	}
	return names
}
