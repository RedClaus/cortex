package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/normanking/cortex-key-vault/internal/service"
	"github.com/normanking/cortex-key-vault/internal/tui/screens"
)

// AppState represents the current screen
type AppState int

const (
	StateLogin AppState = iota
	StateDashboard
	StateAddSecret
	StateEditSecret
	StateDetailSecret
	StateSearch
	StateConfirmDelete
)

// App is the main application model
type App struct {
	state     AppState
	prevState AppState
	width     int
	height    int

	// Services
	vault     *service.VaultService
	clipboard *service.ClipboardService
	search    *service.SearchService

	// Screens
	login     screens.LoginScreen
	dashboard screens.DashboardScreen
	form      screens.FormScreen
	detail    screens.DetailScreen
	searchScr screens.SearchScreen

	// Delete confirmation
	deleteTarget *string
	deleteConfirm bool
}

// NewApp creates a new application
func NewApp() (*App, error) {
	vault, err := service.NewVaultService()
	if err != nil {
		return nil, fmt.Errorf("create vault service: %w", err)
	}

	clipboard := service.NewClipboardService()
	search := service.NewSearchService(vault)

	app := &App{
		state:     StateLogin,
		vault:     vault,
		clipboard: clipboard,
		search:    search,
	}

	// Initialize login screen
	app.login = screens.NewLoginScreen(vault)

	return app, nil
}

// Init initializes the application
func (a App) Init() tea.Cmd {
	return tea.Batch(
		a.login.Init(),
		tea.EnterAltScreen,
	)
}

// Update handles messages for the application
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Handle global messages
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		// Forward to current screen - fall through to routing

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return a, tea.Quit
		}
	}

	// Route to current screen
	switch a.state {
	case StateLogin:
		return a.updateLogin(msg)
	case StateDashboard:
		return a.updateDashboard(msg)
	case StateAddSecret, StateEditSecret:
		return a.updateForm(msg)
	case StateDetailSecret:
		return a.updateDetail(msg)
	case StateSearch:
		return a.updateSearch(msg)
	case StateConfirmDelete:
		return a.updateDeleteConfirm(msg)
	}

	return a, cmd
}

func (a App) updateLogin(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	a.login, cmd = a.login.Update(msg)

	// Check for unlock message
	if _, ok := msg.(screens.UnlockedMsg); ok {
		a.state = StateDashboard
		a.dashboard = screens.NewDashboardScreen(a.vault, a.clipboard)
		// Send window size to new screen
		return a, tea.Batch(a.dashboard.Init(), a.sendWindowSize())
	}

	return a, cmd
}

func (a App) updateDashboard(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	a.dashboard, cmd = a.dashboard.Update(msg)

	// Check for action requests
	if a.dashboard.WantsAdd() {
		a.dashboard.ClearRequests()
		a.state = StateAddSecret
		a.form = screens.NewFormScreen(a.vault)
		return a, tea.Batch(a.form.Init(), a.sendWindowSize())
	}

	if a.dashboard.WantsEdit() {
		a.dashboard.ClearRequests()
		secret := a.dashboard.GetSelectedSecret()
		if secret != nil {
			value, err := a.vault.GetSecretValue(secret.ID)
			if err != nil {
				// Can't edit without value - stay on dashboard
				return a, nil
			}
			a.state = StateEditSecret
			a.form = screens.NewEditFormScreen(a.vault, secret, value)
			return a, tea.Batch(a.form.Init(), a.sendWindowSize())
		}
	}

	if a.dashboard.WantsDelete() {
		a.dashboard.ClearRequests()
		secret := a.dashboard.GetSelectedSecret()
		if secret != nil {
			a.deleteTarget = &secret.ID
			a.state = StateConfirmDelete
		}
	}

	if a.dashboard.WantsDetail() {
		a.dashboard.ClearRequests()
		secret := a.dashboard.GetSelectedSecret()
		if secret != nil {
			a.state = StateDetailSecret
			a.detail = screens.NewDetailScreen(a.vault, a.clipboard, secret)
			return a, tea.Batch(a.detail.Init(), a.sendWindowSize())
		}
	}

	if a.dashboard.WantsSearch() {
		a.dashboard.ClearRequests()
		a.state = StateSearch
		a.searchScr = screens.NewSearchScreen(a.search)
		return a, tea.Batch(a.searchScr.Init(), a.sendWindowSize())
	}

	if a.dashboard.WantsLock() {
		a.dashboard.ClearRequests()
		a.vault.Lock()
		a.state = StateLogin
		a.login = screens.NewLoginScreen(a.vault)
		return a, tea.Batch(a.login.Init(), a.sendWindowSize())
	}

	return a, cmd
}

func (a App) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	a.form, cmd = a.form.Update(msg)

	// Check for save/cancel
	if _, ok := msg.(screens.SecretSavedMsg); ok {
		a.state = StateDashboard
		a.dashboard.Refresh()
		return a, nil
	}

	if _, ok := msg.(screens.FormCanceledMsg); ok {
		a.state = StateDashboard
		return a, nil
	}

	return a, cmd
}

func (a App) updateDetail(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	a.detail, cmd = a.detail.Update(msg)

	if _, ok := msg.(screens.DetailClosedMsg); ok {
		a.state = StateDashboard
		return a, nil
	}

	return a, cmd
}

func (a App) updateSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	a.searchScr, cmd = a.searchScr.Update(msg)

	if _, ok := msg.(screens.SearchClosedMsg); ok {
		a.state = StateDashboard
		return a, nil
	}

	if selected, ok := msg.(screens.SearchSelectedMsg); ok {
		a.state = StateDetailSecret
		a.detail = screens.NewDetailScreen(a.vault, a.clipboard, selected.Secret)
		return a, tea.Batch(a.detail.Init(), a.sendWindowSize())
	}

	return a, cmd
}

func (a App) updateDeleteConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "y", "Y":
			if a.deleteTarget != nil {
				// Attempt delete - errors are logged but we proceed
				_ = a.vault.DeleteSecret(*a.deleteTarget)
				a.dashboard.Refresh()
			}
			a.deleteTarget = nil
			a.state = StateDashboard
		case "n", "N", "esc":
			a.deleteTarget = nil
			a.state = StateDashboard
		}
	}

	return a, nil
}

// View renders the current screen
func (a App) View() string {
	switch a.state {
	case StateLogin:
		return a.login.View()
	case StateDashboard:
		return a.dashboard.View()
	case StateAddSecret, StateEditSecret:
		return a.form.View()
	case StateDetailSecret:
		return a.detail.View()
	case StateSearch:
		return a.searchScr.View()
	case StateConfirmDelete:
		return a.renderDeleteConfirm()
	}

	return ""
}

func (a App) renderDeleteConfirm() string {
	// Render dashboard in background with overlay
	bg := a.dashboard.View()

	// Simple confirmation
	confirm := `
  ╔═══════════════════════════════════════╗
  ║                                       ║
  ║    Are you sure you want to delete    ║
  ║    this secret?                       ║
  ║                                       ║
  ║    This action cannot be undone.      ║
  ║                                       ║
  ║    [Y] Yes, delete   [N] No, cancel   ║
  ║                                       ║
  ╚═══════════════════════════════════════╝
`
	_ = bg // Would overlay in a real implementation
	return confirm
}

// Close cleans up resources
func (a *App) Close() error {
	a.clipboard.Close()
	return a.vault.Close()
}

// sendWindowSize returns a command that sends a WindowSizeMsg with current dimensions
func (a App) sendWindowSize() tea.Cmd {
	w, h := a.width, a.height
	return func() tea.Msg {
		return tea.WindowSizeMsg{Width: w, Height: h}
	}
}
