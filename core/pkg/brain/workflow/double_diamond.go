// Package workflow provides the Double Diamond thinking methodology for CortexBrain.
// The Double Diamond is a design methodology with 4 phases:
// Discover (Probe) → Define (Grasp) → Develop (Tangle) → Deliver (Ink)
package workflow

import (
	"time"
)

// Phase represents a workflow phase.
type Phase string

const (
	// PhaseDiscover (Probe) - Divergent research and exploration
	PhaseDiscover Phase = "discover"

	// PhaseDefine (Grasp) - Convergent consensus building
	PhaseDefine Phase = "define"

	// PhaseDevelop (Tangle) - Divergent implementation
	PhaseDevelop Phase = "develop"

	// PhaseDeliver (Ink) - Convergent validation
	PhaseDeliver Phase = "deliver"
)

// PhaseAliases maps alternative phase names.
var PhaseAliases = map[string]Phase{
	"probe":    PhaseDiscover,
	"discover": PhaseDiscover,
	"research": PhaseDiscover,
	"grasp":    PhaseDefine,
	"define":   PhaseDefine,
	"scope":    PhaseDefine,
	"tangle":   PhaseDevelop,
	"develop":  PhaseDevelop,
	"build":    PhaseDevelop,
	"ink":      PhaseDeliver,
	"deliver":  PhaseDeliver,
	"ship":     PhaseDeliver,
}

// PhaseInfo describes a workflow phase.
type PhaseInfo struct {
	Phase       Phase
	Name        string
	Description string
	Mode        string // "divergent" or "convergent"
	Activities  []string
	Output      string
	Emoji       string
}

// PhaseInfoMap provides details about each phase.
var PhaseInfoMap = map[Phase]PhaseInfo{
	PhaseDiscover: {
		Phase:       PhaseDiscover,
		Name:        "Discover",
		Description: "Divergent research and exploration",
		Mode:        "divergent",
		Activities: []string{
			"Multi-source research",
			"Broad ecosystem analysis",
			"Technology comparison",
			"Best practices research",
			"Community insights",
		},
		Output: "Research synthesis document",
		Emoji:  "🔍",
	},
	PhaseDefine: {
		Phase:       PhaseDefine,
		Name:        "Define",
		Description: "Convergent consensus building",
		Mode:        "convergent",
		Activities: []string{
			"Synthesize research findings",
			"Build consensus on approach",
			"Define requirements clearly",
			"Identify constraints",
			"Establish success criteria",
		},
		Output: "Consensus document with requirements",
		Emoji:  "🎯",
	},
	PhaseDevelop: {
		Phase:       PhaseDevelop,
		Name:        "Develop",
		Description: "Divergent implementation",
		Mode:        "divergent",
		Activities: []string{
			"Code generation",
			"Implementation with quality gates",
			"Testing and validation",
			"Security review",
			"Performance optimization",
		},
		Output: "Implementation with validation report",
		Emoji:  "🛠️",
	},
	PhaseDeliver: {
		Phase:       PhaseDeliver,
		Name:        "Deliver",
		Description: "Convergent final validation",
		Mode:        "convergent",
		Activities: []string{
			"Quality assurance",
			"Final synthesis",
			"Documentation",
			"Delivery certification",
			"User acceptance",
		},
		Output: "Final delivery document",
		Emoji:  "✅",
	},
}

// WorkflowState tracks the current state of a Double Diamond workflow.
type WorkflowState struct {
	ID          string                 `json:"id"`
	CurrentPhase Phase                 `json:"current_phase"`
	StartedAt   time.Time              `json:"started_at"`
	PhaseStarted time.Time             `json:"phase_started"`
	Phases      map[Phase]*PhaseState  `json:"phases"`
	Context     map[string]interface{} `json:"context"`
}

// PhaseState tracks the state of a single phase.
type PhaseState struct {
	Phase       Phase     `json:"phase"`
	Status      string    `json:"status"` // "pending", "in_progress", "completed", "failed"
	StartedAt   time.Time `json:"started_at,omitempty"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
	Output      string    `json:"output,omitempty"`
	QualityGate bool      `json:"quality_gate"` // Did it pass the quality gate?
}

// WorkflowManager manages Double Diamond workflows.
type WorkflowManager struct {
	workflows map[string]*WorkflowState
}

// NewWorkflowManager creates a new workflow manager.
func NewWorkflowManager() *WorkflowManager {
	return &WorkflowManager{
		workflows: make(map[string]*WorkflowState),
	}
}

// StartWorkflow begins a new Double Diamond workflow.
func (m *WorkflowManager) StartWorkflow(id string) *WorkflowState {
	now := time.Now()
	ws := &WorkflowState{
		ID:           id,
		CurrentPhase: PhaseDiscover,
		StartedAt:    now,
		PhaseStarted: now,
		Phases: map[Phase]*PhaseState{
			PhaseDiscover: {Phase: PhaseDiscover, Status: "in_progress", StartedAt: now},
			PhaseDefine:   {Phase: PhaseDefine, Status: "pending"},
			PhaseDevelop:  {Phase: PhaseDevelop, Status: "pending"},
			PhaseDeliver:  {Phase: PhaseDeliver, Status: "pending"},
		},
		Context: make(map[string]interface{}),
	}
	m.workflows[id] = ws
	return ws
}

// GetWorkflow retrieves a workflow by ID.
func (m *WorkflowManager) GetWorkflow(id string) *WorkflowState {
	return m.workflows[id]
}

// AdvancePhase moves the workflow to the next phase.
func (m *WorkflowManager) AdvancePhase(id string, qualityGatePassed bool) (*WorkflowState, error) {
	ws := m.workflows[id]
	if ws == nil {
		return nil, nil
	}

	now := time.Now()

	// Complete current phase
	currentPhaseState := ws.Phases[ws.CurrentPhase]
	currentPhaseState.CompletedAt = now
	currentPhaseState.QualityGate = qualityGatePassed
	if qualityGatePassed {
		currentPhaseState.Status = "completed"
	} else {
		currentPhaseState.Status = "failed"
	}

	// Determine next phase
	nextPhase := m.getNextPhase(ws.CurrentPhase)
	if nextPhase == "" {
		return ws, nil // Workflow complete
	}

	// Start next phase
	ws.CurrentPhase = nextPhase
	ws.PhaseStarted = now
	ws.Phases[nextPhase].Status = "in_progress"
	ws.Phases[nextPhase].StartedAt = now

	return ws, nil
}

// getNextPhase returns the next phase in the workflow.
func (m *WorkflowManager) getNextPhase(current Phase) Phase {
	order := []Phase{PhaseDiscover, PhaseDefine, PhaseDevelop, PhaseDeliver}
	for i, p := range order {
		if p == current && i < len(order)-1 {
			return order[i+1]
		}
	}
	return ""
}

// DeterminePhase suggests the appropriate phase for a task.
func (m *WorkflowManager) DeterminePhase(input string) Phase {
	// Keywords that indicate each phase
	discoverKeywords := []string{"research", "explore", "investigate", "understand", "learn", "study"}
	defineKeywords := []string{"define", "scope", "requirements", "plan", "design", "architect"}
	developKeywords := []string{"build", "implement", "code", "create", "develop", "write"}
	deliverKeywords := []string{"ship", "deploy", "release", "review", "test", "validate"}

	input = string(input)
	for _, kw := range discoverKeywords {
		if contains(input, kw) {
			return PhaseDiscover
		}
	}
	for _, kw := range defineKeywords {
		if contains(input, kw) {
			return PhaseDefine
		}
	}
	for _, kw := range developKeywords {
		if contains(input, kw) {
			return PhaseDevelop
		}
	}
	for _, kw := range deliverKeywords {
		if contains(input, kw) {
			return PhaseDeliver
		}
	}

	// Default to develop for coding tasks
	return PhaseDevelop
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
