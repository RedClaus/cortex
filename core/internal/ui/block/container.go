package block

import (
	"sync"
)

// ═══════════════════════════════════════════════════════════════════════════════
// BLOCK CONTAINER
// ═══════════════════════════════════════════════════════════════════════════════

// BlockContainer manages the hierarchical collection of blocks in a conversation.
// It provides thread-safe operations for adding, retrieving, and navigating blocks.
type BlockContainer struct {
	// blocks is a map of block ID -> Block for O(1) lookups
	blocks map[string]*Block

	// rootBlocks contains top-level blocks (User and Assistant blocks)
	rootBlocks []*Block

	// activeBlock points to the currently streaming block (if any)
	activeBlock *Block

	// focusedBlockID is the ID of the currently focused block for navigation
	focusedBlockID string

	// mu protects concurrent access
	mu sync.RWMutex
}

// NewBlockContainer creates a new empty block container.
func NewBlockContainer() *BlockContainer {
	return &BlockContainer{
		blocks:     make(map[string]*Block),
		rootBlocks: make([]*Block, 0),
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// BLOCK MANAGEMENT
// ═══════════════════════════════════════════════════════════════════════════════

// AddBlock adds a block to the container.
// If the block has no parent, it's added as a root block.
// Returns the block ID.
func (c *BlockContainer) AddBlock(b *Block) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Store in map for O(1) lookup
	c.blocks[b.ID] = b

	// Add to rootBlocks if it's a top-level block
	if b.ParentID == "" {
		c.rootBlocks = append(c.rootBlocks, b)
	}

	return b.ID
}

// AddChildBlock adds a block as a child of another block.
func (c *BlockContainer) AddChildBlock(parentID string, child *Block) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	parent, ok := c.blocks[parentID]
	if !ok {
		// If parent doesn't exist, add as root
		c.blocks[child.ID] = child
		c.rootBlocks = append(c.rootBlocks, child)
		return child.ID
	}

	// Set parent relationship and add child
	child.ParentID = parentID
	parent.Children = append(parent.Children, child)
	c.blocks[child.ID] = child

	return child.ID
}

// GetBlock retrieves a block by ID.
func (c *BlockContainer) GetBlock(id string) *Block {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.blocks[id]
}

// GetRootBlocks returns all top-level blocks.
func (c *BlockContainer) GetRootBlocks() []*Block {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make([]*Block, len(c.rootBlocks))
	copy(result, c.rootBlocks)
	return result
}

// GetAllBlocks returns all blocks in the container.
func (c *BlockContainer) GetAllBlocks() []*Block {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*Block, 0, len(c.blocks))
	for _, b := range c.blocks {
		result = append(result, b)
	}
	return result
}

// BlockCount returns the total number of blocks.
func (c *BlockContainer) BlockCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.blocks)
}

// RootBlockCount returns the number of root-level blocks.
func (c *BlockContainer) RootBlockCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.rootBlocks)
}

// RemoveBlock removes a block by ID.
// If the block has children, they are also removed.
func (c *BlockContainer) RemoveBlock(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	block, ok := c.blocks[id]
	if !ok {
		return false
	}

	// Recursively remove children
	for _, child := range block.Children {
		c.removeBlockInternal(child.ID)
	}

	// Remove from parent's children if it has a parent
	if block.ParentID != "" {
		if parent, ok := c.blocks[block.ParentID]; ok {
			for i, child := range parent.Children {
				if child.ID == id {
					parent.Children = append(parent.Children[:i], parent.Children[i+1:]...)
					break
				}
			}
		}
	} else {
		// Remove from rootBlocks
		for i, root := range c.rootBlocks {
			if root.ID == id {
				c.rootBlocks = append(c.rootBlocks[:i], c.rootBlocks[i+1:]...)
				break
			}
		}
	}

	delete(c.blocks, id)
	return true
}

// removeBlockInternal recursively removes a block and its children.
// Must be called with lock held.
func (c *BlockContainer) removeBlockInternal(id string) {
	block, ok := c.blocks[id]
	if !ok {
		return
	}

	for _, child := range block.Children {
		c.removeBlockInternal(child.ID)
	}

	delete(c.blocks, id)
}

// Clear removes all blocks from the container.
func (c *BlockContainer) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.blocks = make(map[string]*Block)
	c.rootBlocks = make([]*Block, 0)
	c.activeBlock = nil
	c.focusedBlockID = ""
}

// ═══════════════════════════════════════════════════════════════════════════════
// ACTIVE BLOCK MANAGEMENT
// ═══════════════════════════════════════════════════════════════════════════════

// SetActiveBlock sets the currently streaming/active block.
func (c *BlockContainer) SetActiveBlock(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if id == "" {
		c.activeBlock = nil
		return
	}

	if block, ok := c.blocks[id]; ok {
		c.activeBlock = block
	}
}

// GetActiveBlock returns the currently active block.
func (c *BlockContainer) GetActiveBlock() *Block {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.activeBlock
}

// ClearActiveBlock clears the active block reference.
func (c *BlockContainer) ClearActiveBlock() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.activeBlock = nil
}

// HasActiveBlock returns whether there's an active (streaming) block.
func (c *BlockContainer) HasActiveBlock() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.activeBlock != nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// FOCUS MANAGEMENT
// ═══════════════════════════════════════════════════════════════════════════════

// SetFocusedBlock sets the currently focused block for navigation.
func (c *BlockContainer) SetFocusedBlock(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Clear previous focus
	if c.focusedBlockID != "" {
		if prev, ok := c.blocks[c.focusedBlockID]; ok {
			prev.SetFocused(false)
		}
	}

	c.focusedBlockID = id

	// Set new focus
	if id != "" {
		if block, ok := c.blocks[id]; ok {
			block.SetFocused(true)
		}
	}
}

// GetFocusedBlock returns the currently focused block.
func (c *BlockContainer) GetFocusedBlock() *Block {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.focusedBlockID == "" {
		return nil
	}
	return c.blocks[c.focusedBlockID]
}

// GetFocusedBlockID returns the ID of the focused block.
func (c *BlockContainer) GetFocusedBlockID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.focusedBlockID
}

// ═══════════════════════════════════════════════════════════════════════════════
// NAVIGATION HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

// GetBlockLineage returns the ancestor chain from root to the given block.
// The result is ordered from root to the specified block.
func (c *BlockContainer) GetBlockLineage(id string) []*Block {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var lineage []*Block
	current := c.blocks[id]

	for current != nil {
		lineage = append([]*Block{current}, lineage...)
		if current.ParentID == "" {
			break
		}
		current = c.blocks[current.ParentID]
	}

	return lineage
}

// GetSiblings returns all blocks at the same level as the given block.
func (c *BlockContainer) GetSiblings(id string) []*Block {
	c.mu.RLock()
	defer c.mu.RUnlock()

	block, ok := c.blocks[id]
	if !ok {
		return nil
	}

	if block.ParentID == "" {
		// Root level - return other root blocks
		return c.rootBlocks
	}

	// Has parent - return parent's children
	parent, ok := c.blocks[block.ParentID]
	if !ok {
		return nil
	}

	return parent.Children
}

// GetNextSibling returns the next sibling block.
func (c *BlockContainer) GetNextSibling(id string) *Block {
	siblings := c.GetSiblings(id)
	for i, s := range siblings {
		if s.ID == id && i < len(siblings)-1 {
			return siblings[i+1]
		}
	}
	return nil
}

// GetPrevSibling returns the previous sibling block.
func (c *BlockContainer) GetPrevSibling(id string) *Block {
	siblings := c.GetSiblings(id)
	for i, s := range siblings {
		if s.ID == id && i > 0 {
			return siblings[i-1]
		}
	}
	return nil
}

// GetParent returns the parent block.
func (c *BlockContainer) GetParent(id string) *Block {
	c.mu.RLock()
	defer c.mu.RUnlock()

	block, ok := c.blocks[id]
	if !ok || block.ParentID == "" {
		return nil
	}

	return c.blocks[block.ParentID]
}

// GetFirstChild returns the first child of a block.
func (c *BlockContainer) GetFirstChild(id string) *Block {
	c.mu.RLock()
	defer c.mu.RUnlock()

	block, ok := c.blocks[id]
	if !ok || len(block.Children) == 0 {
		return nil
	}

	return block.Children[0]
}

// GetLastBlock returns the last root block.
func (c *BlockContainer) GetLastBlock() *Block {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.rootBlocks) == 0 {
		return nil
	}
	return c.rootBlocks[len(c.rootBlocks)-1]
}

// GetFirstBlock returns the first root block.
func (c *BlockContainer) GetFirstBlock() *Block {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.rootBlocks) == 0 {
		return nil
	}
	return c.rootBlocks[0]
}

// ═══════════════════════════════════════════════════════════════════════════════
// FLAT ITERATION
// ═══════════════════════════════════════════════════════════════════════════════

// GetFlattenedBlocks returns all blocks in display order (depth-first).
// This is useful for rendering and navigation.
func (c *BlockContainer) GetFlattenedBlocks() []*Block {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []*Block
	for _, root := range c.rootBlocks {
		result = append(result, c.flattenBlock(root)...)
	}
	return result
}

// flattenBlock recursively flattens a block and its children.
func (c *BlockContainer) flattenBlock(b *Block) []*Block {
	result := []*Block{b}
	for _, child := range b.Children {
		result = append(result, c.flattenBlock(child)...)
	}
	return result
}

// GetFlattenedIndex returns the index of a block in flattened order.
// Returns -1 if not found.
func (c *BlockContainer) GetFlattenedIndex(id string) int {
	flat := c.GetFlattenedBlocks()
	for i, b := range flat {
		if b.ID == id {
			return i
		}
	}
	return -1
}

// GetBlockAtFlatIndex returns the block at the given flattened index.
func (c *BlockContainer) GetBlockAtFlatIndex(index int) *Block {
	flat := c.GetFlattenedBlocks()
	if index < 0 || index >= len(flat) {
		return nil
	}
	return flat[index]
}

// ═══════════════════════════════════════════════════════════════════════════════
// QUERY METHODS
// ═══════════════════════════════════════════════════════════════════════════════

// FindBlocksByType returns all blocks of a specific type.
func (c *BlockContainer) FindBlocksByType(blockType BlockType) []*Block {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []*Block
	for _, b := range c.blocks {
		if b.Type == blockType {
			result = append(result, b)
		}
	}
	return result
}

// FindBlocksByState returns all blocks in a specific state.
func (c *BlockContainer) FindBlocksByState(state BlockState) []*Block {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []*Block
	for _, b := range c.blocks {
		if b.State == state {
			result = append(result, b)
		}
	}
	return result
}

// FindBookmarkedBlocks returns all bookmarked blocks.
func (c *BlockContainer) FindBookmarkedBlocks() []*Block {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []*Block
	for _, b := range c.blocks {
		if b.Bookmarked {
			result = append(result, b)
		}
	}
	return result
}

// IsEmpty returns whether the container has no blocks.
func (c *BlockContainer) IsEmpty() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.blocks) == 0
}
