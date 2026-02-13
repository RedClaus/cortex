---
project: Cortex
component: UI
phase: Archive
date_created: 2025-12-18T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:51.278607
---

# Command System Integration Guide

## Overview

The `/slash` command system in `commands.go` provides a clean, extensible interface for controlling the Cortex TUI without leaving the chat interface.

## Architecture

```
User Input → handleSubmit() → Command Router → Individual Handlers → tea.Msg
                    ↓
                Check prefix:
                  "/" → HandleCommand()
                  "!" → HandleShellEscape()
                  else → SendMessage()
```

## Integration into Model.Update()

To integrate the command system into your Bubble Tea model's `Update()` method, add this logic to `handleSubmit()`:

```go
// In your Model's Update() method
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "enter":
            return m, m.handleSubmit()
        }
    }
    return m, nil
}

// handleSubmit processes user input and routes it appropriately
func (m *Model) handleSubmit() tea.Cmd {
    content := strings.TrimSpace(m.input.Value())

    if content == "" {
        return nil
    }

    // Clear input
    m.input.SetValue("")

    // Route based on prefix
    if strings.HasPrefix(content, "/") {
        return ui.HandleCommand(content, m.backend)
    }

    if strings.HasPrefix(content, "!") {
        return ui.HandleShellEscape(content)
    }

    // Regular chat message
    return ui.SendMessageCmd(m.backend, content)
}
```

## Message Handling

Your `Update()` method must handle the messages returned by command handlers:

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {

    // Modal triggers
    case ui.ShowHelpMsg:
        m.currentModal = ModalHelp
        return m, nil

    case ui.ShowModelSelectorMsg:
        m.currentModal = ModalModelSelector
        return m, ui.FetchModelsCmd(m.backend)

    case ui.ShowThemeSelectorMsg:
        m.currentModal = ModalThemeSelector
        return m, nil

    case ui.ShowSessionSelectorMsg:
        m.currentModal = ModalSessionSelector
        return m, ui.FetchSessionsCmd(m.backend)

    // Mode toggles
    case ui.ToggleYoloMsg:
        m.yoloMode = !m.yoloMode
        m.statusMessage = fmt.Sprintf("YOLO mode: %v", m.yoloMode)
        return m, nil

    case ui.TogglePlanMsg:
        m.planMode = !m.planMode
        m.statusMessage = fmt.Sprintf("Plan mode: %v", m.planMode)
        return m, nil

    // State changes
    case ui.ClearHistoryMsg:
        m.messages = []Message{}
        m.statusMessage = "Conversation cleared"
        return m, nil

    case ui.ModelSelectedMsg:
        m.currentModel = msg.Model
        m.currentModal = ModalNone
        m.statusMessage = fmt.Sprintf("Switched to model: %s", msg.Model.Name)
        return m, nil

    case ui.ThemeSelectedMsg:
        m.theme = ui.GetTheme(msg.ThemeName)
        m.currentModal = ModalNone
        m.statusMessage = fmt.Sprintf("Switched to theme: %s", msg.ThemeName)
        // Rebuild styles with new theme
        return m, m.rebuildStyles()

    case ui.SessionLoadedMsg:
        if msg.Error != nil {
            m.statusMessage = fmt.Sprintf("Error loading session: %v", msg.Error)
            return m, nil
        }
        m.currentSession = msg.Session
        m.currentModal = ModalNone
        m.statusMessage = fmt.Sprintf("Loaded session: %s", msg.Session.Name)
        return m, nil

    // Shell commands
    case ui.ShellCommandMsg:
        if msg.Error != nil {
            m.addSystemMessage(fmt.Sprintf("Shell command failed: %v\n%s", msg.Error, msg.Output))
        } else {
            m.addSystemMessage(fmt.Sprintf("$ %s\n%s", msg.Command, msg.Output))
        }
        return m, nil

    // Command errors
    case ui.CommandErrorMsg:
        m.statusMessage = msg.Error
        return m, nil
    }

    return m, nil
}
```

## Command Reference

### Basic Commands

| Command | Aliases | Arguments | Description |
|---------|---------|-----------|-------------|
| `/help` | `/h`, `/?` | None | Show help modal |
| `/clear` | `/c` | None | Clear conversation history |
| `/quit` | `/q`, `/exit` | None | Exit Cortex |

### Configuration Commands

| Command | Aliases | Arguments | Description |
|---------|---------|-----------|-------------|
| `/model` | `/m` | `[name]` | Open model selector or switch to model |
| `/theme` | `/t` | `[name]` | Open theme selector or switch to theme |
| `/session` | `/s` | `[id/name]` | Open session selector or switch session |

### Mode Toggles

| Command | Arguments | Description |
|---------|-----------|-------------|
| `/yolo` | None | Toggle auto-run mode (dangerous!) |
| `/plan` | None | Toggle planning mode |

### Shell Escape

| Syntax | Description | Example |
|--------|-------------|---------|
| `!<command>` | Run shell command | `!ls -la` |

## Examples

### Direct Model Switching
```
User: /model gpt-4
→ Searches for "gpt-4" in available models
→ Returns ModelSelectedMsg if found
→ Returns CommandErrorMsg if not found
```

### Opening Model Selector
```
User: /model
→ Returns ShowModelSelectorMsg
→ Model.Update() opens modal and fetches models
```

### Theme Switching
```
User: /theme dracula
→ Validates theme exists
→ Returns ThemeSelectedMsg with "dracula"
→ Model.Update() applies new theme
```

### Shell Commands
```
User: !git status
→ Executes "git status" using sh -c
→ Returns ShellCommandMsg with output
→ Model.Update() displays output as system message
```

### Session Management
```
User: /session list
→ Returns SessionsLoadedMsg with all sessions
→ Model.Update() displays session list

User: /session my-project
→ Searches for session with ID or name "my-project"
→ Returns SessionLoadedMsg if found
```

## Error Handling

All commands return appropriate error messages:

```go
case ui.CommandErrorMsg:
    // Display error in status bar
    m.statusMessage = msg.Error

    // Or add as system message
    m.addSystemMessage(fmt.Sprintf("Error: %s", msg.Error))
```

## Autocomplete Integration

Use `GetCommandSuggestions()` for autocomplete functionality:

```go
// When user types "/" in input field
if strings.HasPrefix(m.input.Value(), "/") {
    suggestions := ui.GetCommandSuggestions(m.input.Value())
    // Display suggestions in dropdown or inline
}
```

## Security Considerations

### Shell Escape Security

The `!` shell escape uses `sh -c`, which:
- ✅ Allows pipes, redirects, and shell features
- ⚠️ Runs with user's privileges
- ⚠️ Can execute any command the user can run

**Recommendations:**
1. Show confirmation dialog before running in non-YOLO mode
2. Display the command that will be executed
3. Warn users about YOLO mode risks
4. Consider sandboxing or limiting commands in production

### YOLO Mode Warning

Display a prominent warning when YOLO mode is enabled:

```go
if m.yoloMode {
    warningText := lipgloss.NewStyle().
        Foreground(m.theme.Error).
        Bold(true).
        Render("⚠ YOLO MODE ACTIVE - Commands run without confirmation!")

    // Add to header or footer
}
```

## Testing

Test command routing:

```go
func TestCommandRouting(t *testing.T) {
    backend := &MockBackend{}

    tests := []struct {
        input    string
        expected tea.Msg
    }{
        {"/help", ui.ShowHelpMsg{}},
        {"/clear", ui.ClearHistoryMsg{}},
        {"/yolo", ui.ToggleYoloMsg{}},
        {"/unknown", ui.CommandErrorMsg{}},
    }

    for _, tt := range tests {
        cmd := ui.HandleCommand(tt.input, backend)
        msg := cmd()

        if reflect.TypeOf(msg) != reflect.TypeOf(tt.expected) {
            t.Errorf("HandleCommand(%q) = %T, want %T",
                tt.input, msg, tt.expected)
        }
    }
}
```

## Future Extensions

To add new commands:

1. **Define message type** (if needed):
   ```go
   type NewCommandMsg struct {
       Data string
   }
   ```

2. **Add to router** in `HandleCommand()`:
   ```go
   case "newcmd", "nc":
       return cmdNewCommand(args)
   ```

3. **Implement handler**:
   ```go
   func cmdNewCommand(args []string) tea.Cmd {
       return func() tea.Msg {
           return NewCommandMsg{Data: "processed"}
       }
   }
   ```

4. **Update autocomplete** in `GetCommandSuggestions()`:
   ```go
   commands := []string{
       // ... existing commands ...
       "/newcmd", "/nc",
   }
   ```

5. **Handle in Model.Update()**:
   ```go
   case ui.NewCommandMsg:
       // Handle the command
       return m, nil
   ```

## Performance Notes

- Command routing is O(1) using switch statements
- Model/session searching is O(n) with partial matching
- Shell commands block until completion (consider timeout)
- No command creates goroutines except shell execution

## Credits

Part of CR-001: Prism UI — Charmbracelet TUI Framework for Cortex
Phase 5: Commands (P5-001 through P5-009)
