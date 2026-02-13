package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

// GitStatus represents the git status of a file
type GitStatus string

const (
	GitStatusNone      GitStatus = ""
	GitStatusModified  GitStatus = "M"
	GitStatusAdded     GitStatus = "A"
	GitStatusDeleted   GitStatus = "D"
	GitStatusRenamed   GitStatus = "R"
	GitStatusCopied    GitStatus = "C"
	GitStatusUpdated   GitStatus = "U"
	GitStatusUntracked GitStatus = "?"
	GitStatusIgnored   GitStatus = "!"
)

// TreeNode represents a node in the file tree
type TreeNode struct {
	Name         string
	Path         string
	IsDir        bool
	IsExpanded   bool
	Children     []*TreeNode
	Parent       *TreeNode
	GitStatus    GitStatus
	Depth        int
}

// FileBrowser represents the file browser component
type FileBrowser struct {
	styles       Styles
	rootPath     string
	rootNode     *TreeNode
	cursor       int
	flatList     []*TreeNode
	selected     *TreeNode
	width        int
	height       int
	focused      bool
	gitRepo      *git.Repository
	gitWorktree  *git.Worktree
	gitStatus    map[string]GitStatus
	keys         BrowserKeyMap
}

// BrowserKeyMap defines key bindings for the file browser
type BrowserKeyMap struct {
	Up         key.Binding
	Down       key.Binding
	Left       key.Binding
	Right      key.Binding
	Enter      key.Binding
	Space      key.Binding
	Toggle     key.Binding
	Reload     key.Binding
	GotoTop    key.Binding
	GotoBottom key.Binding
}

// DefaultBrowserKeyMap returns default key bindings
func DefaultBrowserKeyMap() BrowserKeyMap {
	return BrowserKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "collapse"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "expand"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "open"),
		),
		Space: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "toggle"),
		),
		Toggle: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "expand all"),
		),
		Reload: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "reload"),
		),
		GotoTop: key.NewBinding(
			key.WithKeys("g", "home"),
			key.WithHelp("g/home", "top"),
		),
		GotoBottom: key.NewBinding(
			key.WithKeys("G", "end"),
			key.WithHelp("G/end", "bottom"),
		),
	}
}

// NewFileBrowser creates a new file browser
func NewFileBrowser(styles Styles, rootPath string) (*FileBrowser, error) {
	fb := &FileBrowser{
		styles:   styles,
		rootPath: rootPath,
		cursor:   0,
		keys:     DefaultBrowserKeyMap(),
		focused:  true,
	}
	
	// Try to open git repository
	if err := fb.initGit(); err == nil {
		fb.refreshGitStatus()
	}
	
	// Build the tree
	if err := fb.buildTree(); err != nil {
		return nil, fmt.Errorf("failed to build file tree: %w", err)
	}
	
	return fb, nil
}

// initGit attempts to initialize git repository
func (fb *FileBrowser) initGit() error {
	repo, err := git.PlainOpen(fb.rootPath)
	if err != nil {
		return err
	}
	
	worktree, err := repo.Worktree()
	if err != nil {
		return err
	}
	
	fb.gitRepo = repo
	fb.gitWorktree = worktree
	fb.gitStatus = make(map[string]GitStatus)
	
	return nil
}

// refreshGitStatus updates the git status map
func (fb *FileBrowser) refreshGitStatus() {
	if fb.gitWorktree == nil {
		return
	}
	
	status, err := fb.gitWorktree.Status()
	if err != nil {
		return
	}
	
	fb.gitStatus = make(map[string]GitStatus)
	
	for file, s := range status {
		var gitStatus GitStatus
		
		switch {
		case s.Staging == git.Untracked || s.Worktree == git.Untracked:
			gitStatus = GitStatusUntracked
		case s.Staging == git.Modified || s.Worktree == git.Modified:
			gitStatus = GitStatusModified
		case s.Staging == git.Added || s.Worktree == git.Added:
			gitStatus = GitStatusAdded
		case s.Staging == git.Deleted || s.Worktree == git.Deleted:
			gitStatus = GitStatusDeleted
		case s.Staging == git.Renamed || s.Worktree == git.Renamed:
			gitStatus = GitStatusRenamed
		case s.Staging == git.Copied || s.Worktree == git.Copied:
			gitStatus = GitStatusCopied
		case s.Staging == git.UpdatedButUnmerged || s.Worktree == git.UpdatedButUnmerged:
			gitStatus = GitStatusUpdated
		default:
			gitStatus = GitStatusNone
		}
		
		fb.gitStatus[file] = gitStatus
	}
}

// buildTree builds the file tree from the root path
func (fb *FileBrowser) buildTree() error {
	fb.rootNode = &TreeNode{
		Name:       filepath.Base(fb.rootPath),
		Path:       fb.rootPath,
		IsDir:      true,
		IsExpanded: true,
		Depth:      0,
	}
	
	if err := fb.loadChildren(fb.rootNode); err != nil {
		return err
	}
	
	fb.rebuildFlatList()
	return nil
}

// loadChildren loads children of a directory node
func (fb *FileBrowser) loadChildren(node *TreeNode) error {
	entries, err := os.ReadDir(node.Path)
	if err != nil {
		return err
	}
	
	// Sort: directories first, then files alphabetically
	sort.Slice(entries, func(i, j int) bool {
		iIsDir := entries[i].IsDir()
		jIsDir := entries[j].IsDir()
		if iIsDir != jIsDir {
			return iIsDir
		}
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})
	
	// Load gitignore patterns if in git repo
	var patterns []gitignore.Pattern
	if fb.gitWorktree != nil {
		patterns = fb.loadGitignorePatterns(node.Path)
	}
	
	for _, entry := range entries {
		name := entry.Name()
		
		// Skip hidden files unless they're git-related
		if strings.HasPrefix(name, ".") && name != ".gitignore" && name != ".git" {
			continue
		}
		
		childPath := filepath.Join(node.Path, name)
		relPath, _ := filepath.Rel(fb.rootPath, childPath)
		
		// Check gitignore
		if len(patterns) > 0 {
			match := gitignore.NewMatcher(patterns)
			if match.Match(strings.Split(relPath, string(filepath.Separator)), entry.IsDir()) {
				continue
			}
		}
		
		child := &TreeNode{
			Name:   name,
			Path:   childPath,
			IsDir:  entry.IsDir(),
			Parent: node,
			Depth:  node.Depth + 1,
		}
		
		// Get git status
		if status, ok := fb.gitStatus[relPath]; ok {
			child.GitStatus = status
		}
		
		if entry.IsDir() {
			// Don't expand by default
			child.IsExpanded = false
		}
		
		node.Children = append(node.Children, child)
	}
	
	return nil
}

// loadGitignorePatterns loads gitignore patterns from a directory
func (fb *FileBrowser) loadGitignorePatterns(dir string) []gitignore.Pattern {
	var patterns []gitignore.Pattern
	
	gitignorePath := filepath.Join(dir, ".gitignore")
	if data, err := os.ReadFile(gitignorePath); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				patterns = append(patterns, gitignore.ParsePattern(line, nil))
			}
		}
	}
	
	return patterns
}

// rebuildFlatList rebuilds the flat list of visible nodes
func (fb *FileBrowser) rebuildFlatList() {
	fb.flatList = make([]*TreeNode, 0)
	fb.addVisibleNodes(fb.rootNode)
}

// addVisibleNodes recursively adds visible nodes to the flat list
func (fb *FileBrowser) addVisibleNodes(node *TreeNode) {
	fb.flatList = append(fb.flatList, node)
	
	if node.IsDir && node.IsExpanded {
		for _, child := range node.Children {
			fb.addVisibleNodes(child)
		}
	}
}

// SetSize sets the browser size
func (fb *FileBrowser) SetSize(width, height int) {
	fb.width = width
	fb.height = height
}

// Focus sets focus state
func (fb *FileBrowser) Focus() {
	fb.focused = true
}

// Blur removes focus
func (fb *FileBrowser) Blur() {
	fb.focused = false
}

// IsFocused returns true if the browser is focused
func (fb *FileBrowser) IsFocused() bool {
	return fb.focused
}

// Update handles messages
func (fb *FileBrowser) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if !fb.focused {
			return nil
		}
		
		switch {
		case key.Matches(msg, fb.keys.Up):
			fb.moveCursor(-1)
		case key.Matches(msg, fb.keys.Down):
			fb.moveCursor(1)
		case key.Matches(msg, fb.keys.Left, fb.keys.Enter):
			fb.toggleCurrent()
		case key.Matches(msg, fb.keys.Right):
			fb.expandCurrent()
		case key.Matches(msg, fb.keys.Space):
			fb.toggleCurrent()
		case key.Matches(msg, fb.keys.Reload):
			fb.reload()
		case key.Matches(msg, fb.keys.GotoTop):
			fb.cursor = 0
		case key.Matches(msg, fb.keys.GotoBottom):
			fb.cursor = len(fb.flatList) - 1
		}
	}
	
	return nil
}

// moveCursor moves the cursor by delta
func (fb *FileBrowser) moveCursor(delta int) {
	newCursor := fb.cursor + delta
	if newCursor >= 0 && newCursor < len(fb.flatList) {
		fb.cursor = newCursor
	}
}

// toggleCurrent toggles expansion of the current directory
func (fb *FileBrowser) toggleCurrent() {
	if fb.cursor < 0 || fb.cursor >= len(fb.flatList) {
		return
	}
	
	node := fb.flatList[fb.cursor]
	if !node.IsDir {
		fb.selected = node
		return
	}
	
	if node.IsExpanded {
		node.IsExpanded = false
	} else {
		if len(node.Children) == 0 {
			fb.loadChildren(node)
		}
		node.IsExpanded = true
	}
	
	fb.rebuildFlatList()
}

// expandCurrent expands the current directory
func (fb *FileBrowser) expandCurrent() {
	if fb.cursor < 0 || fb.cursor >= len(fb.flatList) {
		return
	}
	
	node := fb.flatList[fb.cursor]
	if !node.IsDir {
		return
	}
	
	if len(node.Children) == 0 {
		fb.loadChildren(node)
	}
	node.IsExpanded = true
	fb.rebuildFlatList()
}

// reload reloads the file tree
func (fb *FileBrowser) reload() {
	fb.refreshGitStatus()
	fb.buildTree()
}

// View renders the file browser
func (fb *FileBrowser) View() string {
	if len(fb.flatList) == 0 {
		return fb.styles.Browser.Container.Render("No files")
	}
	
	var lines []string
	
	// Title
	title := fb.styles.App.Title.Render("📁 " + fb.rootNode.Name)
	lines = append(lines, title)
	lines = append(lines, "")
	
	// Calculate visible range
	visibleHeight := fb.height - 4 // Account for title and borders
	startIdx := 0
	endIdx := len(fb.flatList)
	
	if visibleHeight > 0 && len(fb.flatList) > visibleHeight {
		startIdx = fb.cursor - visibleHeight/2
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx = startIdx + visibleHeight
		if endIdx > len(fb.flatList) {
			endIdx = len(fb.flatList)
			startIdx = endIdx - visibleHeight
			if startIdx < 0 {
				startIdx = 0
			}
		}
	}
	
	// Render visible nodes
	for i := startIdx; i < endIdx && i < len(fb.flatList); i++ {
		node := fb.flatList[i]
		line := fb.renderNode(node, i == fb.cursor)
		lines = append(lines, line)
	}
	
	content := strings.Join(lines, "\n")
	
	// Apply container styling
	borderStyle := fb.styles.App.PanelBorder
	if fb.focused {
		borderStyle = fb.styles.App.FocusedBorder
	}
	
	return borderStyle.
		Width(fb.width - 2).
		Height(fb.height - 2).
		Render(content)
}

// renderNode renders a single tree node
func (fb *FileBrowser) renderNode(node *TreeNode, isSelected bool) string {
	var parts []string
	
	// Indentation
	indent := strings.Repeat("  ", node.Depth)
	parts = append(parts, fb.styles.Browser.Indent.Render(indent))
	
	// Expand/collapse indicator for directories
	if node.IsDir {
		if node.IsExpanded {
			parts = append(parts, fb.styles.Browser.Icon.Render("▾ "))
		} else {
			parts = append(parts, fb.styles.Browser.Icon.Render("▸ "))
		}
	} else {
		parts = append(parts, "  ")
	}
	
	// Icon
	var icon string
	if node.IsDir {
		if node.IsExpanded {
			icon = "📂"
		} else {
			icon = "📁"
		}
	} else {
		icon = GetFileIcon(node.Name)
	}
	parts = append(parts, fb.styles.Browser.Icon.Render(icon+" "))
	
	// Name with git status
	nameStyle := fb.styles.Browser.File
	if node.IsDir {
		nameStyle = fb.styles.Browser.Directory
	}
	
	name := nameStyle.Render(node.Name)
	
	// Git status indicator
	if node.GitStatus != GitStatusNone {
		var statusStyle lipgloss.Style
		switch node.GitStatus {
		case GitStatusModified:
			statusStyle = fb.styles.Browser.GitModified
		case GitStatusUntracked:
			statusStyle = fb.styles.Browser.GitUntracked
		case GitStatusAdded:
			statusStyle = fb.styles.Browser.GitStaged
		default:
			statusStyle = fb.styles.Browser.File
		}
		name = name + " " + statusStyle.Render(string(node.GitStatus))
	}
	
	parts = append(parts, name)
	
	line := lipgloss.JoinHorizontal(lipgloss.Left, parts...)
	
	// Apply selection style
	if isSelected {
		line = fb.styles.Browser.Selected.Render(line)
	}
	
	return line
}

// GetSelected returns the currently selected node
func (fb *FileBrowser) GetSelected() *TreeNode {
	if fb.cursor < 0 || fb.cursor >= len(fb.flatList) {
		return nil
	}
	return fb.flatList[fb.cursor]
}

// GetSelectedPath returns the path of the currently selected item
func (fb *FileBrowser) GetSelectedPath() string {
	node := fb.GetSelected()
	if node == nil {
		return ""
	}
	return node.Path
}

// IsDirSelected returns true if the selected item is a directory
func (fb *FileBrowser) IsDirSelected() bool {
	node := fb.GetSelected()
	if node == nil {
		return false
	}
	return node.IsDir
}

// GetGitStatus returns the git status for a path
func (fb *FileBrowser) GetGitStatus(path string) GitStatus {
	relPath, err := filepath.Rel(fb.rootPath, path)
	if err != nil {
		return GitStatusNone
	}
	
	if status, ok := fb.gitStatus[relPath]; ok {
		return status
	}
	return GitStatusNone
}

// GetRootPath returns the root path
func (fb *FileBrowser) GetRootPath() string {
	return fb.rootPath
}
