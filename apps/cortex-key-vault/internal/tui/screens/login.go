package screens

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/normanking/cortex-key-vault/internal/service"
	"github.com/normanking/cortex-key-vault/internal/tui/styles"
)

// LoginMode represents the current login screen mode
type LoginMode int

const (
	LoginModeUnlock LoginMode = iota
	LoginModeCreate
	LoginModeConfirm
)

// LoginScreen is the login/unlock screen model
type LoginScreen struct {
	vault         *service.VaultService
	mode          LoginMode
	passwordInput textinput.Model
	confirmInput  textinput.Model
	err           string
	width         int
	height        int
	focused       int // 0 = password, 1 = confirm (for create mode)
}

// UnlockedMsg is sent when the vault is successfully unlocked
type UnlockedMsg struct{}

// NewLoginScreen creates a new login screen
func NewLoginScreen(vault *service.VaultService) LoginScreen {
	pi := textinput.New()
	pi.Placeholder = "Enter master password"
	pi.EchoMode = textinput.EchoPassword
	pi.EchoCharacter = '•'
	pi.Focus()

	ci := textinput.New()
	ci.Placeholder = "Confirm master password"
	ci.EchoMode = textinput.EchoPassword
	ci.EchoCharacter = '•'

	mode := LoginModeUnlock
	if !vault.IsInitialized() {
		mode = LoginModeCreate
	}

	return LoginScreen{
		vault:         vault,
		mode:          mode,
		passwordInput: pi,
		confirmInput:  ci,
	}
}

// Init initializes the login screen
func (m LoginScreen) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages for the login screen
func (m LoginScreen) Update(msg tea.Msg) (LoginScreen, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			return m.handleSubmit()
		case "tab", "shift+tab":
			if m.mode == LoginModeCreate {
				m.focused = (m.focused + 1) % 2
				if m.focused == 0 {
					m.passwordInput.Focus()
					m.confirmInput.Blur()
				} else {
					m.passwordInput.Blur()
					m.confirmInput.Focus()
				}
			}
		case "esc":
			m.err = ""
		}
	}

	// Update the focused input
	if m.focused == 0 {
		m.passwordInput, cmd = m.passwordInput.Update(msg)
	} else {
		m.confirmInput, cmd = m.confirmInput.Update(msg)
	}

	return m, cmd
}

func (m LoginScreen) handleSubmit() (LoginScreen, tea.Cmd) {
	password := m.passwordInput.Value()

	switch m.mode {
	case LoginModeUnlock:
		if password == "" {
			m.err = "Password cannot be empty"
			return m, nil
		}

		if err := m.vault.Unlock(password); err != nil {
			m.err = "Incorrect password"
			m.passwordInput.SetValue("")
			return m, nil
		}

		return m, func() tea.Msg { return UnlockedMsg{} }

	case LoginModeCreate:
		if password == "" {
			m.err = "Password cannot be empty"
			return m, nil
		}

		if len(password) < 8 {
			m.err = "Password must be at least 8 characters"
			return m, nil
		}

		confirm := m.confirmInput.Value()
		if confirm == "" {
			// Move to confirm
			m.focused = 1
			m.passwordInput.Blur()
			m.confirmInput.Focus()
			return m, nil
		}

		if password != confirm {
			m.err = "Passwords do not match"
			m.confirmInput.SetValue("")
			return m, nil
		}

		// Initialize the vault
		if err := m.vault.Initialize(password); err != nil {
			m.err = "Failed to create vault: " + err.Error()
			return m, nil
		}

		// Unlock after creation
		if err := m.vault.Unlock(password); err != nil {
			m.err = "Failed to unlock vault: " + err.Error()
			return m, nil
		}

		return m, func() tea.Msg { return UnlockedMsg{} }
	}

	return m, nil
}

// View renders the login screen
func (m LoginScreen) View() string {
	// Handle zero dimensions
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	// ASCII art logo
	logo := `
   ██████╗ ██████╗ ██████╗ ████████╗███████╗██╗  ██╗
  ██╔════╝██╔═══██╗██╔══██╗╚══██╔══╝██╔════╝╚██╗██╔╝
  ██║     ██║   ██║██████╔╝   ██║   █████╗   ╚███╔╝
  ██║     ██║   ██║██╔══██╗   ██║   ██╔══╝   ██╔██╗
  ╚██████╗╚██████╔╝██║  ██║   ██║   ███████╗██╔╝ ██╗
   ╚═════╝ ╚═════╝ ╚═╝  ╚═╝   ╚═╝   ╚══════╝╚═╝  ╚═╝

        ██╗  ██╗███████╗██╗   ██╗
        ██║ ██╔╝██╔════╝╚██╗ ██╔╝
        █████╔╝ █████╗   ╚████╔╝
        ██╔═██╗ ██╔══╝    ╚██╔╝
        ██║  ██╗███████╗   ██║
        ╚═╝  ╚═╝╚══════╝   ╚═╝

    ██╗   ██╗ █████╗ ██╗   ██╗██╗  ████████╗
    ██║   ██║██╔══██╗██║   ██║██║  ╚══██╔══╝
    ██║   ██║███████║██║   ██║██║     ██║
    ╚██╗ ██╔╝██╔══██║██║   ██║██║     ██║
     ╚████╔╝ ██║  ██║╚██████╔╝███████╗██║
      ╚═══╝  ╚═╝  ╚═╝ ╚═════╝ ╚══════╝╚═╝
`

	styledLogo := styles.Logo.Render(logo)

	// Build the form
	var formContent strings.Builder

	if m.mode == LoginModeCreate {
		formContent.WriteString(styles.HeaderTitle.Render("Create Your Vault") + "\n\n")
		formContent.WriteString(styles.TextSecondaryStyle.Render("Choose a strong master password to secure your secrets.") + "\n")
		formContent.WriteString(styles.TextMutedStyle.Render("Minimum 8 characters required.") + "\n\n")
	} else {
		formContent.WriteString(styles.HeaderTitle.Render("Unlock Your Vault") + "\n\n")
		formContent.WriteString(styles.TextSecondaryStyle.Render("Enter your master password to access your secrets.") + "\n\n")
	}

	// Password input
	inputStyle := styles.Input
	if m.focused == 0 {
		inputStyle = styles.InputFocused
	}
	formContent.WriteString(styles.InputLabel.Render("Master Password") + "\n")
	formContent.WriteString(inputStyle.Width(40).Render(m.passwordInput.View()) + "\n\n")

	// Confirm input (create mode only)
	if m.mode == LoginModeCreate {
		inputStyle = styles.Input
		if m.focused == 1 {
			inputStyle = styles.InputFocused
		}
		formContent.WriteString(styles.InputLabel.Render("Confirm Password") + "\n")
		formContent.WriteString(inputStyle.Width(40).Render(m.confirmInput.View()) + "\n\n")
	}

	// Error message
	if m.err != "" {
		formContent.WriteString(styles.ErrorText.Render("⚠ " + m.err) + "\n\n")
	}

	// Help text
	if m.mode == LoginModeCreate {
		formContent.WriteString(styles.HelpText.Render("[Tab] Switch fields  [Enter] Submit  [Ctrl+C] Quit"))
	} else {
		formContent.WriteString(styles.HelpText.Render("[Enter] Unlock  [Ctrl+C] Quit"))
	}

	// Create the form panel
	formPanel := styles.Panel.
		Width(50).
		Render(formContent.String())

	// Center everything
	content := lipgloss.JoinVertical(
		lipgloss.Center,
		styledLogo,
		"\n",
		formPanel,
	)

	// Center in the screen
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		content,
	)
}

