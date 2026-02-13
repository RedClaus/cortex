// Package agent implements context building for the agentic loop.
package agent

import (
	"fmt"
	"strings"

	"github.com/normanking/pinky/internal/brain"
	"github.com/normanking/pinky/internal/persona"
)

// ContextBuilder constructs prompts for the brain with proper formatting.
type ContextBuilder struct {
	maxTokens     int // Model's context limit
	reserveOutput int // Reserve for response
}

// NewContextBuilder creates a new context builder.
func NewContextBuilder(maxTokens, reserveOutput int) *ContextBuilder {
	if maxTokens == 0 {
		maxTokens = 8192 // reasonable default
	}
	if reserveOutput == 0 {
		reserveOutput = 2048
	}
	return &ContextBuilder{
		maxTokens:     maxTokens,
		reserveOutput: reserveOutput,
	}
}

// BuildSystemPrompt creates the system prompt including persona and tool definitions.
func (cb *ContextBuilder) BuildSystemPrompt(persona *persona.Persona, tools []brain.ToolSpec) string {
	var parts []string

	// Base system prompt
	basePrompt := `You are Pinky, an intelligent AI assistant that can execute tools to help users.
You have access to various tools that let you run shell commands, manage files, interact with git, and more.

When you need to perform an action, use the appropriate tool. Tools are executed with user approval when needed.

Always explain what you're about to do before using a tool, and summarize the results after.`

	// Apply persona if provided
	if persona != nil && persona.SystemPrompt != "" {
		parts = append(parts, persona.SystemPrompt)
	} else {
		parts = append(parts, basePrompt)
	}

	// Add tool definitions
	if len(tools) > 0 {
		parts = append(parts, "\n## Available Tools\n")
		for _, tool := range tools {
			parts = append(parts, cb.formatToolDefinition(tool))
		}

		parts = append(parts, `
## Tool Usage Format

When you want to use a tool, respond with a tool call in this format:
<tool_call>
{"tool": "tool_name", "input": {"param": "value"}}
</tool_call>

Wait for the tool result before continuing. You can make multiple tool calls in sequence.
If a tool fails, explain the error and suggest alternatives.`)
	}

	return strings.Join(parts, "\n")
}

// formatToolDefinition formats a single tool for the prompt.
func (cb *ContextBuilder) formatToolDefinition(tool brain.ToolSpec) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "### %s\n", tool.Name)
	fmt.Fprintf(&sb, "%s\n", tool.Description)

	if len(tool.Parameters) > 0 {
		sb.WriteString("Parameters:\n")
		for name, param := range tool.Parameters {
			required := ""
			if param.Required {
				required = " (required)"
			}
			fmt.Fprintf(&sb, "  - %s: %s%s\n", name, param.Description, required)
		}
	}
	sb.WriteString("\n")

	return sb.String()
}

// BuildConversationContext formats the conversation history.
func (cb *ContextBuilder) BuildConversationContext(messages []brain.Message) string {
	var parts []string

	for _, msg := range messages {
		switch msg.Role {
		case "user":
			parts = append(parts, fmt.Sprintf("User: %s", msg.Content))
		case "assistant":
			if msg.Content != "" {
				parts = append(parts, fmt.Sprintf("Assistant: %s", msg.Content))
			}
			// Format tool calls
			for _, tc := range msg.ToolCalls {
				parts = append(parts, fmt.Sprintf("Assistant: [Using tool: %s]\n%s", tc.Tool, formatToolCallJSON(tc)))
			}
		case "tool":
			for _, tr := range msg.ToolResults {
				status := "success"
				content := tr.Output
				if !tr.Success {
					status = "error"
					content = tr.Error
				}
				parts = append(parts, fmt.Sprintf("[Tool Result (%s)]: %s", status, content))
			}
		case "system":
			parts = append(parts, msg.Content)
		}
	}

	return strings.Join(parts, "\n\n")
}

// BuildMemoryContext formats memories for inclusion in the prompt.
func (cb *ContextBuilder) BuildMemoryContext(memories []brain.Memory) string {
	if len(memories) == 0 {
		return ""
	}

	var parts []string
	parts = append(parts, "## Relevant Context from Memory\n")

	for _, mem := range memories {
		parts = append(parts, fmt.Sprintf("- %s", mem.Content))
	}

	return strings.Join(parts, "\n")
}

// formatToolCallJSON formats a tool call as JSON-like for the prompt.
func formatToolCallJSON(tc brain.ToolCall) string {
	// Simple formatting, could use json.Marshal for more complex cases
	var params []string
	for k, v := range tc.Input {
		params = append(params, fmt.Sprintf("%q: %q", k, fmt.Sprint(v)))
	}
	return fmt.Sprintf(`{"tool": %q, "input": {%s}}`, tc.Tool, strings.Join(params, ", "))
}

// EstimateTokens provides a rough token count estimate.
// This is a simple approximation; real token counting requires the tokenizer.
func (cb *ContextBuilder) EstimateTokens(text string) int {
	// Rough estimate: ~4 characters per token on average
	return len(text) / 4
}

// TrimToFit trims the conversation to fit within token limits.
func (cb *ContextBuilder) TrimToFit(messages []brain.Message, systemPrompt string) []brain.Message {
	budget := cb.maxTokens - cb.reserveOutput - cb.EstimateTokens(systemPrompt)

	// Start from the most recent messages
	var result []brain.Message
	totalTokens := 0

	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		msgTokens := cb.EstimateTokens(msg.Content)

		if totalTokens+msgTokens > budget {
			break
		}

		result = append([]brain.Message{msg}, result...)
		totalTokens += msgTokens
	}

	return result
}
