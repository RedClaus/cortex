---
project: Cortex
component: Docs
phase: Ideation
date_created: 2026-02-10T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-11T01:40:42.673778
---

# CortexBrain Plugin System PRD

## Executive Summary

The CortexBrain Plugin System enables extensibility through a standardized plugin architecture. Plugins can add specialized agents, skills, commands, and tools to enhance CortexBrain's capabilities. The system includes marketplace discovery, version management, and CLI tooling.

---

## Goals

1. **Extensibility** - Allow third-party capabilities without modifying core
2. **Discoverability** - Marketplace for finding and sharing plugins
3. **Standardization** - Consistent plugin format compatible with Claude Code
4. **Simplicity** - Easy installation, updates, and removal

---

## Architecture

### Plugin Structure

```
plugin-name/
├── .claude-plugin/
│   └── plugin.json        # Metadata (required)
├── CLAUDE.md              # Quick reference
├── agents/                # Specialized agents
├── skills/                # Workflow skills
├── commands/              # Quick commands
├── hooks/                 # Automation hooks
└── tools/                 # Custom tools (optional)
```

### Plugin Manager

```
plugins/
├── manager/
│   ├── plugin_manager.go  # Core manager logic
│   ├── cmd/
│   │   └── cortex-plugin/ # CLI tool
│   └── README.md
├── marketplace.json       # Registry
├── PLUGIN-SPEC.md         # Specification
└── <installed-plugins>/
```

### Marketplace Registry

```json
{
  "plugins": [
    {
      "name": "gateflow",
      "version": "1.5.0",
      "repository": "https://github.com/...",
      "categories": ["hardware-design"],
      "verified": true,
      "featured": true
    }
  ]
}
```

---

## Components

### 1. Plugin Specification (`PLUGIN-SPEC.md`)

Defines the standard format for CortexBrain plugins:
- `plugin.json` schema
- Agent definition format
- Skill definition format
- Command definition format
- Hook configuration

### 2. Plugin Manager (`manager/plugin_manager.go`)

Go package for programmatic plugin management:
- `LoadMarketplace()` - Fetch registry
- `Search(query)` - Find plugins
- `Install(source)` - Install from marketplace/GitHub/local
- `Remove(name)` - Uninstall plugin
- `Update(name)` - Update to latest version
- `List()` - List installed plugins

### 3. CLI Tool (`cortex-plugin`)

Command-line interface for users:

| Command | Description |
|---------|-------------|
| `list` | List installed plugins |
| `search <query>` | Search marketplace |
| `install <source>` | Install plugin |
| `remove <name>` | Remove plugin |
| `update <name>` | Update plugin |
| `info <name>` | Show details |
| `marketplace` | Browse all |
| `categories` | List categories |
| `featured` | Show featured |

### 4. Marketplace (`marketplace.json`)

Central registry of available plugins:
- Official plugins from CortexBrain team
- Community contributions via PR
- Multiple registry sources (official + community)

---

## Installation Sources

| Source | Example | Description |
|--------|---------|-------------|
| Marketplace | `gateflow` | Install by name |
| GitHub | `https://github.com/...` | Clone from repo |
| Local | `/path/to/plugin` | Copy local directory |

---

## Plugin Categories

| Category | Capability |
|----------|------------|
| `hardware-design` | RTL, FPGA, ASIC |
| `code-generation` | Source code generation |
| `ai-enhancement` | ML/AI capabilities |
| `voice` | Speech processing |
| `vision` | Image/video processing |
| `testing` | Test automation |
| `integration` | External services |
| `memory` | Knowledge management |

---

## Validation

Plugins are validated on installation:

1. **Structure Check** - `.claude-plugin/plugin.json` exists
2. **Schema Check** - Required fields present (name, version)
3. **Dependency Check** - Required plugins installed
4. **Security Check** - No malicious patterns (future)

---

## Integration with CortexBrain

### Intent Detection

Plugins can declare triggers for routing:

```json
{
  "cortexbrain": {
    "triggers": ["design hardware", "create module", "verilog"]
  }
}
```

### Tool Registration

Plugins can provide tools:

```json
{
  "cortexbrain": {
    "tools": ["gateflow"]
  }
}
```

### Lobe Integration

Plugins can extend lobes:

```json
{
  "cortexbrain": {
    "lobes": ["hardware-design-lobe"]
  }
}
```

---

## Security Considerations

1. **Source Verification** - Verified badge for trusted plugins
2. **Code Review** - Marketplace plugins reviewed before listing
3. **Sandboxing** - Future: isolate plugin execution
4. **Permissions** - Future: declare required permissions

---

## Implementation Status

### Phase 1: Core (Complete)
- [x] Plugin specification
- [x] Plugin manager package
- [x] CLI tool
- [x] Marketplace registry
- [x] GateFlow as reference implementation

### Phase 2: Integration (Planned)
- [ ] Auto-load plugins on CortexBrain start
- [ ] Trigger-based routing to plugins
- [ ] Plugin tool registration
- [ ] Plugin capability discovery

### Phase 3: Ecosystem (Future)
- [ ] Community plugin registry
- [ ] Plugin ratings and reviews
- [ ] Automatic updates
- [ ] Security scanning

---

## Usage Examples

### Install from Marketplace
```bash
cortex-plugin install gateflow
```

### Search for Plugins
```bash
cortex-plugin search verilog
```

### Create Custom Plugin
```bash
mkdir my-plugin
mkdir -p my-plugin/.claude-plugin my-plugin/agents

cat > my-plugin/.claude-plugin/plugin.json << 'EOF'
{
  "name": "my-plugin",
  "version": "1.0.0",
  "description": "My custom plugin",
  "author": {"name": "Me"},
  "keywords": ["custom"],
  "license": "MIT",
  "repository": "https://github.com/me/my-plugin"
}
EOF

# Install locally
cortex-plugin install ./my-plugin
```

---

## Files

| File | Purpose |
|------|---------|
| `plugins/PLUGIN-SPEC.md` | Plugin format specification |
| `plugins/marketplace.json` | Marketplace registry |
| `plugins/manager/plugin_manager.go` | Manager package |
| `plugins/manager/cmd/cortex-plugin/main.go` | CLI tool |
| `plugins/README.md` | User documentation |

---

## See Also

- [Plugin Specification](../plugins/PLUGIN-SPEC.md)
- [Plugin Manager](../plugins/manager/README.md)
- [Hardware Design PRD](./HARDWARE-DESIGN-PRD.md)
- [GateFlow Plugin](../plugins/gateflow/)
