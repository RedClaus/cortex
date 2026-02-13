---
project: Cortex
component: Brain Kernel
phase: Ideation
date_created: 2026-02-12T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-12T23:25:41.822723
---

# Product Requirements Document: Agentic Brain System

**Document Version:** 1.0
**Created:** 2026-02-12
**Author:** Norman King & Codie (Claude Code Agent)
**Status:** Draft

---

## Executive Summary

### What We Are Building

A **self-improving agentic AI system** that combines local specialized AI models ("lobes") with frontier AI models (Claude, GPT-4) through an intelligent Agent layer. The system learns from successful frontier model executions and continuously trains local models to become more capable, reducing costs while increasing performance over time.

### Why We Are Building It

1. **Cost Optimization**: Frontier AI APIs are expensive ($0.01-0.10+ per call). A local brain that handles routine tasks costs $0.
2. **Speed**: Small specialized models (1-3B params) run 10x faster than large general models.
3. **Privacy**: Sensitive operations stay on local hardware, no data leaves the system.
4. **Continuous Improvement**: The system gets smarter with use, learning from every successful frontier interaction.
5. **Resilience**: Works offline once trained, no dependency on external APIs for learned capabilities.

### Overall Goal

Create an AI system that:
- **Today**: Uses frontier models for complex tasks, local models for simple ones
- **Month 1**: Local handles 50% of tasks (learned from frontier successes)
- **Month 6**: Local handles 80%+ of tasks
- **Long-term**: A highly capable local AI that only needs frontier models for truly novel situations

---

## Table of Contents

1. [Vision & Goals](#1-vision--goals)
2. [System Architecture](#2-system-architecture)
3. [Component Specifications](#3-component-specifications)
4. [Data Flow & Learning Pipeline](#4-data-flow--learning-pipeline)
5. [User Stories](#5-user-stories)
6. [Technical Requirements](#6-technical-requirements)
7. [Success Metrics](#7-success-metrics)
8. [Implementation Phases](#8-implementation-phases)
9. [Risks & Mitigations](#9-risks--mitigations)
10. [Appendix](#10-appendix)

---

## 1. Vision & Goals

### 1.1 Vision Statement

> Build an AI brain that learns and improves with every interaction, combining the power of frontier AI models with the speed and privacy of local specialized models, creating a self-improving system that gets smarter and cheaper to operate over time.

### 1.2 Core Goals

| Goal | Description | Measurable Target |
|------|-------------|-------------------|
| **Agentic Capability** | System can autonomously plan, execute, and complete multi-step tasks | Complete 80% of application generation requests without human intervention |
| **Self-Improvement** | System learns from frontier model successes | Local model accuracy improves 5% monthly |
| **Cost Reduction** | Reduce reliance on expensive API calls | 80% of tasks handled locally within 6 months |
| **Speed** | Fast response times for routine tasks | <500ms for local lobe responses |
| **Privacy** | Sensitive data stays local | Zero sensitive data sent to external APIs |

### 1.3 Non-Goals (Out of Scope for v1)

- Real-time voice interaction
- Multi-modal (image/video) processing
- Multi-user collaboration features
- Mobile/embedded deployment

---

## 2. System Architecture

### 2.1 High-Level Architecture

```
┌──────────────────────────────────────────────────────────────────────────────────┐
│                              CORTEX AGENTIC SYSTEM                                │
│                                                                                   │
│  ┌─────────────────────────────────────────────────────────────────────────────┐ │
│  │                              USER INTERFACE                                  │ │
│  │                                                                              │ │
│  │   CLI (Pinky)  │  Web UI  │  API  │  IDE Extensions  │  Voice (future)      │ │
│  └─────────────────────────────────────────────────────────────────────────────┘ │
│                                        │                                          │
│                                        ▼                                          │
│  ┌─────────────────────────────────────────────────────────────────────────────┐ │
│  │                              AGENT LAYER                                     │ │
│  │                                                                              │ │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │ │
│  │  │    Goal     │  │    Task     │  │   State     │  │   Clarification     │ │ │
│  │  │   Manager   │  │    Queue    │  │   Tracker   │  │      Engine         │ │ │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────────────┘ │ │
│  │                                                                              │ │
│  │  ┌───────────────────────────────────────────────────────────────────────┐  │ │
│  │  │                         BRAIN ROUTER                                   │  │ │
│  │  │                                                                        │  │ │
│  │  │   Input ──▶ Classify ──▶ Check Skills ──▶ Route Decision              │  │ │
│  │  │                              │                    │                    │  │ │
│  │  │                              ▼                    ▼                    │  │ │
│  │  │                     Known skill?           Complexity?                 │  │ │
│  │  │                      YES → Local            LOW → Local                │  │ │
│  │  │                      NO  → Check           HIGH → Frontier             │  │ │
│  │  └───────────────────────────────────────────────────────────────────────┘  │ │
│  │                          │                              │                    │ │
│  └──────────────────────────┼──────────────────────────────┼────────────────────┘ │
│                             │                              │                      │
│           ┌─────────────────┴──────────────┐   ┌──────────┴───────────────┐      │
│           ▼                                │   │                          ▼      │
│  ┌─────────────────────────────────────┐   │   │   ┌─────────────────────────┐   │
│  │           LOCAL BRAIN               │   │   │   │     FRONTIER BRAIN      │   │
│  │                                     │   │   │   │                         │   │
│  │  ┌─────────┐ ┌─────────┐ ┌───────┐ │   │   │   │  ┌───────────────────┐  │   │
│  │  │Planning │ │ Coding  │ │Safety │ │   │   │   │  │  Claude/GPT-4     │  │   │
│  │  │ Lobe    │ │  Lobe   │ │ Lobe  │ │   │   │   │  │                   │  │   │
│  │  │ (1B)    │ │  (3B)   │ │ (1B)  │ │   │   │   │  │  API Calls        │  │   │
│  │  └─────────┘ └─────────┘ └───────┘ │   │   │   │  │  with context     │  │   │
│  │  ┌─────────┐ ┌─────────┐ ┌───────┐ │   │   │   │  └───────────────────┘  │   │
│  │  │Reasoning│ │Creative │ │Memory │ │   │   │   │           │             │   │
│  │  │ Lobe    │ │  Lobe   │ │ Lobe  │ │   │   │   │           │             │   │
│  │  │ (3B)    │ │  (3B)   │ │ (1B)  │ │   │   │   │           ▼             │   │
│  │  └─────────┘ └─────────┘ └───────┘ │   │   │   │  ┌───────────────────┐  │   │
│  │                                     │   │   │   │  │ Success Capture   │  │   │
│  │  Ollama / MLX Runtime              │   │   │   │  │ (for training)    │  │   │
│  └─────────────────────────────────────┘   │   │   │  └───────────────────┘  │   │
│                                            │   │   │                         │   │
│                                            │   │   └─────────────────────────┘   │
│                                            │   │              │                  │
│           ┌────────────────────────────────┘   └──────────────┼──────────────┐   │
│           │                                                    │              │   │
│           ▼                                                    ▼              │   │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │   │
│  │                         TOOL EXECUTOR                                    │ │   │
│  │                                                                          │ │   │
│  │  write_file │ read_file │ run_command │ web_search │ ide_open │ ...     │ │   │
│  └─────────────────────────────────────────────────────────────────────────┘ │   │
│                                                                               │   │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │   │
│  │                         LEARNING PIPELINE                                │◀┘   │
│  │                                                                          │     │
│  │  Experience    Dataset     Training      Model        Deployment         │     │
│  │  Collector  ──▶ Curator ──▶ Pipeline ──▶ Registry ──▶ to Lobes          │     │
│  │                                                                          │     │
│  └─────────────────────────────────────────────────────────────────────────┘     │
│                                                                                   │
│  ┌─────────────────────────────────────────────────────────────────────────────┐ │
│  │                         PERSISTENCE LAYER                                    │ │
│  │                                                                              │ │
│  │  User Memory │ Skill Memory │ Training Data │ Model Versions │ Logs         │ │
│  └─────────────────────────────────────────────────────────────────────────────┘ │
│                                                                                   │
└───────────────────────────────────────────────────────────────────────────────────┘
```

### 2.2 Component Responsibilities

| Component | Responsibility | Key Interfaces |
|-----------|---------------|----------------|
| **Agent Layer** | Orchestrates the agentic loop (perceive→think→act→observe) | BrainInterface, ToolExecutor |
| **Brain Router** | Decides which brain to use based on task complexity and skill availability | SkillMemory, ClassificationResult |
| **Local Brain** | Fast, specialized cognitive processing using fine-tuned small models | LobeInterface, OllamaClient |
| **Frontier Brain** | High-capability processing using Claude/GPT-4 APIs | AnthropicClient, OpenAIClient |
| **Tool Executor** | Executes actions on the world (files, commands, web) | ToolResult |
| **Learning Pipeline** | Captures successes and trains local models | ExperienceBuffer, TrainingScheduler |

---

## 3. Component Specifications

### 3.1 Agent Layer

#### 3.1.1 Purpose
The Agent Layer is the orchestration layer that implements the agentic loop. It receives user goals, breaks them into tasks, executes them using the appropriate brain, and learns from the results.

#### 3.1.2 Agentic Loop

```go
// Pseudocode for the agentic loop
func (a *Agent) Execute(ctx context.Context, goal string) (*Result, error) {
    // 1. Understand the goal
    understanding := a.brain.Process(ctx, "Understand this goal: " + goal)

    // 2. Plan the tasks
    tasks := a.brain.Plan(ctx, understanding)

    // 3. Execute each task
    for _, task := range tasks {
        // Choose brain based on task complexity and skill availability
        brain := a.router.SelectBrain(task)

        // Think about the task
        thought := brain.Process(ctx, task.Description)

        // Decide on action
        action := a.decider.Decide(thought)

        // If confused, ask for clarification
        if action.NeedsClarification {
            question := a.clarifier.GenerateQuestion(task, thought)
            answer := a.askUser(question)
            thought = brain.Process(ctx, task.Description + "\nUser clarification: " + answer)
            action = a.decider.Decide(thought)
        }

        // Execute the action
        result := a.executor.Execute(ctx, action)

        // Observe and learn
        a.observer.Record(task, action, result)

        // If frontier was used and successful, capture for training
        if brain.Type() == "frontier" && result.Success {
            a.learner.CaptureExperience(task, thought, action, result)
        }
    }

    // 4. Summarize and return
    return a.summarize(tasks), nil
}
```

#### 3.1.3 State Management

```go
type AgentState struct {
    GoalID          string                 `json:"goal_id"`
    Goal            string                 `json:"goal"`
    Status          string                 `json:"status"` // planning, executing, clarifying, completed, failed
    Tasks           []Task                 `json:"tasks"`
    CurrentTaskIdx  int                    `json:"current_task_idx"`
    Context         map[string]interface{} `json:"context"`
    StartedAt       time.Time              `json:"started_at"`
    CompletedAt     *time.Time             `json:"completed_at,omitempty"`
}

type Task struct {
    ID              string            `json:"id"`
    Name            string            `json:"name"`
    Description     string            `json:"description"`
    Status          string            `json:"status"`
    BrainUsed       string            `json:"brain_used"` // local, frontier
    ToolsUsed       []string          `json:"tools_used"`
    Result          *TaskResult       `json:"result,omitempty"`
    DependsOn       []string          `json:"depends_on,omitempty"`
}
```

### 3.2 Brain Router

#### 3.2.1 Purpose
Intelligently routes tasks to the appropriate brain (local or frontier) based on:
- Task complexity
- Available skills in local brain
- User preferences (cost vs quality)
- Confidence thresholds

#### 3.2.2 Routing Logic

```go
func (r *BrainRouter) SelectBrain(task Task) BrainInterface {
    // 1. Check if we have a matching skill
    skill := r.skillMemory.FindSkill(task.Description)
    if skill != nil && skill.SuccessRate > 0.9 {
        r.log.Info("Using local brain - matched skill: %s", skill.Name)
        return r.localBrain
    }

    // 2. Classify task complexity
    complexity := r.classifier.Classify(task)

    // 3. Route based on complexity
    switch complexity.Level {
    case "trivial", "simple":
        return r.localBrain
    case "moderate":
        // Try local first, fallback to frontier
        return r.localBrainWithFallback
    case "complex", "novel":
        return r.frontierBrain
    }

    return r.frontierBrain // Default to frontier for safety
}
```

#### 3.2.3 Complexity Classification

| Level | Characteristics | Examples | Brain |
|-------|----------------|----------|-------|
| Trivial | Single step, well-defined | "Create a file", "Run npm install" | Local |
| Simple | Few steps, clear path | "Initialize a React project" | Local |
| Moderate | Multiple steps, some ambiguity | "Add authentication to existing app" | Local + Fallback |
| Complex | Many steps, requires planning | "Create a full-stack app from PRD" | Frontier |
| Novel | Never seen before, requires creativity | "Design a new architecture pattern" | Frontier |

### 3.3 Local Brain (Lobe System)

#### 3.3.1 Purpose
Provides fast, specialized cognitive processing using fine-tuned small language models. Each lobe is an expert in a specific domain.

#### 3.3.2 Lobe Specifications

| Lobe | Base Model | Size | Specialty | Latency Target |
|------|-----------|------|-----------|----------------|
| Planning | Llama-3.2 | 1B | Task decomposition, dependency ordering | <300ms |
| Coding | CodeLlama | 3B | Code generation, debugging, refactoring | <500ms |
| Reasoning | Llama-3.2 | 3B | Logic, analysis, problem-solving | <400ms |
| Safety | Llama-3.2 | 1B | Security review, risk assessment | <200ms |
| Creativity | Mistral | 3B | Brainstorming, ideation, alternatives | <400ms |
| Memory | Llama-3.2 | 1B | Context retrieval, fact lookup | <200ms |

#### 3.3.3 Lobe Interface

```go
type LobeInterface interface {
    ID() LobeID
    Process(ctx context.Context, input LobeInput, bb *Blackboard) (*LobeResult, error)
    CanHandle(input string) float64  // 0.0 to 1.0 confidence
    ResourceEstimate(input LobeInput) ResourceEstimate
}

type LobeInput struct {
    RawInput    string
    Context     map[string]interface{}
    Strategy    *ThinkingStrategy
    Constraints *Constraints
}

type LobeResult struct {
    LobeID     LobeID
    Content    string
    Confidence float64
    Reasoning  string
    Meta       LobeMeta
}
```

### 3.4 Frontier Brain

#### 3.4.1 Purpose
Provides high-capability cognitive processing using frontier AI models (Claude, GPT-4) for complex, novel, or high-stakes tasks.

#### 3.4.2 Provider Support

| Provider | Models | Use Case | Cost (approx) |
|----------|--------|----------|---------------|
| Anthropic | claude-sonnet-4-20250514, claude-opus-4-5-20251101 | Complex reasoning, safety-critical | $0.003-0.015/1K tokens |
| OpenAI | gpt-4o, gpt-4-turbo | General purpose, code generation | $0.005-0.03/1K tokens |
| Google | gemini-pro | Multimodal, long context | $0.00025-0.0005/1K tokens |

#### 3.4.3 Frontier Brain Interface

```go
type FrontierBrain struct {
    provider    string
    model       string
    client      *http.Client
    rateLimiter *RateLimiter
    costTracker *CostTracker
}

func (f *FrontierBrain) Process(ctx context.Context, input string) (*BrainResult, error) {
    // Track costs
    defer f.costTracker.Record(input, result)

    // Rate limit
    if err := f.rateLimiter.Wait(ctx); err != nil {
        return nil, err
    }

    // Make API call
    response, err := f.client.Chat(ctx, &ChatRequest{
        Model:    f.model,
        Messages: []Message{{Role: "user", Content: input}},
    })

    return &BrainResult{
        Content:    response.Content,
        Confidence: 0.95,
        Source:     "frontier",
        Model:      f.model,
        TokensUsed: response.Usage.TotalTokens,
    }, nil
}
```

### 3.5 Tool Executor

#### 3.5.1 Available Tools

| Tool | Description | Parameters | Safety Level |
|------|-------------|------------|--------------|
| `write_file` | Create or overwrite a file | path, content | Medium |
| `read_file` | Read file contents | path | Low |
| `run_command` | Execute shell command | command, working_dir | High |
| `list_directory` | List directory contents | path | Low |
| `web_search` | Search the web | query | Low |
| `ide_open` | Open file in IDE | path, line | Low |
| `scaffold_project` | Create project from template | name, template, path | Medium |

#### 3.5.2 Safety Levels

- **Low**: Read-only operations, no side effects
- **Medium**: Creates/modifies files, reversible
- **High**: Executes code, potentially irreversible, requires confirmation for dangerous operations

### 3.6 Learning Pipeline

#### 3.6.1 Purpose
Captures successful frontier model executions and uses them to train local lobe models, enabling continuous improvement.

#### 3.6.2 Experience Format

```go
type Experience struct {
    ID            string                 `json:"id"`
    Timestamp     time.Time              `json:"timestamp"`
    LobeType      string                 `json:"lobe_type"`
    TaskType      string                 `json:"task_type"`

    Input         ExperienceInput        `json:"input"`
    Output        ExperienceOutput       `json:"output"`
    Execution     ExecutionMetrics       `json:"execution"`
    Quality       QualityMetrics         `json:"quality"`

    Source        string                 `json:"source"` // "frontier"
    Model         string                 `json:"model"`
}

type ExperienceInput struct {
    SystemPrompt  string                 `json:"system_prompt"`
    UserInput     string                 `json:"user_input"`
    Context       map[string]interface{} `json:"context"`
}

type ExperienceOutput struct {
    Content       string                 `json:"content"`
    Reasoning     string                 `json:"reasoning,omitempty"`
    Confidence    float64                `json:"confidence"`
}

type QualityMetrics struct {
    Completeness   float64  `json:"completeness"`   // 0-1: Did output cover all requirements?
    Executability  float64  `json:"executability"`  // 0-1: Could the output be executed?
    Efficiency     float64  `json:"efficiency"`     // 0-1: Was it concise and direct?
    UserSatisfied  bool     `json:"user_satisfied"` // Implicit or explicit feedback
    OverallScore   float64  `json:"overall_score"`
}
```

#### 3.6.3 Training Pipeline

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         TRAINING PIPELINE                                    │
│                                                                              │
│  1. COLLECTION                                                               │
│     ────────────                                                             │
│     Frontier Success ──▶ Experience Buffer ──▶ Quality Filter               │
│                                                    │                         │
│                                                    ▼                         │
│  2. CURATION                                    Score > 0.85?                │
│     ─────────                                      │                         │
│                              ┌─────────────────────┴─────────────────────┐  │
│                              │ YES                                   NO  │  │
│                              ▼                                       ▼   │  │
│                         Add to Dataset                           Discard │  │
│                              │                                           │  │
│                              ▼                                           │  │
│  3. FORMATTING          Tag by Lobe Type                                │  │
│     ──────────               │                                           │  │
│                              ▼                                           │  │
│                    ┌─────────┴─────────┐                                │  │
│                    ▼         ▼         ▼                                │  │
│               Planning   Coding   Reasoning  ...                        │  │
│               Dataset    Dataset   Dataset                              │  │
│                    │         │         │                                │  │
│                    └─────────┴─────────┘                                │  │
│                              │                                           │  │
│  4. TRAINING                 ▼                                           │  │
│     ────────        Dataset size > threshold?                           │  │
│                    OR scheduled time?                                    │  │
│                              │                                           │  │
│                              ▼ YES                                       │  │
│                    ┌─────────────────────┐                              │  │
│                    │  Fine-tune with LoRA │                              │  │
│                    │  • Base: Llama-3.2   │                              │  │
│                    │  • Rank: 32          │                              │  │
│                    │  • Epochs: 3         │                              │  │
│                    └─────────────────────┘                              │  │
│                              │                                           │  │
│  5. EVALUATION               ▼                                           │  │
│     ──────────        Run on test set                                   │  │
│                              │                                           │  │
│                    New model better?                                     │  │
│                              │                                           │  │
│                    ┌─────────┴─────────┐                                │  │
│                    ▼ YES               ▼ NO                             │  │
│  6. DEPLOYMENT   Deploy to Ollama    Keep old model                     │  │
│     ──────────   Update lobe config                                     │  │
│                                                                          │  │
└──────────────────────────────────────────────────────────────────────────┘
```

---

## 4. Data Flow & Learning Pipeline

### 4.1 Request Flow (Happy Path)

```
User: "Create a task management app with user auth and CRUD operations"
                                    │
                                    ▼
┌──────────────────────────────────────────────────────────────────────────┐
│ AGENT: Receive Goal                                                       │
│                                                                           │
│ 1. Parse request                                                          │
│ 2. Check skill memory: "task management app" → No exact match            │
│ 3. Classify complexity: COMPLEX (multi-step, requires planning)          │
│ 4. Route to: FRONTIER BRAIN                                              │
└──────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌──────────────────────────────────────────────────────────────────────────┐
│ FRONTIER BRAIN: Plan                                                      │
│                                                                           │
│ Input: "Create a task management app with user auth and CRUD operations" │
│ Output:                                                                   │
│   1. Initialize project structure                                        │
│   2. Create User model with auth fields                                  │
│   3. Create Task model with CRUD                                         │
│   4. Implement authentication (JWT)                                      │
│   5. Create API endpoints                                                │
│   6. Add basic UI                                                        │
│   7. Write tests                                                         │
└──────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌──────────────────────────────────────────────────────────────────────────┐
│ AGENT: Execute Tasks                                                      │
│                                                                           │
│ For each task:                                                           │
│   • Route simple tasks to LOCAL (mkdir, write boilerplate)               │
│   • Route complex tasks to FRONTIER (auth implementation)                │
│   • Execute tools (write_file, run_command)                              │
│   • Record results                                                        │
└──────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌──────────────────────────────────────────────────────────────────────────┐
│ LEARNING: Capture Success                                                 │
│                                                                           │
│ Frontier outputs captured:                                               │
│   • Planning output → planning_dataset.jsonl                             │
│   • Code generation → coding_dataset.jsonl                               │
│                                                                           │
│ Skill registered:                                                         │
│   • "task management app with auth" → success pattern stored             │
└──────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌──────────────────────────────────────────────────────────────────────────┐
│ RESULT: Complete                                                          │
│                                                                           │
│ Summary: Created TaskManager with 15 files                               │
│ Files: src/models/User.ts, src/models/Task.ts, src/auth/...             │
│ Next steps: npm install && npm run dev                                   │
└──────────────────────────────────────────────────────────────────────────┘
```

### 4.2 Learning Flow

```
WEEK 1: System deployed
─────────────────────────
• All complex tasks → Frontier
• Local handles: 20% of tasks
• Cost: $50/week in API calls
• Training data collected: 500 experiences

WEEK 2: First training run
─────────────────────────
• PlanningLobe fine-tuned on 400 planning examples
• CodingLobe fine-tuned on 800 coding examples
• Local accuracy: 65% → 78%

MONTH 1: Improvement visible
─────────────────────────
• Local handles: 50% of tasks
• Cost reduced: $30/week
• Training data: 2,000 experiences
• Skills registered: 50 patterns

MONTH 3: Significant capability
─────────────────────────
• Local handles: 70% of tasks
• Cost reduced: $15/week
• Local accuracy: 88%
• Frontier only for novel tasks

MONTH 6: Mature system
─────────────────────────
• Local handles: 85% of tasks
• Cost reduced: $5/week
• System handles most routine tasks locally
• Frontier reserved for truly novel situations
```

---

## 5. User Stories

### 5.1 Application Generation

**As a** developer
**I want to** say "Create an application called TaskMaster with this PRD: [features]"
**So that** the system automatically generates a working application skeleton

**Acceptance Criteria:**
- [ ] System extracts application name from natural language
- [ ] System parses PRD to identify features, entities, and tech stack
- [ ] System creates project structure with appropriate files
- [ ] System generates README with setup instructions
- [ ] System opens project in IDE when complete
- [ ] System asks clarifying questions if requirements are ambiguous

### 5.2 Skill Learning

**As a** system administrator
**I want** the local brain to learn from successful frontier model executions
**So that** routine tasks are handled locally without API costs

**Acceptance Criteria:**
- [ ] Successful frontier executions are captured with full context
- [ ] Experiences are quality-filtered before adding to training data
- [ ] Training runs automatically when dataset threshold is reached
- [ ] New models are evaluated before deployment
- [ ] System shows improvement metrics over time

### 5.3 Cost Optimization

**As a** cost-conscious user
**I want** the system to prefer local processing when possible
**So that** I minimize API costs while maintaining quality

**Acceptance Criteria:**
- [ ] Simple tasks never hit frontier APIs
- [ ] Known skills are handled locally
- [ ] Cost tracking shows savings over time
- [ ] User can set cost/quality preferences

### 5.4 Offline Operation

**As a** user with intermittent connectivity
**I want** the system to work offline for learned capabilities
**So that** I can be productive without internet access

**Acceptance Criteria:**
- [ ] Learned skills work without network
- [ ] System gracefully degrades when frontier unavailable
- [ ] User is informed when a task requires frontier access

---

## 6. Technical Requirements

### 6.1 Performance Requirements

| Metric | Target | Rationale |
|--------|--------|-----------|
| Local lobe latency | <500ms | Fast enough for interactive use |
| Agentic loop iteration | <2s | Perceive→Think→Act cycle |
| Training pipeline | <4 hours | Complete overnight |
| Model deployment | <5 minutes | Hot-swap models |

### 6.2 Reliability Requirements

| Metric | Target | Rationale |
|--------|--------|-----------|
| Local brain availability | 99.9% | Core functionality |
| Frontier fallback success | 99% | When local fails |
| Training success rate | 95% | Pipeline reliability |
| Data durability | 99.99% | Training data is valuable |

### 6.3 Scalability Requirements

| Dimension | Initial | Target | Notes |
|-----------|---------|--------|-------|
| Concurrent users | 1 | 10 | Single-user focus initially |
| Experiences/day | 100 | 10,000 | Learning capacity |
| Model versions | 5 | 50 | Per lobe |
| Training data size | 10GB | 100GB | Growth capacity |

### 6.4 Hardware Requirements

**Minimum (Development):**
- Apple M1/M2 Mac with 16GB RAM
- 50GB disk space
- Internet for frontier access

**Recommended (Production):**
- Apple M2 Pro/Max or NVIDIA RTX 3090+
- 32GB+ RAM
- 200GB SSD
- Internet for frontier access

---

## 7. Success Metrics

### 7.1 Primary Metrics

| Metric | Definition | Target (6 months) |
|--------|------------|-------------------|
| **Local Task Ratio** | % of tasks handled by local brain | 80% |
| **Cost Reduction** | Frontier API costs vs baseline | -75% |
| **User Satisfaction** | Task completion without intervention | 85% |
| **Learning Rate** | Skills learned per week | 10+ |

### 7.2 Secondary Metrics

| Metric | Definition | Target |
|--------|------------|--------|
| Local accuracy | Correct outputs from local brain | 90% |
| Latency p95 | 95th percentile response time | <2s |
| Training success | Successful training runs | 95% |
| Rollback rate | Models rolled back after deploy | <5% |

### 7.3 Tracking Dashboard

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    CORTEX LEARNING DASHBOARD                             │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  TASK ROUTING                          COST SAVINGS                      │
│  ─────────────                         ────────────                      │
│  Local:    ████████████░░░ 78%         This week:    $42 saved          │
│  Frontier: ████░░░░░░░░░░░ 22%         This month:   $180 saved         │
│                                        Total:        $1,250 saved        │
│                                                                          │
│  LEARNING PROGRESS                     MODEL VERSIONS                    │
│  ─────────────────                     ──────────────                    │
│  Experiences: 12,847                   Planning: v3.2 (deployed)        │
│  Training runs: 15                     Coding:   v4.1 (deployed)        │
│  Skills learned: 127                   Reasoning: v2.8 (training)       │
│                                        Safety:   v1.5 (deployed)        │
│                                                                          │
│  ACCURACY OVER TIME                                                      │
│  ─────────────────                                                       │
│  100%│                                    ╭─────                        │
│   90%│                        ╭───────────╯                             │
│   80%│            ╭───────────╯                                         │
│   70%│    ╭───────╯                                                     │
│   60%│────╯                                                             │
│      └────────────────────────────────────────────────                  │
│       Week 1    Week 4    Week 8    Week 12   Week 16                   │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 8. Implementation Phases

### Phase 1: Agent Foundation (2 weeks)

**Goal:** Establish the Agent layer with brain routing

**Deliverables:**
- [ ] BrainInterface definition
- [ ] Agent core with agentic loop
- [ ] LocalBrain wrapper for existing lobes
- [ ] FrontierBrain wrapper for Claude API
- [ ] Basic BrainRouter (complexity-based)
- [ ] Tool Executor integration

**Success Criteria:**
- Agent can complete simple multi-step tasks
- Routing works between local and frontier
- Tools execute correctly

### Phase 2: Learning Infrastructure (2 weeks)

**Goal:** Build data collection and storage

**Deliverables:**
- [ ] Experience data format and schema
- [ ] Experience Collector (captures frontier successes)
- [ ] Quality Filter (score-based filtering)
- [ ] Dataset Store (organized by lobe type)
- [ ] Dataset versioning

**Success Criteria:**
- Experiences captured automatically from frontier runs
- Quality filtering removes low-quality examples
- Datasets organized and queryable

### Phase 3: Training Pipeline (3 weeks)

**Goal:** Automated model training and deployment

**Deliverables:**
- [ ] Training Scheduler (triggers on threshold or schedule)
- [ ] LoRA fine-tuning script for MLX
- [ ] Model Evaluator (test set evaluation)
- [ ] Model Registry (version management)
- [ ] Deployment to Ollama (hot-swap models)

**Success Criteria:**
- Training runs automatically when enough data
- New models evaluated before deployment
- Models can be rolled back if needed

### Phase 4: Skill Learning (2 weeks)

**Goal:** Pattern recognition and skill reuse

**Deliverables:**
- [ ] Skill Pattern format
- [ ] Skill Extractor (captures successful patterns)
- [ ] Skill Matcher (finds similar past skills)
- [ ] Skill-based routing in BrainRouter

**Success Criteria:**
- Similar tasks matched to past successes
- Local brain attempts known skills
- Success rate tracking per skill

### Phase 5: Polish & Monitoring (1 week)

**Goal:** Production readiness

**Deliverables:**
- [ ] Metrics dashboard
- [ ] Cost tracking
- [ ] Error handling and recovery
- [ ] Documentation

**Success Criteria:**
- System runs reliably
- Metrics visible
- Costs tracked

---

## 9. Risks & Mitigations

### 9.1 Technical Risks

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| **Fine-tuning degrades model** | High | Medium | Always evaluate before deploy, keep rollback versions |
| **Training data quality issues** | High | Medium | Strict quality filtering, human review sampling |
| **Frontier API costs spike** | Medium | Low | Cost limits, aggressive local routing |
| **Local models too slow** | Medium | Low | Quantization, model selection, caching |

### 9.2 Process Risks

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| **Scope creep** | Medium | High | Strict phase boundaries, MVP focus |
| **Training pipeline complexity** | Medium | Medium | Start simple (manual triggers), automate incrementally |

### 9.3 Data Risks

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| **Training data loss** | High | Low | Backups, versioning |
| **Privacy leakage** | High | Low | Local-only training, no PII in datasets |

---

## 10. Appendix

### 10.1 Glossary

| Term | Definition |
|------|------------|
| **Agent** | The orchestration layer that implements the agentic loop |
| **Brain** | Any cognitive processor (local lobes or frontier API) |
| **Lobe** | Specialized local model for a specific cognitive task |
| **Frontier** | State-of-the-art cloud AI models (Claude, GPT-4) |
| **Experience** | Captured input/output pair from a successful run |
| **Skill** | A learned pattern that can be reused for similar tasks |
| **LoRA** | Low-Rank Adaptation, efficient fine-tuning method |

### 10.2 References

- [LoRA: Low-Rank Adaptation of Large Language Models](https://arxiv.org/abs/2106.09685)
- [MLX: Apple's Machine Learning Framework](https://github.com/ml-explore/mlx)
- [Ollama: Local LLM Runtime](https://ollama.ai)
- [A2A Protocol: Agent-to-Agent Communication](https://github.com/google/a2a)

### 10.3 File Structure

```
/pkg/
  agent/                      # Agent Layer
    agent.go                  # Agentic loop implementation
    state.go                  # State management
    brains/
      interface.go            # BrainInterface
      local.go               # LocalBrain wrapper
      frontier.go            # FrontierBrain wrapper
      router.go              # BrainRouter

  learning/                   # Learning Pipeline
    collector/
      experience.go          # Experience capture
      buffer.go              # Experience buffer
    curator/
      filter.go              # Quality filtering
      dedup.go               # Deduplication
      formatter.go           # Training format
    datasets/
      store.go               # Dataset storage
      versioning.go          # Version management
    training/
      scheduler.go           # Training triggers
      trainer.go             # Fine-tuning logic
      evaluator.go           # Model evaluation
    registry/
      models.go              # Model versions
      deployment.go          # Ollama deployment
    skills/
      pattern.go             # Skill patterns
      extractor.go           # Pattern extraction
      matcher.go             # Skill matching

  brain/                      # Existing Brain (unchanged)
    executive.go
    lobes/
    strategy.go
```

---

## Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2026-02-12 | Norman King & Codie | Initial PRD |

---

*This PRD defines the Agentic Brain System, a self-improving AI architecture that combines local specialized models with frontier AI to create a system that gets smarter and cheaper to operate over time.*
