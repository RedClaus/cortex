---
project: Cortex
component: Docs
phase: Archive
date_created: 2025-12-18T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:51.304094
---

# Modal Overlay System

Complete modal overlay system for Cortex TUI using Charmbracelet Bubble Tea framework.

## Features

All modals include:
- ✅ Rounded borders
- ✅ Escape key dismissal
- ✅ Proper title styling
- ✅ Centered overlay with dimmed background
- ✅ Tea.Model interface implementation

## Components

### 1. Overlay System (`overlay.go`)

Base overlay renderer that centers modals on dimmed backgrounds.

```go
import "github.com/normanking/cortex/internal/ui/modals"

// Render a modal on top of main UI
dimColor := lipgloss.Color("#1a1b26")
screen := modals.OverlayModal(
    mainUIView,    // Background content (will be dimmed)
    modalView,     // Modal content
    termWidth,     // Terminal width
    termHeight,    // Terminal height
    dimColor,      // Dim color
)
```

### 2. Help Modal (`help.go`)

Displays keyboard shortcuts organized by category.

```go
// Define keybindings
keyMap := modals.DefaultKeyMap()

// Or create custom keymap
keyMap := modals.KeyMap{
    Navigation: []modals.KeyBinding{
        {"↑/k", "Scroll up"},
        {"↓/j", "Scroll down"},
    },
    Chat: []modals.KeyBinding{
        {"enter", "Send message"},
    },
}

// Create and render
help := modals.NewHelpModal(keyMap)
help.SetSize(80, 24)
view := help.View()

// Use with overlay
screen := modals.OverlayModal(mainUI, view, 80, 24, dimColor)
```

**Keys:**
- `esc`, `?`, `q` - Close modal

### 3. Model Selector (`model_selector.go`)

List-based modal for selecting AI models with provider tags.

```go
// Get models from backend
models := []ui.ModelInfo{
    {
        ID:       "gpt-4",
        Name:     "GPT-4",
        Provider: "openai",
        IsLocal:  false,
    },
    {
        ID:       "llama2",
        Name:     "Llama 2",
        Provider: "ollama",
        IsLocal:  true,
    },
}

// Create selector
selector := modals.NewModelSelector(models)
selector.SetSize(80, 24)

// In your Update() function
switch msg := msg.(type) {
case modals.ModelSelectedMsg:
    // User selected a model
    selectedModel := msg.Model
    log.Printf("Selected: %s (%s)", selectedModel.Name, selectedModel.Provider)
}
```

**Keys:**
- `↑/↓`, `j/k` - Navigate list
- `/` - Filter/search models
- `enter` - Select model
- `esc` - Cancel

**Display Format:**
```
GPT-4
openai • Cloud

Llama 2
ollama • Local
```

### 4. Theme Selector (`theme_selector.go`)

List-based modal for selecting color themes with previews.

```go
// Create selector with current theme
selector := modals.NewThemeSelector("tokyo_night")
selector.SetSize(80, 24)

// In your Update() function
switch msg := msg.(type) {
case modals.ThemeSelectedMsg:
    // User selected a theme
    themeID := msg.ThemeID
    palette := msg.Theme
    log.Printf("Selected theme: %s", palette.Name)
}
```

**Keys:**
- `↑/↓`, `j/k` - Navigate list
- `/` - Filter/search themes
- `enter` - Select theme
- `esc` - Cancel

**Display Format:**
```
Tokyo Night
🌙 dark • ████████████

Gruvbox Light
☀️ light • ████████████
```

Color boxes show: Primary, Secondary, Success, Warning

### 5. Confirm Modal (`confirm.go`)

Yes/No confirmation dialog for dangerous operations.

```go
// Create confirmation modal
modal := modals.NewConfirmModal(
    "Delete Conversation?",
    "This will permanently delete all messages in this conversation. This action cannot be undone.",
    conversationID,  // Optional payload to identify what's being confirmed
)
modal.SetSize(80, 24)

// In your Update() function
switch msg := msg.(type) {
case modals.ConfirmMsg:
    if msg.Result == modals.ConfirmResultYes {
        // User confirmed - proceed with dangerous operation
        conversationID := msg.Payload.(string)
        deleteConversation(conversationID)
    }
    // User chose No or cancelled - do nothing
}
```

**Keys:**
- `tab`, `shift+tab` - Switch between Yes/No
- `left/h`, `right/l` - Switch between Yes/No
- `enter`, `space` - Confirm selection
- `y` - Quick Yes
- `n` - Quick No
- `esc` - Cancel (same as No)

**Display:**
```
╭───────────────────────────────────────────────╮
│          Delete Conversation?                 │
│                                                │
│  This will permanently delete all messages    │
│  in this conversation. This action cannot     │
│  be undone.                                   │
│                                                │
│          ╭─────╮      ╭─────╮                │
│          │ Yes │      │ No  │                │
│          ╰─────╯      ╰─────╯                │
│                                                │
│ Tab to switch • Enter to confirm • Esc cancel │
╰───────────────────────────────────────────────╯
```

## Integration Example

Complete example showing modal integration in a Bubble Tea app:

```go
package main

import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/normanking/cortex/internal/ui/modals"
)

type ActiveModal int

const (
    ModalNone ActiveModal = iota
    ModalHelp
    ModalModelSelect
    ModalThemeSelect
    ModalConfirm
)

type Model struct {
    activeModal ActiveModal
    helpModal   *modals.HelpModal
    modelModal  *modals.ModelSelector
    themeModal  *modals.ThemeSelector
    confirmModal *modals.ConfirmModal
    width       int
    height      int
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // Handle modal-specific keys first
        if m.activeModal != ModalNone {
            switch msg.String() {
            case "esc":
                // Close any active modal
                m.activeModal = ModalNone
                return m, nil
            }
        }

        // Global keybindings
        if m.activeModal == ModalNone {
            switch msg.String() {
            case "?":
                m.activeModal = ModalHelp
                return m, nil
            case "m":
                m.activeModal = ModalModelSelect
                return m, nil
            case "t":
                m.activeModal = ModalThemeSelect
                return m, nil
            }
        }

    case modals.ModelSelectedMsg:
        // Handle model selection
        m.activeModal = ModalNone
        // Apply selected model...

    case modals.ThemeSelectedMsg:
        // Handle theme selection
        m.activeModal = ModalNone
        // Apply selected theme...

    case modals.ConfirmMsg:
        // Handle confirmation
        m.activeModal = ModalNone
        if msg.Result == modals.ConfirmResultYes {
            // Proceed with dangerous operation...
        }
    }

    // Update active modal
    switch m.activeModal {
    case ModalHelp:
        var cmd tea.Cmd
        m.helpModal, cmd = m.helpModal.Update(msg)
        return m, cmd
    case ModalModelSelect:
        var cmd tea.Cmd
        m.modelModal, cmd = m.modelModal.Update(msg)
        return m, cmd
    case ModalThemeSelect:
        var cmd tea.Cmd
        m.themeModal, cmd = m.themeModal.Update(msg)
        return m, cmd
    case ModalConfirm:
        var cmd tea.Cmd
        m.confirmModal, cmd = m.confirmModal.Update(msg)
        return m, cmd
    }

    // Update main UI...
    return m, nil
}

func (m Model) View() string {
    // Render main UI
    mainView := "Main application content here..."

    // Overlay modal if active
    if m.activeModal != ModalNone {
        var modalView string
        switch m.activeModal {
        case ModalHelp:
            modalView = m.helpModal.View()
        case ModalModelSelect:
            modalView = m.modelModal.View()
        case ModalThemeSelect:
            modalView = m.themeModal.View()
        case ModalConfirm:
            modalView = m.confirmModal.View()
        }

        dimColor := lipgloss.Color("#1a1b26")
        return modals.OverlayModal(mainView, modalView, m.width, m.height, dimColor)
    }

    return mainView
}
```

## Styling

All modals use Tokyo Night theme colors by default but can be customized:

```go
// Customize help modal
help := modals.NewHelpModal(keyMap)
customStyle := lipgloss.NewStyle().
    Foreground(lipgloss.Color("#your-color")).
    Background(lipgloss.Color("#your-bg"))
help.SetStyles(customStyle)
```

## Message Types

### ModelSelectedMsg
```go
type ModelSelectedMsg struct {
    Model ui.ModelInfo
}
```

### ThemeSelectedMsg
```go
type ThemeSelectedMsg struct {
    ThemeID string
    Theme   theme.Palette
}
```

### ConfirmMsg
```go
type ConfirmMsg struct {
    Result  ConfirmResult  // ConfirmResultYes or ConfirmResultNo
    Payload interface{}    // Optional data passed to NewConfirmModal
}
```

## Best Practices

1. **Always handle Escape** - All modals should be dismissible with `esc`
2. **Use OverlayModal** - Always render modals using the overlay system for consistency
3. **Size modals appropriately** - Call `SetSize()` after creating modals
4. **Handle messages** - Add message handlers in your `Update()` function
5. **Default to No** - Confirmation modals default to "No" for safety
6. **Provide context** - Use the payload parameter in ConfirmModal to track what's being confirmed

## Dependencies

- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/bubbles` - List component
- `github.com/charmbracelet/lipgloss` - Styling
- `github.com/normanking/cortex/pkg/theme` - Theme system
- `github.com/normanking/cortex/internal/ui` - ModelInfo type
