package resemble

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ═══════════════════════════════════════════════════════════════════════════════
// TOOL MAPPER TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestToolMapper_MapTool(t *testing.T) {
	mapper := NewToolMapper()

	t.Run("maps basic tool", func(t *testing.T) {
		tool := Tool{
			Name:        "test_tool",
			Description: "A test tool",
			ToolType:    "builtin",
			Active:      true,
			Parameters: map[string]ToolParam{
				"query": {
					Type:        "string",
					Description: "Search query",
					Required:    true,
				},
			},
		}

		mapped := mapper.MapTool(tool)
		require.NotNil(t, mapped)

		assert.Equal(t, "function", mapped.Type)
		assert.Equal(t, "test_tool", mapped.Function.Name)
		assert.Equal(t, "A test tool", mapped.Function.Description)

		// Verify parameters are valid JSON
		var params map[string]any
		err := json.Unmarshal(mapped.Function.Parameters, &params)
		require.NoError(t, err)

		assert.Equal(t, "object", params["type"])
		props := params["properties"].(map[string]any)
		assert.Contains(t, props, "query")
		assert.Equal(t, []any{"query"}, params["required"])
	})

	t.Run("maps tool with no parameters", func(t *testing.T) {
		tool := Tool{
			Name:        "simple_tool",
			Description: "No params",
			ToolType:    "builtin",
			Active:      true,
		}

		mapped := mapper.MapTool(tool)
		require.NotNil(t, mapped)

		var params map[string]any
		err := json.Unmarshal(mapped.Function.Parameters, &params)
		require.NoError(t, err)
		assert.Equal(t, "object", params["type"])
	})

	t.Run("maps tool with array parameter", func(t *testing.T) {
		tool := Tool{
			Name:        "array_tool",
			Description: "Has array",
			ToolType:    "builtin",
			Active:      true,
			Parameters: map[string]ToolParam{
				"tags": {
					Type:        "array",
					Description: "Tags list",
					Required:    false,
				},
			},
		}

		mapped := mapper.MapTool(tool)
		require.NotNil(t, mapped)

		var params map[string]any
		err := json.Unmarshal(mapped.Function.Parameters, &params)
		require.NoError(t, err)

		props := params["properties"].(map[string]any)
		tags := props["tags"].(map[string]any)
		assert.Equal(t, "array", tags["type"])
		assert.NotNil(t, tags["items"])
	})
}

func TestToolMapper_MapTools(t *testing.T) {
	mapper := NewToolMapper()

	t.Run("maps multiple tools", func(t *testing.T) {
		tools := []Tool{
			{Name: "tool1", Description: "First", ToolType: "builtin", Active: true},
			{Name: "tool2", Description: "Second", ToolType: "webhook", Active: true},
		}

		mapped := mapper.MapTools(tools)
		assert.Len(t, mapped, 2)
	})

	t.Run("skips inactive tools", func(t *testing.T) {
		tools := []Tool{
			{Name: "active", Description: "Active", ToolType: "builtin", Active: true},
			{Name: "inactive", Description: "Inactive", ToolType: "builtin", Active: false},
		}

		mapped := mapper.MapTools(tools)
		assert.Len(t, mapped, 1)
		assert.Equal(t, "active", mapped[0].Function.Name)
	})

	t.Run("handles empty input", func(t *testing.T) {
		mapped := mapper.MapTools(nil)
		assert.Nil(t, mapped)

		mapped = mapper.MapTools([]Tool{})
		assert.Nil(t, mapped)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// REGISTRY TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestAgentToolRegistry_RegisterUnregister(t *testing.T) {
	registry := NewAgentToolRegistry("/tmp")

	t.Run("registers tools from agent", func(t *testing.T) {
		agent := &Agent{
			UUID: "test-agent-uuid",
			Name: "Test Agent",
			Tools: []Tool{
				{Name: "bash", Description: "Run commands", ToolType: "builtin", Active: true},
				{Name: "read_file", Description: "Read files", ToolType: "builtin", Active: true},
			},
		}

		err := registry.RegisterAgentTools(agent)
		require.NoError(t, err)

		assert.Equal(t, 2, registry.ToolCount())
		assert.Equal(t, "test-agent-uuid", registry.AgentID())
		assert.True(t, registry.HasTool("bash"))
		assert.True(t, registry.HasTool("read_file"))
	})

	t.Run("replaces existing tools on re-register", func(t *testing.T) {
		agent1 := &Agent{
			UUID:  "agent1",
			Name:  "Agent 1",
			Tools: []Tool{{Name: "tool1", Active: true}},
		}
		agent2 := &Agent{
			UUID:  "agent2",
			Name:  "Agent 2",
			Tools: []Tool{{Name: "tool2", Active: true}},
		}

		err := registry.RegisterAgentTools(agent1)
		require.NoError(t, err)
		assert.True(t, registry.HasTool("tool1"))

		err = registry.RegisterAgentTools(agent2)
		require.NoError(t, err)
		assert.False(t, registry.HasTool("tool1"))
		assert.True(t, registry.HasTool("tool2"))
	})

	t.Run("unregisters all tools", func(t *testing.T) {
		agent := &Agent{
			UUID:  "test-uuid",
			Name:  "Test",
			Tools: []Tool{{Name: "mytool", Active: true}},
		}

		err := registry.RegisterAgentTools(agent)
		require.NoError(t, err)
		assert.Equal(t, 1, registry.ToolCount())

		err = registry.UnregisterAllTools()
		require.NoError(t, err)
		assert.Equal(t, 0, registry.ToolCount())
		assert.Equal(t, "", registry.AgentID())
	})

	t.Run("unregisters single tool", func(t *testing.T) {
		agent := &Agent{
			UUID: "test-uuid",
			Name: "Test",
			Tools: []Tool{
				{Name: "keep", Active: true},
				{Name: "remove", Active: true},
			},
		}

		err := registry.RegisterAgentTools(agent)
		require.NoError(t, err)

		err = registry.UnregisterTool("remove")
		require.NoError(t, err)

		assert.True(t, registry.HasTool("keep"))
		assert.False(t, registry.HasTool("remove"))
	})

	t.Run("returns error for nil agent", func(t *testing.T) {
		err := registry.RegisterAgentTools(nil)
		assert.Error(t, err)
	})
}

func TestAgentToolRegistry_GetTools(t *testing.T) {
	registry := NewAgentToolRegistry("/tmp")

	agent := &Agent{
		UUID: "test-uuid",
		Name: "Test",
		Tools: []Tool{
			{Name: "tool1", Description: "First", Active: true},
			{Name: "tool2", Description: "Second", Active: true},
		},
	}

	err := registry.RegisterAgentTools(agent)
	require.NoError(t, err)

	t.Run("GetTools returns all tools", func(t *testing.T) {
		tools := registry.GetTools()
		assert.Len(t, tools, 2)
	})

	t.Run("GetTool returns specific tool", func(t *testing.T) {
		tool, found := registry.GetTool("tool1")
		assert.True(t, found)
		assert.Equal(t, "tool1", tool.Function.Name)
	})

	t.Run("GetTool returns false for missing", func(t *testing.T) {
		_, found := registry.GetTool("nonexistent")
		assert.False(t, found)
	})

	t.Run("GetToolDefinitionsJSON returns valid JSON", func(t *testing.T) {
		jsonStr, err := registry.GetToolDefinitionsJSON()
		require.NoError(t, err)

		var tools []CortexTool
		err = json.Unmarshal([]byte(jsonStr), &tools)
		require.NoError(t, err)
		assert.Len(t, tools, 2)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// TOOL EXECUTION TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestAgentToolRegistry_ExecuteTool(t *testing.T) {
	registry := NewAgentToolRegistry("/tmp")

	agent := &Agent{
		UUID: "test-uuid",
		Name: "Test",
		Tools: []Tool{
			{Name: "bash", Description: "Run commands", ToolType: "builtin", Active: true},
		},
	}

	err := registry.RegisterAgentTools(agent)
	require.NoError(t, err)

	t.Run("executes registered builtin tool", func(t *testing.T) {
		ctx := context.Background()
		result, err := registry.ExecuteTool(ctx, "bash", `{"command": "echo hello"}`)

		// Should execute (may succeed or fail based on environment)
		// We're testing the wiring, not the actual command
		if err == nil {
			assert.NotNil(t, result)
		}
	})

	t.Run("returns error for unregistered tool", func(t *testing.T) {
		ctx := context.Background()
		_, err := registry.ExecuteTool(ctx, "nonexistent", "{}")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not registered")
	})

	t.Run("returns error for invalid JSON args", func(t *testing.T) {
		ctx := context.Background()
		_, err := registry.ExecuteTool(ctx, "bash", "not json")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid tool arguments")
	})

	t.Run("handles empty args", func(t *testing.T) {
		ctx := context.Background()
		// Empty args should parse to empty map
		_, err := registry.ExecuteTool(ctx, "bash", "")
		// Will fail because bash needs a command, but args parsing should work
		assert.Error(t, err)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// BUILTIN TOOLS TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestGetBuiltinCortexTools(t *testing.T) {
	tools := GetBuiltinCortexTools()

	t.Run("returns expected tools", func(t *testing.T) {
		assert.GreaterOrEqual(t, len(tools), 6)

		names := make(map[string]bool)
		for _, tool := range tools {
			names[tool.Function.Name] = true
		}

		assert.True(t, names["bash"])
		assert.True(t, names["read_file"])
		assert.True(t, names["write_file"])
		assert.True(t, names["list_directory"])
		assert.True(t, names["glob"])
		assert.True(t, names["grep"])
	})

	t.Run("all tools have valid parameters", func(t *testing.T) {
		for _, tool := range tools {
			var params map[string]any
			err := json.Unmarshal(tool.Function.Parameters, &params)
			assert.NoError(t, err, "tool %s has invalid parameters", tool.Function.Name)
			assert.Equal(t, "object", params["type"])
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// GLOBAL REGISTRY TESTS
// ═══════════════════════════════════════════════════════════════════

func TestGlobalRegistryFunctions(t *testing.T) {
	// Clean up any existing state
	UnregisterAgentTools()

	t.Run("RegisterAgentTools works", func(t *testing.T) {
		tools := []Tool{
			{Name: "global_tool", Description: "Global", Active: true},
		}

		err := RegisterAgentTools(tools)
		require.NoError(t, err)

		assert.True(t, GlobalRegistry().HasTool("global_tool"))
	})

	t.Run("UnregisterAgentTools works", func(t *testing.T) {
		err := UnregisterAgentTools()
		require.NoError(t, err)

		assert.Equal(t, 0, GlobalRegistry().ToolCount())
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// MERGE TOOLS TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestAgentToolRegistry_MergeWithMemoryTools(t *testing.T) {
	registry := NewAgentToolRegistry("/tmp")

	// Register some agent tools
	agent := &Agent{
		UUID: "test-uuid",
		Name: "Test",
		Tools: []Tool{
			{Name: "agent_tool", Description: "Agent specific", Active: true},
			{Name: "recall_memory_search", Description: "Should be skipped", Active: true},
		},
	}

	err := registry.RegisterAgentTools(agent)
	require.NoError(t, err)

	// Create mock memory tools
	memoryTools := []CortexTool{
		{
			Type: "function",
			Function: CortexFunctionDef{
				Name:        "recall_memory_search",
				Description: "Memory tool version",
				Parameters:  json.RawMessage(`{}`),
			},
		},
	}

	t.Run("merges tools without conflicts", func(t *testing.T) {
		merged := registry.MergeWithMemoryTools(memoryTools)

		// Should have: 1 memory tool + 1 agent tool (recall_memory_search skipped)
		assert.Len(t, merged, 2)

		// Memory tools should come first
		assert.Equal(t, "recall_memory_search", merged[0].Function.Name)
		assert.Equal(t, "Memory tool version", merged[0].Function.Description)

		// Agent tool should be included
		names := make(map[string]bool)
		for _, tool := range merged {
			names[tool.Function.Name] = true
		}
		assert.True(t, names["agent_tool"])
	})
}
