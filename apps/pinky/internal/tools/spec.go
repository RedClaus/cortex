// Package tools provides the tool execution framework for Pinky
package tools

import "encoding/json"

// ToolSpec describes a tool for LLM function calling
type ToolSpec struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Parameters  *ParamSchema `json:"parameters,omitempty"`
	Category    ToolCategory `json:"category"`
	RiskLevel   RiskLevel    `json:"risk_level"`
}

// ParamSchema defines the JSON Schema for tool parameters
type ParamSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]*ParamProp  `json:"properties,omitempty"`
	Required   []string               `json:"required,omitempty"`
}

// ParamProp defines a single parameter property
type ParamProp struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	Default     any      `json:"default,omitempty"`
}

// Specifiable is implemented by tools that can describe themselves
type Specifiable interface {
	Spec() *ToolSpec
}

// GetSpec returns a tool's spec if it implements Specifiable,
// otherwise constructs a basic spec from the Tool interface
func GetSpec(t Tool) *ToolSpec {
	if spec, ok := t.(Specifiable); ok {
		return spec.Spec()
	}

	// Fallback: construct from Tool interface
	return &ToolSpec{
		Name:        t.Name(),
		Description: t.Description(),
		Category:    t.Category(),
		RiskLevel:   t.RiskLevel(),
	}
}

// RegistrySpecs returns specs for all tools in a registry
func (r *Registry) Specs() []*ToolSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()

	specs := make([]*ToolSpec, 0, len(r.tools))
	for _, t := range r.tools {
		specs = append(specs, GetSpec(t))
	}
	return specs
}

// SpecsByCategory returns specs for tools in a category
func (r *Registry) SpecsByCategory(cat ToolCategory) []*ToolSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var specs []*ToolSpec
	for _, t := range r.tools {
		if t.Category() == cat {
			specs = append(specs, GetSpec(t))
		}
	}
	return specs
}

// ToJSON converts a ToolSpec to JSON for LLM consumption
func (s *ToolSpec) ToJSON() ([]byte, error) {
	return json.Marshal(s)
}

// ToolCall represents a tool invocation request from the LLM
type ToolCall struct {
	ID     string         `json:"id"`
	Tool   string         `json:"tool"`
	Input  map[string]any `json:"input"`
	Reason string         `json:"reason,omitempty"`
}

// ToToolInput converts a ToolCall to a ToolInput for execution
func (tc *ToolCall) ToToolInput(userID, workingDir string) *ToolInput {
	cmd := ""
	if c, ok := tc.Input["command"].(string); ok {
		cmd = c
	}

	return &ToolInput{
		Command:    cmd,
		Args:       tc.Input,
		WorkingDir: workingDir,
		UserID:     userID,
	}
}
