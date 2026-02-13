// Package block provides block rendering functions for the Cortex TUI.
package block

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// ═══════════════════════════════════════════════════════════════════════════════
// BLOCK RENDERER
// ═══════════════════════════════════════════════════════════════════════════════

// BlockRenderer handles rendering of blocks with a specific style set.
type BlockRenderer struct {
	styles     BlockStyles
	width      int
	showFooter bool
}

// NewBlockRenderer creates a new BlockRenderer with the given styles and width.
func NewBlockRenderer(styles BlockStyles, width int) *BlockRenderer {
	return &BlockRenderer{
		styles:     styles,
		width:      width,
		showFooter: true,
	}
}

// SetWidth updates the available width for rendering.
func (r *BlockRenderer) SetWidth(width int) {
	r.width = width
}

// SetShowFooter controls whether action footers are shown.
func (r *BlockRenderer) SetShowFooter(show bool) {
	r.showFooter = show
}

// ═══════════════════════════════════════════════════════════════════════════════
// MAIN RENDER FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════════

// RenderBlock renders a single block with its chrome (header, borders, actions).
func (r *BlockRenderer) RenderBlock(b *Block) string {
	if b == nil {
		return ""
	}

	// Check cache first
	b.mu.RLock()
	if !b.NeedsRender && b.CachedRender != "" {
		cached := b.CachedRender
		b.mu.RUnlock()
		return cached
	}
	b.mu.RUnlock()

	var result string

	switch b.Type {
	case BlockTypeUser:
		result = r.renderUserBlock(b)
	case BlockTypeAssistant:
		result = r.renderAssistantBlock(b)
	case BlockTypeTool:
		result = r.renderToolBlock(b)
	case BlockTypeThinking:
		result = r.renderThinkingBlock(b)
	case BlockTypeCode:
		result = r.renderCodeBlock(b)
	case BlockTypeError:
		result = r.renderErrorBlock(b)
	case BlockTypeSystem:
		result = r.renderSystemBlock(b)
	case BlockTypeText:
		result = r.renderTextBlock(b)
	default:
		result = r.renderGenericBlock(b)
	}

	// Update cache if block is complete (not streaming)
	if b.State == BlockStateComplete || b.State == BlockStateCollapsed {
		b.mu.Lock()
		b.CachedRender = result
		b.NeedsRender = false
		b.mu.Unlock()
	}

	return result
}

// RenderBlockList renders a list of blocks with proper spacing.
func (r *BlockRenderer) RenderBlockList(blocks []*Block) string {
	if len(blocks) == 0 {
		return ""
	}

	var parts []string
	for _, b := range blocks {
		rendered := r.RenderBlock(b)
		if rendered != "" {
			parts = append(parts, rendered)
		}
	}

	return strings.Join(parts, "\n")
}

// RenderBlockWithChildren renders a block and its nested children.
func (r *BlockRenderer) RenderBlockWithChildren(b *Block, depth int) string {
	if b == nil {
		return ""
	}

	// Render the block itself
	result := r.RenderBlock(b)

	// If collapsed, don't render children
	if b.Collapsed || b.State == BlockStateCollapsed {
		return result
	}

	// Render children with indentation
	children := b.GetChildren()
	if len(children) > 0 {
		var childParts []string
		for _, child := range children {
			childRendered := r.RenderBlockWithChildren(child, depth+1)
			if childRendered != "" {
				// Add indentation for nesting
				indented := r.indentBlock(childRendered, depth+1)
				childParts = append(childParts, indented)
			}
		}
		if len(childParts) > 0 {
			result += "\n" + strings.Join(childParts, "\n")
		}
	}

	return result
}

// ═══════════════════════════════════════════════════════════════════════════════
// INDIVIDUAL BLOCK RENDERERS
// ═══════════════════════════════════════════════════════════════════════════════

// renderUserBlock renders a user message block.
func (r *BlockRenderer) renderUserBlock(b *Block) string {
	icons := BlockIcons()
	header := r.renderHeader(icons[BlockTypeUser], "YOU", b)
	content := r.styles.BlockContent.
		Width(r.contentWidth()).
		Render(b.Content)

	inner := header + "\n" + content

	if r.showFooter && b.State == BlockStateComplete {
		inner += "\n" + r.renderFooter(b)
	}

	return r.styles.UserBlock.
		Width(r.width).
		Render(inner)
}

// renderAssistantBlock renders an assistant response block.
func (r *BlockRenderer) renderAssistantBlock(b *Block) string {
	icons := BlockIcons()

	// Build header with model info if available
	headerLabel := "ASSISTANT"
	if model, ok := b.Metadata["model"].(string); ok && model != "" {
		headerLabel = fmt.Sprintf("ASSISTANT [%s]", model)
	}

	header := r.renderHeader(icons[BlockTypeAssistant], headerLabel, b)

	// Render content with streaming cursor if active
	content := b.Content
	if b.State == BlockStateStreaming {
		content += r.styles.StreamingCursor.Render("▌")
	}

	contentRendered := r.styles.BlockContent.
		Width(r.contentWidth()).
		Render(content)

	inner := header + "\n" + contentRendered

	// Add footer if complete and show footer is enabled
	if r.showFooter && b.State == BlockStateComplete {
		inner += "\n" + r.renderFooter(b)
	}

	return r.styles.AssistantBlock.
		Width(r.width).
		Render(inner)
}

// renderToolBlock renders a tool execution block.
func (r *BlockRenderer) renderToolBlock(b *Block) string {
	icons := BlockIcons()

	// Build header with tool name and status
	statusIcon := r.getStatusIcon(b)
	toolName := b.ToolName
	if toolName == "" {
		toolName = "unknown"
	}

	var durationStr string
	if b.ToolDuration > 0 {
		durationStr = r.styles.Duration.Render(fmt.Sprintf(" %.2fs", b.ToolDuration.Seconds()))
	}

	collapseIcon := CollapseIcon(b.Collapsed)

	header := lipgloss.JoinHorizontal(
		lipgloss.Top,
		r.styles.CollapseIndicator.Render(collapseIcon),
		r.styles.ToolHeader.Render(fmt.Sprintf("%s TOOL: %s", icons[BlockTypeTool], toolName)),
		r.styles.StatusComplete.Render(fmt.Sprintf(" [%s%s]", statusIcon, durationStr)),
	)

	// If collapsed, only show header
	if b.Collapsed {
		return r.styles.ToolBlock.Width(r.width).Render(header)
	}

	var parts []string
	parts = append(parts, header)

	// Render input
	if b.ToolInput != "" {
		inputLabel := r.styles.ToolInput.Render("Input: " + truncateString(b.ToolInput, 100))
		parts = append(parts, inputLabel)
	}

	// Render output
	if b.ToolOutput != "" {
		outputLines := strings.Split(b.ToolOutput, "\n")
		if len(outputLines) > 10 {
			// Truncate long output
			outputLines = append(outputLines[:10], fmt.Sprintf("... (%d more lines)", len(outputLines)-10))
		}
		output := r.styles.ToolOutput.
			Width(r.contentWidth()).
			Render(strings.Join(outputLines, "\n"))
		parts = append(parts, output)
	}

	return r.styles.ToolBlock.
		Width(r.width).
		Render(strings.Join(parts, "\n"))
}

// renderThinkingBlock renders an AI thinking/reasoning block.
func (r *BlockRenderer) renderThinkingBlock(b *Block) string {
	icons := BlockIcons()
	collapseIcon := CollapseIcon(b.Collapsed)

	header := lipgloss.JoinHorizontal(
		lipgloss.Top,
		r.styles.CollapseIndicator.Render(collapseIcon),
		r.styles.ThinkingHeader.Render(fmt.Sprintf("%s THINKING", icons[BlockTypeThinking])),
	)

	// If collapsed, only show header
	if b.Collapsed {
		return r.styles.ThinkingBlock.Width(r.width).Render(header)
	}

	content := r.styles.ThinkingContent.
		Width(r.contentWidth()).
		Render(b.Content)

	// Add streaming cursor if active
	if b.State == BlockStateStreaming {
		content += r.styles.StreamingCursor.Render("▌")
	}

	return r.styles.ThinkingBlock.
		Width(r.width).
		Render(header + "\n" + content)
}

// renderCodeBlock renders a code block with syntax highlighting indicator.
func (r *BlockRenderer) renderCodeBlock(b *Block) string {
	// Language header
	lang := b.Language
	if lang == "" {
		lang = "plaintext"
	}

	header := r.styles.CodeHeader.Render(lang)

	// Code content
	content := r.styles.CodeContent.
		Width(r.contentWidth()).
		Render(b.Content)

	return r.styles.CodeBlock.
		Width(r.width).
		Render(header + "\n" + content)
}

// renderErrorBlock renders an error block.
func (r *BlockRenderer) renderErrorBlock(b *Block) string {
	icons := BlockIcons()
	header := r.styles.ErrorHeader.Render(fmt.Sprintf("%s ERROR", icons[BlockTypeError]))

	errMsg := b.Content
	if b.Error != nil {
		errMsg = b.Error.Error()
	}

	content := r.styles.BlockContent.
		Width(r.contentWidth()).
		Foreground(lipgloss.Color("#fc8181")).
		Render(errMsg)

	return r.styles.ErrorBlock.
		Width(r.width).
		Render(header + "\n" + content)
}

// renderSystemBlock renders a system message block.
func (r *BlockRenderer) renderSystemBlock(b *Block) string {
	icons := BlockIcons()
	return r.styles.SystemBlock.
		Width(r.width).
		Render(fmt.Sprintf("%s %s", icons[BlockTypeSystem], b.Content))
}

// renderTextBlock renders a plain text block (usually child of assistant).
func (r *BlockRenderer) renderTextBlock(b *Block) string {
	content := b.Content
	if b.State == BlockStateStreaming {
		content += r.styles.StreamingCursor.Render("▌")
	}

	return r.styles.BlockContent.
		Width(r.contentWidth()).
		Render(content)
}

// renderGenericBlock renders a generic fallback block.
func (r *BlockRenderer) renderGenericBlock(b *Block) string {
	return r.styles.BlockContent.
		Width(r.width).
		Render(b.Content)
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPER RENDERING FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════════

// renderHeader renders a block header with icon, label, timestamp, and status.
func (r *BlockRenderer) renderHeader(icon, label string, b *Block) string {
	iconPart := r.styles.BlockIcon.Render(icon)
	labelPart := r.styles.AssistantHeader.Render(label)

	// Timestamp on the right
	timestamp := b.Timestamp.Format("15:04")
	timestampPart := r.styles.Timestamp.Render(timestamp)

	// Status indicator
	statusPart := ""
	if b.Bookmarked {
		statusPart += r.styles.BookmarkIndicator.Render(" ⭐")
	}

	// Build header with proper spacing
	left := lipgloss.JoinHorizontal(lipgloss.Top, iconPart, labelPart)
	right := timestampPart + statusPart

	// Calculate spacing
	availableWidth := r.contentWidth()
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	spacerWidth := availableWidth - leftWidth - rightWidth
	if spacerWidth < 1 {
		spacerWidth = 1
	}
	spacer := strings.Repeat(" ", spacerWidth)

	return left + spacer + right
}

// renderFooter renders the action bar for a block.
func (r *BlockRenderer) renderFooter(b *Block) string {
	actions := ActionShortcuts()

	var parts []string

	// Copy action
	parts = append(parts, r.styles.ActionButton.Render(fmt.Sprintf("[%s]opy", actions["copy"])))

	// Toggle (for blocks with children or collapsible content)
	if len(b.Children) > 0 || b.Type == BlockTypeTool || b.Type == BlockTypeThinking {
		parts = append(parts, r.styles.ActionButton.Render(fmt.Sprintf("[%s]oggle", actions["toggle"])))
	}

	// Regenerate (for assistant blocks)
	if b.Type == BlockTypeAssistant || b.Type == BlockTypeUser {
		parts = append(parts, r.styles.ActionButton.Render(fmt.Sprintf("[%s]egenerate", actions["regenerate"])))
	}

	// Bookmark
	bookmarkLabel := fmt.Sprintf("[%s]ookmark", actions["bookmark"])
	if b.Bookmarked {
		parts = append(parts, r.styles.ActionButtonActive.Render(bookmarkLabel+" ⭐"))
	} else {
		parts = append(parts, r.styles.ActionButton.Render(bookmarkLabel))
	}

	return r.styles.BlockFooter.
		Width(r.contentWidth()).
		Render(strings.Join(parts, "  "))
}

// getStatusIcon returns the appropriate status icon for a block.
func (r *BlockRenderer) getStatusIcon(b *Block) string {
	stateIcons := BlockStateIcons()

	switch b.State {
	case BlockStatePending:
		return r.styles.StatusPending.Render(stateIcons[BlockStatePending])
	case BlockStateStreaming:
		return r.styles.StatusStreaming.Render(stateIcons[BlockStateStreaming])
	case BlockStateComplete:
		if b.Type == BlockTypeTool {
			if b.ToolSuccess {
				return r.styles.StatusComplete.Render("✓")
			}
			return r.styles.StatusError.Render("✗")
		}
		return r.styles.StatusComplete.Render(stateIcons[BlockStateComplete])
	case BlockStateError:
		return r.styles.StatusError.Render(stateIcons[BlockStateError])
	default:
		return ""
	}
}

// indentBlock adds indentation for nested blocks.
func (r *BlockRenderer) indentBlock(content string, depth int) string {
	if depth <= 0 {
		return content
	}

	indent := strings.Repeat("  ", depth)
	connector := r.styles.NestedConnector.Render("│ ")

	lines := strings.Split(content, "\n")
	var indented []string
	for i, line := range lines {
		if i == 0 {
			// First line gets connector
			indented = append(indented, indent+connector+line)
		} else {
			indented = append(indented, indent+connector+line)
		}
	}

	return strings.Join(indented, "\n")
}

// contentWidth returns the available width for content (accounting for padding/borders).
func (r *BlockRenderer) contentWidth() int {
	// Account for padding (2 on each side) and border (1 on each side)
	return max(r.width-6, 20)
}

// ═══════════════════════════════════════════════════════════════════════════════
// UTILITY FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════════

// truncateString truncates a string to maxLen and adds ellipsis.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// formatDuration formats a duration nicely.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// max returns the larger of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ═══════════════════════════════════════════════════════════════════════════════
// GLOBAL RENDERER ACCESS
// ═══════════════════════════════════════════════════════════════════════════════

// RenderBlock is a convenience function using default styles.
func RenderBlock(b *Block, width int) string {
	renderer := NewBlockRenderer(DefaultBlockStyles(), width)
	return renderer.RenderBlock(b)
}

// RenderBlockList is a convenience function using default styles.
func RenderBlockList(blocks []*Block, width int) string {
	renderer := NewBlockRenderer(DefaultBlockStyles(), width)
	return renderer.RenderBlockList(blocks)
}

// RenderBlockTree renders a block with all its children using default styles.
func RenderBlockTree(b *Block, width int) string {
	renderer := NewBlockRenderer(DefaultBlockStyles(), width)
	return renderer.RenderBlockWithChildren(b, 0)
}
