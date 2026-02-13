// Package resemble provides Resemble.ai API clients and Voice Agent tool mapping.
// This file implements Phase 4 of CR-016: Map Resemble agent tools to Cortex tool format.
package resemble

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/normanking/cortex/internal/logging"
)

// ═══════════════════════════════════════════════════════════════════════════════
// CORTEX TOOL DEFINITIONS (for LLM function calling)
// ═══════════════════════════════════════════════════════════════════════════════

// CortexTool represents a tool definition for LLM function calling.
// This follows the OpenAI/Anthropic function calling format used by memory.Tool.
type CortexTool struct {
	Type     string            `json:"type"`
	Function CortexFunctionDef `json:"function"`
}

// CortexFunctionDef defines a callable function for LLMs.
type CortexFunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// TOOL MAPPER
// ═══════════════════════════════════════════════════════════════════════════════

// ToolMapper converts Resemble Agent tools to Cortex tool format.
// It handles the translation between Resemble's tool schema and Cortex's
// function calling format used by the LLM layer.
type ToolMapper struct {
	log *logging.Logger
}

// NewToolMapper creates a new tool mapper.
func NewToolMapper() *ToolMapper {
	return &ToolMapper{
		log: logging.Global().WithComponent("ToolMapper"),
	}
}

// MapTools converts Resemble tools to Cortex tool definitions.
// Only active tools are mapped.
func (m *ToolMapper) MapTools(resembleTools []Tool) []CortexTool {
	if len(resembleTools) == 0 {
		return nil
	}

	tools := make([]CortexTool, 0, len(resembleTools))
	for _, t := range resembleTools {
		if !t.Active {
			m.log.Debug("Skipping inactive tool: %s", t.Name)
			continue
		}

		mapped := m.MapTool(t)
		if mapped != nil {
			tools = append(tools, *mapped)
		}
	}

	m.log.Info("Mapped %d/%d Resemble tools to Cortex format", len(tools), len(resembleTools))
	return tools
}

// MapTool converts a single Resemble tool to Cortex format.
// Returns nil if the tool cannot be mapped.
func (m *ToolMapper) MapTool(t Tool) *CortexTool {
	// Build JSON Schema for parameters
	params := m.buildParameterSchema(t.Parameters)

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		m.log.Warn("Failed to marshal parameters for tool %s: %v", t.Name, err)
		// Use empty object schema as fallback
		paramsJSON = []byte(`{"type": "object", "properties": {}}`)
	}

	return &CortexTool{
		Type: "function",
		Function: CortexFunctionDef{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  paramsJSON,
		},
	}
}

// buildParameterSchema converts Resemble's parameter format to JSON Schema.
func (m *ToolMapper) buildParameterSchema(params map[string]ToolParam) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}

	if len(params) == 0 {
		return schema
	}

	properties := make(map[string]any)
	required := make([]string, 0)

	for name, param := range params {
		propSchema := map[string]any{
			"type":        param.Type,
			"description": param.Description,
		}

		// Map complex types
		switch param.Type {
		case "array":
			// Default to string array if not specified
			propSchema["items"] = map[string]any{"type": "string"}
		case "object":
			// Allow additional properties for flexible objects
			propSchema["additionalProperties"] = true
		}

		properties[name] = propSchema

		if param.Required {
			required = append(required, name)
		}
	}

	schema["properties"] = properties
	if len(required) > 0 {
		schema["required"] = required
	}

	return schema
}

// ═══════════════════════════════════════════════════════════════════════════════
// DYNAMIC TOOL REGISTRY
// ═══════════════════════════════════════════════════════════════════════════════

// AgentToolRegistry manages dynamically registered agent tools.
// It allows Resemble agent tools to be registered/unregistered at runtime,
// making them callable by the local LLM.
type AgentToolRegistry struct {
	mu       sync.RWMutex
	tools    map[string]CortexTool  // name -> tool definition
	handlers map[string]ToolHandler // name -> execution handler
	agentID  string                 // Current agent's UUID
	mapper   *ToolMapper
	executor *CortexToolExecutor
	log      *logging.Logger
}

// ToolHandler is a function that executes a tool with given arguments.
type ToolHandler func(ctx context.Context, args map[string]any) (any, error)

// globalRegistry is the singleton registry instance.
var (
	globalRegistry     *AgentToolRegistry
	globalRegistryOnce sync.Once
)

// GlobalRegistry returns the global agent tool registry.
func GlobalRegistry() *AgentToolRegistry {
	globalRegistryOnce.Do(func() {
		globalRegistry = NewAgentToolRegistry("")
	})
	return globalRegistry
}

// NewAgentToolRegistry creates a new agent tool registry.
func NewAgentToolRegistry(workDir string) *AgentToolRegistry {
	return &AgentToolRegistry{
		tools:    make(map[string]CortexTool),
		handlers: make(map[string]ToolHandler),
		mapper:   NewToolMapper(),
		executor: NewCortexToolExecutor(workDir),
		log:      logging.Global().WithComponent("AgentToolRegistry"),
	}
}

// RegisterAgentTools registers all tools from a Resemble agent.
// This replaces any previously registered agent tools.
func (r *AgentToolRegistry) RegisterAgentTools(agent *Agent) error {
	if agent == nil {
		return fmt.Errorf("agent cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Clear existing tools
	r.tools = make(map[string]CortexTool)
	r.handlers = make(map[string]ToolHandler)
	r.agentID = agent.UUID

	// Map and register each tool
	for _, t := range agent.Tools {
		if !t.Active {
			continue
		}

		mapped := r.mapper.MapTool(t)
		if mapped == nil {
			r.log.Warn("Failed to map tool: %s", t.Name)
			continue
		}

		r.tools[t.Name] = *mapped

		// Create handler based on tool type
		handler := r.createHandler(t)
		r.handlers[t.Name] = handler

		r.log.Info("Registered agent tool: %s (type=%s)", t.Name, t.ToolType)
	}

	r.log.Info("Registered %d tools from agent %s (%s)", len(r.tools), agent.Name, agent.UUID)
	return nil
}

// RegisterTools registers tools from a slice (convenience method for testing).
func RegisterAgentTools(agentTools []Tool) error {
	return GlobalRegistry().RegisterTools(agentTools)
}

// RegisterTools registers tools from a slice directly.
func (r *AgentToolRegistry) RegisterTools(tools []Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, t := range tools {
		if !t.Active {
			continue
		}

		mapped := r.mapper.MapTool(t)
		if mapped == nil {
			r.log.Warn("Failed to map tool: %s", t.Name)
			continue
		}

		r.tools[t.Name] = *mapped
		r.handlers[t.Name] = r.createHandler(t)
		r.log.Info("Registered tool: %s", t.Name)
	}

	return nil
}

// UnregisterAgentTools removes all registered agent tools.
func UnregisterAgentTools() error {
	return GlobalRegistry().UnregisterAllTools()
}

// UnregisterAllTools removes all registered tools.
func (r *AgentToolRegistry) UnregisterAllTools() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := len(r.tools)
	r.tools = make(map[string]CortexTool)
	r.handlers = make(map[string]ToolHandler)
	r.agentID = ""

	r.log.Info("Unregistered %d agent tools", count)
	return nil
}

// UnregisterTool removes a single tool by name.
func (r *AgentToolRegistry) UnregisterTool(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[name]; !exists {
		return fmt.Errorf("tool not found: %s", name)
	}

	delete(r.tools, name)
	delete(r.handlers, name)
	r.log.Info("Unregistered tool: %s", name)
	return nil
}

// GetTools returns all registered tool definitions.
func (r *AgentToolRegistry) GetTools() []CortexTool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]CortexTool, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	return tools
}

// GetTool returns a single tool definition by name.
func (r *AgentToolRegistry) GetTool(name string) (*CortexTool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	t, exists := r.tools[name]
	if !exists {
		return nil, false
	}
	return &t, true
}

// HasTool checks if a tool is registered.
func (r *AgentToolRegistry) HasTool(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.tools[name]
	return exists
}

// ToolCount returns the number of registered tools.
func (r *AgentToolRegistry) ToolCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}

// AgentID returns the UUID of the currently registered agent.
func (r *AgentToolRegistry) AgentID() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.agentID
}

// ═══════════════════════════════════════════════════════════════════════════════
// TOOL EXECUTION
// ═══════════════════════════════════════════════════════════════════════════════

// ExecuteTool executes a registered tool by name.
func (r *AgentToolRegistry) ExecuteTool(ctx context.Context, name string, argsJSON string) (any, error) {
	r.mu.RLock()
	handler, exists := r.handlers[name]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("tool not registered: %s", name)
	}

	// Parse arguments
	var args map[string]any
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return nil, fmt.Errorf("invalid tool arguments: %w", err)
		}
	} else {
		args = make(map[string]any)
	}

	r.log.Debug("Executing tool: %s with args: %v", name, args)
	return handler(ctx, args)
}

// createHandler creates an execution handler for a tool.
func (r *AgentToolRegistry) createHandler(t Tool) ToolHandler {
	switch t.ToolType {
	case "builtin":
		return r.createBuiltinHandler(t)
	case "webhook":
		return r.createWebhookHandler(t)
	default:
		// Default to builtin handling
		return r.createBuiltinHandler(t)
	}
}

// createBuiltinHandler creates a handler that delegates to CortexToolExecutor.
func (r *AgentToolRegistry) createBuiltinHandler(t Tool) ToolHandler {
	toolName := t.Name
	return func(ctx context.Context, args map[string]any) (any, error) {
		return r.executor.Execute(ctx, toolName, args, r.agentID)
	}
}

// createWebhookHandler creates a handler for webhook-based tools.
func (r *AgentToolRegistry) createWebhookHandler(t Tool) ToolHandler {
	// For webhook tools, we need to make an HTTP call to the configured endpoint
	return func(ctx context.Context, args map[string]any) (any, error) {
		if t.APISchema == nil {
			return nil, fmt.Errorf("webhook tool %s has no API schema", t.Name)
		}

		// TODO: Implement actual webhook execution
		// This would make an HTTP request to t.APISchema.URL with the method
		// t.APISchema.Method, headers t.APISchema.RequestHeaders, and body
		// constructed from args and t.APISchema.RequestBody
		r.log.Warn("Webhook tool execution not yet implemented: %s", t.Name)
		return map[string]any{
			"error":   "webhook execution not implemented",
			"tool":    t.Name,
			"webhook": t.APISchema.URL,
		}, nil
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// TOOL DEFINITIONS FOR LLM
// ═══════════════════════════════════════════════════════════════════════════════

// GetToolDefinitions returns all registered tools in the format needed for LLM function calling.
// This can be combined with other tool definitions (memory tools, task tools, etc.).
func (r *AgentToolRegistry) GetToolDefinitions() []CortexTool {
	return r.GetTools()
}

// GetToolDefinitionsJSON returns all registered tools as a JSON string.
func (r *AgentToolRegistry) GetToolDefinitionsJSON() (string, error) {
	tools := r.GetTools()
	data, err := json.MarshalIndent(tools, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal tool definitions: %w", err)
	}
	return string(data), nil
}

// MergeWithMemoryTools merges agent tools with memory tools for complete LLM tool set.
// The memoryTools parameter should come from memory.MemoryTools.GetToolDefinitions().
func (r *AgentToolRegistry) MergeWithMemoryTools(memoryTools []CortexTool) []CortexTool {
	agentTools := r.GetTools()

	// Create combined slice with memory tools first (they have priority)
	combined := make([]CortexTool, 0, len(memoryTools)+len(agentTools))
	combined = append(combined, memoryTools...)

	// Add agent tools, skipping any that conflict with memory tool names
	memoryToolNames := make(map[string]bool)
	for _, t := range memoryTools {
		memoryToolNames[t.Function.Name] = true
	}

	for _, t := range agentTools {
		if !memoryToolNames[t.Function.Name] {
			combined = append(combined, t)
		} else {
			r.log.Debug("Skipping agent tool %s (conflicts with memory tool)", t.Function.Name)
		}
	}

	return combined
}

// ═══════════════════════════════════════════════════════════════════════════════
// BUILT-IN CORTEX TOOLS FOR AGENTS
// ═══════════════════════════════════════════════════════════════════════════════

// GetBuiltinCortexTools returns the standard Cortex tools that should be available
// to all voice agents. These map to the CortexToolExecutor's capabilities.
func GetBuiltinCortexTools() []CortexTool {
	return []CortexTool{
		{
			Type: "function",
			Function: CortexFunctionDef{
				Name:        "bash",
				Description: "Execute a shell command. Use for running terminal commands, scripts, or system operations.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"command": {
							"type": "string",
							"description": "The shell command to execute"
						}
					},
					"required": ["command"]
				}`),
			},
		},
		{
			Type: "function",
			Function: CortexFunctionDef{
				Name:        "read_file",
				Description: "Read the contents of a file. Returns the file content as text.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"path": {
							"type": "string",
							"description": "Path to the file to read (relative to working directory or absolute)"
						}
					},
					"required": ["path"]
				}`),
			},
		},
		{
			Type: "function",
			Function: CortexFunctionDef{
				Name:        "write_file",
				Description: "Write content to a file. Creates the file if it doesn't exist, overwrites if it does.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"path": {
							"type": "string",
							"description": "Path where the file should be written"
						},
						"content": {
							"type": "string",
							"description": "Content to write to the file"
						}
					},
					"required": ["path", "content"]
				}`),
			},
		},
		{
			Type: "function",
			Function: CortexFunctionDef{
				Name:        "list_directory",
				Description: "List files and directories in a given path. Returns names, sizes, and types.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"path": {
							"type": "string",
							"description": "Directory path to list (relative or absolute)"
						}
					},
					"required": ["path"]
				}`),
			},
		},
		{
			Type: "function",
			Function: CortexFunctionDef{
				Name:        "glob",
				Description: "Find files matching a glob pattern. Supports wildcards like *.go, **/*.ts",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"pattern": {
							"type": "string",
							"description": "Glob pattern to match files against"
						}
					},
					"required": ["pattern"]
				}`),
			},
		},
		{
			Type: "function",
			Function: CortexFunctionDef{
				Name:        "grep",
				Description: "Search file contents for a pattern. Returns matching lines with file paths and line numbers.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"pattern": {
							"type": "string",
							"description": "Regular expression pattern to search for"
						},
						"path": {
							"type": "string",
							"description": "Directory or file path to search in (defaults to working directory)"
						}
					},
					"required": ["pattern"]
				}`),
			},
		},
	}
}
