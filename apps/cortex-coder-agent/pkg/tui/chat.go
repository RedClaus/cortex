package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ChatMode represents the current chat mode
type ChatMode int

const (
	ModeNormal ChatMode = iota
	ModeInsert
)

// ChatPanel represents the chat interface
type ChatPanel struct {
	styles       Styles
	width        int
	height       int
	focused      bool
	mode         ChatMode
	messages     []ChatMessage
	viewport     viewport.Model
	input        textarea.Model
	keys         ChatKeyMap
	onSubmit     func(string) tea.Cmd // Callback when user submits message
}

// ChatKeyMap defines key bindings
type ChatKeyMap struct {
	Submit     key.Binding
	Clear      key.Binding
	EnterInsert key.Binding
	ExitInsert  key.Binding
}

func DefaultChatKeyMap() ChatKeyMap {
	return ChatKeyMap{
		Submit: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "send"),
		),
		Clear: key.NewBinding(
			key.WithKeys("ctrl+u"),
			key.WithHelp("ctrl+u", "clear"),
		),
		EnterInsert: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "insert mode"),
		),
		ExitInsert: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "normal mode"),
		),
	}
}

// NewChatPanel creates a new chat panel
func NewChatPanel(styles Styles, width, height int) *ChatPanel {
	input := textarea.New()
	input.SetWidth(width - 4)
	input.SetHeight(3)
	input.Prompt = "> "
	input.FocusedStyle.CursorLine = lipgloss.NewStyle()
	input.ShowLineNumbers = false
	input.KeyMap.InsertNewline.SetEnabled(false) // Disable multi-line for now
	
	vp := viewport.New(width-4, height-8)
	vp.Style = lipgloss.NewStyle().
		Background(lipgloss.Color(styles.Colors.Background))
	
	return &ChatPanel{
		styles:   styles,
		width:    width,
		height:   height,
		mode:     ModeNormal,
		messages: make([]ChatMessage, 0),
		viewport: vp,
		input:    input,
		keys:     DefaultChatKeyMap(),
	}
}

// SetSize updates dimensions
func (cp *ChatPanel) SetSize(width, height int) {
	cp.width = width
	cp.height = height
	cp.input.SetWidth(width - 4)
	cp.viewport.Width = width - 4
	cp.viewport.Height = height - 8
}

// Focus activates the panel
func (cp *ChatPanel) Focus() {
	cp.focused = true
	cp.input.Focus()
}

// Blur deactivates the panel
func (cp *ChatPanel) Blur() {
	cp.focused = false
	cp.input.Blur()
}

// IsFocused returns focus state
func (cp *ChatPanel) IsFocused() bool {
	return cp.focused
}

// SetOnSubmit sets the submit callback
func (cp *ChatPanel) SetOnSubmit(fn func(string) tea.Cmd) {
	cp.onSubmit = fn
}

// AddMessage adds a message
func (cp *ChatPanel) AddMessage(role ChatRole, content string) {
	msg := ChatMessage{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}
	cp.messages = append(cp.messages, msg)
	cp.updateViewport()
}

// AddUserMessage adds user message
func (cp *ChatPanel) AddUserMessage(content string) {
	cp.AddMessage(ChatRoleUser, content)
}

// AddAgentMessage adds agent message
func (cp *ChatPanel) AddAgentMessage(content string) {
	cp.AddMessage(ChatRoleAgent, content)
}

// GetInput returns current input
func (cp *ChatPanel) GetInput() string {
	return cp.input.Value()
}

// ClearInput clears the input
func (cp *ChatPanel) ClearInput() {
	cp.input.Reset()
}

// HasInput returns true if there's input
func (cp *ChatPanel) HasInput() bool {
	return cp.input.Value() != ""
}

// updateViewport refreshes the message display
func (cp *ChatPanel) updateViewport() {
	var content strings.Builder
	
	for _, msg := range cp.messages {
		content.WriteString(cp.renderMessage(msg))
		content.WriteString("\n\n")
	}
	
	cp.viewport.SetContent(content.String())
	cp.viewport.GotoBottom()
}

// renderMessage formats a single message
func (cp *ChatPanel) renderMessage(msg ChatMessage) string {
	var prefix string
	var style lipgloss.Style
	
	switch msg.Role {
	case ChatRoleUser:
		prefix = "You"
		style = cp.styles.Chat.UserMessage
	case ChatRoleAgent:
		prefix = "Agent"
		style = cp.styles.Chat.AgentMessage
	case ChatRoleSystem:
		prefix = "System"
		style = cp.styles.Chat.SystemMessage
	}
	
	header := cp.styles.Chat.Timestamp.Render(
		fmt.Sprintf("[%s] %s: ", msg.Timestamp.Format("15:04"), prefix),
	)
	
	return header + style.Render(msg.Content)
}

// Update handles messages
func (cp *ChatPanel) Update(msg tea.Msg) tea.Cmd {
	if !cp.focused {
		return nil
	}
	
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, cp.keys.Submit):
			if cp.HasInput() {
				input := cp.GetInput()
				cp.ClearInput()
				cp.AddUserMessage(input)
				if cp.onSubmit != nil {
					return cp.onSubmit(input)
				}
			}
			return nil
		case key.Matches(msg, cp.keys.Clear):
			cp.ClearInput()
			return nil
		}
	case tea.WindowSizeMsg:
		cp.SetSize(msg.Width, msg.Height)
	}
	
	// Always update textarea for typing
	var cmd tea.Cmd
	cp.input, cmd = cp.input.Update(msg)
	return cmd
}

// View renders the chat panel
func (cp *ChatPanel) View() string {
	if !cp.focused {
		return cp.styles.App.PanelBorder.Render("Chat (press '2' to focus)")
	}
	
	// Header
	header := cp.styles.App.Title.Render("💬 Chat")
	
	// Messages viewport
	messages := cp.styles.App.PanelBorder.
		Width(cp.width - 2).
		Height(cp.height - 6).
		Render(cp.viewport.View())
	
	// Input area
	inputLabel := cp.styles.Chat.InputPrompt.Render("> ")
	inputArea := lipgloss.JoinHorizontal(lipgloss.Left, inputLabel, cp.input.View())
	
	// Mode indicator
	modeText := ""
	if cp.mode == ModeInsert {
		modeText = "[INSERT]"
	}
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		messages,
		cp.styles.StatusBar.Mode.Render(modeText),
		inputArea,
	)
}

// GetMessages returns all messages
func (cp *ChatPanel) GetMessages() []ChatMessage {
	return cp.messages
}

// Clear clears all messages
func (cp *ChatPanel) Clear() {
	cp.messages = make([]ChatMessage, 0)
	cp.viewport.SetContent("")
}

// ChatMessage represents a chat message
type ChatMessage struct {
	Role      ChatRole
	Content   string
	Timestamp time.Time
}

// ChatRole represents message sender type
type ChatRole string

const (
	ChatRoleUser   ChatRole = "user"
	ChatRoleAgent  ChatRole = "agent"
	ChatRoleSystem ChatRole = "system"
)
