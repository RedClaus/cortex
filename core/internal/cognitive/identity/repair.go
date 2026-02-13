package identity

import (
	"fmt"
	"strings"
)

// Repairer generates repair plans for identity drift.
type Repairer struct {
	creed  *CreedManager
	config *Config
}

// NewRepairer creates a new identity repairer.
func NewRepairer(creed *CreedManager, cfg *Config) *Repairer {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	return &Repairer{
		creed:  creed,
		config: cfg,
	}
}

// GenerateRepairPlan creates a repair plan based on drift analysis.
func (r *Repairer) GenerateRepairPlan(analysis *DriftAnalysis) *RepairPlan {
	if analysis == nil {
		return nil
	}

	plan := &RepairPlan{
		Actions:          []RepairAction{},
		RequiresApproval: !r.config.AutoRepair,
	}

	// Determine severity
	plan.Severity = r.determineSeverity(analysis.OverallDrift)

	// Generate actions based on per-statement drift
	for statement, drift := range analysis.PerStatementDrift {
		if drift > r.config.DriftThreshold {
			action := r.generateActionForStatement(statement, drift)
			plan.Actions = append(plan.Actions, action)
		}
	}

	// If no specific actions, add a general reinforcement
	if len(plan.Actions) == 0 && analysis.OverallDrift > r.config.DriftThreshold {
		plan.Actions = append(plan.Actions, RepairAction{
			Type:      "reinforce",
			Statement: "general identity",
			Injection: r.generateGeneralReinforcement(),
			Priority:  0.5,
		})
	}

	// Sort by priority (highest first)
	r.sortActionsByPriority(plan.Actions)

	// Generate reason
	plan.Reason = r.generateReason(analysis, plan)

	return plan
}

// determineSeverity categorizes drift severity.
func (r *Repairer) determineSeverity(drift float64) string {
	switch {
	case drift < 0.2:
		return "low"
	case drift < 0.4:
		return "medium"
	case drift < 0.6:
		return "high"
	default:
		return "critical"
	}
}

// generateActionForStatement creates a repair action for a specific statement.
func (r *Repairer) generateActionForStatement(statement string, drift float64) RepairAction {
	lowerStatement := strings.ToLower(statement)

	action := RepairAction{
		Statement: statement,
		Priority:  drift,
	}

	// Determine action type based on drift severity
	if drift > 0.5 {
		action.Type = "anchor"
	} else {
		action.Type = "reinforce"
	}

	// Generate appropriate injection
	action.Injection = r.generateInjectionForStatement(lowerStatement)

	return action
}

// generateInjectionForStatement creates injection text for a statement.
func (r *Repairer) generateInjectionForStatement(lowerStatement string) string {
	switch {
	case strings.Contains(lowerStatement, "cortex") && strings.Contains(lowerStatement, "cognitive"):
		return "As an AI that emulates human cognitive processes, I approach this by"

	case strings.Contains(lowerStatement, "privacy") || strings.Contains(lowerStatement, "locally"):
		return "Respecting your privacy and operating locally, I"

	case strings.Contains(lowerStatement, "uncertainty") || strings.Contains(lowerStatement, "fabricate"):
		return "Being transparent about my limitations, I should note that"

	case strings.Contains(lowerStatement, "autonomy") || strings.Contains(lowerStatement, "manipulation"):
		return "Supporting your decision-making, I'd suggest considering"

	case strings.Contains(lowerStatement, "reflection") && strings.Contains(lowerStatement, "improve"):
		return "Through continuous self-reflection, I've learned that"

	default:
		return "Staying true to my core principles, I"
	}
}

// generateGeneralReinforcement creates a general identity reinforcement.
func (r *Repairer) generateGeneralReinforcement() string {
	return "As Cortex, an AI assistant designed to emulate human cognitive processes while prioritizing your privacy and autonomy, I"
}

// generateReason explains the repair plan.
func (r *Repairer) generateReason(analysis *DriftAnalysis, plan *RepairPlan) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("Drift score %.2f exceeds threshold %.2f.",
		analysis.OverallDrift, r.config.DriftThreshold))

	if len(plan.Actions) == 1 {
		parts = append(parts, fmt.Sprintf("One creed statement requires reinforcement: %s",
			plan.Actions[0].Statement))
	} else if len(plan.Actions) > 1 {
		parts = append(parts, fmt.Sprintf("%d creed statements require reinforcement.",
			len(plan.Actions)))
	}

	parts = append(parts, fmt.Sprintf("Severity: %s.", plan.Severity))

	if analysis.DriftTrend == "increasing" {
		parts = append(parts, "Drift trend is increasing - recommend prompt intervention.")
	}

	return strings.Join(parts, " ")
}

// sortActionsByPriority sorts actions by priority descending.
func (r *Repairer) sortActionsByPriority(actions []RepairAction) {
	// Simple bubble sort (small list)
	for i := 0; i < len(actions); i++ {
		for j := i + 1; j < len(actions); j++ {
			if actions[j].Priority > actions[i].Priority {
				actions[i], actions[j] = actions[j], actions[i]
			}
		}
	}
}

// ApplyRepairAction formats a response with repair injection.
func (r *Repairer) ApplyRepairAction(action *RepairAction, response string) string {
	if action == nil || action.Injection == "" {
		return response
	}

	// For anchor actions, prepend the injection
	if action.Type == "anchor" {
		return action.Injection + " " + response
	}

	// For reinforce actions, look for natural insertion points
	sentences := strings.SplitN(response, ". ", 2)
	if len(sentences) > 1 {
		return sentences[0] + ". " + action.Injection + " " + sentences[1]
	}

	// Fallback: prepend
	return action.Injection + " " + response
}

// ValidateRepairPlan checks if a repair plan is valid.
func (r *Repairer) ValidateRepairPlan(plan *RepairPlan) error {
	if plan == nil {
		return fmt.Errorf("repair plan is nil")
	}

	if len(plan.Actions) == 0 {
		return fmt.Errorf("repair plan has no actions")
	}

	for i, action := range plan.Actions {
		if action.Type == "" {
			return fmt.Errorf("action %d has no type", i)
		}
		if action.Statement == "" {
			return fmt.Errorf("action %d has no statement", i)
		}
	}

	return nil
}
