package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/RedClaus/cortex-coder-agent/pkg/cortexbrain"
)

// Screen represents the current screen
type Screen int

const (
	ScreenModelSelect Screen = iota
	ScreenModelTest
	ScreenMain
	ScreenHelp
	ScreenDiff
)

// ModelInfo holds model information
type ModelInfo struct {
	Name        string
	Provider    string
	Description string
	Available   bool
}

// AppConfig holds TUI configuration
type AppConfig struct {
	Theme          Theme
	RootPath       string
	SessionID      string
	SessionName    string
	AutoSave       bool
	SaveInterval   time.Duration
	Models         []ModelInfo
	SelectedModel  string
	MaxFileSize    int64 // Maximum file size to load
	ChatRetention  int   // Maximum chat messages to keep
}

// AppModel is the main application model
type AppModel struct {
	styles        Styles
	config        AppConfig
	screen        Screen
	width         int
	height        int
	quit          bool
	
	// Model selection
	modelCursor     int
	modelTestResult string
	modelTesting    bool
	
	// Main screen components
	browser       *FileBrowser
	chat          *ChatPanel
	editor        *EditorPanel
	diffViewer    *DiffViewer
	changeManager *ChangeManager
	activePanel   int // 0 = browser, 1 = chat, 2 = editor
	
	// CortexBrain client
	cbClient    *cortexbrain.Client
	cbConnected bool
	
	// Status
	statusMsg  string
	statusTime time.Time
	
	// Help state
	helpVisible bool
	
	keys AppKeyMap
}

// AppKeyMap defines key bindings
type AppKeyMap struct {
	Quit        key.Binding
	Help        key.Binding
	FocusNext   key.Binding
	SelectModel key.Binding
	TestModel   key.Binding
	ShowEditor  key.Binding
	ShowDiff    key.Binding
}

func DefaultAppKeyMap() AppKeyMap {
	return AppKeyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q/ctrl+c", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		FocusNext: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "switch panel"),
		),
		SelectModel: key.NewBinding(
			key.WithKeys("m"),
			key.WithHelp("m", "change model"),
		),
		TestModel: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "test model"),
		),
		ShowEditor: key.NewBinding(
			key.WithKeys("3"),
			key.WithHelp("3", "focus editor"),
		),
		ShowDiff: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "show diff"),
		),
	}
}

// NewAppModel creates a new app model
func NewAppModel(config AppConfig) (*AppModel, error) {
	styles := NewStyles(config.Theme)
	
	app := &AppModel{
		styles:       styles,
		config:       config,
		screen:       ScreenModelSelect,
		activePanel:  0,
		keys:         DefaultAppKeyMap(),
		helpVisible:  false,
	}
	
	// Set default values
	if config.MaxFileSize == 0 {
		config.MaxFileSize = 10 * 1024 * 1024 // 10MB default
	}
	if config.ChatRetention == 0 {
		config.ChatRetention = 100 // 100 messages default
	}
	
	// Create CortexBrain client (Pink runs the brain)
	cbClient := cortexbrain.NewClient("http://192.168.1.186:18892", "ws://192.168.1.186:18892/ws", "")
	app.cbClient = cbClient
	
	// Create components
	browser, err := NewFileBrowser(styles, config.RootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file browser: %w", err)
	}
	app.browser = browser
	
	chat := NewChatPanel(styles, 80, 24)
	chat.SetOnSubmit(app.handleChatSubmit)
	app.chat = chat
	
	editor := NewEditorPanel(styles, 80, 24)
	editor.SetOnFileOpen(app.onFileOpen)
	editor.SetOnFileClose(app.onFileClose)
	app.editor = editor
	
	diffViewer := NewDiffViewer(styles, 80, 24)
	diffViewer.SetOnClose(app.onDiffClose)
	app.diffViewer = diffViewer
	
	changeManager := NewChangeManager(styles, 80, 24)
	changeManager.SetOnApply(app.onApplyChange)
	changeManager.SetOnAccept(app.onAcceptChange)
	changeManager.SetOnReject(app.onRejectChange)
	app.changeManager = changeManager
	
	return app, nil
}

// Init initializes the app
func (m *AppModel) Init() tea.Cmd {
	// Fetch available models from CortexBrain
	return m.fetchModelsCmd()
}

// Update handles updates
func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateSizes()
		
	case modelsFetchedMsg:
		m.handleModelsFetched(msg)
		
	case modelTestResultMsg:
		m.handleModelTestResult(msg)
		
	case agentResponseMsg:
		m.handleAgentResponse(msg)
		
	case tea.KeyMsg:
		// Global keys
		if key.Matches(msg, m.keys.Quit) {
			m.quit = true
			return m, tea.Quit
		}
		
		if key.Matches(msg, m.keys.Help) {
			if m.screen == ScreenHelp {
				m.screen = ScreenMain
			} else {
				m.screen = ScreenHelp
			}
			return m, nil
		}
		
		// Screen-specific handling
		switch m.screen {
		case ScreenModelSelect:
			return m.updateModelSelect(msg)
		case ScreenModelTest:
			return m.updateModelTest(msg)
		case ScreenMain:
			return m.updateMain(msg)
		case ScreenDiff:
			return m.updateDiff(msg)
		}
	}
	
	return m, nil
}

// handleModelsFetched handles the models fetched message
func (m *AppModel) handleModelsFetched(msg modelsFetchedMsg) {
	if msg.error != "" {
		// Fall back to hardcoded models if fetch fails
		m.config.Models = []ModelInfo{
			{Name: "kimi-for-coding", Provider: "kimi-code", Description: "Kimi Code (fast)", Available: true},
			{Name: "glm-4.7", Provider: "zai-coding", Description: "GLM 4.7 (reasoning)", Available: true},
			{Name: "claude-opus-4", Provider: "anthropic", Description: "Claude Opus (smart)", Available: true},
		}
	} else {
		// Convert API models to UI models
		m.config.Models = make([]ModelInfo, 0, len(msg.models))
		for _, model := range msg.models {
			m.config.Models = append(m.config.Models, ModelInfo{
				Name:        model.ID,
				Provider:    model.Backend,
				Description: model.Type,
				Available:   true,
			})
		}
	}
}

// handleModelTestResult handles the model test result
func (m *AppModel) handleModelTestResult(msg modelTestResultMsg) {
	m.modelTesting = false
	if msg.success {
		m.modelTestResult = fmt.Sprintf("✅ Connected (%s latency)", msg.latency)
	} else {
		m.modelTestResult = fmt.Sprintf("❌ Failed: %s", msg.error)
	}
}

// handleAgentResponse handles the agent response
func (m *AppModel) handleAgentResponse(msg agentResponseMsg) {
	if msg.error != "" {
		m.chat.AddAgentMessage(fmt.Sprintf("❌ Error: %s", msg.error))
	} else {
		m.chat.AddAgentMessage(msg.content)
	}
	
	// Apply chat message retention limit
	if m.config.ChatRetention > 0 && len(m.chat.GetMessages()) > m.config.ChatRetention {
		// Keep only the last N messages
		messages := m.chat.GetMessages()
		if len(messages) > m.config.ChatRetention {
			messages = messages[len(messages)-m.config.ChatRetention:]
			// Note: This is a simple approach; a more sophisticated one would be to truncate oldest
		}
	}
}

// updateModelSelect handles model selection screen
func (m *AppModel) updateModelSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		m.quit = true
		return m, tea.Quit
		
	case msg.String() == "up", msg.String() == "k":
		if m.modelCursor > 0 {
			m.modelCursor--
		}
		
	case msg.String() == "down", msg.String() == "j":
		if m.modelCursor < len(m.config.Models)-1 {
			m.modelCursor++
		}
		
	case msg.String() == "enter":
		if m.modelCursor < len(m.config.Models) {
			m.config.SelectedModel = m.config.Models[m.modelCursor].Name
			m.screen = ScreenModelTest
			m.modelTesting = true
			m.modelTestResult = "Testing connection..."
			return m, m.testModelCmd()
		}
	}
	
	return m, nil
}

// updateModelTest handles model test screen
func (m *AppModel) updateModelTest(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		m.quit = true
		return m, tea.Quit
		
	case msg.String() == "enter", msg.String() == "y":
		if !m.modelTesting {
			m.initMainScreen()
		}
		
	case msg.String() == "n", msg.String() == "esc":
		if !m.modelTesting {
			m.screen = ScreenModelSelect
		}
		
	case key.Matches(msg, m.keys.TestModel):
		m.modelTesting = true
		m.modelTestResult = "Retesting connection..."
		return m, m.testModelCmd()
	}
	
	return m, nil
}

// initMainScreen initializes the main screen
func (m *AppModel) initMainScreen() {
	m.screen = ScreenMain
	m.activePanel = 0
	m.browser.Focus()
	m.chat.Blur()
	m.editor.Blur()
	m.setStatus(fmt.Sprintf("Connected to %s", m.config.SelectedModel))
}

// updateMain handles main screen
func (m *AppModel) updateMain(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Only handle global shortcuts if chat is not focused
	// (when chat is focused, keys go to text input)
	if !m.chat.IsFocused() && !m.editor.IsFocused() {
		switch {
		case key.Matches(msg, m.keys.SelectModel):
			m.screen = ScreenModelSelect
			return m, nil
			
		case key.Matches(msg, m.keys.FocusNext):
			m.switchPanel()
			return m, nil
			
		case key.Matches(msg, m.keys.ShowEditor):
			m.activePanel = 2
			m.browser.Blur()
			m.chat.Blur()
			m.editor.Focus()
			m.setStatus("Editor focused")
			return m, nil
			
		case key.Matches(msg, m.keys.ShowDiff):
			if m.changeManager.HasChanges() {
				m.screen = ScreenDiff
				m.diffViewer.Focus()
			} else {
				m.setStatus("No changes to review")
			}
			return m, nil
		}
	}
	
	// Panel selection always works
	switch {
	case msg.String() == "1":
		m.activePanel = 0
		m.browser.Focus()
		m.chat.Blur()
		m.editor.Blur()
		return m, nil
		
	case msg.String() == "2":
		m.activePanel = 1
		m.browser.Blur()
		m.chat.Focus()
		m.editor.Blur()
		return m, nil
		
	case msg.String() == "3":
		m.activePanel = 2
		m.browser.Blur()
		m.chat.Blur()
		m.editor.Focus()
		return m, nil
	}
	
	// Handle Enter in browser to open file
	if m.activePanel == 0 {
		switch msg.String() {
		case "enter":
			if node := m.browser.GetSelected(); node != nil && !node.IsDir {
				if err := m.editor.OpenFile(node.Path); err != nil {
					m.setStatus(fmt.Sprintf("Error opening file: %v", err))
				} else {
					m.setStatus(fmt.Sprintf("Opened: %s", node.Name))
				}
			}
			return m, nil
		}
		
		cmd := m.browser.Update(msg)
		return m, cmd
	} else if m.activePanel == 1 {
		cmd := m.chat.Update(msg)
		return m, cmd
	} else {
		cmd := m.editor.Update(msg)
		return m, cmd
	}
}

// updateDiff handles diff screen
func (m *AppModel) updateDiff(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	cmd := m.diffViewer.Update(msg)
	
	// Check if diff was closed
	if !m.diffViewer.HasDiff() || msg.String() == "esc" || msg.String() == "q" {
		m.screen = ScreenMain
		m.diffViewer.Blur()
	}
	
	return m, cmd
}

// switchPanel switches between panels
func (m *AppModel) switchPanel() {
	m.activePanel++
	if m.activePanel > 2 {
		m.activePanel = 0
	}
	
	switch m.activePanel {
	case 0:
		m.browser.Focus()
		m.chat.Blur()
		m.editor.Blur()
	case 1:
		m.browser.Blur()
		m.chat.Focus()
		m.editor.Blur()
	case 2:
		m.browser.Blur()
		m.chat.Blur()
		m.editor.Focus()
	}
}

// updateSizes updates component sizes
func (m *AppModel) updateSizes() {
	if m.width == 0 || m.height == 0 {
		return
	}
	
	// Three-panel layout: browser (25%) | editor (35%) | chat (40%)
	browserWidth := m.width / 4
	editorWidth := m.width * 7 / 20 // 35%
	chatWidth := m.width - browserWidth - editorWidth - 2 // -2 for borders
	panelHeight := m.height - 4
	
	m.browser.SetSize(browserWidth, panelHeight)
	m.editor.SetSize(editorWidth, panelHeight)
	m.chat.SetSize(chatWidth, panelHeight)
	
	// Update diff viewer and change manager
	m.diffViewer.SetSize(m.width, m.height)
	m.changeManager.SetSize(m.width, m.height)
}

// setStatus sets a status message
func (m *AppModel) setStatus(msg string) {
	m.statusMsg = msg
	m.statusTime = time.Now()
}

// handleChatSubmit handles chat submission - returns command for async processing
func (m *AppModel) handleChatSubmit(content string) tea.Cmd {
	return func() tea.Msg {
		// Check for skill command (starts with /)
		if strings.HasPrefix(content, "/") {
			return m.executeSkillCommand(content)
		}

		// Send to CortexBrain
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		req := cortexbrain.PromptRequest{
			SessionID: m.config.SessionID,
			Prompt:    content,
			Context: map[string]interface{}{
				"model": m.config.SelectedModel,
				"project_path": m.config.RootPath,
			},
		}

		resp, err := m.cbClient.SendPrompt(ctx, req)
		if err != nil {
			return agentResponseMsg{error: err.Error()}
		}

		return agentResponseMsg{content: resp.Content}
	}
}

// executeSkillCommand executes a skill from /command
func (m *AppModel) executeSkillCommand(content string) tea.Msg {
	// Parse command
	parts := strings.Fields(content)
	if len(parts) == 0 {
		return agentResponseMsg{error: "empty command"}
	}

	cmd := strings.TrimPrefix(parts[0], "/")
	args := parts[1:]

	// TODO: Get skills executor from app and execute
	return agentResponseMsg{content: fmt.Sprintf("[Skill: /%s] Executing with args: %v", cmd, args)}
}

// testModelCmd returns a command to test the model
func (m *AppModel) testModelCmd() tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		
		// Test connection to CortexBrain
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		_, err := m.cbClient.HealthCheck(ctx)
		if err != nil {
			return modelTestResultMsg{success: false, error: err.Error()}
		}
		
		latency := time.Since(start)
		return modelTestResultMsg{
			success: true, 
			latency: latency.Round(time.Millisecond).String(),
		}
	}
}

// modelTestResultMsg is sent when model test completes
type modelTestResultMsg struct {
	success bool
	latency string
	error   string
}

// agentResponseMsg is sent when agent responds
type agentResponseMsg struct {
	content string
	error   string
}

// onFileOpen is called when a file is opened in the editor
func (m *AppModel) onFileOpen(filename string) {
	m.setStatus(fmt.Sprintf("Opened: %s", filename))
}

// onFileClose is called when a file is closed in the editor
func (m *AppModel) onFileClose(filename string) {
	m.setStatus(fmt.Sprintf("Closed: %s", filename))
}

// onDiffClose is called when the diff viewer is closed
func (m *AppModel) onDiffClose() {
	m.screen = ScreenMain
}

// onApplyChange is called when a change is applied
func (m *AppModel) onApplyChange(change *SuggestedChange) error {
	m.setStatus(fmt.Sprintf("Applied change: %s", change.Description))
	return nil
}

// onAcceptChange is called when a change is accepted
func (m *AppModel) onAcceptChange(change *SuggestedChange) {
	m.setStatus(fmt.Sprintf("Accepted: %s", change.Description))
}

// onRejectChange is called when a change is rejected
func (m *AppModel) onRejectChange(change *SuggestedChange) {
	m.setStatus(fmt.Sprintf("Rejected: %s", change.Description))
}

// View renders the app
func (m *AppModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}
	
	switch m.screen {
	case ScreenModelSelect:
		return m.viewModelSelect()
	case ScreenModelTest:
		return m.viewModelTest()
	case ScreenMain:
		return m.viewMain()
	case ScreenHelp:
		return m.viewHelp()
	case ScreenDiff:
		return m.viewDiff()
	}
	
	return "Unknown screen"
}

// viewModelSelect renders model selection screen
func (m *AppModel) viewModelSelect() string {
	title := m.styles.App.Title.Render("🤖 Select Model")
	
	var models []string
	for i, model := range m.config.Models {
		cursor := "  "
		if i == m.modelCursor {
			cursor = "▸ "
		}
		
		status := "❌"
		if model.Available {
			status = "✅"
		}
		
		line := fmt.Sprintf("%s%s %s - %s (%s)",
			cursor,
			status,
			model.Name,
			model.Description,
			model.Provider,
		)
		
		if i == m.modelCursor {
			line = m.styles.App.FocusedBorder.Render(line)
		} else {
			line = m.styles.App.PanelBorder.Render(line)
		}
		
		models = append(models, line)
	}
	
	help := m.styles.Help.Desc.Render("↑/↓ to navigate • Enter to select • q to quit")
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		lipgloss.JoinVertical(lipgloss.Left, models...),
		"",
		help,
	)
}

// viewModelTest renders model test screen
func (m *AppModel) viewModelTest() string {
	title := m.styles.App.Title.Render("🔍 Test Model")
	
	model := m.config.SelectedModel
	
	var content []string
	content = append(content, fmt.Sprintf("Model: %s", model))
	content = append(content, "")
	content = append(content, m.modelTestResult)
	
	if !m.modelTesting {
		content = append(content, "")
		content = append(content, m.styles.Help.Desc.Render("Enter/y to continue • n/Esc to go back • t to retest"))
	}
	
	box := m.styles.App.Container.
		Width(m.width - 4).
		Height(m.height - 4).
		Render(lipgloss.JoinVertical(lipgloss.Left, content...))
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		box,
	)
}

// viewMain renders main screen
func (m *AppModel) viewMain() string {
	// Update sizes
	m.updateSizes()
	
	// Header
	header := m.styles.App.Title.Render(fmt.Sprintf("🧠 Cortex Coder — %s", m.config.SelectedModel))
	
	// Panels
	browserView := m.browser.View()
	editorView := m.editor.View()
	chatView := m.chat.View()
	
	panels := lipgloss.JoinHorizontal(lipgloss.Top, browserView, editorView, chatView)
	
	// Status bar
	status := m.renderStatusBar()
	
	// Help
	help := m.renderHelpBar()
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		panels,
		status,
		help,
	)
}

// viewHelp renders the help screen
func (m *AppModel) viewHelp() string {
	title := m.styles.App.Title.Render("⌨️ Keyboard Shortcuts")
	
	var help []string
	help = append(help, "Navigation:")
	help = append(help, "  1 / 2 / 3    Focus panel (files / editor / chat)")
	help = append(help, "  tab          Switch to next panel")
	help = append(help, "")
	help = append(help, "File Browser:")
	help = append(help, "  ↑/↓          Navigate files")
	help = append(help, "  ←/→          Collapse/expand directories")
	help = append(help, "  enter        Open file in editor")
	help = append(help, "  space        Toggle directory")
	help = append(help, "  r            Reload file tree")
	help = append(help, "")
	help = append(help, "Editor:")
	help = append(help, "  ctrl+s       Save file")
	help = append(help, "  ctrl+w       Close tab")
	help = append(help, "  ctrl+tab     Next tab")
	help = append(help, "  shift+tab    Previous tab")
	help = append(help, "  ctrl+z       Undo")
	help = append(help, "  ctrl+y       Redo")
	help = append(help, "  ctrl+l       Toggle line numbers")
	help = append(help, "  pgup/pgdown  Scroll page")
	help = append(help, "")
	help = append(help, "Chat:")
	help = append(help, "  enter        Send message")
	help = append(help, "  ctrl+u       Clear input")
	help = append(help, "  i            Insert mode")
	help = append(help, "  esc          Normal mode")
	help = append(help, "")
	help = append(help, "Changes/Diff:")
	help = append(help, "  d            Show changes/diff")
	help = append(help, "  n/j/↓        Next change")
	help = append(help, "  p/k/↑        Previous change")
	help = append(help, "  a/y/enter    Accept change")
	help = append(help, "  r/n          Reject change")
	help = append(help, "  t            Toggle view mode")
	help = append(help, "  A            Accept all")
	help = append(help, "  R            Reject all")
	help = append(help, "")
	help = append(help, "Global:")
	help = append(help, "  m            Change model")
	help = append(help, "  ?            Toggle this help")
	help = append(help, "  q/ctrl+c     Quit")
	
	content := lipgloss.JoinVertical(lipgloss.Left, help...)
	
	box := m.styles.App.Container.
		Width(m.width - 4).
		Height(m.height - 4).
		Render(content)
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		box,
	)
}

// viewDiff renders the diff viewer
func (m *AppModel) viewDiff() string {
	return m.diffViewer.View()
}

// renderStatusBar renders status bar
func (m *AppModel) renderStatusBar() string {
	var parts []string
	
	// Current model
	parts = append(parts, m.styles.StatusBar.Mode.Render(m.config.SelectedModel))
	
	// Session
	if m.config.SessionName != "" {
		parts = append(parts, m.styles.StatusBar.Info.Render(m.config.SessionName))
	}
	
	// Editor state
	if m.editor.HasTabs() {
		tabCount := m.editor.GetTabCount()
		activeTab := m.editor.GetActiveTabIndex() + 1
		parts = append(parts, m.styles.StatusBar.Info.Render(
			fmt.Sprintf("Editor: %d/%d", activeTab, tabCount),
		))
	}
	
	// Pending changes
	if m.changeManager.HasChanges() {
		pending := m.changeManager.PendingCount()
		parts = append(parts, m.styles.StatusBar.Value.Render(
			fmt.Sprintf("%d changes", pending),
		))
	}
	
	// Status message
	if m.statusMsg != "" && time.Since(m.statusTime) < 5*time.Second {
		parts = append(parts, m.styles.StatusBar.Value.Render(m.statusMsg))
	}
	
	return lipgloss.JoinHorizontal(lipgloss.Left, parts...)
}

// renderHelpBar renders the help bar
func (m *AppModel) renderHelpBar() string {
	var parts []string
	parts = append(parts, "1:files")
	parts = append(parts, "2:chat")
	parts = append(parts, "3:editor")
	parts = append(parts, "d:diff")
	parts = append(parts, "tab:switch")
	parts = append(parts, "m:model")
	parts = append(parts, "?:help")
	parts = append(parts, "q:quit")
	
	return m.styles.Help.Desc.Render(strings.Join(parts, " • "))
}

// fetchModelsCmd returns a command to fetch models from CortexBrain
func (m *AppModel) fetchModelsCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		models, err := m.cbClient.GetModels(ctx)
		if err != nil {
			return modelsFetchedMsg{error: err.Error()}
		}

		return modelsFetchedMsg{models: models}
	}
}

// modelsFetchedMsg is sent when models are fetched
type modelsFetchedMsg struct {
	models []cortexbrain.ModelInfo
	error  string
}

// RunApp runs the application
func RunApp(config AppConfig) error {
	model, err := NewAppModel(config)
	if err != nil {
		return err
	}
	
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}
	
	return nil
}
