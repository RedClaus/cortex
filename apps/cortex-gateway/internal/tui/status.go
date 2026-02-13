package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

type Status struct {
	cortexConnected bool
	ollamaModels    []string
	activeLane      string
	activeSessions  int
	memoryStats     string
}

func NewStatus() *Status {
	return &Status{
		cortexConnected: true,
		ollamaModels:    []string{"llama2", "codellama"},
		activeLane:      "default",
		activeSessions:  1,
		memoryStats:     "512MB / 1GB",
	}
}

func (s *Status) Init() tea.Cmd {
	return nil
}

func (s *Status) Update(msg tea.Msg) (*Status, tea.Cmd) {
	return s, nil
}

func (s *Status) View(width, height int) string {
	content := fmt.Sprintf(
		"CortexBrain: %s\nOllama Models: %s\nActive Lane: %s\nSessions: %d\nMemory: %s",
		map[bool]string{true: "Connected", false: "Disconnected"}[s.cortexConnected],
		fmt.Sprintf("%v", s.ollamaModels),
		s.activeLane,
		s.activeSessions,
		s.memoryStats,
	)
	return StatusPanelStyle.Width(width).Height(height).Render(content)
}
