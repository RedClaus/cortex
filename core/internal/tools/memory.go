package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/normanking/cortex/internal/memory"
)

// MemoryToolType constants for memory operations.
const (
	ToolRecallSearch       ToolType = "recall_memory_search"
	ToolCoreMemoryRead     ToolType = "core_memory_read"
	ToolCoreMemoryAppend   ToolType = "core_memory_append"
	ToolArchivalSearch     ToolType = "archival_memory_search"
	ToolArchivalInsert     ToolType = "archival_memory_insert"
)

// BaseMemoryTool provides common functionality for memory tools.
type BaseMemoryTool struct {
	toolType     ToolType
	memoryTools  *memory.MemoryTools
	userID       string // Current user ID for memory operations
}

// NewBaseMemoryTool creates a base memory tool wrapper.
func NewBaseMemoryTool(toolType ToolType, memoryTools *memory.MemoryTools, userID string) *BaseMemoryTool {
	return &BaseMemoryTool{
		toolType:    toolType,
		memoryTools: memoryTools,
		userID:      userID,
	}
}

// Name returns the tool identifier.
func (t *BaseMemoryTool) Name() ToolType {
	return t.toolType
}

// Validate checks if the request is valid.
func (t *BaseMemoryTool) Validate(req *ToolRequest) error {
	if req.Tool != t.toolType {
		return fmt.Errorf("wrong tool type: expected %s, got %s", t.toolType, req.Tool)
	}
	return nil
}

// AssessRisk evaluates the risk level.
// Memory operations are generally low risk (reads are none, writes are low).
func (t *BaseMemoryTool) AssessRisk(req *ToolRequest) RiskLevel {
	switch t.toolType {
	case ToolRecallSearch, ToolCoreMemoryRead, ToolArchivalSearch:
		return RiskNone // Read operations are safe
	case ToolCoreMemoryAppend, ToolArchivalInsert:
		return RiskLow // Writes to memory are low risk
	default:
		return RiskNone
	}
}

// Execute runs the memory tool.
func (t *BaseMemoryTool) Execute(ctx context.Context, req *ToolRequest) (*ToolResult, error) {
	// Build args JSON from params
	argsJSON, err := json.Marshal(req.Params)
	if err != nil {
		return &ToolResult{
			Tool:      t.toolType,
			Success:   false,
			Error:     fmt.Sprintf("failed to marshal params: %v", err),
			RiskLevel: RiskNone,
		}, err
	}

	// Execute the memory tool
	result, err := t.memoryTools.ExecuteTool(ctx, t.userID, string(t.toolType), string(argsJSON))
	if err != nil {
		return &ToolResult{
			Tool:      t.toolType,
			Success:   false,
			Error:     fmt.Sprintf("memory tool execution failed: %v", err),
			RiskLevel: t.AssessRisk(req),
		}, err
	}

	// Convert memory.ToolResult to tools.ToolResult
	toolResult := &ToolResult{
		Tool:      t.toolType,
		Success:   result.Success,
		Output:    result.Result,
		Error:     result.Error,
		RiskLevel: t.AssessRisk(req),
		Metadata: map[string]interface{}{
			"latency_ms": result.LatencyMs,
		},
	}

	return toolResult, nil
}

// RecallSearchTool searches conversation history.
type RecallSearchTool struct {
	*BaseMemoryTool
}

// NewRecallSearchTool creates a recall search tool.
func NewRecallSearchTool(memoryTools *memory.MemoryTools, userID string) *RecallSearchTool {
	return &RecallSearchTool{
		BaseMemoryTool: NewBaseMemoryTool(ToolRecallSearch, memoryTools, userID),
	}
}

// CoreMemoryReadTool reads persistent user/project facts.
type CoreMemoryReadTool struct {
	*BaseMemoryTool
}

// NewCoreMemoryReadTool creates a core memory read tool.
func NewCoreMemoryReadTool(memoryTools *memory.MemoryTools, userID string) *CoreMemoryReadTool {
	return &CoreMemoryReadTool{
		BaseMemoryTool: NewBaseMemoryTool(ToolCoreMemoryRead, memoryTools, userID),
	}
}

// CoreMemoryAppendTool remembers new facts about the user.
type CoreMemoryAppendTool struct {
	*BaseMemoryTool
}

// NewCoreMemoryAppendTool creates a core memory append tool.
func NewCoreMemoryAppendTool(memoryTools *memory.MemoryTools, userID string) *CoreMemoryAppendTool {
	return &CoreMemoryAppendTool{
		BaseMemoryTool: NewBaseMemoryTool(ToolCoreMemoryAppend, memoryTools, userID),
	}
}

// ArchivalSearchTool searches the knowledge base.
type ArchivalSearchTool struct {
	*BaseMemoryTool
}

// NewArchivalSearchTool creates an archival search tool.
func NewArchivalSearchTool(memoryTools *memory.MemoryTools, userID string) *ArchivalSearchTool {
	return &ArchivalSearchTool{
		BaseMemoryTool: NewBaseMemoryTool(ToolArchivalSearch, memoryTools, userID),
	}
}

// ArchivalInsertTool stores lessons in the knowledge base.
type ArchivalInsertTool struct {
	*BaseMemoryTool
}

// NewArchivalInsertTool creates an archival insert tool.
func NewArchivalInsertTool(memoryTools *memory.MemoryTools, userID string) *ArchivalInsertTool {
	return &ArchivalInsertTool{
		BaseMemoryTool: NewBaseMemoryTool(ToolArchivalInsert, memoryTools, userID),
	}
}
