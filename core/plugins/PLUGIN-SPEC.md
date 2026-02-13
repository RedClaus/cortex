---
project: Cortex
component: Docs
phase: Design
date_created: 2026-02-10T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-11T01:40:42.418906
---

# CortexBrain Plugin Specification v1.0

## Overview

CortexBrain plugins extend the brain's capabilities with specialized agents, skills, and tools. Plugins follow a standard format compatible with Claude Code plugins.

---

## Plugin Structure

```
my-plugin/
├── .claude-plugin/
│   └── plugin.json        # Plugin metadata (REQUIRED)
├── CLAUDE.md              # Quick reference for the capability
├── README.md              # Documentation
├── agents/                # Specialized agent definitions
│   └── agent-name.md
├── skills/                # Workflow skills
│   └── skill-name/
│       └── SKILL.md
├── commands/              # Quick-action commands
│   └── command-name.md
├── hooks/                 # Automation hooks
│   ├── hooks.json
│   └── scripts/
└── tools/                 # Custom tool implementations (optional)
    └── tool-name.go
```

---

## plugin.json Schema

```json
{
  "name": "my-plugin",
  "version": "1.0.0",
  "description": "What this plugin does",
  "author": {
    "name": "Author Name",
    "github": "https://github.com/username",
    "email": "author@example.com"
  },
  "keywords": ["keyword1", "keyword2"],
  "license": "MIT",
  "repository": "https://github.com/username/my-plugin",

  "cortexbrain": {
    "minVersion": "1.0.0",
    "capabilities": ["hardware-design", "code-generation"],
    "triggers": ["design", "create module", "generate"],
    "dependencies": [],
    "tools": ["gateflow"],
    "lobes": []
  }
}
```

### Field Descriptions

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Unique plugin identifier (kebab-case) |
| `version` | Yes | Semantic version (major.minor.patch) |
| `description` | Yes | Short description (max 200 chars) |
| `author` | Yes | Author information |
| `keywords` | Yes | Searchable keywords |
| `license` | Yes | SPDX license identifier |
| `repository` | Yes | Git repository URL |
| `cortexbrain.minVersion` | No | Minimum CortexBrain version |
| `cortexbrain.capabilities` | No | Capability categories |
| `cortexbrain.triggers` | No | Phrases that activate this plugin |
| `cortexbrain.dependencies` | No | Other required plugins |
| `cortexbrain.tools` | No | Tools this plugin provides |
| `cortexbrain.lobes` | No | Lobes this plugin integrates with |

---

## Capability Categories

Plugins should declare their capabilities for routing:

| Category | Description |
|----------|-------------|
| `hardware-design` | RTL/FPGA/ASIC design |
| `code-generation` | Generate source code |
| `testing` | Test generation and execution |
| `documentation` | Doc generation |
| `analysis` | Code/data analysis |
| `integration` | External service integration |
| `ai-enhancement` | AI/ML capabilities |
| `voice` | Voice processing |
| `vision` | Image/video processing |
| `memory` | Memory/knowledge management |

---

## Agent Definition Format

Agents are markdown files in `agents/`:

```markdown
---
name: Agent Name
description: What this agent does
triggers:
  - "trigger phrase 1"
  - "trigger phrase 2"
expertise:
  - area1
  - area2
handoff:
  - other-agent-on-completion
---

# Agent Name

## Role
Describe the agent's role and expertise.

## Instructions
Step-by-step instructions for the agent.

## Examples
Example interactions.
```

---

## Skill Definition Format

Skills are in `skills/<skill-name>/SKILL.md`:

```markdown
---
name: skill-name
description: What this skill does
triggers:
  - "/skill-command"
  - "natural language trigger"
---

# Skill Name

## Purpose
What this skill accomplishes.

## Steps
1. Step one
2. Step two
3. Step three

## Inputs
Required inputs and how to obtain them.

## Outputs
What the skill produces.
```

---

## Command Definition Format

Commands are quick-action shortcuts in `commands/`:

```markdown
---
name: command-name
description: Quick description
usage: /command-name [args]
---

# /command-name

## Usage
```
/command-name [options]
```

## Examples
- `/command-name` - Basic usage
- `/command-name --option` - With option
```

---

## Installation Sources

Plugins can be installed from:

1. **Marketplace** - Official registry at `marketplace.json`
2. **GitHub URL** - Direct repository URL
3. **Local Path** - Development plugins

```bash
# From marketplace
cortex plugin install gateflow

# From GitHub
cortex plugin install https://github.com/user/plugin.git

# From local
cortex plugin install /path/to/plugin
```

---

## Plugin Lifecycle

### Installation
1. Clone/copy to `CortexBrain/plugins/`
2. Validate `plugin.json` schema
3. Check dependencies
4. Register capabilities
5. Index agents/skills/commands

### Activation
1. Load `CLAUDE.md` into context
2. Register triggers
3. Enable tools

### Update
1. Pull latest from repository
2. Re-validate and re-register

### Removal
1. Unregister capabilities
2. Remove from `plugins/`
3. Clean up indices

---

## Best Practices

1. **Keep plugins focused** - One capability domain per plugin
2. **Include CLAUDE.md** - Quick reference for the capability
3. **Document triggers** - Help routing find your plugin
4. **Test locally first** - Use local install before publishing
5. **Version semantically** - Breaking changes = major version bump
6. **Declare dependencies** - If you need another plugin, say so

---

## Example: Minimal Plugin

```
minimal-plugin/
├── .claude-plugin/
│   └── plugin.json
├── CLAUDE.md
└── agents/
    └── helper.md
```

**plugin.json:**
```json
{
  "name": "minimal-plugin",
  "version": "1.0.0",
  "description": "A minimal example plugin",
  "author": {"name": "Example", "github": "https://github.com/example"},
  "keywords": ["example", "minimal"],
  "license": "MIT",
  "repository": "https://github.com/example/minimal-plugin"
}
```

**CLAUDE.md:**
```markdown
# Minimal Plugin

Quick reference for the minimal plugin capability.

## Usage
Just ask and I'll help!
```

**agents/helper.md:**
```markdown
---
name: Helper Agent
description: A helpful example agent
triggers: ["help me", "assist"]
---

# Helper Agent

I'm here to help with example tasks.
```

---

## See Also

- [Plugin Manager](./manager/README.md)
- [Marketplace Registry](./marketplace.json)
- [GateFlow Plugin](./gateflow/) - Reference implementation
