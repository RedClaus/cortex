---
project: Cortex
component: UI
phase: Archive
date_created: 2025-12-18T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:51.266025
---

# Cortex Theming System - Quick Start

## Installation

The theming system is part of the `internal/ui` package:

```go
import "github.com/normanking/cortex/internal/ui"
```

## 30-Second Tutorial

```go
// 1. Pick a theme
theme := ui.GetTheme("nord")

// 2. Create styles
styles := ui.NewStyles(theme)

// 3. Render!
fmt.Println(styles.RenderUserMessage("Hello!"))
fmt.Println(styles.RenderAssistantMessage("Hi there!"))
fmt.Println(styles.RenderSuccess("It works!"))
```

## Available Themes

| ID | Name | Style |
|----|------|-------|
| `default` | Default (VS Code Dark) | Professional blue tones |
| `dracula` | Dracula | Vibrant purple and pink |
| `nord` | Nord | Arctic, north-bluish |
| `gruvbox` | Gruvbox Dark | Warm, earthy retro |

## Common Rendering Functions

```go
styles := ui.NewStyles(ui.ThemeDefault)

// Messages
styles.RenderUserMessage("text")
styles.RenderAssistantMessage("text")
styles.RenderSystemMessage("text")

// Status
styles.RenderSuccess("text")
styles.RenderError("text")

// UI Elements
styles.RenderCode("code")
styles.RenderBadge("text")
styles.RenderFooter("mode", "text")  // mode: normal, yolo, plan
styles.RenderHorizontalLine(80)
```

## Using with Bubble Tea

```go
type Model struct {
    styles ui.Styles
}

func NewModel() Model {
    return Model{
        styles: ui.NewStyles(ui.GetTheme("nord")),
    }
}

func (m Model) View() string {
    return m.styles.RenderUserMessage("Hello from Bubble Tea!")
}
```

## Switching Themes

```go
// Initialize
styles := ui.InitDefaultStyles(ui.ThemeDefault)

// Switch
newTheme := ui.GetTheme("dracula")
styles = ui.NewStyles(newTheme)
```

## Direct Style Access

```go
styles := ui.NewStyles(ui.ThemeNord)

// Use individual styles
logo := styles.Logo.Render("CORTEX")
error := styles.ErrorBox.Render("Oops!")
spinner := styles.Spinner.Render("⠋")
```

## Custom Styling

```go
// Access theme colors
theme := ui.GetTheme("gruvbox")

customStyle := lipgloss.NewStyle().
    Foreground(theme.Primary).
    Background(theme.Background).
    Bold(true).
    Padding(1, 2)

result := customStyle.Render("Custom!")
```

## Footer Modes

```go
styles.RenderFooter("normal", "Press ? for help")
// Output: Press ? for help

styles.RenderFooter("yolo", "Auto-executing commands")
// Output: ⚡ YOLO MODE: Auto-executing commands

styles.RenderFooter("plan", "Strategic planning mode")
// Output: 📋 PLAN MODE: Strategic planning mode
```

## Full Example

```go
package main

import (
    "fmt"
    "github.com/normanking/cortex/internal/ui"
)

func main() {
    // Setup
    theme := ui.GetTheme("nord")
    styles := ui.NewStyles(theme)

    // Build UI
    fmt.Println(styles.Logo.Render("CORTEX"))
    fmt.Println(styles.RenderHorizontalLine(50))
    fmt.Println()

    // Messages
    fmt.Println(styles.RenderUserMessage("Deploy to production?"))
    fmt.Println(styles.RenderAssistantMessage("Checking deployment config..."))
    fmt.Println(styles.RenderSuccess("Ready to deploy!"))
    fmt.Println()

    // Footer
    fmt.Println(styles.RenderFooter("normal", "↑↓ Navigate • Enter Send • Ctrl+C Quit"))
}
```

## Documentation

- **Full Documentation:** [README_THEMING.md](./README_THEMING.md)
- **Implementation Details:** [../../docs/CR-001_THEMING_COMPLETE.md](../../docs/CR-001_THEMING_COMPLETE.md)
- **Tests:** [example_test.go](./example_test.go)

## Support

For questions or issues, see the documentation or check the test files for examples.
