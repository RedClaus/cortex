package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/normanking/cortex/pkg/types"
)

// setupToolsTest creates test dependencies for memory tools.
func setupToolsTest(t *testing.T) (*MemoryTools, *sql.DB) {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	coreStore, err := NewCoreMemoryStore(db, DefaultCoreMemoryConfig())
	if err != nil {
		db.Close()
		t.Fatalf("failed to create core store: %v", err)
	}

	fabric := &mockFabric{
		items: []*types.KnowledgeItem{
			{
				ID:         "kb-1",
				Title:      "Docker Compose Basics",
				Content:    "Use docker-compose up -d to start services",
				Scope:      types.ScopePersonal,
				TrustScore: 0.9,
			},
			{
				ID:         "kb-2",
				Title:      "Kubernetes Debugging",
				Content:    "Use kubectl describe pod to debug issues",
				Scope:      types.ScopeTeam,
				TrustScore: 0.8,
			},
		},
	}

	tools := NewMemoryTools(coreStore, fabric, DefaultMemoryToolsConfig())
	return tools, db
}

// TestMemoryTools_GetToolDefinitions tests tool definitions.
func TestMemoryTools_GetToolDefinitions(t *testing.T) {
	tools, db := setupToolsTest(t)
	defer db.Close()

	defs := tools.GetToolDefinitions()

	// With default config, should have 6 tools (added core_memory_update)
	if len(defs) != 6 {
		t.Errorf("expected 6 tool definitions, got %d", len(defs))
	}

	// Check required tools are present
	toolNames := make(map[string]bool)
	for _, d := range defs {
		toolNames[d.Function.Name] = true
	}

	expectedTools := []string{
		"recall_memory_search",
		"core_memory_read",
		"core_memory_append",
		"core_memory_update",
		"archival_memory_search",
		"archival_memory_insert",
	}

	for _, name := range expectedTools {
		if !toolNames[name] {
			t.Errorf("missing expected tool: %s", name)
		}
	}
}

// TestMemoryTools_GetToolDefinitions_WriteDisabled tests tool definitions with writes disabled.
func TestMemoryTools_GetToolDefinitions_WriteDisabled(t *testing.T) {
	db, _ := sql.Open("sqlite3", ":memory:")
	defer db.Close()

	coreStore, _ := NewCoreMemoryStore(db, DefaultCoreMemoryConfig())
	fabric := &mockFabric{}

	config := DefaultMemoryToolsConfig()
	config.AllowCoreMemoryWrite = false
	config.AllowArchivalInsert = false

	tools := NewMemoryTools(coreStore, fabric, config)
	defs := tools.GetToolDefinitions()

	// With writes disabled, should only have 3 tools
	if len(defs) != 3 {
		t.Errorf("expected 3 tool definitions with writes disabled, got %d", len(defs))
	}

	// Verify write tools are not present
	for _, d := range defs {
		if d.Function.Name == "core_memory_append" || d.Function.Name == "archival_memory_insert" {
			t.Errorf("write tool %s should not be present when disabled", d.Function.Name)
		}
	}
}

// TestMemoryTools_CoreMemoryRead tests reading core memory.
func TestMemoryTools_CoreMemoryRead(t *testing.T) {
	tools, db := setupToolsTest(t)
	defer db.Close()

	ctx := context.Background()
	userID := "test-user-tools"

	// Setup some user data first
	tools.coreStore.UpdateUserField(ctx, userID, "name", "Alice", "test")
	tools.coreStore.UpdateUserField(ctx, userID, "os", "macOS", "test")
	tools.coreStore.AppendUserFact(ctx, userID, "Prefers vim", "test")

	// Execute tool
	argsJSON := `{"section": "user"}`
	result, err := tools.ExecuteTool(ctx, userID, "core_memory_read", argsJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Errorf("tool execution failed: %s", result.Error)
	}

	// Check result contains expected data
	if !strings.Contains(result.Result, "Alice") {
		t.Error("result should contain user name")
	}
	if !strings.Contains(result.Result, "macOS") {
		t.Error("result should contain OS")
	}
	if !strings.Contains(result.Result, "Prefers vim") {
		t.Error("result should contain custom fact")
	}
}

// TestMemoryTools_CoreMemoryRead_EmptyUser tests reading empty user memory.
func TestMemoryTools_CoreMemoryRead_EmptyUser(t *testing.T) {
	tools, db := setupToolsTest(t)
	defer db.Close()

	ctx := context.Background()

	argsJSON := `{"section": "user"}`
	result, _ := tools.ExecuteTool(ctx, "nonexistent-user", "core_memory_read", argsJSON)

	if !result.Success {
		t.Error("should succeed even with no data")
	}

	if !strings.Contains(result.Result, "No user information") {
		t.Error("should indicate no data available")
	}
}

// TestMemoryTools_CoreMemoryRead_InvalidSection tests invalid section.
func TestMemoryTools_CoreMemoryRead_InvalidSection(t *testing.T) {
	tools, db := setupToolsTest(t)
	defer db.Close()

	argsJSON := `{"section": "invalid"}`
	result, _ := tools.ExecuteTool(context.Background(), "user", "core_memory_read", argsJSON)

	if result.Success {
		t.Error("should fail with invalid section")
	}
	if !strings.Contains(result.Error, "unknown section") {
		t.Errorf("error should mention unknown section: %s", result.Error)
	}
}

// TestMemoryTools_CoreMemoryAppend tests appending facts.
func TestMemoryTools_CoreMemoryAppend(t *testing.T) {
	tools, db := setupToolsTest(t)
	defer db.Close()

	ctx := context.Background()
	userID := "test-user-append"

	// Append a fact
	argsJSON := `{"fact": "User prefers dark mode"}`
	result, err := tools.ExecuteTool(ctx, userID, "core_memory_append", argsJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Errorf("tool execution failed: %s", result.Error)
	}

	if !strings.Contains(result.Result, "Remembered") {
		t.Error("result should confirm fact was saved")
	}

	// Verify fact was actually saved
	mem, _ := tools.coreStore.GetUserMemory(ctx, userID)
	found := false
	for _, f := range mem.CustomFacts {
		if strings.Contains(f.Fact, "dark mode") {
			found = true
			break
		}
	}
	if !found {
		t.Error("fact should be saved to core memory")
	}
}

// TestMemoryTools_CoreMemoryAppend_Empty tests empty fact rejection.
func TestMemoryTools_CoreMemoryAppend_Empty(t *testing.T) {
	tools, db := setupToolsTest(t)
	defer db.Close()

	argsJSON := `{"fact": ""}`
	result, _ := tools.ExecuteTool(context.Background(), "user", "core_memory_append", argsJSON)

	if result.Success {
		t.Error("should reject empty fact")
	}
}

// TestMemoryTools_CoreMemoryAppend_Disabled tests write when disabled.
func TestMemoryTools_CoreMemoryAppend_Disabled(t *testing.T) {
	db, _ := sql.Open("sqlite3", ":memory:")
	defer db.Close()

	coreStore, _ := NewCoreMemoryStore(db, DefaultCoreMemoryConfig())
	config := DefaultMemoryToolsConfig()
	config.AllowCoreMemoryWrite = false

	tools := NewMemoryTools(coreStore, &mockFabric{}, config)

	argsJSON := `{"fact": "Test fact"}`
	result, _ := tools.ExecuteTool(context.Background(), "user", "core_memory_append", argsJSON)

	if result.Success {
		t.Error("should fail when writes are disabled")
	}
	if !strings.Contains(result.Error, "disabled") {
		t.Error("error should indicate writes are disabled")
	}
}

// TestMemoryTools_ArchivalSearch tests knowledge base search.
func TestMemoryTools_ArchivalSearch(t *testing.T) {
	tools, db := setupToolsTest(t)
	defer db.Close()

	argsJSON := `{"query": "docker compose"}`
	result, _ := tools.ExecuteTool(context.Background(), "user", "archival_memory_search", argsJSON)

	if !result.Success {
		t.Errorf("search should succeed: %s", result.Error)
	}

	// Should find our mock data
	if !strings.Contains(result.Result, "Docker Compose") {
		t.Error("result should contain matching knowledge")
	}
}

// TestMemoryTools_ArchivalSearch_WithScope tests scoped search.
func TestMemoryTools_ArchivalSearch_WithScope(t *testing.T) {
	// Create fabric that respects scope
	fabric := &mockFabric{
		searchFunc: func(ctx context.Context, query string, opts types.SearchOptions) (*types.RetrievalResult, error) {
			// Return different results based on Tiers
			if len(opts.Tiers) > 0 && opts.Tiers[0] == types.ScopeTeam {
				return &types.RetrievalResult{
					Items: []*types.KnowledgeItem{
						{ID: "team-1", Title: "Team Only Item", Content: "Team content", Scope: types.ScopeTeam},
					},
					Tier: types.TierFuzzy,
				}, nil
			}
			return &types.RetrievalResult{
				Items: []*types.KnowledgeItem{
					{ID: "all-1", Title: "All Scopes Item", Content: "All content", Scope: types.ScopePersonal},
				},
				Tier: types.TierFuzzy,
			}, nil
		},
	}

	db, _ := sql.Open("sqlite3", ":memory:")
	defer db.Close()
	coreStore, _ := NewCoreMemoryStore(db, DefaultCoreMemoryConfig())
	tools := NewMemoryTools(coreStore, fabric, DefaultMemoryToolsConfig())

	// Search with scope
	argsJSON := `{"query": "test", "scope": "team"}`
	result, _ := tools.ExecuteTool(context.Background(), "user", "archival_memory_search", argsJSON)

	if !strings.Contains(result.Result, "Team Only") {
		t.Error("scoped search should return team items")
	}
}

// TestMemoryTools_ArchivalInsert tests knowledge insertion.
func TestMemoryTools_ArchivalInsert(t *testing.T) {
	// Create fabric that tracks creates
	var createdItem *types.KnowledgeItem
	fabric := &mockFabric{
		searchFunc: func(ctx context.Context, query string, opts types.SearchOptions) (*types.RetrievalResult, error) {
			return &types.RetrievalResult{Items: []*types.KnowledgeItem{}}, nil
		},
	}

	// Override Create to capture
	originalCreate := fabric.Create
	_ = originalCreate // Satisfy compiler
	fabric.items = nil // Clear items

	db, _ := sql.Open("sqlite3", ":memory:")
	defer db.Close()
	coreStore, _ := NewCoreMemoryStore(db, DefaultCoreMemoryConfig())
	tools := NewMemoryTools(coreStore, fabric, DefaultMemoryToolsConfig())

	// Insert knowledge (note: mockFabric.Create does nothing, but we test the flow)
	argsJSON := `{
		"title": "Git Stash Workflow",
		"content": "Use git stash to save changes temporarily",
		"tags": ["git", "workflow"]
	}`
	result, _ := tools.ExecuteTool(context.Background(), "user", "archival_memory_insert", argsJSON)

	// Since mockFabric.Create returns nil (success), this should succeed
	if !result.Success {
		t.Errorf("insert should succeed: %s", result.Error)
	}

	if !strings.Contains(result.Result, "Saved to knowledge base") {
		t.Error("result should confirm save")
	}

	// Cleanup
	_ = createdItem
}

// TestMemoryTools_ArchivalInsert_Disabled tests insert when disabled.
func TestMemoryTools_ArchivalInsert_Disabled(t *testing.T) {
	db, _ := sql.Open("sqlite3", ":memory:")
	defer db.Close()

	coreStore, _ := NewCoreMemoryStore(db, DefaultCoreMemoryConfig())
	config := DefaultMemoryToolsConfig()
	config.AllowArchivalInsert = false

	tools := NewMemoryTools(coreStore, &mockFabric{}, config)

	argsJSON := `{"title": "Test", "content": "Test content"}`
	result, _ := tools.ExecuteTool(context.Background(), "user", "archival_memory_insert", argsJSON)

	if result.Success {
		t.Error("should fail when inserts are disabled")
	}
}

// TestMemoryTools_ArchivalInsert_MissingFields tests required field validation.
func TestMemoryTools_ArchivalInsert_MissingFields(t *testing.T) {
	tools, db := setupToolsTest(t)
	defer db.Close()

	testCases := []struct {
		name     string
		argsJSON string
	}{
		{"missing title", `{"content": "Test content"}`},
		{"missing content", `{"title": "Test"}`},
		{"empty title", `{"title": "", "content": "Test content"}`},
		{"empty content", `{"title": "Test", "content": ""}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, _ := tools.ExecuteTool(context.Background(), "user", "archival_memory_insert", tc.argsJSON)
			if result.Success {
				t.Errorf("%s should fail", tc.name)
			}
		})
	}
}

// TestMemoryTools_UnknownTool tests handling of unknown tool.
func TestMemoryTools_UnknownTool(t *testing.T) {
	tools, db := setupToolsTest(t)
	defer db.Close()

	result, err := tools.ExecuteTool(context.Background(), "user", "unknown_tool", "{}")
	if err != nil {
		t.Fatalf("should not return error for unknown tool: %v", err)
	}

	if result.Success {
		t.Error("unknown tool should fail")
	}
	if !strings.Contains(result.Error, "unknown tool") {
		t.Error("error should mention unknown tool")
	}
}

// TestMemoryTools_InvalidJSON tests handling of invalid JSON arguments.
func TestMemoryTools_InvalidJSON(t *testing.T) {
	tools, db := setupToolsTest(t)
	defer db.Close()

	result, _ := tools.ExecuteTool(context.Background(), "user", "core_memory_read", "not valid json")

	if result.Success {
		t.Error("should fail with invalid JSON")
	}
	if !strings.Contains(result.Error, "invalid arguments") {
		t.Error("error should mention invalid arguments")
	}
}

// TestMemoryTools_Metrics tests metrics tracking.
func TestMemoryTools_Metrics(t *testing.T) {
	tools, db := setupToolsTest(t)
	defer db.Close()

	ctx := context.Background()

	// Execute some tools
	tools.ExecuteTool(ctx, "user", "core_memory_read", `{"section": "user"}`)
	tools.ExecuteTool(ctx, "user", "archival_memory_search", `{"query": "test"}`)
	tools.ExecuteTool(ctx, "user", "unknown_tool", `{}`) // This will fail

	metrics := tools.Metrics()

	if metrics.TotalCalls != 3 {
		t.Errorf("expected 3 total calls, got %d", metrics.TotalCalls)
	}

	if metrics.CallCounts["core_memory_read"] != 1 {
		t.Error("core_memory_read should have 1 call")
	}
	if metrics.CallCounts["archival_memory_search"] != 1 {
		t.Error("archival_memory_search should have 1 call")
	}
}

// TestMemoryTools_ToolDefinitionsFormat tests that tool definitions are valid JSON schema.
func TestMemoryTools_ToolDefinitionsFormat(t *testing.T) {
	tools, db := setupToolsTest(t)
	defer db.Close()

	defs := tools.GetToolDefinitions()

	for _, def := range defs {
		// Verify type is "function"
		if def.Type != "function" {
			t.Errorf("tool %s should have type 'function'", def.Function.Name)
		}

		// Verify name is not empty
		if def.Function.Name == "" {
			t.Error("tool name should not be empty")
		}

		// Verify description is not empty
		if def.Function.Description == "" {
			t.Errorf("tool %s should have description", def.Function.Name)
		}

		// Verify parameters is valid JSON
		var params map[string]interface{}
		if err := json.Unmarshal(def.Function.Parameters, &params); err != nil {
			t.Errorf("tool %s has invalid parameter schema: %v", def.Function.Name, err)
		}

		// Verify parameters has "type" field
		if _, ok := params["type"]; !ok {
			t.Errorf("tool %s parameters should have 'type' field", def.Function.Name)
		}
	}
}

// TestMemoryTools_RecallSearch tests recall search placeholder.
func TestMemoryTools_RecallSearch(t *testing.T) {
	tools, db := setupToolsTest(t)
	defer db.Close()

	argsJSON := `{"query": "previous conversation about kubernetes"}`
	result, _ := tools.ExecuteTool(context.Background(), "user", "recall_memory_search", argsJSON)

	// Should succeed (even though it's a placeholder)
	if !result.Success {
		t.Errorf("recall search should succeed: %s", result.Error)
	}

	// Result should indicate no conversations found (placeholder behavior)
	if !strings.Contains(result.Result, "No relevant conversations") {
		t.Error("placeholder should indicate no conversations")
	}
}
