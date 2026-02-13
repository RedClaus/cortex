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

// DashboardFocus indicates which pane has focus
type DashboardFocus int

const (
	FocusSidebar DashboardFocus = iota
	FocusSecrets
)

// DashboardScreen is the main dashboard model
type DashboardScreen struct {
	vault     *service.VaultService
	clipboard *service.ClipboardService
	search    *service.SearchService

	// UI state
	width        int
	height       int
	focus        DashboardFocus
	showHelp     bool
	notification string
	notifyTime   time.Time

	// Sidebar state
	categories       []storage.Category
	categoryCounts   map[string]int
	tags             []storage.Tag
	tagCounts        map[string]int
	selectedCategory int
	selectedTag      int // -1 means no tag selected
	sidebarMode      int // 0 = categories, 1 = tags

	// Secrets list state
	secrets        []storage.Secret
	selectedSecret int
	secretScroll   int

	// Action requests
	requestAdd    bool
	requestEdit   bool
	requestDelete bool
	requestDetail bool
	requestSearch bool
	requestLock   bool
}

// Messages for dashboard actions
type (
	RefreshSecretsMsg struct{}
	NotificationMsg   struct{ Message string }
	CopySuccessMsg    struct{}
)

// NewDashboardScreen creates a new dashboard screen
func NewDashboardScreen(vault *service.VaultService, clipboard *service.ClipboardService) DashboardScreen {
	search := service.NewSearchService(vault)

	d := DashboardScreen{
		vault:          vault,
		clipboard:      clipboard,
		search:         search,
		categoryCounts: make(map[string]int),
		tagCounts:      make(map[string]int),
		selectedTag:    -1,
	}

	d.loadCategories()
	d.loadTags()
	d.loadSecrets()

	return d
}

func (m *DashboardScreen) loadCategories() {
	cats, _ := m.vault.GetCategories()
	m.categories = cats

	for _, cat := range cats {
		count, _ := m.vault.GetCategoryCount(cat.ID)
		m.categoryCounts[cat.ID] = count
	}
}

func (m *DashboardScreen) loadTags() {
	tags, _ := m.vault.GetTags()
	m.tags = tags

	for _, tag := range tags {
		count, _ := m.vault.GetTagCount(tag.Name)
		m.tagCounts[tag.Name] = count
	}
}

func (m *DashboardScreen) loadSecrets() {
	categoryID := ""
	if m.selectedCategory < len(m.categories) {
		categoryID = m.categories[m.selectedCategory].ID
	}

	var err error
	if m.selectedTag >= 0 && m.selectedTag < len(m.tags) {
		// Filter by tag
		m.secrets, err = m.vault.ListSecretsByTag(m.tags[m.selectedTag].Name)
	} else {
		// Filter by category
		m.secrets, err = m.vault.ListSecrets(categoryID)
	}

	if err != nil {
		// Show error as notification
		m.notification = "Error: " + err.Error()
		m.notifyTime = time.Now()
	}

	// Reset selection if needed
	if m.selectedSecret >= len(m.secrets) {
		m.selectedSecret = len(m.secrets) - 1
	}
	if m.selectedSecret < 0 {
		m.selectedSecret = 0
	}
}

// Init initializes the dashboard
func (m DashboardScreen) Init() tea.Cmd {
	return nil
}

// Update handles messages for the dashboard
func (m DashboardScreen) Update(msg tea.Msg) (DashboardScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case RefreshSecretsMsg:
		m.loadCategories()
		m.loadTags()
		m.loadSecrets()

	case NotificationMsg:
		m.notification = msg.Message
		m.notifyTime = time.Now()

	case tea.KeyMsg:
		// Clear notification on any key
		if time.Since(m.notifyTime) > 3*time.Second {
			m.notification = ""
		}

		// Update vault activity
		m.vault.Touch()

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "?":
			m.showHelp = !m.showHelp

		case "tab":
			if m.focus == FocusSidebar {
				m.focus = FocusSecrets
			} else {
				m.focus = FocusSidebar
			}

		case "h", "left":
			m.focus = FocusSidebar

		case "l", "right":
			m.focus = FocusSecrets

		case "j", "down":
			m.handleDown()

		case "k", "up":
			m.handleUp()

		case "enter":
			if m.focus == FocusSecrets && len(m.secrets) > 0 {
				m.requestDetail = true
			}

		case "a":
			m.requestAdd = true

		case "e":
			if len(m.secrets) > 0 {
				m.requestEdit = true
			}

		case "d":
			if len(m.secrets) > 0 {
				m.requestDelete = true
			}

		case "c":
			if len(m.secrets) > 0 {
				m.copySelectedSecret()
			}

		case "/":
			m.requestSearch = true

		case "L":
			m.requestLock = true

		case "1":
			m.sidebarMode = 0
			m.selectedTag = -1
			m.loadSecrets()

		case "2":
			m.sidebarMode = 1
		}
	}

	return m, nil
}

func (m *DashboardScreen) handleDown() {
	if m.focus == FocusSidebar {
		if m.sidebarMode == 0 {
			// Categories
			if m.selectedCategory < len(m.categories)-1 {
				m.selectedCategory++
				m.selectedTag = -1
				m.loadSecrets()
			}
		} else {
			// Tags
			if m.selectedTag < len(m.tags)-1 {
				m.selectedTag++
				m.loadSecrets()
			}
		}
	} else {
		// Secrets
		if m.selectedSecret < len(m.secrets)-1 {
			m.selectedSecret++
		}
	}
}

func (m *DashboardScreen) handleUp() {
	if m.focus == FocusSidebar {
		if m.sidebarMode == 0 {
			// Categories
			if m.selectedCategory > 0 {
				m.selectedCategory--
				m.selectedTag = -1
				m.loadSecrets()
			}
		} else {
			// Tags
			if m.selectedTag > -1 {
				m.selectedTag--
				if m.selectedTag == -1 {
					m.loadSecrets()
				} else {
					m.loadSecrets()
				}
			}
		}
	} else {
		// Secrets
		if m.selectedSecret > 0 {
			m.selectedSecret--
		}
	}
}

func (m *DashboardScreen) copySelectedSecret() {
	if m.selectedSecret >= len(m.secrets) {
		return
	}

	secret := m.secrets[m.selectedSecret]
	value, err := m.vault.GetSecretValue(secret.ID)
	if err != nil {
		m.notification = "Failed to copy: " + err.Error()
		m.notifyTime = time.Now()
		return
	}

	m.clipboard.Copy(value)
	m.notification = fmt.Sprintf("Copied %s to clipboard (clears in 30s)", secret.Name)
	m.notifyTime = time.Now()
}

// View renders the dashboard
func (m DashboardScreen) View() string {
	// Handle zero dimensions (before WindowSizeMsg)
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	if m.showHelp {
		return m.renderHelp()
	}

	// Build output manually without lipgloss layout functions
	var output strings.Builder

	// Header line
	output.WriteString(fmt.Sprintf("🔐 CORTEX KEY VAULT | %d secrets | Press ? for help\n", len(m.secrets)))
	output.WriteString(strings.Repeat("─", m.width) + "\n")

	// Simple two-column layout using string formatting
	sidebarWidth := 28
	secretsWidth := m.width - sidebarWidth - 2

	// Get sidebar lines
	var sidebarLines []string
	sidebarLines = append(sidebarLines, "[1] Categories [2] Tags")
	sidebarLines = append(sidebarLines, strings.Repeat("-", sidebarWidth-2))
	for i, cat := range m.categories {
		prefix := "  "
		if i == m.selectedCategory {
			prefix = "> "
		}
		sidebarLines = append(sidebarLines, fmt.Sprintf("%s%s %s (%d)", prefix, cat.Icon, cat.Name, m.categoryCounts[cat.ID]))
	}

	// Get secrets lines
	var secretLines []string
	secretLines = append(secretLines, "Secrets")
	secretLines = append(secretLines, strings.Repeat("-", secretsWidth-2))
	if len(m.secrets) == 0 {
		secretLines = append(secretLines, "No secrets found")
	} else {
		for i, secret := range m.secrets {
			if i >= 20 { // Limit display
				secretLines = append(secretLines, fmt.Sprintf("... and %d more", len(m.secrets)-20))
				break
			}
			icon := storage.GetIconForType(secret.Type)
			prefix := "  "
			if i == m.selectedSecret && m.focus == FocusSecrets {
				prefix = "> "
			}
			secretLines = append(secretLines, fmt.Sprintf("%s%s %s", prefix, icon, secret.Name))
			secretLines = append(secretLines, fmt.Sprintf("     %s • %s", secret.Type, formatTimeAgo(secret.UpdatedAt)))
		}
	}

	// Combine columns side by side
	maxLines := len(sidebarLines)
	if len(secretLines) > maxLines {
		maxLines = len(secretLines)
	}

	for i := 0; i < maxLines && i < m.height-4; i++ {
		sidebarLine := ""
		if i < len(sidebarLines) {
			sidebarLine = sidebarLines[i]
		}
		secretLine := ""
		if i < len(secretLines) {
			secretLine = secretLines[i]
		}

		// Pad sidebar to fixed width
		for len(sidebarLine) < sidebarWidth {
			sidebarLine += " "
		}
		if len(sidebarLine) > sidebarWidth {
			sidebarLine = sidebarLine[:sidebarWidth]
		}

		output.WriteString(sidebarLine + "│ " + secretLine + "\n")
	}

	// Status bar
	output.WriteString(strings.Repeat("─", m.width) + "\n")
	output.WriteString("[/] Search  [a] Add  [e] Edit  [c] Copy  [d] Delete  [L] Lock  [?] Help  [q] Quit")

	return output.String()
}

func (m DashboardScreen) renderHeader() string {
	// DEBUG: Simple plain text header
	return fmt.Sprintf("=== CORTEX KEY VAULT === (%d secrets, width=%d)\n", len(m.secrets), m.width)
}

func (m DashboardScreen) renderSidebar(width, height int) string {
	var content strings.Builder

	// DEBUG: Add visible marker
	content.WriteString("=== SIDEBAR ===\n")
	content.WriteString(fmt.Sprintf("Categories: %d\n", len(m.categories)))
	content.WriteString("[1] Categories [2] Tags\n")
	content.WriteString(strings.Repeat("-", width-4) + "\n")

	if m.sidebarMode == 0 {
		// Categories - plain text for debugging
		for i, cat := range m.categories {
			count := m.categoryCounts[cat.ID]
			prefix := "  "
			if i == m.selectedCategory {
				prefix = "> "
			}
			content.WriteString(fmt.Sprintf("%s%s %s (%d)\n", prefix, cat.Icon, cat.Name, count))
		}
	} else {
		// Tags - plain text
		prefix := "  "
		if m.selectedTag == -1 {
			prefix = "> "
		}
		content.WriteString(prefix + "All Tags\n")

		for i, tag := range m.tags {
			count := m.tagCounts[tag.Name]
			prefix = "  "
			if i == m.selectedTag {
				prefix = "> "
			}
			content.WriteString(fmt.Sprintf("%s#%s (%d)\n", prefix, tag.Name, count))
		}
	}

	// DEBUG: Return plain text without styling to test
	return content.String()
}

func (m DashboardScreen) renderSecretsList(width, height int) string {
	var content strings.Builder

	content.WriteString(styles.HeaderTitle.Render("Secrets") + "\n")
	content.WriteString(styles.Divider.Render(strings.Repeat("─", width-4)) + "\n")

	if len(m.secrets) == 0 {
		content.WriteString("\n")
		content.WriteString(styles.TextMutedStyle.Render("  No secrets found.\n"))
		content.WriteString(styles.TextMutedStyle.Render("  Press [a] to add a new secret.\n"))
	} else {
		for i, secret := range m.secrets {
			icon := storage.GetIconForType(secret.Type)
			name := styles.TruncateWithEllipsis(secret.Name, width-15)
			typeStr := string(secret.Type)
			timeStr := formatTimeAgo(secret.UpdatedAt)

			line1 := fmt.Sprintf("%s %s", icon, name)
			line2 := fmt.Sprintf("   %s • %s", typeStr, timeStr)

			if i == m.selectedSecret {
				if m.focus == FocusSecrets {
					content.WriteString(styles.ListItemSelected.Width(width - 4).Render(line1) + "\n")
					content.WriteString(styles.ListItemSelected.Width(width - 4).Render(line2) + "\n")
				} else {
					content.WriteString(styles.ListItem.Width(width - 4).Render("▶ " + line1[2:]) + "\n")
					content.WriteString(styles.ListItemSubtext.Width(width - 4).Render(line2) + "\n")
				}
			} else {
				content.WriteString(styles.ListItem.Width(width - 4).Render(line1) + "\n")
				content.WriteString(styles.ListItemSubtext.Width(width - 4).Render(line2) + "\n")
			}
		}
	}

	style := styles.Panel.Width(width).Height(height)
	if m.focus == FocusSecrets {
		style = style.BorderForeground(styles.Primary)
	}

	return style.Render(content.String())
}

func (m DashboardScreen) renderStatusBar() string {
	keys := []string{
		styles.RenderKeybind("/", "Search"),
		styles.RenderKeybind("a", "Add"),
		styles.RenderKeybind("e", "Edit"),
		styles.RenderKeybind("c", "Copy"),
		styles.RenderKeybind("d", "Delete"),
		styles.RenderKeybind("L", "Lock"),
		styles.RenderKeybind("?", "Help"),
	}

	return styles.StatusBar.Width(m.width).Render(strings.Join(keys, "  "))
}

func (m DashboardScreen) renderHelp() string {
	help := `
  Cortex Key Vault - Help
  ═══════════════════════════════════════════

  NAVIGATION
  ──────────────────────────────────────────
  Tab / h,l     Switch between sidebar and secrets
  j,k / ↑,↓     Move up/down in current pane
  Enter         View secret details
  1,2           Switch sidebar tabs (Categories/Tags)

  ACTIONS
  ──────────────────────────────────────────
  a             Add new secret
  e             Edit selected secret
  d             Delete selected secret
  c             Copy secret value to clipboard
  /             Search secrets

  OTHER
  ──────────────────────────────────────────
  L             Lock vault
  ?             Toggle this help
  q / Ctrl+C    Quit

  Press any key to close help...
`
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		styles.Panel.Render(help),
	)
}

// GetSelectedSecret returns the currently selected secret
func (m DashboardScreen) GetSelectedSecret() *storage.Secret {
	if m.selectedSecret >= 0 && m.selectedSecret < len(m.secrets) {
		return &m.secrets[m.selectedSecret]
	}
	return nil
}

// Request flags - checked by parent to know what action to take
func (m DashboardScreen) WantsAdd() bool    { return m.requestAdd }
func (m DashboardScreen) WantsEdit() bool   { return m.requestEdit }
func (m DashboardScreen) WantsDelete() bool { return m.requestDelete }
func (m DashboardScreen) WantsDetail() bool { return m.requestDetail }
func (m DashboardScreen) WantsSearch() bool { return m.requestSearch }
func (m DashboardScreen) WantsLock() bool   { return m.requestLock }

// ClearRequests clears all action requests
func (m *DashboardScreen) ClearRequests() {
	m.requestAdd = false
	m.requestEdit = false
	m.requestDelete = false
	m.requestDetail = false
	m.requestSearch = false
	m.requestLock = false
}

// Refresh reloads the dashboard data
func (m *DashboardScreen) Refresh() {
	m.loadCategories()
	m.loadTags()
	m.loadSecrets()
}

func formatTimeAgo(t time.Time) string {
	d := time.Since(t)

	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d mins ago", mins)
	case d < 24*time.Hour:
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case d < 7*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	case d < 30*24*time.Hour:
		weeks := int(d.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	default:
		months := int(d.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	}
}
