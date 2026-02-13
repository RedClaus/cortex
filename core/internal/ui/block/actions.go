// Package block provides block action handlers for the Cortex TUI.
package block

import (
	"fmt"
	"strings"
)

// ═══════════════════════════════════════════════════════════════════════════════
// ACTION TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// ActionType represents the type of action to perform on a block.
type ActionType int

const (
	// ActionCopy copies the block content to clipboard
	ActionCopy ActionType = iota

	// ActionToggle toggles the block's collapsed state
	ActionToggle

	// ActionBookmark toggles the block's bookmark status
	ActionBookmark

	// ActionRegenerate regenerates from this block (creates a branch)
	ActionRegenerate

	// ActionEdit allows editing user blocks
	ActionEdit

	// ActionDelete removes the block
	ActionDelete

	// ActionExpand expands a collapsed block
	ActionExpand

	// ActionCollapse collapses an expanded block
	ActionCollapse

	// ActionCopyCode copies only code blocks within the block
	ActionCopyCode

	// ActionScrollToTop scrolls to the top of the block content
	ActionScrollToTop

	// ActionScrollToBottom scrolls to the bottom of the block content
	ActionScrollToBottom
)

// String returns the string representation of an action type.
func (a ActionType) String() string {
	switch a {
	case ActionCopy:
		return "copy"
	case ActionToggle:
		return "toggle"
	case ActionBookmark:
		return "bookmark"
	case ActionRegenerate:
		return "regenerate"
	case ActionEdit:
		return "edit"
	case ActionDelete:
		return "delete"
	case ActionExpand:
		return "expand"
	case ActionCollapse:
		return "collapse"
	case ActionCopyCode:
		return "copy-code"
	case ActionScrollToTop:
		return "scroll-top"
	case ActionScrollToBottom:
		return "scroll-bottom"
	default:
		return "unknown"
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// ACTION RESULT
// ═══════════════════════════════════════════════════════════════════════════════

// ActionResult represents the result of performing an action.
type ActionResult struct {
	// Success indicates whether the action completed successfully
	Success bool

	// Message provides feedback about the action
	Message string

	// ClipboardContent contains content to be copied to clipboard
	ClipboardContent string

	// RequiresRefresh indicates the UI should refresh
	RequiresRefresh bool

	// NewBranchID is set when ActionRegenerate creates a new branch
	NewBranchID string

	// Error contains any error that occurred
	Error error
}

// ═══════════════════════════════════════════════════════════════════════════════
// ACTION EXECUTOR
// ═══════════════════════════════════════════════════════════════════════════════

// ActionExecutor handles block actions.
type ActionExecutor struct {
	container     *BlockContainer
	branchManager *BranchManager
}

// NewActionExecutor creates a new action executor.
func NewActionExecutor(container *BlockContainer, branchManager *BranchManager) *ActionExecutor {
	return &ActionExecutor{
		container:     container,
		branchManager: branchManager,
	}
}

// Execute performs an action on a block.
func (e *ActionExecutor) Execute(blockID string, action ActionType) ActionResult {
	if e.container == nil {
		return ActionResult{
			Success: false,
			Error:   fmt.Errorf("no block container"),
		}
	}

	block := e.container.GetBlock(blockID)
	if block == nil {
		return ActionResult{
			Success: false,
			Error:   fmt.Errorf("block not found: %s", blockID),
		}
	}

	switch action {
	case ActionCopy:
		return e.executeCopy(block)
	case ActionToggle:
		return e.executeToggle(block)
	case ActionBookmark:
		return e.executeBookmark(block)
	case ActionRegenerate:
		return e.executeRegenerate(block)
	case ActionEdit:
		return e.executeEdit(block)
	case ActionDelete:
		return e.executeDelete(block)
	case ActionExpand:
		return e.executeExpand(block)
	case ActionCollapse:
		return e.executeCollapse(block)
	case ActionCopyCode:
		return e.executeCopyCode(block)
	default:
		return ActionResult{
			Success: false,
			Error:   fmt.Errorf("unknown action: %v", action),
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// ACTION IMPLEMENTATIONS
// ═══════════════════════════════════════════════════════════════════════════════

// executeCopy copies the block content.
func (e *ActionExecutor) executeCopy(block *Block) ActionResult {
	content := block.GetContent()

	// Include tool output for tool blocks
	if block.Type == BlockTypeTool && block.ToolOutput != "" {
		content = fmt.Sprintf("Tool: %s\nInput: %s\nOutput:\n%s",
			block.ToolName, block.ToolInput, block.ToolOutput)
	}

	return ActionResult{
		Success:          true,
		Message:          "Copied to clipboard",
		ClipboardContent: content,
		RequiresRefresh:  false,
	}
}

// executeToggle toggles the collapsed state.
func (e *ActionExecutor) executeToggle(block *Block) ActionResult {
	wasCollapsed := block.Collapsed
	block.Toggle()

	var message string
	if wasCollapsed {
		message = "Block expanded"
	} else {
		message = "Block collapsed"
	}

	return ActionResult{
		Success:         true,
		Message:         message,
		RequiresRefresh: true,
	}
}

// executeBookmark toggles the bookmark status.
func (e *ActionExecutor) executeBookmark(block *Block) ActionResult {
	wasBookmarked := block.Bookmarked
	block.ToggleBookmark()

	var message string
	if wasBookmarked {
		message = "Bookmark removed"
	} else {
		message = "Block bookmarked"
	}

	return ActionResult{
		Success:         true,
		Message:         message,
		RequiresRefresh: true,
	}
}

// executeRegenerate initiates regeneration from this block.
func (e *ActionExecutor) executeRegenerate(block *Block) ActionResult {
	// Only user and assistant blocks can trigger regeneration
	if block.Type != BlockTypeUser && block.Type != BlockTypeAssistant {
		return ActionResult{
			Success: false,
			Error:   fmt.Errorf("cannot regenerate from this block type"),
		}
	}

	// Create a new branch from this point
	if e.branchManager == nil {
		return ActionResult{
			Success: false,
			Error:   fmt.Errorf("branch manager not available"),
		}
	}

	newBranchID, err := e.branchManager.CreateBranch(block.ID)
	if err != nil {
		return ActionResult{
			Success: false,
			Error:   err,
		}
	}

	return ActionResult{
		Success:         true,
		Message:         "Created new branch for regeneration",
		NewBranchID:     newBranchID,
		RequiresRefresh: true,
	}
}

// executeEdit initiates editing of a user block.
func (e *ActionExecutor) executeEdit(block *Block) ActionResult {
	// Only user blocks can be edited
	if block.Type != BlockTypeUser {
		return ActionResult{
			Success: false,
			Error:   fmt.Errorf("only user blocks can be edited"),
		}
	}

	// Return the current content for editing
	// The actual editing will be handled by the UI layer
	return ActionResult{
		Success:          true,
		Message:          "Edit mode enabled",
		ClipboardContent: block.Content, // Return content for editing
		RequiresRefresh:  false,
	}
}

// executeDelete removes a block.
func (e *ActionExecutor) executeDelete(block *Block) ActionResult {
	// Only allow deleting user blocks for safety
	if block.Type != BlockTypeUser {
		return ActionResult{
			Success: false,
			Error:   fmt.Errorf("only user blocks can be deleted"),
		}
	}

	// Remove from container
	if !e.container.RemoveBlock(block.ID) {
		return ActionResult{
			Success: false,
			Error:   fmt.Errorf("failed to delete block"),
		}
	}

	return ActionResult{
		Success:         true,
		Message:         "Block deleted",
		RequiresRefresh: true,
	}
}

// executeExpand expands a collapsed block.
func (e *ActionExecutor) executeExpand(block *Block) ActionResult {
	if !block.Collapsed {
		return ActionResult{
			Success: false,
			Message: "Block is already expanded",
		}
	}

	block.Expand()
	return ActionResult{
		Success:         true,
		Message:         "Block expanded",
		RequiresRefresh: true,
	}
}

// executeCollapse collapses an expanded block.
func (e *ActionExecutor) executeCollapse(block *Block) ActionResult {
	if block.Collapsed {
		return ActionResult{
			Success: false,
			Message: "Block is already collapsed",
		}
	}

	block.Collapse()
	return ActionResult{
		Success:         true,
		Message:         "Block collapsed",
		RequiresRefresh: true,
	}
}

// executeCopyCode extracts and copies only code blocks.
func (e *ActionExecutor) executeCopyCode(block *Block) ActionResult {
	var codeBlocks []string

	// If this is a code block, copy its content
	if block.Type == BlockTypeCode {
		codeBlocks = append(codeBlocks, block.Content)
	}

	// Check children for code blocks
	for _, child := range block.GetChildren() {
		if child.Type == BlockTypeCode {
			codeBlocks = append(codeBlocks, child.Content)
		}
	}

	if len(codeBlocks) == 0 {
		return ActionResult{
			Success: false,
			Message: "No code blocks found",
		}
	}

	content := strings.Join(codeBlocks, "\n\n")

	return ActionResult{
		Success:          true,
		Message:          fmt.Sprintf("Copied %d code block(s)", len(codeBlocks)),
		ClipboardContent: content,
		RequiresRefresh:  false,
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// AVAILABLE ACTIONS
// ═══════════════════════════════════════════════════════════════════════════════

// GetAvailableActions returns the actions available for a specific block type.
func GetAvailableActions(blockType BlockType) []ActionType {
	// Base actions available for all blocks
	baseActions := []ActionType{
		ActionCopy,
		ActionBookmark,
	}

	switch blockType {
	case BlockTypeUser:
		return append(baseActions,
			ActionEdit,
			ActionRegenerate,
			ActionDelete,
		)

	case BlockTypeAssistant:
		return append(baseActions,
			ActionToggle,
			ActionRegenerate,
			ActionCopyCode,
		)

	case BlockTypeTool:
		return append(baseActions,
			ActionToggle,
		)

	case BlockTypeThinking:
		return append(baseActions,
			ActionToggle,
		)

	case BlockTypeCode:
		return baseActions

	case BlockTypeError:
		return baseActions

	case BlockTypeSystem:
		return []ActionType{ActionCopy} // Minimal actions for system blocks

	default:
		return baseActions
	}
}

// CanPerformAction checks if an action can be performed on a block.
func CanPerformAction(block *Block, action ActionType) bool {
	if block == nil {
		return false
	}

	availableActions := GetAvailableActions(block.Type)
	for _, a := range availableActions {
		if a == action {
			return true
		}
	}
	return false
}

// ═══════════════════════════════════════════════════════════════════════════════
// ACTION SHORTCUTS
// ═══════════════════════════════════════════════════════════════════════════════

// ActionKeyBinding represents a keyboard shortcut for an action.
type ActionKeyBinding struct {
	Key         string
	Action      ActionType
	Description string
}

// DefaultActionKeyBindings returns the default keybindings for block actions.
func DefaultActionKeyBindings() []ActionKeyBinding {
	return []ActionKeyBinding{
		{Key: "c", Action: ActionCopy, Description: "Copy block content"},
		{Key: "t", Action: ActionToggle, Description: "Toggle collapse"},
		{Key: "b", Action: ActionBookmark, Description: "Toggle bookmark"},
		{Key: "r", Action: ActionRegenerate, Description: "Regenerate from here"},
		{Key: "e", Action: ActionEdit, Description: "Edit block"},
		{Key: "x", Action: ActionCopyCode, Description: "Copy code only"},
	}
}

// GetActionForKey returns the action for a given key, or -1 if not found.
func GetActionForKey(key string) ActionType {
	bindings := DefaultActionKeyBindings()
	for _, binding := range bindings {
		if binding.Key == key {
			return binding.Action
		}
	}
	return ActionType(-1)
}

// GetKeyForAction returns the key for a given action, or empty string if not found.
func GetKeyForAction(action ActionType) string {
	bindings := DefaultActionKeyBindings()
	for _, binding := range bindings {
		if binding.Action == action {
			return binding.Key
		}
	}
	return ""
}
