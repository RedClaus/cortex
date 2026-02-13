---
project: Cortex
component: Docs
phase: Archive
date_created: 2025-12-18T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:51.358054
---

# Decomposer Package

The `decomposer` package implements task complexity analysis and intelligent task decomposition for Cortex's Cognitive Architecture v2.1.

## Overview

This package provides three main components:

1. **Complexity Scorer** - Analyzes task complexity (0-100 scale)
2. **Task Decomposer** - Breaks complex tasks into manageable steps
3. **Workflow Engine** - Executes multi-step workflows with dependency resolution

## Complexity Scoring

### Scoring Factors

The complexity scorer analyzes multiple factors to calculate a score from 0-100:

| Factor | Weight | Max Points |
|--------|--------|------------|
| Token count | 0.1 per token | 20 |
| Step indicators ("then", "after", etc.) | 5 per indicator | 15 |
| File references | 3 per file | 15 |
| Conditional language ("if", "when", etc.) | 4 per conditional | 12 |
| Technical terms (database, API, etc.) | 5 per term | 20 |
| Question complexity ("why", "how", etc.) | 3 per question | 10 |

### Task Type Multipliers

Different task types have complexity multipliers:

```go
TaskGeneral:        1.0   // No multiplier
TaskExplain:        0.6   // Simpler explanations
TaskReview:         0.8   // Straightforward reviews
TaskCodeGen:        1.3   // More complex
TaskDebug:          1.2   // Requires investigation
TaskPlanning:       1.4   // Inherently complex
TaskRefactor:       1.5   // High complexity
TaskInfrastructure: 1.5   // High risk/complexity
```

### Scoring Rules

```
0-30:   Simple    - Single template, local model
31-60:  Medium    - May need mid-tier model
61-80:  Complex   - Decompose into steps
81-100: Very Complex - Frontier model or multi-step workflow
```

## Usage

### Basic Complexity Scoring

```go
import "github.com/normanking/cortex/internal/cognitive/decomposer"

// Simple scoring
score := decomposer.ScoreComplexity("Create a user login form")
// Returns: ~15 (simple)

// With task type
score := decomposer.ScoreComplexityWithType(
    "Refactor database schema",
    cognitive.TaskRefactor,
)
// Returns: ~45 * 1.5 = 67 (complex, due to refactor multiplier)
```

### Decomposition Decision

```go
score := decomposer.ScoreComplexity(input)
templateMatch := 0.65 // From semantic router

if decomposer.ShouldDecompose(score, templateMatch) {
    // Task needs decomposition
}

// With detailed reasoning
shouldDecomp, reason := decomposer.ShouldDecomposeDetailed(score, templateMatch)
fmt.Println(reason) // "high complexity score" or "medium complexity with weak template match"
```

### Full Decomposition Workflow

```go
// Create decomposer with LLM provider
llm := &MyLLMProvider{}
dec := decomposer.NewDecomposer(llm)

// Analyze task
ctx := context.Background()
input := "Build a microservices architecture with monitoring"
result, err := dec.Decompose(ctx, input, cognitive.TaskInfrastructure)

// Result contains:
// - result.Complexity: Complexity analysis
// - result.Steps: Decomposed steps
// - result.EstimatedTime: Estimated execution time
// - result.RequiresApproval: Whether approval is needed
```

## Workflow Building

### Manual Workflow Creation

```go
workflow := decomposer.NewWorkflow("Deploy Application").
    WithDescription("Deploy to production environment").
    WithEstimatedTime("15-20 minutes").
    AddLLMStep("Analyze requirements", "What are the deployment requirements?").
    AddToolStep("Run tests", "bash", "medium", "step1").
    AddApprovalStep("Approve production deploy", "step2").
    AddToolStep("Deploy", "kubectl", "high", "step3").
    Build()
```

### Workflow Validation

```go
if err := decomposer.ValidateWorkflow(workflow); err != nil {
    // Invalid workflow (circular deps, missing steps, etc.)
}
```

### Workflow Execution

```go
// Create executor with providers
executor := decomposer.NewEnhancedExecutor(decomposer.ExecutorConfig{
    LLM:         llmProvider,
    ToolExec:    toolExecutor,
    TemplateExec: templateExecutor,
    ApprovalHandler: approvalHandler,
})

// Execute with callback for progress tracking
ctx := context.Background()
result, err := executor.Execute(ctx, workflow, func(step *Step, result *StepResult) {
    fmt.Printf("Step %s: %s\n", step.ID, result.Output)
})

// Result contains:
// - result.Success: Overall success
// - result.StepResults: Results for each step
// - result.CompletedSteps: Number completed
// - result.FailedSteps: Number failed
// - result.TotalDuration: Total execution time
```

## Step Types

Workflows support four step types:

### 1. LLM Steps
Query an LLM for guidance or analysis.

```go
Step{
    Type:   StepLLM,
    Prompt: "Analyze the error logs and suggest fixes",
}
```

### 2. Tool Steps
Execute a tool (read, write, edit, bash, etc.).

```go
Step{
    Type:      StepTool,
    Tool:      "bash",
    Variables: map[string]interface{}{"command": "npm test"},
    RiskLevel: "medium",
}
```

### 3. Template Steps
Execute via the template engine.

```go
Step{
    Type:       StepTemplate,
    TemplateID: "code-review-template",
    Variables:  map[string]interface{}{"file": "main.go"},
}
```

### 4. Approval Steps
Request user approval before proceeding.

```go
Step{
    Type:        StepApproval,
    Description: "Approve deployment to production",
    RiskLevel:   "high",
}
```

## Workflow Features

### Dependency Resolution

Steps can depend on other steps:

```go
Step{
    ID:        "step2",
    DependsOn: []string{"step1"},
}
```

The executor ensures dependencies are satisfied before execution.

### Shared Context

Workflows maintain shared context between steps:

```go
workflow := decomposer.NewWorkflow("Example").
    WithContext(map[string]interface{}{
        "environment": "production",
        "version": "1.2.3",
    }).
    Build()
```

Step outputs are automatically added to context as `{stepID}_output`.

### Risk Levels

Steps can specify risk levels:

- `"low"` - Safe operations
- `"medium"` - Potentially impactful
- `"high"` - Dangerous operations (requires approval)

Workflows with any high-risk step automatically require approval.

### Optional Steps

Mark steps as optional to continue on failure:

```go
Step{
    Optional: true,
}
```

## LLM Decomposition Prompt

The decomposer uses this system prompt for LLM-based decomposition:

```
You are a task decomposition assistant. Break complex tasks into simple, executable steps.

Each step should be:
1. Atomic - does one thing
2. Clear - easily understood
3. Executable - can be performed with available tools

Specify for each step:
- description: What this step does
- type: "tool", "template", "llm", or "approval"
- tool: Which tool to use (if type is "tool")
- risk_level: "low", "medium", or "high"
- depends_on: Array of step IDs this depends on
```

## JSON Format

LLM responses are parsed from JSON (with automatic extraction from markdown):

```json
{
  "steps": [
    {
      "id": "step1",
      "description": "Analyze requirements",
      "type": "llm",
      "risk_level": "low"
    },
    {
      "id": "step2",
      "description": "Implement solution",
      "type": "tool",
      "tool": "edit",
      "risk_level": "medium",
      "depends_on": ["step1"]
    }
  ],
  "estimated_time": "10-15 minutes",
  "requires_approval": false
}
```

## Validation

The package includes comprehensive validation:

- ✅ Circular dependency detection
- ✅ Invalid step reference detection
- ✅ Duplicate step ID detection
- ✅ Empty step ID detection

```go
err := decomposer.ValidateWorkflow(workflow)
// Returns specific error messages for each validation failure
```

## Performance

Complexity scoring is highly optimized:

```
BenchmarkComplexityScoring-8   92827   11204 ns/op   8975 B/op   96 allocs/op
```

- **~11µs per score** on Apple M1
- **~9KB memory** per score
- Suitable for high-throughput routing

## Integration with Cognitive Router

```go
// In router logic:
score := decomposer.ScoreComplexity(userInput)
templateMatch := router.FindBestTemplate(userInput)

decision := router.RouteDecision{
    ComplexityScore: score,
}

if templateMatch.SimilarityScore >= cognitive.ThresholdHigh {
    // Use template directly
    decision.Decision = cognitive.RouteTemplate
} else if decomposer.ShouldDecompose(score, templateMatch.SimilarityScore) {
    // Decompose and execute workflow
    decision.Decision = cognitive.RouteNovel
    decision.RecommendedTier = cognitive.TierFrontier
} else {
    // Use mid-tier model
    decision.Decision = cognitive.RouteNovel
    decision.RecommendedTier = cognitive.TierMid
}
```

## Error Handling

The package provides detailed error messages:

```go
// Decomposition errors
err := dec.Decompose(ctx, input, taskType)
// "decomposition failed: LLM timeout"
// "no JSON found in response"
// "failed to parse JSON: invalid character"

// Validation errors
err := ValidateWorkflow(workflow)
// "circular dependency detected: step2 -> step1"
// "step foo references non-existent dependency: bar"
// "duplicate step ID: step1"

// Execution errors
result, err := executor.Execute(ctx, workflow, nil)
// "workflow cancelled: context deadline exceeded"
// "tool bash failed: command not found"
// "user rejected approval"
```

## Testing

The package includes comprehensive tests:

```bash
# Run all tests
go test ./internal/cognitive/decomposer/...

# Run specific test
go test -v ./internal/cognitive/decomposer/... -run TestDecomposition

# Run benchmarks
go test -bench=. ./internal/cognitive/decomposer/... -benchmem

# Run with coverage
go test -cover ./internal/cognitive/decomposer/...
```

## Future Enhancements

Planned improvements:

- [ ] Machine learning-based complexity scoring
- [ ] Adaptive thresholds based on success rate
- [ ] Parallel step execution (when dependencies allow)
- [ ] Workflow resume/retry on failure
- [ ] Workflow templates library
- [ ] Cost estimation per step
- [ ] Performance profiling per step
- [ ] Workflow visualization/DAG rendering

## License

Part of Cortex Cognitive Architecture v2.1
