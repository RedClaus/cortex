// Package agent provides the agentic layer that orchestrates brain routing,
// skill learning, and autonomous task execution.
package agent

import (
	"context"
	"time"
)

// BrainInterface defines the common interface for all brain implementations.
// Both LocalBrain (lobes) and FrontierBrain (Claude/GPT-4) implement this.
type BrainInterface interface {
	// Process handles a cognitive task and returns a result.
	Process(ctx context.Context, input *BrainInput) (*BrainResult, error)

	// Type returns the brain type identifier ("local" or "frontier").
	Type() string

	// Available returns true if the brain is ready to process requests.
	Available() bool
}

// BrainInput represents input to a brain for processing.
type BrainInput struct {
	// Query is the user's request or task description.
	Query string `json:"query"`

	// Context provides additional context for the task.
	Context map[string]interface{} `json:"context,omitempty"`

	// SystemPrompt overrides the default system prompt if provided.
	SystemPrompt string `json:"system_prompt,omitempty"`

	// ConversationHistory provides prior conversation for context.
	ConversationHistory []Message `json:"conversation_history,omitempty"`

	// Tools available for the brain to use.
	Tools []ToolDefinition `json:"tools,omitempty"`

	// MaxTokens limits the response length.
	MaxTokens int `json:"max_tokens,omitempty"`

	// Temperature controls response randomness (0.0-1.0).
	Temperature float64 `json:"temperature,omitempty"`
}

// Message represents a conversation message.
type Message struct {
	Role    string `json:"role"`    // "user", "assistant", "system"
	Content string `json:"content"`
}

// ToolDefinition describes a tool the brain can use.
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// BrainResult represents the output from a brain.
type BrainResult struct {
	// Content is the main response text.
	Content string `json:"content"`

	// Reasoning explains the brain's thought process (if available).
	Reasoning string `json:"reasoning,omitempty"`

	// ToolCalls contains any tools the brain wants to invoke.
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// Confidence indicates how confident the brain is (0.0-1.0).
	Confidence float64 `json:"confidence"`

	// Source identifies which brain produced this result.
	Source string `json:"source"` // "local", "frontier:claude", "frontier:openai"

	// Model identifies the specific model used.
	Model string `json:"model,omitempty"`

	// TokensUsed tracks token consumption (for cost tracking).
	TokensUsed int `json:"tokens_used,omitempty"`

	// Latency records how long processing took.
	Latency time.Duration `json:"latency"`

	// Success indicates if processing completed without error.
	Success bool `json:"success"`

	// Error contains any error message if Success is false.
	Error string `json:"error,omitempty"`
}

// ToolCall represents a request to execute a tool.
type ToolCall struct {
	ID     string                 `json:"id"`
	Name   string                 `json:"name"`
	Params map[string]interface{} `json:"params"`
}

// Skill represents a learned pattern from successful executions.
type Skill struct {
	ID          string            `json:"id"`
	Intent      string            `json:"intent"`       // What the user wanted
	Tool        string            `json:"tool"`         // What tool was used
	Params      map[string]string `json:"params"`       // Parameters used
	Success     bool              `json:"success"`      // Was it successful
	SuccessRate float64           `json:"success_rate"` // Historical success rate
	UseCount    int               `json:"use_count"`    // How many times used
	Source      string            `json:"source"`       // "local" or "frontier"
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}
