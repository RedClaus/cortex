// Package block provides block-specific lipgloss styles for the Cortex TUI.
package block

import (
	"github.com/charmbracelet/lipgloss"
)

// ═══════════════════════════════════════════════════════════════════════════════
// BLOCK STYLES STRUCT
// ═══════════════════════════════════════════════════════════════════════════════

// BlockStyles contains pre-computed lipgloss styles for block rendering.
// This provides the visual "chrome" for the block-based conversation model.
type BlockStyles struct {
	// ─────────────────────────────────────────────────────────────────────────
	// Block Container Styles
	// ─────────────────────────────────────────────────────────────────────────

	// UserBlock is the container style for user message blocks
	UserBlock lipgloss.Style

	// AssistantBlock is the container style for assistant response blocks
	AssistantBlock lipgloss.Style

	// ToolBlock is the container style for tool execution blocks
	ToolBlock lipgloss.Style

	// ThinkingBlock is the container style for AI thinking/reasoning blocks
	ThinkingBlock lipgloss.Style

	// CodeBlock is the container style for code blocks
	CodeBlock lipgloss.Style

	// ErrorBlock is the container style for error blocks
	ErrorBlock lipgloss.Style

	// SystemBlock is the container style for system message blocks
	SystemBlock lipgloss.Style

	// ─────────────────────────────────────────────────────────────────────────
	// Block Header Styles
	// ─────────────────────────────────────────────────────────────────────────

	// UserHeader is the header style for user blocks (icon + label)
	UserHeader lipgloss.Style

	// AssistantHeader is the header style for assistant blocks
	AssistantHeader lipgloss.Style

	// ToolHeader is the header style for tool blocks
	ToolHeader lipgloss.Style

	// ThinkingHeader is the header style for thinking blocks
	ThinkingHeader lipgloss.Style

	// CodeHeader is the header style for code blocks (language indicator)
	CodeHeader lipgloss.Style

	// ErrorHeader is the header style for error blocks
	ErrorHeader lipgloss.Style

	// SystemHeader is the header style for system blocks
	SystemHeader lipgloss.Style

	// ─────────────────────────────────────────────────────────────────────────
	// Block Content Styles
	// ─────────────────────────────────────────────────────────────────────────

	// BlockContent is the default content area style
	BlockContent lipgloss.Style

	// ToolInput is the style for tool input/arguments display
	ToolInput lipgloss.Style

	// ToolOutput is the style for tool output display
	ToolOutput lipgloss.Style

	// CodeContent is the style for code content
	CodeContent lipgloss.Style

	// ThinkingContent is the style for thinking content (muted)
	ThinkingContent lipgloss.Style

	// ─────────────────────────────────────────────────────────────────────────
	// Block Footer Styles
	// ─────────────────────────────────────────────────────────────────────────

	// BlockFooter is the action bar at the bottom of blocks
	BlockFooter lipgloss.Style

	// ActionButton is the style for individual action buttons
	ActionButton lipgloss.Style

	// ActionButtonActive is the style for active/selected action buttons
	ActionButtonActive lipgloss.Style

	// ─────────────────────────────────────────────────────────────────────────
	// Block Status Indicators
	// ─────────────────────────────────────────────────────────────────────────

	// StatusPending is the style for pending state indicator
	StatusPending lipgloss.Style

	// StatusStreaming is the style for streaming state indicator
	StatusStreaming lipgloss.Style

	// StatusComplete is the style for complete state indicator
	StatusComplete lipgloss.Style

	// StatusError is the style for error state indicator
	StatusError lipgloss.Style

	// StreamingCursor is the style for the streaming cursor (▌)
	StreamingCursor lipgloss.Style

	// ─────────────────────────────────────────────────────────────────────────
	// Block Chrome Elements
	// ─────────────────────────────────────────────────────────────────────────

	// CollapseIndicator is the style for collapse/expand indicator (▼/▲)
	CollapseIndicator lipgloss.Style

	// Timestamp is the style for block timestamps
	Timestamp lipgloss.Style

	// Duration is the style for tool execution duration
	Duration lipgloss.Style

	// BlockIcon is the style for block type icons
	BlockIcon lipgloss.Style

	// FocusedBorder is the border style when a block is focused
	FocusedBorder lipgloss.Style

	// BookmarkIndicator is the style for bookmarked blocks
	BookmarkIndicator lipgloss.Style

	// ─────────────────────────────────────────────────────────────────────────
	// Nested Block Styles
	// ─────────────────────────────────────────────────────────────────────────

	// NestedIndent is the left indentation for nested blocks
	NestedIndent lipgloss.Style

	// NestedConnector is the connector line for nested blocks (│)
	NestedConnector lipgloss.Style
}

// ═══════════════════════════════════════════════════════════════════════════════
// THEME COLORS
// ═══════════════════════════════════════════════════════════════════════════════

// BlockThemeColors defines colors used by block styles.
// This allows customization while maintaining a cohesive look.
type BlockThemeColors struct {
	// Base colors
	Background    string
	Foreground    string
	Muted         string
	Border        string
	BorderFocused string

	// Semantic colors
	Primary   string
	Secondary string
	Success   string
	Warning   string
	Error     string

	// Block-specific colors
	UserBg       string
	AssistantBg  string
	ToolBg       string
	ThinkingBg   string
	CodeBg       string
	ErrorBg      string
	SystemBg     string

	// Header colors
	UserHeader      string
	AssistantHeader string
	ToolHeader      string
	ThinkingHeader  string
	CodeHeader      string
}

// DefaultBlockColors returns the default color palette for blocks.
func DefaultBlockColors() BlockThemeColors {
	return BlockThemeColors{
		// Base colors
		Background:    "#1a1a1a",
		Foreground:    "#e0e0e0",
		Muted:         "#666666",
		Border:        "#333333",
		BorderFocused: "#4a9eff",

		// Semantic colors
		Primary:   "#4a9eff",
		Secondary: "#b794f4",
		Success:   "#48bb78",
		Warning:   "#ecc94b",
		Error:     "#fc8181",

		// Block backgrounds
		UserBg:       "#1e293b",
		AssistantBg:  "#1a1a1a",
		ToolBg:       "#1c1c1e",
		ThinkingBg:   "#1c1c1e",
		CodeBg:       "#0d1117",
		ErrorBg:      "#2d1b1b",
		SystemBg:     "#1a1a1a",

		// Header colors
		UserHeader:      "#4a9eff",
		AssistantHeader: "#b794f4",
		ToolHeader:      "#48bb78",
		ThinkingHeader:  "#666666",
		CodeHeader:      "#ecc94b",
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// STYLE INITIALIZATION
// ═══════════════════════════════════════════════════════════════════════════════

// NewBlockStyles creates a complete BlockStyles instance from theme colors.
func NewBlockStyles(colors BlockThemeColors) BlockStyles {
	s := BlockStyles{}

	// ─────────────────────────────────────────────────────────────────────────
	// Block Container Styles
	// ─────────────────────────────────────────────────────────────────────────

	baseBlock := lipgloss.NewStyle().
		Padding(1, 2).
		MarginBottom(1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colors.Border))

	s.UserBlock = baseBlock.
		Background(lipgloss.Color(colors.UserBg)).
		BorderForeground(lipgloss.Color(colors.UserHeader))

	s.AssistantBlock = baseBlock.
		Background(lipgloss.Color(colors.AssistantBg)).
		BorderForeground(lipgloss.Color(colors.AssistantHeader))

	s.ToolBlock = lipgloss.NewStyle().
		Padding(0, 1).
		MarginLeft(2).
		MarginBottom(1).
		BorderStyle(lipgloss.NormalBorder()).
		BorderLeft(true).
		BorderForeground(lipgloss.Color(colors.ToolHeader)).
		Background(lipgloss.Color(colors.ToolBg))

	s.ThinkingBlock = lipgloss.NewStyle().
		Padding(0, 1).
		MarginLeft(2).
		MarginBottom(1).
		BorderStyle(lipgloss.NormalBorder()).
		BorderLeft(true).
		BorderForeground(lipgloss.Color(colors.ThinkingHeader)).
		Background(lipgloss.Color(colors.ThinkingBg))

	s.CodeBlock = lipgloss.NewStyle().
		Padding(1, 2).
		MarginBottom(1).
		Background(lipgloss.Color(colors.CodeBg)).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colors.Border))

	s.ErrorBlock = baseBlock.
		Background(lipgloss.Color(colors.ErrorBg)).
		BorderForeground(lipgloss.Color(colors.Error))

	s.SystemBlock = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.Muted)).
		Italic(true).
		Padding(0, 2).
		MarginBottom(1)

	// ─────────────────────────────────────────────────────────────────────────
	// Block Header Styles
	// ─────────────────────────────────────────────────────────────────────────

	baseHeader := lipgloss.NewStyle().
		Bold(true).
		Padding(0, 1).
		MarginBottom(1)

	s.UserHeader = baseHeader.
		Foreground(lipgloss.Color(colors.UserHeader))

	s.AssistantHeader = baseHeader.
		Foreground(lipgloss.Color(colors.AssistantHeader))

	s.ToolHeader = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.ToolHeader)).
		Bold(true)

	s.ThinkingHeader = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.ThinkingHeader)).
		Italic(true)

	s.CodeHeader = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.CodeHeader)).
		Background(lipgloss.Color(colors.CodeBg)).
		Padding(0, 1).
		Bold(true)

	s.ErrorHeader = baseHeader.
		Foreground(lipgloss.Color(colors.Error))

	s.SystemHeader = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.Muted)).
		Italic(true)

	// ─────────────────────────────────────────────────────────────────────────
	// Block Content Styles
	// ─────────────────────────────────────────────────────────────────────────

	s.BlockContent = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.Foreground))

	s.ToolInput = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.Muted)).
		Italic(true).
		MarginTop(1)

	s.ToolOutput = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.Foreground)).
		MarginTop(1).
		Padding(0, 1).
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderForeground(lipgloss.Color(colors.Border))

	s.CodeContent = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.Foreground)).
		Background(lipgloss.Color(colors.CodeBg))

	s.ThinkingContent = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.Muted)).
		Italic(true)

	// ─────────────────────────────────────────────────────────────────────────
	// Block Footer Styles
	// ─────────────────────────────────────────────────────────────────────────

	s.BlockFooter = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.Muted)).
		MarginTop(1).
		Padding(0, 1).
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderForeground(lipgloss.Color(colors.Border))

	s.ActionButton = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.Muted)).
		Padding(0, 1)

	s.ActionButtonActive = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.Primary)).
		Bold(true).
		Padding(0, 1)

	// ─────────────────────────────────────────────────────────────────────────
	// Block Status Indicators
	// ─────────────────────────────────────────────────────────────────────────

	s.StatusPending = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.Muted))

	s.StatusStreaming = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.Primary)).
		Bold(true)

	s.StatusComplete = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.Success))

	s.StatusError = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.Error)).
		Bold(true)

	s.StreamingCursor = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.Primary)).
		Bold(true).
		Blink(true)

	// ─────────────────────────────────────────────────────────────────────────
	// Block Chrome Elements
	// ─────────────────────────────────────────────────────────────────────────

	s.CollapseIndicator = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.Muted)).
		MarginRight(1)

	s.Timestamp = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.Muted)).
		Italic(true)

	s.Duration = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.Success)).
		Bold(true)

	s.BlockIcon = lipgloss.NewStyle().
		MarginRight(1)

	s.FocusedBorder = lipgloss.NewStyle().
		BorderForeground(lipgloss.Color(colors.BorderFocused)).
		Bold(true)

	s.BookmarkIndicator = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.Warning))

	// ─────────────────────────────────────────────────────────────────────────
	// Nested Block Styles
	// ─────────────────────────────────────────────────────────────────────────

	s.NestedIndent = lipgloss.NewStyle().
		PaddingLeft(2)

	s.NestedConnector = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.Border))

	return s
}

// ═══════════════════════════════════════════════════════════════════════════════
// STYLE HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

// BlockIcons returns the icon for each block type.
func BlockIcons() map[BlockType]string {
	return map[BlockType]string{
		BlockTypeUser:      "👤",
		BlockTypeAssistant: "🤖",
		BlockTypeTool:      "🔧",
		BlockTypeThinking:  "💭",
		BlockTypeCode:      "💻",
		BlockTypeError:     "❌",
		BlockTypeSystem:    "ℹ️",
		BlockTypeText:      "📝",
	}
}

// BlockTypeNames returns human-readable names for block types.
func BlockTypeNames() map[BlockType]string {
	return map[BlockType]string{
		BlockTypeUser:      "USER",
		BlockTypeAssistant: "ASSISTANT",
		BlockTypeTool:      "TOOL",
		BlockTypeThinking:  "THINKING",
		BlockTypeCode:      "CODE",
		BlockTypeError:     "ERROR",
		BlockTypeSystem:    "SYSTEM",
		BlockTypeText:      "TEXT",
	}
}

// BlockStateIcons returns the icon for each block state.
func BlockStateIcons() map[BlockState]string {
	return map[BlockState]string{
		BlockStatePending:   "○",
		BlockStateStreaming: "◐",
		BlockStateComplete:  "●",
		BlockStateCollapsed: "▼",
		BlockStateError:     "✗",
	}
}

// CollapseIcon returns the collapse/expand indicator.
func CollapseIcon(collapsed bool) string {
	if collapsed {
		return "▶"
	}
	return "▼"
}

// ActionShortcuts returns the keyboard shortcuts for block actions.
func ActionShortcuts() map[string]string {
	return map[string]string{
		"copy":       "c",
		"toggle":     "t",
		"regenerate": "r",
		"bookmark":   "b",
		"edit":       "e",
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// GLOBAL DEFAULT STYLES
// ═══════════════════════════════════════════════════════════════════════════════

var defaultBlockStyles *BlockStyles

// DefaultBlockStyles returns the default BlockStyles instance.
func DefaultBlockStyles() BlockStyles {
	if defaultBlockStyles == nil {
		s := NewBlockStyles(DefaultBlockColors())
		defaultBlockStyles = &s
	}
	return *defaultBlockStyles
}

// InitDefaultBlockStyles initializes the global default block styles with custom colors.
func InitDefaultBlockStyles(colors BlockThemeColors) BlockStyles {
	s := NewBlockStyles(colors)
	defaultBlockStyles = &s
	return s
}
