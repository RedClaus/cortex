// Package ui provides the Charmbracelet TUI framework integration for Cortex.
package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/normanking/cortex/internal/ui/block"
	"github.com/normanking/cortex/internal/ui/modals"
)

// ═══════════════════════════════════════════════════════════════════════════════
// APPLICATION CONFIGURATION
// ═══════════════════════════════════════════════════════════════════════════════

// Config holds configuration options for initializing the TUI.
type Config struct {
	// Backend is the interface to the Cortex orchestrator
	Backend Backend

	// Theme is the name of the theme to use (defaults to "default")
	Theme string

	// InitialMode is the starting operational mode (defaults to ModeNormal)
	InitialMode Mode

	// EnableMouseSupport enables mouse interactions
	EnableMouseSupport bool

	// CurrentModel is the default AI model to use
	CurrentModel string

	// CurrentProvider is the provider of the default model
	CurrentProvider string

	// SessionID is the conversation session identifier
	SessionID string

	// UseBlockSystem enables the hierarchical block-based conversation model
	// When false, uses the legacy flat message system for backward compatibility
	UseBlockSystem bool
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Theme:              "default",
		InitialMode:        ModeNormal,
		EnableMouseSupport: false,
		CurrentModel:       "",
		CurrentProvider:    "",
		SessionID:          "",
		UseBlockSystem:     false, // Disabled by default for gradual rollout
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// APPLICATION INITIALIZATION
// ═══════════════════════════════════════════════════════════════════════════════

// New creates a new TUI application with the given configuration.
// This is the main entry point for initializing the Cortex TUI.
func New(cfg *Config) (*tea.Program, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// Initialize the model
	model, err := newModel(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize model: %w", err)
	}

	// Create program options
	opts := []tea.ProgramOption{
		tea.WithAltScreen(),       // Use alternate screen buffer
		tea.WithMouseCellMotion(), // Enable mouse support if configured
	}

	// Create and return the Bubble Tea program
	prog := tea.NewProgram(model, opts...)

	return prog, nil
}

// newModel creates and initializes the Model struct with all components.
func newModel(cfg *Config) (Model, error) {
	// ═══════════════════════════════════════════════════════════════════════════
	// INITIALIZE BUBBLE TEA COMPONENTS
	// ═══════════════════════════════════════════════════════════════════════════

	// Viewport for message history
	vp := viewport.New(0, 0) // Dimensions will be set on first WindowSizeMsg
	vp.HighPerformanceRendering = true
	vp.SetContent("") // Start with empty content

	// Textarea for user input
	ti := textarea.New()
	ti.Placeholder = "Type a message... (Enter to send, Ctrl+C to quit)"
	ti.Focus()
	ti.CharLimit = 4000 // Reasonable limit for input
	ti.SetHeight(3)     // Multi-line input
	ti.ShowLineNumbers = false
	ti.KeyMap.InsertNewline.SetEnabled(false) // Disable newline on Enter

	// Spinner for loading states
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	// Help component
	h := help.New()
	h.ShowAll = false // Start with short help

	// ═══════════════════════════════════════════════════════════════════════════
	// INITIALIZE THEME AND STYLES
	// ═══════════════════════════════════════════════════════════════════════════

	themeName := cfg.Theme
	if themeName == "" {
		themeName = "default"
	}

	theme := GetTheme(themeName)
	styles := createStyleSet(theme)

	// ═══════════════════════════════════════════════════════════════════════════
	// INITIALIZE BLOCK SYSTEM (CR-002)
	// ═══════════════════════════════════════════════════════════════════════════

	var blockContainer *block.BlockContainer
	var branchManager *block.BranchManager
	var blockNavigator *block.BlockNavigator
	var actionExecutor *block.ActionExecutor

	if cfg.UseBlockSystem {
		blockContainer = block.NewBlockContainer()
		branchManager = block.NewBranchManager(blockContainer)
		blockNavigator = block.NewBlockNavigator(blockContainer)
		actionExecutor = block.NewActionExecutor(blockContainer, branchManager)
	}

	// ═══════════════════════════════════════════════════════════════════════════
	// INITIALIZE DEVICE SELECTOR
	// ═══════════════════════════════════════════════════════════════════════════

	deviceSelector := modals.NewDeviceSelector()

	// ═══════════════════════════════════════════════════════════════════════════
	// CREATE MODEL
	// ═══════════════════════════════════════════════════════════════════════════

	m := Model{
		// Dimensions (will be set on first WindowSizeMsg)
		width:  0,
		height: 0,
		ready:  false,

		// Theme and styles
		themeName: themeName,
		styles:    styles,

		// Mode and state
		mode:        cfg.InitialMode,
		activeModal: ModalNone,

		// Streaming state
		isStreaming:  false,
		streamBuffer: "",

		// Messages
		messages: []*Message{},

		// Components
		viewport: vp,
		input:    ti,
		spinner:  sp,
		help:     h,
		keys:     DefaultKeyMap(),

		// Backend
		backend: cfg.Backend,

		// Session info
		currentModel:    cfg.CurrentModel,
		currentProvider: cfg.CurrentProvider,
		sessionID:       cfg.SessionID,

		// Error state
		err: nil,

		// Block system (CR-002)
		useBlockSystem: cfg.UseBlockSystem,
		blockContainer: blockContainer,
		branchManager:  branchManager,
		blockNavigator: blockNavigator,
		actionExecutor: actionExecutor,

		// Audio device selector
		deviceSelector: deviceSelector,
	}

	return m, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// STYLE SET CREATION
// ═══════════════════════════════════════════════════════════════════════════════

// createStyleSet creates a Styles instance based on the given theme.
// This is a simple wrapper around NewStyles().
func createStyleSet(theme Theme) Styles {
	return NewStyles(theme)
}

// ═══════════════════════════════════════════════════════════════════════════════
// PUBLIC API
// ═══════════════════════════════════════════════════════════════════════════════

// Run starts the TUI application and blocks until it exits.
// This is a convenience wrapper around tea.Program.Run().
func Run(cfg *Config) error {
	prog, err := New(cfg)
	if err != nil {
		return err
	}

	if _, err := prog.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}
