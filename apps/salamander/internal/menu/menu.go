// Package menu provides a BubbleTea component for rendering menus from YAML definitions.
//
// The menu system supports nested submenus, filtering, category grouping, and
// keyboard navigation. It renders menus based on schema.MenuConfig definitions.
package menu

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/normanking/salamander/pkg/schema"
)

// ═══════════════════════════════════════════════════════════════════════════════
// MESSAGES
// ═══════════════════════════════════════════════════════════════════════════════

// MenuSelectedMsg is sent when a menu item is selected.
type MenuSelectedMsg struct {
	Item *schema.MenuItemConfig
}

// MenuClosedMsg is sent when the menu is closed.
type MenuClosedMsg struct{}

// ═══════════════════════════════════════════════════════════════════════════════
// KEY BINDINGS
// ═══════════════════════════════════════════════════════════════════════════════

type keyMap struct {
	Up        key.Binding
	Down      key.Binding
	Select    key.Binding
	Close     key.Binding
	Back      key.Binding
	Backspace key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Close: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "close"),
		),
		Back: key.NewBinding(
			key.WithKeys("backspace"),
			key.WithHelp("backspace", "back"),
		),
		Backspace: key.NewBinding(
			key.WithKeys("backspace"),
			key.WithHelp("backspace", "delete char"),
		),
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// STYLES
// ═══════════════════════════════════════════════════════════════════════════════

// MenuStyles defines the visual styling for the menu.
type MenuStyles struct {
	// Container is the outer border/frame
	Container lipgloss.Style
	// Title styles the menu title
	Title lipgloss.Style
	// Filter styles the filter input
	Filter lipgloss.Style
	// FilterCursor styles the filter cursor
	FilterCursor lipgloss.Style
	// Item styles a normal menu item
	Item lipgloss.Style
	// SelectedItem styles the currently selected item
	SelectedItem lipgloss.Style
	// Description styles item descriptions
	Description lipgloss.Style
	// SelectedDescription styles description of selected item
	SelectedDescription lipgloss.Style
	// Category styles category headers
	Category lipgloss.Style
	// Icon styles the item icon
	Icon lipgloss.Style
	// Shortcut styles the shortcut key
	Shortcut lipgloss.Style
	// Cursor is the selection indicator
	Cursor string
	// NoCursor is shown for non-selected items
	NoCursor string
}

// DefaultStyles returns the default menu styles.
func DefaultStyles() MenuStyles {
	subtle := lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"}
	highlight := lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#AD8CFF"}
	text := lipgloss.AdaptiveColor{Light: "#1A1A1A", Dark: "#FAFAFA"}
	muted := lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"}
	border := lipgloss.AdaptiveColor{Light: "#DDDDDD", Dark: "#444444"}

	return MenuStyles{
		Container: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(border).
			Padding(0, 1),
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(highlight).
			MarginBottom(1),
		Filter: lipgloss.NewStyle().
			Foreground(text).
			MarginBottom(1),
		FilterCursor: lipgloss.NewStyle().
			Foreground(highlight),
		Item: lipgloss.NewStyle().
			Foreground(text).
			PaddingLeft(2),
		SelectedItem: lipgloss.NewStyle().
			Foreground(highlight).
			Bold(true).
			PaddingLeft(0),
		Description: lipgloss.NewStyle().
			Foreground(subtle).
			MarginLeft(1),
		SelectedDescription: lipgloss.NewStyle().
			Foreground(muted).
			MarginLeft(1),
		Category: lipgloss.NewStyle().
			Foreground(subtle).
			Bold(true).
			MarginTop(1).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(border),
		Icon: lipgloss.NewStyle().
			MarginRight(1),
		Shortcut: lipgloss.NewStyle().
			Foreground(subtle).
			Italic(true),
		Cursor:   "> ",
		NoCursor: "  ",
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// MENU ITEM (internal)
// ═══════════════════════════════════════════════════════════════════════════════

// menuItem wraps a schema.MenuItemConfig with display state.
type menuItem struct {
	config   *schema.MenuItemConfig
	category string // for grouping
}

// ═══════════════════════════════════════════════════════════════════════════════
// MENU MODEL
// ═══════════════════════════════════════════════════════════════════════════════

// Menu is a BubbleTea model for rendering menus from YAML definitions.
type Menu struct {
	// Configuration
	config *schema.MenuConfig

	// State
	items     []menuItem // filtered/visible items
	allItems  []menuItem // all items from config
	cursor    int        // current selection index
	filter    string     // current filter text
	visible   bool       // whether menu is shown
	scrollTop int        // scroll offset for items

	// Dimensions
	width  int
	height int

	// Navigation
	parent *Menu // parent menu for submenu navigation

	// Styling
	styles MenuStyles
	keys   keyMap
}

// New creates a new Menu from a schema.MenuConfig.
func New(cfg *schema.MenuConfig) *Menu {
	if cfg == nil {
		cfg = &schema.MenuConfig{}
	}

	m := &Menu{
		config:  cfg,
		styles:  DefaultStyles(),
		keys:    defaultKeyMap(),
		visible: false,
		width:   40,
		height:  20,
	}

	// Build items from config
	m.allItems = make([]menuItem, 0, len(cfg.Items))
	for i := range cfg.Items {
		m.allItems = append(m.allItems, menuItem{
			config:   &cfg.Items[i],
			category: cfg.Items[i].Category,
		})
	}
	m.items = m.allItems

	return m
}

// ═══════════════════════════════════════════════════════════════════════════════
// TEA.MODEL INTERFACE
// ═══════════════════════════════════════════════════════════════════════════════

// Init implements tea.Model.
func (m *Menu) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m *Menu) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}

	return m, nil
}

// handleKeyPress processes keyboard input.
func (m *Menu) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle navigation keys
	switch {
	case key.Matches(msg, m.keys.Up):
		m.moveCursor(-1)
		return m, nil

	case key.Matches(msg, m.keys.Down):
		m.moveCursor(1)
		return m, nil

	case key.Matches(msg, m.keys.Select):
		return m.selectItem()

	case key.Matches(msg, m.keys.Close):
		// If we have a parent, go back to it
		if m.parent != nil {
			return m.CloseSubmenu(), nil
		}
		// Otherwise close the menu
		m.Hide()
		return m, func() tea.Msg { return MenuClosedMsg{} }

	case key.Matches(msg, m.keys.Back):
		// Backspace: if we have filter text, delete a char
		if m.config.Filterable && len(m.filter) > 0 {
			m.SetFilter(m.filter[:len(m.filter)-1])
			return m, nil
		}
		// If no filter and we have a parent, go back
		if m.parent != nil {
			return m.CloseSubmenu(), nil
		}
		return m, nil
	}

	// Handle typing for filterable menus
	if m.config.Filterable {
		switch msg.Type {
		case tea.KeyRunes:
			m.SetFilter(m.filter + string(msg.Runes))
			return m, nil
		}
	}

	return m, nil
}

// moveCursor moves the selection cursor up or down.
func (m *Menu) moveCursor(delta int) {
	if len(m.items) == 0 {
		return
	}

	m.cursor += delta

	// Wrap around
	if m.cursor < 0 {
		m.cursor = len(m.items) - 1
	} else if m.cursor >= len(m.items) {
		m.cursor = 0
	}

	// Adjust scroll
	m.adjustScroll()
}

// adjustScroll ensures the cursor is visible within the scroll window.
func (m *Menu) adjustScroll() {
	maxVisible := m.maxVisible()
	if maxVisible <= 0 {
		return
	}

	// Scroll down if cursor is below visible area
	if m.cursor >= m.scrollTop+maxVisible {
		m.scrollTop = m.cursor - maxVisible + 1
	}
	// Scroll up if cursor is above visible area
	if m.cursor < m.scrollTop {
		m.scrollTop = m.cursor
	}
}

// maxVisible returns the maximum number of visible items.
func (m *Menu) maxVisible() int {
	if m.config.MaxVisible > 0 {
		return m.config.MaxVisible
	}
	// Default to height minus chrome (title, filter, borders)
	return m.height - 6
}

// selectItem handles selection of the current item.
func (m *Menu) selectItem() (tea.Model, tea.Cmd) {
	if len(m.items) == 0 || m.cursor >= len(m.items) {
		return m, nil
	}

	selected := m.items[m.cursor].config

	// If item has a submenu, open it
	if selected.Submenu != nil {
		return m.OpenSubmenu(selected), nil
	}

	// Otherwise emit selection message
	return m, func() tea.Msg {
		return MenuSelectedMsg{Item: selected}
	}
}

// View implements tea.Model.
func (m *Menu) View() string {
	if !m.visible {
		return ""
	}

	var b strings.Builder

	// Title
	if m.config.Title != "" {
		b.WriteString(m.styles.Title.Render(m.config.Title))
		b.WriteString("\n")
	}

	// Filter input
	if m.config.Filterable {
		filterDisplay := m.filter
		if filterDisplay == "" {
			filterDisplay = "Type to filter..."
		}
		b.WriteString(m.styles.Filter.Render("> " + filterDisplay))
		b.WriteString(m.styles.FilterCursor.Render("█"))
		b.WriteString("\n")
	}

	// Items
	if len(m.items) == 0 {
		b.WriteString(m.styles.Description.Render("No matching items"))
	} else {
		b.WriteString(m.renderItems())
	}

	return m.styles.Container.Render(b.String())
}

// renderItems renders the visible menu items.
func (m *Menu) renderItems() string {
	var b strings.Builder

	maxVisible := m.maxVisible()
	start := m.scrollTop
	end := start + maxVisible
	if end > len(m.items) {
		end = len(m.items)
	}

	lastCategory := ""
	for i := start; i < end; i++ {
		item := m.items[i]
		isSelected := i == m.cursor

		// Category header
		if item.category != "" && item.category != lastCategory {
			if i > start {
				b.WriteString("\n")
			}
			b.WriteString(m.styles.Category.Render("─── " + item.category + " "))
			b.WriteString("\n")
			lastCategory = item.category
		}

		// Render item
		b.WriteString(m.renderItem(item, isSelected))
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	// Scroll indicator
	if len(m.items) > maxVisible {
		if end < len(m.items) {
			b.WriteString("\n")
			b.WriteString(m.styles.Description.Render("  ↓ more..."))
		}
		if start > 0 {
			// Prepend scroll up indicator
		}
	}

	return b.String()
}

// renderItem renders a single menu item.
func (m *Menu) renderItem(item menuItem, selected bool) string {
	var b strings.Builder

	// Cursor
	if selected {
		b.WriteString(m.styles.Cursor)
	} else {
		b.WriteString(m.styles.NoCursor)
	}

	// Icon
	if item.config.Icon != "" {
		b.WriteString(m.styles.Icon.Render(item.config.Icon))
	}

	// Label
	label := item.config.Label
	if selected {
		label = m.styles.SelectedItem.Render(label)
	} else {
		label = m.styles.Item.Render(label)
	}
	b.WriteString(label)

	// Description
	if item.config.Description != "" {
		desc := item.config.Description
		if selected {
			desc = m.styles.SelectedDescription.Render(desc)
		} else {
			desc = m.styles.Description.Render(desc)
		}
		b.WriteString(desc)
	}

	// Shortcut
	if item.config.Shortcut != "" {
		b.WriteString(" ")
		b.WriteString(m.styles.Shortcut.Render("[" + item.config.Shortcut + "]"))
	}

	// Submenu indicator
	if item.config.Submenu != nil {
		b.WriteString(" →")
	}

	return b.String()
}

// ═══════════════════════════════════════════════════════════════════════════════
// PUBLIC METHODS
// ═══════════════════════════════════════════════════════════════════════════════

// Show makes the menu visible.
func (m *Menu) Show() {
	m.visible = true
	m.cursor = 0
	m.scrollTop = 0
}

// Hide makes the menu invisible.
func (m *Menu) Hide() {
	m.visible = false
	m.filter = ""
	m.items = m.allItems
}

// IsVisible returns whether the menu is currently visible.
func (m *Menu) IsVisible() bool {
	return m.visible
}

// SetFilter filters the menu items by the given string.
func (m *Menu) SetFilter(filter string) {
	m.filter = filter
	m.filterItems()
}

// filterItems applies the current filter to the items.
func (m *Menu) filterItems() {
	if m.filter == "" {
		m.items = m.allItems
		m.cursor = 0
		m.scrollTop = 0
		return
	}

	filter := strings.ToLower(m.filter)
	m.items = make([]menuItem, 0)

	for _, item := range m.allItems {
		// Fuzzy match on label
		if fuzzyMatch(strings.ToLower(item.config.Label), filter) {
			m.items = append(m.items, item)
			continue
		}
		// Also match on description
		if item.config.Description != "" && fuzzyMatch(strings.ToLower(item.config.Description), filter) {
			m.items = append(m.items, item)
		}
	}

	// Reset cursor
	m.cursor = 0
	m.scrollTop = 0
}

// fuzzyMatch performs a simple fuzzy match.
// Returns true if all characters in pattern appear in str in order.
func fuzzyMatch(str, pattern string) bool {
	if pattern == "" {
		return true
	}
	if str == "" {
		return false
	}

	patternIdx := 0
	for i := 0; i < len(str) && patternIdx < len(pattern); i++ {
		if str[i] == pattern[patternIdx] {
			patternIdx++
		}
	}
	return patternIdx == len(pattern)
}

// Selected returns the currently selected menu item, or nil if none.
func (m *Menu) Selected() *schema.MenuItemConfig {
	if len(m.items) == 0 || m.cursor >= len(m.items) {
		return nil
	}
	return m.items[m.cursor].config
}

// OpenSubmenu opens a submenu for the given item.
func (m *Menu) OpenSubmenu(item *schema.MenuItemConfig) *Menu {
	if item == nil || item.Submenu == nil {
		return m
	}

	submenu := New(item.Submenu)
	submenu.parent = m
	submenu.width = m.width
	submenu.height = m.height
	submenu.styles = m.styles
	submenu.Show()

	return submenu
}

// CloseSubmenu returns to the parent menu.
func (m *Menu) CloseSubmenu() *Menu {
	if m.parent == nil {
		return m
	}
	return m.parent
}

// Parent returns the parent menu, if any.
func (m *Menu) Parent() *Menu {
	return m.parent
}

// SetSize sets the menu dimensions.
func (m *Menu) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetStyles sets custom styles for the menu.
func (m *Menu) SetStyles(styles MenuStyles) {
	m.styles = styles
}

// Config returns the menu configuration.
func (m *Menu) Config() *schema.MenuConfig {
	return m.config
}
