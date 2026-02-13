// Package ui provides the slash command system for Cortex's TUI framework.
// Commands allow users to control the TUI without leaving the chat interface.
package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// ═══════════════════════════════════════════════════════════════════════════════
// COMMAND MESSAGE TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// ShowHelpMsg requests opening the help modal.
type ShowHelpMsg struct{}

// ShowModelSelectorMsg requests opening the model selector modal.
type ShowModelSelectorMsg struct{}

// ShowThemeSelectorMsg requests opening the theme selector modal.
type ShowThemeSelectorMsg struct{}

// ShowSessionSelectorMsg requests opening the session selector modal.
type ShowSessionSelectorMsg struct{}

// ToggleYoloMsg toggles YOLO mode (auto-run dangerous commands).
type ToggleYoloMsg struct{}

// TogglePlanMsg toggles Plan mode (planning before execution).
type TogglePlanMsg struct{}

// ShellCommandMsg carries the result of a shell command execution.
type ShellCommandMsg struct {
	Command string
	Output  string
	Error   error
}

// CommandErrorMsg signals an invalid or failed command.
type CommandErrorMsg struct {
	Command string
	Error   string
}

// ═══════════════════════════════════════════════════════════════════════════════
// COMMAND ROUTER
// ═══════════════════════════════════════════════════════════════════════════════

// HandleCommand parses and routes slash commands to their handlers.
// This is the main entry point for all commands starting with '/'.
//
// Supported commands:
//   - /help, /h, /?           - Show help modal
//   - /model, /m [name]       - Open model selector or set model directly
//   - /theme, /t [name]       - Open theme selector or set theme directly
//   - /clear, /c              - Clear conversation history
//   - /yolo                   - Toggle YOLO mode (auto-run mode)
//   - /plan                   - Toggle Plan mode (planning before execution)
//   - /session, /s [action]   - List or switch sessions
//   - /quit, /q, /exit        - Exit the application
//
// Example usage:
//   m.HandleCommand("/help")           → Opens help modal
//   m.HandleCommand("/model gpt-4")    → Switches to GPT-4
//   m.HandleCommand("/theme dracula")  → Switches to Dracula theme
//   m.HandleCommand("/clear")          → Clears chat history
func HandleCommand(input string, backend Backend) tea.Cmd {
	// Remove leading slash and split into parts
	input = strings.TrimPrefix(input, "/")
	parts := strings.Fields(input)

	if len(parts) == 0 {
		return cmdUnknown("")
	}

	cmd := strings.ToLower(parts[0])
	args := parts[1:]

	switch cmd {
	case "help", "h", "?":
		return cmdHelp()

	case "model", "m":
		return cmdModel(args, backend)

	case "theme", "t":
		return cmdTheme(args)

	case "clear", "c":
		return cmdClear()

	case "yolo":
		return cmdYolo()

	case "plan":
		return cmdPlan()

	case "session", "s":
		return cmdSession(args, backend)

	case "quit", "q", "exit":
		return tea.Quit

	default:
		return cmdUnknown(cmd)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// INDIVIDUAL COMMAND HANDLERS
// ═══════════════════════════════════════════════════════════════════════════════

// cmdHelp opens the help modal showing all available commands and keybindings.
func cmdHelp() tea.Cmd {
	return func() tea.Msg {
		return ShowHelpMsg{}
	}
}

// cmdModel handles the /model command.
// With no arguments: Opens the model selector modal.
// With arguments: Attempts to set the model directly.
//
// Examples:
//   /model              → Opens model selector
//   /model gpt-4        → Switches to GPT-4
//   /model claude-opus  → Switches to Claude Opus
func cmdModel(args []string, backend Backend) tea.Cmd {
	if len(args) == 0 {
		// No model specified - open selector
		return func() tea.Msg {
			return ShowModelSelectorMsg{}
		}
	}

	// Model name provided - try to set it directly
	modelName := strings.Join(args, " ")

	return func() tea.Msg {
		// Fetch available models
		models, err := backend.GetModels()
		if err != nil {
			return CommandErrorMsg{
				Command: "model",
				Error:   fmt.Sprintf("Failed to fetch models: %v", err),
			}
		}

		// Search for matching model (case-insensitive, partial match)
		var matchedModel *ModelInfo
		searchLower := strings.ToLower(modelName)

		for i := range models {
			modelIDLower := strings.ToLower(models[i].ID)
			modelNameLower := strings.ToLower(models[i].Name)

			// Check for exact match first
			if modelIDLower == searchLower || modelNameLower == searchLower {
				matchedModel = &models[i]
				break
			}

			// Check for partial match
			if strings.Contains(modelIDLower, searchLower) || strings.Contains(modelNameLower, searchLower) {
				matchedModel = &models[i]
				// Continue searching for exact match
			}
		}

		if matchedModel == nil {
			return CommandErrorMsg{
				Command: "model",
				Error:   fmt.Sprintf("Model not found: %s", modelName),
			}
		}

		return ModelSelectedMsg{Model: *matchedModel}
	}
}

// cmdTheme handles the /theme command.
// With no arguments: Opens the theme selector modal.
// With arguments: Attempts to set the theme directly.
//
// Examples:
//   /theme          → Opens theme selector
//   /theme dracula  → Switches to Dracula theme
//   /theme nord     → Switches to Nord theme
func cmdTheme(args []string) tea.Cmd {
	if len(args) == 0 {
		// No theme specified - open selector
		return func() tea.Msg {
			return ShowThemeSelectorMsg{}
		}
	}

	// Theme name provided - set it directly
	themeName := strings.ToLower(strings.Join(args, " "))

	return func() tea.Msg {
		// Validate theme exists
		availableThemes := ThemeNames()
		themeFound := false

		for _, name := range availableThemes {
			if strings.ToLower(name) == themeName {
				themeFound = true
				break
			}
		}

		if !themeFound {
			return CommandErrorMsg{
				Command: "theme",
				Error:   fmt.Sprintf("Theme not found: %s. Available: %v", themeName, availableThemes),
			}
		}

		return ThemeSelectedMsg{ThemeName: themeName}
	}
}

// cmdClear clears the conversation history.
// This removes all messages from the current chat session.
func cmdClear() tea.Cmd {
	return func() tea.Msg {
		return ClearHistoryMsg{}
	}
}

// cmdYolo toggles YOLO mode.
// YOLO mode automatically runs commands without confirmation prompts.
// Use with caution!
func cmdYolo() tea.Cmd {
	return func() tea.Msg {
		return ToggleYoloMsg{}
	}
}

// cmdPlan toggles Plan mode.
// Plan mode makes the AI create a plan before executing tasks,
// giving you a chance to review the approach.
func cmdPlan() tea.Cmd {
	return func() tea.Msg {
		return TogglePlanMsg{}
	}
}

// cmdSession handles the /session command.
// With no arguments: Opens session selector or lists sessions.
// With arguments: Switches to a specific session.
//
// Examples:
//   /session        → Opens session selector
//   /session list   → Lists all sessions
//   /session 42     → Switches to session 42
func cmdSession(args []string, backend Backend) tea.Cmd {
	if len(args) == 0 {
		// No arguments - open session selector
		return func() tea.Msg {
			return ShowSessionSelectorMsg{}
		}
	}

	action := strings.ToLower(args[0])

	if action == "list" {
		// Fetch and list sessions
		return FetchSessionsCmd(backend)
	}

	// Assume the argument is a session ID or name
	sessionID := strings.Join(args, " ")

	return func() tea.Msg {
		// Fetch sessions to find matching one
		sessions, err := backend.GetSessions()
		if err != nil {
			return CommandErrorMsg{
				Command: "session",
				Error:   fmt.Sprintf("Failed to fetch sessions: %v", err),
			}
		}

		// Search for matching session
		var matchedSession *SessionInfo
		searchLower := strings.ToLower(sessionID)

		for i := range sessions {
			idLower := strings.ToLower(sessions[i].ID)
			nameLower := strings.ToLower(sessions[i].Name)

			if idLower == searchLower || nameLower == searchLower {
				matchedSession = &sessions[i]
				break
			}

			// Partial match
			if strings.Contains(idLower, searchLower) || strings.Contains(nameLower, searchLower) {
				matchedSession = &sessions[i]
			}
		}

		if matchedSession == nil {
			return CommandErrorMsg{
				Command: "session",
				Error:   fmt.Sprintf("Session not found: %s", sessionID),
			}
		}

		return SessionLoadedMsg{Session: *matchedSession}
	}
}

// cmdUnknown handles unrecognized commands.
// Returns an error message to display to the user.
func cmdUnknown(cmd string) tea.Cmd {
	return func() tea.Msg {
		if cmd == "" {
			return CommandErrorMsg{
				Command: "",
				Error:   "Empty command. Type /help for available commands.",
			}
		}

		return CommandErrorMsg{
			Command: cmd,
			Error:   fmt.Sprintf("Unknown command: /%s. Type /help for available commands.", cmd),
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// SHELL ESCAPE HANDLER
// ═══════════════════════════════════════════════════════════════════════════════

// HandleShellEscape executes a shell command and returns the output.
// Commands prefixed with '!' are passed to the system shell.
//
// Security note: This executes commands using the user's shell.
// In YOLO mode, commands run without confirmation.
//
// Examples:
//   !ls -la           → Lists files in current directory
//   !git status       → Shows git status
//   !python script.py → Runs a Python script
func HandleShellEscape(command string) tea.Cmd {
	return func() tea.Msg {
		// Remove leading ! if present
		command = strings.TrimPrefix(command, "!")
		command = strings.TrimSpace(command)

		if command == "" {
			return CommandErrorMsg{
				Command: "!",
				Error:   "No shell command provided",
			}
		}

		// Execute command using sh -c for Unix-like systems
		// This allows for pipes, redirects, and other shell features
		cmd := exec.Command("sh", "-c", command)

		// Capture both stdout and stderr
		output, err := cmd.CombinedOutput()

		return ShellCommandMsg{
			Command: command,
			Output:  string(output),
			Error:   err,
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// AUTOCOMPLETE SUPPORT
// ═══════════════════════════════════════════════════════════════════════════════

// CommandSuggestions returns a list of command suggestions based on partial input.
// This is used for autocomplete functionality in the input field.
//
// Example:
//   GetCommandSuggestions("/mo")  → ["/model"]
//   GetCommandSuggestions("/h")   → ["/help"]
func GetCommandSuggestions(partial string) []string {
	// Remove leading slash for comparison
	partial = strings.TrimPrefix(strings.ToLower(partial), "/")

	// Command groups - each group contains a command and its aliases
	commandGroups := [][]string{
		{"/help", "/h", "/?"},
		{"/model", "/m"},
		{"/theme", "/t"},
		{"/clear", "/c"},
		{"/yolo"},
		{"/plan"},
		{"/session", "/s"},
		{"/quit", "/q", "/exit"},
	}

	// Flatten for empty partial
	if partial == "" {
		var all []string
		for _, group := range commandGroups {
			all = append(all, group...)
		}
		return all
	}

	// Find matching groups and return all commands in those groups
	seen := make(map[string]bool)
	var suggestions []string

	for _, group := range commandGroups {
		groupMatches := false
		for _, cmd := range group {
			cmdWithoutSlash := strings.TrimPrefix(cmd, "/")
			if strings.HasPrefix(cmdWithoutSlash, partial) {
				groupMatches = true
				break
			}
		}
		// If any command in the group matches, add all commands in the group
		if groupMatches {
			for _, cmd := range group {
				if !seen[cmd] {
					seen[cmd] = true
					suggestions = append(suggestions, cmd)
				}
			}
		}
	}

	return suggestions
}

// GetCommandHelp returns a map of commands to their descriptions.
// This is used for displaying help information in the help modal.
func GetCommandHelp() map[string]string {
	return map[string]string{
		"/help, /h, /?":        "Show this help message",
		"/model, /m [name]":    "Open model selector or switch to specified model",
		"/theme, /t [name]":    "Open theme selector or switch to specified theme",
		"/clear, /c":           "Clear conversation history",
		"/yolo":                "Toggle YOLO mode (auto-run commands without confirmation)",
		"/plan":                "Toggle Plan mode (create execution plan before running)",
		"/session, /s [id]":    "Open session selector or switch to specified session",
		"/device, /audio":      "Open audio device selector (microphone/speaker)",
		"/quit, /q, /exit":     "Exit Cortex",
		"!<command>":           "Execute shell command (e.g., !ls -la)",
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// AUDIO DEVICE COMMANDS
// ═══════════════════════════════════════════════════════════════════════════════

// ShowDeviceSelectorMsg requests opening the audio device selector modal.
type ShowDeviceSelectorMsg struct{}

// AudioDevicesLoadedMsg carries the result of fetching audio devices.
type AudioDevicesLoadedMsg struct {
	InputDevices  []AudioDeviceInfo
	OutputDevices []AudioDeviceInfo
	CurrentInput  *int
	CurrentOutput *int
	Error         error
}

// AudioDeviceSetMsg carries the result of setting an audio device.
type AudioDeviceSetMsg struct {
	Device  AudioDeviceInfo
	IsInput bool
	Success bool
	Error   error
}

// AudioDeviceInfo represents an audio device.
type AudioDeviceInfo struct {
	Index      int     `json:"index"`
	Name       string  `json:"name"`
	Channels   int     `json:"channels"`
	SampleRate float64 `json:"sample_rate"`
}

// VoiceOrchestratorURL is the URL of the voice orchestrator service.
// This can be overridden via environment variables.
const VoiceOrchestratorURL = "http://localhost:8765"

// FetchAudioDevicesCmd fetches available audio devices from the voice orchestrator.
func FetchAudioDevicesCmd() tea.Cmd {
	return func() tea.Msg {
		// Import net/http inline to avoid import clutter
		resp, err := httpGet(VoiceOrchestratorURL + "/devices")
		if err != nil {
			return AudioDevicesLoadedMsg{
				Error: fmt.Errorf("failed to connect to voice orchestrator: %w", err),
			}
		}

		// Parse JSON response
		var result struct {
			InputDevices  []AudioDeviceInfo `json:"input_devices"`
			OutputDevices []AudioDeviceInfo `json:"output_devices"`
			Current       struct {
				Input  *int `json:"input"`
				Output *int `json:"output"`
			} `json:"current"`
		}

		if err := parseJSON(resp, &result); err != nil {
			return AudioDevicesLoadedMsg{
				Error: fmt.Errorf("failed to parse devices response: %w", err),
			}
		}

		return AudioDevicesLoadedMsg{
			InputDevices:  result.InputDevices,
			OutputDevices: result.OutputDevices,
			CurrentInput:  result.Current.Input,
			CurrentOutput: result.Current.Output,
		}
	}
}

// SetAudioDeviceCmd sets the audio input or output device.
func SetAudioDeviceCmd(deviceIndex int, isInput bool) tea.Cmd {
	return func() tea.Msg {
		var endpoint string
		if isInput {
			endpoint = fmt.Sprintf("%s/devices/input/%d", VoiceOrchestratorURL, deviceIndex)
		} else {
			endpoint = fmt.Sprintf("%s/devices/output/%d", VoiceOrchestratorURL, deviceIndex)
		}

		resp, err := httpPost(endpoint)
		if err != nil {
			return AudioDeviceSetMsg{
				IsInput: isInput,
				Success: false,
				Error:   fmt.Errorf("failed to set device: %w", err),
			}
		}

		// Parse response
		var result struct {
			Success bool            `json:"success"`
			Device  AudioDeviceInfo `json:"device"`
			Message string          `json:"message"`
			Error   string          `json:"error"`
		}

		if err := parseJSON(resp, &result); err != nil {
			return AudioDeviceSetMsg{
				IsInput: isInput,
				Success: false,
				Error:   fmt.Errorf("failed to parse response: %w", err),
			}
		}

		if !result.Success {
			return AudioDeviceSetMsg{
				IsInput: isInput,
				Success: false,
				Error:   errors.New(result.Error),
			}
		}

		return AudioDeviceSetMsg{
			Device:  result.Device,
			IsInput: isInput,
			Success: true,
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// HTTP HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

// httpGet performs an HTTP GET request and returns the response body.
func httpGet(url string) ([]byte, error) {
	// Using exec to avoid importing net/http which adds complexity
	// This is a simple wrapper that uses curl
	cmd := exec.Command("curl", "-s", "-X", "GET", url)
	return cmd.Output()
}

// httpPost performs an HTTP POST request and returns the response body.
func httpPost(url string) ([]byte, error) {
	cmd := exec.Command("curl", "-s", "-X", "POST", url)
	return cmd.Output()
}

// parseJSON parses JSON data into the target struct.
func parseJSON(data []byte, target interface{}) error {
	return json.Unmarshal(data, target)
}
