package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Message struct {
	Role    string
	Content string
}

type Chat struct {
	viewport viewport.Model
	messages []Message
}

func NewChat() *Chat {
	vp := viewport.New(0, 0)
	vp.SetContent("Welcome to Cortex-Gateway Chat\n")
	return &Chat{
		viewport: vp,
		messages: []Message{},
	}
}

func (c *Chat) Init() tea.Cmd {
	return nil
}

func (c *Chat) Update(msg tea.Msg) (*Chat, tea.Cmd) {
	var cmd tea.Cmd
	c.viewport, cmd = c.viewport.Update(msg)
	return c, cmd
}

func (c *Chat) View(width, height int) string {
	c.viewport.Width = width - 2 // padding
	c.viewport.Height = height - 2
	return ChatPanelStyle.Width(width).Height(height).Render(c.viewport.View())
}

func (c *Chat) AddMessage(role, content string) {
	c.messages = append(c.messages, Message{Role: role, Content: content})
	c.updateContent()
}

func (c *Chat) updateContent() {
	var sb strings.Builder
	for _, msg := range c.messages {
		var style lipgloss.Style
		if msg.Role == "user" {
			style = UserMessageStyle
		} else {
			style = AssistantMessageStyle
		}
		sb.WriteString(style.Render(msg.Role + ": " + msg.Content))
		sb.WriteString("\n")
	}
	c.viewport.SetContent(sb.String())
}
