package wizard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/normanking/pinky/internal/config"
)

// Static model lists for cloud providers
var (
	anthropicModels = []modelInfo{
		{ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4", Description: "Balanced intelligence and speed"},
		{ID: "claude-3-5-sonnet-20241022", Name: "Claude 3.5 Sonnet", Description: "Reliable for most tasks"},
		{ID: "claude-3-opus-20240229", Name: "Claude 3 Opus", Description: "Highest capability"},
		{ID: "claude-3-haiku-20240307", Name: "Claude 3 Haiku", Description: "Fast and efficient"},
	}

	openaiModels = []modelInfo{
		{ID: "gpt-4o", Name: "GPT-4o", Description: "Latest multimodal model"},
		{ID: "gpt-4o-mini", Name: "GPT-4o Mini", Description: "Fast and affordable"},
		{ID: "gpt-4-turbo", Name: "GPT-4 Turbo", Description: "High capability"},
		{ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo", Description: "Cost effective"},
	}

	groqModels = []modelInfo{
		{ID: "llama-3.3-70b-versatile", Name: "Llama 3.3 70B", Description: "Powerful open model"},
		{ID: "llama-3.1-8b-instant", Name: "Llama 3.1 8B", Description: "Fast inference"},
		{ID: "mixtral-8x7b-32768", Name: "Mixtral 8x7B", Description: "Mixture of experts"},
		{ID: "gemma2-9b-it", Name: "Gemma 2 9B", Description: "Google's efficient model"},
	}

	ollamaDefaultModels = []modelInfo{
		{ID: "llama3.2:3b", Name: "Llama 3.2 3B", Description: "Lightweight local model"},
		{ID: "llama3.2:1b", Name: "Llama 3.2 1B", Description: "Ultra-lightweight"},
		{ID: "llama3.1:8b", Name: "Llama 3.1 8B", Description: "Balanced performance"},
		{ID: "gemma2:2b", Name: "Gemma 2 2B", Description: "Google's small model"},
		{ID: "phi3:mini", Name: "Phi-3 Mini", Description: "Microsoft's efficient model"},
	}
)

type modelInfo struct {
	ID          string
	Name        string
	Description string
}

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
				m.step = StepChannels
			} else {
				m.step = StepAPIKeys
			}
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

// Step 1.5: API Keys Configuration

func (m Model) updateAPIKeys(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.apiKeyFocus > 0 {
				m.apiKeyFocus--
				m.blurAllAPIKeys()
			}
		case "down", "j":
			if m.apiKeyFocus < 3 {
				m.apiKeyFocus++
				m.blurAllAPIKeys()
			}
		case " ":
			// Toggle enabled state
			switch m.apiKeyFocus {
			case 0:
				m.ollamaURLEnabled = !m.ollamaURLEnabled
				if m.ollamaURLEnabled {
					m.ollamaURL.Focus()
				} else {
					m.ollamaURL.Blur()
				}
			case 1:
				m.anthropicEnabled = !m.anthropicEnabled
				if m.anthropicEnabled {
					m.anthropicKey.Focus()
				} else {
					m.anthropicKey.Blur()
				}
			case 2:
				m.openaiEnabled = !m.openaiEnabled
				if m.openaiEnabled {
					m.openaiKey.Focus()
				} else {
					m.openaiKey.Blur()
				}
			case 3:
				m.groqEnabled = !m.groqEnabled
				if m.groqEnabled {
					m.groqKey.Focus()
				} else {
					m.groqKey.Blur()
				}
			}
		case "enter":
			// Setup default lanes based on enabled providers
			m.setupDefaultLanes()
			m.refreshLaneNames()
			m.step = StepModelPicker
			return m, nil
		case "tab":
			// Focus the input for the current provider
			switch m.apiKeyFocus {
			case 0:
				if m.ollamaURLEnabled {
					if m.ollamaURL.Focused() {
						m.ollamaURL.Blur()
					} else {
						m.ollamaURL.Focus()
					}
				}
			case 1:
				if m.anthropicEnabled {
					if m.anthropicKey.Focused() {
						m.anthropicKey.Blur()
					} else {
						m.anthropicKey.Focus()
					}
				}
			case 2:
				if m.openaiEnabled {
					if m.openaiKey.Focused() {
						m.openaiKey.Blur()
					} else {
						m.openaiKey.Focus()
					}
				}
			case 3:
				if m.groqEnabled {
					if m.groqKey.Focused() {
						m.groqKey.Blur()
					} else {
						m.groqKey.Focus()
					}
				}
			}
		}
	}

	// Update the focused input
	var cmd tea.Cmd
	switch m.apiKeyFocus {
	case 0:
		if m.ollamaURLEnabled {
			m.ollamaURL, cmd = m.ollamaURL.Update(msg)
		}
	case 1:
		if m.anthropicEnabled {
			m.anthropicKey, cmd = m.anthropicKey.Update(msg)
		}
	case 2:
		if m.openaiEnabled {
			m.openaiKey, cmd = m.openaiKey.Update(msg)
		}
	case 3:
		if m.groqEnabled {
			m.groqKey, cmd = m.groqKey.Update(msg)
		}
	}

	return m, cmd
}

func (m *Model) blurAllAPIKeys() {
	m.ollamaURL.Blur()
	m.anthropicKey.Blur()
	m.openaiKey.Blur()
	m.groqKey.Blur()
}

func (m *Model) setupDefaultLanes() {
	// Start with default lanes
	lanes := make(map[string]config.Lane)

	// Always add local/ollama lane
	if m.ollamaURLEnabled && m.ollamaURL.Value() != "" {
		lanes["local"] = config.Lane{
			Engine: "ollama",
			Model:  "llama3.2:3b",
			URL:    m.ollamaURL.Value(),
		}
	} else {
		lanes["local"] = config.Lane{
			Engine: "ollama",
			Model:  "llama3.2:3b",
			URL:    "http://localhost:11434",
		}
	}

	// Add fast lane (groq if enabled, otherwise ollama)
	if m.groqEnabled && m.groqKey.Value() != "" {
		lanes["fast"] = config.Lane{
			Engine: "groq",
			Model:  "llama-3.1-8b-instant",
			APIKey: m.groqKey.Value(),
		}
	} else {
		lanes["fast"] = config.Lane{
			Engine: "ollama",
			Model:  "llama3.2:3b",
			URL:    "http://localhost:11434",
		}
	}

	// Add smart lane (anthropic or openai if enabled)
	if m.anthropicEnabled && m.anthropicKey.Value() != "" {
		lanes["smart"] = config.Lane{
			Engine: "anthropic",
			Model:  "claude-3-5-sonnet-20241022",
			APIKey: m.anthropicKey.Value(),
		}
	} else if m.openaiEnabled && m.openaiKey.Value() != "" {
		lanes["smart"] = config.Lane{
			Engine: "openai",
			Model:  "gpt-4o-mini",
			APIKey: m.openaiKey.Value(),
		}
	}

	m.config.Inference.Lanes = lanes
	m.config.Inference.DefaultLane = "fast"
}

func (m Model) viewAPIKeys() string {
	var b strings.Builder

	b.WriteString(m.renderHeader())
	b.WriteString(m.renderProgress())

	b.WriteString(titleStyle.Render("Step 2: API Keys"))
	b.WriteString("\n")
	b.WriteString(stepStyle.Render("Configure AI providers (optional - can use local Ollama only)"))
	b.WriteString("\n\n")

	providers := []struct {
		name    string
		enabled *bool
		input   string
		focus   int
	}{
		{"Ollama URL", &m.ollamaURLEnabled, m.ollamaURL.View(), 0},
		{"Anthropic", &m.anthropicEnabled, m.anthropicKey.View(), 1},
		{"OpenAI", &m.openaiEnabled, m.openaiKey.View(), 2},
		{"Groq", &m.groqEnabled, m.groqKey.View(), 3},
	}

	for _, p := range providers {
		prefix := "○"
		if *p.enabled {
			prefix = "✓"
		}

		if m.apiKeyFocus == p.focus {
			b.WriteString(selectedStyle.Render(fmt.Sprintf("%s %s", prefix, p.name)))
		} else {
			b.WriteString(normalStyle.Render(fmt.Sprintf("%s %s", prefix, p.name)))
		}

		// Show input if enabled
		if *p.enabled || (p.focus == 0 && m.ollamaURLEnabled) {
			b.WriteString("\n   ")
			b.WriteString(p.input)
		}
		b.WriteString("\n\n")
	}

	b.WriteString(dimStyle.Render("Enable providers you want to use. Ollama runs locally; others require API keys."))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("↑/↓ select • space toggle • tab edit • enter continue • esc back"))

	return boxStyle.Render(b.String())
}

// Step 2: Model Picker

func (m Model) updateModelPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.modelPickerModelFocus > 0 {
				m.modelPickerModelFocus--
			}
		case "down", "j":
			// Get max models for current lane
			maxModels := m.getMaxModelsForCurrentLane()
			if m.modelPickerModelFocus < maxModels-1 {
				m.modelPickerModelFocus++
			}
		case "left", "h":
			if m.modelPickerLaneFocus > 0 {
				m.modelPickerLaneFocus--
				m.modelPickerModelFocus = 0
			}
		case "right", "l":
			if m.modelPickerLaneFocus < len(m.modelPickerLaneNames)-1 {
				m.modelPickerLaneFocus++
				m.modelPickerModelFocus = 0
			}
		case "tab":
			// Toggle auto-routing
			m.config.Inference.AutoLLM = !m.config.Inference.AutoLLM
		case "enter", " ":
			// Select the current model for the current lane
			m.selectCurrentModel()
		case "n":
			// Move to next step
			m.step = StepChannels
			return m, nil
		case "d":
			// Toggle details view
			m.modelPickerShowDetails = !m.modelPickerShowDetails
		}
	}
	return m, nil
}

func (m *Model) getMaxModelsForCurrentLane() int {
	if m.modelPickerLaneFocus >= len(m.modelPickerLaneNames) {
		return 0
	}
	laneName := m.modelPickerLaneNames[m.modelPickerLaneFocus]
	lane, ok := m.config.Inference.Lanes[laneName]
	if !ok {
		return 0
	}

	models := m.getModelsForEngine(lane.Engine)
	return len(models)
}

func (m *Model) getModelsForEngine(engine string) []modelInfo {
	switch engine {
	case "anthropic":
		return anthropicModels
	case "openai":
		return openaiModels
	case "groq":
		return groqModels
	case "ollama":
		// Try to fetch from Ollama if possible, otherwise use defaults
		return m.fetchOllamaModels()
	default:
		return []modelInfo{}
	}
}

func (m *Model) fetchOllamaModels() []modelInfo {
	// Try to fetch from Ollama API
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Get Ollama URL from config
	ollamaURL := "http://localhost:11434"
	for _, lane := range m.config.Inference.Lanes {
		if lane.Engine == "ollama" && lane.URL != "" {
			ollamaURL = lane.URL
			break
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", ollamaURL+"/api/tags", nil)
	if err != nil {
		return ollamaDefaultModels
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ollamaDefaultModels
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ollamaDefaultModels
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
			Size int64  `json:"size"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ollamaDefaultModels
	}

	if len(result.Models) == 0 {
		return ollamaDefaultModels
	}

	// Convert to modelInfo
	models := make([]modelInfo, len(result.Models))
	for i, mdl := range result.Models {
		sizeStr := ""
		if mdl.Size > 0 {
			if mdl.Size > 1024*1024*1024 {
				sizeStr = fmt.Sprintf(" (%.1f GB)", float64(mdl.Size)/(1024*1024*1024))
			} else if mdl.Size > 1024*1024 {
				sizeStr = fmt.Sprintf(" (%.1f MB)", float64(mdl.Size)/(1024*1024))
			}
		}
		models[i] = modelInfo{
			ID:          mdl.Name,
			Name:        mdl.Name,
			Description: "Ollama model" + sizeStr,
		}
	}

	return models
}

func (m *Model) selectCurrentModel() {
	if m.modelPickerLaneFocus >= len(m.modelPickerLaneNames) {
		return
	}
	laneName := m.modelPickerLaneNames[m.modelPickerLaneFocus]
	lane, ok := m.config.Inference.Lanes[laneName]
	if !ok {
		return
	}

	models := m.getModelsForEngine(lane.Engine)
	if m.modelPickerModelFocus >= len(models) {
		return
	}

	selectedModel := models[m.modelPickerModelFocus]
	lane.Model = selectedModel.ID
	m.config.Inference.Lanes[laneName] = lane
}

func (m Model) viewModelPicker() string {
	var b strings.Builder

	b.WriteString(m.renderHeader())
	b.WriteString(m.renderProgress())

	b.WriteString(titleStyle.Render("Step 3: Model Selection"))
	b.WriteString("\n\n")

	// Auto-routing toggle
	autoStatus := "OFF"
	autoStyle := dimStyle
	if m.config.Inference.AutoLLM {
		autoStatus = "ON"
		autoStyle = selectedStyle
	}
	b.WriteString(fmt.Sprintf("Auto-routing: %s", autoStyle.Render("["+autoStatus+"]")))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  Automatically selects lane based on task complexity"))
	b.WriteString("\n\n")

	// Model selection for each lane
	if len(m.modelPickerLaneNames) == 0 {
		b.WriteString(dimStyle.Render("No lanes configured. Press 'n' to skip."))
	} else {
		for laneIdx, laneName := range m.modelPickerLaneNames {
			lane, ok := m.config.Inference.Lanes[laneName]
			if !ok {
				continue
			}

			// Lane header
			isFocusedLane := laneIdx == m.modelPickerLaneFocus
			laneTitle := fmt.Sprintf("── %s Lane (%s) ──", strings.Title(laneName), lane.Engine)
			if isFocusedLane {
				b.WriteString(selectedStyle.Render(laneTitle))
			} else {
				b.WriteString(dimStyle.Render(laneTitle))
			}
			b.WriteString("\n")

			// Get models for this lane's engine
			models := m.getModelsForEngine(lane.Engine)
			if len(models) == 0 {
				b.WriteString(dimStyle.Render("  No models available\n"))
				continue
			}

			// Display models
			for modelIdx, model := range models {
				var cursor string
				if isFocusedLane && modelIdx == m.modelPickerModelFocus {
					cursor = selectedStyle.Render("> ")
				} else if model.ID == lane.Model {
					cursor = successStyle.Render("✓ ")
				} else {
					cursor = "  "
				}

				modelName := model.Name
				if isFocusedLane && modelIdx == m.modelPickerModelFocus {
					modelName = selectedStyle.Render(modelName)
				} else if model.ID == lane.Model {
					modelName = normalStyle.Render(modelName)
				} else {
					modelName = dimStyle.Render(modelName)
				}

				b.WriteString(fmt.Sprintf("%s%s", cursor, modelName))

				// Show description if details enabled or this is the selected model
				if m.modelPickerShowDetails || (model.ID == lane.Model && !isFocusedLane) {
					b.WriteString(dimStyle.Render(fmt.Sprintf(" - %s", model.Description)))
				}
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}
	}

	b.WriteString(helpStyle.Render("←/→ lane • ↑/↓ model • enter/space select • tab toggle auto • d details • n next • esc back"))

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

	b.WriteString(titleStyle.Render("Step 2: Channels"))
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

	b.WriteString(titleStyle.Render("Step 3: Permission Level"))
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

	b.WriteString(titleStyle.Render("Step 4: Persona"))
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

	b.WriteString(titleStyle.Render("Step 5: Confirm Settings"))
	b.WriteString("\n\n")

	// Brain Mode
	b.WriteString(normalStyle.Render("Brain Mode: "))
	if m.brainMode == 0 {
		b.WriteString(selectedStyle.Render("Embedded"))
	} else {
		b.WriteString(selectedStyle.Render("Remote (" + m.remoteURL.Value() + ")"))
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
