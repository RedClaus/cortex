// Package ui provides Bubble Tea message types and command functions.
package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ═══════════════════════════════════════════════════════════════════════════════
// MESSAGE TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// StreamChunkMsg carries a single chunk from a streaming AI response.
// This is sent repeatedly as the AI generates output.
type StreamChunkMsg struct {
	Chunk StreamChunk
}

// StreamDoneMsg signals that streaming is complete (success or error).
type StreamDoneMsg struct {
	MessageID string
	Error     error
}

// StreamErrorMsg signals an error during streaming.
type StreamErrorMsg struct {
	MessageID string
	Error     error
}

// GlamourRenderTickMsg is sent by the debounce timer to trigger markdown rendering.
// This prevents excessive rendering during fast typing or streaming.
type GlamourRenderTickMsg struct {
	MessageID string
}

// ModelSelectedMsg signals that the user selected a different AI model.
type ModelSelectedMsg struct {
	Model ModelInfo
}

// ModelsLoadedMsg carries the list of available models after fetching.
type ModelsLoadedMsg struct {
	Models []ModelInfo
	Error  error
}

// ThemeSelectedMsg signals a theme change request.
type ThemeSelectedMsg struct {
	ThemeName string
}

// ModeChangedMsg signals a mode switch (normal, settings, help, etc.).
type ModeChangedMsg struct {
	Mode string
}

// ClearHistoryMsg requests clearing the conversation history.
type ClearHistoryMsg struct{}

// ErrorMsg carries a general error to display in the UI.
type ErrorMsg struct {
	Error error
}

// SessionLoadedMsg carries a loaded conversation session.
type SessionLoadedMsg struct {
	Session SessionInfo
	Error   error
}

// SessionsLoadedMsg carries the list of available sessions.
type SessionsLoadedMsg struct {
	Sessions []SessionInfo
	Error    error
}

// MessageSentMsg confirms a user message was sent to the backend.
type MessageSentMsg struct {
	MessageID string
	Content   string
}

// ═══════════════════════════════════════════════════════════════════════════════
// COMMAND FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════════

// SendMessageCmd creates a Bubble Tea command that sends a message to the backend
// and returns a MessageSentMsg along with a command to start reading the stream.
// The caller should batch this with streamReaderCmd to start receiving chunks.
func SendMessageCmd(backend Backend, content string) tea.Cmd {
	return func() tea.Msg {
		// Send message and get stream channel
		_, err := backend.SendMessage(content)
		if err != nil {
			return ErrorMsg{Error: err}
		}

		// Return confirmation that message was sent
		// The caller should then call streamReaderCmd with backend.StreamChannel()
		return MessageSentMsg{
			MessageID: generateMessageID(),
			Content:   content,
		}
	}
}

// streamReaderCmd reads from a stream channel and converts chunks to tea.Msg.
// This should be called after SendMessageCmd to start receiving stream chunks.
func streamReaderCmd(ch <-chan StreamChunk) tea.Cmd {
	return func() tea.Msg {
		// Wait for next chunk
		chunk, ok := <-ch
		if !ok {
			// Channel closed - stream is done
			return StreamDoneMsg{}
		}

		// Check for error
		if chunk.Error != nil {
			return StreamErrorMsg{
				Error: chunk.Error,
			}
		}

		// Check if this is the final chunk
		if chunk.Done {
			return StreamDoneMsg{}
		}

		// Return the chunk - the Update function will call streamReaderCmd again
		// to continue reading the stream
		return StreamChunkMsg{Chunk: chunk}
	}
}

// ScheduleGlamourRender returns a command that waits 100ms before triggering
// a markdown render. This debounces rendering during fast streaming.
func ScheduleGlamourRender(messageID string) tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return GlamourRenderTickMsg{MessageID: messageID}
	})
}

// FetchModelsCmd fetches the list of available models from the backend.
func FetchModelsCmd(backend Backend) tea.Cmd {
	return func() tea.Msg {
		models, err := backend.GetModels()
		return ModelsLoadedMsg{
			Models: models,
			Error:  err,
		}
	}
}

// FetchSessionsCmd fetches the list of conversation sessions from the backend.
func FetchSessionsCmd(backend Backend) tea.Cmd {
	return func() tea.Msg {
		sessions, err := backend.GetSessions()
		return SessionsLoadedMsg{
			Sessions: sessions,
			Error:    err,
		}
	}
}

// CancelStreamCmd cancels the current streaming operation.
func CancelStreamCmd(backend Backend) tea.Cmd {
	return func() tea.Msg {
		err := backend.CancelStream()
		if err != nil {
			return ErrorMsg{Error: err}
		}
		return StreamDoneMsg{}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPER FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════════

// generateMessageID creates a unique ID for a message.
// In a real implementation, this should use a proper UUID library.
func generateMessageID() string {
	return time.Now().Format("20060102150405.000000")
}
