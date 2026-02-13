---
project: Cortex
component: Memory/Skills
phase: Implementation
date_created: 2026-02-11T00:00:00
date_updated: 2026-02-11T00:00:00
source: ServerProjectsMac
---

# Skill Distillation PRD

**Project:** CortexBrain
**Feature:** Skill Distillation Extension to Observational Memory
**Priority:** P2.5 (Integrated with Observational Memory)
**Status:** ✅ Implementation Complete (Phases 1-4), Pending Swarm Integration
**Author:** Codie
**Date:** 2026-02-11
**Extends:** [Observational Memory PRD](./OBSERVATIONAL-MEMORY-PRD.md)

---

## Implementation Summary

> **Completed 2026-02-11** - Core implementation ready for integration testing

### Files Created

| File | Description |
|------|-------------|
| `internal/cognitive/skill_types.go` | DynamicSkill, FailurePattern, SkillDistillationConfig types |
| `internal/cognitive/registry_dynamic.go` | Dynamic Skill Registry with SQL storage |
| `internal/cognitive/distillation/skill_distiller.go` | Skill Distillation Agent |
| `internal/memory/observational_types.go` | Three-tier memory types (Message, Observation, Reflection) |
| `internal/memory/observational.go` | ObservationalMemory coordinator |
| `internal/memory/observer_agent.go` | Observer Agent for message compression |
| `internal/memory/reflector_agent.go` | Reflector Agent for observation consolidation |
| `internal/memory/observational_store_sqlite.go` | SQLite storage implementation |
| `internal/memory/skill_bridge.go` | Memory-to-Skill bridge integration |
| `internal/memory/observational_test.go` | Unit tests |

---

## Executive Summary

Implement a **Skill Distillation** layer that extends Observational Memory to enable CortexBrain to **evolve** by automatically extracting reusable behavioral patterns from agent experiences. Inspired by [SkillRL](https://github.com/aiming-lab/SkillRL), this system transforms successful interaction trajectories into new skills and failed trajectories into explicit "what NOT to do" rules—without requiring actual reinforcement learning training.

**Key Innovation:** Evolution through **pattern extraction** (LLM-powered), not **weight training** (RL-powered). This fits CortexBrain's architecture as an orchestration layer that routes to LLMs rather than trains them.

**Decision:** EMBEDDED (not plugin) — skill evolution is core cognition, like synaptic plasticity in the biological brain.

**Storage Backend:** Extends Observational Memory's Memvid .mv2 files with dedicated skill/pattern frames.

---

## Problem Statement

### Current Pain Points

| Problem | Impact | Affected |
|---------|--------|----------|
| **Static Skills** | Skills only grow via manual configuration | All agents |
| **Lost Experience** | Successful strategies aren't captured for reuse | Swarm agents |
| **Repeated Mistakes** | Same failures happen across sessions | Harold, Albert, Codie |
| **No Cross-Agent Learning** | One agent's success doesn't help others | Multi-agent swarm |
| **Manual Skill Creation** | Adding skills requires human intervention | System maintenance |

### What SkillRL Teaches Us

[SkillRL](https://github.com/aiming-lab/SkillRL) demonstrates that agents can learn reusable behavioral patterns by:

1. **Distilling successful trajectories** → Strategic patterns
2. **Learning from failures** → Concise lessons
3. **Hierarchical organization** → General + Task-Specific skills
4. **Recursive evolution** → Skills improve over time

**Our Adaptation:** SkillRL uses RL training (weight updates). CortexBrain uses **pattern extraction via LLM** (no training). Same concepts, different mechanism.

### Business Case

- **Continuous improvement** without manual intervention
- **Cross-agent learning** — Codie's success becomes Harold's skill
- **Failure prevention** — Mistakes are learned once, avoided forever
- **Reduced skill maintenance** — System auto-discovers new skills
- **10-20% token compression** vs raw trajectory storage (per SkillRL)

---

## Goals & Success Metrics

### Primary Goals

1. Enable CortexBrain to automatically extract new skills from successful interactions
2. Build a "failure library" that prevents repeating mistakes
3. Implement hierarchical skill organization (General + Task-Specific)
4. Enable cross-agent skill sharing within the swarm
5. Achieve skill evolution without actual model training

### Success Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| Skills Auto-Generated | ≥ 5/week | Count of new skills added |
| Failure Patterns Captured | ≥ 10/week | Count of failure rules |
| Skill Reuse Rate | ≥ 30% | Times auto-generated skill used |
| Cross-Agent Adoption | ≥ 50% | Skills used by >1 agent |
| False Positive Rate | ≤ 10% | Invalid skills that need pruning |
| Pattern Extraction Time | < 30s | LLM processing latency |

---

## Technical Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        Skill Distillation System                             │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                  Observational Memory (Existing)                     │    │
│  │  Tier 1: Messages → Tier 2: Observations → Tier 3: Reflections      │    │
│  └────────────────────────────┬────────────────────────────────────────┘    │
│                               │                                              │
│                               ▼                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                    Distillation Agent (NEW)                          │    │
│  │  • Background goroutine                                              │    │
│  │  • Model: Gemini 2.5 Flash / DeepSeek                                │    │
│  │  • Analyzes reflections for extractable patterns                     │    │
│  │  • Classifies: success pattern vs failure lesson                     │    │
│  │  • Determines skill type: General vs Task-Specific                   │    │
│  └────────────────────────────┬────────────────────────────────────────┘    │
│                               │                                              │
│              ┌────────────────┴────────────────┐                            │
│              ▼                                 ▼                             │
│  ┌───────────────────────┐       ┌───────────────────────┐                  │
│  │   Success Patterns    │       │   Failure Lessons     │                  │
│  │   (New Skills)        │       │   (Anti-Patterns)     │                  │
│  │                       │       │                       │                  │
│  │  • Strategic patterns │       │  • What NOT to do     │                  │
│  │  • Reusable workflows │       │  • Error signatures   │                  │
│  │  • Best practices     │       │  • Recovery hints     │                  │
│  └───────────┬───────────┘       └───────────┬───────────┘                  │
│              │                               │                               │
│              ▼                               ▼                               │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                    Hierarchical Skill Library                        │    │
│  │                                                                      │    │
│  │  ┌─────────────────────────────────────────────────────────────┐    │    │
│  │  │  General Skills (L1)                                         │    │    │
│  │  │  • Universal patterns applicable to any task                 │    │    │
│  │  │  • e.g., "break complex tasks into subtasks"                 │    │    │
│  │  │  • e.g., "verify assumptions before proceeding"              │    │    │
│  │  └─────────────────────────────────────────────────────────────┘    │    │
│  │                                                                      │    │
│  │  ┌─────────────────────────────────────────────────────────────┐    │    │
│  │  │  Task-Specific Skills (L2)                                   │    │    │
│  │  │  • Category-level patterns                                   │    │    │
│  │  │  • e.g., "for Redis issues, check connection first"          │    │    │
│  │  │  • e.g., "for deployment failures, verify env vars"          │    │    │
│  │  └─────────────────────────────────────────────────────────────┘    │    │
│  │                                                                      │    │
│  │  ┌─────────────────────────────────────────────────────────────┐    │    │
│  │  │  Failure Library (Anti-Patterns)                             │    │    │
│  │  │  • What NOT to do                                            │    │    │
│  │  │  • Error signatures and recovery                             │    │    │
│  │  │  • e.g., "don't assume Redis is up without checking"         │    │    │
│  │  └─────────────────────────────────────────────────────────────┘    │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                               │                                              │
│                               ▼                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                    Dynamic Skill Registry                            │    │
│  │  • 43 static skills (from Octopus) + N auto-generated               │    │
│  │  • Hot-reload capability                                             │    │
│  │  • Version tracking                                                  │    │
│  │  • Usage statistics                                                  │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Component Design

#### 1. Distillation Agent

```go
// pkg/brain/memory/distillation.go

package memory

import (
    "context"
    "time"
)

// DistillationAgent extracts skills from observational memory.
type DistillationAgent struct {
    model        LLMClient
    refStore     *ReflectionStore
    skillStore   *SkillStore
    failureStore *FailureStore
    config       DistillationConfig
    running      bool
    stopCh       chan struct{}
}

// DistillationConfig configures the distillation agent.
type DistillationConfig struct {
    // Minimum reflections before distillation attempt
    MinReflections int `yaml:"min_reflections"` // Default: 5

    // Distillation interval
    Interval time.Duration `yaml:"interval"` // Default: 1h

    // Model for pattern extraction
    Model string `yaml:"model"` // Default: gemini-2.5-flash

    // Confidence threshold for skill creation
    ConfidenceThreshold float64 `yaml:"confidence_threshold"` // Default: 0.7

    // Enable cross-agent learning
    CrossAgentLearning bool `yaml:"cross_agent_learning"` // Default: true

    // Storage path for auto-generated skills
    SkillOutputDir string `yaml:"skill_output_dir"` // Default: ./config/skills/auto

    // Storage path for failure patterns
    FailureOutputDir string `yaml:"failure_output_dir"` // Default: ./config/failures
}

// DistillationPrompt is the system prompt for the Distillation agent.
const DistillationPrompt = `You are a Skill Distillation Agent. Your job is to extract reusable behavioral patterns from agent experiences.

Analyze the following reflections and identify:

1. **Success Patterns**: Strategies that led to successful outcomes
   - What approach was taken?
   - What made it effective?
   - Is this generalizable to other situations?
   - Confidence score (0.0-1.0)

2. **Failure Lessons**: Mistakes that should be avoided
   - What went wrong?
   - What was the root cause?
   - How to detect similar situations?
   - How to recover or prevent?

3. **Skill Classification**:
   - GENERAL: Applicable to any task (universal wisdom)
   - TASK_SPECIFIC: Applicable to a category (e.g., "deployment", "debugging", "redis")
   - AGENT_SPECIFIC: Only relevant to one agent's context (not shared)

Output format:
---
## Success Patterns

### Pattern: <pattern_name>
- Type: GENERAL | TASK_SPECIFIC
- Category: <if task-specific, e.g., "debugging", "deployment">
- Description: <what to do>
- When to Apply: <trigger conditions>
- Confidence: <0.0-1.0>
- Source Reflections: <reflection IDs>

## Failure Lessons

### Failure: <failure_name>
- Type: GENERAL | TASK_SPECIFIC
- Category: <if task-specific>
- Description: <what NOT to do>
- Error Signature: <how to detect this situation>
- Recovery: <how to recover if it happens>
- Prevention: <how to avoid it>
- Confidence: <0.0-1.0>
- Source Reflections: <reflection IDs>
---

Analyze the following reflections:`

// Start begins the background distillation loop.
func (d *DistillationAgent) Start(ctx context.Context) {
    d.running = true
    go d.distillationLoop(ctx)
}

// distillationLoop periodically analyzes reflections.
func (d *DistillationAgent) distillationLoop(ctx context.Context) {
    ticker := time.NewTicker(d.config.Interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-d.stopCh:
            return
        case <-ticker.C:
            d.distill(ctx)
        }
    }
}

// distill analyzes reflections and extracts patterns.
func (d *DistillationAgent) distill(ctx context.Context) error {
    // Get recent reflections not yet analyzed
    reflections, err := d.refStore.GetUnanalyzed(ctx, d.config.MinReflections)
    if err != nil || len(reflections) < d.config.MinReflections {
        return nil // Not enough data yet
    }

    // Call LLM to extract patterns
    result, err := d.extractPatterns(ctx, reflections)
    if err != nil {
        return err
    }

    // Store success patterns as skills
    for _, pattern := range result.SuccessPatterns {
        if pattern.Confidence >= d.config.ConfidenceThreshold {
            if err := d.createSkill(ctx, pattern); err != nil {
                continue // Log and continue
            }
        }
    }

    // Store failure lessons
    for _, failure := range result.FailureLessons {
        if failure.Confidence >= d.config.ConfidenceThreshold {
            if err := d.createFailurePattern(ctx, failure); err != nil {
                continue
            }
        }
    }

    // Mark reflections as analyzed
    return d.refStore.MarkAnalyzed(ctx, reflections)
}
```

#### 2. Skill Types and Storage

```go
// pkg/brain/skills/dynamic.go

package skills

import (
    "time"
)

// SkillType represents the skill hierarchy level.
type SkillType string

const (
    SkillTypeGeneral      SkillType = "GENERAL"       // L1: Universal patterns
    SkillTypeTaskSpecific SkillType = "TASK_SPECIFIC" // L2: Category-level
    SkillTypeAgentSpecific SkillType = "AGENT_SPECIFIC" // Not shared
)

// SkillSource indicates where the skill came from.
type SkillSource string

const (
    SkillSourceStatic      SkillSource = "STATIC"      // From config/skills/*.md
    SkillSourceDistilled   SkillSource = "DISTILLED"   // Auto-generated
    SkillSourceMerged      SkillSource = "MERGED"      // Combined from multiple
)

// DynamicSkill represents an auto-generated skill.
type DynamicSkill struct {
    ID          string      `json:"id" yaml:"id"`
    Name        string      `json:"name" yaml:"name"`
    Type        SkillType   `json:"type" yaml:"type"`
    Category    string      `json:"category,omitempty" yaml:"category,omitempty"` // For TASK_SPECIFIC
    Source      SkillSource `json:"source" yaml:"source"`

    // Content
    Description  string   `json:"description" yaml:"description"`
    WhenToApply  string   `json:"when_to_apply" yaml:"when_to_apply"`
    Steps        []string `json:"steps,omitempty" yaml:"steps,omitempty"`
    Examples     []string `json:"examples,omitempty" yaml:"examples,omitempty"`

    // Provenance
    SourceReflections []string  `json:"source_reflections" yaml:"source_reflections"`
    SourceAgents      []string  `json:"source_agents" yaml:"source_agents"`
    Confidence        float64   `json:"confidence" yaml:"confidence"`
    CreatedAt         time.Time `json:"created_at" yaml:"created_at"`
    UpdatedAt         time.Time `json:"updated_at" yaml:"updated_at"`

    // Usage tracking
    UsageCount int       `json:"usage_count" yaml:"usage_count"`
    LastUsed   time.Time `json:"last_used,omitempty" yaml:"last_used,omitempty"`
    Successes  int       `json:"successes" yaml:"successes"`
    Failures   int       `json:"failures" yaml:"failures"`

    // Version control
    Version    int    `json:"version" yaml:"version"`
    ParentID   string `json:"parent_id,omitempty" yaml:"parent_id,omitempty"` // If evolved from another
}

// FailurePattern represents a "what NOT to do" rule.
type FailurePattern struct {
    ID          string    `json:"id" yaml:"id"`
    Name        string    `json:"name" yaml:"name"`
    Type        SkillType `json:"type" yaml:"type"`
    Category    string    `json:"category,omitempty" yaml:"category,omitempty"`

    // Content
    Description    string `json:"description" yaml:"description"`
    ErrorSignature string `json:"error_signature" yaml:"error_signature"` // How to detect
    Recovery       string `json:"recovery" yaml:"recovery"`               // How to fix
    Prevention     string `json:"prevention" yaml:"prevention"`           // How to avoid

    // Provenance
    SourceReflections []string  `json:"source_reflections" yaml:"source_reflections"`
    SourceAgents      []string  `json:"source_agents" yaml:"source_agents"`
    Confidence        float64   `json:"confidence" yaml:"confidence"`
    CreatedAt         time.Time `json:"created_at" yaml:"created_at"`

    // Tracking
    TimesTriggered int `json:"times_triggered" yaml:"times_triggered"`
    TimesPrevented int `json:"times_prevented" yaml:"times_prevented"`
}
```

#### 3. Dynamic Skill Registry

```go
// pkg/brain/skills/registry_dynamic.go

package skills

import (
    "context"
    "os"
    "path/filepath"
    "sync"

    "gopkg.in/yaml.v3"
)

// DynamicRegistry extends the static skill registry with auto-generated skills.
type DynamicRegistry struct {
    staticSkills  map[string]*Skill        // From config/skills/*.md (43 skills)
    dynamicSkills map[string]*DynamicSkill // Auto-generated
    failures      map[string]*FailurePattern

    // Hot-reload support
    watchDir string
    mu       sync.RWMutex
}

// NewDynamicRegistry creates a registry that combines static and dynamic skills.
func NewDynamicRegistry(staticDir, dynamicDir string) (*DynamicRegistry, error) {
    r := &DynamicRegistry{
        staticSkills:  make(map[string]*Skill),
        dynamicSkills: make(map[string]*DynamicSkill),
        failures:      make(map[string]*FailurePattern),
        watchDir:      dynamicDir,
    }

    // Load static skills (existing 43)
    if err := r.loadStaticSkills(staticDir); err != nil {
        return nil, err
    }

    // Load dynamic skills (auto-generated)
    if err := r.loadDynamicSkills(dynamicDir); err != nil {
        return nil, err
    }

    return r, nil
}

// Match finds the best skill for a task, checking both static and dynamic.
func (r *DynamicRegistry) Match(ctx context.Context, task string) (*MatchResult, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()

    var candidates []ScoredSkill

    // Check static skills
    for _, skill := range r.staticSkills {
        score := r.scoreSkill(skill, task)
        if score > 0.3 {
            candidates = append(candidates, ScoredSkill{
                Skill: skill,
                Score: score,
            })
        }
    }

    // Check dynamic skills (potentially higher score for learned patterns)
    for _, skill := range r.dynamicSkills {
        score := r.scoreDynamicSkill(skill, task)
        if score > 0.3 {
            candidates = append(candidates, ScoredSkill{
                DynamicSkill: skill,
                Score:        score * 1.1, // Slight boost for learned skills
            })
        }
    }

    // Check for failure patterns (negative matching)
    failureWarnings := r.checkFailurePatterns(task)

    return &MatchResult{
        Candidates:      candidates,
        FailureWarnings: failureWarnings,
    }, nil
}

// AddSkill adds a new auto-generated skill.
func (r *DynamicRegistry) AddSkill(ctx context.Context, skill *DynamicSkill) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    // Check for duplicates/similar skills
    if existing := r.findSimilar(skill); existing != nil {
        // Merge instead of duplicate
        return r.mergeSkills(existing, skill)
    }

    // Save to file
    filename := filepath.Join(r.watchDir, skill.ID+".yaml")
    data, err := yaml.Marshal(skill)
    if err != nil {
        return err
    }

    if err := os.WriteFile(filename, data, 0644); err != nil {
        return err
    }

    r.dynamicSkills[skill.ID] = skill
    return nil
}

// AddFailurePattern adds a new failure pattern.
func (r *DynamicRegistry) AddFailurePattern(ctx context.Context, failure *FailurePattern) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    filename := filepath.Join(r.watchDir, "failures", failure.ID+".yaml")
    data, err := yaml.Marshal(failure)
    if err != nil {
        return err
    }

    if err := os.WriteFile(filename, data, 0644); err != nil {
        return err
    }

    r.failures[failure.ID] = failure
    return nil
}

// GetStats returns registry statistics.
func (r *DynamicRegistry) GetStats() RegistryStats {
    r.mu.RLock()
    defer r.mu.RUnlock()

    return RegistryStats{
        StaticSkills:    len(r.staticSkills),
        DynamicSkills:   len(r.dynamicSkills),
        FailurePatterns: len(r.failures),
        TotalSkills:     len(r.staticSkills) + len(r.dynamicSkills),
    }
}
```

#### 4. Cross-Agent Learning

```go
// pkg/brain/memory/cross_agent.go

package memory

import (
    "context"
)

// CrossAgentLearning enables skills to be shared across agents.
type CrossAgentLearning struct {
    storage *MemvidStorage
    config  CrossAgentConfig
}

// CrossAgentConfig configures cross-agent learning.
type CrossAgentConfig struct {
    // Minimum confidence for sharing
    ShareThreshold float64 `yaml:"share_threshold"` // Default: 0.8

    // Agents to share with
    ShareWith []string `yaml:"share_with"` // Default: all

    // Skill types to share
    ShareTypes []SkillType `yaml:"share_types"` // Default: [GENERAL, TASK_SPECIFIC]
}

// ShareSkill shares a skill from one agent to all others.
func (c *CrossAgentLearning) ShareSkill(ctx context.Context, skill *DynamicSkill) error {
    // Only share if confidence meets threshold
    if skill.Confidence < c.config.ShareThreshold {
        return nil
    }

    // Only share GENERAL and TASK_SPECIFIC skills
    if skill.Type == SkillTypeAgentSpecific {
        return nil
    }

    // Store in shared.mv2
    frame := map[string]interface{}{
        "type":         "shared_skill",
        "skill":        skill,
        "shared_by":    skill.SourceAgents,
        "timestamp":    time.Now().Unix(),
    }

    data, _ := json.Marshal(frame)
    return c.storage.agentStores["shared"].writer.Put(ctx, data)
}

// GetSharedSkills retrieves skills shared by other agents.
func (c *CrossAgentLearning) GetSharedSkills(ctx context.Context, agentID string) ([]*DynamicSkill, error) {
    // Query shared.mv2 for skills not created by this agent
    results, err := c.storage.SearchMemory(ctx, "shared", "type:shared_skill", 100)
    if err != nil {
        return nil, err
    }

    var skills []*DynamicSkill
    for _, r := range results {
        skill := r.Skill
        // Don't return skills this agent created
        if !contains(skill.SourceAgents, agentID) {
            skills = append(skills, skill)
        }
    }

    return skills, nil
}
```

#### 5. Memvid Integration

Skill Distillation extends the existing Observational Memory Memvid storage:

```go
// pkg/brain/memory/storage_memvid_skills.go

package memory

// Frame types in .mv2 files:
// - "observation" — Tier 2 compressed notes (existing)
// - "reflection" — Tier 3 patterns (existing)
// - "skill" — Auto-generated skill (NEW)
// - "failure" — Failure pattern (NEW)
// - "shared_skill" — Cross-agent shared skill (NEW)

// StoreDistilledSkill writes an auto-generated skill to the agent's .mv2 file.
func (m *MemvidStorage) StoreDistilledSkill(ctx context.Context, skill *DynamicSkill) error {
    store, err := m.getOrCreateStore(skill.SourceAgents[0])
    if err != nil {
        return err
    }

    frame := map[string]interface{}{
        "type":              "skill",
        "id":                skill.ID,
        "name":              skill.Name,
        "skill_type":        skill.Type,
        "category":          skill.Category,
        "description":       skill.Description,
        "when_to_apply":     skill.WhenToApply,
        "steps":             skill.Steps,
        "confidence":        skill.Confidence,
        "source_reflections": skill.SourceReflections,
        "timestamp":         skill.CreatedAt.Unix(),
    }

    data, _ := json.Marshal(frame)
    return store.writer.Put(ctx, data)
}

// StoreFailurePattern writes a failure pattern to the agent's .mv2 file.
func (m *MemvidStorage) StoreFailurePattern(ctx context.Context, failure *FailurePattern) error {
    store, err := m.getOrCreateStore(failure.SourceAgents[0])
    if err != nil {
        return err
    }

    frame := map[string]interface{}{
        "type":              "failure",
        "id":                failure.ID,
        "name":              failure.Name,
        "skill_type":        failure.Type,
        "category":          failure.Category,
        "description":       failure.Description,
        "error_signature":   failure.ErrorSignature,
        "recovery":          failure.Recovery,
        "prevention":        failure.Prevention,
        "confidence":        failure.Confidence,
        "source_reflections": failure.SourceReflections,
        "timestamp":         failure.CreatedAt.Unix(),
    }

    data, _ := json.Marshal(frame)
    return store.writer.Put(ctx, data)
}

// SearchSkills finds skills matching a query.
func (m *MemvidStorage) SearchSkills(ctx context.Context, resourceID, query string, limit int) ([]*DynamicSkill, error) {
    results, err := m.SearchMemory(ctx, resourceID, "type:skill "+query, limit)
    if err != nil {
        return nil, err
    }

    var skills []*DynamicSkill
    for _, r := range results {
        if r.FrameType == "skill" {
            skills = append(skills, r.AsSkill())
        }
    }

    return skills, nil
}
```

### File Layout

```
data/episodic/
├── albert.mv2       # Contains: observations, reflections, skills, failures
├── harold.mv2       # Contains: observations, reflections, skills, failures
├── codie.mv2        # Contains: observations, reflections, skills, failures
├── shared.mv2       # Cross-agent shared skills
└── system.mv2       # System-level patterns

config/skills/
├── static/          # 43 original Octopus skills (read-only)
│   ├── skill-debug.md
│   ├── skill-review.md
│   └── ...
├── auto/            # Auto-generated skills (YAML format)
│   ├── redis-connection-check.yaml
│   ├── deployment-env-verify.yaml
│   └── ...
└── failures/        # Failure patterns
    ├── assume-redis-up.yaml
    ├── skip-env-validation.yaml
    └── ...
```

---

## Skill Lifecycle

### 1. Extraction

```
Reflections (5+) → Distillation Agent → Pattern Analysis → Skill Candidate
```

### 2. Validation

```
Skill Candidate → Confidence Check (≥0.7) → Duplicate Check → Creation
```

### 3. Storage

```
New Skill → YAML file (config/skills/auto/) + Memvid frame (.mv2)
```

### 4. Usage Tracking

```
Skill Used → Increment UsageCount → Record Success/Failure → Update Confidence
```

### 5. Evolution

```
Low-performing Skill → Analysis → Merge/Update/Deprecate
```

### 6. Cross-Agent Sharing

```
High-confidence Skill (≥0.8) → shared.mv2 → Available to all agents
```

---

## Integration with Brain

```go
// pkg/brain/brain.go (additions)

type Brain struct {
    // ... existing fields ...

    // Observational Memory (P2)
    observationalMemory *memory.ObservationalMemory

    // Skill Distillation (P2.5)
    distillationAgent *memory.DistillationAgent
    dynamicRegistry   *skills.DynamicRegistry
}

// Process now includes dynamic skill matching.
func (b *Brain) Process(ctx context.Context, input Input) (*Output, error) {
    // Add message to observational memory
    if err := b.observationalMemory.AddMessage(ctx, input.ToMessage()); err != nil {
        b.log.Warn("failed to add message to observational memory", "error", err)
    }

    // Get memory context
    memoryContext, err := b.observationalMemory.GetContext(ctx, input.ThreadID, input.ResourceID)
    if err != nil {
        b.log.Warn("failed to get memory context", "error", err)
    }

    // Match skills (static + dynamic)
    skillMatch, err := b.dynamicRegistry.Match(ctx, input.Content)
    if err != nil {
        b.log.Warn("failed to match skills", "error", err)
    }

    // Include failure warnings in response
    if len(skillMatch.FailureWarnings) > 0 {
        input.FailureWarnings = skillMatch.FailureWarnings
    }

    // Include memory context in processing
    input.MemoryContext = memoryContext
    input.MatchedSkills = skillMatch.Candidates

    // ... rest of processing ...
}

// RecordOutcome records whether the skill usage was successful.
func (b *Brain) RecordOutcome(ctx context.Context, skillID string, success bool) error {
    return b.dynamicRegistry.RecordUsage(ctx, skillID, success)
}
```

---

## Swarm Integration

```javascript
// agents/lib/skill-distillation.js

const { ObservationalMemory } = require('./observational-memory');

class SkillDistillation {
    constructor(config = {}) {
        this.memory = new ObservationalMemory(config);
        this.config = {
            minReflections: config.minReflections || 5,
            confidenceThreshold: config.confidenceThreshold || 0.7,
            distillationModel: config.distillationModel || 'gemini-2.5-flash',
            skillOutputDir: config.skillOutputDir || './config/skills/auto',
            ...config
        };
    }

    async distill(agentId) {
        // Get unanalyzed reflections
        const reflections = await this.memory.getReflections(agentId, { analyzed: false });
        if (reflections.length < this.config.minReflections) {
            return { skills: [], failures: [] };
        }

        // Call LLM to extract patterns
        const result = await this.extractPatterns(reflections);

        // Store skills that meet threshold
        for (const pattern of result.successPatterns) {
            if (pattern.confidence >= this.config.confidenceThreshold) {
                await this.createSkill(agentId, pattern);
            }
        }

        // Store failure patterns
        for (const failure of result.failureLessons) {
            if (failure.confidence >= this.config.confidenceThreshold) {
                await this.createFailurePattern(agentId, failure);
            }
        }

        // Mark reflections as analyzed
        await this.memory.markReflectionsAnalyzed(reflections);

        return result;
    }

    async getMatchingSkills(task) {
        // Check static skills (43 Octopus skills)
        const staticMatches = await this.matchStaticSkills(task);

        // Check dynamic skills (auto-generated)
        const dynamicMatches = await this.matchDynamicSkills(task);

        // Check failure patterns (negative matching)
        const failureWarnings = await this.checkFailurePatterns(task);

        return {
            skills: [...staticMatches, ...dynamicMatches].sort((a, b) => b.score - a.score),
            warnings: failureWarnings
        };
    }

    async shareSkill(skill) {
        if (skill.confidence < 0.8) return; // Only share high-confidence
        if (skill.type === 'AGENT_SPECIFIC') return; // Don't share agent-specific

        // Store in shared.mv2
        await this.memory.storeSharedSkill(skill);
    }
}

module.exports = { SkillDistillation };
```

---

## Configuration

```yaml
# config/skill-distillation.yaml

skill_distillation:
  enabled: true

  # Distillation agent settings
  min_reflections: 5          # Minimum reflections before distillation
  interval: "1h"              # How often to run distillation
  model: "gemini-2.5-flash"   # Model for pattern extraction
  confidence_threshold: 0.7   # Minimum confidence for skill creation

  # Cross-agent learning
  cross_agent_learning:
    enabled: true
    share_threshold: 0.8      # Minimum confidence for sharing
    share_with: ["harold", "albert", "codie"]
    share_types: ["GENERAL", "TASK_SPECIFIC"]

  # Storage
  storage:
    skill_output_dir: "./config/skills/auto"
    failure_output_dir: "./config/skills/failures"

  # Skill lifecycle
  lifecycle:
    min_usage_for_evolution: 10   # Uses before considering evolution
    deprecation_threshold: 0.3     # Success rate below this → deprecate
    merge_similarity: 0.85         # Similarity threshold for merging

  # Limits
  max_skills_per_agent: 100
  max_failures_per_agent: 50
  max_shared_skills: 200
```

---

## Implementation Phases

> **Implementation Started:** 2026-02-11
> **Status:** In Progress

### Phase 1: Distillation Agent (Week 1) ✅ COMPLETE

**Deliverables:**
- [x] `internal/cognitive/distillation/skill_distiller.go` - Core distillation agent
- [x] `internal/cognitive/skill_types.go` - Dynamic skill types
- [x] Distillation prompt optimization (SkillDistillerSystemPrompt)
- [x] Cross-agent learning methods (ShareSkillCrossAgent, GetShareableSkills)

**Acceptance Criteria:**
- ✅ Reflections analyzed for patterns
- ✅ Skills extracted with confidence scores
- ✅ Failure patterns captured

### Phase 2: Dynamic Skill Registry (Week 1-2) ✅ COMPLETE

**Deliverables:**
- [x] `internal/cognitive/registry_dynamic.go` - Extended registry
- [x] JSON serialization for auto-generated skills
- [x] FTS5 search capability
- [x] Skill lifecycle management (probation → active → deprecated)

**Acceptance Criteria:**
- ✅ Static + dynamic skills queryable together (DynamicSkillRegistry embeds SQLiteRegistry)
- ✅ Skills stored in SQLite with FTS5 search
- ✅ Promotion/deprecation based on usage metrics

### Phase 3: Failure Library (Week 2) ✅ COMPLETE

**Deliverables:**
- [x] Failure pattern storage (FailurePattern type + SQL tables)
- [x] Negative matching in skill selection (SearchFailurePatterns)
- [x] Warning injection in matching (FailureWarning in SkillMatchResult)

**Acceptance Criteria:**
- ✅ Failure patterns captured from failed interactions
- ✅ Warnings surfaced when similar situations detected
- ✅ Recovery/prevention hints provided

### Phase 4: Cross-Agent Learning (Week 2-3) ✅ COMPLETE

**Deliverables:**
- [x] `internal/cognitive/distillation/skill_distiller.go` - Sharing logic
- [x] Skill adoption tracking (SourceAgents, ParentID)
- [x] Confidence reduction for shared context (0.8 multiplier)

**Acceptance Criteria:**
- ✅ High-confidence skills shared across agents (ShareThreshold: 0.8)
- ✅ Agents can query shared skill library
- ✅ Adoption tracked per agent

### Phase 5: Swarm Integration (Week 3) 🔄 PENDING

**Deliverables:**
- [ ] `agents/lib/skill-distillation.js` - Node.js integration
- [ ] Harold, Albert, Codie configured
- [ ] Dashboard updates for skill stats

**Acceptance Criteria:**
- Swarm agents use distilled skills
- Cross-agent learning working
- Metrics visible in dashboard

### Phase 6: Documentation & Monitoring (Week 3-4) 🔄 IN PROGRESS

**Deliverables:**
- [x] API documentation (this PRD)
- [ ] Prometheus metrics
- [ ] Dashboard for skill library

---

## Metrics & Monitoring

```
# Distillation metrics
cortex_distillation_runs_total{agent_id}
cortex_distillation_skills_created_total{agent_id, skill_type}
cortex_distillation_failures_captured_total{agent_id}
cortex_distillation_duration_seconds{agent_id}

# Skill usage metrics
cortex_skill_matches_total{skill_id, source}
cortex_skill_usage_total{skill_id}
cortex_skill_success_total{skill_id}
cortex_skill_failure_total{skill_id}

# Cross-agent metrics
cortex_shared_skills_total
cortex_skill_adoptions_total{skill_id, adopted_by}

# Registry metrics
cortex_registry_static_skills
cortex_registry_dynamic_skills
cortex_registry_failure_patterns
```

---

## Risks & Mitigations

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Low-quality skills generated | Medium | Medium | Confidence threshold, usage tracking |
| Skill explosion (too many) | Medium | Low | Max limits, merging, deprecation |
| False failure patterns | Low | Medium | Confidence threshold, manual review |
| Cross-agent contamination | Low | Medium | Agent-specific isolation option |
| Model hallucination | Medium | Medium | Source reflection verification |
| Storage growth | Low | Low | Memvid compression, TTL |

---

## Timeline

| Phase | Duration | Dependencies |
|-------|----------|--------------|
| Phase 1: Distillation Agent | 1 week | Observational Memory Phase 3 |
| Phase 2: Dynamic Registry | 1 week | Phase 1 |
| Phase 3: Failure Library | 0.5 week | Phase 1 |
| Phase 4: Cross-Agent Learning | 0.5 week | Phase 2 |
| Phase 5: Swarm Integration | 1 week | Phase 4 |
| Phase 6: Documentation | 0.5 week | Phase 5 |
| **Total** | **4.5 weeks** | After Observational Memory |

**Note:** Can run in parallel with Observational Memory Phases 4-5.

---

## Dependencies

### Required
- Observational Memory (P2) - Provides reflections to analyze
- Memvid storage - Extends .mv2 with skill frames
- vLLM Integration (P1) - For fast pattern extraction

### Optional
- Prometheus/Grafana - Metrics
- Swarm Dashboard - Visualization

---

## SkillRL Concept Mapping

| SkillRL Concept | CortexBrain Implementation |
|-----------------|----------------------------|
| Experience Distillation | Observational Memory → Reflections → Distillation Agent |
| Hierarchical Skills | GENERAL (L1) + TASK_SPECIFIC (L2) skill types |
| Success Patterns | DynamicSkill with steps and when_to_apply |
| Failure Lessons | FailurePattern with error_signature and prevention |
| Recursive Evolution | Usage tracking → confidence update → merge/deprecate |
| Token Compression | Reflections already compressed; skills are dense patterns |
| RL Training | **NOT USED** — LLM pattern extraction instead |

---

## Example Generated Skill

```yaml
# config/skills/auto/redis-connection-verify.yaml

id: redis-connection-verify
name: Redis Connection Verification
type: TASK_SPECIFIC
category: redis
source: DISTILLED

description: |
  Before performing any Redis operations, verify the connection is alive
  and the target stream/key exists.

when_to_apply: |
  - Starting any task involving Redis Streams
  - After infrastructure changes
  - When resuming work after a break

steps:
  - "Check Redis connectivity: redis-cli -h HOST ping"
  - "Verify target stream exists: redis-cli -h HOST EXISTS stream:name"
  - "If stream doesn't exist, decide: create or error"
  - "Test a simple read operation before complex queries"

examples:
  - "Before polling CCP messages, ping Redis first"
  - "After Harold restart, verify stream connectivity"

source_reflections:
  - "ref-harold-2026-02-07-001"
  - "ref-codie-2026-02-08-003"
source_agents:
  - "harold"
  - "codie"
confidence: 0.85
created_at: 2026-02-11T10:30:00Z
updated_at: 2026-02-11T10:30:00Z

usage_count: 0
successes: 0
failures: 0
version: 1
```

---

## Example Failure Pattern

```yaml
# config/skills/failures/assume-redis-available.yaml

id: assume-redis-available
name: Don't Assume Redis is Available
type: TASK_SPECIFIC
category: redis

description: |
  Never assume Redis is running and accessible. Always verify before operations.

error_signature: |
  - ECONNREFUSED on Redis port
  - "Redis connection timeout"
  - "Stream does not exist" after attempting read

recovery: |
  1. Check if Redis service is running: systemctl status redis
  2. Verify network connectivity: ping HOST
  3. Check firewall rules: iptables -L
  4. Restart if needed: systemctl restart redis

prevention: |
  - Always use redis-connection-verify skill before Redis operations
  - Implement health checks with retries
  - Add connection timeout handling

source_reflections:
  - "ref-harold-2026-02-07-failure-001"
source_agents:
  - "harold"
confidence: 0.9
created_at: 2026-02-11T10:35:00Z

times_triggered: 0
times_prevented: 0
```

---

## References

### Internal Documents
- [Observational Memory PRD](./OBSERVATIONAL-MEMORY-PRD.md) - Foundation for distillation
- [Memory Enhancement PRD](../../Cortex/Gateway/CortexBrain-Memory-Enhancement-PRD.md) - Memvid architecture
- [Octopus Integration PRD](./OCTOPUS-INTEGRATION-PRD.md) - Static skill baseline

### External Sources
- [SkillRL GitHub](https://github.com/aiming-lab/SkillRL) - Inspiration for hierarchical skill learning
- [State of RL for LLMs 2025](https://www.turingpost.com/p/stateofrl2025) - RLVR and alternatives
- [Continual Learning with RL](https://cameronrwolfe.substack.com/p/rl-continual-learning) - Background

---

## Approval

| Role | Name | Date | Status |
|------|------|------|--------|
| Author | Codie | 2026-02-11 | Created |
| Technical Review | - | - | Pending |
| Product Owner | Norman | - | Pending |
