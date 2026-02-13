// Package ui provides message data structures for the TUI.
package ui

import (
	"strings"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// MESSAGE ROLES
// ═══════════════════════════════════════════════════════════════════════════════

// MessageRole represents the role/sender of a message.
type MessageRole int

const (
	// RoleUser is a message from the human user
	RoleUser MessageRole = iota

	// RoleAssistant is a message from the AI assistant
	RoleAssistant

	// RoleSystem is a system message (errors, notifications, etc.)
	RoleSystem
)

// String returns the string representation of a MessageRole.
func (r MessageRole) String() string {
	switch r {
	case RoleUser:
		return "user"
	case RoleAssistant:
		return "assistant"
	case RoleSystem:
		return "system"
	default:
		return "unknown"
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// MESSAGE STATE
// ═══════════════════════════════════════════════════════════════════════════════

// MessageState represents the current state of a message.
type MessageState int

const (
	// MessagePending means the message is waiting to be sent/processed
	MessagePending MessageState = iota

	// MessageStreaming means the message is currently being streamed from the AI
	MessageStreaming

	// MessageComplete means the message is fully received and rendered
	MessageComplete

	// MessageError means an error occurred while processing the message
	MessageError
)

// String returns the string representation of a MessageState.
func (s MessageState) String() string {
	switch s {
	case MessagePending:
		return "pending"
	case MessageStreaming:
		return "streaming"
	case MessageComplete:
		return "complete"
	case MessageError:
		return "error"
	default:
		return "unknown"
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// MESSAGE STRUCT
// ═══════════════════════════════════════════════════════════════════════════════

// Message represents a single message in the conversation.
// It tracks both the raw content and cached rendered output for performance.
type Message struct {
	// ID is a unique identifier for this message
	ID string

	// Role identifies who sent this message
	Role MessageRole

	// State tracks the current processing state
	State MessageState

	// RawContent is the unformatted message text.
	// For streaming messages, this is incrementally updated as chunks arrive.
	RawContent string

	// CachedRender is the pre-rendered markdown/formatted output.
	// This is populated after rendering to avoid re-rendering on every frame.
	// Empty string means it needs to be (re-)rendered.
	CachedRender string

	// Timestamp is when the message was created
	Timestamp time.Time

	// CompletedAt is when streaming finished (for calculating duration)
	CompletedAt time.Time

	// Error holds any error that occurred during processing
	Error error

	// Metadata stores additional message properties (model used, token count, etc.)
	Metadata map[string]interface{}
}

// ═══════════════════════════════════════════════════════════════════════════════
// MESSAGE METHODS
// ═══════════════════════════════════════════════════════════════════════════════

// NewUserMessage creates a new user message with the given content.
func NewUserMessage(content string) *Message {
	return &Message{
		ID:           generateMessageID(),
		Role:         RoleUser,
		State:        MessageComplete, // User messages are immediately complete
		RawContent:   content,
		CachedRender: "", // Will be rendered on first display
		Timestamp:    time.Now(),
		CompletedAt:  time.Now(),
		Metadata:     make(map[string]interface{}),
	}
}

// NewAssistantMessage creates a new assistant message in pending state.
func NewAssistantMessage() *Message {
	return &Message{
		ID:           generateMessageID(),
		Role:         RoleAssistant,
		State:        MessagePending,
		RawContent:   "",
		CachedRender: "",
		Timestamp:    time.Now(),
		Metadata:     make(map[string]interface{}),
	}
}

// NewSystemMessage creates a new system message with the given content.
func NewSystemMessage(content string) *Message {
	return &Message{
		ID:           generateMessageID(),
		Role:         RoleSystem,
		State:        MessageComplete,
		RawContent:   content,
		CachedRender: "",
		Timestamp:    time.Now(),
		CompletedAt:  time.Now(),
		Metadata:     make(map[string]interface{}),
	}
}

// AppendContent adds content to the message (used during streaming).
// This invalidates the cached render.
func (m *Message) AppendContent(content string) {
	m.RawContent += content
	m.CachedRender = "" // Invalidate cache
	if m.State == MessagePending {
		m.State = MessageStreaming
	}
}

// MarkComplete marks the message as complete and records completion time.
func (m *Message) MarkComplete() {
	m.State = MessageComplete
	m.CompletedAt = time.Now()
	m.CachedRender = "" // Invalidate cache to trigger final render
}

// MarkError marks the message as having an error.
func (m *Message) MarkError(err error) {
	m.State = MessageError
	m.Error = err
	m.CompletedAt = time.Now()
}

// Duration returns the time elapsed between message creation and completion.
func (m *Message) Duration() time.Duration {
	if m.CompletedAt.IsZero() {
		return time.Since(m.Timestamp)
	}
	return m.CompletedAt.Sub(m.Timestamp)
}

// IsComplete returns true if the message is in a terminal state.
func (m *Message) IsComplete() bool {
	return m.State == MessageComplete || m.State == MessageError
}

// IsStreaming returns true if the message is currently being streamed.
func (m *Message) IsStreaming() bool {
	return m.State == MessageStreaming
}

// NeedsRender returns true if the cached render is invalid and needs updating.
func (m *Message) NeedsRender() bool {
	return m.CachedRender == ""
}

// SetRender caches the rendered output for this message.
func (m *Message) SetRender(rendered string) {
	m.CachedRender = rendered
}

// Preview returns a short preview of the message content (first 100 chars).
func (m *Message) Preview() string {
	content := strings.ReplaceAll(m.RawContent, "\n", " ")
	if len(content) > 100 {
		return content[:97] + "..."
	}
	return content
}
