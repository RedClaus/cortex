---
project: Cortex
component: Unknown
phase: Archive
date_created: 2025-12-18T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:51.345090
---

# Decomposer Architecture

## Component Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                    COMPLEXITY SCORER                             │
├─────────────────────────────────────────────────────────────────┤
│  Input: User request string                                     │
│  Output: Score (0-100) + Complexity Level + Factors             │
│                                                                  │
│  Factors Analyzed:                                              │
│  • Token count (length)                                         │
│  • Step indicators ("then", "after", "next")                    │
│  • File references (paths, extensions)                          │
│  • Conditional language ("if", "when", "unless")                │
│  • Technical terms (database, API, kubernetes)                  │
│  • Question complexity ("why", "how", "explain")                │
│  • Task type multipliers (1.5x for infrastructure)              │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    DECOMPOSITION DECISION                        │
├─────────────────────────────────────────────────────────────────┤
│  ShouldDecompose(score, templateMatch)                          │
│                                                                  │
│  Rules:                                                          │
│  • Score > 70  →  Always decompose                              │
│  • Score 31-70 + weak template (<0.70)  →  Decompose            │
│  • Score < 30  →  Use template/single step                      │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    TASK DECOMPOSER                               │
├─────────────────────────────────────────────────────────────────┤
│  Input: Complex task + Task type                                │
│  Output: DecompositionResult with Steps                         │
│                                                                  │
│  Process:                                                        │
│  1. Analyze complexity                                          │
│  2. If simple → Return single-step workflow                     │
│  3. If complex → Query LLM with decomposition prompt            │
│  4. Parse JSON response (handles markdown wrapping)             │
│  5. Extract steps with dependencies                             │
│  6. Validate workflow structure                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    WORKFLOW BUILDER                              │
├─────────────────────────────────────────────────────────────────┤
│  Fluent API for manual workflow construction                    │
│                                                                  │
│  workflow := NewWorkflow("name").                               │
│      AddLLMStep(...).                                           │
│      AddToolStep(..., deps...).                                 │
│      AddApprovalStep(...).                                      │
│      Build()                                                     │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    WORKFLOW VALIDATOR                            │
├─────────────────────────────────────────────────────────────────┤
│  Validates workflow before execution:                           │
│  • No duplicate step IDs                                        │
│  • All dependencies reference valid steps                       │
│  • No circular dependencies (DFS cycle detection)               │
│  • No empty step IDs                                            │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    WORKFLOW EXECUTOR                             │
├─────────────────────────────────────────────────────────────────┤
│  Executes workflow with dependency resolution                   │
│                                                                  │
│  Features:                                                       │
│  • Dependency-aware execution order                             │
│  • Shared context between steps                                 │
│  • Step output propagation (stepID_output)                      │
│  • Progress callbacks                                            │
│  • Graceful failure handling                                    │
│  • Optional step support                                        │
│  • Context cancellation support                                 │
└─────────────────────────────────────────────────────────────────┘
```

## Step Type Execution

```
┌────────────────────────────────────────────────────────────────┐
│  STEP TYPE ROUTING                                             │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────┐     ┌──────────────────────────────────┐        │
│  │ StepLLM  │────►│ LLMProvider.Chat()               │        │
│  └──────────┘     │ • Variable substitution          │        │
│                   │ • Prompt construction             │        │
│                   └──────────────────────────────────┘        │
│                                                                 │
│  ┌──────────┐     ┌──────────────────────────────────┐        │
│  │ StepTool │────►│ ToolExecutor.ExecuteTool()       │        │
│  └──────────┘     │ • read, write, edit, bash, etc.  │        │
│                   │ • Context-aware execution         │        │
│                   └──────────────────────────────────┘        │
│                                                                 │
│  ┌─────────────┐  ┌──────────────────────────────────┐        │
│  │StepTemplate │─►│ TemplateExecutor.ExecuteTemplate()│       │
│  └─────────────┘  │ • Template rendering             │        │
│                   │ • Variable extraction             │        │
│                   └──────────────────────────────────┘        │
│                                                                 │
│  ┌─────────────┐  ┌──────────────────────────────────┐        │
│  │StepApproval │─►│ ApprovalHandler.RequestApproval()│        │
│  └─────────────┘  │ • User confirmation required     │        │
│                   │ • Pause until approved/rejected   │        │
│                   └──────────────────────────────────┘        │
└────────────────────────────────────────────────────────────────┘
```

## Dependency Resolution

```
Workflow Steps:
┌──────┐     ┌──────┐     ┌──────┐     ┌──────┐
│step1 │────►│step2 │────►│step4 │────►│step5 │
└──────┘     └──────┘     └──────┘     └──────┘
               │
               ▼
             ┌──────┐
             │step3 │
             └──────┘

Execution Order (Topological Sort):
step1 → step2 → step3 → step4 → step5
        └───────┘

Context Flow:
step1 produces → step1_output
step2 uses step1_output, produces → step2_output
step3 uses step1_output, produces → step3_output
step4 uses step2_output, produces → step4_output
step5 uses step4_output
```

## Integration with Cognitive Router

```
┌─────────────────────────────────────────────────────────────────┐
│  User Request: "Deploy microservices to Kubernetes"            │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│  SEMANTIC ROUTER                                                │
│  • Embed request                                                │
│  • Find best template match                                     │
│  • templateMatch.SimilarityScore = 0.55 (MEDIUM)                │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│  COMPLEXITY SCORER                                              │
│  • ScoreComplexity(request)                                     │
│  • TaskType: TaskInfrastructure (1.5x multiplier)               │
│  • Base score: 50 → Final: 75 (COMPLEX)                         │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│  ROUTING DECISION                                               │
│  • ShouldDecompose(75, 0.55) → TRUE                             │
│  • Decision: RouteNovel                                         │
│  • RecommendedTier: TierFrontier                                │
│  • Action: Decompose + Execute Workflow                         │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│  DECOMPOSER                                                     │
│  • Query frontier LLM for decomposition                         │
│  • Parse JSON response                                          │
│  • Validate workflow                                            │
│  • Returns: 5 steps with dependencies                           │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│  WORKFLOW EXECUTOR                                              │
│  • Execute steps in order                                       │
│  • Request approval for high-risk steps                         │
│  • Report progress via callbacks                                │
│  • Return final result                                          │
└─────────────────────────────────────────────────────────────────┘
```

## Data Structures

### ComplexityResult
```go
type ComplexityResult struct {
    Score       int              // 0-100
    Level       ComplexityLevel  // simple/medium/complex
    Factors     []string         // Reasons for score
    NeedsDecomp bool             // Decomposition recommended
}
```

### Step
```go
type Step struct {
    ID          string                 // Unique identifier
    Description string                 // Human-readable description
    Type        StepType               // tool/template/llm/approval
    Tool        string                 // Tool name (if StepTool)
    TemplateID  string                 // Template ID (if StepTemplate)
    Prompt      string                 // Prompt (if StepLLM)
    Variables   map[string]interface{} // Step variables
    DependsOn   []string               // Dependency step IDs
    Optional    bool                   // Continue on failure
    RiskLevel   string                 // low/medium/high
}
```

### Workflow
```go
type Workflow struct {
    ID            string                 // UUID
    Name          string                 // Workflow name
    Description   string                 // Description
    Steps         []Step                 // Ordered steps
    Context       map[string]interface{} // Shared context
    CreatedAt     time.Time              // Creation timestamp
    EstimatedTime string                 // Time estimate
}
```

### WorkflowResult
```go
type WorkflowResult struct {
    Success        bool           // Overall success
    StepResults    []StepResult   // Per-step results
    TotalSteps     int            // Total step count
    CompletedSteps int            // Completed count
    FailedSteps    int            // Failed count
    SkippedSteps   int            // Skipped count
    TotalDuration  time.Duration  // Total execution time
    FinalOutput    string         // Final result
}
```

## Performance Characteristics

| Operation | Time Complexity | Space Complexity | Actual Performance |
|-----------|----------------|------------------|-------------------|
| Complexity Scoring | O(n) | O(n) | ~11µs |
| Workflow Validation | O(V + E) | O(V) | <1ms for typical workflows |
| Dependency Check | O(E) | O(1) | Negligible |
| Step Execution | O(n) | O(n) | Varies by step type |

Where:
- n = input length
- V = number of steps (vertices)
- E = number of dependencies (edges)

## Error Handling Strategy

```
┌─────────────────────────────────────────────────────────────────┐
│  ERROR LEVEL                                                    │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  VALIDATION ERRORS (Before execution)                           │
│  • Circular dependencies → Reject workflow                      │
│  • Invalid step references → Reject workflow                    │
│  • Duplicate IDs → Reject workflow                              │
│                                                                  │
│  EXECUTION ERRORS (During execution)                            │
│  • Required step fails → Stop workflow, mark as failed          │
│  • Optional step fails → Continue, increment SkippedSteps       │
│  • User rejects approval → Stop workflow, mark as failed        │
│  • Context cancelled → Stop workflow, return partial results    │
│                                                                  │
│  PARSING ERRORS (LLM response)                                  │
│  • No JSON found → Fallback to single-step                      │
│  • Invalid JSON → Fallback to single-step                       │
│  • Empty steps array → Fallback to single-step                  │
└─────────────────────────────────────────────────────────────────┘
```

## Extension Points

The decomposer is designed for extensibility:

1. **Custom Step Types**: Add new step types by extending `StepType` enum
2. **Custom Complexity Factors**: Modify `Scorer` to add domain-specific factors
3. **Custom Executors**: Implement `ToolExecutor`, `TemplateExecutor` interfaces
4. **Custom Validation**: Add validation rules in `ValidateWorkflow`
5. **Custom Callbacks**: Use `StepCallback` for custom progress tracking

## Future Architecture Improvements

### Planned Features
- Parallel execution of independent steps
- Workflow templates library
- Cost estimation per step
- Retry/resume on failure
- Workflow versioning
- DAG visualization export

### Performance Optimizations
- Memoization of complexity scores
- Adaptive thresholds based on success rates
- ML-based complexity prediction
- Streaming workflow execution
