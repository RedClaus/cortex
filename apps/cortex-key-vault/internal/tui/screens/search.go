package screens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/normanking/cortex-key-vault/internal/service"
	"github.com/normanking/cortex-key-vault/internal/storage"
	"github.com/normanking/cortex-key-vault/internal/tui/styles"
)

// SearchScreen provides fuzzy search overlay
type SearchScreen struct {
	search   *service.SearchService
	input    textinput.Model
	results  []service.SearchResult
	selected int
	width    int
	height   int
	closed   bool
	chosen   *storage.Secret
}

// SearchClosedMsg is sent when search is closed
type SearchClosedMsg struct{}

// SearchSelectedMsg is sent when a result is selected
type SearchSelectedMsg struct {
	Secret *storage.Secret
}

// NewSearchScreen creates a new search screen
func NewSearchScreen(search *service.SearchService) SearchScreen {
	input := textinput.New()
	input.Placeholder = "Search secrets..."
	input.Focus()

	return SearchScreen{
		search: search,
		input:  input,
	}
}

// Init initializes the search screen
func (m SearchScreen) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages
func (m SearchScreen) Update(msg tea.Msg) (SearchScreen, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.closed = true
			return m, func() tea.Msg { return SearchClosedMsg{} }

		case "enter":
			if len(m.results) > 0 && m.selected < len(m.results) {
				m.chosen = &m.results[m.selected].Secret
				return m, func() tea.Msg {
					return SearchSelectedMsg{Secret: m.chosen}
				}
			}

		case "up", "ctrl+p":
			if m.selected > 0 {
				m.selected--
			}

		case "down", "ctrl+n":
			if m.selected < len(m.results)-1 {
				m.selected++
			}

		default:
			// Update input and search
			m.input, cmd = m.input.Update(msg)
			m.performSearch()
			return m, cmd
		}
	}

	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *SearchScreen) performSearch() {
	query := m.input.Value()
	if query == "" {
		m.results = nil
		m.selected = 0
		return
	}

	results, err := m.search.FuzzySearch(query)
	if err != nil {
		m.results = nil
		return
	}

	m.results = results
	if m.selected >= len(m.results) {
		m.selected = len(m.results) - 1
	}
	if m.selected < 0 {
		m.selected = 0
	}
}

// View renders the search screen
func (m SearchScreen) View() string {
	// Handle zero dimensions
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	var content strings.Builder

	// Search header
	content.WriteString(styles.HeaderTitle.Render("🔍 Search Secrets") + "\n\n")

	// Search input
	inputStyle := styles.InputFocused.Width(50)
	content.WriteString(inputStyle.Render(m.input.View()) + "\n\n")

	// Results
	if m.input.Value() == "" {
		content.WriteString(styles.TextMutedStyle.Render("  Type to search...") + "\n")
	} else if len(m.results) == 0 {
		content.WriteString(styles.TextMutedStyle.Render("  No results found") + "\n")
	} else {
		content.WriteString(styles.TextSecondaryStyle.Render(fmt.Sprintf("  %d results", len(m.results))) + "\n\n")

		// Show up to 10 results
		maxResults := 10
		if len(m.results) < maxResults {
			maxResults = len(m.results)
		}

		for i := 0; i < maxResults; i++ {
			result := m.results[i]
			icon := storage.GetIconForType(result.Secret.Type)
			name := result.Secret.Name

			// Highlight matched characters
			if len(result.MatchedChars) > 0 {
				name = highlightMatches(name, result.MatchedChars)
			}

			line := fmt.Sprintf("%s %s", icon, name)

			if i == m.selected {
				content.WriteString(styles.ListItemSelected.Width(50).Render(line) + "\n")
			} else {
				content.WriteString(styles.ListItem.Width(50).Render(line) + "\n")
			}
		}

		if len(m.results) > maxResults {
			content.WriteString(styles.TextMutedStyle.Render(fmt.Sprintf("  ... and %d more", len(m.results)-maxResults)) + "\n")
		}
	}

	content.WriteString("\n")
	content.WriteString(styles.HelpText.Render("[↑/↓] Navigate  [Enter] Select  [Esc] Close"))

	// Wrap in panel
	panel := styles.Panel.Width(60).Render(content.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, panel)
}

// IsClosed returns true if search is closed
func (m SearchScreen) IsClosed() bool {
	return m.closed
}

// GetChosen returns the selected secret
func (m SearchScreen) GetChosen() *storage.Secret {
	return m.chosen
}

// highlightMatches highlights matched characters in a string
func highlightMatches(s string, indices []int) string {
	if len(indices) == 0 {
		return s
	}

	// Create a set of matched indices
	matchSet := make(map[int]bool)
	for _, i := range indices {
		matchSet[i] = true
	}

	// Build highlighted string
	var result strings.Builder
	highlight := lipgloss.NewStyle().Foreground(styles.Secondary).Bold(true)

	for i, r := range s {
		if matchSet[i] {
			result.WriteString(highlight.Render(string(r)))
		} else {
			result.WriteRune(r)
		}
	}

	return result.String()
}
