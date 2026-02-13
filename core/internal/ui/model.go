// Package ui provides the Charmbracelet TUI framework integration for Cortex.
package ui

import (
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/normanking/cortex/internal/ui/block"
	"github.com/normanking/cortex/internal/ui/modals"
)

// Mode represents the current operational mode of the TUI.
type Mode int

const (
	// ModeNormal is the standard chat/interaction mode
	ModeNormal Mode = iota

	// ModeYolo is the auto-execute mode without confirmations
	ModeYolo

	// ModePlan is the planning mode that shows execution plan before running
	ModePlan
)

// String returns the string representation of the mode.
func (m Mode) String() string {
	switch m {
	case ModeNormal:
		return "Normal"
	case ModeYolo:
		return "YOLO"
	case ModePlan:
		return "Plan"
	default:
		return "Unknown"
	}
}

// ModalType represents different modal states
type ModalType int

const (
	ModalNone ModalType = iota
	ModalHelp
	ModalModel
	ModalTheme
	ModalSession
	ModalConfirm
	ModalDevice // Audio device selection (microphone/speaker)
)

// Note: Message struct is now defined in message.go with enhanced streaming support

// Model is the main Bubble Tea model for the Cortex TUI.
// It implements the tea.Model interface and follows Elm Architecture principles.
type Model struct {
	// ═══════════════════════════════════════════════════════════════════════════
	// DIMENSIONS AND LAYOUT
	// ═══════════════════════════════════════════════════════════════════════════

	// width is the current terminal width
	width int

	// height is the current terminal height
	height int

	// ready indicates if the terminal has been sized (initial WindowSizeMsg received)
	ready bool

	// ═══════════════════════════════════════════════════════════════════════════
	// THEME AND STYLES
	// ═══════════════════════════════════════════════════════════════════════════

	// themeName is the name of the current theme
	themeName string

	// styles contains all lipgloss styles for rendering
	styles Styles

	// ═══════════════════════════════════════════════════════════════════════════
	// MODE AND STATE
	// ═══════════════════════════════════════════════════════════════════════════

	// mode is the current operational mode (Normal, Yolo, Plan)
	mode Mode

	// activeModal is the currently displayed modal (None, Help, Model, etc.)
	activeModal ModalType

	// ═══════════════════════════════════════════════════════════════════════════
	// STREAMING STATE
	// ═══════════════════════════════════════════════════════════════════════════

	// isStreaming indicates if an AI response is currently being streamed
	isStreaming bool

	// streamBuffer accumulates chunks during streaming
	streamBuffer string

	// activeMessage points to the currently streaming assistant message
	activeMessage *Message

	// lastRenderTime tracks when Glamour last rendered (for debouncing)
	lastRenderTime time.Time

	// ═══════════════════════════════════════════════════════════════════════════
	// MESSAGES AND HISTORY
	// ═══════════════════════════════════════════════════════════════════════════

	// messages is the conversation history
	messages []*Message

	// ═══════════════════════════════════════════════════════════════════════════
	// BUBBLE TEA COMPONENTS
	// ═══════════════════════════════════════════════════════════════════════════

	// viewport is the scrollable message view area
	viewport viewport.Model

	// input is the multi-line text input area
	input textarea.Model

	// spinner is the loading indicator
	spinner spinner.Model

	// help is the help/keybindings component
	help help.Model

	// keys contains the key bindings
	keys KeyMap

	// ═══════════════════════════════════════════════════════════════════════════
	// BACKEND INTERFACE
	// ═══════════════════════════════════════════════════════════════════════════

	// backend is the interface to the Cortex orchestrator/LLM
	backend Backend

	// ═══════════════════════════════════════════════════════════════════════════
	// SESSION AND MODEL INFO
	// ═══════════════════════════════════════════════════════════════════════════

	// currentModel is the currently selected AI model
	currentModel string

	// currentProvider is the provider of the current model
	currentProvider string

	// sessionID is the current conversation session identifier
	sessionID string

	// ═══════════════════════════════════════════════════════════════════════════
	// ERROR STATE
	// ═══════════════════════════════════════════════════════════════════════════

	// err stores the last error that occurred
	err error

	// ═══════════════════════════════════════════════════════════════════════════
	// BLOCK SYSTEM (CR-002)
	// ═══════════════════════════════════════════════════════════════════════════

	// useBlockSystem indicates whether to use the block-based conversation model
	useBlockSystem bool

	// blockContainer manages the hierarchical block structure
	blockContainer *block.BlockContainer

	// branchManager handles conversation branching and forking
	branchManager *block.BranchManager

	// activeBlock points to the currently streaming block (when using block system)
	activeBlock *block.Block

	// blockNavigator handles block focus and navigation
	blockNavigator *block.BlockNavigator

	// actionExecutor handles block actions (copy, toggle, etc.)
	actionExecutor *block.ActionExecutor

	// ═══════════════════════════════════════════════════════════════════════════
	// AUDIO DEVICE SELECTOR
	// ═══════════════════════════════════════════════════════════════════════════

	// deviceSelector is the audio device selection modal
	deviceSelector *modals.DeviceSelector
}

// NOTE: StyleSet was removed - use Styles from styles.go instead

// Init initializes the model and returns the initial command.
// This implements the tea.Model interface.
func (m Model) Init() tea.Cmd {
	// Start the spinner animation
	return m.spinner.Tick
}

// Update handles messages and updates the model state.
// This implements the tea.Model interface.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Delegate to update.go for the actual implementation
	return update(m, msg)
}

// View renders the model to a string.
// This implements the tea.Model interface.
func (m Model) View() string {
	// Delegate to view.go for the actual implementation
	return view(m)
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPER METHODS
// ═══════════════════════════════════════════════════════════════════════════════

// AddMessage adds a new message to the conversation history.
func (m *Model) AddMessage(msg *Message) {
	m.messages = append(m.messages, msg)
}

// AddUserMessage is a convenience method for adding a user message.
func (m *Model) AddUserMessage(content string) {
	m.AddMessage(NewUserMessage(content))
}

// AddAssistantMessage is a convenience method for adding an assistant message.
func (m *Model) AddAssistantMessage(content string) {
	msg := NewAssistantMessage()
	msg.RawContent = content
	msg.MarkComplete()
	m.AddMessage(msg)
}

// AddSystemMessage is a convenience method for adding a system message.
func (m *Model) AddSystemMessage(content string) {
	m.AddMessage(NewSystemMessage(content))
}

// AddErrorMessage is a convenience method for adding an error message.
func (m *Model) AddErrorMessage(content string) {
	msg := NewSystemMessage(content)
	msg.MarkError(nil)
	m.AddMessage(msg)
}

// ClearMessages clears all messages from the conversation history.
func (m *Model) ClearMessages() {
	m.messages = []*Message{}
}

// SetMode sets the operational mode.
func (m *Model) SetMode(mode Mode) {
	m.mode = mode
}

// SetModal sets the active modal.
func (m *Model) SetModal(modal ModalType) {
	m.activeModal = modal
}
