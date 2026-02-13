package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/normanking/cortex/internal/cognitive"
	cogtemplates "github.com/normanking/cortex/internal/cognitive/templates"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ═══════════════════════════════════════════════════════════════════════════════
// COGNITIVE COORDINATOR TESTS
// CR-017: Phase 2 - Comprehensive unit tests for CognitiveCoordinator
// ═══════════════════════════════════════════════════════════════════════════════

func TestNewCognitiveCoordinator(t *testing.T) {
	t.Run("creates with nil config", func(t *testing.T) {
		cc := NewCognitiveCoordinator(nil)
		require.NotNil(t, cc)
		assert.NotNil(t, cc.templateEng, "should create default template engine")
		assert.False(t, cc.enabled, "should be disabled by default")
	})

	t.Run("creates with empty config", func(t *testing.T) {
		cc := NewCognitiveCoordinator(&CognitiveConfig{})
		require.NotNil(t, cc)
		assert.NotNil(t, cc.templateEng)
		assert.False(t, cc.enabled)
	})

	t.Run("creates with enabled config", func(t *testing.T) {
		cc := NewCognitiveCoordinator(&CognitiveConfig{
			Enabled: true,
		})
		require.NotNil(t, cc)
		assert.True(t, cc.enabled)
	})

	t.Run("creates with custom template engine", func(t *testing.T) {
		customEngine := cogtemplates.NewEngine()
		cc := NewCognitiveCoordinator(&CognitiveConfig{
			TemplateEngine: customEngine,
		})
		require.NotNil(t, cc)
		assert.Equal(t, customEngine, cc.templateEng)
	})
}

func TestCognitiveCoordinator_Enabled(t *testing.T) {
	t.Run("returns false when disabled", func(t *testing.T) {
		cc := NewCognitiveCoordinator(&CognitiveConfig{
			Enabled: false,
		})
		assert.False(t, cc.Enabled())
	})

	t.Run("returns true when enabled", func(t *testing.T) {
		cc := NewCognitiveCoordinator(&CognitiveConfig{
			Enabled: true,
		})
		assert.True(t, cc.Enabled())
	})

	t.Run("SetEnabled works", func(t *testing.T) {
		cc := NewCognitiveCoordinator(&CognitiveConfig{
			Enabled: false,
		})
		assert.False(t, cc.Enabled())

		cc.SetEnabled(true)
		assert.True(t, cc.Enabled())

		cc.SetEnabled(false)
		assert.False(t, cc.Enabled())
	})
}

func TestCognitiveCoordinator_Route(t *testing.T) {
	ctx := context.Background()

	t.Run("returns novel decision when disabled", func(t *testing.T) {
		cc := NewCognitiveCoordinator(&CognitiveConfig{
			Enabled: false,
		})

		result, err := cc.Route(ctx, "test input")
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, cognitive.RouteNovel, result.Decision)
		assert.Equal(t, cognitive.TierFrontier, result.RecommendedTier)
	})

	t.Run("returns novel decision when router is nil", func(t *testing.T) {
		cc := NewCognitiveCoordinator(&CognitiveConfig{
			Enabled: true,
			Router:  nil,
		})

		result, err := cc.Route(ctx, "test input")
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, cognitive.RouteNovel, result.Decision)
	})
}

func TestCognitiveCoordinator_RenderTemplateSimple(t *testing.T) {
	t.Run("renders simple template", func(t *testing.T) {
		cc := NewCognitiveCoordinator(&CognitiveConfig{})

		output, err := cc.RenderTemplateSimple("Hello, {{.name}}!", map[string]any{
			"name": "World",
		})
		require.NoError(t, err)
		assert.Equal(t, "Hello, World!", output)
	})

	t.Run("renders template with multiple variables", func(t *testing.T) {
		cc := NewCognitiveCoordinator(&CognitiveConfig{})

		output, err := cc.RenderTemplateSimple("{{.greeting}}, {{.name}}! Count: {{.count}}", map[string]any{
			"greeting": "Hi",
			"name":     "Alice",
			"count":    42,
		})
		require.NoError(t, err)
		assert.Equal(t, "Hi, Alice! Count: 42", output)
	})

	t.Run("returns error for invalid template", func(t *testing.T) {
		cc := NewCognitiveCoordinator(&CognitiveConfig{})

		_, err := cc.RenderTemplateSimple("{{.unclosed", map[string]any{})
		require.Error(t, err)
	})

	t.Run("returns error when template engine is nil", func(t *testing.T) {
		cc := &CognitiveCoordinator{
			templateEng: nil,
		}

		_, err := cc.RenderTemplateSimple("{{.name}}", map[string]any{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "template engine not configured")
	})
}

func TestCognitiveCoordinator_RenderTemplate(t *testing.T) {
	ctx := context.Background()

	t.Run("returns error when registry is nil", func(t *testing.T) {
		cc := NewCognitiveCoordinator(&CognitiveConfig{})

		_, err := cc.RenderTemplate(ctx, "template-123", map[string]any{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "registry not configured")
	})

	t.Run("returns error when template engine is nil", func(t *testing.T) {
		cc := &CognitiveCoordinator{
			templateEng: nil,
		}

		_, err := cc.RenderTemplate(ctx, "template-123", map[string]any{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "template engine not configured")
	})
}

func TestCognitiveCoordinator_Distill(t *testing.T) {
	ctx := context.Background()

	t.Run("returns empty result when disabled", func(t *testing.T) {
		cc := NewCognitiveCoordinator(&CognitiveConfig{
			Enabled: false,
		})

		result, err := cc.Distill(ctx, "test input", cognitive.TaskGeneral)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Empty(t, result.Solution)
	})

	t.Run("returns empty result when distiller is nil", func(t *testing.T) {
		cc := NewCognitiveCoordinator(&CognitiveConfig{
			Enabled:   true,
			Distiller: nil,
		})

		result, err := cc.Distill(ctx, "test input", cognitive.TaskCodeGen)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Empty(t, result.Solution)
	})
}

func TestCognitiveCoordinator_RecordFeedback(t *testing.T) {
	ctx := context.Background()

	t.Run("silently skips when disabled", func(t *testing.T) {
		cc := NewCognitiveCoordinator(&CognitiveConfig{
			Enabled: false,
		})

		err := cc.RecordFeedback(ctx, "template-123", "input", "output", true, 100)
		require.NoError(t, err)
	})

	t.Run("silently skips when feedback is nil", func(t *testing.T) {
		cc := NewCognitiveCoordinator(&CognitiveConfig{
			Enabled:  true,
			Feedback: nil,
		})

		err := cc.RecordFeedback(ctx, "template-123", "input", "output", true, 100)
		require.NoError(t, err)
	})
}

func TestCognitiveCoordinator_Analyze(t *testing.T) {
	t.Run("returns simple result when disabled", func(t *testing.T) {
		cc := NewCognitiveCoordinator(&CognitiveConfig{
			Enabled: false,
		})

		result := cc.Analyze("test input", cognitive.TaskGeneral)
		require.NotNil(t, result)
		assert.Equal(t, "test input", result.OriginalInput)
		require.NotNil(t, result.Complexity)
		assert.Equal(t, 0, result.Complexity.Score)
		assert.False(t, result.Complexity.NeedsDecomp)
	})

	t.Run("returns simple result when decomposer is nil", func(t *testing.T) {
		cc := NewCognitiveCoordinator(&CognitiveConfig{
			Enabled:    true,
			Decomposer: nil,
		})

		result := cc.Analyze("test input", cognitive.TaskDebug)
		require.NotNil(t, result)
		assert.Equal(t, "test input", result.OriginalInput)
		assert.False(t, result.Complexity.NeedsDecomp)
	})
}

func TestCognitiveCoordinator_Decompose(t *testing.T) {
	ctx := context.Background()

	t.Run("returns simple result when disabled", func(t *testing.T) {
		cc := NewCognitiveCoordinator(&CognitiveConfig{
			Enabled: false,
		})

		result, err := cc.Decompose(ctx, "test input", cognitive.TaskGeneral)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.Complexity.NeedsDecomp)
	})

	t.Run("returns simple result when decomposer is nil", func(t *testing.T) {
		cc := NewCognitiveCoordinator(&CognitiveConfig{
			Enabled:    true,
			Decomposer: nil,
		})

		result, err := cc.Decompose(ctx, "complex multi-step task", cognitive.TaskPlanning)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.Complexity.NeedsDecomp)
	})
}

func TestCognitiveCoordinator_Stats(t *testing.T) {
	t.Run("returns stats when disabled", func(t *testing.T) {
		cc := NewCognitiveCoordinator(&CognitiveConfig{
			Enabled: false,
		})

		stats := cc.Stats()
		require.NotNil(t, stats)
		assert.False(t, stats.Enabled)
		assert.False(t, stats.RouterAvailable)
		assert.Equal(t, 0, stats.TemplatesIndexed)
	})

	t.Run("returns stats when enabled without router", func(t *testing.T) {
		cc := NewCognitiveCoordinator(&CognitiveConfig{
			Enabled: true,
		})

		stats := cc.Stats()
		require.NotNil(t, stats)
		assert.True(t, stats.Enabled)
		assert.False(t, stats.RouterAvailable)
	})
}

func TestCognitiveCoordinator_Initialize(t *testing.T) {
	ctx := context.Background()

	t.Run("initializes without error when components are nil", func(t *testing.T) {
		cc := NewCognitiveCoordinator(&CognitiveConfig{
			Enabled: true,
		})

		err := cc.Initialize(ctx)
		require.NoError(t, err)
	})

	t.Run("is idempotent", func(t *testing.T) {
		cc := NewCognitiveCoordinator(&CognitiveConfig{
			Enabled: true,
		})

		err := cc.Initialize(ctx)
		require.NoError(t, err)

		// Second call should be no-op
		err = cc.Initialize(ctx)
		require.NoError(t, err)
	})
}

func TestCognitiveCoordinator_Shutdown(t *testing.T) {
	ctx := context.Background()

	t.Run("shuts down gracefully", func(t *testing.T) {
		cc := NewCognitiveCoordinator(&CognitiveConfig{
			Enabled: true,
		})

		err := cc.Initialize(ctx)
		require.NoError(t, err)

		// Should not panic
		cc.Shutdown()
	})

	t.Run("is idempotent", func(t *testing.T) {
		cc := NewCognitiveCoordinator(&CognitiveConfig{
			Enabled: true,
		})

		err := cc.Initialize(ctx)
		require.NoError(t, err)

		cc.Shutdown()
		// Second call should be no-op
		cc.Shutdown()
	})

	t.Run("handles shutdown before initialize", func(t *testing.T) {
		cc := NewCognitiveCoordinator(&CognitiveConfig{
			Enabled: true,
		})

		// Should not panic
		cc.Shutdown()
	})
}

func TestCognitiveCoordinator_ComponentAccess(t *testing.T) {
	customEngine := cogtemplates.NewEngine()

	cc := NewCognitiveCoordinator(&CognitiveConfig{
		TemplateEngine: customEngine,
	})

	t.Run("Router returns nil when not set", func(t *testing.T) {
		assert.Nil(t, cc.Router())
	})

	t.Run("TemplateEngine returns the engine", func(t *testing.T) {
		assert.Equal(t, customEngine, cc.TemplateEngine())
	})

	t.Run("Registry returns nil when not set", func(t *testing.T) {
		assert.Nil(t, cc.Registry())
	})

	t.Run("PromptManager returns nil when not set", func(t *testing.T) {
		assert.Nil(t, cc.PromptManager())
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// INTEGRATION TESTS WITH ORCHESTRATOR
// ═══════════════════════════════════════════════════════════════════════════════

func TestOrchestrator_WithCognitiveCoordinator(t *testing.T) {
	t.Run("creates orchestrator with cognitive coordinator", func(t *testing.T) {
		cc := NewCognitiveCoordinator(&CognitiveConfig{
			Enabled: true,
		})

		o := New(WithCognitiveCoordinator(cc))
		require.NotNil(t, o)
		assert.NotNil(t, o.cognitive)
		assert.True(t, o.cogEnabled)
	})

	t.Run("legacy options still work", func(t *testing.T) {
		o := New(EnableCognitive())
		require.NotNil(t, o)
		assert.True(t, o.cogEnabled)
		// CognitiveCoordinator should be created from legacy options
		assert.NotNil(t, o.cognitive)
	})

	t.Run("WithCognitiveCoordinator takes precedence", func(t *testing.T) {
		cc := NewCognitiveCoordinator(&CognitiveConfig{
			Enabled: true,
		})

		// Both legacy and new options set
		o := New(
			EnableCognitive(),
			WithCognitiveRouter(nil),     // Legacy
			WithCognitiveCoordinator(cc), // New takes precedence
		)
		require.NotNil(t, o)
		assert.Equal(t, cc, o.cognitive)
	})
}

func TestCognitiveCoordinator_ConcurrencySafety(t *testing.T) {
	cc := NewCognitiveCoordinator(&CognitiveConfig{
		Enabled: true,
	})

	ctx := context.Background()

	// Initialize
	err := cc.Initialize(ctx)
	require.NoError(t, err)

	// Run concurrent operations
	done := make(chan bool)

	// Concurrent enabled checks
	go func() {
		for i := 0; i < 100; i++ {
			cc.Enabled()
		}
		done <- true
	}()

	// Concurrent stats checks
	go func() {
		for i := 0; i < 100; i++ {
			cc.Stats()
		}
		done <- true
	}()

	// Concurrent set enabled
	go func() {
		for i := 0; i < 100; i++ {
			cc.SetEnabled(i%2 == 0)
		}
		done <- true
	}()

	// Concurrent route calls
	go func() {
		for i := 0; i < 100; i++ {
			cc.Route(ctx, "test input")
		}
		done <- true
	}()

	// Wait for all goroutines
	timeout := time.After(5 * time.Second)
	for i := 0; i < 4; i++ {
		select {
		case <-done:
			// OK
		case <-timeout:
			t.Fatal("timeout waiting for concurrent operations")
		}
	}

	// Should still be functional
	stats := cc.Stats()
	require.NotNil(t, stats)
}

// ═══════════════════════════════════════════════════════════════════════════════
// INTERFACE COMPLIANCE
// ═══════════════════════════════════════════════════════════════════════════════

func TestCognitiveArchitecture_InterfaceCompliance(t *testing.T) {
	// Verify at compile time that CognitiveCoordinator implements CognitiveArchitecture
	var _ CognitiveArchitecture = (*CognitiveCoordinator)(nil)

	t.Run("interface methods are callable", func(t *testing.T) {
		var iface CognitiveArchitecture = NewCognitiveCoordinator(&CognitiveConfig{
			Enabled: true,
		})

		ctx := context.Background()

		// All interface methods should be callable without panic
		_, _ = iface.Route(ctx, "test")
		_, _ = iface.RenderTemplateSimple("{{.x}}", map[string]any{"x": 1})
		_, _ = iface.RenderTemplate(ctx, "id", nil)
		_, _ = iface.Distill(ctx, "input", cognitive.TaskGeneral)
		_ = iface.RecordFeedback(ctx, "id", "in", "out", true, 0)
		_ = iface.Analyze("input", cognitive.TaskDebug)
		_, _ = iface.Decompose(ctx, "input", cognitive.TaskPlanning)
		_ = iface.Enabled()
		_ = iface.Stats()
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// COMPLEXITY RESULT TESTS (from decomposer)
// ═══════════════════════════════════════════════════════════════════════════════

func TestComplexityResult_Integration(t *testing.T) {
	t.Run("analyze returns correct structure", func(t *testing.T) {
		cc := NewCognitiveCoordinator(&CognitiveConfig{
			Enabled:    true,
			Decomposer: nil, // Will return default result
		})

		result := cc.Analyze("simple question", cognitive.TaskGeneral)
		require.NotNil(t, result)
		require.NotNil(t, result.Complexity)

		// Verify ComplexityResult fields
		assert.IsType(t, int(0), result.Complexity.Score)
		assert.IsType(t, false, result.Complexity.NeedsDecomp)
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// BENCHMARK TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func BenchmarkCognitiveCoordinator_Route_Disabled(b *testing.B) {
	cc := NewCognitiveCoordinator(&CognitiveConfig{
		Enabled: false,
	})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cc.Route(ctx, "test input")
	}
}

func BenchmarkCognitiveCoordinator_RenderTemplateSimple(b *testing.B) {
	cc := NewCognitiveCoordinator(&CognitiveConfig{})
	vars := map[string]any{"name": "World", "count": 42}
	template := "Hello, {{.name}}! Count: {{.count}}"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cc.RenderTemplateSimple(template, vars)
	}
}

func BenchmarkCognitiveCoordinator_Analyze_Disabled(b *testing.B) {
	cc := NewCognitiveCoordinator(&CognitiveConfig{
		Enabled: false,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cc.Analyze("test input", cognitive.TaskGeneral)
	}
}

func BenchmarkCognitiveCoordinator_Stats(b *testing.B) {
	cc := NewCognitiveCoordinator(&CognitiveConfig{
		Enabled: true,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cc.Stats()
	}
}
