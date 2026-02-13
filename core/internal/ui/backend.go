// Package ui provides the Charmbracelet TUI framework integration for Cortex.
// It defines the interface between the Bubble Tea UI and the Cortex backend.
package ui

import (
	"time"

	"github.com/normanking/cortex/internal/ui/types"
)

// ModelInfo is an alias for types.ModelInfo for backward compatibility.
type ModelInfo = types.ModelInfo

// Backend defines the interface for TUI-backend communication.
// This abstraction allows the TUI to work with different backend implementations
// (orchestrator, direct LLM, mock for testing, etc.).
type Backend interface {
	// SendMessage sends a user message to the backend and returns a channel
	// that streams response chunks back to the UI.
	SendMessage(content string) (<-chan StreamChunk, error)

	// StreamChannel returns a read-only channel for receiving streaming chunks.
	// This can be used for long-running operations where the backend pushes
	// updates asynchronously.
	StreamChannel() <-chan StreamChunk

	// CancelStream cancels the current streaming operation.
	// Returns an error if no stream is active or cancellation fails.
	CancelStream() error

	// GetModels returns a list of available AI models from all configured providers.
	GetModels() ([]ModelInfo, error)

	// GetSessions returns a list of conversation sessions.
	// Can be used for session history, switching between conversations, etc.
	GetSessions() ([]SessionInfo, error)
}

// StreamChunk represents a single chunk of a streaming response.
// The backend sends these incrementally as the AI generates output.
type StreamChunk struct {
	// Content is the text content of this chunk (could be partial sentence/word)
	Content string

	// Done indicates this is the final chunk in the stream
	Done bool

	// Error carries any error that occurred during streaming.
	// If Error is set, this is typically the last chunk and Done will be true.
	Error error

	// Metadata can carry additional information (token count, model used, etc.)
	Metadata map[string]interface{}

	// ─────────────────────────────────────────────────────────────────────────
	// Block System Fields (CR-002)
	// ─────────────────────────────────────────────────────────────────────────

	// BlockID is the ID of the block this chunk belongs to (when block system is enabled)
	BlockID string

	// BlockType indicates what type of block this chunk is for
	// Common values: "text", "code", "tool", "thinking"
	BlockType string

	// IsBlockStart indicates this is the first chunk for a new block
	IsBlockStart bool

	// IsBlockEnd indicates this is the final chunk for this block
	IsBlockEnd bool

	// ToolName is set when BlockType is "tool" - indicates which tool is being called
	ToolName string

	// ToolInput is the input/arguments for a tool call
	ToolInput string

	// CodeLanguage is set when BlockType is "code" - indicates the programming language
	CodeLanguage string
}

// Note: ModelInfo is defined in internal/ui/types to avoid import cycles.
// The type alias above (type ModelInfo = types.ModelInfo) provides backward compatibility.

// SessionInfo describes a conversation session.
type SessionInfo struct {
	// ID is the unique session identifier
	ID string

	// Name is the user-assigned name for this session
	Name string

	// CreatedAt is when the session was created
	CreatedAt time.Time

	// UpdatedAt is when the session was last modified
	UpdatedAt time.Time

	// Messages is the count of messages in this session
	Messages int

	// Model is the AI model being used for this session
	Model string

	// Tags are optional labels for organizing sessions
	Tags []string
}
