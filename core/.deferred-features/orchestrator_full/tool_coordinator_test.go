// Package orchestrator provides the central coordination layer for Cortex.
package orchestrator

import (
	"context"
	"testing"

	"github.com/normanking/cortex/internal/fingerprint"
	"github.com/normanking/cortex/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ═══════════════════════════════════════════════════════════════════════════════
// TOOL COORDINATOR TESTS
// CR-017: Phase 4 - Tool Coordinator Unit Tests
// ═══════════════════════════════════════════════════════════════════════════════

func TestNewToolCoordinator(t *testing.T) {
	tests := []struct {
		name   string
		config *ToolCoordinatorConfig
	}{
		{
			name:   "creates with nil config",
			config: nil,
		},
		{
			name:   "creates with empty config",
			config: &ToolCoordinatorConfig{},
		},
		{
			name: "creates with executor",
			config: &ToolCoordinatorConfig{
				Executor: tools.NewExecutor(),
			},
		},
		{
			name: "creates with fingerprinter",
			config: &ToolCoordinatorConfig{
				Fingerprinter: fingerprint.NewFingerprinter(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := NewToolCoordinator(tt.config)
			require.NotNil(t, tc)

			// Verify defaults are created
			assert.NotNil(t, tc.Executor(), "Executor should be created by default")
			assert.NotNil(t, tc.Fingerprint(), "Fingerprinter should be created by default")
		})
	}
}

func TestToolCoordinator_Execute(t *testing.T) {
	t.Run("executes bash tool successfully", func(t *testing.T) {
		tc := NewToolCoordinator(&ToolCoordinatorConfig{})

		// Register the bash tool
		tc.Register(tools.NewBashTool())

		req := &tools.ToolRequest{
			Tool:  tools.ToolBash,
			Input: "echo hello",
		}

		ctx := context.Background()
		result, err := tc.Execute(ctx, req)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.Success)
	})

	t.Run("returns error for unknown tool", func(t *testing.T) {
		tc := NewToolCoordinator(&ToolCoordinatorConfig{})
		// Don't register any tools

		req := &tools.ToolRequest{
			Tool:  "unknown_tool",
			Input: "test",
		}

		ctx := context.Background()
		result, err := tc.Execute(ctx, req)

		// Executor returns both error and result for unknown tools
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown tool")
		// Result may or may not be nil depending on implementation
		if result != nil {
			assert.False(t, result.Success)
		}
	})
}

func TestToolCoordinator_Register(t *testing.T) {
	t.Run("registers tool successfully", func(t *testing.T) {
		tc := NewToolCoordinator(&ToolCoordinatorConfig{})

		tc.Register(tools.NewBashTool())

		tool, ok := tc.GetTool("bash")
		assert.True(t, ok)
		assert.NotNil(t, tool)
	})

	t.Run("handles nil executor gracefully", func(t *testing.T) {
		tc := NewToolCoordinator(&ToolCoordinatorConfig{})
		tc.executor = nil // Simulate nil executor

		// Should not panic, just log warning
		tc.Register(tools.NewBashTool())
	})
}

func TestToolCoordinator_GetTool(t *testing.T) {
	t.Run("returns registered tool", func(t *testing.T) {
		tc := NewToolCoordinator(&ToolCoordinatorConfig{})
		tc.Register(tools.NewBashTool())

		tool, ok := tc.GetTool("bash")
		assert.True(t, ok)
		assert.NotNil(t, tool)
	})

	t.Run("returns false for unknown tool", func(t *testing.T) {
		tc := NewToolCoordinator(&ToolCoordinatorConfig{})

		tool, ok := tc.GetTool("unknown")
		assert.False(t, ok)
		assert.Nil(t, tool)
	})

	t.Run("returns false when executor is nil", func(t *testing.T) {
		tc := NewToolCoordinator(&ToolCoordinatorConfig{})
		tc.executor = nil // Simulate nil executor

		tool, ok := tc.GetTool("bash")
		assert.False(t, ok)
		assert.Nil(t, tool)
	})
}

func TestToolCoordinator_ListTools(t *testing.T) {
	t.Run("lists registered tools", func(t *testing.T) {
		tc := NewToolCoordinator(&ToolCoordinatorConfig{})
		tc.Register(tools.NewBashTool())
		tc.Register(tools.NewReadTool())

		defs := tc.ListTools()
		assert.GreaterOrEqual(t, len(defs), 2)
	})

	t.Run("returns empty list when no tools registered", func(t *testing.T) {
		tc := NewToolCoordinator(&ToolCoordinatorConfig{})

		defs := tc.ListTools()
		assert.Empty(t, defs)
	})

	t.Run("returns nil when executor is nil", func(t *testing.T) {
		tc := NewToolCoordinator(&ToolCoordinatorConfig{})
		tc.executor = nil // Simulate nil executor

		defs := tc.ListTools()
		assert.Nil(t, defs)
	})
}

func TestToolCoordinator_ValidateArgs(t *testing.T) {
	t.Run("validates known tool args", func(t *testing.T) {
		tc := NewToolCoordinator(&ToolCoordinatorConfig{})
		tc.Register(tools.NewBashTool())

		err := tc.ValidateArgs("bash", map[string]any{
			"command": "echo hello",
		})

		// Bash tool requires command, should validate
		assert.NoError(t, err)
	})

	t.Run("returns error for unknown tool", func(t *testing.T) {
		tc := NewToolCoordinator(&ToolCoordinatorConfig{})

		err := tc.ValidateArgs("unknown", map[string]any{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown tool")
	})
}

func TestToolCoordinator_DetectProject(t *testing.T) {
	t.Run("detects project in current directory", func(t *testing.T) {
		tc := NewToolCoordinator(&ToolCoordinatorConfig{})
		ctx := context.Background()

		fp, err := tc.DetectProject(ctx, ".")

		// Should not error, even if no project detected
		assert.NoError(t, err)
		assert.NotNil(t, fp)
	})

	t.Run("returns error when fingerprinter not configured", func(t *testing.T) {
		tc := NewToolCoordinator(&ToolCoordinatorConfig{})
		tc.fingerprinter = nil // Simulate nil fingerprinter
		ctx := context.Background()

		fp, err := tc.DetectProject(ctx, ".")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "fingerprinter not configured")
		assert.Nil(t, fp)
	})
}

func TestToolCoordinator_Stats(t *testing.T) {
	t.Run("returns stats with no tools", func(t *testing.T) {
		tc := NewToolCoordinator(&ToolCoordinatorConfig{})
		stats := tc.Stats()

		require.NotNil(t, stats)
		assert.Equal(t, 0, stats.ToolCount)
		assert.True(t, stats.HasFingerprinter)
		assert.False(t, stats.HasTaskManager)
	})

	t.Run("counts registered tools", func(t *testing.T) {
		tc := NewToolCoordinator(&ToolCoordinatorConfig{})
		tc.Register(tools.NewBashTool())
		tc.Register(tools.NewReadTool())
		tc.Register(tools.NewWriteTool())

		stats := tc.Stats()
		assert.Equal(t, 3, stats.ToolCount)
	})
}

func TestToolCoordinator_ComponentAccess(t *testing.T) {
	t.Run("Executor returns configured executor", func(t *testing.T) {
		exec := tools.NewExecutor()
		tc := NewToolCoordinator(&ToolCoordinatorConfig{
			Executor: exec,
		})
		assert.Equal(t, exec, tc.Executor())
	})

	t.Run("Fingerprint returns configured fingerprinter", func(t *testing.T) {
		fp := fingerprint.NewFingerprinter()
		tc := NewToolCoordinator(&ToolCoordinatorConfig{
			Fingerprinter: fp,
		})
		assert.Equal(t, fp, tc.Fingerprint())
	})

	t.Run("TaskManager returns nil when not set", func(t *testing.T) {
		tc := NewToolCoordinator(&ToolCoordinatorConfig{})
		assert.Nil(t, tc.TaskManager())
	})
}

func TestToolCoordinator_SecurityPolicy(t *testing.T) {
	t.Run("sets security policy", func(t *testing.T) {
		tc := NewToolCoordinator(&ToolCoordinatorConfig{})

		policy := &tools.SecurityPolicy{
			AllowedDirs:  []string{"/tmp", "/home"},
			AllowNetwork: true,
		}

		tc.SetSecurityPolicy(policy)

		retrieved := tc.GetSecurityPolicy()
		assert.NotNil(t, retrieved)
	})

	t.Run("handles nil executor when setting policy", func(t *testing.T) {
		tc := NewToolCoordinator(&ToolCoordinatorConfig{})
		tc.executor = nil // Simulate nil executor

		policy := &tools.SecurityPolicy{}
		// Should not panic
		tc.SetSecurityPolicy(policy)
	})

	t.Run("returns nil policy when executor is nil", func(t *testing.T) {
		tc := NewToolCoordinator(&ToolCoordinatorConfig{})
		tc.executor = nil // Simulate nil executor

		policy := tc.GetSecurityPolicy()
		assert.Nil(t, policy)
	})
}

func TestToolCoordinator_ExecutorStats(t *testing.T) {
	t.Run("returns executor stats", func(t *testing.T) {
		tc := NewToolCoordinator(&ToolCoordinatorConfig{})

		stats := tc.ExecutorStats()
		// Should return valid (possibly empty) stats
		assert.NotNil(t, stats)
	})

	t.Run("returns empty stats when executor is nil", func(t *testing.T) {
		tc := NewToolCoordinator(&ToolCoordinatorConfig{})
		tc.executor = nil // Simulate nil executor

		stats := tc.ExecutorStats()
		// Should return empty stats, not panic
		assert.Equal(t, tools.ExecutorStats{}, stats)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// INTERFACE COMPLIANCE TEST
// ═══════════════════════════════════════════════════════════════════════════════

func TestToolCoordinator_ImplementsToolExecutor(t *testing.T) {
	// This test verifies at compile time that ToolCoordinator implements ToolExecutor
	var _ ToolExecutor = (*ToolCoordinator)(nil)
}

// ═══════════════════════════════════════════════════════════════════════════════
// ORCHESTRATOR INTEGRATION TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestOrchestrator_WithToolCoordinator(t *testing.T) {
	t.Run("creates orchestrator with tool coordinator", func(t *testing.T) {
		tc := NewToolCoordinator(&ToolCoordinatorConfig{})

		o := New(WithToolCoordinator(tc))
		require.NotNil(t, o)
		assert.NotNil(t, o.ToolCoordinator())
		assert.Equal(t, tc, o.ToolCoordinator())
	})

	t.Run("orchestrator creates default tool coordinator", func(t *testing.T) {
		o := New()
		require.NotNil(t, o)
		// ToolCoordinator should be created by default
		assert.NotNil(t, o.ToolCoordinator())
	})

	t.Run("WithToolCoordinator takes precedence over legacy options", func(t *testing.T) {
		tc := NewToolCoordinator(&ToolCoordinatorConfig{})

		o := New(
			WithToolExecutor(tools.NewExecutor()), // Legacy
			WithToolCoordinator(tc),               // New takes precedence
		)
		require.NotNil(t, o)
		assert.Equal(t, tc, o.ToolCoordinator())
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// CONCURRENCY TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestToolCoordinator_Concurrency(t *testing.T) {
	t.Run("handles concurrent registrations", func(t *testing.T) {
		tc := NewToolCoordinator(&ToolCoordinatorConfig{})

		done := make(chan bool)

		// Concurrent registrations
		for i := 0; i < 5; i++ {
			go func() {
				tc.Register(tools.NewBashTool())
				tc.Register(tools.NewReadTool())
				done <- true
			}()
		}

		// Concurrent reads
		for i := 0; i < 5; i++ {
			go func() {
				_ = tc.ListTools()
				_ = tc.Stats()
				done <- true
			}()
		}

		// Wait for all
		for i := 0; i < 10; i++ {
			<-done
		}
	})
}
