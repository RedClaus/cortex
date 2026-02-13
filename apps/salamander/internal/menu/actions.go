// Package menu provides action execution and menu handling for Salamander TUI.
//
// The Executor handles action execution from menu selections and keybindings,
// supporting commands, A2A requests, variable manipulation, dialogs, and submenus.
// It also provides variable interpolation for dynamic message construction.
package menu

import (
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/normanking/salamander/pkg/schema"
)

// ═══════════════════════════════════════════════════════════════════════════════
// ACTION RESULT
// ═══════════════════════════════════════════════════════════════════════════════

// ActionResult represents the outcome of executing an action.
// It contains all relevant data for the caller to handle the result appropriately.
type ActionResult struct {
	// Type indicates what type of action was executed
	// ("command", "submenu", "a2a_request", "set_variable", "open_dialog", "quit")
	Type string

	// Message contains the message to send (for a2a_request)
	Message string

	// Command contains the command name (for command type)
	Command string

	// Args contains the command arguments
	Args map[string]interface{}

	// Dialog contains the dialog ID to open (for open_dialog)
	Dialog string

	// Variable contains the variable name (for set_variable)
	Variable string

	// Value contains the value to set (for set_variable)
	Value interface{}

	// Quit indicates whether the application should quit
	Quit bool

	// Submenu contains the submenu configuration (for submenu type)
	Submenu *schema.MenuConfig
}

// ═══════════════════════════════════════════════════════════════════════════════
// BUBBLETEA MESSAGES
// ═══════════════════════════════════════════════════════════════════════════════

// CommandMsg is sent when a command action is executed.
type CommandMsg struct {
	Command string
	Args    map[string]interface{}
}

// A2ARequestMsg is sent when an A2A request action is executed.
type A2ARequestMsg struct {
	Message string
}

// OpenDialogMsg is sent when an open_dialog action is executed.
type OpenDialogMsg struct {
	Dialog string
}

// SetVariableMsg is sent when a set_variable action is executed.
type SetVariableMsg struct {
	Variable string
	Value    interface{}
}

// QuitMsg is sent when a quit action is executed.
type QuitMsg struct{}

// ═══════════════════════════════════════════════════════════════════════════════
// COMMAND HANDLER
// ═══════════════════════════════════════════════════════════════════════════════

// CommandHandler is a function that handles a specific command.
// It receives command arguments and returns a BubbleTea command to execute.
type CommandHandler func(args map[string]interface{}) tea.Cmd

// ═══════════════════════════════════════════════════════════════════════════════
// EXECUTOR
// ═══════════════════════════════════════════════════════════════════════════════

// Executor handles action execution from menu selections and keybindings.
// It maintains variable state and registered command handlers.
type Executor struct {
	variables map[string]interface{}
	handlers  map[string]CommandHandler
}

// NewExecutor creates a new action executor with default handlers.
func NewExecutor() *Executor {
	e := &Executor{
		variables: make(map[string]interface{}),
		handlers:  make(map[string]CommandHandler),
	}
	e.registerBuiltinHandlers()
	return e
}

// registerBuiltinHandlers registers the default command handlers.
func (e *Executor) registerBuiltinHandlers() {
	// Help and information commands
	e.handlers["show_help"] = e.handleShowHelp
	e.handlers["show_config"] = e.handleShowConfig
	e.handlers["show_status"] = e.handleShowStatus

	// Chat commands
	e.handlers["clear_chat"] = e.handleClearChat
	e.handlers["stop_task"] = e.handleStopTask

	// Configuration commands
	e.handlers["set_model"] = e.handleSetModel
	e.handlers["set_theme"] = e.handleSetTheme

	// Menu navigation commands
	e.handlers["open_menu"] = e.handleOpenMenu
	e.handlers["close_menu"] = e.handleCloseMenu
	e.handlers["menu_up"] = e.handleMenuUp
	e.handlers["menu_down"] = e.handleMenuDown
	e.handlers["menu_select"] = e.handleMenuSelect
	e.handlers["menu_back"] = e.handleMenuBack

	// Focus commands
	e.handlers["focus_input"] = e.handleFocusInput

	// Application commands
	e.handlers["stop_or_quit"] = e.handleStopOrQuit
}

// ═══════════════════════════════════════════════════════════════════════════════
// PUBLIC METHODS
// ═══════════════════════════════════════════════════════════════════════════════

// Execute processes an action configuration and returns the result.
// The caller is responsible for handling the result based on its type.
func (e *Executor) Execute(action schema.ActionConfig) ActionResult {
	result := ActionResult{
		Type: action.Type,
	}

	switch action.Type {
	case "command":
		result.Command = action.Command
		result.Args = action.Args

	case "submenu":
		// Submenu is handled by the menu component directly
		// The action just signals that a submenu should be opened

	case "a2a_request":
		// Interpolate variables in the message
		result.Message = e.Interpolate(action.Message)

	case "set_variable":
		result.Variable = action.Variable
		result.Value = action.Value
		// Also set the variable locally
		e.SetVariable(action.Variable, action.Value)

	case "open_dialog":
		result.Dialog = action.Dialog

	case "quit":
		result.Quit = true
	}

	return result
}

// ExecuteCommand executes a command by name with the given arguments.
// Returns a BubbleTea command if a handler is registered, nil otherwise.
func (e *Executor) ExecuteCommand(command string, args map[string]interface{}) tea.Cmd {
	handler, ok := e.handlers[command]
	if !ok {
		return nil
	}
	return handler(args)
}

// RegisterHandler registers a custom command handler.
// This allows extending the executor with application-specific commands.
func (e *Executor) RegisterHandler(command string, handler CommandHandler) {
	e.handlers[command] = handler
}

// SetVariable sets a variable value in the executor's state.
func (e *Executor) SetVariable(name string, value interface{}) {
	e.variables[name] = value
}

// GetVariable retrieves a variable value from the executor's state.
// Returns nil if the variable is not set.
func (e *Executor) GetVariable(name string) interface{} {
	return e.variables[name]
}

// GetVariableString retrieves a variable value as a string.
// Returns an empty string if the variable is not set or is not a string.
func (e *Executor) GetVariableString(name string) string {
	v := e.variables[name]
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// Interpolate replaces ${variable} placeholders in the string with their values.
// Variables that are not found are replaced with empty strings.
func (e *Executor) Interpolate(s string) string {
	if s == "" {
		return s
	}

	// Match ${variable} pattern
	re := regexp.MustCompile(`\$\{([^}]+)\}`)

	return re.ReplaceAllStringFunc(s, func(match string) string {
		// Extract variable name from ${variable}
		varName := strings.TrimPrefix(match, "${")
		varName = strings.TrimSuffix(varName, "}")

		value := e.GetVariable(varName)
		if value == nil {
			return ""
		}

		// Convert value to string
		switch v := value.(type) {
		case string:
			return v
		case int:
			return strings.TrimSpace(strings.Repeat(" ", v)) // Simple int to string
		case bool:
			if v {
				return "true"
			}
			return "false"
		default:
			return ""
		}
	})
}

// ToCmd converts an ActionResult to a BubbleTea command.
// This is a convenience method for common action handling patterns.
func (e *Executor) ToCmd(result ActionResult) tea.Cmd {
	switch result.Type {
	case "command":
		// First try to execute registered handler
		if cmd := e.ExecuteCommand(result.Command, result.Args); cmd != nil {
			return cmd
		}
		// Otherwise return a command message for the parent to handle
		return func() tea.Msg {
			return CommandMsg{
				Command: result.Command,
				Args:    result.Args,
			}
		}

	case "a2a_request":
		return func() tea.Msg {
			return A2ARequestMsg{
				Message: result.Message,
			}
		}

	case "open_dialog":
		return func() tea.Msg {
			return OpenDialogMsg{
				Dialog: result.Dialog,
			}
		}

	case "set_variable":
		return func() tea.Msg {
			return SetVariableMsg{
				Variable: result.Variable,
				Value:    result.Value,
			}
		}

	case "quit":
		return tea.Quit

	default:
		return nil
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// BUILTIN COMMAND HANDLERS
// ═══════════════════════════════════════════════════════════════════════════════

// handleShowHelp handles the show_help command.
func (e *Executor) handleShowHelp(args map[string]interface{}) tea.Cmd {
	return func() tea.Msg {
		return OpenDialogMsg{Dialog: "help"}
	}
}

// handleClearChat handles the clear_chat command.
func (e *Executor) handleClearChat(args map[string]interface{}) tea.Cmd {
	return func() tea.Msg {
		return CommandMsg{Command: "clear_chat", Args: args}
	}
}

// handleStopTask handles the stop_task command.
func (e *Executor) handleStopTask(args map[string]interface{}) tea.Cmd {
	return func() tea.Msg {
		return CommandMsg{Command: "stop_task", Args: args}
	}
}

// handleShowConfig handles the show_config command.
func (e *Executor) handleShowConfig(args map[string]interface{}) tea.Cmd {
	return func() tea.Msg {
		return OpenDialogMsg{Dialog: "config"}
	}
}

// handleShowStatus handles the show_status command.
func (e *Executor) handleShowStatus(args map[string]interface{}) tea.Cmd {
	return func() tea.Msg {
		return OpenDialogMsg{Dialog: "status"}
	}
}

// handleSetModel handles the set_model command.
func (e *Executor) handleSetModel(args map[string]interface{}) tea.Cmd {
	provider, _ := args["provider"].(string)
	model, _ := args["model"].(string)

	e.SetVariable("model_provider", provider)
	e.SetVariable("model_name", model)

	return func() tea.Msg {
		return CommandMsg{
			Command: "set_model",
			Args:    args,
		}
	}
}

// handleSetTheme handles the set_theme command.
func (e *Executor) handleSetTheme(args map[string]interface{}) tea.Cmd {
	theme, _ := args["theme"].(string)
	e.SetVariable("theme", theme)

	return func() tea.Msg {
		return CommandMsg{
			Command: "set_theme",
			Args:    args,
		}
	}
}

// handleOpenMenu handles the open_menu command.
func (e *Executor) handleOpenMenu(args map[string]interface{}) tea.Cmd {
	return func() tea.Msg {
		return CommandMsg{
			Command: "open_menu",
			Args:    args,
		}
	}
}

// handleCloseMenu handles the close_menu command.
func (e *Executor) handleCloseMenu(args map[string]interface{}) tea.Cmd {
	return func() tea.Msg {
		return CommandMsg{Command: "close_menu", Args: nil}
	}
}

// handleMenuUp handles the menu_up command.
func (e *Executor) handleMenuUp(args map[string]interface{}) tea.Cmd {
	return func() tea.Msg {
		return CommandMsg{Command: "menu_up", Args: nil}
	}
}

// handleMenuDown handles the menu_down command.
func (e *Executor) handleMenuDown(args map[string]interface{}) tea.Cmd {
	return func() tea.Msg {
		return CommandMsg{Command: "menu_down", Args: nil}
	}
}

// handleMenuSelect handles the menu_select command.
func (e *Executor) handleMenuSelect(args map[string]interface{}) tea.Cmd {
	return func() tea.Msg {
		return CommandMsg{Command: "menu_select", Args: nil}
	}
}

// handleMenuBack handles the menu_back command.
func (e *Executor) handleMenuBack(args map[string]interface{}) tea.Cmd {
	return func() tea.Msg {
		return CommandMsg{Command: "menu_back", Args: nil}
	}
}

// handleFocusInput handles the focus_input command.
func (e *Executor) handleFocusInput(args map[string]interface{}) tea.Cmd {
	return func() tea.Msg {
		return CommandMsg{Command: "focus_input", Args: nil}
	}
}

// handleStopOrQuit handles the stop_or_quit command.
// If a task is running, it stops the task. Otherwise, it quits the application.
func (e *Executor) handleStopOrQuit(args map[string]interface{}) tea.Cmd {
	// Check if a task is running
	taskRunning, _ := e.GetVariable("task_running").(bool)

	if taskRunning {
		return func() tea.Msg {
			return CommandMsg{Command: "stop_task", Args: nil}
		}
	}

	return tea.Quit
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPER FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════════

// HasHandler returns true if a handler is registered for the given command.
func (e *Executor) HasHandler(command string) bool {
	_, ok := e.handlers[command]
	return ok
}

// ListCommands returns a list of all registered command names.
func (e *Executor) ListCommands() []string {
	commands := make([]string, 0, len(e.handlers))
	for name := range e.handlers {
		commands = append(commands, name)
	}
	return commands
}

// ClearVariables removes all variables from the executor's state.
func (e *Executor) ClearVariables() {
	e.variables = make(map[string]interface{})
}

// GetAllVariables returns a copy of all variables.
func (e *Executor) GetAllVariables() map[string]interface{} {
	copy := make(map[string]interface{}, len(e.variables))
	for k, v := range e.variables {
		copy[k] = v
	}
	return copy
}
