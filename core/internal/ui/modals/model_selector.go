package modals

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/normanking/cortex/internal/ui/types"
)

// ModelSelectedMsg is sent when a model is selected.
type ModelSelectedMsg struct {
	Model types.ModelInfo
}

// modelItem wraps a ModelInfo for use in the bubbles list.
type modelItem struct {
	model types.ModelInfo
}

// FilterValue implements list.Item interface.
func (i modelItem) FilterValue() string {
	return i.model.Name + " " + i.model.ID + " " + i.model.Provider
}

// Title implements list.DefaultItem interface.
func (i modelItem) Title() string {
	return i.model.Name
}

// Description implements list.DefaultItem interface.
func (i modelItem) Description() string {
	providerTag := "Cloud"
	if i.model.IsLocal {
		providerTag = "Local"
	}

	// Show provider and tag
	return fmt.Sprintf("%s • %s", i.model.Provider, providerTag)
}

// ModelSelector is a modal for selecting an AI model.
type ModelSelector struct {
	list   list.Model
	width  int
	height int
	filter string
}

// NewModelSelector creates a new model selector modal.
func NewModelSelector(models []types.ModelInfo) *ModelSelector {
	// Convert ModelInfo to list items
	items := make([]list.Item, len(models))
	for i, model := range models {
		items[i] = modelItem{model: model}
	}

	// Create the list with default delegate
	delegate := list.NewDefaultDelegate()

	// Customize delegate styling
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
	l.Title = "Select Model"
	l.SetShowHelp(true)
	l.SetFilteringEnabled(true)
	l.SetShowStatusBar(true)

	// Customize title style
	l.Styles.Title = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7aa2f7")).
		Bold(true).
		Padding(0, 1)

	return &ModelSelector{
		list:   l,
		width:  60,
		height: 20,
	}
}

// SetSize sets the modal dimensions.
func (m *ModelSelector) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.list.SetSize(width-4, height-4) // Account for border and padding
}

// Init implements tea.Model.
func (m *ModelSelector) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m *ModelSelector) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			// Close modal - parent should handle this
			return m, nil

		case "enter":
			// Select current item
			if item, ok := m.list.SelectedItem().(modelItem); ok {
				return m, func() tea.Msg {
					return ModelSelectedMsg{Model: item.model}
				}
			}
			return m, nil

		case "/":
			// Toggle filtering
			if m.list.FilterState() != list.Filtering {
				m.list.SetFilteringEnabled(true)
			}
		}
	}

	// Update the list
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View implements tea.Model and renders the model selector.
func (m *ModelSelector) View() string {
	// Render the list
	content := m.list.View()

	// Wrap in rounded border
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7aa2f7")).
		Padding(0, 1).
		Width(m.width)

	return boxStyle.Render(content)
}

// SetFilter sets the search filter for the list.
func (m *ModelSelector) SetFilter(filter string) {
	m.filter = strings.ToLower(filter)
	// Enable filtering if filter is not empty
	if filter != "" {
		m.list.SetFilteringEnabled(true)
	}
}
