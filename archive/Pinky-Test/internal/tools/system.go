package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// SystemTool handles system-level operations like notifications, clipboard, and opening files
type SystemTool struct {
	timeout          time.Duration
	maxClipboardSize int
}

// SystemConfig configures the system tool
type SystemConfig struct {
	Timeout          time.Duration
	MaxClipboardSize int
}

// DefaultSystemConfig returns sensible defaults
func DefaultSystemConfig() *SystemConfig {
	return &SystemConfig{
		Timeout:          10 * time.Second,
		MaxClipboardSize: 100 * 1024, // 100KB
	}
}

// NewSystemTool creates a new system tool
func NewSystemTool(cfg *SystemConfig) *SystemTool {
	if cfg == nil {
		cfg = DefaultSystemConfig()
	}

	return &SystemTool{
		timeout:          cfg.Timeout,
		maxClipboardSize: cfg.MaxClipboardSize,
	}
}

func (t *SystemTool) Name() string           { return "system" }
func (t *SystemTool) Category() ToolCategory { return CategorySystem }
func (t *SystemTool) RiskLevel() RiskLevel   { return RiskMedium }

func (t *SystemTool) Description() string {
	return "System operations: notifications, clipboard, open files/URLs/apps"
}

// Spec returns the tool specification for LLM function calling
func (t *SystemTool) Spec() *ToolSpec {
	return &ToolSpec{
		Name:        t.Name(),
		Description: t.Description(),
		Category:    t.Category(),
		RiskLevel:   t.RiskLevel(),
		Parameters: &ParamSchema{
			Type: "object",
			Properties: map[string]*ParamProp{
				"operation": {
					Type:        "string",
					Description: "The operation to perform",
					Enum:        []string{"notify", "clipboard_read", "clipboard_write", "open"},
				},
				"title": {
					Type:        "string",
					Description: "Notification title (for notify operation)",
				},
				"message": {
					Type:        "string",
					Description: "Notification message (for notify operation)",
				},
				"content": {
					Type:        "string",
					Description: "Content to write to clipboard (for clipboard_write operation)",
				},
				"target": {
					Type:        "string",
					Description: "URL, file path, or app to open (for open operation)",
				},
			},
			Required: []string{"operation"},
		},
	}
}

// Validate checks if the input is valid
func (t *SystemTool) Validate(input *ToolInput) error {
	if input == nil {
		return errors.New("input is nil")
	}

	op, ok := input.Args["operation"].(string)
	if !ok || op == "" {
		// Also accept the operation as Command
		if input.Command != "" {
			op = input.Command
		} else {
			return errors.New("operation is required")
		}
	}

	switch op {
	case "notify":
		msg, _ := input.Args["message"].(string)
		if msg == "" {
			return errors.New("message is required for notify operation")
		}

	case "clipboard_read":
		// No additional validation needed

	case "clipboard_write":
		content, _ := input.Args["content"].(string)
		if content == "" {
			return errors.New("content is required for clipboard_write operation")
		}
		if len(content) > t.maxClipboardSize {
			return fmt.Errorf("content too large: %d bytes (max %d)", len(content), t.maxClipboardSize)
		}

	case "open":
		target, _ := input.Args["target"].(string)
		if target == "" {
			return errors.New("target is required for open operation")
		}

	default:
		return fmt.Errorf("unknown operation: %s (valid: notify, clipboard_read, clipboard_write, open)", op)
	}

	return nil
}

// Execute performs the system operation
func (t *SystemTool) Execute(ctx context.Context, input *ToolInput) (*ToolOutput, error) {
	start := time.Now()

	// Get operation from Args or Command
	op, _ := input.Args["operation"].(string)
	if op == "" {
		op = input.Command
	}

	var result string
	var err error

	switch op {
	case "notify":
		title, _ := input.Args["title"].(string)
		message := input.Args["message"].(string)
		result, err = t.notify(ctx, title, message)

	case "clipboard_read":
		result, err = t.clipboardRead(ctx)

	case "clipboard_write":
		content := input.Args["content"].(string)
		result, err = t.clipboardWrite(ctx, content)

	case "open":
		target := input.Args["target"].(string)
		result, err = t.open(ctx, target)

	default:
		err = fmt.Errorf("unknown operation: %s", op)
	}

	duration := time.Since(start)

	if err != nil {
		return &ToolOutput{
			Success:  false,
			Error:    err.Error(),
			Duration: duration,
		}, nil
	}

	return &ToolOutput{
		Success:  true,
		Output:   result,
		Duration: duration,
	}, nil
}

// notify sends a system notification
func (t *SystemTool) notify(ctx context.Context, title, message string) (string, error) {
	execCtx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		// Use osascript for macOS notifications
		script := fmt.Sprintf(`display notification "%s"`, escapeAppleScript(message))
		if title != "" {
			script += fmt.Sprintf(` with title "%s"`, escapeAppleScript(title))
		}
		cmd = exec.CommandContext(execCtx, "osascript", "-e", script)

	case "linux":
		// Use notify-send for Linux
		args := []string{}
		if title != "" {
			args = append(args, title)
		}
		args = append(args, message)
		cmd = exec.CommandContext(execCtx, "notify-send", args...)

	case "windows":
		// Use PowerShell for Windows toast notifications
		script := fmt.Sprintf(`
			[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
			$Template = [Windows.UI.Notifications.ToastTemplateType]::ToastText02
			$ToastXML = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent($Template)
			$ToastXML.GetElementsByTagName("text").Item(0).InnerText = "%s"
			$ToastXML.GetElementsByTagName("text").Item(1).InnerText = "%s"
			$Toast = [Windows.UI.Notifications.ToastNotification]::new($ToastXML)
			[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier("Pinky").Show($Toast)
		`, escapePowerShell(title), escapePowerShell(message))
		cmd = exec.CommandContext(execCtx, "powershell", "-Command", script)

	default:
		return "", fmt.Errorf("notifications not supported on %s", runtime.GOOS)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return "", fmt.Errorf("notification failed: %v (%s)", err, errMsg)
		}
		return "", fmt.Errorf("notification failed: %v", err)
	}

	return "notification sent", nil
}

// clipboardRead reads from the system clipboard
func (t *SystemTool) clipboardRead(ctx context.Context) (string, error) {
	execCtx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(execCtx, "pbpaste")

	case "linux":
		// Try xclip first, then xsel
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.CommandContext(execCtx, "xclip", "-selection", "clipboard", "-o")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.CommandContext(execCtx, "xsel", "--clipboard", "--output")
		} else {
			return "", errors.New("clipboard tools not found (install xclip or xsel)")
		}

	case "windows":
		cmd = exec.CommandContext(execCtx, "powershell", "-Command", "Get-Clipboard")

	default:
		return "", fmt.Errorf("clipboard not supported on %s", runtime.GOOS)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return "", fmt.Errorf("clipboard read failed: %v (%s)", err, errMsg)
		}
		return "", fmt.Errorf("clipboard read failed: %v", err)
	}

	content := stdout.String()
	if len(content) > t.maxClipboardSize {
		return "", fmt.Errorf("clipboard content too large: %d bytes (max %d)", len(content), t.maxClipboardSize)
	}

	return content, nil
}

// clipboardWrite writes to the system clipboard
func (t *SystemTool) clipboardWrite(ctx context.Context, content string) (string, error) {
	execCtx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(execCtx, "pbcopy")

	case "linux":
		// Try xclip first, then xsel
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.CommandContext(execCtx, "xclip", "-selection", "clipboard", "-i")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.CommandContext(execCtx, "xsel", "--clipboard", "--input")
		} else {
			return "", errors.New("clipboard tools not found (install xclip or xsel)")
		}

	case "windows":
		cmd = exec.CommandContext(execCtx, "clip")

	default:
		return "", fmt.Errorf("clipboard not supported on %s", runtime.GOOS)
	}

	cmd.Stdin = strings.NewReader(content)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return "", fmt.Errorf("clipboard write failed: %v (%s)", err, errMsg)
		}
		return "", fmt.Errorf("clipboard write failed: %v", err)
	}

	return fmt.Sprintf("copied %d bytes to clipboard", len(content)), nil
}

// open opens a file, URL, or application
func (t *SystemTool) open(ctx context.Context, target string) (string, error) {
	// Validate target to prevent command injection
	// Reject targets that look like command-line flags
	if strings.HasPrefix(target, "-") {
		return "", fmt.Errorf("invalid target: cannot start with dash")
	}

	// Reject empty targets
	if strings.TrimSpace(target) == "" {
		return "", fmt.Errorf("invalid target: empty string")
	}

	execCtx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		// Use "--" to prevent target from being interpreted as flags
		cmd = exec.CommandContext(execCtx, "open", "--", target)

	case "linux":
		// xdg-open doesn't support "--" but we've validated target doesn't start with "-"
		cmd = exec.CommandContext(execCtx, "xdg-open", target)

	case "windows":
		// On Windows, "start" command has its own escaping issues
		// Use cmd.exe with proper escaping
		cmd = exec.CommandContext(execCtx, "cmd", "/c", "start", "", target)

	default:
		return "", fmt.Errorf("open not supported on %s", runtime.GOOS)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return "", fmt.Errorf("open failed: %v (%s)", err, errMsg)
		}
		return "", fmt.Errorf("open failed: %v", err)
	}

	return fmt.Sprintf("opened: %s", target), nil
}

// escapeAppleScript escapes a string for use in AppleScript.
// AppleScript uses backslash escapes within double-quoted strings.
// We escape: backslash, double quote, and control characters.
func escapeAppleScript(s string) string {
	var b strings.Builder
	b.Grow(len(s) * 2)
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString("\\\\")
		case '"':
			b.WriteString("\\\"")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		default:
			// Reject control characters entirely for safety
			if r < 32 || r == 127 {
				continue
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}

// escapePowerShell escapes a string for use in PowerShell.
// Escapes: backtick (escape char), double quote, dollar sign, newlines, and other special chars.
func escapePowerShell(s string) string {
	var b strings.Builder
	b.Grow(len(s) * 2)
	for _, r := range s {
		switch r {
		case '`':
			b.WriteString("``")
		case '"':
			b.WriteString("`\"")
		case '$':
			b.WriteString("`$")
		case '\n':
			b.WriteString("`n")
		case '\r':
			b.WriteString("`r")
		case '\t':
			b.WriteString("`t")
		case '\x00':
			b.WriteString("`0")
		case '\'':
			// Single quotes in PowerShell don't interpolate but can break out
			b.WriteString("''")
		default:
			// Reject other control characters for safety
			if r < 32 && r != '\n' && r != '\r' && r != '\t' {
				continue
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}
