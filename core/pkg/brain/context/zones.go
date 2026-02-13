// Package context implements attention-aware context management for CortexBrain.
// It provides zone-based organization leveraging LLM attention patterns.
//
// Based on "Lost in the Middle" research: LLMs attend more strongly to content
// at the beginning and end of context, with reduced attention to middle content.
package context

// AttentionZone represents a position-aware zone in the context window.
// Zones are ordered to maximize LLM attention allocation.
type AttentionZone string

const (
	// ZoneCritical holds high-priority content placed at the BEGINNING.
	// Receives strong attention. Examples: system context, user profile, critical facts.
	// Budget: ~20% of total tokens (configurable)
	ZoneCritical AttentionZone = "critical"

	// ZoneSupporting holds supplementary content placed in the MIDDLE.
	// Receives weaker attention (the "lost in middle" zone).
	// Examples: retrieved docs, historical data, supporting evidence.
	// Budget: ~50% of total tokens (configurable)
	ZoneSupporting AttentionZone = "supporting"

	// ZoneActionable holds action-oriented content placed at the END.
	// Receives strong attention. Examples: current task, lobe outputs, action items.
	// Budget: ~30% of total tokens (configurable)
	ZoneActionable AttentionZone = "actionable"
)

// ZoneConfig defines token budget allocation for each zone.
type ZoneConfig struct {
	Critical   int `yaml:"critical" json:"critical"`
	Supporting int `yaml:"supporting" json:"supporting"`
	Actionable int `yaml:"actionable" json:"actionable"`
}

// DefaultZoneConfig returns the default zone token budgets.
// Total: 4500 tokens (fits within most model context limits)
func DefaultZoneConfig() ZoneConfig {
	return ZoneConfig{
		Critical:   1500, // 20% - system context, user profile
		Supporting: 2000, // 50% - retrieved knowledge, history
		Actionable: 1000, // 30% - current task, outputs
	}
}

// Total returns the total token budget across all zones.
func (c ZoneConfig) Total() int {
	return c.Critical + c.Supporting + c.Actionable
}

// ZoneOrder returns zones in the order they should appear in context.
// This ordering maximizes attention: critical first, supporting middle, actionable last.
func ZoneOrder() []AttentionZone {
	return []AttentionZone{ZoneCritical, ZoneSupporting, ZoneActionable}
}

// ZonePriority returns the attention priority for a zone (1.0 = highest).
// Used for deciding what to keep during compaction.
func ZonePriority(zone AttentionZone) float64 {
	switch zone {
	case ZoneCritical:
		return 1.0 // Highest priority - never prune first
	case ZoneActionable:
		return 0.9 // High priority - recent/actionable
	case ZoneSupporting:
		return 0.6 // Lower priority - can be pruned
	default:
		return 0.5
	}
}
