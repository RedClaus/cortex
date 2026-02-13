package block

import (
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// MESSAGE TO BLOCK CONVERSION
// ═══════════════════════════════════════════════════════════════════════════════

// MessageRole represents the role of a message (for conversion).
type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleSystem    MessageRole = "system"
)

// MessageState represents the state of a message (for conversion).
type MessageState int

const (
	MessagePending MessageState = iota
	MessageStreaming
	MessageComplete
	MessageError
)

// MessageData holds message fields needed for conversion.
// This is a simplified interface to avoid circular imports with ui.Message.
type MessageData struct {
	Role       MessageRole
	RawContent string
	State      MessageState
	Timestamp  time.Time
	Metadata   map[string]interface{}
	Error      error
}

// MessageToBlock converts a message to a block.
// This is used for backward compatibility with the existing Message system.
func MessageToBlock(msg MessageData) *Block {
	var blockType BlockType
	var state BlockState

	// Map role to block type
	switch msg.Role {
	case RoleUser:
		blockType = BlockTypeUser
	case RoleAssistant:
		blockType = BlockTypeAssistant
	case RoleSystem:
		blockType = BlockTypeSystem
	default:
		blockType = BlockTypeText
	}

	// Map message state to block state
	switch msg.State {
	case MessagePending:
		state = BlockStatePending
	case MessageStreaming:
		state = BlockStateStreaming
	case MessageComplete:
		state = BlockStateComplete
	case MessageError:
		state = BlockStateError
	default:
		state = BlockStateComplete
	}

	b := &Block{
		ID:          GenerateID(),
		Type:        blockType,
		State:       state,
		Content:     msg.RawContent,
		Metadata:    msg.Metadata,
		Timestamp:   msg.Timestamp,
		Children:    make([]*Block, 0),
		NeedsRender: true,
	}

	if msg.Error != nil {
		b.Error = msg.Error
		b.State = BlockStateError
	}

	if state == BlockStateComplete {
		b.CompletedAt = time.Now()
	}

	return b
}

// ═══════════════════════════════════════════════════════════════════════════════
// BLOCK TO MESSAGE CONVERSION
// ═══════════════════════════════════════════════════════════════════════════════

// BlockToMessage converts a block back to message data.
// Useful for exporting or interfacing with systems expecting Message format.
func BlockToMessage(b *Block) MessageData {
	var role MessageRole
	var state MessageState

	// Map block type to role
	switch b.Type {
	case BlockTypeUser:
		role = RoleUser
	case BlockTypeAssistant, BlockTypeText, BlockTypeCode, BlockTypeTool, BlockTypeThinking:
		role = RoleAssistant
	case BlockTypeSystem, BlockTypeError:
		role = RoleSystem
	default:
		role = RoleAssistant
	}

	// Map block state to message state
	switch b.State {
	case BlockStatePending:
		state = MessagePending
	case BlockStateStreaming:
		state = MessageStreaming
	case BlockStateComplete, BlockStateCollapsed:
		state = MessageComplete
	case BlockStateError:
		state = MessageError
	default:
		state = MessageComplete
	}

	return MessageData{
		Role:       role,
		RawContent: b.Content,
		State:      state,
		Timestamp:  b.Timestamp,
		Metadata:   b.Metadata,
		Error:      b.Error,
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// BATCH CONVERSION
// ═══════════════════════════════════════════════════════════════════════════════

// MessagesToBlocks converts a slice of messages to blocks.
func MessagesToBlocks(messages []MessageData) []*Block {
	blocks := make([]*Block, len(messages))
	for i, msg := range messages {
		blocks[i] = MessageToBlock(msg)
	}
	return blocks
}

// BlocksToMessages converts a slice of blocks to message data.
func BlocksToMessages(blocks []*Block) []MessageData {
	messages := make([]MessageData, len(blocks))
	for i, b := range blocks {
		messages[i] = BlockToMessage(b)
	}
	return messages
}

// ═══════════════════════════════════════════════════════════════════════════════
// ASSISTANT BLOCK DECOMPOSITION
// ═══════════════════════════════════════════════════════════════════════════════

// DecomposeAssistantContent analyzes content and creates appropriate child blocks.
// This is a placeholder for the content parser that will be implemented
// to detect code blocks, tool invocations, and thinking sections.
func DecomposeAssistantContent(parentBlock *Block, content string) []*Block {
	// For now, just create a single text block
	// TODO: Implement content parsing for:
	// - ```code``` blocks -> BlockTypeCode
	// - Tool invocations -> BlockTypeTool
	// - Thinking markers -> BlockTypeThinking

	textBlock := NewTextBlock(content)
	textBlock.ParentID = parentBlock.ID
	textBlock.State = parentBlock.State

	return []*Block{textBlock}
}

// ═══════════════════════════════════════════════════════════════════════════════
// FLATTENING HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

// FlattenBlockContent returns all text content from a block and its children.
// Useful for copying or exporting.
func FlattenBlockContent(b *Block) string {
	content := b.Content

	for _, child := range b.Children {
		childContent := FlattenBlockContent(child)
		if childContent != "" {
			if content != "" {
				content += "\n\n"
			}
			content += childContent
		}
	}

	// For tool blocks, include tool output
	if b.Type == BlockTypeTool && b.ToolOutput != "" {
		if content != "" {
			content += "\n\n"
		}
		content += b.ToolOutput
	}

	return content
}

// GetVisibleContent returns content respecting collapsed state.
// Returns placeholder text for collapsed blocks.
func GetVisibleContent(b *Block) string {
	if b.Collapsed {
		switch b.Type {
		case BlockTypeTool:
			return "[Tool: " + b.ToolName + " - collapsed]"
		case BlockTypeThinking:
			return "[Thinking - collapsed]"
		case BlockTypeCode:
			return "[Code block - collapsed]"
		default:
			return "[Collapsed]"
		}
	}
	return b.Content
}
