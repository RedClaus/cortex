---
project: Cortex
component: Agents
phase: Build
date_created: 2026-02-01T15:34:31
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:29.474852
---

# OLLAMA-AGENT-INTEGRATION.go

```go
// File: ~/clawd/cortex-brain/pkg/brain/lobes/agent/ollama_agent.go
package agent

import (
    "context"
    "fmt"
    "log"
)

// Ollama agent (uses DeepSeek-Coder-V2-Lite on Pink)
type OllamaAgent struct {
    name   string
    base   string
    model  string
}

// New Ollama agent
func NewOllamaAgent(name, base, model string) *OllamaAgent {
    return &OllamaAgent{
        name:   name,
        base:   base,
        model:  model,
    }
}

// Execute coding task via A2A to Ollama adapter
func (o *OllamaAgent) ExecuteTask(ctx context.Context, task string) (string, error) {
    // Send A2A message to Pink (Ollama adapter)
    // This requires A2A bridge functionality
    // For now, log the task (to be implemented)
    log.Printf("Ollama agent received task: %s", task)

    // TODO: Implement A2A message to Pink's Ollama adapter
    // Message format:
    // {
    //   "agent": "harold",
    //   "target": "ollama-adapter",
    //   "message": {
    //     "task": "coding",
    //     "code": task
    //   }
    // }

    return fmt.Sprintf("Ollama agent executed task: %s", task), nil
}

// Get agent name
func (o *OllamaAgent) Name() string {
    return o.name
}

// Get agent base URL (for debugging)
func (o *OllamaAgent) BaseURL() string {
    return o.base
}

// Get agent model (for debugging)
func (o *OllamaAgent) Model() string {
    return o.model
}
```

---

## HOW TO INTEGRATE

```bash
# 1. On Harold (192.168.1.229)
cd ~/clawd/cortex-brain/pkg/brain/lobes/agent

# 2. Create ollama_agent.go
nano ollama_agent.go
# Paste the code above and save

# 3. Modify registry.go to register Ollama agent
nano registry.go

# 4. Add to init() function:
#    RegisterAgent(NewOllamaAgent(
#        "ollama-pink",
#        "http://192.168.1.186:11434",
#        "deepseek-coder-v2-lite",
#    ))

# 5. Modify orchestrator.go to route small coding tasks to Ollama
nano orchestrator.go

# 6. Add routing logic (see below)

# 7. Rebuild Harold
cd ~/clawd/cortex-brain
go build -o cortex-brain cmd/cortex-brain/main.go

# 8. Restart Harold
sudo systemctl restart cortex-brain
```

---

## ORCHESTRATOR ROUTING LOGIC

```go
// File: ~/clawd/cortex-brain/pkg/brain/lobes/agent/orchestrator.go (modify existing)
package agent

import (
    "context"
    "fmt"
    "strings"
)

// Add to existing Orchestrator struct

// Route task to appropriate agent
func (o *Orchestrator) RouteTask(ctx context.Context, task string) (string, error) {
    // Check if task is small coding task → use Ollama
    if o.isSmallCodingTask(task) {
        log.Printf("Routing small coding task to Ollama agent")
        return o.agents["ollama-pink"].ExecuteTask(ctx, task)
    }

    // Check if task is coding task → use Pink/Red
    if o.isCodingTask(task) {
        log.Printf("Routing coding task to Pink agent")
        return o.agents["pink"].ExecuteTask(ctx, task)
    }

    // Default: Use other agents...
    log.Printf("Routing task to default agent")
    return o.agents["default"].ExecuteTask(ctx, task)
}

// Check if task is small coding task (< 500 lines)
func (o *Orchestrator) isSmallCodingTask(task string) bool {
    // Heuristic: keywords for small coding tasks
    keywords := []string{
        "utility function",
        "helper function",
        "calculate",
        "algorithm",
        "fibonacci",
        "factorial",
        "data structure",
        "simple",
        "sort",
        "stack",
        "queue",
        "linked list",
        "binary search",
        "quick sort",
        "merge sort",
    }

    taskLower := strings.ToLower(task)
    for _, keyword := range keywords {
        if strings.Contains(taskLower, keyword) {
            return true
        }
    }

    return false
}

// Check if task is coding task
func (o *Orchestrator) isCodingTask(task string) bool {
    keywords := []string{
        "write a function",
        "implement",
        "code",
        "program",
        "algorithm",
        "data structure",
    }

    taskLower := strings.ToLower(task)
    for _, keyword := range keywords {
        if strings.Contains(taskLower, keyword) {
            return true
        }
    }

    return false
}
```

---

## A2A MESSAGE FORMAT

```json
{
  "agent": "harold",
  "target": "ollama-adapter",
  "message": {
    "task": "coding",
    "code": "Write a Go function to calculate fibonacci numbers",
    "context": "This is a small utility function for the CortexBrain project"
  }
}
```

---

## TESTING

```bash
# 1. After integrating, send A2A message to Harold
echo '{
  "agent": "harold",
  "target": "ollama-pink",
  "message": "Write a Python function to sort a list"
}' | curl -X POST http://localhost:18802/messages -H "Content-Type: application/json" -d @-

# 2. Check Harold's logs
tail -f ~/clawd/logs/cortex-brain.log

# 3. Verify task is routed to Ollama
# Expected log: "Routing small coding task to Ollama agent"

# 4. Verify Ollama adapter receives task
# Check Pink's logs:
tail -f ~/clawd/ollama-adapter/ollama-client.log
```

---

## EXPECTED BEHAVIOR

1. **Small coding task** → Routed to Ollama (DeepSeek-Coder-V2-Lite)
   - Faster (50-70 tok/s on RTX 3090)
   - No API costs
   - Good for utility functions, algorithms, basic I/O

2. **Large coding task** → Routed to Pink/Red (GLM-4.7)
   - More capable (4.7B parameters)
   - Better for complex features, architectural changes
   - API costs apply

3. **Non-coding task** → Routed to other agents
   - Research → Harold
   - Testing → Pink/Red
   - Coordination → Harold

---

## SUCCESS CRITERIA

```
✅ Ollama agent registered in Harold
✅ Task routing updated (small tasks → Ollama)
✅ Small coding tasks routed to Ollama
✅ Large coding tasks routed to Pink/Red
✅ A2A messages flow correctly (Harold → Pink Ollama adapter)
✅ Generated code quality validated
✅ Performance measured (50-70 tok/s expected)
✅ Cost savings documented (Ollama: $0, GLM-4.7: API costs)
```

---

## NOTES

- Ollama agent is lightweight (no complex orchestration)
- Task routing uses keyword heuristics (can be improved)
- A2A integration required (Harold ↔ Pink Ollama adapter)
- Offloads 20-40% of small coding tasks from Pink/Red
- Reduces API costs for small tasks
- Uses existing RTX 3090 hardware (no new hardware needed)