package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/normanking/pinky/internal/permissions"
)

// Focus tracks which panel is active
type Focus int

const (
	FocusChat Focus = iota
	FocusApproval
	FocusHelp
	FocusSettings
)

// ChatMessage represents a message in the chat history
type ChatMessage struct {
	Role      string    // "user", "assistant", "tool"
	Content   string
	Timestamp time.Time
	ToolInfo  *ToolInfo // Optional tool execution info
}

// ToolInfo contains information about a tool execution
type ToolInfo struct {
	Name    string
	Command string
	Status  string // "pending", "running", "success", "failed", "awaiting"
	Output  string
}

// Model is the main TUI state
type Model struct {
	// Dimensions
	width  int
	height int

	// UI State
	focus       Focus
	verboseMode bool
	showHelp    bool
	ready       bool

	// Chat
	messages []ChatMessage
	viewport viewport.Model
	textarea textarea.Model

	// Approval dialog
	pendingApproval *permissions.ApprovalRequest
	alwaysAllow     bool // checkbox state
	allowDir        bool // checkbox state

	// Styling and keys
	styles Styles
	keys   KeyMap

	// Channels for async communication
	approvalChan chan<- *permissions.ApprovalResponse
	messageChan  chan<- string

	// Status
	channelStatus map[string]bool // channel name -> connected
	memoryCount   int

	// Settings panel state
	showSettings     bool
	settingsLane     int      // 0=fast, 1=balanced, 2=quality
	settingsModels   []string // available models for current lane
	settingsSelected int      // cursor position in model list
}

// NewModel creates a new TUI model
func NewModel() Model {
	ta := textarea.New()
	ta.Placeholder = "Type a message..."
	ta.Focus()
	ta.CharLimit = 4096
	ta.SetWidth(80)
	ta.SetHeight(1)
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false)

	vp := viewport.New(80, 20)
	vp.SetContent("")

	return Model{
		textarea:      ta,
		viewport:      vp,
		styles:        DefaultStyles(),
		keys:          DefaultKeyMap(),
		messages:      make([]ChatMessage, 0),
		channelStatus: make(map[string]bool),
		focus:         FocusChat,
	}
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m = m.updateDimensions()

	case ApprovalRequestMsg:
		m.pendingApproval = msg.Request
		m.focus = FocusApproval
		m.alwaysAllow = false
		m.allowDir = false

	case ToolStatusMsg:
		m = m.updateToolStatus(msg)

	case ChatResponseMsg:
		m.messages = append(m.messages, ChatMessage{
			Role:      "assistant",
			Content:   msg.Content,
			Timestamp: time.Now(),
		})
		m.viewport.SetContent(m.renderChat())
		m.viewport.GotoBottom()

	case SettingsToggleMsg:
		m.showSettings = !m.showSettings
		if m.showSettings {
			m.focus = FocusSettings
			m.settingsLane = 1 // Default to balanced
			m.settingsModels = []string{"claude-3-opus", "claude-3-sonnet", "claude-3-haiku", "gpt-4", "gpt-4-turbo", "ollama/llama3"}
			m.settingsSelected = 0
		} else {
			m.focus = FocusChat
		}
	}

	// Update textarea
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// handleKeyMsg processes keyboard input
func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle approval dialog keys first if in approval mode
	if m.focus == FocusApproval && m.pendingApproval != nil {
		return m.handleApprovalKeys(msg)
	}

	// Handle settings panel keys if in settings mode
	if m.focus == FocusSettings {
		return m.handleSettingsKeys(msg)
	}

	// Global keys
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		m.showHelp = !m.showHelp
		if m.showHelp {
			m.focus = FocusHelp
		} else {
			m.focus = FocusChat
		}
		return m, nil

	case key.Matches(msg, m.keys.Cancel):
		if m.focus == FocusHelp {
			m.showHelp = false
			m.focus = FocusChat
		}
		return m, nil

	case key.Matches(msg, m.keys.ToggleVerbose):
		m.verboseMode = !m.verboseMode
		return m, nil

	case key.Matches(msg, m.keys.Settings):
		m.showSettings = !m.showSettings
		if m.showSettings {
			m.focus = FocusSettings
			m.settingsLane = 1 // Default to balanced
			m.settingsModels = []string{"claude-3-opus", "claude-3-sonnet", "claude-3-haiku", "gpt-4", "gpt-4-turbo", "ollama/llama3"}
			m.settingsSelected = 0
		} else {
			m.focus = FocusChat
		}
		return m, nil

	case key.Matches(msg, m.keys.Clear):
		m.messages = make([]ChatMessage, 0)
		m.viewport.SetContent("")
		return m, nil

	case key.Matches(msg, m.keys.Send):
		return m.handleSend()
	}

	// Update textarea for other keys
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

// handleSettingsKeys handles key presses in settings mode
func (m Model) handleSettingsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Cancel):
		m.showSettings = false
		m.focus = FocusChat
		return m, nil

	case key.Matches(msg, m.keys.Up):
		if m.settingsSelected > 0 {
			m.settingsSelected--
		}
		return m, nil

	case key.Matches(msg, m.keys.Down):
		if m.settingsSelected < len(m.settingsModels)-1 {
			m.settingsSelected++
		}
		return m, nil

	case msg.String() == "left", msg.String() == "h":
		if m.settingsLane > 0 {
			m.settingsLane--
			m.settingsSelected = 0
		}
		return m, nil

	case msg.String() == "right", msg.String() == "l":
		if m.settingsLane < 2 {
			m.settingsLane++
			m.settingsSelected = 0
		}
		return m, nil

	case key.Matches(msg, m.keys.Send):
		// Select the current model
		if m.settingsSelected < len(m.settingsModels) {
			selectedModel := m.settingsModels[m.settingsSelected]
			m.messages = append(m.messages, ChatMessage{
				Role:      "tool",
				Content:   "Model changed to: " + selectedModel,
				Timestamp: time.Now(),
			})
		}
		m.showSettings = false
		m.focus = FocusChat
		m.viewport.SetContent(m.renderChat())
		m.viewport.GotoBottom()
		return m, nil
	}

	return m, nil
}

// handleApprovalKeys handles key presses in approval mode
func (m Model) handleApprovalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Approve):
		return m.respondToApproval(true, false)

	case key.Matches(msg, m.keys.Deny):
		return m.respondToApproval(false, false)

	case key.Matches(msg, m.keys.AlwaysAllow):
		return m.respondToApproval(true, true)

	case key.Matches(msg, m.keys.Cancel):
		return m.respondToApproval(false, false)

	case msg.String() == " ":
		// Toggle checkbox (cycle through options)
		if !m.alwaysAllow {
			m.alwaysAllow = true
		} else if !m.allowDir {
			m.allowDir = true
		} else {
			m.alwaysAllow = false
			m.allowDir = false
		}
		return m, nil
	}

	return m, nil
}

// respondToApproval sends the approval response
func (m Model) respondToApproval(approved bool, alwaysAllow bool) (tea.Model, tea.Cmd) {
	if m.pendingApproval == nil {
		return m, nil
	}

	response := &permissions.ApprovalResponse{
		Approved:    approved,
		AlwaysAllow: alwaysAllow || m.alwaysAllow,
		AllowDir:    m.allowDir,
	}

	// Add a message about the approval decision
	action := "denied"
	if approved {
		action = "approved"
		if response.AlwaysAllow {
			action = "approved (always allow)"
		}
	}

	m.messages = append(m.messages, ChatMessage{
		Role:      "tool",
		Content:   "Tool execution " + action,
		Timestamp: time.Now(),
		ToolInfo: &ToolInfo{
			Name:    m.pendingApproval.Tool,
			Command: m.pendingApproval.Command,
			Status:  action,
		},
	})

	// Send response through channel
	if m.approvalChan != nil {
		m.approvalChan <- response
	}

	// Clear approval state
	m.pendingApproval = nil
	m.focus = FocusChat
	m.alwaysAllow = false
	m.allowDir = false

	m.viewport.SetContent(m.renderChat())
	m.viewport.GotoBottom()

	return m, nil
}

// handleSend processes sending a message
func (m Model) handleSend() (tea.Model, tea.Cmd) {
	content := strings.TrimSpace(m.textarea.Value())
	if content == "" {
		return m, nil
	}

	// Add user message
	m.messages = append(m.messages, ChatMessage{
		Role:      "user",
		Content:   content,
		Timestamp: time.Now(),
	})

	// Clear input
	m.textarea.Reset()

	// Update viewport
	m.viewport.SetContent(m.renderChat())
	m.viewport.GotoBottom()

	// Send message to handler
	if m.messageChan != nil {
		return m, func() tea.Msg {
			m.messageChan <- content
			return nil
		}
	}

	return m, nil
}

// updateDimensions recalculates component sizes
func (m Model) updateDimensions() Model {
	headerHeight := 2
	statusHeight := 1
	inputHeight := 3
	padding := 2

	chatHeight := m.height - headerHeight - statusHeight - inputHeight - padding

	m.viewport.Width = m.width - 4
	m.viewport.Height = chatHeight

	m.textarea.SetWidth(m.width - 4)

	return m
}

// updateToolStatus updates a tool's status in the chat
func (m Model) updateToolStatus(msg ToolStatusMsg) Model {
	// Find and update the tool status in messages
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].ToolInfo != nil && m.messages[i].ToolInfo.Name == msg.Tool {
			m.messages[i].ToolInfo.Status = msg.Status
			m.messages[i].ToolInfo.Output = msg.Output
			break
		}
	}
	m.viewport.SetContent(m.renderChat())
	return m
}

// renderChat renders the chat messages for the viewport
func (m Model) renderChat() string {
	var sb strings.Builder

	for _, msg := range m.messages {
		switch msg.Role {
		case "user":
			sb.WriteString(m.styles.UserMsg.Render("You: "))
			sb.WriteString(msg.Content)
		case "assistant":
			sb.WriteString(m.styles.BotMsg.Render("Pinky: "))
			sb.WriteString(msg.Content)
		case "tool":
			icon := ">"
			if msg.ToolInfo != nil {
				switch msg.ToolInfo.Status {
				case "success", "approved", "approved (always allow)":
					icon = "[OK]"
				case "failed", "denied":
					icon = "[X]"
				case "running":
					icon = "[...]"
				case "awaiting":
					icon = "[?]"
				}
			}
			sb.WriteString(m.styles.ToolStatus.Render(icon + " " + msg.Content))
		}
		sb.WriteString("\n\n")
	}

	return sb.String()
}

// SetApprovalChannel sets the channel for sending approval responses
func (m *Model) SetApprovalChannel(ch chan<- *permissions.ApprovalResponse) {
	m.approvalChan = ch
}

// SetMessageChannel sets the channel for sending user messages
func (m *Model) SetMessageChannel(ch chan<- string) {
	m.messageChan = ch
}

// SetChannelStatus updates the status of a channel
func (m *Model) SetChannelStatus(name string, connected bool) {
	m.channelStatus[name] = connected
}

// SetMemoryCount updates the memory count display
func (m *Model) SetMemoryCount(count int) {
	m.memoryCount = count
}

// Message types for tea.Msg

// ApprovalRequestMsg signals that an approval is needed
type ApprovalRequestMsg struct {
	Request *permissions.ApprovalRequest
}

// ToolStatusMsg updates tool execution status
type ToolStatusMsg struct {
	Tool   string
	Status string
	Output string
}

// ChatResponseMsg contains a response from the brain
type ChatResponseMsg struct {
	Content string
}

// SettingsToggleMsg toggles the settings panel visibility
type SettingsToggleMsg struct{}
