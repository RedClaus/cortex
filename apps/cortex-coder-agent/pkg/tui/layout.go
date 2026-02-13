package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// PanelType represents the type of panel
type PanelType int

const (
	// PanelFileBrowser is the file browser panel
	PanelFileBrowser PanelType = iota
	// PanelEditor is the editor panel
	PanelEditor
	// PanelChat is the chat panel
	PanelChat
)

// LayoutConfig holds layout configuration
type LayoutConfig struct {
	Width           int
	Height          int
	ShowStatusBar   bool
	ShowHelp        bool
	StatusBarHeight int
	HelpHeight      int
}

// DefaultLayoutConfig returns the default layout configuration
func DefaultLayoutConfig() LayoutConfig {
	return LayoutConfig{
		ShowStatusBar:   true,
		ShowHelp:        true,
		StatusBarHeight: 1,
		HelpHeight:      3,
	}
}

// PanelSizes holds the calculated sizes for each panel
type PanelSizes struct {
	FileBrowserWidth int
	EditorWidth      int
	ChatWidth        int
	FileBrowserHeight int
	EditorHeight     int
	ChatHeight       int
	FileBrowserX     int
	FileBrowserY     int
	EditorX          int
	EditorY          int
	ChatX            int
	ChatY            int
	StatusBarY       int
	HelpY            int
}

// LayoutManager handles the responsive layout calculations
type LayoutManager struct {
	Config     LayoutConfig
	Sizes      PanelSizes
	Focused    PanelType
}

// NewLayoutManager creates a new layout manager
func NewLayoutManager(config LayoutConfig) *LayoutManager {
	return &LayoutManager{
		Config:  config,
		Focused: PanelFileBrowser,
	}

}

// UpdateSizes recalculates panel sizes based on terminal dimensions
func (lm *LayoutManager) UpdateSizes(width, height int) {
	lm.Config.Width = width
	lm.Config.Height = height
	
	// Calculate available height for panels
	availableHeight := height
	if lm.Config.ShowStatusBar {
		availableHeight -= lm.Config.StatusBarHeight
	}
	if lm.Config.ShowHelp {
		availableHeight -= lm.Config.HelpHeight
	}
	
	// Calculate widths based on available width
	// Three-panel layout: browser (20%) | editor (40%) | chat (40%)
	browserWidth := width / 5      // 20%
	editorWidth := (width * 2) / 5 // 40%
	chatWidth := width - browserWidth - editorWidth // remaining
	
	// Minimum widths
	if browserWidth < 20 {
		browserWidth = 20
	}
	if editorWidth < 30 {
		editorWidth = 30
	}
	if chatWidth < 30 {
		chatWidth = 30
	}
	
	// Adjust if total exceeds width
	if browserWidth+editorWidth+chatWidth > width {
		// Prioritize chat and browser, shrink editor
		editorWidth = width - browserWidth - chatWidth
		if editorWidth < 20 {
			editorWidth = 20
			// If still too big, use two-panel layout
			browserWidth = width / 3
			chatWidth = width - browserWidth
			editorWidth = 0
		}
	}
	
	lm.Sizes = PanelSizes{
		FileBrowserWidth:  browserWidth,
		EditorWidth:       editorWidth,
		ChatWidth:         chatWidth,
		FileBrowserHeight: availableHeight,
		EditorHeight:      availableHeight,
		ChatHeight:        availableHeight,
		FileBrowserX:      0,
		FileBrowserY:      0,
		EditorX:           browserWidth,
		EditorY:           0,
		ChatX:             browserWidth + editorWidth,
		ChatY:             0,
		StatusBarY:        availableHeight,
		HelpY:             availableHeight + lm.Config.StatusBarHeight,
	}
}

// GetPanelBounds returns the bounds for a specific panel
func (lm *LayoutManager) GetPanelBounds(panel PanelType) (x, y, width, height int) {
	switch panel {
	case PanelFileBrowser:
		return lm.Sizes.FileBrowserX, lm.Sizes.FileBrowserY, lm.Sizes.FileBrowserWidth, lm.Sizes.FileBrowserHeight
	case PanelEditor:
		return lm.Sizes.EditorX, lm.Sizes.EditorY, lm.Sizes.EditorWidth, lm.Sizes.EditorHeight
	case PanelChat:
		return lm.Sizes.ChatX, lm.Sizes.ChatY, lm.Sizes.ChatWidth, lm.Sizes.ChatHeight
	}
	return 0, 0, 0, 0
}

// FocusNext moves focus to the next panel
func (lm *LayoutManager) FocusNext() {
	switch lm.Focused {
	case PanelFileBrowser:
		if lm.Sizes.EditorWidth > 0 {
			lm.Focused = PanelEditor
		} else {
			lm.Focused = PanelChat
		}
	case PanelEditor:
		lm.Focused = PanelChat
	case PanelChat:
		lm.Focused = PanelFileBrowser
	}
}

// FocusPrev moves focus to the previous panel
func (lm *LayoutManager) FocusPrev() {
	switch lm.Focused {
	case PanelFileBrowser:
		lm.Focused = PanelChat
	case PanelEditor:
		lm.Focused = PanelFileBrowser
	case PanelChat:
		if lm.Sizes.EditorWidth > 0 {
			lm.Focused = PanelEditor
		} else {
			lm.Focused = PanelFileBrowser
		}
	}
}

// SetFocused sets the focused panel
func (lm *LayoutManager) SetFocused(panel PanelType) {
	lm.Focused = panel
}

// IsFocused returns true if the panel is focused
func (lm *LayoutManager) IsFocused(panel PanelType) bool {
	return lm.Focused == panel
}

// GetAvailableHeight returns the height available for panels
func (lm *LayoutManager) GetAvailableHeight() int {
	height := lm.Config.Height
	if lm.Config.ShowStatusBar {
		height -= lm.Config.StatusBarHeight
	}
	if lm.Config.ShowHelp {
		height -= lm.Config.HelpHeight
	}
	return height
}

// PlacePanel places content within a panel with proper styling
func (lm *LayoutManager) PlacePanel(content string, panel PanelType, styles Styles) string {
	x, y, width, height := lm.GetPanelBounds(panel)
	_ = x
	_ = y
	
	// Determine border style based on focus
	borderStyle := styles.App.PanelBorder
	if lm.IsFocused(panel) {
		borderStyle = styles.App.FocusedBorder
	}
	
	// Apply border and sizing
	styled := borderStyle.
		Width(width - 2).
		Height(height - 2).
		Render(content)
	
	// Position the panel (lipgloss handles positioning via strings)
	return styled
}

// RenderStatusBar renders the status bar
func (lm *LayoutManager) RenderStatusBar(content string, styles Styles) string {
	if !lm.Config.ShowStatusBar {
		return ""
	}
	
	return styles.StatusBar.Container.
		Width(lm.Config.Width).
		Height(lm.Config.StatusBarHeight).
		Render(content)
}

// RenderHelp renders the help bar
func (lm *LayoutManager) RenderHelp(content string, styles Styles) string {
	if !lm.Config.ShowHelp {
		return ""
	}
	
	return styles.Help.Container.
		Width(lm.Config.Width).
		Height(lm.Config.HelpHeight).
		Render(content)
}

// IsSmallScreen returns true if the terminal is too small for the full layout
func (lm *LayoutManager) IsSmallScreen() bool {
	return lm.Config.Width < 80 || lm.Config.Height < 20
}

// GetMinRecommendedSize returns the minimum recommended terminal size
func GetMinRecommendedSize() (width, height int) {
	return 80, 24
}

// TruncateToWidth truncates a string to fit within a given width
func TruncateToWidth(s string, maxWidth int) string {
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	
	// Simple truncation - remove characters from the end
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes)) > maxWidth-3 {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "..."
}
