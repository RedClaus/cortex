---
project: Cortex
component: Agents
phase: Ideation
date_created: 2026-02-04T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:29.679955
---

# Coding Guidelines for LLM Agents

**Document ID:** AGENT-CODING-GUIDELINES  
**Version:** 1.0  
**Date:** 2026-02-04  
**Applies To:** All coding agents (Albert, Harold, Pink, Red, sub-agents)  
**Tradeoff:** These guidelines bias toward caution over speed. For trivial tasks, use judgment.

---

## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- **State your assumptions explicitly.** If uncertain, ask.
- **If multiple interpretations exist, present them** — don't pick silently.
- **If something is unclear, stop.** Name what's confusing and ask.

### Checklist
- [ ] I understand the exact requirement
- [ ] I've identified any ambiguities
- [ ] I've stated my assumptions
- [ ] I've asked about anything unclear

---

## 2. Simplicity First

**Write the minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" unless requested.
- No error handling for impossible scenarios.
- **If you wrote 200 lines and it could be 50, rewrite it.**

### Self-Test
Ask yourself: *"Would a senior engineer say this is overcomplicated?"*

If yes, **simplify.**

---

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- **Match the existing style,** even if you'd do it differently.
- If you notice unrelated dead code, mention it — don't delete it.

### Cleanup Rules
When your changes create orphans:
- ✅ **DO** remove imports, variables, or functions that your changes made unused.
- ❌ **DON'T** remove pre-existing dead code unless asked.

### The Test
**Every changed line should trace directly to the user's request.**

---

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

### Examples

| Request | Success Criteria |
|---------|------------------|
| "Add validation" | Write tests for invalid inputs, then make them pass |
| "Fix the bug" | Write a test that reproduces it, then make it pass |
| "Refactor X" | Ensure tests pass before and after |

### Multi-Step Tasks
State a brief plan:
```
Step 1: …
Verify: …

Step 2: …
Verify: …
```

### Why This Matters
- **Strong success criteria** let you work independently
- **Weak criteria** ("make it better") lead to drift

---

## Quick Reference Card

```
┌─────────────────────────────────────────────────────────────┐
│  BEFORE CODING                                              │
│  □ State assumptions                                        │
│  □ Ask if unclear                                           │
│  □ Define success criteria                                  │
├─────────────────────────────────────────────────────────────┤
│  WHILE CODING                                               │
│  □ Minimum code to solve problem                            │
│  □ No speculative features                                  │
│  □ Match existing style                                     │
│  □ Only touch what's needed                                 │
├─────────────────────────────────────────────────────────────┤
│  AFTER CODING                                               │
│  □ Verify against success criteria                          │
│  □ Clean up your orphans (imports, vars, funcs)             │
│  □ Don't touch unrelated code                               │
└─────────────────────────────────────────────────────────────┘
```

---

## Exceptions

These guidelines bias toward **caution over speed.**

**Use judgment for:**
- Trivial one-liners
- Emergency fixes
- Prototypes/experiments (clearly marked)

**Always apply for:**
- Production code
- Shared libraries
- Code reviewed by others
- Complex refactors

---

## Agent Acknowledgment

All coding agents must acknowledge these guidelines:

| Agent | Status | Date |
|-------|--------|------|
| Albert (main) | ✅ Acknowledged | 2026-02-04 |
| Harold | ⏳ Pending | — |
| Pink | ⏳ Pending | — |
| Red | ⏳ Pending | — |

---

## Related Documents

- `AGENTS.md` — Agent responsibilities
- `MEMORY.md` — Long-term memory
- Individual project PRDs — Specific requirements

---

*These guidelines reduce common LLM coding mistakes: over-engineering, scope creep, style inconsistency, and unclear completion criteria.*
