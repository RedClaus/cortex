// Package ui provides the Charmbracelet TUI framework integration for Cortex.
package ui

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/normanking/cortex/internal/ui/block"
	"github.com/normanking/cortex/internal/ui/modals"
)

// update handles all messages and updates the model state.
// This is called by Model.Update() and follows Elm Architecture principles.
func update(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	// Handle different message types
	switch msg := msg.(type) {
	// ═══════════════════════════════════════════════════════════════════════════
	// TERMINAL EVENTS
	// ═══════════════════════════════════════════════════════════════════════════

	case tea.WindowSizeMsg:
		// Update dimensions
		m.width = msg.Width
		m.height = msg.Height

		// Mark as ready after first size message
		if !m.ready {
			m.ready = true
		}

		// Update viewport dimensions (leaving space for input and status bar)
		headerHeight := 3
		footerHeight := 3
		inputHeight := 5
		verticalMargin := headerHeight + footerHeight + inputHeight

		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - verticalMargin

		// Update input width
		m.input.SetWidth(msg.Width - 4)

		return m, nil

	// ═══════════════════════════════════════════════════════════════════════════
	// KEYBOARD EVENTS
	// ═══════════════════════════════════════════════════════════════════════════

	case tea.KeyMsg:
		// Handle modal-specific keys first
		if m.activeModal != ModalNone {
			return handleModalKeys(m, msg)
		}

		// Handle global keys
		switch {
		// Quit application (Ctrl+C when not streaming, or when no modal is open)
		case key.Matches(msg, m.keys.Quit):
			if m.isStreaming {
				// Cancel streaming instead of quitting
				if m.backend != nil {
					m.backend.CancelStream()
				}
				m.isStreaming = false
				m.streamBuffer = ""
				return m, nil
			}
			return m, tea.Quit

		// Send message (Enter without shift/alt)
		case key.Matches(msg, m.keys.Send):
			if m.input.Focused() && m.input.Value() != "" {
				return handleSendMessage(m)
			}

		// Open help modal
		case key.Matches(msg, m.keys.Help):
			m.activeModal = ModalHelp
			return m, nil

		// Open model selection modal
		case key.Matches(msg, m.keys.Model):
			m.activeModal = ModalModel
			return m, nil

		// Open theme selection modal
		case key.Matches(msg, m.keys.Theme):
			m.activeModal = ModalTheme
			return m, nil

		// Open session management modal
		case key.Matches(msg, m.keys.Session):
			m.activeModal = ModalSession
			return m, nil

		// Open audio device selection modal
		case key.Matches(msg, m.keys.Device):
			m.activeModal = ModalDevice
			return m, FetchAudioDevicesCmd()

		// Switch to normal mode
		case key.Matches(msg, m.keys.ModeNormal):
			m.mode = ModeNormal
			return m, nil

		// Switch to YOLO mode
		case key.Matches(msg, m.keys.ModeYolo):
			m.mode = ModeYolo
			return m, nil

		// Switch to plan mode
		case key.Matches(msg, m.keys.ModePlan):
			m.mode = ModePlan
			return m, nil

		// Clear conversation history
		case key.Matches(msg, m.keys.Clear):
			m.ClearMessages()
			m.viewport.SetContent("")
			return m, nil

		// New session
		case key.Matches(msg, m.keys.NewSession):
			m.ClearMessages()
			m.viewport.SetContent("")
			// TODO: Generate new session ID
			return m, nil
		}

		// ═══════════════════════════════════════════════════════════════════════
		// BLOCK NAVIGATION (CR-002)
		// Only active when block system is enabled and input is not focused
		// ═══════════════════════════════════════════════════════════════════════
		if m.useBlockSystem && !m.input.Focused() {
			switch {
			// Block navigation
			case key.Matches(msg, m.keys.BlockNext):
				if m.blockNavigator != nil {
					m.blockNavigator.FocusNext()
					m.viewport.SetContent(renderConversation(m))
				}
				return m, nil

			case key.Matches(msg, m.keys.BlockPrev):
				if m.blockNavigator != nil {
					m.blockNavigator.FocusPrev()
					m.viewport.SetContent(renderConversation(m))
				}
				return m, nil

			case key.Matches(msg, m.keys.BlockChild):
				if m.blockNavigator != nil {
					m.blockNavigator.FocusChild()
					m.viewport.SetContent(renderConversation(m))
				}
				return m, nil

			case key.Matches(msg, m.keys.BlockParent):
				if m.blockNavigator != nil {
					m.blockNavigator.FocusParent()
					m.viewport.SetContent(renderConversation(m))
				}
				return m, nil

			// Block actions
			case key.Matches(msg, m.keys.BlockCopy):
				return handleBlockAction(m, block.ActionCopy)

			case key.Matches(msg, m.keys.BlockToggle):
				return handleBlockAction(m, block.ActionToggle)

			case key.Matches(msg, m.keys.BlockBookmark):
				return handleBlockAction(m, block.ActionBookmark)

			case key.Matches(msg, m.keys.BlockRegenerate):
				return handleBlockAction(m, block.ActionRegenerate)

			case key.Matches(msg, m.keys.BlockEdit):
				return handleBlockAction(m, block.ActionEdit)

			case key.Matches(msg, m.keys.NextBookmark):
				if m.blockNavigator != nil {
					m.blockNavigator.FindNextBookmarked()
					m.viewport.SetContent(renderConversation(m))
				}
				return m, nil

			case key.Matches(msg, m.keys.PrevBookmark):
				if m.blockNavigator != nil {
					m.blockNavigator.FindPrevBookmarked()
					m.viewport.SetContent(renderConversation(m))
				}
				return m, nil
			}
		}

		// Update viewport for scrolling (when input is not focused)
		if !m.input.Focused() {
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
		}

		// Update input
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)

		return m, tea.Batch(cmds...)

	// ═══════════════════════════════════════════════════════════════════════════
	// STREAMING MESSAGES
	// ═══════════════════════════════════════════════════════════════════════════

	case StreamChunkMsg:
		return handleStreamChunk(m, msg)

	case StreamDoneMsg:
		return handleStreamDone(m, msg)

	case StreamErrorMsg:
		return handleStreamError(m, msg)

	case GlamourRenderTickMsg:
		return handleGlamourTick(m, msg)

	case MessageSentMsg:
		return handleMessageSent(m, msg)

	// ═══════════════════════════════════════════════════════════════════════════
	// BLOCK SYSTEM MESSAGES (CR-002)
	// ═══════════════════════════════════════════════════════════════════════════

	case BlockCreatedMsg:
		return handleBlockCreated(m, msg)

	case BlockUpdatedMsg:
		return handleBlockUpdated(m, msg)

	case BlockStateChangedMsg:
		return handleBlockStateChanged(m, msg)

	case ToolBlockStartedMsg:
		return handleToolBlockStarted(m, msg)

	case ToolBlockCompletedMsg:
		return handleToolBlockCompleted(m, msg)

	case BlockFocusMsg:
		return handleBlockFocus(m, msg)

	case BlockToggleMsg:
		return handleBlockToggle(m, msg)

	// ═══════════════════════════════════════════════════════════════════════════
	// AUDIO DEVICE MESSAGES
	// ═══════════════════════════════════════════════════════════════════════════

	case AudioDevicesLoadedMsg:
		return handleAudioDevicesLoaded(m, msg)

	case AudioDeviceSetMsg:
		return handleAudioDeviceSet(m, msg)

	case modals.DeviceSelectedMsg:
		return handleDeviceSelected(m, msg)

	case modals.DevicesLoadedMsg:
		// Forward to device selector
		if m.deviceSelector != nil {
			m.deviceSelector.SetDevices(msg)
		}
		return m, nil

	// ═══════════════════════════════════════════════════════════════════════════
	// SPINNER TICK
	// ═══════════════════════════════════════════════════════════════════════════

	default:
		// Update spinner
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// handleModalKeys handles keyboard input when a modal is open.
func handleModalKeys(m Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Close modal on ESC
	if key.Matches(msg, m.keys.Close) {
		m.activeModal = ModalNone
		return m, nil
	}

	// Handle device modal specifically
	if m.activeModal == ModalDevice && m.deviceSelector != nil {
		updated, cmd := m.deviceSelector.Update(msg)
		m.deviceSelector = updated.(*modals.DeviceSelector)
		return m, cmd
	}

	// TODO: Handle modal-specific navigation and selection
	// This will be expanded based on which modal is active

	return m, nil
}

// handleSendMessage processes a user message and initiates backend communication.
func handleSendMessage(m Model) (tea.Model, tea.Cmd) {
	content := m.input.Value()

	// ─────────────────────────────────────────────────────────────────────────
	// Block System Path (CR-002)
	// ─────────────────────────────────────────────────────────────────────────
	if m.useBlockSystem && m.blockContainer != nil {
		return handleSendMessageWithBlocks(m, content)
	}

	// ─────────────────────────────────────────────────────────────────────────
	// Legacy Message Path
	// ─────────────────────────────────────────────────────────────────────────

	// Add user message to history
	userMsg := NewUserMessage(content)
	m.AddMessage(userMsg)

	// Create pending assistant message
	assistantMsg := NewAssistantMessage()
	m.AddMessage(assistantMsg)

	// Set as active message
	m.activeMessage = assistantMsg

	// Set streaming state
	m.isStreaming = true
	m.streamBuffer = ""

	// Update viewport
	m.viewport.SetContent(renderConversation(m))
	m.viewport.GotoBottom()

	// Clear input
	m.input.Reset()

	// Return commands to send message to backend and start streaming
	return m, tea.Batch(
		m.spinner.Tick,
		SendMessageCmd(m.backend, content),
	)
}

// handleSendMessageWithBlocks handles sending a message using the block system.
func handleSendMessageWithBlocks(m Model, content string) (tea.Model, tea.Cmd) {
	// Create user block
	userBlock := block.NewUserBlock(content)
	m.blockContainer.AddBlock(userBlock)

	// Track in branch manager
	if m.branchManager != nil {
		m.branchManager.AddBlockToBranch(userBlock.ID)
	}

	// Create pending assistant block
	assistantBlock := block.NewAssistantBlock()
	assistantBlock.StartStreaming()
	m.blockContainer.AddBlock(assistantBlock)
	m.blockContainer.SetActiveBlock(assistantBlock.ID)

	// Track in branch manager
	if m.branchManager != nil {
		m.branchManager.AddBlockToBranch(assistantBlock.ID)
	}

	// Set as active block
	m.activeBlock = assistantBlock

	// Also maintain legacy state for compatibility
	m.isStreaming = true
	m.streamBuffer = ""

	// Update viewport
	m.viewport.SetContent(renderConversation(m))
	m.viewport.GotoBottom()

	// Clear input
	m.input.Reset()

	// Return commands to send message to backend and start streaming
	return m, tea.Batch(
		m.spinner.Tick,
		SendMessageCmd(m.backend, content),
	)
}

// waitForStreamChunk returns a command that waits for the next streaming chunk.
func waitForStreamChunk(backend Backend) tea.Cmd {
	if backend == nil {
		return nil
	}

	return func() tea.Msg {
		ch := backend.StreamChannel()
		chunk := <-ch
		return StreamChunkMsg{Chunk: chunk}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// MESSAGE HANDLERS
// ═══════════════════════════════════════════════════════════════════════════════

// handleStreamChunk processes incoming stream chunks from the backend.
// It accumulates chunks in the streamBuffer and debounces Glamour rendering
// to limit rendering to ~10 renders/sec max (100ms debounce).
func handleStreamChunk(m Model, msg StreamChunkMsg) (tea.Model, tea.Cmd) {
	// Append chunk content to stream buffer
	m.streamBuffer += msg.Chunk.Content

	// Update active message
	if m.activeMessage != nil {
		m.activeMessage.State = MessageStreaming
		m.activeMessage.RawContent = m.streamBuffer

		// Update the message in the messages array
		for i, message := range m.messages {
			if message.ID == m.activeMessage.ID {
				m.messages[i] = m.activeMessage
				break
			}
		}
	}

	var cmds []tea.Cmd

	// Continue reading from the stream
	cmds = append(cmds, streamReaderCmd(m.backend.StreamChannel()))

	// Debounce Glamour rendering (100ms threshold = ~10 renders/sec max)
	if time.Since(m.lastRenderTime) > 100*time.Millisecond {
		if m.activeMessage != nil {
			cmds = append(cmds, ScheduleGlamourRender(m.activeMessage.ID))
		}
	}

	return m, tea.Batch(cmds...)
}

// handleStreamDone handles the completion of a streaming operation.
// It marks the message as complete, performs a final Glamour render,
// caches the result, and clears the stream buffer.
func handleStreamDone(m Model, msg StreamDoneMsg) (tea.Model, tea.Cmd) {
	// Set streaming state to false
	m.isStreaming = false

	// Finalize the active message
	if m.activeMessage != nil {
		// Set final content from buffer
		m.activeMessage.RawContent = m.streamBuffer

		// Mark as complete or error
		if msg.Error != nil {
			m.activeMessage.MarkError(msg.Error)
		} else {
			m.activeMessage.MarkComplete()
		}

		// Invalidate cache to trigger final render
		// The actual rendering will be done by renderMessages in the viewport update
		if m.activeMessage.Role == RoleAssistant && m.activeMessage.RawContent != "" {
			m.activeMessage.CachedRender = "" // Force re-render
		}

		// Update the message in the array
		for i, message := range m.messages {
			if message.ID == m.activeMessage.ID {
				m.messages[i] = m.activeMessage
				break
			}
		}

		// Clear active message reference
		m.activeMessage = nil
	}

	// Clear stream buffer
	m.streamBuffer = ""

	// Update viewport with new content
	m.viewport.SetContent(renderConversation(m))
	m.viewport.GotoBottom()

	return m, nil
}

// handleStreamError handles errors during streaming.
func handleStreamError(m Model, msg StreamErrorMsg) (tea.Model, tea.Cmd) {
	m.isStreaming = false
	m.AddErrorMessage(msg.Error.Error())

	if m.activeMessage != nil {
		m.activeMessage.MarkError(msg.Error)
		m.activeMessage = nil
	}

	m.streamBuffer = ""
	return m, nil
}

// handleGlamourTick handles the debounced markdown rendering timer.
// This is triggered by a timer to limit rendering to ~10 renders/sec max.
func handleGlamourTick(m Model, msg GlamourRenderTickMsg) (tea.Model, tea.Cmd) {
	// Only render if we're still streaming
	if !m.isStreaming {
		return m, nil
	}

	// Only render if this is for the active message
	if m.activeMessage == nil || m.activeMessage.ID != msg.MessageID {
		return m, nil
	}

	// Invalidate cache to trigger debounced render
	// The actual rendering will be done by renderMessages in the viewport update
	if m.activeMessage.RawContent != "" {
		m.activeMessage.CachedRender = "" // Force re-render

		// Update the message in the array
		for i, message := range m.messages {
			if message.ID == m.activeMessage.ID {
				m.messages[i] = m.activeMessage
				break
			}
		}
	}

	// Update last render time
	m.lastRenderTime = time.Now()

	// Refresh viewport content
	m.viewport.SetContent(renderConversation(m))
	m.viewport.GotoBottom()

	return m, nil
}

// handleMessageSent handles confirmation that a message was sent to the backend.
func handleMessageSent(m Model, msg MessageSentMsg) (tea.Model, tea.Cmd) {
	// Create a new assistant message in pending state
	m.activeMessage = NewAssistantMessage()
	m.AddMessage(m.activeMessage)
	m.isStreaming = true

	// Start reading from the stream
	return m, streamReaderCmd(m.backend.StreamChannel())
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPER FUNCTIONS FOR BUBBLE COMPONENTS
// ═══════════════════════════════════════════════════════════════════════════════

// updateInput updates the input textarea component.
func updateInput(input textarea.Model, msg tea.Msg) (textarea.Model, tea.Cmd) {
	return input.Update(msg)
}

// ═══════════════════════════════════════════════════════════════════════════════
// BLOCK SYSTEM HANDLERS (CR-002)
// ═══════════════════════════════════════════════════════════════════════════════

// handleBlockCreated handles the creation of a new block.
func handleBlockCreated(m Model, msg BlockCreatedMsg) (tea.Model, tea.Cmd) {
	if m.blockContainer == nil {
		return m, nil
	}

	// Add block to container
	if msg.ParentID != "" {
		m.blockContainer.AddChildBlock(msg.ParentID, msg.Block)
	} else {
		m.blockContainer.AddBlock(msg.Block)
	}

	// Track in branch manager
	if m.branchManager != nil {
		m.branchManager.AddBlockToBranch(msg.Block.ID)
	}

	// If this is an assistant block, set it as active
	if msg.Block.Type == block.BlockTypeAssistant {
		m.blockContainer.SetActiveBlock(msg.Block.ID)
		m.activeBlock = msg.Block
	}

	// Refresh viewport
	m.viewport.SetContent(renderConversation(m))
	m.viewport.GotoBottom()

	return m, nil
}

// handleBlockUpdated handles updates to block content.
func handleBlockUpdated(m Model, msg BlockUpdatedMsg) (tea.Model, tea.Cmd) {
	if m.blockContainer == nil {
		return m, nil
	}

	b := m.blockContainer.GetBlock(msg.BlockID)
	if b == nil {
		return m, nil
	}

	// Update content
	if msg.Append {
		b.AppendContent(msg.Content)
	} else {
		b.SetContent(msg.Content)
	}

	// Debounce rendering during streaming
	var cmds []tea.Cmd
	if time.Since(m.lastRenderTime) > 100*time.Millisecond {
		m.viewport.SetContent(renderConversation(m))
		m.lastRenderTime = time.Now()
	}

	return m, tea.Batch(cmds...)
}

// handleBlockStateChanged handles block state transitions.
func handleBlockStateChanged(m Model, msg BlockStateChangedMsg) (tea.Model, tea.Cmd) {
	if m.blockContainer == nil {
		return m, nil
	}

	b := m.blockContainer.GetBlock(msg.BlockID)
	if b == nil {
		return m, nil
	}

	// Update state
	switch msg.NewState {
	case block.BlockStateStreaming:
		b.StartStreaming()
	case block.BlockStateComplete:
		b.MarkComplete()
		// If this was the active block, clear it
		if m.activeBlock != nil && m.activeBlock.ID == msg.BlockID {
			m.blockContainer.ClearActiveBlock()
			m.activeBlock = nil
		}
	case block.BlockStateError:
		b.MarkError(msg.Error)
		if m.activeBlock != nil && m.activeBlock.ID == msg.BlockID {
			m.blockContainer.ClearActiveBlock()
			m.activeBlock = nil
		}
	}

	// Refresh viewport
	m.viewport.SetContent(renderConversation(m))

	return m, nil
}

// handleToolBlockStarted handles the start of a tool execution.
func handleToolBlockStarted(m Model, msg ToolBlockStartedMsg) (tea.Model, tea.Cmd) {
	if m.blockContainer == nil {
		return m, nil
	}

	// Create a new tool block
	toolBlock := block.NewToolBlock(msg.ToolName, msg.ToolInput)
	toolBlock.ID = msg.BlockID // Use provided ID if set

	// Add as child of parent (usually an AssistantBlock)
	if msg.ParentID != "" {
		m.blockContainer.AddChildBlock(msg.ParentID, toolBlock)
	} else {
		m.blockContainer.AddBlock(toolBlock)
	}

	// Refresh viewport
	m.viewport.SetContent(renderConversation(m))
	m.viewport.GotoBottom()

	return m, nil
}

// handleToolBlockCompleted handles tool execution completion.
func handleToolBlockCompleted(m Model, msg ToolBlockCompletedMsg) (tea.Model, tea.Cmd) {
	if m.blockContainer == nil {
		return m, nil
	}

	b := m.blockContainer.GetBlock(msg.BlockID)
	if b == nil {
		return m, nil
	}

	// Update tool block with results
	b.SetToolOutput(msg.Output, msg.Success, time.Duration(msg.DurationMs)*time.Millisecond)

	if msg.Error != nil {
		b.MarkError(msg.Error)
	}

	// Refresh viewport
	m.viewport.SetContent(renderConversation(m))

	return m, nil
}

// handleBlockFocus handles focus changes between blocks.
func handleBlockFocus(m Model, msg BlockFocusMsg) (tea.Model, tea.Cmd) {
	if m.blockContainer == nil {
		return m, nil
	}

	m.blockContainer.SetFocusedBlock(msg.BlockID)

	// Refresh viewport to show focus highlight
	m.viewport.SetContent(renderConversation(m))

	return m, nil
}

// handleBlockToggle handles toggling a block's collapsed state.
func handleBlockToggle(m Model, msg BlockToggleMsg) (tea.Model, tea.Cmd) {
	if m.blockContainer == nil {
		return m, nil
	}

	b := m.blockContainer.GetBlock(msg.BlockID)
	if b != nil {
		b.Toggle()
		m.viewport.SetContent(renderConversation(m))
	}

	return m, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// BLOCK-AWARE STREAMING HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

// handleStreamChunkWithBlocks processes streaming chunks when block system is enabled.
// It creates and updates blocks based on the chunk's BlockType field.
func handleStreamChunkWithBlocks(m Model, msg StreamChunkMsg) (tea.Model, tea.Cmd) {
	chunk := msg.Chunk

	// If no active block, this shouldn't happen - fall back to legacy handling
	if m.activeBlock == nil {
		return handleStreamChunk(m, msg)
	}

	var cmds []tea.Cmd

	// Handle block start - create new child block if needed
	if chunk.IsBlockStart && chunk.BlockType != "" {
		var newBlock *block.Block

		switch chunk.BlockType {
		case "tool":
			newBlock = block.NewToolBlock(chunk.ToolName, chunk.ToolInput)
		case "code":
			newBlock = block.NewCodeBlock(chunk.Content, chunk.CodeLanguage)
		case "thinking":
			newBlock = block.NewThinkingBlock(chunk.Content)
		default:
			newBlock = block.NewTextBlock(chunk.Content)
		}

		// Add as child of active assistant block
		m.blockContainer.AddChildBlock(m.activeBlock.ID, newBlock)

		// Use the block ID from chunk if provided
		if chunk.BlockID != "" {
			// ID is already set during creation via GenerateID
		}
	} else if chunk.BlockID != "" {
		// Update existing block
		b := m.blockContainer.GetBlock(chunk.BlockID)
		if b != nil {
			b.AppendContent(chunk.Content)

			// Handle block end
			if chunk.IsBlockEnd {
				b.MarkComplete()
			}
		}
	} else {
		// No block ID - append to active block's content
		m.activeBlock.AppendContent(chunk.Content)
	}

	// Continue reading from stream
	cmds = append(cmds, streamReaderCmd(m.backend.StreamChannel()))

	// Debounce rendering
	if time.Since(m.lastRenderTime) > 100*time.Millisecond {
		m.viewport.SetContent(renderConversation(m))
		m.viewport.GotoBottom()
		m.lastRenderTime = time.Now()
	}

	return m, tea.Batch(cmds...)
}

// ═══════════════════════════════════════════════════════════════════════════════
// BLOCK ACTION HANDLER (CR-002)
// ═══════════════════════════════════════════════════════════════════════════════

// handleBlockAction executes an action on the currently focused block.
func handleBlockAction(m Model, action block.ActionType) (tea.Model, tea.Cmd) {
	if m.blockNavigator == nil || m.actionExecutor == nil {
		return m, nil
	}

	// Get focused block
	focusedBlockID := m.blockNavigator.GetFocusedBlockID()
	if focusedBlockID == "" {
		// No block focused - try to focus first block
		m.blockNavigator.FocusFirst()
		focusedBlockID = m.blockNavigator.GetFocusedBlockID()
		if focusedBlockID == "" {
			return m, nil
		}
	}

	// Execute the action
	result := m.actionExecutor.Execute(focusedBlockID, action)

	// Handle results
	if result.ClipboardContent != "" {
		// TODO: Implement clipboard copy
		// For now, we just acknowledge the action
		// In a real implementation, we'd use a clipboard library
	}

	// Refresh viewport if needed
	if result.RequiresRefresh {
		m.viewport.SetContent(renderConversation(m))
	}

	// Handle regeneration (creates a new branch)
	if result.NewBranchID != "" {
		// TODO: Switch to the new branch and initiate regeneration
		// This would trigger a new AI request from the branch point
	}

	return m, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// AUDIO DEVICE HANDLERS
// ═══════════════════════════════════════════════════════════════════════════════

// handleAudioDevicesLoaded handles the response from fetching audio devices.
func handleAudioDevicesLoaded(m Model, msg AudioDevicesLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		// Add error message to chat
		m.AddErrorMessage("Failed to load audio devices: " + msg.Error.Error())
		m.activeModal = ModalNone
		return m, nil
	}

	// Convert AudioDeviceInfo to modals.AudioDevice
	inputDevices := make([]modals.AudioDevice, len(msg.InputDevices))
	for i, dev := range msg.InputDevices {
		inputDevices[i] = modals.AudioDevice{
			Index:      dev.Index,
			Name:       dev.Name,
			Channels:   dev.Channels,
			SampleRate: dev.SampleRate,
			IsInput:    true,
		}
	}

	outputDevices := make([]modals.AudioDevice, len(msg.OutputDevices))
	for i, dev := range msg.OutputDevices {
		outputDevices[i] = modals.AudioDevice{
			Index:      dev.Index,
			Name:       dev.Name,
			Channels:   dev.Channels,
			SampleRate: dev.SampleRate,
			IsInput:    false,
		}
	}

	// Create the modals.DevicesLoadedMsg and pass to device selector
	devicesMsg := modals.DevicesLoadedMsg{
		InputDevices:  inputDevices,
		OutputDevices: outputDevices,
		CurrentInput:  msg.CurrentInput,
		CurrentOutput: msg.CurrentOutput,
		Error:         nil,
	}

	if m.deviceSelector != nil {
		m.deviceSelector.SetDevices(devicesMsg)
	}

	return m, nil
}

// handleAudioDeviceSet handles the response from setting an audio device.
func handleAudioDeviceSet(m Model, msg AudioDeviceSetMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		m.AddErrorMessage("Failed to set audio device: " + msg.Error.Error())
		return m, nil
	}

	// Success - show confirmation
	deviceType := "speaker"
	if msg.IsInput {
		deviceType = "microphone"
	}

	m.AddSystemMessage("Audio " + deviceType + " changed to: " + msg.Device.Name)

	// Close the modal
	m.activeModal = ModalNone

	return m, nil
}

// handleDeviceSelected handles device selection from the device selector modal.
func handleDeviceSelected(m Model, msg modals.DeviceSelectedMsg) (tea.Model, tea.Cmd) {
	// Trigger the actual device change via HTTP API
	return m, SetAudioDeviceCmd(msg.Device.Index, msg.IsInput)
}
