---
date: YYYY-MM-DD
author: Norman
tags: [tag1, tag2, tag3]
status: active                    # active | archived | superseded
related_adr:                      # ADR number if this led to a decision
related_apps: []                  # Which apps this research applies to (pinky, gateway, avatar, etc.)
cortex_component:                 # Brain Kernel, AutoLLM, Neural Bus, Knowledge Fabric, etc.
superseded_by:                    # Path to newer research if superseded
---

# [Research Topic Title]

## Motivation

Why are we investigating this? What problem or opportunity triggered this research?

Link to the roadmap item, Cortex Request (CR-NNN), or GitHub issue that prompted this.

## Summary

Two to three paragraph executive summary of findings. Someone should be able to read this section alone and understand the key takeaway.

## Detailed Findings

### Background

What did we already know going in?

### Methodology

How did we investigate? What did we test, read, or prototype?

### Results

Present data, benchmarks, comparisons, or qualitative findings.

Use tables for structured comparisons:

| Criterion        | Option A       | Option B       | Option C       |
|-----------------|----------------|----------------|----------------|
| Performance     |                |                |                |
| Complexity      |                |                |                |
| Apple Silicon   |                |                |                |
| Memory Usage    |                |                |                |
| Go Compatibility|                |                |                |

### Key Observations

- Observation 1
- Observation 2
- Observation 3

## Implications for Cortex

### For Core (CortexBrain)

How does this affect the brain engine? Any cognitive lobe changes? AutoLLM routing impact?

### For Apps

Which plugins are impacted? How? (Pinky, cortex-coder-agent, CortexAvatar, etc.)

### For Roadmap

Does this change priorities? Should something move up or down in the Phase 1/2/3 timeline?

### First Principles Check

Does this research suggest anything that conflicts with CortexBrain's Tier-1 Inviolable Principles? (Single binary, Apple Silicon primary, local-first, Go only in core, <500MB memory)

## Recommendation

1. **Proceed** — Write an ADR and implement.
2. **Investigate further** — Specific follow-up questions to answer.
3. **Shelve** — Interesting but not actionable right now. Revisit on [date/trigger].

## References

- [Title](URL) — Brief annotation
- [Title](URL) — Brief annotation

## Follow-Up

- [ ] Action item 1
- [ ] Action item 2
- [ ] Write ADR if proceeding (link here when created)
