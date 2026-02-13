// Package app provides the main BubbleTea application model for Salamander TUI.
//
// The App struct is the central component that ties together all UI elements:
// chat view, input field, command menus, and status bar. It implements the
// tea.Model interface and handles all user interactions including keyboard
// input, message sending, and menu navigation.
//
// Usage:
//
//	cfg, _ := config.LoadConfig("salamander.yaml")
//	app := app.New(cfg)
//	p := tea.NewProgram(app, tea.WithAltScreen())
//	p.Run()
package app

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/normanking/salamander/internal/backend"
	"github.com/normanking/salamander/internal/theme"
	"github.com/normanking/salamander/pkg/schema"
)

// ═══════════════════════════════════════════════════════════════════════════════
// MESSAGE TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// SendMessageMsg is sent when the user submits a message.
type SendMessageMsg struct {
	Content string
}

// ResponseMsg is received when a complete response is available.
type ResponseMsg struct {
	Content   string
	Artifacts []Artifact
}

// StreamingMsg is received during streaming responses.
type StreamingMsg struct {
	Content string
	Done    bool
}

// ErrorMsg is received when an error occurs.
type ErrorMsg struct {
	Error error
}

// tickMsg is used for loading animation.
type tickMsg time.Time

// ═══════════════════════════════════════════════════════════════════════════════
// DATA TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// ChatMessage represents a message in the chat history.
type ChatMessage struct {
	Role      string // "user", "assistant", "system"
	Content   string
	Timestamp time.Time
	Artifacts []Artifact // for A2A artifacts
}

// Artifact represents an A2A artifact attached to a message.
type Artifact struct {
	Type string
	Name string
	Data interface{}
}

// MenuItem represents a menu item for slash commands.
type MenuItem struct {
	ID          string
	Label       string
	Description string
	Icon        string
	Action      func() tea.Cmd
}

// ═══════════════════════════════════════════════════════════════════════════════
// APP MODEL
// ═══════════════════════════════════════════════════════════════════════════════

// App is the main BubbleTea model for Salamander TUI applications.
type App struct {
	// Config
	config *schema.Config

	// Components
	chatView   viewport.Model  // scrollable chat history
	inputField textinput.Model // user input

	// Menu state
	menuItems    []MenuItem
	menuCursor   int
	menuFilter   string
	isMenuOpen   bool
	menuMaxItems int

	// State
	messages         []ChatMessage
	isLoading        bool
	loadingFrame     int
	currentTask      string // A2A task ID
	streamContent    string // accumulating streaming content
	connectionStatus string

	// Dimensions
	width, height int

	// Theme
	theme *theme.Theme

	// A2A Backend client
	a2aClient *backend.Client
	agentName string // Name from agent card

	// Styles (derived from theme)
	styles appStyles
}

// appStyles holds all computed lipgloss styles.
type appStyles struct {
	titleBar       lipgloss.Style
	titleBarText   lipgloss.Style
	statusDot      lipgloss.Style
	chatArea       lipgloss.Style
	userLabel      lipgloss.Style
	assistantLabel lipgloss.Style
	systemLabel    lipgloss.Style
	messageContent lipgloss.Style
	inputArea      lipgloss.Style
	inputPrompt    lipgloss.Style
	statusBar      lipgloss.Style
	statusBarItem  lipgloss.Style
	menuBox        lipgloss.Style
	menuTitle      lipgloss.Style
	menuItem       lipgloss.Style
	menuItemSel    lipgloss.Style
	menuDesc       lipgloss.Style
	loading        lipgloss.Style
	timestamp      lipgloss.Style
	code           lipgloss.Style
	bold           lipgloss.Style
}

// ═══════════════════════════════════════════════════════════════════════════════
// CONSTRUCTOR
// ═══════════════════════════════════════════════════════════════════════════════

// New creates a new App from a configuration.
func New(cfg *schema.Config) *App {
	// Initialize theme
	t := theme.NewTheme(cfg.Theme)

	// Initialize input field
	ti := textinput.New()
	ti.Placeholder = "Ask anything..."
	ti.Focus()
	ti.CharLimit = 4096
	ti.Width = 80

	// Initialize viewport (will be resized on WindowSizeMsg)
	vp := viewport.New(80, 20)
	vp.SetContent("")

	app := &App{
		config:           cfg,
		chatView:         vp,
		inputField:       ti,
		messages:         []ChatMessage{},
		theme:            t,
		menuItems:        buildMenuItems(cfg),
		menuMaxItems:     8,
		connectionStatus: "Disconnected",
	}

	// Initialize A2A client if backend URL is configured
	if cfg.Backend.URL != "" && cfg.Backend.Type == "a2a" {
		client, err := backend.NewClient(cfg.Backend)
		if err == nil {
			app.a2aClient = client
			app.connectionStatus = "Connecting..."
		}
	} else if cfg.Backend.Type == "mock" {
		app.connectionStatus = "Mock Mode"
	}

	// Build styles from theme
	app.styles = buildStyles(t)

	// Add welcome message if configured
	if cfg.App.WelcomeMessage != "" {
		app.addMessage("system", cfg.App.WelcomeMessage)
	}

	return app
}

// buildStyles creates all lipgloss styles from a theme.
func buildStyles(t *theme.Theme) appStyles {
	return appStyles{
		titleBar: lipgloss.NewStyle().
			Background(lipgloss.Color(t.Colors.Surface)).
			Foreground(lipgloss.Color(t.Colors.Text)).
			Padding(0, 1),
		titleBarText: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(t.Colors.Primary)),
		statusDot: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.Success)),
		chatArea: lipgloss.NewStyle().
			Padding(1, 2),
		userLabel: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(t.Colors.Primary)),
		assistantLabel: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(t.Colors.Secondary)),
		systemLabel: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(t.Colors.Info)),
		messageContent: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.Text)),
		inputArea: lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(lipgloss.Color(t.Colors.Border)).
			Padding(0, 1),
		inputPrompt: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.Primary)).
			Bold(true),
		statusBar: lipgloss.NewStyle().
			Background(lipgloss.Color(t.Colors.Surface)).
			Foreground(lipgloss.Color(t.Colors.TextMuted)).
			Padding(0, 1),
		statusBarItem: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.TextMuted)).
			Padding(0, 1),
		menuBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(t.Colors.Border)).
			Background(lipgloss.Color(t.Colors.Surface)).
			Padding(0, 1),
		menuTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(t.Colors.Primary)).
			MarginBottom(1),
		menuItem: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.Text)).
			Padding(0, 1),
		menuItemSel: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.Background)).
			Background(lipgloss.Color(t.Colors.Primary)).
			Bold(true).
			Padding(0, 1),
		menuDesc: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.TextMuted)).
			Italic(true),
		loading: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.Warning)),
		timestamp: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.TextMuted)).
			Italic(true),
		code: lipgloss.NewStyle().
			Background(lipgloss.Color(t.Colors.Surface)).
			Foreground(lipgloss.Color(t.Colors.Accent)).
			Padding(0, 1),
		bold: lipgloss.NewStyle().
			Bold(true),
	}
}

// buildMenuItems creates menu items from config.
func buildMenuItems(cfg *schema.Config) []MenuItem {
	items := []MenuItem{}

	// Add default items
	items = append(items,
		MenuItem{
			ID:          "help",
			Label:       "help",
			Description: "Show help",
			Icon:        "?",
		},
		MenuItem{
			ID:          "clear",
			Label:       "clear",
			Description: "Clear chat history",
			Icon:        "x",
		},
		MenuItem{
			ID:          "model",
			Label:       "model",
			Description: "Select model",
			Icon:        "*",
		},
	)

	// Add items from config menus
	for _, menu := range cfg.Menus {
		if menu.Trigger == "/" {
			for _, item := range menu.Items {
				items = append(items, MenuItem{
					ID:          item.ID,
					Label:       item.Label,
					Description: item.Description,
					Icon:        item.Icon,
				})
			}
		}
	}

	return items
}

// ═══════════════════════════════════════════════════════════════════════════════
// TEA MODEL IMPLEMENTATION
// ═══════════════════════════════════════════════════════════════════════════════

// Init implements tea.Model.
func (a *App) Init() tea.Cmd {
	cmds := []tea.Cmd{textinput.Blink}

	// Start A2A connection if client is configured
	if a.a2aClient != nil {
		cmds = append(cmds, a.a2aClient.Connect())
	}

	return tea.Batch(cmds...)
}

// Update implements tea.Model.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return a.handleWindowSize(msg)

	case tea.KeyMsg:
		return a.handleKey(msg)

	case tickMsg:
		if a.isLoading {
			a.loadingFrame = (a.loadingFrame + 1) % 4
			return a, tick()
		}
		return a, nil

	case SendMessageMsg:
		return a.handleSendMessage(msg)

	case ResponseMsg:
		return a.handleResponse(msg)

	case StreamingMsg:
		return a.handleStreaming(msg)

	case ErrorMsg:
		a.isLoading = false
		a.addMessage("system", fmt.Sprintf("Error: %v", msg.Error))
		return a, nil

	// A2A Backend messages
	case backend.ConnectedMsg:
		a.connectionStatus = "Connected"
		if msg.AgentCard != nil {
			a.agentName = msg.AgentCard.Name
			a.addMessage("system", fmt.Sprintf("Connected to %s", msg.AgentCard.Name))
		}
		return a, nil

	case backend.DisconnectedMsg:
		a.connectionStatus = "Disconnected"
		if msg.Error != nil {
			a.addMessage("system", fmt.Sprintf("Connection failed: %v", msg.Error))
		}
		return a, nil

	case backend.TaskStartedMsg:
		a.currentTask = msg.TaskID
		return a, nil

	case backend.TaskUpdateMsg:
		return a.handleA2AUpdate(msg.Update)

	case backend.TaskCompletedMsg:
		return a.handleA2ACompleted(msg.Result)
	}

	// Update input field
	if !a.isMenuOpen {
		var cmd tea.Cmd
		a.inputField, cmd = a.inputField.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Update viewport
	var cmd tea.Cmd
	a.chatView, cmd = a.chatView.Update(msg)
	cmds = append(cmds, cmd)

	return a, tea.Batch(cmds...)
}

// handleWindowSize handles terminal resize events.
func (a *App) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	a.width = msg.Width
	a.height = msg.Height

	// Calculate component heights
	titleBarHeight := 1
	statusBarHeight := 1
	inputHeight := 3
	chatHeight := a.height - titleBarHeight - statusBarHeight - inputHeight - 2

	if chatHeight < 5 {
		chatHeight = 5
	}

	// Update viewport
	a.chatView.Width = a.width - 4
	a.chatView.Height = chatHeight

	// Update input width
	a.inputField.Width = a.width - 6

	// Re-render chat content
	a.updateChatContent()

	return a, nil
}

// handleKey handles keyboard input.
func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global keys
	switch msg.String() {
	case "ctrl+c":
		if a.isLoading {
			a.isLoading = false
			a.currentTask = ""
			// Cancel A2A task if active
			if a.a2aClient != nil {
				return a, a.a2aClient.Cancel()
			}
			return a, nil
		}
		// Clean up A2A client before quitting
		if a.a2aClient != nil {
			a.a2aClient.Close()
		}
		return a, tea.Quit

	case "ctrl+l":
		a.messages = []ChatMessage{}
		a.updateChatContent()
		return a, nil
	}

	// Menu-specific keys
	if a.isMenuOpen {
		return a.handleMenuKey(msg)
	}

	// Input-specific keys
	switch msg.String() {
	case "enter":
		if a.inputField.Value() != "" {
			content := a.inputField.Value()
			a.inputField.SetValue("")
			return a, func() tea.Msg {
				return SendMessageMsg{Content: content}
			}
		}

	case "up":
		a.chatView.ScrollUp(1)
		return a, nil

	case "down":
		a.chatView.ScrollDown(1)
		return a, nil

	case "pgup":
		a.chatView.HalfViewUp()
		return a, nil

	case "pgdown":
		a.chatView.HalfViewDown()
		return a, nil

	case "esc":
		if a.isLoading {
			a.isLoading = false
			a.currentTask = ""
			// Cancel A2A task if active
			if a.a2aClient != nil {
				return a, a.a2aClient.Cancel()
			}
		}
		return a, nil
	}

	// Check for slash command trigger
	currentValue := a.inputField.Value()
	if msg.String() == "/" && currentValue == "" {
		a.openMenu()
		return a, nil
	}

	// Update input
	var cmd tea.Cmd
	a.inputField, cmd = a.inputField.Update(msg)

	// Check if input starts with / to open menu
	if strings.HasPrefix(a.inputField.Value(), "/") && len(a.inputField.Value()) > 0 {
		a.menuFilter = strings.TrimPrefix(a.inputField.Value(), "/")
		if !a.isMenuOpen {
			a.openMenu()
		}
	} else if a.isMenuOpen && !strings.HasPrefix(a.inputField.Value(), "/") {
		a.closeMenu()
	}

	return a, cmd
}

// handleMenuKey handles keyboard input when menu is open.
func (a *App) handleMenuKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	filteredItems := a.getFilteredMenuItems()

	switch msg.String() {
	case "esc":
		a.closeMenu()
		a.inputField.SetValue("")
		return a, nil

	case "enter":
		if len(filteredItems) > 0 && a.menuCursor < len(filteredItems) {
			item := filteredItems[a.menuCursor]
			a.closeMenu()
			a.inputField.SetValue("")
			return a.executeMenuItem(item)
		}
		return a, nil

	case "up":
		if a.menuCursor > 0 {
			a.menuCursor--
		} else {
			a.menuCursor = len(filteredItems) - 1
		}
		return a, nil

	case "down":
		if a.menuCursor < len(filteredItems)-1 {
			a.menuCursor++
		} else {
			a.menuCursor = 0
		}
		return a, nil

	case "tab":
		// Auto-complete selected item
		if len(filteredItems) > 0 && a.menuCursor < len(filteredItems) {
			a.inputField.SetValue("/" + filteredItems[a.menuCursor].Label)
			a.menuFilter = filteredItems[a.menuCursor].Label
		}
		return a, nil

	default:
		// Update input and filter
		var cmd tea.Cmd
		a.inputField, cmd = a.inputField.Update(msg)
		a.menuFilter = strings.TrimPrefix(a.inputField.Value(), "/")
		a.menuCursor = 0 // Reset cursor on filter change
		return a, cmd
	}
}

// handleSendMessage handles sending a message.
func (a *App) handleSendMessage(msg SendMessageMsg) (tea.Model, tea.Cmd) {
	// Check for slash command
	if strings.HasPrefix(msg.Content, "/") {
		cmdName := strings.TrimPrefix(msg.Content, "/")
		cmdName = strings.Split(cmdName, " ")[0]
		for _, item := range a.menuItems {
			if item.Label == cmdName {
				return a.executeMenuItem(item)
			}
		}
	}

	// Add user message
	a.addMessage("user", msg.Content)

	// Start loading
	a.isLoading = true
	a.loadingFrame = 0

	// Use A2A client if connected, otherwise use mock
	if a.a2aClient != nil && a.a2aClient.IsConnected() {
		return a, tea.Batch(
			tick(),
			a.a2aClient.Send(msg.Content),
		)
	}

	// Fall back to mock response
	return a, tea.Batch(
		tick(),
		simulateResponse(msg.Content),
	)
}

// handleResponse handles a complete response.
func (a *App) handleResponse(msg ResponseMsg) (tea.Model, tea.Cmd) {
	a.isLoading = false
	a.currentTask = ""
	a.streamContent = ""

	chatMsg := ChatMessage{
		Role:      "assistant",
		Content:   msg.Content,
		Timestamp: time.Now(),
		Artifacts: msg.Artifacts,
	}
	a.messages = append(a.messages, chatMsg)
	a.updateChatContent()

	return a, nil
}

// handleStreaming handles streaming content updates.
func (a *App) handleStreaming(msg StreamingMsg) (tea.Model, tea.Cmd) {
	a.streamContent += msg.Content

	if msg.Done {
		a.isLoading = false
		a.addMessage("assistant", a.streamContent)
		a.streamContent = ""
		return a, nil
	}

	// Update chat with partial content
	a.updateChatContent()
	return a, nil
}

// handleA2AUpdate handles streaming updates from the A2A backend.
func (a *App) handleA2AUpdate(update backend.StreamUpdate) (tea.Model, tea.Cmd) {
	switch update.Type {
	case "content":
		a.streamContent += update.Content
		a.updateChatContent()
	case "status":
		// Could show status in UI if desired
	case "artifact":
		// Handle artifact updates
		if update.Artifact != nil {
			// Store artifact for later display
		}
	case "error":
		a.isLoading = false
		if update.Error != nil {
			a.addMessage("system", fmt.Sprintf("Error: %v", update.Error))
		}
		return a, nil
	case "done":
		a.isLoading = false
		if a.streamContent != "" {
			a.addMessage("assistant", a.streamContent)
			a.streamContent = ""
		}
		return a, nil
	}
	return a, nil
}

// handleA2ACompleted handles task completion from the A2A backend.
func (a *App) handleA2ACompleted(result backend.TaskResult) (tea.Model, tea.Cmd) {
	a.isLoading = false
	a.currentTask = result.TaskID
	a.streamContent = ""

	switch result.Status {
	case "completed":
		if result.Response != "" {
			// Convert backend artifacts to app artifacts
			var artifacts []Artifact
			for _, a := range result.Artifacts {
				artifacts = append(artifacts, Artifact{
					Type: a.Type,
					Name: a.Name,
					Data: a.Data,
				})
			}
			chatMsg := ChatMessage{
				Role:      "assistant",
				Content:   result.Response,
				Timestamp: time.Now(),
				Artifacts: artifacts,
			}
			a.messages = append(a.messages, chatMsg)
			a.updateChatContent()
		}
	case "failed":
		errMsg := "Task failed"
		if result.Error != nil {
			errMsg = result.Error.Error()
		}
		a.addMessage("system", fmt.Sprintf("Error: %s", errMsg))
	case "cancelled":
		a.addMessage("system", "Task cancelled")
	}

	return a, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// VIEW
// ═══════════════════════════════════════════════════════════════════════════════

// View implements tea.Model.
func (a *App) View() string {
	if a.width == 0 || a.height == 0 {
		return "Loading..."
	}

	var sections []string

	// Title bar
	if a.config.App.ShowTitleBar {
		sections = append(sections, a.renderTitleBar())
	}

	// Chat view with optional menu overlay
	chatSection := a.renderChatView()
	if a.isMenuOpen {
		chatSection = a.overlayMenu(chatSection)
	}
	sections = append(sections, chatSection)

	// Input area
	sections = append(sections, a.renderInput())

	// Status bar
	if a.config.App.ShowStatusBar {
		sections = append(sections, a.renderStatusBar())
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderTitleBar renders the title bar.
func (a *App) renderTitleBar() string {
	// App name and version
	title := a.styles.titleBarText.Render(
		fmt.Sprintf("%s v%s", a.config.App.Name, a.config.App.Version),
	)

	// Connection status
	statusIcon := "o"
	if a.connectionStatus == "Connected" {
		statusIcon = a.styles.statusDot.Render("o")
	}
	status := fmt.Sprintf("%s %s", statusIcon, a.connectionStatus)

	// Calculate spacing
	titleWidth := lipgloss.Width(title)
	statusWidth := lipgloss.Width(status)
	padding := a.width - titleWidth - statusWidth - 4
	if padding < 1 {
		padding = 1
	}

	bar := fmt.Sprintf("%s%s%s", title, strings.Repeat(" ", padding), status)
	return a.styles.titleBar.Width(a.width).Render(bar)
}

// renderChatView renders the scrollable chat area.
func (a *App) renderChatView() string {
	return a.chatView.View()
}

// renderInput renders the input area.
func (a *App) renderInput() string {
	prompt := a.styles.inputPrompt.Render("> ")
	input := a.inputField.View()

	line := fmt.Sprintf("%s%s", prompt, input)
	return a.styles.inputArea.Width(a.width).Render(line)
}

// renderStatusBar renders the status bar.
func (a *App) renderStatusBar() string {
	// Agent/Model info
	agentInfo := "Mock Mode"
	if a.a2aClient != nil && a.a2aClient.IsConnected() {
		if a.agentName != "" {
			agentInfo = a.agentName
		} else {
			agentInfo = "A2A Agent"
		}
	}
	modelInfo := a.styles.statusBarItem.Render(fmt.Sprintf("Agent: %s", agentInfo))

	// Token count (placeholder)
	tokens := a.styles.statusBarItem.Render("Tokens: 0")

	// Loading or timing
	var timing string
	if a.isLoading {
		frames := []string{"|", "/", "-", "\\"}
		timing = a.styles.loading.Render(fmt.Sprintf("%s Loading...", frames[a.loadingFrame]))
	} else {
		timing = a.styles.statusBarItem.Render("")
	}

	// Join with separators
	sep := a.styles.statusBarItem.Render("|")
	content := fmt.Sprintf("%s %s %s %s %s", modelInfo, sep, tokens, sep, timing)

	return a.styles.statusBar.Width(a.width).Render(content)
}

// renderMenu renders the slash command menu.
func (a *App) renderMenu() string {
	var sb strings.Builder

	sb.WriteString(a.styles.menuTitle.Render("Commands"))
	sb.WriteString("\n")

	filteredItems := a.getFilteredMenuItems()
	displayCount := a.menuMaxItems
	if len(filteredItems) < displayCount {
		displayCount = len(filteredItems)
	}

	for i := 0; i < displayCount; i++ {
		item := filteredItems[i]
		icon := item.Icon
		if icon == "" {
			icon = " "
		}

		// Format: > label     description
		label := fmt.Sprintf("%s %s", icon, item.Label)
		desc := a.styles.menuDesc.Render(item.Description)

		// Pad label to align descriptions
		labelPadded := fmt.Sprintf("%-16s", label)

		line := fmt.Sprintf("%s %s", labelPadded, desc)
		if i == a.menuCursor {
			sb.WriteString(a.styles.menuItemSel.Render(line))
		} else {
			sb.WriteString(a.styles.menuItem.Render(line))
		}
		sb.WriteString("\n")
	}

	if len(filteredItems) > displayCount {
		sb.WriteString(a.styles.menuDesc.Render(fmt.Sprintf("  ... and %d more", len(filteredItems)-displayCount)))
	}

	return a.styles.menuBox.Render(sb.String())
}

// overlayMenu places the menu on top of the chat area.
func (a *App) overlayMenu(chat string) string {
	menu := a.renderMenu()

	// Split chat into lines
	chatLines := strings.Split(chat, "\n")
	menuLines := strings.Split(menu, "\n")

	menuWidth := lipgloss.Width(menu)
	menuHeight := len(menuLines)

	// Position menu near the bottom of chat area
	startLine := len(chatLines) - menuHeight - 2
	if startLine < 2 {
		startLine = 2
	}
	startCol := 2

	// Overlay menu on chat
	result := make([]string, len(chatLines))
	for i, line := range chatLines {
		if i >= startLine && i < startLine+menuHeight {
			menuLineIdx := i - startLine
			if menuLineIdx < len(menuLines) {
				// Replace portion of chat line with menu line
				result[i] = overlayString(line, menuLines[menuLineIdx], startCol, menuWidth)
			} else {
				result[i] = line
			}
		} else {
			result[i] = line
		}
	}

	return strings.Join(result, "\n")
}

// overlayString overlays src onto dst at the given position.
func overlayString(dst, src string, col, width int) string {
	// Ensure dst is long enough
	for len(dst) < col+width {
		dst += " "
	}

	// Simple overlay - just replace characters
	dstRunes := []rune(dst)
	srcRunes := []rune(src)

	for i, r := range srcRunes {
		pos := col + i
		if pos < len(dstRunes) {
			dstRunes[pos] = r
		}
	}

	return string(dstRunes)
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPER METHODS
// ═══════════════════════════════════════════════════════════════════════════════

// addMessage adds a message to the chat history.
func (a *App) addMessage(role, content string) {
	msg := ChatMessage{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}
	a.messages = append(a.messages, msg)
	a.updateChatContent()
}

// updateChatContent updates the viewport content from messages.
func (a *App) updateChatContent() {
	var sb strings.Builder

	for _, msg := range a.messages {
		// Role label
		var label string
		switch msg.Role {
		case "user":
			label = a.styles.userLabel.Render("You:")
		case "assistant":
			label = a.styles.assistantLabel.Render("Assistant:")
		case "system":
			label = a.styles.systemLabel.Render("System:")
		default:
			label = msg.Role + ":"
		}

		sb.WriteString(label)
		sb.WriteString("\n")

		// Message content with basic markdown rendering
		content := a.renderMarkdown(msg.Content)
		sb.WriteString(a.styles.messageContent.Render(content))
		sb.WriteString("\n\n")
	}

	// Add streaming content if present
	if a.streamContent != "" {
		sb.WriteString(a.styles.assistantLabel.Render("Assistant:"))
		sb.WriteString("\n")
		sb.WriteString(a.styles.messageContent.Render(a.streamContent))
		if a.isLoading {
			sb.WriteString(a.styles.loading.Render(" ..."))
		}
		sb.WriteString("\n")
	}

	a.chatView.SetContent(sb.String())
	a.chatView.GotoBottom()
}

// renderMarkdown performs basic markdown rendering.
func (a *App) renderMarkdown(content string) string {
	// Bold: **text** or __text__
	boldRe := regexp.MustCompile(`\*\*(.+?)\*\*|__(.+?)__`)
	content = boldRe.ReplaceAllStringFunc(content, func(match string) string {
		inner := strings.Trim(match, "*_")
		return a.styles.bold.Render(inner)
	})

	// Inline code: `code`
	codeRe := regexp.MustCompile("`([^`]+)`")
	content = codeRe.ReplaceAllStringFunc(content, func(match string) string {
		inner := strings.Trim(match, "`")
		return a.styles.code.Render(inner)
	})

	return content
}

// openMenu opens the slash command menu.
func (a *App) openMenu() {
	a.isMenuOpen = true
	a.menuCursor = 0
	a.menuFilter = ""
}

// closeMenu closes the slash command menu.
func (a *App) closeMenu() {
	a.isMenuOpen = false
	a.menuCursor = 0
	a.menuFilter = ""
}

// getFilteredMenuItems returns menu items matching the current filter.
func (a *App) getFilteredMenuItems() []MenuItem {
	if a.menuFilter == "" {
		return a.menuItems
	}

	filter := strings.ToLower(a.menuFilter)
	var filtered []MenuItem
	for _, item := range a.menuItems {
		if strings.Contains(strings.ToLower(item.Label), filter) ||
			strings.Contains(strings.ToLower(item.Description), filter) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// executeMenuItem executes a menu item's action.
func (a *App) executeMenuItem(item MenuItem) (tea.Model, tea.Cmd) {
	switch item.ID {
	case "help":
		a.addMessage("system", "Available commands:\n"+
			"  /help  - Show this help\n"+
			"  /clear - Clear chat history\n"+
			"  /model - Select model\n"+
			"\nKeyboard shortcuts:\n"+
			"  Ctrl+C - Quit or cancel\n"+
			"  Ctrl+L - Clear chat\n"+
			"  Up/Down - Scroll chat\n"+
			"  Esc - Close menu or cancel")
		return a, nil

	case "clear":
		a.messages = []ChatMessage{}
		a.updateChatContent()
		return a, nil

	case "model":
		a.addMessage("system", "Model selection not yet implemented.")
		return a, nil

	default:
		// Execute custom action if defined
		if item.Action != nil {
			return a, item.Action()
		}
		a.addMessage("system", fmt.Sprintf("Command '%s' executed.", item.Label))
		return a, nil
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// COMMANDS
// ═══════════════════════════════════════════════════════════════════════════════

// tick returns a command that ticks the loading animation.
func tick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// simulateResponse simulates a backend response (stub).
func simulateResponse(input string) tea.Cmd {
	return func() tea.Msg {
		// Simulate network delay
		time.Sleep(500 * time.Millisecond)

		// Generate a mock response
		response := fmt.Sprintf(
			"This is a simulated response to: **%s**\n\n"+
				"The A2A backend is not yet connected. "+
				"Configure `backend.url` in your YAML file to connect to an agent.",
			input,
		)

		return ResponseMsg{
			Content: response,
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// PUBLIC API
// ═══════════════════════════════════════════════════════════════════════════════

// Config returns the current configuration.
func (a *App) Config() *schema.Config {
	return a.config
}

// SetTheme updates the app's theme.
func (a *App) SetTheme(t *theme.Theme) {
	a.theme = t
	a.styles = buildStyles(t)
}

// Messages returns a copy of the chat messages.
func (a *App) Messages() []ChatMessage {
	msgs := make([]ChatMessage, len(a.messages))
	copy(msgs, a.messages)
	return msgs
}

// IsLoading returns whether the app is waiting for a response.
func (a *App) IsLoading() bool {
	return a.isLoading
}

// Run starts the app as a standalone BubbleTea program.
func Run(cfg *schema.Config) error {
	app := New(cfg)
	p := tea.NewProgram(app, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
