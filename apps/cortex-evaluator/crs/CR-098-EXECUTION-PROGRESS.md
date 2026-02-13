---
project: Cortex
component: Docs
phase: Design
date_created: 2026-01-17T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:18:14.478242
---

# CR-098: Execution and Progress Monitoring

**Status:** Draft
**Phase:** 5
**Priority:** P1
**Estimated Effort:** 5-7 days
**Created:** 2026-01-17
**Depends On:** CR-094, CR-097

---

## Summary

Implement the code execution system that runs Change Requests via Claude Code CLI and monitors progress in real-time. The system watches TODO completion, displays progress with percentage and progress bars, and sends notification banners when CRs complete.

---

## Requirements

### Functional Requirements

1. **CR Execution**
   - Execute CR via Claude Code CLI
   - Pass CR content as structured prompt
   - Run in target project directory
   - Capture stdout/stderr for logging

2. **TODO Watching**
   - Monitor Claude Code's TODO output in real-time
   - Parse TODO items (pending, in-progress, completed)
   - Calculate completion percentage
   - Update progress state continuously

3. **Progress Display**
   - Show progress bar with percentage
   - List completed items with checkmarks
   - Show currently in-progress item
   - List pending items
   - Show estimated time remaining

4. **Notifications**
   - Send notification banner when CR completes
   - Include summary of what was done
   - Provide quick actions (view details, start next CR)
   - Support system notifications (macOS)

5. **Error Handling**
   - Detect execution failures
   - Capture error output
   - Update CR status to failed
   - Provide recovery options

---

## Technical Design

### Data Models

```go
// internal/progress/types.go

type CRProgress struct {
    CRID            string    `json:"cr_id"`
    Status          string    `json:"status"`  // running, completed, failed
    TotalTodos      int       `json:"total_todos"`
    CompletedTodos  int       `json:"completed_todos"`
    InProgressTodos int       `json:"in_progress_todos"`
    PendingTodos    int       `json:"pending_todos"`
    PercentComplete float64   `json:"percent_complete"`

    // Detailed items
    Completed   []TodoItem `json:"completed"`
    InProgress  []TodoItem `json:"in_progress"`
    Pending     []TodoItem `json:"pending"`

    // Timing
    StartedAt   time.Time  `json:"started_at"`
    LastUpdated time.Time  `json:"last_updated"`
    EstimatedRemaining time.Duration `json:"estimated_remaining,omitempty"`
}

type TodoItem struct {
    Description string    `json:"description"`
    Status      string    `json:"status"`  // pending, in_progress, completed
    StartedAt   *time.Time `json:"started_at,omitempty"`
    CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type ProgressUpdate struct {
    CRID            string
    PercentComplete float64
    CurrentItem     string
    Completed       []string
    Pending         []string
    Timestamp       time.Time
}

type Notification struct {
    ID        string           `json:"id"`
    Type      NotificationType `json:"type"`
    Title     string           `json:"title"`
    Message   string           `json:"message"`
    CRID      string           `json:"cr_id,omitempty"`
    Actions   []NotifyAction   `json:"actions,omitempty"`
    CreatedAt time.Time        `json:"created_at"`
}

type NotificationType string

const (
    NotifySuccess NotificationType = "success"
    NotifyFailure NotificationType = "failure"
    NotifyInfo    NotificationType = "info"
)

type NotifyAction struct {
    Label   string `json:"label"`
    Command string `json:"command"`
}
```

### Package Structure

```
internal/
├── executor/
│   ├── executor.go     # Claude Code execution
│   ├── types.go        # Execution types
│   └── process.go      # Process management
├── progress/
│   ├── watcher.go      # TODO watcher service
│   ├── types.go        # Progress types
│   ├── parser.go       # TODO output parser
│   └── display.go      # Progress display
├── notify/
│   ├── service.go      # Notification service
│   ├── types.go        # Notification types
│   ├── banner.go       # TUI banner display
│   └── system.go       # System notifications (macOS)
```

### API Design

```go
// internal/executor/executor.go

type CRExecutor interface {
    // Execute a CR via Claude Code
    Execute(cr *ChangeRequest) error

    // Get execution status
    GetStatus(crID string) (*ExecutionStatus, error)

    // Cancel execution
    Cancel(crID string) error

    // Get output logs
    GetLogs(crID string) (string, error)
}

type ExecutionStatus struct {
    CRID     string
    Running  bool
    ExitCode *int
    Error    string
    Duration time.Duration
}

// internal/progress/watcher.go

type ProgressWatcher interface {
    // Start watching a CR execution
    Watch(cr *ChangeRequest) error

    // Stop watching
    Stop(crID string) error

    // Get current progress
    GetProgress(crID string) (*CRProgress, error)

    // Subscribe to updates
    Subscribe(crID string) <-chan ProgressUpdate

    // Unsubscribe
    Unsubscribe(crID string, ch <-chan ProgressUpdate)
}

// internal/notify/service.go

type NotificationService interface {
    // Send notification
    Send(notification Notification) error

    // Show banner in TUI
    ShowBanner(notification Notification) error

    // Send system notification (macOS)
    SendSystemNotification(notification Notification) error

    // Get recent notifications
    GetRecent(limit int) ([]Notification, error)

    // Dismiss notification
    Dismiss(notificationID string) error
}
```

### TODO Parser

```go
// internal/progress/parser.go

type TodoParser interface {
    // Parse TODO output from Claude Code
    Parse(output string) ([]TodoItem, error)

    // Parse incremental update
    ParseUpdate(line string) (*TodoUpdate, error)
}

// Expected Claude Code TODO format:
// ✅ Completed: Create VoiceExecutive struct
// 🔄 In Progress: Implement Groq provider routing
// ⏳ Pending: Add tests and documentation
//
// Or structured format:
// [x] Create VoiceExecutive struct
// [ ] Implement Groq provider routing
// [ ] Add tests and documentation

type TodoUpdate struct {
    Item      string
    OldStatus string
    NewStatus string
}
```

### Progress Display Format

```
┌─────────────────────────────────────────────────────────────────┐
│  CR-094: Core Session Management                                │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Progress: [████████████░░░░░░░░] 67%                           │
│                                                                  │
│  ✅ Completed (4/6):                                            │
│     • Create project structure with go.mod                      │
│     • Implement internal/data/database.go                       │
│     • Create database schema and migrations                     │
│     • Implement session types and storage                       │
│                                                                  │
│  🔄 In Progress (1/6):                                          │
│     • Implement session manager CRUD operations                 │
│                                                                  │
│  ⏳ Pending (1/6):                                              │
│     • Add CLI commands and write tests                          │
│                                                                  │
│  ⏱️  Elapsed: 8m 32s | Est. Remaining: ~4m                       │
│  Last Updated: 2 seconds ago                                    │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Notification Banner

```
┌─────────────────────────────────────────────────────────────────┐
│  🎉 CR-094 COMPLETE!                                            │
│                                                                  │
│  Core Session Management finished successfully.                 │
│  6/6 tasks completed in 12 minutes.                             │
│                                                                  │
│  [View Details]  [Start CR-095]  [Dismiss]                      │
└─────────────────────────────────────────────────────────────────┘
```

### System Notification (macOS)

```go
// internal/notify/system.go

func (s *SystemNotifier) SendMacOS(n Notification) error {
    script := fmt.Sprintf(`
        display notification "%s" with title "%s" sound name "Glass"
    `, n.Message, n.Title)

    cmd := exec.Command("osascript", "-e", script)
    return cmd.Run()
}
```

---

## Implementation Tasks

- [ ] Implement internal/executor/types.go with execution types
- [ ] Implement internal/executor/process.go for process management
- [ ] Implement internal/executor/executor.go for Claude Code execution
- [ ] Implement internal/progress/types.go with progress types
- [ ] Implement internal/progress/parser.go for TODO parsing
- [ ] Implement internal/progress/watcher.go for real-time monitoring
- [ ] Implement internal/progress/display.go for progress display
- [ ] Implement internal/notify/types.go with notification types
- [ ] Implement internal/notify/service.go for notification handling
- [ ] Implement internal/notify/banner.go for TUI banners
- [ ] Implement internal/notify/system.go for macOS notifications
- [ ] Add `/cr execute <id>` command
- [ ] Add `/cr status <id>` command
- [ ] Add real-time progress display in TUI
- [ ] Add notification preferences (sound, system, banner)
- [ ] Write unit tests for TODO parser
- [ ] Write unit tests for progress calculation
- [ ] Write integration tests for execution flow

---

## Files to Create/Modify

| File | Action | Description |
|------|--------|-------------|
| `internal/executor/types.go` | Create | Execution types |
| `internal/executor/process.go` | Create | Process management |
| `internal/executor/executor.go` | Create | Claude Code execution |
| `internal/progress/types.go` | Create | Progress types |
| `internal/progress/parser.go` | Create | TODO parsing |
| `internal/progress/watcher.go` | Create | Real-time monitoring |
| `internal/progress/display.go` | Create | Progress display |
| `internal/notify/types.go` | Create | Notification types |
| `internal/notify/service.go` | Create | Notification handling |
| `internal/notify/banner.go` | Create | TUI banners |
| `internal/notify/system.go` | Create | macOS notifications |
| `internal/cr/storage.go` | Modify | Add progress state |
| `cmd/evaluator/main.go` | Modify | Add execute/status commands |

---

## Acceptance Criteria

- [ ] User can execute CR with `/cr execute <id>`
- [ ] Claude Code runs in project directory with CR prompt
- [ ] Progress is displayed in real-time with progress bar
- [ ] Completed items show with checkmarks
- [ ] Current item shows as in-progress
- [ ] Pending items are listed
- [ ] Percentage updates as tasks complete
- [ ] Notification banner appears on completion
- [ ] macOS system notification is sent
- [ ] Failed executions are handled gracefully
- [ ] User can check status with `/cr status <id>`
- [ ] Logs are available for debugging

---

## Dependencies

- CR-094: Core Session Management
- CR-097: PRD and CR Generation
- Claude Code CLI installed and configured

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| TODO format varies | High | Multiple parsers, fallback heuristics |
| Claude Code not installed | Medium | Check on startup, helpful error |
| Long-running executions | Low | Background execution, persistent state |
| Process crashes | Medium | Capture state, enable resume |
