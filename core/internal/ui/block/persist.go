// Package block provides persistence for block-based conversations.
package block

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// PERSISTENCE DATA STRUCTURES
// ═══════════════════════════════════════════════════════════════════════════════

// ConversationData represents a complete conversation for serialization.
type ConversationData struct {
	// Version is the schema version for future migrations
	Version int `json:"version"`

	// ID is the unique conversation identifier
	ID string `json:"id"`

	// Title is a human-readable title (auto-generated or user-set)
	Title string `json:"title"`

	// CreatedAt is when the conversation started
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the conversation was last modified
	UpdatedAt time.Time `json:"updated_at"`

	// Model is the AI model used for this conversation
	Model string `json:"model,omitempty"`

	// Provider is the AI provider
	Provider string `json:"provider,omitempty"`

	// Blocks contains the block hierarchy (schema v2+)
	Blocks []*BlockData `json:"blocks,omitempty"`

	// LegacyMessages contains flat messages (schema v1, for migration)
	LegacyMessages []*LegacyMessage `json:"messages,omitempty"`

	// Branches contains conversation branches
	Branches []*BranchData `json:"branches,omitempty"`

	// ActiveBranchID is the currently active branch
	ActiveBranchID string `json:"active_branch_id,omitempty"`

	// Metadata stores additional conversation-level data
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// BlockData represents a block for serialization (subset of Block).
type BlockData struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	State       string                 `json:"state"`
	ParentID    string                 `json:"parent_id,omitempty"`
	Children    []*BlockData           `json:"children,omitempty"`
	Content     string                 `json:"content"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	CompletedAt time.Time              `json:"completed_at,omitempty"`
	ToolName    string                 `json:"tool_name,omitempty"`
	ToolInput   string                 `json:"tool_input,omitempty"`
	ToolOutput  string                 `json:"tool_output,omitempty"`
	ToolSuccess bool                   `json:"tool_success,omitempty"`
	Collapsed   bool                   `json:"collapsed,omitempty"`
	Bookmarked  bool                   `json:"bookmarked,omitempty"`
	Language    string                 `json:"language,omitempty"`
}

// LegacyMessage represents the old flat message format (v1).
type LegacyMessage struct {
	Role      string                 `json:"role"`
	Content   string                 `json:"content"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// BranchData represents a conversation branch for serialization.
type BranchData struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	ParentID   string    `json:"parent_id,omitempty"`
	FromBlock  string    `json:"from_block"`
	CreatedAt  time.Time `json:"created_at"`
	IsActive   bool      `json:"is_active"`
	BlockOrder []string  `json:"block_order"`
}

// Current schema version
const currentSchemaVersion = 2

// ═══════════════════════════════════════════════════════════════════════════════
// SERIALIZATION
// ═══════════════════════════════════════════════════════════════════════════════

// BlockToData converts a Block to serializable BlockData.
func BlockToData(b *Block) *BlockData {
	if b == nil {
		return nil
	}

	data := &BlockData{
		ID:          b.ID,
		Type:        b.Type.String(),
		State:       b.State.String(),
		ParentID:    b.ParentID,
		Content:     b.Content,
		Metadata:    b.Metadata,
		Timestamp:   b.Timestamp,
		CompletedAt: b.CompletedAt,
		ToolName:    b.ToolName,
		ToolInput:   b.ToolInput,
		ToolOutput:  b.ToolOutput,
		ToolSuccess: b.ToolSuccess,
		Collapsed:   b.Collapsed,
		Bookmarked:  b.Bookmarked,
	}

	// Handle language from metadata
	if lang, ok := b.Metadata["language"].(string); ok {
		data.Language = lang
	}

	// Recursively convert children
	if len(b.Children) > 0 {
		data.Children = make([]*BlockData, len(b.Children))
		for i, child := range b.Children {
			data.Children[i] = BlockToData(child)
		}
	}

	return data
}

// DataToBlock converts BlockData back to a Block.
func DataToBlock(data *BlockData) *Block {
	if data == nil {
		return nil
	}

	b := &Block{
		ID:          data.ID,
		Type:        ParseBlockType(data.Type),
		State:       ParseBlockState(data.State),
		ParentID:    data.ParentID,
		Content:     data.Content,
		Metadata:    data.Metadata,
		Timestamp:   data.Timestamp,
		CompletedAt: data.CompletedAt,
		ToolName:    data.ToolName,
		ToolInput:   data.ToolInput,
		ToolOutput:  data.ToolOutput,
		ToolSuccess: data.ToolSuccess,
		Collapsed:   data.Collapsed,
		Bookmarked:  data.Bookmarked,
		NeedsRender: true, // Trigger initial render
	}

	if b.Metadata == nil {
		b.Metadata = make(map[string]interface{})
	}

	// Restore language to metadata if present
	if data.Language != "" {
		b.Metadata["language"] = data.Language
	}

	// Recursively convert children
	if len(data.Children) > 0 {
		b.Children = make([]*Block, len(data.Children))
		for i, childData := range data.Children {
			child := DataToBlock(childData)
			child.ParentID = b.ID
			b.Children[i] = child
		}
	}

	return b
}

// ParseBlockType converts a string to BlockType.
func ParseBlockType(s string) BlockType {
	switch s {
	case "user":
		return BlockTypeUser
	case "assistant":
		return BlockTypeAssistant
	case "text":
		return BlockTypeText
	case "code":
		return BlockTypeCode
	case "tool":
		return BlockTypeTool
	case "thinking":
		return BlockTypeThinking
	case "error":
		return BlockTypeError
	case "system":
		return BlockTypeSystem
	default:
		return BlockTypeText
	}
}

// ParseBlockState converts a string to BlockState.
func ParseBlockState(s string) BlockState {
	switch s {
	case "pending":
		return BlockStatePending
	case "streaming":
		return BlockStateStreaming
	case "complete":
		return BlockStateComplete
	case "collapsed":
		return BlockStateCollapsed
	case "error":
		return BlockStateError
	default:
		return BlockStateComplete
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// SAVE/LOAD OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════════

// ConversationPersister handles saving and loading conversations.
type ConversationPersister struct {
	// dataDir is the base directory for conversation storage
	dataDir string
}

// NewConversationPersister creates a new persister with the given data directory.
func NewConversationPersister(dataDir string) *ConversationPersister {
	return &ConversationPersister{
		dataDir: dataDir,
	}
}

// Save persists a conversation to disk.
func (p *ConversationPersister) Save(conv *ConversationData) error {
	if conv == nil {
		return fmt.Errorf("cannot save nil conversation")
	}

	// Ensure directory exists
	if err := os.MkdirAll(p.dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Update metadata
	conv.Version = currentSchemaVersion
	conv.UpdatedAt = time.Now()

	// Serialize to JSON
	data, err := json.MarshalIndent(conv, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize conversation: %w", err)
	}

	// Write to file
	filename := filepath.Join(p.dataDir, conv.ID+".json")
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write conversation file: %w", err)
	}

	return nil
}

// Load reads a conversation from disk.
func (p *ConversationPersister) Load(conversationID string) (*ConversationData, error) {
	filename := filepath.Join(p.dataDir, conversationID+".json")

	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("conversation not found: %s", conversationID)
		}
		return nil, fmt.Errorf("failed to read conversation file: %w", err)
	}

	var conv ConversationData
	if err := json.Unmarshal(data, &conv); err != nil {
		return nil, fmt.Errorf("failed to parse conversation: %w", err)
	}

	// Migrate if needed
	if conv.Version < currentSchemaVersion {
		if err := p.migrate(&conv); err != nil {
			return nil, fmt.Errorf("failed to migrate conversation: %w", err)
		}
	}

	return &conv, nil
}

// List returns all conversation IDs in the data directory.
func (p *ConversationPersister) List() ([]string, error) {
	entries, err := os.ReadDir(p.dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read data directory: %w", err)
	}

	var ids []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			id := strings.TrimSuffix(entry.Name(), ".json")
			ids = append(ids, id)
		}
	}

	return ids, nil
}

// Delete removes a conversation from disk.
func (p *ConversationPersister) Delete(conversationID string) error {
	filename := filepath.Join(p.dataDir, conversationID+".json")
	if err := os.Remove(filename); err != nil {
		if os.IsNotExist(err) {
			return nil // Already deleted
		}
		return fmt.Errorf("failed to delete conversation: %w", err)
	}
	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// MIGRATION
// ═══════════════════════════════════════════════════════════════════════════════

// migrate upgrades a conversation to the current schema version.
func (p *ConversationPersister) migrate(conv *ConversationData) error {
	// Version 1 -> 2: Convert legacy messages to blocks
	if conv.Version == 1 || conv.Version == 0 {
		if len(conv.LegacyMessages) > 0 && len(conv.Blocks) == 0 {
			conv.Blocks = migrateMessagesToBlocks(conv.LegacyMessages)
			conv.LegacyMessages = nil // Clear after migration
		}
		conv.Version = 2
	}

	return nil
}

// migrateMessagesToBlocks converts legacy flat messages to block hierarchy.
func migrateMessagesToBlocks(messages []*LegacyMessage) []*BlockData {
	var blocks []*BlockData

	for _, msg := range messages {
		var blockType string
		switch msg.Role {
		case "user":
			blockType = "user"
		case "assistant":
			blockType = "assistant"
		case "system":
			blockType = "system"
		default:
			blockType = "text"
		}

		block := &BlockData{
			ID:        generateMigrationID(),
			Type:      blockType,
			State:     "complete",
			Content:   msg.Content,
			Metadata:  msg.Metadata,
			Timestamp: msg.Timestamp,
		}

		blocks = append(blocks, block)
	}

	return blocks
}

// generateMigrationID creates a unique ID for migrated blocks.
func generateMigrationID() string {
	return GenerateID()
}

// ═══════════════════════════════════════════════════════════════════════════════
// CONTAINER SERIALIZATION
// ═══════════════════════════════════════════════════════════════════════════════

// ContainerToConversation converts a BlockContainer to ConversationData.
func ContainerToConversation(container *BlockContainer, id, title, model, provider string) *ConversationData {
	if container == nil {
		return nil
	}

	rootBlocks := container.GetRootBlocks()
	blocks := make([]*BlockData, len(rootBlocks))
	for i, b := range rootBlocks {
		blocks[i] = BlockToData(b)
	}

	return &ConversationData{
		Version:   currentSchemaVersion,
		ID:        id,
		Title:     title,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Model:     model,
		Provider:  provider,
		Blocks:    blocks,
		Metadata:  make(map[string]interface{}),
	}
}

// ConversationToContainer loads a ConversationData into a BlockContainer.
func ConversationToContainer(conv *ConversationData) *BlockContainer {
	if conv == nil {
		return nil
	}

	container := NewBlockContainer()

	for _, blockData := range conv.Blocks {
		block := DataToBlock(blockData)
		container.AddBlock(block)
	}

	return container
}

// ═══════════════════════════════════════════════════════════════════════════════
// EXPORT
// ═══════════════════════════════════════════════════════════════════════════════

// ExportAsMarkdown exports a conversation as a formatted markdown document.
func ExportAsMarkdown(conv *ConversationData) string {
	var sb strings.Builder

	// Header
	sb.WriteString("# ")
	if conv.Title != "" {
		sb.WriteString(conv.Title)
	} else {
		sb.WriteString("Conversation")
	}
	sb.WriteString("\n\n")

	// Metadata
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("Created: %s\n", conv.CreatedAt.Format("2006-01-02 15:04:05")))
	if conv.Model != "" {
		sb.WriteString(fmt.Sprintf("Model: %s\n", conv.Model))
	}
	if conv.Provider != "" {
		sb.WriteString(fmt.Sprintf("Provider: %s\n", conv.Provider))
	}
	sb.WriteString("---\n\n")

	// Export blocks
	for _, block := range conv.Blocks {
		exportBlockToMarkdown(&sb, block, 0)
	}

	return sb.String()
}

// exportBlockToMarkdown writes a single block as markdown.
func exportBlockToMarkdown(sb *strings.Builder, block *BlockData, depth int) {
	indent := strings.Repeat("  ", depth)

	switch block.Type {
	case "user":
		sb.WriteString(indent + "## 👤 You\n\n")
		sb.WriteString(indent + block.Content + "\n\n")

	case "assistant":
		sb.WriteString(indent + "## 🤖 Assistant\n\n")
		// Children will be exported separately
		if len(block.Children) == 0 && block.Content != "" {
			sb.WriteString(indent + block.Content + "\n\n")
		}

	case "tool":
		sb.WriteString(indent + "### 🔧 Tool: " + block.ToolName + "\n\n")
		if block.ToolInput != "" {
			sb.WriteString(indent + "**Input:**\n")
			sb.WriteString(indent + "```\n" + block.ToolInput + "\n" + indent + "```\n\n")
		}
		if block.ToolOutput != "" {
			sb.WriteString(indent + "**Output:**\n")
			sb.WriteString(indent + "```\n" + block.ToolOutput + "\n" + indent + "```\n\n")
		}

	case "code":
		lang := ""
		if block.Language != "" {
			lang = block.Language
		}
		sb.WriteString(indent + "```" + lang + "\n")
		sb.WriteString(block.Content + "\n")
		sb.WriteString(indent + "```\n\n")

	case "thinking":
		sb.WriteString(indent + "> 💭 *Thinking:* " + block.Content + "\n\n")

	case "system":
		sb.WriteString(indent + "> ℹ️ " + block.Content + "\n\n")

	case "error":
		sb.WriteString(indent + "> ❌ **Error:** " + block.Content + "\n\n")

	default:
		sb.WriteString(indent + block.Content + "\n\n")
	}

	// Export children
	for _, child := range block.Children {
		exportBlockToMarkdown(sb, child, depth+1)
	}
}

// ExportAsJSON exports a conversation as pretty-printed JSON.
func ExportAsJSON(conv *ConversationData) (string, error) {
	data, err := json.MarshalIndent(conv, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
