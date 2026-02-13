---
project: Cortex
component: Agents
phase: Ideation
date_created: 2026-02-04T16:57:22
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:21:13.356800
---

# Usage Guide for Cortex Coder Agent

This guide provides comprehensive usage instructions for the Cortex Coder Agent TUI.

## Table of Contents

- [Getting Started](#getting-started)
- [Navigation](#navigation)
- [File Browser](#file-browser)
- [Editor](#editor)
- [Chat Interface](#chat-interface)
- [Diff Viewer](#diff-viewer)
- [Change Management](#change-management)
- [Skills](#skills)
- [Advanced Features](#advanced-features)

## Getting Started

### First Launch

When you first launch Cortex Coder Agent, you'll see the model selection screen:

```
🤖 Select Model

  ❌ kimi-for-coding - Kimi Code (fast) (kimi-code)
  ▸ ✅ glm-4.7 - GLM 4.7 (reasoning) (zai-coding)
  ❌ claude-opus-4 - Claude Opus (smart) (anthropic)

↑/↓ to navigate • Enter to select • q to quit
```

1. Use `↑`/`↓` or `k`/`j` to navigate the model list
2. Press `Enter` to select a model
3. Connection will be tested automatically
4. Press `y` or `Enter` to continue to the main interface

### Main Interface

The main screen consists of three panels:

```
┌──────────┬─────────────┬──────────────────┐
│  Files   │  Editor     │  Chat           │
│          │             │                 │
│          │             │                 │
│          │             │                 │
└──────────┴─────────────┴──────────────────┘
Status bar • Help bar
```

## Navigation

### Panel Focus

- `1`: Focus File Browser
- `2`: Focus Chat
- `3`: Focus Editor
- `Tab`: Cycle through panels

### Screen Navigation

- `?`: Toggle help screen
- `m`: Return to model selection
- `q` or `Ctrl+C`: Quit application

## File Browser

### Navigation

| Key | Action |
|------|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `←` / `h` | Collapse directory |
| `→` / `l` | Expand directory |
| `Enter` | Open file in editor |
| `Space` | Toggle directory |
| `g` / `Home` | Go to top |
| `G` / `End` | Go to bottom |
| `r` | Reload file tree |

### Git Status Integration

Files are color-coded based on Git status:

- `M` (yellow): Modified
- `?` (red): Untracked
- `A` (green): Added/Staged

### File Icons

Common file extensions have corresponding icons:

- `🐹` Go files
- `📝` Markdown
- `🔧` Shell scripts
- `📋` JSON/YAML
- `🌐` HTML
- `🎨` CSS
- `📄` Other files

## Editor

### File Operations

| Key | Action |
|------|--------|
| `Ctrl+S` | Save current file |
| `Ctrl+W` | Close current tab |
| `Tab` | Switch to next tab |
| `Shift+Tab` | Switch to previous tab |

### Editing

| Key | Action |
|------|--------|
| `Ctrl+Z` | Undo |
| `Ctrl+Y` | Redo |

### View Options

| Key | Action |
|------|--------|
| `Ctrl+L` | Toggle line numbers |
| `PgUp` | Scroll up one page |
| `PgDn` | Scroll down one page |

### Tab Bar

The editor supports multiple tabs:

```
main.go  ●  utils.go  [buffer.go]
```

- `●` indicates unsaved changes
- Active tab is highlighted

### Syntax Highlighting

Code is syntax-highlighted based on file extension using Chroma. Supported languages include:

- Go, Rust, Python, JavaScript/TypeScript
- HTML, CSS, JSON, YAML
- Shell scripts, SQL, Protocol Buffers
- And many more

## Chat Interface

### Sending Messages

| Key | Action |
|------|--------|
| `Enter` | Send message |
| `Ctrl+U` | Clear input |

### Modes

| Key | Action |
|------|--------|
| `i` | Enter insert mode |
| `Esc` | Return to normal mode |

In **insert mode**, all keystrokes go to the input field.
In **normal mode**, use keyboard shortcuts for navigation.

### Message Types

- **You**: Your messages (cyan)
- **Agent**: AI responses (white)
- **System**: System messages (yellow, italic)

### Code Blocks

Messages containing code are displayed in a special formatted block with syntax highlighting.

## Diff Viewer

### Activating Diff Viewer

Press `d` in the main interface to show the diff viewer when there are pending changes.

### Navigation

| Key | Action |
|------|--------|
| `n` / `j` / `↓` | Next change |
| `p` / `k` / `↑` | Previous change |
| `Enter` | Jump to change |

### Reviewing Changes

| Key | Action |
|------|--------|
| `a` / `y` / `Enter` | Accept change |
| `r` / `n` | Reject change |

### View Modes

| Key | Action |
|------|--------|
| `t` | Toggle side-by-side / inline view |

### Bulk Operations

| Key | Action |
|------|--------|
| `A` | Accept all pending changes |
| `R` | Reject all pending changes |

### Diff Display

#### Side-by-Side Mode

```
Original  │  Modified
──────────┼────────────
  1 │  1  package main
  2 │  2  
  3 │  3  func main() {
- 4 │  4  -  fmt.Println("old")
+   │  4  +  fmt.Println("new")
  5 │  5  }
```

#### Inline Mode

```
  1|  1  package main
  2|     │  
  3|  3  func main() {
- 4|    │  -  fmt.Println("old")
+   │  4  +  fmt.Println("new")
  5|  5  }
```

## Change Management

### Workflow

1. **Suggestion Received**: AI suggests code changes
2. **Review**: Use diff viewer to review changes
3. **Action**: Accept, reject, or modify changes
4. **Apply**: Accepted changes are written to disk

### Change Status

- `○`: Pending review
- `✓`: Accepted
- `✗`: Rejected

### Previewing Changes

Press `p` in the change manager to preview the current change in the editor.

### Modifying Changes

Before applying, you can:

1. Preview in diff viewer
2. Open file in editor
3. Make manual adjustments
4. Accept with modifications

## Skills

### Listing Skills

```
/skills list
```

### Running a Skill

```
/skill <skill-name> [arguments]
```

### Common Skills

- `/refactor`: Refactor selected code
- `/test`: Generate tests
- `/doc`: Generate documentation
- `/explain`: Explain selected code
- `/fix`: Fix errors in code

## Advanced Features

### Configuration

#### File Size Limits

Prevent loading large files:

```go
config := tui.AppConfig{
    MaxFileSize: 10 * 1024 * 1024, // 10MB
}
```

#### Chat Retention

Control chat message history:

```go
config := tui.AppConfig{
    ChatRetention: 100, // Keep last 100 messages
}
```

### Performance Tips

1. **For large projects**:
   - Limit chat retention
   - Use file size limits
   - Close unused tabs

2. **For optimal performance**:
   - Keep terminal size reasonable (80x24 minimum)
   - Avoid excessive undo/redo chains
   - Clear chat history periodically

### Keyboard Customization

While the application uses sensible defaults, keyboard shortcuts can be customized by modifying the keybinding definitions in the source code.

## Troubleshooting

### Editor Not Responding

- Check if the editor has focus (highlighted border)
- Press `3` to focus editor
- Verify you're not in chat insert mode

### File Not Opening

- Check file permissions
- Verify file size is under limit
- Ensure file isn't binary (not supported)

### Diff Not Showing

- Changes must be AI-suggested
- Press `d` to open diff viewer
- Verify there are pending changes

### Performance Issues

- Reduce chat retention
- Close unused editor tabs
- Increase file size limits for large files

## Best Practices

### Workflow

1. **Start**: Use file browser to locate files
2. **Review**: Open files in editor to understand code
3. **Interact**: Use chat to ask questions or request changes
4. **Review Changes**: Use diff viewer to review AI suggestions
5. **Apply**: Accept/reject changes as needed
6. **Save**: Save files regularly with `Ctrl+S`

### Code Review

1. Have AI suggest changes
2. Review in diff viewer
3. Preview in editor
4. Test modifications
5. Apply if satisfied

### Learning

- Ask AI to explain unfamiliar code
- Use `/explain` skill for detailed breakdowns
- Review diff to understand changes
- Chat to clarify concepts

## Tips and Tricks

### Quick File Navigation

- Use file browser to find files quickly
- Press `Enter` to open directly
- Tab through open files in editor

### Efficient Chat

- Use insert mode (`i`) for typing
- Use normal mode (`Esc`) for shortcuts
- Provide clear, specific prompts

### Productivity

- Keep related files open in tabs
- Use diff viewer to understand changes
- Apply changes in batches

### Code Quality

- Review all AI suggestions
- Test changes before applying
- Use skills for repetitive tasks
- Generate documentation as you code

## Keyboard Reference Card

### Global

```
1/2/3     - Focus panel
Tab         - Next panel
m           - Change model
?           - Help
q/Ctrl+C     - Quit
```

### File Browser

```
↑/↓  k/j   - Navigate
←/→  h/l   - Collapse/Expand
Enter       - Open
Space       - Toggle
r           - Reload
```

### Editor

```
Ctrl+S      - Save
Ctrl+W      - Close tab
Tab         - Next tab
Shift+Tab   - Prev tab
Ctrl+Z      - Undo
Ctrl+Y      - Redo
Ctrl+L      - Toggle line nums
PgUp/PgDn  - Scroll page
```

### Diff

```
n/j/↓       - Next change
p/k/↑       - Prev change
a/y/Enter   - Accept
r/n         - Reject
t           - Toggle mode
A            - Accept all
R            - Reject all
```

## Getting Help

Press `?` at any time to see the comprehensive help screen with all keyboard shortcuts.

## Additional Resources

- [README.md](README.md) - Project overview
- [CONTRIBUTING.md](CONTRIBUTING.md) - Development guide
- GitHub Issues - Bug reports and feature requests
- Discord - Community support
