// Package block provides the hierarchical block data model for Cortex TUI.
// Blocks are discrete, addressable conversation units that enable per-block
// actions, nested tool execution display, and conversation branching.
package block

// ═══════════════════════════════════════════════════════════════════════════════
// BLOCK TYPE ENUM
// ═══════════════════════════════════════════════════════════════════════════════

// BlockType represents the semantic type of a conversation block.
// Each type has distinct rendering and behavior characteristics.
type BlockType int

const (
	// BlockTypeUser represents a user input block (prompts, questions)
	BlockTypeUser BlockType = iota

	// BlockTypeAssistant represents a top-level assistant response container
	// This is the parent block that contains Text, Code, Tool, and Thinking children
	BlockTypeAssistant

	// BlockTypeText represents a markdown text segment within an assistant response
	BlockTypeText

	// BlockTypeCode represents a code block with syntax highlighting
	BlockTypeCode

	// BlockTypeTool represents a tool execution with input/output
	BlockTypeTool

	// BlockTypeThinking represents AI reasoning/thinking content (collapsible)
	BlockTypeThinking

	// BlockTypeError represents an error message block
	BlockTypeError

	// BlockTypeSystem represents a system notification block
	BlockTypeSystem
)

// String returns the string representation of the block type.
func (t BlockType) String() string {
	switch t {
	case BlockTypeUser:
		return "User"
	case BlockTypeAssistant:
		return "Assistant"
	case BlockTypeText:
		return "Text"
	case BlockTypeCode:
		return "Code"
	case BlockTypeTool:
		return "Tool"
	case BlockTypeThinking:
		return "Thinking"
	case BlockTypeError:
		return "Error"
	case BlockTypeSystem:
		return "System"
	default:
		return "Unknown"
	}
}

// Icon returns an emoji icon for the block type.
func (t BlockType) Icon() string {
	switch t {
	case BlockTypeUser:
		return "👤"
	case BlockTypeAssistant:
		return "🤖"
	case BlockTypeText:
		return "📝"
	case BlockTypeCode:
		return "💻"
	case BlockTypeTool:
		return "🔧"
	case BlockTypeThinking:
		return "💭"
	case BlockTypeError:
		return "❌"
	case BlockTypeSystem:
		return "ℹ️"
	default:
		return "❓"
	}
}

// IsCollapsible returns whether blocks of this type support collapse/expand.
func (t BlockType) IsCollapsible() bool {
	switch t {
	case BlockTypeTool, BlockTypeThinking, BlockTypeCode:
		return true
	default:
		return false
	}
}

// CanHaveChildren returns whether blocks of this type can contain nested blocks.
func (t BlockType) CanHaveChildren() bool {
	switch t {
	case BlockTypeAssistant, BlockTypeTool:
		return true
	default:
		return false
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// BLOCK STATE ENUM
// ═══════════════════════════════════════════════════════════════════════════════

// BlockState represents the current lifecycle state of a block.
type BlockState int

const (
	// BlockStatePending indicates the block is waiting to receive content
	BlockStatePending BlockState = iota

	// BlockStateStreaming indicates the block is actively receiving streamed content
	BlockStateStreaming

	// BlockStateComplete indicates the block has finished receiving content
	BlockStateComplete

	// BlockStateCollapsed indicates the block is collapsed (content hidden)
	BlockStateCollapsed

	// BlockStateError indicates the block encountered an error
	BlockStateError
)

// String returns the string representation of the block state.
func (s BlockState) String() string {
	switch s {
	case BlockStatePending:
		return "Pending"
	case BlockStateStreaming:
		return "Streaming"
	case BlockStateComplete:
		return "Complete"
	case BlockStateCollapsed:
		return "Collapsed"
	case BlockStateError:
		return "Error"
	default:
		return "Unknown"
	}
}

// IsActive returns whether the block is actively changing (pending or streaming).
func (s BlockState) IsActive() bool {
	return s == BlockStatePending || s == BlockStateStreaming
}

// IsFinished returns whether the block has finished processing.
func (s BlockState) IsFinished() bool {
	return s == BlockStateComplete || s == BlockStateError
}

// StatusIndicator returns a status character for display.
func (s BlockState) StatusIndicator() string {
	switch s {
	case BlockStatePending:
		return "○"
	case BlockStateStreaming:
		return "◐"
	case BlockStateComplete:
		return "●"
	case BlockStateCollapsed:
		return "▸"
	case BlockStateError:
		return "✗"
	default:
		return "?"
	}
}
