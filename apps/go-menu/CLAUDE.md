---
project: Cortex
component: Unknown
phase: Ideation
date_created: 2026-01-16T11:58:54
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:18:25.818903
---

# GoMenu - Claude Context

## Project Overview

GoMenu is a macOS menu bar application written in Go that provides quick access to applications, scripts, and commands. It displays a red lightning bolt icon in the menu bar and offers a configurable dropdown menu.

## Project Location

```
/Users/normanking/ServerProjectsMac/GoMenu
```

## Architecture

### Core Components

1. **main.go** - Single-file application containing:
   - Menu bar icon rendering (red lightning bolt)
   - Configuration loading/saving
   - Menu item creation and click handling
   - Command execution (CortexBrain fallback to direct bash)
   - Script scanning functionality

2. **config.json** - JSON configuration file with:
   - `items[]` - Array of menu items (name, command, category)
   - `scanPaths[]` - Directories to scan for scripts

3. **GoMenu.app** - macOS app bundle for distribution

### Dependencies

- `github.com/getlantern/systray` - System tray/menu bar library for Go

## Key Features

### Menu Structure
- Items grouped by category with headers
- Categories appear in order they're first encountered in config
- Separator between each category group
- Utility items at bottom: Scan, Edit Config, Reload, Quit

### Command Execution Flow
1. Try CortexBrain JSON-RPC endpoint (`http://localhost:8080/`)
2. On failure, fall back to direct bash execution
3. Commands run detached (new session, no controlling terminal)
4. Notifications shown via macOS AppleScript

### Script Scanning
- **Configurable time period**: 1 Week, 1 Month, 2 Months, 6 Months, 1 Year, or All Time
- **Configurable location**: All configured paths, individual path, or custom folder
- Custom folders can be saved to config for future scans
- Shows AppleScript dialog to select scripts to add
- Auto-restarts app after adding scripts

## Configuration

### Default Scan Paths
```
~/scripts
~/bin
~/.local/bin
```

### Menu Item Structure
```json
{
  "name": "Display Name",
  "command": "command to execute",
  "category": "Category Name",
  "icon": "optional icon path"
}
```

## Build Commands

```bash
# Build binary
cd /Users/normanking/ServerProjectsMac/GoMenu
go build -o GoMenu main.go

# Build for release (smaller binary)
go build -ldflags="-s -w" -o GoMenu main.go
```

## Running

```bash
# Run directly (for development)
./GoMenu

# Run the installed app
open /Applications/GoMenu.app
```

## Logging

Logs written to: `~/gomenu.log`

Log includes:
- Startup information
- Config loading status
- Menu item creation
- Click events
- Command execution results
- CortexBrain responses

## App Bundle Structure

```
GoMenu.app/
  Contents/
    Info.plist          # App metadata
    MacOS/
      GoMenu            # Binary
      config.json       # Config (optional, copied for bundled use)
    _CodeSignature/
      CodeResources     # Code signing data
```

## Installation to Applications

To install as a proper macOS app:

1. Build the binary:
   ```bash
   go build -ldflags="-s -w" -o GoMenu main.go
   ```

2. Create/update app bundle:
   ```bash
   mkdir -p GoMenu.app/Contents/MacOS
   cp GoMenu GoMenu.app/Contents/MacOS/
   cp config.json GoMenu.app/Contents/MacOS/
   ```

3. Copy to Applications:
   ```bash
   cp -R GoMenu.app /Applications/
   ```

4. Optional: Add to Login Items for auto-start

## CortexBrain Integration

Currently tries to send commands via CortexBrain JSON-RPC:
- Endpoint: `http://localhost:8080/`
- Method: `tasks/send`
- Always fails with "method not found" (CortexBrain doesn't implement this method)
- Fallback to direct execution works fine

**TODO**: Either implement proper method in CortexBrain or remove this integration.

## Known Issues

1. CortexBrain integration always fails - uses fallback
2. No icon file support yet (generates programmatic icon)
3. `strings.Title()` is deprecated - should use `cases.Title()`

## Future Enhancements Ideas

- [ ] Custom icon support per menu item
- [ ] Submenu support for nested categories
- [ ] Keyboard shortcuts for menu items
- [ ] Recent items section
- [ ] Favorites/pinned items
- [ ] Dark mode icon variant
- [ ] Status indicators for running processes
- [ ] Integration with Cortex task system

## Files

| File | Purpose |
|------|---------|
| main.go | Main application source |
| config.json | Menu configuration |
| go.mod | Go module definition |
| go.sum | Dependency checksums |
| GoMenu | Compiled binary |
| GoMenu.app/ | macOS app bundle |
| icons/ | Icon assets (empty) |
| scripts/ | Build scripts (empty) |
| .gitignore | Git ignore patterns |
| CLAUDE.md | This documentation |

## Development Workflow

1. Edit `main.go` or `config.json`
2. Build: `go build -o GoMenu main.go`
3. Test: `./GoMenu`
4. Check logs: `tail -f ~/gomenu.log`
5. Kill running instance: `pkill -f GoMenu`

## Quick Commands

```bash
# Kill all GoMenu processes
pkill -f GoMenu

# Rebuild and run
go build -o GoMenu main.go && ./GoMenu

# View recent logs
tail -50 ~/gomenu.log

# Watch logs live
tail -f ~/gomenu.log
```


---

## Workspace Standards Reference

This project inherits the **Cortex Project Rules & Standards** from the workspace root.

**See:** [ServerProjectsMac/CLAUDE.md](../CLAUDE.md#cortex-project-rules--standards) for:
- Software Engineering Principles (SOLID, DRY, naming conventions)
- Testing & Quality Control (TDD, coverage, edge cases)
- Bug Fixing & Diagnostics (root cause analysis, regression tests)
- Security & Reliability (input validation, secrets management)
- Documentation & Workflow (conventional commits, self-correction)
- Language-Specific Standards (linting, testing, package managers)


---

## Software Manufacturing Process

This project follows a structured software manufacturing process to ensure quality and consistency across the full development lifecycle.

**Full specification:** See [`.claude/process/PROCESS.md`](.claude/process/PROCESS.md)

### Quick Reference

| Phase | Purpose | Key Artifacts |
|-------|---------|---------------|
| Discovery | Understand the problem | requirements.md, success-criteria.md |
| Design | Define the solution | architecture.md, API contracts, ADRs |
| Implementation | Build the solution | Source code, unit tests |
| Testing | Verify correctness | Test plan, coverage reports |
| Deployment | Release safely | Release notes, rollback plan |
| Operations | Monitor and maintain | Incident logs, retrospectives |

### Process Commands

| Command | Action |
|---------|--------|
| `process:status` | Show current phase and gate status |
| `process:advance` | Validate gates and transition to next phase |
| `process:init [name]` | Initialize directory structure for new feature |
| `process:gate-check` | Audit artifacts against current phase |

### State Tracking

Process state is maintained in `.claude/process/state.json`. Always check current phase status before beginning work on a feature.
