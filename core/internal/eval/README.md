---
project: Cortex
component: Docs
phase: Archive
date_created: 2025-12-18T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:51.683089
---

# eval - Model Capability Evaluation

The `eval` package provides model capability scoring, conversation logging, and performance assessment for Cortex.

## Overview

This package implements a unified scoring system (0-100) for all LLM providers, with registry-based lookups for known models and heuristic fallbacks for unknown models.

## Components

### Capability Scorer (`scorer.go`)

The `CapabilityScorer` provides model scoring:

```go
scorer := eval.NewCapabilityScorer()

// Get full capability info
cap := scorer.GetCapabilities("anthropic", "claude-sonnet-4")
fmt.Printf("Score: %d, Tier: %s\n", cap.Score.Overall, cap.Tier)

// Just get the score
score := scorer.Score("ollama", "llama3:8b")
```

### Model Registry (`registry.go`, `registry_data.go`)

Static registry of 50+ model definitions with curated scores:

- **Anthropic**: claude-opus-4, claude-sonnet-4, claude-3-haiku, etc.
- **OpenAI**: gpt-4o, gpt-4o-mini, o1, o1-mini, etc.
- **Gemini**: gemini-1.5-pro, gemini-1.5-flash, gemini-2.0, etc.
- **Ollama/Local**: llama3, mistral, codellama, qwen, deepseek, etc.

### Conversation Logger (`logger.go`)

Logs all LLM interactions for analysis:

```go
logger, _ := eval.NewConversationLogger(db)

log := &eval.ConversationLog{
    Provider:   "ollama",
    Model:      "llama3:8b",
    Prompt:     "How do I...",
    DurationMs: 2340,
    Success:    true,
}
logger.Log(ctx, log)
```

### Performance Assessor (`assessor.go`)

Detects capability issues and recommends upgrades:

```go
assessor := eval.NewAssessor(recommender)

assessment := assessor.Assess(log)
if assessment.NeedsUpgrade() {
    fmt.Printf("Consider: %s (%s)\n",
        assessment.RecommendedUpgrade,
        assessment.UpgradeReason)
}
```

### Model Recommender (`recommender.go`)

Suggests model upgrades based on detected issues:

```go
recommender := eval.NewModelRecommender()
recommender.SetAvailableModels(models)

recommendation := recommender.Recommend(currentProvider, currentModel, issues)
```

## Types

### CapabilityScore

```go
type CapabilityScore struct {
    Overall     int     // 0-100 composite score
    Reasoning   int     // Logic/analysis capability
    Coding      int     // Code generation quality
    Instruction int     // Following directions
    Speed       int     // Relative speed (inverse latency)
    Confidence  float64 // 0-1 score confidence
    Source      string  // "registry" or "heuristic"
}
```

### ModelCapability

```go
type ModelCapability struct {
    ID            string          // e.g., "anthropic/claude-sonnet-4"
    Provider      string          // e.g., "anthropic"
    Model         string          // e.g., "claude-sonnet-4"
    DisplayName   string          // Human-readable name
    Tier          ModelTier       // small/medium/large/xl/frontier
    Score         CapabilityScore // Capability scores
    Capabilities  CapabilityFlags // Vision, FunctionCalling, etc.
    Pricing       *PricingInfo    // Cost per 1M tokens
    ContextWindow int             // Max context tokens
}
```

### Score Tiers

| Score | Tier | Examples |
|-------|------|----------|
| 90-100 | Frontier | claude-opus-4, o1-preview |
| 76-89 | XL | llama3:70b, gpt-4o |
| 56-75 | Large | qwen2.5-coder:14b |
| 36-55 | Medium | llama3:8b, mistral:7b |
| 0-35 | Small | llama3.2:1b |

## Heuristic Scoring

Unknown models get estimated scores based on:

1. **Parameter count** extracted from model name (e.g., "7b" → 7B params)
2. **Tier estimation** using `ClassifyModelTier()`
3. **Family bonuses** (coder models +10 coding, reasoning models +15)
4. **Quantization penalties** (q4 → -12, q8 → -3)

Heuristic scores have 0.5 confidence vs 0.95 for registry scores.

## Issue Detection

The assessor detects:

- **Timeout** - Response >30 seconds
- **Repetition** - Model stuck in loops
- **Tool Failure** - Invalid JSON, malformed tool calls
- **Truncation** - Incomplete responses
- **JSON Error** - Malformed structured output

## Usage in TUI

The TUI displays scores in model selection menus:

```
Llama 3.2 3B     🟢[45] Fast, lightweight
Claude Sonnet 4  🔴[92] Balanced performance
```

Use `/score` command to view detailed capability info.
