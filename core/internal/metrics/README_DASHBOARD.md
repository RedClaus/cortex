---
project: Cortex
component: UI
phase: Archive
date_created: 2025-12-18T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:51.237554
---

# Event Bus Metrics Dashboard - CR-013 Track 2

## Overview
Real-time metrics dashboard that subscribes to the event bus and displays analytics in the Cortex TUI.

## Components

### 1. Metrics Collector (`internal/metrics/collector.go`)
Subscribes to the event bus and aggregates real-time metrics.

**Features:**
- Thread-safe event processing
- Tracks session statistics (requests, tokens, latency, etc.)
- Maintains recent event history
- Optional integration with metrics Store for persistence
- Graceful handling when event bus is nil

**Event Subscriptions:**
- `agent.started` - Increments active agents, updates last event
- `agent.completed` - Tracks success/failure, tokens, latency, tool calls
- `tool.executed` - Counts tool executions
- `stream.chunk` - Tracks streaming completion

**API:**
```go
collector := metrics.NewCollector(eventBus, store)
collector.Start()  // Begin listening
stats := collector.GetSessionStats()  // Get current stats
events := collector.GetRecentEvents(5)  // Get recent events
collector.Stop()  // Stop listening
```

### 2. Metrics Dashboard (`internal/metrics/dashboard.go`)
Formats metrics for TUI display with lipgloss styling.

**Features:**
- Apple-inspired design with lipgloss
- Two rendering modes: full and compact
- Color-coded success rates (green >90%, yellow >70%, red <70%)
- Event activity indicators (●●●●○)
- Token count formatting (k/M suffixes)

**Full Display Format:**
```
┌─ METRICS ──────────────────────────────────────────────────────┐
│ Session: 5 requests │ Tokens: 12.4k / 8.2k │ Success: 100%    │
│ Latency: 1.2s avg   │ Local: 80%          │ Tools: 12 calls  │
│ Active: 1 agent     │ Last: bash (0.3s)   │ ●●●●○ events     │
└────────────────────────────────────────────────────────────────┘
```

**Compact Format (footer):**
```
[Metrics] 5 req │ 12.4k/8.2k tokens │ 1.2s avg │ 12 tools │ ●●●○
```

**API:**
```go
dashboard := metrics.NewDashboard(collector)
dashboard.SetWidth(80)
fullView := dashboard.Render()  // Full bordered view
compact := dashboard.RenderCompact()  // Single-line summary
```

### 3. TUI Metrics View (`internal/tui/metrics_view.go`)
Wrapper for the dashboard that manages visibility and mode in the TUI.

**Features:**
- Toggle visibility
- Toggle compact/full mode
- Width-aware rendering

**API:**
```go
view := NewMetricsView(dashboard)
view.SetVisible(true)
view.SetCompact(false)
rendered := view.Render(width)
```

### 4. TUI Integration (`internal/tui/app.go`)
Integrated into the main Cortex TUI application.

**Added Fields:**
- `metricsView *MetricsView` - Metrics view component
- `metricsCollector *metrics.Collector` - Metrics collector
- `showMetrics bool` - Visibility toggle

**Keyboard Shortcuts:**
- `m` - Toggle metrics visibility
- `M` (Shift+m) - Toggle compact/full mode

**Display Modes:**
- **Compact** - Appears in footer status line
- **Full** - Displays between header and chat panel

**Initialization:**
```go
eventBus := orch.EventBus()
if eventBus != nil {
    collector := metrics.NewCollector(eventBus, nil)
    collector.Start()
    dashboard := metrics.NewDashboard(collector)
    app.metricsCollector = collector
    app.metricsView = NewMetricsView(dashboard)
    app.showMetrics = false  // Hidden by default
}
```

### 5. Orchestrator API (`internal/orchestrator/orchestrator.go`)
Added method to expose the event bus instance.

```go
func (o *Orchestrator) EventBus() *bus.EventBus {
    return o.eventBus
}
```

## Usage

1. **Start Cortex TUI** - Metrics collector automatically starts if event bus is available
2. **Press `m`** - Toggle metrics visibility
3. **Press `M`** - Switch between compact (footer) and full (panel) modes
4. **Metrics update in real-time** as events flow through the system

## Metrics Tracked

### Session Statistics
- **RequestCount** - Total agent requests in session
- **TokensIn/TokensOut** - Total tokens processed
- **ToolCalls** - Number of tool executions
- **TotalLatencyMs** - Cumulative latency
- **SuccessCount/FailureCount** - Request outcomes
- **ActiveAgents** - Currently running agents
- **LocalRequests** - Requests to local models (Ollama)
- **LastEvent** - Most recent event description
- **LastEventTime** - Timestamp of last event

### Derived Metrics
- **Average Latency** - TotalLatencyMs / RequestCount
- **Success Rate** - (SuccessCount / RequestCount) * 100
- **Local Model Rate** - (LocalRequests / RequestCount) * 100

## Thread Safety
All metrics operations are thread-safe:
- `sync.RWMutex` protects session stats and recent events
- Safe for concurrent event handling
- No data races in high-frequency scenarios

## Performance Considerations
- **Event Buffer** - Recent events limited to 50 items
- **Non-blocking** - Async event publishing to prevent blocking
- **Minimal Overhead** - Lightweight aggregation, no heavy processing
- **Optional Store** - Can run without database persistence

## Testing
```bash
# Build and verify compilation
go build ./internal/metrics/...
go build ./internal/tui/...
go build ./internal/orchestrator/...
go build ./...

# All packages compile successfully
```

## Future Enhancements
- [ ] Configurable event buffer size
- [ ] Historical metrics (last hour, last day)
- [ ] Per-provider breakdowns
- [ ] Export metrics to Prometheus/Grafana
- [ ] Real-time charts (sparklines)
- [ ] Alert thresholds (high latency, low success rate)
