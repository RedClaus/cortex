---
project: Cortex
component: UI
phase: Archive
date_created: 2025-12-18T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:51.250558
---

# Cortex Theming and Styling System

This document describes the theming and styling system for Cortex's Charmbracelet TUI framework (CR-001).

## Overview

The theming system provides a clean separation between visual styling and business logic. It consists of two main components:

1. **Theme** (`theme.go`) - Color palettes and theme definitions
2. **Styles** (`styles.go`) - Pre-computed lipgloss styles for all UI components

## Architecture

```
Theme (color palette)
    ↓
NewStyles(theme)
    ↓
Styles (lipgloss styles)
    ↓
Render functions
```

## Files

### `theme.go`

Defines the `Theme` struct and provides 4 built-in themes:

- **ThemeDefault** - VS Code dark-inspired theme (default)
- **ThemeDracula** - Popular Dracula color scheme
- **ThemeNord** - Arctic, north-bluish theme
- **ThemeGruvbox** - Retro groove with warm, earthy tones

#### Theme Structure

```go
type Theme struct {
    Name string

    // Base Colors
    Background, Foreground, Border lipgloss.Color

    // Semantic Colors
    Primary, Secondary, Success, Warning, Error, Muted lipgloss.Color

    // Layout Background Colors
    HeaderBg, FooterBg, InputBg, ModalBg lipgloss.Color

    // Markdown Rendering
    GlamourStyle string
}
```

#### Theme Functions

- `GetTheme(id string) Theme` - Retrieve a theme by ID
- `ThemeNames() []string` - List all available theme IDs

### `styles.go`

Defines the `Styles` struct with pre-computed lipgloss styles for all UI components.

#### Style Categories

**Layout Styles:**
- `Header` - Top navigation/title bar
- `ChatArea` - Main scrollable message container
- `InputArea` - User input region
- `Footer` - Bottom status/help bar

**Header Component Styles:**
- `Logo` - Cortex branding/title
- `HeaderContext` - Current context (model, mode)
- `HeaderStatus` - Connection/session status

**Message Styles:**
- `UserLabel` / `UserMessage` - User messages
- `AssistantLabel` / `AssistantMessage` - AI responses
- `SystemMessage` - System notifications

**Footer Mode Styles:**
- `FooterNormal` - Default footer
- `FooterYolo` - YOLO mode (auto-execute)
- `FooterPlan` - Plan mode (strategic planning)

**Modal Styles:**
- `ModalBorder` - Modal container with border
- `ModalTitle` - Modal header/title
- `ModalDim` - Background overlay/dimming

**Utility Styles:**
- `Spinner` - Loading/thinking indicators
- `CodeBlock` - Code formatting
- `Separator` - Horizontal dividers
- `Timestamp` - Message timestamps
- `Badge` - Status badges
- `ErrorBox` / `SuccessBox` - Status containers

#### Core Functions

**Creation:**
```go
func NewStyles(theme Theme) Styles
```

**Rendering Helpers:**
```go
func (s *Styles) RenderUserMessage(message string) string
func (s *Styles) RenderAssistantMessage(message string) string
func (s *Styles) RenderSystemMessage(message string) string
func (s *Styles) RenderError(message string) string
func (s *Styles) RenderSuccess(message string) string
func (s *Styles) RenderCode(code string) string
func (s *Styles) RenderBadge(text string) string
func (s *Styles) RenderFooter(mode string, content string) string
func (s *Styles) RenderHorizontalLine(width int) string
```

**Global Access:**
```go
func DefaultStyles() Styles
func InitDefaultStyles(theme Theme) Styles
```

## Usage Examples

### Basic Usage

```go
// Get a theme
theme := ui.GetTheme("dracula")

// Create styles from theme
styles := ui.NewStyles(theme)

// Render messages
userMsg := styles.RenderUserMessage("Deploy to production")
assistantMsg := styles.RenderAssistantMessage("Starting deployment...")
```

### Switching Themes

```go
// Initialize with default theme
styles := ui.InitDefaultStyles(ui.ThemeDefault)

// Later, switch to Nord theme
nordTheme := ui.GetTheme("nord")
styles = ui.NewStyles(nordTheme)
```

### Custom Rendering

```go
styles := ui.NewStyles(ui.ThemeGruvbox)

// Render different message types
fmt.Println(styles.RenderUserMessage("What's the status?"))
fmt.Println(styles.RenderAssistantMessage("All systems operational"))
fmt.Println(styles.RenderSuccess("Deployment completed"))
fmt.Println(styles.RenderError("Connection timeout"))
fmt.Println(styles.RenderCode("kubectl get pods"))

// Render footer modes
fmt.Println(styles.RenderFooter("normal", "Press ? for help"))
fmt.Println(styles.RenderFooter("yolo", "Auto-executing"))
fmt.Println(styles.RenderFooter("plan", "Planning mode"))
```

### Using Styles Directly

```go
styles := ui.NewStyles(ui.ThemeDefault)

// Access specific styles for custom rendering
logo := styles.Logo.Render("CORTEX")
badge := styles.Badge.Render("GPT-4")
separator := styles.RenderHorizontalLine(80)
```

## Color Palettes

### ThemeDefault (VS Code Dark)
- Background: `#1e1e1e`
- Primary: `#007acc` (Blue)
- Success: `#4ec9b0` (Teal)
- Warning: `#dcdcaa` (Yellow)
- Error: `#f48771` (Salmon)

### ThemeDracula
- Background: `#282a36`
- Primary: `#bd93f9` (Purple)
- Success: `#50fa7b` (Green)
- Warning: `#f1fa8c` (Yellow)
- Error: `#ff5555` (Red)

### ThemeNord
- Background: `#2e3440` (Polar Night)
- Primary: `#88c0d0` (Frost Cyan)
- Success: `#a3be8c` (Aurora Green)
- Warning: `#ebcb8b` (Aurora Yellow)
- Error: `#bf616a` (Aurora Red)

### ThemeGruvbox
- Background: `#282828`
- Primary: `#fe8019` (Orange)
- Success: `#b8bb26` (Bright Green)
- Warning: `#fabd2f` (Bright Yellow)
- Error: `#fb4934` (Bright Red)

## Design Principles

1. **Separation of Concerns** - Colors (Theme) are separate from styles (Styles)
2. **Pre-computation** - All styles are computed once when created, not per-render
3. **Semantic Colors** - Colors have semantic meaning (Primary, Success, Error)
4. **Consistency** - All UI components use the same theme colors
5. **Flexibility** - Easy to add new themes or customize existing ones

## Performance Considerations

- Styles are created once per theme change, not per render
- Lipgloss caching handles style reuse efficiently
- Rendering functions are lightweight wrappers
- No runtime theme lookups during rendering

## Extending the System

### Adding a New Theme

```go
var ThemeCustom = Theme{
    Name: "My Custom Theme",
    Background: lipgloss.Color("#1a1a1a"),
    Foreground: lipgloss.Color("#ffffff"),
    Border: lipgloss.Color("#444444"),
    Primary: lipgloss.Color("#00ff00"),
    Secondary: lipgloss.Color("#0000ff"),
    Success: lipgloss.Color("#00cc00"),
    Warning: lipgloss.Color("#ffaa00"),
    Error: lipgloss.Color("#ff0000"),
    Muted: lipgloss.Color("#888888"),
    HeaderBg: lipgloss.Color("#222222"),
    FooterBg: lipgloss.Color("#1a1a1a"),
    InputBg: lipgloss.Color("#1a1a1a"),
    ModalBg: lipgloss.Color("#252525"),
    GlamourStyle: "dark",
}

// Register it
availableThemes["custom"] = ThemeCustom
```

### Adding New Styles

```go
// In NewStyles function, add:
s.MyCustomStyle = lipgloss.NewStyle().
    Foreground(theme.Primary).
    Background(theme.Background).
    Bold(true)
```

## Integration with Bubble Tea

The theming system is designed to integrate seamlessly with Charmbracelet's Bubble Tea framework:

```go
type Model struct {
    styles ui.Styles
    // ... other fields
}

func NewModel() Model {
    theme := ui.GetTheme("nord")
    return Model{
        styles: ui.NewStyles(theme),
    }
}

func (m Model) View() string {
    var b strings.Builder

    // Use styles for rendering
    for _, msg := range m.messages {
        if msg.Role == "user" {
            b.WriteString(m.styles.RenderUserMessage(msg.Content))
        } else {
            b.WriteString(m.styles.RenderAssistantMessage(msg.Content))
        }
        b.WriteString("\n")
    }

    return b.String()
}
```

## Status

✅ **Complete** - The theming and styling system is fully implemented and ready to use.

### Files Created
- `internal/ui/theme.go` - Theme definitions and registry
- `internal/ui/styles.go` - Styles struct and rendering functions
- `internal/ui/example_test.go` - Test suite and examples
- `internal/ui/README_THEMING.md` - This documentation

### Compilation Status
The theme.go and styles.go files compile successfully on their own. There are unrelated compilation errors in other files in the internal/ui package (update.go, messages.go, etc.) that need to be fixed separately.

## Next Steps

1. Fix compilation errors in other `internal/ui` files
2. Integrate theming system with the main TUI application
3. Add theme switching UI (modal/menu)
4. Persist user theme preference to config
5. Add more themes (Solarized, Monokai, etc.)
6. Add light theme variants
