---
project: Cortex
component: Docs
phase: Ideation
date_created: 2026-02-08T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-08T22:26:44.418627
---

# PRD: EffGen Intelligence Port to CortexBrain

**Document ID:** CR-025
**Created:** 2026-02-08
**Status:** Draft
**Priority:** P1 - High
**Author:** Claude Code
**Source:** https://github.com/ctrl-gaurav/effGen

---

## Executive Summary

This PRD outlines the strategic port of key intelligence components from the effGen Python framework to CortexBrain's Go codebase. The primary goal is to enhance CortexBrain's cognitive capabilities with sophisticated task complexity analysis, intelligent routing, and advanced orchestration patterns—while preserving CortexBrain's architectural strengths (20 cognitive lobes, A2A protocol, Go performance).

### Business Value

| Metric | Current State | Target State | Impact |
|--------|---------------|--------------|--------|
| AutoLLM Accuracy | ~60% correct lane selection | 90%+ correct routing | Reduced API costs, faster responses |
| Task Decomposition | Manual lobe selection | Automatic subtask generation | Better cognitive utilization |
| Orchestration Patterns | 2 (sequential, parallel) | 6 patterns | More flexible agent coordination |
| Complexity Detection | Keyword + length heuristics | 5-metric weighted scoring | Smarter task classification |

---

## Problem Statement

### Current Limitations

1. **Primitive AutoLLM Routing**
   - Current implementation uses simple keyword matching and message length
   - Three hardcoded thresholds (0.3, 0.7) with no learning
   - Misroutes ~40% of tasks to wrong inference lane

2. **Static Lobe Selection**
   - PhaseExecutor uses predefined ThinkingStrategy
   - No dynamic adaptation based on task complexity
   - Cognitive lobes underutilized for simple tasks

3. **Limited Orchestration**
   - Only sequential and parallel execution patterns
   - No hierarchical, collaborative, or competitive strategies
   - Single-brain architecture limits scaling

4. **No Task Decomposition**
   - Complex requests processed as monolithic units
   - No automatic subtask generation
   - Dependencies between operations not modeled

### Opportunity

The effGen framework demonstrates a mature approach to these problems with:
- 5-metric complexity scoring system
- Intelligent routing with strategy selection
- 6 orchestration patterns
- LLM-assisted task decomposition with dependency management

---

## Goals & Non-Goals

### Goals

1. **Port ComplexityAnalyzer** - Implement 5-metric weighted scoring in Go
2. **Enhance AutoLLM Router** - Use complexity scores for lane selection
3. **Add Task Decomposition** - Generate subtasks with dependency tracking
4. **Expand Orchestration** - Implement 6 coordination patterns
5. **Integrate with Cognitive Lobes** - Map complexity to lobe activation

### Non-Goals

- Full effGen framework rewrite (Python → Go)
- Replacing BubbleTea TUI
- Changing A2A protocol
- Modifying 20-lobe cognitive architecture
- Adding effGen's tool system (we have our own)

---

## Technical Specification

### Component 1: ComplexityAnalyzer

**Location:** `pkg/complexity/analyzer.go`

#### Metrics (Ported from effGen)

| Metric | Weight | Range | Detection Method |
|--------|--------|-------|------------------|
| Task Length | 15% | 0-10 | Word count scaling |
| Requirement Count | 25% | 0-10 | Questions, bullets, clauses, semicolons |
| Domain Breadth | 20% | 0-10 | 8 knowledge domains detected |
| Tool Requirements | 20% | 0-10 | Capability identification |
| Reasoning Depth | 20% | 0-10 | Cognitive verb classification |

#### Data Structures

```go
// pkg/complexity/analyzer.go

package complexity

// ComplexityScore represents a multi-dimensional task analysis
type ComplexityScore struct {
    Overall          float64            `json:"overall"`           // 0-10 weighted score
    TaskLength       float64            `json:"task_length"`       // 0-10
    RequirementCount float64            `json:"requirement_count"` // 0-10
    DomainBreadth    float64            `json:"domain_breadth"`    // 0-10
    ToolRequirements float64            `json:"tool_requirements"` // 0-10
    ReasoningDepth   float64            `json:"reasoning_depth"`   // 0-10
    Domains          []string           `json:"domains"`           // Detected domains
    RequiredTools    []string           `json:"required_tools"`    // Suggested tools
    ReasoningLevel   ReasoningLevel     `json:"reasoning_level"`   // Simple/Moderate/Complex/VeryComplex
    Confidence       float64            `json:"confidence"`        // 0-1 analysis confidence
}

// ReasoningLevel classifies cognitive complexity
type ReasoningLevel int

const (
    ReasoningSimple ReasoningLevel = iota      // list, define, identify
    ReasoningModerate                           // explain, describe, compare
    ReasoningComplex                            // analyze, evaluate, design
    ReasoningVeryComplex                        // synthesize, architect, optimize
)

// Domain represents a knowledge area
type Domain string

const (
    DomainTechnical   Domain = "technical"
    DomainResearch    Domain = "research"
    DomainBusiness    Domain = "business"
    DomainCreative    Domain = "creative"
    DomainData        Domain = "data"
    DomainScientific  Domain = "scientific"
    DomainLegal       Domain = "legal"
    DomainFinancial   Domain = "financial"
)

// Analyzer performs complexity analysis on tasks
type Analyzer struct {
    weights           MetricWeights
    domainKeywords    map[Domain][]string
    toolKeywords      map[string][]string
    reasoningKeywords map[ReasoningLevel][]string
}

// MetricWeights configures the scoring weights
type MetricWeights struct {
    TaskLength       float64 `yaml:"task_length"`
    RequirementCount float64 `yaml:"requirement_count"`
    DomainBreadth    float64 `yaml:"domain_breadth"`
    ToolRequirements float64 `yaml:"tool_requirements"`
    ReasoningDepth   float64 `yaml:"reasoning_depth"`
}

// DefaultWeights returns the effGen-derived weights
func DefaultWeights() MetricWeights {
    return MetricWeights{
        TaskLength:       0.15,
        RequirementCount: 0.25,
        DomainBreadth:    0.20,
        ToolRequirements: 0.20,
        ReasoningDepth:   0.20,
    }
}
```

#### API

```go
// NewAnalyzer creates a complexity analyzer with default settings
func NewAnalyzer() *Analyzer

// NewAnalyzerWithWeights creates an analyzer with custom weights
func NewAnalyzerWithWeights(weights MetricWeights) *Analyzer

// Analyze performs full complexity analysis on input text
func (a *Analyzer) Analyze(ctx context.Context, input string) (*ComplexityScore, error)

// QuickScore returns just the overall score (0-10) for routing decisions
func (a *Analyzer) QuickScore(input string) float64

// SuggestLane recommends an inference lane based on complexity
func (a *Analyzer) SuggestLane(score *ComplexityScore) string
```

#### Scoring Algorithm

```go
func (a *Analyzer) Analyze(ctx context.Context, input string) (*ComplexityScore, error) {
    score := &ComplexityScore{}

    // 1. Task Length (0-10)
    wordCount := len(strings.Fields(input))
    switch {
    case wordCount < 20:
        score.TaskLength = 2.0
    case wordCount < 50:
        score.TaskLength = 4.0
    case wordCount < 100:
        score.TaskLength = 6.0
    case wordCount < 200:
        score.TaskLength = 8.0
    default:
        score.TaskLength = 10.0
    }

    // 2. Requirement Count (0-10)
    requirements := a.countRequirements(input)
    score.RequirementCount = min(float64(requirements)*1.5, 10.0)

    // 3. Domain Breadth (0-10)
    domains := a.detectDomains(input)
    score.Domains = domains
    score.DomainBreadth = min(float64(len(domains))*2.5, 10.0)

    // 4. Tool Requirements (0-10)
    tools := a.detectToolRequirements(input)
    score.RequiredTools = tools
    score.ToolRequirements = min(float64(len(tools))*2.0, 10.0)

    // 5. Reasoning Depth (0-10)
    level := a.classifyReasoningLevel(input)
    score.ReasoningLevel = level
    score.ReasoningDepth = float64(level+1) * 2.5

    // Calculate weighted overall score
    score.Overall = a.calculateWeightedScore(score)
    score.Confidence = a.calculateConfidence(score)

    return score, nil
}

func (a *Analyzer) countRequirements(input string) int {
    count := 0
    // Count question marks
    count += strings.Count(input, "?")
    // Count "and" clauses (but not "and the", "and a")
    count += len(regexp.MustCompile(`\band\s+[a-z]+[^the|a]\b`).FindAllString(input, -1))
    // Count numbered items (1., 2., etc.)
    count += len(regexp.MustCompile(`\d+\.\s`).FindAllString(input, -1))
    // Count bullet points
    count += len(regexp.MustCompile(`^[\-\*]\s`).FindAllString(input, -1))
    // Count semicolons (separate clauses)
    count += strings.Count(input, ";")
    return count
}
```

#### Lane Mapping

```go
func (a *Analyzer) SuggestLane(score *ComplexityScore) string {
    switch {
    case score.Overall < 3.0:
        return "local"   // Simple tasks → local Ollama
    case score.Overall < 5.0:
        return "fast"    // Moderate tasks → Groq (fast cloud)
    case score.Overall < 7.0:
        return "fast"    // Complex but not extreme → fast cloud
    case score.Overall < 9.0:
        return "smart"   // Very complex → Claude/GPT-4
    default:
        return "smart"   // Extreme complexity → best model
    }
}
```

---

### Component 2: TaskRouter

**Location:** `pkg/routing/router.go`

#### Strategy Selection

```go
// pkg/routing/router.go

package routing

import "github.com/normanking/cortex/pkg/complexity"

// RoutingStrategy determines how a task should be executed
type RoutingStrategy string

const (
    StrategySingleAgent RoutingStrategy = "single"      // One agent handles all
    StrategyParallel    RoutingStrategy = "parallel"    // Independent subtasks
    StrategySequential  RoutingStrategy = "sequential"  // Dependent chain
    StrategyHierarchical RoutingStrategy = "hierarchical" // Manager + workers
    StrategyCollaborative RoutingStrategy = "collaborative" // Multi-round consensus
    StrategyCompetitive RoutingStrategy = "competitive"  // Best solution wins
    StrategyPipeline    RoutingStrategy = "pipeline"     // Specialized stages
)

// RoutingDecision contains the router's recommendation
type RoutingDecision struct {
    Strategy        RoutingStrategy           `json:"strategy"`
    UseSubAgents    bool                      `json:"use_sub_agents"`
    Subtasks        []Subtask                 `json:"subtasks,omitempty"`
    Specializations []string                  `json:"specializations,omitempty"`
    InferenceLane   string                    `json:"inference_lane"`
    Confidence      float64                   `json:"confidence"`
    Reasoning       string                    `json:"reasoning"`
}

// Subtask represents a decomposed unit of work
type Subtask struct {
    ID              string   `json:"id"`
    Description     string   `json:"description"`
    ExpectedOutput  string   `json:"expected_output"`
    Complexity      float64  `json:"complexity"`
    Specialization  string   `json:"specialization"`
    Dependencies    []string `json:"dependencies"`
    EstimatedTokens int      `json:"estimated_tokens"`
}

// Router makes intelligent task routing decisions
type Router struct {
    analyzer           *complexity.Analyzer
    decomposer         *Decomposer
    complexityThreshold float64  // Default: 7.0
    enableSubAgents    bool
}

// Route analyzes a task and returns routing recommendations
func (r *Router) Route(ctx context.Context, task string) (*RoutingDecision, error) {
    // 1. Analyze complexity
    score, err := r.analyzer.Analyze(ctx, task)
    if err != nil {
        return nil, err
    }

    // 2. Determine if sub-agents needed
    useSubAgents := r.shouldUseSubAgents(task, score)

    // 3. Select strategy
    strategy := r.selectStrategy(task, score, useSubAgents)

    // 4. Decompose if needed
    var subtasks []Subtask
    if useSubAgents {
        subtasks, err = r.decomposer.Decompose(ctx, task, strategy)
        if err != nil {
            // Fallback to single agent
            useSubAgents = false
            strategy = StrategySingleAgent
        }
    }

    // 5. Build decision
    return &RoutingDecision{
        Strategy:        strategy,
        UseSubAgents:    useSubAgents,
        Subtasks:        subtasks,
        Specializations: r.extractSpecializations(subtasks),
        InferenceLane:   r.analyzer.SuggestLane(score),
        Confidence:      score.Confidence,
        Reasoning:       r.buildReasoning(score, strategy),
    }, nil
}

func (r *Router) shouldUseSubAgents(task string, score *complexity.ComplexityScore) bool {
    // Complexity threshold check
    if score.Overall >= r.complexityThreshold {
        return true
    }

    // Keyword triggers (override threshold)
    triggers := []string{
        "research and analyze",
        "comprehensive",
        "step by step",
        "multiple",
        "compare and contrast",
        "in-depth",
    }
    taskLower := strings.ToLower(task)
    for _, trigger := range triggers {
        if strings.Contains(taskLower, trigger) {
            return true
        }
    }

    // Multiple requirements check
    if score.RequirementCount >= 6.0 {
        return true
    }

    return false
}

func (r *Router) selectStrategy(task string, score *complexity.ComplexityScore, useSubAgents bool) RoutingStrategy {
    if !useSubAgents {
        return StrategySingleAgent
    }

    // Extreme complexity → hierarchical
    if score.Overall > 9.0 {
        return StrategyHierarchical
    }

    // Check for parallelizable structure
    hasIndependentParts := score.RequirementCount > 4.0 && !r.hasDependencyKeywords(task)
    if hasIndependentParts {
        return StrategyParallel
    }

    // Check for sequential dependencies
    if r.hasDependencyKeywords(task) {
        return StrategySequential
    }

    // Very complex with synthesis needs → hybrid/pipeline
    if score.Overall > 8.0 && strings.Contains(strings.ToLower(task), "synthesize") {
        return StrategyPipeline
    }

    // Default to sequential for safety
    return StrategySequential
}
```

---

### Component 3: TaskDecomposer

**Location:** `pkg/routing/decomposer.go`

```go
// pkg/routing/decomposer.go

package routing

// Decomposer breaks complex tasks into subtasks
type Decomposer struct {
    llmClient     LLMClient        // For LLM-based decomposition
    fallbackRules []DecomposeRule  // Rule-based fallback
}

// DecomposeRule defines a pattern-based decomposition rule
type DecomposeRule struct {
    Pattern     *regexp.Regexp
    SplitFunc   func(string) []string
    Specialization string
}

// Decompose breaks a task into subtasks based on strategy
func (d *Decomposer) Decompose(ctx context.Context, task string, strategy RoutingStrategy) ([]Subtask, error) {
    // Try LLM-based decomposition first
    subtasks, err := d.llmDecompose(ctx, task, strategy)
    if err == nil && len(subtasks) > 0 {
        return d.validateAndOptimize(subtasks)
    }

    // Fallback to rule-based
    return d.ruleBasedDecompose(task, strategy)
}

func (d *Decomposer) llmDecompose(ctx context.Context, task string, strategy RoutingStrategy) ([]Subtask, error) {
    prompt := d.buildDecompositionPrompt(task, strategy)

    response, err := d.llmClient.Generate(ctx, prompt)
    if err != nil {
        return nil, err
    }

    return d.parseSubtasks(response)
}

func (d *Decomposer) buildDecompositionPrompt(task string, strategy RoutingStrategy) string {
    var strategyGuidance string
    switch strategy {
    case StrategyParallel:
        strategyGuidance = "Break into INDEPENDENT subtasks that can run simultaneously."
    case StrategySequential:
        strategyGuidance = "Break into ORDERED steps where each depends on the previous."
    case StrategyHierarchical:
        strategyGuidance = "Create a manager task and worker tasks. Manager coordinates."
    case StrategyPipeline:
        strategyGuidance = "Create specialized stages: research → analysis → synthesis → output."
    default:
        strategyGuidance = "Break into logical subtasks."
    }

    return fmt.Sprintf(`Decompose this task into subtasks.

Strategy: %s
%s

Task: %s

For each subtask provide:
- id: st_1, st_2, etc.
- description: What to do
- expected_output: What result looks like
- specialization: research|coding|analysis|synthesis|writing
- dependencies: [list of st_X this depends on]
- estimated_complexity: 0-10

Return as JSON array.`, strategy, strategyGuidance, task)
}

func (d *Decomposer) ruleBasedDecompose(task string, strategy RoutingStrategy) ([]Subtask, error) {
    var parts []string

    // Try numbered items first
    if matches := regexp.MustCompile(`\d+\.\s+([^.]+\.)`).FindAllStringSubmatch(task, -1); len(matches) > 1 {
        for _, m := range matches {
            parts = append(parts, strings.TrimSpace(m[1]))
        }
    } else if strings.Contains(task, " and ") {
        // Split by "and" clauses
        parts = strings.Split(task, " and ")
    } else {
        // Split by sentences
        parts = strings.Split(task, ". ")
    }

    subtasks := make([]Subtask, len(parts))
    for i, part := range parts {
        subtasks[i] = Subtask{
            ID:              fmt.Sprintf("st_%d", i+1),
            Description:     strings.TrimSpace(part),
            Specialization:  d.guessSpecialization(part),
            Dependencies:    d.inferDependencies(i, parts),
            Complexity:      5.0, // Default mid-complexity
        }
    }

    return subtasks, nil
}

// validateAndOptimize removes circular dependencies and orders subtasks
func (d *Decomposer) validateAndOptimize(subtasks []Subtask) ([]Subtask, error) {
    // Build dependency graph
    graph := make(map[string][]string)
    for _, st := range subtasks {
        graph[st.ID] = st.Dependencies
    }

    // Detect cycles using DFS
    if d.hasCycle(graph) {
        // Remove problematic edges
        subtasks = d.removeCycles(subtasks, graph)
    }

    // Topological sort
    return d.topologicalSort(subtasks, graph)
}
```

---

### Component 4: Enhanced Orchestrator

**Location:** `pkg/brain/orchestrator.go`

Extends the existing PhaseExecutor with 6 orchestration patterns.

```go
// pkg/brain/orchestrator.go

package brain

import "github.com/normanking/cortex/pkg/routing"

// OrchestrationPattern defines how agents coordinate
type OrchestrationPattern int

const (
    PatternSequential OrchestrationPattern = iota
    PatternParallel
    PatternHierarchical
    PatternCollaborative
    PatternCompetitive
    PatternPipeline
)

// Orchestrator coordinates multi-agent execution
type Orchestrator struct {
    executor     *PhaseExecutor
    router       *routing.Router
    agentFactory AgentFactory
    tracker      *ExecutionTracker
}

// Execute runs a task using the recommended orchestration pattern
func (o *Orchestrator) Execute(ctx context.Context, task string, input LobeInput) (*ExecutionResult, error) {
    // 1. Get routing decision
    decision, err := o.router.Route(ctx, task)
    if err != nil {
        return nil, err
    }

    // 2. Select execution method based on strategy
    switch decision.Strategy {
    case routing.StrategySingleAgent:
        return o.executeSingle(ctx, input, decision)
    case routing.StrategyParallel:
        return o.executeParallel(ctx, input, decision)
    case routing.StrategySequential:
        return o.executeSequential(ctx, input, decision)
    case routing.StrategyHierarchical:
        return o.executeHierarchical(ctx, input, decision)
    case routing.StrategyCollaborative:
        return o.executeCollaborative(ctx, input, decision)
    case routing.StrategyCompetitive:
        return o.executeCompetitive(ctx, input, decision)
    case routing.StrategyPipeline:
        return o.executePipeline(ctx, input, decision)
    default:
        return o.executeSingle(ctx, input, decision)
    }
}

// executeHierarchical uses a manager agent to coordinate workers
func (o *Orchestrator) executeHierarchical(ctx context.Context, input LobeInput, decision *routing.RoutingDecision) (*ExecutionResult, error) {
    // Create manager agent
    manager := o.agentFactory.CreateManager(decision.Subtasks)

    // Create worker agents for each subtask
    workers := make(map[string]Agent)
    for _, st := range decision.Subtasks {
        workers[st.ID] = o.agentFactory.CreateWorker(st.Specialization)
    }

    // Manager distributes work
    assignments, err := manager.AssignTasks(ctx, decision.Subtasks)
    if err != nil {
        return nil, err
    }

    // Execute assignments (respecting dependencies)
    results := make(map[string]*LobeResult)
    for _, assignment := range o.orderByDependencies(assignments) {
        worker := workers[assignment.WorkerID]
        result, err := worker.Execute(ctx, assignment.Task, input)
        if err != nil {
            // Manager handles failure
            recovery, _ := manager.HandleFailure(ctx, assignment, err)
            if recovery != nil {
                result = recovery
            }
        }
        results[assignment.Task.ID] = result
    }

    // Manager synthesizes results
    return manager.Synthesize(ctx, results)
}

// executeCollaborative enables multi-round agent discussion
func (o *Orchestrator) executeCollaborative(ctx context.Context, input LobeInput, decision *routing.RoutingDecision) (*ExecutionResult, error) {
    const maxRounds = 5

    // Create discussion agents
    agents := o.agentFactory.CreateDiscussionPanel(len(decision.Subtasks))

    var consensus *LobeResult
    for round := 0; round < maxRounds; round++ {
        // Each agent proposes solution
        proposals := make([]*LobeResult, len(agents))
        for i, agent := range agents {
            proposals[i], _ = agent.Propose(ctx, input, proposals[:i])
        }

        // Check for consensus
        if consensus = o.checkConsensus(proposals); consensus != nil {
            break
        }

        // Agents critique each other
        for i, agent := range agents {
            input = o.incorporateFeedback(input, agent.Critique(ctx, proposals))
        }
    }

    if consensus == nil {
        // Vote on best proposal
        consensus = o.voteOnProposals(agents, proposals)
    }

    return &ExecutionResult{LobeResults: []*LobeResult{consensus}}, nil
}

// executeCompetitive runs multiple agents and selects best result
func (o *Orchestrator) executeCompetitive(ctx context.Context, input LobeInput, decision *routing.RoutingDecision) (*ExecutionResult, error) {
    const numCompetitors = 3

    // Create competing agents with different strategies
    competitors := o.agentFactory.CreateCompetitors(numCompetitors)

    // Run all in parallel
    results := make(chan *LobeResult, numCompetitors)
    for _, agent := range competitors {
        go func(a Agent) {
            result, _ := a.Execute(ctx, input.RawInput, input)
            results <- result
        }(agent)
    }

    // Collect results
    var allResults []*LobeResult
    for i := 0; i < numCompetitors; i++ {
        allResults = append(allResults, <-results)
    }

    // Judge selects best
    judge := o.agentFactory.CreateJudge()
    best := judge.SelectBest(ctx, allResults)

    return &ExecutionResult{LobeResults: []*LobeResult{best}}, nil
}

// executePipeline runs specialized stages in sequence
func (o *Orchestrator) executePipeline(ctx context.Context, input LobeInput, decision *routing.RoutingDecision) (*ExecutionResult, error) {
    // Standard pipeline stages
    stages := []string{"research", "analysis", "synthesis", "output"}

    var currentOutput interface{} = input.RawInput
    var allResults []*LobeResult

    for _, stage := range stages {
        agent := o.agentFactory.CreateSpecialist(stage)
        stageInput := LobeInput{
            RawInput:    currentOutput,
            PhaseConfig: map[string]interface{}{"stage": stage},
        }

        result, err := agent.Execute(ctx, stageInput)
        if err != nil {
            return nil, fmt.Errorf("pipeline stage %s failed: %w", stage, err)
        }

        allResults = append(allResults, result)
        currentOutput = result.Content
    }

    return &ExecutionResult{LobeResults: allResults}, nil
}
```

---

### Component 5: Cognitive Lobe Integration

**Location:** `pkg/brain/lobe_mapper.go`

Maps complexity scores to cognitive lobe activation.

```go
// pkg/brain/lobe_mapper.go

package brain

import "github.com/normanking/cortex/pkg/complexity"

// LobeActivationMap maps complexity dimensions to cognitive lobes
type LobeActivationMap struct {
    domainToLobes     map[complexity.Domain][]LobeID
    reasoningToLobes  map[complexity.ReasoningLevel][]LobeID
    toolToLobes       map[string][]LobeID
}

// DefaultLobeActivationMap creates standard mappings
func DefaultLobeActivationMap() *LobeActivationMap {
    return &LobeActivationMap{
        domainToLobes: map[complexity.Domain][]LobeID{
            complexity.DomainTechnical:  {LobeLogical, LobeAnalytical, LobeProblemSolving},
            complexity.DomainResearch:   {LobeInquisitive, LobeAnalytical, LobeMemory},
            complexity.DomainBusiness:   {LobeStrategic, LobeAnalytical, LobeSocial},
            complexity.DomainCreative:   {LobeCreative, LobeImaginative, LobeIntuitive},
            complexity.DomainData:       {LobeAnalytical, LobeLogical, LobePattern},
            complexity.DomainScientific: {LobeLogical, LobeInquisitive, LobeAnalytical},
            complexity.DomainLegal:      {LobeLogical, LobeMemory, LobeVerbal},
            complexity.DomainFinancial:  {LobeAnalytical, LobeLogical, LobeStrategic},
        },
        reasoningToLobes: map[complexity.ReasoningLevel][]LobeID{
            complexity.ReasoningSimple:      {LobeVerbal},
            complexity.ReasoningModerate:    {LobeVerbal, LobeAnalytical},
            complexity.ReasoningComplex:     {LobeAnalytical, LobeLogical, LobeProblemSolving},
            complexity.ReasoningVeryComplex: {LobeStrategic, LobeCreative, LobeIntegrative},
        },
        toolToLobes: map[string][]LobeID{
            "web_search":     {LobeInquisitive, LobeResearch},
            "code_execution": {LobeLogical, LobeTechnical},
            "file_ops":       {LobeOrganizational, LobeMemory},
            "calculation":    {LobeLogical, LobeAnalytical},
        },
    }
}

// SuggestLobes returns recommended lobes based on complexity score
func (m *LobeActivationMap) SuggestLobes(score *complexity.ComplexityScore) []LobeID {
    lobeSet := make(map[LobeID]bool)

    // Add lobes for detected domains
    for _, domain := range score.Domains {
        for _, lobe := range m.domainToLobes[complexity.Domain(domain)] {
            lobeSet[lobe] = true
        }
    }

    // Add lobes for reasoning level
    for _, lobe := range m.reasoningToLobes[score.ReasoningLevel] {
        lobeSet[lobe] = true
    }

    // Add lobes for required tools
    for _, tool := range score.RequiredTools {
        for _, lobe := range m.toolToLobes[tool] {
            lobeSet[lobe] = true
        }
    }

    // Convert to slice
    var lobes []LobeID
    for lobe := range lobeSet {
        lobes = append(lobes, lobe)
    }

    return lobes
}

// GenerateStrategy creates a ThinkingStrategy based on complexity
func (m *LobeActivationMap) GenerateStrategy(score *complexity.ComplexityScore) *ThinkingStrategy {
    lobes := m.SuggestLobes(score)

    // Determine phases based on complexity
    var phases []ExecutionPhase

    if score.Overall < 3.0 {
        // Simple: single phase, few lobes
        phases = []ExecutionPhase{{
            Name:   "Quick Response",
            Lobes:  lobes[:min(2, len(lobes))],
            Parallel: false,
        }}
    } else if score.Overall < 7.0 {
        // Moderate: two phases
        phases = []ExecutionPhase{
            {Name: "Analysis", Lobes: filterLobes(lobes, isAnalytical), Parallel: true},
            {Name: "Response", Lobes: filterLobes(lobes, isGenerative), Parallel: false},
        }
    } else {
        // Complex: full pipeline
        phases = []ExecutionPhase{
            {Name: "Research", Lobes: filterLobes(lobes, isResearch), Parallel: true},
            {Name: "Analysis", Lobes: filterLobes(lobes, isAnalytical), Parallel: true},
            {Name: "Synthesis", Lobes: filterLobes(lobes, isSynthetic), Parallel: false},
            {Name: "Output", Lobes: filterLobes(lobes, isGenerative), Parallel: false},
        }
    }

    return &ThinkingStrategy{
        Name:    fmt.Sprintf("Complexity-%.1f", score.Overall),
        Phases:  phases,
    }
}
```

---

## Implementation Plan

### Phase 1: ComplexityAnalyzer (Week 1)

| Task | Description | Effort |
|------|-------------|--------|
| 1.1 | Create `pkg/complexity/` package structure | 2h |
| 1.2 | Implement metric scoring functions | 4h |
| 1.3 | Build keyword detection (domains, tools, reasoning) | 4h |
| 1.4 | Implement weighted scoring calculation | 2h |
| 1.5 | Add lane suggestion logic | 2h |
| 1.6 | Write unit tests (90%+ coverage) | 4h |
| 1.7 | Integrate with existing AutoLLM in Pinky | 4h |

**Deliverable:** Drop-in replacement for Pinky's AutoLLM with 5-metric scoring

### Phase 2: TaskRouter (Week 2)

| Task | Description | Effort |
|------|-------------|--------|
| 2.1 | Create `pkg/routing/` package | 2h |
| 2.2 | Implement strategy selection logic | 4h |
| 2.3 | Build sub-agent decision logic | 3h |
| 2.4 | Implement RoutingDecision builder | 2h |
| 2.5 | Write unit tests | 3h |
| 2.6 | Integration tests with ComplexityAnalyzer | 2h |

**Deliverable:** Intelligent task router with 7 strategy options

### Phase 3: TaskDecomposer (Week 3)

| Task | Description | Effort |
|------|-------------|--------|
| 3.1 | Implement LLM-based decomposition | 4h |
| 3.2 | Build rule-based fallback | 3h |
| 3.3 | Implement dependency graph validation | 4h |
| 3.4 | Add topological sorting | 2h |
| 3.5 | Build decomposition prompts per strategy | 3h |
| 3.6 | Write tests with mock LLM | 4h |

**Deliverable:** Task decomposition with dependency management

### Phase 4: Orchestrator Patterns (Week 4)

| Task | Description | Effort |
|------|-------------|--------|
| 4.1 | Extend PhaseExecutor with pattern support | 4h |
| 4.2 | Implement hierarchical execution | 4h |
| 4.3 | Implement collaborative execution | 4h |
| 4.4 | Implement competitive execution | 3h |
| 4.5 | Implement pipeline execution | 3h |
| 4.6 | Integration tests for all patterns | 4h |

**Deliverable:** 6 orchestration patterns fully operational

### Phase 5: Lobe Integration (Week 5)

| Task | Description | Effort |
|------|-------------|--------|
| 5.1 | Create LobeActivationMap | 3h |
| 5.2 | Implement dynamic strategy generation | 4h |
| 5.3 | Wire into existing Executive | 3h |
| 5.4 | Add complexity-aware lobe selection | 3h |
| 5.5 | End-to-end integration tests | 4h |
| 5.6 | Performance benchmarks | 2h |

**Deliverable:** Cognitive lobes dynamically activated based on task complexity

### Phase 6: Pinky Integration (Week 6)

| Task | Description | Effort |
|------|-------------|--------|
| 6.1 | Update Pinky's brain.New() factory | 2h |
| 6.2 | Expose complexity scores in A2A responses | 3h |
| 6.3 | Add /complexity TUI command | 2h |
| 6.4 | Update WebUI to show complexity analysis | 4h |
| 6.5 | End-to-end testing Pinky → CortexBrain | 4h |
| 6.6 | Documentation and examples | 3h |

**Deliverable:** Full Pinky-CortexBrain integration with intelligent routing

---

## File Structure

```
pkg/
├── complexity/
│   ├── analyzer.go          # Core complexity analyzer
│   ├── analyzer_test.go     # Unit tests
│   ├── metrics.go           # Metric calculation helpers
│   ├── keywords.go          # Domain/tool/reasoning keywords
│   └── scoring.go           # Weighted scoring logic
│
├── routing/
│   ├── router.go            # Task router
│   ├── router_test.go       # Router tests
│   ├── decomposer.go        # Task decomposition
│   ├── decomposer_test.go   # Decomposer tests
│   ├── strategies.go        # Strategy definitions
│   └── subtask.go           # Subtask structures
│
└── brain/
    ├── orchestrator.go      # Enhanced orchestrator
    ├── orchestrator_test.go # Orchestrator tests
    ├── lobe_mapper.go       # Complexity → lobe mapping
    └── lobe_mapper_test.go  # Mapper tests
```

---

## Success Metrics

| Metric | Current | Target | Measurement |
|--------|---------|--------|-------------|
| Lane Selection Accuracy | ~60% | 90%+ | A/B test with human evaluation |
| Task Routing Latency | N/A | <50ms | p99 latency measurement |
| Decomposition Quality | N/A | 85%+ valid subtasks | Manual review of 100 samples |
| Lobe Utilization | ~30% | 70%+ | Track activated lobes per task |
| User Satisfaction | Baseline | +20% | Survey before/after |

---

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| LLM decomposition unreliable | Medium | Medium | Rule-based fallback always available |
| Complexity scoring too slow | Low | High | Cache keyword dictionaries, optimize regex |
| Over-decomposition | Medium | Medium | Minimum complexity threshold for sub-agents |
| Pattern selection wrong | Medium | Low | Default to sequential (safest) |
| Integration breaks existing | Low | High | Feature flags, gradual rollout |

---

## Dependencies

### External
- None (pure Go implementation)

### Internal
- `pkg/brain` - PhaseExecutor, Blackboard, Lobes
- `internal/config` - Configuration structures
- `internal/logging` - Logging infrastructure

---

## Open Questions

1. **LLM for Decomposition**: Which model should decompose tasks?
   - Option A: Use same lane as task execution
   - Option B: Always use fast lane (speed)
   - Option C: Dedicated decomposition model
   - **Recommendation:** Option B for latency

2. **Caching**: Should complexity scores be cached?
   - **Recommendation:** Yes, with 5-minute TTL for identical inputs

3. **Collaborative Rounds**: How many discussion rounds?
   - **Recommendation:** Max 5, with early exit on consensus

4. **Competitive Agents**: How many competitors?
   - **Recommendation:** 3 (balance cost vs. quality)

---

## References

- **Source Repository:** https://github.com/ctrl-gaurav/effGen
- **effGen Paper:** arXiv:2602.00887
- **CortexBrain Architecture:** `docs/ARCHITECTURE.md`
- **Pinky PRD:** `Pinky/docs/plans/PINKY-PRD.md`

---

## Appendix A: Keyword Dictionaries

### Domain Keywords

```go
var domainKeywords = map[Domain][]string{
    DomainTechnical: {
        "code", "programming", "software", "api", "database", "algorithm",
        "debug", "deploy", "server", "frontend", "backend", "devops",
    },
    DomainResearch: {
        "research", "study", "investigate", "literature", "survey",
        "analyze", "compare", "review", "examine", "explore",
    },
    DomainBusiness: {
        "business", "strategy", "market", "revenue", "customer",
        "roi", "stakeholder", "kpi", "growth", "competitive",
    },
    DomainCreative: {
        "creative", "design", "art", "write", "story", "brand",
        "visual", "aesthetic", "innovative", "original",
    },
    DomainData: {
        "data", "analytics", "metrics", "statistics", "visualization",
        "dataset", "ml", "model", "prediction", "correlation",
    },
    DomainScientific: {
        "scientific", "experiment", "hypothesis", "theory", "physics",
        "chemistry", "biology", "mathematics", "proof", "empirical",
    },
    DomainLegal: {
        "legal", "law", "contract", "compliance", "regulation",
        "liability", "rights", "patent", "copyright", "terms",
    },
    DomainFinancial: {
        "financial", "budget", "investment", "cost", "profit",
        "accounting", "tax", "valuation", "portfolio", "risk",
    },
}
```

### Reasoning Keywords

```go
var reasoningKeywords = map[ReasoningLevel][]string{
    ReasoningSimple: {
        "list", "define", "identify", "name", "state", "recall",
        "describe", "tell", "what is", "who is",
    },
    ReasoningModerate: {
        "explain", "summarize", "compare", "contrast", "classify",
        "illustrate", "interpret", "outline", "discuss",
    },
    ReasoningComplex: {
        "analyze", "evaluate", "assess", "critique", "design",
        "develop", "formulate", "construct", "investigate",
    },
    ReasoningVeryComplex: {
        "synthesize", "architect", "optimize", "integrate", "transform",
        "revolutionize", "pioneer", "comprehensive", "holistic",
    },
}
```

---

## Appendix B: Example Complexity Scores

| Input | Overall | Length | Reqs | Domains | Tools | Reasoning | Lane |
|-------|---------|--------|------|---------|-------|-----------|------|
| "What is Go?" | 2.1 | 2.0 | 1.5 | 2.5 | 0.0 | 2.5 | local |
| "Explain how goroutines work" | 4.2 | 4.0 | 1.5 | 5.0 | 0.0 | 5.0 | fast |
| "Build a REST API with auth" | 6.8 | 4.0 | 6.0 | 7.5 | 8.0 | 7.5 | fast |
| "Research, design, and implement a distributed cache" | 9.1 | 6.0 | 10.0 | 10.0 | 10.0 | 10.0 | smart |

---

*Document Version: 1.0*
*Last Updated: 2026-02-08*
