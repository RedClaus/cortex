package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ChangeStatus represents the status of a change
type ChangeStatus int

const (
	ChangeStatusPending ChangeStatus = iota
	ChangeStatusAccepted
	ChangeStatusRejected
)

// SuggestedChange represents an AI-suggested change
type SuggestedChange struct {
	ID          string
	Filename    string
	Description string
	Original    string
	Modified    string
	StartLine   int
	EndLine     int
	Status      ChangeStatus
	Diff        *Diff
}

// ChangeManager manages AI-suggested changes
type ChangeManager struct {
	styles       Styles
	width        int
	height       int
	focused      bool
	changes      []*SuggestedChange
	list         list.Model
	viewport     viewport.Model
	selectedIdx  int
	keys         ChangeKeyMap
	onApply      func(*SuggestedChange) error   // Callback to apply change
	onPreview    func(*SuggestedChange)        // Callback to preview change
	onAccept     func(*SuggestedChange)         // Callback when change is accepted
	onReject     func(*SuggestedChange)         // Callback when change is rejected
	onClose      func()                        // Callback when manager is closed
	editor       *EditorPanel                   // Reference to editor for preview
}

// ChangeKeyMap defines key bindings for change manager
type ChangeKeyMap struct {
	Accept       key.Binding
	Reject       key.Binding
	Preview      key.Binding
	NextChange   key.Binding
	PrevChange   key.Binding
	ApplyAll     key.Binding
	RejectAll    key.Binding
	Close        key.Binding
	ScrollUp     key.Binding
	ScrollDown   key.Binding
	PageUp       key.Binding
	PageDown     key.Binding
}

// DefaultChangeKeyMap returns default key bindings
func DefaultChangeKeyMap() ChangeKeyMap {
	return ChangeKeyMap{
		Accept: key.NewBinding(
			key.WithKeys("a", "y", "enter"),
			key.WithHelp("a/y/enter", "accept"),
		),
		Reject: key.NewBinding(
			key.WithKeys("r", "n"),
			key.WithHelp("r/n", "reject"),
		),
		Preview: key.NewBinding(
			key.WithKeys("p", "ctrl+o"),
			key.WithHelp("p", "preview"),
		),
		NextChange: key.NewBinding(
			key.WithKeys("j", "down", "n"),
			key.WithHelp("j/↓/n", "next"),
		),
		PrevChange: key.NewBinding(
			key.WithKeys("k", "up", "p"),
			key.WithHelp("k/↑/p", "prev"),
		),
		ApplyAll: key.NewBinding(
			key.WithKeys("A", "ctrl+a"),
			key.WithHelp("A", "accept all"),
		),
		RejectAll: key.NewBinding(
			key.WithKeys("R", "ctrl+r"),
			key.WithHelp("R", "reject all"),
		),
		Close: key.NewBinding(
			key.WithKeys("esc", "q", "ctrl+c"),
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

// NewChangeManager creates a new change manager
func NewChangeManager(styles Styles, width, height int) *ChangeManager {
	// Create list for changes
	items := make([]list.Item, 0)
	l := list.New(items, list.NewDefaultDelegate(), width-4, height-12)
	l.Title = ""
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetSize(width-4, height-12)
	
	vp := viewport.New(width-4, height/2)
	vp.Style = lipgloss.NewStyle().
		Background(lipgloss.Color(styles.Colors.Background))
	
	return &ChangeManager{
		styles:      styles,
		width:       width,
		height:      height,
		changes:     make([]*SuggestedChange, 0),
		list:        l,
		viewport:    vp,
		selectedIdx: 0,
		keys:        DefaultChangeKeyMap(),
	}
}

// SetSize updates dimensions
func (cm *ChangeManager) SetSize(width, height int) {
	cm.width = width
	cm.height = height
	
	cm.list.SetWidth(width - 4)
	cm.list.SetHeight(height - 12)
	
	cm.viewport.Width = width - 4
	cm.viewport.Height = height / 2
	cm.refreshViewport()
}

// Focus activates the manager
func (cm *ChangeManager) Focus() {
	cm.focused = true
}

// Blur deactivates the manager
func (cm *ChangeManager) Blur() {
	cm.focused = false
}

// IsFocused returns focus state
func (cm *ChangeManager) IsFocused() bool {
	return cm.focused
}

// SetOnApply sets the apply callback
func (cm *ChangeManager) SetOnApply(fn func(*SuggestedChange) error) {
	cm.onApply = fn
}

// SetOnPreview sets the preview callback
func (cm *ChangeManager) SetOnPreview(fn func(*SuggestedChange)) {
	cm.onPreview = fn
}

// SetOnAccept sets the accept callback
func (cm *ChangeManager) SetOnAccept(fn func(*SuggestedChange)) {
	cm.onAccept = fn
}

// SetOnReject sets the reject callback
func (cm *ChangeManager) SetOnReject(fn func(*SuggestedChange)) {
	cm.onReject = fn
}

// SetOnClose sets the close callback
func (cm *ChangeManager) SetOnClose(fn func()) {
	cm.onClose = fn
}

// SetEditor sets the editor reference for preview
func (cm *ChangeManager) SetEditor(editor *EditorPanel) {
	cm.editor = editor
}

// AddChange adds a new suggested change
func (cm *ChangeManager) AddChange(change *SuggestedChange) {
	cm.changes = append(cm.changes, change)
	cm.updateList()
}

// AddChanges adds multiple suggested changes
func (cm *ChangeManager) AddChanges(changes []*SuggestedChange) {
	cm.changes = append(cm.changes, changes...)
	cm.updateList()
}

// RemoveChange removes a change by ID
func (cm *ChangeManager) RemoveChange(id string) {
	for i, change := range cm.changes {
		if change.ID == id {
			cm.changes = append(cm.changes[:i], cm.changes[i+1:]...)
			cm.updateList()
			return
		}
	}
}

// Clear removes all changes
func (cm *ChangeManager) Clear() {
	cm.changes = make([]*SuggestedChange, 0)
	cm.selectedIdx = 0
	cm.updateList()
}

// updateList updates the list model with current changes
func (cm *ChangeManager) updateList() {
	items := make([]list.Item, 0, len(cm.changes))
	
	for _, change := range cm.changes {
		var statusIcon string
		switch change.Status {
		case ChangeStatusPending:
			statusIcon = "○"
		case ChangeStatusAccepted:
			statusIcon = "✓"
		case ChangeStatusRejected:
			statusIcon = "✗"
		}
		
		title := fmt.Sprintf("%s %s", statusIcon, change.Description)
		desc := fmt.Sprintf("  %s", change.Filename)
		
		items = append(items, changeItem{
			title:     title,
			desc:      desc,
			changeID:  change.ID,
			status:    change.Status,
		})
	}
	
	cm.list.SetItems(items)
	cm.refreshViewport()
}

// GetCurrentChange returns the currently selected change
func (cm *ChangeManager) GetCurrentChange() *SuggestedChange {
	if cm.selectedIdx < 0 || cm.selectedIdx >= len(cm.changes) {
		return nil
	}
	return cm.changes[cm.selectedIdx]
}

// GetChanges returns all changes
func (cm *ChangeManager) GetChanges() []*SuggestedChange {
	return cm.changes
}

// GetPendingChanges returns only pending changes
func (cm *ChangeManager) GetPendingChanges() []*SuggestedChange {
	pending := make([]*SuggestedChange, 0)
	for _, change := range cm.changes {
		if change.Status == ChangeStatusPending {
			pending = append(pending, change)
		}
	}
	return pending
}

// AcceptCurrent accepts the current change
func (cm *ChangeManager) AcceptCurrent() {
	change := cm.GetCurrentChange()
	if change == nil {
		return
	}
	
	change.Status = ChangeStatusAccepted
	cm.updateList()
	
	if cm.onAccept != nil {
		cm.onAccept(change)
	}
	
	// Auto-apply if configured
	if cm.onApply != nil {
		cm.onApply(change)
	}
}

// RejectCurrent rejects the current change
func (cm *ChangeManager) RejectCurrent() {
	change := cm.GetCurrentChange()
	if change == nil {
		return
	}
	
	change.Status = ChangeStatusRejected
	cm.updateList()
	
	if cm.onReject != nil {
		cm.onReject(change)
	}
}

// AcceptAll accepts all pending changes
func (cm *ChangeManager) AcceptAll() {
	for _, change := range cm.changes {
		if change.Status == ChangeStatusPending {
			change.Status = ChangeStatusAccepted
			if cm.onApply != nil {
				cm.onApply(change)
			}
		}
	}
	cm.updateList()
}

// RejectAll rejects all pending changes
func (cm *ChangeManager) RejectAll() {
	for _, change := range cm.changes {
		if change.Status == ChangeStatusPending {
			change.Status = ChangeStatusRejected
		}
	}
	cm.updateList()
}

// PreviewCurrent shows a preview of the current change
func (cm *ChangeManager) PreviewCurrent() {
	change := cm.GetCurrentChange()
	if change == nil {
		return
	}
	
	if cm.onPreview != nil {
		cm.onPreview(change)
	}
	
	if cm.editor != nil {
		// Open the file and highlight the change region
		cm.editor.OpenFileWithContent(change.Filename, change.Modified)
	}
	
	cm.refreshViewport()
}

// HasChanges returns true if there are any changes
func (cm *ChangeManager) HasChanges() bool {
	return len(cm.changes) > 0
}

// PendingCount returns the number of pending changes
func (cm *ChangeManager) PendingCount() int {
	count := 0
	for _, change := range cm.changes {
		if change.Status == ChangeStatusPending {
			count++
		}
	}
	return count
}

// refreshViewport updates the preview viewport
func (cm *ChangeManager) refreshViewport() {
	change := cm.GetCurrentChange()
	if change == nil {
		cm.viewport.SetContent("No changes to preview")
		return
	}
	
	// Build diff preview
	var content strings.Builder
	
	content.WriteString(fmt.Sprintf("File: %s\n", change.Filename))
	content.WriteString(fmt.Sprintf("Lines: %d - %d\n\n", change.StartLine, change.EndLine))
	
	content.WriteString("Original:\n")
	content.WriteString(strings.Repeat("-", 40))
	content.WriteString("\n")
	content.WriteString(change.Original)
	content.WriteString("\n\n")
	
	content.WriteString("Modified:\n")
	content.WriteString(strings.Repeat("+", 40))
	content.WriteString("\n")
	content.WriteString(change.Modified)
	
	cm.viewport.SetContent(content.String())
}

// Update handles messages
func (cm *ChangeManager) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return cm.handleKeyMsg(msg)
		
	case tea.WindowSizeMsg:
		cm.SetSize(msg.Width, msg.Height)
	}
	
	// Update list
	var cmd tea.Cmd
	cm.list, cmd = cm.list.Update(msg)
	
	// Update selected index
	if cm.list.Index() != cm.selectedIdx {
		cm.selectedIdx = cm.list.Index()
		cm.refreshViewport()
	}
	
	// Update viewport
	var vpCmd tea.Cmd
	cm.viewport, vpCmd = cm.viewport.Update(msg)
	
	return func() tea.Msg {
		cmd()
		vpCmd()
		return nil
	}
}

// handleKeyMsg handles key messages
func (cm *ChangeManager) handleKeyMsg(msg tea.KeyMsg) tea.Cmd {
	if !cm.focused {
		return nil
	}
	
	switch {
	case key.Matches(msg, cm.keys.Accept):
		cm.AcceptCurrent()
		
	case key.Matches(msg, cm.keys.Reject):
		cm.RejectCurrent()
		
	case key.Matches(msg, cm.keys.Preview):
		cm.PreviewCurrent()
		
	case key.Matches(msg, cm.keys.ApplyAll):
		cm.AcceptAll()
		
	case key.Matches(msg, cm.keys.RejectAll):
		cm.RejectAll()
		
	case key.Matches(msg, cm.keys.Close):
		if cm.onClose != nil {
			cm.onClose()
		}
	}
	
	return nil
}

// View renders the change manager
func (cm *ChangeManager) View() string {
	borderStyle := cm.styles.App.PanelBorder
	if cm.focused {
		borderStyle = cm.styles.App.FocusedBorder
	}
	
	var sections []string
	
	// Header
	pendingCount := cm.PendingCount()
	header := cm.styles.App.Title.Render(
		fmt.Sprintf("🔄 Changes (%d pending)", pendingCount),
	)
	sections = append(sections, header)
	
	// Changes list
	listView := borderStyle.
		Width(cm.width - 2).
		Height(cm.height/2).
		Render(cm.list.View())
	sections = append(sections, listView)
	
	// Preview
	previewLabel := cm.styles.StatusBar.Mode.Render(" Preview ")
	previewBorder := borderStyle.Width(cm.width - 2)
	previewView := previewBorder.
		Height(cm.height/2 - 2).
		Render(previewLabel + "\n" + cm.viewport.View())
	sections = append(sections, previewView)
	
	// Help
	helpText := cm.renderHelp()
	help := cm.styles.Help.Desc.Render(helpText)
	sections = append(sections, help)
	
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderHelp renders the help text
func (cm *ChangeManager) renderHelp() string {
	var parts []string
	parts = append(parts, "a/y:accept r/n:reject p:preview A:accept all R:reject all")
	return strings.Join(parts, " • ")
}

// changeItem implements list.Item for suggested changes
type changeItem struct {
	title    string
	desc     string
	changeID string
	status   ChangeStatus
}

func (i changeItem) Title() string       { return i.title }
func (i changeItem) Description() string { return i.desc }
func (i changeItem) FilterValue() string { return i.title }

// NewSuggestedChange creates a new suggested change
func NewSuggestedChange(id, filename, description, original, modified string, startLine, endLine int) *SuggestedChange {
	// Build diff
	diff := &Diff{
		Filename:  filename,
		Hunks:     []DiffHunk{},
		OrigLines: strings.Split(original, "\n"),
		ModLines:  strings.Split(modified, "\n"),
	}
	
	// Simple diff generation
	lines := generateSimpleDiff(strings.Split(original, "\n"), strings.Split(modified, "\n"))
	diff.Hunks = append(diff.Hunks, DiffHunk{
		StartLeft:  startLine,
		StartRight: startLine,
		Lines:     lines,
		Header:    fmt.Sprintf("@@ -%d +%d @@", startLine, startLine),
	})
	
	return &SuggestedChange{
		ID:          id,
		Filename:    filename,
		Description: description,
		Original:    original,
		Modified:    modified,
		StartLine:   startLine,
		EndLine:     endLine,
		Status:      ChangeStatusPending,
		Diff:        diff,
	}
}

// generateSimpleDiff generates a simple line-by-line diff
func generateSimpleDiff(orig, mod []string) []DiffLine {
	var lines []DiffLine
	
	i, j := 0, 0
	for i < len(orig) || j < len(mod) {
		if i >= len(orig) {
			// Remaining lines are additions
			lines = append(lines, DiffLine{
				Type:      ChangeTypeAdded,
				RightLine: j + 1,
				Content:   mod[j],
			})
			j++
		} else if j >= len(mod) {
			// Remaining lines are deletions
			lines = append(lines, DiffLine{
				Type:     ChangeTypeDeleted,
				LeftLine: i + 1,
				Content:  orig[i],
			})
			i++
		} else if orig[i] == mod[j] {
			// Unchanged
			lines = append(lines, DiffLine{
				Type:       ChangeTypeUnchanged,
				LeftLine:   i + 1,
				RightLine:  j + 1,
				Content:    orig[i],
			})
			i++
			j++
		} else {
			// Modified (simplified: treat as delete + add)
			lines = append(lines, DiffLine{
				Type:     ChangeTypeDeleted,
				LeftLine: i + 1,
				Content:  orig[i],
			})
			i++
			lines = append(lines, DiffLine{
				Type:      ChangeTypeAdded,
				RightLine: j + 1,
				Content:   mod[j],
			})
			j++
		}
	}
	
	return lines
}
