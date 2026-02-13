// Package block provides block navigation for the Cortex TUI.
package block

import "sync"

// ═══════════════════════════════════════════════════════════════════════════════
// BLOCK NAVIGATOR
// ═══════════════════════════════════════════════════════════════════════════════

// BlockNavigator handles focus and navigation between blocks.
// It maintains a reference to the block container and tracks the current focus.
type BlockNavigator struct {
	container      *BlockContainer
	focusedBlockID string
	history        []string // Navigation history for back/forward
	historyIndex   int
	mu             sync.RWMutex
}

// NewBlockNavigator creates a new navigator for the given container.
func NewBlockNavigator(container *BlockContainer) *BlockNavigator {
	return &BlockNavigator{
		container:    container,
		history:      make([]string, 0, 100),
		historyIndex: -1,
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// FOCUS MANAGEMENT
// ═══════════════════════════════════════════════════════════════════════════════

// GetFocusedBlockID returns the ID of the currently focused block.
func (n *BlockNavigator) GetFocusedBlockID() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.focusedBlockID
}

// GetFocusedBlock returns the currently focused block.
func (n *BlockNavigator) GetFocusedBlock() *Block {
	n.mu.RLock()
	blockID := n.focusedBlockID
	n.mu.RUnlock()

	if blockID == "" || n.container == nil {
		return nil
	}
	return n.container.GetBlock(blockID)
}

// SetFocus sets focus to a specific block by ID.
func (n *BlockNavigator) SetFocus(blockID string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Unfocus current block
	if n.focusedBlockID != "" && n.container != nil {
		if oldBlock := n.container.GetBlock(n.focusedBlockID); oldBlock != nil {
			oldBlock.SetFocused(false)
		}
	}

	// Focus new block
	n.focusedBlockID = blockID
	if blockID != "" && n.container != nil {
		if newBlock := n.container.GetBlock(blockID); newBlock != nil {
			newBlock.SetFocused(true)
		}
		// Also update container's focused block
		n.container.SetFocusedBlock(blockID)
	}

	// Add to history
	n.addToHistory(blockID)
}

// ClearFocus removes focus from the current block.
func (n *BlockNavigator) ClearFocus() {
	n.SetFocus("")
}

// HasFocus returns true if any block is currently focused.
func (n *BlockNavigator) HasFocus() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.focusedBlockID != ""
}

// ═══════════════════════════════════════════════════════════════════════════════
// NAVIGATION METHODS
// ═══════════════════════════════════════════════════════════════════════════════

// FocusFirst focuses the first visible block.
func (n *BlockNavigator) FocusFirst() bool {
	if n.container == nil {
		return false
	}

	blocks := n.container.GetFlattenedBlocks()
	if len(blocks) == 0 {
		return false
	}

	// Find first non-collapsed block
	for _, b := range blocks {
		if !b.Collapsed {
			n.SetFocus(b.ID)
			return true
		}
	}

	// Fallback to first block
	n.SetFocus(blocks[0].ID)
	return true
}

// FocusLast focuses the last visible block.
func (n *BlockNavigator) FocusLast() bool {
	if n.container == nil {
		return false
	}

	blocks := n.container.GetFlattenedBlocks()
	if len(blocks) == 0 {
		return false
	}

	// Find last visible block
	for i := len(blocks) - 1; i >= 0; i-- {
		if !blocks[i].Collapsed {
			n.SetFocus(blocks[i].ID)
			return true
		}
	}

	// Fallback to last block
	n.SetFocus(blocks[len(blocks)-1].ID)
	return true
}

// FocusNext moves focus to the next sibling block (down).
func (n *BlockNavigator) FocusNext() bool {
	if n.container == nil {
		return false
	}

	n.mu.RLock()
	currentID := n.focusedBlockID
	n.mu.RUnlock()

	// If no focus, focus first block
	if currentID == "" {
		return n.FocusFirst()
	}

	// Get flattened list of visible blocks
	blocks := n.container.GetFlattenedBlocks()
	if len(blocks) == 0 {
		return false
	}

	// Find current block index
	currentIndex := -1
	for i, b := range blocks {
		if b.ID == currentID {
			currentIndex = i
			break
		}
	}

	if currentIndex == -1 {
		return n.FocusFirst()
	}

	// Move to next block
	nextIndex := currentIndex + 1
	if nextIndex >= len(blocks) {
		// Wrap around to first block
		nextIndex = 0
	}

	n.SetFocus(blocks[nextIndex].ID)
	return true
}

// FocusPrev moves focus to the previous sibling block (up).
func (n *BlockNavigator) FocusPrev() bool {
	if n.container == nil {
		return false
	}

	n.mu.RLock()
	currentID := n.focusedBlockID
	n.mu.RUnlock()

	// If no focus, focus last block
	if currentID == "" {
		return n.FocusLast()
	}

	// Get flattened list of visible blocks
	blocks := n.container.GetFlattenedBlocks()
	if len(blocks) == 0 {
		return false
	}

	// Find current block index
	currentIndex := -1
	for i, b := range blocks {
		if b.ID == currentID {
			currentIndex = i
			break
		}
	}

	if currentIndex == -1 {
		return n.FocusLast()
	}

	// Move to previous block
	prevIndex := currentIndex - 1
	if prevIndex < 0 {
		// Wrap around to last block
		prevIndex = len(blocks) - 1
	}

	n.SetFocus(blocks[prevIndex].ID)
	return true
}

// FocusChild moves focus to the first child of the current block.
// Also expands the block if it's collapsed.
func (n *BlockNavigator) FocusChild() bool {
	block := n.GetFocusedBlock()
	if block == nil {
		return false
	}

	// Expand if collapsed
	if block.Collapsed {
		block.Expand()
		return true // Expanding counts as successful action
	}

	// Get children
	children := block.GetChildren()
	if len(children) == 0 {
		return false
	}

	// Focus first child
	n.SetFocus(children[0].ID)
	return true
}

// FocusParent moves focus to the parent block.
// Also collapses the current block if it has no parent.
func (n *BlockNavigator) FocusParent() bool {
	block := n.GetFocusedBlock()
	if block == nil {
		return false
	}

	// If block has parent, go to parent
	if block.ParentID != "" && n.container != nil {
		n.SetFocus(block.ParentID)
		return true
	}

	// No parent - collapse current block if it has children
	if len(block.Children) > 0 && !block.Collapsed {
		block.Collapse()
		return true
	}

	return false
}

// FocusNextSibling moves focus to the next sibling (skipping children).
func (n *BlockNavigator) FocusNextSibling() bool {
	block := n.GetFocusedBlock()
	if block == nil {
		return n.FocusNext()
	}

	if n.container == nil {
		return false
	}

	// Get siblings
	var siblings []*Block
	if block.ParentID != "" {
		parent := n.container.GetBlock(block.ParentID)
		if parent != nil {
			siblings = parent.GetChildren()
		}
	} else {
		// Root level - get root blocks
		siblings = n.container.GetRootBlocks()
	}

	// Find current index among siblings
	currentIndex := -1
	for i, s := range siblings {
		if s.ID == block.ID {
			currentIndex = i
			break
		}
	}

	if currentIndex == -1 || len(siblings) <= 1 {
		return false
	}

	// Move to next sibling
	nextIndex := currentIndex + 1
	if nextIndex >= len(siblings) {
		nextIndex = 0 // Wrap around
	}

	n.SetFocus(siblings[nextIndex].ID)
	return true
}

// FocusPrevSibling moves focus to the previous sibling (skipping children).
func (n *BlockNavigator) FocusPrevSibling() bool {
	block := n.GetFocusedBlock()
	if block == nil {
		return n.FocusPrev()
	}

	if n.container == nil {
		return false
	}

	// Get siblings
	var siblings []*Block
	if block.ParentID != "" {
		parent := n.container.GetBlock(block.ParentID)
		if parent != nil {
			siblings = parent.GetChildren()
		}
	} else {
		// Root level - get root blocks
		siblings = n.container.GetRootBlocks()
	}

	// Find current index among siblings
	currentIndex := -1
	for i, s := range siblings {
		if s.ID == block.ID {
			currentIndex = i
			break
		}
	}

	if currentIndex == -1 || len(siblings) <= 1 {
		return false
	}

	// Move to previous sibling
	prevIndex := currentIndex - 1
	if prevIndex < 0 {
		prevIndex = len(siblings) - 1 // Wrap around
	}

	n.SetFocus(siblings[prevIndex].ID)
	return true
}

// ═══════════════════════════════════════════════════════════════════════════════
// HISTORY NAVIGATION
// ═══════════════════════════════════════════════════════════════════════════════

// addToHistory adds a block ID to the navigation history.
func (n *BlockNavigator) addToHistory(blockID string) {
	if blockID == "" {
		return
	}

	// Don't add duplicates
	if n.historyIndex >= 0 && n.historyIndex < len(n.history) {
		if n.history[n.historyIndex] == blockID {
			return
		}
	}

	// Truncate history if we navigated back and then went somewhere new
	if n.historyIndex < len(n.history)-1 {
		n.history = n.history[:n.historyIndex+1]
	}

	// Add new entry
	n.history = append(n.history, blockID)
	n.historyIndex = len(n.history) - 1

	// Limit history size
	maxHistory := 100
	if len(n.history) > maxHistory {
		n.history = n.history[len(n.history)-maxHistory:]
		n.historyIndex = len(n.history) - 1
	}
}

// GoBack navigates to the previous block in history.
func (n *BlockNavigator) GoBack() bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.historyIndex <= 0 {
		return false
	}

	n.historyIndex--
	blockID := n.history[n.historyIndex]

	// Verify block still exists
	if n.container != nil {
		if b := n.container.GetBlock(blockID); b != nil {
			n.focusedBlockID = blockID
			b.SetFocused(true)
			n.container.SetFocusedBlock(blockID)
			return true
		}
	}

	return false
}

// GoForward navigates to the next block in history.
func (n *BlockNavigator) GoForward() bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.historyIndex >= len(n.history)-1 {
		return false
	}

	n.historyIndex++
	blockID := n.history[n.historyIndex]

	// Verify block still exists
	if n.container != nil {
		if b := n.container.GetBlock(blockID); b != nil {
			n.focusedBlockID = blockID
			b.SetFocused(true)
			n.container.SetFocusedBlock(blockID)
			return true
		}
	}

	return false
}

// ═══════════════════════════════════════════════════════════════════════════════
// SEARCH & JUMP
// ═══════════════════════════════════════════════════════════════════════════════

// FindNextBookmarked finds and focuses the next bookmarked block.
func (n *BlockNavigator) FindNextBookmarked() bool {
	if n.container == nil {
		return false
	}

	blocks := n.container.GetFlattenedBlocks()
	if len(blocks) == 0 {
		return false
	}

	n.mu.RLock()
	currentID := n.focusedBlockID
	n.mu.RUnlock()

	// Find current block index
	currentIndex := -1
	for i, b := range blocks {
		if b.ID == currentID {
			currentIndex = i
			break
		}
	}

	// Search from current position forward
	startIndex := 0
	if currentIndex >= 0 {
		startIndex = currentIndex + 1
	}

	// First pass: search from current to end
	for i := startIndex; i < len(blocks); i++ {
		if blocks[i].Bookmarked {
			n.SetFocus(blocks[i].ID)
			return true
		}
	}

	// Second pass: wrap around from beginning to current
	for i := 0; i < startIndex; i++ {
		if blocks[i].Bookmarked {
			n.SetFocus(blocks[i].ID)
			return true
		}
	}

	return false
}

// FindPrevBookmarked finds and focuses the previous bookmarked block.
func (n *BlockNavigator) FindPrevBookmarked() bool {
	if n.container == nil {
		return false
	}

	blocks := n.container.GetFlattenedBlocks()
	if len(blocks) == 0 {
		return false
	}

	n.mu.RLock()
	currentID := n.focusedBlockID
	n.mu.RUnlock()

	// Find current block index
	currentIndex := len(blocks)
	for i, b := range blocks {
		if b.ID == currentID {
			currentIndex = i
			break
		}
	}

	// Search from current position backward
	for i := currentIndex - 1; i >= 0; i-- {
		if blocks[i].Bookmarked {
			n.SetFocus(blocks[i].ID)
			return true
		}
	}

	// Wrap around from end
	for i := len(blocks) - 1; i >= currentIndex; i-- {
		if blocks[i].Bookmarked {
			n.SetFocus(blocks[i].ID)
			return true
		}
	}

	return false
}

// FindBlockByType finds and focuses the next block of a specific type.
func (n *BlockNavigator) FindBlockByType(blockType BlockType) bool {
	if n.container == nil {
		return false
	}

	blocks := n.container.GetFlattenedBlocks()
	if len(blocks) == 0 {
		return false
	}

	n.mu.RLock()
	currentID := n.focusedBlockID
	n.mu.RUnlock()

	// Find current block index
	currentIndex := -1
	for i, b := range blocks {
		if b.ID == currentID {
			currentIndex = i
			break
		}
	}

	// Search from current position forward
	startIndex := 0
	if currentIndex >= 0 {
		startIndex = currentIndex + 1
	}

	for i := startIndex; i < len(blocks); i++ {
		if blocks[i].Type == blockType {
			n.SetFocus(blocks[i].ID)
			return true
		}
	}

	// Wrap around
	for i := 0; i < startIndex; i++ {
		if blocks[i].Type == blockType {
			n.SetFocus(blocks[i].ID)
			return true
		}
	}

	return false
}

// JumpToBlock focuses a specific block by ID.
// Returns false if block doesn't exist.
func (n *BlockNavigator) JumpToBlock(blockID string) bool {
	if n.container == nil || blockID == "" {
		return false
	}

	block := n.container.GetBlock(blockID)
	if block == nil {
		return false
	}

	// Expand any collapsed ancestors
	n.expandAncestors(block)

	n.SetFocus(blockID)
	return true
}

// expandAncestors expands all collapsed ancestors of a block.
func (n *BlockNavigator) expandAncestors(block *Block) {
	if block == nil || n.container == nil {
		return
	}

	currentID := block.ParentID
	for currentID != "" {
		parent := n.container.GetBlock(currentID)
		if parent == nil {
			break
		}
		if parent.Collapsed {
			parent.Expand()
		}
		currentID = parent.ParentID
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// UTILITIES
// ═══════════════════════════════════════════════════════════════════════════════

// GetVisibleBlockCount returns the number of visible (non-collapsed) blocks.
func (n *BlockNavigator) GetVisibleBlockCount() int {
	if n.container == nil {
		return 0
	}
	return len(n.container.GetFlattenedBlocks())
}

// GetFocusedBlockIndex returns the index of the focused block in the flattened list.
// Returns -1 if no block is focused.
func (n *BlockNavigator) GetFocusedBlockIndex() int {
	if n.container == nil {
		return -1
	}

	n.mu.RLock()
	currentID := n.focusedBlockID
	n.mu.RUnlock()

	if currentID == "" {
		return -1
	}

	blocks := n.container.GetFlattenedBlocks()
	for i, b := range blocks {
		if b.ID == currentID {
			return i
		}
	}

	return -1
}
