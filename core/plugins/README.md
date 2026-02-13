---
project: Cortex
component: Docs
phase: Design
date_created: 2026-02-10T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-11T01:40:42.432623
---

# CortexBrain Plugins

Extend CortexBrain's capabilities with specialized plugins for hardware design, AI enhancement, voice processing, and more.

## Quick Start

```bash
# List installed plugins
cortex-plugin list

# Browse marketplace
cortex-plugin marketplace

# Install a plugin
cortex-plugin install gateflow

# Search for plugins
cortex-plugin search verilog
```

## Installed Plugins

| Plugin | Version | Description |
|--------|---------|-------------|
| [GateFlow](./gateflow/) | 1.5.0 | SystemVerilog/RTL development assistant |

## Marketplace Categories

| Category | Description |
|----------|-------------|
| Hardware Design | RTL, FPGA, ASIC development |
| Code Generation | Source code and boilerplate |
| AI Enhancement | Machine learning capabilities |
| Voice | Speech recognition and synthesis |
| Vision | Image and video processing |
| Testing | Test generation and automation |
| Integration | External service connectors |

## Installing Plugins

### From Marketplace
```bash
cortex-plugin install gateflow
```

### From GitHub
```bash
cortex-plugin install https://github.com/user/plugin.git
```

### From Local Path
```bash
cortex-plugin install /path/to/plugin
```

## Creating Plugins

See [PLUGIN-SPEC.md](./PLUGIN-SPEC.md) for the full plugin specification.

### Minimal Structure
```
my-plugin/
├── .claude-plugin/
│   └── plugin.json      # Required
├── CLAUDE.md            # Quick reference
└── agents/              # Agent definitions
```

### plugin.json
```json
{
  "name": "my-plugin",
  "version": "1.0.0",
  "description": "What this plugin does",
  "author": {"name": "Your Name", "github": "https://github.com/you"},
  "keywords": ["keyword1", "keyword2"],
  "license": "MIT",
  "repository": "https://github.com/you/my-plugin"
}
```

## Plugin Manager

The plugin manager provides:
- Marketplace browsing and search
- Installation from multiple sources
- Version management and updates
- Plugin validation

See [manager/README.md](./manager/README.md) for details.

## Files

| File | Purpose |
|------|---------|
| [PLUGIN-SPEC.md](./PLUGIN-SPEC.md) | Plugin format specification |
| [marketplace.json](./marketplace.json) | Marketplace registry |
| [manager/](./manager/) | Plugin manager code |
| [gateflow/](./gateflow/) | GateFlow hardware design plugin |

## Contributing

To add a plugin to the marketplace:

1. Create your plugin following [PLUGIN-SPEC.md](./PLUGIN-SPEC.md)
2. Test locally: `cortex-plugin install /path/to/your-plugin`
3. Push to GitHub
4. Submit a PR adding your plugin to `marketplace.json`

## See Also

- [Hardware Design PRD](../docs/HARDWARE-DESIGN-PRD.md)
- [Plugin System PRD](../docs/PLUGIN-SYSTEM-PRD.md)
- [GateFlow Reference](./gateflow/GATEFLOW-REFERENCE.md)
