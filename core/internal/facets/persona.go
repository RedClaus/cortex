// Package facets provides persona management for the Cortex cognitive architecture.
// Personas define structured AI identities with expertise domains, communication styles,
// and behavioral modes that compile into system prompts at load time (zero runtime cost).
package facets

import (
	"fmt"
	"strings"
	"time"
)

// PersonaCore defines a structured persona with rich metadata.
// All fields compile into SystemPrompt at load time (zero runtime cost).
type PersonaCore struct {
	// Primary key
	ID      string `json:"id" db:"id"`
	Version string `json:"version" db:"version"`

	// === IDENTITY ===
	Name       string   `json:"name" db:"name"`
	Role       string   `json:"role" db:"role"`             // "Senior SRE", "Git Expert"
	Background string   `json:"background" db:"background"` // Brief backstory
	Traits     []string `json:"traits" db:"-"`              // ["methodical", "patient"]
	Values     []string `json:"values" db:"-"`              // ["reliability", "clarity"]

	// === EXPERTISE ===
	Expertise []ExpertiseDomain `json:"expertise" db:"-"`

	// === COMMUNICATION ===
	Style CommunicationStyle `json:"style" db:"-"`

	// === BEHAVIORAL MODES ===
	Modes       []BehavioralMode `json:"modes" db:"-"`
	DefaultMode string           `json:"default_mode" db:"default_mode"`

	// === KNOWLEDGE LINKS ===
	KnowledgeSourceIDs []string `json:"knowledge_source_ids,omitempty" db:"-"`

	// === COMPILED OUTPUT ===
	// Generated from above fields, cached in DB
	SystemPrompt string `json:"system_prompt" db:"system_prompt"`

	// === METADATA ===
	IsBuiltIn bool      `json:"is_built_in" db:"is_built_in"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// ExpertiseDomain defines a knowledge area with depth level.
type ExpertiseDomain struct {
	Domain      string   `json:"domain"`                // "kubernetes", "git", "networking"
	Depth       string   `json:"depth"`                 // "expert", "proficient", "familiar"
	Specialties []string `json:"specialties,omitempty"` // ["helm", "ingress", "rbac"]
	Boundaries  []string `json:"boundaries,omitempty"`  // Things NOT to answer
}

// CommunicationStyle defines how the persona communicates.
type CommunicationStyle struct {
	Tone       string   `json:"tone"`                 // "professional", "casual", "academic"
	Verbosity  string   `json:"verbosity"`            // "concise", "detailed", "adaptive"
	Formatting string   `json:"formatting"`           // "markdown", "plain", "code-heavy"
	Patterns   []string `json:"patterns,omitempty"`   // ["asks clarifying questions", "provides examples"]
	Avoids     []string `json:"avoids,omitempty"`     // ["jargon without explanation", "assumptions"]
}

// BehavioralMode defines a behavioral state with transition triggers.
// Modes form a shallow FSM (max 1 level, no nested states).
type BehavioralMode struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`

	// Prompt augmentation when this mode is active
	PromptAugment string `json:"prompt_augment,omitempty"`

	// Transition triggers (keyword-based, no LLM calls)
	EntryKeywords []string `json:"entry_keywords,omitempty"` // Keywords that activate this mode
	ExitKeywords  []string `json:"exit_keywords,omitempty"`  // Keywords that deactivate this mode
	ManualTrigger string   `json:"manual_trigger,omitempty"` // Explicit trigger phrase

	// Style overrides
	ForceVerbose bool `json:"force_verbose,omitempty"`
	ForceConcise bool `json:"force_concise,omitempty"`

	// Ordering
	SortOrder int `json:"sort_order,omitempty"`
}

// CompileSystemPrompt generates the system prompt from structured fields.
// Called once when persona is loaded or updated - zero runtime cost.
func (p *PersonaCore) CompileSystemPrompt() string {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("# %s\n\n", p.Name))
	sb.WriteString(fmt.Sprintf("You are %s, a %s.\n\n", p.Name, p.Role))

	// Background
	if p.Background != "" {
		sb.WriteString(fmt.Sprintf("## Background\n%s\n\n", p.Background))
	}

	// Traits and Values
	if len(p.Traits) > 0 || len(p.Values) > 0 {
		sb.WriteString("## Your Character\n")
		if len(p.Traits) > 0 {
			sb.WriteString(fmt.Sprintf("- Traits: %s\n", strings.Join(p.Traits, ", ")))
		}
		if len(p.Values) > 0 {
			sb.WriteString(fmt.Sprintf("- Values: %s\n", strings.Join(p.Values, ", ")))
		}
		sb.WriteString("\n")
	}

	// Expertise
	if len(p.Expertise) > 0 {
		sb.WriteString("## Your Expertise\n")
		for _, exp := range p.Expertise {
			sb.WriteString(fmt.Sprintf("- **%s** (%s)", exp.Domain, exp.Depth))
			if len(exp.Specialties) > 0 {
				sb.WriteString(fmt.Sprintf(": %s", strings.Join(exp.Specialties, ", ")))
			}
			sb.WriteString("\n")
			if len(exp.Boundaries) > 0 {
				sb.WriteString(fmt.Sprintf("  - Outside scope: %s\n", strings.Join(exp.Boundaries, ", ")))
			}
		}
		sb.WriteString("\n")
	}

	// Communication Style
	sb.WriteString("## Communication Style\n")
	sb.WriteString(fmt.Sprintf("- Tone: %s\n", p.Style.Tone))
	sb.WriteString(fmt.Sprintf("- Detail level: %s\n", p.Style.Verbosity))
	sb.WriteString(fmt.Sprintf("- Formatting: %s\n", p.Style.Formatting))
	if len(p.Style.Patterns) > 0 {
		sb.WriteString(fmt.Sprintf("- You: %s\n", strings.Join(p.Style.Patterns, "; ")))
	}
	if len(p.Style.Avoids) > 0 {
		sb.WriteString(fmt.Sprintf("- Avoid: %s\n", strings.Join(p.Style.Avoids, "; ")))
	}
	sb.WriteString("\n")

	// Guidelines
	sb.WriteString("## Guidelines\n")
	sb.WriteString("- Stay focused on your areas of expertise\n")
	sb.WriteString("- If asked about something outside your expertise, acknowledge the boundary\n")
	sb.WriteString("- Be helpful but honest about limitations\n")

	return sb.String()
}

// Validate checks the persona for required fields and consistency.
func (p *PersonaCore) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("persona name is required")
	}
	if p.Role == "" {
		return fmt.Errorf("persona role is required")
	}
	if p.DefaultMode == "" && len(p.Modes) > 0 {
		return fmt.Errorf("default_mode is required when modes are defined")
	}

	// Validate default mode exists
	if p.DefaultMode != "" {
		found := false
		for _, m := range p.Modes {
			if m.ID == p.DefaultMode {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("default_mode %q not found in modes", p.DefaultMode)
		}
	}

	// Validate mode IDs are unique
	modeIDs := make(map[string]bool)
	for _, m := range p.Modes {
		if m.ID == "" {
			return fmt.Errorf("mode ID is required")
		}
		if modeIDs[m.ID] {
			return fmt.Errorf("duplicate mode ID: %s", m.ID)
		}
		modeIDs[m.ID] = true
	}

	return nil
}

// GetMode returns a mode by ID, or nil if not found.
func (p *PersonaCore) GetMode(modeID string) *BehavioralMode {
	for i := range p.Modes {
		if p.Modes[i].ID == modeID {
			return &p.Modes[i]
		}
	}
	return nil
}

// GetDefaultMode returns the default mode.
func (p *PersonaCore) GetDefaultMode() *BehavioralMode {
	return p.GetMode(p.DefaultMode)
}
