package screens

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/normanking/cortex-key-vault/internal/service"
	"github.com/normanking/cortex-key-vault/internal/storage"
	"github.com/normanking/cortex-key-vault/internal/tui/styles"
)

// DetailScreen shows full secret details
type DetailScreen struct {
	vault     *service.VaultService
	clipboard *service.ClipboardService
	secret    *storage.Secret
	value     string
	width     int
	height    int
	showValue bool
	copied    bool
	copyTime  time.Time
	closed    bool
}

// DetailClosedMsg is sent when detail view is closed
type DetailClosedMsg struct{}

// NewDetailScreen creates a new detail view
func NewDetailScreen(vault *service.VaultService, clipboard *service.ClipboardService, secret *storage.Secret) DetailScreen {
	value, _ := vault.GetSecretValue(secret.ID)

	return DetailScreen{
		vault:     vault,
		clipboard: clipboard,
		secret:    secret,
		value:     value,
	}
}

// Init initializes the detail screen
func (m DetailScreen) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m DetailScreen) Update(msg tea.Msg) (DetailScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			m.closed = true
			return m, func() tea.Msg { return DetailClosedMsg{} }

		case "c":
			m.clipboard.Copy(m.value)
			m.copied = true
			m.copyTime = time.Now()

		case "r", "v":
			m.showValue = !m.showValue

		case "u":
			// Copy username
			if m.secret.Username != "" {
				m.clipboard.Copy(m.secret.Username)
				m.copied = true
				m.copyTime = time.Now()
			}
		}
	}

	return m, nil
}

// View renders the detail screen
func (m DetailScreen) View() string {
	// Handle zero dimensions
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	var content strings.Builder

	// Header
	icon := storage.GetIconForType(m.secret.Type)
	content.WriteString(styles.HeaderTitle.Render(fmt.Sprintf("%s %s", icon, m.secret.Name)) + "\n")
	content.WriteString(styles.Divider.Render(strings.Repeat("─", 50)) + "\n\n")

	// Type
	content.WriteString(styles.InputLabel.Render("Type") + "\n")
	typeInfo := getTypeInfo(m.secret.Type)
	content.WriteString(styles.TextSecondaryStyle.Render(typeInfo) + "\n\n")

	// Value
	content.WriteString(styles.InputLabel.Render("Value") + " ")
	if m.showValue {
		content.WriteString(styles.TextMutedStyle.Render("[r] Hide") + "\n")
		content.WriteString(styles.TextPrimaryStyle.Render(m.value) + "\n\n")
	} else {
		content.WriteString(styles.TextMutedStyle.Render("[r] Reveal") + "\n")
		content.WriteString(styles.MaskedText.Render(strings.Repeat("•", min(len(m.value), 30))) + "\n\n")
	}

	// Username (if present)
	if m.secret.Username != "" {
		content.WriteString(styles.InputLabel.Render("Username") + " " + styles.TextMutedStyle.Render("[u] Copy") + "\n")
		content.WriteString(styles.TextSecondaryStyle.Render(m.secret.Username) + "\n\n")
	}

	// URL (if present)
	if m.secret.URL != "" {
		content.WriteString(styles.InputLabel.Render("URL") + "\n")
		content.WriteString(styles.TextSecondaryStyle.Render(m.secret.URL) + "\n\n")
	}

	// Notes (if present)
	if m.secret.Notes != "" {
		content.WriteString(styles.InputLabel.Render("Notes") + "\n")
		content.WriteString(styles.TextSecondaryStyle.Render(m.secret.Notes) + "\n\n")
	}

	// Tags
	if len(m.secret.Tags) > 0 {
		content.WriteString(styles.InputLabel.Render("Tags") + "\n")
		var tags []string
		for _, t := range m.secret.Tags {
			tags = append(tags, styles.Tag.Render("#"+t))
		}
		content.WriteString(strings.Join(tags, " ") + "\n\n")
	}

	// Timestamps
	content.WriteString(styles.Divider.Render(strings.Repeat("─", 50)) + "\n")
	content.WriteString(styles.TextMutedStyle.Render(fmt.Sprintf("Created: %s", m.secret.CreatedAt.Format("Jan 2, 2006 3:04 PM"))) + "\n")
	content.WriteString(styles.TextMutedStyle.Render(fmt.Sprintf("Updated: %s", m.secret.UpdatedAt.Format("Jan 2, 2006 3:04 PM"))) + "\n\n")

	// Copy notification
	if m.copied && time.Since(m.copyTime) < 3*time.Second {
		content.WriteString(styles.SuccessText.Render("✓ Copied to clipboard (clears in 30s)") + "\n\n")
	}

	// Help
	content.WriteString(styles.HelpText.Render("[c] Copy value  [r] Reveal/hide  [Esc] Close"))

	// Wrap in panel
	panel := styles.Panel.Width(60).Render(content.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, panel)
}

// IsClosed returns true if the detail view is closed
func (m DetailScreen) IsClosed() bool {
	return m.closed
}

func getTypeInfo(t storage.SecretType) string {
	info := storage.GetSecretTypeInfo()
	for _, i := range info {
		if i.Type == t {
			return fmt.Sprintf("%s %s - %s", i.Icon, i.Name, i.Description)
		}
	}
	return string(t)
}
