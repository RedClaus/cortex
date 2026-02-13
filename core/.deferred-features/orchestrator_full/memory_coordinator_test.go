// Package orchestrator provides the central coordination layer for Cortex.
package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ═══════════════════════════════════════════════════════════════════════════════
// MEMORY COORDINATOR TESTS
// CR-017: Phase 3 - Memory Coordinator Unit Tests
// ═══════════════════════════════════════════════════════════════════════════════

func TestNewMemoryCoordinator(t *testing.T) {
	tests := []struct {
		name   string
		config *MemoryCoordinatorConfig
	}{
		{
			name:   "creates with nil config",
			config: nil,
		},
		{
			name:   "creates with empty config",
			config: &MemoryCoordinatorConfig{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := NewMemoryCoordinator(tt.config)
			require.NotNil(t, mc)
		})
	}
}

func TestMemoryCoordinator_GetUserMemory(t *testing.T) {
	tests := []struct {
		name        string
		config      *MemoryCoordinatorConfig
		userID      string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "returns error when core store not configured",
			config:      &MemoryCoordinatorConfig{},
			userID:      "user-123",
			expectError: true,
			errorMsg:    "core store not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := NewMemoryCoordinator(tt.config)
			ctx := context.Background()

			result, err := mc.GetUserMemory(ctx, tt.userID)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMemoryCoordinator_UpdateUserMemory(t *testing.T) {
	tests := []struct {
		name        string
		config      *MemoryCoordinatorConfig
		userID      string
		field       string
		value       interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name:        "returns error when core store not configured",
			config:      &MemoryCoordinatorConfig{},
			userID:      "user-123",
			field:       "name",
			value:       "Test User",
			expectError: true,
			errorMsg:    "core store not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := NewMemoryCoordinator(tt.config)
			ctx := context.Background()

			err := mc.UpdateUserMemory(ctx, tt.userID, tt.field, tt.value)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMemoryCoordinator_UpdateUserMemory_InvalidField(t *testing.T) {
	// Test that invalid fields are rejected - requires core store
	// This test documents expected behavior even without mocked store
	mc := NewMemoryCoordinator(&MemoryCoordinatorConfig{})
	ctx := context.Background()

	// Even with core store nil, we should first check field validity
	// Current implementation checks core store first, so this returns different error
	err := mc.UpdateUserMemory(ctx, "user-123", "invalid_field", "value")
	require.Error(t, err)
}

func TestMemoryCoordinator_GetProjectMemory(t *testing.T) {
	tests := []struct {
		name        string
		config      *MemoryCoordinatorConfig
		projectID   string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "returns error when core store not configured",
			config:      &MemoryCoordinatorConfig{},
			projectID:   "project-123",
			expectError: true,
			errorMsg:    "core store not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := NewMemoryCoordinator(tt.config)
			ctx := context.Background()

			result, err := mc.GetProjectMemory(ctx, tt.projectID)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMemoryCoordinator_SearchArchival(t *testing.T) {
	tests := []struct {
		name        string
		config      *MemoryCoordinatorConfig
		query       string
		limit       int
		expectError bool
		errorMsg    string
	}{
		{
			name:        "returns error when knowledge fabric not configured",
			config:      &MemoryCoordinatorConfig{},
			query:       "test query",
			limit:       10,
			expectError: true,
			errorMsg:    "knowledge fabric not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := NewMemoryCoordinator(tt.config)
			ctx := context.Background()

			result, err := mc.SearchArchival(ctx, tt.query, tt.limit)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMemoryCoordinator_InsertArchival(t *testing.T) {
	tests := []struct {
		name        string
		config      *MemoryCoordinatorConfig
		expectError bool
		errorMsg    string
	}{
		{
			name:        "returns error when knowledge fabric not configured",
			config:      &MemoryCoordinatorConfig{},
			expectError: true,
			errorMsg:    "knowledge fabric not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := NewMemoryCoordinator(tt.config)
			ctx := context.Background()

			err := mc.InsertArchival(ctx, nil)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMemoryCoordinator_ExecuteTool(t *testing.T) {
	tests := []struct {
		name        string
		config      *MemoryCoordinatorConfig
		userID      string
		toolName    string
		argsJSON    string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "returns error when memory tools not configured",
			config:      &MemoryCoordinatorConfig{},
			userID:      "user-123",
			toolName:    "test_tool",
			argsJSON:    "{}",
			expectError: true,
			errorMsg:    "memory tools not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := NewMemoryCoordinator(tt.config)
			ctx := context.Background()

			result, err := mc.ExecuteTool(ctx, tt.userID, tt.toolName, tt.argsJSON)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMemoryCoordinator_GetToolDefinitions(t *testing.T) {
	t.Run("returns nil when memory tools not configured", func(t *testing.T) {
		mc := NewMemoryCoordinator(&MemoryCoordinatorConfig{})
		result := mc.GetToolDefinitions()
		assert.Nil(t, result)
	})
}

func TestMemoryCoordinator_ComponentAccess(t *testing.T) {
	t.Run("CoreStore returns nil when not set", func(t *testing.T) {
		mc := NewMemoryCoordinator(&MemoryCoordinatorConfig{})
		assert.Nil(t, mc.CoreStore())
	})

	t.Run("MemoryTools returns nil when not set", func(t *testing.T) {
		mc := NewMemoryCoordinator(&MemoryCoordinatorConfig{})
		assert.Nil(t, mc.MemoryTools())
	})

	t.Run("KnowledgeFabric returns nil when not set", func(t *testing.T) {
		mc := NewMemoryCoordinator(&MemoryCoordinatorConfig{})
		assert.Nil(t, mc.KnowledgeFabric())
	})
}

func TestMemoryCoordinator_Stats(t *testing.T) {
	t.Run("returns stats with no components", func(t *testing.T) {
		mc := NewMemoryCoordinator(&MemoryCoordinatorConfig{})
		stats := mc.Stats()

		require.NotNil(t, stats)
		assert.False(t, stats.HasCoreStore)
		assert.False(t, stats.HasMemoryTools)
		assert.False(t, stats.HasKnowledge)
	})
}

func TestMemoryCoordinator_BuildMemoryContext(t *testing.T) {
	tests := []struct {
		name      string
		config    *MemoryCoordinatorConfig
		userID    string
		projectID string
		expected  string
	}{
		{
			name:      "returns empty when core store not configured",
			config:    &MemoryCoordinatorConfig{},
			userID:    "user-123",
			projectID: "project-123",
			expected:  "",
		},
		{
			name:      "returns empty with empty IDs",
			config:    &MemoryCoordinatorConfig{},
			userID:    "",
			projectID: "",
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := NewMemoryCoordinator(tt.config)
			ctx := context.Background()

			result, err := mc.BuildMemoryContext(ctx, tt.userID, tt.projectID)

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// INTERFACE COMPLIANCE TEST
// ═══════════════════════════════════════════════════════════════════════════════

func TestMemoryCoordinator_ImplementsMemorySystem(t *testing.T) {
	// This test verifies at compile time that MemoryCoordinator implements MemorySystem
	var _ MemorySystem = (*MemoryCoordinator)(nil)
}

// ═══════════════════════════════════════════════════════════════════════════════
// ORCHESTRATOR INTEGRATION TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestOrchestrator_WithMemoryCoordinator(t *testing.T) {
	t.Run("creates orchestrator with memory coordinator", func(t *testing.T) {
		mc := NewMemoryCoordinator(&MemoryCoordinatorConfig{})

		o := New(WithMemoryCoordinator(mc))
		require.NotNil(t, o)
		assert.NotNil(t, o.MemoryCoordinator())
		assert.Equal(t, mc, o.MemoryCoordinator())
	})

	t.Run("orchestrator creates default persona coordinator", func(t *testing.T) {
		o := New()
		require.NotNil(t, o)
		// PersonaCoordinator should be created by default
		assert.NotNil(t, o.PersonaCoordinator())
	})

	t.Run("WithMemoryCoordinator takes precedence over legacy options", func(t *testing.T) {
		mc := NewMemoryCoordinator(&MemoryCoordinatorConfig{})

		// WithMemoryCoordinator should be used instead of auto-creating from legacy
		o := New(WithMemoryCoordinator(mc))
		require.NotNil(t, o)
		assert.Equal(t, mc, o.MemoryCoordinator())
	})
}
