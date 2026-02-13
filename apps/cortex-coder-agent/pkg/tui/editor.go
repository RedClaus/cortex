package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/RedClaus/cortex-coder-agent/pkg/editor"
)

// Tab represents an open file tab
type Tab struct {
	Buffer   *editor.Buffer
	Filename string
	Title    string
	Dirty    bool
}

// EditorPanel represents the code editor component
type EditorPanel struct {
	styles       Styles
	width        int
	height       int
	focused      bool
	tabs         []*Tab
	activeTab    int
	viewport     viewport.Model
	showLineNums bool
	keys         EditorKeyMap
	onFileOpen   func(string) // Callback when file is opened
	onFileClose  func(string) // Callback when file is closed
	
	// Syntax highlighting
	chromaStyle  *chroma.Style
	termProfile  termenv.Profile
}

// EditorKeyMap defines key bindings for the editor
type EditorKeyMap struct {
	Save          key.Binding
	CloseTab      key.Binding
	NextTab       key.Binding
	PrevTab       key.Binding
	Undo          key.Binding
	Redo          key.Binding
	ToggleLineNum key.Binding
	PageUp        key.Binding
	PageDown      key.Binding
	GoToLine      key.Binding
	Find          key.Binding
}

// DefaultEditorKeyMap returns default key bindings
func DefaultEditorKeyMap() EditorKeyMap {
	return EditorKeyMap{
		Save: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "save"),
		),
		CloseTab: key.NewBinding(
			key.WithKeys("ctrl+w"),
			key.WithHelp("ctrl+w", "close tab"),
		),
		NextTab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next tab"),
		),
		PrevTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "prev tab"),
		),
		Undo: key.NewBinding(
			key.WithKeys("ctrl+z"),
			key.WithHelp("ctrl+z", "undo"),
		),
		Redo: key.NewBinding(
			key.WithKeys("ctrl+y"),
			key.WithHelp("ctrl+y", "redo"),
		),
		ToggleLineNum: key.NewBinding(
			key.WithKeys("ctrl+l"),
			key.WithHelp("ctrl+l", "toggle line nums"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown"),
			key.WithHelp("pgdown", "page down"),
		),
		GoToLine: key.NewBinding(
			key.WithKeys("ctrl+g"),
			key.WithHelp("ctrl+g", "goto line"),
		),
		Find: key.NewBinding(
			key.WithKeys("ctrl+f"),
			key.WithHelp("ctrl+f", "find"),
		),
	}
}

// NewEditorPanel creates a new editor panel
func NewEditorPanel(s Styles, width, height int) *EditorPanel {
	vp := viewport.New(width-4, height-6)
	vp.Style = lipgloss.NewStyle().
		Background(lipgloss.Color(s.Colors.Background))
	
	// Initialize chroma style
	chromaStyle := styles.Get("dracula")
	if chromaStyle == nil {
		chromaStyle = styles.Fallback
	}
	
	return &EditorPanel{
		styles:       s,
		width:        width,
		height:       height,
		tabs:         make([]*Tab, 0),
		viewport:     vp,
		showLineNums: true,
		keys:         DefaultEditorKeyMap(),
		chromaStyle:  chromaStyle,
		termProfile:  termenv.ColorProfile(),
	}
}

// SetSize updates dimensions
func (ep *EditorPanel) SetSize(width, height int) {
	ep.width = width
	ep.height = height
	
	contentWidth := width - 4
	if ep.showLineNums {
		contentWidth -= 6 // Space for line numbers
	}
	
	ep.viewport.Width = contentWidth
	ep.viewport.Height = height - 6 // Account for tabs and borders
	ep.refreshViewport()
}

// Focus activates the panel
func (ep *EditorPanel) Focus() {
	ep.focused = true
}

// Blur deactivates the panel
func (ep *EditorPanel) Blur() {
	ep.focused = false
}

// IsFocused returns focus state
func (ep *EditorPanel) IsFocused() bool {
	return ep.focused
}

// SetOnFileOpen sets the file open callback
func (ep *EditorPanel) SetOnFileOpen(fn func(string)) {
	ep.onFileOpen = fn
}

// SetOnFileClose sets the file close callback
func (ep *EditorPanel) SetOnFileClose(fn func(string)) {
	ep.onFileClose = fn
}

// OpenFile opens a file in a new tab
func (ep *EditorPanel) OpenFile(filename string) error {
	// Check if already open
	for i, tab := range ep.tabs {
		if tab.Filename == filename {
			ep.activeTab = i
			ep.refreshViewport()
			return nil
		}
	}
	
	// Load file into buffer
	buffer := editor.NewBuffer()
	if err := buffer.LoadFromFile(filename); err != nil {
		return fmt.Errorf("failed to load file: %w", err)
	}
	
	// Create tab
	tab := &Tab{
		Buffer:   buffer,
		Filename: filename,
		Title:    filepath.Base(filename),
		Dirty:    false,
	}
	
	ep.tabs = append(ep.tabs, tab)
	ep.activeTab = len(ep.tabs) - 1
	ep.refreshViewport()
	
	if ep.onFileOpen != nil {
		ep.onFileOpen(filename)
	}
	
	return nil
}

// OpenFileWithContent opens a file with given content (for diffs/AI suggestions)
func (ep *EditorPanel) OpenFileWithContent(filename, content string) error {
	// Check if already open
	for i, tab := range ep.tabs {
		if tab.Filename == filename {
			tab.Buffer.SetContent(content)
			tab.Dirty = true
			ep.activeTab = i
			ep.refreshViewport()
			return nil
		}
	}
	
	// Create buffer with content
	buffer := editor.NewBuffer()
	buffer.SetFilename(filename)
	buffer.SetContent(content)
	
	tab := &Tab{
		Buffer:   buffer,
		Filename: filename,
		Title:    filepath.Base(filename) + "*",
		Dirty:    true,
	}
	
	ep.tabs = append(ep.tabs, tab)
	ep.activeTab = len(ep.tabs) - 1
	ep.refreshViewport()
	
	return nil
}

// CloseTab closes the current tab
func (ep *EditorPanel) CloseTab() {
	if len(ep.tabs) == 0 {
		return
	}
	
	filename := ep.tabs[ep.activeTab].Filename
	
	// Remove tab
	ep.tabs = append(ep.tabs[:ep.activeTab], ep.tabs[ep.activeTab+1:]...)
	
	// Adjust active tab
	if ep.activeTab >= len(ep.tabs) {
		ep.activeTab = len(ep.tabs) - 1
	}
	if ep.activeTab < 0 {
		ep.activeTab = 0
	}
	
	ep.refreshViewport()
	
	if ep.onFileClose != nil {
		ep.onFileClose(filename)
	}
}

// CloseTabByIndex closes a specific tab
func (ep *EditorPanel) CloseTabByIndex(index int) {
	if index < 0 || index >= len(ep.tabs) {
		return
	}
	
	filename := ep.tabs[index].Filename
	ep.tabs = append(ep.tabs[:index], ep.tabs[index+1:]...)
	
	if ep.activeTab >= len(ep.tabs) {
		ep.activeTab = len(ep.tabs) - 1
	}
	if ep.activeTab < 0 {
		ep.activeTab = 0
	}
	
	ep.refreshViewport()
	
	if ep.onFileClose != nil {
		ep.onFileClose(filename)
	}
}

// NextTab switches to the next tab
func (ep *EditorPanel) NextTab() {
	if len(ep.tabs) == 0 {
		return
	}
	ep.activeTab = (ep.activeTab + 1) % len(ep.tabs)
	ep.refreshViewport()
}

// PrevTab switches to the previous tab
func (ep *EditorPanel) PrevTab() {
	if len(ep.tabs) == 0 {
		return
	}
	ep.activeTab--
	if ep.activeTab < 0 {
		ep.activeTab = len(ep.tabs) - 1
	}
	ep.refreshViewport()
}

// SaveCurrent saves the current tab
func (ep *EditorPanel) SaveCurrent() error {
	if len(ep.tabs) == 0 {
		return nil
	}
	
	tab := ep.tabs[ep.activeTab]
	if err := tab.Buffer.Save(); err != nil {
		return err
	}
	
	tab.Dirty = false
	tab.Title = filepath.Base(tab.Filename)
	ep.refreshViewport()
	
	return nil
}

// HasTabs returns true if there are open tabs
func (ep *EditorPanel) HasTabs() bool {
	return len(ep.tabs) > 0
}

// GetCurrentTab returns the current tab
func (ep *EditorPanel) GetCurrentTab() *Tab {
	if len(ep.tabs) == 0 || ep.activeTab >= len(ep.tabs) {
		return nil
	}
	return ep.tabs[ep.activeTab]
}

// refreshViewport updates the viewport content
func (ep *EditorPanel) refreshViewport() {
	if len(ep.tabs) == 0 {
		ep.viewport.SetContent("No file open")
		return
	}
	
	tab := ep.tabs[ep.activeTab]
	buffer := tab.Buffer
	lines := buffer.GetLines()
	
	var content strings.Builder
	
	for i, line := range lines {
		// Line number
		if ep.showLineNums {
			lineNumStr := fmt.Sprintf("%4d │ ", i+1)
			content.WriteString(ep.styles.Editor.LineNumber.Render(lineNumStr))
		}
		
		// Syntax highlighted content
		highlighted := ep.highlightLine(line)
		content.WriteString(highlighted)
		content.WriteString("\n")
	}
	
	ep.viewport.SetContent(content.String())
}

// highlightLine applies syntax highlighting to a line
func (ep *EditorPanel) highlightLine(line string) string {
	if line == "" {
		return ""
	}
	
	tab := ep.GetCurrentTab()
	if tab == nil {
		return ep.styles.Editor.Content.Render(line)
	}
	
	// Get lexer based on file extension
	lexer := lexers.Match(tab.Filename)
	if lexer == nil {
		return ep.styles.Editor.Content.Render(line)
	}
	
	// Tokenize the line
	iterator, err := lexer.Tokenise(nil, line)
	if err != nil {
		return ep.styles.Editor.Content.Render(line)
	}
	
	// Format tokens
	var result strings.Builder
	for _, token := range iterator.Tokens() {
		style := ep.chromaStyle.Get(token.Type)
		colored := ep.formatToken(token.Value, style)
		result.WriteString(colored)
	}
	
	return result.String()
}

// formatToken formats a token with the appropriate style
func (ep *EditorPanel) formatToken(value string, style chroma.StyleEntry) string {
	// Convert chroma style to lipgloss
	var lipStyle lipgloss.Style
	
	// Map chroma colors to lipgloss
	if style.Colour.IsSet() {
		color := style.Colour.String()
		lipStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	} else {
		lipStyle = ep.styles.Editor.Content
	}
	
	if style.Bold == chroma.Yes {
		lipStyle = lipgloss.NewStyle().Bold(true)
	}
	if style.Italic == chroma.Yes {
		lipStyle = lipStyle.Italic(true)
	}
	
	return lipStyle.Render(value)
}

// Update handles messages
func (ep *EditorPanel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return ep.handleKeyMsg(msg)
	case tea.WindowSizeMsg:
		ep.SetSize(msg.Width, msg.Height)
	}
	
	// Update viewport
	var cmd tea.Cmd
	ep.viewport, cmd = ep.viewport.Update(msg)
	return cmd
}

// handleKeyMsg handles key messages
func (ep *EditorPanel) handleKeyMsg(msg tea.KeyMsg) tea.Cmd {
	if !ep.focused {
		return nil
	}
	
	switch {
	case key.Matches(msg, ep.keys.Save):
		if err := ep.SaveCurrent(); err != nil {
			// TODO: Show error
		}
		return nil
		
	case key.Matches(msg, ep.keys.CloseTab):
		ep.CloseTab()
		return nil
		
	case key.Matches(msg, ep.keys.NextTab):
		ep.NextTab()
		return nil
		
	case key.Matches(msg, ep.keys.PrevTab):
		ep.PrevTab()
		return nil
		
	case key.Matches(msg, ep.keys.ToggleLineNum):
		ep.showLineNums = !ep.showLineNums
		ep.SetSize(ep.width, ep.height)
		return nil
		
	case key.Matches(msg, ep.keys.PageUp):
		ep.viewport.LineUp(ep.viewport.Height / 2)
		return nil
		
	case key.Matches(msg, ep.keys.PageDown):
		ep.viewport.LineDown(ep.viewport.Height / 2)
		return nil
	}
	
	// Pass to viewport for scrolling
	var cmd tea.Cmd
	ep.viewport, cmd = ep.viewport.Update(msg)
	return cmd
}

// View renders the editor panel
func (ep *EditorPanel) View() string {
	if len(ep.tabs) == 0 {
		borderStyle := ep.styles.App.PanelBorder
		if ep.focused {
			borderStyle = ep.styles.App.FocusedBorder
		}
		
		content := lipgloss.JoinVertical(
			lipgloss.Center,
			"",
			"",
			ep.styles.App.Title.Render("📄 Editor"),
			"",
			ep.styles.Help.Desc.Render("No file open"),
			"",
			ep.styles.Help.Desc.Render("Press Enter on a file in the browser to open"),
		)
		
		return borderStyle.
			Width(ep.width - 2).
			Height(ep.height - 2).
			Render(content)
	}
	
	var sections []string
	
	// Tab bar
	tabBar := ep.renderTabBar()
	sections = append(sections, tabBar)
	
	// Editor content
	content := ep.renderContent()
	sections = append(sections, content)
	
	// Status bar
	status := ep.renderStatusBar()
	sections = append(sections, status)
	
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderTabBar renders the tab bar
func (ep *EditorPanel) renderTabBar() string {
	var tabs []string
	
	for i, tab := range ep.tabs {
		title := tab.Title
		if tab.Dirty {
			title = title + " ●"
		}
		
		var style lipgloss.Style
		if i == ep.activeTab {
			style = lipgloss.NewStyle().
				Background(lipgloss.Color(ep.styles.Colors.CurrentLine)).
				Foreground(lipgloss.Color(ep.styles.Colors.Foreground)).
				Bold(true)
		} else {
			style = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ep.styles.Colors.Comment))
		}
		
		tabStr := style.Render(" " + title + " ")
		tabs = append(tabs, tabStr)
	}
	
	return lipgloss.JoinHorizontal(lipgloss.Left, tabs...)
}

// renderContent renders the editor content
func (ep *EditorPanel) renderContent() string {
	borderStyle := ep.styles.App.PanelBorder
	if ep.focused {
		borderStyle = ep.styles.App.FocusedBorder
	}
	
	return borderStyle.
		Width(ep.width - 2).
		Height(ep.height - 6).
		Render(ep.viewport.View())
}

// renderStatusBar renders the editor status bar
func (ep *EditorPanel) renderStatusBar() string {
	if len(ep.tabs) == 0 {
		return ""
	}
	
	tab := ep.tabs[ep.activeTab]
	buffer := tab.Buffer
	
	line, col := buffer.GetCursor()
	
	var parts []string
	
	// File info
	parts = append(parts, ep.styles.StatusBar.Mode.Render(fmt.Sprintf(" %s ", tab.Title)))
	
	// Position
	parts = append(parts, ep.styles.StatusBar.Info.Render(
		fmt.Sprintf("Ln %d, Col %d", line+1, col+1),
	))
	
	// Dirty indicator
	if tab.Dirty {
		parts = append(parts, ep.styles.StatusBar.Value.Render("[+]"))
	}
	
	// Line count
	parts = append(parts, ep.styles.Help.Desc.Render(
		fmt.Sprintf("(%d lines)", buffer.LineCount()),
	))
	
	return lipgloss.JoinHorizontal(lipgloss.Left, parts...)
}

// GetTabCount returns the number of open tabs
func (ep *EditorPanel) GetTabCount() int {
	return len(ep.tabs)
}

// GetActiveTabIndex returns the active tab index
func (ep *EditorPanel) GetActiveTabIndex() int {
	return ep.activeTab
}

// Tab represents an open file tab
type TabInfo struct {
	Title    string
	Filename string
	Dirty    bool
}

// GetTabs returns information about all tabs
func (ep *EditorPanel) GetTabs() []TabInfo {
	infos := make([]TabInfo, len(ep.tabs))
	for i, tab := range ep.tabs {
		infos[i] = TabInfo{
			Title:    tab.Title,
			Filename: tab.Filename,
			Dirty:    tab.Dirty,
		}
	}
	return infos
}

// SetActiveTab sets the active tab by index
func (ep *EditorPanel) SetActiveTab(index int) {
	if index >= 0 && index < len(ep.tabs) {
		ep.activeTab = index
		ep.refreshViewport()
	}
}
