---
project: Cortex
component: Agents
phase: Ideation
date_created: 2026-02-04T16:56:53
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:21:13.365724
---

# Cortex Coder Agent

An intelligent coding assistant built on top of CortexBrain, featuring a modern terminal UI (TUI) with syntax highlighting, code browsing, and AI-powered suggestions.

## Features

- **Smart AI Assistant**: Powered by CortexBrain with multiple model support
- **Modern TUI**: Beautiful terminal interface built with BubbleTea
- **Code Editor**: Full-featured editor with syntax highlighting
- **File Browser**: Navigate your project with Git integration
- **Diff Viewer**: Side-by-side and inline diff views
- **Change Management**: Review and apply AI-suggested changes
- **Multiple Tabs**: Work with multiple files simultaneously
- **Skills System**: Extensible plugin architecture for custom commands

## Screenshots

```
┌─────────────────────────────────────────────────────────────────────┐
│  🧠 Cortex Coder — kimi-for-coding                          │
├────────────┬────────────────────┬──────────────────────────────────┤
│  📁 src/  │  ┌─ main.go * ───┐  │  💬 Chat                   │
│  ▸ cmd/    │  │ 1 package main   │  │  > Add error handling      │
│  ▸ pkg/    │  │ 2              │  │                          │
│    tui/    │  │ 3 func main() { │  │  Agent: I'll help you     │
│      ...    │  │ 4   // TODO    │  │  add error handling...     │
│  ▸ go.mod  │  │ 5 }            │  │                          │
│            │  └─────────────────┘  │                          │
│            │  Editor: 2/3          │                          │
└────────────┴────────────────────┴──────────────────────────────────┘
1:files • 2:chat • 3:editor • d:diff • tab:switch • m:model • ?:help
```

## Installation

### Prerequisites

- Go 1.24.2 or later
- CortexBrain server running (Pink: http://192.168.1.186:18892)

### Build from Source

```bash
# Clone the repository
git clone https://github.com/RedClaus/cortex-coder-agent.git
cd cortex-coder-agent

# Build the binary
make build

# (Optional) Install to $GOPATH/bin
make install
```

### Using Makefile

```bash
make build    # Build the binary
make install  # Install to $GOPATH/bin
make test     # Run tests
make clean    # Clean build artifacts
make uninstall # Remove installed binary
```

## Quick Start

```bash
# Start the TUI
./coder

# Navigate the model selection screen with ↑/↓
# Press Enter to select a model
# Confirm connection to start the main interface
```

## Usage

### Main Interface

The TUI is divided into three panels:

1. **File Browser (1)**: Navigate and select files
2. **Editor (3)**: View and edit code with syntax highlighting
3. **Chat (2)**: Interact with the AI assistant

### File Browser

- `↑/↓` or `k/j`: Navigate files
- `←/→` or `h/l`: Collapse/expand directories
- `Enter`: Open file in editor
- `Space`: Toggle directory
- `r`: Reload file tree

### Editor

- `Ctrl+S`: Save file
- `Ctrl+W`: Close current tab
- `Tab`: Next tab
- `Shift+Tab`: Previous tab
- `Ctrl+Z`: Undo
- `Ctrl+Y`: Redo
- `Ctrl+L`: Toggle line numbers
- `PgUp/PgDn`: Scroll page

### Chat

- `Enter`: Send message
- `Ctrl+U`: Clear input
- `i`: Enter insert mode
- `Esc`: Normal mode

### Diff Viewer

- `d`: Show changes/diff viewer
- `n/j/↓`: Next change
- `p/k/↑`: Previous change
- `a/y/Enter`: Accept change
- `r/n`: Reject change
- `t`: Toggle view mode (side-by-side/inline)
- `A`: Accept all changes
- `R`: Reject all changes

### Global Shortcuts

- `1/2/3`: Focus panel (files/editor/chat)
- `Tab`: Switch to next panel
- `m`: Change model
- `?`: Toggle help
- `q` or `Ctrl+C`: Quit

## Configuration

The application can be configured via command-line flags or environment variables:

```bash
# Run with custom root path
./coder --root-path /path/to/project

# Set session name
./coder --session-name "My Session"

# Use specific theme
./coder --theme dracula
```

### Available Themes

- `dracula`: Dark theme (default)
- `default`: Basic dark theme

## Skills System

Cortex Coder Agent supports an extensible skills system. Skills are plugins that provide custom functionality:

```bash
# List available skills
/skills list

# Execute a skill
/skill <skill-name> [args...]
```

## AI Integration

The AI assistant can help with:

- Code generation and completion
- Bug finding and fixing
- Refactoring suggestions
- Code explanations
- Test generation
- Documentation writing

### Example Interactions

```
You: Add input validation to this function
Agent: [Provides suggested code changes]

You: Explain how this works
Agent: [Provides detailed explanation]

You: Write tests for this module
Agent: [Generates comprehensive test suite]
```

## Troubleshooting

### Connection Issues

If you can't connect to CortexBrain:

```bash
# Check if CortexBrain is running
curl http://192.168.1.186:18892/health

# Verify the URL in your configuration
# Update if running on different host/port
```

### Performance Issues

For large projects:

- Increase file size limit in configuration
- Enable virtual scrolling (automatic)
- Clear old chat messages with retention policy

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md) for development guidelines.

## License

MIT License - see LICENSE file for details

## Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Changelog

### Version 0.3.0 (Phase 3)

- ✅ Added syntax highlighting with Chroma
- ✅ Implemented multi-tab editor
- ✅ Created diff viewer with side-by-side/inline modes
- ✅ Added change management workflow
- ✅ Integrated editor into main app
- ✅ Added keyboard shortcuts help panel
- ✅ Implemented file size limits
- ✅ Added chat message retention
- ✅ Performance optimizations with virtual scrolling

### Version 0.2.0 (Phase 2)

- ✅ Skills system implementation
- ✅ LSP integration
- ✅ Extensible plugin architecture

### Version 0.1.0 (Phase 1)

- ✅ Initial TUI implementation
- ✅ Chat interface
- ✅ File browser
- ✅ Model selection

## Support

For issues, questions, or suggestions:
- Open an issue on GitHub
- Join our Discord community
- Check the documentation at https://docs.cortex.ai

## Acknowledgments

- Built with [BubbleTea](https://github.com/charmbracelet/bubbletea)
- Syntax highlighting by [Chroma](https://github.com/alecthomas/chroma)
- Powered by [CortexBrain](https://github.com/RedClaus/cortex-brain)
