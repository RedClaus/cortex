// Package agent provides the core agent logic for the Cortex Coder Agent
package agent

import (
	"context"
	"fmt"
)

// Agent represents the core coding agent
type Agent struct {
	name    string
	model   string
	skills  map[string]Skill
	state   *State
}

// State represents the current agent state
type State struct {
	Context    string
	History    []Message
	Variables  map[string]interface{}
}

// Message represents a conversation message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Skill represents a loaded skill
type Skill struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Prompt      string            `json:"prompt"`
	Parameters  map[string]Param  `json:"parameters"`
}

// Param represents a skill parameter
type Param struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     interface{} `json:"default,omitempty"`
}

// New creates a new Agent instance
func New(name, model string) *Agent {
	return &Agent{
		name:   name,
		model:  model,
		skills: make(map[string]Skill),
		state: &State{
			Variables: make(map[string]interface{}),
		},
	}
}

// LoadSkill loads a skill into the agent
func (a *Agent) LoadSkill(skill Skill) error {
	if skill.Name == "" {
		return fmt.Errorf("skill name is required")
	}
	a.skills[skill.Name] = skill
	return nil
}

// GetSkill returns a loaded skill by name
func (a *Agent) GetSkill(name string) (Skill, bool) {
	skill, ok := a.skills[name]
	return skill, ok
}

// ListSkills returns all loaded skill names
func (a *Agent) ListSkills() []string {
	names := make([]string, 0, len(a.skills))
	for name := range a.skills {
		names = append(names, name)
	}
	return names
}

// Execute runs the agent with the given input
func (a *Agent) Execute(ctx context.Context, input string) (string, error) {
	// TODO: Implement core execution logic
	// 1. Parse input to determine skill
	// 2. Load relevant context from CortexBrain
	// 3. Build prompt with skill template
	// 4. Call LLM
	// 5. Return response
	
	return fmt.Sprintf("Agent '%s' received: %s", a.name, input), nil
}

// Reset clears the agent's conversation history
func (a *Agent) Reset() {
	a.state.History = nil
	a.state.Variables = make(map[string]interface{})
}
