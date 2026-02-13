package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ChangeType represents the type of change in a diff
type ChangeType int

const (
	ChangeTypeUnchanged ChangeType = iota
	ChangeTypeAdded
	ChangeTypeDeleted
	ChangeTypeModified
)

// DiffLine represents a single line in a diff
type DiffLine struct {
	Type       ChangeType
	LeftLine   int  // Line number in original (0 if doesn't exist)
	RightLine  int  // Line number in modified (0 if doesn't exist)
	Content    string
	Selected   bool // For navigation
}

// DiffHunk represents a section of changes
type DiffHunk struct {
	StartLeft   int
	StartRight  int
	Lines      []DiffLine
	Header     string
}

// Diff represents a complete diff with hunks
type Diff struct {
	Filename   string
	Hunks      []DiffHunk
	OrigLines  []string
	ModLines   []string
}

// DiffViewer represents the diff viewer component
type DiffViewer struct {
	styles      Styles
	width       int
	height      int
	focused     bool
	diff        *Diff
	viewMode    DiffViewMode
	viewport    viewport.Model
	currentHunk int
	currentLine int
	keys        DiffKeyMap
	onAccept    func(*DiffHunk) // Callback when change is accepted
	onReject    func(*DiffHunk) // Callback when change is rejected
	onClose     func()          // Callback when diff view is closed
}

// DiffViewMode represents how to display the diff
type DiffViewMode int

const (
	DiffViewModeSideBySide DiffViewMode = iota
	DiffViewModeInline
)

// DiffKeyMap defines key bindings for the diff viewer
type DiffKeyMap struct {
	NextChange    key.Binding
	PrevChange    key.Binding
	JumpChange    key.Binding
	Accept        key.Binding
	Reject        key.Binding
	ToggleMode    key.Binding
	Refresh       key.Binding
	Close         key.Binding
	ScrollUp      key.Binding
	ScrollDown    key.Binding
	PageUp        key.Binding
	PageDown      key.Binding
}

// DefaultDiffKeyMap returns default key bindings
func DefaultDiffKeyMap() DiffKeyMap {
	return DiffKeyMap{
		NextChange: key.NewBinding(
			key.WithKeys("n", "j", "down"),
			key.WithHelp("n/j/↓", "next change"),
		),
		PrevChange: key.NewBinding(
			key.WithKeys("p", "k", "up"),
			key.WithHelp("p/k/↑", "prev change"),
		),
		JumpChange: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "jump to change"),
		),
		Accept: key.NewBinding(
			key.WithKeys("a", "y"),
			key.WithHelp("a/y", "accept change"),
		),
		Reject: key.NewBinding(
			key.WithKeys("r", "n"),
			key.WithHelp("r/n", "reject change"),
		),
		ToggleMode: key.NewBinding(
			key.WithKeys("t", "ctrl+h"),
			key.WithHelp("t", "toggle view mode"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("ctrl+r", "refresh diff"),
		),
		Close: key.NewBinding(
			key.WithKeys("esc", "q"),
			key.WithHelp("esc/q", "close"),
		),
		ScrollUp: key.NewBinding(
			key.WithKeys("pgup"),
			key.WithHelp("pgup", "page up"),
		),
		ScrollDown: key.NewBinding(
			key.WithKeys("pgdown"),
			key.WithHelp("pgdown", "page down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("alt+up"),
			key.WithHelp("alt+↑", "scroll up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("alt+down"),
			key.WithHelp("alt+↓", "scroll down"),
		),
	}
}

// NewDiffViewer creates a new diff viewer
func NewDiffViewer(styles Styles, width, height int) *DiffViewer {
	vp := viewport.New(width-4, height-8)
	vp.Style = lipgloss.NewStyle().
		Background(lipgloss.Color(styles.Colors.Background))
	
	return &DiffViewer{
		styles:     styles,
		width:      width,
		height:     height,
		viewMode:   DiffViewModeSideBySide,
		viewport:   vp,
		currentHunk: 0,
		currentLine: 0,
		keys:       DefaultDiffKeyMap(),
	}
}

// SetSize updates dimensions
func (dv *DiffViewer) SetSize(width, height int) {
	dv.width = width
	dv.height = height
	
	if dv.viewMode == DiffViewModeSideBySide {
		dv.viewport.Width = width - 6
	} else {
		dv.viewport.Width = width - 4
	}
	dv.viewport.Height = height - 10
	dv.refreshViewport()
}

// Focus activates the viewer
func (dv *DiffViewer) Focus() {
	dv.focused = true
}

// Blur deactivates the viewer
func (dv *DiffViewer) Blur() {
	dv.focused = false
}

// IsFocused returns focus state
func (dv *DiffViewer) IsFocused() bool {
	return dv.focused
}

// SetDiff sets the diff to display
func (dv *DiffViewer) SetDiff(diff *Diff) {
	dv.diff = diff
	dv.currentHunk = 0
	dv.currentLine = 0
	dv.refreshViewport()
}

// SetOnAccept sets the accept callback
func (dv *DiffViewer) SetOnAccept(fn func(*DiffHunk)) {
	dv.onAccept = fn
}

// SetOnReject sets the reject callback
func (dv *DiffViewer) SetOnReject(fn func(*DiffHunk)) {
	dv.onReject = fn
}

// SetOnClose sets the close callback
func (dv *DiffViewer) SetOnClose(fn func()) {
	dv.onClose = fn
}

// ToggleViewMode toggles between side-by-side and inline views
func (dv *DiffViewer) ToggleViewMode() {
	if dv.viewMode == DiffViewModeSideBySide {
		dv.viewMode = DiffViewModeInline
		dv.viewport.Width = dv.width - 4
	} else {
		dv.viewMode = DiffViewModeSideBySide
		dv.viewport.Width = dv.width - 6
	}
	dv.refreshViewport()
}

// refreshViewport updates the viewport content
func (dv *DiffViewer) refreshViewport() {
	if dv.diff == nil {
		dv.viewport.SetContent("No diff to display")
		return
	}
	
	var content strings.Builder
	
	switch dv.viewMode {
	case DiffViewModeSideBySide:
		content.WriteString(dv.renderSideBySide())
	case DiffViewModeInline:
		content.WriteString(dv.renderInline())
	}
	
	dv.viewport.SetContent(content.String())
}

// renderSideBySide renders the diff in side-by-side mode
func (dv *DiffViewer) renderSideBySide() string {
	var lines []string
	
	// Header
	header := dv.styles.App.Title.Render("📋 Diff: " + dv.diff.Filename)
	lines = append(lines, header)
	lines = append(lines, "")
	
	// Mode indicator
	modeText := "Side-by-Side"
	if dv.viewMode == DiffViewModeInline {
		modeText = "Inline"
	}
	lines = append(lines, dv.styles.Help.Desc.Render(fmt.Sprintf("[%s] n:next p:prev a:accept r:reject enter:jump t:toggle q:close", modeText)))
	lines = append(lines, "")
	
	// Column headers
	leftHeader := dv.styles.StatusBar.Mode.Render(" Original ")
	rightHeader := dv.styles.StatusBar.Mode.Render(" Modified ")
	lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left, leftHeader, rightHeader))
	
	for _, hunk := range dv.diff.Hunks {
		// Hunk header
		hunkHeader := dv.styles.Help.Desc.Render(fmt.Sprintf("@@ -%d +%d @@", hunk.StartLeft, hunk.StartRight))
		lines = append(lines, hunkHeader)
		
		for _, line := range hunk.Lines {
			leftStr, rightStr := dv.renderDiffLine(line)
			
			// Highlight selected line
			if line.Selected {
				leftStr = dv.styles.Browser.Selected.Render(leftStr)
				rightStr = dv.styles.Browser.Selected.Render(rightStr)
			}
			
			lines = append(lines, lipgloss.JoinHorizontal(
				lipgloss.Left,
				leftStr,
				rightStr,
			))
		}
		
		lines = append(lines, "") // Spacer between hunks
	}
	
	return strings.Join(lines, "\n")
}

// renderInline renders the diff in inline mode
func (dv *DiffViewer) renderInline() string {
	var lines []string
	
	// Header
	header := dv.styles.App.Title.Render("📋 Diff: " + dv.diff.Filename)
	lines = append(lines, header)
	lines = append(lines, "")
	
	// Mode indicator
	modeText := "Inline"
	lines = append(lines, dv.styles.Help.Desc.Render(fmt.Sprintf("[%s] n:next p:prev a:accept r:reject enter:jump t:toggle q:close", modeText)))
	lines = append(lines, "")
	
	for _, hunk := range dv.diff.Hunks {
		// Hunk header
		hunkHeader := dv.styles.Help.Desc.Render(fmt.Sprintf("@@ -%d +%d @@", hunk.StartLeft, hunk.StartRight))
		lines = append(lines, hunkHeader)
		
		for _, line := range hunk.Lines {
			styled := dv.renderInlineLine(line)
			lines = append(lines, styled)
		}
		
		lines = append(lines, "") // Spacer between hunks
	}
	
	return strings.Join(lines, "\n")
}

// renderDiffLine renders a single diff line for side-by-side view
func (dv *DiffViewer) renderDiffLine(line DiffLine) (left, right string) {
	switch line.Type {
	case ChangeTypeUnchanged:
		leftNum := fmt.Sprintf("%4d", line.LeftLine)
		rightNum := fmt.Sprintf("%4d", line.RightLine)
		left = dv.styles.Editor.LineNumber.Render(leftNum + "  " + line.Content)
		right = dv.styles.Editor.LineNumber.Render(rightNum + "  " + line.Content)
		
	case ChangeTypeAdded:
		rightNum := fmt.Sprintf("%4d", line.RightLine)
		left = dv.styles.Editor.LineNumber.Render("     ")
		right = dv.styles.StatusBar.Success.Render("+   ") + dv.styles.Editor.Content.Render(line.Content)
		_ = rightNum // unused but kept for consistency
		
	case ChangeTypeDeleted:
		leftNum := fmt.Sprintf("%4d", line.LeftLine)
		left = dv.styles.StatusBar.Value.Render("-   ") + dv.styles.Editor.Content.Render(line.Content)
		right = dv.styles.Editor.LineNumber.Render("     ")
		_ = leftNum // unused but kept for consistency
		
	case ChangeTypeModified:
		leftNum := fmt.Sprintf("%4d", line.LeftLine)
		rightNum := fmt.Sprintf("%4d", line.RightLine)
		left = dv.styles.StatusBar.Warning.Render("~   ") + dv.styles.Editor.Content.Render(line.Content)
		right = dv.styles.StatusBar.Warning.Render("~   ") + dv.styles.Editor.Content.Render(line.Content)
		_ = leftNum
		_ = rightNum
	}
	
	return left, right
}

// renderInlineLine renders a single diff line for inline view
func (dv *DiffViewer) renderInlineLine(line DiffLine) string {
	switch line.Type {
	case ChangeTypeUnchanged:
		return dv.styles.Editor.LineNumber.Render(fmt.Sprintf("%4d|%4d  ", line.LeftLine, line.RightLine)) + line.Content
		
	case ChangeTypeAdded:
		prefix := dv.styles.StatusBar.Success.Render("+")
		return dv.styles.Editor.LineNumber.Render(fmt.Sprintf("   |%4d  ", line.RightLine)) + prefix + line.Content
		
	case ChangeTypeDeleted:
		prefix := dv.styles.StatusBar.Value.Render("-")
		return dv.styles.Editor.LineNumber.Render(fmt.Sprintf("%4d|    ", line.LeftLine)) + prefix + line.Content
		
	case ChangeTypeModified:
		prefix := dv.styles.StatusBar.Warning.Render("~")
		return dv.styles.Editor.LineNumber.Render(fmt.Sprintf("%4d|%4d  ", line.LeftLine, line.RightLine)) + prefix + line.Content
	}
	
	return ""
}

// NextChange moves to the next change
func (dv *DiffViewer) NextChange() {
	if dv.diff == nil || len(dv.diff.Hunks) == 0 {
		return
	}
	
	// Find next change in current hunk
	if dv.currentLine < len(dv.diff.Hunks[dv.currentHunk].Lines)-1 {
		// Clear current selection
		if dv.currentLine < len(dv.diff.Hunks[dv.currentHunk].Lines) {
			dv.diff.Hunks[dv.currentHunk].Lines[dv.currentLine].Selected = false
		}
		dv.currentLine++
		dv.diff.Hunks[dv.currentHunk].Lines[dv.currentLine].Selected = true
	} else if dv.currentHunk < len(dv.diff.Hunks)-1 {
		// Move to next hunk
		if dv.currentLine < len(dv.diff.Hunks[dv.currentHunk].Lines) {
			dv.diff.Hunks[dv.currentHunk].Lines[dv.currentLine].Selected = false
		}
		dv.currentHunk++
		dv.currentLine = 0
		
		// Find first change in new hunk
		for i, line := range dv.diff.Hunks[dv.currentHunk].Lines {
			if line.Type != ChangeTypeUnchanged {
				line.Selected = true
				dv.currentLine = i
				break
			}
		}
	}
	
	dv.refreshViewport()
}

// PrevChange moves to the previous change
func (dv *DiffViewer) PrevChange() {
	if dv.diff == nil || len(dv.diff.Hunks) == 0 {
		return
	}
	
	// Find previous change in current hunk
	if dv.currentLine > 0 {
		// Clear current selection
		dv.diff.Hunks[dv.currentHunk].Lines[dv.currentLine].Selected = false
		dv.currentLine--
		dv.diff.Hunks[dv.currentHunk].Lines[dv.currentLine].Selected = true
	} else if dv.currentHunk > 0 {
		// Move to previous hunk
		dv.diff.Hunks[dv.currentHunk].Lines[dv.currentLine].Selected = false
		dv.currentHunk--
		
		// Find last change in previous hunk
		hunk := dv.diff.Hunks[dv.currentHunk]
		for i := len(hunk.Lines) - 1; i >= 0; i-- {
			if hunk.Lines[i].Type != ChangeTypeUnchanged {
				hunk.Lines[i].Selected = true
				dv.currentLine = i
				break
			}
		}
	}
	
	dv.refreshViewport()
}

// JumpToChange jumps to the currently selected change
func (dv *DiffViewer) JumpToChange() {
	if dv.diff == nil {
		return
	}
	
	// The current position is already selected, so just ensure it's visible
	// The viewport should scroll to show it
	dv.refreshViewport()
}

// GetCurrentHunk returns the currently selected hunk
func (dv *DiffViewer) GetCurrentHunk() *DiffHunk {
	if dv.diff == nil || dv.currentHunk >= len(dv.diff.Hunks) {
		return nil
	}
	return &dv.diff.Hunks[dv.currentHunk]
}

// AcceptCurrent accepts the current change
func (dv *DiffViewer) AcceptCurrent() {
	hunk := dv.GetCurrentHunk()
	if hunk != nil && dv.onAccept != nil {
		dv.onAccept(hunk)
	}
}

// RejectCurrent rejects the current change
func (dv *DiffViewer) RejectCurrent() {
	hunk := dv.GetCurrentHunk()
	if hunk != nil && dv.onReject != nil {
		dv.onReject(hunk)
	}
}

// Close closes the diff viewer
func (dv *DiffViewer) Close() {
	if dv.onClose != nil {
		dv.onClose()
	}
}

// GetChangeCount returns the total number of changes
func (dv *DiffViewer) GetChangeCount() int {
	if dv.diff == nil {
		return 0
	}
	
	count := 0
	for _, hunk := range dv.diff.Hunks {
		for _, line := range hunk.Lines {
			if line.Type != ChangeTypeUnchanged {
				count++
			}
		}
	}
	return count
}

// Update handles messages
func (dv *DiffViewer) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return dv.handleKeyMsg(msg)
	case tea.WindowSizeMsg:
		dv.SetSize(msg.Width, msg.Height)
	}
	
	// Update viewport
	var cmd tea.Cmd
	dv.viewport, cmd = dv.viewport.Update(msg)
	return cmd
}

// handleKeyMsg handles key messages
func (dv *DiffViewer) handleKeyMsg(msg tea.KeyMsg) tea.Cmd {
	if !dv.focused {
		return nil
	}
	
	switch {
	case key.Matches(msg, dv.keys.NextChange):
		dv.NextChange()
		
	case key.Matches(msg, dv.keys.PrevChange):
		dv.PrevChange()
		
	case key.Matches(msg, dv.keys.JumpChange):
		dv.JumpToChange()
		
	case key.Matches(msg, dv.keys.Accept):
		dv.AcceptCurrent()
		
	case key.Matches(msg, dv.keys.Reject):
		dv.RejectCurrent()
		
	case key.Matches(msg, dv.keys.ToggleMode):
		dv.ToggleViewMode()
		
	case key.Matches(msg, dv.keys.Refresh):
		dv.refreshViewport()
		
	case key.Matches(msg, dv.keys.Close):
		dv.Close()
		
	case key.Matches(msg, dv.keys.ScrollUp):
		dv.viewport.LineUp(1)
		
	case key.Matches(msg, dv.keys.ScrollDown):
		dv.viewport.LineDown(1)
		
	case key.Matches(msg, dv.keys.PageUp):
		dv.viewport.PageUp()
		
	case key.Matches(msg, dv.keys.PageDown):
		dv.viewport.PageDown()
	}
	
	return nil
}

// View renders the diff viewer
func (dv *DiffViewer) View() string {
	borderStyle := dv.styles.App.PanelBorder
	if dv.focused {
		borderStyle = dv.styles.App.FocusedBorder
	}
	
	content := dv.viewport.View()
	
	return borderStyle.
		Width(dv.width - 2).
		Height(dv.height - 2).
		Render(content)
}

// GetViewMode returns the current view mode
func (dv *DiffViewer) GetViewMode() DiffViewMode {
	return dv.viewMode
}

// HasDiff returns true if a diff is loaded
func (dv *DiffViewer) HasDiff() bool {
	return dv.diff != nil
}
