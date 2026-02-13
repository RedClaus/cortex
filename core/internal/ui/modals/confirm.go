package modals

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ConfirmResult represents the user's choice in a confirmation dialog.
type ConfirmResult int

const (
	ConfirmResultNone ConfirmResult = iota
	ConfirmResultYes
	ConfirmResultNo
)

// ConfirmMsg is sent when the user makes a choice.
type ConfirmMsg struct {
	Result  ConfirmResult
	Payload interface{} // Optional payload to identify what was confirmed
}

// ConfirmModal displays a Yes/No confirmation dialog for dangerous operations.
type ConfirmModal struct {
	title    string
	message  string
	selected int // 0 = Yes, 1 = No
	width    int
	height   int
	payload  interface{} // Optional data to pass back with the confirmation
}

// NewConfirmModal creates a new confirmation modal.
//
// Parameters:
//   - title: The title of the confirmation dialog
//   - message: The message/question to display
//   - payload: Optional data to identify what is being confirmed (returned in ConfirmMsg)
func NewConfirmModal(title, message string, payload interface{}) *ConfirmModal {
	return &ConfirmModal{
		title:    title,
		message:  message,
		selected: 1,       // Default to "No" for safety
		width:    60,      // Fixed width
		payload:  payload, // Store payload for later
	}
}

// SetSize sets the modal dimensions.
func (c *ConfirmModal) SetSize(width, height int) {
	c.width = width
	c.height = height
}

// Init implements tea.Model.
func (c *ConfirmModal) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (c *ConfirmModal) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			// Cancel (same as No)
			return c, func() tea.Msg {
				return ConfirmMsg{
					Result:  ConfirmResultNo,
					Payload: c.payload,
				}
			}

		case "left", "h", "shift+tab":
			// Move to Yes
			c.selected = 0
			return c, nil

		case "right", "l", "tab":
			// Move to No
			c.selected = 1
			return c, nil

		case "enter", " ":
			// Confirm selection
			result := ConfirmResultNo
			if c.selected == 0 {
				result = ConfirmResultYes
			}
			return c, func() tea.Msg {
				return ConfirmMsg{
					Result:  result,
					Payload: c.payload,
				}
			}

		case "y", "Y":
			// Quick Yes
			return c, func() tea.Msg {
				return ConfirmMsg{
					Result:  ConfirmResultYes,
					Payload: c.payload,
				}
			}

		case "n", "N":
			// Quick No
			return c, func() tea.Msg {
				return ConfirmMsg{
					Result:  ConfirmResultNo,
					Payload: c.payload,
				}
			}
		}
	}

	return c, nil
}

// View implements tea.Model and renders the confirmation dialog.
func (c *ConfirmModal) View() string {
	var lines []string

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#f7768e")). // Error/warning color
		Align(lipgloss.Center).
		Width(c.width - 4)

	lines = append(lines, titleStyle.Render(c.title))
	lines = append(lines, "")

	// Message - wrap text to fit width
	messageStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#c0caf5")).
		Width(c.width - 4).
		Align(lipgloss.Center)

	// Word wrap the message
	wrappedMessage := wordWrap(c.message, c.width-8)
	for _, line := range wrappedMessage {
		lines = append(lines, messageStyle.Render(line))
	}
	lines = append(lines, "")
	lines = append(lines, "")

	// Buttons
	yesStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#c0caf5")).
		Background(lipgloss.Color("#1a1b26")).
		Padding(0, 3).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#565f89"))

	yesSelectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#1a1b26")).
		Background(lipgloss.Color("#9ece6a")). // Green for Yes
		Padding(0, 3).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#9ece6a")).
		Bold(true)

	noStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#c0caf5")).
		Background(lipgloss.Color("#1a1b26")).
		Padding(0, 3).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#565f89"))

	noSelectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#1a1b26")).
		Background(lipgloss.Color("#f7768e")). // Red for No
		Padding(0, 3).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#f7768e")).
		Bold(true)

	// Render buttons based on selection
	yesButton := "Yes"
	noButton := "No"

	if c.selected == 0 {
		yesButton = yesSelectedStyle.Render(yesButton)
		noButton = noStyle.Render(noButton)
	} else {
		yesButton = yesStyle.Render(yesButton)
		noButton = noSelectedStyle.Render(noButton)
	}

	// Join buttons with spacing
	buttons := lipgloss.JoinHorizontal(lipgloss.Center, yesButton, "  ", noButton)
	buttonLine := lipgloss.NewStyle().
		Width(c.width - 4).
		Align(lipgloss.Center).
		Render(buttons)

	lines = append(lines, buttonLine)
	lines = append(lines, "")

	// Hint
	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#565f89")).
		Italic(true).
		Align(lipgloss.Center).
		Width(c.width - 4)

	lines = append(lines, hintStyle.Render("Tab to switch • Enter to confirm • Esc to cancel"))

	content := strings.Join(lines, "\n")

	// Wrap in rounded border
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#f7768e")).
		Padding(1, 2).
		Width(c.width)

	return boxStyle.Render(content)
}

// wordWrap wraps text to the specified width.
func wordWrap(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{}
	}

	var lines []string
	var currentLine string

	for _, word := range words {
		if currentLine == "" {
			currentLine = word
		} else if len(currentLine)+1+len(word) <= width {
			currentLine += " " + word
		} else {
			lines = append(lines, currentLine)
			currentLine = word
		}
	}

	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return lines
}
