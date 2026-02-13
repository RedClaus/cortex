---
project: Cortex
component: Docs
phase: Archive
date_created: 2025-12-18T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:51.640073
---

# autollm - Intelligent Model Router

The `autollm` package implements a two-lane model routing system for Cortex, automatically selecting the best model for each request.

## Overview

The AutoLLM router uses a **Fast/Smart lane** architecture:

- **Fast Lane** (default): Local Ollama → Groq → Cheap Cloud
- **Smart Lane** (frontier): Claude Opus 4, GPT-4o, etc.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     AutoLLM Router                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Request ────► Phase 1 ────► Phase 2 ────► Phase 3              │
│                   │            │            │                   │
│            Hard Constraints  User Intent  Default               │
│                   │            │            │                   │
│            Vision needed?   --strong?    Fast Lane              │
│            Context overflow?                                    │
│                   │            │            │                   │
│                   ▼            ▼            ▼                   │
│              Smart Lane    Smart Lane   Fast Lane               │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│  Fast Lane:   llama3:8b → groq/llama → gpt-4o-mini             │
│  Smart Lane:  claude-opus-4 → gpt-4o → gemini-pro               │
└─────────────────────────────────────────────────────────────────┘
```

## Components

### Router (`router.go`)

The main routing logic with three-phase algorithm:

```go
router := autollm.NewRouter(config, availability)

req := autollm.Request{
    Prompt:          "Explain quantum computing",
    Images:          nil,        // Vision requirement
    EstimatedTokens: 1000,       // Context size
    Mode:            autollm.LaneFast, // or LaneSmart
    LocalOnly:       false,
}

decision := router.Route(req)
// decision.Model = "llama3:8b"
// decision.Lane = LaneFast
// decision.Reason = "default fast lane"
```

### Three-Phase Routing

**Phase 1: Hard Constraints (Physics)**
- Cannot override - you can't send images to non-vision models
- Checks vision requirements against fast lane capabilities
- Checks context length against model limits

**Phase 2: User Intent (Agency)**
- Respects explicit `--strong` flag for Smart lane
- Respects `--local` flag for local-only routing

**Phase 3: Default Fast Lane**
- Priority: Local → Groq → Cheap Cloud
- Selects first available model in priority order

### Availability Checker (`availability.go`)

Monitors which models are currently available:

```go
availability := autollm.NewAvailabilityChecker(providers)

// Check if specific model is available
if availability.IsAvailable("llama3:8b", "ollama") {
    // Model is ready
}

// Refresh availability cache
availability.Refresh(ctx)
```

### Configuration (`config.go`)

Router configuration with model lists:

```go
config := autollm.RouterConfig{
    FastModels: []string{
        "llama3:8b",
        "groq/llama-3.1-70b",
        "gpt-4o-mini",
    },
    SmartModels: []string{
        "claude-opus-4-20250514",
        "gpt-4o",
        "gemini-1.5-pro",
    },
    DefaultSmartModel: "claude-sonnet-4-20250514",
}
```

## Types

### Request

```go
type Request struct {
    Prompt          string   // User prompt
    SystemPrompt    string   // System instructions
    Messages        []Message // Conversation history
    Images          []Image  // Vision inputs
    EstimatedTokens int      // Estimated context size
    Mode            Lane     // LaneFast or LaneSmart
    LocalOnly       bool     // Restrict to local models
}
```

### RoutingDecision

```go
type RoutingDecision struct {
    Model           string           // Selected model
    Lane            Lane             // Fast or Smart
    Provider        string           // Provider name
    Reason          string           // Why this model was selected
    Forced          bool             // Was this forced by constraints?
    Constraint      string           // What constraint forced it
    ModelCapability *eval.ModelCapability // Full capability info
}
```

### Lane

```go
type Lane string

const (
    LaneFast  Lane = "fast"  // Local/cheap models
    LaneSmart Lane = "smart" // Frontier models
)
```

## Provider Detection

The router auto-detects providers from model names:

- `claude-*` → Anthropic
- `gpt-*`, `o1-*` → OpenAI
- `gemini-*` → Gemini
- `groq/*` → Groq
- `model:tag` → Ollama (colon indicates local model)
- Default → Ollama (assume local for unknown)

## Integration

The router integrates with the eval package for capability scoring:

```go
// Get capability info for routing decision
cap := router.GetModelCapability("claude-sonnet-4")
fmt.Printf("Vision: %v, Context: %d\n",
    cap.Capabilities.Vision,
    cap.ContextWindow)
```

## Status

Get current router status for debugging:

```go
status := router.Status()
// {
//   "available_fast": ["llama3:8b"],
//   "available_smart": ["claude-sonnet-4"],
//   "fast_count": 1,
//   "smart_count": 1,
// }
```

## Usage Notes

1. **Vision requests** automatically route to Smart lane if no Fast model supports vision
2. **Context overflow** routes to Smart lane if Fast models can't handle the token count
3. **Local-only mode** restricts to Ollama models only
4. **Unavailable models** are automatically skipped in priority order
