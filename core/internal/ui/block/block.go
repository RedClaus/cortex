package block

import (
	"sync"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// BLOCK STRUCT
// ═══════════════════════════════════════════════════════════════════════════════

// Block represents a discrete, addressable unit in a conversation.
// Blocks can be nested (e.g., ToolBlocks inside AssistantBlocks) and support
// per-block actions like copy, collapse, regenerate, and bookmark.
type Block struct {
	// ─────────────────────────────────────────────────────────────────────────
	// Identity
	// ─────────────────────────────────────────────────────────────────────────

	// ID is the unique identifier for this block
	ID string

	// Type defines the semantic type (User, Assistant, Tool, etc.)
	Type BlockType

	// State is the current lifecycle state (Pending, Streaming, Complete, etc.)
	State BlockState

	// ─────────────────────────────────────────────────────────────────────────
	// Hierarchy
	// ─────────────────────────────────────────────────────────────────────────

	// ParentID links to the parent block (empty for root blocks)
	ParentID string

	// Children contains nested blocks (e.g., Tool blocks within Assistant blocks)
	Children []*Block

	// ─────────────────────────────────────────────────────────────────────────
	// Content
	// ─────────────────────────────────────────────────────────────────────────

	// Content is the raw text content of the block
	Content string

	// Metadata holds additional key-value data (model name, tokens, etc.)
	Metadata map[string]interface{}

	// ─────────────────────────────────────────────────────────────────────────
	// Timestamps
	// ─────────────────────────────────────────────────────────────────────────

	// Timestamp is when the block was created
	Timestamp time.Time

	// CompletedAt is when the block finished (streaming completed or error)
	CompletedAt time.Time

	// ─────────────────────────────────────────────────────────────────────────
	// Error State
	// ─────────────────────────────────────────────────────────────────────────

	// Error holds any error that occurred during block processing
	Error error

	// ─────────────────────────────────────────────────────────────────────────
	// Tool-Specific Fields
	// ─────────────────────────────────────────────────────────────────────────

	// ToolName is the name of the executed tool (for BlockTypeTool)
	ToolName string

	// ToolInput is the input/arguments passed to the tool
	ToolInput string

	// ToolOutput is the result returned by the tool
	ToolOutput string

	// ToolSuccess indicates whether the tool execution succeeded
	ToolSuccess bool

	// ToolDuration is how long the tool took to execute
	ToolDuration time.Duration

	// ─────────────────────────────────────────────────────────────────────────
	// Code-Specific Fields
	// ─────────────────────────────────────────────────────────────────────────

	// Language is the programming language for code blocks
	Language string

	// ─────────────────────────────────────────────────────────────────────────
	// UI State
	// ─────────────────────────────────────────────────────────────────────────

	// Collapsed indicates whether the block content is hidden
	Collapsed bool

	// Bookmarked indicates whether the user has bookmarked this block
	Bookmarked bool

	// Focused indicates whether this block currently has keyboard focus
	Focused bool

	// ─────────────────────────────────────────────────────────────────────────
	// Render Cache
	// ─────────────────────────────────────────────────────────────────────────

	// CachedRender stores the last rendered output for performance
	CachedRender string

	// NeedsRender indicates whether the cache is stale
	NeedsRender bool

	// ─────────────────────────────────────────────────────────────────────────
	// Concurrency
	// ─────────────────────────────────────────────────────────────────────────

	// mu protects concurrent access to block fields
	mu sync.RWMutex
}

// ═══════════════════════════════════════════════════════════════════════════════
// CONSTRUCTORS
// ═══════════════════════════════════════════════════════════════════════════════

// newBlock creates a base block with common initialization.
func newBlock(blockType BlockType) *Block {
	return &Block{
		ID:          GenerateID(),
		Type:        blockType,
		State:       BlockStatePending,
		Children:    make([]*Block, 0),
		Metadata:    make(map[string]interface{}),
		Timestamp:   time.Now(),
		NeedsRender: true,
	}
}

// NewUserBlock creates a new user input block with the given content.
func NewUserBlock(content string) *Block {
	b := newBlock(BlockTypeUser)
	b.Content = content
	b.State = BlockStateComplete // User blocks are immediately complete
	b.CompletedAt = time.Now()
	return b
}

// NewAssistantBlock creates a new assistant response container block.
// The block starts in Pending state, waiting for streaming content.
func NewAssistantBlock() *Block {
	return newBlock(BlockTypeAssistant)
}

// NewTextBlock creates a new markdown text block.
func NewTextBlock(content string) *Block {
	b := newBlock(BlockTypeText)
	b.Content = content
	return b
}

// NewCodeBlock creates a new code block with syntax highlighting support.
func NewCodeBlock(content, language string) *Block {
	b := newBlock(BlockTypeCode)
	b.Content = content
	b.Language = language
	return b
}

// NewToolBlock creates a new tool execution block.
func NewToolBlock(toolName, input string) *Block {
	b := newBlock(BlockTypeTool)
	b.ToolName = toolName
	b.ToolInput = input
	b.State = BlockStateStreaming // Tool is actively executing
	return b
}

// NewThinkingBlock creates a new AI thinking/reasoning block.
func NewThinkingBlock(content string) *Block {
	b := newBlock(BlockTypeThinking)
	b.Content = content
	b.Collapsed = true // Thinking blocks start collapsed by default
	return b
}

// NewErrorBlock creates a new error block.
func NewErrorBlock(err error) *Block {
	b := newBlock(BlockTypeError)
	b.Error = err
	if err != nil {
		b.Content = err.Error()
	}
	b.State = BlockStateError
	b.CompletedAt = time.Now()
	return b
}

// NewSystemBlock creates a new system notification block.
func NewSystemBlock(content string) *Block {
	b := newBlock(BlockTypeSystem)
	b.Content = content
	b.State = BlockStateComplete
	b.CompletedAt = time.Now()
	return b
}

// ═══════════════════════════════════════════════════════════════════════════════
// STATE TRANSITIONS
// ═══════════════════════════════════════════════════════════════════════════════

// StartStreaming transitions the block to streaming state.
func (b *Block) StartStreaming() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.State = BlockStateStreaming
	b.NeedsRender = true
}

// AppendContent adds content during streaming.
func (b *Block) AppendContent(chunk string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.Content += chunk
	b.NeedsRender = true
}

// SetContent replaces the block's content entirely.
func (b *Block) SetContent(content string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.Content = content
	b.NeedsRender = true
}

// MarkComplete transitions the block to complete state.
func (b *Block) MarkComplete() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.State = BlockStateComplete
	b.CompletedAt = time.Now()
	b.NeedsRender = true
}

// MarkError transitions the block to error state.
func (b *Block) MarkError(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.State = BlockStateError
	b.Error = err
	b.CompletedAt = time.Now()
	b.NeedsRender = true
}

// ═══════════════════════════════════════════════════════════════════════════════
// TOOL STATE TRANSITIONS
// ═══════════════════════════════════════════════════════════════════════════════

// SetToolOutput sets the tool execution result.
func (b *Block) SetToolOutput(output string, success bool, duration time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.ToolOutput = output
	b.ToolSuccess = success
	b.ToolDuration = duration
	b.State = BlockStateComplete
	b.CompletedAt = time.Now()
	b.NeedsRender = true
}

// ═══════════════════════════════════════════════════════════════════════════════
// UI STATE MANAGEMENT
// ═══════════════════════════════════════════════════════════════════════════════

// Toggle toggles the collapsed state of the block.
func (b *Block) Toggle() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.Type.IsCollapsible() {
		return
	}

	b.Collapsed = !b.Collapsed
	b.NeedsRender = true
}

// Collapse collapses the block (hides content).
func (b *Block) Collapse() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.Type.IsCollapsible() {
		b.Collapsed = true
		b.NeedsRender = true
	}
}

// Expand expands the block (shows content).
func (b *Block) Expand() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.Collapsed = false
	b.NeedsRender = true
}

// SetBookmarked sets the bookmark state.
func (b *Block) SetBookmarked(bookmarked bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.Bookmarked = bookmarked
	b.NeedsRender = true
}

// ToggleBookmark toggles the bookmark state.
func (b *Block) ToggleBookmark() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.Bookmarked = !b.Bookmarked
	b.NeedsRender = true
}

// SetFocused sets the focus state.
func (b *Block) SetFocused(focused bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.Focused = focused
	b.NeedsRender = true
}

// ═══════════════════════════════════════════════════════════════════════════════
// CHILD MANAGEMENT
// ═══════════════════════════════════════════════════════════════════════════════

// AddChild adds a child block and sets the parent relationship.
func (b *Block) AddChild(child *Block) {
	b.mu.Lock()
	defer b.mu.Unlock()

	child.ParentID = b.ID
	b.Children = append(b.Children, child)
	b.NeedsRender = true
}

// GetChildren returns a copy of the children slice.
func (b *Block) GetChildren() []*Block {
	b.mu.RLock()
	defer b.mu.RUnlock()

	children := make([]*Block, len(b.Children))
	copy(children, b.Children)
	return children
}

// ChildCount returns the number of child blocks.
func (b *Block) ChildCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return len(b.Children)
}

// FindChild finds a child block by ID (non-recursive).
func (b *Block) FindChild(id string) *Block {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, child := range b.Children {
		if child.ID == id {
			return child
		}
	}
	return nil
}

// FindChildRecursive finds a descendant block by ID (recursive).
func (b *Block) FindChildRecursive(id string) *Block {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, child := range b.Children {
		if child.ID == id {
			return child
		}
		if found := child.FindChildRecursive(id); found != nil {
			return found
		}
	}
	return nil
}

// LastChild returns the last child block, or nil if no children.
func (b *Block) LastChild() *Block {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.Children) == 0 {
		return nil
	}
	return b.Children[len(b.Children)-1]
}

// RemoveChild removes a child block by ID.
func (b *Block) RemoveChild(id string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i, child := range b.Children {
		if child.ID == id {
			b.Children = append(b.Children[:i], b.Children[i+1:]...)
			b.NeedsRender = true
			return true
		}
	}
	return false
}

// ═══════════════════════════════════════════════════════════════════════════════
// METADATA HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

// SetMetadata sets a metadata key-value pair.
func (b *Block) SetMetadata(key string, value interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.Metadata == nil {
		b.Metadata = make(map[string]interface{})
	}
	b.Metadata[key] = value
}

// GetMetadata retrieves a metadata value by key.
func (b *Block) GetMetadata(key string) (interface{}, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.Metadata == nil {
		return nil, false
	}
	val, ok := b.Metadata[key]
	return val, ok
}

// GetMetadataString retrieves a string metadata value.
func (b *Block) GetMetadataString(key string) string {
	val, ok := b.GetMetadata(key)
	if !ok {
		return ""
	}
	if str, ok := val.(string); ok {
		return str
	}
	return ""
}

// ═══════════════════════════════════════════════════════════════════════════════
// RENDER CACHE
// ═══════════════════════════════════════════════════════════════════════════════

// GetCachedRender returns the cached render if valid, or empty string if stale.
func (b *Block) GetCachedRender() string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.NeedsRender {
		return ""
	}
	return b.CachedRender
}

// SetCachedRender updates the render cache.
func (b *Block) SetCachedRender(rendered string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.CachedRender = rendered
	b.NeedsRender = false
}

// InvalidateCache marks the render cache as stale.
func (b *Block) InvalidateCache() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.NeedsRender = true
}

// ═══════════════════════════════════════════════════════════════════════════════
// QUERY HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

// IsStreaming returns whether the block is actively streaming.
func (b *Block) IsStreaming() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.State == BlockStateStreaming
}

// IsComplete returns whether the block has finished.
func (b *Block) IsComplete() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.State == BlockStateComplete
}

// IsError returns whether the block is in error state.
func (b *Block) IsError() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.State == BlockStateError
}

// IsRoot returns whether this is a root-level block (no parent).
func (b *Block) IsRoot() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.ParentID == ""
}

// HasChildren returns whether this block has child blocks.
func (b *Block) HasChildren() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return len(b.Children) > 0
}

// Duration returns the time elapsed since block creation.
// For completed blocks, returns the time between creation and completion.
func (b *Block) Duration() time.Duration {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if !b.CompletedAt.IsZero() {
		return b.CompletedAt.Sub(b.Timestamp)
	}
	return time.Since(b.Timestamp)
}

// ═══════════════════════════════════════════════════════════════════════════════
// THREAD-SAFE ACCESSORS
// ═══════════════════════════════════════════════════════════════════════════════

// GetContent returns the block content safely.
func (b *Block) GetContent() string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.Content
}

// GetState returns the block state safely.
func (b *Block) GetState() BlockState {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.State
}

// GetType returns the block type safely.
func (b *Block) GetType() BlockType {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.Type
}

// IsCollapsed returns whether the block is collapsed.
func (b *Block) IsCollapsed() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.Collapsed
}
