// Package skills provides skill management for the Cortex Coder Agent
package skills

import (
	"fmt"
	"strings"
)

// Skill represents a skill definition loaded from YAML
type Skill struct {
	Name         string           `yaml:"name"`
	Version      string           `yaml:"version"`
	Description  string           `yaml:"description"`
	Category     string           `yaml:"category,omitempty"`
	Author       string           `yaml:"author,omitempty"`
	Triggers     []Trigger        `yaml:"triggers"`
	Template     string           `yaml:"template"`
	TemplateType string           `yaml:"template_type,omitempty"` // prompt, command, file
	Parameters   map[string]Param `yaml:"parameters,omitempty"`
	Tools        []SkillTool      `yaml:"tools,omitempty"`
	Examples     []Example        `yaml:"examples,omitempty"`
	PreHooks     []Hook           `yaml:"pre_hooks,omitempty"`
	PostHooks    []Hook           `yaml:"post_hooks,omitempty"`
	Source       string           `yaml:"-"` // File path where skill was loaded from
}

// Trigger defines when a skill should be activated
type Trigger struct {
	Type       string   `yaml:"type"`                 // selection, command, file, explicit
	Pattern    string   `yaml:"pattern,omitempty"`    // regex pattern for file/command matching
	Extensions []string `yaml:"extensions,omitempty"` // file extensions (e.g., ".go", ".py")
	Command    string   `yaml:"command,omitempty"`    // command name for command trigger
}

// Param defines a skill parameter
type Param struct {
	Type        string      `yaml:"type"`                   // string, int, bool, file, selection
	Description string      `yaml:"description"`
	Required    bool        `yaml:"required"`
	Default     interface{} `yaml:"default,omitempty"`
	Options     []string    `yaml:"options,omitempty"`
}

// SkillTool defines a tool that can be used by the skill
type SkillTool struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Parameters  Param    `yaml:"parameters,omitempty"`
	Required    []string `yaml:"required,omitempty"`
}

// Example provides example usage
type Example struct {
	Description string `yaml:"description"`
	Input       string `yaml:"input"`
	Output      string `yaml:"output,omitempty"`
}

// Hook defines a pre/post execution hook
type Hook struct {
	Type   string                 `yaml:"type"`   // validation, transformation, notification
	Action string                 `yaml:"action"` // action name
	Config map[string]interface{} `yaml:"config,omitempty"`
}

// Validate validates the skill definition
func (s *Skill) Validate() []ValidationError {
	var errors []ValidationError

	if strings.TrimSpace(s.Name) == "" {
		errors = append(errors, ValidationError{
			Field:   "name",
			Message: "skill name is required",
		})
	}

	if strings.TrimSpace(s.Version) == "" {
		errors = append(errors, ValidationError{
			Field:   "version",
			Message: "skill version is required",
		})
	}

	if strings.TrimSpace(s.Description) == "" {
		errors = append(errors, ValidationError{
			Field:   "description",
			Message: "skill description is required",
		})
	}

	if len(s.Triggers) == 0 {
		errors = append(errors, ValidationError{
			Field:   "triggers",
			Message: "at least one trigger is required",
		})
	}

	if strings.TrimSpace(s.Template) == "" {
		errors = append(errors, ValidationError{
			Field:   "template",
			Message: "skill template is required",
		})
	}

	// Validate triggers
	for i, trigger := range s.Triggers {
		if trigger.Type == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("triggers[%d].type", i),
				Message: "trigger type is required",
			})
		}
	}

	// Validate parameters
	for name, param := range s.Parameters {
		if param.Type == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("parameters[%s].type", name),
				Message: "parameter type is required",
			})
		}
	}

	return errors
}

// HasTrigger checks if skill has a specific trigger type
func (s *Skill) HasTrigger(triggerType string) bool {
	for _, t := range s.Triggers {
		if t.Type == triggerType {
			return true
		}
	}
	return false
}

// HasCommandTrigger checks if skill has a specific command trigger
func (s *Skill) HasCommandTrigger(cmd string) bool {
	for _, t := range s.Triggers {
		if t.Type == "command" && t.Command == cmd {
			return true
		}
	}
	return false
}

// HasFileExtensionTrigger checks if skill handles a specific file extension
func (s *Skill) HasFileExtensionTrigger(ext string) bool {
	for _, t := range s.Triggers {
		if t.Type == "file" {
			for _, e := range t.Extensions {
				if e == ext || "."+e == ext {
					return true
				}
			}
		}
	}
	return false
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}
