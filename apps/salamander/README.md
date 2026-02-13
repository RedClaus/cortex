---
project: Cortex
component: Docs
phase: Design
date_created: 2025-12-20T01:13:33
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:20:40.339618
---

# Salamander

A YAML-driven Terminal User Interface (TUI) framework for A2A-compliant AI agents.

## Overview

Salamander allows you to create fully customizable terminal interfaces through YAML configuration files. Instead of writing code, you define your menus, themes, keybindings, and layout in YAML, and Salamander renders a flicker-free BubbleTea TUI.

## Features

- **YAML-Driven Configuration**: Define your entire TUI in YAML
- **A2A Protocol Support**: Connect to any A2A-compliant agent
- **Visual YAML Builder**: Interactive UI for creating/editing configurations
- **Flicker-Free Rendering**: Diff-based updates via BubbleTea
- **Cascading Menus**: Full menu system with submenus
- **Theming**: Customizable colors and styles
- **Keybindings**: Configurable keyboard shortcuts

## Architecture

```
┌──────────────────────────────────────────────────────────────────────┐
│                         SALAMANDER TUI                               │
│                                                                      │
│   salamander.yaml ──────────────────────────────────────────────┐    │
│                                                                  │    │
│   ┌────────────────┐   ┌────────────────┐   ┌────────────────┐  │    │
│   │  Menu System   │   │  Theme Engine  │   │  Keybindings   │  │    │
│   │  (from YAML)   │   │  (from YAML)   │   │  (from YAML)   │  │    │
│   └───────┬────────┘   └───────┬────────┘   └───────┬────────┘  │    │
│           │                    │                    │           │    │
│           └────────────────────┴────────────────────┘           │    │
│                                │                                │    │
│                    ┌───────────▼───────────┐                    │    │
│                    │    BubbleTea App      │                    │    │
│                    │  (flicker-free TUI)   │                    │    │
│                    └───────────┬───────────┘                    │    │
│                                │                                │    │
└────────────────────────────────┼────────────────────────────────┘    │
                                 │ A2A Protocol                        │
                                 ▼                                     │
                    ┌────────────────────────┐                         │
                    │    Cortex / Any A2A    │                         │
                    │         Agent          │                         │
                    └────────────────────────┘                         │
```

## Quick Start

```bash
# Run with default config
salamander

# Use specific config file
salamander --config cortex.yaml

# Connect to A2A agent
salamander --url http://localhost:8080

# Open YAML Builder UI
salamander --builder
```

## YAML Configuration

Example configuration (see `configs/cortex.yaml` for full example):

```yaml
version: "1.0"

app:
  name: "My AI Assistant"
  show_status_bar: true
  welcome_message: "Type a message or use / for commands."

theme:
  name: "Dark Mode"
  mode: "dark"
  colors:
    primary: "#7C3AED"
    background: "#0F172A"
    text: "#F8FAFC"

menus:
  - id: "main_menu"
    trigger: "/"
    title: "Commands"
    filterable: true
    items:
      - id: "help"
        label: "help"
        description: "Show help"
        icon: "❓"
        action:
          type: "command"
          command: "show_help"
          
      - id: "select_model"
        label: "select"
        description: "Select AI model"
        icon: "🤖"
        action:
          type: "submenu"
        submenu:
          # ... cascading menu
          
keybindings:
  - key: "ctrl+c"
    action:
      type: "quit"
    description: "Exit"

backend:
  type: "a2a"
  url: "http://localhost:8080"
  streaming: true
```

## YAML Builder

Salamander includes an interactive YAML Builder that lets you create and modify configurations without editing YAML directly:

```bash
salamander --builder
```

The builder provides:
- Menu editor with add/remove/reorder
- Theme customization
- Keybinding configuration
- Backend settings
- Live preview

Access the builder from within Salamander via `/builder` command.

## Pre-built Configurations

The `configs/` directory contains ready-to-use configurations:

- **cortex.yaml** - Full Cortex Brain TUI experience
- **minimal.yaml** - Minimal chat interface
- **developer.yaml** - Development-focused tools

## Schema

See `pkg/schema/types.go` for the complete YAML schema definition.

### Key Types

- **AppConfig** - Application settings
- **ThemeConfig** - Colors and styling
- **LayoutConfig** - Component positioning
- **MenuConfig** - Command menus
- **MenuItemConfig** - Individual menu items with actions
- **KeybindingConfig** - Keyboard shortcuts
- **BackendConfig** - A2A connection settings

### Action Types

Menu items and keybindings support these action types:

- `command` - Execute a built-in command
- `submenu` - Open a submenu
- `a2a_request` - Send message to A2A agent
- `set_variable` - Set a configuration variable
- `open_dialog` - Open an input dialog
- `quit` - Exit the application

## Integration with Cortex

Salamander is designed to work with Cortex Brain's A2A server:

```bash
# Terminal 1: Start Cortex A2A Server
cd /path/to/CortexBrain
go run ./cmd/cortex-server --port 8080

# Terminal 2: Start Salamander
cd /path/to/Salamander
go run ./cmd/salamander --config configs/cortex.yaml
```

## Directory Structure

```
Salamander/
├── cmd/
│   └── salamander/
│       └── main.go          # Entry point
├── configs/
│   └── cortex.yaml          # Cortex TUI config
├── internal/
│   ├── app/                 # TUI application
│   ├── builder/             # YAML Builder UI
│   ├── menu/                # Menu system
│   ├── widgets/             # UI components
│   └── backend/             # A2A client
├── pkg/
│   └── schema/
│       └── types.go         # YAML schema types
├── go.mod
└── README.md
```

## License

Apache 2.0
