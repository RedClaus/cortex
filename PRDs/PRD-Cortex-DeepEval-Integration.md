---
project: Cortex
component: Docs
phase: Ideation
date_created: 2026-02-04T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:29.769640
---

# PRD: DeepEval Model Benchmarking Integration

**Project:** Cortex Coder Agent (CCA)  
**Feature:** Model Evaluation & Ranking System  
**Status:** DRAFT  
**Created:** 2026-02-04  
**Author:** Albert (AI Partner)  
**Owner:** Norman King  

---

## 1. Executive Summary

Integrate the [DeepEval](https://github.com/confident-ai/deepeval) framework into Cortex Coder Agent to enable systematic, automated evaluation of LLM models. This will provide real-time rankings, per-task recommendations, and data-driven model selection.

### Key Objectives
- Benchmark all available models (Kimi, GLM, Grok, Ollama) against coding tasks
- Create custom metrics relevant to software engineering workflows
- Build a TUI dashboard for real-time model comparison
- Enable smart auto-selection of the best model per task type

---

## 2. Background & Motivation

### Current State
- Multiple models available via CortexBrain (Kimi-for-coding, GLM-4.7, Grok, Ollama models)
- No systematic way to compare performance
- Model selection is manual or based on assumptions
- No data on cost/performance trade-offs

### Problems
1. **Uncertainty:** Which model is best for Go refactoring vs Python debugging?
2. **Cost:** Am I using an expensive model when a cheaper one would suffice?
3. **Latency:** Which model is fastest for real-time coding assistance?
4. **Quality:** Does model X actually produce better code than model Y?

### Solution
DeepEval integration provides:
- Automated, repeatable benchmarking
- Quantifiable metrics for each model
- Historical tracking of performance
- Data-driven model selection

---

## 3. Requirements

### 3.1 Functional Requirements

#### FR1: Benchmark Execution
- **FR1.1:** Execute benchmarks via CLI: `coder benchmark --model <name> --dataset <path>`
- **FR1.2:** Support multiple benchmark datasets (coding tasks, refactoring, explanation)
- **FR1.3:** Run benchmarks against all configured models
- **FR1.4:** Parallel execution with rate limiting
- **FR1.5:** Resume interrupted benchmarks

#### FR2: Metrics Collection
- **FR2.1:** G-Eval scores for code quality (0-1 scale)
- **FR2.2:** Latency: time-to-first-token, tokens/second, total duration
- **FR2.3:** Cost: tokens in/out, estimated API cost
- **FR2.4:** Correctness: compile success, test pass rate
- **FR2.5:** Style: adherence to project conventions
- **FR2.6:** Hallucination detection for factual accuracy

#### FR3: Model Ranking
- **FR3.1:** Aggregate scores into composite rankings
- **FR3.2:** Per-task-type rankings (Go, Python, Rust, etc.)
- **FR3.3:** Configurable weights for metrics
- **FR3.4:** Trend analysis (improving/degrading over time)
- **FR3.5:** Statistical significance testing

#### FR4: Dashboard
- **FR4.1:** TUI panel showing model comparison table
- **FR4.2:** Sort by any metric (click column header)
- **FR4.3:** Filter by task type, date range, model provider
- **FR4.4:** Visual indicators for trends (↑↓)
- **FR4.5:** Detail view: per-model breakdown

#### FR5: Smart Selection
- **FR5.1:** Auto-select best model for current task
- **FR5.2:** Fallback chain if primary model fails
- **FR5.3:** A/B mode: compare two models side-by-side
- **FR5.4:** Override: manual model selection always possible

#### FR6: Reporting
- **FR6.1:** Generate HTML reports: `coder eval report --output report.html`
- **FR6.2:** Export to JSON/CSV for external analysis
- **FR6.3:** Share reports via URL or file
- **FR6.4:** Scheduled benchmarks (cron integration)

### 3.2 Non-Functional Requirements

| Requirement | Target | Notes |
|-------------|--------|-------|
| Benchmark speed | <5 min per 100 tasks | Parallel execution |
| Dashboard refresh | <2 seconds | Cached results |
| Storage | <100MB per 10K results | SQLite/JSON |
| Accuracy | ±5% confidence interval | Statistical rigor |
| Extensibility | New metric <1 day | Plugin architecture |

---

## 4. Architecture

### 4.1 System Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                    Cortex Coder Agent                        │
├─────────────────────────────────────────────────────────────┤
│  TUI Dashboard        │  Benchmark Runner    │  Rank Engine │
│  ├─ Model Table       │  ├─ Task Executor    │  ├─ Scoring  │
│  ├─ Metric Charts     │  ├─ DeepEval Bridge  │  ├─ Weighting│
│  └─ Detail View       │  └─ Result Store     │  └─ Trends   │
├─────────────────────────────────────────────────────────────┤
│  pkg/eval/                                                   │
│  ├─ runner.go         │  deep_eval.py        │  metrics/    │
│  ├─ dashboard.go      │  ├─ G-Eval           │  ├─ latency  │
│  ├─ ranking.go        │  ├─ RAGAS            │  ├─ cost     │
│  ├─ metrics.go        │  └─ Custom           │  └─ quality  │
│  └─ report.go         │                      │              │
├─────────────────────────────────────────────────────────────┤
│  Data                                                        │
│  ├─ datasets/         │  results.sqlite      │  reports/    │
│  │  ├─ go-tasks.yaml  │  ├─ benchmarks       │  ├─ html     │
│  │  ├─ python-tasks    │  ├─ rankings         │  ├─ json     │
│  │  └─ refactor-tasks  │  └─ trends           │  └─ csv      │
├─────────────────────────────────────────────────────────────┤
│  External: CortexBrain (Pink:18892)                          │
│  Models: Kimi, GLM, Grok, Ollama (go-coder, deepseek-coder) │
└─────────────────────────────────────────────────────────────┘
```

### 4.2 Component Design

#### 4.2.1 Benchmark Runner (`pkg/eval/runner.go`)
```go
type BenchmarkRunner struct {
    Dataset     Dataset
    Models      []string
    Parallelism int
    Timeout     time.Duration
}

func (r *BenchmarkRunner) Run() (*BenchmarkResult, error)
func (r *BenchmarkRunner) RunModel(model string) (*ModelResult, error)
```

#### 4.2.2 DeepEval Bridge (`deep_eval.py`)
- Python subprocess wrapper
- JSON-RPC communication
- Metric plugins (G-Eval, RAGAS, custom)

#### 4.2.3 Dashboard (`pkg/eval/dashboard.go`)
- BubbleTea component
- Table view with sorting
- Filters and search
- Real-time updates via WebSocket

#### 4.2.4 Rank Engine (`pkg/eval/ranking.go`)
```go
type RankEngine struct {
    Weights map[string]float64 // metric -> weight
}

func (e *RankEngine) Rank(results []ModelResult) []RankedModel
func (e *RankEngine) BestForTask(taskType string) string
```

---

## 5. Data Model

### 5.1 Benchmark Task
```yaml
# datasets/go-refactor.yaml
tasks:
  - id: go-refactor-001
    type: refactoring
    language: go
    input: |
      func process(data []string) []string {
          result := []string{}
          for i := 0; i < len(data); i++ {
              result = append(result, strings.ToUpper(data[i]))
          }
          return result
      }
    criteria:
      - Use range loop instead of index
      - Preallocate slice with capacity
    expected_patterns:
      - "range"
      - "make([]string, 0, len"
```

### 5.2 Benchmark Result
```json
{
  "benchmark_id": "bench-20260204-001",
  "timestamp": "2026-02-04T15:00:00Z",
  "model": "deepseek-coder-v2:latest",
  "task_id": "go-refactor-001",
  "metrics": {
    "g_eval_score": 0.92,
    "latency_ms": 2450,
    "tokens_in": 150,
    "tokens_out": 80,
    "cost_usd": 0.0012,
    "compile_success": true,
    "test_pass_rate": 1.0
  },
  "raw_output": "..."
}
```

### 5.3 Model Ranking
```json
{
  "ranking_id": "rank-20260204",
  "task_type": "go-refactoring",
  "weights": {
    "g_eval_score": 0.4,
    "latency_ms": 0.2,
    "cost_usd": 0.2,
    "test_pass_rate": 0.2
  },
  "rankings": [
    {
      "rank": 1,
      "model": "deepseek-coder-v2:latest",
      "composite_score": 0.89,
      "metrics": { ... },
      "trend": "stable"
    }
  ]
}
```

---

## 6. User Interface

### 6.1 CLI Commands

```bash
# Run benchmark on specific model
coder benchmark --model deepseek-coder-v2:latest --dataset datasets/go-tasks.yaml

# Benchmark all models
coder benchmark --all --dataset datasets/

# View rankings
coder eval rankings --task-type go --sort-by composite

# Compare two models
coder eval compare --model-a go-coder --model-b deepseek-coder-v2

# Generate report
coder eval report --output report.html --format html

# Auto-select best model for current file
coder eval recommend --file main.go
```

### 6.2 TUI Dashboard

```
┌──────────────────────────────────────────────────────────────────────┐
│ 🤖 Model Rankings — Go Refactoring                                   │
├────────────┬──────────┬────────┬─────────┬───────────┬─────────────┤
│ Model      │ Score    │ Latency│ Cost    │ Quality   │ Trend       │
├────────────┼──────────┼────────┼─────────┼───────────┼─────────────┤
│ deepseek.. │ 0.92 ⭐  │ 2.4s   │ $0.0012 │ 0.95      │ ↑ improving │
│ go-coder   │ 0.87     │ 1.2s   │ $0.0005 │ 0.88      │ → stable    │
│ glm-4.7    │ 0.84     │ 3.1s   │ $0.0020 │ 0.90      │ ↓ degrading │
│ kimi-code  │ 0.81     │ 1.8s   │ $0.0015 │ 0.85      │ → stable    │
└────────────┴──────────┴────────┴─────────┴───────────┴─────────────┘
│ [F]ilter [S]ort [R]efresh [A]uto-select [D]etail [Q]uit            │
└──────────────────────────────────────────────────────────────────────┘
```

### 6.3 Detail View

```
┌──────────────────────────────────────────────────────────────────────┐
│ Model: deepseek-coder-v2:latest                                      │
├──────────────────────────────────────────────────────────────────────┤
│ Provider: Ollama              │ Type: Local                         │
│ Last Benchmark: 2 hours ago   │ Status: ✅ Available                │
├──────────────────────────────────────────────────────────────────────┤
│ Metrics Summary                                                      │
│ ├─ G-Eval Score: 0.92 (excellent)                                    │
│ ├─ Latency: 2.4s avg ( acceptable)                                   │
│ ├─ Cost: $0.0012/task (very low)                                     │
│ ├─ Compile Success: 98%                                              │
│ └─ Test Pass Rate: 95%                                               │
├──────────────────────────────────────────────────────────────────────┤
│ Per-Task Breakdown                                                   │
│ ├─ Refactoring: 0.94 ⭐                                              │
│ ├─ Explanation: 0.91                                                 │
│ ├─ Bug Fixing: 0.89                                                  │
│ └─ Generation: 0.93                                                  │
└──────────────────────────────────────────────────────────────────────┘
```

---

## 7. Implementation Plan

### Phase 6, Week 9 (2 weeks)

| Day | Task | Owner | Deliverable |
|-----|------|-------|-------------|
| 1-2 | DeepEval setup, Python bridge | TBD | `pkg/eval/` skeleton, `deep_eval.py` |
| 3-4 | Metrics implementation | TBD | Latency, cost, quality metrics |
| 5-6 | Benchmark runner | TBD | `runner.go`, dataset loader |
| 7-8 | Rank engine + dashboard | TBD | `ranking.go`, dashboard TUI |
| 9-10 | Smart selection, reporting | TBD | Auto-select, HTML reports |

---

## 8. Dependencies

### External
- **DeepEval:** `pip install deepeval`
- **Python:** 3.8+ (for DeepEval bridge)
- **Datasets:** Curated coding tasks (YAML format)

### Internal
- CortexBrain API (Pink:18892)
- All models configured and available
- Existing TUI framework (BubbleTea)

---

## 9. Risks & Mitigations

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| DeepEval Python dep | Medium | High | Containerize, optional feature |
| Benchmark cost | Medium | Medium | Rate limiting, small datasets |
| Flaky evaluations | High | Medium | Multiple runs, confidence intervals |
| Model API changes | Low | Low | Abstraction layer in CortexBrain |
| Storage growth | Low | Low | Auto-archive old results |

---

## 10. Success Criteria

- [ ] Can benchmark 5+ models against 50+ tasks
- [ ] Dashboard renders rankings in <2 seconds
- [ ] Auto-selection improves perceived quality (user survey)
- [ ] Report generation works (HTML export)
- [ ] All metrics within ±5% confidence

---

## 11. Future Enhancements (Post-MVP)

- **v6.1:** Custom metric plugins (user-defined)
- **v6.2:** Distributed benchmarking (swarm workers)
- **v6.3:** Online learning (adapt weights based on feedback)
- **v6.4:** Community datasets (share benchmarks)
- **v6.5:** Predictive cost estimation before sending prompts

---

## 12. Related Work

- **DeepEval:** https://github.com/confident-ai/deepeval
- **HELM:** https://crfm.stanford.edu/helm/
- **OpenLLM Leaderboard:** https://huggingface.co/spaces/open-llm-leaderboard/open_llm_leaderboard
- **Cortex Coder Agent PRD:** `Cortex/PRDs/PRD-Cortex-Coder-Agent.md`

---

*Last Updated: 2026-02-04*  
*Status: DRAFT — PENDING REVIEW*
