package macos

import (
	"runtime"

	"github.com/normanking/cortex/internal/tools"
)

// RegisterAll registers all macOS tools with the executor.
// Returns an error if not running on macOS.
func RegisterAll(executor *tools.Executor) error {
	if runtime.GOOS != "darwin" {
		// Silently skip on non-macOS systems
		return nil
	}

	// App management tools
	if err := executor.Register(&OpenAppTool{}); err != nil {
		return err
	}
	if err := executor.Register(&FocusAppTool{}); err != nil {
		return err
	}
	if err := executor.Register(&QuitAppTool{}); err != nil {
		return err
	}
	if err := executor.Register(&ListAppsTool{}); err != nil {
		return err
	}
	if err := executor.Register(&GetFrontmostTool{}); err != nil {
		return err
	}

	// Script tools
	if err := executor.Register(&AppleScriptTool{}); err != nil {
		return err
	}

	// Clipboard tools
	if err := executor.Register(&ClipboardGetTool{}); err != nil {
		return err
	}
	if err := executor.Register(&ClipboardSetTool{}); err != nil {
		return err
	}

	// Notification tools
	if err := executor.Register(&NotifyTool{}); err != nil {
		return err
	}
	if err := executor.Register(&DialogTool{}); err != nil {
		return err
	}
	if err := executor.Register(&SayTool{}); err != nil {
		return err
	}

	return nil
}

// AllTools returns all macOS tool instances.
func AllTools() []tools.Tool {
	if runtime.GOOS != "darwin" {
		return nil
	}

	return []tools.Tool{
		&OpenAppTool{},
		&FocusAppTool{},
		&QuitAppTool{},
		&ListAppsTool{},
		&GetFrontmostTool{},
		&AppleScriptTool{},
		&ClipboardGetTool{},
		&ClipboardSetTool{},
		&NotifyTool{},
		&DialogTool{},
		&SayTool{},
	}
}

// ToolDescriptions returns human-readable descriptions for LLM tool selection.
func ToolDescriptions() map[tools.ToolType]string {
	return map[tools.ToolType]string{
		ToolOpenApp: `Opens a macOS application by name.
Input: Application name (e.g., "Safari", "Visual Studio Code", "Slack")
Example: {"tool": "open_app", "input": "Safari"}`,

		ToolFocusApp: `Brings an application to the foreground (activates it).
Input: Application name
Example: {"tool": "focus_app", "input": "Finder"}`,

		ToolQuitApp: `Gracefully quits an application.
Input: Application name
Params: {"force": true} to force quit
Example: {"tool": "quit_app", "input": "Safari", "params": {"force": false}}`,

		ToolListApps: `Lists all currently running applications.
Input: (none required)
Example: {"tool": "list_apps"}`,

		ToolGetFrontmost: `Returns the name of the currently active (frontmost) application.
Input: (none required)
Example: {"tool": "get_frontmost"}`,

		ToolAppleScript: `Executes AppleScript code for macOS automation.
Input: AppleScript code
Example: {"tool": "applescript", "input": "tell application \"Finder\" to get name of front window"}`,

		ToolClipboardGet: `Reads the current contents of the system clipboard.
Input: (none required)
Example: {"tool": "clipboard_get"}`,

		ToolClipboardSet: `Copies text to the system clipboard.
Input: Text to copy
Example: {"tool": "clipboard_set", "input": "Hello, world!"}`,

		ToolNotify: `Sends a macOS notification.
Input: Notification message
Params: {"title": "...", "subtitle": "...", "sound": "default", "silent": false}
Example: {"tool": "notify", "input": "Task completed!", "params": {"title": "Cortex"}}`,
	}
}

// GetToolDescription returns the description for a specific tool.
func GetToolDescription(toolType tools.ToolType) string {
	descs := ToolDescriptions()
	if desc, ok := descs[toolType]; ok {
		return desc
	}
	return ""
}
