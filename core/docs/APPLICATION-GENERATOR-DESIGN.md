---
project: Cortex
component: Docs
phase: Ideation
date_created: 2026-02-12T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-12T23:25:41.806731
---

# Application Generator Design

## Goal
Enable requests like:
> "Create an application called 'TaskMaster' with this PRD: [prd]"

The system should:
1. Parse and understand the PRD
2. Plan the implementation
3. Execute tools autonomously (create files, folders, install deps)
4. Ask clarifying questions when confused
5. Report progress and completion

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    ApplicationGenerator                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────┐  │
│  │  PRD Parser  │───▶│   Planner    │───▶│  Task Executor   │  │
│  │              │    │  (Planning   │    │  (Agentic Loop)  │  │
│  │  - Parse     │    │   Lobe)      │    │                  │  │
│  │  - Extract   │    │              │    │  - Execute       │  │
│  │    features  │    │  - Break     │    │  - Observe       │  │
│  │  - Identify  │    │    into      │    │  - Decide        │  │
│  │    entities  │    │    tasks     │    │  - Ask/Continue  │  │
│  │  - Extract   │    │  - Order by  │    │                  │  │
│  │    tech      │    │    deps      │    │                  │  │
│  └──────────────┘    └──────────────┘    └──────────────────┘  │
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                     Task Queue                            │  │
│  │  [Setup] → [Models] → [API] → [UI] → [Tests] → [Deploy]  │  │
│  │     ✓         ▶         ○       ○        ○         ○      │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                  Clarification Engine                     │  │
│  │  "I notice the PRD mentions 'user auth' but doesn't      │  │
│  │   specify the method. Should I use:                      │  │
│  │   1. JWT tokens                                          │  │
│  │   2. Session-based auth                                  │  │
│  │   3. OAuth (Google/GitHub)?"                             │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## Components

### 1. PRD Parser
Extracts structured information from a PRD:
- **Application Name** - From request
- **Features** - User-facing functionality
- **Entities/Models** - Data structures needed
- **Tech Stack** - Languages, frameworks, databases
- **Constraints** - Security requirements, performance needs

### 2. Application Planner
Uses PlanningLobe to break the PRD into executable tasks:
```
1. Initialize project structure
2. Set up development environment
3. Create data models
4. Implement API endpoints
5. Build UI components
6. Add authentication
7. Write tests
8. Configure deployment
```

### 3. Task Executor (Agentic Loop)
For each task:
1. **Execute** - Call appropriate tools (write_file, run_command, etc.)
2. **Observe** - Check output for success/failure
3. **Decide** - Continue, retry with fix, or ask for clarification
4. **Report** - Update user on progress

### 4. Clarification Engine
Detects ambiguity and asks focused questions:
- Missing technical decisions
- Conflicting requirements
- Unclear scope
- Missing dependencies

---

## Implementation Phases

### Phase 1: Core Infrastructure
- [ ] Add `ApplicationGeneratorHandler` to pinky_compat.go
- [ ] Create `ParsePRD()` function to extract structured data
- [ ] Implement `TaskQueue` with status tracking

### Phase 2: Agentic Loop
- [ ] Create `AgenticStrategy` for multi-step execution
- [ ] Add tool result observation and decision logic
- [ ] Implement retry with self-correction

### Phase 3: Clarification
- [ ] Add ambiguity detection
- [ ] Create question generation
- [ ] Handle user responses and continue

### Phase 4: IDE Integration
- [ ] Add `ide_open` tool to open files in IDE
- [ ] Add `ide_run` tool to execute IDE commands
- [ ] Support for VS Code, Cursor, etc.

---

## Request Format

```json
{
  "type": "create_application",
  "name": "TaskMaster",
  "prd": "## Overview\nTaskMaster is a task management app...\n\n## Features\n- User authentication\n- Create/edit/delete tasks\n...",
  "tech_hints": ["typescript", "react", "postgres"]
}
```

---

## Response Format

### During Execution (streaming)
```json
{
  "status": "in_progress",
  "current_task": "Creating project structure",
  "progress": { "completed": 1, "total": 8 },
  "actions_taken": [
    { "tool": "run_command", "params": { "command": "mkdir -p TaskMaster/src" }, "success": true }
  ]
}
```

### Clarification Needed
```json
{
  "status": "needs_clarification",
  "question": "The PRD mentions 'user auth' but doesn't specify the method. Which should I use?",
  "options": ["JWT tokens", "Session-based auth", "OAuth (Google/GitHub)"],
  "context": "This affects how I structure the authentication module."
}
```

### Completion
```json
{
  "status": "complete",
  "summary": "Created TaskMaster application with 12 files across 5 directories.",
  "files_created": ["TaskMaster/src/index.ts", "..."],
  "next_steps": ["Run 'npm install' to install dependencies", "Run 'npm run dev' to start"]
}
```

---

## New Tools Needed

```go
// IDE Tools
{
    Name:        "ide_open",
    Description: "Open a file in the user's IDE (VS Code, Cursor, etc.)",
    Parameters: []Parameter{
        {Name: "path", Type: "string", Description: "Path to open", Required: true},
        {Name: "line", Type: "integer", Description: "Line number to jump to", Required: false},
    },
},
{
    Name:        "scaffold_project",
    Description: "Create a project structure from a template",
    Parameters: []Parameter{
        {Name: "name", Type: "string", Description: "Project name", Required: true},
        {Name: "template", Type: "string", Description: "Template: react, nextjs, express, fastapi, etc.", Required: true},
        {Name: "path", Type: "string", Description: "Where to create the project", Required: false},
    },
},
```

---

## Detection Pattern

Add to `detectToolFromQuery()`:

```go
// Application creation detection
if containsAny("create", "build", "make", "generate") &&
   containsAny("application", "app", "project") {
    // Check for PRD or detailed requirements
    if containsAny("prd", "requirements", "spec", "with these features") {
        return "create_application", extractAppParams(input)
    }
}
```
