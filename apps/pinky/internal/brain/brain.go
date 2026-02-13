// Package brain defines the Brain interface and implementations
// for Pinky's cognitive processing.
package brain

import (
	"context"
	"time"

	"github.com/normanking/pinky/internal/persona"
)

// BrainMode indicates whether the brain is embedded or remote
type BrainMode string

const (
	ModeEmbedded BrainMode = "embedded"
	ModeRemote   BrainMode = "remote"
)

// Brain is the interface for Pinky's cognitive engine
type Brain interface {
	// Think processes a request and returns a response
	Think(ctx context.Context, req *ThinkRequest) (*ThinkResponse, error)

	// ThinkStream returns a channel of response chunks for streaming
	ThinkStream(ctx context.Context, req *ThinkRequest) (<-chan *ThinkChunk, error)

	// Remember stores a memory
	Remember(ctx context.Context, memory *Memory) error

	// Recall retrieves relevant memories
	Recall(ctx context.Context, query string, limit int) ([]Memory, error)

	// Ping checks brain connectivity
	Ping(ctx context.Context) error

	// Mode returns whether this is embedded or remote
	Mode() BrainMode
}

// ThinkRequest contains all context for a thinking operation
type ThinkRequest struct {
	UserID      string
	Persona     *persona.Persona
	Messages    []Message
	Memories    []Memory
	Tools       []ToolSpec
	MaxTokens   int
	Temperature float64
	Stream      bool
}

// ThinkResponse contains the brain's response
type ThinkResponse struct {
	Content   string
	ToolCalls []ToolCall
	Reasoning string // Visible thinking (if verbose mode)
	Usage     TokenUsage
	Done      bool
}

// ThinkChunk is a streaming response chunk
type ThinkChunk struct {
	Content   string
	ToolCalls []ToolCall
	Done      bool
	Error     error
}

// Message represents a conversation message
type Message struct {
	Role      string    // "user", "assistant", "system", "tool"
	Content   string
	ToolCalls []ToolCall
	ToolResults []ToolResult
	Timestamp time.Time
}

// ToolSpec describes an available tool
type ToolSpec struct {
	Name        string
	Description string
	Parameters  map[string]ParameterSpec
}

// ParameterSpec describes a tool parameter
type ParameterSpec struct {
	Type        string
	Description string
	Required    bool
	Default     any
}

// ToolCall represents a request to execute a tool
type ToolCall struct {
	ID     string
	Tool   string
	Input  map[string]any
	Reason string // Why the brain wants to run this
}

// ToolResult contains the output of a tool execution
type ToolResult struct {
	ToolCallID string
	Success    bool
	Output     string
	Error      string
}

// Memory represents a stored memory
type Memory struct {
	ID           string
	UserID       string
	Type         MemoryType
	Content      string
	Embedding    []float64
	Importance   float64
	Source       string
	Context      map[string]string
	TemporalTags []TemporalTag // Time-related metadata for temporal recall
	CreatedAt    time.Time
	AccessedAt   time.Time
	AccessCount  int
}

// TemporalTag stores time-related metadata for a memory.
type TemporalTag struct {
	Type  string    // "relative", "absolute", "recurring"
	Value string    // The original expression (e.g., "yesterday")
	Time  time.Time // The computed absolute time
}

// MemoryType categorizes memories
type MemoryType string

const (
	MemoryEpisodic   MemoryType = "episodic"   // Events, conversations
	MemorySemantic   MemoryType = "semantic"   // Facts, knowledge
	MemoryProcedural MemoryType = "procedural" // How to do things
)

// TokenUsage tracks token consumption
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}
