package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/normanking/cortex/internal/knowledge"
	"github.com/normanking/cortex/pkg/types"
	"github.com/rs/zerolog/log"
)

// MemoryTools provides LLM-callable tools for memory management.
// These are used by Smart Lane for active retrieval.
//
// Per CR-003:
// - recall_memory_search: Search conversation history
// - core_memory_read: Read user/project facts
// - core_memory_append: Remember new facts about user
// - archival_memory_search: Search knowledge base
// - archival_memory_insert: Store lessons for future
type MemoryTools struct {
	coreStore *CoreMemoryStore
	fabric    knowledge.KnowledgeFabric
	config    MemoryToolsConfig
	metrics   *ToolMetrics
}

// MemoryToolsConfig configures memory tools behavior.
type MemoryToolsConfig struct {
	AllowCoreMemoryWrite bool `json:"allow_core_memory_write"` // Can LLM write to core memory
	AllowArchivalInsert  bool `json:"allow_archival_insert"`   // Can LLM store knowledge
	MaxRecallResults     int  `json:"max_recall_results"`      // Max conversation search results
	MaxArchivalResults   int  `json:"max_archival_results"`    // Max knowledge search results
	MaxToolRounds        int  `json:"max_tool_rounds"`         // Max tool call rounds (default: 3)
}

// DefaultMemoryToolsConfig returns sensible defaults.
func DefaultMemoryToolsConfig() MemoryToolsConfig {
	return MemoryToolsConfig{
		AllowCoreMemoryWrite: true,
		AllowArchivalInsert:  true,
		MaxRecallResults:     5,
		MaxArchivalResults:   5,
		MaxToolRounds:        3, // CR-003 v1.1 reduced from 5 to 3
	}
}

// ToolMetrics tracks tool usage.
type ToolMetrics struct {
	CallCounts  map[string]int64         `json:"call_counts"`
	ErrorCounts map[string]int64         `json:"error_counts"`
	AvgLatency  map[string]time.Duration `json:"avg_latency"`
	TotalCalls  int64                    `json:"total_calls"`
	TotalErrors int64                    `json:"total_errors"`
}

// NewToolMetrics creates initialized tool metrics.
func NewToolMetrics() *ToolMetrics {
	return &ToolMetrics{
		CallCounts:  make(map[string]int64),
		ErrorCounts: make(map[string]int64),
		AvgLatency:  make(map[string]time.Duration),
	}
}

// NewMemoryTools creates a new memory tools instance.
func NewMemoryTools(coreStore *CoreMemoryStore, fabric knowledge.KnowledgeFabric, config MemoryToolsConfig) *MemoryTools {
	return &MemoryTools{
		coreStore: coreStore,
		fabric:    fabric,
		config:    config,
		metrics:   NewToolMetrics(),
	}
}

// Tool represents a tool definition for function calling.
type Tool struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

// FunctionDef defines a callable function.
type FunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// ToolResult represents the result of a tool execution.
type ToolResult struct {
	ToolName  string `json:"tool_name"`
	Success   bool   `json:"success"`
	Result    string `json:"result"`
	Error     string `json:"error,omitempty"`
	LatencyMs int64  `json:"latency_ms"`
}

// GetToolDefinitions returns tool schemas for function calling.
// These follow the OpenAI/Anthropic function calling format.
func (mt *MemoryTools) GetToolDefinitions() []Tool {
	tools := []Tool{
		// Search tools (always available)
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "recall_memory_search",
				Description: "Search past conversations for relevant context. Use when you need information from previous interactions with the user.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"query": {
							"type": "string",
							"description": "What to search for in conversation history"
						},
						"limit": {
							"type": "integer",
							"description": "Maximum results to return (default: 5)",
							"default": 5
						}
					},
					"required": ["query"]
				}`),
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "core_memory_read",
				Description: "Read persistent facts about the user or current project. Use to recall user preferences, environment, or project details.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"section": {
							"type": "string",
							"enum": ["user", "project"],
							"description": "Which section to read: 'user' for user facts, 'project' for project context"
						}
					},
					"required": ["section"]
				}`),
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "archival_memory_search",
				Description: "Search the long-term knowledge base for lessons, solutions, and learned information. Use when you need historical solutions or accumulated knowledge.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"query": {
							"type": "string",
							"description": "What to search for in the knowledge base"
						},
						"scope": {
							"type": "string",
							"enum": ["personal", "team", "global", ""],
							"description": "Limit search to a specific scope, or empty for all"
						},
						"limit": {
							"type": "integer",
							"description": "Maximum results to return (default: 5)",
							"default": 5
						}
					},
					"required": ["query"]
				}`),
			},
		},
	}

	// Write tools (if enabled)
	if mt.config.AllowCoreMemoryWrite {
		tools = append(tools, Tool{
			Type: "function",
			Function: FunctionDef{
				Name:        "core_memory_append",
				Description: "Remember a new fact about the user. Use when you learn something important about the user that should be remembered for future conversations.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"fact": {
							"type": "string",
							"description": "The fact to remember about the user"
						}
					},
					"required": ["fact"]
				}`),
			},
		})

		// Core memory update - for structured profile fields
		tools = append(tools, Tool{
			Type: "function",
			Function: FunctionDef{
				Name:        "core_memory_update",
				Description: "Update a specific profile field about the user. Use when you learn the user's name, role, operating system, shell, or editor. IMPORTANT: Use this when you first learn the user's name to ensure it's stored in their profile.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"field": {
							"type": "string",
							"enum": ["name", "role", "experience", "os", "shell", "editor"],
							"description": "The profile field to update: name (user's first name or nickname), role (job title like 'developer', 'designer'), experience (skill level), os (operating system like 'macOS', 'Linux'), shell (like 'zsh', 'bash'), editor (like 'vim', 'vscode')"
						},
						"value": {
							"type": "string",
							"description": "The value to set for the field"
						}
					},
					"required": ["field", "value"]
				}`),
			},
		})
	}

	if mt.config.AllowArchivalInsert {
		tools = append(tools, Tool{
			Type: "function",
			Function: FunctionDef{
				Name:        "archival_memory_insert",
				Description: "Store a lesson or solution in the knowledge base for future reference. Use after solving a problem that might recur, or learning something valuable.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"title": {
							"type": "string",
							"description": "A descriptive title for the knowledge item"
						},
						"content": {
							"type": "string",
							"description": "The detailed content or solution"
						},
						"tags": {
							"type": "array",
							"items": {"type": "string"},
							"description": "Tags to help find this later"
						}
					},
					"required": ["title", "content"]
				}`),
			},
		})
	}

	return tools
}

// ExecuteTool executes a tool by name with the given arguments.
func (mt *MemoryTools) ExecuteTool(
	ctx context.Context,
	userID string,
	toolName string,
	argsJSON string,
) (*ToolResult, error) {
	start := time.Now()
	mt.metrics.CallCounts[toolName]++
	mt.metrics.TotalCalls++

	var result *ToolResult
	var err error

	switch toolName {
	case "recall_memory_search":
		result, err = mt.executeRecallSearch(ctx, argsJSON)
	case "core_memory_read":
		result, err = mt.executeCoreRead(ctx, userID, argsJSON)
	case "core_memory_append":
		result, err = mt.executeCoreAppend(ctx, userID, argsJSON)
	case "core_memory_update":
		result, err = mt.executeCoreUpdate(ctx, userID, argsJSON)
	case "archival_memory_search":
		result, err = mt.executeArchivalSearch(ctx, argsJSON)
	case "archival_memory_insert":
		result, err = mt.executeArchivalInsert(ctx, userID, argsJSON)
	default:
		return &ToolResult{
			ToolName: toolName,
			Success:  false,
			Error:    fmt.Sprintf("unknown tool: %s", toolName),
		}, nil
	}

	latency := time.Since(start)
	if result != nil {
		result.LatencyMs = latency.Milliseconds()
	}

	if err != nil {
		mt.metrics.ErrorCounts[toolName]++
		mt.metrics.TotalErrors++
		log.Warn().
			Str("tool", toolName).
			Err(err).
			Msg("tool execution failed")
	}

	log.Debug().
		Str("tool", toolName).
		Bool("success", result != nil && result.Success).
		Int64("latency_ms", latency.Milliseconds()).
		Msg("tool executed")

	return result, err
}

// executeRecallSearch searches conversation history.
// Note: This is a placeholder - actual implementation would search a conversation store.
func (mt *MemoryTools) executeRecallSearch(ctx context.Context, argsJSON string) (*ToolResult, error) {
	var args struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return &ToolResult{
			ToolName: "recall_memory_search",
			Success:  false,
			Error:    fmt.Sprintf("invalid arguments: %v", err),
		}, nil
	}

	if args.Limit <= 0 {
		args.Limit = mt.config.MaxRecallResults
	}

	// TODO: Implement actual conversation search
	// For now, return a placeholder response
	return &ToolResult{
		ToolName: "recall_memory_search",
		Success:  true,
		Result:   fmt.Sprintf("No relevant conversations found for query: %s", args.Query),
	}, nil
}

// executeCoreRead reads core memory sections.
func (mt *MemoryTools) executeCoreRead(ctx context.Context, userID string, argsJSON string) (*ToolResult, error) {
	var args struct {
		Section string `json:"section"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return &ToolResult{
			ToolName: "core_memory_read",
			Success:  false,
			Error:    fmt.Sprintf("invalid arguments: %v", err),
		}, nil
	}

	switch args.Section {
	case "user":
		mem, err := mt.coreStore.GetUserMemory(ctx, userID)
		if err != nil {
			return &ToolResult{
				ToolName: "core_memory_read",
				Success:  false,
				Error:    fmt.Sprintf("failed to read user memory: %v", err),
			}, nil
		}

		// Format as readable text
		result := formatUserMemory(mem)
		return &ToolResult{
			ToolName: "core_memory_read",
			Success:  true,
			Result:   result,
		}, nil

	case "project":
		// TODO: Get project ID from context
		mem, err := mt.coreStore.GetProjectMemory(ctx, "current")
		if err != nil {
			return &ToolResult{
				ToolName: "core_memory_read",
				Success:  false,
				Error:    fmt.Sprintf("failed to read project memory: %v", err),
			}, nil
		}

		result := formatProjectMemory(mem)
		return &ToolResult{
			ToolName: "core_memory_read",
			Success:  true,
			Result:   result,
		}, nil

	default:
		return &ToolResult{
			ToolName: "core_memory_read",
			Success:  false,
			Error:    fmt.Sprintf("unknown section: %s (use 'user' or 'project')", args.Section),
		}, nil
	}
}

// executeCoreAppend adds a fact to core memory.
func (mt *MemoryTools) executeCoreAppend(ctx context.Context, userID string, argsJSON string) (*ToolResult, error) {
	if !mt.config.AllowCoreMemoryWrite {
		return &ToolResult{
			ToolName: "core_memory_append",
			Success:  false,
			Error:    "core memory write is disabled",
		}, nil
	}

	var args struct {
		Fact string `json:"fact"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return &ToolResult{
			ToolName: "core_memory_append",
			Success:  false,
			Error:    fmt.Sprintf("invalid arguments: %v", err),
		}, nil
	}

	if args.Fact == "" {
		return &ToolResult{
			ToolName: "core_memory_append",
			Success:  false,
			Error:    "fact cannot be empty",
		}, nil
	}

	err := mt.coreStore.AppendUserFact(ctx, userID, args.Fact, "llm_learned")
	if err != nil {
		return &ToolResult{
			ToolName: "core_memory_append",
			Success:  false,
			Error:    fmt.Sprintf("failed to save fact: %v", err),
		}, nil
	}

	return &ToolResult{
		ToolName: "core_memory_append",
		Success:  true,
		Result:   fmt.Sprintf("Remembered: %s", args.Fact),
	}, nil
}

// executeCoreUpdate updates a specific profile field in user memory.
func (mt *MemoryTools) executeCoreUpdate(ctx context.Context, userID string, argsJSON string) (*ToolResult, error) {
	if !mt.config.AllowCoreMemoryWrite {
		return &ToolResult{
			ToolName: "core_memory_update",
			Success:  false,
			Error:    "core memory write is disabled",
		}, nil
	}

	var args struct {
		Field string `json:"field"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return &ToolResult{
			ToolName: "core_memory_update",
			Success:  false,
			Error:    fmt.Sprintf("invalid arguments: %v", err),
		}, nil
	}

	if args.Field == "" || args.Value == "" {
		return &ToolResult{
			ToolName: "core_memory_update",
			Success:  false,
			Error:    "both field and value are required",
		}, nil
	}

	// Validate field is one we allow
	allowedFields := map[string]bool{
		"name":       true,
		"role":       true,
		"experience": true,
		"os":         true,
		"shell":      true,
		"editor":     true,
	}
	if !allowedFields[args.Field] {
		return &ToolResult{
			ToolName: "core_memory_update",
			Success:  false,
			Error:    fmt.Sprintf("field '%s' is not allowed - use one of: name, role, experience, os, shell, editor", args.Field),
		}, nil
	}

	err := mt.coreStore.UpdateUserField(ctx, userID, args.Field, args.Value, "llm_learned")
	if err != nil {
		return &ToolResult{
			ToolName: "core_memory_update",
			Success:  false,
			Error:    fmt.Sprintf("failed to update field: %v", err),
		}, nil
	}

	log.Info().
		Str("user_id", userID).
		Str("field", args.Field).
		Str("value", args.Value).
		Msg("profile field updated via LLM")

	return &ToolResult{
		ToolName: "core_memory_update",
		Success:  true,
		Result:   fmt.Sprintf("Updated %s to: %s", args.Field, args.Value),
	}, nil
}

// executeArchivalSearch searches the knowledge base.
func (mt *MemoryTools) executeArchivalSearch(ctx context.Context, argsJSON string) (*ToolResult, error) {
	var args struct {
		Query string `json:"query"`
		Scope string `json:"scope"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return &ToolResult{
			ToolName: "archival_memory_search",
			Success:  false,
			Error:    fmt.Sprintf("invalid arguments: %v", err),
		}, nil
	}

	if args.Limit <= 0 {
		args.Limit = mt.config.MaxArchivalResults
	}

	// Convert scope string to types.Scope
	var scope types.Scope
	switch args.Scope {
	case "personal":
		scope = types.ScopePersonal
	case "team":
		scope = types.ScopeTeam
	case "global":
		scope = types.ScopeGlobal
	default:
		scope = "" // All scopes
	}

	// Build search options - only include scope filter if specified
	searchOpts := types.SearchOptions{
		Limit: args.Limit,
	}
	if scope != "" {
		searchOpts.Tiers = []types.Scope{scope}
	}
	results, err := mt.fabric.Search(ctx, args.Query, searchOpts)
	if err != nil {
		return &ToolResult{
			ToolName: "archival_memory_search",
			Success:  false,
			Error:    fmt.Sprintf("search failed: %v", err),
		}, nil
	}

	if len(results.Items) == 0 {
		return &ToolResult{
			ToolName: "archival_memory_search",
			Success:  true,
			Result:   fmt.Sprintf("No knowledge found for: %s", args.Query),
		}, nil
	}

	// Format results
	resultText := formatSearchResults(results.Items)
	return &ToolResult{
		ToolName: "archival_memory_search",
		Success:  true,
		Result:   resultText,
	}, nil
}

// executeArchivalInsert stores new knowledge.
func (mt *MemoryTools) executeArchivalInsert(ctx context.Context, userID string, argsJSON string) (*ToolResult, error) {
	if !mt.config.AllowArchivalInsert {
		return &ToolResult{
			ToolName: "archival_memory_insert",
			Success:  false,
			Error:    "archival insert is disabled",
		}, nil
	}

	var rawArgs map[string]interface{}
	if err := json.Unmarshal([]byte(argsJSON), &rawArgs); err != nil {
		return &ToolResult{
			ToolName: "archival_memory_insert",
			Success:  false,
			Error:    fmt.Sprintf("invalid arguments: %v", err),
		}, nil
	}

	title, _ := rawArgs["title"].(string)
	content, _ := rawArgs["content"].(string)

	var tags []string
	switch t := rawArgs["tags"].(type) {
	case []interface{}:
		for _, v := range t {
			if s, ok := v.(string); ok {
				tags = append(tags, s)
			}
		}
	case string:
		if t != "" {
			tags = []string{t}
		}
	}

	if title == "" || content == "" {
		return &ToolResult{
			ToolName: "archival_memory_insert",
			Success:  false,
			Error:    "title and content are required",
		}, nil
	}

	authorID := userID
	authorName := userID
	if mt.coreStore != nil {
		if userMem, err := mt.coreStore.GetUserMemory(ctx, userID); err == nil && userMem.Name != "" {
			authorName = userMem.Name
		}
	}

	now := time.Now()
	item := &types.KnowledgeItem{
		ID:         fmt.Sprintf("archival-%d", now.UnixNano()),
		Type:       types.TypeDocument,
		Scope:      types.ScopePersonal,
		Title:      title,
		Content:    content,
		Tags:       tags,
		TrustScore: 0.5,
		AuthorID:   authorID,
		AuthorName: authorName,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	err := mt.fabric.Create(ctx, item)
	if err != nil {
		return &ToolResult{
			ToolName: "archival_memory_insert",
			Success:  false,
			Error:    fmt.Sprintf("failed to save knowledge: %v", err),
		}, nil
	}

	return &ToolResult{
		ToolName: "archival_memory_insert",
		Success:  true,
		Result:   fmt.Sprintf("Saved to knowledge base: %s", title),
	}, nil
}

// formatUserMemory formats user memory as readable text.
func formatUserMemory(mem *UserMemory) string {
	if mem == nil {
		return "No user information available."
	}

	var parts []string

	if mem.Name != "" {
		parts = append(parts, fmt.Sprintf("Name: %s", mem.Name))
	}
	if mem.Role != "" {
		parts = append(parts, fmt.Sprintf("Role: %s", mem.Role))
	}
	if mem.OS != "" {
		parts = append(parts, fmt.Sprintf("OS: %s", mem.OS))
	}
	if mem.Shell != "" {
		parts = append(parts, fmt.Sprintf("Shell: %s", mem.Shell))
	}
	if mem.Editor != "" {
		parts = append(parts, fmt.Sprintf("Editor: %s", mem.Editor))
	}

	if mem.PrefersConcise {
		parts = append(parts, "Prefers concise responses")
	}
	if mem.PrefersVerbose {
		parts = append(parts, "Prefers detailed responses")
	}

	for _, p := range mem.Preferences {
		parts = append(parts, fmt.Sprintf("[%s] %s", p.Category, p.Preference))
	}

	for _, f := range mem.CustomFacts {
		parts = append(parts, fmt.Sprintf("• %s", f.Fact))
	}

	if len(parts) == 0 {
		return "No user information stored yet."
	}

	return fmt.Sprintf("User Memory:\n%s", join(parts, "\n"))
}

// formatProjectMemory formats project memory as readable text.
func formatProjectMemory(mem *ProjectMemory) string {
	if mem == nil || mem.Name == "" {
		return "No project information available."
	}

	var parts []string

	parts = append(parts, fmt.Sprintf("Project: %s", mem.Name))

	if mem.Type != "" {
		parts = append(parts, fmt.Sprintf("Type: %s", mem.Type))
	}
	if mem.Path != "" {
		parts = append(parts, fmt.Sprintf("Path: %s", mem.Path))
	}
	if len(mem.TechStack) > 0 {
		parts = append(parts, fmt.Sprintf("Tech Stack: %s", join(mem.TechStack, ", ")))
	}
	if mem.GitBranch != "" {
		parts = append(parts, fmt.Sprintf("Branch: %s", mem.GitBranch))
	}

	for _, c := range mem.Conventions {
		parts = append(parts, fmt.Sprintf("• %s", c))
	}

	return fmt.Sprintf("Project Memory:\n%s", join(parts, "\n"))
}

// formatSearchResults formats knowledge search results.
func formatSearchResults(items []*types.KnowledgeItem) string {
	if len(items) == 0 {
		return "No results found."
	}

	var parts []string
	for i, item := range items {
		if item == nil {
			continue
		}
		content := item.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		parts = append(parts, fmt.Sprintf("%d. [%s] %s\n   %s",
			i+1, item.Scope, item.Title, content))
	}

	return fmt.Sprintf("Found %d results:\n%s", len(items), join(parts, "\n"))
}

// join is a simple string join helper.
func join(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += sep + parts[i]
	}
	return result
}

// Metrics returns current tool metrics.
func (mt *MemoryTools) Metrics() *ToolMetrics {
	return mt.metrics
}

// Config returns the tools configuration.
func (mt *MemoryTools) Config() MemoryToolsConfig {
	return mt.config
}
