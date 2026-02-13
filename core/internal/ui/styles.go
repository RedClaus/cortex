package ui

import "github.com/charmbracelet/lipgloss"

// ═══════════════════════════════════════════════════════════════════════════════
// STYLES STRUCT
// ═══════════════════════════════════════════════════════════════════════════════

// Styles contains pre-computed lipgloss styles for all UI components.
// This separates visual styling from business logic and layout code.
type Styles struct {
	// Theme reference
	theme Theme

	// ─────────────────────────────────────────────────────────────────────────
	// Layout Styles - Main UI regions
	// ─────────────────────────────────────────────────────────────────────────

	// Header is the top navigation/title bar
	Header lipgloss.Style

	// ChatArea is the main scrollable message container
	ChatArea lipgloss.Style

	// InputArea is the user input region
	InputArea lipgloss.Style

	// Footer is the bottom status/help bar
	Footer lipgloss.Style

	// ─────────────────────────────────────────────────────────────────────────
	// Header Component Styles
	// ─────────────────────────────────────────────────────────────────────────

	// Logo is the Cortex branding/title in the header
	Logo lipgloss.Style

	// HeaderContext shows current context (model, mode, etc.)
	HeaderContext lipgloss.Style

	// HeaderStatus shows connection/session status
	HeaderStatus lipgloss.Style

	// ─────────────────────────────────────────────────────────────────────────
	// Message Styles - Chat message components
	// ─────────────────────────────────────────────────────────────────────────

	// UserLabel is the "You:" label prefix
	UserLabel lipgloss.Style

	// UserMessage is the user's message text
	UserMessage lipgloss.Style

	// AssistantLabel is the "Assistant:" label prefix
	AssistantLabel lipgloss.Style

	// AssistantMessage is the assistant's response text
	AssistantMessage lipgloss.Style

	// SystemMessage is for system notifications and info
	SystemMessage lipgloss.Style

	// ─────────────────────────────────────────────────────────────────────────
	// Footer Mode Styles - Different footer states
	// ─────────────────────────────────────────────────────────────────────────

	// FooterNormal is the default footer style (normal conversation mode)
	FooterNormal lipgloss.Style

	// FooterYolo is the footer in YOLO mode (auto-execute commands)
	FooterYolo lipgloss.Style

	// FooterPlan is the footer in plan mode (strategic planning)
	FooterPlan lipgloss.Style

	// ─────────────────────────────────────────────────────────────────────────
	// Modal Styles - Overlay dialogs
	// ─────────────────────────────────────────────────────────────────────────

	// ModalBorder is the modal container with border
	ModalBorder lipgloss.Style

	// ModalTitle is the modal header/title
	ModalTitle lipgloss.Style

	// ModalDim is the background overlay/dimming effect
	ModalDim lipgloss.Style

	// ─────────────────────────────────────────────────────────────────────────
	// Additional Utility Styles
	// ─────────────────────────────────────────────────────────────────────────

	// Spinner is for loading/thinking indicators
	Spinner lipgloss.Style

	// CodeBlock is for inline code and code blocks
	CodeBlock lipgloss.Style

	// Separator is for horizontal dividers
	Separator lipgloss.Style

	// Timestamp is for message timestamps
	Timestamp lipgloss.Style

	// Badge is for status badges and labels
	Badge lipgloss.Style

	// ErrorBox is for error message containers
	ErrorBox lipgloss.Style

	// SuccessBox is for success confirmation containers
	SuccessBox lipgloss.Style
}

// ═══════════════════════════════════════════════════════════════════════════════
// STYLE INITIALIZATION
// ═══════════════════════════════════════════════════════════════════════════════

// NewStyles creates a complete Styles instance from a theme.
// All styles are pre-computed for maximum rendering performance.
func NewStyles(theme Theme) Styles {
	s := Styles{
		theme: theme,
	}

	// ─────────────────────────────────────────────────────────────────────────
	// Layout Styles
	// ─────────────────────────────────────────────────────────────────────────

	s.Header = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Foreground)).
		Background(lipgloss.Color(theme.HeaderBg)).
		Bold(true).
		Padding(0, 2).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(lipgloss.Color(theme.Border))

	s.ChatArea = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Foreground)).
		Background(lipgloss.Color(theme.Background)).
		Padding(1, 2)

	s.InputArea = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Foreground)).
		Background(lipgloss.Color(theme.InputBg)).
		Padding(1, 2).
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderForeground(lipgloss.Color(theme.Border))

	s.Footer = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Muted)).
		Background(lipgloss.Color(theme.FooterBg)).
		Padding(0, 2).
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderForeground(lipgloss.Color(theme.Border))

	// ─────────────────────────────────────────────────────────────────────────
	// Header Component Styles
	// ─────────────────────────────────────────────────────────────────────────

	s.Logo = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Primary)).
		Background(lipgloss.Color(theme.HeaderBg)).
		Bold(true).
		Italic(false)

	s.HeaderContext = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Secondary)).
		Background(lipgloss.Color(theme.HeaderBg)).
		Italic(true).
		MarginLeft(2)

	s.HeaderStatus = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Success)).
		Background(lipgloss.Color(theme.HeaderBg)).
		Bold(false).
		MarginLeft(1)

	// ─────────────────────────────────────────────────────────────────────────
	// Message Styles
	// ─────────────────────────────────────────────────────────────────────────

	s.UserLabel = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Primary)).
		Background(lipgloss.Color(theme.Background)).
		Bold(true).
		MarginRight(1)

	s.UserMessage = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Foreground)).
		Background(lipgloss.Color(theme.Background)).
		PaddingLeft(1)

	s.AssistantLabel = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Secondary)).
		Background(lipgloss.Color(theme.Background)).
		Bold(true).
		MarginRight(1)

	s.AssistantMessage = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Foreground)).
		Background(lipgloss.Color(theme.Background)).
		PaddingLeft(1)

	s.SystemMessage = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Muted)).
		Background(lipgloss.Color(theme.Background)).
		Italic(true).
		PaddingLeft(2)

	// ─────────────────────────────────────────────────────────────────────────
	// Footer Mode Styles
	// ─────────────────────────────────────────────────────────────────────────

	s.FooterNormal = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Muted)).
		Background(lipgloss.Color(theme.FooterBg)).
		Padding(0, 1)

	s.FooterYolo = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Warning)).
		Background(lipgloss.Color(theme.FooterBg)).
		Bold(true).
		Padding(0, 1)

	s.FooterPlan = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Primary)).
		Background(lipgloss.Color(theme.FooterBg)).
		Bold(false).
		Italic(true).
		Padding(0, 1)

	// ─────────────────────────────────────────────────────────────────────────
	// Modal Styles
	// ─────────────────────────────────────────────────────────────────────────

	s.ModalBorder = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Foreground)).
		Background(lipgloss.Color(theme.ModalBg)).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.Primary)).
		Padding(1, 3)

	s.ModalTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Primary)).
		Background(lipgloss.Color(theme.ModalBg)).
		Bold(true).
		Align(lipgloss.Center).
		MarginBottom(1)

	s.ModalDim = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#444444")).
		Background(lipgloss.Color("0")) // Transparent/default terminal background

	// ─────────────────────────────────────────────────────────────────────────
	// Utility Styles
	// ─────────────────────────────────────────────────────────────────────────

	s.Spinner = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Primary)).
		Background(lipgloss.Color(theme.Background)).
		Bold(true)

	s.CodeBlock = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Secondary)).
		Background(lipgloss.Color(theme.InputBg)).
		Padding(0, 1).
		BorderStyle(lipgloss.NormalBorder()).
		BorderLeft(true).
		BorderForeground(lipgloss.Color(theme.Border))

	s.Separator = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Border)).
		Background(lipgloss.Color(theme.Background))

	s.Timestamp = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Muted)).
		Background(lipgloss.Color(theme.Background)).
		Italic(true).
		MarginLeft(1)

	s.Badge = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Background)).
		Background(lipgloss.Color(theme.Primary)).
		Bold(true).
		Padding(0, 1).
		MarginLeft(1)

	s.ErrorBox = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Error)).
		Background(lipgloss.Color(theme.Background)).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.Error)).
		Padding(0, 1).
		Bold(true)

	s.SuccessBox = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Success)).
		Background(lipgloss.Color(theme.Background)).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.Success)).
		Padding(0, 1)

	return s
}

// ═══════════════════════════════════════════════════════════════════════════════
// STYLE HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

// Theme returns the underlying theme used by these styles.
func (s *Styles) Theme() Theme {
	return s.theme
}

// Adapt applies theme-specific adaptations to a custom style.
// Useful for component-specific styling while maintaining theme consistency.
func (s *Styles) Adapt(style lipgloss.Style) lipgloss.Style {
	return style.
		Foreground(lipgloss.Color(s.theme.Foreground)).
		Background(lipgloss.Color(s.theme.Background))
}

// RenderHorizontalLine renders a horizontal separator line of the given width.
func (s *Styles) RenderHorizontalLine(width int) string {
	line := ""
	for i := 0; i < width; i++ {
		line += "─"
	}
	return s.Separator.Render(line)
}

// RenderBox renders text in a bordered box with the given style and title.
func (s *Styles) RenderBox(title, content string, style lipgloss.Style) string {
	if title != "" {
		titleStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(s.theme.Primary)).
			Bold(true).
			Padding(0, 1)

		renderedTitle := titleStyle.Render(title)

		return style.Render(renderedTitle + "\n" + content)
	}
	return style.Render(content)
}

// RenderMessage formats a chat message with label and content.
func (s *Styles) RenderMessage(labelStyle, messageStyle lipgloss.Style, label, message string) string {
	renderedLabel := labelStyle.Render(label)
	renderedMessage := messageStyle.Render(message)
	return lipgloss.JoinHorizontal(lipgloss.Top, renderedLabel, renderedMessage)
}

// RenderUserMessage formats a user message.
func (s *Styles) RenderUserMessage(message string) string {
	return s.RenderMessage(s.UserLabel, s.UserMessage, "You:", message)
}

// RenderAssistantMessage formats an assistant message.
func (s *Styles) RenderAssistantMessage(message string) string {
	return s.RenderMessage(s.AssistantLabel, s.AssistantMessage, "Assistant:", message)
}

// RenderSystemMessage formats a system message.
func (s *Styles) RenderSystemMessage(message string) string {
	return s.SystemMessage.Render("ℹ " + message)
}

// RenderFooter formats footer content based on the current mode.
func (s *Styles) RenderFooter(mode string, content string) string {
	switch mode {
	case "yolo":
		return s.FooterYolo.Render("⚡ YOLO MODE: " + content)
	case "plan":
		return s.FooterPlan.Render("📋 PLAN MODE: " + content)
	default:
		return s.FooterNormal.Render(content)
	}
}

// RenderBadge formats a badge with the given text.
func (s *Styles) RenderBadge(text string) string {
	return s.Badge.Render(text)
}

// RenderError formats an error message in an error box.
func (s *Styles) RenderError(message string) string {
	return s.ErrorBox.Render("✗ " + message)
}

// RenderSuccess formats a success message in a success box.
func (s *Styles) RenderSuccess(message string) string {
	return s.SuccessBox.Render("✓ " + message)
}

// RenderCode formats a code block.
func (s *Styles) RenderCode(code string) string {
	return s.CodeBlock.Render(code)
}

// ═══════════════════════════════════════════════════════════════════════════════
// GLOBAL DEFAULT STYLES
// ═══════════════════════════════════════════════════════════════════════════════

var defaultStyles *Styles

// DefaultStyles returns the default Styles instance.
// Uses ThemeDefault if not previously initialized.
func DefaultStyles() Styles {
	if defaultStyles == nil {
		s := NewStyles(ThemeDefault)
		defaultStyles = &s
	}
	return *defaultStyles
}

// InitDefaultStyles initializes the global default styles with a specific theme.
func InitDefaultStyles(theme Theme) Styles {
	s := NewStyles(theme)
	defaultStyles = &s
	return s
}
