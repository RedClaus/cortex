---
project: Cortex
component: Docs
phase: Ideation
date_created: 2026-02-10T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-11T01:40:42.449202
---

# CortexBrain Plugin Manager

Manage plugins for CortexBrain - install from the marketplace, GitHub, or local development.

## Quick Start

```bash
# Build the CLI
cd CortexBrain/plugins/manager/cmd/cortex-plugin
go build -o cortex-plugin .

# Or install globally
go install ./...

# List installed plugins
cortex-plugin list

# Browse marketplace
cortex-plugin marketplace

# Install a plugin
cortex-plugin install gateflow
```

## Commands

| Command | Description |
|---------|-------------|
| `list` | List installed plugins |
| `search <query>` | Search the marketplace |
| `install <source>` | Install a plugin |
| `remove <name>` | Remove an installed plugin |
| `update <name>` | Update to latest version |
| `info <name>` | Show plugin details |
| `marketplace` | Browse all marketplace plugins |
| `categories` | List plugin categories |
| `featured` | Show featured plugins |

## Installation Sources

The `install` command accepts three source types:

### Marketplace (by name)
```bash
cortex-plugin install gateflow
```

### GitHub URL
```bash
cortex-plugin install https://github.com/codejunkie99/Gateflow-Plugin
```

### Local Path
```bash
cortex-plugin install /path/to/my-plugin
cortex-plugin install ./my-plugin
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `CORTEX_PLUGINS_DIR` | Plugins directory | `~/ServerProjectsMac/CortexBrain/plugins` |

## Plugin Structure

Plugins must follow the standard structure:

```
my-plugin/
├── .claude-plugin/
│   └── plugin.json      # Required metadata
├── CLAUDE.md            # Quick reference
├── README.md            # Documentation
├── agents/              # Agent definitions
├── skills/              # Skill workflows
├── commands/            # Quick commands
└── hooks/               # Automation hooks
```

See [PLUGIN-SPEC.md](../PLUGIN-SPEC.md) for full specification.

## Examples

### Search for hardware plugins
```bash
cortex-plugin search hardware
```

### Install from GitHub
```bash
cortex-plugin install https://github.com/user/cool-plugin.git
```

### Update all plugins
```bash
for plugin in $(cortex-plugin list | tail -n +3 | awk '{print $1}'); do
  cortex-plugin update "$plugin"
done
```

### Show plugin details
```bash
cortex-plugin info gateflow
```

## Marketplace

The marketplace registry is stored in `marketplace.json` and can be extended with community sources.

### Adding Plugins to Marketplace

To add your plugin to the marketplace:

1. Ensure your plugin follows the [Plugin Spec](../PLUGIN-SPEC.md)
2. Submit a PR to add your plugin to `marketplace.json`
3. Include: name, version, description, repository, categories, keywords

### Registry Sources

The manager checks multiple registry sources in priority order:

1. **Official** - CortexBrain main repo
2. **Community** - Community plugin registry (planned)

## Development

### Building

```bash
cd plugins/manager/cmd/cortex-plugin
go build -o cortex-plugin .
```

### Testing

```bash
go test ./...
```

### Using the Manager Package

```go
import "github.com/normanking/cortex/plugins/manager"

m := manager.NewManager("/path/to/plugins")
m.LoadMarketplace()
m.LoadInstalled()

// Search
results := m.Search("hardware")

// Install
m.Install("gateflow")

// List
for _, p := range m.List() {
    fmt.Println(p.Name, p.Version)
}
```

## See Also

- [Plugin Specification](../PLUGIN-SPEC.md)
- [Marketplace Registry](../marketplace.json)
- [GateFlow Plugin](../gateflow/) - Example plugin
