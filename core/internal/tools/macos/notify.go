package macos

import (
	"context"
	"fmt"
	"time"

	"github.com/normanking/cortex/internal/tools"
)

// ═══════════════════════════════════════════════════════════════════════════════
// NOTIFY TOOL
// ═══════════════════════════════════════════════════════════════════════════════

// NotifyTool sends macOS system notifications.
type NotifyTool struct{}

func (t *NotifyTool) Name() tools.ToolType { return ToolNotify }

func (t *NotifyTool) Validate(req *tools.ToolRequest) error {
	if err := checkMacOS(); err != nil {
		return err
	}
	if req.Input == "" {
		return fmt.Errorf("notification message is required")
	}
	if len(req.Input) > 500 {
		return fmt.Errorf("notification message too long (max 500 chars)")
	}
	return nil
}

func (t *NotifyTool) AssessRisk(req *tools.ToolRequest) tools.RiskLevel {
	return tools.RiskNone // Notifications are non-invasive
}

func (t *NotifyTool) Execute(ctx context.Context, req *tools.ToolRequest) (*tools.ToolResult, error) {
	start := time.Now()
	message := req.Input

	// Get optional title and subtitle from params
	title := "Cortex"
	if val, ok := req.Params["title"].(string); ok && val != "" {
		title = val
	}

	subtitle := ""
	if val, ok := req.Params["subtitle"].(string); ok {
		subtitle = val
	}

	sound := "default"
	if val, ok := req.Params["sound"].(string); ok {
		sound = val
	}
	if val, ok := req.Params["silent"].(bool); ok && val {
		sound = ""
	}

	// Build AppleScript for notification
	script := fmt.Sprintf(`display notification "%s"`, escapeAppleScriptString(message))

	if title != "" {
		script += fmt.Sprintf(` with title "%s"`, escapeAppleScriptString(title))
	}
	if subtitle != "" {
		script += fmt.Sprintf(` subtitle "%s"`, escapeAppleScriptString(subtitle))
	}
	if sound != "" {
		script += fmt.Sprintf(` sound name "%s"`, escapeAppleScriptString(sound))
	}

	output, err := runOsascript(ctx, script)
	if err != nil {
		return &tools.ToolResult{
			Tool:     ToolNotify,
			Success:  false,
			Error:    fmt.Sprintf("failed to send notification: %v", err),
			Duration: time.Since(start),
		}, err
	}

	return &tools.ToolResult{
		Tool:     ToolNotify,
		Success:  true,
		Output:   fmt.Sprintf("Notification sent: %s", message),
		Duration: time.Since(start),
		Metadata: map[string]interface{}{
			"title":      title,
			"subtitle":   subtitle,
			"message":    message,
			"sound":      sound,
			"raw_output": output,
		},
	}, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// DIALOG TOOLS
// ═══════════════════════════════════════════════════════════════════════════════

// DialogTool shows a dialog box and returns user input.
type DialogTool struct{}

func (t *DialogTool) Name() tools.ToolType { return "dialog" }

func (t *DialogTool) Validate(req *tools.ToolRequest) error {
	if err := checkMacOS(); err != nil {
		return err
	}
	if req.Input == "" {
		return fmt.Errorf("dialog message is required")
	}
	return nil
}

func (t *DialogTool) AssessRisk(req *tools.ToolRequest) tools.RiskLevel {
	return tools.RiskLow // Dialogs require user interaction
}

func (t *DialogTool) Execute(ctx context.Context, req *tools.ToolRequest) (*tools.ToolResult, error) {
	start := time.Now()
	message := req.Input

	// Get dialog type from params
	dialogType := "alert" // alert, input, list
	if val, ok := req.Params["type"].(string); ok {
		dialogType = val
	}

	title := "Cortex"
	if val, ok := req.Params["title"].(string); ok && val != "" {
		title = val
	}

	var script string
	switch dialogType {
	case "input":
		defaultAnswer := ""
		if val, ok := req.Params["default"].(string); ok {
			defaultAnswer = val
		}
		script = fmt.Sprintf(`display dialog "%s" with title "%s" default answer "%s"
return text returned of result`,
			escapeAppleScriptString(message),
			escapeAppleScriptString(title),
			escapeAppleScriptString(defaultAnswer))

	case "confirm":
		script = fmt.Sprintf(`display dialog "%s" with title "%s" buttons {"Cancel", "OK"} default button "OK"
return button returned of result`,
			escapeAppleScriptString(message),
			escapeAppleScriptString(title))

	default: // alert
		script = fmt.Sprintf(`display alert "%s" message "%s"`,
			escapeAppleScriptString(title),
			escapeAppleScriptString(message))
	}

	output, err := runOsascript(ctx, script)
	if err != nil {
		// Check if user cancelled
		if ctx.Err() != nil {
			return &tools.ToolResult{
				Tool:     "dialog",
				Success:  false,
				Error:    "dialog cancelled or timed out",
				Duration: time.Since(start),
			}, ctx.Err()
		}
		return &tools.ToolResult{
			Tool:     "dialog",
			Success:  false,
			Error:    fmt.Sprintf("dialog failed: %v", err),
			Duration: time.Since(start),
		}, err
	}

	return &tools.ToolResult{
		Tool:     "dialog",
		Success:  true,
		Output:   output,
		Duration: time.Since(start),
		Metadata: map[string]interface{}{
			"dialog_type": dialogType,
			"title":       title,
			"message":     message,
		},
	}, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// SAY TOOL (Text-to-Speech)
// ═══════════════════════════════════════════════════════════════════════════════

// SayTool speaks text using macOS text-to-speech.
type SayTool struct{}

func (t *SayTool) Name() tools.ToolType { return "say" }

func (t *SayTool) Validate(req *tools.ToolRequest) error {
	if err := checkMacOS(); err != nil {
		return err
	}
	if req.Input == "" {
		return fmt.Errorf("text to speak is required")
	}
	if len(req.Input) > 2000 {
		return fmt.Errorf("text too long for speech (max 2000 chars)")
	}
	return nil
}

func (t *SayTool) AssessRisk(req *tools.ToolRequest) tools.RiskLevel {
	return tools.RiskNone
}

func (t *SayTool) Execute(ctx context.Context, req *tools.ToolRequest) (*tools.ToolResult, error) {
	start := time.Now()
	text := req.Input

	// Get voice from params
	voice := "" // Use system default
	if val, ok := req.Params["voice"].(string); ok {
		voice = val
	}

	// Build say command
	args := []string{}
	if voice != "" {
		args = append(args, "-v", voice)
	}
	args = append(args, text)

	script := fmt.Sprintf(`do shell script "say %s"`, escapeAppleScriptString(text))
	if voice != "" {
		script = fmt.Sprintf(`do shell script "say -v '%s' %s"`,
			escapeAppleScriptString(voice),
			escapeAppleScriptString(text))
	}

	_, err := runOsascript(ctx, script)
	if err != nil {
		return &tools.ToolResult{
			Tool:     "say",
			Success:  false,
			Error:    fmt.Sprintf("speech failed: %v", err),
			Duration: time.Since(start),
		}, err
	}

	return &tools.ToolResult{
		Tool:     "say",
		Success:  true,
		Output:   fmt.Sprintf("Spoke: %s", text),
		Duration: time.Since(start),
		Metadata: map[string]interface{}{
			"text_length": len(text),
			"voice":       voice,
		},
	}, nil
}
