// Package a2a provides A2A Protocol v0.3.0 client implementation
package a2a

import (
	"time"
)

// AgentCard represents the /.well-known/agent-card.json response
type AgentCard struct {
	Name               string            `json:"name"`
	Description        string            `json:"description"`
	Version            string            `json:"version"`
	ProtocolVersion    string            `json:"protocolVersion"`
	URL                string            `json:"url"`
	PreferredTransport string            `json:"preferredTransport"`
	Capabilities       AgentCapabilities `json:"capabilities"`
	Skills             []AgentSkill      `json:"skills"`
	DefaultInputModes  []string          `json:"defaultInputModes"`
	DefaultOutputModes []string          `json:"defaultOutputModes"`
}

// AgentCapabilities describes what the agent supports
type AgentCapabilities struct {
	Streaming              bool `json:"streaming"`
	PushNotifications      bool `json:"pushNotifications"`
	StateTransitionHistory bool `json:"stateTransitionHistory"`
}

// AgentSkill represents a skill the agent can perform
type AgentSkill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
	Examples    []string `json:"examples,omitempty"`
	InputModes  []string `json:"inputModes,omitempty"`
	OutputModes []string `json:"outputModes,omitempty"`
}

// Message represents an A2A message
type Message struct {
	Role     string         `json:"role"` // "user" or "agent"
	Parts    []Part         `json:"parts"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Part is a part of a message (text or data)
type Part interface {
	PartType() string
}

// TextPart contains text content (A2A v0.3.0 uses "kind" not "type")
type TextPart struct {
	Kind     string         `json:"kind"` // "text"
	Text     string         `json:"text"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

func (t TextPart) PartType() string { return "text" }

// DataPart contains structured data
type DataPart struct {
	Kind     string         `json:"kind"` // "data"
	Data     map[string]any `json:"data"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

func (d DataPart) PartType() string { return "data" }

// FilePart contains file/binary data
type FilePart struct {
	Kind     string         `json:"kind"` // "file"
	Name     string         `json:"name,omitempty"`
	MimeType string         `json:"mimeType,omitempty"`
	Bytes    string         `json:"bytes"` // base64 encoded
	Metadata map[string]any `json:"metadata,omitempty"`
}

func (f FilePart) PartType() string { return "file" }

// TaskState represents the state of a task
type TaskState string

const (
	TaskStateSubmitted TaskState = "submitted"
	TaskStateWorking   TaskState = "working"
	TaskStateCompleted TaskState = "completed"
	TaskStateFailed    TaskState = "failed"
	TaskStateCanceled  TaskState = "canceled"
)

// Task represents an A2A task
type Task struct {
	ID       string    `json:"id"`
	State    TaskState `json:"status"`
	Message  *Message  `json:"message,omitempty"`
	Artifact *Artifact `json:"artifact,omitempty"`
}

// Artifact represents an artifact produced during task execution
type Artifact struct {
	Name     string         `json:"name"`
	Type     string         `json:"type"` // "text" or "data"
	Content  any            `json:"content"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// TaskEvent represents an SSE task update event
type TaskEvent struct {
	EventType string    `json:"type"` // "status-update" or "artifact"
	TaskID    string    `json:"taskId"`
	State     TaskState `json:"status,omitempty"`
	Message   *Message  `json:"message,omitempty"`
	Artifact  *Artifact `json:"artifact,omitempty"`
	Final     bool      `json:"final"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
	ID      any    `json:"id"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  any             `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
	ID      any             `json:"id"`
}

// JSONRPCError represents a JSON-RPC error
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// MessageSendConfig defines configuration options for message/send requests
type MessageSendConfig struct {
	AcceptedOutputModes []string `json:"acceptedOutputModes,omitempty"`
	Blocking            *bool    `json:"blocking,omitempty"`
	HistoryLength       *int     `json:"historyLength,omitempty"`
}

// MessageSendParams are the parameters for the message/send method (A2A v0.3.0)
type MessageSendParams struct {
	Config   *MessageSendConfig `json:"configuration,omitempty"`
	Message  *Message           `json:"message"`
	Metadata map[string]any     `json:"metadata,omitempty"`
	Mode     string             `json:"mode,omitempty"` // "voice" for Voice Executive routing
}

// SendMessageOptions configures how a message is sent to CortexBrain
type SendMessageOptions struct {
	// Mode specifies the interaction mode ("voice" for Voice Executive)
	Mode string
	// Persona overrides the default persona ID for this request
	Persona string
	// ImageBase64 contains optional base64-encoded image data for vision
	ImageBase64 string
	// MimeType specifies the MIME type of the image (e.g., "image/png")
	MimeType string
	// Stream enables streaming responses via SSE (Server-Sent Events)
	// When true, responses are delivered incrementally as they're generated
	Stream bool
}

// NewTextMessage creates a new text message
func NewTextMessage(role, text string, metadata map[string]any) *Message {
	return &Message{
		Role: role,
		Parts: []Part{
			TextPart{Kind: "text", Text: text},
		},
		Metadata: metadata,
	}
}

// NewVisionMessage creates a message with text and image data (uses FilePart for A2A v0.3.0)
func NewVisionMessage(role, text, imageBase64, mimeType string, metadata map[string]any) *Message {
	return &Message{
		Role: role,
		Parts: []Part{
			TextPart{Kind: "text", Text: text},
			FilePart{
				Kind:     "file",
				MimeType: mimeType,
				Bytes:    imageBase64,
			},
		},
		Metadata: metadata,
	}
}

// ExtractText extracts all text from message parts
func (m *Message) ExtractText() string {
	var result string
	for _, part := range m.Parts {
		if tp, ok := part.(TextPart); ok {
			if result != "" {
				result += "\n"
			}
			result += tp.Text
		}
	}
	return result
}
