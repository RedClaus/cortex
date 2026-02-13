package block

import (
	"fmt"
	"sync"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// BRANCH TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// Branch represents a conversation branch point.
// Branches allow users to regenerate from any point in the conversation,
// creating alternative response paths while preserving the original.
type Branch struct {
	// ID is the unique identifier for this branch
	ID string

	// Name is an optional human-readable name for the branch
	Name string

	// ParentBranchID is the ID of the parent branch (empty for main branch)
	ParentBranchID string

	// BranchPointBlockID is the block ID where this branch diverges from parent
	BranchPointBlockID string

	// Blocks contains the block IDs in this branch (in order)
	Blocks []string

	// CreatedAt is when the branch was created
	CreatedAt time.Time

	// Metadata holds additional branch information
	Metadata map[string]interface{}
}

// ═══════════════════════════════════════════════════════════════════════════════
// BRANCH MANAGER
// ═══════════════════════════════════════════════════════════════════════════════

// BranchManager handles conversation branching and forking.
type BranchManager struct {
	// branches maps branch ID -> Branch
	branches map[string]*Branch

	// activeBranchID is the currently active branch
	activeBranchID string

	// mainBranchID is the ID of the main/original branch
	mainBranchID string

	// container is the block container this manager works with
	container *BlockContainer

	// branchCounter for generating unique branch IDs
	branchCounter uint64

	// mu protects concurrent access
	mu sync.RWMutex
}

// NewBranchManager creates a new branch manager.
func NewBranchManager(container *BlockContainer) *BranchManager {
	mainBranchID := "branch_main"

	bm := &BranchManager{
		branches:       make(map[string]*Branch),
		container:      container,
		mainBranchID:   mainBranchID,
		activeBranchID: mainBranchID,
	}

	// Create the main branch
	bm.branches[mainBranchID] = &Branch{
		ID:        mainBranchID,
		Name:      "Main",
		Blocks:    make([]string, 0),
		CreatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	return bm
}

// ═══════════════════════════════════════════════════════════════════════════════
// BRANCH OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════════

// CreateBranch creates a new branch from a specific block.
// The new branch includes all blocks up to and including the branch point,
// then diverges for new content.
func (bm *BranchManager) CreateBranch(fromBlockID string) (string, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// Verify the block exists
	block := bm.container.GetBlock(fromBlockID)
	if block == nil {
		return "", fmt.Errorf("block not found: %s", fromBlockID)
	}

	// Generate branch ID
	bm.branchCounter++
	branchID := fmt.Sprintf("branch_%d_%x", bm.branchCounter, time.Now().UnixMilli()%0xFFFF)

	// Get the lineage to the branch point
	lineage := bm.container.GetBlockLineage(fromBlockID)
	blockIDs := make([]string, len(lineage))
	for i, b := range lineage {
		blockIDs[i] = b.ID
	}

	// Create the branch
	branch := &Branch{
		ID:                 branchID,
		ParentBranchID:     bm.activeBranchID,
		BranchPointBlockID: fromBlockID,
		Blocks:             blockIDs,
		CreatedAt:          time.Now(),
		Metadata:           make(map[string]interface{}),
	}

	bm.branches[branchID] = branch

	return branchID, nil
}

// CreateBranchWithName creates a named branch from a block.
func (bm *BranchManager) CreateBranchWithName(fromBlockID, name string) (string, error) {
	branchID, err := bm.CreateBranch(fromBlockID)
	if err != nil {
		return "", err
	}

	bm.mu.Lock()
	bm.branches[branchID].Name = name
	bm.mu.Unlock()

	return branchID, nil
}

// SwitchBranch switches to a different branch.
func (bm *BranchManager) SwitchBranch(branchID string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if _, ok := bm.branches[branchID]; !ok {
		return fmt.Errorf("branch not found: %s", branchID)
	}

	bm.activeBranchID = branchID
	return nil
}

// DeleteBranch removes a branch (cannot delete main branch).
func (bm *BranchManager) DeleteBranch(branchID string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if branchID == bm.mainBranchID {
		return fmt.Errorf("cannot delete main branch")
	}

	if _, ok := bm.branches[branchID]; !ok {
		return fmt.Errorf("branch not found: %s", branchID)
	}

	// If we're deleting the active branch, switch to main
	if bm.activeBranchID == branchID {
		bm.activeBranchID = bm.mainBranchID
	}

	delete(bm.branches, branchID)
	return nil
}

// RenameBranch renames a branch.
func (bm *BranchManager) RenameBranch(branchID, newName string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	branch, ok := bm.branches[branchID]
	if !ok {
		return fmt.Errorf("branch not found: %s", branchID)
	}

	branch.Name = newName
	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// BLOCK TRACKING
// ═══════════════════════════════════════════════════════════════════════════════

// AddBlockToBranch adds a block ID to the active branch.
func (bm *BranchManager) AddBlockToBranch(blockID string) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	branch, ok := bm.branches[bm.activeBranchID]
	if !ok {
		return
	}

	branch.Blocks = append(branch.Blocks, blockID)
}

// GetBranchBlocks returns the block IDs in a branch.
func (bm *BranchManager) GetBranchBlocks(branchID string) []string {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	branch, ok := bm.branches[branchID]
	if !ok {
		return nil
	}

	// Return a copy
	result := make([]string, len(branch.Blocks))
	copy(result, branch.Blocks)
	return result
}

// GetActiveBranchBlocks returns block IDs in the active branch.
func (bm *BranchManager) GetActiveBranchBlocks() []string {
	return bm.GetBranchBlocks(bm.activeBranchID)
}

// ═══════════════════════════════════════════════════════════════════════════════
// QUERY METHODS
// ═══════════════════════════════════════════════════════════════════════════════

// GetBranch retrieves a branch by ID.
func (bm *BranchManager) GetBranch(branchID string) *Branch {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	return bm.branches[branchID]
}

// GetActiveBranch returns the currently active branch.
func (bm *BranchManager) GetActiveBranch() *Branch {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	return bm.branches[bm.activeBranchID]
}

// GetActiveBranchID returns the ID of the active branch.
func (bm *BranchManager) GetActiveBranchID() string {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	return bm.activeBranchID
}

// GetMainBranchID returns the ID of the main branch.
func (bm *BranchManager) GetMainBranchID() string {
	return bm.mainBranchID
}

// IsMainBranch returns whether the given branch is the main branch.
func (bm *BranchManager) IsMainBranch(branchID string) bool {
	return branchID == bm.mainBranchID
}

// GetAllBranches returns all branches.
func (bm *BranchManager) GetAllBranches() []*Branch {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	result := make([]*Branch, 0, len(bm.branches))
	for _, b := range bm.branches {
		result = append(result, b)
	}
	return result
}

// BranchCount returns the number of branches.
func (bm *BranchManager) BranchCount() int {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	return len(bm.branches)
}

// GetChildBranches returns branches that diverge from a specific branch.
func (bm *BranchManager) GetChildBranches(parentBranchID string) []*Branch {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	var children []*Branch
	for _, b := range bm.branches {
		if b.ParentBranchID == parentBranchID {
			children = append(children, b)
		}
	}
	return children
}

// GetBranchLineage returns the ancestry of a branch from main to the branch.
func (bm *BranchManager) GetBranchLineage(branchID string) []*Branch {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	var lineage []*Branch
	current := bm.branches[branchID]

	for current != nil {
		lineage = append([]*Branch{current}, lineage...)
		if current.ParentBranchID == "" {
			break
		}
		current = bm.branches[current.ParentBranchID]
	}

	return lineage
}

// ═══════════════════════════════════════════════════════════════════════════════
// BRANCH VISUALIZATION
// ═══════════════════════════════════════════════════════════════════════════════

// GetBranchTree returns a tree representation of all branches.
// Returns a map of branch ID -> child branch IDs.
func (bm *BranchManager) GetBranchTree() map[string][]string {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	tree := make(map[string][]string)

	// Initialize with empty slices
	for id := range bm.branches {
		tree[id] = make([]string, 0)
	}

	// Build parent -> children relationships
	for id, branch := range bm.branches {
		if branch.ParentBranchID != "" {
			tree[branch.ParentBranchID] = append(tree[branch.ParentBranchID], id)
		}
	}

	return tree
}

// HasBranches returns whether there are multiple branches.
func (bm *BranchManager) HasBranches() bool {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	return len(bm.branches) > 1
}

// ═══════════════════════════════════════════════════════════════════════════════
// BRANCH METADATA
// ═══════════════════════════════════════════════════════════════════════════════

// SetBranchMetadata sets a metadata value on a branch.
func (bm *BranchManager) SetBranchMetadata(branchID, key string, value interface{}) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	branch, ok := bm.branches[branchID]
	if !ok {
		return fmt.Errorf("branch not found: %s", branchID)
	}

	if branch.Metadata == nil {
		branch.Metadata = make(map[string]interface{})
	}
	branch.Metadata[key] = value
	return nil
}

// GetBranchMetadata retrieves a metadata value from a branch.
func (bm *BranchManager) GetBranchMetadata(branchID, key string) (interface{}, bool) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	branch, ok := bm.branches[branchID]
	if !ok {
		return nil, false
	}

	if branch.Metadata == nil {
		return nil, false
	}

	val, ok := branch.Metadata[key]
	return val, ok
}
