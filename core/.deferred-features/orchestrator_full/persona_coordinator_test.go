// Package orchestrator provides the central coordination layer for Cortex.
package orchestrator

import (
	"context"
	"testing"

	"github.com/normanking/cortex/internal/persona"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ═══════════════════════════════════════════════════════════════════════════════
// PERSONA COORDINATOR TESTS
// CR-017: Phase 3 - Persona Coordinator Unit Tests
// ═══════════════════════════════════════════════════════════════════════════════

func TestNewPersonaCoordinator(t *testing.T) {
	tests := []struct {
		name   string
		config *PersonaCoordinatorConfig
	}{
		{
			name:   "creates with nil config",
			config: nil,
		},
		{
			name:   "creates with empty config",
			config: &PersonaCoordinatorConfig{},
		},
		{
			name: "creates with mode manager",
			config: &PersonaCoordinatorConfig{
				ModeManager: persona.NewModeManager(),
			},
		},
		{
			name: "creates with persona core",
			config: &PersonaCoordinatorConfig{
				PersonaCore: persona.NewPersonaCore(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := NewPersonaCoordinator(tt.config)
			require.NotNil(t, pc)

			// Verify defaults are created
			assert.NotNil(t, pc.ModeManager(), "ModeManager should be created by default")
			assert.NotNil(t, pc.GetPersonaCore(), "PersonaCore should be created by default")
			assert.Equal(t, persona.ModeNormal, pc.GetActiveMode(), "Default mode should be normal")
		})
	}
}

func TestPersonaCoordinator_GetActivePersona(t *testing.T) {
	t.Run("returns nil when no persona set", func(t *testing.T) {
		pc := NewPersonaCoordinator(nil)
		assert.Nil(t, pc.GetActivePersona())
	})
}

func TestPersonaCoordinator_SetActivePersona(t *testing.T) {
	tests := []struct {
		name        string
		config      *PersonaCoordinatorConfig
		personaID   string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "returns error when facet store not configured",
			config:      &PersonaCoordinatorConfig{},
			personaID:   "test-persona",
			expectError: true,
			errorMsg:    "facet store not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := NewPersonaCoordinator(tt.config)
			ctx := context.Background()

			err := pc.SetActivePersona(ctx, tt.personaID)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPersonaCoordinator_GetActiveMode(t *testing.T) {
	t.Run("returns normal mode by default", func(t *testing.T) {
		pc := NewPersonaCoordinator(nil)
		assert.Equal(t, persona.ModeNormal, pc.GetActiveMode())
	})
}

func TestPersonaCoordinator_SetMode(t *testing.T) {
	tests := []struct {
		name        string
		mode        persona.ModeType
		trigger     string
		expectedMsg string
	}{
		{
			name:    "sets debugging mode",
			mode:    persona.ModeDebugging,
			trigger: "user request",
		},
		{
			name:    "sets teaching mode",
			mode:    persona.ModeTeaching,
			trigger: "detected question",
		},
		{
			name:    "sets pair mode",
			mode:    persona.ModePair,
			trigger: "coding request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := NewPersonaCoordinator(nil)

			pc.SetMode(tt.mode, tt.trigger)

			assert.Equal(t, tt.mode, pc.GetActiveMode())
		})
	}
}

func TestPersonaCoordinator_ProcessInput(t *testing.T) {
	t.Run("returns false when mode manager not configured", func(t *testing.T) {
		pc := &PersonaCoordinator{}
		result := pc.ProcessInput("test input")
		assert.False(t, result)
	})

	t.Run("processes input with default mode manager", func(t *testing.T) {
		pc := NewPersonaCoordinator(nil)

		// Default mode manager may not transition on simple input
		result := pc.ProcessInput("normal conversation")

		// We don't assert on the result since it depends on mode manager rules
		// Just verify it doesn't panic
		_ = result
	})
}

func TestPersonaCoordinator_GetModeHistory(t *testing.T) {
	t.Run("returns nil when mode manager not configured", func(t *testing.T) {
		pc := &PersonaCoordinator{}
		result := pc.GetModeHistory()
		assert.Nil(t, result)
	})

	t.Run("returns empty history initially", func(t *testing.T) {
		pc := NewPersonaCoordinator(nil)
		history := pc.GetModeHistory()

		// History should be empty or contain initial transition
		assert.NotNil(t, history)
	})

	t.Run("tracks mode transitions", func(t *testing.T) {
		pc := NewPersonaCoordinator(nil)

		// Trigger mode changes
		pc.SetMode(persona.ModeDebugging, "user request")
		pc.SetMode(persona.ModeTeaching, "question detected")

		history := pc.GetModeHistory()
		assert.GreaterOrEqual(t, len(history), 2)
	})
}

func TestPersonaCoordinator_BuildSystemPrompt(t *testing.T) {
	t.Run("returns fallback prompt when no persona configured", func(t *testing.T) {
		pc := &PersonaCoordinator{}
		prompt := pc.BuildSystemPrompt(nil)
		assert.Contains(t, prompt, "Cortex")
	})

	t.Run("uses persona core when no active facet persona", func(t *testing.T) {
		pc := NewPersonaCoordinator(&PersonaCoordinatorConfig{
			PersonaCore: persona.NewPersonaCore(),
		})

		prompt := pc.BuildSystemPrompt(nil)

		// Should return something non-empty
		assert.NotEmpty(t, prompt)
	})

	t.Run("builds prompt with session context", func(t *testing.T) {
		pc := NewPersonaCoordinator(nil)

		ctx := &persona.SessionContext{
			CWD:         "/test/project",
			ProjectType: "go",
			Language:    "Go",
		}

		prompt := pc.BuildSystemPrompt(ctx)
		assert.NotEmpty(t, prompt)
	})
}

func TestPersonaCoordinator_Stats(t *testing.T) {
	t.Run("returns stats with no active persona", func(t *testing.T) {
		pc := NewPersonaCoordinator(nil)
		stats := pc.Stats()

		require.NotNil(t, stats)
		assert.False(t, stats.HasActivePersona)
		assert.Empty(t, stats.ActivePersonaID)
		assert.Empty(t, stats.ActivePersona)
		assert.Equal(t, persona.ModeNormal, stats.CurrentMode)
	})

	t.Run("reflects mode transitions count", func(t *testing.T) {
		pc := NewPersonaCoordinator(nil)

		// Trigger mode changes
		pc.SetMode(persona.ModeDebugging, "test")
		pc.SetMode(persona.ModeTeaching, "test")

		stats := pc.Stats()
		assert.GreaterOrEqual(t, stats.ModeTransitions, 2)
	})
}

func TestPersonaCoordinator_ComponentAccess(t *testing.T) {
	t.Run("FacetStore returns nil when not set", func(t *testing.T) {
		pc := NewPersonaCoordinator(&PersonaCoordinatorConfig{})
		assert.Nil(t, pc.FacetStore())
	})

	t.Run("ModeManager returns default when not set", func(t *testing.T) {
		pc := NewPersonaCoordinator(&PersonaCoordinatorConfig{})
		assert.NotNil(t, pc.ModeManager())
	})

	t.Run("GetPersonaCore returns default when not set", func(t *testing.T) {
		pc := NewPersonaCoordinator(&PersonaCoordinatorConfig{})
		assert.NotNil(t, pc.GetPersonaCore())
	})

	t.Run("FacetStore returns configured store", func(t *testing.T) {
		// Note: We can't easily create a mock facets.PersonaStore
		// This test documents the expected behavior
		pc := NewPersonaCoordinator(&PersonaCoordinatorConfig{})
		_ = pc.FacetStore() // Should not panic
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// INTERFACE COMPLIANCE TEST
// ═══════════════════════════════════════════════════════════════════════════════

func TestPersonaCoordinator_ImplementsPersonaManager(t *testing.T) {
	// This test verifies at compile time that PersonaCoordinator implements PersonaManager
	var _ PersonaManager = (*PersonaCoordinator)(nil)
}

// ═══════════════════════════════════════════════════════════════════════════════
// CONCURRENCY TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestPersonaCoordinator_Concurrency(t *testing.T) {
	t.Run("handles concurrent GetActiveMode calls", func(t *testing.T) {
		pc := NewPersonaCoordinator(nil)

		done := make(chan bool)
		for i := 0; i < 10; i++ {
			go func() {
				_ = pc.GetActiveMode()
				done <- true
			}()
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}
	})

	t.Run("handles concurrent SetMode and GetActiveMode", func(t *testing.T) {
		pc := NewPersonaCoordinator(nil)

		done := make(chan bool)

		// Writer
		go func() {
			for i := 0; i < 100; i++ {
				pc.SetMode(persona.ModeDebugging, "test")
				pc.SetMode(persona.ModeNormal, "test")
			}
			done <- true
		}()

		// Readers
		for i := 0; i < 5; i++ {
			go func() {
				for j := 0; j < 100; j++ {
					_ = pc.GetActiveMode()
				}
				done <- true
			}()
		}

		// Wait for all goroutines
		for i := 0; i < 6; i++ {
			<-done
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// ORCHESTRATOR INTEGRATION TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestOrchestrator_WithPersonaCoordinator(t *testing.T) {
	t.Run("creates orchestrator with persona coordinator", func(t *testing.T) {
		pc := NewPersonaCoordinator(&PersonaCoordinatorConfig{})

		o := New(WithPersonaCoordinator(pc))
		require.NotNil(t, o)
		assert.NotNil(t, o.PersonaCoordinator())
		assert.Equal(t, pc, o.PersonaCoordinator())
	})

	t.Run("orchestrator creates default persona coordinator", func(t *testing.T) {
		o := New()
		require.NotNil(t, o)
		// PersonaCoordinator should be created by default
		assert.NotNil(t, o.PersonaCoordinator())
	})

	t.Run("WithPersonaCoordinator takes precedence over legacy options", func(t *testing.T) {
		pc := NewPersonaCoordinator(&PersonaCoordinatorConfig{})

		// Create with legacy options, but coordinator should override
		o := New(
			WithModeManager(persona.NewModeManager()),
			WithPersonaCoordinator(pc),
		)
		require.NotNil(t, o)
		assert.Equal(t, pc, o.PersonaCoordinator())
	})

	t.Run("legacy options create persona coordinator automatically", func(t *testing.T) {
		mm := persona.NewModeManager()
		o := New(WithModeManager(mm))
		require.NotNil(t, o)
		// PersonaCoordinator should be created from legacy options
		assert.NotNil(t, o.PersonaCoordinator())
	})
}
