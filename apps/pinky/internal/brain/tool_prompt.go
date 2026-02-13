// Package brain provides tool prompt generation and parsing for prompt-based tool calling.
// This approach allows any LLM to use tools without native function calling support.
package brain

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ToolsDescription generates the tool descriptions section for the system prompt.
// This is used for prompt-based tool calling where the LLM outputs tool calls in a
// structured text format rather than using native function calling.
func ToolsDescription(tools []ToolSpec) string {
	if len(tools) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n## Available Tools\n\n")
	sb.WriteString("You can use these tools to help complete tasks. To use a tool, include a tool call in your response using this exact format:\n\n")
	sb.WriteString("```\n<tool>tool_name</tool><params>{\"param_name\": \"value\"}</params>\n```\n\n")
	sb.WriteString("Important:\n")
	sb.WriteString("- Use only ONE tool call per response\n")
	sb.WriteString("- Wait for the tool result before calling another tool\n")
	sb.WriteString("- The params must be valid JSON\n")
	sb.WriteString("- Only use tools when necessary to complete the user's request\n\n")
	sb.WriteString("### Tools:\n\n")

	for _, tool := range tools {
		sb.WriteString(fmt.Sprintf("**%s**: %s\n", tool.Name, tool.Description))

		if len(tool.Parameters) > 0 {
			sb.WriteString("Parameters:\n")
			for paramName, paramSpec := range tool.Parameters {
				required := ""
				if paramSpec.Required {
					required = " (required)"
				}
				sb.WriteString(fmt.Sprintf("  - `%s` (%s)%s: %s\n",
					paramName, paramSpec.Type, required, paramSpec.Description))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// ParseToolCalls extracts tool calls from an LLM response.
// It looks for the pattern: <tool>tool_name</tool><params>{"param": "value"}</params>
// Returns the extracted tool calls and the cleaned response text (with tool calls removed).
func ParseToolCalls(response string) ([]ToolCall, string) {
	var calls []ToolCall
	cleanedResponse := response

	for {
		// Find <tool> tag
		toolStart := strings.Index(cleanedResponse, "<tool>")
		if toolStart == -1 {
			break
		}

		// Find </tool>
		toolEndTag := "</tool>"
		toolEnd := strings.Index(cleanedResponse[toolStart:], toolEndTag)
		if toolEnd == -1 {
			break
		}
		toolEnd += toolStart

		// Extract tool name
		toolName := cleanedResponse[toolStart+6 : toolEnd]
		toolName = strings.TrimSpace(toolName)

		// Find <params> tag after </tool>
		afterTool := cleanedResponse[toolEnd+len(toolEndTag):]
		paramsStart := strings.Index(afterTool, "<params>")
		if paramsStart == -1 {
			// No params tag - remove tool call and continue
			cleanedResponse = cleanedResponse[:toolStart] + afterTool
			continue
		}

		// Find </params>
		paramsEnd := strings.Index(afterTool[paramsStart:], "</params>")
		if paramsEnd == -1 {
			// No closing params tag - remove what we found and continue
			cleanedResponse = cleanedResponse[:toolStart] + afterTool[paramsStart+8:]
			continue
		}
		paramsEnd += paramsStart

		// Extract params JSON
		paramsJSON := afterTool[paramsStart+8 : paramsEnd]
		paramsJSON = strings.TrimSpace(paramsJSON)

		// Parse the JSON
		var params map[string]any
		if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
			// Invalid JSON - try to continue
			cleanedResponse = cleanedResponse[:toolStart] + afterTool[paramsEnd+9:]
			continue
		}

		// Create the tool call
		call := ToolCall{
			ID:    fmt.Sprintf("call_%d", len(calls)+1),
			Tool:  toolName,
			Input: params,
		}
		calls = append(calls, call)

		// Remove the entire tool call from the response
		fullEnd := toolEnd + len(toolEndTag) + paramsEnd + 9 // 9 = len("</params>")
		cleanedResponse = cleanedResponse[:toolStart] + cleanedResponse[fullEnd:]
	}

	// Clean up any extra whitespace
	cleanedResponse = strings.TrimSpace(cleanedResponse)

	return calls, cleanedResponse
}

// FormatToolResult formats a tool result for inclusion in the conversation.
// This creates a message that can be added to the conversation history.
func FormatToolResult(toolName string, result ToolResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n[Tool Result: %s]\n", toolName))
	if result.Success {
		sb.WriteString(result.Output)
	} else {
		sb.WriteString(fmt.Sprintf("Error: %s", result.Error))
	}
	sb.WriteString("\n")
	return sb.String()
}
