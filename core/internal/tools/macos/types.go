// Package macos provides macOS-specific automation tools for Cortex.
// These tools enable Cortex to control applications, execute AppleScript,
// manage the clipboard, and send notifications on macOS.
//
// Security: All tools include safety checks and respect the executor's
// security policy. Destructive operations require confirmation.
package macos

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/tools"
)

// Tool type constants for macOS automation
const (
	ToolOpenApp      tools.ToolType = "open_app"
	ToolFocusApp     tools.ToolType = "focus_app"
	ToolQuitApp      tools.ToolType = "quit_app"
	ToolListApps     tools.ToolType = "list_apps"
	ToolAppleScript  tools.ToolType = "applescript"
	ToolClipboardGet tools.ToolType = "clipboard_get"
	ToolClipboardSet tools.ToolType = "clipboard_set"
	ToolNotify       tools.ToolType = "notify"
	ToolGetFrontmost tools.ToolType = "get_frontmost"
)

// Default timeout for macOS operations
const defaultTimeout = 30 * time.Second

// runOsascript executes an AppleScript and returns the output.
func runOsascript(ctx context.Context, script string) (string, error) {
	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		return output, fmt.Errorf("osascript error: %w - output: %s", err, output)
	}
	return output, nil
}

// runOpen executes the macOS 'open' command.
func runOpen(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "open", args...)
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		return output, fmt.Errorf("open command error: %w - output: %s", err, output)
	}
	return output, nil
}

// checkMacOS returns an error if not running on macOS.
func checkMacOS() error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("macOS tools are only available on darwin, current OS: %s", runtime.GOOS)
	}
	return nil
}

// escapeAppleScriptString escapes a string for use in AppleScript.
func escapeAppleScriptString(s string) string {
	// Escape backslashes first, then quotes
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}

// blockedApps contains apps that should not be automated for security.
var blockedApps = map[string]bool{
	"Keychain Access":     true,
	"System Preferences":  true,
	"System Settings":     true,
	"Security & Privacy":  true,
	"Disk Utility":        true,
	"Terminal":            false, // Allow but be careful
	"Activity Monitor":    false,
}

// isBlockedApp checks if an app is blocked from automation.
func isBlockedApp(appName string) bool {
	return blockedApps[appName]
}
