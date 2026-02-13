// Package tui provides BubbleTea-based TUI components
package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStyles(t *testing.T) {
	styles := NewStyles(ThemeDracula)
	
	assert.NotNil(t, styles)
	assert.Equal(t, ThemeDracula, styles.Theme)
	assert.NotEmpty(t, styles.Colors.Background)
	assert.NotEmpty(t, styles.Colors.Foreground)
}

func TestDefaultStyles(t *testing.T) {
	styles := DefaultStyles()
	
	assert.NotNil(t, styles)
	assert.Equal(t, ThemeDracula, styles.Theme)
}

func TestGetFileIcon(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"main.go", "🐹"},
		{"README.md", "📝"},
		{"config.json", "📋"},
		{"script.sh", "🔧"},
		{"index.html", "🌐"},
		{"style.css", "🎨"},
		{"unknown.xyz", "📄"},
		{"test.test.test", "📄"},
	}
	
	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			icon := GetFileIcon(tt.filename)
			assert.Equal(t, tt.expected, icon)
		})
	}
}

func TestNewLayoutManager(t *testing.T) {
	config := DefaultLayoutConfig()
	lm := NewLayoutManager(config)
	
	assert.NotNil(t, lm)
	assert.Equal(t, PanelFileBrowser, lm.Focused)
}

func TestLayoutManagerUpdateSizes(t *testing.T) {
	config := DefaultLayoutConfig()
	lm := NewLayoutManager(config)
	
	lm.UpdateSizes(100, 30)
	
	// UpdateSizes modifies lm.Config, not the original config
	assert.Equal(t, 100, lm.Config.Width)
	assert.Equal(t, 30, lm.Config.Height)
	assert.Greater(t, lm.Sizes.FileBrowserWidth, 0)
	assert.Greater(t, lm.Sizes.ChatWidth, 0)
}

func TestLayoutManagerGetPanelBounds(t *testing.T) {
	config := DefaultLayoutConfig()
	lm := NewLayoutManager(config)
	lm.UpdateSizes(100, 30)
	
	x, y, width, height := lm.GetPanelBounds(PanelFileBrowser)
	
	assert.Equal(t, 0, x)
	assert.Equal(t, 0, y)
	assert.Greater(t, width, 0)
	assert.Greater(t, height, 0)
}

func TestLayoutManagerFocusNext(t *testing.T) {
	config := DefaultLayoutConfig()
	lm := NewLayoutManager(config)
	
	assert.Equal(t, PanelFileBrowser, lm.Focused)
	
	lm.FocusNext()
	assert.Equal(t, PanelChat, lm.Focused)
	
	lm.FocusNext()
	assert.Equal(t, PanelFileBrowser, lm.Focused)
}

func TestLayoutManagerIsSmallScreen(t *testing.T) {
	config := DefaultLayoutConfig()
	lm := NewLayoutManager(config)
	
	lm.UpdateSizes(79, 24)
	assert.True(t, lm.IsSmallScreen())
	
	lm.UpdateSizes(80, 24)
	assert.False(t, lm.IsSmallScreen())
}

func TestNewChatPanel(t *testing.T) {
	styles := DefaultStyles()
	panel := NewChatPanel(styles, 80, 24)
	
	assert.NotNil(t, panel)
	assert.Equal(t, 80, panel.width)
	assert.Equal(t, 24, panel.height)
	assert.False(t, panel.IsFocused())
	assert.Equal(t, 0, len(panel.GetMessages()))
}

func TestChatPanelAddMessage(t *testing.T) {
	styles := DefaultStyles()
	panel := NewChatPanel(styles, 80, 24)
	
	panel.AddUserMessage("hello")
	panel.AddAgentMessage("world")
	
	messages := panel.GetMessages()
	assert.Equal(t, 2, len(messages))
	assert.Equal(t, ChatRoleUser, messages[0].Role)
	assert.Equal(t, "hello", messages[0].Content)
	assert.Equal(t, ChatRoleAgent, messages[1].Role)
	assert.Equal(t, "world", messages[1].Content)
}

func TestChatPanelFocus(t *testing.T) {
	styles := DefaultStyles()
	panel := NewChatPanel(styles, 80, 24)
	
	assert.False(t, panel.IsFocused())
	
	panel.Focus()
	assert.True(t, panel.IsFocused())
	
	panel.Blur()
	assert.False(t, panel.IsFocused())
}

func TestChatPanelClear(t *testing.T) {
	styles := DefaultStyles()
	panel := NewChatPanel(styles, 80, 24)
	
	panel.AddUserMessage("test")
	assert.Equal(t, 1, len(panel.GetMessages()))
	
	panel.Clear()
	assert.Equal(t, 0, len(panel.GetMessages()))
}

func TestNewFileBrowser(t *testing.T) {
	styles := DefaultStyles()
	tmpDir := t.TempDir()
	
	browser, err := NewFileBrowser(styles, tmpDir)
	
	require.NoError(t, err)
	assert.NotNil(t, browser)
	assert.Equal(t, tmpDir, browser.GetRootPath())
	assert.True(t, browser.IsFocused())
}

func TestFileBrowserFocus(t *testing.T) {
	styles := DefaultStyles()
	tmpDir := t.TempDir()
	
	browser, err := NewFileBrowser(styles, tmpDir)
	require.NoError(t, err)
	
	assert.True(t, browser.IsFocused())
	
	browser.Blur()
	assert.False(t, browser.IsFocused())
}

func TestNewEditorPanel(t *testing.T) {
	styles := DefaultStyles()
	editor := NewEditorPanel(styles, 80, 24)
	
	assert.NotNil(t, editor)
	assert.Equal(t, 80, editor.width)
	assert.Equal(t, 24, editor.height)
	assert.False(t, editor.IsFocused())
	assert.False(t, editor.HasTabs())
}

func TestEditorPanelHasTabs(t *testing.T) {
	styles := DefaultStyles()
	editor := NewEditorPanel(styles, 80, 24)
	
	assert.False(t, editor.HasTabs())
	assert.Equal(t, 0, editor.GetTabCount())
}

func TestNewDiffViewer(t *testing.T) {
	styles := DefaultStyles()
	viewer := NewDiffViewer(styles, 80, 24)
	
	assert.NotNil(t, viewer)
	assert.Equal(t, 80, viewer.width)
	assert.Equal(t, 24, viewer.height)
	assert.False(t, viewer.IsFocused())
	assert.False(t, viewer.HasDiff())
}

func TestNewChangeManager(t *testing.T) {
	styles := DefaultStyles()
	manager := NewChangeManager(styles, 80, 24)
	
	assert.NotNil(t, manager)
	assert.Equal(t, 80, manager.width)
	assert.Equal(t, 24, manager.height)
	assert.False(t, manager.IsFocused())
	assert.False(t, manager.HasChanges())
}

func TestSuggestedChange(t *testing.T) {
	change := NewSuggestedChange(
		"test-id",
		"test.go",
		"Update function",
		"func old() {}\n",
		"func new() {\n\treturn\n}\n",
		1,
		2,
	)
	
	assert.NotNil(t, change)
	assert.Equal(t, "test-id", change.ID)
	assert.Equal(t, "test.go", change.Filename)
	assert.Equal(t, "Update function", change.Description)
	assert.Equal(t, ChangeStatusPending, change.Status)
	assert.NotNil(t, change.Diff)
}

func TestChangeKeyMap(t *testing.T) {
	keys := DefaultChangeKeyMap()
	
	assert.NotNil(t, keys.Accept)
	assert.NotNil(t, keys.Reject)
	assert.NotNil(t, keys.Preview)
	assert.NotNil(t, keys.NextChange)
	assert.NotNil(t, keys.PrevChange)
}

func TestEditorKeyMap(t *testing.T) {
	keys := DefaultEditorKeyMap()
	
	assert.NotNil(t, keys.Save)
	assert.NotNil(t, keys.CloseTab)
	assert.NotNil(t, keys.NextTab)
	assert.NotNil(t, keys.PrevTab)
	assert.NotNil(t, keys.Undo)
	assert.NotNil(t, keys.Redo)
}

func TestDiffKeyMap(t *testing.T) {
	keys := DefaultDiffKeyMap()
	
	assert.NotNil(t, keys.NextChange)
	assert.NotNil(t, keys.PrevChange)
	assert.NotNil(t, keys.JumpChange)
	assert.NotNil(t, keys.Accept)
	assert.NotNil(t, keys.Reject)
	assert.NotNil(t, keys.ToggleMode)
}

// Test app model initialization
func TestNewAppModel(t *testing.T) {
	config := AppConfig{
		Theme:      ThemeDracula,
		RootPath:    ".",
		SessionID:   "test-session",
		SelectedModel: "test-model",
	}
	
	model, err := NewAppModel(config)
	
	require.NoError(t, err)
	assert.NotNil(t, model)
	assert.NotNil(t, model.browser)
	assert.NotNil(t, model.chat)
	assert.NotNil(t, model.editor)
	assert.NotNil(t, model.diffViewer)
	assert.NotNil(t, model.changeManager)
}

func TestAppKeyMap(t *testing.T) {
	keys := DefaultAppKeyMap()
	
	assert.NotNil(t, keys.Quit)
	assert.NotNil(t, keys.Help)
	assert.NotNil(t, keys.FocusNext)
	assert.NotNil(t, keys.SelectModel)
	assert.NotNil(t, keys.ShowEditor)
	assert.NotNil(t, keys.ShowDiff)
}
