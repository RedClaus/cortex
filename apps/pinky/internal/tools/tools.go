// Package tools provides the tool execution framework for Pinky
package tools

import (
	"context"
	"sync"
	"time"
)

// ToolCategory categorizes tools by function
type ToolCategory string

const (
	CategoryShell  ToolCategory = "shell"
	CategoryFiles  ToolCategory = "files"
	CategoryWeb    ToolCategory = "web"
	CategoryAPI    ToolCategory = "api"
	CategoryGit    ToolCategory = "git"
	CategoryCode   ToolCategory = "code"
	CategorySystem ToolCategory = "system"
)

// RiskLevel indicates tool risk for permission system
type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

// Tool is the interface all tools must implement
type Tool interface {
	Name() string
	Description() string
	Category() ToolCategory
	RiskLevel() RiskLevel
	Execute(ctx context.Context, input *ToolInput) (*ToolOutput, error)
	Validate(input *ToolInput) error
}

// ToolInput contains parameters for tool execution
type ToolInput struct {
	Command    string         // Primary command/action
	Args       map[string]any // Structured arguments
	WorkingDir string         // Execution context
	UserID     string         // For audit trail
}

// ToolOutput contains the result of tool execution
type ToolOutput struct {
	Success   bool
	Output    string
	Error     string
	Duration  time.Duration
	Artifacts []string // Created files, URLs, etc.
}

// Registry manages available tools
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry creates a new tool registry
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry
func (r *Registry) Register(tool Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name()] = tool
}

// Get retrieves a tool by name
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	return tool, ok
}

// List returns all registered tools
func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tools := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	return tools
}

// ListByCategory returns tools filtered by category
func (r *Registry) ListByCategory(cat ToolCategory) []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var tools []Tool
	for _, t := range r.tools {
		if t.Category() == cat {
			tools = append(tools, t)
		}
	}
	return tools
}

// ListByRisk returns tools filtered by risk level
func (r *Registry) ListByRisk(risk RiskLevel) []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var tools []Tool
	for _, t := range r.tools {
		if t.RiskLevel() == risk {
			tools = append(tools, t)
		}
	}
	return tools
}
