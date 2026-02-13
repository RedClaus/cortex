// Package tui provides the terminal user interface for Pinky
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/bubbles/key"

	"github.com/normanking/pinky/internal/brain"
	"github.com/normanking/pinky/internal/config"
)

// LaneManager interface for brain lane operations
type LaneManager interface {
	SetLane(name string) error
	GetLane() string
	GetLanes() []brain.LaneInfo
	SetAutoLLM(enabled bool)
	GetAutoLLM() bool
}

// ModelInfo represents a model choice
type ModelInfo struct {
	ID   string
	Name string
}

// SettingsPanel provides a TUI for configuring inference settings
type SettingsPanel struct {
	brain         LaneManager
	config        *config.Config
	configPath    string

	autoLLM       bool
	lanes         []brain.LaneInfo
	models        map[string][]ModelInfo // lane name -> available models

	focusedLane   int    // which lane is selected
	focusedModel  int    // which model within the lane is selected
	editingLane   bool   // true when editing a lane's model
	showModelList bool   // true when showing model selection list

	width, height int
	errors        []string
}

// Static model lists for each engine
var staticModels = map[string][]ModelInfo{
	"ollama": {
		{ID: "llama3.2:3b", Name: "Llama 3.2 3B"},
		{ID: "llama3.2:1b", Name: "Llama 3.2 1B"},
		{ID: "llama3.1:8b", Name: "Llama 3.1 8B"},
		{ID: "llama3:8b", Name: "Llama 3 8B"},
		{ID: "qwen2.5:7b", Name: "Qwen 2.5 7B"},
		{ID: "phi4:14b", Name: "Phi-4 14B"},
		{ID: "mistral:7b", Name: "Mistral 7B"},
		{ID: "codellama:7b", Name: "CodeLlama 7B"},
		{ID: "deepseek-coder:6.7b", Name: "DeepSeek Coder 6.7B"},
		{ID: "gemma2:9b", Name: "Gemma 2 9B"},
	},
	"anthropic": {
		{ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4"},
		{ID: "claude-3-5-sonnet-20241022", Name: "Claude 3.5 Sonnet"},
		{ID: "claude-3-opus-20240229", Name: "Claude 3 Opus"},
		{ID: "claude-3-haiku-20240307", Name: "Claude 3 Haiku"},
	},
	"openai": {
		{ID: "gpt-4o", Name: "GPT-4o"},
		{ID: "gpt-4o-mini", Name: "GPT-4o Mini"},
		{ID: "gpt-4-turbo", Name: "GPT-4 Turbo"},
		{ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo"},
	},
	"groq": {
		{ID: "llama-3.3-70b-versatile", Name: "Llama 3.3 70B"},
		{ID: "llama-3.1-8b-instant", Name: "Llama 3.1 8B"},
		{ID: "mixtral-8x7b-32768", Name: "Mixtral 8x7B"},
		{ID: "gemma2-9b-it", Name: "Gemma 2 9B"},
	},
}

// NewSettingsPanel creates a new settings panel
func NewSettingsPanel(brain LaneManager, cfg *config.Config, configPath string) *SettingsPanel {
	return &SettingsPanel{
		brain:      brain,
		config:     cfg,
		configPath: configPath,
		autoLLM:    brain.GetAutoLLM(),
		lanes:      brain.GetLanes(),
		models:     make(map[string][]ModelInfo),
		errors:     make([]string, 0),
	}
}

// Init initializes the settings panel
func (s *SettingsPanel) Init() tea.Cmd {
	// Load available models for each lane
	for _, lane := range s.lanes {
		if models, ok := staticModels[lane.Engine]; ok {
			s.models[lane.Name] = models
		} else {
			s.models[lane.Name] = []ModelInfo{
				{ID: lane.Model, Name: lane.Model},
			}
		}
	}
	return nil
}

// Update handles messages and updates the settings panel
func (s *SettingsPanel) Update(msg tea.Msg) (*SettingsPanel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return s.handleKeyMsg(msg)
	}
	return s, nil
}

// handleKeyMsg processes keyboard input
func (s *SettingsPanel) handleKeyMsg(msg tea.KeyMsg) (*SettingsPanel, tea.Cmd) {
	// When showing model list, handle model selection
	if s.showModelList {
		return s.handleModelSelection(msg)
	}

	switch {
	case key.Matches(msg, s.keys().Quit),
		 key.Matches(msg, s.keys().Cancel),
		 msg.String() == "esc":
		return s, func() tea.Msg { return CloseSettingsMsg{} }

	case msg.String() == "tab":
		// Toggle auto-routing
		s.autoLLM = !s.autoLLM
		s.brain.SetAutoLLM(s.autoLLM)
		s.saveConfig()

	case msg.String() == "up", msg.String() == "k":
		if s.focusedLane > 0 {
			s.focusedLane--
			s.focusedModel = 0
		}

	case msg.String() == "down", msg.String() == "j":
		if s.focusedLane < len(s.lanes)-1 {
			s.focusedLane++
			s.focusedModel = 0
		}

	case msg.String() == "enter", msg.String() == " ":
		// Enter model selection mode for focused lane
		if s.focusedLane < len(s.lanes) {
			lane := s.lanes[s.focusedLane]
			models, ok := s.models[lane.Name]
			if !ok || len(models) == 0 {
				// No models available for this lane
				s.errors = append(s.errors, fmt.Sprintf("No models available for %s lane", lane.Name))
				if len(s.errors) > 3 {
					s.errors = s.errors[len(s.errors)-3:]
				}
				return s, nil
			}
			s.showModelList = true
			s.editingLane = true
			// Find current model index
			for i, m := range models {
				if m.ID == lane.Model {
					s.focusedModel = i
					break
				}
			}
		}
	}

	return s, nil
}

// handleModelSelection handles keys when selecting a model
func (s *SettingsPanel) handleModelSelection(msg tea.KeyMsg) (*SettingsPanel, tea.Cmd) {
	switch {
	case key.Matches(msg, s.keys().Cancel),
		 msg.String() == "esc":
		// Cancel model selection
		s.showModelList = false
		s.editingLane = false

	case msg.String() == "up", msg.String() == "k":
		if s.focusedModel > 0 {
			s.focusedModel--
		}

	case msg.String() == "down", msg.String() == "j":
		if s.focusedLane >= len(s.lanes) {
			return s, nil
		}
		lane := s.lanes[s.focusedLane]
		models, ok := s.models[lane.Name]
		if !ok || len(models) == 0 {
			return s, nil
		}
		if s.focusedModel < len(models)-1 {
			s.focusedModel++
		}

	case msg.String() == "enter", msg.String() == " ":
		// Select the model
		if s.focusedLane >= len(s.lanes) {
			s.showModelList = false
			s.editingLane = false
			return s, nil
		}
		lane := s.lanes[s.focusedLane]
		models, ok := s.models[lane.Name]
		if !ok || len(models) == 0 {
			s.showModelList = false
			s.editingLane = false
			return s, nil
		}
		if s.focusedModel < len(models) {
			selectedModel := models[s.focusedModel]
			s.setLaneModel(lane.Name, selectedModel.ID)
		}
		s.showModelList = false
		s.editingLane = false
	}

	return s, nil
}

// setLaneModel updates the model for a lane and persists to config
func (s *SettingsPanel) setLaneModel(laneName, model string) {
	// Update config
	if lane, ok := s.config.Inference.Lanes[laneName]; ok {
		lane.Model = model
		s.config.Inference.Lanes[laneName] = lane
	}

	// Update local lane info
	for i := range s.lanes {
		if s.lanes[i].Name == laneName {
			s.lanes[i].Model = model
			break
		}
	}

	// Persist to file
	s.saveConfig()
}

// saveConfig persists the config to disk
func (s *SettingsPanel) saveConfig() {
	// Update autollm in config
	s.config.Inference.AutoLLM = s.autoLLM

	// Save to file
	if err := s.config.Save(s.configPath); err != nil {
		s.errors = append(s.errors, fmt.Sprintf("Failed to save config: %v", err))
		if len(s.errors) > 3 {
			s.errors = s.errors[len(s.errors)-3:]
		}
	}
}

// SetSize updates the panel dimensions
func (s *SettingsPanel) SetSize(width, height int) {
	s.width = width
	s.height = height
}

// keys returns key bindings for the settings panel
func (s *SettingsPanel) keys() KeyMap {
	return DefaultKeyMap()
}

// View renders the settings panel
func (s *SettingsPanel) View() string {
	if s.width == 0 {
		s.width = 80
	}
	if s.height == 0 {
		s.height = 24
	}

	var sections []string

	// Title bar with close hint
	titleBar := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorPrimary).
		Width(s.width-4).
		Render("⚙️  Inference Settings")
	
	closeHint := lipgloss.NewStyle().
		Foreground(colorMuted).
		Render("[Esc] Close")
	
	sections = append(sections, lipgloss.JoinHorizontal(lipgloss.Left, titleBar, closeHint))
	sections = append(sections, strings.Repeat("─", s.width-2))

	// Auto-routing toggle
	autoOn := "[ ON ]"
	autoOff := "[OFF]"
	if s.autoLLM {
		autoOn = lipgloss.NewStyle().Foreground(colorSuccess).Bold(true).Render("[ ON ]")
	} else {
		autoOff = lipgloss.NewStyle().Foreground(colorMuted).Bold(true).Render("[OFF]")
	}

	autoSection := lipgloss.JoinHorizontal(lipgloss.Left,
		"Auto-routing: ",
		autoOn,
		" ",
		autoOff,
		lipgloss.NewStyle().Foreground(colorMuted).Render("  ← Tab to toggle"),
	)
	sections = append(sections, "")
	sections = append(sections, autoSection)
	sections = append(sections, lipgloss.NewStyle().Foreground(colorMuted).Italic(true).Render("  Automatically selects lane based on task complexity"))
	sections = append(sections, "")
	sections = append(sections, strings.Repeat("─", s.width-2))

	// Lane list or model selection
	if s.showModelList {
		sections = append(sections, s.renderModelSelection())
	} else {
		sections = append(sections, s.renderLaneList())
	}

	// Errors
	if len(s.errors) > 0 {
		sections = append(sections, "")
		for _, err := range s.errors {
			sections = append(sections, lipgloss.NewStyle().Foreground(colorDanger).Render("⚠ "+err))
		}
	}

	// Footer help
	sections = append(sections, "")
	sections = append(sections, strings.Repeat("─", s.width-2))
	
	if s.showModelList {
		sections = append(sections, lipgloss.NewStyle().Foreground(colorMuted).Render("↑↓ Navigate  Enter Select  Esc Cancel"))
	} else {
		sections = append(sections, lipgloss.NewStyle().Foreground(colorMuted).Render("↑↓ Navigate  Enter Select Model  Tab Toggle Auto-routing  Esc Close"))
	}

	// Join all sections
	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Create container with border
	container := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(1, 2).
		Width(s.width).
		Height(s.height)

	return container.Render(content)
}

// renderLaneList renders the list of lanes
func (s *SettingsPanel) renderLaneList() string {
	var lines []string
	lines = append(lines, "")

	for i, lane := range s.lanes {
		isFocused := i == s.focusedLane
		activeMarker := "  "
		if lane.Active {
			activeMarker = "▶ "
		}

		laneStyle := lipgloss.NewStyle()
		if isFocused {
			laneStyle = laneStyle.Background(colorBgAlt).Bold(true)
		}

		engineColor := colorSecondary
		switch lane.Engine {
		case "ollama":
			engineColor = lipgloss.Color("#FF6B6B")
		case "anthropic":
			engineColor = lipgloss.Color("#E57035")
		case "openai":
			engineColor = lipgloss.Color("#74AA9C")
		case "groq":
			engineColor = lipgloss.Color("#F55036")
		}

		laneLine := fmt.Sprintf("%s%s (%s)",
			activeMarker,
			strings.Title(lane.Name),
			lipgloss.NewStyle().Foreground(engineColor).Render(lane.Engine),
		)
		
		modelLine := fmt.Sprintf("    Model: %s", lane.Model)
		if isFocused {
			modelLine = lipgloss.NewStyle().Foreground(colorPrimary).Render(modelLine + " ← Enter to change")
		}

		lines = append(lines, laneStyle.Render(laneLine))
		lines = append(lines, modelLine)
		lines = append(lines, "")
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderModelSelection renders the model selection list
func (s *SettingsPanel) renderModelSelection() string {
	if s.focusedLane >= len(s.lanes) {
		return ""
	}

	lane := s.lanes[s.focusedLane]
	models, ok := s.models[lane.Name]
	if !ok || len(models) == 0 {
		return lipgloss.NewStyle().Foreground(colorDanger).Render("\n  No models available for " + lane.Name + " lane\n")
	}

	var lines []string
	lines = append(lines, "")
	lines = append(lines, lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("Select Model for %s Lane:", strings.Title(lane.Name))))
	lines = append(lines, "")

	for i, model := range models {
		cursor := "  "
		if i == s.focusedModel {
			cursor = "> "
		}

		modelStyle := lipgloss.NewStyle()
		if i == s.focusedModel {
			modelStyle = modelStyle.Foreground(colorPrimary).Bold(true)
		}

		marker := ""
		if model.ID == lane.Model {
			marker = lipgloss.NewStyle().Foreground(colorSuccess).Render(" ✓")
		}

		line := fmt.Sprintf("%s%s%s", cursor, model.Name, marker)
		lines = append(lines, modelStyle.Render(line))
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// CloseSettingsMsg signals that the settings panel should be closed
type CloseSettingsMsg struct{}
