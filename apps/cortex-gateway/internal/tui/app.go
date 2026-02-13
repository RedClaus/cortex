package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/bubbles/key"
)

type Panel int

const (
	ChatPanel Panel = iota
	StatusPanel
	NeuralPanel
)

type App struct {
	width, height int
	currentPanel  Panel
	chat          *Chat
	status        *Status
	neural        *Neural
	input         *Input
	keys          KeyMap
}

func NewApp() *App {
	return &App{
		currentPanel: ChatPanel,
		chat:         NewChat(),
		status:       NewStatus(),
		neural:       NewNeural(),
		input:        NewInput(),
		keys:         DefaultKeyMap,
	}
}

func (a *App) Init() tea.Cmd {
	return tea.Batch(a.chat.Init(), a.status.Init(), a.neural.Init(), a.input.Init())
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, a.keys.Quit):
			return a, tea.Quit
		case key.Matches(msg, a.keys.Tab):
			a.currentPanel = (a.currentPanel + 1) % 3
		case key.Matches(msg, a.keys.Command):
			// TODO: command mode
		case msg.String() == "enter":
			if a.input.Value() != "" {
				a.chat.AddMessage("user", a.input.Value())
				a.input.Reset()
				// Simulate response
				a.chat.AddMessage("assistant", "Echo: "+a.input.Value())
			}
		}
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.resize()
	}

	// Update submodels
	var cmd tea.Cmd
	a.chat, cmd = a.chat.Update(msg)
	cmds = append(cmds, cmd)
	a.status, cmd = a.status.Update(msg)
	cmds = append(cmds, cmd)
	a.neural, cmd = a.neural.Update(msg)
	cmds = append(cmds, cmd)
	a.input, cmd = a.input.Update(msg)
	cmds = append(cmds, cmd)

	return a, tea.Batch(cmds...)
}

func (a *App) View() string {
	if a.width == 0 || a.height == 0 {
		return "Initializing..."
	}

	statusBar := a.statusBarView()
	inputBar := a.input.View()

	contentHeight := a.height - lipgloss.Height(statusBar) - lipgloss.Height(inputBar)

	leftWidth := int(float64(a.width) * 0.7)
	rightWidth := a.width - leftWidth

	chatView := a.chat.View(leftWidth, contentHeight)
	var rightView string
	switch a.currentPanel {
	case StatusPanel:
		rightView = a.status.View(rightWidth, contentHeight)
	case NeuralPanel:
		rightView = a.neural.View(rightWidth, contentHeight)
	default:
		rightView = a.status.View(rightWidth, contentHeight)
	}

	layout := lipgloss.JoinHorizontal(lipgloss.Top, chatView, rightView)

	return lipgloss.JoinVertical(lipgloss.Left, statusBar, layout, inputBar)
}

func (a *App) statusBarView() string {
	version := "v1.0.0"
	uptime := "00:00:00"
	channel := "webchat"
	return StatusBarStyle.Width(a.width).Render(fmt.Sprintf("Cortex-Gateway %s | Uptime: %s | Channel: %s", version, uptime, channel))
}

func (a *App) resize() {
	// Resize submodels if needed
}
