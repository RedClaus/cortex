package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all key bindings for the TUI
type KeyMap struct {
	// General navigation
	Quit    key.Binding
	Help    key.Binding
	Tab     key.Binding
	Cancel  key.Binding
	Clear   key.Binding

	// Chat
	Send key.Binding

	// Toggles
	ToggleVerbose key.Binding
	ChangePersona key.Binding

	// Scrolling
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding

	// Approval dialog
	Approve     key.Binding
	Deny        key.Binding
	AlwaysAllow key.Binding
	Edit        key.Binding
}

// DefaultKeyMap returns the default key bindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("ctrl+q"),
			key.WithHelp("ctrl+q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "cycle panels"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("ctrl+c", "esc"),
			key.WithHelp("ctrl+c", "cancel"),
		),
		Clear: key.NewBinding(
			key.WithKeys("ctrl+l"),
			key.WithHelp("ctrl+l", "clear chat"),
		),
		Send: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "send"),
		),
		ToggleVerbose: key.NewBinding(
			key.WithKeys("ctrl+v"),
			key.WithHelp("ctrl+v", "toggle verbose"),
		),
		ChangePersona: key.NewBinding(
			key.WithKeys("ctrl+p"),
			key.WithHelp("ctrl+p", "change persona"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("up/k", "scroll up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("down/j", "scroll down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown"),
			key.WithHelp("pgdown", "page down"),
		),
		Approve: key.NewBinding(
			key.WithKeys("a", "y"),
			key.WithHelp("a/y", "approve"),
		),
		Deny: key.NewBinding(
			key.WithKeys("d", "n"),
			key.WithHelp("d/n", "deny"),
		),
		AlwaysAllow: key.NewBinding(
			key.WithKeys("A"),
			key.WithHelp("A", "always allow"),
		),
		Edit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit command"),
		),
	}
}

// ShortHelp returns key bindings for the mini help view
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Send, k.Quit}
}

// FullHelp returns key bindings for the full help view
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Send, k.Clear, k.Cancel},
		{k.ToggleVerbose, k.ChangePersona, k.Tab},
		{k.Up, k.Down, k.PageUp, k.PageDown},
		{k.Help, k.Quit},
	}
}

// ApprovalHelp returns key bindings for approval mode
func (k KeyMap) ApprovalHelp() []key.Binding {
	return []key.Binding{k.Approve, k.Deny, k.AlwaysAllow, k.Edit}
}
