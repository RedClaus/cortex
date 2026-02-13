// Package ui provides Bubble Tea message types for the block system.
package ui

import (
	"github.com/normanking/cortex/internal/ui/block"
)

// ═══════════════════════════════════════════════════════════════════════════════
// BLOCK CREATION MESSAGES
// ═══════════════════════════════════════════════════════════════════════════════

// BlockCreatedMsg is sent when a new block is created.
// This triggers adding the block to the container and updating the view.
type BlockCreatedMsg struct {
	// Block is the newly created block
	Block *block.Block

	// ParentID is the parent block ID (empty for root blocks)
	ParentID string
}

// BlockUpdatedMsg is sent when a block's content changes.
// This is used during streaming to update the block content.
type BlockUpdatedMsg struct {
	// BlockID is the ID of the block being updated
	BlockID string

	// Content is the content to append (for streaming) or set (for replace)
	Content string

	// Append indicates whether to append or replace content
	Append bool
}

// BlockStateChangedMsg is sent when a block's state changes.
type BlockStateChangedMsg struct {
	// BlockID is the ID of the block
	BlockID string

	// NewState is the new state of the block
	NewState block.BlockState

	// Error is set if the state is Error
	Error error
}

// ═══════════════════════════════════════════════════════════════════════════════
// TOOL BLOCK MESSAGES
// ═══════════════════════════════════════════════════════════════════════════════

// ToolBlockStartedMsg is sent when a tool execution begins.
type ToolBlockStartedMsg struct {
	// BlockID is the ID of the tool block
	BlockID string

	// ParentID is the parent block (usually an AssistantBlock)
	ParentID string

	// ToolName is the name of the tool being executed
	ToolName string

	// ToolInput is the input/arguments for the tool
	ToolInput string
}

// ToolBlockCompletedMsg is sent when a tool execution completes.
type ToolBlockCompletedMsg struct {
	// BlockID is the ID of the tool block
	BlockID string

	// Output is the tool's output
	Output string

	// Success indicates whether the tool succeeded
	Success bool

	// DurationMs is how long the tool took in milliseconds
	DurationMs int64

	// Error is set if the tool failed
	Error error
}

// ═══════════════════════════════════════════════════════════════════════════════
// THINKING BLOCK MESSAGES
// ═══════════════════════════════════════════════════════════════════════════════

// ThinkingBlockStartedMsg is sent when AI thinking/reasoning begins.
type ThinkingBlockStartedMsg struct {
	// BlockID is the ID of the thinking block
	BlockID string

	// ParentID is the parent AssistantBlock
	ParentID string
}

// ThinkingBlockUpdatedMsg is sent when thinking content updates.
type ThinkingBlockUpdatedMsg struct {
	// BlockID is the ID of the thinking block
	BlockID string

	// Content is the thinking content
	Content string
}

// ═══════════════════════════════════════════════════════════════════════════════
// CODE BLOCK MESSAGES
// ═══════════════════════════════════════════════════════════════════════════════

// CodeBlockCreatedMsg is sent when a code block is detected.
type CodeBlockCreatedMsg struct {
	// BlockID is the ID of the code block
	BlockID string

	// ParentID is the parent block
	ParentID string

	// Language is the programming language
	Language string

	// Content is the code content
	Content string
}

// ═══════════════════════════════════════════════════════════════════════════════
// NAVIGATION MESSAGES
// ═══════════════════════════════════════════════════════════════════════════════

// BlockFocusMsg requests focus change to a specific block.
type BlockFocusMsg struct {
	// BlockID is the ID of the block to focus
	BlockID string
}

// BlockToggleMsg requests toggling a block's collapsed state.
type BlockToggleMsg struct {
	// BlockID is the ID of the block to toggle
	BlockID string
}

// BlockCopyMsg requests copying a block's content to clipboard.
type BlockCopyMsg struct {
	// BlockID is the ID of the block to copy
	BlockID string
}

// BlockBookmarkMsg requests toggling a block's bookmark status.
type BlockBookmarkMsg struct {
	// BlockID is the ID of the block
	BlockID string
}

// BlockRegenerateMsg requests regenerating from a specific block.
type BlockRegenerateMsg struct {
	// BlockID is the block to regenerate from
	BlockID string
}

// ═══════════════════════════════════════════════════════════════════════════════
// STREAMING EXTENSION
// ═══════════════════════════════════════════════════════════════════════════════

// StreamChunkWithBlock extends streaming with block information.
// This is used when the block system is enabled.
type StreamChunkWithBlock struct {
	// BlockID is the ID of the block receiving the chunk
	BlockID string

	// BlockType indicates what type of block this belongs to
	BlockType block.BlockType

	// Content is the chunk content
	Content string

	// IsFirst indicates if this is the first chunk for the block
	IsFirst bool

	// IsFinal indicates if this is the final chunk
	IsFinal bool

	// Metadata holds additional chunk information
	Metadata map[string]interface{}
}

// ═══════════════════════════════════════════════════════════════════════════════
// BRANCH MESSAGES
// ═══════════════════════════════════════════════════════════════════════════════

// BranchCreatedMsg is sent when a new conversation branch is created.
type BranchCreatedMsg struct {
	// BranchID is the ID of the new branch
	BranchID string

	// FromBlockID is the block where the branch diverges
	FromBlockID string
}

// BranchSwitchedMsg is sent when the active branch changes.
type BranchSwitchedMsg struct {
	// BranchID is the new active branch ID
	BranchID string
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPER FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════════

// NewBlockCreatedMsg creates a BlockCreatedMsg for a new block.
func NewBlockCreatedMsg(b *block.Block) BlockCreatedMsg {
	return BlockCreatedMsg{
		Block:    b,
		ParentID: b.ParentID,
	}
}

// NewBlockUpdatedMsg creates a BlockUpdatedMsg for appending content.
func NewBlockUpdatedMsg(blockID, content string) BlockUpdatedMsg {
	return BlockUpdatedMsg{
		BlockID: blockID,
		Content: content,
		Append:  true,
	}
}

// NewBlockStateChangedMsg creates a BlockStateChangedMsg.
func NewBlockStateChangedMsg(blockID string, newState block.BlockState) BlockStateChangedMsg {
	return BlockStateChangedMsg{
		BlockID:  blockID,
		NewState: newState,
	}
}

// NewToolBlockStartedMsg creates a ToolBlockStartedMsg.
func NewToolBlockStartedMsg(blockID, parentID, toolName, toolInput string) ToolBlockStartedMsg {
	return ToolBlockStartedMsg{
		BlockID:   blockID,
		ParentID:  parentID,
		ToolName:  toolName,
		ToolInput: toolInput,
	}
}

// NewToolBlockCompletedMsg creates a ToolBlockCompletedMsg.
func NewToolBlockCompletedMsg(blockID, output string, success bool, durationMs int64) ToolBlockCompletedMsg {
	return ToolBlockCompletedMsg{
		BlockID:    blockID,
		Output:     output,
		Success:    success,
		DurationMs: durationMs,
	}
}
