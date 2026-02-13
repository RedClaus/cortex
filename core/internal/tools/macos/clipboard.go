package macos

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/tools"
)

// ═══════════════════════════════════════════════════════════════════════════════
// CLIPBOARD GET TOOL
// ═══════════════════════════════════════════════════════════════════════════════

// ClipboardGetTool reads content from the system clipboard.
type ClipboardGetTool struct{}

func (t *ClipboardGetTool) Name() tools.ToolType { return ToolClipboardGet }

func (t *ClipboardGetTool) Validate(req *tools.ToolRequest) error {
	return checkMacOS()
}

func (t *ClipboardGetTool) AssessRisk(req *tools.ToolRequest) tools.RiskLevel {
	return tools.RiskNone // Read-only operation
}

func (t *ClipboardGetTool) Execute(ctx context.Context, req *tools.ToolRequest) (*tools.ToolResult, error) {
	start := time.Now()

	// Use pbpaste to get clipboard contents
	cmd := exec.CommandContext(ctx, "pbpaste")
	out, err := cmd.Output()
	if err != nil {
		return &tools.ToolResult{
			Tool:     ToolClipboardGet,
			Success:  false,
			Error:    fmt.Sprintf("failed to read clipboard: %v", err),
			Duration: time.Since(start),
		}, err
	}

	content := string(out)
	contentLen := len(content)

	// Truncate for display if very long
	displayContent := content
	if contentLen > 1000 {
		displayContent = content[:1000] + fmt.Sprintf("\n... [truncated, total %d chars]", contentLen)
	}

	return &tools.ToolResult{
		Tool:     ToolClipboardGet,
		Success:  true,
		Output:   displayContent,
		Duration: time.Since(start),
		Metadata: map[string]interface{}{
			"content_length": contentLen,
			"full_content":   content, // Full content in metadata
		},
	}, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// CLIPBOARD SET TOOL
// ═══════════════════════════════════════════════════════════════════════════════

// ClipboardSetTool writes content to the system clipboard.
type ClipboardSetTool struct{}

func (t *ClipboardSetTool) Name() tools.ToolType { return ToolClipboardSet }

func (t *ClipboardSetTool) Validate(req *tools.ToolRequest) error {
	if err := checkMacOS(); err != nil {
		return err
	}
	// Allow empty string to clear clipboard
	return nil
}

func (t *ClipboardSetTool) AssessRisk(req *tools.ToolRequest) tools.RiskLevel {
	return tools.RiskLow // Modifies clipboard but non-destructive
}

func (t *ClipboardSetTool) Execute(ctx context.Context, req *tools.ToolRequest) (*tools.ToolResult, error) {
	start := time.Now()
	content := req.Input

	// Use pbcopy to set clipboard contents
	cmd := exec.CommandContext(ctx, "pbcopy")
	cmd.Stdin = strings.NewReader(content)
	err := cmd.Run()
	if err != nil {
		return &tools.ToolResult{
			Tool:     ToolClipboardSet,
			Success:  false,
			Error:    fmt.Sprintf("failed to set clipboard: %v", err),
			Duration: time.Since(start),
		}, err
	}

	// Truncate for display
	displayContent := content
	if len(content) > 100 {
		displayContent = content[:100] + "..."
	}

	return &tools.ToolResult{
		Tool:     ToolClipboardSet,
		Success:  true,
		Output:   fmt.Sprintf("Copied to clipboard: %s", displayContent),
		Duration: time.Since(start),
		Metadata: map[string]interface{}{
			"content_length": len(content),
		},
	}, nil
}
