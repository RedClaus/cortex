// Package wizard implements the first-run configuration wizard for Pinky
package wizard

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/normanking/pinky/internal/config"
)

// Step represents a wizard step
type Step int

const (
	StepBrain Step = iota
	StepAPIKeys
	StepModelPicker
	StepChannels
	StepPermissions
	StepPersona
	StepConfirm
	StepDone
)

// Model holds the wizard state
type Model struct {
	step     Step
	config   *config.Config
	err      error
	quitting bool

	// Brain mode selection
	brainMode int // 0=embedded, 1=remote
	remoteURL textinput.Model

	// API Keys configuration (StepAPIKeys)
	apiKeyFocus      int // Which provider is focused
	ollamaURLEnabled bool
	ollamaURL        textinput.Model
	anthropicEnabled bool
	anthropicKey     textinput.Model
	openaiEnabled    bool
	openaiKey        textinput.Model
	groqEnabled      bool
	groqKey          textinput.Model

	// Model picker (StepModelPicker)
	modelPickerLaneFocus   int    // Which lane is focused for model selection
	modelPickerModelFocus  int    // Which model is focused within the lane
	modelPickerShowDetails bool   // Show model details
	modelPickerLaneNames   []string // Ordered list of lane names

	// Channel configuration
	channelFocus     int // Which channel is focused
	telegramEnabled  bool
	telegramToken    textinput.Model
	discordEnabled   bool
	discordToken     textinput.Model
	slackEnabled     bool
	slackToken       textinput.Model

	// Permission tier
	permissionTier int // 0=unrestricted, 1=some, 2=restricted

	// Persona
	persona int // 0=professional, 1=casual, 2=mentor, 3=minimalist

	// Window size
	width  int
	height int
}

// New creates a new wizard model
func New() Model {
	remoteURL := textinput.New()
	remoteURL.Placeholder = "http://localhost:18892"
	remoteURL.CharLimit = 256

	// API Keys inputs
	ollamaURL := textinput.New()
	ollamaURL.Placeholder = "http://localhost:11434"
	ollamaURL.CharLimit = 256

	anthropicKey := textinput.New()
	anthropicKey.Placeholder = "Enter Anthropic API key..."
	anthropicKey.EchoMode = textinput.EchoPassword
	anthropicKey.CharLimit = 256

	openaiKey := textinput.New()
	openaiKey.Placeholder = "Enter OpenAI API key..."
	openaiKey.EchoMode = textinput.EchoPassword
	openaiKey.CharLimit = 256

	groqKey := textinput.New()
	groqKey.Placeholder = "Enter Groq API key..."
	groqKey.EchoMode = textinput.EchoPassword
	groqKey.CharLimit = 256

	telegramToken := textinput.New()
	telegramToken.Placeholder = "Enter Telegram bot token..."
	telegramToken.EchoMode = textinput.EchoPassword
	telegramToken.CharLimit = 256

	discordToken := textinput.New()
	discordToken.Placeholder = "Enter Discord bot token..."
	discordToken.EchoMode = textinput.EchoPassword
	discordToken.CharLimit = 256

	slackToken := textinput.New()
	slackToken.Placeholder = "Enter Slack bot token..."
	slackToken.EchoMode = textinput.EchoPassword
	slackToken.CharLimit = 256

	m := Model{
		step:           StepBrain,
		config:         config.Default(),
		brainMode:      0, // Default to embedded
		remoteURL:      remoteURL,
		ollamaURL:      ollamaURL,
		anthropicKey:   anthropicKey,
		openaiKey:      openaiKey,
		groqKey:        groqKey,
		telegramToken:  telegramToken,
		discordToken:   discordToken,
		slackToken:     slackToken,
		permissionTier: 1, // Default to "some restrictions"
		persona:        0, // Default to professional
	}

	// Initialize lane names for model picker
	m.refreshLaneNames()

	return m
}

// refreshLaneNames updates the ordered list of lane names from config
func (m *Model) refreshLaneNames() {
	m.modelPickerLaneNames = make([]string, 0, len(m.config.Inference.Lanes))
	// Add lanes in a consistent order: local, fast, smart, then others
	order := []string{"local", "fast", "smart"}
	for _, name := range order {
		if _, ok := m.config.Inference.Lanes[name]; ok {
			m.modelPickerLaneNames = append(m.modelPickerLaneNames, name)
		}
	}
	// Add any remaining lanes
	for name := range m.config.Inference.Lanes {
		found := false
		for _, existing := range m.modelPickerLaneNames {
			if existing == name {
				found = true
				break
			}
		}
		if !found {
			m.modelPickerLaneNames = append(m.modelPickerLaneNames, name)
		}
	}
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.step != StepDone {
				m.quitting = true
				return m, tea.Quit
			}
		case "esc":
			if m.step > StepBrain && m.step < StepDone {
				m.step--
				return m, nil
			}
		}
	}

	// Delegate to step-specific update
	switch m.step {
	case StepBrain:
		return m.updateBrain(msg)
	case StepAPIKeys:
		return m.updateAPIKeys(msg)
	case StepModelPicker:
		return m.updateModelPicker(msg)
	case StepChannels:
		return m.updateChannels(msg)
	case StepPermissions:
		return m.updatePermissions(msg)
	case StepPersona:
		return m.updatePersona(msg)
	case StepConfirm:
		return m.updateConfirm(msg)
	case StepDone:
		return m, tea.Quit
	}

	return m, nil
}

// View implements tea.Model
func (m Model) View() string {
	if m.quitting {
		return quitStyle.Render("Setup cancelled. Goodbye!\n")
	}

	var content string

	switch m.step {
	case StepBrain:
		content = m.viewBrain()
	case StepAPIKeys:
		content = m.viewAPIKeys()
	case StepModelPicker:
		content = m.viewModelPicker()
	case StepChannels:
		content = m.viewChannels()
	case StepPermissions:
		content = m.viewPermissions()
	case StepPersona:
		content = m.viewPersona()
	case StepConfirm:
		content = m.viewConfirm()
	case StepDone:
		content = m.viewDone()
	}

	return content
}

// Config returns the configured settings
func (m Model) Config() *config.Config {
	return m.config
}

// buildConfig creates the final config from wizard choices
func (m *Model) buildConfig() {
	cfg := m.config

	// Brain mode
	if m.brainMode == 0 {
		cfg.Brain.Mode = "embedded"
	} else {
		cfg.Brain.Mode = "remote"
		cfg.Brain.RemoteURL = m.remoteURL.Value()
	}

	// API Keys - update lane configurations
	if m.ollamaURLEnabled && m.ollamaURL.Value() != "" {
		for name, lane := range cfg.Inference.Lanes {
			if lane.Engine == "ollama" {
				lane.URL = m.ollamaURL.Value()
				cfg.Inference.Lanes[name] = lane
			}
		}
	}

	if m.anthropicEnabled && m.anthropicKey.Value() != "" {
		for name, lane := range cfg.Inference.Lanes {
			if lane.Engine == "anthropic" {
				lane.APIKey = m.anthropicKey.Value()
				cfg.Inference.Lanes[name] = lane
			}
		}
	}

	if m.openaiEnabled && m.openaiKey.Value() != "" {
		for name, lane := range cfg.Inference.Lanes {
			if lane.Engine == "openai" {
				lane.APIKey = m.openaiKey.Value()
				cfg.Inference.Lanes[name] = lane
			}
		}
	}

	if m.groqEnabled && m.groqKey.Value() != "" {
		for name, lane := range cfg.Inference.Lanes {
			if lane.Engine == "groq" {
				lane.APIKey = m.groqKey.Value()
				cfg.Inference.Lanes[name] = lane
			}
		}
	}

	// Channels
	cfg.Channels.Telegram.Enabled = m.telegramEnabled
	if m.telegramEnabled {
		cfg.Channels.Telegram.Token = m.telegramToken.Value()
	}
	cfg.Channels.Discord.Enabled = m.discordEnabled
	if m.discordEnabled {
		cfg.Channels.Discord.Token = m.discordToken.Value()
	}
	cfg.Channels.Slack.Enabled = m.slackEnabled
	if m.slackEnabled {
		cfg.Channels.Slack.Token = m.slackToken.Value()
	}

	// Permissions
	switch m.permissionTier {
	case 0:
		cfg.Permissions.DefaultTier = "unrestricted"
	case 1:
		cfg.Permissions.DefaultTier = "some"
	case 2:
		cfg.Permissions.DefaultTier = "restricted"
	}

	// Persona
	personas := []string{"professional", "casual", "mentor", "minimalist"}
	cfg.Persona.Default = personas[m.persona]
}

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginBottom(1)

	stepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("205")).
			Padding(1, 2)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true)

	quitStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
)

func (m Model) renderHeader() string {
	logo := `
    ____  _       __
   / __ \(_)___  / /____  __
  / /_/ / / __ \/ //_/ / / /
 / ____/ / / / / ,< / /_/ /
/_/   /_/_/ /_/_/|_|\__, /
                   /____/
`
	return lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render(logo)
}

func (m Model) renderProgress() string {
	steps := []string{"Brain", "API Keys", "Models", "Channels", "Permissions", "Persona", "Confirm"}
	var progress string
	// Map step index to step constants (accounting for skipped indices)
	stepMap := []Step{StepBrain, StepAPIKeys, StepModelPicker, StepChannels, StepPermissions, StepPersona, StepConfirm}
	for i, name := range steps {
		if stepMap[i] < m.step {
			progress += successStyle.Render("✓ " + name)
		} else if stepMap[i] == m.step {
			progress += selectedStyle.Render("● " + name)
		} else {
			progress += dimStyle.Render("○ " + name)
		}
		if i < len(steps)-1 {
			progress += dimStyle.Render(" → ")
		}
	}
	return progress + "\n\n"
}

// Run starts the wizard and returns the resulting config
func Run() (*config.Config, error) {
	p := tea.NewProgram(New(), tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("wizard error: %w", err)
	}

	model := finalModel.(Model)
	if model.quitting {
		return nil, fmt.Errorf("wizard cancelled")
	}

	return model.Config(), nil
}
