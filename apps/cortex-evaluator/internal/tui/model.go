package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cortex-evaluator/cortex-evaluator/internal/brainstorm"
	"github.com/cortex-evaluator/cortex-evaluator/internal/session"
)

// View represents the current view/screen
type View int

const (
	ViewSessionList View = iota
	ViewChat
	ViewHelp
	ViewSettings
)

// Provider represents an LLM provider
type Provider struct {
	Name   string
	Models []string
}

// Available providers and their models
var providers = []Provider{
	{Name: "openai", Models: []string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-3.5-turbo"}},
	{Name: "anthropic", Models: []string{"claude-sonnet-4-20250514", "claude-3-5-sonnet-20241022", "claude-3-haiku-20240307", "claude-3-opus-20240229"}},
	{Name: "gemini", Models: []string{"gemini-1.5-pro", "gemini-1.5-flash", "gemini-pro"}},
	{Name: "groq", Models: []string{"llama-3.3-70b-versatile", "llama-3.1-8b-instant", "mixtral-8x7b-32768"}},
	{Name: "ollama", Models: []string{"llama3.2", "llama3.1", "codellama", "mistral", "deepseek-coder", "qwen2.5-coder"}},
}

// Model is the main TUI model
type Model struct {
	// Core state
	store         session.Store
	sessions      []*session.Session
	currentIdx    int
	activeSession *session.Session
	engine        *brainstorm.Engine

	// UI components
	viewport viewport.Model
	textarea textarea.Model

	// Messages
	messages []brainstorm.Message

	// Provider configuration
	providerIdx int
	modelIdx    int

	// View state
	view        View
	width       int
	height      int
	ready       bool
	err         error
	statusMsg   string
	indexing    bool
	indexStatus string
}

// sessionLoadedMsg is sent when sessions are loaded
type sessionLoadedMsg struct {
	sessions []*session.Session
	err      error
}

// indexCompleteMsg is sent when indexing completes
type indexCompleteMsg struct {
	err error
}

// indexProgressMsg is sent during indexing
type indexProgressMsg struct {
	status string
}

// NewModel creates a new TUI model
func NewModel(sess *session.Session, store session.Store) Model {
	ta := textarea.New()
	ta.Placeholder = "Ask a question about your codebase..."
	ta.Focus()
	ta.Prompt = "│ "
	ta.CharLimit = 2000
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.ShowLineNumbers = false

	m := Model{
		store:    store,
		textarea: ta,
		view:     ViewSessionList,
		messages: make([]brainstorm.Message, 0),
	}

	if sess != nil {
		m.activeSession = sess
		m.view = ViewChat
	}

	return m
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.loadSessions,
	)
}

// loadSessions loads sessions from the store
func (m Model) loadSessions() tea.Msg {
	sessions, err := m.store.List(false)
	return sessionLoadedMsg{sessions: sessions, err: err}
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.view == ViewHelp {
				m.view = ViewChat
				return m, nil
			}
			if m.view == ViewSessionList {
				return m, tea.Quit
			}
			// In chat view, q quits if not typing
			if !m.textarea.Focused() {
				return m, tea.Quit
			}

		case "esc":
			if m.view == ViewChat {
				m.view = ViewSessionList
				m.activeSession = nil
				m.engine = nil
				m.messages = nil
				return m, nil
			}
			if m.view == ViewHelp {
				m.view = ViewChat
				return m, nil
			}
			if m.view == ViewSettings {
				m.view = ViewChat
				return m, nil
			}

		case "?":
			if m.view != ViewHelp && !m.textarea.Focused() {
				m.view = ViewHelp
				return m, nil
			}

		case "enter":
			if m.view == ViewSessionList && len(m.sessions) > 0 {
				m.activeSession = m.sessions[m.currentIdx]
				m.view = ViewChat
				m.statusMsg = "Loading session..."
				return m, m.initSession
			}
			if m.view == ViewChat && m.textarea.Focused() {
				question := strings.TrimSpace(m.textarea.Value())
				if question != "" {
					m.textarea.Reset()
					return m, m.askQuestion(question)
				}
			}

		case "up", "k":
			if m.view == ViewSessionList && m.currentIdx > 0 {
				m.currentIdx--
			}

		case "down", "j":
			if m.view == ViewSessionList && m.currentIdx < len(m.sessions)-1 {
				m.currentIdx++
			}

		case "tab":
			if m.view == ViewChat {
				if m.textarea.Focused() {
					m.textarea.Blur()
				} else {
					m.textarea.Focus()
				}
			}

		case "p":
			// Cycle through providers (when not typing)
			if m.view == ViewChat && !m.textarea.Focused() {
				m.providerIdx = (m.providerIdx + 1) % len(providers)
				m.modelIdx = 0 // Reset model when changing provider
				m.statusMsg = fmt.Sprintf("Provider: %s | Model: %s", providers[m.providerIdx].Name, providers[m.providerIdx].Models[m.modelIdx])
			}

		case "m":
			// Cycle through models (when not typing)
			if m.view == ViewChat && !m.textarea.Focused() {
				models := providers[m.providerIdx].Models
				m.modelIdx = (m.modelIdx + 1) % len(models)
				m.statusMsg = fmt.Sprintf("Provider: %s | Model: %s", providers[m.providerIdx].Name, models[m.modelIdx])
			}

		case "s":
			// Show settings view (when not typing)
			if m.view == ViewChat && !m.textarea.Focused() {
				m.view = ViewSettings
				return m, nil
			}
			if m.view == ViewSettings {
				m.view = ViewChat
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

		headerHeight := 3
		footerHeight := 5
		inputHeight := 5

		if m.view == ViewChat {
			m.viewport = viewport.New(msg.Width-4, msg.Height-headerHeight-footerHeight-inputHeight)
			m.viewport.YPosition = headerHeight
		}

		m.textarea.SetWidth(msg.Width - 4)

	case sessionLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.sessions = msg.sessions
		}

	case indexCompleteMsg:
		m.indexing = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Indexing failed: %v", msg.err)
		} else {
			m.statusMsg = "Ready! Ask a question about your codebase."
		}

	case indexProgressMsg:
		m.indexStatus = msg.status
	}

	// Update textarea
	if m.view == ViewChat {
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Update viewport
	if m.view == ViewChat && m.ready {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// initSession initializes the brainstorm engine for the active session
func (m Model) initSession() tea.Msg {
	if m.activeSession == nil {
		return indexCompleteMsg{err: fmt.Errorf("no session selected")}
	}

	engine, err := brainstorm.NewEngine(m.activeSession, brainstorm.DefaultOptions())
	if err != nil {
		return indexCompleteMsg{err: err}
	}

	// Index the project
	if err := engine.IndexProject(); err != nil {
		return indexCompleteMsg{err: err}
	}

	m.engine = engine
	return indexCompleteMsg{err: nil}
}

// askQuestion sends a question to the engine
func (m Model) askQuestion(question string) tea.Cmd {
	return func() tea.Msg {
		if m.engine == nil {
			return indexCompleteMsg{err: fmt.Errorf("engine not initialized")}
		}

		resp, err := m.engine.Ask(question)
		if err != nil {
			return indexCompleteMsg{err: err}
		}

		// The engine already tracks history, so we just need to update our view
		_ = resp
		return indexCompleteMsg{err: nil}
	}
}

// View implements tea.Model
func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	switch m.view {
	case ViewSessionList:
		return m.viewSessionList()
	case ViewChat:
		return m.viewChat()
	case ViewHelp:
		return m.viewHelp()
	case ViewSettings:
		return m.viewSettings()
	default:
		return "Unknown view"
	}
}

func (m Model) viewSessionList() string {
	var b strings.Builder

	b.WriteString(Logo())
	b.WriteString("\n\n")

	if len(m.sessions) == 0 {
		b.WriteString(dimStyle.Render("No sessions found.\n"))
		b.WriteString(dimStyle.Render("Create one with: evaluator new [name] [path]\n"))
	} else {
		b.WriteString(titleStyle.Render("Sessions"))
		b.WriteString("\n\n")

		for i, sess := range m.sessions {
			cursor := "  "
			style := normalSessionStyle
			if i == m.currentIdx {
				cursor = "▸ "
				style = selectedSessionStyle
			}
			if sess.IsArchived() {
				style = archivedSessionStyle
			}

			line := fmt.Sprintf("%s%s", cursor, sess.Name)
			b.WriteString(style.Render(line))
			b.WriteString("\n")
			b.WriteString(dimStyle.Render(fmt.Sprintf("    %s", sess.ProjectPath)))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓: navigate • enter: select • q: quit"))

	return b.String()
}

func (m Model) viewChat() string {
	var b strings.Builder

	// Header
	sessionName := "No Session"
	if m.activeSession != nil {
		sessionName = m.activeSession.Name
	}
	header := titleStyle.Render("Cortex Evaluator") + " " + dimStyle.Render("- "+sessionName)
	b.WriteString(header)
	b.WriteString("\n")

	// Provider/Model indicator
	provider := providers[m.providerIdx]
	model := provider.Models[m.modelIdx]
	providerInfo := fmt.Sprintf("Provider: %s | Model: %s", provider.Name, model)
	b.WriteString(contextHeaderStyle.Render(providerInfo))
	b.WriteString("\n\n")

	// Messages area
	if m.engine != nil {
		messages := m.engine.History()
		var msgContent strings.Builder

		if len(messages) == 0 {
			msgContent.WriteString(dimStyle.Render("No messages yet. Ask a question!\n"))
		} else {
			for _, msg := range messages {
				switch msg.Role {
				case brainstorm.RoleUser:
					msgContent.WriteString(userMessageStyle.Render("You: "))
					msgContent.WriteString(msg.Content)
				case brainstorm.RoleAssistant:
					msgContent.WriteString(assistantMessageStyle.Render("Assistant: "))
					msgContent.WriteString(msg.Content)
				}
				msgContent.WriteString("\n\n")
			}
		}

		m.viewport.SetContent(msgContent.String())
		b.WriteString(chatBoxStyle.Width(m.width - 4).Render(m.viewport.View()))
	} else if m.indexing {
		b.WriteString(dimStyle.Render("Indexing project... " + m.indexStatus))
	} else {
		b.WriteString(dimStyle.Render(m.statusMsg))
	}

	b.WriteString("\n\n")

	// Input area
	b.WriteString(inputPromptStyle.Render("Question:"))
	b.WriteString("\n")
	b.WriteString(inputStyle.Width(m.width - 4).Render(m.textarea.View()))
	b.WriteString("\n")

	// Status bar with provider shortcuts
	status := statusKeyStyle.Render(" CHAT ") +
		statusValueStyle.Render(" p: provider • m: model • s: settings • esc: back • tab: focus • ctrl+c: quit ")
	b.WriteString(status)

	return b.String()
}

func (m Model) viewHelp() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Help"))
	b.WriteString("\n\n")

	help := `
Keyboard Shortcuts:

  Session List:
    ↑/k        Move up
    ↓/j        Move down
    enter      Select session
    q          Quit

  Chat View:
    tab        Toggle input focus
    enter      Send question
    p          Cycle providers
    m          Cycle models
    s          Settings view
    esc        Back to sessions
    ?          Show this help
    ctrl+c     Quit

  General:
    q          Quit (when not typing)
    ?          Show help

Commands (CLI):
    evaluator new [name] [path]    Create new session
    evaluator list                 List sessions
    evaluator resume [id]          Resume session
    evaluator archive [id]         Archive session
    evaluator tui                  Launch TUI
`
	b.WriteString(help)
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Press esc or ? to close"))

	return b.String()
}

func (m Model) viewSettings() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Settings"))
	b.WriteString("\n\n")

	// Current provider
	b.WriteString(contextHeaderStyle.Render("AI Provider Configuration"))
	b.WriteString("\n\n")

	for i, p := range providers {
		prefix := "  "
		style := normalSessionStyle
		if i == m.providerIdx {
			prefix = "▸ "
			style = selectedSessionStyle
		}
		b.WriteString(style.Render(fmt.Sprintf("%s%s", prefix, p.Name)))
		b.WriteString("\n")

		// Show models for selected provider
		if i == m.providerIdx {
			for j, model := range p.Models {
				modelPrefix := "    "
				modelStyle := dimStyle
				if j == m.modelIdx {
					modelPrefix = "  → "
					modelStyle = successStyle
				}
				b.WriteString(modelStyle.Render(fmt.Sprintf("%s%s", modelPrefix, model)))
				b.WriteString("\n")
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Note: Configure API keys via environment variables:"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  OPENAI_API_KEY, ANTHROPIC_API_KEY, GEMINI_API_KEY, GROQ_API_KEY"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  OLLAMA_URL (default: http://localhost:11434)"))
	b.WriteString("\n\n")

	status := statusKeyStyle.Render(" SETTINGS ") +
		statusValueStyle.Render(" p: provider • m: model • s/esc: back to chat ")
	b.WriteString(status)

	return b.String()
}

// Run starts the TUI application
func Run(sess *session.Session, store session.Store) error {
	m := NewModel(sess, store)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}
