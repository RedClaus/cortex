package metrics

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Dashboard provides formatted metrics for TUI display.
type Dashboard struct {
	collector *Collector
	styles    DashboardStyles
	width     int
}

// DashboardStyles defines the styling for the dashboard.
type DashboardStyles struct {
	Border    lipgloss.Style
	Header    lipgloss.Style
	Label     lipgloss.Style
	Value     lipgloss.Style
	Success   lipgloss.Style
	Error     lipgloss.Style
	Highlight lipgloss.Style
}

// NewDashboard creates a dashboard renderer.
func NewDashboard(collector *Collector) *Dashboard {
	return &Dashboard{
		collector: collector,
		width:     80,
		styles:    defaultDashboardStyles(),
	}
}

// defaultDashboardStyles returns the default dashboard styling.
func defaultDashboardStyles() DashboardStyles {
	return DashboardStyles{
		Border: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1),
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86")),
		Label: lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")),
		Value: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255")),
		Success: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("82")),
		Error: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("196")),
		Highlight: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("214")),
	}
}

// SetWidth sets the dashboard width.
func (d *Dashboard) SetWidth(w int) {
	d.width = w
}

// Render returns the formatted metrics string for TUI.
func (d *Dashboard) Render() string {
	stats := d.collector.GetSessionStats()

	// Calculate derived metrics
	avgLatency := float64(0)
	if stats.RequestCount > 0 {
		avgLatency = float64(stats.TotalLatencyMs) / float64(stats.RequestCount) / 1000.0
	}

	successRate := float64(100)
	if stats.RequestCount > 0 {
		successRate = float64(stats.SuccessCount) / float64(stats.RequestCount) * 100
	}

	localRate := float64(0)
	if stats.RequestCount > 0 {
		localRate = float64(stats.LocalRequests) / float64(stats.RequestCount) * 100
	}

	// Format token counts
	tokensIn := formatTokenCount(stats.TokensIn)
	tokensOut := formatTokenCount(stats.TokensOut)

	// Build dashboard content
	var content strings.Builder

	// Header
	header := d.styles.Header.Render("METRICS")
	content.WriteString(header)
	content.WriteString("\n")

	// Row 1: Session, Tokens, Success Rate
	row1 := fmt.Sprintf("%s %s │ %s %s / %s │ %s %s",
		d.styles.Label.Render("Session:"),
		d.styles.Value.Render(fmt.Sprintf("%d requests", stats.RequestCount)),
		d.styles.Label.Render("Tokens:"),
		d.styles.Highlight.Render(tokensIn),
		d.styles.Highlight.Render(tokensOut),
		d.styles.Label.Render("Success:"),
		d.formatSuccessRate(successRate),
	)
	content.WriteString(row1)
	content.WriteString("\n")

	// Row 2: Latency, Local Rate, Tools
	row2 := fmt.Sprintf("%s %s │ %s %s │ %s %s",
		d.styles.Label.Render("Latency:"),
		d.styles.Value.Render(fmt.Sprintf("%.2fs avg", avgLatency)),
		d.styles.Label.Render("Local:"),
		d.styles.Highlight.Render(fmt.Sprintf("%.0f%%", localRate)),
		d.styles.Label.Render("Tools:"),
		d.styles.Value.Render(fmt.Sprintf("%d calls", stats.ToolCalls)),
	)
	content.WriteString(row2)
	content.WriteString("\n")

	// Row 3: Active agents, Last event, Event activity
	lastEvent := stats.LastEvent
	if lastEvent == "" {
		lastEvent = "none"
	}
	if len(lastEvent) > 20 {
		lastEvent = lastEvent[:17] + "..."
	}

	timeSinceLast := ""
	if !stats.LastEventTime.IsZero() {
		elapsed := time.Since(stats.LastEventTime)
		if elapsed < time.Second {
			timeSinceLast = "now"
		} else if elapsed < time.Minute {
			timeSinceLast = fmt.Sprintf("%.0fs", elapsed.Seconds())
		} else {
			timeSinceLast = fmt.Sprintf("%.0fm", elapsed.Minutes())
		}
	}

	row3 := fmt.Sprintf("%s %s │ %s %s │ %s",
		d.styles.Label.Render("Active:"),
		d.styles.Value.Render(fmt.Sprintf("%d agent", stats.ActiveAgents)),
		d.styles.Label.Render("Last:"),
		d.styles.Value.Render(fmt.Sprintf("%s (%s)", lastEvent, timeSinceLast)),
		d.renderEventActivity(),
	)
	content.WriteString(row3)

	// Apply border
	return d.styles.Border.Width(d.width - 4).Render(content.String())
}

// RenderCompact returns a single-line summary.
func (d *Dashboard) RenderCompact() string {
	stats := d.collector.GetSessionStats()

	tokensIn := formatTokenCount(stats.TokensIn)
	tokensOut := formatTokenCount(stats.TokensOut)

	avgLatency := float64(0)
	if stats.RequestCount > 0 {
		avgLatency = float64(stats.TotalLatencyMs) / float64(stats.RequestCount) / 1000.0
	}

	return fmt.Sprintf("[Metrics] %d req │ %s/%s tokens │ %.2fs avg │ %d tools │ %s",
		stats.RequestCount,
		tokensIn,
		tokensOut,
		avgLatency,
		stats.ToolCalls,
		d.renderEventActivity(),
	)
}

// formatSuccessRate formats the success rate with color.
func (d *Dashboard) formatSuccessRate(rate float64) string {
	formatted := fmt.Sprintf("%.0f%%", rate)
	if rate >= 90 {
		return d.styles.Success.Render(formatted)
	} else if rate >= 70 {
		return d.styles.Highlight.Render(formatted)
	}
	return d.styles.Error.Render(formatted)
}

// renderEventActivity renders a visual indicator of recent event activity.
func (d *Dashboard) renderEventActivity() string {
	events := d.collector.GetRecentEvents(5)

	activity := make([]string, 5)
	for i := 0; i < 5; i++ {
		if i < len(events) {
			activity[i] = "●"
		} else {
			activity[i] = "○"
		}
	}

	return strings.Join(activity, "")
}

// formatTokenCount formats large token counts with k/M suffixes.
func formatTokenCount(count int64) string {
	if count < 1000 {
		return fmt.Sprintf("%d", count)
	} else if count < 1000000 {
		return fmt.Sprintf("%.1fk", float64(count)/1000.0)
	}
	return fmt.Sprintf("%.1fM", float64(count)/1000000.0)
}
