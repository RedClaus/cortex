package macos

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/tools"
)

// ═══════════════════════════════════════════════════════════════════════════════
// APPLESCRIPT TOOL
// ═══════════════════════════════════════════════════════════════════════════════

// AppleScriptTool executes AppleScript code.
type AppleScriptTool struct{}

func (t *AppleScriptTool) Name() tools.ToolType { return ToolAppleScript }

// blockedScriptPatterns contains dangerous AppleScript patterns.
var blockedScriptPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)do\s+shell\s+script.*rm\s+-rf`),
	regexp.MustCompile(`(?i)do\s+shell\s+script.*sudo`),
	regexp.MustCompile(`(?i)do\s+shell\s+script.*mkfs`),
	regexp.MustCompile(`(?i)do\s+shell\s+script.*dd\s+if=`),
	regexp.MustCompile(`(?i)keystroke.*password`),
	regexp.MustCompile(`(?i)tell\s+application\s+"Keychain`),
	regexp.MustCompile(`(?i)tell\s+application\s+"System Preferences`),
	regexp.MustCompile(`(?i)tell\s+application\s+"System Settings`),
}

func (t *AppleScriptTool) Validate(req *tools.ToolRequest) error {
	if err := checkMacOS(); err != nil {
		return err
	}
	if req.Input == "" {
		return fmt.Errorf("AppleScript code is required")
	}

	// Check for blocked patterns
	for _, pattern := range blockedScriptPatterns {
		if pattern.MatchString(req.Input) {
			return fmt.Errorf("script contains blocked pattern: %s", pattern.String())
		}
	}

	return nil
}

func (t *AppleScriptTool) AssessRisk(req *tools.ToolRequest) tools.RiskLevel {
	script := strings.ToLower(req.Input)

	// High risk patterns
	if strings.Contains(script, "do shell script") {
		return tools.RiskHigh
	}
	if strings.Contains(script, "keystroke") || strings.Contains(script, "key code") {
		return tools.RiskMedium
	}
	if strings.Contains(script, "click") {
		return tools.RiskMedium
	}
	if strings.Contains(script, "delete") || strings.Contains(script, "remove") {
		return tools.RiskMedium
	}

	// Read operations are low risk
	if strings.Contains(script, "get ") || strings.Contains(script, "return ") {
		return tools.RiskLow
	}

	return tools.RiskMedium
}

func (t *AppleScriptTool) Execute(ctx context.Context, req *tools.ToolRequest) (*tools.ToolResult, error) {
	start := time.Now()
	script := req.Input

	output, err := runOsascript(ctx, script)
	if err != nil {
		return &tools.ToolResult{
			Tool:     ToolAppleScript,
			Success:  false,
			Error:    fmt.Sprintf("AppleScript execution failed: %v", err),
			Output:   output,
			Duration: time.Since(start),
		}, err
	}

	return &tools.ToolResult{
		Tool:     ToolAppleScript,
		Success:  true,
		Output:   output,
		Duration: time.Since(start),
		Metadata: map[string]interface{}{
			"script_length": len(script),
		},
	}, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPER SCRIPTS - Pre-built safe AppleScript templates
// ═══════════════════════════════════════════════════════════════════════════════

// ScriptTemplates provides safe, pre-built AppleScript templates.
var ScriptTemplates = map[string]string{
	// Window management
	"minimize_all": `tell application "System Events"
		set visible of every process whose visible is true to false
	end tell`,

	"get_window_list": `tell application "System Events"
		set windowList to {}
		repeat with proc in (every process whose background only is false)
			set procName to name of proc
			try
				set winNames to name of every window of proc
				repeat with winName in winNames
					set end of windowList to procName & ": " & winName
				end repeat
			end try
		end repeat
		set AppleScript's text item delimiters to linefeed
		return windowList as text
	end tell`,

	// Volume control
	"get_volume": `output volume of (get volume settings)`,

	"set_volume": `set volume output volume %d`, // Use with fmt.Sprintf

	"mute": `set volume output muted true`,

	"unmute": `set volume output muted false`,

	// Display
	"get_screen_size": `tell application "Finder"
		set screenBounds to bounds of window of desktop
		return item 3 of screenBounds & "x" & item 4 of screenBounds
	end tell`,

	// Safari
	"safari_current_url": `tell application "Safari"
		return URL of current tab of front window
	end tell`,

	"safari_current_title": `tell application "Safari"
		return name of current tab of front window
	end tell`,

	// Finder
	"finder_selection": `tell application "Finder"
		set selectedItems to selection
		set itemPaths to {}
		repeat with anItem in selectedItems
			set end of itemPaths to POSIX path of (anItem as alias)
		end repeat
		set AppleScript's text item delimiters to linefeed
		return itemPaths as text
	end tell`,

	"finder_current_folder": `tell application "Finder"
		return POSIX path of (target of front window as alias)
	end tell`,
}

// RunTemplate executes a pre-built template with optional formatting.
func RunTemplate(ctx context.Context, templateName string, args ...interface{}) (string, error) {
	template, ok := ScriptTemplates[templateName]
	if !ok {
		return "", fmt.Errorf("unknown template: %s", templateName)
	}

	script := template
	if len(args) > 0 {
		script = fmt.Sprintf(template, args...)
	}

	return runOsascript(ctx, script)
}
