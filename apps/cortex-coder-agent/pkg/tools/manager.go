// Package tools provides local tool implementations for the Cortex Coder Agent
package tools

import (
	"context"
	"os/exec"
)

// Tool represents a local tool
type Tool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, args []string, input string) (string, error)
}

// Manager handles tool registration and execution
type Manager struct {
	tools map[string]Tool
}

// NewManager creates a new tool manager
func NewManager() *Manager {
	return &Manager{
		tools: make(map[string]Tool),
	}
}

// Register registers a tool
func (m *Manager) Register(tool Tool) {
	m.tools[tool.Name()] = tool
}

// Get retrieves a tool by name
func (m *Manager) Get(name string) (Tool, bool) {
	tool, ok := m.tools[name]
	return tool, ok
}

// Execute runs a tool by name
func (m *Manager) Execute(ctx context.Context, name string, args []string, input string) (string, error) {
	tool, ok := m.tools[name]
	if !ok {
		return "", ErrToolNotFound(name)
	}
	return tool.Execute(ctx, args, input)
}

// List returns all registered tool names
func (m *Manager) List() []string {
	names := make([]string, 0, len(m.tools))
	for name := range m.tools {
		names = append(names, name)
	}
	return names
}

// ToolError represents a tool execution error
type ToolError struct {
	Tool   string
	Stderr string
}

func (e *ToolError) Error() string {
	return e.Tool + ": " + e.Stderr
}

// ErrToolNotFound returns a tool not found error
func ErrToolNotFound(name string) error {
	return &ToolError{Tool: name}
}

// RunCommand executes a shell command
func RunCommand(ctx context.Context, name string, args []string, input string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = nil
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", &ToolError{
			Tool:   name,
			Stderr: string(output),
		}
	}
	
	return string(output), nil
}
