// Package tui provides BubbleTea-based TUI components
package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Theme represents a color theme
type Theme string

const (
	// ThemeDracula is the Dracula theme
	ThemeDracula Theme = "dracula"
	// ThemeDefault is the default theme
	ThemeDefault Theme = "default"
)

// Dracula color palette
var draculaColors = struct {
	Background    string
	CurrentLine   string
	Foreground    string
	Comment       string
	Cyan          string
	Green         string
	Orange        string
	Pink          string
	Purple        string
	Red           string
	Yellow        string
	BrightWhite   string
	LightGray     string
	Selection     string
	Inactive      string
	Success       string
	Warning       string
	Error         string
	Info          string
}{
	Background:    "#282a36",
	CurrentLine:   "#44475a",
	Foreground:    "#f8f8f2",
	Comment:       "#6272a4",
	Cyan:          "#8be9fd",
	Green:         "#50fa7b",
	Orange:        "#ffb86c",
	Pink:          "#ff79c6",
	Purple:        "#bd93f9",
	Red:           "#ff5555",
	Yellow:        "#f1fa8c",
	BrightWhite:   "#ffffff",
	LightGray:     "#a4a4a4",
	Selection:     "#44475a",
	Inactive:      "#6272a4",
	Success:       "#50fa7b",
	Warning:       "#ffb86c",
	Error:         "#ff5555",
	Info:          "#8be9fd",
}

// Default color palette
var defaultColors = struct {
	Background    string
	CurrentLine   string
	Foreground    string
	Comment       string
	Cyan          string
	Green         string
	Orange        string
	Pink          string
	Purple        string
	Red           string
	Yellow        string
	BrightWhite   string
	LightGray     string
	Selection     string
	Inactive      string
	Success       string
	Warning       string
	Error         string
	Info          string
}{
	Background:    "#000000",
	CurrentLine:   "#333333",
	Foreground:    "#ffffff",
	Comment:       "#888888",
	Cyan:          "#00ffff",
	Green:         "#00ff00",
	Orange:        "#ffaa00",
	Pink:          "#ff00ff",
	Purple:        "#aa00ff",
	Red:           "#ff0000",
	Yellow:        "#ffff00",
	BrightWhite:   "#ffffff",
	LightGray:     "#aaaaaa",
	Selection:     "#444444",
	Inactive:      "#666666",
	Success:       "#00ff00",
	Warning:       "#ffaa00",
	Error:         "#ff0000",
	Info:          "#00ffff",
}

// Styles holds all TUI styles
type Styles struct {
	Theme         Theme
	Colors        Colors
	App           AppStyles
	Browser       BrowserStyles
	Editor        EditorStyles
	Chat          ChatStyles
	StatusBar     StatusBarStyles
	Help          HelpStyles
}

// Colors holds color definitions
type Colors struct {
	Background    string
	CurrentLine   string
	Foreground    string
	Comment       string
	Cyan          string
	Green         string
	Orange        string
	Pink          string
	Purple        string
	Red           string
	Yellow        string
	BrightWhite   string
	LightGray     string
	Selection     string
	Inactive      string
	Success       string
	Warning       string
	Error         string
	Info          string
}

// AppStyles holds application-level styles
type AppStyles struct {
	Container     lipgloss.Style
	Title         lipgloss.Style
	Subtitle      lipgloss.Style
	PanelBorder   lipgloss.Style
	FocusedBorder lipgloss.Style
}

// BrowserStyles holds file browser styles
type BrowserStyles struct {
	Container     lipgloss.Style
	Tree          lipgloss.Style
	Directory     lipgloss.Style
	File          lipgloss.Style
	Selected      lipgloss.Style
	Icon          lipgloss.Style
	GitModified   lipgloss.Style
	GitUntracked  lipgloss.Style
	GitStaged     lipgloss.Style
	Indent        lipgloss.Style
}

// EditorStyles holds editor styles
type EditorStyles struct {
	Container     lipgloss.Style
	LineNumber    lipgloss.Style
	Content       lipgloss.Style
	SyntaxKeyword lipgloss.Style
	SyntaxString  lipgloss.Style
	SyntaxComment lipgloss.Style
	SyntaxNumber  lipgloss.Style
	SyntaxFunction lipgloss.Style
}

// ChatStyles holds chat panel styles
type ChatStyles struct {
	Container     lipgloss.Style
	Message       lipgloss.Style
	UserMessage   lipgloss.Style
	AgentMessage  lipgloss.Style
	SystemMessage lipgloss.Style
	CodeBlock     lipgloss.Style
	Input         lipgloss.Style
	InputPrompt   lipgloss.Style
	Timestamp     lipgloss.Style
	ScrollIndicator lipgloss.Style
}

// StatusBarStyles holds status bar styles
type StatusBarStyles struct {
	Container     lipgloss.Style
	Mode          lipgloss.Style
	Info          lipgloss.Style
	Key           lipgloss.Style
	Value         lipgloss.Style
	Success       lipgloss.Style
	Warning       lipgloss.Style
}

// HelpStyles holds help/styles
type HelpStyles struct {
	Container     lipgloss.Style
	Key           lipgloss.Style
	Desc          lipgloss.Style
	Separator     lipgloss.Style
}

// NewStyles creates a new Styles instance with the specified theme
func NewStyles(theme Theme) Styles {
	var colors Colors
	
	switch theme {
	case ThemeDracula:
		colors = Colors(draculaColors)
	default:
		colors = Colors(defaultColors)
	}
	
	s := Styles{
		Theme:  theme,
		Colors: colors,
	}
	
	s.App = AppStyles{
		Container: lipgloss.NewStyle().
			Background(lipgloss.Color(colors.Background)),
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colors.Purple)).
			PaddingLeft(1).
			PaddingRight(1),
		Subtitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Comment)).
			PaddingLeft(1),
		PanelBorder: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(colors.CurrentLine)),
		FocusedBorder: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(colors.Purple)),
	}
	
	s.Browser = BrowserStyles{
		Container: lipgloss.NewStyle().
			Background(lipgloss.Color(colors.Background)),
		Tree: lipgloss.NewStyle().
			PaddingLeft(1),
		Directory: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Cyan)).
			Bold(true),
		File: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Foreground)),
		Selected: lipgloss.NewStyle().
			Background(lipgloss.Color(colors.Selection)).
			Foreground(lipgloss.Color(colors.Foreground)),
		Icon: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Orange)),
		GitModified: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Yellow)),
		GitUntracked: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Red)),
		GitStaged: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Green)),
		Indent: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Comment)),
	}
	
	s.Editor = EditorStyles{
		Container: lipgloss.NewStyle().
			Background(lipgloss.Color(colors.Background)),
		LineNumber: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Comment)).
			PaddingRight(1),
		Content: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Foreground)),
		SyntaxKeyword: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Pink)),
		SyntaxString: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Yellow)),
		SyntaxComment: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Comment)),
		SyntaxNumber: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Purple)),
		SyntaxFunction: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Green)),
	}
	
	s.Chat = ChatStyles{
		Container: lipgloss.NewStyle().
			Background(lipgloss.Color(colors.Background)),
		Message: lipgloss.NewStyle().
			PaddingLeft(1).
			PaddingRight(1).
			MarginBottom(1),
		UserMessage: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Cyan)).
			Bold(true),
		AgentMessage: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Foreground)),
		SystemMessage: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Yellow)).
			Italic(true),
		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color(colors.CurrentLine)).
			Foreground(lipgloss.Color(colors.Foreground)).
			PaddingLeft(1).
			PaddingRight(1),
		Input: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Foreground)).
			Background(lipgloss.Color(colors.CurrentLine)),
		InputPrompt: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Green)).
			Bold(true),
		Timestamp: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Comment)).
			Italic(true),
		ScrollIndicator: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Purple)),
	}
	
	s.StatusBar = StatusBarStyles{
		Container: lipgloss.NewStyle().
			Background(lipgloss.Color(colors.CurrentLine)).
			Foreground(lipgloss.Color(colors.Foreground)),
		Mode: lipgloss.NewStyle().
			Background(lipgloss.Color(colors.Purple)).
			Foreground(lipgloss.Color(colors.Background)).
			Bold(true).
			PaddingLeft(1).
			PaddingRight(1),
		Info: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Foreground)),
		Key: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Pink)),
		Value: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Cyan)),
		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Success)),
		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Warning)),
	}
	
	s.Help = HelpStyles{
		Container: lipgloss.NewStyle().
			Background(lipgloss.Color(colors.Background)),
		Key: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Pink)).
			Bold(true),
		Desc: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Comment)),
		Separator: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.CurrentLine)),
	}
	
	return s
}

// DefaultStyles returns default Dracula styles
func DefaultStyles() Styles {
	return NewStyles(ThemeDracula)
}

// GetFileIcon returns an icon for a file based on extension
func GetFileIcon(filename string) string {
	ext := getFileExtension(filename)
	
	icons := map[string]string{
		".go":    "🐹",
		".mod":   "🐹",
		".sum":   "🐹",
		".rs":    "🦀",
		".py":    "🐍",
		".js":    "📜",
		".ts":    "📘",
		".jsx":   "⚛️",
		".tsx":   "⚛️",
		".html":  "🌐",
		".css":   "🎨",
		".scss":  "🎨",
		".json":  "📋",
		".yaml":  "📋",
		".yml":   "📋",
		".toml":  "📋",
		".md":    "📝",
		".txt":   "📄",
		".sh":    "🔧",
		".bash":  "🔧",
		".zsh":   "🔧",
		".fish":  "🔧",
		".dockerfile": "🐳",
		".sql":   "🗄️",
		".proto": "📡",
		".graphql": "📡",
		".vue":   "💚",
		".svelte": "🧡",
		".rb":    "💎",
		".php":   "🐘",
		".java":  "☕",
		".kt":    "🎯",
		".scala": "⚡",
		".c":     "🔷",
		".cpp":   "🔷",
		".h":     "🔷",
		".hpp":   "🔷",
		".cs":    "🔷",
		".swift": "🐦",
		".m":     "🍎",
		".r":     "📊",
		".jl":    "🔴",
		".ex":    "🟣",
		".exs":   "🟣",
		".elm":   "🌳",
		".hs":    "🦄",
		".lhs":   "🦄",
		".clj":   "🌰",
		".cljs":  "🌰",
		".lisp":  "🔄",
		".vim":   "💚",
		".lua":   "🌙",
		".nim":   "👑",
		".zig":   "⚡",
		".v":     "✅",
		".odin":  "🔷",
		".wat":   "🧊",
		".wasm":  "🧊",
	}
	
	if icon, ok := icons[ext]; ok {
		return icon
	}
	return "📄"
}

// getFileExtension extracts file extension from filename
func getFileExtension(filename string) string {
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '.' {
			return filename[i:]
		}
		if filename[i] == '/' || filename[i] == '\\' {
			break
		}
	}
	return ""
}
