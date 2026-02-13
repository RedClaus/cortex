package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type Input struct {
	textinput textinput.Model
}

func NewInput() *Input {
	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.Focus()
	return &Input{textinput: ti}
}

func (i *Input) Init() tea.Cmd {
	return textinput.Blink
}

func (i *Input) Update(msg tea.Msg) (*Input, tea.Cmd) {
	var cmd tea.Cmd
	i.textinput, cmd = i.textinput.Update(msg)
	return i, cmd
}

func (i *Input) View() string {
	return InputBarStyle.Render(i.textinput.View())
}

func (i *Input) Value() string {
	return i.textinput.Value()
}

func (i *Input) SetValue(value string) {
	i.textinput.SetValue(value)
}

func (i *Input) Reset() {
	i.textinput.Reset()
}
