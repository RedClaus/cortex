// Package ui provides message rendering functions for the Cortex TUI.
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/normanking/cortex/internal/ui/block"
)

// ═══════════════════════════════════════════════════════════════════════════════
// MESSAGE RENDERING
// ═══════════════════════════════════════════════════════════════════════════════

// renderMessages renders all messages in the conversation history.
// This function iterates through the messages and renders each one with appropriate styling.
func renderMessages(m Model) string {
	if len(m.messages) == 0 {
		return renderEmptyState(m)
	}

	var renderedMessages []string

	for _, msg := range m.messages {
		if msg != nil {
			rendered := renderMessage(*msg, m.styles, m.width)
			renderedMessages = append(renderedMessages, rendered)
		}
	}

	// Join all messages with a newline separator
	return strings.Join(renderedMessages, "\n\n")
}

// ═══════════════════════════════════════════════════════════════════════════════
// BLOCK RENDERING (CR-002)
// ═══════════════════════════════════════════════════════════════════════════════

// Virtualization threshold - use virtual rendering for lists larger than this.
const virtualizationThreshold = 50

// renderBlocks renders all blocks in the conversation using the block rendering system.
// This is the block-based alternative to renderMessages().
// For long conversations (>50 blocks), it uses virtualized rendering for performance.
func renderBlocks(m Model) string {
	if m.blockContainer == nil {
		return renderEmptyState(m)
	}

	rootBlocks := m.blockContainer.GetRootBlocks()
	if len(rootBlocks) == 0 {
		return renderEmptyState(m)
	}

	// Create block renderer with current theme colors
	colors := themeToBlockColors(m.themeName)
	styles := block.NewBlockStyles(colors)
	renderer := block.NewBlockRenderer(styles, m.width-4) // Account for viewport padding

	// Flatten blocks for rendering (include visible children)
	flatBlocks := flattenBlocksForRender(rootBlocks)

	// Use virtualized rendering for long conversations
	if block.ShouldUseVirtualization(len(flatBlocks)) {
		return renderBlocksVirtualized(flatBlocks, renderer, m)
	}

	// Standard rendering for shorter conversations
	return renderBlocksStandard(rootBlocks, renderer)
}

// renderBlocksStandard renders all blocks without virtualization.
func renderBlocksStandard(rootBlocks []*block.Block, renderer *block.BlockRenderer) string {
	var renderedBlocks []string
	for _, b := range rootBlocks {
		rendered := renderer.RenderBlockWithChildren(b, 0)
		if rendered != "" {
			renderedBlocks = append(renderedBlocks, rendered)
		}
	}
	return strings.Join(renderedBlocks, "\n")
}

// renderBlocksVirtualized renders only visible blocks for performance.
func renderBlocksVirtualized(flatBlocks []*block.Block, renderer *block.BlockRenderer, m Model) string {
	// Create virtual renderer with viewport height
	viewportHeight := m.viewport.Height
	if viewportHeight <= 0 {
		viewportHeight = 30 // Default fallback
	}

	virtualRenderer := block.NewVirtualListRenderer(renderer, viewportHeight)
	virtualRenderer.SetWidth(m.width - 4)

	// Get scroll offset from viewport
	scrollOffset := m.viewport.YOffset

	// Render only visible blocks
	result := virtualRenderer.RenderVirtual(flatBlocks, scrollOffset)

	return result.Content
}

// flattenBlocksForRender creates a flat list of visible blocks for rendering.
// This recursively includes non-collapsed children.
func flattenBlocksForRender(blocks []*block.Block) []*block.Block {
	var flat []*block.Block
	for _, b := range blocks {
		flat = append(flat, b)
		// Include children if not collapsed
		if !b.Collapsed && len(b.Children) > 0 {
			flat = append(flat, flattenBlocksForRender(b.Children)...)
		}
	}
	return flat
}

// renderConversation renders the conversation using either blocks or messages.
// This is the unified entry point for conversation rendering.
func renderConversation(m Model) string {
	if m.useBlockSystem {
		return renderBlocks(m)
	}
	return renderMessages(m)
}

// themeToBlockColors converts a TUI theme to block theme colors.
func themeToBlockColors(themeName string) block.BlockThemeColors {
	theme := GetTheme(themeName)

	return block.BlockThemeColors{
		// Base colors from theme
		Background:    theme.Background,
		Foreground:    theme.Foreground,
		Muted:         theme.Muted,
		Border:        theme.Border,
		BorderFocused: theme.Primary,

		// Semantic colors from theme
		Primary:   theme.Primary,
		Secondary: theme.Secondary,
		Success:   theme.Success,
		Warning:   theme.Warning,
		Error:     theme.Error,

		// Block backgrounds (derived from theme)
		UserBg:      darkenColor(theme.Primary, 0.8),
		AssistantBg: theme.Background,
		ToolBg:      darkenColor(theme.Success, 0.85),
		ThinkingBg:  darkenColor(theme.Muted, 0.9),
		CodeBg:      darkenColor(theme.Background, 0.8),
		ErrorBg:     darkenColor(theme.Error, 0.85),
		SystemBg:    theme.Background,

		// Header colors
		UserHeader:      theme.Primary,
		AssistantHeader: theme.Secondary,
		ToolHeader:      theme.Success,
		ThinkingHeader:  theme.Muted,
		CodeHeader:      theme.Warning,
	}
}

// darkenColor creates a darker version of a color for backgrounds.
// factor should be 0.0 (black) to 1.0 (original color).
func darkenColor(hexColor string, factor float64) string {
	// Simple implementation - just return a hardcoded darker shade
	// A proper implementation would parse hex and calculate
	if factor < 0.5 {
		return "#0a0a0a"
	} else if factor < 0.7 {
		return "#121212"
	} else if factor < 0.85 {
		return "#1a1a1a"
	}
	return "#1e1e1e"
}

// renderEmptyState renders a helpful message when the conversation is empty.
func renderEmptyState(m Model) string {
	theme := GetTheme(m.themeName)
	emptyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Muted)).
		Italic(true).
		Align(lipgloss.Center).
		Width(m.width).
		Padding(2, 0)

	return emptyStyle.Render(
		"Welcome to Cortex!\n\n" +
			"Type a message below to start a conversation.\n" +
			"Press Ctrl+H for help.",
	)
}

// ═══════════════════════════════════════════════════════════════════════════════
// SINGLE MESSAGE RENDERING
// ═══════════════════════════════════════════════════════════════════════════════

// renderMessage renders a single message with role-appropriate styling.
// Note: This works with the Message type from message.go (pointer-based)
func renderMessage(msg Message, styles Styles, width int) string {
	// Render based on message role
	var rendered string

	// Handle error messages
	if msg.State == MessageError {
		return renderErrorMessage(msg, styles, width)
	}

	switch msg.Role {
	case RoleUser:
		rendered = renderUserMessage(msg, styles, width)
	case RoleAssistant:
		rendered = renderAssistantMessage(msg, styles, width)
	case RoleSystem:
		rendered = renderSystemMessage(msg, styles, width)
	default:
		rendered = renderUnknownMessage(msg, styles, width)
	}

	return rendered
}

// ═══════════════════════════════════════════════════════════════════════════════
// ROLE-SPECIFIC MESSAGE RENDERING
// ═══════════════════════════════════════════════════════════════════════════════

// renderUserMessage renders a user message with user styling.
func renderUserMessage(msg Message, styles Styles, width int) string {
	// Role indicator
	roleIndicator := styles.UserMessage.Copy().Bold(true).Render("👤 You:")

	// Message content (plain text for user messages)
	content := styles.UserMessage.Copy().
		Width(width - 4).
		Padding(0, 2).
		Render(msg.RawContent)

	// Timestamp
	theme := GetTheme("")
	timestamp := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Muted)).
		Italic(true).
		Render(fmt.Sprintf("[%s]", msg.Timestamp.Format("15:04:05")))

	// Combine: role indicator, content, timestamp
	return lipgloss.JoinVertical(
		lipgloss.Left,
		roleIndicator,
		content,
		timestamp,
	)
}

// renderAssistantMessage renders an assistant message with markdown rendering.
func renderAssistantMessage(msg Message, styles Styles, width int) string {
	// Role indicator with model name if available
	roleText := "🤖 Assistant:"
	if modelName, ok := msg.Metadata["model"].(string); ok && modelName != "" {
		roleText = fmt.Sprintf("🤖 Assistant (%s):", modelName)
	}
	roleIndicator := styles.AssistantMessage.Copy().Bold(true).Render(roleText)

	// Render markdown content (check if streaming first)
	var content string
	theme := GetTheme("")
	if msg.IsStreaming() {
		// During streaming, show raw content with cursor
		cursor := lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.Primary)).
			Render("▌")
		content = styles.AssistantMessage.Copy().
			Width(width - 4).
			Padding(0, 2).
			Render(msg.RawContent + cursor)
	} else {
		// Completed message - render markdown
		content = renderMarkdown(msg.RawContent, styles, width-4)
	}

	// Build the complete message
	parts := []string{roleIndicator, content}

	// Add timestamp
	timestamp := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Muted)).
		Italic(true).
		Render(fmt.Sprintf("[%s]", msg.Timestamp.Format("15:04:05")))
	parts = append(parts, timestamp)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// renderSystemMessage renders a system message with muted styling.
func renderSystemMessage(msg Message, styles Styles, width int) string {
	// Format: icon + content
	content := fmt.Sprintf("ℹ️ %s", msg.RawContent)

	return styles.SystemMessage.Copy().
		Width(width - 4).
		Padding(0, 2).
		Render(content)
}

// renderErrorMessage renders an error message.
func renderErrorMessage(msg Message, styles Styles, width int) string {
	// Format: error icon + content
	errText := msg.RawContent
	if msg.Error != nil {
		errText = msg.Error.Error()
	}
	content := fmt.Sprintf("❌ %s", errText)

	return styles.ErrorBox.Copy().
		Width(width - 4).
		Padding(0, 2).
		Render(content)
}

// renderUnknownMessage renders a message with unknown role (fallback).
func renderUnknownMessage(msg Message, styles Styles, width int) string {
	theme := GetTheme("")
	unknownStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Warning)).
		Width(width - 4).
		Padding(0, 2)

	content := fmt.Sprintf("⚠️ Unknown message type: %s", msg.RawContent)

	return unknownStyle.Render(content)
}

// ═══════════════════════════════════════════════════════════════════════════════
// MARKDOWN RENDERING
// ═══════════════════════════════════════════════════════════════════════════════

// renderMarkdown renders markdown content using Glamour.
// This wraps glamour.Render() with error handling and theme integration.
func renderMarkdown(content string, styles Styles, width int) string {
	// If content is empty, return empty string
	if strings.TrimSpace(content) == "" {
		return ""
	}

	// Create glamour renderer with auto style
	// TODO: Use theme-specific glamour style when available
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		// Fallback to plain text if glamour fails to initialize
		return renderPlainText(content, styles, width)
	}

	// Render the markdown
	rendered, err := renderer.Render(content)
	if err != nil {
		// Fallback to plain text if rendering fails
		return renderPlainText(content, styles, width)
	}

	// Trim trailing whitespace
	rendered = strings.TrimRight(rendered, "\n")

	// Apply padding
	paddedStyle := lipgloss.NewStyle().
		Padding(0, 2)

	return paddedStyle.Render(rendered)
}

// renderPlainText renders plain text as a fallback when markdown rendering fails.
func renderPlainText(content string, styles Styles, width int) string {
	theme := GetTheme("")
	plainStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Foreground)).
		Width(width).
		Padding(0, 2)

	return plainStyle.Render(content)
}

// ═══════════════════════════════════════════════════════════════════════════════
// MESSAGE METADATA RENDERING
// ═══════════════════════════════════════════════════════════════════════════════

// renderMessageMetadata renders metadata like model info, etc.
// This is used for debugging or advanced display modes.
func renderMessageMetadata(msg Message, styles Styles) string {
	if msg.Metadata == nil || len(msg.Metadata) == 0 {
		return ""
	}

	theme := GetTheme("")
	metaStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Muted)).
		Italic(true)

	var metaParts []string

	// Display common metadata fields
	if model, ok := msg.Metadata["model"].(string); ok {
		metaParts = append(metaParts, fmt.Sprintf("Model: %s", model))
	}

	if tokens, ok := msg.Metadata["tokens"].(int); ok {
		metaParts = append(metaParts, fmt.Sprintf("Tokens: %d", tokens))
	}

	if len(metaParts) == 0 {
		return ""
	}

	metaText := strings.Join(metaParts, " • ")
	return metaStyle.Render(fmt.Sprintf("  [%s]", metaText))
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPER FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════════

// truncateContent truncates message content to a maximum length for preview.
func truncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen-3] + "..."
}

// wrapText wraps text to a specified width.
// This is a simple word-wrap implementation for cases where lipgloss wrapping isn't sufficient.
func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	var wrapped []string
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		if len(line) <= width {
			wrapped = append(wrapped, line)
			continue
		}

		// Simple word wrapping
		words := strings.Fields(line)
		var currentLine string

		for _, word := range words {
			if len(currentLine)+len(word)+1 > width {
				if currentLine != "" {
					wrapped = append(wrapped, currentLine)
					currentLine = word
				} else {
					// Word itself is longer than width - just add it
					wrapped = append(wrapped, word)
				}
			} else {
				if currentLine == "" {
					currentLine = word
				} else {
					currentLine += " " + word
				}
			}
		}

		if currentLine != "" {
			wrapped = append(wrapped, currentLine)
		}
	}

	return strings.Join(wrapped, "\n")
}
