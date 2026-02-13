// Package orchestrator provides the central coordination layer for Cortex.
package orchestrator

import (
	"context"
	"fmt"
	"sync"

	"github.com/normanking/cortex/internal/bus"
	"github.com/normanking/cortex/internal/knowledge"
	"github.com/normanking/cortex/internal/logging"
	"github.com/normanking/cortex/internal/memory"
	"github.com/normanking/cortex/pkg/types"
)

// ═══════════════════════════════════════════════════════════════════════════════
// MEMORY SYSTEM INTERFACE
// CR-017: Phase 3 - Memory Coordinator Extraction
// ═══════════════════════════════════════════════════════════════════════════════

// MemorySystem defines the interface for memory operations.
// It encapsulates core memory, archival memory, and memory tools into a
// single coherent subsystem.
type MemorySystem interface {
	// GetUserMemory returns the user memory for a given user ID.
	GetUserMemory(ctx context.Context, userID string) (*memory.UserMemory, error)

	// UpdateUserMemory updates a specific field in user memory.
	UpdateUserMemory(ctx context.Context, userID string, field string, value any) error

	// GetProjectMemory returns the project memory for a given project ID.
	GetProjectMemory(ctx context.Context, projectID string) (*memory.ProjectMemory, error)

	// SearchArchival searches the archival/knowledge store.
	SearchArchival(ctx context.Context, query string, limit int) ([]*types.KnowledgeItem, error)

	// InsertArchival stores new knowledge in archival memory.
	InsertArchival(ctx context.Context, item *types.KnowledgeItem) error

	// ExecuteTool executes a memory tool by name.
	ExecuteTool(ctx context.Context, userID, toolName, argsJSON string) (*memory.ToolResult, error)

	// GetToolDefinitions returns available memory tool schemas.
	GetToolDefinitions() []memory.Tool

	// CoreStore returns the underlying core memory store (for legacy compatibility).
	CoreStore() *memory.CoreMemoryStore

	// MemoryTools returns the underlying memory tools (for legacy compatibility).
	MemoryTools() *memory.MemoryTools

	// Stats returns memory coordinator statistics.
	Stats() *MemoryStats
}

// MemoryStats contains statistics about the memory subsystem.
type MemoryStats struct {
	HasCoreStore   bool `json:"has_core_store"`
	HasMemoryTools bool `json:"has_memory_tools"`
	HasKnowledge   bool `json:"has_knowledge"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// MEMORY COORDINATOR IMPLEMENTATION
// ═══════════════════════════════════════════════════════════════════════════════

// MemoryCoordinator manages all memory-related operations.
// It encapsulates core memory store, memory tools, and knowledge fabric.
type MemoryCoordinator struct {
	// Memory components
	coreStore   *memory.CoreMemoryStore
	memoryTools *memory.MemoryTools
	fabric      knowledge.KnowledgeFabric

	// CR-017 Phase 5: Event bus for publishing memory events
	eventBus *bus.EventBus

	// Configuration
	config *MemoryCoordinatorConfig
	log    *logging.Logger

	// State
	mu sync.RWMutex
}

// MemoryCoordinatorConfig configures the MemoryCoordinator.
type MemoryCoordinatorConfig struct {
	// CoreStore is the core memory store for user/project memory.
	CoreStore *memory.CoreMemoryStore

	// MemoryTools provides LLM-callable memory tools.
	MemoryTools *memory.MemoryTools

	// Fabric is the knowledge fabric for archival storage.
	Fabric knowledge.KnowledgeFabric

	// EventBus for publishing memory events (CR-017 Phase 5).
	EventBus *bus.EventBus
}

// NewMemoryCoordinator creates a new memory coordinator.
func NewMemoryCoordinator(cfg *MemoryCoordinatorConfig) *MemoryCoordinator {
	if cfg == nil {
		cfg = &MemoryCoordinatorConfig{}
	}

	mc := &MemoryCoordinator{
		coreStore:   cfg.CoreStore,
		memoryTools: cfg.MemoryTools,
		fabric:      cfg.Fabric,
		eventBus:    cfg.EventBus,
		config:      cfg,
		log:         logging.Global(),
	}

	return mc
}

// Verify MemoryCoordinator implements MemorySystem at compile time.
var _ MemorySystem = (*MemoryCoordinator)(nil)

// ═══════════════════════════════════════════════════════════════════════════════
// USER MEMORY
// ═══════════════════════════════════════════════════════════════════════════════

// GetUserMemory returns the user memory for a given user ID.
func (mc *MemoryCoordinator) GetUserMemory(ctx context.Context, userID string) (*memory.UserMemory, error) {
	if mc.coreStore == nil {
		return nil, fmt.Errorf("core store not configured")
	}
	return mc.coreStore.GetUserMemory(ctx, userID)
}

// UpdateUserMemory updates a specific field in user memory.
func (mc *MemoryCoordinator) UpdateUserMemory(ctx context.Context, userID string, field string, value any) error {
	if mc.coreStore == nil {
		return fmt.Errorf("core store not configured")
	}

	var err error
	// Map field to update method
	switch field {
	case "name":
		err = mc.coreStore.UpdateUserField(ctx, userID, "name", value, "coordinator")
	case "role":
		err = mc.coreStore.UpdateUserField(ctx, userID, "role", value, "coordinator")
	case "experience":
		err = mc.coreStore.UpdateUserField(ctx, userID, "experience", value, "coordinator")
	case "os":
		err = mc.coreStore.UpdateUserField(ctx, userID, "os", value, "coordinator")
	case "shell":
		err = mc.coreStore.UpdateUserField(ctx, userID, "shell", value, "coordinator")
	case "editor":
		err = mc.coreStore.UpdateUserField(ctx, userID, "editor", value, "coordinator")
	default:
		return fmt.Errorf("unknown field: %s", field)
	}

	// CR-017 Phase 5: Publish MemoryUpdated event on success
	if err == nil && mc.eventBus != nil {
		mc.eventBus.Publish(bus.NewMemoryUpdatedEvent(userID, field, "coordinator"))
	}

	return err
}

// ═══════════════════════════════════════════════════════════════════════════════
// PROJECT MEMORY
// ═══════════════════════════════════════════════════════════════════════════════

// GetProjectMemory returns the project memory for a given project ID.
func (mc *MemoryCoordinator) GetProjectMemory(ctx context.Context, projectID string) (*memory.ProjectMemory, error) {
	if mc.coreStore == nil {
		return nil, fmt.Errorf("core store not configured")
	}
	return mc.coreStore.GetProjectMemory(ctx, projectID)
}

// ═══════════════════════════════════════════════════════════════════════════════
// ARCHIVAL MEMORY (Knowledge)
// ═══════════════════════════════════════════════════════════════════════════════

// SearchArchival searches the archival/knowledge store.
func (mc *MemoryCoordinator) SearchArchival(ctx context.Context, query string, limit int) ([]*types.KnowledgeItem, error) {
	if mc.fabric == nil {
		return nil, fmt.Errorf("knowledge fabric not configured")
	}

	result, err := mc.fabric.Search(ctx, query, types.SearchOptions{
		Limit: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("search archival: %w", err)
	}

	if result == nil {
		return nil, nil
	}

	return result.Items, nil
}

// InsertArchival stores new knowledge in archival memory.
func (mc *MemoryCoordinator) InsertArchival(ctx context.Context, item *types.KnowledgeItem) error {
	if mc.fabric == nil {
		return fmt.Errorf("knowledge fabric not configured")
	}

	return mc.fabric.Create(ctx, item)
}

// ═══════════════════════════════════════════════════════════════════════════════
// MEMORY TOOLS
// ═══════════════════════════════════════════════════════════════════════════════

// ExecuteTool executes a memory tool by name.
func (mc *MemoryCoordinator) ExecuteTool(ctx context.Context, userID, toolName, argsJSON string) (*memory.ToolResult, error) {
	if mc.memoryTools == nil {
		return nil, fmt.Errorf("memory tools not configured")
	}

	return mc.memoryTools.ExecuteTool(ctx, userID, toolName, argsJSON)
}

// GetToolDefinitions returns available memory tool schemas.
func (mc *MemoryCoordinator) GetToolDefinitions() []memory.Tool {
	if mc.memoryTools == nil {
		return nil
	}

	return mc.memoryTools.GetToolDefinitions()
}

// ═══════════════════════════════════════════════════════════════════════════════
// COMPONENT ACCESS (for legacy compatibility)
// ═══════════════════════════════════════════════════════════════════════════════

// CoreStore returns the underlying core memory store.
func (mc *MemoryCoordinator) CoreStore() *memory.CoreMemoryStore {
	return mc.coreStore
}

// MemoryTools returns the underlying memory tools.
func (mc *MemoryCoordinator) MemoryTools() *memory.MemoryTools {
	return mc.memoryTools
}

// KnowledgeFabric returns the underlying knowledge fabric.
func (mc *MemoryCoordinator) KnowledgeFabric() knowledge.KnowledgeFabric {
	return mc.fabric
}

// SetEventBus sets the event bus for publishing memory events (CR-017 Phase 5).
func (mc *MemoryCoordinator) SetEventBus(eb *bus.EventBus) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.eventBus = eb
}

// ═══════════════════════════════════════════════════════════════════════════════
// STATISTICS
// ═══════════════════════════════════════════════════════════════════════════════

// Stats returns memory coordinator statistics.
func (mc *MemoryCoordinator) Stats() *MemoryStats {
	return &MemoryStats{
		HasCoreStore:   mc.coreStore != nil,
		HasMemoryTools: mc.memoryTools != nil,
		HasKnowledge:   mc.fabric != nil,
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// CONTEXT BUILDING HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

// BuildMemoryContext builds memory context for system prompt injection.
// Returns a formatted string with relevant user and project memory.
func (mc *MemoryCoordinator) BuildMemoryContext(ctx context.Context, userID, projectID string) (string, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if mc.coreStore == nil {
		return "", nil
	}

	var context string

	// Get user memory
	if userID != "" {
		userMem, err := mc.coreStore.GetUserMemory(ctx, userID)
		if err == nil && userMem != nil {
			context += fmt.Sprintf("## User Context\n")
			if userMem.Name != "" {
				context += fmt.Sprintf("- Name: %s\n", userMem.Name)
			}
			if userMem.Role != "" {
				context += fmt.Sprintf("- Role: %s\n", userMem.Role)
			}
			if userMem.OS != "" {
				context += fmt.Sprintf("- OS: %s\n", userMem.OS)
			}
			if userMem.Shell != "" {
				context += fmt.Sprintf("- Shell: %s\n", userMem.Shell)
			}
			if userMem.Editor != "" {
				context += fmt.Sprintf("- Editor: %s\n", userMem.Editor)
			}
			if len(userMem.CustomFacts) > 0 {
				context += "\n### Known Facts About User\n"
				for _, f := range userMem.CustomFacts {
					context += fmt.Sprintf("- %s\n", f.Fact)
				}
			}
			context += "\n"
		}
	}

	// Get project memory
	if projectID != "" {
		projMem, err := mc.coreStore.GetProjectMemory(ctx, projectID)
		if err == nil && projMem != nil {
			context += fmt.Sprintf("## Project Context\n")
			if projMem.Name != "" {
				context += fmt.Sprintf("- Name: %s\n", projMem.Name)
			}
			if projMem.Type != "" {
				context += fmt.Sprintf("- Type: %s\n", projMem.Type)
			}
			if projMem.Path != "" {
				context += fmt.Sprintf("- Path: %s\n", projMem.Path)
			}
			context += "\n"
		}
	}

	return context, nil
}
