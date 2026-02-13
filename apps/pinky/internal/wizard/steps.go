package wizard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Step 1: Brain Mode Selection

func (m Model) updateBrain(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.brainMode > 0 {
				m.brainMode--
			}
		case "down", "j":
			if m.brainMode < 1 {
				m.brainMode++
			}
		case "enter":
			if m.brainMode == 1 {
				m.remoteURL.Focus()
			}
			m.step = StepAPIKeys
			return m, nil
		case "tab":
			if m.brainMode == 1 && m.remoteURL.Focused() {
				m.remoteURL.Blur()
			} else if m.brainMode == 1 {
				m.remoteURL.Focus()
			}
		}
	}

	// Update remote URL input if in remote mode
	if m.brainMode == 1 {
		var cmd tea.Cmd
		m.remoteURL, cmd = m.remoteURL.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) viewBrain() string {
	var b strings.Builder

	b.WriteString(m.renderHeader())
	b.WriteString(m.renderProgress())

	b.WriteString(titleStyle.Render("Step 1: Brain Mode"))
	b.WriteString("\n")
	b.WriteString(stepStyle.Render("How should Pinky think?"))
	b.WriteString("\n\n")

	// Embedded option
	if m.brainMode == 0 {
		b.WriteString(selectedStyle.Render("● Embedded (single binary, runs locally)"))
	} else {
		b.WriteString(normalStyle.Render("○ Embedded (single binary, runs locally)"))
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("   Best for: Personal use, simple setup"))
	b.WriteString("\n\n")

	// Remote option
	if m.brainMode == 1 {
		b.WriteString(selectedStyle.Render("● Remote (connect to CortexBrain server)"))
	} else {
		b.WriteString(normalStyle.Render("○ Remote (connect to CortexBrain server)"))
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("   Best for: Teams, separate GPU server"))

	// Show URL input if remote selected
	if m.brainMode == 1 {
		b.WriteString("\n\n")
		b.WriteString(normalStyle.Render("   CortexBrain URL: "))
		b.WriteString(m.remoteURL.View())
	}

	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("↑/↓ select • enter confirm • esc back • q quit"))

	return boxStyle.Render(b.String())
}

// Step 2: API Keys Configuration

func (m Model) updateAPIKeys(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			// Blur current, move up
			m.blurAPIKeyInputs()
			if m.apiKeyFocus > 0 {
				m.apiKeyFocus--
			}
			m.focusAPIKeyInput()
		case "down", "j":
			// Blur current, move down
			m.blurAPIKeyInputs()
			if m.apiKeyFocus < 2 {
				m.apiKeyFocus++
			}
			m.focusAPIKeyInput()
		case "tab":
			// Focus the current input
			m.focusAPIKeyInput()
		case "enter":
			m.blurAPIKeyInputs()
			m.step = StepChannels
			return m, nil
		}
	}

	// Update the focused input
	var cmd tea.Cmd
	switch m.apiKeyFocus {
	case 0:
		m.fastAPIKey, cmd = m.fastAPIKey.Update(msg)
	case 1:
		m.smartAPIKey, cmd = m.smartAPIKey.Update(msg)
	case 2:
		m.openaiAPIKey, cmd = m.openaiAPIKey.Update(msg)
	}

	return m, cmd
}

func (m *Model) blurAPIKeyInputs() {
	m.fastAPIKey.Blur()
	m.smartAPIKey.Blur()
	m.openaiAPIKey.Blur()
}

func (m *Model) focusAPIKeyInput() {
	switch m.apiKeyFocus {
	case 0:
		m.fastAPIKey.Focus()
	case 1:
		m.smartAPIKey.Focus()
	case 2:
		m.openaiAPIKey.Focus()
	}
}

func (m Model) viewAPIKeys() string {
	var b strings.Builder

	b.WriteString(m.renderHeader())
	b.WriteString(m.renderProgress())

	b.WriteString(titleStyle.Render("Step 2: API Keys"))
	b.WriteString("\n")
	b.WriteString(stepStyle.Render("Configure API keys for inference (optional - can set later)"))
	b.WriteString("\n\n")

	// Lane info
	lanes := []struct {
		name   string
		engine string
		desc   string
	}{
		{"Fast Lane (Groq)", "groq", "Fast inference, llama-3.3-70b"},
		{"Smart Lane (Anthropic)", "anthropic", "Claude for complex tasks"},
		{"OpenAI Lane", "openai", "GPT-4o alternative"},
	}

	inputs := []*textinput.Model{&m.fastAPIKey, &m.smartAPIKey, &m.openaiAPIKey}

	for i, lane := range lanes {
		if m.apiKeyFocus == i {
			b.WriteString(selectedStyle.Render("● " + lane.name))
		} else {
			b.WriteString(normalStyle.Render("○ " + lane.name))
		}
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("   " + lane.desc))
		b.WriteString("\n   ")

		// Show input or status
		if inputs[i].Value() != "" {
			b.WriteString(successStyle.Render("✓ Key set"))
		} else {
			b.WriteString(inputs[i].View())
		}
		b.WriteString("\n\n")
	}

	b.WriteString(dimStyle.Render("Note: Local lane (Ollama) doesn't need an API key"))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("↑/↓ select • tab edit • enter continue • esc back"))

	return boxStyle.Render(b.String())
}

// Step 3: Channel Configuration

func (m Model) updateChannels(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.channelFocus > 0 {
				m.channelFocus--
			}
		case "down", "j":
			if m.channelFocus < 2 {
				m.channelFocus++
			}
		case " ":
			// Toggle channel enabled state
			switch m.channelFocus {
			case 0:
				m.telegramEnabled = !m.telegramEnabled
				if m.telegramEnabled {
					m.telegramToken.Focus()
				} else {
					m.telegramToken.Blur()
				}
			case 1:
				m.discordEnabled = !m.discordEnabled
				if m.discordEnabled {
					m.discordToken.Focus()
				} else {
					m.discordToken.Blur()
				}
			case 2:
				m.slackEnabled = !m.slackEnabled
				if m.slackEnabled {
					m.slackToken.Focus()
				} else {
					m.slackToken.Blur()
				}
			}
		case "enter":
			m.telegramToken.Blur()
			m.discordToken.Blur()
			m.slackToken.Blur()
			m.step = StepPermissions
			return m, nil
		case "tab":
			// Cycle through token inputs for enabled channels
			if m.channelFocus == 0 && m.telegramEnabled {
				if m.telegramToken.Focused() {
					m.telegramToken.Blur()
				} else {
					m.telegramToken.Focus()
				}
			} else if m.channelFocus == 1 && m.discordEnabled {
				if m.discordToken.Focused() {
					m.discordToken.Blur()
				} else {
					m.discordToken.Focus()
				}
			} else if m.channelFocus == 2 && m.slackEnabled {
				if m.slackToken.Focused() {
					m.slackToken.Blur()
				} else {
					m.slackToken.Focus()
				}
			}
		}
	}

	// Update the focused token input
	var cmd tea.Cmd
	switch m.channelFocus {
	case 0:
		if m.telegramEnabled {
			m.telegramToken, cmd = m.telegramToken.Update(msg)
		}
	case 1:
		if m.discordEnabled {
			m.discordToken, cmd = m.discordToken.Update(msg)
		}
	case 2:
		if m.slackEnabled {
			m.slackToken, cmd = m.slackToken.Update(msg)
		}
	}

	return m, cmd
}

func (m Model) viewChannels() string {
	var b strings.Builder

	b.WriteString(m.renderHeader())
	b.WriteString(m.renderProgress())

	b.WriteString(titleStyle.Render("Step 3: Channels"))
	b.WriteString("\n")
	b.WriteString(stepStyle.Render("Enable messaging channels (optional - configure later with config file)"))
	b.WriteString("\n\n")

	// Telegram
	prefix := "○"
	if m.telegramEnabled {
		prefix = "✓"
	}
	if m.channelFocus == 0 {
		b.WriteString(selectedStyle.Render(fmt.Sprintf("%s Telegram", prefix)))
	} else {
		b.WriteString(normalStyle.Render(fmt.Sprintf("%s Telegram", prefix)))
	}
	if m.telegramEnabled {
		b.WriteString("\n   ")
		b.WriteString(m.telegramToken.View())
	}
	b.WriteString("\n\n")

	// Discord
	prefix = "○"
	if m.discordEnabled {
		prefix = "✓"
	}
	if m.channelFocus == 1 {
		b.WriteString(selectedStyle.Render(fmt.Sprintf("%s Discord", prefix)))
	} else {
		b.WriteString(normalStyle.Render(fmt.Sprintf("%s Discord", prefix)))
	}
	if m.discordEnabled {
		b.WriteString("\n   ")
		b.WriteString(m.discordToken.View())
	}
	b.WriteString("\n\n")

	// Slack
	prefix = "○"
	if m.slackEnabled {
		prefix = "✓"
	}
	if m.channelFocus == 2 {
		b.WriteString(selectedStyle.Render(fmt.Sprintf("%s Slack", prefix)))
	} else {
		b.WriteString(normalStyle.Render(fmt.Sprintf("%s Slack", prefix)))
	}
	if m.slackEnabled {
		b.WriteString("\n   ")
		b.WriteString(m.slackToken.View())
	}

	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("↑/↓ select • space toggle • tab edit token • enter continue • esc back"))

	return boxStyle.Render(b.String())
}

// Step 3: Permission Tier Selection

func (m Model) updatePermissions(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.permissionTier > 0 {
				m.permissionTier--
			}
		case "down", "j":
			if m.permissionTier < 2 {
				m.permissionTier++
			}
		case "enter":
			m.step = StepPersona
			return m, nil
		}
	}
	return m, nil
}

func (m Model) viewPermissions() string {
	var b strings.Builder

	b.WriteString(m.renderHeader())
	b.WriteString(m.renderProgress())

	b.WriteString(titleStyle.Render("Step 4: Permission Level"))
	b.WriteString("\n")
	b.WriteString(stepStyle.Render("How much autonomy should Pinky have?"))
	b.WriteString("\n\n")

	// Unrestricted
	if m.permissionTier == 0 {
		b.WriteString(selectedStyle.Render("● Unrestricted"))
	} else {
		b.WriteString(normalStyle.Render("○ Unrestricted"))
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("   Execute all tools automatically, no approval prompts"))
	b.WriteString("\n\n")

	// Some Restrictions (default)
	if m.permissionTier == 1 {
		b.WriteString(selectedStyle.Render("● Some Restrictions (Recommended)"))
	} else {
		b.WriteString(normalStyle.Render("○ Some Restrictions (Recommended)"))
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("   Auto-approve low-risk tools, ask for high-risk actions"))
	b.WriteString("\n\n")

	// Restricted
	if m.permissionTier == 2 {
		b.WriteString(selectedStyle.Render("● Restricted"))
	} else {
		b.WriteString(normalStyle.Render("○ Restricted"))
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("   Ask before every tool execution, maximum control"))

	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("↑/↓ select • enter confirm • esc back • q quit"))

	return boxStyle.Render(b.String())
}

// Step 4: Persona Selection

func (m Model) updatePersona(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.persona > 0 {
				m.persona--
			}
		case "down", "j":
			if m.persona < 3 {
				m.persona++
			}
		case "enter":
			m.step = StepConfirm
			return m, nil
		}
	}
	return m, nil
}

func (m Model) viewPersona() string {
	var b strings.Builder

	b.WriteString(m.renderHeader())
	b.WriteString(m.renderProgress())

	b.WriteString(titleStyle.Render("Step 5: Persona"))
	b.WriteString("\n")
	b.WriteString(stepStyle.Render("Choose Pinky's personality"))
	b.WriteString("\n\n")

	personas := []struct {
		name string
		desc string
	}{
		{"Professional", "Clear, concise, formal responses"},
		{"Casual", "Friendly, conversational tone"},
		{"Mentor", "Patient, educational, explains concepts"},
		{"Minimalist", "Terse, just the facts"},
	}

	for i, p := range personas {
		if m.persona == i {
			b.WriteString(selectedStyle.Render("● " + p.name))
		} else {
			b.WriteString(normalStyle.Render("○ " + p.name))
		}
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("   " + p.desc))
		b.WriteString("\n\n")
	}

	b.WriteString(helpStyle.Render("↑/↓ select • enter confirm • esc back • q quit"))

	return boxStyle.Render(b.String())
}

// Step 5: Confirmation

func (m Model) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", "y":
			m.buildConfig()
			if err := m.config.Save(""); err != nil {
				m.err = err
				return m, nil
			}
			m.step = StepDone
			return m, nil
		case "n":
			m.step = StepBrain
			return m, nil
		}
	}
	return m, nil
}

func (m Model) viewConfirm() string {
	var b strings.Builder

	b.WriteString(m.renderHeader())
	b.WriteString(m.renderProgress())

	b.WriteString(titleStyle.Render("Step 6: Confirm Settings"))
	b.WriteString("\n\n")

	// Brain Mode
	b.WriteString(normalStyle.Render("Brain Mode: "))
	if m.brainMode == 0 {
		b.WriteString(selectedStyle.Render("Embedded"))
	} else {
		b.WriteString(selectedStyle.Render("Remote (" + m.remoteURL.Value() + ")"))
	}
	b.WriteString("\n\n")

	// API Keys
	b.WriteString(normalStyle.Render("API Keys: "))
	var keys []string
	if m.fastAPIKey.Value() != "" {
		keys = append(keys, "Groq ✓")
	}
	if m.smartAPIKey.Value() != "" {
		keys = append(keys, "Anthropic ✓")
	}
	if m.openaiAPIKey.Value() != "" {
		keys = append(keys, "OpenAI ✓")
	}
	if len(keys) == 0 {
		b.WriteString(dimStyle.Render("None (use env vars or set later)"))
	} else {
		b.WriteString(selectedStyle.Render(strings.Join(keys, ", ")))
	}
	b.WriteString("\n\n")

	// Channels
	b.WriteString(normalStyle.Render("Channels: "))
	var channels []string
	if m.telegramEnabled {
		channels = append(channels, "Telegram")
	}
	if m.discordEnabled {
		channels = append(channels, "Discord")
	}
	if m.slackEnabled {
		channels = append(channels, "Slack")
	}
	if len(channels) == 0 {
		b.WriteString(dimStyle.Render("None (TUI/WebUI only)"))
	} else {
		b.WriteString(selectedStyle.Render(strings.Join(channels, ", ")))
	}
	b.WriteString("\n\n")

	// Permissions
	b.WriteString(normalStyle.Render("Permissions: "))
	perms := []string{"Unrestricted", "Some Restrictions", "Restricted"}
	b.WriteString(selectedStyle.Render(perms[m.permissionTier]))
	b.WriteString("\n\n")

	// Persona
	b.WriteString(normalStyle.Render("Persona: "))
	personas := []string{"Professional", "Casual", "Mentor", "Minimalist"}
	b.WriteString(selectedStyle.Render(personas[m.persona]))
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("Error: " + m.err.Error()))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(normalStyle.Render("Save configuration to ~/.pinky/config.yaml?"))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("y/enter confirm • n restart • esc back • q quit"))

	return boxStyle.Render(b.String())
}

// Done view
func (m Model) viewDone() string {
	var b strings.Builder

	b.WriteString(m.renderHeader())
	b.WriteString("\n")
	b.WriteString(successStyle.Render("✓ Configuration saved to ~/.pinky/config.yaml"))
	b.WriteString("\n\n")
	b.WriteString(normalStyle.Render("Pinky is ready! Start with:"))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("  ./pinky --tui    # Terminal interface"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  ./pinky          # Server mode (WebUI + channels)"))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("Press any key to exit..."))

	return boxStyle.Render(b.String())
}
