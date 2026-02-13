---
project: Cortex
component: Unknown
phase: Ideation
date_created: 2026-02-04T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:29.739333
---

# Analysis: Porting pi-mono to Go

**Date:** 2026-02-04  
**Scope:** Full TypeScript → Go port assessment  
**Status:** NOT RECOMMENDED — Targeted adoption preferred

---

## Effort Assessment

| Package | TS Lines (est) | Go Port Effort | Value to Swarm |
|---------|---------------|----------------|----------------|
| **pi-tui** | ~5,000 | 4-6 weeks | Medium (TUI optimization) |
| **pi-agent-core** | ~8,000 | 6-8 weeks | Low (overlap with CortexBrain) |
| **pi-coding-agent** | ~10,000 | 8-10 weeks | Medium (coding harness) |
| **pi-ai** | ~3,000 | 2-3 weeks | Low (already have multi-model) |
| **pi-pods** | ~4,000 | 3-4 weeks | Low (use Ollama) |
| **pi-web-ui** | ~6,000 | 5-6 weeks | Low (React already used) |
| **pi-mom** | ~2,000 | 1-2 weeks | Low (Slack bot exists) |
| **TOTAL** | **~38,000** | **6-9 months** | — |

**Verdict:** Full port = 6-9 months of dedicated work. **Not recommended.**

---

## Strategic Analysis

### What pi-mono Provides vs. Swarm Has

| Feature | pi-mono | Swarm Current | Port Value |
|---------|---------|---------------|------------|
| **TUI framework** | Differential rendering | BubbleTea | Medium — could optimize |
| **Agent runtime** | State + tools | CortexBrain lobes | Low — already exists |
| **LLM routing** | Unified API | cortex-gateway | Low — already exists |
| **Coding agent** | Interactive harness | None | Medium — gap exists |
| **vLLM deploy** | Pod management | Ollama + Proxmox | Low — different approach |

### Key Insight
80% of pi-mono **overlaps** with existing swarm infrastructure. Porting would duplicate effort.

---

## Recommended Approach: Targeted Adoption

Instead of full port, extract specific concepts:

### 1. Differential Rendering for BubbleTea (2-3 weeks)
**File:** `cortex-tui/optimizations/diff_render.go`

```go
// Concept from pi-tui, implemented for BubbleTea
package diffrender

// Track last rendered state
type DifferentialRenderer struct {
    lastFrame []Line
    terminal  *Terminal
}

// Only update changed lines
func (d *DifferentialRenderer) Render(newFrame []Line) {
    for i, line := range newFrame {
        if i >= len(d.lastFrame) || d.lastFrame[i] != line {
            d.terminal.MoveCursor(i, 0)
            d.terminal.ClearLine()
            d.terminal.Write(line.Content)
        }
    }
    d.lastFrame = newFrame
}
```

**Value:** Reduce TUI flicker, improve performance  
**Effort:** 2-3 weeks (targeted) vs 4-6 weeks (full port)

### 2. CSI 2026 Synchronized Output (1 week)
**File:** `cortex-tui/optimizations/sync_output.go`

```go
// Atomic screen updates (no flicker)
const (
    StartSynchronizedUpdate = "\x1b[?2026h"
    EndSynchronizedUpdate   = "\x1b[?2026l"
)

func (t *Terminal) SyncUpdate(fn func()) {
    t.Write(StartSynchronizedUpdate)
    fn()
    t.Write(EndSynchronizedUpdate)
}
```

**Value:** Eliminates frame tearing in fast-updating TUIs  
**Effort:** 1 week

### 3. Pi Coding Agent Concepts → New Go Tool (4-6 weeks)
**Repo:** `cortex-coder-agent/` (new project)

**Features to port:**
- Interactive coding sessions
- Skills/prompt templates
- Extensions system
- Not full agent runtime (use CortexBrain)

```go
// cortex-coder-agent main.go
package main

// Lightweight coding harness that delegates to CortexBrain
// for heavy lifting, but provides pi-like interactivity

type CodingSession struct {
    BrainClient *cortex.Client
    Editor      *TUIEditor
    Skills      []Skill
}
```

**Value:** Fills "quick coding task" gap without replacing CortexBrain  
**Effort:** 4-6 weeks

### 4. Inline Terminal Images (2-3 weeks)
**File:** `cortex-monitor/terminal_images.go`

```go
// Kitty/iTerm2 graphics protocol support
package termimg

func RenderImage(data []byte, width, height int) string {
    // Kitty: \x1b_G...\x1b\\
    // iTerm2: \x1b]1337;File=...\x07
}
```

**Value:** Display images in Neural Monitor  
**Effort:** 2-3 weeks

---

## What NOT to Port

| Package | Reason |
|---------|--------|
| **pi-agent-core** | CortexBrain agent runtime superior |
| **pi-ai** | cortex-gateway already handles multi-model |
| **pi-pods** | Proxmox + Ollama infrastructure mature |
| **pi-web-ui** | React/Three.js monitor already built |
| **pi-mom** | Slack bot exists in cortex-gateway |

---

## Recommended Priority

| Item | Effort | Value | Priority |
|------|--------|-------|----------|
| CSI 2026 sync output | 1 week | High (flicker-free TUI) | P1 |
| Differential rendering | 2-3 weeks | Medium (performance) | P2 |
| Cortex-coder-agent | 4-6 weeks | Medium (new capability) | P3 |
| Terminal images | 2-3 weeks | Low (nice-to-have) | P4 |
| **Full pi-mono port** | 6-9 months | Low (duplication) | ❌ Skip |

**Total targeted effort:** 2-3 months vs 6-9 months for full port

---

## Alternative: Hybrid Architecture

Instead of porting, use pi-mono as external process:

```
CortexBrain (Go) ←→ pi-coding-agent (TypeScript via RPC)
                      ↓
               Handles quick coding tasks
               Reports results back to Brain
```

**Pros:**
- No porting effort
- Use pi as-is
- Clean separation

**Cons:**
- Another runtime (Node.js)
- IPC complexity
- Maintenance burden

**Verdict:** Only if pi-coding-agent fills critical gap

---

## Final Recommendation

**DO NOT port full pi-mono.**

**DO extract 2-3 high-value concepts:**
1. CSI 2026 sync output → immediate TUI improvement
2. Differential rendering → BubbleTea optimization
3. (Optional) Cortex-coder-agent → Go-based coding harness

**Total investment:** 2-3 months vs 6-9 months  
**Strategic value:** Higher (builds on existing, doesn't duplicate)

---

**Decision Needed:**
- Should I create a PRD for "BubbleTea Optimizations" (CSI 2026 + differential rendering)?
- Or prioritize the cortex-coder-agent as a new Go project?
- Or skip entirely and focus on WebAPI Harvester?
