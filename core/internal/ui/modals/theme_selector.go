package modals

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/normanking/cortex/pkg/theme"
)

// ThemeSelectedMsg is sent when a theme is selected.
type ThemeSelectedMsg struct {
	ThemeID string
	Theme   theme.Palette
}

// themeItem wraps a theme for use in the bubbles list.
type themeItem struct {
	id      string
	palette theme.Palette
}

// FilterValue implements list.Item interface.
func (i themeItem) FilterValue() string {
	return i.palette.Name + " " + i.palette.Type
}

// Title implements list.DefaultItem interface.
func (i themeItem) Title() string {
	return i.palette.Name
}

// Description implements list.DefaultItem interface.
func (i themeItem) Description() string {
	// Show type (dark/light) and a color preview
	typeIcon := "🌙"
	if i.palette.Type == "light" {
		typeIcon = "☀️"
	}

	// Create color preview boxes
	preview := fmt.Sprintf("%s %s %s %s",
		colorBox(i.palette.Primary),
		colorBox(i.palette.Secondary),
		colorBox(i.palette.Success),
		colorBox(i.palette.Warning),
	)

	return fmt.Sprintf("%s %s • %s", typeIcon, i.palette.Type, preview)
}

// colorBox renders a small colored box for color preview.
func colorBox(color string) string {
	style := lipgloss.NewStyle().
		Background(lipgloss.Color(color)).
		Foreground(lipgloss.Color(color)).
		Width(2)
	return style.Render("  ")
}

// ThemeSelector is a modal for selecting a color theme.
type ThemeSelector struct {
	list      list.Model
	width     int
	height    int
	currentID string
}

// NewThemeSelector creates a new theme selector modal.
func NewThemeSelector(currentThemeID string) *ThemeSelector {
	// Get all themes from registry
	themeIDs := theme.List()
	items := make([]list.Item, len(themeIDs))

	// Find current theme index for pre-selection
	currentIdx := 0
	for i, id := range themeIDs {
		palette := theme.Get(id)
		items[i] = themeItem{
			id:      id,
			palette: palette,
		}
		if id == currentThemeID {
			currentIdx = i
		}
	}

	// Create the list with default delegate
	delegate := list.NewDefaultDelegate()

	// Customize delegate styling (use Tokyo Night colors as default)
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#1a1b26")).
		Background(lipgloss.Color("#7aa2f7")).
		Bold(true)

	delegate.Styles.SelectedDesc = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#1a1b26")).
		Background(lipgloss.Color("#7aa2f7"))

	delegate.Styles.NormalTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#c0caf5"))

	delegate.Styles.NormalDesc = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#565f89"))

	l := list.New(items, delegate, 60, 20)
	l.Title = "Select Theme"
	l.SetShowHelp(true)
	l.SetFilteringEnabled(true)
	l.SetShowStatusBar(true)

	// Set cursor to current theme
	l.Select(currentIdx)

	// Customize title style
	l.Styles.Title = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7aa2f7")).
		Bold(true).
		Padding(0, 1)

	return &ThemeSelector{
		list:      l,
		width:     60,
		height:    20,
		currentID: currentThemeID,
	}
}

// SetSize sets the modal dimensions.
func (t *ThemeSelector) SetSize(width, height int) {
	t.width = width
	t.height = height
	t.list.SetSize(width-4, height-4) // Account for border and padding
}

// Init implements tea.Model.
func (t *ThemeSelector) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (t *ThemeSelector) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			// Close modal - parent should handle this
			return t, nil

		case "enter":
			// Select current item
			if item, ok := t.list.SelectedItem().(themeItem); ok {
				return t, func() tea.Msg {
					return ThemeSelectedMsg{
						ThemeID: item.id,
						Theme:   item.palette,
					}
				}
			}
			return t, nil
		}
	}

	// Update the list
	var cmd tea.Cmd
	t.list, cmd = t.list.Update(msg)
	return t, cmd
}

// View implements tea.Model and renders the theme selector.
func (t *ThemeSelector) View() string {
	// Render the list
	content := t.list.View()

	// Wrap in rounded border
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7aa2f7")).
		Padding(0, 1).
		Width(t.width)

	return boxStyle.Render(content)
}

// CurrentThemeID returns the currently selected theme ID.
func (t *ThemeSelector) CurrentThemeID() string {
	return t.currentID
}
