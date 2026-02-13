package sleep

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// generateProposals performs Phase 3: Proposal Generation.
// This creates actionable personality changes based on insights.
func (sm *SleepManager) generateProposals(ctx context.Context, insights []ReflectionInsight) ([]PersonalityProposal, error) {
	proposals := []PersonalityProposal{}

	currentPersonality, err := sm.personality.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load personality: %w", err)
	}

	for _, insight := range insights {
		if !insight.IsActionable() {
			continue
		}

		proposal := sm.createProposalFromInsight(insight, currentPersonality)
		if proposal != nil {
			proposals = append(proposals, *proposal)
		}
	}

	// Prioritize and limit proposals
	proposals = sm.prioritizeProposals(proposals)

	// Enforce safety constraints
	proposals = sm.enforceConstraints(proposals, currentPersonality)

	sm.log.Debug("[Sleep] Proposal generation complete: proposals=%d", len(proposals))

	return proposals, nil
}

// createProposalFromInsight generates a proposal from a single insight.
func (sm *SleepManager) createProposalFromInsight(insight ReflectionInsight, current *Personality) *PersonalityProposal {
	proposal := &PersonalityProposal{
		ID:         generateID(),
		Evidence:   insight.Evidence,
		Confidence: insight.Confidence,
		Reversible: true,
		CreatedAt:  time.Now(),
		Changes:    []PersonalityChange{},
	}

	switch insight.Category {
	case "weakness":
		return sm.createWeaknessProposal(insight, current, proposal)
	case "opportunity":
		return sm.createOpportunityProposal(insight, current, proposal)
	default:
		return nil
	}
}

// createWeaknessProposal creates a proposal to address a weakness.
func (sm *SleepManager) createWeaknessProposal(insight ReflectionInsight, current *Personality, proposal *PersonalityProposal) *PersonalityProposal {
	proposal.Type = "trait_adjustment"

	for _, trait := range insight.ActionableFor {
		switch trait {
		case "patience":
			oldValue := current.Traits.Patience
			newValue := math.Min(1.0, oldValue+0.1)
			if newValue != oldValue {
				proposal.Changes = append(proposal.Changes, PersonalityChange{
					Path:     "traits.patience",
					OldValue: oldValue,
					NewValue: newValue,
					Reason:   "User frustration suggests more patience needed",
				})
			}

		case "verbosity", "thoroughness":
			oldValue := current.Traits.Verbosity
			newValue := math.Min(1.0, oldValue+0.1)
			if newValue != oldValue {
				proposal.Changes = append(proposal.Changes, PersonalityChange{
					Path:     "traits.verbosity",
					OldValue: oldValue,
					NewValue: newValue,
					Reason:   "User confusion suggests more detailed explanations needed",
				})
			}

		case "directness", "clarity":
			oldValue := current.Traits.Directness
			newValue := math.Min(1.0, oldValue+0.1)
			if newValue != oldValue {
				proposal.Changes = append(proposal.Changes, PersonalityChange{
					Path:     "traits.directness",
					OldValue: oldValue,
					NewValue: newValue,
					Reason:   "User confusion suggests more direct communication needed",
				})
			}

		case "confidence", "accuracy":
			oldValue := current.Traits.Confidence
			newValue := math.Max(0.0, oldValue-0.05)
			if newValue != oldValue {
				proposal.Changes = append(proposal.Changes, PersonalityChange{
					Path:     "traits.confidence",
					OldValue: oldValue,
					NewValue: newValue,
					Reason:   "User corrections suggest being less assertive when uncertain",
				})
			}
		}
	}

	if len(proposal.Changes) == 0 {
		return nil
	}

	proposal.Description = sm.generateProposalDescription(proposal)
	proposal.Impact = sm.predictImpact(proposal)
	proposal.RiskLevel = sm.assessRisk(proposal)

	return proposal
}

// createOpportunityProposal creates a proposal for a new learned pattern.
func (sm *SleepManager) createOpportunityProposal(insight ReflectionInsight, current *Personality, proposal *PersonalityProposal) *PersonalityProposal {
	proposal.Type = "new_pattern"

	newPattern := LearnedPattern{
		Pattern:      insight.Description,
		Confidence:   insight.Confidence,
		Source:       "sleep_cycle_analysis",
		AppliedSince: time.Now().Format(time.RFC3339),
	}

	proposal.Changes = append(proposal.Changes, PersonalityChange{
		Path:     "learned_patterns",
		OldValue: nil,
		NewValue: newPattern,
		Reason:   "Pattern detected from user interactions",
	})

	proposal.Description = fmt.Sprintf("Add learned pattern: %s", insight.Description)
	proposal.Impact = "Cortex will adapt behavior based on this learned preference"
	proposal.RiskLevel = RiskSafe

	return proposal
}

// prioritizeProposals sorts proposals by importance and limits count.
func (sm *SleepManager) prioritizeProposals(proposals []PersonalityProposal) []PersonalityProposal {
	// Sort by confidence descending
	sort.Slice(proposals, func(i, j int) bool {
		return proposals[i].Confidence > proposals[j].Confidence
	})

	// Limit to top 5 proposals per cycle
	if len(proposals) > 5 {
		proposals = proposals[:5]
	}

	return proposals
}

// enforceConstraints ensures proposals don't violate safety constraints.
func (sm *SleepManager) enforceConstraints(proposals []PersonalityProposal, current *Personality) []PersonalityProposal {
	constraints := current.GetConstraints()
	filtered := []PersonalityProposal{}

	for _, p := range proposals {
		valid := true

		for i, change := range p.Changes {
			traitName := extractTraitName(change.Path)

			// Check immutable traits
			if containsString(constraints.ImmutableTraits, traitName) {
				valid = false
				break
			}

			// Check and clamp max delta for trait changes
			if isTraitChange(change.Path) {
				oldVal, ok1 := change.OldValue.(float64)
				newVal, ok2 := change.NewValue.(float64)

				if ok1 && ok2 {
					delta := math.Abs(newVal - oldVal)
					if delta > constraints.MaxTraitDelta {
						// Clamp the change
						if newVal > oldVal {
							p.Changes[i].NewValue = oldVal + constraints.MaxTraitDelta
						} else {
							p.Changes[i].NewValue = oldVal - constraints.MaxTraitDelta
						}
					}
				}
			}
		}

		if valid {
			filtered = append(filtered, p)
		}
	}

	return filtered
}

// generateProposalDescription creates a human-readable description.
func (sm *SleepManager) generateProposalDescription(proposal *PersonalityProposal) string {
	if len(proposal.Changes) == 1 {
		change := proposal.Changes[0]
		return change.Reason
	}

	return fmt.Sprintf("%d trait adjustments based on recent interactions", len(proposal.Changes))
}

// predictImpact explains how this change will affect behavior.
func (sm *SleepManager) predictImpact(proposal *PersonalityProposal) string {
	impacts := []string{}

	for _, change := range proposal.Changes {
		traitName := extractTraitName(change.Path)
		oldVal, ok1 := change.OldValue.(float64)
		newVal, ok2 := change.NewValue.(float64)

		if !ok1 || !ok2 {
			continue
		}

		increasing := newVal > oldVal

		switch traitName {
		case "patience":
			if increasing {
				impacts = append(impacts, "more thorough responses, willing to explain multiple times")
			} else {
				impacts = append(impacts, "faster, more direct responses")
			}
		case "verbosity":
			if increasing {
				impacts = append(impacts, "more detailed explanations")
			} else {
				impacts = append(impacts, "more concise responses")
			}
		case "directness":
			if increasing {
				impacts = append(impacts, "more straightforward communication")
			} else {
				impacts = append(impacts, "more diplomatic phrasing")
			}
		case "confidence":
			if increasing {
				impacts = append(impacts, "more assertive statements")
			} else {
				impacts = append(impacts, "more hedging when uncertain")
			}
		case "warmth":
			if increasing {
				impacts = append(impacts, "friendlier, more approachable tone")
			} else {
				impacts = append(impacts, "more professional, focused tone")
			}
		}
	}

	if len(impacts) == 0 {
		return "Subtle behavioral adjustment"
	}

	return strings.Join(impacts, "; ")
}

// assessRisk determines the risk level of a proposal.
func (sm *SleepManager) assessRisk(proposal *PersonalityProposal) RiskLevel {
	// New patterns are always safe
	if proposal.Type == "new_pattern" {
		return RiskSafe
	}

	// Calculate max delta across all changes
	maxDelta := 0.0
	for _, change := range proposal.Changes {
		if isTraitChange(change.Path) {
			oldVal, ok1 := change.OldValue.(float64)
			newVal, ok2 := change.NewValue.(float64)

			if ok1 && ok2 {
				delta := math.Abs(newVal - oldVal)
				if delta > maxDelta {
					maxDelta = delta
				}
			}
		}
	}

	if maxDelta <= 0.05 {
		return RiskSafe
	} else if maxDelta <= 0.15 {
		return RiskModerate
	}
	return RiskSignificant
}

// Utility functions

func extractTraitName(path string) string {
	parts := strings.Split(path, ".")
	if len(parts) >= 2 {
		return parts[1]
	}
	return path
}

func isTraitChange(path string) bool {
	return strings.HasPrefix(path, "traits.")
}

func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
