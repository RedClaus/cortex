---
project: Cortex
component: Docs
phase: Archive
date_created: 2025-12-18T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:51.655062
---

# Prompts Store Integration

This package provides tier-optimized system prompts for different AI models, integrated with the Cortex cognitive architecture.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Prompts Integration                      │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────┐         ┌──────────────────┐             │
│  │ Store        │────────►│ TemplateProvider │             │
│  │ (YAML)       │         │ (Integration)    │             │
│  └──────────────┘         └──────────────────┘             │
│         │                           │                       │
│         │                           │                       │
│         ▼                           ▼                       │
│  optimized.yaml            ┌──────────────────┐             │
│  - terminal_error_diagnosis│ PromptManager    │             │
│  - command_suggestion      │ (Cognitive)      │             │
│  - code_explanation        └──────────────────┘             │
│  - vision_*                         │                       │
│  - tui_*                            │                       │
│  - agentic_tool_use                 ▼                       │
│                            ┌──────────────────┐             │
│                            │ Orchestrator     │             │
│                            │ (WithPromptMgr)  │             │
│                            └──────────────────┘             │
└─────────────────────────────────────────────────────────────┘
```

## Components

### 1. Store (`store.go`)
- Loads embedded YAML with tier-optimized prompts
- Maps tasks to tiers (small/large)
- Thread-safe access with graceful fallback

### 2. TemplateProvider (`integration.go`)
- Wraps Store for cognitive template use
- Supports custom prompt registration
- Provides task listing and tier enumeration

### 3. PromptManager (`cognitive/prompts.go`)
- High-level API for cognitive architecture
- Model tier mapping (local/mid/advanced/frontier)
- Template context enrichment

## Tier Selection

**Model Size → Tier Mapping:**
- < 3B params → `TierLocal` → `small` prompts
- 3B - 14B → `TierMid` → `small` prompts
- 14B - 70B → `TierAdvanced` → `large` prompts
- >= 70B → `TierFrontier` → `large` prompts

**Prompt Tiers:**
- `small`: Concise, focused prompts for smaller models
- `large`: Detailed, comprehensive prompts for larger models

## Usage

### Basic Usage

```go
import "github.com/normanking/cortex/internal/prompts"

// Load prompts store
store := prompts.Load()
provider := prompts.NewTemplateProvider(store)

// Get prompt for 7B model (small tier)
prompt := provider.GetSystemPrompt("terminal_error_diagnosis", 7_000_000_000)

// List available tasks
tasks := provider.ListTasks()
```

### Cognitive Integration

```go
import "github.com/normanking/cortex/internal/cognitive"

// Create PromptManager
pm := cognitive.NewPromptManager()

// Get optimized prompt
prompt := pm.GetOptimizedPrompt("command_suggestion", 7_000_000_000)

// Enrich template context
ctx := pm.EnrichTemplateContext("tui_troubleshooting", 14_000_000_000, map[string]interface{}{
    "error": "rendering issue",
})
// ctx now has: system_prompt, prompt_tier, model_tier
```

### Orchestrator Integration

```go
import (
    "github.com/normanking/cortex/internal/cognitive"
    "github.com/normanking/cortex/internal/orchestrator"
)

// Create PromptManager
pm := cognitive.NewPromptManager()

// Add to orchestrator
orch := orchestrator.New(
    orchestrator.WithPromptManager(pm),
    // ... other options
)
```

### Custom Prompts

```go
// Register custom prompt (overrides defaults)
provider.RegisterCustomPrompt("my_task", "small", "Custom prompt for small models")
provider.RegisterCustomPrompt("my_task", "large", "Custom prompt for large models")

// Remove custom prompt
provider.RemoveCustomPrompt("my_task", "small")

// Clear all custom prompts
provider.ClearCustomPrompts()
```

## Available Tasks

The following tasks are available in `optimized.yaml`:

1. **terminal_error_diagnosis** - Diagnose and fix terminal errors
2. **command_suggestion** - Suggest appropriate terminal commands
3. **code_explanation** - Explain code in plain English
4. **vision_code_analysis** - Analyze code from screenshots
5. **vision_error_extraction** - Extract errors from terminal screenshots
6. **tui_troubleshooting** - Debug Lipgloss/Bubbletea TUI issues
7. **tui_layout_design** - Design Lipgloss layouts
8. **tui_component_architecture** - Bubbletea component patterns
9. **agentic_tool_use** - Guide for agentic tool execution

## Adding New Prompts

Edit `static/optimized.yaml`:

```yaml
prompts:
  my_new_task:
    small: |
      Concise prompt for small models.
      Focus on essential instructions.
    large: |
      Detailed prompt for large models.
      Include comprehensive guidelines, examples, and edge cases.
```

Then rebuild:

```bash
go build ./internal/prompts/...
```

## Testing

```bash
# Run all tests
go test ./internal/prompts/... -v

# Run integration tests
go test ./internal/cognitive/... -run TestPrompt -v

# Run examples
go test ./internal/prompts/... -run Example -v
```

## Thread Safety

All components are thread-safe:
- `Store`: Read-only after Load()
- `TemplateProvider`: RWMutex for custom prompts
- `PromptManager`: Delegates to thread-safe provider

## Performance

- **Load time**: < 1ms (embedded YAML)
- **Lookup time**: O(1) hash map access
- **Memory**: ~50KB for default prompts
- **No network calls**: All prompts are embedded

## Future Enhancements

- [ ] Dynamic prompt loading from external sources
- [ ] Prompt versioning and A/B testing
- [ ] Prompt performance metrics
- [ ] LLM-based prompt optimization
- [ ] User feedback integration
