package tui

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Tab     key.Binding
	Quit    key.Binding
	Command key.Binding
	Up      key.Binding
	Down    key.Binding
}

var DefaultKeyMap = KeyMap{
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch panels"),
	),
	Quit: key.NewBinding(
		key.WithKeys("ctrl+c", "q"),
		key.WithHelp("q", "quit"),
	),
	Command: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "command mode"),
	),
	Up: key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "scroll up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "scroll down"),
	),
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Tab, k.Quit, k.Command}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Tab, k.Quit},
		{k.Command, k.Up, k.Down},
	}
}
