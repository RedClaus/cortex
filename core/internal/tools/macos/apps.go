package macos

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/tools"
)

// ═══════════════════════════════════════════════════════════════════════════════
// OPEN APP TOOL
// ═══════════════════════════════════════════════════════════════════════════════

// OpenAppTool opens a macOS application by name.
type OpenAppTool struct{}

func (t *OpenAppTool) Name() tools.ToolType { return ToolOpenApp }

func (t *OpenAppTool) Validate(req *tools.ToolRequest) error {
	if err := checkMacOS(); err != nil {
		return err
	}
	if req.Input == "" {
		return fmt.Errorf("app name is required")
	}
	if isBlockedApp(req.Input) {
		return fmt.Errorf("app '%s' is blocked for security reasons", req.Input)
	}
	return nil
}

func (t *OpenAppTool) AssessRisk(req *tools.ToolRequest) tools.RiskLevel {
	// Opening apps is generally low risk
	appName := strings.ToLower(req.Input)
	if strings.Contains(appName, "terminal") || strings.Contains(appName, "iterm") {
		return tools.RiskMedium
	}
	return tools.RiskLow
}

func (t *OpenAppTool) Execute(ctx context.Context, req *tools.ToolRequest) (*tools.ToolResult, error) {
	start := time.Now()
	appName := req.Input

	// Use 'open -a' to launch the application
	output, err := runOpen(ctx, "-a", appName)
	if err != nil {
		return &tools.ToolResult{
			Tool:     ToolOpenApp,
			Success:  false,
			Error:    fmt.Sprintf("failed to open %s: %v", appName, err),
			Duration: time.Since(start),
		}, err
	}

	return &tools.ToolResult{
		Tool:     ToolOpenApp,
		Success:  true,
		Output:   fmt.Sprintf("Opened application: %s", appName),
		Duration: time.Since(start),
		Metadata: map[string]interface{}{
			"app_name":   appName,
			"raw_output": output,
		},
	}, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// FOCUS APP TOOL
// ═══════════════════════════════════════════════════════════════════════════════

// FocusAppTool brings an application to the foreground.
type FocusAppTool struct{}

func (t *FocusAppTool) Name() tools.ToolType { return ToolFocusApp }

func (t *FocusAppTool) Validate(req *tools.ToolRequest) error {
	if err := checkMacOS(); err != nil {
		return err
	}
	if req.Input == "" {
		return fmt.Errorf("app name is required")
	}
	if isBlockedApp(req.Input) {
		return fmt.Errorf("app '%s' is blocked for security reasons", req.Input)
	}
	return nil
}

func (t *FocusAppTool) AssessRisk(req *tools.ToolRequest) tools.RiskLevel {
	return tools.RiskLow
}

func (t *FocusAppTool) Execute(ctx context.Context, req *tools.ToolRequest) (*tools.ToolResult, error) {
	start := time.Now()
	appName := req.Input

	// Use AppleScript to activate the app
	script := fmt.Sprintf(`tell application "%s" to activate`, escapeAppleScriptString(appName))
	output, err := runOsascript(ctx, script)
	if err != nil {
		return &tools.ToolResult{
			Tool:     ToolFocusApp,
			Success:  false,
			Error:    fmt.Sprintf("failed to focus %s: %v", appName, err),
			Duration: time.Since(start),
		}, err
	}

	return &tools.ToolResult{
		Tool:     ToolFocusApp,
		Success:  true,
		Output:   fmt.Sprintf("Focused application: %s", appName),
		Duration: time.Since(start),
		Metadata: map[string]interface{}{
			"app_name":   appName,
			"raw_output": output,
		},
	}, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// QUIT APP TOOL
// ═══════════════════════════════════════════════════════════════════════════════

// QuitAppTool gracefully quits an application.
type QuitAppTool struct{}

func (t *QuitAppTool) Name() tools.ToolType { return ToolQuitApp }

func (t *QuitAppTool) Validate(req *tools.ToolRequest) error {
	if err := checkMacOS(); err != nil {
		return err
	}
	if req.Input == "" {
		return fmt.Errorf("app name is required")
	}
	// Don't allow quitting critical system apps
	criticalApps := map[string]bool{
		"Finder":        true,
		"loginwindow":   true,
		"SystemUIServer": true,
		"Dock":          true,
	}
	if criticalApps[req.Input] {
		return fmt.Errorf("cannot quit system-critical app: %s", req.Input)
	}
	return nil
}

func (t *QuitAppTool) AssessRisk(req *tools.ToolRequest) tools.RiskLevel {
	return tools.RiskMedium // Quitting apps can cause data loss
}

func (t *QuitAppTool) Execute(ctx context.Context, req *tools.ToolRequest) (*tools.ToolResult, error) {
	start := time.Now()
	appName := req.Input

	// Check if force quit is requested
	force := false
	if val, ok := req.Params["force"].(bool); ok {
		force = val
	}

	var script string
	if force {
		// Force quit using System Events
		script = fmt.Sprintf(`tell application "System Events" to set quitApp to name of every process whose name is "%s"
if (count of quitApp) > 0 then
    do shell script "killall '%s'"
end if`, escapeAppleScriptString(appName), escapeAppleScriptString(appName))
	} else {
		// Graceful quit
		script = fmt.Sprintf(`tell application "%s" to quit`, escapeAppleScriptString(appName))
	}

	output, err := runOsascript(ctx, script)
	if err != nil {
		return &tools.ToolResult{
			Tool:     ToolQuitApp,
			Success:  false,
			Error:    fmt.Sprintf("failed to quit %s: %v", appName, err),
			Duration: time.Since(start),
		}, err
	}

	action := "Quit"
	if force {
		action = "Force quit"
	}

	return &tools.ToolResult{
		Tool:     ToolQuitApp,
		Success:  true,
		Output:   fmt.Sprintf("%s application: %s", action, appName),
		Duration: time.Since(start),
		Metadata: map[string]interface{}{
			"app_name":   appName,
			"force":      force,
			"raw_output": output,
		},
	}, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// LIST APPS TOOL
// ═══════════════════════════════════════════════════════════════════════════════

// ListAppsTool lists running applications.
type ListAppsTool struct{}

func (t *ListAppsTool) Name() tools.ToolType { return ToolListApps }

func (t *ListAppsTool) Validate(req *tools.ToolRequest) error {
	return checkMacOS()
}

func (t *ListAppsTool) AssessRisk(req *tools.ToolRequest) tools.RiskLevel {
	return tools.RiskNone // Read-only operation
}

func (t *ListAppsTool) Execute(ctx context.Context, req *tools.ToolRequest) (*tools.ToolResult, error) {
	start := time.Now()

	// Get list of running apps via AppleScript
	script := `tell application "System Events"
    set appList to name of every process whose background only is false
    set AppleScript's text item delimiters to linefeed
    return appList as text
end tell`

	output, err := runOsascript(ctx, script)
	if err != nil {
		return &tools.ToolResult{
			Tool:     ToolListApps,
			Success:  false,
			Error:    fmt.Sprintf("failed to list apps: %v", err),
			Duration: time.Since(start),
		}, err
	}

	apps := strings.Split(output, "\n")
	var filtered []string
	for _, app := range apps {
		app = strings.TrimSpace(app)
		if app != "" {
			filtered = append(filtered, app)
		}
	}

	return &tools.ToolResult{
		Tool:     ToolListApps,
		Success:  true,
		Output:   fmt.Sprintf("Running applications (%d):\n%s", len(filtered), strings.Join(filtered, "\n")),
		Duration: time.Since(start),
		Metadata: map[string]interface{}{
			"app_count": len(filtered),
			"apps":      filtered,
		},
	}, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// GET FRONTMOST APP TOOL
// ═══════════════════════════════════════════════════════════════════════════════

// GetFrontmostTool returns the currently active application.
type GetFrontmostTool struct{}

func (t *GetFrontmostTool) Name() tools.ToolType { return ToolGetFrontmost }

func (t *GetFrontmostTool) Validate(req *tools.ToolRequest) error {
	return checkMacOS()
}

func (t *GetFrontmostTool) AssessRisk(req *tools.ToolRequest) tools.RiskLevel {
	return tools.RiskNone // Read-only operation
}

func (t *GetFrontmostTool) Execute(ctx context.Context, req *tools.ToolRequest) (*tools.ToolResult, error) {
	start := time.Now()

	script := `tell application "System Events" to get name of first process whose frontmost is true`
	output, err := runOsascript(ctx, script)
	if err != nil {
		return &tools.ToolResult{
			Tool:     ToolGetFrontmost,
			Success:  false,
			Error:    fmt.Sprintf("failed to get frontmost app: %v", err),
			Duration: time.Since(start),
		}, err
	}

	appName := strings.TrimSpace(output)

	return &tools.ToolResult{
		Tool:     ToolGetFrontmost,
		Success:  true,
		Output:   fmt.Sprintf("Frontmost application: %s", appName),
		Duration: time.Since(start),
		Metadata: map[string]interface{}{
			"app_name": appName,
		},
	}, nil
}
