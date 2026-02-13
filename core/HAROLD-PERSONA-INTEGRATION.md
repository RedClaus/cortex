---
project: Cortex
component: Brain Kernel
phase: Experiment
date_created: 2026-02-06T16:38:48
source: ServerProjectsMac
librarian_indexed: 2026-02-06T16:45:16.106730
---

# Harold's Gateway Persona Integration (NEW)

**File:** `cortex-gateway/internal/brain/persona.go` (7.8KB)

**Persona Client Functions:**
- `ListPersonas()` — Get all personas from CortexBrain
- `GetPersona(personaID)` — Get full persona details
- `GetPersonasByCategory(category)` — Filter by category
- `GetPersonasCategories()` — List all categories
- `GetPersonasRoles()` — List all roles
- `GetPersonaIdentity(personaID)` — Get just identity prompt
- `GetPersonaSystemPrompt(personaID)` — Get complete system prompt
- `InferWithPersona(req, personaID)` — Inference with persona injection

**Usage in Go:**
```go
import "github.com/cortexhub/cortex-gateway/internal/brain"

func main() {
    brainClient := brain.NewClient(cfg.CortexBrain)
    
    // List personas
    personas, err := brainClient.ListPersonas()
    
    // Load Harold's persona
    harold, err := brainClient.GetPersona("harold")
    
    // Get system prompt
    systemPrompt, err := brainClient.GetPersonaSystemPrompt("harold")
    
    // Infer with persona
    response, err := brainClient.InferWithPersona(req, "harold")
}
```

**Demo Script:** `scripts/harold-persona-demo.py`
- Lists all available personas
- Shows full persona details
- Generates system prompts
- Tests chat with persona injection
