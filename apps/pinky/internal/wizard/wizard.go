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

	// API Keys for inference lanes
	apiKeyFocus    int // Which lane is focused (0=fast, 1=smart, 2=openai)
	fastAPIKey     textinput.Model
	smartAPIKey    textinput.Model
	openaiAPIKey   textinput.Model

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

	// API key inputs
	fastAPIKey := textinput.New()
	fastAPIKey.Placeholder = "Enter Groq API key (gsk_...)..."
	fastAPIKey.EchoMode = textinput.EchoPassword
	fastAPIKey.CharLimit = 256

	smartAPIKey := textinput.New()
	smartAPIKey.Placeholder = "Enter Anthropic API key (sk-ant-...)..."
	smartAPIKey.EchoMode = textinput.EchoPassword
	smartAPIKey.CharLimit = 256

	openaiAPIKey := textinput.New()
	openaiAPIKey.Placeholder = "Enter OpenAI API key (sk-...)..."
	openaiAPIKey.EchoMode = textinput.EchoPassword
	openaiAPIKey.CharLimit = 256

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

	return Model{
		step:           StepBrain,
		config:         config.Default(),
		brainMode:      0, // Default to embedded
		remoteURL:      remoteURL,
		fastAPIKey:     fastAPIKey,
		smartAPIKey:    smartAPIKey,
		openaiAPIKey:   openaiAPIKey,
		telegramToken:  telegramToken,
		discordToken:   discordToken,
		slackToken:     slackToken,
		permissionTier: 1, // Default to "some restrictions"
		persona:        0, // Default to professional
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

	// API Keys for inference lanes
	if fastKey := m.fastAPIKey.Value(); fastKey != "" {
		if lane, ok := cfg.Inference.Lanes["fast"]; ok {
			lane.APIKey = fastKey
			cfg.Inference.Lanes["fast"] = lane
		}
	}
	if smartKey := m.smartAPIKey.Value(); smartKey != "" {
		if lane, ok := cfg.Inference.Lanes["smart"]; ok {
			lane.APIKey = smartKey
			cfg.Inference.Lanes["smart"] = lane
		}
	}
	if openaiKey := m.openaiAPIKey.Value(); openaiKey != "" {
		if lane, ok := cfg.Inference.Lanes["openai"]; ok {
			lane.APIKey = openaiKey
			cfg.Inference.Lanes["openai"] = lane
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
	steps := []string{"Brain", "API Keys", "Channels", "Permissions", "Persona", "Confirm"}
	var progress string
	for i, name := range steps {
		if Step(i) < m.step {
			progress += successStyle.Render("✓ " + name)
		} else if Step(i) == m.step {
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
