---
project: Cortex
component: Brain Kernel
phase: Ideation
date_created: 2026-01-31T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:29.846247
---

# CortexBrain as Mission Control — Detailed Build Plan
**Created:** 2026-01-31
**Status:** PLANNED — awaiting prioritization
**Inspired by:** Bhanu Teja's Mission Control post (10-agent squad on OpenClaw)

---

## Vision

Use CortexBrain itself as the coordination substrate for the Swarm V2 multi-agent system. Instead of building a separate task board (SQLite, Convex, etc.), leverage existing CortexBrain subsystems — Blackboard, Neural Bus, MemCell, Sleep Cycle — to provide task management, notifications, shared context, and automated standups. The brain becomes Mission Control.

---

## Why CortexBrain (Not a Separate Tool)

| Need | CortexBrain Subsystem | Status |
|---|---|---|
| Shared task state | Blackboard (key-value store, all lobes read/write) | ✅ Built |
| Notifications / @mentions | Neural Bus (pub/sub events) | ✅ Built |
| Persistent shared knowledge | MemCell / Memory Lobe (semantic search) | ✅ Built |
| Distributed state consistency | State Store with Raft consensus | ✅ Built |
| Auto-summarize / daily standup | Sleep Cycle (consolidation) | ✅ Built |
| External agent access (HTTP API) | Brain-as-Service (:18892) | 🔨 Track A |
| Task schema + REST endpoints | New code | 📝 This plan |
| Dashboard UI for task board | New panel in Neural Monitor or standalone | 📝 This plan |

**Key insight:** Bhanu needed 3 separate systems (Convex DB + delivery daemon + React app). We can do it with CortexBrain alone because the architecture already maps 1:1.

---

## Architecture

```
┌──────────────────────────────────────────────────┐
│                  Swarm Agents                     │
│  Harold  │  Pink  │  Red  │  CTs  │  Albert      │
└────┬─────┴───┬────┴───┬───┴───┬───┴──────┬───────┘
     │         │        │       │          │
     └─────────┴────────┴───────┴──────────┘
                    │ HTTP/curl
                    ▼
     ┌──────────────────────────────┐
     │   CortexBrain API (:18892)  │
     │   (Brain-as-Service on Pink) │
     └──────────┬───────────────────┘
                │
     ┌──────────▼───────────────────┐
     │      Internal Routing        │
     ├──────────────────────────────┤
     │  Blackboard  = Task Board    │
     │  Neural Bus  = Notifications │
     │  MemCell     = Shared Docs   │
     │  Sleep Cycle = Auto-Standup  │
     │  Executive   = Task Routing  │
     │  Metacog     = Quality Track │
     └──────────────────────────────┘
```

---

## Data Model

### Task
```go
type Task struct {
    ID          string            `json:"id"`          // UUID
    Title       string            `json:"title"`
    Description string            `json:"description"`
    Status      TaskStatus        `json:"status"`      // inbox|assigned|in_progress|review|done|blocked
    Priority    string            `json:"priority"`    // p0|p1|p2|p3
    AssigneeIDs []string          `json:"assignee_ids"`// agent names: "pink", "red", "harold"
    CreatedBy   string            `json:"created_by"`  // who created it
    CreatedAt   time.Time         `json:"created_at"`
    UpdatedAt   time.Time         `json:"updated_at"`
    DueDate     *time.Time        `json:"due_date,omitempty"`
    Tags        []string          `json:"tags"`        // "cortex-journey", "swarm", "brain"
    ParentID    *string           `json:"parent_id,omitempty"` // subtask support
    Meta        map[string]string `json:"meta,omitempty"`
}

type TaskStatus string
const (
    StatusInbox      TaskStatus = "inbox"
    StatusAssigned   TaskStatus = "assigned"
    StatusInProgress TaskStatus = "in_progress"
    StatusReview     TaskStatus = "review"
    StatusDone       TaskStatus = "done"
    StatusBlocked    TaskStatus = "blocked"
)
```

### Comment (on a task)
```go
type Comment struct {
    ID        string    `json:"id"`
    TaskID    string    `json:"task_id"`
    AuthorID  string    `json:"author_id"`  // agent name
    Content   string    `json:"content"`    // markdown
    Mentions  []string  `json:"mentions"`   // parsed @mentions
    CreatedAt time.Time `json:"created_at"`
}
```

### Agent Registry
```go
type AgentEntry struct {
    ID         string    `json:"id"`         // "pink", "red", "harold"
    Name       string    `json:"name"`       // display name
    Role       string    `json:"role"`       // "Backend Developer", "Dashboard Builder"
    Status     string    `json:"status"`     // "idle"|"active"|"blocked"|"offline"
    CurrentTask *string  `json:"current_task,omitempty"`
    LastSeen   time.Time `json:"last_seen"`
    SessionKey string    `json:"session_key"`
    Endpoint   string    `json:"endpoint"`   // IP:port for direct contact
}
```

### Notification
```go
type Notification struct {
    ID          string    `json:"id"`
    TargetAgent string    `json:"target_agent"`
    SourceAgent string    `json:"source_agent"`
    TaskID      string    `json:"task_id"`
    Content     string    `json:"content"`
    Type        string    `json:"type"`       // "mention"|"assignment"|"status_change"|"comment"
    Delivered   bool      `json:"delivered"`
    CreatedAt   time.Time `json:"created_at"`
}
```

### Document (shared deliverables/research)
```go
type Document struct {
    ID        string    `json:"id"`
    Title     string    `json:"title"`
    Content   string    `json:"content"`    // markdown
    Type      string    `json:"type"`       // "deliverable"|"research"|"spec"|"protocol"
    TaskID    *string   `json:"task_id,omitempty"`
    AuthorID  string    `json:"author_id"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

---

## API Endpoints (Brain-as-Service :18892)

### Tasks
```
GET    /api/tasks                    — List tasks (filter: ?status=in_progress&assignee=pink)
POST   /api/tasks                    — Create task
GET    /api/tasks/{id}               — Get task with comments
PATCH  /api/tasks/{id}               — Update task (status, assignees, etc.)
DELETE /api/tasks/{id}               — Archive task (soft delete)
```

### Comments
```
GET    /api/tasks/{id}/comments      — List comments on task
POST   /api/tasks/{id}/comments      — Add comment (auto-parses @mentions)
```

### Agents
```
GET    /api/agents                   — List all registered agents + status
PATCH  /api/agents/{id}              — Update agent status/current task
POST   /api/agents/{id}/heartbeat    — Agent check-in (updates last_seen)
```

### Notifications
```
GET    /api/notifications/{agent_id} — Get undelivered notifications for agent
POST   /api/notifications/{id}/ack   — Mark notification as delivered
```

### Documents
```
GET    /api/documents                — List documents (?task_id=X&type=research)
POST   /api/documents                — Create document
GET    /api/documents/{id}           — Get document
PATCH  /api/documents/{id}           — Update document
```

### Standup
```
GET    /api/standup                  — Get today's auto-generated standup
GET    /api/standup?date=2026-01-31  — Get standup for specific date
```

---

## Internal Wiring (CortexBrain Subsystems)

### Blackboard Integration
- Tasks stored as Blackboard entries: `task:{id}` → serialized Task JSON
- Agent status: `agent:{id}` → AgentEntry JSON
- Indexes maintained: `tasks:by_status:{status}` → list of task IDs
- Comments: `task:{id}:comments` → ordered list

### Neural Bus Events
- `task.created` → notify assignees
- `task.status_changed` → notify subscribers
- `comment.added` → notify task subscribers + @mentioned agents
- `agent.heartbeat` → update agent status on Blackboard
- `task.blocked` → alert Harold (escalation)

### MemCell Integration
- Documents stored in MemCell with semantic embeddings
- Agents can search shared knowledge: "What did we learn about competitor pricing?"
- Task context automatically indexed for retrieval

### Sleep Cycle Integration
- End-of-day: Sleep Cycle reads all tasks updated today
- Generates standup summary (completed, in progress, blocked, needs review)
- Consolidates: move stale "in_progress" tasks → "blocked" if no activity in 24h
- Prunes: archive "done" tasks older than 7 days

### Executive Lobe Integration
- Task routing: when a new task arrives without assignee, Executive suggests assignment based on agent roles/load
- Escalation: if task blocked >2 heartbeats, Executive notifies Harold
- Rebalancing: if one agent has 5 tasks and another has 0, suggest redistribution

### Metacognition Integration
- Track task quality: how many revisions before "done"?
- Track agent reliability: task completion rate, average time
- Flag patterns: "Pink has been blocked 3 times on the same dependency"

---

## Build Phases

### Phase 1: Foundation (1-2 days)
**Who:** Pink (backend)
**Deliverables:**
- [ ] Task struct + CRUD operations on Blackboard
- [ ] Comment struct + CRUD
- [ ] Agent registry struct + heartbeat endpoint
- [ ] 5 REST endpoints: tasks CRUD, comments, agents
- [ ] Wire into existing brain-as-service HTTP server (:18892)
- [ ] Basic tests

**Exit criteria:** `curl http://pink:18892/api/tasks` returns empty list, can create/update/list tasks via curl.

### Phase 2: Notifications + Neural Bus (1 day)
**Who:** Pink (backend)
**Deliverables:**
- [ ] Notification struct + storage
- [ ] @mention parser (scan comment content for `@agent_name`)
- [ ] Neural Bus event emission on task/comment changes
- [ ] Notification delivery endpoint
- [ ] Thread subscription: commenting on a task auto-subscribes you

**Exit criteria:** Create task assigned to "red", check `GET /api/notifications/red` shows the assignment notification.

### Phase 3: Agent Integration (1 day)
**Who:** Harold (coordination) + all agents
**Deliverables:**
- [ ] Update all agent heartbeat prompts to check Mission Control
- [ ] Heartbeat flow: wake → `GET /notifications/{me}` → `GET /tasks?assignee={me}&status=assigned` → do work → `PATCH /tasks/{id}` → sleep
- [ ] Harold's delegation flow: create task via API instead of bridge message
- [ ] Stagger heartbeats: Pink :00, Red :05, CTs spread :10-:50

**Exit criteria:** Harold creates a task, Pink picks it up on next heartbeat, posts progress comment, marks done. Full lifecycle via API.

### Phase 4: Sleep Cycle Standup (0.5 day)
**Who:** Pink (backend)
**Deliverables:**
- [ ] Sleep Cycle hook: read today's task changes at 23:00
- [ ] Generate standup markdown (completed, in progress, blocked, needs review)
- [ ] Store as Document (type: "standup")
- [ ] Cron on Albert: fetch standup, send to Norman's Telegram

**Exit criteria:** Norman gets nightly standup digest on Telegram with accurate task summary.

### Phase 5: Dashboard (1-2 days)
**Who:** Red (frontend)
**Deliverables:**
- [ ] Task Board tab in Neural Monitor (or standalone page)
- [ ] Kanban columns: Inbox → Assigned → In Progress → Review → Done
- [ ] Agent status cards (name, role, current task, last seen)
- [ ] Activity feed (recent comments, status changes)
- [ ] Click task → expand with full comments + context
- [ ] Design: teal/off-white/dark text, NO purple, NO gradients

**Exit criteria:** Open dashboard, see all tasks in kanban view, click to expand, see agent status.

### Phase 6: MemCell + Smart Features (1 day, stretch goal)
**Who:** Pink (backend)
**Deliverables:**
- [ ] Documents stored in MemCell with embeddings
- [ ] Search endpoint: `GET /api/documents/search?q=competitor+pricing`
- [ ] Executive lobe auto-assignment suggestions
- [ ] Metacognition quality tracking
- [ ] Blocked task escalation alerts

**Exit criteria:** Agent can search shared documents semantically, Executive suggests assignees for unassigned tasks.

---

## Total Estimated Effort
- **Minimum viable (Phases 1-3):** 3-4 days — task board works, agents use it
- **Full system (Phases 1-5):** 5-7 days — dashboard, standups, notifications
- **Smart features (Phase 6):** +1 day — semantic search, auto-assignment

---

## Dependencies
- Brain-as-service running on Pink (:18892) — **Track A prerequisite**
- CortexBrain compiles clean — **✅ DONE (fixed tonight)**
- Bridge operational for Harold coordination — **currently fragile**
- All agents have curl access to Pink:18892 — **network verified**

---

## Success Metrics
- Harold stops being a relay — creates tasks via API, agents self-serve
- Zero "what's Pink working on?" questions — visible on dashboard
- Blocked tasks surface within 30 min (2 heartbeat cycles)
- Daily standup accuracy >90% (matches actual work done)
- Agent utilization visible: who's idle, who's overloaded

---

## How This Validates CortexBrain
This is the first real-world use of CortexBrain as **infrastructure**, not just a chatbot:
- Blackboard = coordination state (not just chat context)
- Neural Bus = event-driven notifications (not just lobe-to-lobe)
- MemCell = team knowledge base (not just personal memory)
- Sleep Cycle = operational automation (not just memory consolidation)
- Executive = workflow management (not just response planning)

If CortexBrain can run Mission Control for its own build team, that's the strongest possible demo of the architecture.
