// Package orchestrator provides the central coordination layer for Cortex.
// This is a minimal implementation providing only MemoryCoordinator for cortex-server.
package orchestrator

import (
	"context"
	"sync"

	"github.com/normanking/cortex/internal/knowledge"
	"github.com/normanking/cortex/internal/logging"
	"github.com/normanking/cortex/internal/memory"
	"github.com/normanking/cortex/pkg/types"
)

// MemorySystem defines the interface for memory operations.
type MemorySystem interface {
	GetUserMemory(ctx context.Context, userID string) (*memory.UserMemory, error)
	UpdateUserMemory(ctx context.Context, userID string, field string, value any) error
	GetProjectMemory(ctx context.Context, projectID string) (*memory.ProjectMemory, error)
	SearchArchival(ctx context.Context, query string, limit int) ([]*types.KnowledgeItem, error)
	InsertArchival(ctx context.Context, item *types.KnowledgeItem) error
	ExecuteTool(ctx context.Context, userID, toolName, argsJSON string) (*memory.ToolResult, error)
	GetToolDefinitions() []memory.Tool
	CoreStore() *memory.CoreMemoryStore
	MemoryTools() *memory.MemoryTools
	Stats() *MemoryStats
}

// MemoryStats contains statistics about the memory subsystem.
type MemoryStats struct {
	HasCoreStore   bool `json:"has_core_store"`
	HasMemoryTools bool `json:"has_memory_tools"`
	HasKnowledge   bool `json:"has_knowledge"`
}

// MemoryCoordinator manages all memory-related operations.
type MemoryCoordinator struct {
	coreStore   *memory.CoreMemoryStore
	memoryTools *memory.MemoryTools
	fabric      knowledge.KnowledgeFabric
	config      *MemoryCoordinatorConfig
	log         *logging.Logger
	mu          sync.RWMutex
}

// MemoryCoordinatorConfig configures the MemoryCoordinator.
type MemoryCoordinatorConfig struct {
	CoreStore   *memory.CoreMemoryStore
	MemoryTools *memory.MemoryTools
	Fabric      knowledge.KnowledgeFabric
}

// NewMemoryCoordinator creates a new memory coordinator.
func NewMemoryCoordinator(cfg *MemoryCoordinatorConfig) *MemoryCoordinator {
	if cfg == nil {
		cfg = &MemoryCoordinatorConfig{}
	}
	return &MemoryCoordinator{
		coreStore:   cfg.CoreStore,
		memoryTools: cfg.MemoryTools,
		fabric:      cfg.Fabric,
		config:      cfg,
		log:         logging.Global(),
	}
}

// Verify MemoryCoordinator implements MemorySystem at compile time.
var _ MemorySystem = (*MemoryCoordinator)(nil)

// GetUserMemory returns the user memory for a given user ID.
func (mc *MemoryCoordinator) GetUserMemory(ctx context.Context, userID string) (*memory.UserMemory, error) {
	if mc.coreStore == nil {
		return nil, nil
	}
	return mc.coreStore.GetUserMemory(ctx, userID)
}

// UpdateUserMemory updates a specific field in user memory.
func (mc *MemoryCoordinator) UpdateUserMemory(ctx context.Context, userID string, field string, value any) error {
	if mc.coreStore == nil {
		return nil
	}
	return mc.coreStore.UpdateUserField(ctx, userID, field, value, "orchestrator")
}

// GetProjectMemory returns the project memory for a given project ID.
func (mc *MemoryCoordinator) GetProjectMemory(ctx context.Context, projectID string) (*memory.ProjectMemory, error) {
	if mc.coreStore == nil {
		return nil, nil
	}
	return mc.coreStore.GetProjectMemory(ctx, projectID)
}

// SearchArchival searches the archival/knowledge store.
func (mc *MemoryCoordinator) SearchArchival(ctx context.Context, query string, limit int) ([]*types.KnowledgeItem, error) {
	if mc.fabric == nil {
		return nil, nil
	}
	opts := types.SearchOptions{
		Limit: limit,
	}
	result, err := mc.fabric.Search(ctx, query, opts)
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// InsertArchival stores new knowledge in archival memory.
func (mc *MemoryCoordinator) InsertArchival(ctx context.Context, item *types.KnowledgeItem) error {
	if mc.fabric == nil {
		return nil
	}
	return mc.fabric.Create(ctx, item)
}

// ExecuteTool executes a memory tool by name.
func (mc *MemoryCoordinator) ExecuteTool(ctx context.Context, userID, toolName, argsJSON string) (*memory.ToolResult, error) {
	if mc.memoryTools == nil {
		return nil, nil
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

// CoreStore returns the underlying core memory store.
func (mc *MemoryCoordinator) CoreStore() *memory.CoreMemoryStore {
	return mc.coreStore
}

// MemoryTools returns the underlying memory tools.
func (mc *MemoryCoordinator) MemoryTools() *memory.MemoryTools {
	return mc.memoryTools
}

// Stats returns memory coordinator statistics.
func (mc *MemoryCoordinator) Stats() *MemoryStats {
	return &MemoryStats{
		HasCoreStore:   mc.coreStore != nil,
		HasMemoryTools: mc.memoryTools != nil,
		HasKnowledge:   mc.fabric != nil,
	}
}
