// Package orchestrator provides the central coordination layer for Cortex.
package orchestrator

import (
	"context"
	"fmt"
	"sync"

	"github.com/normanking/cortex/internal/facets"
	"github.com/normanking/cortex/internal/logging"
	"github.com/normanking/cortex/internal/persona"
)

// ═══════════════════════════════════════════════════════════════════════════════
// PERSONA MANAGER INTERFACE
// CR-017: Phase 3 - Persona Coordinator Extraction
// ═══════════════════════════════════════════════════════════════════════════════

// PersonaManager defines the interface for persona and behavioral mode management.
// It encapsulates persona switching, mode transitions, and prompt generation.
type PersonaManager interface {
	// GetActivePersona returns the currently active persona.
	// Returns nil if no persona is set.
	GetActivePersona() *facets.PersonaCore

	// SetActivePersona sets the active persona by ID.
	// Returns error if persona is not found.
	SetActivePersona(ctx context.Context, personaID string) error

	// GetActiveMode returns the current behavioral mode.
	GetActiveMode() persona.ModeType

	// SetMode sets the active behavioral mode.
	SetMode(mode persona.ModeType, trigger string)

	// ProcessInput analyzes user input for mode transitions.
	// Returns true if a mode transition occurred.
	ProcessInput(input string) bool

	// GetModeHistory returns recent mode transitions.
	GetModeHistory() []persona.ModeTransition

	// BuildSystemPrompt generates a system prompt using the active persona and mode.
	// If ctx is nil, uses default context.
	BuildSystemPrompt(ctx *persona.SessionContext) string

	// GetPersonaCore returns the underlying PersonaCore (for legacy compatibility).
	GetPersonaCore() *persona.PersonaCore

	// Stats returns persona coordinator statistics.
	Stats() *PersonaStats
}

// PersonaStats contains statistics about the persona subsystem.
type PersonaStats struct {
	HasActivePersona bool             `json:"has_active_persona"`
	ActivePersonaID  string           `json:"active_persona_id,omitempty"`
	ActivePersona    string           `json:"active_persona_name,omitempty"`
	CurrentMode      persona.ModeType `json:"current_mode"`
	ModeTransitions  int              `json:"mode_transitions"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// PERSONA COORDINATOR IMPLEMENTATION
// ═══════════════════════════════════════════════════════════════════════════════

// PersonaCoordinator manages persona selection and behavioral modes.
// It encapsulates the facet store, mode manager, and persona core into a single
// coherent subsystem.
type PersonaCoordinator struct {
	// Persona components
	facetStore  *facets.PersonaStore
	modeManager *persona.ModeManager
	personaCore *persona.PersonaCore

	// Active state
	activePersona *facets.PersonaCore
	activeMode    persona.ModeType

	// Configuration
	log *logging.Logger

	// State
	mu sync.RWMutex
}

// PersonaCoordinatorConfig configures the PersonaCoordinator.
type PersonaCoordinatorConfig struct {
	// FacetStore is the persona store for loading personas.
	FacetStore *facets.PersonaStore

	// ModeManager handles behavioral mode transitions.
	ModeManager *persona.ModeManager

	// PersonaCore is the base persona configuration.
	PersonaCore *persona.PersonaCore
}

// NewPersonaCoordinator creates a new persona coordinator.
func NewPersonaCoordinator(cfg *PersonaCoordinatorConfig) *PersonaCoordinator {
	if cfg == nil {
		cfg = &PersonaCoordinatorConfig{}
	}

	pc := &PersonaCoordinator{
		facetStore:  cfg.FacetStore,
		modeManager: cfg.ModeManager,
		personaCore: cfg.PersonaCore,
		activeMode:  persona.ModeNormal,
		log:         logging.Global(),
	}

	// Create default mode manager if not provided
	if pc.modeManager == nil {
		pc.modeManager = persona.NewModeManager()
	}

	// Create default persona core if not provided
	if pc.personaCore == nil {
		pc.personaCore = persona.NewPersonaCore()
	}

	return pc
}

// Verify PersonaCoordinator implements PersonaManager at compile time.
var _ PersonaManager = (*PersonaCoordinator)(nil)

// ═══════════════════════════════════════════════════════════════════════════════
// PERSONA MANAGEMENT
// ═══════════════════════════════════════════════════════════════════════════════

// GetActivePersona returns the currently active facet persona.
func (pc *PersonaCoordinator) GetActivePersona() *facets.PersonaCore {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.activePersona
}

// SetActivePersona sets the active persona by ID from the facet store.
func (pc *PersonaCoordinator) SetActivePersona(ctx context.Context, personaID string) error {
	if pc.facetStore == nil {
		return fmt.Errorf("facet store not configured")
	}

	newPersona, err := pc.facetStore.Get(ctx, personaID)
	if err != nil {
		return fmt.Errorf("get persona: %w", err)
	}

	pc.mu.Lock()
	pc.activePersona = newPersona
	pc.mu.Unlock()

	pc.log.Info("[PersonaCoordinator] Active persona set to: %s (%s)", newPersona.Name, newPersona.ID)
	return nil
}

// GetPersonaCore returns the underlying PersonaCore.
func (pc *PersonaCoordinator) GetPersonaCore() *persona.PersonaCore {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.personaCore
}

// ═══════════════════════════════════════════════════════════════════════════════
// MODE MANAGEMENT
// ═══════════════════════════════════════════════════════════════════════════════

// GetActiveMode returns the current behavioral mode.
func (pc *PersonaCoordinator) GetActiveMode() persona.ModeType {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.activeMode
}

// SetMode sets the active behavioral mode.
func (pc *PersonaCoordinator) SetMode(mode persona.ModeType, trigger string) {
	pc.mu.Lock()
	pc.activeMode = mode
	pc.mu.Unlock()

	// Update mode manager
	if pc.modeManager != nil {
		pc.modeManager.SetMode(mode, trigger)
	}

	pc.log.Info("[PersonaCoordinator] Active mode set to: %s (trigger: %s)", mode, trigger)
}

// ProcessInput analyzes user input for mode transitions.
// Returns true if a mode transition occurred.
func (pc *PersonaCoordinator) ProcessInput(input string) bool {
	if pc.modeManager == nil {
		return false
	}

	transitioned := pc.modeManager.ProcessInput(input)
	if transitioned {
		// Sync the active mode with mode manager
		pc.mu.Lock()
		pc.activeMode = pc.modeManager.Current().Type
		pc.mu.Unlock()
		pc.log.Debug("[PersonaCoordinator] Mode transitioned to: %s", pc.activeMode)
	}

	return transitioned
}

// GetModeHistory returns recent mode transitions.
func (pc *PersonaCoordinator) GetModeHistory() []persona.ModeTransition {
	if pc.modeManager == nil {
		return nil
	}
	return pc.modeManager.History()
}

// ═══════════════════════════════════════════════════════════════════════════════
// PROMPT GENERATION
// ═══════════════════════════════════════════════════════════════════════════════

// BuildSystemPrompt generates a system prompt using the active persona and mode.
func (pc *PersonaCoordinator) BuildSystemPrompt(ctx *persona.SessionContext) string {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	// If we have an active facet persona, use its system prompt
	if pc.activePersona != nil {
		prompt := pc.activePersona.SystemPrompt

		// Add mode-specific augmentation
		if pc.activeMode != persona.ModeNormal && pc.activeMode != "" {
			if mode := pc.activePersona.GetMode(string(pc.activeMode)); mode != nil {
				if mode.PromptAugment != "" {
					prompt += "\n\n" + mode.PromptAugment
				}
			}
		}

		return prompt
	}

	// Fall back to persona core
	if pc.personaCore != nil {
		// Set the mode on persona core before building prompt
		if pc.modeManager != nil {
			pc.personaCore.SetMode(pc.modeManager.Current())
		}
		return pc.personaCore.BuildSystemPrompt(ctx)
	}

	// Ultimate fallback
	return "You are Cortex, an intelligent AI assistant for software development and system administration."
}

// ═══════════════════════════════════════════════════════════════════════════════
// STATISTICS
// ═══════════════════════════════════════════════════════════════════════════════

// Stats returns persona coordinator statistics.
func (pc *PersonaCoordinator) Stats() *PersonaStats {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	stats := &PersonaStats{
		CurrentMode: pc.activeMode,
	}

	if pc.activePersona != nil {
		stats.HasActivePersona = true
		stats.ActivePersonaID = pc.activePersona.ID
		stats.ActivePersona = pc.activePersona.Name
	}

	if pc.modeManager != nil {
		stats.ModeTransitions = len(pc.modeManager.History())
	}

	return stats
}

// ═══════════════════════════════════════════════════════════════════════════════
// COMPONENT ACCESS (for advanced use cases)
// ═══════════════════════════════════════════════════════════════════════════════

// FacetStore returns the underlying facet store (may be nil).
func (pc *PersonaCoordinator) FacetStore() *facets.PersonaStore {
	return pc.facetStore
}

// ModeManager returns the underlying mode manager.
func (pc *PersonaCoordinator) ModeManager() *persona.ModeManager {
	return pc.modeManager
}
