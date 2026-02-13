---
project: Cortex
component: Docs
phase: Ideation
date_created: 2026-02-06T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T11:39:28.811995
---

# Intelligent Model Picker Proposal for Cortex Development Swarm

**Date:** 2026-02-06
**Status:** Proposal
**Priority:** High

---

## Executive Summary

Current swarm configuration defaults to cloud models (Claude Sonnet via OpenRouter) despite having excellent local and alternative models available. This proposal introduces an **Intelligent Model Picker** that automatically selects the best model based on task type, optimizing for cost, speed, and capability while maximizing use of local and free alternatives.

---

## Current State Analysis

### 🔍 What's Configured

#### cortex-gateway-test Configuration

**Local Lane (Ollama on Pink - 192.168.1.186:11434):**
```yaml
models:
  - cortex-coder:latest      # 9GB - Fine-tuned, knows CortexBrain patterns
  - go-coder:latest          # 5GB - Go specialist
  - deepseek-coder-v2:latest # 9GB - General coding
```

**Cloud Lane (OpenRouter):**
```yaml
models:
  - anthropic/claude-3-5-sonnet-20241022  # DEFAULT (expensive!)
```

**Default:** `cloud` ❌ (Should be local!)

#### Available Alternative Models (Not Yet Configured)

**GLM 4.7 (Free, Unlimited):**
- Endpoint: `https://api.z.ai/api/coding/paas/v4`
- API Key: `ac9ce3331d0246d085698659b6d59971.jFVeWptw73HiTsJ1`
- Models: `glm-4.7` (reasoning), `glm-4.7-flash` (fast)
- Format: OpenAI-compatible
- **Cost:** FREE ✅
- **Best for:** Quick code generation, boilerplate, tests

**Kimi K2.5 (Free):**
- Endpoint: `https://api.moonshot.ai/v1`
- API Key: `sk-cp-4xE4HWjhHmDU4DdsIFDU8uyNCrTeXFHuqpgieAMoohm7XSEEeaNdgCnJnyJ7zns4ZRda69XN5i1aaWwHlxpoU32o-a67GYaLYWsfXcbBvt348hjxGQX6H8U`
- Model: `kimi-k2.5`
- **Cost:** FREE ✅
- **Best for:** Code review, architecture decisions, long context

**MiniMax (Not Yet Configured):**
- Endpoint: `https://api.minimax.chat/v1`
- API Key: (needs to be obtained)
- Models: `abab6.5-chat`, `abab6.5s-chat`
- **Cost:** Free tier available
- **Best for:** Conversational tasks, reasoning

#### CortexBrain Configuration

**Primary:** MLX (Qwen2.5-Coder-7B-Instruct-4bit)
**Fallbacks:** Ollama (llama3.2:3b), dnet, cloud (disabled)

---

## Problems with Current Setup

### ❌ Problem 1: Wrong Default Lane
```yaml
default_lane: cloud  # Uses expensive Claude Sonnet!
```

**Impact:**
- Every coding task goes to OpenRouter (costs money)
- Local models sitting idle
- Slower responses (network latency)
- Unnecessary API costs

### ❌ Problem 2: Free Models Not Configured
- GLM 4.7 (free, unlimited) not in config
- Kimi K2.5 (free) not in config
- MiniMax not explored

### ❌ Problem 3: No Task-Based Routing
All tasks use same model, regardless of:
- Complexity (simple boilerplate vs. complex algorithm)
- Language (Go vs. Python vs. Bash)
- Type (code generation vs. code review vs. debugging)
- Speed requirements (quick fix vs. architectural design)

### ❌ Problem 4: No Cost Optimization
No preference for:
1. Local models (free, fast)
2. Free cloud alternatives (GLM, Kimi)
3. Paid models only when necessary

---

## Proposed Solution: Intelligent Model Picker

### Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                     INTELLIGENT MODEL PICKER                         │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Incoming Request                                                    │
│        ↓                                                             │
│  ┌──────────────────────────────────────────────────────┐          │
│  │  1. TASK ANALYSIS                                    │          │
│  │  - Detect task type (code, review, debug, docs)     │          │
│  │  - Estimate complexity (simple/medium/complex)       │          │
│  │  - Identify language (Go, Python, Bash, etc.)       │          │
│  │  - Check context length requirement                  │          │
│  └──────────────────┬───────────────────────────────────┘          │
│                     ↓                                                │
│  ┌──────────────────────────────────────────────────────┐          │
│  │  2. MODEL SELECTION LOGIC                            │          │
│  │  Priority order:                                     │          │
│  │  1. Local models (cortex-coder, go-coder)           │          │
│  │  2. Free alternatives (GLM 4.7, Kimi K2.5)          │          │
│  │  3. Paid cloud (OpenRouter, only if needed)         │          │
│  └──────────────────┬───────────────────────────────────┘          │
│                     ↓                                                │
│  ┌──────────────────────────────────────────────────────┐          │
│  │  3. ROUTE TO MODEL                                   │          │
│  │  - Send request to selected model                   │          │
│  │  - Log selection decision                            │          │
│  │  - Track performance & cost                          │          │
│  └──────────────────┬───────────────────────────────────┘          │
│                     ↓                                                │
│  Response                                                            │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Model Selection Matrix

### Task Type → Model Mapping

| Task Type | Complexity | Best Model | Fallback | Rationale |
|-----------|-----------|------------|----------|-----------|
| **Go Code Generation** | Simple | `go-coder:latest` | `cortex-coder:latest` | Specialized Go model, fast |
| **Go Code Generation** | Medium | `cortex-coder:latest` | `glm-4.7` | Knows CortexBrain patterns |
| **Go Code Generation** | Complex | `glm-4.7` | `kimi-k2.5` | Free reasoning model |
| **Code Review** | Any | `kimi-k2.5` | `glm-4.7` | Free, good for analysis |
| **Debugging** | Simple | `go-coder:latest` | `cortex-coder:latest` | Fast local |
| **Debugging** | Complex | `glm-4.7` | `kimi-k2.5` | Reasoning capability |
| **Architecture Design** | Any | `kimi-k2.5` | `claude-sonnet` | Long context, only use paid if needed |
| **Boilerplate/Tests** | Any | `glm-4.7-flash` | `go-coder:latest` | Fast generation |
| **Documentation** | Any | `glm-4.7` | `cortex-coder:latest` | Good text generation |
| **Python Code** | Any | `deepseek-coder-v2:latest` | `glm-4.7` | General coding model |
| **Bash Scripts** | Any | `cortex-coder:latest` | `glm-4.7` | Knows system patterns |
| **Refactoring** | Any | `kimi-k2.5` | `cortex-coder:latest` | Analysis + generation |

### Complexity Detection Rules

**Simple (use fastest local models):**
- Code < 100 lines
- Single function changes
- Boilerplate generation
- Test writing
- Documentation updates

**Medium (use specialized local or free cloud):**
- Code 100-500 lines
- Multiple function changes
- API endpoint implementation
- Error handling logic

**Complex (use reasoning models, free alternatives first):**
- Code > 500 lines
- Architecture changes
- Algorithm implementation
- Performance optimization
- Security-critical code

---

## Implementation Plan

### Phase 1: Configuration Update (Week 1)

#### 1.1 Update cortex-gateway-test/config.yaml

```yaml
inference:
    auto_detect: true  # Enable intelligent picking
    lanes:
        # Local models (highest priority)
        - name: local-go
          provider: ollama
          base_url: http://192.168.1.186:11434
          models:
            - go-coder:latest
          task_types: [go-code-simple, go-code-medium]
          priority: 1

        - name: local-cortex
          provider: ollama
          base_url: http://192.168.1.186:11434
          models:
            - cortex-coder:latest
          task_types: [go-code-medium, bash, documentation]
          priority: 2

        - name: local-deepseek
          provider: ollama
          base_url: http://192.168.1.186:11434
          models:
            - deepseek-coder-v2:latest
          task_types: [python, javascript, general-code]
          priority: 3

        # Free cloud alternatives (second priority)
        - name: glm-reasoning
          provider: openai-compatible
          base_url: https://api.z.ai/api/coding/paas/v4
          api_key: ac9ce3331d0246d085698659b6d59971.jFVeWptw73HiTsJ1
          models:
            - glm-4.7
          task_types: [go-code-complex, debugging, refactoring]
          priority: 4
          cost: free

        - name: glm-fast
          provider: openai-compatible
          base_url: https://api.z.ai/api/coding/paas/v4
          api_key: ac9ce3331d0246d085698659b6d59971.jFVeWptw73HiTsJ1
          models:
            - glm-4.7-flash
          task_types: [boilerplate, tests, quick-fix]
          priority: 5
          cost: free

        - name: kimi-analysis
          provider: openai-compatible
          base_url: https://api.moonshot.ai/v1
          api_key: sk-cp-4xE4HWjhHmDU4DdsIFDU8uyNCrTeXFHuqpgieAMoohm7XSEEeaNdgCnJnyJ7zns4ZRda69XN5i1aaWwHlxpoU32o-a67GYaLYWsfXcbBvt348hjxGQX6H8U
          models:
            - kimi-k2.5
          task_types: [code-review, architecture, long-context]
          priority: 6
          cost: free

        # Paid cloud (last resort, lowest priority)
        - name: cloud-premium
          provider: openai-compatible
          base_url: https://openrouter.ai/api/v1
          api_key: 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
          models:
            - anthropic/claude-3-5-sonnet-20241022
          task_types: [critical, emergency]
          priority: 99
          cost: paid

    default_lane: local-go  # Start with fastest local

    # Model picker configuration
    picker:
        enabled: true
        strategy: cost-optimized  # Options: cost-optimized, speed-first, quality-first

        # Task detection
        task_detection:
            enabled: true
            keywords:
                go-code: ["func ", "package ", "import ", "type ", "struct "]
                python: ["def ", "import ", "class ", "from "]
                bash: ["#!/bin/bash", "#!/bin/sh"]
                test: ["func Test", "def test_", "it(", "describe("]
                review: ["review", "analyze", "check", "audit"]
                debug: ["debug", "error", "fix", "bug"]
                architecture: ["design", "architecture", "system", "diagram"]

        # Complexity heuristics
        complexity:
            simple_max_lines: 100
            medium_max_lines: 500
            context_length_threshold: 4000

        # Cost preferences
        cost_policy:
            prefer_free: true
            max_paid_per_hour: 10  # Max $10/hour on paid APIs
            alert_on_paid_use: true

        # Performance tracking
        tracking:
            enabled: true
            log_selections: true
            track_latency: true
            track_cost: true
```

#### 1.2 Add Picker Logic (Go Implementation)

**File:** `internal/picker/picker.go`

```go
package picker

import (
    "strings"
    "github.com/normanking/cortex-gateway/internal/config"
)

type TaskType string

const (
    TaskGoCodeSimple    TaskType = "go-code-simple"
    TaskGoCodeMedium    TaskType = "go-code-medium"
    TaskGoCodeComplex   TaskType = "go-code-complex"
    TaskPython          TaskType = "python"
    TaskBash            TaskType = "bash"
    TaskCodeReview      TaskType = "code-review"
    TaskDebug           TaskType = "debugging"
    TaskArchitecture    TaskType = "architecture"
    TaskBoilerplate     TaskType = "boilerplate"
    TaskTests           TaskType = "tests"
    TaskRefactoring     TaskType = "refactoring"
)

type Complexity int

const (
    ComplexitySimple  Complexity = 1
    ComplexityMedium  Complexity = 2
    ComplexityComplex Complexity = 3
)

type TaskAnalysis struct {
    Type       TaskType
    Complexity Complexity
    Language   string
    LineCount  int
}

func AnalyzeTask(prompt string) TaskAnalysis {
    analysis := TaskAnalysis{}

    // Detect language
    analysis.Language = detectLanguage(prompt)

    // Detect task type
    analysis.Type = detectTaskType(prompt)

    // Estimate complexity
    analysis.Complexity = estimateComplexity(prompt)
    analysis.LineCount = estimateLines(prompt)

    return analysis
}

func detectLanguage(prompt string) string {
    lower := strings.ToLower(prompt)

    if strings.Contains(lower, "func ") || strings.Contains(lower, "package ") {
        return "go"
    }
    if strings.Contains(lower, "def ") || strings.Contains(lower, "import ") {
        return "python"
    }
    if strings.Contains(lower, "#!/bin/bash") {
        return "bash"
    }

    return "unknown"
}

func detectTaskType(prompt string) TaskType {
    lower := strings.ToLower(prompt)

    // Check for specific task types
    if strings.Contains(lower, "review") || strings.Contains(lower, "analyze") {
        return TaskCodeReview
    }
    if strings.Contains(lower, "debug") || strings.Contains(lower, "fix bug") {
        return TaskDebug
    }
    if strings.Contains(lower, "architecture") || strings.Contains(lower, "design") {
        return TaskArchitecture
    }
    if strings.Contains(lower, "boilerplate") || strings.Contains(lower, "scaffold") {
        return TaskBoilerplate
    }
    if strings.Contains(lower, "test") || strings.Contains(lower, "unit test") {
        return TaskTests
    }
    if strings.Contains(lower, "refactor") {
        return TaskRefactoring
    }

    // Default to code generation
    return TaskGoCodeMedium
}

func estimateComplexity(prompt string) Complexity {
    lines := estimateLines(prompt)

    if lines < 100 {
        return ComplexitySimple
    }
    if lines < 500 {
        return ComplexityMedium
    }
    return ComplexityComplex
}

func estimateLines(prompt string) int {
    // Simple heuristic: count newlines + estimate from keywords
    lines := strings.Count(prompt, "\n")

    // Adjust based on prompt length
    if len(prompt) > 2000 {
        lines += 200
    } else if len(prompt) > 1000 {
        lines += 100
    } else if len(prompt) > 500 {
        lines += 50
    }

    return lines
}

func SelectBestModel(cfg *config.Config, analysis TaskAnalysis) (*config.Lane, string) {
    // Get all lanes sorted by priority
    lanes := getSortedLanes(cfg)

    // Find first lane that matches task type
    for _, lane := range lanes {
        if matchesTaskType(lane, analysis.Type) {
            return lane, lane.Models[0]
        }
    }

    // Fallback to default lane
    for _, lane := range lanes {
        if lane.Name == cfg.Inference.DefaultLane {
            return lane, lane.Models[0]
        }
    }

    // Last resort: first lane
    return &lanes[0], lanes[0].Models[0]
}

func matchesTaskType(lane *config.Lane, taskType TaskType) bool {
    for _, t := range lane.TaskTypes {
        if t == string(taskType) {
            return true
        }
    }
    return false
}

func getSortedLanes(cfg *config.Config) []config.Lane {
    // Sort by priority (lower number = higher priority)
    lanes := make([]config.Lane, len(cfg.Inference.Lanes))
    copy(lanes, cfg.Inference.Lanes)

    // Simple bubble sort by priority
    for i := 0; i < len(lanes); i++ {
        for j := i + 1; j < len(lanes); j++ {
            if lanes[j].Priority < lanes[i].Priority {
                lanes[i], lanes[j] = lanes[j], lanes[i]
            }
        }
    }

    return lanes
}
```

---

### Phase 2: Add MiniMax Support (Week 2)

**Steps:**
1. Obtain MiniMax API key
2. Test API endpoints
3. Add to configuration
4. Benchmark vs. GLM and Kimi

**Estimated cost:** Free tier should be sufficient

---

### Phase 3: Performance Tracking (Week 3)

**Metrics to track:**
- Model selection decisions (which model picked for which task)
- Latency per model
- Cost per model (track paid API usage)
- Success rate (did the generated code work?)
- User satisfaction (implicit via retry rate)

**Implementation:**
- Add metrics endpoint: `/api/v1/picker/metrics`
- Store in SQLite: `~/.cortex/picker-metrics.db`
- Dashboard in web UI

---

## Expected Benefits

### 💰 Cost Savings

**Current:** ~$5-10/day on Claude Sonnet via OpenRouter
**With Picker:** ~$0-2/day (mostly free models)

**Annual savings:** ~$1,800 - $3,600

### ⚡ Speed Improvements

| Task | Current (Cloud) | With Picker (Local/Free) | Improvement |
|------|----------------|--------------------------|-------------|
| Simple Go code | 3-5s | 0.5-1s | 3-5x faster |
| Code review | 5-8s | 1-2s | 2.5-4x faster |
| Boilerplate | 2-4s | 0.3-0.8s | 4-5x faster |

### 🎯 Quality Improvements

- **Specialized models** for specific tasks (go-coder for Go, deepseek for Python)
- **Context-aware** selection (architecture → long context models)
- **Failure recovery** (automatic fallback to next best model)

---

## Rollout Plan

### Week 1: Configuration & Testing
- Update config.yaml
- Test GLM 4.7 and Kimi K2.5 APIs
- Implement basic picker logic
- Test on Harold (swarm node)

### Week 2: Swarm Deployment
- Deploy to all swarm nodes (harold, pink, red, kentaro)
- Monitor selections and performance
- Gather metrics

### Week 3: Optimization
- Analyze metrics
- Tune task detection rules
- Adjust priority rankings
- Add MiniMax if beneficial

### Week 4: Production
- Enable cost tracking
- Set up alerts for paid API usage
- Document usage patterns
- Train team on override commands

---

## Override Commands

Users can override picker decisions:

```bash
# Force specific model
cortex ask --model go-coder "write function..."

# Force specific lane
cortex ask --lane glm-reasoning "debug this..."

# Force paid model (for critical tasks)
cortex ask --lane cloud-premium --force "urgent: fix production bug..."
```

---

## Monitoring & Alerts

### Alerts
- **Cost Alert:** Paid API usage > $10/hour
- **Performance Alert:** Model latency > 10s
- **Failure Alert:** All models in lane failing

### Dashboard Metrics
- Total requests per model
- Average latency per model
- Cost per model
- Success rate per model
- Task type distribution

---

## Risk Mitigation

### Risk 1: Free API Rate Limits

**Mitigation:**
- Monitor usage carefully
- Have paid fallback configured
- Cache common responses

### Risk 2: Model Quality Varies

**Mitigation:**
- Track success rates
- Gather user feedback
- A/B test model selections

### Risk 3: Complex Task Misclassification

**Mitigation:**
- Conservative complexity estimation
- Easy override mechanism
- Learn from user overrides

---

## Success Criteria

After 1 month:
- [ ] 80%+ requests served by local or free models
- [ ] Cost reduced by 70%+
- [ ] Average latency improved by 2x+
- [ ] Zero complaints about model quality
- [ ] < 5% override rate (indicates good automatic selection)

---

## Next Steps

1. **Review this proposal** - Get your approval
2. **Week 1 Implementation** - Update config, implement picker logic
3. **Testing** - Verify GLM and Kimi APIs work
4. **Deployment** - Roll out to swarm
5. **Monitoring** - Track metrics for 1 week
6. **Optimization** - Tune based on real usage

---

## Questions for You

1. **Priority:** Should I implement this proposal? (Yes/No/Modify)
2. **Timeline:** Is 4-week timeline acceptable, or faster/slower?
3. **MiniMax:** Should I obtain MiniMax API key, or skip it?
4. **Cost Budget:** Is $0-2/day acceptable for paid APIs, or strict $0?
5. **Override Frequency:** How often do you want to manually override picker?

---

**Let me know if you approve this proposal, and I'll start implementation immediately!**

**Last Updated:** 2026-02-06
**Estimated Implementation Time:** 4 weeks
**Estimated Cost Savings:** $150-300/month
