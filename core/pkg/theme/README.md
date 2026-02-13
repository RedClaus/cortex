---
project: Cortex
component: Docs
phase: Archive
date_created: 2025-12-18T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:17:08.870116
---

# theme - Color Palette System

The `theme` package provides a structured color palette system for the Cortex TUI, enabling consistent styling across components.

## Overview

The package defines semantic color palettes that work for both light and dark themes, allowing UI code to be separated from visual styling decisions.

## Palette Structure

```go
type Palette struct {
    // Metadata
    Name string // Human-readable name (e.g., "Dracula")
    Type string // "light" or "dark"

    // Base Colors
    Background string // Main background
    Foreground string // Primary text
    Selection  string // Highlighted item background
    Border     string // Borders and dividers

    // Semantic Colors
    Primary   string // Main action, focus indicators
    Secondary string // Subtitles, info text
    Success   string // Success states, confirmations
    Warning   string // Warnings, caution indicators
    Error     string // Error states, destructive actions

    // Extended UI Colors (optional)
    Muted   string // Dim/muted text
    Accent  string // Accent highlights
    Surface string // Elevated surfaces (cards, modals)
}
```

## Built-in Themes

### Dark Themes

| ID | Name | Background | Primary | Description |
|----|------|------------|---------|-------------|
| `dracula` | Dracula | #282a36 | Pink | Purple/pink on dark |
| `nord` | Nord | #2e3440 | Cyan | Cool arctic blues |
| `monokai` | Monokai | #272822 | Pink | Classic Sublime |
| `tokyo_night` | Tokyo Night | #1a1b26 | Blue | Purple accents |
| `gruvbox_dark` | Gruvbox Dark | #1d2021 | Yellow | Earthy retro |

### Light Themes

| ID | Name | Background | Primary | Description |
|----|------|------------|---------|-------------|
| `solarized` | Solarized Light | #fdf6e3 | Orange | Warm scientific |
| `github` | GitHub Light | #ffffff | Blue | Clean minimal |
| `gruvbox` | Gruvbox Light | #fbf1c7 | Orange | Warm earthy |

## Usage

### Get a Theme

```go
import "github.com/normanking/cortex/pkg/theme"

// Get by ID
palette := theme.Get("dracula")

// Get default
palette := theme.Get(theme.DefaultTheme) // "dracula"
```

### List Themes

```go
// All theme IDs
ids := theme.List() // ["dracula", "nord", "monokai", ...]

// Dark themes only
dark := theme.ListDark() // ["dracula", "nord", "monokai", ...]

// Light themes only
light := theme.ListLight() // ["solarized", "github", "gruvbox"]

// Check theme type
if palette.IsDark() {
    // Apply dark mode adjustments
}
```

### Cycle Themes

```go
current := "dracula"
next := theme.Next(current) // "nord"
```

### Access Colors

```go
palette := theme.Get("nord")

// Base colors
bg := palette.Background     // "#2e3440"
fg := palette.Foreground     // "#eceff4"

// Semantic colors
err := palette.Error         // "#bf616a"
success := palette.Success   // "#a3be8c"

// Extended colors (with fallbacks)
muted := palette.GetMuted()    // Uses Muted or derives from Foreground
accent := palette.GetAccent()  // Uses Accent or derives from Primary
surface := palette.GetSurface() // Uses Surface or derives from Background
```

## Integration with TUI

The theme integrates with the TUI's Styles system:

```go
import (
    "github.com/normanking/cortex/internal/tui"
    "github.com/normanking/cortex/pkg/theme"
)

// Create styles from theme
palette := theme.Get("dracula")
styles := tui.NewStyles(palette)

// Apply to components
header := styles.Header.Render("Cortex")
error := styles.Error.Render("Failed!")
```

## Fallback Methods

Extended colors have fallback methods for themes that don't define them:

```go
// GetMuted returns Muted color, or dims Foreground
func (p Palette) GetMuted() string

// GetAccent returns Accent color, or adjusts Primary
func (p Palette) GetAccent() string

// GetSurface returns Surface color, or lightens Background
func (p Palette) GetSurface() string
```

## Adding Custom Themes

Add themes to the Registry in `theme.go`:

```go
var Registry = map[string]Palette{
    "my_theme": {
        Name:       "My Theme",
        Type:       "dark",
        Background: "#1e1e1e",
        Foreground: "#d4d4d4",
        Selection:  "#264f78",
        Border:     "#454545",
        Primary:    "#569cd6",
        Secondary:  "#9cdcfe",
        Success:    "#6a9955",
        Warning:    "#ce9178",
        Error:      "#f44747",
        Muted:      "#808080",
        Accent:     "#c586c0",
        Surface:    "#252526",
    },
}
```

## Constants

```go
const DefaultTheme = "dracula"
```

## Theme Guidelines

When creating themes:

1. **Contrast**: Ensure text is readable on background (WCAG AA minimum)
2. **Semantic consistency**: Error should feel "dangerous", Success should feel "positive"
3. **Hierarchy**: Primary > Secondary > Muted for visual importance
4. **Border visibility**: Border should be visible but not distracting
5. **Selection visibility**: Selection must be clearly distinguishable
