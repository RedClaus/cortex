// Package prompts provides template engine functionality for the Cortex Coder Agent
package prompts

import (
	"bytes"
	"fmt"
	"text/template"
)

// TemplateEngine handles prompt template processing
type TemplateEngine struct {
	templates map[string]*template.Template
}

// New creates a new template engine
func New() *TemplateEngine {
	return &TemplateEngine{
		templates: make(map[string]*template.Template),
	}
}

// Load loads a template by name
func (e *TemplateEngine) Load(name, content string) error {
	tmpl, err := template.New(name).Parse(content)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}
	e.templates[name] = tmpl
	return nil
}

// Execute executes a template with the given data
func (e *TemplateEngine) Execute(name string, data interface{}) (string, error) {
	tmpl, ok := e.templates[name]
	if !ok {
		return "", fmt.Errorf("template not found: %s", name)
	}
	
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}
	
	return buf.String(), nil
}

// HasTemplate checks if a template exists
func (e *TemplateEngine) HasTemplate(name string) bool {
	_, ok := e.templates[name]
	return ok
}

// ListTemplates returns all loaded template names
func (e *TemplateEngine) ListTemplates() []string {
	names := make([]string, 0, len(e.templates))
	for name := range e.templates {
		names = append(names, name)
	}
	return names
}
