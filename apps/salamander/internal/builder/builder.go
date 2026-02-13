// Package builder provides the YAML Builder UI for Salamander.
//
// The YAML Builder allows users to visually create and modify their TUI configuration
// without directly editing YAML files. It provides a menu-driven interface for:
//   - Adding/removing menu items
//   - Customizing themes and colors
//   - Configuring keybindings
//   - Setting up backend connections
//   - Managing extensions
package builder

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/normanking/salamander/pkg/schema"
	"gopkg.in/yaml.v3"
)

// ═══════════════════════════════════════════════════════════════════════════════
// BUILDER MODEL
// ═══════════════════════════════════════════════════════════════════════════════

// Mode represents the current builder mode
type Mode int

const (
	ModeMain Mode = iota
	ModeMenus
	ModeMenuEditor
	ModeItemEditor
	ModeTheme
	ModeKeybindings
	ModeBackend
	ModeExport
)

// Builder is the YAML Builder UI model
type Builder struct {
	config *schema.Config
	mode   Mode

	// Navigation
	cursor       int
	menuCursor   int
	itemCursor   int
	scrollOffset int

	// Editor state
	editingMenu  *schema.MenuConfig
	editingItem  *schema.MenuItemConfig
	inputs       []textinput.Model
	focusedInput int

	// Dimensions
	width  int
	height int

	// Styling
	styles BuilderStyles

	// State
	modified bool
	message  string
	filePath string
}

// BuilderStyles contains the styles for the builder UI
type BuilderStyles struct {
	Title      lipgloss.Style
	Subtitle   lipgloss.Style
	Normal     lipgloss.Style
	Selected   lipgloss.Style
	Muted      lipgloss.Style
	Success    lipgloss.Style
	Error      lipgloss.Style
	Border     lipgloss.Style
	Input      lipgloss.Style
	InputFocus lipgloss.Style
}

// DefaultBuilderStyles returns the default builder styles
func DefaultBuilderStyles() BuilderStyles {
	return BuilderStyles{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7C3AED")).
			MarginBottom(1),
		Subtitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#10B981")),
		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F8FAFC")),
		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#0F172A")).
			Background(lipgloss.Color("#7C3AED")).
			Padding(0, 1),
		Muted: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#94A3B8")),
		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#22C55E")),
		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")),
		Border: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#334155")).
			Padding(1, 2),
		Input: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#334155")).
			Padding(0, 1),
		InputFocus: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7C3AED")).
			Padding(0, 1),
	}
}

// New creates a new Builder with default config
func New() *Builder {
	return &Builder{
		config: &schema.Config{
			Version: "1.0",
			App: schema.AppConfig{
				Name:          "My TUI",
				ShowStatusBar: true,
				ShowTitleBar:  true,
			},
			Theme: schema.ThemeConfig{
				Name: "default",
				Mode: "dark",
			},
			Menus:  []schema.MenuConfig{},
			Layout: schema.LayoutConfig{Type: "chat"},
			Backend: schema.BackendConfig{
				Type:      "a2a",
				Streaming: true,
			},
		},
		styles: DefaultBuilderStyles(),
	}
}

// NewFromFile creates a Builder from an existing config file
func NewFromFile(path string) (*Builder, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var config schema.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	b := New()
	b.config = &config
	b.filePath = path
	return b, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// TEA MODEL IMPLEMENTATION
// ═══════════════════════════════════════════════════════════════════════════════

// Init implements tea.Model
func (b *Builder) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (b *Builder) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		b.width = msg.Width
		b.height = msg.Height
		return b, nil

	case tea.KeyMsg:
		return b.handleKey(msg)
	}

	// Update text inputs if in editor mode
	if b.mode == ModeMenuEditor || b.mode == ModeItemEditor {
		for i := range b.inputs {
			var cmd tea.Cmd
			b.inputs[i], cmd = b.inputs[i].Update(msg)
			if cmd != nil {
				return b, cmd
			}
		}
	}

	return b, nil
}

// handleKey handles keyboard input
func (b *Builder) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		if b.mode != ModeMain {
			b.goBack()
			return b, nil
		}
		return b, tea.Quit

	case "up", "k":
		b.moveCursor(-1)
		return b, nil

	case "down", "j":
		b.moveCursor(1)
		return b, nil

	case "enter":
		return b.handleSelect()

	case "tab":
		if b.mode == ModeMenuEditor || b.mode == ModeItemEditor {
			b.focusedInput = (b.focusedInput + 1) % len(b.inputs)
			b.updateInputFocus()
		}
		return b, nil

	case "ctrl+s":
		return b.saveConfig()

	case "n":
		if b.mode == ModeMenus {
			b.addNewMenu()
		}
		return b, nil

	case "d":
		if b.mode == ModeMenus && len(b.config.Menus) > 0 {
			b.deleteMenu(b.menuCursor)
		}
		return b, nil
	}

	return b, nil
}

// moveCursor moves the selection cursor
func (b *Builder) moveCursor(delta int) {
	switch b.mode {
	case ModeMain:
		b.cursor += delta
		maxItems := 6 // Number of main menu items
		if b.cursor < 0 {
			b.cursor = maxItems - 1
		} else if b.cursor >= maxItems {
			b.cursor = 0
		}

	case ModeMenus:
		b.menuCursor += delta
		if b.menuCursor < 0 {
			b.menuCursor = len(b.config.Menus) - 1
		} else if b.menuCursor >= len(b.config.Menus) {
			b.menuCursor = 0
		}

	case ModeMenuEditor:
		if b.editingMenu != nil {
			b.itemCursor += delta
			if b.itemCursor < 0 {
				b.itemCursor = len(b.editingMenu.Items) - 1
			} else if b.itemCursor >= len(b.editingMenu.Items) {
				b.itemCursor = 0
			}
		}
	}
}

// handleSelect handles enter key
func (b *Builder) handleSelect() (tea.Model, tea.Cmd) {
	switch b.mode {
	case ModeMain:
		switch b.cursor {
		case 0: // Menus
			b.mode = ModeMenus
		case 1: // Theme
			b.mode = ModeTheme
		case 2: // Keybindings
			b.mode = ModeKeybindings
		case 3: // Backend
			b.mode = ModeBackend
		case 4: // Export
			return b.saveConfig()
		case 5: // Exit
			return b, tea.Quit
		}

	case ModeMenus:
		if len(b.config.Menus) > 0 {
			b.editingMenu = &b.config.Menus[b.menuCursor]
			b.mode = ModeMenuEditor
			b.itemCursor = 0
		}

	case ModeMenuEditor:
		if b.editingMenu != nil && len(b.editingMenu.Items) > 0 {
			b.editingItem = &b.editingMenu.Items[b.itemCursor]
			b.mode = ModeItemEditor
			b.initItemInputs()
		}
	}

	return b, nil
}

// goBack returns to the previous mode
func (b *Builder) goBack() {
	switch b.mode {
	case ModeMenus, ModeTheme, ModeKeybindings, ModeBackend, ModeExport:
		b.mode = ModeMain
	case ModeMenuEditor:
		b.mode = ModeMenus
		b.editingMenu = nil
	case ModeItemEditor:
		b.mode = ModeMenuEditor
		b.saveItemInputs()
		b.editingItem = nil
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// MENU OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════════

// addNewMenu adds a new menu to the config
func (b *Builder) addNewMenu() {
	newMenu := schema.MenuConfig{
		ID:         fmt.Sprintf("menu_%d", len(b.config.Menus)+1),
		Trigger:    "/",
		Title:      "New Menu",
		Filterable: true,
		MaxVisible: 10,
		Items:      []schema.MenuItemConfig{},
	}
	b.config.Menus = append(b.config.Menus, newMenu)
	b.modified = true
	b.message = "Added new menu"
}

// deleteMenu removes a menu at the given index
func (b *Builder) deleteMenu(idx int) {
	if idx >= 0 && idx < len(b.config.Menus) {
		b.config.Menus = append(b.config.Menus[:idx], b.config.Menus[idx+1:]...)
		b.modified = true
		b.message = "Deleted menu"
		if b.menuCursor >= len(b.config.Menus) {
			b.menuCursor = len(b.config.Menus) - 1
		}
	}
}

// addNewItem adds a new item to the current menu
func (b *Builder) addNewItem() {
	if b.editingMenu == nil {
		return
	}
	newItem := schema.MenuItemConfig{
		ID:          fmt.Sprintf("item_%d", len(b.editingMenu.Items)+1),
		Label:       "new_item",
		Description: "New menu item",
		Category:    "General",
		Action: schema.ActionConfig{
			Type: "command",
		},
	}
	b.editingMenu.Items = append(b.editingMenu.Items, newItem)
	b.modified = true
	b.message = "Added new item"
}

// ═══════════════════════════════════════════════════════════════════════════════
// ITEM EDITOR
// ═══════════════════════════════════════════════════════════════════════════════

// initItemInputs initializes text inputs for item editing
func (b *Builder) initItemInputs() {
	if b.editingItem == nil {
		return
	}

	b.inputs = make([]textinput.Model, 5)

	// ID input
	b.inputs[0] = textinput.New()
	b.inputs[0].Placeholder = "Item ID"
	b.inputs[0].SetValue(b.editingItem.ID)
	b.inputs[0].CharLimit = 50

	// Label input
	b.inputs[1] = textinput.New()
	b.inputs[1].Placeholder = "Label"
	b.inputs[1].SetValue(b.editingItem.Label)
	b.inputs[1].CharLimit = 50

	// Description input
	b.inputs[2] = textinput.New()
	b.inputs[2].Placeholder = "Description"
	b.inputs[2].SetValue(b.editingItem.Description)
	b.inputs[2].CharLimit = 100

	// Category input
	b.inputs[3] = textinput.New()
	b.inputs[3].Placeholder = "Category"
	b.inputs[3].SetValue(b.editingItem.Category)
	b.inputs[3].CharLimit = 30

	// Icon input
	b.inputs[4] = textinput.New()
	b.inputs[4].Placeholder = "Icon (emoji)"
	b.inputs[4].SetValue(b.editingItem.Icon)
	b.inputs[4].CharLimit = 10

	b.focusedInput = 0
	b.updateInputFocus()
}

// updateInputFocus updates which input is focused
func (b *Builder) updateInputFocus() {
	for i := range b.inputs {
		if i == b.focusedInput {
			b.inputs[i].Focus()
		} else {
			b.inputs[i].Blur()
		}
	}
}

// saveItemInputs saves the input values back to the item
func (b *Builder) saveItemInputs() {
	if b.editingItem == nil || len(b.inputs) < 5 {
		return
	}

	b.editingItem.ID = b.inputs[0].Value()
	b.editingItem.Label = b.inputs[1].Value()
	b.editingItem.Description = b.inputs[2].Value()
	b.editingItem.Category = b.inputs[3].Value()
	b.editingItem.Icon = b.inputs[4].Value()
	b.modified = true
}

// ═══════════════════════════════════════════════════════════════════════════════
// SAVE/EXPORT
// ═══════════════════════════════════════════════════════════════════════════════

// saveConfig saves the configuration to file
func (b *Builder) saveConfig() (tea.Model, tea.Cmd) {
	if b.filePath == "" {
		b.filePath = "salamander.yaml"
	}

	data, err := yaml.Marshal(b.config)
	if err != nil {
		b.message = fmt.Sprintf("Error: %v", err)
		return b, nil
	}

	// Ensure directory exists
	dir := filepath.Dir(b.filePath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			b.message = fmt.Sprintf("Error creating dir: %v", err)
			return b, nil
		}
	}

	if err := os.WriteFile(b.filePath, data, 0644); err != nil {
		b.message = fmt.Sprintf("Error saving: %v", err)
		return b, nil
	}

	b.modified = false
	b.message = fmt.Sprintf("Saved to %s", b.filePath)
	return b, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// VIEW
// ═══════════════════════════════════════════════════════════════════════════════

// View implements tea.Model
func (b *Builder) View() string {
	var sb strings.Builder

	// Title
	sb.WriteString(b.styles.Title.Render("🛠️  Salamander YAML Builder"))
	sb.WriteString("\n\n")

	switch b.mode {
	case ModeMain:
		sb.WriteString(b.viewMain())
	case ModeMenus:
		sb.WriteString(b.viewMenus())
	case ModeMenuEditor:
		sb.WriteString(b.viewMenuEditor())
	case ModeItemEditor:
		sb.WriteString(b.viewItemEditor())
	case ModeTheme:
		sb.WriteString(b.viewTheme())
	case ModeKeybindings:
		sb.WriteString(b.viewKeybindings())
	case ModeBackend:
		sb.WriteString(b.viewBackend())
	}

	// Message line
	if b.message != "" {
		sb.WriteString("\n")
		if strings.HasPrefix(b.message, "Error") {
			sb.WriteString(b.styles.Error.Render(b.message))
		} else {
			sb.WriteString(b.styles.Success.Render(b.message))
		}
	}

	// Help
	sb.WriteString("\n\n")
	sb.WriteString(b.styles.Muted.Render(b.getHelp()))

	return sb.String()
}

// viewMain renders the main menu
func (b *Builder) viewMain() string {
	var sb strings.Builder

	items := []struct {
		icon  string
		label string
		desc  string
	}{
		{"📋", "Menus", fmt.Sprintf("%d menus configured", len(b.config.Menus))},
		{"🎨", "Theme", b.config.Theme.Name},
		{"⌨️", "Keybindings", fmt.Sprintf("%d bindings", len(b.config.Keybindings))},
		{"🔌", "Backend", fmt.Sprintf("%s: %s", b.config.Backend.Type, b.config.Backend.URL)},
		{"💾", "Save & Export", "Save configuration to file"},
		{"🚪", "Exit", "Close the builder"},
	}

	for i, item := range items {
		line := fmt.Sprintf("%s  %s  %s", item.icon, item.label, b.styles.Muted.Render(item.desc))
		if i == b.cursor {
			sb.WriteString(b.styles.Selected.Render(line))
		} else {
			sb.WriteString(b.styles.Normal.Render(line))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// viewMenus renders the menus list
func (b *Builder) viewMenus() string {
	var sb strings.Builder

	sb.WriteString(b.styles.Subtitle.Render("📋 Menus"))
	sb.WriteString("\n\n")

	if len(b.config.Menus) == 0 {
		sb.WriteString(b.styles.Muted.Render("No menus configured. Press 'n' to add one."))
	} else {
		for i, menu := range b.config.Menus {
			line := fmt.Sprintf("%s  %s  (%d items)", menu.Trigger, menu.Title, len(menu.Items))
			if i == b.menuCursor {
				sb.WriteString(b.styles.Selected.Render(line))
			} else {
				sb.WriteString(b.styles.Normal.Render(line))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// viewMenuEditor renders the menu item editor
func (b *Builder) viewMenuEditor() string {
	var sb strings.Builder

	if b.editingMenu == nil {
		return "No menu selected"
	}

	sb.WriteString(b.styles.Subtitle.Render(fmt.Sprintf("📋 %s", b.editingMenu.Title)))
	sb.WriteString("\n\n")

	if len(b.editingMenu.Items) == 0 {
		sb.WriteString(b.styles.Muted.Render("No items. Press 'n' to add one."))
	} else {
		for i, item := range b.editingMenu.Items {
			line := fmt.Sprintf("%s %s  %s", item.Icon, item.Label, b.styles.Muted.Render(item.Description))
			if i == b.itemCursor {
				sb.WriteString(b.styles.Selected.Render(line))
			} else {
				sb.WriteString(b.styles.Normal.Render(line))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// viewItemEditor renders the item property editor
func (b *Builder) viewItemEditor() string {
	var sb strings.Builder

	if b.editingItem == nil {
		return "No item selected"
	}

	sb.WriteString(b.styles.Subtitle.Render(fmt.Sprintf("✏️ Editing: %s", b.editingItem.Label)))
	sb.WriteString("\n\n")

	labels := []string{"ID:", "Label:", "Description:", "Category:", "Icon:"}
	for i, label := range labels {
		sb.WriteString(b.styles.Muted.Render(label))
		sb.WriteString("\n")
		if i == b.focusedInput {
			sb.WriteString(b.styles.InputFocus.Render(b.inputs[i].View()))
		} else {
			sb.WriteString(b.styles.Input.Render(b.inputs[i].View()))
		}
		sb.WriteString("\n\n")
	}

	return sb.String()
}

// viewTheme renders the theme editor
func (b *Builder) viewTheme() string {
	var sb strings.Builder
	sb.WriteString(b.styles.Subtitle.Render("🎨 Theme"))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("Name: %s\n", b.config.Theme.Name))
	sb.WriteString(fmt.Sprintf("Mode: %s\n", b.config.Theme.Mode))
	sb.WriteString("\n")
	sb.WriteString(b.styles.Muted.Render("Theme editing coming soon..."))
	return sb.String()
}

// viewKeybindings renders the keybindings editor
func (b *Builder) viewKeybindings() string {
	var sb strings.Builder
	sb.WriteString(b.styles.Subtitle.Render("⌨️ Keybindings"))
	sb.WriteString("\n\n")

	if len(b.config.Keybindings) == 0 {
		sb.WriteString(b.styles.Muted.Render("No keybindings configured."))
	} else {
		for _, kb := range b.config.Keybindings {
			sb.WriteString(fmt.Sprintf("%-12s  %s\n", kb.Key, kb.Description))
		}
	}

	return sb.String()
}

// viewBackend renders the backend configuration
func (b *Builder) viewBackend() string {
	var sb strings.Builder
	sb.WriteString(b.styles.Subtitle.Render("🔌 Backend"))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("Type: %s\n", b.config.Backend.Type))
	sb.WriteString(fmt.Sprintf("URL: %s\n", b.config.Backend.URL))
	sb.WriteString(fmt.Sprintf("Streaming: %v\n", b.config.Backend.Streaming))
	sb.WriteString(fmt.Sprintf("Timeout: %ds\n", b.config.Backend.Timeout))
	sb.WriteString("\n")
	sb.WriteString(b.styles.Muted.Render("Backend editing coming soon..."))
	return sb.String()
}

// getHelp returns context-sensitive help text
func (b *Builder) getHelp() string {
	switch b.mode {
	case ModeMain:
		return "↑/↓: navigate • enter: select • q: quit"
	case ModeMenus:
		return "↑/↓: navigate • enter: edit • n: new menu • d: delete • esc: back"
	case ModeMenuEditor:
		return "↑/↓: navigate • enter: edit item • n: new item • esc: back"
	case ModeItemEditor:
		return "tab: next field • ctrl+s: save • esc: back"
	default:
		return "esc: back • ctrl+s: save"
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// PUBLIC API
// ═══════════════════════════════════════════════════════════════════════════════

// Config returns the current configuration
func (b *Builder) Config() *schema.Config {
	return b.config
}

// SetConfig sets the configuration
func (b *Builder) SetConfig(config *schema.Config) {
	b.config = config
}

// IsModified returns true if the config has been modified
func (b *Builder) IsModified() bool {
	return b.modified
}

// Run starts the builder as a standalone program
func Run(path string) error {
	var builder *Builder
	var err error

	if path != "" {
		builder, err = NewFromFile(path)
		if err != nil {
			// File doesn't exist, create new
			builder = New()
			builder.filePath = path
		}
	} else {
		builder = New()
	}

	p := tea.NewProgram(builder, tea.WithAltScreen())
	_, err = p.Run()
	return err
}
