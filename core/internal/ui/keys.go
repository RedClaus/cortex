// Package ui provides the Charmbracelet TUI framework integration for Cortex.
package ui

import (
	"github.com/charmbracelet/bubbles/key"
)

// KeyMap defines all keyboard shortcuts available in the TUI.
// It implements the help.KeyMap interface for automatic help text generation.
type KeyMap struct {
	// ═══════════════════════════════════════════════════════════════════════════
	// CORE ACTIONS
	// ═══════════════════════════════════════════════════════════════════════════

	// Send sends the current input message
	Send key.Binding

	// Cancel stops the current streaming operation
	Cancel key.Binding

	// Quit exits the application
	Quit key.Binding

	// ═══════════════════════════════════════════════════════════════════════════
	// NAVIGATION
	// ═══════════════════════════════════════════════════════════════════════════

	// Up scrolls the viewport up
	Up key.Binding

	// Down scrolls the viewport down
	Down key.Binding

	// PageUp scrolls up one page
	PageUp key.Binding

	// PageDown scrolls down one page
	PageDown key.Binding

	// Top scrolls to the top
	Top key.Binding

	// Bottom scrolls to the bottom
	Bottom key.Binding

	// ═══════════════════════════════════════════════════════════════════════════
	// MODALS AND MENUS
	// ═══════════════════════════════════════════════════════════════════════════

	// Help opens the help modal
	Help key.Binding

	// Model opens the model selection modal
	Model key.Binding

	// Theme opens the theme selection modal
	Theme key.Binding

	// Session opens the session management modal
	Session key.Binding

	// Device opens the audio device selection modal
	Device key.Binding

	// Close closes the current modal
	Close key.Binding

	// ═══════════════════════════════════════════════════════════════════════════
	// MODE SWITCHING
	// ═══════════════════════════════════════════════════════════════════════════

	// ModeNormal switches to normal mode
	ModeNormal key.Binding

	// ModeYolo switches to YOLO mode (auto-execute)
	ModeYolo key.Binding

	// ModePlan switches to plan mode (show execution plan)
	ModePlan key.Binding

	// ═══════════════════════════════════════════════════════════════════════════
	// CONVERSATION MANAGEMENT
	// ═══════════════════════════════════════════════════════════════════════════

	// Clear clears the conversation history
	Clear key.Binding

	// NewSession starts a new conversation session
	NewSession key.Binding

	// ═══════════════════════════════════════════════════════════════════════════
	// BLOCK NAVIGATION (CR-002)
	// ═══════════════════════════════════════════════════════════════════════════

	// BlockNext focuses the next block
	BlockNext key.Binding

	// BlockPrev focuses the previous block
	BlockPrev key.Binding

	// BlockChild focuses the first child / expands collapsed block
	BlockChild key.Binding

	// BlockParent focuses the parent / collapses block
	BlockParent key.Binding

	// ═══════════════════════════════════════════════════════════════════════════
	// BLOCK ACTIONS (CR-002)
	// ═══════════════════════════════════════════════════════════════════════════

	// BlockCopy copies the focused block content to clipboard
	BlockCopy key.Binding

	// BlockToggle toggles the focused block's collapsed state
	BlockToggle key.Binding

	// BlockBookmark toggles the focused block's bookmark
	BlockBookmark key.Binding

	// BlockRegenerate regenerates from the focused block
	BlockRegenerate key.Binding

	// BlockEdit edits the focused user block
	BlockEdit key.Binding

	// NextBookmark jumps to the next bookmarked block
	NextBookmark key.Binding

	// PrevBookmark jumps to the previous bookmarked block
	PrevBookmark key.Binding
}

// DefaultKeyMap returns the default keyboard shortcuts.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		// Core actions
		Send: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "send message"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "cancel/quit"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c", "ctrl+d"),
			key.WithHelp("ctrl+c/ctrl+d", "quit"),
		),

		// Navigation
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "scroll up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "scroll down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+u"),
			key.WithHelp("pgup/ctrl+u", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+d"),
			key.WithHelp("pgdn/ctrl+d", "page down"),
		),
		Top: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("home/g", "top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("end/G", "bottom"),
		),

		// Modals and menus
		Help: key.NewBinding(
			key.WithKeys("?", "f1"),
			key.WithHelp("?/f1", "help"),
		),
		Model: key.NewBinding(
			key.WithKeys("ctrl+m"),
			key.WithHelp("ctrl+m", "select model"),
		),
		Theme: key.NewBinding(
			key.WithKeys("ctrl+t"),
			key.WithHelp("ctrl+t", "select theme"),
		),
		Session: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "sessions"),
		),
		Device: key.NewBinding(
			key.WithKeys("ctrl+a"),
			key.WithHelp("ctrl+a", "audio devices"),
		),
		Close: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "close"),
		),

		// Mode switching
		ModeNormal: key.NewBinding(
			key.WithKeys("ctrl+n"),
			key.WithHelp("ctrl+n", "normal mode"),
		),
		ModeYolo: key.NewBinding(
			key.WithKeys("ctrl+y"),
			key.WithHelp("ctrl+y", "yolo mode"),
		),
		ModePlan: key.NewBinding(
			key.WithKeys("ctrl+p"),
			key.WithHelp("ctrl+p", "plan mode"),
		),

		// Conversation management
		Clear: key.NewBinding(
			key.WithKeys("ctrl+l"),
			key.WithHelp("ctrl+l", "clear history"),
		),
		NewSession: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("ctrl+r", "new session"),
		),

		// Block navigation (CR-002)
		// Note: j/k are also used for viewport scrolling. When block system is enabled,
		// these will focus blocks instead of scrolling.
		BlockNext: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j/↓", "next block"),
		),
		BlockPrev: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k/↑", "prev block"),
		),
		BlockChild: key.NewBinding(
			key.WithKeys("l", "right"),
			key.WithHelp("l/→", "expand/enter"),
		),
		BlockParent: key.NewBinding(
			key.WithKeys("h", "left"),
			key.WithHelp("h/←", "collapse/parent"),
		),

		// Block actions (CR-002)
		BlockCopy: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "copy block"),
		),
		BlockToggle: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "toggle collapse"),
		),
		BlockBookmark: key.NewBinding(
			key.WithKeys("b"),
			key.WithHelp("b", "bookmark"),
		),
		BlockRegenerate: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "regenerate"),
		),
		BlockEdit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit block"),
		),
		NextBookmark: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "next bookmark"),
		),
		PrevBookmark: key.NewBinding(
			key.WithKeys("N"),
			key.WithHelp("N", "prev bookmark"),
		),
	}
}

// ShortHelp returns a slice of key bindings to show in the short help view.
// This implements part of the help.KeyMap interface.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Send,
		k.Help,
		k.Quit,
	}
}

// FullHelp returns a slice of slices of key bindings to show in the full help view.
// This implements part of the help.KeyMap interface.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		// Column 1: Core actions
		{
			k.Send,
			k.Cancel,
			k.Quit,
		},
		// Column 2: Navigation
		{
			k.Up,
			k.Down,
			k.PageUp,
			k.PageDown,
			k.Top,
			k.Bottom,
		},
		// Column 3: Modals
		{
			k.Help,
			k.Model,
			k.Theme,
			k.Session,
			k.Device,
			k.Close,
		},
		// Column 4: Modes and management
		{
			k.ModeNormal,
			k.ModeYolo,
			k.ModePlan,
			k.Clear,
			k.NewSession,
		},
		// Column 5: Block navigation (CR-002)
		{
			k.BlockNext,
			k.BlockPrev,
			k.BlockChild,
			k.BlockParent,
		},
		// Column 6: Block actions (CR-002)
		{
			k.BlockCopy,
			k.BlockToggle,
			k.BlockBookmark,
			k.BlockRegenerate,
			k.BlockEdit,
			k.NextBookmark,
		},
	}
}
