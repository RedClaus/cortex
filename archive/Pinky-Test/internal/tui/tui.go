package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/normanking/pinky/internal/config"
	"github.com/normanking/pinky/internal/permissions"
)

// TUI is the terminal user interface for Pinky
type TUI struct {
	program *tea.Program
	model   *Model

	// Channels for communication with the agent loop
	approvalChan  chan *permissions.ApprovalResponse
	messageChan   chan string
	responseChan  chan string

	// Configuration
	config     *config.Config
	configPath string

	// Brain for lane management
	laneManager LaneManager
}

// Options configures the TUI
type Options struct {
	Config      *config.Config
	ConfigPath  string
	LaneManager LaneManager
}

// New creates a new TUI instance
func New(opts Options) *TUI {
	model := NewModel()

	// Get config path
	configPath := opts.ConfigPath
	if configPath == "" {
		home, _ := os.UserHomeDir()
		configPath = filepath.Join(home, ".pinky", "config.yaml")
	}

	t := &TUI{
		model:        &model,
		config:       opts.Config,
		configPath:   configPath,
		laneManager:  opts.LaneManager,
		approvalChan: make(chan *permissions.ApprovalResponse, 1),
		messageChan:  make(chan string, 10),
		responseChan: make(chan string, 10),
	}

	// Wire up channels
	model.SetApprovalChannel(t.approvalChan)
	model.SetMessageChannel(t.messageChan)
	model.SetConfig(opts.Config, configPath)

	// Initialize settings panel if lane manager is available
	if opts.LaneManager != nil {
		settingsPanel := NewSettingsPanel(opts.LaneManager, opts.Config, configPath)
		model.SetSettingsPanel(settingsPanel)
	}

	t.model = &model

	return t
}

// Run starts the TUI and blocks until it exits
func (t *TUI) Run(ctx context.Context) error {
	t.program = tea.NewProgram(
		*t.model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	// Handle context cancellation
	go func() {
		<-ctx.Done()
		t.program.Quit()
	}()

	// Run the program
	finalModel, err := t.program.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	// Update our reference to the final model
	if m, ok := finalModel.(Model); ok {
		t.model = &m
	}

	return nil
}

// RequestApproval sends an approval request to the TUI and waits for response
func (t *TUI) RequestApproval(req *permissions.ApprovalRequest) *permissions.ApprovalResponse {
	if t.program == nil {
		return &permissions.ApprovalResponse{Approved: false}
	}

	// Send the approval request to the TUI
	t.program.Send(ApprovalRequestMsg{Request: req})

	// Wait for response
	return <-t.approvalChan
}

// SendResponse sends a response from the brain to display
func (t *TUI) SendResponse(content string) {
	if t.program != nil {
		t.program.Send(ChatResponseMsg{Content: content})
	}
}

// UpdateToolStatus updates the status of a tool execution
func (t *TUI) UpdateToolStatus(tool, status, output string) {
	if t.program != nil {
		t.program.Send(ToolStatusMsg{
			Tool:   tool,
			Status: status,
			Output: output,
		})
	}
}

// Messages returns a channel of user messages
func (t *TUI) Messages() <-chan string {
	return t.messageChan
}

// SetChannelStatus updates a channel's connection status
func (t *TUI) SetChannelStatus(name string, connected bool) {
	t.model.SetChannelStatus(name, connected)
}

// SetMemoryCount updates the memory count display
func (t *TUI) SetMemoryCount(count int) {
	t.model.SetMemoryCount(count)
}

// ShowSettings opens the settings panel
func (t *TUI) ShowSettings() {
	if t.program != nil && t.model != nil {
		t.model.ShowSettings()
	}
}
