package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Event struct {
	Type    string
	Message string
}

type Neural struct {
	viewport viewport.Model
	events   []Event
}

func NewNeural() *Neural {
	vp := viewport.New(0, 0)
	vp.SetContent("Neural Bus Events\n")
	return &Neural{
		viewport: vp,
		events:   []Event{},
	}
}

func (n *Neural) Init() tea.Cmd {
	return nil
}

func (n *Neural) Update(msg tea.Msg) (*Neural, tea.Cmd) {
	var cmd tea.Cmd
	n.viewport, cmd = n.viewport.Update(msg)
	return n, cmd
}

func (n *Neural) View(width, height int) string {
	n.viewport.Width = width - 2
	n.viewport.Height = height - 2
	return NeuralPanelStyle.Width(width).Height(height).Render(n.viewport.View())
}

func (n *Neural) AddEvent(eventType, message string) {
	n.events = append(n.events, Event{Type: eventType, Message: message})
	n.updateContent()
}

func (n *Neural) updateContent() {
	var sb strings.Builder
	for _, event := range n.events {
		color := Teal
		if event.Type == "error" {
			color = lipgloss.Color("#ff0000")
		}
		style := EventStyle.Foreground(color)
		sb.WriteString(style.Render(fmt.Sprintf("[%s] %s", event.Type, event.Message)))
		sb.WriteString("\n")
	}
	n.viewport.SetContent(sb.String())
}
